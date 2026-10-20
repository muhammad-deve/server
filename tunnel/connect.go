package tunnel

import (
	"bufio"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math/rand/v2"
	"net"
	"os"
	"runtime"
	"strings"
	"time"

	"github.com/hashicorp/yamux"
	"github.com/muhammad-deve/server/cmd/config"
)

const (
	dialTimeout       = 10 * time.Second
	handshakeTimeout  = 15 * time.Second
	reconnectMinDelay = 1 * time.Second
	reconnectMaxDelay = 30 * time.Second
	// For the first minute after a drop the CLI retries every 1.5-3s, so a
	// tunnel is back within seconds of a server restart (every deploy).
	fastRetryDelay  = 3 * time.Second
	fastRetryWindow = time.Minute
)

var errInterrupted = errors.New("interrupted")

// registrationError is the server refusing the tunnel, as opposed to the
// network failing on the way.
type registrationError struct{ msg string }

func (e *registrationError) Error() string { return e.msg }

// permanentRegistrationErrors are refusals that retrying cannot fix: the token,
// the plan, or the requested name is wrong. Anything else (a subdomain still
// held by the dropped session, a database hiccup) clears up on its own.
var permanentRegistrationErrors = []string{
	"authentication required",
	"available on pro",
	"traffic limit",
	"outside your plan allowance",
	"cannot be used together",
	"subdomain must",
	"subdomain cannot",
	"subdomain can only",
	"is reserved",
	"missing port",
}

func (e *registrationError) permanent() bool {
	msg := strings.ToLower(e.msg)
	for _, fragment := range permanentRegistrationErrors {
		if strings.Contains(msg, fragment) {
			return true
		}
	}
	return false
}

// sessionConfig tolerates slow uplinks. yamux sends keepalive pings on the same
// TCP connection as data, so with the default 10s timeout a ping queued behind
// a large response on a slow link failed and the whole tunnel dropped with
// "keepalive timeout".
func sessionConfig() *yamux.Config {
	cfg := yamux.DefaultConfig()
	cfg.KeepAliveInterval = 20 * time.Second
	cfg.ConnectionWriteTimeout = 30 * time.Second
	// The CLI reports a lost connection itself; yamux's raw "[ERR] yamux: ..."
	// lines would land in the middle of the request log.
	cfg.LogOutput = io.Discard
	return cfg
}

// register dials the server and claims a tunnel. resume is the subdomain held
// before a dropped connection; it is empty on the first start.
// reconnectReason says why the previous connection ended; it is empty on
// the first start.
func register(cfg Config, resume, reconnectReason string) (net.Conn, registrationResponse, error) {
	conn, err := net.DialTimeout("tcp", config.ServerAddr, dialTimeout)
	if err != nil {
		return nil, registrationResponse{}, fmt.Errorf("error connecting to server: %w", err)
	}
	_ = conn.SetDeadline(time.Now().Add(handshakeTimeout))

	req := registrationRequest{
		Type:      cfg.Type,
		Port:      cfg.Port,
		Subdomain: cfg.Subdomain,
		// --reset applies to the first start only. Sending it on a reconnect
		// would hand out a new random URL every time the link drops.
		Reset:  cfg.Reset && resume == "",
		Token:  config.Token,
		Resume: resume,

		Version:         Version,
		OS:              runtime.GOOS,
		Arch:            runtime.GOARCH,
		Install:         installMethod(),
		ReconnectReason: reconnectReason,
		ShareToken:      cfg.ShareToken,
	}
	if !cfg.ExpiresAt.IsZero() {
		req.ExpiresAt = cfg.ExpiresAt.Unix()
	}
	if err := json.NewEncoder(conn).Encode(req); err != nil {
		conn.Close()
		return nil, registrationResponse{}, fmt.Errorf("error sending tunnel request: %w", err)
	}

	// The server answers with one JSON line (json.Encoder appends '\n'), and
	// yamux frames may follow in the same read. Read exactly that line and hand
	// whatever is buffered after it to yamux; a json.Decoder would either drop
	// those bytes or leave the newline in front of the first frame.
	reader := bufio.NewReader(conn)
	line, err := reader.ReadSlice('\n')
	if err != nil {
		conn.Close()
		return nil, registrationResponse{}, fmt.Errorf("error reading tunnel response: %w", err)
	}
	var resp registrationResponse
	if err := json.Unmarshal(line, &resp); err != nil {
		conn.Close()
		return nil, registrationResponse{}, fmt.Errorf("error reading tunnel response: %w", err)
	}
	if resp.Error != "" {
		conn.Close()
		return nil, registrationResponse{}, &registrationError{msg: resp.Error}
	}
	if resp.URL == "" {
		conn.Close()
		return nil, registrationResponse{}, errors.New("server error: empty tunnel url")
	}
	_ = conn.SetDeadline(time.Time{})

	return &bufferedConn{Conn: conn, r: reader}, resp, nil
}

// bufferedConn reads through the handshake's reader, so bytes it already
// buffered past the response reach yamux before anything else.
type bufferedConn struct {
	net.Conn
	r *bufio.Reader
}

func (c *bufferedConn) Read(p []byte) (int, error) { return c.r.Read(p) }

// serve runs one tunnel session until it ends. It returns nil when the user
// stopped the CLI, and the reason the session died otherwise.
func serve(conn net.Conn, cfg Config, state *dashboardState, interrupt <-chan os.Signal) error {
	defer conn.Close()

	// Count every byte flowing through the tunnel (both directions, including
	// framing) for the dashboard.
	tunnelConn := conn
	if state != nil {
		tunnelConn = newCountingConn(conn, state.store.bytesCounter())
	}

	session, err := yamux.Client(tunnelConn, sessionConfig())
	if err != nil {
		return fmt.Errorf("error starting tunnel session: %w", err)
	}
	defer session.Close()

	done := make(chan error, 1)
	go func() {
		for {
			stream, err := session.Accept()
			if err != nil {
				done <- err
				return
			}
			go handleStream(stream, cfg.Port, state)
		}
	}()

	select {
	case <-interrupt:
		return nil
	case err := <-done:
		if err == nil {
			err = errors.New("session closed")
		}
		return err
	}
}

// reconnect keeps trying to restore the tunnel, asking for the same subdomain,
// until it succeeds, the server refuses for good, or the user presses Ctrl+C.
// It never gives up on network errors: a laptop may sleep or lose Wi-Fi for
// hours and should come back on its own.
func reconnect(cfg Config, previous registrationResponse, reason string, state *dashboardState, interrupt <-chan os.Signal) (net.Conn, registrationResponse, error) {
	delay := reconnectMinDelay
	lastErr := ""
	lostAt := time.Now()
	for attempt := 1; ; attempt++ {
		select {
		case <-interrupt:
			return nil, registrationResponse{}, errInterrupted
		case <-time.After(withJitter(delay)):
		}

		start := time.Now()
		conn, resp, err := register(cfg, previous.Subdomain, reason)
		if err == nil {
			if resp.URL != previous.URL {
				logNotice("Reconnected with a new public URL: " + resp.URL)
			} else {
				logNotice("Reconnected.")
			}
			updateTunnelStatus(state, "online", resp.URL, time.Since(start))
			return conn, resp, nil
		}

		var refused *registrationError
		if errors.As(err, &refused) && refused.permanent() {
			return nil, registrationResponse{}, err
		}

		// Say it once per distinct error so an hour offline doesn't fill the
		// terminal with identical lines.
		if msg := errText(err); msg != lastErr {
			lastErr = msg
			logNotice(fmt.Sprintf("Reconnect failed: %s. Still retrying...", msg))
		}
		delay = nextReconnectDelay(delay, time.Since(lostAt))
	}
}

// nextReconnectDelay keeps retries quick for the first minute after a drop,
// then doubles the wait up to reconnectMaxDelay so a long outage (offline
// laptop) costs almost nothing.
func nextReconnectDelay(delay, sinceLost time.Duration) time.Duration {
	if sinceLost < fastRetryWindow {
		return fastRetryDelay
	}
	return min(max(delay, fastRetryDelay)*2, reconnectMaxDelay)
}

// errText drops the trailing period Windows puts on socket errors, so notices
// don't end up with "host.. Reconnecting".
func errText(err error) string {
	return strings.TrimRight(err.Error(), ". ")
}

// withJitter spreads retries so every CLI doesn't hit the server at the same
// instant after it restarts.
func withJitter(d time.Duration) time.Duration {
	half := d / 2
	return half + rand.N(half+1)
}

func updateTunnelStatus(state *dashboardState, status, url string, latency time.Duration) {
	if state == nil {
		return
	}
	info := state.getTunnel()
	info.Status = status
	if url != "" {
		info.URL = url
	}
	if latency > 0 {
		info.Latency = latency.Milliseconds()
	}
	state.setTunnel(info)
}
