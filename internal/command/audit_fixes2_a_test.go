package command

import (
	"bytes"
	"fmt"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/cachestorm/cachestorm/internal/resp"
	"github.com/cachestorm/cachestorm/internal/store"
)

func auditFixes2ARouter() *Router {
	router := NewRouter()
	RegisterExtendedCommands(router)
	RegisterEventCommands(router)
	RegisterActorCommands(router)
	RegisterAdvancedCommands2(router)
	return router
}

func auditBulk(reply string) string {
	parts := strings.Split(reply, "\r\n")
	if len(parts) < 2 || !strings.HasPrefix(parts[0], "$") || parts[0] == "$-1" {
		return ""
	}
	return parts[1]
}

func TestAuditFixes2ADocIndexFollowsWrites(t *testing.T) {
	s := store.NewStore()
	r := auditFixes2ARouter()
	runCmd(t, s, r, "DOC.INSERT", "af2-docs", "d1", "color", "red")
	runCmd(t, s, r, "DOC.INSERT", "af2-docs", "d2", "color", "red")
	if got := runCmd(t, s, r, "DOC.INDEX", "af2-docs", "color"); got != "+OK\r\n" {
		t.Fatalf("DOC.INDEX = %q", got)
	}

	find := func(value string) string { return runCmd(t, s, r, "DOC.FIND", "af2-docs", "color", value) }
	d1 := func(color string) string {
		return fmt.Sprintf("*4\r\n$3\r\n_id\r\n$2\r\nd1\r\n$5\r\ncolor\r\n$%d\r\n%s\r\n", len(color), color)
	}
	d2Red := "*4\r\n$3\r\n_id\r\n$2\r\nd2\r\n$5\r\ncolor\r\n$3\r\nred\r\n"

	if got, want := find("red"), "*2\r\n"+d1("red")+d2Red; got != want {
		t.Fatalf("FIND red = %q, want %q", got, want)
	}

	runCmd(t, s, r, "DOC.UPDATE", "af2-docs", "d1", "color", "blue")
	if got, want := find("red"), "*1\r\n"+d2Red; got != want {
		t.Fatalf("FIND red after update = %q, want %q", got, want)
	}
	if got, want := find("blue"), "*1\r\n"+d1("blue"); got != want {
		t.Fatalf("FIND blue after update = %q, want %q", got, want)
	}
	if got, want := runCmd(t, s, r, "DOC.FINDONE", "af2-docs", "color", "blue"), d1("blue"); got != want {
		t.Fatalf("FINDONE blue = %q, want %q", got, want)
	}

	runCmd(t, s, r, "DOC.DELETE", "af2-docs", "d2")
	if got := find("red"); got != "*0\r\n" {
		t.Fatalf("FIND red after delete = %q, want empty", got)
	}
	if got := runCmd(t, s, r, "DOC.FINDONE", "af2-docs", "color", "red"); got != "*-1\r\n" && got != "_\r\n" && got != "$-1\r\n" {
		t.Fatalf("FINDONE red after delete = %q, want null", got)
	}

	runCmd(t, s, r, "DOC.INSERT", "af2-docs", "d1", "color", "green")
	if got := find("blue"); got != "*0\r\n" {
		t.Fatalf("FIND blue after re-insert = %q, want empty", got)
	}
	if got, want := find("green"), "*1\r\n"+d1("green"); got != want {
		t.Fatalf("FIND green after re-insert = %q, want %q", got, want)
	}
	if got := runCmd(t, s, r, "DOC.DISTINCT", "af2-docs", "color"); got != "*1\r\n$5\r\ngreen\r\n" {
		t.Fatalf("DISTINCT = %q", got)
	}

	if got := runCmd(t, s, r, "DOC.DROPINDEX", "af2-docs", "color"); got != ":1\r\n" {
		t.Fatalf("DROPINDEX = %q", got)
	}
	if got, want := find("green"), "*1\r\n"+d1("green"); got != want {
		t.Fatalf("FIND without index = %q, want %q", got, want)
	}
}

func TestAuditFixes2AMsgQueueVisibilityTimeout(t *testing.T) {
	s := store.NewStore()
	r := auditFixes2ARouter()
	q := "af2-mq"
	runCmd(t, s, r, "MSGQUEUE.CREATE", q, "2")
	id := auditBulk(runCmd(t, s, r, "MSGQUEUE.PUBLISH", q, "body"))
	if id == "" {
		t.Fatal("PUBLISH returned no id")
	}

	if got := runCmd(t, s, r, "MSGQUEUE.CONSUME", q, "30"); !strings.Contains(got, id) || !strings.HasSuffix(got, ":0\r\n") {
		t.Fatalf("first CONSUME = %q", got)
	}
	if got := runCmd(t, s, r, "MSGQUEUE.CONSUME", q, "30"); strings.Contains(got, id) {
		t.Fatalf("message delivered twice inside its visibility timeout: %q", got)
	}

	time.Sleep(60 * time.Millisecond)
	if got := runCmd(t, s, r, "MSGQUEUE.ACK", q, id); got != ":0\r\n" {
		t.Fatalf("ACK after the timeout = %q, want :0", got)
	}
	if got := runCmd(t, s, r, "MSGQUEUE.CONSUME", q, "30"); !strings.Contains(got, id) || !strings.HasSuffix(got, ":1\r\n") {
		t.Fatalf("redelivery = %q, want the message with retries 1", got)
	}

	time.Sleep(60 * time.Millisecond)
	if got := runCmd(t, s, r, "MSGQUEUE.NACK", q, id); got != ":0\r\n" {
		t.Fatalf("NACK after the timeout = %q, want :0", got)
	}
	stats := runCmd(t, s, r, "MSGQUEUE.STATS", q)
	if !strings.Contains(stats, "$7\r\npending\r\n:0\r\n$10\r\nprocessing\r\n:0\r\n$12\r\ndead_letters\r\n:1\r\n") {
		t.Fatalf("STATS after max retries = %q, want the message dead-lettered", stats)
	}
	if got := runCmd(t, s, r, "MSGQUEUE.DEADLETTER", q); !strings.Contains(got, id) {
		t.Fatalf("DEADLETTER = %q", got)
	}

	if got := runCmd(t, s, r, "MSGQUEUE.REQUEUE", q, id); got != ":1\r\n" {
		t.Fatalf("REQUEUE = %q", got)
	}
	if got := runCmd(t, s, r, "MSGQUEUE.CONSUME", q, "10000"); !strings.Contains(got, id) || !strings.HasSuffix(got, ":0\r\n") {
		t.Fatalf("CONSUME after REQUEUE = %q", got)
	}
	if got := runCmd(t, s, r, "MSGQUEUE.NACK", q, id); got != ":1\r\n" {
		t.Fatalf("NACK in time = %q, want :1", got)
	}
	runCmd(t, s, r, "MSGQUEUE.CONSUME", q, "10000")
	if got := runCmd(t, s, r, "MSGQUEUE.ACK", q, id); got != ":1\r\n" {
		t.Fatalf("ACK in time = %q, want :1", got)
	}
	time.Sleep(20 * time.Millisecond)
	if got := runCmd(t, s, r, "MSGQUEUE.CONSUME", q); !strings.HasPrefix(got, "*-1") && !strings.HasPrefix(got, "_") && !strings.HasPrefix(got, "$-1") {
		t.Fatalf("an acked message came back: %q", got)
	}

	for _, bad := range [][]string{{"MSGQUEUE.CONSUME", q, "0"}, {"MSGQUEUE.CONSUME", q, "x"}, {"MSGQUEUE.CREATE", "af2-mq2", "0"}} {
		if got := runCmd(t, s, r, bad[0], bad[1:]...); !strings.HasPrefix(got, "-ERR") {
			t.Fatalf("%v = %q, want an error", bad, got)
		}
	}
	runCmd(t, s, r, "MSGQUEUE.DELETE", q)
}

func TestAuditFixes2AEventGetReturnsData(t *testing.T) {
	s := store.NewStore()
	r := auditFixes2ARouter()
	runCmd(t, s, r, "EVENT.EMIT", "af2.event", "user", "alice", "action", "login")
	got := runCmd(t, s, r, "EVENT.GET", "af2.event", "1")
	if !strings.Contains(got, "$4\r\ndata\r\n*4\r\n$6\r\naction\r\n$5\r\nlogin\r\n$4\r\nuser\r\n$5\r\nalice\r\n") {
		t.Fatalf("EVENT.GET = %q, want the event's data", got)
	}
	if got := runCmd(t, s, r, "EVENT.EMIT", "af2.event", "dangling"); !strings.HasPrefix(got, "-ERR") {
		t.Fatalf("EVENT.EMIT with an odd field list = %q, want an error", got)
	}
	if got := runCmd(t, s, r, "EVENT.GET", "af2.event", "-1"); !strings.HasPrefix(got, "-ERR") {
		t.Fatalf("EVENT.GET with a negative limit = %q, want an error", got)
	}
}

func TestAuditFixes2ADagRejectsCycles(t *testing.T) {
	s := store.NewStore()
	r := auditFixes2ARouter()
	runCmd(t, s, r, "DAG.CREATE", "af2-dag")
	for _, n := range []string{"c", "b", "a", "d"} {
		runCmd(t, s, r, "DAG.ADDNODE", "af2-dag", n)
	}
	for _, e := range [][2]string{{"a", "b"}, {"b", "c"}, {"a", "c"}, {"a", "b"}} {
		if got := runCmd(t, s, r, "DAG.ADDEDGE", "af2-dag", e[0], e[1]); got != "+OK\r\n" {
			t.Fatalf("ADDEDGE %v = %q", e, got)
		}
	}
	for _, e := range [][2]string{{"c", "a"}, {"b", "a"}, {"d", "d"}} {
		if got := runCmd(t, s, r, "DAG.ADDEDGE", "af2-dag", e[0], e[1]); !strings.Contains(got, "cycle") {
			t.Fatalf("ADDEDGE %v = %q, want a cycle error", e, got)
		}
	}
	want := "*4\r\n$1\r\na\r\n$1\r\nb\r\n$1\r\nc\r\n$1\r\nd\r\n"
	if got := runCmd(t, s, r, "DAG.TOPO", "af2-dag"); got != want {
		t.Fatalf("TOPO = %q, want %q", got, want)
	}
	if got := runCmd(t, s, r, "DAG.PARENTS", "af2-dag", "c"); got != "*2\r\n$1\r\na\r\n$1\r\nb\r\n" {
		t.Fatalf("PARENTS c = %q", got)
	}
	runCmd(t, s, r, "DAG.DELETE", "af2-dag")
}

func TestAuditFixes2ARingIsConsistent(t *testing.T) {
	s := store.NewStore()
	r := auditFixes2ARouter()
	runCmd(t, s, r, "RING.CREATE", "af2-ring", "100")
	for i := 1; i <= 4; i++ {
		runCmd(t, s, r, "RING.ADD", "af2-ring", fmt.Sprintf("n%d", i))
	}
	if got := runCmd(t, s, r, "RING.ADD", "af2-ring", "n1"); got != ":4\r\n" {
		t.Fatalf("adding a node twice = %q, want :4", got)
	}

	const keys = 2000
	lookup := func() []string {
		out := make([]string, keys)
		for i := range out {
			out[i] = auditBulk(runCmd(t, s, r, "RING.GET", "af2-ring", fmt.Sprintf("key:%d", i)))
		}
		return out
	}
	before := lookup()
	counts := map[string]int{}
	for _, n := range before {
		counts[n]++
	}
	for i := 1; i <= 4; i++ {
		if c := counts[fmt.Sprintf("n%d", i)]; c < keys/8 {
			t.Fatalf("node n%d owns only %d of %d keys: %v", i, c, keys, counts)
		}
	}

	runCmd(t, s, r, "RING.ADD", "af2-ring", "n5")
	after := lookup()
	moved := 0
	for i := range before {
		if before[i] != after[i] {
			moved++
			if after[i] != "n5" {
				t.Fatalf("key:%d moved from %s to %s, not to the new node", i, before[i], after[i])
			}
		}
	}
	if moved == 0 || moved > keys*2/5 {
		t.Fatalf("adding a fifth node moved %d of %d keys", moved, keys)
	}

	runCmd(t, s, r, "RING.REMOVE", "af2-ring", "n5")
	restored := lookup()
	for i := range before {
		if before[i] != restored[i] {
			t.Fatalf("key:%d maps to %s after removing n5, was %s", i, restored[i], before[i])
		}
	}
	if got := runCmd(t, s, r, "RING.CREATE", "af2-ring2", "0"); !strings.HasPrefix(got, "-ERR") {
		t.Fatalf("RING.CREATE with 0 replicas = %q, want an error", got)
	}
}

func semCtx(s *store.Store, args ...string) (*Context, *bytes.Buffer) {
	parsed := make([][]byte, len(args))
	for i, a := range args {
		parsed[i] = []byte(a)
	}
	ctx := newTestContext("SEM.ACQUIRE", parsed, s)
	buf := &bytes.Buffer{}
	ctx.Writer = resp.NewWriter(buf)
	return ctx, buf
}

func TestAuditFixes2ASemaphoreCounts(t *testing.T) {
	s := store.NewStore()
	r := auditFixes2ARouter()
	runCmd(t, s, r, "SEM.CREATE", "af2-sem", "2")
	if got := runCmd(t, s, r, "SEM.TRYACQUIRE", "af2-sem", "2"); got != ":1\r\n" {
		t.Fatalf("TRYACQUIRE 2 = %q", got)
	}
	if got := runCmd(t, s, r, "SEM.TRYACQUIRE", "af2-sem", "1"); got != ":0\r\n" {
		t.Fatalf("TRYACQUIRE on an empty semaphore = %q", got)
	}
	start := time.Now()
	if got := runCmd(t, s, r, "SEM.ACQUIRE", "af2-sem", "1", "40"); got != ":0\r\n" {
		t.Fatalf("ACQUIRE with a timeout = %q, want :0", got)
	}
	if time.Since(start) < 30*time.Millisecond {
		t.Fatal("ACQUIRE with a timeout returned without waiting")
	}
	want := "*6\r\n$5\r\nvalue\r\n:0\r\n$3\r\nmax\r\n:2\r\n$7\r\nwaiters\r\n:0\r\n"
	if got := runCmd(t, s, r, "SEM.VALUE", "af2-sem"); got != want {
		t.Fatalf("VALUE after a timed-out ACQUIRE = %q, want %q", got, want)
	}

	var before, after atomic.Int32
	ctx, buf := semCtx(s, "af2-sem", "1")
	ctx.BeforeWait = func() { before.Add(1) }
	ctx.AfterWait = func() { after.Add(1) }
	done := make(chan struct{})
	go func() {
		_ = r.Execute(ctx)
		close(done)
	}()
	deadline := time.Now().Add(2 * time.Second)
	for !strings.Contains(runCmd(t, s, r, "SEM.VALUE", "af2-sem"), "waiters\r\n:1\r\n") {
		if time.Now().After(deadline) {
			t.Fatal("blocked ACQUIRE never showed up as a waiter")
		}
		time.Sleep(time.Millisecond)
	}
	if got := runCmd(t, s, r, "SEM.RELEASE", "af2-sem", "1"); got != "+OK\r\n" {
		t.Fatalf("RELEASE = %q", got)
	}
	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("RELEASE did not wake the blocked ACQUIRE")
	}
	if buf.String() != ":1\r\n" || before.Load() != 1 || after.Load() != 1 {
		t.Fatalf("blocked ACQUIRE replied %q, BeforeWait %d, AfterWait %d", buf.String(), before.Load(), after.Load())
	}
	if got := runCmd(t, s, r, "SEM.VALUE", "af2-sem"); got != want {
		t.Fatalf("VALUE after the woken ACQUIRE = %q, want %q", got, want)
	}

	cancel := make(chan struct{})
	ctx, _ = semCtx(s, "af2-sem", "1", "0")
	ctx.Done = cancel
	done = make(chan struct{})
	go func() {
		_ = r.Execute(ctx)
		close(done)
	}()
	time.Sleep(10 * time.Millisecond)
	close(cancel)
	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("ACQUIRE did not stop when the client went away")
	}
	if got := runCmd(t, s, r, "SEM.VALUE", "af2-sem"); got != want {
		t.Fatalf("VALUE after a cancelled ACQUIRE = %q, want %q", got, want)
	}

	ctx, buf = semCtx(s, "af2-sem", "1", "0")
	ctx.InExec = true
	_ = r.Execute(ctx)
	if buf.String() != ":0\r\n" {
		t.Fatalf("ACQUIRE inside EXEC = %q, want :0 without blocking", buf.String())
	}

	runCmd(t, s, r, "SEM.RELEASE", "af2-sem", "2")
	for _, bad := range [][]string{
		{"SEM.RELEASE", "af2-sem", "1"},
		{"SEM.ACQUIRE", "af2-sem", "3"},
		{"SEM.ACQUIRE", "af2-sem", "-1"},
		{"SEM.TRYACQUIRE", "af2-sem", "0"},
		{"SEM.CREATE", "af2-sem2", "0"},
	} {
		if got := runCmd(t, s, r, bad[0], bad[1:]...); !strings.HasPrefix(got, "-ERR") {
			t.Fatalf("%v = %q, want an error", bad, got)
		}
	}
	if got := runCmd(t, s, r, "SEM.VALUE", "af2-sem"); !strings.Contains(got, "value\r\n:2\r\n") {
		t.Fatalf("VALUE after rejected calls = %q, want value 2", got)
	}
}

func TestAuditFixes2ALevelGauge(t *testing.T) {
	s := store.NewStore()
	r := auditFixes2ARouter()
	runCmd(t, s, r, "LEVEL.CREATE", "a", "10")
	runCmd(t, s, r, "LEVEL.CREATE", "a_current", "3")
	if got := runCmd(t, s, r, "LEVEL.SET", "a", "7"); got != "+OK\r\n" {
		t.Fatalf("SET = %q", got)
	}
	if got := runCmd(t, s, r, "LEVEL.GET", "a"); got != "*4\r\n$7\r\ncurrent\r\n:7\r\n$3\r\nmax\r\n:10\r\n" {
		t.Fatalf("GET a = %q", got)
	}
	if got := runCmd(t, s, r, "LEVEL.GET", "a_current"); got != "*4\r\n$7\r\ncurrent\r\n:0\r\n$3\r\nmax\r\n:3\r\n" {
		t.Fatalf("GET a_current = %q", got)
	}
	if got := runCmd(t, s, r, "LEVEL.LIST"); !strings.Contains(got, "$1\r\na\r\n") || !strings.Contains(got, "$9\r\na_current\r\n") {
		t.Fatalf("LIST = %q", got)
	}
	for _, bad := range [][]string{{"LEVEL.SET", "a", "11"}, {"LEVEL.SET", "a", "-1"}, {"LEVEL.SET", "af2-missing", "1"}, {"LEVEL.GET", "af2-missing"}} {
		if got := runCmd(t, s, r, bad[0], bad[1:]...); !strings.HasPrefix(got, "-ERR") {
			t.Fatalf("%v = %q, want an error", bad, got)
		}
	}
	runCmd(t, s, r, "LEVEL.DELETE", "a")
	runCmd(t, s, r, "LEVEL.DELETE", "a_current")
	if got := runCmd(t, s, r, "LEVEL.GET", "a"); !strings.HasPrefix(got, "-ERR") {
		t.Fatalf("GET after DELETE = %q, want an error", got)
	}
}

func TestAuditFixes2ASessionXDataAndTTL(t *testing.T) {
	s := store.NewStore()
	r := auditFixes2ARouter()
	id := auditBulk(runCmd(t, s, r, "SESSIONX.CREATE", "alice", "60"))
	if got := runCmd(t, s, r, "SESSIONX.SET", id, "theme", "dark", "lang", "en"); got != "+OK\r\n" {
		t.Fatalf("SET = %q", got)
	}
	got := runCmd(t, s, r, "SESSIONX.GET", id)
	if !strings.Contains(got, "$4\r\ndata\r\n*4\r\n$4\r\nlang\r\n$2\r\nen\r\n$5\r\ntheme\r\n$4\r\ndark\r\n") || !strings.Contains(got, "$5\r\nalice\r\n") {
		t.Fatalf("GET = %q, want the user and the session data", got)
	}
	if !strings.Contains(runCmd(t, s, r, "SESSIONX.LIST"), id) {
		t.Fatal("LIST does not show a live session")
	}

	time.Sleep(90 * time.Millisecond)
	if got := runCmd(t, s, r, "SESSIONX.GET", id); !strings.HasPrefix(got, "-ERR") {
		t.Fatalf("GET after the TTL = %q, want an error", got)
	}
	if got := runCmd(t, s, r, "SESSIONX.SET", id, "k", "v"); !strings.HasPrefix(got, "-ERR") {
		t.Fatalf("SET after the TTL = %q, want an error", got)
	}
	if strings.Contains(runCmd(t, s, r, "SESSIONX.LIST"), id) {
		t.Fatal("LIST still shows an expired session")
	}
	for _, bad := range [][]string{{"SESSIONX.CREATE", "bob", "0"}, {"SESSIONX.CREATE", "bob", "x"}, {"SESSIONX.SET", "x", "k"}} {
		if got := runCmd(t, s, r, bad[0], bad[1:]...); !strings.HasPrefix(got, "-ERR") {
			t.Fatalf("%v = %q, want an error", bad, got)
		}
	}
}
