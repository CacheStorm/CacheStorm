package command

import (
	"testing"
)

// remainingFamilyCase pairs a family with the command file that registers it.
type remainingFamilyCase struct {
	registryFamilyCase
	register func(*Router)
}

// TestRemainingRegistryReadersAreSynchronised covers the last registry
// families in internal/command that keep a shared entry behind a
// package-level map: stats_commands.go, encoding_commands.go, ds_commands.go
// and probabilistic_commands.go.
//
// Run with -race this pins the data race. Each family is a subtest so a
// single family can be run in isolation
// (`-run 'TestRemainingRegistryReadersAreSynchronised/<family>'`).
func TestRemainingRegistryReadersAreSynchronised(t *testing.T) {
	cases := []remainingFamilyCase{
		{
			register: RegisterStatsCommands,
			registryFamilyCase: registryFamilyCase{
				name:  "tdigest",
				setup: [][]string{{"TDIGEST.CREATE", "k"}},
				// ADD feeds the digest; the readers query it with no lock.
				writer: [][]string{{"TDIGEST.ADD", "k", "1"}},
				reader: [][]string{{"TDIGEST.QUANTILE", "k", "0.5"}, {"TDIGEST.CDF", "k", "1"}, {"TDIGEST.MEAN", "k"}, {"TDIGEST.MIN", "k"}, {"TDIGEST.MAX", "k"}, {"TDIGEST.INFO", "k"}},
			},
		},
		{
			register: RegisterStatsCommands,
			registryFamilyCase: registryFamilyCase{
				name:   "sample",
				setup:  [][]string{{"SAMPLE.CREATE", "k", "100"}},
				writer: [][]string{{"SAMPLE.ADD", "k", "1"}},
				reader: [][]string{{"SAMPLE.GET", "k"}, {"SAMPLE.INFO", "k"}},
			},
		},
		{
			register: RegisterStatsCommands,
			registryFamilyCase: registryFamilyCase{
				name:   "histogram",
				setup:  [][]string{{"HISTOGRAM.CREATE", "k", "10", "10"}},
				writer: [][]string{{"HISTOGRAM.ADD", "k", "5"}, {"HISTOGRAM.RESET", "k"}},
				reader: [][]string{{"HISTOGRAM.GET", "k"}, {"HISTOGRAM.MEAN", "k"}, {"HISTOGRAM.INFO", "k"}},
			},
		},
		{
			register: RegisterEncodingCommands,
			registryFamilyCase: registryFamilyCase{
				name:  "pool",
				setup: [][]string{{"POOL.CREATE", "k", "10"}, {"POOL.PUT", "k", "seed"}},
				// PUT adds an item and GET takes one out; STATS reads the
				// pool's item list with no lock.
				writer: [][]string{{"POOL.PUT", "k", "item"}, {"POOL.GET", "k"}},
				reader: [][]string{{"POOL.STATS", "k"}},
			},
		},
		{
			register: RegisterDataStructuresCommands,
			registryFamilyCase: registryFamilyCase{
				name:  "pq",
				setup: [][]string{{"PQ.CREATE", "k"}},
				// PUSH adds an item and POP takes one out; the readers look
				// at the queue with no lock.
				writer: [][]string{{"PQ.PUSH", "k", "value", "1"}, {"PQ.POP", "k"}},
				reader: [][]string{{"PQ.PEEK", "k"}, {"PQ.LEN", "k"}, {"PQ.GETALL", "k"}},
			},
		},
		{
			register: RegisterDataStructuresCommands,
			registryFamilyCase: registryFamilyCase{
				name:  "lru",
				setup: [][]string{{"LRU.CREATE", "k", "10"}},
				// SET writes an entry and GET refreshes its recency; the
				// readers look at the cache with no lock.
				writer: [][]string{{"LRU.SET", "k", "key", "value"}, {"LRU.GET", "k", "key"}},
				reader: [][]string{{"LRU.KEYS", "k"}, {"LRU.STATS", "k"}},
			},
		},
		{
			register: RegisterDataStructuresCommands,
			registryFamilyCase: registryFamilyCase{
				name:  "tokenbucket",
				setup: [][]string{{"TOKENBUCKET.CREATE", "k", "100", "100"}},
				// CONSUME spends tokens; AVAILABLE reads the count with no
				// lock.
				writer: [][]string{{"TOKENBUCKET.CONSUME", "k", "1"}},
				reader: [][]string{{"TOKENBUCKET.AVAILABLE", "k"}},
			},
		},
		{
			register: RegisterDataStructuresCommands,
			registryFamilyCase: registryFamilyCase{
				name:  "leakybucket",
				setup: [][]string{{"LEAKYBUCKET.CREATE", "k", "100", "10"}},
				// ADD pours into the bucket; AVAILABLE reads it with no lock.
				writer: [][]string{{"LEAKYBUCKET.ADD", "k", "1"}},
				reader: [][]string{{"LEAKYBUCKET.AVAILABLE", "k"}},
			},
		},
		{
			register: RegisterDataStructuresCommands,
			registryFamilyCase: registryFamilyCase{
				name:  "slidingwindowcounter",
				setup: [][]string{{"SLIDINGWINDOW.CREATE", "k", "1000", "10000"}},
				// INCR bumps the counter; COUNT reads it with no lock.
				writer: [][]string{{"SLIDINGWINDOW.INCR", "k", "key"}},
				reader: [][]string{{"SLIDINGWINDOW.COUNT", "k"}},
			},
		},
		{
			register: RegisterProbabilisticCommands,
			registryFamilyCase: registryFamilyCase{
				name:   "bloomfilter",
				setup:  [][]string{{"BF.ADD", "k", "item"}},
				writer: [][]string{{"BF.ADD", "k", "item"}},
				reader: [][]string{{"BF.EXISTS", "k", "item"}, {"BF.INFO", "k"}},
			},
		},
		{
			register: RegisterProbabilisticCommands,
			registryFamilyCase: registryFamilyCase{
				name:   "cuckoofilter",
				setup:  [][]string{{"CF.ADD", "k", "item"}},
				writer: [][]string{{"CF.ADD", "k", "item"}},
				reader: [][]string{{"CF.EXISTS", "k", "item"}, {"CF.INFO", "k"}},
			},
		},
		{
			register: RegisterProbabilisticCommands,
			registryFamilyCase: registryFamilyCase{
				name:  "countminsketch",
				setup: [][]string{{"CMS.INITBYDIM", "k", "100", "4"}},
				// INCRBY bumps the sketch's counters; QUERY reads them with
				// no lock.
				writer: [][]string{{"CMS.INCRBY", "k", "item", "1"}},
				reader: [][]string{{"CMS.QUERY", "k", "item"}, {"CMS.INFO", "k"}},
			},
		},
		{
			register: RegisterProbabilisticCommands,
			registryFamilyCase: registryFamilyCase{
				name:  "topk",
				setup: [][]string{{"TOPK.RESERVE", "k", "10"}},
				// ADD feeds the sketch; the readers query it with no lock.
				writer: [][]string{{"TOPK.ADD", "k", "item"}},
				reader: [][]string{{"TOPK.QUERY", "k", "5"}, {"TOPK.LIST", "k"}, {"TOPK.INFO", "k"}},
			},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			runRegistryFamilyRace(t, tc.registryFamilyCase, tc.register, false)
		})
	}
}
