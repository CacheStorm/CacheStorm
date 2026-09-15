// Package examplehooks is a reference implementation of a hook-consuming
// CacheStorm plugin. It implements the base Plugin contract plus the store
// event hooks — eviction, expiry, and tag invalidation — and records every
// event it receives, demonstrating the complete contract a consuming plugin
// must satisfy before plugin.Manager dispatchers reach it.
package examplehooks

import (
	"fmt"
	"sync"
)

// Plugin records every store event it is notified about. Register it with
// plugin.Manager; the server bridges the store's StoreHooks to the Manager's
// dispatchers, so events originate deep in the store and arrive here.
type Plugin struct {
	mu      sync.Mutex
	evicts  []string
	expires []string
	tags    []TagInvalidate
}

// TagInvalidate is one recorded tag-invalidation event.
type TagInvalidate struct {
	Tag  string
	Keys []string
}

// New returns an example-hooks plugin ready for Manager.Register.
func New() *Plugin {
	return &Plugin{}
}

// Name implements plugin.Plugin.
func (p *Plugin) Name() string { return "example-hooks" }

// Version implements plugin.Plugin.
func (p *Plugin) Version() string { return "1.0.0" }

// Init implements plugin.Plugin.
func (p *Plugin) Init(config interface{}) error { return nil }

// Close implements plugin.Plugin.
func (p *Plugin) Close() error { return nil }

// OnEvict implements plugin.OnEvictHook.
func (p *Plugin) OnEvict(key string, value interface{}) {
	p.mu.Lock()
	p.evicts = append(p.evicts, key)
	p.mu.Unlock()
}

// OnExpire implements plugin.OnExpireHook.
func (p *Plugin) OnExpire(key string, value interface{}) {
	p.mu.Lock()
	p.expires = append(p.expires, key)
	p.mu.Unlock()
}

// OnTagInvalidate implements plugin.OnTagInvalidateHook.
func (p *Plugin) OnTagInvalidate(tag string, keys []string) {
	p.mu.Lock()
	p.tags = append(p.tags, TagInvalidate{Tag: tag, Keys: append([]string(nil), keys...)})
	p.mu.Unlock()
}

// EvictedKeys returns a copy of the recorded eviction keys.
func (p *Plugin) EvictedKeys() []string {
	p.mu.Lock()
	defer p.mu.Unlock()
	return append([]string(nil), p.evicts...)
}

// ExpiredKeys returns a copy of the recorded expiry keys.
func (p *Plugin) ExpiredKeys() []string {
	p.mu.Lock()
	defer p.mu.Unlock()
	return append([]string(nil), p.expires...)
}

// TagInvalidations returns a copy of the recorded tag invalidations.
func (p *Plugin) TagInvalidations() []TagInvalidate {
	p.mu.Lock()
	defer p.mu.Unlock()
	return append([]TagInvalidate(nil), p.tags...)
}

// Log returns a compact human-readable event log, oldest first.
func (p *Plugin) Log() string {
	p.mu.Lock()
	defer p.mu.Unlock()
	out := ""
	for _, k := range p.evicts {
		out += fmt.Sprintf("evict %s\n", k)
	}
	for _, k := range p.expires {
		out += fmt.Sprintf("expire %s\n", k)
	}
	for _, t := range p.tags {
		out += fmt.Sprintf("tag-invalidate %s %v\n", t.Tag, t.Keys)
	}
	return out
}
