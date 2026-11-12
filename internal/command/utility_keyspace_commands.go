package command

import (
	"errors"
	"fmt"
	"sort"
	"strconv"
	"strings"

	"github.com/cachestorm/cachestorm/internal/resp"
	"github.com/cachestorm/cachestorm/internal/store"
)

// STATEM, CHAINED, FLAG, COUNTER and ID keep each object in a key: a state
// machine, feature flag, counter or ID sequence is a hash marked by a field
// naming its kind, and a CHAINED root a plain hash of its paths. Being keys
// they are per database, persisted and replicated, and changes are recorded
// as HSET, HDEL and DEL.

const (
	statemMarker  = "__statem"
	flagMarker    = "__flag"
	counterMarker = "__counter"
	idMarker      = "__id"
)

// --- STATEM ---

// A state machine's marker holds its initial state, current its current
// one; each state is the field s:<name> (1 when final) and each transition
// the field t:<len(from)>:<from><event> holding the target state.

func transitionField(from, event string) string {
	return "t:" + strconv.Itoa(len(from)) + ":" + from + event
}

func parseTransitionField(f string) (from, event string, ok bool) {
	rest, ok := strings.CutPrefix(f, "t:")
	if !ok {
		return "", "", false
	}
	n, rest, ok := strings.Cut(rest, ":")
	if !ok {
		return "", "", false
	}
	l, err := strconv.Atoi(n)
	if err != nil || l < 0 || l > len(rest) {
		return "", "", false
	}
	return rest[:l], rest[l:], true
}

func loadStatem(ctx *Context, name string) (map[string]string, error) {
	return loadMarked(ctx, name, statemMarker, "state machine")
}

func statemInfo(name string, r map[string]string) []*resp.Value {
	return []*resp.Value{
		resp.BulkString("name"), resp.BulkString(name),
		resp.BulkString("initial"), resp.BulkString(r[statemMarker]),
		resp.BulkString("current"), resp.BulkString(r["current"]),
	}
}

// STATEM.CREATE name initial makes a state machine in its initial state,
// or replies the one at name: its name, initial and current state.
func cmdSTATEMCREATE(ctx *Context) error {
	if ctx.ArgCount() != 2 {
		return ctx.WriteError(ErrWrongArgCount)
	}
	name, initial := ctx.ArgString(0), ctx.ArgString(1)
	r, err := readMarked(ctx, name, statemMarker)
	if err != nil {
		return ctx.WriteError(err)
	}
	if r != nil {
		ctx.PropagateAs()
		return ctx.WriteArray(statemInfo(name, r))
	}
	if err := replaceRecord(ctx, name, statemMarker, initial, "current", initial); err != nil {
		return ctx.WriteError(err)
	}
	return ctx.WriteArray(statemInfo(name, map[string]string{statemMarker: initial, "current": initial}))
}

func cmdSTATEMDELETE(ctx *Context) error { return deleteMarked(ctx, statemMarker) }

// STATEM.ADDSTATE name state [TRUE|FALSE] adds the state, final with TRUE.
func cmdSTATEMADDSTATE(ctx *Context) error {
	if ctx.ArgCount() != 2 && ctx.ArgCount() != 3 {
		return ctx.WriteError(ErrWrongArgCount)
	}
	final := "0"
	if ctx.ArgCount() == 3 {
		switch strings.ToUpper(ctx.ArgString(2)) {
		case "TRUE":
			final = "1"
		case "FALSE":
		default:
			return ctx.WriteError(ErrSyntaxError)
		}
	}
	name := ctx.ArgString(0)
	if _, err := loadStatem(ctx, name); err != nil {
		return ctx.WriteError(err)
	}
	if err := updateRecord(ctx, name, "s:"+ctx.ArgString(1), final); err != nil {
		return ctx.WriteError(err)
	}
	return ctx.WriteOK()
}

// STATEM.ADDTRANS name from to event makes event move from to to,
// replacing the transition the event had from from.
func cmdSTATEMADDTRANS(ctx *Context) error {
	if ctx.ArgCount() != 4 {
		return ctx.WriteError(ErrWrongArgCount)
	}
	name := ctx.ArgString(0)
	if _, err := loadStatem(ctx, name); err != nil {
		return ctx.WriteError(err)
	}
	if err := updateRecord(ctx, name, transitionField(ctx.ArgString(1), ctx.ArgString(3)), ctx.ArgString(2)); err != nil {
		return ctx.WriteError(err)
	}
	return ctx.WriteOK()
}

// STATEM.TRIGGER name event moves along the event's transition: the new
// state.
func cmdSTATEMTRIGGER(ctx *Context) error {
	if ctx.ArgCount() != 2 {
		return ctx.WriteError(ErrWrongArgCount)
	}
	name := ctx.ArgString(0)
	r, err := loadStatem(ctx, name)
	if err != nil {
		return ctx.WriteError(err)
	}
	to, ok := r[transitionField(r["current"], ctx.ArgString(1))]
	if !ok {
		return ctx.WriteError(errors.New("ERR invalid transition"))
	}
	if err := updateRecord(ctx, name, "current", to); err != nil {
		return ctx.WriteError(err)
	}
	return ctx.WriteBulkString(to)
}

func statemRead(ctx *Context, n int, reply func(map[string]string) error) error {
	if ctx.ArgCount() != n {
		return ctx.WriteError(ErrWrongArgCount)
	}
	r, err := loadStatem(ctx, ctx.ArgString(0))
	if err != nil {
		return ctx.WriteError(err)
	}
	return reply(r)
}

func cmdSTATEMCURRENT(ctx *Context) error {
	return statemRead(ctx, 1, func(r map[string]string) error { return ctx.WriteBulkString(r["current"]) })
}

func cmdSTATEMCANTRIGGER(ctx *Context) error {
	return statemRead(ctx, 2, func(r map[string]string) error {
		_, ok := r[transitionField(r["current"], ctx.ArgString(1))]
		return ctx.WriteInteger(boolInt(ok))
	})
}

// STATEM.EVENTS name: the events with a transition from the current state,
// sorted.
func cmdSTATEMEVENTS(ctx *Context) error {
	return statemRead(ctx, 1, func(r map[string]string) error {
		var events []string
		for f := range r {
			if from, event, ok := parseTransitionField(f); ok && from == r["current"] {
				events = append(events, event)
			}
		}
		sort.Strings(events)
		return ctx.WriteArray(bulkStrings(events))
	})
}

func cmdSTATEMRESET(ctx *Context) error {
	if ctx.ArgCount() != 1 {
		return ctx.WriteError(ErrWrongArgCount)
	}
	name := ctx.ArgString(0)
	r, err := loadStatem(ctx, name)
	if err != nil {
		return ctx.WriteError(err)
	}
	if err := updateRecord(ctx, name, "current", r[statemMarker]); err != nil {
		return ctx.WriteError(err)
	}
	return ctx.WriteOK()
}

func statemFinal(r map[string]string) bool { return r["s:"+r["current"]] == "1" }

func cmdSTATEMISFINAL(ctx *Context) error {
	return statemRead(ctx, 1, func(r map[string]string) error { return ctx.WriteInteger(boolInt(statemFinal(r))) })
}

func cmdSTATEMINFO(ctx *Context) error {
	return statemRead(ctx, 1, func(r map[string]string) error {
		return ctx.WriteArray([]*resp.Value{
			resp.BulkString("name"), resp.BulkString(ctx.ArgString(0)),
			resp.BulkString("current"), resp.BulkString(r["current"]),
			resp.BulkString("initial"), resp.BulkString(r[statemMarker]),
			resp.BulkString("is_final"), resp.BulkString(strconv.FormatBool(statemFinal(r))),
		})
	})
}

func cmdSTATEMLIST(ctx *Context) error { return listMarked(ctx, statemMarker) }

// --- CHAINED ---

func cmdCHAINEDSET(ctx *Context) error { return cmdOBJECTSET(ctx) }
func cmdCHAINEDGET(ctx *Context) error { return cmdOBJECTGET(ctx) }
func cmdCHAINEDDEL(ctx *Context) error { return cmdOBJECTDELETE(ctx) }

// --- FLAG ---

// A flag's marker holds 1 when it is enabled, with description,
// created_at, updated_at and each variant as v:<key>.

func loadFlag(ctx *Context, name string) (map[string]string, error) {
	return readMarked(ctx, name, flagMarker)
}

func flagEnabled(r map[string]string) string { return strconv.FormatBool(r[flagMarker] == "1") }

// FLAG.CREATE name [description] makes a disabled flag, replacing one at
// name.
func cmdFLAGCREATE(ctx *Context) error {
	if ctx.ArgCount() != 1 && ctx.ArgCount() != 2 {
		return ctx.WriteError(ErrWrongArgCount)
	}
	desc := ""
	if ctx.ArgCount() == 2 {
		desc = ctx.ArgString(1)
	}
	now := i64(ctx.nowMS())
	if err := createMarked(ctx, ctx.ArgString(0), flagMarker, flagMarker, "0", "description", desc, "created_at", now, "updated_at", now); err != nil {
		return ctx.WriteError(err)
	}
	return ctx.WriteArray([]*resp.Value{
		resp.BulkString("name"), resp.BulkString(ctx.ArgString(0)),
		resp.BulkString("enabled"), resp.BulkString("false"),
	})
}

func cmdFLAGDELETE(ctx *Context) error { return deleteMarked(ctx, flagMarker) }

func cmdFLAGGET(ctx *Context) error {
	if ctx.ArgCount() != 1 {
		return ctx.WriteError(ErrWrongArgCount)
	}
	r, err := loadFlag(ctx, ctx.ArgString(0))
	if err != nil {
		return ctx.WriteError(err)
	}
	if r == nil {
		return ctx.WriteNull()
	}
	rec := &record{vals: r}
	created, updated := rec.int("created_at"), rec.int("updated_at")
	if rec.bad {
		return ctx.WriteError(store.ErrWrongType)
	}
	return ctx.WriteArray([]*resp.Value{
		resp.BulkString("name"), resp.BulkString(ctx.ArgString(0)),
		resp.BulkString("enabled"), resp.BulkString(flagEnabled(r)),
		resp.BulkString("description"), resp.BulkString(r["description"]),
		resp.BulkString("created_at"), resp.IntegerValue(created),
		resp.BulkString("updated_at"), resp.IntegerValue(updated),
	})
}

// flagSet sets the flag's state, on(current) giving the new one: 1, or 0
// when the flag is missing.
func flagSet(ctx *Context, on func(bool) bool) error {
	if ctx.ArgCount() != 1 {
		return ctx.WriteError(ErrWrongArgCount)
	}
	name := ctx.ArgString(0)
	r, err := loadFlag(ctx, name)
	if err != nil {
		return ctx.WriteError(err)
	}
	if r == nil {
		ctx.PropagateAs()
		return ctx.WriteInteger(0)
	}
	state := "0"
	if on(r[flagMarker] == "1") {
		state = "1"
	}
	if err := updateRecord(ctx, name, flagMarker, state, "updated_at", i64(ctx.nowMS())); err != nil {
		return ctx.WriteError(err)
	}
	return ctx.WriteInteger(1)
}

func cmdFLAGENABLE(ctx *Context) error  { return flagSet(ctx, func(bool) bool { return true }) }
func cmdFLAGDISABLE(ctx *Context) error { return flagSet(ctx, func(bool) bool { return false }) }
func cmdFLAGTOGGLE(ctx *Context) error  { return flagSet(ctx, func(on bool) bool { return !on }) }

func cmdFLAGISENABLED(ctx *Context) error {
	if ctx.ArgCount() != 1 {
		return ctx.WriteError(ErrWrongArgCount)
	}
	r, err := loadFlag(ctx, ctx.ArgString(0))
	if err != nil {
		return ctx.WriteError(err)
	}
	return ctx.WriteInteger(boolInt(r != nil && r[flagMarker] == "1"))
}

func cmdFLAGLIST(ctx *Context) error { return listMarked(ctx, flagMarker) }

// FLAG.LISTENABLED: the enabled flags of the selected database, sorted.
func cmdFLAGLISTENABLED(ctx *Context) error {
	if ctx.ArgCount() != 0 {
		return ctx.WriteError(ErrWrongArgCount)
	}
	var out []string
	for _, key := range markedKeys(ctx, flagMarker) {
		if r, err := loadFlag(ctx, key); err == nil && r != nil && r[flagMarker] == "1" {
			out = append(out, key)
		}
	}
	return ctx.WriteArray(bulkStrings(out))
}

func cmdFLAGADDVARIANT(ctx *Context) error {
	if ctx.ArgCount() != 3 {
		return ctx.WriteError(ErrWrongArgCount)
	}
	name := ctx.ArgString(0)
	r, err := loadFlag(ctx, name)
	if err == nil && r == nil {
		err = errors.New("ERR flag not found")
	}
	if err != nil {
		return ctx.WriteError(err)
	}
	if err := updateRecord(ctx, name, "v:"+ctx.ArgString(1), ctx.ArgString(2), "updated_at", i64(ctx.nowMS())); err != nil {
		return ctx.WriteError(err)
	}
	return ctx.WriteOK()
}

func cmdFLAGGETVARIANT(ctx *Context) error {
	if ctx.ArgCount() != 2 {
		return ctx.WriteError(ErrWrongArgCount)
	}
	r, err := loadFlag(ctx, ctx.ArgString(0))
	if err != nil {
		return ctx.WriteError(err)
	}
	v, ok := r["v:"+ctx.ArgString(1)]
	if !ok {
		return ctx.WriteNull()
	}
	return ctx.WriteBulkString(v)
}

// --- COUNTER ---

// A counter is a hash marked by __counter holding its value.

func readCounter(ctx *Context, name string) (int64, bool, error) {
	r, err := readMarked(ctx, name, counterMarker)
	if err != nil || r == nil {
		return 0, false, err
	}
	n, err := strconv.ParseInt(r[counterMarker], 10, 64)
	if err != nil {
		return 0, false, store.ErrWrongType
	}
	return n, true, nil
}

func setCounter(ctx *Context, name string, v int64) error {
	return updateRecord(ctx, name, counterMarker, i64(v))
}

func cmdCOUNTERGET(ctx *Context) error {
	if ctx.ArgCount() != 1 {
		return ctx.WriteError(ErrWrongArgCount)
	}
	n, _, err := readCounter(ctx, ctx.ArgString(0))
	if err != nil {
		return ctx.WriteError(err)
	}
	return ctx.WriteInteger(n)
}

func cmdCOUNTERSET(ctx *Context) error {
	if ctx.ArgCount() != 2 {
		return ctx.WriteError(ErrWrongArgCount)
	}
	v, err := parseInt64Arg(ctx.ArgString(1))
	if err != nil {
		return ctx.WriteError(err)
	}
	if _, _, err := readCounter(ctx, ctx.ArgString(0)); err != nil {
		return ctx.WriteError(err)
	}
	if err := setCounter(ctx, ctx.ArgString(0), v); err != nil {
		return ctx.WriteError(err)
	}
	return ctx.WriteOK()
}

func counterAdd(ctx *Context, n int, sign int64) error {
	if ctx.ArgCount() != n {
		return ctx.WriteError(ErrWrongArgCount)
	}
	delta := int64(1)
	if n == 2 {
		d, err := parseInt64Arg(ctx.ArgString(1))
		if err != nil {
			return ctx.WriteError(err)
		}
		delta = d
	}
	if sign < 0 {
		if delta == -1<<63 {
			return ctx.WriteError(errors.New("ERR increment or decrement would overflow"))
		}
		delta = -delta
	}
	name := ctx.ArgString(0)
	cur, _, err := readCounter(ctx, name)
	if err != nil {
		return ctx.WriteError(err)
	}
	if (delta > 0 && cur > (1<<63-1)-delta) || (delta < 0 && cur < -1<<63-delta) {
		return ctx.WriteError(errors.New("ERR increment or decrement would overflow"))
	}
	if err := setCounter(ctx, name, cur+delta); err != nil {
		return ctx.WriteError(err)
	}
	return ctx.WriteInteger(cur + delta)
}

func cmdCOUNTERINCR(ctx *Context) error   { return counterAdd(ctx, 1, 1) }
func cmdCOUNTERDECR(ctx *Context) error   { return counterAdd(ctx, 1, -1) }
func cmdCOUNTERINCRBY(ctx *Context) error { return counterAdd(ctx, 2, 1) }
func cmdCOUNTERDECRBY(ctx *Context) error { return counterAdd(ctx, 2, -1) }
func cmdCOUNTERDELETE(ctx *Context) error { return deleteMarked(ctx, counterMarker) }
func cmdCOUNTERLIST(ctx *Context) error   { return listMarked(ctx, counterMarker) }

// COUNTER.GETALL: name, value pairs of the selected database's counters,
// sorted by name.
func cmdCOUNTERGETALL(ctx *Context) error {
	if ctx.ArgCount() != 0 {
		return ctx.WriteError(ErrWrongArgCount)
	}
	out := make([]*resp.Value, 0)
	for _, key := range markedKeys(ctx, counterMarker) {
		if n, ok, err := readCounter(ctx, key); err == nil && ok {
			out = append(out, resp.BulkString(key), resp.IntegerValue(n))
		}
	}
	return ctx.WriteArray(out)
}

func cmdCOUNTERRESET(ctx *Context) error {
	if ctx.ArgCount() != 1 {
		return ctx.WriteError(ErrWrongArgCount)
	}
	name := ctx.ArgString(0)
	_, ok, err := readCounter(ctx, name)
	if err == nil && !ok {
		err = errors.New("ERR counter not found")
	}
	if err != nil {
		return ctx.WriteError(err)
	}
	if err := setCounter(ctx, name, 0); err != nil {
		return ctx.WriteError(err)
	}
	return ctx.WriteOK()
}

// COUNTER.RESETALL sets every counter of the selected database to 0,
// recorded as an HSET of each.
func cmdCOUNTERRESETALL(ctx *Context) error {
	if ctx.ArgCount() != 0 {
		return ctx.WriteError(ErrWrongArgCount)
	}
	var cmds []ExecutedCommand
	for _, key := range markedKeys(ctx, counterMarker) {
		args := [][]byte{[]byte(counterMarker), []byte("0")}
		if _, err := ctx.Store.ApplyHashPairs(key, false, args); err != nil {
			continue
		}
		ctx.notify(store.NotifyHash, "hset", key)
		cmds = append(cmds, ExecutedCommand{Name: "HSET", Args: append([][]byte{[]byte(key)}, args...), DB: ctx.DB})
	}
	ctx.PropagateAs(cmds...)
	return ctx.WriteOK()
}

// --- ID ---

// An ID sequence's marker holds its current number, with increment,
// prefix, suffix and padding.

type idSequence struct {
	current, increment int64
	prefix, suffix     string
	padding            int
}

func readIDSequence(ctx *Context, name string) (*idSequence, error) {
	r, err := loadMarked(ctx, name, idMarker, "sequence")
	if err != nil {
		return nil, err
	}
	rec := &record{vals: r}
	s := &idSequence{current: rec.int(idMarker), increment: rec.int("increment"), prefix: r["prefix"], suffix: r["suffix"], padding: int(rec.int("padding"))}
	return s, errWrongTypeOr(rec.bad, nil)
}

func (s *idSequence) format(n int64) string {
	digits := strconv.FormatInt(n, 10)
	sign := ""
	if n < 0 {
		sign, digits = "-", digits[1:]
	}
	if pad := s.padding - len(digits); pad > 0 {
		digits = strings.Repeat("0", pad) + digits
	}
	return s.prefix + sign + digits + s.suffix
}

// ID.CREATE name start increment [PREFIX p] [SUFFIX s] [PADDING n] makes a
// sequence whose first ID is start, replacing one at name.
func cmdIDCREATE(ctx *Context) error {
	if ctx.ArgCount() < 3 || (ctx.ArgCount()-3)%2 != 0 {
		return ctx.WriteError(ErrWrongArgCount)
	}
	start, err := parseInt64Arg(ctx.ArgString(1))
	if err != nil {
		return ctx.WriteError(err)
	}
	inc, err := parseInt64Arg(ctx.ArgString(2))
	if err != nil {
		return ctx.WriteError(err)
	}
	pairs := []string{idMarker, i64(start - inc), "increment", i64(inc), "prefix", "", "suffix", "", "padding", "0"}
	for i := 3; i < ctx.ArgCount(); i += 2 {
		v := ctx.ArgString(i + 1)
		switch strings.ToUpper(ctx.ArgString(i)) {
		case "PREFIX":
			pairs[5] = v
		case "SUFFIX":
			pairs[7] = v
		case "PADDING":
			n, err := strconv.Atoi(v)
			if err != nil || n < 0 || n > maxIDPadding {
				return ctx.WriteError(fmt.Errorf("ERR padding must be between 0 and %d", maxIDPadding))
			}
			pairs[9] = strconv.Itoa(n)
		default:
			return ctx.WriteError(ErrSyntaxError)
		}
	}
	if err := createMarked(ctx, ctx.ArgString(0), idMarker, pairs...); err != nil {
		return ctx.WriteError(err)
	}
	return ctx.WriteOK()
}

// idTake hands out count IDs.
func idTake(ctx *Context, count int64) ([]string, error) {
	name := ctx.ArgString(0)
	s, err := readIDSequence(ctx, name)
	if err != nil {
		return nil, err
	}
	ids := make([]string, count)
	for i := range ids {
		s.current += s.increment
		ids[i] = s.format(s.current)
	}
	return ids, updateRecord(ctx, name, idMarker, i64(s.current))
}

func cmdIDNEXT(ctx *Context) error {
	if ctx.ArgCount() != 1 {
		return ctx.WriteError(ErrWrongArgCount)
	}
	ids, err := idTake(ctx, 1)
	if err != nil {
		return ctx.WriteError(err)
	}
	return ctx.WriteBulkString(ids[0])
}

func cmdIDNEXTN(ctx *Context) error {
	if ctx.ArgCount() != 2 {
		return ctx.WriteError(ErrWrongArgCount)
	}
	count, err := strconv.ParseInt(ctx.ArgString(1), 10, 64)
	if err != nil || count < 1 || count > maxIDBatch {
		return ctx.WriteError(fmt.Errorf("ERR count must be between 1 and %d", maxIDBatch))
	}
	ids, err := idTake(ctx, count)
	if err != nil {
		return ctx.WriteError(err)
	}
	return ctx.WriteArray(bulkStrings(ids))
}

func cmdIDCURRENT(ctx *Context) error {
	if ctx.ArgCount() != 1 {
		return ctx.WriteError(ErrWrongArgCount)
	}
	r, err := readMarked(ctx, ctx.ArgString(0), idMarker)
	if err != nil {
		return ctx.WriteError(err)
	}
	if r == nil {
		return ctx.WriteNull()
	}
	s, err := readIDSequence(ctx, ctx.ArgString(0))
	if err != nil {
		return ctx.WriteError(err)
	}
	return ctx.WriteArray([]*resp.Value{resp.BulkString(s.format(s.current)), resp.IntegerValue(s.current)})
}

func cmdIDSET(ctx *Context) error {
	if ctx.ArgCount() != 2 {
		return ctx.WriteError(ErrWrongArgCount)
	}
	v, err := parseInt64Arg(ctx.ArgString(1))
	if err != nil {
		return ctx.WriteError(err)
	}
	if _, err := readIDSequence(ctx, ctx.ArgString(0)); err != nil {
		return ctx.WriteError(err)
	}
	if err := updateRecord(ctx, ctx.ArgString(0), idMarker, i64(v)); err != nil {
		return ctx.WriteError(err)
	}
	return ctx.WriteOK()
}

func cmdIDDELETE(ctx *Context) error { return deleteMarked(ctx, idMarker) }
