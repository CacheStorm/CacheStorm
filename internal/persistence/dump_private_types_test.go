package persistence

import (
	"bufio"
	"bytes"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/cachestorm/cachestorm/internal/store"
)

// Streams, JSON documents and time series have no Redis DUMP encoding; they
// travel under private type bytes so MIGRATE moves them between nodes whole.
func TestDumpPrivateTypesRoundTrip(t *testing.T) {
	stream := store.NewStreamValue(0)
	if _, err := stream.Add("1-1", map[string][]byte{"f": []byte("v")}); err != nil {
		t.Fatal(err)
	}
	if _, err := stream.Add("2-1", map[string][]byte{"g": []byte("w")}); err != nil {
		t.Fatal(err)
	}
	if err := stream.CreateGroup("grp", "0-0"); err != nil {
		t.Fatal(err)
	}
	ts := store.NewTimeSeriesValue(0)
	ts.SetLabels([]string{"a"}, []string{"b"})
	ts.Upsert(1000, 1.5, "") //nolint:errcheck
	for _, value := range []store.Value{stream, store.NewStreamValue(0), ts, store.MustParseJSONValue(`{"k":"v"}`)} {
		payload, err := EncodeDump(value)
		if err != nil {
			t.Fatalf("%T: %v", value, err)
		}
		got, err := DecodeDump(payload)
		if err != nil {
			t.Fatalf("%T: decode: %v", value, err)
		}
		again, err := EncodeDump(got)
		if err != nil || string(again) != string(payload) {
			t.Fatalf("%T: the round trip changed the value (%v)", value, err)
		}
	}
	decoded, _ := DecodeDump(mustDump(t, stream))
	s := decoded.(*store.StreamValue)
	if len(s.Entries) != 2 || s.LastID != "2-1" || s.Groups["grp"] == nil {
		t.Fatalf("stream after the round trip: %+v", s)
	}
	payload := mustDump(t, stream)
	payload[3] ^= 0xff
	if _, err := DecodeDump(payload); err == nil {
		t.Fatal("a corrupted payload was accepted")
	}
}

func mustDump(t *testing.T, v store.Value) []byte {
	t.Helper()
	b, err := EncodeDump(v)
	if err != nil {
		t.Fatal(err)
	}
	return b
}

func TestTimeSeriesSettingsSurviveDumpAndAOF(t *testing.T) {
	ts := store.NewTimeSeriesValue(5 * time.Second)
	ts.SetLabels([]string{"z", "a"}, []string{"1", "2"})
	ts.DuplicatePolicy, ts.ChunkSize, ts.IgnoreMaxTimeDiff, ts.IgnoreMaxValDiff = "SUM", 128, 10, 0.5
	ts.Rules = []store.TSRule{{Dest: "dst", Aggregation: "avg", Bucket: 10, Open: []store.TimeSeriesSample{{Timestamp: 1, Value: 2}}}}
	ts.SourceKey = "src"
	ts.Upsert(1, 2, "") //nolint:errcheck
	payload, err := EncodeDump(ts)
	if err != nil {
		t.Fatal(err)
	}
	back, err := DecodeDump(payload)
	if err != nil {
		t.Fatal(err)
	}
	got := back.(*store.TimeSeriesValue)
	if got.Policy() != "SUM" || got.ChunkSize != 128 || got.IgnoreMaxValDiff != 0.5 || got.SourceKey != "src" ||
		len(got.Rules) != 1 || len(got.Rules[0].Open) != 1 || got.Retention != 5*time.Second ||
		fmt.Sprint(got.LabelList()) != "[[z 1] [a 2]]" || len(got.Samples) != 1 {
		t.Fatalf("decoded series = %+v", got)
	}
	var out bytes.Buffer
	w := bufio.NewWriter(&out)
	if err := (&AOFRewriter{}).writeValueCommands(w, "k", ts); err != nil {
		t.Fatal(err)
	}
	w.Flush()
	if !strings.Contains(out.String(), "RESTORE") {
		t.Fatalf("a time series must be rewritten as RESTORE, got %q", out.String())
	}
}
