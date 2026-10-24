package store

import "strconv"

// Redis stores a stream as a radix tree of listpack nodes. Approximate ("~")
// trimming only ever drops whole nodes, so its result depends on where the
// node boundaries fall. streamNode models one listpack closely enough to put
// the boundaries where Redis puts them with the default
// stream-node-max-entries and stream-node-max-bytes.
const (
	streamNodeMaxEntries = 100
	streamNodeMaxBytes   = 4096
	streamDefaultLimit   = 100 * streamNodeMaxEntries
)

type streamNode struct {
	master       StreamID
	masterFields []string
	bytes        int
	total        int
	live         int
	last         StreamID
}

func lpIntSize(v int64) int {
	switch {
	case v >= 0 && v <= 127:
		return 1
	case v >= -4096 && v <= 4095:
		return 2
	case v >= -32768 && v <= 32767:
		return 3
	case v >= -8388608 && v <= 8388607:
		return 4
	case v >= -2147483648 && v <= 2147483647:
		return 5
	}
	return 9
}

func lpBacklenSize(l int) int {
	switch {
	case l <= 127:
		return 1
	case l < 16383:
		return 2
	case l < 2097151:
		return 3
	case l < 268435455:
		return 4
	}
	return 5
}

func lpElem(enc int) int { return enc + lpBacklenSize(enc) }

func lpIntElem(v int64) int { return lpElem(lpIntSize(v)) }

func lpStrElem(s string) int {
	if len(s) > 0 && len(s) <= 20 {
		if v, err := strconv.ParseInt(s, 10, 64); err == nil && strconv.FormatInt(v, 10) == s {
			return lpIntElem(v)
		}
	}
	n := len(s)
	switch {
	case n <= 63:
		return lpElem(1 + n)
	case n <= 4095:
		return lpElem(2 + n)
	}
	return lpElem(5 + n)
}

func lpBytesElem(b []byte) int { return lpStrElem(string(b)) }

func sameStreamFields(master []string, pairs []StreamField) bool {
	if len(master) != len(pairs) {
		return false
	}
	for i, p := range pairs {
		if master[i] != p.Name {
			return false
		}
	}
	return true
}

// placeEntry appends e to tail, or to a fresh node when tail is full, and
// returns the node now holding e. The caller holds the write lock.
func (v *StreamValue) placeEntry(tail *streamNode, e *StreamEntry) *streamNode {
	pairs := e.FieldPairs()
	id := e.streamID()
	raw := 0
	for _, p := range pairs {
		raw += len(p.Name) + len(p.Value)
	}
	if tail == nil || tail.bytes+raw >= streamNodeMaxBytes || tail.total >= streamNodeMaxEntries {
		names := make([]string, len(pairs))
		size := 7 + lpIntElem(1) + lpIntElem(0) + lpIntElem(int64(len(pairs))) + lpIntElem(0)
		for i, p := range pairs {
			names[i] = p.Name
			size += lpStrElem(p.Name)
		}
		tail = &streamNode{master: id, masterFields: names, bytes: size}
		v.nodeCount++
	}
	same := sameStreamFields(tail.masterFields, pairs)
	flags := int64(0)
	if same {
		flags = 2
	}
	size := lpIntElem(flags) + lpIntElem(int64(id.Ms-tail.master.Ms)) + lpIntElem(int64(id.Seq-tail.master.Seq))
	lpCount := int64(len(pairs) + 3)
	if same {
		for _, p := range pairs {
			size += lpBytesElem(p.Value)
		}
	} else {
		size += lpIntElem(int64(len(pairs)))
		for _, p := range pairs {
			size += lpStrElem(p.Name) + lpBytesElem(p.Value)
		}
		lpCount += int64(len(pairs) + 1)
	}
	size += lpIntElem(lpCount)
	tail.bytes += size
	tail.total++
	tail.live++
	tail.last = id
	e.node = tail
	v.nodeLive++
	return tail
}

func (v *StreamValue) nodesValid() bool {
	n := len(v.Entries)
	if n == 0 {
		return v.nodeLive == 0
	}
	return v.nodeLive == n && v.Entries[0].node != nil && v.Entries[n-1].node != nil
}

// ensureNodes rebuilds the node model when the entries were set up without
// it (RDB load, tests, Clone). The caller holds the write lock.
func (v *StreamValue) ensureNodes() {
	if v.nodesValid() {
		if len(v.Entries) == 0 {
			v.nodeCount = 0
		}
		return
	}
	v.nodeCount, v.nodeLive = 0, 0
	var tail *streamNode
	for _, e := range v.Entries {
		e.node = nil
		tail = v.placeEntry(tail, e)
	}
	v.Length = int64(len(v.Entries))
	if v.EntriesAdded < v.Length {
		v.EntriesAdded = v.Length
	}
}

// countNodes reports the node count without mutating state, for readers.
func (v *StreamValue) countNodes() int {
	if v.nodesValid() {
		if len(v.Entries) == 0 {
			return 0
		}
		return v.nodeCount
	}
	shadow := &StreamValue{}
	var tail *streamNode
	for _, e := range v.Entries {
		copyEntry := &StreamEntry{ID: e.ID, Fields: e.Fields, Pairs: e.Pairs}
		tail = shadow.placeEntry(tail, copyEntry)
	}
	return shadow.nodeCount
}

// dropEntryNode accounts for removing e from its node. The caller holds the
// write lock.
func (v *StreamValue) dropEntryNode(e *StreamEntry) {
	if e.node == nil {
		return
	}
	e.node.live--
	if e.node.live == 0 {
		v.nodeCount--
	}
	v.nodeLive--
	e.node = nil
}
