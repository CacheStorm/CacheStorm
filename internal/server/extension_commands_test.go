package server

import (
	"strings"
	"testing"

	"github.com/cachestorm/cachestorm/internal/config"
)

// extension-commands decides which of CacheStorm's own commands are served;
// Redis and Redis Stack commands are served under every setting, and a left
// out command answers as Redis answers a command it does not have.
func TestExtensionCommandsSetting(t *testing.T) {
	for _, tc := range []struct {
		setting  string
		utility  bool // STR.REVERSE, a stateless helper
		stateful bool // QUEUE.PUSH
	}{
		{"", false, true},
		{"stateful", false, true},
		{"all", true, true},
		{"none", false, false},
	} {
		t.Run("setting="+tc.setting, func(t *testing.T) {
			cfg := config.Default()
			cfg.HTTP.Enabled = false
			cfg.Plugins.Metrics.Enabled = false
			cfg.Server.ExtensionCommands = tc.setting
			s := startToggleServer(t, cfg)
			defer stopReplServer(s)
			c := dialRepl(t, s)

			for _, args := range [][]string{{"SET", "k", "v"}, {"JSON.SET", "j", "$", "1"}, {"BF.ADD", "b", "x"}} {
				if got := c.do(args...); got.Type == '-' {
					t.Fatalf("%v: %v", args, got)
				}
			}
			check := func(served bool, args ...string) {
				t.Helper()
				got := c.do(args...)
				if served {
					if got.Type == '-' {
						t.Fatalf("%v is not served: %v", args, got)
					}
					return
				}
				want := "ERR unknown command '" + args[0] + "', with args beginning with: "
				for _, a := range args[1:] {
					want += "'" + a + "' "
				}
				if got.Type != '-' || got.Err != want {
					t.Fatalf("%v: %q, want %q", args, got.Err, want)
				}
			}
			check(tc.utility, "STR.REVERSE", "abc")
			check(tc.stateful, "QUEUE.PUSH", "q", "x")

			want := tc.setting
			if want == "" {
				want = "stateful"
			}
			if got := c.do("CONFIG", "GET", "extension-commands"); len(got.Array) != 2 || string(got.Array[1].Bulk) != want {
				t.Fatalf("CONFIG GET extension-commands: %v", got)
			}
			if got := c.do("CONFIG", "SET", "extension-commands", "all"); got.Type != '-' || !strings.Contains(got.Err, "immutable") {
				t.Fatalf("CONFIG SET extension-commands: %v", got)
			}
			if got := c.do("COMMAND", "INFO", "str.reverse"); len(got.Array) != 1 || (got.Array[0].IsNull != !tc.utility) {
				t.Fatalf("COMMAND INFO str.reverse: %v", got)
			}
		})
	}
}
