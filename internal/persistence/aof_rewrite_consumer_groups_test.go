package persistence_test

import (
	"bytes"
	"testing"

	"github.com/cachestorm/cachestorm/internal/command"
	"github.com/cachestorm/cachestorm/internal/persistence"
	"github.com/cachestorm/cachestorm/internal/resp"
	"github.com/cachestorm/cachestorm/internal/store"
)

// aofGroupAdapter flattens Store.GetAll() entries as *store.Entry, the shape
// production adapters use, so writeEntry takes its typed branch.
type aofGroupAdapter struct {
	entries map[string]*store.Entry
}

func (a *aofGroupAdapter) GetAll() map[string]interface{} {
	out := make(map[string]interface{}, len(a.entries))
	for k, v := range a.entries {
		out[k] = v
	}
	return out
}

func aofGroupRewriteAndReplay(t *testing.T, src *store.Store) *store.Store {
	t.Helper()
	dir := t.TempDir()
	aofPath := dir + "/appendonly.aof"
	rw := persistence.NewAOFRewriter(persistence.AOFConfig{DataDir: dir}, &aofGroupAdapter{entries: src.GetAll()})
	if err := rw.Rewrite(aofPath); err != nil {
		t.Fatalf("Rewrite failed: %v", err)
	}
	cmds, err := persistence.NewAOFReader().Load(aofPath)
	if err != nil {
		t.Fatalf("Load failed: %v", err)
	}

	dst := store.NewStore()
	router := command.NewRouter()
	command.RegisterStreamCommands(router)
	command.RegisterStringCommands(router)
	command.RegisterHashCommands(router)
	command.RegisterListCommands(router)
	command.RegisterSetCommands(router)
	command.RegisterSortedSetCommands(router)
	command.RegisterKeyCommands(router)

	var out bytes.Buffer
	w := resp.NewWriter(&out)
	for _, c := range cmds {
		if err := router.ExecuteSilent(command.NewContext(c.Name, c.Args, dst, w)); err != nil {
			t.Fatalf("replay %s failed: %v", c.Name, err)
		}
	}
	return dst
}

func aofGroupStream(t *testing.T, s *store.Store, key string) *store.StreamValue {
	t.Helper()
	entry, ok := s.Get(key)
	if !ok {
		t.Fatalf("key %q missing", key)
	}
	sv, ok := entry.Value.(*store.StreamValue)
	if !ok {
		t.Fatalf("key %q is %T, want *store.StreamValue", key, entry.Value)
	}
	return sv
}

// A rewrite must preserve a stream's consumer groups. The rewriter emitted
// XADD for every entry but no XGROUP, so after a rewrite + restart every group
// was gone: a client resuming its group got NOGROUP and the group's read
// position silently reset.
func TestAOFRewritePreservesConsumerGroups(t *testing.T) {
	src := store.NewStore()
	st := store.NewStreamValue(0)
	if _, err := st.Add("1-1", map[string][]byte{"f": []byte("v")}); err != nil {
		t.Fatalf("setup Add: %v", err)
	}
	if _, err := st.Add("2-2", map[string][]byte{"f": []byte("v")}); err != nil {
		t.Fatalf("setup Add: %v", err)
	}
	if err := st.CreateGroup("workers", "1-1"); err != nil {
		t.Fatalf("setup CreateGroup: %v", err)
	}
	src.Set("st", st, store.SetOptions{})

	if _, ok := st.Groups["workers"]; !ok {
		t.Fatal("setup: source stream has no consumer group")
	}

	dst := aofGroupRewriteAndReplay(t, src)
	replayed := aofGroupStream(t, dst, "st")

	if _, ok := replayed.Groups["workers"]; !ok {
		t.Fatalf("consumer group %q did not survive rewrite+replay; groups now: %v", "workers", replayed.Groups)
	}
}

// The group's read position must survive too, otherwise a restart would reset
// it to 0-0 and redeliver already-consumed entries.
func TestAOFRewritePreservesConsumerGroupPosition(t *testing.T) {
	src := store.NewStore()
	st := store.NewStreamValue(0)
	for _, id := range []string{"1-1", "2-2", "3-3"} {
		if _, err := st.Add(id, map[string][]byte{"f": []byte("v")}); err != nil {
			t.Fatalf("setup Add %s: %v", id, err)
		}
	}
	if err := st.CreateGroup("workers", "0-0"); err != nil {
		t.Fatalf("setup CreateGroup: %v", err)
	}
	src.Set("st", st, store.SetOptions{})

	// Advance the group to the second entry, as a real consumer would.
	if g := st.Groups["workers"]; g != nil {
		g.LastID = "2-2"
	}

	dst := aofGroupRewriteAndReplay(t, src)
	replayed := aofGroupStream(t, dst, "st")

	g, ok := replayed.Groups["workers"]
	if !ok {
		t.Fatal("consumer group did not survive rewrite+replay")
	}
	if g.LastID != "2-2" {
		t.Fatalf("group LastID after rewrite = %q, want %q — a reset position redelivers consumed entries", g.LastID, "2-2")
	}
}

// Several groups on one stream must all survive.
func TestAOFRewritePreservesMultipleConsumerGroups(t *testing.T) {
	src := store.NewStore()
	st := store.NewStreamValue(0)
	if _, err := st.Add("1-1", map[string][]byte{"f": []byte("v")}); err != nil {
		t.Fatalf("setup Add: %v", err)
	}
	for _, name := range []string{"a", "b", "c"} {
		if err := st.CreateGroup(name, "0-0"); err != nil {
			t.Fatalf("setup CreateGroup %s: %v", name, err)
		}
	}
	src.Set("st", st, store.SetOptions{})

	dst := aofGroupRewriteAndReplay(t, src)
	replayed := aofGroupStream(t, dst, "st")

	for _, name := range []string{"a", "b", "c"} {
		if _, ok := replayed.Groups[name]; !ok {
			t.Fatalf("group %q did not survive rewrite+replay", name)
		}
	}
}

// Control: the stream ENTRIES still survive — this isolates the defect to
// consumer groups rather than stream data.
func TestAOFRewritePreservesStreamEntriesControl(t *testing.T) {
	src := store.NewStore()
	st := store.NewStreamValue(0)
	if _, err := st.Add("1-1", map[string][]byte{"f": []byte("v")}); err != nil {
		t.Fatalf("setup Add: %v", err)
	}
	if _, err := st.Add("2-2", map[string][]byte{"f": []byte("g")}); err != nil {
		t.Fatalf("setup Add: %v", err)
	}
	src.Set("st", st, store.SetOptions{})

	dst := aofGroupRewriteAndReplay(t, src)
	replayed := aofGroupStream(t, dst, "st")

	if len(replayed.Entries) != 2 {
		t.Fatalf("stream has %d entries after rewrite, want 2", len(replayed.Entries))
	}
	if replayed.Entries[0].ID != "1-1" || replayed.Entries[1].ID != "2-2" {
		t.Fatalf("entry IDs = %q,%q, want 1-1,2-2", replayed.Entries[0].ID, replayed.Entries[1].ID)
	}
}

// Control: a stream with no consumer group must not gain one.
func TestAOFStreamWithoutGroupsStaysGrouplessControl(t *testing.T) {
	src := store.NewStore()
	st := store.NewStreamValue(0)
	if _, err := st.Add("1-1", map[string][]byte{"f": []byte("v")}); err != nil {
		t.Fatalf("setup Add: %v", err)
	}
	src.Set("st", st, store.SetOptions{})

	dst := aofGroupRewriteAndReplay(t, src)
	replayed := aofGroupStream(t, dst, "st")

	if len(replayed.Groups) != 0 {
		t.Fatalf("replayed stream gained %d groups, want 0", len(replayed.Groups))
	}
}

// The rewritten file must actually contain XGROUP CREATE, not merely happen to
// round-trip.
func TestAOFRewriteEmitsXGroupCreate(t *testing.T) {
	src := store.NewStore()
	st := store.NewStreamValue(0)
	if _, err := st.Add("1-1", map[string][]byte{"f": []byte("v")}); err != nil {
		t.Fatalf("setup Add: %v", err)
	}
	if err := st.CreateGroup("workers", "0-0"); err != nil {
		t.Fatalf("setup CreateGroup: %v", err)
	}
	src.Set("st", st, store.SetOptions{})

	dir := t.TempDir()
	aofPath := dir + "/appendonly.aof"
	rw := persistence.NewAOFRewriter(persistence.AOFConfig{DataDir: dir}, &aofGroupAdapter{entries: src.GetAll()})
	if err := rw.Rewrite(aofPath); err != nil {
		t.Fatalf("Rewrite failed: %v", err)
	}

	cmds, err := persistence.NewAOFReader().Load(aofPath)
	if err != nil {
		t.Fatalf("Load failed: %v", err)
	}

	seen := false
	for _, c := range cmds {
		if c.Name != "XGROUP" {
			continue
		}
		seen = true
		if len(c.Args) < 4 {
			t.Fatalf("XGROUP has %d args, want at least 4 (CREATE key group id): %v", len(c.Args), c.Args)
		}
		if !hasUpperPrefix(string(c.Args[0]), "CREATE") {
			t.Fatalf("XGROUP subcommand = %q, want CREATE", c.Args[0])
		}
	}
	if !seen {
		t.Fatal("the rewritten AOF contains no XGROUP command")
	}
}

func hasUpperPrefix(s, prefix string) bool {
	if len(s) < len(prefix) {
		return false
	}
	for i := 0; i < len(prefix); i++ {
		c := s[i]
		if c >= 'a' && c <= 'z' {
			c -= 32
		}
		if c != prefix[i] {
			return false
		}
	}
	return true
}
