package command

import (
	"bytes"
	"strconv"
	"testing"

	"github.com/cachestorm/cachestorm/internal/resp"
	"github.com/cachestorm/cachestorm/internal/store"
)

// A scan of a large collection returns every member that is present for the
// whole scan exactly once, whether the membership stays put (pages served
// from the cached order) or changes between pages (pages computed afresh).
func TestCollectionScanWithChangesBetweenPages(t *testing.T) {
	r := newMemAcctRouter()
	run := func(s *store.Store, args ...string) *resp.Value {
		var buf bytes.Buffer
		ctx := NewContext(args[0], migrateBytes(args[1:]), s, resp.NewWriter(&buf))
		if err := r.Execute(ctx); err != nil {
			t.Fatal(err)
		}
		v, err := resp.NewReader(bytes.NewReader(buf.Bytes())).ReadValue()
		if err != nil {
			t.Fatal(err)
		}
		return v
	}
	for _, kind := range []struct{ add, rem, scan string }{
		{"SADD", "SREM", "SSCAN"}, {"HSET", "HDEL", "HSCAN"}, {"ZADD", "ZREM", "ZSCAN"},
	} {
		for _, churn := range []bool{false, true} {
			s := store.NewStore()
			add := func(m string) {
				switch kind.add {
				case "SADD":
					run(s, "SADD", "c", m)
				case "HSET":
					run(s, "HSET", "c", m, "v")
				default:
					run(s, "ZADD", "c", "1", m)
				}
			}
			for i := 0; i < 1000; i++ {
				add("m" + strconv.Itoa(i))
			}
			seen := map[string]int{}
			cursor := "0"
			for page := 0; ; page++ {
				v := run(s, kind.scan, "c", cursor, "COUNT", "37")
				cursor = string(v.Array[0].Bulk)
				items := v.Array[1].Array
				step := 1
				if kind.scan != "SSCAN" {
					step = 2
				}
				for i := 0; i < len(items); i += step {
					seen[string(items[i].Bulk)]++
				}
				if churn {
					add("new" + strconv.Itoa(page))
					run(s, kind.rem, "c", "m"+strconv.Itoa(900+page%100))
				}
				if cursor == "0" {
					break
				}
			}
			for i := 0; i < 900; i++ {
				m := "m" + strconv.Itoa(i)
				if seen[m] != 1 {
					t.Fatalf("%s churn=%v: %s returned %d times", kind.scan, churn, m, seen[m])
				}
			}
		}
	}
}
