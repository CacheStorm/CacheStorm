package command

import (
	"bytes"
	"testing"

	"github.com/cachestorm/cachestorm/internal/resp"
	"github.com/cachestorm/cachestorm/internal/store"
)

func psyncReply(t *testing.T, s *store.Store, args ...string) *resp.Value {
	t.Helper()
	router := NewRouter()
	RegisterReplicationCommands(router)
	RegisterStringCommands(router)

	var buf bytes.Buffer
	ctx := NewContext(args[0], psyncTestBytes(args[1:]), s, resp.NewWriter(&buf))
	if err := router.Execute(ctx); err != nil {
		t.Fatalf("%s: Execute returned error: %v", args[0], err)
	}
	v, err := resp.NewReader(bytes.NewReader(buf.Bytes())).ReadValue()
	if err != nil {
		t.Fatalf("%s: could not parse RESP reply %q: %v", args[0], buf.String(), err)
	}
	return v
}

func psyncTestBytes(ss []string) [][]byte {
	out := make([][]byte, len(ss))
	for i, s := range ss {
		out[i] = []byte(s)
	}
	return out
}

func hasPrefix(s, prefix string) bool {
	return len(s) >= len(prefix) && s[:len(prefix)] == prefix
}

// A master may answer PSYNC with +CONTINUE only when it can actually serve the
// backlog the resuming replica is missing. This manager keeps no backlog, so it
// replied +CONTINUE and then sent nothing — leaving the replica believing it was
// in sync while silently missing every write since its offset. It must fall
// back to a full resync, which at least carries a loadable snapshot.
func TestPsyncNeverPromisesBacklogItCannotServe(t *testing.T) {
	s := store.NewStore()
	InitReplicationManager(s)
	replID := GetReplicationManager().GetReplicaID()
	if replID == "" {
		t.Fatal("setup: replication manager has no replica id")
	}

	got := psyncReply(t, s, "PSYNC", replID, "0")
	if got.Type == resp.TypeSimpleString && hasPrefix(got.Str, "CONTINUE") {
		t.Fatalf("PSYNC %s 0 replied +%s, but no backlog can be served; want +FULLRESYNC", replID, got.Str)
	}
	if got.Type != resp.TypeSimpleString || !hasPrefix(got.Str, "FULLRESYNC") {
		t.Fatalf("PSYNC %s 0 returned type %v %q, want a +FULLRESYNC simple string", replID, got.Type, got.Str)
	}
}

// Any replid/offset combination must full-resync, including ones that used to
// be parsed only to be discarded.
func TestPsyncAlwaysFullResyncs(t *testing.T) {
	s := store.NewStore()
	InitReplicationManager(s)
	replID := GetReplicationManager().GetReplicaID()

	for _, tc := range []struct {
		name string
		args []string
	}{
		{"unknown replid", []string{"PSYNC", "0000000000000000000000000000000000000000", "0"}},
		{"matching replid, negative offset", []string{"PSYNC", replID, "-1"}},
		{"matching replid, large offset", []string{"PSYNC", replID, "999999"}},
		{"non-numeric offset", []string{"PSYNC", replID, "not-a-number"}},
		{"no args", []string{"PSYNC"}},
		{"replid only", []string{"PSYNC", replID}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got := psyncReply(t, s, tc.args...)
			if got.Type != resp.TypeSimpleString || !hasPrefix(got.Str, "FULLRESYNC") {
				t.Fatalf("%v returned type %v %q, want a +FULLRESYNC simple string", tc.args, got.Type, got.Str)
			}
		})
	}
}

// A full resync must still carry a snapshot after the +FULLRESYNC line, or the
// replica has nothing to load.
func TestPsyncFullResyncCarriesSnapshot(t *testing.T) {
	s := store.NewStore()
	InitReplicationManager(s)
	if err := s.Set("k", &store.StringValue{Data: []byte("v")}, store.SetOptions{}); err != nil {
		t.Fatalf("setup Set: %v", err)
	}

	router := NewRouter()
	RegisterReplicationCommands(router)
	RegisterStringCommands(router)

	var buf bytes.Buffer
	ctx := NewContext("PSYNC", psyncTestBytes([]string{GetReplicationManager().GetReplicaID(), "0"}), s, resp.NewWriter(&buf))
	if err := router.Execute(ctx); err != nil {
		t.Fatalf("PSYNC: Execute returned error: %v", err)
	}

	reader := resp.NewReader(bytes.NewReader(buf.Bytes()))
	first, err := reader.ReadValue()
	if err != nil {
		t.Fatalf("reading the first reply: %v", err)
	}
	if first.Type != resp.TypeSimpleString || !hasPrefix(first.Str, "FULLRESYNC") {
		t.Fatalf("first reply = type %v %q, want +FULLRESYNC", first.Type, first.Str)
	}

	second, err := reader.ReadValue()
	if err != nil {
		t.Fatalf("a +FULLRESYNC must be followed by an RDB snapshot: %v", err)
	}
	if second.Type != resp.TypeBulkString {
		t.Fatalf("snapshot reply type = %v, want a bulk string", second.Type)
	}
	if len(second.Bulk) == 0 {
		t.Fatal("the RDB snapshot is empty")
	}
}
