package command

import (
	"strconv"
	"strings"
	"sync"
	"testing"

	"github.com/cachestorm/cachestorm/internal/store"
)

// resilienceFamilyCase extends the shared family case with a flag for the
// families whose writer can only fire once: PROMISE transitions from pending
// to resolved a single time, so pre-running the writer would leave the
// overlapping read unprovoked.
type resilienceFamilyCase struct {
	registryFamilyCase
	skipWriterPrecheck bool
}

// runResilienceFamilyRace drives one registry family's writer against its
// readers.
//
// The read paths took the map lock only to look the entry up and then
// released it, reading the entry's fields with no lock at all while the
// family's writer updated them under the write lock: a data race.
//
// Run with -race this pins the data race. Each family is a subtest so a
// single family can be run in isolation
// (`-run 'TestResilienceRegistryReadersAreSynchronised/<family>'`).
func runResilienceFamilyRace(t *testing.T, tc resilienceFamilyCase) {
	t.Helper()

	s := store.NewStore()
	router := NewRouter()
	RegisterResilienceCommands(router)

	// The setup creates the entry the family works on. The last setup reply
	// is available to later commands as "{last}".
	last := ""
	for _, step := range tc.setup {
		reply, err := registryReply(router, s, step[0], step[1:]...)
		if err != nil {
			t.Fatalf("setup %s: unexpected error: %v", step[0], err)
		}
		if strings.HasPrefix(reply, "-") {
			t.Fatalf("setup %s: error reply %q", step[0], reply)
		}
		last = registryBulkValue(reply)
	}

	expand := func(args []string, iter int) []string {
		out := make([]string, len(args))
		for i, a := range args {
			switch a {
			case "{last}":
				out[i] = last
			case "{i}":
				out[i] = strconv.Itoa(iter)
			default:
				out[i] = a
			}
		}
		return out
	}

	// The writer must succeed, otherwise the family would look clean for the
	// wrong reason.
	if !tc.skipWriterPrecheck {
		for _, w := range tc.writer {
			reply, err := registryReply(router, s, w[0], expand(w[1:], 0)...)
			if err != nil {
				t.Fatalf("writer %s: unexpected error: %v", w[0], err)
			}
			if strings.HasPrefix(reply, "-") {
				t.Fatalf("writer %s: error reply %q", w[0], reply)
			}
		}
	}
	for _, r := range tc.reader {
		reply, err := registryReply(router, s, r[0], expand(r[1:], 0)...)
		if err != nil {
			t.Fatalf("reader %s: unexpected error: %v", r[0], err)
		}
		if strings.HasPrefix(reply, "-") {
			t.Fatalf("reader %s: error reply %q", r[0], reply)
		}
	}

	const iterations = 2000

	var wg sync.WaitGroup
	start := make(chan struct{})
	// A reader must have looked the entry up before the first write of the
	// loop, otherwise a one-shot writer (PROMISE transitions once) can
	// finish before any reader starts and the overlap never happens.
	readersStarted := make(chan struct{})
	var readersOnce sync.Once

	for w := 0; w < 2; w++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			<-start
			<-readersStarted
			for i := 0; i < iterations; i++ {
				for _, cmd := range tc.writer {
					ctx, _ := bufCtx(cmd[0], bytesArgs(expand(cmd[1:], i)...), s)
					if err := router.Execute(ctx); err != nil {
						t.Errorf("%s: unexpected error: %v", cmd[0], err)
						return
					}
				}
			}
		}()
	}

	for r := 0; r < 2; r++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			<-start
			for i := 0; i < iterations; i++ {
				for _, cmd := range tc.reader {
					ctx, _ := bufCtx(cmd[0], bytesArgs(expand(cmd[1:], i)...), s)
					if err := router.Execute(ctx); err != nil {
						t.Errorf("%s: unexpected error: %v", cmd[0], err)
						return
					}
				}
				if i == 0 {
					readersOnce.Do(func() { close(readersStarted) })
				}
			}
		}()
	}

	close(start)
	wg.Wait()
}

// TestResilienceRegistryReadersAreSynchronised covers every registry family
// in internal/command/resilience_commands.go that keeps a shared entry behind
// a package-level map.
func TestResilienceRegistryReadersAreSynchronised(t *testing.T) {
	cases := []resilienceFamilyCase{
		{
			registryFamilyCase: registryFamilyCase{
				name:   "circuitx",
				setup:  [][]string{{"CIRCUITX.CREATE", "k", "3", "1000"}},
				writer: [][]string{{"CIRCUITX.RESET", "k"}},
				reader: [][]string{{"CIRCUITX.STATUS", "k"}, {"CIRCUITX.METRICS", "k"}},
			},
		},
		{
			registryFamilyCase: registryFamilyCase{
				name:   "bulkhead",
				setup:  [][]string{{"BULKHEAD.CREATE", "k", "10"}},
				writer: [][]string{{"BULKHEAD.ACQUIRE", "k"}, {"BULKHEAD.RELEASE", "k"}},
				reader: [][]string{{"BULKHEAD.STATUS", "k"}},
			},
		},
		{
			registryFamilyCase: registryFamilyCase{
				name:   "telemetry",
				setup:  [][]string{{"TELEMETRY.RECORD", "k", "1000", "1.5"}},
				writer: [][]string{{"TELEMETRY.RECORD", "k", "1000", "2.5"}},
				reader: [][]string{{"TELEMETRY.QUERY", "k"}},
			},
		},
		{
			registryFamilyCase: registryFamilyCase{
				name:   "promise",
				setup:  [][]string{{"PROMISE.CREATE", "k"}},
				writer: [][]string{{"PROMISE.RESOLVE", "{last}", "value"}},
				reader: [][]string{{"PROMISE.STATUS", "{last}"}, {"PROMISE.AWAIT", "{last}", "1"}},
			},
			skipWriterPrecheck: true,
		},
		{
			registryFamilyCase: registryFamilyCase{
				name:   "future",
				setup:  [][]string{{"FUTURE.CREATE", "k"}},
				writer: [][]string{{"FUTURE.COMPLETE", "{last}", "value"}},
				reader: [][]string{{"FUTURE.GET", "{last}"}},
			},
			skipWriterPrecheck: true,
		},
		{
			registryFamilyCase: registryFamilyCase{
				name:   "streamproc",
				setup:  [][]string{{"STREAMPROC.CREATE", "k"}},
				writer: [][]string{{"STREAMPROC.PUSH", "k", "payload"}, {"STREAMPROC.POP", "k"}},
				reader: [][]string{{"STREAMPROC.PEEK", "k"}},
			},
		},
		{
			registryFamilyCase: registryFamilyCase{
				name:   "eventsourcing",
				setup:  [][]string{{"EVENTSOURCING.APPEND", "k", "type", "data"}},
				writer: [][]string{{"EVENTSOURCING.APPEND", "k", "type", "data"}},
				reader: [][]string{{"EVENTSOURCING.REPLAY", "k"}, {"EVENTSOURCING.GET", "k", "1"}},
			},
		},
		{
			registryFamilyCase: registryFamilyCase{
				name:   "backpressure",
				setup:  [][]string{{"BACKPRESSURE.CREATE", "k", "100", "50"}},
				writer: [][]string{{"BACKPRESSURE.CHECK", "k", "1"}},
				reader: [][]string{{"BACKPRESSURE.STATUS", "k"}},
			},
		},
		{
			registryFamilyCase: registryFamilyCase{
				name:   "coalesce",
				setup:  [][]string{{"COALESCE.CREATE", "k"}},
				writer: [][]string{{"COALESCE.ADD", "k", "value"}},
				reader: [][]string{{"COALESCE.GET", "k"}},
			},
		},
		{
			registryFamilyCase: registryFamilyCase{
				name:   "aggregator",
				setup:  [][]string{{"AGGREGATOR.CREATE", "k", "sum"}},
				writer: [][]string{{"AGGREGATOR.ADD", "k", "value"}},
				reader: [][]string{{"AGGREGATOR.GET", "k"}},
			},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			runResilienceFamilyRace(t, tc)
		})
	}
}
