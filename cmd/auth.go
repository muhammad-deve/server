package cmd

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/muhammad-deve/server/cmd/config"
	"github.com/spf13/cobra"
)

var authCmd = &cobra.Command{
	Use:   "auth [token]",
	Short: "Link this machine to your GoPort account",
	Long: "Save the account token from your GoPort dashboard so your tunnels and\n" +
		"their traffic stats are linked to your account.\n\n" +
		"The token is verified with the GoPort server before it is saved, so an\n" +
		"invalid token is rejected instead of silently accepted.\n\n" +
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

		// Verify the token with the server before saving it. This is what
		// stops an invalid token from being accepted (and tunnels from
		// silently sharing the anonymous account).
		account, err := verifyToken(token)
		if err != nil {
			return err
		}

		if err := config.SaveToken(token); err != nil {
			return fmt.Errorf("failed to save token: %w", err)
		}

		who := strings.TrimSpace(account)
		if who != "" {
			fmt.Printf("Authenticated as %s. Your tunnels are now linked to your account.\n", who)
		} else {
			fmt.Println("Authenticated. Your tunnels are now linked to your account.")
		}
		return nil
	},
}

// verifyToken asks the GoPort backend whether the token is valid. On success it
// returns a human-readable account label (email or name) for the confirmation
// message. An invalid token, or any failure to reach the server, returns an
// error so the token is never saved without being checked.
func verifyToken(token string) (string, error) {
	url := config.APIBaseURL + "/api/v1/auth/verify-token"
	payload, _ := json.Marshal(map[string]string{"token": token})

	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Post(url, "application/json", bytes.NewReader(payload))
	if err != nil {
		return "", fmt.Errorf("could not reach GoPort to verify the token: %w", err)
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)

	if resp.StatusCode == http.StatusOK {
		var ok struct {
			Email string `json:"email"`
			Name  string `json:"name"`
		}
		_ = json.Unmarshal(body, &ok)
		if ok.Email != "" {
			return ok.Email, nil
		}
		return ok.Name, nil
	}

	if resp.StatusCode == http.StatusUnauthorized {
		return "", fmt.Errorf("invalid token. Copy a valid token from your GoPort dashboard")
	}

	var apiErr struct {
		Error string `json:"error"`
	}
	if json.Unmarshal(body, &apiErr) == nil && apiErr.Error != "" {
		return "", fmt.Errorf("could not verify token: %s", apiErr.Error)
	}
	return "", fmt.Errorf("could not verify token (server returned %d)", resp.StatusCode)
}

func init() {
	authCmd.Flags().Bool("logout", false, "forget the saved account token")
	rootCmd.AddCommand(authCmd)
}
