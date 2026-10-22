package command

import (
	"bytes"
	"fmt"
	"sync"
	"testing"

	"github.com/cachestorm/cachestorm/internal/resp"
	"github.com/cachestorm/cachestorm/internal/store"
)

// Regression: cmdXINFO and cmdXGROUP read and mutated StreamValue state with NO
// lock, while StreamValue's own methods mutate it under v.mu:
//
//	cmdXINFO   lockcalls = 0   ranges stream.Groups (x3) and group.Consumers (x3)
//	cmdXGROUP  lockcalls = 0   reads/deletes group.Consumers and group.Pending
//
//	internal/store/stream.go
//	CreateGroup / DestroyGroup write v.Groups under v.mu.Lock()
//
// A map iteration concurrent with a map write is a PROCESS-FATAL runtime error,
// so XINFO GROUPS racing XGROUP CREATE/DESTROY could kill the server.
//
// Note the reentrancy constraint these fixes respect: StreamValue.Len() and
// GetRange() take v.mu.RLock() themselves, and sync.RWMutex is not reentrant,
// so the locks added here span only the direct map accesses.

func streamRaceRouter() *Router {
	router := NewRouter()
	RegisterStreamCommands(router)
	return router
}

func streamRaceRun(t *testing.T, s *store.Store, router *Router, cmd string, args ...string) string {
	t.Helper()
	parsed := make([][]byte, len(args))
	for i, a := range args {
		parsed[i] = []byte(a)
	}
	buf := &bytes.Buffer{}
	if err := router.Execute(NewContext(cmd, parsed, s, resp.NewWriter(buf))); err != nil {
		t.Fatalf("%s: Execute returned error: %v", cmd, err)
	}
	return buf.String()
}

func streamRaceSeed(t *testing.T, s *store.Store, router *Router, key string, groups int) {
	t.Helper()
	if out := streamRaceRun(t, s, router, "XADD", key, "*", "f", "v"); out == "" {
		t.Fatalf("XADD seed failed for %s", key)
	}
	for i := 0; i < groups; i++ {
		streamRaceRun(t, s, router, "XGROUP", "CREATE", key, fmt.Sprintf("g%03d", i), "0-0")
	}
}

// TestXINFOGroupsIsRaceFreeWithGroupCreate is the primary case: XINFO GROUPS
// ranges stream.Groups while CreateGroup writes it under v.mu.Lock().
func TestXINFOGroupsIsRaceFreeWithGroupCreate(t *testing.T) {
	s := store.NewStore()
	router := streamRaceRouter()
	streamRaceSeed(t, s, router, "st", 32)

	var wg sync.WaitGroup
	wg.Add(2)

	go func() {
		defer wg.Done()
		for i := 0; i < 300; i++ {
			streamRaceRun(t, s, router, "XGROUP", "CREATE", "st", fmt.Sprintf("w%03d", i), "0-0")
		}
	}()

	go func() {
		defer wg.Done()
		for i := 0; i < 300; i++ {
			streamRaceRun(t, s, router, "XINFO", "GROUPS", "st")
		}
	}()

	wg.Wait()
}

// TestXINFOGroupsIsRaceFreeWithGroupDestroy is the deleting writer, the variant
// most likely to trip the runtime's map-corruption abort. It also exercises
// XINFO STREAM, whose group COUNT was read unlocked.
func TestXINFOGroupsIsRaceFreeWithGroupDestroy(t *testing.T) {
	s := store.NewStore()
	router := streamRaceRouter()
	streamRaceSeed(t, s, router, "st", 256)

	var wg sync.WaitGroup
	wg.Add(2)

	go func() {
		defer wg.Done()
		for i := 0; i < 300; i++ {
			streamRaceRun(t, s, router, "XGROUP", "DESTROY", "st", fmt.Sprintf("g%03d", i%256))
			streamRaceRun(t, s, router, "XGROUP", "CREATE", "st", fmt.Sprintf("g%03d", i%256), "0-0")
		}
	}()

	go func() {
		defer wg.Done()
		for i := 0; i < 300; i++ {
			streamRaceRun(t, s, router, "XINFO", "GROUPS", "st")
			streamRaceRun(t, s, router, "XINFO", "STREAM", "st")
		}
	}()

	wg.Wait()
}

// TestXINFOConsumersIsRaceFreeWithConsumerOps covers group.Consumers: XINFO
// CONSUMERS ranges it while CREATECONSUMER inserts and DESTCONSUMER deletes.
func TestXINFOConsumersIsRaceFreeWithConsumerOps(t *testing.T) {
	s := store.NewStore()
	router := streamRaceRouter()
	streamRaceSeed(t, s, router, "st", 1)
	for i := 0; i < 64; i++ {
		streamRaceRun(t, s, router, "XGROUP", "CREATECONSUMER", "st", "g000", fmt.Sprintf("c%03d", i))
	}

	var wg sync.WaitGroup
	wg.Add(2)

	go func() {
		defer wg.Done()
		for i := 0; i < 300; i++ {
			streamRaceRun(t, s, router, "XGROUP", "DELCONSUMER", "st", "g000", fmt.Sprintf("c%03d", i%64))
			streamRaceRun(t, s, router, "XGROUP", "CREATECONSUMER", "st", "g000", fmt.Sprintf("c%03d", i%64))
		}
	}()

	go func() {
		defer wg.Done()
		for i := 0; i < 300; i++ {
			streamRaceRun(t, s, router, "XINFO", "CONSUMERS", "st", "g000")
		}
	}()

	wg.Wait()
}

// TestXINFOConsumersControlUncontended is the control: against a stream nobody
// mutates, XINFO must still report the right groups, count, and consumers, and
// the consumer add/remove round trip must still work.
func TestXINFOConsumersControlUncontended(t *testing.T) {
	s := store.NewStore()
	router := streamRaceRouter()
	streamRaceSeed(t, s, router, "st", 3)

	out := streamRaceRun(t, s, router, "XINFO", "GROUPS", "st")
	v, err := resp.NewReader(bytes.NewReader([]byte(out))).ReadValue()
	if err != nil {
		t.Fatalf("CONTROL FAIL: unparsable XINFO GROUPS reply %q: %v", out, err)
	}
	if v.Type != resp.TypeArray || len(v.Array) != 3 {
		t.Fatalf("CONTROL FAIL: XINFO GROUPS returned %d groups, want 3", len(v.Array))
	}

	// XINFO STREAM must report the group count.
	streamOut := streamRaceRun(t, s, router, "XINFO", "STREAM", "st")
	v, err = resp.NewReader(bytes.NewReader([]byte(streamOut))).ReadValue()
	if err != nil {
		t.Fatalf("CONTROL FAIL: unparsable XINFO STREAM reply %q: %v", streamOut, err)
	}
	found := false
	for i := 0; i+1 < len(v.Array); i++ {
		if string(v.Array[i].Bulk) == "groups" {
			if v.Array[i+1].Int != 3 {
				t.Fatalf("CONTROL FAIL: XINFO STREAM reported %d groups, want 3", v.Array[i+1].Int)
			}
			found = true
		}
	}
	if !found {
		t.Fatal("CONTROL FAIL: XINFO STREAM did not report a \"groups\" field")
	}

	// CREATECONSUMER then XINFO CONSUMERS then DESTCONSUMER.
	if got := streamRaceRun(t, s, router, "XGROUP", "CREATECONSUMER", "st", "g000", "alice"); got != ":1\r\n" {
		t.Fatalf("CONTROL FAIL: CREATECONSUMER returned %q, want :1", got)
	}
	consOut := streamRaceRun(t, s, router, "XINFO", "CONSUMERS", "st", "g000")
	v, err = resp.NewReader(bytes.NewReader([]byte(consOut))).ReadValue()
	if err != nil {
		t.Fatalf("CONTROL FAIL: unparsable XINFO CONSUMERS reply %q: %v", consOut, err)
	}
	if len(v.Array) != 1 {
		t.Fatalf("CONTROL FAIL: XINFO CONSUMERS returned %d consumers, want 1", len(v.Array))
	}
	if got := streamRaceRun(t, s, router, "XGROUP", "DELCONSUMER", "st", "g000", "alice"); got != ":0\r\n" {
		t.Fatalf("CONTROL FAIL: DESTCONSUMER returned %q, want :0 pending", got)
	}
	// The consumer must be gone.
	consOut = streamRaceRun(t, s, router, "XINFO", "CONSUMERS", "st", "g000")
	if v, err = resp.NewReader(bytes.NewReader([]byte(consOut))).ReadValue(); err != nil {
		t.Fatalf("CONTROL FAIL: unparsable XINFO CONSUMERS reply %q: %v", consOut, err)
	}
	if len(v.Array) != 0 {
		t.Fatalf("CONTROL FAIL: consumer survived DESTCONSUMER (%d consumers)", len(v.Array))
	}

	// A stream with no groups reports an empty array, not an error.
	streamRaceRun(t, s, router, "XADD", "empty", "*", "f", "v")
	if got := streamRaceRun(t, s, router, "XINFO", "GROUPS", "empty"); got != "*0\r\n" {
		t.Fatalf("CONTROL FAIL: XINFO GROUPS on a group-less stream returned %q, want an empty array", got)
	}
}
