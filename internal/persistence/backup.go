package persistence

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"sync"
)

// Backup is the set of files Redis 8.10's BACKUP leaves in its directory: a
// multi-part AOF a Redis or CacheStorm server loads as its appendonlydir.
// The BASE file holds the dataset as it was when the backup started, the
// INCR file every write after that until the backup is sealed, and the
// manifest names the two:
//
//	<filename>.1.base.aof   the snapshot, as commands
//	<filename>.1.incr.aof   the writes made while the backup was open
//	<filename>.manifest     written when the backup is sealed
type Backup struct {
	dir, filename string
	snapshot      *pieceBuffer

	mu     sync.Mutex
	incr   *os.File
	buf    []byte
	curDB  int
	err    error
	closed bool
}

// backupFlushAt is how many bytes of writes the INCR file buffers before
// they go to the file.
const backupFlushAt = 64 * 1024

// BaseName, IncrName and ManifestName are the names of a backup's files.
func (b *Backup) BaseName() string     { return b.filename + ".1.base.aof" }
func (b *Backup) IncrName() string     { return b.filename + ".1.incr.aof" }
func (b *Backup) ManifestName() string { return b.filename + ".manifest" }

// StartBackup encodes src into memory and opens the INCR file in dir; from
// then on Append records writes. The caller holds writes off while it runs,
// so the snapshot and the INCR file meet exactly.
func StartBackup(dir, filename string, src MultiDBSource) (*Backup, error) {
	b := &Backup{dir: dir, filename: filename, snapshot: &pieceBuffer{}}
	if err := WriteSnapshot(b.snapshot, src); err != nil {
		return nil, err
	}
	f, err := os.OpenFile(filepath.Join(dir, b.IncrName()), os.O_CREATE|os.O_EXCL|os.O_WRONLY|os.O_APPEND, 0o600)
	if err != nil {
		return nil, err
	}
	b.incr = f
	b.buf = append(b.buf, aofVersionLine...)
	return b, nil
}

// WriteBase writes the snapshot to the BASE file and syncs it.
func (b *Backup) WriteBase() error {
	f, err := os.OpenFile(filepath.Join(b.dir, b.BaseName()), os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o600)
	if err != nil {
		return err
	}
	defer f.Close()
	if err := writeAOFHeader(f); err != nil {
		return err
	}
	for i, piece := range b.snapshot.pieces {
		if _, err := f.Write(piece); err != nil {
			return err
		}
		b.snapshot.pieces[i] = nil
	}
	b.snapshot = nil
	if err := f.Sync(); err != nil {
		return err
	}
	return f.Close()
}

func (b *Backup) selectDB(db int) {
	if db != b.curDB {
		b.buf = appendCommand(b.buf, "SELECT", [][]byte{strconv.AppendInt(nil, int64(db), 10)})
		b.curDB = db
	}
}

// Append records a write made in database db.
func (b *Backup) Append(db int, cmd string, args [][]byte) {
	b.mu.Lock()
	defer b.mu.Unlock()
	if b.closed || b.err != nil {
		return
	}
	b.selectDB(db)
	b.buf = appendCommand(b.buf, cmd, args)
	b.flushIfFullLocked()
}

// AppendTransaction records the writes of an EXEC or a script as one
// MULTI ... EXEC block.
func (b *Backup) AppendTransaction(cmds []Command) {
	b.mu.Lock()
	defer b.mu.Unlock()
	if b.closed || b.err != nil || len(cmds) == 0 {
		return
	}
	b.buf = appendCommand(b.buf, "MULTI", nil)
	for _, c := range cmds {
		b.selectDB(c.DB)
		b.buf = appendCommand(b.buf, c.Name, c.Args)
	}
	b.buf = appendCommand(b.buf, "EXEC", nil)
	b.flushIfFullLocked()
}

func (b *Backup) flushIfFullLocked() {
	if len(b.buf) >= backupFlushAt {
		b.flushLocked()
	}
}

func (b *Backup) flushLocked() {
	if len(b.buf) == 0 || b.err != nil {
		return
	}
	if _, err := b.incr.Write(b.buf); err != nil {
		b.err = fmt.Errorf("failed to write INCR file: %w", err)
	}
	b.buf = b.buf[:0]
}

// Err reports a failed write to the INCR file.
func (b *Backup) Err() error {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.err
}

// Seal stops recording writes, syncs the INCR file and writes the manifest.
func (b *Backup) Seal() error {
	b.mu.Lock()
	defer b.mu.Unlock()
	if b.closed {
		return errors.New("backup already closed")
	}
	b.flushLocked()
	b.closed = true
	err := b.err
	if err == nil {
		if serr := b.incr.Sync(); serr != nil {
			err = fmt.Errorf("failed to fsync incr file: %w", serr)
		}
	}
	if cerr := b.incr.Close(); err == nil && cerr != nil {
		err = cerr
	}
	if err != nil {
		return err
	}
	manifest := fmt.Sprintf("file %s seq 1 type b\nfile %s seq 1 type i\n", b.BaseName(), b.IncrName())
	return WriteFileAtomic(filepath.Join(b.dir, b.ManifestName()), []byte(manifest))
}

// Close stops recording writes, for a backup given up.
func (b *Backup) Close() {
	b.mu.Lock()
	defer b.mu.Unlock()
	if !b.closed {
		b.closed = true
		b.incr.Close()
	}
}

// RemoveFiles removes the regular files in dir, not descending into
// directories, as Redis' BACKUP CLEANUP does.
func RemoveFiles(dir string) error {
	entries, err := os.ReadDir(dir)
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	if err != nil {
		return err
	}
	for _, e := range entries {
		if e.Type().IsRegular() || e.Type()&os.ModeSymlink != 0 {
			if err := os.Remove(filepath.Join(dir, e.Name())); err != nil && !errors.Is(err, os.ErrNotExist) {
				return err
			}
		}
	}
	return nil
}
