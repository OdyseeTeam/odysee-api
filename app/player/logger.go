package player

import (
	"fmt"

	"github.com/lbryio/lbry.go/v2/stream"
	"github.com/lbryio/lbrytv/internal/metrics"
	"github.com/lbryio/lbrytv/internal/monitor"
)

type localLogger struct {
	monitor.ModuleLogger
}

// Logger is a package-wide logger.
// Warning: will generate a lot of output if DEBUG loglevel is enabled.
var Logger = localLogger{monitor.NewModuleLogger("player")}

func (l localLogger) streamPlaybackRequested(uri, remoteIP string) {
	l.WithFields(monitor.F{"remote_ip": remoteIP, "uri": uri}).Info("starting stream playback")
}

func (l localLogger) streamPlaybackEnd(s *Stream) {
	l.WithFields(monitor.F{"uri": s.URI}).Info("stream playback complete")
}

func (l localLogger) streamSeek(s *Stream, offset, newOffset int64, whence string) {
	Logger.WithFields(monitor.F{"stream": s.URI}).Tracef("seeking from %v to %v (%v), new position = %v", s.seekOffset, offset, whence, newOffset)
}

func (l localLogger) streamRead(s *Stream, n int, calc BlobCalculator) {
	l.WithFields(monitor.F{"uri": s.URI}).Debugf("read %v bytes (%v..%v) from blob stream", n, calc.Offset, s.seekOffset)
}

func (l localLogger) streamReadFailed(s *Stream, calc BlobCalculator, err error) {
	excFields := map[string]string{
		"uri":         s.URI,
		"blob_calc":   calc.String(),
		"seek_offset": fmt.Sprintf("%v", calc.Offset),
		"size":        fmt.Sprintf("%v", s.Size),
	}
	logFields := monitor.F{}
	for k, v := range excFields {
		logFields[k] = v
	}

	monitor.CaptureException(err, excFields)
	l.WithFields(logFields).Info("stream read failed:", err)
}

func (l localLogger) streamResolved(s *Stream) {
	l.WithFields(monitor.F{"uri": s.URI}).Debug("stream resolved")
}

func (l localLogger) streamResolveFailure(uri string, err error) {
	l.WithFields(monitor.F{"uri": uri}).Error("stream failed to resolve: ", err)
}

func (l localLogger) blobDownloaded(b stream.Blob, t *metrics.Timer) {
	speed := float64(len(b)) / (1024 * 1024) / t.Duration
	l.WithFields(monitor.F{"duration": t.Duration, "speed": speed}).Debug("blob downloaded")
}

func (l localLogger) blobDownloadFailure(b stream.Blob, err error) {
	l.Log().Error("blob failed to download:", err)
}
