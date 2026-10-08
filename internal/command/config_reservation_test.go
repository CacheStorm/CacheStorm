package command

import (
	"strings"
	"testing"

	"github.com/cachestorm/cachestorm/internal/acl"
	"github.com/cachestorm/cachestorm/internal/store"
)

// The CONFIG surface is reserved for the default user exactly like ACL
// administration: a user granted +config must still be refused, because
// holding the command must not be enough to reach server security
// tunables. Additionally, CONFIG SET does not implement requirepass at all
// (it replies ERR Unknown option), so authentication cannot be defeated
// through CONFIG by any identity, and the script engine has no CONFIG case
// either (redis.call('config',...) is a no-op).

func requirePassUser(t *testing.T, router *Router, rules ...string) *acl.User {
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

func TestConfigSetIsReservedForACLUsers(t *testing.T) {
	s := store.NewStore()
	router := NewRouter()
	RegisterConfigCommands(router)
	router.SetRequirePass("s3cret")

	user := requirePassUser(t, router, "-@all", "+config", "+get", "~*")

	ctx, buf := bufCtx("CONFIG", bytesArgs("SET", "requirepass", ""), s)
	ctx.ACLUser = user
	ctx.Authenticated = true
	if err := router.Execute(ctx); err != nil {
		t.Fatalf("CONFIG SET should be refused inside the router, got error: %v", err)
	}
	if !strings.Contains(buf.String(), "NOPERM") {
		t.Fatalf("a +config user must be refused CONFIG SET requirepass (the reservation fires before command permissions), got %q", buf.String())
	}
	if router.RequirePass() != "s3cret" {
		t.Fatalf("authentication must survive a refused CONFIG SET, requirepass now %q", router.RequirePass())
	}
}

func TestConfigSetDefaultUserKeepsAdministration(t *testing.T) {
	s := store.NewStore()
	router := NewRouter()
	RegisterConfigCommands(router)
	router.SetRequirePass("s3cret")

	// Control: without an ACL identity the command reaches the handler.
	// The handler answers ERR Unknown option because requirepass is not an
	// implemented CONFIG SET parameter — either way the password stands.
	ctx, buf := bufCtx("CONFIG", bytesArgs("SET", "requirepass", ""), s)
	if err := router.Execute(ctx); err != nil {
		t.Fatalf("default user CONFIG SET failed: %v", err)
	}
	if strings.Contains(buf.String(), "NOPERM") {
		t.Fatalf("the default user must keep CONFIG access, got %q", buf.String())
	}
	if router.RequirePass() != "s3cret" {
		t.Fatalf("requirepass must not be mutable through CONFIG SET, now %q", router.RequirePass())
	}
}

func TestConfigScriptCallCannotDefeatAuthentication(t *testing.T) {
	s := store.NewStore()
	router := NewRouter()
	RegisterConfigCommands(router)
	RegisterScriptCommands(router)
	router.SetRequirePass("s3cret")

	savedEngine := scriptEngine
	scriptEngine = nil
	t.Cleanup(func() { scriptEngine = savedEngine })

	user := requirePassUser(t, router, "-@all", "+eval", "+config", "~*")

	ctx, _ := bufCtx("EVAL", bytesArgs("return redis.call('config','set','requirepass','')", "0"), s)
	ctx.ACLUser = user
	ctx.Authenticated = true
	if err := router.Execute(ctx); err != nil {
		t.Fatalf("EVAL failed: %v", err)
	}
	if router.RequirePass() != "s3cret" {
		t.Fatalf("a script must not defeat authentication, requirepass now %q", router.RequirePass())
	}
}
