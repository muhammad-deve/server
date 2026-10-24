package tunnel

import (
	"errors"
	"fmt"
	"io"
	"net"
	"strings"
	"sync"
	"syscall"
	"time"
)

const localDialTimeout = 10 * time.Second

// localTarget dials the user's app. Most dev servers listen on 127.0.0.1, but
// some listen on ::1 only (Vite on macOS), so a refused connection is retried
// on the other loopback address, and the one that answered is tried first
// next time.
type localTarget struct {
	port string

	mu        sync.Mutex
	preferV6  bool
	lastError string
}

func newLocalTarget(port string) *localTarget {
	return &localTarget{port: port}
}

func (l *localTarget) addrs() (first, second string) {
	v4 := net.JoinHostPort("127.0.0.1", l.port)
	v6 := net.JoinHostPort("::1", l.port)
	l.mu.Lock()
	defer l.mu.Unlock()
	if l.preferV6 {
		return v6, v4
	}
	return v4, v6
}

// addr is the loopback address tried first, "127.0.0.1:<port>" or "[::1]:<port>".
func (l *localTarget) addr() string {
	first, _ := l.addrs()
	return first
}

func (l *localTarget) dial() (net.Conn, string, error) {
	first, second := l.addrs()
	conn, err := net.DialTimeout("tcp", first, localDialTimeout)
	if err == nil {
		return conn, first, nil
	}
	if !isConnRefused(err) {
		return nil, first, err
	}
	conn, otherErr := net.DialTimeout("tcp", second, localDialTimeout)
	if otherErr != nil {
		return nil, first, err
	}
	l.mu.Lock()
	l.preferV6 = strings.HasPrefix(second, "[")
	l.mu.Unlock()
	return conn, second, nil
}

// reportFailure prints why the local app could not be reached, once per
// distinct reason, so a stopped app doesn't flood the request log.
func (l *localTarget) reportFailure(err error) {
	msg := fmt.Sprintf("Could not reach your app on localhost:%s: %s", l.port, errText(err))
	l.mu.Lock()
	repeated := msg == l.lastError
	l.lastError = msg
	l.mu.Unlock()
	if !repeated {
		logNotice(msg)
	}
}

func (l *localTarget) reportSuccess() {
	l.mu.Lock()
	l.lastError = ""
	l.mu.Unlock()
}

// isConnRefused also matches WSAECONNREFUSED, which Go does not map to
// syscall.ECONNREFUSED on Windows.
func isConnRefused(err error) bool {
	return errors.Is(err, syscall.ECONNREFUSED) || errors.Is(err, syscall.Errno(10061))
}

// bodyCapture keeps the first bytes of a body that streams through, for the
// inspector, and notes whether the whole body was seen.
type bodyCapture struct {
	mu       sync.Mutex
	buf      []byte
	limit    int
	total    int64
	complete bool
}

func newBodyCapture(limit int) *bodyCapture {
	return &bodyCapture{limit: limit}
}

func (c *bodyCapture) add(p []byte, eof bool) {
	c.mu.Lock()
	if room := c.limit - len(c.buf); room > 0 {
		c.buf = append(c.buf, p[:min(room, len(p))]...)
	}
	c.total += int64(len(p))
	c.complete = c.complete || eof
	c.mu.Unlock()
}

// Truncated reports that the kept bytes are not the whole body: it was longer
// than the limit, or it was not read to the end.
func (c *bodyCapture) Truncated() bool {
	if c == nil {
		return false
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	return !c.complete || c.total > int64(len(c.buf))
}

// Text returns the kept bytes, marked when they are not the whole body.
func (c *bodyCapture) Text() string {
	if c == nil {
		return ""
	}
	truncated := c.Truncated()
	c.mu.Lock()
	defer c.mu.Unlock()
	if truncated && c.total > 0 {
		return string(c.buf) + truncatedMarker
	}
	return string(c.buf)
}

// watchedBody remembers whether reading the visitor's body failed, as opposed
// to writing it to the app.
type watchedBody struct {
	io.ReadCloser
	mu  sync.Mutex
	err error
}

func (b *watchedBody) Read(p []byte) (int, error) {
	n, err := b.ReadCloser.Read(p)
	if err != nil && err != io.EOF {
		b.mu.Lock()
		b.err = err
		b.mu.Unlock()
	}
	return n, err
}

func (b *watchedBody) failed() bool {
	if b == nil {
		return false
	}
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.err != nil
}

// capturingBody copies what is read from a body into a bodyCapture.
type capturingBody struct {
	io.ReadCloser
	c *bodyCapture
}

func (b capturingBody) Read(p []byte) (int, error) {
	n, err := b.ReadCloser.Read(p)
	b.c.add(p[:n], err == io.EOF)
	return n, err
}

func captureBody(body io.ReadCloser, c *bodyCapture) io.ReadCloser {
	return capturingBody{ReadCloser: body, c: c}
}
