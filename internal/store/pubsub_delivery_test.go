package store

import (
	"reflect"
	"sort"
	"testing"
	"time"
)

func recvFrame(t *testing.T, sub *Subscriber) string {
	t.Helper()
	select {
	case f, ok := <-sub.Channel():
		if !ok {
			t.Fatal("subscriber queue closed")
		}
		sub.Delivered(f)
		return string(f)
	default:
		t.Fatal("no frame queued")
	}
	return ""
}

func expectEmpty(t *testing.T, sub *Subscriber) {
	t.Helper()
	select {
	case f := <-sub.Channel():
		t.Fatalf("unexpected frame %q", f)
	default:
	}
}

func TestPublishBuildsMessageFrames(t *testing.T) {
	ps := NewPubSub()
	sub := NewSubscriber(1)
	ps.Subscribe(sub, "news.a")
	ps.PSubscribe(sub, "news.*")

	if n := ps.Publish("news.a", []byte("x")); n != 2 {
		t.Fatalf("Publish = %d, want 2 (channel + pattern)", n)
	}
	if got, want := recvFrame(t, sub), "*3\r\n$7\r\nmessage\r\n$6\r\nnews.a\r\n$1\r\nx\r\n"; got != want {
		t.Fatalf("message frame %q, want %q", got, want)
	}
	if got, want := recvFrame(t, sub), "*4\r\n$8\r\npmessage\r\n$6\r\nnews.*\r\n$6\r\nnews.a\r\n$1\r\nx\r\n"; got != want {
		t.Fatalf("pmessage frame %q, want %q", got, want)
	}
	expectEmpty(t, sub)
}

func TestPublishBinaryPayloadFrame(t *testing.T) {
	ps := NewPubSub()
	sub := NewSubscriber(1)
	ps.Subscribe(sub, "ch")
	ps.Publish("ch", []byte("a\x00b\r\nc"))
	if got, want := recvFrame(t, sub), "*3\r\n$7\r\nmessage\r\n$2\r\nch\r\n$6\r\na\x00b\r\nc\r\n"; got != want {
		t.Fatalf("frame %q, want %q", got, want)
	}
}

func TestShardChannelsAreSeparateNamespace(t *testing.T) {
	ps := NewPubSub()
	plain, shard := NewSubscriber(1), NewSubscriber(2)
	ps.Subscribe(plain, "sh")
	acks := ps.SSubscribe(shard, "sh", "other")
	if want := []SubAck{{"sh", 1}, {"other", 2}}; !reflect.DeepEqual(acks, want) {
		t.Fatalf("SSubscribe acks %v, want %v", acks, want)
	}

	if n := ps.SPublish("sh", []byte("x")); n != 1 {
		t.Fatalf("SPublish = %d, want 1", n)
	}
	if got, want := recvFrame(t, shard), "*3\r\n$8\r\nsmessage\r\n$2\r\nsh\r\n$1\r\nx\r\n"; got != want {
		t.Fatalf("smessage frame %q, want %q", got, want)
	}
	expectEmpty(t, plain)

	if n := ps.Publish("sh", []byte("y")); n != 1 {
		t.Fatalf("Publish = %d, want 1 (plain subscriber only)", n)
	}
	expectEmpty(t, shard)
	recvFrame(t, plain)

	got := ps.ShardChannels("")
	sort.Strings(got)
	if want := []string{"other", "sh"}; !reflect.DeepEqual(got, want) {
		t.Fatalf("ShardChannels = %v, want %v", got, want)
	}
	if got := ps.ShardChannels("o*"); !reflect.DeepEqual(got, []string{"other"}) {
		t.Fatalf("ShardChannels(o*) = %v", got)
	}
	if got := ps.ShardNumSub("sh", "none"); got["sh"] != 1 || got["none"] != 0 {
		t.Fatalf("ShardNumSub = %v", got)
	}
	if got := ps.Channels(""); !reflect.DeepEqual(got, []string{"sh"}) {
		t.Fatalf("Channels = %v, want only the plain channel", got)
	}

	if shard.ShardSubscriptionCount() != 2 || shard.SubscriptionCount() != 0 || shard.TotalSubscriptions() != 2 {
		t.Fatal("shard counts are tracked apart from channel+pattern counts")
	}
	acks = ps.SUnsubscribe(shard)
	if want := []SubAck{{"other", 1}, {"sh", 0}}; !reflect.DeepEqual(acks, want) {
		t.Fatalf("SUnsubscribe-all acks %v, want %v", acks, want)
	}
	if ps.SPublish("sh", []byte("z")) != 0 {
		t.Fatal("delivery after SUNSUBSCRIBE")
	}
}

func TestSubscribeAckCountsAreTotals(t *testing.T) {
	ps := NewPubSub()
	sub := NewSubscriber(1)

	if got, want := ps.SubscribeChannels(sub, "a", "b", "a"), []SubAck{{"a", 1}, {"b", 2}, {"a", 2}}; !reflect.DeepEqual(got, want) {
		t.Fatalf("SUBSCRIBE acks %v, want %v", got, want)
	}
	if got, want := ps.SubscribePatterns(sub, "p*"), []SubAck{{"p*", 3}}; !reflect.DeepEqual(got, want) {
		t.Fatalf("PSUBSCRIBE acks %v, want %v", got, want)
	}
	if got, want := ps.UnsubscribeChannels(sub, "zz", "a"), []SubAck{{"zz", 3}, {"a", 2}}; !reflect.DeepEqual(got, want) {
		t.Fatalf("UNSUBSCRIBE acks %v, want %v", got, want)
	}
	if got, want := ps.UnsubscribeChannels(sub), []SubAck{{"b", 1}}; !reflect.DeepEqual(got, want) {
		t.Fatalf("UNSUBSCRIBE-all acks %v, want %v", got, want)
	}
	if got := ps.UnsubscribeChannels(sub); len(got) != 0 {
		t.Fatalf("UNSUBSCRIBE-all with nothing left: %v, want none", got)
	}
	if got, want := ps.UnsubscribePatterns(sub), []SubAck{{"p*", 0}}; !reflect.DeepEqual(got, want) {
		t.Fatalf("PUNSUBSCRIBE-all acks %v, want %v", got, want)
	}
	if sub.TotalSubscriptions() != 0 || len(ps.subscribers) != 0 {
		t.Fatal("subscriber still registered after leaving everything")
	}
}

func TestRemoveSubscriberDropsShardSubscriptions(t *testing.T) {
	ps := NewPubSub()
	sub := NewSubscriber(1)
	ps.SSubscribe(sub, "s")
	ps.Subscribe(sub, "c")
	ps.PSubscribe(sub, "p")
	ps.RemoveSubscriber(sub)
	if len(ps.shards) != 0 || len(ps.channels) != 0 || len(ps.patterns) != 0 || len(ps.subscribers) != 0 {
		t.Fatal("RemoveSubscriber left registrations behind")
	}
	if sub.TotalSubscriptions() != 0 {
		t.Fatal("counts not reset")
	}
}

func TestSubscriberOverflowMarksAndNotifies(t *testing.T) {
	prevQ, prevB := SetSubscriberLimits(2, 0)
	sub := NewSubscriber(1)
	SetSubscriberLimits(prevQ, prevB)

	fired := make(chan struct{}, 2)
	sub.SetOverflowHandler(func() { fired <- struct{}{} })

	ps := NewPubSub()
	ps.Subscribe(sub, "ch")
	for i := 0; i < 2; i++ {
		if ps.Publish("ch", []byte("m")) != 1 {
			t.Fatal("delivery before the queue is full")
		}
	}
	if ps.Publish("ch", []byte("m")) != 0 {
		t.Fatal("overflowing publish counted as delivered")
	}
	select {
	case <-sub.Overflowed():
	default:
		t.Fatal("Overflowed not closed")
	}
	select {
	case <-fired:
	case <-time.After(2 * time.Second):
		t.Fatal("overflow handler not called")
	}
	if ps.Publish("ch", []byte("m")) != 0 {
		t.Fatal("overflowed subscriber still accepting frames")
	}
	select {
	case <-fired:
		t.Fatal("overflow handler called twice")
	case <-time.After(50 * time.Millisecond):
	}

	late := make(chan struct{})
	sub.SetOverflowHandler(func() { close(late) })
	select {
	case <-late:
	case <-time.After(2 * time.Second):
		t.Fatal("handler registered after overflow not called")
	}
}

func TestSubscriberPendingBytesBound(t *testing.T) {
	prevQ, prevB := SetSubscriberLimits(0, 100)
	sub := NewSubscriber(1)
	SetSubscriberLimits(prevQ, prevB)

	payload := make([]byte, 60)
	if !sub.Send(payload) {
		t.Fatal("first frame within the byte bound rejected")
	}
	if sub.Send(payload) {
		t.Fatal("frame past the byte bound accepted")
	}
	if !sub.IsOverflowed() {
		t.Fatal("byte overflow not marked")
	}
}

func TestSubscriberDeliveredReleasesBytes(t *testing.T) {
	prevQ, prevB := SetSubscriberLimits(0, 100)
	sub := NewSubscriber(1)
	SetSubscriberLimits(prevQ, prevB)

	payload := make([]byte, 60)
	for i := 0; i < 5; i++ {
		if !sub.Send(payload) {
			t.Fatalf("send %d rejected although the consumer keeps up", i)
		}
		sub.Delivered(<-sub.Channel())
	}
}

func TestMatchPatternRedisGlob(t *testing.T) {
	cases := []struct {
		pattern, s string
		want       bool
	}{
		{"*", "", false},
		{"*", "anything", true},
		{"news.*", "news.tech", true},
		{"news.*", "news", false},
		{"h?llo", "hello", true},
		{"h?llo", "hllo", false},
		{"h[ae]llo", "hallo", true},
		{"h[ae]llo", "hillo", false},
		{"h[^e]llo", "hallo", true},
		{"h[^e]llo", "hello", false},
		{"h[a-b]llo", "hbllo", true},
		{"h[b-a]llo", "hallo", true},
		{"h[a-b]llo", "hcllo", false},
		{`h\*llo`, "h*llo", true},
		{`h\*llo`, "hello", false},
		{`h[\]]llo`, "h]llo", true},
		{`a\`, `a\`, true},
		{"a[bc", "ab", true},
		{"a[bc", "ac", true},
		{"a[bc", "ad", false},
		{"**a", "ba", true},
		{"a*b*c", "axxbyyc", true},
		{"a*b*c", "axxbyy", false},
		{"", "", true},
		{"", "a", false},
		{"abc", "abc", true},
		{"abc*", "abc", true},
	}
	for _, c := range cases {
		if got := matchPattern(c.s, c.pattern); got != c.want {
			t.Errorf("match(%q, %q) = %v, want %v", c.pattern, c.s, got, c.want)
		}
	}
}
