//go:build !linux && !darwin && !freebsd && !netbsd && !openbsd && !dragonfly && !windows

package persistence

import (
	"errors"
	"os"
)

func lockDataFile(*os.File) error {
	return errors.New("persistent data directory locking is unsupported on this platform")
}
func unlockDataFile(*os.File) error { return nil }
