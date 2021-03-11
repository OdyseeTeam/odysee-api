package watchman

import (
	"context"
	"log"

	playback "github.com/lbryio/lbrytv/apps/watchman/gen/playback"
)

// playback service example implementation.
// The example methods log the requests and return zero values.
type playbacksrvc struct {
	logger *log.Logger
}

// NewPlayback returns the playback service implementation.
func NewPlayback(logger *log.Logger) playback.Service {
	return &playbacksrvc{logger}
}

// Add implements add.
func (s *playbacksrvc) Add(ctx context.Context, p *playback.Playback) (err error) {
	s.logger.Print("playback.add")
	return
}
