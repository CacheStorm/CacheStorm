package persistence

import (
	"encoding/binary"
	"encoding/hex"
	"reflect"
	"testing"

	"github.com/cachestorm/cachestorm/internal/store"
)

// redisStreamDump is Redis 7.4's DUMP of a stream built with XADD s 1-1 f v,
// XADD s 1-2 f w, XADD s 2-1 a b c 300, XDEL s 1-2, XGROUP CREATE s g1 0 and
// XREADGROUP GROUP g1 alice COUNT 1 STREAMS s >. Its listpack is LZF
// compressed and keeps the deleted entry.
const redisStreamDump = "15011000000000000000010000000000000001c3393e073e000000180002014000078166020001020100200105817602040103200820150077200a4008401b0e816102816202816302c12c020801ff020201010101020301026731010181ffffffffffffffff0100000000000000010000000000000001bf0cbd82a1010000010105616c696365bf0cbd82a1010000bf0cbd82a101000001000000000000000100000000000000010c00c354a0e48bbf060d"

type streamView struct {
	Entries              [][]store.StreamField
	IDs                  []string
	Length, EntriesAdded int64
	LastID, MaxDeletedID string
	Groups               map[string]string
	Pending              map[string]store.PendingEntry
	ConsumerPending      map[string]int64
	ConsumerSeen         map[string][2]int64
	GroupRead            map[string]int64
}

func viewStream(v *store.StreamValue) streamView {
	out := streamView{Length: v.Length, EntriesAdded: v.EntriesAdded, LastID: v.LastID, MaxDeletedID: v.MaxDeletedID,
		Groups: map[string]string{}, Pending: map[string]store.PendingEntry{}, ConsumerPending: map[string]int64{},
		ConsumerSeen: map[string][2]int64{}, GroupRead: map[string]int64{}}
	for _, e := range v.Entries {
		out.IDs = append(out.IDs, e.ID)
		out.Entries = append(out.Entries, e.FieldPairs())
	}
	for name, g := range v.Groups {
		out.Groups[name] = g.LastID
		out.GroupRead[name] = g.EntriesRead
		for id, p := range g.Pending {
			out.Pending[name+"/"+id] = *p
		}
		for cname, c := range g.Consumers {
			out.ConsumerPending[name+"/"+cname] = c.Pending
			out.ConsumerSeen[name+"/"+cname] = [2]int64{c.SeenTime, c.ActiveTime}
		}
	}
	return out
}

func TestRestoreRedisStreamDump(t *testing.T) {
	payload, err := hex.DecodeString(redisStreamDump)
	if err != nil {
		t.Fatal(err)
	}
	value, err := DecodeDump(payload)
	if err != nil {
		t.Fatalf("decode: %v", err)
	}
	v := value.(*store.StreamValue)
	got := viewStream(v)
	if !reflect.DeepEqual(got.IDs, []string{"1-1", "2-1"}) || got.Length != 2 || got.LastID != "2-1" ||
		got.MaxDeletedID != "1-2" || got.EntriesAdded != 3 {
		t.Fatalf("stream: %+v", got)
	}
	want := [][]store.StreamField{{{Name: "f", Value: []byte("v")}}, {{Name: "a", Value: []byte("b")}, {Name: "c", Value: []byte("300")}}}
	if !reflect.DeepEqual(got.Entries, want) {
		t.Fatalf("entries: %+v", got.Entries)
	}
	p, ok := got.Pending["g1/1-1"]
	if got.Groups["g1"] != "1-1" || got.GroupRead["g1"] != store.EntriesReadInvalid || !ok || p.Consumer != "alice" || p.Deliveries != 1 ||
		got.ConsumerPending["g1/alice"] != 1 || len(got.Pending) != 1 {
		t.Fatalf("groups: %+v", got)
	}

	again, err := EncodeDump(v)
	if err != nil {
		t.Fatal(err)
	}
	if again[0] != rdbTypeStreamListpacks3 {
		t.Fatalf("DUMP wrote type %d, want Redis' stream type %d", again[0], rdbTypeStreamListpacks3)
	}
	back, err := DecodeDump(again)
	if err != nil {
		t.Fatalf("decode own dump: %v", err)
	}
	if b := viewStream(back.(*store.StreamValue)); !reflect.DeepEqual(b, got) {
		t.Fatalf("round trip changed the stream:\n got  %+v\n want %+v", b, got)
	}
}

func TestStreamDumpRejectsCorruptPayloads(t *testing.T) {
	payload, _ := hex.DecodeString(redisStreamDump)
	for i := 1; i < len(payload)-10; i++ {
		bad := append([]byte(nil), payload[:len(payload)-10]...)
		bad[i] ^= 0x5a
		bad = appendDumpTrailer(bad, 11)
		_, _ = DecodeDump(bad)
	}
	short := appendDumpTrailer(payload[:20], 11)
	if _, err := DecodeDump(short); err == nil {
		t.Fatal("a truncated stream payload must be refused")
	}
}

func appendDumpTrailer(body []byte, version uint16) []byte {
	out := binary.LittleEndian.AppendUint16(append([]byte(nil), body...), version)
	return binary.LittleEndian.AppendUint64(out, rdbChecksum(0, out))
}
