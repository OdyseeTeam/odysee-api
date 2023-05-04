package migrator

import (
	"database/sql"
	"embed"
	"fmt"

	"github.com/Pallinder/go-randomdata"
)

type TestDBCleanup func() error

func CreateTestDB(cfg *DBConfig, mfs embed.FS) (*sql.DB, TestDBCleanup, error) {
	db, err := ConnectDB(cfg)
	testDBName := fmt.Sprintf("test-%s-%s-%s", cfg.dbName, randomdata.Noun(), randomdata.Adjective())
	if err != nil {
		return nil, nil, err
	}
	m := New(db, mfs)
	m.CreateDB(testDBName)

	tdb, err := ConnectDB(cfg.Name(testDBName), mfs)
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
		err := m.DropDB(testDBName)
		db.Close()
		if err != nil {
			return err
		}
		return nil
	}, nil
}
