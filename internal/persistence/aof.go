package persistence

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/cachestorm/cachestorm/internal/logger"
	"github.com/cachestorm/cachestorm/internal/resp"
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
	size      int64
	dirty     int64
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
	w.size = stat.Size()

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
		w.writer.Flush()
	}
	if w.file != nil {
		w.file.Sync()
		w.lastSync = time.Now()
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
	w.writerBuf = append(w.writerBuf, fmt.Sprintf("%d", len(args)+1)...)
	w.writerBuf = append(w.writerBuf, '\r', '\n')

	w.writerBuf = append(w.writerBuf, '$')
	w.writerBuf = append(w.writerBuf, fmt.Sprintf("%d", len(cmd))...)
	w.writerBuf = append(w.writerBuf, '\r', '\n')
	w.writerBuf = append(w.writerBuf, cmd...)
	w.writerBuf = append(w.writerBuf, '\r', '\n')

	for _, arg := range args {
		w.writerBuf = append(w.writerBuf, '$')
		w.writerBuf = append(w.writerBuf, fmt.Sprintf("%d", len(arg))...)
		w.writerBuf = append(w.writerBuf, '\r', '\n')
		w.writerBuf = append(w.writerBuf, arg...)
		w.writerBuf = append(w.writerBuf, '\r', '\n')
	}

	n, err := w.writer.Write(w.writerBuf)
	if err != nil {
		return fmt.Errorf("failed to write to AOF: %v", err)
	}

	w.size += int64(n)
	w.dirty++

	if w.config.SyncPolicy == AOFAlways {
		w.writer.Flush()
		w.file.Sync()
	}

	return nil
}

func (w *AOFWriter) Size() int64 {
	w.mu.Lock()
	defer w.mu.Unlock()
	return w.size
}

func (w *AOFWriter) Dirty() int64 {
	w.mu.Lock()
	defer w.mu.Unlock()
	return w.dirty
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
	mu        sync.Mutex
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
	defer f.Close()

	writer := bufio.NewWriterSize(f, 8192)

	entries := rw.store.GetAll()
	for key, entry := range entries {
		if err := rw.writeEntry(writer, key, entry); err != nil {
			os.Remove(tempPath)
			return err
		}
	}

	if err := writer.Flush(); err != nil {
		os.Remove(tempPath)
		return err
	}

	if err := f.Sync(); err != nil {
		os.Remove(tempPath)
		return err
	}

	if err := os.Rename(tempPath, aofPath); err != nil {
		os.Remove(tempPath)
		return fmt.Errorf("failed to rename AOF file: %v", err)
	}

	stat, _ := f.Stat()
	rw.lastSize = stat.Size()

	logger.Info().Str("path", aofPath).Msg("AOF rewrite completed")
	return nil
}

func (rw *AOFRewriter) writeEntry(w *bufio.Writer, key string, entry interface{}) error {
	var buf []byte
	buf = append(buf, '*')
	buf = append(buf, '3')
	buf = append(buf, '\r', '\n')

	buf = append(buf, '$', '3', '\r', '\n', 'S', 'E', 'T', '\r', '\n')

	keyBytes := []byte(key)
	buf = append(buf, '$')
	buf = append(buf, fmt.Sprintf("%d", len(keyBytes))...)
	buf = append(buf, '\r', '\n')
	buf = append(buf, keyBytes...)
	buf = append(buf, '\r', '\n')

	var valueBytes []byte
	switch v := entry.(type) {
	case string:
		valueBytes = []byte(v)
	case []byte:
		valueBytes = v
	default:
		valueBytes = []byte(fmt.Sprintf("%v", v))
	}

	buf = append(buf, '$')
	buf = append(buf, fmt.Sprintf("%d", len(valueBytes))...)
	buf = append(buf, '\r', '\n')
	buf = append(buf, valueBytes...)
	buf = append(buf, '\r', '\n')

	_, err := w.Write(buf)
	return err
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
