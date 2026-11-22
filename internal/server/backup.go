package server

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/cachestorm/cachestorm/internal/command"
	"github.com/cachestorm/cachestorm/internal/logger"
	"github.com/cachestorm/cachestorm/internal/persistence"
)

// Redis 8.10's BACKUP states.
const (
	backupIdle         = "idle"
	backupPending      = "pending"
	backupSnapshotting = "snapshotting"
	backupIncrementing = "incrementing"
	backupSealed       = "sealed"
	backupFailed       = "failed"
)

// defaultBackupDir is backupdirname's default.
const defaultBackupDir = "backupdir"

// backupManager runs BACKUP: START takes a snapshot with writes held off
// and from then on every write the AOF and replicas get also goes to the
// backup's INCR file, until SEAL writes the manifest. Unlike Redis it does
// not need the AOF on, or touch it: the backup's files are its own.
type backupManager struct {
	s *Server

	mu        sync.Mutex
	state     string
	err       string
	start     int64
	end       int64
	files     *persistence.Backup
	base      bool
	manifest  bool
	recording *persistence.Backup
}

func newBackupManager(s *Server) *backupManager {
	return &backupManager{s: s, state: backupIdle}
}

// backupDirName is CONFIG GET backupdirname.
func (s *Server) backupDirName() string {
	if s.cfg.Persistence.BackupDir != "" {
		return s.cfg.Persistence.BackupDir
	}
	return defaultBackupDir
}

func (b *backupManager) dir() string {
	return filepath.Join(b.s.persistenceDirectory(), b.s.backupDirName())
}

func (b *backupManager) inProgressLocked() bool {
	return b.state == backupPending || b.state == backupSnapshotting || b.state == backupIncrementing
}

// InProgress is INFO's backup_in_progress.
func (b *backupManager) InProgress() bool {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.inProgressLocked()
}

// recorder returns the backup recording writes, nil when none is.
func (b *backupManager) recorder() *persistence.Backup {
	if b == nil {
		return nil
	}
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.recording
}

func (b *backupManager) Start() error {
	b.mu.Lock()
	defer b.mu.Unlock()
	if b.inProgressLocked() {
		return errors.New("ERR A backup is already in progress, ABORT it first")
	}
	if b.state == backupSealed {
		return errors.New("ERR A sealed backup exists, CLEANUP it first")
	}
	dir := b.dir()
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return fmt.Errorf("ERR Can't create backup directory %s: %v", b.s.backupDirName(), errors.Unwrap(err))
	}
	if entries, err := os.ReadDir(dir); err != nil || len(entries) > 0 {
		return fmt.Errorf("ERR Backup directory %s is not empty, CLEANUP it first", b.s.backupDirName())
	}
	b.resetLocked()
	b.state = backupPending
	// The snapshot waits for the write gate, which the command asking for
	// the backup (or the EXEC or script it runs in) holds.
	go b.snapshot(dir)
	return nil
}

func (b *backupManager) snapshot(dir string) {
	s := b.s
	s.gate.global.Lock()
	b.mu.Lock()
	if b.state != backupPending {
		b.mu.Unlock()
		s.gate.global.Unlock()
		return
	}
	src, err := s.persistenceSource(s.dbs)
	var files *persistence.Backup
	if err == nil {
		files, err = persistence.StartBackup(dir, redisAOFFilename, src)
	}
	if err != nil {
		b.failLocked("failed to start the backup snapshot: " + err.Error())
		b.mu.Unlock()
		s.gate.global.Unlock()
		return
	}
	b.files, b.recording = files, files
	b.state = backupSnapshotting
	b.start = time.Now().Unix()
	b.mu.Unlock()
	s.gate.global.Unlock()

	err = files.WriteBase()
	b.mu.Lock()
	defer b.mu.Unlock()
	if b.files != files || b.state != backupSnapshotting {
		return
	}
	if err != nil {
		b.failLocked("failed to pin snapshot file: " + err.Error())
		return
	}
	b.base = true
	b.state = backupIncrementing
}

func (b *backupManager) Seal() error {
	b.mu.Lock()
	defer b.mu.Unlock()
	if b.state != backupIncrementing {
		return errors.New("ERR No backup ready to seal (must be in the incrementing state)")
	}
	files := b.files
	// Writes stop going to the INCR file from here.
	b.recording = nil
	if err := files.Seal(); err != nil {
		b.failLocked(err.Error())
		return errors.New("ERR Can't write the backup manifest")
	}
	b.manifest = true
	b.state = backupSealed
	b.end = time.Now().Unix()
	logger.Info().Str("dir", b.dir()).Msg("BACKUP: sealed backup")
	go b.expireSealed(files)
	return nil
}

// expireSealed removes a sealed backup once backup-sealed-ttl seconds have
// passed since it was sealed, as Redis' backupCron does.
func (b *backupManager) expireSealed(files *persistence.Backup) {
	ticker := time.NewTicker(time.Second)
	defer ticker.Stop()
	for {
		select {
		case <-b.s.stopCh:
			return
		case <-ticker.C:
		}
		b.mu.Lock()
		if b.files != files || b.state != backupSealed {
			b.mu.Unlock()
			return
		}
		ttl := command.BackupSealedTTL()
		if ttl > 0 && time.Now().Unix()-b.end >= ttl {
			logger.Info().Int64("seconds", time.Now().Unix()-b.end).Msg("BACKUP: auto cleaning sealed backup")
			if err := persistence.RemoveFiles(b.dir()); err != nil {
				logger.Warn().Err(err).Msg("BACKUP: removing the sealed backup failed")
			}
			b.resetLocked()
			b.mu.Unlock()
			return
		}
		b.mu.Unlock()
	}
}

func (b *backupManager) Abort() error {
	b.mu.Lock()
	defer b.mu.Unlock()
	if !b.inProgressLocked() {
		return errors.New("ERR No backup in progress")
	}
	b.failLocked("aborted by user")
	return nil
}

func (b *backupManager) Cleanup() error {
	b.mu.Lock()
	defer b.mu.Unlock()
	if b.inProgressLocked() {
		return errors.New("ERR Backup is in progress")
	}
	if err := persistence.RemoveFiles(b.dir()); err != nil {
		return fmt.Errorf("ERR %v", err)
	}
	b.resetLocked()
	return nil
}

func (b *backupManager) Status() command.BackupStatus {
	b.mu.Lock()
	defer b.mu.Unlock()
	return command.BackupStatus{State: b.state, Error: b.err, StartTime: b.start, EndTime: b.end}
}

func (b *backupManager) List() []string {
	b.mu.Lock()
	defer b.mu.Unlock()
	var out []string
	if b.files == nil {
		return out
	}
	dir, err := filepath.Abs(b.dir())
	if err != nil {
		return out
	}
	if b.base {
		out = append(out, filepath.Join(dir, b.files.BaseName()))
	}
	if b.manifest {
		out = append(out, filepath.Join(dir, b.files.IncrName()), filepath.Join(dir, b.files.ManifestName()))
	}
	return out
}

// failLocked gives up the backup in progress: its files are removed and
// the state is failed, keeping the error and the start time to show.
func (b *backupManager) failLocked(reason string) {
	if !b.inProgressLocked() {
		return
	}
	b.state = backupFailed
	b.err = reason
	b.recording = nil
	if b.files != nil {
		b.files.Close()
	}
	b.files, b.base, b.manifest = nil, false, false
	b.end = 0
	if err := persistence.RemoveFiles(b.dir()); err != nil {
		logger.Warn().Err(err).Msg("BACKUP: removing the failed backup's files failed")
	}
	logger.Warn().Str("error", reason).Msg("BACKUP: failed")
}

func (b *backupManager) resetLocked() {
	b.state, b.err = backupIdle, ""
	b.start, b.end = 0, 0
	b.files, b.recording = nil, nil
	b.base, b.manifest = false, false
}

// stop gives up a backup in progress when the server stops.
func (b *backupManager) stop() {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.failLocked("server stopped")
}
