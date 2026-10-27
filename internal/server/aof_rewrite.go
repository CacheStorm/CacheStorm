package server

import (
	"errors"
	"fmt"
	"time"

	"github.com/cachestorm/cachestorm/internal/config"
	"github.com/cachestorm/cachestorm/internal/logger"
)

// aofRewriteCheckInterval is how often the AOF size is compared with the
// automatic rewrite threshold.
const aofRewriteCheckInterval = time.Second

var errAOFRewriteRunning = errors.New("ERR Background append only file rewriting already in progress")

// configureAOFRewrite installs the BGREWRITEAOF hook and reads the automatic
// rewrite thresholds. It is called once the AOF writer exists.
func (s *Server) configureAOFRewrite() error {
	if s.aof == nil {
		return nil
	}
	minSize, err := config.ParseMemorySize(s.cfg.Persistence.AOFRewriteMinSize)
	if err != nil || minSize < 0 {
		return fmt.Errorf("invalid persistence.aof_rewrite_min_size: %q", s.cfg.Persistence.AOFRewriteMinSize)
	}
	s.aofRewriteMinSize = minSize
	s.aofRewritePercentage = int64(s.cfg.Persistence.AOFRewritePercentage)
	s.aofRewriteBase = s.aof.Size()
	s.router.SetAOFRewriteHook(s.startAOFRewrite)
	return nil
}

// startAOFRewrite schedules a rewrite in the background. The rewrite takes the
// write gate exclusively, so it starts once the command that asked for it has
// released the gate, and no write can land between the snapshot and the
// switch to the new file.
func (s *Server) startAOFRewrite() error {
	s.snapshotMu.Lock()
	defer s.snapshotMu.Unlock()
	if s.snapshotClosing || s.stopping.Load() {
		return errors.New("ERR server is stopping")
	}
	if s.aofRewriteRunning {
		return errAOFRewriteRunning
	}
	s.aofRewriteRunning = true
	s.snapshotWG.Add(1)
	go func() {
		defer s.snapshotWG.Done()
		s.gate.global.Lock()
		err := s.rewriteAOFLocked()
		s.gate.global.Unlock()
		if err != nil {
			logger.Error().Err(err).Msg("AOF rewrite failed")
		}
	}()
	return nil
}

// rewriteAOFLocked replaces the AOF with the current dataset. The caller holds
// the write gate exclusively.
func (s *Server) rewriteAOFLocked() error {
	start := time.Now()
	src, err := s.persistenceSource(s.dbs)
	if err == nil {
		err = s.aof.Rewrite(src)
	}
	size := s.aof.Size()
	s.snapshotMu.Lock()
	s.aofRewriteRunning = false
	s.aofRewriteLastError = err
	if err == nil {
		s.aofRewriteBase = size
		s.aofRewrites++
	}
	s.snapshotMu.Unlock()
	if err == nil {
		logger.Info().Int64("size", size).Dur("took", time.Since(start)).Msg("AOF rewritten")
	}
	return err
}

// aofRewriteDue reports whether the AOF has grown past the automatic rewrite
// threshold.
func (s *Server) aofRewriteDue() bool {
	return s.aofRewriteDueForSize(s.aof.Size())
}

func (s *Server) aofRewriteDueForSize(size int64) bool {
	if s.aofRewritePercentage <= 0 {
		return false
	}
	s.snapshotMu.Lock()
	base, running, failed := s.aofRewriteBase, s.aofRewriteRunning, s.aofRewriteLastError != nil
	s.snapshotMu.Unlock()
	if running || size < s.aofRewriteMinSize {
		return false
	}
	if base <= 0 {
		base = 1
	}
	if failed {
		// A failed rewrite is retried only after the file grows again, so a
		// persistent failure (disk full) does not rewrite in a tight loop.
		return size >= base*2
	}
	return (size-base)*100/base >= s.aofRewritePercentage
}

func (s *Server) startAOFRewriteLoop() {
	if s.aof == nil || s.aofRewritePercentage <= 0 {
		return
	}
	s.wg.Add(1)
	go func() {
		defer s.wg.Done()
		ticker := time.NewTicker(aofRewriteCheckInterval)
		defer ticker.Stop()
		for {
			select {
			case <-s.stopCh:
				return
			case <-ticker.C:
				if !s.aofRewriteDue() {
					continue
				}
				if err := s.startAOFRewrite(); err != nil && !errors.Is(err, errAOFRewriteRunning) {
					logger.Debug().Err(err).Msg("automatic AOF rewrite skipped")
				}
			}
		}
	}()
}

func (s *Server) aofRewriteInfo() string {
	if s.aof == nil {
		return ""
	}
	running, status := 0, "ok"
	if s.aofRewriteRunning {
		running = 1
	}
	if s.aofRewriteLastError != nil {
		status = "err"
	}
	return fmt.Sprintf("aof_rewrite_in_progress:%d\r\naof_rewrite_scheduled:0\r\naof_last_bgrewrite_status:%s\r\naof_rewrites:%d\r\naof_current_size:%d\r\naof_base_size:%d\r\n",
		running, status, s.aofRewrites, s.aof.Size(), s.aofRewriteBase)
}
