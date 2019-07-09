package main

import (
	"github.com/lbryio/lbrytv/cmd"
	"github.com/lbryio/lbrytv/config"
)

func main() {
	config.InitConfig()
	cmd.Execute()
}
