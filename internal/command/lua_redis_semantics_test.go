package command

import (
	"fmt"
	"strings"
	"sync"
	"testing"

	"github.com/cachestorm/cachestorm/internal/store"
)

// These tests pin the Redis 7.4 scripting semantics through the router:
// KEYS/ARGV globals, redis.call dispatching to the registered handlers,
// reply conversion both ways and Redis' error texts. The expected replies
// were taken from a Redis 7.4 server.

type luaHarness struct {
	t      *testing.T
	s      *store.Store
	router *Router
}

func newLuaHarness(t *testing.T) *luaHarness {
	t.Helper()
	savedEngine := scriptEngine
	scriptEngine = nil
	functionOnce = sync.Once{}
	functionRegistry = nil
	t.Cleanup(func() {
		scriptEngine = savedEngine
		functionOnce = sync.Once{}
		functionRegistry = nil
	})
	router := NewRouter()
	for _, register := range []func(*Router){
		RegisterStringCommands, RegisterKeyCommands, RegisterServerCommands,
		RegisterHashCommands, RegisterListCommands, RegisterStreamCommands,
		RegisterPubSubCommands, RegisterScriptCommands, RegisterFunctionCommands,
		RegisterTransactionCommands,
	} {
		register(router)
	}
	return &luaHarness{t: t, s: store.NewStore(), router: router}
}

func (h *luaHarness) do(args ...string) string {
	h.t.Helper()
	ctx, buf := bufCtx(args[0], bytesArgs(args[1:]...), h.s)
	if err := h.router.Execute(ctx); err != nil {
		ctx.Writer.WriteError(err.Error())
	}
	return buf.String()
}

func (h *luaHarness) expect(want string, args ...string) {
	h.t.Helper()
	if got := h.do(args...); got != want {
		h.t.Fatalf("%q\n got: %q\nwant: %q", args, got, want)
	}
}

func scriptErr(msg, script string, line int) string {
	return fmt.Sprintf("-%s script: %s, on @user_script:%d.\r\n", msg, ScriptSHA(script), line)
}

func TestLuaKeysArgvGlobals(t *testing.T) {
	h := newLuaHarness(t)
	h.expect("*6\r\n$2\r\nk1\r\n$2\r\nk2\r\n$2\r\na1\r\n$2\r\na2\r\n:2\r\n:2\r\n",
		"EVAL", "return {KEYS[1],KEYS[2],ARGV[1],ARGV[2],#KEYS,#ARGV}", "2", "k1", "k2", "a1", "a2")
	// redis.KEYS / redis.ARGV stay as aliases of the globals.
	h.expect("$2\r\nk1\r\n", "EVAL", "return redis.KEYS[1]", "1", "k1")
	h.expect("-ERR Number of keys can't be negative\r\n", "EVAL", "return 1", "-1")
	h.expect("-ERR Number of keys can't be greater than number of args\r\n", "EVAL", "return 1", "5", "a")
	h.expect("-ERR value is not an integer or out of range\r\n", "EVAL", "return 1", "abc")
}

func TestLuaRedisCallReplyConversion(t *testing.T) {
	h := newLuaHarness(t)
	h.do("SET", "s", "v")
	h.do("RPUSH", "l", "a", "b")
	h.expect("$1\r\nv\r\n", "EVAL", "return redis.call('GET',KEYS[1])", "1", "s")
	h.expect("*2\r\n$1\r\na\r\n$1\r\nb\r\n", "EVAL", "return redis.call('LRANGE',KEYS[1],0,-1)", "1", "l")
	h.expect("+OK\r\n", "EVAL", "return redis.call('SET','x','1')", "0")
	h.expect("$5\r\ntable\r\n", "EVAL", "return type(redis.call('SET','y','1'))", "0")
	h.expect("$7\r\nboolean\r\n", "EVAL", "return type(redis.call('GET','missing'))", "0")
	h.expect("+PONG\r\n", "EVAL", "return redis.call('PING')", "0")
	h.expect("$3\r\n1.5\r\n", "EVAL", "return redis.call('INCRBYFLOAT','f','1.5')", "0")
	h.expect("$3\r\n1-1\r\n", "EVAL", "return redis.call('XADD','st','1-1','f','v')", "0")
	h.expect(":0\r\n", "EVAL", "return redis.call('PUBLISH','ch','m')", "0")
	// Numbers are passed as their shortest form; integers in plain digits.
	h.do("EVAL", "redis.call('SET','n',100000000000000000) redis.call('SET','m',3.5)", "0")
	h.expect("$18\r\n100000000000000000\r\n", "GET", "n")
	h.expect("$3\r\n3.5\r\n", "GET", "m")
}

func TestLuaReturnConversion(t *testing.T) {
	h := newLuaHarness(t)
	h.expect("+FINE\r\n", "EVAL", "return {ok='FINE'}", "0")
	h.expect("-MYERR boom\r\n", "EVAL", "return {err='MYERR boom'}", "0")
	h.expect("$-1\r\n", "EVAL", "return false", "0")
	h.expect(":1\r\n", "EVAL", "return true", "0")
	h.expect(":3\r\n", "EVAL", "return 3.99", "0")
	h.expect(":-3\r\n", "EVAL", "return -3.7", "0")
	h.expect("*1\r\n:1\r\n", "EVAL", "return {1,nil,3}", "0")
	h.expect("+X\r\n", "EVAL", "return redis.status_reply('X')", "0")
	h.expect("-E1 nope\r\n", "EVAL", "return redis.error_reply('E1 nope')", "0")
	h.expect("-ERR nocode\r\n", "EVAL", "return redis.error_reply('nocode')", "0")
	h.expect("*3\r\n:1\r\n+x\r\n-E y\r\n", "EVAL", "return {1,{ok='x'},{err='E y'}}", "0")
	h.expect("$40\r\na9993e364706816aba3e25717850c26c9cd0d89d\r\n", "EVAL", "return redis.sha1hex('abc')", "0")
}

func TestLuaRedisCallErrors(t *testing.T) {
	h := newLuaHarness(t)
	h.do("SET", "s", "v")

	script := "return redis.call('INCR',KEYS[1])"
	h.expect(scriptErr("ERR value is not an integer or out of range", script, 1), "EVAL", script, "1", "s")
	h.expect("-ERR value is not an integer or out of range\r\n", "EVAL", "return redis.pcall('INCR',KEYS[1])", "1", "s")
	h.expect("$5\r\ntable\r\n", "EVAL", "return type(redis.pcall('INCR',KEYS[1]))", "1", "s")

	script = "return redis.call('NOSUCHCMD')"
	h.expect(scriptErr("ERR Unknown Redis command called from script", script, 1), "EVAL", script, "0")
	script = "return redis.call('GET')"
	h.expect(scriptErr("ERR Wrong number of args calling Redis command from script", script, 1), "EVAL", script, "0")
	script = "return redis.call('EVAL','return 1','0')"
	h.expect(scriptErr("ERR This Redis command is not allowed from script", script, 1), "EVAL", script, "0")
	script = "return redis.call('GET', {})"
	h.expect(scriptErr("ERR Lua redis lib command arguments must be strings or integers", script, 1), "EVAL", script, "0")

	// A raised error reports the line it was raised on.
	script = "\n\nerror('custom')"
	h.expect(scriptErr("ERR user_script:3: custom", script, 3), "EVAL", script, "0")

	// A failed redis.call stops the script: the write after it never runs.
	script = "redis.call('INCR', KEYS[1]) redis.call('SET', 'after', '1')"
	h.do("EVAL", script, "1", "s")
	h.expect(":0\r\n", "EXISTS", "after")

	// Lua's pcall gets the error message of a failed redis.call.
	h.expect("$65\r\nWRONGTYPE Operation against a key holding the wrong kind of value\r\n",
		"EVAL", "redis.call('RPUSH','lst','a') local ok, e = pcall(redis.call, 'GET', 'lst') return e", "0")
}

func TestLuaEvalshaAndScriptCache(t *testing.T) {
	h := newLuaHarness(t)
	h.expect("$40\r\n098e0f0d1448c0a81dafe820f66d460eb09263da\r\n", "SCRIPT", "LOAD", "return ARGV[1]")
	h.expect("$1\r\nx\r\n", "EVALSHA", "098e0f0d1448c0a81dafe820f66d460eb09263da", "0", "x")
	h.expect("$1\r\ny\r\n", "EVALSHA", "098E0F0D1448C0A81DAFE820F66D460EB09263DA", "0", "y")
	h.expect("-NOSCRIPT No matching script. Please use EVAL.\r\n", "EVALSHA", "0000000000000000000000000000000000000000", "0")
	// EVAL caches the script it runs.
	h.do("EVAL", "return 7", "0")
	h.expect("*2\r\n:1\r\n:0\r\n", "SCRIPT", "EXISTS", ScriptSHA("return 7"), "nope")
	h.expect("+OK\r\n", "SCRIPT", "FLUSH")
	h.expect("*1\r\n:0\r\n", "SCRIPT", "EXISTS", ScriptSHA("return 7"))
	h.expect("-NOTBUSY No scripts in execution right now.\r\n", "SCRIPT", "KILL")
	if got := h.do("SCRIPT", "LOAD", "syntax error"); !strings.HasPrefix(got, "-ERR Error compiling script (new function): user_script:1:") {
		t.Fatalf("SCRIPT LOAD of a bad script: %q", got)
	}
}

func TestLuaEvalCacheIsBounded(t *testing.T) {
	h := newLuaHarness(t)
	pinned := "return 'pinned'"
	h.do("SCRIPT", "LOAD", pinned)
	first := "return 0"
	for i := 0; i <= maxEvalScripts; i++ {
		h.do("EVAL", fmt.Sprintf("return %d", i), "0")
	}
	h.expect("*2\r\n:0\r\n:1\r\n", "SCRIPT", "EXISTS", ScriptSHA(first), ScriptSHA(pinned))
	h.expect("*1\r\n:1\r\n", "SCRIPT", "EXISTS", ScriptSHA(fmt.Sprintf("return %d", maxEvalScripts)))
}

func TestLuaReadOnlyScripts(t *testing.T) {
	h := newLuaHarness(t)
	h.do("SET", "s", "v")
	h.expect("$1\r\nv\r\n", "EVAL_RO", "return redis.call('GET','s')", "0")
	script := "return redis.call('SET','s','x')"
	h.expect(scriptErr("ERR Write commands are not allowed from read-only scripts.", script, 1), "EVAL_RO", script, "0")
	script = "#!lua flags=no-writes\nreturn redis.call('SET','s','x')"
	h.expect(scriptErr("ERR Write commands are not allowed from read-only scripts.", script, 2), "EVAL", script, "0")
	h.expect("$1\r\nv\r\n", "GET", "s")
}

func TestLuaSandbox(t *testing.T) {
	h := newLuaHarness(t)
	script := "x = 1 return x"
	h.expect(scriptErr("ERR user_script:1: Attempt to modify a readonly table", script, 1), "EVAL", script, "0")
	script = "return undefinedvar"
	h.expect(scriptErr("ERR user_script:1: Script attempted to access nonexistent global variable 'undefinedvar'", script, 1), "EVAL", script, "0")
	h.expect("$8\r\nfunction\r\n", "EVAL", "return type(os.clock)", "0")
	h.expect("$3\r\nnil\r\n", "EVAL", "return type(rawget(_G, 'print'))", "0")
}

func TestLuaBundledLibraries(t *testing.T) {
	h := newLuaHarness(t)
	h.expect("$7\r\n{\"a\":1}\r\n", "EVAL", "return cjson.encode({a=1})", "0")
	h.expect("$20\r\n[1,2,\"x\",true,false]\r\n", "EVAL", "return cjson.encode({1,2,'x',true,false})", "0")
	h.expect("$9\r\n\"a\\/b\\\"c\"\r\n", "EVAL", "return cjson.encode('a/b\"c')", "0")
	h.expect("*4\r\n:1\r\n:2\r\n$1\r\nx\r\n:1\r\n",
		"EVAL", "local t = cjson.decode('{\"a\":[1,2,{\"b\":null}],\"c\":\"x\"}') return {t.a[1], t.a[2], t.c, t.a[3].b == cjson.null and 1 or 0}", "0")
	script := "return cjson.decode('[1,]')"
	h.expect(scriptErr("ERR user_script:1: Expected value but found T_ARR_END at character 4", script, 1), "EVAL", script, "0")
	h.expect("$4\r\n\x93\x01\x02\x03\r\n", "EVAL", "return cmsgpack.pack({1,2,3})", "0")
	h.expect("*4\r\n:1\r\n$1\r\na\r\n:2\r\n:5\r\n",
		"EVAL", "local a, b, c, d = cmsgpack.unpack(cmsgpack.pack(1,'a',{1,2},{x=5})) return {a, b, c[2], d.x}", "0")
	h.expect(":15\r\n", "EVAL", "return bit.band(0xff, 0x0f)", "0")
	h.expect(":-2147483648\r\n", "EVAL", "return bit.lshift(1, 31)", "0")
	h.expect("$8\r\n000000ff\r\n", "EVAL", "return bit.tohex(255)", "0")
}
