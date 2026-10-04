package command

import (
	"bytes"
	"testing"

	"github.com/cachestorm/cachestorm/internal/resp"
	"github.com/cachestorm/cachestorm/internal/store"
)

func migrateRun(t *testing.T, s *store.Store, args ...string) *resp.Value {
	t.Helper()
	router := NewRouter()
	RegisterClusterCommands(router)
	RegisterServerCommands(router)
	RegisterStringCommands(router)
	RegisterKeyCommands(router)

	var buf bytes.Buffer
	ctx := NewContext(args[0], migrateBytes(args[1:]), s, resp.NewWriter(&buf))
	if err := router.Execute(ctx); err != nil {
		t.Fatalf("%s: Execute returned error: %v", args[0], err)
	}
	v, err := resp.NewReader(bytes.NewReader(buf.Bytes())).ReadValue()
	if err != nil {
		t.Fatalf("%s: could not parse RESP reply %q: %v", args[0], buf.String(), err)
	}
	return v
}

func migrateBytes(ss []string) [][]byte {
	out := make([][]byte, len(ss))
	for i, s := range ss {
		out[i] = []byte(s)
	}
	return out
}

func migrateExists(t *testing.T, s *store.Store, key string) int64 {
	t.Helper()
	v := migrateRun(t, s, "EXISTS", key)
	if v.Type != resp.TypeInteger {
		t.Fatalf("EXISTS returned type %v, want an integer", v.Type)
	}
	return v.Int
}

// MIGRATE must either perform the transfer or refuse. Redis reports OK only
// once the key has actually moved to the destination AND — without COPY — been
// removed from this instance.
//
// cmdMIGRATE could never reach a destination: host, port and destinationDB were
// parsed and discarded, and the handler replied +OK anyway. A client that
// trusts OK (it is how a caller knows this node no longer owns the key) would
// go on reading a key that was never migrated anywhere.
func TestMigrateNeverReportsSuccessWithoutMigrating(t *testing.T) {
	s := store.NewStore()
	migrateRun(t, s, "SET", "k", "v")

	got := migrateRun(t, s, "MIGRATE", "127.0.0.1", "6379", "k", "0")

	if got.Type == resp.TypeSimpleString && got.Str == "OK" {
		if migrateExists(t, s, "k") == 1 {
			t.Fatalf("MIGRATE replied +%s but the key is still here: nothing was transferred.\n"+
				"Redis reports OK only once the key has moved and been removed from the source", got.Str)
		}
		t.Fatalf("MIGRATE replied +%s; with no transfer possible it must not claim success", got.Str)
	}
	if got.Type != resp.TypeError {
		t.Fatalf("MIGRATE returned type %v, want an explicit error rather than a false success", got.Type)
	}
}

// A refused MIGRATE must not consume the key: nothing moved, so the key stays.
func TestMigrateRefusalLeavesKeyIntact(t *testing.T) {
	s := store.NewStore()
	migrateRun(t, s, "SET", "k", "v")

	migrateRun(t, s, "MIGRATE", "127.0.0.1", "6379", "k", "0")

	if n := migrateExists(t, s, "k"); n != 1 {
		t.Fatalf("EXISTS = %d, want 1: a MIGRATE that transfers nothing must not drop the key", n)
	}
	got := migrateRun(t, s, "GET", "k")
	if got.Type != resp.TypeBulkString || string(got.Bulk) != "v" {
		t.Fatalf("GET returned type %v %q, want bulk \"v\"", got.Type, got.Bulk)
	}
}

// COPY must not turn the refusal into a silent local success either.
func TestMigrateCopyOptionStillRefuses(t *testing.T) {
	s := store.NewStore()
	migrateRun(t, s, "SET", "k", "v")

	got := migrateRun(t, s, "MIGRATE", "127.0.0.1", "6379", "k", "0", "COPY")
	if got.Type == resp.TypeSimpleString && got.Str == "OK" {
		t.Fatalf("MIGRATE ... COPY replied +%s, want an error", got.Str)
	}
	if n := migrateExists(t, s, "k"); n != 1 {
		t.Fatalf("EXISTS = %d, want 1", n)
	}
}

// Control: a missing key must still report NOKEY — the one thing the stub got
// right, and the fix must preserve it ahead of the refusal.
func TestMigrateMissingKeyReturnsNokeyControl(t *testing.T) {
	s := store.NewStore()

	got := migrateRun(t, s, "MIGRATE", "127.0.0.1", "6379", "absent", "0")
	if got.Type != resp.TypeBulkString || string(got.Bulk) != "NOKEY" {
		t.Fatalf("control: MIGRATE on a missing key returned type %v %q, want bulk \"NOKEY\"", got.Type, got.Bulk)
	}
}

// Control: argument validation is unchanged — too few args is still an error.
func TestMigrateWrongArgCountControl(t *testing.T) {
	s := store.NewStore()

	if v := migrateRun(t, s, "MIGRATE", "127.0.0.1", "6379"); v.Type != resp.TypeError {
		t.Fatalf("control: MIGRATE with 2 args returned type %v, want an error", v.Type)
	}
	if v := migrateRun(t, s, "MIGRATE"); v.Type != resp.TypeError {
		t.Fatalf("control: MIGRATE with no args returned type %v, want an error", v.Type)
	}
}

// Control: a non-numeric TIMEOUT value is still rejected, so option parsing
// keeps working after the change.
func TestMigrateBadTimeoutControl(t *testing.T) {
	s := store.NewStore()
	migrateRun(t, s, "SET", "k", "v")

	if v := migrateRun(t, s, "MIGRATE", "127.0.0.1", "6379", "k", "0", "TIMEOUT", "abc"); v.Type != resp.TypeError {
		t.Fatalf("control: MIGRATE ... TIMEOUT abc returned type %v, want an error", v.Type)
	}
}

// Control: the neighbouring cluster handshake commands still reply +OK — only
// MIGRATE is being changed.
func TestClusterHandshakeCommandsStillOkControl(t *testing.T) {
	s := store.NewStore()

	for _, cmd := range []string{"ASKING", "READONLY", "READWRITE"} {
		got := migrateRun(t, s, cmd)
		if got.Type != resp.TypeSimpleString || got.Str != "OK" {
			t.Fatalf("control: %s returned type %v %q, want +OK", cmd, got.Type, got.Str)
		}
	}
}
