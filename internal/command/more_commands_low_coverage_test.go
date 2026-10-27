package command

import (
	"testing"

	"github.com/cachestorm/cachestorm/internal/store"
)

func TestMoreCommandsBucketXLowCoverage(t *testing.T) {
	s := store.NewStore()
	router := NewRouter()
	RegisterMoreCommands(router)

	tests := []struct {
		name string
		cmd  string
		args [][]byte
	}{
		{"BUCKETX.CREATE basic", "BUCKETX.CREATE", [][]byte{[]byte("bucket1"), []byte("100"), []byte("10"), []byte("1000")}},
		{"BUCKETX.CREATE no args", "BUCKETX.CREATE", nil},
		{"BUCKETX.CREATE missing args", "BUCKETX.CREATE", [][]byte{[]byte("bucket2"), []byte("100")}},
		{"BUCKETX.TAKE success", "BUCKETX.TAKE", [][]byte{[]byte("bucket1"), []byte("10")}},
		{"BUCKETX.TAKE too many", "BUCKETX.TAKE", [][]byte{[]byte("bucket1"), []byte("200")}},
		{"BUCKETX.TAKE not found", "BUCKETX.TAKE", [][]byte{[]byte("notfound"), []byte("10")}},
		{"BUCKETX.TAKE no args", "BUCKETX.TAKE", nil},
		{"BUCKETX.RETURN success", "BUCKETX.RETURN", [][]byte{[]byte("bucket1"), []byte("5")}},
		{"BUCKETX.RETURN not found", "BUCKETX.RETURN", [][]byte{[]byte("notfound"), []byte("5")}},
		{"BUCKETX.RETURN no args", "BUCKETX.RETURN", nil},
		{"BUCKETX.REFILL success", "BUCKETX.REFILL", [][]byte{[]byte("bucket1")}},
		{"BUCKETX.REFILL not found", "BUCKETX.REFILL", [][]byte{[]byte("notfound")}},
		{"BUCKETX.REFILL no args", "BUCKETX.REFILL", nil},
		{"BUCKETX.DELETE success", "BUCKETX.DELETE", [][]byte{[]byte("bucket1")}},
		{"BUCKETX.DELETE not found", "BUCKETX.DELETE", [][]byte{[]byte("notfound")}},
		{"BUCKETX.DELETE no args", "BUCKETX.DELETE", nil},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			runCommandTest(t, router, s, tt.cmd, tt.args)
		})
	}
}

func TestMoreCommandsIdempotencyLowCoverage(t *testing.T) {
	s := store.NewStore()
	router := NewRouter()
	RegisterMoreCommands(router)

	tests := []struct {
		name string
		cmd  string
		args [][]byte
	}{
		{"IDEMPOTENCY.SET basic", "IDEMPOTENCY.SET", [][]byte{[]byte("key1"), []byte("value1"), []byte("3600")}},
		{"IDEMPOTENCY.SET no args", "IDEMPOTENCY.SET", nil},
		{"IDEMPOTENCY.SET missing args", "IDEMPOTENCY.SET", [][]byte{[]byte("key1")}},
		{"IDEMPOTENCY.GET exists", "IDEMPOTENCY.GET", [][]byte{[]byte("key1")}},
		{"IDEMPOTENCY.GET not found", "IDEMPOTENCY.GET", [][]byte{[]byte("notfound")}},
		{"IDEMPOTENCY.GET no args", "IDEMPOTENCY.GET", nil},
		{"IDEMPOTENCY.CHECK exists", "IDEMPOTENCY.CHECK", [][]byte{[]byte("key1")}},
		{"IDEMPOTENCY.CHECK not found", "IDEMPOTENCY.CHECK", [][]byte{[]byte("notfound")}},
		{"IDEMPOTENCY.CHECK no args", "IDEMPOTENCY.CHECK", nil},
		{"IDEMPOTENCY.DELETE success", "IDEMPOTENCY.DELETE", [][]byte{[]byte("key1")}},
		{"IDEMPOTENCY.DELETE not found", "IDEMPOTENCY.DELETE", [][]byte{[]byte("notfound")}},
		{"IDEMPOTENCY.DELETE no args", "IDEMPOTENCY.DELETE", nil},
		{"IDEMPOTENCY.LIST empty", "IDEMPOTENCY.LIST", nil},
		{"IDEMPOTENCY.LIST with pattern", "IDEMPOTENCY.LIST", [][]byte{[]byte("key*")}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			runCommandTest(t, router, s, tt.cmd, tt.args)
		})
	}
}

func TestMoreCommandsNotifyLowCoverage(t *testing.T) {
	s := store.NewStore()
	router := NewRouter()
	RegisterMoreCommands(router)

	tests := []struct {
		name string
		cmd  string
		args [][]byte
	}{}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			runCommandTest(t, router, s, tt.cmd, tt.args)
		})
	}
}

func TestMoreCommandsSlidingWindowLowCoverage(t *testing.T) {
	s := store.NewStore()
	router := NewRouter()
	RegisterMoreCommands(router)

	tests := []struct {
		name string
		cmd  string
		args [][]byte
	}{
		{"SLIDING.CHECK basic", "SLIDING.CHECK", [][]byte{[]byte("window1"), []byte("100"), []byte("60000")}},
		{"SLIDING.CHECK no args", "SLIDING.CHECK", nil},
		{"SLIDING.CHECK missing args", "SLIDING.CHECK", [][]byte{[]byte("window1")}},
		{"SLIDING.RESET success", "SLIDING.RESET", [][]byte{[]byte("window1")}},
		{"SLIDING.RESET not found", "SLIDING.RESET", [][]byte{[]byte("notfound")}},
		{"SLIDING.RESET no args", "SLIDING.RESET", nil},
		{"SLIDING.DELETE success", "SLIDING.DELETE", [][]byte{[]byte("window1")}},
		{"SLIDING.DELETE not found", "SLIDING.DELETE", [][]byte{[]byte("notfound")}},
		{"SLIDING.DELETE no args", "SLIDING.DELETE", nil},
		{"SLIDING.STATS exists", "SLIDING.STATS", [][]byte{[]byte("window1")}},
		{"SLIDING.STATS not found", "SLIDING.STATS", [][]byte{[]byte("notfound")}},
		{"SLIDING.STATS no args", "SLIDING.STATS", nil},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			runCommandTest(t, router, s, tt.cmd, tt.args)
		})
	}
}

func TestMoreCommandsExperimentLowCoverage(t *testing.T) {
	s := store.NewStore()
	router := NewRouter()
	RegisterMoreCommands(router)

	tests := []struct {
		name string
		cmd  string
		args [][]byte
	}{
		{"EXPERIMENT.CREATE basic", "EXPERIMENT.CREATE", [][]byte{[]byte("exp1"), []byte("A/B")}},
		{"EXPERIMENT.CREATE no args", "EXPERIMENT.CREATE", nil},
		{"EXPERIMENT.DELETE success", "EXPERIMENT.DELETE", [][]byte{[]byte("exp1")}},
		{"EXPERIMENT.DELETE not found", "EXPERIMENT.DELETE", [][]byte{[]byte("notfound")}},
		{"EXPERIMENT.DELETE no args", "EXPERIMENT.DELETE", nil},
		{"EXPERIMENT.ASSIGN", "EXPERIMENT.ASSIGN", [][]byte{[]byte("exp1"), []byte("user1")}},
		{"EXPERIMENT.ASSIGN not found", "EXPERIMENT.ASSIGN", [][]byte{[]byte("notfound"), []byte("user1")}},
		{"EXPERIMENT.ASSIGN no args", "EXPERIMENT.ASSIGN", nil},
		{"EXPERIMENT.TRACK", "EXPERIMENT.TRACK", [][]byte{[]byte("exp1"), []byte("user1"), []byte("conversion")}},
		{"EXPERIMENT.TRACK no args", "EXPERIMENT.TRACK", nil},
		{"EXPERIMENT.RESULTS", "EXPERIMENT.RESULTS", [][]byte{[]byte("exp1")}},
		{"EXPERIMENT.RESULTS not found", "EXPERIMENT.RESULTS", [][]byte{[]byte("notfound")}},
		{"EXPERIMENT.RESULTS no args", "EXPERIMENT.RESULTS", nil},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			runCommandTest(t, router, s, tt.cmd, tt.args)
		})
	}
}

func TestMoreCommandsRolloutLowCoverage(t *testing.T) {
	s := store.NewStore()
	router := NewRouter()
	RegisterMoreCommands(router)

	tests := []struct {
		name string
		cmd  string
		args [][]byte
	}{
		{"ROLLOUT.CREATE basic", "ROLLOUT.CREATE", [][]byte{[]byte("feature1")}},
		{"ROLLOUT.CREATE no args", "ROLLOUT.CREATE", nil},
		{"ROLLOUT.DELETE success", "ROLLOUT.DELETE", [][]byte{[]byte("feature1")}},
		{"ROLLOUT.DELETE not found", "ROLLOUT.DELETE", [][]byte{[]byte("notfound")}},
		{"ROLLOUT.DELETE no args", "ROLLOUT.DELETE", nil},
		{"ROLLOUT.CHECK", "ROLLOUT.CHECK", [][]byte{[]byte("feature1"), []byte("user1")}},
		{"ROLLOUT.CHECK not found", "ROLLOUT.CHECK", [][]byte{[]byte("notfound"), []byte("user1")}},
		{"ROLLOUT.CHECK no args", "ROLLOUT.CHECK", nil},
		{"ROLLOUT.PERCENTAGE", "ROLLOUT.PERCENTAGE", [][]byte{[]byte("feature1"), []byte("50")}},
		{"ROLLOUT.PERCENTAGE not found", "ROLLOUT.PERCENTAGE", [][]byte{[]byte("notfound"), []byte("50")}},
		{"ROLLOUT.PERCENTAGE no args", "ROLLOUT.PERCENTAGE", nil},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			runCommandTest(t, router, s, tt.cmd, tt.args)
		})
	}
}

func TestMoreCommandsSchemaLowCoverage(t *testing.T) {
	s := store.NewStore()
	router := NewRouter()
	RegisterMoreCommands(router)

	tests := []struct {
		name string
		cmd  string
		args [][]byte
	}{}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			runCommandTest(t, router, s, tt.cmd, tt.args)
		})
	}
}

func TestMoreCommandsPipelineLowCoverage(t *testing.T) {
	s := store.NewStore()
	router := NewRouter()
	RegisterMoreCommands(router)

	tests := []struct {
		name string
		cmd  string
		args [][]byte
	}{}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			runCommandTest(t, router, s, tt.cmd, tt.args)
		})
	}
}

func TestMoreCommandsAlertLowCoverage(t *testing.T) {
	s := store.NewStore()
	router := NewRouter()
	RegisterMoreCommands(router)

	tests := []struct {
		name string
		cmd  string
		args [][]byte
	}{
		{"ALERT.CREATE basic", "ALERT.CREATE", [][]byte{[]byte("alert1"), []byte("threshold > 100")}},
		{"ALERT.CREATE no args", "ALERT.CREATE", nil},
		{"ALERT.TRIGGER", "ALERT.TRIGGER", [][]byte{[]byte("alert1"), []byte("150")}},
		{"ALERT.TRIGGER not found", "ALERT.TRIGGER", [][]byte{[]byte("notfound"), []byte("150")}},
		{"ALERT.TRIGGER no args", "ALERT.TRIGGER", nil},
		{"ALERT.ACKNOWLEDGE", "ALERT.ACKNOWLEDGE", [][]byte{[]byte("alert1")}},
		{"ALERT.ACKNOWLEDGE not found", "ALERT.ACKNOWLEDGE", [][]byte{[]byte("notfound")}},
		{"ALERT.ACKNOWLEDGE no args", "ALERT.ACKNOWLEDGE", nil},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			runCommandTest(t, router, s, tt.cmd, tt.args)
		})
	}
}
