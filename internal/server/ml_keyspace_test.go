package server

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// FEATURE, EMBEDDING, TENSOR, DATASET, MLXPERIMENT and HYPERPARAM are keys:
// their objects survive a restart through an AOF holding only Redis
// commands.
func TestMLNamespacesAreKeys(t *testing.T) {
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
	ok("FEATURE.SET", "user:1", "age", "30")
	ok("FEATURE.INCR", "user:1", "score", "0.25")
	ok("FEATURE.SET", "user:1", "tmp", "1")
	ok("FEATURE.DEL", "user:1", "tmp")
	ok("EMBEDDING.CREATE", "emb", "0.1", "0.2", "0.3")
	ok("TENSOR.CREATE", "t1", "2", "2", "|", "1", "2", "3", "4")
	ok("TENSOR.CREATE", "t2", "4", "|", "1", "1", "1", "1")
	ok("TENSOR.ADD", "t1", "t2")
	ok("TENSOR.RESHAPE", "t1", "4")
	ok("DATASET.CREATE", "ds")
	for _, item := range []string{"a", "b c", "d"} {
		ok("DATASET.ADD", "ds", item)
	}
	ok("MLXPERIMENT.CREATE", "exp")
	ok("MLXPERIMENT.LOG", "exp", "loss", "0.5")
	ok("MLXPERIMENT.LOG", "exp", "acc", "0.9")
	ok("HYPERPARAM.SET", "hp", "lr", "0.001")
	ok("SAMPLE.CREATE", "smp", "2")
	if r := c.do("SAMPLE.ADD", "smp", "1", "2", "3", "4"); r.Int != 4 {
		t.Fatalf("SAMPLE.ADD: %+v", r)
	}
	ok("HISTOGRAM.CREATE", "hist", "0", "10", "5")
	ok("HISTOGRAM.ADD", "hist", "1", "2", "7", "-3", "12")
	before := renderReply(c.do("SAMPLE.GET", "smp"), ",")

	ok("SET", "plain", "v")
	for _, args := range [][]string{{"FEATURE.SET", "plain", "a", "1"}, {"DATASET.ADD", "emb", "x"}, {"TENSOR.ADD", "ds", "t2"},
		{"MLXPERIMENT.LOG", "hp", "m", "1"}, {"HYPERPARAM.SET", "exp", "p", "v"}, {"EMBEDDING.GET", "t1"},
		{"SAMPLE.ADD", "hist", "1"}, {"HISTOGRAM.ADD", "smp", "1"}} {
		if r := c.do(args...); r.Type != '-' || !strings.HasPrefix(r.Err, "WRONGTYPE") {
			t.Fatalf("%v: %v", args, r)
		}
	}
	for _, args := range [][]string{{"FEATURE.SET", "f", "a", "nan"}, {"FEATURE.SET", "f", "__feature", "1"},
		{"EMBEDDING.CREATE", "e2", "x"}, {"TENSOR.CREATE", "t3", "3", "|", "1", "2"}, {"TENSOR.RESHAPE", "t1", "3"},
		{"DATASET.ADD", "nods", "x"}, {"MLXPERIMENT.LOG", "noexp", "m", "1"},
		{"SAMPLE.ADD", "smp", "x"}, {"HISTOGRAM.CREATE", "h2", "5", "1"}, {"HISTOGRAM.ADD", "hist", "nan"}} {
		if r := c.do(args...); r.Type != '-' {
			t.Fatalf("%v accepted: %v", args, r)
		}
	}
	stopReplServer(s)

	data, err := os.ReadFile(filepath.Join(cfg.Persistence.DataDir, "appendonly.aof"))
	if err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"FEATURE.", "EMBEDDING.", "TENSOR.", "DATASET.", "MLXPERIMENT.", "HYPERPARAM.", "SAMPLE.", "HISTOGRAM."} {
		if strings.Contains(string(data), name) {
			t.Fatalf("the AOF records %s commands", name)
		}
	}

	s = startToggleServer(t, cfg)
	defer stopReplServer(s)
	c = dialRepl(t, s)
	expect := func(want string, args ...string) {
		t.Helper()
		if r := renderReply(c.do(args...), ","); r != want {
			t.Fatalf("%v after restart = %q, want %q", args, r, want)
		}
	}
	expect("[age 30.000000],[score 0.250000]", "FEATURE.VECTOR", "user:1")
	expect("0.100000,0.200000,0.300000", "EMBEDDING.GET", "emb")
	expect("shape,[4],data,[2.000000 3.000000 4.000000 5.000000]", "TENSOR.GET", "t1")
	expect("a,b c,d", "DATASET.GET", "ds")
	expect("train,[a],test,[b c d]", "DATASET.SPLIT", "ds", "0.4")
	expect("acc,0.900000,loss,0.500000", "MLXPERIMENT.METRICS", "exp")
	expect("0.001", "HYPERPARAM.GET", "hp", "lr")
	expect(before, "SAMPLE.GET", "smp")
	expect("size,2,total_count,4", "SAMPLE.INFO", "smp")
	expect("[-5,0),1,[0,5),2,[5,10),1,[10,15),1", "HISTOGRAM.GET", "hist")
	expect("3.8", "HISTOGRAM.MEAN", "hist")
}
