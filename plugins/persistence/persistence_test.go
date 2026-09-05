package persistence

import (
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/cachestorm/cachestorm/internal/command"
	"github.com/cachestorm/cachestorm/internal/store"
)

// TestAfterCommandVsCloseRace drives the AOF Append hot path concurrently
// with plugin shutdown; under -race this fails if AOFWriter Flush/Close are
// not serialized against AfterCommand (plugin mutex regression guard).
func TestAfterCommandVsCloseRace(t *testing.T) {
	dir := t.TempDir()
	p := NewPersistencePlugin(store.NewStore(), dir, "everysec", time.Hour)
	if err := p.Init(nil); err != nil {
		t.Fatalf("Init failed: %v", err)
	}
	if err := p.OnStartup(); err != nil {
		t.Fatalf("OnStartup failed: %v", err)
	}

	stop := make(chan struct{})
	var wg sync.WaitGroup
	for i := 0; i < 8; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for {
				select {
				case <-stop:
					return
				default:
					p.AfterCommand(&command.Context{
						Command:   "SET",
						Args:      [][]byte{[]byte("race-key"), []byte("race-value")},
						StartTime: time.Now(),
					})
				}
			}
		}()
	}

	time.Sleep(50 * time.Millisecond)
	close(stop)
	wg.Wait()

	if err := p.OnShutdown(); err != nil {
		t.Fatalf("OnShutdown failed: %v", err)
	}
	if err := p.Close(); err != nil {
		t.Fatalf("Close failed: %v", err)
	}

	fi, err := os.Stat(filepath.Join(dir, "appendonly.aof"))
	if err != nil {
		t.Fatalf("AOF file missing: %v", err)
	}
	if fi.Size() == 0 {
		t.Fatal("expected non-empty AOF file after concurrent appends")
	}
}

// TestCloseWithoutAppends exercises the shutdown path when no command was
// ever appended (empty AOF file, no everysec goroutines in flight).
func TestCloseWithoutAppends(t *testing.T) {
	p := NewPersistencePlugin(store.NewStore(), t.TempDir(), "always", time.Hour)
	if err := p.Init(nil); err != nil {
		t.Fatalf("Init failed: %v", err)
	}
	if err := p.Close(); err != nil {
		t.Fatalf("Close failed: %v", err)
	}
}
