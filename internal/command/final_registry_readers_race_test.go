package command

import (
	"testing"
)

// finalFamilyCase pairs a family with the command file that registers it.
type finalFamilyCase struct {
	registryFamilyCase
	register func(*Router)
}

// TestFinalRegistryReadersAreSynchronised covers the last registry families
// in internal/command that the earlier sweeps never reached: BUFFERX,
// CONNECTIONX, JOBX, SESSIONX, STREAMX, SECRET, TRIE, ROLLUP, LOCKX, SHUFFLE,
// PARTITIONX and TRANSX.
//
// Run with -race this pins the data race. Each family is a subtest so a
// single family can be run in isolation
// (`-run 'TestFinalRegistryReadersAreSynchronised/<family>'`).
func TestFinalRegistryReadersAreSynchronised(t *testing.T) {
	cases := []finalFamilyCase{
		{
			register: RegisterAdvancedCommands2,
			registryFamilyCase: registryFamilyCase{
				name:   "bufferx",
				setup:  [][]string{{"BUFFERX.CREATE", "k", "10"}},
				writer: [][]string{{"BUFFERX.WRITE", "k", "data"}},
				reader: [][]string{{"BUFFERX.READ", "k"}},
			},
		},
		{
			register: RegisterAdvancedCommands2,
			registryFamilyCase: registryFamilyCase{
				name:   "sessionx",
				setup:  [][]string{{"SESSIONX.CREATE", "user", "60000"}},
				writer: [][]string{{"SESSIONX.SET", "{last}", "key", "value"}},
				reader: [][]string{{"SESSIONX.GET", "{last}"}},
			},
		},
		{
			register: RegisterAdvancedCommands2,
			registryFamilyCase: registryFamilyCase{
				name:   "streamx",
				setup:  [][]string{{"STREAMX.CREATE", "k"}},
				writer: [][]string{{"STREAMX.WRITE", "k", "data"}},
				reader: [][]string{{"STREAMX.READ", "k"}},
			},
		},
		{
			register: RegisterActorCommands,
			registryFamilyCase: registryFamilyCase{
				name:   "secret",
				setup:  [][]string{{"SECRET.SET", "k", "value"}},
				writer: [][]string{{"SECRET.ROTATE", "k", "value2"}},
				reader: [][]string{{"SECRET.GET", "k"}, {"SECRET.VERSION", "k"}},
			},
		},
		{
			register: RegisterActorCommands,
			registryFamilyCase: registryFamilyCase{
				name:   "trie",
				setup:  [][]string{{"TRIE.ADD", "k", "word"}},
				writer: [][]string{{"TRIE.ADD", "k", "word2"}},
				reader: [][]string{{"TRIE.SEARCH", "k", "word"}, {"TRIE.PREFIX", "k", "wo"}, {"TRIE.AUTOCOMPLETE", "k", "wo"}},
			},
		},
		{
			register: RegisterMVCCCommands,
			registryFamilyCase: registryFamilyCase{
				name:   "rollup",
				setup:  [][]string{{"ROLLUP.CREATE", "k", "1000"}},
				writer: [][]string{{"ROLLUP.ADD", "k", "100", "1"}},
				reader: [][]string{{"ROLLUP.GET", "k", "100"}},
			},
		},
		{
			register: RegisterResilienceCommands,
			registryFamilyCase: registryFamilyCase{
				name:   "lockx",
				setup:  [][]string{{"LOCKX.ACQUIRE", "k", "holder", "60000"}},
				writer: [][]string{{"LOCKX.EXTEND", "k", "holder", "60000"}},
				reader: [][]string{{"LOCKX.STATUS", "k"}},
			},
		},
		{
			register: RegisterResilienceCommands,
			registryFamilyCase: registryFamilyCase{
				name:   "shuffle",
				setup:  [][]string{{"SHUFFLE.CREATE", "k"}},
				writer: [][]string{{"SHUFFLE.ADD", "k", "value"}},
				reader: [][]string{{"SHUFFLE.GET", "k"}},
			},
		},
		{
			register: RegisterResilienceCommands,
			registryFamilyCase: registryFamilyCase{
				name:   "partitionx",
				setup:  [][]string{{"PARTITIONX.CREATE", "k", "4"}},
				writer: [][]string{{"PARTITIONX.ADD", "k", "key", "value"}},
				reader: [][]string{{"PARTITIONX.GET", "k", "0"}},
			},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			runRegistryFamilyRace(t, tc.registryFamilyCase, tc.register, false)
		})
	}
}
