package command

import (
	"bytes"
	"fmt"
	"strings"
	"sync"
	"testing"

	"github.com/cachestorm/cachestorm/internal/resp"
	"github.com/cachestorm/cachestorm/internal/store"
)

// Regression: DUMP serialized a key's value by iterating its collections
// WITHOUT holding the value's read lock, while the command handlers mutate
// those exact collections under their write lock:
//
//	case *store.HashValue:  for k, val := range v.Fields
//	case *store.ListValue:  for _, el := range v.Elements
//	case *store.SetValue:   for k := range v.Members
//
// A map iteration concurrent with a map write is a PROCESS-FATAL runtime error
// ("fatal error: concurrent map iteration and map write"), so a client issuing
// DUMP against a key another client is writing to could kill the server.
//
// Each arm now serializes under the matching read lock.

func dumpRaceRouter() *Router {
	router := NewRouter()
	RegisterKeyCommands(router)
	RegisterHashCommands(router)
	RegisterSetCommands(router)
	RegisterListCommands(router)
	RegisterSortedSetCommands(router)
	return router
}

func dumpRaceRun(t *testing.T, s *store.Store, router *Router, cmd string, args ...string) string {
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

func dumpRaceSeedHash(t *testing.T, s *store.Store, router *Router, key string, n int) {
	t.Helper()
	fields := make([]string, 0, 2*n)
	for i := 0; i < n; i++ {
		fields = append(fields, fmt.Sprintf("seed%03d", i), fmt.Sprintf("v%03d", i))
	}
	dumpRaceRun(t, s, router, "HSET", append([]string{key}, fields...)...)
}

func dumpRaceSeedSet(t *testing.T, s *store.Store, router *Router, key string, n int) {
	t.Helper()
	for i := 0; i < n; i++ {
		dumpRaceRun(t, s, router, "SADD", key, fmt.Sprintf("seed%03d", i))
	}
}

// TestDumpHashIsRaceFreeWithHashWriters is the primary case: DUMP's HashValue
// arm against the locked HSET writer on the same key.
func TestDumpHashIsRaceFreeWithHashWriters(t *testing.T) {
	s := store.NewStore()
	router := dumpRaceRouter()
	dumpRaceSeedHash(t, s, router, "h", 256)

	var wg sync.WaitGroup
	wg.Add(2)

	go func() {
		defer wg.Done()
		for i := 0; i < 300; i++ {
			dumpRaceRun(t, s, router, "HSET", "h", fmt.Sprintf("w%d", i%64), "x")
		}
	}()

	go func() {
		defer wg.Done()
		for i := 0; i < 300; i++ {
			dumpRaceRun(t, s, router, "DUMP", "h")
		}
	}()

	wg.Wait()
}

// TestDumpSetIsRaceFreeWithSetWriters covers the SetValue arm against both the
// additive and the deleting writer.
func TestDumpSetIsRaceFreeWithSetWriters(t *testing.T) {
	s := store.NewStore()
	router := dumpRaceRouter()
	dumpRaceSeedSet(t, s, router, "st", 256)

	var wg sync.WaitGroup
	wg.Add(2)

	go func() {
		defer wg.Done()
		for i := 0; i < 300; i++ {
			dumpRaceRun(t, s, router, "SADD", "st", fmt.Sprintf("w%d", i%64))
			dumpRaceRun(t, s, router, "SREM", "st", fmt.Sprintf("seed%03d", i%128))
		}
	}()

	go func() {
		defer wg.Done()
		for i := 0; i < 300; i++ {
			dumpRaceRun(t, s, router, "DUMP", "st")
		}
	}()

	wg.Wait()
}

// TestDumpListIsRaceFreeWithListWriters covers the ListValue arm.
func TestDumpListIsRaceFreeWithListWriters(t *testing.T) {
	s := store.NewStore()
	router := dumpRaceRouter()
	for i := 0; i < 256; i++ {
		dumpRaceRun(t, s, router, "RPUSH", "l", fmt.Sprintf("seed%03d", i))
	}

	var wg sync.WaitGroup
	wg.Add(2)

	go func() {
		defer wg.Done()
		for i := 0; i < 300; i++ {
			dumpRaceRun(t, s, router, "RPUSH", "l", fmt.Sprintf("w%d", i%64))
			dumpRaceRun(t, s, router, "LPOP", "l")
		}
	}()

	go func() {
		defer wg.Done()
		for i := 0; i < 300; i++ {
			dumpRaceRun(t, s, router, "DUMP", "l")
		}
	}()

	wg.Wait()
}

// TestDumpSortedSetIsRaceFreeWithSortedSetWriters covers the SortedSetValue arm,
// which reaches Members through GetSortedRange rather than a direct range.
func TestDumpSortedSetIsRaceFreeWithSortedSetWriters(t *testing.T) {
	s := store.NewStore()
	router := dumpRaceRouter()
	for i := 0; i < 256; i++ {
		dumpRaceRun(t, s, router, "ZADD", "z", fmt.Sprintf("%d", i), fmt.Sprintf("m%03d", i))
	}

	var wg sync.WaitGroup
	wg.Add(2)

	go func() {
		defer wg.Done()
		for i := 0; i < 300; i++ {
			dumpRaceRun(t, s, router, "ZADD", "z", fmt.Sprintf("%d", 10000+i), fmt.Sprintf("w%d", i%64))
			dumpRaceRun(t, s, router, "ZREM", "z", fmt.Sprintf("m%03d", i%128))
		}
	}()

	go func() {
		defer wg.Done()
		for i := 0; i < 300; i++ {
			dumpRaceRun(t, s, router, "DUMP", "z")
		}
	}()

	wg.Wait()
}

// TestDumpControlsUncontended is the control: against collections nobody
// mutates, DUMP must still serialize every element of every type. This proves
// the harness reaches real DUMP behaviour, so a race failure above is the
// defect and not a broken setup.
func TestDumpControlsUncontended(t *testing.T) {
	s := store.NewStore()
	router := dumpRaceRouter()

	dumpRaceSeedHash(t, s, router, "h", 4)
	dumpRaceSeedSet(t, s, router, "st", 4)
	for i := 0; i < 4; i++ {
		dumpRaceRun(t, s, router, "RPUSH", "l", fmt.Sprintf("seed%03d", i))
	}
	for i := 0; i < 4; i++ {
		dumpRaceRun(t, s, router, "ZADD", "z", fmt.Sprintf("%d", i), fmt.Sprintf("m%03d", i))
	}

	dumpBulk := func(key string) string {
		t.Helper()
		raw := dumpRaceRun(t, s, router, "DUMP", key)
		v, err := resp.NewReader(bytes.NewReader([]byte(raw))).ReadValue()
		if err != nil {
			t.Fatalf("CONTROL FAIL: unparsable DUMP %s reply %q: %v", key, raw, err)
		}
		if v.Type != resp.TypeBulkString {
			t.Fatalf("CONTROL FAIL: DUMP %s returned type %v, want a bulk string", key, v.Type)
		}
		return string(v.Bulk)
	}

	// Hash arm writes each field as "key=value&": 4 fields => 4 '&'.
	if got := strings.Count(dumpBulk("h"), "&"); got != 4 {
		t.Fatalf("CONTROL FAIL: DUMP h serialized %d fields, want 4", got)
	}
	// Set arm writes each member followed by ',': 4 members => 4 ','.
	if got := strings.Count(dumpBulk("st"), ","); got != 4 {
		t.Fatalf("CONTROL FAIL: DUMP st serialized %d members, want 4", got)
	}
	// List arm writes each element followed by ',': 4 elements => 4 ','.
	if got := strings.Count(dumpBulk("l"), ","); got != 4 {
		t.Fatalf("CONTROL FAIL: DUMP l serialized %d elements, want 4", got)
	}
	// Sorted set arm writes "member:score," per member: 4 members => 4 ','.
	if got := strings.Count(dumpBulk("z"), ","); got != 4 {
		t.Fatalf("CONTROL FAIL: DUMP z serialized %d members, want 4", got)
	}
	if got := dumpBulk("h"); !strings.HasPrefix(got, "CACHSTORM001") {
		t.Fatalf("CONTROL FAIL: DUMP h = %q, want a CACHSTORM001 prefix", got)
	}

	// A missing key still yields a null bulk string.
	raw := dumpRaceRun(t, s, router, "DUMP", "nosuchkey")
	v, err := resp.NewReader(bytes.NewReader([]byte(raw))).ReadValue()
	if err != nil {
		t.Fatalf("CONTROL FAIL: unparsable DUMP missing-key reply %q: %v", raw, err)
	}
	if v.Type != resp.TypeBulkString || !v.IsNull {
		t.Fatalf("CONTROL FAIL: DUMP of a missing key returned type %v null=%v, want a null bulk string", v.Type, v.IsNull)
	}
}
