package command

import (
	"errors"
	"math/rand/v2"
	"sync/atomic"

	"github.com/cachestorm/cachestorm/internal/hashslot"
)

// Redis 8's cluster compatibility statistic: a standalone server set with
// cluster-compatibility-sample-ratio counts, in INFO's
// cluster_incompatible_ops, the operations that would fail on a cluster —
// keys of one command, transaction or script in different slots, databases
// other than 0, SORT patterns reaching other slots and no-cluster scripts —
// so an application can be checked before it moves to one.
var (
	clusterCompatRatio     atomic.Int64
	clusterIncompatibleOps atomic.Int64
)

func noteClusterIncompatible() { clusterIncompatibleOps.Add(1) }

// sampleClusterCompat is Redis' SHOULD_CLUSTER_COMPATIBILITY_SAMPLE.
func sampleClusterCompat() bool {
	r := clusterCompatRatio.Load()
	return r == 100 || rand.Float64()*100 < float64(r)
}

// clusterCompatSampling reports whether a standalone server samples
// operations for the statistic.
func (ctx *Context) clusterCompatSampling() bool {
	return clusterCompatRatio.Load() != 0 && ctx.clusterState() == nil
}

// The slot a command checked for the statistic keeps for the scripts it
// runs: compatOff when it was not checked or already counted, compatNoSlot
// when it named no key, otherwise the slot plus one.
const (
	compatOff    = 0
	compatNoSlot = -1
)

// checkClusterCompat counts the command in ctx when its keys, or those of
// the transaction EXEC runs, are in different slots, as Redis' processCommand
// does for a sampled command.
func checkClusterCompat(ctx *Context) {
	if ctx.Command != "EXEC" && !clusterHasKeys(ctx.Command) {
		return
	}
	if !sampleClusterCompat() {
		return
	}
	cmds, _, _ := clusterCommands(ctx)
	slot, ok := sameSlot(cmds)
	switch {
	case !ok:
		noteClusterIncompatible()
		ctx.compatSlot = compatOff
	case slot < 0:
		ctx.compatSlot = compatNoSlot
	default:
		ctx.compatSlot = slot + 1
	}
}

// sameSlot is Redis' areCommandKeysInSameSlot: the slot of the keys of cmds,
// -1 when they name none, and false when they are in different slots.
func sameSlot(cmds []queuedCommand) (int, bool) {
	slot := -1
	for _, qc := range cmds {
		for _, key := range clusterKeys(qc.cmd, qc.args) {
			s := hashslot.Key(key)
			if slot == -1 {
				slot = s
			} else if s != slot {
				return 0, false
			}
		}
	}
	return slot, true
}

// compatCheck is Redis' scriptCheckClusterCompatibility for one command a
// script runs: its keys must share a slot, and, unless the script allows
// cross-slot keys, the slot of the keys the script declared or used before.
func (s *scriptRun) compatCheck(name string, args [][]byte) {
	if s.compatSlot == compatOff {
		return
	}
	slot, ok := sameSlot([]queuedCommand{{cmd: name, args: args}})
	if !ok {
		noteClusterIncompatible()
		s.compatSlot = compatOff
		return
	}
	if slot < 0 || s.crossSlot {
		return
	}
	if s.compatSlot == compatNoSlot {
		s.compatSlot = slot + 1
	} else if s.compatSlot != slot+1 {
		noteClusterIncompatible()
		s.compatSlot = compatOff
	}
}

// patternSlot is Redis' patternHashSlot: the slot every key a SORT BY or GET
// pattern forms is in, or -1 when they may be in different slots.
func patternSlot(p string) int {
	open := -1
	for i := 0; i < len(p); i++ {
		switch c := p[i]; {
		case c == '*' || c == '?' || c == '[' || c == '\\':
			return -1
		case open == -1 && c == '{':
			open = i
		case open >= 0 && c == '}' && i == open+1:
			open = -2
		case open >= 0 && c == '}':
			return int(hashslot.CRC16(p[open+1:i]) & 0x3FFF)
		}
	}
	return int(hashslot.CRC16(p) & 0x3FFF)
}

// sortPatternSlotCheck refuses, on a cluster, a SORT BY or GET pattern whose
// keys may be outside the slot of the sorted key, and counts one for the
// statistic on a standalone server.
func sortPatternSlotCheck(ctx *Context, key, pattern, option string) error {
	if ctx.clusterRefuses() {
		if patternSlot(pattern) != hashslot.Key(key) {
			return errors.New("ERR " + option + " option of SORT denied in Cluster mode when keys formed by the pattern may be in different slots.")
		}
		return nil
	}
	if ctx.clusterCompatSampling() && sampleClusterCompat() && patternSlot(pattern) != hashslot.Key(key) {
		noteClusterIncompatible()
	}
	return nil
}
