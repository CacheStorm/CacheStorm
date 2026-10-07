package command

import (
	"math"
	"strings"
	"testing"

	"github.com/cachestorm/cachestorm/internal/store"
)

func TestBlockingMovePreservesDataOnTypeError(t *testing.T) {
	for _, cmd := range []string{"BLMOVE", "BRPOPLPUSH"} {
		t.Run(cmd, func(t *testing.T) {
			s := store.NewStore()
			r := NewRouter()
			RegisterListCommands(r)
			move := func(src, dst string) string {
				if cmd == "BLMOVE" {
					return runCmd(t, s, r, cmd, src, dst, "RIGHT", "LEFT", "0")
				}
				return runCmd(t, s, r, cmd, src, dst, "0")
			}
			runCmd(t, s, r, "RPUSH", "src", "a", "b")
			if err := s.Set("bad", &store.StringValue{Data: []byte("untouched")}, store.SetOptions{}); err != nil {
				t.Fatal(err)
			}
			for i := 0; i < 2; i++ {
				if got := move("src", "bad"); !strings.HasPrefix(got, "-WRONGTYPE") {
					t.Fatalf("expected WRONGTYPE, got %q", got)
				}
				if got := runCmd(t, s, r, "LRANGE", "src", "0", "-1"); got != "*2\r\n$1\r\na\r\n$1\r\nb\r\n" {
					t.Fatalf("source changed: %q", got)
				}
				entry, _ := s.Get("bad")
				if string(entry.Value.(*store.StringValue).Data) != "untouched" {
					t.Fatal("destination changed")
				}
			}
			if got := move("bad", "unused"); !strings.HasPrefix(got, "-WRONGTYPE") {
				t.Fatalf("source type error: %q", got)
			}
			if _, exists := s.Get("unused"); exists {
				t.Fatal("created destination after source type error")
			}
			if got := move("missing", "unused"); got != "_\r\n" {
				t.Fatalf("missing source: %q", got)
			}
			if got := move("src", "dst"); got != "$1\r\nb\r\n" {
				t.Fatalf("normal move: %q", got)
			}
			if got := move("src", "dst"); got != "$1\r\na\r\n" {
				t.Fatalf("last move: %q", got)
			}
			if _, exists := s.Get("src"); exists {
				t.Fatal("exhausted distinct source retained")
			}
			runCmd(t, s, r, "RPUSH", "same", "x")
			entry, _ := s.Get("same")
			entry.ExpiresAt = math.MaxInt64
			if got := move("same", "same"); got != "$1\r\nx\r\n" {
				t.Fatalf("same-key move: %q", got)
			}
			after, _ := s.Get("same")
			if after != entry || after.ExpiresAt != math.MaxInt64 {
				t.Fatal("same-key entry or expiration changed")
			}
		})
	}
}
