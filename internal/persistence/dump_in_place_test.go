package persistence

import (
	"bytes"
	"fmt"
	"strings"
	"testing"

	"github.com/cachestorm/cachestorm/internal/store"
)

// Strings and hashes are decoded from the payload in place and copied only
// into the value the store keeps: nothing decoded may share the payload's
// bytes, which RESTORE's argument buffer and a loaded file's buffer are
// reused or dropped after the load.
func TestDecodedValuesDoNotSharePayload(t *testing.T) {
	hash := func(n, valLen int) store.Value {
		items := make([]string, 0, 2*n)
		for i := range n {
			items = append(items, fmt.Sprint("field-", i), fmt.Sprint(strings.Repeat("q", valLen), i))
		}
		v, ok := store.NewLoadedHash(items)
		if !ok {
			t.Fatal("NewLoadedHash refused unique fields")
		}
		return v
	}
	set := &store.SetValue{}
	for _, m := range []string{"alpha", "beta", "gamma"} {
		set.AddMember(m)
	}
	zset := store.NewSortedSet(2)
	zset.Put("one", 1)
	zset.Put("two", 2)
	for _, tc := range []struct {
		name  string
		value store.Value
	}{
		{"plain string", &store.StringValue{Data: []byte("not compressible, short")}},
		{"compressed string", &store.StringValue{Data: []byte(strings.Repeat("abc", 40))}},
		{"integer string", &store.StringValue{Data: []byte("12345")}},
		{"listpack hash", hash(10, 5)},
		{"hashtable hash", hash(300, 5)},
		{"long-value hash", hash(3, 100)},
		{"listpack set", set},
		{"listpack zset", zset},
	} {
		t.Run(tc.name, func(t *testing.T) {
			payload, err := EncodeDump(tc.value)
			if err != nil {
				t.Fatal(err)
			}
			want := bytes.Clone(payload)
			decoded, err := DecodeDump(payload)
			if err != nil {
				t.Fatal(err)
			}
			for i := range payload {
				payload[i] = 'X'
			}
			got, err := EncodeDump(decoded)
			if err != nil {
				t.Fatal(err)
			}
			if !bytes.Equal(got, want) {
				t.Fatalf("value changed with the payload:\n got %q\nwant %q", got, want)
			}
		})
	}
}
