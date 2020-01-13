package player

import (
	"encoding/hex"
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
	"github.com/lbryio/lbrytv/config"
	"github.com/lbryio/lbrytv/internal/metrics"
	"github.com/lbryio/lbrytv/internal/monitor"

	ljsonrpc "github.com/lbryio/lbry.go/v2/extras/jsonrpc"
	"github.com/lbryio/lbry.go/v2/stream"
	"github.com/lbryio/reflector.go/peer"
	"github.com/lbryio/reflector.go/store"
)

const (
	// ChunkSize is a size of decrypted blob.
	ChunkSize = stream.MaxBlobSize - 1

	// DefaultPrefetchLen is how many blobs we should prefetch ahead.
	// 3 should be enough to deliver 2 x 4 = 8MB/s streams.
	DefaultPrefetchLen = 3

	// RetrieverSourceL2Cache is for labeling cache speed sourced from level 2 chunk cache (LFU)
	RetrieverSourceL2Cache = "l2_cache"

	// RetrieverSourceReflector is for labeling cache speed sourced from reflector
	RetrieverSourceReflector = "reflector"
)

// Player is an entry-point object to the new player package.
type Player struct {
	lbrynetClient  *ljsonrpc.Client
	chunkGetter    chunkGetter
	localCache     ChunkCache
	enablePrefetch bool
}

// Opts are options to be set for Player instance.
type Opts struct {
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
	chunkGetter chunkGetter
}

// chunkGetter is an object for retrieving blobs from BlobStore or optionally from local cache.
type chunkGetter struct {
	blobStore      store.BlobStore
	localCache     ChunkCache
	sdBlob         *stream.SDBlob
	seenChunks     []ReadableChunk
	enablePrefetch bool
}

// ReadableChunk interface describes generic chunk object that Stream can Read() from.
type ReadableChunk interface {
	Read(offset, n int, dest []byte) (int, error)
	Size() int
}

type reflectedChunk struct {
	body []byte
}

// GetBlobStore returns default pre-configured blob store.
func GetBlobStore() *peer.Store {
	return peer.NewStore(peer.StoreOpts{
		Address: config.GetRefractorAddress(),
		Timeout: time.Second * time.Duration(config.GetRefractorTimeout()),
	})
}

// NewPlayer initializes an instance with optional BlobStore.
func NewPlayer(opts *Opts) *Player {
	if opts == nil {
		opts = &Opts{}
	}
	p := &Player{
		lbrynetClient: opts.Lbrynet,
	}
	if opts.EnableLocalCache {
		cPath := path.Join(os.TempDir(), "blob_cache")
		cache, err := InitFSCache(&FSCacheOpts{Path: cPath})
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
func (p *Player) Play(s *Stream, w http.ResponseWriter, r *http.Request) error {
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
	s.sdBlob = &sdBlob
	s.chunkGetter = chunkGetter{
		sdBlob:         &sdBlob,
		localCache:     p.localCache,
		enablePrefetch: p.enablePrefetch,
		seenChunks:     make([]ReadableChunk, len(sdBlob.BlobInfos)-1),
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
				size += ChunkSize
			} else {
				size += uint64(blob.Length - 1)
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
// Actual chunk retrieval and delivery happens in s.readFromChunks().
func (s *Stream) Read(dest []byte) (n int, err error) {
	calc := NewChunkCalculator(s.Size, s.seekOffset, len(dest))

	Logger.Log().Tracef("reading %v-%v bytes from stream (size=%v, dest len=%v)", s.seekOffset, s.seekOffset+int64(calc.ReadLen), s.Size, len(dest))
	n, err = s.readFromChunks(calc, dest)
	Logger.Log().Tracef("done reading %v-%v bytes from stream", s.seekOffset, s.seekOffset+int64(n))
	s.seekOffset += int64(n)

	metrics.PlayerOutBytes.Add(float64(n))

	if err != nil {
		Logger.streamReadFailed(s, calc, err)
	}

	return n, err
}

func (s *Stream) readFromChunks(calc ChunkCalculator, dest []byte) (int, error) {
	var b ReadableChunk
	var err error
	var read int

	log := Logger.WithField("stream", s.URI)

	for i := calc.FirstChunkIdx; i < calc.LastChunkIdx+1; i++ {
		var start, readLen int

		if i == calc.FirstChunkIdx {
			start = calc.FirstChunkOffset
			readLen = ChunkSize - calc.FirstChunkOffset
		} else if i == calc.LastChunkIdx {
			start = calc.LastChunkOffset
			readLen = calc.LastChunkReadLen
		} else if calc.FirstChunkIdx == calc.LastChunkIdx {
			start = calc.FirstChunkOffset
			readLen = calc.LastChunkReadLen
		}
		log.Debugf("requesting %v-%v bytes from chunk #%v", start, start+readLen, i)

		b, err = s.chunkGetter.Get(i)
		if err != nil {
			return read, err
		}

		n, err := b.Read(start, readLen, dest[read:])
		read += n
		if err != nil {
			return read, err
		}
		log.Debugf("read %v-%v bytes from chunk #%v (%v read, %v total)", start, start+readLen, i, n, read)
	}

	return read, nil
}

// Get returns a Blob object that can be Read() from.
// It first tries to get it from the local cache, and if it is not found, fetches it from the reflector.
func (b *chunkGetter) Get(n int) (ReadableChunk, error) {
	var (
		cChunk   ReadableChunk
		rChunk   *reflectedChunk
		cacheHit bool
		err      error
	)

	if n > len(b.sdBlob.BlobInfos) {
		return nil, errors.New("blob index out of bounds")
	}

	if b.seenChunks[n] != nil {
		return b.seenChunks[n], nil
	}

	bi := b.sdBlob.BlobInfos[n]
	hash := hex.EncodeToString(bi.BlobHash)

	timerCache := metrics.TimerStart()
	if b.localCache != nil {
		cChunk, cacheHit = b.getChunkFromCache(hash)
	} else {
		cacheHit = false
	}
	timerCache.Done()

	if cacheHit {
		rate := float64(cChunk.Size()) / (1024 * 1024) / timerCache.Duration * 8
		metrics.PlayerRetrieverSpeed.With(map[string]string{metrics.LabelSource: RetrieverSourceL2Cache}).Set(rate)
		metrics.PlayerCacheHitCount.Inc()

		RetLogger.WithFields(monitor.F{
			"hash":      hash,
			"duration":  timerCache.String(),
			"source":    true,
			"rate_mbps": rate,
		}).Info("chunk retrieved")

		b.saveToHotCache(n, cChunk)
		return cChunk, nil
	}

	metrics.PlayerCacheMissCount.Inc()
	timerReflector := metrics.TimerStart()
	rChunk, err = b.getChunkFromReflector(hash, b.sdBlob.Key, bi.IV)
	if err != nil {
		return nil, err
	}
	timerReflector.Done()

	rate := float64(rChunk.Size()) / (1024 * 1024) / timerReflector.Duration * 8
	metrics.PlayerRetrieverSpeed.With(map[string]string{metrics.LabelSource: RetrieverSourceReflector}).Set(rate)

	RetLogger.WithFields(monitor.F{
		"hash":       hash,
		"duration":   timerReflector.String(),
		"from_cache": false,
		"rate_mbps":  rate,
	}).Info("chunk retrieved")

	b.saveToHotCache(n, rChunk)
	go b.saveToCache(hash, rChunk)
	go b.prefetchToCache(n + 1)

	return rChunk, nil
}

func (b *chunkGetter) saveToHotCache(n int, chunk ReadableChunk) {
	// Save chunk in the hot cache so next Get() / Read() goes to it
	b.seenChunks[n] = chunk
	// Remove already read chunks to preserve memory
	if n > 0 {
		b.seenChunks[n-1] = nil
	}
}

func (b *chunkGetter) saveToCache(hash string, chunk *reflectedChunk) (ReadableChunk, error) {
	if b.localCache == nil {
		return nil, nil
	}

	body := make([]byte, len(chunk.body))
	if _, err := chunk.Read(0, ChunkSize, body); err != nil {
		RetLogger.Log().Errorf("couldn't read from chunk %v: %v", hash, err)
		return nil, err
	}
	return b.localCache.Set(hash, body)
}

func (b *chunkGetter) prefetchToCache(startN int) {
	if b.localCache == nil || !b.enablePrefetch {
		return
	}

	prefetchLen := DefaultPrefetchLen
	chunksLeft := len(b.sdBlob.BlobInfos) - startN - 1 // Last blob is empty
	if chunksLeft <= 0 {
		return
	} else if chunksLeft < DefaultPrefetchLen {
		prefetchLen = chunksLeft
	}

	RetLogger.Log().Debugf("prefetching %v chunks to local cache", prefetchLen)
	for _, bi := range b.sdBlob.BlobInfos[startN : startN+prefetchLen] {
		hash := hex.EncodeToString(bi.BlobHash)
		if b.localCache.Has(hash) {
			RetLogger.Log().Debugf("chunk %v found in cache, not prefetching", hash)
			continue
		}
		RetLogger.Log().Debugf("prefetching chunk %v", hash)
		reflected, err := b.getChunkFromReflector(hash, b.sdBlob.Key, bi.IV)
		if err != nil {
			RetLogger.Log().Warnf("failed to prefetch chunk %v: %v", hash, err)
			return
		}
		Logger.blobRetrieved(b.sdBlob.StreamName, bi.BlobNum)
		b.saveToCache(hash, reflected)
	}

}

func (b *chunkGetter) getChunkFromCache(hash string) (ReadableChunk, bool) {
	c, ok := b.localCache.Get(hash)
	return c, ok
}

func (b *chunkGetter) getChunkFromReflector(hash string, key, iv []byte) (*reflectedChunk, error) {
	bStore := GetBlobStore()
	blob, err := bStore.Get(hash)
	if err != nil {
		Logger.blobDownloadFailed(blob, err)
		return nil, err
	}

	metrics.PlayerInBytes.Add(float64(len(blob)))

	body, err := stream.DecryptBlob(blob, key, iv)
	if err != nil {
		return nil, err
	}

	chunk := &reflectedChunk{body}
	return chunk, nil
}

// Read is called by stream.Read.
func (b *reflectedChunk) Read(offset, n int, dest []byte) (int, error) {
	if offset+n > len(b.body) {
		n = len(b.body) - offset
	}

	timer := metrics.TimerStart().Observe(metrics.PlayerBlobDeliveryDurations)
	read := copy(dest, b.body[offset:offset+n])
	timer.Done()

	return read, nil
}

func (b *reflectedChunk) Size() int {
	return len(b.body)
}

// ChunkCalculator provides handy blob calculations for a requested stream range.
type ChunkCalculator struct {
	Offset           int64
	ReadLen          int
	FirstChunkIdx    int
	LastChunkIdx     int
	FirstChunkOffset int
	LastChunkReadLen int
	LastChunkOffset  int
}

// NewChunkCalculator initializes ChunkCalculator with provided stream size, start offset and reader buffer length.
func NewChunkCalculator(size, offset int64, readLen int) ChunkCalculator {
	bc := ChunkCalculator{Offset: offset, ReadLen: readLen}

	bc.FirstChunkIdx = int(offset / int64(ChunkSize))
	bc.LastChunkIdx = int((offset + int64(readLen)) / int64(ChunkSize))
	bc.FirstChunkOffset = int(offset - int64(bc.FirstChunkIdx*ChunkSize))
	if bc.FirstChunkIdx == bc.LastChunkIdx {
		bc.LastChunkOffset = int(offset - int64(bc.LastChunkIdx*ChunkSize))
	}
	bc.LastChunkReadLen = int((offset + int64(readLen)) - int64(bc.LastChunkOffset) - int64(ChunkSize)*int64(bc.LastChunkIdx))

	return bc
}

func (c ChunkCalculator) String() string {
	return fmt.Sprintf("B%v[%v:]-B%v[%v:%v]", c.FirstChunkIdx, c.FirstChunkOffset, c.LastChunkIdx, c.LastChunkOffset, c.LastChunkReadLen)
}
