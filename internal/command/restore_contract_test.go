package command

import (
	"bytes"
	"strconv"
	"testing"
	"time"

	"github.com/cachestorm/cachestorm/internal/resp"
	"github.com/cachestorm/cachestorm/internal/store"
)

func TestRestoreRelativeTTLContract(t *testing.T) {
	s := store.NewStore()
	result := restoreRun(t, s, "RESTORE", "k", "60000", restorePayload(t, "v"))
	if result.Type == resp.TypeError {
		t.Fatal(result.Err)
	}
	deadline, ok := s.ExpiryDeadline("k")
	if !ok || deadline < time.Now().UnixMilli()+59000 || deadline > time.Now().UnixMilli()+61000 {
		t.Fatalf("relative TTL deadline=%d", deadline)
	}
}

func TestDumpBinaryCollectionRoundTrip(t *testing.T) {
	for _, tc := range []struct {
		name  string
		value store.Value
	}{
		{"hash", &store.HashValue{Fields: map[string][]byte{"f&=\x00": []byte("v&=\x00"), "": {}}}},
		{"list", &store.ListValue{Elements: [][]byte{[]byte("a,b\x00"), {}, []byte("tail")}}},
		{"set", &store.SetValue{Members: map[string]struct{}{"a,b\x00": {}, "": {}}}},
		{"zset", &store.SortedSetValue{Members: map[string]float64{"m,:\x00": 1.23456789012345, "": -2}}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			s := store.NewStore()
			if err := s.Set("source", tc.value, store.SetOptions{}); err != nil {
				t.Fatal(err)
			}
			payload := restoreRun(t, s, "DUMP", "source")
			if payload.Type != resp.TypeBulkString {
				t.Fatalf("dump=%v", payload)
			}
			result := restoreRun(t, s, "RESTORE", "target", "0", string(payload.Bulk))
			if result.Type == resp.TypeError {
				t.Fatal(result.Err)
			}
			target, ok := s.Get("target")
			if !ok || !dumpValuesEqual(tc.value, target.Value) {
				t.Fatalf("roundtrip lost %s: %#v", tc.name, target)
			}
		})
	}
}

func dumpValuesEqual(a, b store.Value) bool {
	switch v := a.(type) {
	case *store.HashValue:
		other, ok := b.(*store.HashValue)
		if !ok || len(v.Fields) != len(other.Fields) {
			return false
		}
		for k, val := range v.Fields {
			got, exists := other.Fields[k]
			if !exists || !bytes.Equal(val, got) {
				return false
			}
		}
		return true
	case *store.ListValue:
		other, ok := b.(*store.ListValue)
		if !ok || len(v.Elements) != len(other.Elements) {
			return false
		}
		for i, val := range v.Elements {
			if !bytes.Equal(val, other.Elements[i]) {
				return false
			}
		}
		return true
	case *store.SetValue:
		other, ok := b.(*store.SetValue)
		if !ok || len(v.Members) != len(other.Members) {
			return false
		}
		for k := range v.Members {
			if _, exists := other.Members[k]; !exists {
				return false
			}
		}
		return true
	case *store.SortedSetValue:
		other, ok := b.(*store.SortedSetValue)
		if !ok || len(v.Members) != len(other.Members) {
			return false
		}
		for k, score := range v.Members {
			got, exists := other.Members[k]
			if !exists || got != score {
				return false
			}
		}
		return true
	}
	return false
}

func TestRestoreRejectsInvalidOptionsAndTTL(t *testing.T) {
	payload := restorePayload(t, "fresh")
	for _, args := range [][]string{
		{"-1", payload}, {"9223372036854775807", payload}, {"0", payload, "BOGUS"}, {"0", payload, "IDLETIME"}, {"0", payload, "IDLETIME", "-1"}, {"0", payload, "FREQ", "256"}, {"0", payload, "IDLETIME", "0", "FREQ", "1"},
	} {
		t.Run(strconv.Quote(args[0])+strconv.Itoa(len(args))+args[len(args)-1], func(t *testing.T) {
			s := store.NewStore()
			restoreRun(t, s, "SET", "k", "old")
			command := append([]string{"RESTORE", "k"}, args...)
			command = append(command, "REPLACE")
			result := restoreRun(t, s, command...)
			if result.Type != resp.TypeError {
				t.Fatal("invalid restore accepted")
			}
			got := restoreRun(t, s, "GET", "k")
			if string(got.Bulk) != "old" {
				t.Fatal("failed restore changed existing value")
			}
		})
	}
}

func TestRestoreChecksumAndStoreErrorsPreserveExistingValue(t *testing.T) {
	payload := restorePayload(t, "fresh")
	for _, corrupt := range []string{payload[:len(payload)-1], payload[:len(payload)-1] + string([]byte{payload[len(payload)-1] ^ 1}), "CACHSTORM001\x010:fresh"} {
		s := store.NewStore()
		restoreRun(t, s, "SET", "k", "old")
		result := restoreRun(t, s, "RESTORE", "k", "0", corrupt, "REPLACE")
		if result.Type != resp.TypeError {
			t.Fatal("invalid checksum accepted")
		}
		if got := restoreRun(t, s, "GET", "k"); string(got.Bulk) != "old" {
			t.Fatal("checksum failure replaced key")
		}
	}
	s := store.NewStore()
	restoreRun(t, s, "SET", "k", "old")
	s.ConfigureMemory(1, store.EvictionNoEviction, 70, 90, 5)
	result := restoreRun(t, s, "RESTORE", "k", "0", payload, "REPLACE")
	if result.Type != resp.TypeError {
		t.Fatal("restore acknowledged rejected allocation")
	}
	if got := restoreRun(t, s, "GET", "k"); string(got.Bulk) != "old" {
		t.Fatal("rejected allocation replaced key")
	}
}

func TestRestoreFrequencyUsesConfiguredLFUPolicy(t *testing.T) {
	s := store.NewStore()
	s.ConfigureMemory(0, store.EvictionAllKeysLFU, 70, 90, 5)
	got := restoreRun(t, s, "RESTORE", "k", "0", restorePayload(t, "value"), "FREQ", "200")
	if got.Type == resp.TypeError {
		t.Fatal(got.Err)
	}
	entry, ok := s.GetShard("k").Get("k")
	if !ok || entry.LFUCount() != 200 {
		t.Fatal("frequency was not restored")
	}
}

// A time series or JSON document must not be dumped as a string: it travels
// under its own private type and comes back as the same type.
func TestDumpKeepsNonRedisTypes(t *testing.T) {
	for _, value := range []store.Value{store.NewTimeSeriesValue(0), store.MustParseJSONValue(`{"k":"v"}`)} {
		s := store.NewStore()
		if err := s.Set("k", value, store.SetOptions{}); err != nil {
			t.Fatal(err)
		}
		got := restoreRun(t, s, "DUMP", "k")
		if got.Type != resp.TypeBulkString {
			t.Fatalf("DUMP of %T: %v", value, got.Err)
		}
		if r := restoreRun(t, s, "RESTORE", "copy", "0", string(got.Bulk)); r.Type == resp.TypeError {
			t.Fatalf("RESTORE of %T: %s", value, r.Err)
		}
		entry, ok := s.GetShard("copy").Get("copy")
		if !ok || entry.Value.Type() != value.Type() {
			t.Fatalf("RESTORE of %T gave %T", value, entry.Value)
		}
	}
}

func TestDumpBitmapRestoresRawStringBytes(t *testing.T) {
	s := store.NewStore()
	data := []byte{0, 255, 128, 1}
	if err := s.Set("k", &BitmapValue{Data: data}, store.SetOptions{}); err != nil {
		t.Fatal(err)
	}
	payload := restoreRun(t, s, "DUMP", "k")
	if payload.Type != resp.TypeBulkString {
		t.Fatal("bitmap DUMP failed")
	}
	if got := restoreRun(t, s, "RESTORE", "copy", "0", string(payload.Bulk)); got.Type == resp.TypeError {
		t.Fatal(got.Err)
	}
	if got := restoreRun(t, s, "GET", "copy"); !bytes.Equal(got.Bulk, data) {
		t.Fatal("bitmap bytes lost")
	}
}
