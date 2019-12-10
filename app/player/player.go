package player

import (
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"

	// "io/ioutil"
	"math"
	"net/http"
	"os"
	"path"
	"time"

	"github.com/lbryio/lbrytv/app/router"
	"github.com/lbryio/lbrytv/app/users"
	"github.com/lbryio/lbrytv/config"
	"github.com/lbryio/lbrytv/internal/metrics"
	"github.com/lbryio/lbrytv/internal/monitor"

	ljsonrpc "github.com/lbryio/lbry.go/v2/extras/jsonrpc"
	"github.com/lbryio/lbry.go/v2/stream"
	"github.com/lbryio/reflector.go/peer"
	"github.com/lbryio/reflector.go/store"
)

// Player is an entry-point object to the new player package.
type Player struct {
	lbrynetClient  *ljsonrpc.Client
	blobGetter     BlobGetter
	localCache     BlobCache
	enablePrefetch bool
}

type PlayerOpts struct {
	Lbrynet          *ljsonrpc.Client
	EnableLocalCache bool
	EnablePrefetch   bool
}

// Stream provides an io.ReadSeeker interface to a stream of blobs to be used by standard http library for range requests,
// as well as some stream metadata.
type Stream struct {
	URI         string
	Hash        string
	sdBlob      *stream.SDBlob
	Size        int64
	ContentType string
	Claim       *ljsonrpc.Claim
	seekOffset  int64
	blobGetter  BlobGetter
}

// BlobGetter is an object for retrieving blobs from BlobStore or optionally from local cache.
type BlobGetter struct {
	blobStore      store.BlobStore
	localCache     BlobCache
	sdBlob         *stream.SDBlob
	enablePrefetch bool
}

// ReadableBlob interface describes generic blob object that Stream can Read() from.
type ReadableBlob interface {
	Read(offset, n int, dest []byte) (int, error)
}

type reflectedBlob struct {
	body stream.Blob
	hash string
	key  []byte
	iv   []byte
}

// GetBlobStore returns default pre-configured blob store.
func GetBlobStore() *peer.Store {
	return peer.NewStore(peer.StoreOpts{
		Address: config.GetRefractorAddress(),
		Timeout: time.Second * time.Duration(config.GetRefractorTimeout()),
	})
}

// NewPlayer initializes an instance with optional BlobStore.
func NewPlayer(opts *PlayerOpts) *Player {
	if opts == nil {
		opts = &PlayerOpts{}
	}
	p := &Player{
		lbrynetClient: opts.Lbrynet,
	}
	if opts.EnableLocalCache {
		cPath := path.Join(os.TempDir(), "blob_cache")
		cache, err := InitFSCache(cPath)
		if err != nil {
			Logger.Log().Error("unable to initialize cache: ", err)
		} else {
			Logger.Log().Infof("player cache initialized at %v", cPath)
			p.localCache = cache
			p.enablePrefetch = opts.EnablePrefetch
		}
	}
	return p
}

func (p *Player) getLbrynetClient() *ljsonrpc.Client {
	if p.lbrynetClient != nil {
		return p.lbrynetClient
	}
	sdkRouter := router.New(config.GetLbrynetServers())
	return ljsonrpc.NewClient(sdkRouter.GetBalancedSDKAddress())
}

func (p *Player) getBlobStore() store.BlobStore {
	return GetBlobStore()
}

// Play delivers requested URI onto the supplied http.ResponseWriter.
func (p *Player) Play(uri string, w http.ResponseWriter, r *http.Request) error {
	Logger.streamPlaybackRequested(uri, users.GetIPAddressForRequest(r))

	s, err := p.ResolveStream(uri)
	if err != nil {
		Logger.streamResolveFailed(uri, err)
		return err
	}
	Logger.streamResolved(s)

	err = p.RetrieveStream(s)
	if err != nil {
		Logger.streamRetrievalFailed(uri, err)
		return err
	}
	Logger.streamRetrieved(s)

	w.Header().Set("Content-Type", s.ContentType)

	metrics.PlayerStreamsRunning.Inc()
	defer metrics.PlayerStreamsRunning.Dec()
	http.ServeContent(w, r, "stream", s.Timestamp(), s)

	return nil
}

// ResolveStream resolves provided URI by calling the SDK.
func (p *Player) ResolveStream(uri string) (*Stream, error) {
	s := &Stream{URI: uri}

	r, err := p.getLbrynetClient().Resolve(uri)
	if err != nil {
		return nil, err
	}

	claim := (*r)[uri]
	if claim.CanonicalURL == "" {
		return nil, errStreamNotFound
	}

	stream := claim.Value.GetStream()
	if stream.Fee != nil && stream.Fee.Amount > 0 {
		return nil, errPaidStream
	}

	s.Claim = &claim
	s.Hash = hex.EncodeToString(stream.Source.SdHash)
	s.ContentType = stream.Source.MediaType
	s.Size = int64(stream.Source.Size)

	return s, nil
}

// RetrieveStream downloads stream description from the reflector and tries to determine stream size
// using several methods, including legacy ones for streams that do not have metadata.
func (p *Player) RetrieveStream(s *Stream) error {
	sdBlob := stream.SDBlob{}
	bStore := p.getBlobStore()
	blob, err := bStore.Get(s.Hash)
	if err != nil {
		return err
	}

	err = sdBlob.FromBlob(blob)
	if err != nil {
		return err
	}

	s.setSize(&sdBlob.BlobInfos)
	s.blobGetter = BlobGetter{
		blobStore:      bStore,
		sdBlob:         &sdBlob,
		localCache:     p.localCache,
		enablePrefetch: p.enablePrefetch,
	}

	return nil
}

func (s *Stream) setSize(blobs *[]stream.BlobInfo) {
	if s.Size > 0 {
		return
	}

	size, err := s.Claim.GetStreamSizeByMagic()

	if err != nil {
		Logger.LogF(monitor.F{
			"uri":  s.URI,
			"size": s.Size,
		}).Infof("couldn't figure out size by magic (%v)", err)
		for _, blob := range *blobs {
			if blob.Length == stream.MaxBlobSize {
				size += stream.MaxBlobSize - 1
			} else {
				size += uint64(blob.Length)
			}
		}
		// last padding is unguessable
		size -= 16
	}

	s.Size = int64(size)
}

// Timestamp returns stream creation timestamp, used in HTTP response header.
func (s *Stream) Timestamp() time.Time {
	return time.Unix(int64(s.Claim.Timestamp), 0)
}

// Seek implements io.ReadSeeker interface and is meant to be called by http.ServeContent.
func (s *Stream) Seek(offset int64, whence int) (int64, error) {
	var (
		newOffset  int64
		whenceText string
	)

	if s.Size == 0 {
		return 0, errStreamSizeZero
	} else if int64(math.Abs(float64(offset))) > s.Size {
		return 0, errOutOfBounds
	}

	if whence == io.SeekEnd {
		newOffset = s.Size - offset
		whenceText = "relative to end"
	} else if whence == io.SeekStart {
		newOffset = offset
		whenceText = "relative to start"
	} else if whence == io.SeekCurrent {
		newOffset = s.seekOffset + offset
		whenceText = "relative to current"
	} else {
		return 0, errors.New("invalid seek whence argument")
	}

	if 0 > newOffset {
		return 0, errSeekingBeforeStart
	}

	Logger.streamSeek(s, offset, newOffset, whenceText)
	s.seekOffset = newOffset

	return newOffset, nil
}

// Read implements io.ReadSeeker interface and is meant to be called by http.ServeContent.
// Actual blob retrieval and delivery happens in s.readFromBlobs().
func (s *Stream) Read(dest []byte) (n int, err error) {
	calc := NewBlobCalculator(s.Size, s.seekOffset, len(dest))
	n, err = s.readFromBlobs(calc, dest)
	s.seekOffset += int64(n)

	if err != nil {
		Logger.streamReadFailed(s, calc, err)
	}

	return n, err
}

func (s *Stream) readFromBlobs(calc BlobCalculator, dest []byte) (int, error) {
	var b ReadableBlob
	var err error
	var read int

	log := Logger.WithField("stream", s.URI)

	for i := calc.FirstBlobNum; i < calc.LastBlobNum+1; i++ {
		var start, readLen int
		readLen = -1

		if i == calc.FirstBlobNum {
			start = calc.FirstBlobOffset
		} else if i == calc.LastBlobNum {
			start = 0
			readLen = calc.LastBlobReadLen
		}
		log.Tracef("requesting %v bytes starting from %v from blob #%v", readLen, start, i)

		b, err = s.blobGetter.Get(i)
		if err != nil {
			return read, err
		}

		n, err := b.Read(start, readLen, dest)
		read += n
		if err != nil {
			return read, err
		}
		log.Tracef("read %v bytes from blob #%v (%v read total)", n, i, read)
	}

	return read, nil
}

// Get returns a Blob object that can be Read() from.
// It first tries to get it from the local cache, and if it is not found, fetches it from the reflector.
func (b *BlobGetter) Get(n int) (ReadableBlob, error) {
	var (
		cached    ReadableBlob
		reflected *reflectedBlob
		cacheHit  bool
		err       error
	)

	if n > len(b.sdBlob.BlobInfos) {
		return nil, errors.New("blob index out of bounds")
	}
	bi := b.sdBlob.BlobInfos[n]
	hash := hex.EncodeToString(bi.BlobHash)

	if b.localCache != nil {
		cached, cacheHit = b.localCache.Get(hash)
		if !cacheHit {
			b.prefetchToLocalCache(n + 1)
		}
	} else {
		cacheHit = false
	}

	if !cacheHit {
		reflected, err = b.getReflectedBlobByHash(hash, b.sdBlob.Key, bi.IV)
		if err != nil {
			return nil, err
		}
		Logger.blobRetrieved(b.sdBlob.StreamName, bi.BlobNum)
		blob, err := b.saveToLocalCache(hash, reflected)
		if err != nil {
			CacheLogger.Log().Warnf("failed to stream off cache: %v", err)
			return reflected, nil
		}
		return blob, nil
	}

	return cached, nil
}

func (b *BlobGetter) saveToLocalCache(hash string, blob *reflectedBlob) (ReadableBlob, error) {
	if b.localCache == nil {
		return nil, nil
	}

	body := make([]byte, len(blob.body))
	if _, err := blob.Read(0, -1, body); err != nil {
		Logger.Log().Errorf("couldn't read from blob %v: %v", hash, err)
		return nil, err
	}
	return b.localCache.Set(hash, body)
}

func prettyPrint(i interface{}) {
	s, _ := json.MarshalIndent(i, "", "\t")
	fmt.Println(string(s))
}

func (b *BlobGetter) prefetchToLocalCache(startN int) {
	if b.localCache == nil || !b.enablePrefetch {
		return
	}

	var prefetchLen int
	blobsLeft := len(b.sdBlob.BlobInfos) - startN - 1 // Last blob is empty
	if blobsLeft < 0 {
		return
	} else if blobsLeft > 3 {
		prefetchLen = 3
	} else {
		prefetchLen = blobsLeft
	}

	CacheLogger.Log().Debugf("prefetching %v blobs to local cache", prefetchLen)
	// prettyPrint(b.sdBlob.BlobInfos)
	for _, bi := range b.sdBlob.BlobInfos[startN : startN+prefetchLen] {
		// prettyPrint(bi)
		hash := hex.EncodeToString(bi.BlobHash)
		if b.localCache.Has(hash) {
			CacheLogger.Log().Debugf("blob %v found in cache, not prefetching", hash)
			continue
		}
		CacheLogger.Log().Debugf("prefetching blob %v", hash)
		reflected, err := b.getReflectedBlobByHash(hash, b.sdBlob.Key, bi.IV)
		if err != nil {
			CacheLogger.Log().Warnf("failed to prefetch blob %v: %v", hash, err)
			return
		}
		Logger.blobRetrieved(b.sdBlob.StreamName, bi.BlobNum)
		go b.saveToLocalCache(hash, reflected)
	}

}

func (b *BlobGetter) getReflectedBlobByHash(hash string, key, iv []byte) (*reflectedBlob, error) {
	timer := metrics.TimerStart(metrics.PlayerBlobDownloadDurations)
	streamBlob, err := b.blobStore.Get(hash)
	if err != nil {
		Logger.blobDownloadFailed(streamBlob, err)
		return nil, err
	}
	timer.Done()
	Logger.blobDownloaded(streamBlob, timer)

	blob := &reflectedBlob{body: streamBlob, key: key, iv: iv, hash: hash}
	return blob, nil
}

// Read decrypts the blob and writes into the supplied buffer.
func (b *reflectedBlob) Read(offset, n int, dest []byte) (int, error) {
	decBody, err := stream.DecryptBlob(b.body, b.key, b.iv)
	// ioutil.WriteFile(fmt.Sprintf("dump%v", b.hash), decBody, 0600)
	if err != nil {
		return 0, err
	}
	if n == -1 {
		n = len(decBody) + 1
	}

	timer := metrics.TimerStart(metrics.PlayerBlobDeliveryDurations)
	read := copy(dest, decBody[offset:n])
	timer.Done()

	return read, nil
}

// BlobCalculator provides handy blob calculations for a requested stream range.
type BlobCalculator struct {
	Offset          int64
	ReadLen         int
	FirstBlobNum    int
	LastBlobNum     int
	FirstBlobOffset int
	LastBlobReadLen int
}

// NewBlobCalculator initializes BlobCalculator with provided stream size, start offset and reader buffer length.
func NewBlobCalculator(size, offset int64, readLen int) BlobCalculator {
	bc := BlobCalculator{Offset: offset, ReadLen: readLen}
	blobSize := stream.MaxBlobSize

	bc.FirstBlobNum = int(offset / int64(blobSize-2)) // TODO: why -2?
	bc.LastBlobNum = bc.FirstBlobNum + readLen/blobSize
	bc.FirstBlobOffset = int(offset + int64(bc.FirstBlobNum) - int64(bc.FirstBlobNum*blobSize))
	bc.LastBlobReadLen = readLen - (bc.LastBlobNum-bc.FirstBlobNum)*blobSize

	return bc
}

func (c BlobCalculator) String() string {
	return fmt.Sprintf("B%v[%v:]-B%v[:%v]", c.FirstBlobNum, c.FirstBlobOffset, c.LastBlobNum, c.LastBlobReadLen)
}
