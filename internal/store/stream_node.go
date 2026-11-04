package store

import (
	"encoding/binary"
	"strconv"
)

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

// A node also holds its entries, packed into buf as Redis packs them into
// the listpack: each entry is a flags byte, its ID's milliseconds less the
// master's and its sequence (uvarints), then its values when its fields are
// the master's, or else the field count and each name and value; a name or
// value is an uvarint length and the bytes. A deleted entry keeps its bytes
// with streamDeleted set, as Redis keeps it flagged in the listpack, until
// the whole node goes. Bytes once written are never written again except a
// flags byte, so a value handed out as a slice of buf stays valid.
type streamNode struct {
	master       StreamID
	masterFields []string
	// bytes models the size of Redis' listpack, which decides where a node
	// ends.
	bytes int
	total int
	live  int
	last  StreamID
	buf   []byte
}

const (
	streamDeleted    = 1
	streamSameFields = 2
	// streamNilFields marks an entry added with no field list at all,
	// which hands out nil fields rather than none.
	streamNilFields = 4
	// streamNodeOverhead is what a node costs besides its packed entries.
	streamNodeOverhead = 160
)

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

// full reports whether an entry of raw field bytes goes to a new node.
func (n *streamNode) full(raw int) bool {
	return n.bytes+raw >= streamNodeMaxBytes || n.total >= streamNodeMaxEntries
}

func newStreamNode(id StreamID, pairs []StreamField) *streamNode {
	names := make([]string, len(pairs))
	size := 7 + lpIntElem(1) + lpIntElem(0) + lpIntElem(int64(len(pairs))) + lpIntElem(0)
	for i, p := range pairs {
		names[i] = p.Name
		size += lpStrElem(p.Name)
	}
	return &streamNode{master: id, masterFields: names, bytes: size}
}

func rawFieldBytes(pairs []StreamField) int {
	raw := 0
	for _, p := range pairs {
		raw += len(p.Name) + len(p.Value)
	}
	return raw
}

func appendStreamBytes(buf, b []byte) []byte {
	buf = binary.AppendUvarint(buf, uint64(len(b)))
	return append(buf, b...)
}

// add packs an entry at the end of the node and returns the bytes it took.
func (n *streamNode) add(id StreamID, pairs []StreamField) int {
	same := sameStreamFields(n.masterFields, pairs)
	flags := int64(0)
	if same {
		flags = streamSameFields
	}
	size := lpIntElem(flags) + lpIntElem(int64(id.Ms-n.master.Ms)) + lpIntElem(int64(id.Seq-n.master.Seq))
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
	n.bytes += size
	n.total++
	n.live++
	n.last = id

	before := len(n.buf)
	if pairs == nil {
		flags |= streamNilFields
	}
	n.buf = append(n.buf, byte(flags))
	n.buf = binary.AppendUvarint(n.buf, id.Ms-n.master.Ms)
	n.buf = binary.AppendUvarint(n.buf, id.Seq)
	if !same {
		n.buf = binary.AppendUvarint(n.buf, uint64(len(pairs)))
	}
	for _, p := range pairs {
		if !same {
			n.buf = binary.AppendUvarint(n.buf, uint64(len(p.Name)))
			n.buf = append(n.buf, p.Name...)
		}
		n.buf = appendStreamBytes(n.buf, p.Value)
	}
	return len(n.buf) - before
}

// seal drops the slack of a node no more entries go to.
func (n *streamNode) seal() {
	if cap(n.buf) > len(n.buf) {
		n.buf = append([]byte(nil), n.buf...)
	}
}

// head decodes the flags and ID of the entry at off and returns the offset
// of its fields.
func (n *streamNode) head(off int) (flags byte, id StreamID, fields int) {
	flags = n.buf[off]
	off++
	ms, k := binary.Uvarint(n.buf[off:])
	off += k
	seq, k := binary.Uvarint(n.buf[off:])
	off += k
	return flags, StreamID{Ms: n.master.Ms + ms, Seq: seq}, off
}

func (n *streamNode) bytesAt(off int) ([]byte, int) {
	l, k := binary.Uvarint(n.buf[off:])
	start := off + k
	end := start + int(l)
	return n.buf[start:end:end], end
}

// fields decodes the fields of an entry from off, the offset head returned,
// and returns the offset of the next entry. With skip it only finds the end.
func (n *streamNode) fields(flags byte, off int, skip bool) ([]StreamField, int) {
	var pairs []StreamField
	if flags&streamSameFields != 0 {
		if !skip && flags&streamNilFields == 0 {
			pairs = make([]StreamField, len(n.masterFields))
		}
		for i := range n.masterFields {
			var v []byte
			v, off = n.bytesAt(off)
			if !skip {
				pairs[i] = StreamField{Name: n.masterFields[i], Value: v}
			}
		}
		return pairs, off
	}
	count, k := binary.Uvarint(n.buf[off:])
	off += k
	if !skip && flags&streamNilFields == 0 {
		pairs = make([]StreamField, count)
	}
	for i := 0; i < int(count); i++ {
		var name, v []byte
		name, off = n.bytesAt(off)
		v, off = n.bytesAt(off)
		if !skip {
			pairs[i] = StreamField{Name: string(name), Value: v}
		}
	}
	return pairs, off
}

// streamPos is an entry's place: its node and offset in it.
type streamPos struct {
	node int
	off  int
}

// each calls fn on the node's entries from offset from (deleted ones
// included) until fn returns false, and reports whether it ran out.
func (n *streamNode) each(from int, fn func(off int, flags byte, id StreamID, fields int) bool) bool {
	for off := from; off < len(n.buf); {
		flags, id, f := n.head(off)
		if !fn(off, flags, id, f) {
			return false
		}
		_, off = n.fields(flags, f, true)
	}
	return true
}

// entry builds the entry at off.
func (n *streamNode) entry(off int) *StreamEntry {
	flags, id, f := n.head(off)
	pairs, _ := n.fields(flags, f, false)
	return &StreamEntry{ID: id.String(), Pairs: pairs}
}
