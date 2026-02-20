package persistence

import (
	"encoding/binary"
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
}

type RDBWriter struct {
	config RDBConfig
	store  *store.Store
	mu     sync.Mutex
}

func NewRDBWriter(s *store.Store, cfg RDBConfig) *RDBWriter {
	return &RDBWriter{
		config: cfg,
		store:  s,
	}
}

func (w *RDBWriter) Save(path string) error {
	w.mu.Lock()
	defer w.mu.Unlock()

	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("failed to create directory: %v", err)
	}

	tempPath := path + ".tmp"

	f, err := os.Create(tempPath)
	if err != nil {
		return fmt.Errorf("failed to create file: %v", err)
	}
	defer f.Close()

	if err := w.writeRDB(f); err != nil {
		os.Remove(tempPath)
		return err
	}

	if err := f.Sync(); err != nil {
		os.Remove(tempPath)
		return fmt.Errorf("failed to sync file: %v", err)
	}

	if err := os.Rename(tempPath, path); err != nil {
		os.Remove(tempPath)
		return fmt.Errorf("failed to rename file: %v", err)
	}

	logger.Info().Str("path", path).Msg("RDB saved successfully")
	return nil
}

func (w *RDBWriter) writeRDB(f io.Writer) error {
	if err := w.writeHeader(f); err != nil {
		return err
	}

	if err := w.writeDatabase(f, 0); err != nil {
		return err
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
		{"redis-ver", "7.0.0"},
		{"redis-bits", "64"},
		{"ctime", fmt.Sprintf("%d", time.Now().Unix())},
		{"used-mem", fmt.Sprintf("%d", w.store.MemUsage())},
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
	if err := w.writeByte(f, 0xFE); err != nil {
		return err
	}
	if err := w.writeLength(f, db); err != nil {
		return err
	}

	resizeDB := w.store.KeyCount()
	if err := w.writeByte(f, 0xFB); err != nil {
		return err
	}
	if err := w.writeLength(f, int(resizeDB)); err != nil {
		return err
	}
	if err := w.writeLength(f, 0); err != nil {
		return err
	}

	entries := w.store.GetAll()
	for key, entry := range entries {
		if entry == nil || entry.IsExpired() {
			continue
		}

		if err := w.writeEntry(f, key, entry); err != nil {
			return err
		}
	}

	return nil
}

func (w *RDBWriter) writeEntry(f io.Writer, key string, entry *store.Entry) error {
	if entry.TTL() > 0 {
		if err := w.writeByte(f, 0xFC); err != nil {
			return err
		}
		expiresAt := time.Unix(0, entry.ExpiresAt).UnixMilli()
		if err := binary.Write(f, binary.LittleEndian, expiresAt); err != nil {
			return err
		}
	}

	valueType := w.getValueType(entry.Value)
	if err := w.writeByte(f, byte(valueType)); err != nil {
		return err
	}

	if err := w.writeString(f, key); err != nil {
		return err
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
	default:
		return 0
	}
}

func (w *RDBWriter) writeValue(f io.Writer, v store.Value, valueType int) error {
	switch vt := v.(type) {
	case *store.StringValue:
		return w.writeString(f, string(vt.Data))
	case *store.ListValue:
		items := vt.Elements
		if err := w.writeLength(f, len(items)); err != nil {
			return err
		}
		for _, item := range items {
			if err := w.writeString(f, string(item)); err != nil {
				return err
			}
		}
	case *store.SetValue:
		members := vt.Members
		if err := w.writeLength(f, len(members)); err != nil {
			return err
		}
		for member := range members {
			if err := w.writeString(f, member); err != nil {
				return err
			}
		}
	case *store.HashValue:
		fields := vt.Fields
		if err := w.writeLength(f, len(fields)); err != nil {
			return err
		}
		for field, value := range fields {
			if err := w.writeString(f, field); err != nil {
				return err
			}
			if err := w.writeString(f, string(value)); err != nil {
				return err
			}
		}
	default:
		return w.writeString(f, v.String())
	}
	return nil
}

func (w *RDBWriter) writeEnd(f io.Writer) error {
	if err := w.writeByte(f, 0xFF); err != nil {
		return err
	}

	checksum := make([]byte, 8)
	_, err := f.Write(checksum)
	return err
}

func (w *RDBWriter) writeByte(f io.Writer, b byte) error {
	_, err := f.Write([]byte{b})
	return err
}

func (w *RDBWriter) writeLength(f io.Writer, length int) error {
	if length < 64 {
		return w.writeByte(f, byte(length))
	} else if length < 16384 {
		buf := make([]byte, 2)
		buf[0] = byte((length >> 8) | 0x40)
		buf[1] = byte(length & 0xFF)
		_, err := f.Write(buf)
		return err
	} else {
		buf := make([]byte, 4)
		buf[0] = byte((length >> 24) | 0x80)
		buf[1] = byte((length >> 16) & 0xFF)
		buf[2] = byte((length >> 8) & 0xFF)
		buf[3] = byte(length & 0xFF)
		_, err := f.Write(buf)
		return err
	}
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
	store *store.Store
	mu    sync.Mutex
}

func NewRDBReader(s *store.Store) *RDBReader {
	return &RDBReader{
		store: s,
	}
}

func (r *RDBReader) Load(path string) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	f, err := os.Open(path)
	if err != nil {
		return fmt.Errorf("failed to open file: %v", err)
	}
	defer f.Close()

	if err := r.readRDB(f); err != nil {
		return err
	}

	logger.Info().Str("path", path).Msg("RDB loaded successfully")
	return nil
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

	if version < 5 || version > 11 {
		return fmt.Errorf("unsupported RDB version: %d", version)
	}

	for {
		opcode, err := r.readByte(f)
		if err != nil {
			if err == io.EOF {
				break
			}
			return err
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
			_ = key
			_ = value

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
			r.store.Flush()

		case 0xFC:
			var expiresAt int64
			if err := binary.Read(f, binary.LittleEndian, &expiresAt); err != nil {
				return err
			}
			_ = expiresAt

		case 0xFD:
			var expiresAt uint32
			if err := binary.Read(f, binary.LittleEndian, &expiresAt); err != nil {
				return err
			}
			_ = expiresAt

		case 0xFF:
			return nil

		default:
			if err := r.readEntry(f, opcode); err != nil {
				return err
			}
		}
	}

	return nil
}

func (r *RDBReader) readEntry(f io.Reader, valueType byte) error {
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
		members := make(map[string]struct{})
		for i := 0; i < length; i++ {
			member, err := r.readString(f)
			if err != nil {
				return err
			}
			members[member] = struct{}{}
		}
		value = &store.SetValue{Members: members}

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

	default:
		strVal, err := r.readString(f)
		if err != nil {
			return err
		}
		value = &store.StringValue{Data: []byte(strVal)}
	}

	r.store.Set(key, value, store.SetOptions{})
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
		buf := make([]byte, 4)
		if _, err := io.ReadFull(f, buf); err != nil {
			return 0, err
		}
		return int(buf[0])<<24 | int(buf[1])<<16 | int(buf[2])<<8 | int(buf[3]), nil
	default:
		return 0, fmt.Errorf("unsupported length encoding: %d", encType)
	}
}

func (r *RDBReader) readString(f io.Reader) (string, error) {
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

	go pm.saveRDB()
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
