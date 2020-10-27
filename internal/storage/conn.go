package storage

import (
	"fmt"
	"time"

	"github.com/lbryio/lbrytv/internal/metrics"
	"github.com/lbryio/lbrytv/internal/monitor"

	_ "github.com/jinzhu/gorm/dialects/postgres" // Dialect import
	"github.com/jmoiron/sqlx"
	"github.com/sirupsen/logrus"
	"github.com/volatiletech/sqlboiler/boil"
)

// Handler implements the app database handler.
type Handler interface {
	MigrateUp()
	MigrateDown()
	Connect()
}

// Connection holds connection data.
type Connection struct {
	DB      *sqlx.DB
	dialect string
	params  ConnParams
	logger  monitor.ModuleLogger
}

// ConnParams are accepted by InitConn, containing database server parameters.
type ConnParams struct {
	Connection     string
	DBName         string
	Options        string
	MigrationsPath string
}

// Conn holds a global database connection.
var Conn *Connection

// MakeDSN generates DSN string from ConnParams.
func MakeDSN(params ConnParams) string {
	return fmt.Sprintf(
		"%v/%v?%v",
		params.Connection,
		params.DBName,
		params.Options,
	)
}

// InitConn initializes a module-level connection object.
func InitConn(params ConnParams) *Connection {
	c := &Connection{
		dialect: "postgres",
		logger:  monitor.NewModuleLogger("storage"),
		params:  params,
	}
	return c
}

// Connect initiates a connection to the database server defined in c.params.
func (c *Connection) Connect() error {
	dsn := MakeDSN(c.params)
	c.logger.WithFields(logrus.Fields{"dsn": dsn}).Info("connecting to the DB")
	var err error
	var db *sqlx.DB

	if err != nil {
		c.logger.WithFields(logrus.Fields{"dsn": dsn}).Info("DB connection failed")
		return err
	}
	c.DB = db
	return nil
}

// SetDefaultConnection sets global database connection object that other packages can import and utilize.
// You want to call that once in your main.go (or another entrypoint) after the physical
// DB connection has been established.
func (c *Connection) SetDefaultConnection() {
	boil.SetDB(c.DB)
	Conn = c
}

// Close terminates the database server connection.
func (c *Connection) Close() error {
	err := c.DB.Close()
	if err != nil {
		c.logger.Log().Errorf("error closing connection to %s: %v", c.params.DBName, err)
	}
	return err
}

// SpawnConn creates a connection to another database on the same server.
func (c *Connection) SpawnConn(dbName string) (*Connection, error) {
	p := c.params
	p.DBName = dbName
	cSpawn := InitConn(p)
	return cSpawn, cSpawn.Connect()
}

func (c *Connection) WatchMetrics(interval time.Duration) {
	t := time.NewTicker(interval)
	for {
		<-t.C
		stats := c.DB.Stats()
		metrics.LbrytvDBOpenConnections.Set(float64(stats.OpenConnections))
		metrics.LbrytvDBInUseConnections.Set(float64(stats.InUse))
		metrics.LbrytvDBIdleConnections.Set(float64(stats.Idle))
	}
}
