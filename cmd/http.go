package cmd

import (
	"fmt"

	"github.com/muhammad-deve/server/tunnel"
	"github.com/spf13/cobra"
)

var (
	customSubdomain string
	resetSubdomain  bool
	region          string
)

var httpCmd = &cobra.Command{
	Use:   "http [port]",
	Short: "Expose a local HTTP port",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		if resetSubdomain && customSubdomain != "" {
			return fmt.Errorf("--reset and --custom cannot be used together")
		}

		port := args[0]
		tunnel.Start(tunnel.Config{
			Port:      port,
			Subdomain: customSubdomain,
			Reset:     resetSubdomain,
			Region:    region,
			Type:      "http",
		})
		return nil
	},
}

func init() {
	rootCmd.AddCommand(httpCmd)
	httpCmd.Flags().StringVar(&customSubdomain, "custom", "", "custom subdomain")
	httpCmd.Flags().StringVarP(&customSubdomain, "name", "n", "", "custom subdomain")
	httpCmd.Flags().BoolVar(&resetSubdomain, "reset", false, "get a new random subdomain")
	httpCmd.Flags().StringVarP(&region, "region", "r", "eu", "region")
}
