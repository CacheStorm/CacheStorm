package server

import (
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/cachestorm/cachestorm/internal/resp"
)

func msgField(t *testing.T, v *resp.Value, name string) string {
	t.Helper()
	for i := 0; i+1 < len(v.Array); i += 2 {
		if string(v.Array[i].Bulk) == name {
			if v.Array[i+1].Type == ':' {
				return strconv.FormatInt(v.Array[i+1].Int, 10)
			}
			return string(v.Array[i+1].Bulk)
		}
	}
	t.Fatalf("no %s in %+v", name, v)
	return ""
}

// A MSGQUEUE is a stream read through a consumer group: its messages, the
// ones being processed with their visibility timeouts, and its dead
// letters survive a restart through the AOF, which holds only stream
// commands. It was a process-wide map, lost on restart.
func TestMessageQueuesAreStreams(t *testing.T) {
	cfg := aofTxConfig(t)
	s := startToggleServer(t, cfg)
	c := dialRepl(t, s)
	do := func(args ...string) *resp.Value {
		t.Helper()
		v := c.do(args...)
		if v.Type == '-' {
			t.Fatalf("%v: %v", args, v.Err)
		}
		return v
	}
	do("MSGQUEUE.CREATE", "jobs", "2")
	ids := map[string]string{}
	for _, body := range []string{"a", "b", "c", "d"} {
		ids[body] = string(do("MSGQUEUE.PUBLISH", "jobs", body).Bulk)
	}
	if got := msgField(t, do("MSGQUEUE.CONSUME", "jobs", "60000"), "body"); got != "a" {
		t.Fatalf("first CONSUME: %q", got)
	}
	if got := msgField(t, do("MSGQUEUE.CONSUME", "jobs", "60000"), "body"); got != "b" {
		t.Fatalf("second CONSUME: %q", got)
	}
	if got := do("MSGQUEUE.NACK", "jobs", ids["b"]); got.Int != 1 {
		t.Fatalf("NACK b: %v", got)
	}
	m := do("MSGQUEUE.CONSUME", "jobs", "60000")
	if msgField(t, m, "body") != "b" || msgField(t, m, "retries") != "1" {
		t.Fatalf("redelivery of b: %+v", m)
	}
	if got := do("MSGQUEUE.ACK", "jobs", ids["a"]); got.Int != 1 {
		t.Fatalf("ACK a: %v", got)
	}
	if got := msgField(t, do("MSGQUEUE.CONSUME", "jobs", "50"), "body"); got != "c" {
		t.Fatalf("CONSUME c: %q", got)
	}
	do("MSGQUEUE.CREATE", "dl", "1")
	x := string(do("MSGQUEUE.PUBLISH", "dl", "x").Bulk)
	do("MSGQUEUE.CONSUME", "dl", "60000")
	if got := do("MSGQUEUE.NACK", "dl", x); got.Int != 2 {
		t.Fatalf("NACK past the retries: %v", got)
	}
	if got := c.do("TYPE", "jobs"); got.Str != "stream" {
		t.Fatalf("TYPE jobs: %v", got)
	}
	do("SET", "plain", "v")
	for _, args := range [][]string{{"MSGQUEUE.PUBLISH", "plain", "x"}, {"MSGQUEUE.CREATE", "plain"}, {"MSGQUEUE.DELETE", "plain"}} {
		if got := c.do(args...); got.Type != '-' || !strings.HasPrefix(got.Err, "WRONGTYPE") {
			t.Fatalf("%v: %v", args, got)
		}
	}
	if got := c.do("MSGQUEUE.PUBLISH", "missing", "x"); got.Type != '-' {
		t.Fatalf("PUBLISH to a missing queue: %v", got)
	}
	time.Sleep(80 * time.Millisecond)
	stopReplServer(s)

	data, err := os.ReadFile(filepath.Join(cfg.Persistence.DataDir, "appendonly.aof"))
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(data), "MSGQUEUE.") {
		t.Fatalf("the AOF records MSGQUEUE commands:\n%q", data)
	}

	s = startToggleServer(t, cfg)
	defer stopReplServer(s)
	c = dialRepl(t, s)
	stats := do("MSGQUEUE.STATS", "jobs")
	for field, want := range map[string]string{"pending": "2", "processing": "1", "dead_letters": "0", "max_retries": "2"} {
		if got := msgField(t, stats, field); got != want {
			t.Fatalf("STATS %s after restart = %s, want %s: %+v", field, got, want, stats)
		}
	}
	m = do("MSGQUEUE.CONSUME", "jobs", "60000")
	if msgField(t, m, "body") != "c" || msgField(t, m, "retries") != "1" {
		t.Fatalf("timed-out c after restart: %+v", m)
	}
	if got := msgField(t, do("MSGQUEUE.CONSUME", "jobs", "60000"), "body"); got != "d" {
		t.Fatalf("CONSUME d: %q", got)
	}
	if got := do("MSGQUEUE.ACK", "jobs", ids["b"]); got.Int != 1 {
		t.Fatalf("ACK b after restart: %v", got)
	}
	if got := c.do("MSGQUEUE.CONSUME", "jobs"); !got.IsNull {
		t.Fatalf("CONSUME of an empty queue: %+v", got)
	}
	dead := do("MSGQUEUE.DEADLETTER", "dl")
	if len(dead.Array) != 1 || msgField(t, dead.Array[0], "body") != "x" {
		t.Fatalf("DEADLETTER after restart: %+v", dead)
	}
	if got := do("MSGQUEUE.REQUEUE", "dl", x); got.Int != 1 {
		t.Fatalf("REQUEUE: %v", got)
	}
	if m := do("MSGQUEUE.CONSUME", "dl"); msgField(t, m, "retries") != "0" {
		t.Fatalf("CONSUME after REQUEUE: %+v", m)
	}
	do("MSGQUEUE.PUBLISH", "dl", "y")
	do("MSGQUEUE.PUBLISH", "dl", "z")
	if got := do("MSGQUEUE.PURGE", "dl"); got.Int != 2 {
		t.Fatalf("PURGE: %v", got)
	}
	if got := do("MSGQUEUE.DELETE", "dl"); got.Int != 1 {
		t.Fatalf("DELETE: %v", got)
	}
	do("FLUSHALL")
	if got := c.do("MSGQUEUE.STATS", "jobs"); got.Type != '-' {
		t.Fatalf("STATS after FLUSHALL: %v", got)
	}
}

// Consumers racing on one queue each get a message at most once.
func TestMessageQueueDeliversOnce(t *testing.T) {
	cfg := aofTxConfig(t)
	s := startToggleServer(t, cfg)
	defer stopReplServer(s)
	c := dialRepl(t, s)
	c.do("MSGQUEUE.CREATE", "work")
	const n = 200
	for i := range n {
		c.do("MSGQUEUE.PUBLISH", "work", strconv.Itoa(i))
	}
	var mu sync.Mutex
	seen := map[string]int{}
	var wg sync.WaitGroup
	for range 8 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			w := dialRepl(t, s)
			for {
				v := w.do("MSGQUEUE.CONSUME", "work", "60000")
				if v.IsNull || len(v.Array) == 0 {
					return
				}
				body := ""
				for i := 0; i+1 < len(v.Array); i += 2 {
					if string(v.Array[i].Bulk) == "body" {
						body = string(v.Array[i+1].Bulk)
					}
				}
				mu.Lock()
				seen[body]++
				mu.Unlock()
			}
		}()
	}
	wg.Wait()
	if len(seen) != n {
		t.Fatalf("%d distinct messages delivered, want %d", len(seen), n)
	}
	for body, times := range seen {
		if times != 1 {
			t.Fatalf("message %s delivered %d times", body, times)
		}
	}
}
