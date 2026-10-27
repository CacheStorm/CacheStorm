package server

import (
	"errors"
	"time"

	"github.com/cachestorm/cachestorm/internal/command"
	"github.com/cachestorm/cachestorm/internal/logger"
	"github.com/cachestorm/cachestorm/internal/replication"
)

// shutdownReplicaWait is how long SHUTDOWN waits for replicas to catch up,
// as Redis' shutdown-timeout.
const shutdownReplicaWait = 10 * time.Second

//nolint:staticcheck // Redis' error text ends with a period
var errShutdownFailed = errors.New("ERR Errors trying to SHUTDOWN. Check logs.")

// ShutdownRequested is closed when a client asks the server to stop with
// SHUTDOWN. The process that owns the server then calls Stop.
func (s *Server) ShutdownRequested() <-chan struct{} {
	return s.shutdownCh
}

// requestShutdown is SHUTDOWN: it saves a snapshot when one is configured or
// asked for, waits for replicas to catch up unless NOW is given, and signals
// ShutdownRequested. A failed save refuses the shutdown unless FORCE.
func (s *Server) requestShutdown(o command.ShutdownOptions) error {
	wantSave := o.Save || (s.cfg.Persistence.Enabled && s.snapshotInterval > 0)
	if wantSave && !o.NoSave {
		var err error
		if !s.cfg.Persistence.Enabled {
			err = errors.New("persistence is not configured")
		} else {
			err = s.saveSnapshot(false)
		}
		if err != nil {
			logger.Error().Err(err).Msg("SHUTDOWN could not save a snapshot")
			if !o.Force {
				return errShutdownFailed
			}
		}
	}
	if !o.Now && s.repl != nil {
		if n := s.repl.ReplicaCount(); n > 0 {
			acked := s.repl.Wait(n, shutdownReplicaWait, nil)
			if acked < n {
				logger.Warn().Int("replicas", n).Int("acked", acked).Msg("SHUTDOWN proceeding before every replica caught up")
			}
		}
	}
	logger.Info().Msg("SHUTDOWN requested by a client")
	s.shutdownOnce.Do(func() { close(s.shutdownCh) })
	return nil
}

// waitAOF is WAITAOF: the local AOF is flushed and fsynced, so every write
// acknowledged before the call is on disk, and replicas are waited for as
// WAIT does. Replicas acknowledge the stream offset they applied; a replica
// with AOF fsyncs it per its own appendfsync policy.
//
//nolint:staticcheck // Redis' error texts end with a period
func (s *Server) waitAOF(numLocal, numReplicas int64, timeout time.Duration, done <-chan struct{}) (int64, int64, error) {
	if s.repl != nil && s.repl.Role() == replication.RoleReplica {
		return 0, 0, errors.New("ERR WAITAOF cannot be used with replica instances. Please also note that writes to replicas are just local and are not propagated.")
	}
	if numLocal > 0 && s.aof == nil {
		return 0, 0, errors.New("ERR WAITAOF cannot be used when numlocal is set but appendonly is disabled.")
	}
	var local int64
	if s.aof != nil {
		if err := s.aof.Flush(); err != nil {
			logger.Error().Err(err).Msg("WAITAOF fsync failed")
		} else {
			local = 1
		}
	}
	var replicas int64
	if s.repl != nil {
		// Inside EXEC (a negative timeout) nothing may block: the count is
		// read at once, as WAIT does there.
		want := int(numReplicas)
		if timeout < 0 {
			want, timeout = 0, 0
		}
		replicas = int64(s.repl.Wait(want, timeout, done))
	}
	return local, replicas, nil
}
