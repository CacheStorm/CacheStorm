package command

import (
	"fmt"
	"strconv"
	"strings"
	"sync"
	"testing"

	"github.com/cachestorm/cachestorm/internal/store"
)

// registryArrayField returns the bulk value that follows the given key in a
// RESP array reply, e.g. the message id in a MSGQUEUE.CONSUME reply.
func registryArrayField(reply, key string) string {
	if !strings.HasPrefix(reply, "*") {
		return ""
	}
	parts := strings.Split(reply, "\r\n")
	// parts[0] is the array header, then "$len" / value pairs follow.
	for i := 1; i+3 < len(parts); i += 2 {
		if parts[i+1] == key {
			return parts[i+3]
		}
	}
	return ""
}

// runRegistryFamilyRace drives one registry family's writer against its
// readers through the real Router.
//
// The read paths took the map lock only to look the entry up and then
// released it, reading the entry's fields with no lock at all while the
// family's writer updated them under the write lock: a data race.
//
// Run with -race this pins the data race. Each family is a subtest so a
// single family can be run in isolation
// (`-run 'Test<Family>RegistryReadersAreSynchronised/<family>'`).
//
// register is the command family's registration function; skipWriterPrecheck
// is for the families whose writer can only fire once (PROMISE transitions
// from pending to resolved a single time), where pre-running the writer would
// leave the overlapping read unprovoked.
func runRegistryFamilyRace(t *testing.T, tc registryFamilyCase, register func(*Router), skipWriterPrecheck bool) {
	t.Helper()

	s := store.NewStore()
	router := NewRouter()
	register(router)

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

	// expand substitutes "{last}" with the most recent bulk reply and "{i}"
	// with the iteration index.
	expand := func(args []string, iter int, lastValue, idValue string) []string {
		out := make([]string, len(args))
		for i, a := range args {
			switch a {
			case "{last}":
				out[i] = lastValue
			case "{id}":
				out[i] = idValue
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
	if !skipWriterPrecheck {
		// The writer tracks its own last reply so a writer step can address
		// the message the previous step published ("{last}").
		writerLast := last
		writerID := ""
		for _, w := range tc.writer {
			reply, err := registryReply(router, s, w[0], expand(w[1:], 0, writerLast, writerID)...)
			if err != nil {
				t.Fatalf("writer %s: unexpected error: %v", w[0], err)
			}
			if strings.HasPrefix(reply, "-") {
				t.Fatalf("writer %s: error reply %q", w[0], reply)
			}
			if v := registryBulkValue(reply); v != "" {
				writerLast = v
			}
			if v := registryArrayField(reply, "id"); v != "" {
				writerID = v
			}
		}
	}
	for _, r := range tc.reader {
		reply, err := registryReply(router, s, r[0], expand(r[1:], 0, last, "")...)
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
			// The writer tracks its own last reply so a writer step can
			// address the message the previous step published ("{last}").
			writerLast := last
			writerID := ""
			for i := 0; i < iterations; i++ {
				for _, cmd := range tc.writer {
					ctx, buf := bufCtx(cmd[0], bytesArgs(expand(cmd[1:], i, writerLast, writerID)...), s)
					if err := router.Execute(ctx); err != nil {
						t.Errorf("%s: unexpected error: %v", cmd[0], err)
						return
					}
					reply := buf.String()
					if v := registryBulkValue(reply); v != "" {
						writerLast = v
					}
					if v := registryArrayField(reply, "id"); v != "" {
						writerID = v
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
					ctx, _ := bufCtx(cmd[0], bytesArgs(expand(cmd[1:], i, last, "")...), s)
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

// freqCleanupX removes the frequency counters this session's tests created
// from the package-global registry, so a test leaves no state behind. It is
// named with an X suffix because another round's test file already defines
// freqCleanup, and both files live in this package.
func freqCleanupX(t *testing.T, names ...string) {
	t.Helper()
	t.Cleanup(func() {
		freqsMu.Lock()
		for _, name := range names {
			delete(freqs, name)
		}
		freqsMu.Unlock()
	})
}

// ringBufferArrayLenX parses the element count of a RESP array reply. It
// reports a malformed reply through the returned error rather than
// t.Fatalf, so it is safe to call from a spawned goroutine.
func ringBufferArrayLenX(reply string) (int, error) {
	if !strings.HasPrefix(reply, "*") {
		return 0, fmt.Errorf("expected a RESP array reply, got %q", reply)
	}
	end := strings.Index(reply, "\r\n")
	if end < 0 {
		return 0, fmt.Errorf("malformed RESP array reply: %q", reply)
	}
	n, err := strconv.Atoi(reply[1:end])
	if err != nil {
		return 0, fmt.Errorf("malformed RESP array header %q: %w", reply[:end], err)
	}
	return n, nil
}
