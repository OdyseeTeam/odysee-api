package player

import (
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"sort"
	"time"

	"github.com/lbryio/lbrytv/app/router"
	"github.com/lbryio/lbrytv/app/users"
	"github.com/lbryio/lbrytv/config"
	"github.com/lbryio/lbrytv/internal/lbrynet"
	"github.com/lbryio/lbrytv/internal/metrics"
	"github.com/lbryio/lbrytv/internal/monitor"

	ljsonrpc "github.com/lbryio/lbry.go/v2/extras/jsonrpc"
	"github.com/lbryio/lbry.go/v2/stream"
)

const reflectorURL = "http://blobs.lbry.io"

type reflectedStream struct {
	URI          string
	StartByte    int64
	EndByte      int64
	SdHash       string
	Size         int64
	ContentType  string
	SDBlob       *stream.SDBlob
	seekOffset   int64
	reflectorURL string
	blobs        []*Blob
}

// Logger is a package-wide logger.
// Warning: will generate a lot of output if DEBUG loglevel is enabled.
var Logger = monitor.NewModuleLogger("player")

// PlayURI downloads and streams LBRY video content located at uri and delimited by rangeHeader
// (use rangeHeader := request.Header.Get("Range")).
// Streaming works like this:
// 1. Resolve stream hash through lbrynet daemon (see resolve)
// 2. Retrieve stream details (list of blob hashes and lengths, etc) by the SD hash from the reflector
// (see fetchData)
// 3. Implement io.ReadSeeker interface for http.ServeContent:
// - Seek simply implements io.Seeker
// - Read calculates boundaries and finds blobs that contain the requested stream range,
// then calls streamBlobs, which sequentially downloads and decrypts requested blobs
func PlayURI(uri string, w http.ResponseWriter, req *http.Request) error {
	metrics.PlayerStreamsRunning.Inc()
	defer metrics.PlayerStreamsRunning.Dec()

	s, err := newReflectedStream(uri, reflectorURL)
	if err != nil {
		return err
	}
	err = s.fetchData()
	if err != nil {
		return err
	}
	s.prepareWriter(w)

	Logger.LogF(monitor.F{
		"stream":    s.URI,
		"remote_ip": users.GetIPAddressForRequest(req),
	}).Info("stream requested")
	ServeContent(w, req, "test", time.Time{}, s)

	return err
}

func newReflectedStream(uri string, reflectorURL string) (rs *reflectedStream, err error) {
	sdkRouter := router.New(config.GetLbrynetServers())
	client := ljsonrpc.NewClient(sdkRouter.GetBalancedSDKAddress())
	rs = &reflectedStream{URI: uri, reflectorURL: reflectorURL}
	err = rs.resolve(client)
	return rs, err
}

func (s reflectedStream) blobNum() int {
	return len(s.SDBlob.BlobInfos) - 1
}

// Read implements io.ReadSeeker interface
func (s *reflectedStream) Read(dest []byte) (n int, err error) {
	var startOffsetInBlob int64

	bufferLen := len(dest)
	seekOffsetEnd := s.seekOffset + int64(bufferLen)
	blobNum := int(s.seekOffset / (stream.MaxBlobSize - 2))

	if blobNum == 0 {
		startOffsetInBlob = s.seekOffset - int64(blobNum*stream.MaxBlobSize)
	} else {
		startOffsetInBlob = s.seekOffset - int64(blobNum*stream.MaxBlobSize) + int64(blobNum)
	}

	start := time.Now()
	n, err = s.streamBlob(blobNum, startOffsetInBlob, dest)

	if err != nil {
		metrics.PlayerFailuresCount.Inc()
		Logger.LogF(monitor.F{
			"stream":         s.URI,
			"num":            fmt.Sprintf("%v/%v", blobNum+1, s.blobNum()),
			"current_offset": s.seekOffset,
			"offset_in_blob": startOffsetInBlob,
		}).Errorf("failed to read from blob stream after %vs: %v", time.Since(start).Seconds(), err)
		monitor.CaptureException(err, map[string]string{
			"stream":         s.URI,
			"num":            fmt.Sprintf("%v/%v", blobNum, s.blobNum()),
			"current_offset": fmt.Sprintf("%v", s.seekOffset),
			"offset_in_blob": fmt.Sprintf("%v", startOffsetInBlob),
		})
	} else {
		metrics.PlayerSuccessesCount.Inc()
		Logger.LogF(monitor.F{
			"buffer_len":     bufferLen,
			"num":            fmt.Sprintf("%v/%v", blobNum, s.blobNum()),
			"current_offset": s.seekOffset,
			"offset_in_blob": startOffsetInBlob,
		}).Debugf("read %v bytes (%v..%v) from blob stream", n, s.seekOffset, seekOffsetEnd-1)
	}

	s.seekOffset += int64(n)
	return n, err
}

// Seek implements io.ReadSeeker interface
func (s *reflectedStream) Seek(offset int64, whence int) (int64, error) {
	var (
		newSeekOffset int64
		whenceText    string
	)

	if whence == io.SeekEnd {
		newSeekOffset = s.Size - offset
		whenceText = "relative to end"
	} else if whence == io.SeekStart {
		newSeekOffset = offset
		whenceText = "relative to start"
	} else if whence == io.SeekCurrent {
		newSeekOffset = s.seekOffset + offset
		whenceText = "relative to current"
	} else {
		return 0, errors.New("invalid seek whence argument")
	}

	if 0 > newSeekOffset {
		return 0, errors.New("seeking before the beginning of file")
	}

	s.seekOffset = newSeekOffset

	Logger.LogF(monitor.F{"stream": s.URI}).Debugf("seeking to %v, new seek offset = %v (%v)", offset, newSeekOffset, whenceText)

	return newSeekOffset, nil
}

func (s *reflectedStream) URL() string {
	return fmt.Sprintf("%v/%v", s.reflectorURL, s.SdHash)
}

func (s *reflectedStream) resolve(client *ljsonrpc.Client) error {
	if s.URI == "" {
		return errors.New("stream uri is not set")
	}

	r, err := lbrynet.Resolve(s.URI)
	if err != nil {
		return err
	}

	// TODO: Change when underlying libs are updated for 0.38
	stream := r.Value.GetStream()
	if stream.Fee != nil && stream.Fee.Amount > 0 {
		return errors.New("paid stream")
	}

	s.SdHash = hex.EncodeToString(stream.Source.SdHash)
	s.ContentType = stream.Source.MediaType
	s.Size = int64(stream.Source.Size)

	Logger.LogF(monitor.F{
		"sd_hash":      fmt.Sprintf("%s", s.SdHash),
		"uri":          s.URI,
		"content_type": s.ContentType,
	}).Debug("resolved uri")

	return nil
}

func (s *reflectedStream) fetchData() error {
	if s.SdHash == "" {
		return errors.New("no sd hash set, call `resolve` first")
	}
	Logger.LogF(monitor.F{
		"uri": s.URI, "url": s.URL(),
	}).Debug("requesting stream data")

	resp, err := http.Get(s.URL())
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return err
	}

	sdb := &stream.SDBlob{}
	err = sdb.UnmarshalJSON(body)
	if err != nil {
		return err
	}

	if s.Size == 0 {
		for _, bi := range sdb.BlobInfos {
			if bi.Length == stream.MaxBlobSize {
				s.Size += int64(stream.MaxBlobSize - 1)
			} else {
				s.Size += int64(bi.Length)
			}
		}

		// last padding is unguessable
		s.Size -= 16
	}

	sort.Slice(sdb.BlobInfos, func(i, j int) bool {
		return sdb.BlobInfos[i].BlobNum < sdb.BlobInfos[j].BlobNum
	})
	s.SDBlob = sdb

	Logger.LogF(monitor.F{
		"blob_num": s.blobNum(),
		"size":     s.Size,
		"uri":      s.URI,
	}).Debug("got stream data")
	return nil
}

func (s *reflectedStream) prepareWriter(w http.ResponseWriter) {
	w.Header().Set("Content-Type", s.ContentType)
}

func (s *reflectedStream) downloadBlob(hash []byte) ([]byte, error) {
	var body []byte

	url := s.blobInfoURL(hash)
	start := time.Now()

	request, _ := http.NewRequest("GET", url, nil)
	client := http.Client{Timeout: time.Second * time.Duration(config.GetBlobDownloadTimeout())}
	resp, err := client.Do(request)

	if err != nil {
		return body, err
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusForbidden {
		return body, errMissingBlob
	} else if resp.StatusCode != http.StatusOK {
		return body, fmt.Errorf("server responded with an unexpected status (%v)", resp.Status)
	}
	body, err = ioutil.ReadAll(resp.Body)
	if err != nil {
		return body, err
	}

	elapsedDLoad := time.Since(start).Seconds()
	metrics.PlayerBlobDownloadDurations.Observe(elapsedDLoad)

	Logger.LogF(monitor.F{
		"stream":  s.URI,
		"url":     url,
		"elapsed": elapsedDLoad,
	}).Info("blob downloaded")

	return body, nil
}

func (s *reflectedStream) streamBlob(blobNum int, startOffsetInBlob int64, dest []byte) (int, error) {
	blobInfo := s.SDBlob.BlobInfos[blobNum]
	logBlobNum := fmt.Sprintf("%v/%v", blobInfo.BlobNum+1, s.blobNum())

	Logger.LogF(monitor.F{
		"stream": s.URI,
		"num":    logBlobNum,
	}).Debug("blob requested")

	readLen := 0
	encBody, err := s.downloadBlob(blobInfo.BlobHash)
	if errors.Is(err, errMissingBlob) {
		return 0, nil
	}

	start := time.Now()

	decryptedBody, err := stream.DecryptBlob(stream.Blob(encBody), s.SDBlob.Key, blobInfo.IV)
	if err != nil {
		return 0, err
	}
	endOffsetInBlob := int64(len(dest)) + startOffsetInBlob
	if endOffsetInBlob > int64(len(decryptedBody)) {
		endOffsetInBlob = int64(len(decryptedBody))
	}
	elapsedDecode := time.Since(start).Seconds()
	metrics.PlayerBlobDecodeDurations.Observe(elapsedDecode)

	readLen += copy(dest, decryptedBody[startOffsetInBlob:endOffsetInBlob])

	Logger.LogF(monitor.F{
		"stream":       s.URI,
		"num":          logBlobNum,
		"written":      readLen,
		"elapsed":      elapsedDecode,
		"start_offset": startOffsetInBlob,
		"end_offset":   endOffsetInBlob,
	}).Debug("done streaming a blob")

	return readLen, nil
}

func (s *reflectedStream) blobInfoURL(hash []byte) string {
	return fmt.Sprintf("%v/%v", s.reflectorURL, hex.EncodeToString(hash))
}

/////////////////////////////////////////////////////////////////////

type CacheEntry struct {
	Body []byte
	Hits int32
}

var cache = map[string]*CacheEntry{}

type Blob interface {
	Stream(int64, int64, []byte) (int, error)
}

type reflectedBlob struct {
	stream        *reflectedStream
	hash          string
	iv            []byte
	key           []byte
	decryptedBlob *decryptedBlob
}

type decryptedBlob struct {
	stream     *reflectedStream
	hash       string
	cacheEntry *CacheEntry
}

func GetBlob(s *reflectedStream, n int, hash string) Blob {
	if e, ok := cache[hash]; ok {
		e.Hits++
		return &decryptedBlob{
			stream:     s,
			hash:       hash,
			cacheEntry: e,
		}
	}
	return &reflectedBlob{
		stream: s,
		hash:   hash,
		key:    s.SDBlob.Key,
		iv:     s.SDBlob.BlobInfos[n].IV,
	}
}

func (b *reflectedBlob) url() string {
	return fmt.Sprintf("%v/%v", b.stream.reflectorURL, b.hash)
}

func (b *reflectedBlob) download() ([]byte, error) {
	var body []byte

	start := time.Now()

	request, _ := http.NewRequest("GET", b.url(), nil)
	client := http.Client{Timeout: time.Second * time.Duration(config.GetBlobDownloadTimeout())}
	resp, err := client.Do(request)

	if err != nil {
		return body, err
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusForbidden {
		return body, errMissingBlob
	} else if resp.StatusCode != http.StatusOK {
		return body, fmt.Errorf("server responded with an unexpected status (%v)", resp.Status)
	}
	body, err = ioutil.ReadAll(resp.Body)
	if err != nil {
		return body, err
	}

	elapsedDLoad := time.Since(start).Seconds()
	metrics.PlayerBlobDownloadDurations.Observe(elapsedDLoad)

	Logger.LogF(monitor.F{
		"stream":  b.stream.URI,
		"url":     b.url(),
		"elapsed": elapsedDLoad,
	}).Info("blob downloaded")

	return body, nil
}

func (b *reflectedBlob) decrypt(encBody []byte) ([]byte, error) {
	body, err := stream.DecryptBlob(stream.Blob(encBody), b.key, b.iv)
	if err != nil {
		return body, err
	}
	return body, nil
}

func (b *reflectedBlob) saveToCache(decBody []byte) {
	cache[b.hash] = &CacheEntry{Body: decBody}
}

func (b *reflectedBlob) Stream(start, end int64, dest []byte) (int, error) {
	body, err := b.download()
	if err != nil {
		return 0, err
	}
	decBody, err := b.decrypt(body)
	if err != nil {
		return 0, err
	}
	return copy(dest, decBody[start:end]), nil
}

func (b *decryptedBlob) Stream(start, end int64, dest []byte) (int, error) {
	return copy(dest, b.cacheEntry.Body[start:end]), nil
}

func (s *reflectedStream) prepareBlobs(dest []byte) {
	blobs := make([]*Blob, len(s.SDBlob.BlobInfos))

	for n, blobInfo := range s.SDBlob.BlobInfos {
		b := GetBlob(s, n, hex.EncodeToString(blobInfo.BlobHash))
		blobs[n] = &b
	}

	s.blobs = blobs
}

func (s *reflectedStream) Stream(dest []byte) (*[]Blob, error) {
	// blobs := make([]*Blob, len(s.SDBlob.BlobInfos))

	// var firstBlobOffset int64

	// readLen := int64(len(dest))
	// end := s.seekOffset + readLen
	// firstBlobNum := int(s.seekOffset / (stream.MaxBlobSize - 2))

	// if firstBlobNum == 0 {
	// 	firstBlobOffset = s.seekOffset - int64(firstBlobNum*stream.MaxBlobSize)
	// } else {
	// 	firstBlobOffset = s.seekOffset - int64(firstBlobNum*stream.MaxBlobSize) + int64(firstBlobNum)
	// }

	// for n, blobInfo := range s.SDBlob.BlobInfos {
	// 	b := GetBlob(s, n, hex.EncodeToString(blobInfo.BlobHash))
	// 	blobs[n] = &b
	// }
	// for n, blob := range blobs[firstBlobNum:] {

	// }
	return nil, nil
}
