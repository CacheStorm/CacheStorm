package command

import (
	"sort"

	"github.com/cachestorm/cachestorm/internal/resp"
	"github.com/cachestorm/cachestorm/internal/store"
)

// The set operations follow Redis' sunionDiffGenericCommand and
// sinterGenericCommand: the result is built by adding members, in the order
// the source sets list them, to a set that starts as an intset, so its order
// and encoding are the ones Redis gives it.

const (
	setOpUnion = iota
	setOpDiff
	setOpInter
)

// setOpSources holds the sets an operation reads, nil for a missing key,
// read-locked together in key order so the result is one snapshot.
type setOpSources struct {
	ctx    *Context
	sets   []*store.SetValue
	locked []*store.SetValue
}

// openSetOpSources resolves keys and locks the sets. On error nothing is
// left locked.
func openSetOpSources(ctx *Context, keys []string) (*setOpSources, error) {
	ctx.Store.SetOpsRLock()
	src := &setOpSources{ctx: ctx, sets: make([]*store.SetValue, len(keys))}
	type keyed struct {
		key string
		set *store.SetValue
	}
	var order []keyed
	seen := make(map[*store.SetValue]bool, len(keys))
	for i, key := range keys {
		set, err := getSet(ctx, key)
		if err != nil {
			ctx.Store.SetOpsRUnlock()
			return nil, err
		}
		src.sets[i] = set
		if set != nil && !seen[set] {
			seen[set] = true
			order = append(order, keyed{key, set})
		}
	}
	sort.Slice(order, func(i, j int) bool { return order[i].key < order[j].key })
	for _, k := range order {
		k.set.RLock()
		src.locked = append(src.locked, k.set)
	}
	return src, nil
}

func (s *setOpSources) close() {
	for _, set := range s.locked {
		set.RUnlock()
	}
	s.ctx.Store.SetOpsRUnlock()
}

// unionDiff is sunionDiffGenericCommand's computation.
func (s *setOpSources) unionDiff(op int) *store.SetValue {
	dst := store.NewIntsetSet()
	sets := s.sets
	switch op {
	case setOpUnion:
		for _, set := range sets {
			if set == nil {
				continue
			}
			set.Range(func(m string) bool {
				dst.AddMember(m)
				return true
			})
		}
	case setOpDiff:
		if sets[0] == nil {
			return dst
		}
		for _, set := range sets[1:] {
			if set == sets[0] {
				return dst
			}
		}
		var one, two int
		for _, set := range sets {
			if set != nil {
				one += len(sets[0].Members)
				two += len(set.Members)
			}
		}
		if one/2 <= two {
			sets[0].Range(func(m string) bool {
				for _, other := range sets[1:] {
					if other != nil {
						if _, ok := other.Members[m]; ok {
							return true
						}
					}
				}
				dst.AddMember(m)
				return true
			})
		} else {
			sets[0].Range(func(m string) bool {
				dst.AddMember(m)
				return true
			})
			for _, other := range sets[1:] {
				if other == nil {
					continue
				}
				other.Range(func(m string) bool {
					dst.RemoveMember(m)
					return true
				})
			}
		}
	}
	return dst
}

// inter is sinterGenericCommand's computation: the members of the smallest
// set found in all the others, in its order. It returns nil when a key is
// missing.
func (s *setOpSources) inter() []string {
	sets := append([]*store.SetValue(nil), s.sets...)
	for _, set := range sets {
		if set == nil {
			return nil
		}
	}
	sort.SliceStable(sets, func(i, j int) bool { return len(sets[i].Members) < len(sets[j].Members) })
	var out []string
	sets[0].Range(func(m string) bool {
		for _, other := range sets[1:] {
			if other == sets[0] {
				continue
			}
			if _, ok := other.Members[m]; !ok {
				return true
			}
		}
		out = append(out, m)
		return true
	})
	return out
}

func setMembersReply(set *store.SetValue) []*resp.Value {
	out := make([]*resp.Value, 0, len(set.Members))
	set.Range(func(m string) bool {
		out = append(out, resp.BulkString(m))
		return true
	})
	return out
}

func setOpKeys(ctx *Context, from int) []string {
	keys := make([]string, 0, ctx.ArgCount()-from)
	for i := from; i < ctx.ArgCount(); i++ {
		keys = append(keys, ctx.ArgString(i))
	}
	return keys
}

// setOpCommand answers SUNION, SDIFF and SINTER.
func setOpCommand(ctx *Context, op int) error {
	if ctx.ArgCount() < 1 {
		return ctx.WriteError(arityError(ctx.Command))
	}
	src, err := openSetOpSources(ctx, setOpKeys(ctx, 0))
	if err != nil {
		return ctx.WriteError(err)
	}
	var reply []*resp.Value
	if op == setOpInter {
		members := src.inter()
		reply = make([]*resp.Value, len(members))
		for i, m := range members {
			reply[i] = resp.BulkString(m)
		}
	} else {
		reply = setMembersReply(src.unionDiff(op))
	}
	src.close()
	return ctx.WriteSet(reply)
}

// setOpStoreCommand answers SUNIONSTORE, SDIFFSTORE and SINTERSTORE.
func setOpStoreCommand(ctx *Context, op int, event string) error {
	if ctx.ArgCount() < 2 {
		return ctx.WriteError(arityError(ctx.Command))
	}
	ctx.writeEffectsHandled = true
	dstKey := ctx.ArgString(0)
	src, err := openSetOpSources(ctx, setOpKeys(ctx, 1))
	if err != nil {
		return ctx.WriteError(err)
	}
	var dst *store.SetValue
	if op == setOpInter {
		dst = store.NewIntsetSet()
		for _, m := range src.inter() {
			dst.AddMember(m)
		}
	} else {
		dst = src.unionDiff(op)
	}
	for _, set := range src.locked {
		set.RUnlock()
	}
	src.locked = nil
	defer ctx.Store.SetOpsRUnlock()

	if len(dst.Members) == 0 {
		if ctx.Store.Delete(dstKey) {
			ctx.notify(store.NotifyGeneric, "del", dstKey)
			ctx.Store.KeyNotifier().NotifyKey(dstKey)
		} else {
			ctx.PropagateAs()
		}
		return ctx.WriteInteger(0)
	}
	if err := ctx.Store.Set(dstKey, dst, store.SetOptions{}); err != nil {
		return ctx.WriteError(err)
	}
	ctx.notify(store.NotifySet, event, dstKey)
	return ctx.WriteInteger(int64(len(dst.Members)))
}

func cmdSUNION(ctx *Context) error { return setOpCommand(ctx, setOpUnion) }
func cmdSINTER(ctx *Context) error { return setOpCommand(ctx, setOpInter) }
func cmdSDIFF(ctx *Context) error  { return setOpCommand(ctx, setOpDiff) }

func cmdSUNIONSTORE(ctx *Context) error { return setOpStoreCommand(ctx, setOpUnion, "sunionstore") }
func cmdSINTERSTORE(ctx *Context) error { return setOpStoreCommand(ctx, setOpInter, "sinterstore") }
func cmdSDIFFSTORE(ctx *Context) error  { return setOpStoreCommand(ctx, setOpDiff, "sdiffstore") }
