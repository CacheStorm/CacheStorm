package store

// How a deletion treats the consumer groups' references to an entry, Redis
// 8.2's KEEPREF, DELREF and ACKED: StreamKeepRef leaves the entry in the
// groups' pending lists, StreamDelRef removes it from them, and StreamAcked
// deletes only an entry every group has read and acknowledged.
const (
	StreamKeepRef = iota
	StreamDelRef
	StreamAcked
)

// The replies of XDELEX and XACKDEL for one ID.
const (
	StreamDelNoID       = -1
	StreamDelDeleted    = 1
	StreamDelReferenced = 2
)

// referencedLocked reports whether a group has not read id yet or holds it
// pending.
func (v *StreamValue) referencedLocked(id StreamID) bool {
	key := id.String()
	for _, g := range v.Groups {
		g.mu.RLock()
		_, pending := g.Pending[key]
		unread := streamIDOf(g.LastID).Less(id)
		g.mu.RUnlock()
		if pending || unread {
			return true
		}
	}
	return false
}

// dropRefsLocked removes id from every group's pending list and reports
// whether one held it.
func (v *StreamValue) dropRefsLocked(id StreamID) bool {
	key := id.String()
	dropped := false
	for _, g := range v.Groups {
		g.mu.Lock()
		if g.ackLocked(key) {
			dropped = true
		}
		g.mu.Unlock()
	}
	return dropped
}

// deleteIDLocked deletes the entry id, as XDEL does, and reports whether it
// existed.
func (v *StreamValue) deleteIDLocked(id StreamID) bool {
	pos, ok := v.seekLocked(id)
	if !ok {
		return false
	}
	if _, eid, _ := v.nodes[pos.node].head(pos.off); eid != id {
		return false
	}
	v.deleteAtLocked(pos)
	if v.maxDeletedLocked().Less(id) {
		v.MaxDeletedID = id.String()
	}
	return true
}

// DeleteWithRefs deletes ids as XDELEX does under the given strategy. It
// returns each ID's reply, how many entries it deleted, and whether it
// changed the stream or a group.
func (v *StreamValue) DeleteWithRefs(ids []StreamID, strategy int) (replies []int64, deleted int, changed bool) {
	v.mu.Lock()
	defer v.mu.Unlock()
	replies = make([]int64, len(ids))
	for i, id := range ids {
		switch {
		case strategy == StreamAcked && v.referencedLocked(id):
			replies[i] = StreamDelReferenced
			continue
		case strategy == StreamDelRef && v.dropRefsLocked(id):
			changed = true
		}
		replies[i] = StreamDelNoID
		if v.deleteIDLocked(id) {
			replies[i] = StreamDelDeleted
			deleted++
			changed = true
		}
	}
	return replies, deleted, changed
}

// AckDelete acknowledges ids in the group and deletes them, as XACKDEL does
// under the given strategy. An ID the group did not hold pending is
// NoID; one acknowledged counts as deleted even if the stream no longer
// had it. It returns each ID's reply, how many entries it deleted, whether
// it changed anything, and false when the group does not exist.
func (v *StreamValue) AckDelete(group string, ids []StreamID, strategy int) (replies []int64, deleted int, changed bool, ok bool) {
	v.mu.Lock()
	defer v.mu.Unlock()
	g, ok := v.Groups[group]
	if !ok {
		return nil, 0, false, false
	}
	replies = make([]int64, len(ids))
	for i, id := range ids {
		g.mu.Lock()
		acked := g.ackLocked(id.String())
		g.mu.Unlock()
		if !acked {
			replies[i] = StreamDelNoID
			continue
		}
		changed = true
		switch strategy {
		case StreamAcked:
			if v.referencedLocked(id) {
				replies[i] = StreamDelReferenced
				continue
			}
		case StreamDelRef:
			v.dropRefsLocked(id)
		}
		if v.deleteIDLocked(id) {
			deleted++
		}
		replies[i] = StreamDelDeleted
	}
	return replies, deleted, changed, true
}

// trimRefsLocked trims as Redis 8.2 does under DELREF or ACKED: node by
// node from the head, entry by entry since each entry's references matter,
// skipping the entries ACKED keeps. It goes on to the next node only while
// the whole node was due for trimming; approximation only limits the work.
func (v *StreamValue) trimRefsLocked(args StreamTrimArgs) int64 {
	maxLen := args.Strategy == StreamTrimMaxLen
	deleted := int64(0)
	for i := 0; i < len(v.nodes); {
		if maxLen && int64(v.count) <= args.MaxLen {
			break
		}
		node := v.nodes[i]
		entries := int64(node.live)
		if args.Limit > 0 && deleted+entries > args.Limit {
			break
		}
		var whole bool
		if maxLen {
			whole = int64(v.count)-entries >= args.MaxLen
		} else {
			whole = node.last.Less(args.MinID)
		}
		remaining := int64(v.count)
		var offs []int
		node.each(0, func(off int, flags byte, id StreamID, _ int) bool {
			if maxLen && remaining <= args.MaxLen || !maxLen && !id.Less(args.MinID) {
				return false
			}
			if flags&streamDeleted != 0 {
				return true
			}
			if args.Refs == StreamAcked && v.referencedLocked(id) {
				return true
			}
			if args.Refs == StreamDelRef {
				v.dropRefsLocked(id)
			}
			offs = append(offs, off)
			remaining--
			return true
		})
		for _, off := range offs {
			v.deleteAtLocked(streamPos{node: i, off: off})
		}
		deleted += int64(len(offs))
		if !whole {
			break
		}
		if int64(len(offs)) < entries {
			i++
		}
	}
	return deleted
}
