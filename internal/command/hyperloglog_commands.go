package command

import (
	"encoding/binary"
	"errors"
	"fmt"
	"math"
	"strconv"
	"strings"

	"github.com/cachestorm/cachestorm/internal/resp"
	"github.com/cachestorm/cachestorm/internal/store"
)

func RegisterHyperLogLogCommands(router *Router) {
	router.Register(&CommandDef{Name: "PFADD", Handler: cmdPFADD})
	router.Register(&CommandDef{Name: "PFCOUNT", Handler: cmdPFCOUNT})
	router.Register(&CommandDef{Name: "PFMERGE", Handler: cmdPFMERGE})
	router.Register(&CommandDef{Name: "PFDEBUG", Handler: cmdPFDEBUG})
	router.Register(&CommandDef{Name: "PFSELFTEST", Handler: cmdPFSELFTEST})
}

// cmdPFDEBUG implements PFDEBUG GETREG|ENCODING|TODENSE|DECODE key.
func cmdPFDEBUG(ctx *Context) error {
	ctx.writeEffectsHandled = true
	if ctx.ArgCount() != 2 {
		return ctx.WriteError(arityError("pfdebug"))
	}
	sub, key := strings.ToUpper(ctx.ArgString(0)), ctx.ArgString(1)
	switch sub {
	case "GETREG", "ENCODING", "TODENSE", "DECODE":
	default:
		return ctx.WriteError(fmt.Errorf("ERR Unknown PFDEBUG subcommand '%s'", ctx.ArgString(0)))
	}
	entry, ok := ctx.Store.Get(key)
	if !ok {
		return ctx.WriteError(errors.New("ERR The specified key does not exist"))
	}
	data, err := hllValueBytes(entry.Value)
	if err != nil {
		return ctx.WriteError(err)
	}
	if err := store.ValidateHLL(data); err != nil {
		return ctx.WriteError(err)
	}
	dense := data[4] == 0
	switch sub {
	case "ENCODING":
		ctx.PropagateAs()
		if dense {
			return ctx.WriteSimpleString("dense")
		}
		return ctx.WriteSimpleString("sparse")
	case "DECODE":
		ctx.PropagateAs()
		if dense {
			return ctx.WriteError(errors.New("ERR HLL encoding is not sparse"))
		}
		text, err := decodeSparseHLL(data)
		if err != nil {
			return ctx.WriteError(err)
		}
		return ctx.WriteSimpleString(text)
	}
	state, err := store.DecodeHLL(data)
	if err != nil {
		return ctx.WriteError(err)
	}
	converted := false
	if !dense {
		state.Dense = true
		next, err := state.Encode(false)
		if err != nil {
			return ctx.WriteError(err)
		}
		copy(next[8:16], data[8:16])
		next[15] = data[15]
		if err := ctx.Store.ApplyValuePlan(key, func(store.Value) (store.ValuePlan, error) {
			return hllStringPlan(next), nil
		}); err != nil {
			return ctx.WriteError(err)
		}
		converted = true
		ctx.PropagateAs(ExecutedCommand{Name: "PFDEBUG", Args: [][]byte{[]byte("TODENSE"), []byte(key)}, DB: ctx.DB})
	} else {
		ctx.PropagateAs()
	}
	if sub == "GETREG" {
		regs := make([]*resp.Value, len(state.Registers))
		for i, r := range state.Registers {
			regs[i] = resp.IntegerValue(int64(r))
		}
		return ctx.WriteArray(regs)
	}
	if converted {
		return ctx.WriteInteger(1)
	}
	return ctx.WriteInteger(0)
}

// decodeSparseHLL renders a sparse HLL's opcodes as Redis' PFDEBUG DECODE
// does: z:len for ZERO, Z:len for XZERO and v:value,len for VAL.
func decodeSparseHLL(data []byte) (string, error) {
	var parts []string
	for pos := 16; pos < len(data); {
		op := data[pos]
		pos++
		switch {
		case op&128 != 0:
			parts = append(parts, "v:"+strconv.Itoa(int((op>>2)&31)+1)+","+strconv.Itoa(int(op&3)+1))
		case op&64 != 0:
			if pos >= len(data) {
				return "", store.CorruptHLLError()
			}
			parts = append(parts, "Z:"+strconv.Itoa((int(op&63)<<8|int(data[pos]))+1))
			pos++
		default:
			parts = append(parts, "z:"+strconv.Itoa(int(op&63)+1))
		}
	}
	return strings.Join(parts, " "), nil
}

// cmdPFSELFTEST checks the HyperLogLog implementation: the dense and sparse
// encodings must decode to the registers they were built from, and the
// estimate must stay within a few standard errors of the true cardinality.
func cmdPFSELFTEST(ctx *Context) error {
	if ctx.ArgCount() != 0 {
		return ctx.WriteError(arityError("pfselftest"))
	}
	fail := func(format string, args ...any) error {
		return ctx.WriteError(fmt.Errorf("ERR TESTFAILED "+format, args...))
	}
	var h store.HLLState
	relErr := 1.04 / math.Sqrt(float64(store.HLLRegisterCount))
	for i := 1; i <= 100000; i++ {
		h.Add([]byte("selftest:" + strconv.Itoa(i)))
		if i == 10 || i == 100 || i == 1000 || i == 10000 || i == 100000 {
			for _, dense := range []bool{false, true} {
				h.Dense = dense
				data, err := h.Encode(false)
				if err != nil {
					return fail("encode: %v", err)
				}
				back, err := store.DecodeHLL(data)
				if err != nil {
					return fail("decode: %v", err)
				}
				if back.Registers != h.Registers {
					return fail("registers differ after a round trip at %d elements", i)
				}
			}
			h.Dense = false
			got := float64(h.Count())
			if math.Abs(got-float64(i))/float64(i) > 5*relErr && math.Abs(got-float64(i)) > 1 {
				return fail("cardinality estimate %.0f for %d elements", got, i)
			}
		}
	}
	return ctx.WriteOK()
}

type HyperLogLogValue = store.HyperLogLogValue

func hllValueBytes(value store.Value) ([]byte, error) {
	if legacy, ok := value.(*HyperLogLogValue); ok {
		return legacy.RedisStringBytes()
	}
	data, ok := stringValueOf(value)
	if !ok {
		return nil, store.ErrWrongType
	}
	return data, nil
}
func decodeHLLValue(value store.Value) (*store.HLLState, error) {
	if value == nil {
		return &store.HLLState{}, nil
	}
	data, err := hllValueBytes(value)
	if err != nil {
		return nil, err
	}
	return store.DecodeHLL(data)
}
func hllStringPlan(data []byte) store.ValuePlan {
	return store.ValuePlan{Size: int64(len(data)) + 24, Apply: func() store.Value { return &store.StringValue{Data: data} }}
}

func cmdPFADD(ctx *Context) error {
	ctx.writeEffectsHandled = true
	if ctx.ArgCount() < 1 {
		return ctx.WriteError(ErrWrongArgCount)
	}
	updated := false
	err := ctx.Store.ApplyValuePlan(ctx.ArgString(0), func(value store.Value) (store.ValuePlan, error) {
		if value != nil && ctx.ArgCount() == 1 {
			data, err := hllValueBytes(value)
			if err != nil {
				return store.ValuePlan{}, err
			}
			if err := store.ValidateHLL(data); err != nil {
				return store.ValuePlan{}, err
			}
			updated = false
			return store.ValuePlan{}, nil
		}
		if value != nil {
			current, err := hllValueBytes(value)
			if err != nil {
				return store.ValuePlan{}, err
			}
			if err := store.ValidateHLL(current); err != nil {
				return store.ValuePlan{}, err
			}
			if current[4] == 0 {
				next, changed, err := store.AddDenseHLL(current, ctx.Args[1:])
				if err != nil {
					return store.ValuePlan{}, err
				}
				updated = changed
				if !updated {
					return store.ValuePlan{}, nil
				}
				return hllStringPlan(next), nil
			}
		}
		state, err := decodeHLLValue(value)
		if err != nil {
			return store.ValuePlan{}, err
		}
		updated = value == nil
		for _, element := range ctx.Args[1:] {
			if state.Add(element) {
				updated = true
			}
		}
		if !updated {
			return store.ValuePlan{}, nil
		}
		data, err := state.Encode(true)
		if err != nil {
			return store.ValuePlan{}, err
		}
		return hllStringPlan(data), nil
	})
	if err != nil {
		return ctx.WriteError(err)
	}
	if !updated {
		ctx.PropagateAs()
		return ctx.WriteInteger(0)
	}
	return ctx.WriteInteger(1)
}

func hllCacheWriteAllowed(ctx *Context) bool {
	r := ctx.dispatcher()
	if r == nil {
		return true
	}
	r.mu.RLock()
	check := r.writeCheck
	r.mu.RUnlock()
	return check == nil || check("PFCOUNT", ctx.Args) == nil
}

func cmdPFCOUNT(ctx *Context) error {
	ctx.writeEffectsHandled = true
	if ctx.ArgCount() < 1 {
		return ctx.WriteError(ErrWrongArgCount)
	}
	ctx.PropagateAs()
	if ctx.ArgCount() > 1 {
		merged := &store.HLLState{}
		for _, key := range ctx.Args {
			entry, ok := ctx.Store.Get(string(key))
			if !ok {
				continue
			}
			state, err := decodeHLLValue(entry.Value)
			if err != nil {
				return ctx.WriteError(err)
			}
			merged.Merge(state)
		}
		return ctx.WriteInteger(merged.Count())
	}
	var count int64
	changed := false
	err := ctx.Store.ApplyValuePlan(ctx.ArgString(0), func(value store.Value) (store.ValuePlan, error) {
		changed = false
		if value == nil {
			count = 0
			return store.ValuePlan{}, nil
		}
		data, err := hllValueBytes(value)
		if err != nil {
			return store.ValuePlan{}, err
		}
		if err := store.ValidateHLL(data); err != nil {
			return store.ValuePlan{}, err
		}
		if data[15]&128 == 0 {
			count = int64(binary.LittleEndian.Uint64(data[8:16]))
			return store.ValuePlan{}, nil
		}
		state, err := store.DecodeHLL(data)
		if err != nil {
			return store.ValuePlan{}, err
		}
		count = state.Count()
		if !hllCacheWriteAllowed(ctx) {
			return store.ValuePlan{}, nil
		}
		next := append([]byte(nil), data...)
		binary.LittleEndian.PutUint64(next[8:16], uint64(count))
		changed = true
		return hllStringPlan(next), nil
	})
	if err != nil {
		return ctx.WriteError(err)
	}
	if changed {
		ctx.PropagateAs(ExecutedCommand{Name: "PFCOUNT", Args: ctx.Args, DB: ctx.DB})
	}
	return ctx.WriteInteger(count)
}

func cmdPFMERGE(ctx *Context) error {
	ctx.writeEffectsHandled = true
	if ctx.ArgCount() < 1 {
		return ctx.WriteError(ErrWrongArgCount)
	}
	sources := &store.HLLState{}
	for _, key := range ctx.Args[1:] {
		entry, ok := ctx.Store.Get(string(key))
		if !ok {
			continue
		}
		state, err := decodeHLLValue(entry.Value)
		if err != nil {
			return ctx.WriteError(err)
		}
		sources.Merge(state)
	}
	err := ctx.Store.ApplyValuePlan(ctx.ArgString(0), func(value store.Value) (store.ValuePlan, error) {
		state, err := decodeHLLValue(value)
		if err != nil {
			return store.ValuePlan{}, err
		}
		state.Merge(sources)
		data, err := state.Encode(true)
		if err != nil {
			return store.ValuePlan{}, err
		}
		return hllStringPlan(data), nil
	})
	if err != nil {
		return ctx.WriteError(err)
	}
	return ctx.WriteOK()
}
