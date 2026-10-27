package store

import (
	"strings"
	"testing"
	"time"
)

func TestNotifyFlagsParseAndFormat(t *testing.T) {
	cases := map[string]string{
		"":           "",
		"KEA":        "AKE",
		"KEAnm":      "AKEm",
		"g$lshzxetd": "A",
		"Elg$K":      "g$lKE",
		"Ex":         "xE",
		"nmK":        "nKm",
		"AAg":        "A",
	}
	for in, want := range cases {
		flags, ok := ParseNotifyFlags(in)
		if !ok {
			t.Fatalf("ParseNotifyFlags(%q) refused", in)
		}
		if got := FormatNotifyFlags(flags); got != want {
			t.Fatalf("FormatNotifyFlags(%q) = %q, want %q", in, got, want)
		}
	}
	for _, bad := range []string{"Q", "KE ", "a"} {
		if _, ok := ParseNotifyFlags(bad); ok {
			t.Fatalf("ParseNotifyFlags(%q) accepted", bad)
		}
	}
}

func subscribeAll(t *testing.T, s *Store) *Subscriber {
	t.Helper()
	sub := NewSubscriber(1)
	s.GetPubSub().PSubscribe(sub, "*")
	t.Cleanup(func() { s.GetPubSub().RemoveSubscriber(sub) })
	return sub
}

func drainFrames(sub *Subscriber) []string {
	var out []string
	for {
		select {
		case f := <-sub.ch:
			out = append(out, string(f))
		default:
			return out
		}
	}
}

func TestNotifyOffAllocatesNothing(t *testing.T) {
	s := NewStore()
	subscribeAll(t, s)
	if n := testing.AllocsPerRun(100, func() { s.Notify(NotifyString, "set", "k") }); n != 0 {
		t.Fatalf("Notify with notifications off allocates %v times", n)
	}
	s.SetNotifyFlags(NotifyKeyevent | NotifyList)
	if n := testing.AllocsPerRun(100, func() { s.Notify(NotifyString, "set", "k") }); n != 0 {
		t.Fatalf("Notify of a disabled class allocates %v times", n)
	}
}

func TestNotifyChannelsAndDatabases(t *testing.T) {
	base := NewStore()
	dbs := NewDatabases(base, 4)
	sub := subscribeAll(t, base)
	base.SetNotifyFlags(NotifyKeyspace | NotifyKeyevent | NotifyGeneric)
	dbs.DB(2).Notify(NotifyGeneric, "del", "k")
	frames := drainFrames(sub)
	if len(frames) != 2 || !strings.Contains(frames[0], "__keyspace@2__:k") || !strings.Contains(frames[1], "__keyevent@2__:del") {
		t.Fatalf("frames %q", frames)
	}
	if err := dbs.Swap(0, 2); err != nil {
		t.Fatal(err)
	}
	base.Notify(NotifyGeneric, "del", "k")
	if frames := drainFrames(sub); len(frames) != 2 || !strings.Contains(frames[0], "__keyspace@2__:k") {
		t.Fatalf("after SWAPDB the old database 0 is 2: %q", frames)
	}
	if dbs.DB(0).DBIndex() != 0 {
		t.Fatalf("swapped database index %d", dbs.DB(0).DBIndex())
	}
}

func TestNotifyExpiredOnlyWhenTheStoreExpires(t *testing.T) {
	s := NewStore()
	sub := subscribeAll(t, s)
	s.SetNotifyFlags(NotifyKeyevent | NotifyExpired)
	if err := s.Set("k", &StringValue{Data: []byte("v")}, SetOptions{TTL: time.Millisecond}); err != nil {
		t.Fatal(err)
	}
	time.Sleep(5 * time.Millisecond)

	// A replica keeps expired keys until the master's DEL: no event.
	s.passiveExpiry.Store(true)
	if _, ok := s.Get("k"); ok {
		t.Fatal("expired key visible")
	}
	if frames := drainFrames(sub); len(frames) != 0 {
		t.Fatalf("passive expiry announced %q", frames)
	}
	s.passiveExpiry.Store(false)
	if _, ok := s.Get("k"); ok {
		t.Fatal("expired key visible")
	}
	if frames := drainFrames(sub); len(frames) != 1 || !strings.Contains(frames[0], "__keyevent@0__:expired") {
		t.Fatalf("lazy expiry frames %q", frames)
	}
}

func TestNotifyNewKeys(t *testing.T) {
	s := NewStore()
	sub := subscribeAll(t, s)
	s.SetNotifyFlags(NotifyKeyevent | NotifyNew | NotifyString)
	_ = s.Set("a", &StringValue{Data: []byte("1")}, SetOptions{})
	_ = s.Set("a", &StringValue{Data: []byte("2")}, SetOptions{})
	if _, err := s.SetStringBatch([]StringPair{{Key: "b", Data: []byte("1")}, {Key: "a", Data: []byte("3")}}, false); err != nil {
		t.Fatal(err)
	}
	frames := drainFrames(sub)
	var got []string
	for _, f := range frames {
		i := strings.Index(f, "__keyevent@0__:")
		got = append(got, strings.Fields(strings.ReplaceAll(f[i+len("__keyevent@0__:"):], "\r\n", " "))[0])
	}
	if strings.Join(got, ",") != "new,new,set,set" {
		t.Fatalf("events %v", got)
	}
}
