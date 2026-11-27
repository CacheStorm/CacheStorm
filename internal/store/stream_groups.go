package store

import (
	"errors"
	"math"
	"sort"
)

// hasTombstonesLocked is Redis' streamRangeHasTombstones(s, start, NULL):
// whether the last entry deleted with XDEL lies at or after start.
func (v *StreamValue) hasTombstonesLocked(start StreamID) bool {
	if v.count == 0 {
		return false
	}
	maxDel := v.maxDeletedLocked()
	if maxDel.IsZero() {
		return false
	}
	return start.Compare(maxDel) <= 0
}

// estimateDistanceLocked is Redis' streamEstimateDistanceFromFirstEverEntry:
// the logical position of id counted from the first entry ever added, when
// it can be known.
func (v *StreamValue) estimateDistanceLocked(id StreamID) (int64, bool) {
	if v.EntriesAdded == 0 {
		return 0, true
	}
	length := int64(v.count)
	last := v.lastIDLocked()
	if length == 0 && id.Compare(last) < 1 {
		return v.EntriesAdded, true
	}
	maxDel := v.maxDeletedLocked()
	if !id.IsZero() && id.Less(maxDel) {
		// Entries deleted after id fragment the stream.
		return 0, false
	}
	switch c := id.Compare(last); {
	case c == 0:
		return v.EntriesAdded, true
	case c > 0:
		return 0, false
	}
	first := v.firstIDLocked()
	if maxDel.IsZero() || maxDel.Less(first) {
		switch id.Compare(first) {
		case -1:
			return v.EntriesAdded - length, true
		case 0:
			return v.EntriesAdded - length + 1, true
		}
	}
	return 0, false
}

// groupLagLocked is Redis' streamReplyWithCGLag.
func (v *StreamValue) groupLagLocked(g *ConsumerGroup) (int64, bool) {
	if v.EntriesAdded == 0 || v.count == 0 {
		return 0, true
	}
	last := streamIDOf(g.LastID)
	first := v.firstIDLocked()
	if last.Less(first) && v.maxDeletedLocked().Less(first) {
		// Every remaining entry comes after both the group's position and
		// the last tombstone.
		return int64(v.count), true
	}
	if g.EntriesRead != EntriesReadInvalid && !v.hasTombstonesLocked(last) {
		return v.EntriesAdded - g.EntriesRead, true
	}
	if read, ok := v.estimateDistanceLocked(last); ok {
		return v.EntriesAdded - read, true
	}
	return 0, false
}

// advanceGroupLocked moves the group's last-delivered ID to id, keeping its
// logical read counter in step, as streamReplyWithRange does.
func (v *StreamValue) advanceGroupLocked(g *ConsumerGroup, id StreamID) {
	last := streamIDOf(g.LastID)
	if !last.Less(id) {
		return
	}
	if g.EntriesRead != EntriesReadInvalid && last.Compare(v.firstIDLocked()) >= 0 && !v.hasTombstonesLocked(last) {
		g.EntriesRead++
	} else if v.EntriesAdded != 0 {
		if read, ok := v.estimateDistanceLocked(id); ok {
			g.EntriesRead = read
		} else {
			g.EntriesRead = EntriesReadInvalid
		}
	}
	g.LastID = id.String()
}

// StreamGroupItem is one entry of a group read or claim; Entry is nil when
// the entry was deleted after it was delivered.
type StreamGroupItem struct {
	ID    StreamID
	Entry *StreamEntry
	// For XREADGROUP CLAIM: how long the entry had been idle and how often
	// delivered before this read (0 and 0 for a new entry), and whether the
	// read took it from the PEL.
	Idle, Deliveries int64
	Claimed          bool
	// Pending reports that the read added or moved the entry in the PEL,
	// now delivered DeliveredCount times; replicas get it as an XCLAIM.
	Pending        bool
	DeliveredCount int64
}

// ReadGroupOptions are XREADGROUP's: History reads the consumer's own PEL
// after After; otherwise the read serves, with MinIdle >= 0 (CLAIM), the
// PEL entries idle at least MinIdle ms, then new entries.
type ReadGroupOptions struct {
	History bool
	After   StreamID
	Count   int64
	NoAck   bool
	MinIdle int64
	Now     int64
	// Admit, when set, is asked before each entry is delivered; false
	// ends the read there, the entry neither sent nor made pending, as
	// Redis 8.10's MAXSIZE does.
	Admit func(*StreamGroupItem) bool
}

// ReadGroupResult is what an XREADGROUP of one stream did.
type ReadGroupResult struct {
	Items  []StreamGroupItem
	Served bool
	// ConsumerCreated reports the read created the consumer.
	ConsumerCreated bool
	// LastIDBefore is the group's last delivered ID before the read;
	// Advanced reports the read moved it, to LastID with EntriesRead.
	LastIDBefore, LastID string
	Advanced             bool
	EntriesRead          int64
	// Stopped reports Admit ended the read.
	Stopped bool
}

// ClaimableAt returns when the oldest delivery in the group's PEL whose
// entry still exists becomes idle for minIdle ms, for XREADGROUP CLAIM to
// wake a blocked reader; false when no such entry is pending.
func (v *StreamValue) ClaimableAt(groupName string, minIdle int64) (int64, bool) {
	v.mu.RLock()
	defer v.mu.RUnlock()
	g, ok := v.Groups[groupName]
	if !ok {
		return 0, false
	}
	g.mu.RLock()
	defer g.mu.RUnlock()
	for _, p := range g.timeOrderLocked() {
		if v.entryLocked(streamIDOf(p.ID)) != nil {
			return p.DeliveryTS + minIdle, true
		}
	}
	return 0, false
}

// HasGroup reports whether the stream has a consumer group called name.
func (v *StreamValue) HasGroup(name string) bool {
	v.mu.RLock()
	defer v.mu.RUnlock()
	_, ok := v.Groups[name]
	return ok
}

// GroupHasNew reports whether the group has entries left to deliver with ">".
func (v *StreamValue) GroupHasNew(name string) (hasNew bool, exists bool) {
	v.mu.RLock()
	defer v.mu.RUnlock()
	g, ok := v.Groups[name]
	if !ok {
		return false, false
	}
	top, ok := v.lastEntryIDLocked()
	if !ok {
		return false, true
	}
	g.mu.RLock()
	defer g.mu.RUnlock()
	return streamIDOf(g.LastID).Less(top), true
}

// ReadGroup is XREADGROUP for one stream. It returns false when the
// group does not exist; served reports whether the stream belongs in the
// reply.
func (v *StreamValue) ReadGroup(groupName, consumerName string, history bool, after StreamID, count int64, noack bool, now int64) (items []StreamGroupItem, served bool, ok bool) {
	r, ok := v.ReadGroupWith(groupName, consumerName, ReadGroupOptions{History: history, After: after, Count: count, NoAck: noack, MinIdle: -1, Now: now})
	return r.Items, r.Served, ok
}

// ReadGroupWith is XREADGROUP for one stream, with Redis 8.4's CLAIM.
func (v *StreamValue) ReadGroupWith(groupName, consumerName string, o ReadGroupOptions) (ReadGroupResult, bool) {
	v.mu.Lock()
	defer v.mu.Unlock()
	g, exists := v.Groups[groupName]
	if !exists {
		return ReadGroupResult{}, false
	}
	g.mu.Lock()
	defer g.mu.Unlock()

	now := o.Now
	_, known := g.Consumers[consumerName]
	r := ReadGroupResult{ConsumerCreated: !known, LastIDBefore: g.LastID}
	c := g.consumerLocked(consumerName, now)
	c.SeenTime = now

	if o.History {
		for _, p := range g.sortedPendingLocked() {
			if !p.ownedBy(consumerName) {
				continue
			}
			id := streamIDOf(p.ID)
			if !o.After.Less(id) {
				continue
			}
			e := v.entryLocked(id)
			item := StreamGroupItem{ID: id, Entry: e}
			if o.Admit != nil && !o.Admit(&item) {
				break
			}
			if e != nil {
				g.stampLocked(p, now)
				p.bumpDeliveries()
			}
			r.Items = append(r.Items, item)
			if o.Count > 0 && int64(len(r.Items)) >= o.Count {
				break
			}
		}
		r.Served = true
		r.LastID, r.EntriesRead = g.LastID, g.EntriesRead
		return r, true
	}

	if o.MinIdle >= 0 {
		// The PEL oldest delivery first; it is claimable up to the first
		// entry not idle long enough. Entries deleted from the stream are
		// passed over.
		for _, p := range g.timeOrderLocked() {
			idle := now - p.DeliveryTS
			if idle < o.MinIdle {
				break
			}
			id := streamIDOf(p.ID)
			e := v.entryLocked(id)
			if e == nil {
				continue
			}
			item := StreamGroupItem{ID: id, Entry: e, Idle: idle, Deliveries: p.Deliveries, Claimed: true, Pending: true}
			if o.Admit != nil && !o.Admit(&item) {
				r.Stopped = true
				break
			}
			g.handOverLocked(p, consumerName, c)
			p.bumpDeliveries()
			g.stampLocked(p, now)
			c.ActiveTime = now
			item.DeliveredCount = p.Deliveries
			r.Items = append(r.Items, item)
			if o.Count > 0 && int64(len(r.Items)) >= o.Count {
				break
			}
		}
	}

	if !r.Stopped && (o.Count == 0 || int64(len(r.Items)) < o.Count) {
		if top, ok := v.lastEntryIDLocked(); ok {
			if last := streamIDOf(g.LastID); last.Less(top) {
				start, _ := last.Next()
				count := o.Count
				if count > 0 {
					count -= int64(len(r.Items))
				}
				for _, e := range v.rangeLocked(start, MaxStreamID, count, false) {
					id := e.streamID()
					item := StreamGroupItem{ID: id, Entry: e}
					if o.Admit != nil && !o.Admit(&item) {
						break
					}
					v.advanceGroupLocked(g, id)
					r.Advanced = true
					if !o.NoAck {
						g.addPendingLocked(e.ID, consumerName, now)
						c.ActiveTime = now
						item.Pending, item.DeliveredCount = true, 1
					}
					r.Items = append(r.Items, item)
				}
			}
		}
	}
	r.Served = len(r.Items) > 0
	r.LastID, r.EntriesRead = g.LastID, g.EntriesRead
	return r, true
}

// AckIDs removes ids from the group's PEL and returns how many were pending.
func (v *StreamValue) AckIDs(groupName string, ids []StreamID) (int64, bool) {
	v.mu.Lock()
	defer v.mu.Unlock()
	g, ok := v.Groups[groupName]
	if !ok {
		return 0, false
	}
	g.mu.Lock()
	defer g.mu.Unlock()
	acked := int64(0)
	for _, id := range ids {
		if g.ackLocked(id.String()) {
			acked++
		}
	}
	return acked, true
}

type StreamConsumerCount struct {
	Name  string
	Count int64
}

type StreamPendingSummary struct {
	Count     int64
	Min, Max  string
	Consumers []StreamConsumerCount
}

// PendingSummary is the short form of XPENDING.
func (v *StreamValue) PendingSummary(groupName string) (StreamPendingSummary, bool) {
	v.mu.RLock()
	defer v.mu.RUnlock()
	g, ok := v.Groups[groupName]
	if !ok {
		return StreamPendingSummary{}, false
	}
	g.mu.RLock()
	defer g.mu.RUnlock()
	var out StreamPendingSummary
	out.Count = int64(len(g.Pending))
	if out.Count == 0 {
		return out, true
	}
	pel := g.sortedPendingLocked()
	out.Min, out.Max = pel[0].ID, pel[len(pel)-1].ID
	counts := make(map[string]int64)
	for _, p := range pel {
		if !p.nacked {
			counts[p.Consumer]++
		}
	}
	for name, n := range counts {
		out.Consumers = append(out.Consumers, StreamConsumerCount{Name: name, Count: n})
	}
	sort.Slice(out.Consumers, func(i, j int) bool { return out.Consumers[i].Name < out.Consumers[j].Name })
	return out, true
}

// PendingRange is the extended form of XPENDING. A named consumer that does
// not exist yields no rows.
func (v *StreamValue) PendingRange(groupName string, start, end StreamID, count, minIdle int64, consumer string, byConsumer bool, now int64) ([]PendingEntry, bool) {
	v.mu.RLock()
	defer v.mu.RUnlock()
	g, ok := v.Groups[groupName]
	if !ok {
		return nil, false
	}
	g.mu.RLock()
	defer g.mu.RUnlock()
	if byConsumer {
		if _, exists := g.Consumers[consumer]; !exists {
			return nil, true
		}
	}
	var rows []PendingEntry
	if count <= 0 {
		return rows, true
	}
	for _, p := range g.sortedPendingLocked() {
		id := streamIDOf(p.ID)
		if id.Less(start) {
			continue
		}
		if end.Less(id) {
			break
		}
		if byConsumer && !p.ownedBy(consumer) {
			continue
		}
		if !p.nacked && minIdle > 0 && now-p.DeliveryTS < minIdle {
			continue
		}
		rows = append(rows, *p)
		if int64(len(rows)) >= count {
			break
		}
	}
	return rows, true
}

// StreamClaimArgs carries the XCLAIM options. DeliveryTS and RetryCount are
// -1 when not given.
type StreamClaimArgs struct {
	MinIdle    int64
	DeliveryTS int64
	RetryCount int64
	Force      bool
	JustID     bool
	LastID     StreamID
	// Restore makes FORCE recreate a pending entry whose stream entry was
	// deleted, which a client's XCLAIM ignores. A rewritten AOF and a full
	// sync carry the PEL as XCLAIM ... FORCE, and the master kept such
	// entries pending (XTRIM and XDEL leave them in the PEL), as Redis'
	// RDB does.
	Restore bool
}

// StreamClaimEffect is one change XCLAIM or XAUTOCLAIM made to the PEL, as
// replicas and the AOF are told of it: the entry's holder, delivery time
// and count afterwards, or, for an entry deleted from the stream, before
// it left the PEL. Released marks an entry XNACK had released.
type StreamClaimEffect struct {
	ID         StreamID
	Consumer   string
	DeliveryTS int64
	Deliveries int64
	Released   bool
	Deleted    bool
}

func claimEffect(id StreamID, p *PendingEntry, deleted bool) StreamClaimEffect {
	return StreamClaimEffect{ID: id, Consumer: p.Consumer, DeliveryTS: p.DeliveryTS, Deliveries: p.Deliveries, Released: p.nacked, Deleted: deleted}
}

// StreamClaimResult is what XCLAIM did: the entries to reply with, the PEL
// changes, the group's last delivered ID and read counter afterwards and
// whether LASTID moved them, and whether the consumer was created.
type StreamClaimResult struct {
	Items           []StreamGroupItem
	Effects         []StreamClaimEffect
	LastID          string
	EntriesRead     int64
	LastIDMoved     bool
	ConsumerCreated bool
}

// ClaimIDs is XCLAIM: it moves pending entries to consumer. Pending entries
// whose stream entry was deleted are dropped from the PEL.
func (v *StreamValue) ClaimIDs(groupName, consumerName string, ids []StreamID, a StreamClaimArgs, now int64) (StreamClaimResult, bool) {
	v.mu.Lock()
	defer v.mu.Unlock()
	g, ok := v.Groups[groupName]
	if !ok {
		return StreamClaimResult{}, false
	}
	g.mu.Lock()
	defer g.mu.Unlock()

	var r StreamClaimResult
	if streamIDOf(g.LastID).Less(a.LastID) {
		g.LastID = a.LastID.String()
		r.LastIDMoved = true
	}
	deliveryTS := a.DeliveryTS
	if deliveryTS < 0 || deliveryTS > now {
		deliveryTS = now
	}

	// Redis 7.4 creates the consumer even when nothing is claimed.
	_, known := g.Consumers[consumerName]
	r.ConsumerCreated = !known
	c := g.consumerLocked(consumerName, now)
	items := make([]StreamGroupItem, 0, len(ids))
	for _, id := range ids {
		key := id.String()
		p := g.Pending[key]
		e := v.entryLocked(id)
		if e == nil {
			if p != nil {
				r.Effects = append(r.Effects, claimEffect(id, p, true))
				g.ackLocked(key)
			} else if a.Force && a.Restore {
				p = &PendingEntry{ID: key, Consumer: consumerName, Deliveries: 1}
				if a.RetryCount >= 0 {
					p.Deliveries = a.RetryCount
				}
				g.Pending[key] = p
				c.Pending++
				g.stampLocked(p, deliveryTS)
			}
			continue
		}
		created := false
		if p == nil {
			if !a.Force {
				continue
			}
			p = &PendingEntry{ID: key, Deliveries: 1}
			g.Pending[key] = p
			created = true
		}
		if !created && !p.nacked && a.MinIdle > 0 && now-p.DeliveryTS < a.MinIdle {
			continue
		}
		if created {
			p.Consumer = consumerName
			c.Pending++
		} else {
			g.handOverLocked(p, consumerName, c)
		}
		g.stampLocked(p, deliveryTS)
		if a.RetryCount >= 0 {
			p.Deliveries = a.RetryCount
		} else if !a.JustID {
			p.bumpDeliveries()
		}
		c.ActiveTime = now
		items = append(items, StreamGroupItem{ID: id, Entry: e})
		r.Effects = append(r.Effects, claimEffect(id, p, false))
	}
	r.Items = items
	r.LastID, r.EntriesRead = g.LastID, g.EntriesRead
	return r, true
}

// AutoClaim is XAUTOCLAIM. It scans the PEL from start, claims entries idle
// for at least minIdle, drops entries deleted from the stream and returns the
// cursor for the next call (0-0 when the scan reached the end).
func (v *StreamValue) AutoClaim(groupName, consumerName string, start StreamID, minIdle, count int64, justID bool, now int64) (StreamAutoClaimResult, bool) {
	v.mu.Lock()
	defer v.mu.Unlock()
	g, ok := v.Groups[groupName]
	if !ok {
		return StreamAutoClaimResult{}, false
	}
	g.mu.Lock()
	defer g.mu.Unlock()

	pel := g.sortedPendingLocked()
	i := sort.Search(len(pel), func(i int) bool { return streamIDOf(pel[i].ID).Compare(start) >= 0 })
	attempts := count * 10
	_, known := g.Consumers[consumerName]
	r := StreamAutoClaimResult{ConsumerCreated: !known, LastID: g.LastID}
	c := g.consumerLocked(consumerName, now)
	items := []StreamGroupItem{}
	deleted := []StreamID{}
	for attempts > 0 && count > 0 && i < len(pel) {
		attempts--
		p := pel[i]
		i++
		id := streamIDOf(p.ID)
		e := v.entryLocked(id)
		if e == nil {
			r.Effects = append(r.Effects, claimEffect(id, p, true))
			g.ackLocked(p.ID)
			deleted = append(deleted, id)
			count--
			continue
		}
		if !p.nacked && minIdle > 0 && now-p.DeliveryTS < minIdle {
			continue
		}
		g.handOverLocked(p, consumerName, c)
		g.stampLocked(p, now)
		if !justID {
			p.bumpDeliveries()
		}
		c.ActiveTime = now
		items = append(items, StreamGroupItem{ID: id, Entry: e})
		r.Effects = append(r.Effects, claimEffect(id, p, false))
		count--
	}
	if i < len(pel) {
		r.Next = streamIDOf(pel[i].ID)
	}
	r.Items, r.Deleted = items, deleted
	return r, true
}

// StreamAutoClaimResult is what XAUTOCLAIM did: the cursor for the next
// call, the claimed entries, the IDs dropped as deleted from the stream,
// the PEL changes, the group's last delivered ID and whether the consumer
// was created.
type StreamAutoClaimResult struct {
	Next            StreamID
	Items           []StreamGroupItem
	Deleted         []StreamID
	Effects         []StreamClaimEffect
	LastID          string
	ConsumerCreated bool
}

// CreateGroupAt is XGROUP CREATE; useLast starts the group at the stream's
// top ID ("$"). It returns false when the group already exists.
func (v *StreamValue) CreateGroupAt(name string, id StreamID, useLast bool, entriesRead int64) bool {
	v.mu.Lock()
	defer v.mu.Unlock()
	if _, exists := v.Groups[name]; exists {
		return false
	}
	if useLast {
		id = v.lastIDLocked()
	}
	g := NewConsumerGroup(name)
	g.LastID = id.String()
	g.EntriesRead = entriesRead
	if v.Groups == nil {
		v.Groups = make(map[string]*ConsumerGroup)
	}
	v.Groups[name] = g
	return true
}

// SetGroupID is XGROUP SETID.
func (v *StreamValue) SetGroupID(name string, id StreamID, useLast bool, entriesRead int64) bool {
	v.mu.Lock()
	defer v.mu.Unlock()
	g, ok := v.Groups[name]
	if !ok {
		return false
	}
	if useLast {
		id = v.lastIDLocked()
	}
	g.mu.Lock()
	g.LastID = id.String()
	g.EntriesRead = entriesRead
	g.mu.Unlock()
	return true
}

// CreateConsumer is XGROUP CREATECONSUMER.
func (v *StreamValue) CreateConsumer(groupName, name string, now int64) (created bool, ok bool) {
	v.mu.Lock()
	defer v.mu.Unlock()
	g, exists := v.Groups[groupName]
	if !exists {
		return false, false
	}
	g.mu.Lock()
	defer g.mu.Unlock()
	if _, exists := g.Consumers[name]; exists {
		return false, true
	}
	g.Consumers[name] = newStreamConsumer(name, now)
	return true, true
}

// DeleteConsumer is XGROUP DELCONSUMER: the consumer and its pending entries
// are removed, and the number of those entries is returned.
func (v *StreamValue) DeleteConsumer(groupName, name string) (int64, bool) {
	v.mu.Lock()
	defer v.mu.Unlock()
	g, exists := v.Groups[groupName]
	if !exists {
		return 0, false
	}
	g.mu.Lock()
	defer g.mu.Unlock()
	if _, exists := g.Consumers[name]; !exists {
		return 0, true
	}
	pending := int64(0)
	for id, p := range g.Pending {
		if p.ownedBy(name) {
			delete(g.Pending, id)
			pending++
		}
	}
	delete(g.Consumers, name)
	return pending, true
}

var (
	errXSetIDSmallerThanTop     = errors.New("ERR The ID specified in XSETID is smaller than the target stream top item")
	errXSetIDEntriesAdded       = errors.New("ERR The entries_added specified in XSETID is smaller than the target stream length")
	errXSetIDSmallerThanDeleted = errors.New("ERR The ID specified in XSETID is smaller than current max_deleted_entry_id")
)

// SetID is XSETID. entriesAdded is -1 when not given; a zero maxDeleted
// leaves the recorded maximal deleted ID alone.
func (v *StreamValue) SetID(id StreamID, entriesAdded int64, maxDeleted StreamID) error {
	v.mu.Lock()
	defer v.mu.Unlock()
	if id.Less(v.maxDeletedLocked()) {
		return errXSetIDSmallerThanDeleted
	}
	if top, ok := v.lastEntryIDLocked(); ok {
		if id.Less(top) {
			return errXSetIDSmallerThanTop
		}
		if entriesAdded != -1 && int64(v.count) > entriesAdded {
			return errXSetIDEntriesAdded
		}
	}
	v.LastID = id.String()
	if entriesAdded != -1 {
		v.EntriesAdded = entriesAdded
	}
	if !maxDeleted.IsZero() {
		v.MaxDeletedID = maxDeleted.String()
	}
	return nil
}

type StreamConsumerInfo struct {
	Name       string
	SeenTime   int64
	ActiveTime int64
	Pending    int64
	PEL        []PendingEntry
}

type StreamGroupInfo struct {
	Name        string
	Consumers   int64
	Pending     int64
	Nacked      int64
	LastID      string
	EntriesRead int64
	Lag         int64
	LagValid    bool
	PEL         []PendingEntry
	ConsumerSet []StreamConsumerInfo
}

type StreamInfo struct {
	Length       int64
	Nodes        int
	RadixNodes   int
	LastID       string
	MaxDeletedID string
	EntriesAdded int64
	FirstID      string
	Groups       int
	First, Last  *StreamEntry
	Entries      []*StreamEntry
	GroupInfos   []StreamGroupInfo
}

func limitPending(pel []*PendingEntry, count int64, consumer string, byConsumer bool) []PendingEntry {
	out := []PendingEntry{}
	for _, p := range pel {
		if byConsumer && !p.ownedBy(consumer) {
			continue
		}
		if count > 0 && int64(len(out)) >= count {
			break
		}
		out = append(out, *p)
	}
	return out
}

func (v *StreamValue) groupInfoLocked(g *ConsumerGroup, full bool, count int64) StreamGroupInfo {
	g.mu.RLock()
	defer g.mu.RUnlock()
	info := StreamGroupInfo{
		Name:        g.Name,
		Consumers:   int64(len(g.Consumers)),
		Pending:     int64(len(g.Pending)),
		Nacked:      g.nackedCountLocked(),
		LastID:      streamIDOf(g.LastID).String(),
		EntriesRead: g.EntriesRead,
	}
	info.Lag, info.LagValid = v.groupLagLocked(g)
	if !full {
		return info
	}
	pel := g.sortedPendingLocked()
	info.PEL = limitPending(pel, count, "", false)
	names := make([]string, 0, len(g.Consumers))
	for name := range g.Consumers {
		names = append(names, name)
	}
	sort.Strings(names)
	for _, name := range names {
		c := g.Consumers[name]
		own := limitPending(pel, 0, name, true)
		ci := StreamConsumerInfo{Name: name, SeenTime: c.SeenTime, ActiveTime: c.ActiveTime, Pending: int64(len(own))}
		if count > 0 && int64(len(own)) > count {
			own = own[:count]
		}
		ci.PEL = own
		info.ConsumerSet = append(info.ConsumerSet, ci)
	}
	return info
}

func (v *StreamValue) sortedGroupsLocked() []*ConsumerGroup {
	groups := make([]*ConsumerGroup, 0, len(v.Groups))
	for _, g := range v.Groups {
		groups = append(groups, g)
	}
	sort.Slice(groups, func(i, j int) bool { return groups[i].Name < groups[j].Name })
	return groups
}

// Info is XINFO STREAM; count limits the FULL listings (0 lists all).
func (v *StreamValue) Info(full bool, count int64) StreamInfo {
	v.mu.RLock()
	defer v.mu.RUnlock()
	info := StreamInfo{
		Length:       int64(v.count),
		Nodes:        len(v.nodes),
		RadixNodes:   streamRaxNodes(v.nodes),
		LastID:       v.lastIDLocked().String(),
		MaxDeletedID: v.maxDeletedLocked().String(),
		EntriesAdded: v.EntriesAdded,
		FirstID:      v.firstIDLocked().String(),
		Groups:       len(v.Groups),
	}
	if info.EntriesAdded < info.Length {
		info.EntriesAdded = info.Length
	}
	if v.count > 0 {
		info.First = v.rangeLocked(StreamID{}, MaxStreamID, 1, false)[0]
		info.Last = v.rangeLocked(StreamID{}, MaxStreamID, 1, true)[0]
	}
	if !full {
		return info
	}
	n := int64(0)
	if count > 0 {
		n = count
	}
	info.Entries = v.rangeLocked(StreamID{}, MaxStreamID, n, false)
	for _, g := range v.sortedGroupsLocked() {
		info.GroupInfos = append(info.GroupInfos, v.groupInfoLocked(g, true, count))
	}
	return info
}

// GroupsInfo is XINFO GROUPS.
func (v *StreamValue) GroupsInfo() []StreamGroupInfo {
	v.mu.RLock()
	defer v.mu.RUnlock()
	out := []StreamGroupInfo{}
	for _, g := range v.sortedGroupsLocked() {
		out = append(out, v.groupInfoLocked(g, false, 0))
	}
	return out
}

// ConsumersInfo is XINFO CONSUMERS.
func (v *StreamValue) ConsumersInfo(groupName string) ([]StreamConsumerInfo, bool) {
	v.mu.RLock()
	defer v.mu.RUnlock()
	g, ok := v.Groups[groupName]
	if !ok {
		return nil, false
	}
	g.mu.RLock()
	defer g.mu.RUnlock()
	counts := make(map[string]int64)
	for _, p := range g.Pending {
		if !p.nacked {
			counts[p.Consumer]++
		}
	}
	out := []StreamConsumerInfo{}
	for name, c := range g.Consumers {
		out = append(out, StreamConsumerInfo{Name: name, SeenTime: c.SeenTime, ActiveTime: c.ActiveTime, Pending: counts[name]})
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Name < out[j].Name })
	return out, true
}

// XNACK modes: how releasing an entry changes its delivery count.
const (
	NackSilent = iota
	NackFail
	NackFatal
)

// Nack is XNACK: each pending entry among ids loses its consumer and moves
// to the end of the NACK zone, the head of the PEL's time order, so the next
// XREADGROUP CLAIM takes it first. SILENT takes the delivery back, FAIL
// keeps the count and FATAL raises it to the maximum; a retryCount of 0 or
// more sets it instead. force creates the entry for an ID in the stream
// that no consumer was delivered. It returns how many entries it released,
// and false when the group does not exist.
func (v *StreamValue) Nack(groupName string, ids []StreamID, mode int, retryCount int64, force bool) (int64, bool) {
	v.mu.Lock()
	defer v.mu.Unlock()
	g, ok := v.Groups[groupName]
	if !ok {
		return 0, false
	}
	g.mu.Lock()
	defer g.mu.Unlock()
	released := int64(0)
	for _, id := range ids {
		key := id.String()
		p := g.Pending[key]
		if p == nil {
			if !force || v.entryLocked(id) == nil {
				continue
			}
			p = &PendingEntry{ID: key}
			g.Pending[key] = p
		} else {
			g.releaseLocked(p)
		}
		switch {
		case retryCount >= 0:
			p.Deliveries = retryCount
		case mode == NackSilent:
			if p.Deliveries > 0 {
				p.Deliveries--
			}
		case mode == NackFatal:
			p.Deliveries = math.MaxInt64
		}
		g.pelSeq++
		p.Consumer, p.DeliveryTS, p.seq, p.nacked = "", 0, g.pelSeq, true
		released++
	}
	return released, true
}
