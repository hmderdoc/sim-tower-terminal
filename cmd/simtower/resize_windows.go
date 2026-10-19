//go:build windows

package main

import (
	"time"

	"simtower/internal/term"
)

// watchResize polls the console size on Windows (no SIGWINCH there).
func watchResize(events chan<- term.Event) {
	go func() {
		lc, lr, _ := term.LocalSize()
		for range time.Tick(500 * time.Millisecond) {
			if c, r, ok := term.LocalSize(); ok && (c != lc || r != lr) {
				lc, lr = c, r
				events <- term.Event{Resize: true, Cols: c, Rows: r}
			}
		}
	}()
}
