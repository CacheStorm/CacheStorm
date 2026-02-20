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

func (v *SortedSetValue) LexCount(min, max string) int {
	count := 0
	for member := range v.Members {
		if lexCompare(member, min, max) {
			count++
		}
	}
	return count
}

func (v *SortedSetValue) RangeByLex(min, max string, offset, count int, reverse bool) []string {
	entries := make(sortedEntries, 0, len(v.Members))
	for member, score := range v.Members {
		entries = append(entries, SortedEntry{Member: member, Score: score})
	}
	sort.Sort(entries)

	var result []string
	for _, e := range entries {
		if lexCompare(e.Member, min, max) {
			result = append(result, e.Member)
		}
	}

	if reverse {
		for i, j := 0, len(result)-1; i < j; i, j = i+1, j-1 {
			result[i], result[j] = result[j], result[i]
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
	end := offset + count
	if end > len(result) {
		end = len(result)
	}
	return result[offset:end]
}

func (v *SortedSetValue) RemoveRangeByLex(min, max string) int {
	removed := 0
	for member := range v.Members {
		if lexCompare(member, min, max) {
			delete(v.Members, member)
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

	if minVal != "" {
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

	if maxVal != "" && maxVal != string([]byte{0xFF}) {
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

func (v *SortedSetValue) GetAllEntries() []SortedEntry {
	entries := make(sortedEntries, 0, len(v.Members))
	for member, score := range v.Members {
		entries = append(entries, SortedEntry{Member: member, Score: score})
	}
	sort.Sort(entries)
	return entries
}
