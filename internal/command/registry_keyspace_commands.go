package command

import (
	"encoding/json"
	"errors"
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"sync"

	"github.com/cachestorm/cachestorm/internal/resp"
	"github.com/cachestorm/cachestorm/internal/store"
)

// VALIDATE, STAGE, CONTEXT, PERMIT, GRANT, TIMER, COUNTERX2, LEVEL, RECORD,
// ENTITY, RELATION, POOLX, STREAMX, CACHEX, STOREX, INDEX, VIEW, AUDITX,
// TOKEN and PROFILE keep each object in a hash key marked by a field naming
// its kind; a BUFFERX buffer is a string key. Objects created with a
// generated id are keyed by that id. Being keys, they are per database,
// persisted and replicated; changes are recorded as HSET, HDEL, SET, APPEND,
// PEXPIREAT and DEL, and the LIST commands cover the selected database and
// reply sorted.

const (
	validateMarker = "__validate"
	stageMarker    = "__stage"
	contextMarker  = "__context"
	permitMarker   = "__permit"
	grantMarker    = "__grant"
	timerMarker    = "__timer"
	counterXMarker = "__counterx2"
	levelMarker    = "__level"
	recordMarker   = "__record"
	entityMarker   = "__entity"
	relationMarker = "__relation"
	poolXMarker    = "__poolx"
	streamXMarker  = "__streamx"
	cacheXMarker   = "__cachex"
	storeXMarker   = "__storex"
	indexMarker    = "__index"
	viewMarker     = "__view"
	auditXMarker   = "__auditx"
	tokenMarker    = "__token"
	profileMarker  = "__profile"
)

// newObjectID returns a generated id no key uses yet.
func newObjectID(ctx *Context) string {
	id := generateUUID()
	for ctx.Store.Exists(id) {
		id = generateUUID()
	}
	return id
}

func listMarked(ctx *Context, marker string) error {
	if ctx.ArgCount() != 0 {
		return ctx.WriteError(ErrWrongArgCount)
	}
	return ctx.WriteArray(bulkStrings(markedKeys(ctx, marker)))
}

// --- attribute bags: CONTEXT, CACHEX, STOREX, RECORD, ENTITY, PROFILE ---

// A bag's attributes are plain fields of its hash; the marker holds the
// bag's header (its name, type or user) and is the one reserved name.

func reservedField(marker, field string) error {
	if field == marker {
		return fmt.Errorf("ERR field name %s is reserved", marker)
	}
	return nil
}

// bagSet sets field of the existing bag at key.
func bagSet(ctx *Context, key, marker, what, field, value string) error {
	if err := reservedField(marker, field); err != nil {
		return ctx.WriteError(err)
	}
	if _, err := loadMarked(ctx, key, marker, what); err != nil {
		return ctx.WriteError(err)
	}
	if err := updateRecord(ctx, key, field, value); err != nil {
		return ctx.WriteError(err)
	}
	return ctx.WriteOK()
}

// bagGet replies the field of the bag at key, null when either is missing.
func bagGet(ctx *Context, marker string) error {
	if ctx.ArgCount() != 2 {
		return ctx.WriteError(ErrWrongArgCount)
	}
	r, err := readMarked(ctx, ctx.ArgString(0), marker)
	if err != nil {
		return ctx.WriteError(err)
	}
	field := ctx.ArgString(1)
	v, ok := r[field]
	if !ok || field == marker {
		return ctx.WriteNull()
	}
	return ctx.WriteBulkString(v)
}

// bagReply replies id, the header under headerName, then the attributes
// sorted.
func bagReply(ctx *Context, marker, what, headerName string) error {
	if ctx.ArgCount() != 1 {
		return ctx.WriteError(ErrWrongArgCount)
	}
	id := ctx.ArgString(0)
	r, err := loadMarked(ctx, id, marker, what)
	if err != nil {
		return ctx.WriteError(err)
	}
	out := []*resp.Value{resp.BulkString("id"), resp.BulkString(id), resp.BulkString(headerName), resp.BulkString(r[marker])}
	delete(r, marker)
	for _, k := range sortedKeys(r) {
		out = append(out, resp.BulkString(k), resp.BulkString(r[k]))
	}
	return ctx.WriteArray(out)
}

func cmdCONTEXTCREATE(ctx *Context) error {
	if ctx.ArgCount() != 1 {
		return ctx.WriteError(ErrWrongArgCount)
	}
	if err := createMarked(ctx, ctx.ArgString(0), contextMarker, contextMarker, "1"); err != nil {
		return ctx.WriteError(err)
	}
	return ctx.WriteOK()
}
func cmdCONTEXTDELETE(ctx *Context) error { return deleteMarked(ctx, contextMarker) }
func cmdCONTEXTSET(ctx *Context) error {
	if ctx.ArgCount() != 3 {
		return ctx.WriteError(ErrWrongArgCount)
	}
	return bagSet(ctx, ctx.ArgString(0), contextMarker, "context", ctx.ArgString(1), ctx.ArgString(2))
}
func cmdCONTEXTGET(ctx *Context) error  { return bagGet(ctx, contextMarker) }
func cmdCONTEXTLIST(ctx *Context) error { return listMarked(ctx, contextMarker) }

func cmdCACHEXCREATE(ctx *Context) error {
	if ctx.ArgCount() != 1 {
		return ctx.WriteError(ErrWrongArgCount)
	}
	if err := createMarked(ctx, ctx.ArgString(0), cacheXMarker, cacheXMarker, "1"); err != nil {
		return ctx.WriteError(err)
	}
	return ctx.WriteOK()
}
func cmdCACHEXDELETE(ctx *Context) error { return deleteMarked(ctx, cacheXMarker) }
func cmdCACHEXSET(ctx *Context) error {
	if ctx.ArgCount() != 3 {
		return ctx.WriteError(ErrWrongArgCount)
	}
	return bagSet(ctx, ctx.ArgString(0), cacheXMarker, "cache", ctx.ArgString(1), ctx.ArgString(2))
}
func cmdCACHEXGET(ctx *Context) error  { return bagGet(ctx, cacheXMarker) }
func cmdCACHEXLIST(ctx *Context) error { return listMarked(ctx, cacheXMarker) }

func cmdSTOREXCREATE(ctx *Context) error {
	if ctx.ArgCount() != 1 {
		return ctx.WriteError(ErrWrongArgCount)
	}
	if err := createMarked(ctx, ctx.ArgString(0), storeXMarker, storeXMarker, "1"); err != nil {
		return ctx.WriteError(err)
	}
	return ctx.WriteOK()
}
func cmdSTOREXDELETE(ctx *Context) error { return deleteMarked(ctx, storeXMarker) }
func cmdSTOREXPUT(ctx *Context) error {
	if ctx.ArgCount() != 3 {
		return ctx.WriteError(ErrWrongArgCount)
	}
	return bagSet(ctx, ctx.ArgString(0), storeXMarker, "store", ctx.ArgString(1), ctx.ArgString(2))
}
func cmdSTOREXGET(ctx *Context) error  { return bagGet(ctx, storeXMarker) }
func cmdSTOREXLIST(ctx *Context) error { return listMarked(ctx, storeXMarker) }

// RECORD.CREATE name replies the new record's id.
func cmdRECORDCREATE(ctx *Context) error {
	if ctx.ArgCount() != 1 {
		return ctx.WriteError(ErrWrongArgCount)
	}
	id := newObjectID(ctx)
	if err := replaceRecord(ctx, id, recordMarker, ctx.ArgString(0)); err != nil {
		return ctx.WriteError(err)
	}
	return ctx.WriteBulkString(id)
}
func cmdRECORDADD(ctx *Context) error {
	if ctx.ArgCount() != 3 {
		return ctx.WriteError(ErrWrongArgCount)
	}
	return bagSet(ctx, ctx.ArgString(0), recordMarker, "record", ctx.ArgString(1), ctx.ArgString(2))
}
func cmdRECORDGET(ctx *Context) error    { return bagReply(ctx, recordMarker, "record", "name") }
func cmdRECORDDELETE(ctx *Context) error { return deleteMarked(ctx, recordMarker) }

// ENTITY.CREATE id type makes an entity, replacing one at id.
func cmdENTITYCREATE(ctx *Context) error {
	if ctx.ArgCount() != 2 {
		return ctx.WriteError(ErrWrongArgCount)
	}
	if err := createMarked(ctx, ctx.ArgString(0), entityMarker, entityMarker, ctx.ArgString(1)); err != nil {
		return ctx.WriteError(err)
	}
	return ctx.WriteOK()
}
func cmdENTITYDELETE(ctx *Context) error { return deleteMarked(ctx, entityMarker) }
func cmdENTITYGET(ctx *Context) error    { return bagReply(ctx, entityMarker, "entity", "type") }
func cmdENTITYSET(ctx *Context) error {
	if ctx.ArgCount() != 3 {
		return ctx.WriteError(ErrWrongArgCount)
	}
	return bagSet(ctx, ctx.ArgString(0), entityMarker, "entity", ctx.ArgString(1), ctx.ArgString(2))
}
func cmdENTITYLIST(ctx *Context) error { return listMarked(ctx, entityMarker) }

// PROFILE.CREATE user replies the new profile's id.
func cmdPROFILECREATE(ctx *Context) error {
	if ctx.ArgCount() != 1 {
		return ctx.WriteError(ErrWrongArgCount)
	}
	id := newObjectID(ctx)
	if err := replaceRecord(ctx, id, profileMarker, ctx.ArgString(0)); err != nil {
		return ctx.WriteError(err)
	}
	return ctx.WriteBulkString(id)
}
func cmdPROFILEDELETE(ctx *Context) error { return deleteMarked(ctx, profileMarker) }
func cmdPROFILEGET(ctx *Context) error    { return bagReply(ctx, profileMarker, "profile", "user") }
func cmdPROFILESET(ctx *Context) error {
	if ctx.ArgCount() != 3 {
		return ctx.WriteError(ErrWrongArgCount)
	}
	return bagSet(ctx, ctx.ArgString(0), profileMarker, "profile", ctx.ArgString(1), ctx.ArgString(2))
}
func cmdPROFILELIST(ctx *Context) error { return listMarked(ctx, profileMarker) }

// --- VALIDATE ---

// maxClientRegexLen bounds the patterns clients ask the server to compile:
// a compiled program takes hundreds of bytes per pattern byte, so a
// megabyte pattern cost hundreds of megabytes.
const maxClientRegexLen = 4096

// compileClientRegex compiles a pattern sent by a client.
func compileClientRegex(pattern string) (*regexp.Regexp, error) {
	if len(pattern) > maxClientRegexLen {
		return nil, fmt.Errorf("pattern longer than %d bytes", maxClientRegexLen)
	}
	return regexp.Compile(pattern)
}

// validatorRegexps caches compiled validator patterns by their text.
var (
	validatorRegexpsMu sync.Mutex
	validatorRegexps   = map[string]*regexp.Regexp{}
)

func validatorRegexp(pattern string) (*regexp.Regexp, error) {
	validatorRegexpsMu.Lock()
	re, ok := validatorRegexps[pattern]
	validatorRegexpsMu.Unlock()
	if ok {
		return re, nil
	}
	re, err := compileClientRegex(pattern)
	if err != nil {
		return nil, err
	}
	validatorRegexpsMu.Lock()
	if len(validatorRegexps) >= 256 {
		validatorRegexps = map[string]*regexp.Regexp{}
	}
	validatorRegexps[pattern] = re
	validatorRegexpsMu.Unlock()
	return re, nil
}

func cmdVALIDATECREATE(ctx *Context) error {
	if ctx.ArgCount() != 2 {
		return ctx.WriteError(ErrWrongArgCount)
	}
	if _, err := validatorRegexp(ctx.ArgString(1)); err != nil {
		return ctx.WriteError(fmt.Errorf("ERR invalid validation pattern: %v", err))
	}
	if err := createMarked(ctx, ctx.ArgString(0), validateMarker, validateMarker, ctx.ArgString(1)); err != nil {
		return ctx.WriteError(err)
	}
	return ctx.WriteOK()
}
func cmdVALIDATEDELETE(ctx *Context) error { return deleteMarked(ctx, validateMarker) }
func cmdVALIDATECHECK(ctx *Context) error {
	if ctx.ArgCount() != 2 {
		return ctx.WriteError(ErrWrongArgCount)
	}
	r, err := loadMarked(ctx, ctx.ArgString(0), validateMarker, "validator")
	if err != nil {
		return ctx.WriteError(err)
	}
	re, err := validatorRegexp(r[validateMarker])
	if err != nil {
		return ctx.WriteError(store.ErrWrongType)
	}
	return ctx.WriteInteger(boolInt(re.MatchString(ctx.ArgString(1))))
}
func cmdVALIDATELIST(ctx *Context) error { return listMarked(ctx, validateMarker) }

// --- VIEW ---

func cmdVIEWCREATE(ctx *Context) error {
	if ctx.ArgCount() != 2 {
		return ctx.WriteError(ErrWrongArgCount)
	}
	if err := createMarked(ctx, ctx.ArgString(0), viewMarker, viewMarker, ctx.ArgString(1)); err != nil {
		return ctx.WriteError(err)
	}
	return ctx.WriteOK()
}
func cmdVIEWDELETE(ctx *Context) error { return deleteMarked(ctx, viewMarker) }
func cmdVIEWGET(ctx *Context) error {
	if ctx.ArgCount() != 1 {
		return ctx.WriteError(ErrWrongArgCount)
	}
	r, err := loadMarked(ctx, ctx.ArgString(0), viewMarker, "view")
	if err != nil {
		return ctx.WriteError(err)
	}
	return ctx.WriteBulkString(r[viewMarker])
}
func cmdVIEWLIST(ctx *Context) error { return listMarked(ctx, viewMarker) }

// --- numeric records: STAGE, COUNTERX2, LEVEL ---

// markedInts reads integer fields of the marked hash at key.
func markedInts(ctx *Context, key, marker, what string, fields ...string) ([]int64, error) {
	r, err := loadMarked(ctx, key, marker, what)
	if err != nil {
		return nil, err
	}
	out := make([]int64, len(fields))
	for i, f := range fields {
		n, err := strconv.ParseInt(r[f], 10, 64)
		if err != nil {
			return nil, store.ErrWrongType
		}
		out[i] = n
	}
	return out, nil
}

func cmdSTAGECREATE(ctx *Context) error {
	if ctx.ArgCount() != 2 {
		return ctx.WriteError(ErrWrongArgCount)
	}
	total, err := strconv.ParseInt(ctx.ArgString(1), 10, 64)
	if err != nil || total < 0 {
		return ctx.WriteError(errors.New("ERR total must be a non-negative integer"))
	}
	if err := createMarked(ctx, ctx.ArgString(0), stageMarker, stageMarker, "0", "total", i64(total)); err != nil {
		return ctx.WriteError(err)
	}
	return ctx.WriteOK()
}
func cmdSTAGEDELETE(ctx *Context) error { return deleteMarked(ctx, stageMarker) }

// stageStep moves the stage by delta within [0, total] and replies where it
// is.
func stageStep(ctx *Context, delta int64) error {
	if ctx.ArgCount() != 1 {
		return ctx.WriteError(ErrWrongArgCount)
	}
	name := ctx.ArgString(0)
	v, err := markedInts(ctx, name, stageMarker, "stage", stageMarker, "total")
	if err != nil {
		return ctx.WriteError(err)
	}
	next := min(max(v[0]+delta, 0), v[1])
	if next == v[0] {
		ctx.PropagateAs()
		return ctx.WriteInteger(next)
	}
	if err := updateRecord(ctx, name, stageMarker, i64(next)); err != nil {
		return ctx.WriteError(err)
	}
	return ctx.WriteInteger(next)
}
func cmdSTAGENEXT(ctx *Context) error { return stageStep(ctx, 1) }
func cmdSTAGEPREV(ctx *Context) error { return stageStep(ctx, -1) }
func cmdSTAGELIST(ctx *Context) error {
	if ctx.ArgCount() != 0 {
		return ctx.WriteError(ErrWrongArgCount)
	}
	var out []*resp.Value
	for _, name := range markedKeys(ctx, stageMarker) {
		v, err := markedInts(ctx, name, stageMarker, "stage", stageMarker, "total")
		if err != nil {
			continue
		}
		out = append(out, resp.ArrayValue([]*resp.Value{
			resp.BulkString("name"), resp.BulkString(name),
			resp.BulkString("current"), resp.IntegerValue(v[0]),
			resp.BulkString("total"), resp.IntegerValue(v[1]),
		}))
	}
	return ctx.WriteArray(out)
}

func cmdCOUNTERX2CREATE(ctx *Context) error {
	if ctx.ArgCount() < 1 || ctx.ArgCount() > 2 {
		return ctx.WriteError(ErrWrongArgCount)
	}
	initial := int64(0)
	if ctx.ArgCount() == 2 {
		var err error
		if initial, err = parseInt64Arg(ctx.ArgString(1)); err != nil {
			return ctx.WriteError(err)
		}
	}
	if err := createMarked(ctx, ctx.ArgString(0), counterXMarker, counterXMarker, i64(initial)); err != nil {
		return ctx.WriteError(err)
	}
	return ctx.WriteOK()
}

// counterXAdd adds sign times the optional amount (1 by default) to the
// counter, creating it at 0.
func counterXAdd(ctx *Context, sign int64) error {
	if ctx.ArgCount() < 1 || ctx.ArgCount() > 2 {
		return ctx.WriteError(ErrWrongArgCount)
	}
	by := int64(1)
	if ctx.ArgCount() == 2 {
		var err error
		if by, err = parseInt64Arg(ctx.ArgString(1)); err != nil {
			return ctx.WriteError(err)
		}
	}
	name := ctx.ArgString(0)
	r, err := readMarked(ctx, name, counterXMarker)
	if err != nil {
		return ctx.WriteError(err)
	}
	cur := int64(0)
	if r != nil {
		if cur, err = strconv.ParseInt(r[counterXMarker], 10, 64); err != nil {
			return ctx.WriteError(store.ErrWrongType)
		}
	}
	if by == math64Min && sign < 0 {
		return ctx.WriteError(errors.New("ERR increment or decrement would overflow"))
	}
	delta := sign * by
	if (delta > 0 && cur > (1<<63-1)-delta) || (delta < 0 && cur < -(1<<63-1)-1-delta) {
		return ctx.WriteError(errors.New("ERR increment or decrement would overflow"))
	}
	if err := updateRecord(ctx, name, counterXMarker, i64(cur+delta)); err != nil {
		return ctx.WriteError(err)
	}
	return ctx.WriteInteger(cur + delta)
}

const math64Min = -1 << 63

func cmdCOUNTERX2INCR(ctx *Context) error { return counterXAdd(ctx, 1) }
func cmdCOUNTERX2DECR(ctx *Context) error { return counterXAdd(ctx, -1) }
func cmdCOUNTERX2GET(ctx *Context) error {
	if ctx.ArgCount() != 1 {
		return ctx.WriteError(ErrWrongArgCount)
	}
	r, err := readMarked(ctx, ctx.ArgString(0), counterXMarker)
	if err != nil {
		return ctx.WriteError(err)
	}
	if r == nil {
		return ctx.WriteInteger(0)
	}
	n, err := strconv.ParseInt(r[counterXMarker], 10, 64)
	if err != nil {
		return ctx.WriteError(store.ErrWrongType)
	}
	return ctx.WriteInteger(n)
}
func cmdCOUNTERX2LIST(ctx *Context) error {
	if ctx.ArgCount() != 0 {
		return ctx.WriteError(ErrWrongArgCount)
	}
	var out []*resp.Value
	for _, name := range markedKeys(ctx, counterXMarker) {
		v, err := markedInts(ctx, name, counterXMarker, "counter", counterXMarker)
		if err != nil {
			continue
		}
		out = append(out, resp.ArrayValue([]*resp.Value{
			resp.BulkString("name"), resp.BulkString(name),
			resp.BulkString("value"), resp.IntegerValue(v[0]),
		}))
	}
	return ctx.WriteArray(out)
}

func cmdLEVELCREATE(ctx *Context) error {
	if ctx.ArgCount() != 2 {
		return ctx.WriteError(ErrWrongArgCount)
	}
	maxLevel, err := strconv.ParseInt(ctx.ArgString(1), 10, 64)
	if err != nil || maxLevel < 0 {
		return ctx.WriteError(errors.New("ERR max level must be a non-negative integer"))
	}
	if err := createMarked(ctx, ctx.ArgString(0), levelMarker, levelMarker, "0", "max", i64(maxLevel)); err != nil {
		return ctx.WriteError(err)
	}
	return ctx.WriteOK()
}
func cmdLEVELDELETE(ctx *Context) error { return deleteMarked(ctx, levelMarker) }
func cmdLEVELSET(ctx *Context) error {
	if ctx.ArgCount() != 2 {
		return ctx.WriteError(ErrWrongArgCount)
	}
	name := ctx.ArgString(0)
	level, err := parseInt64Arg(ctx.ArgString(1))
	if err != nil {
		return ctx.WriteError(err)
	}
	v, err := markedInts(ctx, name, levelMarker, "level", levelMarker, "max")
	if err != nil {
		return ctx.WriteError(err)
	}
	if level < 0 || level > v[1] {
		return ctx.WriteError(fmt.Errorf("ERR level must be between 0 and %d", v[1]))
	}
	if err := updateRecord(ctx, name, levelMarker, i64(level)); err != nil {
		return ctx.WriteError(err)
	}
	return ctx.WriteOK()
}
func cmdLEVELGET(ctx *Context) error {
	if ctx.ArgCount() != 1 {
		return ctx.WriteError(ErrWrongArgCount)
	}
	v, err := markedInts(ctx, ctx.ArgString(0), levelMarker, "level", levelMarker, "max")
	if err != nil {
		return ctx.WriteError(err)
	}
	return ctx.WriteArray([]*resp.Value{
		resp.BulkString("current"), resp.IntegerValue(v[0]),
		resp.BulkString("max"), resp.IntegerValue(v[1]),
	})
}
func cmdLEVELLIST(ctx *Context) error { return listMarked(ctx, levelMarker) }

// --- PERMIT ---

// A user's permissions are a hash key named after the user, marked by
// __permit, with one field resource:action per permission.

func permitField(ctx *Context) string { return ctx.ArgString(1) + ":" + ctx.ArgString(2) }

func cmdPERMITGRANT(ctx *Context) error {
	if ctx.ArgCount() != 3 {
		return ctx.WriteError(ErrWrongArgCount)
	}
	user := ctx.ArgString(0)
	if _, err := readMarked(ctx, user, permitMarker); err != nil {
		return ctx.WriteError(err)
	}
	if err := setMarkedFields(ctx, user, permitMarker, permitField(ctx), ""); err != nil {
		return ctx.WriteError(err)
	}
	return ctx.WriteOK()
}
func cmdPERMITREVOKE(ctx *Context) error {
	if ctx.ArgCount() != 3 {
		return ctx.WriteError(ErrWrongArgCount)
	}
	user := ctx.ArgString(0)
	if _, err := readMarked(ctx, user, permitMarker); err != nil {
		return ctx.WriteError(err)
	}
	return ctx.WriteInteger(int64(deleteMarkedFields(ctx, user, permitMarker, permitField(ctx))))
}
func cmdPERMITCHECK(ctx *Context) error {
	if ctx.ArgCount() != 3 {
		return ctx.WriteError(ErrWrongArgCount)
	}
	h, err := readMarkedHash(ctx, ctx.ArgString(0), permitMarker)
	if err != nil {
		return ctx.WriteError(err)
	}
	granted := false
	if h != nil {
		h.RLock()
		_, granted = h.Get(permitField(ctx))
		h.RUnlock()
	}
	return ctx.WriteInteger(boolInt(granted))
}

// PERMIT.LIST user replies user:resource:action for each permission, sorted.
func cmdPERMITLIST(ctx *Context) error {
	if ctx.ArgCount() != 1 {
		return ctx.WriteError(ErrWrongArgCount)
	}
	user := ctx.ArgString(0)
	r, err := readMarked(ctx, user, permitMarker)
	if err != nil {
		return ctx.WriteError(err)
	}
	delete(r, permitMarker)
	keys := sortedKeys(r)
	for i, k := range keys {
		keys[i] = user + ":" + k
	}
	return ctx.WriteArray(bulkStrings(keys))
}

// --- GRANT ---

// A grant is a hash key named by its generated id, marked by __grant (the
// user), with resource and the actions space-separated. GRANT.CHECK scans
// the database's grants; PERMIT keeps a user's permissions in one key.

func cmdGRANTCREATE(ctx *Context) error {
	if ctx.ArgCount() < 3 {
		return ctx.WriteError(ErrWrongArgCount)
	}
	actions := make([]string, 0, ctx.ArgCount()-2)
	for i := 2; i < ctx.ArgCount(); i++ {
		a := ctx.ArgString(i)
		if a == "" || strings.ContainsAny(a, " \t\r\n") {
			return ctx.WriteError(errors.New("ERR actions must be non-empty and contain no whitespace"))
		}
		actions = append(actions, a)
	}
	id := newObjectID(ctx)
	if err := replaceRecord(ctx, id, grantMarker, ctx.ArgString(0), "resource", ctx.ArgString(1), "actions", strings.Join(actions, " ")); err != nil {
		return ctx.WriteError(err)
	}
	return ctx.WriteBulkString(id)
}
func cmdGRANTDELETE(ctx *Context) error { return deleteMarked(ctx, grantMarker) }
func cmdGRANTCHECK(ctx *Context) error {
	if ctx.ArgCount() != 3 {
		return ctx.WriteError(ErrWrongArgCount)
	}
	user, resource, action := ctx.ArgString(0), ctx.ArgString(1), ctx.ArgString(2)
	for _, id := range markedKeys(ctx, grantMarker) {
		r, err := readMarked(ctx, id, grantMarker)
		if err != nil || r == nil || r[grantMarker] != user || r["resource"] != resource {
			continue
		}
		for _, a := range strings.Fields(r["actions"]) {
			if a == action || a == "*" {
				return ctx.WriteInteger(1)
			}
		}
	}
	return ctx.WriteInteger(0)
}
func cmdGRANTLIST(ctx *Context) error {
	if ctx.ArgCount() != 0 {
		return ctx.WriteError(ErrWrongArgCount)
	}
	var out []*resp.Value
	for _, id := range markedKeys(ctx, grantMarker) {
		r, err := readMarked(ctx, id, grantMarker)
		if err != nil || r == nil {
			continue
		}
		out = append(out, resp.ArrayValue([]*resp.Value{
			resp.BulkString("id"), resp.BulkString(id),
			resp.BulkString("user"), resp.BulkString(r[grantMarker]),
			resp.BulkString("resource"), resp.BulkString(r["resource"]),
		}))
	}
	return ctx.WriteArray(out)
}

// --- RELATION ---

func cmdRELATIONCREATE(ctx *Context) error {
	if ctx.ArgCount() != 3 {
		return ctx.WriteError(ErrWrongArgCount)
	}
	id := newObjectID(ctx)
	if err := replaceRecord(ctx, id, relationMarker, ctx.ArgString(2), "from", ctx.ArgString(0), "to", ctx.ArgString(1)); err != nil {
		return ctx.WriteError(err)
	}
	return ctx.WriteBulkString(id)
}
func cmdRELATIONDELETE(ctx *Context) error { return deleteMarked(ctx, relationMarker) }
func cmdRELATIONGET(ctx *Context) error {
	if ctx.ArgCount() != 1 {
		return ctx.WriteError(ErrWrongArgCount)
	}
	id := ctx.ArgString(0)
	r, err := loadMarked(ctx, id, relationMarker, "relation")
	if err != nil {
		return ctx.WriteError(err)
	}
	return ctx.WriteArray([]*resp.Value{
		resp.BulkString("id"), resp.BulkString(id),
		resp.BulkString("from"), resp.BulkString(r["from"]),
		resp.BulkString("to"), resp.BulkString(r["to"]),
		resp.BulkString("type"), resp.BulkString(r[relationMarker]),
	})
}
func cmdRELATIONLIST(ctx *Context) error { return listMarked(ctx, relationMarker) }

// --- TIMER ---

func cmdTIMERCREATE(ctx *Context) error {
	if ctx.ArgCount() != 2 {
		return ctx.WriteError(ErrWrongArgCount)
	}
	duration, err := strconv.ParseInt(ctx.ArgString(1), 10, 64)
	if err != nil || duration < 0 {
		return ctx.WriteError(errors.New("ERR duration must be a non-negative integer (milliseconds)"))
	}
	id := newObjectID(ctx)
	if err := replaceRecord(ctx, id, timerMarker, ctx.ArgString(0), "duration", i64(duration), "start", i64(ctx.nowMS())); err != nil {
		return ctx.WriteError(err)
	}
	return ctx.WriteBulkString(id)
}
func cmdTIMERDELETE(ctx *Context) error { return deleteMarked(ctx, timerMarker) }
func cmdTIMERSTATUS(ctx *Context) error {
	if ctx.ArgCount() != 1 {
		return ctx.WriteError(ErrWrongArgCount)
	}
	id := ctx.ArgString(0)
	v, err := markedInts(ctx, id, timerMarker, "timer", "duration", "start")
	if err != nil {
		return ctx.WriteError(err)
	}
	remaining := max(v[0]-(ctx.nowMS()-v[1]), 0)
	return ctx.WriteArray([]*resp.Value{
		resp.BulkString("id"), resp.BulkString(id),
		resp.BulkString("running"), resp.BulkString(strconv.FormatBool(remaining > 0)),
		resp.BulkString("remaining_ms"), resp.IntegerValue(remaining),
	})
}
func cmdTIMERLIST(ctx *Context) error { return listMarked(ctx, timerMarker) }

// --- TOKEN ---

// A token is a hash key named by its generated id, marked by __token (the
// user), that expires with the token.

func setToken(ctx *Context, id, user string, ttl int64) error {
	at := ctx.nowMS() + ttl
	h, _ := store.NewLoadedHash([]string{tokenMarker, user})
	if err := ctx.Store.Set(id, h, store.SetOptions{ExpireAtMS: at}); err != nil {
		return err
	}
	ctx.notify(store.NotifyHash, "hset", id)
	ctx.PropagateAs(
		ExecutedCommand{Name: "DEL", Args: [][]byte{[]byte(id)}, DB: ctx.DB},
		ExecutedCommand{Name: "HSET", Args: pairArgs(id, []string{tokenMarker, user}), DB: ctx.DB},
		ExecutedCommand{Name: "PEXPIREAT", Args: [][]byte{[]byte(id), strconv.AppendInt(nil, at, 10)}, DB: ctx.DB},
	)
	return nil
}

func parseTokenTTL(arg string) (int64, error) {
	ttl, err := strconv.ParseInt(arg, 10, 64)
	if err != nil || ttl < 1 || ttl > 1<<53 {
		return 0, errors.New("ERR ttl must be a positive integer (milliseconds)")
	}
	return ttl, nil
}

func cmdTOKENCREATE(ctx *Context) error {
	if ctx.ArgCount() != 2 {
		return ctx.WriteError(ErrWrongArgCount)
	}
	ttl, err := parseTokenTTL(ctx.ArgString(1))
	if err != nil {
		return ctx.WriteError(err)
	}
	id := newObjectID(ctx)
	if err := setToken(ctx, id, ctx.ArgString(0), ttl); err != nil {
		return ctx.WriteError(err)
	}
	return ctx.WriteBulkString(id)
}
func cmdTOKENDELETE(ctx *Context) error { return deleteMarked(ctx, tokenMarker) }

// TOKEN.VALIDATE id is 1 while the token lives.
func cmdTOKENVALIDATE(ctx *Context) error {
	if ctx.ArgCount() != 1 {
		return ctx.WriteError(ErrWrongArgCount)
	}
	r, err := readMarked(ctx, ctx.ArgString(0), tokenMarker)
	if err != nil {
		return ctx.WriteError(err)
	}
	return ctx.WriteInteger(boolInt(r != nil))
}

// TOKEN.REFRESH id ttl_ms makes a live token expire ttl_ms from now.
func cmdTOKENREFRESH(ctx *Context) error {
	if ctx.ArgCount() != 2 {
		return ctx.WriteError(ErrWrongArgCount)
	}
	ttl, err := parseTokenTTL(ctx.ArgString(1))
	if err != nil {
		return ctx.WriteError(err)
	}
	id := ctx.ArgString(0)
	r, err := loadMarked(ctx, id, tokenMarker, "token")
	if err == nil {
		err = setToken(ctx, id, r[tokenMarker], ttl)
	}
	if err != nil {
		return ctx.WriteError(err)
	}
	return ctx.WriteOK()
}
func cmdTOKENLIST(ctx *Context) error { return listMarked(ctx, tokenMarker) }

// --- POOLX ---

// A pool is a hash key marked by __poolx (its size) whose resources are
// resource-0 to resource-<size-1>; one in use is the field u:<resource>.
// ACQUIRE hands out the lowest-numbered free resource.

func loadPoolX(ctx *Context, name string) (int, map[string]string, error) {
	r, err := loadMarked(ctx, name, poolXMarker, "pool")
	if err != nil {
		return 0, nil, err
	}
	size, err := strconv.Atoi(r[poolXMarker])
	if err != nil || size < 1 || size > maxExtensionSlots {
		return 0, nil, store.ErrWrongType
	}
	return size, r, nil
}

func cmdPOOLXCREATE(ctx *Context) error {
	if ctx.ArgCount() != 2 {
		return ctx.WriteError(ErrWrongArgCount)
	}
	size, err := strconv.Atoi(ctx.ArgString(1))
	if err != nil {
		return ctx.WriteError(ErrNotInteger)
	}
	if err := checkSlots("size", size); err != nil {
		return ctx.WriteError(err)
	}
	if err := createMarked(ctx, ctx.ArgString(0), poolXMarker, poolXMarker, strconv.Itoa(size)); err != nil {
		return ctx.WriteError(err)
	}
	return ctx.WriteOK()
}
func cmdPOOLXDELETE(ctx *Context) error { return deleteMarked(ctx, poolXMarker) }
func cmdPOOLXACQUIRE(ctx *Context) error {
	if ctx.ArgCount() != 1 {
		return ctx.WriteError(ErrWrongArgCount)
	}
	name := ctx.ArgString(0)
	size, r, err := loadPoolX(ctx, name)
	if err != nil {
		return ctx.WriteError(err)
	}
	for i := 0; i < size; i++ {
		res := "resource-" + strconv.Itoa(i)
		if _, used := r["u:"+res]; !used {
			if err := updateRecord(ctx, name, "u:"+res, ""); err != nil {
				return ctx.WriteError(err)
			}
			return ctx.WriteBulkString(res)
		}
	}
	ctx.PropagateAs()
	return ctx.WriteNull()
}
func cmdPOOLXRELEASE(ctx *Context) error {
	if ctx.ArgCount() != 2 {
		return ctx.WriteError(ErrWrongArgCount)
	}
	name := ctx.ArgString(0)
	if _, _, err := loadPoolX(ctx, name); err != nil {
		return ctx.WriteError(err)
	}
	return ctx.WriteInteger(int64(deleteMarkedFields(ctx, name, poolXMarker, "u:"+ctx.ArgString(1))))
}
func cmdPOOLXSTATUS(ctx *Context) error {
	if ctx.ArgCount() != 1 {
		return ctx.WriteError(ErrWrongArgCount)
	}
	name := ctx.ArgString(0)
	size, r, err := loadPoolX(ctx, name)
	if err != nil {
		return ctx.WriteError(err)
	}
	inUse := len(r) - 1
	return ctx.WriteArray([]*resp.Value{
		resp.BulkString("name"), resp.BulkString(name),
		resp.BulkString("size"), resp.IntegerValue(int64(size)),
		resp.BulkString("available"), resp.IntegerValue(int64(size - inUse)),
		resp.BulkString("in_use"), resp.IntegerValue(int64(inUse)),
	})
}

// --- BUFFERX ---

// A buffer is a string key: BUFFERX.CREATE sets it empty, WRITE appends.

func readBuffer(ctx *Context, name string) (*store.Entry, error) {
	entry, ok := ctx.Store.Get(name)
	if !ok {
		return nil, nil
	}
	if entry.Value.Type() != store.DataTypeString {
		return nil, store.ErrWrongType
	}
	return entry, nil
}

// BUFFERX.CREATE name size makes an empty buffer, replacing one at name;
// size is accepted for compatibility.
func cmdBUFFERXCREATE(ctx *Context) error {
	if ctx.ArgCount() != 2 {
		return ctx.WriteError(ErrWrongArgCount)
	}
	size, err := strconv.ParseInt(ctx.ArgString(1), 10, 64)
	if err != nil || size < 0 {
		return ctx.WriteError(errors.New("ERR size must be a non-negative integer"))
	}
	name := ctx.ArgString(0)
	if _, err := readBuffer(ctx, name); err != nil {
		return ctx.WriteError(err)
	}
	if err := ctx.Store.Set(name, &store.StringValue{Data: []byte{}}, store.SetOptions{}); err != nil {
		return ctx.WriteError(err)
	}
	ctx.notify(store.NotifyString, "set", name)
	ctx.PropagateAs(ExecutedCommand{Name: "SET", Args: [][]byte{[]byte(name), {}}, DB: ctx.DB})
	return ctx.WriteOK()
}
func cmdBUFFERXWRITE(ctx *Context) error {
	if ctx.ArgCount() != 2 {
		return ctx.WriteError(ErrWrongArgCount)
	}
	entry, err := readBuffer(ctx, ctx.ArgString(0))
	if err == nil && entry == nil {
		err = errors.New("ERR buffer not found")
	}
	if err != nil {
		return ctx.WriteError(err)
	}
	return runAs(ctx, "APPEND", func(c *Context) error {
		saved := c.Writer
		var sink strings.Builder
		c.Writer = resp.NewWriter(&sink)
		err := cmdAPPEND(c)
		c.Writer = saved
		if err != nil {
			return err
		}
		if reply := sink.String(); strings.HasPrefix(reply, "-") {
			c.PropagateAs()
			return c.WriteError(errors.New(strings.TrimRight(reply[1:], "\r\n")))
		}
		return c.WriteOK()
	}, ctx.ArgString(0), ctx.ArgString(1))
}
func cmdBUFFERXREAD(ctx *Context) error {
	if ctx.ArgCount() != 1 {
		return ctx.WriteError(ErrWrongArgCount)
	}
	if _, err := readBuffer(ctx, ctx.ArgString(0)); err != nil {
		return ctx.WriteError(err)
	}
	return cmdGET(ctx)
}
func cmdBUFFERXDELETE(ctx *Context) error {
	if ctx.ArgCount() != 1 {
		return ctx.WriteError(ErrWrongArgCount)
	}
	name := ctx.ArgString(0)
	entry, err := readBuffer(ctx, name)
	if err != nil {
		return ctx.WriteError(err)
	}
	if entry == nil {
		ctx.PropagateAs()
		return ctx.WriteInteger(0)
	}
	ctx.Store.Delete(name)
	ctx.notify(store.NotifyGeneric, "del", name)
	ctx.propagateDelete(name)
	return ctx.WriteInteger(1)
}

// --- STREAMX and AUDITX logs ---

func cmdSTREAMXCREATE(ctx *Context) error {
	if ctx.ArgCount() != 1 {
		return ctx.WriteError(ErrWrongArgCount)
	}
	if err := createMarked(ctx, ctx.ArgString(0), streamXMarker, streamXMarker, "0"); err != nil {
		return ctx.WriteError(err)
	}
	return ctx.WriteOK()
}
func cmdSTREAMXWRITE(ctx *Context) error {
	if ctx.ArgCount() != 2 {
		return ctx.WriteError(ErrWrongArgCount)
	}
	name := ctx.ArgString(0)
	n, _, ok, err := logHead(ctx, name, streamXMarker)
	if err == nil && !ok {
		err = errors.New("ERR stream not found")
	}
	if err == nil {
		err = appendLog(ctx, name, streamXMarker, n, ctx.ArgString(1))
	}
	if err != nil {
		return ctx.WriteError(err)
	}
	return ctx.WriteOK()
}
func cmdSTREAMXREAD(ctx *Context) error {
	if ctx.ArgCount() != 1 {
		return ctx.WriteError(ErrWrongArgCount)
	}
	entries, _, err := readLog(ctx, ctx.ArgString(0), streamXMarker)
	if err != nil {
		return ctx.WriteError(err)
	}
	return ctx.WriteArray(bulkStrings(entries))
}
func cmdSTREAMXDELETE(ctx *Context) error { return deleteMarked(ctx, streamXMarker) }

type auditEntry struct {
	Timestamp int64  `json:"timestamp"`
	Action    string `json:"action"`
	User      string `json:"user"`
	Resource  string `json:"resource"`
}

// AUDITX.LOG log action user [resource] appends an entry, creating the log.
func cmdAUDITXLOG(ctx *Context) error {
	if ctx.ArgCount() < 3 || ctx.ArgCount() > 4 {
		return ctx.WriteError(ErrWrongArgCount)
	}
	name := ctx.ArgString(0)
	e := auditEntry{Timestamp: ctx.nowMS(), Action: ctx.ArgString(1), User: ctx.ArgString(2)}
	if ctx.ArgCount() == 4 {
		e.Resource = ctx.ArgString(3)
	}
	raw, err := json.Marshal(e)
	if err != nil {
		return ctx.WriteError(err)
	}
	n, _, _, err := logHead(ctx, name, auditXMarker)
	if err == nil {
		err = appendLog(ctx, name, auditXMarker, n, string(raw))
	}
	if err != nil {
		return ctx.WriteError(err)
	}
	return ctx.WriteOK()
}

func readAudit(ctx *Context, name string) ([]auditEntry, error) {
	raw, _, err := readLog(ctx, name, auditXMarker)
	if err != nil {
		return nil, err
	}
	out := make([]auditEntry, len(raw))
	for i, r := range raw {
		if json.Unmarshal([]byte(r), &out[i]) != nil {
			return nil, store.ErrWrongType
		}
	}
	return out, nil
}

func auditReply(e auditEntry, resource bool) *resp.Value {
	v := []*resp.Value{
		resp.BulkString("timestamp"), resp.IntegerValue(e.Timestamp),
		resp.BulkString("action"), resp.BulkString(e.Action),
		resp.BulkString("user"), resp.BulkString(e.User),
	}
	if resource {
		v = append(v, resp.BulkString("resource"), resp.BulkString(e.Resource))
	}
	return resp.ArrayValue(v)
}

func cmdAUDITXGET(ctx *Context) error {
	if ctx.ArgCount() != 1 {
		return ctx.WriteError(ErrWrongArgCount)
	}
	entries, err := readAudit(ctx, ctx.ArgString(0))
	if err != nil {
		return ctx.WriteError(err)
	}
	out := make([]*resp.Value, len(entries))
	for i, e := range entries {
		out[i] = auditReply(e, true)
	}
	return ctx.WriteArray(out)
}

// AUDITX.SEARCH log query replies the entries whose action, user or
// resource equals query.
func cmdAUDITXSEARCH(ctx *Context) error {
	if ctx.ArgCount() != 2 {
		return ctx.WriteError(ErrWrongArgCount)
	}
	entries, err := readAudit(ctx, ctx.ArgString(0))
	if err != nil {
		return ctx.WriteError(err)
	}
	q := ctx.ArgString(1)
	var out []*resp.Value
	for _, e := range entries {
		if e.Action == q || e.User == q || e.Resource == q {
			out = append(out, auditReply(e, false))
		}
	}
	return ctx.WriteArray(out)
}
func cmdAUDITXLIST(ctx *Context) error { return listMarked(ctx, auditXMarker) }

// --- INDEX ---

// An index is a hash key marked by __index with each indexed key as the
// field k:<key> holding its ids as a JSON array, in the order added.

func cmdINDEXCREATE(ctx *Context) error {
	if ctx.ArgCount() != 1 {
		return ctx.WriteError(ErrWrongArgCount)
	}
	if err := createMarked(ctx, ctx.ArgString(0), indexMarker, indexMarker, "1"); err != nil {
		return ctx.WriteError(err)
	}
	return ctx.WriteOK()
}
func cmdINDEXDELETE(ctx *Context) error { return deleteMarked(ctx, indexMarker) }

func indexIDs(ctx *Context, name, key string, must bool) ([]string, bool, error) {
	h, err := readMarkedHash(ctx, name, indexMarker)
	if err != nil {
		return nil, false, err
	}
	if h == nil {
		if must {
			return nil, false, errors.New("ERR index not found")
		}
		return nil, false, nil
	}
	h.RLock()
	raw, ok := h.Get("k:" + key)
	h.RUnlock()
	var ids []string
	if ok && json.Unmarshal(raw, &ids) != nil {
		return nil, false, store.ErrWrongType
	}
	return ids, true, nil
}

func cmdINDEXADD(ctx *Context) error {
	if ctx.ArgCount() != 3 {
		return ctx.WriteError(ErrWrongArgCount)
	}
	name, key := ctx.ArgString(0), ctx.ArgString(1)
	ids, _, err := indexIDs(ctx, name, key, true)
	if err != nil {
		return ctx.WriteError(err)
	}
	raw, err := json.Marshal(append(ids, ctx.ArgString(2)))
	if err == nil {
		err = updateRecord(ctx, name, "k:"+key, string(raw))
	}
	if err != nil {
		return ctx.WriteError(err)
	}
	return ctx.WriteOK()
}
func cmdINDEXSEARCH(ctx *Context) error {
	if ctx.ArgCount() != 2 {
		return ctx.WriteError(ErrWrongArgCount)
	}
	ids, _, err := indexIDs(ctx, ctx.ArgString(0), ctx.ArgString(1), false)
	if err != nil {
		return ctx.WriteError(err)
	}
	return ctx.WriteArray(bulkStrings(ids))
}
func cmdINDEXLIST(ctx *Context) error { return listMarked(ctx, indexMarker) }
