package server

import (
	"fmt"
	"testing"
	"time"

	"github.com/cachestorm/cachestorm/internal/resp"
)

// vsetState is what must survive a restart, a rewrite or a sync: every
// element's stored vector and attributes, and the TRUTH search scores.
func vsetState(c *replClient, key string, n int) string {
	var out string
	for i := 0; i < n; i++ {
		e := fmt.Sprintf("e%d", i)
		raw := c.do("VEMB", key, e, "RAW")
		attr := c.do("VGETATTR", key, e)
		out += fmt.Sprintf("%s:%v:%q|", e, flat(raw), attr.Bulk)
	}
	out += fmt.Sprint(flat(c.do("VSIM", key, "VALUES", "4", "1", "0", "-1", "0.5", "TRUTH", "WITHSCORES", "COUNT", "20")))
	// max-level depends on the levels a replay draws, and vset-uid numbers
	// the sets of a process.
	info := flat(c.do("VINFO", key))
	for i := 0; i+1 < len(info); i += 2 {
		if info[i] != "+max-level0" && info[i] != "+vset-uid0" {
			out += info[i] + info[i+1]
		}
	}
	return out
}

func flat(v *resp.Value) []string {
	if v == nil {
		return nil
	}
	if v.Array == nil {
		return []string{fmt.Sprintf("%c%s%s%d", v.Type, v.Str, v.Bulk, v.Int)}
	}
	var out []string
	for _, it := range v.Array {
		out = append(out, flat(it)...)
	}
	return out
}

func fillVectorSet(c *replClient, key string, n int) {
	for i := 0; i < n; i++ {
		args := []string{"VADD", key, "VALUES", "4",
			fmt.Sprint(i % 5), fmt.Sprint(i%3 - 1), fmt.Sprint(float64(i) / 7), "1", fmt.Sprintf("e%d", i)}
		if i%3 == 0 {
			args = append(args, "SETATTR", fmt.Sprintf(`{"i":%d}`, i))
		}
		if got := c.do(args...); got.Int != 1 {
			c.t.Fatalf("%v = %+v", args, got)
		}
	}
	c.do("VREM", key, "e4")
	c.do("VSETATTR", key, "e5", `{"set":true}`)
	c.do("VADD", key, "VALUES", "4", "9", "9", "9", "9", "e7", "SETATTR", "")
}

func TestVectorSetSurvivesAOFRestartAndRewrite(t *testing.T) {
	dir := t.TempDir()
	cfg := aofRewriteConfig(t, dir, nil)
	s := startReplServerCfg(t, cfg)
	c := dialRepl(t, s)
	fillVectorSet(c, "v", 40)
	want := vsetState(c, "v", 40)

	stopReplServer(s)
	s = startReplServerCfg(t, cfg)
	c = dialRepl(t, s)
	if got := vsetState(c, "v", 40); got != want {
		t.Fatalf("after AOF replay\n got %s\nwant %s", got, want)
	}
	c.do("BGREWRITEAOF")
	waitAOFRewrites(t, s, 1)
	stopReplServer(s)
	s = startReplServerCfg(t, cfg)
	c = dialRepl(t, s)
	if got := vsetState(c, "v", 40); got != want {
		t.Fatalf("after rewrite\n got %s\nwant %s", got, want)
	}
}

func TestVectorSetSurvivesSnapshotRestart(t *testing.T) {
	dir := t.TempDir()
	cfg := snapshotConfig(t, dir)
	s := startReplServerCfg(t, cfg)
	c := dialRepl(t, s)
	fillVectorSet(c, "v", 30)
	want := vsetState(c, "v", 30)
	if got := c.do("SAVE"); got.Str != "OK" {
		t.Fatalf("SAVE = %+v", got)
	}
	stopReplServer(s)
	time.Sleep(350 * time.Millisecond)
	s = startReplServerCfg(t, cfg)
	c = dialRepl(t, s)
	if got := vsetState(c, "v", 30); got != want {
		t.Fatalf("after snapshot restart\n got %s\nwant %s", got, want)
	}
}

func TestVectorSetReplicates(t *testing.T) {
	master := startReplServer(t, nil)
	mc := dialRepl(t, master)
	fillVectorSet(mc, "synced", 25)
	replica := startReplServer(t, nil)
	rc := replicate(t, replica, master)
	waitSynced(t, replica, master)
	if got, want := vsetState(rc, "synced", 25), vsetState(mc, "synced", 25); got != want {
		t.Fatalf("after full sync\n got %s\nwant %s", got, want)
	}
	fillVectorSet(mc, "live", 25)
	waitSynced(t, replica, master)
	// A live replica builds its own graph, so only what does not depend on
	// it is compared.
	for i := 0; i < 25; i++ {
		e := fmt.Sprintf("e%d", i)
		if got, want := flat(rc.do("VEMB", "live", e, "RAW")), flat(mc.do("VEMB", "live", e, "RAW")); fmt.Sprint(got) != fmt.Sprint(want) {
			t.Fatalf("replica VEMB %s = %v, master %v", e, got, want)
		}
		if got, want := rc.do("VGETATTR", "live", e), mc.do("VGETATTR", "live", e); string(got.Bulk) != string(want.Bulk) || got.IsNull != want.IsNull {
			t.Fatalf("replica VGETATTR %s = %+v, master %+v", e, got, want)
		}
	}
}

func TestVectorSetKeyspace(t *testing.T) {
	s := startReplServer(t, nil)
	c := dialRepl(t, s)
	fillVectorSet(c, "v", 5)
	if got := c.do("TYPE", "v"); got.Str != "vectorset" {
		t.Fatalf("TYPE = %+v", got)
	}
	if got := c.do("COPY", "v", "w"); got.Err != "ERR not supported for this module key" {
		t.Fatalf("COPY = %+v", got)
	}
	c.do("SET", "w", "x")
	if got := c.do("COPY", "v", "w"); got.Int != 0 {
		t.Fatalf("COPY onto an existing key without REPLACE = %+v", got)
	}
	if got := c.do("RENAME", "v", "u"); got.Str != "OK" {
		t.Fatalf("RENAME = %+v", got)
	}
	for _, e := range []string{"e0", "e1", "e2", "e3", "e7"} {
		c.do("VREM", "u", e)
	}
	if got := c.do("EXISTS", "u"); got.Int != 0 {
		t.Fatal("removing the last element must delete the key")
	}
	c.do("VADD", "r", "VALUES", "2", "1", "0", "a", "SETATTR", `{"x":1}`)
	c.do("VADD", "r", "VALUES", "2", "0", "1", "b")
	c.do("HELLO", "3")
	if got := c.do("VSIM", "r", "VALUES", "2", "1", "1", "WITHSCORES", "WITHATTRIBS"); got.Type != resp.TypeMap || len(got.Array) != 4 || len(got.Array[1].Array) != 2 {
		t.Fatalf("RESP3 VSIM WITHSCORES WITHATTRIBS = %+v", got)
	}
	if got := c.do("VISMEMBER", "r", "a"); got.Type != resp.TypeBoolean || got.Int != 1 {
		t.Fatalf("RESP3 VISMEMBER = %+v", got)
	}
}
