package main

import (
	"context"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/OdyseeTeam/odysee-api/apps/uploads"
	"github.com/OdyseeTeam/odysee-api/apps/uploads/database"
	"github.com/OdyseeTeam/odysee-api/pkg/configng"
	"github.com/OdyseeTeam/odysee-api/pkg/keybox"
	"github.com/OdyseeTeam/odysee-api/pkg/logging"
	"github.com/OdyseeTeam/odysee-api/pkg/logging/zapadapter"
	"github.com/OdyseeTeam/odysee-api/pkg/migrator"
	"github.com/OdyseeTeam/odysee-api/pkg/redislocker"

	"github.com/alecthomas/kong"
	"github.com/go-redis/redis/v8"
)

var cli struct {
	migrator.CLI
	Serve struct{} `cmd:"" help:"Start upload service"`
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
	case "migrate-up":
		logger.Fatal("migrate command is not supported")
	default:
		logger.Fatal("unknown command", "name", ctx.Command())
	}
}

func serve(logger logging.KVLogger) {
	cfg, err := configng.Read("./config", "uploads", "yaml")
	if err != nil {
		logger.Fatal("config reading failed", "err", err)
	}
	s3cfg, err := cfg.ReadS3Config("Storage")
	if err != nil {
		logger.Fatal("s3 config failed", "err", err)
	}
	client, err := configng.NewS3Client(s3cfg)
	if err != nil {
		logger.Fatal("s3 client failed", "err", err)
	}

	redisOpts, err := redis.ParseURL(cfg.V.GetString("RedisLocker"))
	if err != nil {
		logger.Fatal("redis config parse failed", "err", err)
	}
	locker, err := redislocker.New(redisOpts)
	if err != nil {
		logger.Fatal("redislocker launch failed", "err", err)
	}

	k, err := keybox.PublicKeyFromURL(cfg.V.GetString("PublicKeyURL"))
	if err != nil {
		logger.Fatal("public key loading failed", "url", cfg.V.GetString("PublicKeyURL"), "err", err)
	}

	pgcfg := cfg.ReadPostgresConfig("Database")
	db, err := migrator.ConnectDB(pgcfg, database.MigrationsFS)
	if err != nil {
		logger.Fatal("db connection failed", "err", err)
	}

	runCtx, runCancel := context.WithCancel(context.Background())

	launcher := uploads.NewLauncher(
		uploads.WithFileLocker(locker),
		uploads.WithS3Client(client),
		uploads.WithS3Bucket(s3cfg.Bucket),
		uploads.WithDB(db),
		uploads.WithPublicKey(k),
		uploads.WithLogger(logger),
		uploads.WithCORSDomains(cfg.V.GetStringSlice("CORSDomains")),
		uploads.WithBusRedisURL(cfg.V.GetString("RedisBus")),
	)

	go func() {
		trap := make(chan os.Signal, 1)
		signal.Notify(trap, syscall.SIGTERM, syscall.SIGINT, syscall.SIGQUIT)
		<-trap

		launcher.StartShutdown()
		// Wait for the readiness probe to detect the failure
		<-time.After(cfg.V.GetDuration("GracefulShutdown"))
		launcher.CompleteShutdown()
		runCancel()
	}()

	_, err = launcher.Build()
	if err != nil {
		logger.Fatal(err.Error())
	}
	launcher.Launch()
	<-runCtx.Done()
}
