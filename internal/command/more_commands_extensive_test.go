package command

import (
	"testing"

	"github.com/cachestorm/cachestorm/internal/store"
)

func TestWorkflowCommandsExtensive(t *testing.T) {
	s := store.NewStore()
	router := NewRouter()
	RegisterWorkflowCommands(router)

	tests := []struct {
		name string
		cmd  string
		args [][]byte
	}{
		{"WORKFLOW.CREATE no args", "WORKFLOW.CREATE", nil},
		{"WORKFLOW.CREATE workflow", "WORKFLOW.CREATE", [][]byte{[]byte("wf1")}},
		{"WORKFLOW.ADDSTEP no args", "WORKFLOW.ADDSTEP", nil},
		{"WORKFLOW.ADDSTEP missing args", "WORKFLOW.ADDSTEP", [][]byte{[]byte("wf1")}},
		{"WORKFLOW.START no args", "WORKFLOW.START", nil},
		{"WORKFLOW.START not found", "WORKFLOW.START", [][]byte{[]byte("notfound")}},
		{"WORKFLOW.EXECUTE no args", "WORKFLOW.EXECUTE", nil},
		{"WORKFLOW.EXECUTE not found", "WORKFLOW.EXECUTE", [][]byte{[]byte("notfound")}},
		{"WORKFLOW.STATUS no args", "WORKFLOW.STATUS", nil},
		{"WORKFLOW.STATUS not found", "WORKFLOW.STATUS", [][]byte{[]byte("notfound")}},
		{"WORKFLOW.PAUSE no args", "WORKFLOW.PAUSE", nil},
		{"WORKFLOW.PAUSE not found", "WORKFLOW.PAUSE", [][]byte{[]byte("notfound")}},
		{"WORKFLOW.RESUME no args", "WORKFLOW.RESUME", nil},
		{"WORKFLOW.RESUME not found", "WORKFLOW.RESUME", [][]byte{[]byte("notfound")}},
		{"WORKFLOW.CANCEL no args", "WORKFLOW.CANCEL", nil},
		{"WORKFLOW.CANCEL not found", "WORKFLOW.CANCEL", [][]byte{[]byte("notfound")}},
		{"WORKFLOW.LIST no args", "WORKFLOW.LIST", nil},
		{"WORKFLOW.DELETE no args", "WORKFLOW.DELETE", nil},
		{"WORKFLOW.DELETE not found", "WORKFLOW.DELETE", [][]byte{[]byte("notfound")}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			runCommandTest(t, router, s, tt.cmd, tt.args)
		})
	}
}

func TestSchedulerCommandsExtensive(t *testing.T) {
	s := store.NewStore()
	router := NewRouter()
	RegisterSchedulerCommands(router)

	tests := []struct {
		name string
		cmd  string
		args [][]byte
	}{
		{"SCHEDULER.CREATE no args", "SCHEDULER.CREATE", nil},
		{"SCHEDULER.CREATE scheduler", "SCHEDULER.CREATE", [][]byte{[]byte("sched1")}},
		{"SCHEDULER.ADD no args", "SCHEDULER.ADD", nil},
		{"SCHEDULER.ADD missing args", "SCHEDULER.ADD", [][]byte{[]byte("sched1")}},
		{"SCHEDULER.REMOVE no args", "SCHEDULER.REMOVE", nil},
		{"SCHEDULER.REMOVE not found", "SCHEDULER.REMOVE", [][]byte{[]byte("notfound"), []byte("job1")}},
		{"SCHEDULER.RUN no args", "SCHEDULER.RUN", nil},
		{"SCHEDULER.RUN not found", "SCHEDULER.RUN", [][]byte{[]byte("notfound"), []byte("job1")}},
		{"SCHEDULER.LIST no args", "SCHEDULER.LIST", nil},
		{"SCHEDULER.LIST not found", "SCHEDULER.LIST", [][]byte{[]byte("notfound")}},
		{"SCHEDULER.PAUSE no args", "SCHEDULER.PAUSE", nil},
		{"SCHEDULER.PAUSE not found", "SCHEDULER.PAUSE", [][]byte{[]byte("notfound")}},
		{"SCHEDULER.RESUME no args", "SCHEDULER.RESUME", nil},
		{"SCHEDULER.RESUME not found", "SCHEDULER.RESUME", [][]byte{[]byte("notfound")}},
		{"SCHEDULER.DELETE no args", "SCHEDULER.DELETE", nil},
		{"SCHEDULER.DELETE not found", "SCHEDULER.DELETE", [][]byte{[]byte("notfound")}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			runCommandTest(t, router, s, tt.cmd, tt.args)
		})
	}
}

func TestTemplateCommandsExtensive(t *testing.T) {
	s := store.NewStore()
	router := NewRouter()
	RegisterTemplateCommands(router)

	tests := []struct {
		name string
		cmd  string
		args [][]byte
	}{
		{"TEMPLATE.CREATE no args", "TEMPLATE.CREATE", nil},
		{"TEMPLATE.CREATE template", "TEMPLATE.CREATE", [][]byte{[]byte("tpl1"), []byte("Hello {{name}}")}},
		{"TEMPLATE.RENDER no args", "TEMPLATE.RENDER", nil},
		{"TEMPLATE.RENDER not found", "TEMPLATE.RENDER", [][]byte{[]byte("notfound"), []byte(`{"name":"World"}`)}},
		{"TEMPLATE.GET no args", "TEMPLATE.GET", nil},
		{"TEMPLATE.GET not found", "TEMPLATE.GET", [][]byte{[]byte("notfound")}},
		{"TEMPLATE.DELETE no args", "TEMPLATE.DELETE", nil},
		{"TEMPLATE.DELETE not found", "TEMPLATE.DELETE", [][]byte{[]byte("notfound")}},
		{"TEMPLATE.LIST no args", "TEMPLATE.LIST", nil},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			runCommandTest(t, router, s, tt.cmd, tt.args)
		})
	}
}

func TestMonitoringCommandsExtensive(t *testing.T) {
	s := store.NewStore()
	router := NewRouter()
	RegisterMonitoringCommands(router)

	tests := []struct {
		name string
		cmd  string
		args [][]byte
	}{
		{"METRIC.CREATE no args", "METRIC.CREATE", nil},
		{"METRIC.CREATE metric", "METRIC.CREATE", [][]byte{[]byte("metric1"), []byte("counter")}},
		{"METRIC.RECORD no args", "METRIC.RECORD", nil},
		{"METRIC.RECORD not found", "METRIC.RECORD", [][]byte{[]byte("notfound"), []byte("100")}},
		{"METRIC.GET no args", "METRIC.GET", nil},
		{"METRIC.GET not found", "METRIC.GET", [][]byte{[]byte("notfound")}},
		{"METRIC.RESET no args", "METRIC.RESET", nil},
		{"METRIC.RESET not found", "METRIC.RESET", [][]byte{[]byte("notfound")}},
		{"METRIC.DELETE no args", "METRIC.DELETE", nil},
		{"METRIC.DELETE not found", "METRIC.DELETE", [][]byte{[]byte("notfound")}},
		{"METRIC.LIST no args", "METRIC.LIST", nil},
		{"ALERT.CREATE no args", "ALERT.CREATE", nil},
		{"ALERT.CREATE alert", "ALERT.CREATE", [][]byte{[]byte("alert1"), []byte("metric > 100")}},
		{"ALERT.TRIGGER no args", "ALERT.TRIGGER", nil},
		{"ALERT.TRIGGER not found", "ALERT.TRIGGER", [][]byte{[]byte("notfound"), []byte("150")}},
		{"ALERT.ACK no args", "ALERT.ACK", nil},
		{"ALERT.ACK not found", "ALERT.ACK", [][]byte{[]byte("notfound")}},
		{"ALERT.STATUS no args", "ALERT.STATUS", nil},
		{"ALERT.STATUS not found", "ALERT.STATUS", [][]byte{[]byte("notfound")}},
		{"ALERT.DELETE no args", "ALERT.DELETE", nil},
		{"ALERT.DELETE not found", "ALERT.DELETE", [][]byte{[]byte("notfound")}},
		{"ALERT.LIST no args", "ALERT.LIST", nil},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			runCommandTest(t, router, s, tt.cmd, tt.args)
		})
	}
}

func TestEncodingCommandsExtensive(t *testing.T) {
	s := store.NewStore()
	router := NewRouter()
	RegisterEncodingCommands(router)

	tests := []struct {
		name string
		cmd  string
		args [][]byte
	}{
		{"JSON.ENCODE no args", "JSON.ENCODE", nil},
		{"JSON.ENCODE data", "JSON.ENCODE", [][]byte{[]byte(`{"key":"value"}`)}},
		{"JSON.DECODE no args", "JSON.DECODE", nil},
		{"JSON.DECODE invalid", "JSON.DECODE", [][]byte{[]byte("invalid")}},
		{"XML.ENCODE no args", "XML.ENCODE", nil},
		{"XML.ENCODE data", "XML.ENCODE", [][]byte{[]byte(`<root><item>value</item></root>`)}},
		{"XML.DECODE no args", "XML.DECODE", nil},
		{"XML.DECODE invalid", "XML.DECODE", [][]byte{[]byte("invalid")}},
		{"YAML.ENCODE no args", "YAML.ENCODE", nil},
		{"YAML.ENCODE data", "YAML.ENCODE", [][]byte{[]byte("key: value")}},
		{"YAML.DECODE no args", "YAML.DECODE", nil},
		{"YAML.DECODE invalid", "YAML.DECODE", [][]byte{[]byte("invalid")}},
		{"CSV.ENCODE no args", "CSV.ENCODE", nil},
		{"CSV.ENCODE data", "CSV.ENCODE", [][]byte{[]byte("col1,col2\nval1,val2")}},
		{"CSV.DECODE no args", "CSV.DECODE", nil},
		{"CSV.DECODE data", "CSV.DECODE", [][]byte{[]byte("col1,col2\nval1,val2")}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			runCommandTest(t, router, s, tt.cmd, tt.args)
		})
	}
}

func TestUtilityExtCommandsExtensive(t *testing.T) {
	s := store.NewStore()
	router := NewRouter()
	RegisterUtilityExtCommands(router)

	tests := []struct {
		name string
		cmd  string
		args [][]byte
	}{
		{"UUID.GENERATE", "UUID.GENERATE", nil},
		{"UUID.PARSE no args", "UUID.PARSE", nil},
		{"UUID.PARSE invalid", "UUID.PARSE", [][]byte{[]byte("invalid")}},
		{"TIME.NOW", "TIME.NOW", nil},
		{"TIME.FORMAT no args", "TIME.FORMAT", nil},
		{"TIME.FORMAT timestamp", "TIME.FORMAT", [][]byte{[]byte("1609459200")}},
		{"TIME.PARSE no args", "TIME.PARSE", nil},
		{"TIME.PARSE invalid", "TIME.PARSE", [][]byte{[]byte("invalid")}},
		{"RANDOM.STRING no args", "RANDOM.STRING", nil},
		{"RANDOM.STRING length", "RANDOM.STRING", [][]byte{[]byte("10")}},
		{"RANDOM.NUMBER no args", "RANDOM.NUMBER", nil},
		{"RANDOM.NUMBER range", "RANDOM.NUMBER", [][]byte{[]byte("1"), []byte("100")}},
		{"RANDOM.UUID", "RANDOM.UUID", nil},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			runCommandTest(t, router, s, tt.cmd, tt.args)
		})
	}
}
