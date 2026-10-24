package tunnel

import (
	"bufio"
	"bytes"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"
)

func portOf(t *testing.T, server *httptest.Server) string {
	t.Helper()
	parsed, err := url.Parse(server.URL)
	if err != nil {
		t.Fatal(err)
	}
	return parsed.Port()
}

func newTestState() *dashboardState {
	return &dashboardState{store: newRequestStore(10)}
}

// visit sends req through handleStream as a visitor would and returns the
// response, read from the visitor's side of the stream.
func visit(t *testing.T, local *localTarget, state *dashboardState, req *http.Request) (*http.Response, net.Conn) {
	t.Helper()
	visitor, tunnelSide := net.Pipe()
	go handleStream(tunnelSide, local, state)
	_ = visitor.SetDeadline(time.Now().Add(5 * time.Second))
	go func() { _ = req.Write(visitor) }()
	resp, err := http.ReadResponse(bufio.NewReader(visitor), req)
	if err != nil {
		t.Fatal(err)
	}
	return resp, visitor
}

func waitForEntry(t *testing.T, store *requestStore) *CapturedRequest {
	t.Helper()
	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		if items := store.List(); len(items) > 0 {
			return items[0]
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatal("no request captured")
	return nil
}

// With the inspector on, every response used to be read to the end before
// the visitor got a byte, so server-sent events never arrived.
func TestStreamingResponseReachesVisitorWithInspectorOn(t *testing.T) {
	release := make(chan struct{})
	app := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		w.WriteHeader(http.StatusOK)
		fmt.Fprint(w, "data: first\n\n")
		w.(http.Flusher).Flush()
		<-release
		fmt.Fprint(w, "data: second\n\n")
	}))
	defer app.Close()
	defer close(release)

	state := newTestState()
	req, _ := http.NewRequest(http.MethodGet, "http://abc.goport.uz/events", nil)
	resp, visitor := visit(t, newLocalTarget(portOf(t, app)), state, req)
	defer visitor.Close()

	buf := make([]byte, 64)
	n, err := resp.Body.Read(buf)
	if err != nil && n == 0 {
		t.Fatalf("first event did not arrive while the stream was open: %v", err)
	}
	if got := string(buf[:n]); got != "data: first\n\n" {
		t.Fatalf("first read = %q", got)
	}
}

func TestLargeBodiesStreamAndInspectorKeepsOnlyTheStart(t *testing.T) {
	const size = maxBodySize + 50*1024
	app := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		received, _ := io.Copy(io.Discard, r.Body)
		w.Header().Set("X-Received", fmt.Sprint(received))
		_, _ = w.Write(bytes.Repeat([]byte("r"), size))
	}))
	defer app.Close()

	state := newTestState()
	req, _ := http.NewRequest(http.MethodPost, "http://abc.goport.uz/upload", bytes.NewReader(bytes.Repeat([]byte("q"), size)))
	resp, visitor := visit(t, newLocalTarget(portOf(t, app)), state, req)
	defer visitor.Close()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatal(err)
	}
	if len(body) != size || resp.Header.Get("X-Received") != fmt.Sprint(size) {
		t.Fatalf("visitor got %d bytes, app received %s; want %d both ways", len(body), resp.Header.Get("X-Received"), size)
	}

	entry := waitForEntry(t, state.store)
	if entry.ResBody != strings.Repeat("r", maxBodySize)+truncatedMarker {
		t.Fatalf("captured response has %d bytes", len(entry.ResBody))
	}
	if !entry.ReqTruncated || entry.ReqBody != strings.Repeat("q", maxBodySize)+truncatedMarker {
		t.Fatalf("captured request: truncated=%v, %d bytes", entry.ReqTruncated, len(entry.ReqBody))
	}
	if _, err := replayRequest(entry, newLocalTarget(portOf(t, app)), state.store); err == nil {
		t.Fatal("a truncated request body was replayed")
	}
}

// An app may answer before reading a large upload (401, 413) and close the
// connection; the visitor must get that answer, not a 502.
func TestEarlyResponseToLargeUploadIsRelayed(t *testing.T) {
	app := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "too large", http.StatusRequestEntityTooLarge)
	}))
	defer app.Close()

	for _, state := range []*dashboardState{nil, newTestState()} {
		req, _ := http.NewRequest(http.MethodPost, "http://abc.goport.uz/upload", bytes.NewReader(bytes.Repeat([]byte("u"), 8<<20)))
		resp, visitor := visit(t, newLocalTarget(portOf(t, app)), state, req)
		body, _ := io.ReadAll(resp.Body)
		visitor.Close()
		if resp.StatusCode != http.StatusRequestEntityTooLarge || !strings.Contains(string(body), "too large") {
			t.Fatalf("inspector=%v: got %d %q, want 413", state != nil, resp.StatusCode, body)
		}
	}
}

func TestBadGatewayHidesLocalDetails(t *testing.T) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	port := fmt.Sprint(ln.Addr().(*net.TCPAddr).Port)
	ln.Close()

	state := newTestState()
	req, _ := http.NewRequest(http.MethodPost, "http://abc.goport.uz/", strings.NewReader(`{"retry":true}`))
	resp, visitor := visit(t, newLocalTarget(port), state, req)
	defer visitor.Close()
	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusBadGateway {
		t.Fatalf("status = %d", resp.StatusCode)
	}
	if string(body) != badGatewayBody || strings.Contains(string(body), port) {
		t.Fatalf("502 body = %q", body)
	}
	// The inspector still records the reason, and the body so the request
	// can be replayed once the app is running.
	entry := waitForEntry(t, state.store)
	if !strings.Contains(entry.ResBody, port) {
		t.Fatalf("inspector entry lacks the reason: %q", entry.ResBody)
	}
	if entry.ReqTruncated || entry.ReqBody != `{"retry":true}` {
		t.Fatalf("request body = %q, truncated = %v", entry.ReqBody, entry.ReqTruncated)
	}
}

func TestBodyCaptureMarksPartialReads(t *testing.T) {
	c := newBodyCapture(8)
	body := captureBody(io.NopCloser(strings.NewReader("0123456789")), c)
	buf := make([]byte, 4)
	_, _ = body.Read(buf)
	if !c.Truncated() || c.Text() != "0123"+truncatedMarker {
		t.Fatalf("partial read: truncated=%v text=%q", c.Truncated(), c.Text())
	}
	_, _ = io.ReadAll(body)
	if !c.Truncated() || c.Text() != "01234567"+truncatedMarker {
		t.Fatalf("long body: truncated=%v text=%q", c.Truncated(), c.Text())
	}
	short := newBodyCapture(8)
	_, _ = io.ReadAll(captureBody(io.NopCloser(strings.NewReader("abc")), short))
	if short.Truncated() || short.Text() != "abc" {
		t.Fatalf("short body: truncated=%v text=%q", short.Truncated(), short.Text())
	}
}

func TestDevServerOnIPv6LoopbackOnly(t *testing.T) {
	ln, err := net.Listen("tcp", "[::1]:0")
	if err != nil {
		t.Skip("IPv6 loopback not available:", err)
	}
	app := &http.Server{Handler: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = io.WriteString(w, "from ::1")
	})}
	go func() { _ = app.Serve(ln) }()
	defer app.Close()

	local := newLocalTarget(fmt.Sprint(ln.Addr().(*net.TCPAddr).Port))
	req, _ := http.NewRequest(http.MethodGet, "http://abc.goport.uz/", nil)
	resp, visitor := visit(t, local, nil, req)
	defer visitor.Close()
	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusOK || string(body) != "from ::1" {
		t.Fatalf("got %d %q", resp.StatusCode, body)
	}
	if !strings.HasPrefix(local.addr(), "[::1]:") {
		t.Fatalf("the working address was not remembered: %s", local.addr())
	}
}

// A panic while handling one request must not end the process.
func TestHandleStreamRecoversFromPanic(t *testing.T) {
	app := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = io.WriteString(w, "ok")
	}))
	defer app.Close()

	visitor, tunnelSide := net.Pipe()
	defer visitor.Close()
	done := make(chan struct{})
	go func() {
		defer close(done)
		// A state without a store makes the capture step panic.
		handleStream(tunnelSide, newLocalTarget(portOf(t, app)), &dashboardState{})
	}()
	_ = visitor.SetDeadline(time.Now().Add(5 * time.Second))
	req, _ := http.NewRequest(http.MethodGet, "http://abc.goport.uz/", nil)
	go func() { _ = req.Write(visitor) }()
	if resp, err := http.ReadResponse(bufio.NewReader(visitor), req); err == nil {
		_, _ = io.Copy(io.Discard, resp.Body)
	}
	select {
	case <-done:
	case <-time.After(5 * time.Second):
		t.Fatal("handleStream did not return")
	}
}
