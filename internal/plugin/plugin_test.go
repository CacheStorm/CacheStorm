package plugin

import (
	"errors"
	"testing"

	"github.com/cachestorm/cachestorm/internal/command"
)

type mockPlugin struct {
	name    string
	version string
	initErr error
}

func (m *mockPlugin) Name() string                  { return m.name }
func (m *mockPlugin) Version() string               { return m.version }
func (m *mockPlugin) Init(config interface{}) error { return m.initErr }
func (m *mockPlugin) Close() error                  { return nil }

type mockBeforeHook struct {
	mockPlugin
	called bool
}

func (m *mockBeforeHook) BeforeCommand(ctx *command.Context) error {
	m.called = true
	return nil
}

type mockAfterHook struct {
	mockPlugin
	called bool
}

func (m *mockAfterHook) AfterCommand(ctx *command.Context) {
	m.called = true
}

type mockEvictHook struct {
	mockPlugin
	called bool
}

func (m *mockEvictHook) OnEvict(key string, value interface{}) {
	m.called = true
}

type mockExpireHook struct {
	mockPlugin
	called bool
}

func (m *mockExpireHook) OnExpire(key string, value interface{}) {
	m.called = true
}

type mockTagHook struct {
	mockPlugin
	called bool
}

func (m *mockTagHook) OnTagInvalidate(tag string, keys []string) {
	m.called = true
}

type mockStartupHook struct {
	mockPlugin
	called bool
}

func (m *mockStartupHook) OnStartup() error {
	m.called = true
	return nil
}

type mockShutdownHook struct {
	mockPlugin
	called bool
}

func (m *mockShutdownHook) OnShutdown() error {
	m.called = true
	return nil
}

func TestNewManager(t *testing.T) {
	m := NewManager()

	if m == nil {
		t.Fatal("expected manager")
	}

	if m.plugins == nil {
		t.Error("expected plugins slice")
	}
}

func TestManagerRegister(t *testing.T) {
	m := NewManager()
	p := &mockPlugin{name: "test", version: "1.0"}

	err := m.Register(p)
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}

	if len(m.plugins) != 1 {
		t.Errorf("expected 1 plugin, got %d", len(m.plugins))
	}
}

func TestManagerInitAll(t *testing.T) {
	m := NewManager()
	p := &mockPlugin{name: "test", version: "1.0"}
	m.Register(p)

	err := m.InitAll(map[string]interface{}{"test": nil})
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestManagerInitAllError(t *testing.T) {
	m := NewManager()
	p := &mockPlugin{name: "test", version: "1.0", initErr: errors.New("init error")}
	m.Register(p)

	err := m.InitAll(map[string]interface{}{"test": nil})
	if err == nil {
		t.Error("expected error")
	}
}

func TestManagerCloseAll(t *testing.T) {
	m := NewManager()
	p := &mockPlugin{name: "test", version: "1.0"}
	m.Register(p)

	err := m.CloseAll()
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestManagerRunBeforeHooks(t *testing.T) {
	m := NewManager()
	h := &mockBeforeHook{mockPlugin: mockPlugin{name: "test"}}
	m.Register(h)

	err := m.RunBeforeHooks(nil)
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}

	if !h.called {
		t.Error("hook should be called")
	}
}

func TestManagerRunAfterHooks(t *testing.T) {
	m := NewManager()
	h := &mockAfterHook{mockPlugin: mockPlugin{name: "test"}}
	m.Register(h)

	m.RunAfterHooks(nil)

	if !h.called {
		t.Error("hook should be called")
	}
}

func TestManagerRunEvictHooks(t *testing.T) {
	m := NewManager()
	h := &mockEvictHook{mockPlugin: mockPlugin{name: "test"}}
	m.Register(h)

	m.RunEvictHooks("key", "value")

	if !h.called {
		t.Error("hook should be called")
	}
}

func TestManagerRunExpireHooks(t *testing.T) {
	m := NewManager()
	h := &mockExpireHook{mockPlugin: mockPlugin{name: "test"}}
	m.Register(h)

	m.RunExpireHooks("key", "value")

	if !h.called {
		t.Error("hook should be called")
	}
}

func TestManagerRunTagInvalidateHooks(t *testing.T) {
	m := NewManager()
	h := &mockTagHook{mockPlugin: mockPlugin{name: "test"}}
	m.Register(h)

	m.RunTagInvalidateHooks("tag", []string{"key1", "key2"})

	if !h.called {
		t.Error("hook should be called")
	}
}

func TestManagerRunStartupHooks(t *testing.T) {
	m := NewManager()
	h := &mockStartupHook{mockPlugin: mockPlugin{name: "test"}}
	m.Register(h)

	err := m.RunStartupHooks()
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}

	if !h.called {
		t.Error("hook should be called")
	}
}

func TestManagerRunShutdownHooks(t *testing.T) {
	m := NewManager()
	h := &mockShutdownHook{mockPlugin: mockPlugin{name: "test"}}
	m.Register(h)

	err := m.RunShutdownHooks()
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}

	if !h.called {
		t.Error("hook should be called")
	}
}

func TestManagerPlugins(t *testing.T) {
	m := NewManager()
	p1 := &mockPlugin{name: "test1"}
	p2 := &mockPlugin{name: "test2"}
	m.Register(p1)
	m.Register(p2)

	plugins := m.Plugins()
	if len(plugins) != 2 {
		t.Errorf("expected 2 plugins, got %d", len(plugins))
	}
}

func TestHTTPRoute(t *testing.T) {
	route := HTTPRoute{
		Method:  "GET",
		Path:    "/test",
		Handler: nil,
	}

	if route.Method != "GET" {
		t.Errorf("expected GET, got %s", route.Method)
	}

	if route.Path != "/test" {
		t.Errorf("expected /test, got %s", route.Path)
	}
}

func TestMultipleHooks(t *testing.T) {
	m := NewManager()
	h1 := &mockBeforeHook{mockPlugin: mockPlugin{name: "test1"}}
	h2 := &mockBeforeHook{mockPlugin: mockPlugin{name: "test2"}}
	m.Register(h1)
	m.Register(h2)

	m.RunBeforeHooks(nil)

	if !h1.called || !h2.called {
		t.Error("both hooks should be called")
	}
}

func TestPluginWithMultipleHooks(t *testing.T) {
	m := NewManager()

	plugin := &struct {
		mockPlugin
		mockBeforeHook
		mockAfterHook
	}{
		mockPlugin:     mockPlugin{name: "multi"},
		mockBeforeHook: mockBeforeHook{},
		mockAfterHook:  mockAfterHook{},
	}

	m.Register(plugin)

	m.RunBeforeHooks(nil)
	m.RunAfterHooks(nil)

	if !plugin.mockBeforeHook.called {
		t.Error("before hook should be called")
	}

	if !plugin.mockAfterHook.called {
		t.Error("after hook should be called")
	}
}
