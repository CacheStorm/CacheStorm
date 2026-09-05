package persistence

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/cachestorm/cachestorm/internal/logger"
	"github.com/cachestorm/cachestorm/internal/resp"
	"github.com/cachestorm/cachestorm/internal/store"
)

type AOFSyncPolicy int

const (
	AOFNoSync AOFSyncPolicy = iota
	AOFEverySecond
	AOFAlways
)

type AOFConfig struct {
	Enabled     bool
	Filename    string
	DataDir     string
	SyncPolicy  AOFSyncPolicy
	RewriteSize int64
	RewritePct  int
	AutoRewrite bool
}

type AOFWriter struct {
	config    AOFConfig
	mu        sync.Mutex
	file      *os.File
	writer    *bufio.Writer
	writerBuf []byte
	size      atomic.Int64
	dirty     atomic.Int64
	lastSync  time.Time
	stopCh    chan struct{}
	wg        sync.WaitGroup
	running   atomic.Bool
}

func NewAOFWriter(cfg AOFConfig) *AOFWriter {
	return &AOFWriter{
		config:    cfg,
		writerBuf: make([]byte, 0, 4096),
		stopCh:    make(chan struct{}),
	}
}

func (w *AOFWriter) Start() error {
	if !w.config.Enabled {
		return nil
	}

	path := filepath.Join(w.config.DataDir, w.config.Filename)

	var err error
	w.file, err = os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
	if err != nil {
		return fmt.Errorf("failed to open AOF file: %v", err)
	}

	stat, err := w.file.Stat()
	if err != nil {
		w.file.Close()
		return fmt.Errorf("failed to stat AOF file: %v", err)
	}
	w.size.Store(stat.Size())

	w.writer = bufio.NewWriterSize(w.file, 8192)
	w.running.Store(true)

	if w.config.SyncPolicy == AOFEverySecond {
		w.wg.Add(1)
		go w.syncLoop()
	}

	logger.Info().Str("path", path).Msg("AOF writer started")
	return nil
}

func (w *AOFWriter) Stop() {
	if !w.running.CompareAndSwap(true, false) {
		return
	}

	close(w.stopCh)
	w.wg.Wait()

	w.mu.Lock()
	if w.writer != nil {
		w.writer.Flush()
	}
	if w.file != nil {
		w.file.Close()
	}
	w.mu.Unlock()

	logger.Info().Msg("AOF writer stopped")
}

func (w *AOFWriter) syncLoop() {
	defer w.wg.Done()

	ticker := time.NewTicker(time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-w.stopCh:
			return
		case <-ticker.C:
			w.syncFile()
		}
	}
}

func (w *AOFWriter) syncFile() {
	w.mu.Lock()
	defer w.mu.Unlock()

	if w.writer != nil {
		_ = w.writer.Flush()
	}
	if w.file != nil {
		if err := w.file.Sync(); err != nil {
			logger.Error().Err(err).Msg("aof fsync failed")
		} else {
			w.lastSync = time.Now()
		}
	}
}

func (w *AOFWriter) Append(cmd string, args [][]byte) error {
	if !w.config.Enabled || !w.running.Load() {
		return nil
	}

	w.mu.Lock()
	defer w.mu.Unlock()

	w.writerBuf = w.writerBuf[:0]
	w.writerBuf = append(w.writerBuf, '*')
	w.writerBuf = strconv.AppendInt(w.writerBuf, int64(len(args)+1), 10)
	w.writerBuf = append(w.writerBuf, '\r', '\n', '$')
	w.writerBuf = strconv.AppendInt(w.writerBuf, int64(len(cmd)), 10)
	w.writerBuf = append(w.writerBuf, '\r', '\n')
	w.writerBuf = append(w.writerBuf, cmd...)
	w.writerBuf = append(w.writerBuf, '\r', '\n')

	for _, arg := range args {
		w.writerBuf = append(w.writerBuf, '$')
		w.writerBuf = strconv.AppendInt(w.writerBuf, int64(len(arg)), 10)
		w.writerBuf = append(w.writerBuf, '\r', '\n')
		w.writerBuf = append(w.writerBuf, arg...)
		w.writerBuf = append(w.writerBuf, '\r', '\n')
	}

	n, err := w.writer.Write(w.writerBuf)
	if err != nil {
		return fmt.Errorf("failed to write to AOF: %v", err)
	}

	w.size.Add(int64(n))
	w.dirty.Add(1)

	if w.config.SyncPolicy == AOFAlways {
		if err := w.writer.Flush(); err != nil {
			return err
		}
		if err := w.file.Sync(); err != nil {
			return err
		}
	}

	return nil
}

func (w *AOFWriter) Size() int64 {
	return w.size.Load()
}

func (w *AOFWriter) Dirty() int64 {
	return w.dirty.Load()
}

func (w *AOFWriter) Flush() error {
	w.mu.Lock()
	defer w.mu.Unlock()

	if w.writer != nil {
		if err := w.writer.Flush(); err != nil {
			return err
		}
	}
	if w.file != nil {
		return w.file.Sync()
	}
	return nil
}

type AOFReader struct {
}

func NewAOFReader() *AOFReader {
	return &AOFReader{}
}

type Command struct {
	Name string
	Args [][]byte
}

func (r *AOFReader) Load(path string) ([]Command, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("failed to open AOF file: %v", err)
	}
	defer f.Close()

	var commands []Command
	reader := resp.NewReader(bufio.NewReader(f))

	for {
		cmd, args, err := reader.ReadCommand()
		if err != nil {
			if err == io.EOF {
				break
			}
			if strings.Contains(err.Error(), "EOF") {
				break
			}
			return nil, fmt.Errorf("failed to read command: %v", err)
		}

		commands = append(commands, Command{
			Name: cmd,
			Args: args,
		})
	}

	logger.Info().Int("commands", len(commands)).Msg("AOF loaded")
	return commands, nil
}

type AOFRewriter struct {
	config    AOFConfig
	store     interface{ GetAll() map[string]interface{} }
	rewriting atomic.Bool
	lastSize  int64
}

func NewAOFRewriter(cfg AOFConfig, store interface{ GetAll() map[string]interface{} }) *AOFRewriter {
	return &AOFRewriter{
		config: cfg,
		store:  store,
	}
}

func (rw *AOFRewriter) Rewrite(aofPath string) error {
	if !rw.rewriting.CompareAndSwap(false, true) {
		return fmt.Errorf("rewrite already in progress")
	}
	defer rw.rewriting.Store(false)

	tempPath := aofPath + ".tmp"

	f, err := os.Create(tempPath)
	if err != nil {
		return fmt.Errorf("failed to create temp file: %v", err)
	}

	writer := bufio.NewWriterSize(f, 8192)

	entries := rw.store.GetAll()
	for key, entry := range entries {
		if err := rw.writeEntry(writer, key, entry); err != nil {
			f.Close()
			os.Remove(tempPath)
			return err
		}
	}

	if err := writer.Flush(); err != nil {
		f.Close()
		os.Remove(tempPath)
		return err
	}

	if err := f.Sync(); err != nil {
		f.Close()
		os.Remove(tempPath)
		return err
	}

	stat, err := f.Stat()
	if err != nil {
		return err
	}
	rw.lastSize = stat.Size()

	// Close before rename — required on Windows where open files can't be renamed
	if err := f.Close(); err != nil {
		os.Remove(tempPath)
		return err
	}

	if err := os.Rename(tempPath, aofPath); err != nil {
		os.Remove(tempPath)
		return fmt.Errorf("failed to rename AOF file: %v", err)
	}

	logger.Info().Str("path", aofPath).Msg("AOF rewrite completed")
	return nil
}

func (rw *AOFRewriter) writeEntry(w *bufio.Writer, key string, entry interface{}) error {
	switch v := entry.(type) {
	case *store.Entry:
		return rw.writeValueCommands(w, key, v.Value)
	case store.Value:
		return rw.writeValueCommands(w, key, v)
	case string:
		return rw.writeCommand(w, "SET", key, v)
	case []byte:
		return rw.writeCommand(w, "SET", key, string(v))
	default:
		return rw.writeCommand(w, "SET", key, fmt.Sprintf("%v", v))
	}
}

// writeCommand appends one RESP array command to the rewrite buffer.
func (rw *AOFRewriter) writeCommand(w *bufio.Writer, name string, args ...string) error {
	total := 1 + len(args)
	if _, err := fmt.Fprintf(w, "*%d\r\n$%d\r\n%s\r\n", total, len(name), name); err != nil {
		return err
	}
	for _, arg := range args {
		if _, err := fmt.Fprintf(w, "$%d\r\n%s\r\n", len(arg), arg); err != nil {
			return err
		}
	}
	return nil
}

// writeValueCommands emits router-executable commands that reconstruct the
// typed value on AOF replay, instead of flattening every type to SET.
func (rw *AOFRewriter) writeValueCommands(w *bufio.Writer, key string, value store.Value) error {
	switch vt := value.(type) {
	case *store.StringValue:
		return rw.writeCommand(w, "SET", key, string(vt.Data))

	case *store.HashValue:
		for field, val := range vt.Fields {
			if err := rw.writeCommand(w, "HSET", key, field, string(val)); err != nil {
				return err
			}
		}
		return nil

	case *store.ListValue:
		args := make([]string, 0, len(vt.Elements)+1)
		args = append(args, key)
		for _, el := range vt.Elements {
			args = append(args, string(el))
		}
		return rw.writeCommand(w, "RPUSH", args...)

	case *store.SetValue:
		args := make([]string, 0, len(vt.Members)+1)
		args = append(args, key)
		for member := range vt.Members {
			args = append(args, member)
		}
		return rw.writeCommand(w, "SADD", args...)

	case *store.SortedSetValue:
		args := make([]string, 0, len(vt.Members)*2+1)
		args = append(args, key)
		for member, score := range vt.Members {
			args = append(args, strconv.FormatFloat(score, 'f', -1, 64), member)
		}
		return rw.writeCommand(w, "ZADD", args...)

	case *store.GeoValue:
		args := make([]string, 0, len(vt.Points)*3+1)
		args = append(args, key)
		for member, point := range vt.Points {
			args = append(args,
				strconv.FormatFloat(point.Lon, 'f', -1, 64),
				strconv.FormatFloat(point.Lat, 'f', -1, 64),
				member)
		}
		return rw.writeCommand(w, "GEOADD", args...)

	case *store.JSONValue:
		return rw.writeCommand(w, "JSON.SET", key, "$", string(vt.Data))

	case *store.StreamValue:
		for _, entry := range vt.Entries {
			args := make([]string, 0, len(entry.Fields)*2+2)
			args = append(args, key, entry.ID)
			for k, val := range entry.Fields {
				args = append(args, k, string(val))
			}
			if err := rw.writeCommand(w, "XADD", args...); err != nil {
				return err
			}
		}
		return nil

	case *store.TimeSeriesValue:
		retentionMS := int64(vt.Retention / time.Millisecond)
		createArgs := []string{key}
		if retentionMS > 0 {
			createArgs = append(createArgs, "RETENTION", strconv.FormatInt(retentionMS, 10))
		}
		if len(vt.Labels) > 0 {
			createArgs = append(createArgs, "LABELS")
			for k, val := range vt.Labels {
				createArgs = append(createArgs, k, val)
			}
		}
		if err := rw.writeCommand(w, "TS.CREATE", createArgs...); err != nil {
			return err
		}
		for _, sample := range vt.Samples {
			if err := rw.writeCommand(w, "TS.ADD", key,
				strconv.FormatInt(sample.Timestamp, 10),
				strconv.FormatFloat(sample.Value, 'f', -1, 64)); err != nil {
				return err
			}
		}
		return nil

	default:
		// Unknown value shape: preserve the legacy lossy SET fallback.
		return rw.writeCommand(w, "SET", key, fmt.Sprintf("%v", value))
	}
}

func (rw *AOFRewriter) IsRewriting() bool {
	return rw.rewriting.Load()
}

func (rw *AOFRewriter) ShouldRewrite(currentSize int64) bool {
	if rw.rewriting.Load() {
		return false
	}

	if currentSize < rw.config.RewriteSize {
		return false
	}

	if rw.lastSize == 0 {
		return currentSize >= rw.config.RewriteSize
	}

	pct := int((currentSize - rw.lastSize) * 100 / rw.lastSize)
	return pct >= rw.config.RewritePct
}

type AOFManager struct {
	writer   *AOFWriter
	reader   *AOFReader
	rewriter *AOFRewriter
	config   AOFConfig
	mu       sync.Mutex
}

func NewAOFManager(cfg AOFConfig, store interface{ GetAll() map[string]interface{} }) *AOFManager {
	return &AOFManager{
		writer:   NewAOFWriter(cfg),
		reader:   NewAOFReader(),
		rewriter: NewAOFRewriter(cfg, store),
		config:   cfg,
	}
}

func (m *AOFManager) Start() error {
	return m.writer.Start()
}

func (m *AOFManager) Stop() {
	m.writer.Stop()
}

func (m *AOFManager) Append(cmd string, args [][]byte) error {
	return m.writer.Append(cmd, args)
}

func (m *AOFManager) Load() ([]Command, error) {
	path := filepath.Join(m.config.DataDir, m.config.Filename)
	return m.reader.Load(path)
}

func (m *AOFManager) BGREWRITEAOF() error {
	m.mu.Lock()
	defer m.mu.Unlock()

	path := filepath.Join(m.config.DataDir, m.config.Filename)
	return m.rewriter.Rewrite(path)
}

func (m *AOFManager) Size() int64 {
	return m.writer.Size()
}

func (m *AOFManager) Dirty() int64 {
	return m.writer.Dirty()
}

func (m *AOFManager) Flush() error {
	return m.writer.Flush()
}

func (m *AOFManager) IsRewriting() bool {
	return m.rewriter.IsRewriting()
}

func (m *AOFManager) Info() map[string]interface{} {
	return map[string]interface{}{
		"aof_enabled":                  m.config.Enabled,
		"aof_rewrite_in_progress":      m.rewriter.IsRewriting(),
		"aof_last_rewrite_time_sec":    0,
		"aof_current_rewrite_time_sec": 0,
		"aof_current_size":             m.writer.Size(),
		"aof_base_size":                m.rewriter.lastSize,
		"aof_pending_rewrite":          0,
		"aof_buffer_length":            0,
		"aof_rewrite_buffer_length":    0,
		"aof_pending_bio_fsync":        0,
		"aof_delayed_fsync":            0,
	}
}

func SyncPolicyFromString(s string) AOFSyncPolicy {
	switch strings.ToLower(s) {
	case "always":
		return AOFAlways
	case "everysec":
		return AOFEverySecond
	case "no", "none":
		return AOFNoSync
	default:
		return AOFEverySecond
	}
}
