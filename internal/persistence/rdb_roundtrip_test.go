package persistence

import (
	"bytes"
	"encoding/binary"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/cachestorm/cachestorm/internal/store"
)

// TestRDBSaveLoadRoundTripAllTypes verifies that a Save→Load cycle preserves
// every serialized value type, their contents, and TTLs. Regression test for
// the 0xFE round-trip break and the dropped 0xFC expiry opcode.
func TestRDBSaveLoadRoundTripAllTypes(t *testing.T) {
	src := store.NewStore()

	mustSet := func(key string, v store.Value, opts store.SetOptions) {
		t.Helper()
		if err := src.Set(key, v, opts); err != nil {
			t.Fatalf("Set %q: %v", key, err)
		}
	}

	mustSet("string_plain", &store.StringValue{Data: []byte("hello")}, store.SetOptions{})
	mustSet("list", &store.ListValue{Elements: [][]byte{[]byte("a"), []byte("b"), []byte("c")}}, store.SetOptions{})
	mustSet("set", &store.SetValue{Members: map[string]struct{}{"x": {}, "y": {}, "z": {}}}, store.SetOptions{})
	mustSet("hash", &store.HashValue{Fields: map[string][]byte{"f1": []byte("v1"), "f2": []byte("v2")}}, store.SetOptions{})
	mustSet("zset", &store.SortedSetValue{Members: map[string]float64{"alice": 90.5, "bob": 85.25, "carol": 77}}, store.SetOptions{})
	mustSet("string_ttl", &store.StringValue{Data: []byte("fleeting")}, store.SetOptions{TTL: 5 * time.Minute})
	mustSet("zset_ttl", &store.SortedSetValue{Members: map[string]float64{"m": 1.5}}, store.SetOptions{TTL: 10 * time.Minute})

	path := filepath.Join(t.TempDir(), "roundtrip.rdb")
	writer := NewRDBWriter(src, RDBConfig{Version: RDBVersion11})
	if err := writer.Save(path); err != nil {
		t.Fatalf("Save: %v", err)
	}

	dst := store.NewStore()
	// Pre-populate: the 0xFE opcode must flush the destination before loading.
	if err := dst.Set("stale", &store.StringValue{Data: []byte("old")}, store.SetOptions{}); err != nil {
		t.Fatalf("Set stale: %v", err)
	}

	if err := NewRDBReader(dst).Load(path); err != nil {
		t.Fatalf("Load: %v", err)
	}

	if _, ok := dst.Get("stale"); ok {
		t.Error("expected 'stale' to be flushed by the 0xFE opcode before load")
	}

	getEntry := func(key string) *store.Entry {
		t.Helper()
		e, ok := dst.Get(key)
		if !ok || e == nil {
			t.Fatalf("key %q missing after round-trip", key)
		}
		return e
	}

	// String (no TTL)
	sv, ok := getEntry("string_plain").Value.(*store.StringValue)
	if !ok || string(sv.Data) != "hello" {
		t.Errorf("string_plain = %#v, want StringValue{hello}", getEntry("string_plain").Value)
	}

	// List — element order preserved
	lv, ok := getEntry("list").Value.(*store.ListValue)
	if !ok {
		t.Fatalf("list type = %T, want *store.ListValue", getEntry("list").Value)
	}
	if len(lv.Elements) != 3 || string(lv.Elements[0]) != "a" || string(lv.Elements[1]) != "b" || string(lv.Elements[2]) != "c" {
		t.Errorf("list elements = %v, want [a b c]", lv.Elements)
	}

	// Set — all members present
	sv2, ok := getEntry("set").Value.(*store.SetValue)
	if !ok {
		t.Fatalf("set type = %T, want *store.SetValue", getEntry("set").Value)
	}
	for _, m := range []string{"x", "y", "z"} {
		if _, ok := sv2.Members[m]; !ok {
			t.Errorf("set missing member %q", m)
		}
	}

	// Hash — all fields present with exact values
	hv, ok := getEntry("hash").Value.(*store.HashValue)
	if !ok {
		t.Fatalf("hash type = %T, want *store.HashValue", getEntry("hash").Value)
	}
	if string(hv.Fields["f1"]) != "v1" || string(hv.Fields["f2"]) != "v2" {
		t.Errorf("hash fields = %v, want f1=v1 f2=v2", hv.Fields)
	}

	// SortedSet — members and scores preserved exactly (previously loaded
	// back as a StringValue rendering).
	zv, ok := getEntry("zset").Value.(*store.SortedSetValue)
	if !ok {
		t.Fatalf("zset type = %T, want *store.SortedSetValue", getEntry("zset").Value)
	}
	if zv.Members["alice"] != 90.5 || zv.Members["bob"] != 85.25 || zv.Members["carol"] != 77 {
		t.Errorf("zset members = %#v, want alice=90.5 bob=85.25 carol=77", zv.Members)
	}

	// TTLs restored via the 0xFC path (millisecond precision loses a little).
	ttlStr := getEntry("string_ttl").TTL()
	if ttlStr <= 4*time.Minute || ttlStr > 5*time.Minute {
		t.Errorf("string_ttl TTL = %v, want in (4m, 5m]", ttlStr)
	}
	ttlZset := getEntry("zset_ttl").TTL()
	if ttlZset <= 9*time.Minute || ttlZset > 10*time.Minute {
		t.Errorf("zset_ttl TTL = %v, want in (9m, 10m]", ttlZset)
	}

	// Keys saved without an expiry must not gain one.
	if ttl := getEntry("string_plain").TTL(); ttl != -1 {
		t.Errorf("string_plain TTL = %v, want -1 (no expiry)", ttl)
	}
}

// TestRDBLoadExpirySecondsAndExpiredSkip covers the 0xFD (seconds) opcode and
// the skip path for entries whose expiry already passed at load time.
func TestRDBLoadExpirySecondsAndExpiredSkip(t *testing.T) {
	var body bytes.Buffer

	// select-db with DB number, then a string entry expiring in 1 hour via 0xFD.
	body.WriteByte(0xFE)
	writeRDBLength(&body, 0)
	body.WriteByte(0xFD)
	binary.Write(&body, binary.LittleEndian, uint32(time.Now().Add(time.Hour).Unix()))
	body.WriteByte(0x00) // value type = string
	writeRDBString(&body, "sec_ttl_key")
	writeRDBString(&body, "sec_ttl_val")

	// An entry whose expiry is already in the past — must be skipped on load.
	body.WriteByte(0xFC)
	binary.Write(&body, binary.LittleEndian, time.Now().Add(-time.Hour).UnixMilli())
	body.WriteByte(0x00)
	writeRDBString(&body, "expired_key")
	writeRDBString(&body, "expired_val")

	rdbData := buildValidRDB("0011", body.Bytes())

	s := store.NewStore()
	path := filepath.Join(t.TempDir(), "expiry.rdb")
	if err := os.WriteFile(path, rdbData, 0644); err != nil {
		t.Fatalf("write rdb: %v", err)
	}

	if err := NewRDBReader(s).Load(path); err != nil {
		t.Fatalf("Load: %v", err)
	}

	e, ok := s.Get("sec_ttl_key")
	if !ok || e == nil {
		t.Fatal("expected 'sec_ttl_key' to be loaded")
	}
	ttl := e.TTL()
	if ttl <= 59*time.Minute || ttl > time.Hour {
		t.Errorf("sec_ttl_key TTL = %v, want in (59m, 1h]", ttl)
	}

	if _, ok := s.Get("expired_key"); ok {
		t.Error("expected 'expired_key' to be skipped (expiry already passed)")
	}
}
