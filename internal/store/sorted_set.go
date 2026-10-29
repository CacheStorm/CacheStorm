package store

import (
	"fmt"
	"sync"
	"sync/atomic"
)

// SortedSetValue keeps each member's score in Members and the members in
// (score, member) order in a skiplist. Code outside this file only fills
// Members directly while building a new value; the order is then derived on
// first use.
type SortedSetValue struct {
	Members map[string]float64
	mu      sync.RWMutex
	dead    bool
	size    sizeCounter
	index   atomic.Pointer[zskiplist]
	indexMu sync.Mutex
	scanCache
	big bool
}

func (v *SortedSetValue) Lock()    { v.mu.Lock() }
func (v *SortedSetValue) Unlock()  { v.mu.Unlock() }
func (v *SortedSetValue) RLock()   { v.mu.RLock() }
func (v *SortedSetValue) RUnlock() { v.mu.RUnlock() }

// IsDead reports whether DeleteIfEmpty removed this value from the keyspace
// (see ListValue.IsDead). The caller holds the value's lock.
func (v *SortedSetValue) IsDead() bool { return v.dead }

func (v *SortedSetValue) emptyLocked() bool { return len(v.Members) == 0 }
func (v *SortedSetValue) markDeadLocked()   { v.dead = true }

func (v *SortedSetValue) Type() DataType { return DataTypeSortedSet }
func (v *SortedSetValue) SizeOf() int64 {
	if n, ok := v.size.load(); ok {
		return zsetBaseSize + n
	}
	v.mu.RLock()
	defer v.mu.RUnlock()
	var n int64
	for k := range v.Members {
		n += zsetMemberSize(k)
	}
	v.size.set(n)
	return zsetBaseSize + n
}
func (v *SortedSetValue) String() string {
	v.mu.RLock()
	defer v.mu.RUnlock()
	result := ""
	for member, score := range v.Members {
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
	cloned := &SortedSetValue{Members: make(map[string]float64, len(v.Members)), big: v.big}
	for k, score := range v.Members {
		cloned.Members[k] = score
	}
	if z := v.index.Load(); z != nil && z.length == len(v.Members) {
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

// ordered returns the skiplist, building it from Members when it is missing
// or out of step. The caller holds the value's lock (a read lock is enough).
func (v *SortedSetValue) ordered() *zskiplist {
	if z := v.index.Load(); z != nil && z.length == len(v.Members) {
		return z
	}
	v.indexMu.Lock()
	defer v.indexMu.Unlock()
	if z := v.index.Load(); z != nil && z.length == len(v.Members) {
		return z
	}
	z := newZskiplist()
	for m, score := range v.Members {
		z.insert(score, m)
	}
	v.index.Store(z)
	return z
}

func (v *SortedSetValue) Remove(member string) bool {
	score, exists := v.Members[member]
	if !exists {
		return false
	}
	if z := v.index.Load(); z != nil {
		z.delete(score, member)
	}
	delete(v.Members, member)
	v.size.add(-zsetMemberSize(member))
	v.invalidateScan()
	return true
}

func (v *SortedSetValue) GetScore(member string) (float64, bool) {
	score, exists := v.Members[member]
	return score, exists
}

// Add sets member's score and reports whether the member is new. The caller
// holds the value's write lock.
func (v *SortedSetValue) Add(member string, score float64) bool {
	if v.Members == nil {
		v.Members = make(map[string]float64)
	}
	z := v.ordered()
	old, exists := v.Members[member]
	if exists && old == score {
		return false
	}
	if exists {
		z.delete(old, member)
	}
	z.insert(score, member)
	v.Members[member] = score
	if !exists {
		v.size.add(zsetMemberSize(member))
		v.invalidateScan()
		v.noteAddLocked(len(v.Members), len(member))
	}
	return !exists
}

func (v *SortedSetValue) Card() int {
	return len(v.Members)
}

// Rank returns member's 0-based rank, counted from the highest score when
// reverse is set, or -1 when it is not a member.
func (v *SortedSetValue) Rank(member string, reverse bool) int {
	score, ok := v.Members[member]
	if !ok {
		return -1
	}
	r := v.ordered().rank(score, member) - 1
	if reverse {
		return len(v.Members) - 1 - r
	}
	return r
}

// RangeByRank returns the members at 0-based ranks lo..hi inclusive,
// counted from the highest score when reverse is set. The caller has
// clamped lo and hi to the set.
func (v *SortedSetValue) RangeByRank(lo, hi int, reverse bool) []SortedEntry {
	n := len(v.Members)
	if lo < 0 {
		lo = 0
	}
	if hi >= n {
		hi = n - 1
	}
	if lo > hi {
		return nil
	}
	z := v.ordered()
	out := make([]SortedEntry, 0, hi-lo+1)
	if reverse {
		for x := z.byRank(n - lo); x != nil && len(out) < hi-lo+1; x = x.backward {
			out = append(out, SortedEntry{Member: x.member, Score: x.score})
		}
		return out
	}
	for x := z.byRank(lo + 1); x != nil && len(out) < hi-lo+1; x = x.level[0].forward {
		out = append(out, SortedEntry{Member: x.member, Score: x.score})
	}
	return out
}

// ScoreRanks returns the 0-based ranks first..last of the members whose
// scores are in the range; first > last when none is.
func (v *SortedSetValue) ScoreRanks(min float64, minEx bool, max float64, maxEx bool) (first, last int) {
	z := v.ordered()
	first = z.countBefore(func(n *zslNode) bool {
		if minEx {
			return n.score <= min
		}
		return n.score < min
	})
	last = z.countBefore(func(n *zslNode) bool {
		if maxEx {
			return n.score < max
		}
		return n.score <= max
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
	z := v.ordered()
	if z.length == 0 || !min.gteMin(z.tail.member) || !max.lteMax(z.first().member) {
		return nil
	}
	var out []SortedEntry
	emit := func(x *zslNode) bool {
		if offset > 0 {
			offset--
			return true
		}
		if limit == 0 {
			return false
		}
		out = append(out, SortedEntry{Member: x.member, Score: x.score})
		if limit > 0 {
			limit--
		}
		return true
	}
	if reverse {
		x := v.lastNotAbove(z, max)
		for ; x != nil && min.gteMin(x.member); x = x.backward {
			if !emit(x) {
				break
			}
		}
		return out
	}
	x := v.firstNotBelow(z, min)
	for ; x != nil && max.lteMax(x.member); x = x.level[0].forward {
		if !emit(x) {
			break
		}
	}
	return out
}

// CountLex is the number of members LexRange returns without a limit.
func (v *SortedSetValue) CountLex(min, max LexBound) int {
	z := v.ordered()
	if z.length == 0 || !min.gteMin(z.tail.member) || !max.lteMax(z.first().member) {
		return 0
	}
	if v.Skiplist() {
		first := z.countBefore(func(n *zslNode) bool { return !min.gteMin(n.member) })
		last := z.countBefore(func(n *zslNode) bool { return max.lteMax(n.member) })
		if last < first {
			return 0
		}
		return last - first
	}
	n := 0
	for x := v.firstNotBelow(z, min); x != nil && max.lteMax(x.member); x = x.level[0].forward {
		n++
	}
	return n
}

func (v *SortedSetValue) firstNotBelow(z *zskiplist, min LexBound) *zslNode {
	if v.Skiplist() {
		return z.byRank(z.countBefore(func(n *zslNode) bool { return !min.gteMin(n.member) }) + 1)
	}
	for x := z.first(); x != nil; x = x.level[0].forward {
		if min.gteMin(x.member) {
			return x
		}
	}
	return nil
}

func (v *SortedSetValue) lastNotAbove(z *zskiplist, max LexBound) *zslNode {
	if v.Skiplist() {
		return z.byRank(z.countBefore(func(n *zslNode) bool { return max.lteMax(n.member) }))
	}
	for x := z.tail; x != nil; x = x.backward {
		if max.lteMax(x.member) {
			return x
		}
	}
	return nil
}

// GetAllEntries returns every member in (score, member) order.
func (v *SortedSetValue) GetAllEntries() []SortedEntry {
	return v.RangeByRank(0, len(v.Members)-1, false)
}

// GetSortedRange returns the members at ranks start..stop, which may be
// negative to count from the end, as ZRANGE does.
func (v *SortedSetValue) GetSortedRange(start, stop int, _ bool, reverse bool) []SortedEntry {
	n := len(v.Members)
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
		n := len(v.Members)
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
	for _, e := range v.RangeByRank(0, len(v.Members)-1, reverse) {
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
	for _, e := range v.RangeByRank(0, len(v.Members)-1, reverse) {
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
	for member := range v.Members {
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
