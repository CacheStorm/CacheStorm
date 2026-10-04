package command

import (
	"bufio"
	"io"
	"testing"

	"github.com/cachestorm/cachestorm/internal/acl"
	"github.com/cachestorm/cachestorm/internal/resp"
	"github.com/cachestorm/cachestorm/internal/store"
)

func aclB(ss ...string) [][]byte {
	out := make([][]byte, len(ss))
	for i, s := range ss {
		out[i] = []byte(s)
	}
	return out
}

// aclCtx builds a Context whose replies are discarded, for assertions on
// enforcement decisions rather than on reply contents.
func aclCtx(t *testing.T, cmd string, args ...string) *Context {
	t.Helper()
	s := store.NewStore()
	return NewContext(cmd, aclB(args...), s, resp.NewWriter(bufio.NewWriter(io.Discard)))
}

// Key extraction is where a key-pattern ACL could silently leak, so pin the
// shapes that differ per command: single key, all-args, alternating MSET,
// NUMKEYS-counted, source+destination, STORE destinations, and admin commands
// whose arguments are NOT keys.
func TestAclKeysForCommandShapes(t *testing.T) {
	cases := []struct {
		cmd  string
		args []string
		want []string
	}{
		{"GET", []string{"k"}, []string{"k"}},
		{"SET", []string{"k", "v"}, []string{"k"}},
		{"LPUSH", []string{"k", "a", "b"}, []string{"k"}},
		{"HSET", []string{"k", "f", "v"}, []string{"k"}},
		{"ZADD", []string{"k", "1", "m"}, []string{"k"}},
		{"SETEX", []string{"k", "10", "v"}, []string{"k"}},

		// every argument is a key
		{"DEL", []string{"a", "b", "c"}, []string{"a", "b", "c"}},
		{"MGET", []string{"a", "b"}, []string{"a", "b"}},
		{"EXISTS", []string{"a", "b"}, []string{"a", "b"}},

		// MSET alternates key/value: only even indices are keys. Treating the
		// values as keys would refuse a legitimate MSET under ~user:*.
		{"MSET", []string{"a", "1", "b", "2"}, []string{"a", "b"}},
		{"MSETNX", []string{"a", "1"}, []string{"a"}},

		// NUMKEYS-counted: destination then count then keys
		{"ZUNIONSTORE", []string{"dst", "2", "a", "b"}, []string{"a", "b"}},
		{"EVAL", []string{"script", "2", "a", "b", "arg"}, []string{"a", "b"}},

		// source + destination
		{"RENAME", []string{"a", "b"}, []string{"a", "b"}},
		{"SMOVE", []string{"src", "dst", "m"}, []string{"src", "dst"}},
		{"BITOP", []string{"AND", "dst", "a", "b"}, []string{"a", "b"}},

		// key plus STORE destination
		{"SORT", []string{"k", "STORE", "out"}, []string{"k", "out"}},
		{"SORT", []string{"k", "ALPHA"}, []string{"k"}},

		// admin commands: arguments are not keys
		{"CONFIG", []string{"GET", "maxmemory"}, nil},
		{"ACL", []string{"LIST"}, nil},
		{"INFO", []string{"memory"}, nil},
		{"SCAN", []string{"0", "MATCH", "*"}, nil},
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

// A malformed NUMKEYS count must yield no keys rather than panicking or reading
// past the end of the argument slice.
func TestAclKeysForCommandMalformedNumkeys(t *testing.T) {
	for _, args := range [][]string{
		{"EVAL", "script", "notanumber", "a"},
		{"ZUNIONSTORE", "dst", "abc"},
		{"EVAL", "script", "99", "a"}, // count beyond the arguments
		{"EVAL", "script"},            // no count at all
	} {
		if keys := aclKeysForCommand(args[0], aclB(args[1:]...)); len(keys) != 0 {
			t.Errorf("%v: got keys %v, want none", args, keys)
		}
	}
}

func aclUnitUser(t *testing.T) *acl.User {
	t.Helper()
	a := acl.NewACL()
	u, err := a.CreateUser("unit_user")
	if err != nil {
		t.Fatal(err)
	}
	if err := acl.ParseACLRule(">pw -@all +get ~user:*", u); err != nil {
		t.Fatal(err)
	}
	u.Enable()
	return u
}

// enforceACL must refuse a command the user was not granted, and a key outside
// the pattern, while allowing the granted combination.
func TestEnforceAclRefusesByCommandAndKey(t *testing.T) {
	u := aclUnitUser(t)

	allowed := func(cmd string, args ...string) bool {
		ctx := aclCtx(t, cmd, args...)
		ctx.ACLUser = u
		return !enforceACL(ctx, cmd)
	}

	if !allowed("GET", "user:1") {
		t.Error("GET on an allowed key was refused, but should be permitted")
	}
	if allowed("SET", "user:1", "v") {
		t.Error("SET was permitted, but -@all +get does not grant it")
	}
	if allowed("GET", "other:1") {
		t.Error("GET on a key outside ~user:* was permitted")
	}
	if allowed("DEL", "user:1") {
		t.Error("DEL was permitted, but -@all +get does not grant it")
	}
}

// AUTH, PING and friends must stay reachable regardless of permissions, or a
// restricted user could not even authenticate or disconnect.
func TestEnforceAclAlwaysAllowsBypassCommands(t *testing.T) {
	u := aclUnitUser(t)

	for _, cmd := range []string{"AUTH", "PING", "QUIT", "RESET", "HELLO", "COMMAND"} {
		ctx := aclCtx(t, cmd, "x")
		ctx.ACLUser = u
		if enforceACL(ctx, cmd) {
			t.Errorf("%s was refused, but it is always allowed", cmd)
		}
	}
}

// A connection with no ACLUser keeps the permissive default behaviour, so
// existing deployments are unaffected.
func TestEnforceAclNoUserIsPermissive(t *testing.T) {
	for _, cmd := range []string{"SET", "DEL", "FLUSHALL", "CONFIG"} {
		if enforceACL(aclCtx(t, cmd, "k", "v"), cmd) {
			t.Errorf("%s was refused for a context with no ACLUser, but the default user is unrestricted", cmd)
		}
	}
}

// A user with no key restriction may touch any key — the empty AllowedKeys
// list must not deny everything.
func TestEnforceAclEmptyKeyListAllowsAnyKey(t *testing.T) {
	a := acl.NewACL()
	u, err := a.CreateUser("nokeys_user")
	if err != nil {
		t.Fatal(err)
	}
	if err := acl.ParseACLRule(">pw +get", u); err != nil {
		t.Fatal(err)
	}
	u.Enable()

	ctx := aclCtx(t, "GET", "anything:at:all")
	ctx.ACLUser = u
	if enforceACL(ctx, "GET") {
		t.Error("a user with no ~pattern was denied a key, but an empty key list means unrestricted")
	}
}

// Channel extraction differs per command: PUBLISH names one channel with the
// payload after it, while the subscribe forms name every argument.
func TestAclChannelsForCommand(t *testing.T) {
	cases := []struct {
		cmd  string
		args []string
		want []string
	}{
		// Only the channel — the payload is NOT a channel.
		{"PUBLISH", []string{"news.sports", "hello world"}, []string{"news.sports"}},
		{"SPUBLISH", []string{"news.sports", "payload"}, []string{"news.sports"}},
		{"PUBLISH", []string{"news.sports"}, []string{"news.sports"}},
		{"PUBLISH", nil, nil},

		// Every argument is a channel.
		{"SUBSCRIBE", []string{"a"}, []string{"a"}},
		{"SUBSCRIBE", []string{"a", "b", "c"}, []string{"a", "b", "c"}},
		{"PSUBSCRIBE", []string{"news.*"}, []string{"news.*"}},
		{"PSUBSCRIBE", []string{"news.*", "sports.*"}, []string{"news.*", "sports.*"}},
		{"SSUBSCRIBE", []string{"a"}, []string{"a"}},

		// Unsubscribe forms and non-channel commands name none here.
		{"UNSUBSCRIBE", []string{"a"}, nil},
		{"PUNSUBSCRIBE", []string{"news.*"}, nil},
		{"PUBSUB", []string{"CHANNELS"}, nil},
		{"GET", []string{"k"}, nil},
	}

	for _, tc := range cases {
		got := aclChannelsForCommand(tc.cmd, aclB(tc.args...))
		if len(got) != len(tc.want) {
			t.Errorf("%s %v: got channels %v, want %v", tc.cmd, tc.args, got, tc.want)
			continue
		}
		for i := range tc.want {
			if got[i] != tc.want[i] {
				t.Errorf("%s %v: got channels %v, want %v", tc.cmd, tc.args, got, tc.want)
				break
			}
		}
	}
}

// A &channel restriction must be enforced per channel, and a user with no &rule
// must remain unrestricted.
func TestEnforceAclChannelRules(t *testing.T) {
	newChanUser := func(t *testing.T, rule string) *acl.User {
		t.Helper()
		a := acl.NewACL()
		u, err := a.CreateUser("chan_user")
		if err != nil {
			t.Fatal(err)
		}
		// +psubscribe is required too, or PSUBSCRIBE is refused by the COMMAND
		// check and never reaches the channel check this test exists to exercise.
		if err := acl.ParseACLRule(">pw -@all +publish +subscribe +psubscribe "+rule, u); err != nil {
			t.Fatal(err)
		}
		u.Enable()
		return u
	}

	allowed := func(u *acl.User, cmd string, args ...string) bool {
		ctx := aclCtx(t, cmd, args...)
		ctx.ACLUser = u
		return !enforceACL(ctx, cmd)
	}

	restricted := newChanUser(t, "&news.*")

	if !allowed(restricted, "PUBLISH", "news.sports", "payload") {
		t.Error("PUBLISH to news.sports was refused, but &news.* allows it")
	}
	if allowed(restricted, "PUBLISH", "secret", "payload") {
		t.Error("PUBLISH to secret was permitted, but &news.* does not allow it")
	}
	if !allowed(restricted, "SUBSCRIBE", "news.tech") {
		t.Error("SUBSCRIBE news.tech was refused, but &news.* allows it")
	}
	if allowed(restricted, "SUBSCRIBE", "private") {
		t.Error("SUBSCRIBE private was permitted, but &news.* does not allow it")
	}
	if !allowed(restricted, "PSUBSCRIBE", "news.*") {
		t.Error("PSUBSCRIBE news.* was refused, but &news.* allows it")
	}
	if allowed(restricted, "PSUBSCRIBE", "private.*") {
		t.Error("PSUBSCRIBE private.* was permitted, but &news.* does not allow it")
	}
	// A multi-channel call must be refused when ANY one channel is disallowed.
	if allowed(restricted, "SUBSCRIBE", "news.tech", "private") {
		t.Error("SUBSCRIBE news.tech private was permitted, but private is outside &news.*")
	}

	// No &rule means every channel is allowed.
	noRule := newChanUser(t, "")
	if !allowed(noRule, "PUBLISH", "any.channel", "payload") {
		t.Error("PUBLISH was refused for a user with no &rule, but an empty channel list means unrestricted")
	}
	if !allowed(noRule, "SUBSCRIBE", "anything") {
		t.Error("SUBSCRIBE was refused for a user with no &rule, but an empty channel list means unrestricted")
	}
}

// ACL administration must be refused for any connection carrying an ACL user,
// even one explicitly granted "+acl" or "+@all" — otherwise holding the ACL
// command is a full privilege escalation via `ACL SETUSER self +@all ~*`.
func TestEnforceAclRefusesAdminCommands(t *testing.T) {
	ruleUser := func(t *testing.T, rules string) *acl.User {
		t.Helper()
		a := acl.NewACL()
		u, err := a.CreateUser("admin_user")
		if err != nil {
			t.Fatal(err)
		}
		if err := acl.ParseACLRule(">pw "+rules, u); err != nil {
			t.Fatal(err)
		}
		u.Enable()
		return u
	}

	refused := func(u *acl.User, cmd string, args ...string) bool {
		ctx := aclCtx(t, cmd, args...)
		ctx.ACLUser = u
		return enforceACL(ctx, cmd)
	}

	// Granted the ACL command explicitly: still refused.
	explicit := ruleUser(t, "-@all +get +acl ~user:*")
	for _, sub := range [][]string{
		{"ACL", "SETUSER", "x", "+@all", "~*"},
		{"ACL", "DELUSER", "somebody"},
		{"ACL", "GETUSER", "admin_user"},
		{"ACL", "LIST"},
		{"ACL", "SAVE"},
		{"ACL", "LOAD"},
		{"ACL", "WHOAMI"},
		{"ACL", "DRYRUN", "admin_user", "set", "k", "v"},
	} {
		if !refused(explicit, sub[0], sub[1:]...) {
			t.Errorf("%v was permitted for a user granted +acl, want NOPERM — a privilege escalation path", sub)
		}
	}

	// Granted EVERYTHING via +@all, which includes ACL: still refused. This is
	// the case the command-permission check alone cannot catch.
	full := ruleUser(t, "+@all ~user:*")
	if !refused(full, "ACL", "SETUSER", "admin_user", "+@all", "~*") {
		t.Error("ACL SETUSER was permitted for a user granted +@all, want NOPERM — +@all must not imply the ability to administer ACL")
	}
	// ...but the same user keeps its ordinary command access.
	if refused(full, "GET", "user:1") {
		t.Error("GET was refused for a +@all user, but the admin gate must not restrict ordinary commands")
	}

	// A context with no ACL user is the default user and keeps ACL administration.
	if enforceACL(aclCtx(t, "ACL", "SETUSER", "u", "+@all", "~*"), "ACL") {
		t.Error("ACL was refused for a context with no ACLUser, but the default user must retain ACL administration")
	}
	if enforceACL(aclCtx(t, "CONFIG", "SET", "requirepass", ""), "CONFIG") {
		t.Error("CONFIG was refused for a context with no ACLUser, but the default user must retain CONFIG administration")
	}

	// CONFIG is admin-gated on the same terms as ACL, including for a user
	// explicitly granted +config or +@all.
	configUser := ruleUser(t, "-@all +get +config ~user:*")
	for _, sub := range [][]string{
		{"CONFIG", "SET", "requirepass", ""},
		{"CONFIG", "GET", "maxmemory"},
		{"CONFIG", "SET", "maxmemory", "1000"},
		{"CONFIG", "REWRITE"},
	} {
		if !refused(configUser, sub[0], sub[1:]...) {
			t.Errorf("%v was permitted for a user granted +config, want NOPERM", sub)
		}
	}
	if !refused(full, "CONFIG", "SET", "maxmemory", "1000") {
		t.Error("CONFIG SET was permitted for a user granted +@all, want NOPERM — +@all must not imply the ability to reconfigure the server")
	}
	// ...but a +@all user keeps its ordinary command access.
	if refused(full, "GET", "user:1") {
		t.Error("GET was refused for a +@all user, but the admin gate must not restrict ordinary commands")
	}
}
