package tunnel

import (
	"net"
	"sync/atomic"
)

// countingConn wraps a net.Conn and counts every byte read and written over
// the underlying TCP tunnel connection (to the server on port 7000). This
// captures all traffic that flows through the tunnel, including yamux framing
// and headers, not just HTTP bodies.
type countingConn struct {
	net.Conn
	count *atomic.Int64
}

func newCountingConn(conn net.Conn, count *atomic.Int64) *countingConn {
	return &countingConn{Conn: conn, count: count}
}

func (c *countingConn) Read(b []byte) (int, error) {
	n, err := c.Conn.Read(b)
	if n > 0 {
		c.count.Add(int64(n))
	}
	return n, err
}

func (c *countingConn) Write(b []byte) (int, error) {
	n, err := c.Conn.Write(b)
	if n > 0 {
		c.count.Add(int64(n))
	}
	return n, err
}
