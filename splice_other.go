//go:build !linux
// +build !linux

package zippy

import "syscall"

// platformSplice stub for non-Linux platforms.
// Always returns ENOTSUP to trigger graceful fallback.
func platformSplice(rfd int, roff *int64, wfd int, woff *int64, len int, flags int) (int, error) {
	return 0, syscall.ENOTSUP
}

// Splice flags (unused on non-Linux, but needed for compilation)
const (
	SPLICE_F_MOVE = 0
)
