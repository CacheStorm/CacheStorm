package examplehooks

import (
	"testing"
)

// The example plugin must satisfy the full hook-consuming contract: the base
// Plugin interface plus the evict, expire, and tag-invalidate hooks — and it
// must record every event it receives, with snapshot accessors safe for
// concurrent use.
func TestPluginRecordsAllHookEvents(t *testing.T) {
	p := New()

	p.OnEvict("k1", "v1")
	p.OnExpire("k2", "v2")
	p.OnTagInvalidate("t1", []string{"k3", "k4"})

	if got := p.EvictedKeys(); len(got) != 1 || got[0] != "k1" {
		t.Fatalf("EvictedKeys() = %v, want [k1]", got)
	}
	if got := p.ExpiredKeys(); len(got) != 1 || got[0] != "k2" {
		t.Fatalf("ExpiredKeys() = %v, want [k2]", got)
	}
	invalidations := p.TagInvalidations()
	if len(invalidations) != 1 || invalidations[0].Tag != "t1" {
		t.Fatalf("TagInvalidations() = %v, want one entry for t1", invalidations)
	}
	if len(invalidations[0].Keys) != 2 || invalidations[0].Keys[0] != "k3" || invalidations[0].Keys[1] != "k4" {
		t.Fatalf("TagInvalidations()[0].Keys = %v, want [k3 k4]", invalidations[0].Keys)
	}
}

// The plugin must satisfy the plugin.Manager registration contract: the base
// Plugin interface plus all three store event hooks.
func TestPluginSatisfiesManagerInterfaces(t *testing.T) {
	p := New()

	var _ interface {
		Name() string
		Version() string
		Init(config interface{}) error
		Close() error
		OnEvict(key string, value interface{})
		OnExpire(key string, value interface{})
		OnTagInvalidate(tag string, keys []string)
	} = p

	if p.Name() != "example-hooks" {
		t.Fatalf("Name() = %q, want example-hooks", p.Name())
	}
}
