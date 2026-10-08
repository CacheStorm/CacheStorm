package server

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/cachestorm/cachestorm/internal/command"
	"github.com/cachestorm/cachestorm/internal/store"
)

// Per-user ACL rules used to be unreachable from the HTTP API: the auth
// middleware only accepted the shared config password or a session cookie,
// so every authenticated request ran as the unrestricted default user. These
// tests pin the identity mapping — HTTP Basic or a username login resolves
// against the ACL registry, the resolved identity is carried into the
// command context, and ExecuteHTTP enforces its command and key rules.

func aclIdentityServer(t *testing.T) (*HTTPServer, *store.Store) {
	t.Helper()
	s := store.NewStore()
	s.Set("user:1", &store.StringValue{Data: []byte("mine")}, store.SetOptions{})
	s.Set("other:1", &store.StringValue{Data: []byte("secret")}, store.SetOptions{})
	router := command.NewRouter()
	command.RegisterServerCommands(router)
	command.RegisterStringCommands(router)
	h := NewHTTPServer(s, router, &HTTPConfig{Password: "adminpass"})
	t.Cleanup(h.cancel)
	return h, s
}

// seedACLUser creates the restricted identity over the RESP path: the HTTP
// execute endpoint refuses ACL admin by design (httpDangerousCommands), so
// the user is declared through a real Connection, exactly as TCP clients do.
// ACL users live in the process-global registry, which is what the HTTP
// identity mapping authenticates against.
func seedACLUser(t *testing.T, username, password string, extraRules ...string) {
	t.Helper()
	sess := newACLSession(t)
	args := []string{"ACL", "SETUSER", username, ">" + password, "on", "-@all", "+get", "~user:*"}
	args = append(args, extraRules...)
	if v := sess.do(args...); !aclIsOK(v) {
		t.Fatalf("seeding ACL user %s failed: %v", username, v)
	}
}

func postExecuteIdentity(t *testing.T, h *HTTPServer, authorization string, command string, args ...string) *httptest.ResponseRecorder {
	t.Helper()
	payload, err := json.Marshal(map[string]any{"command": command, "args": args})
	if err != nil {
		t.Fatal(err)
	}
	req := httptest.NewRequest("POST", "/api/execute", bytes.NewReader(payload))
	if authorization != "" {
		req.Header.Set("Authorization", authorization)
	}
	w := httptest.NewRecorder()
	// Production wiring: authMiddleware composes handleExecute.
	h.authMiddleware(h.handleExecute).ServeHTTP(w, req)
	return w
}

func postLoginIdentity(t *testing.T, h *HTTPServer, body string) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest("POST", "/api/login", strings.NewReader(body))
	w := httptest.NewRecorder()
	h.handleLogin(w, req)
	return w
}

func TestHTTPACLIdentityBasic(t *testing.T) {
	h, _ := aclIdentityServer(t)
	seedACLUser(t, "web", "webpass")

	basic := "Basic " + base64.StdEncoding.EncodeToString([]byte("web:webpass"))
	w := postExecuteIdentity(t, h, basic, "GET", "other:1")
	if w.Code != 400 || !strings.Contains(w.Body.String(), "NOPERM") {
		t.Fatalf("a restricted HTTP identity must get NOPERM outside ~user:*, got status=%d body=%q", w.Code, w.Body.String())
	}
	w = postExecuteIdentity(t, h, basic, "GET", "user:1")
	if w.Code != 200 {
		t.Fatalf("an in-pattern key must be served to the restricted identity, got status=%d body=%q", w.Code, w.Body.String())
	}
}

func TestHTTPACLIdentityLoginAndBearer(t *testing.T) {
	h, _ := aclIdentityServer(t)
	seedACLUser(t, "web", "webpass")

	// A username login authenticates against the ACL registry and hands back
	// the Bearer token the middleware accepts.
	login := postLoginIdentity(t, h, `{"username":"web","password":"webpass"}`)
	if login.Code != 200 {
		t.Fatalf("ACL login with a username must succeed, got status=%d body=%q", login.Code, login.Body.String())
	}
	var loginBody struct {
		Success  bool   `json:"success"`
		Username string `json:"username"`
		Token    string `json:"token"`
	}
	if err := json.Unmarshal(login.Body.Bytes(), &loginBody); err != nil {
		t.Fatal(err)
	}
	if !loginBody.Success || loginBody.Username != "web" || loginBody.Token != "web:webpass" {
		t.Fatalf("unexpected login response: %+v", loginBody)
	}

	w := postExecuteIdentity(t, h, "Bearer "+loginBody.Token, "GET", "other:1")
	if w.Code != 400 || !strings.Contains(w.Body.String(), "NOPERM") {
		t.Fatalf("the Bearer identity must get NOPERM outside ~user:*, got status=%d body=%q", w.Code, w.Body.String())
	}
	w = postExecuteIdentity(t, h, "Bearer "+loginBody.Token, "GET", "user:1")
	if w.Code != 200 {
		t.Fatalf("an in-pattern key must be served to the Bearer identity, got status=%d body=%q", w.Code, w.Body.String())
	}
	// A wrong password for a known user is refused.
	if w := postLoginIdentity(t, h, `{"username":"web","password":"nope"}`); w.Code != 401 {
		t.Fatalf("invalid login credentials must be refused, got status=%d", w.Code)
	}
}

func TestHTTPACLIdentityBoundaries(t *testing.T) {
	h, s := aclIdentityServer(t)
	seedACLUser(t, "web", "webpass")

	// The shared admin password keeps its unrestricted default-user
	// behaviour.
	w := postExecuteIdentity(t, h, "Bearer adminpass", "GET", "other:1")
	if w.Code != 200 {
		t.Fatalf("the admin identity was restricted, status=%d body=%q", w.Code, w.Body.String())
	}
	// No credentials with auth enabled: still refused.
	w = postExecuteIdentity(t, h, "", "GET", "user:1")
	if w.Code != 401 {
		t.Fatalf("an unauthenticated request was accepted, status=%d", w.Code)
	}
	// Presented-but-invalid credentials: refused even though the same
	// request without them would have been accepted as the default user.
	w = postExecuteIdentity(t, h, "Basic "+base64.StdEncoding.EncodeToString([]byte("web:wrong")), "GET", "user:1")
	if w.Code != 401 {
		t.Fatalf("invalid credentials were accepted, status=%d", w.Code)
	}
	// A command the identity does not hold is refused with the key intact.
	basic := "Basic " + base64.StdEncoding.EncodeToString([]byte("web:webpass"))
	w = postExecuteIdentity(t, h, basic, "DEL", "user:1")
	if w.Code != 400 || !strings.Contains(w.Body.String(), "NOPERM") {
		t.Fatalf("DEL without a +del grant must be NOPERM, got status=%d body=%q", w.Code, w.Body.String())
	}
	if _, exists := s.Get("user:1"); !exists {
		t.Fatal("the key was deleted by a refused HTTP command")
	}
}
