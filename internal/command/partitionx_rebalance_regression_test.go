package command

import (
	"strings"
	"testing"

	"github.com/cachestorm/cachestorm/internal/store"
)

// The PARTITIONX commands route a key to a partition and read it back.
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
