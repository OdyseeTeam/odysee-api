package storage

import (
	"path/filepath"

	"github.com/markbates/pkger"
	migrate "github.com/rubenv/sql-migrate"
)

type Migrator struct {
	Path   string
	source *migrate.HttpFileSystemMigrationSource
}

func NewMigrator(path string) Migrator {
	absPath, _ := filepath.Abs(path)
	return Migrator{
		absPath,
		&migrate.HttpFileSystemMigrationSource{
			FileSystem: pkger.Dir(absPath),
		},
	}
}

// MigrateUp executes forward migrations.
func (m *Migrator) MigrateUp(conn *Connection) {
	n, err := migrate.Exec(conn.DB.DB, conn.dialect, m.source, migrate.Up)
	if err != nil {
		conn.logger.Log().Panicf("failed to migrate the database up: %v", err)
	}
	conn.logger.Log().Infof("%v migrations applied from %s", n, m.Path)
}

// MigrateDown undoes a specified number of migrations.
func (m *Migrator) MigrateDown(conn *Connection, max int) {
	n, err := migrate.ExecMax(conn.DB.DB, conn.dialect, m.source, migrate.Down, max)
	if err != nil {
		conn.logger.Log().Panicf("failed to migrate the database down: %v", err)
	}
	conn.logger.Log().Infof("%v migrations un-applied", n)
}
