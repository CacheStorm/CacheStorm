package module

import (
	"testing"
)

type mockModule struct {
	name     string
	version  string
	initErr  error
	shutErr  error
	commands []CommandDef
}

func (m *mockModule) Name() string            { return m.name }
func (m *mockModule) Version() string         { return m.version }
func (m *mockModule) Init(ctx *Context) error { return m.initErr }
func (m *mockModule) Shutdown() error         { return m.shutErr }
func (m *mockModule) Commands() []CommandDef  { return m.commands }

func TestNewContext(t *testing.T) {
	config := map[string]string{"key": "value"}
	ctx := NewContext(config)

	if ctx == nil {
		t.Fatal("expected context")
	}

	if ctx.GetConfig("key") != "value" {
		t.Errorf("expected value, got %s", ctx.GetConfig("key"))
	}
}

func TestContextGetConfig(t *testing.T) {
	ctx := NewContext(map[string]string{"foo": "bar"})

	if ctx.GetConfig("foo") != "bar" {
		t.Errorf("expected bar, got %s", ctx.GetConfig("foo"))
	}

	if ctx.GetConfig("nonexistent") != "" {
		t.Error("expected empty string for nonexistent key")
	}
}

func TestNewRegistry(t *testing.T) {
	r := NewRegistry()

	if r == nil {
		t.Fatal("expected registry")
	}

	if r.modules == nil {
		t.Error("expected modules map")
	}
}

func TestGetRegistry(t *testing.T) {
	r := GetRegistry()

	if r == nil {
		t.Fatal("expected global registry")
	}
}

func TestRegistryRegister(t *testing.T) {
	r := NewRegistry()
	m := &mockModule{name: "test", version: "1.0"}

	err := r.Register(m)
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestRegistryRegisterDuplicate(t *testing.T) {
	r := NewRegistry()
	m := &mockModule{name: "test", version: "1.0"}

	r.Register(m)
	err := r.Register(m)

	if err == nil {
		t.Error("expected error for duplicate module")
	}
}

func TestRegistryLoad(t *testing.T) {
	r := NewRegistry()
	m := &mockModule{name: "test", version: "1.0"}
	r.Register(m)

	ctx := NewContext(nil)
	err := r.Load("test", ctx)

	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestRegistryLoadNotFound(t *testing.T) {
	r := NewRegistry()
	ctx := NewContext(nil)

	err := r.Load("nonexistent", ctx)
	if err == nil {
		t.Error("expected error for nonexistent module")
	}
}

func TestRegistryLoadAlreadyLoaded(t *testing.T) {
	r := NewRegistry()
	m := &mockModule{name: "test", version: "1.0"}
	r.Register(m)

	ctx := NewContext(nil)
	r.Load("test", ctx)
	err := r.Load("test", ctx)

	if err == nil {
		t.Error("expected error for already loaded module")
	}
}

func TestRegistryUnload(t *testing.T) {
	r := NewRegistry()
	m := &mockModule{name: "test", version: "1.0"}
	r.Register(m)

	ctx := NewContext(nil)
	r.Load("test", ctx)
	err := r.Unload("test")

	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestRegistryUnloadNotFound(t *testing.T) {
	r := NewRegistry()

	err := r.Unload("nonexistent")
	if err == nil {
		t.Error("expected error for nonexistent module")
	}
}

func TestRegistryUnloadNotLoaded(t *testing.T) {
	r := NewRegistry()
	m := &mockModule{name: "test", version: "1.0"}
	r.Register(m)

	err := r.Unload("test")
	if err == nil {
		t.Error("expected error for not loaded module")
	}
}

func TestRegistryGetModule(t *testing.T) {
	r := NewRegistry()
	m := &mockModule{name: "test", version: "1.0"}
	r.Register(m)

	module, ok := r.GetModule("test")
	if !ok {
		t.Error("expected to find module")
	}
	if module.Name() != "test" {
		t.Errorf("expected test, got %s", module.Name())
	}
}

func TestRegistryGetModuleNotFound(t *testing.T) {
	r := NewRegistry()

	_, ok := r.GetModule("nonexistent")
	if ok {
		t.Error("expected not to find module")
	}
}

func TestRegistryListModules(t *testing.T) {
	r := NewRegistry()
	m1 := &mockModule{name: "test1", version: "1.0"}
	m2 := &mockModule{name: "test2", version: "2.0"}
	r.Register(m1)
	r.Register(m2)

	ctx := NewContext(nil)
	r.Load("test1", ctx)

	list := r.ListModules()
	if len(list) != 2 {
		t.Errorf("expected 2 modules, got %d", len(list))
	}
}

func TestRegistryGetCommands(t *testing.T) {
	r := NewRegistry()
	m := &mockModule{
		name:     "test",
		version:  "1.0",
		commands: []CommandDef{{Name: "CMD1"}},
	}
	r.Register(m)

	ctx := NewContext(nil)
	r.Load("test", ctx)

	cmds := r.GetCommands()
	if len(cmds) != 1 {
		t.Errorf("expected 1 command, got %d", len(cmds))
	}
}

func TestNewBaseModule(t *testing.T) {
	m := NewBaseModule("test", "1.0")

	if m.Name() != "test" {
		t.Errorf("expected test, got %s", m.Name())
	}

	if m.Version() != "1.0" {
		t.Errorf("expected 1.0, got %s", m.Version())
	}
}

func TestBaseModuleInit(t *testing.T) {
	m := NewBaseModule("test", "1.0")
	ctx := NewContext(nil)

	err := m.Init(ctx)
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestBaseModuleShutdown(t *testing.T) {
	m := NewBaseModule("test", "1.0")

	err := m.Shutdown()
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestBaseModuleCommands(t *testing.T) {
	m := NewBaseModule("test", "1.0")

	cmds := m.Commands()
	if cmds == nil {
		t.Error("expected commands slice")
	}
}

func TestBaseModuleAddCommand(t *testing.T) {
	m := NewBaseModule("test", "1.0")

	m.AddCommand("TEST", func(ctx *CommandContext) error { return nil }, CommandFlags{})

	cmds := m.Commands()
	if len(cmds) != 1 {
		t.Errorf("expected 1 command, got %d", len(cmds))
	}

	if cmds[0].Name != "TEST" {
		t.Errorf("expected TEST, got %s", cmds[0].Name)
	}
}

func TestCommandFlags(t *testing.T) {
	flags := CommandFlags{
		Write:    true,
		ReadOnly: false,
		Admin:    true,
		NOSCRIPT: true,
	}

	if !flags.Write {
		t.Error("expected Write to be true")
	}

	if flags.ReadOnly {
		t.Error("expected ReadOnly to be false")
	}

	if !flags.Admin {
		t.Error("expected Admin to be true")
	}

	if !flags.NOSCRIPT {
		t.Error("expected NOSCRIPT to be true")
	}
}

func TestModuleInfo(t *testing.T) {
	info := ModuleInfo{
		Name:    "test",
		Version: "1.0",
		Loaded:  true,
	}

	if info.Name != "test" {
		t.Errorf("expected test, got %s", info.Name)
	}

	if info.Version != "1.0" {
		t.Errorf("expected 1.0, got %s", info.Version)
	}

	if !info.Loaded {
		t.Error("expected Loaded to be true")
	}
}
