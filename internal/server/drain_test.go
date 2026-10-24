package server

import (
	"bufio"
	"context"
	"net"
	"testing"
	"time"

	"github.com/cachestorm/cachestorm/internal/config"
)

// Idle clients used to hold Stop for the whole 10 s drain timeout before they
// were force-closed; a client parked in BLPOP 0 did too.
func TestStopDoesNotWaitForIdleOrBlockedClients(t *testing.T) {
	cfg := config.Default()
	cfg.Server.Bind = "127.0.0.1"
	cfg.Server.Port = 0
	cfg.HTTP.Enabled = false
	cfg.Plugins.Metrics.Enabled = false
	s, err := New(cfg)
	if err != nil {
		t.Fatal(err)
	}
	if err := s.Start(context.Background()); err != nil {
		t.Fatal(err)
	}
	addr := s.listener.Addr().String()

	idle, err := net.Dial("tcp", addr)
	if err != nil {
		t.Fatal(err)
	}
	defer idle.Close()
	blocked, err := net.Dial("tcp", addr)
	if err != nil {
		t.Fatal(err)
	}
	defer blocked.Close()
	if _, err := blocked.Write([]byte("*3\r\n$5\r\nBLPOP\r\n$4\r\nnone\r\n$1\r\n0\r\n")); err != nil {
		t.Fatal(err)
	}
	// Prove the idle connection is live before stopping.
	if _, err := idle.Write([]byte("*1\r\n$4\r\nPING\r\n")); err != nil {
		t.Fatal(err)
	}
	_ = idle.SetReadDeadline(time.Now().Add(2 * time.Second))
	if line, err := bufio.NewReader(idle).ReadString('\n'); err != nil || line != "+PONG\r\n" {
		t.Fatalf("PING: %q %v", line, err)
	}
	time.Sleep(100 * time.Millisecond)

	start := time.Now()
	if err := s.Stop(context.Background()); err != nil {
		t.Fatal(err)
	}
	if took := time.Since(start); took > 3*time.Second {
		t.Fatalf("Stop took %v with one idle and one blocked client", took)
	}
}
