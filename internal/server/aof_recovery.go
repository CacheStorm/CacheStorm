package server

import (
	"errors"
	"os"
)

func truncateRecoveredAOF(path string, size int64) error {
	f, err := os.OpenFile(path, os.O_WRONLY, 0)
	if err != nil {
		return err
	}
	if err := f.Truncate(size); err != nil {
		return errors.Join(err, f.Close())
	}
	return errors.Join(f.Sync(), f.Close())
}
