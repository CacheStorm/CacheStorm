package store

import (
	"testing"
)

func TestNewConsumerGroup(t *testing.T) {
	cg := NewConsumerGroup("group1")
	if cg == nil {
		t.Fatal("expected ConsumerGroup")
	}
	if cg.Name != "group1" {
		t.Errorf("expected name 'group1', got %s", cg.Name)
	}
}

func TestConsumerGroupGetOrCreateConsumer2(t *testing.T) {
	cg := NewConsumerGroup("group1")

	c1 := cg.GetOrCreateConsumer("consumer1")
	if c1 == nil {
		t.Fatal("expected consumer")
	}

	c2 := cg.GetOrCreateConsumer("consumer1")
	if c1 != c2 {
		t.Error("expected same consumer instance")
	}
}

func TestConsumerGroupAddPending2(t *testing.T) {
	cg := NewConsumerGroup("group1")
	cg.GetOrCreateConsumer("consumer1")

	cg.AddPending("1-0", "consumer1")

	if len(cg.Pending) != 1 {
		t.Errorf("expected 1 pending entry, got %d", len(cg.Pending))
	}
}

func TestConsumerGroupAck2(t *testing.T) {
	cg := NewConsumerGroup("group1")
	cg.GetOrCreateConsumer("consumer1")
	cg.AddPending("1-0", "consumer1")

	result := cg.Ack("1-0")
	if !result {
		t.Error("expected true for ack")
	}

	if len(cg.Pending) != 0 {
		t.Errorf("expected 0 pending entries, got %d", len(cg.Pending))
	}
}

func TestConsumerGroupAckNotFound(t *testing.T) {
	cg := NewConsumerGroup("group1")

	result := cg.Ack("nonexistent")
	if result {
		t.Error("expected false for nonexistent entry")
	}
}

func TestConsumerGroupClaim2(t *testing.T) {
	cg := NewConsumerGroup("group1")
	cg.GetOrCreateConsumer("consumer1")
	cg.GetOrCreateConsumer("consumer2")
	cg.AddPending("1-0", "consumer1")
	cg.AddPending("1-1", "consumer1")

	claimed := cg.Claim([]string{"1-0", "1-1"}, "consumer2")
	if len(claimed) != 2 {
		t.Errorf("expected 2 claimed entries, got %d", len(claimed))
	}
}

func TestConsumerGroupGetPending(t *testing.T) {
	cg := NewConsumerGroup("group1")
	cg.GetOrCreateConsumer("consumer1")
	cg.AddPending("1-0", "consumer1")
	cg.AddPending("1-1", "consumer1")
	cg.AddPending("2-0", "consumer1")

	pending := cg.GetPending("1-0", "1-9", 10)
	if len(pending) != 2 {
		t.Errorf("expected 2 pending entries, got %d", len(pending))
	}
}

func TestConsumerGroupGetPendingCount(t *testing.T) {
	cg := NewConsumerGroup("group1")
	cg.GetOrCreateConsumer("consumer1")
	cg.AddPending("1-0", "consumer1")
	cg.AddPending("1-1", "consumer1")

	count := cg.GetPendingCount()
	if count != 2 {
		t.Errorf("expected count 2, got %d", count)
	}
}

func TestConsumerGroupGetFirstLastID(t *testing.T) {
	cg := NewConsumerGroup("group1")
	cg.GetOrCreateConsumer("consumer1")
	cg.AddPending("1-0", "consumer1")
	cg.AddPending("1-5", "consumer1")
	cg.AddPending("1-3", "consumer1")

	first, last := cg.GetFirstLastID()
	if first != "1-0" {
		t.Errorf("expected first '1-0', got %s", first)
	}
	if last != "1-5" {
		t.Errorf("expected last '1-5', got %s", last)
	}
}

func TestConsumerGroupGetFirstLastIDEmpty(t *testing.T) {
	cg := NewConsumerGroup("group1")

	first, last := cg.GetFirstLastID()
	if first != "" {
		t.Errorf("expected empty first, got %s", first)
	}
	if last != "" {
		t.Errorf("expected empty last, got %s", last)
	}
}
