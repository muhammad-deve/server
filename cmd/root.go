package cmd

import (
	"os"

	"github.com/muhammad-deve/server/cmd/config"
	"github.com/spf13/cobra"
)

var rootCmd = &cobra.Command{
	Use:   "goport",
	Short: "Expose your localhost to the internet",
}

func Execute() {
	if err := rootCmd.Execute(); err != nil {
		os.Exit(1)
	}
}

func init() {
	config.Load()
}
