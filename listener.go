package throttle

import "net"

// Listener implements throttled net.Listener which
// return throttled connections. Connections have
// 2-level bucket hierarchy.
type Listener struct {
	l net.Listener

	// server class bucket
	// TODO: semi-fair queuing
	s Bucket
}

var _ net.Listener = (*Listener)(nil)
var _ Throttle = (*Listener)(nil)

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
	return WrapConn(conn), nil
}

func (l *Listener) Close() error {
	return l.l.Close()
}

func (l *Listener) Addr() net.Addr {
	return l.l.Addr()
}

func (l *Listener) SetCapacity(capacity uint64) {
	l.s.SetCapacity(capacity)
}
