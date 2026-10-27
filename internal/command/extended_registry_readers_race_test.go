package command

import (
	"testing"
)

// TestExtendedRegistryReadersAreSynchronised covers the registry families in
// internal/command/extended_commands.go that keep a shared entry behind a
// package-level map.
//
// Run with -race this pins the data race. Each family is a subtest so a
// single family can be run in isolation
// (`-run 'TestExtendedRegistryReadersAreSynchronised/<family>'`).
func TestExtendedRegistryReadersAreSynchronised(t *testing.T) {
	cases := []registryFamilyCase{
		{
			name:  "msgqueue",
			setup: [][]string{{"MSGQUEUE.CREATE", "k", "1"}},
			// PUBLISH appends to the queue's message list, CONSUME takes the
			// message into the ack-wait map and NACK moves it into the
			// dead-letter list (maxRetries is 1), so both fields the readers
			// read keep being written.
			writer: [][]string{{"MSGQUEUE.PUBLISH", "k", "body"}, {"MSGQUEUE.CONSUME", "k"}, {"MSGQUEUE.NACK", "k", "{id}", "reason"}},
			reader: [][]string{{"MSGQUEUE.STATS", "k"}, {"MSGQUEUE.DEADLETTER", "k"}},
		},
		{
			name:  "leader",
			setup: [][]string{{"LEADER.ELECT", "k", "candidate", "0"}},
			// ELECT records the election in the history and updates the
			// leader, the expiry and the term.
			writer: [][]string{{"LEADER.ELECT", "k", "candidate", "0"}},
			reader: [][]string{{"LEADER.HISTORY", "k"}},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			runRegistryFamilyRace(t, tc, RegisterExtendedCommands, false)
		})
	}
}
