// Package version exposes the build identity of the binary.
//
// The values are stamped in at link time, so a released binary reports the
// release it was built from:
//
//	go build -ldflags "-X github.com/cachestorm/cachestorm/internal/version.Version=v0.3.1"
//
// Both the command line (cmd/cachestorm) and the server commands
// (internal/command) read these, so the version printed by --version and the
// version reported by the INFO command can never disagree.
package version

// Version is the released version, for example "v0.3.1". A build that was
// not stamped reports "dev".
var Version = "dev"

// BuildTime is the build timestamp, or "unknown" when it was not stamped.
var BuildTime = "unknown"
