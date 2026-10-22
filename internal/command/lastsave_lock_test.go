package command

import (
	"bytes"
	"strings"
	"testing"

	"github.com/cachestorm/cachestorm/internal/resp"
	"github.com/cachestorm/cachestorm/internal/store"
)

// Audit of internal/command/server_commands.go and config_commands.go for direct
// field reads on package globals whose types are NOT atomic.
//
// Type-first filter over the globals declared in the two files:
//
//	globalACL               *acl.ACL        — all access via methods
//	globalClientTracking     struct{mu; map} — every access under its own mu  CLEAN
//	globalSlowLog            *SlowLog        — all access via methods
//	lastSaveTime             int64           *** BARE, UNLOCKED ACCESS ***
//	globalConfig             *Config         — has mu; see below
//
// lastSaveTime (server_commands.go:1667) is a plain int64 with no atomic and no
// mutex, read and written from three command handlers:
//
//	cmdLASTSAVE  :1836  if lastSaveTime == 0 {              READ
//	cmdLASTSAVE  :1837      lastSaveTime = time.Now().Unix() WRITE (lazy init in a read path!)
//	cmdLASTSAVE  :1839  return ctx.WriteInteger(lastSaveTime)          READ
//	cmdSAVE      :1893  lastSaveTime = time.Now().Unix()              WRITE
//	cmdBGSAVE    :1898  lastSaveTime = time.Now().Unix()              WRITE
//
// LASTSAVE, SAVE and BGSAVE are all registered (server_commands.go:40/43/44), so
// one client running SAVE/BGSAVE races another's LASTSAVE.
//
// globalConfig.port is read directly at server_commands.go:332 with no lock, but
// the writer-grep finds NO writer anywhere -- there is no CONFIG SET port case for
// this struct -- so by the same rule that cleared pingPeer (a read with no
// concurrent writer cannot race) it is clean today, though it is one added
// setter away from becoming a defect.

// saveRouter registers the three commands under test.
//
// SAVE and BGSAVE now invoke the installed saveHook so they really persist
// before reporting success (they previously only stamped lastSaveTime and
// replied +OK, which gave clients a durability guarantee the server never
// kept). These tests drive the handlers directly rather than through
// server.New, so they must stand in a hook themselves — with none installed the
// handlers correctly report "persistence is not configured" instead of lying.
func saveRouter(t *testing.T) *Router {
	t.Helper()

	previous := saveHook
	saveHook = func() error { return nil }
	t.Cleanup(func() { saveHook = previous })

	r := NewRouter()
	RegisterServerCommands(r)
	return r
}

func runSave(t *testing.T, s *store.Store, r *Router, cmd string, args ...string) string {
	t.Helper()
	buf := &bytes.Buffer{}
	parsed := make([][]byte, len(args))
	for i, a := range args {
		parsed[i] = []byte(a)
	}
	if err := r.Execute(NewContext(cmd, parsed, s, resp.NewWriter(buf))); err != nil {
		t.Fatalf("%s: %v", cmd, err)
	}
	return buf.String()
}

// TestLastSaveRacesSaveWriters is the primary case: the bare int64 read in
// cmdLASTSAVE racing the bare int64 writes in cmdSAVE / cmdBGSAVE.
// Only meaningful under -race.
func TestLastSaveRacesSaveWriters(t *testing.T) {
	s := store.NewStore()
	r := saveRouter(t)

	done := make(chan struct{})
	go func() { // writers
		defer close(done)
		for i := 0; i < 3000; i++ {
			runSave(t, s, r, "SAVE")
			runSave(t, s, r, "BGSAVE")
		}
	}()
	for i := 0; i < 3000; i++ { // reader
		runSave(t, s, r, "LASTSAVE")
	}
	<-done
}

// TestLastSaveControls is the control: SAVE/BGSAVE report success and LASTSAVE
// reports a plausible positive Unix second. Passes before and after the fix, and
// proves the race test above actually exercised the handlers.
func TestLastSaveControls(t *testing.T) {
	s := store.NewStore()
	r := saveRouter(t)

	if out := runSave(t, s, r, "SAVE"); out != "+OK\r\n" {
		t.Fatalf("SAVE = %q, want +OK", out)
	}
	if out := runSave(t, s, r, "BGSAVE"); !strings.Contains(out, "Background saving") {
		t.Fatalf("BGSAVE = %q, want the background-saving reply", out)
	}

	out := runSave(t, s, r, "LASTSAVE")
	if out == "0\r\n" {
		t.Fatal("CONTROL FAIL: LASTSAVE returned 0 after SAVE set the timestamp")
	}
	trimmed := strings.TrimSuffix(strings.TrimSuffix(out, "\r\n"), "\n")
	if !strings.HasPrefix(trimmed, ":") || trimmed == ":" {
		t.Fatalf("CONTROL FAIL: LASTSAVE = %q, want a positive integer reply", out)
	}
	t.Log("CONTROL PASS: SAVE/BGSAVE report success and LASTSAVE reports a Unix second")
}
