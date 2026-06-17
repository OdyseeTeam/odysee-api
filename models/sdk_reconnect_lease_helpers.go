package models

import (
	"time"

	"github.com/volatiletech/sqlboiler/boil"
	"github.com/volatiletech/sqlboiler/queries"
)

func SDKReconnectLeaseDBNow(exec boil.Executor) (time.Time, error) {
	var now time.Time
	row := queries.Raw("SELECT now()").QueryRow(exec)
	err := row.Scan(&now)
	return now, err
}
