package command

import (
	"bytes"
	"errors"
	"strconv"
	"strings"

	"github.com/cachestorm/cachestorm/internal/store"
	"github.com/cachestorm/cachestorm/internal/xxh3"
)

// Redis 8.4's conditional string writes: SET's IFEQ, IFNE, IFDEQ and IFDNE
// options, DELEX, and DIGEST, which gives the XXH3 digest the IFD*
// conditions compare against.

const digestHexLen = 16

var errDigestLength = errors.New("ERR must be exactly 16 hexadecimal characters")

// stringDigest is Redis' stringDigest: the value's XXH3 hash as 16 lower
// case hex digits.
func stringDigest(b []byte) string {
	s := strconv.FormatUint(xxh3.Hash64(b), 16)
	return strings.Repeat("0", digestHexLen-len(s)) + s
}

// stringConditionHolds checks the IF* condition in flags against the
// string cur. A digest must be 16 characters long; Redis compares it
// ignoring case and does not check that it is hex.
func stringConditionHolds(flags int, cur, match []byte) (bool, error) {
	switch {
	case flags&strOptIFEQ != 0:
		return bytes.Equal(cur, match), nil
	case flags&strOptIFNE != 0:
		return !bytes.Equal(cur, match), nil
	}
	if len(match) != digestHexLen {
		return false, errDigestLength
	}
	same := strings.EqualFold(stringDigest(cur), string(match))
	if flags&strOptIFDEQ != 0 {
		return same, nil
	}
	return !same, nil
}

// cmdDIGEST is DIGEST key.
func cmdDIGEST(ctx *Context) error {
	if ctx.ArgCount() != 1 {
		return ctx.WriteError(arityError("digest"))
	}
	entry, ok := ctx.Store.Get(ctx.ArgString(0))
	if !ok {
		return ctx.WriteNullBulkString()
	}
	cur, isString := stringValueOf(entry.Value)
	if !isString {
		return ctx.WriteError(store.ErrWrongType)
	}
	return ctx.WriteBulkString(stringDigest(cur))
}

// cmdDELEX is DELEX key [IFEQ value | IFNE value | IFDEQ digest | IFDNE
// digest]: DEL of one key, or of a string key whose value or digest meets
// the condition. It replies 1 when it deleted the key.
func cmdDELEX(ctx *Context) error {
	n := ctx.ArgCount()
	if n != 1 && n != 3 {
		return ctx.WriteError(arityError("delex"))
	}
	key := ctx.ArgString(0)
	del := func() error {
		if !ctx.Store.Delete(key) {
			ctx.PropagateAs()
			return ctx.WriteInteger(0)
		}
		ctx.notify(store.NotifyGeneric, "del", key)
		ctx.propagateDelete(key)
		return ctx.WriteInteger(1)
	}
	if n == 1 {
		return del()
	}
	entry, ok := ctx.Store.Get(key)
	if !ok {
		ctx.PropagateAs()
		return ctx.WriteInteger(0)
	}
	cur, isString := stringValueOf(entry.Value)
	if !isString {
		return ctx.WriteError(errors.New("ERR Key should be of string type if conditions are specified"))
	}
	var flag int
	switch strings.ToUpper(ctx.ArgString(1)) {
	case "IFEQ":
		flag = strOptIFEQ
	case "IFNE":
		flag = strOptIFNE
	case "IFDEQ":
		flag = strOptIFDEQ
	case "IFDNE":
		flag = strOptIFDNE
	default:
		return ctx.WriteError(errors.New("ERR Invalid condition. Use IFEQ, IFNE, IFDEQ, or IFDNE"))
	}
	holds, err := stringConditionHolds(flag, cur, ctx.Arg(2))
	if err != nil {
		return ctx.WriteError(err)
	}
	if !holds {
		ctx.PropagateAs()
		return ctx.WriteInteger(0)
	}
	return del()
}

// cmdMSETEX is MSETEX numkeys key value [key value ...] [NX | XX]
// [EX seconds | PX milliseconds | EXAT unix-time-seconds |
// PXAT unix-time-milliseconds | KEEPTTL]: MSET with one expiry for every key,
// all set or, when NX or XX fails for any key, none. It replies 1 or 0.
func cmdMSETEX(ctx *Context) error {
	if ctx.ArgCount() < 3 {
		return ctx.WriteError(arityError("msetex"))
	}
	numkeys, err := strconv.ParseInt(ctx.ArgString(0), 10, 64)
	if err != nil || numkeys < 1 || numkeys > 1<<31-1 {
		return ctx.WriteError(errors.New("ERR invalid numkeys value"))
	}
	if numkeys*2+1 > int64(ctx.ArgCount()) {
		return ctx.WriteError(errors.New("ERR wrong number of key-value pairs"))
	}
	end := 1 + int(numkeys)*2
	flags, expireArg, _, err := parseStringArgs(ctx.Args[end:], strModeMsetex)
	if err != nil {
		return ctx.WriteError(err)
	}
	var whenMs int64
	if expireArg != nil {
		if whenMs, err = stringOptionsExpireMs(flags, expireArg, "msetex"); err != nil {
			return ctx.WriteError(err)
		}
	}
	pairs := make([]store.StringPair, 0, numkeys)
	for i := 1; i < end; i += 2 {
		key := ctx.ArgString(i)
		if len(key) > store.MaxKeySize {
			return ctx.WriteError(store.ErrInvalidKey)
		}
		pairs = append(pairs, store.StringPair{Key: key, Data: ctx.Arg(i + 1)})
	}
	ctx.writeEffectsHandled = true
	done, err := ctx.Store.SetStringBatchOpts(pairs, store.BatchSetOptions{
		NX: flags&strOptNX != 0, XX: flags&strOptXX != 0,
		ExpireAtMS: whenMs, KeepTTL: flags&strOptKEEPTTL != 0,
	})
	if err != nil {
		return ctx.WriteError(err)
	}
	if !done {
		ctx.PropagateAs()
		return ctx.WriteInteger(0)
	}
	// Replicas and the AOF get one SET per pair, with the expiry as an
	// absolute PXAT, which any Redis version replays.
	cmds := make([]ExecutedCommand, 0, len(pairs))
	for _, p := range pairs {
		args := [][]byte{[]byte(p.Key), p.Data}
		switch {
		case whenMs != 0:
			args = append(args, []byte("PXAT"), strconv.AppendInt(nil, whenMs, 10))
		case flags&strOptKEEPTTL != 0:
			args = append(args, []byte("KEEPTTL"))
		}
		cmds = append(cmds, ExecutedCommand{Name: "SET", Args: args, DB: ctx.DB})
	}
	ctx.PropagateAs(cmds...)
	return ctx.WriteInteger(1)
}
