package throttle

import (
	"context"
	"net"
	"testing"
	"time"
)

func TestHierarchy(t *testing.T) {
	t.Run("test only overall root bandwidth", func(t *testing.T) {
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
}
