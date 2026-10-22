package command

import (
	"strings"
	"testing"

	"github.com/cachestorm/cachestorm/internal/store"
)

// XRANGE accepts a trailing `COUNT n` option. The handler's option loop was
//
//	if strings.ToUpper(ctx.ArgString(i)) == "COUNT" && i+1 < ctx.ArgCount() {
//
// so a `COUNT` with no value failed the whole condition and fell through
// SILENTLY: count stayed 0, and GetRange treats count==0 as "no limit"
// (internal/store/stream.go:436-438). `XRANGE k - + COUNT` therefore returned
// the ENTIRE stream while the client believed it was constraining the result.
//
// The sibling cmdXAUTOCLAIM returns ErrSyntaxError for the identical input
// (its COUNT arm checks `i+1 >= ctx.ArgCount()`), so the two stream readers
// disagreed, and Redis rejects a dangling COUNT too.
func TestRegressionXrangeRejectsDanglingCount(t *testing.T) {
	s := store.NewStore()
	router := NewRouter()
	RegisterStreamCommands(router)

	runCmd(t, s, router, "XADD", "reg_xr", "1-1", "f", "1")
	runCmd(t, s, router, "XADD", "reg_xr", "2-2", "f", "2")

	// Control: a well-formed COUNT must still limit the reply. Passes before and
	// after the fix, so a broken harness cannot masquerade as the defect.
	out := runCmd(t, s, router, "XRANGE", "reg_xr", "-", "+", "COUNT", "1")
	if !strings.HasPrefix(out, "*1\r\n") {
		t.Fatalf("control failed: XRANGE ... COUNT 1 returned %q, want a 1-element array",
			strings.TrimSpace(out))
	}

	// A COUNT with no value is a syntax error, not a silent "no limit". Red
	// before the fix, which returned every entry.
	out = runCmd(t, s, router, "XRANGE", "reg_xr", "-", "+", "COUNT")
	if !strings.HasPrefix(out, "-") {
		t.Fatalf("FAIL: XRANGE with a dangling COUNT returned %q, want a syntax error — the "+
			"option was silently discarded, count stayed 0, and GetRange treats 0 as "+
			"unlimited, so the client received the whole stream while believing it had "+
			"constrained the result", strings.TrimSpace(out))
	}
}
