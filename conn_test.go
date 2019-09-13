package throttle

import (
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

		ln, err := net.Listen("tcp", ":0")
		if err != nil {
			t.Fatal("net.Listen: ", err)
		}
		addr := ln.Addr()
		t.Log("listening at:", addr)

		wg := &sync.WaitGroup{}
		stop := uint64(0)

		wg.Add(1)
		go func() {
			defer wg.Done()

			id := 0

			for {
				conn, err := ln.Accept()
				if err != nil {
					if atomic.LoadUint64(&stop) == 0 {
						t.Error("ln.accept: ", err)
					}
					break
				}

				wrap := WrapConn(conn)
				wrap.SetCapacity(bandwidth)

				id++

				wg.Add(1)
				go func(id int, conn net.Conn) {
					defer wg.Done()

					var buf = make([]byte, bufSize)
					var consumed1 uint64
					var start = time.Now()

					for {
						n, err := conn.Read(buf)
						if err != nil {
							if atomic.LoadUint64(&stop) == 0 {
								t.Error("read:", err)
							}
							break
						}
						if n > 0 {
							consumed1 += uint64(n)
						}
						// fmt.Println("id=", id, "consumed", consumed1, "last", n, "since", time.Since(start))
						// if atomic.LoadUint64(&stop) > 0 {
						//	break
						//}
					}

					dt := time.Since(start)
					projected := uint64(bandwidth * dt / time.Second)
					accuracy := float64(consumed1)/float64(projected) - 1.0

					var check = t.Log
					if math.Abs(accuracy) > math.Max(0.1, 1.0*bufSize/bandwidth) {
						check = t.Error
					}
					check(
						"id =", id,
						"total consumption =", consumed1,
						"projected =", projected,
						"error =", accuracy, "in =", dt)

					_ = conn.Close()
				}(id, wrap)
			}
		}()

		for i := 0; i < 5; i++ {
			id := byte(i + 1)
			wg.Add(1)
			go func() {
				defer wg.Done()

				var consumed1 uint64
				var b [bandwidth * 2]byte
				for j := 0; j < len(b); j++ {
					b[j] = id
				}

				conn, err := net.Dial("tcp", addr.String())
				if err != nil {
					t.Error("id=", id, "net.dial:", err)
					return
				}
				defer conn.Close()

				conn2 := WrapConn(conn)
				conn2.SetCapacity(bandwidth)
				conn = conn2

				var start = time.Now()
				for {
					buf := b[:bufSize]

					n, err := conn.Write(buf)
					if err != nil {
						if atomic.LoadUint64(&stop) == 0 {
							t.Error("id=", id, "write:", err)
						}
						break
					}
					if n != len(buf) {
						t.Error("id=", id, "invalid len:", n, "!=", len(buf))
					}
					consumed1 += uint64(n)
					// fmt.Println("id=", id, "wrote", consumed1, "last", n, "since", time.Since(start))
					if atomic.LoadUint64(&stop) > 0 {
						break
					}
				}

				dt := time.Since(start)
				projected := uint64(bandwidth * dt / time.Second)
				accuracy := float64(consumed1)/float64(projected) - 1.0

				var check = t.Log
				if math.Abs(accuracy) > math.Max(0.1, 1.0*bufSize/bandwidth) {
					check = t.Error
				}
				check(
					"id =", id,
					"total written =", consumed1,
					"projected =", projected,
					"error =", accuracy, "in =", dt)
			}()
		}

		time.Sleep(window)
		atomic.StoreUint64(&stop, 1)
		_ = ln.Close()
		wg.Wait()
	})

}
