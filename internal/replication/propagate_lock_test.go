package replication

import (
	"net"
	"sync"
	"testing"

	"github.com/cachestorm/cachestorm/internal/config"
	"github.com/cachestorm/cachestorm/internal/store"
)

// drainPipe keeps a replica socket readable so propagation never blocks.
func drainPipe(t *testing.T) net.Conn {
	t.Helper()
	master, replica := net.Pipe()
	t.Cleanup(func() { _ = master.Close(); _ = replica.Close() })
	go func() {
		buf := make([]byte, 4096)
		for {
			if _, err := replica.Read(buf); err != nil {
				return
			}
		}
	}()
	return master
}

// PropagateCommand held only m.mu.RLock while mutating the shared replBacklog,
// each replica's bufio.Writer, and replica.Offset. A read lock admits
// concurrent readers, so two simultaneous propagations raced on those bytes, and
// Offset++ -- a non-atomic read-modify-write -- silently lost updates,
// desynchronising the replication offset the manager reports.
func TestPropagateCommandOffsetCountsEveryCommand(t *testing.T) {
	const (
		goroutines = 4
		perRoutine = 250
		want       = goroutines * perRoutine
	)

	m := newTestManager(&config.ReplicationConfig{}, store.NewStore())
	m.AddReplica(drainPipe(t), "127.0.0.1", 6380, map[string]bool{})

	cmd := []byte("*3\r\n$3\r\nSET\r\n$3\r\nkey\r\n$5\r\nvalue\r\n")

	var wg sync.WaitGroup
	for g := 0; g < goroutines; g++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for i := 0; i < perRoutine; i++ {
				m.PropagateCommand(cmd)
			}
		}()
	}
	wg.Wait()

	m.mu.RLock()
	var got int64
	for _, r := range m.replicas {
		got = r.Offset
	}
	m.mu.RUnlock()

	if got != want {
		t.Errorf("replica offset = %d after %d concurrent propagations, want %d; "+
			"the non-atomic Offset++ lost updates under the read lock", got, want, want)
	}
}

// TestPropagateCommandDoesNotRaceUnderLoad exercises the backlog and writer
// mutation. Only meaningful under -race, the project's CI gate
// (`go test -race ./internal/... -count=1`).
func TestPropagateCommandDoesNotRaceUnderLoad(t *testing.T) {
	m := newTestManager(&config.ReplicationConfig{}, store.NewStore())
	for i := 0; i < 3; i++ {
		m.AddReplica(drainPipe(t), "127.0.0.1", 6380+i, map[string]bool{})
	}

	cmd := []byte("*3\r\n$3\r\nSET\r\n$3\r\nkey\r\n$5\r\nvalue\r\n")

	var wg sync.WaitGroup
	for g := 0; g < 4; g++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for i := 0; i < 500; i++ {
				m.PropagateCommand(cmd)
			}
		}()
	}
	wg.Wait()
}
