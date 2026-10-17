package tunnel

import (
	"errors"
	"fmt"
	"io"
	"testing"
	"time"

	"github.com/hashicorp/yamux"
)

func TestClassifyInstall(t *testing.T) {
	cases := map[string]string{
		"/opt/homebrew/Cellar/goport/0.1.9/bin/goport":                  "homebrew",
		"/usr/local/Cellar/goport/0.1.9/bin/goport":                     "homebrew",
		"/home/linuxbrew/.linuxbrew/bin/goport":                         "homebrew",
		`C:\Users\a\scoop\apps\goport\current\goport-windows-amd64.exe`: "scoop",
		`C:\ProgramData\chocolatey\bin\goport.exe`:                      "chocolatey",
		`C:\Users\a\AppData\Local\Programs\GoPort\goport.exe`:           "installer",
		`C:\Users\a\AppData\Local\Temp\go-build123\b001\exe\goport.exe`: "go-run",
		"/home/a/go/bin/goport":                                         "go-install",
		"/usr/local/bin/goport":                                         "manual",
		`C:\Users\a\Downloads\goport-windows-amd64.exe`:                 "manual",
	}
	for path, want := range cases {
		if got := classifyInstall(path); got != want {
			t.Errorf("classifyInstall(%q) = %q, want %q", path, got, want)
		}
	}
}

func TestDisconnectReason(t *testing.T) {
	cases := []struct {
		err  error
		want string
	}{
		{nil, ""},
		{yamux.ErrKeepAliveTimeout, "keepalive_timeout"},
		{io.EOF, "server_closed"},
		{fmt.Errorf("read: %w", io.EOF), "server_closed"},
		{errors.New("read tcp 192.168.1.5:5000->161.97.135.5:7000: wsarecv: An existing connection was forcibly closed by the remote host."), "connection_reset"},
		{errors.New("read tcp 10.0.0.2:1->1.2.3.4:7000: read: connection reset by peer"), "connection_reset"},
		{errors.New("i/o timeout"), "timeout"},
		{errors.New("something else"), "other"},
	}
	for _, c := range cases {
		if got := disconnectReason(c.err); got != c.want {
			t.Errorf("disconnectReason(%v) = %q, want %q", c.err, got, c.want)
		}
	}
}

func TestNextReconnectDelay(t *testing.T) {
	// First minute: steady quick retries.
	if got := nextReconnectDelay(reconnectMinDelay, 5*time.Second); got != fastRetryDelay {
		t.Fatalf("early retry delay = %v, want %v", got, fastRetryDelay)
	}
	if got := nextReconnectDelay(fastRetryDelay, 59*time.Second); got != fastRetryDelay {
		t.Fatalf("retry delay at 59s = %v, want %v", got, fastRetryDelay)
	}
	// After that: double up to the cap.
	if got := nextReconnectDelay(fastRetryDelay, 61*time.Second); got != 2*fastRetryDelay {
		t.Fatalf("first slow delay = %v, want %v", got, 2*fastRetryDelay)
	}
	if got := nextReconnectDelay(24*time.Second, 5*time.Minute); got != reconnectMaxDelay {
		t.Fatalf("capped delay = %v, want %v", got, reconnectMaxDelay)
	}
}
