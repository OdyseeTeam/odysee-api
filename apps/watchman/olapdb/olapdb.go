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
	conn        *sql.DB
	database    string
	batchWriter *BatchWriter
	repBatch    []*reporter.PlaybackReport
	repChan     chan *reporter.PlaybackReport
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

	repChan = make(chan *reporter.PlaybackReport, 20000)

	MigrateUp(dbName)

	batchWriter = NewBatchWriter(2*time.Second, 16)
	go batchWriter.Start()

	log.Log.Named("clickhouse").Infof("connected to clickhouse server %v (database=%v)", url, dbName)
	return nil
}

func prepareArgs(r *reporter.PlaybackReport, addr string, ts string) ([]interface{}, error) {
	var (
		t                  time.Time
		err                error
		bandwidth, bitrate uint32
		cache              string
	)
	if ts != "" {
		t, err = time.Parse(time.RFC1123Z, ts)
		if err != nil {
			return nil, err
		}
	} else {
		t = time.Now()
	}
	area, subarea := getArea(addr)

	if r.Bandwidth != nil {
		bandwidth = uint32(*r.Bandwidth)
	}
	if r.Bitrate != nil {
		bitrate = uint32(*r.Bitrate)
	}
	if r.Cache != nil {
		cache = (*r.Cache)
	} else {
		cache = "miss"
	}

	return []interface{}{
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
		bandwidth,
		bitrate,
		r.Device,
		area,
		subarea,
		addr,
	}, nil
}

func Write(stmt *sql.Stmt, r *reporter.PlaybackReport, addr string, ts string) error {
	args, err := prepareArgs(r, addr, ts)
	if err != nil {
		return err
	}
	_, err = stmt.Exec(args...)
	if err != nil {
		return err
	}
	log.Log.Named("clickhouse").Debugw(
		"playback record written",
		"user_id", r.UserID, "url", r.URL, "rebuf_count", r.RebufCount, "ip", addr, "ts", args[2])
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

func BatchWrite(r *reporter.PlaybackReport, addr string, ts string) error {
	return batchWriter.Write(r, addr, ts)
}

func prepareWrite(tx *sql.Tx) (*sql.Stmt, error) {
	return tx.Prepare(prepareInsertQuery("(?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)"))
}

func ping() {
	ticker := time.NewTicker(60 * time.Second)
	for range ticker.C {
		if err := conn.Ping(); err != nil {
			log.Log.Named("clickhouse").Errorf("error pinging clickhouse: %v", err)
		}
	}
}

func prepareInsertQuery(values string) string {
	return fmt.Sprintf(`
		INSERT INTO %v.playback
			(URL, Duration, Timestamp, Position, RelPosition, RebufCount,
				RebufDuration, Protocol, Cache, Player, UserID, Bandwidth, Bitrate, Device, Area, SubArea, IP)
		VALUES %v
	`, database, values)
}
