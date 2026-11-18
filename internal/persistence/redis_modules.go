package persistence

import (
	"encoding/binary"
	"math"

	"github.com/cachestorm/cachestorm/internal/store"
)

// The module types of Redis 8's bundled modules CacheStorm saves and loads
// its values as, at the encoding versions Redis 8.2 writes: RedisJSON
// documents, and RedisBloom's Bloom filters, cuckoo filters and count-min
// sketches. CacheStorm's filters use RedisBloom's hashing and sizing, so a
// filter keeps answering the same once moved either way.
var (
	rdbModuleJSON   = rdbModuleID("ReJSON-RL", 3)
	rdbModuleBloom  = rdbModuleID("MBbloom--", 4)
	rdbModuleCuckoo = rdbModuleID("MBbloomCF", 4)
	rdbModuleCMS    = rdbModuleID("CMSk-TYPE", 0)
)

// RedisBloom's Bloom filter options: bits are not rounded to a power of
// two, hashes are 64-bit, and the filter does not scale.
const (
	bloomOptNoRound    = 1
	bloomOptEntsIsBits = 2
	bloomOptForce64    = 4
	bloomOptNoScaling  = 8
)

// moduleWriter appends module data items: each an opcode and its value.
type moduleWriter struct{ out []byte }

func (w *moduleWriter) uint(n uint64) {
	w.out = appendDumpLength(appendDumpLength(w.out, rdbModuleOpcodeUInt), n)
}

func (w *moduleWriter) double(f float64) {
	w.out = appendDumpLength(w.out, rdbModuleOpcodeDouble)
	w.out = binary.LittleEndian.AppendUint64(w.out, math.Float64bits(f))
}

func (w *moduleWriter) buffer(b []byte) {
	w.out = appendDumpLength(appendDumpLength(w.out, rdbModuleOpcodeStr), uint64(len(b)))
	w.out = append(w.out, b...)
}

// appendRedisModuleValue appends value as Redis 8 saves a value of its
// module type: the RDB type, the module id, the data and its EOF opcode. It
// reports false for a value no Redis module type holds.
func appendRedisModuleValue(out []byte, value store.Value) ([]byte, bool) {
	w := &moduleWriter{out: append(out, rdbTypeModule2)}
	switch v := value.(type) {
	case *store.JSONValue:
		w.out = appendDumpLength(w.out, rdbModuleJSON)
		w.buffer(v.Bytes())
	case *store.BloomValue:
		b := v.Clone().(*store.BloomValue)
		w.out = appendDumpLength(w.out, rdbModuleBloom)
		var items uint64
		for _, l := range b.Filters {
			items += l.Count
		}
		options := uint64(bloomOptNoRound | bloomOptForce64)
		growth := uint64(b.Expansion)
		if b.Expansion == 0 {
			options |= bloomOptNoScaling
			growth = 2
		}
		w.uint(items)
		w.uint(uint64(len(b.Filters)))
		w.uint(options)
		w.uint(growth)
		for i, l := range b.Filters {
			rate := store.BloomLayerErrorRate(b.ErrorRate, i, b.Expansion > 0)
			w.uint(l.Capacity)
			w.double(rate)
			w.uint(uint64(l.Hashes))
			w.double(store.BloomBitsPerEntry(rate))
			w.uint(l.NBits)
			w.uint(0)
			bits := make([]byte, 0, len(l.Bits)*8)
			for _, word := range l.Bits {
				bits = binary.LittleEndian.AppendUint64(bits, word)
			}
			w.buffer(bits)
			w.uint(l.Count)
		}
	case *store.CuckooValue:
		c := v.Clone().(*store.CuckooValue)
		w.out = appendDumpLength(w.out, rdbModuleCuckoo)
		w.uint(uint64(len(c.Filters)))
		w.uint(c.Filters[0].NumBuckets)
		w.uint(c.Inserted)
		w.uint(c.Deleted)
		w.uint(uint64(c.BucketSize))
		w.uint(uint64(c.MaxIterations))
		w.uint(uint64(c.Expansion))
		for _, l := range c.Filters {
			w.uint(l.NumBuckets)
			w.buffer(l.Slots)
		}
	case *store.CMSValue:
		m := v.Clone().(*store.CMSValue)
		w.out = appendDumpLength(w.out, rdbModuleCMS)
		w.uint(uint64(m.Width))
		w.uint(uint64(m.Depth))
		w.uint(m.Count)
		counters := make([]byte, 0, len(m.Counters)*4)
		for _, c := range m.Counters {
			counters = binary.LittleEndian.AppendUint32(counters, c)
		}
		w.buffer(counters)
	default:
		return out, false
	}
	return appendDumpLength(w.out, rdbModuleOpcodeEOF), true
}

// moduleUint reads an unsigned module item.
func (d *dumpDecoder) moduleUint() uint64 {
	if op, _ := d.length(); op != rdbModuleOpcodeUInt {
		d.err = ErrDumpFormat
		return 0
	}
	n, encoded := d.length()
	if encoded {
		d.err = ErrDumpFormat
	}
	return n
}

func (d *dumpDecoder) moduleDouble() float64 {
	if op, _ := d.length(); op != rdbModuleOpcodeDouble {
		d.err = ErrDumpFormat
		return 0
	}
	b := d.take(8)
	if len(b) != 8 {
		return 0
	}
	return math.Float64frombits(binary.LittleEndian.Uint64(b))
}

func (d *dumpDecoder) moduleBuffer() []byte {
	if op, _ := d.length(); op != rdbModuleOpcodeStr {
		d.err = ErrDumpFormat
		return nil
	}
	return d.view()
}

// redisBloomValue reads a RedisBloom value of module id id; handled is false
// for another module's. A filter RedisBloom made with options CacheStorm's
// filters do not have (32-bit hashes, bits rounded to a power of two or
// given as the capacity) is a format error.
func (d *dumpDecoder) redisBloomValue(id uint64) (value store.Value, handled bool) {
	encver := id & 1023
	switch id >> 10 {
	case rdbModuleBloom >> 10:
		if encver < 2 || encver > 4 {
			return nil, false
		}
		return d.bloom(encver), true
	case rdbModuleCuckoo >> 10:
		if encver != 4 {
			return nil, false
		}
		return d.cuckoo(), true
	case rdbModuleCMS >> 10:
		if encver != 0 {
			return nil, false
		}
		return d.cms(), true
	}
	return nil, false
}

func (d *dumpDecoder) bloom(encver uint64) store.Value {
	d.moduleUint()
	n := d.moduleUint()
	options := d.moduleUint()
	growth := uint64(2)
	if encver >= 4 {
		growth = d.moduleUint()
	}
	if d.err != nil || n == 0 || n > uint64(len(d.data)-d.pos) || growth > math.MaxUint32 ||
		options&(bloomOptNoRound|bloomOptForce64|bloomOptEntsIsBits) != bloomOptNoRound|bloomOptForce64 {
		d.err = ErrDumpFormat
		return nil
	}
	b := &store.BloomValue{Expansion: uint32(growth)}
	scaling := options&bloomOptNoScaling == 0
	if !scaling {
		b.Expansion = 0
	}
	for i := uint64(0); i < n && d.err == nil; i++ {
		entries := d.moduleUint()
		rate := d.moduleDouble()
		hashes := d.moduleUint()
		d.moduleDouble()
		nbits := d.moduleUint()
		d.moduleUint()
		bits := d.moduleBuffer()
		count := d.moduleUint()
		if d.err != nil || entries == 0 || hashes == 0 || hashes > math.MaxUint32 || nbits == 0 || nbits%64 != 0 ||
			uint64(len(bits)) != nbits/8 || !(rate > 0 && rate < 1) {
			d.err = ErrDumpFormat
			return nil
		}
		l := &store.BloomLayer{NBits: nbits, Hashes: uint32(hashes), Capacity: entries, Count: count, Bits: make([]uint64, nbits/64)}
		for j := range l.Bits {
			l.Bits[j] = binary.LittleEndian.Uint64(bits[j*8:])
		}
		if i == 0 {
			b.ErrorRate = rate
			if scaling {
				b.ErrorRate = rate / store.BloomTightening
			}
		}
		b.Filters = append(b.Filters, l)
	}
	if d.err != nil {
		return nil
	}
	return b
}

func (d *dumpDecoder) cuckoo() store.Value {
	n := d.moduleUint()
	d.moduleUint()
	c := &store.CuckooValue{Inserted: d.moduleUint(), Deleted: d.moduleUint()}
	bucketSize, maxIterations, expansion := d.moduleUint(), d.moduleUint(), d.moduleUint()
	if d.err != nil || n == 0 || n > uint64(len(d.data)-d.pos) || bucketSize == 0 || bucketSize > math.MaxUint8 ||
		maxIterations == 0 || maxIterations > math.MaxUint16 || expansion > math.MaxUint16 {
		d.err = ErrDumpFormat
		return nil
	}
	c.BucketSize, c.MaxIterations, c.Expansion = uint8(bucketSize), uint16(maxIterations), uint16(expansion)
	for i := uint64(0); i < n && d.err == nil; i++ {
		buckets := d.moduleUint()
		slots := d.moduleBuffer()
		if d.err != nil || buckets == 0 || buckets&(buckets-1) != 0 || buckets > uint64(len(slots)) || uint64(len(slots)) != buckets*bucketSize {
			d.err = ErrDumpFormat
			return nil
		}
		c.Filters = append(c.Filters, &store.CuckooLayer{NumBuckets: buckets, Slots: append([]byte(nil), slots...)})
	}
	if d.err != nil {
		return nil
	}
	return c
}

func (d *dumpDecoder) cms() store.Value {
	width, depth, count := d.moduleUint(), d.moduleUint(), d.moduleUint()
	counters := d.moduleBuffer()
	if d.err != nil || width == 0 || depth == 0 || width > uint64(len(counters)) || depth > uint64(len(counters)) ||
		width > math.MaxUint32 || depth > math.MaxUint32 || uint64(len(counters)) != width*depth*4 {
		d.err = ErrDumpFormat
		return nil
	}
	m := &store.CMSValue{Width: uint32(width), Depth: uint32(depth), Count: count, Counters: make([]uint32, width*depth)}
	for i := range m.Counters {
		m.Counters[i] = binary.LittleEndian.Uint32(counters[i*4:])
	}
	return m
}
