package command

import (
	"bytes"
	"fmt"
	"strconv"
	"testing"

	"github.com/cachestorm/cachestorm/internal/resp"
	"github.com/cachestorm/cachestorm/internal/store"
)

func chunkRouter() (*store.Store, *Router) {
	r := NewRouter()
	RegisterProbabilisticCommands(r)
	RegisterRedisBloomChunkCommands(r)
	RegisterStringCommands(r)
	return store.NewStore(), r
}

func chunkCmd(t *testing.T, s *store.Store, r *Router, args ...string) *resp.Value {
	t.Helper()
	raw := make([][]byte, len(args)-1)
	for i, a := range args[1:] {
		raw[i] = []byte(a)
	}
	var b bytes.Buffer
	if err := r.Execute(NewContext(args[0], raw, s, resp.NewWriter(&b))); err != nil {
		t.Fatal(err)
	}
	v, err := resp.NewReader(&b).ReadValue()
	if err != nil {
		t.Fatal(err)
	}
	return v
}

// Redis 8.2's replies for the same steps.
func TestScanDumpRedisReplies(t *testing.T) {
	s, r := chunkRouter()
	chunkCmd(t, s, r, "BF.RESERVE", "b", "0.01", "100")
	chunkCmd(t, s, r, "BF.ADD", "b", "x")
	hdr := chunkCmd(t, s, r, "BF.SCANDUMP", "b", "0")
	want := "\x01\x00\x00\x00\x00\x00\x00\x00\x01\x00\x00\x00\x05\x00\x00\x00\x02\x00\x00\x00\x90\x00\x00\x00\x00\x00\x00\x00\x80\x04\x00\x00\x00\x00\x00\x00\x01\x00\x00\x00\x00\x00\x00\x00{\x14\xaeG\xe1zt?\xe9\x86/\xb25\x0e&@\x08\x00\x00\x00d\x00\x00\x00\x00\x00\x00\x00\x00"
	if hdr.Array[0].Int != 1 || string(hdr.Array[1].Bulk) != want {
		t.Fatalf("BF.SCANDUMP b 0 = %d %q", hdr.Array[0].Int, hdr.Array[1].Bulk)
	}
	if got := chunkCmd(t, s, r, "BF.SCANDUMP", "b", "121"); got.Array[0].Int != 145 || len(got.Array[1].Bulk) != 24 {
		t.Fatalf("BF.SCANDUMP b 121 = %+v", got)
	}
	if got := chunkCmd(t, s, r, "BF.DEBUG", "b"); string(got.Array[1].Bulk) != "bytes:144 bits:1152 hashes:8 hashwidth:64 capacity:100 size:1 ratio:0.005" {
		t.Fatalf("BF.DEBUG = %+v", got)
	}
	chunkCmd(t, s, r, "CF.RESERVE", "c", "10")
	if got := chunkCmd(t, s, r, "CF.SCANDUMP", "c", "0"); got.Array[0].Int != 0 || !got.Array[1].IsNull {
		t.Fatalf("CF.SCANDUMP of an empty filter = %+v", got)
	}
	chunkCmd(t, s, r, "CF.ADD", "c", "x")
	if got := chunkCmd(t, s, r, "CF.SCANDUMP", "c", "0"); string(got.Array[1].Bulk) != "\x01\x00\x00\x00\x00\x00\x00\x00\x08\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x01\x00\x00\x00\x00\x00\x00\x00\x02\x00\x14\x00\x01\x00" {
		t.Fatalf("CF.SCANDUMP c 0 = %q", got.Array[1].Bulk)
	}
	if got := chunkCmd(t, s, r, "CF.SCANDUMP", "c", "1"); got.Array[0].Int != 17 || string(got.Array[1].Bulk) != "\x00\x00\x0f\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00" {
		t.Fatalf("CF.SCANDUMP c 1 = %+v", got)
	}
	if got := chunkCmd(t, s, r, "CF.DEBUG", "c"); string(got.Bulk) != "bktsize:2 buckets:8 items:1 deletes:0 filters:1 max_iterations:20 expansion:1" {
		t.Fatalf("CF.DEBUG = %+v", got)
	}
	chunkCmd(t, s, r, "SET", "s", "x")
	for _, tc := range []struct {
		err  string
		args []string
	}{
		{"Second argument must be numeric", []string{"BF.SCANDUMP", "b", "abc"}},
		{"ERR not found", []string{"BF.SCANDUMP", "none", "0"}},
		{"Invalid position", []string{"CF.SCANDUMP", "c", "-1"}},
		{"Cuckoo filter was not found", []string{"CF.COMPACT", "none"}},
		{"Cuckoo filter was not found", []string{"CF.COMPACT", "s"}},
		{"ERR item exists", []string{"CF.LOADCHUNK", "c", "1", "abc"}},
		{"Invalid position", []string{"CF.LOADCHUNK", "none", "0", "abc"}},
		{"ERR not found", []string{"CF.LOADCHUNK", "none", "5", "abc"}},
		{"Invalid header", []string{"CF.LOADCHUNK", "none", "1", "abc"}},
		{"ERR received bad data", []string{"BF.LOADCHUNK", "none", "1", "abc"}},
		{"ERR not found", []string{"BF.LOADCHUNK", "none", "5", "abc"}},
		{"ERR received bad data", []string{"BF.LOADCHUNK", "b", "1", "abc"}},
		{"ERR invalid offset - no link found", []string{"BF.LOADCHUNK", "b", "9999", "abc"}},
		{"ERR Second argument must be numeric", []string{"BF.LOADCHUNK", "b", "x", "abc"}},
		{"WRONGTYPE Operation against a key holding the wrong kind of value", []string{"BF.SCANDUMP", "s", "0"}},
		{"WRONGTYPE Operation against a key holding the wrong kind of value", []string{"CF.LOADCHUNK", "s", "1", "x"}},
	} {
		if got := chunkCmd(t, s, r, tc.args...); got.Err != tc.err {
			t.Errorf("%v = %+v, want error %q", tc.args, got, tc.err)
		}
	}
}

// Filters copied chunk by chunk answer as the originals do.
func TestScanDumpLoadChunkCopiesFilters(t *testing.T) {
	s, r := chunkRouter()
	chunkCmd(t, s, r, "BF.RESERVE", "b", "0.01", "50", "EXPANSION", "4")
	chunkCmd(t, s, r, "CF.RESERVE", "c", "64", "BUCKETSIZE", "4", "EXPANSION", "2")
	for i := 0; i < 700; i++ {
		chunkCmd(t, s, r, "BF.ADD", "b", strconv.Itoa(i))
		chunkCmd(t, s, r, "CF.ADD", "c", strconv.Itoa(i))
	}
	for _, kind := range []string{"BF", "CF"} {
		src, dst := map[string]string{"BF": "b", "CF": "c"}[kind], "copy"+kind
		for it := int64(0); ; {
			got := chunkCmd(t, s, r, kind+".SCANDUMP", src, strconv.FormatInt(it, 10))
			if it = got.Array[0].Int; it == 0 {
				break
			}
			if ok := chunkCmd(t, s, r, kind+".LOADCHUNK", dst, strconv.FormatInt(it, 10), string(got.Array[1].Bulk)); ok.Str != "OK" {
				t.Fatalf("%s.LOADCHUNK = %+v", kind, ok)
			}
		}
		for i := 0; i < 2000; i += 7 {
			a := chunkCmd(t, s, r, kind+".EXISTS", src, strconv.Itoa(i))
			b := chunkCmd(t, s, r, kind+".EXISTS", dst, strconv.Itoa(i))
			if a.Int != b.Int {
				t.Fatalf("%s.EXISTS %d: %d vs copy %d", kind, i, a.Int, b.Int)
			}
		}
		if a, b := fmt.Sprint(chunkCmd(t, s, r, kind+".INFO", src)), fmt.Sprint(chunkCmd(t, s, r, kind+".INFO", dst)); a != b {
			t.Fatalf("%s.INFO %s vs copy %s", kind, a, b)
		}
	}
}

func TestJSONNumPowByRedisReplies(t *testing.T) {
	s := store.NewStore()
	r := NewRouter()
	RegisterJSONCommands(r)
	for _, tc := range []struct{ want, args string }{
		{"+OK\r\n", `JSON.SET k $ {"a":2,"b":[1,2.5,"x"],"c":{"a":3},"d":-8}`},
		{"$3\r\n[8]\r\n", "JSON.NUMPOWBY k $.a 3"},
		{"$6\r\n[64,9]\r\n", "JSON.NUMPOWBY k $..a 2"},
		{"$13\r\n[1,6.25,null]\r\n", "JSON.NUMPOWBY k $.b[*] 2"},
		{"$3\r\n8.0\r\n", "JSON.NUMPOWBY k .a 0.5"},
		{"-result is not a number\r\n", "JSON.NUMPOWBY k $.d 0.5"},
		{"$2\r\n[]\r\n", "JSON.NUMPOWBY k $.nope abc"},
		{"-ERR expected value at line 1 column 1\r\n", "JSON.NUMPOWBY k $.a x"},
	} {
		args := splitJSONArgs(tc.args)
		if got := runCmd(t, s, r, args[0], args[1:]...); got != tc.want {
			t.Errorf("%s\n got %q\nwant %q", tc.args, got, tc.want)
		}
	}
}

// splitJSONArgs splits on spaces, keeping a final JSON document whole.
func splitJSONArgs(s string) []string {
	var out []string
	for len(s) > 0 && s[0] != '{' {
		i := bytes.IndexByte([]byte(s), ' ')
		if i < 0 {
			return append(out, s)
		}
		out = append(out, s[:i])
		s = s[i+1:]
	}
	if s != "" {
		out = append(out, s)
	}
	return out
}
