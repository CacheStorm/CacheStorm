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
		{0, "j", store.NewEntry(&store.JSONValue{Data: []byte(`{"a":1}`)})},
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
