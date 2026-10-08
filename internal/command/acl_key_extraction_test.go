package command

import (
	"strings"
	"testing"

	"github.com/cachestorm/cachestorm/internal/acl"
	"github.com/cachestorm/cachestorm/internal/store"
)

// A command the key-extraction tables do not classify used to fall through
// with NO keys, so a user confined to ~user:* could read and write any other
// key through it — XADD was the proof. The fail-closed default in
// aclKeysForCommand closes that hole; this pins it through the real router.
func TestACLUncoveredCommandIsKeyChecked(t *testing.T) {
	old := globalACL
	globalACL = acl.NewACL()
	defer func() { globalACL = old }()

	s := store.NewStore()
	router := NewRouter()
	RegisterServerCommands(router)
	RegisterStreamCommands(router)
	RegisterStringCommands(router)

	run := func(user *acl.User, parts ...string) string {
		t.Helper()
		ctx, buf := bufCtx(parts[0], bytesArgs(parts[1:]...), s)
		ctx.ACLUser = user
		ctx.Authenticated = true
		if err := router.Execute(ctx); err != nil {
			t.Fatalf("Execute(%v): %v", parts, err)
		}
		return buf.String()
	}

	if out := run(nil, "ACL", "SETUSER", "streamer", ">pw", "on", "-@all", "+xadd", "+get", "~user:*"); !strings.Contains(out, "+OK") {
		t.Fatalf("ACL SETUSER streamer: %q", out)
	}
	streamer, ok := globalACL.GetUser("streamer")
	if !ok {
		t.Fatal("streamer missing after SETUSER")
	}
	if !streamer.CanExecuteCommand("XADD") {
		t.Fatal("setup: XADD should be allowed by command permission")
	}

	if out := run(streamer, "XADD", "other:1", "*", "f", "v"); !strings.Contains(out, "NOPERM") {
		t.Fatalf("XADD on a key outside ~user:* must be refused, got %q", out)
	}
	if _, exists := s.Get("other:1"); exists {
		t.Fatal("key other:1 was created outside the key sandbox")
	}

	// Control: the same command on a key inside the sandbox still works.
	if out := run(streamer, "XADD", "user:1", "*", "f", "v"); strings.Contains(out, "NOPERM") {
		t.Fatalf("XADD inside ~user:* was refused: %q", out)
	}
	if _, exists := s.Get("user:1"); !exists {
		t.Fatal("XADD inside ~user:* did not create the key")
	}
	// Control: a classified command is refused on the same path.
	if out := run(streamer, "GET", "other:1"); !strings.Contains(out, "NOPERM") {
		t.Fatalf("GET other:1 must be NOPERM, got %q", out)
	}
}

// The fail-closed default itself: an unclassified command is checked against
// its first argument, so the key pattern applies whatever the command is. A
// command that truly takes no keys belongs in aclNoKeyCommands.
func TestAclKeysForCommandUncoveredFallsClosed(t *testing.T) {
	cases := []struct {
		cmd  string
		args []string
		want []string
	}{
		{"XADD", []string{"k", "*", "f", "v"}, []string{"k"}},
		{"XDEL", []string{"k", "1-1"}, []string{"k"}},
		{"XTRIM", []string{"k", "MAXLEN", "10"}, []string{"k"}},
		{"XACK", []string{"k", "g", "1-1"}, []string{"k"}},
		{"XPENDING", []string{"k", "g"}, []string{"k"}},
		{"XSETID", []string{"k", "1-1"}, []string{"k"}},
		{"BF.ADD", []string{"k", "item"}, []string{"k"}},
		{"JSON.SET", []string{"k", "$", "1"}, []string{"k"}},
		{"ARRAY.PUSH", []string{"k", "a", "b"}, []string{"k"}},
		// no arguments: nothing to check
		{"ASKING", nil, nil},
	}
	for _, tc := range cases {
		got := aclKeysForCommand(tc.cmd, aclB(tc.args...))
		if len(got) != len(tc.want) {
			t.Errorf("%s %v: got keys %v, want %v", tc.cmd, tc.args, got, tc.want)
			continue
		}
		for i := range tc.want {
			if got[i] != tc.want[i] {
				t.Errorf("%s %v: got keys %v, want %v", tc.cmd, tc.args, got, tc.want)
				break
			}
		}
	}
}

// Stream commands do not put the key at args[0], so they need explicit
// extraction: XREAD/XREADGROUP name their keys after the STREAMS keyword and
// XINFO's key follows its subcommand. Getting these wrong would either miss a
// key or refuse a legitimate read because an ID was treated as one.
func TestAclKeysForCommandStreamShapes(t *testing.T) {
	cases := []struct {
		cmd  string
		args []string
		want []string
	}{
		{"XREAD", []string{"STREAMS", "k", "$"}, []string{"k"}},
		{"XREAD", []string{"COUNT", "2", "STREAMS", "k1", "k2", "0", "0"}, []string{"k1", "k2"}},
		{"XREADGROUP", []string{"GROUP", "g", "c", "STREAMS", "k1", "k2", ">", ">"}, []string{"k1", "k2"}},
		// malformed: an odd remainder cannot be split into keys and IDs, so
		// every remaining argument is checked rather than silently dropping
		// a key; the handler rejects the command on its own
		{"XREAD", []string{"STREAMS", "k1", "k2", "0"}, []string{"k1", "k2", "0"}},
		// malformed: no STREAMS keyword — the handler rejects the command,
		// so there is no key to check
		{"XREAD", []string{"COUNT", "2"}, nil},
		{"XINFO", []string{"STREAM", "k"}, []string{"k"}},
		{"XINFO", []string{"GROUPS", "k"}, []string{"k"}},
		{"XINFO", []string{"STREAM"}, nil},
		// XGROUP takes its key first, like every other data command
		{"XGROUP", []string{"CREATE", "k", "g", "$"}, []string{"k"}},
	}
	for _, tc := range cases {
		got := aclKeysForCommand(tc.cmd, aclB(tc.args...))
		if len(got) != len(tc.want) {
			t.Errorf("%s %v: got keys %v, want %v", tc.cmd, tc.args, got, tc.want)
			continue
		}
		for i := range tc.want {
			if got[i] != tc.want[i] {
				t.Errorf("%s %v: got keys %v, want %v", tc.cmd, tc.args, got, tc.want)
				break
			}
		}
	}
}
