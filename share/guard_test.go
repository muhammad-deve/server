package share

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func unlockAs(handler *Handler, ip, password string) *httptest.ResponseRecorder {
	request := httptest.NewRequest(http.MethodPost, "/s/"+testToken+"/api/unlock", strings.NewReader(`{"password":"`+password+`"}`))
	request.Header.Set("X-Real-IP", ip)
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	return response
}

func TestUnlockLocksOutAfterThreeWrongPasswords(t *testing.T) {
	handler, _ := newTestHandler(t, "correct horse", time.Time{})
	now := time.Now()
	handler.guard.now = func() time.Time { return now }

	for i := 1; i <= 2; i++ {
		if code := unlockAs(handler, "203.0.113.7", "wrong guess").Code; code != http.StatusUnauthorized {
			t.Fatalf("attempt %d: status = %d, want 401", i, code)
		}
	}
	third := unlockAs(handler, "203.0.113.7", "wrong guess")
	if third.Code != http.StatusTooManyRequests || third.Header().Get("Retry-After") != "60" {
		t.Fatalf("third attempt: status = %d, Retry-After = %q", third.Code, third.Header().Get("Retry-After"))
	}
	// Locked even with the right password, but other visitors are not.
	if code := unlockAs(handler, "203.0.113.7", "correct horse").Code; code != http.StatusTooManyRequests {
		t.Fatalf("locked visitor: status = %d, want 429", code)
	}
	if code := unlockAs(handler, "198.51.100.1", "correct horse").Code; code != http.StatusNoContent {
		t.Fatalf("other visitor: status = %d, want 204", code)
	}

	now = now.Add(61 * time.Second)
	if code := unlockAs(handler, "203.0.113.7", "correct horse").Code; code != http.StatusNoContent {
		t.Fatalf("after the pause: status = %d, want 204", code)
	}
}

func TestUnlockGuardSchedule(t *testing.T) {
	guard := newUnlockGuard()
	now := time.Now()
	guard.now = func() time.Time { return now }
	want := map[int]time.Duration{1: 0, 2: 0, 3: time.Minute, 4: 0, 5: 30 * time.Minute, 6: 0, 7: 24 * time.Hour}
	for attempt := 1; attempt <= 7; attempt++ {
		if _, ok := guard.begin("v"); !ok {
			t.Fatalf("attempt %d refused", attempt)
		}
		if got := guard.finish("v", false); got != want[attempt] {
			t.Fatalf("attempt %d: pause = %v, want %v", attempt, got, want[attempt])
		}
		now = now.Add(guard.visitors["v"].lockedUntil.Sub(now) + time.Second)
	}
}

func TestUnlockGuardCapsConcurrentChecks(t *testing.T) {
	guard := newUnlockGuard()
	for i := 0; i < unlockMaxChecking; i++ {
		if _, ok := guard.begin(string(rune('a' + i))); !ok {
			t.Fatalf("check %d refused", i)
		}
	}
	if _, ok := guard.begin("z"); ok {
		t.Fatal("more than unlockMaxChecking checks ran at once")
	}
	if _, ok := guard.begin("a"); ok {
		t.Fatal("one visitor ran two checks at once")
	}
	guard.finish("a", true)
	if _, ok := guard.begin("z"); !ok {
		t.Fatal("slot not released")
	}
}

// Spoofed addresses don't help: the share pauses after too many failures overall.
func TestUnlockGuardPausesTheShareAcrossVisitors(t *testing.T) {
	guard := newUnlockGuard()
	for i := 0; i < unlockShareMax; i++ {
		visitor := "visitor-" + string(rune('A'+i))
		if _, ok := guard.begin(visitor); !ok {
			t.Fatalf("visitor %d refused early", i)
		}
		guard.finish(visitor, false)
	}
	if wait, ok := guard.begin("fresh"); ok || wait <= 0 {
		t.Fatalf("share not paused: ok=%v wait=%v", ok, wait)
	}
}

func TestVisitorKey(t *testing.T) {
	request := httptest.NewRequest(http.MethodPost, "/", nil)
	request.RemoteAddr = "127.0.0.1:5000"
	request.Header.Set("X-Forwarded-For", "10.0.0.1, 203.0.113.9")
	if got := visitorKey(request); got != "203.0.113.9" {
		t.Fatalf("X-Forwarded-For key = %q", got)
	}
	request.Header.Set("X-Real-IP", "2001:db8:1:2:3:4:5:6")
	if got := visitorKey(request); got != "2001:db8:1:2::/64" {
		t.Fatalf("IPv6 key = %q", got)
	}
}
