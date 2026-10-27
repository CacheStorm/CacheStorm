package command

import (
	"bytes"
	"fmt"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/cachestorm/cachestorm/internal/resp"
	"github.com/cachestorm/cachestorm/internal/store"
)

func auditFix2CRouter() *Router {
	r := NewRouter()
	RegisterStringCommands(r)
	RegisterKeyCommands(r)
	RegisterExtraCommands(r)
	RegisterMoreCommands(r)
	RegisterMonitoringCommands(r)
	RegisterStatsCommands(r)
	RegisterUtilityExtCommands(r)
	RegisterMLCommands(r)
	RegisterMVCCCommands(r)
	return r
}

func auditFix2CRun(t *testing.T, r *Router, s *store.Store, args ...string) *resp.Value {
	t.Helper()
	var buf bytes.Buffer
	w := resp.NewWriter(&buf)
	argv := make([][]byte, len(args)-1)
	for i, a := range args[1:] {
		argv[i] = []byte(a)
	}
	ctx := NewContext(args[0], argv, s, w)
	if err := r.Execute(ctx); err != nil {
		t.Fatalf("%v: Execute error: %v", args, err)
	}
	_ = w.Flush()
	v, err := resp.NewReader(bytes.NewReader(buf.Bytes())).ReadValue()
	if err != nil {
		t.Fatalf("%v: parse reply %q: %v", args, buf.String(), err)
	}
	return v
}

func auditFix2CErr(v *resp.Value) string {
	if v.Type != resp.TypeError {
		return ""
	}
	if v.Str != "" {
		return v.Str
	}
	return v.Err
}

func auditFix2CStr(v *resp.Value) string {
	if v.Type == resp.TypeBulkString {
		return string(v.Bulk)
	}
	return v.Str
}

func auditFix2CStrings(t *testing.T, v *resp.Value) []string {
	t.Helper()
	if v.Type != resp.TypeArray {
		t.Fatalf("want array, got %v (%s)", v.Type, auditFix2CErr(v))
	}
	out := make([]string, len(v.Array))
	for i, e := range v.Array {
		out[i] = auditFix2CStr(e)
	}
	return out
}

func auditFix2CField(t *testing.T, v *resp.Value, name string) *resp.Value {
	t.Helper()
	if v.Type != resp.TypeArray {
		t.Fatalf("want array, got %v (%s)", v.Type, auditFix2CErr(v))
	}
	for i := 0; i+1 < len(v.Array); i += 2 {
		if auditFix2CStr(v.Array[i]) == name {
			return v.Array[i+1]
		}
	}
	t.Fatalf("field %q not found", name)
	return nil
}

func TestAuditFix2CORSetObservedRemove(t *testing.T) {
	r := auditFix2CRouter()
	s := store.NewStore()
	name := fmt.Sprintf("orset-%d", time.Now().UnixNano())

	tagA := auditFix2CStr(auditFix2CRun(t, r, s, "CRDT.ORSET.ADD", name, "x", "nodeA:1"))
	if tagA != "nodeA:1" {
		t.Fatalf("explicit tag reply = %q", tagA)
	}
	observed := auditFix2CStrings(t, auditFix2CRun(t, r, s, "CRDT.ORSET.TAGS", name, "x"))
	auditFix2CRun(t, r, s, "CRDT.ORSET.ADD", name, "x", "nodeB:1")

	removeArgs := append([]string{"CRDT.ORSET.REMOVE", name, "x"}, observed...)
	if v := auditFix2CRun(t, r, s, removeArgs...); v.Int != 1 {
		t.Fatalf("remove of observed tags removed %d, want 1", v.Int)
	}
	if got := auditFix2CStrings(t, auditFix2CRun(t, r, s, "CRDT.ORSET.GET", name)); len(got) != 1 || got[0] != "x" {
		t.Fatalf("concurrent add must win, GET = %v", got)
	}

	auditFix2CRun(t, r, s, "CRDT.ORSET.ADD", name, "x", "nodeA:1")
	if tags := auditFix2CStrings(t, auditFix2CRun(t, r, s, "CRDT.ORSET.TAGS", name, "x")); len(tags) != 1 || tags[0] != "nodeB:1" {
		t.Fatalf("a removed tag must stay removed, tags = %v", tags)
	}

	if v := auditFix2CRun(t, r, s, "CRDT.ORSET.REMOVE", name, "x"); v.Int != 1 {
		t.Fatalf("remove all observed = %d", v.Int)
	}
	if got := auditFix2CStrings(t, auditFix2CRun(t, r, s, "CRDT.ORSET.GET", name)); len(got) != 0 {
		t.Fatalf("GET after full remove = %v", got)
	}
	auto := auditFix2CStr(auditFix2CRun(t, r, s, "CRDT.ORSET.ADD", name, "x"))
	if auto == "" || auto == "nodeA:1" || auto == "nodeB:1" {
		t.Fatalf("auto tag = %q", auto)
	}
	if got := auditFix2CStrings(t, auditFix2CRun(t, r, s, "CRDT.ORSET.GET", name)); len(got) != 1 {
		t.Fatalf("re-add with a fresh tag must be visible, GET = %v", got)
	}
}

func TestAuditFix2CAlertHistory(t *testing.T) {
	r := auditFix2CRouter()
	s := store.NewStore()
	name := fmt.Sprintf("alert-%d", time.Now().UnixNano())
	auditFix2CRun(t, r, s, "ALERT.CREATE", name, "disk full")
	auditFix2CRun(t, r, s, "ALERT.TRIGGER", name)
	auditFix2CRun(t, r, s, "ALERT.ACKNOWLEDGE", name)
	auditFix2CRun(t, r, s, "ALERT.RESOLVE", name)

	hist := auditFix2CField(t, auditFix2CRun(t, r, s, "ALERT.HISTORY", name), "history")
	want := []string{"active", "firing", "acknowledged", "resolved"}
	if len(hist.Array) != len(want) {
		t.Fatalf("history has %d entries, want %d", len(hist.Array), len(want))
	}
	for i, h := range hist.Array {
		if got := auditFix2CStr(auditFix2CField(t, h, "status")); got != want[i] {
			t.Fatalf("history[%d] = %q, want %q", i, got, want[i])
		}
		if auditFix2CField(t, h, "timestamp").Int <= 0 {
			t.Fatalf("history[%d] has no timestamp", i)
		}
	}
}

func TestAuditFix2CExperimentResults(t *testing.T) {
	r := auditFix2CRouter()
	s := store.NewStore()
	name := fmt.Sprintf("exp-%d", time.Now().UnixNano())
	auditFix2CRun(t, r, s, "EXPERIMENT.CREATE", name, "A", "B")

	byVariant := map[string][]string{}
	for i := 0; i < 20; i++ {
		u := fmt.Sprintf("u%d", i)
		v := auditFix2CStr(auditFix2CRun(t, r, s, "EXPERIMENT.ASSIGN", name, u))
		byVariant[v] = append(byVariant[v], u)
	}
	if len(byVariant["A"]) == 0 || len(byVariant["B"]) == 0 {
		t.Fatalf("assignment did not use both variants: %v", byVariant)
	}
	a := byVariant["A"]
	auditFix2CRun(t, r, s, "EXPERIMENT.TRACK", name, a[0], "click")
	auditFix2CRun(t, r, s, "EXPERIMENT.TRACK", name, a[0], "click")
	if v := auditFix2CRun(t, r, s, "EXPERIMENT.TRACK", name, "stranger", "click"); auditFix2CErr(v) == "" {
		t.Fatalf("tracking an unassigned user must fail")
	}

	res := auditFix2CRun(t, r, s, "EXPERIMENT.RESULTS", name)
	if auditFix2CField(t, res, "assignments").Int != 20 {
		t.Fatalf("assignments total wrong")
	}
	if got := auditFix2CField(t, res, "variant_A").Int; got != int64(len(a)) {
		t.Fatalf("variant_A = %d, want %d", got, len(a))
	}
	evA := auditFix2CField(t, res, "events_A")
	if len(evA.Array) != 1 {
		t.Fatalf("events_A = %v", evA.Array)
	}
	if c := auditFix2CField(t, evA.Array[0], "count").Int; c != 2 {
		t.Fatalf("click count = %d, want 2", c)
	}
	if u := auditFix2CField(t, evA.Array[0], "users").Int; u != 1 {
		t.Fatalf("click users = %d, want 1", u)
	}
	evB := auditFix2CField(t, res, "events_B")
	if c := auditFix2CField(t, evB.Array[0], "count").Int; c != 0 {
		t.Fatalf("variant B click count = %d, want 0", c)
	}
}

func TestAuditFix2CStatsKeyspaceExpires(t *testing.T) {
	r := auditFix2CRouter()
	s := store.NewStore()
	auditFix2CRun(t, r, s, "SET", "a", "1")
	auditFix2CRun(t, r, s, "SET", "b", "1")
	auditFix2CRun(t, r, s, "EXPIRE", "b", "100")
	v := auditFix2CRun(t, r, s, "STATS.KEYSPACE")
	if got := auditFix2CField(t, v, "expires").Int; got != 1 {
		t.Fatalf("expires = %d, want 1", got)
	}
	if ttl := auditFix2CField(t, v, "avg_ttl").Int; ttl <= 90_000 || ttl > 100_000 {
		t.Fatalf("avg_ttl = %d ms, want ~100000", ttl)
	}
	mem := auditFix2CRun(t, r, s, "STATS.MEMORY")
	if auditFix2CField(t, mem, "total_allocated").Int <= 0 || auditFix2CField(t, mem, "used_memory_rss").Int <= 0 {
		t.Fatalf("runtime memory figures must be real: %v", auditFix2CStrings(t, mem))
	}
}

func TestAuditFix2CHealthReadiness(t *testing.T) {
	r := auditFix2CRouter()
	s := store.NewStore()

	if v := auditFix2CRun(t, r, s, "HEALTH.READINESS"); v.Str != "OK" {
		t.Fatalf("readiness without replication = %v %q", v.Type, auditFix2CErr(v))
	}
	info := "# Replication\r\nrole:slave\r\nmaster_link_status:down\r\nmaster_sync_in_progress:0\r\n"
	r.SetReplicationInfo(func() string { return info })
	if v := auditFix2CRun(t, r, s, "HEALTH.READINESS"); !strings.HasPrefix(auditFix2CErr(v), "NOTREADY") {
		t.Fatalf("replica with link down must not be ready, got %v %q", v.Type, v.Str)
	}
	info = "# Replication\r\nrole:slave\r\nmaster_link_status:up\r\nmaster_sync_in_progress:1\r\n"
	if v := auditFix2CRun(t, r, s, "HEALTH.READINESS"); !strings.HasPrefix(auditFix2CErr(v), "NOTREADY") {
		t.Fatalf("replica mid-sync must not be ready, got %v %q", v.Type, v.Str)
	}
	info = "# Replication\r\nrole:slave\r\nmaster_link_status:up\r\nmaster_sync_in_progress:0\r\n"
	if v := auditFix2CRun(t, r, s, "HEALTH.READINESS"); v.Str != "OK" {
		t.Fatalf("synced replica = %v %q", v.Type, auditFix2CErr(v))
	}
}

func TestAuditFix2CTDigestRedisBloomSyntax(t *testing.T) {
	r := auditFix2CRouter()
	s := store.NewStore()
	k := fmt.Sprintf("td-%d", time.Now().UnixNano())

	if v := auditFix2CRun(t, r, s, "TDIGEST.ADD", k, "1"); auditFix2CErr(v) != errTDigestNoKey.Error() {
		t.Fatalf("ADD on a missing key = %q", auditFix2CErr(v))
	}
	if v := auditFix2CRun(t, r, s, "TDIGEST.CREATE", k, "200"); auditFix2CErr(v) == "" {
		t.Fatalf("positional compression must be rejected")
	}
	if v := auditFix2CRun(t, r, s, "TDIGEST.CREATE", k, "COMPRESSION", "200"); v.Str != "OK" {
		t.Fatalf("CREATE = %q", auditFix2CErr(v))
	}
	if v := auditFix2CRun(t, r, s, "TDIGEST.CREATE", k); auditFix2CErr(v) != errTDigestKeyExists.Error() {
		t.Fatalf("second CREATE = %q", auditFix2CErr(v))
	}
	if got := auditFix2CStrings(t, auditFix2CRun(t, r, s, "TDIGEST.QUANTILE", k, "0.5")); got[0] != "nan" {
		t.Fatalf("empty quantile = %v", got)
	}
	if v := auditFix2CRun(t, r, s, "TDIGEST.MIN", k); auditFix2CStr(v) != "nan" {
		t.Fatalf("empty MIN = %q", auditFix2CStr(v))
	}
	if v := auditFix2CRun(t, r, s, "TDIGEST.ADD", k, "10", "x"); auditFix2CErr(v) == "" {
		t.Fatalf("ADD with a bad value must fail")
	}
	if v := auditFix2CRun(t, r, s, "TDIGEST.ADD", k, "10", "20", "30", "40", "50", "60"); v.Str != "OK" {
		t.Fatalf("ADD = %q", auditFix2CErr(v))
	}

	if v := auditFix2CRun(t, r, s, "TDIGEST.MIN", k); auditFix2CStr(v) != "10" {
		t.Fatalf("MIN = %q", auditFix2CStr(v))
	}
	if v := auditFix2CRun(t, r, s, "TDIGEST.MAX", k); auditFix2CStr(v) != "60" {
		t.Fatalf("MAX = %q", auditFix2CStr(v))
	}
	q := auditFix2CStrings(t, auditFix2CRun(t, r, s, "TDIGEST.QUANTILE", k, "0", "0.5", "1"))
	if strings.Join(q, ",") != "10,40,60" {
		t.Fatalf("QUANTILE = %v", q)
	}
	if v := auditFix2CRun(t, r, s, "TDIGEST.QUANTILE", k, "1.5"); auditFix2CErr(v) != errTDigestQuantile.Error() {
		t.Fatalf("out-of-range quantile = %q", auditFix2CErr(v))
	}
	cdf := auditFix2CStrings(t, auditFix2CRun(t, r, s, "TDIGEST.CDF", k, "0", "35", "100"))
	if cdf[0] != "0" || cdf[1] != "0.5" || cdf[2] != "1" {
		t.Fatalf("CDF = %v", cdf)
	}

	rank := auditFix2CRun(t, r, s, "TDIGEST.RANK", k, "0", "10", "20", "30", "40", "50", "60", "70")
	revrank := auditFix2CRun(t, r, s, "TDIGEST.REVRANK", k, "0", "10", "20", "30", "40", "50", "60", "70")
	wantRank := []int64{-1, 0, 1, 2, 3, 4, 5, 6}
	wantRev := []int64{6, 5, 4, 3, 2, 1, 0, -1}
	for i := range wantRank {
		if rank.Array[i].Int != wantRank[i] || revrank.Array[i].Int != wantRev[i] {
			t.Fatalf("RANK[%d]=%d REVRANK[%d]=%d, want %d/%d", i, rank.Array[i].Int, i, revrank.Array[i].Int, wantRank[i], wantRev[i])
		}
	}
	by := auditFix2CStrings(t, auditFix2CRun(t, r, s, "TDIGEST.BYRANK", k, "0", "1", "2", "3", "4", "5", "6"))
	if strings.Join(by, ",") != "10,20,30,40,50,60,inf" {
		t.Fatalf("BYRANK = %v", by)
	}
	byRev := auditFix2CStrings(t, auditFix2CRun(t, r, s, "TDIGEST.BYREVRANK", k, "0", "1", "2", "3", "4", "5", "6"))
	if strings.Join(byRev, ",") != "60,50,40,30,20,10,-inf" {
		t.Fatalf("BYREVRANK = %v", byRev)
	}
	if v := auditFix2CRun(t, r, s, "TDIGEST.TRIMMED_MEAN", k, "0", "1"); auditFix2CStr(v) != "35" {
		t.Fatalf("TRIMMED_MEAN 0 1 = %q", auditFix2CStr(v))
	}
	if v := auditFix2CRun(t, r, s, "TDIGEST.TRIMMED_MEAN", k, "0.5", "0.1"); auditFix2CErr(v) == "" {
		t.Fatalf("inverted cut must fail")
	}

	info := auditFix2CRun(t, r, s, "TDIGEST.INFO", k)
	if auditFix2CField(t, info, "Compression").Int != 200 || auditFix2CField(t, info, "Observations").Int != 6 {
		t.Fatalf("INFO = %v", auditFix2CStrings(t, info))
	}

	k2 := k + "-b"
	auditFix2CRun(t, r, s, "TDIGEST.CREATE", k2)
	auditFix2CRun(t, r, s, "TDIGEST.ADD", k2, "100")
	dst := k + "-dst"
	if v := auditFix2CRun(t, r, s, "TDIGEST.MERGE", dst, "2", k, k2); v.Str != "OK" {
		t.Fatalf("MERGE = %q", auditFix2CErr(v))
	}
	mi := auditFix2CRun(t, r, s, "TDIGEST.INFO", dst)
	if auditFix2CField(t, mi, "Observations").Int != 7 || auditFix2CField(t, mi, "Compression").Int != 200 {
		t.Fatalf("merged INFO = %v", auditFix2CStrings(t, mi))
	}
	if v := auditFix2CRun(t, r, s, "TDIGEST.MAX", dst); auditFix2CStr(v) != "100" {
		t.Fatalf("merged MAX = %q", auditFix2CStr(v))
	}
	auditFix2CRun(t, r, s, "TDIGEST.MERGE", dst, "1", k2)
	if got := auditFix2CField(t, auditFix2CRun(t, r, s, "TDIGEST.INFO", dst), "Observations").Int; got != 8 {
		t.Fatalf("MERGE without OVERRIDE must keep the destination's data, observations = %d", got)
	}
	auditFix2CRun(t, r, s, "TDIGEST.MERGE", dst, "1", k2, "OVERRIDE")
	if got := auditFix2CField(t, auditFix2CRun(t, r, s, "TDIGEST.INFO", dst), "Observations").Int; got != 1 {
		t.Fatalf("MERGE OVERRIDE observations = %d, want 1", got)
	}
	if v := auditFix2CRun(t, r, s, "TDIGEST.MERGE", dst, "1", "missing-"+k); auditFix2CErr(v) != errTDigestNoKey.Error() {
		t.Fatalf("MERGE from a missing key = %q", auditFix2CErr(v))
	}

	if v := auditFix2CRun(t, r, s, "TDIGEST.RESET", k); v.Str != "OK" {
		t.Fatalf("RESET = %q", auditFix2CErr(v))
	}
	if v := auditFix2CRun(t, r, s, "TDIGEST.RANK", k, "1"); v.Array[0].Int != -2 {
		t.Fatalf("RANK on empty = %d, want -2", v.Array[0].Int)
	}
}

func TestAuditFix2CTDigestAccuracy(t *testing.T) {
	td := newTDigestSketch(100)
	for i := 1; i <= 100000; i++ {
		td.addLocked(float64(i), 1)
	}
	for _, q := range []float64{0.01, 0.1, 0.5, 0.9, 0.99} {
		got := td.quantileLocked(q)
		want := q * 100000
		if d := got - want; d > 1000 || d < -1000 {
			t.Fatalf("quantile(%v) = %v, want ~%v", q, got, want)
		}
	}
	if c := td.cdfLocked(50000); c < 0.49 || c > 0.51 {
		t.Fatalf("cdf(50000) = %v", c)
	}
	if len(td.means) > td.capacity() {
		t.Fatalf("digest not compressed: %d centroids", len(td.means))
	}
	if td.min != 1 || td.max != 100000 {
		t.Fatalf("min/max must be exact: %v %v", td.min, td.max)
	}
}

func TestAuditFix2CTDigestConcurrent(t *testing.T) {
	r := auditFix2CRouter()
	s := store.NewStore()
	k := fmt.Sprintf("tdc-%d", time.Now().UnixNano())
	auditFix2CRun(t, r, s, "TDIGEST.CREATE", k)
	var wg sync.WaitGroup
	for g := 0; g < 4; g++ {
		wg.Add(1)
		go func(g int) {
			defer wg.Done()
			for i := 0; i < 200; i++ {
				ctx := NewContext("TDIGEST.ADD", bytesArgs(k, fmt.Sprint(i)), s, resp.NewWriter(&bytes.Buffer{}))
				_ = r.Execute(ctx)
				ctx = NewContext("TDIGEST.QUANTILE", bytesArgs(k, "0.5"), s, resp.NewWriter(&bytes.Buffer{}))
				_ = r.Execute(ctx)
				if i%50 == 0 {
					ctx = NewContext("TDIGEST.MERGE", bytesArgs(k+"-m", "1", k), s, resp.NewWriter(&bytes.Buffer{}))
					_ = r.Execute(ctx)
				}
			}
		}(g)
	}
	wg.Wait()
	if got := auditFix2CField(t, auditFix2CRun(t, r, s, "TDIGEST.INFO", k), "Observations").Int; got != 800 {
		t.Fatalf("observations = %d, want 800", got)
	}
}

func TestAuditFix2CAuditDispatch(t *testing.T) {
	r := auditFix2CRouter()
	r.Register(&CommandDef{Name: "AUTH", Handler: func(ctx *Context) error { return ctx.WriteOK() }})
	s := store.NewStore()
	defer auditDispatch.Store(false)
	key := fmt.Sprintf("auditkey-%d", time.Now().UnixNano())

	auditDispatch.Store(false)
	auditFix2CRun(t, r, s, "SET", key, "v0")
	if n := len(store.GlobalAuditLog.GetByKey(key, 10)); n != 0 {
		t.Fatalf("disabled audit recorded %d entries", n)
	}

	auditFix2CRun(t, r, s, "AUDIT.ENABLE")
	auditFix2CRun(t, r, s, "SET", key, strings.Repeat("v", 1000))
	auditFix2CRun(t, r, s, "GET", key)
	auditFix2CRun(t, r, s, "AUTH", "secret-password")
	auditFix2CRun(t, r, s, "AUDIT.DISABLE")
	auditFix2CRun(t, r, s, "DEL", key)

	entries := store.GlobalAuditLog.GetByKey(key, 10)
	if len(entries) != 2 {
		t.Fatalf("want SET and GET recorded, got %d entries", len(entries))
	}
	if entries[0].Command != "GET" || entries[1].Command != "SET" {
		t.Fatalf("recorded commands = %s, %s", entries[0].Command, entries[1].Command)
	}
	if len(entries[1].Args) != 1 || len(entries[1].Args[0]) > auditMaxArgLen+3 {
		t.Fatalf("SET args must be recorded and truncated: %d args", len(entries[1].Args))
	}
	for _, e := range store.GlobalAuditLog.GetByCommand("AUTH", 10) {
		if e.Key != "" || len(e.Args) != 0 {
			t.Fatalf("AUTH arguments must be redacted, got key %q args %v", e.Key, e.Args)
		}
	}
	if !entries[0].Success {
		t.Fatalf("GET must be recorded as a success")
	}
	missing := fmt.Sprintf("auditmissing-%d", time.Now().UnixNano())
	auditFix2CRun(t, r, s, "AUDIT.ENABLE")
	auditFix2CRun(t, r, s, "SET", missing, "abc")
	auditFix2CRun(t, r, s, "INCR", missing)
	auditFix2CRun(t, r, s, "AUDIT.DISABLE")
	failed := store.GlobalAuditLog.GetByKey(missing, 10)
	if len(failed) != 2 || failed[0].Command != "INCR" || failed[0].Success {
		t.Fatalf("an error reply must be recorded as a failure: %+v", failed)
	}
	stats := auditFix2CRun(t, r, s, "AUDIT.STATS")
	if auditFix2CStr(auditFix2CField(t, stats, "enabled")) != "false" {
		t.Fatalf("AUDIT.STATS enabled must reflect AUDIT.DISABLE")
	}
}

func TestAuditFix2CAuditDisabledNoAlloc(t *testing.T) {
	r := NewRouter()
	r.Register(&CommandDef{Name: "NOPX", Handler: func(ctx *Context) error { return nil }})
	defer auditDispatch.Store(false)
	ctx := NewContext("NOPX", nil, store.NewStore(), resp.NewWriter(&bytes.Buffer{}))
	ctx.Transaction = nil

	auditDispatch.Store(false)
	before := len(store.GlobalAuditLog.GetByCommand("NOPX", 1000))
	disabled := testing.AllocsPerRun(200, func() {
		_ = r.Execute(ctx)
	})
	if got := len(store.GlobalAuditLog.GetByCommand("NOPX", 1000)); got != before {
		t.Fatalf("disabled audit recorded %d entries", got-before)
	}
	auditDispatch.Store(true)
	enabled := testing.AllocsPerRun(200, func() {
		_ = r.Execute(ctx)
	})
	if disabled >= enabled {
		t.Fatalf("audit-disabled dispatch allocates %v per command, enabled %v: the disabled path must add nothing", disabled, enabled)
	}
}

func TestAuditFix2CDatasetSplit(t *testing.T) {
	r := auditFix2CRouter()
	s := store.NewStore()
	name := fmt.Sprintf("ds-%d", time.Now().UnixNano())
	auditFix2CRun(t, r, s, "DATASET.CREATE", name)
	for i := 0; i < 10; i++ {
		auditFix2CRun(t, r, s, "DATASET.ADD", name, fmt.Sprint(i))
	}
	v := auditFix2CRun(t, r, s, "DATASET.SPLIT", name, "0.7")
	train := auditFix2CStrings(t, auditFix2CField(t, v, "train"))
	test := auditFix2CStrings(t, auditFix2CField(t, v, "test"))
	if strings.Join(train, "") != "0123456" || strings.Join(test, "") != "789" {
		t.Fatalf("split = %v / %v", train, test)
	}
	if v := auditFix2CRun(t, r, s, "DATASET.SPLIT", name, "1.5"); auditFix2CErr(v) == "" {
		t.Fatalf("ratio > 1 must be rejected")
	}
	a := auditFix2CRun(t, r, s, "DATASET.SPLIT", name, "0.5", "SEED", "7")
	b := auditFix2CRun(t, r, s, "DATASET.SPLIT", name, "0.5", "SEED", "7")
	ta := auditFix2CStrings(t, auditFix2CField(t, a, "train"))
	tb := auditFix2CStrings(t, auditFix2CField(t, b, "train"))
	tt := auditFix2CStrings(t, auditFix2CField(t, a, "test"))
	if strings.Join(ta, ",") != strings.Join(tb, ",") || len(ta) != 5 || len(tt) != 5 {
		t.Fatalf("seeded split must be deterministic and sized: %v %v", ta, tb)
	}
	seen := map[string]bool{}
	for _, x := range append(ta, tt...) {
		seen[x] = true
	}
	if len(seen) != 10 {
		t.Fatalf("seeded split lost or duplicated items: %v", seen)
	}
}

func TestAuditFix2CChainValidate(t *testing.T) {
	r := auditFix2CRouter()
	s := store.NewStore()
	name := fmt.Sprintf("chain-%d", time.Now().UnixNano())
	auditFix2CRun(t, r, s, "CHAIN.CREATE", name)
	for i := 0; i < 5; i++ {
		auditFix2CRun(t, r, s, "CHAIN.ADD", name, fmt.Sprintf("tx%d", i))
	}
	if v := auditFix2CRun(t, r, s, "CHAIN.VALIDATE", name); v.Int != 1 {
		t.Fatalf("intact chain VALIDATE = %d", v.Int)
	}

	chainsMu.Lock()
	blocks := chains[name].Blocks
	orig := *blocks[0]
	blocks[0].Data = "forged"
	chainsMu.Unlock()
	if v := auditFix2CRun(t, r, s, "CHAIN.VALIDATE", name); v.Int != 0 {
		t.Fatalf("tampered genesis data must fail validation")
	}

	chainsMu.Lock()
	*blocks[0] = orig
	blocks[2].Timestamp++
	chainsMu.Unlock()
	if v := auditFix2CRun(t, r, s, "CHAIN.VALIDATE", name); v.Int != 0 {
		t.Fatalf("tampered timestamp must fail validation")
	}

	chainsMu.Lock()
	blocks[2].Timestamp--
	forged := *blocks[3]
	forged.Data = "forged"
	forged.Hash = computeBlockHash(&forged)
	*blocks[3] = forged
	chainsMu.Unlock()
	if v := auditFix2CRun(t, r, s, "CHAIN.VALIDATE", name); v.Int != 0 {
		t.Fatalf("a rehashed block must break the next block's link")
	}
}
