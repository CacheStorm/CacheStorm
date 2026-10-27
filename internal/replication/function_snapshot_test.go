package replication

import (
	"bufio"
	"bytes"
	"errors"
	"fmt"
	"testing"
)

func TestFullSyncFunctionExportFailureReleasesGate(t *testing.T) {
	m, _ := newTestManager(t, Config{})
	h := m.host.(*fakeHost)
	failure := errors.New("injected function export failure")
	h.functionError = failure
	if err := m.fullSync(&replica{}, false); !errors.Is(err, failure) {
		t.Fatalf("fullSync = %v", err)
	}
	if !h.mu.TryLock() {
		t.Fatal("failed function export retained the write gate")
	}
	h.mu.Unlock()
}

func TestFullSyncPersistenceFailureDoesNotCachePosition(t *testing.T) {
	m, _ := newTestManager(t, Config{})
	h := m.host.(*fakeHost)
	failure := errors.New("injected snapshot persistence failure")
	h.persistError = failure
	payload := encodeCommand("SET", "k", "v")
	var input bytes.Buffer
	fmt.Fprintf(&input, "$%d\r\n", len(payload))
	input.Write(payload)
	err := m.loadSnapshot(bufio.NewReader(&input), "0123456789012345678901234567890123456789", 1234)
	if !errors.Is(err, failure) {
		t.Fatalf("loadSnapshot = %v, want persistence error", err)
	}
	m.mu.Lock()
	cached, offset := m.cached, m.offset
	m.mu.Unlock()
	if cached || offset == 1234 {
		t.Fatalf("failed snapshot accepted as durable: cached=%v offset=%d", cached, offset)
	}
}

func TestFullSyncRejectsCommandErrorReplies(t *testing.T) {
	for _, invalid := range []bool{false, true} {
		m, _ := newTestManager(t, Config{})
		payload := encodeCommand("SET", "k", "not-an-integer")
		if invalid {
			payload = append(payload, encodeCommand("INCR", "k")...)
		}
		var input bytes.Buffer
		fmt.Fprintf(&input, "$%d\r\n", len(payload))
		input.Write(payload)
		err := m.loadSnapshot(bufio.NewReader(&input), "0123456789012345678901234567890123456789", 1234)
		if (err != nil) != invalid {
			t.Fatalf("invalid=%v: loadSnapshot=%v", invalid, err)
		}
		m.mu.Lock()
		cached := m.cached
		m.mu.Unlock()
		if cached == invalid {
			t.Fatalf("invalid=%v: cached=%v", invalid, cached)
		}
	}
}
