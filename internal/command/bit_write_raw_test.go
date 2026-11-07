package command

import (
	"testing"

	"github.com/cachestorm/cachestorm/internal/store"
)

// A bit write leaves the string raw even when it changes no bit, as Redis
// unshares the string before looking at it; a read-only BITFIELD does not.
func TestBitWritesThatChangeNothingLeaveTheStringRaw(t *testing.T) {
	for _, c := range []struct {
		value string
		cmd   []string
		want  string
	}{
		{"abc", []string{"SETBIT", "k", "15", "0"}, "raw"},
		{"12", []string{"SETBIT", "k", "2", "1"}, "raw"},
		{"abc", []string{"BITFIELD", "k", "SET", "u8", "0", "97"}, "raw"},
		{"abc", []string{"BITFIELD", "k", "OVERFLOW", "FAIL", "INCRBY", "u2", "0", "100"}, "raw"},
		{"abc", []string{"BITFIELD", "k", "GET", "u8", "0"}, "embstr"},
		{"12", []string{"GETBIT", "k", "2"}, "int"},
	} {
		s := store.NewStore()
		router := NewRouter()
		RegisterStringCommands(router)
		RegisterBitmapCommands(router)
		RegisterDebugCommands(router)
		runCmd(t, s, router, "SET", "k", c.value)
		version := s.GetVersion("k")
		runCmd(t, s, router, c.cmd[0], c.cmd[1:]...)
		if got := runCmd(t, s, router, "OBJECT", "ENCODING", "k"); got != "$"+string(rune('0'+len(c.want)))+"\r\n"+c.want+"\r\n" {
			t.Errorf("%q after %v: encoding %q, want %s", c.value, c.cmd, got, c.want)
		}
		if s.GetVersion("k") != version {
			t.Errorf("%q after %v: key counted as modified", c.value, c.cmd)
		}
	}
}
