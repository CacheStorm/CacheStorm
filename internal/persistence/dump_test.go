package persistence

import (
	"bytes"
	"encoding/binary"
	"encoding/hex"
	"testing"

	"github.com/cachestorm/cachestorm/internal/store"
)

func dumpFixture(t *testing.T, s string) []byte {
	t.Helper()
	b, err := hex.DecodeString(s)
	if err != nil {
		t.Fatal(err)
	}
	return b
}

func TestDecodeRedis74DumpFixtures(t *testing.T) {
	for _, tc := range []struct{ name, hex string }{
		{"string-binary", "00086100620d0a2c3d260c0079fc5b6578887069"},
		{"string-int8", "00c0850c00a97ffeb3ae5975c9"},
		{"string-int16", "00c1c7cf0c00ac9b96f64ef1b899"},
		{"string-int32", "00c215cd5b070c006fc1fdc91ac3a9d7"},
		{"list-packed", "1201024041410000000d00800183612c00040001dfff02cfff02d00002f1ff7f03f1008003f2ffff7f04f200008004f3ffffff7f05f30000008005f4ffffffffffffff7f09ff0c00fcf640f4e8c444d0"},
		{"list-plain", "120201c3408167100a6c617267652d6e6f64656ce0ff09e1ff0de2ff11e2ff11e2ff11e2ff11e2ff11e2ff11e2ff11e2ff11e2ff11e2ff11e2ff11e2ff11e2ff11e2ff11e2ff11e2ff11e2ff11e2ff11e2ff11e2ff11e2ff11e2ff11e2ff11e2ff11e2ff11e2ff11e2ff11e2ff11e2ff11e2ff11e2ff11e2ff11e2ff11e2ff11e2ff11e2d211016465020d0d0000000100846c61737405ff0c00d626cf1c23cde583"},
		{"hash-packed", "101c1c00000006008466263d00058476263d0005800180010101dffe02ff0c003896a3ae31c870f6"},
		{"hash-table", "04020000056669656c64c31342ee056c617267656ce0ff04e1ff08e1cd080167650c00704a93de9696e7de"},
		{"set-packed", "1416160000000300800184612c62000585706c61696e06ff0c00154a01b89abdc7d2"},
		{"set-intset", "0bc320300408000000052003a00003800080ff8000007f8011200c60092007200001ff7f0c009584c0fc7a6f4034"},
		{"zset-packed", "11363600000006008001842d696e6605846d2c3a000590312e32333435363738393031323334351188696e66696e6974650983696e6604ff0c00649b908d2c93e9c1"},
		{"zset-table", "0502067365636f6e640000000000000840c30e40b4066d656d6265726de0a20501657200000000000000c00c00c6b05b4b69c02502"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			payload := dumpFixture(t, tc.hex)
			value, err := DecodeDump(payload)
			if err != nil {
				t.Fatal(err)
			}
			canonical, err := EncodeDump(value)
			if err != nil {
				t.Fatal(err)
			}
			again, err := DecodeDump(canonical)
			if err != nil {
				t.Fatal(err)
			}
			second, err := EncodeDump(again)
			if err != nil || !bytes.Equal(canonical, second) {
				t.Fatalf("canonical roundtrip err=%v", err)
			}
			for i := range payload {
				bad := bytes.Clone(payload)
				bad[i] ^= 1
				if _, err := DecodeDump(bad); err == nil {
					t.Fatalf("corruption accepted at %d", i)
				}
			}
		})
	}
}

func FuzzDecodeDump(f *testing.F) {
	for _, body := range [][]byte{{0, 0}, {0, 0xc0, 0xff}, {1, 1, 1, 'a'}, {2, 1, 1, 'a'}, {4, 1, 1, 'a', 1, 'b'}, {18, 1, 2, 7, 7, 0, 0, 0, 0, 0, 255}, {16, 0}, {0, 0xc3, 1, 3, 0, 'a'}} {
		f.Add(body)
	}
	f.Fuzz(func(t *testing.T, body []byte) {
		if len(body) > 65536 {
			t.Skip()
		}
		payload := binary.LittleEndian.AppendUint16(bytes.Clone(body), 12)
		payload = binary.LittleEndian.AppendUint64(payload, rdbChecksum(0, payload))
		value, err := DecodeDump(payload)
		if err == nil {
			encoded, err := EncodeDump(value)
			if err != nil {
				t.Fatal(err)
			}
			if _, err := DecodeDump(encoded); err != nil {
				t.Fatal(err)
			}
		}
	})
}

func dumpTestPayload(body []byte) []byte {
	payload := binary.LittleEndian.AppendUint16(bytes.Clone(body), 12)
	return binary.LittleEndian.AppendUint64(payload, rdbChecksum(0, payload))
}

func TestDecodeLegacyRedisCollectionEncodings(t *testing.T) {
	list := []byte{14, 0, 0, 0, 10, 0, 0, 0, 1, 0, 0, 1, 'a', 255}
	hash := []byte{17, 0, 0, 0, 13, 0, 0, 0, 2, 0, 0, 1, 'a', 3, 1, 'b', 255}
	zset := []byte{19, 0, 0, 0, 13, 0, 0, 0, 2, 0, 0, 1, 'a', 3, 3, '1', '.', '5', 255}
	blob := func(typ byte, data []byte) []byte {
		return append(appendDumpLength([]byte{typ}, uint64(len(data))), data...)
	}
	for _, tc := range []struct {
		name     string
		body     []byte
		expected store.Value
	}{
		{"ziplist-list", blob(10, list), &store.ListValue{Elements: [][]byte{[]byte("a")}}},
		{"ziplist-hash", blob(13, hash), &store.HashValue{Fields: map[string][]byte{"a": []byte("b")}}},
		{"ziplist-zset", blob(12, zset), &store.SortedSetValue{Members: map[string]float64{"a": 1.5}}},
		{"zipmap", blob(9, []byte{1, 1, 'a', 1, 0, 'b', 255}), &store.HashValue{Fields: map[string][]byte{"a": []byte("b")}}},
		{"quicklist", append([]byte{14, 1}, blob(0, list)[1:]...), &store.ListValue{Elements: [][]byte{[]byte("a")}}},
		{"old-zset", []byte{3, 1, 1, 'a', 3, '1', '.', '5'}, &store.SortedSetValue{Members: map[string]float64{"a": 1.5}}},
		{"intset16", blob(11, []byte{2, 0, 0, 0, 2, 0, 0, 0, 255, 255, 255, 127}), &store.SetValue{Members: map[string]struct{}{"-1": {}, "32767": {}}}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			value, err := DecodeDump(dumpTestPayload(tc.body))
			if err != nil {
				t.Fatal(err)
			}
			got, err := EncodeDump(value)
			if err != nil {
				t.Fatal(err)
			}
			expected, err := EncodeDump(tc.expected)
			if err != nil || !bytes.Equal(got, expected) {
				t.Fatalf("decoded value differs: err=%v", err)
			}
		})
	}
}

func TestDumpRejectsMalformedStructureAndEmptyCollections(t *testing.T) {
	for _, body := range [][]byte{
		{0, 1, 'a', 0}, {0, 0xc3, 2, 1, 32, 0}, {0, 0xc3, 1, 2, 1}, {1, 0x81, 255, 255, 255, 255, 255, 255, 255, 255},
		{2, 2, 1, 'a', 1, 'a'}, {4, 2, 1, 'a', 0, 1, 'a', 0}, {5, 1, 1, 'a', 0, 0, 0, 0, 0, 0, 248, 127},
		{16, 7, 7, 0, 0, 0, 1, 0, 255}, {20, 9, 9, 0, 0, 0, 1, 0, 1, 0, 255}, {18, 1, 3, 0},
	} {
		if _, err := DecodeDump(dumpTestPayload(body)); err == nil {
			t.Fatalf("malformed structure accepted: %x", body)
		}
	}
	for _, value := range []store.Value{&store.ListValue{}, &store.HashValue{}, &store.SetValue{}, &store.SortedSetValue{}} {
		if _, err := EncodeDump(value); err == nil {
			t.Fatalf("invalid or unsupported value encoded: %T", value)
		}
	}
}
