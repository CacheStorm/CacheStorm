package command

import (
	"bytes"
	"testing"

	"github.com/cachestorm/cachestorm/internal/resp"
	"github.com/cachestorm/cachestorm/internal/store"
)

// execTx runs one command through the real Router. The Context is SHARED by the
// caller because MULTI/EXEC state lives on ctx.Transaction and the QUEUE
// interception happens in Router.Execute — a fresh Context per command (as a
// connection does NOT do) silently loses the transaction.
func execTx(ctx *Context, r *Router, cmd string, args ...string) string {
	raw := make([][]byte, 0, len(args))
	for _, v := range args {
		raw = append(raw, []byte(v))
	}
	var buf bytes.Buffer
	ctx.Command = cmd
	ctx.Args = raw
	ctx.Writer = resp.NewWriter(&buf)
	if err := r.Execute(ctx); err != nil {
		return "ERR HARNESS: " + err.Error()
	}
	return buf.String()
}

// replayTx returns the reply EXEC produced for a single queued command,
// stripping the outer array header.
func replayTx(ctx *Context, r *Router, cmd string, args ...string) string {
	execTx(ctx, r, "MULTI")
	execTx(ctx, r, cmd, args...)
	out := execTx(ctx, r, "EXEC")
	if len(out) > 4 && out[0] == '*' {
		return out[4:]
	}
	return out
}

// TestProofExecReplayGetDistinguishesWrongTypeFromMissing is the round proof.
//
// Contract: a command's behaviour must not change merely because it was
// queued in a transaction. Directly, CacheStorm answers GET on a non-string key
// with WRONGTYPE; replayed through EXEC it answered a NULL — reporting a
// non-string key as absent.
//
// Defect: executeQueuedCommand's "GET" case did
//
//	if entry, exists := ctx.Store.Get(key); exists {
//	    if sv, ok := entry.Value.(*store.StringValue); ok { return BulkBytes(sv.Data) }
//	}
//	return resp.NullBulkString()
//
// The type assertion failing fell through to the SAME return as a missing key,
// so "wrong type" and "no such key" became indistinguishable. IN-REPO BASIS: the
// neighbouring replay cases get this right — the "APPEND" and "STRLEN" cases
// both return WRONGTYPE explicitly — so GET was the lone outlier.
//
// A second, independent defect sits in the same assertion: it accepted only
// *store.StringValue, so a key holding bitmap bits (a second Go representation
// of the same Redis string type, which cmdGET serves via stringValueOf) also
// fell through to NULL — losing real data inside a transaction.
//
// Controls (must pass before AND after): a real string key round-trips, a
// missing key stays NULL, and the sibling replay cases already report
// WRONGTYPE.
func TestProofExecReplayGetDistinguishesWrongTypeFromMissing(t *testing.T) {
	s := store.NewStore()
	r := NewRouter()
	RegisterStringCommands(r)
	RegisterListCommands(r)
	RegisterHashCommands(r)
	RegisterBitmapCommands(r)
	RegisterTransactionCommands(r)

	ctx := NewContext("", nil, s, resp.NewWriter(&bytes.Buffer{}))

	execTx(ctx, r, "RPUSH", "alist", "x")
	execTx(ctx, r, "HSET", "ahash", "f", "v")
	execTx(ctx, r, "SETBIT", "abmp", "0", "1")
	execTx(ctx, r, "SET", "astr", "hello")

	// ---- CONTROL 1: a real string key behaves identically direct and replayed.
	if got := replayTx(ctx, r, "GET", "astr"); got != "$5\r\nhello\r\n" {
		t.Fatalf("CONTROL 1 broken harness: replayed GET astr = %q, want \"$5\\r\\nhello\\r\\n\"", got)
	}
	t.Log("CONTROL 1 ok: a string key round-trips through EXEC")

	// ---- CONTROL 2: a missing key is NULL both ways.
	if got := execTx(ctx, r, "GET", "nokey"); got != "$-1\r\n" {
		t.Fatalf("CONTROL 2 broken harness: direct GET nokey = %q, want nil", got)
	}
	if got := replayTx(ctx, r, "GET", "nokey"); got != "$-1\r\n" {
		t.Fatalf("CONTROL 2 broken harness: replayed GET nokey = %q, want nil", got)
	}
	t.Log("CONTROL 2 ok: a missing key stays NULL")

	// ---- CONTROL 3: the sibling replay case already reports WRONGTYPE, which
	// is the in-repo basis that GET is the outlier rather than the rule.
	stLenReplay := replayTx(ctx, r, "STRLEN", "alist")
	if !bytes.Contains([]byte(stLenReplay), []byte("WRONGTYPE")) {
		t.Fatalf("CONTROL 3 broken harness: replayed STRLEN on a list = %q, want WRONGTYPE", stLenReplay)
	}
	t.Log("CONTROL 3 ok: replayed STRLEN already answers WRONGTYPE")

	// ---- THE DEFECT: a list must be WRONGTYPE, not NULL, when replayed.
	direct := execTx(ctx, r, "GET", "alist")
	replayed := replayTx(ctx, r, "GET", "alist")
	if !bytes.Contains([]byte(direct), []byte("WRONGTYPE")) {
		t.Fatalf("DEFECT setup: direct GET on a list = %q, want WRONGTYPE", direct)
	}
	if replayed != direct {
		t.Fatalf("FAIL: GET inside a transaction changed behaviour. Direct GET alist = %q but "+
			"MULTI/GET/EXEC returned %q. executeQueuedCommand's GET case falls through to "+
			"NullBulkString when the type assertion fails, so a WRONGTYPE key is reported as "+
			"absent — a transaction must not turn a type error into 'no such key'.",
			bytes.TrimSpace([]byte(direct)), bytes.TrimSpace([]byte(replayed)))
	}
	t.Log("PASS: a list reports WRONGTYPE inside a transaction too")

	// ---- Same for a hash: any non-string type, not just lists.
	replayedHash := replayTx(ctx, r, "GET", "ahash")
	if !bytes.Contains([]byte(replayedHash), []byte("WRONGTYPE")) {
		t.Fatalf("FAIL: replayed GET on a hash = %q, want WRONGTYPE — every non-string type "+
			"must be rejected, not reported as missing", bytes.TrimSpace([]byte(replayedHash)))
	}
	t.Log("PASS: a hash reports WRONGTYPE inside a transaction too")

	// ---- SECOND DEFECT, same assertion: bitmap bits are real data and must
	// not vanish inside a transaction.
	replayedBmp := replayTx(ctx, r, "GET", "abmp")
	directBmp := execTx(ctx, r, "GET", "abmp")
	if directBmp != "$1\r\n\x80\r\n" {
		t.Fatalf("SECONDARY setup: direct GET on a bitmap key = %q, want the byte 0x80", directBmp)
	}
	if replayedBmp != directBmp {
		t.Fatalf("FAIL: GET inside a transaction LOST real data. Direct GET abmp = %q but "+
			"MULTI/GET/EXEC returned %q. The replay GET accepts only *store.StringValue, so a "+
			"key holding bitmap bits — which cmdGET serves — reads back as nil.",
			bytes.TrimSpace([]byte(directBmp)), bytes.TrimSpace([]byte(replayedBmp)))
	}
	t.Log("PASS: bitmap bytes survive the transaction")
}
