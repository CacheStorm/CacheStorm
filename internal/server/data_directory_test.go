package server

import (
	"bytes"
	"context"
	"errors"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/cachestorm/cachestorm/internal/command"
	"github.com/cachestorm/cachestorm/internal/persistence"
	"github.com/cachestorm/cachestorm/internal/resp"
	"github.com/cachestorm/cachestorm/internal/store"
)

func TestPersistentDataDirectoryRejectsSecondOwner(t *testing.T) {
	cfg := aofTxConfig(t)
	s, err := New(cfg)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { stopReplServer(s) })
	second, err := New(cfg)
	if err == nil {
		if second != nil {
			stopReplServer(second)
		}
		t.Fatal("second server acquired an already-owned persistent data directory")
	}
}

func TestDataDirectoryRetainedOnHTTPShutdownTimeout(t *testing.T) {
	cfg := aofTxConfig(t)
	s, err := newOwnedTestServer(t, cfg)
	if err != nil {
		t.Fatal(err)
	}
	if err := s.aof.Start(); err != nil {
		t.Fatal(err)
	}
	lease := s.dataLease
	entered, release := make(chan struct{}), make(chan struct{})
	var releaseOnce sync.Once
	releaseHandler := func() { releaseOnce.Do(func() { close(release) }) }
	done := make(chan struct{})
	var requestErr error
	web := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		close(entered)
		<-release
		var reply bytes.Buffer
		ctx := command.NewContext("SET", [][]byte{[]byte("late-http"), []byte("durable")}, s.store, resp.NewWriter(&reply))
		if err := s.router.Execute(ctx); err != nil {
			w.WriteHeader(http.StatusServiceUnavailable)
			return
		}
		w.WriteHeader(http.StatusOK)
	}))
	s.httpServer = &HTTPServer{server: web.Config, cancel: func() {}}
	go func() {
		response, err := web.Client().Get(web.URL)
		if err == nil {
			response.Body.Close()
			if response.StatusCode != http.StatusOK {
				err = errors.New("late request failed")
			}
		}
		requestErr = err
		close(done)
	}()
	defer func() { releaseHandler(); <-done; web.Close(); s.aof.Stop(); _ = lease.Close() }()
	<-entered
	stopErr := s.Stop(context.Background())
	other, ownershipErr := New(cfg)
	if other != nil {
		stopReplServer(other)
	}
	if stopErr == nil || !errors.Is(ownershipErr, persistence.ErrDataDirectoryLocked) {
		t.Fatalf("HTTP timeout released ownership or hid failure: stop %v owner %v", stopErr, ownershipErr)
	}
	releaseHandler()
	<-done
	if requestErr != nil {
		t.Fatal(requestErr)
	}
	commands, err := persistence.NewAOFReader().Load(filepath.Join(cfg.Persistence.DataDir, "appendonly.aof"))
	if err != nil {
		t.Fatal(err)
	}
	found := false
	for _, cmd := range commands {
		if cmd.Name == "SET" && string(cmd.Args[0]) == "late-http" {
			found = true
		}
	}
	if !found {
		t.Fatal("unfinished HTTP request acknowledged after its AOF writer was closed")
	}
}

type delayedAcceptedConnection struct {
	entered  chan struct{}
	release  chan struct{}
	closed   chan struct{}
	conn     net.Conn
	once     sync.Once
	accepted bool
}

func (l *delayedAcceptedConnection) Accept() (net.Conn, error) {
	if !l.accepted {
		l.accepted = true
		close(l.entered)
		<-l.release
		return l.conn, nil
	}
	return nil, net.ErrClosed
}

func (l *delayedAcceptedConnection) Close() error   { l.once.Do(func() { close(l.closed) }); return nil }
func (l *delayedAcceptedConnection) Addr() net.Addr { return &net.TCPAddr{IP: net.IPv4(127, 0, 0, 1)} }

func TestDataDirectoryHeldUntilAcceptorExits(t *testing.T) {
	cfg := aofTxConfig(t)
	cfg.Server.Bind, cfg.Server.Port, cfg.Server.ProtectedMode = "127.0.0.1", 0, false
	cfg.Sentinel.Enabled = false
	s, err := newOwnedTestServer(t, cfg)
	if err != nil {
		t.Fatal(err)
	}
	serverConn, clientConn := net.Pipe()
	defer clientConn.Close()
	l := &delayedAcceptedConnection{entered: make(chan struct{}), release: make(chan struct{}), closed: make(chan struct{}), conn: serverConn}
	s.listenTCP = func(string, string) (net.Listener, error) { return l, nil }
	if err := s.Start(context.Background()); err != nil {
		t.Fatal(err)
	}
	<-l.entered
	done := make(chan error, 1)
	go func() { done <- s.Stop(context.Background()) }()
	<-l.closed
	finished := false
	select {
	case <-done:
		finished = true
	case <-time.After(100 * time.Millisecond):
	}
	other, ownershipErr := New(cfg)
	if other != nil {
		stopReplServer(other)
	}
	close(l.release)
	clientConn.Close()
	if !finished {
		if err := <-done; err != nil {
			t.Fatal(err)
		}
	}
	if finished || !errors.Is(ownershipErr, persistence.ErrDataDirectoryLocked) {
		t.Fatalf("acceptor outlived directory ownership: stopped %v new owner error %v", finished, ownershipErr)
	}
}

func TestDataDirectoryRetainedOnIncompleteDrain(t *testing.T) {
	cfg := aofTxConfig(t)
	s, err := newOwnedTestServer(t, cfg)
	if err != nil {
		t.Fatal(err)
	}
	s.wg.Add(1)
	lease := s.dataLease
	defer func() { s.wg.Done(); _ = lease.Close() }()
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if err := s.Stop(ctx); err == nil || !strings.Contains(err.Error(), "shutdown incomplete") {
		t.Fatalf("incomplete drain hidden: %v", err)
	}
	other, err := New(cfg)
	if other != nil {
		stopReplServer(other)
	}
	if !errors.Is(err, persistence.ErrDataDirectoryLocked) {
		t.Fatalf("unjoined worker lost ownership: %v", err)
	}
}

func TestDataDirectoryConcurrentStopReleasesOnce(t *testing.T) {
	cfg := aofTxConfig(t)
	s, err := newOwnedTestServer(t, cfg)
	if err != nil {
		t.Fatal(err)
	}
	var wg sync.WaitGroup
	for i := 0; i < 8; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if err := s.Stop(context.Background()); err != nil {
				t.Error(err)
			}
		}()
	}
	wg.Wait()
	if _, err := newOwnedTestServer(t, cfg); err != nil {
		t.Fatalf("concurrent Stop retained ownership: %v", err)
	}
}

func TestNonpersistentServerDoesNotOwnDataDirectory(t *testing.T) {
	cfg := aofTxConfig(t)
	cfg.Persistence.Enabled = false
	cfg.Persistence.DataDir = filepath.Join(t.TempDir(), "unused")
	for i := 0; i < 2; i++ {
		s, err := New(cfg)
		if err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() { stopReplServer(s) })
	}
	if _, err := os.Stat(cfg.Persistence.DataDir); !os.IsNotExist(err) {
		t.Fatalf("stateless server created data directory: %v", err)
	}
}

func TestDataDirectoryPathRemainsPinned(t *testing.T) {
	cfg := snapshotConfig(t, t.TempDir())
	oldDir := cfg.Persistence.DataDir
	s, err := newOwnedTestServer(t, cfg)
	if err != nil {
		t.Fatal(err)
	}
	newDir := t.TempDir()
	cfg.Persistence.DataDir = newDir
	if err := s.saveSnapshot(false); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(oldDir, "dump.rdb")); err != nil {
		t.Fatalf("snapshot left owned directory: %v", err)
	}
	if _, err := os.Stat(filepath.Join(newDir, "dump.rdb")); !os.IsNotExist(err) {
		t.Fatalf("snapshot wrote outside ownership: %v", err)
	}
}

func TestDataDirectoryOwnershipReleaseAndFailedStart(t *testing.T) {
	cfg := aofTxConfig(t)
	cfg.Server.Bind, cfg.Server.Port = "127.0.0.1", -1
	first, err := newOwnedTestServer(t, cfg)
	if err != nil {
		t.Fatal(err)
	}
	if err := first.Start(context.Background()); err == nil {
		t.Fatal("invalid listener started")
	}
	if err := first.Start(context.Background()); err == nil {
		t.Fatal("failed persistent server reused stale loaded state")
	}
	cfg.Server.Port = 0
	second, err := newOwnedTestServer(t, cfg)
	if err != nil {
		t.Fatalf("failed Start leaked ownership: %v", err)
	}
	if err := second.Stop(context.Background()); err != nil {
		t.Fatal(err)
	}
	if err := second.Stop(context.Background()); err != nil {
		t.Fatalf("repeated Stop failed: %v", err)
	}
	third, err := newOwnedTestServer(t, cfg)
	if err != nil {
		t.Fatalf("normal Stop leaked ownership: %v", err)
	}
	if err := third.Start(context.Background()); err != nil {
		t.Fatal(err)
	}
	if err := third.Start(context.Background()); err == nil {
		t.Fatal("server started twice")
	}
	other, err := New(cfg)
	if other != nil {
		stopReplServer(other)
	}
	if !errors.Is(err, persistence.ErrDataDirectoryLocked) {
		t.Fatalf("second Start released live ownership: %v", err)
	}
}

func TestDataDirectoryConstructorFailureReleasesOwnership(t *testing.T) {
	cfg := aofTxConfig(t)
	path := filepath.Join(cfg.Persistence.DataDir, "appendonly.aof")
	if err := os.WriteFile(path, encodeArgs("UNKNOWN.REPLAY"), 0600); err != nil {
		t.Fatal(err)
	}
	if _, err := New(cfg); err == nil {
		t.Fatal("bad AOF accepted")
	}
	if err := os.WriteFile(path, nil, 0600); err != nil {
		t.Fatal(err)
	}
	if _, err := newOwnedTestServer(t, cfg); err != nil {
		t.Fatalf("constructor failure leaked ownership: %v", err)
	}
}

func TestDataDirectoryHeldUntilSnapshotWorkerCompletes(t *testing.T) {
	cfg := snapshotConfig(t, t.TempDir())
	s, err := newOwnedTestServer(t, cfg)
	if err != nil {
		t.Fatal(err)
	}
	if err := s.store.Set("key", &store.StringValue{Data: []byte("v")}, store.SetOptions{}); err != nil {
		t.Fatal(err)
	}
	entered, release := make(chan struct{}), make(chan struct{})
	write := s.snapshotWrite
	s.snapshotWrite = func(data []byte) error { close(entered); <-release; return write(data) }
	if err := s.saveSnapshot(true); err != nil {
		t.Fatal(err)
	}
	<-entered
	done := make(chan error, 1)
	go func() { done <- s.Stop(context.Background()) }()
	second, err := New(cfg)
	if second != nil {
		_ = second.Stop(context.Background())
	}
	close(release)
	if stopErr := <-done; stopErr != nil {
		t.Fatal(stopErr)
	}
	if !errors.Is(err, persistence.ErrDataDirectoryLocked) {
		t.Fatalf("snapshot worker lost directory ownership: %v", err)
	}
	if _, err := newOwnedTestServer(t, cfg); err != nil {
		t.Fatalf("finished worker kept ownership: %v", err)
	}
}
