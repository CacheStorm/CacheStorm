package server

import (
	"reflect"
	"testing"
)

// A script that writes and then fails keeps its writes, and Redis
// propagates them. Run inside MULTI, EXEC left a command that answered with
// an error out of what it recorded, script effects and all, so the replicas
// and the AOF never had the script's RPUSH.
func TestScriptThatFailsInsideATransactionKeepsItsWrites(t *testing.T) {
	cfg := aofRewriteConfig(t, t.TempDir(), nil)
	master := startReplServerCfg(t, cfg)
	replica := startReplServer(t, nil)
	replicate(t, replica, master)
	c := dialRepl(t, master)
	c.do("SET", "s", "str")
	c.do("MULTI")
	c.do("EVAL", "redis.call('RPUSH', KEYS[1], 'v') return redis.call('LPOP', KEYS[2])", "2", "l", "s")
	c.do("RPUSH", "l", "w")
	if got := str(c.do("EXEC")); got == "" {
		t.Fatal("EXEC gave nothing")
	}
	if got := str(c.do("LRANGE", "l", "0", "-1")); got != "[v w]" {
		t.Fatalf("master l = %s", got)
	}
	want := dumpAll(t, c)
	waitSynced(t, replica, master)
	if got := dumpAll(t, dialRepl(t, replica)); !reflect.DeepEqual(got, want) {
		t.Fatalf("replica %v, master %v", got, want)
	}
	restarted := checkAOFRestart(t, master, cfg)
	if got := dumpAll(t, dialRepl(t, restarted)); !reflect.DeepEqual(got, want) {
		t.Fatalf("after an AOF load %v, before %v", got, want)
	}
}
