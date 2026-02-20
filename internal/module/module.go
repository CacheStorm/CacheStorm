package module

import (
	"fmt"
	"sync"
)

type Module interface {
	Name() string
	Version() string
	Init(ctx *Context) error
	Shutdown() error
	Commands() []CommandDef
}

type CommandDef struct {
	Name    string
	Handler CommandHandler
	Flags   CommandFlags
}

type CommandHandler func(ctx *CommandContext) error

type CommandFlags struct {
	Write    bool
	ReadOnly bool
	Admin    bool
	NOSCRIPT bool
}

type CommandContext struct {
	Args   [][]byte
	Store  Store
	Writer Writer
	Client ClientInfo
}

type Store interface {
	Get(key string) ([]byte, bool)
	Set(key string, value []byte) error
	Delete(key string) bool
	Exists(key string) bool
}

type Writer interface {
	WriteOK() error
	WriteError(msg string) error
	WriteString(s string) error
	WriteBytes(b []byte) error
	WriteInteger(n int64) error
	WriteArray(items []interface{}) error
	WriteNull() error
}

type ClientInfo struct {
	ID      int64
	Address string
}

type Context struct {
	config map[string]string
}

func NewContext(config map[string]string) *Context {
	return &Context{config: config}
}

func (c *Context) GetConfig(key string) string {
	return c.config[key]
}

type Registry struct {
	mu      sync.RWMutex
	modules map[string]Module
	loaded  map[string]bool
}

var globalRegistry = NewRegistry()

func NewRegistry() *Registry {
	return &Registry{
		modules: make(map[string]Module),
		loaded:  make(map[string]bool),
	}
}

func GetRegistry() *Registry {
	return globalRegistry
}

func (r *Registry) Register(m Module) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	if _, exists := r.modules[m.Name()]; exists {
		return fmt.Errorf("module '%s' already registered", m.Name())
	}

	r.modules[m.Name()] = m
	return nil
}

func (r *Registry) Load(name string, ctx *Context) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	m, exists := r.modules[name]
	if !exists {
		return fmt.Errorf("module '%s' not found", name)
	}

	if r.loaded[name] {
		return fmt.Errorf("module '%s' already loaded", name)
	}

	if err := m.Init(ctx); err != nil {
		return fmt.Errorf("failed to initialize module '%s': %v", name, err)
	}

	r.loaded[name] = true
	return nil
}

func (r *Registry) Unload(name string) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	m, exists := r.modules[name]
	if !exists {
		return fmt.Errorf("module '%s' not found", name)
	}

	if !r.loaded[name] {
		return fmt.Errorf("module '%s' not loaded", name)
	}

	if err := m.Shutdown(); err != nil {
		return fmt.Errorf("failed to shutdown module '%s': %v", name, err)
	}

	r.loaded[name] = false
	return nil
}

func (r *Registry) GetModule(name string) (Module, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	m, ok := r.modules[name]
	return m, ok
}

func (r *Registry) ListModules() []ModuleInfo {
	r.mu.RLock()
	defer r.mu.RUnlock()

	result := make([]ModuleInfo, 0, len(r.modules))
	for name, m := range r.modules {
		result = append(result, ModuleInfo{
			Name:    name,
			Version: m.Version(),
			Loaded:  r.loaded[name],
		})
	}
	return result
}

func (r *Registry) GetCommands() []CommandDef {
	r.mu.RLock()
	defer r.mu.RUnlock()

	var commands []CommandDef
	for name, m := range r.modules {
		if r.loaded[name] {
			commands = append(commands, m.Commands()...)
		}
	}
	return commands
}

type ModuleInfo struct {
	Name    string
	Version string
	Loaded  bool
}

type BaseModule struct {
	name     string
	version  string
	commands []CommandDef
}

func NewBaseModule(name, version string) *BaseModule {
	return &BaseModule{
		name:     name,
		version:  version,
		commands: make([]CommandDef, 0),
	}
}

func (m *BaseModule) Name() string {
	return m.name
}

func (m *BaseModule) Version() string {
	return m.version
}

func (m *BaseModule) Init(ctx *Context) error {
	return nil
}

func (m *BaseModule) Shutdown() error {
	return nil
}

func (m *BaseModule) Commands() []CommandDef {
	return m.commands
}

func (m *BaseModule) AddCommand(name string, handler CommandHandler, flags CommandFlags) {
	m.commands = append(m.commands, CommandDef{
		Name:    name,
		Handler: handler,
		Flags:   flags,
	})
}
