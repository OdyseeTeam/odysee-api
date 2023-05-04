package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/OdyseeTeam/odysee-api/apps/upload"
	"github.com/OdyseeTeam/odysee-api/apps/upload/database"
	"github.com/OdyseeTeam/odysee-api/pkg/configng"
	"github.com/OdyseeTeam/odysee-api/pkg/keybox"
	"github.com/OdyseeTeam/odysee-api/pkg/logging"
	"github.com/OdyseeTeam/odysee-api/pkg/logging/zapadapter"
	"github.com/OdyseeTeam/odysee-api/pkg/migrator"
	"github.com/OdyseeTeam/odysee-api/pkg/redislocker"

	"github.com/alecthomas/kong"
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/credentials"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/s3"
	"github.com/go-redis/redis/v8"
	"github.com/tus/tusd/pkg/s3store"
)

var cli struct {
	S3    string `required:"" help:"S3 config"`
	Redis string `required:"" help:"Redis config"`
	DB    string `required:"" help:"DB config"`
	Debug bool   `help:"Enable verbose logging"`
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

	cfg, err := configng.Read(".", "upload", "yaml")
	if err != nil {
		panic(err)
	}
	sc := cfg.ReadS3Config("storage")

	s3cfg := aws.NewConfig().
		WithCredentials(credentials.NewStaticCredentials(sc.Key, sc.Secret, "")).
		WithRegion(sc.Region)

	if sc.Endpoint != "" {
		s3cfg = s3cfg.WithEndpoint(sc.Endpoint)
	}

	if sc.Minio {
		s3cfg = s3cfg.WithS3ForcePathStyle(true)
	}

	sess, err := session.NewSession(s3cfg)
	if err != nil {
		panic(fmt.Errorf("unable to create AWS session: %w", err))
	}
	client := s3.New(sess)
	store := s3store.New(sc.Bucket, client)

	redisOpts, err := redis.ParseURL(cfg.V.GetString("redis"))
	if err != nil {
		panic(fmt.Errorf("cannot parse redis config: %w", err))
	}
	locker, err := redislocker.New(redisOpts)
	if err != nil {
		panic(fmt.Errorf("cannot start redislocker: %w", err))
	}

	k, err := keybox.NewPublicKeyFromURL(cfg.V.GetString("publickeyurl"))
	if err != nil {
		panic(fmt.Errorf("cannot load key from url %s: %w", cfg.V.GetString("publickeyurl"), err))
	}

	pgcfg := cfg.ReadPostgresConfig("storage")
	db, err := migrator.ConnectDB(pgcfg, database.MigrationsFS)
	if err != nil {
		panic(fmt.Errorf("cannot connect to DB: %w", err))
	}

	runCtx, runCancel := context.WithCancel(context.Background())

	launcher := upload.NewLauncher().
		FileLocker(locker).
		Store(store).
		DB(db).
		PublicKey(k).
		Logger(logger)

	go func() {
		trap := make(chan os.Signal, 1)
		signal.Notify(trap, syscall.SIGTERM, syscall.SIGINT, syscall.SIGQUIT)
		<-trap

		launcher.StartShutdown()
		// Wait for the readiness probe to detect the failure
		<-time.After(30 * time.Second)
		launcher.ServerShutdown()
		launcher.CompleteShutdown()
		runCancel()
	}()

	launcher.Launch()
	<-runCtx.Done()
}
