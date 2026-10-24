package command

import (
	"strings"
	"testing"
)

// FUNCTION LOAD / FCALL with Redis 7.4 semantics; the expected replies were
// taken from a Redis 7.4 server (except the DUMP payload, which is opaque).
func TestLuaFunctions(t *testing.T) {
	h := newLuaHarness(t)
	lib := "#!lua name=mylib\nredis.register_function('myfunc', function(keys, args) return args[1] end)\n" +
		"redis.register_function{function_name='ro', callback=function(keys) return redis.call('GET', keys[1]) end, flags={'no-writes'}, description='reads'}\n" +
		"redis.register_function('w', function(keys) return redis.call('SET', keys[1], 'x') end)\n" +
		"redis.register_function('e', function(keys) return redis.call('INCR', keys[1]) end)"
	h.expect("$5\r\nmylib\r\n", "FUNCTION", "LOAD", lib)
	h.expect("-ERR Library 'mylib' already exists\r\n", "FUNCTION", "LOAD", lib)
	h.expect("$5\r\nhello\r\n", "FCALL", "myfunc", "0", "hello")
	h.expect("-ERR Function not found\r\n", "FCALL", "nofunc", "0")
	h.expect("-ERR Bad number of keys provided\r\n", "FCALL", "myfunc", "x")
	h.expect("-ERR Can not execute a script with write flag using *_ro command.\r\n", "FCALL_RO", "myfunc", "0")
	h.expect("+OK\r\n", "FCALL", "w", "1", "k")
	h.expect("$1\r\nx\r\n", "FCALL_RO", "ro", "1", "k")
	h.expect("-ERR value is not an integer or out of range script: e, on @user_function:5.\r\n", "FCALL", "e", "1", "k")
	h.expect("*1\r\n*6\r\n$12\r\nlibrary_name\r\n$5\r\nmylib\r\n$6\r\nengine\r\n$3\r\nLUA\r\n$9\r\nfunctions\r\n*4\r\n"+
		"*6\r\n$4\r\nname\r\n$1\r\ne\r\n$11\r\ndescription\r\n$-1\r\n$5\r\nflags\r\n*0\r\n"+
		"*6\r\n$4\r\nname\r\n$6\r\nmyfunc\r\n$11\r\ndescription\r\n$-1\r\n$5\r\nflags\r\n*0\r\n"+
		"*6\r\n$4\r\nname\r\n$2\r\nro\r\n$11\r\ndescription\r\n$5\r\nreads\r\n$5\r\nflags\r\n*1\r\n+no-writes\r\n"+
		"*6\r\n$4\r\nname\r\n$1\r\nw\r\n$11\r\ndescription\r\n$-1\r\n$5\r\nflags\r\n*0\r\n",
		"FUNCTION", "LIST")

	h.expect("-ERR Missing library metadata\r\n", "FUNCTION", "LOAD", "return 1")
	h.expect("-ERR No functions registered\r\n", "FUNCTION", "LOAD", "#!lua name=lib2\nreturn 1")
	h.expect("-ERR Function myfunc already exists\r\n", "FUNCTION", "LOAD", "#!lua name=lib2\nredis.register_function('myfunc', function() return 1 end)")
	h.expect("-ERR Error registering functions: ERR user_function:2: Script attempted to access nonexistent global variable 'call'\r\n",
		"FUNCTION", "LOAD", "#!lua name=lib2\nredis.call('SET','x','1')")

	dump := h.do("FUNCTION", "DUMP")
	payload := dump[strings.Index(dump, "\r\n")+2 : len(dump)-2]
	h.expect("+OK\r\n", "FUNCTION", "FLUSH")
	h.expect("-ERR Function not found\r\n", "FCALL", "myfunc", "0")
	h.expect("+OK\r\n", "FUNCTION", "RESTORE", payload)
	h.expect("$2\r\nhi\r\n", "FCALL", "myfunc", "0", "hi")
	h.expect("-ERR Library mylib already exists\r\n", "FUNCTION", "RESTORE", payload)
	h.expect("-ERR DUMP payload version or checksum are wrong\r\n", "FUNCTION", "RESTORE", "garbage")

	h.expect("+OK\r\n", "FUNCTION", "DELETE", "mylib")
	h.expect("-ERR Library not found\r\n", "FUNCTION", "DELETE", "mylib")
	h.expect("-ERR Function not found\r\n", "FCALL", "myfunc", "0")
}
