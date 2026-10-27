package persistence

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sync"
)

var ErrDataDirectoryLocked = errors.New("persistent data directory is already in use")

type DataDirectoryLock struct {
	mu   sync.Mutex
	file *os.File
	dir  string
}

func LockDataDirectory(dir string) (*DataDirectoryLock, error) {
	if dir == "" {
		dir = "."
	}
	absolute, err := filepath.Abs(dir)
	if err != nil {
		return nil, err
	}
	if err := os.MkdirAll(absolute, 0755); err != nil {
		return nil, err
	}
	canonical, err := filepath.EvalSymlinks(absolute)
	if err != nil {
		return nil, err
	}
	f, err := os.OpenFile(filepath.Join(canonical, ".cachestorm.lock"), os.O_CREATE|os.O_RDWR, 0600)
	if err != nil {
		return nil, err
	}
	info, err := f.Stat()
	if err != nil || !info.Mode().IsRegular() {
		f.Close()
		if err != nil {
			return nil, err
		}
		return nil, errors.New("data directory lock must be a regular file")
	}
	if err := lockDataFile(f); err != nil {
		f.Close()
		return nil, fmt.Errorf("lock data directory %q: %w", canonical, err)
	}
	return &DataDirectoryLock{file: f, dir: canonical}, nil
}

func (l *DataDirectoryLock) Directory() string { return l.dir }

func (l *DataDirectoryLock) Close() error {
	l.mu.Lock()
	defer l.mu.Unlock()
	if l.file == nil {
		return nil
	}
	err := errors.Join(unlockDataFile(l.file), l.file.Close())
	l.file = nil
	return err
}
