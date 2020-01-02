package player

import (
	"fmt"

	"github.com/lbryio/lbry.go/v2/stream"
	"github.com/lbryio/lbrytv/internal/monitor"
)

type localLogger struct {
	monitor.ModuleLogger
}

// Logger is a package-wide logger.
// Warning: will generate a lot of output if DEBUG loglevel is enabled.
// Logger variables here are made public so logging can be disabled on the spot when needed (in tests etc).
var Logger = localLogger{monitor.NewModuleLogger("player")}

// CacheLogger is for caching operations only.
var CacheLogger = localLogger{monitor.NewModuleLogger("player_cache")}

// RetLogger is for blob/chunk retrieval operations logging.
var RetLogger = localLogger{monitor.NewModuleLogger("player_retriever")}

func (l localLogger) streamPlaybackRequested(uri, remoteIP string) {
	l.WithFields(monitor.F{"remote_ip": remoteIP, "uri": uri}).Info("starting stream playback")
}

func (l localLogger) streamSeek(s *Stream, offset, newOffset int64, whence string) {
	Logger.WithFields(monitor.F{"stream": s.URI}).Debugf("seeking from %v to %v (%v), new position = %v", s.seekOffset, offset, whence, newOffset)
}

func (l localLogger) streamRead(s *Stream, n int, calc ChunkCalculator) {
	l.WithFields(monitor.F{"uri": s.URI}).Debugf("read %v bytes (%v..%v) from stream", n, calc.Offset, s.seekOffset)
}

func (l localLogger) streamReadFailed(s *Stream, calc ChunkCalculator, err error) {
	logFields := monitor.F{
		"uri":         s.URI,
		"blob_calc":   calc.String(),
		"seek_offset": fmt.Sprintf("%v", calc.Offset),
		"size":        fmt.Sprintf("%v", s.Size),
	}
	l.WithFields(logFields).Info("stream read failed: ", err)
}

func (l localLogger) streamResolved(s *Stream) {
	l.WithFields(monitor.F{"uri": s.URI}).Debug("stream resolved")
}

func (l localLogger) streamResolveFailed(uri string, err error) {
	l.WithFields(monitor.F{"uri": uri}).Error("failed to resolve stream: ", err)
}

func (l localLogger) streamRetrieved(s *Stream) {
	l.WithFields(monitor.F{"uri": s.URI}).Debug("stream retrieved")
}

func (l localLogger) streamRetrievalFailed(uri string, err error) {
	l.WithFields(monitor.F{"uri": uri}).Error("failed to retrieve stream: ", err)
}

func (l localLogger) blobRetrieved(uri string, n int) {
	l.WithFields(monitor.F{"uri": uri, "num": n}).Debug("blob retrieved")
}

func (l localLogger) blobDownloadFailed(b stream.Blob, err error) {
	l.Log().Error("blob failed to download: ", err)
}
