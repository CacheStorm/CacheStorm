package persistence

import (
	"bytes"
	"path/filepath"
	"testing"
	"time"

	"github.com/cachestorm/cachestorm/internal/store"
)

// TestRDBSaveLoadRoundTripExtendedTypes covers the Geo/JSON/Stream/TimeSeries
// value types, which previously fell to the lossy String() encoding (type 0).
// Each now has a dedicated RDB type byte (5-8) and must survive Save->Load
// with its contents, type identity, and TTL intact.
func TestRDBSaveLoadRoundTripExtendedTypes(t *testing.T) {
	src := store.NewStore()

	geo := store.NewGeoValue()
	geo.Add("palermo", 13.361389, 38.115556)
	geo.Add("catania", 15.087269, 37.502669)
	src.Set("geo:sicily", geo, store.SetOptions{})

	jsonVal, err := store.NewJSONValue(map[string]interface{}{"name": "doc", "count": float64(42)})
	if err != nil {
		t.Fatalf("NewJSONValue failed: %v", err)
	}
	src.Set("json:doc", jsonVal, store.SetOptions{})

	stream := store.NewStreamValue(1000)
	stream.Entries = append(stream.Entries,
		&store.StreamEntry{ID: "1-1", Fields: map[string][]byte{"sensor": []byte("42")}},
		&store.StreamEntry{ID: "1-2", Fields: map[string][]byte{"sensor": []byte("43"), "unit": []byte("C")}},
	)
	stream.LastID = "1-2"
	stream.Length = 2
	src.Set("stream:sensors", stream, store.SetOptions{})

	ts := store.NewTimeSeriesValue(24 * time.Hour)
	ts.Labels = map[string]string{"sensor_id": "42", "area": "factory"}
	ts.Samples = []store.TimeSeriesSample{
		{Timestamp: 1700000000000, Value: 21.5},
		{Timestamp: 1700000060000, Value: 22.0, Labels: map[string]string{"quality": "good"}},
	}
	src.Set("ts:temp", ts, store.SetOptions{TTL: 10 * time.Minute})

	path := filepath.Join(t.TempDir(), "dump.rdb")
	w := NewRDBWriter(src, RDBConfig{Version: RDBVersion11})
	if err := w.Save(path); err != nil {
		t.Fatalf("Save failed: %v", err)
	}

	dst := store.NewStore()
	dst.Set("preexisting", &store.StringValue{Data: []byte("gone")}, store.SetOptions{})
	r := NewRDBReader(dst)
	if err := r.Load(path); err != nil {
		t.Fatalf("Load failed: %v", err)
	}

	if _, exists := dst.Get("preexisting"); exists {
		t.Error("preexisting key should have been flushed by the select-db opcode")
	}

	// Geo: members and exact coordinates survive.
	geoEntry, ok := dst.Get("geo:sicily")
	if !ok {
		t.Fatal("geo:sicily missing after load")
	}
	geoOut, ok := geoEntry.Value.(*store.GeoValue)
	if !ok {
		t.Fatalf("geo:sicily type = %T, want *store.GeoValue", geoEntry.Value)
	}
	if len(geoOut.Points) != 2 {
		t.Fatalf("geo points = %d, want 2", len(geoOut.Points))
	}
	pal, ok := geoOut.Points["palermo"]
	if !ok || pal.Lon != 13.361389 || pal.Lat != 38.115556 {
		t.Fatalf("palermo = %+v", pal)
	}
	cat, ok := geoOut.Points["catania"]
	if !ok || cat.Lon != 15.087269 || cat.Lat != 37.502669 {
		t.Fatalf("catania = %+v", cat)
	}

	// JSON: the raw document bytes survive.
	jsonEntry, ok := dst.Get("json:doc")
	if !ok {
		t.Fatal("json:doc missing after load")
	}
	jsonOut, ok := jsonEntry.Value.(*store.JSONValue)
	if !ok {
		t.Fatalf("json:doc type = %T, want *store.JSONValue", jsonEntry.Value)
	}
	if !bytes.Equal(jsonOut.Data, jsonVal.Data) {
		t.Fatalf("json data mismatch: got %s, want %s", jsonOut.Data, jsonVal.Data)
	}

	// Stream: entries, IDs, fields, and bounds survive; groups start empty.
	streamEntry, ok := dst.Get("stream:sensors")
	if !ok {
		t.Fatal("stream:sensors missing after load")
	}
	streamOut, ok := streamEntry.Value.(*store.StreamValue)
	if !ok {
		t.Fatalf("stream:sensors type = %T, want *store.StreamValue", streamEntry.Value)
	}
	if len(streamOut.Entries) != 2 || streamOut.Entries[0].ID != "1-1" || streamOut.Entries[1].ID != "1-2" {
		t.Fatalf("stream entries = %+v", streamOut.Entries)
	}
	if string(streamOut.Entries[1].Fields["unit"]) != "C" {
		t.Fatalf("stream entry[1] fields = %+v", streamOut.Entries[1].Fields)
	}
	if streamOut.LastID != "1-2" || streamOut.Length != 2 || streamOut.MaxLen != 1000 {
		t.Fatalf("stream bounds = lastID:%s len:%d maxLen:%d", streamOut.LastID, streamOut.Length, streamOut.MaxLen)
	}
	if len(streamOut.Groups) != 0 {
		t.Fatalf("consumer groups should start empty on load, got %d", len(streamOut.Groups))
	}

	// TimeSeries: retention, labels, samples (incl. per-sample labels), TTL.
	tsEntry, ok := dst.Get("ts:temp")
	if !ok {
		t.Fatal("ts:temp missing after load")
	}
	tsOut, ok := tsEntry.Value.(*store.TimeSeriesValue)
	if !ok {
		t.Fatalf("ts:temp type = %T, want *store.TimeSeriesValue", tsEntry.Value)
	}
	if tsOut.Retention != 24*time.Hour {
		t.Fatalf("retention = %v, want 24h", tsOut.Retention)
	}
	if tsOut.Labels["sensor_id"] != "42" || tsOut.Labels["area"] != "factory" {
		t.Fatalf("labels = %+v", tsOut.Labels)
	}
	if len(tsOut.Samples) != 2 {
		t.Fatalf("samples = %d, want 2", len(tsOut.Samples))
	}
	if tsOut.Samples[0].Timestamp != 1700000000000 || tsOut.Samples[0].Value != 21.5 {
		t.Fatalf("sample[0] = %+v", tsOut.Samples[0])
	}
	if tsOut.Samples[1].Value != 22.0 || tsOut.Samples[1].Labels["quality"] != "good" {
		t.Fatalf("sample[1] = %+v", tsOut.Samples[1])
	}
	ttl := tsEntry.TTL()
	if ttl <= 9*time.Minute || ttl > 10*time.Minute {
		t.Fatalf("TTL = %v, want ~(9m,10m]", ttl)
	}
}
