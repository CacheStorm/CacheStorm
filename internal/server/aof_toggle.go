package server

import (
	"errors"
	"os"
	"time"

	"github.com/cachestorm/cachestorm/internal/logger"
	"github.com/cachestorm/cachestorm/internal/persistence"
)

// setAppendOnly is CONFIG SET appendonly. As in Redis it answers at once and
// switches the AOF in the background: on, it writes the dataset to a new
// appendonly.aof as a rewrite does, holding writes only while the dataset is
// encoded in memory, then appends every write; off, it writes out and closes
// the AOF. CONFIG GET and INFO report the asked-for state meanwhile.
func (s *Server) setAppendOnly(on bool) error {
	if on && !s.cfg.Persistence.Enabled {
		return errors.New("ERR persistence is disabled (persistence.enabled: false), so the AOF cannot be switched on")
	}
	s.snapshotMu.Lock()
	defer s.snapshotMu.Unlock()
	if s.snapshotClosing || s.stopping.Load() {
		return errors.New("ERR server is stopping")
	}
	s.aofWanted.Store(on)
	s.snapshotWG.Add(1)
	go func() {
		defer s.snapshotWG.Done()
		if err := s.switchAOF(); err != nil {
			logger.Error().Err(err).Msg("switching the AOF failed")
		}
	}()
	return nil
}

// switchAOF brings the AOF to the asked-for state. Switches and rewrites run
// one at a time, so the latest CONFIG SET wins.
func (s *Server) switchAOF() error {
	s.aofToggleMu.Lock()
	defer s.aofToggleMu.Unlock()
	if s.aofWanted.Load() {
		return s.startAOF()
	}
	return s.stopAOF()
}

func (s *Server) startAOF() error {
	if s.aof() != nil || s.stopping.Load() {
		return nil
	}
	start := time.Now()
	dir := s.persistenceDirectory()
	if err := os.MkdirAll(dir, 0755); err != nil {
		s.aofWanted.Store(false)
		return err
	}
	w := persistence.NewAOFWriter(persistence.AOFConfig{
		Enabled:    true,
		Filename:   "appendonly.aof",
		DataDir:    dir,
		SyncPolicy: persistence.AOFSyncPolicy(s.aofPolicy.Load()),
	})
	if err := w.Start(); err != nil {
		s.aofWanted.Store(false)
		return err
	}
	// The dataset becomes the new file's start; writes are published to
	// the writer before the gate opens again, so the ones made while the
	// file is written are kept and follow it.
	s.gate.global.Lock()
	src, err := s.persistenceSource(s.dbs)
	var rewrite *persistence.AOFRewrite
	if err == nil {
		rewrite, err = w.BeginRewrite(src)
	}
	if err == nil {
		s.aofW.Store(w)
		s.installCommitHooks()
	}
	s.gate.global.Unlock()
	if err == nil {
		err = rewrite.Finish()
	}
	if err != nil {
		if s.aof() == w {
			s.gate.global.Lock()
			s.aofW.Store(nil)
			s.installCommitHooks()
			s.gate.global.Unlock()
		}
		s.aofWanted.Store(false)
		return errors.Join(err, w.StopWithError())
	}
	s.snapshotMu.Lock()
	s.aofRewriteBase = w.Size()
	s.snapshotMu.Unlock()
	s.router.SetAOFRewriteHook(s.startAOFRewrite)
	logger.Info().Int64("size", w.Size()).Dur("took", time.Since(start)).Msg("AOF switched on")
	return nil
}

func (s *Server) stopAOF() error {
	w := s.aof()
	if w == nil {
		return nil
	}
	// No write is between its apply and its append while the gate is held.
	s.gate.global.Lock()
	s.aofW.Store(nil)
	s.installCommitHooks()
	s.gate.global.Unlock()
	err := w.StopWithError()
	logger.Info().Msg("AOF switched off")
	return err
}

// setAppendFsync is CONFIG SET appendfsync.
func (s *Server) setAppendFsync(policy persistence.AOFSyncPolicy) error {
	s.aofToggleMu.Lock()
	defer s.aofToggleMu.Unlock()
	s.aofPolicy.Store(int32(policy))
	if w := s.aof(); w != nil {
		return w.SetSyncPolicy(policy)
	}
	return nil
}
