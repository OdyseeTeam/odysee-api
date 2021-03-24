package watchman

import (
	"context"
	"database/sql"
	"log"

	"github.com/lbryio/lbrytv/apps/watchman/db"
	reporter "github.com/lbryio/lbrytv/apps/watchman/gen/reporter"
)

// reporter service example implementation.
// The example methods log the requests and return zero values.
type reportersrvc struct {
	db     *sql.DB
	logger *log.Logger
}

// NewReporter returns the reporter service implementation.
func NewReporter(db *sql.DB, logger *log.Logger) reporter.Service {
	return &reportersrvc{
		db:     db,
		logger: logger,
	}
}

// Add implements add.
func (s *reportersrvc) Add(ctx context.Context, p *reporter.PlaybackReport) (err error) {
	s.logger.Print("reporter.add")
	db.New(s.db).CreatePlaybackReport(context.Background(), db.CreatePlaybackReportParams{
		URL: p.URL,
		Pos: p.Pos,
		Por: p.Por,
		Dur: p.Dur,
		Bfc: p.Bfc,
		Bfd: p.Bfd,
		Fmt: p.Fmt,
		Pid: p.Pid,
		Cid: p.Cid,
		Cdv: p.Cdv,
		Crt: *p.Crt,
		Car: *p.Car,
	})

	return
}
