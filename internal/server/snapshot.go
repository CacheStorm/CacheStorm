package server

import (
	"bytes"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
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

func boolInt(b bool) int {
	if b {
		return 1
	}
	return 0
}

func (s *Server) snapshotInfo() string {
	s.snapshotMu.Lock()
	defer s.snapshotMu.Unlock()
	status, aofStatus, rewriteStatus := "ok", "ok", "ok"
	if s.snapshotLastError != nil {
		status = "err"
	}
	if s.persistenceError() != nil {
		aofStatus = "err"
	}
	if s.aofRewriteLastError != nil {
		rewriteStatus = "err"
	}
	seconds := func(d time.Duration) int64 {
		if d < 0 {
			return -1
		}
		return int64(d / time.Second)
	}
	current := func(running bool, started time.Time) int64 {
		if !running {
			return -1
		}
		return int64(time.Since(started) / time.Second)
	}
	var b strings.Builder
	field := func(name string, value any) { fmt.Fprintf(&b, "%s:%v\r\n", name, value) }
	b.WriteString("# Persistence\r\n")
	field("loading", 0)
	field("async_loading", 0)
	field("current_cow_peak", 0)
	field("current_cow_size", 0)
	field("current_cow_size_age", 0)
	field("current_fork_perc", "0.00")
	field("current_save_keys_processed", 0)
	field("current_save_keys_total", 0)
	field("rdb_changes_since_last_save", command.Dirty())
	field("rdb_bgsave_in_progress", boolInt(s.snapshotRunning))
	field("rdb_last_save_time", s.snapshotLastTime)
	field("rdb_last_bgsave_status", status)
	field("rdb_last_bgsave_time_sec", seconds(s.snapshotTook))
	field("rdb_current_bgsave_time_sec", current(s.snapshotRunning, s.snapshotStarted))
	field("rdb_saves", s.rdbSaves)
	field("rdb_last_cow_size", 0)
	field("rdb_last_load_keys_expired", 0)
	field("rdb_last_load_keys_loaded", 0)
	field("aof_enabled", boolInt(s.aof != nil))
	field("aof_rewrite_in_progress", boolInt(s.aofRewriteRunning))
	field("aof_rewrite_scheduled", 0)
	field("aof_last_rewrite_time_sec", seconds(s.aofRewriteTook))
	field("aof_current_rewrite_time_sec", current(s.aofRewriteRunning, s.aofRewriteStarted))
	field("aof_last_bgrewrite_status", rewriteStatus)
	field("aof_rewrites", s.aofRewrites)
	field("aof_rewrites_consecutive_failures", s.aofRewriteFailures)
	field("aof_last_write_status", aofStatus)
	field("aof_last_cow_size", 0)
	field("module_fork_in_progress", 0)
	field("module_fork_last_cow_size", 0)
	if s.aof != nil {
		field("aof_current_size", s.aof.Size())
		field("aof_base_size", s.aofRewriteBase)
		field("aof_pending_rewrite", 0)
		field("aof_buffer_length", 0)
		field("aof_pending_bio_fsync", 0)
		field("aof_delayed_fsync", 0)
	}
	b.WriteString("\r\n")
	return b.String()
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
	s.snapshotStarted = time.Now()
	s.snapshotWG.Add(1)
	write := s.snapshotWrite
	s.snapshotMu.Unlock()
	covered := command.Dirty()
	finish := func(err error) {
		s.snapshotMu.Lock()
		s.snapshotRunning = false
		s.snapshotLastError = err
		s.snapshotTook = time.Since(s.snapshotStarted)
		if err == nil {
			s.snapshotLastTime = time.Now().Unix()
			s.rdbSaves++
			command.SaveDone(covered)
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
	if !s.cfg.Persistence.Enabled {
		return
	}
	s.snapshotEvery.Store(int64(s.snapshotInterval))
	s.snapshotReset = make(chan struct{}, 1)
	s.wg.Add(1)
	go func() {
		defer s.wg.Done()
		// An interval of zero leaves the timer stopped until CONFIG SET save
		// sets one.
		timer := time.NewTimer(time.Hour)
		timer.Stop()
		arm := func() {
			if every := time.Duration(s.snapshotEvery.Load()); every > 0 {
				timer.Reset(every)
			}
		}
		arm()
		defer timer.Stop()
		for {
			select {
			case <-s.stopCh:
				return
			case <-s.snapshotReset:
				timer.Stop()
				arm()
			case <-timer.C:
				s.periodicSnapshot()
				arm()
			}
		}
	}()
}

// setSnapshotRules applies CONFIG SET save: snapshots run every <seconds> of
// the most frequent rule, and an empty value stops them.
func (s *Server) setSnapshotRules(value string) error {
	fields := strings.Fields(value)
	if len(fields)%2 != 0 {
		return command.ConfigSetError("save", "Invalid save parameters")
	}
	shortest := int64(0)
	for i := 0; i < len(fields); i += 2 {
		secs, err1 := strconv.ParseInt(fields[i], 10, 64)
		changes, err2 := strconv.ParseInt(fields[i+1], 10, 64)
		if err1 != nil || err2 != nil || secs < 1 || changes < 0 {
			return command.ConfigSetError("save", "Invalid save parameters")
		}
		if shortest == 0 || secs < shortest {
			shortest = secs
		}
	}
	if s.snapshotReset == nil {
		if shortest == 0 {
			return nil
		}
		return command.ConfigSetError("save", "snapshots need persistence.enabled")
	}
	s.snapshotEvery.Store(int64(time.Duration(shortest) * time.Second))
	select {
	case s.snapshotReset <- struct{}{}:
	default:
	}
	return nil
}

// snapshotRules is CONFIG GET save: the snapshot interval as a Redis rule.
func (s *Server) snapshotRules() string {
	if every := time.Duration(s.snapshotEvery.Load()); every > 0 {
		return strconv.FormatInt(int64(every/time.Second), 10) + " 1"
	}
	return ""
}

func (s *Server) stopSnapshots() {
	s.snapshotMu.Lock()
	s.snapshotClosing = true
	s.snapshotMu.Unlock()
	s.snapshotWG.Wait()
}
