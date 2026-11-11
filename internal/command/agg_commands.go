package command

import (
	"errors"
	"fmt"
	"math"
	"strconv"
)

// An AGG series is a list key of numbers: AGG.PUSH appends with RPUSH,
// AGG.CLEAR deletes the key, and the aggregates read it. Being a list, a
// series is per database, persisted, replicated, and readable with LRANGE.

var errAggValue = errors.New("ERR value is not a valid float")

// AGG.PUSH key value [value ...] appends the values and replies the length.
func cmdAGGPUSH(ctx *Context) error {
	if ctx.ArgCount() < 2 {
		return ctx.WriteError(ErrWrongArgCount)
	}
	for i := 1; i < ctx.ArgCount(); i++ {
		v, err := strconv.ParseFloat(ctx.ArgString(i), 64)
		if err != nil || math.IsNaN(v) || math.IsInf(v, 0) {
			return ctx.WriteError(errAggValue)
		}
	}
	return recordedAs(ctx, "RPUSH", func() error { return listPush(ctx, false) })
}

func cmdAGGCLEAR(ctx *Context) error {
	return recordedAs(ctx, "DEL", func() error { return listCollectionClear(ctx) })
}

// aggValues reads the numbers of the series at key, nil when it is missing.
func aggValues(ctx *Context) ([]float64, error) {
	if ctx.ArgCount() != 1 {
		return nil, ErrWrongArgCount
	}
	list, err := getList(ctx, ctx.ArgString(0))
	if err != nil || list == nil {
		return nil, err
	}
	list.RLock()
	elems := list.Elements()
	list.RUnlock()
	out := make([]float64, len(elems))
	for i, el := range elems {
		v, err := strconv.ParseFloat(string(el), 64)
		if err != nil || math.IsNaN(v) || math.IsInf(v, 0) {
			return nil, errors.New("ERR the list holds a value that is not a number")
		}
		out[i] = v
	}
	return out, nil
}

// aggregate replies fold of the series as a float with two decimals, empty
// when the series is missing or empty.
func aggregate(ctx *Context, empty func() error, fold func([]float64) float64) error {
	data, err := aggValues(ctx)
	if err != nil {
		return ctx.WriteError(err)
	}
	if len(data) == 0 {
		return empty()
	}
	return ctx.WriteBulkString(fmt.Sprintf("%.2f", fold(data)))
}

func aggSum(data []float64) float64 {
	var sum float64
	for _, v := range data {
		sum += v
	}
	return sum
}

func cmdAGGSUM(ctx *Context) error {
	return aggregate(ctx, func() error { return ctx.WriteInteger(0) }, aggSum)
}

func cmdAGGAVG(ctx *Context) error {
	return aggregate(ctx, func() error { return ctx.WriteBulkString("0") }, func(d []float64) float64 {
		return aggSum(d) / float64(len(d))
	})
}

func cmdAGGMIN(ctx *Context) error {
	return aggregate(ctx, func() error { return ctx.WriteBulkString("0") }, func(d []float64) float64 {
		m := d[0]
		for _, v := range d {
			m = math.Min(m, v)
		}
		return m
	})
}

func cmdAGGMAX(ctx *Context) error {
	return aggregate(ctx, func() error { return ctx.WriteBulkString("0") }, func(d []float64) float64 {
		m := d[0]
		for _, v := range d {
			m = math.Max(m, v)
		}
		return m
	})
}

func cmdAGGCOUNT(ctx *Context) error {
	if ctx.ArgCount() != 1 {
		return ctx.WriteError(ErrWrongArgCount)
	}
	return cmdLLEN(ctx)
}
