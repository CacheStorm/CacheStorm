package server

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/cachestorm/cachestorm/internal/config"
	"github.com/cachestorm/cachestorm/internal/persistence"
)

func startupCleanupWriterX(t *testing.T) *persistence.AOFWriter {
	t.Helper()
	w := persistence.NewAOFWriter(persistence.AOFConfig{Enabled: true, DataDir: t.TempDir(), Filename: "proof.aof", SyncPolicy: persistence.AOFEverySecond})
	t.Cleanup(w.Stop)
	return w
}

func startupCleanupLateAppendX(t *testing.T, w *persistence.AOFWriter, stop func()) (int64, bool) {
	t.Helper()
	ready := make(chan struct{})
	release := make(chan struct{})
	done := make(chan error, 1)
	before := w.Dirty()
	go func() {
		close(ready)
		<-release
		done <- w.Append("SET", [][]byte{[]byte("late"), []byte("value")})
	}()
	<-ready
	stop()
	close(release)
	if err := <-done; err != nil {
		t.Fatalf("append: %v", err)
	}
	return w.Dirty() - before, errors.Is(w.Flush(), os.ErrClosed)
}

func startupCleanupFailureX(t *testing.T, mode string, attempts int) bool {
	t.Helper()
	w := startupCleanupWriterX(t)
	cfg := &config.Config{Server: config.ServerConfig{Bind: "127.0.0.1", Port: -1}}
	if mode == "missing TLS" {
		cfg.Server.TLSCertFile = filepath.Join(t.TempDir(), "missing.crt")
		cfg.Server.TLSKeyFile = filepath.Join(t.TempDir(), "missing.key")
	}
	if mode == "malformed TLS" {
		cfg.Server.TLSCertFile = filepath.Join(t.TempDir(), "bad.crt")
		cfg.Server.TLSKeyFile = filepath.Join(t.TempDir(), "bad.key")
		for _, name := range []string{cfg.Server.TLSCertFile, cfg.Server.TLSKeyFile} {
			if err := os.WriteFile(name, []byte("invalid PEM"), 0600); err != nil {
				t.Fatal(err)
			}
		}
	}
	s := &Server{cfg: cfg, aof: w, stopCh: make(chan struct{})}
	ok := true
	for i := 0; i < attempts; i++ {
		delta, closed := startupCleanupLateAppendX(t, w, func() {
			if err := s.Start(context.Background()); err == nil {
				t.Fatal("expected startup error")
			}
		})
		t.Logf("%s attempt %d EXPECTED: dirty delta 0, file closed true | ACTUAL: dirty delta %d, file closed %v", mode, i+1, delta, closed)
		ok = ok && delta == 0 && closed
		select {
		case <-s.stopCh:
			t.Fatal("startup failure permanently stopped server")
		default:
		}
	}
	if err := w.Start(); err != nil {
		t.Fatalf("writer restart: %v", err)
	}
	before := w.Dirty()
	if err := w.Append("SET", [][]byte{[]byte("restart"), []byte("value")}); err != nil {
		t.Fatal(err)
	}
	t.Logf("restart control EXPECTED: dirty delta 1 | ACTUAL: %d", w.Dirty()-before)
	if w.Dirty()-before != 1 {
		t.Fatal("writer cannot restart")
	}
	w.Stop()
	return ok
}

func TestStartupFailureReleasesAOFX(t *testing.T) {
	w := startupCleanupWriterX(t)
	if err := w.Start(); err != nil {
		t.Fatal(err)
	}
	if err := w.Append("SET", [][]byte{[]byte("control"), []byte("value")}); err != nil {
		t.Fatal(err)
	}
	t.Logf("control: running writer EXPECTED: dirty 1 | ACTUAL: %d", w.Dirty())
	if w.Dirty() != 1 {
		t.Fatal("control failed")
	}
	delta, closed := startupCleanupLateAppendX(t, w, w.Stop)
	t.Logf("control: explicit Stop EXPECTED: dirty delta 0, file closed true | ACTUAL: %d, %v", delta, closed)
	if delta != 0 || !closed {
		t.Fatal("stop control failed")
	}
	ok := startupCleanupFailureX(t, "missing TLS", 1)
	ok = startupCleanupFailureX(t, "invalid port", 1) && ok
	ok = startupCleanupFailureX(t, "malformed TLS", 1) && ok
	ok = startupCleanupFailureX(t, "missing TLS", 3) && ok
	if !ok {
		t.Fatal("startup failure left the persistence writer active")
	}
}
