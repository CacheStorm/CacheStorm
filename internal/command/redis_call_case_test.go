package command

import (
	"bytes"
	"strings"
	"testing"

	"github.com/cachestorm/cachestorm/internal/resp"
	"github.com/cachestorm/cachestorm/internal/store"
)

// evalLua runs a script through the real EVAL handler and returns the reply.
func evalLua(s *store.Store, r *Router, script string, args ...string) string {
	handler, ok := r.Get("EVAL")
	if !ok {
		return "ERR HARNESS: EVAL not registered"
	}
	raw := make([][]byte, 0, 2+len(args))
	raw = append(raw, []byte(script), []byte("0"))
	for _, a := range args {
		raw = append(raw, []byte(a))
	}
	var buf bytes.Buffer
	ctx := NewContext("EVAL", raw, s, resp.NewWriter(&buf))
	if err := handler.Handler(ctx); err != nil {
		return "ERR HARNESS: " + err.Error()
	}
	return buf.String()
}

// TestProofRedisCallIsCaseInsensitive is the round proof.
//
// Contract (Redis): command names are CASE-INSENSITIVE. `redis.call('get', k)`
// and `redis.call('GET', k)` are the same call, and every Lua script in the
// wild spells commands in lowercase — that is the conventional style.
//
// Defect: ScriptEngine.executeCommand dispatches on `switch cmd` with
// UPPERCASE case labels only ("GET", "SET", "INCR", ...) and normalises the
// name nowhere — there is no strings.ToUpper on the path from redis.call to the
// switch. A lowercase name therefore matched no arm, fell out of the switch
// (which has no default), and returned nil while doing NOTHING.
//
// The damage is worse than a wrong return value: a mutating command called in
// lowercase silently did not mutate. `redis.call('del', k)` left the key alive
// and returned nil; `redis.call('incr', k)` left the counter unchanged and
// returned nil — so a script's arithmetic, deletes and increments all quietly
// no-op while the script itself reports success and EXEC replies +OK.
//
// IN-REPO BASIS: every other command entry point in this codebase is already
// case-insensitive — Router.Execute does upperCmd := strings.ToUpper(...),
// and the other redis.* bindings (pcall) route through the same helper. Only
// this switch is case-sensitive.
//
// Controls (must pass before AND after): the UPPERCASE spelling keeps working
// for read, write and arithmetic; a genuinely unknown command fails loudly
// with an ERR reply (silent nil until round r37); and EVALSHA reaches the same
// engine so it is fixed too.
func TestProofRedisCallIsCaseInsensitive(t *testing.T) {
	s := store.NewStore()
	r := NewRouter()
	RegisterStringCommands(r)
	RegisterScriptCommands(r)

	// ---- CONTROL 1: UPPERCASE works for a read, a write and an arithmetic op.
	// The Lua engine returns SET's "OK" as a bulk string, not a simple string.
	if got := evalLua(s, r, "return redis.call('SET','k','7')"); got != "$2\r\nOK\r\n" {
		t.Fatalf("CONTROL 1 broken harness: uppercase SET = %q, want \"$2\\r\\nOK\\r\\n\"", got)
	}
	if got := evalLua(s, r, "return redis.call('GET','k')"); got != "$1\r\n7\r\n" {
		t.Fatalf("CONTROL 1 broken harness: uppercase GET = %q, want \"$1\\r\\n7\\r\\n\"", got)
	}
	if got := evalLua(s, r, "return redis.call('INCR','k')"); got != ":8\r\n" {
		t.Fatalf("CONTROL 1 broken harness: uppercase INCR = %q, want \":8\\r\\n\"", got)
	}
	if got := evalLua(s, r, "return redis.call('TYPE','k')"); !strings.Contains(got, "string") {
		t.Fatalf("CONTROL 1 broken harness: uppercase TYPE = %q, want \"string\"", got)
	}
	t.Log("CONTROL 1 ok: uppercase spellings all work")

	// ---- CONTROL 2: a genuinely unknown command fails loudly. Round r37
	// changed the switch's default arm from a silent lua.LNil to an explicit
	// ERR string, matching Redis where an unknown command called from a
	// script is an error — the old nil contract was the silent no-op.
	if got := evalLua(s, r, "return redis.call('NOSUCHCOMMAND','k')"); !strings.Contains(got, "ERR unknown command") {
		t.Fatalf("CONTROL 2 broken harness: unknown command = %q, want an \"ERR unknown command\" reply", got)
	}
	t.Log("CONTROL 2 ok: an unknown command fails loudly")

	// ---- THE DEFECT: lowercase is a READ. It must return the value.
	if got := evalLua(s, r, "return redis.call('get','k')"); got != "$1\r\n8\r\n" {
		t.Fatalf("FAIL: redis.call('get','k') returned %q, want \"$1\\r\\n8\\r\\n\" — Redis command "+
			"names are case-insensitive and lowercase is the conventional Lua spelling, but "+
			"executeCommand switches on UPPERCASE labels with no normalisation, so the "+
			"lowercase name matched no arm and fell out of the switch as nil.",
			bytes.TrimSpace([]byte(got)))
	}
	t.Log("PASS: lowercase 'get' now reads like 'GET'")

	// ---- THE DEFECT, MUTATING: lowercase 'incr' must increment the store.
	// This is the severe form: before the fix it returned nil AND left the
	// counter untouched, so a script's arithmetic silently no-opped.
	evalLua(s, r, "return redis.call('SET','n','5')")
	got := evalLua(s, r, "return redis.call('incr','n')")
	if got != ":6\r\n" {
		t.Fatalf("FAIL: redis.call('incr','n') returned %q, want \":6\\r\\n\" — before the fix this "+
			"returned nil and left n at 5, so the increment silently vanished",
			bytes.TrimSpace([]byte(got)))
	}
	if stored := evalLua(s, r, "return redis.call('GET','n')"); stored != "$1\r\n6\r\n" {
		t.Fatalf("FAIL: after redis.call('incr','n') the store holds %q, want \"$1\\r\\n6\\r\\n\" — "+
			"the mutation must actually be applied", bytes.TrimSpace([]byte(stored)))
	}
	t.Log("PASS: lowercase 'incr' increments the store")

	// ---- THE DEFECT, DELETING: lowercase 'del' must remove the key.
	evalLua(s, r, "return redis.call('SET','d','abc')")
	got = evalLua(s, r, "return redis.call('del','d')")
	if got != ":1\r\n" {
		t.Fatalf("FAIL: redis.call('del','d') returned %q, want \":1\\r\\n\" — before the fix the key "+
			"survived a 'del' the script believed it performed", bytes.TrimSpace([]byte(got)))
	}
	if survived := evalLua(s, r, "return redis.call('GET','d')"); survived != "_\r\n" {
		t.Fatalf("FAIL: key 'd' still exists after redis.call('del','d') (GET returned %q)", survived)
	}
	t.Log("PASS: lowercase 'del' removes the key")

	// ---- BOUNDARY: mixed case must work too, not just all-lower or all-upper.
	if got := evalLua(s, r, "return redis.call('GeT','n')"); got != "$1\r\n6\r\n" {
		t.Fatalf("BOUNDARY FAIL: redis.call('GeT','n') = %q, want \"$1\\r\\n6\\r\\n\" — case "+
			"insensitivity is total, not a lower/upper special case", bytes.TrimSpace([]byte(got)))
	}
	t.Log("PASS: mixed-case spelling works too")

	// ---- The fix lives in the shared helper, so EVALSHA is covered as well.
	// Load a script by SHA and run the same lowercase call through it.
	loadHandler, ok := r.Get("SCRIPT")
	if !ok {
		t.Fatalf("SCRIPT not registered")
	}
	var buf bytes.Buffer
	script := "return redis.call('set','shak','9')"
	loadCtx := NewContext("SCRIPT", bytesArgs("LOAD", script), s, resp.NewWriter(&buf))
	if err := loadHandler.Handler(loadCtx); err != nil {
		t.Fatalf("SCRIPT LOAD failed: %v", err)
	}
	// SCRIPT LOAD replies with a BULK STRING: "$40\r\n<sha>\r\n". Strip that
	// header instead of mistaking it for part of the digest.
	raw := buf.String()
	sha := ""
	if len(raw) > 4 && raw[0] == '$' {
		if idx := strings.Index(raw, "\r\n"); idx >= 0 {
			sha = raw[idx+2:]
		}
	}
	sha = strings.TrimSpace(sha)
	if len(sha) != 40 {
		t.Fatalf("SCRIPT LOAD harness broken: parsed SHA %q from %q, want 40 hex chars", sha, raw)
	}

	evalHandler, _ := r.Get("EVALSHA")
	var buf2 bytes.Buffer
	evalCtx := NewContext("EVALSHA", bytesArgs(sha, "0"), s, resp.NewWriter(&buf2))
	if err := evalHandler.Handler(evalCtx); err != nil {
		t.Fatalf("EVALSHA failed: %v", err)
	}
	if stored := evalLua(s, r, "return redis.call('GET','shak')"); stored != "$1\r\n9\r\n" {
		t.Fatalf("FAIL: the lowercase call through EVALSHA returned %q, want \"$1\\r\\n9\\r\\n\" — "+
			"the normalisation must live in the shared engine, not in EVAL alone",
			bytes.TrimSpace([]byte(stored)))
	}
	t.Log("PASS: EVALSHA reaches the same fixed engine")
}