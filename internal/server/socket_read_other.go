//go:build !unix

package server

import (
	"errors"
	"net"
	"syscall"
)

var errNoDirectRead = errors.New("direct socket reads are not supported")

func readNonblocking(int, []byte) (int, error) { return 0, errNoDirectRead }

func wouldBlock(error) bool { return false }

func rawSocket(net.Conn) syscall.RawConn { return nil }
