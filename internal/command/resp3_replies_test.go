package command

import (
	"strings"
	"testing"
)

// TestRESP3ReplyTypes pins the RESP3 types Redis 7.4 replies with where a
// RESP2 client gets a flat array or a bulk string. The expected replies were
// taken from a Redis 7.4 server.
func TestRESP3ReplyTypes(t *testing.T) {
	h := newLuaHarness(t)
	RegisterSetCommands(h.router)
	RegisterGeoCommands(h.router)
	do3 := func(args ...string) string {
		t.Helper()
		ctx, buf := bufCtx(args[0], bytesArgs(args[1:]...), h.s)
		ctx.Writer.SetRESP3(true)
		if err := h.router.Execute(ctx); err != nil {
			ctx.Writer.WriteError(err.Error())
		}
		return buf.String()
	}
	h.do("SADD", "s", "a")
	h.do("SET", "a", "ohmytext")
	h.do("SET", "b", "mynewtext")
	h.do("HSET", "h", "f", "v")
	h.do("GEOADD", "geo", "13.361389", "38.115556", "p1")
	h.do("XADD", "x", "1-1", "f", "v")
	h.do("XGROUP", "CREATE", "x", "g", "0")
	for _, c := range []struct {
		args []string
		want string
	}{
		{[]string{"SMEMBERS", "s"}, "~1\r\n$1\r\na\r\n"},
		{[]string{"SMEMBERS", "nos"}, "~0\r\n"},
		{[]string{"SINTER", "s", "nos"}, "~0\r\n"},
		{[]string{"SPOP", "nos", "2"}, "~0\r\n"},
		{[]string{"SRANDMEMBER", "nos", "2"}, "*0\r\n"},
		{[]string{"LCS", "a", "b", "IDX"}, "%2\r\n$7\r\nmatches\r\n*2\r\n*2\r\n*2\r\n:4\r\n:7\r\n*2\r\n:5\r\n:8\r\n*2\r\n*2\r\n:2\r\n:3\r\n*2\r\n:0\r\n:1\r\n$3\r\nlen\r\n:6\r\n"},
		{[]string{"HRANDFIELD", "h", "1", "WITHVALUES"}, "*1\r\n*2\r\n$1\r\nf\r\n$1\r\nv\r\n"},
		{[]string{"GEOPOS", "geo", "p1", "nope"}, "*2\r\n*2\r\n,13.36138933897018433\r\n,38.11555639549629859\r\n_\r\n"},
		{[]string{"XINFO", "GROUPS", "x"}, "*1\r\n%6\r\n$4\r\nname\r\n$1\r\ng\r\n$9\r\nconsumers\r\n:0\r\n$7\r\npending\r\n:0\r\n$17\r\nlast-delivered-id\r\n$3\r\n0-0\r\n$12\r\nentries-read\r\n_\r\n$3\r\nlag\r\n:1\r\n"},
		{[]string{"EVAL", "return {double=0.1}", "0"}, ",0.1\r\n"},
		{[]string{"EVAL", "return {map={a=1}}", "0"}, "%1\r\n$1\r\na\r\n:1\r\n"},
		{[]string{"EVAL", "return {set={a=true}}", "0"}, "~1\r\n$1\r\na\r\n"},
		{[]string{"EVAL", "return {big_number='123'}", "0"}, "(123\r\n"},
		{[]string{"EVAL", "return {verbatim_string={format='txt', string='x'}}", "0"}, "=5\r\ntxt:x\r\n"},
		{[]string{"EVAL", "redis.setresp(3) return {true, false}", "0"}, "*2\r\n#t\r\n#f\r\n"},
		{[]string{"EVAL", "redis.setresp(3) return redis.call('smembers', 's')", "0"}, "~1\r\n$1\r\na\r\n"},
		{[]string{"EVAL", "redis.setresp(3) return redis.call('hgetall', 'h').map.f", "0"}, "$1\r\nv\r\n"},
		{[]string{"EVAL", "redis.setresp(3) return type(redis.call('get', 'nokey'))", "0"}, "$3\r\nnil\r\n"},
		{[]string{"EVAL", "redis.setresp(3) return tostring(redis.call('smembers', 's').set.a)", "0"}, "$4\r\ntrue\r\n"},
	} {
		if got := do3(c.args...); got != c.want {
			t.Errorf("%v\n got: %q\nwant: %q", c.args, got, c.want)
		}
	}
	if got := do3("XINFO", "STREAM", "x"); !strings.HasPrefix(got, "%10\r\n$6\r\nlength\r\n:1\r\n") {
		t.Errorf("XINFO STREAM in RESP3: %q", got)
	}
	for _, args := range [][]string{{"INFO", "server"}, {"LOLWUT"}} {
		if got := do3(args...); !strings.HasPrefix(got, "=") {
			t.Errorf("%v in RESP3 is not a verbatim string: %q", args, got)
		}
	}
	// RESP2 keeps the flat forms.
	for _, c := range []struct {
		args []string
		want string
	}{
		{[]string{"SMEMBERS", "s"}, "*1\r\n$1\r\na\r\n"},
		{[]string{"HRANDFIELD", "h", "1", "WITHVALUES"}, "*2\r\n$1\r\nf\r\n$1\r\nv\r\n"},
		{[]string{"EVAL", "return {double=0.1}", "0"}, "$3\r\n0.1\r\n"},
		{[]string{"EVAL", "redis.setresp(3) return {true, false}", "0"}, "*2\r\n:1\r\n:0\r\n"},
		{[]string{"EVAL", "return {map={a=1}}", "0"}, "*2\r\n$1\r\na\r\n:1\r\n"},
	} {
		if got := h.do(c.args...); got != c.want {
			t.Errorf("RESP2 %v\n got: %q\nwant: %q", c.args, got, c.want)
		}
	}
}
