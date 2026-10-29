package command

import (
	"strconv"
	"strings"
	"testing"
)

// TestLua51Semantics pins script behavior that differs between gopher-lua and
// the Lua 5.1 Redis embeds. The expected replies were taken from Redis 7.4.
func TestLua51Semantics(t *testing.T) {
	h := newLuaHarness(t)
	bulk := func(s string) string { return "$" + strconv.Itoa(len(s)) + "\r\n" + s + "\r\n" }
	for _, c := range []struct {
		script, want string
	}{
		{"return tostring(0.1+0.2)", bulk("0.3")},
		{"return tostring(1/3)", bulk("0.33333333333333")},
		{"return 'x' .. 1e15", bulk("x1e+15")},
		{"return tostring(math.huge) .. tostring(-math.huge)", bulk("inf-inf")},
		{"return tostring(-(0/0))", bulk("nan")},
		{"return tostring(tonumber('1e2'))", bulk("100")},
		{"return tostring(tonumber('  0x10  '))", bulk("16")},
		{"return tostring(tonumber('010'))", bulk("10")},
		{"return tostring(tonumber('z', 36))", bulk("35")},
		{"return tostring(tonumber('12a'))", bulk("nil")},
		{"return tostring('10' + 5)", bulk("15")},
		{"return string.format('%5.2f|%d|%x|%g|%e|%i|%c', 3.14159, 42, 255, 0.1, 12345.678, 7, 65)", bulk(" 3.14|42|ff|0.1|1.234568e+04|7|A")},
		{"return ('%x'):format(-1)", bulk("ffffffffffffffff")},
		{"return ('%q'):format('a\\0b\\r\\\\')", bulk(`"a\000b\r\\"`)},
		{"return ('%c%c'):format(0, 255)", bulk("\xff")},
		{"return ('%5.1s|'):format('abc')", bulk("    a|")},
		{"return struct.pack('>I', 1)", bulk("\x00\x00\x00\x01")},
		{"return struct.pack('<i2 h b B', -2, -3, -1, 255)", bulk("\xfe\xff\xfd\xff\xff\xff")},
		{"return {struct.unpack('b c0', struct.pack('b', 3) .. 'xyz')}", "*2\r\n$3\r\nxyz\r\n:5\r\n"},
		{"return struct.size('!4 b i d c3')", ":19\r\n"},
		{"math.randomseed(1) return {tostring(math.random()), math.random(1,1000000), math.random(7)}", "*3\r\n" + bulk("0.041630344484761") + ":454493\r\n:6\r\n"},
		{"return redis.acl_check_cmd('set', 'k', 'v')", ":1\r\n"},
		{"local t = {} t.x = 1 return t.x", ":1\r\n"},
		{"KEYS[1] = 'z' return KEYS[1]", bulk("z")},
	} {
		if got := h.do("EVAL", c.script, "0"); got != c.want {
			t.Errorf("%s\n got: %q\nwant: %q", c.script, got, c.want)
		}
	}
	for _, c := range []struct {
		script, want string
	}{
		{"return ('abc'):rep(1e10)", "ERR user_script:1: resulting string too large"},
		{"return string.format('%d', 'x')", "ERR user_script:1: bad argument #2 to 'format' (number expected, got string)"},
		{"return string.format('%s %s', 1)", "ERR user_script:1: bad argument #3 to 'format' (no value)"},
		{"return string.format('%z', 1)", "ERR user_script:1: invalid option '%z' to 'format'"},
		{"return string.rep()", "ERR user_script:1: bad argument #1 to 'rep' (string expected, got no value)"},
		{"return string.char(256)", "ERR user_script:1: bad argument #1 to 'char' (invalid value)"},
		{"return struct.pack('z', 1)", "ERR user_script:1: bad argument #1 to 'pack' (invalid format option 'z')"},
		{"return struct.size('s')", "ERR user_script:1: bad argument #1 to 'size' (option 's' has no fixed size)"},
		{"return redis.acl_check_cmd('nosuch')", "ERR Invalid command passed to redis.acl_check_cmd()"},
		{"return redis.acl_check_cmd('get')", "ERR Wrong number of args for redis.acl_check_cmd()"},
		{"return table.concat({1,2,{}})", "ERR user_script:1: invalid value (table) at index 3 in table for 'concat'"},
		{"local x = {} return 'a' .. x", "ERR user_script:1: attempt to concatenate local 'x' (a table value)"},
		{"local t = {} t.x.y = 1", "ERR user_script:1: attempt to index field 'x' (a nil value)"},
		{"for i = 1, 3 do local t = nil t.x = i end", "ERR user_script:1: attempt to index local 't' (a nil value)"},
		{"local s = 'x' return s:bad()", "ERR user_script:1: attempt to call method 'bad' (a nil value)"},
		{"return 1 + {}", "ERR user_script:1: attempt to perform arithmetic on a table value"},
		{"return #5", "ERR user_script:1: attempt to get length of a number value"},
		{"return {} < {}", "ERR user_script:1: attempt to compare two table values"},
		{"local t = {1,'x'} table.sort(t) return t", "ERR attempt to compare string with number"},
		{"error(42)", "ERR user_script:1: 42"},
		{"redis.call = nil", "ERR user_script:1: Attempt to modify a readonly table"},
		{"rawset(string, 'x', 1)", "ERR Attempt to modify a readonly table"},
		{"setmetatable(string, {})", "ERR Attempt to modify a readonly table"},
		{"table.insert(string, 1)", "ERR Attempt to modify a readonly table"},
	} {
		if got := h.do("EVAL", c.script, "0"); !strings.HasPrefix(got, "-"+c.want+" script: ") {
			t.Errorf("%s\n got: %q\nwant: -%s script: ...", c.script, got, c.want)
		}
	}
	for _, c := range []struct {
		script, want string
	}{
		{"return tostring(select(2, pcall(string.rep)))", "bad argument #1 to '?' (string expected, got no value)"},
		{"return tostring(select(2, pcall(error, 'msg')))", "msg"},
		{"return tostring(select(2, pcall(next, {}, 'nokey')))", "invalid key to 'next'"},
		{"return tostring(select(2, pcall(setmetatable, 1, {})))", "bad argument #1 to '?' (table expected, got number)"},
		{"return tostring(select(2, pcall(table.insert, {}, 1, 2, 3)))", "wrong number of arguments to 'insert'"},
	} {
		if got := h.do("EVAL", c.script, "0"); got != bulk(c.want) {
			t.Errorf("%s\n got: %q\nwant: %q", c.script, got, bulk(c.want))
		}
	}

	h.do("FUNCTION", "LOAD", "#!lua name=lib\nredis.register_function('f', function() string.upper = nil return 1 end)")
	if got := h.do("FCALL", "f", "0"); !strings.HasPrefix(got, "-ERR user_function:2: Attempt to modify a readonly table") {
		t.Errorf("FCALL writing a library table: %q", got)
	}
}
