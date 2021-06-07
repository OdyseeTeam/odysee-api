package watchman

import (
	"context"
	"database/sql"
	"log"

	reporter "github.com/lbryio/lbrytv/apps/watchman/gen/reporter"
	"github.com/lbryio/lbrytv/apps/watchman/tsdb"

	"goa.design/goa/v3/http/middleware"
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
func (s *reportersrvc) Add(ctx context.Context, p *reporter.PlaybackReport) error {
	s.logger.Print("reporter.add")
	// db.New(s.db).CreatePlaybackReport(context.Background(), db.CreatePlaybackReportParams{
	// 	URL: p.URL,
	// 	Pos: p.Pos,
	// 	Por: p.Por,
	// 	Dur: p.Dur,
	// 	Bfc: p.Bfc,
	// 	Bfd: p.Bfd,
	// 	Fmt: p.Fmt,
	// 	Pid: p.Pid,
	// 	Cid: p.Cid,
	// 	Cdv: p.Cdv,
	// 	Crt: *p.Crt,
	// 	Car: *p.Car,
	// })
	addr := ctx.Value(middleware.RequestRemoteAddrKey).(string)
	tsdb.Write(p, addr)
	return nil
}

func (s *reportersrvc) Healthz(ctx context.Context) (string, error) {
	return "OK", nil
}
