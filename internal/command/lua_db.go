package command

import (
	lua "github.com/cachestorm/cachestorm/third_party/gopherlua"
)

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
