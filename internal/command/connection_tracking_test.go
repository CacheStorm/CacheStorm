package command

import (
	"strings"
	"sync"
	"testing"

	"github.com/cachestorm/cachestorm/internal/store"
)

// TestConnectionTrackingIsReportedByTheConnectionCommands checks that the
// server's connection tracking reaches the CONNECTION.* commands: a tracked
// connection is reported by CONNECTION.INFO, CONNECTION.LIST and
// CONNECTION.COUNT, its command counter advances, and untracking removes it.
func TestConnectionTrackingIsReportedByTheConnectionCommands(t *testing.T) {
	s := store.NewStore()
	router := NewRouter()
	RegisterMVCCCommands(router)

	run := func(cmd string, args ...string) string {
		ctx, buf := bufCtx(cmd, bytesArgs(args...), s)
		if err := router.Execute(ctx); err != nil {
			return "-ERR " + err.Error()
		}
		return buf.String()
	}

	TrackConnection(101, "10.0.0.7:4242")
	t.Cleanup(func() { UntrackConnection(101) })

	info := run("CONNECTION.INFO", "101")
	if !strings.Contains(info, "10.0.0.7:4242") {
		t.Fatalf("CONNECTION.INFO reply %q does not report the tracked connection", info)
	}
	if !strings.Contains(info, ":0\r\n") {
		t.Fatalf("CONNECTION.INFO reply %q does not report a zero command count", info)
	}

	list := run("CONNECTION.LIST")
	if !strings.Contains(list, "10.0.0.7:4242") {
		t.Fatalf("CONNECTION.LIST reply %q does not list the tracked connection", list)
	}

	count := run("CONNECTION.COUNT")
	if !strings.Contains(count, ":1\r\n") {
		t.Fatalf("CONNECTION.COUNT reply %q does not count the tracked connection", count)
	}

	BumpConnectionCommand(101)
	info = run("CONNECTION.INFO", "101")
	if !strings.Contains(info, ":1\r\n") {
		t.Fatalf("CONNECTION.INFO reply %q does not report the issued command", info)
	}

	UntrackConnection(101)
	info = run("CONNECTION.INFO", "101")
	if strings.Contains(info, "10.0.0.7:4242") {
		t.Fatalf("CONNECTION.INFO reply %q still reports an untracked connection", info)
	}
}

// TestConnectionCommandCountIsSafeAgainstTheReader races the real tracking
// writer against the real reader. The counter lives in the shared entry, so
// the writer must take the registry write lock and the reader the read lock.
func TestConnectionCommandCountIsSafeAgainstTheReader(t *testing.T) {
	s := store.NewStore()
	router := NewRouter()
	RegisterMVCCCommands(router)

	TrackConnection(55, "10.0.0.8:5252")
	t.Cleanup(func() { UntrackConnection(55) })

	const iterations = 2000

	var wg sync.WaitGroup
	start := make(chan struct{})

	wg.Add(1)
	go func() {
		defer wg.Done()
		<-start
		for i := 0; i < iterations; i++ {
			BumpConnectionCommand(55)
		}
	}()

	wg.Add(1)
	go func() {
		defer wg.Done()
		<-start
		for i := 0; i < iterations; i++ {
			ctx, _ := bufCtx("CONNECTION.INFO", bytesArgs("55"), s)
			if err := router.Execute(ctx); err != nil {
				t.Errorf("CONNECTION.INFO: %v", err)
				return
			}
		}
	}()

	close(start)
	wg.Wait()
}
