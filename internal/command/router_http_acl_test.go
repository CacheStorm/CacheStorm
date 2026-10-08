package command

import (
	"bytes"
	"strings"
	"testing"

	"github.com/cachestorm/cachestorm/internal/acl"
	"github.com/cachestorm/cachestorm/internal/resp"
	"github.com/cachestorm/cachestorm/internal/store"
)

// Router.ExecuteHTTP documents "auth enforcement" but used to skip ACL
// enforcement entirely: only Router.Execute called enforceACL, so a caller
// that authenticated via ACL and set ctx.ACLUser lost every command and key
// restriction on the HTTP path. These tests pin the wired enforcement through
// the exported router entry point. (The production HTTP handler does not set
// ACLUser today — one shared Bearer password, no ACL identity — so requests
// without an ACL user keep the permissive default behaviour.)
func httpACLEntryRouter(t *testing.T) *Router {
	t.Helper()
	router := NewRouter()
	RegisterServerCommands(router)
	RegisterStringCommands(router)
	return router
}

func httpACLEntryUser(t *testing.T, rules string) *acl.User {
	t.Helper()
	user, err := acl.NewACL().CreateUser("web")
	if err != nil {
		t.Fatalf("CreateUser: %v", err)
	}
	if err := acl.ParseACLRule("on -@all "+rules, user); err != nil {
		t.Fatalf("ParseACLRule: %v", err)
	}
	return user
}

func runThroughExecuteHTTP(t *testing.T, router *Router, s *store.Store, user *acl.User, cmd string, args ...string) (string, error) {
	t.Helper()
	buf := &bytes.Buffer{}
	ctx := &Context{
		Command: cmd,
		Args:    bytesArgs(args...),
		Store:   s,
		Writer:  resp.NewWriter(buf),
		ACLUser: user,
	}
	// ExecuteHTTP returns (result, error); the observable ACL behaviour is
	// the error plus whatever the handler wrote to ctx.Writer.
	_, err := router.ExecuteHTTP(ctx)
	return buf.String(), err
}

func TestExecuteHTTPEnforcesACLCommandAndKeys(t *testing.T) {
	s := store.NewStore()
	s.Set("other:1", &store.StringValue{Data: []byte("secret")}, store.SetOptions{})
	s.Set("user:1", &store.StringValue{Data: []byte("mine")}, store.SetOptions{})
	router := httpACLEntryRouter(t)
	user := httpACLEntryUser(t, "+get ~user:*")

	// Key pattern: GET on a key outside ~user:* is refused, like on TCP.
	written, err := runThroughExecuteHTTP(t, router, s, user, "GET", "other:1")
	if err == nil || !strings.Contains(err.Error(), "NOPERM") {
		t.Fatalf("ExecuteHTTP must not run GET on a key outside ~user:*, err=%v written=%q", err, written)
	}

	// Command: DEL is not granted to this user.
	written, err = runThroughExecuteHTTP(t, router, s, user, "DEL", "user:1")
	if err == nil || !strings.Contains(err.Error(), "NOPERM") {
		t.Fatalf("ExecuteHTTP must not run DEL without a +del grant, err=%v written=%q", err, written)
	}
	if _, exists := s.Get("user:1"); !exists {
		t.Fatal("the key was deleted by a refused HTTP command")
	}

	// Control: a granted command on an in-pattern key is served.
	written, err = runThroughExecuteHTTP(t, router, s, user, "GET", "user:1")
	if err != nil || !strings.Contains(written, "mine") {
		t.Fatalf("a granted command on an in-pattern key was restricted, err=%v written=%q", err, written)
	}
	// Control: a connection without an ACL user keeps the permissive default.
	written, err = runThroughExecuteHTTP(t, router, s, nil, "GET", "other:1")
	if err != nil || !strings.Contains(written, "secret") {
		t.Fatalf("the default caller lost HTTP access, err=%v written=%q", err, written)
	}
}
