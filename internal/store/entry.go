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
	Fields map[string][]byte
	mu     sync.RWMutex
	dead   bool
	size   sizeCounter
	// ttl holds the field expiries; nil until a field is first given one.
	ttl *hashFieldTTLs
	// minExpire is the earliest field expiry (unix ms), 0 when no field has
	// one, readable without the lock.
	minExpire atomic.Int64
	// order lists the fields in Redis' listpack order while big is false
	// (see hash_order.go).
	order []string
	big   bool
	scanCache
}

func (v *HashValue) Lock()    { v.mu.Lock() }
func (v *HashValue) Unlock()  { v.mu.Unlock() }
func (v *HashValue) RLock()   { v.mu.RLock() }
func (v *HashValue) RUnlock() { v.mu.RUnlock() }

// IsDead reports whether DeleteIfEmpty removed this value from the keyspace.
// A dead value is empty and detached: writers must not add to it. The caller
// holds the value's lock.
func (v *HashValue) IsDead() bool { return v.dead }

func (v *HashValue) emptyLocked() bool { return len(v.Fields) == 0 }
func (v *HashValue) markDeadLocked()   { v.dead = true }

func (v *HashValue) Type() DataType { return DataTypeHash }
func (v *HashValue) SizeOf() int64 {
	if n, ok := v.size.load(); ok {
		return hashBaseSize + n
	}
	v.mu.RLock()
	defer v.mu.RUnlock()
	var n int64
	for k, val := range v.Fields {
		n += hashFieldSize(k, val)
	}
	n += v.ttlSizeLocked()
	v.size.set(n)
	return hashBaseSize + n
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
	if v.Fields == nil {
		v.Fields = make(map[string][]byte)
	}
	old, exists := v.Fields[field]
	v.Fields[field] = val
	v.noteLengthsLocked(field, val)
	if exists {
		v.size.add(int64(len(val)) - int64(len(old)))
	} else {
		v.orderAppendLocked(field)
		v.size.add(hashFieldSize(field, val))
		v.invalidateScan()
	}
	return !exists
}

// DeleteField removes field and reports whether it existed. The caller holds
// the value's write lock.
func (v *HashValue) DeleteField(field string) bool {
	old, exists := v.Fields[field]
	if !exists {
		return false
	}
	delete(v.Fields, field)
	v.orderRemoveLocked(field)
	v.size.add(-hashFieldSize(field, old))
	v.clearFieldTTL(field)
	v.invalidateScan()
	return true
}
func (v *HashValue) String() string {
	v.mu.RLock()
	defer v.mu.RUnlock()
	result := ""
	for k, val := range v.Fields {
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
	cloned := &HashValue{Fields: make(map[string][]byte, len(v.Fields)), big: v.big}
	for k, val := range v.Fields {
		cv := make([]byte, len(val))
		copy(cv, val)
		cloned.Fields[k] = cv
	}
	if v.order != nil {
		cloned.order = append([]string(nil), v.order...)
	}
	v.cloneTTLsLocked(cloned)
	return cloned
}

type ListValue struct {
	Elements [][]byte
	mu       sync.RWMutex
	dead     bool
	// backing and head let PushFront prepend in amortised O(1): Elements is
	// backing[head:] with free slots before head. Code that reassigns
	// Elements directly simply invalidates the headroom (PushFront checks
	// that Elements still starts at backing[head]) and the next PushFront
	// reallocates.
	backing [][]byte
	head    int
	size    sizeCounter
	// shape is how Redis would hold the list, for OBJECT ENCODING; nil
	// until a write needs it (see list_encoding.go).
	shape *listShape
}

// PushFront prepends items so that items[0] becomes the new head, as LPUSH
// of the items in reverse order does. Prepending
// to a plain slice copies the whole list each time, which made a list built
// by LPUSH quadratic; the headroom kept before Elements makes it amortised
// constant. The caller holds the value's lock.
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
	if !front {
		v.Elements = append(v.Elements, items...)
		v.size.add(sumListElems(items))
		return
	}
	n := len(items)
	v.size.add(sumListElems(items))
	if v.head >= n && v.headroomValid() {
		v.head -= n
		copy(v.backing[v.head:], items)
		v.Elements = v.backing[v.head : v.head+n+len(v.Elements)]
		return
	}
	size := n + len(v.Elements)
	room := size
	if room < 8 {
		room = 8
	}
	buf := make([][]byte, room+size, room+size+size/2+8)
	copy(buf[room:], items)
	copy(buf[room+n:], v.Elements)
	v.backing = buf
	v.head = room
	v.Elements = buf[room : room+size]
}

// headroomValid reports whether Elements still starts at backing[head], so
// the slots before it are free to reuse.
func (v *ListValue) headroomValid() bool {
	if v.backing == nil || len(v.Elements) == 0 || v.head >= len(v.backing) {
		return false
	}
	return &v.backing[v.head] == &v.Elements[0] && cap(v.backing)-v.head == cap(v.Elements)
}

// PopFront removes and returns up to n elements from the head. The vacated
// slots are cleared so the popped values can be collected, and they become
// headroom for later PushFront calls. The caller holds the value's lock.
func (v *ListValue) PopFront(n int) [][]byte {
	return v.pop(n, true)
}

func (v *ListValue) pop(n int, front bool) [][]byte {
	if n <= 0 || len(v.Elements) == 0 {
		return nil
	}
	s := v.shapeLocked()
	out := v.popElems(n, front)
	if front {
		s.popFront(out)
	} else {
		s.popBack(out)
	}
	if len(v.Elements) > 0 {
		s.toListpack(true)
	}
	return out
}

func (v *ListValue) popElems(n int, front bool) [][]byte {
	if !front {
		return v.popBackElems(n)
	}
	if n > len(v.Elements) {
		n = len(v.Elements)
	}
	if n <= 0 {
		return nil
	}
	out := make([][]byte, n)
	copy(out, v.Elements[:n])
	v.size.add(-sumListElems(out))
	valid := v.headroomValid()
	clear(v.Elements[:n])
	v.Elements = v.Elements[n:]
	if valid {
		v.head += n
	} else {
		v.backing, v.head = nil, 0
	}
	if len(v.Elements) == 0 {
		// Start over so an emptied queue does not pin its old array.
		v.Elements, v.backing, v.head = v.Elements[:0:0], nil, 0
	}
	return out
}

// PopBack removes and returns up to n elements from the tail, last element
// first. The vacated slots are cleared. The caller holds the value's lock.
func (v *ListValue) PopBack(n int) [][]byte {
	return v.pop(n, false)
}

func (v *ListValue) popBackElems(n int) [][]byte {
	if n > len(v.Elements) {
		n = len(v.Elements)
	}
	if n <= 0 {
		return nil
	}
	out := make([][]byte, n)
	for i := 0; i < n; i++ {
		out[i] = v.Elements[len(v.Elements)-1-i]
	}
	v.size.add(-sumListElems(out))
	clear(v.Elements[len(v.Elements)-n:])
	v.Elements = v.Elements[:len(v.Elements)-n]
	return out
}

func (v *ListValue) Lock()    { v.mu.Lock() }
func (v *ListValue) Unlock()  { v.mu.Unlock() }
func (v *ListValue) RLock()   { v.mu.RLock() }
func (v *ListValue) RUnlock() { v.mu.RUnlock() }

// IsDead reports whether DeleteIfEmpty removed this value from the keyspace.
// A dead value is empty and detached: writers must not add to it. The caller
// holds the value's lock.
func (v *ListValue) IsDead() bool { return v.dead }

func (v *ListValue) emptyLocked() bool { return len(v.Elements) == 0 }
func (v *ListValue) markDeadLocked()   { v.dead = true }

func (v *ListValue) Type() DataType { return DataTypeList }
func (v *ListValue) SizeOf() int64 {
	if n, ok := v.size.load(); ok {
		return listBaseSize + n
	}
	v.mu.RLock()
	defer v.mu.RUnlock()
	n := sumListElems(v.Elements)
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
	s.replace(v.Elements, i, el)
	s.toListpack(true)
	old := v.Elements[i]
	v.Elements[i] = el
	v.size.add(int64(len(el)) - int64(len(old)))
}

// Trim keeps only the elements in [start, stop], which must be valid indexes
// with start <= stop, as LTRIM does. It costs O(removed elements). The caller
// holds the value's write lock.
func (v *ListValue) Trim(start, stop int) {
	s := v.shapeLocked()
	s.popFront(v.Elements[:start])
	for i := len(v.Elements) - 1; i > stop; i-- {
		s.popBack(v.Elements[i : i+1])
	}
	s.toListpack(true)
	removed := sumListElems(v.Elements[:start]) + sumListElems(v.Elements[stop+1:])
	clear(v.Elements[:start])
	clear(v.Elements[stop+1:])
	v.Elements = v.Elements[start : stop+1]
	v.size.add(-removed)
}

// ReplaceElements installs elems as the whole list, for the commands that
// rebuild it (LINSERT, LREM, LTRIM, ...). Recounting the size is O(len(elems)),
// which those commands already pay to build elems. The caller holds the
// value's write lock.
func (v *ListValue) ReplaceElements(elems [][]byte) {
	v.Elements, v.shape = elems, nil
	v.size.set(sumListElems(elems))
}
func (v *ListValue) String() string {
	v.mu.RLock()
	defer v.mu.RUnlock()
	var result strings.Builder
	for i, el := range v.Elements {
		if i > 0 {
			result.WriteString(", ")
		}
		result.Write(el)
	}
	return result.String()
}
func (v *ListValue) Clone() Value {
	v.mu.RLock()
	defer v.mu.RUnlock()
	cloned := &ListValue{Elements: make([][]byte, len(v.Elements))}
	for i, el := range v.Elements {
		cel := make([]byte, len(el))
		copy(cel, el)
		cloned.Elements[i] = cel
	}
	if v.shape != nil && v.shape.count == len(v.Elements) {
		cloned.shape = v.shape.clone()
	}
	return cloned
}

type SetValue struct {
	Members map[string]struct{}
	mu      sync.RWMutex
	dead    bool
	size    sizeCounter
	// enc and order are the encoding Redis would hold the set in and, for
	// an intset or listpack, its member order (see set_order.go).
	enc   setEncoding
	order []string
	scanCache
}

func (v *SetValue) Lock()    { v.mu.Lock() }
func (v *SetValue) Unlock()  { v.mu.Unlock() }
func (v *SetValue) RLock()   { v.mu.RLock() }
func (v *SetValue) RUnlock() { v.mu.RUnlock() }

// IsDead reports whether DeleteIfEmpty removed this value from the keyspace.
// A dead value is empty and detached: writers must not add to it. The caller
// holds the value's lock.
func (v *SetValue) IsDead() bool { return v.dead }

func (v *SetValue) emptyLocked() bool { return len(v.Members) == 0 }
func (v *SetValue) markDeadLocked()   { v.dead = true }

func (v *SetValue) Type() DataType { return DataTypeSet }
func (v *SetValue) SizeOf() int64 {
	if n, ok := v.size.load(); ok {
		return setBaseSize + n
	}
	v.mu.RLock()
	defer v.mu.RUnlock()
	var n int64
	for k := range v.Members {
		n += setMemberSize(k)
	}
	v.size.set(n)
	return setBaseSize + n
}

// AddMember adds member and reports whether it is new. The caller holds the
// value's write lock.
func (v *SetValue) AddMember(member string) bool {
	if v.Members == nil {
		v.Members = make(map[string]struct{})
	}
	if _, exists := v.Members[member]; exists {
		return false
	}
	v.orderAddLocked(member)
	v.Members[member] = struct{}{}
	v.size.add(setMemberSize(member))
	v.invalidateScan()
	return true
}

// RemoveMember removes member and reports whether it was present. The caller
// holds the value's write lock.
func (v *SetValue) RemoveMember(member string) bool {
	if _, exists := v.Members[member]; !exists {
		return false
	}
	delete(v.Members, member)
	v.orderRemoveLocked(member)
	v.size.add(-setMemberSize(member))
	v.invalidateScan()
	return true
}

// ClearMembers empties the set. The caller holds the value's write lock.
func (v *SetValue) ClearMembers() {
	v.Members = make(map[string]struct{})
	v.enc, v.order = setEncNone, nil
	v.size.set(0)
	v.invalidateScan()
}
func (v *SetValue) String() string {
	v.mu.RLock()
	defer v.mu.RUnlock()
	result := ""
	for k := range v.Members {
		if result != "" {
			result += ", "
		}
		result += k
	}
	return result
}
func (v *SetValue) Clone() Value {
	v.mu.RLock()
	defer v.mu.RUnlock()
	cloned := &SetValue{Members: make(map[string]struct{}, len(v.Members)), enc: v.enc}
	for k := range v.Members {
		cloned.Members[k] = struct{}{}
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
