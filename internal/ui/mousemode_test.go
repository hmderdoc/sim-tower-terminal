package ui

import (
	"bytes"
	"strings"
	"testing"
	"time"

	"simtower/internal/term"
)

type sinkTerm struct{ bytes.Buffer }

func (s *sinkTerm) Read(p []byte) (int, error)                         { select {} }
func (s *sinkTerm) ReadTimeout(p []byte, d time.Duration) (int, error) { return 0, nil }
func (s *sinkTerm) Close() error                                       { return nil }

// Dropping a build tool must leave click tracking on: SyncTERM has a single
// tracking mode, and a bare "?1003l" switches its mouse off completely.
func TestDisarmKeepsClickTracking(t *testing.T) {
	d := NewDemo()
	sink := &sinkTerm{}
	a := New(Config{Cols: 80, Rows: 24, Model: d, Term: sink})
	a.render()
	a.arm(d.Catalog()[0])
	if !strings.Contains(sink.String(), "\x1b[?1003h") {
		t.Fatalf("arm did not enable any-motion tracking: %q", sink.String())
	}
	sink.Reset()
	a.disarm()
	out := sink.String()
	off := strings.Index(out, "\x1b[?1003l")
	on := strings.Index(out, term.MouseOn)
	if off < 0 || on < 0 || on < off {
		t.Fatalf("disarm must reset 1003 then re-select click tracking, got %q", out)
	}
}
