package main

import (
	"github.com/OdyseeTeam/odysee-api/apps/forklift"
	"github.com/OdyseeTeam/odysee-api/pkg/logging"
	"github.com/OdyseeTeam/odysee-api/pkg/logging/zapadapter"
	"go.uber.org/zap"

	"github.com/alecthomas/kong"
)

var cli struct {
	BlobsPath string `required:"" help:"Directory to store split blobs before sending them off to reflector"`
	Debug     bool   `help:"Enable verbose logging"`
}

func main() {
	kong.Parse(&cli)
	var lcfg zap.Config
	if cli.Debug {
		lcfg = logging.Dev
	} else {
		lcfg = logging.Prod
	}
	logger := zapadapter.NewKV(logging.Create("forklift", lcfg).Desugar())
	forklift.Start(cli.BlobsPath, logger)
}
