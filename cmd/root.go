package cmd

import (
	"os"

	"github.com/muhammad-deve/server/cmd/config"
	"github.com/muhammad-deve/server/tunnel"
	"github.com/spf13/cobra"
)

var rootCmd = &cobra.Command{
	Use:     "goport",
	Short:   "Expose your localhost to the internet",
	Version: tunnel.Version,
}

func Execute() {
	if err := rootCmd.Execute(); err != nil {
		os.Exit(1)
	}
}

func init() {
	config.Load()
}
