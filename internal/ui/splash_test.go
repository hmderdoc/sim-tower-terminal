package ui

import (
	"testing"

	"simtower/internal/term"
)

// TestGoldenStartMenu dumps the splash + start menu at the minimum and a
// large size so the art can be reviewed (render testdata/*.txt to PNG).
func TestGoldenStartMenu(t *testing.T) {
	for _, sz := range [][2]int{{80, 24}, {132, 50}} {
		a, _ := newLauncherApp(t, true)
		a.handle(term.Event{Resize: true, Cols: sz[0], Rows: sz[1]})
		a.render()
		golden(t, itoa(sz[0])+"x"+itoa(sz[1])+"_start", a)
	}
}
