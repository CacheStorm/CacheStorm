package persistence

import (
	"math/rand/v2"
	"testing"
)

func bytewiseRDBChecksum(crc uint64, data []byte) uint64 {
	for _, b := range data {
		crc = rdbCRCTables[0][byte(crc)^b] ^ crc>>8
	}
	return crc
}

func TestRDBChecksumMatchesRedis(t *testing.T) {
	// Redis' crc64 test vector: crc64(0, "123456789") == 0xe9c6d914c4b8d9ca.
	if got := rdbChecksum(0, []byte("123456789")); got != 0xe9c6d914c4b8d9ca {
		t.Fatalf("crc64(123456789) = %#x", got)
	}
	rng := rand.New(rand.NewPCG(1, 2))
	data := make([]byte, 4096)
	for i := range data {
		data[i] = byte(rng.Uint32())
	}
	for n := range 300 {
		split := rng.IntN(n + 1)
		want := bytewiseRDBChecksum(7, data[:n])
		if got := rdbChecksum(rdbChecksum(7, data[:split]), data[split:n]); got != want {
			t.Fatalf("length %d split %d: %#x, want %#x", n, split, got, want)
		}
	}
	if rdbChecksum(0, data) != bytewiseRDBChecksum(0, data) {
		t.Fatal("4096 bytes differ")
	}
}
