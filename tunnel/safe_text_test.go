package tunnel

import (
	"strings"
	"testing"
	"time"
)

func TestTerminalSafe(t *testing.T) {
	cases := map[string]string{
		"/api/users?page=2":            "/api/users?page=2",
		"/café/日本":                     "/café/日本",
		"/x\x1b]0;pwned\x07":           `/x\x1b]0;pwned\x07`,
		"/a\r\nb\tc":                   `/a\x0d\x0ab\x09c`,
		"del\x7f":                      `del\x7f`,
		"c1\u009b31m":                  `c1\u009b31m`,
		"bad\xffutf8":                  `bad\xffutf8`,
		"https://abc.goport.uz\x1b[2J": `https://abc.goport.uz\x1b[2J`,
	}
	for in, want := range cases {
		if got := TerminalSafe(in); got != want {
			t.Errorf("TerminalSafe(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestTerminalOutputEscapesUntrustedText(t *testing.T) {
	var out strings.Builder
	writeRequestLine(&out, terminalRequestLine{Time: time.Now(), Method: "GET", Path: "/\x1b]52;c;cm0gLXJmIH4=\x07", StatusCode: 200, StatusText: "OK"})
	board := buildTerminalDashboard(Config{Type: "http", Port: "3000"}, registrationResponse{URL: "https://x.goport.uz\x1b]0;title\x07"}, time.Millisecond, 4040)
	for _, text := range []string{out.String(), board} {
		if strings.Contains(text, "\x07") || strings.Contains(text, "\x1b]") {
			t.Fatalf("raw escape sequence printed: %q", text)
		}
	}
}

func TestDashboardLineWhenInspectorIsOff(t *testing.T) {
	board := buildTerminalDashboard(Config{Type: "http", Port: "3000"}, registrationResponse{URL: "https://x.goport.uz"}, time.Millisecond, 0)
	if strings.Contains(board, "127.0.0.1:0") {
		t.Fatalf("inspector address shown while it is off: %q", board)
	}
}
