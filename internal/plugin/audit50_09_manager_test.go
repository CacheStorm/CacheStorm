package plugin

import (
	"errors"
	"reflect"
	"testing"

	"github.com/cachestorm/cachestorm/internal/command"
)

type fakeAudit50R09X struct{ callback func() error }

func (*fakeAudit50R09X) Name() string    { return "fixture" }
func (*fakeAudit50R09X) Version() string { return "1" }
func (p *fakeAudit50R09X) invoke() error {
	if p.callback != nil {
		return p.callback()
	}
	return nil
}
func (p *fakeAudit50R09X) Init(interface{}) error               { return p.invoke() }
func (p *fakeAudit50R09X) Close() error                         { return p.invoke() }
func (p *fakeAudit50R09X) BeforeCommand(*command.Context) error { return p.invoke() }
func (p *fakeAudit50R09X) AfterCommand(*command.Context)        { p.invoke() }
func (p *fakeAudit50R09X) OnEvict(string, interface{})          { p.invoke() }
func (p *fakeAudit50R09X) OnExpire(string, interface{})         { p.invoke() }
func (p *fakeAudit50R09X) OnTagInvalidate(string, []string)     { p.invoke() }
func (p *fakeAudit50R09X) OnStartup() error                     { return p.invoke() }
func (p *fakeAudit50R09X) OnShutdown() error                    { return p.invoke() }

func checkAudit50R09X(t *testing.T, label string, want, got interface{}) {
	t.Helper()
	t.Logf("%s EXPECTED: %v | ACTUAL: %v", label, want, got)
	if !reflect.DeepEqual(want, got) {
		t.Errorf("%s mismatch", label)
	}
}

func TestAudit50R09X(t *testing.T) {
	defer func() {
		if t.Failed() {
			t.Log("PROBLEM CONFIRMED")
		} else {
			t.Log("FIX VERIFIED")
		}
	}()
	invoke := []struct {
		name string
		run  func(*Manager) error
	}{
		{"init", func(m *Manager) error { return m.InitAll(nil) }},
		{"close", func(m *Manager) error { return m.CloseAll() }},
		{"before", func(m *Manager) error { return m.RunBeforeHooks(nil) }},
		{"after", func(m *Manager) error { m.RunAfterHooks(nil); return nil }},
		{"evict", func(m *Manager) error { m.RunEvictHooks("k", nil); return nil }},
		{"expire", func(m *Manager) error { m.RunExpireHooks("k", nil); return nil }},
		{"tag", func(m *Manager) error { m.RunTagInvalidateHooks("tag", nil); return nil }},
		{"startup", func(m *Manager) error { return m.RunStartupHooks() }},
		{"shutdown", func(m *Manager) error { return m.RunShutdownHooks() }},
	}
	control := NewManager()
	if err := control.Register(&fakeAudit50R09X{}); err != nil {
		t.Fatal(err)
	}
	checkAudit50R09X(t, "control ordinary init", true, control.InitAll(nil) == nil)
	for _, c := range invoke {
		m := NewManager()
		available := false
		p := &fakeAudit50R09X{callback: func() error {
			available = m.mu.TryLock()
			if !available {
				return errors.New("callback cannot register: manager lock held")
			}
			m.mu.Unlock()
			return m.Register(&fakeAudit50R09X{})
		}}
		if err := m.Register(p); err != nil {
			t.Fatal(err)
		}
		c.run(m)
		checkAudit50R09X(t, c.name+" callback permits registry writer", true, available)
		checkAudit50R09X(t, c.name+" callback registration completes", 2, len(m.Plugins()))
	}
	empty := NewManager()
	checkAudit50R09X(t, "empty init", true, empty.InitAll(nil) == nil)
	checkAudit50R09X(t, "empty close", true, empty.CloseAll() == nil)
	sentinel := errors.New("hook failure")
	fail := NewManager()
	if err := fail.Register(&fakeAudit50R09X{callback: func() error { return sentinel }}); err != nil {
		t.Fatal(err)
	}
	checkAudit50R09X(t, "before error propagated", true, errors.Is(fail.RunBeforeHooks(nil), sentinel))
	checkAudit50R09X(t, "close error propagated", true, errors.Is(fail.CloseAll(), sentinel))
	gated := NewManager()
	entered, release, done := make(chan struct{}), make(chan struct{}), make(chan error, 1)
	if err := gated.Register(&fakeAudit50R09X{callback: func() error { close(entered); <-release; return nil }}); err != nil {
		t.Fatal(err)
	}
	go func() { done <- gated.InitAll(nil) }()
	<-entered
	available := gated.mu.TryLock()
	if available {
		gated.mu.Unlock()
		if err := gated.Register(&fakeAudit50R09X{}); err != nil {
			t.Fatal(err)
		}
	}
	close(release)
	checkAudit50R09X(t, "gated concurrent writer allowed", true, available)
	checkAudit50R09X(t, "gated init completion", true, (<-done) == nil)

}
