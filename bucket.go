package throttle

import (
	"sync/atomic"
	"time"
)

// Bucket is a variant of a token bucket in which
// bucket size does not exceed output speed.
// (R = capacity/sec, B = R * 1 sec = capacity).
type Bucket struct {
	fill     uint64
	capacity uint64

	ts uint64
}

func NewBucket(capacity uint64) *Bucket {
	return &Bucket{
		fill:     0,
		capacity: capacity,
		ts:       0,
	}
}

// Consume consumes bucket tokens if there is enough of them
// in a thread safe manner. If there are not enough tokens
// it blocks waiting for the maximum capacity. Consume returns
// a capacity at most at once.
//
// It does not use mutexes as it assumes that there are no
// concurrent consumers of the bucket.
func (b *Bucket) Consume(consume uint64) uint64 {
	var capacity = atomic.LoadUint64(&b.capacity)
	var consumed = false
	var fill uint64

	if capacity == 0 {
		return consume
	}

	if consume > capacity {
		consume = capacity
	}

	for !consumed {
		// generate tokens past last timestamp
		var prev = atomic.LoadUint64(&b.ts)
		var now = uint64(time.Now().UnixNano())
		var since = now - prev
		if since >= uint64(time.Second) {
			if !atomic.CompareAndSwapUint64(&b.ts, prev, now) {
				continue
			}
			// forget possible race condition
			atomic.StoreUint64(&b.fill, 0)
			fill = 0
		} else {
			var delta_ms = (now - prev) / uint64(time.Millisecond)
			var tokens = capacity * delta_ms / 1000
			if tokens > 0 {
				for {
					if tokens > fill {
						// forget possible race condition
						atomic.StoreUint64(&b.fill, 0)
						fill = 0
						break
					} else if atomic.CompareAndSwapUint64(&b.fill, fill, fill-tokens) {
						fill -= tokens
						break
					}
					fill = atomic.LoadUint64(&b.fill)
				}
			}
		}

		// lock-free consume tokens
		for {
			fill = atomic.LoadUint64(&b.fill)

			if fill+consume <= capacity {
				consumed = atomic.CompareAndSwapUint64(&b.fill, fill, fill+consume)
				if !consumed {
					continue
				}
				fill += consume
			}

			break
		}

		// wait if there are no enough tokens
		if !consumed {
			var free uint64
			if capacity >= fill {
				free = capacity - fill
			}
			if free < consume {
				var wait = 1 + 1000*(consume-free)/capacity
				time.Sleep(time.Millisecond * time.Duration(wait))
			}
		}
	}

	return consume
}

func (b *Bucket) Fill() uint64 {
	return atomic.LoadUint64(&b.fill)
}

func (b *Bucket) Capacity() uint64 {
	return atomic.LoadUint64(&b.capacity)
}

func (b *Bucket) SetCapacity(capacity uint64) {
	atomic.StoreUint64(&b.capacity, capacity)
}

func (b *Bucket) Timestamp() uint64 {
	return atomic.LoadUint64(&b.ts)
}
