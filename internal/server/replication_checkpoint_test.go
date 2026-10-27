package server

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/cachestorm/cachestorm/internal/config"
	"github.com/cachestorm/cachestorm/internal/persistence"
	"github.com/cachestorm/cachestorm/internal/replication"
)

func TestReplicaCheckpointRejectsChangedAOF(t *testing.T) {
	master := startReplServer(t, nil)
	mc := dialRepl(t, master)
	mc.do("SET", "checkpoint-key", "aaa")
	cfg := replTestConfig(t, func(c *config.Config) {
		c.Persistence.Enabled, c.Persistence.AOF = true, true
		c.Persistence.AOFSync = "everysec"
		c.Persistence.DataDir = t.TempDir()
		c.Replication.Role = "replica"
		c.Replication.MasterHost, c.Replication.MasterPort = "127.0.0.1", master.testPort()
	})
	replica := startReplServerCfg(t, cfg)
	waitLinkUp(t, dialRepl(t, replica))
	waitSynced(t, replica, master)
	stopReplServer(replica)
	path := filepath.Join(cfg.Persistence.DataDir, "appendonly.aof")
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	changed := bytes.Replace(data, []byte("$3\r\naaa\r\n"), []byte("$3\r\nbbb\r\n"), 1)
	if bytes.Equal(data, changed) {
		t.Fatal("test did not change the AOF")
	}
	if err := os.WriteFile(path, changed, 0600); err != nil {
		t.Fatal(err)
	}
	restored, err := newOwnedTestServer(t, cfg)
	if err != nil {
		t.Fatal(err)
	}
	masterID, _ := master.repl.ReplID()
	id, _ := restored.repl.ReplID()
	if id == masterID {
		t.Fatal("checkpoint for different AOF contents was accepted for partial resynchronization")
	}
	if err := restored.Start(context.Background()); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { stopReplServer(restored) })
	rc := dialRepl(t, restored)
	waitLinkUp(t, rc)
	waitSynced(t, restored, master)
	if got := rc.do("GET", "checkpoint-key"); string(got.Bulk) != "aaa" {
		t.Fatalf("full sync did not repair changed data: %#v", got)
	}
	full, partial, _ := syncStats(master)
	if full != 2 || partial != 0 {
		t.Fatalf("unexpected resync counts: full %d partial %d", full, partial)
	}
}

func encodeCheckpointTest(t *testing.T, cp replicaCheckpoint) []byte {
	t.Helper()
	payload, err := json.Marshal(cp)
	if err != nil {
		t.Fatal(err)
	}
	sum := sha256.Sum256(payload)
	data, err := json.Marshal(replicaCheckpointEnvelope{Data: payload, SHA256: hex.EncodeToString(sum[:])})
	if err != nil {
		t.Fatal(err)
	}
	return data
}

func TestReplicaCheckpointValidation(t *testing.T) {
	for _, mode := range []string{"valid", "legacy", "metadata-corruption", "changed-value", "truncated-aof", "appended-aof", "missing-aof", "bad-db", "negative-offset", "bad-id", "different-source", "version"} {
		t.Run(mode, func(t *testing.T) {
			cfg := aofTxConfig(t)
			cfg.Replication.Role = "replica"
			cfg.Replication.MasterHost, cfg.Replication.MasterPort = "127.0.0.1", 10001
			dir := cfg.Persistence.DataDir
			path := filepath.Join(dir, "appendonly.aof")
			aof := []byte("*3\r\n$3\r\nSET\r\n$1\r\nk\r\n$3\r\naaa\r\n")
			if err := os.WriteFile(path, aof, 0600); err != nil {
				t.Fatal(err)
			}
			size, digest, err := fingerprintAOF(path)
			if err != nil {
				t.Fatal(err)
			}
			cp := replicaCheckpoint{Version: 1, State: replication.State{ReplID: strings.Repeat("a", 40), Offset: 42, DB: 2, MasterHost: cfg.Replication.MasterHost, MasterPort: cfg.Replication.MasterPort}, AOFSize: size, AOFSHA256: digest}
			switch mode {
			case "bad-db":
				cp.State.DB = 1000
			case "negative-offset":
				cp.State.Offset = -1
			case "bad-id":
				cp.State.ReplID = strings.Repeat("z", 40)
			case "different-source":
				cp.State.MasterPort++
			case "version":
				cp.Version++
			}
			data := encodeCheckpointTest(t, cp)
			switch mode {
			case "legacy":
				data, err = json.Marshal(cp.State)
			case "metadata-corruption":
				data = bytes.Replace(data, []byte(`"Offset":42`), []byte(`"Offset":43`), 1)
			case "changed-value":
				err = os.WriteFile(path, bytes.Replace(aof, []byte("aaa"), []byte("bbb"), 1), 0600)
			case "truncated-aof":
				err = os.WriteFile(path, aof[:len(aof)-3], 0600)
			case "appended-aof":
				err = os.WriteFile(path, append(aof, aof...), 0600)
			case "missing-aof":
				err = os.Remove(path)
			}
			if err != nil {
				t.Fatal(err)
			}
			checkpointPath := filepath.Join(dir, replicationStateFile)
			if err := os.WriteFile(checkpointPath, data, 0600); err != nil {
				t.Fatal(err)
			}
			s, err := newOwnedTestServer(t, cfg)
			if err != nil {
				t.Fatal(err)
			}
			id, offset := s.repl.ReplID()
			if (id == cp.State.ReplID) != (mode == "valid") {
				t.Fatalf("unexpected checkpoint acceptance: %s %d", id, offset)
			}
			if mode == "valid" && offset != 42 {
				t.Fatalf("wrong restored offset: %d", offset)
			}
			if _, err := os.Stat(checkpointPath); !os.IsNotExist(err) {
				t.Fatalf("checkpoint not consumed: %v", err)
			}
		})
	}
}

func TestCheckpointConsumeRequiresDurableInvalidation(t *testing.T) {
	for _, mode := range []string{"remove", "directory", "missing-directory", "success"} {
		t.Run(mode, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), replicationStateFile)
			if mode != "missing-directory" {
				if err := os.WriteFile(path, []byte("checkpoint"), 0600); err != nil {
					t.Fatal(err)
				}
			}
			ops := defaultCheckpointIO()
			injected := errors.New("injected invalidation failure")
			if mode == "remove" {
				ops.remove = func(string) error { return injected }
			}
			if mode == "directory" || mode == "missing-directory" {
				ops.syncDir = func(string) error { return injected }
			}
			data, err := consumeReplicaCheckpoint(path, ops)
			if mode != "success" {
				if !errors.Is(err, injected) || data != nil {
					t.Fatalf("unsafe checkpoint admitted: %q %v", data, err)
				}
			} else if err != nil || string(data) != "checkpoint" {
				t.Fatalf("consume failed: %q %v", data, err)
			}
		})
	}
}

func TestCheckpointPublicationFailures(t *testing.T) {
	for _, phase := range []string{"create", "write", "sync", "close", "rename", "directory"} {
		t.Run(phase, func(t *testing.T) {
			dir := t.TempDir()
			path := filepath.Join(dir, replicationStateFile)
			ops := defaultCheckpointIO()
			injected := errors.New("injected checkpoint failure")
			switch phase {
			case "create":
				ops.create = func(string, string) (*os.File, error) { return nil, injected }
			case "write":
				ops.create = func(dir, pattern string) (*os.File, error) {
					f, err := os.CreateTemp(dir, pattern)
					if err != nil {
						return nil, err
					}
					name := f.Name()
					f.Close()
					return os.Open(name)
				}
			case "sync":
				ops.sync = func(*os.File) error { return injected }
			case "close":
				ops.sync = func(f *os.File) error { return f.Close() }
			case "rename":
				ops.rename = func(string, string) error { return injected }
			case "directory":
				calls := 0
				ops.syncDir = func(dir string) error {
					calls++
					if calls == 1 {
						return injected
					}
					return defaultCheckpointIO().syncDir(dir)
				}
			}
			if err := writeReplicaCheckpoint(path, []byte("checkpoint"), ops); err == nil {
				t.Fatal("publication succeeded despite injected failure")
			}
			if _, err := os.Stat(path); !os.IsNotExist(err) {
				t.Fatalf("checkpoint published after failure: %v", err)
			}
			files, err := filepath.Glob(filepath.Join(dir, ".cachestorm-repl-*"))
			if err != nil || len(files) != 0 {
				t.Fatalf("temporary files left behind: %v %v", files, err)
			}
		})
	}
}

func TestCheckpointInvalidationFailureStopsStartup(t *testing.T) {
	cfg := aofTxConfig(t)
	path := filepath.Join(cfg.Persistence.DataDir, replicationStateFile)
	if err := os.Mkdir(path, 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(path, "obstruction"), []byte("x"), 0600); err != nil {
		t.Fatal(err)
	}
	if _, err := newOwnedTestServer(t, cfg); err == nil || !strings.Contains(err.Error(), "invalidate replication checkpoint") {
		t.Fatalf("startup ignored checkpoint invalidation failure: %v", err)
	}
}

func TestReplicaShutdownFailureDoesNotPublishCheckpoint(t *testing.T) {
	if _, err := os.Stat("/dev/full"); err != nil {
		t.Skip("requires /dev/full")
	}
	master := startReplServer(t, nil)
	mc := dialRepl(t, master)
	mc.do("SET", "checkpoint-key", "aaa")
	cfg := replTestConfig(t, func(c *config.Config) {
		c.Persistence.Enabled, c.Persistence.AOF = true, true
		c.Persistence.DataDir = t.TempDir()
		c.Replication.Role = "replica"
		c.Replication.MasterHost, c.Replication.MasterPort = "127.0.0.1", master.testPort()
	})
	replica := startReplServerCfg(t, cfg)
	waitLinkUp(t, dialRepl(t, replica))
	waitSynced(t, replica, master)
	replica.repl.Stop()
	if err := replica.aof.StopWithError(); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink("/dev/full", filepath.Join(cfg.Persistence.DataDir, "shutdown.aof")); err != nil {
		t.Fatal(err)
	}
	w := persistence.NewAOFWriter(persistence.AOFConfig{Enabled: true, DataDir: cfg.Persistence.DataDir, Filename: "shutdown.aof", SyncPolicy: persistence.AOFNoSync})
	if err := w.Start(); err != nil {
		t.Fatal(err)
	}
	if err := w.Append("SET", [][]byte{[]byte("buffered"), []byte("v")}); err != nil {
		t.Fatal(err)
	}
	replica.aof = w
	if err := replica.Stop(context.Background()); err == nil {
		t.Fatal("server shutdown hid AOF failure")
	}
	if _, err := os.Stat(filepath.Join(cfg.Persistence.DataDir, replicationStateFile)); !os.IsNotExist(err) {
		t.Fatalf("checkpoint published despite unsynced AOF: %v", err)
	}
}

func TestWritableReplicaDoesNotCheckpointDivergedData(t *testing.T) {
	master := startReplServer(t, nil)
	mc := dialRepl(t, master)
	mc.do("SET", "upstream", "v")
	for _, mode := range []string{"command", "transaction", "script", "rest"} {
		t.Run(mode, func(t *testing.T) {
			cfg := replTestConfig(t, func(c *config.Config) {
				c.Persistence.Enabled, c.Persistence.AOF = true, true
				c.Persistence.DataDir = t.TempDir()
				c.Replication.Role, c.Replication.ReadOnly = "replica", false
				c.Replication.MasterHost, c.Replication.MasterPort = "127.0.0.1", master.testPort()
			})
			replica := startReplServerCfg(t, cfg)
			rc := dialRepl(t, replica)
			waitLinkUp(t, rc)
			waitSynced(t, replica, master)
			switch mode {
			case "command":
				rc.do("SET", "local-only", "v")
			case "transaction":
				rc.do("MULTI")
				rc.do("SET", "local-only", "v")
				rc.do("EXEC")
			case "script":
				rc.do("EVAL", "return redis.call('SET', 'local-only', 'v')", "0")
			case "rest":
				h := NewHTTPServer(replica.store, replica.router, &HTTPConfig{})
				h.srv, h.dbs = replica, replica.dbs
				t.Cleanup(h.cancel)
				out := httptest.NewRecorder()
				h.handleKeys(out, httptest.NewRequest(http.MethodPost, "/api/keys", strings.NewReader(`{"key":"local-only","value":"v"}`)))
				if out.Code != http.StatusOK {
					t.Fatalf("local REST write failed: %d %s", out.Code, out.Body.String())
				}
			}
			if !replica.store.Exists("local-only") {
				t.Fatal("test did not diverge replica data")
			}
			if _, ok := replica.repl.SavedState(); ok {
				t.Fatal("locally diverged dataset was admitted as an upstream checkpoint")
			}
			if mode == "command" {
				before, _, _ := syncStats(master)
				master.repl.DisconnectReplicas()
				eventually(t, 5*time.Second, "dirty replica fully resynchronized", func() bool {
					full, _, _ := syncStats(master)
					_, eligible := replica.repl.SavedState()
					return full > before && !replica.store.Exists("local-only") && eligible
				})
				waitSynced(t, replica, master)
				if _, ok := replica.repl.SavedState(); !ok {
					t.Fatal("full resync did not restore checkpoint eligibility")
				}
				rc.do("SET", "local-only", "v")
			}
			stopReplServer(replica)
			if _, err := os.Stat(filepath.Join(cfg.Persistence.DataDir, replicationStateFile)); !os.IsNotExist(err) {
				t.Fatalf("checkpoint saved for local divergence: %v", err)
			}
			replica = startReplServerCfg(t, cfg)
			rc = dialRepl(t, replica)
			waitLinkUp(t, rc)
			waitSynced(t, replica, master)
			if got := rc.do("GET", "local-only"); !got.IsNull {
				t.Fatalf("diverged data survived full resync: %#v", got)
			}
		})
	}
}
