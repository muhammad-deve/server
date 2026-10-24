package cmd

import (
	"errors"

	"github.com/spf13/cobra"
)

// tcpCmd is kept so `goport tcp` explains itself instead of failing as an
// unknown command. The server only routes HTTP, so raw TCP can't work yet.
var tcpCmd = &cobra.Command{
	Use:    "tcp [port]",
	Short:  "Expose a local TCP port (not supported yet)",
	Hidden: true,
	Args:   cobra.ArbitraryArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		cmd.SilenceUsage = true
		return errors.New("raw TCP tunnels aren't supported yet; use goport http <port> for web apps and APIs")
	},
}

func init() {
	rootCmd.AddCommand(tcpCmd)
}
