package sessions

import (
	"time"

	"github.com/lbryio/lbryweb.go/db"

	_ "github.com/jinzhu/gorm/dialects/postgres" // Dialect import
	"github.com/wader/gormstore"
)

var store *gormstore.Store

// User contains necessary internal-apis and SDK's account_id
type User struct {
	AuthToken    string
	SDKAccountID string
}

func init() {
	initializeStore()
}

func initializeStore() {
	store = gormstore.New(db.Conn, []byte("secret"))

	// db cleanup every hour
	// close quit channel to stop cleanup
	quit := make(chan struct{})
	go store.PeriodicCleanup(60*time.Minute, quit)
}
