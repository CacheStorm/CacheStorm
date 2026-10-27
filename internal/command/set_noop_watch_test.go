package command

import (
	"testing"

	"github.com/cachestorm/cachestorm/internal/store"
)

func TestSetWatchEffectsX(t *testing.T) {
	for _, name := range []string{"SADD", "SMOVE", "SUNIONSTORE", "SINTERSTORE", "SDIFFSTORE"} {
		t.Run(name, func(t *testing.T) {
			s := store.NewStore()
			r := NewRouter()
			RegisterSetCommands(r)
			if got := runEntryTransfer(t, s, nil, r, "SADD", "s", "m"); got != ":1\r\n" {
				t.Fatal("CONTROL", got)
			}
			version := s.WatchKey("s")
			defer s.UnwatchKey("s")
			args := []string{"s", "m"}
			if name == "SMOVE" {
				args = []string{"s", "d", "absent"}
			} else if name != "SADD" {
				args = []string{"d", "s"}
			}
			got := runEntryTransfer(t, s, nil, r, name, args...)
			t.Logf("EXPECTED: unchanged read/no-op source WATCH=%d | ACTUAL: reply=%q version=%d", version, got, s.GetVersion("s"))
			if s.GetVersion("s") != version {
				t.Error("unchanged/read-only source dirtied WATCH")
			}
			if got := runEntryTransfer(t, s, nil, r, "SADD", "s", "changed"); got != ":1\r\n" || s.GetVersion("s") == version {
				t.Fatal("CONTROL actual write failed to invalidate")
			}
		})
	}
}
