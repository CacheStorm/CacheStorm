package store

import (
	"fmt"
	"testing"
)

func addN(t *testing.T, v *StreamValue, n int, wide bool) {
	t.Helper()
	for i := 1; i <= n; i++ {
		pairs := []StreamField{{Name: "f", Value: []byte("v")}}
		if wide {
			pairs = []StreamField{{Name: fmt.Sprintf("field%d", i), Value: []byte(fmt.Sprintf("value-with-some-length-%d", i))}}
		}
		if _, err := v.AddEntry(StreamAddArgs{ID: StreamID{Ms: uint64(i)}, IDGiven: true, SeqGiven: true}, pairs, 0); err != nil {
			t.Fatal(err)
		}
	}
}

// Redis splits listpack nodes at 100 entries or 4096 bytes; approximate
// trimming removes whole nodes only. The counts below come from redis:7.4.
func TestStreamApproxTrimFollowsRedisNodes(t *testing.T) {
	v := NewStreamValue(0)
	addN(t, v, 250, false)
	if got := v.Info(false, 0).Nodes; got != 3 {
		t.Fatalf("250 small entries use %d nodes, want 3", got)
	}
	approx := StreamTrimArgs{Strategy: StreamTrimMaxLen, MaxLen: 10, Approx: true, Limit: 30}
	if got := v.TrimStream(approx); got != 0 {
		t.Fatalf("LIMIT 30 trimmed %d, want 0 (a node holds 100)", got)
	}
	approx.Limit = 10000
	if got := v.TrimStream(approx); got != 200 {
		t.Fatalf("MAXLEN ~ 10 trimmed %d, want 200", got)
	}

	wide := NewStreamValue(0)
	addN(t, wide, 120, true)
	if got := wide.Info(false, 0).Nodes; got != 2 {
		t.Fatalf("120 wide entries use %d nodes, want 2", got)
	}
	if got := wide.TrimStream(StreamTrimArgs{Strategy: StreamTrimMaxLen, MaxLen: 1, Approx: true, Limit: 10000}); got != 89 {
		t.Fatalf("MAXLEN ~ 1 trimmed %d wide entries, want 89", got)
	}
}

// A stream built without the node model (RDB load, Clone) rebuilds it on the
// next write and still trims by nodes.
func TestStreamNodesRebuiltForLoadedStreams(t *testing.T) {
	src := NewStreamValue(0)
	addN(t, src, 150, false)
	loaded := &StreamValue{Entries: make([]*StreamEntry, 0, 150), Groups: map[string]*ConsumerGroup{}}
	for _, e := range src.Entries {
		loaded.Entries = append(loaded.Entries, &StreamEntry{ID: e.ID, Fields: e.Fields})
	}
	loaded.LastID = src.LastID
	if got := loaded.Info(false, 0).Nodes; got != 2 {
		t.Fatalf("loaded stream reports %d nodes, want 2", got)
	}
	if got := loaded.TrimStream(StreamTrimArgs{Strategy: StreamTrimMaxLen, Approx: true, Limit: 10000}); got != 150 {
		t.Fatalf("MAXLEN ~ 0 trimmed %d, want all 150", got)
	}
	clone := src.Clone().(*StreamValue)
	if got := clone.TrimStream(StreamTrimArgs{Strategy: StreamTrimMaxLen, MaxLen: 50, Approx: true, Limit: 10000}); got != 100 {
		t.Fatalf("clone trimmed %d, want the first node of 100", got)
	}
	if got := src.Len(); got != 150 {
		t.Fatalf("trimming the clone changed the source to %d entries", got)
	}
}

func TestStreamDeleteTracksTombstonesAndNodes(t *testing.T) {
	v := NewStreamValue(0)
	addN(t, v, 3, false)
	if got := v.DeleteIDs([]StreamID{{Ms: 2}, {Ms: 9}}); got != 1 {
		t.Fatalf("deleted %d, want 1", got)
	}
	info := v.Info(false, 0)
	if info.MaxDeletedID != "2-0" || info.EntriesAdded != 3 || info.FirstID != "1-0" || info.Length != 2 {
		t.Fatalf("info after XDEL = %+v", info)
	}
	v.DeleteIDs([]StreamID{{Ms: 1}, {Ms: 3}})
	if info := v.Info(false, 0); info.Nodes != 0 || info.FirstID != "0-0" || info.LastID != "3-0" {
		t.Fatalf("info after deleting everything = %+v", info)
	}
	if _, err := v.AddEntry(StreamAddArgs{ID: StreamID{Ms: 3}, IDGiven: true, SeqGiven: true}, nil, 0); err != ErrStreamIDTooSmall {
		t.Fatalf("re-adding the deleted top ID = %v, want ErrStreamIDTooSmall", err)
	}
}

func TestParseStreamInterval(t *testing.T) {
	for _, tc := range []struct {
		in    string
		end   bool
		want  StreamID
		valid bool
	}{
		{"-", false, StreamID{}, true},
		{"+", true, MaxStreamID, true},
		{"5", false, StreamID{Ms: 5}, true},
		{"5", true, StreamID{Ms: 5, Seq: MaxStreamID.Seq}, true},
		{"(5", false, StreamID{Ms: 5, Seq: 1}, true},
		{"(5", true, StreamID{Ms: 5, Seq: MaxStreamID.Seq - 1}, true},
		{"(1-0", true, StreamID{Ms: 0, Seq: MaxStreamID.Seq}, true},
		{"(-", false, StreamID{}, false},
		{"(", false, StreamID{}, false},
		{"(0-0", true, StreamID{}, false},
		{"1-2-3", false, StreamID{}, false},
		{"+1", false, StreamID{}, false},
	} {
		got, err := ParseStreamInterval(tc.in, tc.end)
		if (err == nil) != tc.valid || (tc.valid && got != tc.want) {
			t.Errorf("ParseStreamInterval(%q, %v) = %v, %v; want %v valid=%v", tc.in, tc.end, got, err, tc.want, tc.valid)
		}
	}
}
