package cmd

import (
	"github.com/muhammad-deve/server/tunnel"
	"github.com/spf13/cobra"
)

var tcpCmd = &cobra.Command{
	Use:   "tcp [port]",
	Short: "Expose a local TCP port",
	Args:  cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		port := args[0]
		tunnel.Start(tunnel.Config{
			Port: port,
			Type: "tcp",
		})
	},
}

func init() {
	rootCmd.AddCommand(tcpCmd)
}
