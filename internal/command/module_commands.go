package command

import (
	"fmt"
	"strings"

	"github.com/cachestorm/cachestorm/internal/module"
	"github.com/cachestorm/cachestorm/internal/resp"
)

func cmdMODULE(ctx *Context) error {
	if ctx.ArgCount() < 1 {
		return ctx.WriteError(ErrWrongArgCount)
	}

	subCmd := strings.ToUpper(ctx.ArgString(0))
	registry := module.GetRegistry()

	switch subCmd {
	case "LIST":
		return handleModuleList(ctx, registry)
	case "LOAD":
		return handleModuleLoad(ctx, registry)
	case "UNLOAD":
		return handleModuleUnload(ctx, registry)
	default:
		return ctx.WriteError(fmt.Errorf("ERR unknown subcommand '%s'", subCmd))
	}
}

func handleModuleList(ctx *Context, registry *module.Registry) error {
	modules := registry.ListModules()
	result := make([]*resp.Value, 0, len(modules))

	for _, m := range modules {
		result = append(result, resp.ArrayValue([]*resp.Value{
			resp.BulkString("name"),
			resp.BulkString(m.Name),
			resp.BulkString("version"),
			resp.BulkString(m.Version),
			resp.BulkString("loaded"),
			resp.IntegerValue(boolToInt(m.Loaded)),
		}))
	}

	return ctx.WriteArray(result)
}

func handleModuleLoad(ctx *Context, registry *module.Registry) error {
	if ctx.ArgCount() < 2 {
		return ctx.WriteError(ErrWrongArgCount)
	}

	name := ctx.ArgString(1)
	config := make(map[string]string)

	for i := 2; i+1 < ctx.ArgCount(); i += 2 {
		key := ctx.ArgString(i)
		value := ctx.ArgString(i + 1)
		config[key] = value
	}

	modCtx := module.NewContext(config)
	if err := registry.Load(name, modCtx); err != nil {
		return ctx.WriteError(err)
	}

	return ctx.WriteOK()
}

func handleModuleUnload(ctx *Context, registry *module.Registry) error {
	if ctx.ArgCount() < 2 {
		return ctx.WriteError(ErrWrongArgCount)
	}

	name := ctx.ArgString(1)
	if err := registry.Unload(name); err != nil {
		return ctx.WriteError(err)
	}

	return ctx.WriteOK()
}

func boolToInt(b bool) int64 {
	if b {
		return 1
	}
	return 0
}

func RegisterModuleCommands(router *Router) {
	router.Register(&CommandDef{Name: "MODULE", Handler: cmdMODULE})
}

func RegisterModule(m module.Module) error {
	return module.GetRegistry().Register(m)
}
