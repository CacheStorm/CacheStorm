package persistence

import (
	"encoding/binary"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/cachestorm/cachestorm/internal/store"
)

// A stream with IDMP state and an XNACK-released entry is dumped as Redis
// 8.8's RDB_TYPE_STREAM_LISTPACKS_5 and read back whole.
func TestDumpStreamIDMPAndNackZone(t *testing.T) {
	v := store.NewStreamValue(0)
	now := uint64(time.Now().UnixMilli())
	for i := uint64(1); i <= 3; i++ {
		if !v.LoadEntry(store.StreamID{Ms: now, Seq: i}, []store.StreamField{{Name: "f", Value: []byte("v")}}) {
			t.Fatal("LoadEntry")
		}
	}
	v.LastID = store.StreamID{Ms: now, Seq: 3}.String()
	v.Length, v.EntriesAdded = 3, 3
	g := store.NewConsumerGroup("g")
	g.LastID = v.LastID
	g.EntriesRead = 3
	g.Consumers["c"] = &store.Consumer{Name: "c", Pending: 1, Active: true}
	g.AddLoadedPending(&store.PendingEntry{ID: store.StreamID{Ms: now, Seq: 1}.String(), Consumer: "c", DeliveryTS: int64(now), Deliveries: 1})
	g.AddLoadedPending(&store.PendingEntry{ID: store.StreamID{Ms: now, Seq: 2}.String(), DeliveryTS: 0, Deliveries: 2})
	if !g.MarkLoadedNacked(store.StreamID{Ms: now, Seq: 2}.String()) {
		t.Fatal("MarkLoadedNacked")
	}
	v.Groups["g"] = g
	v.SetLoadedIDMP(&store.StreamIDMP{Duration: 100, MaxSize: 50, Added: 7, Duplicates: 2,
		Producers: map[string][]store.IDMPEntry{"p1": {{IID: "a", ID: store.StreamID{Ms: now, Seq: 1}}}}})

	payload, err := EncodeDump(v)
	if err != nil {
		t.Fatal(err)
	}
	if payload[0] != rdbTypeStreamListpacks5 {
		t.Fatalf("type %d, want %d", payload[0], rdbTypeStreamListpacks5)
	}
	if ver := binary.LittleEndian.Uint16(payload[len(payload)-10:]); ver != 14 {
		t.Fatalf("DUMP version %d, want 14", ver)
	}
	back, err := DecodeDump(payload)
	if err != nil {
		t.Fatal(err)
	}
	s := back.(*store.StreamValue)
	m := s.IDMP()
	if m == nil || m.Duration != 100 || m.MaxSize != 50 || m.Added != 7 || m.Duplicates != 2 || len(m.Producers["p1"]) != 1 {
		t.Fatalf("IDMP after round trip: %+v", m)
	}
	bg := s.Groups["g"]
	bg.RLock()
	nacked := bg.NackedLocked()
	owner := bg.Pending[store.StreamID{Ms: now, Seq: 1}.String()].Consumer
	bg.RUnlock()
	if len(nacked) != 1 || nacked[0] != (store.StreamID{Ms: now, Seq: 2}).String() || owner != "c" {
		t.Fatalf("NACK zone %v, owner %q", nacked, owner)
	}

	// Without IDMP state or released entries the stream keeps _3.
	plain := store.NewStreamValue(0)
	plain.LoadEntry(store.StreamID{Ms: 1, Seq: 1}, []store.StreamField{{Name: "f", Value: []byte("v")}})
	plain.LastID, plain.Length, plain.EntriesAdded = "1-1", 1, 1
	if p, _ := EncodeDump(plain); p[0] != rdbTypeStreamListpacks3 {
		t.Fatalf("plain stream type %d", p[0])
	}
}

// Redis 8.6 puts module key metadata before a value's type; it is skipped,
// and payloads up to RDB version 15 are read.
func TestDumpKeyMetadataAndVersion15(t *testing.T) {
	body := []byte{rdbOpcodeKeyMeta, 1, 'a', 'b', 'c', 'd', rdbModuleOpcodeEOF, 0, 2, 'h', 'i'}
	payload := binary.LittleEndian.AppendUint16(append([]byte(nil), body...), 15)
	payload = binary.LittleEndian.AppendUint64(payload, rdbChecksum(0, payload))
	v, err := DecodeDump(payload)
	if err != nil {
		t.Fatal(err)
	}
	if s, ok := v.(store.CompactString); !ok || string(s.Bytes()) != "hi" {
		t.Fatalf("value %#v", v)
	}
	payload = binary.LittleEndian.AppendUint16([]byte{0, 1, 'x'}, 16)
	payload = binary.LittleEndian.AppendUint64(payload, rdbChecksum(0, payload))
	if _, err := DecodeDump(payload); err != ErrDumpChecksum {
		t.Fatalf("version 16: %v", err)
	}
}

// A Redis 8.10 RDB file (version 15) whose key carries module metadata
// loads: the version is accepted and the metadata skipped.
func TestLoadRedis810RDBFile(t *testing.T) {
	str := func(b []byte, s string) []byte { return append(append(b, byte(len(s))), s...) }
	b := []byte("REDIS0015")
	b = str(str(append(b, 0xFA), "redis-ver"), "8.10.3")
	b = append(b, 0xFE, 0, 0xFB, 1, 0)
	b = append(b, rdbOpcodeKeyMeta, 1, 'a', 'b', 'c', 'd', rdbModuleOpcodeEOF)
	b = str(str(append(b, 0), "k"), "v")
	b = append(b, 0xFF)
	b = binary.LittleEndian.AppendUint64(b, rdbChecksum(0, b))
	path := t.TempDir() + "/dump.rdb"
	if err := os.WriteFile(path, b, 0o600); err != nil {
		t.Fatal(err)
	}
	s := store.NewStore()
	if err := NewRDBReader(s).Load(path); err != nil {
		t.Fatal(err)
	}
	if e, ok := s.Get("k"); !ok || e == nil {
		t.Fatal("key k not loaded")
	}

	// The types CacheStorm does not have are refused by name.
	b = []byte("REDIS0015")
	b = str(str(append(b, 0xFA), "redis-ver"), "8.10.3")
	b = append(b, 0xFE, 0, 0xFB, 1, 0, 28, 1, 'a', 0)
	b = append(b, 0xFF)
	b = binary.LittleEndian.AppendUint64(b, rdbChecksum(0, b))
	if err := os.WriteFile(path, b, 0o600); err != nil {
		t.Fatal(err)
	}
	if err := NewRDBReader(store.NewStore()).Load(path); err == nil || !strings.Contains(err.Error(), "array values (Redis 8.8) are not supported") {
		t.Fatalf("array type: %v", err)
	}
}
