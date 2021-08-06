package watchman

import (
	"context"
	"database/sql"

	reporter "github.com/lbryio/lbrytv/apps/watchman/gen/reporter"
	"github.com/lbryio/lbrytv/apps/watchman/olapdb"

	"go.uber.org/zap"
)

// reporter service example implementation.
// The example methods log the requests and return zero values.
type reportersrvc struct {
	db     *sql.DB
	logger *zap.SugaredLogger
}

// NewReporter returns the reporter service implementation.
func NewReporter(db *sql.DB, logger *zap.SugaredLogger) reporter.Service {
	svc := &reportersrvc{
		db:     db,
		logger: logger,
	}
	return svc
}

// Add implements add.
func (s *reportersrvc) Add(ctx context.Context, p *reporter.PlaybackReport) error {
	s.logger.Debug("reporter.add")

	if p.RebufDuration > p.Duration {
		return &reporter.MultiFieldError{Message: "rebufferung duration cannot be larger than duration"}
	}
	addr := ctx.Value(RemoteAddressKey).(string)
	err := olapdb.WriteOne(p, addr, "")
	if err != nil {
		return err
	}
	return nil
}

func (s *reportersrvc) Healthz(ctx context.Context) (string, error) {
	return "OK", nil
}
