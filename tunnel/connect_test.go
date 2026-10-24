package tunnel

import (
	"encoding/json"
	"errors"
	"io"
	"net"
	"os"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/muhammad-deve/server/cmd/config"
)

func TestRegistrationErrorPermanent(t *testing.T) {
	cases := map[string]bool{
		"failed to register tunnel: authentication required: run `goport auth <token>`":                         true,
		"failed to register tunnel: pro required: custom subdomains are available on Pro; upgrade at https://x": true,
		"failed to register tunnel: monthly traffic limit reached for Free (5 GB); upgrade to Pro for 70 GB":    true,
		"failed to register tunnel: pro required: this device token is outside your plan allowance":             true,
		`failed to register tunnel: subdomain "api" is reserved`:                                                true,
		`failed to register tunnel: subdomain "abc123" is currently in use`:                                     false,
		`subdomain "abc123" is already connected`:                                                               false,
		"failed to register tunnel: could not verify your subscription or monthly usage":                        false,
		"your plan allows up to 1 active tunnels; stop one or upgrade at https://goport.uz/dashboard#billing":   false,
		"failed to register tunnel: subdomain cannot be used: it belongs to another account":                    true,
		"failed to register tunnel: subdomain cannot be resumed: the tunnel was stopped from the dashboard":     true,
		"failed to register tunnel: too many tunnel registrations, try again in a minute":                       false,
	}
	for msg, want := range cases {
		if got := (&registrationError{msg: msg}).permanent(); got != want {
			t.Errorf("permanent(%q) = %v, want %v", msg, got, want)
		}
	}
	if !(&registrationError{msg: "too many tunnel registrations, try again in a minute"}).rateLimited() {
		t.Error("rate limit refusal not recognised")
	}
}

func TestReconnectEndsWhenStoppedFromTheDashboard(t *testing.T) {
	startFakeServer(t, `{"error":"failed to register tunnel: subdomain cannot be resumed: the tunnel was stopped from the dashboard"}`+"\n")

	_, _, err := reconnect(Config{Port: "3000", Type: "http"}, registrationResponse{Subdomain: "abc123"}, "", nil, make(chan os.Signal))
	if !errors.Is(err, errStoppedFromDashboard) {
		t.Fatalf("reconnect() error = %v, want errStoppedFromDashboard", err)
	}
}

func TestRegisterEscapesServerText(t *testing.T) {
	startFakeServer(t, `{"error":"bad \u001b]0;owned\u0007 request"}`+"\n")

	_, _, err := register(Config{Port: "3000", Type: "http"}, "", "")
	if err == nil || strings.ContainsAny(err.Error(), "\x1b\x07") {
		t.Fatalf("register() error = %q, want escaped text", err)
	}
}

// fakeServer answers each registration with the next reply and records the
// requests it received.
type fakeServer struct {
	ln       net.Listener
	requests chan registrationRequest
}

func startFakeServer(t *testing.T, replies ...string) *fakeServer {
	t.Helper()
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	s := &fakeServer{ln: ln, requests: make(chan registrationRequest, len(replies))}
	accepted := make(chan net.Conn, len(replies))
	go func() {
		for _, reply := range replies {
			conn, err := ln.Accept()
			if err != nil {
				return
			}
			// Connections stay open until the test ends, like a live session.
			accepted <- conn
			var req registrationRequest
			if err := json.NewDecoder(conn).Decode(&req); err == nil {
				s.requests <- req
				_, _ = io.WriteString(conn, reply)
			}
		}
	}()

	previous := config.ServerAddr
	config.ServerAddr = ln.Addr().String()
	t.Cleanup(func() {
		config.ServerAddr = previous
		ln.Close()
		for {
			select {
			case conn := <-accepted:
				conn.Close()
			default:
				return
			}
		}
	})
	return s
}

func TestRegisterKeepsBytesSentRightAfterResponse(t *testing.T) {
	// The response and the start of the first yamux frame arrive in one read.
	startFakeServer(t, `{"subdomain":"abc123","url":"https://abc123.goport.uz"}`+"\nYAMUX")

	conn, resp, err := register(Config{Port: "3000", Type: "http"}, "", "")
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()
	if resp.Subdomain != "abc123" {
		t.Fatalf("subdomain = %q", resp.Subdomain)
	}

	// yamux must see exactly the bytes after the response line: not the
	// newline that ends it (that broke every session), and nothing dropped.
	_ = conn.SetReadDeadline(time.Now().Add(2 * time.Second))
	got := make([]byte, 5)
	if _, err := io.ReadFull(conn, got); err != nil {
		t.Fatal(err)
	}
	if string(got) != "YAMUX" {
		t.Fatalf("bytes after the response = %q, want %q", got, "YAMUX")
	}
}

func TestReconnectAsksForTheSameSubdomain(t *testing.T) {
	server := startFakeServer(t,
		`{"error":"failed to register tunnel: subdomain \"abc123\" is currently in use"}`+"\n",
		`{"subdomain":"abc123","url":"https://abc123.goport.uz"}`+"\n",
	)

	cfg := Config{Port: "3000", Type: "http", Reset: true}
	previous := registrationResponse{Subdomain: "abc123", URL: "https://abc123.goport.uz"}
	conn, resp, err := reconnect(cfg, previous, "server_closed", nil, make(chan os.Signal))
	if err != nil {
		t.Fatalf("reconnect() error = %v", err)
	}
	defer conn.Close()
	if resp.URL != previous.URL {
		t.Fatalf("URL = %q, want %q", resp.URL, previous.URL)
	}

	for attempt := 1; attempt <= 2; attempt++ {
		req := <-server.requests
		if req.Resume != "abc123" {
			t.Errorf("attempt %d: resume = %q, want abc123", attempt, req.Resume)
		}
		if req.Reset {
			t.Errorf("attempt %d: --reset must not be resent on a reconnect", attempt)
		}
		if req.ReconnectReason != "server_closed" {
			t.Errorf("attempt %d: reconnect_reason = %q, want server_closed", attempt, req.ReconnectReason)
		}
		if req.Version != Version || req.OS != runtime.GOOS || req.Arch != runtime.GOARCH || req.Install == "" {
			t.Errorf("attempt %d: client details = %q %q %q %q", attempt, req.Version, req.OS, req.Arch, req.Install)
		}
	}
}

func TestReconnectStopsWhenTheServerRefusesForGood(t *testing.T) {
	startFakeServer(t, `{"error":"failed to register tunnel: authentication required: run goport auth"}`+"\n")

	_, _, err := reconnect(Config{Port: "3000", Type: "http"}, registrationResponse{Subdomain: "abc123"}, "", nil, make(chan os.Signal))
	var refused *registrationError
	if !errors.As(err, &refused) {
		t.Fatalf("reconnect() error = %v, want a registration error", err)
	}
}

func TestReconnectStopsOnInterrupt(t *testing.T) {
	interrupt := make(chan os.Signal, 1)
	interrupt <- os.Interrupt

	_, _, err := reconnect(Config{Port: "3000", Type: "http"}, registrationResponse{Subdomain: "abc123"}, "", nil, interrupt)
	if !errors.Is(err, errInterrupted) {
		t.Fatalf("reconnect() error = %v, want errInterrupted", err)
	}
}
