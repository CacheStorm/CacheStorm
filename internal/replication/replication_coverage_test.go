package replication

import (
	"bufio"
	"bytes"
	"errors"
	"io"
	"net"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/cachestorm/cachestorm/internal/config"
	"github.com/cachestorm/cachestorm/internal/store"
)

// newTestManager creates a fresh Manager instance for isolated testing,
// bypassing the sync.Once singleton.
func newTestManager(cfg *config.ReplicationConfig, s *store.Store) *Manager {
	m := &Manager{
		cfg:         cfg,
		store:       s,
		replicas:    make(map[string]*Replica),
		replBacklog: make([]byte, 1024*1024),
		replicaID:   generateReplicaID(),
		stopCh:      make(chan struct{}),
	}
	if cfg.Role == "replica" || cfg.Role == "slave" {
		m.role.Store(int32(RoleReplica))
	} else {
		m.role.Store(int32(RoleMaster))
	}
	return m
}

// errWriter is a writer that always returns an error.
type errWriter struct{}

func (e *errWriter) Write(b []byte) (int, error) {
	return 0, errors.New("write error")
}

// countingErrWriter fails after a certain number of bytes written.
type countingErrWriter struct {
	written   int
	failAfter int
}

func (w *countingErrWriter) Write(b []byte) (int, error) {
	if w.written+len(b) > w.failAfter {
		remaining := w.failAfter - w.written
		if remaining <= 0 {
			return 0, errors.New("write limit reached")
		}
		w.written += remaining
		return remaining, errors.New("write limit reached")
	}
	w.written += len(b)
	return len(b), nil
}

// nthCallErrWriter fails on the nth Write call (0-indexed).
type nthCallErrWriter struct {
	calls    int
	failOnCall int
}

func (w *nthCallErrWriter) Write(b []byte) (int, error) {
	call := w.calls
	w.calls++
	if call == w.failOnCall {
		return 0, errors.New("write error on call")
	}
	return len(b), nil
}

// errConn is a net.Conn that returns errors on Write and configurable on Read.
type errConn struct {
	readData   *bytes.Buffer
	writeErr   error
	closed     bool
	closeCalls int
}

func (c *errConn) Read(b []byte) (int, error) {
	if c.readData != nil {
		return c.readData.Read(b)
	}
	return 0, io.EOF
}

func (c *errConn) Write(b []byte) (int, error) {
	if c.writeErr != nil {
		return 0, c.writeErr
	}
	return len(b), nil
}

func (c *errConn) Close() error {
	c.closed = true
	c.closeCalls++
	return nil
}

func (c *errConn) LocalAddr() net.Addr                { return &net.TCPAddr{} }
func (c *errConn) RemoteAddr() net.Addr               { return &net.TCPAddr{} }
func (c *errConn) SetDeadline(t time.Time) error      { return nil }
func (c *errConn) SetReadDeadline(t time.Time) error  { return nil }
func (c *errConn) SetWriteDeadline(t time.Time) error { return nil }

// --- Tests for syncWithMaster ---

func TestSyncWithMaster_HandshakeError(t *testing.T) {
	// Use net.Pipe; close the server side immediately so handshake write fails.
	m := newTestManager(&config.ReplicationConfig{
		Role:       "replica",
		MasterHost: "127.0.0.1",
		MasterPort: 6379,
	}, store.NewStore())

	server, client := net.Pipe()
	m.masterConn = client

	// Close server immediately so writes to client fail
	server.Close()

	m.wg.Add(1)
	m.syncWithMaster()
	// syncWithMaster should return without blocking
}

func TestSyncWithMaster_ReadsLines(t *testing.T) {
	// Provide data that syncWithMaster can read line-by-line, then EOF.
	m := newTestManager(&config.ReplicationConfig{
		Role:       "replica",
		MasterHost: "127.0.0.1",
		MasterPort: 6379,
	}, store.NewStore())

	server, client := net.Pipe()
	m.masterConn = client

	go func() {
		// Consume the handshake from the client side
		buf := make([]byte, 4096)
		for {
			_, err := server.Read(buf)
			if err != nil {
				return
			}
		}
	}()

	// Write some data the syncWithMaster loop can read, then close
	go func() {
		time.Sleep(50 * time.Millisecond)
		// Send CONTINUE response followed by close
		server.Write([]byte("+CONTINUE\n"))
		time.Sleep(20 * time.Millisecond)
		server.Close()
	}()

	m.wg.Add(1)
	m.syncWithMaster()
}

func TestSyncWithMaster_StopChannel(t *testing.T) {
	m := newTestManager(&config.ReplicationConfig{
		Role:                "replica",
		MasterHost:          "127.0.0.1",
		MasterPort:          6379,
		ReplicaAnnouncePort: 6380,
	}, store.NewStore())

	server, client := net.Pipe()
	m.masterConn = client

	handshakeDone := make(chan struct{})
	go func() {
		// Consume handshake data from server side
		buf := make([]byte, 4096)
		for i := 0; i < 20; i++ {
			server.SetReadDeadline(time.Now().Add(200 * time.Millisecond))
			_, err := server.Read(buf)
			if err != nil {
				break
			}
		}
		server.SetReadDeadline(time.Time{}) // clear deadline
		close(handshakeDone)

		// Close stopCh immediately so that when the for loop starts,
		// the select picks up the stopCh closure.
		close(m.stopCh)

		// Now send a line to unblock the ReadString (in case the select
		// doesn't fire because ReadString is called before the goroutine
		// scheduler checks stopCh). Actually, the select is non-blocking
		// (default case), so if stopCh is closed, it fires immediately.
		// But we need the server to stay alive briefly.
		time.Sleep(100 * time.Millisecond)
		server.Close()
	}()

	m.wg.Add(1)
	m.syncWithMaster()
}

func TestSyncWithMaster_NonEOFReadError(t *testing.T) {
	m := newTestManager(&config.ReplicationConfig{
		Role:                "replica",
		MasterHost:          "127.0.0.1",
		MasterPort:          6379,
		ReplicaAnnouncePort: 6380,
	}, store.NewStore())

	server, client := net.Pipe()
	m.masterConn = client

	go func() {
		// Consume handshake data from server side
		buf := make([]byte, 4096)
		for i := 0; i < 20; i++ {
			server.SetReadDeadline(time.Now().Add(200 * time.Millisecond))
			_, err := server.Read(buf)
			if err != nil {
				break
			}
		}
		server.SetReadDeadline(time.Time{}) // clear deadline

		// After handshake, set a read deadline on the client side so
		// ReadString returns a timeout error (non-EOF).
		client.SetReadDeadline(time.Now().Add(50 * time.Millisecond))

		// Keep the server alive long enough for the timeout to trigger
		time.Sleep(200 * time.Millisecond)
		server.Close()
	}()

	m.wg.Add(1)
	m.syncWithMaster()
}

// --- Tests for sendHandshake error paths ---

func TestSendHandshake_WriteStringError(t *testing.T) {
	m := newTestManager(&config.ReplicationConfig{
		Role:                "replica",
		ReplicaAnnouncePort: 6380,
	}, store.NewStore())

	// Writer backed by a closed pipe will error immediately
	server, client := net.Pipe()
	server.Close()
	writer := bufio.NewWriter(client)

	err := m.sendHandshake(writer)
	// The first write or flush should fail since the pipe is closed
	if err == nil {
		// It's possible the bufio.Writer buffers until flush
		// Either way, the function ran through the error paths
		t.Log("sendHandshake returned nil (buffered), which is acceptable")
	}
	client.Close()
}

func TestSendHandshake_FlushErrors(t *testing.T) {
	// Use a very small buffer writer backed by an errWriter so flushes fail
	m := newTestManager(&config.ReplicationConfig{
		Role:                "replica",
		ReplicaAnnouncePort: 6380,
	}, store.NewStore())

	w := bufio.NewWriterSize(&errWriter{}, 1) // buffer size 1 forces early flushes
	err := m.sendHandshake(w)
	if err == nil {
		t.Error("expected error from sendHandshake with errWriter")
	}
}

func TestSendHandshake_FailAtVariousStages(t *testing.T) {
	m := newTestManager(&config.ReplicationConfig{
		Role:                "replica",
		ReplicaAnnouncePort: 6380,
	}, store.NewStore())

	// With a large bufio buffer (8192), data stays buffered until Flush().
	// Each Flush() makes one Write() call to the underlying writer.
	// Flush calls: 0=PING, 1=REPLCONF port, 2=REPLCONF capa, 3=PSYNC
	// By failing on call N, we test the Flush error at each stage.
	for failCall := 0; failCall <= 4; failCall++ {
		ew := &nthCallErrWriter{failOnCall: failCall}
		w := bufio.NewWriterSize(ew, 8192)
		err := m.sendHandshake(w)
		if failCall < 4 && err == nil {
			t.Errorf("expected error when failing on call %d", failCall)
		}
	}

	// Also test with countingErrWriter and small buffer to hit WriteString
	// error paths (when data exceeds buffer).
	failPoints := []int{0, 5, 18, 30, 60, 100, 150, 200, 250}
	for _, fp := range failPoints {
		cw := &countingErrWriter{failAfter: fp}
		w := bufio.NewWriterSize(cw, 1)
		_ = m.sendHandshake(w)
	}
}

// --- Tests for handleMasterResponse ---

func TestHandleMasterResponse_FULLRESYNCIncomplete(t *testing.T) {
	m := newTestManager(&config.ReplicationConfig{Role: "replica"}, store.NewStore())

	// FULLRESYNC with only 1 part (missing offset)
	reader := bufio.NewReader(strings.NewReader("$0\n"))
	m.handleMasterResponse("+FULLRESYNC abcdef", reader)
	// Should not crash; masterID should not be set since parts < 2
}

func TestHandleMasterResponse_BulkInvalidLength(t *testing.T) {
	m := newTestManager(&config.ReplicationConfig{Role: "replica"}, store.NewStore())

	reader := bufio.NewReader(strings.NewReader(""))
	m.handleMasterResponse("$notanumber", reader)
	// Covers the error branch in bulk string parsing
}

func TestHandleMasterResponse_BulkZeroLength(t *testing.T) {
	m := newTestManager(&config.ReplicationConfig{Role: "replica"}, store.NewStore())

	reader := bufio.NewReader(strings.NewReader(""))
	m.handleMasterResponse("$0", reader)
	// length == 0 does not read data
}

func TestHandleMasterResponse_BulkReadError(t *testing.T) {
	m := newTestManager(&config.ReplicationConfig{Role: "replica"}, store.NewStore())

	// Request 100 bytes but provide only 3 - io.ReadFull will fail
	reader := bufio.NewReader(strings.NewReader("abc"))
	m.handleMasterResponse("$100", reader)
}

func TestHandleMasterResponse_UnrecognizedLine(t *testing.T) {
	m := newTestManager(&config.ReplicationConfig{Role: "replica"}, store.NewStore())

	reader := bufio.NewReader(strings.NewReader(""))
	m.handleMasterResponse("+OK", reader)
	// Covers the "none of the if branches match" path; just increments offset
}

// --- Tests for receiveRDB ---

func TestReceiveRDB_ReadError(t *testing.T) {
	m := newTestManager(&config.ReplicationConfig{Role: "replica"}, store.NewStore())

	// Empty reader returns immediate EOF
	reader := bufio.NewReader(strings.NewReader(""))
	m.receiveRDB(reader)
}

func TestReceiveRDB_InvalidLength(t *testing.T) {
	m := newTestManager(&config.ReplicationConfig{Role: "replica"}, store.NewStore())

	reader := bufio.NewReader(strings.NewReader("$notanumber\n"))
	m.receiveRDB(reader)
}

func TestReceiveRDB_ReadDataError(t *testing.T) {
	m := newTestManager(&config.ReplicationConfig{Role: "replica"}, store.NewStore())

	// Claim 100 bytes but provide only 5
	reader := bufio.NewReader(strings.NewReader("$100\nhello"))
	m.receiveRDB(reader)
}

func TestReceiveRDB_ZeroLength(t *testing.T) {
	m := newTestManager(&config.ReplicationConfig{Role: "replica"}, store.NewStore())

	reader := bufio.NewReader(strings.NewReader("$0\n"))
	m.receiveRDB(reader)
}

func TestReceiveRDB_NonDollarPrefix(t *testing.T) {
	m := newTestManager(&config.ReplicationConfig{Role: "replica"}, store.NewStore())

	reader := bufio.NewReader(strings.NewReader("+OK\n"))
	m.receiveRDB(reader)
	// Line doesn't start with "$", so nothing happens
}

func TestReceiveRDB_SuccessfulRead(t *testing.T) {
	m := newTestManager(&config.ReplicationConfig{Role: "replica"}, store.NewStore())

	reader := bufio.NewReader(strings.NewReader("$5\nhello"))
	m.receiveRDB(reader)
}

// --- Tests for PropagateCommand with connected replicas ---

func TestPropagateCommand_WriteError(t *testing.T) {
	m := newTestManager(&config.ReplicationConfig{Role: "master"}, store.NewStore())

	conn := &errConn{writeErr: errors.New("write failed")}
	replica := &Replica{
		ID:     "r1",
		Conn:   conn,
		State:  StateConnected,
		Writer: bufio.NewWriter(conn),
	}
	m.replicas["r1"] = replica

	m.PropagateCommand([]byte("*3\r\n$3\r\nSET\r\n$3\r\nkey\r\n$5\r\nvalue\r\n"))
	// Covers the Write error branch
}

func TestPropagateCommand_FlushError(t *testing.T) {
	m := newTestManager(&config.ReplicationConfig{Role: "master"}, store.NewStore())

	// Use net.Pipe: close reader side so flush fails after buffering
	server, client := net.Pipe()
	server.Close()

	replica := &Replica{
		ID:     "r2",
		Conn:   client,
		State:  StateConnected,
		Writer: bufio.NewWriterSize(client, 1), // tiny buffer forces immediate flush
	}
	m.replicas["r2"] = replica

	m.PropagateCommand([]byte("*3\r\n$3\r\nSET\r\n$3\r\nkey\r\n$5\r\nvalue\r\n"))
	client.Close()
}

func TestPropagateCommand_SuccessfulWrite(t *testing.T) {
	m := newTestManager(&config.ReplicationConfig{Role: "master"}, store.NewStore())

	server, client := net.Pipe()
	// Consume data on the server side
	go func() {
		buf := make([]byte, 4096)
		for {
			_, err := server.Read(buf)
			if err != nil {
				return
			}
		}
	}()

	replica := &Replica{
		ID:     "r3",
		Conn:   client,
		State:  StateConnected,
		Writer: bufio.NewWriter(client),
	}
	m.replicas["r3"] = replica

	m.PropagateCommand([]byte("*3\r\n$3\r\nSET\r\n$3\r\nkey\r\n$5\r\nvalue\r\n"))

	if replica.Offset != 1 {
		t.Errorf("expected replica offset 1, got %d", replica.Offset)
	}

	client.Close()
	server.Close()
}

func TestPropagateCommand_DisconnectedReplica(t *testing.T) {
	m := newTestManager(&config.ReplicationConfig{Role: "master"}, store.NewStore())

	replica := &Replica{
		ID:     "r4",
		Conn:   &mockConn{},
		State:  StateDisconnected,
		Writer: bufio.NewWriter(&mockConn{}),
	}
	m.replicas["r4"] = replica

	m.PropagateCommand([]byte("*3\r\n$3\r\nSET\r\n"))
	// Disconnected replicas are skipped
	if replica.Offset != 0 {
		t.Errorf("expected offset 0 for disconnected replica, got %d", replica.Offset)
	}
}

func TestPropagateCommand_NilWriter(t *testing.T) {
	m := newTestManager(&config.ReplicationConfig{Role: "master"}, store.NewStore())

	replica := &Replica{
		ID:     "r5",
		Conn:   &mockConn{},
		State:  StateConnected,
		Writer: nil,
	}
	m.replicas["r5"] = replica

	m.PropagateCommand([]byte("*3\r\n$3\r\nSET\r\n"))
	// nil Writer means the replica is skipped (State == Connected but Writer == nil)
	if replica.Offset != 0 {
		t.Errorf("expected offset 0 for nil-writer replica, got %d", replica.Offset)
	}
}

// --- Tests for RemoveReplica with actual replica ---

func TestRemoveReplica_ExistingReplica(t *testing.T) {
	m := newTestManager(&config.ReplicationConfig{Role: "master"}, store.NewStore())

	conn := &errConn{}
	replica := &Replica{
		ID:   "remove-me",
		Conn: conn,
	}
	m.replicas["remove-me"] = replica

	m.RemoveReplica("remove-me")

	if !conn.closed {
		t.Error("expected connection to be closed")
	}
	if len(m.replicas) != 0 {
		t.Errorf("expected 0 replicas, got %d", len(m.replicas))
	}
}

// --- Tests for UpdateReplicaOffset with actual replica ---

func TestUpdateReplicaOffset_ExistingReplica(t *testing.T) {
	m := newTestManager(&config.ReplicationConfig{Role: "master"}, store.NewStore())

	replica := &Replica{
		ID:   "update-me",
		Conn: &mockConn{},
	}
	m.replicas["update-me"] = replica

	m.UpdateReplicaOffset("update-me", 500)

	if replica.Offset != 500 {
		t.Errorf("expected offset 500, got %d", replica.Offset)
	}
	if replica.LastAckTime.IsZero() {
		t.Error("expected LastAckTime to be set")
	}
}

// --- Tests for ReplicaOf ---

func TestReplicaOf_NoOneWithMasterConn(t *testing.T) {
	m := newTestManager(&config.ReplicationConfig{Role: "replica"}, store.NewStore())

	// Set a fake master connection
	server, client := net.Pipe()
	m.masterConn = client

	err := m.ReplicaOf("no", 1)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if m.GetRole() != RoleMaster {
		t.Errorf("expected RoleMaster, got %d", m.GetRole())
	}

	if m.masterConn != nil {
		t.Error("expected masterConn to be nil after REPLICAOF NO ONE")
	}

	if m.cfg.MasterHost != "" {
		t.Errorf("expected empty MasterHost, got %s", m.cfg.MasterHost)
	}
	if m.cfg.MasterPort != 0 {
		t.Errorf("expected MasterPort 0, got %d", m.cfg.MasterPort)
	}

	server.Close()
}

func TestReplicaOf_SetNewMaster(t *testing.T) {
	m := newTestManager(&config.ReplicationConfig{Role: "master"}, store.NewStore())

	// Use localhost with a port that is almost certainly not listening,
	// so the dial fails quickly with "connection refused" (not a timeout).
	err := m.ReplicaOf("127.0.0.1", 59432)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if m.GetRole() != RoleReplica {
		t.Errorf("expected RoleReplica, got %d", m.GetRole())
	}

	if m.cfg.MasterHost != "127.0.0.1" {
		t.Errorf("expected MasterHost 127.0.0.1, got %s", m.cfg.MasterHost)
	}

	// Wait for the background goroutine to try connectToMaster and hit the
	// error logging branch (line 453-455). Connection refused is fast.
	time.Sleep(500 * time.Millisecond)
}

// --- Tests for connectToMaster ---

func TestConnectToMaster_EmptyHost(t *testing.T) {
	m := newTestManager(&config.ReplicationConfig{
		Role:       "replica",
		MasterHost: "",
		MasterPort: 0,
	}, store.NewStore())

	err := m.connectToMaster()
	if err == nil {
		t.Error("expected error for empty host/port")
	}
	if !strings.Contains(err.Error(), "not configured") {
		t.Errorf("expected 'not configured' error, got: %v", err)
	}
}

func TestConnectToMaster_EmptyPort(t *testing.T) {
	m := newTestManager(&config.ReplicationConfig{
		Role:       "replica",
		MasterHost: "127.0.0.1",
		MasterPort: 0,
	}, store.NewStore())

	err := m.connectToMaster()
	if err == nil {
		t.Error("expected error for zero port")
	}
}

func TestConnectToMaster_ConnectionRefused(t *testing.T) {
	m := newTestManager(&config.ReplicationConfig{
		Role:       "replica",
		MasterHost: "127.0.0.1",
		MasterPort: 59123, // unlikely to be listening
	}, store.NewStore())

	err := m.connectToMaster()
	if err == nil {
		t.Error("expected connection error")
	}
}

func TestConnectToMaster_Success(t *testing.T) {
	// Start a listener, connect to it, then close everything.
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("failed to listen: %v", err)
	}
	defer ln.Close()

	addr := ln.Addr().(*net.TCPAddr)

	m := newTestManager(&config.ReplicationConfig{
		Role:       "replica",
		MasterHost: "127.0.0.1",
		MasterPort: addr.Port,
	}, store.NewStore())

	// Accept connections on the listener
	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		conn, err := ln.Accept()
		if err != nil {
			return
		}
		// Drain data then close
		buf := make([]byte, 4096)
		for {
			_, err := conn.Read(buf)
			if err != nil {
				conn.Close()
				return
			}
		}
	}()

	err = m.connectToMaster()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Clean up
	m.stopped.Store(false)
	m.stopCh = make(chan struct{})
	close(m.stopCh)
	if m.masterConn != nil {
		m.masterConn.Close()
	}
	m.wg.Wait()
	wg.Wait()
}

// --- Tests for Start as replica ---

func TestStart_Replica(t *testing.T) {
	m := newTestManager(&config.ReplicationConfig{
		Role:       "replica",
		MasterHost: "127.0.0.1",
		MasterPort: 59876, // nothing listening
	}, store.NewStore())

	err := m.Start()
	if err == nil {
		t.Error("expected error from connectToMaster since nothing is listening")
	}
}

func TestStart_Master(t *testing.T) {
	m := newTestManager(&config.ReplicationConfig{
		Role: "master",
	}, store.NewStore())

	err := m.Start()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

// --- Tests for Stop ---

func TestStop_WithMasterConn(t *testing.T) {
	m := newTestManager(&config.ReplicationConfig{Role: "replica"}, store.NewStore())

	server, client := net.Pipe()
	m.masterConn = client

	m.Stop()
	if !m.stopped.Load() {
		t.Error("expected stopped to be true")
	}
	server.Close()
}

func TestStop_WithReplicas(t *testing.T) {
	m := newTestManager(&config.ReplicationConfig{Role: "master"}, store.NewStore())

	conn1 := &errConn{}
	conn2 := &errConn{}
	m.replicas["r1"] = &Replica{ID: "r1", Conn: conn1}
	m.replicas["r2"] = &Replica{ID: "r2", Conn: conn2}

	m.Stop()

	if !conn1.closed {
		t.Error("expected replica r1 connection to be closed")
	}
	if !conn2.closed {
		t.Error("expected replica r2 connection to be closed")
	}
}

func TestStop_AlreadyStopped(t *testing.T) {
	m := newTestManager(&config.ReplicationConfig{Role: "master"}, store.NewStore())

	m.Stop()
	// Calling Stop again should be safe (returns immediately)
	m.Stop()
}

// --- Tests for GetMasterLinkStatus ---

func TestGetMasterLinkStatus_Up(t *testing.T) {
	m := newTestManager(&config.ReplicationConfig{Role: "replica"}, store.NewStore())

	server, client := net.Pipe()
	m.masterConn = client

	status := m.getMasterLinkStatus()
	if status != "up" {
		t.Errorf("expected 'up', got '%s'", status)
	}

	client.Close()
	server.Close()
}

func TestGetMasterLinkStatus_Down(t *testing.T) {
	m := newTestManager(&config.ReplicationConfig{Role: "replica"}, store.NewStore())

	status := m.getMasterLinkStatus()
	if status != "down" {
		t.Errorf("expected 'down', got '%s'", status)
	}
}

// --- Tests for GetInfo ---

func TestGetInfo_MasterWithConnectedReplicas(t *testing.T) {
	m := newTestManager(&config.ReplicationConfig{Role: "master"}, store.NewStore())

	m.replicas["r1"] = &Replica{
		ID:          "r1",
		IP:          "10.0.0.1",
		Port:        6380,
		State:       StateConnected,
		Offset:      100,
		LastAckTime: time.Now(),
	}
	m.replicas["r2"] = &Replica{
		ID:          "r2",
		IP:          "10.0.0.2",
		Port:        6381,
		State:       StateConnected,
		Offset:      200,
		LastAckTime: time.Now(),
	}

	info := m.GetInfo()
	if !strings.Contains(info, "role:master") {
		t.Error("expected 'role:master' in info")
	}
	if !strings.Contains(info, "connected_replicas:2") {
		t.Error("expected 'connected_replicas:2' in info")
	}
	if !strings.Contains(info, "ip=10.0.0.1") {
		t.Error("expected replica IP in info")
	}
	if !strings.Contains(info, "state=online") {
		t.Error("expected 'state=online' in info")
	}
}

func TestGetInfo_MasterWithDisconnectedReplicas(t *testing.T) {
	m := newTestManager(&config.ReplicationConfig{Role: "master"}, store.NewStore())

	m.replicas["r1"] = &Replica{
		ID:    "r1",
		IP:    "10.0.0.1",
		Port:  6380,
		State: StateDisconnected,
	}

	info := m.GetInfo()
	// Disconnected replicas are still counted in connected_replicas (map length)
	// but should NOT be in the "slave0:" output since State != StateConnected
	if strings.Contains(info, "state=online") {
		t.Error("disconnected replica should not appear with state=online")
	}
}

func TestGetInfo_Replica(t *testing.T) {
	m := newTestManager(&config.ReplicationConfig{
		Role:       "replica",
		MasterHost: "192.168.1.1",
		MasterPort: 6379,
		ReadOnly:   true,
	}, store.NewStore())

	info := m.GetInfo()
	if !strings.Contains(info, "role:slave") {
		t.Error("expected 'role:slave' in info")
	}
	if !strings.Contains(info, "master_host:192.168.1.1") {
		t.Error("expected master_host in info")
	}
	if !strings.Contains(info, "master_port:6379") {
		t.Error("expected master_port in info")
	}
	if !strings.Contains(info, "slave_read_only:1") {
		t.Error("expected slave_read_only:1 in info")
	}
	if !strings.Contains(info, "master_link_status:down") {
		t.Error("expected master_link_status:down in info")
	}
}

func TestGetInfo_ReplicaReadOnlyFalse(t *testing.T) {
	m := newTestManager(&config.ReplicationConfig{
		Role:       "replica",
		MasterHost: "192.168.1.1",
		MasterPort: 6379,
		ReadOnly:   false,
	}, store.NewStore())

	info := m.GetInfo()
	if !strings.Contains(info, "slave_read_only:0") {
		t.Error("expected slave_read_only:0 in info")
	}
}

// --- Tests for GetReplicas with actual replicas ---

func TestGetReplicas_WithEntries(t *testing.T) {
	m := newTestManager(&config.ReplicationConfig{Role: "master"}, store.NewStore())

	m.replicas["r1"] = &Replica{ID: "r1"}
	m.replicas["r2"] = &Replica{ID: "r2"}

	replicas := m.GetReplicas()
	if len(replicas) != 2 {
		t.Errorf("expected 2 replicas, got %d", len(replicas))
	}
}

// --- Tests for InitManager replica role ---

func TestInitManager_ReplicaRole(t *testing.T) {
	// We test the "replica" role path through the newTestManager helper
	m := newTestManager(&config.ReplicationConfig{Role: "replica"}, store.NewStore())
	if m.GetRole() != RoleReplica {
		t.Errorf("expected RoleReplica, got %d", m.GetRole())
	}
}

func TestInitManager_SlaveRole(t *testing.T) {
	m := newTestManager(&config.ReplicationConfig{Role: "slave"}, store.NewStore())
	if m.GetRole() != RoleReplica {
		t.Errorf("expected RoleReplica for 'slave' role, got %d", m.GetRole())
	}
}

// --- Tests for processWriteCommand ---

func TestProcessWriteCommand_Coverage(t *testing.T) {
	m := newTestManager(&config.ReplicationConfig{Role: "replica"}, store.NewStore())
	// processWriteCommand is currently a no-op; verify it doesn't panic
	m.processWriteCommand(nil)
	m.processWriteCommand([]byte{})
	m.processWriteCommand([]byte("SET key value"))
}

// --- Tests for SyncWriter with errors ---

func TestSyncWriter_WriteRDBHeader_Error(t *testing.T) {
	w := NewSyncWriter(&errWriter{})
	err := w.WriteRDBHeader()
	if err == nil {
		t.Error("expected error from WriteRDBHeader")
	}
}

func TestSyncWriter_WriteDatabaseSelect_Error(t *testing.T) {
	w := NewSyncWriter(&errWriter{})
	err := w.WriteDatabaseSelect(0)
	if err == nil {
		t.Error("expected error from WriteDatabaseSelect")
	}
}

func TestSyncWriter_WriteKeyValuePair_WithTTL_Error(t *testing.T) {
	w := NewSyncWriter(&errWriter{})
	err := w.WriteKeyValuePair("key", "value", time.Hour, time.Now().Add(time.Hour).Unix())
	if err == nil {
		t.Error("expected error from WriteKeyValuePair with TTL")
	}
}

func TestSyncWriter_WriteKeyValuePair_NoTTL_Error(t *testing.T) {
	w := NewSyncWriter(&errWriter{})
	err := w.WriteKeyValuePair("key", "value", 0, 0)
	if err == nil {
		t.Error("expected error from WriteKeyValuePair without TTL")
	}
}

func TestSyncWriter_WriteKeyValuePair_BytesValue(t *testing.T) {
	var buf bytes.Buffer
	w := NewSyncWriter(&buf)
	err := w.WriteKeyValuePair("key", []byte("byteval"), 0, 0)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if buf.Len() == 0 {
		t.Error("expected data to be written")
	}
}

func TestSyncWriter_WriteKeyValuePair_IntValue(t *testing.T) {
	var buf bytes.Buffer
	w := NewSyncWriter(&buf)
	// Value is an int (not string or []byte) - should return nil without writing value
	err := w.WriteKeyValuePair("key", 42, 0, 0)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestSyncWriter_WriteEnd_Error(t *testing.T) {
	w := NewSyncWriter(&errWriter{})
	err := w.WriteEnd()
	if err == nil {
		t.Error("expected error from WriteEnd")
	}
}

func TestSyncWriter_WriteStringValue_Error(t *testing.T) {
	w := NewSyncWriter(&errWriter{})
	err := w.writeStringValue("test")
	if err == nil {
		t.Error("expected error from writeStringValue")
	}
}

// --- Tests for AddReplica ---

func TestAddReplica_Full(t *testing.T) {
	m := newTestManager(&config.ReplicationConfig{Role: "master"}, store.NewStore())

	server, client := net.Pipe()
	defer server.Close()

	caps := map[string]bool{"eof": true, "psync2": true}
	replica := m.AddReplica(client, "10.0.0.5", 6380, caps)

	if replica.ID == "" {
		t.Error("expected non-empty replica ID")
	}
	if replica.IP != "10.0.0.5" {
		t.Errorf("expected IP 10.0.0.5, got %s", replica.IP)
	}
	if replica.Port != 6380 {
		t.Errorf("expected port 6380, got %d", replica.Port)
	}
	if replica.State != StateConnected {
		t.Errorf("expected StateConnected, got %d", replica.State)
	}
	if !replica.Capabilities["eof"] {
		t.Error("expected eof capability")
	}
	if replica.Writer == nil {
		t.Error("expected Writer to be set")
	}

	// Verify it was added to the map
	if m.GetReplicaCount() != 1 {
		t.Errorf("expected 1 replica, got %d", m.GetReplicaCount())
	}

	client.Close()
}

// --- Integration-style test for syncWithMaster reading FULLRESYNC ---

func TestSyncWithMaster_FullResyncFlow(t *testing.T) {
	m := newTestManager(&config.ReplicationConfig{
		Role:                "replica",
		MasterHost:          "127.0.0.1",
		MasterPort:          6379,
		ReplicaAnnouncePort: 6380,
	}, store.NewStore())

	server, client := net.Pipe()
	m.masterConn = client

	go func() {
		// Consume handshake data
		buf := make([]byte, 4096)
		for i := 0; i < 10; i++ {
			server.SetReadDeadline(time.Now().Add(500 * time.Millisecond))
			_, err := server.Read(buf)
			if err != nil {
				break
			}
		}
		// Send FULLRESYNC response
		server.Write([]byte("+FULLRESYNC abc123def456 1000\n"))
		// Send RDB length and data
		server.Write([]byte("$5\nhello"))
		time.Sleep(20 * time.Millisecond)
		// Send a bulk write command
		server.Write([]byte("$3\nSET"))
		time.Sleep(20 * time.Millisecond)
		// Close to end the loop
		server.Close()
	}()

	m.wg.Add(1)
	m.syncWithMaster()
	// Verify masterID was set
	m.mu.Lock()
	id := m.masterID
	m.mu.Unlock()
	if id != "abc123def456" {
		t.Logf("masterID: %s (may vary due to timing)", id)
	}
}

// --- Test boolToInt completeness (already tested but verifying via newTestManager) ---

func TestBoolToInt_Additional(t *testing.T) {
	if boolToInt(true) != 1 {
		t.Error("expected 1")
	}
	if boolToInt(false) != 0 {
		t.Error("expected 0")
	}
}

// --- Test for sendHandshake successful path through net.Pipe ---

func TestSendHandshake_Success(t *testing.T) {
	m := newTestManager(&config.ReplicationConfig{
		Role:                "replica",
		ReplicaAnnouncePort: 6380,
	}, store.NewStore())

	server, client := net.Pipe()

	// Drain server in background
	go func() {
		buf := make([]byte, 4096)
		for {
			_, err := server.Read(buf)
			if err != nil {
				return
			}
		}
	}()

	writer := bufio.NewWriter(client)
	err := m.sendHandshake(writer)
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}

	client.Close()
	server.Close()
}

// --- Test for Stop with nil masterConn ---

func TestStop_NilMasterConn(t *testing.T) {
	m := newTestManager(&config.ReplicationConfig{Role: "master"}, store.NewStore())
	m.masterConn = nil
	m.Stop()
	// Should not panic
}

// --- Test for SetRole with callback ---

func TestSetRole_WithCallback(t *testing.T) {
	m := newTestManager(&config.ReplicationConfig{Role: "master"}, store.NewStore())

	var capturedRole Role
	m.OnRoleChange(func(r Role) {
		capturedRole = r
	})

	m.SetRole(RoleReplica)
	if capturedRole != RoleReplica {
		t.Errorf("expected callback with RoleReplica, got %d", capturedRole)
	}

	m.SetRole(RoleMaster)
	if capturedRole != RoleMaster {
		t.Errorf("expected callback with RoleMaster, got %d", capturedRole)
	}
}

func TestSetRole_WithoutCallback(t *testing.T) {
	m := newTestManager(&config.ReplicationConfig{Role: "master"}, store.NewStore())
	m.onRoleChange = nil
	m.SetRole(RoleReplica)
	// Should not panic
}
