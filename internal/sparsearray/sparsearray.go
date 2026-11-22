// Package sparsearray is a Go port of Redis 8.10's sparsearray.c (Copyright Redis Ltd., RSALv2/SSPLv1/AGPLv3).
package sparsearray

import (
	"math"
	"math/bits"
)

const (
	SliceSizeDefault = 4096
	SliceSizeMin     = 256
	SliceSizeMax     = 65536
	KMaxDefault      = 10
	KMinDefault      = 5
	// SuperBlockSlots is the number of slice pointers in one superdir block.
	SuperBlockSlots = 2048
	// InsertIdxNone marks an array on which no insert was performed yet.
	InsertIdxNone = math.MaxUint64

	sliceMinAlloc = 8

	cSliceHeader = 32
	cPtrSize     = 8
)

// Config holds the array tunables. SliceSize is fixed at creation; KMax and
// KMin are consulted on every operation, as Redis reads them from the server
// config each time. When Limits is set it is called at the start of every
// mutating operation to refresh KMax and KMin.
type Config struct {
	SliceSize uint32
	KMax      uint32
	KMin      uint32
	Limits    func() (kmax, kmin uint32)
}

// DefaultConfig returns the Redis defaults.
func DefaultConfig() Config {
	return Config{SliceSize: SliceSizeDefault, KMax: KMaxDefault, KMin: KMinDefault}
}

type slice struct {
	dense bool
	count uint32

	offset  uint32
	winsize uint32
	maxIdx  uint32
	items   []string
	present []uint64

	scap uint32
	offs []uint16
	vals []string
}

type sdirEntry struct {
	blockID uint64
	count   uint32
	slots   *[SuperBlockSlots]*slice
}

// Array is a sparse array indexed by uint64 (UINT64_MAX excluded). It is not
// safe for concurrent use.
type Array struct {
	// InsertIdx is managed by the command layer (ARINSERT/ARSEEK); Dup copies it.
	InsertIdx uint64
	// KMax and KMin are the sparse-slice thresholds used by each operation.
	KMax uint32
	KMin uint32

	limits func() (kmax, kmin uint32)

	count          uint64
	dirHighestUsed uint64
	numSlices      uint64
	sliceSize      uint32
	sliceBits      uint

	dir      []*slice
	superdir bool
	sdir     []sdirEntry
	sdirCap  uint32

	sliceBytes int64
	valBytes   int64
}

// New creates an empty array. A zero SliceSize selects the default; any other
// value must be a power of two in [SliceSizeMin, SliceSizeMax].
func New(cfg Config) *Array {
	ss := cfg.SliceSize
	if ss == 0 {
		ss = SliceSizeDefault
	}
	if ss < SliceSizeMin || ss > SliceSizeMax || ss&(ss-1) != 0 {
		panic("sparsearray: invalid slice size")
	}
	a := &Array{
		InsertIdx: InsertIdxNone,
		KMax:      cfg.KMax,
		KMin:      cfg.KMin,
		limits:    cfg.Limits,
		sliceSize: ss,
		sliceBits: uint(bits.TrailingZeros32(ss)),
	}
	a.refresh()
	return a
}

func (a *Array) refresh() {
	if a.limits != nil {
		a.KMax, a.KMin = a.limits()
	}
}

// SliceSize returns the slice size fixed at creation.
func (a *Array) SliceSize() uint32 { return a.sliceSize }

func (a *Array) sliceID(idx uint64) uint64  { return idx >> a.sliceBits }
func (a *Array) sliceOff(idx uint64) uint32 { return uint32(idx & uint64(a.sliceSize-1)) }

func nextPow2(n uint32) uint32 {
	if n <= 1 {
		return 1
	}
	return 1 << (32 - bits.LeadingZeros32(n-1))
}

func denseAllocSize(winsize uint32) uint64 {
	return cSliceHeader + uint64(winsize)*cPtrSize
}

func sparseAllocSize(c uint32) uint64 {
	offs := uint64(c) * 2
	pad := (cPtrSize - offs%cPtrSize) % cPtrSize
	return cSliceHeader + offs + pad + uint64(c)*cPtrSize
}

func (s *slice) footprint() int64 {
	if s == nil {
		return 0
	}
	if s.dense {
		return 80 + int64(s.winsize)*16 + int64(len(s.present))*8
	}
	return 80 + int64(s.scap)*18
}

func (s *slice) has(pos uint32) bool { return s.present[pos>>6]&(1<<(pos&63)) != 0 }
func (s *slice) mark(pos uint32)     { s.present[pos>>6] |= 1 << (pos & 63) }

func (s *slice) unmark(pos uint32) {
	s.present[pos>>6] &^= 1 << (pos & 63)
	s.items[pos] = ""
}

func (s *slice) setWindow(offset, winsize uint32) {
	s.offset = offset
	s.winsize = winsize
	s.items = make([]string, winsize)
	s.present = make([]uint64, (winsize+63)/64)
}

func (s *slice) maxIndex() uint32 {
	if s.dense {
		return s.maxIdx
	}
	if s.count == 0 {
		return 0
	}
	return uint32(s.offs[s.count-1])
}

func (s *slice) valueBytes() int64 {
	var n int64
	if s.dense {
		for i := range s.items {
			n += int64(len(s.items[i]))
		}
		return n
	}
	for i := range s.vals {
		n += int64(len(s.vals[i]))
	}
	return n
}

func newDense(relIdx, sliceSize uint32) *slice {
	offset := relIdx
	if offset+sliceMinAlloc > sliceSize {
		offset = sliceSize - sliceMinAlloc
	}
	s := &slice{dense: true}
	s.setWindow(offset, sliceMinAlloc)
	return s
}

func (a *Array) newSparse() *slice {
	c := a.KMax
	if c > 4 {
		c = 4
	}
	return &slice{scap: c, offs: make([]uint16, 0, c), vals: make([]string, 0, c)}
}

func (s *slice) rewindow(newOffset, newWinsize uint32) {
	oldItems, oldPresent, oldOffset := s.items, s.present, s.offset
	s.setWindow(newOffset, newWinsize)
	shift := oldOffset - newOffset
	for i := range oldItems {
		if oldPresent[i>>6]&(1<<(uint(i)&63)) != 0 {
			p := uint32(i) + shift
			s.items[p] = oldItems[i]
			s.mark(p)
		}
	}
}

func (s *slice) growRight(rel, sliceSize uint32) {
	nw := s.winsize
	for rel >= s.offset+nw && nw < sliceSize {
		nw <<= 1
	}
	no := s.offset
	if nw >= sliceSize {
		nw = sliceSize
		no = 0
	} else if no+nw > sliceSize {
		no = sliceSize - nw
	}
	s.rewindow(no, nw)
}

func (s *slice) growLeft(rel, sliceSize uint32) {
	oldEnd := s.offset + s.winsize
	nw := nextPow2(oldEnd - rel)
	if nw < sliceMinAlloc {
		nw = sliceMinAlloc
	}
	if nw > sliceSize {
		nw = sliceSize
	}
	no := int64(oldEnd) - int64(nw)
	if no < 0 {
		no = 0
	}
	if nw == sliceSize {
		no = 0
	}
	s.rewindow(uint32(no), nw)
}

func (s *slice) growIfNeeded(rel, sliceSize uint32) {
	if rel >= s.offset+s.winsize {
		s.growRight(rel, sliceSize)
	} else if rel < s.offset {
		s.growLeft(rel, sliceSize)
	}
}

func (s *slice) findPos(rel uint16) (uint32, bool) {
	lo, hi := uint32(0), s.count
	for lo < hi {
		mid := lo + (hi-lo)/2
		if s.offs[mid] < rel {
			lo = mid + 1
		} else {
			hi = mid
		}
	}
	return lo, lo < s.count && s.offs[lo] == rel
}

func (s *slice) promote(sliceSize uint32) {
	if s.count == 0 {
		*s = *newDense(0, sliceSize)
		return
	}
	minOff := uint32(s.offs[0])
	maxOff := uint32(s.offs[s.count-1])
	winsize := nextPow2(maxOff - minOff + 1)
	if winsize < sliceMinAlloc {
		winsize = sliceMinAlloc
	}
	offset := minOff
	if winsize >= sliceSize {
		winsize = sliceSize
		offset = 0
	} else if offset+winsize > sliceSize {
		offset = sliceSize - winsize
	}
	offs, vals := s.offs, s.vals
	*s = slice{dense: true, count: s.count, maxIdx: maxOff}
	s.setWindow(offset, winsize)
	for i := range offs {
		p := uint32(offs[i]) - offset
		s.items[p] = vals[i]
		s.mark(p)
	}
}

func (a *Array) maybeDemote(d *slice) {
	kmax, kmin := a.KMax, a.KMin
	if kmax == 0 || d.count > kmin || d.count > kmax || d.winsize == sliceMinAlloc {
		return
	}
	if d.winsize < 4*kmin {
		return
	}
	if denseAllocSize(d.winsize) < sparseAllocSize(kmin)*5/4 {
		return
	}
	offs := make([]uint16, 0, kmin)
	vals := make([]string, 0, kmin)
	for i := uint32(0); i < d.winsize && uint32(len(offs)) < d.count; i++ {
		if d.has(i) {
			offs = append(offs, uint16(d.offset+i))
			vals = append(vals, d.items[i])
		}
	}
	*d = slice{count: uint32(len(offs)), scap: kmin, offs: offs, vals: vals}
}

func (s *slice) updateMaxIdx(deletedOff uint32) {
	if s.count == 0 || deletedOff < s.maxIdx {
		return
	}
	for pos := int64(s.maxIdx) - int64(s.offset); pos >= 0; pos-- {
		if s.has(uint32(pos)) {
			s.maxIdx = s.offset + uint32(pos)
			return
		}
	}
}

func (a *Array) superDirFind(blockID uint64) (uint32, bool) {
	lo, hi := uint32(0), uint32(len(a.sdir))
	for lo < hi {
		mid := lo + (hi-lo)/2
		if a.sdir[mid].blockID < blockID {
			lo = mid + 1
		} else {
			hi = mid
		}
	}
	return lo, lo < uint32(len(a.sdir)) && a.sdir[lo].blockID == blockID
}

func (a *Array) superDirEnsure(blockID uint64) *sdirEntry {
	pos, found := a.superDirFind(blockID)
	if !found {
		n := len(a.sdir)
		if uint32(n) >= a.sdirCap {
			nc := uint32(4)
			if a.sdirCap != 0 {
				nc = a.sdirCap * 2
			}
			ns := make([]sdirEntry, n, nc)
			copy(ns, a.sdir)
			a.sdir = ns
			a.sdirCap = nc
		}
		a.sdir = a.sdir[:n+1]
		copy(a.sdir[pos+1:], a.sdir[pos:n])
		a.sdir[pos] = sdirEntry{blockID: blockID, slots: new([SuperBlockSlots]*slice)}
	}
	return &a.sdir[pos]
}

func (a *Array) superDirRemoveBlock(pos uint32) {
	n := len(a.sdir)
	copy(a.sdir[pos:], a.sdir[pos+1:])
	a.sdir[n-1] = sdirEntry{}
	a.sdir = a.sdir[:n-1]
}

func (a *Array) promoteToSuperDir() {
	a.superdir = true
	a.sdirCap = 4
	a.sdir = make([]sdirEntry, 0, 4)
	if len(a.dir) > 0 {
		e := sdirEntry{blockID: 0, slots: new([SuperBlockSlots]*slice)}
		for i, s := range a.dir {
			e.slots[i] = s
			if s != nil {
				e.count++
			}
		}
		a.sdir = append(a.sdir, e)
	}
	a.dir = nil
}

func (a *Array) dirGrow(sliceID uint64) {
	if !a.superdir && sliceID >= SuperBlockSlots {
		a.promoteToSuperDir()
	}
	if a.superdir {
		return
	}
	cur := uint64(len(a.dir))
	if sliceID < cur {
		return
	}
	n := cur
	if n == 0 {
		n = 1
	}
	for n <= sliceID {
		n <<= 1
	}
	nd := make([]*slice, n)
	copy(nd, a.dir)
	a.dir = nd
}

func (a *Array) dirMaybeShrink() {
	if a.superdir || a.count == 0 {
		return
	}
	alloc := uint64(len(a.dir))
	if a.dirHighestUsed >= alloc*9/10 {
		return
	}
	n := uint64(1)
	for n <= a.dirHighestUsed {
		n <<= 1
	}
	if n >= alloc {
		return
	}
	nd := make([]*slice, n)
	copy(nd, a.dir)
	a.dir = nd
}

func (a *Array) scanHighestSuperDir() bool {
	for bi := len(a.sdir) - 1; bi >= 0; bi-- {
		e := &a.sdir[bi]
		if e.count == 0 {
			continue
		}
		for si := SuperBlockSlots - 1; si >= 0; si-- {
			if e.slots[si] != nil {
				a.dirHighestUsed = e.blockID*SuperBlockSlots + uint64(si)
				return true
			}
		}
	}
	return false
}

func (a *Array) dirUpdateHighest(freedID uint64) {
	if a.count == 0 || freedID < a.dirHighestUsed {
		return
	}
	if a.superdir {
		if !a.scanHighestSuperDir() {
			a.dirHighestUsed = 0
		}
		return
	}
	for i := int64(freedID) - 1; i >= 0; i-- {
		if a.dir[i] != nil {
			a.dirHighestUsed = uint64(i)
			return
		}
	}
	a.dirHighestUsed = 0
}

func (a *Array) getSlice(sliceID uint64) *slice {
	if a.superdir {
		pos, found := a.superDirFind(sliceID / SuperBlockSlots)
		if !found {
			return nil
		}
		return a.sdir[pos].slots[sliceID%SuperBlockSlots]
	}
	if sliceID >= uint64(len(a.dir)) {
		return nil
	}
	return a.dir[sliceID]
}

func (a *Array) setSlice(sliceID uint64, s *slice) {
	if a.superdir {
		off := sliceID % SuperBlockSlots
		if s == nil {
			pos, found := a.superDirFind(sliceID / SuperBlockSlots)
			if !found {
				return
			}
			e := &a.sdir[pos]
			if e.slots[off] != nil {
				e.slots[off] = nil
				e.count--
				a.numSlices--
				if e.count == 0 {
					a.superDirRemoveBlock(pos)
				}
			}
			return
		}
		e := a.superDirEnsure(sliceID / SuperBlockSlots)
		if e.slots[off] == nil {
			e.count++
			a.numSlices++
		}
		e.slots[off] = s
		return
	}
	if s == nil && a.dir[sliceID] != nil {
		a.numSlices--
	} else if s != nil && a.dir[sliceID] == nil {
		a.numSlices++
	}
	a.dir[sliceID] = s
}

func (a *Array) freeSlice(s *slice) {
	a.sliceBytes -= s.footprint()
	a.valBytes -= s.valueBytes()
}

// Get returns the value at idx and whether it is populated.
func (a *Array) Get(idx uint64) (string, bool) {
	s := a.getSlice(a.sliceID(idx))
	if s == nil {
		return "", false
	}
	rel := a.sliceOff(idx)
	if s.dense {
		if rel < s.offset || rel >= s.offset+s.winsize {
			return "", false
		}
		p := rel - s.offset
		if !s.has(p) {
			return "", false
		}
		return s.items[p], true
	}
	pos, found := s.findPos(uint16(rel))
	if !found {
		return "", false
	}
	return s.vals[pos], true
}

// Set stores v at idx. idx must not be math.MaxUint64.
func (a *Array) Set(idx uint64, v string) {
	if idx == math.MaxUint64 {
		panic("sparsearray: index UINT64_MAX is reserved")
	}
	a.refresh()
	sid := a.sliceID(idx)
	rel := a.sliceOff(idx)
	a.dirGrow(sid)

	s := a.getSlice(sid)
	if s == nil {
		if a.KMax > 0 {
			s = a.newSparse()
		} else {
			s = newDense(rel, a.sliceSize)
		}
		a.sliceBytes += s.footprint()
		a.setSlice(sid, s)
	}
	before := s.footprint()

	if s.dense {
		a.denseStore(s, rel, v)
	} else {
		pos, found := s.findPos(uint16(rel))
		switch {
		case found:
			a.valBytes += int64(len(v)) - int64(len(s.vals[pos]))
			s.vals[pos] = v
		case s.count >= a.KMax:
			s.promote(a.sliceSize)
			a.denseStore(s, rel, v)
		default:
			if s.count >= s.scap {
				nc := s.scap * 2
				if nc > a.KMax {
					nc = a.KMax
				}
				no := make([]uint16, s.count, nc)
				nv := make([]string, s.count, nc)
				copy(no, s.offs)
				copy(nv, s.vals)
				s.offs, s.vals, s.scap = no, nv, nc
			}
			s.offs = append(s.offs, 0)
			s.vals = append(s.vals, "")
			copy(s.offs[pos+1:], s.offs[pos:s.count])
			copy(s.vals[pos+1:], s.vals[pos:s.count])
			s.offs[pos] = uint16(rel)
			s.vals[pos] = v
			s.count++
			a.count++
			a.valBytes += int64(len(v))
		}
	}
	a.sliceBytes += s.footprint() - before

	if sid > a.dirHighestUsed || a.count == 1 {
		a.dirHighestUsed = sid
	}
}

func (a *Array) denseStore(s *slice, rel uint32, v string) {
	s.growIfNeeded(rel, a.sliceSize)
	p := rel - s.offset
	if s.has(p) {
		a.valBytes -= int64(len(s.items[p]))
	} else {
		s.count++
		a.count++
		s.mark(p)
	}
	s.items[p] = v
	a.valBytes += int64(len(v))
	if rel > s.maxIdx {
		s.maxIdx = rel
	}
}

// Del removes the element at idx, reporting whether it was populated.
func (a *Array) Del(idx uint64) bool {
	a.refresh()
	sid := a.sliceID(idx)
	rel := a.sliceOff(idx)
	s := a.getSlice(sid)
	if s == nil {
		return false
	}
	if s.dense {
		if rel < s.offset || rel >= s.offset+s.winsize {
			return false
		}
		p := rel - s.offset
		if !s.has(p) {
			return false
		}
		a.valBytes -= int64(len(s.items[p]))
		s.unmark(p)
		s.count--
		a.count--
		s.updateMaxIdx(rel)
		if s.count != 0 {
			before := s.footprint()
			a.maybeDemote(s)
			a.sliceBytes += s.footprint() - before
			return true
		}
	} else {
		pos, found := s.findPos(uint16(rel))
		if !found {
			return false
		}
		a.valBytes -= int64(len(s.vals[pos]))
		copy(s.offs[pos:], s.offs[pos+1:])
		copy(s.vals[pos:], s.vals[pos+1:])
		s.vals[s.count-1] = ""
		s.offs = s.offs[:s.count-1]
		s.vals = s.vals[:s.count-1]
		s.count--
		a.count--
	}
	if s.count == 0 {
		a.freeSlice(s)
		a.setSlice(sid, nil)
		a.dirUpdateHighest(sid)
		a.dirMaybeShrink()
	}
	return true
}

func (a *Array) deleteSliceRange(sid uint64, delLo, delHi uint32) uint64 {
	s := a.getSlice(sid)
	if s == nil {
		return 0
	}
	var deleted uint64
	if s.dense {
		winLo := s.offset
		winHi := s.offset + s.winsize - 1
		effLo, effHi := max(delLo, winLo), min(delHi, winHi)
		if effLo <= effHi {
			for off := effLo; off <= effHi; off++ {
				p := off - s.offset
				if s.has(p) {
					a.valBytes -= int64(len(s.items[p]))
					s.unmark(p)
					s.count--
					a.count--
					deleted++
				}
			}
			if s.count > 0 && s.maxIdx >= effLo {
				s.maxIdx = s.offset
				for i := int64(winHi); i >= int64(winLo); i-- {
					if s.has(uint32(i) - s.offset) {
						s.maxIdx = uint32(i)
						break
					}
				}
			}
		}
		if s.count == 0 {
			a.freeSlice(s)
			a.setSlice(sid, nil)
		} else {
			before := s.footprint()
			a.maybeDemote(s)
			a.sliceBytes += s.footprint() - before
		}
		return deleted
	}
	first, _ := s.findPos(uint16(delLo))
	last, found := s.findPos(uint16(delHi))
	if found {
		last++
	}
	if first < last {
		for i := first; i < last; i++ {
			a.valBytes -= int64(len(s.vals[i]))
		}
		n := copy(s.offs[first:], s.offs[last:])
		copy(s.vals[first:], s.vals[last:])
		newCount := first + uint32(n)
		clear(s.vals[newCount:s.count])
		s.offs = s.offs[:newCount]
		s.vals = s.vals[:newCount]
		deleted = uint64(last - first)
		s.count = newCount
		a.count -= deleted
	}
	if s.count == 0 {
		a.freeSlice(s)
		a.setSlice(sid, nil)
	}
	return deleted
}

// DeleteRange removes every element in [lo, hi] and returns how many were
// removed. Work is proportional to the slices and elements touched.
func (a *Array) DeleteRange(lo, hi uint64) uint64 {
	a.refresh()
	if a.count == 0 || lo > hi {
		return 0
	}
	ss := a.sliceSize
	loS, hiS := a.sliceID(lo), a.sliceID(hi)
	loOff, hiOff := a.sliceOff(lo), a.sliceOff(hi)
	var deleted uint64
	touched := false

	if loS == hiS {
		deleted = a.deleteSliceRange(loS, loOff, hiOff)
		if loS >= a.dirHighestUsed {
			touched = true
		}
	} else {
		deleted += a.deleteSliceRange(loS, loOff, ss-1)
		deleted += a.deleteSliceRange(hiS, 0, hiOff)
		if hiS >= a.dirHighestUsed {
			touched = true
		}
		if loS+1 <= hiS-1 {
			midLo, midHi := loS+1, hiS-1
			if a.superdir {
				start, _ := a.superDirFind(midLo / SuperBlockSlots)
				end, found := a.superDirFind(midHi / SuperBlockSlots)
				if found {
					end++
				}
				for bi := int64(end) - 1; bi >= int64(start); bi-- {
					e := &a.sdir[bi]
					base := e.blockID * SuperBlockSlots
					bend := base + SuperBlockSlots - 1
					firstSI := uint32(0)
					if midLo > base {
						firstSI = uint32(midLo - base)
					}
					lastSI := uint32(SuperBlockSlots - 1)
					if midHi < bend {
						lastSI = uint32(midHi - base)
					}
					for si := firstSI; si <= lastSI; si++ {
						if s := e.slots[si]; s != nil {
							deleted += uint64(s.count)
							a.count -= uint64(s.count)
							a.freeSlice(s)
							e.slots[si] = nil
							e.count--
							a.numSlices--
							if base+uint64(si) >= a.dirHighestUsed {
								touched = true
							}
						}
					}
					if e.count == 0 {
						a.superDirRemoveBlock(uint32(bi))
					}
				}
			} else {
				end := midHi
				if end >= uint64(len(a.dir)) {
					end = uint64(len(a.dir)) - 1
				}
				for sid := midLo; sid <= end; sid++ {
					if s := a.dir[sid]; s != nil {
						deleted += uint64(s.count)
						a.count -= uint64(s.count)
						a.freeSlice(s)
						a.dir[sid] = nil
						a.numSlices--
						if sid >= a.dirHighestUsed {
							touched = true
						}
					}
				}
			}
		}
	}

	if touched && a.count > 0 {
		a.dirHighestUsed = 0
		if a.superdir {
			a.scanHighestSuperDir()
		} else {
			for i := len(a.dir) - 1; i >= 0; i-- {
				if a.dir[i] != nil {
					a.dirHighestUsed = uint64(i)
					break
				}
			}
		}
	}
	if a.count == 0 {
		a.dirHighestUsed = 0
	}
	a.dirMaybeShrink()
	return deleted
}

// Truncate removes every element with index >= limit.
func (a *Array) Truncate(limit uint64) {
	if a.count == 0 {
		return
	}
	n := a.Len()
	if limit >= n {
		return
	}
	a.DeleteRange(limit, n-1)
}

// Count returns the number of populated elements.
func (a *Array) Count() uint64 { return a.count }

// Len returns the highest populated index plus one, or 0 when empty.
func (a *Array) Len() uint64 {
	if a.count == 0 {
		return 0
	}
	s := a.getSlice(a.dirHighestUsed)
	if s == nil {
		return 0
	}
	return (a.dirHighestUsed<<a.sliceBits | uint64(s.maxIndex())) + 1
}

// MayPromoteToDenseForRangeSet prepares the slices covering [lo, hi] for a
// bulk write of every index in the range, promoting to dense (or creating
// empty dense slices) the ones the write would push past KMax. The caller is
// expected to Set every index in the range afterwards; the loop is
// proportional to the number of slices in the range.
func (a *Array) MayPromoteToDenseForRangeSet(lo, hi uint64) {
	a.refresh()
	if a.KMax == 0 {
		return
	}
	sLo, sHi := a.sliceID(lo), a.sliceID(hi)
	a.dirGrow(sHi)
	for sid := sLo; sid <= sHi; sid++ {
		rs := sid << a.sliceBits
		if sid == sLo {
			rs = lo
		}
		var re uint64
		if sid == sHi {
			re = hi
		} else {
			re = (sid+1)<<a.sliceBits - 1
		}
		startOff, endOff := a.sliceOff(rs), a.sliceOff(re)
		rangeSize := endOff - startOff + 1

		s := a.getSlice(sid)
		if s == nil {
			if rangeSize > a.KMax {
				ns := newDense(startOff, a.sliceSize)
				a.sliceBytes += ns.footprint()
				a.setSlice(sid, ns)
			}
		} else if !s.dense {
			promote := rangeSize > a.KMax
			if !promote {
				var existing uint32
				for _, o := range s.offs {
					if uint32(o) >= startOff && uint32(o) <= endOff {
						existing++
					}
				}
				promote = s.count+rangeSize-existing > a.KMax
			}
			if promote {
				before := s.footprint()
				s.promote(a.sliceSize)
				a.sliceBytes += s.footprint() - before
			}
		}
	}
}

// Dup returns a deep copy of the array, including InsertIdx and the limits.
func (a *Array) Dup() *Array {
	d := *a
	if a.superdir {
		d.sdir = make([]sdirEntry, len(a.sdir), a.sdirCap)
		for i := range a.sdir {
			src := &a.sdir[i]
			dst := &d.sdir[i]
			dst.blockID = src.blockID
			dst.count = src.count
			dst.slots = new([SuperBlockSlots]*slice)
			for j, s := range src.slots {
				if s != nil {
					dst.slots[j] = s.clone()
				}
			}
		}
	} else if a.dir != nil {
		d.dir = make([]*slice, len(a.dir))
		for i, s := range a.dir {
			if s != nil {
				d.dir[i] = s.clone()
			}
		}
	}
	return &d
}

func (s *slice) clone() *slice {
	c := *s
	if s.dense {
		c.items = append([]string(nil), s.items...)
		c.present = append([]uint64(nil), s.present...)
	} else {
		c.offs = make([]uint16, s.count, s.scap)
		c.vals = make([]string, s.count, s.scap)
		copy(c.offs, s.offs)
		copy(c.vals, s.vals)
	}
	return &c
}

// Scan calls fn for every populated element with index in [min(start,end),
// max(start,end)], in ascending order when start <= end and descending order
// otherwise, stopping early when fn returns false. fn must not modify the
// array. Work is proportional to the slices and elements touched, not to the
// numeric span.
func (a *Array) Scan(start, end uint64, fn func(idx uint64, v string) bool) {
	if a.count == 0 {
		return
	}
	reverse := start > end
	lo, hi := start, end
	if reverse {
		lo, hi = end, start
	}
	loS, hiS := a.sliceID(lo), a.sliceID(hi)
	if loS > a.dirHighestUsed {
		return
	}
	if hiS > a.dirHighestUsed {
		hiS = a.dirHighestUsed
		hi = hiS<<a.sliceBits | uint64(a.sliceSize-1)
	}
	visit := func(sid uint64, s *slice) bool {
		offLo, offHi := uint32(0), a.sliceSize-1
		if sid == loS {
			offLo = a.sliceOff(lo)
		}
		if sid == hiS {
			offHi = a.sliceOff(hi)
		}
		return s.scan(sid<<a.sliceBits, offLo, offHi, reverse, fn)
	}

	if !a.superdir {
		if reverse {
			for sid := hiS; ; sid-- {
				if s := a.dir[sid]; s != nil && !visit(sid, s) {
					return
				}
				if sid == loS {
					return
				}
			}
		}
		for sid := loS; sid <= hiS; sid++ {
			if s := a.dir[sid]; s != nil && !visit(sid, s) {
				return
			}
		}
		return
	}

	startB, _ := a.superDirFind(loS / SuperBlockSlots)
	endB, found := a.superDirFind(hiS / SuperBlockSlots)
	if found {
		endB++
	}
	blockRange := func(e *sdirEntry) (uint32, uint32) {
		base := e.blockID * SuperBlockSlots
		first, last := uint32(0), uint32(SuperBlockSlots-1)
		if loS > base {
			first = uint32(loS - base)
		}
		if hiS < base+SuperBlockSlots-1 {
			last = uint32(hiS - base)
		}
		return first, last
	}
	if reverse {
		for bi := int64(endB) - 1; bi >= int64(startB); bi-- {
			e := &a.sdir[bi]
			first, last := blockRange(e)
			base := e.blockID * SuperBlockSlots
			for si := int64(last); si >= int64(first); si-- {
				if s := e.slots[si]; s != nil && !visit(base+uint64(si), s) {
					return
				}
			}
		}
		return
	}
	for bi := startB; bi < endB; bi++ {
		e := &a.sdir[bi]
		first, last := blockRange(e)
		base := e.blockID * SuperBlockSlots
		for si := first; si <= last; si++ {
			if s := e.slots[si]; s != nil && !visit(base+uint64(si), s) {
				return
			}
		}
	}
}

func (s *slice) scan(base uint64, offLo, offHi uint32, reverse bool, fn func(uint64, string) bool) bool {
	if s.dense {
		winHi := s.offset + s.winsize - 1
		offLo, offHi = max(offLo, s.offset), min(offHi, winHi)
		if offLo > offHi {
			return true
		}
		if reverse {
			for off := int64(offHi); off >= int64(offLo); off-- {
				p := uint32(off) - s.offset
				if s.has(p) && !fn(base+uint64(off), s.items[p]) {
					return false
				}
			}
			return true
		}
		for off := offLo; off <= offHi; off++ {
			p := off - s.offset
			if s.has(p) && !fn(base+uint64(off), s.items[p]) {
				return false
			}
		}
		return true
	}
	if reverse {
		pos, found := s.findPos(uint16(offHi))
		i := int64(pos)
		if !found {
			i--
		}
		for ; i >= 0 && uint32(s.offs[i]) >= offLo; i-- {
			if !fn(base+uint64(s.offs[i]), s.vals[i]) {
				return false
			}
		}
		return true
	}
	pos, _ := s.findPos(uint16(offLo))
	for i := pos; i < s.count && uint32(s.offs[i]) <= offHi; i++ {
		if !fn(base+uint64(s.offs[i]), s.vals[i]) {
			return false
		}
	}
	return true
}

// Info mirrors the fields ARINFO reports. The per-encoding totals are only
// filled when Info is called with full set, which costs O(directory).
type Info struct {
	Count           uint64
	Len             uint64
	NumSlices       uint64
	DirectorySize   uint64
	SuperDirEntries uint64
	SliceSize       uint32

	DenseSlices       uint64
	SparseSlices      uint64
	DenseTotalWinsize uint64
	DenseTotalCount   uint64
	SparseTotalCap    uint64
}

// Info returns the ARINFO metadata; full adds the per-encoding slice totals.
func (a *Array) Info(full bool) Info {
	in := Info{
		Count:     a.count,
		Len:       a.Len(),
		NumSlices: a.numSlices,
		SliceSize: a.sliceSize,
	}
	if a.superdir {
		in.DirectorySize = uint64(a.sdirCap)
		in.SuperDirEntries = uint64(len(a.sdir))
	} else {
		in.DirectorySize = uint64(len(a.dir))
	}
	if !full {
		return in
	}
	add := func(s *slice) {
		if s == nil {
			return
		}
		if s.dense {
			in.DenseSlices++
			in.DenseTotalWinsize += uint64(s.winsize)
			in.DenseTotalCount += uint64(s.count)
		} else {
			in.SparseSlices++
			in.SparseTotalCap += uint64(s.scap)
		}
	}
	if a.superdir {
		for i := range a.sdir {
			for _, s := range a.sdir[i].slots {
				add(s)
			}
		}
	} else {
		for _, s := range a.dir {
			add(s)
		}
	}
	return in
}

// SizeOf returns an approximate memory footprint in bytes.
func (a *Array) SizeOf() int64 {
	n := int64(160) + a.sliceBytes + a.valBytes
	if a.superdir {
		n += int64(a.sdirCap)*32 + int64(len(a.sdir))*SuperBlockSlots*8
	} else {
		n += int64(len(a.dir)) * 8
	}
	return n
}
