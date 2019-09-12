package throttle

type Throttle interface {
	SetCapacity(capacity uint64)
}
