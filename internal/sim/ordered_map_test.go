package sim

import (
	"encoding/json"
	"reflect"
	"testing"
)

func TestOrderedMapJSSemantics(t *testing.T) {
	var m OrderedMap[string, int]
	m.Set("b", 1)
	m.Set("a", 2)
	m.Set("c", 3)
	m.Set("a", 20) // keeps position
	if got := m.Keys(); !reflect.DeepEqual(got, []string{"b", "a", "c"}) {
		t.Fatalf("keys %v", got)
	}
	m.Delete("b")
	m.Set("b", 5) // re-insert appends
	if got := m.Keys(); !reflect.DeepEqual(got, []string{"a", "c", "b"}) {
		t.Fatalf("keys after delete/set %v", got)
	}
	if v, ok := m.Get("a"); !ok || v != 20 {
		t.Fatalf("get a = %v %v", v, ok)
	}
	// Range iterates a snapshot: mutations do not change what is visited.
	var seen []string
	m.Range(func(k string, _ int) bool {
		seen = append(seen, k)
		m.Delete("c")
		m.Set("z", 9)
		return true
	})
	if !reflect.DeepEqual(seen, []string{"a", "c", "b"}) {
		t.Fatalf("range %v", seen)
	}
	if got := m.Keys(); !reflect.DeepEqual(got, []string{"a", "b", "z"}) {
		t.Fatalf("keys after range %v", got)
	}
}

func TestOrderedMapCompactionKeepsOrder(t *testing.T) {
	var m OrderedMap[string, int]
	for i := 0; i < 200; i++ {
		m.Set(cellKey(i, 0), i)
	}
	for i := 0; i < 200; i += 2 {
		m.Delete(cellKey(i, 0))
	}
	m.Set("tail", -1)
	keys := m.Keys()
	if len(keys) != 101 || keys[0] != "1,0" || keys[99] != "199,0" || keys[100] != "tail" {
		t.Fatalf("bad order after compaction: len=%d first=%s last=%s", len(keys), keys[0], keys[len(keys)-1])
	}
	if m.At("101,0") != 101 {
		t.Fatalf("lookup after compaction failed")
	}
}

func TestOrderedMapJSONRoundTrip(t *testing.T) {
	var m OrderedMap[string, string]
	m.Set("9,1", "x")
	m.Set("10,1", "y")
	m.Set("2,1", "z")
	b, err := json.Marshal(m)
	if err != nil {
		t.Fatal(err)
	}
	if string(b) != `{"9,1":"x","10,1":"y","2,1":"z"}` {
		t.Fatalf("json %s", b)
	}
	var back OrderedMap[string, string]
	if err := json.Unmarshal(b, &back); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(back.Keys(), m.Keys()) {
		t.Fatalf("order lost: %v", back.Keys())
	}
	var im OrderedMap[int, int]
	if err := json.Unmarshal([]byte(`{"3":1,"1":2}`), &im); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(im.Keys(), []int{3, 1}) {
		t.Fatalf("int keys %v", im.Keys())
	}
}
