package throttle

// Hierarchy of buckets. 2 level.
type Hierarchy struct {
	b Bucket
	p *Bucket
	// TODO: fairness queueing for top level
}

var _ Throttle = (*Hierarchy)(nil)
var _ Capacity = (*Hierarchy)(nil)

func NewHierarchy(parent *Bucket) *Hierarchy {
	return &Hierarchy{p: parent}
}

// Consume consumes required bandwidth at local bucket first
// and then requests the same from the parent. At parent level
// consumers are up to the question of fair scheduling.
func (h *Hierarchy) Consume(consume uint64) uint64 {
	if h.p == nil || h.p.Unlimited() {
		return h.b.Consume(consume)
	}

	consume = h.Project(consume)
	consume = h.b.Consume(consume)
	return h.p.Consume(consume)
}

func (h *Hierarchy) SetCapacity(capacity uint64) {
	h.b.SetCapacity(capacity)
}

// Project tries to give best estimate of the reservation
// available for a single unit of scheduling at parent level.
func (h *Hierarchy) Project(consume uint64) uint64 {
	return consume
}

func (h *Hierarchy) Leaf() *Bucket {
	return &h.b
}

func (h *Hierarchy) Root() *Bucket {
	return h.p
}
