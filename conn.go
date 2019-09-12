package throttle

import (
	"net"
	"time"
)

// Conn throttles net.Conn overall read/write bandwidth
// using hierarchy of 2 buckets: a server class one and
// per connection.
type Conn struct {
	c net.Conn
	// TODO: bucket hierarchy
	b Bucket
}

var _ net.Conn = (*Conn)(nil)
var _ Throttle = (*Conn)(nil)

func WrapConn(c net.Conn) *Conn {
	return &Conn{c: c}
}

func (c *Conn) Read(b []byte) (n int, err error) {
	panic("implement me")
}

func (c *Conn) Write(b []byte) (n int, err error) {
	panic("implement me")
}

func (c *Conn) Close() error {
	return c.c.Close()
}

func (c *Conn) LocalAddr() net.Addr {
	return c.c.LocalAddr()
}

func (c *Conn) RemoteAddr() net.Addr {
	return c.c.RemoteAddr()
}

func (c *Conn) SetDeadline(t time.Time) error {
	return c.c.SetDeadline(t)
}

func (c *Conn) SetReadDeadline(t time.Time) error {
	return c.c.SetReadDeadline(t)
}

func (c *Conn) SetWriteDeadline(t time.Time) error {
	return c.c.SetWriteDeadline(t)
}

func (c *Conn) SetCapacity(capacity uint64) {
	c.b.SetCapacity(capacity)
}
