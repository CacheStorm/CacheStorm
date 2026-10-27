package persistence

import (
	"errors"
	"os"
	"path/filepath"
	"sync"
	"testing"
)

func TestDataDirectoryLockAliasesAndRelease(t *testing.T) {
	dir := t.TempDir()
	first, err := LockDataDirectory(dir)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = first.Close() })
	canonical, err := filepath.EvalSymlinks(dir)
	if err != nil {
		t.Fatal(err)
	}
	if first.Directory() != canonical {
		t.Fatalf("directory not pinned: %q", first.Directory())
	}
	paths := []string{dir, filepath.Join(dir, ".")}
	cwd, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	if relative, err := filepath.Rel(cwd, dir); err == nil {
		paths = append(paths, relative)
	}
	alias := filepath.Join(t.TempDir(), "alias")
	if err := os.Symlink(dir, alias); err == nil {
		paths = append(paths, alias)
	}
	for _, path := range paths {
		if second, err := LockDataDirectory(path); !errors.Is(err, ErrDataDirectoryLocked) {
			if second != nil {
				second.Close()
			}
			t.Fatalf("second owner allowed via %q: %v", path, err)
		}
	}
	if err := first.Close(); err != nil {
		t.Fatal(err)
	}
	if err := first.Close(); err != nil {
		t.Fatal(err)
	}
	second, err := LockDataDirectory(dir)
	if err != nil {
		t.Fatal(err)
	}
	if err := second.Close(); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(dir, ".cachestorm.lock")); err != nil {
		t.Fatalf("lock inode removed on release: %v", err)
	}
}

func TestDataDirectoryLockConcurrentOwners(t *testing.T) {
	dir := t.TempDir()
	const attempts = 32
	start := make(chan struct{})
	owners := make(chan *DataDirectoryLock, attempts)
	errorsX := make(chan error, attempts)
	var wg sync.WaitGroup
	for i := 0; i < attempts; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			<-start
			owner, err := LockDataDirectory(dir)
			if err == nil {
				owners <- owner
			} else if !errors.Is(err, ErrDataDirectoryLocked) {
				errorsX <- err
			}
		}()
	}
	close(start)
	wg.Wait()
	close(owners)
	close(errorsX)
	count := 0
	for owner := range owners {
		count++
		if err := owner.Close(); err != nil {
			t.Error(err)
		}
	}
	for err := range errorsX {
		t.Error(err)
	}
	if count != 1 {
		t.Fatalf("got %d concurrent owners", count)
	}
}
