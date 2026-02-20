package store

import (
	"fmt"
	"sort"
	"sync"
)

type SortedSetValue struct {
	Members map[string]float64
	mu      sync.RWMutex
}

func (v *SortedSetValue) Lock()    { v.mu.Lock() }
func (v *SortedSetValue) Unlock()  { v.mu.Unlock() }
func (v *SortedSetValue) RLock()   { v.mu.RLock() }
func (v *SortedSetValue) RUnlock() { v.mu.RUnlock() }

func (v *SortedSetValue) Type() DataType { return DataTypeSortedSet }
func (v *SortedSetValue) SizeOf() int64 {
	v.mu.RLock()
	defer v.mu.RUnlock()
	var size int64 = 48
	for k := range v.Members {
		size += int64(len(k)) + 16 + 80
	}
	return size
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
	cloned := &SortedSetValue{Members: make(map[string]float64, len(v.Members))}
	for k, score := range v.Members {
		cloned.Members[k] = score
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

func (v *SortedSetValue) GetSortedRange(start, stop int, _ bool, reverse bool) []SortedEntry {
	entries := make(sortedEntries, 0, len(v.Members))
	for member, score := range v.Members {
		entries = append(entries, SortedEntry{Member: member, Score: score})
	}

	if reverse {
		sort.Sort(sort.Reverse(entries))
	} else {
		sort.Sort(entries)
	}

	n := len(entries)
	if start < 0 {
		start = n + start
	}
	if stop < 0 {
		stop = n + stop
	}
	if start < 0 {
		start = 0
	}
	if stop >= n {
		stop = n - 1
	}
	if start > stop || start >= n {
		return nil
	}

	return entries[start : stop+1]
}

func (v *SortedSetValue) Rank(member string, reverse bool) int {
	entries := make(sortedEntries, 0, len(v.Members))
	for m, score := range v.Members {
		entries = append(entries, SortedEntry{Member: m, Score: score})
	}

	if reverse {
		sort.Sort(sort.Reverse(entries))
	} else {
		sort.Sort(entries)
	}

	for i, e := range entries {
		if e.Member == member {
			return i
		}
	}
	return -1
}

func (v *SortedSetValue) Count(minScore, maxScore float64) int {
	count := 0
	for _, score := range v.Members {
		if score >= minScore && score <= maxScore {
			count++
		}
	}
	return count
}

func (v *SortedSetValue) RangeByScore(minScore, maxScore float64, _ bool, reverse bool) []SortedEntry {
	entries := make(sortedEntries, 0)
	for member, score := range v.Members {
		if score >= minScore && score <= maxScore {
			entries = append(entries, SortedEntry{Member: member, Score: score})
		}
	}

	if reverse {
		sort.Sort(sort.Reverse(entries))
	} else {
		sort.Sort(entries)
	}

	return entries
}

func (v *SortedSetValue) RemoveRangeByRank(start, stop int) int {
	entries := make(sortedEntries, 0, len(v.Members))
	for member, score := range v.Members {
		entries = append(entries, SortedEntry{Member: member, Score: score})
	}
	sort.Sort(entries)

	n := len(entries)
	if start < 0 {
		start = n + start
	}
	if stop < 0 {
		stop = n + stop
	}
	if start < 0 {
		start = 0
	}
	if stop >= n {
		stop = n - 1
	}
	if start > stop || start >= n {
		return 0
	}

	removed := 0
	for i := start; i <= stop; i++ {
		delete(v.Members, entries[i].Member)
		removed++
	}
	return removed
}

func (v *SortedSetValue) RemoveRangeByScore(minScore, maxScore float64) int {
	removed := 0
	for member, score := range v.Members {
		if score >= minScore && score <= maxScore {
			delete(v.Members, member)
			removed++
		}
	}
	return removed
}

func (v *SortedSetValue) Remove(member string) bool {
	if _, exists := v.Members[member]; exists {
		delete(v.Members, member)
		return true
	}
	return false
}

func (v *SortedSetValue) GetScore(member string) (float64, bool) {
	score, exists := v.Members[member]
	return score, exists
}

func (v *SortedSetValue) Add(member string, score float64) bool {
	_, exists := v.Members[member]
	v.Members[member] = score
	return !exists
}

func (v *SortedSetValue) Card() int {
	return len(v.Members)
}
