package migrator

import (
	"database/sql"
	"embed"
	"fmt"
)

type DSNConfig interface {
	GetFullDSN() string
	GetDBName() string
}

type DBConfig struct {
	appName, dsn, dbName, connOpts string
}

func DefaultDBConfig() *DBConfig {
	return &DBConfig{
		dsn:      "postgres://postgres:odyseeteam@localhost",
		dbName:   "postgres",
		connOpts: "sslmode=disable",
	}
}

func (c *DBConfig) DSN(dsn string) *DBConfig {
	c.dsn = dsn
	return c
}

func (c *DBConfig) Name(dbName string) *DBConfig {
	c.dbName = dbName
	return c
}

func (c *DBConfig) AppName(appName string) *DBConfig {
	c.appName = appName
	return c
}

func (c *DBConfig) ConnOpts(connOpts string) *DBConfig {
	c.connOpts = connOpts
	return c
}

func (c *DBConfig) GetFullDSN() string {
	return fmt.Sprintf("%s/%s?%s", c.dsn, c.dbName, c.connOpts)
}

func (c *DBConfig) GetDBName() string {
	return c.dbName
}

func ConnectDB(cfg DSNConfig, migrationsFS ...embed.FS) (*sql.DB, error) {
	var err error
	db, err := sql.Open("postgres", cfg.GetFullDSN())
	if err != nil {
		return nil, err
	}
	if len(migrationsFS) > 0 {
		_, err := New(db, migrationsFS[0]).MigrateUp(0)
		if err != nil {
			return nil, err
		}
	}

	return db, nil
}

func DBConfigFromApp(cfg DSNConfig) *DBConfig {
	return DefaultDBConfig().DSN(cfg.GetFullDSN()).Name(cfg.GetDBName())
}
