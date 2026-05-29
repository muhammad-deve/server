package tunnel

import (
	"bytes"
	"embed"
	"encoding/json"
	"fmt"
	"io"
	"io/fs"
	"net"
	"net/http"
	"path"
	"strconv"
	"strings"
	"sync"
	"time"
)

const dashboardStartPort = 4040

//go:embed all:dashboard_dist
var dashboardFS embed.FS

// dashboardState holds runtime data exposed to the embedded UI.
type dashboardState struct {
	mu        sync.RWMutex
	tunnel    tunnelInfo
	store     *requestStore
	localPort string
}

type tunnelInfo struct {
	URL     string    `json:"url"`
	Status  string    `json:"status"`
	Region  string    `json:"region"`
	Latency int64     `json:"latency"` // ms
	Started time.Time `json:"started"`
}

func (d *dashboardState) setTunnel(t tunnelInfo) {
	d.mu.Lock()
	d.tunnel = t
	d.mu.Unlock()
}

func (d *dashboardState) getTunnel() tunnelInfo {
	d.mu.RLock()
	defer d.mu.RUnlock()
	return d.tunnel
}

// findAvailablePort tries ports starting from dashboardStartPort until it finds
// one that's not in use. Returns the listener bound to that port.
func findAvailablePort() (net.Listener, int, error) {
	for port := dashboardStartPort; port < dashboardStartPort+100; port++ {
		addr := "127.0.0.1:" + strconv.Itoa(port)
		ln, err := net.Listen("tcp", addr)
		if err == nil {
			return ln, port, nil
		}
	}
	return nil, 0, fmt.Errorf("no available port found in range %d-%d", dashboardStartPort, dashboardStartPort+99)
}

// startDashboard binds the dashboard HTTP server on the first available port
// starting from 4040 and returns the port plus a state handle the tunnel can
// update with live data.
func startDashboard(localPort string) (*dashboardState, int, error) {
	ln, port, err := findAvailablePort()
	if err != nil {
		return nil, 0, err
	}

	state := &dashboardState{
		store:     newRequestStore(500),
		localPort: localPort,
	}

	mux := http.NewServeMux()
	registerAPIRoutes(mux, state)
	registerStaticRoutes(mux)

	go func() {
		_ = http.Serve(ln, mux)
	}()

	return state, port, nil
}

func registerAPIRoutes(mux *http.ServeMux, state *dashboardState) {
	mux.HandleFunc("/api/tunnel", func(w http.ResponseWriter, r *http.Request) {
		t := state.getTunnel()
		writeJSON(w, http.StatusOK, map[string]any{
			"url":           t.URL,
			"status":        t.Status,
			"region":        t.Region,
			"latency":       t.Latency,
			"requestsToday": state.store.Total(),
			"totalBytes":    state.store.TotalBytes(),
			"started":       t.Started,
		})
	})

	mux.HandleFunc("/api/requests", func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodGet:
			writeJSON(w, http.StatusOK, state.store.List())
		case http.MethodDelete:
			state.store.Clear()
			writeJSON(w, http.StatusOK, map[string]any{"ok": true})
		default:
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		}
	})

	mux.HandleFunc("/api/requests/", func(w http.ResponseWriter, r *http.Request) {
		// Path: /api/requests/{id} or /api/requests/{id}/replay
		trimmed := strings.TrimPrefix(r.URL.Path, "/api/requests/")
		parts := strings.SplitN(trimmed, "/", 2)
		if len(parts) == 0 || parts[0] == "" {
			http.Error(w, "missing request id", http.StatusBadRequest)
			return
		}
		id := parts[0]
		captured, ok := state.store.Get(id)
		if !ok {
			http.Error(w, "request not found", http.StatusNotFound)
			return
		}
		if len(parts) == 2 && parts[1] == "replay" {
			if r.Method != http.MethodPost {
				http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
				return
			}
			replay, err := replayRequest(captured, state.localPort, state.store)
			if err != nil {
				http.Error(w, err.Error(), http.StatusBadGateway)
				return
			}
			writeJSON(w, http.StatusOK, replay)
			return
		}
		writeJSON(w, http.StatusOK, captured)
	})

	mux.HandleFunc("/api/events", func(w http.ResponseWriter, r *http.Request) {
		flusher, ok := w.(http.Flusher)
		if !ok {
			http.Error(w, "streaming not supported", http.StatusInternalServerError)
			return
		}
		w.Header().Set("Content-Type", "text/event-stream")
		w.Header().Set("Cache-Control", "no-cache")
		w.Header().Set("Connection", "keep-alive")
		w.Header().Set("Access-Control-Allow-Origin", "*")

		ch := state.store.Subscribe()
		defer state.store.Unsubscribe(ch)

		// Initial ping to establish the stream.
		fmt.Fprintf(w, ": connected\n\n")
		flusher.Flush()

		ctx := r.Context()
		ticker := time.NewTicker(15 * time.Second)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case req, ok := <-ch:
				if !ok {
					return
				}
				data, _ := json.Marshal(req)
				fmt.Fprintf(w, "event: request\ndata: %s\n\n", data)
				flusher.Flush()
			case <-ticker.C:
				fmt.Fprintf(w, ": ping\n\n")
				flusher.Flush()
			}
		}
	})
}

func registerStaticRoutes(mux *http.ServeMux) {
	sub, err := fs.Sub(dashboardFS, "dashboard_dist")
	if err != nil {
		// No build embedded — serve a minimal placeholder.
		mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "text/html; charset=utf-8")
			fmt.Fprint(w, fallbackHTML)
		})
		return
	}

	fileServer := http.FileServer(http.FS(sub))
	mux.Handle("/", spaHandler{root: sub, fallback: fileServer})
}

// spaHandler serves embedded files; on 404 it falls back to index.html so client-side
// routing works for static-exported Next.js apps.
type spaHandler struct {
	root     fs.FS
	fallback http.Handler
}

func (s spaHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	clean := path.Clean(r.URL.Path)
	if clean == "/" {
		s.serveIndex(w, r)
		return
	}
	// Skip the API namespace.
	if strings.HasPrefix(clean, "/api/") {
		http.NotFound(w, r)
		return
	}
	rel := strings.TrimPrefix(clean, "/")
	if _, err := fs.Stat(s.root, rel); err == nil {
		s.fallback.ServeHTTP(w, r)
		return
	}
	// Try with .html suffix (Next.js export pattern).
	if _, err := fs.Stat(s.root, rel+".html"); err == nil {
		r2 := *r
		r2.URL.Path = "/" + rel + ".html"
		s.fallback.ServeHTTP(w, &r2)
		return
	}
	s.serveIndex(w, r)
}

func (s spaHandler) serveIndex(w http.ResponseWriter, r *http.Request) {
	data, err := fs.ReadFile(s.root, "index.html")
	if err != nil {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		fmt.Fprint(w, fallbackHTML)
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	_, _ = w.Write(data)
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

// replayRequest re-sends the captured request directly to the local app,
// captures the new response, and stores it.
func replayRequest(orig *CapturedRequest, localPort string, store *requestStore) (*CapturedRequest, error) {
	addr := "127.0.0.1:" + localPort
	urlStr := "http://" + addr + orig.Path
	var bodyReader io.Reader
	if orig.ReqBody != "" {
		bodyReader = bytes.NewBufferString(orig.ReqBody)
	}
	httpReq, err := http.NewRequest(orig.Method, urlStr, bodyReader)
	if err != nil {
		return nil, fmt.Errorf("build replay request: %w", err)
	}
	for k, v := range orig.ReqHeaders {
		if isHopByHopHeaderName(k) {
			continue
		}
		httpReq.Header.Set(k, v)
	}
	httpReq.Host = addr

	client := &http.Client{Timeout: 30 * time.Second}
	start := time.Now()
	resp, err := client.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("replay failed: %w", err)
	}
	defer resp.Body.Close()
	respBody, _ := io.ReadAll(io.LimitReader(resp.Body, maxBodySize+1))

	captured := &CapturedRequest{
		ID:         newRequestID(),
		Timestamp:  time.Now(),
		Method:     orig.Method,
		Path:       orig.Path,
		StatusCode: resp.StatusCode,
		Duration:   time.Since(start).Milliseconds(),
		ReqHeaders: orig.ReqHeaders,
		ReqBody:    orig.ReqBody,
		ResHeaders: flattenHeaders(resp.Header),
		ResBody:    truncateBody(respBody),
	}
	store.Add(captured)
	logRequest(captured.Method, captured.Path, captured.StatusCode, http.StatusText(captured.StatusCode))
	return captured, nil
}

func isHopByHopHeaderName(name string) bool {
	for _, h := range hopByHopHeaders {
		if strings.EqualFold(h, name) {
			return true
		}
	}
	return false
}

const fallbackHTML = `<!DOCTYPE html>
<html>
<head>
<title>GoPort Dashboard</title>
<style>
body { font-family: system-ui, monospace; background: #0a0a0f; color: #e2e8f0; padding: 2rem; }
h1 { color: #25f59a; }
code { background: #18181f; padding: 0.25rem 0.5rem; border-radius: 4px; }
</style>
</head>
<body>
<h1>GoPort Dashboard</h1>
<p>The full UI hasn't been built yet. The API is live at <code>/api/requests</code>.</p>
<p>To build the UI: <code>cd dashboard && pnpm install && pnpm build</code>, then rebuild goport.</p>
</body>
</html>`
