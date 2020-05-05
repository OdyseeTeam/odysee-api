package main

import (
	"math/rand"
	"net/http"
	"time"

	"github.com/lbryio/lbrytv/cmd"
	"github.com/lbryio/lbrytv/config"
	"github.com/lbryio/lbrytv/internal/reflection"
	"github.com/lbryio/lbrytv/internal/storage"
)

func main() {
	rand.Seed(time.Now().UnixNano())

	// this is a *client-side* timeout (for when we make http requests, not when we serve them)
	//https://blog.cloudflare.com/the-complete-guide-to-golang-net-http-timeouts/
	http.DefaultClient.Timeout = 20 * time.Second

	dbConfig := config.GetDatabase()
	conn := storage.InitConn(storage.ConnParams{
		Connection: dbConfig.Connection,
		DBName:     dbConfig.DBName,
		Options:    dbConfig.Options,
	})

	err := conn.Connect()
	if err != nil {
		panic(err)
	}
	defer conn.Close()
	conn.SetDefaultConnection()

	rMgr := reflection.NewManager("/nonexistent", config.GetReflectorAddress())
	rMgr.Initialize()
	rMgr.Start(time.Minute * 1)

	cmd.Execute()
}
