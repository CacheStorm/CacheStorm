package command

import (
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

	tags := make([]*resp.Value, 0, len(entry.Tags))
	for _, tag := range entry.Tags {
		tags = append(tags, resp.BulkString(tag))
	}

	return ctx.WriteArray(tags)
}

func cmdADDTAG(ctx *Context) error {
	if ctx.ArgCount() < 2 {
		return ctx.WriteError(ErrWrongArgCount)
	}

	key := ctx.ArgString(0)
	entry, exists := ctx.Store.Get(key)
	if !exists {
		return ctx.WriteInteger(0)
	}

	existingTags := make(map[string]struct{})
	for _, t := range entry.Tags {
		existingTags[t] = struct{}{}
	}

	added := 0
	for i := 1; i < ctx.ArgCount(); i++ {
		tag := ctx.ArgString(i)
		if _, exists := existingTags[tag]; !exists {
			entry.Tags = append(entry.Tags, tag)
			added++
		}
	}

	return ctx.WriteInteger(int64(added))
}

func cmdREMTAG(ctx *Context) error {
	if ctx.ArgCount() < 2 {
		return ctx.WriteError(ErrWrongArgCount)
	}

	key := ctx.ArgString(0)
	entry, exists := ctx.Store.Get(key)
	if !exists {
		return ctx.WriteInteger(0)
	}

	removeTags := make(map[string]struct{})
	for i := 1; i < ctx.ArgCount(); i++ {
		removeTags[ctx.ArgString(i)] = struct{}{}
	}

	newTags := make([]string, 0)
	removed := 0
	for _, tag := range entry.Tags {
		if _, shouldRemove := removeTags[tag]; shouldRemove {
			removed++
		} else {
			newTags = append(newTags, tag)
		}
	}
	entry.Tags = newTags

	return ctx.WriteInteger(int64(removed))
}

func cmdINVALIDATE(ctx *Context) error {
	if ctx.ArgCount() < 1 {
		return ctx.WriteError(ErrWrongArgCount)
	}

	tag := ctx.ArgString(0)
	cascade := false
	if ctx.ArgCount() >= 2 {
		if ctx.ArgString(1) == "CASCADE" {
			cascade = true
		}
	}

	var keysToDelete []string

	if cascade {
		allKeys := ctx.Store.GetTagIndex().InvalidateCascade(tag)
		for _, keys := range allKeys {
			keysToDelete = append(keysToDelete, keys...)
		}
	} else {
		keysToDelete = ctx.Store.GetTagIndex().Invalidate(tag)
	}

	deleted := 0
	uniqueKeys := make(map[string]struct{})
	for _, key := range keysToDelete {
		uniqueKeys[key] = struct{}{}
	}

	for key := range uniqueKeys {
		if ctx.Store.Delete(key) {
			deleted++
		}
	}

	return ctx.WriteInteger(int64(deleted))
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
