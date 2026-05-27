package cmd

import (
	"github.com/muhammad-deve/server/tunnel"
	"github.com/spf13/cobra"
)

var (
	subdomain string
	region    string
)

var httpCmd = &cobra.Command{
	Use:   "http [port]",
	Short: "Expose a local HTTP port",
	Args:  cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		port := args[0]
		tunnel.Start(tunnel.Config{
			Port:      port,
			Subdomain: subdomain,
			Region:    region,
			Type:      "http",
		})
	},
}

func init() {
	rootCmd.AddCommand(httpCmd)
	httpCmd.Flags().StringVarP(&subdomain, "name", "n", "", "custom subdomain")
	httpCmd.Flags().StringVarP(&region, "region", "r", "eu", "region")
}
