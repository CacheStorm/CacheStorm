package command

import (
	"testing"
)

// TestAuditFixedFamiliesAreSynchronised drives each family whose handlers
// used to touch shared state outside its lock (a plain map write with no lock
// is a fatal, unrecoverable throw). Only meaningful under -race.
func TestAuditFixedFamiliesAreSynchronised(t *testing.T) {
	cases := []remainingFamilyCase{
		{
			register: RegisterMVCCCommands,
			registryFamilyCase: registryFamilyCase{
				name:   "spatial",
				setup:  [][]string{{"SPATIAL.CREATE", "k"}, {"SPATIAL.ADD", "k", "seed", "1", "1"}},
				writer: [][]string{{"SPATIAL.ADD", "k", "p{i}", "1", "2"}, {"SPATIAL.DELETE", "k", "p{i}"}},
				reader: [][]string{{"SPATIAL.NEARBY", "k", "1", "1", "1000"}, {"SPATIAL.WITHIN", "k", "0", "0", "3", "3"}},
			},
		},
		{
			register: RegisterMVCCCommands,
			registryFamilyCase: registryFamilyCase{
				name:   "cooldown",
				setup:  [][]string{{"COOLDOWN.SET", "k", "0"}},
				writer: [][]string{{"COOLDOWN.CHECK", "k"}, {"COOLDOWN.RESET", "k"}},
				reader: [][]string{{"COOLDOWN.CHECK", "k"}, {"COOLDOWN.LIST"}},
			},
		},
		{
			register: RegisterEventCommands,
			registryFamilyCase: registryFamilyCase{
				name:   "event",
				setup:  [][]string{{"EVENT.EMIT", "e"}},
				writer: [][]string{{"EVENT.EMIT", "e", "n", "{i}"}, {"EVENT.CLEAR"}},
				reader: [][]string{{"EVENT.LIST"}, {"EVENT.GET", "e"}},
			},
		},
		{
			register: RegisterMLCommands,
			registryFamilyCase: registryFamilyCase{
				name:   "feature",
				setup:  [][]string{{"FEATURE.SET", "k", "f", "1"}},
				writer: [][]string{{"FEATURE.SET", "k", "f{i}", "1"}, {"FEATURE.DEL", "k", "f{i}"}},
				reader: [][]string{{"FEATURE.VECTOR", "k"}},
			},
		},
		{
			register: RegisterExtendedCommands,
			registryFamilyCase: registryFamilyCase{
				name:   "vector",
				setup:  [][]string{{"VECTOR.CREATE", "k", "2"}, {"VECTOR.ADD", "k", "a", "2", "1", "0"}},
				writer: [][]string{{"VECTOR.ADD", "k", "v{i}", "2", "0", "1"}, {"VECTOR.DELETE", "k", "v{i}"}, {"VECTOR.ADD", "k", "a", "2", "1", "0"}},
				reader: [][]string{{"VECTOR.GET", "k", "a"}, {"VECTOR.SEARCH", "k", "1", "2", "1", "0"}},
			},
		},
		{
			register: RegisterExtendedCommands,
			registryFamilyCase: registryFamilyCase{
				name:   "leader",
				setup:  [][]string{{"LEADER.ELECT", "k", "c"}},
				writer: [][]string{{"LEADER.RESIGN", "k", "c"}, {"LEADER.ELECT", "k", "c"}},
				reader: [][]string{{"LEADER.CURRENT", "k"}},
			},
		},
		{
			register: RegisterResilienceCommands,
			registryFamilyCase: registryFamilyCase{
				name:   "windowx",
				setup:  [][]string{{"WINDOWX.CREATE", "k", "10"}, {"WINDOWX.ADD", "k", "1"}},
				writer: [][]string{{"WINDOWX.ADD", "k", "2"}},
				reader: [][]string{{"WINDOWX.AGGREGATE", "k", "sum"}, {"WINDOWX.AGGREGATE", "k", "avg"}},
			},
		},
		{
			register: RegisterExtraCommands,
			registryFamilyCase: registryFamilyCase{
				name:   "vectorclock",
				setup:  [][]string{{"VECTOR_CLOCK.CREATE", "a", "n"}, {"VECTOR_CLOCK.CREATE", "b", "n"}},
				writer: [][]string{{"VECTOR_CLOCK.INCREMENT", "a", "n{i}"}, {"VECTOR_CLOCK.INCREMENT", "b", "n"}},
				reader: [][]string{{"VECTOR_CLOCK.COMPARE", "a", "b"}},
			},
		},
		{
			register: RegisterWorkflowCommands,
			registryFamilyCase: registryFamilyCase{
				name:   "chained",
				setup:  [][]string{{"CHAINED.SET", "k", "p", "v"}},
				writer: [][]string{{"CHAINED.SET", "k", "p{i}", "v"}, {"CHAINED.DEL", "k", "p{i}"}, {"CHAINED.SET", "r{i}", "p", "v"}},
				reader: [][]string{{"CHAINED.GET", "k", "p"}, {"CHAINED.GET", "k", "missing"}},
			},
		},
		{
			register: RegisterDataStructuresCommands,
			registryFamilyCase: registryFamilyCase{
				name:   "slidingwindow_per_key",
				setup:  [][]string{{"SLIDINGWINDOW.CREATE", "k", "1000", "1000000"}},
				writer: [][]string{{"SLIDINGWINDOW.INCR", "k", "key{i}"}, {"SLIDINGWINDOW.RESET", "k"}},
				reader: [][]string{{"SLIDINGWINDOW.COUNT", "k"}, {"SLIDINGWINDOW.COUNT", "k", "key1"}},
			},
		},
		{
			register: RegisterIntegrationCommands,
			registryFamilyCase: registryFamilyCase{
				name:   "ratelimit_per_id",
				setup:  [][]string{{"RATELIMIT.CREATE", "k", "1000000", "60000"}},
				writer: [][]string{{"RATELIMIT.CHECK", "k", "id{i}"}},
				reader: [][]string{{"RATELIMIT.CHECK", "k", "reader"}},
			},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			runRegistryFamilyRace(t, tc.registryFamilyCase, tc.register, false)
		})
	}
}
