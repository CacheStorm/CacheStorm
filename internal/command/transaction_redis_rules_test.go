package command

import (
	"strings"
	"testing"

	"github.com/cachestorm/cachestorm/internal/store"
)

func TestTransactionQueueChecksMatchRedis(t *testing.T) {
	r := newMemAcctRouter()
	s := store.NewStore()
	tx := NewTransaction()
	run := func(args ...string) string { return memAcctRun(r, s, tx, args...) }
	for _, c := range []struct {
		args []string
		want string
	}{
		{[]string{"MULTI"}, "+OK\r\n"},
		{[]string{"UNWATCH"}, "+QUEUED\r\n"},
		{[]string{"SET", "a", "1"}, "+QUEUED\r\n"},
		{[]string{"EXEC"}, "*2\r\n+OK\r\n+OK\r\n"},
		{[]string{"MULTI"}, "+OK\r\n"},
		{[]string{"XGROUP", "CREATECONSUMER", "s", "g"}, "-ERR wrong number of arguments for 'xgroup|createconsumer' command\r\n"},
		{[]string{"EXEC"}, "-EXECABORT Transaction discarded because of previous errors.\r\n"},
		{[]string{"MULTI"}, "+OK\r\n"},
		{[]string{"XINFO", "NOSUCH"}, "-ERR unknown subcommand 'NOSUCH'. Try XINFO HELP.\r\n"},
		{[]string{"EXEC"}, "-EXECABORT Transaction discarded because of previous errors.\r\n"},
		{[]string{"MULTI"}, "+OK\r\n"},
		{[]string{"SET", "a", "2"}, "+QUEUED\r\n"},
		{[]string{"EXEC", "x"}, "-EXECABORT Transaction discarded because of: wrong number of arguments for 'exec' command\r\n"},
		{[]string{"EXEC"}, "-ERR EXEC without MULTI\r\n"},
		{[]string{"GET", "a"}, "$1\r\n1\r\n"},
	} {
		if got := run(c.args...); got != c.want {
			t.Fatalf("%v: got %q, want %q", c.args, got, c.want)
		}
	}
}

func TestWatchIgnoresWritesThatChangeNothing(t *testing.T) {
	r := newMemAcctRouter()
	RegisterHyperLogLogCommands(r)
	cases := []struct {
		setup []string
		cmd   string
		abort bool
	}{
		{[]string{"SADD w a"}, "SREM w zz", false},
		{[]string{"SADD w a"}, "SREM w a", true},
		{[]string{"SET w 5"}, "SREM w zz", false},
		{nil, "SREM w zz", false},
		{nil, "LPOP w", false},
		{nil, "SET w x NX", true},
		{[]string{"SET w 5"}, "SET w x NX", false},
		{[]string{"SET w 5"}, "EXPIRE w 100 XX", false},
		{[]string{"SET w 5"}, "SETRANGE w 0 ''", false},
		{[]string{"RPUSH w a"}, "LMOVE nokey w LEFT LEFT", false},
		{[]string{"RPUSH w a"}, "LREM w 0 zz", false},
		{[]string{"ZADD w 1 a"}, "ZADD w XX 1 nope", false},
		{[]string{"ZADD w 1 a"}, "ZADD w 2 a", true},
		{[]string{"ZADD w 1 a"}, "ZREMRANGEBYRANK w 5 6", false},
		{[]string{"HSET w f v"}, "HDEL w nope", false},
		{[]string{"XADD w 1-1 f v", "XGROUP CREATE w g 0"}, "XREADGROUP GROUP g c STREAMS w >", false},
		{[]string{"XADD w 1-1 f v"}, "XDEL w 9-9", false},
		{[]string{"XADD w 1-1 f v"}, "XDEL w 1-1", true},
		{[]string{"XADD w 1-1 f v"}, "XTRIM w MAXLEN 10", false},
		{[]string{"XADD w 1-1 f v"}, "XADD w 2-1 f v", true},
	}
	for _, c := range cases {
		s := store.NewStore()
		tx := NewTransaction()
		run := func(line string) string {
			args := strings.Fields(line)
			for i, a := range args {
				if a == "''" {
					args[i] = ""
				}
			}
			return memAcctRun(r, s, tx, args...)
		}
		for _, line := range c.setup {
			run(line)
		}
		run("WATCH w")
		run(c.cmd)
		run("MULTI")
		got := run("EXEC")
		if aborted := got == "*-1\r\n"; aborted != c.abort {
			t.Errorf("%v then %q: EXEC replied %q, want abort=%v", c.setup, c.cmd, got, c.abort)
		}
	}
}
