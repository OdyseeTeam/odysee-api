package db

import (
	"github.com/jinzhu/gorm"
	_ "github.com/jinzhu/gorm/dialects/postgres" // Dialect import
	"github.com/lbryio/lbryweb.go/config"
)

// DB holds global database connection
var DB *gorm.DB

// User contains necessary internal-apis and SDK's account_id
type User struct {
	AuthToken    string
	SDKAccountID string
}

func init() {
	openConnection()
}

func openConnection() {
	var err error
	DB, err = gorm.Open("postgres", config.Settings.GetString("DbURL"))
	if err != nil {
		panic(err)
	}

}
