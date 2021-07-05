package olapdb

import (
	"database/sql"
	"fmt"
	"time"

	"github.com/lbryio/lbrytv/apps/watchman/gen/reporter"
	"github.com/lbryio/lbrytv/apps/watchman/log"

	"github.com/ClickHouse/clickhouse-go"
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
	if err := conn.Ping(); err != nil {
		if exception, ok := err.(*clickhouse.Exception); ok {
			log.Log.Named("clickhouse").Fatalf("[%d] %s \n%s\n", exception.Code, exception.Message, exception.StackTrace)
		} else {
			return err
		}
	}
	_, err = conn.Exec(fmt.Sprintf(`CREATE DATABASE IF NOT EXISTS %v`, dbName))
	if err != nil {
		return err
	}
	_, err = conn.Exec(fmt.Sprintf(`
	CREATE TABLE IF NOT EXISTS %v.playback
	(
		"URL" String,
		"Duration" UInt32,
		"Timestamp" Timestamp,
		"Position" UInt32,
		"RelPosition" UInt8,
		"RebufCount" UInt8,
		"RebufDuration" UInt32,
		"Format" FixedString(3),
		"Player" FixedString(16),
		"UserID" UInt32,
		"Rate" UInt32,
		"Device" FixedString(3),
		"Area" FixedString(3),
		"IP" String
	)
	ENGINE = MergeTree
	ORDER BY (Timestamp, UserID, URL)
	TTL Timestamp + INTERVAL 30 DAY`, dbName))
	if err != nil {
		return err
	}
	log.Log.Named("clickhouse").Infof("connected to clickhouse server %v (database=%v)", url, dbName)
	return nil
}

func Write(stmt *sql.Stmt, r *reporter.PlaybackReport, addr string) error {
	// t, err := time.Parse(time.RFC1123Z, r.T)
	t := time.Now()
	area := getArea(addr)
	_, err := stmt.Exec(
		r.URL,
		uint32(r.Duration),
		t,
		uint32(r.Position),
		uint8(r.RelPosition),
		uint8(r.RebufCount),
		uint32(r.RebufDuration),
		r.Format,
		r.Player,
		uint32(r.UserID),
		uint32(*r.Rate),
		r.Device,
		area,
		addr,
	)
	if err != nil {
		return err
	}
	log.Log.Named("clickhouse").Infow(
		"playback record written",
		"user_id", r.UserID, "url", r.URL, "rebuf_count", r.RebufCount, "area", area, "ip", addr)
	return nil
}

func WriteOne(r *reporter.PlaybackReport, addr string) error {
	tx, _ := conn.Begin()
	stmt, err := prepareWrite(tx)
	if err != nil {
		return err
	}
	Write(stmt, r, addr)
	if err := tx.Commit(); err != nil {
		return err
	}
	return nil
}

func prepareWrite(tx *sql.Tx) (*sql.Stmt, error) {
	return tx.Prepare(fmt.Sprintf(`
	INSERT INTO %v.playback
		(URL, Duration, Timestamp, Position, RelPosition, RebufCount,
			RebufDuration, Format, Player, UserID, Rate, Device, Area, IP)
	VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	`, database))
}
