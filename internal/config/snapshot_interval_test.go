package config

import "testing"

func TestSnapshotIntervalValidation(t *testing.T) {
	for _, interval := range []string{"", "0", "1ms", "5m", "invalid", "-1s"} {
		t.Run(interval, func(t *testing.T) {
			cfg := Default()
			cfg.Persistence.Enabled = true
			cfg.Persistence.SnapshotInterval = interval
			err := Validate(cfg)
			invalid := interval == "invalid" || interval == "-1s"
			if (err != nil) != invalid {
				t.Fatalf("Validate(snapshot_interval=%q) = %v", interval, err)
			}
		})
	}
}
