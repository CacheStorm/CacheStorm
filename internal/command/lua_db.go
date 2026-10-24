package command

import (
	"fmt"

	lua "github.com/yuin/gopher-lua"
)

// scriptEngineFor returns an engine that runs one script in the caller's
// database. It shares nothing mutable with the process engine, so a SELECT
// inside the script switches only that script's database, as in Redis 7.
func scriptEngineFor(ctx *Context, _ *ScriptEngine) *ScriptEngine {
	return &ScriptEngine{
		scripts: map[string]string{},
		store:   ctx.Store,
		dbs:     ctx.databases(),
	}
}

// evalSHAFrom runs the script that global has cached under sha.
func (e *ScriptEngine) evalSHAFrom(global *ScriptEngine, sha string, keys, args []string, guard ScriptCallGuard) (interface{}, error) {
	global.mu.RLock()
	script, exists := global.scripts[sha]
	global.mu.RUnlock()
	if !exists {
		return nil, fmt.Errorf("NOSCRIPT No matching script. Please use EVAL")
	}
	return e.eval(script, keys, args, guard)
}

// scriptSelect is SELECT called from a script: it changes the database of the
// running script only.
func (e *ScriptEngine) scriptSelect(args []string) lua.LValue {
	if len(args) != 1 {
		return lua.LString("ERR wrong number of arguments for 'select' command")
	}
	index, ok := parseDBIndex(args[0])
	if !ok {
		return lua.LString(ErrNotInteger.Error())
	}
	if e.dbs == nil || !e.dbs.Valid(index) {
		return lua.LString(errDBIndexOutOfRange.Error())
	}
	e.store = e.dbs.DB(index)
	return lua.LString("OK")
}
