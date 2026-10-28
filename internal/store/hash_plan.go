package store

import (
	"bytes"
	"math"
	"sync"
)

func (s *Store) ApplyHashFields(key string, prepare func(*HashValue) (map[string][]byte, error)) error {
	return s.ApplyValuePlan(key, func(value Value) (ValuePlan, error) {
		var hash *HashValue
		if value == nil {
			hash = &HashValue{}
		} else {
			var ok bool
			hash, ok = value.(*HashValue)
			if !ok {
				return ValuePlan{}, ErrWrongType
			}
		}
		hash.SizeOf()
		hash.Lock()
		now := s.HashFieldClock()
		fields, err := prepare(hash)
		if err != nil || len(fields) == 0 {
			hash.Unlock()
			return ValuePlan{}, err
		}
		size := hash.SizeOf()
		for field := range fields {
			if old, exists := hash.Fields[field]; exists {
				size -= hashFieldSize(field, old)
			}
		}
		for field, data := range fields {
			n := hashFieldSize(field, data)
			if n > math.MaxInt64-size {
				hash.Unlock()
				return ValuePlan{}, ErrValueTooLarge
			}
			size += n
		}
		return ValuePlan{Size: size, Apply: func() Value {
			for field, data := range fields {
				hash.dropExpiredTTL(field, now)
				hash.SetFieldKeepTTL(field, data)
			}
			return hash
		}, unlock: hash.Unlock, allowEmptyKey: value != nil}, nil
	})
}

// ApplyHashPairs sets the field/value pairs in args (field, value, field,
// value, ...) on the hash at key and reports how many fields are new. With
// nx only the first field is set, and only when it is absent. A field named
// twice takes its last value, as in Redis. It is ApplyHashFields without the
// intermediate map: each field is looked up once to plan and stored once.
func (s *Store) ApplyHashPairs(key string, nx bool, args [][]byte) (int, error) {
	added := 0
	err := s.ApplyValuePlan(key, func(value Value) (ValuePlan, error) {
		var hash *HashValue
		if value == nil {
			hash = &HashValue{}
		} else {
			var ok bool
			hash, ok = value.(*HashValue)
			if !ok {
				return ValuePlan{}, ErrWrongType
			}
		}
		hash.SizeOf()
		hash.Lock()
		added = 0
		pairs := args
		var now int64
		if hash.HasFieldTTLs() {
			now = s.HashFieldClock()
		}
		if nx {
			if hash.FieldLive(string(pairs[0]), now) {
				hash.Unlock()
				return ValuePlan{}, nil
			}
			pairs = pairs[:2]
		}
		op := hashSetOpPool.Get().(*hashSetOp)
		op.hash, op.pairs = hash, pairs
		n := len(pairs) / 2
		// keep lists the pairs that are not overridden by a later pair for
		// the same field.
		var last map[string]int
		if n > len(op.keepBuf) {
			last = make(map[string]int, n)
			for i := 0; i < n; i++ {
				last[string(pairs[2*i])] = i
			}
		}
		for i := 0; i < n; i++ {
			if last != nil {
				if last[string(pairs[2*i])] != i {
					continue
				}
			} else {
				later := false
				for j := i + 1; j < n; j++ {
					if bytes.Equal(pairs[2*j], pairs[2*i]) {
						later = true
						break
					}
				}
				if later {
					continue
				}
			}
			op.keep = append(op.keep, i)
		}
		size := hash.SizeOf()
		for _, i := range op.keep {
			field, val := pairs[2*i], pairs[2*i+1]
			var d int64
			if old, exists := hash.Fields[string(field)]; exists {
				d = int64(len(val)) - int64(len(old))
				if !hash.FieldLive(string(field), now) {
					added++
				}
			} else {
				d = hashFieldSize(string(field), val)
				added++
			}
			if d > 0 && d > math.MaxInt64-size {
				op.release()
				return ValuePlan{}, ErrValueTooLarge
			}
			size += d
			op.deltas = append(op.deltas, d)
		}
		return ValuePlan{Size: size, op: op, allowEmptyKey: value != nil}, nil
	})
	return added, err
}

// hashSetOp is ApplyHashPairs' plan, pooled so that HSET on a large hash does
// not allocate a closure and its slices on every call.
type hashSetOp struct {
	hash     *HashValue
	pairs    [][]byte
	keep     []int
	deltas   []int64
	keepBuf  [8]int
	deltaBuf [8]int64
}

var hashSetOpPool = sync.Pool{New: func() any {
	op := &hashSetOp{}
	op.keep, op.deltas = op.keepBuf[:0], op.deltaBuf[:0]
	return op
}}

func (op *hashSetOp) apply() Value {
	hash := op.hash
	if hash.Fields == nil {
		hash.Fields = make(map[string][]byte, len(op.keep))
	}
	for k, i := range op.keep {
		field := string(op.pairs[2*i])
		hash.clearFieldTTL(field)
		hash.Fields[field] = op.pairs[2*i+1]
		hash.size.add(op.deltas[k])
	}
	return hash
}

// release unlocks the hash and returns op to the pool.
func (op *hashSetOp) release() {
	op.hash.Unlock()
	op.hash, op.pairs = nil, nil
	if cap(op.keep) > len(op.keepBuf) {
		op.keep, op.deltas = op.keepBuf[:0], op.deltaBuf[:0]
	} else {
		op.keep, op.deltas = op.keep[:0], op.deltas[:0]
	}
	hashSetOpPool.Put(op)
}

// HashSetExOptions are HSETEX's options: FNX sets the fields only when none
// exists, FXX only when all exist; KeepTTL keeps the fields' expiries and
// ExpireAt (unix ms, 0 for none) gives them one. Without either a set field
// loses its expiry.
type HashSetExOptions struct {
	FNX      bool
	FXX      bool
	KeepTTL  bool
	ExpireAt int64
}

// ApplyHashSetEx sets the field/value pairs in args on the hash at key, all
// of them or, when the FNX/FXX condition fails, none, and reports whether it
// set them. A field named twice takes its last value.
func (s *Store) ApplyHashSetEx(key string, args [][]byte, o HashSetExOptions) (bool, error) {
	applied := false
	err := s.ApplyValuePlan(key, func(value Value) (ValuePlan, error) {
		applied = false
		var hash *HashValue
		if value == nil {
			if o.FXX {
				return ValuePlan{}, nil
			}
			hash = &HashValue{}
		} else {
			var ok bool
			hash, ok = value.(*HashValue)
			if !ok {
				return ValuePlan{}, ErrWrongType
			}
		}
		hash.SizeOf()
		hash.Lock()
		now := s.HashFieldClock()
		size := hash.SizeOf()
		seen := make(map[string]int, len(args)/2)
		for i := 0; i < len(args); i += 2 {
			seen[string(args[i])] = i
		}
		for field, i := range seen {
			old, exists := hash.Fields[field]
			if live := hash.FieldLive(field, now); (o.FNX && live) || (o.FXX && !live) {
				hash.Unlock()
				return ValuePlan{}, nil
			}
			n := hashFieldSize(field, args[i+1])
			if exists {
				n -= hashFieldSize(field, old)
			}
			if o.ExpireAt > 0 {
				n += hashFieldTTLOverhead
			}
			if n > 0 && n > math.MaxInt64-size {
				hash.Unlock()
				return ValuePlan{}, ErrValueTooLarge
			}
			size += n
		}
		applied = true
		return ValuePlan{Size: size, Apply: func() Value {
			for field, i := range seen {
				if o.KeepTTL {
					hash.dropExpiredTTL(field, now)
					hash.SetFieldKeepTTL(field, args[i+1])
				} else {
					hash.SetField(field, args[i+1])
				}
				if o.ExpireAt > 0 {
					hash.SetFieldExpireAt(field, o.ExpireAt)
				}
			}
			return hash
		}, unlock: hash.Unlock, allowEmptyKey: value != nil}, nil
	})
	if err != nil {
		applied = false
	}
	return applied, err
}
