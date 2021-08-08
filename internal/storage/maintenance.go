package storage

import (
	"fmt"
	"strings"

	"github.com/gobuffalo/packr/v2"
	"github.com/lib/pq"
	migrate "github.com/rubenv/sql-migrate"
	"github.com/sirupsen/logrus"
	"github.com/volatiletech/sqlboiler/queries"
)

var utilDB = "postgres"

// MigrateUp executes forward migrations.
func (c *Connection) MigrateUp(nrMigrations int) {
	migrations := &migrate.PackrMigrationSource{
		Box: packr.New("migrations", "./migrations"),
		Dir: ".",
	}
	n, err := migrate.ExecMax(c.DB.DB, c.dialect, migrations, migrate.Up, nrMigrations)
	if err != nil {
		c.logger.Log().Panicf("failed to migrate the database up: %v", err)
	}
	c.logger.WithFields(logrus.Fields{"migrations_number": n}).Info("migrated the database up")
}

// MigrateDown undoes the previous migration.
func (c *Connection) MigrateDown(nrMigrations int) {
	migrations := &migrate.PackrMigrationSource{
		Box: packr.New("migrations", "./migrations"),
		Dir: ".",
	}
	n, err := migrate.ExecMax(c.DB.DB, c.dialect, migrations, migrate.Down, nrMigrations)
	if err != nil {
		c.logger.Log().Panicf("failed to migrate the database down: %v", err)
	}
	c.logger.WithFields(logrus.Fields{"migrations_number": n}).Info("migrated the database down")
}

// Truncate purges records from the requested tables.
func (c *Connection) Truncate(tables []string) {
	queries.Raw(fmt.Sprintf("TRUNCATE %s CASCADE;", strings.Join(tables, ", "))).Exec(c.DB)
	c.logger.Log().Infof("truncated tables %v", tables)
}

// CreateDB creates the requested database.
func (c *Connection) CreateDB(dbName string) error {
	utilConn, err := c.SpawnConn(utilDB)
	if err != nil {
		return err
	}
	defer utilConn.Close()
	// fmt.Sprintf is used instead of query placeholders because postgres does not
	// handle them in schema-modifying queries.
	_, err = utilConn.DB.Exec(fmt.Sprintf("create database %s;", pq.QuoteIdentifier(dbName)))
	c.logger.WithFields(logrus.Fields{"db_name": dbName}).Info("created the database")
	return err
}

// DropDB drops the requested database.
func (c *Connection) DropDB(dbName string) error {
	utilConn, err := c.SpawnConn(utilDB)
	if err != nil {
		return err
	}
	defer utilConn.Close()
	// fmt.Sprintf is used instead of query placeholders because postgres does not
	// handle them in schema-modifying queries.
	_, err = utilConn.DB.Exec(fmt.Sprintf("drop database %s;", pq.QuoteIdentifier(dbName)))
	c.logger.WithFields(logrus.Fields{"db_name": dbName}).Info("dropped the database")
	return err
}
