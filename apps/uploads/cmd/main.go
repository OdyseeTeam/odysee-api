package main

import (
	"context"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/OdyseeTeam/odysee-api/apps/uploads"
	"github.com/OdyseeTeam/odysee-api/apps/uploads/database"
	"github.com/OdyseeTeam/odysee-api/internal/tasks"
	"github.com/OdyseeTeam/odysee-api/pkg/configng"
	"github.com/OdyseeTeam/odysee-api/pkg/keybox"
	"github.com/OdyseeTeam/odysee-api/pkg/logging"
	"github.com/OdyseeTeam/odysee-api/pkg/logging/zapadapter"
	"github.com/OdyseeTeam/odysee-api/pkg/migrator"
	"github.com/OdyseeTeam/odysee-api/pkg/redislocker"

	"github.com/alecthomas/kong"
	"github.com/redis/go-redis/v9"
)

var cli struct {
	migrator.CLI
	Serve         struct{} `cmd:"" help:"Start upload service"`
	RetryComplete struct {
		UploadID string `help:"Upload ID"`
		UserID   int32  `help:"User ID"`
	} `cmd:"" help:"Retry upload hand-off for further processing"`
	Debug bool `help:"Enable verbose logging"`
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
	case "retry-complete":
		retryComplete(logger)
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
		uploads.WithForkliftRequestsConnURL(cfg.V.GetString("ForkliftRequestsConnURL")),
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

	_, err = launcher.BuildHandler()
	if err != nil {
		logger.Fatal(err.Error())
	}
	launcher.Launch()
	<-runCtx.Done()
}

func retryComplete(logger logging.KVLogger) {
	cfg, err := configng.Read("./config", "uploads", "yaml")
	if err != nil {
		logger.Fatal("config reading failed", "err", err)
	}

	s3cfg, err := cfg.ReadS3Config("Storage")
	if err != nil {
		logger.Fatal("s3 config failed", "err", err)
	}

	pgcfg := cfg.ReadPostgresConfig("Database")
	db, err := migrator.ConnectDB(pgcfg, database.MigrationsFS)
	if err != nil {
		logger.Fatal("db connection failed", "err", err)
	}

	launcher := uploads.NewLauncher(
		uploads.WithDB(db),
		uploads.WithLogger(logger),
		uploads.WithForkliftRequestsConnURL(cfg.V.GetString("ForkliftRequestsConnURL")),
	)
	notifier, err := launcher.Notifier()
	if err != nil {
		logger.Fatal(err.Error())
	}

	queries := database.New(db)
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	upload, err := queries.GetUpload(ctx, database.GetUploadParams{
		ID:     cli.RetryComplete.UploadID,
		UserID: cli.RetryComplete.UserID,
	})
	if err != nil {
		logger.Fatal("failed to retrieve upload", "upload_id", cli.RetryComplete.UploadID, "user_id", cli.RetryComplete.UserID, "err", err)
	}
	if upload.Status != database.UploadStatusCompleted {
		logger.Fatal("upload is not in completed state", "status", upload.Status)
	}

	err = notifier.UploadReceived(
		upload.UserID,
		upload.ID,
		upload.Filename,
		tasks.FileLocationS3{
			Key:    upload.Key,
			Bucket: s3cfg.Bucket,
		})
	if err != nil {
		logger.Fatal("failed to complete upload", "err", err)
		return
	}
	logger.Info("upload sent off for processing")
}
