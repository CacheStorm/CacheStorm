package store

import (
	"math/bits"
	"sync"
)

// Redis 8's INFO keysizes: per database, how many strings, lists, sets,
// sorted sets and hashes fall in each power-of-two length bin. Bin 0 holds
// empty strings and bin i the lengths in [2^(i-1), 2^i).
//
// Each entry's bin rides in the top bits of its accounted word, next to the
// footprint the shard charged for it, so every place that charges or credits
// an entry moves it between bins under the same shard lock.

const (
	KeysizeTypes = 5
	KeysizeBins  = 50

	chargeBits = 52
	chargeMask = 1<<chargeBits - 1
)

// KeysizeHist counts keys per type (string, list, set, zset, hash) and bin.
type KeysizeHist [KeysizeTypes][KeysizeBins]int64

func (e *Entry) charged() int64 { return e.accounted.Load() & chargeMask }

func sizeTagOf(word int64) int64 { return word >> chargeBits }

// accountWord is what the shard records for entry under key: its footprint
// and its keysizes tag. A collection another goroutine holds locked keeps the
// tag it had; that writer reaccounts the key once it is done.
// held says the caller holds the value's lock.
func accountWord[K string | []byte](key K, entry *Entry, held bool) int64 {
	tag, ok := sizeTag(entry.Value, held)
	if !ok {
		tag = sizeTagOf(entry.accounted.Load())
	}
	return footprint(key, entry) | tag<<chargeBits
}

func lengthUnder(mu *sync.RWMutex, held bool, n func() int) (int, bool) {
	if held {
		return n(), true
	}
	if !mu.TryRLock() {
		return 0, false
	}
	defer mu.RUnlock()
	return n(), true
}

// subexpiryBit marks a hash with field expiries, which INFO keyspace counts
// as subexpiry.
const subexpiryBit = 1 << 9

// sizeTag is (type+1)<<6 | bin, with subexpiryBit for a hash with field
// expiries, or 0 for a key keysizes does not count.
func sizeTag(v Value, held bool) (int64, bool) {
	var typ, n int
	ok := true
	switch v := v.(type) {
	case *StringValue:
		n = len(v.Data)
	case CompactString:
		n = len(v.Bytes())
	case *HyperLogLogValue:
		n = hllDenseBytes
	case *ListValue:
		typ = 1
		n, ok = lengthUnder(&v.mu, held, v.Len)
	case *SetValue:
		typ = 2
		n, ok = lengthUnder(&v.mu, held, v.Card)
	case *SortedSetValue:
		typ = 3
		n, ok = lengthUnder(&v.mu, held, v.Card)
	case *GeoValue:
		typ = 3
		n, ok = lengthUnder(&v.mu, held, func() int { return len(v.Points) })
	case *HashValue:
		typ = 4
		n, ok = lengthUnder(&v.mu, held, v.Len)
	case StringLener:
		n, ok = v.StringLen(held)
	default:
		return 0, true
	}
	if !ok {
		return 0, false
	}
	if n == 0 && typ != 0 {
		return 0, true
	}
	bin := bits.Len64(uint64(n))
	if bin >= KeysizeBins {
		bin = KeysizeBins - 1
	}
	tag := int64(typ+1)<<6 | int64(bin)
	if h, isHash := v.(*HashValue); isHash && h.HasFieldTTLs() {
		tag |= subexpiryBit
	}
	return tag, true
}

// StringLener is a string value defined outside this package, such as a
// bitmap: StringLen reports its length in bytes, or false when it cannot
// take the value's read lock without waiting. held says the caller holds the
// value's lock.
type StringLener interface {
	StringLen(held bool) (int, bool)
}

// hllDenseBytes is the length of a dense HyperLogLog string.
const hllDenseBytes = 16 + HLLRegisterCount*6/8

// moveKeysize moves a key from the bin of tag from to that of tag to. The
// caller holds s.mu for writing.
func (s *Shard) moveKeysize(from, to int64) {
	if from == to {
		return
	}
	if from != 0 && s.keysizes != nil {
		s.keysizes[from>>6&7-1][from&63]--
	}
	if to != 0 {
		if s.keysizes == nil {
			s.keysizes = new(KeysizeHist)
		}
		s.keysizes[to>>6&7-1][to&63]++
	}
	s.subexpiry += to>>9&1 - from>>9&1
}

// Subexpiry returns how many of the store's hashes have field expiries.
func (s *Store) Subexpiry() int64 {
	var n int64
	for _, sh := range &s.shards {
		sh.mu.RLock()
		n += sh.subexpiry
		sh.mu.RUnlock()
	}
	return n
}

// Keysizes returns the store's keysizes histogram.
func (s *Store) Keysizes() KeysizeHist {
	var h KeysizeHist
	for _, sh := range &s.shards {
		sh.mu.RLock()
		if sh.keysizes != nil {
			for t := range sh.keysizes {
				for b, n := range sh.keysizes[t] {
					h[t][b] += n
				}
			}
		}
		sh.mu.RUnlock()
	}
	return h
}
