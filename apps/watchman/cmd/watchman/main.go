package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"os"
	"os/signal"
	"sync"
	"syscall"

	watchman "github.com/lbryio/lbrytv/apps/watchman"
	reporter "github.com/lbryio/lbrytv/apps/watchman/gen/reporter"
)

func main() {
	// Define command line flags, add any other flag required to configure the
	// service.
	var (
		bindF = flag.String("bind", ":8080", "Server listening address")
		dbgF  = flag.Bool("debug", false, "Log request and response bodies")
	)
	flag.Parse()

	// Setup logger. Replace logger with your own log package of choice.
	var (
		logger *log.Logger
	)
	{
		logger = log.New(os.Stderr, "[watchman] ", log.Ltime)
	}

	// Initialize the services.
	var (
		reporterSvc reporter.Service
	)
	{
		reporterSvc = watchman.NewReporter(nil, logger)
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
	handleHTTPServer(ctx, *bindF, reporterEndpoints, &wg, errc, logger, *dbgF)

	// Wait for signal.
	logger.Printf("exiting (%v)", <-errc)

	// Send cancellation signal to the goroutines.
	cancel()

	wg.Wait()
	logger.Println("exited")
}
