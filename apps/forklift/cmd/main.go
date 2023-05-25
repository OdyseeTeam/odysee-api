package main

import (
	"fmt"

	"github.com/OdyseeTeam/odysee-api/apps/forklift"
	"github.com/OdyseeTeam/odysee-api/pkg/configng"
	"github.com/OdyseeTeam/odysee-api/pkg/logging"
	"github.com/OdyseeTeam/odysee-api/pkg/logging/zapadapter"
	"github.com/OdyseeTeam/odysee-api/pkg/migrator"

	"github.com/alecthomas/kong"
)

var cli struct {
	BlobsPath string `required:"" help:"Directory to store split blobs before sending them off to reflector"`
	Debug     bool   `help:"Enable verbose logging"`
}

type loggingConfig struct {
	level, format string
}

func main() {
	kong.Parse(&cli)
	lcfg := loggingConfig{}
	if cli.Debug {
		lcfg.format = "console"
		lcfg.level = logging.LevelDebug
	} else {
		lcfg.format = "json"
		lcfg.level = logging.LevelDebug
	}
	logger := zapadapter.NewKV(nil)

	cfg, err := configng.Read("./config", "upload", "yaml")
	if err != nil {
		panic(err)
	}

	s3cfg, err := cfg.ReadS3Config("IncomingStorage")
	if err != nil {
		panic(fmt.Errorf("cannot parse s3 config: %w", err))
	}

	client, err := configng.NewS3ClientV2(s3cfg)
	if err != nil {
		panic(fmt.Errorf("cannot create s3 client: %w", err))
	}

	pgcfg := cfg.ReadPostgresConfig("Database")
	db, err := migrator.ConnectDB(pgcfg)
	if err != nil {
		logger.Fatal("db connection failed", "err", err)
	}

	l := forklift.NewLauncher(
		forklift.WithLogger(logger),
		forklift.WithReflectorConfig(cfg.V.GetStringMapString("Reflector")),
		forklift.WithConcurrency(cfg.V.GetInt("Concurrency")),
		forklift.WithBlobPath(cfg.V.GetString("BlobPath")),
		forklift.WithRetriever(forklift.NewS3Retriever(cfg.V.GetString("UploadPath"), client)),
		forklift.WithRedisURL(cfg.V.GetString("RedisBus")),
		forklift.WithDB(db),
	)

	b, err := l.Build()
	if err != nil {
		panic(err)
	}
	b.StartHandlers()
}
