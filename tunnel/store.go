package tunnel

import (
	"crypto/rand"
	"encoding/hex"
	"net/http"
	"sync"
	"sync/atomic"
	"time"
)

// CapturedRequest is a fully captured HTTP request/response pair tunnelled
// through goport. It is stored in memory and exposed to the dashboard.
type CapturedRequest struct {
	ID         string            `json:"id"`
	Timestamp  time.Time         `json:"timestamp"`
	Method     string            `json:"method"`
	Path       string            `json:"path"`
	StatusCode int               `json:"statusCode"`
	Duration   int64             `json:"duration"` // milliseconds
	ReqHeaders map[string]string `json:"reqHeaders"`
	ReqBody    string            `json:"reqBody"`
	ResHeaders map[string]string `json:"resHeaders"`
	ResBody    string            `json:"resBody"`
}

// requestStore is a thread-safe ring buffer of captured requests with
// pub/sub support so the dashboard can stream new entries in real time.
type requestStore struct {
	mu          sync.RWMutex
	items       []*CapturedRequest
	cap         int
	totalCount  int64
	totalBytes  atomic.Int64
	subscribers map[chan *CapturedRequest]struct{}
}

func newRequestStore(capacity int) *requestStore {
	return &requestStore{
		cap:         capacity,
		items:       make([]*CapturedRequest, 0, capacity),
		subscribers: make(map[chan *CapturedRequest]struct{}),
	}
}

func (s *requestStore) Add(r *CapturedRequest) {
	s.mu.Lock()
	s.items = append(s.items, r)
	if len(s.items) > s.cap {
		s.items = s.items[len(s.items)-s.cap:]
	}
	s.totalCount++
	subs := make([]chan *CapturedRequest, 0, len(s.subscribers))
	for ch := range s.subscribers {
		subs = append(subs, ch)
	}
	s.mu.Unlock()

	// Non-blocking fan-out so a slow subscriber doesn't stall request handling.
	for _, ch := range subs {
		select {
		case ch <- r:
		default:
		}
	}
}

func (s *requestStore) List() []*CapturedRequest {
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := make([]*CapturedRequest, len(s.items))
	// Newest first for the dashboard.
	for i, r := range s.items {
		out[len(s.items)-1-i] = r
	}
	return out
}

func (s *requestStore) Get(id string) (*CapturedRequest, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	for _, r := range s.items {
		if r.ID == id {
			return r, true
		}
	}
	return nil, false
}

func (s *requestStore) Clear() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.items = s.items[:0]
}

func (s *requestStore) Total() int64 {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.totalCount
}

func (s *requestStore) TotalBytes() int64 {
	return s.totalBytes.Load()
}

// bytesCounter returns the underlying atomic counter so callers (e.g. a
// byte-counting connection wrapper) can increment it directly on the hot path.
func (s *requestStore) bytesCounter() *atomic.Int64 {
	return &s.totalBytes
}

func (s *requestStore) Subscribe() chan *CapturedRequest {
	ch := make(chan *CapturedRequest, 32)
	s.mu.Lock()
	s.subscribers[ch] = struct{}{}
	s.mu.Unlock()
	return ch
}

func (s *requestStore) Unsubscribe(ch chan *CapturedRequest) {
	s.mu.Lock()
	delete(s.subscribers, ch)
	s.mu.Unlock()
	close(ch)
}

func newRequestID() string {
	b := make([]byte, 6)
	if _, err := rand.Read(b); err != nil {
		return "req_unknown"
	}
	return "req_" + hex.EncodeToString(b)
}

// flattenHeaders converts an http.Header into a string map keeping the first
// value for each key (sufficient for the dashboard display).
func flattenHeaders(h http.Header) map[string]string {
	out := make(map[string]string, len(h))
	for k, v := range h {
		if len(v) > 0 {
			out[k] = v[0]
		}
	}
	return out
}

// truncateBody returns body data, capping at maxBodySize to keep memory bounded.
const maxBodySize = 256 * 1024 // 256 KiB

func truncateBody(b []byte) string {
	if len(b) <= maxBodySize {
		return string(b)
	}
	return string(b[:maxBodySize]) + "\n...[truncated]"
}
