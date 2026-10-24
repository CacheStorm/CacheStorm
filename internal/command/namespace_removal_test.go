package command

import (
	"bytes"
	"strings"
	"testing"

	"github.com/cachestorm/cachestorm/internal/resp"
	"github.com/cachestorm/cachestorm/internal/store"
)

func newNamespaceRemovalRouter(s *store.Store) *Router {
	router := NewRouter()
	RegisterStringCommands(router)
	RegisterNamespaceCommands(router)
	return router
}

// SELECT switches to another database within the context: the key written
// after SELECT 5 is not visible in database 0, and an index past the last
// database is refused.
func TestSelectSwitchesDatabase(t *testing.T) {
	s := store.NewStore()
	r := newNamespaceRemovalRouter(s)

	ctx := NewContext("SELECT", [][]byte{[]byte("5")}, s, resp.NewWriter(&bytes.Buffer{}))
	if err := r.Execute(ctx); err != nil {
		t.Fatal(err)
	}
	if ctx.DB != 5 || ctx.Store == s {
		t.Fatalf("SELECT 5 must switch the context to database 5, got db=%d", ctx.DB)
	}
	if err := ctx.Store.Set("k", &store.StringValue{Data: []byte("v")}, store.SetOptions{}); err != nil {
		t.Fatal(err)
	}
	if s.Exists("k") {
		t.Fatal("a key written in database 5 must not appear in database 0")
	}
	if reply := runCmd(t, s, r, "SELECT", "16"); !strings.Contains(reply, "DB index is out of range") {
		t.Fatalf("SELECT 16 must be refused, got %q", reply)
	}
}

// The dead namespace surface must be gone: NAMESPACES, NAMESPACE,
// NAMESPACEDEL, and NAMESPACEINFO operated on a namespace manager that no
// production path could ever write data into, so they answered with zeros
// for keyspaces that could never hold keys. They are removed; Redis
// clients keep SELECT. A removed command surfaces as an Execute error, not
// a reply, so this test drives the router directly.
func TestNamespaceSurfaceRemoved(t *testing.T) {
	s := store.NewStore()
	r := newNamespaceRemovalRouter(s)

	for _, cmd := range []string{"NAMESPACES", "NAMESPACE", "NAMESPACEDEL", "NAMESPACEINFO"} {
		var buf bytes.Buffer
		ctx := &Context{
			Store:   s,
			Command: cmd,
			Writer:  resp.NewWriter(&buf),
			Args:    [][]byte{[]byte("x")},
		}
		err := r.Execute(ctx)
		if err == nil || !strings.Contains(err.Error(), "unknown command") {
			t.Fatalf("%s must be removed as a dead surface, got err=%v reply=%q", cmd, err, buf.String())
		}
	}
}
