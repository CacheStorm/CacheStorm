package command

import (
	"encoding/binary"
	"errors"
	"math/bits"
	"strings"
	"time"

	"github.com/cachestorm/cachestorm/internal/resp"
	"github.com/cachestorm/cachestorm/internal/store"
	"github.com/cachestorm/cachestorm/internal/xxh3"
)

// idmpAutoIID is the iid XADD IDMPAUTO derives from an entry's fields, as
// Redis' createIdempotencyHash: each field and value pair hashed with
// XXH3-128 over field, the field's length as a little-endian int64 and
// value, the pair hashes combined so their order does not matter, and the
// 16 bytes laid out low half first, as Redis stores them on x86 and ARM.
func idmpAutoIID(pairs []store.StreamField) string {
	var sumLo, sumHi, xorLo, xorHi uint64
	var buf []byte
	for _, p := range pairs {
		buf = append(buf[:0], p.Name...)
		buf = binary.LittleEndian.AppendUint64(buf, uint64(len(p.Name)))
		buf = append(buf, p.Value...)
		h := xxh3.Hash128(buf)
		sumLo += h.Lo
		sumHi += h.Hi
		xorLo ^= h.Lo
		xorHi ^= h.Hi
	}
	var out [16]byte
	binary.LittleEndian.PutUint64(out[:8], sumLo^bits.RotateLeft64(xorHi, 1))
	binary.LittleEndian.PutUint64(out[8:], sumHi^bits.RotateLeft64(xorLo, 1))
	return string(out[:])
}

// xaddIDMP is XADD with IDMP or IDMPAUTO. A duplicate answers with the ID
// the producer's first XADD got, and reaches replicas and the AOF with that
// ID instead of "*": a replica that still remembers the iid counts the
// duplicate, and one that has forgotten it refuses the old ID rather than
// adding the entry twice.
func xaddIDMP(ctx *Context, key string, idIdx int, stream *store.StreamValue, created bool, a *addTrimArgs, pairs []store.StreamField) error {
	req := store.IDMPRequest{PID: a.idmpPID, IID: a.idmpIID}
	if a.idmpAuto {
		req.IID = idmpAutoIID(pairs)
	}
	now := uint64(ctx.nowMS())
	id, trimmed, dup, err := stream.XAddIDMP(a.id, pairs, a.trim, now, req)
	if err != nil {
		return ctx.WriteError(err)
	}
	if created {
		v, installed, err := ctx.Store.GetOrCreate(key, func() store.Value { return stream })
		if err != nil {
			return ctx.WriteError(err)
		}
		if !installed {
			existing, ok := v.(*store.StreamValue)
			if !ok {
				return ctx.WriteError(store.ErrWrongType)
			}
			if id, trimmed, dup, err = existing.XAddIDMP(a.id, pairs, a.trim, now, req); err != nil {
				return ctx.WriteError(err)
			}
		}
	}
	ctx.Args[idIdx] = []byte(id.String())
	if dup {
		// Redis does not count a duplicate as changing the key for WATCH.
		ctx.writeEffectsHandled = true
		return ctx.WriteBulkString(id.String())
	}
	ctx.notify(store.NotifyStream, "xadd", key)
	if trimmed > 0 {
		ctx.notify(store.NotifyStream, "xtrim", key)
	}
	ctx.Store.KeyNotifier().NotifyKey(key)
	return ctx.WriteBulkString(id.String())
}

// cmdXCFGSET is Redis 8.6's XCFGSET key [IDMP-DURATION seconds]
// [IDMP-MAXSIZE entries].
func cmdXCFGSET(ctx *Context) error {
	if ctx.ArgCount() < 1 {
		return ctx.WriteError(arityError("xcfgset"))
	}
	stream, err := lookupStream(ctx, ctx.ArgString(0))
	if err != nil {
		return ctx.WriteError(err)
	}
	if stream == nil {
		return ctx.WriteError(errors.New("ERR no such key"))
	}
	duration, maxSize := int64(-1), int64(-1)
	for i := 1; i < ctx.ArgCount(); i++ {
		more := ctx.ArgCount() - 1 - i
		opt := ctx.ArgString(i)
		switch {
		case strings.EqualFold(opt, "IDMP-DURATION") && more > 0:
			if duration != -1 {
				return ctx.WriteError(errors.New("ERR IDMP-DURATION specified multiple times"))
			}
			i++
			n, ok := parseStreamLL(ctx.ArgString(i))
			if !ok {
				return ctx.WriteError(ErrNotInteger)
			}
			if n < 1 || n > 86400 {
				return ctx.WriteError(errors.New("ERR IDMP-DURATION must be between 1 and 86400 seconds"))
			}
			duration = n
		case strings.EqualFold(opt, "IDMP-MAXSIZE") && more > 0:
			if maxSize != -1 {
				return ctx.WriteError(errors.New("ERR IDMP-MAXSIZE specified multiple times"))
			}
			i++
			n, ok := parseStreamLL(ctx.ArgString(i))
			if !ok {
				return ctx.WriteError(ErrNotInteger)
			}
			if n < 1 || n > 10000 {
				return ctx.WriteError(errors.New("ERR IDMP-MAXSIZE must be between 1 and 10000 entries"))
			}
			maxSize = n
		default:
			return ctx.WriteError(ErrSyntaxError)
		}
	}
	if duration == -1 && maxSize == -1 {
		return ctx.WriteError(errors.New("ERR At least one parameter must be specified"))
	}
	if !stream.SetIDMPConfig(duration, maxSize) {
		ctx.PropagateAs()
	}
	return ctx.WriteOK()
}

// cmdXIDMPRECORD is Redis 8.6's XIDMPRECORD key pid iid id: it records
// that producer pid added entry id for iid, as the AOF restores IDMP.
func cmdXIDMPRECORD(ctx *Context) error {
	if ctx.ArgCount() != 4 {
		return ctx.WriteError(arityError("xidmprecord"))
	}
	id, err := parseStrictStreamID(ctx.ArgString(3))
	if err != nil {
		return ctx.WriteError(err)
	}
	pid, iid := ctx.ArgString(1), ctx.ArgString(2)
	if pid == "" {
		return ctx.WriteError(errors.New("ERR producer ID must be non-empty"))
	}
	if iid == "" {
		return ctx.WriteError(errors.New("ERR idempotent ID must be non-empty"))
	}
	stream, err := lookupStream(ctx, ctx.ArgString(0))
	if err != nil {
		return ctx.WriteError(err)
	}
	if stream == nil {
		return ctx.WriteError(errors.New("ERR no such key"))
	}
	changed, err := stream.IDMPRecord(pid, iid, id, ctx.nowMS())
	if err != nil {
		return ctx.WriteError(err)
	}
	if !changed {
		ctx.PropagateAs()
	}
	return ctx.WriteOK()
}

// idmpInfoFields is what XINFO STREAM shows of IDMP, after
// recorded-first-entry-id.
func idmpInfoFields(stream *store.StreamValue) []*resp.Value {
	i := stream.IDMPInfoAt(time.Now().UnixMilli())
	return []*resp.Value{
		resp.BulkString("idmp-duration"), resp.IntegerValue(i.Duration),
		resp.BulkString("idmp-maxsize"), resp.IntegerValue(i.MaxSize),
		resp.BulkString("pids-tracked"), resp.IntegerValue(i.Producers),
		resp.BulkString("iids-tracked"), resp.IntegerValue(i.IIDs),
		resp.BulkString("iids-added"), resp.IntegerValue(i.Added),
		resp.BulkString("iids-duplicates"), resp.IntegerValue(i.Duplicates),
	}
}
