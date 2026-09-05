package persistence_test

import (
	"bytes"
	"encoding/json"
	"path/filepath"
	"reflect"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/cachestorm/cachestorm/internal/command"
	"github.com/cachestorm/cachestorm/internal/persistence"
	"github.com/cachestorm/cachestorm/internal/resp"
	"github.com/cachestorm/cachestorm/internal/store"
)

// rewriteStoreAdapter adapts a *store.Store to the AOFRewriter's
// GetAll() map[string]interface{} source, exposing entries as *store.Entry
// so writeEntry can emit typed commands.
type rewriteStoreAdapter struct {
	entries map[string]*store.Entry
}

func (a *rewriteStoreAdapter) GetAll() map[string]interface{} {
	out := make(map[string]interface{}, len(a.entries))
	for k, v := range a.entries {
		out[k] = v
	}
	return out
}

// TestAOFRewriteRoundTripPreservesTypes verifies that the AOF rewriter emits
// router-executable commands that reconstruct every typed value on replay,
// instead of flattening them to lossy SET strings.
func TestAOFRewriteRoundTripPreservesTypes(t *testing.T) {
	src := store.NewStore()

	src.Set("str:key", &store.StringValue{Data: []byte("hello")}, store.SetOptions{})
	src.Set("hash:key", &store.HashValue{Fields: map[string][]byte{
		"field1": []byte("value1"),
		"field2": []byte("value2"),
	}}, store.SetOptions{})
	src.Set("list:key", &store.ListValue{Elements: [][]byte{
		[]byte("a"), []byte("b"), []byte("c"),
	}}, store.SetOptions{})
	src.Set("set:key", &store.SetValue{Members: map[string]struct{}{
		"m1": {}, "m2": {},
	}}, store.SetOptions{})
	src.Set("zset:key", &store.SortedSetValue{Members: map[string]float64{
		"alice": 90.5, "bob": 85.25,
	}}, store.SetOptions{})

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
	// Use recent timestamps: AOF replay goes through TS.ADD, which enforces
	// retention against wall-clock — stale fixture samples would be pruned.
	now := time.Now().UnixMilli()
	ts.Samples = []store.TimeSeriesSample{
		{Timestamp: now - 60000, Value: 21.5},
		{Timestamp: now, Value: 22.0},
	}
	src.Set("ts:temp", ts, store.SetOptions{})

	dir := t.TempDir()
	aofPath := filepath.Join(dir, "appendonly.aof")
	rw := persistence.NewAOFRewriter(persistence.AOFConfig{DataDir: dir}, &rewriteStoreAdapter{entries: src.GetAll()})
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
	command.RegisterGeoCommands(router)
	command.RegisterJSONCommands(router)
	command.RegisterStreamCommands(router)
	command.RegisterTSCommands(router)

	for i, cmd := range commands {
		var respBuf bytes.Buffer
		err := router.ExecuteSilent(command.NewContext(cmd.Name, cmd.Args, dst, resp.NewWriter(&respBuf)))
		t.Logf("replay[%d] %s %v -> err=%v resp=%q", i, cmd.Name, cmd.Args, err, respBuf.String())
		if err != nil {
			t.Fatalf("replay %s failed: %v", cmd.Name, err)
		}
	}

	// String
	strEntry, ok := dst.Get("str:key")
	if !ok {
		t.Fatal("str:key missing after replay")
	}
	strOut, ok := strEntry.Value.(*store.StringValue)
	if !ok || string(strOut.Data) != "hello" {
		t.Fatalf("str:key = %T %+v", strEntry.Value, strEntry.Value)
	}

	// Hash
	hashEntry, ok := dst.Get("hash:key")
	if !ok {
		t.Fatal("hash:key missing after replay")
	}
	hashOut, ok := hashEntry.Value.(*store.HashValue)
	if !ok {
		t.Fatalf("hash:key type = %T, want *store.HashValue", hashEntry.Value)
	}
	if string(hashOut.Fields["field1"]) != "value1" || string(hashOut.Fields["field2"]) != "value2" {
		t.Fatalf("hash fields = %+v", hashOut.Fields)
	}

	// List: RPUSH preserves order.
	listEntry, ok := dst.Get("list:key")
	if !ok {
		t.Fatal("list:key missing after replay")
	}
	listOut, ok := listEntry.Value.(*store.ListValue)
	if !ok {
		t.Fatalf("list:key type = %T, want *store.ListValue", listEntry.Value)
	}
	if len(listOut.Elements) != 3 ||
		string(listOut.Elements[0]) != "a" || string(listOut.Elements[2]) != "c" {
		t.Fatalf("list elements = %+v", listOut.Elements)
	}

	// Set
	setEntry, ok := dst.Get("set:key")
	if !ok {
		t.Fatal("set:key missing after replay")
	}
	setOut, ok := setEntry.Value.(*store.SetValue)
	if !ok {
		t.Fatalf("set:key type = %T, want *store.SetValue", setEntry.Value)
	}
	if _, ok := setOut.Members["m1"]; !ok {
		t.Fatalf("set members = %+v", setOut.Members)
	}
	if _, ok := setOut.Members["m2"]; !ok {
		t.Fatalf("set members = %+v", setOut.Members)
	}

	// SortedSet: exact scores.
	zsetEntry, ok := dst.Get("zset:key")
	if !ok {
		t.Fatal("zset:key missing after replay")
	}
	zsetOut, ok := zsetEntry.Value.(*store.SortedSetValue)
	if !ok {
		t.Fatalf("zset:key type = %T, want *store.SortedSetValue", zsetEntry.Value)
	}
	if zsetOut.Members["alice"] != 90.5 || zsetOut.Members["bob"] != 85.25 {
		t.Fatalf("zset members = %+v", zsetOut.Members)
	}

	// Geo: exact coordinates.
	geoEntry, ok := dst.Get("geo:sicily")
	if !ok {
		t.Fatal("geo:sicily missing after replay")
	}
	geoOut, ok := geoEntry.Value.(*store.GeoValue)
	if !ok {
		t.Fatalf("geo:sicily type = %T, want *store.GeoValue", geoEntry.Value)
	}
	if pal := geoOut.Points["palermo"]; pal.Lon != 13.361389 || pal.Lat != 38.115556 {
		t.Fatalf("palermo = %+v", pal)
	}
	if cat := geoOut.Points["catania"]; cat.Lon != 15.087269 || cat.Lat != 37.502669 {
		t.Fatalf("catania = %+v", cat)
	}

	// JSON: semantic document equality (JSON.SET re-normalizes bytes).
	jsonEntry, ok := dst.Get("json:doc")
	if !ok {
		t.Fatal("json:doc missing after replay")
	}
	jsonOut, ok := jsonEntry.Value.(*store.JSONValue)
	if !ok {
		t.Fatalf("json:doc type = %T, want *store.JSONValue", jsonEntry.Value)
	}
	var got, want interface{}
	if err := json.Unmarshal(jsonOut.Data, &got); err != nil {
		t.Fatalf("loaded JSON unmarshal failed: %v", err)
	}
	if err := json.Unmarshal(jsonVal.Data, &want); err != nil {
		t.Fatalf("source JSON unmarshal failed: %v", err)
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("json document mismatch: got %s, want %s", jsonOut.Data, jsonVal.Data)
	}

	// Stream: entries, IDs, fields survive; MaxLen/groups are runtime state.
	streamEntry, ok := dst.Get("stream:sensors")
	if !ok {
		t.Fatal("stream:sensors missing after replay")
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
	if streamOut.LastID != "1-2" {
		t.Fatalf("stream LastID = %s, want 1-2", streamOut.LastID)
	}

	// TimeSeries replay lands in the TS manager (TS.CREATE/TS.ADD), not the
	// store — verify via TS.GET through a captured response writer.
	var tsResp bytes.Buffer
	tsCtx := command.NewContext("TS.GET", [][]byte{[]byte("ts:temp")}, dst, resp.NewWriter(&tsResp))
	if err := router.ExecuteSilent(tsCtx); err != nil {
		t.Fatalf("replay TS.GET failed: %v", err)
	}
	out := tsResp.String()
	if !strings.Contains(out, strconv.FormatInt(now, 10)) {
		t.Fatalf("TS.GET response missing latest timestamp: %q", out)
	}
	if !strings.Contains(out, "22") {
		t.Fatalf("TS.GET response missing latest value: %q", out)
	}
}
