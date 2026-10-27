package store

import "math"

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
