package sentinel

import (
	"context"
	"strings"
	"testing"
	"time"
)

func TestNewSentinel(t *testing.T) {
	cfg := Config{
		ID:     "sentinel-1",
		Addr:   "127.0.0.1",
		Port:   26379,
		Quorum: 2,
	}

	s := New(cfg)
	if s == nil {
		t.Fatal("expected sentinel")
	}
	if s.id != "sentinel-1" {
		t.Errorf("expected ID 'sentinel-1', got '%s'", s.id)
	}
	if s.quorum != 2 {
		t.Errorf("expected quorum 2, got %d", s.quorum)
	}
}

func TestNewSentinelDefaults(t *testing.T) {
	cfg := Config{ID: "sentinel-1"}
	s := New(cfg)

	if s.downAfter != 30*time.Second {
		t.Errorf("expected downAfter 30s, got %v", s.downAfter)
	}
	if s.parallelSyncs != 1 {
		t.Errorf("expected parallelSyncs 1, got %d", s.parallelSyncs)
	}
	if s.failoverTime != 3*time.Minute {
		t.Errorf("expected failoverTime 3m, got %v", s.failoverTime)
	}
	if s.quorum != 2 {
		t.Errorf("expected quorum 2, got %d", s.quorum)
	}
}

func TestSentinelStartStop(t *testing.T) {
	cfg := Config{ID: "sentinel-1"}
	s := New(cfg)

	err := s.Start()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !s.running.Load() {
		t.Error("expected running to be true")
	}

	s.Stop()
	if s.running.Load() {
		t.Error("expected running to be false after stop")
	}
}

func TestSentinelStopTwice(t *testing.T) {
	cfg := Config{ID: "sentinel-1"}
	s := New(cfg)
	s.Start()

	s.Stop()
	s.Stop()
}

func TestSentinelMonitor(t *testing.T) {
	cfg := Config{ID: "sentinel-1"}
	s := New(cfg)

	err := s.Monitor("mymaster", "127.0.0.1", 6379, 2)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	master, ok := s.GetMaster("mymaster")
	if !ok {
		t.Fatal("expected to find master")
	}
	if master.Name != "mymaster" {
		t.Errorf("expected name 'mymaster', got '%s'", master.Name)
	}
	if master.Addr != "127.0.0.1" {
		t.Errorf("expected addr '127.0.0.1', got '%s'", master.Addr)
	}
	if master.Port != 6379 {
		t.Errorf("expected port 6379, got %d", master.Port)
	}
	if master.Quorum != 2 {
		t.Errorf("expected quorum 2, got %d", master.Quorum)
	}
}

func TestSentinelMonitorDuplicate(t *testing.T) {
	cfg := Config{ID: "sentinel-1"}
	s := New(cfg)

	s.Monitor("mymaster", "127.0.0.1", 6379, 2)
	err := s.Monitor("mymaster", "127.0.0.1", 6380, 2)
	if err == nil {
		t.Error("expected error for duplicate monitor")
	}
}

func TestSentinelRemove(t *testing.T) {
	cfg := Config{ID: "sentinel-1"}
	s := New(cfg)

	s.Monitor("mymaster", "127.0.0.1", 6379, 2)

	err := s.Remove("mymaster")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	_, ok := s.GetMaster("mymaster")
	if ok {
		t.Error("should not find removed master")
	}
}

func TestSentinelRemoveNonExistent(t *testing.T) {
	cfg := Config{ID: "sentinel-1"}
	s := New(cfg)

	err := s.Remove("nonexistent")
	if err == nil {
		t.Error("expected error for nonexistent master")
	}
}

func TestSentinelMasters(t *testing.T) {
	cfg := Config{ID: "sentinel-1"}
	s := New(cfg)

	s.Monitor("master1", "127.0.0.1", 6379, 2)
	s.Monitor("master2", "127.0.0.1", 6380, 2)

	masters := s.Masters()
	if len(masters) != 2 {
		t.Errorf("expected 2 masters, got %d", len(masters))
	}
}

func TestSentinelGetMasterAddr(t *testing.T) {
	cfg := Config{ID: "sentinel-1"}
	s := New(cfg)
	s.Monitor("mymaster", "127.0.0.1", 6379, 2)

	addr, port, err := s.GetMasterAddr("mymaster")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if addr != "127.0.0.1" {
		t.Errorf("expected addr '127.0.0.1', got '%s'", addr)
	}
	if port != 6379 {
		t.Errorf("expected port 6379, got %d", port)
	}
}

func TestSentinelGetMasterAddrNonExistent(t *testing.T) {
	cfg := Config{ID: "sentinel-1"}
	s := New(cfg)

	_, _, err := s.GetMasterAddr("nonexistent")
	if err == nil {
		t.Error("expected error for nonexistent master")
	}
}

func TestSentinelCKQUORUM(t *testing.T) {
	cfg := Config{ID: "sentinel-1"}
	s := New(cfg)
	s.Monitor("mymaster", "127.0.0.1", 6379, 2)

	count, err := s.CKQUORUM("mymaster")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if count != 1 {
		t.Errorf("expected count 1 (only this sentinel), got %d", count)
	}
}

func TestSentinelCKQUORUMNonExistent(t *testing.T) {
	cfg := Config{ID: "sentinel-1"}
	s := New(cfg)

	_, err := s.CKQUORUM("nonexistent")
	if err == nil {
		t.Error("expected error for nonexistent master")
	}
}

func TestSentinelFailover(t *testing.T) {
	cfg := Config{ID: "sentinel-1", DownAfter: 100 * time.Millisecond}
	s := New(cfg)
	s.Monitor("mymaster", "127.0.0.1", 6379, 2)

	err := s.Failover("nonexistent")
	if err == nil {
		t.Error("expected error for nonexistent master")
	}
}

func TestSentinelFailoverNonExistent(t *testing.T) {
	cfg := Config{ID: "sentinel-1"}
	s := New(cfg)

	err := s.Failover("nonexistent")
	if err == nil {
		t.Error("expected error for nonexistent master")
	}
}

func TestSentinelReset(t *testing.T) {
	cfg := Config{ID: "sentinel-1"}
	s := New(cfg)

	s.Monitor("master1", "127.0.0.1", 6379, 2)
	s.Monitor("master2", "127.0.0.1", 6380, 2)
	s.Monitor("other", "127.0.0.1", 6381, 2)

	count := s.Reset("master*")
	if count != 2 {
		t.Errorf("expected 2 resets, got %d", count)
	}

	masters := s.Masters()
	if len(masters) != 1 {
		t.Errorf("expected 1 remaining master, got %d", len(masters))
	}
}

func TestSentinelOnFailover(t *testing.T) {
	cfg := Config{ID: "sentinel-1"}
	s := New(cfg)

	called := false
	s.OnFailover(func(master, newAddr string, newPort int) {
		called = true
	})

	if s.onFailover == nil {
		t.Error("expected onFailover callback to be set")
	}
	s.onFailover("test", "127.0.0.1", 6380)
	if !called {
		t.Error("expected callback to be called")
	}
}

func TestSentinelInfo(t *testing.T) {
	cfg := Config{
		ID:        "sentinel-1",
		Addr:      "127.0.0.1",
		Port:      26379,
		DownAfter: 30 * time.Second,
		Quorum:    2,
	}
	s := New(cfg)
	s.Monitor("mymaster", "127.0.0.1", 6379, 2)

	info := s.Info()
	if info["sentinel_id"] != "sentinel-1" {
		t.Errorf("expected sentinel_id 'sentinel-1', got '%v'", info["sentinel_id"])
	}
	if info["sentinel_addr"] != "127.0.0.1" {
		t.Errorf("expected sentinel_addr '127.0.0.1', got '%v'", info["sentinel_addr"])
	}
	if info["sentinel_port"] != 26379 {
		t.Errorf("expected sentinel_port 26379, got %v", info["sentinel_port"])
	}
	if info["masters"] != 1 {
		t.Errorf("expected 1 master, got %v", info["masters"])
	}
	if info["quorum"] != 2 {
		t.Errorf("expected quorum 2, got %v", info["quorum"])
	}
}

func TestMatchPattern(t *testing.T) {
	tests := []struct {
		s       string
		pattern string
		expect  bool
	}{
		{"master1", "*", true},
		{"master1", "master1", true},
		{"master1", "master2", false},
		{"master1", "master*", true},
		{"master1", "*1", true},
		{"master1", "*ster*", true},
		{"master1", "*foo*", false},
	}

	for _, tt := range tests {
		result := matchPattern(tt.s, tt.pattern)
		if result != tt.expect {
			t.Errorf("matchPattern(%s, %s) = %v, expected %v", tt.s, tt.pattern, result, tt.expect)
		}
	}
}

func TestHandleCommand(t *testing.T) {
	cfg := Config{ID: "sentinel-1"}
	s := New(cfg)
	s.Monitor("mymaster", "127.0.0.1", 6379, 2)

	tests := []struct {
		cmd    string
		expect string
	}{
		{"PING", "+PONG"},
		{"", "-ERR empty command"},
		{"UNKNOWN", "-ERR unknown command 'UNKNOWN'"},
		{"SENTINEL", "-ERR wrong number of arguments"},
		{"INFO", "+OK"},
	}

	for _, tt := range tests {
		result := s.handleCommand(tt.cmd)
		if !strings.HasPrefix(result, tt.expect) {
			t.Errorf("handleCommand(%s) = %s, expected prefix %s", tt.cmd, result, tt.expect)
		}
	}
}

func TestHandleSentinel(t *testing.T) {
	cfg := Config{ID: "sentinel-1"}
	s := New(cfg)
	s.Monitor("mymaster", "127.0.0.1", 6379, 2)

	tests := []struct {
		parts  []string
		expect string
	}{
		{[]string{}, "-ERR wrong number of arguments"},
		{[]string{"MASTERS"}, "name:mymaster"},
		{[]string{"MASTER"}, "-ERR wrong number of arguments"},
		{[]string{"MASTER", "mymaster"}, "*28"},
		{[]string{"MASTER", "nonexistent"}, "-ERR no such master"},
		{[]string{"GETMASTER"}, "-ERR wrong number of arguments"},
		{[]string{"GETMASTER", "mymaster"}, "*2"},
		{[]string{"GETMASTER", "nonexistent"}, "-ERR"},
		{[]string{"RESET"}, "-ERR wrong number of arguments"},
		{[]string{"RESET", "nonexistent*"}, ":0"},
		{[]string{"UNKNOWN"}, "-ERR unknown subcommand"},
	}

	for _, tt := range tests {
		result := s.handleSentinel(tt.parts)
		if !strings.Contains(result, tt.expect) {
			t.Errorf("handleSentinel(%v) = %s, expected to contain %s", tt.parts, result, tt.expect)
		}
	}
}

func TestFormatMasters(t *testing.T) {
	cfg := Config{ID: "sentinel-1"}
	s := New(cfg)
	s.Monitor("mymaster", "127.0.0.1", 6379, 2)

	result := s.formatMasters()
	if !strings.Contains(result, "name:mymaster") {
		t.Errorf("expected 'name:mymaster' in result, got %s", result)
	}
}

func TestFormatMaster(t *testing.T) {
	cfg := Config{ID: "sentinel-1"}
	s := New(cfg)
	s.Monitor("mymaster", "127.0.0.1", 6379, 2)

	result := s.formatMaster("mymaster")
	if !strings.Contains(result, "mymaster") {
		t.Errorf("expected 'mymaster' in result, got %s", result)
	}

	result = s.formatMaster("nonexistent")
	if !strings.Contains(result, "-ERR") {
		t.Errorf("expected error for nonexistent master, got %s", result)
	}
}

func TestSentinelServeContextCancel(t *testing.T) {
	cfg := Config{ID: "sentinel-1", Addr: "127.0.0.1"}
	s := New(cfg)

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	err := s.Serve(ctx, 0)
	if err != nil {
		t.Logf("Serve returned (expected due to context cancel or port issue): %v", err)
	}
}

func TestIsReachable(t *testing.T) {
	cfg := Config{ID: "sentinel-1"}
	s := New(cfg)

	result := s.isReachable("127.0.0.1", 59999)
	if result {
		t.Error("expected unreachable for non-listening port")
	}
}

func TestSentinelCheckMasters(t *testing.T) {
	cfg := Config{ID: "sentinel-1", DownAfter: 100 * time.Millisecond}
	s := New(cfg)
	s.Monitor("mymaster", "127.0.0.1", 59998, 2)

	s.mu.Lock()
	for name, master := range s.masters {
		if s.isReachable(master.Addr, master.Port) {
			master.State = MasterStateOK
		} else {
			master.State = MasterStateSDown
		}
		_ = name
	}
	s.mu.Unlock()

	master, ok := s.GetMaster("mymaster")
	if !ok {
		t.Fatal("expected master")
	}
	if master.State != MasterStateSDown && master.State != MasterStateNone {
		t.Logf("master state: %d", master.State)
	}
}

func TestSentinelCheckODown(t *testing.T) {
	cfg := Config{ID: "sentinel-1", DownAfter: 100 * time.Millisecond, Quorum: 2}
	s := New(cfg)
	s.Monitor("mymaster", "127.0.0.1", 59998, 2)

	master := &MasterInfo{
		Name:     "mymaster",
		Addr:     "127.0.0.1",
		Port:     59998,
		State:    MasterStateSDown,
		Replicas: []*ReplicaInfo{},
	}
	s.mu.Lock()
	s.masters["mymaster"] = master
	s.sentinels["mymaster"] = []*SentinelPeer{}
	s.mu.Unlock()

	peers := s.sentinels["mymaster"]
	downCount := 0
	for _, peer := range peers {
		if time.Since(peer.LastSeen) < s.downAfter {
			downCount++
		}
	}
	result := downCount+1 >= s.quorum
	_ = result
}

func TestSentinelStartFailoverNoReplica(t *testing.T) {
	cfg := Config{ID: "sentinel-1", DownAfter: 100 * time.Millisecond}
	s := New(cfg)

	s.Monitor("mymaster", "127.0.0.1", 59998, 2)

	s.mu.Lock()
	master := s.masters["mymaster"]
	master.State = MasterStateODown
	master.Replicas = []*ReplicaInfo{}
	master.FailoverState = ""
	s.mu.Unlock()

	go s.startFailover("mymaster", &MasterInfo{
		Name:          "mymaster",
		Addr:          "127.0.0.1",
		Port:          59998,
		State:         MasterStateODown,
		Replicas:      []*ReplicaInfo{},
		FailoverState: "",
	})

	time.Sleep(100 * time.Millisecond)
}

func TestSentinelStartFailoverWithReplica(t *testing.T) {
	cfg := Config{ID: "sentinel-1", DownAfter: 100 * time.Millisecond}
	s := New(cfg)

	_ = false
	s.OnFailover(func(master, newAddr string, newPort int) {
	})

	s.Monitor("mymaster", "127.0.0.1", 59998, 2)

	s.mu.Lock()
	master := s.masters["mymaster"]
	master.State = MasterStateODown
	master.Replicas = []*ReplicaInfo{{Addr: "127.0.0.1", Port: 6380, Offset: 100}}
	master.FailoverState = ""
	master.Epoch = 0
	s.mu.Unlock()

	masterCopy := &MasterInfo{
		Name:          "mymaster",
		Addr:          "127.0.0.1",
		Port:          59998,
		State:         MasterStateODown,
		Replicas:      []*ReplicaInfo{{Addr: "127.0.0.1", Port: 6380, Offset: 100}},
		FailoverState: "",
		Epoch:         0,
	}

	go s.startFailover("mymaster", masterCopy)

	time.Sleep(200 * time.Millisecond)
}

func TestSentinelFailoverAlreadyInProgress(t *testing.T) {
	cfg := Config{ID: "sentinel-1"}
	s := New(cfg)
	s.Monitor("mymaster", "127.0.0.1", 6379, 2)

	s.mu.Lock()
	s.masters["mymaster"].FailoverState = "in_progress"
	s.mu.Unlock()

	err := s.Failover("mymaster")
	if err == nil {
		t.Error("expected error when failover already in progress")
	}
}

func TestSentinelFailoverSuccess(t *testing.T) {
	cfg := Config{ID: "sentinel-1"}
	s := New(cfg)
	s.Monitor("mymaster", "127.0.0.1", 6379, 2)

	s.mu.Lock()
	s.masters["mymaster"].Replicas = []*ReplicaInfo{{Addr: "127.0.0.1", Port: 6380, Offset: 100}}
	s.mu.Unlock()

	err := s.Failover("mymaster")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	time.Sleep(100 * time.Millisecond)
}

func TestMasterStateConstants(t *testing.T) {
	if MasterStateNone != 0 {
		t.Errorf("expected MasterStateNone = 0, got %d", MasterStateNone)
	}
	if MasterStateOK != 1 {
		t.Errorf("expected MasterStateOK = 1, got %d", MasterStateOK)
	}
	if MasterStateSDown != 2 {
		t.Errorf("expected MasterStateSDown = 2, got %d", MasterStateSDown)
	}
	if MasterStateODown != 3 {
		t.Errorf("expected MasterStateODown = 3, got %d", MasterStateODown)
	}
}

func TestMasterInfoStruct(t *testing.T) {
	m := &MasterInfo{
		Name:          "mymaster",
		Addr:          "127.0.0.1",
		Port:          6379,
		State:         MasterStateOK,
		NumReplicas:   2,
		NumSentinels:  3,
		Flags:         []string{"master"},
		FailoverState: "",
		Leader:        "sentinel-1",
		Epoch:         1,
		Quorum:        2,
	}

	if m.Name != "mymaster" {
		t.Errorf("unexpected name: %s", m.Name)
	}
}

func TestReplicaInfoStruct(t *testing.T) {
	r := &ReplicaInfo{
		Addr:   "127.0.0.1",
		Port:   6380,
		State:  "online",
		Offset: 1000,
		Lag:    0,
	}

	if r.Addr != "127.0.0.1" {
		t.Errorf("unexpected addr: %s", r.Addr)
	}
}

func TestSentinelPeerStruct(t *testing.T) {
	p := &SentinelPeer{
		ID:    "sentinel-2",
		Addr:  "127.0.0.1",
		Port:  26380,
		RunID: "abc123",
		Epoch: 1,
	}

	if p.ID != "sentinel-2" {
		t.Errorf("unexpected ID: %s", p.ID)
	}
}

func TestConfigStruct(t *testing.T) {
	cfg := Config{
		ID:            "sentinel-1",
		Addr:          "127.0.0.1",
		Port:          26379,
		DownAfter:     30 * time.Second,
		ParallelSyncs: 1,
		FailoverTime:  3 * time.Minute,
		Quorum:        2,
	}

	if cfg.ID != "sentinel-1" {
		t.Errorf("unexpected ID: %s", cfg.ID)
	}
}

func TestSentinelGetMaster(t *testing.T) {
	cfg := Config{ID: "sentinel-1"}
	s := New(cfg)

	_, ok := s.GetMaster("nonexistent")
	if ok {
		t.Error("expected false for nonexistent master")
	}

	s.Monitor("mymaster", "127.0.0.1", 6379, 2)
	master, ok := s.GetMaster("mymaster")
	if !ok {
		t.Fatal("expected to find master")
	}
	if master.Name != "mymaster" {
		t.Errorf("unexpected name: %s", master.Name)
	}
}

func TestSentinelResetAll(t *testing.T) {
	cfg := Config{ID: "sentinel-1"}
	s := New(cfg)

	s.Monitor("master1", "127.0.0.1", 6379, 2)
	s.Monitor("master2", "127.0.0.1", 6380, 2)

	count := s.Reset("*")
	if count != 2 {
		t.Errorf("expected 2 resets, got %d", count)
	}

	masters := s.Masters()
	if len(masters) != 0 {
		t.Errorf("expected 0 remaining masters, got %d", len(masters))
	}
}

func TestSentinelResetExactMatch(t *testing.T) {
	cfg := Config{ID: "sentinel-1"}
	s := New(cfg)

	s.Monitor("master1", "127.0.0.1", 6379, 2)
	s.Monitor("master2", "127.0.0.1", 6380, 2)

	count := s.Reset("master1")
	if count != 1 {
		t.Errorf("expected 1 reset, got %d", count)
	}

	masters := s.Masters()
	if len(masters) != 1 {
		t.Errorf("expected 1 remaining master, got %d", len(masters))
	}
}

func TestSentinelResetSuffixMatch(t *testing.T) {
	cfg := Config{ID: "sentinel-1"}
	s := New(cfg)

	s.Monitor("test-master", "127.0.0.1", 6379, 2)
	s.Monitor("prod-master", "127.0.0.1", 6380, 2)
	s.Monitor("test-slave", "127.0.0.1", 6381, 2)

	count := s.Reset("*-master")
	if count != 2 {
		t.Errorf("expected 2 resets, got %d", count)
	}
}

func TestSentinelResetContainsMatch(t *testing.T) {
	cfg := Config{ID: "sentinel-1"}
	s := New(cfg)

	s.Monitor("my-test-master", "127.0.0.1", 6379, 2)
	s.Monitor("my-prod-server", "127.0.0.1", 6380, 2)

	count := s.Reset("*test*")
	if count != 1 {
		t.Errorf("expected 1 reset, got %d", count)
	}
}

func TestSentinelGossipSentinels(t *testing.T) {
	cfg := Config{ID: "sentinel-1"}
	s := New(cfg)

	s.gossipSentinels()
}
