package cluster_test

import (
	"reflect"
	"testing"

	"github.com/cachestorm/cachestorm/internal/cluster"
)

type migrationObservationX struct {
	Name      string
	Want, Got any
}

func migrationMustX(err error) {
	if err != nil {
		panic(err)
	}
}
func migrationFixtureX() *cluster.Cluster {
	c := cluster.New("source", "127.0.0.1", 0, 0, nil)
	c.AssignSlots([]cluster.SlotRange{{Start: 0, End: 3}})
	for _, id := range []string{"target", "replacement"} {
		c.AddNode(&cluster.Node{ID: id, Role: cluster.RolePrimary})
	}
	return c
}
func migrationOwnerX(c *cluster.Cluster, slot uint16) string {
	n := c.GetSlotOwner(slot)
	if n == nil {
		return ""
	}
	return n.ID
}
func migrationControlX() []migrationObservationX {
	c := migrationFixtureX()
	m := cluster.NewSlotMigrator(c)
	migrationMustX(m.StartMigration("source", "target", []uint16{0}))
	migrationMustX(m.Complete())
	return []migrationObservationX{{"control: ordinary migration", "target", migrationOwnerX(c, 0)}, {"control: untouched neighbor", "source", migrationOwnerX(c, 1)}}
}
func migrationInputOwnershipX(verify bool) []migrationObservationX {
	results := migrationControlX()
	c := migrationFixtureX()
	m := cluster.NewSlotMigrator(c)
	slots := []uint16{0}
	migrationMustX(m.StartMigration("source", "target", slots))
	release, done := make(chan struct{}), make(chan struct{})
	go func() { <-release; slots[0] = 1; close(done) }()
	close(release)
	<-done
	migrationMustX(m.Complete())
	results = append(results, migrationObservationX{"original slot remains selected", "target", migrationOwnerX(c, 0)}, migrationObservationX{"caller mutation does not add slot", "source", migrationOwnerX(c, 1)})
	if verify {
		c = migrationFixtureX()
		m = cluster.NewSlotMigrator(c)
		slots = []uint16{0, 2}
		migrationMustX(m.StartMigration("source", "target", slots))
		slots = append(slots, 3)
		slots[1] = 1
		migrationMustX(m.Complete())
		results = append(results, migrationObservationX{"multi-slot snapshot", "target", migrationOwnerX(c, 2)}, migrationObservationX{"appended slot excluded", "source", migrationOwnerX(c, 3)})
		results = append(results, migrationObservationX{"repeated completion rejected", true, m.Complete() != nil})
	}
	return results
}
func migrationStaleCompletionX(verify bool) []migrationObservationX {
	results := migrationControlX()
	c := migrationFixtureX()
	stale := cluster.NewSlotMigrator(c)
	migrationMustX(stale.StartMigration("source", "target", []uint16{0, 1}))
	started, release, done := make(chan struct{}), make(chan struct{}), make(chan error, 1)
	go func() { close(started); <-release; done <- stale.Complete() }()
	<-started
	winner := cluster.NewSlotMigrator(c)
	migrationMustX(winner.StartMigration("source", "replacement", []uint16{1}))
	migrationMustX(winner.Complete())
	close(release)
	err := <-done
	results = append(results, migrationObservationX{"stale completion rejected", true, err != nil}, migrationObservationX{"winner retains slot", "replacement", migrationOwnerX(c, 1)}, migrationObservationX{"rejection leaves whole transaction untouched", "source", migrationOwnerX(c, 0)})
	if verify {
		results = append(results, migrationObservationX{"target ranges untouched", 0, len(c.GetNode("target").Slots)}, migrationObservationX{"winner ranges remain", []cluster.SlotRange{{Start: 1, End: 1}}, c.GetNode("replacement").Slots})
		c = migrationFixtureX()
		m := cluster.NewSlotMigrator(c)
		migrationMustX(m.StartMigration("source", "target", []uint16{0}))
		c.RemoveNode("source")
		results = append(results, migrationObservationX{"removed source rejected", true, m.Complete() != nil}, migrationObservationX{"removed source slots remain unassigned", "", migrationOwnerX(c, 0)})
		c = migrationFixtureX()
		m = cluster.NewSlotMigrator(c)
		migrationMustX(m.StartMigration("source", "target", []uint16{0}))
		c.RemoveNode("target")
		results = append(results, migrationObservationX{"removed target rejected", true, m.Complete() != nil}, migrationObservationX{"source retains slot after target removal", "source", migrationOwnerX(c, 0)})
	}
	return results
}

func migrationCheckX(t *testing.T, rows []migrationObservationX) {
	t.Helper()
	for _, row := range rows {
		if !reflect.DeepEqual(row.Want, row.Got) {
			t.Errorf("%s: got %#v, want %#v", row.Name, row.Got, row.Want)
		}
	}
}
func TestMigrationCopiesPreparedSlotsX(t *testing.T) {
	migrationCheckX(t, migrationInputOwnershipX(true))
}
func TestMigrationRejectsStaleCompletionX(t *testing.T) {
	migrationCheckX(t, migrationStaleCompletionX(true))
}
