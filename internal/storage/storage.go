package storage

import (
	"database/sql"
	"embed"
	"time"

	"github.com/lbryio/lbrytv/internal/metrics"
	"github.com/lbryio/lbrytv/pkg/migrator"
	"github.com/volatiletech/sqlboiler/boil"
)

//go:embed migrations/*.sql
var MigrationsFS embed.FS

var DB *sql.DB
var Migrator migrator.Migrator

func SetDB(db *sql.DB) {
	boil.SetDB(db)
	DB = db
	Migrator = migrator.New(db, MigrationsFS)
}

func WatchMetrics(interval time.Duration) {
	t := time.NewTicker(interval)
	for {
		<-t.C
		stats := DB.Stats()
		metrics.LbrytvDBOpenConnections.Set(float64(stats.OpenConnections))
		metrics.LbrytvDBInUseConnections.Set(float64(stats.InUse))
		metrics.LbrytvDBIdleConnections.Set(float64(stats.Idle))
	}
}
