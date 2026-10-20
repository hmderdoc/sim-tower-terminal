package term

import (
	"io"
	"testing"
	"time"
)

type scriptTerm struct {
	chunks [][]byte
}

func (s *scriptTerm) Read(p []byte) (int, error) {
	if len(s.chunks) == 0 {
		return 0, io.EOF
	}
	n := copy(p, s.chunks[0])
	s.chunks = s.chunks[1:]
	return n, nil
}
func (s *scriptTerm) ReadTimeout(p []byte, d time.Duration) (int, error) { return 0, nil }
func (s *scriptTerm) Write(p []byte) (int, error)                        { return len(p), nil }
func (s *scriptTerm) Close() error                                       { return nil }

// Telnet NVT Enter arrives as CR LF or CR NUL, possibly split across reads;
// each must become exactly one Enter. A bare LF is still Enter.
func TestReadLoopEnterOnce(t *testing.T) {
	st := &scriptTerm{chunks: [][]byte{[]byte("a\r\n"), []byte("\r"), []byte("\x00b"), []byte("\n"), []byte("\r\rc")}}
	ch := make(chan Event, 32)
	ReadLoop(st, ch)
	var got []Event
	for ev := range ch {
		got = append(got, ev)
		if ev.Disconnect {
			break
		}
	}
	want := []Event{
		{Kind: KeyChar, Ch: 'a'}, {Kind: KeyEnter},
		{Kind: KeyEnter}, {Kind: KeyChar, Ch: 'b'},
		{Kind: KeyEnter},
		{Kind: KeyEnter}, {Kind: KeyEnter}, {Kind: KeyChar, Ch: 'c'},
		{Disconnect: true},
	}
	if len(got) != len(want) {
		t.Fatalf("got %d events %+v, want %d", len(got), got, len(want))
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("event %d: got %+v want %+v", i, got[i], want[i])
		}
	}
}
