//go:build !unix

package cachestorm

import "syscall"

func connCheck(syscall.Conn) bool { return true }
