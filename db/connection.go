package db

import (
	"fmt"

	"github.com/jinzhu/gorm"
	_ "github.com/jinzhu/gorm/dialects/postgres" // Dialect import
	"github.com/lbryio/lbryweb.go/config"
	"github.com/lbryio/lbryweb.go/monitor"
	log "github.com/sirupsen/logrus"
)

// DB holds global database connection
var DB *gorm.DB

// User contains necessary internal-apis and SDK's account_id
type User struct {
	AuthToken    string
	SDKAccountID string
}

type connectionParams struct {
	DatabaseConnection string
	DatabaseName       string
	DatabaseOptions    string
}

func GetURL(params connectionParams) string {
	if params.DatabaseConnection == "" {
		params.DatabaseConnection = config.Settings.GetString("DatabaseConnection")
	}
	if params.DatabaseName == "" {
		params.DatabaseName = config.Settings.GetString("DatabaseName")
	}
	if params.DatabaseOptions == "" {
		params.DatabaseOptions = config.Settings.GetString("DatabaseOptions")
	}
	return fmt.Sprintf(
		"%v/%v?%v",
		params.DatabaseConnection,
		params.DatabaseName,
		params.DatabaseOptions,
	)
}

func init() {
	initializeConnection()
}

func openDefaultDB() {
	var err error

	dbURL := GetURL(connectionParams{})
	monitor.Logger.WithFields(log.Fields{
		"db_url": dbURL,
	}).Info("connecting to the database")
	DB, err = gorm.Open("postgres", dbURL)
	if err != nil {
		panic(err)
	}
}

func initializeConnection() {
	dbName := config.Settings.GetString("DatabaseName")
	dbURL := GetURL(connectionParams{DatabaseName: "postgres"})

	if dbName == "" {
		panic("DatabaseName not configured")
	}
	monitor.Logger.WithFields(log.Fields{
		"db_url": dbURL,
	}).Info("connecting to the database")

	db, err := gorm.Open("postgres", dbURL)
	if err != nil {
		panic(err)
	}
	db = db.Exec(fmt.Sprintf("CREATE DATABASE %v;", dbName))
	if db.Error != nil {
		openDefaultDB()
	}
}
