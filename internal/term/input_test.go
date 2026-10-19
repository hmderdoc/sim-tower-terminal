package term

import "testing"

func TestDecodeMouse(t *testing.T) {
	ev, ok := decodeCSI([]byte("<0;12;5"), 'M')
	if !ok || !ev.Mouse || ev.MKind != MousePress || ev.MX != 12 || ev.MY != 5 || ev.Button != 0 {
		t.Fatalf("press: %+v %v", ev, ok)
	}
	ev, _ = decodeCSI([]byte("<2;3;4"), 'm')
	if ev.MKind != MouseRelease || ev.Button != 2 {
		t.Fatalf("release: %+v", ev)
	}
	ev, _ = decodeCSI([]byte("<32;9;9"), 'M')
	if ev.MKind != MouseMotion || ev.Button != 0 {
		t.Fatalf("motion: %+v", ev)
	}
	ev, _ = decodeCSI([]byte("<64;1;1"), 'M')
	if ev.MKind != MouseWheelUp {
		t.Fatalf("wheel up: %+v", ev)
	}
	ev, _ = decodeCSI([]byte("<65;1;1"), 'M')
	if ev.MKind != MouseWheelDown {
		t.Fatalf("wheel down: %+v", ev)
	}
	ev, _ = decodeCSI([]byte("5"), '~')
	if ev.Kind != KeyPgUp {
		t.Fatalf("pgup: %+v", ev)
	}
	ev, _ = decodeCSI([]byte("24;80"), 'R')
	if !ev.Resize || ev.Cols != 80 || ev.Rows != 24 {
		t.Fatalf("cpr: %+v", ev)
	}
	ev, _ = decodeCSI([]byte("1;2"), 'A')
	if ev.Kind != KeyUp || !ev.Shift {
		t.Fatalf("shift-up: %+v", ev)
	}
	ev, _ = decodeCSI([]byte("1;2"), 'R')
	if ev.Resize || ev.Kind != KeyF3 {
		t.Fatalf("shift-f3: %+v", ev)
	}
}
