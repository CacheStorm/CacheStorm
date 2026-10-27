package server

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/cachestorm/cachestorm/internal/logger"
	"github.com/cachestorm/cachestorm/internal/persistence"
	"github.com/cachestorm/cachestorm/internal/replication"
)

type replicaCheckpoint struct {
	Version   int
	State     replication.State
	AOFSize   int64
	AOFSHA256 string
}

type replicaCheckpointEnvelope struct {
	Data   json.RawMessage
	SHA256 string
}

type checkpointIO struct {
	create  func(string, string) (*os.File, error)
	sync    func(*os.File) error
	rename  func(string, string) error
	remove  func(string) error
	syncDir func(string) error
}

func defaultCheckpointIO() checkpointIO {
	return checkpointIO{os.CreateTemp, func(f *os.File) error { return f.Sync() }, os.Rename, os.Remove, persistence.SyncDataDirectory}
}

func consumeReplicaCheckpoint(path string, ops checkpointIO) ([]byte, error) {
	data, err := os.ReadFile(path)
	if err != nil && !os.IsNotExist(err) {
		return nil, err
	}
	if err == nil {
		if err := ops.remove(path); err != nil {
			return nil, err
		}
	}
	if err := ops.syncDir(filepath.Dir(path)); err != nil {
		return nil, err
	}
	return data, nil
}

func fingerprintAOF(path string) (int64, string, error) {
	f, err := os.Open(path)
	if err != nil {
		return 0, "", err
	}
	defer f.Close()
	info, err := f.Stat()
	if err != nil {
		return 0, "", err
	}
	if !info.Mode().IsRegular() {
		return 0, "", errors.New("checkpoint requires a regular AOF")
	}
	h := sha256.New()
	size, err := io.Copy(h, f)
	if err != nil {
		return 0, "", err
	}
	return size, hex.EncodeToString(h.Sum(nil)), nil
}

func (s *Server) loadReplicationState(dataDir string) error {
	data, err := consumeReplicaCheckpoint(filepath.Join(dataDir, replicationStateFile), defaultCheckpointIO())
	if err != nil {
		return fmt.Errorf("invalidate replication checkpoint: %w", err)
	}
	if len(data) == 0 || !s.cfg.Replication.IsReplica() {
		return nil
	}
	var envelope replicaCheckpointEnvelope
	if err := json.Unmarshal(data, &envelope); err != nil {
		return nil
	}
	sum := sha256.Sum256(envelope.Data)
	if hex.EncodeToString(sum[:]) != envelope.SHA256 {
		return nil
	}
	var checkpoint replicaCheckpoint
	if err := json.Unmarshal(envelope.Data, &checkpoint); err != nil || checkpoint.Version != 1 {
		return nil
	}
	st := checkpoint.State
	if len(st.ReplID) != 40 || st.Offset < 0 || !s.dbs.Valid(st.DB) || st.MasterHost != s.cfg.Replication.MasterHost || st.MasterPort != s.cfg.Replication.MasterPort {
		return nil
	}
	if _, err := hex.DecodeString(st.ReplID); err != nil {
		return nil
	}
	size, digest, err := fingerprintAOF(filepath.Join(dataDir, "appendonly.aof"))
	if err != nil || size != checkpoint.AOFSize || digest != checkpoint.AOFSHA256 {
		logger.Warn().Msg("replication checkpoint does not match the AOF; a full resync is required")
		return nil
	}
	s.repl.RestoreState(st)
	logger.Info().Str("replid", st.ReplID).Int64("offset", st.Offset).Msg("replication checkpoint restored")
	return nil
}

func writeReplicaCheckpoint(path string, data []byte, ops checkpointIO) error {
	dir := filepath.Dir(path)
	f, err := ops.create(dir, ".cachestorm-repl-*")
	if err != nil {
		return err
	}
	defer os.Remove(f.Name())
	defer f.Close()
	if _, err := f.Write(data); err != nil {
		return err
	}
	if err := ops.sync(f); err != nil {
		return err
	}
	if err := f.Close(); err != nil {
		return err
	}
	if err := ops.rename(f.Name(), path); err != nil {
		return err
	}
	if err := ops.syncDir(dir); err != nil {
		removeErr := ops.remove(path)
		return errors.Join(err, removeErr, ops.syncDir(dir))
	}
	return nil
}

func (s *Server) saveReplicationState() error {
	if s.aof == nil {
		return nil
	}
	if err := s.aof.Err(); err != nil {
		return err
	}
	st, ok := s.repl.SavedState()
	if !ok {
		return nil
	}
	dataDir := s.persistenceDirectory()
	size, digest, err := fingerprintAOF(filepath.Join(dataDir, "appendonly.aof"))
	if err != nil {
		return err
	}
	checkpoint := replicaCheckpoint{Version: 1, State: st, AOFSize: size, AOFSHA256: digest}
	payload, err := json.Marshal(checkpoint)
	if err != nil {
		return err
	}
	sum := sha256.Sum256(payload)
	data, err := json.Marshal(replicaCheckpointEnvelope{Data: payload, SHA256: hex.EncodeToString(sum[:])})
	if err != nil {
		return err
	}
	return writeReplicaCheckpoint(filepath.Join(dataDir, replicationStateFile), data, defaultCheckpointIO())
}
