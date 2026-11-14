//go:build unix

package server

import (
	"net"
	"syscall"
)

// readNonblocking reads what the socket holds without waiting: EAGAIN
// when it holds nothing, and 0 with no error at end of input.
func readNonblocking(fd int, p []byte) (int, error) {
	for {
		n, err := syscall.Read(fd, p)
		if err == syscall.EINTR {
			continue
		}
		if n < 0 {
			n = 0
		}
		return n, err
	}
}

func wouldBlock(err error) bool { return err == syscall.EAGAIN }

// rawSocket returns the socket under a plain TCP or Unix connection, which
// a connection may wait on and read directly; nil for any other (TLS, in
// memory).
func rawSocket(conn net.Conn) syscall.RawConn {
	switch conn.(type) {
	case *net.TCPConn, *net.UnixConn:
	default:
		return nil
	}
	raw, err := conn.(syscall.Conn).SyscallConn()
	if err != nil {
		return nil
	}
	return raw
}
