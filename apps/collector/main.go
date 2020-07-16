package main

import (
	"math/rand"
	"time"

	"github.com/lbryio/lbrytv/apps/collector/cmd"
)

func main() {
	rand.Seed(time.Now().UnixNano())
	cmd.Execute()
}
