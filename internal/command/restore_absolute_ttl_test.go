package command

import (
	"bytes"
	"strconv"
	"testing"
	"time"

	"github.com/cachestorm/cachestorm/internal/resp"
	"github.com/cachestorm/cachestorm/internal/store"
)

func restoreRun(t *testing.T, s *store.Store, args ...string) *resp.Value {
	t.Helper()
	router := NewRouter()
	RegisterServerCommands(router)
	RegisterStringCommands(router)
	RegisterKeyCommands(router)

	var buf bytes.Buffer
	ctx := NewContext(args[0], restoreBytes(args[1:]), s, resp.NewWriter(&buf))
	if err := router.Execute(ctx); err != nil {
		t.Fatalf("%s: Execute returned error: %v", args[0], err)
	}
	v, err := resp.NewReader(bytes.NewReader(buf.Bytes())).ReadValue()
	if err != nil {
		t.Fatalf("%s: could not parse RESP reply %q: %v", args[0], buf.String(), err)
	}
	return v
}

func restoreBytes(ss []string) [][]byte {
	out := make([][]byte, len(ss))
	for i, s := range ss {
		out[i] = []byte(s)
	}
	return out
}

func restorePayload(t *testing.T, value string) string {
	t.Helper()
	s := store.NewStore()
	restoreRun(t, s, "SET", "src", value)
	v := restoreRun(t, s, "DUMP", "src")
	if v.Type != resp.TypeBulkString {
		t.Fatalf("DUMP returned type %v, want a bulk string", v.Type)
	}
	return string(v.Bulk)
}

// Redis defines RESTORE's ttl as an ABSOLUTE Unix timestamp in milliseconds
// (since 3.0), not a relative duration. cmdRESTORE read it as relative
// (`opts.TTL = time.Duration(ttl) * time.Millisecond`), so a past timestamp —
// which means "this key should already be gone" — instead revived the key with
// a brand-new lifetime.
func TestRestorePastAbsoluteTimestampIsAlreadyExpired(t *testing.T) {
	payload := restorePayload(t, "v")
	s := store.NewStore()

	past := time.Now().Add(-time.Hour).UnixMilli()
	restoreRun(t, s, "RESTORE", "k", strconv.FormatInt(past, 10), payload)

	if v := restoreRun(t, s, "EXISTS", "k"); v.Type != resp.TypeInteger || v.Int != 0 {
		t.Fatalf("EXISTS = %d, want 0: a past absolute ttl must not leave a visible key", v.Int)
	}
}

// A future absolute timestamp must expire AT that instant, not that many
// milliseconds from now (which would extend the key's life by decades).
func TestRestoreFutureAbsoluteTimestampIsNotExtended(t *testing.T) {
	payload := restorePayload(t, "v")
	s := store.NewStore()

	future := time.Now().Add(5 * time.Minute).UnixMilli()
	restoreRun(t, s, "RESTORE", "k", strconv.FormatInt(future, 10), payload)

	entry, ok := s.Get("k")
	if !ok {
		t.Fatal("setup: the key vanished entirely")
	}
	if entry.ExpiresAt == 0 {
		t.Fatal("setup: the key has no expiry, so this test proves nothing")
	}
	// Allow 2s of slack for clock granularity, but nothing like the ~94 years
	// a relative reading would produce.
	limit := time.UnixMilli(future).Add(2 * time.Second).UnixNano()
	if entry.ExpiresAt > limit {
		t.Fatalf("ExpiresAt = %d, want <= %d (the absolute timestamp passed in, plus slack)", entry.ExpiresAt, limit)
	}
}

// An absolute timestamp a few seconds out must still yield a live, visible key.
func TestRestoreNearFutureTimestampIsVisible(t *testing.T) {
	payload := restorePayload(t, "v")
	s := store.NewStore()

	soon := time.Now().Add(30 * time.Second).UnixMilli()
	restoreRun(t, s, "RESTORE", "k", strconv.FormatInt(soon, 10), payload)

	if v := restoreRun(t, s, "EXISTS", "k"); v.Int != 1 {
		t.Fatalf("EXISTS = %d, want 1 for a timestamp 30s in the future", v.Int)
	}
}

// Control: ttl 0 means "no expiry" in both readings, so it must stay permanent.
func TestRestoreZeroTtlIsPermanentControl(t *testing.T) {
	payload := restorePayload(t, "v")
	s := store.NewStore()

	restoreRun(t, s, "RESTORE", "k", "0", payload)

	entry, ok := s.Get("k")
	if !ok {
		t.Fatal("control: the key is missing")
	}
	if entry.ExpiresAt != 0 {
		t.Fatalf("control: ExpiresAt = %d, want 0 — ttl 0 means no expiry", entry.ExpiresAt)
	}
}

// Control: the VALUE survives the round trip — the defect is the ttl only.
func TestRestorePreservesValueControl(t *testing.T) {
	payload := restorePayload(t, "hello")
	s := store.NewStore()

	restoreRun(t, s, "RESTORE", "k", "0", payload)

	got := restoreRun(t, s, "GET", "k")
	if got.Type != resp.TypeBulkString || string(got.Bulk) != "hello" {
		t.Fatalf("control: GET returned type %v %q, want bulk \"hello\"", got.Type, got.Bulk)
	}
}

// Control: a past-timestamped key is gone, but REPLACE onto a fresh name still
// reports success — the fix must not turn RESTORE into an error path.
func TestRestorePastTimestampStillReturnsOK(t *testing.T) {
	payload := restorePayload(t, "v")
	s := store.NewStore()

	past := time.Now().Add(-time.Hour).UnixMilli()
	if v := restoreRun(t, s, "RESTORE", "k", strconv.FormatInt(past, 10), payload); v.Type == resp.TypeError {
		t.Fatalf("RESTORE with a past ttl returned %q, want +OK", v.Err)
	}
}

// Control: BUSYKEY is still returned when the key exists and REPLACE is absent.
func TestRestoreBusyKeyControl(t *testing.T) {
	payload := restorePayload(t, "v")
	s := store.NewStore()
	restoreRun(t, s, "SET", "k", "existing")

	if v := restoreRun(t, s, "RESTORE", "k", "0", payload); v.Type != resp.TypeError {
		t.Fatalf("control: RESTORE onto an existing key returned type %v, want an error", v.Type)
	}
}

// Control: REPLACE still overwrites.
func TestRestoreReplaceControl(t *testing.T) {
	payload := restorePayload(t, "fresh")
	s := store.NewStore()
	restoreRun(t, s, "SET", "k", "existing")

	if v := restoreRun(t, s, "RESTORE", "k", "0", payload, "REPLACE"); v.Type == resp.TypeError {
		t.Fatalf("control: RESTORE ... REPLACE returned %q", v.Err)
	}
	got := restoreRun(t, s, "GET", "k")
	if string(got.Bulk) != "fresh" {
		t.Fatalf("control: value = %q, want %q", got.Bulk, "fresh")
	}
}
