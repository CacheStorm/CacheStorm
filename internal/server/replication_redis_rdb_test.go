package server

import (
	"bufio"
	"io"
	"net"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/cachestorm/cachestorm/internal/persistence"
	"github.com/cachestorm/cachestorm/internal/store"
)

// A replica that does not announce capa cachestorm, such as Redis or
// redis-cli --rdb, gets the full resynchronization as an RDB in Redis'
// format, naming the database the stream continues in, then the stream.
func TestRedisReplicaGetsAnRDB(t *testing.T) {
	master := startReplServer(t, nil)
	mc := dialRepl(t, master)
	mc.do("SET", "k", "v")
	mc.do("RPUSH", "l", "a", "b")
	mc.do("SELECT", "3")
	mc.do("SET", "in3", "x")

	conn, err := net.DialTimeout("tcp", "127.0.0.1:"+strconv.Itoa(master.testPort()), 2*time.Second)
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()
	_ = conn.SetDeadline(time.Now().Add(10 * time.Second))
	br := bufio.NewReader(conn)
	line := func() string {
		s, err := br.ReadString('\n')
		if err != nil {
			t.Fatal(err)
		}
		return strings.TrimRight(s, "\r\n")
	}
	conn.Write(encodeArgs("REPLCONF", "capa", "eof", "capa", "psync2"))
	if got := line(); got != "+OK" {
		t.Fatalf("REPLCONF = %s", got)
	}
	conn.Write(encodeArgs("PSYNC", "?", "-1"))
	if got := line(); !strings.HasPrefix(got, "+FULLRESYNC ") {
		t.Fatalf("PSYNC = %s", got)
	}
	size, _ := strconv.Atoi(strings.TrimPrefix(line(), "$"))
	rdb := make([]byte, size)
	if _, err := io.ReadFull(br, rdb); err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(string(rdb), "REDIS0011") {
		t.Fatalf("snapshot starts %q, want a Redis RDB", rdb[:min(len(rdb), 9)])
	}
	dbs := store.NewDatabases(store.NewStore(), 16)
	r := persistence.NewRDBReaderForDatabases(dbs)
	if err := r.LoadBytes(rdb); err != nil {
		t.Fatal(err)
	}
	if _, ok := dbs.DB(0).Get("l"); !ok {
		t.Fatal("list missing from the RDB")
	}
	if _, ok := dbs.DB(3).Get("in3"); !ok {
		t.Fatal("database 3 missing from the RDB")
	}
	// The stream had no database yet, so the RDB names 0 and the stream
	// selects the client's database before its first write.
	if db := r.AuxFields["repl-stream-db"]; db != "0" {
		t.Fatalf("repl-stream-db = %q, want 0", db)
	}
	mc.do("SET", "after", "1")
	var stream []string
	for len(stream) == 0 || stream[len(stream)-1] != "after" {
		stream = append(stream, line())
	}
	if got := strings.Join(stream, " "); !strings.Contains(got, "SELECT $1 3") {
		t.Fatalf("stream %q does not select database 3 before the write", got)
	}
}
