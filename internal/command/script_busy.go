package command

import (
	"context"
	"sync"
	"sync/atomic"
	"time"

	lua "github.com/cachestorm/cachestorm/third_party/gopherlua"

	"github.com/cachestorm/cachestorm/internal/resp"
)

// A script runs until it returns, as in Redis: there is no time limit. Once
// one has run for busy-reply-threshold milliseconds, other clients' commands
// are answered BUSY, except the few Redis lets through, and SCRIPT KILL or
// FUNCTION KILL stops it if it has not written yet.

// busyReplyThresholdMS is busy-reply-threshold (lua-time-limit).
var busyReplyThresholdMS atomic.Int64

func init() { busyReplyThresholdMS.Store(5000) }

const (
	errBusyEval     = "BUSY Redis is busy running a script. You can only call SCRIPT KILL or SHUTDOWN NOSAVE."
	errBusyFunction = "BUSY Redis is busy running a script. You can only call FUNCTION KILL or SHUTDOWN NOSAVE."
	errNotBusy      = "NOTBUSY No scripts in execution right now."
	errKillMaster   = "UNKILLABLE The busy script was sent by a master instance in the context of replication and cannot be killed."
	errKillWritten  = "UNKILLABLE Sorry the script already executed write commands against the dataset. You can either wait the script termination or kill the server in a hard way using the SHUTDOWN NOSAVE command."
	errScriptKilled = "ERR Script killed by user with SCRIPT KILL..."
)

// runningScripts are the scripts a router is running. Redis runs one at a
// time; CacheStorm may run scripts on unrelated keys at once, and the
// commands below act on the one that started first.
type runningScripts struct {
	mu   sync.Mutex
	runs []*scriptRun
}

// begin registers r as running with cancel stopping it. command is the
// EVAL or FCALL that runs it.
func (r *scriptRun) begin(cancel context.CancelFunc, eval bool) {
	r.started = time.Now()
	r.cancel = cancel
	r.eval = eval
	r.fromMaster = r.parent.FromMaster || r.parent.Replaying
	r.command = append([]string{r.parent.Command}, argStrings(r.parent)...)
	if r.router == nil {
		return
	}
	s := &r.router.scripts
	s.mu.Lock()
	s.runs = append(s.runs, r)
	s.mu.Unlock()
}

// end removes r from the running scripts.
func (r *scriptRun) end() {
	if r.router == nil {
		return
	}
	s := &r.router.scripts
	s.mu.Lock()
	for i, run := range s.runs {
		if run == r {
			s.runs = append(s.runs[:i], s.runs[i+1:]...)
			break
		}
	}
	s.mu.Unlock()
}

// oldestScript is the script running longest, nil when none runs.
func (rt *Router) oldestScript() *scriptRun {
	if rt == nil {
		return nil
	}
	s := &rt.scripts
	s.mu.Lock()
	defer s.mu.Unlock()
	if len(s.runs) == 0 {
		return nil
	}
	return s.runs[0]
}

// allowedWhileBusy are the commands Redis runs while a script is busy (the
// allow-busy flag).
func allowedWhileBusy(cmd string, args [][]byte) bool {
	switch cmd {
	case "AUTH", "HELLO", "MULTI", "DISCARD", "WATCH", "UNWATCH", "QUIT", "RESET", "REPLCONF":
		return true
	case "SHUTDOWN":
		// Only SHUTDOWN NOSAVE: saving would record a script half run.
		return shutdownNoSave(args)
	case "SCRIPT":
		return len(args) > 0 && equalFoldASCII(args[0], "KILL")
	case "FUNCTION":
		return len(args) > 0 && (equalFoldASCII(args[0], "KILL") || equalFoldASCII(args[0], "STATS"))
	}
	return false
}

// ScriptControl reports whether the command is SCRIPT KILL, FUNCTION KILL or
// FUNCTION STATS, which act on a running script and must not wait for the
// locks that script holds.
func ScriptControl(cmd string, args [][]byte) bool {
	switch cmd {
	case "SCRIPT":
		return len(args) == 1 && equalFoldASCII(args[0], "KILL")
	case "FUNCTION":
		return len(args) == 1 && (equalFoldASCII(args[0], "KILL") || equalFoldASCII(args[0], "STATS"))
	}
	return false
}

func shutdownNoSave(args [][]byte) bool {
	for _, a := range args {
		if equalFoldASCII(a, "NOSAVE") {
			return true
		}
	}
	return false
}

// ShutdownBesideScript reports whether the command is a SHUTDOWN NOSAVE
// sent while a script runs: it must not wait for the locks the script holds,
// as it is how Redis ends a script SCRIPT KILL cannot stop.
func (rt *Router) ShutdownBesideScript(cmd string, args [][]byte) bool {
	return cmd == "SHUTDOWN" && shutdownNoSave(args) && rt.oldestScript() != nil
}

// stopScripts stops every running script, written or not, for SHUTDOWN
// NOSAVE.
func (rt *Router) stopScripts() {
	if rt == nil {
		return
	}
	s := &rt.scripts
	s.mu.Lock()
	defer s.mu.Unlock()
	for _, run := range s.runs {
		run.killed.Store(true)
		run.cancel()
	}
}

func equalFoldASCII(b []byte, s string) bool {
	if len(b) != len(s) {
		return false
	}
	for i := range b {
		c := b[i]
		if 'a' <= c && c <= 'z' {
			c -= 'a' - 'A'
		}
		if c != s[i] {
			return false
		}
	}
	return true
}

// ScriptBusy is the BUSY error a client's command gets while a script has
// run for busy-reply-threshold or longer, "" when the command may run. The
// connection asks before the command takes any lock, as Redis refuses it
// before it runs.
func (rt *Router) ScriptBusy(cmd string, args [][]byte) string {
	run := rt.oldestScript()
	if run == nil || allowedWhileBusy(cmd, args) {
		return ""
	}
	if time.Since(run.started) < time.Duration(busyReplyThresholdMS.Load())*time.Millisecond {
		return ""
	}
	if run.eval {
		return errBusyEval
	}
	return errBusyFunction
}

// killScript is SCRIPT KILL (eval) and FUNCTION KILL.
func killScript(ctx *Context, eval bool) error {
	run := ctx.dispatcher().oldestScript()
	switch {
	case run == nil:
		return ctx.WriteError(replyError(errNotBusy))
	case run.fromMaster:
		return ctx.WriteError(replyError(errKillMaster))
	case run.dirty.Load():
		return ctx.WriteError(replyError(errKillWritten))
	case eval && !run.eval:
		return ctx.WriteError(replyError(errBusyFunction))
	case !eval && run.eval:
		return ctx.WriteError(replyError(errBusyEval))
	}
	run.killed.Store(true)
	run.cancel()
	return ctx.WriteOK()
}

// runningScriptValue is FUNCTION STATS' running_script: the function
// running, the FCALL that runs it and how long it has run.
func runningScriptValue(run *scriptRun) *resp.Value {
	if run == nil {
		return resp.NullBulkString()
	}
	cmd := make([]*resp.Value, len(run.command))
	for i, a := range run.command {
		cmd[i] = resp.BulkString(a)
	}
	return resp.PairsValue([]*resp.Value{
		resp.BulkString("name"), resp.BulkString(run.name),
		resp.BulkString("command"), resp.ArrayValue(cmd),
		resp.BulkString("duration_ms"), resp.IntegerValue(time.Since(run.started).Milliseconds()),
	})
}

// killedReply is the error a killed script ends with, at the line it was
// running, as Redis words it.
func (r *scriptRun) killedReply(err error) *resp.Value {
	if apiErr, ok := err.(*lua.ApiError); ok {
		if tbl, ok := apiErr.Object.(*lua.LTable); ok {
			tbl.RawSetString("err", lua.LString(errScriptKilled))
			return luaErrReply(tbl, r.name)
		}
	}
	return resp.ErrorValue(errScriptKilled)
}
