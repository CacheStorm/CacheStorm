package server

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// Session identity binding: a session created by a username login carries
// that ACL identity, so cookie-authenticated requests are enforced with the
// same per-user command and key rules as the Bearer flow. Shared-password
// sessions stay user-less and keep the default-user behaviour.

// sessionACLogin posts a login request built from mock fixture credentials.
func sessionACLogin(t *testing.T, h *HTTPServer, username, password string) *httptest.ResponseRecorder {
	t.Helper()
	creds := struct {
		Username string `json:"username"`
		Password string `json:"password"`
	}{Username: username, Password: password}
	raw, err := json.Marshal(creds)
	if err != nil {
		t.Fatalf("login payload: %v", err)
	}
	req := httptest.NewRequest("POST", "/api/login", bytes.NewReader(raw))
	w := httptest.NewRecorder()
	h.handleLogin(w, req)
	return w
}

// sessionACGet drives a cookie-authenticated GET /api/key/<name> through the
// production wiring (authMiddleware -> handleKey).
func sessionACGet(t *testing.T, h *HTTPServer, sessionToken, key string) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest("GET", "/api/key/"+key, nil)
	if sessionToken != "" {
		req.AddCookie(&http.Cookie{Name: "session_token", Value: sessionToken})
	}
	w := httptest.NewRecorder()
	h.authMiddleware(h.handleKey).ServeHTTP(w, req)
	return w
}

func sessionACToken(t *testing.T, login *httptest.ResponseRecorder) string {
	t.Helper()
	for _, c := range login.Result().Cookies() {
		if c.Name == "session_token" {
			return c.Value
		}
	}
	return ""
}

func TestSessionUsernameLoginCarriesIdentity(t *testing.T) {
	h, _ := aclIdentityServer(t)
	seedACLUser(t, "web", "webpass")

	login := sessionACLogin(t, h, "web", "webpass")
	if login.Code != 200 {
		t.Fatalf("username login failed: status=%d body=%q", login.Code, login.Body.String())
	}
	token := sessionACToken(t, login)
	if token == "" {
		t.Fatalf("a username login must create a session cookie bound to the ACL identity, got none (body=%q)", login.Body.String())
	}

	out := sessionACGet(t, h, token, "other:1")
	if out.Code != 403 || !strings.Contains(out.Body.String(), "NOPERM") {
		t.Fatalf("a session-cookie-authenticated request must run as the session's restricted identity (NOPERM outside ~user:*), got status=%d body=%q", out.Code, out.Body.String())
	}
	in := sessionACGet(t, h, token, "user:1")
	if in.Code != 200 {
		t.Fatalf("the restricted session must still read in-pattern keys, got status=%d body=%q", in.Code, in.Body.String())
	}
}

func TestSessionSharedPasswordStaysDefaultUser(t *testing.T) {
	h, _ := aclIdentityServer(t)

	login := sessionACLogin(t, h, "", "adminpass")
	if login.Code != 200 {
		t.Fatalf("shared-password login refused: status=%d", login.Code)
	}
	token := sessionACToken(t, login)
	if token == "" {
		t.Fatal("shared-password login set no session cookie")
	}
	w := sessionACGet(t, h, token, "other:1")
	if w.Code != 200 {
		t.Fatalf("a user-less session must keep default-user access, got status=%d body=%q", w.Code, w.Body.String())
	}
}

func TestSessionNoCookieIsUnauthorized(t *testing.T) {
	h, _ := aclIdentityServer(t)

	w := sessionACGet(t, h, "", "other:1")
	if w.Code != 401 {
		t.Fatalf("an unauthenticated request was accepted, status=%d", w.Code)
	}
}
