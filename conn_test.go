// I am sorry for such a messy tests, but this is only a proof of concept.
package throttle

import (
	"context"
	"math"
	"net"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

func TestConn(t *testing.T) {
	t.Run("test 1 way conns", func(t *testing.T) {
		const window = 3 * time.Second
		const bandwidth = 1000 // bytes / sec
		const bufSize = 100

		ts := NewTestSystem(t)
		ts.StartListener(func(ctx context.Context, id uint64, conn net.Conn) {
			var buf = make([]byte, bufSize)
			var stat = NewMeasureBandwidth(t, bandwidth, "read")

			for {
				n, err := conn.Read(buf)
				if err != nil {
					if ts.ctx.Err() == nil {
						t.Error("read:", err)
					}
					break
				}
				stat.Consume(uint64(n))
				// fmt.Println("id=", id, "consumed", consumed1, "last", n, "since", time.Since(start))
				// if atomic.LoadUint64(&stop) > 0 {
				//	break
				//}
			}

			stat.Check(id, bufSize)
		}, 0, bandwidth)

		ts.StartClient(func(ctx context.Context, id uint64, conn net.Conn) {
			var buf = make([]byte, bufSize)
			for j := 0; j < len(buf); j++ {
				buf[j] = byte(id)
			}
			var stat = NewMeasureBandwidth(t, bandwidth, "write")

			for {
				buf := buf[:bufSize]

				n, err := conn.Write(buf)
				if err != nil {
					if ctx.Err() == nil {
						t.Error("id=", id, "write:", err)
					}
					break
				}
				if n != len(buf) {
					t.Error("id=", id, "invalid len:", n, "!=", len(buf))
				}
				stat.Consume(uint64(n))
				// fmt.Println("id=", id, "wrote", consumed1, "last", n, "since", time.Since(start))
				if ctx.Err() != nil {
					break
				}
			}

			stat.Check(id, bufSize)
		}, bandwidth, 5)

		time.Sleep(window)
		ts.Stop()
	})
}

type testSystem struct {
	t *testing.T

	ctx    context.Context
	cancel context.CancelFunc
	wg     sync.WaitGroup
	stop   uint64

	ln net.Listener

	sid uint64
	cid uint64
}

func NewTestSystem(t *testing.T) *testSystem {
	ctx, f := context.WithCancel(context.Background())
	return &testSystem{
		t:      t,
		ctx:    ctx,
		cancel: f,
	}
}

func (t *testSystem) StartListener(
	f func(ctx context.Context, id uint64, conn net.Conn),
	serverBandwidth uint64,
	serverConnBandwidth uint64,
) {
	if t.ln != nil {
		t.t.Fatal("listener already inited")
	}

	ln, err := net.Listen("tcp", ":0")
	if err != nil {
		t.t.Fatal("net.Listen: ", err)
	}
	t.t.Log("new listener at:", ln.Addr().String())

	wrap := WrapListener(ln)
	wrap.SetCapacity(serverBandwidth)
	wrap.SetConnCapacity(serverConnBandwidth)
	ln = wrap

	t.ln = ln

	t.wg.Add(1)
	go func() {
		defer t.wg.Done()

		for {
			conn, err := ln.Accept()
			if err != nil {
				if t.ctx.Err() == nil {
					t.t.Error("ln.accept: ", err)
				}
				break
			}

			id := atomic.AddUint64(&t.sid, 1)

			t.wg.Add(1)
			go func(conn net.Conn) {
				defer t.wg.Done()
				defer conn.Close()
				f(t.ctx, id, conn)
			}(conn)
		}
	}()
}

func (t *testSystem) StartClient(
	f func(ctx context.Context, id uint64, conn net.Conn),
	clientConnBandwidth uint64,
	count int,
) {
	addr := t.ln.Addr()

	for i := 0; i < count; i++ {
		id := atomic.AddUint64(&t.cid, 1)

		t.wg.Add(1)
		go func() {
			defer t.wg.Done()

			conn, err := net.Dial("tcp", addr.String())
			if err != nil {
				t.t.Error("id=", id, "net.dial:", err)
				return
			}
			defer conn.Close()

			conn2 := WrapConn(conn)
			conn2.SetCapacity(clientConnBandwidth)
			conn = conn2

			f(t.ctx, id, conn)
		}()
	}
}

func (t *testSystem) Stop() {
	t.cancel()
	atomic.StoreUint64(&t.stop, 1)
	if t.ln != nil {
		if err := t.ln.Close(); err != nil {
			t.t.Error("net.listener close:", err)
		}
	}
	t.wg.Wait()
	atomic.StoreUint64(&t.stop, 2)
}

type MeasureBandwidth struct {
	t  *testing.T
	op string
	s  time.Time
	c  uint64 // consumed
	b  uint64 // bandwidth
}

func NewMeasureBandwidth(t *testing.T, bandwidth uint64, op string) *MeasureBandwidth {
	return &MeasureBandwidth{
		t:  t,
		op: op,
		s:  time.Now(),
		b:  bandwidth,
	}
}

func (m *MeasureBandwidth) Consumed() uint64 {
	return atomic.LoadUint64(&m.c)
}

func (m *MeasureBandwidth) Consume(c uint64) {
	atomic.AddUint64(&m.c, c)
}

func (m *MeasureBandwidth) Projected(dt time.Duration) uint64 {
	return uint64(time.Duration(m.b) * dt / time.Second)
}

func (m *MeasureBandwidth) Accuracy(projected uint64) float64 {
	return float64(m.Consumed())/float64(projected) - 1.0
}

func (m *MeasureBandwidth) Print(id uint64) {
	dt := time.Since(m.s)
	projected := m.Projected(dt)
	accuracy := m.Accuracy(projected)

	m.t.Log(m.op,
		"id =", id,
		"total =", m.Consumed(),
		"projected =", projected,
		"accuracy =", accuracy, "in =", dt)
}

func (m *MeasureBandwidth) Check(id uint64, buf uint64) {
	dt := time.Since(m.s)
	projected := m.Projected(dt)
	accuracy := m.Accuracy(projected)

	var check = m.t.Log
	var args = []interface{}{
		m.op,
		"id =", id,
		"total =", m.Consumed(),
		"projected =", projected,
		"accuracy =", accuracy, "in =", dt,
	}
	if math.Abs(accuracy) > math.Max(0.1, float64(buf)/float64(m.b)) {
		check = m.t.Error
		args = append([]interface{}{"ERROR:"}, args...)
	}
	check(args...)
}
