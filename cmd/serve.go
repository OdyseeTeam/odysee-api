package cmd

import (
	"fmt"
	"math/rand"
	"net/http"
	"os"
	"time"

	"github.com/lbryio/lbrytv/server"

	"github.com/spf13/cobra"
)

var rootCmd = &cobra.Command{
	Use:   "lbrytv",
	Short: "lbrytv is a backend API server for lbry.tv frontend",
	Run: func(cmd *cobra.Command, args []string) {
		server.ServeUntilInterrupted()
	},
}

func init() {
	rand.Seed(time.Now().UnixNano())

	// this is a *client-side* timeout (for when we make http requests, not when we serve them)
	//https://blog.cloudflare.com/the-complete-guide-to-golang-net-http-timeouts/
	http.DefaultClient.Timeout = 20 * time.Second
}

func Execute() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
}
