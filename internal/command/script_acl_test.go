package command

import (
	"strings"
	"sync"
	"testing"

	"github.com/cachestorm/cachestorm/internal/acl"
	"github.com/cachestorm/cachestorm/internal/store"
)

// A script used to be able to touch keys it never declared in numkeys: the
// ACL check on EVAL/EVALSHA covered only the keys named on the command line,
// while redis.call dispatched straight to the store. These tests pin the
// script-call guard that makes undeclared keys subject to the user's key
// patterns, through the real router.
func aclScriptSetup(t *testing.T) (*store.Store, *Router) {
	t.Helper()
	// scriptEngine is a package global lazily bound to the first ctx.Store it
	// sees; reset it so this test's script runs against THIS test's store.
	scriptEngine = nil
	s := store.NewStore()
	router := NewRouter()
	RegisterServerCommands(router)
	RegisterScriptCommands(router)
	return s, router
}

func runACLScript(t *testing.T, router *Router, s *store.Store, user *acl.User, parts ...string) string {
	t.Helper()
	ctx, buf := bufCtx(parts[0], bytesArgs(parts[1:]...), s)
	ctx.ACLUser = user
	ctx.Authenticated = true
	if err := router.Execute(ctx); err != nil {
		t.Fatalf("Execute(%v): %v", parts, err)
	}
	return buf.String()
}

func TestACLScriptUndeclaredKeyIsRefused(t *testing.T) {
	old := globalACL
	globalACL = acl.NewACL()
	defer func() { globalACL = old }()

	s, router := aclScriptSetup(t)
	s.Set("other:1", &store.StringValue{Data: []byte("secret")}, store.SetOptions{})

	if out := runACLScript(t, router, s, nil, "ACL", "SETUSER", "runner", ">pw", "on", "-@all", "+eval", "+get", "~user:*"); !strings.Contains(out, "+OK") {
		t.Fatalf("ACL SETUSER runner: %q", out)
	}
	runner, ok := globalACL.GetUser("runner")
	if !ok {
		t.Fatal("runner missing after SETUSER")
	}

	if out := runACLScript(t, router, s, runner, "EVAL", "return redis.call('get','other:1')", "0"); !strings.Contains(out, "NOPERM") {
		t.Fatalf("EVAL must not read an undeclared key outside ~user:*, got %q", out)
	}
	if _, exists := s.Get("other:1"); !exists {
		t.Fatal("the key must be untouched by the refused script")
	}

	// EVALSHA reaches the same engine by SHA and gets the same treatment.
	out := runACLScript(t, router, s, nil, "SCRIPT", "LOAD", "return redis.call('get','other:1')")
	parts := strings.Split(out, "\r\n")
	if len(parts) < 2 || !strings.HasPrefix(parts[0], "$") {
		t.Fatalf("unexpected SCRIPT LOAD reply %q", out)
	}
	if out := runACLScript(t, router, s, runner, "EVALSHA", parts[1], "0"); !strings.Contains(out, "NOPERM") {
		t.Fatalf("EVALSHA must not read an undeclared key outside ~user:*, got %q", out)
	}

	// Control: a key that is declared AND matches the pattern still works.
	s.Set("user:1", &store.StringValue{Data: []byte("mine")}, store.SetOptions{})
	if out := runACLScript(t, router, s, runner, "EVAL", "return redis.call('get','user:1')", "1", "user:1"); !strings.Contains(out, "mine") {
		t.Fatalf("the declared, allowed key should still be readable, got %q", out)
	}
	// Control: a connection without an ACL user keeps the default behaviour.
	if out := runACLScript(t, router, s, nil, "EVAL", "return redis.call('get','other:1')", "0"); !strings.Contains(out, "secret") {
		t.Fatalf("default user lost script access, got %q", out)
	}
}

// The guard is key-pattern shaped, not a blanket refusal: a command whose key
// extraction yields nothing (e.g. PING) stays runnable from a script, and a
// connection with no ACL user gets no guard at all.
func TestScriptGuardShape(t *testing.T) {
	user, err := acl.NewACL().CreateUser("runner")
	if err != nil {
		t.Fatalf("CreateUser: %v", err)
	}
	if err := acl.ParseACLRule("on -@all +eval +get ~user:*", user); err != nil {
		t.Fatalf("ParseACLRule: %v", err)
	}

	guard := scriptGuard(&Context{ACLUser: user})
	if guard == nil {
		t.Fatal("expected a guard for a connection with an ACL user")
	}
	if err := guard("get", []string{"user:1"}); err != nil {
		t.Fatalf("granted command on an allowed key refused: %v", err)
	}
	if err := guard("get", []string{"other:1"}); err == nil {
		t.Fatal("a key outside ~user:* must be refused")
	}
	if err := guard("set", []string{"user:1"}); err == nil {
		t.Fatal("a command the user is not granted must be refused")
	}
	if err := guard("ping", nil); err != nil {
		t.Fatalf("bypass command refused: %v", err)
	}
	if scriptGuard(&Context{}) != nil {
		t.Fatal("a connection with no ACL user must get no guard")
	}
}

// Scripts run as the invoking user: a command the user cannot run at the top
// level must not run from inside a script — including key-less destructive
// ones such as FLUSHDB, which the key-pattern check never sees. redis.call
// with a command like FLUSHALL that the engine does not implement stays a
// silent no-op either way; the permission check must not depend on that.
func TestACLScriptCommandPermissionIsChecked(t *testing.T) {
	old := globalACL
	globalACL = acl.NewACL()
	defer func() { globalACL = old }()

	s, router := aclScriptSetup(t)
	// The command-permission contract spans both script entry points, so the
	// function registry must be wired too. It is a process-wide singleton
	// bound to the first store it sees: reset it so no leftover library
	// collides and the function runs against THIS test's store.
	functionOnce = sync.Once{}
	functionRegistry = nil
	RegisterFunctionCommands(router)
	s.Set("user:1", &store.StringValue{Data: []byte("mine")}, store.SetOptions{})
	s.Set("user:2", &store.StringValue{Data: []byte("keep")}, store.SetOptions{})

	// +eval and +fcall only: no +get, no +flushdb. The key IS in pattern, so
	// only the command check can refuse the call.
	if out := runACLScript(t, router, s, nil, "ACL", "SETUSER", "runner", ">pw", "on", "-@all", "+eval", "+fcall", "~user:*"); !strings.Contains(out, "+OK") {
		t.Fatalf("ACL SETUSER runner: %q", out)
	}
	runner, ok := globalACL.GetUser("runner")
	if !ok {
		t.Fatal("runner missing after SETUSER")
	}

	if out := runACLScript(t, router, s, runner, "EVAL", "return redis.call('get','user:1')", "1", "user:1"); !strings.Contains(out, "NOPERM") {
		t.Fatalf("a script must not run GET without a +get grant, got %q", out)
	}

	// FCALL reaches the same guard through the function registry.
	if out := runACLScript(t, router, s, nil, "FUNCTION", "CREATE", "fclib", "redis = redis or {}\nredis.getit = function() return redis.call('get','user:1') end"); !strings.HasPrefix(out, "$") {
		t.Fatalf("FUNCTION CREATE failed: %q", out)
	}
	if out := runACLScript(t, router, s, runner, "FCALL", "fclib.getit", "0"); !strings.Contains(out, "NOPERM") {
		t.Fatalf("a function must not run GET without a +get grant, got %q", out)
	}

	out := runACLScript(t, router, s, runner, "EVAL", "redis.call('flushdb') return 1", "0")
	if !strings.Contains(out, "NOPERM") {
		t.Fatalf("a script must not run FLUSHDB without a +flushdb grant, got %q", out)
	}
	if _, exists := s.Get("user:2"); !exists {
		t.Fatal("the store was wiped by a refused script")
	}

	// Controls: a granted command still runs inside a script, and the
	// default user keeps the permissive behaviour.
	granted, err := globalACL.CreateUser("granted")
	if err != nil {
		t.Fatalf("CreateUser: %v", err)
	}
	if err := acl.ParseACLRule("on -@all +eval +get ~user:*", granted); err != nil {
		t.Fatalf("ParseACLRule: %v", err)
	}
	if out := runACLScript(t, router, s, granted, "EVAL", "return redis.call('get','user:1')", "1", "user:1"); !strings.Contains(out, "mine") {
		t.Fatalf("a granted command was refused inside a script, got %q", out)
	}
	if out := runACLScript(t, router, s, nil, "EVAL", "redis.call('flushdb') return 1", "0"); strings.Contains(out, "NOPERM") {
		t.Fatalf("the default user was restricted, got %q", out)
	}
}
