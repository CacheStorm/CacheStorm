package store

import (
	"bytes"
	"math"
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
		n := len(pairs) / 2
		// keep lists the pairs that are not overridden by a later pair for
		// the same field.
		var inline [8]int
		keep := inline[:0]
		var last map[string]int
		if n > len(inline) {
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
			keep = append(keep, i)
		}
		size := hash.SizeOf()
		var deltas [8]int64
		deltaList := deltas[:0]
		for _, i := range keep {
			field, val := pairs[2*i], pairs[2*i+1]
			var d int64
			if old, exists := hash.Fields[string(field)]; exists {
				d = int64(len(val)) - int64(len(old))
			} else {
				d = hashFieldSize(string(field), val)
				added++
			}
			if d > 0 && d > math.MaxInt64-size {
				hash.Unlock()
				return ValuePlan{}, ErrValueTooLarge
			}
			size += d
			deltaList = append(deltaList, d)
		}
		return ValuePlan{Size: size, Apply: func() Value {
			if hash.Fields == nil {
				hash.Fields = make(map[string][]byte, len(keep))
			}
			for k, i := range keep {
				hash.Fields[string(pairs[2*i])] = pairs[2*i+1]
				hash.size.add(deltaList[k])
			}
			return hash
		}, unlock: hash.Unlock, allowEmptyKey: value != nil}, nil
	})
	return added, err
}
