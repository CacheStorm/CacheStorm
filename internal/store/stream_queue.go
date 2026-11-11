package store

// The helpers below let a work queue built on a consumer group (MSGQUEUE)
// look at a pending entry and change it in one step, under the stream's
// lock, so two clients cannot both take or settle the same message.

// TakeForGroup delivers one entry to consumer: the first pending entry, in
// ID order, that redeliver accepts is claimed by consumer with its delivery
// count raised, or else the group's next new entry is delivered. found is
// false when there is nothing to deliver, ok false when the group is
// missing. entriesRead is the group's read counter after a new delivery.
func (v *StreamValue) TakeForGroup(groupName, consumer string, redeliver func(PendingEntry) bool, now int64) (item StreamGroupItem, p PendingEntry, entriesRead int64, claimed, found, ok bool) {
	v.mu.Lock()
	defer v.mu.Unlock()
	g, exists := v.Groups[groupName]
	if !exists {
		return item, p, 0, false, false, false
	}
	g.mu.Lock()
	defer g.mu.Unlock()
	for _, pe := range g.sortedPendingLocked() {
		if !redeliver(*pe) {
			continue
		}
		id := streamIDOf(pe.ID)
		e := v.entryLocked(id)
		if e == nil {
			g.ackLocked(pe.ID)
			continue
		}
		c := g.consumerLocked(consumer, now)
		if pe.Consumer != consumer {
			if old, exists := g.Consumers[pe.Consumer]; exists {
				old.Pending--
			}
			pe.Consumer = consumer
			c.Pending++
		}
		pe.DeliveryTS = now
		pe.Deliveries++
		c.ActiveTime, c.SeenTime = now, now
		return StreamGroupItem{ID: id, Entry: e}, *pe, 0, true, true, true
	}
	top, has := v.lastEntryIDLocked()
	last := streamIDOf(g.LastID)
	if !has || !last.Less(top) {
		return item, p, 0, false, false, true
	}
	start, _ := last.Next()
	entries := v.rangeLocked(start, MaxStreamID, 1, false)
	if len(entries) == 0 {
		return item, p, 0, false, false, true
	}
	e := entries[0]
	id := e.streamID()
	c := g.consumerLocked(consumer, now)
	v.advanceGroupLocked(g, id)
	g.addPendingLocked(e.ID, consumer, now)
	c.ActiveTime, c.SeenTime = now, now
	return StreamGroupItem{ID: id, Entry: e}, *g.Pending[e.ID], g.EntriesRead, false, true, true
}

// MovePendingIf hands the pending entry id to the consumer decide names,
// with its delivery time set to now and its delivery count to the count
// decide returns; decide sees the entry as it is and may refuse. It
// returns where the entry went and whether it moved.
func (v *StreamValue) MovePendingIf(groupName string, id StreamID, now int64, decide func(PendingEntry) (string, int64, bool)) (consumer string, deliveries int64, moved bool) {
	v.mu.Lock()
	defer v.mu.Unlock()
	g, exists := v.Groups[groupName]
	if !exists {
		return "", 0, false
	}
	g.mu.Lock()
	defer g.mu.Unlock()
	pe := g.Pending[id.String()]
	if pe == nil || v.entryLocked(id) == nil {
		return "", 0, false
	}
	consumer, deliveries, moved = decide(*pe)
	if !moved {
		return "", 0, false
	}
	c := g.consumerLocked(consumer, now)
	if pe.Consumer != consumer {
		if old, exists := g.Consumers[pe.Consumer]; exists {
			old.Pending--
		}
		pe.Consumer = consumer
		c.Pending++
	}
	pe.DeliveryTS = now
	pe.Deliveries = deliveries
	c.ActiveTime = now
	return consumer, deliveries, true
}

// AckAndDeleteIf acknowledges the pending entry id and deletes it from the
// stream when cond accepts it, and reports whether it did.
func (v *StreamValue) AckAndDeleteIf(groupName string, id StreamID, cond func(PendingEntry) bool) bool {
	v.mu.Lock()
	g, exists := v.Groups[groupName]
	if !exists {
		v.mu.Unlock()
		return false
	}
	g.mu.Lock()
	pe := g.Pending[id.String()]
	if pe == nil || !cond(*pe) {
		g.mu.Unlock()
		v.mu.Unlock()
		return false
	}
	g.ackLocked(pe.ID)
	g.mu.Unlock()
	v.mu.Unlock()
	v.DeleteIDs([]StreamID{id})
	return true
}

// PendingOf returns a copy of the group's pending entries in ID order.
func (v *StreamValue) PendingOf(groupName string) ([]PendingEntry, bool) {
	v.mu.RLock()
	defer v.mu.RUnlock()
	g, exists := v.Groups[groupName]
	if !exists {
		return nil, false
	}
	g.mu.RLock()
	defer g.mu.RUnlock()
	pel := g.sortedPendingLocked()
	out := make([]PendingEntry, len(pel))
	for i, p := range pel {
		out[i] = *p
	}
	return out, true
}

// Undelivered returns the IDs of the entries after the group's last
// delivered ID.
func (v *StreamValue) Undelivered(groupName string) ([]StreamID, bool) {
	v.mu.RLock()
	defer v.mu.RUnlock()
	g, exists := v.Groups[groupName]
	if !exists {
		return nil, false
	}
	g.mu.RLock()
	last := streamIDOf(g.LastID)
	g.mu.RUnlock()
	start, ok := last.Next()
	if !ok {
		return nil, true
	}
	var ids []StreamID
	for _, e := range v.rangeLocked(start, MaxStreamID, 0, false) {
		ids = append(ids, e.streamID())
	}
	return ids, true
}

// UndeliveredCount counts the entries after the group's last delivered ID.
func (v *StreamValue) UndeliveredCount(groupName string) (int64, bool) {
	v.mu.RLock()
	defer v.mu.RUnlock()
	g, exists := v.Groups[groupName]
	if !exists {
		return 0, false
	}
	g.mu.RLock()
	last := streamIDOf(g.LastID)
	g.mu.RUnlock()
	start, ok := last.Next()
	if !ok {
		return 0, true
	}
	return int64(v.count - v.countBeforeLocked(start)), true
}
