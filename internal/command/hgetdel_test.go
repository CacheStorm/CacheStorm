package command

import (
	"reflect"
	"strings"
	"testing"

	"github.com/cachestorm/cachestorm/internal/store"
)

func hgetdelEvents(sub *store.Subscriber) []string {
	var out []string
	for {
		select {
		case f := <-sub.Channel():
			s := string(f)
			if i := strings.LastIndex(s, "__keyevent@0__:"); i >= 0 {
				out = append(out, strings.SplitN(s[i+len("__keyevent@0__:"):], "\r\n", 2)[0])
			}
		default:
			return out
		}
	}
}

func TestHGETDELRepliesDeletesAndPropagates(t *testing.T) {
	r := newHFERun(t)
	sub := store.NewSubscriber(1)
	r.s.GetPubSub().PSubscribe(sub, "*")
	defer r.s.GetPubSub().RemoveSubscriber(sub)
	r.s.SetNotifyFlags(store.NotifyKeyevent | store.NotifyHash | store.NotifyGeneric)

	r.want(":3\r\n", "HSET", "h", "a", "1", "b", "2", "c", "3")
	_ = hgetdelEvents(sub)

	got, prop := r.do("HGETDEL", "h", "FIELDS", "2", "a", "x")
	if got != bulks("1", "<nil>") {
		t.Fatalf("reply %q", got)
	}
	if want := []string{"HDEL h a"}; !reflect.DeepEqual(prop, want) {
		t.Fatalf("propagated %q, want %q", prop, want)
	}
	if ev := hgetdelEvents(sub); !reflect.DeepEqual(ev, []string{"hdel"}) {
		t.Fatalf("events %q", ev)
	}

	got, prop = r.do("HGETDEL", "h", "fields", "1", "x")
	if got != bulks("<nil>") || len(prop) != 0 {
		t.Fatalf("missing field: reply %q, propagated %q", got, prop)
	}
	if ev := hgetdelEvents(sub); len(ev) != 0 {
		t.Fatalf("missing field notified %q", ev)
	}

	got, prop = r.do("HGETDEL", "h", "FIELDS", "3", "b", "c", "b")
	if got != bulks("2", "3", "<nil>") {
		t.Fatalf("reply %q", got)
	}
	if want := []string{"HDEL h b c"}; !reflect.DeepEqual(prop, want) {
		t.Fatalf("propagated %q, want %q", prop, want)
	}
	if ev := hgetdelEvents(sub); !reflect.DeepEqual(ev, []string{"hdel", "del"}) {
		t.Fatalf("events %q", ev)
	}
	r.want(":0\r\n", "EXISTS", "h")

	got, prop = r.do("HGETDEL", "h", "FIELDS", "1", "a")
	if got != bulks("<nil>") || len(prop) != 0 {
		t.Fatalf("missing key: reply %q, propagated %q", got, prop)
	}
}

func TestHGETDELErrors(t *testing.T) {
	r := newHFERun(t)
	r.want("+OK\r\n", "SET", "s", "v")
	r.want(":1\r\n", "HSET", "h", "a", "1")
	for _, tc := range []struct {
		args []string
		want string
	}{
		{[]string{"HGETDEL", "h", "FIELDS", "1"}, "-ERR wrong number of arguments for 'hgetdel' command\r\n"},
		{[]string{"HGETDEL", "h", "BOGUS", "1", "a"}, "-ERR Mandatory argument FIELDS is missing or not at the right position\r\n"},
		{[]string{"HGETDEL", "h", "FIELDS", "0", "a"}, "-ERR Number of fields must be a positive integer\r\n"},
		{[]string{"HGETDEL", "h", "FIELDS", "x", "a"}, "-ERR Number of fields must be a positive integer\r\n"},
		{[]string{"HGETDEL", "h", "FIELDS", "2", "a"}, "-ERR The `numfields` parameter must match the number of arguments\r\n"},
		{[]string{"HGETDEL", "h", "FIELDS", "1", "a", "b"}, "-ERR The `numfields` parameter must match the number of arguments\r\n"},
		{[]string{"HGETDEL", "s", "BOGUS", "1", "a"}, "-WRONGTYPE Operation against a key holding the wrong kind of value\r\n"},
	} {
		got, prop := r.do(tc.args...)
		if got != tc.want {
			t.Errorf("%v: got %q, want %q", tc.args, got, tc.want)
		}
		_ = prop
	}
	r.want("$1\r\n1\r\n", "HGET", "h", "a")
}
