package command

import (
	"github.com/cachestorm/cachestorm/internal/hashslot"
	"github.com/cachestorm/cachestorm/internal/resp"
	"github.com/cachestorm/cachestorm/internal/store"
)

func RegisterTagCommands(router *Router) {
	router.Register(&CommandDef{Name: "SETTAG", Handler: cmdSETTAG})
	router.Register(&CommandDef{Name: "TAGS", Handler: cmdTAGS})
	router.Register(&CommandDef{Name: "ADDTAG", Handler: cmdADDTAG})
	router.Register(&CommandDef{Name: "REMTAG", Handler: cmdREMTAG})
	router.Register(&CommandDef{Name: "INVALIDATE", Handler: cmdINVALIDATE})
	router.Register(&CommandDef{Name: "TAGKEYS", Handler: cmdTAGKEYS})
	router.Register(&CommandDef{Name: "TAGCOUNT", Handler: cmdTAGCOUNT})
	router.Register(&CommandDef{Name: "TAGLINK", Handler: cmdTAGLINK})
	router.Register(&CommandDef{Name: "TAGUNLINK", Handler: cmdTAGUNLINK})
	router.Register(&CommandDef{Name: "TAGCHILDREN", Handler: cmdTAGCHILDREN})
}

func cmdSETTAG(ctx *Context) error {
	if ctx.ArgCount() < 3 {
		return ctx.WriteError(ErrWrongArgCount)
	}

	key := ctx.ArgString(0)
	value := ctx.Arg(1)

	tags := make([]string, 0, ctx.ArgCount()-2)
	for i := 2; i < ctx.ArgCount(); i++ {
		tags = append(tags, ctx.ArgString(i))
	}

	opts := store.SetOptions{Tags: tags}
	err := ctx.Store.Set(key, &store.StringValue{Data: value}, opts)
	if err != nil {
		return ctx.WriteError(err)
	}

	return ctx.WriteOK()
}

func cmdTAGS(ctx *Context) error {
	if ctx.ArgCount() != 1 {
		return ctx.WriteError(ErrWrongArgCount)
	}

	key := ctx.ArgString(0)
	entry, exists := ctx.Store.Get(key)
	if !exists {
		return ctx.WriteArray([]*resp.Value{})
	}

	tags := make([]*resp.Value, 0, len(entry.TagList()))
	for _, tag := range entry.TagList() {
		tags = append(tags, resp.BulkString(tag))
	}

	return ctx.WriteArray(tags)
}

func cmdADDTAG(ctx *Context) error {
	if ctx.ArgCount() < 2 {
		return ctx.WriteError(ErrWrongArgCount)
	}
	added, _ := ctx.Store.AddKeyTags(ctx.ArgString(0), tagArgs(ctx))
	return ctx.WriteInteger(int64(added))
}

func cmdREMTAG(ctx *Context) error {
	if ctx.ArgCount() < 2 {
		return ctx.WriteError(ErrWrongArgCount)
	}
	removed, _ := ctx.Store.RemoveKeyTags(ctx.ArgString(0), tagArgs(ctx))
	return ctx.WriteInteger(int64(removed))
}

// tagArgs is the tags ADDTAG and REMTAG name after the key.
func tagArgs(ctx *Context) []string {
	tags := make([]string, ctx.ArgCount()-1)
	for i := range tags {
		tags[i] = ctx.ArgString(i + 1)
	}
	return tags
}

// InvalidateTags returns the tags INVALIDATE tag [tag ...] [CASCADE] with
// args reaches: the tags named, and with a final CASCADE every tag below
// them, each once. The write gate plans its hold from the same list.
func InvalidateTags(ti *store.TagIndex, args [][]byte) []string {
	cascade := len(args) > 1 && string(args[len(args)-1]) == "CASCADE"
	if cascade {
		args = args[:len(args)-1]
	}
	tags := make([]string, 0, len(args))
	seen := make(map[string]struct{}, len(args))
	add := func(tag string) {
		if _, dup := seen[tag]; !dup {
			seen[tag] = struct{}{}
			tags = append(tags, tag)
		}
	}
	for _, a := range args {
		add(string(a))
	}
	if cascade {
		for _, root := range tags[:len(tags):len(tags)] {
			for _, d := range ti.GetAllDescendants(root) {
				add(d)
			}
		}
	}
	return tags
}

func cmdINVALIDATE(ctx *Context) error {
	if ctx.ArgCount() < 1 {
		return ctx.WriteError(ErrWrongArgCount)
	}

	ti := ctx.Store.GetTagIndex()
	tags := InvalidateTags(ti, ctx.Args)
	// With KeyHeld the command holds only the stripes of the keys its tags
	// had when it was planned; a tag that has gained a key since makes it
	// take the whole gate for the rest.
	striped := ctx.KeyHeld != nil && ctx.Escalate != nil
	var keysToDelete []string
	for _, t := range tags {
		if striped {
			if keys, held := ti.InvalidateIf(t, ctx.KeyHeld); held {
				keysToDelete = append(keysToDelete, keys...)
				continue
			}
			ctx.Escalate()
			striped = false
		}
		keysToDelete = append(keysToDelete, ti.Invalidate(t)...)
	}

	if len(tags) > 1 {
		// A key with several of the tags is listed once per tag.
		uniqueKeys := make([]string, 0, len(keysToDelete))
		seen := make(map[string]struct{}, len(keysToDelete))
		for _, key := range keysToDelete {
			if _, exists := seen[key]; !exists {
				seen[key] = struct{}{}
				uniqueKeys = append(uniqueKeys, key)
			}
		}
		keysToDelete = uniqueKeys
	}
	deleted := ctx.Store.DeleteBatchKeys(keysToDelete)
	// Replicas, the AOF and a slot migration get the keys deleted, not the
	// tags: writes to other keys may have run beside a striped run, and a
	// slot migration copies only the writes naming keys of its slots.
	propagateDeletes(ctx, deleted)
	// Each key deleted is announced as DEL announces it.
	for _, key := range deleted {
		ctx.notify(store.NotifyGeneric, "del", key)
	}
	return ctx.WriteInteger(int64(len(deleted)))
}

func cmdTAGKEYS(ctx *Context) error {
	if ctx.ArgCount() != 1 {
		return ctx.WriteError(ErrWrongArgCount)
	}

	tag := ctx.ArgString(0)
	keys := ctx.Store.GetTagIndex().GetKeys(tag)

	results := make([]*resp.Value, 0, len(keys))
	for _, key := range keys {
		results = append(results, resp.BulkString(key))
	}

	return ctx.WriteArray(results)
}

func cmdTAGCOUNT(ctx *Context) error {
	if ctx.ArgCount() != 1 {
		return ctx.WriteError(ErrWrongArgCount)
	}

	tag := ctx.ArgString(0)
	count := ctx.Store.GetTagIndex().Count(tag)

	return ctx.WriteInteger(int64(count))
}

func cmdTAGLINK(ctx *Context) error {
	if ctx.ArgCount() != 2 {
		return ctx.WriteError(ErrWrongArgCount)
	}

	parent := ctx.ArgString(0)
	child := ctx.ArgString(1)

	ctx.Store.GetTagIndex().Link(parent, child)

	return ctx.WriteOK()
}

func cmdTAGUNLINK(ctx *Context) error {
	if ctx.ArgCount() != 2 {
		return ctx.WriteError(ErrWrongArgCount)
	}

	parent := ctx.ArgString(0)
	child := ctx.ArgString(1)

	ctx.Store.GetTagIndex().Unlink(parent, child)

	return ctx.WriteOK()
}

func cmdTAGCHILDREN(ctx *Context) error {
	if ctx.ArgCount() != 1 {
		return ctx.WriteError(ErrWrongArgCount)
	}

	tag := ctx.ArgString(0)
	children := ctx.Store.GetTagIndex().GetChildren(tag)

	results := make([]*resp.Value, 0, len(children))
	for _, child := range children {
		results = append(results, resp.BulkString(child))
	}

	return ctx.WriteArray(results)
}

// deleteChunk bounds the keys of one propagated DEL.
const deleteChunk = 1024

// propagateDeletes records the deletion of keys, in place of the command,
// as DELs: of at most deleteChunk keys, and in a cluster one per slot, as a
// slot migration copies only single-slot writes to the slots it moves.
func propagateDeletes(ctx *Context, keys []string) {
	if len(keys) == 0 {
		ctx.PropagateAs()
		return
	}
	groups := [][]string{keys}
	if ctx.ClusterEnabled() {
		bySlot := map[int][]string{}
		var order []int
		for _, k := range keys {
			slot := hashslot.Key(k)
			if _, seen := bySlot[slot]; !seen {
				order = append(order, slot)
			}
			bySlot[slot] = append(bySlot[slot], k)
		}
		groups = groups[:0]
		for _, slot := range order {
			groups = append(groups, bySlot[slot])
		}
	}
	var cmds []ExecutedCommand
	for _, group := range groups {
		for len(group) > 0 {
			n := min(len(group), deleteChunk)
			// One buffer holds the chunk's key names.
			size := 0
			for _, k := range group[:n] {
				size += len(k)
			}
			buf := make([]byte, 0, size)
			args := make([][]byte, n)
			for i, k := range group[:n] {
				start := len(buf)
				buf = append(buf, k...)
				args[i] = buf[start:len(buf):len(buf)]
			}
			cmds = append(cmds, ExecutedCommand{Name: "DEL", Args: args, DB: ctx.DB})
			group = group[n:]
		}
	}
	ctx.PropagateAs(cmds...)
}
