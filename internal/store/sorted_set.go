package store

import (
	"encoding/binary"
	"fmt"
	"iter"
	"math"
	"sync"
	"sync/atomic"
	"unsafe"
)

// SortedSetValue holds a set of at most zsetSmallMax members packed in one
// buffer in (score, member) order, like Redis' listpack, and a bigger one as
// a map of scores with a skiplist for the order, built on first use. The
// buffer is copy-on-write, so members read from it stay valid.
type SortedSetValue struct {
	zl []byte
	zn int
	// adj corrects size, counted at the map form's rates, to the form the
	// set is in.
	adj     atomic.Int64
	dict    map[string]float64
	mu      sync.RWMutex
	dead    bool
	size    sizeCounter
	index   atomic.Pointer[zskiplist]
	indexMu sync.Mutex
	scanCache
	big bool
}

// zsetSmallMax is the most members the packed form holds: its lookups are
// linear, as a listpack's are, and zsetSmallBytes bounds what a change
// copies.
const (
	zsetSmallMax   = 128
	zsetSmallBytes = 16 << 10
)

// zlEntry locates a member in the packed buffer: an uvarint length, the
// member, and its score as 8 little-endian bytes.
type zlEntry struct {
	start, ms, me, end int
}

func (v *SortedSetValue) zlAt(off int) zlEntry {
	n, k := binary.Uvarint(v.zl[off:])
	e := zlEntry{start: off, ms: off + k}
	e.me = e.ms + int(n)
	e.end = e.me + 8
	return e
}

func (v *SortedSetValue) zlScore(e zlEntry) float64 {
	return math.Float64frombits(binary.LittleEndian.Uint64(v.zl[e.me:e.end]))
}

func (v *SortedSetValue) zlEntryOf(e zlEntry) SortedEntry {
	m := ""
	if e.me > e.ms {
		m = unsafe.String(&v.zl[e.ms], e.me-e.ms)
	}
	return SortedEntry{Member: m, Score: v.zlScore(e)}
}

func (v *SortedSetValue) zlEach(fn func(i int, e zlEntry) bool) {
	for i, off := 0, 0; off < len(v.zl); i++ {
		e := v.zlAt(off)
		if !fn(i, e) {
			return
		}
		off = e.end
	}
}

// zlSplice replaces zl[from:to] with ins in a new buffer of the exact size.
func (v *SortedSetValue) zlSplice(from, to int, ins []byte) {
	buf := make([]byte, 0, len(v.zl)-(to-from)+len(ins))
	buf = append(buf, v.zl[:from]...)
	buf = append(buf, ins...)
	v.zl = append(buf, v.zl[to:]...)
}

func zlEncode(member string, score float64) []byte {
	if score == 0 {
		// A listpack keeps a zero score as the integer 0, so -0 loses its
		// sign there, and keeps losing it once the set converts.
		score = 0
	}
	buf := make([]byte, 0, binary.MaxVarintLen32+len(member)+8)
	buf = binary.AppendUvarint(buf, uint64(len(member)))
	buf = append(buf, member...)
	return binary.LittleEndian.AppendUint64(buf, math.Float64bits(score))
}

// NewSortedSet returns an empty sorted set sized for n members.
func NewSortedSet(n int) *SortedSetValue {
	if n > zsetSmallMax {
		v := &SortedSetValue{dict: make(map[string]float64, n)}
		v.adj.Store(zsetBaseSize - zsetPackedBase)
		return v
	}
	return &SortedSetValue{}
}

// NewSortedSetFrom returns a sorted set of the members and scores.
func NewSortedSetFrom(members map[string]float64) *SortedSetValue {
	v := NewSortedSet(len(members))
	for m, score := range members {
		v.Put(m, score)
	}
	return v
}

func (v *SortedSetValue) Lock()    { v.mu.Lock() }
func (v *SortedSetValue) Unlock()  { v.mu.Unlock() }
func (v *SortedSetValue) RLock()   { v.mu.RLock() }
func (v *SortedSetValue) RUnlock() { v.mu.RUnlock() }

// IsDead reports whether DeleteIfEmpty removed this value from the keyspace
// (see ListValue.IsDead). The caller holds the value's lock.
func (v *SortedSetValue) IsDead() bool { return v.dead }

func (v *SortedSetValue) emptyLocked() bool { return v.Card() == 0 }
func (v *SortedSetValue) markDeadLocked()   { v.dead = true }

func (v *SortedSetValue) Type() DataType { return DataTypeSortedSet }
func (v *SortedSetValue) SizeOf() int64 {
	if n, ok := v.size.load(); ok {
		return zsetPackedBase + n + v.adj.Load()
	}
	v.mu.RLock()
	defer v.mu.RUnlock()
	var n int64
	for k := range v.All() {
		n += zsetMemberSize(k)
	}
	v.size.set(n)
	return zsetPackedBase + n + v.adj.Load()
}
func (v *SortedSetValue) String() string {
	v.mu.RLock()
	defer v.mu.RUnlock()
	result := ""
	for member, score := range v.All() {
		if result != "" {
			result += ", "
		}
		result += fmt.Sprintf("%s: %.2f", member, score)
	}
	return result
}
func (v *SortedSetValue) Clone() Value {
	v.mu.RLock()
	defer v.mu.RUnlock()
	cloned := &SortedSetValue{big: v.big}
	cloned.adj.Store(v.adj.Load())
	if v.dict == nil {
		cloned.zl, cloned.zn = append([]byte(nil), v.zl...), v.zn
		return cloned
	}
	cloned.dict = make(map[string]float64, len(v.dict))
	for k, score := range v.dict {
		cloned.dict[k] = score
	}
	if z := v.index.Load(); z != nil && z.length == len(v.dict) {
		c := newZskiplist()
		for x := z.first(); x != nil; x = x.level[0].forward {
			c.insert(x.score, x.member)
		}
		cloned.index.Store(c)
	}
	return cloned
}

type SortedEntry struct {
	Member string
	Score  float64
}

type sortedEntries []SortedEntry

func (s sortedEntries) Len() int { return len(s) }
func (s sortedEntries) Less(i, j int) bool {
	return s[i].Score < s[j].Score || (s[i].Score == s[j].Score && s[i].Member < s[j].Member)
}
func (s sortedEntries) Swap(i, j int) { s[i], s[j] = s[j], s[i] }

// All yields every member and its score: in (score, member) order for a
// small set, in no particular order for a big one. The caller holds the
// value's lock and does not change the set while iterating.
func (v *SortedSetValue) All() iter.Seq2[string, float64] {
	return func(yield func(string, float64) bool) {
		if v.dict == nil {
			v.zlEach(func(_ int, e zlEntry) bool {
				se := v.zlEntryOf(e)
				return yield(se.Member, se.Score)
			})
			return
		}
		for m, score := range v.dict {
			if !yield(m, score) {
				return
			}
		}
	}
}

// ordered returns the skiplist of a big set, building it from the map when
// it is missing or out of step. The caller holds the value's lock (a read
// lock is enough).
func (v *SortedSetValue) ordered() *zskiplist {
	if z := v.index.Load(); z != nil && z.length == len(v.dict) {
		return z
	}
	v.indexMu.Lock()
	defer v.indexMu.Unlock()
	if z := v.index.Load(); z != nil && z.length == len(v.dict) {
		return z
	}
	z := newZskiplist()
	for m, score := range v.dict {
		z.insert(score, m)
	}
	v.index.Store(z)
	return z
}

func (v *SortedSetValue) smallFind(member string) (zlEntry, bool) {
	var found zlEntry
	ok := false
	v.zlEach(func(_ int, e zlEntry) bool {
		if e.me-e.ms == len(member) && string(v.zl[e.ms:e.me]) == member {
			found, ok = e, true
			return false
		}
		return true
	})
	return found, ok
}

// smallPos is the rank and offset where (score, member) goes in the buffer.
func (v *SortedSetValue) smallPos(score float64, member string) (int, int) {
	rank, off := v.zn, len(v.zl)
	v.zlEach(func(i int, e zlEntry) bool {
		s := v.zlScore(e)
		if s > score || (s == score && string(v.zl[e.ms:e.me]) >= member) {
			rank, off = i, e.start
			return false
		}
		return true
	})
	return rank, off
}

func (v *SortedSetValue) Remove(member string) bool {
	if v.dict == nil {
		e, ok := v.smallFind(member)
		if !ok {
			return false
		}
		v.zlSplice(e.start, e.end, nil)
		v.zn--
		v.adj.Add(zsetMemberOverhd - zsetPackedOverhd)
	} else {
		score, exists := v.dict[member]
		if !exists {
			return false
		}
		if z := v.index.Load(); z != nil {
			z.delete(score, member)
		}
		delete(v.dict, member)
	}
	v.size.add(-zsetMemberSize(member))
	v.invalidateScan()
	return true
}

func (v *SortedSetValue) GetScore(member string) (float64, bool) {
	if v.dict == nil {
		if e, ok := v.smallFind(member); ok {
			return v.zlScore(e), true
		}
		return 0, false
	}
	score, exists := v.dict[member]
	return score, exists
}

// Dict returns a big set's map of scores, nil for a small set. The caller
// holds the value's lock and does not change the map.
func (v *SortedSetValue) Dict() map[string]float64 { return v.dict }

// Map returns a copy of the members and their scores.
func (v *SortedSetValue) Map() map[string]float64 {
	m := make(map[string]float64, v.Card())
	for k, score := range v.All() {
		m[k] = score
	}
	return m
}

// Score returns member's score, 0 when it is not a member.
func (v *SortedSetValue) Score(member string) float64 {
	score, _ := v.GetScore(member)
	return score
}

// Add sets member's score and reports whether the member is new. The caller
// holds the value's write lock.
func (v *SortedSetValue) Add(member string, score float64) bool {
	added := v.Put(member, score)
	if added {
		v.noteAddLocked(v.Card(), len(member))
	}
	return added
}

// Put is Add for a set being built, which leaves the encoding to the caller.
func (v *SortedSetValue) Put(member string, score float64) bool {
	if v.dict == nil {
		e, exists := v.smallFind(member)
		if exists {
			if v.zlScore(e) == score {
				return false
			}
			v.zlSplice(e.start, e.end, nil)
			v.zn--
		} else if v.zn >= zsetSmallMax {
			v.toDict()
			return v.Put(member, score)
		}
		_, off := v.smallPos(score, member)
		v.zlSplice(off, off, zlEncode(member, score))
		v.zn++
		if !exists {
			v.adj.Add(zsetPackedOverhd - zsetMemberOverhd)
		}
		if len(v.zl) > zsetSmallBytes {
			v.toDict()
		}
		if exists {
			return false
		}
	} else {
		z := v.ordered()
		old, exists := v.dict[member]
		if exists && old == score {
			return false
		}
		if exists {
			z.delete(old, member)
		}
		z.insert(score, member)
		v.dict[member] = score
		if exists {
			return false
		}
	}
	v.size.add(zsetMemberSize(member))
	v.invalidateScan()
	return true
}

func (v *SortedSetValue) toDict() {
	v.dict = make(map[string]float64, v.zn*2)
	z := newZskiplist()
	v.zlEach(func(_ int, e zlEntry) bool {
		m, score := string(v.zl[e.ms:e.me]), v.zlScore(e)
		v.dict[m] = score
		z.insert(score, m)
		return true
	})
	v.zl, v.zn = nil, 0
	v.index.Store(z)
	v.adj.Store(zsetBaseSize - zsetPackedBase)
}

func (v *SortedSetValue) Card() int {
	if v.dict == nil {
		return v.zn
	}
	return len(v.dict)
}

// Rank returns member's 0-based rank, counted from the highest score when
// reverse is set, or -1 when it is not a member.
func (v *SortedSetValue) Rank(member string, reverse bool) int {
	score, ok := v.GetScore(member)
	if !ok {
		return -1
	}
	var r int
	if v.dict == nil {
		r, _ = v.smallPos(score, member)
	} else {
		r = v.ordered().rank(score, member) - 1
	}
	if reverse {
		return v.Card() - 1 - r
	}
	return r
}

// walk calls fn on the members from 0-based rank from onwards, or
// backwards with reverse, until fn returns false.
func (v *SortedSetValue) walk(from int, reverse bool, fn func(SortedEntry) bool) {
	n := v.Card()
	if from < 0 || from >= n {
		return
	}
	if v.dict == nil {
		if reverse {
			var offs [zsetSmallMax]int
			v.zlEach(func(i int, e zlEntry) bool {
				offs[i] = e.start
				return i < from
			})
			for i := from; i >= 0; i-- {
				if !fn(v.zlEntryOf(v.zlAt(offs[i]))) {
					return
				}
			}
			return
		}
		v.zlEach(func(i int, e zlEntry) bool {
			return i < from || fn(v.zlEntryOf(e))
		})
		return
	}
	x := v.ordered().byRank(from + 1)
	for x != nil {
		if !fn(SortedEntry{Member: x.member, Score: x.score}) {
			return
		}
		if reverse {
			x = x.backward
		} else {
			x = x.level[0].forward
		}
	}
}

// at returns the member at 0-based rank i, which is in range.
func (v *SortedSetValue) at(i int) SortedEntry {
	if v.dict == nil {
		var out SortedEntry
		v.zlEach(func(j int, e zlEntry) bool {
			if j == i {
				out = v.zlEntryOf(e)
				return false
			}
			return true
		})
		return out
	}
	x := v.ordered().byRank(i + 1)
	return SortedEntry{Member: x.member, Score: x.score}
}

// countBefore returns how many members satisfy before, which holds for a
// prefix of the order.
func (v *SortedSetValue) countBefore(before func(score float64, member string) bool) int {
	if v.dict == nil {
		if v.Skiplist() {
			// A set Redis holds as a skiplist answers a search by the
			// nodes' random levels, which matters when before is not
			// monotone (a lexicographic bound over mixed scores).
			z := newZskiplist()
			v.zlEach(func(_ int, e zlEntry) bool {
				se := v.zlEntryOf(e)
				z.insert(se.Score, se.Member)
				return true
			})
			return z.countBefore(func(n *zslNode) bool { return before(n.score, n.member) })
		}
		c := 0
		v.zlEach(func(_ int, e zlEntry) bool {
			se := v.zlEntryOf(e)
			if !before(se.Score, se.Member) {
				return false
			}
			c++
			return true
		})
		return c
	}
	return v.ordered().countBefore(func(n *zslNode) bool { return before(n.score, n.member) })
}

// RangeByRank returns the members at 0-based ranks lo..hi inclusive,
// counted from the highest score when reverse is set. The caller has
// clamped lo and hi to the set.
func (v *SortedSetValue) RangeByRank(lo, hi int, reverse bool) []SortedEntry {
	n := v.Card()
	if lo < 0 {
		lo = 0
	}
	if hi >= n {
		hi = n - 1
	}
	if lo > hi {
		return nil
	}
	out := make([]SortedEntry, 0, hi-lo+1)
	from := lo
	if reverse {
		from = n - 1 - lo
	}
	v.walk(from, reverse, func(e SortedEntry) bool {
		out = append(out, e)
		return len(out) < hi-lo+1
	})
	return out
}

// ScoreRanks returns the 0-based ranks first..last of the members whose
// scores are in the range; first > last when none is.
func (v *SortedSetValue) ScoreRanks(min float64, minEx bool, max float64, maxEx bool) (first, last int) {
	first = v.countBefore(func(score float64, _ string) bool {
		if minEx {
			return score <= min
		}
		return score < min
	})
	last = v.countBefore(func(score float64, _ string) bool {
		if maxEx {
			return score < max
		}
		return score <= max
	}) - 1
	return first, last
}

// LexBound is one end of a ZRANGEBYLEX interval: Inf is -1 for "-" and +1
// for "+", otherwise Val is the bound and Ex makes it exclusive.
type LexBound struct {
	Val string
	Ex  bool
	Inf int
}

func (b LexBound) gteMin(member string) bool {
	switch b.Inf {
	case -1:
		return true
	case 1:
		return false
	}
	if b.Ex {
		return member > b.Val
	}
	return member >= b.Val
}

func (b LexBound) lteMax(member string) bool {
	switch b.Inf {
	case -1:
		return false
	case 1:
		return true
	}
	if b.Ex {
		return member < b.Val
	}
	return member <= b.Val
}

// LexRange returns the members of a lexicographic range in Redis' order: it
// assumes all members share a score, and with mixed scores returns what
// Redis does — the run of members, in (score, member) order, that starts at
// the first one not below min and ends before the first one above max. With
// reverse it runs backwards from max. offset and limit (< 0 for all) apply
// in that order.
func (v *SortedSetValue) LexRange(min, max LexBound, reverse bool, offset, limit int) []SortedEntry {
	n := v.Card()
	if n == 0 || !min.gteMin(v.at(n-1).Member) || !max.lteMax(v.at(0).Member) {
		return nil
	}
	return v.lexRun(min, max, reverse, offset, limit)
}

// LexRemoveRange returns the members ZREMRANGEBYLEX removes: LexRange's, but
// a skiplist, as Redis' zslDeleteRangeByLex, skips the check that the range
// overlaps the set's first and last members.
func (v *SortedSetValue) LexRemoveRange(min, max LexBound) []SortedEntry {
	if !v.Skiplist() {
		return v.LexRange(min, max, false, 0, -1)
	}
	if v.Card() == 0 {
		return nil
	}
	return v.lexRun(min, max, false, 0, -1)
}

func (v *SortedSetValue) lexRun(min, max LexBound, reverse bool, offset, limit int) []SortedEntry {
	var out []SortedEntry
	emit := func(e SortedEntry) bool {
		if offset > 0 {
			offset--
			return true
		}
		if limit == 0 {
			return false
		}
		out = append(out, e)
		if limit > 0 {
			limit--
		}
		return true
	}
	if reverse {
		v.walk(v.lastNotAbove(max), true, func(e SortedEntry) bool {
			return min.gteMin(e.Member) && emit(e)
		})
		return out
	}
	v.walk(v.firstNotBelow(min), false, func(e SortedEntry) bool {
		return max.lteMax(e.Member) && emit(e)
	})
	return out
}

// CountLex is the number of members LexRange returns without a limit.
func (v *SortedSetValue) CountLex(min, max LexBound) int {
	n := v.Card()
	if n == 0 || !min.gteMin(v.at(n-1).Member) || !max.lteMax(v.at(0).Member) {
		return 0
	}
	if v.Skiplist() {
		first := v.countBefore(func(_ float64, m string) bool { return !min.gteMin(m) })
		last := v.countBefore(func(_ float64, m string) bool { return max.lteMax(m) })
		if last < first {
			return 0
		}
		return last - first
	}
	c := 0
	v.walk(v.firstNotBelow(min), false, func(e SortedEntry) bool {
		if !max.lteMax(e.Member) {
			return false
		}
		c++
		return true
	})
	return c
}

// firstNotBelow is the rank of the first member not below min, -1 when
// there is none: found by search in a skiplist and by a scan in a listpack,
// as Redis does.
func (v *SortedSetValue) firstNotBelow(min LexBound) int {
	if v.Skiplist() {
		return v.countBefore(func(_ float64, m string) bool { return !min.gteMin(m) })
	}
	r := -1
	i := 0
	v.walk(0, false, func(e SortedEntry) bool {
		if min.gteMin(e.Member) {
			r = i
			return false
		}
		i++
		return true
	})
	return r
}

func (v *SortedSetValue) lastNotAbove(max LexBound) int {
	if v.Skiplist() {
		return v.countBefore(func(_ float64, m string) bool { return max.lteMax(m) }) - 1
	}
	n := v.Card()
	r := -1
	i := n - 1
	v.walk(n-1, true, func(e SortedEntry) bool {
		if max.lteMax(e.Member) {
			r = i
			return false
		}
		i--
		return true
	})
	return r
}

// GetAllEntries returns every member in (score, member) order.
func (v *SortedSetValue) GetAllEntries() []SortedEntry {
	return v.RangeByRank(0, v.Card()-1, false)
}

// GetSortedRange returns the members at ranks start..stop, which may be
// negative to count from the end, as ZRANGE does.
func (v *SortedSetValue) GetSortedRange(start, stop int, _ bool, reverse bool) []SortedEntry {
	n := v.Card()
	if start < 0 {
		start += n
	}
	if stop < 0 {
		stop += n
	}
	if start < 0 {
		start = 0
	}
	if start > stop || start >= n {
		return nil
	}
	return v.RangeByRank(start, stop, reverse)
}

// Count returns the number of members with min <= score <= max.
func (v *SortedSetValue) Count(minScore, maxScore float64) int {
	first, last := v.ScoreRanks(minScore, false, maxScore, false)
	if last < first {
		return 0
	}
	return last - first + 1
}

// GetByScoreRange returns the members whose scores are in the range, in
// descending order when reverse is set.
func (v *SortedSetValue) GetByScoreRange(minScore float64, minExclusive bool, maxScore float64, maxExclusive bool, reverse bool) []SortedEntry {
	first, last := v.ScoreRanks(minScore, minExclusive, maxScore, maxExclusive)
	if last < first {
		return []SortedEntry{}
	}
	if reverse {
		n := v.Card()
		return v.RangeByRank(n-1-last, n-1-first, true)
	}
	return v.RangeByRank(first, last, false)
}

// RangeByScore is GetByScoreRange with an inclusive range.
func (v *SortedSetValue) RangeByScore(minScore, maxScore float64, _ bool, reverse bool) []SortedEntry {
	return v.GetByScoreRange(minScore, false, maxScore, false, reverse)
}

// RemoveRangeByRank removes the members at ranks start..stop, which may be
// negative to count from the end, and returns how many it removed.
func (v *SortedSetValue) RemoveRangeByRank(start, stop int) int {
	victims := v.GetSortedRange(start, stop, false, false)
	for _, e := range victims {
		v.Remove(e.Member)
	}
	return len(victims)
}

// RemoveRangeByScore removes the members with min <= score <= max.
func (v *SortedSetValue) RemoveRangeByScore(minScore, maxScore float64) int {
	victims := v.RangeByScore(minScore, maxScore, false, false)
	for _, e := range victims {
		v.Remove(e.Member)
	}
	return len(victims)
}

// GetByLexRange returns the members between minLex and maxLex (an empty
// bound is open), in (score, member) order or reversed.
func (v *SortedSetValue) GetByLexRange(minLex string, minExclusive bool, maxLex string, maxExclusive bool, reverse bool) []SortedEntry {
	out := []SortedEntry{}
	for _, e := range v.RangeByRank(0, v.Card()-1, reverse) {
		if minLex != "" && (e.Member < minLex || (minExclusive && e.Member == minLex)) {
			continue
		}
		if maxLex != "" && (e.Member > maxLex || (maxExclusive && e.Member == maxLex)) {
			continue
		}
		out = append(out, e)
	}
	return out
}

// RangeByLex returns the members within the ZRANGEBYLEX-style bounds min and
// max ("-", "+", "[x", "(x"), after offset, at most count when count > 0.
func (v *SortedSetValue) RangeByLex(min, max string, offset, count int, reverse bool) []string {
	var result []string
	for _, e := range v.RangeByRank(0, v.Card()-1, reverse) {
		if lexCompare(e.Member, min, max) {
			result = append(result, e.Member)
		}
	}
	if offset < 0 {
		offset = 0
	}
	if offset >= len(result) {
		return nil
	}
	if count <= 0 {
		return result[offset:]
	}
	end := len(result)
	if count < end-offset {
		end = offset + count
	}
	return result[offset:end]
}

// LexCount counts the members within the bounds min and max.
func (v *SortedSetValue) LexCount(min, max string) int {
	count := 0
	for member := range v.All() {
		if lexCompare(member, min, max) {
			count++
		}
	}
	return count
}

// RemoveRangeByLex removes the members within the bounds min and max.
func (v *SortedSetValue) RemoveRangeByLex(min, max string) int {
	removed := 0
	for _, e := range v.GetAllEntries() {
		if lexCompare(e.Member, min, max) {
			v.Remove(e.Member)
			removed++
		}
	}
	return removed
}

func lexCompare(member, min, max string) bool {
	minInclusive := true
	maxInclusive := true
	minVal := min
	maxVal := max
	minBounded := min != "" && min != "-"
	maxBounded := max != "" && max != "+"

	if len(min) > 0 {
		if min[0] == '[' {
			minInclusive = true
			minVal = min[1:]
		} else if min[0] == '(' {
			minInclusive = false
			minVal = min[1:]
		} else if min == "-" {
			minVal = ""
			minInclusive = true
		} else if min == "+" {
			return false
		}
	}

	if len(max) > 0 {
		if max[0] == '[' {
			maxInclusive = true
			maxVal = max[1:]
		} else if max[0] == '(' {
			maxInclusive = false
			maxVal = max[1:]
		} else if max == "+" {
			maxVal = string([]byte{0xFF})
			maxInclusive = true
		} else if max == "-" {
			return false
		}
	}

	if minBounded {
		if minInclusive {
			if member < minVal {
				return false
			}
		} else {
			if member <= minVal {
				return false
			}
		}
	}

	if maxBounded {
		if maxInclusive {
			if member > maxVal {
				return false
			}
		} else {
			if member >= maxVal {
				return false
			}
		}
	}

	return true
}
