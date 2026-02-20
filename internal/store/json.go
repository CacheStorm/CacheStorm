package store

import (
	"encoding/json"
	"sync"
)

type JSONValue struct {
	Data []byte
	mu   sync.RWMutex
}

func NewJSONValue(data interface{}) (*JSONValue, error) {
	b, err := json.Marshal(data)
	if err != nil {
		return nil, err
	}
	return &JSONValue{Data: b}, nil
}

func (v *JSONValue) Type() DataType { return DataTypeString }

func (v *JSONValue) SizeOf() int64 {
	v.mu.RLock()
	defer v.mu.RUnlock()
	return int64(len(v.Data)) + 24
}

func (v *JSONValue) String() string {
	v.mu.RLock()
	defer v.mu.RUnlock()
	return string(v.Data)
}

func (v *JSONValue) Clone() Value {
	v.mu.RLock()
	defer v.mu.RUnlock()
	data := make([]byte, len(v.Data))
	copy(data, v.Data)
	return &JSONValue{Data: data}
}

func (v *JSONValue) Get() (interface{}, error) {
	v.mu.RLock()
	defer v.mu.RUnlock()
	var result interface{}
	err := json.Unmarshal(v.Data, &result)
	return result, err
}

func (v *JSONValue) GetPath(path string) (interface{}, error) {
	v.mu.RLock()
	defer v.mu.RUnlock()

	if path == "" || path == "$" || path == "." {
		var result interface{}
		err := json.Unmarshal(v.Data, &result)
		return result, err
	}

	var data interface{}
	if err := json.Unmarshal(v.Data, &data); err != nil {
		return nil, err
	}

	return getByPath(data, path)
}

func getByPath(data interface{}, path string) (interface{}, error) {
	parts := parseJSONPath(path)
	current := data

	for _, part := range parts {
		switch v := current.(type) {
		case map[string]interface{}:
			if val, ok := v[part]; ok {
				current = val
			} else {
				return nil, nil
			}
		case []interface{}:
			idx := 0
			for _, c := range part {
				if c >= '0' && c <= '9' {
					idx = idx*10 + int(c-'0')
				} else {
					return nil, nil
				}
			}
			if idx >= 0 && idx < len(v) {
				current = v[idx]
			} else {
				return nil, nil
			}
		default:
			return nil, nil
		}
	}

	return current, nil
}

func parseJSONPath(path string) []string {
	if path == "" || path == "$" || path == "." {
		return nil
	}

	if path[0] == '$' || path[0] == '.' {
		path = path[1:]
	}

	if path == "" {
		return nil
	}

	parts := []string{}
	current := ""

	for i := 0; i < len(path); i++ {
		c := path[i]
		if c == '.' || c == '[' {
			if current != "" {
				parts = append(parts, current)
				current = ""
			}
			if c == '[' {
				j := i + 1
				for j < len(path) && path[j] != ']' {
					j++
				}
				parts = append(parts, path[i+1:j])
				i = j
			}
		} else {
			current += string(c)
		}
	}

	if current != "" {
		parts = append(parts, current)
	}

	return parts
}

func (v *JSONValue) Set(data interface{}) error {
	v.mu.Lock()
	defer v.mu.Unlock()
	b, err := json.Marshal(data)
	if err != nil {
		return err
	}
	v.Data = b
	return nil
}

func (v *JSONValue) SetPath(path string, value interface{}) error {
	v.mu.Lock()
	defer v.mu.Unlock()

	if path == "" || path == "$" || path == "." {
		b, err := json.Marshal(value)
		if err != nil {
			return err
		}
		v.Data = b
		return nil
	}

	var data interface{}
	if len(v.Data) > 0 {
		if err := json.Unmarshal(v.Data, &data); err != nil {
			data = make(map[string]interface{})
		}
	} else {
		data = make(map[string]interface{})
	}

	if err := setByPath(data, path, value); err != nil {
		return err
	}

	b, err := json.Marshal(data)
	if err != nil {
		return err
	}
	v.Data = b
	return nil
}

func setByPath(data interface{}, path string, value interface{}) error {
	parts := parseJSONPath(path)
	if len(parts) == 0 {
		return nil
	}

	switch d := data.(type) {
	case map[string]interface{}:
		if len(parts) == 1 {
			d[parts[0]] = value
			return nil
		}
		if _, ok := d[parts[0]]; !ok {
			d[parts[0]] = make(map[string]interface{})
		}
		return setByPath(d[parts[0]], path[len(parts[0])+1:], value)
	}

	return nil
}

func (v *JSONValue) DeletePath(path string) error {
	v.mu.Lock()
	defer v.mu.Unlock()

	if path == "" || path == "$" || path == "." {
		v.Data = []byte{}
		return nil
	}

	var data interface{}
	if err := json.Unmarshal(v.Data, &data); err != nil {
		return err
	}

	parts := parseJSONPath(path)
	if len(parts) == 0 {
		return nil
	}

	deleteByPath(data, parts)

	b, err := json.Marshal(data)
	if err != nil {
		return err
	}
	v.Data = b
	return nil
}

func deleteByPath(data interface{}, parts []string) {
	if len(parts) == 0 {
		return
	}

	switch d := data.(type) {
	case map[string]interface{}:
		if len(parts) == 1 {
			delete(d, parts[0])
			return
		}
		if val, ok := d[parts[0]]; ok {
			deleteByPath(val, parts[1:])
		}
	}
}

func (v *JSONValue) TypeAt(path string) (string, error) {
	val, err := v.GetPath(path)
	if err != nil {
		return "", err
	}
	if val == nil {
		return "null", nil
	}

	switch val.(type) {
	case bool:
		return "boolean", nil
	case float64:
		if float64(int(val.(float64))) == val.(float64) {
			return "integer", nil
		}
		return "number", nil
	case string:
		return "string", nil
	case []interface{}:
		return "array", nil
	case map[string]interface{}:
		return "object", nil
	default:
		return "unknown", nil
	}
}

func (v *JSONValue) NumIncrBy(path string, increment float64) (float64, error) {
	v.mu.Lock()
	defer v.mu.Unlock()

	var data interface{}
	if err := json.Unmarshal(v.Data, &data); err != nil {
		return 0, err
	}

	parts := parseJSONPath(path)
	if len(parts) == 0 {
		if num, ok := data.(float64); ok {
			result := num + increment
			b, _ := json.Marshal(result)
			v.Data = b
			return result, nil
		}
	}

	result, err := incrByPath(data, parts, increment)
	if err != nil {
		return 0, err
	}

	b, err := json.Marshal(data)
	if err != nil {
		return 0, err
	}
	v.Data = b
	return result, nil
}

func incrByPath(data interface{}, parts []string, increment float64) (float64, error) {
	if len(parts) == 0 {
		if num, ok := data.(float64); ok {
			return num + increment, nil
		}
		return 0, nil
	}

	switch d := data.(type) {
	case map[string]interface{}:
		if len(parts) == 1 {
			if num, ok := d[parts[0]].(float64); ok {
				result := num + increment
				d[parts[0]] = result
				return result, nil
			}
		}
		if val, ok := d[parts[0]]; ok {
			return incrByPath(val, parts[1:], increment)
		}
	}

	return 0, nil
}

func (v *JSONValue) ArrAppend(path string, values []interface{}) (int, error) {
	v.mu.Lock()
	defer v.mu.Unlock()

	var data interface{}
	if err := json.Unmarshal(v.Data, &data); err != nil {
		return 0, err
	}

	parts := parseJSONPath(path)
	length, err := arrAppendPath(data, parts, values)
	if err != nil {
		return 0, err
	}

	b, err := json.Marshal(data)
	if err != nil {
		return 0, err
	}
	v.Data = b
	return length, nil
}

func arrAppendPath(data interface{}, parts []string, values []interface{}) (int, error) {
	if len(parts) == 0 {
		if arr, ok := data.([]interface{}); ok {
			arr = append(arr, values...)
			return len(arr), nil
		}
		return 0, nil
	}

	switch d := data.(type) {
	case map[string]interface{}:
		if len(parts) == 1 {
			if arr, ok := d[parts[0]].([]interface{}); ok {
				arr = append(arr, values...)
				d[parts[0]] = arr
				return len(arr), nil
			}
		}
		if val, ok := d[parts[0]]; ok {
			return arrAppendPath(val, parts[1:], values)
		}
	}

	return 0, nil
}

func (v *JSONValue) StrLen(path string) (int, error) {
	val, err := v.GetPath(path)
	if err != nil {
		return 0, err
	}
	if str, ok := val.(string); ok {
		return len(str), nil
	}
	return 0, nil
}

func (v *JSONValue) ObjLen(path string) (int, error) {
	val, err := v.GetPath(path)
	if err != nil {
		return 0, err
	}
	if obj, ok := val.(map[string]interface{}); ok {
		return len(obj), nil
	}
	return 0, nil
}

func (v *JSONValue) ArrLen(path string) (int, error) {
	val, err := v.GetPath(path)
	if err != nil {
		return 0, err
	}
	if arr, ok := val.([]interface{}); ok {
		return len(arr), nil
	}
	return 0, nil
}
