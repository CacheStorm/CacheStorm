package persistence

import (
	"bytes"
	"errors"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestRewriteFailureKeepsDurableWriter(t *testing.T) {
	injected := errors.New("injected replacement failure")
	for _, phase := range []string{"create", "write", "sync", "old-sync", "rename", "directory", "copy-directory"} {
		t.Run(phase, func(t *testing.T) {
			dir := t.TempDir()
			path := filepath.Join(dir, "appendonly.aof")
			w := NewAOFWriter(AOFConfig{Enabled: true, DataDir: dir, Filename: "appendonly.aof", SyncPolicy: AOFAlways})
			if err := w.Start(); err != nil {
				t.Fatal(err)
			}
			t.Cleanup(w.Stop)
			if err := w.AppendDB(3, "SET", [][]byte{[]byte("old"), []byte("v")}); err != nil {
				t.Fatal(err)
			}
			before, err := os.ReadFile(path)
			if err != nil {
				t.Fatal(err)
			}
			ops := defaultAOFReplaceOps()
			switch phase {
			case "create":
				ops.create = func(string) (*os.File, error) { return nil, injected }
			case "write":
				ops.write = func(dst io.Writer, _ MultiDBSource) error { _, _ = dst.Write([]byte("partial")); return injected }
			case "sync":
				ops.sync = func(*os.File) error { return injected }
			case "old-sync":
				ops.sync = func(f *os.File) error {
					if f == w.file {
						return injected
					}
					return f.Sync()
				}
			case "rename":
				ops.rename = func(string, string) error { return injected }
			case "directory", "copy-directory":
				calls := 0
				ops.syncDir = func(dir string) error {
					calls++
					if calls == 1 {
						return injected
					}
					return SyncDataDirectory(dir)
				}
				if phase == "copy-directory" {
					ops.link = func(string, string) error { return errors.New("links unsupported") }
				}
			}
			w.replaceOps = ops
			if err := w.Rewrite(multiDBSource{dbs: []map[string]interface{}{{"snapshot": "value"}}}); !errors.Is(err, injected) {
				t.Fatalf("rewrite error: %v", err)
			}
			after, err := os.ReadFile(path)
			if err != nil || !bytes.Equal(before, after) {
				t.Fatalf("previous AOF changed: %v", err)
			}
			if !w.running.Load() || w.curDB != 3 {
				t.Fatal("writer state changed on failed rewrite")
			}
			if err := w.AppendDB(0, "SET", [][]byte{[]byte("after"), []byte("durable")}); err != nil {
				t.Fatal(err)
			}
			commands, err := NewAOFReader().Load(path)
			if err != nil {
				t.Fatal(err)
			}
			db, found := 0, false
			for _, c := range commands {
				if c.Name == "SELECT" {
					if string(c.Args[0]) == "3" {
						db = 3
					} else {
						db = 0
					}
				}
				if c.Name == "SET" && string(c.Args[0]) == "after" {
					found = true
					if db != 0 {
						t.Fatal("append inherited stale database")
					}
				}
			}
			if !found {
				t.Fatal("append after failed rewrite disappeared")
			}
			files, err := filepath.Glob(filepath.Join(dir, ".aof-rewrite-*"))
			if err != nil || len(files) != 0 {
				t.Fatalf("temporary files remain: %v %v", files, err)
			}
		})
	}
}

func TestRewriteRollbackFailurePreservesBackupAndRejectsWrites(t *testing.T) {
	dir := t.TempDir()
	w := NewAOFWriter(AOFConfig{Enabled: true, DataDir: dir, Filename: "appendonly.aof", SyncPolicy: AOFAlways})
	if err := w.Start(); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(w.Stop)
	if err := w.Append("SET", [][]byte{[]byte("old"), []byte("v")}); err != nil {
		t.Fatal(err)
	}
	before, err := os.ReadFile(filepath.Join(dir, "appendonly.aof"))
	if err != nil {
		t.Fatal(err)
	}
	ops := defaultAOFReplaceOps()
	ops.syncDir = func(string) error { return errors.New("injected directory sync failure") }
	calls := 0
	ops.rename = func(from, to string) error {
		calls++
		if calls > 1 {
			return errors.New("injected rollback failure")
		}
		return os.Rename(from, to)
	}
	w.replaceOps = ops
	if err := w.Rewrite(multiDBSource{dbs: []map[string]interface{}{{"snapshot": "value"}}}); err == nil {
		t.Fatal("rewrite succeeded despite rollback failure")
	}
	if err := w.Append("SET", [][]byte{[]byte("after"), []byte("v")}); err == nil {
		t.Fatal("append accepted after failed rollback")
	}
	if err := w.Flush(); err == nil {
		t.Fatal("flush accepted after failed rollback")
	}
	files, err := filepath.Glob(filepath.Join(dir, ".aof-rewrite-*"))
	if err != nil || len(files) != 1 {
		t.Fatalf("missing recovery backup: %v %v", files, err)
	}
	backup, err := os.ReadFile(files[0])
	if err != nil || !bytes.Equal(before, backup) {
		t.Fatalf("backup lost previous AOF: %v", err)
	}
}

func TestRewriteSerializesConcurrentAppend(t *testing.T) {
	dir := t.TempDir()
	w := NewAOFWriter(AOFConfig{Enabled: true, DataDir: dir, Filename: "appendonly.aof", SyncPolicy: AOFAlways})
	if err := w.Start(); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(w.Stop)
	entered, release := make(chan struct{}), make(chan struct{})
	ops := defaultAOFReplaceOps()
	ops.write = func(dst io.Writer, src MultiDBSource) error {
		close(entered)
		<-release
		return WriteSnapshot(dst, src)
	}
	w.replaceOps = ops
	rewritten, appended := make(chan error, 1), make(chan error, 1)
	go func() { rewritten <- w.Rewrite(multiDBSource{dbs: []map[string]interface{}{{"snapshot": "value"}}}) }()
	<-entered
	if !w.running.Load() {
		close(release)
		t.Fatal("writer stopped during rewrite")
	}
	go func() { appended <- w.Append("SET", [][]byte{[]byte("after"), []byte("v")}) }()
	close(release)
	if err := <-rewritten; err != nil {
		t.Fatal(err)
	}
	if err := <-appended; err != nil {
		t.Fatal(err)
	}
	commands, err := NewAOFReader().Load(filepath.Join(dir, "appendonly.aof"))
	if err != nil {
		t.Fatal(err)
	}
	found := map[string]bool{}
	for _, c := range commands {
		if c.Name == "SET" {
			found[string(c.Args[0])] = true
		}
	}
	if !found["snapshot"] || !found["after"] {
		t.Fatalf("missing persisted keys: %v", found)
	}
}

func TestRewriteRollbackSyncFailureReportsRecoveryPath(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "appendonly.aof")
	w := NewAOFWriter(AOFConfig{Enabled: true, DataDir: dir, Filename: "appendonly.aof", SyncPolicy: AOFAlways})
	if err := w.Start(); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(w.Stop)
	if err := w.Append("SET", [][]byte{[]byte("old"), []byte("v")}); err != nil {
		t.Fatal(err)
	}
	before, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	ops := defaultAOFReplaceOps()
	ops.syncDir = func(string) error { return errors.New("injected directory sync failure") }
	w.replaceOps = ops
	err = w.Rewrite(multiDBSource{dbs: []map[string]interface{}{{"snapshot": "value"}}})
	if err == nil || !strings.Contains(err.Error(), path) {
		t.Fatalf("missing recovery path: %v", err)
	}
	after, err := os.ReadFile(path)
	if err != nil || !bytes.Equal(before, after) {
		t.Fatalf("previous AOF lost: %v", err)
	}
	if err := w.Append("SET", [][]byte{[]byte("after"), []byte("v")}); err == nil {
		t.Fatal("append accepted after rollback sync failure")
	}
}

func TestRewriteDoesNotLoseWriterOnReopenFailure(t *testing.T) {
	dir := t.TempDir()
	w := NewAOFWriter(AOFConfig{Enabled: true, DataDir: dir, Filename: "appendonly.aof", SyncPolicy: AOFAlways})
	opens := 0
	w.openFile = func(path string, flags int, mode os.FileMode) (*os.File, error) {
		opens++
		if opens > 1 {
			return nil, errors.New("injected reopen failure")
		}
		return os.OpenFile(path, flags, mode)
	}
	if err := w.Start(); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(w.Stop)
	if err := w.AppendDB(3, "SET", [][]byte{[]byte("old"), []byte("v")}); err != nil {
		t.Fatal(err)
	}
	_ = w.Rewrite(multiDBSource{dbs: []map[string]interface{}{{"snapshot": "value"}}})
	if !w.running.Load() {
		t.Fatal("rewrite reopen failure disabled the previously healthy writer")
	}
	if err := w.AppendDB(0, "SET", [][]byte{[]byte("after"), []byte("durable")}); err != nil {
		t.Fatal(err)
	}
	commands, err := NewAOFReader().Load(filepath.Join(dir, "appendonly.aof"))
	if err != nil {
		t.Fatal(err)
	}
	found := false
	for _, c := range commands {
		if c.Name == "SET" && len(c.Args) == 2 && string(c.Args[0]) == "after" {
			found = true
		}
	}
	if !found {
		t.Fatal("an acknowledged append after rewrite was silently dropped")
	}
}
