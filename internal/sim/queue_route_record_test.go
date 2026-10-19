package sim

import (
	"reflect"
	"strconv"
	"testing"
)

// Port of queue/route-record.test.ts.

func TestRouteRequestRingFIFO40(t *testing.T) {
	ring := NewRouteRequestRing()
	for i := 0; i < 40; i++ {
		ring.Push("r" + strconv.Itoa(i))
	}
	if ring.Size() != 40 || ring.PeekAll()[0] != "r0" || ring.PeekAll()[39] != "r39" {
		t.Fatalf("bad ring %v", ring.PeekAll())
	}
}

func TestRouteRequestRingOverwritesHeadOn41st(t *testing.T) {
	ring := NewRouteRequestRing()
	for i := 0; i < 40; i++ {
		ring.Push("r" + strconv.Itoa(i))
	}
	ring.Push("r40")
	items := ring.PeekAll()
	if ring.Size() != 40 || items[0] != "r1" || items[39] != "r40" {
		t.Fatalf("bad wrap %v", items)
	}
	ring.Push("r41")
	if ring.PeekAll()[0] != "r2" || ring.PeekAll()[39] != "r41" {
		t.Fatalf("bad second wrap %v", ring.PeekAll())
	}
}

func TestRouteRequestRingPopAfterWrap(t *testing.T) {
	ring := NewRouteRequestRing()
	for i := 0; i < 41; i++ {
		ring.Push("r" + strconv.Itoa(i))
	}
	var popped []string
	for !ring.IsEmpty() {
		item, _ := ring.Pop()
		popped = append(popped, item)
	}
	if len(popped) != 40 || popped[0] != "r1" || popped[39] != "r40" {
		t.Fatalf("bad pops %v", popped)
	}
}

func TestRouteRequestRingRemoveFirst(t *testing.T) {
	ring := NewRouteRequestRing()
	for _, id := range []string{"a", "b", "c", "d"} {
		ring.Push(id)
	}
	if !ring.RemoveFirst("c") || !reflect.DeepEqual(ring.PeekAll(), []string{"a", "b", "d"}) {
		t.Fatalf("removeFirst c: %v", ring.PeekAll())
	}
	if ring.RemoveFirst("z") {
		t.Fatal("removed missing id")
	}
}

func TestRouteRequestRingRemoveFirstFromAdvancedHead(t *testing.T) {
	ring := NewRouteRequestRing()
	for i := 0; i < 8; i++ {
		ring.Push("r" + strconv.Itoa(i))
	}
	for i := 0; i < 5; i++ {
		ring.Pop()
	}
	if ring.Head != 5 {
		t.Fatalf("head %d", ring.Head)
	}
	ring.Push("r8")
	ring.Push("r9")
	if !reflect.DeepEqual(ring.PeekAll(), []string{"r5", "r6", "r7", "r8", "r9"}) {
		t.Fatalf("peek %v", ring.PeekAll())
	}
	if !ring.RemoveFirst("r7") {
		t.Fatal("r7 not removed")
	}
	if ring.Head != 10 || !reflect.DeepEqual(ring.PeekAll(), []string{"r5", "r6", "r8", "r9"}) {
		t.Fatalf("head %d peek %v", ring.Head, ring.PeekAll())
	}
}

func TestRingBufferBasics(t *testing.T) {
	b := NewRingBuffer(3, "")
	b.Push("a")
	b.Push("b")
	b.Push("c")
	if b.Push("d") {
		t.Fatal("push into full buffer succeeded")
	}
	v, _ := b.Pop()
	b.Push("d")
	if v != "a" || !reflect.DeepEqual(b.PeekAll(), []string{"b", "c", "d"}) {
		t.Fatalf("ring buffer %v %v", v, b.PeekAll())
	}
}
