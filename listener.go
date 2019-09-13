package throttle

import (
	"net"
	"sync/atomic"
)

// Listener implements throttled net.Listener which
// return throttled connections. Connections have
// 2-level bucket hierarchy.
type Listener struct {
	l net.Listener

	// server class bucket
	b Bucket

	// bandwidth per incoming connection
	connBandwidth uint64
}

var _ net.Listener = (*Listener)(nil)
var _ Capacity = (*Listener)(nil)

func WrapListener(listener net.Listener) *Listener {
	return &Listener{
		l: listener,
	}
}

func (l *Listener) Accept() (net.Conn, error) {
	conn, err := l.l.Accept()
	if err != nil {
		return nil, err
	}
	wrap := WrapConnWithParent(conn, &l.b)
	wrap.SetCapacity(atomic.LoadUint64(&l.connBandwidth))
	return wrap, nil
}

func (l *Listener) Close() error {
	return l.l.Close()
}

func (l *Listener) Addr() net.Addr {
	return l.l.Addr()
}

func (l *Listener) SetCapacity(capacity uint64) {
	l.b.SetCapacity(capacity)
	// forgive race condition for concurrent sets
	l.b.SetFill(capacity)
}

func (l *Listener) SetConnCapacity(capacity uint64) {
	atomic.StoreUint64(&l.connBandwidth, capacity)
}
