package player

import (
	"encoding/hex"
	"encoding/json"
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
	URI         string          `json:"-"`
	SDHash      []byte          `json:"stream_hash"`
	Blobs       []reflectedBlob `json:"blobs"`
	Key         string          `json:"key"`
	DecodedKey  []byte          `json:"-"`
	Size        int             `json:"-"`
	ContentType string          `json:"-"`
}

type reflectedBlob struct {
	Hash          string `json:"blob_hash"`
	Number        int    `json:"blob_num"`
	IV            string `json:"iv"`
	DecodedIV     []byte `json:"-"`
	Length        int    `json:"length"`
	encryptedBody []byte
}

// PlayURI downloads and streams LBRY video content located at uri delimited by rangeHeader
// (use rangeHeader := request.Header.Get("Range"))
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

// resolve gets SD hash from the daemon for a given URI to enable further retrieval of stream content from
// reflector servers
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
	// httpResponse, err := httpClient.Do(httpRequest)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	// body, err := ioutil.ReadAll(resp.Body)
	err = json.NewDecoder(resp.Body).Decode(&s)
	// sdb := &stream.SDBlob{}
	// sdb.FromBlob(body)

	if err != nil {
		return err
	}
	decodedKey, err := hex.DecodeString(s.Key)
	if err != nil {
		return err
	}
	s.DecodedKey = decodedKey
	var blobsSizes string
	for n, blob := range s.Blobs {
		if blob.Length == stream.MaxBlobSize {
			s.Size += blob.Length - 1
		} else {
			s.Size += blob.Length
		}
		decodedIV, err := hex.DecodeString(blob.IV)
		if err != nil {
			return err
		}
		s.Blobs[n].DecodedIV = decodedIV
		if blobsSizes == "" {
			blobsSizes = fmt.Sprintf("%v", blob.Length)
		} else {
			blobsSizes = fmt.Sprintf("%v+%v", blobsSizes, blob.Length)
		}
	}
	// last padding is unguessable
	s.Size -= 15
	sort.Slice(s.Blobs, func(i, j int) bool { return s.Blobs[i].Number < s.Blobs[j].Number })
	monitor.Logger.WithFields(log.Fields{
		"blobs_number": len(s.Blobs),
		"blobs_sizes":  blobsSizes,
		"stream_size":  s.Size,
		"uri":          s.URI,
	}).Info("got stream data")
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

// streamBlobs downloads, decodes and streams blobs `blobStart-blobEnd` into a given HTTP response
func (s *reflectedStream) streamBlobs(blobStart, blobEnd int, writer http.ResponseWriter) error {
	for _, blob := range s.Blobs[blobStart : blobEnd+1] {
		monitor.Logger.WithFields(log.Fields{
			"url": blob.URL(),
		}).Info("requesting a blob")
		resp, err := http.Get(blob.URL())
		if err != nil {
			return err
		}
		defer resp.Body.Close()
		if resp.StatusCode == http.StatusOK {
			encryptedBody, err := ioutil.ReadAll(resp.Body)
			if err != nil {
				return err
			}
			blob.encryptedBody = encryptedBody
			decryptedBody, err := stream.DecryptBlob(
				stream.Blob(encryptedBody), s.DecodedKey, blob.DecodedIV)
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

func (b *reflectedBlob) decrypt(s *reflectedStream) (decryptedBody []byte, err error) {
	streamBlob := stream.Blob(b.encryptedBody)
	return streamBlob.Plaintext(s.DecodedKey, b.DecodedIV)
}

func (b *reflectedBlob) URL() string {
	return reflectorURL + b.Hash
}
