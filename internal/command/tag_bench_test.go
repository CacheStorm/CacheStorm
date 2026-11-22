package command

import (
	"io"
	"strconv"
	"sync/atomic"
	"testing"

	"github.com/cachestorm/cachestorm/internal/resp"
	"github.com/cachestorm/cachestorm/internal/store"
)

// BenchmarkTagWrites runs SET, SETTAG and ADDTAG from parallel clients on
// distinct keys sharing a few tags, as a cache tagging its entries does.
func BenchmarkTagWrites(b *testing.B) {
	for _, name := range []string{"SET", "SETTAG", "ADDTAG"} {
		b.Run(name, func(b *testing.B) {
			r := newMemAcctRouter()
			RegisterTagCommands(r)
			s := store.NewStore()
			var next atomic.Int64
			b.RunParallel(func(pb *testing.PB) {
				w := resp.NewWriter(io.Discard)
				id := next.Add(1)
				i := 0
				for pb.Next() {
					i++
					key := []byte("key:" + strconv.FormatInt(id, 10) + ":" + strconv.Itoa(i&4095))
					var args [][]byte
					switch name {
					case "SET":
						args = [][]byte{key, []byte("value")}
					case "SETTAG":
						args = [][]byte{key, []byte("value"), []byte("t" + strconv.Itoa(i%1000)), []byte("u" + strconv.Itoa(i%50))}
					default:
						if i < 4096 {
							memAcctRun(r, s, nil, "SET", string(key), "value")
						}
						args = [][]byte{key, []byte("x" + strconv.Itoa(i%100))}
					}
					ctx := NewContext(name, args, s, w)
					if err := r.Execute(ctx); err != nil {
						b.Fatal(err)
					}
				}
			})
		})
	}
}
