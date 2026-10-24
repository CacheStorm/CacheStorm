package command

import (
	"bytes"
	"sync"
	"testing"
	"time"

	"github.com/cachestorm/cachestorm/internal/resp"
	"github.com/cachestorm/cachestorm/internal/store"
)

func blockingFIFORun(r *Router, s *store.Store, args ...string) string {
	var out bytes.Buffer
	argv := make([][]byte, len(args)-1)
	for i, a := range args[1:] {
		argv[i] = []byte(a)
	}
	ctx := NewContext(args[0], argv, s, resp.NewWriter(&out))
	if err := r.Execute(ctx); err != nil {
		ctx.Writer.WriteError(err.Error())
	}
	return out.String()
}

// Blocked clients are served in the order they blocked. A write used to wake
// every waiter, so whichever goroutine ran first took the element.
func TestBlockedPopsAreServedFirstComeFirstServed(t *testing.T) {
	for round := 0; round < 10; round++ {
		r := execDispatchRouter()
		s := store.NewStore()
		const waiters = 4
		results := make([]string, waiters)
		var wg sync.WaitGroup
		for i := 0; i < waiters; i++ {
			wg.Add(1)
			go func(i int) {
				defer wg.Done()
				results[i] = blockingFIFORun(r, s, "BLPOP", "q", "0.5")
			}(i)
			// Let waiter i block before waiter i+1 does.
			deadline := time.Now().Add(time.Second)
			for blockedOn(s, "q") <= i && time.Now().Before(deadline) {
				time.Sleep(time.Millisecond)
			}
		}
		blockingFIFORun(r, s, "RPUSH", "q", "a")
		blockingFIFORun(r, s, "RPUSH", "q", "b", "c")
		wg.Wait()

		want := []string{"a", "b", "c"}
		for i, w := range want {
			if exp := "*2\r\n$1\r\nq\r\n$1\r\n" + w + "\r\n"; results[i] != exp {
				t.Fatalf("round %d: waiter %d got %q, want %q (all: %q)", round, i, results[i], w, results)
			}
		}
		if results[3] != "*-1\r\n" {
			t.Fatalf("round %d: the last waiter got %q, want a timeout", round, results[3])
		}
	}
}

func blockedOn(s *store.Store, key string) int {
	return s.KeyNotifier().WaiterCount(key)
}
