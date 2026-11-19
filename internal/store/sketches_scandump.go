package store

import (
	"encoding/binary"
	"errors"
	"fmt"
	"math"
	"strconv"
)

// RedisBloom's BF.SCANDUMP / CF.SCANDUMP chunk formats, which BF.LOADCHUNK
// and CF.LOADCHUNK read back: a header that is a packed C struct of the
// filter's settings, then the filter bytes in chunks addressed by a running
// position, so filters move between servers chunk by chunk.

// ScanDumpChunkMax is the largest chunk SCANDUMP returns.
const ScanDumpChunkMax = 16 << 20

// scanDumpMaxFilterBytes bounds what a SCANDUMP header may allocate.
const scanDumpMaxFilterBytes = 512 << 20

const (
	bloomDumpHeader = 20 // size u64, nfilters u32, options u32, growth u32
	bloomDumpLink   = 53 // bytes, bits, size u64; error, bpe f64; hashes u32; entries u64; n2 u8
	cuckooDumpHdr   = 38 // items, buckets, deletes, filters u64; bucket size, iterations, expansion u16
)

//nolint:staticcheck // RedisBloom's reply texts
var (
	ErrBloomBadData = errors.New("ERR received bad data")
	ErrBloomNoLink  = errors.New("ERR invalid offset - no link found")
	ErrBloomTooBig  = errors.New("ERR invalid chunk - Too big for current filter")
)

const (
	bloomOptNoRound    = 1
	bloomOptEntsIsBits = 2
	bloomOptForce64    = 4
	bloomOptNoScaling  = 8
)

// ScanDumpHeader is BF.SCANDUMP's first chunk, SBChain_GetEncodedHeader.
func (b *BloomValue) ScanDumpHeader() []byte {
	b.mu.RLock()
	defer b.mu.RUnlock()
	var items uint64
	for _, l := range b.Filters {
		items += l.Count
	}
	options := uint32(bloomOptNoRound | bloomOptForce64)
	growth := b.Expansion
	if b.Expansion == 0 {
		options |= bloomOptNoScaling
		growth = 2
	}
	out := binary.LittleEndian.AppendUint64(nil, items)
	out = binary.LittleEndian.AppendUint32(out, uint32(len(b.Filters)))
	out = binary.LittleEndian.AppendUint32(out, options)
	out = binary.LittleEndian.AppendUint32(out, growth)
	for i, l := range b.Filters {
		rate := BloomLayerErrorRate(b.ErrorRate, i, b.Expansion > 0)
		out = binary.LittleEndian.AppendUint64(out, l.NBits/8)
		out = binary.LittleEndian.AppendUint64(out, l.NBits)
		out = binary.LittleEndian.AppendUint64(out, l.Count)
		out = binary.LittleEndian.AppendUint64(out, math.Float64bits(rate))
		out = binary.LittleEndian.AppendUint64(out, math.Float64bits(BloomBitsPerEntry(rate)))
		out = binary.LittleEndian.AppendUint32(out, l.Hashes)
		out = binary.LittleEndian.AppendUint64(out, l.Capacity)
		out = append(out, 0)
	}
	return out
}

// bloomLinkAt finds the filter holding byte pos of the concatenated
// filters and the offset in it, as RedisBloom's getLinkPos does.
func (b *BloomValue) bloomLinkAt(pos int64) (*BloomLayer, uint64, bool) {
	if pos < 0 {
		return nil, 0, false
	}
	off := uint64(pos)
	for _, l := range b.Filters {
		if off < l.NBits/8 {
			return l, off, true
		}
		off -= l.NBits / 8
	}
	return nil, 0, false
}

// ScanDumpChunk is SBChain_GetEncodedChunk: the bytes from position iter
// (1-based) to the end of its filter, at most max; next is the position
// after them, 0 when iter is past the end.
func (b *BloomValue) ScanDumpChunk(iter int64, max int) (next int64, chunk []byte) {
	b.mu.RLock()
	defer b.mu.RUnlock()
	l, off, ok := b.bloomLinkAt(iter - 1)
	if iter < 1 || !ok {
		return 0, nil
	}
	n := l.NBits/8 - off
	if n > uint64(max) {
		n = uint64(max)
	}
	chunk = make([]byte, n)
	for i := range chunk {
		p := off + uint64(i)
		chunk[i] = byte(l.Bits[p/8] >> (8 * (p % 8)))
	}
	return iter + int64(n), chunk
}

// LoadChunk is SBChain_LoadEncodedChunk: iter is the position after buf.
func (b *BloomValue) LoadChunk(iter int64, buf []byte) error {
	if len(buf) == 0 || iter <= 0 || uint64(iter) < uint64(len(buf)) {
		return ErrBloomBadData
	}
	b.mu.Lock()
	defer b.mu.Unlock()
	l, off, ok := b.bloomLinkAt(iter - int64(len(buf)) - 1)
	if !ok {
		return ErrBloomNoLink
	}
	if uint64(len(buf)) > l.NBits/8-off {
		return ErrBloomTooBig
	}
	for i, c := range buf {
		p := off + uint64(i)
		shift := 8 * (p % 8)
		l.Bits[p/8] = l.Bits[p/8]&^(0xff<<shift) | uint64(c)<<shift
	}
	return nil
}

// BloomFromScanDumpHeader is SB_NewChainFromHeader: an empty filter with
// the header's settings, whose bits the following chunks fill. Settings
// CacheStorm's filters do not have (32-bit hashes, rounded or entry-sized
// bit arrays) are bad data.
func BloomFromScanDumpHeader(buf []byte) (*BloomValue, error) {
	if len(buf) < bloomDumpHeader {
		return nil, ErrBloomBadData
	}
	le := binary.LittleEndian
	size, n, options, growth := le.Uint64(buf), le.Uint32(buf[8:]), le.Uint32(buf[12:]), le.Uint32(buf[16:])
	if n == 0 || uint64(len(buf)) != bloomDumpHeader+uint64(n)*bloomDumpLink ||
		options&^(bloomOptNoRound|bloomOptEntsIsBits|bloomOptForce64|bloomOptNoScaling) != 0 ||
		options&(bloomOptNoRound|bloomOptForce64|bloomOptEntsIsBits) != bloomOptNoRound|bloomOptForce64 {
		return nil, ErrBloomBadData
	}
	b := &BloomValue{Expansion: growth}
	scaling := options&bloomOptNoScaling == 0
	if !scaling {
		b.Expansion = 0
	}
	var total, bytes uint64
	for i := uint32(0); i < n; i++ {
		p := buf[bloomDumpHeader+int(i)*bloomDumpLink:]
		nbytes, bits, count := le.Uint64(p), le.Uint64(p[8:]), le.Uint64(p[16:])
		rate, hashes, entries := math.Float64frombits(le.Uint64(p[24:])), le.Uint32(p[40:]), le.Uint64(p[44:])
		if bits == 0 || bits%64 != 0 || nbytes != bits/8 || hashes == 0 || entries == 0 || !(rate > 0 && rate < 1) ||
			p[52] != 0 || count > math.MaxUint64-total || nbytes > scanDumpMaxFilterBytes-bytes {
			return nil, ErrBloomBadData
		}
		total += count
		bytes += nbytes
		if i == 0 {
			b.ErrorRate = rate
			if scaling {
				b.ErrorRate = rate / BloomTightening
			}
		}
		b.Filters = append(b.Filters, &BloomLayer{NBits: bits, Hashes: hashes, Capacity: entries, Count: count, Bits: make([]uint64, bits/64)})
	}
	if total != size {
		return nil, ErrBloomBadData
	}
	return b, nil
}

// DebugLines is BF.DEBUG's reply.
func (b *BloomValue) DebugLines() []string {
	b.mu.RLock()
	defer b.mu.RUnlock()
	var items uint64
	for _, l := range b.Filters {
		items += l.Count
	}
	lines := []string{fmt.Sprintf("size:%d", items)}
	for i, l := range b.Filters {
		rate := BloomLayerErrorRate(b.ErrorRate, i, b.Expansion > 0)
		lines = append(lines, fmt.Sprintf("bytes:%d bits:%d hashes:%d hashwidth:64 capacity:%d size:%d ratio:%s",
			l.NBits/8, l.NBits, l.Hashes, l.Capacity, l.Count, formatCG(rate)))
	}
	return lines
}

// formatCG formats f as C's %g does: six significant digits.
func formatCG(f float64) string { return strconv.FormatFloat(f, 'g', 6, 64) }

// ScanDumpHeader is CF.SCANDUMP's first chunk, fillCFHeader's struct.
func (c *CuckooValue) ScanDumpHeader() []byte {
	c.mu.RLock()
	defer c.mu.RUnlock()
	out := binary.LittleEndian.AppendUint64(nil, c.Inserted)
	out = binary.LittleEndian.AppendUint64(out, c.Filters[0].NumBuckets)
	out = binary.LittleEndian.AppendUint64(out, c.Deleted)
	out = binary.LittleEndian.AppendUint64(out, uint64(len(c.Filters)))
	out = binary.LittleEndian.AppendUint16(out, uint16(c.BucketSize))
	out = binary.LittleEndian.AppendUint16(out, c.MaxIterations)
	return binary.LittleEndian.AppendUint16(out, c.Expansion)
}

// Items is the number of items the filter holds.
func (c *CuckooValue) Items() uint64 {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.Inserted
}

// ScanDumpChunk is CF_GetEncodedChunk: the bytes of the filter holding
// position pos, from pos (a whole filter when it fits in max, as RedisBloom
// returns it); next is the position after them, 0 when pos is past the end.
func (c *CuckooValue) ScanDumpChunk(pos int64, max int) (next int64, chunk []byte) {
	c.mu.RLock()
	defer c.mu.RUnlock()
	off := pos - 1
	for _, l := range c.Filters {
		size := int64(len(l.Slots))
		if off < size {
			if size <= int64(max) {
				return pos + size, append([]byte(nil), l.Slots...)
			}
			n := min(size-off, int64(max))
			return pos + n, append([]byte(nil), l.Slots[off:off+n]...)
		}
		off -= size
	}
	return 0, nil
}

// LoadChunk is CF_LoadEncodedChunk: pos is the position after data.
func (c *CuckooValue) LoadChunk(pos int64, data []byte) bool {
	if len(data) == 0 || pos <= 0 || uint64(pos-1) < uint64(len(data)) {
		return false
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	off := pos - int64(len(data)) - 1
	for _, l := range c.Filters {
		size := int64(len(l.Slots))
		if off < size {
			if off+int64(len(data)) > size {
				return false
			}
			copy(l.Slots[off:], data)
			return true
		}
		off -= size
	}
	return false
}

// CuckooFromScanDumpHeader is CFHeader_Load with RedisBloom's integrity
// checks; nil for a header it refuses.
func CuckooFromScanDumpHeader(buf []byte) *CuckooValue {
	if len(buf) != cuckooDumpHdr {
		return nil
	}
	le := binary.LittleEndian
	items, buckets, deletes, filters := le.Uint64(buf), le.Uint64(buf[8:]), le.Uint64(buf[16:]), le.Uint64(buf[24:])
	bucketSize, iterations, expansion := le.Uint16(buf[32:]), le.Uint16(buf[34:]), le.Uint16(buf[36:])
	if bucketSize < 1 || bucketSize > 255 || filters < 1 || filters > 65536 || iterations < 1 || expansion > 32768 ||
		expansion == 0 && filters > 1 || buckets == 0 || buckets > 0x00FFFFFFFFFFFFFF || buckets&(buckets-1) != 0 {
		return nil
	}
	c := &CuckooValue{BucketSize: uint8(bucketSize), MaxIterations: iterations, Expansion: expansion, Inserted: items, Deleted: deletes}
	var total uint64
	for i := uint64(0); i < filters; i++ {
		n := uint64(float64(buckets) * math.Pow(float64(expansion), float64(i)))
		if n == 0 || n > scanDumpMaxFilterBytes/uint64(bucketSize) || total+n*uint64(bucketSize) > scanDumpMaxFilterBytes {
			return nil
		}
		total += n * uint64(bucketSize)
		c.Filters = append(c.Filters, newCuckooLayer(n, uint8(bucketSize)))
	}
	return c
}

// Compact is CF.COMPACT, RedisBloom's CuckooFilter_Compact going on past a
// filter it cannot empty.
func (c *CuckooValue) Compact() {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.compactFilters(true)
}

// Debug is CF.DEBUG's reply.
func (c *CuckooValue) Debug() string {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return fmt.Sprintf("bktsize:%d buckets:%d items:%d deletes:%d filters:%d max_iterations:%d expansion:%d",
		c.BucketSize, c.Filters[0].NumBuckets, c.Inserted, c.Deleted, len(c.Filters), c.MaxIterations, c.Expansion)
}
