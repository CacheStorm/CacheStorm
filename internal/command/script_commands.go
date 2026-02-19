package command

import (
	"strconv"
	"strings"

	"github.com/cachestorm/cachestorm/internal/resp"
)

var scriptEngine *ScriptEngine

func InitScriptEngine(s *ScriptEngine) {
	scriptEngine = s
}

func RegisterScriptCommands(router *Router) {
	router.Register(&CommandDef{Name: "EVAL", Handler: cmdEVAL})
	router.Register(&CommandDef{Name: "EVALSHA", Handler: cmdEVALSHA})
	router.Register(&CommandDef{Name: "SCRIPT", Handler: cmdSCRIPT})
}

func cmdEVAL(ctx *Context) error {
	if ctx.ArgCount() < 2 {
		return ctx.WriteError(ErrWrongArgCount)
	}

	if scriptEngine == nil {
		scriptEngine = NewScriptEngine(ctx.Store)
	}

	script := ctx.ArgString(0)
	numKeys, err := strconv.Atoi(ctx.ArgString(1))
	if err != nil {
		return ctx.WriteError(ErrNotInteger)
	}

	if ctx.ArgCount() < 2+numKeys {
		return ctx.WriteError(ErrWrongArgCount)
	}

	keys := make([]string, numKeys)
	for i := 0; i < numKeys; i++ {
		keys[i] = ctx.ArgString(2 + i)
	}

	args := make([]string, 0)
	for i := 2 + numKeys; i < ctx.ArgCount(); i++ {
		args = append(args, ctx.ArgString(i))
	}

	result, err := scriptEngine.Eval(script, keys, args)
	if err != nil {
		return ctx.WriteError(err)
	}

	return writeLuaResult(ctx, result)
}

func cmdEVALSHA(ctx *Context) error {
	if ctx.ArgCount() < 2 {
		return ctx.WriteError(ErrWrongArgCount)
	}

	if scriptEngine == nil {
		scriptEngine = NewScriptEngine(ctx.Store)
	}

	sha := ctx.ArgString(0)
	numKeys, err := strconv.Atoi(ctx.ArgString(1))
	if err != nil {
		return ctx.WriteError(ErrNotInteger)
	}

	if ctx.ArgCount() < 2+numKeys {
		return ctx.WriteError(ErrWrongArgCount)
	}

	keys := make([]string, numKeys)
	for i := 0; i < numKeys; i++ {
		keys[i] = ctx.ArgString(2 + i)
	}

	args := make([]string, 0)
	for i := 2 + numKeys; i < ctx.ArgCount(); i++ {
		args = append(args, ctx.ArgString(i))
	}

	result, err := scriptEngine.EvalSHA(sha, keys, args)
	if err != nil {
		return ctx.WriteError(err)
	}

	return writeLuaResult(ctx, result)
}

func cmdSCRIPT(ctx *Context) error {
	if ctx.ArgCount() < 1 {
		return ctx.WriteError(ErrWrongArgCount)
	}

	if scriptEngine == nil {
		scriptEngine = NewScriptEngine(ctx.Store)
	}

	subCmd := strings.ToUpper(ctx.ArgString(0))

	switch subCmd {
	case "LOAD":
		if ctx.ArgCount() < 2 {
			return ctx.WriteError(ErrWrongArgCount)
		}
		script := ctx.ArgString(1)
		sha := scriptEngine.ScriptLoad(script)
		return ctx.WriteBulkString(sha)

	case "EXISTS":
		if ctx.ArgCount() < 2 {
			return ctx.WriteError(ErrWrongArgCount)
		}
		results := make([]*resp.Value, ctx.ArgCount()-1)
		for i := 1; i < ctx.ArgCount(); i++ {
			sha := ctx.ArgString(i)
			if scriptEngine.ScriptExists(sha) {
				results[i-1] = resp.IntegerValue(1)
			} else {
				results[i-1] = resp.IntegerValue(0)
			}
		}
		return ctx.WriteArray(results)

	case "FLUSH":
		scriptEngine.ScriptFlush()
		return ctx.WriteOK()

	case "DEBUG":
		return ctx.WriteOK()

	case "KILL":
		return ctx.WriteOK()

	default:
		return ctx.WriteError(ErrUnknownCommand)
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
