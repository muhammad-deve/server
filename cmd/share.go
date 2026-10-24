package cmd

import (
	"crypto/rand"
	"encoding/base64"
	"fmt"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	sharehttp "github.com/muhammad-deve/server/share"
	"github.com/muhammad-deve/server/tunnel"
	"github.com/spf13/cobra"
	"golang.org/x/term"
)

var (
	sharePassword  bool
	shareExpires   string
	shareNoPreview bool
)

var shareCmd = &cobra.Command{
	Use:   "share <folder>",
	Short: "Share a local folder with a secure public link",
	Args:  cobra.ExactArgs(1),
	RunE:  runShare,
}

func init() {
	rootCmd.AddCommand(shareCmd)
	shareCmd.Flags().BoolVar(&sharePassword, "password", false, "prompt for a password")
	shareCmd.Flags().StringVar(&shareExpires, "expires", "", "stop access after a duration or RFC3339 time (for example 2h)")
	shareCmd.Flags().BoolVar(&shareNoPreview, "no-preview", false, "disable inline file previews")
}

func runShare(cmd *cobra.Command, args []string) error {
	cmd.SilenceUsage = true
	root, err := filepath.Abs(args[0])
	if err != nil {
		return fmt.Errorf("resolve shared folder: %w", err)
	}
	info, err := os.Stat(root)
	if err != nil {
		return fmt.Errorf("open shared folder: %w", err)
	}
	if !info.IsDir() {
		return fmt.Errorf("%q is not a directory", args[0])
	}
	if isFilesystemRoot(root) {
		return fmt.Errorf("refusing to share a filesystem root; choose a specific folder")
	}

	expiresAt, err := parseShareExpiry(shareExpires)
	if err != nil {
		return err
	}
	password := ""
	if sharePassword {
		fmt.Fprint(cmd.ErrOrStderr(), "Share password: ")
		value, readErr := term.ReadPassword(int(os.Stdin.Fd()))
		fmt.Fprintln(cmd.ErrOrStderr())
		if readErr != nil {
			return fmt.Errorf("read password: %w", readErr)
		}
		password = strings.TrimSpace(string(value))
		if len(password) < 8 {
			return fmt.Errorf("password must contain at least 8 characters")
		}
	}

	token, err := secureToken(24)
	if err != nil {
		return fmt.Errorf("generate share token: %w", err)
	}
	handler, err := sharehttp.New(sharehttp.Config{
		Root: root, Token: token, Password: password, ExpiresAt: expiresAt, Preview: !shareNoPreview,
	})
	if err != nil {
		return err
	}
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return fmt.Errorf("start local file server: %w", err)
	}
	server := &http.Server{Handler: handler, ReadHeaderTimeout: 10 * time.Second, IdleTimeout: 2 * time.Minute}
	go func() { _ = server.Serve(listener) }()
	defer server.Close()

	fmt.Fprintf(cmd.OutOrStdout(), "Warning: files in %s will be remotely accessible while this command runs.\n", root)
	port := strconv.Itoa(listener.Addr().(*net.TCPAddr).Port)
	return tunnel.StartE(tunnel.Config{
		Port: port, Type: "share", Region: "eu", Reset: true,
		ShareToken: token, ExpiresAt: expiresAt, DisableInspector: true, ShareName: root,
	})
}

func secureToken(size int) (string, error) {
	buffer := make([]byte, size)
	if _, err := rand.Read(buffer); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(buffer), nil
}

func parseShareExpiry(value string) (time.Time, error) {
	value = strings.TrimSpace(value)
	if value == "" {
		return time.Time{}, nil
	}
	if duration, err := time.ParseDuration(value); err == nil {
		if duration <= 0 {
			return time.Time{}, fmt.Errorf("--expires must be in the future")
		}
		return time.Now().Add(duration), nil
	}
	parsed, err := time.Parse(time.RFC3339, value)
	if err != nil || !parsed.After(time.Now()) {
		return time.Time{}, fmt.Errorf("--expires must be a future duration or RFC3339 time")
	}
	return parsed, nil
}

func isFilesystemRoot(path string) bool {
	volume := filepath.VolumeName(path)
	clean := filepath.Clean(path)
	return clean == string(filepath.Separator) || clean == volume+string(filepath.Separator)
}
