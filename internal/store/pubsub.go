package store

import (
	"sync"
)

type PubSub struct {
	mu          sync.RWMutex
	channels    map[string]map[*Subscriber]struct{}
	patterns    map[string]map[*Subscriber]struct{}
	subscribers map[*Subscriber]struct{}
}

type Subscriber struct {
	ID     int64
	ch     chan []byte
	mu     sync.Mutex
	closed bool
}

func NewPubSub() *PubSub {
	return &PubSub{
		channels:    make(map[string]map[*Subscriber]struct{}),
		patterns:    make(map[string]map[*Subscriber]struct{}),
		subscribers: make(map[*Subscriber]struct{}),
	}
}

func NewSubscriber(id int64) *Subscriber {
	return &Subscriber{
		ID: id,
		ch: make(chan []byte, 256),
	}
}

func (s *Subscriber) Send(message []byte) bool {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.closed {
		return false
	}

	select {
	case s.ch <- message:
		return true
	default:
		return false
	}
}

func (s *Subscriber) Channel() <-chan []byte {
	return s.ch
}

func (s *Subscriber) Close() {
	s.mu.Lock()
	defer s.mu.Unlock()

	if !s.closed {
		s.closed = true
		close(s.ch)
	}
}

func (ps *PubSub) Subscribe(sub *Subscriber, channels ...string) int {
	ps.mu.Lock()
	defer ps.mu.Unlock()

	ps.subscribers[sub] = struct{}{}

	for _, ch := range channels {
		if ps.channels[ch] == nil {
			ps.channels[ch] = make(map[*Subscriber]struct{})
		}
		ps.channels[ch][sub] = struct{}{}
	}

	return len(channels)
}

func (ps *PubSub) Unsubscribe(sub *Subscriber, channels ...string) int {
	ps.mu.Lock()
	defer ps.mu.Unlock()

	count := 0
	if len(channels) == 0 {
		for ch := range ps.channels {
			delete(ps.channels[ch], sub)
			count++
		}
	} else {
		for _, ch := range channels {
			if subs, exists := ps.channels[ch]; exists {
				delete(subs, sub)
				count++
			}
		}
	}

	ps.checkRemoveSubscriber(sub)
	return count
}

func (ps *PubSub) PSubscribe(sub *Subscriber, patterns ...string) int {
	ps.mu.Lock()
	defer ps.mu.Unlock()

	ps.subscribers[sub] = struct{}{}

	for _, p := range patterns {
		if ps.patterns[p] == nil {
			ps.patterns[p] = make(map[*Subscriber]struct{})
		}
		ps.patterns[p][sub] = struct{}{}
	}

	return len(patterns)
}

func (ps *PubSub) PUnsubscribe(sub *Subscriber, patterns ...string) int {
	ps.mu.Lock()
	defer ps.mu.Unlock()

	count := 0
	if len(patterns) == 0 {
		for p := range ps.patterns {
			delete(ps.patterns[p], sub)
			count++
		}
	} else {
		for _, p := range patterns {
			if subs, exists := ps.patterns[p]; exists {
				delete(subs, sub)
				count++
			}
		}
	}

	ps.checkRemoveSubscriber(sub)
	return count
}

func (ps *PubSub) checkRemoveSubscriber(sub *Subscriber) {
	for _, subs := range ps.channels {
		if _, exists := subs[sub]; exists {
			return
		}
	}
	for _, subs := range ps.patterns {
		if _, exists := subs[sub]; exists {
			return
		}
	}
	delete(ps.subscribers, sub)
}

func (ps *PubSub) Publish(channel string, message []byte) int {
	ps.mu.RLock()
	defer ps.mu.RUnlock()

	count := 0

	if subs, exists := ps.channels[channel]; exists {
		for sub := range subs {
			if sub.Send(message) {
				count++
			}
		}
	}

	for pattern, subs := range ps.patterns {
		if matchPattern(channel, pattern) {
			for sub := range subs {
				if sub.Send(message) {
					count++
				}
			}
		}
	}

	return count
}

func (ps *PubSub) Channels(pattern string) []string {
	ps.mu.RLock()
	defer ps.mu.RUnlock()

	channels := make([]string, 0)
	for ch := range ps.channels {
		if pattern == "" || matchPattern(ch, pattern) {
			channels = append(channels, ch)
		}
	}
	return channels
}

func (ps *PubSub) NumSub(channels ...string) map[string]int {
	ps.mu.RLock()
	defer ps.mu.RUnlock()

	result := make(map[string]int)
	for _, ch := range channels {
		if subs, exists := ps.channels[ch]; exists {
			result[ch] = len(subs)
		} else {
			result[ch] = 0
		}
	}
	return result
}

func (ps *PubSub) NumPat() int {
	ps.mu.RLock()
	defer ps.mu.RUnlock()

	count := 0
	for _, subs := range ps.patterns {
		count += len(subs)
	}
	return count
}

func (ps *PubSub) RemoveSubscriber(sub *Subscriber) {
	ps.Unsubscribe(sub)
	ps.PUnsubscribe(sub)
	sub.Close()
}

func matchPattern(s, pattern string) bool {
	if pattern == "*" {
		return true
	}

	si, pi := 0, 0
	starIdx, match := -1, 0

	for si < len(s) {
		if pi < len(pattern) && (pattern[pi] == '?' || pattern[pi] == s[si]) {
			si++
			pi++
		} else if pi < len(pattern) && pattern[pi] == '*' {
			starIdx = pi
			match = si
			pi++
		} else if starIdx != -1 {
			pi = starIdx + 1
			match++
			si = match
		} else {
			return false
		}
	}

	for pi < len(pattern) && pattern[pi] == '*' {
		pi++
	}

	return pi == len(pattern)
}
