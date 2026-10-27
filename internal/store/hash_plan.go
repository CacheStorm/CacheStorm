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
				hash.SetField(field, data)
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
		if nx {
			if _, exists := hash.Fields[string(pairs[0])]; exists {
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
		hash.Fields[string(op.pairs[2*i])] = op.pairs[2*i+1]
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
