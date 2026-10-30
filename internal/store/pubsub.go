package store

import (
	"sort"
	"strconv"
	"sync"
	"sync/atomic"
	"time"
)

type PubSub struct {
	mu          sync.RWMutex
	channels    map[string]map[*Subscriber]struct{}
	patterns    map[string]map[*Subscriber]struct{}
	shards      map[string]map[*Subscriber]struct{}
	subscribers map[*Subscriber]struct{}
	// notifyFlags holds the notify-keyspace-events classes; every database
	// sharing the hub shares them.
	notifyFlags atomic.Int32
}

// Subscriber is one client's Pub/Sub state: the channels, patterns and shard
// channels it listens on, and the bounded queue of ready-to-write RESP frames
// that Publish fills and the client connection drains onto its socket.
//
// The subscription sets are guarded by the owning PubSub's mutex; the counters
// mirror their sizes so a connection can test "is this client subscribed"
// without taking that lock.
type Subscriber struct {
	ID int64
	ch chan []byte
	// ready is signalled after a frame is queued. A reader waits on it and
	// takes the frames themselves only while it holds its connection's write
	// lock, so frames are written in the order they were published.
	ready chan struct{}

	mu        sync.Mutex
	closed    bool
	dropCount atomic.Int64

	channelSet map[string]struct{}
	patternSet map[string]struct{}
	shardSet   map[string]struct{}
	nChannels  atomic.Int64
	nPatterns  atomic.Int64
	nShards    atomic.Int64

	pendingBytes atomic.Int64
	// softSince is when the pending bytes went over the soft limit, in unix
	// seconds, or 0. It is guarded by mu.
	softSince int64

	overflowed   atomic.Bool
	overflowCh   chan struct{}
	overflowOnce sync.Once
	onOverflow   func()
}

// subscriberQueueSize and pubsubLimits bound one subscriber's undelivered
// frames. Like Redis with client-output-buffer-limit pubsub, a subscriber
// whose pending bytes pass the hard limit, or stay over the soft limit for
// longer than its seconds, is marked overflowed so its connection closes,
// instead of silently losing messages. The limits apply to existing
// subscribers too; the queue size only to new ones.
var (
	subscriberQueueSize atomic.Int64
	pubsubLimits        atomic.Pointer[outputLimits]
)

type outputLimits struct {
	hard, soft, softSeconds int64
}

func init() {
	subscriberQueueSize.Store(4096)
	pubsubLimits.Store(&outputLimits{hard: 32 << 20, soft: 8 << 20, softSeconds: 60})
}

// SetPubsubOutputLimits sets the pending-bytes limits of every subscriber;
// a zero limit is no limit.
func SetPubsubOutputLimits(hard, soft, softSeconds int64) {
	pubsubLimits.Store(&outputLimits{hard: hard, soft: soft, softSeconds: softSeconds})
}

// SetSubscriberLimits changes the queue size for subscribers created after
// the call and the hard pending-bytes limit, and returns the previous values.
// A value <= 0 leaves that bound as it was.
func SetSubscriberLimits(queueSize int, maxPendingBytes int64) (int, int64) {
	lim := pubsubLimits.Load()
	prevQ, prevB := int(subscriberQueueSize.Load()), lim.hard
	if queueSize > 0 {
		subscriberQueueSize.Store(int64(queueSize))
	}
	if maxPendingBytes > 0 {
		SetPubsubOutputLimits(maxPendingBytes, lim.soft, lim.softSeconds)
	}
	return prevQ, prevB
}

func NewPubSub() *PubSub {
	return &PubSub{
		channels:    make(map[string]map[*Subscriber]struct{}),
		patterns:    make(map[string]map[*Subscriber]struct{}),
		shards:      make(map[string]map[*Subscriber]struct{}),
		subscribers: make(map[*Subscriber]struct{}),
	}
}

func NewSubscriber(id int64) *Subscriber {
	return &Subscriber{
		ID:         id,
		ch:         make(chan []byte, int(subscriberQueueSize.Load())),
		ready:      make(chan struct{}, 1),
		channelSet: make(map[string]struct{}),
		patternSet: make(map[string]struct{}),
		shardSet:   make(map[string]struct{}),
		overflowCh: make(chan struct{}),
	}
}

// Send enqueues one encoded frame. It never blocks: a full queue marks the
// subscriber overflowed (see Overflowed) and reports false.
func (s *Subscriber) Send(frame []byte) bool {
	s.mu.Lock()
	if s.closed {
		s.mu.Unlock()
		return false
	}
	if s.overflowed.Load() {
		s.mu.Unlock()
		s.dropCount.Add(1)
		return false
	}
	if s.overLimitLocked(s.pendingBytes.Load() + int64(len(frame))) {
		s.mu.Unlock()
		s.overflow()
		return false
	}
	select {
	case s.ch <- frame:
		s.pendingBytes.Add(int64(len(frame)))
		select {
		case s.ready <- struct{}{}:
		default:
		}
		s.mu.Unlock()
		return true
	default:
		s.mu.Unlock()
		s.overflow()
		return false
	}
}

func (s *Subscriber) overLimitLocked(used int64) bool {
	lim := pubsubLimits.Load()
	if lim.hard > 0 && used > lim.hard {
		return true
	}
	if lim.soft <= 0 || used <= lim.soft {
		s.softSince = 0
		return false
	}
	now := time.Now().Unix()
	if s.softSince == 0 {
		s.softSince = now
		return false
	}
	return now-s.softSince > lim.softSeconds
}

func (s *Subscriber) overflow() {
	s.dropCount.Add(1)
	s.overflowOnce.Do(func() {
		s.overflowed.Store(true)
		close(s.overflowCh)
		s.mu.Lock()
		fn := s.onOverflow
		s.mu.Unlock()
		if fn != nil {
			go fn()
		}
	})
}

// SetOverflowHandler registers fn to run, on its own goroutine, once the
// subscriber overflows. If it already has, fn runs right away.
func (s *Subscriber) SetOverflowHandler(fn func()) {
	s.mu.Lock()
	s.onOverflow = fn
	s.mu.Unlock()
	if fn != nil && s.overflowed.Load() {
		go fn()
	}
}

// Overflowed is closed once the subscriber's queue has overflowed.
func (s *Subscriber) Overflowed() <-chan struct{} {
	return s.overflowCh
}

func (s *Subscriber) IsOverflowed() bool {
	return s.overflowed.Load()
}

// Delivered tells the queue that a frame taken from Channel has been written,
// releasing its bytes from the pending-bytes bound.
func (s *Subscriber) Delivered(frame []byte) {
	s.pendingBytes.Add(-int64(len(frame)))
}

func (s *Subscriber) DropCount() int64 {
	return s.dropCount.Load()
}

// Ready is signalled when frames may be waiting in Channel, and closed with
// the subscriber.
func (s *Subscriber) Ready() <-chan struct{} {
	return s.ready
}

// Channel yields encoded RESP frames (message, pmessage, smessage) ready to be
// written to the client verbatim. It is closed when the subscriber is closed.
func (s *Subscriber) Channel() <-chan []byte {
	return s.ch
}

func (s *Subscriber) Close() {
	s.mu.Lock()
	defer s.mu.Unlock()

	if !s.closed {
		s.closed = true
		close(s.ch)
		close(s.ready)
	}
}

func (s *Subscriber) IsClosed() bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.closed
}

// SubscriptionCount is the number of channels plus patterns, the count Redis
// reports in SUBSCRIBE/PSUBSCRIBE/UNSUBSCRIBE/PUNSUBSCRIBE replies.
func (s *Subscriber) SubscriptionCount() int {
	return int(s.nChannels.Load() + s.nPatterns.Load())
}

// ShardSubscriptionCount is the number of shard channels, the count Redis
// reports in SSUBSCRIBE/SUNSUBSCRIBE replies.
func (s *Subscriber) ShardSubscriptionCount() int {
	return int(s.nShards.Load())
}

// TotalSubscriptions counts every kind; a client with a non-zero total is in
// the RESP2 subscribed state.
func (s *Subscriber) TotalSubscriptions() int {
	return int(s.nChannels.Load() + s.nPatterns.Load() + s.nShards.Load())
}

const (
	maxChannelsPerSubscriber = 10000 // Max channels a single subscriber can join
	maxChannelNameLength     = 256   // Max channel name length in bytes
)

// SubAck is one (un)subscribe confirmation: the channel or pattern and the
// subscriber's count after it was processed.
type SubAck struct {
	Name  string
	Count int
}

type subKind int

const (
	kindChannel subKind = iota
	kindPattern
	kindShard
)

func (ps *PubSub) index(k subKind) map[string]map[*Subscriber]struct{} {
	switch k {
	case kindPattern:
		return ps.patterns
	case kindShard:
		return ps.shards
	default:
		return ps.channels
	}
}

func (s *Subscriber) set(k subKind) (map[string]struct{}, *atomic.Int64) {
	switch k {
	case kindPattern:
		return s.patternSet, &s.nPatterns
	case kindShard:
		return s.shardSet, &s.nShards
	default:
		return s.channelSet, &s.nChannels
	}
}

func (s *Subscriber) countFor(k subKind) int {
	if k == kindShard {
		return s.ShardSubscriptionCount()
	}
	return s.SubscriptionCount()
}

// subscribe adds names of kind k and returns one ack per name, in order, plus
// how many names were accepted (duplicates included). Caller holds ps.mu.
func (ps *PubSub) subscribe(k subKind, sub *Subscriber, names []string) ([]SubAck, int) {
	sub.mu.Lock()
	closed := sub.closed
	sub.mu.Unlock()
	acks := make([]SubAck, 0, len(names))
	if closed {
		for _, n := range names {
			acks = append(acks, SubAck{Name: n, Count: 0})
		}
		return acks, 0
	}

	idx := ps.index(k)
	own, counter := sub.set(k)
	accepted := 0
	for _, name := range names {
		if len(name) > maxChannelNameLength {
			acks = append(acks, SubAck{Name: name, Count: sub.countFor(k)})
			continue
		}
		if _, have := own[name]; !have {
			if len(own) >= maxChannelsPerSubscriber {
				acks = append(acks, SubAck{Name: name, Count: sub.countFor(k)})
				continue
			}
			own[name] = struct{}{}
			counter.Add(1)
			if idx[name] == nil {
				idx[name] = make(map[*Subscriber]struct{})
			}
			idx[name][sub] = struct{}{}
		}
		accepted++
		acks = append(acks, SubAck{Name: name, Count: sub.countFor(k)})
	}
	if sub.TotalSubscriptions() > 0 {
		ps.subscribers[sub] = struct{}{}
	}
	return acks, accepted
}

// unsubscribe removes names of kind k (all of the subscriber's names when
// none are given). With explicit names there is one ack per name; with none,
// one ack per name actually removed. Caller holds ps.mu.
func (ps *PubSub) unsubscribe(k subKind, sub *Subscriber, names []string) ([]SubAck, int) {
	idx := ps.index(k)
	own, counter := sub.set(k)
	if len(names) == 0 {
		names = make([]string, 0, len(own))
		for n := range own {
			names = append(names, n)
		}
		sort.Strings(names)
	}
	acks := make([]SubAck, 0, len(names))
	removed := 0
	for _, name := range names {
		if _, have := own[name]; have {
			delete(own, name)
			counter.Add(-1)
			removed++
			if subs := idx[name]; subs != nil {
				delete(subs, sub)
				if len(subs) == 0 {
					delete(idx, name)
				}
			}
		}
		acks = append(acks, SubAck{Name: name, Count: sub.countFor(k)})
	}
	if sub.TotalSubscriptions() == 0 {
		delete(ps.subscribers, sub)
	}
	return acks, removed
}

func (ps *PubSub) Subscribe(sub *Subscriber, channels ...string) int {
	ps.mu.Lock()
	defer ps.mu.Unlock()
	_, n := ps.subscribe(kindChannel, sub, channels)
	return n
}

func (ps *PubSub) Unsubscribe(sub *Subscriber, channels ...string) int {
	ps.mu.Lock()
	defer ps.mu.Unlock()
	_, n := ps.unsubscribe(kindChannel, sub, channels)
	return n
}

func (ps *PubSub) PSubscribe(sub *Subscriber, patterns ...string) int {
	ps.mu.Lock()
	defer ps.mu.Unlock()
	_, n := ps.subscribe(kindPattern, sub, patterns)
	return n
}

func (ps *PubSub) PUnsubscribe(sub *Subscriber, patterns ...string) int {
	ps.mu.Lock()
	defer ps.mu.Unlock()
	_, n := ps.unsubscribe(kindPattern, sub, patterns)
	return n
}

// SubscribeChannels subscribes to channels and returns the SUBSCRIBE acks.
func (ps *PubSub) SubscribeChannels(sub *Subscriber, channels ...string) []SubAck {
	ps.mu.Lock()
	defer ps.mu.Unlock()
	acks, _ := ps.subscribe(kindChannel, sub, channels)
	return acks
}

// UnsubscribeChannels returns the UNSUBSCRIBE acks; with no channels given it
// returns one ack per channel left, which is none when there were none.
func (ps *PubSub) UnsubscribeChannels(sub *Subscriber, channels ...string) []SubAck {
	ps.mu.Lock()
	defer ps.mu.Unlock()
	acks, _ := ps.unsubscribe(kindChannel, sub, channels)
	return acks
}

func (ps *PubSub) SubscribePatterns(sub *Subscriber, patterns ...string) []SubAck {
	ps.mu.Lock()
	defer ps.mu.Unlock()
	acks, _ := ps.subscribe(kindPattern, sub, patterns)
	return acks
}

func (ps *PubSub) UnsubscribePatterns(sub *Subscriber, patterns ...string) []SubAck {
	ps.mu.Lock()
	defer ps.mu.Unlock()
	acks, _ := ps.unsubscribe(kindPattern, sub, patterns)
	return acks
}

// SSubscribe subscribes to shard channels, a namespace separate from plain
// channels: SPUBLISH reaches only SSUBSCRIBE subscribers and vice versa.
func (ps *PubSub) SSubscribe(sub *Subscriber, channels ...string) []SubAck {
	ps.mu.Lock()
	defer ps.mu.Unlock()
	acks, _ := ps.subscribe(kindShard, sub, channels)
	return acks
}

func (ps *PubSub) SUnsubscribe(sub *Subscriber, channels ...string) []SubAck {
	ps.mu.Lock()
	defer ps.mu.Unlock()
	acks, _ := ps.unsubscribe(kindShard, sub, channels)
	return acks
}

func appendBulk(b []byte, s []byte) []byte {
	b = append(b, '$')
	b = strconv.AppendInt(b, int64(len(s)), 10)
	b = append(b, '\r', '\n')
	b = append(b, s...)
	return append(b, '\r', '\n')
}

func buildFrame(parts ...[]byte) []byte {
	size := 16
	for _, p := range parts {
		size += len(p) + 16
	}
	b := make([]byte, 0, size)
	b = append(b, '*')
	b = strconv.AppendInt(b, int64(len(parts)), 10)
	b = append(b, '\r', '\n')
	for _, p := range parts {
		b = appendBulk(b, p)
	}
	return b
}

var (
	kwMessage  = []byte("message")
	kwPMessage = []byte("pmessage")
	kwSMessage = []byte("smessage")
)

// Publish delivers message to the channel's subscribers ("message" frames)
// and to every subscriber of a matching pattern ("pmessage" frames). It
// returns the number of deliveries, which is what PUBLISH replies with.
func (ps *PubSub) Publish(channel string, message []byte) int {
	ps.mu.RLock()
	defer ps.mu.RUnlock()

	count := 0
	if subs := ps.channels[channel]; len(subs) > 0 {
		frame := buildFrame(kwMessage, []byte(channel), message)
		for sub := range subs {
			if sub.Send(frame) {
				count++
			}
		}
	}

	for pattern, subs := range ps.patterns {
		if len(subs) == 0 || !matchPattern(channel, pattern) {
			continue
		}
		frame := buildFrame(kwPMessage, []byte(pattern), []byte(channel), message)
		for sub := range subs {
			if sub.Send(frame) {
				count++
			}
		}
	}

	return count
}

// SPublish delivers message to the shard channel's subscribers as
// "smessage" frames.
func (ps *PubSub) SPublish(channel string, message []byte) int {
	ps.mu.RLock()
	defer ps.mu.RUnlock()

	count := 0
	if subs := ps.shards[channel]; len(subs) > 0 {
		frame := buildFrame(kwSMessage, []byte(channel), message)
		for sub := range subs {
			if sub.Send(frame) {
				count++
			}
		}
	}
	return count
}

func (ps *PubSub) listNames(idx map[string]map[*Subscriber]struct{}, pattern string) []string {
	names := make([]string, 0)
	for ch, subs := range idx {
		if len(subs) == 0 {
			continue
		}
		if pattern == "" || matchPattern(ch, pattern) {
			names = append(names, ch)
		}
	}
	return names
}

// Channels lists the active channels, optionally filtered by a glob pattern.
func (ps *PubSub) Channels(pattern string) []string {
	ps.mu.RLock()
	defer ps.mu.RUnlock()
	return ps.listNames(ps.channels, pattern)
}

// ShardChannels lists the active shard channels, optionally filtered.
func (ps *PubSub) ShardChannels(pattern string) []string {
	ps.mu.RLock()
	defer ps.mu.RUnlock()
	return ps.listNames(ps.shards, pattern)
}

func countSubs(idx map[string]map[*Subscriber]struct{}, channels []string) map[string]int {
	result := make(map[string]int, len(channels))
	for _, ch := range channels {
		result[ch] = len(idx[ch])
	}
	return result
}

func (ps *PubSub) NumSub(channels ...string) map[string]int {
	ps.mu.RLock()
	defer ps.mu.RUnlock()
	return countSubs(ps.channels, channels)
}

func (ps *PubSub) ShardNumSub(channels ...string) map[string]int {
	ps.mu.RLock()
	defer ps.mu.RUnlock()
	return countSubs(ps.shards, channels)
}

// NumPat counts the distinct patterns clients are subscribed to, as Redis'
// PUBSUB NUMPAT does.
func (ps *PubSub) NumPat() int {
	ps.mu.RLock()
	defer ps.mu.RUnlock()
	return len(ps.patterns)
}

// Counts returns how many channels, patterns and shard channels have
// subscribers, and how many clients are subscribed to any.
func (ps *PubSub) Counts() (channels, patterns, shards, clients int) {
	ps.mu.RLock()
	defer ps.mu.RUnlock()
	return len(ps.channels), len(ps.patterns), len(ps.shards), len(ps.subscribers)
}

// RemoveSubscriber closes the subscriber and drops every subscription it holds.
func (ps *PubSub) RemoveSubscriber(sub *Subscriber) {
	sub.Close()
	ps.mu.Lock()
	defer ps.mu.Unlock()
	ps.unsubscribe(kindChannel, sub, nil)
	ps.unsubscribe(kindPattern, sub, nil)
	ps.unsubscribe(kindShard, sub, nil)
	delete(ps.subscribers, sub)
}

// matchPattern reports whether s matches the glob pattern with Redis's
// stringmatchlen semantics: *, ?, [abc], [a-z], [^a] and backslash escapes.
func matchPattern(s, pattern string) bool {
	skip := false
	return stringMatch(pattern, s, &skip, 0)
}

// MatchPattern exposes the Redis glob matcher used by Pub/Sub.
func MatchPattern(s, pattern string) bool {
	return matchPattern(s, pattern)
}

func stringMatch(p, s string, skipLonger *bool, nesting int) bool {
	if nesting > 1000 {
		return false
	}
	for len(p) > 0 && len(s) > 0 {
		switch p[0] {
		case '*':
			for len(p) > 1 && p[1] == '*' {
				p = p[1:]
			}
			if len(p) == 1 {
				return true
			}
			for len(s) > 0 {
				if stringMatch(p[1:], s, skipLonger, nesting+1) {
					return true
				}
				if *skipLonger {
					return false
				}
				s = s[1:]
			}
			*skipLonger = true
			return false
		case '?':
			s = s[1:]
		case '[':
			p = p[1:]
			not := len(p) > 0 && p[0] == '^'
			if not {
				p = p[1:]
			}
			match := false
			for {
				if len(p) == 0 {
					// Unterminated class: step back so the shared
					// advance below lands on the end of the pattern.
					p = "]"
					break
				}
				if p[0] == '\\' && len(p) >= 2 {
					p = p[1:]
					if p[0] == s[0] {
						match = true
					}
				} else if p[0] == ']' {
					break
				} else if len(p) >= 3 && p[1] == '-' {
					start, end := p[0], p[2]
					if start > end {
						start, end = end, start
					}
					if s[0] >= start && s[0] <= end {
						match = true
					}
					p = p[2:]
				} else if p[0] == s[0] {
					match = true
				}
				p = p[1:]
			}
			if not {
				match = !match
			}
			if !match {
				return false
			}
			s = s[1:]
		case '\\':
			if len(p) >= 2 {
				p = p[1:]
			}
			if p[0] != s[0] {
				return false
			}
			s = s[1:]
		default:
			if p[0] != s[0] {
				return false
			}
			s = s[1:]
		}
		p = p[1:]
		if len(s) == 0 {
			for len(p) > 0 && p[0] == '*' {
				p = p[1:]
			}
			break
		}
	}
	return len(p) == 0 && len(s) == 0
}
