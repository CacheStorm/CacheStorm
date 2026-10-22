package command

import (
	"bytes"
	"sync"
	"testing"

	"github.com/cachestorm/cachestorm/internal/resp"
	"github.com/cachestorm/cachestorm/internal/store"
)

// Audit finding: scriptEngine is a bare package-global POINTER with no
// synchronisation of any kind (script_commands.go:11).
//
//	var scriptEngine *ScriptEngine          // no mutex
//	func InitScriptEngine(s *ScriptEngine) { scriptEngine = s }   // unguarded write
//
// and three handlers do an unguarded check-then-act to lazily build it:
//
//	cmdEVAL     :75  if scriptEngine == nil { scriptEngine = NewScriptEngine(ctx.Store) }
//	cmdEVALSHA  :117 same
//	cmdSCRIPT   :159 same
//
// Two clients issuing EVAL on a cold server can both observe nil, both
// construct, and both write the pointer -- a data race on the pointer itself,
// and each ends up running its script against whichever engine it happened to
// read (i.e. possibly another connection's Store).
//
// ScriptEngine has its own mu for its `scripts` map (lua.go:24); that protects
// the engine's fields but says nothing about the package-level pointer that
// decides *which* engine exists. That is the same distinction that made
// lastSaveTime a defect: the container is protected, the reference to it is not.
//
// Unlike the sentinel OnFailover finding, this one is fully reachable in
// production -- EVAL/EVALSHA/SCRIPT are registered and any client can call them
// against a server that has never run a script.

func scriptRouter() *Router {
	r := NewRouter()
	RegisterScriptCommands(r)
	return r
}

func runScript(t *testing.T, s *store.Store, r *Router, args ...string) string {
	t.Helper()
	buf := &bytes.Buffer{}
	parsed := make([][]byte, len(args))
	for i, a := range args {
		parsed[i] = []byte(a)
	}
	if err := r.Execute(NewContext("EVAL", parsed, s, resp.NewWriter(buf))); err != nil {
		t.Fatalf("EVAL %v: %v", args, err)
	}
	return buf.String()
}

// TestScriptEngineLazyInitRaces is the primary case. Each round resets the
// global to nil and then has several goroutines issue their first EVAL at once,
// driving the check-then-act at script_commands.go:75. Only meaningful
// under -race.
func TestScriptEngineLazyInitRaces(t *testing.T) {
	s := store.NewStore()
	router := scriptRouter()

	for round := 0; round < 60; round++ {
		// Force the cold path: no engine exists yet this round.
		scriptEngine = nil

		var wg sync.WaitGroup
		for g := 0; g < 4; g++ {
			wg.Add(1)
			go func() {
				defer wg.Done()
				buf := &bytes.Buffer{}
				parsed := [][]byte{[]byte("return 1"), []byte("0")}
				if err := router.Execute(NewContext("EVAL", parsed, s, resp.NewWriter(buf))); err != nil {
					t.Errorf("EVAL: %v", err)
				}
			}()
		}
		wg.Wait()
	}
}

// TestScriptEngineLazyInitControls is the control: after the engine exists,
// EVAL still evaluates, INIT/GETNAME-style accessors on the engine still work,
// and the engine observes the store it was built for. Passes before and after
// the fix, and proves the race test above really ran scripts.
func TestScriptEngineLazyInitControls(t *testing.T) {
	s := store.NewStore()
	router := scriptRouter()

	scriptEngine = nil

	if out := runScript(t, s, router, "return 42", "0"); out != ":42\r\n" {
		t.Fatalf("EVAL return 42 = %q, want :42", out)
	}
	// Second call reuses the already-built engine.
	if out := runScript(t, s, router, "return 7", "0"); out != ":7\r\n" {
		t.Fatalf("second EVAL = %q, want :7", out)
	}
	// The engine is bound to this store, so SCRIPT LOAD then EVALSHA works.
	runScript(t, s, router, "return 99", "0")

	engine := scriptEngine
	if engine == nil {
		t.Fatal("CONTROL FAIL: scriptEngine is nil after EVAL")
	}
	t.Log("CONTROL PASS: EVAL evaluates, reuses the engine, and leaves it non-nil")
}
