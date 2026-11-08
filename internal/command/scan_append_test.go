package command

import (
	"bytes"
	"strconv"
	"strings"
	"testing"

	"github.com/cachestorm/cachestorm/internal/resp"
	"github.com/cachestorm/cachestorm/internal/store"
)

// SCAN fills one pair of slices across every shard it visits, and TYPE
// filters each key by its own entry.
func TestScanAppendsAcrossShards(t *testing.T) {
	s := store.NewStore()
	for i := 0; i < 2000; i++ {
		k := "key:" + strconv.Itoa(i)
		if i%2 == 0 {
			s.Set(k, &store.StringValue{Data: []byte("v")}, store.SetOptions{})
		} else {
			s.Set(k, store.NewHashFrom(map[string][]byte{"f": []byte("1")}), store.SetOptions{})
		}
	}
	ctx := discardCtx("SCAN", bytesArgs("0", "COUNT", "100"), s)
	if n := testing.AllocsPerRun(50, func() { _ = cmdSCAN(ctx) }); n > 4 {
		t.Fatalf("SCAN COUNT 100 made %.0f allocations, want at most 4", n)
	}

	seen := 0
	cursor := "0"
	for {
		var buf bytes.Buffer
		c := &Context{Command: "SCAN", Args: bytesArgs(cursor, "COUNT", "37", "TYPE", "string"), Store: s, Writer: resp.NewWriter(&buf)}
		if err := cmdSCAN(c); err != nil {
			t.Fatal(err)
		}
		lines := strings.Split(buf.String(), "\r\n")
		if len(lines) < 4 {
			t.Fatalf("SCAN reply %q", buf.String())
		}
		cursor = lines[2]
		for _, l := range lines[4:] {
			if !strings.HasPrefix(l, "key:") {
				continue
			}
			i, _ := strconv.Atoi(strings.TrimPrefix(l, "key:"))
			if i%2 != 0 {
				t.Fatalf("TYPE string returned list key %s", l)
			}
			seen++
		}
		if cursor == "0" {
			break
		}
	}
	if seen != 1000 {
		t.Fatalf("full TYPE string scan returned %d keys, want 1000", seen)
	}
}
