package cluster

import (
	"encoding/json"
	"sync"
	"time"

	"github.com/cachestorm/cachestorm/internal/logger"
)

type TagBroadcastMessage struct {
	Type       string   `json:"type"`
	Tag        string   `json:"tag"`
	Keys       []string `json:"keys"`
	OriginNode string   `json:"origin_node"`
	Timestamp  int64    `json:"timestamp"`
}

type TagBroadcaster struct {
	cluster    *Cluster
	handlers   []func(tag string, keys []string)
	recentMsgs map[string]int64
	mu         sync.RWMutex
}

func NewTagBroadcaster(c *Cluster) *TagBroadcaster {
	return &TagBroadcaster{
		cluster:    c,
		handlers:   make([]func(tag string, keys []string), 0),
		recentMsgs: make(map[string]int64),
	}
}

func (tb *TagBroadcaster) RegisterHandler(handler func(tag string, keys []string)) {
	tb.mu.Lock()
	defer tb.mu.Unlock()
	tb.handlers = append(tb.handlers, handler)
}

func (tb *TagBroadcaster) Broadcast(tag string, keys []string) error {
	if !tb.cluster.IsEnabled() {
		return nil
	}

	msg := TagBroadcastMessage{
		Type:       "TAG_INVALIDATE",
		Tag:        tag,
		Keys:       keys,
		OriginNode: tb.cluster.Self().ID,
		Timestamp:  time.Now().UnixNano(),
	}

	tb.cleanOldMessages()

	msgID := msg.OriginNode + string(rune(msg.Timestamp))
	tb.mu.Lock()
	tb.recentMsgs[msgID] = msg.Timestamp
	tb.mu.Unlock()

	data, err := json.Marshal(msg)
	if err != nil {
		return err
	}

	logger.Debug().
		Str("tag", tag).
		Int("keys", len(keys)).
		Msg("broadcasting tag invalidation")

	_ = data

	return nil
}

func (tb *TagBroadcaster) HandleMessage(data []byte) error {
	var msg TagBroadcastMessage
	if err := json.Unmarshal(data, &msg); err != nil {
		return err
	}

	if msg.OriginNode == tb.cluster.Self().ID {
		return nil
	}

	msgID := msg.OriginNode + string(rune(msg.Timestamp))
	tb.mu.RLock()
	_, seen := tb.recentMsgs[msgID]
	tb.mu.RUnlock()

	if seen {
		return nil
	}

	tb.mu.Lock()
	tb.recentMsgs[msgID] = msg.Timestamp
	tb.mu.Unlock()

	logger.Debug().
		Str("tag", msg.Tag).
		Str("origin", msg.OriginNode).
		Int("keys", len(msg.Keys)).
		Msg("received tag invalidation broadcast")

	tb.mu.RLock()
	handlers := make([]func(tag string, keys []string), len(tb.handlers))
	copy(handlers, tb.handlers)
	tb.mu.RUnlock()

	for _, h := range handlers {
		h(msg.Tag, msg.Keys)
	}

	return nil
}

func (tb *TagBroadcaster) cleanOldMessages() {
	tb.mu.Lock()
	defer tb.mu.Unlock()

	threshold := time.Now().Add(-5 * time.Minute).UnixNano()
	for id, ts := range tb.recentMsgs {
		if ts < threshold {
			delete(tb.recentMsgs, id)
		}
	}
}
