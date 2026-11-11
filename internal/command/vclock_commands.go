package command

import (
	"errors"
	"sort"

	"github.com/cachestorm/cachestorm/internal/resp"
)

// A VECTOR_CLOCK is a hash key marked by the field __vclock with one field
// per node holding its counter. Being a key, a clock is per database,
// persisted and replicated; changes are recorded as HSET (DEL and HSET when
// a clock is created over another).

const vclockMarker = "__vclock"

var errVClockNotFound = errors.New("ERR clock not found")

func loadVClock(ctx *Context, name string) (map[string]int64, error) {
	h, err := readMarkedHash(ctx, name, vclockMarker)
	if err != nil {
		return nil, err
	}
	if h == nil {
		return nil, errVClockNotFound
	}
	return counterFields(ctx, name, vclockMarker)
}

// VECTOR_CLOCK.CREATE name node makes a clock holding node at 0.
func cmdVECTORCLOCKCREATE(ctx *Context) error {
	if ctx.ArgCount() != 2 {
		return ctx.WriteError(ErrWrongArgCount)
	}
	name := ctx.ArgString(0)
	if _, err := readMarkedHash(ctx, name, vclockMarker); err != nil {
		return ctx.WriteError(err)
	}
	if err := replaceRecord(ctx, name, vclockMarker, "1", ctx.ArgString(1), "0"); err != nil {
		return ctx.WriteError(err)
	}
	return ctx.WriteOK()
}

// VECTOR_CLOCK.INCREMENT name node ticks node and replies its counter.
func cmdVECTORCLOCKINCREMENT(ctx *Context) error {
	if ctx.ArgCount() != 2 {
		return ctx.WriteError(ErrWrongArgCount)
	}
	n, err := bumpCounter(ctx, ctx.ArgString(0), vclockMarker, ctx.ArgString(1), 1)
	if err != nil {
		return ctx.WriteError(err)
	}
	return ctx.WriteInteger(n)
}

// VECTOR_CLOCK.COMPARE a b replies equal, before, after or concurrent.
func cmdVECTORCLOCKCOMPARE(ctx *Context) error {
	if ctx.ArgCount() != 2 {
		return ctx.WriteError(ErrWrongArgCount)
	}
	a, err := loadVClock(ctx, ctx.ArgString(0))
	if err != nil {
		return ctx.WriteError(err)
	}
	b, err := loadVClock(ctx, ctx.ArgString(1))
	if err != nil {
		return ctx.WriteError(err)
	}
	aAhead, bAhead := false, false
	for k, v := range a {
		aAhead = aAhead || v > b[k]
	}
	for k, v := range b {
		bAhead = bAhead || v > a[k]
	}
	switch {
	case !aAhead && !bAhead:
		return ctx.WriteBulkString("equal")
	case bAhead && !aAhead:
		return ctx.WriteBulkString("before")
	case aAhead && !bAhead:
		return ctx.WriteBulkString("after")
	}
	return ctx.WriteBulkString("concurrent")
}

// VECTOR_CLOCK.MERGE a b raises each of a's counters to b's.
func cmdVECTORCLOCKMERGE(ctx *Context) error {
	if ctx.ArgCount() != 2 {
		return ctx.WriteError(ErrWrongArgCount)
	}
	name := ctx.ArgString(0)
	a, err := loadVClock(ctx, name)
	if err != nil {
		return ctx.WriteError(err)
	}
	b, err := loadVClock(ctx, ctx.ArgString(1))
	if err != nil {
		return ctx.WriteError(err)
	}
	var pairs []string
	for k, v := range b {
		if v > a[k] {
			pairs = append(pairs, k, i64(v))
		}
	}
	if len(pairs) == 0 {
		ctx.PropagateAs()
		return ctx.WriteOK()
	}
	if err := updateRecord(ctx, name, pairs...); err != nil {
		return ctx.WriteError(err)
	}
	return ctx.WriteOK()
}

// VECTOR_CLOCK.GET name replies node, counter pairs sorted by node.
func cmdVECTORCLOCKGET(ctx *Context) error {
	if ctx.ArgCount() != 1 {
		return ctx.WriteError(ErrWrongArgCount)
	}
	vc, err := counterFields(ctx, ctx.ArgString(0), vclockMarker)
	if err != nil {
		return ctx.WriteError(err)
	}
	nodes := make([]string, 0, len(vc))
	for k := range vc {
		nodes = append(nodes, k)
	}
	sort.Strings(nodes)
	out := make([]*resp.Value, 0, 2*len(nodes))
	for _, k := range nodes {
		out = append(out, resp.BulkString(k), resp.IntegerValue(vc[k]))
	}
	return ctx.WriteArray(out)
}
