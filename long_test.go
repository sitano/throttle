// +build integration

// I am sorry for such a messy tests, but this is only a proof of concept.
package throttle

import (
	"context"
	"fmt"
	"math/rand"
	"net"
	"strconv"
	"sync/atomic"
	"testing"
	"time"
)

// TestHour runs a server consumer test for 1 hour.
// It uses 1MB/s total server throughput with unlimited channels.
// Unlimited server conns allows consuming of the whole throughput by 1 conn.
// Writers are unlimited but randomly picks write bursts.
//
// This test shows if the throttler is unfair on unlimited throttle
// leafs but limited root bucket. If the consume sizes are small
// (what is regulated on the app level), the overall distribution
// will be naturally uniform.
func TestHour(t *testing.T) {
	const window = time.Hour
	const bandwidth = 1024 * 1024
	const threads = 10

	ts := NewTestSystem(t)
	overallRead := NewMeasureBandwidth(t, bandwidth, "overall_read")
	var ag15stat [threads]uint64
	ts.StartListener(func(ctx context.Context, id uint64, conn net.Conn) {
		var buf = make([]byte, bandwidth)
		var stat = NewMeasureBandwidth(t, bandwidth, "read")

		go func() {
			var prevRead uint64
			var w15start = time.Now()
			var w15stat [15]uint64

			for ctx.Err() == nil {
				time.Sleep(time.Second)

				m := stat
				dt := time.Since(m.s)

				w15stat[(time.Since(w15start)/time.Second)%15] = m.Consumed() - prevRead
				w15sum := uint64(0)
				for i := 0; i < len(w15stat); i++ {
					w15sum += w15stat[i]
				}

				fmt.Println(
					">>>",
					m.op,
					"id=", id,
					"total=", m.Consumed(),
					"u=", fmt.Sprintf("%0.4f", float64(m.Consumed()-prevRead)/bandwidth),
					"ds=", m.Consumed()-prevRead,
					"dsa/15s=", w15sum/15,
					"dss/15s=", w15sum,
					"md=", bandwidth,
					"in=", dt)
				prevRead = m.Consumed()
				atomic.StoreUint64(&ag15stat[id-1], w15sum)
			}
		}()

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
			fmt.Println("id=", id, "consumed", stat.Consumed(), "last", n, "since", time.Since(stat.s))
			if ctx.Err() != nil {
				break
			}
			n, err = conn.Write([]byte(strconv.FormatUint(stat.Consumed(), 10)))
			if err != nil {
				if ctx.Err() == nil {
					t.Error("id=", id, "server write:", err)
				}
				break
			}
			// fmt.Println("id=", id, "wrote", stat.Consumed(), "last", n, "since", time.Since(stat.s))
			if ctx.Err() != nil {
				break
			}
		}

		t.Log(
			stat.op,
			"id =", id,
			"total =", stat.Consumed(),
			"max =", stat.Projected(time.Since(stat.s)),
			"in =", time.Since(stat.s))
	}, bandwidth, 0)

	ts.ln.(Capacity).Reset()

	ts.StartClient(func(ctx context.Context, id uint64, conn net.Conn) {
		var buf = make([]byte, 10*bandwidth)
		for j := 0; j < len(buf); j++ {
			buf[j] = byte(id)
		}
		var stat = NewMeasureBandwidth(t, bandwidth, "write")

		for {
			send := buf[:1+rand.Intn(10*bandwidth)]
			fmt.Println("id=", id, "writing", len(send), "since", time.Since(stat.s))
			n, err := conn.Write(send)
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
			fmt.Println("id=", id, "wrote", stat.Consumed(), "last", n, "since", time.Since(stat.s))
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
			"total =", stat.Consumed(),
			"max = ", stat.Projected(time.Since(stat.s)),
			"in =", time.Since(stat.s))
	}, 0, threads)

	start := time.Now()
	var lastConsumed uint64
	for time.Since(start) < window {
		time.Sleep(time.Second)

		m := overallRead
		dt := time.Since(m.s)
		projected := m.Projected(dt)
		accuracy := m.Accuracy(projected)

		ag15sum := uint64(0)
		for i := 0; i < len(ag15stat); i++ {
			ag15sum += atomic.LoadUint64(&ag15stat[i])
		}

		fmt.Println(
			">>>",
			m.op,
			"total=", m.Consumed(),
			"proj=", projected,
			"acc=", accuracy,
			"ds=", m.Consumed()-lastConsumed,
			"avg/15s=", ag15sum/15,
			"sum/15s=", ag15sum,
			"util/15s=", float64(ag15sum)/float64(bandwidth*15),
			"in=", dt)

		lastConsumed = m.Consumed()
	}

	ts.Stop()
}
