package persistence

import (
	"bytes"
	"math/rand/v2"
	"strings"
	"testing"

	"github.com/cachestorm/cachestorm/internal/store"
)

func TestLZFRoundTrips(t *testing.T) {
	rng := rand.New(rand.NewPCG(3, 4))
	inputs := [][]byte{
		bytes.Repeat([]byte("a"), 21),
		bytes.Repeat([]byte("abc"), 5000),
		[]byte(strings.Repeat("hello world ", 40) + "tail"),
	}
	for range 400 {
		n := 3 + rng.IntN(3000)
		b := make([]byte, n)
		alphabet := 1 + rng.IntN(255)
		if len(inputs)%2 == 0 {
			alphabet = 1 + rng.IntN(4)
		}
		for i := range b {
			b[i] = byte(rng.IntN(alphabet))
		}
		inputs = append(inputs, b)
	}
	compressed := 0
	for _, in := range inputs {
		out := make([]byte, len(in)+len(in)/16+64)
		n := lzfCompress(out, in)
		if n == 0 {
			t.Fatalf("%d bytes did not fit a generous buffer", len(in))
		}
		got, err := decodeDumpLZF(out[:n], len(in))
		if err != nil || !bytes.Equal(got, in) {
			t.Fatalf("%d bytes did not round trip: %v", len(in), err)
		}
		// A tight limit either fits or reports 0, never writes past it.
		limit := len(in) - 4
		tight := make([]byte, limit, limit+8)
		if n := lzfCompress(tight, in); n > 0 {
			compressed++
			if got, err := decodeDumpLZF(tight[:n], len(in)); err != nil || !bytes.Equal(got, in) {
				t.Fatalf("%d bytes under a tight limit did not round trip: %v", len(in), err)
			}
		}
		if n := lzfCompress(tight[:limit], in); n > limit {
			t.Fatalf("wrote %d bytes into %d", n, limit)
		}
	}
	if compressed < len(inputs)/2 {
		t.Fatalf("only %d of %d inputs compressed", compressed, len(inputs))
	}
}

func TestLZFCompressesText(t *testing.T) {
	in := []byte(strings.Repeat(`{"user":"alice","roles":["admin","dev"],"active":true}`, 20))
	out := make([]byte, len(in))
	n := lzfCompress(out, in)
	if n == 0 || n > len(in)/5 {
		t.Fatalf("%d bytes of repeated JSON compressed to %d", len(in), n)
	}
}

func TestRDBStringEncodings(t *testing.T) {
	for _, c := range []struct {
		in  string
		tag byte
	}{
		{"0", 0xC0}, {"-128", 0xC0}, {"127", 0xC0}, {"128", 0xC1}, {"-32768", 0xC1},
		{"32768", 0xC2}, {"2147483647", 0xC2}, {"-2147483648", 0xC2},
		{"2147483648", 10}, {"-0", 2}, {"007", 3}, {"+5", 2}, {"1 ", 2}, {"", 0},
		{strings.Repeat("x", 21), 0xC3}, {"abcdefghijklmnopqrstuvwxyz0123", 30},
	} {
		enc := appendRDBRawString(nil, c.in)
		if enc[0] != c.tag {
			t.Errorf("%q encodes as %#x, want %#x", c.in, enc[0], c.tag)
		}
		d := dumpDecoder{data: enc}
		if got := d.str(); d.err != nil || got != c.in {
			t.Errorf("%q decodes as %q, %v", c.in, got, d.err)
		}
	}
	SetRDBCompression(false)
	defer SetRDBCompression(true)
	if enc := appendRDBRawString(nil, strings.Repeat("x", 40)); enc[0] != 40 {
		t.Fatalf("rdbcompression no still compressed: %#x", enc[0])
	}
}

func TestDumpOfCompressibleValuesRestores(t *testing.T) {
	long := strings.Repeat("value-", 50)
	values := []store.Value{
		&store.StringValue{Data: []byte(long)},
		store.NewListFrom([][]byte{[]byte(long), []byte("12"), []byte("-7")}),
		store.NewHashFrom(map[string][]byte{long: []byte(long), "n": []byte("100000")}),
		store.NewSortedSetFrom(map[string]float64{long: 1.5, "42": 2}),
	}
	for _, v := range values {
		payload, err := EncodeDump(v)
		if err != nil {
			t.Fatal(err)
		}
		back, err := DecodeDump(payload)
		if err != nil {
			t.Fatal(err)
		}
		again, _ := EncodeDump(back)
		if !bytes.Equal(payload, again) {
			t.Fatalf("%T changed through DUMP and RESTORE", v)
		}
	}
}
