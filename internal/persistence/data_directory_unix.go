//go:build linux || darwin || freebsd || netbsd || openbsd || dragonfly

package persistence

import (
	"errors"
	"os"

	"golang.org/x/sys/unix"
)

func lockDataFile(f *os.File) error {
	err := unix.Flock(int(f.Fd()), unix.LOCK_EX|unix.LOCK_NB)
	if errors.Is(err, unix.EWOULDBLOCK) || errors.Is(err, unix.EAGAIN) {
		return ErrDataDirectoryLocked
	}
	return err
}

func unlockDataFile(f *os.File) error { return unix.Flock(int(f.Fd()), unix.LOCK_UN) }
