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

// Round r41 proof: username-Bearer parsing splits on the FIRST colon, so a
// username containing a colon (a:b, password c) parses as user "a" with
// password "b:c" and can never authenticate over the Bearer flow — while
// /api/login (JSON, no splitting) and session binding handle the same user
// fine. The fix splits the Bearer token on the LAST colon. HTTP Basic keeps
// its RFC 7617 first-colon split (a Basic user-id cannot contain a colon),
// which stays a documented limitation.

func colonSetup(t *testing.T) *HTTPServer {
	t.Helper()
	h, _ := aclIdentityServer(t)
	seedACLUser(t, "a:b", "c", "-@all", "+get", "~ok:*")
	return h
}

func colonSeedKey(t *testing.T, h *HTTPServer) {
	t.Helper()
	if err := h.store.Set("ok:1", &store.StringValue{Data: []byte("value")}, store.SetOptions{}); err != nil {
		t.Fatal(err)
	}
}

func colonGet(t *testing.T, h *HTTPServer, authorization string) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest("GET", "/api/key/ok:1", nil)
	if authorization != "" {
		req.Header.Set("Authorization", authorization)
	}
	w := httptest.NewRecorder()
	h.authMiddleware(h.handleKey).ServeHTTP(w, req)
	return w
}

// THE DEFECT: the correct username-Bearer credentials for a colon username
// must authenticate.
func TestProofColonUsernameBearerAuthenticates(t *testing.T) {
	h := colonSetup(t)
	colonSeedKey(t, h)

	w := colonGet(t, h, "Bearer a:b:c")
	if w.Code == 401 || !strings.Contains(w.Body.String(), "value") {
		t.Fatalf("FAIL: the correct Bearer credentials for username 'a:b' must authenticate, got status=%d body=%q", w.Code, w.Body.String())
	}
}

// CONTROL: the same user's /api/login session already works — only the
// Bearer parse diverges.
func TestProofControlColonUsernameSessionWorks(t *testing.T) {
	h := colonSetup(t)
	colonSeedKey(t, h)

	loginBody, _ := json.Marshal(map[string]string{"username": "a:b", "password": "c"})
	req := httptest.NewRequest("POST", "/api/login", bytes.NewReader(loginBody))
	lw := httptest.NewRecorder()
	h.handleLogin(lw, req)
	if lw.Code != 200 {
		t.Fatalf("control failed: /api/login must authenticate a colon username, status=%d body=%q", lw.Code, lw.Body.String())
	}
	var token string
	for _, c := range lw.Result().Cookies() {
		if c.Name == "session_token" {
			token = c.Value
		}
	}
	if token == "" {
		t.Fatal("control failed: login set no session cookie")
	}

	req2 := httptest.NewRequest("GET", "/api/key/ok:1", nil)
	req2.AddCookie(&http.Cookie{Name: "session_token", Value: token})
	w2 := httptest.NewRecorder()
	h.authMiddleware(h.handleKey).ServeHTTP(w2, req2)
	if w2.Code != 200 || !strings.Contains(w2.Body.String(), "value") {
		t.Fatalf("control failed: the colon username's session must read its key, status=%d body=%q", w2.Code, w2.Body.String())
	}
}

// CONTROL: a normal (colon-less) username over Bearer is unaffected.
func TestProofControlPlainBearerUnaffected(t *testing.T) {
	h, _ := aclIdentityServer(t)
	seedACLUser(t, "web", "webpass", "-@all", "+get", "~ok:*")
	colonSeedKey(t, h)

	w := colonGet(t, h, "Bearer web:webpass")
	if w.Code != 200 || !strings.Contains(w.Body.String(), "value") {
		t.Fatalf("control failed: a plain Bearer identity must keep working, status=%d body=%q", w.Code, w.Body.String())
	}
}

// CONTROL (named limitation): HTTP Basic keeps its RFC 7617 first-colon
// split — a Basic user-id cannot contain a colon — so this stays refused
// before AND after the fix.
func TestProofControlColonUsernameBasicStaysRefused(t *testing.T) {
	h := colonSetup(t)
	colonSeedKey(t, h)

	b64 := base64.StdEncoding.EncodeToString([]byte("a:b:c"))
	w := colonGet(t, h, "Basic "+b64)
	if w.Code != 401 {
		t.Fatalf("control failed: Basic with a colon username must stay refused (RFC 7617), status=%d body=%q", w.Code, w.Body.String())
	}
}
