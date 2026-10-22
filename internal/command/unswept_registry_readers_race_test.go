package command

import (
	"testing"
)

// unsweptFamilyCase pairs a family with the command file that registers it.
type unsweptFamilyCase struct {
	registryFamilyCase
	register func(*Router)
}

// TestUnsweptRegistryReadersAreSynchronised covers the registry families in
// internal/command that the earlier sweeps never reached: the ML,
// RATELIMITER, SEMAPHOREX, THROTTLEX, DEBOUNCEX, WINDOWX, JOINX, BUCKETX,
// CHAIN, ANALYTICS, COOLDOWN, HEALTHX, SENTINELX and POOLX families.
//
// Most of these handlers took no lock at all: the writer mutated the shared
// entry with no registry lock and the reader read its fields with none
// either, so the two raced on the same memory.
//
// Run with -race this pins the data race. Each family is a subtest so a
// single family can be run in isolation
// (`-run 'TestUnsweptRegistryReadersAreSynchronised/<family>'`).
func TestUnsweptRegistryReadersAreSynchronised(t *testing.T) {
	cases := []unsweptFamilyCase{
		{
			register: RegisterMLCommands,
			registryFamilyCase: registryFamilyCase{
				name:   "model",
				setup:  [][]string{{"MODEL.CREATE", "k", "regression"}},
				writer: [][]string{{"MODEL.TRAIN", "k", "data"}},
				reader: [][]string{{"MODEL.STATUS", "k"}},
			},
		},
		{
			register: RegisterMLCommands,
			registryFamilyCase: registryFamilyCase{
				name:   "tensor",
				setup:  [][]string{{"TENSOR.CREATE", "k", "2", "3", "|", "f32"}, {"TENSOR.CREATE", "other", "2", "3", "|", "f32"}},
				writer: [][]string{{"TENSOR.ADD", "k", "other"}},
				reader: [][]string{{"TENSOR.GET", "k"}},
			},
		},
		{
			register: RegisterMLCommands,
			registryFamilyCase: registryFamilyCase{
				name:   "classifier",
				setup:  [][]string{{"CLASSIFIER.CREATE", "k", "a", "b"}},
				writer: [][]string{{"CLASSIFIER.TRAIN", "k"}},
				reader: [][]string{{"CLASSIFIER.TRAIN", "k"}},
			},
		},
		{
			register: RegisterMLCommands,
			registryFamilyCase: registryFamilyCase{
				name:   "regressor",
				setup:  [][]string{{"REGRESSOR.CREATE", "k"}},
				writer: [][]string{{"REGRESSOR.TRAIN", "k"}},
				reader: [][]string{{"REGRESSOR.TRAIN", "k"}},
			},
		},
		{
			register: RegisterMLCommands,
			registryFamilyCase: registryFamilyCase{
				name:   "cluster",
				setup:  [][]string{{"CLUSTER.CREATE", "k", "3"}},
				writer: [][]string{{"CLUSTER.FIT", "k"}},
				reader: [][]string{{"CLUSTER.CENTROIDS", "k"}},
			},
		},
		{
			register: RegisterMLCommands,
			registryFamilyCase: registryFamilyCase{
				name:   "anomaly",
				setup:  [][]string{{"ANOMALY.CREATE", "k"}},
				writer: [][]string{{"ANOMALY.LEARN", "k"}},
				reader: [][]string{{"ANOMALY.LEARN", "k"}},
			},
		},
		{
			register: RegisterMLCommands,
			registryFamilyCase: registryFamilyCase{
				name:   "dataset",
				setup:  [][]string{{"DATASET.CREATE", "k"}},
				writer: [][]string{{"DATASET.ADD", "k", "item"}},
				reader: [][]string{{"DATASET.GET", "k"}},
			},
		},
		{
			register: RegisterMLCommands,
			registryFamilyCase: registryFamilyCase{
				name:   "pipelineml",
				setup:  [][]string{{"PIPELINEML.CREATE", "k"}},
				writer: [][]string{{"PIPELINEML.ADD", "k", "step"}},
				reader: [][]string{{"PIPELINEML.ADD", "k", "step"}},
			},
		},
		{
			register: RegisterResilienceCommands,
			registryFamilyCase: registryFamilyCase{
				name:   "ratelimiter",
				setup:  [][]string{{"RATELIMITER.CREATE", "k", "1000000", "1000"}},
				writer: [][]string{{"RATELIMITER.TRY", "k"}},
				reader: [][]string{{"RATELIMITER.STATUS", "k"}},
			},
		},
		{
			register: RegisterResilienceCommands,
			registryFamilyCase: registryFamilyCase{
				name:   "semaphorex",
				setup:  [][]string{{"SEMAPHOREX.CREATE", "k", "1000000"}},
				writer: [][]string{{"SEMAPHOREX.ACQUIRE", "k"}},
				reader: [][]string{{"SEMAPHOREX.STATUS", "k"}},
			},
		},
		{
			register: RegisterResilienceCommands,
			registryFamilyCase: registryFamilyCase{
				name:   "throttlex",
				setup:  [][]string{{"THROTTLEX.CREATE", "k", "1000000"}},
				writer: [][]string{{"THROTTLEX.CHECK", "k"}},
				reader: [][]string{{"THROTTLEX.STATUS", "k"}},
			},
		},
		{
			register: RegisterResilienceCommands,
			registryFamilyCase: registryFamilyCase{
				name:   "debouncex",
				setup:  [][]string{{"DEBOUNCEX.CREATE", "k", "100"}},
				writer: [][]string{{"DEBOUNCEX.CALL", "k", "arg"}},
				reader: [][]string{{"DEBOUNCEX.FLUSH", "k"}},
			},
		},
		{
			register: RegisterResilienceCommands,
			registryFamilyCase: registryFamilyCase{
				name:   "windowx",
				setup:  [][]string{{"WINDOWX.CREATE", "k", "10"}},
				writer: [][]string{{"WINDOWX.ADD", "k", "1.5"}},
				reader: [][]string{{"WINDOWX.GET", "k"}},
			},
		},
		{
			register: RegisterResilienceCommands,
			registryFamilyCase: registryFamilyCase{
				name:   "joinx",
				setup:  [][]string{{"JOINX.CREATE", "k"}},
				writer: [][]string{{"JOINX.ADD", "k", "left", "1"}},
				reader: [][]string{{"JOINX.GET", "k"}},
			},
		},
		{
			register: RegisterMoreCommands,
			registryFamilyCase: registryFamilyCase{
				name:   "bucketx",
				setup:  [][]string{{"BUCKETX.CREATE", "k", "1000000", "1000000", "1000000"}},
				writer: [][]string{{"BUCKETX.TAKE", "k", "1"}},
				reader: [][]string{{"BUCKETX.TAKE", "k", "1"}},
			},
		},
		{
			register: RegisterMVCCCommands,
			registryFamilyCase: registryFamilyCase{
				name:   "chain",
				setup:  [][]string{{"CHAIN.CREATE", "k"}},
				writer: [][]string{{"CHAIN.ADD", "k", "data"}},
				reader: [][]string{{"CHAIN.LENGTH", "k"}},
			},
		},
		{
			register: RegisterMVCCCommands,
			registryFamilyCase: registryFamilyCase{
				name:   "analytics",
				setup:  [][]string{{"ANALYTICS.INCR", "k", "1"}},
				writer: [][]string{{"ANALYTICS.INCR", "k", "1"}},
				reader: [][]string{{"ANALYTICS.AVG", "k"}},
			},
		},
		{
			register: RegisterMVCCCommands,
			registryFamilyCase: registryFamilyCase{
				name:   "cooldown",
				setup:  [][]string{{"COOLDOWN.SET", "k", "100000"}},
				writer: [][]string{{"COOLDOWN.CHECK", "k"}},
				reader: [][]string{{"COOLDOWN.CHECK", "k"}},
			},
		},
		{
			register: RegisterExtendedCommands,
			registryFamilyCase: registryFamilyCase{
				name:   "healthx",
				setup:  [][]string{{"HEALTHX.REGISTER", "k", "target", "check"}},
				writer: [][]string{{"HEALTHX.CHECK", "k"}},
				reader: [][]string{{"HEALTHX.STATUS", "k"}},
			},
		},
		{
			register: RegisterExtendedCommands,
			registryFamilyCase: registryFamilyCase{
				name:   "sentinelx",
				setup:  [][]string{{"SENTINELX.WATCH", "k", "target", "100"}},
				writer: [][]string{{"SENTINELX.CONFIG", "k", "target", "200"}},
				reader: [][]string{{"SENTINELX.STATUS", "k"}},
			},
		},
		{
			register: RegisterAdvancedCommands2,
			registryFamilyCase: registryFamilyCase{
				name:   "poolx",
				setup:  [][]string{{"POOLX.CREATE", "k", "1000000"}},
				writer: [][]string{{"POOLX.ACQUIRE", "k"}},
				reader: [][]string{{"POOLX.STATUS", "k"}},
			},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			runRegistryFamilyRace(t, tc.registryFamilyCase, tc.register, false)
		})
	}
}
