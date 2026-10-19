//go:build !windows

package main

import (
	"os"
	"os/signal"
	"syscall"

	"simtower/internal/term"
)

// watchResize feeds terminal window-size changes (SIGWINCH) to the UI as
// Resize events when playing on a local tty.
func watchResize(events chan<- term.Event) {
	winch := make(chan os.Signal, 1)
	signal.Notify(winch, syscall.SIGWINCH)
	go func() {
		for range winch {
			if c, r, ok := term.LocalSize(); ok {
				events <- term.Event{Resize: true, Cols: c, Rows: r}
			}
		}
	}()
}
