package replication

import (
	"bufio"
	"bytes"
	"io"
	"net"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/cachestorm/cachestorm/internal/command"
	"github.com/cachestorm/cachestorm/internal/persistence"
	"github.com/cachestorm/cachestorm/internal/store"
)

type fakeHost struct {
	dbs           *store.Databases
	router        *command.Router
	mu            sync.Mutex
	fullSyncs     int
	functionError error
	persistError  error
}

func (h *fakeHost) LockWrites()   { h.mu.Lock() }
func (h *fakeHost) UnlockWrites() { h.mu.Unlock() }
func (h *fakeHost) CommitFullSync(dbs *store.Databases, replay *command.Router) error {
	h.fullSyncs++
	if h.persistError != nil {
		return h.persistError
	}
	if err := h.dbs.ReplaceFrom(dbs); err != nil {
		return err
	}
	h.router.SetFunctionRegistry(replay.FunctionRegistry().CloneForStore(h.dbs.DB(0)))
	return nil
}

func (h *fakeHost) FunctionPayload() ([]byte, error) { return nil, h.functionError }
func (h *fakeHost) SearchCommands() [][]string       { return nil }
func (h *fakeHost) ListeningPort() int               { return 1234 }

func newTestManager(t *testing.T, cfg Config) (*Manager, *store.Databases) {
	t.Helper()
	dbs := store.NewDatabases(store.NewStore(), 4)
	r := command.NewRouter()
	command.RegisterStringCommands(r)
	command.RegisterServerCommands(r)
	command.RegisterKeyCommands(r)
	command.RegisterListCommands(r)
	command.RegisterTransactionCommands(r)
	command.RegisterNamespaceCommands(r)
	m := New(cfg, &fakeHost{dbs: dbs, router: r}, r, dbs)
	m.RegisterCommands(r)
	t.Cleanup(m.Stop)
	return m, dbs
}

func TestReadCommandReturnsArgsAndRawBytes(t *testing.T) {
	stream := append(encodeCommand("SET", "k", "v\r\nwith crlf"), encodeCommand("PING")...)
	br := bufio.NewReader(bytes.NewReader(stream))
	args, raw, err := readCommand(br)
	if err != nil {
		t.Fatal(err)
	}
	if len(args) != 3 || string(args[0]) != "SET" || string(args[2]) != "v\r\nwith crlf" {
		t.Fatalf("args %q", args)
	}
	if !bytes.Equal(raw, encodeCommand("SET", "k", "v\r\nwith crlf")) {
		t.Fatalf("raw %q", raw)
	}
	args, _, err = readCommand(br)
	if err != nil || string(args[0]) != "PING" {
		t.Fatalf("second command %q %v", args, err)
	}
	if _, _, err := readCommand(br); err != io.EOF {
		t.Fatalf("end of stream: %v", err)
	}
}

func TestReadCommandRejectsMalformedInput(t *testing.T) {
	for _, in := range []string{
		"+OK\r\n",
		"*0\r\n",
		"*1\r\n:1\r\n",
		"*1\r\n$-5\r\n",
		"*1\r\n$3\r\nabcXY",
		"*x\r\n",
	} {
		if _, _, err := readCommand(bufio.NewReader(strings.NewReader(in))); err == nil || err == io.EOF {
			t.Errorf("%q: got %v, want a protocol error", in, err)
		}
	}
}

func TestParseReplies(t *testing.T) {
	id, off, err := parseFullResync("+FULLRESYNC abc 42")
	if err != nil || id != "abc" || off != 42 {
		t.Fatalf("FULLRESYNC: %s %d %v", id, off, err)
	}
	if _, _, err := parseFullResync("+FULLRESYNC abc"); err == nil {
		t.Fatal("FULLRESYNC without an offset was accepted")
	}
	if got := parseContinue("+CONTINUE newid"); got != "newid" {
		t.Fatalf("CONTINUE id %q", got)
	}
	if got := parseContinue("+CONTINUE"); got != "" {
		t.Fatalf("bare CONTINUE id %q", got)
	}
	if n, err := parseBulkHeader("$123"); err != nil || n != 123 {
		t.Fatalf("bulk header %d %v", n, err)
	}
	if _, err := parseBulkHeader("+OK"); err == nil {
		t.Fatal("bulk header accepted +OK")
	}
}

func TestBacklogRingWrapsAndServesRanges(t *testing.T) {
	m, _ := newTestManager(t, Config{BacklogSize: 16})
	m.mu.Lock()
	defer m.mu.Unlock()
	m.createBacklogLocked()
	var all []byte
	for i := 0; i < 7; i++ {
		chunk := []byte(strings.Repeat(string(rune('a'+i)), 5))
		all = append(all, chunk...)
		m.appendLocked(chunk)
	}
	if m.offset != 35 || m.histLen != 16 {
		t.Fatalf("offset %d histlen %d", m.offset, m.histLen)
	}
	for from := int64(19); from <= 35; from++ {
		got, ok := m.backlogFromLocked(from)
		if !ok || !bytes.Equal(got, all[from:]) {
			t.Fatalf("from %d: %q %v, want %q", from, got, ok, all[from:])
		}
	}
	if _, ok := m.backlogFromLocked(18); ok {
		t.Fatal("served a position that left the backlog")
	}
	if _, ok := m.backlogFromLocked(36); ok {
		t.Fatal("served a position past the end of the stream")
	}
	// A write larger than the ring keeps its tail.
	big := []byte(strings.Repeat("x", 20) + "0123456789abcdef")
	m.appendLocked(big)
	got, ok := m.backlogFromLocked(m.offset - 16)
	if !ok || string(got) != "0123456789abcdef" {
		t.Fatalf("after an oversized write: %q %v", got, ok)
	}
}

func TestContinuableChecksIDAndRange(t *testing.T) {
	m, _ := newTestManager(t, Config{BacklogSize: 64})
	m.mu.Lock()
	defer m.mu.Unlock()
	m.createBacklogLocked()
	m.appendLocked([]byte("0123456789"))
	if _, ok := m.continuableLocked(syncRequest{replid: m.replid, offset: 11}); !ok {
		t.Fatal("a replica at the end of the stream could not continue")
	}
	if data, ok := m.continuableLocked(syncRequest{replid: m.replid, offset: 5}); !ok || string(data) != "456789" {
		t.Fatalf("continue from 5: %q %v", data, ok)
	}
	for _, req := range []syncRequest{
		{replid: "other", offset: 5},
		{replid: "?", offset: -1},
		{replid: m.replid, offset: 12},
	} {
		if _, ok := m.continuableLocked(req); ok {
			t.Fatalf("%+v was allowed to continue", req)
		}
	}
	// After a promotion the former id is honoured up to the switch point.
	m.replid2, m.secondOffset, m.replid = m.replid, m.offset+1, NewReplID()
	m.appendLocked([]byte("abc"))
	if _, ok := m.continuableLocked(syncRequest{replid: m.replid2, offset: 11}); !ok {
		t.Fatal("the former id could not continue at the switch point")
	}
	if _, ok := m.continuableLocked(syncRequest{replid: m.replid2, offset: 12}); ok {
		t.Fatal("the former id continued past the switch point")
	}
}

func TestFeedSelectsDatabasesAndWrapsTransactions(t *testing.T) {
	m, _ := newTestManager(t, Config{})
	m.mu.Lock()
	m.createBacklogLocked()
	m.mu.Unlock()
	m.Feed(0, "SET", [][]byte{[]byte("a"), []byte("1")})
	m.Feed(0, "DEL", [][]byte{[]byte("a")})
	m.FeedTransaction([]command.ExecutedCommand{
		{Name: "INCR", Args: [][]byte{[]byte("b")}, DB: 2},
		{Name: "INCR", Args: [][]byte{[]byte("c")}, DB: 2},
	})
	m.mu.Lock()
	got, _ := m.backlogFromLocked(0)
	m.mu.Unlock()
	var want []byte
	for _, c := range [][]string{
		{"SELECT", "0"}, {"SET", "a", "1"}, {"DEL", "a"},
		{"MULTI"}, {"SELECT", "2"}, {"INCR", "b"}, {"INCR", "c"}, {"EXEC"},
	} {
		want = append(want, encodeCommand(c...)...)
	}
	if !bytes.Equal(got, want) {
		t.Fatalf("stream\n%q\nwant\n%q", got, want)
	}
}

func TestFeedIsIgnoredWithoutBacklogOrOnReplica(t *testing.T) {
	m, _ := newTestManager(t, Config{})
	m.Feed(0, "SET", [][]byte{[]byte("a"), []byte("1")})
	if _, off := m.ReplID(); off != 0 {
		t.Fatalf("a master without replicas counted offset %d", off)
	}
	m.mu.Lock()
	m.createBacklogLocked()
	m.role = RoleReplica
	m.mu.Unlock()
	m.Feed(0, "SET", [][]byte{[]byte("a"), []byte("1")})
	if _, off := m.ReplID(); off != 0 {
		t.Fatalf("a replica fed its own writes: offset %d", off)
	}
}

func TestReplicaQueueOverflowDetaches(t *testing.T) {
	m, _ := newTestManager(t, Config{ReplicaBufferLimit: 32})
	a, b := net.Pipe()
	defer b.Close()
	r := &replica{m: m, conn: a, addr: "pipe", state: stateOnline}
	r.cond = sync.NewCond(&r.mu)
	m.mu.Lock()
	m.createBacklogLocked()
	m.replicas[r] = struct{}{}
	dropped := m.appendLocked(make([]byte, 20))
	if len(dropped) != 0 {
		t.Fatal("dropped below the limit")
	}
	dropped = m.appendLocked(make([]byte, 20))
	_, still := m.replicas[r]
	m.mu.Unlock()
	if len(dropped) != 1 || still {
		t.Fatalf("over the limit: dropped %d, still attached %v", len(dropped), still)
	}
	for _, d := range dropped {
		d.close()
	}
	if _, err := b.Read(make([]byte, 1)); err == nil {
		t.Fatal("the dropped replica's connection is still open")
	}
}

func TestWaitTimesOutAndCountsAcks(t *testing.T) {
	m, _ := newTestManager(t, Config{})
	start := time.Now()
	if n := m.Wait(1, 50*time.Millisecond, nil); n != 0 {
		t.Fatalf("WAIT without replicas: %d", n)
	}
	if time.Since(start) < 40*time.Millisecond {
		t.Fatal("WAIT returned before its timeout")
	}
	a, b := net.Pipe()
	defer a.Close()
	defer b.Close()
	go func() { _, _ = io.Copy(io.Discard, b) }()
	r := &replica{m: m, conn: a, addr: "pipe", state: stateOnline}
	r.cond = sync.NewCond(&r.mu)
	m.mu.Lock()
	m.createBacklogLocked()
	m.replicas[r] = struct{}{}
	m.appendLocked([]byte("xyz"))
	m.mu.Unlock()
	go func() {
		time.Sleep(20 * time.Millisecond)
		r.Command("REPLCONF", [][]byte{[]byte("ACK"), []byte("3")})
	}()
	if n := m.Wait(1, 5*time.Second, nil); n != 1 {
		t.Fatalf("WAIT after an ACK: %d", n)
	}
}

func TestInfoAndSavedState(t *testing.T) {
	m, _ := newTestManager(t, Config{BacklogSize: 100})
	info := m.Info()
	for _, want := range []string{"role:master", "connected_slaves:0", "repl_backlog_active:0", "repl_backlog_size:100"} {
		if !strings.Contains(info, want+"\r\n") {
			t.Fatalf("INFO lacks %q:\n%s", want, info)
		}
	}
	if _, ok := m.SavedState(); ok {
		t.Fatal("a master saved a replica state")
	}
	m.mu.Lock()
	m.role = RoleReplica
	m.masterHost, m.masterPort = "h", 7
	m.offset, m.linkDB = 99, 3
	m.mu.Unlock()
	st, ok := m.SavedState()
	if !ok || st.Offset != 99 || st.DB != 3 {
		t.Fatalf("saved state %+v %v", st, ok)
	}
	info = m.Info()
	for _, want := range []string{"role:slave", "master_host:h", "master_port:7", "master_link_status:down", "slave_repl_offset:99"} {
		if !strings.Contains(info, want+"\r\n") {
			t.Fatalf("replica INFO lacks %q:\n%s", want, info)
		}
	}
	m.mu.Lock()
	m.role = RoleMaster
	m.mu.Unlock()
}

func TestSnapshotRoundTripThroughLoad(t *testing.T) {
	src, srcDBs := newTestManager(t, Config{})
	_ = src
	srcDBs.DB(0).Set("a", &store.StringValue{Data: []byte("1")}, store.SetOptions{})
	srcDBs.DB(2).Set("b", &store.ListValue{Elements: [][]byte{[]byte("x"), []byte("y")}}, store.SetOptions{TTL: time.Hour})
	var snap bytes.Buffer
	if err := writeSnapshotForTest(&snap, srcDBs); err != nil {
		t.Fatal(err)
	}
	payload := append([]byte("$"+itoa(snap.Len())+"\r\n"), snap.Bytes()...)

	dst, dstDBs := newTestManager(t, Config{})
	dstDBs.DB(1).Set("stale", &store.StringValue{Data: []byte("x")}, store.SetOptions{})
	if err := dst.loadSnapshot(bufio.NewReader(bytes.NewReader(payload)), "id1", 77); err != nil {
		t.Fatal(err)
	}
	if id, off := dst.ReplID(); id != "id1" || off != 77 {
		t.Fatalf("after load: %s %d", id, off)
	}
	if _, ok := dstDBs.DB(1).Get("stale"); ok {
		t.Fatal("a full resync kept a key the master does not have")
	}
	if e, ok := dstDBs.DB(2).Get("b"); !ok || e.TTL() <= 0 {
		t.Fatal("list with TTL in database 2 was not loaded")
	}
	if e, ok := dstDBs.DB(0).Get("a"); !ok || string(e.Value.(*store.StringValue).Data) != "1" {
		t.Fatal("string in database 0 was not loaded")
	}
	// A truncated payload is an error, not a partial success.
	short := append([]byte("$"+itoa(snap.Len()+10)+"\r\n"), snap.Bytes()...)
	if err := dst.loadSnapshot(bufio.NewReader(bytes.NewReader(short)), "id2", 1); err == nil {
		t.Fatal("a truncated snapshot loaded")
	}
}

func writeSnapshotForTest(w io.Writer, dbs *store.Databases) error {
	return persistence.WriteSnapshot(w, DBSource{DBs: dbs})
}

func itoa(n int) string { return strconv.Itoa(n) }
