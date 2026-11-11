package command

import (
	"errors"
	"fmt"
	"math"
	"sort"
	"strconv"
	"strings"

	"github.com/cachestorm/cachestorm/internal/resp"
	"github.com/cachestorm/cachestorm/internal/store"
)

// THRESHOLD, SWITCH, BOOKMARK, DEADLINE, ROUTE, GATEWAY, GRID, GHOST, TAPE,
// SLICE and ROLLUPX keep each of their objects in a hash key marked by a
// field naming its kind (__threshold, __switch, ...). Being keys, they are
// per database, persisted and replicated, and their LIST commands cover the
// selected database; changes are recorded as HSET, HDEL and DEL. GHOST,
// TAPE and SLICE are logs: the marker holds the entry count and the entries
// are the fields 0, 1, 2 and on.

const (
	thresholdMarker = "__threshold"
	switchMarker    = "__switch"
	bookmarkMarker  = "__bookmark"
	deadlineMarker  = "__deadline"
	routeMarker     = "__route"
	gatewayMarker   = "__gateway"
	gridMarker      = "__grid"
	ghostMarker     = "__ghost"
	tapeMarker      = "__tape"
	sliceMarker     = "__slice"
	rollupXMarker   = "__rollupx"
)

// readMarked returns the fields of the marked hash at key, marker included,
// nil when the key is missing.
func readMarked(ctx *Context, key, marker string) (map[string]string, error) {
	h, err := readMarkedHash(ctx, key, marker)
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

// loadMarked is readMarked for an object that must exist.
func loadMarked(ctx *Context, key, marker, what string) (map[string]string, error) {
	r, err := readMarked(ctx, key, marker)
	if err == nil && r == nil {
		err = fmt.Errorf("ERR %s not found", what)
	}
	return r, err
}

// deleteMarked deletes the marked hash at the first argument: 1 when it was
// there.
func deleteMarked(ctx *Context, marker string) error {
	if ctx.ArgCount() != 1 {
		return ctx.WriteError(ErrWrongArgCount)
	}
	key := ctx.ArgString(0)
	r, err := readMarked(ctx, key, marker)
	if err != nil {
		return ctx.WriteError(err)
	}
	if r == nil {
		ctx.PropagateAs()
		return ctx.WriteInteger(0)
	}
	ctx.Store.Delete(key)
	ctx.notify(store.NotifyGeneric, "del", key)
	ctx.propagateDelete(key)
	return ctx.WriteInteger(1)
}

// createMarked makes key a new marked hash of pairs, refusing a key of
// another kind.
func createMarked(ctx *Context, key, marker string, pairs ...string) error {
	if _, err := readMarked(ctx, key, marker); err != nil {
		return err
	}
	return replaceRecord(ctx, key, pairs...)
}

func bulkStrings(items []string) []*resp.Value {
	out := make([]*resp.Value, len(items))
	for i, s := range items {
		out[i] = resp.BulkString(s)
	}
	return out
}

func parseInt64Arg(arg string) (int64, error) {
	n, err := strconv.ParseInt(arg, 10, 64)
	if err != nil {
		return 0, ErrNotInteger
	}
	return n, nil
}

// --- THRESHOLD ---

func cmdTHRESHOLDSET(ctx *Context) error {
	if ctx.ArgCount() != 2 {
		return ctx.WriteError(ErrWrongArgCount)
	}
	v, err := parseInt64Arg(ctx.ArgString(1))
	if err != nil {
		return ctx.WriteError(err)
	}
	if err := createMarked(ctx, ctx.ArgString(0), thresholdMarker, thresholdMarker, i64(v)); err != nil {
		return ctx.WriteError(err)
	}
	return ctx.WriteOK()
}

// THRESHOLD.CHECK name value is 1 when value reaches the threshold.
func cmdTHRESHOLDCHECK(ctx *Context) error {
	if ctx.ArgCount() != 2 {
		return ctx.WriteError(ErrWrongArgCount)
	}
	v, err := parseInt64Arg(ctx.ArgString(1))
	if err != nil {
		return ctx.WriteError(err)
	}
	r, err := readMarked(ctx, ctx.ArgString(0), thresholdMarker)
	if err != nil {
		return ctx.WriteError(err)
	}
	if r == nil {
		return ctx.WriteInteger(0)
	}
	t, err := strconv.ParseInt(r[thresholdMarker], 10, 64)
	if err != nil {
		return ctx.WriteError(store.ErrWrongType)
	}
	return ctx.WriteInteger(boolInt(v >= t))
}

func cmdTHRESHOLDLIST(ctx *Context) error {
	if ctx.ArgCount() != 0 {
		return ctx.WriteError(ErrWrongArgCount)
	}
	var out []*resp.Value
	for _, name := range markedKeys(ctx, thresholdMarker) {
		r, err := readMarked(ctx, name, thresholdMarker)
		if err != nil || r == nil {
			continue
		}
		t, err := strconv.ParseInt(r[thresholdMarker], 10, 64)
		if err != nil {
			continue
		}
		out = append(out, resp.ArrayValue([]*resp.Value{
			resp.BulkString("name"), resp.BulkString(name),
			resp.BulkString("value"), resp.IntegerValue(t),
		}))
	}
	return ctx.WriteArray(out)
}

func cmdTHRESHOLDDELETE(ctx *Context) error { return deleteMarked(ctx, thresholdMarker) }

// --- SWITCH ---

func switchState(ctx *Context, name string) (bool, error) {
	r, err := readMarked(ctx, name, switchMarker)
	if err != nil || r == nil {
		return false, err
	}
	return r[switchMarker] == "1", nil
}

func setSwitch(ctx *Context, on bool) error {
	state := "0"
	if on {
		state = "1"
	}
	return createMarked(ctx, ctx.ArgString(0), switchMarker, switchMarker, state)
}

func cmdSWITCHSTATE(ctx *Context) error {
	if ctx.ArgCount() != 1 {
		return ctx.WriteError(ErrWrongArgCount)
	}
	on, err := switchState(ctx, ctx.ArgString(0))
	if err != nil {
		return ctx.WriteError(err)
	}
	return ctx.WriteInteger(boolInt(on))
}

func cmdSWITCHTOGGLE(ctx *Context) error {
	if ctx.ArgCount() != 1 {
		return ctx.WriteError(ErrWrongArgCount)
	}
	on, err := switchState(ctx, ctx.ArgString(0))
	if err == nil {
		err = setSwitch(ctx, !on)
	}
	if err != nil {
		return ctx.WriteError(err)
	}
	return ctx.WriteInteger(boolInt(!on))
}

func switchTo(ctx *Context, on bool) error {
	if ctx.ArgCount() != 1 {
		return ctx.WriteError(ErrWrongArgCount)
	}
	if err := setSwitch(ctx, on); err != nil {
		return ctx.WriteError(err)
	}
	return ctx.WriteOK()
}

func cmdSWITCHON(ctx *Context) error  { return switchTo(ctx, true) }
func cmdSWITCHOFF(ctx *Context) error { return switchTo(ctx, false) }

func cmdSWITCHLIST(ctx *Context) error {
	if ctx.ArgCount() != 0 {
		return ctx.WriteError(ErrWrongArgCount)
	}
	var out []*resp.Value
	for _, name := range markedKeys(ctx, switchMarker) {
		on, err := switchState(ctx, name)
		if err != nil {
			continue
		}
		state := "off"
		if on {
			state = "on"
		}
		out = append(out, resp.ArrayValue([]*resp.Value{
			resp.BulkString("name"), resp.BulkString(name),
			resp.BulkString("state"), resp.BulkString(state),
		}))
	}
	return ctx.WriteArray(out)
}

// --- BOOKMARK ---

func cmdBOOKMARKSET(ctx *Context) error {
	if ctx.ArgCount() != 2 {
		return ctx.WriteError(ErrWrongArgCount)
	}
	if err := createMarked(ctx, ctx.ArgString(0), bookmarkMarker, bookmarkMarker, ctx.ArgString(1), "created_at", i64(ctx.nowMS())); err != nil {
		return ctx.WriteError(err)
	}
	return ctx.WriteOK()
}

func cmdBOOKMARKGET(ctx *Context) error {
	if ctx.ArgCount() != 1 {
		return ctx.WriteError(ErrWrongArgCount)
	}
	r, err := readMarked(ctx, ctx.ArgString(0), bookmarkMarker)
	if err != nil {
		return ctx.WriteError(err)
	}
	if r == nil {
		return ctx.WriteNull()
	}
	return ctx.WriteBulkString(r[bookmarkMarker])
}

func cmdBOOKMARKDELETE(ctx *Context) error { return deleteMarked(ctx, bookmarkMarker) }

func cmdBOOKMARKLIST(ctx *Context) error {
	if ctx.ArgCount() != 0 {
		return ctx.WriteError(ErrWrongArgCount)
	}
	return ctx.WriteArray(bulkStrings(markedKeys(ctx, bookmarkMarker)))
}

// --- DEADLINE ---

// DEADLINE.SET id ttl_ms [callback] sets a deadline ttl_ms from now.
func cmdDEADLINESET(ctx *Context) error {
	if ctx.ArgCount() < 2 || ctx.ArgCount() > 3 {
		return ctx.WriteError(ErrWrongArgCount)
	}
	ttl, err := strconv.ParseInt(ctx.ArgString(1), 10, 64)
	if err != nil || ttl < 0 || ttl > math.MaxInt64/2 {
		return ctx.WriteError(errors.New("ERR ttl must be a non-negative integer (milliseconds)"))
	}
	callback := ""
	if ctx.ArgCount() == 3 {
		callback = ctx.ArgString(2)
	}
	if err := createMarked(ctx, ctx.ArgString(0), deadlineMarker, deadlineMarker, i64(ctx.nowMS()+ttl), "callback", callback); err != nil {
		return ctx.WriteError(err)
	}
	return ctx.WriteOK()
}

// DEADLINE.CHECK id replies the milliseconds left, 0 once passed, -1 for an
// unknown id.
func cmdDEADLINECHECK(ctx *Context) error {
	if ctx.ArgCount() != 1 {
		return ctx.WriteError(ErrWrongArgCount)
	}
	r, err := readMarked(ctx, ctx.ArgString(0), deadlineMarker)
	if err != nil {
		return ctx.WriteError(err)
	}
	if r == nil {
		return ctx.WriteInteger(-1)
	}
	at, err := strconv.ParseInt(r[deadlineMarker], 10, 64)
	if err != nil {
		return ctx.WriteError(store.ErrWrongType)
	}
	return ctx.WriteInteger(max(at-ctx.nowMS(), 0))
}

func cmdDEADLINECANCEL(ctx *Context) error { return deleteMarked(ctx, deadlineMarker) }

func cmdDEADLINELIST(ctx *Context) error {
	if ctx.ArgCount() != 0 {
		return ctx.WriteError(ErrWrongArgCount)
	}
	return ctx.WriteArray(bulkStrings(markedKeys(ctx, deadlineMarker)))
}

// --- ROUTE ---

const routePrefix = "p:"

func readRoutes(ctx *Context, name string) (map[string]string, error) {
	r, err := readMarked(ctx, name, routeMarker)
	if err != nil || r == nil {
		return nil, err
	}
	routes := make(map[string]string, len(r))
	for f, v := range r {
		if p, ok := strings.CutPrefix(f, routePrefix); ok {
			routes[p] = v
		} else if f != routeMarker {
			return nil, store.ErrWrongType
		}
	}
	return routes, nil
}

func sortedKeys(m map[string]string) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}

func cmdROUTEADD(ctx *Context) error {
	if ctx.ArgCount() != 3 {
		return ctx.WriteError(ErrWrongArgCount)
	}
	name := ctx.ArgString(0)
	if _, err := readRoutes(ctx, name); err != nil {
		return ctx.WriteError(err)
	}
	if err := setMarkedFields(ctx, name, routeMarker, routePrefix+ctx.ArgString(1), ctx.ArgString(2)); err != nil {
		return ctx.WriteError(err)
	}
	return ctx.WriteOK()
}

func cmdROUTEREMOVE(ctx *Context) error {
	if ctx.ArgCount() != 2 {
		return ctx.WriteError(ErrWrongArgCount)
	}
	name := ctx.ArgString(0)
	if _, err := readRoutes(ctx, name); err != nil {
		return ctx.WriteError(err)
	}
	return ctx.WriteInteger(int64(deleteMarkedFields(ctx, name, routeMarker, routePrefix+ctx.ArgString(1))))
}

// ROUTE.MATCH name path replies the target of the route equal to path, else
// of the first glob pattern matching it in pattern order, else null.
func cmdROUTEMATCH(ctx *Context) error {
	if ctx.ArgCount() != 2 {
		return ctx.WriteError(ErrWrongArgCount)
	}
	routes, err := readRoutes(ctx, ctx.ArgString(0))
	if err != nil {
		return ctx.WriteError(err)
	}
	path := ctx.ArgString(1)
	if target, ok := routes[path]; ok {
		return ctx.WriteBulkString(target)
	}
	for _, p := range sortedKeys(routes) {
		if matchPattern(path, p) {
			return ctx.WriteBulkString(routes[p])
		}
	}
	return ctx.WriteNull()
}

func cmdROUTELIST(ctx *Context) error {
	if ctx.ArgCount() != 1 {
		return ctx.WriteError(ErrWrongArgCount)
	}
	routes, err := readRoutes(ctx, ctx.ArgString(0))
	if err != nil {
		return ctx.WriteError(err)
	}
	out := make([]*resp.Value, 0, len(routes))
	for _, p := range sortedKeys(routes) {
		out = append(out, resp.ArrayValue([]*resp.Value{
			resp.BulkString("pattern"), resp.BulkString(p),
			resp.BulkString("target"), resp.BulkString(routes[p]),
		}))
	}
	return ctx.WriteArray(out)
}

// --- GATEWAY ---

// GATEWAY.CREATE name makes a gateway keyed by a generated id and replies
// the id.
func cmdGATEWAYCREATE(ctx *Context) error {
	if ctx.ArgCount() != 1 {
		return ctx.WriteError(ErrWrongArgCount)
	}
	id := generateUUID()
	for ctx.Store.Exists(id) {
		id = generateUUID()
	}
	if err := replaceRecord(ctx, id, gatewayMarker, ctx.ArgString(0), "created_at", i64(ctx.nowMS())); err != nil {
		return ctx.WriteError(err)
	}
	return ctx.WriteBulkString(id)
}

func cmdGATEWAYDELETE(ctx *Context) error { return deleteMarked(ctx, gatewayMarker) }

func cmdGATEWAYROUTE(ctx *Context) error {
	if ctx.ArgCount() != 3 {
		return ctx.WriteError(ErrWrongArgCount)
	}
	id := ctx.ArgString(0)
	if _, err := loadMarked(ctx, id, gatewayMarker, "gateway"); err != nil {
		return ctx.WriteError(err)
	}
	if err := updateRecord(ctx, id, "r:"+ctx.ArgString(1), ctx.ArgString(2)); err != nil {
		return ctx.WriteError(err)
	}
	return ctx.WriteOK()
}

func cmdGATEWAYLIST(ctx *Context) error {
	if ctx.ArgCount() != 0 {
		return ctx.WriteError(ErrWrongArgCount)
	}
	var out []*resp.Value
	for _, id := range markedKeys(ctx, gatewayMarker) {
		r, err := readMarked(ctx, id, gatewayMarker)
		if err != nil || r == nil {
			continue
		}
		out = append(out, resp.ArrayValue([]*resp.Value{
			resp.BulkString("id"), resp.BulkString(id),
			resp.BulkString("name"), resp.BulkString(r[gatewayMarker]),
		}))
	}
	return ctx.WriteArray(out)
}

// --- GRID ---

type grid struct {
	width, height int64
	cells         map[string]string
}

func readGrid(ctx *Context, name string) (*grid, error) {
	r, err := readMarked(ctx, name, gridMarker)
	if err != nil || r == nil {
		return nil, err
	}
	w, err1 := strconv.ParseInt(r["width"], 10, 64)
	h, err2 := strconv.ParseInt(r["height"], 10, 64)
	if err1 != nil || err2 != nil {
		return nil, store.ErrWrongType
	}
	g := &grid{width: w, height: h, cells: map[string]string{}}
	for f, v := range r {
		if c, ok := strings.CutPrefix(f, "c:"); ok {
			g.cells[c] = v
		}
	}
	return g, nil
}

func loadGrid(ctx *Context, name string) (*grid, error) {
	g, err := readGrid(ctx, name)
	if err == nil && g == nil {
		err = errors.New("ERR grid not found")
	}
	return g, err
}

// cell parses the coordinates x y, which must lie in the grid.
func (g *grid) cell(xs, ys string) (string, error) {
	x, err1 := strconv.ParseInt(xs, 10, 64)
	y, err2 := strconv.ParseInt(ys, 10, 64)
	if err1 != nil || err2 != nil || x < 0 || y < 0 || x >= g.width || y >= g.height {
		return "", fmt.Errorf("ERR coordinates must be integers within %dx%d", g.width, g.height)
	}
	return strconv.FormatInt(x, 10) + "," + strconv.FormatInt(y, 10), nil
}

// GRID.CREATE name width height makes an empty grid, replacing one at name.
func cmdGRIDCREATE(ctx *Context) error {
	if ctx.ArgCount() != 3 {
		return ctx.WriteError(ErrWrongArgCount)
	}
	w, err1 := strconv.ParseInt(ctx.ArgString(1), 10, 64)
	h, err2 := strconv.ParseInt(ctx.ArgString(2), 10, 64)
	if err1 != nil || err2 != nil || w < 1 || h < 1 {
		return ctx.WriteError(errors.New("ERR width and height must be positive integers"))
	}
	if err := createMarked(ctx, ctx.ArgString(0), gridMarker, gridMarker, "1", "width", i64(w), "height", i64(h)); err != nil {
		return ctx.WriteError(err)
	}
	return ctx.WriteOK()
}

func cmdGRIDSET(ctx *Context) error {
	if ctx.ArgCount() != 4 {
		return ctx.WriteError(ErrWrongArgCount)
	}
	name := ctx.ArgString(0)
	g, err := loadGrid(ctx, name)
	if err != nil {
		return ctx.WriteError(err)
	}
	c, err := g.cell(ctx.ArgString(1), ctx.ArgString(2))
	if err != nil {
		return ctx.WriteError(err)
	}
	if err := updateRecord(ctx, name, "c:"+c, ctx.ArgString(3)); err != nil {
		return ctx.WriteError(err)
	}
	return ctx.WriteOK()
}

func cmdGRIDGET(ctx *Context) error {
	if ctx.ArgCount() != 3 {
		return ctx.WriteError(ErrWrongArgCount)
	}
	g, err := readGrid(ctx, ctx.ArgString(0))
	if err != nil {
		return ctx.WriteError(err)
	}
	if g == nil {
		return ctx.WriteNull()
	}
	c, err := g.cell(ctx.ArgString(1), ctx.ArgString(2))
	if err != nil {
		return ctx.WriteError(err)
	}
	v, ok := g.cells[c]
	if !ok {
		return ctx.WriteNull()
	}
	return ctx.WriteBulkString(v)
}

func cmdGRIDDELETE(ctx *Context) error { return deleteMarked(ctx, gridMarker) }

// GRID.QUERY name replies the set cells, by row then column.
func cmdGRIDQUERY(ctx *Context) error {
	if ctx.ArgCount() != 1 {
		return ctx.WriteError(ErrWrongArgCount)
	}
	g, err := readGrid(ctx, ctx.ArgString(0))
	if err != nil {
		return ctx.WriteError(err)
	}
	if g == nil {
		return ctx.WriteArray([]*resp.Value{})
	}
	type pos struct {
		x, y int64
		c    string
	}
	cells := make([]pos, 0, len(g.cells))
	for c := range g.cells {
		xs, ys, _ := strings.Cut(c, ",")
		x, err1 := strconv.ParseInt(xs, 10, 64)
		y, err2 := strconv.ParseInt(ys, 10, 64)
		if err1 != nil || err2 != nil {
			return ctx.WriteError(store.ErrWrongType)
		}
		cells = append(cells, pos{x, y, c})
	}
	sort.Slice(cells, func(i, j int) bool {
		if cells[i].y != cells[j].y {
			return cells[i].y < cells[j].y
		}
		return cells[i].x < cells[j].x
	})
	out := make([]*resp.Value, len(cells))
	for i, p := range cells {
		out[i] = resp.ArrayValue([]*resp.Value{
			resp.BulkString("coord"), resp.BulkString(p.c),
			resp.BulkString("value"), resp.BulkString(g.cells[p.c]),
		})
	}
	return ctx.WriteArray(out)
}

func cmdGRIDCLEAR(ctx *Context) error {
	if ctx.ArgCount() != 1 {
		return ctx.WriteError(ErrWrongArgCount)
	}
	name := ctx.ArgString(0)
	g, err := loadGrid(ctx, name)
	if err != nil {
		return ctx.WriteError(err)
	}
	if err := replaceRecord(ctx, name, gridMarker, "1", "width", i64(g.width), "height", i64(g.height)); err != nil {
		return ctx.WriteError(err)
	}
	return ctx.WriteOK()
}

// --- logs: GHOST, TAPE, SLICE ---

// readLog returns the entries of the log at key and its other fields, nil
// when the key is missing.
func readLog(ctx *Context, key, marker string, extra ...string) ([]string, map[string]string, error) {
	r, err := readMarked(ctx, key, marker)
	if err != nil || r == nil {
		return nil, nil, err
	}
	n, err := strconv.Atoi(r[marker])
	if err != nil || n < 0 || len(r) != 1+n+len(extra) {
		return nil, nil, store.ErrWrongType
	}
	entries := make([]string, n)
	for i := range entries {
		v, ok := r[strconv.Itoa(i)]
		if !ok {
			return nil, nil, store.ErrWrongType
		}
		entries[i] = v
	}
	for _, f := range extra {
		if _, ok := r[f]; !ok {
			return nil, nil, store.ErrWrongType
		}
	}
	return entries, r, nil
}

// appendLog appends values to the log at key, which holds n entries.
func appendLog(ctx *Context, key, marker string, n int, values ...string) error {
	pairs := make([]string, 0, 2+2*len(values))
	pairs = append(pairs, marker, strconv.Itoa(n+len(values)))
	for i, v := range values {
		pairs = append(pairs, strconv.Itoa(n+i), v)
	}
	return updateRecord(ctx, key, pairs...)
}

// logHead reads only the entry count of the log at key and the extra
// fields named, so appends and positioned reads do not load every entry.
func logHead(ctx *Context, key, marker string, extra ...string) (int, map[string]string, bool, error) {
	h, err := readMarkedHash(ctx, key, marker)
	if err != nil || h == nil {
		return 0, nil, false, err
	}
	vals := make(map[string]string, len(extra))
	h.RLock()
	raw, _ := h.Get(marker)
	for _, f := range extra {
		if v, ok := h.Get(f); ok {
			vals[f] = string(v)
		}
	}
	h.RUnlock()
	n, err := strconv.Atoi(string(raw))
	if err != nil || n < 0 || len(vals) != len(extra) {
		return 0, nil, false, store.ErrWrongType
	}
	return n, vals, true, nil
}

// logEntry reads entry i of the log at key.
func logEntry(ctx *Context, key, marker string, i int) (string, error) {
	h, err := readMarkedHash(ctx, key, marker)
	if err != nil || h == nil {
		return "", errWrongTypeOr(true, err)
	}
	h.RLock()
	v, ok := h.Get(strconv.Itoa(i))
	h.RUnlock()
	if !ok {
		return "", store.ErrWrongType
	}
	return string(v), nil
}

func cmdGHOSTCREATE(ctx *Context) error {
	if ctx.ArgCount() != 1 {
		return ctx.WriteError(ErrWrongArgCount)
	}
	if err := createMarked(ctx, ctx.ArgString(0), ghostMarker, ghostMarker, "0"); err != nil {
		return ctx.WriteError(err)
	}
	return ctx.WriteOK()
}

func cmdGHOSTWRITE(ctx *Context) error {
	if ctx.ArgCount() != 2 {
		return ctx.WriteError(ErrWrongArgCount)
	}
	id := ctx.ArgString(0)
	n, _, ok, err := logHead(ctx, id, ghostMarker)
	if err == nil && !ok {
		err = errors.New("ERR ghost not found")
	}
	if err == nil {
		err = appendLog(ctx, id, ghostMarker, n, ctx.ArgString(1))
	}
	if err != nil {
		return ctx.WriteError(err)
	}
	return ctx.WriteOK()
}

func cmdGHOSTREAD(ctx *Context) error {
	if ctx.ArgCount() != 1 {
		return ctx.WriteError(ErrWrongArgCount)
	}
	entries, _, err := readLog(ctx, ctx.ArgString(0), ghostMarker)
	if err != nil {
		return ctx.WriteError(err)
	}
	return ctx.WriteArray(bulkStrings(entries))
}

func cmdGHOSTDELETE(ctx *Context) error { return deleteMarked(ctx, ghostMarker) }

func cmdTAPECREATE(ctx *Context) error {
	if ctx.ArgCount() != 1 {
		return ctx.WriteError(ErrWrongArgCount)
	}
	if err := createMarked(ctx, ctx.ArgString(0), tapeMarker, tapeMarker, "0", "pos", "0"); err != nil {
		return ctx.WriteError(err)
	}
	return ctx.WriteOK()
}

func loadTape(ctx *Context, name string) (n, pos int, err error) {
	n, vals, ok, err := logHead(ctx, name, tapeMarker, "pos")
	if err != nil {
		return 0, 0, err
	}
	if !ok {
		return 0, 0, errors.New("ERR tape not found")
	}
	pos, err = strconv.Atoi(vals["pos"])
	if err != nil || pos < 0 {
		return 0, 0, store.ErrWrongType
	}
	return n, pos, nil
}

func cmdTAPEWRITE(ctx *Context) error {
	if ctx.ArgCount() != 2 {
		return ctx.WriteError(ErrWrongArgCount)
	}
	name := ctx.ArgString(0)
	n, _, err := loadTape(ctx, name)
	if err == nil {
		err = appendLog(ctx, name, tapeMarker, n, ctx.ArgString(1))
	}
	if err != nil {
		return ctx.WriteError(err)
	}
	return ctx.WriteOK()
}

// TAPE.READ name replies the entry at the tape's position, null past the end
// or for an unknown tape.
func cmdTAPEREAD(ctx *Context) error {
	if ctx.ArgCount() != 1 {
		return ctx.WriteError(ErrWrongArgCount)
	}
	name := ctx.ArgString(0)
	n, pos, err := loadTape(ctx, name)
	if err != nil {
		if err == store.ErrWrongType {
			return ctx.WriteError(err)
		}
		return ctx.WriteNull()
	}
	if pos >= n {
		return ctx.WriteNull()
	}
	v, err := logEntry(ctx, name, tapeMarker, pos)
	if err != nil {
		return ctx.WriteError(err)
	}
	return ctx.WriteBulkString(v)
}

func cmdTAPESEEK(ctx *Context) error {
	if ctx.ArgCount() != 2 {
		return ctx.WriteError(ErrWrongArgCount)
	}
	name := ctx.ArgString(0)
	n, _, err := loadTape(ctx, name)
	if err != nil {
		return ctx.WriteError(err)
	}
	pos, err := strconv.Atoi(ctx.ArgString(1))
	if err != nil || pos < 0 || pos >= n {
		return ctx.WriteError(errors.New("ERR invalid tape position"))
	}
	if err := updateRecord(ctx, name, "pos", strconv.Itoa(pos)); err != nil {
		return ctx.WriteError(err)
	}
	return ctx.WriteOK()
}

func cmdTAPEDELETE(ctx *Context) error { return deleteMarked(ctx, tapeMarker) }

func cmdSLICECREATE(ctx *Context) error {
	if ctx.ArgCount() != 1 {
		return ctx.WriteError(ErrWrongArgCount)
	}
	if err := createMarked(ctx, ctx.ArgString(0), sliceMarker, sliceMarker, "0"); err != nil {
		return ctx.WriteError(err)
	}
	return ctx.WriteOK()
}

// SLICE.APPEND name value [value ...] appends, creating the slice.
func cmdSLICEAPPEND(ctx *Context) error {
	if ctx.ArgCount() < 2 {
		return ctx.WriteError(ErrWrongArgCount)
	}
	name := ctx.ArgString(0)
	n, _, _, err := logHead(ctx, name, sliceMarker)
	if err != nil {
		return ctx.WriteError(err)
	}
	values := make([]string, 0, ctx.ArgCount()-1)
	for i := 1; i < ctx.ArgCount(); i++ {
		values = append(values, ctx.ArgString(i))
	}
	if err := appendLog(ctx, name, sliceMarker, n, values...); err != nil {
		return ctx.WriteError(err)
	}
	return ctx.WriteOK()
}

func cmdSLICEGET(ctx *Context) error {
	if ctx.ArgCount() != 1 {
		return ctx.WriteError(ErrWrongArgCount)
	}
	entries, _, err := readLog(ctx, ctx.ArgString(0), sliceMarker)
	if err != nil {
		return ctx.WriteError(err)
	}
	return ctx.WriteArray(bulkStrings(entries))
}

func cmdSLICEDELETE(ctx *Context) error { return deleteMarked(ctx, sliceMarker) }

// --- ROLLUPX ---

func loadRollupX(ctx *Context, name string) (count int64, sum float64, ok bool, err error) {
	r, err := readMarked(ctx, name, rollupXMarker)
	if err != nil || r == nil {
		return 0, 0, false, err
	}
	count, err1 := strconv.ParseInt(r["count"], 10, 64)
	sum, err2 := strconv.ParseFloat(r["sum"], 64)
	if err1 != nil || err2 != nil {
		return 0, 0, false, store.ErrWrongType
	}
	return count, sum, true, nil
}

func cmdROLLUPXCREATE(ctx *Context) error {
	if ctx.ArgCount() != 1 {
		return ctx.WriteError(ErrWrongArgCount)
	}
	if err := createMarked(ctx, ctx.ArgString(0), rollupXMarker, rollupXMarker, "1", "count", "0", "sum", "0"); err != nil {
		return ctx.WriteError(err)
	}
	return ctx.WriteOK()
}

func cmdROLLUPXADD(ctx *Context) error {
	if ctx.ArgCount() != 2 {
		return ctx.WriteError(ErrWrongArgCount)
	}
	v, err := strconv.ParseFloat(ctx.ArgString(1), 64)
	if err != nil || math.IsNaN(v) || math.IsInf(v, 0) {
		return ctx.WriteError(errAggValue)
	}
	name := ctx.ArgString(0)
	count, sum, ok, err := loadRollupX(ctx, name)
	if err == nil && !ok {
		err = errors.New("ERR rollup not found")
	}
	if err == nil {
		err = updateRecord(ctx, name, "count", i64(count+1), "sum", f64(sum+v))
	}
	if err != nil {
		return ctx.WriteError(err)
	}
	return ctx.WriteOK()
}

// ROLLUPX.GET name replies the mean of the values added, "0" when none.
func cmdROLLUPXGET(ctx *Context) error {
	if ctx.ArgCount() != 1 {
		return ctx.WriteError(ErrWrongArgCount)
	}
	count, sum, _, err := loadRollupX(ctx, ctx.ArgString(0))
	if err != nil {
		return ctx.WriteError(err)
	}
	if count == 0 {
		return ctx.WriteBulkString("0")
	}
	return ctx.WriteBulkString(fmt.Sprintf("%.6f", sum/float64(count)))
}

func cmdROLLUPXDELETE(ctx *Context) error { return deleteMarked(ctx, rollupXMarker) }
