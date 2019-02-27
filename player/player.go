package player

import (
	"encoding/hex"
	"fmt"
	"io/ioutil"
	"net/http"
	"regexp"
	"sort"
	"strconv"

	"github.com/lbryio/lbry.go/extras/errors"
	ljsonrpc "github.com/lbryio/lbry.go/extras/jsonrpc"
	"github.com/lbryio/lbry.go/stream"
	"github.com/lbryio/lbryweb.go/config"
	"github.com/lbryio/lbryweb.go/monitor"
	log "github.com/sirupsen/logrus"
)

const reflectorURL = "http://blobs.lbry.io/"

type reflectedStream struct {
	URI         string
	StartByte   int64
	EndByte     int64
	SDHash      []byte
	Size        int
	ContentType string
	SDBlob      *stream.SDBlob
}

// PlayURI downloads and streams LBRY video content located at uri and delimited by rangeHeader
// (use rangeHeader := request.Header.Get("Range")).
// Streaming works like this:
//  1. Resolve stream hash through lbrynet daemon (see resolve)
//  2. Retrieve stream details (list of blob hashes and lengths, etc) by the SD hash from the reflector
//  (see fetchData)
//  3. Calculate which blobs contain the requested stream range (getBlobsRange)
//	4. Prepare http writer with necessary headers (prepareWriter)
//  5. Sequentially download, decrypt and stream blobs to the provided writer (streamBlobs)
func PlayURI(uri string, rangeHeader string, w http.ResponseWriter) (err error) {
	rs, err := newReflectedStream(uri)
	if err != nil {
		return err
	}
	err = rs.fetchData()
	if err != nil {
		return err
	}
	rs.setRangeFromHeader(rangeHeader)
	blobStart, blobEnd := rs.getBlobsRange()
	rs.prepareWriter(w)
	err = rs.streamBlobs(blobStart, blobEnd, w)
	return err
}

func parseRange(header string) (int64, int64) {
	r := regexp.MustCompile(`bytes=(\d+)-(\d*)`)
	m := r.FindStringSubmatch(header)
	if len(m) == 0 {
		return 0, 0
	}
	start, err1 := strconv.ParseInt(m[1], 10, 64)
	end, err2 := strconv.ParseInt(m[2], 10, 64)
	if err1 != nil || err2 != nil {
		return 0, 0
	}
	if start < 0 {
		start = 0
	}
	if end < 0 {
		end = 0
	}
	if start > end {
		start = 0
		end = 0
	}
	return start, end
}

func newReflectedStream(uri string) (rs *reflectedStream, err error) {
	client := ljsonrpc.NewClient(config.Settings.GetString("Lbrynet"))
	rs = &reflectedStream{URI: uri}
	err = rs.resolve(client)
	return rs, err
}

func (s *reflectedStream) URL() string {
	return reflectorURL + string(s.SDHash)
}

func (s *reflectedStream) resolve(client *ljsonrpc.Client) error {
	if s.URI == "" {
		return errors.Err("stream URI is not set")
	}
	response, err := client.Resolve(s.URI)
	if err != nil {
		return err
	}
	source := (*response)[s.URI].Claim.Value.Stream.Source
	s.SDHash = source.Source
	s.ContentType = source.GetContentType()
	monitor.Logger.WithFields(log.Fields{
		"sd_hash":      fmt.Sprintf("%s", s.SDHash),
		"uri":          s.URI,
		"content_type": s.ContentType,
	}).Info("resolved uri")
	return nil
}

func (s *reflectedStream) fetchData() error {
	if s.SDHash == nil {
		return errors.Err("No hash set, call `resolve` first")
	}
	monitor.Logger.WithFields(log.Fields{
		"uri": s.URI, "url": s.URL(),
	}).Info("requesting stream data")

	resp, err := http.Get(s.URL())
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	body, err := ioutil.ReadAll(resp.Body)
	sdb := &stream.SDBlob{}
	sdb.UnmarshalJSON(body)

	if err != nil {
		return err
	}

	var blobsSizes string
	for _, bi := range sdb.BlobInfos {
		if bi.Length == stream.MaxBlobSize {
			s.Size += bi.Length - 1
		} else {
			s.Size += bi.Length
		}
		if err != nil {
			return err
		}
		if blobsSizes == "" {
			blobsSizes = fmt.Sprintf("%v", bi.Length)
		} else {
			blobsSizes = fmt.Sprintf("%v+%v", blobsSizes, bi.Length)
		}
	}

	// last padding is unguessable
	s.Size -= 15

	sort.Slice(sdb.BlobInfos, func(i, j int) bool {
		return sdb.BlobInfos[i].BlobNum < sdb.BlobInfos[j].BlobNum
	})
	s.SDBlob = sdb

	monitor.Logger.WithFields(log.Fields{
		"blobs_number": len(sdb.BlobInfos),
		"blobs_sizes":  blobsSizes,
		"stream_size":  s.Size,
		"uri":          s.URI,
	}).Info("got stream data")
	return nil
}

func (s *reflectedStream) setRange(startByte, endByte int64) {
	if endByte == 0 {
		endByte = int64(s.Size - 1)
	}
	s.StartByte = startByte
	s.EndByte = endByte
}

func (s *reflectedStream) setRangeFromHeader(h string) {
	startByte, endByte := parseRange(h)
	s.setRange(startByte, endByte)
}

func (s *reflectedStream) getBlobsRange() (startBlob, endBlob int) {
	startBlob = int(s.StartByte / (stream.MaxBlobSize - 2))
	endBlob = int(s.EndByte / (stream.MaxBlobSize - 2))
	rangeEnd := (endBlob + 1) * (stream.MaxBlobSize - 1)
	if rangeEnd > s.Size {
		rangeEnd = s.Size
	}
	return startBlob, endBlob
}

func (s *reflectedStream) prepareWriter(writer http.ResponseWriter) {
	startBlob, endBlob := s.getBlobsRange()
	rangeStart := startBlob * (stream.MaxBlobSize - 1)
	rangeEnd := (endBlob + 1) * (stream.MaxBlobSize - 1)
	resultingSize := rangeEnd + 1 - rangeStart
	writer.Header().Set("Content-Type", s.ContentType)
	writer.Header().Set("Accept-Ranges", "bytes")
	writer.Header().Set("Content-Length", fmt.Sprintf("%v", resultingSize))
	writer.Header().Set("Content-Range", fmt.Sprintf("bytes %v-%v/%v", rangeStart, rangeEnd, s.Size))
	writer.WriteHeader(http.StatusPartialContent)
}

func (s *reflectedStream) streamBlobs(blobStart, blobEnd int, writer http.ResponseWriter) error {
	for _, bi := range s.SDBlob.BlobInfos[blobStart : blobEnd+1] {
		url := blobInfoURL(bi)
		monitor.Logger.WithFields(log.Fields{
			"url": url,
		}).Info("requesting a blob")

		resp, err := http.Get(url)
		if err != nil {
			return err
		}
		defer resp.Body.Close()

		if resp.StatusCode == http.StatusOK {
			encryptedBody, err := ioutil.ReadAll(resp.Body)
			if err != nil {
				return err
			}
			decryptedBody, err := stream.DecryptBlob(stream.Blob(encryptedBody), s.SDBlob.Key, bi.IV)
			if err != nil {
				return err
			}
			writer.Write(decryptedBody)
		} else {
			return errors.Err("server responded with an unexpected status (%v)", resp.Status)
		}
	}
	return nil
}

func blobInfoURL(bi stream.BlobInfo) string {
	return reflectorURL + hex.EncodeToString(bi.BlobHash)
}
