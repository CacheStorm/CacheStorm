package cluster

import (
	"encoding/json"
	"testing"
	"time"
)

func TestTagBroadcaster(t *testing.T) {
	c := New("node-1", "127.0.0.1", 6380, 7946, nil)
	tb := NewTagBroadcaster(c)

	t.Run("New", func(t *testing.T) {
		if tb == nil {
			t.Fatal("TagBroadcaster should not be nil")
		}
	})

	t.Run("RegisterHandler", func(t *testing.T) {
		tb.RegisterHandler(func(tag string, keys []string) {
		})
		if len(tb.handlers) != 1 {
			t.Errorf("handlers length = %d, want 1", len(tb.handlers))
		}
	})

	t.Run("BroadcastNotEnabled", func(t *testing.T) {
		err := tb.Broadcast("tag1", []string{"key1", "key2"})
		if err != nil {
			t.Errorf("broadcast should succeed: %v", err)
		}
	})

	t.Run("BroadcastEnabled", func(t *testing.T) {
		c2 := New("node-2", "127.0.0.1", 6380, 7946, nil)
		c2.enabled = true
		tb2 := NewTagBroadcaster(c2)

		err := tb2.Broadcast("tag1", []string{"key1"})
		if err != nil {
			t.Errorf("broadcast should succeed: %v", err)
		}
	})

	t.Run("HandleMessage", func(t *testing.T) {
		msg := TagBroadcastMessage{
			Type:       "TAG_INVALIDATE",
			Tag:        "test-tag",
			Keys:       []string{"key1", "key2"},
			OriginNode: "other-node",
			Timestamp:  time.Now().UnixNano(),
		}
		data, _ := json.Marshal(msg)

		err := tb.HandleMessage(data)
		if err != nil {
			t.Errorf("handle message should succeed: %v", err)
		}
	})

	t.Run("HandleMessageFromSelf", func(t *testing.T) {
		msg := TagBroadcastMessage{
			Type:       "TAG_INVALIDATE",
			Tag:        "test-tag",
			Keys:       []string{"key1"},
			OriginNode: "node-1",
			Timestamp:  time.Now().UnixNano(),
		}
		data, _ := json.Marshal(msg)

		err := tb.HandleMessage(data)
		if err != nil {
			t.Errorf("handle message from self should succeed: %v", err)
		}
	})

	t.Run("HandleMessageInvalidJSON", func(t *testing.T) {
		err := tb.HandleMessage([]byte("invalid json"))
		if err == nil {
			t.Error("handle message with invalid JSON should fail")
		}
	})

	t.Run("CleanOldMessages", func(t *testing.T) {
		tb.recentMsgs["old"] = time.Now().Add(-10 * time.Minute).UnixNano()
		tb.recentMsgs["new"] = time.Now().UnixNano()

		tb.cleanOldMessages()

		if _, exists := tb.recentMsgs["old"]; exists {
			t.Error("old message should be cleaned")
		}
		if _, exists := tb.recentMsgs["new"]; !exists {
			t.Error("new message should exist")
		}
	})
}

func TestGossipCoverage(t *testing.T) {
	c := New("node-1", "127.0.0.1", 6380, 7946, nil)
	g := NewGossip(c)

	t.Run("New", func(t *testing.T) {
		if g == nil {
			t.Fatal("Gossip should not be nil")
		}
	})

	t.Run("Meet", func(t *testing.T) {
		err := g.Meet("127.0.0.1", 7947)
		_ = err
	})
}

func TestFailoverExtendedCoverage(t *testing.T) {
	c := New("node-1", "127.0.0.1", 6380, 7946, nil)
	g := NewGossip(c)
	fm := NewFailoverManager(c, g)

	t.Run("GetReplicasOf", func(t *testing.T) {
		replica := &Node{
			ID:        "replica-1",
			Addr:      "127.0.0.1",
			Port:      6381,
			Role:      RoleReplica,
			State:     NodeStateOnline,
			ReplicaOf: "primary-1",
		}
		c.AddNode(replica)

		replicas := fm.getReplicasOf("primary-1")
		if len(replicas) != 1 {
			t.Errorf("replicas length = %d, want 1", len(replicas))
		}
	})

	t.Run("GetReplicaOffset", func(t *testing.T) {
		offset := fm.getReplicaOffset("replica-1")
		if offset == 0 {
			t.Error("offset should not be 0")
		}
	})

	t.Run("Vote", func(t *testing.T) {
		voted := fm.Vote("voter-1", "candidate-1")
		if voted {
			t.Error("vote should fail when not in progress")
		}
	})

	t.Run("RunElection", func(t *testing.T) {
		fm.runElection()
	})
}

func TestNodeInfo(t *testing.T) {
	info := NodeInfo{
		ID:         "node-1",
		Addr:       "127.0.0.1",
		Port:       6380,
		GossipPort: 7946,
		Role:       "primary",
		State:      "online",
		ReplicaOf:  "",
	}

	if info.ID != "node-1" {
		t.Errorf("ID = %s, want node-1", info.ID)
	}
}

func TestGossipMessage(t *testing.T) {
	msg := GossipMessage{
		Type:      "ping",
		SenderID:  "node-1",
		Timestamp: time.Now().UnixNano(),
		Nodes: []NodeInfo{
			{ID: "node-2", Addr: "127.0.0.1", Port: 6381},
		},
		Slot:     1000,
		TargetID: "node-2",
	}

	data, err := json.Marshal(msg)
	if err != nil {
		t.Errorf("marshal should succeed: %v", err)
	}

	var decoded GossipMessage
	err = json.Unmarshal(data, &decoded)
	if err != nil {
		t.Errorf("unmarshal should succeed: %v", err)
	}

	if decoded.Type != "ping" {
		t.Errorf("type = %s, want ping", decoded.Type)
	}
}

func TestTagBroadcastMessage(t *testing.T) {
	msg := TagBroadcastMessage{
		Type:       "TAG_INVALIDATE",
		Tag:        "users",
		Keys:       []string{"user:1", "user:2"},
		OriginNode: "node-1",
		Timestamp:  time.Now().UnixNano(),
	}

	data, err := json.Marshal(msg)
	if err != nil {
		t.Errorf("marshal should succeed: %v", err)
	}

	var decoded TagBroadcastMessage
	err = json.Unmarshal(data, &decoded)
	if err != nil {
		t.Errorf("unmarshal should succeed: %v", err)
	}

	if decoded.Tag != "users" {
		t.Errorf("tag = %s, want users", decoded.Tag)
	}
}

func TestClusterBalanceSlotsCoverage(t *testing.T) {
	c := New("node-1", "127.0.0.1", 6380, 7946, nil)

	node2 := &Node{
		ID:    "node-2",
		Addr:  "127.0.0.1",
		Port:  6381,
		Role:  RolePrimary,
		State: NodeStateOnline,
	}
	c.AddNode(node2)

	c.AssignSlots([]SlotRange{{Start: 0, End: 8191}})
	c.BalanceSlots()
}

func TestClusterGetClusterNodesCoverage(t *testing.T) {
	c := New("node-1", "127.0.0.1", 6380, 7946, nil)

	node2 := &Node{
		ID:    "node-2x",
		Addr:  "127.0.0.1",
		Port:  6381,
		Role:  RoleReplica,
		State: NodeStateOnline,
	}
	c.AddNode(node2)

	nodes := c.GetClusterNodes()
	if len(nodes) == 0 {
		t.Error("cluster nodes should not be empty")
	}
}

func TestGossipStartStop(t *testing.T) {
	c := New("node-1", "127.0.0.1", 6380, 7947, nil)
	g := NewGossip(c)

	err := g.Start()
	if err != nil {
		t.Logf("gossip start returned: %v", err)
	}

	g.Stop()
}

func TestFailoverStartFailover(t *testing.T) {
	c := New("node-1", "127.0.0.1", 6380, 7946, nil)
	g := NewGossip(c)
	fm := NewFailoverManager(c, g)

	primary := &Node{
		ID:    "primary-1",
		Addr:  "127.0.0.1",
		Port:  6380,
		Role:  RolePrimary,
		State: NodeStateOnline,
	}
	c.AddNode(primary)

	replica := &Node{
		ID:        "replica-1",
		Addr:      "127.0.0.1",
		Port:      6381,
		Role:      RoleReplica,
		State:     NodeStateOnline,
		ReplicaOf: "primary-1",
	}
	c.AddNode(replica)

	err := fm.StartFailover("primary-1")
	_ = err
}

func TestSlotMigratorCoverage(t *testing.T) {
	c := New("node-1", "127.0.0.1", 6380, 7946, nil)
	sm := NewSlotMigrator(c)

	node1 := &Node{
		ID:    "node-1",
		Addr:  "127.0.0.1",
		Port:  6380,
		Role:  RolePrimary,
		State: NodeStateOnline,
	}
	c.AddNode(node1)

	node2 := &Node{
		ID:    "node-2",
		Addr:  "127.0.0.1",
		Port:  6381,
		Role:  RolePrimary,
		State: NodeStateOnline,
	}
	c.AddNode(node2)

	err := sm.StartMigration("node-1", "node-2", []uint16{100})
	_ = err

	sm.Cancel()
	sm.IsMigrating()
}

func TestFailoverGetStateCoverage(t *testing.T) {
	c := New("node-1", "127.0.0.1", 6380, 7946, nil)
	g := NewGossip(c)
	fm := NewFailoverManager(c, g)

	state := fm.GetState()
	_ = state
}

func TestClusterGetSlotDistributionExt(t *testing.T) {
	c := New("node-1", "127.0.0.1", 6380, 7946, nil)

	c.AssignSlots([]SlotRange{{Start: 0, End: 5000}})

	distribution := c.GetSlotDistribution()
	_ = distribution
}

func TestHashSlotRouterExt(t *testing.T) {
	c := New("node-1", "127.0.0.1", 6380, 7946, nil)
	router := NewHashSlotRouter(c)

	slot := router.GetSlot("mykey")
	_ = slot

	node := router.GetNodeForKey("mykey")
	_ = node

	slotID, addr, port := router.GetMovedError("mykey")
	_ = slotID
	_ = addr
	_ = port
}

func TestCRC16ExtCoverage(t *testing.T) {
	result := CRC16([]byte("test"))
	_ = result
}

func TestKeySlotExtCoverage(t *testing.T) {
	slot := KeySlot("{user:1}:profile")
	_ = slot
}
