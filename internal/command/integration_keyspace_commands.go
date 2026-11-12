package command

import (
	"bytes"
	"errors"
	"sort"
	"strconv"
	"strings"

	"github.com/cachestorm/cachestorm/internal/resp"
	"github.com/cachestorm/cachestorm/internal/store"
)

// ARRAY, OBJECT, SEQUENCE, CAPTCHA and the CACHE locks are keys. An array
// is a list, an object a hash and a sequence a string holding an integer,
// so the Redis commands of those types read and change them too; a captcha
// is a hash marked by __captcha (its answer) that expires after five
// minutes, and a CACHE lock is a LOCK. Being keys they are per database,
// persisted and replicated, and changes are recorded as Redis commands.

const (
	captchaMarker    = "__captcha"
	captchaTTL       = 300000
	maxCaptchaLength = 64
)

// runSilent runs handler as runAs does but keeps its reply, which it
// returns; an error reply comes back as an error and records nothing.
func runSilent(ctx *Context, name string, handler func(*Context) error, args ...string) (string, error) {
	saved := ctx.Writer
	var sink bytes.Buffer
	ctx.Writer = resp.NewWriter(&sink)
	err := runAs(ctx, name, handler, args...)
	ctx.Writer = saved
	reply := strings.TrimRight(sink.String(), "\r\n")
	if err == nil && strings.HasPrefix(reply, "-") {
		err = errors.New(reply[1:])
	}
	if err != nil {
		ctx.PropagateAs()
	}
	return reply, err
}

func argsFrom(ctx *Context, from int) []string {
	out := make([]string, 0, ctx.ArgCount()-from)
	for i := from; i < ctx.ArgCount(); i++ {
		out = append(out, ctx.ArgString(i))
	}
	return out
}

// --- ARRAY ---

func listItems(ctx *Context, key string) ([]string, error) {
	l, err := getList(ctx, key)
	if err != nil || l == nil {
		return nil, err
	}
	l.RLock()
	out := make([]string, 0, l.Len())
	l.Range(0, false, func(_ int, el []byte) bool {
		out = append(out, string(el))
		return true
	})
	l.RUnlock()
	return out, nil
}

// replaceList makes key the list of items, deleting it when there are
// none; recorded as DEL and RPUSH.
func replaceList(ctx *Context, key string, items []string) error {
	if len(items) == 0 {
		ctx.Store.Delete(key)
		ctx.notify(store.NotifyGeneric, "del", key)
		ctx.propagateDelete(key)
		return nil
	}
	l := newList()
	elems := make([][]byte, len(items))
	for i, it := range items {
		elems[i] = []byte(it)
	}
	l.PushBack(elems...)
	if err := ctx.Store.Set(key, l, store.SetOptions{}); err != nil {
		return err
	}
	ctx.notify(store.NotifyList, "rpush", key)
	ctx.PropagateAs(
		ExecutedCommand{Name: "DEL", Args: [][]byte{[]byte(key)}, DB: ctx.DB},
		ExecutedCommand{Name: "RPUSH", Args: pairArgs(key, items), DB: ctx.DB},
	)
	return nil
}

// rewriteList applies fn to the items of the list at key and stores the
// result; a missing key is left alone.
func rewriteList(ctx *Context, fn func([]string) []string) (bool, error) {
	key := ctx.ArgString(0)
	items, err := listItems(ctx, key)
	if err != nil || items == nil {
		ctx.PropagateAs()
		return false, err
	}
	return true, replaceList(ctx, key, fn(items))
}

func cmdARRAYPUSH(ctx *Context) error {
	if ctx.ArgCount() < 2 {
		return ctx.WriteError(ErrWrongArgCount)
	}
	return runAs(ctx, "RPUSH", cmdRPUSH, argsFrom(ctx, 0)...)
}

func cmdARRAYPOP(ctx *Context) error {
	if ctx.ArgCount() != 1 {
		return ctx.WriteError(ErrWrongArgCount)
	}
	return runAs(ctx, "RPOP", cmdRPOP, ctx.ArgString(0))
}

func cmdARRAYSHIFT(ctx *Context) error {
	if ctx.ArgCount() != 1 {
		return ctx.WriteError(ErrWrongArgCount)
	}
	return runAs(ctx, "LPOP", cmdLPOP, ctx.ArgString(0))
}

// ARRAY.UNSHIFT name value [value ...] puts the values at the head in the
// order given: the new length.
func cmdARRAYUNSHIFT(ctx *Context) error {
	if ctx.ArgCount() < 2 {
		return ctx.WriteError(ErrWrongArgCount)
	}
	args := []string{ctx.ArgString(0)}
	for i := ctx.ArgCount() - 1; i >= 1; i-- {
		args = append(args, ctx.ArgString(i))
	}
	return runAs(ctx, "LPUSH", cmdLPUSH, args...)
}

func parseInts(args ...string) ([]int, error) {
	out := make([]int, len(args))
	for i, a := range args {
		n, err := strconv.Atoi(a)
		if err != nil {
			return nil, ErrNotInteger
		}
		out[i] = n
	}
	return out, nil
}

// ARRAY.SLICE name start end: the items from start up to end, exclusive.
func cmdARRAYSLICE(ctx *Context) error {
	if ctx.ArgCount() != 3 {
		return ctx.WriteError(ErrWrongArgCount)
	}
	r, err := parseInts(ctx.ArgString(1), ctx.ArgString(2))
	if err != nil {
		return ctx.WriteError(err)
	}
	items, err := listItems(ctx, ctx.ArgString(0))
	if err != nil {
		return ctx.WriteError(err)
	}
	start, end := max(r[0], 0), min(r[1], len(items))
	if start >= end {
		return ctx.WriteArray([]*resp.Value{})
	}
	return ctx.WriteArray(bulkStrings(items[start:end]))
}

// ARRAY.SPLICE name start count [item ...] removes count items from start
// and inserts the items there: the removed items.
func cmdARRAYSPLICE(ctx *Context) error {
	if ctx.ArgCount() < 3 {
		return ctx.WriteError(ErrWrongArgCount)
	}
	r, err := parseInts(ctx.ArgString(1), ctx.ArgString(2))
	if err != nil {
		return ctx.WriteError(err)
	}
	var removed []string
	_, err = rewriteList(ctx, func(items []string) []string {
		start := min(max(r[0], 0), len(items))
		count := min(max(r[1], 0), len(items)-start)
		removed = append(removed, items[start:start+count]...)
		out := append([]string{}, items[:start]...)
		out = append(out, argsFrom(ctx, 3)...)
		return append(out, items[start+count:]...)
	})
	if err != nil {
		return ctx.WriteError(err)
	}
	return ctx.WriteArray(bulkStrings(removed))
}

func cmdARRAYREVERSE(ctx *Context) error {
	if ctx.ArgCount() != 1 {
		return ctx.WriteError(ErrWrongArgCount)
	}
	_, err := rewriteList(ctx, func(items []string) []string {
		for i, j := 0, len(items)-1; i < j; i, j = i+1, j-1 {
			items[i], items[j] = items[j], items[i]
		}
		return items
	})
	if err != nil {
		return ctx.WriteError(err)
	}
	return ctx.WriteOK()
}

// ARRAY.SORT name [ASC|DESC] sorts the items by their bytes.
func cmdARRAYSORT(ctx *Context) error {
	if ctx.ArgCount() != 1 && ctx.ArgCount() != 2 {
		return ctx.WriteError(ErrWrongArgCount)
	}
	desc := false
	if ctx.ArgCount() == 2 {
		switch strings.ToUpper(ctx.ArgString(1)) {
		case "ASC":
		case "DESC":
			desc = true
		default:
			return ctx.WriteError(ErrSyntaxError)
		}
	}
	_, err := rewriteList(ctx, func(items []string) []string {
		sort.SliceStable(items, func(i, j int) bool {
			if desc {
				return items[i] > items[j]
			}
			return items[i] < items[j]
		})
		return items
	})
	if err != nil {
		return ctx.WriteError(err)
	}
	return ctx.WriteOK()
}

// ARRAY.UNIQUE name keeps the first of each item: how many it removed.
func cmdARRAYUNIQUE(ctx *Context) error {
	if ctx.ArgCount() != 1 {
		return ctx.WriteError(ErrWrongArgCount)
	}
	items, err := listItems(ctx, ctx.ArgString(0))
	if err != nil {
		return ctx.WriteError(err)
	}
	seen := make(map[string]bool, len(items))
	unique := make([]string, 0, len(items))
	for _, v := range items {
		if !seen[v] {
			seen[v] = true
			unique = append(unique, v)
		}
	}
	removed := len(items) - len(unique)
	if removed == 0 {
		ctx.PropagateAs()
		return ctx.WriteInteger(0)
	}
	if err := replaceList(ctx, ctx.ArgString(0), unique); err != nil {
		return ctx.WriteError(err)
	}
	return ctx.WriteInteger(int64(removed))
}

// ARRAY.MERGE dest src appends src's items to dest: dest's length.
func cmdARRAYMERGE(ctx *Context) error {
	if ctx.ArgCount() != 2 {
		return ctx.WriteError(ErrWrongArgCount)
	}
	dest := ctx.ArgString(0)
	src, err := listItems(ctx, ctx.ArgString(1))
	if err == nil && len(src) > 0 {
		return runAs(ctx, "RPUSH", cmdRPUSH, append([]string{dest}, src...)...)
	}
	ctx.PropagateAs()
	if err != nil {
		return ctx.WriteError(err)
	}
	cur, err := listItems(ctx, dest)
	if err != nil {
		return ctx.WriteError(err)
	}
	return ctx.WriteInteger(int64(len(cur)))
}

func twoLists(ctx *Context) ([]string, map[string]bool, error) {
	a, err := listItems(ctx, ctx.ArgString(0))
	if err != nil {
		return nil, nil, err
	}
	b, err := listItems(ctx, ctx.ArgString(1))
	if err != nil {
		return nil, nil, err
	}
	in := make(map[string]bool, len(b))
	for _, v := range b {
		in[v] = true
	}
	return a, in, nil
}

// ARRAY.INTERSECT a b: a's distinct items that b has, in a's order.
func cmdARRAYINTERSECT(ctx *Context) error {
	if ctx.ArgCount() != 2 {
		return ctx.WriteError(ErrWrongArgCount)
	}
	a, inB, err := twoLists(ctx)
	if err != nil {
		return ctx.WriteError(err)
	}
	var out []string
	for _, v := range a {
		if inB[v] {
			out = append(out, v)
			delete(inB, v)
		}
	}
	return ctx.WriteArray(bulkStrings(out))
}

// ARRAY.DIFF a b: a's items that b does not have, in a's order.
func cmdARRAYDIFF(ctx *Context) error {
	if ctx.ArgCount() != 2 {
		return ctx.WriteError(ErrWrongArgCount)
	}
	a, inB, err := twoLists(ctx)
	if err != nil {
		return ctx.WriteError(err)
	}
	var out []string
	for _, v := range a {
		if !inB[v] {
			out = append(out, v)
		}
	}
	return ctx.WriteArray(bulkStrings(out))
}

func arrayFind(ctx *Context, last bool) (int, error) {
	items, err := listItems(ctx, ctx.ArgString(0))
	if err != nil {
		return -1, err
	}
	value := ctx.ArgString(1)
	found := -1
	for i, v := range items {
		if v == value {
			found = i
			if !last {
				break
			}
		}
	}
	return found, nil
}

func arrayIndex(ctx *Context, last bool) error {
	if ctx.ArgCount() != 2 {
		return ctx.WriteError(ErrWrongArgCount)
	}
	i, err := arrayFind(ctx, last)
	if err != nil {
		return ctx.WriteError(err)
	}
	return ctx.WriteInteger(int64(i))
}

func cmdARRAYINDEXOF(ctx *Context) error     { return arrayIndex(ctx, false) }
func cmdARRAYLASTINDEXOF(ctx *Context) error { return arrayIndex(ctx, true) }

func cmdARRAYINCLUDES(ctx *Context) error {
	if ctx.ArgCount() != 2 {
		return ctx.WriteError(ErrWrongArgCount)
	}
	i, err := arrayFind(ctx, false)
	if err != nil {
		return ctx.WriteError(err)
	}
	return ctx.WriteInteger(boolInt(i >= 0))
}

// --- OBJECT ---

func objectFields(ctx *Context, key string) (map[string]string, error) {
	h, err := getHash(ctx, key)
	if err != nil || h == nil {
		return nil, err
	}
	h.RLock()
	raw := h.Fields()
	h.RUnlock()
	out := make(map[string]string, len(raw))
	for f, v := range raw {
		out[f] = string(v)
	}
	return out, nil
}

func objectReply(ctx *Context, keys, values bool) error {
	if ctx.ArgCount() != 1 {
		return ctx.WriteError(ErrWrongArgCount)
	}
	obj, err := objectFields(ctx, ctx.ArgString(0))
	if err != nil {
		return ctx.WriteError(err)
	}
	out := make([]string, 0, 2*len(obj))
	for _, k := range sortedKeys(obj) {
		if keys {
			out = append(out, k)
		}
		if values {
			out = append(out, obj[k])
		}
	}
	return ctx.WriteArray(bulkStrings(out))
}

func cmdOBJECTKEYS(ctx *Context) error    { return objectReply(ctx, true, false) }
func cmdOBJECTVALUES(ctx *Context) error  { return objectReply(ctx, false, true) }
func cmdOBJECTENTRIES(ctx *Context) error { return objectReply(ctx, true, true) }

// OBJECT.FROMENTRIES name field value [field value ...] replaces the object.
func cmdOBJECTFROMENTRIES(ctx *Context) error {
	if ctx.ArgCount() < 3 || (ctx.ArgCount()-1)%2 != 0 {
		return ctx.WriteError(ErrWrongArgCount)
	}
	name := ctx.ArgString(0)
	if _, err := getHash(ctx, name); err != nil {
		return ctx.WriteError(err)
	}
	pairs := argsFrom(ctx, 1)
	fields := make(map[string]string, len(pairs)/2)
	for i := 0; i < len(pairs); i += 2 {
		fields[pairs[i]] = pairs[i+1]
	}
	dedup := make([]string, 0, 2*len(fields))
	for _, f := range sortedKeys(fields) {
		dedup = append(dedup, f, fields[f])
	}
	if err := replaceRecord(ctx, name, dedup...); err != nil {
		return ctx.WriteError(err)
	}
	return ctx.WriteOK()
}

// OBJECT.MERGE dest src copies src's fields into dest: how many.
func cmdOBJECTMERGE(ctx *Context) error {
	if ctx.ArgCount() != 2 {
		return ctx.WriteError(ErrWrongArgCount)
	}
	dest := ctx.ArgString(0)
	if _, err := getHash(ctx, dest); err != nil {
		return ctx.WriteError(err)
	}
	src, err := objectFields(ctx, ctx.ArgString(1))
	if err != nil {
		return ctx.WriteError(err)
	}
	if len(src) == 0 {
		ctx.PropagateAs()
		return ctx.WriteInteger(0)
	}
	pairs := make([]string, 0, 2*len(src))
	for _, f := range sortedKeys(src) {
		pairs = append(pairs, f, src[f])
	}
	if err := updateRecord(ctx, dest, pairs...); err != nil {
		return ctx.WriteError(err)
	}
	return ctx.WriteInteger(int64(len(src)))
}

// objectDelete removes fields from the object: how many it removed.
func objectDelete(ctx *Context, name string, fields []string) (int64, error) {
	if len(fields) == 0 {
		ctx.PropagateAs()
		return 0, nil
	}
	reply, err := runSilent(ctx, "HDEL", cmdHDEL, append([]string{name}, fields...)...)
	if err != nil {
		return 0, err
	}
	n, err := strconv.ParseInt(strings.TrimPrefix(reply, ":"), 10, 64)
	if err != nil || n == 0 {
		ctx.PropagateAs()
	}
	return n, err
}

// OBJECT.PICK name field [field ...] keeps only those fields: how many it
// kept.
func cmdOBJECTPICK(ctx *Context) error {
	if ctx.ArgCount() < 2 {
		return ctx.WriteError(ErrWrongArgCount)
	}
	name := ctx.ArgString(0)
	obj, err := objectFields(ctx, name)
	if err != nil {
		return ctx.WriteError(err)
	}
	keep := make(map[string]bool)
	for _, f := range argsFrom(ctx, 1) {
		keep[f] = true
	}
	var drop []string
	kept := 0
	for _, f := range sortedKeys(obj) {
		if keep[f] {
			kept++
		} else {
			drop = append(drop, f)
		}
	}
	if _, err := objectDelete(ctx, name, drop); err != nil {
		return ctx.WriteError(err)
	}
	return ctx.WriteInteger(int64(kept))
}

// OBJECT.OMIT name field [field ...] removes those fields: how many.
func cmdOBJECTOMIT(ctx *Context) error {
	if ctx.ArgCount() < 2 {
		return ctx.WriteError(ErrWrongArgCount)
	}
	obj, err := objectFields(ctx, ctx.ArgString(0))
	if err != nil {
		return ctx.WriteError(err)
	}
	seen := make(map[string]bool)
	var drop []string
	for _, f := range argsFrom(ctx, 1) {
		if _, ok := obj[f]; ok && !seen[f] {
			seen[f] = true
			drop = append(drop, f)
		}
	}
	n, err := objectDelete(ctx, ctx.ArgString(0), drop)
	if err != nil {
		return ctx.WriteError(err)
	}
	return ctx.WriteInteger(n)
}

func cmdOBJECTHAS(ctx *Context) error {
	if ctx.ArgCount() != 2 {
		return ctx.WriteError(ErrWrongArgCount)
	}
	obj, err := objectFields(ctx, ctx.ArgString(0))
	if err != nil {
		return ctx.WriteError(err)
	}
	_, ok := obj[ctx.ArgString(1)]
	return ctx.WriteInteger(boolInt(ok))
}

func cmdOBJECTGET(ctx *Context) error {
	if ctx.ArgCount() != 2 {
		return ctx.WriteError(ErrWrongArgCount)
	}
	obj, err := objectFields(ctx, ctx.ArgString(0))
	if err != nil {
		return ctx.WriteError(err)
	}
	v, ok := obj[ctx.ArgString(1)]
	if !ok {
		return ctx.WriteNull()
	}
	return ctx.WriteBulkString(v)
}

func cmdOBJECTSET(ctx *Context) error {
	if ctx.ArgCount() != 3 {
		return ctx.WriteError(ErrWrongArgCount)
	}
	if err := updateRecord(ctx, ctx.ArgString(0), ctx.ArgString(1), ctx.ArgString(2)); err != nil {
		return ctx.WriteError(err)
	}
	return ctx.WriteOK()
}

func cmdOBJECTDELETE(ctx *Context) error {
	if ctx.ArgCount() != 2 {
		return ctx.WriteError(ErrWrongArgCount)
	}
	n, err := objectDelete(ctx, ctx.ArgString(0), []string{ctx.ArgString(1)})
	if err != nil {
		return ctx.WriteError(err)
	}
	return ctx.WriteInteger(n)
}

// --- CACHE locks ---

func cmdCACHELOCK(ctx *Context) error    { return cmdLOCKXACQUIRE(ctx) }
func cmdCACHEUNLOCK(ctx *Context) error  { return cmdLOCKXRELEASE(ctx) }
func cmdCACHEREFRESH(ctx *Context) error { return cmdLOCKXEXTEND(ctx) }

func cmdCACHELOCKED(ctx *Context) error {
	if ctx.ArgCount() != 1 {
		return ctx.WriteError(ErrWrongArgCount)
	}
	l, err := readLock(ctx, ctx.ArgString(0))
	if err != nil {
		return ctx.WriteError(err)
	}
	return ctx.WriteInteger(boolInt(l != nil))
}

// --- CAPTCHA ---

// CAPTCHA.GENERATE [length] makes a captcha of length digits (6 by
// default): its id and answer.
func cmdCAPTCHAGENERATE(ctx *Context) error {
	if ctx.ArgCount() > 1 {
		return ctx.WriteError(ErrWrongArgCount)
	}
	length := 6
	if ctx.ArgCount() == 1 {
		n, err := strconv.Atoi(ctx.ArgString(0))
		if err != nil || n < 1 || n > maxCaptchaLength {
			return ctx.WriteError(errors.New("ERR length must be between 1 and 64"))
		}
		length = n
	}
	digits := make([]byte, length)
	for i := range digits {
		digits[i] = byte('0' + randomInt(10))
	}
	id := newObjectID(ctx)
	if err := setExpiring(ctx, id, captchaTTL, captchaMarker, string(digits)); err != nil {
		return ctx.WriteError(err)
	}
	return ctx.WriteArray([]*resp.Value{
		resp.BulkString("id"), resp.BulkString(id),
		resp.BulkString("answer"), resp.BulkString(string(digits)),
	})
}

// CAPTCHA.VERIFY id answer: 1 and the captcha is used up when the answer
// is right, else 0.
func cmdCAPTCHAVERIFY(ctx *Context) error {
	if ctx.ArgCount() != 2 {
		return ctx.WriteError(ErrWrongArgCount)
	}
	id := ctx.ArgString(0)
	r, err := readMarked(ctx, id, captchaMarker)
	if err != nil {
		return ctx.WriteError(err)
	}
	if r == nil || r[captchaMarker] != ctx.ArgString(1) {
		ctx.PropagateAs()
		return ctx.WriteInteger(0)
	}
	ctx.Store.Delete(id)
	ctx.notify(store.NotifyGeneric, "del", id)
	ctx.propagateDelete(id)
	return ctx.WriteInteger(1)
}

// --- SEQUENCE ---

// SEQUENCE.NEXT name [increment] adds increment (1 by default): the new
// value.
func cmdSEQUENCENEXT(ctx *Context) error {
	if ctx.ArgCount() != 1 && ctx.ArgCount() != 2 {
		return ctx.WriteError(ErrWrongArgCount)
	}
	by := "1"
	if ctx.ArgCount() == 2 {
		by = ctx.ArgString(1)
	}
	return runAs(ctx, "INCRBY", cmdINCRBY, ctx.ArgString(0), by)
}

// SEQUENCE.CURRENT name: the value, 0 for a missing sequence.
func cmdSEQUENCECURRENT(ctx *Context) error {
	if ctx.ArgCount() != 1 {
		return ctx.WriteError(ErrWrongArgCount)
	}
	entry, ok := ctx.Store.Get(ctx.ArgString(0))
	if !ok {
		return ctx.WriteInteger(0)
	}
	n, err := sequenceValue(entry)
	if err != nil {
		return ctx.WriteError(err)
	}
	return ctx.WriteInteger(n)
}

func sequenceValue(e *store.Entry) (int64, error) {
	b, ok := stringValueOf(e.Value)
	if !ok {
		return 0, store.ErrWrongType
	}
	n, err := strconv.ParseInt(string(b), 10, 64)
	if err != nil {
		return 0, ErrNotInteger
	}
	return n, nil
}

func setSequence(ctx *Context, value string) error {
	if _, err := strconv.ParseInt(value, 10, 64); err != nil {
		return ctx.WriteError(ErrNotInteger)
	}
	if entry, ok := ctx.Store.Get(ctx.ArgString(0)); ok {
		if _, ok := stringValueOf(entry.Value); !ok {
			return ctx.WriteError(store.ErrWrongType)
		}
	}
	return runAs(ctx, "SET", cmdSET, ctx.ArgString(0), value)
}

func cmdSEQUENCERESET(ctx *Context) error {
	if ctx.ArgCount() != 1 {
		return ctx.WriteError(ErrWrongArgCount)
	}
	return setSequence(ctx, "0")
}

func cmdSEQUENCESET(ctx *Context) error {
	if ctx.ArgCount() != 2 {
		return ctx.WriteError(ErrWrongArgCount)
	}
	return setSequence(ctx, ctx.ArgString(1))
}
