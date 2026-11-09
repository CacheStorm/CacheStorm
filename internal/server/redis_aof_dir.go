package server

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"github.com/cachestorm/cachestorm/internal/logger"
	"github.com/cachestorm/cachestorm/internal/persistence"
)

// Where Redis 7 keeps its multi-part AOF, by default.
const (
	redisAOFDir      = "appendonlydir"
	redisAOFFilename = "appendonly.aof"
)

// loadAOFPreamble loads the RDB a Redis 5 or 6 appendonly.aof begins with
// under aof-use-rdb-preamble yes and returns its length, where the file's
// commands start; it returns 0 for a file without one. The file is copied
// to appendonly.aof.redis first, as the caller rewrites it in CacheStorm's
// format.
func (s *Server) loadAOFPreamble(path string) (int64, error) {
	rdb, err := persistence.IsRDBFile(path)
	if err != nil || !rdb {
		return 0, err
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return 0, err
	}
	backup := path + ".redis"
	if _, err := os.Stat(backup); errors.Is(err, os.ErrNotExist) {
		if err := persistence.WriteFileAtomic(backup, data); err != nil {
			return 0, fmt.Errorf("keep a copy of the Redis AOF: %w", err)
		}
	} else if err != nil {
		return 0, err
	}
	reader := persistence.NewRDBReaderForDatabases(s.dbs)
	n, err := reader.LoadPreamble(data)
	if err != nil {
		return 0, fmt.Errorf("the RDB preamble of AOF %s is corrupt: %w", path, err)
	}
	if err := s.restoreRDBAux(reader); err != nil {
		return 0, err
	}
	logger.Info().Str("path", path).Str("copy", backup).
		Msg("loaded a Redis AOF with an RDB preamble; it is rewritten in CacheStorm's format and the original kept as the copy")
	return int64(n), nil
}

// loadRedisMultiPartAOF loads the AOF a Redis 7 server left in the data
// directory's appendonlydir: its base file (an RDB or commands), then each
// incremental file in the manifest's order. It reports false when there is
// no manifest. A torn command at the end of an incremental file is left out,
// as Redis does under aof-load-truncated, without changing the file.
func (s *Server) loadRedisMultiPartAOF(dataDir string) (bool, error) {
	dir := filepath.Join(dataDir, s.redisAOFDirName())
	aof, err := persistence.ReadRedisAOFManifest(dir, redisAOFFilename)
	if errors.Is(err, os.ErrNotExist) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	commands := 0
	replay := func(path string) error {
		reader := persistence.NewAOFReader()
		cmds, valid, err := reader.LoadValid(path)
		if err != nil {
			return fmt.Errorf("the Redis AOF %s is corrupt: %w", path, err)
		}
		if info, err := os.Stat(path); err == nil && valid < info.Size() {
			logger.Warn().Str("path", path).Int64("size", info.Size()).Int64("valid", valid).
				Msg("Redis AOF ends with an incomplete command, which is left out")
		}
		if _, err := s.replayAOF(cmds, true); err != nil {
			return fmt.Errorf("replay the Redis AOF %s: %w", path, err)
		}
		commands += len(cmds)
		return nil
	}
	if aof.Base != "" {
		rdb, err := persistence.IsRDBFile(aof.Base)
		if err != nil {
			return false, err
		}
		if rdb {
			if err := s.loadRDBFile(aof.Base); err != nil {
				return false, err
			}
		} else if err := replay(aof.Base); err != nil {
			return false, err
		}
	}
	for _, path := range aof.Incr {
		if err := replay(path); err != nil {
			return false, err
		}
	}
	logger.Info().Str("dir", dir).Int("incremental_files", len(aof.Incr)).Int("commands", commands).
		Msg("loaded the Redis 7 multi-part AOF; CacheStorm keeps its own appendonly.aof from now on and leaves the Redis files unchanged")
	return true, nil
}

// redisAOFDirName is CONFIG GET appenddirname.
func (s *Server) redisAOFDirName() string {
	if s.cfg.Persistence.RedisAOFDir != "" {
		return s.cfg.Persistence.RedisAOFDir
	}
	return redisAOFDir
}
