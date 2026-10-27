//go:build !linux

package command

// processRSS is not measured on this platform.
func processRSS() (int64, bool) { return 0, false }

// totalSystemMemory is not measured on this platform.
func totalSystemMemory() int64 { return 0 }
