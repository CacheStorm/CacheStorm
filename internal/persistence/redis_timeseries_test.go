package persistence

import (
	"bytes"
	"encoding/binary"
	"encoding/gob"
	"encoding/hex"
	"fmt"
	"math"
	"math/rand/v2"
	"reflect"
	"strconv"
	"testing"
	"time"

	"github.com/cachestorm/cachestorm/internal/store"
)

func mustHex(t *testing.T, s string) []byte {
	t.Helper()
	b, err := hex.DecodeString(s)
	if err != nil {
		t.Fatal(err)
	}
	return b
}

// tsRecord decodes an RDB record of golden data: type, key, value.
func tsRecord(t *testing.T, rec []byte) *store.TimeSeriesValue {
	t.Helper()
	d := &dumpDecoder{data: rec}
	typ := d.byte()
	d.str()
	v := d.valueOf(typ)
	if d.err != nil || d.pos != len(rec) {
		t.Fatalf("record: %v at %d of %d", d.err, d.pos, len(rec))
	}
	ts, ok := v.(*store.TimeSeriesValue)
	if !ok {
		t.Fatalf("record decoded to %T", v)
	}
	return ts
}

// tsItem is one module data item, with strings decoded.
type tsItem struct {
	op  uint64
	n   uint64
	raw []byte
}

// tsItems lists a module value's items after its key name.
func tsItems(t *testing.T, value []byte) []tsItem {
	t.Helper()
	d := &dumpDecoder{data: value}
	if d.byte() != rdbTypeModule2 {
		t.Fatal("not a module value")
	}
	d.length()
	var out []tsItem
	for d.err == nil {
		op, _ := d.length()
		it := tsItem{op: op}
		switch op {
		case rdbModuleOpcodeEOF:
			return out[1:]
		case rdbModuleOpcodeUInt:
			it.n, _ = d.length()
		case rdbModuleOpcodeDouble:
			it.raw = d.take(8)
		case rdbModuleOpcodeStr:
			b, _ := d.raw()
			it.raw = append([]byte(nil), b...)
		default:
			t.Fatalf("opcode %d", op)
		}
		out = append(out, it)
	}
	t.Fatal(d.err)
	return nil
}

func dumpBody(payload []byte) []byte { return payload[:len(payload)-10] }

// TestRedisTimeSeriesDumpRoundTrip decodes Redis' DUMP payloads and dumps
// them back item for item as Redis does.
func TestRedisTimeSeriesDumpRoundTrip(t *testing.T) {
	for key, h := range goldenTSDump {
		payload := mustHex(t, h)
		v, err := DecodeDump(payload)
		if err != nil {
			t.Fatalf("%s: %v", key, err)
		}
		ts := v.(*store.TimeSeriesValue)
		if len(ts.Rules) != 0 || ts.SourceKey != "" {
			t.Fatalf("%s: a DUMP payload has no cross references", key)
		}
		again, err := EncodeDump(ts)
		if err != nil {
			t.Fatal(err)
		}
		want, got := tsItems(t, dumpBody(payload)), tsItems(t, dumpBody(again))
		if ts.Uncompressed {
			// Redis saves an uncompressed chunk's unused tail as it lies
			// in memory; only the samples are compared.
			back, err := DecodeDump(again)
			if err != nil || !reflect.DeepEqual(back.(*store.TimeSeriesValue).Samples, ts.Samples) {
				t.Fatalf("%s: dumped again: %v", key, err)
			}
			continue
		}
		if !reflect.DeepEqual(want, got) {
			t.Fatalf("%s: dumped again differs from Redis' payload", key)
		}
	}
}

func goldenSeries(t *testing.T) map[string]*store.TimeSeriesValue {
	t.Helper()
	out := map[string]*store.TimeSeriesValue{}
	for key, h := range goldenTSRecord {
		out[key] = tsRecord(t, mustHex(t, h))
	}
	return out
}

// TestRedisTimeSeriesRDBRules loads series with their rules and open
// buckets from Redis' full sync and continues them: the compactions match
// Redis' to the last bit.
func TestRedisTimeSeriesRDBRules(t *testing.T) {
	series := goldenSeries(t)
	g1, g2 := series["tsx:g1"], series["tsx:g2"]
	var rules []string
	for _, r := range g1.Rules {
		rules = append(rules, fmt.Sprintf("%s %s %d %d", r.Dest, r.Aggregation, r.Bucket, r.Align))
	}
	// RedisTimeSeries prepends each rule it loads.
	if fmt.Sprint(rules) != "[tsx:g1:max max 1000 0 tsx:g1:twa twa 70 13 tsx:g1:avg avg 100 0]" {
		t.Fatalf("rules = %v", rules)
	}
	if series["tsx:g1:avg"].SourceKey != "tsx:g1" || series["tsx:g2:std"].SourceKey != "tsx:g2" {
		t.Fatal("source keys lost")
	}
	if g1.Policy() != "LAST" || g1.IgnoreMaxTimeDiff != 5 || g1.IgnoreMaxValDiff != 0.5 || g1.ChunkSize != 128 ||
		fmt.Sprint(g1.LabelList()) != "[[sensor t1] [site 12345] [note "+string(bytes.Repeat([]byte("x"), 30))+"]]" {
		t.Fatalf("settings = %+v", g1)
	}
	if !g2.Uncompressed || g2.Policy() != "SUM" || len(g2.Rules) != 2 {
		t.Fatalf("g2 = %+v", g2)
	}
	add := func(src *store.TimeSeriesValue, at int64, value float64) {
		if _, err := src.Upsert(at, value, ""); err != nil {
			t.Fatal(err)
		}
		for i := range src.Rules {
			r := &src.Rules[i]
			if start, v, ok := r.Feed(at, value); ok {
				series[r.Dest].Upsert(start, v, "LAST") //nolint:errcheck
			}
		}
	}
	at := g1.Samples[len(g1.Samples)-1].Timestamp
	for i := range 30 {
		at += 13
		add(g1, at, float64(i)*1.1-3)
		add(g2, int64(7*(23+i)+3), float64(i)*0.5)
	}
	for key, want := range goldenTSAfter {
		got := series[key].Samples
		if len(got) != len(want) {
			t.Fatalf("%s: %d samples, Redis %d", key, len(got), len(want))
		}
		for i, s := range got {
			if strconv.FormatInt(s.Timestamp, 10) != want[i][0] || store.FormatTSValue(s.Value) != want[i][1] {
				t.Fatalf("%s[%d] = %d %s, Redis %s %s", key, i, s.Timestamp, store.FormatTSValue(s.Value), want[i][0], want[i][1])
			}
		}
	}
}

// TestRedisTimeSeriesRDBRoundTrip saves loaded series with their cross
// references as Redis saves them.
func TestRedisTimeSeriesRDBRoundTrip(t *testing.T) {
	for key, h := range goldenTSRecord {
		rec := mustHex(t, h)
		ts := tsRecord(t, rec)
		if ts.Uncompressed {
			continue
		}
		// Saved in the order Redis loaded them from, so the items line up.
		for i, j := 0, len(ts.Rules)-1; i < j; i, j = i+1, j-1 {
			ts.Rules[i], ts.Rules[j] = ts.Rules[j], ts.Rules[i]
		}
		value := rec[2+len(key):]
		got := appendTimeSeriesValue(nil, key, ts, true)
		if !bytes.Equal(got[1:], value) && !reflect.DeepEqual(tsItems(t, append([]byte{rdbTypeModule2}, value...)), tsItems(t, got)) {
			t.Fatalf("%s: saved again differs from Redis' record", key)
		}
	}
}

// TestRedisTimeSeriesInfoAfterLoad keeps TS.INFO's chunk figures of a
// series Redis saved: its chunks as they were, and memory as Redis counts
// it after loading.
func TestRedisTimeSeriesInfoAfterLoad(t *testing.T) {
	v, err := DecodeDump(mustHex(t, goldenTSDump["tsx:g1"]))
	if err != nil {
		t.Fatal(err)
	}
	ts := v.(*store.TimeSeriesValue)
	counts, sizes := ts.ChunkLayout()
	var total int64
	for _, n := range counts {
		total += n
	}
	if total != int64(len(ts.Samples)) || len(counts) != len(sizes) || ts.Info().ChunkCount != int64(len(counts)) {
		t.Fatalf("layout %v %v for %d samples", counts, sizes, len(ts.Samples))
	}
	// The upsert at 1500 grew its chunk past CHUNK_SIZE, as Redis keeps it.
	grown := false
	for _, s := range sizes {
		grown = grown || s > 128
	}
	if !grown {
		t.Fatalf("sizes = %v", sizes)
	}
}

// TestRedisTimeSeriesRefusesDamage refuses every truncation and survives
// corruption of Redis' payloads without panicking.
func TestRedisTimeSeriesRefusesDamage(t *testing.T) {
	for key, h := range goldenTSRecord {
		rec := mustHex(t, h)
		value := rec[2+len(key):]
		decode := func(b []byte) (err error) {
			defer func() {
				if p := recover(); p != nil {
					err = fmt.Errorf("panic: %v", p)
				}
			}()
			d := &dumpDecoder{data: b}
			v := d.value()
			if d.err == nil && d.pos == len(b) && v != nil {
				return nil
			}
			return ErrDumpFormat
		}
		full := append([]byte{rdbTypeModule2}, value...)
		if err := decode(full); err != nil {
			t.Fatalf("%s: %v", key, err)
		}
		for n := 0; n < len(full); n++ {
			if err := decode(full[:n]); err == nil || err != ErrDumpFormat {
				t.Fatalf("%s: truncated to %d: %v", key, n, err)
			}
		}
		rng := rand.New(rand.NewPCG(1, uint64(len(key))))
		for range 1500 {
			b := append([]byte(nil), full...)
			for range 1 + rng.IntN(3) {
				b[1+rng.IntN(len(b)-1)] ^= byte(1 + rng.IntN(255))
			}
			if err := decode(b); err != nil && err != ErrDumpFormat {
				t.Fatalf("%s: %v", key, err)
			}
		}
	}
}

// tsCrafted writes a series in an older encoding version's layout.
func tsCrafted(encver uint64, uncompressed bool) []byte {
	w := &moduleWriter{out: appendDumpLength([]byte{rdbTypeModule2}, rdbModuleID("TSDB-TYPE", encver))}
	w.buffer([]byte("k"))
	w.uint(0)
	chunk := uint64(256)
	if encver < tsEncSize {
		chunk = 16
	}
	w.uint(chunk)
	if encver >= tsEncUncompressed {
		if uncompressed {
			w.uint(tsOptUncompressed)
		} else {
			w.uint(tsOptCompressedGor)
		}
	}
	samples := [][2]float64{{10, 1}, {20, 2.5}, {35, -4}}
	if encver >= tsEncSize {
		w.uint(35)
		w.double(-4)
		w.uint(3)
	}
	if encver >= tsEncResetDupPol {
		w.uint(4)
	}
	if encver >= tsEncSize {
		w.uint(1)
		w.buffer([]byte("src"))
	}
	if encver >= tsEncVer {
		w.uint(3)
		w.double(1.5)
	}
	w.uint(1)
	w.buffer([]byte("a"))
	w.buffer([]byte("b"))
	w.uint(3)
	for _, agg := range []string{"max", "last", "avg"} {
		w.buffer([]byte("d-" + agg))
		w.uint(10)
		if encver >= tsEncAlignment {
			w.uint(0)
		}
		w.uint(tsAggType(agg))
		w.uint(30)
		switch agg {
		case "max":
			w.double(-4)
			w.double(-4)
			if encver < tsEncAlignment {
				w.buffer([]byte{0})
			}
		case "last":
			w.double(-4)
			if encver >= tsEncResetDupPol && encver < tsEncLastAggEmpty {
				w.uint(0)
			}
		case "avg":
			w.double(-4)
			w.double(1)
			if encver >= tsEncOverflow {
				w.uint(0)
			}
		}
	}
	if encver < tsEncSize {
		w.uint(uint64(len(samples)))
		for _, s := range samples {
			w.uint(uint64(s[0]))
			w.double(s[1])
		}
	} else {
		v := store.NewTimeSeriesValue(0)
		v.Uncompressed = uncompressed
		v.ChunkSize = int64(chunk)
		for _, s := range samples {
			v.Upsert(int64(s[0]), s[1], "") //nolint:errcheck
		}
		appendTSChunks(w, v)
	}
	out := appendDumpLength(w.out, rdbModuleOpcodeEOF)
	out = binary.LittleEndian.AppendUint16(out, 11)
	return binary.LittleEndian.AppendUint64(out, rdbChecksum(0, out))
}

// TestRedisTimeSeriesOlderEncodings reads every encoding version
// RedisTimeSeries 8.2 loads.
func TestRedisTimeSeriesOlderEncodings(t *testing.T) {
	for encver := uint64(0); encver <= tsEncVer; encver++ {
		for _, uncompressed := range []bool{false, true} {
			v, err := DecodeDump(tsCrafted(encver, uncompressed))
			if err != nil {
				t.Fatalf("encver %d: %v", encver, err)
			}
			ts := v.(*store.TimeSeriesValue)
			if fmt.Sprint(ts.Samples) != "[{10 1 map[]} {20 2.5 map[]} {35 -4 map[]}]" || ts.Labels["a"] != "b" || len(ts.Rules) != 3 {
				t.Fatalf("encver %d: %+v", encver, ts)
			}
			if ts.Rules[0].Aggregation != "avg" || ts.Rules[0].Ctx.Count != 1 || ts.Rules[2].Ctx.Max != -4 {
				t.Fatalf("encver %d: rules %+v", encver, ts.Rules)
			}
			if encver == 0 && !ts.Uncompressed || encver >= tsEncUncompressed && ts.Uncompressed != uncompressed {
				t.Fatalf("encver %d: uncompressed %v", encver, ts.Uncompressed)
			}
			if (encver >= tsEncResetDupPol) != (ts.Policy() == "MIN") || (encver >= tsEncSize) != (ts.SourceKey == "src") ||
				(encver >= tsEncVer) != (ts.IgnoreMaxTimeDiff == 3) {
				t.Fatalf("encver %d: %+v", encver, ts)
			}
		}
	}
	if _, err := DecodeDump(tsCrafted(9, false)); err == nil {
		t.Fatal("encoding version 9 must be refused")
	}
}

// TestCStormTimeSeriesStillLoads reads series older CacheStorm versions
// saved: gob under the private type, with rules buffering open samples.
func TestCStormTimeSeriesStillLoads(t *testing.T) {
	legacy := legacyTimeSeries{
		Samples:   []store.TimeSeriesSample{{Timestamp: 5, Value: 1}, {Timestamp: 12, Value: 3}, {Timestamp: 14, Value: 7}},
		Labels:    map[string]string{"a": "1"},
		LabelKeys: []string{"a"}, Retention: time.Minute, DuplicatePolicy: "MAX", ChunkSize: 128, SourceKey: "up",
	}
	prev := store.TimeSeriesSample{Timestamp: 5, Value: 1}
	for _, agg := range []string{"avg", "twa", "std.s"} {
		legacy.Rules = append(legacy.Rules, struct {
			Dest        string
			Aggregation string
			Bucket      int64
			Align       int64
			Open        []store.TimeSeriesSample
			Prev        *store.TimeSeriesSample
		}{"d-" + agg, agg, 10, 0, legacy.Samples[1:], &prev})
	}
	var data bytes.Buffer
	if err := gob.NewEncoder(&data).Encode(legacy); err != nil {
		t.Fatal(err)
	}
	out := appendDumpLength([]byte{dumpTypeTimeSeries}, uint64(data.Len()))
	out = append(out, data.Bytes()...)
	out = binary.LittleEndian.AppendUint16(out, 11)
	out = binary.LittleEndian.AppendUint64(out, rdbChecksum(0, out))
	v, err := DecodeDump(out)
	if err != nil {
		t.Fatal(err)
	}
	ts := v.(*store.TimeSeriesValue)
	if len(ts.Samples) != 3 || ts.SourceKey != "up" || ts.Policy() != "MAX" || ts.Retention != time.Minute || len(ts.Rules) != 3 {
		t.Fatalf("legacy series = %+v", ts)
	}
	// The open bucket closes with what the buffered samples aggregate to.
	for i, want := range []float64{5, (3 + 7) / 2.0, math.Sqrt(8)} {
		r := &ts.Rules[i]
		if !r.Started || r.Start != 10 {
			t.Fatalf("%s: not open at 10", r.Aggregation)
		}
		_, got, closed := r.Feed(25, 0)
		if r.Aggregation == "twa" {
			// (1 at 5 .. 3 at 12) gives 2.4285714 at 10; up to 20 the line to 0 at 25.
			want = twaWant()
		}
		if !closed || math.Abs(got-want) > 1e-12 {
			t.Fatalf("%s closed %v at %v, want %v", r.Aggregation, closed, got, want)
		}
	}
}

func twaWant() float64 {
	vab := 1 + float64(10-5)*(3-1)/float64(12-5)
	res := (vab + 3) * float64(12-10) / 2
	res += (3 + 7) * float64(14-12) / 2
	vb := 7 + float64(20-14)*(0-7)/float64(25-14)
	res += (vb + 7) * float64(20-14) / 2
	return res / 10
}

// TestTimeSeriesSnapshotKeepsRules saves a series in an RDB snapshot and the
// AOF rewrite's RESTORE payload with its rules, as Redis persists them.
func TestTimeSeriesSnapshotKeepsRules(t *testing.T) {
	src := store.NewTimeSeriesValue(0)
	src.Rules = []store.TSRule{{Dest: "d", Aggregation: "twa", Bucket: 10}}
	for i := range 25 {
		src.Upsert(int64(i*3), float64(i), "") //nolint:errcheck
		src.Rules[0].Feed(int64(i*3), float64(i))
	}
	dest := store.NewTimeSeriesValue(0)
	dest.SourceKey = "s"
	out, _, err := EncodeRedisRDB(entryList{{0, "s", store.NewEntry(src)}, {0, "d", store.NewEntry(dest)}}, RedisRDBInfo{})
	if err != nil {
		t.Fatal(err)
	}
	if err := SealRDBChecksum(out); err != nil {
		t.Fatal(err)
	}
	dbs := store.NewDatabases(store.NewStore(), 16)
	if err := NewRDBReaderForDatabases(dbs).LoadBytes(out); err != nil {
		t.Fatal(err)
	}
	e, ok := dbs.DB(0).Get("s")
	if !ok {
		t.Fatal("series lost")
	}
	got := e.Value.(*store.TimeSeriesValue)
	if len(got.Rules) != 1 || !reflect.DeepEqual(got.Rules[0], src.Rules[0]) {
		t.Fatalf("rules = %+v, want %+v", got.Rules, src.Rules)
	}
	if e, _ := dbs.DB(0).Get("d"); e.Value.(*store.TimeSeriesValue).SourceKey != "s" {
		t.Fatal("source key lost")
	}
	back, err := DecodeDump(encodeTimeSeriesRestore("s", src))
	if err != nil || !reflect.DeepEqual(back.(*store.TimeSeriesValue).Rules, src.Rules) {
		t.Fatalf("RESTORE payload: %v", err)
	}
}
