package command

import "errors"

// errOOM is Redis' reply to a command that may grow memory while used memory
// is over maxmemory.
var errOOM = errors.New("OOM command not allowed when used memory > 'maxmemory'.") //nolint:staticcheck // Redis reply text, clients match it verbatim

// errExecAbortOOM is the reply to an EXEC refused for memory: Redis discards
// the transaction and reports why.
var errExecAbortOOM = errors.New("EXECABORT Transaction discarded because of: " + errOOM.Error()) //nolint:staticcheck // Redis reply text

var (
	readonlyCommands = make(map[string]bool)
	denyOOMCommands  = make(map[string]bool)
)

func init() {
	for name, spec := range redisCommandSpecs {
		for _, f := range spec.Flags {
			switch f {
			case "readonly":
				readonlyCommands[name] = true
			case "denyoom":
				denyOOMCommands[name] = true
			}
		}
	}
}

// reaccountWrittenKeys charges the memory the command's keys gained or lost
// in place. Store.Set accounts whole values only, so without this a list,
// hash, set, sorted set or stream grown by RPUSH, HSET, SADD, ZADD or XADD
// was never charged and maxmemory did not bound it. Every command Redis does
// not flag readonly is covered, including CacheStorm's own; a key that did
// not change costs a read-locked lookup.
func reaccountWrittenKeys(ctx *Context) {
	if ctx.Store == nil || readonlyCommands[ctx.Command] {
		return
	}
	ctx.Store.Reaccount(ctx.CommandKeys()...)
}

// overMemoryFor reports whether cmd must be refused because used memory is
// over maxmemory and eviction could not bring it back, as Redis refuses
// commands flagged denyoom before running them.
func overMemoryFor(ctx *Context, cmd string) bool {
	if ctx.Store == nil || !denyOOMCommands[cmd] {
		return false
	}
	return ctx.Store.CheckWriteMemory() != nil
}

// execDeniedForMemory reports whether an EXEC must be refused for memory: a
// transaction that queued any denyoom command is, like Redis does.
func execDeniedForMemory(ctx *Context) bool {
	if ctx.Store == nil || ctx.Transaction == nil {
		return false
	}
	for _, qc := range ctx.Transaction.GetQueued() {
		if denyOOMCommands[qc.cmd] {
			return ctx.Store.CheckWriteMemory() != nil
		}
	}
	return false
}
