package store

import (
	"fmt"
	"testing"
)

func TestRemovedSubscriberRejectsStaleRegistration(t *testing.T) {
	ps := NewPubSub()
	active, stale := NewSubscriber(1), NewSubscriber(2)
	defer ps.RemoveSubscriber(active)
	ps.Subscribe(active, "control")
	if got := ps.Publish("control", []byte("ok")); got != 1 {
		t.Fatalf("control delivery: %d", got)
	}
	<-active.Channel()
	ps.Subscribe(stale, "old")
	ps.PSubscribe(stale, "old*")
	started, release, done := make(chan struct{}), make(chan struct{}), make(chan struct{})
	var channels, patterns int
	go func() {
		close(started)
		<-release
		channels = ps.Subscribe(stale, "late")
		patterns = ps.PSubscribe(stale, "late*")
		close(done)
	}()
	<-started
	ps.RemoveSubscriber(stale)
	close(release)
	<-done
	t.Logf("EXPECTED: registrations=0 patterns=0 ACTUAL: registrations=%d patterns=%d", channels, patterns)
	if channels != 0 || patterns != 0 {
		t.Fatal("closed subscriber registered")
	}
	if got := ps.NumSub("late", "old"); got["late"] != 0 || got["old"] != 0 {
		t.Fatalf("retained channels: %v", got)
	}
	if ps.NumPat() != 0 || len(ps.subscribers) != 1 {
		t.Fatal("retained closed subscriber or pattern")
	}
	ps.RemoveSubscriber(stale)
	if ps.Subscribe(stale, "again") != 0 || ps.PSubscribe(stale, "again*") != 0 {
		t.Fatal("repeated removal reopened subscriber")
	}
	if ps.Publish("late", []byte("unused")) != 0 {
		t.Fatal("closed delivery counted")
	}
	if _, open := <-stale.Channel(); open {
		t.Fatal("removed subscriber still open")
	}
	closed := NewSubscriber(3)
	closed.Close()
	if ps.Subscribe(closed, "closed") != 0 || ps.PSubscribe(closed, "closed*") != 0 {
		t.Fatal("directly closed subscriber registered")
	}
	if ps.Subscribe(active, "still-live") != 1 || ps.Publish("still-live", []byte("ok")) != 1 {
		t.Fatal("active subscriber affected")
	}
	fmt.Println("FIX VERIFIED")
}
