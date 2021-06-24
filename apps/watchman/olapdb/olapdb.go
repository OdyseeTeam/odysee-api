package olapdb

import (
	"database/sql"
	"time"

	"github.com/lbryio/lbrytv/apps/watchman/gen/reporter"
	"github.com/lbryio/lbrytv/apps/watchman/log"

	"github.com/ClickHouse/clickhouse-go"
)

var (
	conn *sql.DB
	Log  = log.Log.Named("clickhouse")
)

func Connect(url string) {
	var err error
	conn, err = sql.Open("clickhouse", url)
	if err != nil {
		Log.Fatal(err)
	}
	if err := conn.Ping(); err != nil {
		if exception, ok := err.(*clickhouse.Exception); ok {
			Log.Fatalf("[%d] %s \n%s\n", exception.Code, exception.Message, exception.StackTrace)
		} else {
			Log.Fatal(err)
		}
	}
	_, err = conn.Exec(`
	CREATE TABLE IF NOT EXISTS watchman.playback
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
		"ClientRate" UInt32,
		"ClientDevice" FixedString(3),
		"ClientArea" FixedString(3)
	)
	ENGINE = MergeTree
	ORDER BY (Timestamp, UserID, URL)
	TTL Timestamp + INTERVAL 30 DAY`)
	if err != nil {
		Log.Fatal(err)
	}
	Log.Infof("connected to clickhouse server %v", url)
}

func Write(stmt *sql.Stmt, r *reporter.PlaybackReport, addr string) {
	area := getClientArea(addr)
	t, err := time.Parse(time.RFC1123, *&r.T)
	if err != nil {
		t = time.Now()
	}
	_, err = stmt.Exec(r.URL, uint32(r.Dur), t, uint32(r.Position), uint8(r.RelPosition), uint8(r.RebufCount),
		uint32(r.RebufDuration), r.Format, r.Player, uint32(r.UserID), uint32(*r.ClientRate), r.Device, area,
	)
	if err != nil {
		Log.Error(err)
	}
}

func prepareWrite(tx *sql.Tx) (*sql.Stmt, error) {
	return tx.Prepare(`
	INSERT INTO watchman.playback
		(URL, Duration, Timestamp, Position, RelPosition, RebufCount,
			RebufDuration, Format, Player, UserID, ClientRate, ClientDevice, ClientArea)
	VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	`)
}

func getClientArea(ip string) string {
	return "eu"
}
