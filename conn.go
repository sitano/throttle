package throttle

import (
	"errors"
	"net"
	"strconv"
	"time"
)

// Conn throttles net.Conn overall read/write bandwidth
// using buckets hierarchy. Throttling does not count
// deadlines in current impl.
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

// Read can't peek utilization of the read buffer
// in advance. So doing our best we are just consuming
// min(len(b), bucket.capacity) from the bucket and return it.
func (c *Conn) Read(b []byte) (n int, err error) {
	if len(b) == 0 {
		return 0, nil
	}

	// reserved = min(len(b), bucket.capacity)
	// as there is no sense to wait more than a sec
	// not knowing read buf utilization.
	//
	// so do the best effort and configure size of
	// recv buf b knowing your throttled bandwidth
	// and application level traffic pattern.
	reserved := c.b.Consume(uint64(len(b)))

	return c.c.Read(b[:reserved])
}

// Write naively throttles amount of write. It could
// try to push first available tokens as soon as it could,
// but not this time.
func (c *Conn) Write(b []byte) (n int, err error) {
	var n2 int

	for len(b) > 0 {
		reserved := c.b.Consume(uint64(len(b)))
		if reserved == 0 {
			return n, errors.New("consumed 0 requested " + strconv.Itoa(len(b)))
		}

		n2, err = c.c.Write(b[:reserved])
		n += n2
		if err != nil {
			return n, err
		}

		b = b[reserved:]
	}

	return n, err
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
	// forgive race condition for concurrent sets
	c.b.SetFill(capacity)
}
