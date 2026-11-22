package store

import (
	"sync"

	"github.com/cachestorm/cachestorm/internal/sparsearray"
)

// ArrayValue is Redis 8.8's array: a sparse array of strings indexed from 0
// to 2^64-2, with the cursor ARINSERT and ARRING write at.
type ArrayValue struct {
	mu   sync.RWMutex
	ar   *sparsearray.Array
	dead bool
}

func (v *ArrayValue) emptyLocked() bool { return v.ar.Count() == 0 }
func (v *ArrayValue) markDeadLocked()   { v.dead = true }

// IsDead reports the array was emptied and removed from the keyspace; the
// caller holds its lock.
func (v *ArrayValue) IsDead() bool { return v.dead }

func arrayLimits() (kmax, kmin uint32) {
	_, mx, mn := ArrayConfig()
	return uint32(mx), uint32(mn)
}

// NewArrayValue returns an empty array with the current array-* configs.
func NewArrayValue() *ArrayValue {
	size, kmax, kmin := ArrayConfig()
	return &ArrayValue{ar: sparsearray.New(sparsearray.Config{
		SliceSize: uint32(size), KMax: uint32(kmax), KMin: uint32(kmin), Limits: arrayLimits,
	})}
}

func (v *ArrayValue) Lock()    { v.mu.Lock() }
func (v *ArrayValue) Unlock()  { v.mu.Unlock() }
func (v *ArrayValue) RLock()   { v.mu.RLock() }
func (v *ArrayValue) RUnlock() { v.mu.RUnlock() }

// Array is the sparse array; the caller holds the value's lock.
func (v *ArrayValue) Array() *sparsearray.Array { return v.ar }

// Replace swaps in a rebuilt array (ARRING's resize); the caller holds the
// value's write lock.
func (v *ArrayValue) Replace(a *sparsearray.Array) { v.ar = a }

func (v *ArrayValue) Type() DataType { return DataTypeArray }
func (v *ArrayValue) String() string { return "array" }
func (v *ArrayValue) SizeOf() int64 {
	v.mu.RLock()
	defer v.mu.RUnlock()
	return 48 + v.ar.SizeOf()
}

func (v *ArrayValue) Clone() Value {
	v.mu.RLock()
	defer v.mu.RUnlock()
	return &ArrayValue{ar: v.ar.Dup()}
}

// Count is the number of elements set.
func (v *ArrayValue) Count() uint64 {
	v.mu.RLock()
	defer v.mu.RUnlock()
	return v.ar.Count()
}
