package olapdb

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"github.com/lbryio/lbrytv/apps/watchman/gen/reporter"
	"github.com/lbryio/lbrytv/apps/watchman/log"
	"github.com/pkg/errors"

	_ "github.com/ClickHouse/clickhouse-go"
)

var (
	conn     *sql.DB
	database string
)

func Connect(url string, dbName string) error {
	var err error
	if dbName == "" {
		dbName = database
	}
	database = dbName
	conn, err = sql.Open("clickhouse", url)
	if err != nil {
		return err
	}

	go ping()

	MigrateUp(dbName)
	log.Log.Named("clickhouse").Infof("connected to clickhouse server %v (database=%v)", url, dbName)
	return nil
}

func Write(stmt *sql.Stmt, r *reporter.PlaybackReport, addr string, ts string) error {
	var (
		t     time.Time
		err   error
		rate  uint32
		cache string
	)
	if ts != "" {
		t, err = time.Parse(time.RFC1123Z, ts)
		if err != nil {
			return err
		}
	} else {
		t = time.Now()
	}
	area, subarea := getArea(addr)

	if r.Bandwidth != nil {
		rate = uint32(*r.Bandwidth)
	}
	if r.Cache != nil {
		cache = (*r.Cache)
	} else {
		cache = "miss"
	}

	_, err = stmt.Exec(
		r.URL,
		uint32(r.Duration),
		t,
		uint32(r.Position),
		uint8(r.RelPosition),
		uint8(r.RebufCount),
		uint32(r.RebufDuration),
		r.Protocol,
		cache,
		r.Player,
		r.UserID,
		rate,
		r.Device,
		area,
		subarea,
		addr,
	)
	if err != nil {
		return err
	}
	log.Log.Named("clickhouse").Infow(
		"playback record written",
		"user_id", r.UserID, "url", r.URL, "rebuf_count", r.RebufCount, "area", area, "ip", addr, "ts", t)
	return nil
}

func WriteOne(r *reporter.PlaybackReport, addr string, ts string) error {
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	tx, err := conn.BeginTx(ctx, nil)
	if err != nil {
		return errors.Wrap(err, "cannot begin")
	}
	stmt, err := prepareWrite(tx)
	if err != nil {
		return errors.Wrap(err, "cannot prepare")
	}
	defer stmt.Close()
	Write(stmt, r, addr, ts)
	if err := tx.Commit(); err != nil {
		return errors.Wrap(err, "cannot commit")
	}
	return nil
}

func prepareWrite(tx *sql.Tx) (*sql.Stmt, error) {
	return tx.Prepare(fmt.Sprintf(`
	INSERT INTO %v.playback
		(URL, Duration, Timestamp, Position, RelPosition, RebufCount,
			RebufDuration, Protocol, Cache, Player, UserID, Bandwidth, Device, Area, SubArea, IP)
	VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	`, database))
}

func ping() {
	ticker := time.NewTicker(60 * time.Second)
	for range ticker.C {
		if err := conn.Ping(); err != nil {
			log.Log.Named("clickhouse").Errorf("error pinging clickhouse: %v", err)
		}
	}
}
