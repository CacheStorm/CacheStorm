package command

import (
	"strconv"
	"testing"

	"github.com/cachestorm/cachestorm/internal/store"
)

func TestInvalidatePropagatesChunkedDeletesX(t *testing.T) {
	r := NewRouter()
	RegisterTagCommands(r)
	s := store.NewStore()
	n := deleteChunk*2 + 5
	for i := 0; i < n; i++ {
		if err := s.Set("k"+strconv.Itoa(i), &store.StringValue{Data: []byte("v")}, store.SetOptions{Tags: []string{"t"}}); err != nil {
			t.Fatal(err)
		}
	}
	ctx := newTestCtx("INVALIDATE", [][]byte{[]byte("t")}, s)
	if err := r.Execute(ctx); err != nil {
		t.Fatal(err)
	}
	cmds, set := ctx.Propagation()
	if !set || len(cmds) != 3 {
		t.Fatalf("propagated %d commands (set=%v), want 3 DELs", len(cmds), set)
	}
	seen := map[string]bool{}
	for _, c := range cmds {
		if c.Name != "DEL" || len(c.Args) > deleteChunk {
			t.Fatalf("propagated %s with %d keys", c.Name, len(c.Args))
		}
		for _, a := range c.Args {
			seen[string(a)] = true
		}
	}
	if len(seen) != n {
		t.Fatalf("the DELs name %d distinct keys, want %d", len(seen), n)
	}

	empty := newTestCtx("INVALIDATE", [][]byte{[]byte("none")}, s)
	if err := r.Execute(empty); err != nil {
		t.Fatal(err)
	}
	if cmds, set := empty.Propagation(); !set || len(cmds) != 0 {
		t.Fatalf("an INVALIDATE deleting nothing propagated %v", cmds)
	}
}
