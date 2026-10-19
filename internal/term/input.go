package term

import (
	"strconv"
	"strings"
	"time"
)

// Key names for non-character keys.
type KeyKind int

const (
	KeyChar KeyKind = iota
	KeyUp
	KeyDown
	KeyLeft
	KeyRight
	KeyPgUp
	KeyPgDn
	KeyHome
	KeyEnd
	KeyEnter
	KeyEsc
	KeyTab
	KeyBackspace
	KeyDelete
	KeyInsert
	KeyF1 // F1..F12 are contiguous
	KeyF2
	KeyF3
	KeyF4
	KeyF5
	KeyF6
	KeyF7
	KeyF8
	KeyF9
	KeyF10
	KeyF11
	KeyF12
)

// Mouse event kinds.
type MouseKind int

const (
	MousePress MouseKind = iota
	MouseRelease
	MouseMotion // with a button held (1002) or any motion (1003)
	MouseWheelUp
	MouseWheelDown
)

// Event is one decoded input event. Exactly one of the groups is meaningful:
// Key events (Kind/Ch), Mouse events (Mouse=true, MX/MY 1-based, Button 0/1/2),
// Resize (Resize=true, Cols/Rows), or Disconnect.
type Event struct {
	Kind KeyKind
	Ch   byte // for KeyChar

	Mouse  bool
	MKind  MouseKind
	MX, MY int
	Button int  // 0 left, 1 middle, 2 right (press/release/motion)
	Shift  bool // modifier bits from the SGR report
	Ctrl   bool
	Alt    bool

	Resize     bool
	Cols, Rows int

	Disconnect bool
}

const (
	MouseOn  = "\x1b[?1000;1002;1006h" // click + drag reports, SGR encoding
	MouseOff = "\x1b[?1000;1002;1006l"
)

// escTimeout: wait for a byte after a lone Esc before calling it a real Esc.
const escTimeout = 60 * time.Millisecond

// ReadLoop decodes bytes from t into events on ch until the connection drops.
// It must be the only reader of t once started.
func ReadLoop(t Term, ch chan<- Event) {
	buf := make([]byte, 256)
	for {
		n, err := t.Read(buf)
		if err != nil {
			ch <- Event{Disconnect: true}
			return
		}
		for i := 0; i < n; i++ {
			c := buf[i]
			if c != 27 {
				ch <- plainKey(c)
				continue
			}
			nb, ok := escNext(t, buf, n, &i)
			if !ok {
				ch <- Event{Kind: KeyEsc}
				continue
			}
			switch nb {
			case '[':
				// CSI: collect parameter/intermediate bytes until a final byte 0x40-0x7E
				var params []byte
				var final byte
				for {
					b, ok := escNext(t, buf, n, &i)
					if !ok {
						break
					}
					if b >= 0x40 && b <= 0x7E && !(b == '<' || b == '?' || b == '=') {
						final = b
						break
					}
					params = append(params, b)
				}
				if final == 0 {
					continue
				}
				if ev, ok := decodeCSI(params, final); ok {
					ch <- ev
				}
			case 'O':
				b, ok := escNext(t, buf, n, &i)
				if !ok {
					ch <- Event{Kind: KeyEsc}
					continue
				}
				switch b {
				case 'A':
					ch <- Event{Kind: KeyUp}
				case 'B':
					ch <- Event{Kind: KeyDown}
				case 'C':
					ch <- Event{Kind: KeyRight}
				case 'D':
					ch <- Event{Kind: KeyLeft}
				case 'H':
					ch <- Event{Kind: KeyHome}
				case 'F':
					ch <- Event{Kind: KeyEnd}
				case 'M':
					ch <- Event{Kind: KeyEnter}
				case 'P', 'Q', 'R', 'S':
					ch <- Event{Kind: KeyF1 + KeyKind(b-'P')}
				}
			default:
				// Alt+key or a stray Esc followed by a char: treat as Esc then char.
				ch <- Event{Kind: KeyEsc}
				ch <- plainKey(nb)
			}
		}
	}
}

func plainKey(c byte) Event {
	switch c {
	case 13, 10:
		return Event{Kind: KeyEnter}
	case 9:
		return Event{Kind: KeyTab}
	case 8, 127:
		return Event{Kind: KeyBackspace}
	}
	return Event{Kind: KeyChar, Ch: c}
}

func escNext(t Term, buf []byte, n int, i *int) (byte, bool) {
	if *i+1 < n {
		*i++
		return buf[*i], true
	}
	var pb [1]byte
	if m, _ := t.ReadTimeout(pb[:], escTimeout); m > 0 {
		return pb[0], true
	}
	return 0, false
}

func decodeCSI(params []byte, final byte) (Event, bool) {
	ev, ok := decodeCSIBase(params, final)
	if ok && !ev.Mouse && !ev.Resize {
		// xterm modifier parameter: CSI 1;m A, CSI 5;m ~ (m-1 = shift|alt<<1|ctrl<<2)
		if f := strings.Split(string(params), ";"); len(f) == 2 {
			if m, err := strconv.Atoi(f[1]); err == nil && m >= 2 {
				m--
				ev.Shift = ev.Shift || m&1 != 0
				ev.Alt = m&2 != 0
				ev.Ctrl = m&4 != 0
			}
		}
	}
	return ev, ok
}

func decodeCSIBase(params []byte, final byte) (Event, bool) {
	p := string(params)
	switch final {
	case 'A':
		return Event{Kind: KeyUp}, true
	case 'B':
		return Event{Kind: KeyDown}, true
	case 'C':
		return Event{Kind: KeyRight}, true
	case 'D':
		return Event{Kind: KeyLeft}, true
	case 'H':
		return Event{Kind: KeyHome}, true
	case 'F':
		return Event{Kind: KeyEnd}, true
	case 'Z':
		return Event{Kind: KeyTab, Shift: true}, true
	case '~':
		n, _ := strconv.Atoi(strings.SplitN(p, ";", 2)[0])
		switch n {
		case 1, 7:
			return Event{Kind: KeyHome}, true
		case 2:
			return Event{Kind: KeyInsert}, true
		case 3:
			return Event{Kind: KeyDelete}, true
		case 4, 8:
			return Event{Kind: KeyEnd}, true
		case 5:
			return Event{Kind: KeyPgUp}, true
		case 6:
			return Event{Kind: KeyPgDn}, true
		case 11, 12, 13, 14, 15:
			return Event{Kind: KeyF1 + KeyKind(n-11)}, true
		case 17, 18, 19, 20, 21:
			return Event{Kind: KeyF6 + KeyKind(n-17)}, true
		case 23, 24:
			return Event{Kind: KeyF11 + KeyKind(n-23)}, true
		}
	case 'R':
		// cursor position report (size probe reply): rows;cols. "1;m R" is
		// modified F3 on xterm-style terminals (no real terminal has 1 row).
		f := strings.Split(p, ";")
		if len(f) == 2 && f[0] == "1" {
			return Event{Kind: KeyF3}, true
		}
		if len(f) == 2 {
			r, _ := strconv.Atoi(f[0])
			c, _ := strconv.Atoi(f[1])
			if r > 0 && c > 0 {
				return Event{Resize: true, Cols: c, Rows: r}, true
			}
		}
	case 'M', 'm':
		if strings.HasPrefix(p, "<") {
			f := strings.Split(p[1:], ";")
			if len(f) != 3 {
				return Event{}, false
			}
			b, e1 := strconv.Atoi(f[0])
			x, e2 := strconv.Atoi(f[1])
			y, e3 := strconv.Atoi(f[2])
			if e1 != nil || e2 != nil || e3 != nil {
				return Event{}, false
			}
			ev := Event{Mouse: true, MX: x, MY: y, Shift: b&4 != 0, Alt: b&8 != 0, Ctrl: b&16 != 0}
			switch {
			case b&64 != 0:
				if b&3 == 0 {
					ev.MKind = MouseWheelUp
				} else {
					ev.MKind = MouseWheelDown
				}
			case b&32 != 0:
				ev.MKind = MouseMotion
				ev.Button = b & 3
			case final == 'M':
				ev.MKind = MousePress
				ev.Button = b & 3
			default:
				ev.MKind = MouseRelease
				ev.Button = b & 3
			}
			return ev, true
		}
	}
	return Event{}, false
}
