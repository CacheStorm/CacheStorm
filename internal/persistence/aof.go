package persistence

import (
	"bufio"
	"bytes"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
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
	config     AOFConfig
	lifecycle  sync.Mutex
	replaceOps aofReplaceOps
	fatalErr   error
	mu         sync.Mutex
	file       *os.File
	writer     *bufio.Writer
	writerBuf  []byte
	size       atomic.Int64
	dirty      atomic.Int64
	lastSync   time.Time
	stopCh     chan struct{}
	wg         sync.WaitGroup
	running    atomic.Bool
	// curDB is the database the commands at the end of the file apply to;
	// a command for another database is preceded by SELECT. -1 means
	// unknown, so the next command always gets a SELECT.
	curDB    int
	openFile func(string, int, os.FileMode) (*os.File, error)
}

func NewAOFWriter(cfg AOFConfig) *AOFWriter {
	return &AOFWriter{
		config:     cfg,
		writerBuf:  make([]byte, 0, 4096),
		stopCh:     make(chan struct{}),
		replaceOps: defaultAOFReplaceOps(),
	}
}

func (w *AOFWriter) Start() error {
	w.lifecycle.Lock()
	defer w.lifecycle.Unlock()
	w.mu.Lock()
	defer w.mu.Unlock()
	if w.fatalErr != nil {
		return w.fatalErr
	}
	if !w.config.Enabled {
		return nil
	}
	if w.running.Load() {
		return nil
	}
	w.stopCh = make(chan struct{})

	path := filepath.Join(w.config.DataDir, w.config.Filename)

	var err error
	open := w.openFile
	if open == nil {
		open = openAOFFile
	}
	w.file, err = open(path, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
	if err != nil {
		return fmt.Errorf("failed to open AOF file: %v", err)
	}

	stat, err := w.file.Stat()
	if err != nil {
		w.file.Close()
		return fmt.Errorf("failed to stat AOF file: %v", err)
	}
	w.size.Store(stat.Size())
	if stat.Size() == 0 && stat.Mode().IsRegular() {
		if err := writeAOFHeader(w.file); err != nil {
			w.file.Close()
			return err
		}
		if err := w.file.Sync(); err != nil {
			w.file.Close()
			return err
		}
		info, err := w.file.Stat()
		if err != nil {
			w.file.Close()
			return err
		}
		w.size.Store(info.Size())
	}

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
	if err := w.StopWithError(); err != nil {
		logger.Error().Err(err).Msg("AOF shutdown failed")
	}
}

func (w *AOFWriter) StopWithError() error {
	w.lifecycle.Lock()
	defer w.lifecycle.Unlock()
	if !w.running.CompareAndSwap(true, false) {
		w.mu.Lock()
		defer w.mu.Unlock()
		return w.fatalErr
	}
	close(w.stopCh)
	w.wg.Wait()
	w.mu.Lock()
	defer w.mu.Unlock()
	err := w.fatalErr
	if err == nil {
		if w.writer != nil {
			err = w.writer.Flush()
		}
		if w.file != nil {
			err = errors.Join(err, w.file.Sync())
		}
	}
	if w.file != nil {
		err = errors.Join(err, w.file.Close())
	}
	if err != nil {
		w.fatalErr = err
	}
	logger.Info().Msg("AOF writer stopped")
	return err
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
	if w.fatalErr != nil {
		return
	}

	if w.writer != nil {
		if err := w.writer.Flush(); err != nil {
			w.fatalErr = err
			logger.Error().Err(err).Msg("aof flush failed")
			return
		}
	}
	if w.file != nil {
		if err := w.file.Sync(); err != nil {
			w.fatalErr = err
			logger.Error().Err(err).Msg("aof fsync failed")
		} else {
			w.lastSync = time.Now()
		}
	}
}

// Append records a command in the database the file is currently in.
func (w *AOFWriter) Append(cmd string, args [][]byte) error {
	if !w.config.Enabled || !w.running.Load() {
		return nil
	}

	w.mu.Lock()
	defer w.mu.Unlock()

	w.writerBuf = appendCommand(w.writerBuf[:0], cmd, args)
	return w.writeLocked(1)
}

// SetCurrentDB tells the writer which database the end of the file applies
// to, as left by replaying it. -1 makes the next append start with a SELECT.
func (w *AOFWriter) SetCurrentDB(db int) {
	w.mu.Lock()
	w.curDB = db
	w.mu.Unlock()
}

// CurrentDB returns the database the end of the file applies to.
func (w *AOFWriter) CurrentDB() int {
	w.mu.Lock()
	defer w.mu.Unlock()
	return w.curDB
}

// appendSelect adds a SELECT to the buffer when db is not the database the
// buffer ends in, and returns the database it ends in afterwards.
func (w *AOFWriter) appendSelect(cur, db int) int {
	if db == cur {
		return cur
	}
	w.writerBuf = appendCommand(w.writerBuf, "SELECT", [][]byte{strconv.AppendInt(nil, int64(db), 10)})
	return db
}

// AppendDB records a command that ran in database db, preceded by a SELECT
// when the file is in another database, so replay applies it to the same
// database.
func (w *AOFWriter) AppendDB(db int, cmd string, args [][]byte) error {
	if !w.config.Enabled || !w.running.Load() {
		return nil
	}

	w.mu.Lock()
	defer w.mu.Unlock()

	w.writerBuf = w.writerBuf[:0]
	cur := w.appendSelect(w.curDB, db)
	w.writerBuf = appendCommand(w.writerBuf, cmd, args)
	return w.commitLocked(1, cur)
}

// commitLocked writes the buffer and records the database it ends in. A
// failed write leaves the file's database unknown.
func (w *AOFWriter) commitLocked(commands int64, db int) error {
	if err := w.writeLocked(commands); err != nil {
		w.curDB = -1
		return err
	}
	w.curDB = db
	return nil
}

// AppendTransaction records the commands an EXEC ran as one MULTI ... EXEC
// block, written and synced together. Replay applies the block only when its
// EXEC marker is present, so a crash in the middle of the write cannot leave
// half a transaction behind.
func (w *AOFWriter) AppendTransaction(cmds []Command) error {
	if !w.config.Enabled || !w.running.Load() || len(cmds) == 0 {
		return nil
	}

	w.mu.Lock()
	defer w.mu.Unlock()

	w.writerBuf = appendCommand(w.writerBuf[:0], "MULTI", nil)
	cur := w.curDB
	for _, c := range cmds {
		cur = w.appendSelect(cur, c.DB)
		w.writerBuf = appendCommand(w.writerBuf, c.Name, c.Args)
	}
	w.writerBuf = appendCommand(w.writerBuf, "EXEC", nil)
	return w.commitLocked(int64(len(cmds)), cur)
}

func (w *AOFWriter) writeLocked(commands int64) error {
	if w.fatalErr != nil {
		return w.fatalErr
	}
	n, err := w.writer.Write(w.writerBuf)
	if err != nil {
		w.fatalErr = fmt.Errorf("failed to write to AOF: %w", err)
		return w.fatalErr
	}

	w.size.Add(int64(n))
	w.dirty.Add(commands)

	if w.config.SyncPolicy == AOFAlways {
		if err := w.writer.Flush(); err != nil {
			w.fatalErr = err
			return err
		}
		if err := w.file.Sync(); err != nil {
			w.fatalErr = err
			return err
		}
	}

	return nil
}

func appendCommand(buf []byte, cmd string, args [][]byte) []byte {
	buf = append(buf, '*')
	buf = strconv.AppendInt(buf, int64(len(args)+1), 10)
	buf = append(buf, '\r', '\n', '$')
	buf = strconv.AppendInt(buf, int64(len(cmd)), 10)
	buf = append(buf, '\r', '\n')
	buf = append(buf, cmd...)
	buf = append(buf, '\r', '\n')

	for _, arg := range args {
		buf = append(buf, '$')
		buf = strconv.AppendInt(buf, int64(len(arg)), 10)
		buf = append(buf, '\r', '\n')
		buf = append(buf, arg...)
		buf = append(buf, '\r', '\n')
	}
	return buf
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
	if w.fatalErr != nil {
		return w.fatalErr
	}

	if w.writer != nil {
		if err := w.writer.Flush(); err != nil {
			if w.running.Load() {
				w.fatalErr = err
			}
			return err
		}
	}
	if w.file != nil {
		err := w.file.Sync()
		if w.running.Load() {
			w.fatalErr = err
		}
		return err
	}
	return nil
}

func (w *AOFWriter) Err() error {
	w.mu.Lock()
	defer w.mu.Unlock()
	return w.fatalErr
}

type AOFReader struct {
	Version int
}

func NewAOFReader() *AOFReader {
	return &AOFReader{}
}

// Command is one command of the AOF. DB is the database it applies to when
// it is appended; the reader leaves it 0 and replay follows the file's
// SELECTs instead.
type Command struct {
	Name string
	Args [][]byte
	DB   int
}

func (r *AOFReader) Load(path string) ([]Command, error) {
	commands, _, err := r.LoadValid(path)
	return commands, err
}

// countingReader counts the bytes read through it.
type countingReader struct {
	r io.Reader
	n int64
}

func (c *countingReader) Read(p []byte) (int, error) {
	n, err := c.r.Read(p)
	c.n += int64(n)
	return n, err
}

// LoadValid reads every complete command and returns, with them, the length
// of the file prefix those commands occupy. A file whose tail was cut short by
// a crash loads up to the last complete command and reports a valid length
// shorter than the file; the caller truncates the file there so the next
// append does not land after the torn bytes. Bytes that do not parse as a
// command anywhere before the tail are corruption and return an error.
func (r *AOFReader) LoadValid(path string) ([]Command, int64, error) {
	r.Version = 0
	f, err := os.Open(path)
	if err != nil {
		return nil, 0, fmt.Errorf("failed to open AOF file: %v", err)
	}
	defer f.Close()

	var commands []Command
	counter := &countingReader{r: f}
	br := bufio.NewReader(counter)
	reader := resp.NewReader(br)
	var valid, txOffset int64
	txIndex := -1

	for {
		cmd, args, err := reader.ReadCommand()
		if err != nil {
			if err == io.EOF || err == io.ErrUnexpectedEOF || strings.Contains(err.Error(), "EOF") {
				break
			}
			return nil, valid, fmt.Errorf("failed to read command at offset %d: %v", valid, err)
		}

		name := strings.ToUpper(cmd)
		if name == aofVersionCommand {
			if valid != 0 || r.Version != 0 || len(args) != 1 || string(args[0]) != "2" {
				return nil, valid, errors.New("invalid or unsupported AOF version header")
			}
			r.Version = AOFVersion
			valid = counter.n - int64(br.Buffered())
			continue
		}
		switch name {
		case "MULTI":
			if len(args) != 0 || txIndex >= 0 {
				return nil, valid, fmt.Errorf("invalid MULTI at offset %d", valid)
			}
			txIndex, txOffset = len(commands), valid
		case "EXEC", "DISCARD":
			if len(args) != 0 || txIndex < 0 {
				return nil, valid, fmt.Errorf("invalid %s at offset %d", name, valid)
			}
			if name == "DISCARD" {
				commands = commands[:txIndex]
				txIndex = -1
				valid = counter.n - int64(br.Buffered())
				continue
			}
			txIndex = -1
		}

		commands = append(commands, Command{
			Name: cmd,
			Args: args,
		})
		valid = counter.n - int64(br.Buffered())
	}

	if txIndex >= 0 {
		commands, valid = commands[:txIndex], txOffset
	}

	logger.Info().Int("commands", len(commands)).Msg("AOF loaded")
	return commands, valid, nil
}

type AOFRewriter struct {
	config    AOFConfig
	store     interface{ GetAll() map[string]interface{} }
	rewriting atomic.Bool
	lastSize  int64
	// scratch is reused to encode one command at a time.
	scratch []byte
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
	if err := writeAOFHeader(writer); err != nil {
		f.Close()
		os.Remove(tempPath)
		return err
	}

	if err := rw.writeAll(writer); err != nil {
		f.Close()
		os.Remove(tempPath)
		return err
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

// MultiDBSource is a rewrite source that holds several numbered databases.
// DatabaseEntries returns each database's entries, indexed by database.
type MultiDBSource interface {
	DatabaseEntries() []map[string]interface{}
}

// EntryStreamer is a MultiDBSource that can hand its entries over one at a
// time, in database order, instead of materializing every database as a map.
type EntryStreamer interface {
	ForEachEntry(fn func(db int, key string, entry interface{}) error) error
}

// rewriteItemsPerCommand caps how many collection items one rewritten command
// carries, as Redis' AOF_REWRITE_ITEMS_PER_CMD does.
const rewriteItemsPerCommand = 64

// writeAll writes the source's keys. A multi-database source gets a SELECT
// before each non-empty database and a final SELECT 0, so commands appended
// after the rewrite land in database 0 as the writer assumes.
func (rw *AOFRewriter) writeAll(w *bufio.Writer) error {
	if src, ok := rw.store.(interface{ FunctionPayload() []byte }); ok {
		if payload := src.FunctionPayload(); len(payload) > 0 {
			if err := rw.writeCommand(w, "FUNCTION", "RESTORE", string(payload), "FLUSH"); err != nil {
				return err
			}
		}
	}
	if src, ok := rw.store.(interface{ SearchCommands() [][]string }); ok {
		for _, cmd := range src.SearchCommands() {
			if err := rw.writeCommand(w, cmd[0], cmd[1:]...); err != nil {
				return err
			}
		}
	}
	multi, ok := rw.store.(MultiDBSource)
	if !ok {
		for key, entry := range rw.store.GetAll() {
			if err := rw.writeEntry(w, key, entry); err != nil {
				return err
			}
		}
		return nil
	}
	selected := 0
	if adapter, ok := multi.(multiDBAdapter); ok {
		multi = adapter.MultiDBSource
	}
	if streamer, ok := multi.(EntryStreamer); ok {
		err := streamer.ForEachEntry(func(db int, key string, entry interface{}) error {
			if db != selected {
				if err := rw.writeCommand(w, "SELECT", strconv.Itoa(db)); err != nil {
					return err
				}
				selected = db
			}
			return rw.writeEntry(w, key, entry)
		})
		if err != nil {
			return err
		}
		if selected != 0 {
			return rw.writeCommand(w, "SELECT", "0")
		}
		return nil
	}
	for db, entries := range multi.DatabaseEntries() {
		if len(entries) == 0 {
			continue
		}
		if db != selected {
			if err := rw.writeCommand(w, "SELECT", strconv.Itoa(db)); err != nil {
				return err
			}
			selected = db
		}
		for key, entry := range entries {
			if err := rw.writeEntry(w, key, entry); err != nil {
				return err
			}
		}
	}
	if selected != 0 {
		return rw.writeCommand(w, "SELECT", "0")
	}
	return nil
}

func (rw *AOFRewriter) writeEntry(w *bufio.Writer, key string, entry interface{}) error {
	switch v := entry.(type) {
	case *store.Entry:
		if !hashLive(v.Value) {
			return nil
		}
		if err := rw.writeValueCommands(w, key, v.Value); err != nil {
			return err
		}
		if len(v.Tags) > 0 {
			args := append([]string{key}, v.Tags...)
			if err := rw.writeCommand(w, "ADDTAG", args...); err != nil {
				return err
			}
		}
		return rw.writeExpiry(w, key, v.ExpiresAt.Load())
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

// writeStreamGroups re-emits a stream's consumer groups as XGROUP CREATE so a
// rewrite preserves them, at their read position and logical read counter.
// Consumers come back with XGROUP CREATECONSUMER and pending entries with
// XCLAIM ... FORCE, the way Redis rewrites them.
func (rw *AOFRewriter) writeStreamGroups(w *bufio.Writer, key string, sv *store.StreamValue) error {
	names := make([]string, 0, len(sv.Groups))
	for name := range sv.Groups {
		names = append(names, name)
	}
	sort.Strings(names)
	for _, name := range names {
		group := sv.Groups[name]
		if group == nil {
			continue
		}
		lastID := group.LastID
		if lastID == "" {
			lastID = "0-0"
		}
		args := []string{"CREATE", key, name, lastID, "MKSTREAM"}
		if group.EntriesRead != store.EntriesReadInvalid {
			args = append(args, "ENTRIESREAD", strconv.FormatInt(group.EntriesRead, 10))
		}
		if err := rw.writeCommand(w, "XGROUP", args...); err != nil {
			return err
		}
		if err := rw.writeStreamConsumers(w, key, name, group); err != nil {
			return err
		}
	}
	return nil
}

// writeStreamConsumers re-emits a group's consumers and pending entries. The
// caller holds the stream's read lock, which excludes every group writer.
func (rw *AOFRewriter) writeStreamConsumers(w *bufio.Writer, key, name string, group *store.ConsumerGroup) error {
	consumers := make([]string, 0, len(group.Consumers))
	for consumer := range group.Consumers {
		consumers = append(consumers, consumer)
	}
	sort.Strings(consumers)
	for _, consumer := range consumers {
		if err := rw.writeCommand(w, "XGROUP", "CREATECONSUMER", key, name, consumer); err != nil {
			return err
		}
	}
	type pendingRow struct {
		id store.StreamID
		p  *store.PendingEntry
	}
	rows := make([]pendingRow, 0, len(group.Pending))
	for _, p := range group.Pending {
		id, err := store.ParseStreamID(p.ID, 0)
		if err != nil {
			continue
		}
		rows = append(rows, pendingRow{id, p})
	}
	sort.Slice(rows, func(i, j int) bool { return rows[i].id.Less(rows[j].id) })
	for _, r := range rows {
		if err := rw.writeCommand(w, "XCLAIM", key, name, r.p.Consumer, "0", r.p.ID,
			"TIME", strconv.FormatInt(r.p.DeliveryTS, 10),
			"RETRYCOUNT", strconv.FormatInt(r.p.Deliveries, 10),
			"JUSTID", "FORCE"); err != nil {
			return err
		}
	}
	return nil
}

// writeExpiry re-emits a key's absolute expiry after its value commands.
func (rw *AOFRewriter) writeExpiry(w *bufio.Writer, key string, expiresAt int64) error {
	if expiresAt <= 0 {
		return nil
	}
	return rw.writeCommand(w, "PEXPIREAT", key, strconv.FormatInt(expiresAt, 10))
}

// writeCommand appends one RESP array command to the rewrite buffer.
func (rw *AOFRewriter) writeCommand(w *bufio.Writer, name string, args ...string) error {
	buf := appendArrayHeader(rw.scratch[:0], 1+len(args))
	buf = appendBulkString(buf, name)
	for _, arg := range args {
		buf = appendBulkString(buf, arg)
	}
	rw.scratch = buf
	_, err := w.Write(buf)
	return err
}

// writeKeyValue writes "name key value" without converting value to a
// string.
func (rw *AOFRewriter) writeKeyValue(w *bufio.Writer, name, key string, value []byte) error {
	buf := appendArrayHeader(rw.scratch[:0], 3)
	buf = appendBulkString(buf, name)
	buf = appendBulkString(buf, key)
	buf = appendBulkBytes(buf, value)
	rw.scratch = buf
	_, err := w.Write(buf)
	return err
}

func appendArrayHeader(buf []byte, n int) []byte {
	buf = append(buf, '*')
	buf = strconv.AppendInt(buf, int64(n), 10)
	return append(buf, '\r', '\n')
}

func appendBulkString(buf []byte, s string) []byte {
	buf = append(buf, '$')
	buf = strconv.AppendInt(buf, int64(len(s)), 10)
	buf = append(buf, '\r', '\n')
	buf = append(buf, s...)
	return append(buf, '\r', '\n')
}

func appendBulkBytes(buf []byte, b []byte) []byte {
	buf = append(buf, '$')
	buf = strconv.AppendInt(buf, int64(len(b)), 10)
	buf = append(buf, '\r', '\n')
	buf = append(buf, b...)
	return append(buf, '\r', '\n')
}

// writeBatched writes name key item... in commands of at most
// rewriteItemsPerCommand items, each item being width arguments.
func (rw *AOFRewriter) writeBatched(w *bufio.Writer, name, key string, width int, items []string) error {
	per := rewriteItemsPerCommand * width
	for start := 0; start < len(items); start += per {
		end := start + per
		if end > len(items) {
			end = len(items)
		}
		args := make([]string, 0, end-start+1)
		args = append(args, key)
		args = append(args, items[start:end]...)
		if err := rw.writeCommand(w, name, args...); err != nil {
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
		return rw.writeKeyValue(w, "SET", key, vt.Data)

	case *store.HashValue:
		snap := snapshotHash(vt)
		items := make([]string, 0, len(snap.fields)*2)
		for i, field := range snap.fields {
			items = append(items, field, string(snap.values[i]))
		}
		if err := rw.writeBatched(w, "HSET", key, 2, items); err != nil {
			return err
		}
		return rw.writeHashTTLs(w, key, snap)

	case *store.ListValue:
		vt.RLock()
		args := make([]string, 0, len(vt.Elements)+1)
		args = append(args, key)
		for _, el := range vt.Elements {
			args = append(args, string(el))
		}
		vt.RUnlock()
		return rw.writeCommand(w, "RPUSH", args...)

	case *store.SetValue:
		vt.RLock()
		members := make([]string, 0, len(vt.Members))
		for member := range vt.Members {
			members = append(members, member)
		}
		vt.RUnlock()
		return rw.writeBatched(w, "SADD", key, 1, members)

	case *store.SortedSetValue:
		vt.RLock()
		items := make([]string, 0, len(vt.Members)*2)
		for member, score := range vt.Members {
			items = append(items, strconv.FormatFloat(score, 'f', -1, 64), member)
		}
		vt.RUnlock()
		return rw.writeBatched(w, "ZADD", key, 2, items)

	case *store.GeoValue:
		vt.RLock()
		args := make([]string, 0, len(vt.Points)*3+1)
		args = append(args, key)
		for member, point := range vt.Points {
			args = append(args,
				strconv.FormatFloat(point.Lon, 'f', -1, 64),
				strconv.FormatFloat(point.Lat, 'f', -1, 64),
				member)
		}
		vt.RUnlock()
		return rw.writeCommand(w, "GEOADD", args...)

	case *store.JSONValue:
		vt.RLock()
		data := make([]byte, len(vt.Data))
		copy(data, vt.Data)
		vt.RUnlock()
		return rw.writeCommand(w, "JSON.SET", key, "$", string(data))

	case *store.StreamValue:
		vt.RLock()
		defer vt.RUnlock()
		if len(vt.Entries) == 0 {
			if err := rw.writeCommand(w, "XGROUP", "CREATE", key, "", "0-0", "MKSTREAM"); err != nil {
				return err
			}
			if err := rw.writeCommand(w, "XGROUP", "DESTROY", key, ""); err != nil {
				return err
			}
		}
		for _, entry := range vt.Entries {
			pairs := entry.FieldPairs()
			args := make([]string, 0, len(pairs)*2+2)
			args = append(args, key, entry.ID)
			for _, p := range pairs {
				args = append(args, p.Name, string(p.Value))
			}
			if err := rw.writeCommand(w, "XADD", args...); err != nil {
				return err
			}
		}
		if vt.LastID != "" {
			args := []string{key, vt.LastID}
			if vt.EntriesAdded > int64(len(vt.Entries)) {
				args = append(args, "ENTRIESADDED", strconv.FormatInt(vt.EntriesAdded, 10))
			}
			if vt.MaxDeletedID != "" {
				args = append(args, "MAXDELETEDID", vt.MaxDeletedID)
			}
			if err := rw.writeCommand(w, "XSETID", args...); err != nil {
				return err
			}
		}
		// Consumer groups are stream state, not entry data: without these the
		// entries replay fine but every group is lost, so a client resuming a
		// group gets NOGROUP and the group's read position silently resets.
		// Groups are emitted after the entries so XGROUP CREATE finds the
		// stream already present.
		if err := rw.writeStreamGroups(w, key, vt); err != nil {
			return err
		}
		return nil

	case *store.TimeSeriesValue:
		vt.RLock()
		retentionMS := int64(vt.Retention / time.Millisecond)
		labels := make(map[string]string, len(vt.Labels))
		for k, val := range vt.Labels {
			labels[k] = val
		}
		samples := make([]store.TimeSeriesSample, len(vt.Samples))
		copy(samples, vt.Samples)
		vt.RUnlock()
		createArgs := []string{key}
		if retentionMS > 0 {
			createArgs = append(createArgs, "RETENTION", strconv.FormatInt(retentionMS, 10))
		}
		if len(labels) > 0 {
			createArgs = append(createArgs, "LABELS")
			for k, val := range labels {
				createArgs = append(createArgs, k, val)
			}
		}
		if err := rw.writeCommand(w, "TS.CREATE", createArgs...); err != nil {
			return err
		}
		for _, sample := range samples {
			addArgs := []string{
				key,
				strconv.FormatInt(sample.Timestamp, 10),
				strconv.FormatFloat(sample.Value, 'f', -1, 64),
			}
			// LABELS must come last: cmdTSADD reads label key/value pairs from
			// the keyword to the end of the argument list, so anything after it
			// would be swallowed as another label.
			if len(sample.Labels) > 0 {
				addArgs = append(addArgs, "LABELS")
				for k, val := range sample.Labels {
					addArgs = append(addArgs, k, val)
				}
			}
			if err := rw.writeCommand(w, "TS.ADD", addArgs...); err != nil {
				return err
			}
		}
		return nil

	default:
		if raw, ok := value.(store.RedisStringValue); ok {
			data, err := raw.RedisStringBytes()
			if err != nil {
				return err
			}
			return rw.writeKeyValue(w, "SET", key, data)
		}
		return fmt.Errorf("unsupported value type %T in AOF snapshot", value)
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
	m.mu.Lock()
	defer m.mu.Unlock()
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
	wasRunning := m.writer.running.Load()
	if wasRunning {
		m.writer.Stop()
	}

	rewriteErr := m.rewriter.Rewrite(path)
	if rewriteErr == nil {
		m.writer.SetCurrentDB(0)
	} else {
		m.writer.SetCurrentDB(-1)
	}
	if wasRunning {
		if startErr := m.writer.Start(); startErr != nil {
			if rewriteErr != nil {
				return fmt.Errorf("%v; failed to restart AOF writer: %w", rewriteErr, startErr)
			}
			return startErr
		}
	}
	return rewriteErr
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

// multiDBAdapter lets a MultiDBSource serve as an AOFRewriter source.
type multiDBAdapter struct {
	MultiDBSource
}

func (a multiDBAdapter) FunctionPayload() []byte {
	if src, ok := a.MultiDBSource.(interface{ FunctionPayload() []byte }); ok {
		return src.FunctionPayload()
	}
	return nil
}

func (a multiDBAdapter) SearchCommands() [][]string {
	if src, ok := a.MultiDBSource.(interface{ SearchCommands() [][]string }); ok {
		return src.SearchCommands()
	}
	return nil
}

// EncodeCommands encodes cmds as RESP arrays, the AOF's command format.
func EncodeCommands(cmds [][]string) []byte {
	var buf []byte
	for _, cmd := range cmds {
		buf = appendArrayHeader(buf, len(cmd))
		for _, arg := range cmd {
			buf = appendBulkString(buf, arg)
		}
	}
	return buf
}

// DecodeCommands parses what EncodeCommands produced.
func DecodeCommands(data []byte) ([]Command, error) {
	reader := resp.NewReader(bytes.NewReader(data))
	var out []Command
	for {
		name, args, err := reader.ReadCommand()
		if errors.Is(err, io.EOF) {
			return out, nil
		}
		if err != nil {
			return nil, err
		}
		out = append(out, Command{Name: name, Args: args})
	}
}

func (a multiDBAdapter) GetAll() map[string]interface{} {
	out := make(map[string]interface{})
	for _, entries := range a.DatabaseEntries() {
		for k, v := range entries {
			out[k] = v
		}
	}
	return out
}

// WriteSnapshot writes every key of src as the command stream an AOF rewrite
// produces: typed write commands, PEXPIREAT for keys with a TTL and a SELECT
// before each non-empty database other than 0, ending in database 0.
func WriteSnapshot(w io.Writer, src MultiDBSource) error {
	rw := &AOFRewriter{store: multiDBAdapter{src}}
	bw := bufio.NewWriterSize(w, 64*1024)
	if err := rw.writeAll(bw); err != nil {
		return err
	}
	return bw.Flush()
}

func (w *AOFWriter) Rewrite(src MultiDBSource) error {
	if !w.config.Enabled {
		return nil
	}
	return w.replaceSnapshot(src)
}
