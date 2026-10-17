package tunnel

import (
	"bufio"
	"bytes"
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
)

type Config struct {
	Port      string
	Subdomain string
	Reset     bool
	Region    string
	Type      string
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
}

type registrationResponse struct {
	Subdomain string `json:"subdomain"`
	URL       string `json:"url"`
	Error     string `json:"error,omitempty"`
}

const (
	ansiReset  = "\x1b[0m"
	ansiBold   = "\x1b[1m"
	ansiGray   = "\x1b[90m"
	ansiGreen  = "\x1b[38;2;48;242;148m"
	ansiWhite  = "\x1b[97m"
	ansiYellow = "\x1b[33m"
)

func Start(cfg Config) {
	interrupt := make(chan os.Signal, 1)
	signal.Notify(interrupt, os.Interrupt, syscall.SIGTERM)
	defer signal.Stop(interrupt)

	// The first registration fails fast so a bad token, plan, or name is
	// reported straight away instead of retried.
	start := time.Now()
	conn, resp, err := register(cfg, "", "")
	if err != nil {
		fmt.Println(err)
		return
	}

	state, dashboardPort, dashErr := startDashboard(cfg.Port)
	if dashErr != nil {
		fmt.Println("warning: could not start dashboard:", dashErr)
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
		err := serve(conn, cfg, state, interrupt)
		if err == nil {
			return
		}
		updateTunnelStatus(state, "connecting", "", 0)
		logNotice(fmt.Sprintf("Connection lost: %s. Reconnecting...", errText(err)))

		conn, resp, err = reconnect(cfg, resp, disconnectReason(err), state, interrupt)
		if err != nil {
			if errors.Is(err, errInterrupted) {
				return
			}
			stopTerminalUI()
			fmt.Println("tunnel stopped:", err)
			return
		}
	}
}

func handleStream(stream net.Conn, port string, state *dashboardState) {
	defer stream.Close()
	startTime := time.Now()

	bufStream := bufio.NewReader(stream)
	req, err := http.ReadRequest(bufStream)
	if err != nil {
		writeError(stream, http.StatusBadRequest, fmt.Sprintf("failed to read request: %v", err))
		return
	}
	defer req.Body.Close()

	// Buffer the request body so we can both forward it and capture it for the dashboard.
	// Read the full body so uploads aren't truncated; we only truncate the stored copy.
	var reqBodyBytes []byte
	if req.Body != nil && req.Body != http.NoBody {
		reqBodyBytes, _ = io.ReadAll(req.Body)
	}
	reqHeaders := flattenHeaders(req.Header)
	method := req.Method
	pathStr := req.URL.RequestURI()
	isUpgradeRequest := isUpgrade(req)

	localAddr := "127.0.0.1:" + port
	localConn, err := net.Dial("tcp", localAddr)
	if err != nil {
		logRequest(method, pathStr, http.StatusBadGateway, http.StatusText(http.StatusBadGateway))
		captureFailed(state, method, pathStr, reqHeaders, reqBodyBytes, http.StatusBadGateway, fmt.Sprintf("failed to connect to %s: %v", localAddr, err), time.Since(startTime))
		writeError(stream, http.StatusBadGateway, fmt.Sprintf("failed to connect to %s: %v", localAddr, err))
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

	// Restore the body we buffered so req.Write can resend it.
	if reqBodyBytes != nil {
		req.Body = io.NopCloser(bytes.NewReader(reqBodyBytes))
		req.ContentLength = int64(len(reqBodyBytes))
	}

	// Detect Upgrade requests (e.g. WebSocket) and run a raw bidirectional copy.
	if isUpgradeRequest {
		if err := req.Write(localConn); err != nil {
			logRequest(method, pathStr, http.StatusBadGateway, http.StatusText(http.StatusBadGateway))
			writeError(stream, http.StatusBadGateway, fmt.Sprintf("failed to forward upgrade request: %v", err))
			return
		}
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

	if err := req.Write(localConn); err != nil {
		logRequest(method, pathStr, http.StatusBadGateway, http.StatusText(http.StatusBadGateway))
		captureFailed(state, method, pathStr, reqHeaders, reqBodyBytes, http.StatusBadGateway, fmt.Sprintf("failed to forward: %v", err), time.Since(startTime))
		writeError(stream, http.StatusBadGateway, fmt.Sprintf("failed to forward request: %v", err))
		return
	}

	resp, err := http.ReadResponse(bufio.NewReader(localConn), req)
	if err != nil {
		logRequest(method, pathStr, http.StatusBadGateway, http.StatusText(http.StatusBadGateway))
		captureFailed(state, method, pathStr, reqHeaders, reqBodyBytes, http.StatusBadGateway, fmt.Sprintf("failed to read response: %v", err), time.Since(startTime))
		writeError(stream, http.StatusBadGateway, fmt.Sprintf("failed to read local response: %v", err))
		return
	}
	defer resp.Body.Close()

	// Buffer the response body so we can both forward it and capture it.
	// Read the full body so the browser receives intact assets; truncate only the stored copy.
	respBodyBytes, _ := io.ReadAll(resp.Body)
	logRequest(method, pathStr, resp.StatusCode, http.StatusText(resp.StatusCode))

	if state != nil {
		state.store.Add(&CapturedRequest{
			ID:         newRequestID(),
			Timestamp:  startTime,
			Method:     method,
			Path:       pathStr,
			StatusCode: resp.StatusCode,
			Duration:   time.Since(startTime).Milliseconds(),
			ReqHeaders: reqHeaders,
			ReqBody:    truncateBody(reqBodyBytes),
			ResHeaders: flattenHeaders(resp.Header),
			ResBody:    truncateBody(respBodyBytes),
		})
	}

	stripHopByHop(resp.Header)
	resp.Close = true
	resp.Header.Set("Connection", "close")
	resp.Body = io.NopCloser(bytes.NewReader(respBodyBytes))
	resp.ContentLength = int64(len(respBodyBytes))

	if err := resp.Write(stream); err != nil {
		// Don't print on broken pipe / closed connection — these are normal when the client disconnects.
		if !isClosedConnErr(err) {
			fmt.Println("error writing tunnel response:", err)
		}
	}
}

func captureFailed(state *dashboardState, method, path string, reqHeaders map[string]string, reqBody []byte, statusCode int, message string, dur time.Duration) {
	if state == nil {
		return
	}
	state.store.Add(&CapturedRequest{
		ID:         newRequestID(),
		Timestamp:  time.Now().Add(-dur),
		Method:     method,
		Path:       path,
		StatusCode: statusCode,
		Duration:   dur.Milliseconds(),
		ReqHeaders: reqHeaders,
		ReqBody:    truncateBody(reqBody),
		ResHeaders: map[string]string{"Content-Type": "text/plain"},
		ResBody:    message,
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
	msg := err.Error()
	return strings.Contains(msg, "use of closed network connection") ||
		strings.Contains(msg, "broken pipe") ||
		strings.Contains(msg, "connection reset by peer") ||
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

func printDashboard(cfg Config, resp registrationResponse, latency time.Duration, dashboardPort int) {
	dashboardAddr := fmt.Sprintf("http://127.0.0.1:%d", dashboardPort)
	// Clear the entire terminal (visible + scrollback) and reprint the command at
	// the top so the dashboard sits cleanly without any prior shell output above it.
	fmt.Print("\033[H\033[2J\033[3J")
	fmt.Printf("%s$%s %s%s%s\n", ansiGray, ansiReset, ansiBold+ansiWhite, commandLine(cfg), ansiReset)
	fmt.Println()
	fmt.Printf("%s%-16s%s %s%s%s\n", ansiGray, "Dashboard", ansiReset, ansiBold+ansiWhite, dashboardAddr, ansiReset)
	fmt.Printf("%s%-16s%s %s%s%s\n", ansiGray, "Region", ansiReset, ansiBold+ansiWhite, regionLabel(cfg.Region), ansiReset)
	fmt.Printf("%s%-16s%s %sonline%s %s(%dms)%s\n", ansiGray, "Status", ansiReset, ansiGreen, ansiReset, ansiGray, latency.Milliseconds(), ansiReset)
	fmt.Printf("%s%-16s%s %s%s%s %s→%s %s%slocalhost:%s%s\n", ansiGray, "Forwarding", ansiReset, ansiGreen, resp.URL, ansiReset, ansiGray, ansiReset, ansiBold, ansiWhite, cfg.Port, ansiReset)
	fmt.Println()

	if cfg.Type == "http" {
		fmt.Printf("%s%sHTTP Requests%s\n", ansiBold, ansiWhite, ansiReset)
		fmt.Printf("%s-------------%s\n", ansiGray, ansiReset)
		fmt.Println()
	}
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
	cmd := fmt.Sprintf("goport %s %s", cfg.Type, cfg.Port)
	if cfg.Reset {
		cmd += " --reset"
	}
	if cfg.Subdomain != "" {
		cmd += " --custom " + cfg.Subdomain
	}
	return cmd
}
