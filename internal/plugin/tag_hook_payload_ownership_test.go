package plugin

import (
	"fmt"
	"testing"
)

type tagPayloadFixtureX struct{ fn func([]string) }

func (*tagPayloadFixtureX) Name() string                              { return "fixture" }
func (*tagPayloadFixtureX) Version() string                           { return "1" }
func (*tagPayloadFixtureX) Init(interface{}) error                    { return nil }
func (*tagPayloadFixtureX) Close() error                              { return nil }
func (h *tagPayloadFixtureX) OnTagInvalidate(_ string, keys []string) { h.fn(keys) }

func TestTagHookPayloadsAreIndependent(t *testing.T) {
	m := NewManager()
	var first, second []string
	register := func(fn func([]string)) {
		if err := m.Register(&tagPayloadFixtureX{fn: fn}); err != nil {
			t.Fatal(err)
		}
	}
	register(func(keys []string) {
		first = keys
		if len(keys) > 0 {
			keys[0] = "edited"
		}
	})
	register(func(keys []string) { second = keys })
	input := []string{"original"}
	m.RunTagInvalidateHooks("tag", input)
	t.Logf("EXPECTED: second=original input=original ACTUAL: second=%s input=%s", second[0], input[0])
	if second[0] != "original" || input[0] != "original" {
		t.Fatal("shared event slice")
	}
	ready, release, done := make(chan struct{}), make(chan struct{}), make(chan struct{})
	go func() { close(ready); <-release; first[0] = "late"; close(done) }()
	<-ready
	close(release)
	<-done
	if second[0] != "original" || input[0] != "original" {
		t.Fatal("late mutation affected another owner")
	}
	m.RunTagInvalidateHooks("tag", nil)
	if first != nil || second != nil {
		t.Fatal("nil event shape changed")
	}
	m.RunTagInvalidateHooks("tag", []string{})
	if first == nil || second == nil || len(first) != 0 || len(second) != 0 {
		t.Fatal("empty event shape changed")
	}
	m.RunTagInvalidateHooks("tag", []string{"repeat"})
	if second[0] != "repeat" {
		t.Fatal("repeated event corrupted")
	}
	fmt.Println("FIX VERIFIED")
}
