package persistence

import (
	"bytes"
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
	hash := &store.HashValue{Fields: map[string][]byte{"a": []byte("1"), "b": []byte("2")}}
	hash.SetFieldExpireAt("a", 4102444800000)
	expiring := store.NewEntry(&store.StringValue{Data: []byte("v")})
	expiring.ExpiresAt.Store(4102444800000)
	src := entryList{
		{0, "s", expiring},
		{0, "l", store.NewEntry(&store.ListValue{Elements: [][]byte{[]byte("x"), []byte("y")}})},
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
	gone := &store.HashValue{Fields: map[string][]byte{"f": []byte("v")}}
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
	copy(out[i:], appendDumpLength(nil, rdbModuleID("ReJSON-RL", 3)))
	if err := SealRDBChecksum(out); err != nil {
		t.Fatal(err)
	}
	r := NewRDBReaderForDatabases(store.NewDatabases(store.NewStore(), 16))
	if err := r.LoadBytes(out); err == nil || !strings.Contains(err.Error(), "module values are not supported") {
		t.Fatalf("LoadBytes = %v", err)
	}
}
