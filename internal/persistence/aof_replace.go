package persistence

import (
	"bufio"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"time"

	"github.com/cachestorm/cachestorm/internal/logger"
)

type aofReplaceOps struct {
	create  func(string) (*os.File, error)
	write   func(io.Writer, MultiDBSource) error
	sync    func(*os.File) error
	rename  func(string, string) error
	syncDir func(string) error
	link    func(string, string) error
}

func defaultAOFReplaceOps() aofReplaceOps {
	return aofReplaceOps{
		create:  newAOFTemp,
		write:   WriteSnapshot,
		sync:    func(f *os.File) error { return f.Sync() },
		rename:  os.Rename,
		syncDir: SyncDataDirectory,
		link:    os.Link,
	}
}

func newAOFTemp(dir string) (*os.File, error) {
	for i := 0; i < 10; i++ {
		var token [16]byte
		if _, err := rand.Read(token[:]); err != nil {
			return nil, err
		}
		name := filepath.Join(dir, ".aof-rewrite-"+hex.EncodeToString(token[:]))
		f, err := openAOFFile(name, os.O_CREATE|os.O_EXCL|os.O_RDWR|os.O_APPEND, 0600)
		if !os.IsExist(err) {
			return f, err
		}
	}
	return nil, errors.New("could not allocate an owned AOF temporary file")
}

func SyncDataDirectory(path string) error {
	if runtime.GOOS == "windows" {
		return nil
	}
	dir, err := os.Open(path)
	if err != nil {
		return err
	}
	defer dir.Close()
	return dir.Sync()
}

func backupAOF(path string, ops aofReplaceOps) (string, error) {
	if _, err := os.Stat(path); os.IsNotExist(err) {
		return "", nil
	} else if err != nil {
		return "", err
	}
	backup, err := ops.create(filepath.Dir(path))
	if err != nil {
		return "", err
	}
	name := backup.Name()
	if err := backup.Close(); err != nil {
		os.Remove(name)
		return "", err
	}
	if err := os.Remove(name); err != nil {
		return "", err
	}
	if err := ops.link(path, name); err == nil {
		return name, nil
	} else if os.IsExist(err) {
		return "", err
	}
	backup, err = openAOFFile(name, os.O_CREATE|os.O_EXCL|os.O_RDWR|os.O_APPEND, 0600)
	if err != nil {
		return "", err
	}
	ok := false
	defer func() {
		backup.Close()
		if !ok {
			os.Remove(name)
		}
	}()
	source, err := openAOFFile(path, os.O_RDONLY, 0)
	if err != nil {
		return "", err
	}
	defer source.Close()
	if _, err := io.Copy(backup, source); err != nil {
		return "", err
	}
	if err := ops.sync(backup); err != nil {
		return "", err
	}
	ok = true
	return name, nil
}

func (w *AOFWriter) replaceSnapshot(src MultiDBSource) error {
	w.lifecycle.Lock()
	defer w.lifecycle.Unlock()
	r, err := w.beginRewrite(src)
	if err != nil {
		return err
	}
	return w.finishRewrite(r)
}

// AOFRewrite is a rewrite BeginRewrite started: the dataset is encoded in
// memory and the commands appended since are kept, until Finish writes both
// to a new file and switches to it.
type AOFRewrite struct {
	w        *AOFWriter
	snapshot *pieceBuffer
}

// aofTail is what was appended since a rewrite's snapshot. It starts with
// a SELECT of the database the file was in, as the snapshot ends in 0.
type aofTail struct {
	buf      []byte
	endDB    int
	appended bool
}

// BeginRewrite encodes src for a rewrite and starts keeping the commands
// appended from now on. The caller holds writes off while it runs; Finish
// does the slow part, writing and syncing the new file, with writes going
// on. It returns nil when the AOF is disabled. Stop waits for Finish.
func (w *AOFWriter) BeginRewrite(src MultiDBSource) (*AOFRewrite, error) {
	if !w.config.Enabled {
		return nil, nil
	}
	w.lifecycle.Lock()
	r, err := w.beginRewrite(src)
	if err != nil {
		w.lifecycle.Unlock()
		return nil, err
	}
	return r, nil
}

// Finish writes the rewrite's file and switches to it.
func (r *AOFRewrite) Finish() error {
	defer r.w.lifecycle.Unlock()
	return r.w.finishRewrite(r)
}

func (w *AOFWriter) aofReplaceOps() aofReplaceOps {
	if w.replaceOps.create == nil {
		return defaultAOFReplaceOps()
	}
	return w.replaceOps
}

func (w *AOFWriter) beginRewrite(src MultiDBSource) (*AOFRewrite, error) {
	w.mu.Lock()
	err := w.fatalErr
	w.mu.Unlock()
	if err != nil {
		return nil, err
	}
	snapshot := &pieceBuffer{}
	if err := w.aofReplaceOps().write(snapshot, src); err != nil {
		return nil, err
	}
	w.mu.Lock()
	defer w.mu.Unlock()
	if w.fatalErr != nil {
		return nil, w.fatalErr
	}
	tail := &aofTail{}
	if w.curDB > 0 {
		tail.buf = appendCommand(nil, "SELECT", [][]byte{strconv.AppendInt(nil, int64(w.curDB), 10)})
		tail.endDB = w.curDB
	}
	w.tail = tail
	return &AOFRewrite{w: w, snapshot: snapshot}, nil
}

func (w *AOFWriter) finishRewrite(r *AOFRewrite) (err error) {
	defer func() {
		if err != nil {
			w.mu.Lock()
			w.tail = nil
			w.mu.Unlock()
		}
	}()
	ops := w.aofReplaceOps()
	path := filepath.Join(w.config.DataDir, w.config.Filename)
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return err
	}
	prepared, err := ops.create(dir)
	if err != nil {
		return err
	}
	adopted := false
	defer func() {
		if !adopted {
			prepared.Close()
		}
		os.Remove(prepared.Name())
	}()
	if err := writeAOFHeader(prepared); err != nil {
		return err
	}
	bw := bufio.NewWriterSize(prepared, 256*1024)
	for i, piece := range r.snapshot.pieces {
		if _, err := bw.Write(piece); err != nil {
			return err
		}
		r.snapshot.pieces[i] = nil
	}
	if err := bw.Flush(); err != nil {
		return err
	}
	r.snapshot = nil
	if err := ops.sync(prepared); err != nil {
		return err
	}
	// The commands appended meanwhile go after the snapshot, with appends
	// held off until the new file has replaced the old one.
	w.mu.Lock()
	defer w.mu.Unlock()
	tail := w.tail
	w.tail = nil
	if w.fatalErr != nil {
		return w.fatalErr
	}
	if tail == nil {
		return errors.New("AOF rewrite lost its tail")
	}
	endDB := tail.endDB
	if tail.appended {
		endDB = w.curDB
	}
	if len(tail.buf) > 0 {
		if _, err := prepared.Write(tail.buf); err != nil {
			return err
		}
		if err := ops.sync(prepared); err != nil {
			return err
		}
	}
	info, err := prepared.Stat()
	if err != nil {
		return err
	}
	if w.running.Load() {
		if err := w.writer.Flush(); err != nil {
			return err
		}
		if err := ops.sync(w.file); err != nil {
			return err
		}
	}
	backup, err := backupAOF(path, ops)
	if err != nil {
		return err
	}
	keepBackup := false
	var backupFile *os.File
	defer func() {
		if backupFile != nil {
			backupFile.Close()
		}
		if backup != "" && !keepBackup {
			os.Remove(backup)
		}
	}()
	if backup != "" && w.running.Load() {
		backupFile, err = openAOFFile(backup, os.O_RDWR|os.O_APPEND, 0600)
		if err != nil {
			return err
		}
	}
	if err := ops.rename(prepared.Name(), path); err != nil {
		return err
	}
	if syncErr := ops.syncDir(dir); syncErr != nil {
		var rollbackErr error
		recoveryPath := backup
		if backup != "" {
			rollbackErr = ops.rename(backup, path)
		} else {
			rollbackErr = os.Remove(path)
		}
		if rollbackErr == nil {
			recoveryPath = path
			rollbackErr = ops.syncDir(dir)
		}
		if rollbackErr != nil {
			keepBackup = true
			w.fatalErr = fmt.Errorf("AOF replacement rollback failed (recovery path %q): %w", recoveryPath, errors.Join(syncErr, rollbackErr))
			return w.fatalErr
		}
		if backupFile != nil {
			old := w.file
			w.file, w.writer = backupFile, bufio.NewWriterSize(backupFile, 8192)
			backupFile = nil
			if err := old.Close(); err != nil {
				logger.Warn().Err(err).Msg("previous AOF handle close failed")
			}
		}
		return syncErr
	}
	if w.running.Load() {
		old := w.file
		w.file, w.writer = prepared, bufio.NewWriterSize(prepared, 8192)
		adopted = true
		if err := old.Close(); err != nil {
			logger.Warn().Err(err).Msg("previous AOF handle close failed")
		}
	}
	w.curDB = endDB
	w.size.Store(info.Size())
	w.dirty.Store(0)
	w.lastSync = time.Now()
	return nil
}
