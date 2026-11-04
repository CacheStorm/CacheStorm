package store

import (
	"sort"
	"sync"
	"time"
)

// StreamField is one field-value pair of a stream entry, in insertion order.
type StreamField struct {
	Name  string
	Value []byte
}

// StreamEntry is an entry as the stream hands it out, built from the node
// that packs it.
type StreamEntry struct {
	ID string
	// Pairs keeps the fields in the order XADD received them, duplicates
	// included.
	Pairs []StreamField
}

// FieldPairs returns the entry's fields in reply order.
func (e *StreamEntry) FieldPairs() []StreamField { return e.Pairs }

// FieldMap returns the entry's fields by name, the last value of a
// repeated one winning.
func (e *StreamEntry) FieldMap() map[string][]byte {
	if e.Pairs == nil {
		return nil
	}
	m := make(map[string][]byte, len(e.Pairs))
	for _, p := range e.Pairs {
		m[p.Name] = p.Value
	}
	return m
}

func (e *StreamEntry) streamID() StreamID { return streamIDOf(e.ID) }

type PendingEntry struct {
	ID         string
	Consumer   string
	DeliveryTS int64
	Deliveries int64
}

type Consumer struct {
	Name     string
	SeenTime int64
	// ActiveTime is the last successful read or claim, -1 when never.
	ActiveTime int64
	Pending    int64
	Active     bool
}

// EntriesReadInvalid marks a group whose logical read counter is unknown.
const EntriesReadInvalid int64 = -1

type ConsumerGroup struct {
	Name        string
	LastID      string
	EntriesRead int64
	Consumers   map[string]*Consumer
	Pending     map[string]*PendingEntry
	mu          sync.RWMutex
}

// RLock and RUnlock hold the group for reading, for a caller that walks its
// pending entries and consumers.
func (g *ConsumerGroup) RLock()   { g.mu.RLock() }
func (g *ConsumerGroup) RUnlock() { g.mu.RUnlock() }

func NewConsumerGroup(name string) *ConsumerGroup {
	return &ConsumerGroup{
		Name:        name,
		LastID:      "0-0",
		EntriesRead: EntriesReadInvalid,
		Consumers:   make(map[string]*Consumer),
		Pending:     make(map[string]*PendingEntry),
	}
}

func newStreamConsumer(name string, now int64) *Consumer {
	return &Consumer{Name: name, SeenTime: now, ActiveTime: -1, Active: true}
}

func (g *ConsumerGroup) GetOrCreateConsumer(name string) *Consumer {
	g.mu.Lock()
	defer g.mu.Unlock()
	return g.consumerLocked(name, time.Now().UnixMilli())
}

func (g *ConsumerGroup) consumerLocked(name string, now int64) *Consumer {
	if c, exists := g.Consumers[name]; exists {
		return c
	}
	c := newStreamConsumer(name, now)
	g.Consumers[name] = c
	return c
}

func (g *ConsumerGroup) AddPending(entryID, consumer string) {
	g.mu.Lock()
	defer g.mu.Unlock()
	g.addPendingLocked(entryID, consumer, time.Now().UnixMilli())
}

// addPendingLocked records a fresh delivery of entryID to consumer, taking
// the entry away from any consumer that held it.
func (g *ConsumerGroup) addPendingLocked(entryID, consumer string, now int64) {
	if previous, exists := g.Pending[entryID]; exists {
		if c, exists := g.Consumers[previous.Consumer]; exists {
			c.Pending--
		}
	}
	g.Pending[entryID] = &PendingEntry{
		ID:         entryID,
		Consumer:   consumer,
		DeliveryTS: now,
		Deliveries: 1,
	}
	if c, exists := g.Consumers[consumer]; exists {
		c.Pending++
	}
}

func (g *ConsumerGroup) Ack(entryID string) bool {
	g.mu.Lock()
	defer g.mu.Unlock()
	return g.ackLocked(entryID)
}

func (g *ConsumerGroup) ackLocked(entryID string) bool {
	if p, exists := g.Pending[entryID]; exists {
		if c, exists := g.Consumers[p.Consumer]; exists {
			c.Pending--
		}
		delete(g.Pending, entryID)
		return true
	}
	return false
}

// ClaimOptions tunes ClaimWithOptions: MinIdleTime gates claims to entries
// idle at least this many milliseconds; JustID suppresses the delivery-counter
// increment; SetRetry/RetryCount set the counter outright; Force claims stream
// entries that were never delivered; SetIdle/SetTime override the
// last-delivery timestamp (the IDLE/TIME options).
type ClaimOptions struct {
	MinIdleTime int64
	JustID      bool
	Force       bool
	RetryCount  int64
	SetRetry    bool
	IdleMS      int64
	SetIdle     bool
	TimeMS      int64
	SetTime     bool
}

func (g *ConsumerGroup) Claim(entryIDs []string, newConsumer string) []string {
	return g.ClaimWithOptions(entryIDs, newConsumer, ClaimOptions{})
}

func (g *ConsumerGroup) ClaimWithOptions(entryIDs []string, newConsumer string, opts ClaimOptions) []string {
	g.mu.Lock()
	defer g.mu.Unlock()
	return g.claimLocked(entryIDs, newConsumer, opts, time.Now().UnixMilli())
}

func (g *ConsumerGroup) claimLocked(entryIDs []string, newConsumer string, opts ClaimOptions, now int64) []string {
	deliveryTS := now
	switch {
	case opts.SetTime:
		deliveryTS = opts.TimeMS
	case opts.SetIdle:
		deliveryTS = now - opts.IdleMS
	}
	if deliveryTS < 0 || deliveryTS > now {
		deliveryTS = now
	}
	var claimed []string
	var c *Consumer
	for _, id := range entryIDs {
		p, exists := g.Pending[id]
		forced := false
		if !exists {
			if !opts.Force {
				continue
			}
			p = &PendingEntry{ID: id, Deliveries: 1}
			g.Pending[id] = p
			forced = true
		}
		if !forced && opts.MinIdleTime > 0 && now-p.DeliveryTS < opts.MinIdleTime {
			continue
		}
		if c == nil {
			c = g.consumerLocked(newConsumer, now)
		}
		if !forced && p.Consumer != newConsumer {
			if old, ok := g.Consumers[p.Consumer]; ok {
				old.Pending--
			}
		}
		p.DeliveryTS = deliveryTS
		if opts.SetRetry {
			p.Deliveries = opts.RetryCount
		} else if !opts.JustID {
			p.Deliveries++
		}
		if forced || p.Consumer != newConsumer {
			p.Consumer = newConsumer
			c.Pending++
		}
		c.ActiveTime = now
		claimed = append(claimed, id)
	}
	return claimed
}

// sortedPendingLocked returns the PEL in ID order.
func (g *ConsumerGroup) sortedPendingLocked() []*PendingEntry {
	type keyed struct {
		id StreamID
		p  *PendingEntry
	}
	rows := make([]keyed, 0, len(g.Pending))
	for _, p := range g.Pending {
		rows = append(rows, keyed{streamIDOf(p.ID), p})
	}
	sort.Slice(rows, func(i, j int) bool { return rows[i].id.Less(rows[j].id) })
	out := make([]*PendingEntry, len(rows))
	for i, r := range rows {
		out[i] = r.p
	}
	return out
}

func (g *ConsumerGroup) GetPending(start, end string, count int64) []*PendingEntry {
	g.mu.RLock()
	defer g.mu.RUnlock()
	startID, err := ParseStreamRangeID(start, 0)
	if err != nil {
		return nil
	}
	endID, err := ParseStreamRangeID(end, MaxStreamID.Seq)
	if err != nil {
		return nil
	}
	var result []*PendingEntry
	for _, p := range g.sortedPendingLocked() {
		id := streamIDOf(p.ID)
		if id.Less(startID) || endID.Less(id) {
			continue
		}
		result = append(result, p)
		if count > 0 && int64(len(result)) >= count {
			break
		}
	}
	return result
}

func (g *ConsumerGroup) GetPendingCount() int64 {
	g.mu.RLock()
	defer g.mu.RUnlock()
	return int64(len(g.Pending))
}

func (g *ConsumerGroup) GetFirstLastID() (string, string) {
	g.mu.RLock()
	defer g.mu.RUnlock()
	var firstID, lastID string
	var first, last StreamID
	for id := range g.Pending {
		sid := streamIDOf(id)
		if firstID == "" || sid.Less(first) {
			firstID, first = id, sid
		}
		if lastID == "" || last.Less(sid) {
			lastID, last = id, sid
		}
	}
	return firstID, lastID
}

func (g *ConsumerGroup) GetConsumerPending(consumerName string) int64 {
	g.mu.RLock()
	defer g.mu.RUnlock()
	if c, exists := g.Consumers[consumerName]; exists {
		return c.Pending
	}
	return 0
}

func (g *ConsumerGroup) GetAllConsumers() []string {
	g.mu.RLock()
	defer g.mu.RUnlock()
	consumers := make([]string, 0, len(g.Consumers))
	for name := range g.Consumers {
		consumers = append(consumers, name)
	}
	sort.Strings(consumers)
	return consumers
}

func (g *ConsumerGroup) GetConsumer(name string) *Consumer {
	g.mu.RLock()
	defer g.mu.RUnlock()
	return g.Consumers[name]
}

type StreamValue struct {
	mu sync.RWMutex
	// nodes pack the entries, oldest first; a node with no live entry left
	// is dropped. count is the number of live entries.
	nodes        []*streamNode
	count        int
	LastID       string
	MaxDeletedID string
	Length       int64
	MaxLen       int64
	// EntriesAdded counts every entry ever added, including the deleted.
	EntriesAdded int64
	Groups       map[string]*ConsumerGroup
	size         sizeCounter
}

func (v *StreamValue) Lock()    { v.mu.Lock() }
func (v *StreamValue) Unlock()  { v.mu.Unlock() }
func (v *StreamValue) RLock()   { v.mu.RLock() }
func (v *StreamValue) RUnlock() { v.mu.RUnlock() }

func NewStreamValue(maxLen int64) *StreamValue {
	return &StreamValue{
		MaxLen: maxLen,
		Groups: make(map[string]*ConsumerGroup),
	}
}

func (v *StreamValue) Type() DataType { return DataTypeStream }
func (v *StreamValue) SizeOf() int64 {
	if n, ok := v.size.load(); ok {
		return streamBaseSize + n
	}
	v.mu.RLock()
	defer v.mu.RUnlock()
	var n int64
	for _, node := range v.nodes {
		n += streamNodeOverhead + int64(len(node.buf))
	}
	v.size.set(n)
	return streamBaseSize + n
}

// RecountSize counts SizeOf from the nodes, for tests checking the
// maintained counter.
func (v *StreamValue) RecountSize() int64 {
	v.mu.RLock()
	defer v.mu.RUnlock()
	n := int64(streamBaseSize)
	for _, node := range v.nodes {
		n += streamNodeOverhead + int64(len(node.buf))
	}
	return n
}

func (v *StreamValue) String() string {
	v.mu.RLock()
	defer v.mu.RUnlock()

	result := ""
	for _, entry := range v.EntriesLocked() {
		if result != "" {
			result += "\n"
		}
		fields := ""
		for _, p := range entry.FieldPairs() {
			if fields != "" {
				fields += ", "
			}
			fields += p.Name + ": " + string(p.Value)
		}
		result += entry.ID + " -> " + fields
	}
	return result
}

func (v *StreamValue) Clone() Value {
	v.mu.RLock()
	defer v.mu.RUnlock()

	cloned := &StreamValue{
		nodes:        make([]*streamNode, len(v.nodes)),
		count:        v.count,
		LastID:       v.LastID,
		MaxDeletedID: v.MaxDeletedID,
		Length:       v.Length,
		MaxLen:       v.MaxLen,
		EntriesAdded: v.EntriesAdded,
		Groups:       make(map[string]*ConsumerGroup),
	}

	for i, node := range v.nodes {
		c := *node
		c.masterFields = append([]string(nil), node.masterFields...)
		c.buf = append([]byte(nil), node.buf...)
		cloned.nodes[i] = &c
	}

	for name, group := range v.Groups {
		group.mu.RLock()
		clonedGroup := &ConsumerGroup{
			Name:        group.Name,
			LastID:      group.LastID,
			EntriesRead: group.EntriesRead,
			Consumers:   make(map[string]*Consumer, len(group.Consumers)),
			Pending:     make(map[string]*PendingEntry, len(group.Pending)),
		}
		for consumerName, consumer := range group.Consumers {
			consumerCopy := *consumer
			clonedGroup.Consumers[consumerName] = &consumerCopy
		}
		for id, pending := range group.Pending {
			pendingCopy := *pending
			clonedGroup.Pending[id] = &pendingCopy
		}
		group.mu.RUnlock()
		cloned.Groups[name] = clonedGroup
	}

	return cloned
}

func (v *StreamValue) lastIDLocked() StreamID { return streamIDOf(v.LastID) }

func (v *StreamValue) firstIDLocked() StreamID {
	if pos, ok := v.firstPosLocked(); ok {
		_, id, _ := v.nodes[pos.node].head(pos.off)
		return id
	}
	return StreamID{}
}

func (v *StreamValue) maxDeletedLocked() StreamID { return streamIDOf(v.MaxDeletedID) }

// appendLocked stores a new entry with an already validated ID, packing a
// copy of its fields.
func (v *StreamValue) appendLocked(id StreamID, pairs []StreamField) *StreamEntry {
	var tail *streamNode
	if k := len(v.nodes); k > 0 {
		tail = v.nodes[k-1]
	}
	if tail == nil || tail.full(rawFieldBytes(pairs)) {
		if tail != nil {
			tail.seal()
		}
		tail = newStreamNode(id, pairs)
		v.nodes = append(v.nodes, tail)
		v.size.add(streamNodeOverhead)
	}
	off := len(tail.buf)
	v.size.add(int64(tail.add(id, pairs)))
	v.count++
	v.LastID = id.String()
	v.Length = int64(v.count)
	v.EntriesAdded++
	return tail.entry(off)
}

// LoadEntry appends an entry read from a dump or snapshot. It reports
// false when the ID does not follow the last one packed.
func (v *StreamValue) LoadEntry(id StreamID, pairs []StreamField) bool {
	v.mu.Lock()
	defer v.mu.Unlock()
	// Against the last ID packed, deleted or not: nodes keep their entries
	// in ID order.
	if k := len(v.nodes); k > 0 && !v.nodes[k-1].last.Less(id) {
		return false
	}
	v.appendLocked(id, pairs)
	return true
}

// EntriesLocked returns every entry, oldest first. The caller holds the
// read lock.
func (v *StreamValue) EntriesLocked() []*StreamEntry {
	out := make([]*StreamEntry, 0, v.count)
	v.eachLocked(streamPos{}, func(n *streamNode, off int, _ StreamID) bool {
		out = append(out, n.entry(off))
		return true
	})
	return out
}

// LenLocked is Len for a caller holding the lock.
func (v *StreamValue) LenLocked() int { return v.count }

// eachLocked calls fn on the live entries from pos on, oldest first, until
// fn returns false.
func (v *StreamValue) eachLocked(pos streamPos, fn func(n *streamNode, off int, id StreamID) bool) {
	for ni := pos.node; ni < len(v.nodes); ni++ {
		from := 0
		if ni == pos.node {
			from = pos.off
		}
		n := v.nodes[ni]
		if !n.each(from, func(off int, flags byte, id StreamID, _ int) bool {
			if flags&streamDeleted != 0 {
				return true
			}
			return fn(n, off, id)
		}) {
			return
		}
	}
}

func (v *StreamValue) firstPosLocked() (streamPos, bool) {
	return v.seekLocked(StreamID{})
}

// seekLocked returns the place of the first live entry with ID >= id.
func (v *StreamValue) seekLocked(id StreamID) (streamPos, bool) {
	ni := sort.Search(len(v.nodes), func(i int) bool { return !v.nodes[i].last.Less(id) })
	var pos streamPos
	found := false
	v.eachLocked(streamPos{node: ni}, func(n *streamNode, off int, eid StreamID) bool {
		if eid.Less(id) {
			return true
		}
		for ni < len(v.nodes) && v.nodes[ni] != n {
			ni++
		}
		pos, found = streamPos{node: ni, off: off}, true
		return false
	})
	return pos, found
}

// lastEntryIDLocked is the ID of the newest live entry.
func (v *StreamValue) lastEntryIDLocked() (StreamID, bool) {
	if len(v.nodes) == 0 {
		return StreamID{}, false
	}
	var last StreamID
	found := false
	v.nodes[len(v.nodes)-1].each(0, func(_ int, flags byte, id StreamID, _ int) bool {
		if flags&streamDeleted == 0 {
			last, found = id, true
		}
		return true
	})
	return last, found
}

// deleteAtLocked flags the live entry at pos deleted and drops its node
// once it has none left.
func (v *StreamValue) deleteAtLocked(pos streamPos) {
	n := v.nodes[pos.node]
	n.buf[pos.off] |= streamDeleted
	n.live--
	v.count--
	if n.live == 0 {
		v.size.add(-(streamNodeOverhead + int64(len(n.buf))))
		copy(v.nodes[pos.node:], v.nodes[pos.node+1:])
		v.nodes[len(v.nodes)-1] = nil
		v.nodes = v.nodes[:len(v.nodes)-1]
	}
	v.Length = int64(v.count)
}

// StreamAddArgs is the ID part of an XADD: IDGiven is false for "*", and
// SeqGiven is false for "<ms>-*".
type StreamAddArgs struct {
	ID       StreamID
	IDGiven  bool
	SeqGiven bool
}

// AddEntry appends an entry the way XADD does, generating or completing the
// ID against the stream's top item, and returns the stored entry.
func (v *StreamValue) AddEntry(args StreamAddArgs, pairs []StreamField, nowMS uint64) (*StreamEntry, error) {
	v.mu.Lock()
	defer v.mu.Unlock()
	return v.addEntryLocked(args, pairs, nowMS)
}

func (v *StreamValue) addEntryLocked(args StreamAddArgs, pairs []StreamField, nowMS uint64) (*StreamEntry, error) {
	last := v.lastIDLocked()
	if last == MaxStreamID {
		return nil, ErrStreamExhausted
	}
	var id StreamID
	switch {
	case !args.IDGiven:
		if nowMS > last.Ms {
			id = StreamID{Ms: nowMS}
		} else {
			id, _ = last.Next()
		}
	case !args.SeqGiven:
		id = StreamID{Ms: args.ID.Ms}
		if args.ID.Ms == last.Ms {
			if last.Seq == MaxStreamID.Seq {
				return nil, ErrStreamIDTooSmall
			}
			id.Seq = last.Seq + 1
		}
		if id.Compare(last) <= 0 {
			return nil, ErrStreamIDTooSmall
		}
	default:
		id = args.ID
		if id.IsZero() {
			return nil, ErrStreamIDZero
		}
		if id.Compare(last) <= 0 {
			return nil, ErrStreamIDTooSmall
		}
	}
	return v.appendLocked(id, pairs), nil
}

// XAdd appends an entry and then applies the trim clause, under one lock.
func (v *StreamValue) XAdd(args StreamAddArgs, pairs []StreamField, trim StreamTrimArgs, nowMS uint64) (*StreamEntry, error) {
	entry, _, err := v.XAddTrim(args, pairs, trim, nowMS)
	return entry, err
}

// XAddTrim is XAdd that also reports how many entries the trim removed.
func (v *StreamValue) XAddTrim(args StreamAddArgs, pairs []StreamField, trim StreamTrimArgs, nowMS uint64) (*StreamEntry, int64, error) {
	v.mu.Lock()
	defer v.mu.Unlock()
	entry, err := v.addEntryLocked(args, pairs, nowMS)
	if err != nil {
		return nil, 0, err
	}
	return entry, v.trimLocked(trim), nil
}

// HasConsumer reports whether the group exists and has the consumer.
func (v *StreamValue) HasConsumer(groupName, name string) bool {
	v.mu.RLock()
	defer v.mu.RUnlock()
	g, ok := v.Groups[groupName]
	if !ok {
		return false
	}
	g.mu.Lock()
	defer g.mu.Unlock()
	_, ok = g.Consumers[name]
	return ok
}

// StreamPairsFromMap lists fields sorted by name, the order of an entry
// given as a map.
func StreamPairsFromMap(fields map[string][]byte) []StreamField {
	if fields == nil {
		return nil
	}
	names := make([]string, 0, len(fields))
	for name := range fields {
		names = append(names, name)
	}
	sort.Strings(names)
	pairs := make([]StreamField, len(names))
	for i, name := range names {
		pairs[i] = StreamField{Name: name, Value: fields[name]}
	}
	return pairs
}

// Add appends an entry with an explicit "ms-seq" (or bare "ms") ID.
func (v *StreamValue) Add(id string, fields map[string][]byte) (*StreamEntry, error) {
	parsed, err := ParseStreamID(id, 0)
	if err != nil {
		return nil, err
	}
	if parsed.IsZero() {
		return nil, ErrStreamIDZero
	}

	v.mu.Lock()
	defer v.mu.Unlock()

	if parsed.Compare(v.lastIDLocked()) <= 0 {
		return nil, ErrStreamIDTooSmall
	}
	entry := v.appendLocked(parsed, StreamPairsFromMap(fields))
	if v.MaxLen > 0 && int64(v.count) > v.MaxLen {
		v.trimLocked(StreamTrimArgs{Strategy: StreamTrimMaxLen, MaxLen: v.MaxLen})
	}
	return entry, nil
}

// Range returns the entries with start <= ID <= end, at most count of them
// when count is positive, newest first when rev is set.
func (v *StreamValue) Range(start, end StreamID, count int64, rev bool) []*StreamEntry {
	v.mu.RLock()
	defer v.mu.RUnlock()
	return v.rangeLocked(start, end, count, rev)
}

func (v *StreamValue) rangeLocked(start, end StreamID, count int64, rev bool) []*StreamEntry {
	if end.Less(start) {
		return nil
	}
	var out []*StreamEntry
	full := func() bool { return count > 0 && int64(len(out)) >= count }
	if !rev {
		pos, ok := v.seekLocked(start)
		if !ok {
			return nil
		}
		v.eachLocked(pos, func(n *streamNode, off int, id StreamID) bool {
			if end.Less(id) {
				return false
			}
			out = append(out, n.entry(off))
			return !full()
		})
		return out
	}
	// Newest first: the nodes that may hold entries up to end, each read
	// forward and replied backward.
	ni := sort.Search(len(v.nodes), func(i int) bool { return end.Less(v.nodes[i].master) }) - 1
	var offs []int
	for ; ni >= 0 && !full(); ni-- {
		n := v.nodes[ni]
		if n.last.Less(start) {
			break
		}
		offs = offs[:0]
		n.each(0, func(off int, flags byte, id StreamID, _ int) bool {
			if end.Less(id) {
				return false
			}
			if flags&streamDeleted == 0 && !id.Less(start) {
				offs = append(offs, off)
			}
			return true
		})
		for i := len(offs) - 1; i >= 0 && !full(); i-- {
			out = append(out, n.entry(offs[i]))
		}
	}
	return out
}

// After returns up to count entries (all when count <= 0) with ID > id.
func (v *StreamValue) After(id StreamID, count int64) []*StreamEntry {
	v.mu.RLock()
	defer v.mu.RUnlock()
	start, ok := id.Next()
	if !ok {
		return nil
	}
	return v.rangeLocked(start, MaxStreamID, count, false)
}

// LastEntryID is the ID of the newest entry still in the stream.
func (v *StreamValue) LastEntryID() (StreamID, bool) {
	v.mu.RLock()
	defer v.mu.RUnlock()
	return v.lastEntryIDLocked()
}

// LastGeneratedID is the stream's top ID, which deletions never lower.
func (v *StreamValue) LastGeneratedID() StreamID {
	v.mu.RLock()
	defer v.mu.RUnlock()
	return v.lastIDLocked()
}

func (v *StreamValue) GetRange(start, end string, count int64) []*StreamEntry {
	startID, err := ParseStreamRangeID(start, 0)
	if err != nil {
		return nil
	}
	endID, err := ParseStreamRangeID(end, MaxStreamID.Seq)
	if err != nil {
		return nil
	}
	return v.Range(startID, endID, count, false)
}

func (v *StreamValue) Len() int64 {
	v.mu.RLock()
	defer v.mu.RUnlock()
	return int64(v.count)
}

// Delete removes the entries with the given IDs, as XDEL does, and returns
// how many existed.
func (v *StreamValue) Delete(ids ...string) int64 {
	parsed := make([]StreamID, 0, len(ids))
	for _, id := range ids {
		if sid, err := ParseStreamID(id, 0); err == nil {
			parsed = append(parsed, sid)
		}
	}
	return v.DeleteIDs(parsed)
}

// DeleteIDs is Delete for parsed IDs.
func (v *StreamValue) DeleteIDs(ids []StreamID) int64 {
	v.mu.Lock()
	defer v.mu.Unlock()

	deleted := int64(0)
	maxDel := v.maxDeletedLocked()
	for _, id := range ids {
		pos, ok := v.seekLocked(id)
		if !ok {
			continue
		}
		if _, eid, _ := v.nodes[pos.node].head(pos.off); eid != id {
			continue
		}
		v.deleteAtLocked(pos)
		if maxDel.Less(id) {
			maxDel = id
		}
		deleted++
	}
	if deleted > 0 {
		v.MaxDeletedID = maxDel.String()
	}
	return deleted
}

const (
	StreamTrimNone = iota
	StreamTrimMaxLen
	StreamTrimMinID
)

// StreamTrimArgs is a parsed MAXLEN/MINID clause. Limit 0 is unlimited.
type StreamTrimArgs struct {
	Strategy int
	MaxLen   int64
	MinID    StreamID
	Approx   bool
	Limit    int64
}

// TrimStream trims the way XTRIM does and returns the number of entries
// removed. Approximate trimming only removes whole nodes.
func (v *StreamValue) TrimStream(args StreamTrimArgs) int64 {
	v.mu.Lock()
	defer v.mu.Unlock()
	return v.trimLocked(args)
}

// removeHeadLocked removes the n oldest entries: whole nodes while n
// covers them, then flags entries of the next one deleted.
func (v *StreamValue) removeHeadLocked(n int) {
	for n > 0 && len(v.nodes) > 0 {
		node := v.nodes[0]
		if n >= node.live {
			n -= node.live
			v.count -= node.live
			v.size.add(-(streamNodeOverhead + int64(len(node.buf))))
			v.nodes[0] = nil
			v.nodes = v.nodes[1:]
			continue
		}
		var offs []int
		node.each(0, func(off int, flags byte, _ StreamID, _ int) bool {
			if flags&streamDeleted == 0 {
				offs = append(offs, off)
			}
			return len(offs) < n
		})
		for _, off := range offs {
			v.deleteAtLocked(streamPos{node: 0, off: off})
		}
		n = 0
	}
	v.Length = int64(v.count)
}

// countBeforeLocked counts the live entries with ID < id.
func (v *StreamValue) countBeforeLocked(id StreamID) int {
	n := 0
	v.eachLocked(streamPos{}, func(_ *streamNode, _ int, eid StreamID) bool {
		if !eid.Less(id) {
			return false
		}
		n++
		return true
	})
	return n
}

func (v *StreamValue) trimLocked(args StreamTrimArgs) int64 {
	if args.Strategy == StreamTrimNone {
		return 0
	}
	deleted := int64(0)
	for len(v.nodes) > 0 {
		length := int64(v.count)
		if args.Strategy == StreamTrimMaxLen && length <= args.MaxLen {
			break
		}
		node := v.nodes[0]
		entries := int64(node.live)
		if args.Limit > 0 && deleted+entries > args.Limit {
			break
		}
		var removeNode bool
		if args.Strategy == StreamTrimMaxLen {
			removeNode = length-entries >= args.MaxLen
		} else {
			removeNode = node.last.Less(args.MinID)
		}
		if removeNode {
			v.removeHeadLocked(int(entries))
			deleted += entries
			continue
		}
		if args.Approx {
			break
		}
		n := 0
		node.each(0, func(_ int, flags byte, id StreamID, _ int) bool {
			if flags&streamDeleted != 0 {
				return true
			}
			if args.Strategy == StreamTrimMaxLen && length-int64(n) <= args.MaxLen {
				return false
			}
			if args.Strategy == StreamTrimMinID && !id.Less(args.MinID) {
				return false
			}
			n++
			return true
		})
		if n > 0 {
			v.removeHeadLocked(n)
			deleted += int64(n)
		}
		break
	}
	return deleted
}

func (v *StreamValue) Trim(maxLen int64, _ bool) int64 {
	if maxLen < 0 {
		maxLen = 0
	}
	return v.TrimStream(StreamTrimArgs{Strategy: StreamTrimMaxLen, MaxLen: maxLen})
}

// TrimByMinID removes entries whose ID is numerically below minID.
func (v *StreamValue) TrimByMinID(minID string, _ bool) int64 {
	id, err := ParseStreamID(minID, 0)
	if err != nil {
		return 0
	}
	return v.TrimStream(StreamTrimArgs{Strategy: StreamTrimMinID, MinID: id})
}

// TrimLimited trims to maxLen but evicts at most limit entries when limit is
// positive; a limit of 0 (or absent) keeps the unbounded exact behavior.
func (v *StreamValue) TrimLimited(maxLen int64, limit int64) int64 {
	v.mu.Lock()
	defer v.mu.Unlock()
	if maxLen < 0 {
		maxLen = 0
	}
	length := int64(v.count)
	if maxLen >= length {
		return 0
	}
	remove := length - maxLen
	if limit > 0 && remove > limit {
		remove = limit
	}
	v.removeHeadLocked(int(remove))
	return remove
}

// TrimByMinIDLimited removes entries numerically below minID but evicts at
// most limit entries when limit is positive; 0 or negative keeps unlimited.
func (v *StreamValue) TrimByMinIDLimited(minID string, limit int64) int64 {
	id, err := ParseStreamID(minID, 0)
	if err != nil {
		return 0
	}
	v.mu.Lock()
	defer v.mu.Unlock()
	n := v.countBeforeLocked(id)
	if limit > 0 && int64(n) > limit {
		n = int(limit)
	}
	v.removeHeadLocked(n)
	return int64(n)
}

func (v *StreamValue) CreateGroup(name, lastID string) error {
	v.mu.Lock()
	defer v.mu.Unlock()

	if _, exists := v.Groups[name]; exists {
		return ErrKeyExists
	}

	group := NewConsumerGroup(name)
	if lastID == "$" {
		group.LastID = v.lastIDLocked().String()
	} else {
		group.LastID = lastID
	}
	if v.Groups == nil {
		v.Groups = make(map[string]*ConsumerGroup)
	}
	v.Groups[name] = group
	return nil
}

func (v *StreamValue) DestroyGroup(name string) bool {
	v.mu.Lock()
	defer v.mu.Unlock()

	if _, exists := v.Groups[name]; exists {
		delete(v.Groups, name)
		return true
	}
	return false
}

func (v *StreamValue) GetGroup(name string) *ConsumerGroup {
	v.mu.RLock()
	defer v.mu.RUnlock()
	return v.Groups[name]
}

func (v *StreamValue) SetGroupLastID(groupName, lastID string) bool {
	v.mu.Lock()
	defer v.mu.Unlock()
	group, exists := v.Groups[groupName]
	if !exists {
		return false
	}
	group.mu.Lock()
	if lastID == "$" {
		group.LastID = v.lastIDLocked().String()
	} else {
		group.LastID = lastID
	}
	group.mu.Unlock()
	return true
}

func (v *StreamValue) GetEntriesAfter(id string, count int64) []*StreamEntry {
	sid, err := ParseStreamID(id, 0)
	if err != nil {
		return nil
	}
	return v.After(sid, count)
}

func (v *StreamValue) GetEntryByID(id string) *StreamEntry {
	v.mu.RLock()
	defer v.mu.RUnlock()
	sid, err := ParseStreamID(id, 0)
	if err != nil {
		return nil
	}
	return v.entryLocked(sid)
}

func (v *StreamValue) entryLocked(id StreamID) *StreamEntry {
	pos, ok := v.seekLocked(id)
	if !ok {
		return nil
	}
	n := v.nodes[pos.node]
	if _, eid, _ := n.head(pos.off); eid != id {
		return nil
	}
	return n.entry(pos.off)
}

func (v *StreamValue) SetLastID(lastID string) {
	v.mu.Lock()
	defer v.mu.Unlock()
	v.LastID = lastID
}

func (v *StreamValue) GetLastID() string {
	v.mu.RLock()
	defer v.mu.RUnlock()
	return v.LastID
}

func (v *StreamValue) GetGroups() map[string]*ConsumerGroup {
	v.mu.RLock()
	defer v.mu.RUnlock()
	return v.Groups
}

func (v *StreamValue) GetGroupCount() int {
	v.mu.RLock()
	defer v.mu.RUnlock()
	return len(v.Groups)
}
