package store

import (
	"encoding/binary"
	"iter"
	"slices"
	"sort"
	"strconv"
	"strings"
	"sync/atomic"
	"unsafe"
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

// setIntsMax, setLpMax and setLpBytes bound the compact forms: a set
// that Redis would still hold as an intset or listpack but that outgrows
// them moves to dict, which keeps its order.
const (
	setIntsMax = 512
	setLpMax   = 128
	setLpBytes = 16 << 10
)

// Card is the number of members. The caller holds the value's lock.
func (v *SetValue) Card() int {
	switch {
	case v.dict != nil:
		return len(v.dict)
	case v.enc == setEncIntset:
		return v.ints.Len()
	}
	return v.lpN
}

// Has reports whether member is in the set. The caller holds the value's
// lock.
func (v *SetValue) Has(member string) bool {
	switch {
	case v.dict != nil:
		_, ok := v.dict[member]
		return ok
	case v.enc == setEncIntset:
		_, ok := v.intIndex(member)
		return ok
	}
	_, _, ok := v.lpFind(member)
	return ok
}

func (v *SetValue) intIndex(member string) (int, bool) {
	n, ok := redisSetInt(member)
	if !ok {
		return 0, false
	}
	return v.ints.search(n)
}

// lpFind returns the bounds of member's entry in lp.
func (v *SetValue) lpFind(member string) (int, int, bool) {
	for off := 0; off < len(v.lp); {
		n, k := binary.Uvarint(v.lp[off:])
		s, e := off+k, off+k+int(n)
		if int(n) == len(member) && string(v.lp[s:e]) == member {
			return off, e, true
		}
		off = e
	}
	return 0, 0, false
}

// Range calls fn for every member, in Redis' order while the set is compact,
// until fn returns false. The caller holds the value's lock.
func (v *SetValue) Range(fn func(member string) bool) {
	switch {
	case v.dict != nil:
		if (v.enc == setEncIntset || v.enc == setEncListpack) && len(v.order) == len(v.dict) {
			for _, m := range v.order {
				if !fn(m) {
					return
				}
			}
			return
		}
		for m := range v.dict {
			if !fn(m) {
				return
			}
		}
	case v.enc == setEncIntset:
		for i := range v.ints.Len() {
			if !fn(strconv.FormatInt(v.ints.at(i), 10)) {
				return
			}
		}
	default:
		for off := 0; off < len(v.lp); {
			n, k := binary.Uvarint(v.lp[off:])
			s := off + k
			off = s + int(n)
			if n == 0 {
				if !fn("") {
					return
				}
				continue
			}
			if !fn(unsafe.String(&v.lp[s], int(n))) {
				return
			}
		}
	}
}

// RangeTransient is Range for a caller that does not keep a member past fn:
// an intset's members are formatted into one reused buffer instead of a
// string each.
func (v *SetValue) RangeTransient(fn func(member string) bool) {
	if v.dict != nil || v.enc != setEncIntset {
		v.Range(fn)
		return
	}
	var buf [20]byte
	for i := range v.ints.Len() {
		b := strconv.AppendInt(buf[:0], v.ints.at(i), 10)
		if !fn(unsafe.String(&b[0], len(b))) {
			return
		}
	}
}

// All yields the members as Range does. The caller holds the value's lock
// and does not change the set while iterating.
func (v *SetValue) All() iter.Seq[string] {
	return func(yield func(string) bool) { v.Range(yield) }
}

// Dict returns the map of a set held as one, nil for a compact set. The
// caller holds the value's lock and does not change the map.
func (v *SetValue) Dict() map[string]struct{} { return v.dict }

// Map returns a copy of the members.
func (v *SetValue) Map() map[string]struct{} {
	m := make(map[string]struct{}, v.Card())
	v.Range(func(k string) bool {
		m[k] = struct{}{}
		return true
	})
	return m
}

// NewSetFrom returns a set of the members whose encoding is not tracked.
func NewSetFrom(members map[string]struct{}) *SetValue {
	v := &SetValue{dict: make(map[string]struct{}, len(members))}
	v.adj.Store(setBaseSize - setCompactBase)
	for m := range members {
		v.dict[m] = struct{}{}
	}
	return v
}

// PrepareAdd is what SADD does before adding n members, the first of them
// first: a new set gets the encoding setTypeCreate picks, and a compact set
// too small for n members becomes a hash table. The caller holds the value's
// write lock.
func (v *SetValue) PrepareAdd(first string, n int) {
	if v.Card() == 0 {
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
	v.dict, v.ints, v.lp, v.lpN, v.order = nil, nil, nil, 0, nil
	v.adj.Store(0)
	switch _, isInt := redisSetInt(first); {
	case isInt && int64(n) <= setMaxIntsetEntries.Load():
		v.enc = setEncIntset
	case int64(n) <= setMaxListpackEntries.Load():
		v.enc = setEncListpack
	default:
		v.enc = setEncHashtable
		v.dict = make(map[string]struct{}, n)
		v.adj.Store(setBaseSize - setCompactBase)
	}
}

// NewIntsetSet returns an empty set encoded as an intset, as the sets Redis
// builds for SUNION, SDIFF and the STORE variants start out.
func NewIntsetSet() *SetValue {
	return &SetValue{enc: setEncIntset}
}

// NewListpackSet returns an empty set encoded as a listpack.
func NewListpackSet() *SetValue {
	return &SetValue{enc: setEncListpack}
}

// MaybeConvertToIntset makes a listpack set of integers that an intset
// can hold an intset, as Redis' maybeConvertToIntset. The caller holds the
// value's write lock.
func (v *SetValue) MaybeConvertToIntset() {
	if v.enc != setEncListpack || int64(v.Card()) > setMaxIntsetEntries.Load() {
		return
	}
	ints := make([]int64, 0, v.Card())
	all := true
	v.Range(func(m string) bool {
		n, ok := redisSetInt(m)
		ints = append(ints, n)
		all = ok
		return ok
	})
	if !all {
		return
	}
	slices.Sort(ints)
	v.ClearMembers()
	v.enc = setEncIntset
	for _, n := range ints {
		v.AddMember(strconv.FormatInt(n, 10))
	}
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

// toDict moves a compact set to dict, keeping its order while Redis would
// still hold it compact.
func (v *SetValue) toDict() {
	d := make(map[string]struct{}, v.Card()*2)
	keep := v.enc == setEncIntset || v.enc == setEncListpack
	var order []string
	v.Range(func(m string) bool {
		m = strings.Clone(m)
		d[m] = struct{}{}
		if keep {
			order = append(order, m)
		}
		return true
	})
	v.dict, v.order = d, order
	v.ints, v.lp, v.lpN = nil, nil, 0
	v.adj.Store(setBaseSize - setCompactBase)
}

func (v *SetValue) convertLocked() {
	if v.dict == nil {
		v.toDict()
	}
	v.enc, v.order = setEncHashtable, nil
}

// addNewLocked is setTypeAdd for a member not yet in the set.
func (v *SetValue) addNewLocked(member string) {
	if v.dict == nil && v.enc == setEncNone {
		v.create(member, 1)
	}
	if v.dict != nil {
		v.orderAddLocked(member)
		v.dict[member] = struct{}{}
		return
	}
	if v.enc == setEncIntset {
		if n, ok := redisSetInt(member); ok {
			i, _ := v.ints.search(n)
			v.ints = v.ints.insert(i, n)
			v.adj.Add(setIntsetElem - setMemberSize(member))
			if int64(v.ints.Len()) > setMaxIntsetEntries.Load() {
				v.convertLocked()
			} else if v.ints.Len() > setIntsMax {
				v.toDict()
			}
			return
		}
		if int64(v.ints.Len()) >= setMaxListpackEntries.Load() || int64(len(member)) > setMaxListpackValue.Load() {
			v.convertLocked()
			v.dict[member] = struct{}{}
			return
		}
		var buf []byte
		for i := range v.ints.Len() {
			buf = setLpAppend(buf, strconv.FormatInt(v.ints.at(i), 10))
		}
		v.enc, v.lp, v.lpN, v.ints = setEncListpack, buf, v.ints.Len(), nil
		v.adj.Store(int64(v.lpN) * (setListpackOverhd - setMemberOverhead))
	}
	if int64(v.lpN) >= setMaxListpackEntries.Load() || int64(len(member)) > setMaxListpackValue.Load() {
		v.convertLocked()
		v.dict[member] = struct{}{}
		return
	}
	// A new buffer of the exact size, as Redis reallocates a listpack;
	// members read from the old one stay valid.
	buf := make([]byte, len(v.lp), len(v.lp)+binary.MaxVarintLen32+len(member))
	copy(buf, v.lp)
	v.lp = setLpAppend(buf, member)
	v.lpN++
	v.adj.Add(setListpackOverhd - setMemberOverhead)
	if v.lpN > setLpMax || len(v.lp) > setLpBytes {
		v.toDict()
	}
}

func setLpAppend(buf []byte, member string) []byte {
	buf = binary.AppendUvarint(buf, uint64(len(member)))
	return append(buf, member...)
}

func (v *SetValue) removeLocked(member string) bool {
	switch {
	case v.dict != nil:
		if _, ok := v.dict[member]; !ok {
			return false
		}
		delete(v.dict, member)
		v.orderRemoveLocked(member)
	case v.enc == setEncIntset:
		i, ok := v.intIndex(member)
		if !ok {
			return false
		}
		v.ints = v.ints.remove(i)
		v.adj.Add(setMemberSize(member) - setIntsetElem)
	default:
		s, e, ok := v.lpFind(member)
		if !ok {
			return false
		}
		buf := make([]byte, 0, len(v.lp)-(e-s))
		buf = append(buf, v.lp[:s]...)
		v.lp = append(buf, v.lp[e:]...)
		v.lpN--
		v.adj.Add(setMemberOverhead - setListpackOverhd)
	}
	return true
}

// orderAddLocked is setTypeAdd's encoding step for a member not yet in a
// set kept in dict.
func (v *SetValue) orderAddLocked(member string) {
	if v.enc == setEncNone {
		if len(v.dict) > 0 {
			return
		}
		v.create(member, 1)
		if v.dict == nil {
			v.dict = make(map[string]struct{})
			v.adj.Store(setBaseSize - setCompactBase)
		}
	}
	switch v.enc {
	case setEncIntset:
		if n, ok := redisSetInt(member); ok {
			i := sort.Search(len(v.order), func(i int) bool {
				m, _ := redisSetInt(v.order[i])
				return m >= n
			})
			v.order = slices.Insert(v.order, i, member)
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

// SetListpackOrder makes the set a listpack of members, a saved listpack
// loaded verbatim, converting to a hash table when they do not fit. The
// caller holds the value's write lock.
func (v *SetValue) SetListpackOrder(members []string) {
	v.ClearMembers()
	v.enc = setEncListpack
	for _, m := range members {
		v.AddMember(m)
	}
}

// NewLoadedSet builds a set from the members of a saved set, as Redis'
// rdbLoad does: an intset, or a hash table when there are more members than
// an intset holds, converted as the members are added.
func NewLoadedSet(members []string) *SetValue {
	v := &SetValue{enc: setEncIntset}
	if int64(len(members)) > setMaxIntsetEntries.Load() {
		v.enc = setEncHashtable
		v.dict = make(map[string]struct{}, len(members))
		v.adj.Store(setBaseSize - setCompactBase)
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
	out := make([]string, 0, v.Card())
	v.Range(func(m string) bool {
		out = append(out, m)
		return true
	})
	if v.enc != setEncIntset && v.enc != setEncListpack {
		sort.Strings(out)
	}
	return out
}
