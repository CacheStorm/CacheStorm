package command

import (
	"errors"
	"sort"
	"strings"

	"github.com/cachestorm/cachestorm/internal/acl"
	"github.com/cachestorm/cachestorm/internal/resp"
	"github.com/cachestorm/cachestorm/internal/store"
)

// commandCategories returns the ACL categories of cmd, or of one of its
// subcommands when sub is not empty, without the leading "@".
func commandCategories(cmd, sub string) []string {
	if sub != "" {
		if cats, ok := redisSubcommandCategories[cmd+"|"+strings.ToUpper(sub)]; ok {
			return cats
		}
	}
	if spec, ok := redisCommandSpecs[cmd]; ok {
		return spec.Categories
	}
	return nil
}

// commandInfoValue is one COMMAND / COMMAND INFO entry in Redis 7's shape:
// name, arity, flags, first key, last key, key step, ACL categories, tips,
// key specifications and subcommands. Redis' own commands are served from
// its COMMAND reply (redis_command_data.go); CacheStorm's extension commands
// get an entry built from their registration.
func commandInfoValue(name string, def *CommandDef) *resp.Value {
	if m := redisCommandMeta(name); m != nil {
		return m.info
	}
	arity := -1
	var flags []string
	if def != nil {
		if def.Arity != 0 {
			arity = def.Arity
		}
		flags = def.Flags
	}
	first, last, step := 0, 0, 0
	cats := []*resp.Value{}
	flagVals := make([]*resp.Value, 0, len(flags))
	keyed, fast := false, false
	for _, f := range flags {
		flagVals = append(flagVals, resp.SimpleString(f))
		switch f {
		case "write":
			keyed = true
			cats = append(cats, resp.SimpleString("@write"))
		case "readonly":
			keyed = true
			cats = append(cats, resp.SimpleString("@read"))
		case "fast":
			fast = true
		}
	}
	if fast {
		cats = append(cats, resp.SimpleString("@fast"))
	} else {
		cats = append(cats, resp.SimpleString("@slow"))
	}
	if keyed && !commandDefHasFlag(def, "movablekeys") {
		first, last, step = 1, 1, 1
	}
	return resp.ArrayValue([]*resp.Value{
		resp.BulkString(strings.ToLower(name)),
		resp.IntegerValue(int64(arity)),
		resp.SetValue(flagVals),
		resp.IntegerValue(int64(first)),
		resp.IntegerValue(int64(last)),
		resp.IntegerValue(int64(step)),
		resp.SetValue(cats),
		resp.SetValue([]*resp.Value{}),
		resp.SetValue([]*resp.Value{}),
		resp.SetValue([]*resp.Value{}),
	})
}

func commandDefHasFlag(def *CommandDef, flag string) bool {
	if def == nil {
		return false
	}
	for _, f := range def.Flags {
		if f == flag {
			return true
		}
	}
	return false
}

// extensionDocsValue is the COMMAND DOCS entry of a CacheStorm extension
// command, shaped like the one Redis gives a module command.
func extensionDocsValue() *resp.Value {
	return resp.PairsValue([]*resp.Value{
		resp.BulkString("summary"), resp.BulkString("CacheStorm extension command."),
		resp.BulkString("since"), resp.BulkString("0.1.0"),
		resp.BulkString("group"), resp.BulkString("module"),
		resp.BulkString("module"), resp.BulkString("cachestorm"),
	})
}

// resolveCommandName finds a registered command or "container|subcommand"
// by name, as COMMAND INFO and COMMAND DOCS look names up. meta is nil for
// an extension command; ok is false when there is no such command.
func resolveCommandName(r *Router, name string) (meta *commandMeta, def *CommandDef, ok bool) {
	base, sub, isSub := strings.Cut(name, "|")
	def, ok = r.Get(CanonicalName(base))
	if !ok {
		return nil, nil, false
	}
	if isSub {
		meta = redisCommandMeta(base)
		if meta == nil || meta.subs == nil {
			return nil, nil, false
		}
		meta = meta.subs[strings.ToLower(sub)]
		return meta, def, meta != nil
	}
	return redisCommandMeta(base), def, true
}

func registeredCommandNames(ctx *Context) []string {
	r := ctx.dispatcher()
	if r == nil {
		return nil
	}
	cmds := r.Commands()
	names := make([]string, 0, len(cmds))
	for name := range cmds {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}

// cmdCommandList implements COMMAND LIST [FILTERBY MODULE name | ACLCAT
// category | PATTERN pattern].
func cmdCommandList(ctx *Context) error {
	var filter func(string) bool
	switch {
	case ctx.ArgCount() == 1:
	case ctx.ArgCount() == 4 && strings.EqualFold(ctx.ArgString(1), "FILTERBY"):
		arg := ctx.ArgString(3)
		switch strings.ToUpper(ctx.ArgString(2)) {
		case "MODULE":
			filter = func(string) bool { return false }
		case "ACLCAT":
			cat := strings.ToLower(strings.TrimPrefix(arg, "@"))
			filter = func(name string) bool {
				for _, c := range commandCategories(name, "") {
					if c == cat {
						return true
					}
				}
				return false
			}
		case "PATTERN":
			pattern := strings.ToLower(arg)
			filter = func(name string) bool { return store.MatchPattern(strings.ToLower(name), pattern) }
		default:
			return ctx.WriteError(ErrSyntaxError)
		}
	default:
		return ctx.WriteError(ErrSyntaxError)
	}
	out := []*resp.Value{}
	for _, name := range registeredCommandNames(ctx) {
		if filter == nil || filter(name) {
			out = append(out, resp.BulkString(strings.ToLower(name)))
		}
	}
	return ctx.WriteArray(out)
}

// cmdCommandInfo implements COMMAND INFO [name ...]; without names it
// describes every command, as COMMAND does.
func cmdCommandInfo(ctx *Context, names []string) error {
	r := ctx.dispatcher()
	if r == nil {
		return ctx.WriteError(errors.New("ERR COMMAND is not available in this context"))
	}
	if names == nil {
		names = registeredCommandNames(ctx)
	}
	out := make([]*resp.Value, 0, len(names))
	for _, n := range names {
		meta, def, ok := resolveCommandName(r, n)
		switch {
		case !ok:
			out = append(out, resp.NullBulkString())
		case meta != nil:
			out = append(out, meta.info)
		default:
			out = append(out, commandInfoValue(def.Name, def))
		}
	}
	return ctx.WriteArray(out)
}

// cmdCommandDocs implements COMMAND DOCS [name ...]: a map of command name to
// its documentation; names that are not commands are left out.
func cmdCommandDocs(ctx *Context) error {
	r := ctx.dispatcher()
	if r == nil {
		return ctx.WriteError(errors.New("ERR COMMAND is not available in this context"))
	}
	var names []string
	if ctx.ArgCount() == 1 {
		names = registeredCommandNames(ctx)
	} else {
		for i := 1; i < ctx.ArgCount(); i++ {
			names = append(names, ctx.ArgString(i))
		}
	}
	pairs := make([]*resp.Value, 0, 2*len(names))
	for _, n := range names {
		meta, def, ok := resolveCommandName(r, n)
		if !ok {
			continue
		}
		switch {
		case meta != nil && meta.docs != nil:
			pairs = append(pairs, resp.BulkString(meta.name), meta.docs)
		case meta != nil:
			pairs = append(pairs, resp.BulkString(meta.name), resp.PairsValue([]*resp.Value{}))
		default:
			pairs = append(pairs, resp.BulkString(strings.ToLower(def.Name)), extensionDocsValue())
		}
	}
	return ctx.WriteValue(resp.PairsValue(pairs))
}

// cmdCommandGetKeys implements COMMAND GETKEYS and, withFlags, COMMAND
// GETKEYSANDFLAGS: the keys of a full command line, found with the
// command's key specs as Redis does.
func cmdCommandGetKeys(ctx *Context, withFlags bool) error {
	argv := make([]string, 0, ctx.ArgCount()-1)
	for i := 1; i < ctx.ArgCount(); i++ {
		argv = append(argv, ctx.ArgString(i))
	}
	r := ctx.dispatcher()
	var def *CommandDef
	ok := false
	if r != nil {
		def, ok = r.Get(CanonicalName(argv[0]))
	}
	if !ok {
		return ctx.WriteError(errors.New("ERR Invalid command specified"))
	}
	meta := redisCommandMeta(argv[0])
	if meta != nil {
		meta = lookupRedisCommandMeta(argv)
		if meta == nil {
			return ctx.WriteError(errors.New("ERR Invalid command specified"))
		}
	}
	var keys []commandKeyRef
	if meta != nil {
		if !commandHasKeys(meta) {
			return ctx.WriteError(errors.New("ERR The command has no key arguments"))
		}
		if !arityFits(meta.arity, len(argv)) {
			return ctx.WriteError(errors.New("ERR Invalid number of arguments specified for command"))
		}
		var specOK bool
		keys, specOK = keysFromSpecs(meta, argv)
		if !specOK {
			keys = fallbackCommandKeys(meta, argv)
		}
		if len(keys) == 0 && !meta.hasFlag("no_mandatory_keys") {
			return ctx.WriteError(errors.New("ERR Invalid arguments specified for command"))
		}
	} else {
		if def.Arity != 0 && !arityFits(def.Arity, len(argv)) {
			return ctx.WriteError(errors.New("ERR Invalid number of arguments specified for command"))
		}
		for _, k := range CommandKeys(argv[0], ctx.Args[2:]) {
			keys = append(keys, commandKeyRef{name: k, flags: []string{"RW", "access", "update"}})
		}
		if len(keys) == 0 {
			return ctx.WriteError(errors.New("ERR The command has no key arguments"))
		}
	}
	out := make([]*resp.Value, 0, len(keys))
	for _, k := range keys {
		if !withFlags {
			out = append(out, resp.BulkString(k.name))
			continue
		}
		fl := make([]*resp.Value, len(k.flags))
		for i, f := range k.flags {
			fl[i] = resp.SimpleString(f)
		}
		out = append(out, resp.ArrayValue([]*resp.Value{resp.BulkString(k.name), resp.SetValue(fl)}))
	}
	return ctx.WriteArray(out)
}

// arityFits reports whether n arguments, the command name included, fit a
// Redis arity: exact when positive, a minimum when negative.
func arityFits(arity, n int) bool {
	if arity >= 0 {
		return n == arity
	}
	return n >= -arity
}

// fallbackCommandKeys finds the keys of a command whose key specs cannot be
// applied to argv, as Redis' getkeys procedures do: SET and BITFIELD flag
// their key by the options given, and the other movable-key commands use
// CacheStorm's key extraction with the flags of the matching key spec.
func fallbackCommandKeys(meta *commandMeta, argv []string) []commandKeyRef {
	switch meta.name {
	case "set":
		for _, a := range argv[3:] {
			if strings.EqualFold(a, "GET") {
				return []commandKeyRef{{name: argv[1], flags: []string{"RW", "access", "update"}}}
			}
		}
		return []commandKeyRef{{name: argv[1], flags: []string{"OW", "update"}}}
	case "bitfield":
		readonly := true
	loop:
		for i := 2; i < len(argv); i++ {
			rem := len(argv) - i - 1
			switch op := strings.ToLower(argv[i]); {
			case op == "get" && rem >= 2:
				i += 2
			case (op == "set" || op == "incrby") && rem >= 3:
				readonly = false
				break loop
			case op == "overflow" && rem >= 1:
				i++
			default:
				readonly = false
				break loop
			}
		}
		if readonly {
			return []commandKeyRef{{name: argv[1], flags: []string{"RO", "access"}}}
		}
		return []commandKeyRef{{name: argv[1], flags: []string{"RW", "access", "update"}}}
	}
	args := make([][]byte, len(argv)-1)
	for i, a := range argv[1:] {
		args[i] = []byte(a)
	}
	var names []string
	switch meta.name {
	case "sort", "sort_ro":
		names = append([]string{argv[1]}, storeKeyAfter(args[1:])...)
	case "georadius", "georadiusbymember":
		names = []string{argv[1]}
		if stores := storeKeyAfter(args[1:]); len(stores) > 0 {
			names = append(names, stores[len(stores)-1])
		}
	default:
		names = CommandKeys(argv[0], args)
	}
	// The first key takes the flags of the first key spec, later ones those
	// of the last: the destination of SORT and GEORADIUS STORE, and the
	// same flags for every key of XREAD and MIGRATE.
	var firstFlags, restFlags []string
	for i := range meta.keySpecs {
		if meta.keySpecs[i].hasFlag("not_key") {
			continue
		}
		fl := withoutFlag(meta.keySpecs[i].flags, "incomplete")
		if firstFlags == nil {
			firstFlags = fl
		}
		restFlags = fl
	}
	keys := make([]commandKeyRef, 0, len(names))
	for i, n := range names {
		flags := restFlags
		if i == 0 {
			flags = firstFlags
		}
		keys = append(keys, commandKeyRef{name: n, flags: flags})
	}
	return keys
}

func withoutFlag(flags []string, drop string) []string {
	out := make([]string, 0, len(flags))
	for _, f := range flags {
		if f != drop {
			out = append(out, f)
		}
	}
	return out
}

const commandHelp = `COMMAND <subcommand> [<arg> [value] [opt] ...]. Subcommands are:
(no subcommand)
    Return details about all Redis commands.
COUNT
    Return the total number of commands in this Redis server.
LIST
    Return a list of all commands in this Redis server.
INFO [<command-name> ...]
    Return details about multiple Redis commands.
    If no command names are given, documentation details for all
    commands are returned.
DOCS [<command-name> ...]
    Return documentation details about multiple Redis commands.
    If no command names are given, documentation details for all
    commands are returned.
GETKEYS <full-command>
    Return the keys from a full Redis command.
GETKEYSANDFLAGS <full-command>
    Return the keys and the access flags from a full Redis command.
HELP
    Print this help.`

// checkRedisSubcommand answers a container command's subcommand the way
// Redis does before running it: an unknown subcommand and a wrong argument
// count are errors. It reports whether the subcommand may run; extension
// subcommands Redis does not have are left to the handler.
func checkRedisSubcommand(ctx *Context, container string, known func(sub string) bool) bool {
	sub := ctx.ArgString(0)
	m := redisCommandMeta(container)
	var sm *commandMeta
	if m != nil {
		sm = m.subs[strings.ToLower(sub)]
	}
	if sm == nil {
		if known != nil && known(strings.ToUpper(sub)) {
			return true
		}
		_ = ctx.WriteError(errUnknownSubcommand(sub, strings.ToUpper(container)))
		return false
	}
	if !arityFits(sm.arity, ctx.ArgCount()+1) {
		_ = ctx.WriteError(arityError(sm.name))
		return false
	}
	return true
}

// aclCategoryMembers maps each ACL category to its commands and "CMD|SUB"
// subcommands, from Redis' command table.
var aclCategoryMembers = func() map[string][]string {
	m := make(map[string][]string)
	for name, spec := range redisCommandSpecs {
		for _, c := range spec.Categories {
			m[c] = append(m[c], name)
		}
	}
	for name, cats := range redisSubcommandCategories {
		for _, c := range cats {
			m[c] = append(m[c], name)
		}
	}
	for c := range m {
		sort.Strings(m[c])
	}
	return m
}()

// hasSubcommands reports whether cmd is a container command (CONFIG,
// CLIENT, ...) whose ACL rules may name its subcommands.
func hasSubcommands(cmd string) bool {
	_, ok := containerCommands[cmd]
	return ok
}

var containerCommands = func() map[string]struct{} {
	m := make(map[string]struct{})
	for name := range redisSubcommandCategories {
		if i := strings.IndexByte(name, '|'); i > 0 {
			m[name[:i]] = struct{}{}
		}
	}
	return m
}()

func init() {
	acl.SetCategoryResolver(func(category string) ([]string, bool) {
		members, ok := aclCategoryMembers[category]
		return members, ok
	})
}

// cmdACLCat implements ACL CAT [category].
func cmdACLCat(ctx *Context) error {
	if ctx.ArgCount() == 1 {
		cats := make([]string, 0, len(aclCategoryMembers))
		for c := range aclCategoryMembers {
			cats = append(cats, c)
		}
		sort.Strings(cats)
		out := make([]*resp.Value, len(cats))
		for i, c := range cats {
			out[i] = resp.BulkString(c)
		}
		return ctx.WriteArray(out)
	}
	if ctx.ArgCount() != 2 {
		return ctx.WriteError(errSubcommandSyntax(ctx.ArgString(0), "ACL"))
	}
	members, ok := aclCategoryMembers[strings.ToLower(strings.TrimPrefix(ctx.ArgString(1), "@"))]
	if !ok {
		return ctx.WriteError(errors.New("ERR Unknown category '" + ctx.ArgString(1) + "'"))
	}
	out := make([]*resp.Value, len(members))
	for i, m := range members {
		out[i] = resp.BulkString(strings.ToLower(m))
	}
	return ctx.WriteArray(out)
}
