package command

import (
	"strings"
	"testing"

	"github.com/cachestorm/cachestorm/internal/acl"
	"github.com/cachestorm/cachestorm/internal/store"
)

// Round r37 proof: executeCommand's switch ends in `default: return
// lua.LNil`, so any command the script engine has not implemented —
// FLUSHALL, CONFIG, ACL, or a plain typo — returns nil WITHOUT error from
// inside a script. A script author gets no signal that the command never
// ran, unlike real Redis where an unimplemented/unknown command from a
// script is a script error.

func scriptNoopEval(t *testing.T, s *store.Store, user *acl.User, script string) string {
	t.Helper()
	router := NewRouter()
	RegisterStringCommands(router)
	RegisterScriptCommands(router)

	savedEngine := scriptEngine
	scriptEngine = nil
	t.Cleanup(func() { scriptEngine = savedEngine })

	ctx, buf := bufCtx("EVAL", bytesArgs(script, "0"), s)
	if user != nil {
		ctx.ACLUser = user
		ctx.Authenticated = true
	}
	if err := router.Execute(ctx); err != nil {
		t.Fatalf("EVAL failed: %v", err)
	}
	return buf.String()
}

func scriptNoopUser(t *testing.T, rules ...string) *acl.User {
	t.Helper()
	saved := globalACL
	globalACL = acl.NewACL()
	t.Cleanup(func() { globalACL = saved })

	if _, err := globalACL.CreateUser("web"); err != nil {
		t.Fatal(err)
	}
	user, ok := globalACL.GetUser("web")
	if !ok {
		t.Fatal("created user missing")
	}
	for _, rule := range rules {
		if err := acl.ParseACLRule(rule, user); err != nil {
			t.Fatal(err)
		}
	}
	return user
}

func TestProofUnimplementedCommandFromScriptIsAnError(t *testing.T) {
	s := store.NewStore()
	for _, script := range []string{
		"return redis.call('flushall')",
		"return redis.call('config','get','maxmemory')",
		"return redis.call('nosuchcommand')",
	} {
		reply := scriptNoopEval(t, s, nil, script)
		if !strings.Contains(reply, "unknown command") {
			t.Fatalf("FAIL: script command must fail loudly like Redis, got silent reply %q for %q", reply, script)
		}
	}
}

func TestProofAdminCommandFromScriptIsRefused(t *testing.T) {
	s := store.NewStore()
	user := scriptNoopUser(t, "-@all", "+eval", "+acl", "~*")

	reply := scriptNoopEval(t, s, user, "return redis.call('acl','users')")
	if !strings.Contains(reply, "NOPERM") {
		t.Fatalf("FAIL: the admin-command reservation must refuse ACL from a script even for a +acl user, got %q", reply)
	}
}

func TestProofControlGrantedCommandStillWorks(t *testing.T) {
	s := store.NewStore()
	if err := s.Set("user:1", &store.StringValue{Data: []byte("mine")}, store.SetOptions{}); err != nil {
		t.Fatal(err)
	}
	user := scriptNoopUser(t, "-@all", "+eval", "+get", "~*")

	reply := scriptNoopEval(t, s, user, "return redis.call('get','user:1')")
	if !strings.Contains(reply, "mine") {
		t.Fatalf("control failed: a granted in-pattern command must still work from a script, got %q", reply)
	}
}
