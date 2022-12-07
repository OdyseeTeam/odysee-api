package main

import (
	"path"

	"github.com/OdyseeTeam/odysee-api/apps/forklift"
	"github.com/OdyseeTeam/odysee-api/apps/lbrytv/config"
	"github.com/OdyseeTeam/odysee-api/pkg/logging/zapadapter"
)

func main() {
	logger := zapadapter.NewKV(nil)
	asynqRedisOpts, err := config.GetAsynqRedisOpts()
	if err != nil {
		logger.Fatal("cannot get redis config", "err", err)
	}
	fl, err := forklift.NewForklift(
		path.Join(uploadPath, "blobs"),
		config.GetReflectorUpstream(),
		asynqRedisOpts,
		forklift.WithConcurrency(config.GetGeoPublishConcurrency()),
		forklift.WithLogger(logger),
	)
	if err != nil {
		logger.Fatal("cannot initialize forklift", "err", err)
	}

	if err != nil {
		logger.Fatal("forklift exited", "err", fl.Start())
	}
}
