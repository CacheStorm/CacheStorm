package module

import (
	"errors"
	"reflect"
	"testing"
)

type commandOwnershipObservationX struct {
	Label            string
	Expected, Actual interface{}
}

func mutateCommandSnapshotX(commands []CommandDef, edit func([]CommandDef)) {
	ready, release, done := make(chan struct{}), make(chan struct{}), make(chan struct{})
	go func() { close(ready); <-release; edit(commands); close(done) }()
	<-ready
	close(release)
	<-done
}

func commandOwnershipObservationsX(edges bool) []commandOwnershipObservationX {
	original := errors.New("original handler")
	replacement := errors.New("replacement handler")
	m := NewBaseModule("owner", "1")
	m.AddCommand("ORIGINAL", func(*CommandContext) error { return original }, CommandFlags{ReadOnly: true})
	r := NewRegistry()
	if err := r.Register(m); err != nil {
		panic(err)
	}
	if err := r.Load("owner", NewContext(nil)); err != nil {
		panic(err)
	}
	mutateCommandSnapshotX(r.GetCommands(), func(cmds []CommandDef) { cmds[0].Name = "CONTROL EDIT" })
	obs := []commandOwnershipObservationX{{"control: registry result is isolated", "ORIGINAL", r.GetCommands()[0].Name}}
	first := m.Commands()
	mutateCommandSnapshotX(first, func(cmds []CommandDef) {
		cmds[0].Name = "CALLER EDIT"
		cmds[0].Flags = CommandFlags{Write: true, Admin: true}
		cmds[0].Handler = func(*CommandContext) error { return replacement }
	})
	current := r.GetCommands()[0]
	obs = append(obs, commandOwnershipObservationX{"module name preserved", "ORIGINAL", current.Name}, commandOwnershipObservationX{"module read-only flag preserved", true, current.Flags.ReadOnly}, commandOwnershipObservationX{"module admin flag preserved", false, current.Flags.Admin}, commandOwnershipObservationX{"module handler preserved", true, errors.Is(current.Handler(nil), original)})
	if edges {
		empty := NewBaseModule("empty", "1").Commands()
		obs = append(obs, commandOwnershipObservationX{"empty result remains nonnil", true, empty != nil}, commandOwnershipObservationX{"empty result count", 0, len(empty)})
		n := NewBaseModule("multi", "1")
		n.AddCommand("FIRST", nil, CommandFlags{})
		n.AddCommand("LAST", nil, CommandFlags{Admin: true})
		left, right := n.Commands(), n.Commands()
		mutateCommandSnapshotX(left, func(cmds []CommandDef) { cmds[0].Name = "EDIT"; cmds[1].Flags.Admin = false })
		obs = append(obs, commandOwnershipObservationX{"independent result name", "FIRST", right[0].Name}, commandOwnershipObservationX{"independent result flags", true, right[1].Flags.Admin}, commandOwnershipObservationX{"nil handler retained", true, n.Commands()[0].Handler == nil})
		n.AddCommand("ADDED", nil, CommandFlags{})
		obs = append(obs, commandOwnershipObservationX{"module append is visible to new reads", 3, len(n.Commands())}, commandOwnershipObservationX{"old snapshot length preserved", 2, len(right)})
		right = append(right, CommandDef{Name: "CALLER ONLY"})
		obs = append(obs, commandOwnershipObservationX{"caller append result", "CALLER ONLY", right[len(right)-1].Name})
		obs = append(obs, commandOwnershipObservationX{"caller append does not change registration", "ADDED", n.Commands()[2].Name})
	}
	return obs
}

func TestBaseModuleCommandSnapshotOwnershipX(t *testing.T) {
	for _, o := range commandOwnershipObservationsX(true) {
		if !reflect.DeepEqual(o.Expected, o.Actual) {
			t.Errorf("%s: expected %v, got %v", o.Label, o.Expected, o.Actual)
		}
	}
}
