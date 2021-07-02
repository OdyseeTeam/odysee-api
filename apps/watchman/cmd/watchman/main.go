package main

import (
	"context"
	"fmt"
	stdlog "log"
	"os"
	"os/signal"
	"sync"
	"syscall"

	"github.com/lbryio/lbrytv/apps/watchman"
	"github.com/lbryio/lbrytv/apps/watchman/config"
	"github.com/lbryio/lbrytv/apps/watchman/gen/reporter"
	"github.com/lbryio/lbrytv/apps/watchman/log"
	"github.com/lbryio/lbrytv/apps/watchman/olapdb"

	"github.com/alecthomas/kong"
)

var CLI struct {
	Serve struct {
		Bind  string `optional name:"bind" help:"Server listening address" default:":8080"`
		Debug bool   `optional name:"debug" help:"Log request and response bodies"`
	} `cmd help:"Start watchman service"`
	Generate struct {
		Number int `required name:"number" help:"Number of records to generate"`
	} `cmd help:"Generate test data"`
}

func main() {
	cfg, err := config.Read()
	if err != nil {
		log.Log.Fatal(err)
	}
	logCfg := cfg.GetStringMapString("log")
	if logCfg["encoding"] == "" {
		logCfg["encoding"] = log.EncodingConsole
	}
	if logCfg["level"] == "" {
		logCfg["encoding"] = log.LevelDebug
	}
	log.Configure(logCfg["level"], logCfg["encoding"])

	dbCfg := cfg.GetStringMapString("clickhouse")
	err = olapdb.Connect(dbCfg["url"])
	if err != nil {
		log.Log.Fatal(err)
	}
	err = olapdb.OpenGeoDB(cfg.GetString("geoipdb"))
	if err != nil {
		log.Log.Fatal(err)
	}

	ctx := kong.Parse(&CLI)
	switch ctx.Command() {
	case "serve":
		serve(CLI.Serve.Bind, CLI.Serve.Debug)
	case "generate":
		generate(CLI.Generate.Number)
	default:
		log.Log.Fatal(ctx.Command())
	}
}

func serve(bindF string, dbgF bool) {

	// Initialize the services.
	var (
		reporterSvc reporter.Service
	)
	{
		// TODO: provide DB connection as the first argument
		reporterSvc = watchman.NewReporter(nil, log.Log)
	}

	// Wrap the services in endpoints that can be invoked from other services
	// potentially running in different processes.
	var (
		reporterEndpoints *reporter.Endpoints
	)
	{
		reporterEndpoints = reporter.NewEndpoints(reporterSvc)
	}

	// Create channel used by both the signal handler and server goroutines
	// to notify the main goroutine when to stop the server.
	errc := make(chan error)

	// Setup interrupt handler. This optional step configures the process so
	// that SIGINT and SIGTERM signals cause the services to stop gracefully.
	go func() {
		c := make(chan os.Signal, 1)
		signal.Notify(c, syscall.SIGINT, syscall.SIGTERM)
		errc <- fmt.Errorf("%s", <-c)
	}()

	var wg sync.WaitGroup
	ctx, cancel := context.WithCancel(context.Background())

	// Start the servers and send errors (if any) to the error channel.
	handleHTTPServer(ctx, bindF, reporterEndpoints, &wg, errc, stdlog.New(os.Stderr, "[watchman] ", stdlog.Ltime), dbgF)

	// Wait for signal.
	log.Log.Infof("exiting (%v)", <-errc)

	// Send cancellation signal to the goroutines.
	cancel()

	wg.Wait()
	log.Log.Info("exited")
}

func generate(cnt int) {
	olapdb.Generate(cnt)
}
