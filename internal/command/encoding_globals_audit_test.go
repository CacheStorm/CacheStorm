package command

import (
	"bytes"
	"fmt"
	"sync"
	"testing"

	"github.com/cachestorm/cachestorm/internal/resp"
	"github.com/cachestorm/cachestorm/internal/store"
)

// Audit result for the last of internal/command's package globals. Across
// list_commands.go, set_commands.go, hash_commands.go, sortedset_commands.go,
// encoding_commands.go, bitmap_commands.go and stream_commands.go the ONLY
// mutable package-level state is two globals in encoding_commands.go:
//
//	encodingRand   *rand.Rand                + encodingRandMu sync.Mutex   (lines 18-19)
//	pools          map[string]*ResourcePool  + poolsMu       sync.RWMutex (lines 1061-1063)
//
// Everything else in those files is a sentinel error (errLPosRankZero,
// errCountOutOfRange, errInvalidPercentEncoding, ErrBusyGroup, ErrNoGroup),
// which is immutable after init, or nothing at all -- hash_commands.go and
// sortedset_commands.go declare no package-level vars, bitmap_commands.go's
// `var _ = binary.BigEndian` is a blank identifier rather than state, and
// key_commands.go does not exist (its handlers live in server_commands.go).
//
// Both globals were verified by reading every non-test reference:
//   - encodingRand has exactly four: the decl, Lock/Unlock inside randomInt
//     (encoding_commands.go:621-622), and the single call encodingRand.Intn(n)
//     at :623 that those two lines guard.
//   - pools is written once at :1082 under poolsMu.Lock(), and read at :1100,
//     :1129, :1155 and :1177, each inside an RLock pair released before the pool
//     is touched.
//
// Reading proves only what was read, so this test measures it instead. It is
// meaningful ONLY under -race.
//
// TARGETING NOTE: an earlier version of this test drove randomInt with
// UUID.VALIDATE, which turned out to be vacuous -- UUID.VALIDATE runs
// isValidUUID, a pure predicate that never calls randomInt. The real path is
// JOBX.CREATE -> cmdJOBXCREATE -> generateUUID() -> randomInt(16) ->
// encodingRand.Intn(n), so that is what drives the global here.

func encodingAuditRouter() *Router {
	r := NewRouter()
	RegisterEncodingCommands(r)  // POOL.CREATE / POOL.GET -> pools, poolsMu
	RegisterAdvancedCommands2(r) // JOBX.CREATE -> generateUUID -> randomInt -> encodingRand
	return r
}

func runEncAudit(t *testing.T, s *store.Store, r *Router, cmd string, args ...string) string {
	t.Helper()
	buf := &bytes.Buffer{}
	parsed := make([][]byte, len(args))
	for i, a := range args {
		parsed[i] = []byte(a)
	}
	if err := r.Execute(NewContext(cmd, parsed, s, resp.NewWriter(buf))); err != nil {
		t.Fatalf("%s %v: %v", cmd, args, err)
	}
	return buf.String()
}

// TestEncodingGlobalsAreRaceFree drives randomInt (via generateUUID) and the
// ResourcePool registry concurrently from several goroutines.
func TestEncodingGlobalsAreRaceFree(t *testing.T) {
	s := store.NewStore()
	router := encodingAuditRouter()

	var wg sync.WaitGroup
	// randomInt/encodingRand writer side.
	for g := 0; g < 3; g++ {
		wg.Add(1)
		go func(g int) {
			defer wg.Done()
			for i := 0; i < 1200; i++ {
				runEncAudit(t, s, router, "JOBX.CREATE", fmt.Sprintf("job-%d-%d", g, i))
			}
		}(g)
	}
	// pools writer and reader.
	wg.Add(1)
	go func() {
		defer wg.Done()
		for i := 0; i < 1200; i++ {
			runEncAudit(t, s, router, "POOL.CREATE", fmt.Sprintf("audit-pool-%d", i%50), "4")
		}
	}()
	wg.Add(1)
	go func() {
		defer wg.Done()
		for i := 0; i < 1200; i++ {
			runEncAudit(t, s, router, "POOL.GET", fmt.Sprintf("audit-pool-%d", i%50))
		}
	}()
	wg.Wait()
}

// TestEncodingGlobalsControls is the control, and it is the part that proves the
// race test above is not vacuous: JOBX.CREATE must return a DISTINCT id each
// call, which can only happen if generateUUID actually ran randomInt. (Counting
// distinct replies from UUID.VALIDATE would prove nothing -- every valid UUID
// answers ":1".)
func TestEncodingGlobalsControls(t *testing.T) {
	s := store.NewStore()
	router := encodingAuditRouter()

	const n = 200
	ids := make(map[string]bool, n)
	for i := 0; i < n; i++ {
		out := runEncAudit(t, s, router, "JOBX.CREATE", fmt.Sprintf("ctl-%d", i))
		if len(out) == 0 {
			t.Fatalf("CONTROL FAIL: JOBX.CREATE returned an empty reply")
		}
		if ids[out] {
			t.Fatalf("CONTROL FAIL: JOBX.CREATE returned duplicate id %q on call %d -- "+
				"generateUUID is not running, so randomInt/encodingRand was never exercised", out, i)
		}
		ids[out] = true
	}
	if len(ids) != n {
		t.Fatalf("CONTROL FAIL: %d distinct ids from %d JOBX.CREATE calls, want %d", len(ids), n, n)
	}

	if out := runEncAudit(t, s, router, "POOL.CREATE", "ctl-pool", "8"); out != "+OK\r\n" {
		t.Fatalf("CONTROL FAIL: POOL.CREATE = %q, want +OK", out)
	}
	if out := runEncAudit(t, s, router, "POOL.GET", "ctl-pool"); len(out) == 0 {
		t.Fatal("CONTROL FAIL: POOL.GET returned an empty reply")
	}
	t.Logf("CONTROL PASS: %d distinct ids (randomInt ran), pool round-tripped", len(ids))
}
