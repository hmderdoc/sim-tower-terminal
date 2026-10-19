package sim

// Port of reference/tower-together/sim/queue/dequeue.ts.
//
// 1218:1172 pop_unit_queue_request. The binary follows the pop with
// reduce_elapsed_for_lobby_boarding; upstream runs that inline elsewhere.

// PopUnitQueueRequest pops the head of the ring for the direction. The
// bool is false on an empty ring (TS undefined).
// TS: queue/dequeue.ts popUnitQueueRequest (binary 1218:1172)
func PopUnitQueueRequest(queue *CarrierFloorQueue, directionFlag int) (string, bool) {
	ring := queue.Down
	if directionFlag == 1 {
		ring = queue.Up
	}
	return ring.Pop()
}
