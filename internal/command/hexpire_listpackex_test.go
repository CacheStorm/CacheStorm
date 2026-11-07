package command

import (
	"testing"

	"github.com/cachestorm/cachestorm/internal/store"
)

// HEXPIRE moves a listpack hash to listpackex even when no field gets an
// expiry, as Redis converts before it looks at the fields; HPERSIST and
// HTTL leave it alone.
func TestHExpireConvertsToListpackEx(t *testing.T) {
	for _, c := range []struct {
		cmd  []string
		want string
	}{
		{[]string{"HEXPIRE", "k", "100", "FIELDS", "1", "missing"}, "listpackex"},
		{[]string{"HEXPIRE", "k", "100", "XX", "FIELDS", "1", "a"}, "listpackex"},
		{[]string{"HPEXPIRE", "k", "0", "FIELDS", "1", "a"}, "listpackex"},
		{[]string{"HPERSIST", "k", "FIELDS", "1", "a"}, "listpack"},
		{[]string{"HTTL", "k", "FIELDS", "1", "a"}, "listpack"},
	} {
		s := store.NewStore()
		router := NewRouter()
		RegisterHashCommands(router)
		RegisterHashTTLCommands(router)
		RegisterDebugCommands(router)
		runCmd(t, s, router, "HSET", "k", "a", "1", "b", "2")
		runCmd(t, s, router, c.cmd[0], c.cmd[1:]...)
		want := "$" + string(rune('0'+len(c.want))) + "\r\n" + c.want + "\r\n"
		if len(c.want) >= 10 {
			want = "$10\r\n" + c.want + "\r\n"
		}
		if got := runCmd(t, s, router, "OBJECT", "ENCODING", "k"); got != want {
			t.Errorf("%v: encoding %q, want %s", c.cmd, got, c.want)
		}
	}
}
