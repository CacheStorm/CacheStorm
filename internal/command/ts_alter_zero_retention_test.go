package command

import (
	"bytes"
	"fmt"
	"testing"

	"github.com/cachestorm/cachestorm/internal/resp"
)

func TestTSAlterZeroRetentionX(t *testing.T) {
	router := NewRouter()
	RegisterTSCommands(router)
	run := func(t *testing.T, name string, args ...string) *resp.Value {
		t.Helper()
		raw := make([][]byte, len(args))
		for i, arg := range args {
			raw[i] = []byte(arg)
		}
		var output bytes.Buffer
		if err := router.Execute(NewContext(name, raw, nil, resp.NewWriter(&output))); err != nil {
			t.Fatal(err)
		}
		value, err := resp.NewReader(&output).ReadValue()
		if err != nil {
			t.Fatal(err)
		}
		return value
	}
	checkOK := func(t *testing.T, value *resp.Value) {
		t.Helper()
		if value.Type != resp.TypeSimpleString || value.Str != "OK" {
			t.Fatalf("expected OK, got %v", value)
		}
	}
	field := func(t *testing.T, value *resp.Value, name string) *resp.Value {
		t.Helper()
		if value.Type != resp.TypeArray || len(value.Array)%2 != 0 {
			t.Fatalf("malformed TS.INFO: %v", value)
		}
		for i := 0; i < len(value.Array); i += 2 {
			if value.Array[i].Type == resp.TypeBulkString && string(value.Array[i].Bulk) == name {
				return value.Array[i+1]
			}
		}
		t.Fatalf("TS.INFO missing %s", name)
		return nil
	}
	for _, tc := range []struct {
		name       string
		initial    int64
		args       []string
		want       int64
		wantLabel  string
		validInput bool
	}{
		{"positive update control", 1000, []string{"RETENTION", "2000"}, 2000, "original", true},
		{"omitted retention control", 1000, nil, 1000, "original", true},
		{"labels-only control", 1000, []string{"LABELS", "brand", "changed"}, 1000, "changed", true},
		{"zero disables retention", 1000, []string{"RETENTION", "0"}, 0, "original", true},
		{"signed zero disables retention", 1000, []string{"RETENTION", "+0"}, 0, "original", true},
		{"leading zeros disable retention", 1000, []string{"RETENTION", "000"}, 0, "original", true},
		{"negative zero disables retention", 1000, []string{"RETENTION", "-0"}, 0, "original", true},
		{"zero with label update", 1000, []string{"RETENTION", "0", "LABELS", "brand", "changed"}, 0, "changed", true},
		{"already disabled control", 0, []string{"RETENTION", "0"}, 0, "original", true},
		{"re-enable retention control", 0, []string{"RETENTION", "2000"}, 2000, "original", true},
		{"malformed value must not clear retention", 1000, []string{"RETENTION", "bogus"}, 1000, "original", false},
		{"out-of-range value must not clear retention", 1000, []string{"RETENTION", "9223372036854775808"}, 1000, "original", false},
		{"negative value must not clear retention", 1000, []string{"RETENTION", "-1"}, 1000, "original", false},
		{"missing value must not clear retention", 1000, []string{"RETENTION"}, 1000, "original", false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			key := t.Name()
			t.Cleanup(func() { tsManager.Delete(key) })
			checkOK(t, run(t, "TS.CREATE", key, "RETENTION", fmt.Sprint(tc.initial), "LABELS", "brand", "original"))
			before := field(t, run(t, "TS.INFO", key), "retentionTime")
			if before.Type != resp.TypeInteger || before.Int != tc.initial {
				t.Fatalf("seed retention mismatch: %v", before)
			}
			reply := run(t, "TS.ALTER", append([]string{key}, tc.args...)...)
			if tc.validInput {
				checkOK(t, reply)
			}
			info := run(t, "TS.INFO", key)
			retention := field(t, info, "retentionTime")
			if retention.Type != resp.TypeInteger || retention.Int != tc.want {
				t.Errorf("FAIL: TS.ALTER reply=%v, retentionTime=%v, want %d", reply, retention, tc.want)
			} else {
				t.Logf("PASS: retentionTime=%d", retention.Int)
			}
			labels := field(t, info, "labels")
			if labels.Type != resp.TypeArray || len(labels.Array) != 1 || labels.Array[0].Type != resp.TypeArray || len(labels.Array[0].Array) != 2 || string(labels.Array[0].Array[0].Bulk) != "brand" || string(labels.Array[0].Array[1].Bulk) != tc.wantLabel {
				t.Errorf("labels changed unexpectedly: %v", labels)
			}
		})
	}
}
