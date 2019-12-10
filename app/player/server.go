package player

import (
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/textproto"
	"strconv"
	"strings"
	"time"
)

// CopyN copies n bytes (or until an error) from src to dst.
// It returns the number of bytes copied and the earliest
// error encountered while copying.
// On return, written == n if and only if err == nil.
//
// If dst implements the ReaderFrom interface,
// the copy is implemented using it.
func CopyN(dst io.Writer, src io.Reader, n int64) (written int64, err error) {
	buf := make([]byte, 1024*256)
	written, err = io.CopyBuffer(dst, io.LimitReader(src, n), buf)
	if written == n {
		return n, nil
	}
	if written < n && err == nil {
		// src stopped early; must have been EOF.
		err = io.EOF
	}
	return
}

// errSeeker is returned by ServeContent's sizeFunc when the content
// doesn't seek properly. The underlying Seeker's error text isn't
// included in the sizeFunc reply so it's not sent over HTTP to end
// users.
var errSeeker = errors.New("seeker can't seek")

// errNoOverlap is returned by serveContent's parseRange if first-byte-pos of
// all of the byte-range-spec values is greater than the content size.
var errNoOverlap = errors.New("invalid range: failed to overlap")

// ServeContent replies to the request using the content in the
// provided ReadSeeker. The main benefit of ServeContent over io.Copy
// is that it handles Range requests properly, sets the MIME type, and
// handles If-Match, If-Unmodified-Since, If-None-Match, If-Modified-Since,
// and If-Range requests.
//
// If the response's Content-Type header is not set, ServeContent
// first tries to deduce the type from name's file extension and,
// if that fails, falls back to reading the first block of the content
// and passing it to DetectContentType.
// The name is otherwise unused; in particular it can be empty and is
// never sent in the response.
//
// If modtime is not the zero time or Unix epoch, ServeContent
// includes it in a Last-Modified header in the response. If the
// request includes an If-Modified-Since header, ServeContent uses
// modtime to decide whether the content needs to be sent at all.
//
// The content's Seek method must work: ServeContent uses
// a seek to the end of the content to determine its size.
//
// If the caller has set w's ETag header formatted per RFC 7232, section 2.3,
// ServeContent uses it to handle requests using If-Match, If-None-Match, or If-Range.
//
// Note that *os.File implements the io.ReadSeeker interface.
func ServeContent(w http.ResponseWriter, req *http.Request, name string, modtime time.Time, content io.ReadSeeker) {
	sizeFunc := func() (int64, error) {
		size, err := content.Seek(0, io.SeekEnd)
		if err != nil {
			return 0, errSeeker
		}
		_, err = content.Seek(0, io.SeekStart)
		if err != nil {
			return 0, errSeeker
		}
		return size, nil
	}
	serveContent(w, req, name, modtime, sizeFunc, content)
}

// if name is empty, filename is unknown. (used for mime type, before sniffing)
// if modtime.IsZero(), modtime is unknown.
// content must be seeked to the beginning of the file.
// The sizeFunc is called at most once. Its error, if any, is sent in the HTTP response.
func serveContent(w http.ResponseWriter, r *http.Request, name string, modtime time.Time, sizeFunc func() (int64, error), content io.ReadSeeker) {
	code := http.StatusOK

	size, err := sizeFunc()
	if err != nil {
		Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	// handle Content-Range header.
	sendSize := size
	var sendContent io.Reader = content
	if size >= 0 {
		ranges, err := parseRange(r.Header.Get("Range"), size)
		if err != nil {
			if err == errNoOverlap {
				w.Header().Set("Content-Range", fmt.Sprintf("bytes */%d", size))
			}
			Error(w, err.Error(), http.StatusRequestedRangeNotSatisfiable)
			return
		}
		if sumRangesSize(ranges) > size {
			// The total number of bytes in all the ranges
			// is larger than the size of the file by
			// itself, so this is probably an attack, or a
			// dumb client. Ignore the range request.
			ranges = nil
		}
		if len(ranges) == 1 {
			// RFC 7233, Section 4.1:
			// "If a single part is being transferred, the server
			// generating the 206 response MUST generate a
			// Content-Range header field, describing what range
			// of the selected representation is enclosed, and a
			// payload consisting of the range.
			// ...
			// A server MUST NOT generate a multipart response to
			// a request for a single range, since a client that
			// does not request multiple parts might not support
			// multipart responses."
			ra := ranges[0]
			if _, err := content.Seek(ra.start, io.SeekStart); err != nil {
				Error(w, err.Error(), http.StatusRequestedRangeNotSatisfiable)
				return
			}
			sendSize = ra.length
			code = http.StatusPartialContent
			w.Header().Set("Content-Range", ra.contentRange(size))
		}

		w.Header().Set("Accept-Ranges", "bytes")
		if w.Header().Get("Content-Encoding") == "" {
			w.Header().Set("Content-Length", strconv.FormatInt(sendSize, 10))
		}
	}

	w.WriteHeader(code)

	if r.Method != "HEAD" {
		io.CopyN(w, sendContent, sendSize)
	}
}

// Error replies to the request with the specified error message and HTTP code.
// It does not otherwise end the request; the caller should ensure no further
// writes are done to w.
// The error message should be plain text.
func Error(w http.ResponseWriter, error string, code int) {
	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	w.Header().Set("X-Content-Type-Options", "nosniff")
	w.WriteHeader(code)
	fmt.Fprintln(w, error)
}

// parseRange parses a Range header string as per RFC 7233.
// errNoOverlap is returned if none of the ranges overlap.
func parseRange(s string, size int64) ([]httpRange, error) {
	if s == "" {
		return nil, nil // header not present
	}
	const b = "bytes="
	if !strings.HasPrefix(s, b) {
		return nil, errors.New("invalid range")
	}
	var ranges []httpRange
	noOverlap := false
	for _, ra := range strings.Split(s[len(b):], ",") {
		ra = strings.TrimSpace(ra)
		if ra == "" {
			continue
		}
		i := strings.Index(ra, "-")
		if i < 0 {
			return nil, errors.New("invalid range")
		}
		start, end := strings.TrimSpace(ra[:i]), strings.TrimSpace(ra[i+1:])
		var r httpRange
		if start == "" {
			// If no start is specified, end specifies the
			// range start relative to the end of the file.
			i, err := strconv.ParseInt(end, 10, 64)
			if err != nil {
				return nil, errors.New("invalid range")
			}
			if i > size {
				i = size
			}
			r.start = size - i
			r.length = size - r.start
		} else {
			i, err := strconv.ParseInt(start, 10, 64)
			if err != nil || i < 0 {
				return nil, errors.New("invalid range")
			}
			if i >= size {
				// If the range begins after the size of the content,
				// then it does not overlap.
				noOverlap = true
				continue
			}
			r.start = i
			if end == "" {
				// If no end is specified, range extends to end of the file.
				r.length = size - r.start
			} else {
				i, err := strconv.ParseInt(end, 10, 64)
				if err != nil || r.start > i {
					return nil, errors.New("invalid range")
				}
				if i >= size {
					i = size - 1
				}
				r.length = i - r.start + 1
			}
		}
		ranges = append(ranges, r)
	}
	if noOverlap && len(ranges) == 0 {
		// The specified ranges did not overlap with the content.
		return nil, errNoOverlap
	}
	return ranges, nil
}

// httpRange specifies the byte range to be sent to the client.
type httpRange struct {
	start, length int64
}

func sumRangesSize(ranges []httpRange) (size int64) {
	for _, ra := range ranges {
		size += ra.length
	}
	return
}

func (r httpRange) contentRange(size int64) string {
	return fmt.Sprintf("bytes %d-%d/%d", r.start, r.start+r.length-1, size)
}

func (r httpRange) mimeHeader(contentType string, size int64) textproto.MIMEHeader {
	return textproto.MIMEHeader{
		"Content-Range": {r.contentRange(size)},
		"Content-Type":  {contentType},
	}
}
