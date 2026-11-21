package command

import (
	"errors"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/cachestorm/cachestorm/internal/resp"
	"github.com/cachestorm/cachestorm/internal/store"
)

// Redis 8.10's LMOVEM and BLMOVEM: LMOVE of several elements at once.

const (
	moveDefault = iota // no COUNT or EXACTLY: one element
	moveUpTo           // COUNT: at most count
	moveExactly        // EXACTLY: count or nothing
)

var errMoveCountPositive = errors.New("ERR count should be greater than 0")

type moveManyArgs struct {
	src, dst         string
	fromLeft, toLeft bool
	mode             int
	count            int64
	// obo pushes each element as it is popped; otherwise (BULK) the moved
	// block keeps its order from the source.
	obo bool
}

// parseMoveMany parses LMOVEM's and BLMOVEM's keys and directions.
func parseMoveMany(ctx *Context) (moveManyArgs, error) {
	a := moveManyArgs{src: ctx.ArgString(0), dst: ctx.ArgString(1), count: 1}
	from, to := strings.ToUpper(ctx.ArgString(2)), strings.ToUpper(ctx.ArgString(3))
	if (from != "LEFT" && from != "RIGHT") || (to != "LEFT" && to != "RIGHT") {
		return a, ErrSyntaxError
	}
	a.fromLeft, a.toLeft = from == "LEFT", to == "LEFT"
	return a, nil
}

// parseMoveManyTrailer parses the [COUNT|EXACTLY count OBO|BULK] trailer
// starting at argument opt: absent, or exactly those three words.
func parseMoveManyTrailer(ctx *Context, opt int, a *moveManyArgs) error {
	n := ctx.ArgCount()
	if opt == n {
		return nil
	}
	if n-opt != 3 {
		return ErrSyntaxError
	}
	switch strings.ToUpper(ctx.ArgString(opt)) {
	case "COUNT":
		a.mode = moveUpTo
	case "EXACTLY":
		a.mode = moveExactly
	default:
		return ErrSyntaxError
	}
	c, err := strconv.ParseInt(ctx.ArgString(opt+1), 10, 64)
	if err != nil || c < 1 {
		return errMoveCountPositive
	}
	a.count = c
	switch strings.ToUpper(ctx.ArgString(opt + 2)) {
	case "OBO":
		a.obo = true
	case "BULK":
	default:
		return ErrSyntaxError
	}
	return nil
}

// needed is how many elements the source must hold for the move to happen.
func (a *moveManyArgs) needed() int {
	if a.mode == moveExactly {
		return int(min(a.count, 1<<31))
	}
	return 1
}

// tryListMoveMany moves the elements a asks for, replying nothing; it
// returns them in their order in the destination, or false when the source
// holds too few.
func tryListMoveMany(ctx *Context, a *moveManyArgs) ([][]byte, bool, error) {
	srcList, err := getList(ctx, a.src)
	if err != nil || srcList == nil {
		return nil, false, err
	}
	if srcList, err = lockList(ctx, a.src); err != nil || srcList == nil {
		return nil, false, err
	}
	length := srcList.Len()
	if length == 0 || length < a.needed() {
		srcList.Unlock()
		return nil, false, nil
	}
	if a.src != a.dst {
		if _, err := getList(ctx, a.dst); err != nil {
			srcList.Unlock()
			return nil, false, err
		}
	}
	n := int(min(a.count, int64(length)))
	var vals [][]byte
	if a.fromLeft {
		vals = srcList.PopFront(n)
	} else {
		vals = srcList.PopBack(n)
	}
	// vals is in pop order; turn it into the order the block reads in the
	// destination, left to right.
	if (a.obo && a.toLeft) || (!a.obo && !a.fromLeft) {
		slices.Reverse(vals)
	}
	if a.src == a.dst {
		pushMoved(srcList, vals, a.toLeft)
		srcList.Unlock()
		ctx.notify(store.NotifyList, popEvent(a.fromLeft), a.src)
		ctx.notify(store.NotifyList, pushEvent(a.toLeft), a.dst)
		return vals, true, nil
	}
	srcEmpty := srcList.Len() == 0
	srcList.Unlock()
	srcDeleted := srcEmpty && ctx.Store.DeleteIfEmpty(a.src, srcList)
	dstList, created, err := lockOrInstall(ctx, a.dst, func() *store.ListValue {
		list := &store.ListValue{}
		pushMoved(list, vals, a.toLeft)
		return list
	})
	if err != nil {
		return nil, false, err
	}
	if !created {
		pushMoved(dstList, vals, a.toLeft)
		dstList.Unlock()
		ctx.Store.KeyNotifier().NotifyKey(a.dst)
	}
	ctx.notify(store.NotifyList, pushEvent(a.toLeft), a.dst)
	ctx.notify(store.NotifyList, popEvent(a.fromLeft), a.src)
	if srcDeleted {
		ctx.notify(store.NotifyGeneric, "del", a.src)
	}
	return vals, true, nil
}

// pushMoved adds vals, popped from a list, so they read left to right at the
// head (left) or tail of list.
func pushMoved(list *store.ListValue, vals [][]byte, left bool) {
	if left {
		for i := len(vals) - 1; i >= 0; i-- {
			list.PushMoved(vals[i], true)
		}
		return
	}
	for _, v := range vals {
		list.PushMoved(v, false)
	}
}

// replyMoved replies with the moved elements and records the move for
// replicas and the AOF as a pop of the source and a push of the block onto
// the destination, which every Redis version replays.
func replyMoved(ctx *Context, a *moveManyArgs, vals [][]byte) error {
	pop := "RPOP"
	if a.fromLeft {
		pop = "LPOP"
	}
	push := [][]byte{[]byte(a.dst)}
	name := "RPUSH"
	if a.toLeft {
		name = "LPUSH"
		for i := len(vals) - 1; i >= 0; i-- {
			push = append(push, vals[i])
		}
	} else {
		push = append(push, vals...)
	}
	ctx.PropagateAs(
		ExecutedCommand{Name: pop, Args: [][]byte{[]byte(a.src), strconv.AppendInt(nil, int64(len(vals)), 10)}, DB: ctx.DB},
		ExecutedCommand{Name: name, Args: push, DB: ctx.DB},
	)
	out := make([]*resp.Value, len(vals))
	for i, v := range vals {
		out[i] = resp.BulkBytes(v)
	}
	return ctx.WriteArray(out)
}

// cmdLMOVEM is LMOVEM source destination LEFT|RIGHT LEFT|RIGHT
// [COUNT|EXACTLY count OBO|BULK]. It replies with the moved elements in
// destination order, or a null array when nothing moved.
func cmdLMOVEM(ctx *Context) error {
	if ctx.ArgCount() < 4 {
		return ctx.WriteError(arityError(ctx.Command))
	}
	a, err := parseMoveMany(ctx)
	if err == nil {
		err = parseMoveManyTrailer(ctx, 4, &a)
	}
	if err != nil {
		return ctx.WriteError(err)
	}
	vals, ok, err := tryListMoveMany(ctx, &a)
	if err != nil {
		return ctx.WriteError(err)
	}
	if !ok {
		ctx.PropagateAs()
		return ctx.WriteNullArray()
	}
	return replyMoved(ctx, &a, vals)
}

// cmdBLMOVEM is BLMOVEM source destination LEFT|RIGHT LEFT|RIGHT timeout
// [COUNT|EXACTLY count OBO|BULK]: LMOVEM that waits until the source holds
// an element, or with EXACTLY count elements.
func cmdBLMOVEM(ctx *Context) error {
	if ctx.ArgCount() < 5 {
		return ctx.WriteError(arityError(ctx.Command))
	}
	a, err := parseMoveMany(ctx)
	if err != nil {
		return ctx.WriteError(err)
	}
	timeout, err := parseBlockTimeout(ctx.ArgString(4))
	if err != nil {
		return ctx.WriteError(err)
	}
	if err := parseMoveManyTrailer(ctx, 5, &a); err != nil {
		return ctx.WriteError(err)
	}
	vals, ok, err := tryListMoveMany(ctx, &a)
	if err != nil {
		return ctx.WriteError(err)
	}
	if !ok {
		need := a.needed()
		ready := ctx.holdsType(func(v store.Value) bool {
			l, isList := v.(*store.ListValue)
			if !isList {
				return false
			}
			l.RLock()
			defer l.RUnlock()
			return l.Len() >= need
		})
		var moveErr error
		wait := func(keys []string, d time.Duration) (string, bool) {
			return ctx.waitForKeysReady(keys, d, ready)
		}
		ok = blockOnKeys(ctx, []string{a.src}, timeout, wait, func(string) bool {
			vals, ok, moveErr = tryListMoveMany(ctx, &a)
			return ok || moveErr != nil
		})
		if moveErr != nil {
			return ctx.WriteError(moveErr)
		}
	}
	if !ok {
		ctx.PropagateAs()
		return ctx.WriteNullArray()
	}
	return replyMoved(ctx, &a, vals)
}
