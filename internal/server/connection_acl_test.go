package server

import (
	"bufio"
	"fmt"
	"net"
	"testing"
	"time"

	"github.com/cachestorm/cachestorm/internal/command"
	"github.com/cachestorm/cachestorm/internal/resp"
	"github.com/cachestorm/cachestorm/internal/store"
)

// aclSession drives ONE real Connection over an in-memory pipe.
//
// A reader goroutine drains replies continuously: net.Pipe is unbuffered, so
// if a reply were still unread when the next command is written, the server's
// write and the client's write would block each other forever.
//
// Exactly one Connection is started, because a second one in the same test
// binary races on store.GlobalMetrics inside Metrics.RecordCommand — that is a
// separate pre-existing defect, not this test's concern.
type aclSession struct {
	t    *testing.T
	conn net.Conn
	reps chan *resp.Value
}

func newACLSession(t *testing.T) *aclSession {
	t.Helper()

	s := store.NewStore()
	router := command.NewRouter()
	command.RegisterStringCommands(router)
	command.RegisterKeyCommands(router)
	command.RegisterServerCommands(router)
	command.RegisterListCommands(router)
	command.RegisterHashCommands(router)
	command.RegisterSetCommands(router)
	command.RegisterSortedSetCommands(router)
	command.RegisterScriptCommands(router)
	command.RegisterPubSubCommands(router)
	command.RegisterConfigCommands(router)

	clientSide, serverSide := net.Pipe()
	conn := NewConnection(1, serverSide, s, router, nil)
	go conn.Handle()

	sess := &aclSession{t: t, conn: clientSide, reps: make(chan *resp.Value, 16)}
	go func() {
		r := bufio.NewReader(clientSide)
		for {
			v, err := resp.NewReader(r).ReadValue()
			if err != nil {
				close(sess.reps)
				return
			}
			sess.reps <- v
		}
	}()

	t.Cleanup(func() { _ = clientSide.Close() })
	return sess
}

// do sends a command and returns its reply.
func (a *aclSession) do(args ...string) *resp.Value {
	a.t.Helper()
	var out []byte
	out = append(out, fmt.Sprintf("*%d\r\n", len(args))...)
	for _, s := range args {
		out = append(out, fmt.Sprintf("$%d\r\n%s\r\n", len(s), s)...)
	}
	if _, err := a.conn.Write(out); err != nil {
		a.t.Fatalf("writing %v: %v", args, err)
	}
	select {
	case v, ok := <-a.reps:
		if !ok {
			a.t.Fatalf("connection closed while waiting for a reply to %v", args)
		}
		return v
	case <-time.After(5 * time.Second):
		a.t.Fatalf("timed out waiting for a reply to %v", args)
		return nil
	}
}

func aclIsOK(v *resp.Value) bool { return v.Type == resp.TypeSimpleString && v.Str == "OK" }

func aclIsNoperm(v *resp.Value) bool {
	return v.Type == resp.TypeError && v.Err == "NOPERM No permission"
}

// The headline requirement: a user declared as
//
//	ACL SETUSER <u> >pw -@all +get ~user:*
//
// must be refused on a DISALLOWED COMMAND and on a DISALLOWED KEY, on the real
// connection — not merely reported by DRYRUN.
func TestACLUserIsEnforcedOnRealConnection(t *testing.T) {
	const user = "acl_enforced_user"
	a := newACLSession(t)

	if v := a.do("SET", "user:1", "secret"); !aclIsOK(v) {
		t.Fatalf("setup: SET user:1 returned type %v %q, want +OK", v.Type, v.Str)
	}
	if v := a.do("SET", "other:1", "topsecret"); !aclIsOK(v) {
		t.Fatalf("setup: SET other:1 returned type %v %q, want +OK", v.Type, v.Str)
	}

	if v := a.do("ACL", "SETUSER", user, ">pw", "-@all", "+get", "~user:*"); !aclIsOK(v) {
		t.Fatalf("setup: ACL SETUSER returned type %v %q, want +OK", v.Type, v.Str)
	}
	if v := a.do("AUTH", user, "pw"); !aclIsOK(v) {
		t.Fatalf("AUTH %s pw returned type %v %q, want +OK", user, v.Type, v.Str)
	}

	// 1. An allowed command on an allowed key must still work.
	if v := a.do("GET", "user:1"); v.Type != resp.TypeBulkString || string(v.Bulk) != "secret" {
		t.Fatalf("control: GET user:1 returned type %v %q, want bulk \"secret\"", v.Type, v.Bulk)
	}

	// 2. An allowed command on a FORBIDDEN key must be refused.
	if v := a.do("GET", "other:1"); !aclIsNoperm(v) {
		t.Fatalf("GET other:1 returned type %v %q, want NOPERM — the ~user:* key pattern is not enforced", v.Type, v.Err)
	}

	// 3. A FORBIDDEN command must be refused even on an allowed key.
	if v := a.do("SET", "user:1", "hacked"); !aclIsNoperm(v) {
		t.Fatalf("SET user:1 returned type %v %q, want NOPERM — the -@all +get command set is not enforced", v.Type, v.Err)
	}

	// 4. A refused command must not have mutated anything.
	if v := a.do("GET", "user:1"); string(v.Bulk) != "secret" {
		t.Fatalf("GET user:1 after the refused SET = %q, want \"secret\"", v.Bulk)
	}
}

// A failed AUTH must not install the user: the connection stays on the
// permissive default user rather than becoming half-authorised.
func TestACLAuthWithWrongPasswordGrantsNothing(t *testing.T) {
	const user = "acl_wrongpw_user"
	a := newACLSession(t)

	if v := a.do("ACL", "SETUSER", user, ">pw", "-@all", "+get", "~user:*"); !aclIsOK(v) {
		t.Fatalf("setup: ACL SETUSER returned type %v %q, want +OK", v.Type, v.Str)
	}
	if v := a.do("AUTH", user, "WRONG"); v.Type != resp.TypeError {
		t.Fatalf("AUTH with a wrong password returned type %v, want an error", v.Type)
	}
	if v := a.do("SET", "x:1", "v"); !aclIsOK(v) {
		t.Fatalf("after a failed AUTH, SET was refused (type %v) — auth state leaked from the failed attempt", v.Type)
	}
}

// A connection that never authenticates is unaffected: the default user allows
// everything, so existing deployments keep working.
func TestUnauthenticatedConnectionIsUnrestrictedControl(t *testing.T) {
	a := newACLSession(t)

	if v := a.do("SET", "anything:1", "v"); !aclIsOK(v) {
		t.Fatalf("control: unauthenticated SET was refused (type %v) — default-user behaviour changed", v.Type)
	}
	if v := a.do("GET", "anything:1"); v.Type != resp.TypeBulkString {
		t.Fatalf("control: unauthenticated GET returned type %v, want a bulk string", v.Type)
	}
}

// The permission survives across many commands on one connection, proving the
// user is connection state rather than per-command state.
func TestACLPermissionsPersistAcrossCommands(t *testing.T) {
	const user = "acl_persist_user"
	a := newACLSession(t)

	if v := a.do("ACL", "SETUSER", user, ">pw", "-@all", "+get", "~user:*"); !aclIsOK(v) {
		t.Fatalf("setup: ACL SETUSER returned type %v %q, want +OK", v.Type, v.Str)
	}
	if v := a.do("AUTH", user, "pw"); !aclIsOK(v) {
		t.Fatalf("AUTH returned type %v %q, want +OK", v.Type, v.Str)
	}
	for i := 0; i < 10; i++ {
		if v := a.do("GET", "denied:key"); !aclIsNoperm(v) {
			t.Fatalf("command %d: GET denied:key returned type %v %q, want NOPERM", i, v.Type, v.Err)
		}
	}
}

// DRYRUN must agree with real enforcement: it may not promise a command is
// permitted that the command path then refuses.
func TestDryrunAgreesWithEnforcement(t *testing.T) {
	const user = "acl_dryrun_user"
	a := newACLSession(t)

	if v := a.do("ACL", "SETUSER", user, ">pw", "-@all", "+get", "~user:*"); !aclIsOK(v) {
		t.Fatalf("setup: ACL SETUSER returned type %v %q, want +OK", v.Type, v.Str)
	}

	// DRYRUN runs as the default user, so check the verdict for our user.
	dry := a.do("DRYRUN", user, "set", "user:1", "v")
	if aclIsOK(dry) {
		t.Fatal("DRYRUN said the forbidden SET would succeed")
	}
	// And the real command path agrees.
	if v := a.do("AUTH", user, "pw"); !aclIsOK(v) {
		t.Fatalf("AUTH returned type %v %q, want +OK", v.Type, v.Str)
	}
	if v := a.do("SET", "user:1", "v"); !aclIsNoperm(v) {
		t.Fatalf("real SET returned type %v %q, want NOPERM", v.Type, v.Err)
	}
}

// A &channel restriction from ACL SETUSER must be honoured on the real
// connection. The user is granted the pub/sub commands but confined to &news.*,
// so a publish or subscribe on any other channel must be refused.
//
// Each call names exactly one channel: cmdSUBSCRIBE writes one confirmation per
// channel, so a multi-channel call would emit several replies and desync do().
func TestACLChannelPatternIsEnforcedOnRealConnection(t *testing.T) {
	const user = "acl_channel_user"
	a := newACLSession(t)

	if v := a.do("ACL", "SETUSER", user, ">pw", "-@all",
		"+publish", "+subscribe", "+psubscribe", "&news.*"); !aclIsOK(v) {
		t.Fatalf("setup: ACL SETUSER returned type %v %q, want +OK", v.Type, v.Str)
	}
	if v := a.do("AUTH", user, "pw"); !aclIsOK(v) {
		t.Fatalf("AUTH %s pw returned type %v %q, want +OK", user, v.Type, v.Str)
	}

	// 1. Control: publishing to a channel inside the pattern must work, and
	// must not be refused merely because the message payload does not itself
	// match the pattern.
	if v := a.do("PUBLISH", "news.sports", "totally-unmatched-payload"); v.Type != resp.TypeInteger {
		t.Fatalf("control: PUBLISH news.sports returned type %v %q, want an integer", v.Type, v.Str)
	}

	// 2. Publishing outside the pattern must be refused.
	if v := a.do("PUBLISH", "secret", "hello"); !aclIsNoperm(v) {
		t.Fatalf("PUBLISH secret returned type %v %q, want NOPERM — the &news.* channel pattern is not enforced", v.Type, v.Err)
	}

	// 3. Subscribing inside the pattern must work.
	if v := a.do("SUBSCRIBE", "news.tech"); v.Type != resp.TypeArray {
		t.Fatalf("control: SUBSCRIBE news.tech returned type %v, want an array confirmation", v.Type)
	}

	// 4. Subscribing outside the pattern must be refused.
	if v := a.do("SUBSCRIBE", "private"); !aclIsNoperm(v) {
		t.Fatalf("SUBSCRIBE private returned type %v %q, want NOPERM", v.Type, v.Err)
	}

	// 5. A pattern subscription inside the allowance must work.
	if v := a.do("PSUBSCRIBE", "news.*"); v.Type != resp.TypeArray {
		t.Fatalf("control: PSUBSCRIBE news.* returned type %v, want an array confirmation", v.Type)
	}

	// 6. A pattern subscription outside the allowance must be refused.
	if v := a.do("PSUBSCRIBE", "private.*"); !aclIsNoperm(v) {
		t.Fatalf("PSUBSCRIBE private.* returned type %v %q, want NOPERM", v.Type, v.Err)
	}
}

// A user with no &rule keeps full access to channels, so a key-restricted user
// is not newly confined to publishing nowhere.
func TestACLNoChannelRuleMeansAllChannelsControl(t *testing.T) {
	const user = "acl_nochan_user"
	a := newACLSession(t)

	if v := a.do("ACL", "SETUSER", user, ">pw", "-@all", "+get", "+publish", "~user:*"); !aclIsOK(v) {
		t.Fatalf("setup: ACL SETUSER returned type %v %q, want +OK", v.Type, v.Str)
	}
	if v := a.do("AUTH", user, "pw"); !aclIsOK(v) {
		t.Fatalf("AUTH returned type %v %q, want +OK", v.Type, v.Str)
	}
	if v := a.do("PUBLISH", "any.channel.at.all", "hello"); v.Type != resp.TypeInteger {
		t.Fatalf("control: PUBLISH with no &rule returned type %v %q, want an integer — a key-restricted user was newly confined to no channels", v.Type, v.Str)
	}
}

// A restricted user must not be able to administer the ACL system and widen its
// own permissions.
//
// The user is granted "+acl" on purpose: that is the realistic escalation
// vector, because a user holding ACL access can run
// `ACL SETUSER self +@all ~*` and defeat every key and command restriction at
// once. Granting the command is therefore exactly what must not be enough.
func TestACLRestrictedUserCannotEscalateViaACL(t *testing.T) {
	const user = "acl_esc_user"
	a := newACLSession(t)

	// Control: the default (unauthenticated) user may still administer ACL.
	if v := a.do("ACL", "USERS"); v.Type != resp.TypeArray {
		t.Fatalf("control: ACL USERS as the default user returned type %v, want an array — ACL administration was broken for the default user", v.Type)
	}
	if v := a.do("SET", "user:1", "secret"); !aclIsOK(v) {
		t.Fatalf("setup: SET user:1 returned type %v %q, want +OK", v.Type, v.Str)
	}
	if v := a.do("SET", "other:1", "topsecret"); !aclIsOK(v) {
		t.Fatalf("setup: SET other:1 returned type %v %q, want +OK", v.Type, v.Str)
	}
	if v := a.do("ACL", "SETUSER", user, ">pw", "-@all", "+get", "+acl", "~user:*"); !aclIsOK(v) {
		t.Fatalf("setup: ACL SETUSER returned type %v %q, want +OK", v.Type, v.Str)
	}
	if v := a.do("AUTH", user, "pw"); !aclIsOK(v) {
		t.Fatalf("AUTH %s pw returned type %v %q, want +OK", user, v.Type, v.Str)
	}

	// 1. The escalation attempt itself must be refused.
	if v := a.do("ACL", "SETUSER", user, ">pw2", "+@all", "~*"); !aclIsNoperm(v) {
		t.Fatalf("ACL SETUSER <self> +@all ~* returned type %v %q, want NOPERM — a restricted user can grant itself further permissions", v.Type, v.Err)
	}

	// 2. No other ACL subcommand may be used either.
	for _, sub := range [][]string{
		{"ACL", "DELUSER", "somebody"},
		{"ACL", "GETUSER", user},
		{"ACL", "LIST"},
		{"ACL", "SAVE"},
		{"ACL", "LOAD"},
		{"ACL", "WHOAMI"},
	} {
		if v := a.do(sub...); !aclIsNoperm(v) {
			t.Fatalf("%v returned type %v %q, want NOPERM", sub, v.Type, v.Err)
		}
	}

	// 3. The refused SETUSER must have changed nothing: the key restriction and
	// the command restriction must both still be in force. This is the part
	// that would catch a refused-but-partially-applied escalation.
	if v := a.do("GET", "other:1"); !aclIsNoperm(v) {
		t.Fatalf("GET other:1 returned type %v %q, want NOPERM — the refused ACL SETUSER widened the key restriction", v.Type, v.Err)
	}
	if v := a.do("SET", "user:1", "hacked"); !aclIsNoperm(v) {
		t.Fatalf("SET user:1 returned type %v %q, want NOPERM — the refused ACL SETUSER widened the command restriction", v.Type, v.Err)
	}
	if v := a.do("GET", "user:1"); v.Type != resp.TypeBulkString || string(v.Bulk) != "secret" {
		t.Fatalf("GET user:1 returned type %v %q, want bulk \"secret\"", v.Type, v.Bulk)
	}
}

// A user granted no ACL access at all is refused for the same reason, and the
// refusal must not depend on which check happens to catch it first.
func TestACLUserWithoutACLGrantIsRefusedControl(t *testing.T) {
	const user = "acl_noselfadmin_user"
	a := newACLSession(t)

	if v := a.do("ACL", "SETUSER", user, ">pw", "-@all", "+get", "~user:*"); !aclIsOK(v) {
		t.Fatalf("setup: ACL SETUSER returned type %v %q, want +OK", v.Type, v.Str)
	}
	if v := a.do("AUTH", user, "pw"); !aclIsOK(v) {
		t.Fatalf("AUTH returned type %v %q, want +OK", v.Type, v.Str)
	}
	if v := a.do("ACL", "SETUSER", user, "+@all", "~*"); !aclIsNoperm(v) {
		t.Fatalf("ACL SETUSER returned type %v %q, want NOPERM", v.Type, v.Err)
	}
}

// A user granted "+config" must not be able to reshape server configuration.
//
// The named attack, CONFIG SET requirepass "", is refused. It is worth being
// precise about why: in this implementation requirepass is NOT a settable
// parameter (cmdConfigSet has no case for it, and no default case either), so
// that call would silently no-op even without this gate. The command is still
// refused, because the settable parameters that DO exist — maxmemory,
// maxclients, appendonly, timeout — are a denial-of-service surface that no key
// or command pattern can bound.
func TestACLRestrictedUserCannotRunConfig(t *testing.T) {
	const user = "acl_config_user"
	a := newACLSession(t)

	// Control: the default user keeps CONFIG administration.
	if v := a.do("CONFIG", "GET", "maxmemory"); v.Type != resp.TypeArray {
		t.Fatalf("control: CONFIG GET as the default user returned type %v, want an array — CONFIG administration was broken for the default user", v.Type)
	}
	if v := a.do("SET", "user:1", "secret"); !aclIsOK(v) {
		t.Fatalf("setup: SET user:1 returned type %v %q, want +OK", v.Type, v.Str)
	}
	if v := a.do("ACL", "SETUSER", user, ">pw", "-@all", "+get", "+config", "~user:*"); !aclIsOK(v) {
		t.Fatalf("setup: ACL SETUSER returned type %v %q, want +OK", v.Type, v.Str)
	}
	if v := a.do("AUTH", user, "pw"); !aclIsOK(v) {
		t.Fatalf("AUTH %s pw returned type %v %q, want +OK", user, v.Type, v.Str)
	}

	// 1. The named authentication-removal attempt must be refused.
	if v := a.do("CONFIG", "SET", "requirepass", ""); !aclIsNoperm(v) {
		t.Fatalf("CONFIG SET requirepass \"\" returned type %v %q, want NOPERM", v.Type, v.Err)
	}

	// 2. No CONFIG subcommand may be used either, read or write.
	for _, sub := range [][]string{
		{"CONFIG", "GET", "maxmemory"},
		{"CONFIG", "GET", "*"},
		{"CONFIG", "SET", "maxmemory", "1000"},
		{"CONFIG", "SET", "maxclients", "1"},
		{"CONFIG", "SET", "timeout", "0"},
		{"CONFIG", "RESETSTAT"},
		{"CONFIG", "REWRITE"},
	} {
		if v := a.do(sub...); !aclIsNoperm(v) {
			t.Fatalf("%v returned type %v %q, want NOPERM", sub, v.Type, v.Err)
		}
	}

	// 3. The refusal must be clean: the connection keeps working normally.
	if v := a.do("GET", "user:1"); v.Type != resp.TypeBulkString || string(v.Bulk) != "secret" {
		t.Fatalf("GET user:1 returned type %v %q, want bulk \"secret\" — the CONFIG refusal disturbed the connection", v.Type, v.Bulk)
	}
	if v := a.do("GET", "other:1"); !aclIsNoperm(v) {
		t.Fatalf("GET other:1 returned type %v %q, want NOPERM — the ~user:* key restriction was lost", v.Type, v.Err)
	}
}
