package server

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/cachestorm/cachestorm/internal/command"
	"github.com/cachestorm/cachestorm/internal/config"
	"github.com/cachestorm/cachestorm/internal/persistence"
	"github.com/cachestorm/cachestorm/internal/resp"
)

// newAOFServer builds a real Server with AOF persistence enabled in a temp dir,
// so router.postExecute -> AOFWriter.Append is the production write path.
func newAOFServer(t *testing.T) (*Server, string) {
	t.Helper()

	dir := t.TempDir()
	cfg := &config.Config{}
	cfg.Persistence.Enabled = true
	cfg.Persistence.AOF = true
	cfg.Persistence.DataDir = dir
	cfg.Persistence.AOFSync = "always"

	s, err := newOwnedTestServer(t, cfg)
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	if s.aof == nil {
		t.Fatal("AOF writer was not configured; test cannot exercise the write gate")
	}
	if err := s.aof.Start(); err != nil {
		t.Fatalf("start AOF writer: %v", err)
	}
	t.Cleanup(func() { stopReplServer(s) })
	return s, filepath.Join(dir, "appendonly.aof")
}

func aofExec(t *testing.T, s *Server, args ...string) {
	t.Helper()
	buf := &bytes.Buffer{}
	ctx := command.NewContext(args[0], byteArgs(args[1:]), s.store, resp.NewWriter(buf))
	if err := s.router.Execute(ctx); err != nil {
		t.Fatalf("execute %v: %v", args, err)
	}
}

func byteArgs(in []string) [][]byte {
	out := make([][]byte, len(in))
	for i, a := range in {
		out[i] = []byte(a)
	}
	return out
}

// aofCommandNames returns the command names actually persisted to the AOF, read
// back through the production AOFReader.
func aofCommandNames(t *testing.T, path string) []string {
	t.Helper()
	if _, err := os.Stat(path); err != nil {
		t.Fatalf("AOF file missing: %v", err)
	}
	cmds, err := persistence.NewAOFReader().Load(path)
	if err != nil {
		t.Fatalf("load AOF: %v", err)
	}
	names := make([]string, 0, len(cmds))
	for _, c := range cmds {
		names = append(names, strings.ToUpper(c.Name))
	}
	return names
}

// TestMutatingCommandsReachTheAOF pins the AOF durability contract: every
// command that mutates stored data must be appended to the AOF, or replaying it
// after a restart restores a state the client already deleted or changed.
//
// server.New gates postExecute on the hand-maintained writeCommands map, which
// is missing a large set of registered mutating commands. GETDEL deletes the key
// it returns, so when it is not appended, a restart replays the earlier SET and
// resurrects a key the client deleted.
func TestMutatingCommandsReachTheAOF(t *testing.T) {
	s, path := newAOFServer(t)

	aofExec(t, s, "SET", "k", "v")
	// Control: SET is classified as a write, so it IS persisted. This passes
	// before and after the fix, so a broken harness cannot look like the defect.
	aofExec(t, s, "GETDEL", "k")
	if err := s.aof.Flush(); err != nil {
		t.Fatalf("flush AOF: %v", err)
	}

	names := aofCommandNames(t, path)
	t.Logf("AOF contains: %v", names)

	has := func(want string) bool {
		for _, n := range names {
			if n == want {
				return true
			}
		}
		return false
	}

	if !has("SET") {
		t.Fatalf("control failed: SET was not persisted; the harness is not exercising "+
			"the AOF write gate at all (AOF contains %v)", names)
	}

	if !has("GETDEL") {
		t.Fatalf("FAIL: GETDEL deletes the key it returns, but it was never appended to the "+
			"AOF (AOF contains %v). server.New gates postExecute on the hand-maintained "+
			"writeCommands map, which omits GETDEL, so a restart replays the earlier SET and "+
			"resurrects a key the client already deleted", names)
	}
}
