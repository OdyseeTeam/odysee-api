package main

import (
	"github.com/lbryio/lbrytv/cmd"
	"github.com/lbryio/lbrytv/config"
	"github.com/lbryio/lbrytv/internal/storage"
)

func main() {
	config.InitConfig()

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

	cmd.Execute()
}
