package db

import (
	"fmt"

	"github.com/lbryio/lbrytv/config"
	"github.com/lbryio/lbrytv/internal/monitor"

	"github.com/gobuffalo/packr/v2"
	_ "github.com/jinzhu/gorm/dialects/postgres" // Dialect import
	"github.com/jmoiron/sqlx"
	"github.com/lib/pq"
	migrate "github.com/rubenv/sql-migrate"
	"github.com/volatiletech/sqlboiler/boil"
)

// Connection implements the app database handler.
type Connection interface {
	MigrateUp()
	MigrateDown()
}

// ConnData holds connection data.
type ConnData struct {
	DB      *sqlx.DB
	dialect string
	Logger  monitor.ModuleLogger
}

// ConnParams holds database server parameters.
type ConnParams struct {
	DatabaseConnection string
	DatabaseName       string
	DatabaseOptions    string
}

// Conn is a module-level connection-holding variable
var Conn *ConnData

func init() {
	Init()
}

// Init initializes a module-level connection object
func Init() *ConnData {
	if Conn == nil {
		Conn = NewConnection(GetDefaultDSN())
	}
	boil.SetDB(Conn.DB)
	return Conn
}

// GetDSN generates DSN string from config parameters, which can be overridden in params.
func GetDSN(params ConnParams) string {
	if params.DatabaseConnection == "" {
		params.DatabaseConnection = config.GetDatabaseConnection()
	}
	if params.DatabaseName == "" {
		params.DatabaseName = config.GetDatabaseName()
	}
	if params.DatabaseOptions == "" {
		params.DatabaseOptions = config.GetDatabaseOptions()
	}
	return fmt.Sprintf(
		"%v/%v?%v",
		params.DatabaseConnection,
		params.DatabaseName,
		params.DatabaseOptions,
	)
}

// GetDefaultDSN is a wrapper for GetDSN without params.
func GetDefaultDSN() string {
	return GetDSN(ConnParams{})
}

// NewConnection sets up a database object, panics if unable to connect.
func NewConnection(dsn string) *ConnData {
	c := ConnData{dialect: "postgres", Logger: monitor.NewModuleLogger("storage")}
	c.Logger.LogF(monitor.F{"dsn": dsn}).Info("connecting to the database")
	db, err := connect(c.dialect, dsn)
	if err != nil {
		panic(err)
	}

	c.DB = db
	return &c
}

func connect(dialect, dsn string) (*sqlx.DB, error) {
	conn, err := sqlx.Connect(dialect, dsn)
	return conn, err
}

// MigrateUp executes forward migrations.
func (c ConnData) MigrateUp() {
	migrations := &migrate.PackrMigrationSource{
		Box: packr.New("migrations", "./migrations"),
		Dir: ".",
	}
	n, err := migrate.Exec(c.DB.DB, c.dialect, migrations, migrate.Up)
	if err != nil {
		c.Logger.Log().Panicf("failed to migrate the database up: %v", err)
	}
	c.Logger.LogF(monitor.F{"migrations_number": n}).Info("migrated the database up")
}

// MigrateDown undoes the previous migration.
func (c ConnData) MigrateDown() {
	migrations := &migrate.PackrMigrationSource{
		Box: packr.New("migrations", "./migrations"),
		Dir: ".",
	}
	n, err := migrate.Exec(c.DB.DB, c.dialect, migrations, migrate.Down)
	if err != nil {
		c.Logger.Log().Panicf("failed to migrate the database down: %v", err)
	}
	c.Logger.LogF(monitor.F{"migrations_number": n}).Info("migrated the database down")
}

// CreateDB creates the requested database.
func CreateDB(dbName string) (err error) {
	c := NewConnection(GetDSN(ConnParams{DatabaseName: "postgres"}))
	// fmt.Sprintf is used instead of query placeholders because postgres does not
	// handle them in schema-modifying queries
	_, err = c.DB.Exec(fmt.Sprintf("create database %s;", pq.QuoteIdentifier(dbName)))
	return err
}

// DropDB drops the requested database.
func DropDB(dbName string) (err error) {
	c := NewConnection(GetDSN(ConnParams{DatabaseName: "postgres"}))
	_, err = c.DB.Exec(fmt.Sprintf("drop database %s;", pq.QuoteIdentifier(dbName)))
	return err
}
