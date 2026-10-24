package tunnel

import (
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
)

func TestInspectorGuard(t *testing.T) {
	state := &dashboardState{store: newRequestStore(10), plan: &planWatcher{}}
	mux := http.NewServeMux()
	registerAPIRoutes(mux, state)
	handler := inspectorGuard(4040, mux)

	cases := []struct {
		name    string
		method  string
		host    string
		headers map[string]string
		want    int
	}{
		{"same origin read", http.MethodGet, "127.0.0.1:4040", nil, http.StatusOK},
		{"localhost read", http.MethodGet, "localhost:4040", nil, http.StatusOK},
		{"DNS rebinding", http.MethodGet, "evil.example:4040", nil, http.StatusForbidden},
		{"other port", http.MethodGet, "127.0.0.1:4041", nil, http.StatusForbidden},
		{"cross-site fetch", http.MethodGet, "127.0.0.1:4040", map[string]string{"Origin": "https://evil.example"}, http.StatusForbidden},
		{"null origin", http.MethodGet, "127.0.0.1:4040", map[string]string{"Origin": "null"}, http.StatusForbidden},
		{"same origin with Origin", http.MethodGet, "127.0.0.1:4040", map[string]string{"Origin": "http://127.0.0.1:4040"}, http.StatusOK},
		{"cross-site no-cors", http.MethodGet, "127.0.0.1:4040", map[string]string{"Sec-Fetch-Site": "cross-site"}, http.StatusForbidden},
		{"delete without header", http.MethodDelete, "127.0.0.1:4040", nil, http.StatusForbidden},
		{"delete with header", http.MethodDelete, "127.0.0.1:4040", map[string]string{inspectorHeader: "1"}, http.StatusOK},
	}
	for _, tc := range cases {
		req := httptest.NewRequest(tc.method, "/api/requests", nil)
		req.Host = tc.host
		for k, v := range tc.headers {
			req.Header.Set(k, v)
		}
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, req)
		if rec.Code != tc.want {
			t.Errorf("%s: status = %d, want %d", tc.name, rec.Code, tc.want)
		}
		if rec.Header().Get("Access-Control-Allow-Origin") != "" {
			t.Errorf("%s: CORS header sent", tc.name)
		}
	}
}

func TestReplayWithoutHeaderIsRejected(t *testing.T) {
	state := &dashboardState{store: newRequestStore(10), plan: &planWatcher{}}
	state.store.Add(&CapturedRequest{ID: "req_1", Method: http.MethodGet, Path: "/"})
	mux := http.NewServeMux()
	registerAPIRoutes(mux, state)
	req := httptest.NewRequest(http.MethodPost, "/api/requests/req_1/replay", nil)
	req.Host = "127.0.0.1:4040"
	rec := httptest.NewRecorder()
	inspectorGuard(4040, mux).ServeHTTP(rec, req)
	if rec.Code != http.StatusForbidden {
		t.Fatalf("status = %d, want 403", rec.Code)
	}
}

func TestReplayURLStaysOnTheLocalApp(t *testing.T) {
	const addr = "127.0.0.1:3000"
	for _, path := range []string{"x:@192.168.1.1/admin", "http://evil.example/", "evil.example/x", "*", ""} {
		if u, err := replayURL(addr, path); err == nil {
			t.Errorf("replayURL(%q) = %s, want an error", path, u)
		}
	}
	for path, want := range map[string]string{
		"/":                    "http://127.0.0.1:3000/",
		"/api/users?page=2":    "http://127.0.0.1:3000/api/users?page=2",
		"//evil.example/admin": "http://127.0.0.1:3000//evil.example/admin",
		"/a%2Fb?q=%20":         "http://127.0.0.1:3000/a%2Fb?q=%20",
	} {
		u, err := replayURL(addr, path)
		if err != nil {
			t.Errorf("replayURL(%q) error = %v", path, err)
			continue
		}
		if u.String() != want || u.Host != addr {
			t.Errorf("replayURL(%q) = %s, want %s", path, u, want)
		}
	}
}

func TestReplayDoesNotFollowRedirects(t *testing.T) {
	var elsewhere atomic.Int32
	other := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		elsewhere.Add(1)
	}))
	defer other.Close()
	app := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, other.URL+"/admin", http.StatusFound)
	}))
	defer app.Close()

	store := newRequestStore(10)
	got, err := replayRequest(&CapturedRequest{Method: http.MethodGet, Path: "/start"}, newLocalTarget(portOf(t, app)), store)
	if err != nil {
		t.Fatal(err)
	}
	if got.StatusCode != http.StatusFound || elsewhere.Load() != 0 {
		t.Fatalf("status = %d, redirect followed %d times", got.StatusCode, elsewhere.Load())
	}
}

func TestReplaySendsBodyAndHeaders(t *testing.T) {
	app := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		_, _ = io.WriteString(w, r.Method+" "+r.URL.RequestURI()+" "+r.Header.Get("X-Test")+" "+string(body))
	}))
	defer app.Close()

	got, err := replayRequest(&CapturedRequest{
		Method: http.MethodPost, Path: "/echo?x=1",
		ReqHeaders: map[string]string{"X-Test": "yes", "Connection": "close"},
		ReqBody:    "hello",
	}, newLocalTarget(portOf(t, app)), newRequestStore(10))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(got.ResBody, "POST /echo?x=1 yes hello") {
		t.Fatalf("replay response = %q", got.ResBody)
	}
}
