package storage

import (
	"path/filepath"

	"github.com/markbates/pkger"
	migrate "github.com/rubenv/sql-migrate"
)

type Migrator struct {
	Conn   *Connection
	Path   string
	source *migrate.HttpFileSystemMigrationSource
}

func NewMigrator(conn *Connection, path string) Migrator {
	absPath, _ := filepath.Abs(path)
	return Migrator{
		conn,
		absPath,
		&migrate.HttpFileSystemMigrationSource{
			FileSystem: pkger.Dir(absPath),
		},
	}
}

// MigrateUp executes forward migrations.
func (m *Migrator) MigrateUp() {
	n, err := migrate.Exec(m.Conn.DB.DB, m.Conn.dialect, m.source, migrate.Up)
	if err != nil {
		m.Conn.logger.Log().Panicf("failed to migrate the database up: %v", err)
	}
	m.Conn.logger.Log().Infof("%v migrations applied from %s", n, m.Path)
}

// MigrateDown undoes a specified number of migrations.
func (m *Migrator) MigrateDown(max int) {
	n, err := migrate.ExecMax(m.Conn.DB.DB, m.Conn.dialect, m.source, migrate.Down, max)
	if err != nil {
		m.Conn.logger.Log().Panicf("failed to migrate the database down: %v", err)
	}
	m.Conn.logger.Log().Infof("%v migrations un-applied", n)
}
