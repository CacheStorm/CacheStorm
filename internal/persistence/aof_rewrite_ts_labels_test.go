package persistence_test

import (
	"testing"

	"github.com/cachestorm/cachestorm/internal/persistence"
	"github.com/cachestorm/cachestorm/internal/store"
)

type tsLabelAdapter struct {
	entries map[string]*store.Entry
}

func (a *tsLabelAdapter) GetAll() map[string]interface{} {
	out := make(map[string]interface{}, len(a.entries))
	for k, v := range a.entries {
		out[k] = v
	}
	return out
}

func tsLabelRewriteCommands(t *testing.T, src *store.Store) []persistence.Command {
	t.Helper()
	dir := t.TempDir()
	aofPath := dir + "/appendonly.aof"
	rw := persistence.NewAOFRewriter(persistence.AOFConfig{DataDir: dir}, &tsLabelAdapter{entries: src.GetAll()})
	if err := rw.Rewrite(aofPath); err != nil {
		t.Fatalf("Rewrite failed: %v", err)
	}
	cmds, err := persistence.NewAOFReader().Load(aofPath)
	if err != nil {
		t.Fatalf("Load failed: %v", err)
	}
	return cmds
}

func tsFindCmd(cmds []persistence.Command, name string) *persistence.Command {
	for i := range cmds {
		if cmds[i].Name == name {
			return &cmds[i]
		}
	}
	return nil
}

func tsArgStrings(c *persistence.Command) []string {
	out := make([]string, len(c.Args))
	for i, a := range c.Args {
		out[i] = string(a)
	}
	return out
}

func tsHasLabelPair(args []string, k, v string) bool {
	for i := 0; i < len(args); i++ {
		if args[i] != "LABELS" {
			continue
		}
		for j := i + 1; j+1 < len(args); j += 2 {
			if args[j] == k && args[j+1] == v {
				return true
			}
		}
	}
	return false
}

// AOF rewrite must preserve per-sample time-series labels. The RDB path writes
// them (rdb.go:340-346) and reads them back (rdb.go:727-737), so RDB is the
// reference behaviour — but the AOF rewriter emitted TS.ADD with only the
// timestamp and value, silently destroying labels the RDB keeps.
func TestAOFRewritePreservesTimeSeriesSampleLabels(t *testing.T) {
	src := store.NewStore()
	ts := store.NewTimeSeriesValue(0)
	ts.AddWithLabels(1000, 1.5, map[string]string{"sensor": "a1"})
	src.Set("ts", ts, store.SetOptions{})

	add := tsFindCmd(tsLabelRewriteCommands(t, src), "TS.ADD")
	if add == nil {
		t.Fatal("the rewritten AOF contains no TS.ADD")
	}
	if !tsHasLabelPair(tsArgStrings(add), "sensor", "a1") {
		t.Fatalf("the rewritten TS.ADD lost the per-sample label: %v", tsArgStrings(add))
	}
}

// Each sample keeps its OWN labels — a shared-label bug would pass a
// single-sample test.
func TestAOFRewriteKeepsPerSampleLabelsDistinct(t *testing.T) {
	src := store.NewStore()
	ts := store.NewTimeSeriesValue(0)
	ts.AddWithLabels(1000, 1.5, map[string]string{"sensor": "a1"})
	ts.AddWithLabels(2000, 2.5, map[string]string{"sensor": "a2"})
	src.Set("ts", ts, store.SetOptions{})

	var addCmds [][]string
	for _, c := range tsLabelRewriteCommands(t, src) {
		if c.Name == "TS.ADD" {
			addCmds = append(addCmds, tsArgStrings(&c))
		}
	}
	if len(addCmds) != 2 {
		t.Fatalf("found %d TS.ADD commands, want 2", len(addCmds))
	}

	foundA1, foundA2 := false, false
	for _, args := range addCmds {
		switch {
		case tsHasLabelPair(args, "sensor", "a1"):
			foundA1 = true
		case tsHasLabelPair(args, "sensor", "a2"):
			foundA2 = true
		}
	}
	if !foundA1 || !foundA2 {
		t.Fatalf("per-sample labels were not kept distinct: %v", addCmds)
	}
}

// A sample may carry several labels; every pair must survive.
func TestAOFRewritePreservesMultipleSampleLabels(t *testing.T) {
	src := store.NewStore()
	ts := store.NewTimeSeriesValue(0)
	ts.AddWithLabels(1000, 1.5, map[string]string{"sensor": "a1", "region": "eu"})
	src.Set("ts", ts, store.SetOptions{})

	add := tsFindCmd(tsLabelRewriteCommands(t, src), "TS.ADD")
	if add == nil {
		t.Fatal("the rewritten AOF contains no TS.ADD")
	}
	args := tsArgStrings(add)
	if !tsHasLabelPair(args, "sensor", "a1") || !tsHasLabelPair(args, "region", "eu") {
		t.Fatalf("a multi-label sample lost a label: %v", args)
	}
}

// LABELS must be the LAST clause: cmdTSADD reads label pairs from the keyword
// to the end of the argument list, so a trailing argument would be swallowed as
// another label instead of being parsed.
func TestAOFTSAddLabelsComeLast(t *testing.T) {
	src := store.NewStore()
	ts := store.NewTimeSeriesValue(0)
	ts.AddWithLabels(1000, 1.5, map[string]string{"sensor": "a1"})
	src.Set("ts", ts, store.SetOptions{})

	add := tsFindCmd(tsLabelRewriteCommands(t, src), "TS.ADD")
	if add == nil {
		t.Fatal("the rewritten AOF contains no TS.ADD")
	}
	args := tsArgStrings(add)

	idx := -1
	for i, a := range args {
		if a == "LABELS" {
			idx = i
			break
		}
	}
	if idx < 0 {
		t.Fatalf("no LABELS clause in %v", args)
	}
	if rest := args[idx+1:]; len(rest)%2 != 0 {
		t.Fatalf("LABELS is followed by an odd number of args, which cmdTSADD cannot pair: %v", args)
	}
}

// Control: series-level labels are still persisted via TS.CREATE — the in-repo
// basis for expecting per-sample labels to be persisted too.
func TestAOFSeriesLabelsStillPersistedControl(t *testing.T) {
	src := store.NewStore()
	ts := store.NewTimeSeriesValue(0)
	ts.SetLabels(map[string]string{"region": "eu"})
	ts.Add(1000, 1.5)
	src.Set("ts", ts, store.SetOptions{})

	create := tsFindCmd(tsLabelRewriteCommands(t, src), "TS.CREATE")
	if create == nil {
		t.Fatal("the rewritten AOF contains no TS.CREATE")
	}
	if !tsHasLabelPair(tsArgStrings(create), "region", "eu") {
		t.Fatalf("TS.CREATE lost the series labels: %v", tsArgStrings(create))
	}
}

// Control: an unlabelled sample must not gain a LABELS clause — the fix must
// not invent labels.
func TestAOFUnlabelledSampleEmitsNoLabelsControl(t *testing.T) {
	src := store.NewStore()
	ts := store.NewTimeSeriesValue(0)
	ts.Add(1000, 1.5)
	src.Set("ts", ts, store.SetOptions{})

	add := tsFindCmd(tsLabelRewriteCommands(t, src), "TS.ADD")
	if add == nil {
		t.Fatal("the rewritten AOF contains no TS.ADD")
	}
	for _, a := range tsArgStrings(add) {
		if a == "LABELS" {
			t.Fatalf("an unlabelled sample gained a LABELS clause: %v", tsArgStrings(add))
		}
	}
}

// Control: timestamp and value are preserved for every sample — this isolates
// the defect to labels rather than the whole sample.
func TestAOFTimeSeriesSamplesPreservedControl(t *testing.T) {
	src := store.NewStore()
	ts := store.NewTimeSeriesValue(0)
	ts.AddWithLabels(1000, 1.5, map[string]string{"sensor": "a1"})
	ts.AddWithLabels(2000, 2.5, map[string]string{"sensor": "a2"})
	src.Set("ts", ts, store.SetOptions{})

	var addCmds [][]string
	for _, c := range tsLabelRewriteCommands(t, src) {
		if c.Name == "TS.ADD" {
			addCmds = append(addCmds, tsArgStrings(&c))
		}
	}
	if len(addCmds) != 2 {
		t.Fatalf("found %d TS.ADD commands, want 2", len(addCmds))
	}
	for _, args := range addCmds {
		if len(args) < 3 {
			t.Fatalf("TS.ADD has %d args, want at least 3 (key ts value): %v", len(args), args)
		}
		if args[0] != "ts" {
			t.Fatalf("TS.ADD targets key %q, want %q", args[0], "ts")
		}
	}
}
