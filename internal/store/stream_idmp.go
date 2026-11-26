package store

import (
	"errors"
	"sync/atomic"
)

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
	// index finds a producer's iid, built from Producers when nil.
	index map[string]map[string]StreamID
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
	c.index = nil
	c.Producers = make(map[string][]IDMPEntry, len(m.Producers))
	for pid, es := range m.Producers {
		c.Producers[pid] = append([]IDMPEntry(nil), es...)
	}
	return &c
}

// IDMPLocked is the stream's IDMP state, nil when it has none. The caller
// holds the stream's lock and must not change it.
func (v *StreamValue) IDMPLocked() *StreamIDMP { return v.idmp }

var idmpDefaultDuration, idmpDefaultMaxSize atomic.Int64

func init() {
	idmpDefaultDuration.Store(DefaultIDMPDuration)
	idmpDefaultMaxSize.Store(DefaultIDMPMaxSize)
}

// SetIDMPDefaultDuration and SetIDMPDefaultMaxSize set what a new stream
// starts with, the stream-idmp-duration and stream-idmp-maxsize configs.
func SetIDMPDefaultDuration(v int64) { idmpDefaultDuration.Store(v) }
func SetIDMPDefaultMaxSize(v int64)  { idmpDefaultMaxSize.Store(v) }

// IDMPDefaults returns the stream-idmp-duration and stream-idmp-maxsize
// configs.
func IDMPDefaults() (duration, maxSize int64) {
	return idmpDefaultDuration.Load(), idmpDefaultMaxSize.Load()
}

// newStreamIDMP is the state a stream created now starts with; nil stands
// for the built-in defaults.
func newStreamIDMP() *StreamIDMP {
	d, m := IDMPDefaults()
	if d == DefaultIDMPDuration && m == DefaultIDMPMaxSize {
		return nil
	}
	return &StreamIDMP{Duration: d, MaxSize: m}
}

// idmpStateLocked returns the stream's IDMP state, creating it.
func (v *StreamValue) idmpStateLocked() *StreamIDMP {
	if v.idmp == nil {
		v.idmp = &StreamIDMP{Duration: DefaultIDMPDuration, MaxSize: DefaultIDMPMaxSize}
	}
	return v.idmp
}

// IDMPConfig returns the stream's idmp-duration and idmp-maxsize.
func (v *StreamValue) IDMPConfig() (duration, maxSize int64) {
	v.mu.RLock()
	defer v.mu.RUnlock()
	if v.idmp == nil {
		return DefaultIDMPDuration, DefaultIDMPMaxSize
	}
	return v.idmp.Duration, v.idmp.MaxSize
}

// expireIDMPLocked forgets the iids whose entry ID is idmp-duration seconds
// old or older at nowMS, and the producers left without one, as Redis'
// handleExpiredIdmpEntries does.
func (v *StreamValue) expireIDMPLocked(nowMS int64) {
	m := v.idmp
	if m == nil || len(m.Producers) == 0 {
		return
	}
	limit := nowMS - m.Duration*1000
	for pid, es := range m.Producers {
		i := 0
		for i < len(es) && limit >= 0 && es[i].ID.Ms <= uint64(limit) {
			i++
		}
		if i > 0 && m.index != nil {
			for _, e := range es[:i] {
				delete(m.index[pid], e.IID)
			}
		}
		switch {
		case i == len(es):
			delete(m.Producers, pid)
			if m.index != nil {
				delete(m.index, pid)
			}
		case i > 0:
			m.Producers[pid] = append(es[:0:0], es[i:]...)
		}
	}
}

func (m *StreamIDMP) lookup(pid, iid string) (StreamID, bool) {
	if m.index == nil {
		m.index = make(map[string]map[string]StreamID, len(m.Producers))
		for p, es := range m.Producers {
			ix := make(map[string]StreamID, len(es))
			for _, e := range es {
				ix[e.IID] = e.ID
			}
			m.index[p] = ix
		}
	}
	id, ok := m.index[pid][iid]
	return id, ok
}

// insert remembers iid for pid, forgetting the producer's oldest once it
// holds more than idmp-maxsize.
func (m *StreamIDMP) insert(pid, iid string, id StreamID) {
	if m.Producers == nil {
		m.Producers = make(map[string][]IDMPEntry)
	}
	m.lookup(pid, iid)
	ix := m.index[pid]
	if ix == nil {
		ix = make(map[string]StreamID)
		m.index[pid] = ix
	}
	es := m.Producers[pid]
	es = append(es, IDMPEntry{IID: iid, ID: id})
	ix[iid] = id
	if int64(len(es)) > m.MaxSize {
		delete(ix, es[0].IID)
		es = es[1:]
	}
	m.Producers[pid] = es
	m.Added++
}

// IDMPRequest asks XADD to add the entry once per producer and iid.
type IDMPRequest struct {
	PID, IID string
}

// XAddIDMP is XADD with IDMP: when the producer already added an entry for
// the iid, it returns that entry's ID with duplicate set and adds nothing.
func (v *StreamValue) XAddIDMP(args StreamAddArgs, pairs []StreamField, trim StreamTrimArgs, nowMS uint64, req IDMPRequest) (id StreamID, trimmed int64, duplicate bool, err error) {
	v.mu.Lock()
	defer v.mu.Unlock()
	v.expireIDMPLocked(int64(nowMS))
	m := v.idmpStateLocked()
	if existing, ok := m.lookup(req.PID, req.IID); ok {
		m.Duplicates++
		return existing, 0, true, nil
	}
	entry, err := v.addEntryLocked(args, pairs, nowMS)
	if err != nil {
		return StreamID{}, 0, false, err
	}
	m.insert(req.PID, req.IID, entry.streamID())
	return entry.streamID(), v.trimLocked(trim), false, nil
}

var (
	errIDMPNoMessage = errors.New("ERR No such message in stream")
	errIDMPOtherID   = errors.New("ERR IID already exists for this producer with a different stream ID")
)

// IDMPRecord is XIDMPRECORD: it remembers that pid added entry id for iid.
// changed is false when that was already known.
func (v *StreamValue) IDMPRecord(pid, iid string, id StreamID, nowMS int64) (changed bool, err error) {
	v.mu.Lock()
	defer v.mu.Unlock()
	if v.entryLocked(id) == nil {
		return false, errIDMPNoMessage
	}
	v.expireIDMPLocked(nowMS)
	m := v.idmpStateLocked()
	if existing, ok := m.lookup(pid, iid); ok {
		if existing != id {
			return false, errIDMPOtherID
		}
		return false, nil
	}
	m.insert(pid, iid, id)
	return true, nil
}

// SetIDMPConfig is XCFGSET; a value of -1 is left alone. Changing either
// forgets every remembered iid. It reports whether anything changed.
func (v *StreamValue) SetIDMPConfig(duration, maxSize int64) bool {
	v.mu.Lock()
	defer v.mu.Unlock()
	m := v.idmpStateLocked()
	changed := false
	if duration != -1 && m.Duration != duration {
		m.Duration, m.Producers, m.index, changed = duration, nil, nil, true
	}
	if maxSize != -1 && m.MaxSize != maxSize {
		m.MaxSize, m.Producers, m.index, changed = maxSize, nil, nil, true
	}
	return changed
}

// IDMPInfo is what XINFO STREAM shows of IDMP. InUse reports that the
// stream has used it (XADD IDMP or IDMPAUTO, XCFGSET, XIDMPRECORD).
type IDMPInfo struct {
	InUse             bool
	Duration, MaxSize int64
	Producers, IIDs   int64
	Added, Duplicates int64
}

// IDMPInfoAt returns the stream's IDMP figures at nowMS.
func (v *StreamValue) IDMPInfoAt(nowMS int64) IDMPInfo {
	v.mu.Lock()
	defer v.mu.Unlock()
	v.expireIDMPLocked(nowMS)
	m := v.idmp
	if m == nil {
		return IDMPInfo{Duration: DefaultIDMPDuration, MaxSize: DefaultIDMPMaxSize}
	}
	info := IDMPInfo{InUse: true, Duration: m.Duration, MaxSize: m.MaxSize, Producers: int64(len(m.Producers)), Added: m.Added, Duplicates: m.Duplicates}
	for _, es := range m.Producers {
		info.IIDs += int64(len(es))
	}
	return info
}

// HasEntryLocked reports whether the stream holds entry id. The caller
// holds the stream's lock.
func (v *StreamValue) HasEntryLocked(id StreamID) bool { return v.entryLocked(id) != nil }
