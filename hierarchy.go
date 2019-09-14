package throttle

// Hierarchy of buckets. 2 level.
type Hierarchy struct {
	leaf Bucket

	// Fairness for a root bucket is based on the
	// idea of fair queueing. But I don't want to
	// be too fair, so I will split a whole available
	// bandwidth into a number of buckets which will
	// be given away randomly (aka round-robin).
	//
	// But as far this is very simple implementation,
	// I will do random-based fan-out which must be uniformly
	// distributed for the sake of round-robin-ness.
	// Random ordering could be provided with the means of
	// a mutex or a channel queue for which the consumers
	// are fighting, but the implementation uses the fact
	// of preemptive goroutines scheduling being fair.
	//
	// Fair queueing for the root bucket could use
	// adaptive 1s window capacity adjusting (+/- 5%) to reduce
	// overall bandwidth consumption error in a target window (X secs).
	// For the best results it could benefit from knowing
	// number and configuration of consumers at leafs. But even
	// with the static number of scheduling units being
	// an order of magnitude less than the overall capacity
	// gives good results in a windows of 15-30s which
	// is enough.
	root *Bucket
}

var _ Throttle = (*Hierarchy)(nil)
var _ Capacity = (*Hierarchy)(nil)

func NewHierarchy(root *Bucket) *Hierarchy {
	return &Hierarchy{root: root}
}

// Consume consumes required bandwidth at local bucket first
// and then requests the same from the parent. At parent level
// consumers are up to the question of fair scheduling.
func (h *Hierarchy) Consume(consume uint64) uint64 {
	if h.root == nil || h.root.Unlimited() {
		return h.leaf.Consume(consume)
	}

	consume = h.Project(consume)
	consume = h.leaf.Consume(consume)
	return h.root.Consume(consume)
}

func (h *Hierarchy) SetCapacity(capacity uint64) {
	h.leaf.SetCapacity(capacity)
}

func (h *Hierarchy) Reset() {
	h.leaf.SetFill(0)
}

// Project tries to give best estimate of the reservation
// available for a single unit of scheduling at parent level.
func (h *Hierarchy) Project(consume uint64) uint64 {
	// static estimation - overall capacity split by 16, min 1
	unit := h.root.Capacity() >> 4
	if unit == 0 {
		unit = 1
	}
	if consume > unit {
		return unit
	}
	return consume
}

func (h *Hierarchy) Leaf() *Bucket {
	return &h.leaf
}

func (h *Hierarchy) Root() *Bucket {
	return h.root
}
