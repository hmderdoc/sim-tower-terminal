//go:build darwin

package term

import "golang.org/x/sys/unix"

// macOS names the idle-time option TCP_KEEPALIVE.
const tcpKeepIdleOpt = unix.TCP_KEEPALIVE
