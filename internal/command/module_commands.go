package command

import (
	"errors"
	"strconv"
	"strings"

	"github.com/cachestorm/cachestorm/internal/module"
	"github.com/cachestorm/cachestorm/internal/resp"
)

func cmdMODULE(ctx *Context) error {
	if ctx.ArgCount() < 1 {
		return ctx.WriteError(ErrWrongArgCount)
	}

	if !checkRedisSubcommand(ctx, "module", nil) {
		return nil
	}
	registry := module.GetRegistry()

	switch strings.ToUpper(ctx.ArgString(0)) {
	case "LIST":
		return handleModuleList(ctx, registry)
	case "LOAD":
		return handleModuleLoad(ctx, registry)
	case "UNLOAD":
		return handleModuleUnload(ctx, registry)
	case "HELP":
		return ctx.WriteArray(helpReply(moduleHelp))
	default:
		return ctx.WriteError(errors.New("ERR MODULE command not allowed. If the enable-module-command option is set to \"local\", you can run it from a local connection, otherwise you need to set this option in the configuration file, and then restart the server.")) //nolint:staticcheck // Redis reply text
	}
}

// bundledModule is a module Redis 8 ships whose commands and data types
// CacheStorm implements.
type bundledModule struct {
	name                   string
	ver                    int64
	usedBy, using, options string
}

// bundledModules are named, as Redis 8.2.11 names them, by MODULE LIST, HELLO
// and INFO modules, so that clients which look for a module before using
// its commands (search for RedisVL and Redis OM, ReJSON for JSON documents)
// find it.
var bundledModules = []bundledModule{
	{"search", 80213, "[]", "[ReJSON]", "[handle-io-errors]"},
	{"vectorset", 1, "[]", "[]", "[handle-io-errors|handle-repl-async-load]"},
	{"timeseries", 80214, "[]", "[]", "[handle-io-errors]"},
	{"bf", 80216, "[]", "[]", "[handle-io-errors]"},
	{"ReJSON", 80209, "[search]", "[]", "[handle-io-errors]"},
}

// moduleList is the reply of MODULE LIST and HELLO's modules: a map of
// name, ver, path and args per module.
func moduleList() []*resp.Value {
	entry := func(name string, ver int64) *resp.Value {
		return resp.PairsValue([]*resp.Value{
			resp.BulkString("name"), resp.BulkString(name),
			resp.BulkString("ver"), resp.IntegerValue(ver),
			resp.BulkString("path"), resp.BulkString(""),
			resp.BulkString("args"), resp.ArrayValue([]*resp.Value{}),
		})
	}
	out := make([]*resp.Value, 0, len(bundledModules))
	for _, m := range bundledModules {
		out = append(out, entry(m.name, m.ver))
	}
	for _, m := range module.GetRegistry().ListModules() {
		ver := moduleVersion(m.Version)
		out = append(out, entry(m.Name, ver))
	}
	return out
}

// moduleInfoLines are INFO's module lines.
func moduleInfoLines(w *infoWriter) {
	for _, m := range bundledModules {
		w.field("module", "name="+m.name+",ver="+strconv.FormatInt(m.ver, 10)+",api=1,filters=0,usedby="+m.usedBy+",using="+m.using+",options="+m.options)
	}
	for _, m := range module.GetRegistry().ListModules() {
		ver := moduleVersion(m.Version)
		w.field("module", "name="+m.Name+",ver="+strconv.FormatInt(ver, 10)+",api=1,filters=0,usedby=[],using=[],options=[]")
	}
}

// moduleVersion reads a registered module's version as the integer Redis
// reports; one that is not a number reports 0.
func moduleVersion(v string) int64 {
	n, err := strconv.ParseInt(v, 10, 64)
	if err != nil {
		return 0
	}
	return n
}

func handleModuleList(ctx *Context, _ *module.Registry) error {
	return ctx.WriteArray(moduleList())
}

func handleModuleLoad(ctx *Context, registry *module.Registry) error {
	if ctx.ArgCount() < 2 || ctx.ArgCount()%2 != 0 {
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

func RegisterModuleCommands(router *Router) {
	router.Register(&CommandDef{Name: "MODULE", Handler: cmdMODULE})
}

func RegisterModule(m module.Module) error {
	return module.GetRegistry().Register(m)
}
