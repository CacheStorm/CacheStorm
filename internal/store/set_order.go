package store

import (
	"sort"
	"strconv"
	"sync/atomic"
)

// Redis keeps a small set of integers in an intset, sorted by value, and
// another small set in a listpack, in insertion order; a set that outgrows
// them becomes a hash table for good. SetValue follows the same conversions
// (setTypeCreate, setTypeAdd, setTypeMaybeConvert) so it lists members in
// Redis' order while the set is compact.
var (
	setMaxIntsetEntries   atomic.Int64
	setMaxListpackEntries atomic.Int64
	setMaxListpackValue   atomic.Int64
)

func init() {
	setMaxIntsetEntries.Store(512)
	setMaxListpackEntries.Store(128)
	setMaxListpackValue.Store(64)
}

// SetSetEncodingLimits sets set-max-intset-entries, set-max-listpack-entries
// and set-max-listpack-value.
func SetSetEncodingLimits(intset, entries, value int64) {
	setMaxIntsetEntries.Store(min(intset, 1<<30))
	setMaxListpackEntries.Store(entries)
	setMaxListpackValue.Store(value)
}

type setEncoding uint8

const (
	// setEncNone is a set whose encoding is not tracked: an empty one, or
	// one whose Members were filled directly.
	setEncNone setEncoding = iota
	setEncIntset
	setEncListpack
	setEncHashtable
)

// redisSetInt reports whether s is an integer Redis stores in an intset: the
// canonical decimal form of an int64.
func redisSetInt(s string) (int64, bool) {
	if s == "" || len(s) > 20 {
		return 0, false
	}
	n, err := strconv.ParseInt(s, 10, 64)
	if err != nil || strconv.FormatInt(n, 10) != s {
		return 0, false
	}
	return n, true
}

// Encoding returns the encoding Redis would hold the set in, or "" when it
// is not tracked. The caller holds the value's lock.
func (v *SetValue) Encoding() string {
	switch v.enc {
	case setEncIntset:
		return "intset"
	case setEncListpack:
		return "listpack"
	case setEncHashtable:
		return "hashtable"
	}
	return ""
}

// Range calls fn for every member, in Redis' order while the set is compact,
// until fn returns false. The caller holds the value's lock.
func (v *SetValue) Range(fn func(member string) bool) {
	if (v.enc == setEncIntset || v.enc == setEncListpack) && len(v.order) == len(v.Members) {
		for _, m := range v.order {
			if !fn(m) {
				return
			}
		}
		return
	}
	for m := range v.Members {
		if !fn(m) {
			return
		}
	}
}

// PrepareAdd is what SADD does before adding n members, the first of them
// first: a new set gets the encoding setTypeCreate picks, and a compact set
// too small for n members becomes a hash table. The caller holds the value's
// write lock.
func (v *SetValue) PrepareAdd(first string, n int) {
	if len(v.Members) == 0 {
		v.create(first, n)
		return
	}
	switch v.enc {
	case setEncListpack:
		if int64(n) > setMaxListpackEntries.Load() {
			v.convertLocked()
		}
	case setEncIntset:
		if int64(n) > setMaxIntsetEntries.Load() {
			v.convertLocked()
		}
	}
}

func (v *SetValue) create(first string, n int) {
	v.order = nil
	switch _, isInt := redisSetInt(first); {
	case isInt && int64(n) <= setMaxIntsetEntries.Load():
		v.enc = setEncIntset
	case int64(n) <= setMaxListpackEntries.Load():
		v.enc = setEncListpack
	default:
		v.enc = setEncHashtable
	}
}

// NewIntsetSet returns an empty set encoded as an intset, as the sets Redis
// builds for SUNION, SDIFF and the STORE variants start out.
func NewIntsetSet() *SetValue {
	return &SetValue{Members: make(map[string]struct{}), enc: setEncIntset}
}

// KeepOnly is the third strategy of Redis' SPOP with a count: the set is
// rebuilt as a new set holding keep, the members it keeps — an intset when it
// was one, otherwise a listpack filled in keep's order that converts as it
// grows. The caller holds the value's write lock.
func (v *SetValue) KeepOnly(keep []string) {
	enc := setEncListpack
	if v.enc == setEncIntset {
		enc = setEncIntset
	}
	v.ClearMembers()
	v.enc = enc
	for _, m := range keep {
		v.AddMember(m)
	}
}

func (v *SetValue) convertLocked() {
	v.enc = setEncHashtable
	v.order = nil
}

// orderAddLocked is setTypeAdd's encoding step for a member not yet in the
// set.
func (v *SetValue) orderAddLocked(member string) {
	if v.enc == setEncNone {
		if len(v.Members) > 0 {
			return
		}
		v.create(member, 1)
	}
	switch v.enc {
	case setEncIntset:
		if n, ok := redisSetInt(member); ok {
			i := sort.Search(len(v.order), func(i int) bool {
				m, _ := redisSetInt(v.order[i])
				return m >= n
			})
			v.order = append(v.order, "")
			copy(v.order[i+1:], v.order[i:])
			v.order[i] = member
			if int64(len(v.order)) > setMaxIntsetEntries.Load() {
				v.convertLocked()
			}
			return
		}
		if int64(len(v.order)) < setMaxListpackEntries.Load() && int64(len(member)) <= setMaxListpackValue.Load() {
			v.enc = setEncListpack
			v.order = append(v.order, member)
			return
		}
		v.convertLocked()
	case setEncListpack:
		if int64(len(v.order)) < setMaxListpackEntries.Load() && int64(len(member)) <= setMaxListpackValue.Load() {
			v.order = append(v.order, member)
			return
		}
		v.convertLocked()
	}
}

func (v *SetValue) orderRemoveLocked(member string) {
	if v.enc != setEncIntset && v.enc != setEncListpack {
		return
	}
	for i, m := range v.order {
		if m == member {
			v.order = append(v.order[:i:i], v.order[i+1:]...)
			return
		}
	}
}

// SetListpackOrder records members, every member of the set, as a listpack
// loaded verbatim, converting to a hash table when they do not fit. The
// caller holds the value's write lock.
func (v *SetValue) SetListpackOrder(members []string) {
	v.enc, v.order = setEncListpack, append([]string(nil), members...)
	if int64(len(members)) > setMaxListpackEntries.Load() {
		v.convertLocked()
		return
	}
	for _, m := range members {
		if int64(len(m)) > setMaxListpackValue.Load() {
			v.convertLocked()
			return
		}
	}
}

// NewLoadedSet builds a set from the members of a saved set, as Redis'
// rdbLoad does: an intset, or a hash table when there are more members than
// an intset holds, converted as the members are added.
func NewLoadedSet(members []string) *SetValue {
	v := &SetValue{Members: make(map[string]struct{}, len(members)), enc: setEncIntset}
	if int64(len(members)) > setMaxIntsetEntries.Load() {
		v.enc = setEncHashtable
	}
	for _, m := range members {
		v.AddMember(m)
	}
	return v
}

// SaveOrder returns the members in the order they are saved: Redis' order
// for a compact set, sorted for a hash table, whose own order is arbitrary.
// The caller holds the value's lock.
func (v *SetValue) SaveOrder() []string {
	out := make([]string, 0, len(v.Members))
	v.Range(func(m string) bool {
		out = append(out, m)
		return true
	})
	if v.enc != setEncIntset && v.enc != setEncListpack {
		sort.Strings(out)
	}
	return out
}
