package server

import (
	"encoding/base64"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/cachestorm/cachestorm/internal/store"
)

// base64Std encodes user:password pairs for HTTP Basic credentials.
func base64Std(s string) string {
	return base64.StdEncoding.EncodeToString([]byte(s))
}

// The tag REST endpoints touch the tag index and h.store directly instead of
// going through the router, so they used to leak out-of-pattern key names
// (/api/tag/<tag>, /api/tags) and let a restricted identity drop keys it
// could never delete (/api/invalidate/<tag>). These tests pin the per-user
// enforcement: tag discovery needs the KEYS command grant, tag contents and
// counts are filtered by the user's key patterns, and invalidating a tag is
// all-or-nothing — the DEL grant plus every tagged key within the patterns.

func tagACLSetup(t *testing.T) (*HTTPServer, *store.Store) {
	t.Helper()
	h, s := aclIdentityServer(t)
	// Re-seed the fixture keys WITH tags: user:1 is in pattern for the
	// restricted identity, the other:* keys are not.
	mustSetTagged := func(key, value string, tags ...string) {
		if err := s.Set(key, &store.StringValue{Data: []byte(value)}, store.SetOptions{Tags: tags}); err != nil {
			t.Fatalf("seeding %s: %v", key, err)
		}
	}
	mustSetTagged("user:1", "mine", "shared")
	mustSetTagged("other:1", "secret", "shared")
	mustSetTagged("other:2", "x", "private")
	return h, s
}

func postTagACL(t *testing.T, h *HTTPServer, authorization, method, path string) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(method, path, nil)
	if authorization != "" {
		req.Header.Set("Authorization", authorization)
	}
	w := httptest.NewRecorder()
	var handler http.HandlerFunc
	switch {
	case strings.HasPrefix(path, "/api/invalidate/"):
		handler = h.handleInvalidate
	case strings.HasPrefix(path, "/api/tag/"):
		handler = h.handleTag
	case path == "/api/tags":
		handler = h.handleTags
	default:
		t.Fatalf("unsupported path %q", path)
	}
	// Production wiring: authMiddleware composes each tag handler.
	h.authMiddleware(handler).ServeHTTP(w, req)
	return w
}

func decodeTagKeys(t *testing.T, w *httptest.ResponseRecorder) (int, map[string]bool, int) {
	t.Helper()
	// /api/tag/<tag> returns {"count":N,"keys":["a","b"],"tag":"..."} — the
	// keys are plain strings.
	var body struct {
		Count int      `json:"count"`
		Keys  []string `json:"keys"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
		t.Fatalf("bad JSON %q: %v", w.Body.String(), err)
	}
	seen := map[string]bool{}
	for _, k := range body.Keys {
		seen[k] = true
	}
	return body.Count, seen, len(body.Keys)
}

func TestRestTagReadEnforcesPattern(t *testing.T) {
	h, _ := tagACLSetup(t)
	seedACLUser(t, "web", "webpass", "+keys")
	basic := "Basic " + base64Std("web:webpass")

	w := postTagACL(t, h, basic, "GET", "/api/tag/shared")
	if w.Code != 200 {
		t.Fatalf("tag read failed: status=%d body=%q", w.Code, w.Body.String())
	}
	count, seen, listed := decodeTagKeys(t, w)
	if seen["other:1"] || seen["other:2"] {
		t.Fatalf("tag listing exposed out-of-pattern keys to a ~user:* identity: %v", seen)
	}
	if !seen["user:1"] || count != 1 || listed != 1 {
		t.Fatalf("expected only the in-pattern key, count=%d listed=%d seen=%v", count, listed, seen)
	}
}

func TestRestTagInvalidateEnforcesPatternAndGrant(t *testing.T) {
	h, s := tagACLSetup(t)
	seedACLUser(t, "web", "webpass", "+del")
	basic := "Basic " + base64Std("web:webpass")

	// The tag spans an out-of-pattern key: all-or-nothing refusal.
	w := postTagACL(t, h, basic, "POST", "/api/invalidate/shared")
	if w.Code != 403 || !strings.Contains(w.Body.String(), "NOPERM") {
		t.Fatalf("invalidate of a tag containing out-of-pattern keys must be refused, got status=%d body=%q", w.Code, w.Body.String())
	}
	for _, key := range []string{"other:1", "user:1"} {
		if _, exists := s.Get(key); !exists {
			t.Fatalf("key %s was dropped by a refused invalidate", key)
		}
	}

	// Control: the admin identity invalidates the whole tag.
	w = postTagACL(t, h, "Bearer adminpass", "POST", "/api/invalidate/shared")
	if w.Code != 200 {
		t.Fatalf("admin invalidate was refused, status=%d body=%q", w.Code, w.Body.String())
	}
	for _, key := range []string{"user:1", "other:1"} {
		if _, exists := s.Get(key); exists {
			t.Fatalf("admin invalidate left %s behind", key)
		}
	}
}

func TestRestTagsListFiltersCountsAndGrant(t *testing.T) {
	h, _ := tagACLSetup(t)
	seedACLUser(t, "web", "webpass", "+keys")
	basic := "Basic " + base64Std("web:webpass")

	w := postTagACL(t, h, basic, "GET", "/api/tags")
	if w.Code != 200 {
		t.Fatalf("tags listing failed: status=%d body=%q", w.Code, w.Body.String())
	}
	var body struct {
		Count int `json:"count"`
		Tags  []struct {
			Name  string `json:"name"`
			Count int    `json:"count"`
		} `json:"tags"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	for _, tag := range body.Tags {
		if tag.Name == "private" {
			t.Fatalf("tags listing exposed the out-of-pattern tag %q", tag.Name)
		}
		if tag.Name == "shared" && tag.Count != 1 {
			t.Fatalf("shared count must be filtered to accessible keys, got %d", tag.Count)
		}
	}
	if body.Count != 1 {
		t.Fatalf("expected only the accessible tag, got count=%d", body.Count)
	}

	// Without the +keys command grant the listing is refused even though the
	// key patterns would allow it.
	seedACLUser(t, "getonly", "pw")
	w = postTagACL(t, h, "Basic "+base64Std("getonly:pw"), "GET", "/api/tags", )
	if w.Code != 403 || !strings.Contains(w.Body.String(), "NOPERM") {
		t.Fatalf("tags listing without a +keys grant must be NOPERM, got status=%d body=%q", w.Code, w.Body.String())
	}
}

func TestRestTagEndpointBoundaries(t *testing.T) {
	h, _ := tagACLSetup(t)

	// The admin identity keeps full visibility.
	w := postTagACL(t, h, "Bearer adminpass", "GET", "/api/tag/shared")
	if w.Code != 200 {
		t.Fatalf("admin tag read was refused, status=%d", w.Code)
	}
	// No credentials with auth enabled: refused.
	w = postTagACL(t, h, "", "GET", "/api/tag/shared")
	if w.Code != 401 {
		t.Fatalf("an unauthenticated request was accepted, status=%d", w.Code)
	}
	// A +keys-less identity is refused on tag discovery.
	seedACLUser(t, "getonly", "pw")
	w = postTagACL(t, h, "Basic "+base64Std("getonly:pw"), "GET", "/api/tag/shared")
	if w.Code != 403 || !strings.Contains(w.Body.String(), "NOPERM") {
		t.Fatalf("tag discovery without +keys must be NOPERM, got status=%d body=%q", w.Code, w.Body.String())
	}
}
