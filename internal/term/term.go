// Package term is the door's single I/O channel: either the telnet socket the
// BBS hands us through DOOR32.SYS, or the process's own stdin/stdout (local
// play / development). Same approach as the Spekder door.
package term

import (
	"bufio"
	"io"
	"os"
	"strconv"
	"strings"
	"time"
)

// Term is a byte-oriented bidirectional channel to the caller's terminal.
type Term interface {
	io.Writer
	io.Closer
	// Read blocks until at least one byte is available; returns io.EOF when the
	// connection drops.
	Read(p []byte) (int, error)
	// ReadTimeout reads whatever is available within d, (0,nil) on timeout.
	ReadTimeout(p []byte, d time.Duration) (int, error)
}

// Dropfile holds the fields we care about from DOOR32.SYS.
type Dropfile struct {
	CommType  int    // 0 local, 1 serial, 2 telnet socket
	Handle    uint64 // socket handle (fd on unix) when CommType==2
	BBSName   string
	UserNum   int
	RealName  string
	Alias     string
	Level     int
	TimeLeft  int // minutes
	Emulation int
	Node      int
}

// ReadDoor32 parses a DOOR32.SYS file. Missing file -> zero value, ok=false.
func ReadDoor32(path string) (Dropfile, bool) {
	var d Dropfile
	f, err := os.Open(path)
	if err != nil {
		return d, false
	}
	defer f.Close()
	sc := bufio.NewScanner(f)
	var lines []string
	for sc.Scan() {
		lines = append(lines, strings.TrimSpace(sc.Text()))
	}
	get := func(i int) string {
		if i < len(lines) {
			return lines[i]
		}
		return ""
	}
	atoi := func(s string) int { n, _ := strconv.Atoi(s); return n }
	d.CommType = atoi(get(0))
	d.Handle, _ = strconv.ParseUint(get(1), 10, 64)
	d.BBSName = get(3)
	d.UserNum = atoi(get(4))
	d.RealName = get(5)
	d.Alias = get(6)
	d.Level = atoi(get(7))
	d.TimeLeft = atoi(get(8))
	d.Emulation = atoi(get(9))
	d.Node = atoi(get(10))
	return d, len(lines) >= 2
}

// ProbeSize asks the terminal for its size with the parked-cursor CPR trick
// (SyncTERM/CTerm answer CPR but not ESC[18t). Must run before any reader
// goroutine starts, or the reply gets swallowed.
func ProbeSize(t Term, timeout time.Duration) (cols, rows int, ok bool) {
	if _, err := t.Write([]byte("\x1b7\x1b[999;999H\x1b[6n\x1b8")); err != nil {
		return 0, 0, false
	}
	deadline := time.Now().Add(timeout)
	buf := make([]byte, 0, 128)
	scratch := make([]byte, 64)
	for time.Now().Before(deadline) && len(buf) < 256 {
		rem := time.Until(deadline)
		if rem > 50*time.Millisecond {
			rem = 50 * time.Millisecond
		}
		n, err := t.ReadTimeout(scratch, rem)
		if n > 0 {
			buf = append(buf, scratch[:n]...)
			if r, c, found := ParseCursorReport(buf); found {
				return c, r, true
			}
		} else if err != nil {
			break
		}
	}
	return 0, 0, false
}

// ParseCursorReport scans for ESC [ rows ; cols R.
func ParseCursorReport(buf []byte) (rows, cols int, ok bool) {
	for i := 0; i+4 < len(buf); i++ {
		if buf[i] != 0x1B || buf[i+1] != '[' {
			continue
		}
		j, r := i+2, 0
		for j < len(buf) && buf[j] >= '0' && buf[j] <= '9' {
			r = r*10 + int(buf[j]-'0')
			j++
		}
		if r == 0 || j >= len(buf) || buf[j] != ';' {
			continue
		}
		j++
		c := 0
		for j < len(buf) && buf[j] >= '0' && buf[j] <= '9' {
			c = c*10 + int(buf[j]-'0')
			j++
		}
		if c == 0 || j >= len(buf) || buf[j] != 'R' {
			continue
		}
		return r, c, true
	}
	return 0, 0, false
}
