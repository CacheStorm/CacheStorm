package store

import (
	"fmt"
	"reflect"
	"testing"
)

func checkAudit50R29X(t *testing.T, label string, want, got interface{}) {
	t.Helper()
	t.Logf("%s EXPECTED: %v | ACTUAL: %v", label, want, got)
	if !reflect.DeepEqual(want, got) {
		t.Errorf("%s mismatch", label)
	}
}

func TestAudit50R29X(t *testing.T) {
	defer func() {
		if t.Failed() {
			t.Log("PROBLEM CONFIRMED")
		} else {
			t.Log("FIX VERIFIED")
		}
	}()

	ps := NewPubSub()
	one, two := NewSubscriber(1), NewSubscriber(2)
	ps.Subscribe(one, "active")
	ps.Subscribe(two, "active")
	ps.Unsubscribe(one, "active")
	checkAudit50R29X(t, "control still active", []string{"active"}, ps.Channels(""))
	ps.Unsubscribe(two, "active")
	checkAudit50R29X(t, "last unsubscribe removes inactive channel", []string{}, ps.Channels(""))
	ps.PSubscribe(one, "pattern*")
	ps.PUnsubscribe(one, "pattern*")
	checkAudit50R29X(t, "last pattern removed", 0, len(ps.patterns))

	for i := 0; i < 20; i++ {
		name := fmt.Sprintf("cycle-%d", i)
		ps.Subscribe(one, name)
		ps.PSubscribe(one, name+"*")
		ps.Unsubscribe(one)
		ps.PUnsubscribe(one)
	}
	checkAudit50R29X(t, "repeated channel cleanup", 0, len(ps.channels))
	checkAudit50R29X(t, "repeated pattern cleanup", 0, len(ps.patterns))
	checkAudit50R29X(t, "subscriber cleanup", 0, len(ps.subscribers))
	ps.Subscribe(one, "active")
	ps.PSubscribe(one, "a*")
	removed := make(chan struct{})
	go func() { ps.RemoveSubscriber(one); close(removed) }()
	<-removed
	checkAudit50R29X(t, "post-removal publish", 0, ps.Publish("active", []byte("message")))
	_, open := <-one.Channel()
	checkAudit50R29X(t, "removed subscriber closed", false, open)
	ps.Unsubscribe(two, "unknown")
	ps.PUnsubscribe(two, "unknown*")
	checkAudit50R29X(t, "missing unsubscribe remains empty", 0, len(ps.channels)+len(ps.patterns))

}
