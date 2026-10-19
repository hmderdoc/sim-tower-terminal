package ui

import (
	"simtower/internal/screen"
	"simtower/internal/tdf"
)

// The splash is an 80x24 panel: the title in a TheDraw font over a night sky
// with stars and a moon, a skyline of lit buildings along the bottom and a
// grass strip. On terminals larger than 80x24 the panel is centred and the
// rest of the screen is a dark-blue shaded fill. The start menu is drawn on
// top of it by the caller (see splashMenuRect).

const (
	splashW = 80
	splashH = 24
	skyBlue = 1
)

var splashFont *tdf.Font

func init() {
	if f, err := tdf.Embedded("shadlndx", 0); err == nil {
		splashFont = f
	}
}

// splashOrigin returns the top-left screen cell of the centred 80x24 panel.
func (a *App) splashOrigin() (ox, oy int) {
	return (a.cols - splashW) / 2, (a.rows - splashH) / 2
}

// splashMenuRect is the area inside the panel reserved for the start menu
// (panel-relative rows 11..17, centred), returned in screen coordinates.
func (a *App) splashMenuRect(w, h int) rect {
	ox, oy := a.splashOrigin()
	return rect{ox + (splashW-w)/2, oy + 11 + (7-h)/2, w, h}
}

// splashBuilding is one silhouette in the skyline: left unit, width, height
// in rows (roof row included), door = draw a lit entrance.
type splashBuilding struct {
	x, w, h int
	door    bool
}

var splashSkyline = []splashBuilding{
	{0, 6, 3, false}, {7, 9, 5, false}, {17, 5, 2, false}, {23, 8, 4, false},
	{32, 16, 7, true}, {49, 7, 3, false}, {57, 10, 5, false}, {68, 6, 2, false}, {75, 5, 4, false},
}

// drawSplash paints the backdrop (fill + panel + title + skyline). frame
// drives the star twinkle.
func (a *App) drawSplash(frame int) {
	a.fullClip()
	// shaded fill outside the panel
	a.fill(0, 0, a.cols, a.rows, 0xB1, 8, 0) // ▒ dark gray dither on black
	ox, oy := a.splashOrigin()
	if a.cols > splashW || a.rows > splashH {
		// frame the panel when there is room around it
		fx, fy, fw, fh := ox-1, oy-1, splashW+2, splashH+2
		a.fill(fx, fy, fw, 1, 0xCD, 7, 0)                     // ═
		a.fill(fx, fy+fh-1, fw, 1, 0xCD, 7, 0)                // ═
		a.fill(fx, fy, 1, fh, 0xBA, 7, 0)                     // ║
		a.fill(fx+fw-1, fy, 1, fh, 0xBA, 7, 0)                // ║
		a.set(fx, fy, screen.Cell{Ch: 0xC9, Fg: 7})           // ╔
		a.set(fx+fw-1, fy, screen.Cell{Ch: 0xBB, Fg: 7})      // ╗
		a.set(fx, fy+fh-1, screen.Cell{Ch: 0xC8, Fg: 7})      // ╚
		a.set(fx+fw-1, fy+fh-1, screen.Cell{Ch: 0xBC, Fg: 7}) // ╝
	}
	// sky
	a.fill(ox, oy, splashW, splashH-2, ' ', 7, skyBlue)
	for i := 0; i < 46; i++ {
		x := (i*37 + 11) % splashW
		y := (i*7 + 3) % 16
		if y >= 2 && y <= 9 && x >= 3 && x < 77 {
			continue // keep the title clean
		}
		ch := byte(0xFA) // ·
		fg := uint8(7)
		if (i+frame/3)%5 == 0 {
			ch, fg = 0xF9, 15 // ∙ bright twinkle
		}
		a.set(ox+x, oy+y, screen.Cell{Ch: ch, Fg: fg, Bg: skyBlue})
	}
	// moon
	a.put(ox+76, oy, "\xDC\xDC", 15, skyBlue)
	a.put(ox+76, oy+1, "\xDF\xDF", 15, skyBlue)
	// title
	if splashFont != nil {
		grid, w, _ := splashFont.Cells("SIMTOWER", tdf.RenderOpts{})
		tx := ox + (splashW-w)/2
		for r, row := range grid {
			for c, cell := range row {
				if !cell.Ink {
					continue
				}
				fg := cell.Fg
				if fg == 0 {
					fg = 7
				}
				a.set(tx+c, oy+2+r, screen.Cell{Ch: cell.Ch, Fg: fg, Bg: skyBlue})
			}
		}
	} else {
		a.put(ox+36, oy+5, "SIMTOWER", 15, skyBlue)
	}
	tag := "A tower simulation, faithful to the 1994 original"
	a.put(ox+(splashW-len(tag))/2, oy+10, tag, 11, skyBlue)
	// skyline: rows 15..21 (panel), ground row 22
	ground := oy + 22
	for _, b := range splashSkyline {
		top := ground - b.h
		a.fill(ox+b.x, top, b.w, 1, 0xDC, 0, skyBlue) // ▄ roof edge
		a.fill(ox+b.x, top+1, b.w, b.h-1, ' ', 7, 0)
		for yy := top + 1; yy < ground; yy++ {
			for xx := b.x + 1; xx < b.x+b.w-1; xx += 2 {
				h := (xx*31 + yy*17 + b.x) % 7
				fg := uint8(14)
				if h < 3 {
					fg = 8
				}
				a.set(ox+xx, yy, screen.Cell{Ch: 0xFE, Fg: fg, Bg: 0}) // ■ window
			}
		}
		if b.door {
			dx := ox + b.x + b.w/2 - 1
			a.put(dx, ground-1, "\xDC\xDC", 14, 0)
			a.set(ox+b.x+b.w/2, top-1, screen.Cell{Ch: 0xB3, Fg: 7, Bg: skyBlue}) // antenna │
		}
	}
	a.fill(ox, ground, splashW, 1, 0xB2, 2, 0) // ▓ grass
	a.fill(ox, ground+1, splashW, 1, ' ', 7, 0)
}
