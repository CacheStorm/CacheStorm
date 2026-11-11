package command

import (
	"crypto/rand"
	"encoding/hex"
	"errors"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/cachestorm/cachestorm/internal/resp"
	"github.com/cachestorm/cachestorm/internal/store"
)

// The CRDT types are keys, per database, persisted and replicated:
//   - an LWW register is a hash of its value and timestamp (value, ts);
//   - a G-counter is a hash marked __gcounter with one field per node;
//   - a PN-counter is a hash marked __pncounter with the fields p:<node> and
//     n:<node>;
//   - a G-set is a set;
//   - an OR-set is a hash marked __orset with a field per live tag of each
//     value and per tombstone, named a or r, the value's length, a colon,
//     the value and the tag.
// Changes are recorded as HSET, HDEL, SADD and DEL, so replay does not
// depend on the clock or on generated tags.

const (
	gcounterMarker  = "__gcounter"
	pncounterMarker = "__pncounter"
	orsetMarker     = "__orset"
)

var (
	lwwKind        = recordKind{name: "register", fields: []string{"value", "ts"}}
	errCRDTAmount  = errors.New("ERR amount must be a non-negative integer")
	errCRDTCounter = errors.New("ERR counter would overflow")
)

// CRDT.LWW.SET key value [timestamp] keeps value when its timestamp (unix
// nanoseconds by default) is later than the register's.
func cmdCRDTLWWSET(ctx *Context) error {
	if ctx.ArgCount() < 2 || ctx.ArgCount() > 3 {
		return ctx.WriteError(ErrWrongArgCount)
	}
	key := ctx.ArgString(0)
	ts := time.Now().UnixNano()
	if ctx.ArgCount() == 3 {
		var err error
		if ts, err = strconv.ParseInt(ctx.ArgString(2), 10, 64); err != nil {
			return ctx.WriteError(ErrNotInteger)
		}
	}
	r, err := lwwKind.read(ctx, key)
	if err != nil {
		return ctx.WriteError(err)
	}
	if r != nil {
		cur := r.int("ts")
		if r.bad {
			return ctx.WriteError(store.ErrWrongType)
		}
		if ts <= cur {
			ctx.PropagateAs()
			return ctx.WriteOK()
		}
	}
	if err := updateRecord(ctx, key, "value", ctx.ArgString(1), "ts", i64(ts)); err != nil {
		return ctx.WriteError(err)
	}
	return ctx.WriteOK()
}

func cmdCRDTLWWGET(ctx *Context) error {
	if ctx.ArgCount() != 1 {
		return ctx.WriteError(ErrWrongArgCount)
	}
	r, err := lwwKind.read(ctx, ctx.ArgString(0))
	if err != nil {
		return ctx.WriteError(err)
	}
	if r == nil {
		return ctx.WriteNull()
	}
	return ctx.WriteBulkString(r.vals["value"])
}

func cmdCRDTLWWDELETE(ctx *Context) error {
	if ctx.ArgCount() != 1 {
		return ctx.WriteError(ErrWrongArgCount)
	}
	return deleteRecord(ctx, lwwKind)
}

// counterFields reads the integer fields of a marked counter hash.
func counterFields(ctx *Context, key, marker string) (map[string]int64, error) {
	h, err := readMarkedHash(ctx, key, marker)
	if err != nil || h == nil {
		return nil, err
	}
	out := map[string]int64{}
	for f, raw := range markedFields(h, marker) {
		n, err := strconv.ParseInt(string(raw), 10, 64)
		if err != nil || n < 0 {
			return nil, store.ErrWrongType
		}
		out[f] = n
	}
	return out, nil
}

// bumpCounter adds amount to field of a marked counter hash and returns the
// field's new value.
func bumpCounter(ctx *Context, key, marker, field string, amount int64) (int64, error) {
	fields, err := counterFields(ctx, key, marker)
	if err != nil {
		return 0, err
	}
	n := fields[field]
	if n > (1<<63-1)-amount {
		return 0, errCRDTCounter
	}
	n += amount
	return n, setMarkedFields(ctx, key, marker, field, i64(n))
}

// CRDT.GCOUNTER.INCR name node [amount] adds amount (1 by default) to the
// node's count and replies it.
func cmdCRDTGCOUNTERINCR(ctx *Context) error {
	if ctx.ArgCount() < 2 || ctx.ArgCount() > 3 {
		return ctx.WriteError(ErrWrongArgCount)
	}
	amount := int64(1)
	if ctx.ArgCount() == 3 {
		var err error
		if amount, err = strconv.ParseInt(ctx.ArgString(2), 10, 64); err != nil || amount < 0 {
			return ctx.WriteError(errCRDTAmount)
		}
	}
	n, err := bumpCounter(ctx, ctx.ArgString(0), gcounterMarker, ctx.ArgString(1), amount)
	if err != nil {
		return ctx.WriteError(err)
	}
	return ctx.WriteInteger(n)
}

func cmdCRDTGCOUNTERGET(ctx *Context) error {
	if ctx.ArgCount() != 1 {
		return ctx.WriteError(ErrWrongArgCount)
	}
	fields, err := counterFields(ctx, ctx.ArgString(0), gcounterMarker)
	if err != nil {
		return ctx.WriteError(err)
	}
	var total int64
	for _, n := range fields {
		total += n
	}
	return ctx.WriteInteger(total)
}

func pnCounterBump(ctx *Context, prefix string) error {
	if ctx.ArgCount() != 2 {
		return ctx.WriteError(ErrWrongArgCount)
	}
	if _, err := bumpCounter(ctx, ctx.ArgString(0), pncounterMarker, prefix+ctx.ArgString(1), 1); err != nil {
		return ctx.WriteError(err)
	}
	return ctx.WriteOK()
}

func cmdCRDTPNCOUNTERINCR(ctx *Context) error { return pnCounterBump(ctx, "p:") }
func cmdCRDTPNCOUNTERDECR(ctx *Context) error { return pnCounterBump(ctx, "n:") }

func cmdCRDTPNCOUNTERGET(ctx *Context) error {
	if ctx.ArgCount() != 1 {
		return ctx.WriteError(ErrWrongArgCount)
	}
	fields, err := counterFields(ctx, ctx.ArgString(0), pncounterMarker)
	if err != nil {
		return ctx.WriteError(err)
	}
	var total int64
	for f, n := range fields {
		switch {
		case strings.HasPrefix(f, "p:"):
			total += n
		case strings.HasPrefix(f, "n:"):
			total -= n
		default:
			return ctx.WriteError(store.ErrWrongType)
		}
	}
	return ctx.WriteInteger(total)
}

// CRDT.GSET.ADD name value is SADD of one member.
func cmdCRDTGSETADD(ctx *Context) error {
	if ctx.ArgCount() != 2 {
		return ctx.WriteError(ErrWrongArgCount)
	}
	return recordedAs(ctx, "SADD", func() error { return cmdSADD(ctx) })
}

// CRDT.GSET.GET name replies the members, sorted.
func cmdCRDTGSETGET(ctx *Context) error {
	if ctx.ArgCount() != 1 {
		return ctx.WriteError(ErrWrongArgCount)
	}
	entry, ok := ctx.Store.Get(ctx.ArgString(0))
	if !ok {
		return ctx.WriteArray([]*resp.Value{})
	}
	set, ok := entry.Value.(*store.SetValue)
	if !ok {
		return ctx.WriteError(store.ErrWrongType)
	}
	set.RLock()
	var members []string
	for m := range set.All() {
		members = append(members, m)
	}
	set.RUnlock()
	sort.Strings(members)
	out := make([]*resp.Value, len(members))
	for i, m := range members {
		out[i] = resp.BulkString(m)
	}
	return ctx.WriteArray(out)
}

func orsetField(kind byte, value, tag string) string {
	return string(kind) + strconv.Itoa(len(value)) + ":" + value + tag
}

type orset struct {
	live, dead map[string]map[string]bool
}

func (s *orset) tags(value string) []string {
	tags := make([]string, 0, len(s.live[value]))
	for t := range s.live[value] {
		tags = append(tags, t)
	}
	sort.Strings(tags)
	return tags
}

func readORSet(ctx *Context, key string) (*orset, error) {
	h, err := readMarkedHash(ctx, key, orsetMarker)
	if err != nil {
		return nil, err
	}
	s := &orset{live: map[string]map[string]bool{}, dead: map[string]map[string]bool{}}
	if h == nil {
		return s, nil
	}
	for f := range markedFields(h, orsetMarker) {
		colon := strings.IndexByte(f, ':')
		if len(f) < 3 || colon < 2 {
			return nil, store.ErrWrongType
		}
		n, err := strconv.Atoi(f[1:colon])
		if err != nil || n < 0 || colon+1+n > len(f) {
			return nil, store.ErrWrongType
		}
		value, tag := f[colon+1:colon+1+n], f[colon+1+n:]
		into := s.live
		switch f[0] {
		case 'a':
		case 'r':
			into = s.dead
		default:
			return nil, store.ErrWrongType
		}
		if into[value] == nil {
			into[value] = map[string]bool{}
		}
		into[value][tag] = true
	}
	return s, nil
}

// CRDT.ORSET.ADD name value [tag] adds value under tag (a generated one by
// default) unless that tag was removed, and replies the tag.
func cmdCRDTORSETADD(ctx *Context) error {
	if ctx.ArgCount() < 2 || ctx.ArgCount() > 3 {
		return ctx.WriteError(ErrWrongArgCount)
	}
	name, value := ctx.ArgString(0), ctx.ArgString(1)
	var tag string
	if ctx.ArgCount() == 3 {
		if tag = ctx.ArgString(2); tag == "" {
			return ctx.WriteError(errors.New("ERR tag must not be empty"))
		}
	} else {
		var b [8]byte
		_, _ = rand.Read(b[:])
		tag = "auto-" + hex.EncodeToString(b[:])
	}
	s, err := readORSet(ctx, name)
	if err != nil {
		return ctx.WriteError(err)
	}
	if s.dead[value][tag] || s.live[value][tag] {
		ctx.PropagateAs()
		return ctx.WriteBulkString(tag)
	}
	if err := setMarkedFields(ctx, name, orsetMarker, orsetField('a', value, tag), ""); err != nil {
		return ctx.WriteError(err)
	}
	return ctx.WriteBulkString(tag)
}

// CRDT.ORSET.REMOVE name value [tag ...] removes the observed tags of value
// (all of its live tags by default), keeping them as tombstones, and
// replies how many live tags it removed.
func cmdCRDTORSETREMOVE(ctx *Context) error {
	if ctx.ArgCount() < 2 {
		return ctx.WriteError(ErrWrongArgCount)
	}
	name, value := ctx.ArgString(0), ctx.ArgString(1)
	s, err := readORSet(ctx, name)
	if err != nil {
		return ctx.WriteError(err)
	}
	observed := s.tags(value)
	if ctx.ArgCount() > 2 {
		observed = observed[:0]
		for i := 2; i < ctx.ArgCount(); i++ {
			observed = append(observed, ctx.ArgString(i))
		}
	}
	var pairs, gone []string
	removed := int64(0)
	for _, tag := range observed {
		if !s.dead[value][tag] {
			pairs = append(pairs, orsetField('r', value, tag), "")
		}
		if s.live[value][tag] {
			gone = append(gone, orsetField('a', value, tag))
			removed++
		}
	}
	if len(pairs) == 0 {
		ctx.PropagateAs()
		return ctx.WriteInteger(0)
	}
	if err := setMarkedFields(ctx, name, orsetMarker, pairs...); err != nil {
		return ctx.WriteError(err)
	}
	recorded, _ := ctx.Propagation()
	if len(gone) > 0 {
		deleteMarkedFields(ctx, name, orsetMarker, gone...)
		hdel, _ := ctx.Propagation()
		ctx.PropagateAs(append(recorded, hdel...)...)
	}
	return ctx.WriteInteger(removed)
}

func cmdCRDTORSETGET(ctx *Context) error {
	if ctx.ArgCount() != 1 {
		return ctx.WriteError(ErrWrongArgCount)
	}
	s, err := readORSet(ctx, ctx.ArgString(0))
	if err != nil {
		return ctx.WriteError(err)
	}
	set := make(map[string]struct{}, len(s.live))
	for v := range s.live {
		set[v] = struct{}{}
	}
	return ctx.WriteArray(sortedBulk(set))
}

func cmdCRDTORSETTAGS(ctx *Context) error {
	if ctx.ArgCount() != 2 {
		return ctx.WriteError(ErrWrongArgCount)
	}
	s, err := readORSet(ctx, ctx.ArgString(0))
	if err != nil {
		return ctx.WriteError(err)
	}
	tags := s.tags(ctx.ArgString(1))
	out := make([]*resp.Value, len(tags))
	for i, t := range tags {
		out[i] = resp.BulkString(t)
	}
	return ctx.WriteArray(out)
}
