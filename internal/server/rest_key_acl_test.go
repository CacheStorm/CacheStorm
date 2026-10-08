package server

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/cachestorm/cachestorm/internal/store"
)

// The REST key endpoints touch h.store directly instead of going through the
// router, so they used to serve any key to any authenticated HTTP identity —
// reading, writing, listing, and deleting outside the user's ~pattern with no
// ACL check at all. These tests pin the per-identity enforcement: the command
// grants (GET/SET/DEL/KEYS) and the key patterns that /api/execute applies.

func restKeyACLBasic(t *testing.T) string {
	t.Helper()
	return "Basic " + base64.StdEncoding.EncodeToString([]byte("web:webpass"))
}

// postRestKeyACL drives the production wiring: authMiddleware composed with
// the key handler named by the path.
func postRestKeyACL(t *testing.T, h *HTTPServer, authorization, method, path string, body any) *httptest.ResponseRecorder {
	t.Helper()
	var req *http.Request
	if body != nil {
		payload, err := json.Marshal(body)
		if err != nil {
			t.Fatal(err)
		}
		req = httptest.NewRequest(method, path, bytes.NewReader(payload))
	} else {
		req = httptest.NewRequest(method, path, nil)
	}
	if authorization != "" {
		req.Header.Set("Authorization", authorization)
	}
	w := httptest.NewRecorder()
	var handler http.HandlerFunc
	switch {
	case strings.HasPrefix(path, "/api/keys"):
		handler = h.handleKeys
	case strings.HasPrefix(path, "/api/key/"):
		handler = h.handleKey
	default:
		t.Fatalf("unsupported path %q", path)
	}
	h.authMiddleware(handler).ServeHTTP(w, req)
	return w
}

func TestRestKeyReadEnforcesPattern(t *testing.T) {
	h, _ := aclIdentityServer(t)
	seedACLUser(t, "web", "webpass")
	basic := restKeyACLBasic(t)

	w := postRestKeyACL(t, h, basic, "GET", "/api/key/other:1", nil)
	if w.Code != 403 || !strings.Contains(w.Body.String(), "NOPERM") {
		t.Fatalf("REST read of a key outside ~user:* must be refused, got status=%d body=%q", w.Code, w.Body.String())
	}
	// Control: the in-pattern key is served to the same identity.
	w = postRestKeyACL(t, h, basic, "GET", "/api/key/user:1", nil)
	if w.Code != 200 {
		t.Fatalf("in-pattern REST read was refused, status=%d body=%q", w.Code, w.Body.String())
	}
}

func TestRestKeyWriteEnforcesPatternAndGrant(t *testing.T) {
	h, s := aclIdentityServer(t)
	seedACLUser(t, "web", "webpass")
	basic := restKeyACLBasic(t)

	w := postRestKeyACL(t, h, basic, "POST", "/api/keys", map[string]any{"key": "other:2", "value": "injected"})
	if w.Code != 403 || !strings.Contains(w.Body.String(), "NOPERM") {
		t.Fatalf("REST write outside ~user:* must be refused, got status=%d body=%q", w.Code, w.Body.String())
	}
	if _, exists := s.Get("other:2"); exists {
		t.Fatal("a key was created outside the pattern by a refused request")
	}
	// A +get-only identity cannot write even in pattern (no +set grant).
	w = postRestKeyACL(t, h, basic, "POST", "/api/keys", map[string]any{"key": "user:3", "value": "v"})
	if w.Code != 403 || !strings.Contains(w.Body.String(), "NOPERM") {
		t.Fatalf("REST write without a +set grant must be refused, got status=%d body=%q", w.Code, w.Body.String())
	}
	// Control: the admin identity writes freely.
	w = postRestKeyACL(t, h, "Bearer adminpass", "POST", "/api/keys", map[string]any{"key": "admin:1", "value": "v"})
	if w.Code != 200 {
		t.Fatalf("admin write was refused, status=%d body=%q", w.Code, w.Body.String())
	}
}

func TestRestKeyListEnforcesPatternAndGrant(t *testing.T) {
	h, s := aclIdentityServer(t)
	seedACLUser(t, "web", "webpass", "+keys")
	s.Set("other:9", &store.StringValue{Data: []byte("x")}, store.SetOptions{})

	w := postRestKeyACL(t, h, restKeyACLBasic(t), "GET", "/api/keys", nil)
	if w.Code != 200 {
		t.Fatalf("listing failed, status=%d body=%q", w.Code, w.Body.String())
	}
	var body struct {
		Count int `json:"count"`
		Keys  []struct {
			Key string `json:"key"`
		} `json:"keys"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	for _, k := range body.Keys {
		if !strings.HasPrefix(k.Key, "user:") {
			t.Fatalf("key listing exposed %q to a ~user:* identity", k.Key)
		}
	}
	if body.Count != 1 {
		t.Fatalf("expected only the in-pattern key in the listing, got count=%d", body.Count)
	}

	// Without the +keys command grant the listing is refused even though the
	// key patterns would allow it.
	seedACLUser(t, "getonly", "pw")
	w = postRestKeyACL(t, h, "Basic "+base64.StdEncoding.EncodeToString([]byte("getonly:pw")), "GET", "/api/keys", nil)
	if w.Code != 403 || !strings.Contains(w.Body.String(), "NOPERM") {
		t.Fatalf("listing without a +keys grant must be NOPERM, got status=%d body=%q", w.Code, w.Body.String())
	}
}

func TestRestKeyDeleteEnforcesPattern(t *testing.T) {
	h, s := aclIdentityServer(t)
	seedACLUser(t, "web", "webpass")
	s.Set("other:1", &store.StringValue{Data: []byte("secret")}, store.SetOptions{})

	w := postRestKeyACL(t, h, restKeyACLBasic(t), "DELETE", "/api/key/other:1", nil)
	if w.Code != 403 || !strings.Contains(w.Body.String(), "NOPERM") {
		t.Fatalf("REST delete outside ~user:* must be refused, got status=%d body=%q", w.Code, w.Body.String())
	}
	if _, exists := s.Get("other:1"); !exists {
		t.Fatal("the key was deleted by a refused request")
	}
	// Control: the admin identity deletes.
	w = postRestKeyACL(t, h, "Bearer adminpass", "DELETE", "/api/key/other:1", nil)
	if w.Code != 200 {
		t.Fatalf("admin delete was refused, status=%d body=%q", w.Code, w.Body.String())
	}
	// No credentials with auth enabled: still refused.
	w = postRestKeyACL(t, h, "", "GET", "/api/key/user:1", nil)
	if w.Code != 401 {
		t.Fatalf("an unauthenticated request was accepted, status=%d", w.Code)
	}
}
