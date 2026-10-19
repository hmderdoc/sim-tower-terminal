package sim

import (
	"bytes"
	"encoding/json"
	"fmt"
	"strconv"
)

// OrderedMap is an insertion-ordered map, the Go stand-in for a JS object
// with non-integer string keys (or a JS Map) whose iteration order is
// simulation state (PORT.md rule 3).
//
// Semantics mirror JS exactly:
//   - Set on an existing key replaces the value and KEEPS the key's position.
//   - Delete removes the key; the remaining keys keep their relative order.
//   - Set after Delete appends the key at the end.
//   - Range / Keys / Values / Entries iterate a snapshot taken at call time,
//     like Object.entries / Object.keys / Object.values. Mutating the map
//     inside the callback does not change what is visited.
//
// The zero value is ready to use. Methods have pointer receivers, so keep
// OrderedMap fields addressable (struct fields, not map values).
//
// Implementation: entries slice with tombstones plus key->position index;
// tombstones are compacted when they outnumber live entries, so Delete is
// amortized O(1) instead of an O(n) slice shift.
type OrderedMap[K comparable, V any] struct {
	entries []omEntry[K, V]
	index   map[K]int
	dead    int
}

type omEntry[K comparable, V any] struct {
	key  K
	val  V
	live bool
}

// NewOrderedMap returns an empty map.
func NewOrderedMap[K comparable, V any]() *OrderedMap[K, V] {
	return &OrderedMap[K, V]{}
}

// Len returns the number of live keys.
func (m *OrderedMap[K, V]) Len() int {
	return len(m.entries) - m.dead
}

// Get returns the value for k and whether it is present.
func (m *OrderedMap[K, V]) Get(k K) (V, bool) {
	if m.index != nil {
		if i, ok := m.index[k]; ok {
			return m.entries[i].val, true
		}
	}
	var zero V
	return zero, false
}

// At returns the value for k, or the zero value when absent (JS `obj[k]`
// returning undefined, collapsed to the zero value).
func (m *OrderedMap[K, V]) At(k K) V {
	v, _ := m.Get(k)
	return v
}

// Has reports whether k is present.
func (m *OrderedMap[K, V]) Has(k K) bool {
	if m.index == nil {
		return false
	}
	_, ok := m.index[k]
	return ok
}

// Set inserts or replaces k. Existing keys keep their position.
func (m *OrderedMap[K, V]) Set(k K, v V) {
	if m.index == nil {
		m.index = make(map[K]int)
	}
	if i, ok := m.index[k]; ok {
		m.entries[i].val = v
		return
	}
	m.index[k] = len(m.entries)
	m.entries = append(m.entries, omEntry[K, V]{key: k, val: v, live: true})
}

// Delete removes k (no-op when absent).
func (m *OrderedMap[K, V]) Delete(k K) {
	if m.index == nil {
		return
	}
	i, ok := m.index[k]
	if !ok {
		return
	}
	delete(m.index, k)
	var zero omEntry[K, V]
	m.entries[i] = zero
	m.dead++
	if m.dead > 32 && m.dead*2 > len(m.entries) {
		m.compact()
	}
}

func (m *OrderedMap[K, V]) compact() {
	out := make([]omEntry[K, V], 0, len(m.entries)-m.dead)
	for _, e := range m.entries {
		if e.live {
			m.index[e.key] = len(out)
			out = append(out, e)
		}
	}
	m.entries = out
	m.dead = 0
}

// Clear removes every key.
func (m *OrderedMap[K, V]) Clear() {
	m.entries = nil
	m.index = nil
	m.dead = 0
}

// Keys returns a snapshot of the keys in insertion order.
func (m *OrderedMap[K, V]) Keys() []K {
	out := make([]K, 0, m.Len())
	for _, e := range m.entries {
		if e.live {
			out = append(out, e.key)
		}
	}
	return out
}

// Values returns a snapshot of the values in insertion order.
func (m *OrderedMap[K, V]) Values() []V {
	out := make([]V, 0, m.Len())
	for _, e := range m.entries {
		if e.live {
			out = append(out, e.val)
		}
	}
	return out
}

// OrderedEntry is one key/value pair of an Entries snapshot.
type OrderedEntry[K comparable, V any] struct {
	Key K
	Val V
}

// Entries returns a snapshot of the pairs in insertion order
// (JS Object.entries).
func (m *OrderedMap[K, V]) Entries() []OrderedEntry[K, V] {
	out := make([]OrderedEntry[K, V], 0, m.Len())
	for _, e := range m.entries {
		if e.live {
			out = append(out, OrderedEntry[K, V]{e.key, e.val})
		}
	}
	return out
}

// Range calls fn for each pair of a snapshot taken at call time, in
// insertion order, stopping when fn returns false.
func (m *OrderedMap[K, V]) Range(fn func(k K, v V) bool) {
	for _, e := range m.Entries() {
		if !fn(e.Key, e.Val) {
			return
		}
	}
}

// Clone returns a shallow copy (values copied by assignment) preserving order
// (JS `{ ...obj }`).
func (m *OrderedMap[K, V]) Clone() OrderedMap[K, V] {
	var out OrderedMap[K, V]
	for _, e := range m.entries {
		if e.live {
			out.Set(e.key, e.val)
		}
	}
	return out
}

// MarshalJSON writes a JSON object whose member order is the insertion order.
// Keys must be strings or integers.
func (m OrderedMap[K, V]) MarshalJSON() ([]byte, error) {
	var buf bytes.Buffer
	buf.WriteByte('{')
	first := true
	for _, e := range m.entries {
		if !e.live {
			continue
		}
		if !first {
			buf.WriteByte(',')
		}
		first = false
		var ks string
		switch k := any(e.key).(type) {
		case string:
			ks = k
		case int:
			ks = strconv.Itoa(k)
		default:
			return nil, fmt.Errorf("OrderedMap: unsupported key type %T", e.key)
		}
		kb, err := json.Marshal(ks)
		if err != nil {
			return nil, err
		}
		buf.Write(kb)
		buf.WriteByte(':')
		vb, err := json.Marshal(e.val)
		if err != nil {
			return nil, err
		}
		buf.Write(vb)
	}
	buf.WriteByte('}')
	return buf.Bytes(), nil
}

// UnmarshalJSON reads a JSON object, keeping member order. `null` yields an
// empty map.
func (m *OrderedMap[K, V]) UnmarshalJSON(data []byte) error {
	m.Clear()
	dec := json.NewDecoder(bytes.NewReader(data))
	tok, err := dec.Token()
	if err != nil {
		return err
	}
	if tok == nil {
		return nil
	}
	if d, ok := tok.(json.Delim); !ok || d != '{' {
		return fmt.Errorf("OrderedMap: expected object, got %v", tok)
	}
	for dec.More() {
		kt, err := dec.Token()
		if err != nil {
			return err
		}
		ks, ok := kt.(string)
		if !ok {
			return fmt.Errorf("OrderedMap: bad key %v", kt)
		}
		var key K
		switch p := any(&key).(type) {
		case *string:
			*p = ks
		case *int:
			n, err := strconv.Atoi(ks)
			if err != nil {
				return fmt.Errorf("OrderedMap: bad int key %q", ks)
			}
			*p = n
		default:
			return fmt.Errorf("OrderedMap: unsupported key type %T", key)
		}
		var val V
		if err := dec.Decode(&val); err != nil {
			return err
		}
		m.Set(key, val)
	}
	_, err = dec.Token() // closing '}'
	return err
}
