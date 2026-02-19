package persistence

import (
	"bufio"
	"encoding/binary"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/cachestorm/cachestorm/internal/command"
	"github.com/cachestorm/cachestorm/internal/logger"
	"github.com/cachestorm/cachestorm/internal/plugin"
	"github.com/cachestorm/cachestorm/internal/store"
)

type AOFWriter struct {
	mu       sync.Mutex
	file     *os.File
	writer   *bufio.Writer
	syncMode string
	enabled  bool
	dataDir  string
}

func NewAOFWriter(dataDir string, syncMode string, enabled bool) *AOFWriter {
	return &AOFWriter{
		syncMode: syncMode,
		enabled:  enabled,
		dataDir:  dataDir,
	}
}

func (a *AOFWriter) Open() error {
	if !a.enabled {
		return nil
	}

	if err := os.MkdirAll(a.dataDir, 0755); err != nil {
		return err
	}

	path := filepath.Join(a.dataDir, "appendonly.aof")
	f, err := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return err
	}

	a.file = f
	a.writer = bufio.NewWriter(f)
	return nil
}

func (a *AOFWriter) Append(cmd string, args [][]byte) error {
	if !a.enabled || a.file == nil {
		return nil
	}

	a.mu.Lock()
	defer a.mu.Unlock()

	a.writer.WriteByte('*')
	a.writer.WriteString(fmt.Sprintf("%d", len(args)+1))
	a.writer.WriteString("\r\n")

	a.writer.WriteByte('$')
	a.writer.WriteString(fmt.Sprintf("%d", len(cmd)))
	a.writer.WriteString("\r\n")
	a.writer.WriteString(cmd)
	a.writer.WriteString("\r\n")

	for _, arg := range args {
		a.writer.WriteByte('$')
		a.writer.WriteString(fmt.Sprintf("%d", len(arg)))
		a.writer.WriteString("\r\n")
		a.writer.Write(arg)
		a.writer.WriteString("\r\n")
	}

	switch a.syncMode {
	case "always":
		a.writer.Flush()
		a.file.Sync()
	case "everysec":
		go func() {
			time.Sleep(time.Second)
			a.mu.Lock()
			a.writer.Flush()
			a.file.Sync()
			a.mu.Unlock()
		}()
	}

	return nil
}

func (a *AOFWriter) Flush() error {
	if !a.enabled || a.writer == nil {
		return nil
	}
	return a.writer.Flush()
}

func (a *AOFWriter) Close() error {
	if !a.enabled || a.file == nil {
		return nil
	}
	a.writer.Flush()
	return a.file.Close()
}

type SnapshotWriter struct {
	mu      sync.Mutex
	dataDir string
	enabled bool
}

func NewSnapshotWriter(dataDir string, enabled bool) *SnapshotWriter {
	return &SnapshotWriter{
		dataDir: dataDir,
		enabled: enabled,
	}
}

const snapshotMagic = "CSDB"
const snapshotVersion = 1

func (s *SnapshotWriter) Save(st *store.Store) error {
	if !s.enabled {
		return nil
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	if err := os.MkdirAll(s.dataDir, 0755); err != nil {
		return err
	}

	path := filepath.Join(s.dataDir, "dump.snapshot")
	tmpPath := path + ".tmp"

	f, err := os.Create(tmpPath)
	if err != nil {
		return err
	}
	defer f.Close()

	if err := binary.Write(f, binary.LittleEndian, []byte(snapshotMagic)); err != nil {
		return err
	}
	if err := binary.Write(f, binary.LittleEndian, uint32(snapshotVersion)); err != nil {
		return err
	}
	if err := binary.Write(f, binary.LittleEndian, uint64(time.Now().Unix())); err != nil {
		return err
	}

	keys := st.Keys()
	if err := binary.Write(f, binary.LittleEndian, uint64(len(keys))); err != nil {
		return err
	}

	for _, key := range keys {
		entry, exists := st.Get(key)
		if !exists {
			continue
		}

		keyBytes := []byte(key)
		if err := binary.Write(f, binary.LittleEndian, uint32(len(keyBytes))); err != nil {
			return err
		}
		if _, err := f.Write(keyBytes); err != nil {
			return err
		}

		if err := binary.Write(f, binary.LittleEndian, uint8(entry.Value.Type())); err != nil {
			return err
		}

		switch v := entry.Value.(type) {
		case *store.StringValue:
			if err := writeBytes(f, v.Data); err != nil {
				return err
			}
		case *store.HashValue:
			if err := binary.Write(f, binary.LittleEndian, uint32(len(v.Fields))); err != nil {
				return err
			}
			for field, val := range v.Fields {
				if err := writeBytes(f, []byte(field)); err != nil {
					return err
				}
				if err := writeBytes(f, val); err != nil {
					return err
				}
			}
		case *store.ListValue:
			if err := binary.Write(f, binary.LittleEndian, uint32(len(v.Elements))); err != nil {
				return err
			}
			for _, elem := range v.Elements {
				if err := writeBytes(f, elem); err != nil {
					return err
				}
			}
		case *store.SetValue:
			if err := binary.Write(f, binary.LittleEndian, uint32(len(v.Members))); err != nil {
				return err
			}
			for member := range v.Members {
				if err := writeBytes(f, []byte(member)); err != nil {
					return err
				}
			}
		}

		if err := binary.Write(f, binary.LittleEndian, uint32(len(entry.Tags))); err != nil {
			return err
		}
		for _, tag := range entry.Tags {
			if err := writeBytes(f, []byte(tag)); err != nil {
				return err
			}
		}

		if err := binary.Write(f, binary.LittleEndian, entry.ExpiresAt); err != nil {
			return err
		}
	}

	f.Sync()
	return os.Rename(tmpPath, path)
}

func writeBytes(f *os.File, b []byte) error {
	if err := binary.Write(f, binary.LittleEndian, uint32(len(b))); err != nil {
		return err
	}
	_, err := f.Write(b)
	return err
}

type SnapshotReader struct {
	dataDir string
}

func NewSnapshotReader(dataDir string) *SnapshotReader {
	return &SnapshotReader{dataDir: dataDir}
}

func (r *SnapshotReader) Load(st *store.Store) error {
	path := filepath.Join(r.dataDir, "dump.snapshot")
	f, err := os.Open(path)
	if os.IsNotExist(err) {
		return nil
	}
	if err != nil {
		return err
	}
	defer f.Close()

	var magic [4]byte
	if err := binary.Read(f, binary.LittleEndian, &magic); err != nil {
		return err
	}
	if string(magic[:]) != snapshotMagic {
		return fmt.Errorf("invalid snapshot magic")
	}

	var version uint32
	if err := binary.Read(f, binary.LittleEndian, &version); err != nil {
		return err
	}

	var timestamp uint64
	if err := binary.Read(f, binary.LittleEndian, &timestamp); err != nil {
		return err
	}

	var keyCount uint64
	if err := binary.Read(f, binary.LittleEndian, &keyCount); err != nil {
		return err
	}

	for i := uint64(0); i < keyCount; i++ {
		key, err := readBytes(f)
		if err != nil {
			return err
		}

		var typeByte uint8
		if err := binary.Read(f, binary.LittleEndian, &typeByte); err != nil {
			return err
		}

		var value store.Value
		switch store.DataType(typeByte) {
		case store.DataTypeString:
			data, err := readBytes(f)
			if err != nil {
				return err
			}
			value = &store.StringValue{Data: data}

		case store.DataTypeHash:
			var fieldCount uint32
			if err := binary.Read(f, binary.LittleEndian, &fieldCount); err != nil {
				return err
			}
			fields := make(map[string][]byte)
			for j := uint32(0); j < fieldCount; j++ {
				field, err := readBytes(f)
				if err != nil {
					return err
				}
				val, err := readBytes(f)
				if err != nil {
					return err
				}
				fields[string(field)] = val
			}
			value = &store.HashValue{Fields: fields}

		case store.DataTypeList:
			var elemCount uint32
			if err := binary.Read(f, binary.LittleEndian, &elemCount); err != nil {
				return err
			}
			elements := make([][]byte, elemCount)
			for j := uint32(0); j < elemCount; j++ {
				elem, err := readBytes(f)
				if err != nil {
					return err
				}
				elements[j] = elem
			}
			value = &store.ListValue{Elements: elements}

		case store.DataTypeSet:
			var memberCount uint32
			if err := binary.Read(f, binary.LittleEndian, &memberCount); err != nil {
				return err
			}
			members := make(map[string]struct{})
			for j := uint32(0); j < memberCount; j++ {
				member, err := readBytes(f)
				if err != nil {
					return err
				}
				members[string(member)] = struct{}{}
			}
			value = &store.SetValue{Members: members}
		}

		var tagCount uint32
		if err := binary.Read(f, binary.LittleEndian, &tagCount); err != nil {
			return err
		}
		tags := make([]string, tagCount)
		for j := uint32(0); j < tagCount; j++ {
			tag, err := readBytes(f)
			if err != nil {
				return err
			}
			tags[j] = string(tag)
		}

		var expiresAt int64
		if err := binary.Read(f, binary.LittleEndian, &expiresAt); err != nil {
			return err
		}

		entry := store.NewEntry(value)
		entry.Tags = tags
		entry.ExpiresAt = expiresAt

		st.SetEntry(string(key), entry)
	}

	logger.Info().Uint64("keys", keyCount).Msg("snapshot loaded")
	return nil
}

func readBytes(f io.Reader) ([]byte, error) {
	var length uint32
	if err := binary.Read(f, binary.LittleEndian, &length); err != nil {
		return nil, err
	}
	buf := make([]byte, length)
	if _, err := io.ReadFull(f, buf); err != nil {
		return nil, err
	}
	return buf, nil
}

type PersistencePlugin struct {
	aof      *AOFWriter
	snapshot *SnapshotWriter
	reader   *SnapshotReader
	store    *store.Store
	interval time.Duration
	stopCh   chan struct{}
	wg       sync.WaitGroup
}

func NewPersistencePlugin(store *store.Store, dataDir, syncMode string, interval time.Duration) *PersistencePlugin {
	return &PersistencePlugin{
		aof:      NewAOFWriter(dataDir, syncMode, true),
		snapshot: NewSnapshotWriter(dataDir, true),
		reader:   NewSnapshotReader(dataDir),
		store:    store,
		interval: interval,
		stopCh:   make(chan struct{}),
	}
}

func (p *PersistencePlugin) Name() string    { return "persistence" }
func (p *PersistencePlugin) Version() string { return "1.0.0" }

func (p *PersistencePlugin) Init(config interface{}) error {
	if err := p.aof.Open(); err != nil {
		return err
	}
	if err := p.reader.Load(p.store); err != nil {
		logger.Warn().Err(err).Msg("failed to load snapshot")
	}
	return nil
}

func (p *PersistencePlugin) Close() error {
	close(p.stopCh)
	p.wg.Wait()
	p.aof.Flush()
	return p.aof.Close()
}

func (p *PersistencePlugin) AfterCommand(ctx *command.Context) {
	mutatingCommands := map[string]bool{
		"SET": true, "DEL": true, "MSET": true, "APPEND": true,
		"INCR": true, "DECR": true, "INCRBY": true, "DECRBY": true,
		"HSET": true, "HDEL": true, "HMSET": true,
		"LPUSH": true, "RPUSH": true, "LPOP": true, "RPOP": true, "LSET": true, "LREM": true,
		"SADD": true, "SREM": true,
		"SETTAG": true, "ADDTAG": true, "REMTAG": true, "INVALIDATE": true,
		"EXPIRE": true, "PEXPIRE": true, "PERSIST": true,
		"RENAME": true,
	}

	if mutatingCommands[ctx.Command] {
		p.aof.Append(ctx.Command, ctx.Args)
	}
}

func (p *PersistencePlugin) OnStartup() error {
	p.wg.Add(1)
	go p.snapshotLoop()
	return nil
}

func (p *PersistencePlugin) OnShutdown() error {
	return p.snapshot.Save(p.store)
}

func (p *PersistencePlugin) snapshotLoop() {
	defer p.wg.Done()

	ticker := time.NewTicker(p.interval)
	defer ticker.Stop()

	for {
		select {
		case <-p.stopCh:
			return
		case <-ticker.C:
			if err := p.snapshot.Save(p.store); err != nil {
				logger.Error().Err(err).Msg("snapshot failed")
			}
		}
	}
}

var _ plugin.AfterCommandHook = (*PersistencePlugin)(nil)
var _ plugin.OnStartupHook = (*PersistencePlugin)(nil)
var _ plugin.OnShutdownHook = (*PersistencePlugin)(nil)
