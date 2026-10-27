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

type StreamEntry struct {
	ID        string
	Fields    map[string][]byte
	CreatedAt time.Time
	// Pairs keeps the fields in the order XADD received them, duplicates
	// included. Entries built without it fall back to Fields sorted by name.
	Pairs []StreamField
	node  *streamNode
}

// FieldPairs returns the entry's fields in reply order.
func (e *StreamEntry) FieldPairs() []StreamField {
	if e.Pairs != nil {
		return e.Pairs
	}
	names := make([]string, 0, len(e.Fields))
	for name := range e.Fields {
		names = append(names, name)
	}
	sort.Strings(names)
	pairs := make([]StreamField, len(names))
	for i, name := range names {
		pairs[i] = StreamField{Name: name, Value: e.Fields[name]}
	}
	return pairs
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
	mu           sync.RWMutex
	Entries      []*StreamEntry
	LastID       string
	MaxDeletedID string
	Length       int64
	MaxLen       int64
	// EntriesAdded counts every entry ever added, including the deleted.
	EntriesAdded int64
	Groups       map[string]*ConsumerGroup
	nodeCount    int
	nodeLive     int
	size         sizeCounter
}

func (v *StreamValue) Lock()    { v.mu.Lock() }
func (v *StreamValue) Unlock()  { v.mu.Unlock() }
func (v *StreamValue) RLock()   { v.mu.RLock() }
func (v *StreamValue) RUnlock() { v.mu.RUnlock() }

func NewStreamValue(maxLen int64) *StreamValue {
	return &StreamValue{
		Entries: make([]*StreamEntry, 0),
		MaxLen:  maxLen,
		Groups:  make(map[string]*ConsumerGroup),
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
	for _, entry := range v.Entries {
		n += streamEntrySize(entry)
	}
	v.size.set(n)
	return streamBaseSize + n
}
func (v *StreamValue) String() string {
	v.mu.RLock()
	defer v.mu.RUnlock()

	result := ""
	for _, entry := range v.Entries {
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

func cloneStreamEntry(entry *StreamEntry) *StreamEntry {
	out := &StreamEntry{
		ID:        entry.ID,
		Fields:    make(map[string][]byte, len(entry.Fields)),
		CreatedAt: entry.CreatedAt,
	}
	for k, val := range entry.Fields {
		out.Fields[k] = append([]byte(nil), val...)
	}
	if entry.Pairs != nil {
		out.Pairs = make([]StreamField, len(entry.Pairs))
		for i, p := range entry.Pairs {
			out.Pairs[i] = StreamField{Name: p.Name, Value: append([]byte(nil), p.Value...)}
		}
	}
	return out
}

func (v *StreamValue) Clone() Value {
	v.mu.RLock()
	defer v.mu.RUnlock()

	cloned := &StreamValue{
		Entries:      make([]*StreamEntry, len(v.Entries)),
		LastID:       v.LastID,
		MaxDeletedID: v.MaxDeletedID,
		Length:       v.Length,
		MaxLen:       v.MaxLen,
		EntriesAdded: v.EntriesAdded,
		Groups:       make(map[string]*ConsumerGroup),
	}

	for i, entry := range v.Entries {
		cloned.Entries[i] = cloneStreamEntry(entry)
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
	if len(v.Entries) == 0 {
		return StreamID{}
	}
	return v.Entries[0].streamID()
}

func (v *StreamValue) maxDeletedLocked() StreamID { return streamIDOf(v.MaxDeletedID) }

// appendLocked stores a new entry with an already validated ID.
func (v *StreamValue) appendLocked(id StreamID, pairs []StreamField) *StreamEntry {
	v.ensureNodes()
	owned := make([]StreamField, len(pairs))
	fields := make(map[string][]byte, len(pairs))
	for i, p := range pairs {
		val := append([]byte(nil), p.Value...)
		owned[i] = StreamField{Name: p.Name, Value: val}
		fields[p.Name] = val
	}
	entry := &StreamEntry{ID: id.String(), Fields: fields, CreatedAt: time.Now(), Pairs: owned}
	var tail *streamNode
	if n := len(v.Entries); n > 0 {
		tail = v.Entries[n-1].node
	}
	v.placeEntry(tail, entry)
	v.Entries = append(v.Entries, entry)
	v.size.add(streamEntrySize(entry))
	v.LastID = entry.ID
	v.Length = int64(len(v.Entries))
	v.EntriesAdded++
	return entry
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

func pairsFromMap(fields map[string][]byte) []StreamField {
	if fields == nil {
		return nil
	}
	return (&StreamEntry{Fields: fields}).FieldPairs()
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
	entry := v.appendLocked(parsed, pairsFromMap(fields))
	if fields == nil {
		entry.Fields = nil
	}
	if v.MaxLen > 0 && int64(len(v.Entries)) > v.MaxLen {
		v.trimLocked(StreamTrimArgs{Strategy: StreamTrimMaxLen, MaxLen: v.MaxLen})
	}
	return entry, nil
}

// searchLocked returns the index of the first entry with ID >= id.
func (v *StreamValue) searchLocked(id StreamID) int {
	return sort.Search(len(v.Entries), func(i int) bool {
		return v.Entries[i].streamID().Compare(id) >= 0
	})
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
	lo := v.searchLocked(start)
	hi := sort.Search(len(v.Entries), func(i int) bool {
		return v.Entries[i].streamID().Compare(end) > 0
	})
	if lo >= hi {
		return nil
	}
	n := hi - lo
	if count > 0 && int64(n) > count {
		n = int(count)
	}
	out := make([]*StreamEntry, 0, n)
	if rev {
		for i := hi - 1; i >= lo && len(out) < n; i-- {
			out = append(out, v.Entries[i])
		}
	} else {
		for i := lo; i < hi && len(out) < n; i++ {
			out = append(out, v.Entries[i])
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
	if len(v.Entries) == 0 {
		return StreamID{}, false
	}
	return v.Entries[len(v.Entries)-1].streamID(), true
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
	return int64(len(v.Entries))
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
	v.ensureNodes()

	deleted := int64(0)
	maxDel := v.maxDeletedLocked()
	for _, id := range ids {
		i := v.searchLocked(id)
		if i >= len(v.Entries) || v.Entries[i].streamID() != id {
			continue
		}
		v.dropEntryNode(v.Entries[i])
		v.size.add(-streamEntrySize(v.Entries[i]))
		copy(v.Entries[i:], v.Entries[i+1:])
		v.Entries[len(v.Entries)-1] = nil
		v.Entries = v.Entries[:len(v.Entries)-1]
		if maxDel.Less(id) {
			maxDel = id
		}
		deleted++
	}
	if deleted > 0 {
		v.MaxDeletedID = maxDel.String()
	}
	v.Length = int64(len(v.Entries))
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

func (v *StreamValue) removeHeadLocked(n int) {
	for i := 0; i < n; i++ {
		v.dropEntryNode(v.Entries[i])
		v.size.add(-streamEntrySize(v.Entries[i]))
		v.Entries[i] = nil
	}
	v.Entries = v.Entries[n:]
	v.Length = int64(len(v.Entries))
}

func (v *StreamValue) trimLocked(args StreamTrimArgs) int64 {
	if args.Strategy == StreamTrimNone {
		return 0
	}
	v.ensureNodes()
	deleted := int64(0)
	for len(v.Entries) > 0 {
		length := int64(len(v.Entries))
		if args.Strategy == StreamTrimMaxLen && length <= args.MaxLen {
			break
		}
		node := v.Entries[0].node
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
		for n < int(entries) {
			if args.Strategy == StreamTrimMaxLen && length-int64(n) <= args.MaxLen {
				break
			}
			if args.Strategy == StreamTrimMinID && !v.Entries[n].streamID().Less(args.MinID) {
				break
			}
			n++
		}
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
	v.ensureNodes()
	if maxLen < 0 {
		maxLen = 0
	}
	length := int64(len(v.Entries))
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
	v.ensureNodes()
	n := v.searchLocked(id)
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
	i := v.searchLocked(id)
	if i < len(v.Entries) && v.Entries[i].streamID() == id {
		return v.Entries[i]
	}
	return nil
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
