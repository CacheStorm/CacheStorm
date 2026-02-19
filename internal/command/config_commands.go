package command

import (
	"strings"

	"github.com/cachestorm/cachestorm/internal/resp"
)

func RegisterConfigCommands(router *Router) {
	router.Register(&CommandDef{Name: "CONFIG", Handler: cmdCONFIG})
}

func cmdCONFIG(ctx *Context) error {
	if ctx.ArgCount() < 1 {
		return ctx.WriteError(ErrWrongArgCount)
	}

	subCmd := strings.ToUpper(ctx.ArgString(0))

	switch subCmd {
	case "GET":
		return cmdConfigGet(ctx)
	case "SET":
		return cmdConfigSet(ctx)
	case "RESETSTAT":
		return cmdConfigResetStat(ctx)
	case "REWRITE":
		return ctx.WriteOK()
	default:
		return ctx.WriteError(ErrUnknownCommand)
	}
}

func cmdConfigGet(ctx *Context) error {
	if ctx.ArgCount() != 2 {
		return ctx.WriteError(ErrWrongArgCount)
	}

	param := strings.ToLower(ctx.ArgString(1))

	results := make([]*resp.Value, 0)

	switch {
	case param == "*" || param == "maxmemory":
		results = append(results,
			resp.BulkString("maxmemory"),
			resp.BulkString("0"),
		)
	case param == "*" || param == "maxmemory-policy":
		results = append(results,
			resp.BulkString("maxmemory-policy"),
			resp.BulkString("allkeys-lru"),
		)
	case param == "*" || param == "timeout":
		results = append(results,
			resp.BulkString("timeout"),
			resp.BulkString("0"),
		)
	case param == "*" || param == "databases":
		results = append(results,
			resp.BulkString("databases"),
			resp.BulkString("16"),
		)
	case param == "*" || param == "slowlog-log-slower-than":
		results = append(results,
			resp.BulkString("slowlog-log-slower-than"),
			resp.BulkString("10000"),
		)
	case param == "*" || param == "slowlog-max-len":
		results = append(results,
			resp.BulkString("slowlog-max-len"),
			resp.BulkString("1000"),
		)
	case param == "*" || param == "loglevel":
		results = append(results,
			resp.BulkString("loglevel"),
			resp.BulkString("info"),
		)
	case param == "*" || strings.HasPrefix(param, "save"):
		results = append(results,
			resp.BulkString("save"),
			resp.BulkString(""),
		)
	case param == "*" || param == "appendonly":
		results = append(results,
			resp.BulkString("appendonly"),
			resp.BulkString("no"),
		)
	case param == "*" || param == "appendfsync":
		results = append(results,
			resp.BulkString("appendfsync"),
			resp.BulkString("everysec"),
		)
	}

	return ctx.WriteArray(results)
}

func cmdConfigSet(ctx *Context) error {
	if ctx.ArgCount() != 3 {
		return ctx.WriteError(ErrWrongArgCount)
	}

	param := strings.ToLower(ctx.ArgString(1))
	_ = param

	return ctx.WriteOK()
}

func cmdConfigResetStat(ctx *Context) error {
	return ctx.WriteOK()
}
