package server

import (
	"bytes"
	"errors"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/cachestorm/cachestorm/internal/command"
	"github.com/cachestorm/cachestorm/internal/logger"
	"github.com/cachestorm/cachestorm/internal/persistence"
	"github.com/cachestorm/cachestorm/internal/replication"
	"github.com/cachestorm/cachestorm/internal/resp"
)

func (s *Server) snapshotPath() string {
	dir := s.persistenceDirectory()
	return filepath.Join(dir, "dump.rdb")
}

func (s *Server) configureSnapshots(restore bool) error {
	s.snapshotLastAt = time.Now()
	s.snapshotLastTime = s.snapshotLastAt.Unix()
	s.router.SetSnapshotHooks(s.saveSnapshot, s.lastSnapshotTime, s.snapshotInfo)
	if !s.cfg.Persistence.Enabled {
		return nil
	}
	var rules []saveRule
	if save := s.cfg.Persistence.Save; save != "" {
		var err error
		if rules, err = parseSaveRules(save); err != nil {
			return fmt.Errorf("invalid persistence.save: %q", save)
		}
	} else if interval := s.cfg.Persistence.SnapshotInterval; interval != "" {
		every, err := time.ParseDuration(interval)
		if err != nil || every < 0 {
			return fmt.Errorf("invalid persistence.snapshot_interval: %q", interval)
		}
		if every > 0 {
			rules = []saveRule{{every: every, changes: 1}}
		}
	}
	s.saveRules.Store(&rules)
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
			s.snapshotLastAt = s.snapshotStarted
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
	var aux map[string]string
	if searchCmds := s.SearchCommands(); len(searchCmds) > 0 {
		aux = map[string]string{snapshotSearchAux: string(persistence.EncodeCommands(searchCmds))}
	}
	// The snapshot is in Redis' RDB format, so Redis and its tools load it.
	// The checksum is computed after the encoder returns, so a background
	// save holds the write gate only while the dataset is encoded.
	db0 := s.dbs.DB(0)
	encoded, _, err := persistence.EncodeRedisRDB(replication.DBSource{DBs: s.dbs}, persistence.RedisRDBInfo{
		Functions:  persistence.FunctionCodes(functions),
		Extensions: true,
		Aux:        aux,
		IdleTimes:  db0.UsesLRU(),
		LFU:        db0.UsesLFU(),
	})
	if err != nil {
		finish(err)
		return fmt.Errorf("ERR snapshot failed: %w", err)
	}
	data := bytes.NewBuffer(encoded)
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

// saveRule is one Redis save rule: snapshot once at least changes writes
// happened and every has passed since the last save.
type saveRule struct {
	every   time.Duration
	changes int64
}

func parseSaveRules(value string) ([]saveRule, error) {
	fields := strings.Fields(value)
	if len(fields)%2 != 0 {
		return nil, errors.New("invalid save parameters")
	}
	rules := make([]saveRule, 0, len(fields)/2)
	for i := 0; i < len(fields); i += 2 {
		secs, err1 := strconv.ParseInt(fields[i], 10, 64)
		changes, err2 := strconv.ParseInt(fields[i+1], 10, 64)
		if err1 != nil || err2 != nil || secs < 1 || secs > math.MaxInt64/int64(time.Second) || changes < 0 {
			return nil, errors.New("invalid save parameters")
		}
		rules = append(rules, saveRule{every: time.Duration(secs) * time.Second, changes: changes})
	}
	return rules, nil
}

func (s *Server) currentSaveRules() []saveRule {
	if rules := s.saveRules.Load(); rules != nil {
		return *rules
	}
	return nil
}

// saveRetryDelay is how long a failed background save blocks the next one
// the save rules start, as Redis' CONFIG_BGSAVE_RETRY_DELAY.
const saveRetryDelay = 5 * time.Second

// saveDue reports whether a save rule asks for a snapshot now, after dirty
// writes, as Redis' serverCron decides.
func (s *Server) saveDue(now time.Time, dirty int64) bool {
	rules := s.currentSaveRules()
	if len(rules) == 0 {
		return false
	}
	s.snapshotMu.Lock()
	last, started, failed := s.snapshotLastAt, s.snapshotStarted, s.snapshotLastError != nil
	s.snapshotMu.Unlock()
	if failed && now.Sub(started) < saveRetryDelay {
		return false
	}
	for _, r := range rules {
		if dirty >= r.changes && now.Sub(last) >= r.every {
			return true
		}
	}
	return false
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
	s.snapshotReset = make(chan struct{}, 1)
	s.wg.Add(1)
	go func() {
		defer s.wg.Done()
		// The rules are checked ten times a second, as Redis' serverCron
		// does at its default hz, or at each rule's own pace when that is
		// finer. Without rules the timer stays stopped until CONFIG SET save
		// sets some.
		timer := time.NewTimer(time.Hour)
		timer.Stop()
		arm := func() {
			rules := s.currentSaveRules()
			if len(rules) == 0 {
				return
			}
			tick := 100 * time.Millisecond
			for _, r := range rules {
				tick = min(tick, max(r.every/2, time.Millisecond))
			}
			timer.Reset(tick)
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
			case now := <-timer.C:
				if s.saveDue(now, command.Dirty()) {
					s.periodicSnapshot()
				}
				arm()
			}
		}
	}()
}

// setSnapshotRules applies CONFIG SET save.
func (s *Server) setSnapshotRules(value string) error {
	rules, err := parseSaveRules(value)
	if err != nil {
		return command.ConfigSetError("save", "Invalid save parameters")
	}
	if s.snapshotReset == nil {
		if len(rules) == 0 {
			return nil
		}
		return command.ConfigSetError("save", "snapshots need persistence.enabled")
	}
	s.saveRules.Store(&rules)
	select {
	case s.snapshotReset <- struct{}{}:
	default:
	}
	return nil
}

// snapshotRules is CONFIG GET save.
func (s *Server) snapshotRules() string {
	var b strings.Builder
	for _, r := range s.currentSaveRules() {
		if b.Len() > 0 {
			b.WriteByte(' ')
		}
		fmt.Fprintf(&b, "%d %d", max(int64(r.every/time.Second), 1), r.changes)
	}
	return b.String()
}

func (s *Server) stopSnapshots() {
	s.snapshotMu.Lock()
	s.snapshotClosing = true
	s.snapshotMu.Unlock()
	s.snapshotWG.Wait()
}
