package command

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/cachestorm/cachestorm/internal/resp"
)

func RegisterNamespaceCommands(router *Router) {
	router.Register(&CommandDef{Name: "NAMESPACE", Handler: cmdNAMESPACE})
	router.Register(&CommandDef{Name: "NAMESPACES", Handler: cmdNAMESPACES})
	router.Register(&CommandDef{Name: "NAMESPACEDEL", Handler: cmdNAMESPACEDEL})
	router.Register(&CommandDef{Name: "NAMESPACEINFO", Handler: cmdNAMESPACEINFO})
	router.Register(&CommandDef{Name: "SELECT", Handler: cmdSELECT})
}

func cmdNAMESPACE(ctx *Context) error {
	if ctx.ArgCount() != 1 {
		return ctx.WriteError(ErrWrongArgCount)
	}

	_ = ctx.ArgString(0)

	ctx.WriteOK()
	return nil
}

func cmdNAMESPACES(ctx *Context) error {
	if ctx.ArgCount() != 0 {
		return ctx.WriteError(ErrWrongArgCount)
	}

	nm := ctx.Store.GetNamespaceManager()
	if nm == nil {
		return ctx.WriteArray([]*resp.Value{resp.BulkString("default")})
	}

	names := nm.List()
	results := make([]*resp.Value, 0, len(names))
	for _, name := range names {
		results = append(results, resp.BulkString(name))
	}

	return ctx.WriteArray(results)
}

func cmdNAMESPACEDEL(ctx *Context) error {
	if ctx.ArgCount() != 1 {
		return ctx.WriteError(ErrWrongArgCount)
	}

	name := ctx.ArgString(0)

	nm := ctx.Store.GetNamespaceManager()
	if nm == nil {
		return ctx.WriteError(fmt.Errorf("namespace not found"))
	}

	err := nm.Delete(name)
	if err != nil {
		return ctx.WriteError(err)
	}

	return ctx.WriteOK()
}

func cmdNAMESPACEINFO(ctx *Context) error {
	if ctx.ArgCount() != 1 {
		return ctx.WriteError(ErrWrongArgCount)
	}

	name := ctx.ArgString(0)

	nm := ctx.Store.GetNamespaceManager()
	if nm == nil {
		return ctx.WriteBulkString("# Namespace\r\nname:default\r\nkeys:0\r\nmemory:0\r\n")
	}

	stats, err := nm.Stats(name)
	if err != nil {
		return ctx.WriteError(err)
	}

	var sb strings.Builder
	sb.WriteString("# Namespace\r\n")
	sb.WriteString(fmt.Sprintf("name:%s\r\n", stats["name"]))
	sb.WriteString(fmt.Sprintf("keys:%d\r\n", stats["keys"]))
	sb.WriteString(fmt.Sprintf("memory:%d\r\n", stats["memory"]))

	return ctx.WriteBulkString(sb.String())
}

func cmdSELECT(ctx *Context) error {
	if ctx.ArgCount() != 1 {
		return ctx.WriteError(ErrWrongArgCount)
	}

	index, err := strconv.Atoi(ctx.ArgString(0))
	if err != nil {
		return ctx.WriteError(ErrNotInteger)
	}

	nm := ctx.Store.GetNamespaceManager()
	if nm != nil {
		if index == 0 {
			nm.GetOrCreate("default")
		} else {
			nm.GetOrCreate(fmt.Sprintf("db%d", index))
		}
	}

	return ctx.WriteOK()
}
