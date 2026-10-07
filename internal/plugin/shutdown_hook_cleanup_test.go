package plugin

import (
	"errors"
	"fmt"
	"reflect"
	"testing"
)

type shutdownFixtureX struct{ fn func() error }

func (*shutdownFixtureX) Name() string           { return "fixture" }
func (*shutdownFixtureX) Version() string        { return "1" }
func (*shutdownFixtureX) Init(interface{}) error { return nil }
func (*shutdownFixtureX) Close() error           { return nil }
func (h *shutdownFixtureX) OnShutdown() error    { return h.fn() }

func TestShutdownHooksContinueAfterError(t *testing.T) {
	first, second := errors.New("first"), errors.New("second")
	for _, failures := range [][]error{{nil, nil, nil}, {first, nil, second}, {nil, second, nil}, {first, second, nil}} {
		m := NewManager()
		var calls []int
		var expected error
		for i, failure := range failures {
			idx, err := i, failure
			if expected == nil {
				expected = err
			}
			if err := m.Register(&shutdownFixtureX{fn: func() error { calls = append(calls, idx); return err }}); err != nil {
				t.Fatal(err)
			}
		}
		got := m.RunShutdownHooks()
		t.Logf("EXPECTED: hooks=[0 1 2], error=%v ACTUAL: hooks=%v error=%v", expected, calls, got)
		if !reflect.DeepEqual(calls, []int{0, 1, 2}) || !errors.Is(got, expected) {
			t.Fatal("cleanup aborted or first error changed")
		}
	}
	if err := NewManager().RunShutdownHooks(); err != nil {
		t.Fatal(err)
	}
	m := NewManager()
	late := 0
	if err := m.Register(&shutdownFixtureX{fn: func() error { return m.Register(&shutdownFixtureX{fn: func() error { late++; return nil }}) }}); err != nil {
		t.Fatal(err)
	}
	if err := m.RunShutdownHooks(); err != nil || late != 0 {
		t.Fatal("snapshot changed mid-dispatch")
	}
	if err := m.RunShutdownHooks(); err != nil || late != 1 {
		t.Fatal("registered hook missing from next snapshot")
	}
	fmt.Println("FIX VERIFIED")
}
