package server

import (
	"net"
	"syscall"
)

// socketFD returns the file descriptor of conn's socket, or -1 when it has
// none (an in-memory connection).
func socketFD(conn net.Conn) int {
	if tc, ok := conn.(interface{ NetConn() net.Conn }); ok {
		conn = tc.NetConn()
	}
	sc, ok := conn.(syscall.Conn)
	if !ok {
		return -1
	}
	raw, err := sc.SyscallConn()
	if err != nil {
		return -1
	}
	fd := -1
	if raw.Control(func(f uintptr) { fd = int(f) }) != nil {
		return -1
	}
	return fd
}
