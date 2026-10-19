//go:build !windows && !darwin

package term

import "golang.org/x/sys/unix"

const tcpKeepIdleOpt = unix.TCP_KEEPIDLE
