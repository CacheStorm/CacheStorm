package command

import (
	"errors"
	"fmt"
	"strconv"
	"strings"
	"sync"

	"github.com/cachestorm/cachestorm/internal/resp"
	"github.com/cachestorm/cachestorm/internal/store"
)

var (
	scriptEngine   *ScriptEngine
	scriptEngineMu sync.RWMutex
)

func InitScriptEngine(s *ScriptEngine) {
	scriptEngineMu.Lock()
	scriptEngine = s
	scriptEngineMu.Unlock()
}

// getScriptEngine returns the shared engine, building it on first use.
//
// scriptEngine is a package-level pointer, so it needs its own guard: ScriptEngine
// locks its `scripts` map internally (lua.go), but that says nothing about the
// pointer deciding *which* engine exists. Without this, two connections issuing
// their first EVAL against a cold server both observe nil, both construct an
// engine, and both write the pointer -- and each script then runs against
// whichever engine it happened to read, possibly bound to another Store.
func getScriptEngine(s *store.Store) *ScriptEngine {
	scriptEngineMu.RLock()
	e := scriptEngine
	scriptEngineMu.RUnlock()
	if e != nil {
		return e
	}

	scriptEngineMu.Lock()
	defer scriptEngineMu.Unlock()
	if scriptEngine == nil {
		scriptEngine = NewScriptEngine(s)
	}
	return scriptEngine
}

func RegisterScriptCommands(router *Router) {
	router.Register(&CommandDef{Name: "EVAL", Handler: cmdEVAL})
	router.Register(&CommandDef{Name: "EVALSHA", Handler: cmdEVALSHA})
	router.Register(&CommandDef{Name: "EVAL_RO", Handler: cmdEVALRO})
	router.Register(&CommandDef{Name: "EVALSHA_RO", Handler: cmdEVALSHARO})
	router.Register(&CommandDef{Name: "SCRIPT", Handler: cmdSCRIPT})
}

// scriptGuard returns the ACL guard applied to commands invoked from inside a
// script, or nil for a connection without an ACL user (the permissive default
// user). It makes every key a script touches — declared in numkeys or not —
// subject to the user's key patterns; the keys named on the EVAL/EVALSHA line
// are already checked by enforceACL before the engine runs.
func scriptGuard(ctx *Context) ScriptCallGuard {
	user := ctx.ACLUser
	if user == nil {
		return nil
	}
	// denied records the refusal in ACL LOG with the "lua" context, as Redis
	// does for a command a script was not allowed to run.
	denied := func(reason, object string) {
		inScript := *ctx
		inScript.InScript = true
		recordACLDenial(&inScript, reason, object, user.Name)
	}
	return func(cmd string, args []string) error {
		upper := strings.ToUpper(cmd)
		shown := strings.ToLower(upper)
		// Command permission: a script runs as the invoking user, so a
		// command the user cannot run at the top level must not run from
		// inside a script either — redis.call('get', ...) without +get, or a
		// key-less redis.call('flushdb'), otherwise bypass the user's grants
		// entirely. The bypass set mirrors enforceACL: a client must always
		// be able to probe and authenticate.
		allowed := aclBypassCommands[upper] || user.CanExecuteCommand(upper)
		if hasSubcommands(upper) && len(args) > 0 {
			allowed = aclBypassCommands[upper] || user.CanExecuteSubcommand(upper, args[0])
			shown += "|" + strings.ToLower(args[0])
		}
		if !allowed {
			denied("command", shown)
			return fmt.Errorf("NOPERM script attempted to execute the '%s' command which is not allowed", upper)
		}
		for _, key := range aclKeysForCommand(upper, stringArgsToBytes(args)) {
			if !user.CanAccessKey(key) {
				denied("key", key)
				return fmt.Errorf("NOPERM script attempted to use a key which is not allowed")
			}
		}
		return nil
	}
}

func stringArgsToBytes(args []string) [][]byte {
	out := make([][]byte, len(args))
	for i, a := range args {
		out[i] = []byte(a)
	}
	return out
}

// scriptKeysAndArgs splits the arguments after the script/SHA of an EVAL,
// EVALSHA or FCALL call into keys and args, validating numkeys like Redis.
func scriptKeysAndArgs(ctx *Context) ([]string, []string, error) {
	if ctx.ArgCount() < 2 {
		return nil, nil, ErrWrongArgCount
	}
	numKeys, err := strconv.ParseInt(ctx.ArgString(1), 10, 64)
	if err != nil {
		return nil, nil, ErrNotInteger
	}
	if numKeys < 0 {
		return nil, nil, errors.New("ERR Number of keys can't be negative")
	}
	if numKeys > int64(ctx.ArgCount()-2) {
		return nil, nil, errors.New("ERR Number of keys can't be greater than number of args")
	}
	n := int(numKeys)
	keys := make([]string, n)
	for i := 0; i < n; i++ {
		keys[i] = ctx.ArgString(2 + i)
	}
	args := make([]string, 0, ctx.ArgCount()-2-n)
	for i := 2 + n; i < ctx.ArgCount(); i++ {
		args = append(args, ctx.ArgString(i))
	}
	return keys, args, nil
}

func cmdEVAL(ctx *Context) error {
	return evalScript(ctx, false)
}

func cmdEVALRO(ctx *Context) error {
	return evalScript(ctx, true)
}

func evalScript(ctx *Context, readOnly bool) error {
	keys, args, err := scriptKeysAndArgs(ctx)
	if err != nil {
		return ctx.WriteError(err)
	}
	engine := getScriptEngine(ctx.Store)
	return ctx.WriteValue(engine.runScript(ctx, ctx.ArgString(0), keys, args, readOnly))
}

func cmdEVALSHA(ctx *Context) error {
	return evalSHAScript(ctx, false)
}

func cmdEVALSHARO(ctx *Context) error {
	return evalSHAScript(ctx, true)
}

func evalSHAScript(ctx *Context, readOnly bool) error {
	keys, args, err := scriptKeysAndArgs(ctx)
	if err != nil {
		return ctx.WriteError(err)
	}
	engine := getScriptEngine(ctx.Store)
	script, ok := engine.lookupScript(ctx.ArgString(0))
	if !ok {
		return ctx.WriteError(errNoScript)
	}
	return ctx.WriteValue(engine.runScript(ctx, script, keys, args, readOnly))
}

var errNoScript = replyError("NOSCRIPT No matching script. Please use EVAL.")

func cmdSCRIPT(ctx *Context) error {
	if ctx.ArgCount() < 1 {
		return ctx.WriteError(ErrWrongArgCount)
	}

	engine := getScriptEngine(ctx.Store)

	subCmd := strings.ToUpper(ctx.ArgString(0))
	wrongArgs := func() error {
		return ctx.WriteError(errors.New("ERR wrong number of arguments for 'script|" + strings.ToLower(subCmd) + "' command"))
	}

	switch subCmd {
	case "LOAD":
		if ctx.ArgCount() != 2 {
			return wrongArgs()
		}
		script := ctx.ArgString(1)
		if msg := engine.compileScript(script); msg != "" {
			return ctx.WriteError(errors.New(msg))
		}
		return ctx.WriteBulkString(engine.ScriptLoad(script))

	case "EXISTS":
		if ctx.ArgCount() < 2 {
			return wrongArgs()
		}
		results := make([]*resp.Value, ctx.ArgCount()-1)
		for i := 1; i < ctx.ArgCount(); i++ {
			if _, ok := engine.lookupScript(ctx.ArgString(i)); ok {
				results[i-1] = resp.IntegerValue(1)
			} else {
				results[i-1] = resp.IntegerValue(0)
			}
		}
		return ctx.WriteArray(results)

	case "FLUSH":
		if ctx.ArgCount() > 2 {
			return wrongArgs()
		}
		if ctx.ArgCount() == 2 {
			if mode := strings.ToUpper(ctx.ArgString(1)); mode != "SYNC" && mode != "ASYNC" {
				return ctx.WriteError(errors.New("ERR SCRIPT FLUSH only support SYNC|ASYNC option"))
			}
		}
		engine.ScriptFlush()
		return ctx.WriteOK()

	case "DEBUG":
		if ctx.ArgCount() != 2 {
			return wrongArgs()
		}
		switch strings.ToUpper(ctx.ArgString(1)) {
		case "YES", "SYNC", "NO":
			return ctx.WriteOK()
		}
		return ctx.WriteError(errors.New("ERR Use SCRIPT DEBUG YES/SYNC/NO"))

	case "KILL":
		if ctx.ArgCount() != 1 {
			return wrongArgs()
		}
		return ctx.WriteError(replyError("NOTBUSY No scripts in execution right now."))

	default:
		return ctx.WriteError(replyError("ERR unknown subcommand '" + ctx.ArgString(0) + "'. Try SCRIPT HELP."))
	}
}

func writeLuaResult(ctx *Context, result interface{}) error {
	switch v := result.(type) {
	case nil:
		return ctx.WriteNull()
	case string:
		return ctx.WriteBulkString(v)
	case int:
		return ctx.WriteInteger(int64(v))
	case int64:
		return ctx.WriteInteger(v)
	case float64:
		if v == float64(int64(v)) {
			return ctx.WriteInteger(int64(v))
		}
		return ctx.WriteBulkString(strconv.FormatFloat(v, 'f', -1, 64))
	case bool:
		if v {
			return ctx.WriteInteger(1)
		}
		return ctx.WriteInteger(0)
	case []interface{}:
		values := make([]*resp.Value, 0, len(v))
		for _, item := range v {
			values = append(values, goValueToResp(item))
		}
		return ctx.WriteArray(values)
	default:
		return ctx.WriteBulkString("")
	}
}

func goValueToResp(v interface{}) *resp.Value {
	switch val := v.(type) {
	case nil:
		return resp.NullValue()
	case string:
		return resp.BulkString(val)
	case int:
		return resp.IntegerValue(int64(val))
	case int64:
		return resp.IntegerValue(val)
	case float64:
		if val == float64(int64(val)) {
			return resp.IntegerValue(int64(val))
		}
		return resp.BulkString(strconv.FormatFloat(val, 'f', -1, 64))
	case bool:
		if val {
			return resp.IntegerValue(1)
		}
		return resp.IntegerValue(0)
	case []interface{}:
		values := make([]*resp.Value, 0, len(val))
		for _, item := range val {
			values = append(values, goValueToResp(item))
		}
		return resp.ArrayValue(values)
	default:
		return resp.BulkString("")
	}
}
