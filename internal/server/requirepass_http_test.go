package server

import (
	"bytes"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/cachestorm/cachestorm/internal/command"
	"github.com/cachestorm/cachestorm/internal/resp"
	"github.com/cachestorm/cachestorm/internal/store"
)

// Round r39 proof: Router.ExecuteHTTP never applies the requirepass gate.
// Router.Execute refuses unauthenticated commands with NOAUTH when
// requirepass is set, but ExecuteHTTP sets ctx.Authenticated = true
// unconditionally and runs the handler — so a deployment that secures the
// server with requirepass and enables the HTTP API without an HTTP password
// serves the whole command surface wide open.

func requirepassRouter(t *testing.T) (*command.Router, *store.Store) {
	t.Helper()
	router := command.NewRouter()
	command.RegisterStringCommands(router)
	router.SetRequirePass("s3cret")
	s := store.NewStore()
	if err := s.Set("other:1", &store.StringValue{Data: []byte("secret")}, store.SetOptions{}); err != nil {
		t.Fatal(err)
	}
	return router, s
}

// runRouterPath drives ONE entry point per call with a fresh buffer, so each
// reply is attributable to the path that produced it.
func runRouterPath(t *testing.T, router *command.Router, s *store.Store, authenticated, viaHTTP bool) string {
	t.Helper()
	var buf bytes.Buffer
	ctx := &command.Context{
		Command:       "GET",
		Args:          [][]byte{[]byte("other:1")},
		Store:         s,
		Writer:        resp.NewWriter(&buf),
		Authenticated: authenticated,
	}
	if viaHTTP {
		if _, err := router.ExecuteHTTP(ctx); err != nil {
			t.Logf("ExecuteHTTP error: %v", err)
		}
	} else {
		if err := router.Execute(ctx); err != nil {
			t.Logf("Execute error: %v", err)
		}
	}
	return buf.String()
}

// The TCP contract: requirepass set + not authenticated -> NOAUTH. The HTTP
// entry point must hold the same gate instead of executing the command.
func TestProofExecuteHTTPSkipsRequirepass(t *testing.T) {
	router, s := requirepassRouter(t)

	// TCP control: the gate refuses the anonymous call.
	tcp := runRouterPath(t, router, s, false, false)
	if !strings.Contains(tcp, "NOAUTH") {
		t.Fatalf("control failed: Execute must refuse unauthenticated commands when requirepass is set, got %q", tcp)
	}

	// RED: the HTTP entry point runs the same anonymous command.
	httpReply := runRouterPath(t, router, s, false, true)
	if !strings.Contains(httpReply, "NOAUTH") {
		t.Fatalf("FAIL: ExecuteHTTP must apply the requirepass gate for an unauthenticated caller, got %q", httpReply)
	}

	// Control: an authenticated caller is unaffected.
	authed := runRouterPath(t, router, s, true, true)
	if !strings.Contains(authed, "secret") {
		t.Fatalf("control failed: an authenticated caller must still read, got %q", authed)
	}
}

// Deployment level: requirepass set + HTTP enabled with NO HTTP password —
// the middleware passes everyone as anonymous and the API sits wide open.
func TestProofHTTPOpenDespiteRequirepass(t *testing.T) {
	router, s := requirepassRouter(t)
	h := NewHTTPServer(s, router, &HTTPConfig{Password: ""})
	t.Cleanup(h.cancel)

	req := httptest.NewRequest("GET", "/api/key/other:1", nil)
	w := httptest.NewRecorder()
	h.authMiddleware(h.handleKey).ServeHTTP(w, req)

	if strings.Contains(w.Body.String(), "secret") || w.Code == 200 {
		t.Fatalf("FAIL: a requirepass deployment must not serve the HTTP API wide open, status=%d body=%q", w.Code, w.Body.String())
	}
}

// Control: an authenticated HTTP caller keeps full access even with
// requirepass set.
func TestProofControlAuthenticatedHTTPStillWorks(t *testing.T) {
	router, s := requirepassRouter(t)
	h := NewHTTPServer(s, router, &HTTPConfig{Password: "adminpass"})
	t.Cleanup(h.cancel)

	req := httptest.NewRequest("GET", "/api/key/other:1", nil)
	req.Header.Set("Authorization", "Bearer adminpass")
	w := httptest.NewRecorder()
	h.authMiddleware(h.handleKey).ServeHTTP(w, req)

	if w.Code != 200 || !strings.Contains(w.Body.String(), "secret") {
		t.Fatalf("control failed: an authenticated HTTP request must still read, status=%d body=%q", w.Code, w.Body.String())
	}
}
