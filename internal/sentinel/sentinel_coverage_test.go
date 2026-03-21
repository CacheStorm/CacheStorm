package sentinel

import (
	"context"
	"fmt"
	"net"
	"strings"
	"sync"
	"testing"
	"time"
)

// --- Tests for checkODown ---

func TestCheckODown_NoPeers(t *testing.T) {
	s := New(Config{ID: "s1", Quorum: 2})
	s.Monitor("mymaster", "127.0.0.1", 6379, 2)

	s.mu.RLock()
	master := s.masters["mymaster"]
	s.mu.RUnlock()

	// No peers, only self counts (1), quorum is 2 -> false
	result := s.checkODown(master)
	if result {
		t.Error("expected false with no peers and quorum 2")
	}
}

func TestCheckODown_QuorumOneSelf(t *testing.T) {
	s := New(Config{ID: "s1", Quorum: 1})
	s.Monitor("mymaster", "127.0.0.1", 6379, 1)

	s.mu.RLock()
	master := s.masters["mymaster"]
	s.mu.RUnlock()

	// No peers, self counts as 1, quorum is 1 -> true
	result := s.checkODown(master)
	if !result {
		t.Error("expected true with quorum 1 (self counts)")
	}
}

func TestCheckODown_WithActivePeers(t *testing.T) {
	s := New(Config{ID: "s1", Quorum: 2, DownAfter: 10 * time.Second})
	s.Monitor("mymaster", "127.0.0.1", 6379, 2)

	// Add peers that have been seen recently
	s.mu.Lock()
	s.sentinels["mymaster"] = []*SentinelPeer{
		{ID: "s2", LastSeen: time.Now()},
		{ID: "s3", LastSeen: time.Now()},
	}
	s.mu.Unlock()

	s.mu.RLock()
	master := s.masters["mymaster"]
	s.mu.RUnlock()

	// 2 active peers + self = 3 >= quorum 2 -> true
	result := s.checkODown(master)
	if !result {
		t.Error("expected true with 2 active peers + self >= quorum 2")
	}
}

func TestCheckODown_WithStalePeers(t *testing.T) {
	s := New(Config{ID: "s1", Quorum: 3, DownAfter: 1 * time.Millisecond})
	s.Monitor("mymaster", "127.0.0.1", 6379, 3)

	// Add peers with stale LastSeen
	s.mu.Lock()
	s.sentinels["mymaster"] = []*SentinelPeer{
		{ID: "s2", LastSeen: time.Now().Add(-1 * time.Hour)},
		{ID: "s3", LastSeen: time.Now().Add(-1 * time.Hour)},
	}
	s.mu.Unlock()

	s.mu.RLock()
	master := s.masters["mymaster"]
	s.mu.RUnlock()

	// Stale peers don't count; only self = 1 < quorum 3 -> false
	result := s.checkODown(master)
	if result {
		t.Error("expected false with only stale peers")
	}
}

// --- Tests for startFailover ---

func TestStartFailover_AlreadyInProgress(t *testing.T) {
	s := New(Config{ID: "s1", FailoverTime: 50 * time.Millisecond})
	s.Monitor("mymaster", "127.0.0.1", 6379, 2)

	master := &MasterInfo{
		Name:          "mymaster",
		FailoverState: "waiting", // already in progress
	}

	// Should return immediately without doing anything
	s.startFailover("mymaster", master)
}

func TestStartFailover_NoReplicas(t *testing.T) {
	s := New(Config{ID: "s1", FailoverTime: 10 * time.Millisecond})
	s.Monitor("mymaster", "127.0.0.1", 6379, 2)

	master := &MasterInfo{
		Name:          "mymaster",
		FailoverState: "",
		Replicas:      []*ReplicaInfo{},
	}

	s.startFailover("mymaster", master)

	// After completion, FailoverState should be reset to ""
	s.mu.RLock()
	state := master.FailoverState
	s.mu.RUnlock()
	if state != "" {
		t.Errorf("expected empty FailoverState, got %s", state)
	}
}

func TestStartFailover_WithBestReplica(t *testing.T) {
	s := New(Config{ID: "s1", FailoverTime: 10 * time.Millisecond})
	s.Monitor("mymaster", "127.0.0.1", 6379, 2)

	master := &MasterInfo{
		Name:          "mymaster",
		Addr:          "127.0.0.1",
		Port:          6379,
		FailoverState: "",
		Replicas: []*ReplicaInfo{
			{Addr: "10.0.0.1", Port: 6380, Offset: 100},
			{Addr: "10.0.0.2", Port: 6381, Offset: 200},
			{Addr: "10.0.0.3", Port: 6382, Offset: 150},
		},
		Epoch: 0,
	}

	s.startFailover("mymaster", master)

	s.mu.RLock()
	defer s.mu.RUnlock()

	// Best replica is the one with highest offset (200 at 10.0.0.2:6381)
	if master.Addr != "10.0.0.2" {
		t.Errorf("expected new master addr 10.0.0.2, got %s", master.Addr)
	}
	if master.Port != 6381 {
		t.Errorf("expected new master port 6381, got %d", master.Port)
	}
	if master.State != MasterStateOK {
		t.Errorf("expected MasterStateOK, got %d", master.State)
	}
	if master.FailoverState != "" {
		t.Errorf("expected empty FailoverState, got %s", master.FailoverState)
	}
	if master.Epoch != 1 {
		t.Errorf("expected epoch 1, got %d", master.Epoch)
	}
}

func TestStartFailover_WithCallback(t *testing.T) {
	s := New(Config{ID: "s1", FailoverTime: 10 * time.Millisecond})

	var callbackMaster string
	var callbackAddr string
	var callbackPort int
	var mu sync.Mutex

	s.OnFailover(func(master, newAddr string, newPort int) {
		mu.Lock()
		defer mu.Unlock()
		callbackMaster = master
		callbackAddr = newAddr
		callbackPort = newPort
	})

	master := &MasterInfo{
		Name:          "mymaster",
		FailoverState: "",
		Replicas: []*ReplicaInfo{
			{Addr: "10.0.0.1", Port: 6380, Offset: 500},
		},
		Epoch: 0,
	}

	s.startFailover("mymaster", master)

	mu.Lock()
	defer mu.Unlock()

	if callbackMaster != "mymaster" {
		t.Errorf("expected callback master 'mymaster', got '%s'", callbackMaster)
	}
	if callbackAddr != "10.0.0.1" {
		t.Errorf("expected callback addr '10.0.0.1', got '%s'", callbackAddr)
	}
	if callbackPort != 6380 {
		t.Errorf("expected callback port 6380, got %d", callbackPort)
	}
}

func TestStartFailover_StopDuringWait(t *testing.T) {
	s := New(Config{ID: "s1", FailoverTime: 5 * time.Second})

	master := &MasterInfo{
		Name:          "mymaster",
		FailoverState: "",
		Replicas: []*ReplicaInfo{
			{Addr: "10.0.0.1", Port: 6380, Offset: 100},
		},
	}

	done := make(chan struct{})
	go func() {
		s.startFailover("mymaster", master)
		close(done)
	}()

	// Close stopCh to interrupt the failover wait
	time.Sleep(20 * time.Millisecond)
	close(s.stopCh)

	select {
	case <-done:
		// Good - startFailover returned
	case <-time.After(2 * time.Second):
		t.Error("startFailover did not return after stopCh closed")
	}
}

func TestStartFailover_SingleReplica(t *testing.T) {
	s := New(Config{ID: "s1", FailoverTime: 10 * time.Millisecond})

	master := &MasterInfo{
		Name:          "mymaster",
		Addr:          "127.0.0.1",
		Port:          6379,
		FailoverState: "",
		Replicas: []*ReplicaInfo{
			{Addr: "10.0.0.1", Port: 6380, Offset: 0},
		},
	}

	s.startFailover("mymaster", master)

	s.mu.RLock()
	defer s.mu.RUnlock()

	if master.Addr != "10.0.0.1" {
		t.Errorf("expected failover to 10.0.0.1, got %s", master.Addr)
	}
	if master.Flags[0] != "master" {
		t.Errorf("expected 'master' flag, got %v", master.Flags)
	}
}

// --- Tests for gossipSentinels ---

func TestGossipSentinels_NoPeers(t *testing.T) {
	s := New(Config{ID: "s1"})
	s.Monitor("mymaster", "127.0.0.1", 6379, 2)

	// gossipSentinels is a no-op, but we call it directly to ensure coverage
	s.gossipSentinels()
}

// --- Tests for Serve ---

func TestServe_ListenError(t *testing.T) {
	s := New(Config{ID: "s1", Addr: "999.999.999.999"})

	ctx := context.Background()
	err := s.Serve(ctx, 0)
	if err == nil {
		t.Error("expected error from Serve with invalid address")
	}
}

func TestServe_AcceptAndHandleConnection(t *testing.T) {
	s := New(Config{ID: "s1", Addr: "127.0.0.1"})
	s.Monitor("mymaster", "127.0.0.1", 6379, 2)

	ctx, cancel := context.WithCancel(context.Background())

	errCh := make(chan error, 1)
	go func() {
		errCh <- s.Serve(ctx, 0)
	}()

	// Give server time to start
	time.Sleep(50 * time.Millisecond)

	// Cancel context to stop the server
	cancel()

	select {
	case <-errCh:
		// Server stopped
	case <-time.After(2 * time.Second):
		t.Error("server did not stop after context cancel")
	}
}

func TestServe_WithClientConnection(t *testing.T) {
	s := New(Config{ID: "s1", Addr: "127.0.0.1"})
	s.Monitor("mymaster", "127.0.0.1", 6379, 2)

	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("failed to listen: %v", err)
	}
	port := ln.Addr().(*net.TCPAddr).Port
	ln.Close()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	errCh := make(chan error, 1)
	go func() {
		errCh <- s.Serve(ctx, port)
	}()

	// Give server time to start
	time.Sleep(100 * time.Millisecond)

	// Connect a client
	conn, err := net.DialTimeout("tcp", fmt.Sprintf("127.0.0.1:%d", port), 2*time.Second)
	if err != nil {
		// Port may have been taken; skip
		cancel()
		t.Skipf("could not connect to server: %v", err)
	}
	defer conn.Close()

	// Send PING
	conn.SetDeadline(time.Now().Add(2 * time.Second))
	_, err = conn.Write([]byte("PING\r\n"))
	if err != nil {
		t.Fatalf("write error: %v", err)
	}

	buf := make([]byte, 4096)
	n, err := conn.Read(buf)
	if err != nil {
		t.Fatalf("read error: %v", err)
	}
	response := string(buf[:n])
	if !strings.Contains(response, "+PONG") {
		t.Errorf("expected PONG response, got: %s", response)
	}

	// Send SENTINEL MASTERS
	_, err = conn.Write([]byte("SENTINEL MASTERS\r\n"))
	if err != nil {
		t.Fatalf("write error: %v", err)
	}
	n, err = conn.Read(buf)
	if err != nil {
		t.Fatalf("read error: %v", err)
	}
	response = string(buf[:n])
	if !strings.Contains(response, "mymaster") {
		t.Errorf("expected mymaster in response, got: %s", response)
	}

	// Send INFO
	_, err = conn.Write([]byte("INFO\r\n"))
	if err != nil {
		t.Fatalf("write error: %v", err)
	}
	n, err = conn.Read(buf)
	if err != nil {
		t.Fatalf("read error: %v", err)
	}
	response = string(buf[:n])
	if !strings.Contains(response, "+OK") {
		t.Errorf("expected +OK in response, got: %s", response)
	}

	// Send UNKNOWN command
	_, err = conn.Write([]byte("FOOBAR\r\n"))
	if err != nil {
		t.Fatalf("write error: %v", err)
	}
	n, err = conn.Read(buf)
	if err != nil {
		t.Fatalf("read error: %v", err)
	}
	response = string(buf[:n])
	if !strings.Contains(response, "-ERR") {
		t.Errorf("expected -ERR in response, got: %s", response)
	}

	// Close connection
	conn.Close()
	cancel()
}

// --- Tests for handleConnection ---

func TestHandleConnection_DirectWithPipe(t *testing.T) {
	s := New(Config{ID: "s1"})
	s.Monitor("mymaster", "127.0.0.1", 6379, 2)

	server, client := net.Pipe()

	go func() {
		// Send commands
		client.Write([]byte("PING\r\n"))
		time.Sleep(20 * time.Millisecond)
		client.Write([]byte("SENTINEL MASTERS\r\n"))
		time.Sleep(20 * time.Millisecond)
		client.Write([]byte("SENTINEL MASTER mymaster\r\n"))
		time.Sleep(20 * time.Millisecond)
		client.Write([]byte("SENTINEL GETMASTER mymaster\r\n"))
		time.Sleep(20 * time.Millisecond)
		client.Write([]byte("SENTINEL RESET mymaster\r\n"))
		time.Sleep(20 * time.Millisecond)
		client.Close()
	}()

	// Read responses in background
	go func() {
		buf := make([]byte, 4096)
		for {
			_, err := server.Read(buf)
			if err != nil {
				return
			}
		}
	}()

	// Call handleConnection directly (blocks until client closes)
	s.handleConnection(server)
}

func TestHandleConnection_EmptyLines(t *testing.T) {
	s := New(Config{ID: "s1"})

	server, client := net.Pipe()

	go func() {
		// Send data with empty lines mixed in
		client.Write([]byte("\r\n\r\nPING\r\n\r\n"))
		time.Sleep(20 * time.Millisecond)
		client.Close()
	}()

	go func() {
		buf := make([]byte, 4096)
		for {
			_, err := server.Read(buf)
			if err != nil {
				return
			}
		}
	}()

	s.handleConnection(server)
}

func TestHandleConnection_WriteError(t *testing.T) {
	s := New(Config{ID: "s1"})

	server, client := net.Pipe()

	go func() {
		// Send a command
		client.Write([]byte("PING\r\n"))
		// Immediately close to cause write error on response
		time.Sleep(5 * time.Millisecond)
		client.Close()
	}()

	// Don't read from server side - let writes potentially fail
	s.handleConnection(server)
}

// --- Tests for handleCommand ---

func TestHandleCommand_AllBranches(t *testing.T) {
	s := New(Config{ID: "s1"})
	s.Monitor("mymaster", "127.0.0.1", 6379, 2)

	tests := []struct {
		name   string
		cmd    string
		expect string
	}{
		{"PING", "PING", "+PONG"},
		{"ping lowercase", "ping", "+PONG"},
		{"INFO", "INFO", "+OK"},
		{"info lowercase", "info", "+OK"},
		{"empty", "", "-ERR empty command"},
		{"SENTINEL no args", "SENTINEL", "-ERR wrong number of arguments"},
		{"SENTINEL MASTERS", "SENTINEL MASTERS", "name:mymaster"},
		{"unknown", "RANDOMCMD", "-ERR unknown command"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := s.handleCommand(tt.cmd)
			if !strings.Contains(result, tt.expect) {
				t.Errorf("handleCommand(%q) = %q, expected to contain %q", tt.cmd, result, tt.expect)
			}
		})
	}
}

// --- Tests for handleSentinel ---

func TestHandleSentinel_AllSubcommands(t *testing.T) {
	s := New(Config{ID: "s1"})
	s.Monitor("mymaster", "127.0.0.1", 6379, 2)
	s.mu.Lock()
	s.masters["mymaster"].Flags = []string{"master"}
	s.mu.Unlock()

	tests := []struct {
		name   string
		parts  []string
		expect string
	}{
		{"empty parts", []string{}, "-ERR wrong number of arguments"},
		{"MASTERS", []string{"MASTERS"}, "name:mymaster"},
		{"MASTER with name", []string{"MASTER", "mymaster"}, "*28"},
		{"MASTER missing name", []string{"MASTER"}, "-ERR wrong number of arguments"},
		{"MASTER nonexistent", []string{"MASTER", "missing"}, "-ERR no such master"},
		{"GETMASTER with name", []string{"GETMASTER", "mymaster"}, "*2"},
		{"GETMASTER missing name", []string{"GETMASTER"}, "-ERR wrong number of arguments"},
		{"GETMASTER nonexistent", []string{"GETMASTER", "missing"}, "-ERR"},
		{"RESET with pattern", []string{"RESET", "nonexist*"}, ":0"},
		{"RESET missing pattern", []string{"RESET"}, "-ERR wrong number of arguments"},
		{"unknown subcommand", []string{"FOOBAR"}, "-ERR unknown subcommand"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := s.handleSentinel(tt.parts)
			if !strings.Contains(result, tt.expect) {
				t.Errorf("handleSentinel(%v) = %q, expected to contain %q", tt.parts, result, tt.expect)
			}
		})
	}
}

// --- Tests for formatMasters ---

func TestFormatMasters_Empty(t *testing.T) {
	s := New(Config{ID: "s1"})

	result := s.formatMasters()
	if result != "" {
		t.Errorf("expected empty string for no masters, got: %q", result)
	}
}

func TestFormatMasters_Multiple(t *testing.T) {
	s := New(Config{ID: "s1"})
	s.Monitor("master1", "10.0.0.1", 6379, 2)
	s.Monitor("master2", "10.0.0.2", 6380, 3)

	s.mu.Lock()
	s.masters["master1"].Flags = []string{"master"}
	s.masters["master1"].NumReplicas = 2
	s.masters["master2"].Flags = []string{"master", "s_down"}
	s.masters["master2"].NumReplicas = 1
	s.mu.Unlock()

	result := s.formatMasters()
	if !strings.Contains(result, "name:master1") {
		t.Error("expected master1 in output")
	}
	if !strings.Contains(result, "name:master2") {
		t.Error("expected master2 in output")
	}
	if !strings.Contains(result, "num-replicas:2") {
		t.Error("expected num-replicas:2 in output")
	}
}

// --- Tests for formatMaster ---

func TestFormatMaster_Existing(t *testing.T) {
	s := New(Config{ID: "s1"})
	s.Monitor("mymaster", "10.0.0.1", 6379, 2)

	s.mu.Lock()
	s.masters["mymaster"].Flags = []string{"master"}
	s.mu.Unlock()

	result := s.formatMaster("mymaster")
	if !strings.Contains(result, "*28") {
		t.Errorf("expected *28 prefix, got: %s", result)
	}
	if !strings.Contains(result, "mymaster") {
		t.Errorf("expected mymaster in output, got: %s", result)
	}
	if !strings.Contains(result, "10.0.0.1") {
		t.Errorf("expected 10.0.0.1 in output, got: %s", result)
	}
}

func TestFormatMaster_Nonexistent(t *testing.T) {
	s := New(Config{ID: "s1"})

	result := s.formatMaster("missing")
	if result != "-ERR no such master" {
		t.Errorf("expected '-ERR no such master', got: %s", result)
	}
}

// --- Tests for Failover ---

func TestFailover_NonExistent(t *testing.T) {
	s := New(Config{ID: "s1"})

	err := s.Failover("missing")
	if err == nil {
		t.Error("expected error for nonexistent master")
	}
	if !strings.Contains(err.Error(), "not monitored") {
		t.Errorf("expected 'not monitored' in error, got: %v", err)
	}
}

func TestFailover_AlreadyInProgress(t *testing.T) {
	s := New(Config{ID: "s1"})
	s.Monitor("mymaster", "127.0.0.1", 6379, 2)

	s.mu.Lock()
	s.masters["mymaster"].FailoverState = "waiting"
	s.mu.Unlock()

	err := s.Failover("mymaster")
	if err == nil {
		t.Error("expected error for failover already in progress")
	}
	if !strings.Contains(err.Error(), "already in progress") {
		t.Errorf("expected 'already in progress' in error, got: %v", err)
	}
}

func TestFailover_Success(t *testing.T) {
	s := New(Config{ID: "s1", FailoverTime: 10 * time.Millisecond})
	s.Monitor("mymaster", "127.0.0.1", 6379, 2)

	s.mu.Lock()
	s.masters["mymaster"].Replicas = []*ReplicaInfo{
		{Addr: "10.0.0.1", Port: 6380, Offset: 100},
	}
	s.mu.Unlock()

	err := s.Failover("mymaster")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Wait for async failover to complete
	time.Sleep(50 * time.Millisecond)
}

// --- Tests for Reset ---

func TestReset_ExactMatch(t *testing.T) {
	s := New(Config{ID: "s1"})
	s.Monitor("master1", "127.0.0.1", 6379, 2)
	s.Monitor("master2", "127.0.0.1", 6380, 2)

	count := s.Reset("master1")
	if count != 1 {
		t.Errorf("expected 1 reset, got %d", count)
	}
	if len(s.Masters()) != 1 {
		t.Errorf("expected 1 remaining master, got %d", len(s.Masters()))
	}
}

func TestReset_WildcardAll(t *testing.T) {
	s := New(Config{ID: "s1"})
	s.Monitor("m1", "127.0.0.1", 6379, 2)
	s.Monitor("m2", "127.0.0.1", 6380, 2)
	s.Monitor("m3", "127.0.0.1", 6381, 2)

	count := s.Reset("*")
	if count != 3 {
		t.Errorf("expected 3 resets, got %d", count)
	}
}

func TestReset_PrefixMatch(t *testing.T) {
	s := New(Config{ID: "s1"})
	s.Monitor("prod-master", "127.0.0.1", 6379, 2)
	s.Monitor("prod-slave", "127.0.0.1", 6380, 2)
	s.Monitor("test-master", "127.0.0.1", 6381, 2)

	count := s.Reset("prod*")
	if count != 2 {
		t.Errorf("expected 2 resets for 'prod*', got %d", count)
	}
}

func TestReset_SuffixMatch(t *testing.T) {
	s := New(Config{ID: "s1"})
	s.Monitor("my-master", "127.0.0.1", 6379, 2)
	s.Monitor("your-master", "127.0.0.1", 6380, 2)
	s.Monitor("my-slave", "127.0.0.1", 6381, 2)

	count := s.Reset("*master")
	if count != 2 {
		t.Errorf("expected 2 resets for '*master', got %d", count)
	}
}

func TestReset_ContainsMatch(t *testing.T) {
	s := New(Config{ID: "s1"})
	s.Monitor("a-test-b", "127.0.0.1", 6379, 2)
	s.Monitor("no-match", "127.0.0.1", 6380, 2)

	count := s.Reset("*test*")
	if count != 1 {
		t.Errorf("expected 1 reset for '*test*', got %d", count)
	}
}

func TestReset_NoMatch(t *testing.T) {
	s := New(Config{ID: "s1"})
	s.Monitor("master1", "127.0.0.1", 6379, 2)

	count := s.Reset("nonexistent")
	if count != 0 {
		t.Errorf("expected 0 resets, got %d", count)
	}
}

func TestReset_WithSentinelPeers(t *testing.T) {
	s := New(Config{ID: "s1"})
	s.Monitor("mymaster", "127.0.0.1", 6379, 2)

	s.mu.Lock()
	s.sentinels["mymaster"] = []*SentinelPeer{
		{ID: "s2", Addr: "127.0.0.1", Port: 26380},
	}
	s.mu.Unlock()

	count := s.Reset("mymaster")
	if count != 1 {
		t.Errorf("expected 1 reset, got %d", count)
	}

	// Both masters and sentinels entries should be deleted
	s.mu.RLock()
	_, mastersExist := s.masters["mymaster"]
	_, sentinelsExist := s.sentinels["mymaster"]
	s.mu.RUnlock()

	if mastersExist {
		t.Error("expected mymaster to be removed from masters")
	}
	if sentinelsExist {
		t.Error("expected mymaster to be removed from sentinels")
	}
}

// --- Tests for CKQUORUM ---

func TestCKQUORUM_NonExistent(t *testing.T) {
	s := New(Config{ID: "s1"})

	_, err := s.CKQUORUM("missing")
	if err == nil {
		t.Error("expected error for nonexistent master")
	}
}

func TestCKQUORUM_OnlySelf(t *testing.T) {
	s := New(Config{ID: "s1", Quorum: 2})
	s.Monitor("mymaster", "127.0.0.1", 6379, 2)

	count, err := s.CKQUORUM("mymaster")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if count != 1 {
		t.Errorf("expected 1 (self only), got %d", count)
	}
}

func TestCKQUORUM_WithActivePeers(t *testing.T) {
	s := New(Config{ID: "s1", Quorum: 2, DownAfter: 10 * time.Second})
	s.Monitor("mymaster", "127.0.0.1", 6379, 2)

	s.mu.Lock()
	s.sentinels["mymaster"] = []*SentinelPeer{
		{ID: "s2", LastSeen: time.Now()},
		{ID: "s3", LastSeen: time.Now()},
	}
	s.mu.Unlock()

	count, err := s.CKQUORUM("mymaster")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if count != 3 {
		t.Errorf("expected 3 (2 peers + self), got %d", count)
	}
}

func TestCKQUORUM_WithStalePeers(t *testing.T) {
	s := New(Config{ID: "s1", Quorum: 2, DownAfter: 1 * time.Millisecond})
	s.Monitor("mymaster", "127.0.0.1", 6379, 2)

	s.mu.Lock()
	s.sentinels["mymaster"] = []*SentinelPeer{
		{ID: "s2", LastSeen: time.Now().Add(-1 * time.Hour)},
	}
	s.mu.Unlock()

	// Wait for DownAfter to pass
	time.Sleep(5 * time.Millisecond)

	count, err := s.CKQUORUM("mymaster")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if count != 1 {
		t.Errorf("expected 1 (stale peer not counted), got %d", count)
	}
}

// --- Tests for matchPattern ---

func TestMatchPattern_Comprehensive(t *testing.T) {
	tests := []struct {
		s       string
		pattern string
		expect  bool
	}{
		// wildcard all
		{"anything", "*", true},
		{"", "*", true},
		// exact match
		{"hello", "hello", true},
		{"hello", "world", false},
		// prefix match (pattern ends with *)
		{"hello-world", "hello*", true},
		{"hello", "hello*", true},
		{"hell", "hello*", false},
		// suffix match (pattern starts with *)
		{"hello-world", "*world", true},
		{"world", "*world", true},
		{"worlds", "*world", false},
		// contains match (pattern starts and ends with *)
		{"hello-test-world", "*test*", true},
		{"test", "*test*", true},
		{"hello", "*test*", false},
		// empty pattern exact match
		{"", "", true},
		{"hello", "", false},
	}

	for _, tt := range tests {
		t.Run(fmt.Sprintf("%s_%s", tt.s, tt.pattern), func(t *testing.T) {
			result := matchPattern(tt.s, tt.pattern)
			if result != tt.expect {
				t.Errorf("matchPattern(%q, %q) = %v, expected %v", tt.s, tt.pattern, result, tt.expect)
			}
		})
	}
}

// --- Tests for checkMasters ---
// NOTE: checkMasters has a re-entrant lock issue when calling checkODown
// for unreachable masters (checkMasters holds mu.Lock, checkODown tries
// mu.RLock). We can only safely test the reachable-master path directly.

func TestCheckMasters_ReachableMaster(t *testing.T) {
	// Start a real listener so the master is reachable
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("failed to listen: %v", err)
	}
	defer ln.Close()

	port := ln.Addr().(*net.TCPAddr).Port

	// Accept connections in background
	go func() {
		for {
			conn, err := ln.Accept()
			if err != nil {
				return
			}
			conn.Close()
		}
	}()

	s := New(Config{ID: "s1", DownAfter: 10 * time.Second})

	s.mu.Lock()
	s.masters["mymaster"] = &MasterInfo{
		Name:     "mymaster",
		Addr:     "127.0.0.1",
		Port:     port,
		State:    MasterStateSDown, // was down, now reachable
		Replicas: []*ReplicaInfo{},
	}
	s.mu.Unlock()

	s.checkMasters()

	s.mu.RLock()
	master := s.masters["mymaster"]
	s.mu.RUnlock()

	if master.State != MasterStateOK {
		t.Errorf("expected MasterStateOK, got %d", master.State)
	}
	if master.LastOkPing.IsZero() {
		t.Error("expected LastOkPing to be set")
	}
	if len(master.Flags) != 1 || master.Flags[0] != "master" {
		t.Errorf("expected flags [master], got %v", master.Flags)
	}
}

func TestCheckMasters_Empty(t *testing.T) {
	s := New(Config{ID: "s1"})
	// No masters registered - should not panic
	s.checkMasters()
}

func TestCheckMasters_MultipleMasters_AllReachable(t *testing.T) {
	ln1, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("failed to listen: %v", err)
	}
	defer ln1.Close()
	ln2, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("failed to listen: %v", err)
	}
	defer ln2.Close()

	go func() {
		for {
			conn, err := ln1.Accept()
			if err != nil {
				return
			}
			conn.Close()
		}
	}()
	go func() {
		for {
			conn, err := ln2.Accept()
			if err != nil {
				return
			}
			conn.Close()
		}
	}()

	s := New(Config{ID: "s1", DownAfter: 10 * time.Second})

	port1 := ln1.Addr().(*net.TCPAddr).Port
	port2 := ln2.Addr().(*net.TCPAddr).Port

	s.mu.Lock()
	s.masters["m1"] = &MasterInfo{
		Name:     "m1",
		Addr:     "127.0.0.1",
		Port:     port1,
		State:    MasterStateNone,
		Replicas: []*ReplicaInfo{},
	}
	s.masters["m2"] = &MasterInfo{
		Name:     "m2",
		Addr:     "127.0.0.1",
		Port:     port2,
		State:    MasterStateNone,
		Replicas: []*ReplicaInfo{},
	}
	s.mu.Unlock()

	s.checkMasters()

	s.mu.RLock()
	m1 := s.masters["m1"]
	m2 := s.masters["m2"]
	s.mu.RUnlock()

	if m1.State != MasterStateOK {
		t.Errorf("expected m1 MasterStateOK, got %d", m1.State)
	}
	if m2.State != MasterStateOK {
		t.Errorf("expected m2 MasterStateOK, got %d", m2.State)
	}
}

// --- Tests for isReachable ---

func TestIsReachable_Reachable(t *testing.T) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("failed to listen: %v", err)
	}
	defer ln.Close()

	port := ln.Addr().(*net.TCPAddr).Port
	go func() {
		for {
			conn, err := ln.Accept()
			if err != nil {
				return
			}
			conn.Close()
		}
	}()

	s := New(Config{ID: "s1"})
	if !s.isReachable("127.0.0.1", port) {
		t.Error("expected reachable for active listener")
	}
}

func TestIsReachable_Unreachable(t *testing.T) {
	s := New(Config{ID: "s1"})
	if s.isReachable("127.0.0.1", 59999) {
		t.Error("expected unreachable for non-listening port")
	}
}

// --- Test Serve full flow with Accept and context cancellation ---

func TestServe_FullFlow(t *testing.T) {
	s := New(Config{ID: "s1", Addr: "127.0.0.1"})
	s.Monitor("mymaster", "127.0.0.1", 6379, 2)

	ctx, cancel := context.WithCancel(context.Background())

	errCh := make(chan error, 1)
	go func() {
		errCh <- s.Serve(ctx, 0) // port 0 for random
	}()

	// Give it time to start listening
	time.Sleep(50 * time.Millisecond)

	// Cancel and expect Serve to return
	cancel()

	select {
	case err := <-errCh:
		// Expected - Serve returns with error from closed listener
		_ = err
	case <-time.After(2 * time.Second):
		t.Error("Serve did not return after context cancel")
	}
}

// --- Test for monitorLoop and gossipLoop via Start/Stop ---
// Only test with no masters to avoid the checkODown deadlock.

func TestMonitorAndGossipLoops_NoMasters(t *testing.T) {
	s := New(Config{ID: "s1", DownAfter: 50 * time.Millisecond})
	// No masters - checkMasters will iterate nothing

	err := s.Start()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Let the loops run for a couple ticks
	time.Sleep(150 * time.Millisecond)

	s.Stop()
}

func TestMonitorAndGossipLoops_ReachableMaster(t *testing.T) {
	// Use a reachable master so checkMasters doesn't hit the deadlock path
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("failed to listen: %v", err)
	}
	defer ln.Close()

	go func() {
		for {
			conn, err := ln.Accept()
			if err != nil {
				return
			}
			conn.Close()
		}
	}()

	port := ln.Addr().(*net.TCPAddr).Port

	s := New(Config{ID: "s1", DownAfter: 50 * time.Millisecond})
	s.Monitor("mymaster", "127.0.0.1", port, 2)

	err = s.Start()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Let the loops tick a couple of times
	time.Sleep(2500 * time.Millisecond)

	s.Stop()

	master, ok := s.GetMaster("mymaster")
	if !ok {
		t.Fatal("expected master")
	}
	if master.State != MasterStateOK {
		t.Errorf("expected MasterStateOK after monitoring, got %d", master.State)
	}
}

// --- Test for startFailover with zero failoverTime ---

func TestStartFailover_ZeroFailoverTime(t *testing.T) {
	s := New(Config{ID: "s1"})
	// Explicitly set failoverTime to 0 to test the default
	s.failoverTime = 0

	master := &MasterInfo{
		Name:          "mymaster",
		FailoverState: "",
		Replicas: []*ReplicaInfo{
			{Addr: "10.0.0.1", Port: 6380, Offset: 100},
		},
	}

	done := make(chan struct{})
	go func() {
		s.startFailover("mymaster", master)
		close(done)
	}()

	// Since failoverTime is 0, the function uses 5s default.
	// Close stopCh to break out of the wait.
	time.Sleep(20 * time.Millisecond)
	close(s.stopCh)

	select {
	case <-done:
		// Success
	case <-time.After(2 * time.Second):
		t.Error("startFailover did not return")
	}
}

// --- Test for startFailover with nil onFailover ---

func TestStartFailover_NilCallback(t *testing.T) {
	s := New(Config{ID: "s1", FailoverTime: 10 * time.Millisecond})
	// onFailover is nil by default

	master := &MasterInfo{
		Name:          "mymaster",
		FailoverState: "",
		Replicas: []*ReplicaInfo{
			{Addr: "10.0.0.1", Port: 6380, Offset: 100},
		},
		Epoch: 0,
	}

	// Should complete without panic even with nil callback
	s.startFailover("mymaster", master)

	s.mu.RLock()
	defer s.mu.RUnlock()

	if master.Addr != "10.0.0.1" {
		t.Errorf("expected addr 10.0.0.1, got %s", master.Addr)
	}
}
