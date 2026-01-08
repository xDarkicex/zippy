//go:build linux
// +build linux

package zippy

import (
	"golang.org/x/sys/unix"
)

// platformSplice wraps unix.Splice for Linux.
// This provides access to the Linux splice(2) system call for zero-copy I/O.
// Returns (bytes_transferred, error) matching the syscall signature.
func platformSplice(rfd int, roff *int64, wfd int, woff *int64, len int, flags int) (int, error) {
	// unix.Splice returns (int64, error), we need (int, error)
	n, err := unix.Splice(rfd, roff, wfd, woff, len, flags)
	return int(n), err
}

// Splice flags for use in main code
const (
	SPLICE_F_MOVE = unix.SPLICE_F_MOVE
)
