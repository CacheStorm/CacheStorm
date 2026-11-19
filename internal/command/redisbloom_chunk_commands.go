package command

import (
	"errors"

	"github.com/cachestorm/cachestorm/internal/resp"
	"github.com/cachestorm/cachestorm/internal/store"
)

// RedisBloom's chunked dump commands, which tools use to copy filters
// between servers, and its DEBUG and CF.COMPACT commands. Argument checks
// come in RedisBloom's order, with its reply texts.
func RegisterRedisBloomChunkCommands(router *Router) {
	router.Register(&CommandDef{Name: "BF.SCANDUMP", Handler: cmdBFSCANDUMP})
	router.Register(&CommandDef{Name: "BF.LOADCHUNK", Handler: cmdBFLOADCHUNK})
	router.Register(&CommandDef{Name: "BF.DEBUG", Handler: cmdBFDEBUG})
	router.Register(&CommandDef{Name: "CF.SCANDUMP", Handler: cmdCFSCANDUMP})
	router.Register(&CommandDef{Name: "CF.LOADCHUNK", Handler: cmdCFLOADCHUNK})
	router.Register(&CommandDef{Name: "CF.COMPACT", Handler: cmdCFCOMPACT})
	router.Register(&CommandDef{Name: "CF.DEBUG", Handler: cmdCFDEBUG})
}

//nolint:staticcheck // RedisBloom's reply texts
var (
	errScanDumpIter     = errors.New("Second argument must be numeric")
	errLoadChunkIter    = errors.New("ERR Second argument must be numeric")
	errCFPosition       = errors.New("Invalid position")
	errCFHeader         = errors.New("Invalid header")
	errCFCreate         = errors.New("Couldn't create filter!")
	errCFLoadChunk      = errors.New("Couldn't load chunk!")
	errCFCompactMissing = errors.New("Cuckoo filter was not found")
)

func chunkReply(ctx *Context, pos int64, data []byte, null bool) error {
	item := resp.BulkBytes(data)
	if data == nil {
		item = resp.BulkBytes([]byte{})
		if null {
			item = resp.NullBulkString()
		}
	}
	return ctx.WriteArray([]*resp.Value{resp.IntegerValue(pos), item})
}

func cmdBFSCANDUMP(ctx *Context) error {
	if ctx.ArgCount() != 2 {
		return ctx.WriteError(arityError(ctx.Command))
	}
	bf, exists, err := lookupValue[*store.BloomValue](ctx, ctx.ArgString(0))
	if err != nil {
		return ctx.WriteError(err)
	}
	if !exists {
		return ctx.WriteError(errBloomNotFound)
	}
	iter, ok := parseRedisInt(ctx.Arg(1))
	if !ok {
		return ctx.WriteError(errScanDumpIter)
	}
	if iter == 0 {
		return chunkReply(ctx, 1, bf.ScanDumpHeader(), false)
	}
	next, chunk := bf.ScanDumpChunk(iter, store.ScanDumpChunkMax)
	return chunkReply(ctx, next, chunk, false)
}

func cmdBFLOADCHUNK(ctx *Context) error {
	if ctx.ArgCount() != 3 {
		return ctx.WriteError(arityError(ctx.Command))
	}
	iter, ok := parseRedisInt(ctx.Arg(1))
	if !ok {
		return ctx.WriteError(errLoadChunkIter)
	}
	key := ctx.ArgString(0)
	bf, exists, err := lookupValue[*store.BloomValue](ctx, key)
	if err != nil {
		return ctx.WriteError(err)
	}
	if !exists {
		if iter != 1 {
			return ctx.WriteError(errBloomNotFound)
		}
		bf, err := store.BloomFromScanDumpHeader(ctx.Arg(2))
		if err != nil {
			return ctx.WriteError(err)
		}
		if err := ctx.Store.Set(key, bf, store.SetOptions{}); err != nil {
			return ctx.WriteError(err)
		}
		return ctx.WriteOK()
	}
	if err := bf.LoadChunk(iter, ctx.Arg(2)); err != nil {
		return ctx.WriteError(err)
	}
	valueWritten(ctx, key)
	return ctx.WriteOK()
}

func cmdBFDEBUG(ctx *Context) error {
	if ctx.ArgCount() != 1 {
		return ctx.WriteError(arityError(ctx.Command))
	}
	bf, exists, err := lookupValue[*store.BloomValue](ctx, ctx.ArgString(0))
	if err != nil {
		return ctx.WriteError(err)
	}
	if !exists {
		return ctx.WriteError(errBloomNotFound)
	}
	lines := bf.DebugLines()
	out := make([]*resp.Value, len(lines))
	for i, l := range lines {
		out[i] = resp.BulkString(l)
	}
	return ctx.WriteArray(out)
}

func cmdCFSCANDUMP(ctx *Context) error {
	if ctx.ArgCount() != 2 {
		return ctx.WriteError(arityError(ctx.Command))
	}
	pos, ok := parseRedisInt(ctx.Arg(1))
	if !ok || pos < 0 {
		return ctx.WriteError(errCFPosition)
	}
	cf, exists, err := lookupValue[*store.CuckooValue](ctx, ctx.ArgString(0))
	if err != nil {
		return ctx.WriteError(err)
	}
	if !exists {
		return ctx.WriteError(errBloomNotFound)
	}
	if cf.Items() == 0 {
		return chunkReply(ctx, 0, nil, true)
	}
	if pos == 0 {
		return chunkReply(ctx, 1, cf.ScanDumpHeader(), true)
	}
	next, chunk := cf.ScanDumpChunk(pos, store.ScanDumpChunkMax)
	return chunkReply(ctx, next, chunk, true)
}

func cmdCFLOADCHUNK(ctx *Context) error {
	if ctx.ArgCount() != 3 {
		return ctx.WriteError(arityError(ctx.Command))
	}
	key := ctx.ArgString(0)
	cf, exists, err := lookupValue[*store.CuckooValue](ctx, key)
	pos, ok := parseRedisInt(ctx.Arg(1))
	if !ok || pos == 0 {
		return ctx.WriteError(errCFPosition)
	}
	blob := ctx.Arg(2)
	if pos == 1 {
		switch {
		case err != nil:
			return ctx.WriteError(err)
		case exists:
			return ctx.WriteError(errBloomExists)
		case len(blob) != 38:
			return ctx.WriteError(errCFHeader)
		}
		cf := store.CuckooFromScanDumpHeader(blob)
		if cf == nil {
			return ctx.WriteError(errCFCreate)
		}
		if err := ctx.Store.Set(key, cf, store.SetOptions{}); err != nil {
			return ctx.WriteError(err)
		}
		return ctx.WriteOK()
	}
	if err != nil {
		return ctx.WriteError(err)
	}
	if !exists {
		return ctx.WriteError(errBloomNotFound)
	}
	if !cf.LoadChunk(pos, blob) {
		return ctx.WriteError(errCFLoadChunk)
	}
	valueWritten(ctx, key)
	return ctx.WriteOK()
}

func cmdCFCOMPACT(ctx *Context) error {
	if ctx.ArgCount() != 1 {
		return ctx.WriteError(arityError(ctx.Command))
	}
	key := ctx.ArgString(0)
	cf, exists, err := lookupValue[*store.CuckooValue](ctx, key)
	if err != nil || !exists {
		return ctx.WriteError(errCFCompactMissing)
	}
	cf.Compact()
	valueWritten(ctx, key)
	return ctx.WriteOK()
}

func cmdCFDEBUG(ctx *Context) error {
	if ctx.ArgCount() != 1 {
		return ctx.WriteError(arityError(ctx.Command))
	}
	cf, exists, err := lookupValue[*store.CuckooValue](ctx, ctx.ArgString(0))
	if err != nil {
		return ctx.WriteError(err)
	}
	if !exists {
		return ctx.WriteError(errBloomNotFound)
	}
	return ctx.WriteBulkString(cf.Debug())
}
