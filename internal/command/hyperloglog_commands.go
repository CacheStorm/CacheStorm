package command

import (
	"encoding/binary"

	"github.com/cachestorm/cachestorm/internal/store"
)

func RegisterHyperLogLogCommands(router *Router) {
	router.Register(&CommandDef{Name: "PFADD", Handler: cmdPFADD})
	router.Register(&CommandDef{Name: "PFCOUNT", Handler: cmdPFCOUNT})
	router.Register(&CommandDef{Name: "PFMERGE", Handler: cmdPFMERGE})
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
