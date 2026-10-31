package persistence

import (
	"bytes"
	"encoding/binary"
	"encoding/gob"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/cachestorm/cachestorm/internal/logger"
	"github.com/cachestorm/cachestorm/internal/store"
)

type RDBVersion int

const (
	RDBVersion9  RDBVersion = 9
	RDBVersion10 RDBVersion = 10
	RDBVersion11 RDBVersion = 11
)

type RDBConfig struct {
	Version     RDBVersion
	Compression bool
	Checksum    bool
	AuxFields   map[string]string
}

type RDBWriter struct {
	config RDBConfig
	store  *store.Store
	dbs    *store.Databases
	mu     sync.Mutex
}

func NewRDBWriter(s *store.Store, cfg RDBConfig) *RDBWriter {
	return &RDBWriter{
		config: cfg,
		store:  s,
	}
}

func NewRDBWriterForDatabases(dbs *store.Databases, cfg RDBConfig) *RDBWriter {
	w := NewRDBWriter(dbs.DB(0), cfg)
	w.dbs = dbs
	return w
}

func (w *RDBWriter) Save(path string) error {
	w.mu.Lock()
	defer w.mu.Unlock()
	return writeRDBAtomic(path, w.writeRDB)
}

func (w *RDBWriter) Encode(dst io.Writer) error {
	w.mu.Lock()
	defer w.mu.Unlock()
	return w.writeRDB(dst)
}

func (w *RDBWriter) writeRDB(f io.Writer) error {
	if w.config.Checksum {
		f = &rdbChecksumWriter{Writer: f}
	}
	if err := w.writeHeader(f); err != nil {
		return err
	}

	if w.dbs == nil {
		if err := w.writeDatabase(f, 0); err != nil {
			return err
		}
	} else {
		for db := 0; db < w.dbs.Count(); db++ {
			if st := w.dbs.Existing(db); st != nil {
				if err := w.writeDatabaseStore(f, db, st); err != nil {
					return err
				}
			}
		}
	}

	if err := w.writeEnd(f); err != nil {
		return err
	}

	return nil
}

func (w *RDBWriter) writeHeader(f io.Writer) error {
	header := fmt.Sprintf("REDIS%04d", w.config.Version)
	if _, err := f.Write([]byte(header)); err != nil {
		return err
	}

	auxFields := []struct {
		key   string
		value string
	}{
		{rdbFormatAux, "1"},
		{"redis-ver", "7.0.0"},
		{"redis-bits", "64"},
		{"ctime", fmt.Sprintf("%d", time.Now().Unix())},
		{"used-mem", fmt.Sprintf("%d", w.store.MemUsage())},
	}

	for key, value := range w.config.AuxFields {
		auxFields = append(auxFields, struct{ key, value string }{key, value})
	}

	for _, aux := range auxFields {
		if err := w.writeAuxField(f, aux.key, aux.value); err != nil {
			return err
		}
	}

	return nil
}

func (w *RDBWriter) writeAuxField(f io.Writer, key, value string) error {
	if err := w.writeByte(f, 0xFA); err != nil {
		return err
	}
	if err := w.writeString(f, key); err != nil {
		return err
	}
	return w.writeString(f, value)
}

func (w *RDBWriter) writeDatabase(f io.Writer, db int) error {
	return w.writeDatabaseStore(f, db, w.store)
}

func (w *RDBWriter) writeDatabaseStore(f io.Writer, db int, st *store.Store) error {
	if err := w.writeByte(f, 0xFE); err != nil {
		return err
	}
	if err := w.writeLength(f, db); err != nil {
		return err
	}

	resizeDB := st.KeyCount()
	if err := w.writeByte(f, 0xFB); err != nil {
		return err
	}
	if err := w.writeLength(f, int(resizeDB)); err != nil {
		return err
	}
	if err := w.writeLength(f, 0); err != nil {
		return err
	}

	return st.ForEachLive(func(key string, entry *store.Entry) error {
		if entry == nil {
			return nil
		}
		return w.writeEntry(f, key, entry)
	})
}

func (w *RDBWriter) writeEntry(f io.Writer, key string, entry *store.Entry) error {
	var hashSnap *hashSnapshot
	if h, ok := entry.Value.(*store.HashValue); ok && h.HasFieldTTLs() {
		snap := snapshotHash(h)
		if len(snap.fields) == 0 {
			return nil
		}
		hashSnap = &snap
	}
	switch entry.Value.(type) {
	case *store.StringValue, *store.ListValue, *store.SetValue, *store.HashValue, *store.SortedSetValue, *store.GeoValue, *store.JSONValue, *store.StreamValue, *store.TimeSeriesValue:
	default:
		if _, ok := entry.Value.(store.RedisStringValue); !ok {
			return fmt.Errorf("unsupported snapshot value %T for key %q", entry.Value, key)
		}
	}
	if len(entry.TagList()) > 0 {
		if err := w.writeByte(f, 0xF8); err != nil {
			return err
		}
		if err := w.writeLength(f, len(entry.TagList())); err != nil {
			return err
		}
		for _, tag := range entry.TagList() {
			if err := w.writeString(f, tag); err != nil {
				return err
			}
		}
	}
	if entry.TTL() > 0 {
		if err := w.writeByte(f, 0xFC); err != nil {
			return err
		}
		expiresAt := entry.ExpiresAt.Load()
		if err := binary.Write(f, binary.LittleEndian, expiresAt); err != nil {
			return err
		}
	}

	valueType := w.getValueType(entry.Value)
	if hashSnap != nil && hashSnap.anyTTL {
		valueType = rdbTypeHashTTL
	}
	if err := w.writeByte(f, byte(valueType)); err != nil {
		return err
	}

	if err := w.writeString(f, key); err != nil {
		return err
	}

	if hashSnap != nil {
		return w.writeHashSnapshot(f, hashSnap)
	}
	return w.writeValue(f, entry.Value, valueType)
}

func (w *RDBWriter) getValueType(v store.Value) int {
	switch v.(type) {
	case *store.StringValue:
		return 0
	case *store.ListValue:
		return 1
	case *store.SetValue:
		return 2
	case *store.HashValue:
		return 3
	case *store.SortedSetValue:
		return 4
	case *store.GeoValue:
		return 5
	case *store.JSONValue:
		return 6
	case *store.StreamValue:
		return 9
	case *store.TimeSeriesValue:
		return 8
	default:
		return 0
	}
}

func (w *RDBWriter) writeValue(f io.Writer, v store.Value, valueType int) error {
	switch vt := v.(type) {
	case *store.StringValue:
		return w.writeString(f, string(vt.Data))
	case *store.ListValue:
		vt.RLock()
		items := make([][]byte, len(vt.Elements))
		copy(items, vt.Elements)
		vt.RUnlock()
		if err := w.writeLength(f, len(items)); err != nil {
			return err
		}
		for _, item := range items {
			if err := w.writeString(f, string(item)); err != nil {
				return err
			}
		}
	case *store.SetValue:
		vt.RLock()
		members := vt.SaveOrder()
		vt.RUnlock()
		if err := w.writeLength(f, len(members)); err != nil {
			return err
		}
		for _, member := range members {
			if err := w.writeString(f, member); err != nil {
				return err
			}
		}
	case *store.HashValue:
		vt.RLock()
		fields := make([][2]string, 0, len(vt.Fields))
		for _, field := range hashFieldOrder(vt) {
			fields = append(fields, [2]string{field, string(vt.Fields[field])})
		}
		vt.RUnlock()
		if err := w.writeLength(f, len(fields)); err != nil {
			return err
		}
		for _, fv := range fields {
			if err := w.writeString(f, fv[0]); err != nil {
				return err
			}
			if err := w.writeString(f, fv[1]); err != nil {
				return err
			}
		}
	case *store.SortedSetValue:
		vt.RLock()
		members := make(map[string]float64, len(vt.Members))
		for m, score := range vt.Members {
			members[m] = score
		}
		vt.RUnlock()
		if err := w.writeLength(f, len(members)); err != nil {
			return err
		}
		for member, score := range members {
			if err := w.writeString(f, member); err != nil {
				return err
			}
			if err := binary.Write(f, binary.LittleEndian, score); err != nil {
				return err
			}
		}
	case *store.GeoValue:
		vt.RLock()
		points := make(map[string]store.GeoPoint, len(vt.Points))
		for member, point := range vt.Points {
			points[member] = point
		}
		vt.RUnlock()
		if err := w.writeLength(f, len(points)); err != nil {
			return err
		}
		for member, point := range points {
			if err := w.writeString(f, member); err != nil {
				return err
			}
			if err := binary.Write(f, binary.LittleEndian, point.Lon); err != nil {
				return err
			}
			if err := binary.Write(f, binary.LittleEndian, point.Lat); err != nil {
				return err
			}
		}
	case *store.JSONValue:
		vt.RLock()
		data := make([]byte, len(vt.Data))
		copy(data, vt.Data)
		vt.RUnlock()
		if err := w.writeLength(f, len(data)); err != nil {
			return err
		}
		if _, err := f.Write(data); err != nil {
			return err
		}
	case *store.StreamValue:
		var data bytes.Buffer
		if err := gob.NewEncoder(&data).Encode(vt.Clone()); err != nil {
			return err
		}
		return w.writeString(f, data.String())
	case *store.TimeSeriesValue:
		vt.RLock()
		retention := vt.Retention
		labels := make(map[string]string, len(vt.Labels))
		for k, val := range vt.Labels {
			labels[k] = val
		}
		samples := make([]store.TimeSeriesSample, len(vt.Samples))
		copy(samples, vt.Samples)
		vt.RUnlock()
		if err := binary.Write(f, binary.LittleEndian, int64(retention)); err != nil {
			return err
		}
		if err := w.writeLength(f, len(labels)); err != nil {
			return err
		}
		for k, val := range labels {
			if err := w.writeString(f, k); err != nil {
				return err
			}
			if err := w.writeString(f, val); err != nil {
				return err
			}
		}
		if err := w.writeLength(f, len(samples)); err != nil {
			return err
		}
		for _, sample := range samples {
			if err := binary.Write(f, binary.LittleEndian, sample.Timestamp); err != nil {
				return err
			}
			if err := binary.Write(f, binary.LittleEndian, sample.Value); err != nil {
				return err
			}
			if err := w.writeLength(f, len(sample.Labels)); err != nil {
				return err
			}
			for k, val := range sample.Labels {
				if err := w.writeString(f, k); err != nil {
					return err
				}
				if err := w.writeString(f, val); err != nil {
					return err
				}
			}
		}
	default:
		if raw, ok := v.(store.RedisStringValue); ok {
			data, err := raw.RedisStringBytes()
			if err != nil {
				return err
			}
			return w.writeString(f, string(data))
		}
		return fmt.Errorf("unsupported value type %T in RDB snapshot", v)
	}
	return nil
}

func (w *RDBWriter) writeEnd(f io.Writer) error {
	if err := w.writeByte(f, 0xFF); err != nil {
		return err
	}

	checksum := make([]byte, 8)
	if cw, ok := f.(*rdbChecksumWriter); ok {
		binary.LittleEndian.PutUint64(checksum, cw.checksum)
		f = cw.Writer
	}
	_, err := f.Write(checksum)
	return err
}

func (w *RDBWriter) writeByte(f io.Writer, b byte) error {
	_, err := f.Write([]byte{b})
	return err
}

// WriteRDBLength writes an RDB length value using the canonical length
// encoding: < 64 → 1 byte; < 16384 → 2 bytes (0x40 marker); else 4 bytes
// (0x80 marker). Exported so replication-side RDB emitters produce bytes
// that RDBReader parses symmetrically instead of hand-rolling raw bytes.
func WriteRDBLength(w io.Writer, length int) error {
	if length < 64 {
		_, err := w.Write([]byte{byte(length)})
		return err
	} else if length < 16384 {
		buf := make([]byte, 2)
		buf[0] = byte((length >> 8) | 0x40)
		buf[1] = byte(length & 0xFF)
		_, err := w.Write(buf)
		return err
	}
	buf := make([]byte, 4)
	buf[0] = byte((length >> 24) | 0x80)
	buf[1] = byte((length >> 16) & 0xFF)
	buf[2] = byte((length >> 8) & 0xFF)
	buf[3] = byte(length & 0xFF)
	_, err := w.Write(buf)
	return err
}

func (w *RDBWriter) writeLength(f io.Writer, length int) error {
	return WriteRDBLength(f, length)
}

func (w *RDBWriter) writeString(f io.Writer, s string) error {
	length := len(s)
	if err := w.writeLength(f, length); err != nil {
		return err
	}
	_, err := f.Write([]byte(s))
	return err
}

type RDBReader struct {
	AuxFields map[string]string
	store     *store.Store
	dbs       *store.Databases
	mu        sync.Mutex
	// body is the file Load read, which a Redis file's values are decoded
	// from in place; redis is set once the aux fields show the file was
	// written by Redis, whose type numbers and opcodes differ from
	// CacheStorm's; functions are the library codes a Redis file holds.
	body      []byte
	redis     bool
	functions []string
}

func NewRDBReader(s *store.Store) *RDBReader {
	return &RDBReader{
		store: s,
	}
}

func NewRDBReaderForDatabases(dbs *store.Databases) *RDBReader {
	r := NewRDBReader(dbs.DB(0))
	r.dbs = dbs
	return r
}

func (r *RDBReader) Load(path string) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	data, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("failed to open file: %w", err)
	}
	if err := r.loadBytes(data); err != nil {
		return err
	}
	logger.Info().Str("path", path).Msg("RDB loaded successfully")
	return nil
}

// LoadBytes loads an RDB file held in memory, such as the one a master
// sends a replica.
func (r *RDBReader) LoadBytes(data []byte) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.loadBytes(data)
}

func (r *RDBReader) loadBytes(data []byte) error {
	if len(data) < 18 {
		return r.readRDB(bytes.NewReader(data))
	}
	body := data[:len(data)-8]
	checksum := binary.LittleEndian.Uint64(data[len(data)-8:])
	if checksum != 0 && checksum != rdbChecksum(0, body) {
		return fmt.Errorf("RDB checksum mismatch")
	}
	if r.dbs != nil {
		r.dbs.FlushAll()
		r.store = r.dbs.DB(0)
	}
	r.body = body
	defer func() { r.body = nil }()
	return r.readRDB(bytes.NewReader(body))
}

func (r *RDBReader) readRDB(f io.Reader) error {
	header := make([]byte, 9)
	if _, err := io.ReadFull(f, header); err != nil {
		return fmt.Errorf("failed to read header: %v", err)
	}

	if !strings.HasPrefix(string(header), "REDIS") {
		return fmt.Errorf("invalid RDB file format")
	}

	versionStr := string(header[5:9])
	version, err := strconv.Atoi(versionStr)
	if err != nil {
		return fmt.Errorf("invalid RDB version: %v", err)
	}

	if version < 5 || version > 12 {
		return fmt.Errorf("unsupported RDB version: %d", version)
	}

	r.AuxFields = make(map[string]string)
	r.redis, r.functions = false, nil
	decided := false

	// Expiry (unix ms) from a preceding 0xFC/0xFD opcode, and in a Redis
	// file the key's idle time (0xF8) or LFU counter (0xF9); they apply to
	// the next entry only.
	var pendingExpiryMS int64
	var pendingTags []string
	pendingIdle, pendingFreq := int64(-1), -1

	for {
		opcode, err := r.readByte(f)
		if err != nil {
			if err == io.EOF {
				return io.ErrUnexpectedEOF
			}
			return err
		}
		if !decided {
			decided = true
			r.redis = writtenByRedis(version, opcode, f)
		}
		if r.redis {
			switch opcode {
			case 0xF8:
				idle, err := r.readLength(f)
				if err != nil {
					return err
				}
				pendingIdle = int64(idle)
				continue
			case 0xF9:
				freq, err := r.readByte(f)
				if err != nil {
					return err
				}
				pendingFreq = int(freq)
				continue
			case rdbOpcodeFunction2:
				code, err := r.readString(f)
				if err != nil {
					return err
				}
				r.functions = append(r.functions, code)
				continue
			case rdbOpcodeFunctionPreGA:
				return fmt.Errorf("pre-GA function format not supported")
			case 0xF7:
				return fmt.Errorf("RDB module data is not supported")
			case 0xFA:
				key, err := r.readString(f)
				if err != nil {
					return err
				}
				value, err := r.readString(f)
				if err != nil {
					return err
				}
				if key != rdbTagsAux {
					r.AuxFields[key] = value
					continue
				}
				d := &dumpDecoder{data: []byte(value)}
				pendingTags = make([]string, d.count())
				for i := range pendingTags {
					pendingTags[i] = d.str()
				}
				if d.err != nil || d.pos != len(d.data) {
					return fmt.Errorf("corrupt %s aux field", rdbTagsAux)
				}
				continue
			case 0xFF:
				if len(r.functions) > 0 {
					r.AuxFields["cachestorm-functions"] = string(EncodeFunctionDump(r.functions))
				}
			case 0xFB, 0xFC, 0xFD, 0xFE:
			default:
				if err := r.readRedisEntry(f, opcode, pendingExpiryMS, pendingIdle, pendingFreq, pendingTags); err != nil {
					return err
				}
				pendingExpiryMS, pendingIdle, pendingFreq, pendingTags = 0, -1, -1, nil
				continue
			}
		}

		switch opcode {
		case 0xFA:
			key, err := r.readString(f)
			if err != nil {
				return err
			}
			value, err := r.readString(f)
			if err != nil {
				return err
			}
			r.AuxFields[key] = value

		case 0xFB:
			_, err := r.readLength(f)
			if err != nil {
				return err
			}
			_, err = r.readLength(f)
			if err != nil {
				return err
			}

		case 0xFE:
			db, err := r.readLength(f)
			if err != nil {
				return err
			}
			if r.dbs != nil {
				if !r.dbs.Valid(db) {
					return fmt.Errorf("RDB database %d is out of range", db)
				}
				r.store = r.dbs.DB(db)
			} else if db != 0 {
				return fmt.Errorf("RDB database %d requires a multi-database reader", db)
			}
			r.store.Flush()
		case 0xF8:
			count, err := r.readLength(f)
			if err != nil {
				return err
			}
			pendingTags = make([]string, count)
			for i := range pendingTags {
				pendingTags[i], err = r.readString(f)
				if err != nil {
					return err
				}
			}

		case 0xFC:
			var expiresAt int64
			if err := binary.Read(f, binary.LittleEndian, &expiresAt); err != nil {
				return err
			}
			pendingExpiryMS = expiresAt

		case 0xFD:
			var expiresAt uint32
			if err := binary.Read(f, binary.LittleEndian, &expiresAt); err != nil {
				return err
			}
			pendingExpiryMS = int64(expiresAt) * 1000

		case 0xFF:
			var extra [1]byte
			if _, err := io.ReadFull(f, extra[:]); err != io.EOF {
				return fmt.Errorf("RDB trailing data after EOF")
			}
			return nil

		default:
			if err := r.readEntry(f, opcode, pendingExpiryMS, pendingTags); err != nil {
				return err
			}
			pendingExpiryMS = 0
			pendingTags = nil
		}
	}
}

func (r *RDBReader) readEntry(f io.Reader, valueType byte, expiresAtMS int64, tags ...[]string) error {
	key, err := r.readString(f)
	if err != nil {
		return err
	}

	var value store.Value

	switch valueType {
	case 0:
		strVal, err := r.readString(f)
		if err != nil {
			return err
		}
		value = &store.StringValue{Data: []byte(strVal)}

	case 1:
		length, err := r.readLength(f)
		if err != nil {
			return err
		}
		items := make([][]byte, length)
		for i := 0; i < length; i++ {
			item, err := r.readString(f)
			if err != nil {
				return err
			}
			items[i] = []byte(item)
		}
		value = &store.ListValue{Elements: items}

	case 2:
		length, err := r.readLength(f)
		if err != nil {
			return err
		}
		members := make([]string, 0, length)
		for i := 0; i < length; i++ {
			member, err := r.readString(f)
			if err != nil {
				return err
			}
			members = append(members, member)
		}
		value = store.NewLoadedSet(members)

	case 3:
		length, err := r.readLength(f)
		if err != nil {
			return err
		}
		fields := make(map[string][]byte)
		for i := 0; i < length; i++ {
			field, err := r.readString(f)
			if err != nil {
				return err
			}
			val, err := r.readString(f)
			if err != nil {
				return err
			}
			fields[field] = []byte(val)
		}
		value = &store.HashValue{Fields: fields}

	case rdbTypeHashTTL:
		length, err := r.readLength(f)
		if err != nil {
			return err
		}
		fields := make([]string, 0, length)
		values := make([][]byte, 0, length)
		ttls := make([]int64, 0, length)
		for i := 0; i < length; i++ {
			field, err := r.readString(f)
			if err != nil {
				return err
			}
			val, err := r.readString(f)
			if err != nil {
				return err
			}
			var at int64
			if err := binary.Read(f, binary.LittleEndian, &at); err != nil {
				return err
			}
			fields = append(fields, field)
			values = append(values, []byte(val))
			ttls = append(ttls, at)
		}
		h := newHashWithTTLs(fields, values, ttls)
		if h == nil {
			return nil
		}
		value = h

	case 4:
		length, err := r.readLength(f)
		if err != nil {
			return err
		}
		members := make(map[string]float64, length)
		for i := 0; i < length; i++ {
			member, err := r.readString(f)
			if err != nil {
				return err
			}
			var score float64
			if err := binary.Read(f, binary.LittleEndian, &score); err != nil {
				return err
			}
			members[member] = score
		}
		value = &store.SortedSetValue{Members: members}

	case 5:
		length, err := r.readLength(f)
		if err != nil {
			return err
		}
		points := make(map[string]store.GeoPoint, length)
		for i := 0; i < length; i++ {
			member, err := r.readString(f)
			if err != nil {
				return err
			}
			var point store.GeoPoint
			if err := binary.Read(f, binary.LittleEndian, &point.Lon); err != nil {
				return err
			}
			if err := binary.Read(f, binary.LittleEndian, &point.Lat); err != nil {
				return err
			}
			points[member] = point
		}
		legacy := &store.GeoValue{Points: points}
		if canonical, ok := legacy.SortedSet(); ok {
			value = canonical
		} else {
			value = legacy
		}

	case 6:
		data, err := r.readString(f)
		if err != nil {
			return err
		}
		value = &store.JSONValue{Data: []byte(data)}

	case 7:
		length, err := r.readLength(f)
		if err != nil {
			return err
		}
		maxLen, err := r.readLength(f)
		if err != nil {
			return err
		}
		entries := make([]*store.StreamEntry, 0, length)
		lastID := ""
		for i := 0; i < length; i++ {
			id, err := r.readString(f)
			if err != nil {
				return err
			}
			fieldCount, err := r.readLength(f)
			if err != nil {
				return err
			}
			fields := make(map[string][]byte, fieldCount)
			for j := 0; j < fieldCount; j++ {
				k, err := r.readString(f)
				if err != nil {
					return err
				}
				val, err := r.readString(f)
				if err != nil {
					return err
				}
				fields[k] = []byte(val)
			}
			entries = append(entries, &store.StreamEntry{ID: id, Fields: fields})
			lastID = id
		}
		value = &store.StreamValue{
			Entries: entries,
			LastID:  lastID,
			Length:  int64(len(entries)),
			MaxLen:  int64(maxLen),
			Groups:  make(map[string]*store.ConsumerGroup),
		}

	case 9:
		data, err := r.readString(f)
		if err != nil {
			return err
		}
		stream := store.NewStreamValue(0)
		if err := gob.NewDecoder(strings.NewReader(data)).Decode(stream); err != nil {
			return err
		}
		value = stream

	case 8:
		var retention int64
		if err := binary.Read(f, binary.LittleEndian, &retention); err != nil {
			return err
		}
		labelCount, err := r.readLength(f)
		if err != nil {
			return err
		}
		labels := make(map[string]string, labelCount)
		for i := 0; i < labelCount; i++ {
			k, err := r.readString(f)
			if err != nil {
				return err
			}
			val, err := r.readString(f)
			if err != nil {
				return err
			}
			labels[k] = val
		}
		sampleCount, err := r.readLength(f)
		if err != nil {
			return err
		}
		ts := store.NewTimeSeriesValue(time.Duration(retention))
		ts.Labels = labels
		for i := 0; i < sampleCount; i++ {
			var sample store.TimeSeriesSample
			if err := binary.Read(f, binary.LittleEndian, &sample.Timestamp); err != nil {
				return err
			}
			if err := binary.Read(f, binary.LittleEndian, &sample.Value); err != nil {
				return err
			}
			sampleLabelCount, err := r.readLength(f)
			if err != nil {
				return err
			}
			if sampleLabelCount > 0 {
				sample.Labels = make(map[string]string, sampleLabelCount)
				for j := 0; j < sampleLabelCount; j++ {
					k, err := r.readString(f)
					if err != nil {
						return err
					}
					val, err := r.readString(f)
					if err != nil {
						return err
					}
					sample.Labels[k] = val
				}
			}
			ts.Samples = append(ts.Samples, sample)
		}
		value = ts

	default:
		strVal, err := r.readString(f)
		if err != nil {
			return err
		}
		value = &store.StringValue{Data: []byte(strVal)}
	}

	// Restore expiry from a preceding 0xFC (ms) / 0xFD (s) opcode, if any.
	opts := store.SetOptions{}
	if len(tags) > 0 {
		opts.Tags = tags[0]
	}
	if expiresAtMS > 0 {
		if expiresAtMS <= time.Now().UnixMilli() {
			return nil
		}
		opts.ExpireAtMS = expiresAtMS
	}

	return r.store.Set(key, value, opts)
}

// rdbFormatAux is the aux field CacheStorm's snapshots start with: their
// type numbers, opcodes and long lengths differ from Redis' RDB format.
const rdbFormatAux = "cachestorm-rdb"

// writtenByRedis reports, from the file's first opcode, whether an RDB file
// came from Redis rather than CacheStorm. Both start with an aux field;
// CacheStorm's is rdbFormatAux, or, before it wrote one, redis-ver 7.0.0 in
// a version Redis 7.0.0 did not write (it wrote 10). A file without aux
// fields is CacheStorm's. f is not consumed.
func writtenByRedis(version int, opcode byte, f io.Reader) bool {
	br, ok := f.(*bytes.Reader)
	if opcode != 0xFA || !ok {
		return false
	}
	pos := br.Size() - int64(br.Len())
	defer br.Seek(pos, io.SeekStart) //nolint:errcheck // seeking back within the buffer
	key, err := readPlainString(br)
	if err != nil || key == rdbFormatAux {
		return false
	}
	value, err := readPlainString(br)
	return err != nil || key != "redis-ver" || value != "7.0.0" || version == 10
}

// readPlainString reads a short length-prefixed string, as both formats
// write the first aux field.
func readPlainString(br *bytes.Reader) (string, error) {
	n, err := br.ReadByte()
	if err != nil || n >= 64 {
		return "", fmt.Errorf("not a short string")
	}
	buf := make([]byte, n)
	_, err = io.ReadFull(br, buf)
	return string(buf), err
}

// readRedisEntry loads a key of a Redis file: its value is in Redis' RDB
// encoding, which the DUMP payload decoder reads in place.
func (r *RDBReader) readRedisEntry(f io.Reader, valueType byte, expiresAtMS, idle int64, freq int, tags []string) error {
	br, ok := f.(*bytes.Reader)
	if !ok || r.body == nil {
		return fmt.Errorf("a Redis RDB file must be loaded with Load")
	}
	key, err := r.readString(f)
	if err != nil {
		return err
	}
	d := &dumpDecoder{data: r.body, pos: len(r.body) - br.Len()}
	var value store.Value
	switch valueType {
	case rdbTypeModule2:
		value = d.moduleValue()
		if value == nil && d.err == nil {
			return fmt.Errorf("RDB module values are not supported (key %q)", key)
		}
	case 6:
		return fmt.Errorf("RDB module values are not supported (key %q)", key)
	case 22, 23:
		return fmt.Errorf("pre-GA hash field expiration format not supported (key %q)", key)
	default:
		value = d.valueOf(valueType)
	}
	if d.allExpired {
		_, err := br.Seek(int64(d.pos), io.SeekStart)
		return err
	}
	if d.err != nil || value == nil {
		return fmt.Errorf("RDB value of key %q (type %d) is corrupt or unsupported", key, valueType)
	}
	if _, err := br.Seek(int64(d.pos), io.SeekStart); err != nil {
		return err
	}
	opts := store.SetOptions{Tags: tags}
	if expiresAtMS > 0 {
		if expiresAtMS <= time.Now().UnixMilli() {
			return nil
		}
		opts.ExpireAtMS = expiresAtMS
	}
	if err := r.store.Set(key, value, opts); err != nil {
		return err
	}
	if entry, ok := r.store.GetShard(key).Get(key); ok {
		if freq >= 0 && r.store.UsesLFU() {
			entry.SetLFUCount(uint8(freq))
		}
		if idle >= 0 && !r.store.UsesLFU() {
			entry.SetIdleSeconds(min(idle, int64(1<<24)-1))
		}
	}
	return nil
}

func (r *RDBReader) readByte(f io.Reader) (byte, error) {
	buf := make([]byte, 1)
	_, err := io.ReadFull(f, buf)
	return buf[0], err
}

func (r *RDBReader) readLength(f io.Reader) (int, error) {
	b, err := r.readByte(f)
	if err != nil {
		return 0, err
	}

	encType := (b & 0xC0) >> 6

	switch encType {
	case 0:
		return int(b & 0x3F), nil
	case 1:
		b2, err := r.readByte(f)
		if err != nil {
			return 0, err
		}
		return int(b&0x3F)<<8 | int(b2), nil
	case 2:
		if r.redis {
			size := 4
			if b == 0x81 {
				size = 8
			} else if b != 0x80 {
				return 0, fmt.Errorf("unsupported length encoding: %#x", b)
			}
			buf := make([]byte, size)
			if _, err := io.ReadFull(f, buf); err != nil {
				return 0, err
			}
			var n uint64
			for _, c := range buf {
				n = n<<8 | uint64(c)
			}
			if n > maxDumpString {
				return 0, fmt.Errorf("RDB length %d out of range", n)
			}
			return int(n), nil
		}
		// The length byte carries the 0x80 marker plus the top 7 bits of the
		// length; three more bytes follow (4 bytes total, matching the writer).
		buf := make([]byte, 3)
		if _, err := io.ReadFull(f, buf); err != nil {
			return 0, err
		}
		return int(b&0x7F)<<24 | int(buf[0])<<16 | int(buf[1])<<8 | int(buf[2]), nil
	default:
		return 0, fmt.Errorf("unsupported length encoding: %d", encType)
	}
}

func (r *RDBReader) readString(f io.Reader) (string, error) {
	if r.redis {
		if br, ok := f.(*bytes.Reader); ok && r.body != nil {
			// Redis encodes strings as integers or LZF; the DUMP decoder
			// reads them in place.
			d := &dumpDecoder{data: r.body, pos: len(r.body) - br.Len()}
			s := d.str()
			if d.err != nil {
				return "", d.err
			}
			_, err := br.Seek(int64(d.pos), io.SeekStart)
			return s, err
		}
	}
	length, err := r.readLength(f)
	if err != nil {
		return "", err
	}

	buf := make([]byte, length)
	if _, err := io.ReadFull(f, buf); err != nil {
		return "", err
	}

	return string(buf), nil
}

type PersistenceManager struct {
	store     *store.Store
	config    Config
	rdbWriter *RDBWriter
	rdbReader *RDBReader
	lastSave  time.Time
	dirty     int64
	stopCh    chan struct{}
	wg        sync.WaitGroup
	mu        sync.Mutex
}

type Config struct {
	DataDir        string
	RDBEnabled     bool
	RDBFilename    string
	RDBInterval    time.Duration
	AOFRewriteSize int64
}

func NewPersistenceManager(s *store.Store, cfg Config) *PersistenceManager {
	pm := &PersistenceManager{
		store:  s,
		config: cfg,
		stopCh: make(chan struct{}),
	}

	pm.rdbWriter = NewRDBWriter(s, RDBConfig{
		Version:     RDBVersion11,
		Compression: false,
		Checksum:    true,
	})
	pm.rdbReader = NewRDBReader(s)

	return pm
}

func (pm *PersistenceManager) Start() error {
	if pm.config.RDBEnabled {
		pm.wg.Add(1)
		go pm.autoSaveLoop()
	}

	if pm.config.RDBFilename != "" {
		rdbPath := filepath.Join(pm.config.DataDir, pm.config.RDBFilename)
		if _, err := os.Stat(rdbPath); err == nil {
			if err := pm.rdbReader.Load(rdbPath); err != nil {
				logger.Error().Err(err).Msg("Failed to load RDB file")
			}
		}
	}

	return nil
}

func (pm *PersistenceManager) Stop() {
	close(pm.stopCh)
	pm.wg.Wait()

	pm.mu.Lock()
	if pm.dirty > 0 {
		pm.saveRDB()
	}
	pm.mu.Unlock()
}

func (pm *PersistenceManager) autoSaveLoop() {
	defer pm.wg.Done()

	interval := pm.config.RDBInterval
	if interval == 0 {
		interval = 5 * time.Minute
	}

	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	for {
		select {
		case <-pm.stopCh:
			return
		case <-ticker.C:
			pm.mu.Lock()
			if pm.dirty > 0 {
				pm.saveRDB()
			}
			pm.mu.Unlock()
		}
	}
}

func (pm *PersistenceManager) saveRDB() {
	if pm.config.RDBFilename == "" {
		return
	}

	rdbPath := filepath.Join(pm.config.DataDir, pm.config.RDBFilename)
	if err := pm.rdbWriter.Save(rdbPath); err != nil {
		logger.Error().Err(err).Msg("Failed to save RDB file")
		return
	}

	pm.dirty = 0
	pm.lastSave = time.Now()
}

func (pm *PersistenceManager) BGSAVE() error {
	pm.mu.Lock()
	defer pm.mu.Unlock()

	go func() {
		// saveRDB touches pm.dirty/pm.lastSave; take the manager lock so the
		// async save serializes with Stop/autoSaveLoop/SAVE instead of racing
		// their mu-held accesses.
		pm.mu.Lock()
		defer pm.mu.Unlock()
		pm.saveRDB()
	}()
	return nil
}

func (pm *PersistenceManager) SAVE() error {
	pm.mu.Lock()
	defer pm.mu.Unlock()

	return pm.rdbWriter.Save(filepath.Join(pm.config.DataDir, pm.config.RDBFilename))
}

func (pm *PersistenceManager) LastSave() time.Time {
	pm.mu.Lock()
	defer pm.mu.Unlock()
	return pm.lastSave
}

func (pm *PersistenceManager) Dirty() int64 {
	pm.mu.Lock()
	defer pm.mu.Unlock()
	return pm.dirty
}

func (pm *PersistenceManager) MarkDirty() {
	pm.mu.Lock()
	defer pm.mu.Unlock()
	pm.dirty++
}

// writeHashSnapshot writes a hash as type 3, or as rdbTypeHashTTL when a
// field has an expiry.
func (w *RDBWriter) writeHashSnapshot(f io.Writer, snap *hashSnapshot) error {
	if err := w.writeLength(f, len(snap.fields)); err != nil {
		return err
	}
	for i, field := range snap.fields {
		if err := w.writeString(f, field); err != nil {
			return err
		}
		if err := w.writeString(f, string(snap.values[i])); err != nil {
			return err
		}
		if snap.anyTTL {
			if err := binary.Write(f, binary.LittleEndian, snap.ttls[i]); err != nil {
				return err
			}
		}
	}
	return nil
}
