package server

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/cachestorm/cachestorm/internal/resp"
)

// The namespaces of more_commands.go are keys: their objects survive a
// restart through an AOF holding only Redis commands.
func TestMoreNamespacesAreKeys(t *testing.T) {
	cfg := aofTxConfig(t)
	s := startToggleServer(t, cfg)
	c := dialRepl(t, s)
	ok := func(args ...string) string {
		t.Helper()
		v := c.do(args...)
		if v.Type == '-' {
			t.Fatalf("%v: %v", args, v.Err)
		}
		return string(v.Bulk)
	}
	ok("SLIDING.CREATE", "sl", "2", "3600000")
	ok("SLIDING.CHECK", "sl")
	ok("SLIDING.CHECK", "sl")
	ok("BUCKETX.CREATE", "bx", "10", "1", "3600000")
	ok("BUCKETX.TAKE", "bx", "4")
	ok("IDEMPOTENCY.SET", "req-1", "200 OK", "3600000")
	ok("EXPERIMENT.CREATE", "exp", "a", "b")
	variant := ok("EXPERIMENT.ASSIGN", "exp", "ada")
	ok("EXPERIMENT.TRACK", "exp", "ada", "click")
	ok("ROLLOUT.CREATE", "ro", "100")
	ok("ALERT.CREATE", "disk", "full")
	ok("ALERT.TRIGGER", "disk")
	ok("COUNTERX.CREATE", "cx", "5")
	ok("COUNTERX.INCR", "cx", "2")
	ok("GAUGE.SET", "temp", "21.5")
	ok("TRACE.START", "tr")
	ok("TRACE.SPAN", "tr", "db", "s1")
	ok("TRACE.END", "tr")
	ok("LOGX.WRITE", "app", "info", "started")
	ok("LOGX.WRITE", "app", "error", "disk failed")
	apikey := ok("APIKEY.CREATE", "svc")
	ok("APIKEY.VALIDATE", apikey)
	ok("QUOTAX.CREATE", "qx", "10", "0")
	ok("QUOTAX.CHECK", "qx", "3")
	ok("METER.CREATE", "m", "bytes")
	ok("METER.RECORD", "m", "100")
	ok("METER.RECORD", "m", "50")
	ok("TENANT.CREATE", "t1", "Acme")
	ok("TENANT.CONFIG", "t1", "plan", "pro")
	ok("LEASE.CREATE", "le", "node-a", "3600000")
	ok("HEAP.PUSH", "hp", "5")
	ok("HEAP.PUSH", "hp", "2")
	ok("HEAP.PUSH", "hp", "2")
	ok("BLOOMX.CREATE", "bl", "1000", "3")
	ok("BLOOMX.ADD", "bl", "apple")
	ok("SKETCH.CREATE", "sk", "100", "4")
	ok("SKETCH.UPDATE", "sk", "apple", "3")
	ok("RINGBUFFER.CREATE", "rb", "2")
	for _, v := range []string{"x", "y", "z"} {
		ok("RINGBUFFER.WRITE", "rb", v)
	}
	ok("WINDOW.CREATE", "w", "2")
	for _, v := range []string{"1", "2", "4"} {
		ok("WINDOW.ADD", "w", v)
	}
	ok("FREQ.CREATE", "fq")
	ok("FREQ.ADD", "fq", "go", "3")
	ok("FREQ.ADD", "fq", "redis")
	ok("PARTITION.CREATE", "pt", "4")
	part := c.do("PARTITION.ADD", "pt", "user:1", "v1").Int
	ok("SET", "plain", "v")
	for _, args := range [][]string{
		{"SLIDING.CHECK", "plain"}, {"BUCKETX.TAKE", "plain", "1"}, {"GAUGE.INCR", "plain"}, {"HEAP.PUSH", "plain", "1"},
		{"BLOOMX.ADD", "plain", "x"}, {"FREQ.ADD", "plain", "x"}, {"LOGX.WRITE", "plain", "i", "m"}, {"RINGBUFFER.WRITE", "w", "x"},
	} {
		if got := c.do(args...); got.Type != '-' || !strings.HasPrefix(got.Err, "WRONGTYPE") {
			t.Fatalf("%v: %v", args, got)
		}
	}
	for _, args := range [][]string{
		{"GAUGE.SET", "temp", "nan"}, {"ROLLOUT.CREATE", "r2", "101"}, {"HEAP.PUSH", "hp", "x"}, {"LEASE.CREATE", "le", "node-b", "1000"},
		{"IDEMPOTENCY.SET", "req-1", "dup", "1000"}, {"QUOTAX.CHECK", "qx", "-1"},
	} {
		if got := c.do(args...); got.Type != '-' {
			t.Fatalf("%v accepted: %v", args, got)
		}
	}
	stopReplServer(s)

	data, err := os.ReadFile(filepath.Join(cfg.Persistence.DataDir, "appendonly.aof"))
	if err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"SLIDING.", "BUCKETX.", "IDEMPOTENCY.", "EXPERIMENT.", "ROLLOUT.", "ALERT.", "COUNTERX.", "GAUGE.",
		"TRACE.", "LOGX.", "APIKEY.", "QUOTAX.", "METER.", "TENANT.", "LEASE.", "HEAP.", "BLOOMX.", "SKETCH.", "RINGBUFFER.",
		"WINDOW.", "FREQ.", "PARTITION."} {
		if strings.Contains(string(data), name) {
			t.Fatalf("the AOF records %s commands", name)
		}
	}

	s = startToggleServer(t, cfg)
	defer stopReplServer(s)
	c = dialRepl(t, s)
	expect := func(want string, args ...string) {
		t.Helper()
		if got := renderReply(c.do(args...), ","); got != want {
			t.Fatalf("%v after restart = %q, want %q", args, got, want)
		}
	}
	if got := c.do("SLIDING.CHECK", "sl"); got.Array[0].Int != 0 {
		t.Fatalf("SLIDING.CHECK past the limit after restart: %+v", got)
	}
	expect("1,0", "BUCKETX.TAKE", "bx", "6")
	expect("200 OK", "IDEMPOTENCY.GET", "req-1")
	expect(variant, "EXPERIMENT.ASSIGN", "exp", "ada")
	expect("1", "ROLLOUT.CHECK", "ro", "anyone")
	if got := c.do("ALERT.HISTORY", "disk"); string(got.Array[7].Bulk) != "firing" || len(got.Array[11].Array) != 2 {
		t.Fatalf("ALERT.HISTORY after restart: %+v", got)
	}
	expect("7", "COUNTERX.GET", "cx")
	expect("21.500000", "GAUGE.GET", "temp")
	expect("id,tr,status,completed,spans,1", "TRACE.GET", "tr")
	if got := c.do("LOGX.SEARCH", "app", "disk"); len(got.Array) != 1 {
		t.Fatalf("LOGX.SEARCH after restart: %+v", got)
	}
	expect("1", "APIKEY.USAGE", apikey)
	expect("used,3,limit,10,remaining,7", "QUOTAX.USAGE", "qx")
	expect("name,m,unit,bytes,total,150,count,2", "METER.GET", "m")
	if got := c.do("TENANT.GET", "t1"); string(got.Array[3].Bulk) != "Acme" {
		t.Fatalf("TENANT.GET after restart: %+v", got)
	}
	if got := c.do("LEASE.GET", "le"); string(got.Array[3].Bulk) != "node-a" {
		t.Fatalf("LEASE.GET after restart: %+v", got)
	}
	expect("2", "HEAP.POP", "hp")
	expect("2", "HEAP.PEEK", "hp")
	expect("1", "BLOOMX.CHECK", "bl", "apple")
	expect("3", "SKETCH.QUERY", "sk", "apple")
	expect("y,z", "RINGBUFFER.READ", "rb")
	expect("6.000000", "WINDOW.AGGREGATE", "w", "sum")
	expect("[item go count 3],[item redis count 1]", "FREQ.TOP", "fq")
	expect("v1", "PARTITION.GET", "pt", itoa64(part))
}

func itoa64(n int64) string { return renderReply(&resp.Value{Type: ':', Int: n}, "") }
