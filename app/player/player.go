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

const reflectorURL = "http://blobs.lbry.io/"

var sdkRouter = router.New(config.GetLbrynetServers())

type reflectedStream struct {
	URI         string
	StartByte   int64
	EndByte     int64
	SdHash      string
	Size        int64
	ContentType string
	SDBlob      *stream.SDBlob
	seekOffset  int64
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

	s, err := newReflectedStream(uri)
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

func newReflectedStream(uri string) (rs *reflectedStream, err error) {
	client := ljsonrpc.NewClient(sdkRouter.GetBalancedSDKAddress())
	rs = &reflectedStream{URI: uri}
	err = rs.resolve(client)
	return rs, err
}

func (s reflectedStream) blobNum() int {
	return len(s.SDBlob.BlobInfos) - 1
}

// Read implements io.ReadSeeker interface
func (s *reflectedStream) Read(p []byte) (n int, err error) {
	var startOffsetInBlob int64

	bufferLen := len(p)
	seekOffsetEnd := s.seekOffset + int64(bufferLen)
	blobNum := int(s.seekOffset / (stream.MaxBlobSize - 2))

	if blobNum == 0 {
		startOffsetInBlob = s.seekOffset - int64(blobNum*stream.MaxBlobSize)
	} else {
		startOffsetInBlob = s.seekOffset - int64(blobNum*stream.MaxBlobSize) + int64(blobNum)
	}

	start := time.Now()
	n, err = s.streamBlob(blobNum, startOffsetInBlob, p)

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
	return reflectorURL + s.SdHash
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

func (s *reflectedStream) getBlob(url string) (*http.Response, error) {
	request, _ := http.NewRequest("GET", url, nil)
	client := http.Client{Timeout: time.Second * time.Duration(config.GetBlobDownloadTimeout())}
	r, err := client.Do(request)
	return r, err
}

func (s *reflectedStream) streamBlob(blobNum int, startOffsetInBlob int64, dest []byte) (int, error) {
	bi := s.SDBlob.BlobInfos[blobNum]
	logBlobNum := fmt.Sprintf("%v/%v", bi.BlobNum+1, s.blobNum())

	readLen := 0
	url := blobInfoURL(bi)

	Logger.LogF(monitor.F{
		"stream": s.URI,
		"url":    url,
		"num":    logBlobNum,
	}).Debug("requesting a blob")
	start := time.Now()

	resp, err := s.getBlob(url)
	if err != nil {
		return 0, err
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusOK {
		encryptedBody, err := ioutil.ReadAll(resp.Body)
		if err != nil {
			return 0, err
		}

		message := "done downloading a blob"
		elapsedDLoad := time.Since(start).Seconds()
		metrics.PlayerBlobDownloadDurations.Observe(elapsedDLoad)
		if blobNum == 0 {
			message += ", starting stream playback"
		}
		Logger.LogF(monitor.F{
			"stream":  s.URI,
			"num":     logBlobNum,
			"elapsed": elapsedDLoad,
		}).Info(message)

		start = time.Now()
		decryptedBody, err := stream.DecryptBlob(stream.Blob(encryptedBody), s.SDBlob.Key, bi.IV)
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
	} else {
		return readLen, fmt.Errorf("server responded with an unexpected status (%v)", resp.Status)
	}
	return readLen, nil
}

func blobInfoURL(bi stream.BlobInfo) string {
	return reflectorURL + hex.EncodeToString(bi.BlobHash)
}
