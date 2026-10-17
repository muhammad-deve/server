package tunnel

import (
	"errors"
	"fmt"
	"io"
	"testing"

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
