package server

import (
	"bytes"
	"testing"

	"github.com/cachestorm/cachestorm/internal/command"
	"github.com/cachestorm/cachestorm/internal/config"
	"github.com/cachestorm/cachestorm/internal/resp"
	"github.com/cachestorm/cachestorm/internal/store"
)

// aofSession drives a server the way one connection does: the selected
// database and the transaction carry over from command to command.
type aofSession struct {
	t  *testing.T
	s  *Server
	db int
	tx *command.Transaction
}

func (c *aofSession) do(args ...string) string {
	c.t.Helper()
	buf := &bytes.Buffer{}
	raw := make([][]byte, len(args)-1)
	for i, a := range args[1:] {
		raw[i] = []byte(a)
	}
	ctx := command.NewContext(args[0], raw, c.s.dbs.DB(c.db), resp.NewWriter(buf))
	ctx.DB = c.db
	ctx.DBs = c.s.dbs
	ctx.Transaction = c.tx
	if err := c.s.router.Execute(ctx); err != nil {
		c.t.Fatalf("%v: %v", args, err)
	}
	c.db = ctx.DB
	return buf.String()
}

func newMultiDBAOFServer(t *testing.T, dir string) *Server {
	t.Helper()
	cfg := &config.Config{}
	cfg.Persistence.Enabled = true
	cfg.Persistence.AOF = true
	cfg.Persistence.DataDir = dir
	cfg.Persistence.AOFSync = "always"
	s, err := newOwnedTestServer(t, cfg)
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	if err := s.aof.Start(); err != nil {
		t.Fatalf("start AOF: %v", err)
	}
	return s
}

func getString(t *testing.T, s *store.Store, key string) string {
	t.Helper()
	e, ok := s.Get(key)
	if !ok {
		return "<missing>"
	}
	sv, ok := e.Value.(*store.StringValue)
	if !ok {
		t.Fatalf("%s is not a string", key)
	}
	return string(sv.Data)
}

// A restart from the AOF must put every key back in the database it was
// written to, including writes made inside a transaction that switched
// database with SELECT.
func TestAOFRestartRestoresEveryDatabase(t *testing.T) {
	dir := t.TempDir()
	s := newMultiDBAOFServer(t, dir)
	c := &aofSession{t: t, s: s, tx: command.NewTransaction()}

	c.do("SET", "a", "db0")
	c.do("SELECT", "3")
	c.do("SET", "b", "db3")
	c.do("HSET", "h", "f", "v")
	c.do("MULTI")
	c.do("SET", "c", "db3-tx")
	c.do("SELECT", "0")
	c.do("SET", "d", "db0-tx")
	c.do("SELECT", "3")
	c.do("SET", "e", "db3-tx")
	if got := c.do("EXEC"); got[0] != '*' {
		t.Fatalf("EXEC: %q", got)
	}
	c.do("MOVE", "e", "5")
	c.do("SET", "f", "db3")
	stopReplServer(s)

	s2 := newMultiDBAOFServer(t, dir)
	defer stopReplServer(s2)
	db0, db3, db5 := s2.dbs.DB(0), s2.dbs.DB(3), s2.dbs.DB(5)
	checks := []struct {
		db   *store.Store
		key  string
		want string
	}{
		{db0, "a", "db0"}, {db0, "d", "db0-tx"}, {db0, "b", "<missing>"}, {db0, "f", "<missing>"},
		{db3, "b", "db3"}, {db3, "c", "db3-tx"}, {db3, "f", "db3"}, {db3, "e", "<missing>"}, {db3, "a", "<missing>"},
		{db5, "e", "db3-tx"},
	}
	for _, ck := range checks {
		if got := getString(t, ck.db, ck.key); got != ck.want {
			t.Errorf("%s: got %q want %q", ck.key, got, ck.want)
		}
	}
	if !db3.Exists("h") {
		t.Error("hash written in db 3 must be restored there")
	}

	// The writer continues in the database the replay ended in, so a write
	// to database 0 after the restart gets a SELECT 0 first.
	c2 := &aofSession{t: t, s: s2, tx: command.NewTransaction()}
	c2.do("SET", "g", "db0-after")
	stopReplServer(s2)
	s3 := newMultiDBAOFServer(t, dir)
	defer stopReplServer(s3)
	if got := getString(t, s3.dbs.DB(0), "g"); got != "db0-after" {
		t.Fatalf("write after restart: got %q", got)
	}
	if s3.dbs.DB(3).Exists("g") {
		t.Fatal("a db 0 write after a restart that ended in db 3 must not land in db 3")
	}
}
