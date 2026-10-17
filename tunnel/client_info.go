package tunnel

import (
	"errors"
	"io"
	"net"
	"os"
	"path/filepath"
	"strings"

	"github.com/hashicorp/yamux"
)

// installMethod reports how this binary was installed, judged from where it
// lives on disk. Only the resulting word is sent, never the path.
func installMethod() string {
	exe, err := os.Executable()
	if err != nil {
		return "unknown"
	}
	// Homebrew links bin/goport into its Cellar; follow that to the real file.
	if resolved, err := filepath.EvalSymlinks(exe); err == nil {
		exe = resolved
	}
	return classifyInstall(exe)
}

func classifyInstall(path string) string {
	p := strings.ToLower(filepath.ToSlash(path))
	switch {
	case strings.Contains(p, "/cellar/") || strings.Contains(p, "/homebrew/") || strings.Contains(p, "/linuxbrew/"):
		return "homebrew"
	case strings.Contains(p, "/scoop/"):
		return "scoop"
	case strings.Contains(p, "/chocolatey/"):
		return "chocolatey"
	case strings.Contains(p, "/programs/goport/"):
		// %LOCALAPPDATA%\Programs\GoPort, where the Windows installer puts it.
		return "installer"
	case strings.Contains(p, "/go-build"):
		return "go-run"
	case strings.Contains(p, "/go/bin/"):
		return "go-install"
	default:
		return "manual"
	}
}

// disconnectReason sums up why a tunnel session ended in one fixed word.
// The raw error is not sent because it contains local and remote addresses.
func disconnectReason(err error) string {
	if err == nil {
		return ""
	}
	if errors.Is(err, yamux.ErrKeepAliveTimeout) {
		return "keepalive_timeout"
	}
	if errors.Is(err, io.EOF) {
		return "server_closed"
	}
	var netErr net.Error
	if errors.As(err, &netErr) && netErr.Timeout() {
		return "timeout"
	}
	msg := strings.ToLower(err.Error())
	switch {
	case strings.Contains(msg, "keepalive"):
		return "keepalive_timeout"
	case strings.Contains(msg, "eof"):
		return "server_closed"
	case strings.Contains(msg, "forcibly closed"), strings.Contains(msg, "connection reset"), strings.Contains(msg, "broken pipe"):
		return "connection_reset"
	case strings.Contains(msg, "timeout"), strings.Contains(msg, "timed out"):
		return "timeout"
	default:
		return "other"
	}
}
