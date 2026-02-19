package plugin

import (
	"sync"

	"github.com/cachestorm/cachestorm/internal/command"
)

type Manager struct {
	mu            sync.RWMutex
	plugins       []Plugin
	beforeHooks   []BeforeCommandHook
	afterHooks    []AfterCommandHook
	evictHooks    []OnEvictHook
	expireHooks   []OnExpireHook
	tagHooks      []OnTagInvalidateHook
	startupHooks  []OnStartupHook
	shutdownHooks []OnShutdownHook
}

func NewManager() *Manager {
	return &Manager{
		plugins:       make([]Plugin, 0),
		beforeHooks:   make([]BeforeCommandHook, 0),
		afterHooks:    make([]AfterCommandHook, 0),
		evictHooks:    make([]OnEvictHook, 0),
		expireHooks:   make([]OnExpireHook, 0),
		tagHooks:      make([]OnTagInvalidateHook, 0),
		startupHooks:  make([]OnStartupHook, 0),
		shutdownHooks: make([]OnShutdownHook, 0),
	}
}

func (m *Manager) Register(p Plugin) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.plugins = append(m.plugins, p)

	if hook, ok := p.(BeforeCommandHook); ok {
		m.beforeHooks = append(m.beforeHooks, hook)
	}
	if hook, ok := p.(AfterCommandHook); ok {
		m.afterHooks = append(m.afterHooks, hook)
	}
	if hook, ok := p.(OnEvictHook); ok {
		m.evictHooks = append(m.evictHooks, hook)
	}
	if hook, ok := p.(OnExpireHook); ok {
		m.expireHooks = append(m.expireHooks, hook)
	}
	if hook, ok := p.(OnTagInvalidateHook); ok {
		m.tagHooks = append(m.tagHooks, hook)
	}
	if hook, ok := p.(OnStartupHook); ok {
		m.startupHooks = append(m.startupHooks, hook)
	}
	if hook, ok := p.(OnShutdownHook); ok {
		m.shutdownHooks = append(m.shutdownHooks, hook)
	}

	return nil
}

func (m *Manager) InitAll(configs map[string]interface{}) error {
	m.mu.RLock()
	defer m.mu.RUnlock()

	for _, p := range m.plugins {
		cfg := configs[p.Name()]
		if err := p.Init(cfg); err != nil {
			return err
		}
	}
	return nil
}

func (m *Manager) CloseAll() error {
	m.mu.RLock()
	defer m.mu.RUnlock()

	for i := len(m.plugins) - 1; i >= 0; i-- {
		m.plugins[i].Close()
	}
	return nil
}

func (m *Manager) RunBeforeHooks(ctx *command.Context) error {
	m.mu.RLock()
	defer m.mu.RUnlock()

	for _, hook := range m.beforeHooks {
		if err := hook.BeforeCommand(ctx); err != nil {
			return err
		}
	}
	return nil
}

func (m *Manager) RunAfterHooks(ctx *command.Context) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	for _, hook := range m.afterHooks {
		hook.AfterCommand(ctx)
	}
}

func (m *Manager) RunEvictHooks(key string, value interface{}) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	for _, hook := range m.evictHooks {
		hook.OnEvict(key, value)
	}
}

func (m *Manager) RunExpireHooks(key string, value interface{}) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	for _, hook := range m.expireHooks {
		hook.OnExpire(key, value)
	}
}

func (m *Manager) RunTagInvalidateHooks(tag string, keys []string) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	for _, hook := range m.tagHooks {
		hook.OnTagInvalidate(tag, keys)
	}
}

func (m *Manager) RunStartupHooks() error {
	m.mu.RLock()
	defer m.mu.RUnlock()

	for _, hook := range m.startupHooks {
		if err := hook.OnStartup(); err != nil {
			return err
		}
	}
	return nil
}

func (m *Manager) RunShutdownHooks() error {
	m.mu.RLock()
	defer m.mu.RUnlock()

	for _, hook := range m.shutdownHooks {
		if err := hook.OnShutdown(); err != nil {
			return err
		}
	}
	return nil
}

func (m *Manager) Plugins() []Plugin {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.plugins
}
