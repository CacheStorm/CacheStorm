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

// SELECT must keep working as the standard single-keyspace compatibility
// no-op: real Redis clients issue SELECT during handshake, so it answers OK
// and the server keeps one shared keyspace.
func TestSelectIsCompatibilityNoOp(t *testing.T) {
	s := store.NewStore()
	r := newNamespaceRemovalRouter(s)

	if reply := runCmd(t, s, r, "SELECT", "5"); !strings.Contains(reply, "+OK") {
		t.Fatalf("SELECT must answer OK on a single-keyspace server, got %q", reply)
	}
	runCmd(t, s, r, "SET", "k", "v")
	if got := runCmd(t, s, r, "GET", "k"); !strings.Contains(got, "v") {
		t.Fatalf("GET after SELECT must see the shared keyspace, got %q", got)
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
