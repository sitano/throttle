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

var _ Throttle = (*Bucket)(nil)
var _ Capacity = (*Bucket)(nil)

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
// It is implemented with atomics because I wanted to play with
// lock free bucket, not because it is faster for a single
// consumer.
func (b *Bucket) Consume(consume uint64) uint64 {
	var capacity = atomic.LoadUint64(&b.capacity)
	var consumed bool
	var fill uint64

	if capacity == 0 {
		return consume
	}

	if consume > capacity {
		consume = capacity
	}

	for !consumed {
		// update last update point
		var prev = atomic.LoadUint64(&b.ts)
		var now = uint64(time.Now().UnixNano())
		// fix time not to drop last piece of update
		now = prev + ((now-prev)/uint64(time.Millisecond))*uint64(time.Millisecond)
		if !atomic.CompareAndSwapUint64(&b.ts, prev, now) {
			continue
		}

		// generate tokens past last timestamp
		if now-prev >= uint64(time.Second) {
			// forgive possible race condition
			atomic.StoreUint64(&b.fill, 0)
			fill = 0
		} else {
			var deltaMS = (now - prev) / uint64(time.Millisecond)
			var tokens = capacity * deltaMS / 1000
			if tokens > 0 {
				for {
					fill = atomic.LoadUint64(&b.fill)
					if tokens >= fill {
						// forgive possible race condition
						atomic.StoreUint64(&b.fill, 0)
						fill = 0
						break
					} else if atomic.CompareAndSwapUint64(&b.fill, fill, fill-tokens) {
						fill -= tokens
						break
					}
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
				var wait = 1000 * (consume - free) / capacity
				if wait < 1 {
					wait = 1
				}
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

func (b *Bucket) Unlimited() bool {
	return atomic.LoadUint64(&b.capacity) == 0
}

func (b *Bucket) Available() uint64 {
	c := b.Capacity()
	f := b.Fill()
	if f > c {
		return 0
	}
	return c - f
}

func (b *Bucket) SetCapacity(capacity uint64) {
	atomic.StoreUint64(&b.capacity, capacity)
	if capacity == 0 {
		atomic.StoreUint64(&b.fill, 0)
	}
}

func (b *Bucket) SetFill(fill uint64) {
	atomic.StoreUint64(&b.fill, fill)
	atomic.StoreUint64(&b.ts, uint64(time.Now().UnixNano()))
}

func (b *Bucket) Reset() {
	b.SetFill(0)
}

func (b *Bucket) Timestamp() uint64 {
	return atomic.LoadUint64(&b.ts)
}
