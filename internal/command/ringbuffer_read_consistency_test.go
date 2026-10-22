package command

import (
	"fmt"
	"strconv"
	"strings"
	"sync"
	"testing"

	"github.com/cachestorm/cachestorm/internal/store"
)

// ringBufferRun executes cmd through the real Router and returns the raw RESP
// reply, so the test drives the production dispatch path (Router.Execute ->
// cmdRINGBUFFERREAD / cmdRINGBUFFERWRITE) rather than a copy of the logic.
func ringBufferRun(t *testing.T, router *Router, s *store.Store, cmd string, args ...string) string {
	t.Helper()
	ctx, buf := bufCtx(cmd, bytesArgs(args...), s)
	if err := router.Execute(ctx); err != nil {
		t.Fatalf("%s: unexpected error: %v", cmd, err)
	}
	return buf.String()
}

// ringBufferArrayLen parses the element count of a RESP array reply. It
// reports a malformed reply through the returned error rather than
// t.Fatalf, so it is safe to call from a spawned goroutine: t.Fatalf must
// only be called from the goroutine running the test.
func ringBufferArrayLen(reply string) (int, error) {
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

// ringBufferCleanup removes the buffers this test created from the
// package-global registry, so the test leaves no state behind for the rest
// of the package's suite.
func ringBufferCleanup(t *testing.T, names ...string) {
	t.Helper()
	t.Cleanup(func() {
		ringBuffersMu.Lock()
		for _, name := range names {
			delete(ringBuffers, name)
		}
		ringBuffersMu.Unlock()
	})
}

// TestRingBufferReadIsConsistentUnderConcurrentWrites reads a ring buffer
// while another client writes to it.
//
// The read path took the map lock only to look the buffer up and then released
// it, reading the RingBuffer's mutable fields (Count, Head, Data) with no lock
// at all while RINGBUFFER.WRITE mutated them under the write lock. That was a
// data race, and it could also crash the server: the results slice was sized
// from the Count read at allocation time, but the loop condition re-read
// Count, which a concurrent writer could grow in between, so the loop indexed
// past the end of the slice.
//
// Run with -race this pins the data race; without it, the count/element
// agreement assertion and the absence of a panic pin the crash.
func TestRingBufferReadIsConsistentUnderConcurrentWrites(t *testing.T) {
	s := store.NewStore()
	router := NewRouter()
	RegisterMoreCommands(router)

	const bufSize = 16
	const iterations = 2000

	ringBufferCleanup(t, "rb-race")

	if reply := ringBufferRun(t, router, s, "RINGBUFFER.CREATE", "rb-race", strconv.Itoa(bufSize)); reply != "+OK\r\n" {
		t.Fatalf("RINGBUFFER.CREATE reply = %q, want +OK", reply)
	}

	var wg sync.WaitGroup
	start := make(chan struct{})

	wg.Add(1)
	go func() {
		defer wg.Done()
		<-start
		for i := 0; i < iterations; i++ {
			ctx, _ := bufCtx("RINGBUFFER.WRITE", bytesArgs("rb-race", "payload"), s)
			if err := router.Execute(ctx); err != nil {
				t.Errorf("RINGBUFFER.WRITE: unexpected error: %v", err)
				return
			}
		}
	}()

	for r := 0; r < 3; r++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			<-start
			for i := 0; i < iterations; i++ {
				ctx, buf := bufCtx("RINGBUFFER.READ", bytesArgs("rb-race"), s)
				if err := router.Execute(ctx); err != nil {
					t.Errorf("RINGBUFFER.READ: unexpected error: %v", err)
					return
				}
				reply := buf.String()
				n, err := ringBufferArrayLen(reply)
				if err != nil {
					t.Errorf("RINGBUFFER.READ: %v", err)
					return
				}
				if n < 0 || n > bufSize {
					t.Errorf("RINGBUFFER.READ returned %d elements, want between 0 and %d: %q",
						n, bufSize, reply)
					return
				}
			}
		}()
	}

	close(start)
	wg.Wait()
}

// TestRingBufferSequentialReadReturnsWrittenValues is the control case: with
// no concurrency a read must return exactly the values that were written, in
// write order. It must pass both before and after the fix, so a broken
// harness cannot masquerade as the defect.
func TestRingBufferSequentialReadReturnsWrittenValues(t *testing.T) {
	s := store.NewStore()
	router := NewRouter()
	RegisterMoreCommands(router)

	ringBufferCleanup(t, "rb-ctl")

	if reply := ringBufferRun(t, router, s, "RINGBUFFER.CREATE", "rb-ctl", "8"); reply != "+OK\r\n" {
		t.Fatalf("RINGBUFFER.CREATE reply = %q, want +OK", reply)
	}
	for _, v := range []string{"a", "b", "c"} {
		if reply := ringBufferRun(t, router, s, "RINGBUFFER.WRITE", "rb-ctl", v); reply != "+OK\r\n" {
			t.Fatalf("RINGBUFFER.WRITE reply = %q, want +OK", reply)
		}
	}

	reply := ringBufferRun(t, router, s, "RINGBUFFER.READ", "rb-ctl")
	n, err := ringBufferArrayLen(reply)
	if err != nil {
		t.Fatalf("RINGBUFFER.READ: %v", err)
	}
	if n != 3 {
		t.Fatalf("RINGBUFFER.READ returned %d elements, want 3: %q", n, reply)
	}
	for _, want := range []string{"a", "b", "c"} {
		if !strings.Contains(reply, want) {
			t.Errorf("RINGBUFFER.READ reply %q is missing value %q", reply, want)
		}
	}
}
