package persistence

import (
	"os"
	"path/filepath"
	"testing"
)

func TestAOFConfig(t *testing.T) {
	cfg := AOFConfig{
		Enabled:     true,
		Filename:    "test.aof",
		DataDir:     t.TempDir(),
		SyncPolicy:  AOFEverySecond,
		RewriteSize: 1024,
		RewritePct:  100,
		AutoRewrite: true,
	}

	if !cfg.Enabled {
		t.Error("expected enabled")
	}

	if cfg.Filename != "test.aof" {
		t.Errorf("expected test.aof, got %s", cfg.Filename)
	}
}

func TestNewAOFWriter(t *testing.T) {
	cfg := AOFConfig{
		Enabled:  false,
		Filename: "test.aof",
		DataDir:  t.TempDir(),
	}

	w := NewAOFWriter(cfg)
	if w == nil {
		t.Fatal("expected writer")
	}
}

func TestAOFWriterStartDisabled(t *testing.T) {
	cfg := AOFConfig{
		Enabled:  false,
		Filename: "test.aof",
		DataDir:  t.TempDir(),
	}

	w := NewAOFWriter(cfg)
	err := w.Start()
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestAOFWriterStartEnabled(t *testing.T) {
	tmpDir := t.TempDir()
	cfg := AOFConfig{
		Enabled:     true,
		Filename:    "test.aof",
		DataDir:     tmpDir,
		SyncPolicy:  AOFNoSync,
		RewriteSize: 1024,
	}

	w := NewAOFWriter(cfg)
	err := w.Start()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	w.Stop()
}

func TestAOFWriterStopTwice(t *testing.T) {
	cfg := AOFConfig{Enabled: false}
	w := NewAOFWriter(cfg)
	w.Stop()
	w.Stop()
}

func TestAOFWriterAppendDisabled(t *testing.T) {
	cfg := AOFConfig{Enabled: false}
	w := NewAOFWriter(cfg)

	err := w.Append("SET", [][]byte{[]byte("key"), []byte("value")})
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestAOFWriterAppendEnabled(t *testing.T) {
	tmpDir := t.TempDir()
	cfg := AOFConfig{
		Enabled:    true,
		Filename:   "test.aof",
		DataDir:    tmpDir,
		SyncPolicy: AOFNoSync,
	}

	w := NewAOFWriter(cfg)
	if err := w.Start(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	err := w.Append("SET", [][]byte{[]byte("key"), []byte("value")})
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}

	w.Stop()
}

func TestAOFWriterSize(t *testing.T) {
	w := NewAOFWriter(AOFConfig{Enabled: false})

	size := w.Size()
	if size != 0 {
		t.Errorf("expected 0, got %d", size)
	}
}

func TestAOFWriterDirty(t *testing.T) {
	w := NewAOFWriter(AOFConfig{Enabled: false})

	dirty := w.Dirty()
	if dirty != 0 {
		t.Errorf("expected 0, got %d", dirty)
	}
}

func TestAOFWriterFlush(t *testing.T) {
	w := NewAOFWriter(AOFConfig{Enabled: false})

	err := w.Flush()
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestNewAOFReader(t *testing.T) {
	r := NewAOFReader()
	if r == nil {
		t.Fatal("expected reader")
	}
}

func TestAOFReaderLoadEmpty(t *testing.T) {
	tmpDir := t.TempDir()
	path := filepath.Join(tmpDir, "empty.aof")

	if err := os.WriteFile(path, []byte{}, 0644); err != nil {
		t.Fatalf("failed to write file: %v", err)
	}

	r := NewAOFReader()
	cmds, err := r.Load(path)
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}

	if len(cmds) != 0 {
		t.Errorf("expected 0 commands, got %d", len(cmds))
	}
}

func TestAOFReaderLoadNonExistent(t *testing.T) {
	r := NewAOFReader()
	_, err := r.Load("/nonexistent/path.aof")
	if err == nil {
		t.Error("expected error for non-existent file")
	}
}

func TestCommand(t *testing.T) {
	cmd := Command{
		Name: "SET",
		Args: [][]byte{[]byte("key"), []byte("value")},
	}

	if cmd.Name != "SET" {
		t.Errorf("expected SET, got %s", cmd.Name)
	}

	if len(cmd.Args) != 2 {
		t.Errorf("expected 2 args, got %d", len(cmd.Args))
	}
}

func TestSyncPolicyFromString(t *testing.T) {
	tests := []struct {
		input    string
		expected AOFSyncPolicy
	}{
		{"always", AOFAlways},
		{"ALWAYS", AOFAlways},
		{"everysec", AOFEverySecond},
		{"EVERYSEC", AOFEverySecond},
		{"no", AOFNoSync},
		{"none", AOFNoSync},
		{"unknown", AOFEverySecond},
		{"", AOFEverySecond},
	}

	for _, tt := range tests {
		result := SyncPolicyFromString(tt.input)
		if result != tt.expected {
			t.Errorf("SyncPolicyFromString(%s) = %v, expected %v", tt.input, result, tt.expected)
		}
	}
}

type mockStore struct {
	data map[string]interface{}
}

func (m *mockStore) GetAll() map[string]interface{} {
	return m.data
}

func TestNewAOFRewriter(t *testing.T) {
	store := &mockStore{data: make(map[string]interface{})}
	cfg := AOFConfig{RewriteSize: 1024, RewritePct: 100}

	rw := NewAOFRewriter(cfg, store)
	if rw == nil {
		t.Fatal("expected rewriter")
	}
}

func TestAOFRewriterIsRewriting(t *testing.T) {
	store := &mockStore{data: make(map[string]interface{})}
	rw := NewAOFRewriter(AOFConfig{}, store)

	if rw.IsRewriting() {
		t.Error("expected not rewriting")
	}
}

func TestAOFRewriterShouldRewrite(t *testing.T) {
	store := &mockStore{data: make(map[string]interface{})}
	cfg := AOFConfig{
		RewriteSize: 100,
		RewritePct:  50,
	}
	rw := NewAOFRewriter(cfg, store)

	if rw.ShouldRewrite(50) {
		t.Error("should not rewrite when size < RewriteSize")
	}

	if !rw.ShouldRewrite(100) {
		t.Error("should rewrite when size >= RewriteSize")
	}
}

func TestAOFRewriterShouldRewriteInProgress(t *testing.T) {
	store := &mockStore{data: make(map[string]interface{})}
	cfg := AOFConfig{RewriteSize: 100}
	rw := NewAOFRewriter(cfg, store)

	rw.rewriting.Store(true)
	if rw.ShouldRewrite(1000) {
		t.Error("should not rewrite when already in progress")
	}
}

func TestNewAOFManager(t *testing.T) {
	store := &mockStore{data: make(map[string]interface{})}
	cfg := AOFConfig{Enabled: false}

	m := NewAOFManager(cfg, store)
	if m == nil {
		t.Fatal("expected manager")
	}
}

func TestAOFManagerStart(t *testing.T) {
	store := &mockStore{data: make(map[string]interface{})}
	cfg := AOFConfig{Enabled: false}

	m := NewAOFManager(cfg, store)
	err := m.Start()
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	m.Stop()
}

func TestAOFManagerAppend(t *testing.T) {
	store := &mockStore{data: make(map[string]interface{})}
	cfg := AOFConfig{Enabled: false}

	m := NewAOFManager(cfg, store)
	err := m.Append("SET", [][]byte{[]byte("key"), []byte("value")})
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestAOFManagerSize(t *testing.T) {
	store := &mockStore{data: make(map[string]interface{})}
	cfg := AOFConfig{Enabled: false}

	m := NewAOFManager(cfg, store)
	size := m.Size()
	if size != 0 {
		t.Errorf("expected 0, got %d", size)
	}
}

func TestAOFManagerDirty(t *testing.T) {
	store := &mockStore{data: make(map[string]interface{})}
	cfg := AOFConfig{Enabled: false}

	m := NewAOFManager(cfg, store)
	dirty := m.Dirty()
	if dirty != 0 {
		t.Errorf("expected 0, got %d", dirty)
	}
}

func TestAOFManagerFlush(t *testing.T) {
	store := &mockStore{data: make(map[string]interface{})}
	cfg := AOFConfig{Enabled: false}

	m := NewAOFManager(cfg, store)
	err := m.Flush()
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestAOFManagerIsRewriting(t *testing.T) {
	store := &mockStore{data: make(map[string]interface{})}
	cfg := AOFConfig{Enabled: false}

	m := NewAOFManager(cfg, store)
	if m.IsRewriting() {
		t.Error("expected not rewriting")
	}
}

func TestAOFManagerInfo(t *testing.T) {
	store := &mockStore{data: make(map[string]interface{})}
	cfg := AOFConfig{Enabled: true}

	m := NewAOFManager(cfg, store)
	info := m.Info()

	if info == nil {
		t.Fatal("expected info")
	}

	if info["aof_enabled"] != true {
		t.Error("expected aof_enabled to be true")
	}
}

func TestAOFWriterSyncPolicyAlways(t *testing.T) {
	tmpDir := t.TempDir()
	cfg := AOFConfig{
		Enabled:    true,
		Filename:   "test.aof",
		DataDir:    tmpDir,
		SyncPolicy: AOFAlways,
	}

	w := NewAOFWriter(cfg)
	if err := w.Start(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	err := w.Append("SET", [][]byte{[]byte("key"), []byte("value")})
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}

	w.Stop()
}

func TestAOFWriterSyncPolicyEverySecond(t *testing.T) {
	tmpDir := t.TempDir()
	cfg := AOFConfig{
		Enabled:    true,
		Filename:   "test.aof",
		DataDir:    tmpDir,
		SyncPolicy: AOFEverySecond,
	}

	w := NewAOFWriter(cfg)
	if err := w.Start(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	w.Stop()
}

func TestAOFSyncPolicyConstants(t *testing.T) {
	if AOFNoSync != 0 {
		t.Errorf("expected AOFNoSync = 0, got %d", AOFNoSync)
	}

	if AOFEverySecond != 1 {
		t.Errorf("expected AOFEverySecond = 1, got %d", AOFEverySecond)
	}

	if AOFAlways != 2 {
		t.Errorf("expected AOFAlways = 2, got %d", AOFAlways)
	}
}
