// Package screen is a CP437 cell buffer with a diff-based flush: the game
// draws a whole frame into Cells, Flush emits only what changed since the last
// frame as positioned ANSI, which keeps a 60-row x 132-col tower view cheap
// over a BBS link.
package screen

import (
	"bytes"
	"fmt"
	"io"
)

// Cell is one character cell: CP437 byte + 16-color fg + 8-color bg (+blink
// bit unused). Attr packs fg in the low nibble and bg in bits 4-6.
type Cell struct {
	Ch byte
	Fg uint8 // 0..15
	Bg uint8 // 0..7
}

type Screen struct {
	W, H int
	cur  []Cell
	prev []Cell
	full bool // force full redraw next flush
}

func New(w, h int) *Screen {
	s := &Screen{}
	s.Resize(w, h)
	return s
}

func (s *Screen) Resize(w, h int) {
	s.W, s.H = w, h
	s.cur = make([]Cell, w*h)
	s.prev = make([]Cell, w*h)
	s.full = true
	s.Clear(0)
}

// Invalidate forces the next Flush to repaint every cell.
func (s *Screen) Invalidate() { s.full = true }

func (s *Screen) Clear(bg uint8) {
	for i := range s.cur {
		s.cur[i] = Cell{Ch: ' ', Fg: 7, Bg: bg}
	}
}

func (s *Screen) Set(x, y int, c Cell) {
	if x < 0 || y < 0 || x >= s.W || y >= s.H {
		return
	}
	s.cur[y*s.W+x] = c
}

func (s *Screen) Get(x, y int) Cell {
	if x < 0 || y < 0 || x >= s.W || y >= s.H {
		return Cell{}
	}
	return s.cur[y*s.W+x]
}

// Put writes a CP437 string (bytes) at x,y with the given colors; clips.
func (s *Screen) Put(x, y int, str string, fg, bg uint8) {
	for i := 0; i < len(str); i++ {
		s.Set(x+i, y, Cell{Ch: str[i], Fg: fg, Bg: bg})
	}
}

// Fill paints a rectangle with one cell.
func (s *Screen) Fill(x, y, w, h int, c Cell) {
	for yy := y; yy < y+h; yy++ {
		for xx := x; xx < x+w; xx++ {
			s.Set(xx, yy, c)
		}
	}
}

// Flush writes the frame diff to w. Cursor is left hidden.
func (s *Screen) Flush(w io.Writer) error {
	var b bytes.Buffer
	if s.full {
		b.WriteString("\x1b[?25l\x1b[0m\x1b[2J")
	}
	curFg, curBg := -1, -1
	cx, cy := -1, -1
	for y := 0; y < s.H; y++ {
		for x := 0; x < s.W; x++ {
			i := y*s.W + x
			c := s.cur[i]
			if !s.full && c == s.prev[i] {
				continue
			}
			if cx != x || cy != y {
				fmt.Fprintf(&b, "\x1b[%d;%dH", y+1, x+1)
				cx, cy = x, y
			}
			if int(c.Fg) != curFg || int(c.Bg) != curBg {
				writeSGR(&b, c.Fg, c.Bg)
				curFg, curBg = int(c.Fg), int(c.Bg)
			}
			ch := c.Ch
			if !SafeByte(ch) {
				ch = ' ' // control bytes would corrupt the terminal
			}
			b.WriteByte(ch)
			cx++
			// Avoid relying on autowrap at the last column: re-home next cell.
			if cx >= s.W {
				cx, cy = -1, -1
			}
			s.prev[i] = c
		}
	}
	s.full = false
	if b.Len() == 0 {
		return nil
	}
	_, err := w.Write(b.Bytes())
	return err
}

// cgaToANSI maps CGA/VGA palette indexes (0 black, 1 blue, 2 green, 3 cyan,
// 4 red, 5 magenta, 6 brown, 7 light gray) to the SGR 30-37/40-47 order
// (black, red, green, yellow, blue, magenta, cyan, white).
var cgaToANSI = [8]int{0, 4, 2, 6, 1, 5, 3, 7}

func writeSGR(b *bytes.Buffer, fg, bg uint8) {
	b.WriteString("\x1b[0;")
	if fg >= 8 {
		b.WriteString("1;")
	}
	fmt.Fprintf(b, "%d;%dm", 30+cgaToANSI[fg&7], 40+cgaToANSI[bg&7])
}

// SafeByte reports whether b may be sent to the terminal as cell content.
// SyncTERM and most CP437 terminals render the low control range as glyphs
// (0x01 smiley, 0x0E note, 0x10 triangle ...), except the bytes that act as
// real controls: NUL, BEL, BS, TAB, LF, CR, ESC. DEL is swallowed by many
// clients and 0xFF is the telnet IAC byte, so both are refused too.
func SafeByte(b byte) bool {
	switch b {
	case 0x00, 0x07, 0x08, 0x09, 0x0A, 0x0D, 0x1B, 0x7F, 0xFF:
		return false
	}
	return true
}
