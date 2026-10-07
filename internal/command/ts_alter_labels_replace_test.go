package command

import (
	"bytes"
	"reflect"
	"testing"

	"github.com/cachestorm/cachestorm/internal/resp"
	"github.com/cachestorm/cachestorm/internal/store"
)

func TestTSAlterLabelsReplaceX(t *testing.T) {
	previousManager := tsManager
	tsManager = store.NewTimeSeriesManager()
	t.Cleanup(func() { tsManager = previousManager })
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
	field := func(t *testing.T, info *resp.Value, name string) *resp.Value {
		t.Helper()
		if info.Type != resp.TypeArray || len(info.Array)%2 != 0 {
			t.Fatalf("malformed TS.INFO: %v", info)
		}
		for i := 0; i < len(info.Array); i += 2 {
			if info.Array[i].Type == resp.TypeBulkString && string(info.Array[i].Bulk) == name {
				return info.Array[i+1]
			}
		}
		t.Fatalf("TS.INFO missing %s", name)
		return nil
	}
	labels := func(t *testing.T, info *resp.Value) map[string]string {
		t.Helper()
		value := field(t, info, "labels")
		if value.Type != resp.TypeArray {
			t.Fatalf("malformed labels: %v", value)
		}
		result := make(map[string]string)
		for _, pair := range value.Array {
			if pair.Type != resp.TypeArray || len(pair.Array) != 2 || pair.Array[0].Type != resp.TypeBulkString || pair.Array[1].Type != resp.TypeBulkString {
				t.Fatalf("malformed label pair: %v", pair)
			}
			result[string(pair.Array[0].Bulk)] = string(pair.Array[1].Bulk)
		}
		return result
	}
	originalLabels := map[string]string{"keep": "old", "obsolete": "old"}
	for _, tc := range []struct {
		name       string
		args       []string
		want       map[string]string
		retention  int64
		validInput bool
	}{
		{"replace subset", []string{"LABELS", "keep", "new"}, map[string]string{"keep": "new"}, 1000, true},
		{"replace with disjoint labels", []string{"LABELS", "newtag", "new"}, map[string]string{"newtag": "new"}, 1000, true},
		{"empty LABELS clears all", []string{"LABELS"}, map[string]string{}, 1000, true},
		{"empty label value", []string{"LABELS", "keep", ""}, map[string]string{"keep": ""}, 1000, true},
		{"replace with retention update", []string{"RETENTION", "2000", "LABELS", "keep", "new"}, map[string]string{"keep": "new"}, 2000, true},
		{"replace with retention disabled", []string{"RETENTION", "0", "LABELS", "keep", "new"}, map[string]string{"keep": "new"}, 0, true},
		{"omitted LABELS control", nil, originalLabels, 1000, true},
		{"retention-only control", []string{"RETENTION", "2000"}, originalLabels, 2000, true},
		{"identical labels control", []string{"LABELS", "keep", "old", "obsolete", "old"}, originalLabels, 1000, true},
		{"missing label value must not clear labels", []string{"LABELS", "orphan"}, originalLabels, 1000, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			key := t.Name()
			checkOK(t, run(t, "TS.CREATE", key, "RETENTION", "1000", "LABELS", "keep", "old", "obsolete", "old"))
			if got := labels(t, run(t, "TS.INFO", key)); !reflect.DeepEqual(got, originalLabels) {
				t.Fatalf("seed labels mismatch: %v", got)
			}
			reply := run(t, "TS.ALTER", append([]string{key}, tc.args...)...)
			if tc.validInput {
				checkOK(t, reply)
			}
			info := run(t, "TS.INFO", key)
			got := labels(t, info)
			if !reflect.DeepEqual(got, tc.want) {
				t.Errorf("FAIL: TS.ALTER reply=%v, labels=%v, want %v", reply, got, tc.want)
			} else {
				t.Logf("PASS: labels=%v", got)
			}
			retention := field(t, info, "retentionTime")
			if retention.Type != resp.TypeInteger || retention.Int != tc.retention {
				t.Errorf("retention changed unexpectedly: %v, want %d", retention, tc.retention)
			}
			matches := run(t, "TS.QUERYINDEX", "obsolete", "old")
			if matches.Type != resp.TypeArray {
				t.Fatalf("malformed query reply: %v", matches)
			}
			matched := false
			for _, match := range matches.Array {
				if match.Type != resp.TypeBulkString {
					t.Fatalf("malformed query key: %v", match)
				}
				if string(match.Bulk) == key {
					matched = true
				}
			}
			_, wantMatch := tc.want["obsolete"]
			if matched != wantMatch {
				t.Errorf("FAIL: obsolete-label query membership=%t, want %t", matched, wantMatch)
			}
		})
	}
}
