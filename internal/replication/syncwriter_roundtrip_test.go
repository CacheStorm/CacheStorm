package replication

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/cachestorm/cachestorm/internal/persistence"
	"github.com/cachestorm/cachestorm/internal/store"
)

// writeSyncStream renders a complete RDB payload via SyncWriter (header +
// select-db + pairs + end marker) that persistence.RDBReader must be able to
// parse symmetrically.
func writeSyncStream(t *testing.T, pairs []struct {
	key   string
	value string
	ttl   bool
}) []byte {
	t.Helper()
	var buf bytes.Buffer
	w := NewSyncWriter(&buf)

	if err := w.WriteRDBHeader(); err != nil {
		t.Fatalf("WriteRDBHeader: %v", err)
	}
	if err := w.WriteDatabaseSelect(0); err != nil {
		t.Fatalf("WriteDatabaseSelect: %v", err)
	}
	for _, p := range pairs {
		var (
			ttl      time.Duration
			expireAt int64
		)
		if p.ttl {
			ttl = time.Hour
			expireAt = time.Now().Add(ttl).UnixMilli()
		}
		if err := w.WriteKeyValuePair(p.key, p.value, ttl, expireAt); err != nil {
			t.Fatalf("WriteKeyValuePair(key len=%d): %v", len(p.key), err)
		}
	}
	if err := w.WriteEnd(); err != nil {
		t.Fatalf("WriteEnd: %v", err)
	}
	return buf.Bytes()
}

// loadStreamIntoStore writes the stream to a temp file and loads it with the
// canonical persistence.RDBReader.
func loadStreamIntoStore(t *testing.T, stream []byte) *store.Store {
	t.Helper()
	dst := store.NewStore()
	path := filepath.Join(t.TempDir(), "sync.rdb")
	if err := os.WriteFile(path, stream, 0644); err != nil {
		t.Fatalf("write stream: %v", err)
	}
	if err := persistence.NewRDBReader(dst).Load(path); err != nil {
		t.Fatalf("RDBReader.Load: %v", err)
	}
	return dst
}

// TestSyncWriterRoundTripLargeKeysAndValues verifies that SyncWriter entries
// round-trip through persistence.RDBReader across all three RDB length
// encoding classes (< 64, 64..16383 two-byte, >= 16384 four-byte) for both
// keys and values. Regression for the raw byte(len) emitters, which misparsed
// at >= 64 bytes, and for the former RESP-style "$" value framing, which the
// reader could not parse for any length.
func TestSyncWriterRoundTripLargeKeysAndValues(t *testing.T) {
	sizes := []int{10, 100, 20000}

	for _, keySize := range sizes {
		for _, valSize := range sizes {
			key := fmt.Sprintf("%s-v%d", strings.Repeat("k", keySize), valSize)
			value := strings.Repeat("v", valSize)

			stream := writeSyncStream(t, []struct {
				key   string
				value string
				ttl   bool
			}{{key: key, value: value}})

			dst := loadStreamIntoStore(t, stream)

			entry, ok := dst.Get(key)
			if !ok || entry == nil {
				t.Fatalf("key (len %d) missing after round-trip (value len %d)", keySize, valSize)
			}
			sv, ok := entry.Value.(*store.StringValue)
			if !ok {
				t.Fatalf("key (len %d) type = %T, want *store.StringValue", keySize, entry.Value)
			}
			if string(sv.Data) != value {
				t.Errorf("key (len %d): value len = %d, want %d", keySize, len(sv.Data), valSize)
			}
			if ttl := entry.TTL(); ttl != -1 {
				t.Errorf("key (len %d): TTL = %v, want -1 (no expiry)", keySize, ttl)
			}
		}
	}
}

// TestSyncWriterRoundTripTTL verifies the 0xFC millisecond-expiry path emits
// bytes the RDB reader restores as a live TTL, for a key in the two-byte
// length encoding class.
func TestSyncWriterRoundTripTTL(t *testing.T) {
	key := strings.Repeat("ttl", 40) // 120 bytes → 2-byte length encoding
	value := "expires"

	stream := writeSyncStream(t, []struct {
		key   string
		value string
		ttl   bool
	}{{key: key, value: value, ttl: true}})

	dst := loadStreamIntoStore(t, stream)

	entry, ok := dst.Get(key)
	if !ok || entry == nil {
		t.Fatal("TTL key missing after round-trip")
	}
	ttl := entry.TTL()
	if ttl <= 59*time.Minute || ttl > time.Hour {
		t.Errorf("TTL = %v, want in (59m, 1h]", ttl)
	}
}
