package persistence

import (
	"testing"

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
	ts.AddWithLabels(1000, 1.5, map[string]string{"a": "b"})
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
