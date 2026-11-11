package command

import (
	"testing"

	"github.com/cachestorm/cachestorm/internal/store"
)

// An unregistered command is unknown to clients but still replays, so an
// AOF written while it was served loads after it is turned off.
func TestUnregisteredCommandStillReplays(t *testing.T) {
	r := NewRouter()
	RegisterEventCommands(r)
	r.Unregister("queue.push")
	if _, ok := r.Get("QUEUE.PUSH"); ok {
		t.Fatal("QUEUE.PUSH is still served")
	}
	if _, ok := r.Commands()["QUEUE.PUSH"]; ok {
		t.Fatal("QUEUE.PUSH is still listed")
	}
	s := store.NewStore()
	ctx := NewContext("QUEUE.PUSH", [][]byte{[]byte("q"), []byte("a")}, s, nil)
	if err := r.ExecuteSilent(ctx); err != nil || ctx.Writer.Errors() > 0 {
		t.Fatalf("replay: %v", err)
	}
	if !s.Exists("q") {
		t.Fatal("the replayed push left no list")
	}
}
