package command

import (
	"bytes"
	"fmt"
	"strings"
	"sync"
	"testing"

	"github.com/cachestorm/cachestorm/internal/resp"
	"github.com/cachestorm/cachestorm/internal/store"
)

// Audit finding: two command handlers call SortedSetValue methods WITHOUT taking
// the value lock, breaking the caller-held contract that the other 25 call sites
// honour. SortedSetValue deliberately does not self-lock -- the doc on the value
// mutex states the caller must hold zset.Lock()/zset.RLock() -- so a caller that
// forgets is silently unprotected, exactly as Store.Get handing out a live
// *Entry was in the Entry.ExpiresAt audit.
//
// UNLOCKED SITES (verified by reading each handler in full, not by grep):
//
//	internal/command/sortedset_commands.go:837   zset.Rank(member, true)
//	    in cmdZREVRANK -- getSortedSet, nil-check, then straight to Rank().
//	    Rank() sorts and scans v.Members.
//	internal/command/sortedset_commands.go:1344  zset.GetScore(member)
//	    in cmdZMSCORE, inside the per-member loop. GetScore() does a bare
//	    v.Members[member] map read.
//
// WRITERS, which DO take the write lock (the command package was fixed for this
// in an earlier round -- cmdZADD/cmdZREM etc. hold zset.Lock()):
//	cmdZADD -> zset.Lock() -> zset.Add(...)  writes v.Members
//	cmdZREM -> zset.Lock() -> zset.Remove(...) deletes from v.Members
//
// So one client's ZADD/ZREM races another's ZREVRANK or ZMSCORE on the same set.
// Unlike the int64 case, this is a MAP: concurrent map read and map write is
// process-fatal -- `fatal error: concurrent map read and map write` -- so this
// can take the whole server down, not merely return a stale value.

func zlockRouter() *Router {
	r := NewRouter()
	RegisterSortedSetCommands(r)
	return r
}

func runZlock(t *testing.T, s *store.Store, r *Router, args ...string) string {
	t.Helper()
	buf := &bytes.Buffer{}
	parsed := make([][]byte, len(args))
	for i, a := range args {
		parsed[i] = []byte(a)
	}
	// t.Errorf (not Fatalf) so it is safe to call from spawned goroutines.
	if err := r.Execute(NewContext(args[0], parsed[1:], s, resp.NewWriter(buf))); err != nil {
		t.Errorf("%v: %v", args, err)
		return ""
	}
	return buf.String()
}

// TestZRevRankRacesZAddWriter is the primary case: cmdZREVRANK's unlocked
// zset.Rank() ranging v.Members against a locked ZADD writing it.
// Only meaningful under -race.
func TestZRevRankRacesZAddWriter(t *testing.T) {
	s := store.NewStore()
	r := zlockRouter()

	runZlock(t, s, r, "ZADD", "src", "1", "a", "2", "b", "3", "c")

	var wg sync.WaitGroup
	wg.Add(1)
	go func() { // locked writer
		defer wg.Done()
		for i := 0; i < 1500; i++ {
			runZlock(t, s, r, "ZADD", "src", fmt.Sprint(i%97+1), fmt.Sprintf("m%d", i))
		}
	}()
	for i := 0; i < 1500; i++ { // unlocked reader
		runZlock(t, s, r, "ZREVRANK", "src", fmt.Sprintf("m%d", i))
	}
	wg.Wait()
}

// TestZMScoreRacesZRemWriter is the second primary case: cmdZMSCORE's unlocked
// zset.GetScore() doing a bare v.Members[member] read against a locked ZREM.
func TestZMScoreRacesZRemWriter(t *testing.T) {
	s := store.NewStore()
	r := zlockRouter()

	for i := 0; i < 400; i++ {
		runZlock(t, s, r, "ZADD", "src", fmt.Sprint(i+1), fmt.Sprintf("m%d", i))
	}

	var wg sync.WaitGroup
	wg.Add(1)
	go func() { // locked writer
		defer wg.Done()
		for i := 0; i < 1500; i++ {
			runZlock(t, s, r, "ZREM", "src", fmt.Sprintf("m%d", i%400))
		}
	}()
	for i := 0; i < 1500; i++ { // unlocked reader
		runZlock(t, s, r, "ZMSCORE", "src", fmt.Sprintf("m%d", i%400))
	}
	wg.Wait()
}

// TestZLockControls is the control: ZREVRANK and ZMSCORE both work correctly on
// a settled set. Passes before and after the fix, and proves the race tests
// above actually reached the unlocked reads rather than erroring out early.
func TestZLockControls(t *testing.T) {
	s := store.NewStore()
	r := zlockRouter()

	runZlock(t, s, r, "ZADD", "k", "1", "a", "2", "b", "3", "c")

	// ZMSCORE returns an ARRAY -- one element per requested member, null for a
	// missing one (Redis semantics). An earlier version of this control asserted
	// an integer reply and was simply wrong about the command's contract.
	if out := runZlock(t, s, r, "ZMSCORE", "k", "b"); !strings.HasPrefix(out, "*1\r\n") ||
		!strings.Contains(out, "2") {
		t.Fatalf("CONTROL FAIL: ZMSCORE k b = %q, want a 1-element array holding \"2\"", out)
	}
	if out := runZlock(t, s, r, "ZMSCORE", "k", "nope"); !strings.HasPrefix(out, "*1\r\n") {
		t.Fatalf("CONTROL FAIL: ZMSCORE k nope = %q, want a 1-element array holding null", out)
	}
	// a=1,b=2,c=3 ascending, so c(3) has reverse rank 0.
	if out := runZlock(t, s, r, "ZREVRANK", "k", "c"); out != ":0\r\n" {
		t.Fatalf("CONTROL FAIL: ZREVRANK k c = %q, want :0", out)
	}
	if out := runZlock(t, s, r, "ZREVRANK", "k", "zz"); out != "_\r\n" {
		t.Fatalf("CONTROL FAIL: ZREVRANK k zz = %q, want null", out)
	}
	t.Log("CONTROL PASS: ZREVRANK and ZMSCORE return correct rank and score")
}
