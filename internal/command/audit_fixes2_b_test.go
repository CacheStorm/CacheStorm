package command

import (
	"bytes"
	"strconv"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/cachestorm/cachestorm/internal/resp"
	"github.com/cachestorm/cachestorm/internal/store"
)

func newAuditB(t *testing.T) (*Router, *store.Store) {
	t.Helper()
	r := NewRouter()
	RegisterResilienceCommands(r)
	return r, store.NewStore()
}

func auditBCtx(s *store.Store, cmd string, args ...string) (*Context, *bytes.Buffer) {
	buf := &bytes.Buffer{}
	return NewContext(cmd, bytesArgs(args...), s, resp.NewWriter(buf)), buf
}

func auditBRun(t *testing.T, r *Router, s *store.Store, cmd string, args ...string) string {
	t.Helper()
	ctx, buf := auditBCtx(s, cmd, args...)
	if err := r.Execute(ctx); err != nil {
		t.Fatalf("%s: %v", cmd, err)
	}
	return buf.String()
}

func auditBExpect(t *testing.T, got, want string) {
	t.Helper()
	if got != want {
		t.Fatalf("got %q, want %q", got, want)
	}
}

func auditBPrefix(t *testing.T, got, prefix string) {
	t.Helper()
	if !strings.HasPrefix(got, prefix) {
		t.Fatalf("got %q, want prefix %q", got, prefix)
	}
}

func auditBField(t *testing.T, reply, field string) string {
	t.Helper()
	parts := strings.Split(reply, "\r\n")
	for i, p := range parts {
		if p == field && i+1 < len(parts) {
			next := parts[i+1]
			if strings.HasPrefix(next, "$") && i+2 < len(parts) {
				return parts[i+2]
			}
			return strings.TrimPrefix(next, ":")
		}
	}
	t.Fatalf("field %q not in %q", field, reply)
	return ""
}

func auditBRunAsync(r *Router, s *store.Store, cmd string, args ...string) <-chan string {
	out := make(chan string, 1)
	go func() {
		ctx, buf := auditBCtx(s, cmd, args...)
		_ = r.Execute(ctx)
		out <- buf.String()
	}()
	return out
}

func auditBRecv(t *testing.T, ch <-chan string, within time.Duration) string {
	t.Helper()
	select {
	case v := <-ch:
		return v
	case <-time.After(within):
		t.Fatalf("command did not return within %v", within)
		return ""
	}
}

func TestAuditB_CircuitTransitions(t *testing.T) {
	r, s := newAuditB(t)
	auditBExpect(t, auditBRun(t, r, s, "CIRCUITX.CREATE", "cb", "2", "60", "2"), "+OK\r\n")
	auditBExpect(t, auditBRun(t, r, s, "CIRCUITX.ALLOW", "cb"), ":1\r\n")

	auditBExpect(t, auditBRun(t, r, s, "CIRCUITX.RECORD", "cb", "FAILURE"), "$6\r\nclosed\r\n")
	auditBExpect(t, auditBRun(t, r, s, "CIRCUITX.RECORD", "cb", "SUCCESS"), "$6\r\nclosed\r\n")
	auditBExpect(t, auditBRun(t, r, s, "CIRCUITX.RECORD", "cb", "FAILURE"), "$6\r\nclosed\r\n")
	auditBExpect(t, auditBRun(t, r, s, "CIRCUITX.RECORD", "cb", "failure"), "$4\r\nopen\r\n")
	auditBExpect(t, auditBRun(t, r, s, "CIRCUITX.ALLOW", "cb"), ":0\r\n")
	if v := auditBField(t, auditBRun(t, r, s, "CIRCUITX.STATUS", "cb"), "retry_after"); v == "0" {
		t.Fatalf("an open circuit should report a retry_after, got %s", v)
	}

	time.Sleep(80 * time.Millisecond)
	auditBExpect(t, auditBField(t, auditBRun(t, r, s, "CIRCUITX.STATUS", "cb"), "state"), "half-open")
	auditBExpect(t, auditBRun(t, r, s, "CIRCUITX.ALLOW", "cb"), ":1\r\n")
	auditBExpect(t, auditBRun(t, r, s, "CIRCUITX.RECORD", "cb", "FAILURE"), "$4\r\nopen\r\n")

	time.Sleep(80 * time.Millisecond)
	auditBExpect(t, auditBRun(t, r, s, "CIRCUITX.RECORD", "cb", "SUCCESS"), "$9\r\nhalf-open\r\n")
	auditBExpect(t, auditBRun(t, r, s, "CIRCUITX.RECORD", "cb", "SUCCESS"), "$6\r\nclosed\r\n")

	m := auditBRun(t, r, s, "CIRCUITX.METRICS", "cb")
	auditBExpect(t, auditBField(t, m, "total_failures"), "4")
	auditBExpect(t, auditBField(t, m, "total_successes"), "3")
	auditBExpect(t, auditBField(t, m, "rejected"), "1")
	auditBExpect(t, auditBField(t, m, "failure_rate"), "0.57")

	auditBPrefix(t, auditBRun(t, r, s, "CIRCUITX.RECORD", "cb", "MAYBE"), "-ERR")
	auditBPrefix(t, auditBRun(t, r, s, "CIRCUITX.CREATE", "bad", "0", "10"), "-ERR")
	auditBPrefix(t, auditBRun(t, r, s, "CIRCUITX.CREATE", "bad", "x", "10"), "-ERR")
	auditBPrefix(t, auditBRun(t, r, s, "CIRCUITX.ALLOW", "missing"), "-ERR")
}

func TestAuditB_PromiseAwaitBlocksUntilSettled(t *testing.T) {
	r, s := newAuditB(t)
	id := registryBulkValue(auditBRun(t, r, s, "PROMISE.CREATE"))

	auditBExpect(t, auditBRun(t, r, s, "PROMISE.AWAIT", id, "20"), "$-1\r\n")

	ch := auditBRunAsync(r, s, "PROMISE.AWAIT", id, "5000")
	time.Sleep(30 * time.Millisecond)
	select {
	case v := <-ch:
		t.Fatalf("AWAIT returned before the promise settled: %q", v)
	default:
	}
	auditBExpect(t, auditBRun(t, r, s, "PROMISE.RESOLVE", id, "v1"), "+OK\r\n")
	auditBExpect(t, auditBRecv(t, ch, 2*time.Second), "$2\r\nv1\r\n")
	auditBExpect(t, auditBRun(t, r, s, "PROMISE.AWAIT", id, "1"), "$2\r\nv1\r\n")
	auditBPrefix(t, auditBRun(t, r, s, "PROMISE.RESOLVE", id, "v2"), "-ERR")

	id2 := registryBulkValue(auditBRun(t, r, s, "PROMISE.CREATE"))
	ch = auditBRunAsync(r, s, "PROMISE.AWAIT", id2, "5000")
	time.Sleep(10 * time.Millisecond)
	auditBRun(t, r, s, "PROMISE.REJECT", id2, "boom")
	auditBExpect(t, auditBRecv(t, ch, 2*time.Second), "-ERR promise rejected: boom\r\n")

	auditBPrefix(t, auditBRun(t, r, s, "PROMISE.AWAIT", id2), "-ERR wrong number")
	auditBPrefix(t, auditBRun(t, r, s, "PROMISE.AWAIT", id2, "-1"), "-ERR timeout")
	auditBPrefix(t, auditBRun(t, r, s, "PROMISE.AWAIT", "nope", "1"), "-ERR promise not found")
}

func TestAuditB_PromiseAwaitHonoursExecDoneAndGate(t *testing.T) {
	r, s := newAuditB(t)
	id := registryBulkValue(auditBRun(t, r, s, "PROMISE.CREATE"))

	ctx, buf := auditBCtx(s, "PROMISE.AWAIT", id, "5000")
	ctx.InExec = true
	start := time.Now()
	if err := r.Execute(ctx); err != nil {
		t.Fatal(err)
	}
	if time.Since(start) > time.Second {
		t.Fatal("AWAIT blocked inside EXEC")
	}
	auditBExpect(t, buf.String(), "$-1\r\n")

	done := make(chan struct{})
	var before, after atomic.Int32
	ctx, buf = auditBCtx(s, "PROMISE.AWAIT", id, "0")
	ctx.Done = done
	ctx.BeforeWait = func() { before.Add(1) }
	ctx.AfterWait = func() { after.Add(1) }
	finished := make(chan struct{})
	go func() {
		_ = r.Execute(ctx)
		close(finished)
	}()
	time.Sleep(20 * time.Millisecond)
	close(done)
	select {
	case <-finished:
	case <-time.After(2 * time.Second):
		t.Fatal("AWAIT ignored the client going away")
	}
	auditBExpect(t, buf.String(), "$-1\r\n")
	if before.Load() != 1 || after.Load() != 1 {
		t.Fatalf("BeforeWait/AfterWait called %d/%d times, want 1/1", before.Load(), after.Load())
	}
}

func TestAuditB_FutureLifecycle(t *testing.T) {
	r, s := newAuditB(t)
	id := registryBulkValue(auditBRun(t, r, s, "FUTURE.CREATE"))
	auditBExpect(t, auditBRun(t, r, s, "FUTURE.GET", id), "$-1\r\n")

	ch := auditBRunAsync(r, s, "FUTURE.GET", id, "5000")
	time.Sleep(20 * time.Millisecond)
	auditBExpect(t, auditBRun(t, r, s, "FUTURE.COMPLETE", id, "done"), "+OK\r\n")
	auditBExpect(t, auditBRecv(t, ch, 2*time.Second), "$4\r\ndone\r\n")

	auditBExpect(t, auditBRun(t, r, s, "FUTURE.COMPLETE", id, "again"), "-ERR future is already completed\r\n")
	auditBExpect(t, auditBRun(t, r, s, "FUTURE.CANCEL", id), "-ERR future is already completed\r\n")
	auditBExpect(t, auditBRun(t, r, s, "FUTURE.GET", id), "$4\r\ndone\r\n")

	id2 := registryBulkValue(auditBRun(t, r, s, "FUTURE.CREATE"))
	ch = auditBRunAsync(r, s, "FUTURE.GET", id2, "5000")
	time.Sleep(20 * time.Millisecond)
	auditBExpect(t, auditBRun(t, r, s, "FUTURE.CANCEL", id2), "+OK\r\n")
	auditBExpect(t, auditBRecv(t, ch, 2*time.Second), "-ERR future was cancelled\r\n")
	auditBExpect(t, auditBRun(t, r, s, "FUTURE.COMPLETE", id2, "late"), "-ERR future is already cancelled\r\n")

	id3 := registryBulkValue(auditBRun(t, r, s, "FUTURE.CREATE"))
	auditBExpect(t, auditBRun(t, r, s, "FUTURE.GET", id3, "20"), "$-1\r\n")
}

func TestAuditB_RateLimiterWait(t *testing.T) {
	r, s := newAuditB(t)
	auditBRun(t, r, s, "RATELIMITER.CREATE", "rl", "1", "150")
	auditBExpect(t, auditBRun(t, r, s, "RATELIMITER.WAIT", "rl", "1000"), ":0\r\n")

	start := time.Now()
	got := auditBRun(t, r, s, "RATELIMITER.WAIT", "rl", "2000")
	elapsed := time.Since(start)
	auditBPrefix(t, got, ":")
	if elapsed < 100*time.Millisecond || elapsed > 1500*time.Millisecond {
		t.Fatalf("WAIT took %v, want about one window", elapsed)
	}

	auditBExpect(t, auditBRun(t, r, s, "RATELIMITER.WAIT", "rl", "20"), "$-1\r\n")
	auditBExpect(t, auditBField(t, auditBRun(t, r, s, "RATELIMITER.STATUS", "rl"), "current"), "1")

	auditBRun(t, r, s, "RATELIMITER.CREATE", "slow", "1", "60000")
	auditBRun(t, r, s, "RATELIMITER.TRY", "slow")
	ch := auditBRunAsync(r, s, "RATELIMITER.WAIT", "slow", "5000")
	time.Sleep(20 * time.Millisecond)
	auditBRun(t, r, s, "RATELIMITER.RESET", "slow")
	auditBPrefix(t, auditBRecv(t, ch, 2*time.Second), ":")

	ctx, buf := auditBCtx(s, "RATELIMITER.WAIT", "slow", "5000")
	ctx.InExec = true
	_ = r.Execute(ctx)
	auditBExpect(t, buf.String(), "$-1\r\n")

	auditBPrefix(t, auditBRun(t, r, s, "RATELIMITER.WAIT", "slow"), "-ERR wrong number")
	auditBPrefix(t, auditBRun(t, r, s, "RATELIMITER.CREATE", "bad", "0", "10"), "-ERR")
}

func TestAuditB_BulkheadWaiting(t *testing.T) {
	r, s := newAuditB(t)
	auditBRun(t, r, s, "BULKHEAD.CREATE", "bh", "1")
	auditBExpect(t, auditBRun(t, r, s, "BULKHEAD.ACQUIRE", "bh"), ":1\r\n")
	auditBExpect(t, auditBRun(t, r, s, "BULKHEAD.ACQUIRE", "bh"), ":0\r\n")
	st := auditBRun(t, r, s, "BULKHEAD.STATUS", "bh")
	auditBExpect(t, auditBField(t, st, "waiting"), "0")
	auditBExpect(t, auditBField(t, st, "rejected"), "1")

	ch := auditBRunAsync(r, s, "BULKHEAD.ACQUIRE", "bh", "5000")
	deadline := time.Now().Add(2 * time.Second)
	for auditBField(t, auditBRun(t, r, s, "BULKHEAD.STATUS", "bh"), "waiting") != "1" {
		if time.Now().After(deadline) {
			t.Fatal("the blocked acquire was never counted as waiting")
		}
		time.Sleep(5 * time.Millisecond)
	}
	auditBExpect(t, auditBRun(t, r, s, "BULKHEAD.RELEASE", "bh"), "+OK\r\n")
	auditBExpect(t, auditBRecv(t, ch, 2*time.Second), ":1\r\n")
	st = auditBRun(t, r, s, "BULKHEAD.STATUS", "bh")
	auditBExpect(t, auditBField(t, st, "waiting"), "0")
	auditBExpect(t, auditBField(t, st, "current"), "1")

	auditBExpect(t, auditBRun(t, r, s, "BULKHEAD.ACQUIRE", "bh", "20"), ":0\r\n")
	st = auditBRun(t, r, s, "BULKHEAD.STATUS", "bh")
	auditBExpect(t, auditBField(t, st, "waiting"), "0")
	auditBExpect(t, auditBField(t, st, "rejected"), "2")

	auditBRun(t, r, s, "BULKHEAD.RELEASE", "bh")
	auditBPrefix(t, auditBRun(t, r, s, "BULKHEAD.RELEASE", "bh"), "-ERR")
	auditBPrefix(t, auditBRun(t, r, s, "BULKHEAD.CREATE", "bad", "0"), "-ERR")
}

func TestAuditB_DebounceHonoursDelay(t *testing.T) {
	r, s := newAuditB(t)
	auditBRun(t, r, s, "DEBOUNCEX.CREATE", "d", "60")
	auditBRun(t, r, s, "DEBOUNCEX.CALL", "d", "a")
	auditBExpect(t, auditBRun(t, r, s, "DEBOUNCEX.FLUSH", "d"), "$-1\r\n")
	auditBExpect(t, auditBRun(t, r, s, "DEBOUNCEX.FLUSH", "d", "FORCE"), "$1\r\na\r\n")
	auditBExpect(t, auditBRun(t, r, s, "DEBOUNCEX.FLUSH", "d", "FORCE"), "$-1\r\n")

	auditBRun(t, r, s, "DEBOUNCEX.CALL", "d", "b")
	time.Sleep(40 * time.Millisecond)
	auditBRun(t, r, s, "DEBOUNCEX.CALL", "d", "c")
	time.Sleep(40 * time.Millisecond)
	auditBExpect(t, auditBRun(t, r, s, "DEBOUNCEX.FLUSH", "d"), "$-1\r\n")
	time.Sleep(40 * time.Millisecond)
	auditBExpect(t, auditBRun(t, r, s, "DEBOUNCEX.FLUSH", "d"), "$1\r\nc\r\n")
	auditBExpect(t, auditBRun(t, r, s, "DEBOUNCEX.FLUSH", "d"), "$-1\r\n")

	auditBPrefix(t, auditBRun(t, r, s, "DEBOUNCEX.FLUSH", "d", "NOW"), "-ERR")
	auditBPrefix(t, auditBRun(t, r, s, "DEBOUNCEX.CREATE", "bad", "-5"), "-ERR")
}

func TestAuditB_JoinMatchesKeys(t *testing.T) {
	r, s := newAuditB(t)
	auditBRun(t, r, s, "JOINX.CREATE", "j")
	auditBExpect(t, auditBRun(t, r, s, "JOINX.ADD", "j", "left", "k1", "a"), "+OK\r\n")
	auditBRun(t, r, s, "JOINX.ADD", "j", "LEFT", "k2", "b")
	auditBRun(t, r, s, "JOINX.ADD", "j", "right", "k1", "x")
	auditBRun(t, r, s, "JOINX.ADD", "j", "right", "k1", "y")
	auditBRun(t, r, s, "JOINX.ADD", "j", "right", "k3", "z")

	row := func(k, l, rv string) string {
		return "*6\r\n$3\r\nkey\r\n$" + strconv.Itoa(len(k)) + "\r\n" + k + "\r\n$4\r\nleft\r\n$" + strconv.Itoa(len(l)) + "\r\n" + l +
			"\r\n$5\r\nright\r\n$" + strconv.Itoa(len(rv)) + "\r\n" + rv + "\r\n"
	}
	auditBExpect(t, auditBRun(t, r, s, "JOINX.GET", "j"), "*2\r\n"+row("k1", "a", "x")+row("k1", "a", "y"))
	auditBExpect(t, auditBRun(t, r, s, "JOINX.GET", "j", "k2"), "*0\r\n")
	auditBExpect(t, auditBRun(t, r, s, "JOINX.GET", "j", "k1"), "*2\r\n"+row("k1", "a", "x")+row("k1", "a", "y"))

	auditBPrefix(t, auditBRun(t, r, s, "JOINX.ADD", "j", "middle", "k1", "q"), "-ERR side")
	auditBPrefix(t, auditBRun(t, r, s, "JOINX.ADD", "j", "left", "q"), "-ERR wrong number")
}
