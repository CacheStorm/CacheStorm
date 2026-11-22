package server

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/cachestorm/cachestorm/internal/config"
)

func waitBackupState(t *testing.T, c *replClient, want string) {
	t.Helper()
	deadline := time.Now().Add(5 * time.Second)
	for {
		got := c.str("BACKUP", "STATUS")
		if strings.HasPrefix(got, "[state "+want+" ") {
			return
		}
		if time.Now().After(deadline) {
			t.Fatalf("BACKUP STATUS = %s, want state %s", got, want)
		}
		time.Sleep(10 * time.Millisecond)
	}
}

// Redis 8.10's BACKUP: a snapshot and the writes after it, kept as a
// multi-part AOF that a server loads as its appendonlydir.
func TestBackup(t *testing.T) {
	dir := t.TempDir()
	cfg := aofRewriteConfig(t, dir, func(c *config.Config) { c.Persistence.AOF = false })
	s := startReplServerCfg(t, cfg)
	c := dialRepl(t, s)
	expect := func(want string, args ...string) {
		t.Helper()
		if got := c.str(args...); got != want {
			t.Fatalf("%v = %q, want %q", args, got, want)
		}
	}
	expect("[backupdirname backupdir]", "CONFIG", "GET", "backupdirname")
	expect("[backup-sealed-ttl 0]", "CONFIG", "GET", "backup-sealed-ttl")
	expect("[state idle error  start_time 0 end_time 0]", "BACKUP", "STATUS")
	expect("ERR:ERR No backup ready to seal (must be in the incrementing state)", "BACKUP", "SEAL")
	expect("ERR:ERR No backup in progress", "BACKUP", "ABORT")
	expect("[]", "BACKUP", "LIST")
	if got := c.str("INFO", "persistence"); !strings.Contains(got, "backup_in_progress:0") {
		t.Fatalf("INFO persistence: %s", got)
	}

	c.do("SET", "a", "1")
	c.do("HSET", "h", "f", "v")
	expect("OK", "BACKUP", "START")
	waitBackupState(t, c, "incrementing")
	expect("ERR:ERR A backup is already in progress, ABORT it first", "BACKUP", "START")
	if got := c.str("INFO", "persistence"); !strings.Contains(got, "backup_in_progress:1") {
		t.Fatalf("INFO persistence: %s", got)
	}
	backupDir := filepath.Join(dir, "backupdir")
	expect("["+filepath.Join(backupDir, "appendonly.aof.1.base.aof")+"]", "BACKUP", "LIST")
	c.do("SET", "b", "2")
	c.do("MULTI")
	c.do("SET", "c", "3")
	c.do("INCR", "a")
	c.do("EXEC")
	c.do("SELECT", "1")
	c.do("SET", "d", "4")
	c.do("SELECT", "0")
	c.do("DEL", "h")
	expect("OK", "BACKUP", "SEAL")
	if got := c.str("BACKUP", "STATUS"); !strings.HasPrefix(got, "[state sealed error  start_time ") || strings.HasSuffix(got, "end_time 0]") {
		t.Fatalf("BACKUP STATUS = %s", got)
	}
	expect("["+filepath.Join(backupDir, "appendonly.aof.1.base.aof")+" "+filepath.Join(backupDir, "appendonly.aof.1.incr.aof")+" "+filepath.Join(backupDir, "appendonly.aof.manifest")+"]", "BACKUP", "LIST")
	expect("ERR:ERR A sealed backup exists, CLEANUP it first", "BACKUP", "START")
	// Writes after the seal are not in the backup.
	c.do("SET", "late", "x")
	manifest, err := os.ReadFile(filepath.Join(backupDir, "appendonly.aof.manifest"))
	if err != nil || string(manifest) != "file appendonly.aof.1.base.aof seq 1 type b\nfile appendonly.aof.1.incr.aof seq 1 type i\n" {
		t.Fatalf("manifest %q %v", manifest, err)
	}

	// The backup loads as a server's appendonlydir.
	restoreDir := t.TempDir()
	if err := os.Mkdir(filepath.Join(restoreDir, "appendonlydir"), 0o755); err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"appendonly.aof.1.base.aof", "appendonly.aof.1.incr.aof", "appendonly.aof.manifest"} {
		data, err := os.ReadFile(filepath.Join(backupDir, name))
		if err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(restoreDir, "appendonlydir", name), data, 0o600); err != nil {
			t.Fatal(err)
		}
	}
	restored, err := newOwnedTestServer(t, aofRewriteConfig(t, restoreDir, nil))
	if err != nil {
		t.Fatal(err)
	}
	if err := restored.Start(context.Background()); err != nil {
		t.Fatal(err)
	}
	rc := dialRepl(t, restored)
	for _, kv := range [][2]string{{"a", "2"}, {"b", "2"}, {"c", "3"}, {"h", "0"}, {"late", "0"}} {
		var got string
		if kv[0] == "h" || kv[0] == "late" {
			got = rc.str("EXISTS", kv[0])
		} else {
			got = rc.str("GET", kv[0])
		}
		if got != kv[1] {
			t.Fatalf("restored %s = %s, want %s", kv[0], got, kv[1])
		}
	}
	rc.do("SELECT", "1")
	if got := rc.str("GET", "d"); got != "4" {
		t.Fatalf("restored db 1 d = %s", got)
	}

	expect("OK", "BACKUP", "CLEANUP")
	expect("[state idle error  start_time 0 end_time 0]", "BACKUP", "STATUS")
	if entries, _ := os.ReadDir(backupDir); len(entries) != 0 {
		t.Fatalf("backup dir after CLEANUP: %v", entries)
	}

	// ABORT leaves a failed backup and no files.
	expect("OK", "BACKUP", "START")
	waitBackupState(t, c, "incrementing")
	expect("OK", "BACKUP", "ABORT")
	if got := c.str("BACKUP", "STATUS"); !strings.HasPrefix(got, "[state failed error aborted by user start_time ") || !strings.HasSuffix(got, "end_time 0]") {
		t.Fatalf("BACKUP STATUS = %s", got)
	}
	if entries, _ := os.ReadDir(backupDir); len(entries) != 0 {
		t.Fatalf("backup dir after ABORT: %v", entries)
	}
	expect("[]", "BACKUP", "LIST")
	expect("OK", "BACKUP", "START")
	waitBackupState(t, c, "incrementing")
	expect("ERR:ERR Backup is in progress", "BACKUP", "CLEANUP")
	expect("OK", "BACKUP", "SEAL")

	// backup-sealed-ttl removes a sealed backup. The setting is process
	// wide, so it goes back for the tests that follow.
	t.Cleanup(func() { c.do("CONFIG", "SET", "backup-sealed-ttl", "0") })
	c.do("CONFIG", "SET", "backup-sealed-ttl", "1")
	waitBackupState(t, c, "idle")
	if entries, _ := os.ReadDir(backupDir); len(entries) != 0 {
		t.Fatalf("backup dir after the TTL: %v", entries)
	}

	// A directory with files in it is refused.
	if err := os.WriteFile(filepath.Join(backupDir, "stray"), nil, 0o600); err != nil {
		t.Fatal(err)
	}
	expect("ERR:ERR Backup directory backupdir is not empty, CLEANUP it first", "BACKUP", "START")
	expect("OK", "BACKUP", "CLEANUP")
	expect("OK", "BACKUP", "START")
}
