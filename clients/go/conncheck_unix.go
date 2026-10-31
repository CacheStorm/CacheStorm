//go:build unix

package cachestorm

import (
	"errors"
	"syscall"
)

// connCheck reports whether an idle connection is still open: the server may
// have closed it (timeout, CLIENT KILL, restart) while it sat in the pool. A
// non-blocking peek must find nothing to read.
func connCheck(c syscall.Conn) bool {
	raw, err := c.SyscallConn()
	if err != nil {
		return false
	}
	alive := false
	var buf [1]byte
	if err := raw.Read(func(fd uintptr) bool {
		n, _, err := syscall.Recvfrom(int(fd), buf[:], syscall.MSG_PEEK|syscall.MSG_DONTWAIT)
		alive = n < 0 && (errors.Is(err, syscall.EAGAIN) || errors.Is(err, syscall.EWOULDBLOCK))
		return true
	}); err != nil {
		return false
	}
	return alive
}
