package command

import (
	"context"
	"fmt"
	"math"
	"sort"
	"strconv"
	"strings"

	lua "github.com/yuin/gopher-lua"

	"github.com/cachestorm/cachestorm/internal/resp"
)

// Chunk names Redis 7 gives EVAL scripts and function libraries. They show up
// in Lua error messages ("user_script:1: ...") and, prefixed with '@', in the
// "script: <name>, on @user_script:1." suffix of a script error reply.
const (
	luaChunkScript   = "user_script"
	luaChunkFunction = "user_function"
	luaMaxReplyDepth = 1000
)

// scriptRun is the state of one script invocation on behalf of a client.
// redis.call and redis.pcall dispatch through the router's registered
// handlers with ctx, a copy of the caller's context: state a command changes
// inside the script stays inside the script.
type scriptRun struct {
	ctx      *Context
	router   *Router
	engine   *ScriptEngine
	guard    ScriptCallGuard
	readOnly bool
	// name identifies the script in error replies: the SHA1 for EVAL, the
	// function name for FCALL.
	name  string
	chunk string
	// aclErr records an ACL refusal. It aborts the script with that error
	// even when the script caught the Lua error with pcall.
	aclErr error
	// wrote records that the script ran a write. Like Redis, a script is
	// refused for memory only before its first write, so it is never left
	// half applied.
	wrote bool
}

func newScriptRun(ctx *Context, e *ScriptEngine, name, chunk string, readOnly bool) *scriptRun {
	sc := *ctx
	sc.InExec = true
	return &scriptRun{
		ctx:      &sc,
		router:   ctx.router,
		engine:   e,
		guard:    scriptGuard(ctx),
		readOnly: readOnly,
		name:     name,
		chunk:    chunk,
	}
}

// replyError is an error whose text is sent to the client verbatim; Redis'
// texts are capitalised and some end with a period.
type replyError string

func (e replyError) Error() string { return string(e) }

// luaErrTable builds the {err=msg} table Redis uses for error replies.
func luaErrTable(L *lua.LState, msg string) *lua.LTable {
	tbl := L.NewTable()
	tbl.RawSetString("err", lua.LString(msg))
	return tbl
}

// raiseLuaErr raises a Redis-style error table, which a script error handler
// turns into "<msg> script: <name>, on @user_script:<line>.".
func raiseLuaErr(L *lua.LState, msg string) {
	L.Error(luaErrTable(L, msg), 0)
}

// luaErrorText formats msg the way Redis' luaPushErrorBuff does: "-CODE msg"
// keeps its code, anything else gets the generic ERR code.
func luaErrorText(msg string) string {
	if strings.HasPrefix(msg, "-") {
		if sp := strings.IndexByte(msg, ' '); sp >= 0 {
			return msg[1:sp] + " " + strings.Trim(msg[sp+1:], "\r\n")
		}
		return "ERR " + strings.Trim(msg[1:], "\r\n")
	}
	return "ERR " + strings.Trim(msg, "\r\n")
}

// formatLuaNumber renders a redis.call number argument the way Redis 7.4
// does (fpconv_dtoa): the shortest round-trip form, integers in plain digits.
func formatLuaNumber(n float64) string {
	if n == math.Trunc(n) && math.Abs(n) < 1e21 {
		return strconv.FormatFloat(n, 'f', -1, 64)
	}
	return strconv.FormatFloat(n, 'g', -1, 64)
}

// luaCallArgs converts the redis.call arguments to command arguments.
// Booleans are accepted as "1"/"0" for backward compatibility; Redis refuses
// them along with every other non-string, non-number value.
func luaCallArgs(L *lua.LState) ([]string, string) {
	n := L.GetTop()
	if n == 0 {
		return nil, "ERR Please specify at least one argument for this redis lib call"
	}
	argv := make([]string, 0, n)
	for i := 1; i <= n; i++ {
		switch v := L.Get(i).(type) {
		case lua.LString:
			argv = append(argv, string(v))
		case lua.LNumber:
			argv = append(argv, formatLuaNumber(float64(v)))
		case lua.LBool:
			if bool(v) {
				argv = append(argv, "1")
			} else {
				argv = append(argv, "0")
			}
		default:
			return nil, "ERR Lua redis lib command arguments must be strings or integers"
		}
	}
	return argv, ""
}

// luaCallFunc returns the redis.call (raise=true) or redis.pcall binding.
func (r *scriptRun) luaCallFunc(raise bool) lua.LGFunction {
	return func(L *lua.LState) int {
		v, isErr := r.call(L)
		if isErr && raise {
			L.Error(v, 0)
			return 0
		}
		L.Push(v)
		return 1
	}
}

// call runs one redis.call / redis.pcall and returns its reply converted to
// Lua, and whether the reply is an error.
func (r *scriptRun) call(L *lua.LState) (lua.LValue, bool) {
	argv, argErr := luaCallArgs(L)
	if argErr != "" {
		return luaErrTable(L, argErr), true
	}
	if r.aclErr != nil {
		return luaErrTable(L, r.aclErr.Error()), true
	}
	if r.guard != nil {
		if err := r.guard(argv[0], argv[1:]); err != nil {
			r.aclErr = err
			return luaErrTable(L, err.Error()), true
		}
	}
	if r.router == nil {
		return r.legacyCall(L, argv)
	}

	name := CanonicalName(argv[0])
	if _, ok := r.router.Get(name); !ok {
		return luaErrTable(L, "ERR Unknown Redis command called from script"), true
	}
	if !checkArity(name, len(argv)-1) {
		return luaErrTable(L, "ERR Wrong number of args calling Redis command from script"), true
	}
	if scriptForbidden(name) {
		return luaErrTable(L, "ERR This Redis command is not allowed from script"), true
	}
	if r.readOnly && commandHasFlag(name, "write") {
		return luaErrTable(L, "ERR Write commands are not allowed from read-only scripts."), true
	}

	if !r.wrote && overMemoryFor(r.ctx, name) {
		return luaErrTable(L, errOOM.Error()), true
	}

	args := make([][]byte, len(argv)-1)
	for i, a := range argv[1:] {
		args[i] = []byte(a)
	}
	reply := executeQueuedCommand(r.ctx, queuedCommand{cmd: name, args: args})
	if reply.Type != resp.TypeError && commandHasFlag(name, "write") {
		r.wrote = true
	}
	if reply.Type == resp.TypeError {
		return luaErrTable(L, reply.Err), true
	}
	return respToLua(L, reply), false
}

// legacyCall serves redis.call for a script run without a router (a handler
// invoked directly rather than through Router.Execute): it uses the engine's
// built-in command subset, which reports errors as plain strings.
func (r *scriptRun) legacyCall(L *lua.LState, argv []string) (v lua.LValue, isErr bool) {
	defer func() {
		if rec := recover(); rec != nil {
			v, isErr = luaErrTable(L, fmt.Sprintf("ERR %v", rec)), true
		}
	}()
	return r.engine.executeCommand(nil, L, argv[0], argv[1:]), false
}

// scriptForbidden reports whether Redis refuses cmd inside a script. The
// container commands carry no flags of their own, but every one of their
// subcommands is flagged noscript.
func scriptForbidden(cmd string) bool {
	switch cmd {
	case "SCRIPT", "FUNCTION", "CONFIG", "CLIENT", "ACL", "LATENCY", "MODULE":
		return true
	}
	return commandHasFlag(cmd, "noscript")
}

func commandHasFlag(cmd, flag string) bool {
	spec, ok := redisCommandSpecs[cmd]
	if !ok {
		return false
	}
	for _, f := range spec.Flags {
		if f == flag {
			return true
		}
	}
	return false
}

// respToLua converts a command reply to Lua the way Redis does for RESP2:
// integers become numbers, bulk strings strings, a null bulk or array false,
// arrays tables, a status {ok=...} and an error {err=...}.
func respToLua(L *lua.LState, v *resp.Value) lua.LValue {
	if v == nil {
		return lua.LFalse
	}
	switch v.Type {
	case resp.TypeInteger:
		return lua.LNumber(v.Int)
	case resp.TypeBulkString:
		if v.IsNull {
			return lua.LFalse
		}
		return lua.LString(v.Bulk)
	case resp.TypeSimpleString:
		tbl := L.NewTable()
		tbl.RawSetString("ok", lua.LString(v.Str))
		return tbl
	case resp.TypeError:
		return luaErrTable(L, v.Err)
	case resp.TypeArray:
		if v.IsNull {
			return lua.LFalse
		}
		tbl := L.CreateTable(len(v.Array), 0)
		for _, item := range v.Array {
			tbl.Append(respToLua(L, item))
		}
		return tbl
	case resp.TypeMap:
		keys := make([]string, 0, len(v.Map))
		for k := range v.Map {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		tbl := L.CreateTable(2*len(keys), 0)
		for _, k := range keys {
			tbl.Append(lua.LString(k))
			tbl.Append(respToLua(L, v.Map[k]))
		}
		return tbl
	default:
		return lua.LFalse
	}
}

// luaToResp converts a script's return value to a reply the way Redis does:
// a number becomes an integer (truncated), a string a bulk string, true 1,
// false and nil a null, {ok=...} a status, {err=...} an error, and any other
// table an array that stops at the first nil.
func luaToResp(v lua.LValue, name string, depth int) *resp.Value {
	switch val := v.(type) {
	case lua.LString:
		return resp.BulkString(string(val))
	case lua.LBool:
		if bool(val) {
			return resp.IntegerValue(1)
		}
		return resp.NullValue()
	case lua.LNumber:
		return resp.IntegerValue(luaNumberToInt(float64(val)))
	case *lua.LTable:
		if depth >= luaMaxReplyDepth {
			return resp.ErrorValue("ERR reached lua stack limit")
		}
		if _, ok := val.RawGetString("err").(lua.LString); ok {
			return luaErrReply(val, name)
		}
		if ok, isStr := val.RawGetString("ok").(lua.LString); isStr {
			return resp.SimpleString(string(ok))
		}
		if d, isNum := val.RawGetString("double").(lua.LNumber); isNum {
			return resp.BulkString(strconv.FormatFloat(float64(d), 'g', 17, 64))
		}
		items := make([]*resp.Value, 0, val.Len())
		for i := 1; ; i++ {
			item := val.RawGetInt(i)
			if item == lua.LNil {
				break
			}
			items = append(items, luaToResp(item, name, depth+1))
		}
		return resp.ArrayValue(items)
	default:
		return resp.NullValue()
	}
}

// luaNumberToInt truncates like Redis' (long long) cast; a value out of range
// (or NaN) gives what that cast yields on x86-64, the minimum int64.
func luaNumberToInt(f float64) int64 {
	if math.IsNaN(f) || f >= math.MaxInt64 || f < math.MinInt64 {
		return math.MinInt64
	}
	return int64(f)
}

// luaErrReply turns an {err=msg[, source=, line=]} table into an error reply;
// the location suffix is present when the error was raised.
func luaErrReply(tbl *lua.LTable, name string) *resp.Value {
	msg := "ERR unknown error"
	if s, ok := tbl.RawGetString("err").(lua.LString); ok {
		msg = string(s)
	}
	src, hasSrc := tbl.RawGetString("source").(lua.LString)
	line, hasLine := tbl.RawGetString("line").(lua.LNumber)
	if hasSrc && hasLine {
		msg = fmt.Sprintf("%s script: %s, on %s:%d.", msg, name, string(src), int64(line))
	}
	return resp.ErrorValue(msg)
}

// errHandler is the message handler for the script's protected call. Like
// Redis' __redis__err__handler it turns a plain error into {err='ERR ...'}
// and records the script line the error was raised on.
func (r *scriptRun) errHandler(L *lua.LState) *lua.LFunction {
	return L.NewFunction(func(L *lua.LState) int {
		obj := L.Get(1)
		tbl, ok := obj.(*lua.LTable)
		if !ok {
			tbl = luaErrTable(L, "ERR "+obj.String())
		}
		for lvl := 0; ; lvl++ {
			dbg, ok := L.GetStack(lvl)
			if !ok {
				break
			}
			if _, err := L.GetInfo("Sl", dbg, lua.LNil); err == nil && dbg.Source == r.chunk {
				tbl.RawSetString("source", lua.LString("@"+r.chunk))
				tbl.RawSetString("line", lua.LNumber(dbg.CurrentLine))
				break
			}
		}
		L.Push(tbl)
		return 1
	})
}

// finish converts the outcome of the script's protected call to a reply.
func (r *scriptRun) finish(L *lua.LState, tctx context.Context, err error) *resp.Value {
	if r.aclErr != nil {
		return resp.ErrorValue(r.aclErr.Error())
	}
	if err != nil {
		if tctx.Err() != nil {
			return resp.ErrorValue(fmt.Sprintf("ERR script timeout exceeded %.0f seconds", luaScriptTimeout.Seconds()))
		}
		if apiErr, ok := err.(*lua.ApiError); ok {
			if tbl, ok := apiErr.Object.(*lua.LTable); ok {
				return luaErrReply(tbl, r.name)
			}
			return resp.ErrorValue("ERR " + apiErr.Object.String())
		}
		return resp.ErrorValue("ERR " + err.Error())
	}
	ret := L.Get(-1)
	L.Pop(1)
	return luaToResp(ret, r.name, 0)
}

// luaCompileError renders a gopher-lua syntax error close to Redis'
// "user_script:1: <message>" shape.
func luaCompileError(err error) string {
	msg := err.Error()
	if apiErr, ok := err.(*lua.ApiError); ok && apiErr.Object != nil {
		msg = apiErr.Object.String()
	}
	msg = strings.Join(strings.Fields(msg), " ")
	for _, chunk := range []string{luaChunkScript, luaChunkFunction} {
		prefix := chunk + " line:"
		if strings.HasPrefix(msg, prefix) {
			rest := msg[len(prefix):]
			end := strings.IndexByte(rest, '(')
			if end > 0 {
				lineNo := rest[:end]
				if close := strings.Index(rest, ") "); close >= 0 {
					rest = rest[close+2:]
				}
				return chunk + ":" + lineNo + ": " + rest
			}
		}
		if strings.HasPrefix(msg, chunk+" at EOF:") {
			return chunk + ": " + strings.TrimSpace(msg[len(chunk)+1:])
		}
	}
	return msg
}

// scriptShebang holds what a "#!lua" first line declares.
type scriptShebang struct {
	noWrites bool
}

// parseScriptShebang strips a Redis 7 "#!lua [flags=...]" first line from an
// EVAL script, keeping the line count so error lines stay right.
func parseScriptShebang(script string) (string, scriptShebang, string) {
	var sb scriptShebang
	if !strings.HasPrefix(script, "#!") {
		return script, sb, ""
	}
	line := script
	rest := ""
	if nl := strings.IndexByte(script, '\n'); nl >= 0 {
		line, rest = script[:nl], script[nl:]
	}
	parts := strings.Fields(line[2:])
	if len(parts) == 0 || parts[0] != "lua" {
		engine := ""
		if len(parts) > 0 {
			engine = parts[0]
		}
		return "", sb, "ERR Unexpected engine in script shebang: #!" + engine
	}
	for _, p := range parts[1:] {
		if !strings.HasPrefix(p, "flags=") {
			return "", sb, "ERR Unknown lua shebang option: " + p
		}
		for _, f := range strings.Split(strings.TrimPrefix(p, "flags="), ",") {
			switch f {
			case "no-writes":
				sb.noWrites = true
			case "", "allow-oom", "allow-stale", "no-cluster", "allow-cross-slot-keys":
			default:
				return "", sb, "ERR Unexpected flag in script shebang: " + f
			}
		}
	}
	return rest, sb, ""
}

// compileScript reports the Redis-style compile error for script, or "".
func (e *ScriptEngine) compileScript(script string) string {
	body, _, errMsg := parseScriptShebang(script)
	if errMsg != "" {
		return errMsg
	}
	L := lua.NewState(lua.Options{SkipOpenLibs: true})
	defer L.Close()
	if _, err := L.Load(strings.NewReader(body), luaChunkScript); err != nil {
		return "ERR Error compiling script (new function): " + luaCompileError(err)
	}
	return ""
}

// lookupScript returns the cached script for sha (case-insensitive).
func (e *ScriptEngine) lookupScript(sha string) (string, bool) {
	e.mu.RLock()
	defer e.mu.RUnlock()
	script, ok := e.scripts[strings.ToLower(sha)]
	return script, ok
}

// runScript runs an EVAL/EVALSHA script for the client in ctx and returns
// its reply. A script that compiles is added to the script cache, as Redis
// does for EVAL.
func (e *ScriptEngine) runScript(ctx *Context, script string, keys, args []string, readOnly bool) *resp.Value {
	body, sb, errMsg := parseScriptShebang(script)
	if errMsg != "" {
		return resp.ErrorValue(errMsg)
	}
	run := newScriptRun(ctx, e, ScriptSHA(script), luaChunkScript, readOnly || sb.noWrites)
	L := e.newState(keys, args, run.guard, run)
	defer L.Close()

	fn, err := L.Load(strings.NewReader(body), luaChunkScript)
	if err != nil {
		return resp.ErrorValue("ERR Error compiling script (new function): " + luaCompileError(err))
	}
	e.cacheEvalScript(script)
	protectGlobals(L)

	tctx, cancel := context.WithTimeout(context.Background(), luaScriptTimeout)
	defer cancel()
	L.SetContext(tctx)

	L.Push(fn)
	err = L.PCall(0, 1, run.errHandler(L))
	return run.finish(L, tctx, err)
}
