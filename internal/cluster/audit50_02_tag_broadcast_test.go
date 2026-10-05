package cluster

import (
	"encoding/json"
	"reflect"
	"testing"
)

func checkAudit50R02X(t *testing.T, label string, want, got interface{}) {
	t.Helper()
	t.Logf("%s EXPECTED: %v | ACTUAL: %v", label, want, got)
	if !reflect.DeepEqual(want, got) {
		t.Errorf("%s mismatch", label)
	}
}

func TestAudit50R02X(t *testing.T) {
	defer func() {
		if t.Failed() {
			t.Log("PROBLEM CONFIRMED")
		} else {
			t.Log("FIX VERIFIED")
		}
	}()
	c := New("self", "local", 1, 2, nil)
	tb := NewTagBroadcaster(c)
	encode := func(ts int64) []byte {
		data, err := json.Marshal(TagBroadcastMessage{Type: "TAG_INVALIDATE", Tag: "tag", Keys: []string{"original"}, OriginNode: "peer", Timestamp: ts})
		if err != nil {
			t.Fatal(err)
		}
		return data
	}
	got := ""
	tb.RegisterHandler(func(_ string, keys []string) { got = keys[0] })
	if err := tb.HandleMessage(encode(1)); err != nil {
		t.Fatal(err)
	}
	checkAudit50R02X(t, "control single handler", "original", got)
	isolated := NewTagBroadcaster(c)
	isolated.RegisterHandler(func(_ string, keys []string) {
		ready, release, done := make(chan struct{}), make(chan struct{}), make(chan struct{})
		go func() { close(ready); <-release; keys[0] = "edited"; close(done) }()
		<-ready
		close(release)
		<-done
	})
	isolated.RegisterHandler(func(_ string, keys []string) { got = keys[0] })
	if err := isolated.HandleMessage(encode(2)); err != nil {
		t.Fatal(err)
	}
	checkAudit50R02X(t, "later subscriber sees original payload", "original", got)
	nils := NewTagBroadcaster(c)
	calls := 0
	nils.RegisterHandler(func(_ string, keys []string) {
		calls++
		checkAudit50R02X(t, "nil keys preserved", true, keys == nil)
	})
	if err := nils.HandleMessage([]byte(`{"origin_node":"peer","timestamp":3}`)); err != nil {
		t.Fatal(err)
	}
	checkAudit50R02X(t, "first delivery", 1, calls)
	if err := nils.HandleMessage([]byte(`{"origin_node":"peer","timestamp":3}`)); err != nil {
		t.Fatal(err)
	}
	checkAudit50R02X(t, "duplicate skipped", 1, calls)
	if err := nils.HandleMessage([]byte(`{"origin_node":"self","timestamp":4}`)); err != nil {
		t.Fatal(err)
	}
	checkAudit50R02X(t, "self event skipped", 1, calls)
	checkAudit50R02X(t, "invalid JSON rejected", true, nils.HandleMessage([]byte(`{`)) != nil)
	added := NewTagBroadcaster(c)
	later := 0
	added.RegisterHandler(func(_ string, _ []string) { added.RegisterHandler(func(_ string, _ []string) { later++ }) })
	if err := added.HandleMessage(encode(5)); err != nil {
		t.Fatal(err)
	}
	checkAudit50R02X(t, "new handler deferred", 0, later)
	if err := added.HandleMessage(encode(6)); err != nil {
		t.Fatal(err)
	}
	checkAudit50R02X(t, "new handler receives next event", 1, later)

}
