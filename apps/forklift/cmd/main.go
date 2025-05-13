package main

import (
	"fmt"
	"os"

	"github.com/OdyseeTeam/odysee-api/apps/forklift"
	"github.com/OdyseeTeam/odysee-api/pkg/configng"
	"github.com/OdyseeTeam/odysee-api/pkg/logging"
	"github.com/OdyseeTeam/odysee-api/pkg/logging/zapadapter"
	"github.com/OdyseeTeam/odysee-api/pkg/migrator"

	"github.com/alecthomas/kong"
)

var cli struct {
	Serve struct{} `cmd:"" help:"Start forklift service"`
	Debug bool     `help:"Enable verbose logging"`
}

type loggingConfig struct {
	level, format string
}

func main() {
	ctx := kong.Parse(&cli)

	logCfg := loggingConfig{}
	if cli.Debug {
		logCfg.format = "console"
		logCfg.level = logging.LevelDebug
	} else {
		logCfg.format = "json"
		logCfg.level = logging.LevelDebug
	}
	logger := zapadapter.NewKV(nil)

	switch ctx.Command() {
	case "serve":
		serve(logger)
	default:
		logger.Fatal("unknown command", "name", ctx.Command())
	}
}

func serve(logger logging.KVLogger) {
	cfg, err := configng.Read("./config", "forklift", "yaml")
	if err != nil {
		panic(err)
	}

	s3cfg, err := cfg.ReadS3Config("IncomingStorage")
	if err != nil {
		panic(fmt.Errorf("cannot parse s3 config: %w", err))
	}

	s3cfg.VerifyBucket = true
	client, err := configng.NewS3ClientV2(s3cfg)
	if err != nil {
		panic(fmt.Errorf("cannot create s3 client: %w", err))
	}
	logger.Debug("incoming s3 client configured", "region", s3cfg.Region, "endpoint", s3cfg.Endpoint, "bucket", s3cfg.Bucket)

	pgcfg := cfg.ReadPostgresConfig("Database")
	db, err := migrator.ConnectDB(pgcfg)
	if err != nil {
		logger.Fatal("db connection failed", "err", err)
	}

	blobPath := cfg.V.GetString("BlobPath")
	uploadPath := cfg.V.GetString("UploadPath")
	err = os.MkdirAll(blobPath, os.ModePerm)
	if err != nil {
		logger.Fatal("failed to create working directory", "err", err, "path", blobPath)
	}
	err = os.MkdirAll(uploadPath, os.ModePerm)
	if err != nil {
		logger.Fatal("failed to create working directory", "err", err, "path", uploadPath)
	}

	l := forklift.NewLauncher(
		forklift.WithDB(db),
		forklift.WithReflectorConfig(cfg.V.Sub("ReflectorStorage")),
		forklift.WithConcurrency(cfg.V.GetInt("Concurrency")),
		forklift.WithReflectorWorkers(cfg.V.GetInt("ReflectorWorkers")),
		forklift.WithBlobPath(blobPath),
		forklift.WithS3Client(client),
		forklift.WithDownloadsPath(uploadPath),
		forklift.WithRequestsConnURL(cfg.V.GetString("ForkliftRequestsConnURL")),   // Redis connection for listening to complete upload requests
		forklift.WithResponsesConnURL(cfg.V.GetString("AsynqueryRequestsConnURL")), // Redis connection for publishing processed upload results
		forklift.WithLogger(logger),
		forklift.ExposeMetrics(),
	)

	b, err := l.Build()
	if err != nil {
		panic(err)
	}
	b.ServeUntilShutdown()
}
