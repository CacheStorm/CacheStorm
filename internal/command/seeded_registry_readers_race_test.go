package command

import (
	"strconv"
	"strings"
	"sync"
	"testing"

	"github.com/cachestorm/cachestorm/internal/store"
)

// TestSeededRegistryReadersHoldTheReadLock proves that CONNECTION.INFO,
// PLUGIN.INFO and PLUGIN.CALL read their entry under the registry read lock.
//
// The readers look the entry up, release the read lock and only then read the
// entry's fields. That is safe today only because every writer publishes a
// fresh entry instead of mutating the one a reader holds. The moment a writer
// updates an entry in place — the server bumping a connection's command
// counter, a plugin being enabled or disabled — the unlocked read races with
// it.
//
// The writer here takes the registry WRITE lock and mutates the seeded entry
// in place, exactly as such a writer would. The readers must hold the read
// lock across their field reads to be excluded from it.
func TestSeededRegistryReadersHoldTheReadLock(t *testing.T) {
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

	connectionsMu.Lock()
	connections[42] = &ConnectionInfo{ID: 42, RemoteAddr: "127.0.0.1:1234", ConnectedAt: 1}
	connectionsMu.Unlock()
	t.Cleanup(func() {
		connectionsMu.Lock()
		delete(connections, 42)
		connectionsMu.Unlock()
	})

	pluginsMu.Lock()
	plugins["seeded"] = &Plugin{Name: "seeded", Enabled: true, LoadedAt: 1, Functions: map[string]string{}}
	pluginsMu.Unlock()
	t.Cleanup(func() {
		pluginsMu.Lock()
		delete(plugins, "seeded")
		pluginsMu.Unlock()
	})

	const iterations = 2000

	var wg sync.WaitGroup
	start := make(chan struct{})

	// A writer that updates the connection's counter in place, under the
	// registry write lock.
	wg.Add(1)
	go func() {
		defer wg.Done()
		<-start
		for i := 0; i < iterations; i++ {
			BumpConnectionCommand(42)
		}
	}()

	// A writer that updates the plugin's flag in place, under the registry
	// write lock.
	wg.Add(1)
	go func() {
		defer wg.Done()
		<-start
		for i := 0; i < iterations; i++ {
			pluginsMu.Lock()
			plugins["seeded"].Enabled = !plugins["seeded"].Enabled
			pluginsMu.Unlock()
		}
	}()

	wg.Add(1)
	go func() {
		defer wg.Done()
		<-start
		for i := 0; i < iterations; i++ {
			run("CONNECTION.INFO", "42")
		}
	}()

	wg.Add(1)
	go func() {
		defer wg.Done()
		<-start
		for i := 0; i < iterations; i++ {
			run("PLUGIN.INFO", "seeded")
			run("PLUGIN.CALL", "seeded", "fn")
		}
	}()

	close(start)
	wg.Wait()
}

// TestSeededRegistryReaderReplyIsExact checks that the seeded readers report
// the seeded entry, so the concurrency test above cannot pass by never
// reaching the field reads.
func TestSeededRegistryReaderReplyIsExact(t *testing.T) {
	s := store.NewStore()
	router := NewRouter()
	RegisterMVCCCommands(router)

	connectionsMu.Lock()
	connections[7] = &ConnectionInfo{ID: 7, RemoteAddr: "10.0.0.1:9999", ConnectedAt: 123}
	connections[7].Commands.Store(5)
	connectionsMu.Unlock()
	t.Cleanup(func() {
		connectionsMu.Lock()
		delete(connections, 7)
		connectionsMu.Unlock()
	})

	pluginsMu.Lock()
	plugins["exact"] = &Plugin{Name: "exact", Enabled: true, LoadedAt: 77, Functions: map[string]string{}}
	pluginsMu.Unlock()
	t.Cleanup(func() {
		pluginsMu.Lock()
		delete(plugins, "exact")
		pluginsMu.Unlock()
	})

	ctx, buf := bufCtx("CONNECTION.INFO", bytesArgs(strconv.FormatInt(7, 10)), s)
	if err := router.Execute(ctx); err != nil {
		t.Fatalf("CONNECTION.INFO: %v", err)
	}
	reply := buf.String()
	for _, want := range []string{"10.0.0.1:9999", "5"} {
		if !strings.Contains(reply, want) {
			t.Fatalf("CONNECTION.INFO reply %q does not contain %q", reply, want)
		}
	}

	ctx, buf = bufCtx("PLUGIN.INFO", bytesArgs("exact"), s)
	if err := router.Execute(ctx); err != nil {
		t.Fatalf("PLUGIN.INFO: %v", err)
	}
	reply = buf.String()
	for _, want := range []string{"exact", "77"} {
		if !strings.Contains(reply, want) {
			t.Fatalf("PLUGIN.INFO reply %q does not contain %q", reply, want)
		}
	}
}
