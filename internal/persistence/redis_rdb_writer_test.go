package persistence

import (
	"bytes"
	"encoding/binary"
	"strings"
	"testing"

	"github.com/cachestorm/cachestorm/internal/store"
)

type entryList []struct {
	db    int
	key   string
	entry *store.Entry
}

func (l entryList) ForEachEntry(fn func(db int, key string, entry interface{}) error) error {
	for _, e := range l {
		if err := fn(e.db, e.key, e.entry); err != nil {
			return err
		}
	}
	return nil
}

func TestWriteRedisRDBLoadsBack(t *testing.T) {
	hash := store.NewHashFrom(map[string][]byte{"a": []byte("1"), "b": []byte("2")})
	hash.SetFieldExpireAt("a", 4102444800000)
	expiring := store.NewEntry(&store.StringValue{Data: []byte("v")})
	expiring.ExpiresAt.Store(4102444800000)
	src := entryList{
		{0, "s", expiring},
		{0, "l", store.NewEntry(store.NewListFrom([][]byte{[]byte("x"), []byte("y")}))},
		{0, "j", store.NewEntry(store.MustParseJSONValue(`{"a":1}`))},
		{2, "h", store.NewEntry(hash)},
	}
	var out bytes.Buffer
	code := "#!lua name=lib\nredis.register_function('f', function() return 1 end)"
	skipped, err := WriteRedisRDB(&out, src, RedisRDBInfo{Functions: []string{code}, StreamDB: 2, ReplID: strings.Repeat("a", 40), Offset: 7})
	if err != nil || skipped != 1 {
		t.Fatalf("WriteRedisRDB skipped %d, %v; want the JSON key left out", skipped, err)
	}
	if !strings.HasPrefix(out.String(), "REDIS0012") {
		t.Fatalf("header %q: a hash with field expirations needs RDB 12", out.String()[:9])
	}
	dbs := store.NewDatabases(store.NewStore(), 16)
	r := NewRDBReaderForDatabases(dbs)
	if err := r.LoadBytes(out.Bytes()); err != nil {
		t.Fatal(err)
	}
	if e, ok := dbs.DB(0).Get("s"); !ok || e.ExpiresAt.Load() != 4102444800000 {
		t.Fatal("expiring string lost")
	}
	if _, ok := dbs.DB(0).Get("l"); !ok {
		t.Fatal("list lost")
	}
	if e, ok := dbs.DB(2).Get("h"); !ok || e.Value.(*store.HashValue).FieldExpireAt("a") != 4102444800000 {
		t.Fatal("hash with a field expiration lost")
	}
	if r.AuxFields["repl-stream-db"] != "2" || r.AuxFields["repl-offset"] != "7" {
		t.Fatalf("aux = %v", r.AuxFields)
	}
	if codes := FunctionCodes([]byte(r.AuxFields["cachestorm-functions"])); len(codes) != 1 || codes[0] != code {
		t.Fatalf("functions = %q", codes)
	}
}

func TestRedisRDBExtensionsLoadBack(t *testing.T) {
	ts := store.NewTimeSeriesValue(0)
	ts.Samples = append(ts.Samples, store.TimeSeriesSample{Timestamp: 1000, Value: 5.5})
	tagged := store.NewEntry(&store.StringValue{Data: []byte("v")})
	tagged.SetTags([]string{"red", "blue"})
	hot := store.NewEntry(&store.StringValue{Data: []byte("h")})
	hot.SetLFUCount(42)
	gone := store.NewHashFrom(map[string][]byte{"f": []byte("v")})
	gone.SetFieldExpireAt("f", 1)
	src := entryList{
		{0, "j", store.NewEntry(store.MustParseJSONValue(`{"a":[1,2]}`))},
		{0, "ts", store.NewEntry(ts)},
		{0, "tagged", tagged},
		{0, "hot", hot},
		{1, "gone", store.NewEntry(gone)},
	}
	out, skipped, err := EncodeRedisRDB(src, RedisRDBInfo{Extensions: true, LFU: true, Aux: map[string]string{"cachestorm-search": "x"}})
	if err != nil || skipped != 1 {
		t.Fatalf("EncodeRedisRDB skipped %d, %v; want the expired hash left out", skipped, err)
	}
	if err := SealRDBChecksum(out); err != nil {
		t.Fatal(err)
	}
	if !bytes.Contains(out, []byte{rdbTypeModule2, 1, 'j'}) {
		t.Fatal("JSON document must be saved as a module value")
	}
	base := store.NewStore()
	lfu, _ := store.ParseEvictionPolicy("allkeys-lfu")
	base.ConfigureMemory(0, lfu, 80, 95, 5)
	dbs := store.NewDatabases(base, 16)
	r := NewRDBReaderForDatabases(dbs)
	if err := r.LoadBytes(out); err != nil {
		t.Fatal(err)
	}
	db := dbs.DB(0)
	if e, ok := db.Get("j"); !ok || string(e.Value.(*store.JSONValue).Bytes()) != `{"a":[1,2]}` {
		t.Fatal("JSON document lost")
	}
	if e, ok := db.Get("ts"); !ok || len(e.Value.(*store.TimeSeriesValue).Samples) != 1 {
		t.Fatal("time series lost")
	}
	if e, ok := db.Get("tagged"); !ok || strings.Join(e.TagList(), ",") != "red,blue" {
		t.Fatal("tags lost")
	}
	if e, ok := db.GetShard("hot").Get("hot"); !ok || e.LFUCount() != 42 {
		t.Fatalf("LFU counter lost: %d (saved %d)", e.LFUCount(), hot.LFUCount())
	}
	if r.AuxFields["cachestorm-search"] != "x" {
		t.Fatal("extra aux field lost")
	}
}

func TestRedisRDBRefusesOtherModules(t *testing.T) {
	out, _, err := EncodeRedisRDB(entryList{{0, "j", store.NewEntry(store.MustParseJSONValue(`1`))}}, RedisRDBInfo{Extensions: true})
	if err != nil {
		t.Fatal(err)
	}
	i := bytes.Index(out, []byte{rdbTypeModule2, 1, 'j'}) + 3
	copy(out[i:], appendDumpLength(nil, rdbModuleID("MBbloom--", 3)))
	if err := SealRDBChecksum(out); err != nil {
		t.Fatal(err)
	}
	r := NewRDBReaderForDatabases(store.NewDatabases(store.NewStore(), 16))
	if err := r.LoadBytes(out); err == nil || !strings.Contains(err.Error(), "module values of type MBbloom-- (encoding version 3) are not supported") {
		t.Fatalf("LoadBytes = %v", err)
	}
}

// redisModuleRDB is an RDB file as Redis with modules writes it: a module's
// aux data, then the given keys' module values in db 0.
func redisModuleRDB(t *testing.T, values map[string][]byte) []byte {
	t.Helper()
	out := []byte("REDIS0011")
	out = append(out, 0xFA)
	out = appendRDBString(out, "redis-ver")
	out = appendRDBString(out, "7.4.0")
	out = append(out, 0xF7)
	out = appendDumpLength(out, rdbModuleID("ft-index0", 2))
	out = appendDumpLength(out, rdbModuleOpcodeUInt)
	out = appendDumpLength(out, 1)
	out = appendDumpLength(out, rdbModuleOpcodeUInt)
	out = appendDumpLength(out, 1<<40)
	out = appendDumpLength(out, rdbModuleOpcodeSInt)
	out = appendDumpLength(out, 5)
	out = appendDumpLength(out, rdbModuleOpcodeDouble)
	out = append(out, 1, 2, 3, 4, 5, 6, 7, 8)
	out = appendDumpLength(out, rdbModuleOpcodeFloat)
	out = append(out, 1, 2, 3, 4)
	out = appendDumpLength(out, rdbModuleOpcodeStr)
	out = appendRDBString(out, "idx")
	out = appendDumpLength(out, rdbModuleOpcodeEOF)
	out = append(out, 0xFE, 0)
	for _, key := range sortedTestKeys(values) {
		out = append(out, rdbTypeModule2)
		out = appendRDBString(out, key)
		out = append(out, values[key]...)
	}
	out = append(out, 0xFF, 0, 0, 0, 0, 0, 0, 0, 0)
	if err := SealRDBChecksum(out); err != nil {
		t.Fatal(err)
	}
	return out
}

func sortedTestKeys(m map[string][]byte) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	for i := range keys {
		for j := i + 1; j < len(keys); j++ {
			if keys[j] < keys[i] {
				keys[i], keys[j] = keys[j], keys[i]
			}
		}
	}
	return keys
}

// jsonModuleValue is a document as RedisJSON saves it at encoding version
// encver; version 2 adds a count and, when it is not zero, two strings.
func jsonModuleValue(name string, encver uint64, doc string, extra uint64) []byte {
	out := appendDumpLength(nil, rdbModuleID(name, encver))
	out = appendDumpLength(out, rdbModuleOpcodeStr)
	out = appendRDBString(out, doc)
	if encver == 2 {
		out = appendDumpLength(out, rdbModuleOpcodeUInt)
		out = appendDumpLength(out, extra)
		for range min(extra, 1) * 2 {
			out = appendDumpLength(out, rdbModuleOpcodeStr)
			out = appendRDBString(out, "x")
		}
	}
	return appendDumpLength(out, rdbModuleOpcodeEOF)
}

// A Redis Stack or Redis 8 file loads: its JSON documents, of RedisJSON's
// string encodings and of CacheStorm's former type, and around a search
// module's aux data, which is skipped.
func TestLoadRedisJSONModuleValues(t *testing.T) {
	file := redisModuleRDB(t, map[string][]byte{
		"v3":     jsonModuleValue("ReJSON-RL", 3, `{"a":[1,"x"]}`, 0),
		"v2":     jsonModuleValue("ReJSON-RL", 2, `[true,null]`, 0),
		"v2aux":  jsonModuleValue("ReJSON-RL", 2, `"s"`, 3),
		"legacy": jsonModuleValue("CStorm-JS", 1, `{"b":2}`, 0),
	})
	dbs := store.NewDatabases(store.NewStore(), 16)
	r := NewRDBReaderForDatabases(dbs)
	if err := r.LoadBytes(file); err != nil {
		t.Fatal(err)
	}
	for key, want := range map[string]string{"v3": `{"a":[1,"x"]}`, "v2": `[true,null]`, "v2aux": `"s"`, "legacy": `{"b":2}`} {
		e, ok := dbs.DB(0).Get(key)
		if !ok {
			t.Fatalf("%s lost", key)
		}
		if got := string(e.Value.(*store.JSONValue).Bytes()); got != want {
			t.Fatalf("%s = %s, want %s", key, got, want)
		}
	}
	if strings.Join(r.SkippedModuleAux, ",") != "ft-index0" {
		t.Fatalf("SkippedModuleAux = %v", r.SkippedModuleAux)
	}

	for name, value := range map[string][]byte{
		"a newer RedisJSON encoding": jsonModuleValue("ReJSON-RL", 5, `1`, 0),
		"a truncated document":       jsonModuleValue("ReJSON-RL", 3, `1`, 0)[:9],
		"text that is not JSON":      jsonModuleValue("ReJSON-RL", 3, `{`, 0),
	} {
		r := NewRDBReaderForDatabases(store.NewDatabases(store.NewStore(), 16))
		if err := r.LoadBytes(redisModuleRDB(t, map[string][]byte{"k": value})); err == nil {
			t.Fatalf("%s loaded", name)
		}
	}
}

// DUMP saves a document as RedisJSON does, so Redis with RedisJSON restores
// it; CacheStorm restores its own and its former payloads.
func TestDumpJSONAsRedisJSON(t *testing.T) {
	payload, err := EncodeDump(store.MustParseJSONValue(`{"k":[1,2]}`))
	if err != nil {
		t.Fatal(err)
	}
	want := jsonModuleValue("ReJSON-RL", 3, `{"k":[1,2]}`, 0)
	if !bytes.Equal(payload[1:len(payload)-10], want) || payload[0] != rdbTypeModule2 {
		t.Fatalf("payload %q, want module value %q", payload, want)
	}
	for _, p := range [][]byte{payload, legacyJSONDump(`{"k":[1,2]}`)} {
		v, err := DecodeDump(p)
		if err != nil {
			t.Fatal(err)
		}
		if got := string(v.(*store.JSONValue).Bytes()); got != `{"k":[1,2]}` {
			t.Fatalf("restored %s", got)
		}
	}
}

func legacyJSONDump(doc string) []byte {
	out := appendDumpLength([]byte{dumpTypeJSON}, uint64(len(doc)))
	out = append(out, doc...)
	out = append(out, 11, 0)
	return binary.LittleEndian.AppendUint64(out, rdbChecksum(0, out))
}
