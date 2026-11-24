package store

import (
	"runtime"
	"strconv"
	"strings"
	"testing"
	"time"
)

func TestSharedMetaX(t *testing.T) {
	s := NewStore()
	for _, k := range []string{"a", "b"} {
		if err := s.Set(k, &StringValue{Data: []byte("v")}, SetOptions{Tags: []string{"1:x", "2"}}); err != nil {
			t.Fatal(err)
		}
	}
	if err := s.Set("c", &StringValue{Data: []byte("v")}, SetOptions{Tags: []string{"1", ":x2"}}); err != nil {
		t.Fatal(err)
	}
	ea, _ := s.Get("a")
	eb, _ := s.Get("b")
	ec, _ := s.Get("c")
	if ea.meta.Load() != eb.meta.Load() {
		t.Fatal("keys with the same tags do not share their metadata")
	}
	if ea.meta.Load() == ec.meta.Load() {
		t.Fatal("tags 1:x,2 and 1,:x2 share metadata")
	}
	if got := strings.Join(ea.TagList(), "|"); got != "1:x|2" {
		t.Fatalf("a's tags=%q", got)
	}
	if got := strings.Join(ec.TagList(), "|"); got != "1|:x2" {
		t.Fatalf("c's tags=%q", got)
	}
}

func TestSharedMetaForgottenX(t *testing.T) {
	tag := "forgotten-" + time.Now().String()
	if sharedMeta(nil, []string{tag}) == nil {
		t.Fatal("no metadata")
	}
	enc := encodedTagsForTest(tag)
	shard := &metas[fnv32a(enc)%uint32(len(metas))]
	key := metaKey{tags: string(enc)}
	deadline := time.Now().Add(5 * time.Second)
	for {
		runtime.GC()
		shard.mu.RLock()
		_, ok := shard.m[key]
		shard.mu.RUnlock()
		if !ok {
			return
		}
		if time.Now().After(deadline) {
			t.Fatal("the cache keeps the entry of tags no key has")
		}
		time.Sleep(10 * time.Millisecond)
	}
}

func encodedTagsForTest(tags ...string) []byte {
	var b []byte
	for _, tag := range tags {
		b = strconv.AppendInt(b, int64(len(tag)), 10)
		b = append(b, ':')
		b = append(b, tag...)
	}
	return b
}
