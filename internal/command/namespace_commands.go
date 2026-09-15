package command

import (
	"strconv"
)

// RegisterNamespaceCommands registers SELECT, the Redis compatibility
// command. CacheStorm serves a single shared keyspace: SELECT answers OK
// like the single-db no-op that Redis-compatible single-keyspace servers
// provide, and the dead multi-namespace surface (NAMESPACE, NAMESPACES,
// NAMESPACEDEL, NAMESPACEINFO — a manager no production path could ever
// write data into) has been removed.
func RegisterNamespaceCommands(router *Router) {
	router.Register(&CommandDef{Name: "SELECT", Handler: cmdSELECT})
}

// cmdSELECT answers OK after validating the index argument. The server has
// one shared keyspace, so there is nothing to switch.
func cmdSELECT(ctx *Context) error {
	if ctx.ArgCount() != 1 {
		return ctx.WriteError(ErrWrongArgCount)
	}

	if _, err := strconv.Atoi(ctx.ArgString(0)); err != nil {
		return ctx.WriteError(ErrNotInteger)
	}

	return ctx.WriteOK()
}
