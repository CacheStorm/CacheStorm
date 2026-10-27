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
	w.mu.Lock()
	defer w.mu.Unlock()
	if w.fatalErr != nil {
		return w.fatalErr
	}
	ops := w.replaceOps
	if ops.create == nil {
		ops = defaultAOFReplaceOps()
	}
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
	if err := ops.write(prepared, src); err != nil {
		return err
	}
	if err := ops.sync(prepared); err != nil {
		return err
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
	w.curDB = 0
	w.size.Store(info.Size())
	w.dirty.Store(0)
	w.lastSync = time.Now()
	return nil
}
