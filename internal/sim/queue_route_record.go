package sim

// Port of reference/tower-together/sim/queue/route-record.ts.
//
// 1288:c05a TowerRouteQueueRecord (324-byte per floor-slot): per-floor
// route-request ring, two 40-entry request ref arrays. The only observable
// part is the FIFO order and the size-40 silent wrap.

const routeQueueCapacity = 40

// RouteQueueCapacityConst TS: ROUTE_QUEUE_CAPACITY_CONST
const RouteQueueCapacityConst = routeQueueCapacity

// RouteRequestRing is a fixed-40-entry ring of route request ids for one
// direction of one floor. Binary quirk: enqueue_request_into_route_queue
// (1218:1002) writes at (head+count)%40 without a capacity check, so the
// 41st enqueue clobbers the oldest entry.
// TS: queue/route-record.ts RouteRequestRing
type RouteRequestRing struct {
	Items []string `json:"items"`
	Head  int      `json:"head"`
	Count int      `json:"count"`
	// Capacity mirrors the TS readonly own property (serialized by
	// JSON.stringify); always 40.
	Capacity int `json:"capacity"`
}

// NewRouteRequestRing TS: new RouteRequestRing()
func NewRouteRequestRing() *RouteRequestRing {
	return &RouteRequestRing{Items: make([]string, routeQueueCapacity), Capacity: routeQueueCapacity}
}

// RouteRequestRingFrom reconstitutes a ring from a plain snapshot.
// TS: RouteRequestRing.from
func RouteRequestRingFrom(items []string, head, count int) *RouteRequestRing {
	buf := NewRouteRequestRing()
	n := min(routeQueueCapacity, len(items))
	copy(buf.Items, items[:n])
	buf.Head = head % routeQueueCapacity
	buf.Count = min(routeQueueCapacity, max(0, count))
	return buf
}

// Size / IsFull / IsEmpty: TS RouteRequestRing getters.
func (r *RouteRequestRing) Size() int     { return r.Count }
func (r *RouteRequestRing) IsFull() bool  { return r.Count >= routeQueueCapacity }
func (r *RouteRequestRing) IsEmpty() bool { return r.Count == 0 }

// Push appends a request id, overwriting the head on the 41st enqueue.
// Always returns true (the binary has no full flag).
// TS: RouteRequestRing.push
func (r *RouteRequestRing) Push(item string) bool {
	writeIndex := (r.Head + r.Count) % routeQueueCapacity
	r.Items[writeIndex] = item
	if r.Count < routeQueueCapacity {
		r.Count++
	} else {
		// Binary quirk: size-40 ring silently overwrites head on 41st enqueue.
		r.Head = (r.Head + 1) % routeQueueCapacity
	}
	return true
}

// Pop removes and returns the oldest id. TS: RouteRequestRing.pop
func (r *RouteRequestRing) Pop() (string, bool) {
	if r.Count <= 0 {
		return "", false
	}
	item := r.Items[r.Head]
	r.Head = (r.Head + 1) % routeQueueCapacity
	r.Count--
	return item, true
}

// PeekAll returns all ids in queue order. TS: RouteRequestRing.peekAll
func (r *RouteRequestRing) PeekAll() []string {
	out := make([]string, r.Count)
	for i := range out {
		out[i] = r.Items[(r.Head+i)%routeQueueCapacity]
	}
	return out
}

// RemoveFirst pops the first occurrence of item, preserving the order of
// the rest (compaction pass of remove_request_from_unit_queue 1218:142a).
// Like the TS it drains and re-pushes, so head advances by the drained count.
// TS: RouteRequestRing.removeFirst
func (r *RouteRequestRing) RemoveFirst(item string) bool {
	if r.Count == 0 {
		return false
	}
	var remaining []string
	found := false
	for !r.IsEmpty() {
		popped, _ := r.Pop()
		if !found && popped == item {
			found = true
			continue
		}
		remaining = append(remaining, popped)
	}
	for _, id := range remaining {
		r.Push(id)
	}
	return found
}
