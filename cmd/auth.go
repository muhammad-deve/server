package cmd

import (
	"fmt"
	"strings"

	"github.com/muhammad-deve/server/cmd/config"
	"github.com/spf13/cobra"
)

var authCmd = &cobra.Command{
	Use:   "auth [token]",
	Short: "Link this machine to your GoPort account",
	Long: "Save the account token from your GoPort dashboard so your tunnels and\n" +
		"their traffic stats are linked to your account.\n\n" +
		"Run `goport auth --logout` to forget the saved token.",
	Args: cobra.MaximumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		logout, _ := cmd.Flags().GetBool("logout")
		if logout {
			if err := config.ClearToken(); err != nil {
				return fmt.Errorf("failed to clear token: %w", err)
			}
			fmt.Println("Logged out. Tunnels will now be anonymous.")
			return nil
		}

		if len(args) == 0 {
			if config.Token != "" {
				fmt.Printf("Authenticated. Token saved at %s\n", config.TokenPath())
				return nil
			}
			return fmt.Errorf("provide your token: goport auth <token>")
		}

		token := strings.TrimSpace(args[0])
		if token == "" {
			return fmt.Errorf("token cannot be empty")
		}
		if err := config.SaveToken(token); err != nil {
			return fmt.Errorf("failed to save token: %w", err)
		}

		fmt.Println("Authenticated. Your tunnels are now linked to your account.")
		return nil
	},
}

func init() {
	authCmd.Flags().Bool("logout", false, "forget the saved account token")
	rootCmd.AddCommand(authCmd)
}
