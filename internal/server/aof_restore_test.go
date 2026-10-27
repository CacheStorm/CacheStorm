package server

import (
	"os"
	"path/filepath"
	"strconv"
	"testing"
	"time"

	"github.com/cachestorm/cachestorm/internal/persistence"
	"github.com/cachestorm/cachestorm/internal/store"
)

func TestAOFRestoreKeepsDeadlineAndHistory(t *testing.T) {
	for _, mode := range []string{"relative", "absolute", "persist", "exec", "script"} {
		t.Run(mode, func(t *testing.T) {
			cfg := aofTxConfig(t)
			s, run := aofTxStart(t, cfg)
			payload, err := persistence.EncodeDump(&store.HashValue{Fields: map[string][]byte{"before&": []byte("one=\x00")}})
			if err != nil {
				t.Fatal(err)
			}
			switch mode {
			case "absolute":
				run("RESTORE", "h", strconv.FormatInt(time.Now().Add(time.Minute).UnixMilli(), 10), string(payload), "ABSTTL")
			case "persist":
				run("RESTORE", "h", "150", string(payload))
				run("HSET", "h", "after", "two")
				run("PERSIST", "h")
			case "exec":
				run("MULTI")
				run("RESTORE", "h", "60000", string(payload))
				run("EXEC")
			case "script":
				run("EVAL", "return redis.call('RESTORE',KEYS[1],ARGV[1],ARGV[2])", "1", "h", "60000", string(payload))
			default:
				run("RESTORE", "h", "60000", string(payload))
			}
			deadline, ok := s.store.ExpiryDeadline("h")
			if !ok {
				t.Fatal("RESTORE failed")
			}
			stopReplServer(s)
			if mode == "persist" {
				time.Sleep(160 * time.Millisecond)
			} else {
				time.Sleep(5 * time.Millisecond)
			}
			restored, err := newOwnedTestServer(t, cfg)
			if err != nil {
				t.Fatal(err)
			}
			got, ok := restored.store.Get("h")
			if !ok {
				t.Fatal("restored key lost")
			}
			if got.ExpiresAt.Load() != deadline {
				t.Fatalf("deadline=%d want %d", got.ExpiresAt.Load(), deadline)
			}
			hash, ok := got.Value.(*store.HashValue)
			if !ok || string(hash.Fields["before&"]) != "one=\x00" {
				t.Fatal("binary field lost")
			}
			if mode == "persist" && string(hash.Fields["after"]) != "two" {
				t.Fatal("pre-persist mutation lost")
			}
		})
	}
}

func TestLegacyAOFPrivateRestoreMigrates(t *testing.T) {
	for _, versioned := range []bool{false, true} {
		t.Run(strconv.FormatBool(versioned), func(t *testing.T) {
			cfg := aofTxConfig(t)
			legacy := "CACHSTORM001" + string([]byte{byte(store.DataTypeString)}) + "0:legacy"
			data := encodeArgs("RESTORE", "k", "0", legacy)
			if versioned {
				data = append(encodeArgs("CACHESTORM.AOF", "2"), data...)
			}
			if err := os.WriteFile(filepath.Join(cfg.Persistence.DataDir, "appendonly.aof"), data, 0600); err != nil {
				t.Fatal(err)
			}
			s, err := newOwnedTestServer(t, cfg)
			if err != nil {
				t.Fatal(err)
			}
			value, ok := s.store.Get("k")
			if !ok || value.Value.String() != "legacy" {
				t.Fatal("private RESTORE lost")
			}
			reader := persistence.NewAOFReader()
			if _, _, err := reader.LoadValid(filepath.Join(cfg.Persistence.DataDir, "appendonly.aof")); err != nil || reader.Version != persistence.AOFVersion {
				t.Fatalf("migration err=%v version=%d", err, reader.Version)
			}
		})
	}
}

func TestDelayedReplicaRestoreKeepsPersistedHistory(t *testing.T) {
	master := startReplServer(t, nil)
	master.dbs.StopExpiry()
	mc := dialRepl(t, master)
	replica := startReplServer(t, nil)
	rc := replicate(t, replica, master)
	waitSynced(t, replica, master)
	payload, err := persistence.EncodeDump(&store.StringValue{Data: []byte("whole")})
	if err != nil {
		t.Fatal(err)
	}
	replica.LockWrites()
	result := mc.do("RESTORE", "late-restored", "100", string(payload))
	if result.Str != "OK" {
		replica.UnlockWrites()
		t.Fatalf("restore: %#v", result)
	}
	mc.do("APPEND", "late-restored", "-value")
	mc.do("PERSIST", "late-restored")
	time.Sleep(120 * time.Millisecond)
	replica.UnlockWrites()
	waitSynced(t, replica, master)
	if got := rc.do("GET", "late-restored"); string(got.Bulk) != "whole-value" {
		t.Fatalf("delayed restore: %#v", got)
	}
}
