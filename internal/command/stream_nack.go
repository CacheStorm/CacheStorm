package command

import (
	"errors"
	"fmt"
	"math"
	"strings"

	"github.com/cachestorm/cachestorm/internal/store"
)

// cmdXNACK is Redis 8.8's XNACK key group SILENT|FAIL|FATAL IDS numids
// id... [RETRYCOUNT count] [FORCE].
func cmdXNACK(ctx *Context) error {
	if ctx.ArgCount() < 6 {
		return ctx.WriteError(arityError("xnack"))
	}
	key, groupName := ctx.ArgString(0), ctx.ArgString(1)
	stream, err := lookupStream(ctx, key)
	if err != nil {
		return ctx.WriteError(err)
	}
	if stream == nil || !stream.HasGroup(groupName) {
		return ctx.WriteError(errNoGroupFor(key, groupName))
	}
	var mode int
	switch strings.ToUpper(ctx.ArgString(2)) {
	case "SILENT":
		mode = store.NackSilent
	case "FAIL":
		mode = store.NackFail
	case "FATAL":
		mode = store.NackFatal
	default:
		return ctx.WriteError(errors.New("ERR mode must be SILENT, FAIL, or FATAL"))
	}
	argc := ctx.ArgCount()
	idsStart, numIDs := 0, 0
	force := false
	retryCount := int64(-1)
	for i := 3; i < argc; i++ {
		more := argc - 1 - i
		opt := ctx.ArgString(i)
		switch {
		case strings.EqualFold(opt, "IDS") && more > 0:
			n, ok := parseStreamLL(ctx.ArgString(i + 1))
			if !ok || n < 1 || n > math.MaxInt32 {
				return ctx.WriteError(errors.New("ERR numids must be a positive integer"))
			}
			idsStart = i + 2
			if int(n) > argc-idsStart {
				return ctx.WriteError(errors.New("ERR number of IDs doesn't match numids"))
			}
			numIDs = int(n)
			i = idsStart + numIDs - 1
		case strings.EqualFold(opt, "FORCE"):
			force = true
		case strings.EqualFold(opt, "RETRYCOUNT") && more > 0:
			i++
			n, ok := parseStreamLL(ctx.ArgString(i))
			if !ok {
				return ctx.WriteError(ErrNotInteger)
			}
			if n < 0 {
				return ctx.WriteError(errors.New("ERR Invalid RETRYCOUNT value, must be >= 0"))
			}
			retryCount = n
		default:
			return ctx.WriteError(fmt.Errorf("ERR Unrecognized XNACK option '%s'", opt))
		}
	}
	if idsStart == 0 {
		return ctx.WriteError(errors.New("ERR syntax error, expected IDS keyword"))
	}
	ids := make([]store.StreamID, numIDs)
	for j := range ids {
		id, err := parseStrictStreamID(ctx.ArgString(idsStart + j))
		if err != nil {
			return ctx.WriteError(err)
		}
		ids[j] = id
	}
	released, ok := stream.Nack(groupName, ids, mode, retryCount, force)
	if !ok {
		return ctx.WriteError(errNoGroupFor(key, groupName))
	}
	if released > 0 {
		ctx.Store.KeyNotifier().NotifyKey(key)
	}
	return ctx.WriteInteger(released)
}
