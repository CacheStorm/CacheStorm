package command

import (
	"strings"
	"sync"
	"testing"

	"github.com/cachestorm/cachestorm/internal/acl"
	"github.com/cachestorm/cachestorm/internal/store"
)

// FUNCTION/FCALL scripts used to escape key-pattern enforcement twice over:
// CallFunction built its engine with CreateState, which installs no ACL guard,
// and FCALL had no declared-key extraction, so the top-level check inspected
// the function name as if it were a key. These tests pin both halves of the
// contract through the real router.
const fcallACLLibrary = `redis = redis or {}
redis.readit = function() return redis.call('get', 'other:1') end
redis.ok = function() return redis.call('get', redis.KEYS[1]) end`

func fcallACLSetup(t *testing.T) (*store.Store, *Router) {
	t.Helper()
	// The function registry binds to the first store it sees via sync.Once;
	// reset it so this test's library runs against THIS test's store.
	functionOnce = sync.Once{}
	functionRegistry = nil
	s := store.NewStore()
	router := NewRouter()
	RegisterServerCommands(router)
	RegisterFunctionCommands(router)
	return s, router
}

func runFCALLAs(t *testing.T, router *Router, s *store.Store, user *acl.User, parts ...string) string {
	t.Helper()
	ctx, buf := bufCtx(parts[0], bytesArgs(parts[1:]...), s)
	ctx.ACLUser = user
	ctx.Authenticated = true
	if err := router.Execute(ctx); err != nil {
		t.Fatalf("Execute(%v): %v", parts, err)
	}
	return buf.String()
}

func TestFCALLFunctionUndeclaredKeyIsRefused(t *testing.T) {
	old := globalACL
	globalACL = acl.NewACL()
	defer func() { globalACL = old }()

	s, router := fcallACLSetup(t)
	s.Set("other:1", &store.StringValue{Data: []byte("secret")}, store.SetOptions{})
	s.Set("user:1", &store.StringValue{Data: []byte("mine")}, store.SetOptions{})

	if out := runFCALLAs(t, router, s, nil, "ACL", "SETUSER", "runner", ">pw", "on", "-@all", "+fcall", "+function", "+get", "~user:*", "~fclib.*"); !strings.Contains(out, "+OK") {
		t.Fatalf("ACL SETUSER runner: %q", out)
	}
	runner, ok := globalACL.GetUser("runner")
	if !ok {
		t.Fatal("runner missing after SETUSER")
	}
	if out := runFCALLAs(t, router, s, nil, "FUNCTION", "CREATE", "fclib", fcallACLLibrary); !strings.HasPrefix(out, "$") {
		t.Fatalf("FUNCTION CREATE failed: %q", out)
	}

	if out := runFCALLAs(t, router, s, runner, "FCALL", "fclib.readit", "0"); !strings.Contains(out, "NOPERM") {
		t.Fatalf("FCALL must not run a function that reads an undeclared key outside ~user:*, got %q", out)
	}
	if _, exists := s.Get("other:1"); !exists {
		t.Fatal("the key must be untouched by the refused function")
	}
	// FCALL_RO delegates to the same handler and gets the same treatment.
	if out := runFCALLAs(t, router, s, runner, "FCALL_RO", "fclib.readit", "0"); !strings.Contains(out, "NOPERM") {
		t.Fatalf("FCALL_RO must not read an undeclared key outside ~user:*, got %q", out)
	}

	// Control: the function that only touches its declared in-pattern key
	// still works, and the default user keeps unrestricted access.
	if out := runFCALLAs(t, router, s, runner, "FCALL", "fclib.ok", "1", "user:1"); !strings.Contains(out, "mine") {
		t.Fatalf("the declared, in-pattern function should still work, got %q", out)
	}
	if out := runFCALLAs(t, router, s, nil, "FCALL", "fclib.readit", "0"); !strings.Contains(out, "secret") {
		t.Fatalf("default user lost function access, got %q", out)
	}
}

// The declared-keys contract: the function NAME is not a key, so a user whose
// pattern covers only the data keys can call a function using exactly those
// declared keys — and the malformed forms yield no keys rather than panicking.
func TestFCALLDeclaredKeysAreTheTopLevelContract(t *testing.T) {
	old := globalACL
	globalACL = acl.NewACL()
	defer func() { globalACL = old }()

	cases := []struct {
		cmd  string
		args []string
		want []string
	}{
		{"FCALL", []string{"fclib.ok", "1", "user:1"}, []string{"user:1"}},
		{"FCALL", []string{"fclib.ok", "2", "a", "b", "x"}, []string{"a", "b"}},
		{"FCALL", []string{"fclib.ok", "0"}, nil},
		{"FCALL_RO", []string{"fclib.ok", "1", "user:1"}, []string{"user:1"}},
		// malformed: no count, a non-numeric count, or a count beyond the
		// arguments must not fabricate keys
		{"FCALL", []string{"fclib.ok"}, nil},
		{"FCALL", []string{"fclib.ok", "notanum", "user:1"}, nil},
		{"FCALL", []string{"fclib.ok", "3", "a"}, nil},
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

	// End to end: with only a data-key pattern, a declared in-pattern call
	// is accepted and an out-of-pattern declared key is refused.
	s, router := fcallACLSetup(t)
	s.Set("user:1", &store.StringValue{Data: []byte("mine")}, store.SetOptions{})
	s.Set("other:1", &store.StringValue{Data: []byte("secret")}, store.SetOptions{})

	if out := runFCALLAs(t, router, s, nil, "ACL", "SETUSER", "runner", ">pw", "on", "-@all", "+fcall", "+function", "+get", "~user:*"); !strings.Contains(out, "+OK") {
		t.Fatalf("ACL SETUSER runner: %q", out)
	}
	runner, _ := globalACL.GetUser("runner")
	if out := runFCALLAs(t, router, s, nil, "FUNCTION", "CREATE", "fclib", fcallACLLibrary); !strings.HasPrefix(out, "$") {
		t.Fatalf("FUNCTION CREATE failed: %q", out)
	}

	if out := runFCALLAs(t, router, s, runner, "FCALL", "fclib.ok", "1", "user:1"); !strings.Contains(out, "mine") {
		t.Fatalf("a declared, in-pattern FCALL must be accepted, got %q", out)
	}
	if out := runFCALLAs(t, router, s, runner, "FCALL", "fclib.ok", "1", "other:1"); !strings.Contains(out, "NOPERM") {
		t.Fatalf("a declared, out-of-pattern key must be refused at the top level, got %q", out)
	}
}
