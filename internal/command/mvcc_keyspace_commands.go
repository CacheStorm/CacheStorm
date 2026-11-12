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

// SPATIAL, CHAIN, ANALYTICS, ROLLUP, COOLDOWN and QUOTA keep each object in
// a hash key marked by a field naming its kind. Being keys they are per
// database, persisted, replicated and removed by DEL and FLUSHALL; changes
// are recorded as HSET, HDEL and DEL, so Redis can load the AOF.

const (
	spatialMarker   = "__spatial"
	chainMarker     = "__chain"
	analyticsMarker = "__analytics"
	rollupMarker    = "__rollup"
	cooldownMarker  = "__cooldown"
	quotaMarker     = "__quota"
)

// removeFields deletes fields of the marked hash at key, keeping the key
// even when only its marker is left; recorded as HDEL.
func removeFields(ctx *Context, key, marker string, fields ...string) (int, error) {
	if _, err := readMarkedHash(ctx, key, marker); err != nil {
		return 0, err
	}
	h, err := lockHash(ctx, key)
	if err != nil || h == nil {
		ctx.PropagateAs()
		return 0, err
	}
	args := [][]byte{[]byte(key)}
	for _, f := range fields {
		if _, ok := h.Get(f); ok && f != marker {
			h.DeleteField(f)
			args = append(args, []byte(f))
		}
	}
	h.Unlock()
	if len(args) == 1 {
		ctx.PropagateAs()
		return 0, nil
	}
	ctx.notify(store.NotifyHash, "hdel", key)
	ctx.PropagateAs(ExecutedCommand{Name: "HDEL", Args: args, DB: ctx.DB})
	return len(args) - 1, nil
}

func parseFloatArgs(args ...string) ([]float64, error) {
	out := make([]float64, len(args))
	for i, a := range args {
		v, err := parseFiniteFloat(a)
		if err != nil {
			return nil, err
		}
		out[i] = v
	}
	return out, nil
}

// --- SPATIAL ---

// A spatial index holds each point as a field named by its id, valued
// "lat lon data".

type spatialPoint struct {
	id       string
	lat, lon float64
}

func loadSpatial(ctx *Context, name string) ([]spatialPoint, error) {
	r, err := loadMarked(ctx, name, spatialMarker, "spatial index")
	if err != nil {
		return nil, err
	}
	points := make([]spatialPoint, 0, len(r)-1)
	for _, id := range sortedKeys(r) {
		if id == spatialMarker {
			continue
		}
		parts := strings.SplitN(r[id], " ", 3)
		if len(parts) < 2 {
			return nil, store.ErrWrongType
		}
		lat, err1 := strconv.ParseFloat(parts[0], 64)
		lon, err2 := strconv.ParseFloat(parts[1], 64)
		if err1 != nil || err2 != nil {
			return nil, store.ErrWrongType
		}
		points = append(points, spatialPoint{id, lat, lon})
	}
	return points, nil
}

// SPATIAL.CREATE name makes an empty index, replacing one at name.
func cmdSPATIALCREATE(ctx *Context) error {
	if ctx.ArgCount() != 1 {
		return ctx.WriteError(ErrWrongArgCount)
	}
	if err := createMarked(ctx, ctx.ArgString(0), spatialMarker, spatialMarker, "1"); err != nil {
		return ctx.WriteError(err)
	}
	return ctx.WriteOK()
}

// SPATIAL.ADD name id lat lon [data] sets the point.
func cmdSPATIALADD(ctx *Context) error {
	if ctx.ArgCount() != 4 && ctx.ArgCount() != 5 {
		return ctx.WriteError(ErrWrongArgCount)
	}
	name, id := ctx.ArgString(0), ctx.ArgString(1)
	if err := reservedField(spatialMarker, id); err != nil {
		return ctx.WriteError(err)
	}
	ll, err := parseFloatArgs(ctx.ArgString(2), ctx.ArgString(3))
	if err != nil {
		return ctx.WriteError(err)
	}
	if ll[0] < -90 || ll[0] > 90 || ll[1] < -180 || ll[1] > 180 {
		return ctx.WriteError(errors.New("ERR invalid latitude or longitude"))
	}
	if _, err := loadMarked(ctx, name, spatialMarker, "spatial index"); err != nil {
		return ctx.WriteError(err)
	}
	data := ""
	if ctx.ArgCount() == 5 {
		data = ctx.ArgString(4)
	}
	if err := updateRecord(ctx, name, id, f64(ll[0])+" "+f64(ll[1])+" "+data); err != nil {
		return ctx.WriteError(err)
	}
	return ctx.WriteOK()
}

// SPATIAL.NEARBY name lat lon radius_km: the ids within radius and their
// distances, nearest first.
func cmdSPATIALNEARBY(ctx *Context) error {
	if ctx.ArgCount() != 4 {
		return ctx.WriteError(ErrWrongArgCount)
	}
	q, err := parseFloatArgs(ctx.ArgString(1), ctx.ArgString(2), ctx.ArgString(3))
	if err != nil {
		return ctx.WriteError(err)
	}
	points, err := loadSpatial(ctx, ctx.ArgString(0))
	if err != nil {
		return ctx.WriteError(err)
	}
	type hit struct {
		id   string
		dist float64
	}
	var hits []hit
	for _, p := range points {
		if d := haversine(q[0], q[1], p.lat, p.lon); d <= q[2] {
			hits = append(hits, hit{p.id, d})
		}
	}
	sort.SliceStable(hits, func(i, j int) bool { return hits[i].dist < hits[j].dist })
	out := make([]*resp.Value, 0, 2*len(hits))
	for _, h := range hits {
		out = append(out, resp.BulkString(h.id), resp.BulkString(fmt.Sprintf("%.6f", h.dist)))
	}
	return ctx.WriteArray(out)
}

// SPATIAL.WITHIN name min_lat min_lon max_lat max_lon: the ids inside the
// box, sorted.
func cmdSPATIALWITHIN(ctx *Context) error {
	if ctx.ArgCount() != 5 {
		return ctx.WriteError(ErrWrongArgCount)
	}
	b, err := parseFloatArgs(ctx.ArgString(1), ctx.ArgString(2), ctx.ArgString(3), ctx.ArgString(4))
	if err != nil {
		return ctx.WriteError(err)
	}
	points, err := loadSpatial(ctx, ctx.ArgString(0))
	if err != nil {
		return ctx.WriteError(err)
	}
	out := make([]*resp.Value, 0)
	for _, p := range points {
		if p.lat >= b[0] && p.lat <= b[2] && p.lon >= b[1] && p.lon <= b[3] {
			out = append(out, resp.BulkString(p.id))
		}
	}
	return ctx.WriteArray(out)
}

// SPATIAL.DELETE name id removes the point: 1 when it was there.
func cmdSPATIALDELETE(ctx *Context) error {
	if ctx.ArgCount() != 2 {
		return ctx.WriteError(ErrWrongArgCount)
	}
	n, err := removeFields(ctx, ctx.ArgString(0), spatialMarker, ctx.ArgString(1))
	if err != nil {
		return ctx.WriteError(err)
	}
	return ctx.WriteInteger(int64(n))
}

func cmdSPATIALLIST(ctx *Context) error { return listMarked(ctx, spatialMarker) }

// --- CHAIN ---

// A chain is a log: the marker holds the block count and block i is the
// field i, valued "timestamp prev_hash hash data".

const chainGenesisPrevHash = "0"

type Block struct {
	Index     int64
	Timestamp int64
	Data      string
	PrevHash  string
	Hash      string
}

func (b *Block) encode() string {
	return i64(b.Timestamp) + " " + b.PrevHash + " " + b.Hash + " " + b.Data
}

func decodeBlock(i int, v string) (*Block, error) {
	parts := strings.SplitN(v, " ", 4)
	if len(parts) != 4 {
		return nil, store.ErrWrongType
	}
	ts, err := strconv.ParseInt(parts[0], 10, 64)
	if err != nil {
		return nil, store.ErrWrongType
	}
	return &Block{Index: int64(i), Timestamp: ts, PrevHash: parts[1], Hash: parts[2], Data: parts[3]}, nil
}

func chainBlock(ctx *Context, name string, i int) (*Block, error) {
	v, err := logEntry(ctx, name, chainMarker, i)
	if err != nil {
		return nil, err
	}
	return decodeBlock(i, v)
}

// CHAIN.CREATE name makes an empty chain, replacing one at name.
func cmdCHAINCREATE(ctx *Context) error {
	if ctx.ArgCount() != 1 {
		return ctx.WriteError(ErrWrongArgCount)
	}
	if err := createMarked(ctx, ctx.ArgString(0), chainMarker, chainMarker, "0"); err != nil {
		return ctx.WriteError(err)
	}
	return ctx.WriteOK()
}

// CHAIN.ADD name data appends a block: its index.
func cmdCHAINADD(ctx *Context) error {
	if ctx.ArgCount() != 2 {
		return ctx.WriteError(ErrWrongArgCount)
	}
	name := ctx.ArgString(0)
	n, _, ok, err := logHead(ctx, name, chainMarker)
	if err != nil || !ok {
		return ctx.WriteError(orNotFound(err, "chain"))
	}
	b := &Block{Index: int64(n), Timestamp: ctx.nowMS(), Data: ctx.ArgString(1), PrevHash: chainGenesisPrevHash}
	if n > 0 {
		last, err := chainBlock(ctx, name, n-1)
		if err != nil {
			return ctx.WriteError(err)
		}
		b.PrevHash = last.Hash
	}
	b.Hash = computeBlockHash(b)
	if err := appendLog(ctx, name, chainMarker, n, b.encode()); err != nil {
		return ctx.WriteError(err)
	}
	return ctx.WriteInteger(b.Index)
}

// chainBlockAt returns block at(n) of the chain, n its length; nil when
// at(n) is out of range.
func chainBlockAt(ctx *Context, name string, at func(n int) int64) (*Block, error) {
	n, _, ok, err := logHead(ctx, name, chainMarker)
	if err != nil || !ok {
		return nil, err
	}
	i := at(n)
	if i < 0 || i >= int64(n) {
		return nil, nil
	}
	return chainBlock(ctx, name, int(i))
}

// CHAIN.GET name index: the block, nil when out of range.
func cmdCHAINGET(ctx *Context) error {
	if ctx.ArgCount() != 2 {
		return ctx.WriteError(ErrWrongArgCount)
	}
	i, err := parseInt64Arg(ctx.ArgString(1))
	if err != nil {
		return ctx.WriteError(err)
	}
	name := ctx.ArgString(0)
	if _, _, ok, err := logHead(ctx, name, chainMarker); err != nil || !ok {
		return ctx.WriteError(orNotFound(err, "chain"))
	}
	b, err := chainBlockAt(ctx, name, func(int) int64 { return i })
	if err != nil {
		return ctx.WriteError(err)
	}
	if b == nil {
		return ctx.WriteNull()
	}
	return ctx.WriteArray([]*resp.Value{
		resp.BulkString("index"), resp.IntegerValue(b.Index),
		resp.BulkString("timestamp"), resp.IntegerValue(b.Timestamp),
		resp.BulkString("data"), resp.BulkString(b.Data),
		resp.BulkString("hash"), resp.BulkString(b.Hash),
		resp.BulkString("prev_hash"), resp.BulkString(b.PrevHash),
	})
}

func orNotFound(err error, what string) error {
	if err != nil {
		return err
	}
	return fmt.Errorf("ERR %s not found", what)
}

// CHAIN.VALIDATE name: 1 when every block links to the one before and
// hashes to its recorded hash.
func cmdCHAINVALIDATE(ctx *Context) error {
	if ctx.ArgCount() != 1 {
		return ctx.WriteError(ErrWrongArgCount)
	}
	entries, _, err := readLog(ctx, ctx.ArgString(0), chainMarker)
	if err != nil || entries == nil {
		return ctx.WriteError(orNotFound(err, "chain"))
	}
	blocks := make([]*Block, len(entries))
	for i, v := range entries {
		b, err := decodeBlock(i, v)
		if err != nil {
			return ctx.WriteInteger(0)
		}
		blocks[i] = b
	}
	return ctx.WriteInteger(boolInt(validateBlocks(blocks)))
}

// CHAIN.LENGTH name: the block count, 0 for a missing chain.
func cmdCHAINLENGTH(ctx *Context) error {
	if ctx.ArgCount() != 1 {
		return ctx.WriteError(ErrWrongArgCount)
	}
	n, _, _, err := logHead(ctx, ctx.ArgString(0), chainMarker)
	if err != nil {
		return ctx.WriteError(err)
	}
	return ctx.WriteInteger(int64(n))
}

// CHAIN.LAST name: the last block, nil when the chain is missing or empty.
func cmdCHAINLAST(ctx *Context) error {
	if ctx.ArgCount() != 1 {
		return ctx.WriteError(ErrWrongArgCount)
	}
	b, err := chainBlockAt(ctx, ctx.ArgString(0), func(n int) int64 { return int64(n) - 1 })
	if err != nil {
		return ctx.WriteError(err)
	}
	if b == nil {
		return ctx.WriteNull()
	}
	return ctx.WriteArray([]*resp.Value{
		resp.BulkString("index"), resp.IntegerValue(b.Index),
		resp.BulkString("data"), resp.BulkString(b.Data),
		resp.BulkString("hash"), resp.BulkString(b.Hash),
	})
}

// --- ANALYTICS ---

// A series keeps the count, sum, min and max of the values recorded.

type analyticsData struct {
	count         int64
	sum, min, max float64
}

func readAnalytics(ctx *Context, name string) (*analyticsData, error) {
	r, err := readMarked(ctx, name, analyticsMarker)
	if err != nil || r == nil {
		return nil, err
	}
	rec := &record{vals: r}
	d := &analyticsData{rec.int("count"), rec.float("sum"), rec.float("min"), rec.float("max")}
	return d, errWrongTypeOr(rec.bad, nil)
}

func analyticsAdd(ctx *Context, sign float64) error {
	if ctx.ArgCount() != 2 {
		return ctx.WriteError(ErrWrongArgCount)
	}
	v, err := parseFiniteFloat(ctx.ArgString(1))
	if err != nil {
		return ctx.WriteError(err)
	}
	v *= sign
	name := ctx.ArgString(0)
	d, err := readAnalytics(ctx, name)
	if err != nil {
		return ctx.WriteError(err)
	}
	if d == nil {
		d = &analyticsData{min: v, max: v}
	}
	d.count++
	d.sum += v
	d.min = min(d.min, v)
	d.max = max(d.max, v)
	if err := setMarkedFields(ctx, name, analyticsMarker, "count", i64(d.count), "sum", f64(d.sum), "min", f64(d.min), "max", f64(d.max)); err != nil {
		return ctx.WriteError(err)
	}
	return ctx.WriteInteger(d.count)
}

func cmdANALYTICSINCR(ctx *Context) error { return analyticsAdd(ctx, 1) }
func cmdANALYTICSDECR(ctx *Context) error { return analyticsAdd(ctx, -1) }

func analyticsStat(ctx *Context, missing *resp.Value, stat func(*analyticsData) *resp.Value) error {
	if ctx.ArgCount() != 1 {
		return ctx.WriteError(ErrWrongArgCount)
	}
	d, err := readAnalytics(ctx, ctx.ArgString(0))
	if err != nil {
		return ctx.WriteError(err)
	}
	if d == nil {
		if missing == nil {
			return ctx.WriteNull()
		}
		return ctx.WriteValue(missing)
	}
	return ctx.WriteValue(stat(d))
}

func fmt2(v float64) *resp.Value { return resp.BulkString(fmt.Sprintf("%.2f", v)) }

func cmdANALYTICSGET(ctx *Context) error {
	return analyticsStat(ctx, nil, func(d *analyticsData) *resp.Value {
		return resp.ArrayValue([]*resp.Value{
			resp.BulkString("count"), resp.IntegerValue(d.count),
			resp.BulkString("sum"), fmt2(d.sum),
			resp.BulkString("min"), fmt2(d.min),
			resp.BulkString("max"), fmt2(d.max),
		})
	})
}

func cmdANALYTICSSUM(ctx *Context) error {
	return analyticsStat(ctx, resp.BulkString("0"), func(d *analyticsData) *resp.Value { return fmt2(d.sum) })
}

func cmdANALYTICSAVG(ctx *Context) error {
	return analyticsStat(ctx, resp.BulkString("0"), func(d *analyticsData) *resp.Value {
		if d.count == 0 {
			return resp.BulkString("0")
		}
		return fmt2(d.sum / float64(d.count))
	})
}

func cmdANALYTICSMIN(ctx *Context) error {
	return analyticsStat(ctx, resp.BulkString("0"), func(d *analyticsData) *resp.Value { return fmt2(d.min) })
}

func cmdANALYTICSMAX(ctx *Context) error {
	return analyticsStat(ctx, resp.BulkString("0"), func(d *analyticsData) *resp.Value { return fmt2(d.max) })
}

func cmdANALYTICSCOUNT(ctx *Context) error {
	return analyticsStat(ctx, resp.IntegerValue(0), func(d *analyticsData) *resp.Value { return resp.IntegerValue(d.count) })
}

// ANALYTICS.CLEAR name drops the series; OK.
func cmdANALYTICSCLEAR(ctx *Context) error {
	if ctx.ArgCount() != 1 {
		return ctx.WriteError(ErrWrongArgCount)
	}
	name := ctx.ArgString(0)
	d, err := readAnalytics(ctx, name)
	if err != nil {
		return ctx.WriteError(err)
	}
	if d == nil {
		ctx.PropagateAs()
		return ctx.WriteOK()
	}
	ctx.Store.Delete(name)
	ctx.notify(store.NotifyGeneric, "del", name)
	ctx.propagateDelete(name)
	return ctx.WriteOK()
}

// --- ROLLUP ---

// A rollup's marker holds its interval and each bucket is a field named by
// its start time, valued the sum of the values added to it.

func rollupBucket(ctx *Context) (string, string, error) {
	name := ctx.ArgString(0)
	ts, err := parseInt64Arg(ctx.ArgString(1))
	if err != nil {
		return "", "", err
	}
	r, err := readMarkedHash(ctx, name, rollupMarker)
	if err == nil && r == nil {
		err = errors.New("ERR rollup not found")
	}
	if err != nil {
		return "", "", err
	}
	r.RLock()
	raw, _ := r.Get(rollupMarker)
	r.RUnlock()
	interval, err := strconv.ParseInt(string(raw), 10, 64)
	if err != nil || interval <= 0 {
		return "", "", store.ErrWrongType
	}
	bucket := ts / interval * interval
	if ts < 0 && ts%interval != 0 {
		bucket -= interval
	}
	b := i64(bucket)
	r.RLock()
	v, _ := r.Get(b)
	r.RUnlock()
	return b, string(v), nil
}

// ROLLUP.CREATE name interval makes an empty rollup, replacing one at name.
func cmdROLLUPCREATE(ctx *Context) error {
	if ctx.ArgCount() != 2 {
		return ctx.WriteError(ErrWrongArgCount)
	}
	interval, err := strconv.ParseInt(ctx.ArgString(1), 10, 64)
	if err != nil || interval <= 0 {
		return ctx.WriteError(errors.New("ERR interval must be positive"))
	}
	if err := createMarked(ctx, ctx.ArgString(0), rollupMarker, rollupMarker, i64(interval)); err != nil {
		return ctx.WriteError(err)
	}
	return ctx.WriteOK()
}

// ROLLUP.ADD name timestamp value adds value to the timestamp's bucket.
func cmdROLLUPADD(ctx *Context) error {
	if ctx.ArgCount() != 3 {
		return ctx.WriteError(ErrWrongArgCount)
	}
	v, err := parseFiniteFloat(ctx.ArgString(2))
	if err != nil {
		return ctx.WriteError(err)
	}
	bucket, cur, err := rollupBucket(ctx)
	if err != nil {
		return ctx.WriteError(err)
	}
	if cur != "" {
		sum, err := strconv.ParseFloat(cur, 64)
		if err != nil {
			return ctx.WriteError(store.ErrWrongType)
		}
		v += sum
	}
	if err := updateRecord(ctx, ctx.ArgString(0), bucket, f64(v)); err != nil {
		return ctx.WriteError(err)
	}
	return ctx.WriteOK()
}

// ROLLUP.GET name timestamp: the sum of the timestamp's bucket.
func cmdROLLUPGET(ctx *Context) error {
	if ctx.ArgCount() != 2 {
		return ctx.WriteError(ErrWrongArgCount)
	}
	_, cur, err := rollupBucket(ctx)
	if err != nil {
		return ctx.WriteError(err)
	}
	sum := 0.0
	if cur != "" {
		if sum, err = strconv.ParseFloat(cur, 64); err != nil {
			return ctx.WriteError(store.ErrWrongType)
		}
	}
	return ctx.WriteBulkString(fmt.Sprintf("%.2f", sum))
}

func cmdROLLUPDELETE(ctx *Context) error { return deleteMarked(ctx, rollupMarker) }

// --- COOLDOWN ---

// A cooldown's marker holds its duration and last_used when it last let a
// call through, both in milliseconds.

func readCooldown(ctx *Context, key string) ([]int64, error) {
	r, err := readMarked(ctx, key, cooldownMarker)
	if err != nil || r == nil {
		return nil, err
	}
	rec := &record{vals: r}
	vals := []int64{rec.int(cooldownMarker), rec.int("last_used")}
	return vals, errWrongTypeOr(rec.bad, nil)
}

// COOLDOWN.SET key duration_ms makes a cooldown, replacing one at key.
func cmdCOOLDOWNSET(ctx *Context) error {
	if ctx.ArgCount() != 2 {
		return ctx.WriteError(ErrWrongArgCount)
	}
	d, err := parseNonNegative(ctx.ArgString(1), "duration")
	if err != nil {
		return ctx.WriteError(err)
	}
	if err := createMarked(ctx, ctx.ArgString(0), cooldownMarker, cooldownMarker, i64(d), "last_used", "0"); err != nil {
		return ctx.WriteError(err)
	}
	return ctx.WriteOK()
}

// COOLDOWN.CHECK key: 1 and the cooldown starts again when it is over, else
// [0, remaining_ms]. A missing cooldown is always over.
func cmdCOOLDOWNCHECK(ctx *Context) error {
	if ctx.ArgCount() != 1 {
		return ctx.WriteError(ErrWrongArgCount)
	}
	key := ctx.ArgString(0)
	cd, err := readCooldown(ctx, key)
	if err != nil {
		return ctx.WriteError(err)
	}
	if cd == nil {
		ctx.PropagateAs()
		return ctx.WriteInteger(1)
	}
	now := ctx.nowMS()
	if now >= cd[1]+cd[0] {
		if err := updateRecord(ctx, key, "last_used", i64(now)); err != nil {
			return ctx.WriteError(err)
		}
		return ctx.WriteInteger(1)
	}
	ctx.PropagateAs()
	return ctx.WriteArray([]*resp.Value{resp.IntegerValue(0), resp.IntegerValue(cd[1] + cd[0] - now)})
}

// COOLDOWN.RESET key ends the cooldown: 1 when the key held one.
func cmdCOOLDOWNRESET(ctx *Context) error {
	if ctx.ArgCount() != 1 {
		return ctx.WriteError(ErrWrongArgCount)
	}
	key := ctx.ArgString(0)
	cd, err := readCooldown(ctx, key)
	if err != nil {
		return ctx.WriteError(err)
	}
	if cd == nil {
		ctx.PropagateAs()
		return ctx.WriteInteger(0)
	}
	if err := updateRecord(ctx, key, "last_used", "0"); err != nil {
		return ctx.WriteError(err)
	}
	return ctx.WriteInteger(1)
}

func cmdCOOLDOWNDELETE(ctx *Context) error { return deleteMarked(ctx, cooldownMarker) }

// COOLDOWN.LIST: the keys of the selected database's cooldowns, each
// followed by its duration.
func cmdCOOLDOWNLIST(ctx *Context) error {
	if ctx.ArgCount() != 0 {
		return ctx.WriteError(ErrWrongArgCount)
	}
	out := make([]*resp.Value, 0)
	for _, key := range markedKeys(ctx, cooldownMarker) {
		cd, err := readCooldown(ctx, key)
		if err != nil || cd == nil {
			continue
		}
		out = append(out, resp.BulkString(key), resp.IntegerValue(cd[0]))
	}
	return ctx.WriteArray(out)
}

// --- QUOTA ---

// A quota counts the amount used in a window of window_ms; reset_at is when
// the window ends. A quota with no window never rolls over.

type Quota struct {
	Limit    int64
	Used     int64
	ResetAt  int64
	WindowMs int64
}

// rollQuotaWindow starts a new window once the current one is over.
func rollQuotaWindow(q *Quota, now int64) bool {
	if now < q.ResetAt || q.WindowMs <= 0 {
		return false
	}
	q.Used = 0
	q.ResetAt = now + q.WindowMs
	return true
}

func readQuota(ctx *Context, key string) (*Quota, error) {
	r, err := readMarked(ctx, key, quotaMarker)
	if err != nil || r == nil {
		return nil, err
	}
	rec := &record{vals: r}
	q := &Quota{rec.int("limit"), rec.int("used"), rec.int("reset_at"), rec.int(quotaMarker)}
	return q, errWrongTypeOr(rec.bad, nil)
}

func loadQuota(ctx *Context, key string) (*Quota, error) {
	q, err := readQuota(ctx, key)
	if err == nil && q == nil {
		err = errors.New("ERR quota not found")
	}
	return q, err
}

func saveQuota(ctx *Context, key string, q *Quota) error {
	return updateRecord(ctx, key, "used", i64(q.Used), "reset_at", i64(q.ResetAt))
}

// QUOTA.SET key limit window_ms makes a quota, replacing one at key.
func cmdQUOTASET(ctx *Context) error {
	if ctx.ArgCount() != 3 {
		return ctx.WriteError(ErrWrongArgCount)
	}
	limit, err := parseNonNegative(ctx.ArgString(1), "limit")
	if err != nil {
		return ctx.WriteError(err)
	}
	window, err := parseNonNegative(ctx.ArgString(2), "window")
	if err != nil {
		return ctx.WriteError(err)
	}
	if err := createMarked(ctx, ctx.ArgString(0), quotaMarker, quotaMarker, i64(window), "limit", i64(limit), "used", "0", "reset_at", i64(ctx.nowMS()+window)); err != nil {
		return ctx.WriteError(err)
	}
	return ctx.WriteOK()
}

// QUOTA.CHECK key: the limit, used, remaining and reset_at, rolling the
// window over when it is over.
func cmdQUOTACHECK(ctx *Context) error {
	if ctx.ArgCount() != 1 {
		return ctx.WriteError(ErrWrongArgCount)
	}
	key := ctx.ArgString(0)
	q, err := loadQuota(ctx, key)
	if err != nil {
		return ctx.WriteError(err)
	}
	if rollQuotaWindow(q, ctx.nowMS()) {
		if err := saveQuota(ctx, key, q); err != nil {
			return ctx.WriteError(err)
		}
	} else {
		ctx.PropagateAs()
	}
	return ctx.WriteArray([]*resp.Value{
		resp.BulkString("limit"), resp.IntegerValue(q.Limit),
		resp.BulkString("used"), resp.IntegerValue(q.Used),
		resp.BulkString("remaining"), resp.IntegerValue(q.Limit - q.Used),
		resp.BulkString("reset_at"), resp.IntegerValue(q.ResetAt),
	})
}

// QUOTA.USE key amount uses amount when it fits: 1, else 0.
func cmdQUOTAUSE(ctx *Context) error {
	if ctx.ArgCount() != 2 {
		return ctx.WriteError(ErrWrongArgCount)
	}
	amount, err := parseNonNegative(ctx.ArgString(1), "amount")
	if err != nil {
		return ctx.WriteError(err)
	}
	key := ctx.ArgString(0)
	q, err := loadQuota(ctx, key)
	if err != nil {
		return ctx.WriteError(err)
	}
	rolled := rollQuotaWindow(q, ctx.nowMS())
	allowed := amount <= q.Limit-q.Used
	if allowed {
		q.Used += amount
	}
	if allowed || rolled {
		if err := saveQuota(ctx, key, q); err != nil {
			return ctx.WriteError(err)
		}
	} else {
		ctx.PropagateAs()
	}
	return ctx.WriteInteger(boolInt(allowed))
}

// QUOTA.RESET key clears the amount used: 1 when the key held a quota.
func cmdQUOTARESET(ctx *Context) error {
	if ctx.ArgCount() != 1 {
		return ctx.WriteError(ErrWrongArgCount)
	}
	key := ctx.ArgString(0)
	q, err := readQuota(ctx, key)
	if err != nil {
		return ctx.WriteError(err)
	}
	if q == nil {
		ctx.PropagateAs()
		return ctx.WriteInteger(0)
	}
	if err := updateRecord(ctx, key, "used", "0"); err != nil {
		return ctx.WriteError(err)
	}
	return ctx.WriteInteger(1)
}

func cmdQUOTADELETE(ctx *Context) error { return deleteMarked(ctx, quotaMarker) }

// QUOTA.LIST: the keys of the selected database's quotas, each followed by
// its limit and the amount used.
func cmdQUOTALIST(ctx *Context) error {
	if ctx.ArgCount() != 0 {
		return ctx.WriteError(ErrWrongArgCount)
	}
	out := make([]*resp.Value, 0)
	for _, key := range markedKeys(ctx, quotaMarker) {
		q, err := readQuota(ctx, key)
		if err != nil || q == nil {
			continue
		}
		out = append(out, resp.BulkString(key), resp.IntegerValue(q.Limit), resp.IntegerValue(q.Used))
	}
	return ctx.WriteArray(out)
}
