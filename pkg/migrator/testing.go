package migrator

import (
	"database/sql"
	"embed"

	"github.com/Pallinder/go-randomdata"
)

type TestDBCleanup func() error

func CreateTestDB(cfg *DBConfig, mfs embed.FS) (*sql.DB, TestDBCleanup, error) {
	db, err := ConnectDB(cfg.NoMigration(), mfs)
	tdbn := "test-db-" + randomdata.Alphanumeric(12)
	if err != nil {
		return nil, nil, err
	}
	m := New(db, mfs)
	m.CreateDB(tdbn)

	tdb, err := ConnectDB(cfg.Name(tdbn), mfs)
	if err != nil {
		return nil, nil, err
	}
	tm := New(tdb, mfs)
	_, err = tm.MigrateUp(0)
	if err != nil {
		return nil, nil, err
	}
	return tdb, func() error {
		tdb.Close()
		err := m.DropDB(tdbn)
		db.Close()
		if err != nil {
			return err
		}
		return nil
	}, nil
}
