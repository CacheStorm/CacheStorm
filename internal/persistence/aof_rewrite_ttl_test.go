package persistence_test

import (
	"bytes"
	"path/filepath"
	"testing"
	"time"

	"github.com/cachestorm/cachestorm/internal/command"
	"github.com/cachestorm/cachestorm/internal/persistence"
	"github.com/cachestorm/cachestorm/internal/resp"
	"github.com/cachestorm/cachestorm/internal/store"
)

// aofTTLRewriteAdapter flattens Store.GetAll() entries as *store.Entry, the
// shape production adapters use, so writeEntry takes its typed branch.
type aofTTLRewriteAdapter struct {
	entries map[string]*store.Entry
}

func (a *aofTTLRewriteAdapter) GetAll() map[string]interface{} {
	out := make(map[string]interface{}, len(a.entries))
	for k, v := range a.entries {
		out[k] = v
	}
	return out
}

// rewriteTTLAndReplay rewrites src to a temp AOF and replays it into a fresh
// store, exactly as a server restart would.
func rewriteTTLAndReplay(t *testing.T, src *store.Store) *store.Store {
	t.Helper()

	dir := t.TempDir()
	aofPath := filepath.Join(dir, "appendonly.aof")
	rw := persistence.NewAOFRewriter(persistence.AOFConfig{DataDir: dir}, &aofTTLRewriteAdapter{entries: src.GetAll()})
	if err := rw.Rewrite(aofPath); err != nil {
		t.Fatalf("Rewrite failed: %v", err)
	}

	commands, err := persistence.NewAOFReader().Load(aofPath)
	if err != nil {
		t.Fatalf("Load failed: %v", err)
	}

	dst := store.NewStore()
	router := command.NewRouter()
	command.RegisterStringCommands(router)
	command.RegisterHashCommands(router)
	command.RegisterListCommands(router)
	command.RegisterSetCommands(router)
	command.RegisterSortedSetCommands(router)
	// PEXPIREAT lives in RegisterKeyCommands, not RegisterServerCommands.
	command.RegisterKeyCommands(router)

	for _, c := range commands {
		var respBuf bytes.Buffer
		if err := router.ExecuteSilent(command.NewContext(c.Name, c.Args, dst, resp.NewWriter(&respBuf))); err != nil {
			t.Fatalf("replay %s failed: %v", c.Name, err)
		}
	}
	return dst
}

// A rewrite must preserve each key's expiry. The rewriter emitted only the
// value commands and no expiry command, so every TTL was silently dropped and
// a time-limited key came back PERMANENT after a rewrite.
func TestAOFRewritePreservesTTL(t *testing.T) {
	src := store.NewStore()
	if err := src.Set("ephemeral", &store.StringValue{Data: []byte("v")}, store.SetOptions{TTL: 3600 * time.Second}); err != nil {
		t.Fatalf("setup Set: %v", err)
	}

	// Control: the source really does carry a TTL before the rewrite.
	if ttl := src.TTL("ephemeral"); ttl <= 0 {
		t.Fatalf("setup: source TTL = %v, want a live TTL", ttl)
	}

	dst := rewriteTTLAndReplay(t, src)

	if _, ok := dst.Get("ephemeral"); !ok {
		t.Fatal("the key vanished entirely across rewrite+replay")
	}

	ttl := dst.TTL("ephemeral")
	if ttl <= 0 {
		t.Fatalf("TTL after rewrite+replay = %v, want a live TTL: a rewrite silently turned an expiring key permanent", ttl)
	}
	if ttl > 3600*time.Second {
		t.Fatalf("TTL after rewrite = %v, want <= the original 3600s", ttl)
	}
}

// Boundary: a very short TTL is still positive after the ns->ms conversion
// PEXPIREAT requires, and it must not be rounded away to "already expired".
func TestAOFRewritePreservesShortTTLBoudary(t *testing.T) {
	src := store.NewStore()
	if err := src.Set("brief", &store.StringValue{Data: []byte("v")}, store.SetOptions{TTL: 2 * time.Second}); err != nil {
		t.Fatalf("setup Set: %v", err)
	}

	dst := rewriteTTLAndReplay(t, src)

	ttl := dst.TTL("brief")
	if ttl <= 0 || ttl > 2*time.Second {
		t.Fatalf("TTL after rewrite = %v, want a live TTL within the original 2s", ttl)
	}
}

// Control: values themselves survive the rewrite — this is about the dropped
// expiry, not about losing data.
func TestAOFRewritePreservesValuesWithTTLControl(t *testing.T) {
	src := store.NewStore()
	if err := src.Set("plain", &store.StringValue{Data: []byte("hello")}, store.SetOptions{}); err != nil {
		t.Fatalf("setup Set: %v", err)
	}
	if err := src.Set("tagged", &store.HashValue{Fields: map[string][]byte{"f": []byte("v")}},
		store.SetOptions{TTL: 3600 * time.Second}); err != nil {
		t.Fatalf("setup Set hash: %v", err)
	}

	dst := rewriteTTLAndReplay(t, src)

	entry, ok := dst.Get("plain")
	if !ok {
		t.Fatal("control: key \"plain\" missing after rewrite+replay")
	}
	sv, ok := entry.Value.(*store.StringValue)
	if !ok {
		t.Fatalf("control: value type = %T, want *store.StringValue", entry.Value)
	}
	if string(sv.Data) != "hello" {
		t.Fatalf("control: value = %q, want %q", sv.Data, "hello")
	}

	hashEntry, ok := dst.Get("tagged")
	if !ok {
		t.Fatal("control: hash key missing after rewrite+replay")
	}
	if _, ok := hashEntry.Value.(*store.HashValue); !ok {
		t.Fatalf("control: hash value type = %T, want *store.HashValue", hashEntry.Value)
	}
	if dst.TTL("tagged") <= 0 {
		t.Fatalf("control: hash TTL after rewrite = %v, want a live TTL", dst.TTL("tagged"))
	}
}

// Control: a key that had NO expiry must not gain one, and must not be given a
// PEXPIREAT at all.
func TestAOFNoTTLStaysPermanentControl(t *testing.T) {
	src := store.NewStore()
	if err := src.Set("forever", &store.StringValue{Data: []byte("v")}, store.SetOptions{}); err != nil {
		t.Fatalf("setup Set: %v", err)
	}

	dst := rewriteTTLAndReplay(t, src)

	entry, ok := dst.Get("forever")
	if !ok {
		t.Fatal("control: key \"forever\" missing after rewrite+replay")
	}
	if entry.ExpiresAt != 0 {
		t.Fatalf("control: ExpiresAt = %d, want 0 — a key with no TTL must not gain one", entry.ExpiresAt)
	}
}

// The rewritten file must carry the expiry as a replayable PEXPIREAT rather
// than silently relying on the value commands alone.
func TestAOFRewriteEmitsExpireCommand(t *testing.T) {
	src := store.NewStore()
	if err := src.Set("expiring", &store.StringValue{Data: []byte("v")}, store.SetOptions{TTL: 3600 * time.Second}); err != nil {
		t.Fatalf("setup Set: %v", err)
	}

	dir := t.TempDir()
	aofPath := filepath.Join(dir, "appendonly.aof")
	rw := persistence.NewAOFRewriter(persistence.AOFConfig{DataDir: dir}, &aofTTLRewriteAdapter{entries: src.GetAll()})
	if err := rw.Rewrite(aofPath); err != nil {
		t.Fatalf("Rewrite failed: %v", err)
	}

	commands, err := persistence.NewAOFReader().Load(aofPath)
	if err != nil {
		t.Fatalf("Load failed: %v", err)
	}

	seen := false
	for _, c := range commands {
		if c.Name == "PEXPIREAT" {
			seen = true
			if len(c.Args) != 2 {
				t.Fatalf("PEXPIREAT has %d args, want 2 (key, ms): %v", len(c.Args), c.Args)
			}
			if string(c.Args[0]) != "expiring" {
				t.Fatalf("PEXPIREAT targets %q, want %q", c.Args[0], "expiring")
			}
		}
	}
	if !seen {
		t.Fatalf("the rewritten AOF contains no PEXPIREAT; commands were %v", commandNames(commands))
	}
}

func commandNames(cmds []persistence.Command) []string {
	out := make([]string, len(cmds))
	for i, c := range cmds {
		out[i] = c.Name
	}
	return out
}
