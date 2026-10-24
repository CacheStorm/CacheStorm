package command

import (
	"bytes"
	"strings"
	"testing"
	"time"

	"github.com/cachestorm/cachestorm/internal/resp"
	"github.com/cachestorm/cachestorm/internal/store"
)

// dbSession is one client: it keeps its selected database and transaction
// across commands the way a server connection does.
type dbSession struct {
	t   *testing.T
	r   *Router
	dbs *store.Databases
	db  int
	tx  *Transaction
}

func newDBSession(t *testing.T, r *Router, dbs *store.Databases) *dbSession {
	return &dbSession{t: t, r: r, dbs: dbs, tx: NewTransaction()}
}

func (c *dbSession) do(args ...string) string {
	c.t.Helper()
	buf := &bytes.Buffer{}
	raw := make([][]byte, len(args)-1)
	for i, a := range args[1:] {
		raw[i] = []byte(a)
	}
	ctx := NewContext(args[0], raw, c.dbs.DB(c.db), resp.NewWriter(buf))
	ctx.DB = c.db
	ctx.DBs = c.dbs
	ctx.Transaction = c.tx
	if err := c.r.Execute(ctx); err != nil {
		ctx.Writer.WriteError(err.Error())
	}
	c.db = ctx.DB
	return buf.String()
}

func newDBRouter() *Router {
	r := NewRouter()
	RegisterStringCommands(r)
	RegisterServerCommands(r)
	RegisterKeyCommands(r)
	RegisterHashCommands(r)
	RegisterNamespaceCommands(r)
	RegisterTransactionCommands(r)
	RegisterScriptCommands(r)
	return r
}

func expectReply(t *testing.T, got, want string) {
	t.Helper()
	if got != want {
		t.Fatalf("got %q, want %q", got, want)
	}
}

func TestSelectIsPerConnection(t *testing.T) {
	dbs := store.NewDatabases(store.NewStore(), 16)
	r := newDBRouter()
	a, b := newDBSession(t, r, dbs), newDBSession(t, r, dbs)

	expectReply(t, a.do("SELECT", "1"), "+OK\r\n")
	a.do("SET", "k", "db1")
	expectReply(t, b.do("GET", "k"), "$-1\r\n")
	b.do("SET", "k", "db0")
	expectReply(t, a.do("GET", "k"), "$3\r\ndb1\r\n")
	expectReply(t, a.do("DBSIZE"), ":1\r\n")

	expectReply(t, a.do("SELECT", "16"), "-ERR DB index is out of range\r\n")
	expectReply(t, a.do("SELECT", "-1"), "-ERR DB index is out of range\r\n")
	expectReply(t, a.do("SELECT", "abc"), "-ERR value is not an integer or out of range\r\n")
	expectReply(t, a.do("GET", "k"), "$3\r\ndb1\r\n")
}

func TestMoveCopySwapFlush(t *testing.T) {
	dbs := store.NewDatabases(store.NewStore(), 16)
	c := newDBSession(t, newDBRouter(), dbs)

	c.do("SET", "a", "1")
	c.do("PEXPIRE", "a", "100000")
	expectReply(t, c.do("MOVE", "a", "1"), ":1\r\n")
	expectReply(t, c.do("EXISTS", "a"), ":0\r\n")
	expectReply(t, c.do("MOVE", "a", "1"), ":0\r\n")
	expectReply(t, c.do("MOVE", "a", "0"), "-ERR source and destination objects are the same\r\n")
	expectReply(t, c.do("MOVE", "a", "16"), "-ERR DB index is out of range\r\n")
	if ttl := dbs.DB(1).GetTTL("a"); ttl <= 0 {
		t.Fatalf("MOVE must keep the TTL, got %v", ttl)
	}

	c.do("SET", "x", "0")
	dbs.DB(1).Set("x", &store.StringValue{Data: []byte("1")}, store.SetOptions{})
	expectReply(t, c.do("MOVE", "x", "1"), ":0\r\n")

	c.do("SET", "s", "v", "EX", "100")
	expectReply(t, c.do("COPY", "s", "s", "DB", "2"), ":1\r\n")
	expectReply(t, c.do("COPY", "s", "s", "DB", "2"), ":0\r\n")
	expectReply(t, c.do("COPY", "s", "s", "DB", "2", "REPLACE"), ":1\r\n")
	expectReply(t, c.do("COPY", "s", "s"), "-ERR source and destination objects are the same\r\n")
	expectReply(t, c.do("COPY", "s", "t", "DB"), "-ERR syntax error\r\n")
	expectReply(t, c.do("COPY", "s", "t", "BOGUS"), "-ERR syntax error\r\n")
	if ttl := dbs.DB(2).GetTTL("s"); ttl <= 0 {
		t.Fatalf("COPY ... DB must keep the TTL, got %v", ttl)
	}

	expectReply(t, c.do("SWAPDB", "0", "2"), "+OK\r\n")
	expectReply(t, c.do("DBSIZE"), ":1\r\n")
	expectReply(t, c.do("GET", "x"), "$-1\r\n")
	expectReply(t, c.do("SWAPDB", "0", "x"), "-ERR invalid second DB index\r\n")
	expectReply(t, c.do("SWAPDB", "0", "99"), "-ERR DB index is out of range\r\n")

	info := c.do("INFO")
	if !strings.Contains(info, "db0:keys=1,expires=1,") || !strings.Contains(info, "db1:keys=2,expires=1,") || !strings.Contains(info, "db2:keys=2,expires=1,") {
		t.Fatalf("INFO keyspace must list every non-empty database: %q", info)
	}

	expectReply(t, c.do("FLUSHDB"), "+OK\r\n")
	expectReply(t, c.do("DBSIZE"), ":0\r\n")
	if dbs.KeyCount() == 0 {
		t.Fatal("FLUSHDB must only empty the selected database")
	}
	expectReply(t, c.do("FLUSHALL", "BOGUS"), "-ERR syntax error\r\n")
	expectReply(t, c.do("FLUSHALL", "ASYNC"), "+OK\r\n")
	if dbs.KeyCount() != 0 {
		t.Fatal("FLUSHALL must empty every database")
	}
}

func TestSelectInsideMultiAppliesToQueuedCommands(t *testing.T) {
	dbs := store.NewDatabases(store.NewStore(), 16)
	r := newDBRouter()
	var executed []ExecutedCommand
	r.SetPostExec(func(cmds []ExecutedCommand) { executed = cmds })
	c := newDBSession(t, r, dbs)

	c.do("MULTI")
	c.do("SET", "a", "0")
	c.do("SELECT", "3")
	c.do("SET", "a", "3")
	c.do("EXEC")
	if c.db != 3 {
		t.Fatalf("SELECT inside MULTI must stay selected after EXEC, db=%d", c.db)
	}
	expectReply(t, c.do("GET", "a"), "$1\r\n3\r\n")
	if e, ok := dbs.DB(0).Get("a"); !ok || string(e.Value.(*store.StringValue).Data) != "0" {
		t.Fatal("the command queued before SELECT must run in database 0")
	}
	if len(executed) != 3 || executed[0].DB != 0 || executed[2].DB != 3 {
		t.Fatalf("EXEC must report the database each command ran in: %+v", executed)
	}
}

func TestWatchIsPerDatabaseAndSeesEveryWrite(t *testing.T) {
	dbs := store.NewDatabases(store.NewStore(), 16)
	r := newDBRouter()
	a, b := newDBSession(t, r, dbs), newDBSession(t, r, dbs)

	run := func(touch ...string) string {
		t.Helper()
		a.do("WATCH", "k")
		if len(touch) > 0 {
			b.do(touch...)
		}
		a.do("MULTI")
		a.do("PING")
		return a.do("EXEC")
	}
	const aborted = "*-1\r\n"
	const committed = "*1\r\n+PONG\r\n"

	b.do("SET", "k", "v")
	expectReply(t, run(), committed)
	expectReply(t, run("EXPIRE", "k", "100"), aborted)
	expectReply(t, run("PERSIST", "k"), aborted)
	expectReply(t, run("APPEND", "k", "x"), aborted)
	expectReply(t, run("DEL", "k"), aborted)
	expectReply(t, run("SET", "k", "v"), aborted)
	expectReply(t, run("FLUSHALL"), aborted)
	b.do("HSET", "k", "f", "v")
	expectReply(t, run("HSET", "k", "f", "v2"), aborted)
	expectReply(t, run("MOVE", "k", "1"), aborted)

	// A write to the same key name in another database does not abort.
	b.do("SELECT", "1")
	expectReply(t, run("SET", "k", "other"), committed)

	// The watch stays on the database it was taken in.
	a.do("WATCH", "k")
	a.do("SELECT", "2")
	b.do("SELECT", "0")
	b.do("SET", "k", "x")
	a.do("MULTI")
	a.do("PING")
	expectReply(t, a.do("EXEC"), aborted)

	a.tx.ClearWatch()
	dbs.ForEach(func(_ int, s *store.Store) {
		if s.HasWatchedKeys() {
			t.Fatal("EXEC must release its watches")
		}
	})
}

func TestScriptRunsInCallerDatabaseAndSelectIsLocal(t *testing.T) {
	dbs := store.NewDatabases(store.NewStore(), 16)
	c := newDBSession(t, newDBRouter(), dbs)

	c.do("SELECT", "2")
	c.do("EVAL", "redis.call('SET', 'k', 'v') return 1", "0")
	if !dbs.DB(2).Exists("k") || dbs.DB(0).Exists("k") {
		t.Fatal("a script must run in the caller's database")
	}
	c.do("EVAL", "redis.call('SELECT', 5) redis.call('SET', 'j', 'v') return 1", "0")
	if !dbs.DB(5).Exists("j") || c.db != 2 {
		t.Fatalf("SELECT inside a script must switch only the script's database (db=%d)", c.db)
	}
}

func TestWatchAbortsOnExpiry(t *testing.T) {
	dbs := store.NewDatabases(store.NewStore(), 16)
	c := newDBSession(t, newDBRouter(), dbs)
	c.do("SET", "k", "v", "PX", "20")
	c.do("WATCH", "k")
	time.Sleep(40 * time.Millisecond)
	c.do("GET", "k")
	c.do("MULTI")
	c.do("PING")
	expectReply(t, c.do("EXEC"), "*-1\r\n")
}
