package persistence

import (
	"bufio"
	"bytes"
	"os"
	"path/filepath"
	"testing"

	"github.com/cachestorm/cachestorm/internal/store"
)

type diagnosticStringValue struct{}

func (v *diagnosticStringValue) Type() store.DataType { return store.DataTypeString }
func (v *diagnosticStringValue) SizeOf() int64        { return 24 }
func (v *diagnosticStringValue) Clone() store.Value   { return &diagnosticStringValue{} }
func (v *diagnosticStringValue) String() string       { return "diagnostic, not serialized data" }

func TestStringShapedValueRequiresExplicitSerialization(t *testing.T) {
	v := &diagnosticStringValue{}
	if _, err := EncodeDump(v); err == nil {
		t.Fatal("DUMP accepted a diagnostic representation")
	}
	var output bytes.Buffer
	rw := &AOFRewriter{}
	if err := rw.writeValueCommands(bufio.NewWriter(&output), "key", v); err == nil {
		t.Fatal("AOF accepted a diagnostic representation")
	}
	s := store.NewStore()
	if err := s.Set("key", v, store.SetOptions{}); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(t.TempDir(), "dump.rdb")
	previous := []byte("previous snapshot")
	if err := os.WriteFile(path, previous, 0600); err != nil {
		t.Fatal(err)
	}
	if err := NewRDBWriter(s, RDBConfig{Version: RDBVersion11, Checksum: true}).Save(path); err == nil {
		t.Fatal("RDB accepted a diagnostic representation")
	}
	got, err := os.ReadFile(path)
	if err != nil || !bytes.Equal(got, previous) {
		t.Fatalf("previous snapshot changed: %q, %v", got, err)
	}
}

func TestLegacyHLLExplicitSerializationRetainsRegisters(t *testing.T) {
	h := &store.HyperLogLogValue{}
	h.Add([]byte("alpha"))
	h.Add([]byte("beta"))
	expected, err := h.RedisStringBytes()
	if err != nil {
		t.Fatal(err)
	}
	payload, err := EncodeDump(h)
	if err != nil {
		t.Fatal(err)
	}
	value, err := DecodeDump(payload)
	if err != nil || value == nil || value.String() != string(expected) {
		t.Fatalf("DUMP roundtrip err=%v", err)
	}
	var output bytes.Buffer
	bw := bufio.NewWriter(&output)
	if err := (&AOFRewriter{}).writeValueCommands(bw, "key", h); err != nil {
		t.Fatal(err)
	}
	if err := bw.Flush(); err != nil {
		t.Fatal(err)
	}
	if !bytes.Contains(output.Bytes(), expected) {
		t.Fatal("AOF omitted encoded registers")
	}
	s := store.NewStore()
	if err := s.Set("key", h, store.SetOptions{}); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(t.TempDir(), "dump.rdb")
	if err := NewRDBWriter(s, RDBConfig{Version: RDBVersion11, Checksum: true}).Save(path); err != nil {
		t.Fatal(err)
	}
	restored := store.NewStore()
	if err := NewRDBReader(restored).Load(path); err != nil {
		t.Fatal(err)
	}
	entry, ok := restored.Get("key")
	if !ok || entry.Value.String() != string(expected) {
		t.Fatal("RDB did not retain encoded registers")
	}
}
