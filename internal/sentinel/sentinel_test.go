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
