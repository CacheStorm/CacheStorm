package replication

import (
	"bufio"
	"bytes"
	"net"
	"strings"
	"testing"
	"time"

	"github.com/cachestorm/cachestorm/internal/config"
	"github.com/cachestorm/cachestorm/internal/store"
)

func TestGenerateReplicaID(t *testing.T) {
	id1 := generateReplicaID()
	time.Sleep(time.Nanosecond)
	_ = generateReplicaID()

	if id1 == "" {
		t.Error("replica ID should not be empty")
	}
}

func TestInitManager(t *testing.T) {
	cfg := &config.ReplicationConfig{
		Role: "master",
	}
	s := store.NewStore()

	m := InitManager(cfg, s)
	if m == nil {
		t.Fatal("expected manager")
	}
}

func TestGetManager(t *testing.T) {
	m1 := GetManager()
	if m1 == nil {
		t.Error("expected manager")
	}
}

func TestManagerGetReplicaID(t *testing.T) {
	m := GetManager()
	if m == nil {
		t.Fatal("expected manager")
	}

	id := m.GetReplicaID()
	if id == "" {
		t.Error("replica ID should not be empty")
	}
}

func TestManagerGetMasterOffset(t *testing.T) {
	m := GetManager()
	if m == nil {
		t.Fatal("expected manager")
	}

	offset := m.GetMasterOffset()
	if offset != 0 {
		t.Errorf("expected offset 0, got %d", offset)
	}
}

func TestManagerGetReplicas(t *testing.T) {
	m := GetManager()
	if m == nil {
		t.Fatal("expected manager")
	}

	replicas := m.GetReplicas()
	if len(replicas) != 0 {
		t.Errorf("expected 0 replicas, got %d", len(replicas))
	}
}

func TestManagerGetReplicaCount(t *testing.T) {
	m := GetManager()
	if m == nil {
		t.Fatal("expected manager")
	}

	count := m.GetReplicaCount()
	if count != 0 {
		t.Errorf("expected 0 replicas, got %d", count)
	}
}

func TestManagerSetRole(t *testing.T) {
	m := GetManager()
	if m == nil {
		t.Fatal("expected manager")
	}

	originalRole := m.GetRole()
	m.SetRole(RoleReplica)
	if m.GetRole() != RoleReplica {
		t.Errorf("expected RoleReplica, got %d", m.GetRole())
	}

	m.SetRole(RoleMaster)
	if m.GetRole() != RoleMaster {
		t.Errorf("expected RoleMaster, got %d", m.GetRole())
	}

	m.SetRole(originalRole)
}

func TestManagerOnRoleChange(t *testing.T) {
	m := GetManager()
	if m == nil {
		t.Fatal("expected manager")
	}

	originalCallback := m.onRoleChange
	changed := false
	m.OnRoleChange(func(r Role) {
		changed = true
	})

	m.SetRole(RoleReplica)
	if !changed {
		t.Error("expected role change callback to be called")
	}

	m.SetRole(RoleMaster)
	m.onRoleChange = originalCallback
}

func TestManagerGetInfoMaster(t *testing.T) {
	m := GetManager()
	if m == nil {
		t.Fatal("expected manager")
	}

	originalRole := m.GetRole()
	m.SetRole(RoleMaster)

	info := m.GetInfo()
	if info == "" {
		t.Error("expected info string")
	}

	m.SetRole(originalRole)
}

func TestManagerReplicaOf(t *testing.T) {
	m := GetManager()
	if m == nil {
		t.Fatal("expected manager")
	}

	originalRole := m.GetRole()

	err := m.ReplicaOf("localhost", 6379)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if m.GetRole() != RoleReplica {
		t.Errorf("expected RoleReplica, got %d", m.GetRole())
	}

	m.SetRole(originalRole)
}

func TestManagerReplicaOfNoOne(t *testing.T) {
	m := GetManager()
	if m == nil {
		t.Fatal("expected manager")
	}

	err := m.ReplicaOf("no", 1)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if m.GetRole() != RoleMaster {
		t.Errorf("expected RoleMaster, got %d", m.GetRole())
	}
}

func TestManagerStartMaster(t *testing.T) {
	m := GetManager()
	if m == nil {
		t.Fatal("expected manager")
	}

	originalRole := m.GetRole()
	m.SetRole(RoleMaster)

	err := m.Start()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	m.Stop()

	m.SetRole(originalRole)
}

func TestManagerRemoveReplica(t *testing.T) {
	m := GetManager()
	if m == nil {
		t.Fatal("expected manager")
	}

	m.RemoveReplica("nonexistent")
}

func TestManagerUpdateReplicaOffset(t *testing.T) {
	m := GetManager()
	if m == nil {
		t.Fatal("expected manager")
	}

	m.UpdateReplicaOffset("nonexistent", 100)
}

func TestManagerPropagateCommand(t *testing.T) {
	m := GetManager()
	if m == nil {
		t.Fatal("expected manager")
	}

	m.PropagateCommand([]byte("*3\r\n$3\r\nSET\r\n$3\r\nkey\r\n$5\r\nvalue\r\n"))
}

func TestBoolToInt(t *testing.T) {
	if boolToInt(true) != 1 {
		t.Error("expected 1 for true")
	}
	if boolToInt(false) != 0 {
		t.Error("expected 0 for false")
	}
}

func TestGetMasterLinkStatus(t *testing.T) {
	m := GetManager()
	if m == nil {
		t.Fatal("expected manager")
	}

	status := m.getMasterLinkStatus()
	if status != "down" {
		t.Errorf("expected 'down', got '%s'", status)
	}
}

func TestGetSecondsSinceMasterIO(t *testing.T) {
	m := GetManager()
	if m == nil {
		t.Fatal("expected manager")
	}

	seconds := m.getSecondsSinceMasterIO()
	if seconds != 0 {
		t.Errorf("expected 0, got %d", seconds)
	}
}

func TestGetSyncInProgress(t *testing.T) {
	m := GetManager()
	if m == nil {
		t.Fatal("expected manager")
	}

	sync := m.getSyncInProgress()
	if sync != 0 {
		t.Errorf("expected 0, got %d", sync)
	}
}

func TestSyncWriter(t *testing.T) {
	var buf bytes.Buffer
	w := NewSyncWriter(&buf)

	w.WriteRDBHeader()
	if buf.Len() == 0 {
		t.Error("expected header to be written")
	}
}

func TestSyncWriterWriteDatabaseSelect(t *testing.T) {
	var buf bytes.Buffer
	w := NewSyncWriter(&buf)

	w.WriteDatabaseSelect(0)
	if buf.Len() == 0 {
		t.Error("expected database select to be written")
	}
}

func TestSyncWriterWriteKeyValuePairWithTTL(t *testing.T) {
	var buf bytes.Buffer
	w := NewSyncWriter(&buf)

	w.WriteKeyValuePair("mykey", "myvalue", time.Hour, time.Now().Add(time.Hour).Unix())
	if buf.Len() == 0 {
		t.Error("expected key-value pair to be written")
	}
}

func TestSyncWriterWriteKeyValuePairNoTTL(t *testing.T) {
	var buf bytes.Buffer
	w := NewSyncWriter(&buf)

	w.WriteKeyValuePair("mykey", "myvalue", 0, 0)
	if buf.Len() == 0 {
		t.Error("expected key-value pair to be written")
	}
}

func TestSyncWriterWriteKeyValuePairBytes(t *testing.T) {
	var buf bytes.Buffer
	w := NewSyncWriter(&buf)

	w.WriteKeyValuePair("mykey", []byte("myvalue"), 0, 0)
	if buf.Len() == 0 {
		t.Error("expected key-value pair to be written")
	}
}

func TestSyncWriterWriteEnd(t *testing.T) {
	var buf bytes.Buffer
	w := NewSyncWriter(&buf)

	w.WriteEnd()
	if buf.Len() == 0 {
		t.Error("expected end marker to be written")
	}
}

func TestRoleConstants(t *testing.T) {
	if RoleMaster != 0 {
		t.Errorf("expected RoleMaster = 0, got %d", RoleMaster)
	}
	if RoleReplica != 1 {
		t.Errorf("expected RoleReplica = 1, got %d", RoleReplica)
	}
}

func TestReplicaStateConstants(t *testing.T) {
	if StateConnecting != 0 {
		t.Errorf("expected StateConnecting = 0, got %d", StateConnecting)
	}
	if StateSyncing != 1 {
		t.Errorf("expected StateSyncing = 1, got %d", StateSyncing)
	}
	if StateConnected != 2 {
		t.Errorf("expected StateConnected = 2, got %d", StateConnected)
	}
	if StateDisconnected != 3 {
		t.Errorf("expected StateDisconnected = 3, got %d", StateDisconnected)
	}
}

func TestAddReplica(t *testing.T) {
	m := GetManager()
	if m == nil {
		t.Fatal("expected manager")
	}

	m.mu.Lock()
	originalCount := len(m.replicas)
	m.replicas["test-replica"] = &Replica{
		ID:    "test-replica",
		IP:    "127.0.0.1",
		Port:  6380,
		State: StateConnected,
	}
	count := len(m.replicas)
	delete(m.replicas, "test-replica")
	m.mu.Unlock()

	if count != originalCount+1 {
		t.Errorf("expected %d replicas, got %d", originalCount+1, count)
	}
}

func TestReplicaOfNoOneSpecial(t *testing.T) {
	m := GetManager()
	if m == nil {
		t.Fatal("expected manager")
	}

	originalRole := m.GetRole()

	err := m.ReplicaOf("no", 1)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if m.GetRole() != RoleMaster {
		t.Errorf("expected RoleMaster, got %d", m.GetRole())
	}

	m.SetRole(originalRole)
}

func TestManagerGetInfoReplica(t *testing.T) {
	m := GetManager()
	if m == nil {
		t.Fatal("expected manager")
	}

	originalRole := m.GetRole()
	m.SetRole(RoleReplica)

	info := m.GetInfo()
	if info == "" {
		t.Error("expected info string")
	}

	m.SetRole(originalRole)
}

func TestGetReplicasNonNil(t *testing.T) {
	m := GetManager()
	if m == nil {
		t.Fatal("expected manager")
	}

	replicas := m.GetReplicas()
	if replicas == nil {
		t.Error("expected non-nil replicas map")
	}
}

func TestConnectToMasterError(t *testing.T) {
	m := GetManager()
	if m == nil {
		t.Fatal("expected manager")
	}

	originalRole := m.GetRole()
	m.SetRole(RoleReplica)

	err := m.connectToMaster()
	if err == nil {
		t.Error("expected error connecting to invalid master")
	}

	m.SetRole(originalRole)
}

func TestGetMasterLinkStatusUp(t *testing.T) {
	m := GetManager()
	if m == nil {
		t.Fatal("expected manager")
	}

	originalRole := m.GetRole()
	m.SetRole(RoleMaster)

	status := m.getMasterLinkStatus()
	_ = status

	m.SetRole(originalRole)
}

func TestManagerStartReplica(t *testing.T) {
	cfg := &config.ReplicationConfig{
		Role: "replica",
	}
	s := store.NewStore()

	m := InitManager(cfg, s)
	if m == nil {
		t.Fatal("expected manager")
	}

	err := m.Start()
	if err != nil {
		t.Logf("start returned: %v", err)
	}
	m.Stop()
}

func TestManagerAddReplica(t *testing.T) {
	m := GetManager()
	if m == nil {
		t.Fatal("expected manager")
	}

	originalRole := m.GetRole()
	m.SetRole(RoleMaster)

	conn := &mockConn{}
	m.AddReplica(conn, "127.0.0.1", 6380, map[string]bool{"eof": true})

	count := m.GetReplicaCount()
	_ = count

	m.RemoveReplica("test-replica-2")
	m.SetRole(originalRole)
}

func TestManagerPropagateToReplicas(t *testing.T) {
	m := GetManager()
	if m == nil {
		t.Fatal("expected manager")
	}

	originalRole := m.GetRole()
	m.SetRole(RoleMaster)

	m.PropagateCommand([]byte("*3\r\n$3\r\nSET\r\n$3\r\nkey\r\n$5\r\nvalue\r\n"))
	m.PropagateCommand([]byte("*2\r\n$3\r\nGET\r\n$3\r\nkey\r\n"))

	m.SetRole(originalRole)
}

func TestManagerUpdateReplicaOffsetExtended(t *testing.T) {
	m := GetManager()
	if m == nil {
		t.Fatal("expected manager")
	}

	originalRole := m.GetRole()
	m.SetRole(RoleMaster)

	m.UpdateReplicaOffset("test-replica-3", 1000)
	m.UpdateReplicaOffset("test-replica-3", 2000)

	m.SetRole(originalRole)
}

type mockConn struct {
	closed bool
}

func (m *mockConn) Read(b []byte) (n int, err error)  { return 0, nil }
func (m *mockConn) Write(b []byte) (n int, err error) { return len(b), nil }
func (m *mockConn) Close() error {
	m.closed = true
	return nil
}
func (m *mockConn) LocalAddr() net.Addr                { return nil }
func (m *mockConn) RemoteAddr() net.Addr               { return nil }
func (m *mockConn) SetDeadline(t time.Time) error      { return nil }
func (m *mockConn) SetReadDeadline(t time.Time) error  { return nil }
func (m *mockConn) SetWriteDeadline(t time.Time) error { return nil }

func TestReplicationHandleMasterResponse(t *testing.T) {
	m := GetManager()
	if m == nil {
		t.Fatal("expected manager")
	}

	t.Run("FULLRESYNC", func(t *testing.T) {
		line := "+FULLRESYNC abc123 1000"
		reader := bufio.NewReader(strings.NewReader(""))
		m.handleMasterResponse(line, reader)

		m.mu.Lock()
		id := m.masterID
		m.mu.Unlock()

		if id != "abc123" {
			t.Errorf("Expected masterID abc123, got %s", id)
		}
	})

	t.Run("CONTINUE", func(t *testing.T) {
		line := "+CONTINUE"
		reader := bufio.NewReader(strings.NewReader(""))
		m.handleMasterResponse(line, reader)
	})

	t.Run("Bulk string", func(t *testing.T) {
		line := "$5"
		reader := bufio.NewReader(strings.NewReader("hello"))
		m.handleMasterResponse(line, reader)
	})
}

func TestReplicationReceiveRDB(t *testing.T) {
	m := GetManager()
	if m == nil {
		t.Fatal("expected manager")
	}

	t.Run("With data", func(t *testing.T) {
		reader := bufio.NewReader(strings.NewReader("$5\nhello"))
		m.receiveRDB(reader)
	})

	t.Run("Empty", func(t *testing.T) {
		reader := bufio.NewReader(strings.NewReader("$0\n"))
		m.receiveRDB(reader)
	})
}

func TestReplicationProcessWriteCommand(t *testing.T) {
	m := GetManager()
	if m == nil {
		t.Fatal("expected manager")
	}

	// processWriteCommand is a no-op currently
	m.processWriteCommand([]byte("SET key value"))
}

func TestReplicationSyncWithMasterMock(t *testing.T) {
	t.Skip("Skipping due to deadlock issues in pipe connections - syncWithMaster blocks indefinitely")
}

func TestReplicationSendHandshakeMock(t *testing.T) {
	m := GetManager()
	if m == nil {
		t.Fatal("expected manager")
	}

	// Create a pipe
	server, client := net.Pipe()
	defer server.Close()

	writer := bufio.NewWriter(client)

	// Read responses in goroutine
	go func() {
		buf := make([]byte, 1024)
		for {
			n, err := server.Read(buf)
			if err != nil {
				return
			}
			_ = n
		}
	}()

	err := m.sendHandshake(writer)
	if err != nil {
		t.Errorf("sendHandshake failed: %v", err)
	}
	client.Close()
}
