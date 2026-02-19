package store

import (
	"fmt"
	"sync"
	"time"
)

var (
	ErrNamespaceNotFound = fmt.Errorf("namespace not found")
	ErrNamespaceExists   = fmt.Errorf("namespace already exists")
)

type Namespace struct {
	Name      string
	Store     *Store
	Tags      *TagIndex
	CreatedAt time.Time
}

type NamespaceManager struct {
	mu         sync.RWMutex
	namespaces map[string]*Namespace
	defaultNS  *Namespace
}

func NewNamespaceManager() *NamespaceManager {
	nm := &NamespaceManager{
		namespaces: make(map[string]*Namespace),
	}

	nm.defaultNS = nm.createNamespace("default")
	nm.namespaces["default"] = nm.defaultNS

	return nm
}

func (nm *NamespaceManager) createNamespace(name string) *Namespace {
	return &Namespace{
		Name:      name,
		Store:     NewStore(),
		Tags:      NewTagIndex(),
		CreatedAt: time.Now(),
	}
}

func (nm *NamespaceManager) Get(name string) *Namespace {
	nm.mu.RLock()
	defer nm.mu.RUnlock()
	return nm.namespaces[name]
}

func (nm *NamespaceManager) GetOrCreate(name string) *Namespace {
	nm.mu.Lock()
	defer nm.mu.Unlock()

	if ns, exists := nm.namespaces[name]; exists {
		return ns
	}

	ns := nm.createNamespace(name)
	nm.namespaces[name] = ns
	return ns
}

func (nm *NamespaceManager) Delete(name string) error {
	nm.mu.Lock()
	defer nm.mu.Unlock()

	if name == "default" {
		return fmt.Errorf("cannot delete default namespace")
	}

	if _, exists := nm.namespaces[name]; !exists {
		return ErrNamespaceNotFound
	}

	delete(nm.namespaces, name)
	return nil
}

func (nm *NamespaceManager) List() []string {
	nm.mu.RLock()
	defer nm.mu.RUnlock()

	names := make([]string, 0, len(nm.namespaces))
	for name := range nm.namespaces {
		names = append(names, name)
	}
	return names
}

func (nm *NamespaceManager) Default() *Namespace {
	return nm.defaultNS
}

func (nm *NamespaceManager) Flush(name string) error {
	nm.mu.RLock()
	ns, exists := nm.namespaces[name]
	nm.mu.RUnlock()

	if !exists {
		return ErrNamespaceNotFound
	}

	ns.Store.Flush()
	return nil
}

func (nm *NamespaceManager) FlushAll() {
	nm.mu.RLock()
	defer nm.mu.RUnlock()

	for _, ns := range nm.namespaces {
		ns.Store.Flush()
	}
}

func (nm *NamespaceManager) Stats(name string) (map[string]interface{}, error) {
	nm.mu.RLock()
	ns, exists := nm.namespaces[name]
	nm.mu.RUnlock()

	if !exists {
		return nil, ErrNamespaceNotFound
	}

	return map[string]interface{}{
		"name":       ns.Name,
		"keys":       ns.Store.KeyCount(),
		"memory":     ns.Store.MemUsage(),
		"created_at": ns.CreatedAt,
	}, nil
}
