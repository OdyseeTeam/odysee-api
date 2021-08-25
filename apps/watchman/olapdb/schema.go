package olapdb

import (
	"context"
	"fmt"
	"time"
)

func MigrateUp(dbName string) error {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	_, err := conn.ExecContext(ctx, fmt.Sprintf(`CREATE DATABASE IF NOT EXISTS %v`, dbName))
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
		"Protocol" FixedString(3),
		"Cache" String,
		"Player" FixedString(16),
		"UserID" String,
		"Bandwidth" UInt32,
		"Device" FixedString(3),
		"Area" FixedString(2),
		"SubArea" FixedString(3),
		"IP" IPv6
	)
	ENGINE = MergeTree
	ORDER BY (Timestamp, UserID, URL)
	TTL Timestamp + INTERVAL 15 DAY`, dbName))
	if err != nil {
		return err
	}
	return nil
}
func MigrateDown(dbName string) error {
	_, err := conn.Exec(fmt.Sprintf(`DROP DATABASE %v`, dbName))
	if err != nil {
		return err
	}
	return nil
}
