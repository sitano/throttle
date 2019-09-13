package throttle

type Throttle interface {
	Consume(consume uint64) uint64
}

type Capacity interface {
	SetCapacity(capacity uint64)
	Reset()
}
