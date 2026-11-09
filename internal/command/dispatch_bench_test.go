package command

import (
	"io"
	"strconv"
	"sync/atomic"
	"testing"

	"github.com/cachestorm/cachestorm/internal/resp"
	"github.com/cachestorm/cachestorm/internal/store"
)

// BenchmarkDispatchParallel runs SET and GET through Router.Execute from
// every core at once, each goroutine on its own keys, with the commit hooks
// installed as the server installs them with the AOF on. It measures what
// dispatch costs when it is shared across cores.
func BenchmarkDispatchParallel(b *testing.B) {
	for _, name := range []string{"SET", "GET"} {
		b.Run(name, func(b *testing.B) {
			r := newMemAcctRouter()
			r.SetCommitHooks(
				func(string, [][]byte) error { return nil },
				func(*Context, int, string, [][]byte) error { return nil },
				func(*Context, []ExecutedCommand) error { return nil },
			)
			// As with the AOF on: writes are checked and their replies
			// captured until they are persisted; reads are not.
			r.SetPersistFilter(func(cmd string, _ [][]byte) bool { return cmd == "SET" })
			s := store.NewStore()
			var next atomic.Int64
			b.RunParallel(func(pb *testing.PB) {
				key := []byte("key:" + strconv.FormatInt(next.Add(1), 10))
				args := [][]byte{key, []byte("value")}
				if name == "GET" {
					args = args[:1]
					memAcctRun(r, s, nil, "SET", string(key), "value")
				}
				w := resp.NewWriter(io.Discard)
				ctx := NewContext(name, args, s, w)
				for pb.Next() {
					ResetConnectionContext(ctx, name, args, s, w, 1, "", nil, ctx.StartTime)
					if err := r.Execute(ctx); err != nil {
						b.Fatal(err)
					}
				}
			})
		})
	}
}
