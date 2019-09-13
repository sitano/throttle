// I am sorry for such a messy tests, but this is only a proof of concept.
package throttle

import (
	"context"
	"net"
	"testing"
	"time"
)

func TestHierarchy(t *testing.T) {
	t.Run("test only overall root bandwidth reads", func(t *testing.T) {
		t.Parallel()

		const window = 3 * time.Second
		const bandwidth = 100000 // bytes / sec
		const bufSize = 10000

		ts := NewTestSystem(t)
		overallRead := NewMeasureBandwidth(t, bandwidth, "overall_read")
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
				overallRead.Consume(uint64(n))
				// fmt.Println("id=", id, "consumed", consumed1, "last", n, "since", time.Since(start))
				if ctx.Err() != nil {
					break
				}
			}

			t.Log(
				stat.op,
				"id =", id,
				"total =", stat.c,
				"in =", time.Since(stat.s))
		}, bandwidth, 0)

		ts.StartClient(func(ctx context.Context, id uint64, conn net.Conn) {
			var buf = make([]byte, bufSize)
			for j := 0; j < len(buf); j++ {
				buf[j] = byte(id)
			}
			var stat = NewMeasureBandwidth(t, bandwidth/2, "write")

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
				// fmt.Println("id=", id, "wrote", stat.c, "last", n, "since", time.Since(stat.s))
				if ctx.Err() != nil {
					break
				}
			}

			// Go net stack buffers too actively.
			// so the underlying system does not push back
			// when no one reads on other end fast enough.
			stat.Print(id)
		}, bandwidth/2, 5)

		time.Sleep(window)
		ts.Stop()
		overallRead.Check(0, bufSize)
	})

	t.Run("test only overall root bandwidth reads/writes", func(t *testing.T) {
		t.Parallel()

		const window = 3 * time.Second
		const bandwidth = 100000 // bytes / sec
		const bufSize = 10000

		ts := NewTestSystem(t)
		overallRead := NewMeasureBandwidth(t, bandwidth/2, "overall_halfed_read")
		ts.StartListener(func(ctx context.Context, id uint64, conn net.Conn) {
			var buf = make([]byte, bufSize)
			var stat = NewMeasureBandwidth(t, bandwidth, "read")

			for {
				n, err := conn.Read(buf)
				if err != nil {
					if ts.ctx.Err() == nil {
						t.Error("server read:", err)
					}
					break
				}
				stat.Consume(uint64(n))
				overallRead.Consume(uint64(n))
				// fmt.Println("id=", id, "consumed", consumed1, "last", n, "since", time.Since(start))
				if ctx.Err() != nil {
					break
				}
				// send everything back
				n, err = conn.Write(buf[:n])
				if err != nil {
					if ctx.Err() == nil {
						t.Error("id=", id, "server write:", err)
					}
					break
				}
				if n != len(buf) {
					t.Error("id=", id, "invalid len:", n, "!=", len(buf))
				}
				// fmt.Println("id=", id, "wrote", stat.c, "last", n, "since", time.Since(stat.s))
				if ctx.Err() != nil {
					break
				}
			}

			t.Log(
				stat.op,
				"id =", id,
				"total =", stat.c,
				"max =", stat.Projected(time.Since(stat.s)),
				"in =", time.Since(stat.s))
		}, bandwidth, 0)

		ts.StartClient(func(ctx context.Context, id uint64, conn net.Conn) {
			var buf = make([]byte, bufSize)
			for j := 0; j < len(buf); j++ {
				buf[j] = byte(id)
			}
			var stat = NewMeasureBandwidth(t, bandwidth/2, "write")

			for {
				buf := buf[:bufSize]

				n, err := conn.Write(buf)
				if err != nil {
					if ctx.Err() == nil {
						t.Error("id=", id, "client write:", err)
					}
					break
				}
				if n != len(buf) {
					t.Error("id=", id, "invalid len:", n, "!=", len(buf))
				}
				stat.Consume(uint64(n))
				// fmt.Println("id=", id, "wrote", stat.c, "last", n, "since", time.Since(stat.s))
				if ctx.Err() != nil {
					break
				}
				n, err = conn.Read(buf)
				if err != nil {
					if ts.ctx.Err() == nil {
						t.Error("client read:", err)
					}
					break
				}
				if ctx.Err() != nil {
					break
				}
			}

			t.Log(
				stat.op,
				"id =", id,
				"total =", stat.c,
				"max = ", stat.Projected(time.Since(stat.s)),
				"in =", time.Since(stat.s))
		}, bandwidth/2, 5)

		time.Sleep(window)
		ts.Stop()
		overallRead.Check(0, bufSize)
	})

	t.Run("test 2 level bandwidth limit", func(t *testing.T) {
		t.Parallel()

		const window = 3 * time.Second
		const bandwidth = 100000 // bytes / sec
		const bufSize = 10000

		ts := NewTestSystem(t)
		overallRead := NewMeasureBandwidth(t, bandwidth/2, "overall_halfed_read")
		ts.StartListener(func(ctx context.Context, id uint64, conn net.Conn) {
			var buf = make([]byte, bufSize)
			var stat = NewMeasureBandwidth(t, bandwidth, "read")

			if id == 1 {
				conn.(Capacity).SetCapacity(bandwidth / 100)
			}

			for {
				n, err := conn.Read(buf)
				if err != nil {
					if ts.ctx.Err() == nil {
						t.Error("server read:", err)
					}
					break
				}
				stat.Consume(uint64(n))
				overallRead.Consume(uint64(n))
				// fmt.Println("id=", id, "consumed", consumed1, "last", n, "since", time.Since(start))
				if ctx.Err() != nil {
					break
				}
				// send everything back
				n, err = conn.Write(buf[:n])
				if err != nil {
					if ctx.Err() == nil {
						t.Error("id=", id, "server write:", err)
					}
					break
				}
				// fmt.Println("id=", id, "wrote", stat.c, "last", n, "since", time.Since(stat.s))
				if ctx.Err() != nil {
					break
				}
			}

			t.Log(
				stat.op,
				"id =", id,
				"total =", stat.c,
				"max =", stat.Projected(time.Since(stat.s)),
				"in =", time.Since(stat.s))
		}, bandwidth, bandwidth/2)

		ts.StartClient(func(ctx context.Context, id uint64, conn net.Conn) {
			var buf = make([]byte, bufSize)
			for j := 0; j < len(buf); j++ {
				buf[j] = byte(id)
			}
			var stat = NewMeasureBandwidth(t, bandwidth/2, "write")

			for {
				buf := buf[:bufSize]

				n, err := conn.Write(buf)
				if err != nil {
					if ctx.Err() == nil {
						t.Error("id=", id, "client write:", err)
					}
					break
				}
				if n != len(buf) {
					t.Error("id=", id, "invalid len:", n, "!=", len(buf))
				}
				stat.Consume(uint64(n))
				// fmt.Println("id=", id, "wrote", stat.c, "last", n, "since", time.Since(stat.s))
				if ctx.Err() != nil {
					break
				}
				n, err = conn.Read(buf)
				if err != nil {
					if ts.ctx.Err() == nil {
						t.Error("client read:", err)
					}
					break
				}
				if ctx.Err() != nil {
					break
				}
			}

			t.Log(
				stat.op,
				"id =", id,
				"total =", stat.c,
				"max = ", stat.Projected(time.Since(stat.s)),
				"in =", time.Since(stat.s))
		}, bandwidth/2, 5)

		time.Sleep(window)
		ts.Stop()
		overallRead.Check(0, bufSize)
	})
}
