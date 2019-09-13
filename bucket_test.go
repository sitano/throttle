package throttle

import (
	"math"
	"math/rand"
	"testing"
	"time"
)

func TestBucket_Consume(t *testing.T) {
	t.Run("returns immediately for unlimited bucket", func(t *testing.T) {
		b := NewBucket(0)
		assertEqU64(t, b.Consume(0), 0)
		assertEqU64(t, b.Consume(1), 1)
		assertEqU64(t, b.Consume(100), 100)
		assertEqU64(t, b.capacity, 0)
		assertEqU64(t, b.fill, 0)
		assertEqU64(t, b.ts, 0)
	})

	t.Run("generates 1 token evenly", func(t *testing.T) {
		t.Parallel()
		b := NewBucket(1)
		assertConsumeMax(t, b, 1, 1, time.Millisecond)
		assertConsumeMin(t, b, 1, 1, time.Second)
	})

	t.Run("does not allow over feeding", func(t *testing.T) {
		t.Parallel()
		b := NewBucket(1)
		assertEqU64(t, b.Consume(2), 1)
		assertConsumeMin(t, b, 2, 1, time.Second)
	})

	t.Run("capacity at once", func(t *testing.T) {
		t.Parallel()
		b := NewBucket(10)

		assertConsumeMax(t, b, 100, 10, time.Millisecond, "consume all tokens first")
		assertEqU64(t, b.fill, b.capacity)

		assertConsumeMax(t, b, 100, 10, time.Second)
		assertEqU64(t, b.fill, b.capacity)
	})

	t.Run("artificial timeout generates tokens", func(t *testing.T) {
		t.Parallel()
		b := NewBucket(10)

		assertConsumeMax(t, b, 100, 10, time.Millisecond, "consume all tokens first")
		assertEqU64(t, b.fill, b.capacity)

		assertConsumeMax(t, b, 1, 1, 100*time.Millisecond)
		assertEqU64(t, b.fill, b.capacity)

		time.Sleep(100 * time.Millisecond)
		assertConsumeMax(t, b, 1, 1, time.Millisecond)
		assertEqU64(t, b.fill, b.capacity)

		time.Sleep(200 * time.Millisecond)
		assertConsumeMax(t, b, 1, 1, time.Millisecond)
		assertEqU64(t, b.fill, b.capacity-1)

		assertConsumeMax(t, b, 1, 1, time.Millisecond)
		assertEqU64(t, b.fill, b.capacity)
	})

	t.Run("all twice", func(t *testing.T) {
		t.Parallel()
		b := NewBucket(1000)

		assertConsumeMax(t, b, 2000, 1000, time.Millisecond, "consume all tokens first")
		assertEqU64(t, b.fill, b.capacity)
		assertConsumeMax(t, b, 2000, 1000, time.Second)
		assertEqU64(t, b.fill, b.capacity)
	})

	t.Run("flat/5s", func(t *testing.T) {
		const window = 5 * time.Second
		const bandwidth = 1000 // bytes / sec

		t.Parallel()
		b := NewBucket(1000)

		assertConsumeMax(t, b, 2*bandwidth, bandwidth, time.Millisecond, "consume all tokens first")

		consumed := uint64(0)
		start := time.Now()
		for {
			assertConsumeMax(t, b, 10, 10, 10*time.Millisecond)
			consumed += 10
			// t.Log("consumed =", consumed, ", dt =", time.Since(start))
			if time.Since(start) >= window {
				break
			}
		}

		dt := time.Since(start)
		projected := uint64(bandwidth * dt / time.Second)
		accuracy := float64(consumed)/float64(projected) - 1.0
		t.Log("total consumption =", consumed,
			"projected =", projected,
			"error =", accuracy, "in =", dt)

		if math.Abs(accuracy) > 0.01 {
			t.Error("something went wrong with accuracy:",
				"total consumption =", consumed,
				"projected =", projected,
				"error =", accuracy, "in =", dt)
		}
	})

	t.Run("rnd/5s", func(t *testing.T) {
		const window = 5 * time.Second
		const bandwidth = 1000 // bytes / sec

		t.Parallel()
		b := NewBucket(bandwidth)

		assertConsumeMax(t, b, 2*bandwidth, bandwidth, time.Millisecond, "consume all tokens first")

		consumed := uint64(0)
		start := time.Now()
		for {
			try := uint64(rand.Intn(2 * bandwidth))
			dc := b.Consume(try)
			if dc < 1 {
				t.Error("invalid consume")
			}
			consumed += dc
			// t.Log("consumed =", consumed, ", dt =", time.Since(start))
			if time.Since(start) >= window {
				break
			}
		}

		dt := time.Since(start)
		projected := uint64(bandwidth * dt / time.Second)
		accuracy := float64(consumed)/float64(projected) - 1.0
		t.Log("total consumption =", consumed,
			"projected =", projected,
			"error =", accuracy, "in =", dt)

		if math.Abs(accuracy) > 0.01 {
			t.Error("something went wrong with accuracy:",
				"total consumption =", consumed,
				"projected =", projected,
				"error =", accuracy, "in =", dt)
		}
	})
}

func TestBucket_SetCapacity(t *testing.T) {
	t.Run("change works", func(t *testing.T) {
		b := NewBucket(0)
		assertConsumeMax(t, b, 1000, 1000, time.Millisecond)
		b.SetCapacity(1)
		assertConsumeMax(t, b, 1, 1, time.Millisecond)
		assertConsumeMin(t, b, 1, 1, time.Second)
	})

	t.Run("reset works", func(t *testing.T) {
		b := NewBucket(1000)
		assertConsumeMax(t, b, 1000, 1000, time.Millisecond)
		assertConsumeMin(t, b, 100, 100, 100*time.Millisecond)
		b.SetCapacity(0)
		assertConsumeMax(t, b, 1000, 1000, time.Millisecond)
	})
}

func assertConsumeMin(t *testing.T, b *Bucket, consume uint64, expected uint64, in time.Duration, msg ...interface{}) {
	start := time.Now()
	consumed := b.Consume(consume)
	if consumed != expected {
		t.Error(append([]interface{}{"assert consume: ", consumed, "!=", expected}, msg...)...)
	}
	if time.Since(start) <= in {
		t.Error(append([]interface{}{"assert consume time: ", time.Since(start), ">", in}, msg...)...)
	}
}

func assertConsumeMax(t *testing.T, b *Bucket, consume uint64, expected uint64, in time.Duration, msg ...interface{}) {
	start := time.Now()
	consumed := b.Consume(consume)
	if consumed != expected {
		t.Error(append([]interface{}{"assert consume: ", consumed, "!=", expected}, msg...)...)
	}
	// add 2 ms for scheduling overhead
	if time.Since(start) > 2*in {
		t.Error(append([]interface{}{"assert consume time: ", time.Since(start), ">", in}, msg...)...)
	}
}

func assertEqU64(t *testing.T, val, expected uint64, msg ...interface{}) {
	if val != expected {
		t.Error(append([]interface{}{"assert uint64: ", val, "!=", expected}, msg...)...)
	}
}
