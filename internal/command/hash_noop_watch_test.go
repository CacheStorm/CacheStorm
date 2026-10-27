package command

import (
	"strings"
	"testing"

	"github.com/cachestorm/cachestorm/internal/store"
)

func TestHashNoOpWatchX(t *testing.T) {
	for _, name := range []string{"HSETNX", "HINCRBY", "HINCRBYFLOAT"} {
		t.Run(name, func(t *testing.T) {
			s := store.NewStore()
			r := NewRouter()
			RegisterHashCommands(r)
			if got := runEntryTransfer(t, s, nil, r, "HSET", "h", "f", "text"); got != ":1\r\n" {
				t.Fatal("CONTROL", got)
			}
			version := s.WatchKey("h")
			defer s.UnwatchKey("h")
			got := runEntryTransfer(t, s, nil, r, name, "h", "f", "1")
			if name == "HSETNX" && got != ":0\r\n" || name != "HSETNX" && !strings.HasPrefix(got, "-ERR") {
				t.Fatal("CONTROL reply", got)
			}
			t.Logf("EXPECTED: unchanged WATCH=%d | ACTUAL: %d", version, s.GetVersion("h"))
			if s.GetVersion("h") != version {
				t.Error("no-op/rejection dirtied WATCH")
			}
			if got := runEntryTransfer(t, s, nil, r, "HSET", "h", "g", "new"); got != ":1\r\n" || s.GetVersion("h") == version {
				t.Fatal("CONTROL real write did not invalidate WATCH")
			}
		})
	}
}
