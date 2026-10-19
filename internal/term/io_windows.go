//go:build windows

// Windows terminal I/O (ported from the Spekder door). Two channels, matching
// the unix layer:
//
//   - door mode: the BBS hands us an inherited Winsock SOCKET via DOOR32.SYS.
//     That handle is not a file descriptor, so it cannot be wrapped with
//     os.NewFile; we talk to it with WSARecv/WSASend directly (Synchronet
//     Win32, Mystic Win32, ...).
//   - stdio mode: no socket in the dropfile -> the console (VT processing and
//     raw input enabled) or a redirected pipe from a door launcher.
package term

import (
	"io"
	"log"
	"os"
	"time"

	"golang.org/x/sys/windows"
)

type winSock struct{ h windows.Handle }

func (s *winSock) Write(p []byte) (int, error) {
	total := 0
	for total < len(p) {
		n, err := sockSend(s.h, p[total:])
		if err == windows.WSAEWOULDBLOCK || err == windows.WSAEINTR {
			time.Sleep(2 * time.Millisecond) // backpressure, not failure
			continue
		}
		if err != nil {
			return total, err
		}
		if n == 0 {
			return total, io.ErrShortWrite
		}
		total += n
	}
	return total, nil
}

func (s *winSock) Read(p []byte) (int, error) {
	for {
		n, err := sockRecv(s.h, p)
		if err == windows.WSAEWOULDBLOCK || err == windows.WSAEINTR {
			time.Sleep(6 * time.Millisecond) // inherited non-blocking socket: poll
			continue
		}
		if err != nil {
			return 0, err
		}
		if n == 0 {
			return 0, io.EOF
		}
		return n, nil
	}
}

func (s *winSock) ReadTimeout(p []byte, d time.Duration) (int, error) {
	ms := int(d / time.Millisecond)
	if ms <= 0 {
		ms = 1
	}
	windows.SetsockoptInt(s.h, windows.SOL_SOCKET, windows.SO_RCVTIMEO, ms)
	defer windows.SetsockoptInt(s.h, windows.SOL_SOCKET, windows.SO_RCVTIMEO, 0)
	deadline := time.Now().Add(d)
	for time.Now().Before(deadline) {
		n, err := sockRecv(s.h, p)
		if n > 0 {
			return n, nil
		}
		if err == windows.WSAEWOULDBLOCK || err == windows.WSAEINTR {
			time.Sleep(5 * time.Millisecond)
			continue
		}
		if err == windows.WSAETIMEDOUT {
			return 0, nil
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

func (s *winSock) Close() error { return windows.Closesocket(s.h) }

func sockRecv(h windows.Handle, p []byte) (int, error) {
	if len(p) == 0 {
		return 0, nil
	}
	buf := windows.WSABuf{Len: uint32(len(p)), Buf: &p[0]}
	var recvd, flags uint32
	if err := windows.WSARecv(h, &buf, 1, &recvd, &flags, nil, nil); err != nil {
		return 0, err
	}
	return int(recvd), nil
}

func sockSend(h windows.Handle, p []byte) (int, error) {
	if len(p) == 0 {
		return 0, nil
	}
	buf := windows.WSABuf{Len: uint32(len(p)), Buf: &p[0]}
	var sent uint32
	if err := windows.WSASend(h, &buf, 1, &sent, 0, nil, nil); err != nil {
		return 0, err
	}
	return int(sent), nil
}

type winStdio struct{ in, out *os.File }

func (t *winStdio) Write(p []byte) (int, error) { return t.out.Write(p) }
func (t *winStdio) Close() error                { return nil }

func (t *winStdio) Read(p []byte) (int, error) {
	n, err := t.in.Read(p)
	if err != nil {
		return n, err
	}
	if n == 0 {
		return 0, io.EOF
	}
	return n, nil
}

// ReadTimeout is a no-op on the Windows stdio path: a local console reports
// its size through the console API, and a blind read on a redirected pipe
// cannot be cancelled without leaking a pending read into the key reader.
func (t *winStdio) ReadTimeout(p []byte, d time.Duration) (int, error) { return 0, nil }

// Open returns the inherited Winsock socket when the dropfile names one, else
// the console in VT mode. The returned func restores the console modes.
func Open(drop Dropfile, haveDrop bool, logger *log.Logger) (Term, func()) {
	if haveDrop && drop.CommType == 2 {
		var data windows.WSAData
		if err := windows.WSAStartup(uint32(0x202), &data); err != nil && logger != nil {
			logger.Printf("WSAStartup: %v", err)
		}
		if logger != nil {
			logger.Printf("door winsock handle=%d", drop.Handle)
		}
		return &winSock{h: windows.Handle(uintptr(drop.Handle))}, func() {}
	}
	restore := enableConsoleVT()
	return &winStdio{in: os.Stdin, out: os.Stdout}, restore
}

func enableConsoleVT() func() {
	inH := windows.Handle(os.Stdin.Fd())
	outH := windows.Handle(os.Stdout.Fd())
	var inMode, outMode uint32
	haveIn := windows.GetConsoleMode(inH, &inMode) == nil
	haveOut := windows.GetConsoleMode(outH, &outMode) == nil
	if haveOut {
		windows.SetConsoleMode(outH, outMode|windows.ENABLE_PROCESSED_OUTPUT|windows.ENABLE_VIRTUAL_TERMINAL_PROCESSING)
	}
	if haveIn {
		newIn := inMode &^ (windows.ENABLE_LINE_INPUT | windows.ENABLE_ECHO_INPUT | windows.ENABLE_PROCESSED_INPUT)
		newIn |= windows.ENABLE_VIRTUAL_TERMINAL_INPUT
		windows.SetConsoleMode(inH, newIn)
	}
	return func() {
		if haveIn {
			windows.SetConsoleMode(inH, inMode)
		}
		if haveOut {
			windows.SetConsoleMode(outH, outMode)
		}
	}
}

// LocalSize reads the console window size (fails on a socket or pipe).
func LocalSize() (cols, rows int, ok bool) {
	var info windows.ConsoleScreenBufferInfo
	if err := windows.GetConsoleScreenBufferInfo(windows.Handle(os.Stdout.Fd()), &info); err != nil {
		return 0, 0, false
	}
	cols = int(info.Window.Right - info.Window.Left + 1)
	rows = int(info.Window.Bottom - info.Window.Top + 1)
	if cols <= 0 || rows <= 0 {
		return 0, 0, false
	}
	return cols, rows, true
}
