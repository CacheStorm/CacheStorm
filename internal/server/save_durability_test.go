package server

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/cachestorm/cachestorm/internal/command"
	"github.com/cachestorm/cachestorm/internal/config"
	"github.com/cachestorm/cachestorm/internal/resp"
)

// newSaveServer builds a real Server with AOF persistence in a temp dir using the
// given sync policy, so commands flow through the real router postExecute ->
// AOFWriter.Append path.
func newSaveServer(t *testing.T, syncPolicy string) (*Server, string) {
	t.Helper()

	dir := t.TempDir()
	cfg := &config.Config{}
	cfg.Persistence.Enabled = true
	cfg.Persistence.AOF = true
	cfg.Persistence.DataDir = dir
	cfg.Persistence.AOFSync = syncPolicy

	s, err := newOwnedTestServer(t, cfg)
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	if s.aof == nil {
		t.Fatal("AOF writer not configured; harness cannot exercise SAVE")
	}
	if err := s.aof.Start(); err != nil {
		t.Fatalf("start AOF writer: %v", err)
	}
	t.Cleanup(func() { stopReplServer(s) })
	return s, filepath.Join(dir, "appendonly.aof")
}

func saveExec(t *testing.T, s *Server, args ...string) string {
	t.Helper()
	buf := &bytes.Buffer{}
	raw := make([][]byte, len(args)-1)
	for i, a := range args[1:] {
		raw[i] = []byte(a)
	}
	ctx := command.NewContext(args[0], raw, s.store, resp.NewWriter(buf))
	if err := s.router.Execute(ctx); err != nil {
		t.Fatalf("execute %v: %v", args, err)
	}
	return buf.String()
}

func aofOnDisk(t *testing.T, path string) string {
	t.Helper()
	b, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return ""
		}
		t.Fatalf("read AOF: %v", err)
	}
	return string(b)
}

// TestSaveForcesDurability pins the contract of SAVE: it is the explicit
// "persist now" command, so when it returns +OK the data must be on disk.
//
// cmdSAVE only stamped lastSaveTime and replied +OK. With aof_sync "no" the AOF
// writer never flushes on append (AOFWriter.Append syncs only for AOFAlways) and
// no background syncLoop runs (AOFWriter.Start spawns it only for
// AOFEverySecond), so an acknowledged SET sat in the bufio buffer and SAVE
// returned success without ever moving a byte to disk. A client that issues SAVE
// as a durability barrier got a false guarantee and lost the data on crash.
func TestSaveForcesDurability(t *testing.T) {
	// Control: with aof_sync "always" the writer fsyncs on every append, so the
	// value is already on disk before SAVE. Passes before and after the fix, so a
	// broken harness cannot masquerade as the defect.
	ctl, ctlPath := newSaveServer(t, "always")
	saveExec(t, ctl, "SET", "ctlkey", "v")
	if !strings.Contains(aofOnDisk(t, ctlPath), "ctlkey") {
		t.Fatalf("control failed: with aof_sync=always the value is not on disk even before "+
			"SAVE (AOF=%q); the harness is not reading the real AOF file", aofOnDisk(t, ctlPath))
	}
	saveExec(t, ctl, "SAVE")
	if !strings.Contains(aofOnDisk(t, ctlPath), "ctlkey") {
		t.Fatal("control failed: SAVE lost an already-durable value")
	}

	// The contract: SAVE returning +OK must mean the data is durable, even when
	// the sync policy would otherwise leave it buffered.
	s, path := newSaveServer(t, "no")
	saveExec(t, s, "SET", "durablekey", "v")

	if got := saveExec(t, s, "SAVE"); got != "+OK\r\n" {
		t.Fatalf("SAVE replied %q, want +OK", strings.TrimSpace(got))
	}

	after := aofOnDisk(t, path)
	if !strings.Contains(after, "durablekey") {
		t.Fatalf("FAIL: SAVE returned +OK but the value never reached the AOF on disk "+
			"(file contains %q). cmdSAVE only stamps lastSaveTime and replies; with "+
			"aof_sync=no the writer never flushes on append, so SAVE gave the client a "+
			"false durability guarantee", after)
	}
}
