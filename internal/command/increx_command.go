package command

import (
	"errors"
	"math"
	"math/big"
	"strconv"
	"strings"
	"time"

	"github.com/cachestorm/cachestorm/internal/resp"
	"github.com/cachestorm/cachestorm/internal/store"
)

// Redis 8.8's INCREX key [BYFLOAT incr | BYINT incr] [LBOUND lb] [UBOUND ub]
// [SATURATE] [EX s | PX ms | EXAT ts | PXAT ms-ts | PERSIST] [ENX]:
// INCRBY or INCRBYFLOAT kept within bounds, optionally setting the expiry.
// It replies with the new value and the increment applied; a result out of
// bounds leaves the key alone and replies [current, 0], or with SATURATE is
// clamped to the bound.

const (
	increxByFloat = 1 << iota
	increxByInt
	increxLBound
	increxUBound
	increxSaturate
	increxENX
	increxPersist
	increxEX
	increxPX
	increxEXAT
	increxPXAT

	increxExpire = increxEX | increxPX | increxEXAT | increxPXAT | increxPersist
)

type increxArgs struct {
	flags           int
	expireMs        int64
	incr, lb, ub    int64
	incrF, lbF, ubF *big.Float
}

// ldMax is LDBL_MAX for the x87 long double: a 64-bit mantissa of ones at
// the largest exponent.
var ldMax = func() *big.Float {
	m := new(big.Float).SetPrec(longDoublePrec).SetMantExp(big.NewFloat(1), -64)
	m.Sub(big.NewFloat(1).SetPrec(longDoublePrec), m)
	return m.SetMantExp(m, longDoubleMaxExp)
}()

func ldOverflows(v *big.Float) bool {
	return v.IsInf() || (v.Sign() != 0 && v.MantExp(nil) > longDoubleMaxExp)
}

func newLD() *big.Float { return new(big.Float).SetPrec(longDoublePrec).SetMode(big.ToNearestEven) }

func parseIncrexArgs(ctx *Context) (*increxArgs, error) {
	a := &increxArgs{incr: 1, lb: math.MinInt64, ub: math.MaxInt64}
	a.lbF, a.ubF = newLD().Neg(ldMax), newLD().Set(ldMax)
	var lower, upper, expire []byte
	n := ctx.ArgCount()
	for j := 1; j < n; j++ {
		opt := strings.ToUpper(ctx.ArgString(j))
		hasNext := j+1 < n
		switch {
		case opt == "BYINT" && hasNext && a.flags&(increxByInt|increxByFloat) == 0:
			v, err := redisAtoi64(ctx.ArgString(j + 1))
			if err != nil {
				return nil, errors.New("ERR Increment is not an integer or out of range")
			}
			a.incr, a.flags = v, a.flags|increxByInt
			j++
		case opt == "BYFLOAT" && hasNext && a.flags&(increxByInt|increxByFloat) == 0:
			v, ok := parseIncrByFloatArg(ctx.Arg(j + 1))
			if !ok {
				return nil, errors.New("ERR Increment is not a valid float")
			}
			if v.IsInf() {
				return nil, errors.New("ERR BYFLOAT increment cannot be Infinity")
			}
			a.incrF, a.flags = v, a.flags|increxByFloat
			j++
		case opt == "LBOUND" && hasNext && a.flags&increxLBound == 0:
			lower, a.flags = ctx.Arg(j+1), a.flags|increxLBound
			j++
		case opt == "UBOUND" && hasNext && a.flags&increxUBound == 0:
			upper, a.flags = ctx.Arg(j+1), a.flags|increxUBound
			j++
		case opt == "SATURATE" && a.flags&increxSaturate == 0:
			a.flags |= increxSaturate
		case opt == "ENX" && a.flags&(increxENX|increxPersist) == 0:
			a.flags |= increxENX
		case opt == "PERSIST" && a.flags&(increxExpire|increxENX) == 0:
			a.flags |= increxPersist
		case (opt == "EX" || opt == "PX" || opt == "EXAT" || opt == "PXAT") && hasNext && a.flags&increxExpire == 0:
			a.flags |= map[string]int{"EX": increxEX, "PX": increxPX, "EXAT": increxEXAT, "PXAT": increxPXAT}[opt]
			expire = ctx.Arg(j + 1)
			j++
		default:
			return nil, ErrSyntaxError
		}
	}
	if a.flags&increxByFloat != 0 {
		if lower != nil {
			v, ok := parseIncrByFloatArg(lower)
			if !ok {
				return nil, errors.New("ERR LBOUND is not a valid float")
			}
			a.lbF = v
		}
		if upper != nil {
			v, ok := parseIncrByFloatArg(upper)
			if !ok {
				return nil, errors.New("ERR UBOUND is not a valid float")
			}
			a.ubF = v
		}
		if a.lbF.Cmp(a.ubF) > 0 {
			return nil, errors.New("ERR LBOUND can't be greater than UBOUND")
		}
	} else {
		var err error
		if lower != nil {
			if a.lb, err = redisAtoi64(string(lower)); err != nil {
				return nil, errors.New("ERR LBOUND is not an integer or out of range")
			}
		}
		if upper != nil {
			if a.ub, err = redisAtoi64(string(upper)); err != nil {
				return nil, errors.New("ERR UBOUND is not an integer or out of range")
			}
		}
		if a.lb > a.ub {
			return nil, errors.New("ERR LBOUND can't be greater than UBOUND")
		}
	}
	if a.flags&increxENX != 0 && a.flags&increxExpire == 0 {
		return nil, errors.New("ERR ENX flag requires an expiration")
	}
	if expire != nil {
		ms, err := stringExpireMs(expire, a.flags&(increxEX|increxEXAT) != 0, a.flags&(increxEX|increxPX) != 0, "increx")
		if err != nil {
			return nil, err
		}
		a.expireMs = ms
	}
	return a, nil
}

func cmdINCREX(ctx *Context) error {
	if ctx.ArgCount() < 1 {
		return ctx.WriteError(arityError("increx"))
	}
	a, err := parseIncrexArgs(ctx)
	if err != nil {
		return ctx.WriteError(err)
	}
	key := ctx.ArgString(0)
	entry, exists := ctx.Store.Get(key)
	var cur []byte
	if exists {
		var ok bool
		if cur, ok = stringValueOf(entry.Value); !ok {
			return ctx.WriteError(store.ErrWrongType)
		}
	}
	saturate := a.flags&increxSaturate != 0
	byFloat := a.flags&increxByFloat != 0
	var stored string
	if byFloat {
		old := newLD()
		if exists {
			v, ok := parseIncrByFloatArg(cur)
			if !ok {
				return ctx.WriteError(ErrNotFloat)
			}
			if v.IsInf() {
				return ctx.WriteError(errors.New("ERR value cannot be Infinity"))
			}
			old.Set(v)
		}
		value := newLD().Add(old, a.incrF)
		overflow := ldOverflows(value)
		if overflow || value.Cmp(a.ubF) > 0 || value.Cmp(a.lbF) < 0 {
			if !saturate {
				return ctx.WriteArray([]*resp.Value{resp.DoubleValue(formatLongDoubleHuman(old)), resp.DoubleValue("0")})
			}
			switch {
			case overflow && a.incrF.Sign() >= 0, !overflow && value.Cmp(a.ubF) > 0:
				value.Set(a.ubF)
			default:
				value.Set(a.lbF)
			}
		}
		delta := newLD().Sub(value, old)
		if ldOverflows(delta) {
			return ctx.WriteError(errors.New("ERR applied increment would be Infinity"))
		}
		stored = formatLongDoubleHuman(value)
		if err := ctx.WriteArray([]*resp.Value{resp.DoubleValue(stored), resp.DoubleValue(formatLongDoubleHuman(delta))}); err != nil {
			return err
		}
	} else {
		old := int64(0)
		if exists {
			v, perr := redisAtoi64(string(cur))
			if perr != nil {
				return ctx.WriteError(ErrNotInteger)
			}
			old = v
		}
		value, overflow := old+a.incr, false
		if (a.incr > 0 && value < old) || (a.incr < 0 && value > old) {
			overflow = true
		}
		if overflow || value > a.ub || value < a.lb {
			if !saturate {
				return ctx.WriteArray([]*resp.Value{resp.IntegerValue(old), resp.IntegerValue(0)})
			}
			switch {
			case overflow && a.incr >= 0, !overflow && value > a.ub:
				value = a.ub
			default:
				value = a.lb
			}
		}
		delta := value - old
		if (old < 0 && value > 0 && delta < 0) || (old > 0 && value < 0 && delta > 0) {
			return ctx.WriteError(errors.New("ERR applied increment would overflow"))
		}
		stored = strconv.FormatInt(value, 10)
		if err := ctx.WriteArray([]*resp.Value{resp.IntegerValue(value), resp.IntegerValue(delta)}); err != nil {
			return err
		}
	}

	hadTTL := false
	if exists {
		_, hadTTL = ctx.Store.ExpiryDeadline(key)
	}
	setExpire := a.expireMs != 0 && (a.flags&increxENX == 0 || !hadTTL)
	if setExpire && a.expireMs <= time.Now().UnixMilli() && !ctx.Replaying {
		if exists && ctx.Store.Delete(key) {
			ctx.notify(store.NotifyGeneric, "del", key)
			ctx.propagateDelete(key)
		} else {
			ctx.PropagateAs()
		}
		return nil
	}
	opts := store.SetOptions{KeepTTL: a.flags&increxPersist == 0 && !setExpire}
	if setExpire {
		opts.ExpireAtMS = a.expireMs
	}
	enc := store.StringEncAuto
	if byFloat {
		enc = store.StringEncNoInt
	}
	if err := ctx.Store.Set(key, &store.StringValue{Data: []byte(stored), Enc: enc}, opts); err != nil {
		return err
	}
	args := [][]byte{[]byte(key), []byte(stored)}
	switch {
	case a.flags&increxPersist != 0:
	case setExpire:
		args = append(args, []byte("PXAT"), strconv.AppendInt(nil, a.expireMs, 10))
	default:
		args = append(args, []byte("KEEPTTL"))
	}
	ctx.PropagateAs(ExecutedCommand{Name: "SET", Args: args, DB: ctx.DB})
	if byFloat {
		ctx.notify(store.NotifyString, "incrbyfloat", key)
	} else {
		ctx.notify(store.NotifyString, "incrby", key)
	}
	if a.flags&increxPersist != 0 && hadTTL {
		ctx.notify(store.NotifyGeneric, "persist", key)
	}
	if setExpire {
		ctx.notify(store.NotifyGeneric, "expire", key)
	}
	return nil
}
