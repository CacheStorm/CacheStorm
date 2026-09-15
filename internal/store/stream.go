package store

import (
	"errors"
	"strconv"
	"strings"
	"sync"
	"time"
)

type StreamEntry struct {
	ID        string
	Fields    map[string][]byte
	CreatedAt time.Time
}

type PendingEntry struct {
	ID         string
	Consumer   string
	DeliveryTS int64
	Deliveries int64
}

type Consumer struct {
	Name     string
	SeenTime int64
	Pending  int64
	Active   bool
}

type ConsumerGroup struct {
	Name      string
	LastID    string
	Consumers map[string]*Consumer
	Pending   map[string]*PendingEntry
	mu        sync.RWMutex
}

func NewConsumerGroup(name string) *ConsumerGroup {
	return &ConsumerGroup{
		Name:      name,
		LastID:    "0-0",
		Consumers: make(map[string]*Consumer),
		Pending:   make(map[string]*PendingEntry),
	}
}

func (g *ConsumerGroup) GetOrCreateConsumer(name string) *Consumer {
	g.mu.Lock()
	defer g.mu.Unlock()

	if c, exists := g.Consumers[name]; exists {
		return c
	}

	c := &Consumer{
		Name:     name,
		SeenTime: time.Now().UnixMilli(),
		Active:   true,
	}
	g.Consumers[name] = c
	return c
}

func (g *ConsumerGroup) AddPending(entryID, consumer string) {
	g.mu.Lock()
	defer g.mu.Unlock()

	g.Pending[entryID] = &PendingEntry{
		ID:         entryID,
		Consumer:   consumer,
		DeliveryTS: time.Now().UnixMilli(),
		Deliveries: 1,
	}

	if c, exists := g.Consumers[consumer]; exists {
		c.Pending++
	}
}

func (g *ConsumerGroup) Ack(entryID string) bool {
	g.mu.Lock()
	defer g.mu.Unlock()

	if p, exists := g.Pending[entryID]; exists {
		if c, exists := g.Consumers[p.Consumer]; exists {
			c.Pending--
		}
		delete(g.Pending, entryID)
		return true
	}
	return false
}

func (g *ConsumerGroup) Claim(entryIDs []string, newConsumer string) []string {
	g.mu.Lock()
	defer g.mu.Unlock()

	var claimed []string
	for _, id := range entryIDs {
		if p, exists := g.Pending[id]; exists {
			if c, exists := g.Consumers[p.Consumer]; exists {
				c.Pending--
			}
			p.Consumer = newConsumer
			p.DeliveryTS = time.Now().UnixMilli()
			p.Deliveries++
			if c, exists := g.Consumers[newConsumer]; exists {
				c.Pending++
			}
			claimed = append(claimed, id)
		}
	}
	return claimed
}

func (g *ConsumerGroup) GetPending(start, end string, count int64) []*PendingEntry {
	g.mu.RLock()
	defer g.mu.RUnlock()

	var result []*PendingEntry
	for _, p := range g.Pending {
		if (start == "-" || p.ID >= start) && (end == "+" || p.ID <= end) {
			result = append(result, p)
			if count > 0 && int64(len(result)) >= count {
				break
			}
		}
	}
	return result
}

func (g *ConsumerGroup) GetPendingCount() int64 {
	g.mu.RLock()
	defer g.mu.RUnlock()
	return int64(len(g.Pending))
}

func (g *ConsumerGroup) GetFirstLastID() (string, string) {
	g.mu.RLock()
	defer g.mu.RUnlock()

	var firstID, lastID string
	for id := range g.Pending {
		if firstID == "" || id < firstID {
			firstID = id
		}
		if lastID == "" || id > lastID {
			lastID = id
		}
	}
	return firstID, lastID
}

func (g *ConsumerGroup) GetConsumerPending(consumerName string) int64 {
	g.mu.RLock()
	defer g.mu.RUnlock()
	if c, exists := g.Consumers[consumerName]; exists {
		return c.Pending
	}
	return 0
}

func (g *ConsumerGroup) GetAllConsumers() []string {
	g.mu.RLock()
	defer g.mu.RUnlock()

	var consumers []string
	for name := range g.Consumers {
		consumers = append(consumers, name)
	}
	return consumers
}

func (g *ConsumerGroup) GetConsumer(name string) *Consumer {
	g.mu.RLock()
	defer g.mu.RUnlock()
	return g.Consumers[name]
}

type StreamValue struct {
	mu      sync.RWMutex
	Entries []*StreamEntry
	LastID  string
	Length  int64
	MaxLen  int64
	Groups  map[string]*ConsumerGroup
}

func NewStreamValue(maxLen int64) *StreamValue {
	return &StreamValue{
		Entries: make([]*StreamEntry, 0),
		MaxLen:  maxLen,
		Groups:  make(map[string]*ConsumerGroup),
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
		Groups:  make(map[string]*ConsumerGroup),
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

	for name, group := range v.Groups {
		cloned.Groups[name] = NewConsumerGroup(group.Name)
		cloned.Groups[name].LastID = group.LastID
	}

	return cloned
}

func (v *StreamValue) Add(id string, fields map[string][]byte) (*StreamEntry, error) {
	ms, seq, err := parseStreamIDPair(id)
	if err != nil {
		return nil, err
	}
	if ms == 0 && seq == 0 {
		return nil, errors.New("ERR The ID specified in XADD must be greater than 0-0")
	}

	v.mu.Lock()
	defer v.mu.Unlock()

	if lastMS, lastSeq, lastErr := parseStreamIDPair(v.LastID); lastErr == nil {
		if ms < lastMS || (ms == lastMS && seq <= lastSeq) {
			return nil, errors.New("ERR The ID specified in XADD is equal or smaller than the target stream top item")
		}
	}

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

// parseStreamIDPair parses a stream ID of "ms-seq"; bare, malformed, or
// non-numeric IDs are rejected.
func parseStreamIDPair(id string) (int64, int64, error) {
	parts := strings.Split(id, "-")
	if len(parts) != 2 {
		return 0, 0, errors.New("ERR Invalid stream ID specified as stream command argument")
	}
	ms, err1 := strconv.ParseInt(parts[0], 10, 64)
	seq, err2 := strconv.ParseInt(parts[1], 10, 64)
	if err1 != nil || err2 != nil {
		return 0, 0, errors.New("ERR Invalid stream ID specified as stream command argument")
	}
	return ms, seq, nil
}

func (v *StreamValue) GetRange(start, end string, count int64) []*StreamEntry {
	v.mu.RLock()
	defer v.mu.RUnlock()

	if count == 0 {
		count = int64(len(v.Entries))
	}

	startMS, startSeq, errStart := parseStreamIDPair(start)
	if errStart != nil {
		if start == "-" {
			startMS, startSeq = 0, 0
		} else {
			return nil
		}
	}
	endMS, endSeq, errEnd := parseStreamIDPair(end)
	if errEnd != nil {
		if end == "+" {
			endMS, endSeq = 9223372036854775807, 9223372036854775807
		} else {
			return nil
		}
	}

	var result []*StreamEntry
	for _, entry := range v.Entries {
		eMS, eSeq, err := parseStreamIDPair(entry.ID)
		if err != nil {
			continue
		}
		if eMS < startMS || (eMS == startMS && eSeq < startSeq) {
			continue
		}
		if eMS > endMS || (eMS == endMS && eSeq > endSeq) {
			continue
		}
		result = append(result, entry)
		if int64(len(result)) >= count {
			break
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

func (v *StreamValue) Trim(maxLen int64, _ bool) int64 {
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

func (v *StreamValue) TrimByMinID(minID string, _ bool) int64 {
	v.mu.Lock()
	defer v.mu.Unlock()

	remaining := make([]*StreamEntry, 0)
	removed := int64(0)

	for _, entry := range v.Entries {
		if entry.ID >= minID {
			remaining = append(remaining, entry)
		} else {
			removed++
		}
	}

	v.Entries = remaining
	v.Length -= removed
	return removed
}

func (v *StreamValue) CreateGroup(name, lastID string) error {
	v.mu.Lock()
	defer v.mu.Unlock()

	if _, exists := v.Groups[name]; exists {
		return ErrKeyExists
	}

	group := NewConsumerGroup(name)
	if lastID == "$" {
		group.LastID = v.LastID
	} else {
		group.LastID = lastID
	}
	v.Groups[name] = group
	return nil
}

func (v *StreamValue) DestroyGroup(name string) bool {
	v.mu.Lock()
	defer v.mu.Unlock()

	if _, exists := v.Groups[name]; exists {
		delete(v.Groups, name)
		return true
	}
	return false
}

func (v *StreamValue) GetGroup(name string) *ConsumerGroup {
	v.mu.RLock()
	defer v.mu.RUnlock()
	return v.Groups[name]
}

func (v *StreamValue) SetGroupLastID(groupName, lastID string) bool {
	v.mu.RLock()
	group, exists := v.Groups[groupName]
	v.mu.RUnlock()

	if !exists {
		return false
	}

	group.mu.Lock()
	if lastID == "$" {
		group.LastID = v.LastID
	} else {
		group.LastID = lastID
	}
	group.mu.Unlock()
	return true
}

func (v *StreamValue) GetEntriesAfter(id string, count int64) []*StreamEntry {
	v.mu.RLock()
	defer v.mu.RUnlock()

	var result []*StreamEntry
	for _, entry := range v.Entries {
		if entry.ID > id {
			result = append(result, entry)
			if count > 0 && int64(len(result)) >= count {
				break
			}
		}
	}
	return result
}

func (v *StreamValue) GetEntryByID(id string) *StreamEntry {
	v.mu.RLock()
	defer v.mu.RUnlock()

	for _, entry := range v.Entries {
		if entry.ID == id {
			return entry
		}
	}
	return nil
}

func (v *StreamValue) SetLastID(lastID string) {
	v.mu.Lock()
	defer v.mu.Unlock()
	v.LastID = lastID
}

func (v *StreamValue) GetLastID() string {
	v.mu.RLock()
	defer v.mu.RUnlock()
	return v.LastID
}

func (v *StreamValue) GetGroups() map[string]*ConsumerGroup {
	v.mu.RLock()
	defer v.mu.RUnlock()
	return v.Groups
}

func (v *StreamValue) GetGroupCount() int {
	v.mu.RLock()
	defer v.mu.RUnlock()
	return len(v.Groups)
}
