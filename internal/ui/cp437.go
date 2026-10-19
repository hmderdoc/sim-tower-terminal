package ui

import (
	"fmt"
	"strings"
)

// Tile art and chrome strings are authored in Unicode (so the Go source reads
// like the mock) and converted to CP437 bytes once at init. Nothing outside
// this file ever sees a rune.

// cp437High maps bytes 0x80..0xFF to Unicode.
var cp437High = [128]rune{
	'Ç', 'ü', 'é', 'â', 'ä', 'à', 'å', 'ç', 'ê', 'ë', 'è', 'ï', 'î', 'ì', 'Ä', 'Å',
	'É', 'æ', 'Æ', 'ô', 'ö', 'ò', 'û', 'ù', 'ÿ', 'Ö', 'Ü', '¢', '£', '¥', '₧', 'ƒ',
	'á', 'í', 'ó', 'ú', 'ñ', 'Ñ', 'ª', 'º', '¿', '⌐', '¬', '½', '¼', '¡', '«', '»',
	'░', '▒', '▓', '│', '┤', '╡', '╢', '╖', '╕', '╣', '║', '╗', '╝', '╜', '╛', '┐',
	'└', '┴', '┬', '├', '─', '┼', '╞', '╟', '╚', '╔', '╩', '╦', '╠', '═', '╬', '╧',
	'╨', '╤', '╥', '╙', '╘', '╒', '╓', '╫', '╪', '┘', '┌', '█', '▄', '▌', '▐', '▀',
	'α', 'ß', 'Γ', 'π', 'Σ', 'σ', 'µ', 'τ', 'Φ', 'Θ', 'Ω', 'δ', '∞', 'φ', 'ε', '∩',
	'≡', '±', '≥', '≤', '⌠', '⌡', '÷', '≈', '°', '∙', '·', '√', 'ⁿ', '²', '■', 0x00A0,
}

// cp437Low maps the low bytes that are safe to emit (see screen.SafeByte) to
// their CP437 glyphs. 0x07/0x08/0x09/0x0A/0x0D/0x1B are deliberately absent.
var cp437Low = map[byte]rune{
	0x01: '☺', 0x02: '☻', 0x03: '♥', 0x04: '♦', 0x05: '♣', 0x06: '♠',
	0x0B: '♂', 0x0C: '♀', 0x0E: '♫', 0x0F: '☼', 0x10: '►', 0x11: '◄',
	0x12: '↕', 0x13: '‼', 0x14: '¶', 0x15: '§', 0x16: '▬', 0x17: '↨',
	0x18: '↑', 0x19: '↓', 0x1A: '→', 0x1C: '∟', 0x1D: '↔', 0x1E: '▲', 0x1F: '▼',
}

// substitutes: glyphs the mock uses that CP437 lacks or that would land on an
// unsafe byte.
var cp437Subst = map[rune]byte{
	'♪': 0x0E, // 0x0D is CR: use the double note
	'◙': 'o',  // 0x0A is LF: wheels are plain o
	'▪': 0xFE, // no small square in CP437
	'★': '*',  // no star in CP437
	'○': 'o',  // 0x09 is TAB
	'●': 'o',  // 0x07 is BEL
	'◘': 'o',  // 0x08 is BS
}

var uniToCP = func() map[rune]byte {
	m := make(map[rune]byte, 256)
	for i, r := range cp437High {
		m[r] = byte(0x80 + i)
	}
	for b, r := range cp437Low {
		m[r] = b
	}
	for r, b := range cp437Subst {
		m[r] = b
	}
	return m
}()

// cpToUni is the reverse map used by the text dumps in tests.
var cpToUni = func() [256]rune {
	var t [256]rune
	for i := 0; i < 256; i++ {
		t[i] = '?'
	}
	for i := 0x20; i < 0x7F; i++ {
		t[i] = rune(i)
	}
	for i, r := range cp437High {
		t[0x80+i] = r
	}
	for b, r := range cp437Low {
		t[b] = r
	}
	t[0xFF] = ' '
	return t
}()

// CP converts a Unicode string to CP437 bytes. It panics on a rune with no
// mapping so a bad glyph in a table fails at init, not on the wire.
func CP(s string) string {
	var b strings.Builder
	b.Grow(len(s))
	for _, r := range s {
		switch {
		case r >= 0x20 && r < 0x7F:
			b.WriteByte(byte(r))
		default:
			c, ok := uniToCP[r]
			if !ok {
				panic(fmt.Sprintf("ui: no CP437 mapping for %q (U+%04X) in %q", r, r, s))
			}
			b.WriteByte(c)
		}
	}
	return b.String()
}

// Glyph bytes used directly by the renderer.
var (
	gFace     = CP("☺")[0]
	gFaceInv  = CP("☻")[0]
	gLegsA    = CP("╨")[0]
	gLegsB    = CP("∩")[0]
	gWallD    = CP("║")[0]
	gWallS    = CP("│")[0]
	gCable    = CP("·")[0]
	gDot      = CP("∙")[0]
	gLight    = CP("░")[0]
	gMedium   = CP("▒")[0]
	gDark     = CP("▓")[0]
	gFull     = CP("█")[0]
	gLowHalf  = CP("▄")[0]
	gUpHalf   = CP("▀")[0]
	gSun      = CP("☼")[0]
	gPlant    = CP("♣")[0]
	gTriUp    = CP("▲")[0]
	gTriDn    = CP("▼")[0]
	gTriRt    = CP("►")[0]
	gHLine    = CP("─")[0]
	gHLineD   = CP("═")[0]
	gArrowUp  = CP("↑")[0]
	gArrowDn  = CP("↓")[0]
	gSquare   = CP("■")[0]
	gBar      = CP("▬")[0]
	gCornerTL = CP("┌")[0]
	gCornerTR = CP("┐")[0]
	gCornerBL = CP("└")[0]
	gCornerBR = CP("┘")[0]
)
