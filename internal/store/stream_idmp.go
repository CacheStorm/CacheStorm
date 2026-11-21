package store

// Redis 8.6's stream idempotency (IDMP): XADD ... IDMP pid iid adds an entry
// only once per producer pid and idempotent ID iid, remembering the iids of
// the last idmp-duration seconds, at most idmp-maxsize per producer.

// IDMP defaults, Redis' stream-idmp-duration and stream-idmp-maxsize.
const (
	DefaultIDMPDuration = 100
	DefaultIDMPMaxSize  = 100
)

// IDMPEntry is one remembered iid and the entry XADD added for it.
type IDMPEntry struct {
	IID string
	ID  StreamID
}

// StreamIDMP is a stream's IDMP configuration and memory.
type StreamIDMP struct {
	Duration, MaxSize int64
	// Producers maps a pid to its iids, oldest first.
	Producers map[string][]IDMPEntry
	// Added and Duplicates count every iid added and every duplicate
	// XADD refused.
	Added, Duplicates int64
}

// SetLoadedIDMP installs IDMP state read from an RDB file or DUMP payload.
func (v *StreamValue) SetLoadedIDMP(m *StreamIDMP) {
	v.mu.Lock()
	defer v.mu.Unlock()
	v.idmp = m
}

// IDMP returns a copy of the stream's IDMP state, nil when it has none.
func (v *StreamValue) IDMP() *StreamIDMP {
	v.mu.RLock()
	defer v.mu.RUnlock()
	return v.idmp.clone()
}

func (m *StreamIDMP) clone() *StreamIDMP {
	if m == nil {
		return nil
	}
	c := *m
	c.Producers = make(map[string][]IDMPEntry, len(m.Producers))
	for pid, es := range m.Producers {
		c.Producers[pid] = append([]IDMPEntry(nil), es...)
	}
	return &c
}

// IDMPLocked is the stream's IDMP state, nil when it has none. The caller
// holds the stream's lock and must not change it.
func (v *StreamValue) IDMPLocked() *StreamIDMP { return v.idmp }
