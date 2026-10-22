package command

import (
	"testing"
)

// TestAdvanced2RegistryReadersAreSynchronised covers the registry families in
// internal/command/advanced_commands2.go.
//
// These entries carry no per-entry mutex, so a reader that releases the
// registry lock before touching the entry races with the family's writer.
//
// Run with -race this pins the data race. Each family is a subtest so a
// single family can be run in isolation
// (`-run 'TestAdvanced2RegistryReadersAreSynchronised/<family>'`).
func TestAdvanced2RegistryReadersAreSynchronised(t *testing.T) {
	cases := []registryFamilyCase{
		{
			name:  "timer",
			setup: [][]string{{"TIMER.CREATE", "k", "1000"}},
			// TIMER.CREATE is the only mutator of a timer; TIMER.STATUS
			// reads the timer with no lock.
			writer: [][]string{{"TIMER.CREATE", "k", "1000"}},
			reader: [][]string{{"TIMER.STATUS", "{last}"}},
		},
		{
			name:  "record",
			setup: [][]string{{"RECORD.CREATE", "name"}},
			// RECORD.ADD writes the record's fields under the write lock;
			// RECORD.GET reads them with no lock.
			writer: [][]string{{"RECORD.ADD", "{last}", "key", "value"}},
			reader: [][]string{{"RECORD.GET", "{last}"}},
		},
		{
			name:  "entity",
			setup: [][]string{{"ENTITY.CREATE", "k", "type"}},
			// ENTITY.SET writes the entity's attributes under the write
			// lock; ENTITY.GET reads them with no lock.
			writer: [][]string{{"ENTITY.SET", "k", "key", "value"}},
			reader: [][]string{{"ENTITY.GET", "k"}},
		},
		{
			name:  "relation",
			setup: [][]string{{"RELATION.CREATE", "a", "b", "type"}},
			// RELATION.CREATE is the only mutator of a relation;
			// RELATION.GET reads it with no lock.
			writer: [][]string{{"RELATION.CREATE", "a", "b", "type"}},
			reader: [][]string{{"RELATION.GET", "{last}"}},
		},
		{
			name:  "hook",
			setup: [][]string{{"HOOK.CREATE", "name", "trigger", "action"}},
			// HOOK.CREATE is the only mutator of a hook; HOOK.TRIGGER reads
			// it with no lock.
			writer: [][]string{{"HOOK.CREATE", "name", "trigger", "action"}},
			reader: [][]string{{"HOOK.TRIGGER", "{last}"}},
		},
		{
			name:  "index",
			setup: [][]string{{"INDEX.CREATE", "k"}},
			// INDEX.ADD writes the index's entries under the write lock;
			// INDEX.SEARCH reads them with no lock.
			writer: [][]string{{"INDEX.ADD", "k", "key", "id"}},
			reader: [][]string{{"INDEX.SEARCH", "k", "key"}},
		},
		{
			name:  "token",
			setup: [][]string{{"TOKEN.CREATE", "user", "60000"}},
			// TOKEN.REFRESH writes the token's expiry under the write lock;
			// TOKEN.VALIDATE reads it with no lock.
			writer: [][]string{{"TOKEN.REFRESH", "{last}", "60000"}},
			reader: [][]string{{"TOKEN.VALIDATE", "{last}"}},
		},
		{
			name:  "profile",
			setup: [][]string{{"PROFILE.CREATE", "user"}},
			// PROFILE.SET writes the profile's attributes under the write
			// lock; PROFILE.GET reads them with no lock.
			writer: [][]string{{"PROFILE.SET", "{last}", "key", "value"}},
			reader: [][]string{{"PROFILE.GET", "{last}"}},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			runRegistryFamilyRace(t, tc, RegisterAdvancedCommands2, false)
		})
	}
}
