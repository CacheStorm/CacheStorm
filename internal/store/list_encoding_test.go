package store

import (
	"bytes"
	"testing"
)

func listArgs(vals ...string) [][]byte {
	out := make([][]byte, len(vals))
	for i, v := range vals {
		out[i] = []byte(v)
	}
	return out
}

func wantListEncoding(t *testing.T, step string, v *ListValue, want string) {
	t.Helper()
	if got := v.Encoding(); got != want {
		t.Fatalf("%s: encoding = %s, want %s", step, got, want)
	}
}

// The expectations below are Redis 7.4's replies to the same commands.
func TestListEncodingFollowsRedis(t *testing.T) {
	defer SetListMaxListpackSize(-2)

	SetListMaxListpackSize(4)
	q := &ListValue{}
	q.PushBack(listArgs("1", "2", "3", "4", "5")...)
	wantListEncoding(t, "RPUSH q 1..5", q, "quicklist")
	q.PopBack(1)
	wantListEncoding(t, "RPOP q", q, "quicklist")
	q.PopBack(2)
	wantListEncoding(t, "RPOP q 2", q, "listpack")

	r := &ListValue{}
	r.PushBack(listArgs("1", "2", "3", "4")...)
	r.PushBack(listArgs("5", "6", "7", "8", "9")...)
	r.TryGrow([]byte("x"))
	r.InsertAt(2, []byte("x"), false)
	wantListEncoding(t, "LINSERT r BEFORE 3 x", r, "quicklist")
	if got := bytes.Join(r.PopFront(8), []byte(" ")); string(got) != "1 2 x 3 4 5 6 7" {
		t.Fatalf("LPOP r 8 = %q", got)
	}
	wantListEncoding(t, "LPOP r 8", r, "quicklist")

	SetListMaxListpackSize(-2)
	big := &ListValue{}
	for i := 0; i < 40; i++ {
		big.PushBack(bytes.Repeat([]byte("w"), 300))
	}
	wantListEncoding(t, "RPUSH 40 x 300 bytes", big, "quicklist")
	big.Trim(0, 5)
	wantListEncoding(t, "LTRIM 0 5", big, "listpack")
}

func TestListEncodingLargeElementPinsQuicklist(t *testing.T) {
	v := &ListValue{}
	v.PushBack(listArgs("a", "b")...)
	huge := bytes.Repeat([]byte("z"), 9000)
	v.TryGrow(huge)
	v.SetAt(0, huge)
	wantListEncoding(t, "LSET of a 9000-byte element", v, "quicklist")
	v.DeleteAt([]int{0})
	wantListEncoding(t, "LREM of the large element", v, "listpack")
}

func TestLpEntrySize(t *testing.T) {
	for _, c := range []struct {
		el   string
		want int
	}{
		{"0", 2}, {"127", 2}, {"128", 3}, {"-4096", 3}, {"32767", 4},
		{"8388607", 5}, {"2147483647", 6}, {"9223372036854775807", 10},
		{"007", 5}, {"-0", 4}, {"", 2}, {string(bytes.Repeat([]byte("v"), 63)), 65},
		{string(bytes.Repeat([]byte("v"), 64)), 67},
	} {
		if got := lpEntrySize([]byte(c.el)); got != c.want {
			t.Errorf("lpEntrySize(%q) = %d, want %d", c.el, got, c.want)
		}
	}
}
