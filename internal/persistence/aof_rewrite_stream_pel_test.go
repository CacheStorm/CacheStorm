package persistence_test

import (
	"bytes"
	"testing"

	"github.com/cachestorm/cachestorm/internal/command"
	"github.com/cachestorm/cachestorm/internal/resp"
	"github.com/cachestorm/cachestorm/internal/store"
)

func streamPELRun(t *testing.T, s *store.Store, args ...string) string {
	t.Helper()
	router := command.NewRouter()
	command.RegisterStreamCommands(router)
	raw := make([][]byte, len(args)-1)
	for i, a := range args[1:] {
		raw[i] = []byte(a)
	}
	var out bytes.Buffer
	if err := router.Execute(command.NewContext(args[0], raw, s, resp.NewWriter(&out))); err != nil {
		t.Fatalf("%v: %v", args, err)
	}
	return out.String()
}

// A rewrite must keep what XINFO and XPENDING report: field order, the
// entries-added counter, each group's entries-read counter, its consumers and
// its pending entries with their owners and delivery counts.
func TestAOFRewritePreservesStreamGroupState(t *testing.T) {
	src := store.NewStore()
	for _, args := range [][]string{
		{"XADD", "s", "1-0", "zeta", "1", "alpha", "2"},
		{"XADD", "s", "2-0", "b", "2"},
		{"XADD", "s", "3-0", "c", "3"},
		{"XADD", "s", "4-0", "d", "4"},
		{"XGROUP", "CREATE", "s", "g", "0"},
		{"XREADGROUP", "GROUP", "g", "alice", "COUNT", "2", "STREAMS", "s", ">"},
		{"XREADGROUP", "GROUP", "g", "bob", "COUNT", "1", "STREAMS", "s", ">"},
		{"XREADGROUP", "GROUP", "g", "alice", "STREAMS", "s", "0"},
		{"XGROUP", "CREATECONSUMER", "s", "g", "idle"},
		{"XDEL", "s", "4-0"},
	} {
		if out := streamPELRun(t, src, args...); out[0] == '-' {
			t.Fatalf("setup %v: %s", args, out)
		}
	}

	dst := aofGroupRewriteAndReplay(t, src)

	for _, probe := range [][]string{
		{"XRANGE", "s", "-", "+"},
		{"XINFO", "GROUPS", "s"},
		{"XPENDING", "s", "g"},
	} {
		want, got := streamPELRun(t, src, probe...), streamPELRun(t, dst, probe...)
		if want != got {
			t.Errorf("%v after rewrite:\n got  %q\n want %q", probe, got, want)
		}
	}

	info := func(s *store.Store) store.StreamInfo { return aofGroupStream(t, s, "s").Info(false, 0) }
	if w, g := info(src), info(dst); w.EntriesAdded != g.EntriesAdded || w.MaxDeletedID != g.MaxDeletedID || w.LastID != g.LastID {
		t.Errorf("stream counters after rewrite = %+v, want %+v", g, w)
	}

	wantRows := aofGroupStream(t, src, "s")
	gotRows := aofGroupStream(t, dst, "s")
	w, _ := wantRows.PendingRange("g", store.StreamID{}, store.MaxStreamID, 10, 0, "", false, 0)
	g, _ := gotRows.PendingRange("g", store.StreamID{}, store.MaxStreamID, 10, 0, "", false, 0)
	if len(w) != len(g) {
		t.Fatalf("pending after rewrite = %+v, want %+v", g, w)
	}
	for i := range w {
		if w[i].ID != g[i].ID || w[i].Consumer != g[i].Consumer || w[i].Deliveries != g[i].Deliveries || w[i].DeliveryTS != g[i].DeliveryTS {
			t.Errorf("pending row %d after rewrite = %+v, want %+v", i, g[i], w[i])
		}
	}
	consumers, _ := gotRows.ConsumersInfo("g")
	if len(consumers) != 3 {
		t.Errorf("consumers after rewrite = %+v, want alice, bob and idle", consumers)
	}
}
