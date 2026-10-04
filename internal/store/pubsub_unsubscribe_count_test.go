package store

import "testing"

// The unsubscribe count reports how many channels/patterns THIS subscriber
// actually left. It previously counted every channel that merely existed in
// the system, so channels belonging to other clients inflated the number.
func TestUnsubscribeCountIgnoresForeignChannels(t *testing.T) {
	ps := NewPubSub()

	subA := NewSubscriber(1)
	subB := NewSubscriber(2)

	ps.Subscribe(subA, "alpha")
	ps.Subscribe(subB, "bravo")

	// A holds exactly one channel; "bravo" exists only because B is on it.
	if got := ps.Unsubscribe(subA, "alpha", "bravo"); got != 1 {
		t.Fatalf("Unsubscribe(alpha, bravo) = %d, want 1 — bravo belongs to another subscriber", got)
	}
}

// Same defect through the pattern variant.
func TestPUnsubscribeCountIgnoresForeignPatterns(t *testing.T) {
	ps := NewPubSub()

	subA := NewSubscriber(1)
	subB := NewSubscriber(2)

	ps.PSubscribe(subA, "news.*")
	ps.PSubscribe(subB, "sports.*")

	if got := ps.PUnsubscribe(subA, "news.*", "sports.*"); got != 1 {
		t.Fatalf("PUnsubscribe(news.*, sports.*) = %d, want 1 — sports.* belongs to another subscriber", got)
	}
}

// Control: unsubscribing from channels the subscriber does hold still counts
// them all.
func TestUnsubscribeOwnChannelsControl(t *testing.T) {
	ps := NewPubSub()

	sub := NewSubscriber(1)
	ps.Subscribe(sub, "one", "two", "three")

	if got := ps.Unsubscribe(sub, "one", "two", "three"); got != 3 {
		t.Fatalf("Unsubscribe(one, two, three) = %d, want 3", got)
	}
}

// Control: the no-argument form counts the subscriber's own channels, not
// every channel in the system.
func TestUnsubscribeAllCountsOwnChannelsControl(t *testing.T) {
	ps := NewPubSub()

	subA := NewSubscriber(1)
	subB := NewSubscriber(2)

	ps.Subscribe(subA, "a1", "a2")
	ps.Subscribe(subB, "b1", "b2", "b3")

	if got := ps.Unsubscribe(subA); got != 2 {
		t.Fatalf("Unsubscribe(subA) = %d, want 2 — B's three channels must not be counted", got)
	}
}

// Control: another subscriber's membership is untouched, and delivery still works.
func TestUnsubscribeMembershipAndDeliveryControl(t *testing.T) {
	ps := NewPubSub()

	subA := NewSubscriber(1)
	subB := NewSubscriber(2)

	ps.Subscribe(subA, "shared")
	ps.Subscribe(subB, "shared")
	ps.Unsubscribe(subA, "shared")

	if n := ps.NumSub("shared")["shared"]; n != 1 {
		t.Fatalf("NumSub(shared) = %d, want 1 (B must remain)", n)
	}

	if n := ps.Publish("shared", []byte("hello")); n != 1 {
		t.Fatalf("Publish delivered to %d subscribers, want 1", n)
	}
	select {
	case msg := <-subB.Channel():
		if string(msg) != "hello" {
			t.Fatalf("B received %q, want %q", msg, "hello")
		}
	default:
		t.Fatal("B received no message")
	}
}
