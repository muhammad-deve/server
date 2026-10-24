package share

import (
	"net"
	"net/http"
	"sort"
	"strings"
	"sync"
	"time"
)

// unlockGuard slows down password guessing, like the backend's guard for
// cloud shares. A visitor pauses for a minute after 3 wrong passwords, 30
// minutes after 5 and a day from 7. Because the visitor address comes from a
// header, the whole share also pauses after too many wrong passwords in an
// hour from everyone. Only a few bcrypt checks run at once.
type unlockGuard struct {
	mu       sync.Mutex
	now      func() time.Time
	visitors map[string]*unlockAttempts
	failures []time.Time // wrong passwords from all visitors in the last hour
	checking int
}

type unlockAttempts struct {
	failures    int
	lastFailure time.Time
	lockedUntil time.Time
	checking    bool
}

const (
	unlockFailureMemory = 24 * time.Hour
	unlockShareWindow   = time.Hour
	unlockShareMax      = 30
	unlockMaxChecking   = 4
	unlockPruneAt       = 10000
)

func newUnlockGuard() *unlockGuard {
	return &unlockGuard{now: time.Now, visitors: make(map[string]*unlockAttempts)}
}

// unlockLockAfter is how long a visitor waits after their nth wrong password.
func unlockLockAfter(failures int) time.Duration {
	switch {
	case failures >= 7:
		return 24 * time.Hour
	case failures == 5:
		return 30 * time.Minute
	case failures == 3:
		return time.Minute
	}
	return 0
}

// begin reserves one password check. When the visitor or the share is paused,
// or too many checks are running, it returns how long to wait instead.
func (g *unlockGuard) begin(visitor string) (time.Duration, bool) {
	g.mu.Lock()
	defer g.mu.Unlock()
	now := g.now()
	g.pruneLocked(now)
	attempts := g.visitors[visitor]
	if attempts != nil && attempts.lockedUntil.After(now) {
		return attempts.lockedUntil.Sub(now), false
	}
	if wait := g.shareWaitLocked(now); wait > 0 {
		return wait, false
	}
	if (attempts != nil && attempts.checking) || g.checking >= unlockMaxChecking {
		return time.Second, false
	}
	if attempts == nil {
		attempts = &unlockAttempts{}
		g.visitors[visitor] = attempts
	}
	attempts.checking = true
	g.checking++
	return 0, true
}

// finish records the result of a check reserved with begin. For a wrong
// password it returns the pause that now applies, or zero.
func (g *unlockGuard) finish(visitor string, correct bool) time.Duration {
	g.mu.Lock()
	defer g.mu.Unlock()
	now := g.now()
	if g.checking > 0 {
		g.checking--
	}
	attempts := g.visitors[visitor]
	if attempts == nil {
		return 0
	}
	attempts.checking = false
	if correct {
		delete(g.visitors, visitor)
		return 0
	}
	if now.Sub(attempts.lastFailure) > unlockFailureMemory {
		attempts.failures = 0
	}
	attempts.failures++
	attempts.lastFailure = now
	g.failures = append(g.failures, now)
	if lock := unlockLockAfter(attempts.failures); lock > 0 {
		attempts.lockedUntil = now.Add(lock)
		return lock
	}
	return g.shareWaitLocked(now)
}

// cancel releases a check that never compared a password (a malformed body).
func (g *unlockGuard) cancel(visitor string) {
	g.mu.Lock()
	defer g.mu.Unlock()
	if g.checking > 0 {
		g.checking--
	}
	if attempts := g.visitors[visitor]; attempts != nil {
		attempts.checking = false
		if attempts.failures == 0 {
			delete(g.visitors, visitor)
		}
	}
}

// shareWaitLocked is how long until the share drops below the hourly ceiling.
func (g *unlockGuard) shareWaitLocked(now time.Time) time.Duration {
	cutoff := now.Add(-unlockShareWindow)
	index := sort.Search(len(g.failures), func(i int) bool { return g.failures[i].After(cutoff) })
	if index > 0 {
		g.failures = append(g.failures[:0], g.failures[index:]...)
	}
	if len(g.failures) < unlockShareMax {
		return 0
	}
	oldest := g.failures[len(g.failures)-unlockShareMax]
	return max(oldest.Add(unlockShareWindow).Sub(now), time.Second)
}

func (g *unlockGuard) pruneLocked(now time.Time) {
	if len(g.visitors) < unlockPruneAt {
		return
	}
	for key, attempts := range g.visitors {
		if !attempts.checking && !attempts.lockedUntil.After(now) && now.Sub(attempts.lastFailure) > unlockFailureMemory {
			delete(g.visitors, key)
		}
	}
}

// visitorKey identifies a visitor. Requests reach the share server through
// the tunnel, so RemoteAddr is always loopback; the GoPort server's nginx sets
// X-Real-IP. IPv6 addresses are grouped by /64.
func visitorKey(r *http.Request) string {
	address := net.ParseIP(strings.TrimSpace(r.Header.Get("X-Real-IP")))
	if address == nil {
		parts := strings.Split(r.Header.Get("X-Forwarded-For"), ",")
		address = net.ParseIP(strings.TrimSpace(parts[len(parts)-1]))
	}
	if address == nil {
		host, _, err := net.SplitHostPort(r.RemoteAddr)
		if err != nil {
			host = r.RemoteAddr
		}
		address = net.ParseIP(host)
	}
	if address == nil {
		return r.RemoteAddr
	}
	if address.To4() == nil {
		return address.Mask(net.CIDRMask(64, 128)).String() + "/64"
	}
	return address.String()
}
