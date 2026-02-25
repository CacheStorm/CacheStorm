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
	}{
		{"NOTIFY.CREATE basic", "NOTIFY.CREATE", [][]byte{[]byte("channel1"), []byte("email"), []byte("user@example.com")}},
		{"NOTIFY.CREATE no args", "NOTIFY.CREATE", nil},
		{"NOTIFY.CREATE missing args", "NOTIFY.CREATE", [][]byte{[]byte("channel1")}},
		{"NOTIFY.SEND", "NOTIFY.SEND", [][]byte{[]byte("channel1"), []byte("Test message")}},
		{"NOTIFY.SEND not found", "NOTIFY.SEND", [][]byte{[]byte("notfound"), []byte("Test message")}},
		{"NOTIFY.SEND no args", "NOTIFY.SEND", nil},
		{"NOTIFY.LIST empty", "NOTIFY.LIST", nil},
		{"NOTIFY.LIST with pattern", "NOTIFY.LIST", [][]byte{[]byte("channel*")}},
		{"NOTIFY.DELETE success", "NOTIFY.DELETE", [][]byte{[]byte("channel1")}},
		{"NOTIFY.DELETE not found", "NOTIFY.DELETE", [][]byte{[]byte("notfound")}},
		{"NOTIFY.DELETE no args", "NOTIFY.DELETE", nil},
		{"NOTIFY.TEMPLATE", "NOTIFY.TEMPLATE", [][]byte{[]byte("template1"), []byte("Hello {{name}}")}},
		{"NOTIFY.TEMPLATE no args", "NOTIFY.TEMPLATE", nil},
	}

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
	}{
		{"SCHEMA.REGISTER basic", "SCHEMA.REGISTER", [][]byte{[]byte("schema1"), []byte(`{"type":"object"}`)}},
		{"SCHEMA.REGISTER no args", "SCHEMA.REGISTER", nil},
		{"SCHEMA.VALIDATE valid", "SCHEMA.VALIDATE", [][]byte{[]byte("schema1"), []byte(`{"key":"value"}`)}},
		{"SCHEMA.VALIDATE invalid", "SCHEMA.VALIDATE", [][]byte{[]byte("schema1"), []byte(`invalid`)}},
		{"SCHEMA.VALIDATE not found", "SCHEMA.VALIDATE", [][]byte{[]byte("notfound"), []byte(`{}`)}},
		{"SCHEMA.VALIDATE no args", "SCHEMA.VALIDATE", nil},
		{"SCHEMA.DELETE success", "SCHEMA.DELETE", [][]byte{[]byte("schema1")}},
		{"SCHEMA.DELETE not found", "SCHEMA.DELETE", [][]byte{[]byte("notfound")}},
		{"SCHEMA.DELETE no args", "SCHEMA.DELETE", nil},
	}

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
	}{
		{"PIPELINE.CREATE basic", "PIPELINE.CREATE", [][]byte{[]byte("pipe1")}},
		{"PIPELINE.CREATE no args", "PIPELINE.CREATE", nil},
		{"PIPELINE.ADDSTAGE", "PIPELINE.ADDSTAGE", [][]byte{[]byte("pipe1"), []byte("stage1"), []byte("filter")}},
		{"PIPELINE.ADDSTAGE not found", "PIPELINE.ADDSTAGE", [][]byte{[]byte("notfound"), []byte("stage1"), []byte("filter")}},
		{"PIPELINE.ADDSTAGE no args", "PIPELINE.ADDSTAGE", nil},
		{"PIPELINE.EXECUTE", "PIPELINE.EXECUTE", [][]byte{[]byte("pipe1"), []byte("data")}},
		{"PIPELINE.EXECUTE not found", "PIPELINE.EXECUTE", [][]byte{[]byte("notfound"), []byte("data")}},
		{"PIPELINE.EXECUTE no args", "PIPELINE.EXECUTE", nil},
		{"PIPELINE.STATUS", "PIPELINE.STATUS", [][]byte{[]byte("pipe1")}},
		{"PIPELINE.STATUS not found", "PIPELINE.STATUS", [][]byte{[]byte("notfound")}},
		{"PIPELINE.STATUS no args", "PIPELINE.STATUS", nil},
		{"PIPELINE.DELETE success", "PIPELINE.DELETE", [][]byte{[]byte("pipe1")}},
		{"PIPELINE.DELETE not found", "PIPELINE.DELETE", [][]byte{[]byte("notfound")}},
		{"PIPELINE.DELETE no args", "PIPELINE.DELETE", nil},
	}

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
