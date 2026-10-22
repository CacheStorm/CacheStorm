package server

// Regression coverage for channel-pattern ACL on the HTTP execute endpoint.
//
// handleExecute routes through Router.ExecuteHTTP, which calls the SAME
// enforceACL as the TCP path. That path checks &channel patterns via
// aclChannelsForCommand -> CanAccessChannel before dispatch. Channel enforcement
// was previously only covered over a real TCP connection
// (connection_acl_test.go: TestACLChannelPatternIsEnforcedOnRealConnection), so
// the HTTP entry point had no coverage at all: removing the channel loop from
// enforceACL, or bypassing enforceACL from ExecuteHTTP, would have gone
// unnoticed.
//
// The request is driven with httptest.NewRequest against
// h.authMiddleware(h.handleExecute) — the production composition — so no port is
// bound and the test stays hermetic.

import (
	"encoding/base64"
	"strings"
	"testing"

	"github.com/cachestorm/cachestorm/internal/command"
	"github.com/cachestorm/cachestorm/internal/store"
)

const (
	chanUser     = "http_chan_user"
	chanPassword = "chanpass"
)

// newHTTPPubSubServer builds an HTTPServer whose router also has the pub/sub
// commands, which the pre-existing HTTP identity fixture does not register.
// Password is left empty so a request with Basic credentials takes the
// aclIdentityFromRequest path in authMiddleware, which is the path under test.
func newHTTPPubSubServer(t *testing.T) *HTTPServer {
	t.Helper()

	s := store.NewStore()
	router := command.NewRouter()
	command.RegisterServerCommands(router)
	command.RegisterStringCommands(router)
	command.RegisterPubSubCommands(router)

	h := NewHTTPServer(s, router, &HTTPConfig{Password: ""})
	t.Cleanup(h.cancel)
	return h
}

func basicAuthFor(user, password string) string {
	return "Basic " + base64.StdEncoding.EncodeToString([]byte(user+":"+password))
}

// A user confined to &news.* must be refused on a channel outside that pattern
// when it authenticates over HTTP.
func TestHTTPExecuteRespectsChannelPattern(t *testing.T) {
	// ACL admin is blocked over HTTP (httpDangerousCommands), so the identity is
	// seeded over the RESP path, exactly as a TCP client would.
	seedACLUser(t, chanUser, chanPassword, "+publish", "+subscribe", "&news.*")

	h := newHTTPPubSubServer(t)
	auth := basicAuthFor(chanUser, chanPassword)

	// Control: a channel INSIDE the pattern must be accepted. Without it, a
	// NOPERM below could be caused by bad credentials or an unknown command
	// rather than by channel enforcement.
	ctl := postExecuteIdentity(t, h, auth, "PUBLISH", "news.allowed", "hello")
	if body := ctl.Body.String(); strings.Contains(body, "NOPERM") || strings.Contains(body, "NOAUTH") {
		t.Fatalf("control: PUBLISH to an allowed channel was refused with %q — credentials "+
			"or command routing are broken, so the refusal below would prove nothing", strings.TrimSpace(body))
	}

	// The contract: a channel outside &news.* must be refused.
	got := postExecuteIdentity(t, h, auth, "PUBLISH", "private.secret", "leak")
	body := got.Body.String()
	if !strings.Contains(body, "NOPERM") {
		t.Errorf("an ACL user limited to &news.* published to private.secret over "+
			"POST /api/execute and was NOT refused: status=%d body=%q. The &news.* channel "+
			"pattern is not enforced on the HTTP path — handleExecute routes through "+
			"ExecuteHTTP, whose enforceACL must call CanAccessChannel on args[0].",
			got.Code, strings.TrimSpace(body))
	}
}

// The control for the control: the default (unauthenticated) identity is NOT
// confined to &news.*, so it may publish to the same forbidden channel. Without
// this, the refusal above could be the endpoint blocking private.* outright
// rather than the user's channel rule doing the work.
func TestHTTPExecuteDefaultUserMayPublishToAnyChannel(t *testing.T) {
	h := newHTTPPubSubServer(t)

	got := postExecuteIdentity(t, h, "", "PUBLISH", "private.other", "hello")
	body := strings.TrimSpace(got.Body.String())
	if strings.Contains(body, "NOPERM") {
		t.Fatalf("the default user was refused on private.other with %q — a channel the "+
			"restricted user must also be refused on, so this pair no longer distinguishes "+
			"channel enforcement from a blanket channel block", body)
	}
	// The publish must actually have been carried out, not merely unrefused:
	// a 404/501 for an unroutable command would also clear the check above.
	if !strings.Contains(body, "result") {
		t.Fatalf("default-user PUBLISH returned status %d body=%q, want a result reply — "+
			"an unroutable command would also clear the NOPERM check above", got.Code, body)
	}
}
