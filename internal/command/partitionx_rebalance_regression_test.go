package command

import (
	"strings"
	"testing"

	"github.com/cachestorm/cachestorm/internal/store"
)

// PARTITIONX.REBALANCE used to read both of its arguments into `_` and reply
// +OK unconditionally, reporting success for work that never happened. It is
// not implementable: PARTITIONX.ADD stores values only (the key selects an
// index via hashString(key) % Count and is then discarded), so entries cannot
// be re-hashed onto a different partition count. The handler now returns an
// explicit error instead.
//
// These tests assert on the RESP buffer, not the returned error: ctx.WriteError
// writes the error to the writer and returns nil, so asserting on the transport
// result cannot distinguish an error reply from +OK.

// CONTROL: the implemented PARTITIONX commands must still work, so a broken
// harness cannot masquerade as the defect.
func TestRegressionPartitionXAddAndGetStillWork(t *testing.T) {
	s := store.NewStore()
	router := NewRouter()
	RegisterResilienceCommands(router)

	if out := runIntegrationCmd(t, router, s, "PARTITIONX.CREATE", "reg_px", "2"); strings.HasPrefix(out, "-") {
		t.Fatalf("CONTROL: PARTITIONX.CREATE should succeed, got %q", out)
	}
	if out := runIntegrationCmd(t, router, s, "PARTITIONX.ADD", "reg_px", "k1", "v1"); strings.HasPrefix(out, "-") {
		t.Fatalf("CONTROL: PARTITIONX.ADD should succeed, got %q", out)
	}
	got := runIntegrationCmd(t, router, s, "PARTITIONX.GET", "reg_px", "0")
	if !strings.Contains(got, "v1") {
		t.Errorf("CONTROL: PARTITIONX.GET should return the added value, got %q", got)
	}
}

// DEFECT: REBALANCE must answer with an error reply, never +OK, because it
// performs no rebalancing at all.
func TestRegressionPartitionXRebalanceReturnsErrorNotOK(t *testing.T) {
	s := store.NewStore()
	router := NewRouter()
	RegisterResilienceCommands(router)

	out := runIntegrationCmd(t, router, s, "PARTITIONX.REBALANCE", "reg_px", "4")
	if strings.HasPrefix(out, "+") {
		t.Fatalf("FAIL: PARTITIONX.REBALANCE replied %q; it is not implemented and must not report success", out)
	}
	if !strings.HasPrefix(out, "-") {
		t.Errorf("FAIL: PARTITIONX.REBALANCE should answer with an error reply, got %q", out)
	}
	if !strings.Contains(out, "not implemented") {
		t.Errorf("FAIL: the error reply should say the command is not implemented, got %q", out)
	}
}

// The rejected call must leave the partition's contents untouched: REBALANCE
// performs no work, so it must not appear to move values between partitions.
func TestRegressionPartitionXRebalanceLeavesDataUnchanged(t *testing.T) {
	s := store.NewStore()
	router := NewRouter()
	RegisterResilienceCommands(router)

	runIntegrationCmd(t, router, s, "PARTITIONX.CREATE", "reg_px_data", "2")
	runIntegrationCmd(t, router, s, "PARTITIONX.ADD", "reg_px_data", "k1", "v1")

	before := runIntegrationCmd(t, router, s, "PARTITIONX.GET", "reg_px_data", "0")

	reb := runIntegrationCmd(t, router, s, "PARTITIONX.REBALANCE", "reg_px_data", "8")
	if !strings.HasPrefix(reb, "-") {
		t.Fatalf("PARTITIONX.REBALANCE should be rejected, got %q", reb)
	}

	after := runIntegrationCmd(t, router, s, "PARTITIONX.GET", "reg_px_data", "0")
	if before != after {
		t.Errorf("a rejected REBALANCE must not change the partition: before %q, after %q", before, after)
	}
}

// The arity boundary must be unchanged: one argument is still a wrong-argument
// error, distinct from the not-implemented error for two.
func TestRegressionPartitionXRebalanceStillRejectsOneArgument(t *testing.T) {
	s := store.NewStore()
	router := NewRouter()
	RegisterResilienceCommands(router)

	out := runIntegrationCmd(t, router, s, "PARTITIONX.REBALANCE", "reg_px_only")
	if !strings.HasPrefix(out, "-") {
		t.Errorf("PARTITIONX.REBALANCE with 1 argument should be rejected, got %q", out)
	}
	if strings.Contains(out, "not implemented") {
		t.Errorf("a 1-argument call should fail on arity, not report not-implemented, got %q", out)
	}
}
