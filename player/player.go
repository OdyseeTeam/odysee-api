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
	SDHash      []byte
	Size        int
	ContentType string
	Blobs       *stream.SDBlob
}

type reflectedBlob struct {
	Hash          string `json:"blob_hash"`
	Number        int    `json:"blob_num"`
	IV            string `json:"iv"`
	DecodedIV     []byte `json:"-"`
	Length        int    `json:"length"`
	encryptedBody []byte
}

// PlayURI downloads and streams LBRY video content located at uri and delimited by rangeHeader
// (use rangeHeader := request.Header.Get("Range")).
// Streaming works like this:
//  1. Resolve stream hash through lbrynet daemon (see `resolve`)
//  2. Retrieve stream details (list of blob hashes and lengths, etc) by the SD hash from the reflector
//  (see `fetchData`)
//  3. Calculate which blobs contain the requested stream range
//  4. Sequentially download, decrypt and stream blobs to the provided writer (`streamBlobs`)
func PlayURI(uri string, rangeHeader string, writer http.ResponseWriter) (err error) {
	start, end := parseRange(rangeHeader)
	rs, err := newReflectedStream(uri)
	if err != nil {
		return err
	}
	err = rs.fetchData()
	if err != nil {
		return err
	}
	blobStart, blobEnd := rs.prepareWriter(start, end, writer)
	err = rs.streamBlobs(blobStart, blobEnd, writer)
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

// fetchData gets stream data from the reflector and does some blob magic along the way
func (s *reflectedStream) fetchData() error {
	if s.SDHash == nil {
		return errors.Err("No hash set, call `resolve` first")
	}
	monitor.Logger.WithFields(log.Fields{
		"uri": s.URI,
		"url": s.URL(),
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
	for _, b := range sdb.BlobInfos {
		if b.Length == stream.MaxBlobSize {
			s.Size += b.Length - 1
		} else {
			s.Size += b.Length
		}
		if err != nil {
			return err
		}
		if blobsSizes == "" {
			blobsSizes = fmt.Sprintf("%v", b.Length)
		} else {
			blobsSizes = fmt.Sprintf("%v+%v", blobsSizes, b.Length)
		}
	}

	// last padding is unguessable
	s.Size -= 15

	sort.Slice(sdb.BlobInfos, func(i, j int) bool {
		return sdb.BlobInfos[i].BlobNum < sdb.BlobInfos[j].BlobNum
	})

	monitor.Logger.WithFields(log.Fields{
		"blobs_number": len(sdb.BlobInfos),
		"blobs_sizes":  blobsSizes,
		"stream_size":  s.Size,
		"uri":          s.URI,
	}).Info("got stream data")
	s.Blobs = sdb
	return nil
}

// prepareWriter writes necessary range headers and sets the status, preparing HTTP for partial blob streaming.
// This should be called before streamBlobs
func (s *reflectedStream) prepareWriter(startByte, endByte int64, writer http.ResponseWriter) (startBlob, endBlob int) {
	if endByte == 0 {
		endByte = int64(s.Size - 1)
	}
	startBlob = int(startByte / (stream.MaxBlobSize - 2))
	endBlob = int(endByte / (stream.MaxBlobSize - 2))
	rangeStart := startBlob * (stream.MaxBlobSize - 1)
	rangeEnd := (endBlob + 1) * (stream.MaxBlobSize - 1)
	if rangeEnd > s.Size {
		rangeEnd = s.Size
	}
	resultingSize := rangeEnd + 1 - rangeStart
	writer.Header().Set("Content-Type", s.ContentType)
	writer.Header().Set("Accept-Ranges", "bytes")
	writer.Header().Set("Content-Length", fmt.Sprintf("%v", resultingSize))
	writer.Header().Set("Content-Range", fmt.Sprintf("bytes %v-%v/%v", rangeStart, rangeEnd, s.Size))
	writer.WriteHeader(http.StatusPartialContent)
	return startBlob, endBlob
}

func (s *reflectedStream) streamBlobs(blobStart, blobEnd int, writer http.ResponseWriter) error {
	for _, bi := range s.Blobs.BlobInfos[blobStart : blobEnd+1] {
		monitor.Logger.WithFields(log.Fields{
			"url": blobURL(bi),
		}).Info("requesting a blob")

		resp, err := http.Get(blobURL(bi))
		if err != nil {
			return err
		}
		defer resp.Body.Close()

		if resp.StatusCode == http.StatusOK {
			encryptedBody, err := ioutil.ReadAll(resp.Body)
			if err != nil {
				return err
			}
			decryptedBody, err := stream.DecryptBlob(stream.Blob(encryptedBody), s.Blobs.Key, bi.IV)
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

func blobURL(bi stream.BlobInfo) string {
	return reflectorURL + hex.EncodeToString(bi.BlobHash)
}
