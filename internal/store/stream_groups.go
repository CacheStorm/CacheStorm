package store

import (
	"errors"
	"sort"
)

// hasTombstonesLocked reports whether an entry deleted with XDEL may lie at or
// after start, which makes logical read counters unreliable past it.
func (v *StreamValue) hasTombstonesLocked(start StreamID) bool {
	if len(v.Entries) == 0 {
		return false
	}
	maxDel := v.maxDeletedLocked()
	if maxDel.IsZero() {
		return false
	}
	if v.firstIDLocked().Compare(maxDel) > 0 {
		return false
	}
	return start.Compare(maxDel) <= 0
}

// estimateDistanceLocked returns the logical position of id counted from the
// first entry ever added, when it can be known.
func (v *StreamValue) estimateDistanceLocked(id StreamID) (int64, bool) {
	if v.EntriesAdded == 0 {
		return 0, true
	}
	length := int64(len(v.Entries))
	last := v.lastIDLocked()
	if length == 0 && id.Compare(last) < 1 {
		return v.EntriesAdded, true
	}
	switch c := id.Compare(last); {
	case c == 0:
		return v.EntriesAdded, true
	case c > 0:
		return 0, false
	}
	maxDel := v.maxDeletedLocked()
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

func (v *StreamValue) groupLagLocked(g *ConsumerGroup) (int64, bool) {
	if v.EntriesAdded == 0 {
		return 0, true
	}
	last := streamIDOf(g.LastID)
	if g.EntriesRead != EntriesReadInvalid && !v.hasTombstonesLocked(last) && last.Compare(v.firstIDLocked()) >= 0 {
		return v.EntriesAdded - g.EntriesRead, true
	}
	if read, ok := v.estimateDistanceLocked(last); ok {
		return v.EntriesAdded - read, true
	}
	return 0, false
}

// advanceGroupLocked moves the group's last-delivered ID to id, keeping its
// logical read counter in step.
func (v *StreamValue) advanceGroupLocked(g *ConsumerGroup, id StreamID) {
	if !streamIDOf(g.LastID).Less(id) {
		return
	}
	if g.EntriesRead != EntriesReadInvalid && !v.hasTombstonesLocked(id) {
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
	if len(v.Entries) == 0 {
		return false, true
	}
	g.mu.RLock()
	defer g.mu.RUnlock()
	return streamIDOf(g.LastID).Less(v.Entries[len(v.Entries)-1].streamID()), true
}

// ReadGroup serves XREADGROUP for one stream. history is false for the ">"
// ID; otherwise the consumer's own pending entries after `after` are
// returned. served reports whether the stream belongs in the reply.
func (v *StreamValue) ReadGroup(groupName, consumerName string, history bool, after StreamID, count int64, noack bool, now int64) (items []StreamGroupItem, served bool, ok bool) {
	v.mu.Lock()
	defer v.mu.Unlock()
	g, exists := v.Groups[groupName]
	if !exists {
		return nil, false, false
	}
	g.mu.Lock()
	defer g.mu.Unlock()

	c := g.consumerLocked(consumerName, now)
	c.SeenTime = now

	if history {
		for _, p := range g.sortedPendingLocked() {
			if p.Consumer != consumerName {
				continue
			}
			id := streamIDOf(p.ID)
			if !after.Less(id) {
				continue
			}
			e := v.entryLocked(id)
			if e != nil {
				p.DeliveryTS = now
				p.Deliveries++
			}
			items = append(items, StreamGroupItem{ID: id, Entry: e})
			if count > 0 && int64(len(items)) >= count {
				break
			}
		}
		return items, true, true
	}

	if len(v.Entries) == 0 {
		return nil, false, true
	}
	last := streamIDOf(g.LastID)
	if !last.Less(v.Entries[len(v.Entries)-1].streamID()) {
		return nil, false, true
	}
	start, _ := last.Next()
	for _, e := range v.rangeLocked(start, MaxStreamID, count, false) {
		id := e.streamID()
		v.advanceGroupLocked(g, id)
		if !noack {
			g.addPendingLocked(e.ID, consumerName, now)
			c.ActiveTime = now
		}
		items = append(items, StreamGroupItem{ID: id, Entry: e})
	}
	return items, true, true
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
		counts[p.Consumer]++
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
		if byConsumer && p.Consumer != consumer {
			continue
		}
		if minIdle > 0 && now-p.DeliveryTS < minIdle {
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
}

// ClaimIDs is XCLAIM: it moves pending entries to consumer. Pending entries
// whose stream entry was deleted are dropped from the PEL.
func (v *StreamValue) ClaimIDs(groupName, consumerName string, ids []StreamID, a StreamClaimArgs, now int64) ([]StreamGroupItem, bool) {
	v.mu.Lock()
	defer v.mu.Unlock()
	g, ok := v.Groups[groupName]
	if !ok {
		return nil, false
	}
	g.mu.Lock()
	defer g.mu.Unlock()

	if streamIDOf(g.LastID).Less(a.LastID) {
		g.LastID = a.LastID.String()
	}
	deliveryTS := a.DeliveryTS
	if deliveryTS < 0 || deliveryTS > now {
		deliveryTS = now
	}

	var c *Consumer
	items := make([]StreamGroupItem, 0, len(ids))
	for _, id := range ids {
		key := id.String()
		p := g.Pending[key]
		e := v.entryLocked(id)
		if e == nil {
			if p != nil {
				g.ackLocked(key)
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
		if !created && a.MinIdle > 0 && now-p.DeliveryTS < a.MinIdle {
			continue
		}
		if c == nil {
			c = g.consumerLocked(consumerName, now)
		}
		if created || p.Consumer != consumerName {
			if !created {
				if old, exists := g.Consumers[p.Consumer]; exists {
					old.Pending--
				}
			}
			p.Consumer = consumerName
			c.Pending++
		}
		p.DeliveryTS = deliveryTS
		if a.RetryCount >= 0 {
			p.Deliveries = a.RetryCount
		} else if !a.JustID {
			p.Deliveries++
		}
		c.ActiveTime = now
		items = append(items, StreamGroupItem{ID: id, Entry: e})
	}
	return items, true
}

// AutoClaim is XAUTOCLAIM. It scans the PEL from start, claims entries idle
// for at least minIdle, drops entries deleted from the stream and returns the
// cursor for the next call (0-0 when the scan reached the end).
func (v *StreamValue) AutoClaim(groupName, consumerName string, start StreamID, minIdle, count int64, justID bool, now int64) (StreamID, []StreamGroupItem, []StreamID, bool) {
	v.mu.Lock()
	defer v.mu.Unlock()
	g, ok := v.Groups[groupName]
	if !ok {
		return StreamID{}, nil, nil, false
	}
	g.mu.Lock()
	defer g.mu.Unlock()

	pel := g.sortedPendingLocked()
	i := sort.Search(len(pel), func(i int) bool { return streamIDOf(pel[i].ID).Compare(start) >= 0 })
	attempts := count * 10
	var c *Consumer
	items := []StreamGroupItem{}
	deleted := []StreamID{}
	for attempts > 0 && count > 0 && i < len(pel) {
		attempts--
		p := pel[i]
		i++
		id := streamIDOf(p.ID)
		e := v.entryLocked(id)
		if e == nil {
			g.ackLocked(p.ID)
			deleted = append(deleted, id)
			count--
			continue
		}
		if minIdle > 0 && now-p.DeliveryTS < minIdle {
			continue
		}
		if c == nil {
			c = g.consumerLocked(consumerName, now)
		}
		if p.Consumer != consumerName {
			if old, exists := g.Consumers[p.Consumer]; exists {
				old.Pending--
			}
			p.Consumer = consumerName
			c.Pending++
		}
		p.DeliveryTS = now
		if !justID {
			p.Deliveries++
		}
		c.ActiveTime = now
		items = append(items, StreamGroupItem{ID: id, Entry: e})
		count--
	}
	next := StreamID{}
	if i < len(pel) {
		next = streamIDOf(pel[i].ID)
	}
	return next, items, deleted, true
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
		if p.Consumer == name {
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
	if n := len(v.Entries); n > 0 {
		if id.Less(v.Entries[n-1].streamID()) {
			return errXSetIDSmallerThanTop
		}
		if entriesAdded != -1 && int64(n) > entriesAdded {
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
		if byConsumer && p.Consumer != consumer {
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
		Length:       int64(len(v.Entries)),
		Nodes:        v.countNodes(),
		LastID:       v.lastIDLocked().String(),
		MaxDeletedID: v.maxDeletedLocked().String(),
		EntriesAdded: v.EntriesAdded,
		FirstID:      v.firstIDLocked().String(),
		Groups:       len(v.Groups),
	}
	if info.EntriesAdded < info.Length {
		info.EntriesAdded = info.Length
	}
	if n := len(v.Entries); n > 0 {
		info.First, info.Last = v.Entries[0], v.Entries[n-1]
	}
	if !full {
		return info
	}
	n := len(v.Entries)
	if count > 0 && int64(n) > count {
		n = int(count)
	}
	info.Entries = append([]*StreamEntry(nil), v.Entries[:n]...)
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
		counts[p.Consumer]++
	}
	out := []StreamConsumerInfo{}
	for name, c := range g.Consumers {
		out = append(out, StreamConsumerInfo{Name: name, SeenTime: c.SeenTime, ActiveTime: c.ActiveTime, Pending: counts[name]})
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Name < out[j].Name })
	return out, true
}
