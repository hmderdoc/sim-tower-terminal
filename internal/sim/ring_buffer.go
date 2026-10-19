package sim

// Port of reference/tower-together/sim/ring-buffer.ts: legacy generic
// fixed-capacity ring buffer. Kept for parity with the TS module list;
// carrier floor queues use RouteRequestRing (queue_route_record.go).

// RingBuffer TS: ring-buffer.ts RingBuffer<T>. Serializes as
// {items, head, count}.
type RingBuffer[T any] struct {
	Items    []T `json:"items"`
	Head     int `json:"head"`
	Count    int `json:"count"`
	capacity int
}

// NewRingBuffer TS: new RingBuffer(capacity, fill)
func NewRingBuffer[T any](capacity int, fill T) *RingBuffer[T] {
	items := make([]T, capacity)
	for i := range items {
		items[i] = fill
	}
	return &RingBuffer[T]{Items: items, capacity: capacity}
}

// RingBufferFrom reconstitutes a buffer from its plain-object snapshot.
// TS: RingBuffer.from
func RingBufferFrom[T any](items []T, head, count int) *RingBuffer[T] {
	return &RingBuffer[T]{Items: items, Head: head, Count: count, capacity: len(items)}
}

// Capacity / Size / IsFull / IsEmpty: TS RingBuffer getters.
func (b *RingBuffer[T]) Capacity() int { return b.capacity }
func (b *RingBuffer[T]) Size() int     { return b.Count }
func (b *RingBuffer[T]) IsFull() bool  { return b.Count >= b.capacity }
func (b *RingBuffer[T]) IsEmpty() bool { return b.Count == 0 }

// Push appends an item; false when full. TS: RingBuffer.push
func (b *RingBuffer[T]) Push(item T) bool {
	if b.Count >= b.capacity {
		return false
	}
	writeIndex := (b.Head + b.Count) % b.capacity
	b.Items[writeIndex] = item
	b.Count++
	return true
}

// Pop removes and returns the oldest item. TS: RingBuffer.pop
func (b *RingBuffer[T]) Pop() (T, bool) {
	var zero T
	if b.Count <= 0 {
		return zero, false
	}
	item := b.Items[b.Head]
	b.Head = (b.Head + 1) % b.capacity
	b.Count--
	return item, true
}

// PeekAll returns all items in queue order. TS: RingBuffer.peekAll
func (b *RingBuffer[T]) PeekAll() []T {
	out := make([]T, b.Count)
	for i := range out {
		out[i] = b.Items[(b.Head+i)%b.capacity]
	}
	return out
}
