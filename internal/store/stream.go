package store

import (
	"sync"
	"time"
)

type StreamEntry struct {
	ID        string
	Fields    map[string][]byte
	CreatedAt time.Time
}

type StreamValue struct {
	mu      sync.RWMutex
	Entries []*StreamEntry
	LastID  string
	Length  int64
	MaxLen  int64
}

func NewStreamValue(maxLen int64) *StreamValue {
	return &StreamValue{
		Entries: make([]*StreamEntry, 0),
		MaxLen:  maxLen,
	}
}

func (v *StreamValue) Type() DataType { return DataTypeStream }
func (v *StreamValue) SizeOf() int64 {
	v.mu.RLock()
	defer v.mu.RUnlock()

	var size int64 = 48
	for _, entry := range v.Entries {
		size += int64(len(entry.ID)) + 32
		for k, val := range entry.Fields {
			size += int64(len(k)) + int64(len(val)) + 80
		}
	}
	return size
}
func (v *StreamValue) String() string {
	v.mu.RLock()
	defer v.mu.RUnlock()

	result := ""
	for _, entry := range v.Entries {
		if result != "" {
			result += "\n"
		}
		fields := ""
		for k, val := range entry.Fields {
			if fields != "" {
				fields += ", "
			}
			fields += k + ": " + string(val)
		}
		result += entry.ID + " -> " + fields
	}
	return result
}
func (v *StreamValue) Clone() Value {
	v.mu.RLock()
	defer v.mu.RUnlock()

	cloned := &StreamValue{
		Entries: make([]*StreamEntry, len(v.Entries)),
		LastID:  v.LastID,
		Length:  v.Length,
		MaxLen:  v.MaxLen,
	}

	for i, entry := range v.Entries {
		cloned.Entries[i] = &StreamEntry{
			ID:        entry.ID,
			Fields:    make(map[string][]byte),
			CreatedAt: entry.CreatedAt,
		}
		for k, val := range entry.Fields {
			cloned.Entries[i].Fields[k] = append([]byte(nil), val...)
		}
	}

	return cloned
}

func (v *StreamValue) Add(id string, fields map[string][]byte) (*StreamEntry, error) {
	v.mu.Lock()
	defer v.mu.Unlock()

	entry := &StreamEntry{
		ID:        id,
		Fields:    fields,
		CreatedAt: time.Now(),
	}

	v.Entries = append(v.Entries, entry)
	v.LastID = id
	v.Length++

	if v.MaxLen > 0 && v.Length > v.MaxLen {
		remove := v.Length - v.MaxLen
		v.Entries = v.Entries[remove:]
		v.Length -= remove
	}

	return entry, nil
}

func (v *StreamValue) GetRange(start, end string, count int64) []*StreamEntry {
	v.mu.RLock()
	defer v.mu.RUnlock()

	if count == 0 {
		count = int64(len(v.Entries))
	}

	var result []*StreamEntry
	for _, entry := range v.Entries {
		if entry.ID >= start && (end == "+" || entry.ID <= end) {
			result = append(result, entry)
			if int64(len(result)) >= count {
				break
			}
		}
	}

	return result
}

func (v *StreamValue) Len() int64 {
	v.mu.RLock()
	defer v.mu.RUnlock()
	return v.Length
}

func (v *StreamValue) Delete(ids ...string) int64 {
	v.mu.Lock()
	defer v.mu.Unlock()

	remaining := make([]*StreamEntry, 0)
	deleted := int64(0)

	for _, entry := range v.Entries {
		shouldDelete := false
		for _, id := range ids {
			if entry.ID == id {
				shouldDelete = true
				break
			}
		}
		if shouldDelete {
			deleted++
		} else {
			remaining = append(remaining, entry)
		}
	}

	v.Entries = remaining
	v.Length -= deleted
	return deleted
}

func (v *StreamValue) Trim(maxLen int64, approximate bool) int64 {
	v.mu.Lock()
	defer v.mu.Unlock()

	if maxLen >= v.Length {
		return 0
	}

	remove := v.Length - maxLen
	v.Entries = v.Entries[remove:]
	v.Length = maxLen
	return remove
}
