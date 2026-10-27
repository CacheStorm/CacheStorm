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
// key specifications and subcommands.
func commandInfoValue(name string) *resp.Value {
	spec, known := redisCommandSpecs[name]
	arity, first, last, step := -1, 0, 0, 0
	var flags, cats []string
	if known {
		arity, first, last, step = spec.Arity, spec.FirstKey, spec.LastKey, spec.KeyStep
		flags, cats = spec.Flags, spec.Categories
	}
	flagVals := make([]*resp.Value, len(flags))
	for i, f := range flags {
		flagVals[i] = resp.SimpleString(f)
	}
	catVals := make([]*resp.Value, len(cats))
	for i, c := range cats {
		catVals[i] = resp.SimpleString("@" + c)
	}
	var subs []*resp.Value
	prefix := name + "|"
	subNames := make([]string, 0)
	for sub := range redisSubcommandCategories {
		if strings.HasPrefix(sub, prefix) {
			subNames = append(subNames, sub)
		}
	}
	sort.Strings(subNames)
	for _, sub := range subNames {
		sc := make([]*resp.Value, 0, len(redisSubcommandCategories[sub]))
		for _, c := range redisSubcommandCategories[sub] {
			sc = append(sc, resp.SimpleString("@"+c))
		}
		subs = append(subs, resp.ArrayValue([]*resp.Value{
			resp.BulkString(strings.ToLower(sub)), resp.IntegerValue(-2), resp.ArrayValue([]*resp.Value{}),
			resp.IntegerValue(0), resp.IntegerValue(0), resp.IntegerValue(0),
			resp.ArrayValue(sc), resp.ArrayValue([]*resp.Value{}), resp.ArrayValue([]*resp.Value{}), resp.ArrayValue([]*resp.Value{}),
		}))
	}
	if subs == nil {
		subs = []*resp.Value{}
	}
	return resp.ArrayValue([]*resp.Value{
		resp.BulkString(strings.ToLower(name)),
		resp.IntegerValue(int64(arity)),
		resp.ArrayValue(flagVals),
		resp.IntegerValue(int64(first)),
		resp.IntegerValue(int64(last)),
		resp.IntegerValue(int64(step)),
		resp.ArrayValue(catVals),
		resp.ArrayValue([]*resp.Value{}),
		resp.ArrayValue([]*resp.Value{}),
		resp.ArrayValue(subs),
	})
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
		canon := CanonicalName(n)
		if _, ok := r.Get(canon); !ok {
			out = append(out, resp.NullArray())
			continue
		}
		out = append(out, commandInfoValue(canon))
	}
	return ctx.WriteArray(out)
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
		return ctx.WriteError(arityError("acl|cat"))
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
