package command

import (
	"testing"

	"github.com/cachestorm/cachestorm/internal/store"
)

func TestAllPubSubCommands(t *testing.T) {
	s := store.NewStore()
	ps := s.GetPubSub()
	router := NewRouter()
	RegisterPubSubCommands(router)

	tests := []struct {
		name  string
		cmd   string
		args  [][]byte
		setup func()
	}{
		{"SUBSCRIBE single", "SUBSCRIBE", [][]byte{[]byte("channel1")}, nil},
		{"SUBSCRIBE multiple", "SUBSCRIBE", [][]byte{[]byte("channel1"), []byte("channel2")}, nil},
		{"UNSUBSCRIBE", "UNSUBSCRIBE", [][]byte{[]byte("channel1")}, func() {
			sub := store.NewSubscriber(1)
			ps.Subscribe(sub, "channel1")
		}},
		{"UNSUBSCRIBE no args", "UNSUBSCRIBE", nil, func() {
			sub := store.NewSubscriber(2)
			ps.Subscribe(sub, "channel1")
			ps.Subscribe(sub, "channel2")
		}},
		{"PUBLISH", "PUBLISH", [][]byte{[]byte("channel1"), []byte("Hello World")}, func() {
			sub := store.NewSubscriber(3)
			ps.Subscribe(sub, "channel1")
		}},
		{"PUBLISH no subscribers", "PUBLISH", [][]byte{[]byte("emptychannel"), []byte("message")}, nil},
		{"PSUBSCRIBE pattern", "PSUBSCRIBE", [][]byte{[]byte("news.*")}, nil},
		{"PUNSUBSCRIBE", "PUNSUBSCRIBE", [][]byte{[]byte("news.*")}, func() {
			sub := store.NewSubscriber(4)
			ps.PSubscribe(sub, "news.*")
		}},
		{"PUNSUBSCRIBE no args", "PUNSUBSCRIBE", nil, func() {
			sub := store.NewSubscriber(5)
			ps.PSubscribe(sub, "news.*")
			ps.PSubscribe(sub, "events.*")
		}},
		{"PUBSUB CHANNELS", "PUBSUB", [][]byte{[]byte("CHANNELS")}, func() {
			sub1 := store.NewSubscriber(6)
			sub2 := store.NewSubscriber(7)
			ps.Subscribe(sub1, "channel1")
			ps.Subscribe(sub2, "channel2")
		}},
		{"PUBSUB NUMSUB", "PUBSUB", [][]byte{[]byte("NUMSUB"), []byte("channel1")}, func() {
			sub1 := store.NewSubscriber(8)
			sub2 := store.NewSubscriber(9)
			ps.Subscribe(sub1, "channel1")
			ps.Subscribe(sub2, "channel1")
		}},
		{"PUBSUB NUMPAT", "PUBSUB", [][]byte{[]byte("NUMPAT")}, func() {
			sub1 := store.NewSubscriber(10)
			sub2 := store.NewSubscriber(11)
			ps.PSubscribe(sub1, "pattern1")
			ps.PSubscribe(sub2, "pattern2")
		}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.setup != nil {
				tt.setup()
			}

			handler, ok := router.Get(tt.cmd)
			if !ok {
				t.Fatalf("Command %s not found", tt.cmd)
			}

			ctx := newTestContext(tt.cmd, tt.args, s)
			if err := handler.Handler(ctx); err != nil {
				t.Errorf("Unexpected error: %v", err)
			}
		})
	}
}

func TestPubSubSubscriberOperations(t *testing.T) {
	s := store.NewStore()
	ps := s.GetPubSub()

	t.Run("Subscribe and Publish", func(t *testing.T) {
		sub := store.NewSubscriber(100)
		count := ps.Subscribe(sub, "testchannel")
		if count != 1 {
			t.Errorf("Expected 1 subscription, got %d", count)
		}

		pubCount := ps.Publish("testchannel", []byte("test message"))
		if pubCount != 1 {
			t.Errorf("Expected 1 subscriber to receive message, got %d", pubCount)
		}

		ps.Unsubscribe(sub, "testchannel")
	})

	t.Run("Pattern Subscribe", func(t *testing.T) {
		sub := store.NewSubscriber(101)
		count := ps.PSubscribe(sub, "test.*")
		if count != 1 {
			t.Errorf("Expected 1 pattern subscription, got %d", count)
		}

		pubCount := ps.Publish("test.channel", []byte("pattern message"))
		if pubCount != 1 {
			t.Errorf("Expected 1 pattern subscriber to receive message, got %d", pubCount)
		}

		ps.PUnsubscribe(sub, "test.*")
	})

	t.Run("Multiple Subscribers", func(t *testing.T) {
		sub1 := store.NewSubscriber(102)
		sub2 := store.NewSubscriber(103)
		sub3 := store.NewSubscriber(104)

		ps.Subscribe(sub1, "multichannel")
		ps.Subscribe(sub2, "multichannel")
		ps.Subscribe(sub3, "multichannel")

		count := ps.Publish("multichannel", []byte("broadcast"))
		if count != 3 {
			t.Errorf("Expected 3 subscribers to receive message, got %d", count)
		}

		ps.Unsubscribe(sub1, "multichannel")
		ps.Unsubscribe(sub2, "multichannel")
		ps.Unsubscribe(sub3, "multichannel")
	})

	t.Run("Unsubscribe all channels", func(t *testing.T) {
		sub := store.NewSubscriber(105)
		ps.Subscribe(sub, "ch1")
		ps.Subscribe(sub, "ch2")

		ps.Unsubscribe(sub)
	})
}
