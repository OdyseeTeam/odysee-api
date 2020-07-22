package main

import (
	"math/rand"
	"time"

	"github.com/lbryio/lbrytv/apps/collector/cmd"
	_ "github.com/lbryio/lbrytv/apps/collector/collector"
	"github.com/markbates/pkger"
)

var _ = pkger.Dir("/apps/collector/migrations")

func main() {
	rand.Seed(time.Now().UnixNano())
	cmd.Execute()
}
