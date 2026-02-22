package store

import (
	"testing"
	"time"
)

func TestPubSubSubscribe(t *testing.T) {
	ps := NewPubSub()
	sub := NewSubscriber(1)

	count := ps.Subscribe(sub, "channel1", "channel2")
	if count != 2 {
		t.Errorf("expected 2 subscriptions, got %d", count)
	}

	count = ps.Subscribe(sub, "channel1")
	if count != 1 {
		t.Errorf("expected 1 subscription (duplicate), got %d", count)
	}
}

func TestPubSubUnsubscribe(t *testing.T) {
	ps := NewPubSub()
	sub := NewSubscriber(1)

	ps.Subscribe(sub, "channel1", "channel2")

	count := ps.Unsubscribe(sub, "channel1")
	if count != 1 {
		t.Errorf("expected 1 unsubscription, got %d", count)
	}

	count = ps.Unsubscribe(sub, "channel2")
	if count != 1 {
		t.Errorf("expected 1 unsubscription, got %d", count)
	}
}

func TestPubSubPSubscribe(t *testing.T) {
	ps := NewPubSub()
	sub := NewSubscriber(1)

	count := ps.PSubscribe(sub, "news:*", "sports:*")
	if count != 2 {
		t.Errorf("expected 2 pattern subscriptions, got %d", count)
	}
}

func TestPubSubPUnsubscribe(t *testing.T) {
	ps := NewPubSub()
	sub := NewSubscriber(1)

	ps.PSubscribe(sub, "news:*", "sports:*")

	count := ps.PUnsubscribe(sub, "news:*")
	if count != 1 {
		t.Errorf("expected 1 pattern unsubscription, got %d", count)
	}

	count = ps.PUnsubscribe(sub, "sports:*")
	if count != 1 {
		t.Errorf("expected 1 pattern unsubscription, got %d", count)
	}
}

func TestPubSubPublish(t *testing.T) {
	ps := NewPubSub()
	sub1 := NewSubscriber(1)
	sub2 := NewSubscriber(2)

	ps.Subscribe(sub1, "channel1")
	ps.Subscribe(sub2, "channel1")

	count := ps.Publish("channel1", []byte("message"))
	if count != 2 {
		t.Errorf("expected 2 deliveries, got %d", count)
	}
}

func TestPubSubPublishPattern(t *testing.T) {
	ps := NewPubSub()
	sub := NewSubscriber(1)

	ps.PSubscribe(sub, "news:*")

	count := ps.Publish("news:sports", []byte("message"))
	if count != 1 {
		t.Errorf("expected 1 delivery via pattern, got %d", count)
	}
}

func TestPubSubChannels(t *testing.T) {
	ps := NewPubSub()
	sub := NewSubscriber(1)

	ps.Subscribe(sub, "channel1", "channel2", "other")

	channels := ps.Channels("channel*")
	if len(channels) != 2 {
		t.Errorf("expected 2 channels matching pattern, got %d", len(channels))
	}

	channels = ps.Channels("")
	if len(channels) != 3 {
		t.Errorf("expected 3 channels (all), got %d", len(channels))
	}
}

func TestPubSubNumSub(t *testing.T) {
	ps := NewPubSub()
	sub1 := NewSubscriber(1)
	sub2 := NewSubscriber(2)

	ps.Subscribe(sub1, "channel1")
	ps.Subscribe(sub2, "channel1")
	ps.Subscribe(sub1, "channel2")

	result := ps.NumSub("channel1", "channel2", "nonexistent")
	if result["channel1"] != 2 {
		t.Errorf("expected 2 subscribers for channel1, got %d", result["channel1"])
	}
	if result["channel2"] != 1 {
		t.Errorf("expected 1 subscriber for channel2, got %d", result["channel2"])
	}
	if result["nonexistent"] != 0 {
		t.Errorf("expected 0 subscribers for nonexistent, got %d", result["nonexistent"])
	}
}

func TestPubSubNumPat(t *testing.T) {
	ps := NewPubSub()
	sub1 := NewSubscriber(1)
	sub2 := NewSubscriber(2)

	ps.PSubscribe(sub1, "news:*", "sports:*")
	ps.PSubscribe(sub2, "news:*")

	count := ps.NumPat()
	if count != 3 {
		t.Errorf("expected 3 pattern subscriptions, got %d", count)
	}
}

func TestPubSubRemoveSubscriber(t *testing.T) {
	ps := NewPubSub()
	sub := NewSubscriber(1)

	ps.Subscribe(sub, "channel1")
	ps.PSubscribe(sub, "pattern:*")

	ps.RemoveSubscriber(sub)

	count := ps.Publish("channel1", []byte("message"))
	if count != 0 {
		t.Errorf("expected 0 deliveries after removal, got %d", count)
	}
}

func TestSubscriberSend(t *testing.T) {
	sub := NewSubscriber(1)

	if !sub.Send([]byte("message1")) {
		t.Error("send should succeed")
	}

	if !sub.Send([]byte("message2")) {
		t.Error("send should succeed")
	}

	sub.Close()

	if sub.Send([]byte("message3")) {
		t.Error("send should fail after close")
	}
}

func TestSubscriberChannel(t *testing.T) {
	sub := NewSubscriber(1)

	ch := sub.Channel()
	if ch == nil {
		t.Error("channel should not be nil")
	}
}

func TestSubscriberCloseTwice(t *testing.T) {
	sub := NewSubscriber(1)

	sub.Close()
	sub.Close()
}

func TestMatchPattern(t *testing.T) {
	tests := []struct {
		s       string
		pattern string
		match   bool
	}{
		{"hello", "hello", true},
		{"hello", "*", true},
		{"hello", "h*", true},
		{"hello", "*o", true},
		{"hello", "h*o", true},
		{"hello", "h?llo", true},
		{"hello", "h?ll?", true},
		{"hello", "world", false},
		{"hello", "h?p", false},
		{"news:sports", "news:*", true},
		{"news:sports:latest", "news:*", true},
		{"abc", "a?c", true},
		{"abc", "a??", true},
		{"ab", "a??", false},
	}

	for _, tt := range tests {
		result := matchPattern(tt.s, tt.pattern)
		if result != tt.match {
			t.Errorf("matchPattern(%q, %q) = %v, expected %v", tt.s, tt.pattern, result, tt.match)
		}
	}
}

func TestTagIndexAddTags(t *testing.T) {
	ti := NewTagIndex()

	ti.AddTags("key1", []string{"tag1", "tag2"})
	ti.AddTags("key2", []string{"tag1"})

	if ti.Count("tag1") != 2 {
		t.Errorf("expected 2 keys with tag1, got %d", ti.Count("tag1"))
	}

	if ti.Count("tag2") != 1 {
		t.Errorf("expected 1 key with tag2, got %d", ti.Count("tag2"))
	}
}

func TestTagIndexRemoveTags(t *testing.T) {
	ti := NewTagIndex()

	ti.AddTags("key1", []string{"tag1", "tag2"})
	ti.RemoveTags("key1", []string{"tag1"})

	if ti.Count("tag1") != 0 {
		t.Errorf("expected 0 keys with tag1, got %d", ti.Count("tag1"))
	}

	if ti.Count("tag2") != 1 {
		t.Errorf("expected 1 key with tag2, got %d", ti.Count("tag2"))
	}
}

func TestTagIndexRemoveKeyAll(t *testing.T) {
	ti := NewTagIndex()

	ti.AddTags("key1", []string{"tag1", "tag2"})
	ti.RemoveKey("key1", []string{"tag1", "tag2"})

	if ti.Count("tag1") != 0 {
		t.Errorf("expected 0 keys with tag1, got %d", ti.Count("tag1"))
	}
}

func TestTagIndexRemoveKeyWithoutTags(t *testing.T) {
	ti := NewTagIndex()

	ti.AddTags("key1", []string{"tag1"})
	ti.RemoveKey("key1", nil)

	if ti.Count("tag1") != 0 {
		t.Errorf("expected 0 keys after RemoveKey without tags, got %d", ti.Count("tag1"))
	}
}

func TestTagIndexGetKeys(t *testing.T) {
	ti := NewTagIndex()

	ti.AddTags("key1", []string{"tag1"})
	ti.AddTags("key2", []string{"tag1"})

	keys := ti.GetKeys("tag1")
	if len(keys) != 2 {
		t.Errorf("expected 2 keys, got %d", len(keys))
	}
}

func TestTagIndexInvalidateTag(t *testing.T) {
	ti := NewTagIndex()

	ti.AddTags("key1", []string{"tag1"})
	ti.AddTags("key2", []string{"tag1"})

	keys := ti.Invalidate("tag1")
	if len(keys) != 2 {
		t.Errorf("expected 2 invalidated keys, got %d", len(keys))
	}

	if ti.Count("tag1") != 0 {
		t.Errorf("expected 0 keys after invalidate, got %d", ti.Count("tag1"))
	}
}

func TestTagIndexTags(t *testing.T) {
	ti := NewTagIndex()

	ti.AddTags("key1", []string{"tag1", "tag2"})
	ti.AddTags("key2", []string{"tag3"})

	tags := ti.Tags()
	if len(tags) != 3 {
		t.Errorf("expected 3 tags, got %d", len(tags))
	}
}

func TestTagIndexHierarchy(t *testing.T) {
	ti := NewTagIndex()

	ti.Link("parent", "child1")
	ti.Link("parent", "child2")

	children := ti.GetChildren("parent")
	if len(children) != 2 {
		t.Errorf("expected 2 children, got %d", len(children))
	}

	descendants := ti.GetAllDescendants("parent")
	if len(descendants) != 2 {
		t.Errorf("expected 2 descendants, got %d", len(descendants))
	}

	ti.Unlink("parent", "child1")
	children = ti.GetChildren("parent")
	if len(children) != 1 {
		t.Errorf("expected 1 child after unlink, got %d", len(children))
	}
}

func TestTagIndexInvalidateCascade(t *testing.T) {
	ti := NewTagIndex()

	ti.Link("parent", "child")
	ti.AddTags("key1", []string{"parent"})
	ti.AddTags("key2", []string{"child"})

	result := ti.InvalidateCascade("parent")
	if len(result) != 2 {
		t.Errorf("expected 2 tags invalidated, got %d", len(result))
	}
	if len(result["parent"]) != 1 {
		t.Errorf("expected 1 key in parent, got %d", len(result["parent"]))
	}
	if len(result["child"]) != 1 {
		t.Errorf("expected 1 key in child, got %d", len(result["child"]))
	}
}

func TestNamespaceManagerGet(t *testing.T) {
	nm := NewNamespaceManager()

	if nm.Get("default") == nil {
		t.Error("default namespace should exist")
	}

	if nm.Default() == nil {
		t.Error("default should return namespace")
	}
}

func TestNamespaceManagerGetOrCreate(t *testing.T) {
	nm := NewNamespaceManager()

	ns1 := nm.GetOrCreate("test")
	if ns1 == nil {
		t.Error("namespace should be created")
	}

	ns2 := nm.GetOrCreate("test")
	if ns1 != ns2 {
		t.Error("GetOrCreate should return same namespace")
	}
}

func TestNamespaceManagerDeleteNS(t *testing.T) {
	nm := NewNamespaceManager()

	nm.GetOrCreate("test")
	err := nm.Delete("test")
	if err != nil {
		t.Errorf("delete should succeed: %v", err)
	}

	if nm.Get("test") != nil {
		t.Error("namespace should be deleted")
	}
}

func TestNamespaceManagerDeleteDefault(t *testing.T) {
	nm := NewNamespaceManager()

	err := nm.Delete("default")
	if err == nil {
		t.Error("delete default should fail")
	}
}

func TestNamespaceManagerDeleteNonexistent(t *testing.T) {
	nm := NewNamespaceManager()

	err := nm.Delete("nonexistent")
	if err == nil {
		t.Error("delete nonexistent should fail")
	}
}

func TestNamespaceManagerList(t *testing.T) {
	nm := NewNamespaceManager()

	nm.GetOrCreate("ns1")
	nm.GetOrCreate("ns2")

	list := nm.List()
	if len(list) != 3 {
		t.Errorf("expected 3 namespaces (default, ns1, ns2), got %d", len(list))
	}
}

func TestNamespaceManagerFlush(t *testing.T) {
	nm := NewNamespaceManager()

	ns := nm.GetOrCreate("test")
	ns.Store.Set("key", &StringValue{Data: []byte("value")}, SetOptions{})

	err := nm.Flush("test")
	if err != nil {
		t.Errorf("flush should succeed: %v", err)
	}

	if ns.Store.KeyCount() != 0 {
		t.Error("namespace should be flushed")
	}
}

func TestNamespaceManagerFlushNonexistent(t *testing.T) {
	nm := NewNamespaceManager()

	err := nm.Flush("nonexistent")
	if err == nil {
		t.Error("flush nonexistent should fail")
	}
}

func TestNamespaceManagerFlushAll(t *testing.T) {
	nm := NewNamespaceManager()

	ns1 := nm.GetOrCreate("ns1")
	ns2 := nm.GetOrCreate("ns2")

	ns1.Store.Set("key1", &StringValue{Data: []byte("v1")}, SetOptions{})
	ns2.Store.Set("key2", &StringValue{Data: []byte("v2")}, SetOptions{})

	nm.FlushAll()

	if ns1.Store.KeyCount() != 0 || ns2.Store.KeyCount() != 0 {
		t.Error("all namespaces should be flushed")
	}
}

func TestNamespaceManagerStats(t *testing.T) {
	nm := NewNamespaceManager()

	ns := nm.GetOrCreate("test")
	ns.Store.Set("key", &StringValue{Data: []byte("value")}, SetOptions{})

	stats, err := nm.Stats("test")
	if err != nil {
		t.Fatalf("stats should succeed: %v", err)
	}

	if stats["name"] != "test" {
		t.Errorf("expected name 'test', got %v", stats["name"])
	}

	if stats["keys"].(int64) != 1 {
		t.Errorf("expected 1 key, got %v", stats["keys"])
	}
}

func TestNamespaceManagerStatsNonexistent(t *testing.T) {
	nm := NewNamespaceManager()

	_, err := nm.Stats("nonexistent")
	if err == nil {
		t.Error("stats for nonexistent should fail")
	}
}

func TestNamespace(t *testing.T) {
	ns := &Namespace{
		Name:      "test",
		Store:     NewStore(),
		Tags:      NewTagIndex(),
		CreatedAt: time.Now(),
	}

	if ns.Name != "test" {
		t.Errorf("expected name 'test', got '%s'", ns.Name)
	}

	if ns.Store == nil {
		t.Error("store should not be nil")
	}

	if ns.Tags == nil {
		t.Error("tags should not be nil")
	}
}
