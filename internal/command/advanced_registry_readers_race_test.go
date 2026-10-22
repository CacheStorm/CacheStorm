package command

import (
	"testing"
)

// TestAdvancedRegistryReadersAreSynchronised covers the registry families in
// internal/command/advanced_commands.go.
//
// These handlers are worse than the read-only cases in the other command
// files: DAG.ADDNODE and DAG.ADDEDGE write the DAG's node and edge maps
// with no lock at all, DAG.TOPO iterates the in-degree map with no lock, and
// ACTOR.SEND and SEM.ACQUIRE write the shared mailbox and counter with no
// lock. Concurrent map writes and concurrent map iteration and map write are
// FATAL Go runtime errors.
//
// Run with -race this pins the data race. Each family is a subtest so a
// single family can be run in isolation
// (`-run 'TestAdvancedRegistryReadersAreSynchronised/<family>'`).
func TestAdvancedRegistryReadersAreSynchronised(t *testing.T) {
	cases := []registryFamilyCase{
		{
			name:  "actor",
			setup: [][]string{{"ACTOR.CREATE", "k"}},
			// SEND appends to the actor's mailbox with no lock; LEN and RECV
			// read the same mailbox with no lock.
			writer: [][]string{{"ACTOR.SEND", "k", "msg"}},
			reader: [][]string{{"ACTOR.LEN", "k"}, {"ACTOR.RECV", "k"}},
		},
		{
			name:  "dag",
			setup: [][]string{{"DAG.CREATE", "k"}},
			// ADDNODE writes the node and in-degree maps with no lock, and
			// TOPO iterates the in-degree map with no lock.
			writer: [][]string{{"DAG.ADDNODE", "k", "n"}},
			reader: [][]string{{"DAG.TOPO", "k"}, {"DAG.PARENTS", "k", "n"}, {"DAG.CHILDREN", "k", "n"}},
		},
		{
			name:  "ring",
			setup: [][]string{{"RING.CREATE", "k", "3"}},
			// ADD appends to the ring's node list under the write lock, but
			// GET and NODES read that list with no lock.
			writer: [][]string{{"RING.ADD", "k", "node"}},
			reader: [][]string{{"RING.GET", "k", "key"}, {"RING.NODES", "k"}},
		},
		{
			name:  "sem",
			setup: [][]string{{"SEM.CREATE", "k", "10"}},
			// ACQUIRE writes the semaphore's value and waiter count with no
			// lock, and VALUE reads them with no lock.
			writer: [][]string{{"SEM.ACQUIRE", "k", "1"}},
			reader: [][]string{{"SEM.VALUE", "k"}},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			runRegistryFamilyRace(t, tc, RegisterActorCommands, false)
		})
	}
}
