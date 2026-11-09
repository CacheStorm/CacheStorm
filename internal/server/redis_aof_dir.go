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

// loadRedisMultiPartAOF loads the AOF a Redis 7 server left in the data
// directory's appendonlydir: its base file (an RDB or commands), then each
// incremental file in the manifest's order. It reports false when there is
// no manifest. A torn command at the end of an incremental file is left out,
// as Redis does under aof-load-truncated, without changing the file.
func (s *Server) loadRedisMultiPartAOF(dataDir string) (bool, error) {
	dir := filepath.Join(dataDir, redisAOFDir)
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
