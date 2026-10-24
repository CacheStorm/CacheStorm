package command

import (
	"bytes"
	"sync"
	"testing"
	"time"

	"github.com/cachestorm/cachestorm/internal/resp"
	"github.com/cachestorm/cachestorm/internal/store"
)

// Reading a stream entry does not consume it, so every client blocked in
// XREAD must receive a new entry, not only the one that blocked first.
func TestBlockedXReadersAreAllWoken(t *testing.T) {
	r := NewRouter()
	RegisterStreamCommands(r)
	s := store.NewStore()
	streamReaderRun(r, s, "XADD", "s", "1-1", "f", "v")

	const readers = 3
	results := make([]string, readers)
	var wg sync.WaitGroup
	for i := 0; i < readers; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			results[i] = streamReaderRun(r, s, "XREAD", "BLOCK", "2000", "STREAMS", "s", "$")
		}(i)
	}
	deadline := time.Now().Add(time.Second)
	for s.KeyNotifier().WaiterCount("s") < readers && time.Now().Before(deadline) {
		time.Sleep(time.Millisecond)
	}
	streamReaderRun(r, s, "XADD", "s", "2-1", "f", "w")
	wg.Wait()
	for i, got := range results {
		if got == "*-1\r\n" || got == "" {
			t.Fatalf("reader %d was not woken: %q", i, got)
		}
	}
}

func streamReaderRun(r *Router, s *store.Store, args ...string) string {
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
