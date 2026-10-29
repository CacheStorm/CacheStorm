package store

import (
	"sort"
)

// A SCAN page over a shard returns the keys whose scan hash is the smallest
// at or after the cursor. Finding them by visiting every key made a page cost
// the whole shard, so a full SCAN of n keys cost n²/NumShards. A shard being
// scanned keeps its keys sorted by hash instead, built once and kept up to
// date by recording the keys added since, so a page costs its own keys plus
// the additions: a key present for the whole scan is in one or the other.
// The index is dropped when a page reaches the end of the shard, so only the
// shards scans are in hold one.

// ScanPosBits is how many bits of a SCAN cursor are the position within a
// shard; the bits above name the shard.
const ScanPosBits = 56

// ScanHash is a key's position in a SCAN of its shard.
func ScanHash(key string) uint64 {
	h := uint64(14695981039346656037)
	for i := 0; i < len(key); i++ {
		h ^= uint64(key[i])
		h *= 1099511628211
	}
	return h >> (64 - ScanPosBits)
}

type scanItem struct {
	hash uint64
	key  string
}

type keyScanIndex struct {
	// sorted is nil while it is being built.
	sorted []scanItem
	added  []string
}

// scanIndexMinAdds is how many additions an index records before it is
// dropped for having more additions than keys.
const scanIndexMinAdds = 1024

// noteAddLocked records a key new to the shard. The caller holds s.mu.
func (s *Shard) noteAddLocked(key string) {
	if s.scan == nil {
		return
	}
	s.scan.added = append(s.scan.added, key)
	if len(s.scan.added) > max(len(s.scan.sorted), scanIndexMinAdds) {
		s.scan = nil
	}
}

// ScanShard returns the keys of shard idx whose scan hash is at least pos,
// at least count of them unless the shard ends first, and the position the
// next page starts at, 0 when the shard is done. With withEntries the
// entries are returned too.
func (s *Store) ScanShard(idx int, pos uint64, count int, withEntries bool) ([]string, []*Entry, uint64) {
	sh := s.shards[idx]
	sh.mu.Lock()
	if len(sh.data) <= 2*count {
		defer sh.mu.Unlock()
		return sh.scanAllLocked(pos, withEntries)
	}
	if sh.scan == nil {
		index := &keyScanIndex{}
		sh.scan = index
		items := make([]scanItem, 0, len(sh.data))
		for k := range sh.data {
			items = append(items, scanItem{ScanHash(k), k})
		}
		sh.mu.Unlock()
		sort.Slice(items, func(i, j int) bool {
			if items[i].hash != items[j].hash {
				return items[i].hash < items[j].hash
			}
			return items[i].key < items[j].key
		})
		sh.mu.Lock()
		if sh.scan == index {
			index.sorted = items
		}
	}
	defer sh.mu.Unlock()
	if sh.scan == nil || sh.scan.sorted == nil {
		return sh.scanAllLocked(pos, withEntries)
	}
	keys, entries, next := sh.scanIndexedLocked(pos, count, withEntries)
	if next == 0 {
		sh.scan = nil
	}
	return keys, entries, next
}

func (sh *Shard) live(key string) (*Entry, bool) {
	e, ok := sh.data[key]
	if !ok || e.IsExpired() {
		return nil, false
	}
	return e, true
}

// scanAllLocked returns every key at or after pos, for a small shard or one
// whose index is still being built.
func (sh *Shard) scanAllLocked(pos uint64, withEntries bool) ([]string, []*Entry, uint64) {
	var keys []string
	var entries []*Entry
	for k, e := range sh.data {
		if e.IsExpired() || ScanHash(k) < pos {
			continue
		}
		keys = append(keys, k)
		if withEntries {
			entries = append(entries, e)
		}
	}
	return keys, entries, 0
}

func (sh *Shard) scanIndexedLocked(pos uint64, count int, withEntries bool) ([]string, []*Entry, uint64) {
	sorted, added := sh.scan.sorted, sh.scan.added
	var keys []string
	var entries []*Entry
	emit := func(k string, e *Entry) {
		keys = append(keys, k)
		if withEntries {
			entries = append(entries, e)
		}
	}
	i := sort.Search(len(sorted), func(i int) bool { return sorted[i].hash >= pos })
	limit := ^uint64(0)
	found := 0
	for ; i < len(sorted); i++ {
		it := sorted[i]
		if found >= count && it.hash != limit {
			break
		}
		if e, ok := sh.live(it.key); ok {
			emit(it.key, e)
			found++
			if found == count {
				limit = it.hash
			}
		}
	}
	if found < count {
		limit = ^uint64(0)
	}
	if len(added) > 0 {
		seen := make(map[string]struct{}, len(keys))
		for _, k := range keys {
			seen[k] = struct{}{}
		}
		for _, k := range added {
			if h := ScanHash(k); h < pos || h > limit {
				continue
			}
			if _, dup := seen[k]; dup {
				continue
			}
			if e, ok := sh.live(k); ok {
				seen[k] = struct{}{}
				emit(k, e)
			}
		}
	}
	if limit >= 1<<ScanPosBits-1 {
		return keys, entries, 0
	}
	if i >= len(sorted) {
		for _, k := range added {
			if ScanHash(k) > limit {
				return keys, entries, limit + 1
			}
		}
		return keys, entries, 0
	}
	return keys, entries, limit + 1
}
