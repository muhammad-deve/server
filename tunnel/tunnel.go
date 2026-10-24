package tunnel

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/hashicorp/yamux"
)

type Config struct {
	Port             string
	Subdomain        string
	Reset            bool
	Region           string
	Type             string
	ShareToken       string
	ExpiresAt        time.Time
	DisableInspector bool
	ShareName        string
}

type registrationRequest struct {
	Type      string `json:"type"`
	Port      string `json:"port"`
	Subdomain string `json:"subdomain,omitempty"`
	Reset     bool   `json:"reset,omitempty"`
	Token     string `json:"token,omitempty"`
	// Resume asks for the subdomain held before the connection dropped. Servers
	// that predate it ignore the field and fall back to the account's latest
	// tunnel.
	Resume string `json:"resume,omitempty"`

	// About this install, so the server can tell which versions and systems
	// are in use. No paths, hostnames or local addresses are sent.
	Version string `json:"version,omitempty"`
	OS      string `json:"os,omitempty"`
	Arch    string `json:"arch,omitempty"`
	Install string `json:"install,omitempty"`
	// ReconnectReason is set on a reconnect: why the last connection ended,
	// as one of a few fixed words (never the raw error, which has IPs in it).
	ReconnectReason string `json:"reconnect_reason,omitempty"`
	ShareToken      string `json:"share_token,omitempty"`
	ExpiresAt       int64  `json:"expires_at,omitempty"`
}

type registrationResponse struct {
	Subdomain string `json:"subdomain"`
	URL       string `json:"url"`
	Error     string `json:"error,omitempty"`
}

// Terminal styling. SetupTerminal blanks these when the console can't show
// escape codes.
var (
	ansiReset  = "\x1b[0m"
	ansiBold   = "\x1b[1m"
	ansiGray   = "\x1b[90m"
	ansiGreen  = "\x1b[38;2;48;242;148m"
	ansiWhite  = "\x1b[97m"
	ansiYellow = "\x1b[33m"
	// ansiClear wipes the screen and scrollback and moves the cursor home.
	ansiClear = "\x1b[H\x1b[2J\x1b[3J"
)

// SetupTerminal turns on escape-code support in the Windows console. Where
// that isn't possible, output stays plain.
func SetupTerminal() {
	if !enableVirtualTerminal() {
		ansiReset, ansiBold, ansiGray, ansiGreen, ansiWhite, ansiYellow, ansiClear = "", "", "", "", "", "", ""
	}
}

// StartE runs a tunnel until the user stops it. It returns an error when the
// tunnel could not start or stopped for good.
func StartE(cfg Config) error {
	interrupt := make(chan os.Signal, 1)
	signal.Notify(interrupt, os.Interrupt, syscall.SIGTERM)
	defer signal.Stop(interrupt)
	if !cfg.ExpiresAt.IsZero() {
		timer := time.NewTimer(time.Until(cfg.ExpiresAt))
		defer timer.Stop()
		go func() {
			<-timer.C
			interrupt <- syscall.SIGTERM
		}()
	}

	// The first registration fails fast so a bad token, plan, or name is
	// reported straight away instead of retried.
	start := time.Now()
	conn, resp, err := register(cfg, "", "")
	if err != nil {
		return err
	}

	local := newLocalTarget(cfg.Port)
	var state *dashboardState
	var dashboardPort int
	if !cfg.DisableInspector {
		var dashErr error
		state, dashboardPort, dashErr = startDashboard(local)
		if dashErr != nil {
			fmt.Println("warning: could not start dashboard:", dashErr)
		}
	}
	latency := time.Since(start)
	if state != nil {
		state.setTunnel(tunnelInfo{
			URL:     resp.URL,
			Status:  "online",
			Region:  regionLabel(cfg.Region),
			Latency: latency.Milliseconds(),
			Started: time.Now(),
		})
	}

	startTerminalUI(cfg, resp, latency, dashboardPort)
	defer stopTerminalUI()

	// A dropped connection (network blip, laptop sleep, server restart) used
	// to end the CLI for good. Now the tunnel reconnects and keeps its URL.
	for {
		err := serve(conn, cfg, local, state, interrupt)
		if err == nil {
			return nil
		}
		updateTunnelStatus(state, "connecting", "", 0)
		logNotice(fmt.Sprintf("Connection lost: %s. Reconnecting...", errText(err)))

		conn, resp, err = reconnect(cfg, resp, disconnectReason(err), state, interrupt)
		if err != nil {
			if errors.Is(err, errInterrupted) {
				return nil
			}
			stopTerminalUI()
			if errors.Is(err, errStoppedFromDashboard) {
				fmt.Println("Tunnel stopped from the dashboard.")
				return nil
			}
			return fmt.Errorf("tunnel stopped: %w", err)
		}
	}
}

// badGatewayBody is what visitors see when the local app can't be reached.
// The reason (port, OS error) is printed in the CLI only.
const badGatewayBody = "The local app didn't respond. If this is your tunnel, check that your app is running and look at the GoPort terminal for details.\n"

func handleStream(stream net.Conn, local *localTarget, state *dashboardState) {
	defer stream.Close()
	// One bad request must not take the whole tunnel down.
	defer func() {
		if r := recover(); r != nil {
			logNotice(fmt.Sprintf("Internal error while handling a request: %v", r))
		}
	}()
	startTime := time.Now()

	bufStream := bufio.NewReader(stream)
	req, err := http.ReadRequest(bufStream)
	if err != nil {
		writeError(stream, http.StatusBadRequest, "Bad request.\n")
		return
	}
	defer req.Body.Close()

	reqHeaders := flattenHeaders(req.Header)
	method := req.Method
	pathStr := req.URL.RequestURI()
	isUpgradeRequest := isUpgrade(req)

	// Bodies stream through in both directions. The inspector keeps only the
	// first maxBodySize bytes of each, so streaming responses (SSE, long-poll)
	// reach the visitor right away and large bodies don't sit in RAM.
	var reqBody *bodyCapture
	if state != nil && !isUpgradeRequest && req.Body != nil && req.Body != http.NoBody {
		reqBody = newBodyCapture(maxBodySize)
		req.Body = captureBody(req.Body, reqBody)
	}

	localConn, localAddr, err := local.dial()
	if err != nil {
		// Keep a small body so the request can be replayed once the app is up.
		if reqBody != nil {
			_, _ = io.Copy(io.Discard, io.LimitReader(req.Body, maxBodySize+1))
		}
		local.reportFailure(err)
		logRequest(method, pathStr, http.StatusBadGateway, http.StatusText(http.StatusBadGateway))
		captureFailed(state, method, pathStr, reqHeaders, reqBody, fmt.Sprintf("failed to connect to %s: %v", localAddr, err), time.Since(startTime))
		writeError(stream, http.StatusBadGateway, badGatewayBody)
		return
	}
	defer localConn.Close()

	// Point the connection at the local server but keep the original public Host
	// header intact. Apps frequently build absolute URLs from the Host header
	// (Swagger/OpenAPI "servers", redirects, generated links). Rewriting it to
	// 127.0.0.1:<port> made those URLs point back at localhost, which breaks
	// Swagger "Try it out" and similar flows with CORS / "failed to fetch" errors.
	// ngrok preserves the Host by default, which is why it worked there.
	req.URL.Scheme = "http"
	req.URL.Host = localAddr
	req.RequestURI = ""

	// Upgrade headers are hop-by-hop for ordinary HTTP requests, but must remain
	// intact for WebSocket connections. Removing them before checking for an
	// upgrade turns a WebSocket request into a normal GET.
	if !isUpgradeRequest {
		stripHopByHop(req.Header)
	}

	// Detect Upgrade requests (e.g. WebSocket) and run a raw bidirectional copy.
	if isUpgradeRequest {
		if err := req.Write(localConn); err != nil {
			local.reportFailure(err)
			logRequest(method, pathStr, http.StatusBadGateway, http.StatusText(http.StatusBadGateway))
			writeError(stream, http.StatusBadGateway, badGatewayBody)
			return
		}
		local.reportSuccess()
		logRequest(method, pathStr, http.StatusSwitchingProtocols, "Upgrade")
		// Splice any data already buffered in bufStream first, then continue copying both ways.
		if n := bufStream.Buffered(); n > 0 {
			if buf, err := bufStream.Peek(n); err == nil {
				_, _ = localConn.Write(buf)
			}
		}
		bidirectionalCopy(stream, localConn)
		return
	}

	// Send the request while reading the response, so an app that answers
	// before reading the whole body (401, 413) is relayed instead of turning
	// into a 502 when it closes the connection.
	var body *watchedBody
	if req.Body != nil && req.Body != http.NoBody {
		body = &watchedBody{ReadCloser: req.Body}
		req.Body = body
	}
	writeErr := make(chan error, 1)
	go func() {
		err := req.Write(localConn)
		if err != nil && body.failed() {
			// The visitor went away mid-upload; the app won't answer half a request.
			localConn.Close()
		}
		writeErr <- err
	}()

	resp, err := http.ReadResponse(bufio.NewReader(localConn), req)
	if err != nil {
		select {
		case werr := <-writeErr:
			if werr != nil {
				err = werr
			}
		default:
		}
		local.reportFailure(err)
		logRequest(method, pathStr, http.StatusBadGateway, http.StatusText(http.StatusBadGateway))
		captureFailed(state, method, pathStr, reqHeaders, reqBody, fmt.Sprintf("failed to read response: %v", err), time.Since(startTime))
		writeError(stream, http.StatusBadGateway, badGatewayBody)
		return
	}
	defer resp.Body.Close()
	local.reportSuccess()
	logRequest(method, pathStr, resp.StatusCode, http.StatusText(resp.StatusCode))

	resHeaders := flattenHeaders(resp.Header)
	var resBody *bodyCapture
	if state != nil && resp.Body != http.NoBody {
		resBody = newBodyCapture(maxBodySize)
		resp.Body = captureBody(resp.Body, resBody)
	}

	stripHopByHop(resp.Header)
	resp.Close = true
	resp.Header.Set("Connection", "close")
	if err := resp.Write(stream); err != nil && !isClosedConnErr(err) {
		// A visitor closing the page mid-response is normal; anything else is worth a line.
		logNotice(fmt.Sprintf("Error sending a response for %s: %s", pathStr, errText(err)))
	}

	// The entry is added once the response has finished streaming.
	if state != nil {
		state.store.Add(&CapturedRequest{
			ID:           newRequestID(),
			Timestamp:    startTime,
			Method:       method,
			Path:         pathStr,
			StatusCode:   resp.StatusCode,
			Duration:     time.Since(startTime).Milliseconds(),
			ReqHeaders:   reqHeaders,
			ReqBody:      reqBody.Text(),
			ReqTruncated: reqBody.Truncated(),
			ResHeaders:   resHeaders,
			ResBody:      resBody.Text(),
		})
	}
}

func captureFailed(state *dashboardState, method, path string, reqHeaders map[string]string, reqBody *bodyCapture, message string, dur time.Duration) {
	if state == nil {
		return
	}
	state.store.Add(&CapturedRequest{
		ID:           newRequestID(),
		Timestamp:    time.Now().Add(-dur),
		Method:       method,
		Path:         path,
		StatusCode:   http.StatusBadGateway,
		Duration:     dur.Milliseconds(),
		ReqHeaders:   reqHeaders,
		ReqBody:      reqBody.Text(),
		ReqTruncated: reqBody.Truncated(),
		ResHeaders:   map[string]string{"Content-Type": "text/plain"},
		ResBody:      message,
	})
}

func writeError(w net.Conn, status int, body string) {
	fmt.Fprintf(w,
		"HTTP/1.1 %d %s\r\nContent-Type: text/plain; charset=utf-8\r\nContent-Length: %d\r\nConnection: close\r\n\r\n%s",
		status, http.StatusText(status), len(body), body,
	)
}

func isUpgrade(req *http.Request) bool {
	if !strings.EqualFold(req.Header.Get("Connection"), "upgrade") &&
		!headerContainsToken(req.Header.Get("Connection"), "upgrade") {
		return false
	}
	return req.Header.Get("Upgrade") != ""
}

func headerContainsToken(header, token string) bool {
	for _, part := range strings.Split(header, ",") {
		if strings.EqualFold(strings.TrimSpace(part), token) {
			return true
		}
	}
	return false
}

func bidirectionalCopy(a, b net.Conn) {
	done := make(chan struct{}, 2)
	go func() {
		_, _ = io.Copy(a, b)
		_ = closeWrite(a)
		done <- struct{}{}
	}()
	go func() {
		_, _ = io.Copy(b, a)
		_ = closeWrite(b)
		done <- struct{}{}
	}()
	<-done
}

func closeWrite(c net.Conn) error {
	if cw, ok := c.(interface{ CloseWrite() error }); ok {
		return cw.CloseWrite()
	}
	return nil
}

// hopByHopHeaders are headers that should not be forwarded by a proxy.
// See RFC 7230 section 6.1 and RFC 2616 section 13.5.1.
var hopByHopHeaders = []string{
	"Connection",
	"Proxy-Connection",
	"Keep-Alive",
	"Proxy-Authenticate",
	"Proxy-Authorization",
	"Te",
	"Trailer",
	"Transfer-Encoding",
	"Upgrade",
}

func stripHopByHop(h http.Header) {
	// Headers listed in Connection are also hop-by-hop.
	if connection := h.Get("Connection"); connection != "" {
		for _, name := range strings.Split(connection, ",") {
			name = strings.TrimSpace(name)
			if name != "" {
				h.Del(name)
			}
		}
	}
	for _, name := range hopByHopHeaders {
		h.Del(name)
	}
}

func isClosedConnErr(err error) bool {
	if err == nil {
		return false
	}
	// A visitor leaving mid-response resets the yamux stream.
	if errors.Is(err, yamux.ErrConnectionReset) || errors.Is(err, yamux.ErrStreamClosed) || errors.Is(err, yamux.ErrSessionShutdown) {
		return true
	}
	msg := err.Error()
	return strings.Contains(msg, "use of closed network connection") ||
		strings.Contains(msg, "broken pipe") ||
		strings.Contains(msg, "connection reset by peer") ||
		strings.Contains(msg, "forcibly closed") ||
		strings.Contains(msg, "EOF")
}

// pathColumnWidth is the fixed column width for the request path in terminal logs.
// Paths longer than this are truncated with an ellipsis so the status column
// stays vertically aligned, matching ngrok's behavior.
const pathColumnWidth = 50

func truncatePath(path string) string {
	if len(path) <= pathColumnWidth {
		return path
	}
	if pathColumnWidth <= 1 {
		return "…"
	}
	return path[:pathColumnWidth-1] + "…"
}

func logRequest(method, path string, statusCode int, statusText string) {
	if statusText == "" {
		statusText = "status"
	}
	line := terminalRequestLine{
		Time:       time.Now(),
		Method:     method,
		Path:       path,
		StatusCode: statusCode,
		StatusText: statusText,
	}

	terminalUIMu.RLock()
	ui := activeTerminalUI
	terminalUIMu.RUnlock()
	if ui != nil {
		ui.add(line)
		return
	}

	printRequestLine(line)
}

func regionLabel(region string) string {
	switch region {
	case "eu":
		return "Europe (eu)"
	case "":
		return "Europe (eu)"
	default:
		return region
	}
}

func commandLine(cfg Config) string {
	if cfg.Type == "share" {
		return "goport share " + cfg.ShareName
	}
	cmd := fmt.Sprintf("goport %s %s", cfg.Type, cfg.Port)
	if cfg.Reset {
		cmd += " --reset"
	}
	if cfg.Subdomain != "" {
		cmd += " --custom " + cfg.Subdomain
	}
	return cmd
}
