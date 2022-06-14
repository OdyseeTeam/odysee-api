package migrator

import (
	"database/sql"
	"embed"
	"fmt"
	"strings"

	"github.com/OdyseeTeam/odysee-api/internal/monitor"
	"github.com/sirupsen/logrus"

	"github.com/lib/pq"
	migrate "github.com/rubenv/sql-migrate"
)

const dialect = "postgres"

type Migrator struct {
	db     *sql.DB
	ms     migrate.MigrationSet
	source *migrate.EmbedFileSystemMigrationSource
	log    monitor.ModuleLogger
}

func New(db *sql.DB, fs embed.FS) Migrator {
	return Migrator{
		db: db,
		// ms: migrate.MigrationSet{TableName: migrTableName + "_gorp_migrations"},
		ms: migrate.MigrationSet{TableName: "gorp_migrations"},
		source: &migrate.EmbedFileSystemMigrationSource{
			FileSystem: fs,
			Root:       "migrations",
		},
		log: monitor.NewModuleLogger("migrator"),
	}
}

// MigrateUp executes forward migrations.
func (m Migrator) MigrateUp(max int) (int, error) {
	n, err := m.ms.ExecMax(m.db, dialect, m.source, migrate.Up, max)
	if err != nil {
		return 0, err
	}
	m.log.WithFields(logrus.Fields{"count": n}).Info("migrations applied")
	return n, nil
}

// MigrateDown undoes a specified number of migrations.
func (m Migrator) MigrateDown(max int) (int, error) {
	n, err := m.ms.ExecMax(m.db, dialect, m.source, migrate.Down, max)
	if err != nil {
		return 0, err
	}
	m.log.WithFields(logrus.Fields{"count": n}).Info("migrations unapplied")
	return n, nil
}

// Truncate purges records from the requested tables.
func (m Migrator) Truncate(tables []string) error {
	_, err := m.db.Exec(fmt.Sprintf("TRUNCATE %s CASCADE;", strings.Join(tables, ", ")))
	return err
}

// CreateDB creates the requested database.
func (m Migrator) CreateDB(dbName string) error {
	// fmt.Sprintf is used instead of query placeholders because postgres does not
	// handle them in schema-modifying queries.
	_, err := m.db.Exec(fmt.Sprintf("create database %s;", pq.QuoteIdentifier(dbName)))
	if err != nil {
		return err
	}
	m.log.WithFields(logrus.Fields{"db": dbName}).Info("database created")
	return nil
}

// DropDB drops the requested database.
func (m Migrator) DropDB(dbName string) error {
	_, err := m.db.Exec(fmt.Sprintf("drop database %s;", pq.QuoteIdentifier(dbName)))
	if err != nil {
		return err
	}
	m.log.WithFields(logrus.Fields{"db": dbName}).Info("database dropped")
	return nil
}
