package store

import (
	"hash/maphash"
	"math/bits"
)

// strSet is the set of keys a tag has. A Go map[string]struct{} spends
// about 55 bytes per member at the sizes tags have (its groups, control
// bytes and a table at most 7/8 full right after it doubles); this set keeps
// the 16-byte string headers in one slice, open-addressed with linear
// probing and grown by half, so a member costs 18 to 28 bytes. Up to
// strSetSmall members it is a plain slice searched in order, which is what
// most tags (one entity's few cache entries) are.
//
// The empty string is a key like any other; an empty slot is a string of
// length 0, so the empty key is recorded apart in hasEmpty.
//
// It is not safe for concurrent use: the tag shard's lock guards it.
type strSet struct {
	slots    []string
	n        int // members in slots
	hashed   bool
	hasEmpty bool
}

const strSetSmall = 8

var strSetSeed = maphash.MakeSeed()

func newStrSet() *strSet { return &strSet{} }

// Len returns the number of members.
func (s *strSet) Len() int {
	if s.hasEmpty {
		return s.n + 1
	}
	return s.n
}

func (s *strSet) home(key string) int {
	h := maphash.String(strSetSeed, key)
	hi, _ := bits.Mul64(h, uint64(len(s.slots)))
	return int(hi)
}

// find returns the slot of key and whether it is there; when it is not,
// the slot is where it would go (hashed sets only).
func (s *strSet) find(key string) (int, bool) {
	i := s.home(key)
	for {
		k := s.slots[i]
		if len(k) == 0 {
			return i, false
		}
		if k == key {
			return i, true
		}
		i++
		if i == len(s.slots) {
			i = 0
		}
	}
}

// Has reports whether key is a member.
func (s *strSet) Has(key string) bool {
	if key == "" {
		return s.hasEmpty
	}
	if !s.hashed {
		for _, k := range s.slots[:s.n] {
			if k == key {
				return true
			}
		}
		return false
	}
	_, ok := s.find(key)
	return ok
}

// Add adds key and reports whether it was new.
func (s *strSet) Add(key string) bool {
	if key == "" {
		if s.hasEmpty {
			return false
		}
		s.hasEmpty = true
		return true
	}
	if !s.hashed {
		for _, k := range s.slots[:s.n] {
			if k == key {
				return false
			}
		}
		if s.n < strSetSmall {
			s.slots = append(s.slots[:s.n], key)
			s.n++
			return true
		}
		s.rehash(strSetSmall * 2)
	} else if (s.n+1)*8 > len(s.slots)*7 {
		s.rehash(len(s.slots) + len(s.slots)/2)
	}
	i, ok := s.find(key)
	if ok {
		return false
	}
	s.slots[i] = key
	s.n++
	return true
}

// Remove removes key and reports whether it was a member.
func (s *strSet) Remove(key string) bool {
	if key == "" {
		if !s.hasEmpty {
			return false
		}
		s.hasEmpty = false
		return true
	}
	if !s.hashed {
		for i, k := range s.slots[:s.n] {
			if k == key {
				last := s.n - 1
				s.slots[i] = s.slots[last]
				s.slots[last] = ""
				s.n--
				return true
			}
		}
		return false
	}
	i, ok := s.find(key)
	if !ok {
		return false
	}
	// Backward-shift deletion: move later members of the probe run into
	// the gap when the gap lies between their home and their slot, so no
	// tombstones are needed.
	size := len(s.slots)
	j := i
	for {
		j++
		if j == size {
			j = 0
		}
		k := s.slots[j]
		if len(k) == 0 {
			break
		}
		// k moves into the gap when the gap is no further from k's home
		// than k's own slot, going round the table.
		h := s.home(k)
		if (j-h+size)%size >= (j-i+size)%size {
			s.slots[i] = k
			i = j
		}
	}
	s.slots[i] = ""
	s.n--
	if s.n <= strSetSmall/2 {
		s.toSmall()
	} else if s.n*4 < size && size > strSetSmall*2 {
		s.rehash(max(strSetSmall*2, s.n*2))
	}
	return true
}

// rehash moves the members to a hashed table of size slots.
func (s *strSet) rehash(size int) {
	old := s.slots
	if !s.hashed {
		old = old[:s.n]
	}
	s.slots = make([]string, size)
	s.hashed = true
	for _, k := range old {
		if len(k) == 0 {
			continue
		}
		i, _ := s.find(k)
		s.slots[i] = k
	}
}

// toSmall turns a hashed set with few members back into a plain slice.
func (s *strSet) toSmall() {
	small := make([]string, 0, strSetSmall)
	for _, k := range s.slots {
		if len(k) != 0 {
			small = append(small, k)
		}
	}
	s.slots, s.hashed = small, false
}

// Range calls fn with every member until it returns false.
func (s *strSet) Range(fn func(key string) bool) {
	if s.hasEmpty && !fn("") {
		return
	}
	slots := s.slots
	if !s.hashed {
		slots = slots[:s.n]
	}
	for _, k := range slots {
		if len(k) != 0 && !fn(k) {
			return
		}
	}
}

// AppendTo appends the members to out.
func (s *strSet) AppendTo(out []string) []string {
	if s.hasEmpty {
		out = append(out, "")
	}
	slots := s.slots
	if !s.hashed {
		return append(out, slots[:s.n]...)
	}
	for _, k := range slots {
		if len(k) != 0 {
			out = append(out, k)
		}
	}
	return out
}
