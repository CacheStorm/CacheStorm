package config

import (
	"reflect"
	"testing"
)

func checkAudit50R07X(t *testing.T, label string, want, got interface{}) {
	t.Helper()
	t.Logf("%s EXPECTED: %v | ACTUAL: %v", label, want, got)
	if !reflect.DeepEqual(want, got) {
		t.Errorf("%s mismatch", label)
	}
}

func TestAudit50R07X(t *testing.T) {
	defer func() {
		if t.Failed() {
			t.Log("PROBLEM CONFIRMED")
		} else {
			t.Log("FIX VERIFIED")
		}
	}()
	cfg := Default()
	cfg.Memory.MaxMemory = "1mb"
	checkAudit50R07X(t, "control valid memory setting", true, Validate(cfg) == nil)
	cfg.Memory.MaxMemory = "not-a-size"
	_, parseErr := ParseMemorySize(cfg.Memory.MaxMemory)
	checkAudit50R07X(t, "control parser rejects malformed size", true, parseErr != nil)
	checkAudit50R07X(t, "validator rejects malformed memory limit", true, Validate(cfg) != nil)
	for _, value := range []string{"0", "", "1kb", " 1GiB "} {
		cfg.Memory.MaxMemory = value
		checkAudit50R07X(t, "valid memory boundary "+value, true, Validate(cfg) == nil)
	}
	for _, value := range []string{"-1", "-1kb", "9223372036854775807gb"} {
		cfg.Memory.MaxMemory = value
		checkAudit50R07X(t, "invalid memory boundary "+value, true, Validate(cfg) != nil)
	}
	cfg = Default()
	cfg.HTTP.Enabled = false
	cfg.HTTP.Port = -1
	checkAudit50R07X(t, "disabled optional HTTP stays accepted", true, Validate(cfg) == nil)

}
