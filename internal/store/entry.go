package store

import (
	"strings"
	"sync"
	"sync/atomic"
	"time"
)

type DataType uint8

const (
	DataTypeString DataType = iota + 1
	DataTypeHash
	DataTypeList
	DataTypeSet
	DataTypeSortedSet
	DataTypeStream
	DataTypeGeo
	DataTypeJSON
	DataTypeTimeSeries
	DataTypeBloom
	DataTypeCuckoo
	DataTypeCMS
	DataTypeTopK
	DataTypeTDigest
	DataTypeSuggest
)

func (dt DataType) String() string {
	switch dt {
	case DataTypeString:
		return "string"
	case DataTypeHash:
		return "hash"
	case DataTypeList:
		return "list"
	case DataTypeSet:
		return "set"
	case DataTypeSortedSet:
		return "zset"
	case DataTypeStream:
		return "stream"
	case DataTypeGeo:
		return "geo"
	case DataTypeJSON:
		return "ReJSON-RL"
	case DataTypeTimeSeries:
		return "TSDB-TYPE"
	case DataTypeBloom:
		return "MBbloom--"
	case DataTypeCuckoo:
		return "MBbloomCF"
	case DataTypeCMS:
		return "CMSk-TYPE"
	case DataTypeTopK:
		return "TopK-TYPE"
	case DataTypeTDigest:
		return "TDIS-TYPE"
	case DataTypeSuggest:
		return "trietype0"
	default:
		return "unknown"
	}
}

type Value interface {
	Type() DataType
	SizeOf() int64
	Clone() Value
	String() string
}

type StringValue struct {
	Data []byte
	Enc  StringEncoding
}

// StringEncoding records how a command built a string, for OBJECT ENCODING:
// Redis stores a value it was given as an int or embstr when it can, but a
// string changed in place is raw and INCRBYFLOAT's result is never an int.
type StringEncoding uint8

const (
	StringEncAuto StringEncoding = iota
	StringEncRaw
	StringEncNoInt
)

func (v *StringValue) Type() DataType { return DataTypeString }
func (v *StringValue) SizeOf() int64  { return int64(len(v.Data)) + 24 }
func (v *StringValue) String() string { return string(v.Data) }
func (v *StringValue) Clone() Value {
	cloned := make([]byte, len(v.Data))
	copy(cloned, v.Data)
	return &StringValue{Data: cloned, Enc: v.Enc}
}

type HashValue struct {
	lp   []byte
	dict map[string][]byte
	// order lists the fields in Redis' listpack order while big is false
	// (see hash_order.go).
	order []string
	mu    sync.RWMutex
	size  sizeCounter
	// adj corrects size, counted at the map form's rates, to the form the
	// hash is in.
	adj atomic.Int64
	// minExpire is the earliest field expiry (unix ms), 0 when no field has
	// one, readable without the lock.
	minExpire atomic.Int64
	// ttl holds the field expiries; nil until a field is first given one.
	ttl *hashFieldTTLs
	scanCache
	// The small fields share one word: lp's field count, whether the hash
	// is past the listpack limits, and whether it was deleted.
	lpN  int32
	big  bool
	dead bool
}

func (v *HashValue) Lock()    { v.mu.Lock() }
func (v *HashValue) Unlock()  { v.mu.Unlock() }
func (v *HashValue) RLock()   { v.mu.RLock() }
func (v *HashValue) RUnlock() { v.mu.RUnlock() }

// IsDead reports whether DeleteIfEmpty removed this value from the keyspace.
// A dead value is empty and detached: writers must not add to it. The caller
// holds the value's lock.
func (v *HashValue) IsDead() bool { return v.dead }

func (v *HashValue) emptyLocked() bool { return v.Len() == 0 }
func (v *HashValue) markDeadLocked()   { v.dead = true }

func (v *HashValue) Type() DataType { return DataTypeHash }
func (v *HashValue) SizeOf() int64 {
	if n, ok := v.size.load(); ok {
		return hashPackedBase + n + v.adj.Load()
	}
	v.mu.RLock()
	defer v.mu.RUnlock()
	var n int64
	v.Range(func(k string, val []byte) bool {
		n += hashFieldSize(k, val)
		return true
	})
	n += v.ttlSizeLocked()
	v.size.set(n)
	return hashPackedBase + n + v.adj.Load()
}

// SetField stores val under field, dropping any expiry the field had, as
// HSET does, and reports whether the field is new. The caller holds the
// value's write lock.
func (v *HashValue) SetField(field string, val []byte) bool {
	hadTTL := v.FieldExpireAt(field) != 0
	v.clearFieldTTL(field)
	added := v.SetFieldKeepTTL(field, val)
	if hadTTL && !added {
		v.orderPlaceLocked(field, 0)
	}
	return added
}

// SetFieldKeepTTL stores val under field keeping the field's expiry, as
// HINCRBY does, and reports whether the field is new. The caller holds the
// value's write lock.
func (v *HashValue) SetFieldKeepTTL(field string, val []byte) bool {
	old, exists := v.put(field, val)
	v.noteLengthsLocked(field, val)
	if exists {
		v.size.add(int64(len(val)) - int64(len(old)))
	} else {
		v.size.add(hashFieldSize(field, val))
		v.invalidateScan()
	}
	return !exists
}

// DeleteField removes field and reports whether it existed. The caller holds
// the value's write lock.
func (v *HashValue) DeleteField(field string) bool {
	old, exists := v.del(field)
	if !exists {
		return false
	}
	v.size.add(-hashFieldSize(field, old))
	v.clearFieldTTL(field)
	v.invalidateScan()
	return true
}
func (v *HashValue) String() string {
	v.mu.RLock()
	defer v.mu.RUnlock()
	result := ""
	for k, val := range v.All() {
		if result != "" {
			result += ", "
		}
		result += k + ": " + string(val)
	}
	return result
}
func (v *HashValue) Clone() Value {
	v.mu.RLock()
	defer v.mu.RUnlock()
	cloned := &HashValue{big: v.big}
	cloned.adj.Store(v.adj.Load())
	if v.dict == nil {
		cloned.lp, cloned.lpN = append([]byte(nil), v.lp...), v.lpN
	} else {
		cloned.dict = make(map[string][]byte, len(v.dict))
		for k, val := range v.dict {
			cloned.dict[k] = append([]byte(nil), val...)
		}
	}
	if v.order != nil {
		cloned.order = append([]string(nil), v.order...)
	}
	v.cloneTTLsLocked(cloned)
	return cloned
}

type ListValue struct {
	l    listChunks
	mu   sync.RWMutex
	dead bool
	size sizeCounter
	// shape is how Redis would hold the list, for OBJECT ENCODING; nil
	// until a write needs it (see list_encoding.go).
	shape *listShape
}

// NewListFrom returns a list of elems.
func NewListFrom(elems [][]byte) *ListValue {
	v := &ListValue{}
	for _, el := range elems {
		v.l.pushBack(el)
	}
	return v
}

// AppendLoaded appends el to a list being built, leaving its encoding to
// be derived from the whole list.
func (v *ListValue) AppendLoaded(el []byte) {
	v.l.pushBack(el)
	v.size.add(listElemSize(el))
}

// Len is the number of elements. The caller holds the value's lock.
func (v *ListValue) Len() int { return v.l.count }

// At returns element i, which is in range. The caller holds the value's
// lock.
func (v *ListValue) At(i int) []byte { return v.l.at(i) }

// Range calls fn on the elements from index from on, or from there
// backwards with reverse, until fn returns false. The caller holds the
// value's lock.
func (v *ListValue) Range(from int, reverse bool, fn func(i int, el []byte) bool) {
	v.l.each(from, reverse, fn)
}

// Elements returns the elements. The caller holds the value's lock; the
// slices stay valid after the list changes.
func (v *ListValue) Elements() [][]byte { return v.l.all() }

// Slice returns elements [start, end). The caller holds the value's lock.
func (v *ListValue) Slice(start, end int) [][]byte {
	if start >= end {
		return nil
	}
	return v.l.slice(start, end)
}

// PushFront prepends items so that items[0] becomes the new head, as LPUSH
// of the items in reverse order does. The caller holds the value's lock.
func (v *ListValue) PushFront(items ...[]byte) {
	if len(items) == 0 {
		return
	}
	s := v.shapeLocked()
	s.grow(argBytes(items), len(items))
	for i := len(items) - 1; i >= 0; i-- {
		s.push(items[i], true)
	}
	v.pushElems(true, items...)
}

func (v *ListValue) pushElems(front bool, items ...[]byte) {
	v.size.add(sumListElems(items))
	if !front {
		for _, el := range items {
			v.l.pushBack(el)
		}
		return
	}
	for i := len(items) - 1; i >= 0; i-- {
		v.l.pushFront(items[i])
	}
}

// PopFront removes and returns up to n elements from the head. The caller
// holds the value's lock.
func (v *ListValue) PopFront(n int) [][]byte {
	return v.pop(n, true)
}

func (v *ListValue) pop(n int, front bool) [][]byte {
	if n <= 0 || v.l.count == 0 {
		return nil
	}
	s := v.shapeLocked()
	out := v.popElems(n, front)
	if front {
		s.popFront(out)
	} else {
		s.popBack(out)
	}
	if v.l.count > 0 {
		s.toListpack(true)
	}
	return out
}

func (v *ListValue) popElems(n int, front bool) [][]byte {
	n = min(n, v.l.count)
	if n <= 0 {
		return nil
	}
	out := make([][]byte, n)
	for i := range out {
		if front {
			out[i] = v.l.popFront()
		} else {
			out[i] = v.l.popBack()
		}
	}
	v.size.add(-sumListElems(out))
	return out
}

// PopBack removes and returns up to n elements from the tail, last element
// first. The caller holds the value's lock.
func (v *ListValue) PopBack(n int) [][]byte {
	return v.pop(n, false)
}

func (v *ListValue) Lock()    { v.mu.Lock() }
func (v *ListValue) Unlock()  { v.mu.Unlock() }
func (v *ListValue) RLock()   { v.mu.RLock() }
func (v *ListValue) RUnlock() { v.mu.RUnlock() }

// IsDead reports whether DeleteIfEmpty removed this value from the keyspace.
// A dead value is empty and detached: writers must not add to it. The caller
// holds the value's lock.
func (v *ListValue) IsDead() bool { return v.dead }

func (v *ListValue) emptyLocked() bool { return v.l.count == 0 }
func (v *ListValue) markDeadLocked()   { v.dead = true }

func (v *ListValue) Type() DataType { return DataTypeList }
func (v *ListValue) SizeOf() int64 {
	if n, ok := v.size.load(); ok {
		return listBaseSize + n
	}
	v.mu.RLock()
	defer v.mu.RUnlock()
	var n int64
	v.l.each(0, false, func(_ int, el []byte) bool {
		n += listElemSize(el)
		return true
	})
	v.size.set(n)
	return listBaseSize + n
}

func sumListElems(elems [][]byte) int64 {
	var n int64
	for _, el := range elems {
		n += listElemSize(el)
	}
	return n
}

// PushBack appends items, as RPUSH does. The caller holds the value's write
// lock.
func (v *ListValue) PushBack(items ...[]byte) {
	if len(items) == 0 {
		return
	}
	s := v.shapeLocked()
	s.grow(argBytes(items), len(items))
	for _, el := range items {
		s.push(el, false)
	}
	v.pushElems(false, items...)
}

// SetAt replaces the element at index i, which must be in range, as LSET
// does after TryGrow. The caller holds the value's write lock.
func (v *ListValue) SetAt(i int, el []byte) {
	s := v.shapeLocked()
	ci, off := v.l.locate(i)
	c := v.l.chunks[ci]
	_, ds, de, _ := c.entry(off)
	old := c.elem(ds, de)
	s.replace(knownElem{&v.l, i, old}, i, el)
	s.toListpack(true)
	v.l.setAt(ci, off, el)
	v.size.add(int64(len(el)) - int64(len(old)))
}

// knownElem is a list's elements with element i already looked up.
type knownElem struct {
	l   *listChunks
	i   int
	old []byte
}

func (k knownElem) at(i int) []byte {
	if i == k.i {
		return k.old
	}
	return k.l.at(i)
}

func (k knownElem) slice(start, end int) [][]byte { return k.l.slice(start, end) }

// Trim keeps only the elements in [start, stop], which must be valid indexes
// with start <= stop, as LTRIM does. It costs O(removed elements). The caller
// holds the value's write lock.
func (v *ListValue) Trim(start, stop int) {
	s := v.shapeLocked()
	front := v.l.slice(0, start)
	s.popFront(front)
	removed := sumListElems(front)
	v.l.each(v.l.count-1, true, func(i int, el []byte) bool {
		if i <= stop {
			return false
		}
		s.popBack([][]byte{el})
		removed += listElemSize(el)
		return true
	})
	s.toListpack(true)
	v.l.trim(start, stop)
	v.size.add(-removed)
}

// ReplaceElements installs elems as the whole list, for the commands that
// rebuild it. The caller holds the value's write lock.
func (v *ListValue) ReplaceElements(elems [][]byte) {
	v.l.rebuild(elems)
	v.shape = nil
	v.size.set(sumListElems(elems))
}
func (v *ListValue) String() string {
	v.mu.RLock()
	defer v.mu.RUnlock()
	var result strings.Builder
	v.l.each(0, false, func(i int, el []byte) bool {
		if i > 0 {
			result.WriteString(", ")
		}
		result.Write(el)
		return true
	})
	return result.String()
}
func (v *ListValue) Clone() Value {
	v.mu.RLock()
	defer v.mu.RUnlock()
	cloned := &ListValue{l: v.l.clone()}
	if v.shape != nil && v.shape.count == v.l.count {
		cloned.shape = v.shape.clone()
	}
	return cloned
}

type SetValue struct {
	// dict holds a set in the hashtable form; a set Redis would hold as an
	// intset or listpack, and small enough, is in lp instead: the packed
	// intset bytes (see set_intset.go) or the listpack (see set_order.go).
	dict map[string]struct{}
	lp   []byte
	// order is, for an intset or listpack kept in dict, its member order.
	order []string
	mu    sync.RWMutex
	size  sizeCounter
	// adj corrects size, counted at the map form's rates, to the form the
	// set is in.
	adj atomic.Int64
	scanCache
	// The small fields share one word: lp's member count, the encoding
	// Redis would hold the set in, and whether the set was deleted.
	lpN  int32
	enc  setEncoding
	dead bool
}

func (v *SetValue) Lock()    { v.mu.Lock() }
func (v *SetValue) Unlock()  { v.mu.Unlock() }
func (v *SetValue) RLock()   { v.mu.RLock() }
func (v *SetValue) RUnlock() { v.mu.RUnlock() }

// IsDead reports whether DeleteIfEmpty removed this value from the keyspace.
// A dead value is empty and detached: writers must not add to it. The caller
// holds the value's lock.
func (v *SetValue) IsDead() bool { return v.dead }

func (v *SetValue) emptyLocked() bool { return v.Card() == 0 }
func (v *SetValue) markDeadLocked()   { v.dead = true }

func (v *SetValue) Type() DataType { return DataTypeSet }
func (v *SetValue) SizeOf() int64 {
	if n, ok := v.size.load(); ok {
		return setCompactBase + n + v.adj.Load()
	}
	v.mu.RLock()
	defer v.mu.RUnlock()
	var n int64
	v.Range(func(k string) bool {
		n += setMemberSize(k)
		return true
	})
	v.size.set(n)
	return setCompactBase + n + v.adj.Load()
}
func (v *SetValue) AddMember(member string) bool {
	if v.Has(member) {
		return false
	}
	v.addNewLocked(member)
	v.size.add(setMemberSize(member))
	v.invalidateScan()
	return true
}
func (v *SetValue) RemoveMember(member string) bool {
	if !v.removeLocked(member) {
		return false
	}
	v.size.add(-setMemberSize(member))
	v.invalidateScan()
	return true
}
func (v *SetValue) ClearMembers() {
	v.dict, v.lp, v.lpN = nil, nil, 0
	v.enc, v.order = setEncNone, nil
	v.adj.Store(0)
	v.size.set(0)
	v.invalidateScan()
}
func (v *SetValue) String() string {
	v.mu.RLock()
	defer v.mu.RUnlock()
	result := ""
	v.Range(func(k string) bool {
		if result != "" {
			result += ", "
		}
		result += k
		return true
	})
	return result
}
func (v *SetValue) Clone() Value {
	v.mu.RLock()
	defer v.mu.RUnlock()
	cloned := &SetValue{enc: v.enc, lpN: v.lpN}
	cloned.adj.Store(v.adj.Load())
	cloned.lp = append([]byte(nil), v.lp...)
	if v.dict != nil {
		cloned.dict = make(map[string]struct{}, len(v.dict))
		for k := range v.dict {
			cloned.dict[k] = struct{}{}
		}
	}
	if v.order != nil {
		cloned.order = append([]string(nil), v.order...)
	}
	return cloned
}

type Entry struct {
	Value Value
	// meta holds what few entries need: the owning store's loading flag and
	// the key's tags. Untagged entries of a store share one, so the field
	// costs a pointer and Entry fits a 48-byte allocation.
	meta atomic.Pointer[entryMeta]
	// access is the key's last access and LFU counter (see access.go).
	access atomic.Uint64
	// ExpiresAt is atomic because Entry has no lock of its own and Shard.Get
	// hands out a live pointer, releasing the shard lock on return -- so a
	// reader holding that pointer is outside the shard lock while
	// updateExpiry writes this field inside it.
	ExpiresAt atomic.Int64
	// accounted is the footprint (MemoryUsage plus key overhead) the shard
	// last charged for this entry. Removal credits exactly this much, and
	// Store.Reaccount charges the difference after an in-place change.
	accounted atomic.Int64
}

func NewEntry(value Value) *Entry {
	e := &Entry{Value: value}
	e.access.Store(newAccess(nowUnix()))
	return e
}

type entryMeta struct {
	loading *atomic.Bool
	tags    []string
}

// TagList returns the entry's tags; the caller must not modify the slice.
func (e *Entry) TagList() []string {
	if m := e.meta.Load(); m != nil {
		return m.tags
	}
	return nil
}

// SetTags replaces the entry's tags. Callers hold the shard lock or own an
// entry not yet installed.
func (e *Entry) SetTags(tags []string) {
	var loading *atomic.Bool
	if m := e.meta.Load(); m != nil {
		loading = m.loading
	}
	if len(tags) == 0 {
		tags = nil
		if loading == nil {
			e.meta.Store(nil)
			return
		}
	}
	e.meta.Store(&entryMeta{loading: loading, tags: tags})
}

// bind ties the entry to the store whose untagged entries share plain.
func (e *Entry) bind(plain *entryMeta) {
	m := e.meta.Load()
	if m == plain {
		return
	}
	if m == nil || len(m.tags) == 0 {
		e.meta.Store(plain)
		return
	}
	var loading *atomic.Bool
	if plain != nil {
		loading = plain.loading
	}
	if m.loading != loading {
		e.meta.Store(&entryMeta{loading: loading, tags: m.tags})
	}
}

// rehome returns a new entry for the same value with the same metadata, to
// be installed under another key.
func (e *Entry) rehome() *Entry {
	n := &Entry{Value: e.Value}
	if tags := e.TagList(); len(tags) > 0 {
		n.meta.Store(&entryMeta{tags: tags})
	}
	n.access.Store(e.access.Load())
	n.ExpiresAt.Store(e.ExpiresAt.Load())
	return n
}

func (e *Entry) IsExpired() bool {
	if m := e.meta.Load(); m != nil && m.loading != nil && m.loading.Load() {
		return false
	}
	if expiresAt := e.ExpiresAt.Load(); expiresAt != 0 {
		return time.Now().UnixMilli() >= expiresAt
	}
	return false
}

// EntryOverhead is what a key costs besides its name, value and tags: the
// 48-byte Entry and its slot in the shard's map (measured at about 32 bytes
// per key on a heap profile of a million small strings).
const EntryOverhead = 80

func entryMetadataSize(tags []string) int64 {
	size := int64(EntryOverhead)
	for _, tag := range tags {
		size += int64(len(tag)) + 16
	}
	return size
}

func (e *Entry) MemoryUsage() int64 {
	return entryMetadataSize(e.TagList()) + e.Value.SizeOf()
}

func (e *Entry) SetTTL(ttl time.Duration) {
	e.ExpiresAt.Store(time.Now().Add(ttl).UnixMilli())
}

func (e *Entry) SetExpiresAt(expiresAt int64) {
	e.ExpiresAt.Store(expiresAt)
}

func (e *Entry) TTLMillis() int64 {
	expiresAt := e.ExpiresAt.Load()
	if expiresAt == 0 {
		return -1
	}
	now := time.Now().UnixMilli()
	if expiresAt <= now {
		return -2
	}
	return expiresAt - now
}

func expiryDuration(ms int64) time.Duration {
	if ms > int64((1<<63-1)/time.Millisecond) {
		return time.Duration(1<<63 - 1)
	}
	if ms < int64((-1<<63)/time.Millisecond) {
		return time.Duration(-1 << 63)
	}
	return time.Duration(ms) * time.Millisecond
}

func (e *Entry) TTL() time.Duration {
	ms := e.TTLMillis()
	if ms < 0 {
		return time.Duration(ms)
	}
	return expiryDuration(ms)
}
