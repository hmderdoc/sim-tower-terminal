package screen

import (
	"bytes"
	"strings"
	"testing"
)

func TestDiffFlush(t *testing.T) {
	s := New(10, 3)
	var b bytes.Buffer
	s.Put(0, 0, "hello", 15, 1)
	if err := s.Flush(&b); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(b.String(), "\x1b[2J") || !strings.Contains(b.String(), "hello") {
		t.Fatalf("first flush: %q", b.String())
	}
	b.Reset()
	s.Flush(&b)
	if b.Len() != 0 {
		t.Fatalf("no-change flush emitted %q", b.String())
	}
	s.Put(2, 0, "L", 15, 1)
	b.Reset()
	s.Flush(&b)
	out := b.String()
	if !strings.Contains(out, "\x1b[1;3H") || strings.Count(out, "L") != 1 || strings.Contains(out, "h") {
		t.Fatalf("partial flush: %q", out)
	}
}
