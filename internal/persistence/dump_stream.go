package persistence

import (
	"encoding/binary"
	"math"
	"sort"
	"strconv"

	"github.com/cachestorm/cachestorm/internal/store"
)

// Redis' RDB stream types: RDB_TYPE_STREAM_LISTPACKS, _2 (first ID, deleted
// ID, entries added and group read counters) and _3 (consumer active time).
// DUMP writes _3, as Redis 7.2 and later do.
const (
	rdbTypeStreamListpacks  = 15
	rdbTypeStreamListpacks2 = 19
	rdbTypeStreamListpacks3 = 21
)

// streamNodeMaxEntries is Redis' default stream-node-max-entries: a dumped
// stream has one listpack per this many entries.
const streamNodeMaxEntries = 100

const (
	streamItemDeleted    = 1
	streamItemSameFields = 2
)

// listpack builds a Redis listpack.
type listpack struct {
	buf   []byte
	count int
}

func newListpack() *listpack { return &listpack{buf: make([]byte, 6, 256)} }

func (lp *listpack) element(enc []byte) {
	lp.buf = append(lp.buf, enc...)
	l := len(enc)
	switch {
	case l <= 127:
		lp.buf = append(lp.buf, byte(l))
	case l < 16383:
		lp.buf = append(lp.buf, byte(l>>7), byte(l&127)|128)
	case l < 2097151:
		lp.buf = append(lp.buf, byte(l>>14), byte((l>>7)&127)|128, byte(l&127)|128)
	case l < 268435455:
		lp.buf = append(lp.buf, byte(l>>21), byte((l>>14)&127)|128, byte((l>>7)&127)|128, byte(l&127)|128)
	default:
		lp.buf = append(lp.buf, byte(l>>28), byte((l>>21)&127)|128, byte((l>>14)&127)|128, byte((l>>7)&127)|128, byte(l&127)|128)
	}
	lp.count++
}

func (lp *listpack) str(s string) {
	var enc []byte
	switch n := len(s); {
	case n < 64:
		enc = append(make([]byte, 0, n+1), 0x80|byte(n))
	case n < 4096:
		enc = append(make([]byte, 0, n+2), 0xE0|byte(n>>8), byte(n))
	default:
		enc = binary.LittleEndian.AppendUint32(append(make([]byte, 0, n+5), 0xF0), uint32(n))
	}
	lp.element(append(enc, s...))
}

func (lp *listpack) int(v int64) {
	switch {
	case v >= 0 && v <= 127:
		lp.element([]byte{byte(v)})
	case v >= -4096 && v <= 4095:
		u := uint16(v) & 0x1FFF
		lp.element([]byte{0xC0 | byte(u>>8), byte(u)})
	case v >= math.MinInt16 && v <= math.MaxInt16:
		lp.element(binary.LittleEndian.AppendUint16([]byte{0xF1}, uint16(v)))
	case v >= -1<<23 && v < 1<<23:
		u := uint32(v)
		lp.element([]byte{0xF2, byte(u), byte(u >> 8), byte(u >> 16)})
	case v >= math.MinInt32 && v <= math.MaxInt32:
		lp.element(binary.LittleEndian.AppendUint32([]byte{0xF3}, uint32(v)))
	default:
		lp.element(binary.LittleEndian.AppendUint64([]byte{0xF4}, uint64(v)))
	}
}

func (lp *listpack) bytes() []byte {
	lp.buf = append(lp.buf, 0xFF)
	binary.LittleEndian.PutUint32(lp.buf, uint32(len(lp.buf)))
	binary.LittleEndian.PutUint16(lp.buf[4:], uint16(min(lp.count, 65535)))
	return lp.buf
}

func rawStreamID(id store.StreamID) []byte {
	return binary.BigEndian.AppendUint64(binary.BigEndian.AppendUint64(nil, id.Ms), id.Seq)
}

func parseDumpStreamID(s string) store.StreamID {
	id, err := store.ParseStreamID(s, 0)
	if err != nil {
		return store.StreamID{}
	}
	return id
}

// appendDumpStream writes v as Redis' RDB_TYPE_STREAM_LISTPACKS_3: listpack
// nodes of up to streamNodeMaxEntries entries, the stream's IDs and
// counters, then each consumer group with its pending entries and
// consumers, groups and consumers in name order as Redis' radix trees keep
// them.
func appendDumpStream(out []byte, v *store.StreamValue) []byte {
	v.RLock()
	defer v.RUnlock()
	str := func(s string) { out = appendDumpLength(out, uint64(len(s))); out = append(out, s...) }
	ms := func(t int64) { out = binary.LittleEndian.AppendUint64(out, uint64(t)) }
	id := func(s store.StreamID) { out = appendDumpLength(appendDumpLength(out, s.Ms), s.Seq) }

	out = append(out, rdbTypeStreamListpacks3)
	entries := v.Entries
	out = appendDumpLength(out, uint64((len(entries)+streamNodeMaxEntries-1)/streamNodeMaxEntries))
	for start := 0; start < len(entries); start += streamNodeMaxEntries {
		node := entries[start:min(start+streamNodeMaxEntries, len(entries))]
		master := parseDumpStreamID(node[0].ID)
		masterFields := node[0].FieldPairs()
		lp := newListpack()
		lp.int(int64(len(node)))
		lp.int(0)
		lp.int(int64(len(masterFields)))
		for _, f := range masterFields {
			lp.str(f.Name)
		}
		lp.int(0)
		for _, e := range node {
			eid := parseDumpStreamID(e.ID)
			pairs := e.FieldPairs()
			same := len(pairs) == len(masterFields)
			for i := 0; same && i < len(pairs); i++ {
				same = pairs[i].Name == masterFields[i].Name
			}
			flags := int64(0)
			if same {
				flags = streamItemSameFields
			}
			lp.int(flags)
			lp.int(int64(eid.Ms - master.Ms))
			lp.int(int64(eid.Seq - master.Seq))
			if !same {
				lp.int(int64(len(pairs)))
			}
			for _, f := range pairs {
				if !same {
					lp.str(f.Name)
				}
				lp.str(string(f.Value))
			}
			count := int64(len(pairs)) + 3
			if !same {
				count += int64(len(pairs)) + 1
			}
			lp.int(count)
		}
		key := rawStreamID(master)
		str(string(key))
		str(string(lp.bytes()))
	}
	out = appendDumpLength(out, uint64(v.Length))
	id(parseDumpStreamID(v.LastID))
	first := store.StreamID{}
	if len(entries) > 0 {
		first = parseDumpStreamID(entries[0].ID)
	}
	id(first)
	id(parseDumpStreamID(v.MaxDeletedID))
	out = appendDumpLength(out, uint64(v.EntriesAdded))

	names := make([]string, 0, len(v.Groups))
	for name := range v.Groups {
		names = append(names, name)
	}
	sort.Strings(names)
	out = appendDumpLength(out, uint64(len(names)))
	for _, name := range names {
		g := v.Groups[name]
		g.RLock()
		str(name)
		id(parseDumpStreamID(g.LastID))
		out = appendDumpLength(out, uint64(g.EntriesRead))
		pel := make([]*store.PendingEntry, 0, len(g.Pending))
		for _, p := range g.Pending {
			pel = append(pel, p)
		}
		sort.Slice(pel, func(i, j int) bool {
			return parseDumpStreamID(pel[i].ID).Compare(parseDumpStreamID(pel[j].ID)) < 0
		})
		out = appendDumpLength(out, uint64(len(pel)))
		for _, p := range pel {
			out = append(out, rawStreamID(parseDumpStreamID(p.ID))...)
			ms(p.DeliveryTS)
			out = appendDumpLength(out, uint64(p.Deliveries))
		}
		consumers := make([]string, 0, len(g.Consumers))
		for c := range g.Consumers {
			consumers = append(consumers, c)
		}
		sort.Strings(consumers)
		out = appendDumpLength(out, uint64(len(consumers)))
		for _, cname := range consumers {
			c := g.Consumers[cname]
			str(cname)
			ms(c.SeenTime)
			ms(c.ActiveTime)
			owned := 0
			for _, p := range pel {
				if p.Consumer == cname {
					owned++
				}
			}
			out = appendDumpLength(out, uint64(owned))
			for _, p := range pel {
				if p.Consumer == cname {
					out = append(out, rawStreamID(parseDumpStreamID(p.ID))...)
				}
			}
		}
		g.RUnlock()
	}
	return out
}

func (d *dumpDecoder) streamID() store.StreamID {
	ms, enc := d.length()
	seq, enc2 := d.length()
	if enc || enc2 {
		d.err = ErrDumpFormat
	}
	return store.StreamID{Ms: ms, Seq: seq}
}

func (d *dumpDecoder) msTime() int64 {
	b := d.take(8)
	if len(b) != 8 {
		return 0
	}
	return int64(binary.LittleEndian.Uint64(b))
}

func (d *dumpDecoder) rawID() store.StreamID {
	b := d.take(16)
	if len(b) != 16 {
		return store.StreamID{}
	}
	return store.StreamID{Ms: binary.BigEndian.Uint64(b), Seq: binary.BigEndian.Uint64(b[8:])}
}

// stream reads Redis' stream encodings, as rdbLoadObject does: the listpack
// nodes, the stream's IDs and counters (estimated for the first version),
// and the consumer groups, whose pending entries are given to the consumers
// that list them.
func (d *dumpDecoder) stream(kind byte) store.Value {
	v := store.NewStreamValue(0)
	nodes := d.count()
	for i := 0; i < nodes && d.err == nil; i++ {
		key := []byte(d.str())
		lp := []byte(d.str())
		if d.err != nil {
			return nil
		}
		if len(key) != 16 {
			d.err = ErrDumpFormat
			return nil
		}
		master := store.StreamID{Ms: binary.BigEndian.Uint64(key), Seq: binary.BigEndian.Uint64(key[8:])}
		items, err := decodeDumpListpack(lp)
		if err != nil || len(items) == 0 {
			d.err = ErrDumpFormat
			return nil
		}
		entries, ok := streamNodeEntries(master, items)
		if !ok {
			d.err = ErrDumpFormat
			return nil
		}
		v.Entries = append(v.Entries, entries...)
	}
	length, enc := d.length()
	last := d.streamID()
	if enc {
		d.err = ErrDumpFormat
	}
	v.LastID = last.String()
	if kind >= rdbTypeStreamListpacks2 {
		d.streamID()
		if deleted := d.streamID(); deleted != (store.StreamID{}) {
			v.MaxDeletedID = deleted.String()
		}
		added, enc := d.length()
		if enc {
			d.err = ErrDumpFormat
		}
		v.EntriesAdded = int64(added)
	} else {
		v.EntriesAdded = int64(length)
	}
	if d.err != nil || uint64(len(v.Entries)) != length {
		d.err = ErrDumpFormat
		return nil
	}
	v.Length = int64(length)
	for i := 1; i < len(v.Entries); i++ {
		if parseDumpStreamID(v.Entries[i-1].ID).Compare(parseDumpStreamID(v.Entries[i].ID)) >= 0 {
			d.err = ErrDumpFormat
			return nil
		}
	}

	groups := d.count()
	for i := 0; i < groups && d.err == nil; i++ {
		name := d.str()
		g := store.NewConsumerGroup(name)
		g.LastID = d.streamID().String()
		if kind >= rdbTypeStreamListpacks2 {
			read, enc := d.length()
			if enc {
				d.err = ErrDumpFormat
			}
			g.EntriesRead = int64(read)
		}
		pending := d.count()
		for j := 0; j < pending && d.err == nil; j++ {
			id := d.rawID().String()
			p := &store.PendingEntry{ID: id, DeliveryTS: d.msTime()}
			n, enc := d.length()
			if enc {
				d.err = ErrDumpFormat
			}
			p.Deliveries = int64(n)
			if _, dup := g.Pending[id]; dup {
				d.err = ErrDumpFormat
			}
			g.Pending[id] = p
		}
		consumers := d.count()
		for j := 0; j < consumers && d.err == nil; j++ {
			c := &store.Consumer{Name: d.str(), Active: true}
			c.SeenTime = d.msTime()
			c.ActiveTime = c.SeenTime
			if kind >= rdbTypeStreamListpacks3 {
				c.ActiveTime = d.msTime()
			}
			owned := d.count()
			for k := 0; k < owned && d.err == nil; k++ {
				p, ok := g.Pending[d.rawID().String()]
				if !ok || p.Consumer != "" {
					d.err = ErrDumpFormat
					break
				}
				p.Consumer = c.Name
				c.Pending++
			}
			if _, dup := g.Consumers[c.Name]; dup {
				d.err = ErrDumpFormat
			}
			g.Consumers[c.Name] = c
		}
		for _, p := range g.Pending {
			if p.Consumer == "" {
				d.err = ErrDumpFormat
			}
		}
		if _, dup := v.Groups[name]; dup {
			d.err = ErrDumpFormat
		}
		v.Groups[name] = g
	}
	if d.err != nil {
		return nil
	}
	return v
}

// streamNodeEntries decodes one stream listpack: the master entry (count,
// deleted, master fields, terminator) and then each entry relative to the
// node's master ID, skipping the ones flagged deleted.
func streamNodeEntries(master store.StreamID, items []string) ([]*store.StreamEntry, bool) {
	pos := 0
	next := func() (string, bool) {
		if pos >= len(items) {
			return "", false
		}
		pos++
		return items[pos-1], true
	}
	num := func() (int64, bool) {
		s, ok := next()
		if !ok {
			return 0, false
		}
		n, err := strconv.ParseInt(s, 10, 64)
		return n, err == nil
	}
	count, ok1 := num()
	deleted, ok2 := num()
	nmaster, ok3 := num()
	if !ok1 || !ok2 || !ok3 || count < 0 || deleted < 0 || nmaster < 0 || nmaster > int64(len(items)) {
		return nil, false
	}
	masterFields := make([]string, nmaster)
	for i := range masterFields {
		f, ok := next()
		if !ok {
			return nil, false
		}
		masterFields[i] = f
	}
	if z, ok := num(); !ok || z != 0 {
		return nil, false
	}
	var out []*store.StreamEntry
	seen := int64(0)
	for pos < len(items) {
		flags, ok1 := num()
		msDiff, ok2 := num()
		seqDiff, ok3 := num()
		if !ok1 || !ok2 || !ok3 {
			return nil, false
		}
		var pairs []store.StreamField
		if flags&streamItemSameFields != 0 {
			for _, f := range masterFields {
				val, ok := next()
				if !ok {
					return nil, false
				}
				pairs = append(pairs, store.StreamField{Name: f, Value: []byte(val)})
			}
		} else {
			n, ok := num()
			if !ok || n < 0 || n > int64(len(items)) {
				return nil, false
			}
			for i := int64(0); i < n; i++ {
				f, ok1 := next()
				val, ok2 := next()
				if !ok1 || !ok2 {
					return nil, false
				}
				pairs = append(pairs, store.StreamField{Name: f, Value: []byte(val)})
			}
		}
		if _, ok := num(); !ok {
			return nil, false
		}
		seen++
		if flags&streamItemDeleted != 0 {
			continue
		}
		id := store.StreamID{Ms: master.Ms + uint64(msDiff), Seq: master.Seq + uint64(seqDiff)}
		fields := make(map[string][]byte, len(pairs))
		for _, f := range pairs {
			fields[f.Name] = f.Value
		}
		out = append(out, &store.StreamEntry{ID: id.String(), Fields: fields, Pairs: pairs})
	}
	if seen != count+deleted || int64(len(out)) != count {
		return nil, false
	}
	return out, true
}
