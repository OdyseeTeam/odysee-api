package main

import (
	"github.com/lbryio/lbrytv/apps/environment"
	"github.com/lbryio/lbrytv/apps/watchman/migrations"
	"github.com/lbryio/lbrytv/internal/monitor"
	"github.com/lbryio/lbrytv/internal/storage"

	"github.com/alecthomas/kong"
)

var logger = monitor.NewModuleLogger("watchman")

var CLI struct {
	Serve struct {
		Bind      string `optional name:"bind" help:"Address to listen on." default:":8080"`
		DataPath  string `optional name:"data-path" help:"Path to store database files and configs." type:"existingdir" default:"."`
		VideoPath string `optional name:"video-path" help:"Path to store video." type:"existingdir" default:"."`
		Workers   int    `optional name:"workers" help:"Number of workers to start." type:"int" default:"10"`
		CDN       string `optional name:"cdn" help:"LBRY CDN endpoint address."`
		Debug     bool   `optional name:"debug" help:"Debug mode."`
	} `cmd help:"Start transcoding server."`
	DBMigrateUp   struct{} `cmd help:"Migrate database up."`
	DBMigrateDown struct{} `cmd help:"Migrate database down."`
}

func main() {
	ctx := kong.Parse(&CLI)
	e := environment.ForWatchman()
	conn := e.Get("storage").(*storage.Connection)

	switch ctx.Command() {
	case "serve":
		logger.Log().Fatal("not implemented")
	case "db-migrate-up":
		n, err := migrations.Migrate(conn.DB.DB, migrations.Up, 0)
		if err != nil {
			logger.Log().Fatal("couldn't apply migrations:", err)
		}
		logger.Log().Infof("%v migrations applied", n)
	case "db-migrate-down":
		n, err := migrations.Migrate(conn.DB.DB, migrations.Down, 0)
		if err != nil {
			logger.Log().Fatal("couldn't apply migrations:", err)
		}
		logger.Log().Infof("%v migrations unapplied", n)
	default:
		logger.Log().Fatalf("command not found: %v", ctx.Command())
	}
}
