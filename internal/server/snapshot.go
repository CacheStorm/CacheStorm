package server

import (
	"bytes"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/cachestorm/cachestorm/internal/command"
	"github.com/cachestorm/cachestorm/internal/logger"
	"github.com/cachestorm/cachestorm/internal/persistence"
	"github.com/cachestorm/cachestorm/internal/resp"
)

func (s *Server) snapshotPath() string {
	dir := s.persistenceDirectory()
	return filepath.Join(dir, "dump.rdb")
}

func (s *Server) configureSnapshots(restore bool) error {
	s.snapshotLastTime = time.Now().Unix()
	s.router.SetSnapshotHooks(s.saveSnapshot, s.lastSnapshotTime, s.snapshotInfo)
	if !s.cfg.Persistence.Enabled {
		return nil
	}
	if interval := s.cfg.Persistence.SnapshotInterval; interval != "" {
		var err error
		s.snapshotInterval, err = time.ParseDuration(interval)
		if err != nil || s.snapshotInterval < 0 {
			return fmt.Errorf("invalid persistence.snapshot_interval: %q", interval)
		}
	}
	path := s.snapshotPath()
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return err
	}
	s.snapshotWrite = func(data []byte) error { return persistence.WriteRDBFile(path, data) }
	if !restore {
		return nil
	}
	if _, err := os.Stat(path); errors.Is(err, os.ErrNotExist) {
		return nil
	} else if err != nil {
		return err
	}
	reader := persistence.NewRDBReaderForDatabases(s.dbs)
	if err := reader.Load(path); err != nil {
		return fmt.Errorf("load snapshot %s: %w", path, err)
	}
	functions := []byte(reader.AuxFields["cachestorm-functions"])
	if len(functions) > 0 {
		if _, err := s.snapshotFunctionCommand([][]byte{[]byte("RESTORE"), functions, []byte("FLUSH")}); err != nil {
			return fmt.Errorf("restore snapshot functions: %w", err)
		}
	}
	if err := s.restoreSearch([]byte(reader.AuxFields[snapshotSearchAux])); err != nil {
		return fmt.Errorf("restore snapshot search indexes: %w", err)
	}

	if s.aof != nil {
		src, err := s.persistenceSource(s.dbs)
		if err == nil {
			err = s.aof.Rewrite(src)
		}
		if err != nil {
			return fmt.Errorf("seed AOF from snapshot: %w", err)
		}
	}
	return nil
}

// snapshotSearchAux is the snapshot aux field that holds the search indexes,
// as the FT.CREATE, FT.ADD and FT.ALIASADD commands that rebuild them.
const snapshotSearchAux = "cachestorm-search"

func (s *Server) restoreSearch(payload []byte) error {
	cmds, err := persistence.DecodeCommands(payload)
	if err != nil {
		return err
	}
	for _, cmd := range cmds {
		name := command.CanonicalName(cmd.Name)
		if name != "FT.CREATE" && name != "FT.ADD" && name != "FT.ALIASADD" {
			return fmt.Errorf("unexpected command %s", name)
		}
		ctx := command.NewContext(name, cmd.Args, s.Store(), nil)
		if err := s.router.ExecuteSilent(ctx); err != nil {
			return err
		}
		if ctx.Writer.Errors() > 0 {
			return fmt.Errorf("%s returned an error reply", name)
		}
	}
	return nil
}

func (s *Server) snapshotFunctionCommand(args [][]byte) ([]byte, error) {
	var buf bytes.Buffer
	ctx := command.NewContext("FUNCTION", args, s.Store(), resp.NewWriter(&buf))
	if err := s.router.ExecuteSilent(ctx); err != nil {
		return nil, err
	}
	value, err := resp.NewReader(&buf).ReadValue()
	if err != nil {
		return nil, err
	}
	if value.Type == resp.TypeError {
		return nil, errors.New(value.Err)
	}
	return value.Bulk, nil
}

func (s *Server) lastSnapshotTime() int64 {
	s.snapshotMu.Lock()
	defer s.snapshotMu.Unlock()
	return s.snapshotLastTime
}

func (s *Server) snapshotInfo() string {
	s.snapshotMu.Lock()
	defer s.snapshotMu.Unlock()
	running, aof := 0, 0
	if s.snapshotRunning {
		running = 1
	}
	if s.aof != nil {
		aof = 1
	}
	status := "ok"
	aofStatus := "ok"
	if s.persistenceError() != nil {
		aofStatus = "err"
	}
	if s.snapshotLastError != nil {
		status = "err"
	}
	return fmt.Sprintf("# Persistence\r\nloading:0\r\nrdb_bgsave_in_progress:%d\r\nrdb_last_save_time:%d\r\nrdb_last_bgsave_status:%s\r\naof_enabled:%d\r\naof_last_write_status:%s\r\n%s\r\n", running, s.snapshotLastTime, status, aof, aofStatus, s.aofRewriteInfo())
}

func (s *Server) saveSnapshot(background bool) error {
	s.snapshotMu.Lock()
	if !s.cfg.Persistence.Enabled {
		s.snapshotMu.Unlock()
		return errors.New("ERR persistence is not configured")
	}
	if s.snapshotClosing || s.stopping.Load() {
		s.snapshotMu.Unlock()
		return errors.New("ERR server is stopping")
	}
	if s.snapshotRunning {
		s.snapshotMu.Unlock()
		return errors.New("ERR Background save already in progress")
	}
	s.snapshotRunning = true
	s.snapshotWG.Add(1)
	write := s.snapshotWrite
	s.snapshotMu.Unlock()
	finish := func(err error) {
		s.snapshotMu.Lock()
		s.snapshotRunning = false
		s.snapshotLastError = err
		if err == nil {
			s.snapshotLastTime = time.Now().Unix()
		}
		s.snapshotMu.Unlock()
		s.snapshotWG.Done()
	}
	if s.aof != nil {
		if err := s.aof.Flush(); err != nil {
			finish(err)
			return fmt.Errorf("ERR snapshot failed: %w", err)
		}
	}
	functions, err := s.FunctionPayload()
	if err != nil {
		finish(err)
		return fmt.Errorf("ERR snapshot functions: %w", err)
	}
	aux := map[string]string{"cachestorm-functions": string(functions)}
	if searchCmds := s.SearchCommands(); len(searchCmds) > 0 {
		aux[snapshotSearchAux] = string(persistence.EncodeCommands(searchCmds))
	}
	var data bytes.Buffer
	// The checksum is computed after the encoder returns, so a background
	// save holds the write gate only while the dataset is encoded.
	writer := persistence.NewRDBWriterForDatabases(s.dbs, persistence.RDBConfig{Version: persistence.RDBVersion11, AuxFields: aux})
	if err := writer.Encode(&data); err != nil {
		finish(err)
		return fmt.Errorf("ERR snapshot failed: %w", err)
	}
	store := write
	write = func(data []byte) error {
		if err := persistence.SealRDBChecksum(data); err != nil {
			return err
		}
		return store(data)
	}
	if !background {
		err := write(data.Bytes())
		finish(err)
		if err != nil {
			return fmt.Errorf("ERR snapshot failed: %w", err)
		}
		return nil
	}
	go func() {
		err := write(data.Bytes())
		finish(err)
		if err != nil {
			logger.Error().Err(err).Msg("background snapshot failed")
		}
	}()
	return nil
}

func (s *Server) periodicSnapshot() {
	s.gate.global.Lock()
	defer s.gate.global.Unlock()
	if err := s.saveSnapshot(true); err != nil {
		logger.Debug().Err(err).Msg("periodic snapshot skipped")
	}
}

func (s *Server) startSnapshotLoop() {
	if !s.cfg.Persistence.Enabled || s.snapshotInterval <= 0 {
		return
	}
	s.wg.Add(1)
	go func() {
		defer s.wg.Done()
		ticker := time.NewTicker(s.snapshotInterval)
		defer ticker.Stop()
		for {
			select {
			case <-s.stopCh:
				return
			case <-ticker.C:
				s.periodicSnapshot()
			}
		}
	}()
}

func (s *Server) stopSnapshots() {
	s.snapshotMu.Lock()
	s.snapshotClosing = true
	s.snapshotMu.Unlock()
	s.snapshotWG.Wait()
}
