package command

import (
	"math"

	"github.com/cachestorm/cachestorm/internal/store"
)

func RegisterHyperLogLogCommands(router *Router) {
	router.Register(&CommandDef{Name: "PFADD", Handler: cmdPFADD})
	router.Register(&CommandDef{Name: "PFCOUNT", Handler: cmdPFCOUNT})
	router.Register(&CommandDef{Name: "PFMERGE", Handler: cmdPFMERGE})
}

const hllBits = 14
const hllRegisters = 1 << hllBits

type HyperLogLogValue struct {
	Registers [hllRegisters]uint8
}

func (v *HyperLogLogValue) Type() store.DataType { return store.DataTypeString }
func (v *HyperLogLogValue) SizeOf() int64        { return hllRegisters + 24 }
func (v *HyperLogLogValue) String() string       { return "HyperLogLog" }
func (v *HyperLogLogValue) Clone() store.Value {
	cloned := &HyperLogLogValue{}
	cloned.Registers = v.Registers
	return cloned
}

func getOrCreateHLL(ctx *Context, key string) *HyperLogLogValue {
	entry, exists := ctx.Store.Get(key)
	if !exists {
		hll := &HyperLogLogValue{}
		ctx.Store.Set(key, hll, store.SetOptions{})
		return hll
	}

	if hll, ok := entry.Value.(*HyperLogLogValue); ok {
		return hll
	}
	return nil
}

func getHLL(ctx *Context, key string) *HyperLogLogValue {
	entry, exists := ctx.Store.Get(key)
	if !exists {
		return nil
	}

	if hll, ok := entry.Value.(*HyperLogLogValue); ok {
		return hll
	}
	return nil
}

func murmurHash(data []byte) uint64 {
	var h uint64 = 0

	for _, b := range data {
		h ^= uint64(b)
		h *= 0xc6a4a7935bd1e995
		h ^= h >> 47
	}

	return h
}

func hllHash(data []byte) uint64 {
	return murmurHash(data)
}

func countLeadingZeros(hash uint64) uint8 {
	if hash == 0 {
		return 64
	}
	var count uint8 = 1
	for (hash & (1 << 63)) == 0 {
		hash <<= 1
		count++
	}
	return count
}

func (hll *HyperLogLogValue) Add(data []byte) bool {
	hash := hllHash(data)
	index := hash & (hllRegisters - 1)
	rho := countLeadingZeros(hash>>hllBits) + 1

	if hll.Registers[index] < rho {
		hll.Registers[index] = rho
		return true
	}
	return false
}

func (hll *HyperLogLogValue) Count() int64 {
	var sum float64 = 0
	zeroCount := 0

	for i := 0; i < hllRegisters; i++ {
		sum += 1.0 / float64(uint64(1)<<hll.Registers[i])
		if hll.Registers[i] == 0 {
			zeroCount++
		}
	}

	alpha := 0.7213 / (1.0 + 1.079/float64(hllRegisters))
	estimate := alpha * float64(hllRegisters*hllRegisters) / sum

	if estimate <= 2.5*float64(hllRegisters) && zeroCount > 0 {
		estimate = float64(hllRegisters) * math.Log(float64(hllRegisters)/float64(zeroCount))
	}

	return int64(estimate)
}

func (hll *HyperLogLogValue) Merge(other *HyperLogLogValue) {
	for i := 0; i < hllRegisters; i++ {
		if other.Registers[i] > hll.Registers[i] {
			hll.Registers[i] = other.Registers[i]
		}
	}
}

func cmdPFADD(ctx *Context) error {
	if ctx.ArgCount() < 1 {
		return ctx.WriteError(ErrWrongArgCount)
	}

	key := ctx.ArgString(0)
	hll := getOrCreateHLL(ctx, key)
	if hll == nil {
		return ctx.WriteError(store.ErrWrongType)
	}

	updated := 0
	for i := 1; i < ctx.ArgCount(); i++ {
		if hll.Add(ctx.Arg(i)) {
			updated = 1
		}
	}

	return ctx.WriteInteger(int64(updated))
}

func cmdPFCOUNT(ctx *Context) error {
	if ctx.ArgCount() < 1 {
		return ctx.WriteError(ErrWrongArgCount)
	}

	if ctx.ArgCount() == 1 {
		key := ctx.ArgString(0)
		hll := getHLL(ctx, key)
		if hll == nil {
			return ctx.WriteInteger(0)
		}
		return ctx.WriteInteger(hll.Count())
	}

	merged := &HyperLogLogValue{}
	for i := 0; i < ctx.ArgCount(); i++ {
		hll := getHLL(ctx, ctx.ArgString(i))
		if hll != nil {
			merged.Merge(hll)
		}
	}

	return ctx.WriteInteger(merged.Count())
}

func cmdPFMERGE(ctx *Context) error {
	if ctx.ArgCount() < 1 {
		return ctx.WriteError(ErrWrongArgCount)
	}

	destKey := ctx.ArgString(0)
	dest := getOrCreateHLL(ctx, destKey)
	if dest == nil {
		return ctx.WriteError(store.ErrWrongType)
	}

	for i := 1; i < ctx.ArgCount(); i++ {
		src := getHLL(ctx, ctx.ArgString(i))
		if src != nil {
			dest.Merge(src)
		}
	}

	return ctx.WriteOK()
}
