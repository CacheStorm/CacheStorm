package server

import "net"

// protectedModeMessage explains a refusal in protected mode, after Redis'
// DENIED reply.
const protectedModeMessage = "CacheStorm is running in protected mode because protected mode is enabled and no password is set. " +
	"In this mode connections are only accepted from the loopback interface. " +
	"To accept connections from other hosts, either set a password (server.requirepass or CACHESTORM_REQUIREPASS) " +
	"or disable protected mode (server.protected_mode: false or CACHESTORM_PROTECTED_MODE=false) " +
	"after making sure the server is not reachable from untrusted networks."

// isLoopbackAddr reports whether a "host:port" remote address is on the
// loopback interface. A remote address that is not an IP (a Unix socket or
// an in-process pipe) is local and counts as loopback.
func isLoopbackAddr(addr string) bool {
	host, _, err := net.SplitHostPort(addr)
	if err != nil {
		host = addr
	}
	ip := net.ParseIP(host)
	if ip == nil {
		return true
	}
	return ip.IsLoopback()
}
