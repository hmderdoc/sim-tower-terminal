//go:build !windows

package term

import (
	"io"
	"log"
	"os"
	"os/exec"
	"strings"
	"syscall"
	"time"
	"unsafe"

	"golang.org/x/sys/unix"
)

// enableKeepAlive makes the kernel notice a vanished client (half-open socket)
// so a blocking Read eventually returns and the BBS node is freed.
func enableKeepAlive(fd int) {
	if err := unix.SetsockoptInt(fd, unix.SOL_SOCKET, unix.SO_KEEPALIVE, 1); err != nil {
		return
	}
	unix.SetsockoptInt(fd, unix.IPPROTO_TCP, tcpKeepIdleOpt, 60)
	unix.SetsockoptInt(fd, unix.IPPROTO_TCP, unix.TCP_KEEPINTVL, 15)
	unix.SetsockoptInt(fd, unix.IPPROTO_TCP, unix.TCP_KEEPCNT, 4)
}

type unixTerm struct {
	in, out *os.File
	socket  bool
}

func (t *unixTerm) Write(p []byte) (int, error) { return t.out.Write(p) }

func (t *unixTerm) Close() error {
	if t.socket {
		return t.in.Close()
	}
	return nil
}

func (t *unixTerm) Read(p []byte) (int, error) {
	fd := int(t.in.Fd())
	for {
		n, err := syscall.Read(fd, p)
		if err == syscall.EAGAIN || err == syscall.EWOULDBLOCK {
			time.Sleep(6 * time.Millisecond)
			continue
		}
		if err != nil {
			return n, err
		}
		if n == 0 {
			return 0, io.EOF
		}
		return n, nil
	}
}

func (t *unixTerm) ReadTimeout(p []byte, d time.Duration) (int, error) {
	fd := int(t.in.Fd())
	syscall.SetNonblock(fd, true)
	defer syscall.SetNonblock(fd, false)
	deadline := time.Now().Add(d)
	for time.Now().Before(deadline) {
		n, err := syscall.Read(fd, p)
		if n > 0 {
			return n, nil
		}
		if err == syscall.EAGAIN || err == syscall.EWOULDBLOCK {
			time.Sleep(5 * time.Millisecond)
			continue
		}
		if err != nil {
			return 0, err
		}
		if n == 0 {
			return 0, io.EOF
		}
	}
	return 0, nil
}

// Open returns the inherited door socket when the dropfile names one, else
// stdio in raw mode. The returned func restores the tty.
func Open(drop Dropfile, haveDrop bool, logger *log.Logger) (Term, func()) {
	in, out := os.Stdin, os.Stdout
	socket := false
	if haveDrop && drop.CommType == 2 {
		if sf := os.NewFile(uintptr(drop.Handle), "door-socket"); sf != nil {
			in, out = sf, sf
			socket = true
			enableKeepAlive(int(drop.Handle))
			if logger != nil {
				logger.Printf("door socket fd=%d", drop.Handle)
			}
		}
	}
	restore := func() {}
	if !socket {
		restore = sttyRaw()
	}
	return &unixTerm{in: in, out: out, socket: socket}, restore
}

// LocalSize reads the controlling tty size (only meaningful for stdio play).
func LocalSize() (cols, rows int, ok bool) {
	type winsize struct{ Row, Col, X, Y uint16 }
	var ws winsize
	_, _, e := syscall.Syscall(syscall.SYS_IOCTL, os.Stdout.Fd(),
		uintptr(syscall.TIOCGWINSZ), uintptr(unsafe.Pointer(&ws)))
	if e != 0 || ws.Col == 0 || ws.Row == 0 {
		return 0, 0, false
	}
	return int(ws.Col), int(ws.Row), true
}

func sttyRaw() func() {
	saved, err := exec.Command("stty", "-F", "/dev/tty", "-g").Output()
	if err != nil {
		saved, err = sttyArgs(os.Stdin, "-g")
		if err != nil {
			return func() {}
		}
	}
	sttyArgs(os.Stdin, "-icanon", "-echo", "-isig", "min", "1", "time", "0")
	prev := strings.TrimSpace(string(saved))
	return func() { sttyArgs(os.Stdin, prev) }
}

func sttyArgs(tty *os.File, args ...string) ([]byte, error) {
	cmd := exec.Command("stty", args...)
	cmd.Stdin = tty
	return cmd.Output()
}
