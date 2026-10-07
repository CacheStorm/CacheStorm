package command

import (
	"bytes"
	"fmt"
	"reflect"
	"testing"

	"github.com/cachestorm/cachestorm/internal/resp"
	"github.com/cachestorm/cachestorm/internal/store"
)

func TestTSRevRangeOptionsX(t *testing.T) {
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
	check := func(t *testing.T, value *resp.Value, want [][2]string) {
		t.Helper()
		if value.Type != resp.TypeArray {
			t.Fatalf("expected array, got %v", value)
		}
		got := make([][2]string, 0, len(value.Array))
		for _, sample := range value.Array {
			if sample.Type != resp.TypeArray || len(sample.Array) != 2 || sample.Array[0].Type != resp.TypeInteger || sample.Array[1].Type != resp.TypeBulkString {
				t.Fatalf("malformed sample %v", sample)
			}
			got = append(got, [2]string{fmt.Sprint(sample.Array[0].Int), string(sample.Array[1].Bulk)})
		}
		if !reflect.DeepEqual(got, want) {
			t.Errorf("FAIL: got %v, want %v", got, want)
		} else {
			t.Logf("PASS: %v", got)
		}
	}
	for _, sample := range [][2]string{{"1200", "5"}, {"1000", "8"}, {"2100", "10"}, {"1100", "2"}, {"2000", "4"}} {
		value := run(t, "TS.ADD", "rev", sample[0], sample[1])
		if value.Type != resp.TypeInteger || fmt.Sprint(value.Int) != sample[0] {
			t.Fatalf("seed failed: %v", value)
		}
	}
	allReversed := [][2]string{{"2100", "10"}, {"2000", "4"}, {"1200", "5"}, {"1100", "2"}, {"1000", "8"}}
	for _, tc := range []struct {
		name string
		args []string
		want [][2]string
	}{
		{"plain reverse control", []string{"1000", "2100"}, allReversed},
		{"COUNT limits raw samples", []string{"1000", "2100", "COUNT", "2"}, allReversed[:2]},
		{"COUNT 1 newest sample", []string{"1000", "2100", "COUNT", "1"}, allReversed[:1]},
		{"lowercase count keyword", []string{"1000", "2100", "count", "1"}, allReversed[:1]},
		{"COUNT exceeds total", []string{"1000", "2100", "COUNT", "9"}, allReversed},
		{"AGGREGATION sum buckets", []string{"1000", "2100", "AGGREGATION", "sum", "1000"}, [][2]string{{"2000", "14"}, {"1000", "15"}}},
		{"AGGREGATION avg buckets", []string{"1000", "2100", "AGGREGATION", "avg", "1000"}, [][2]string{{"2000", "7"}, {"1000", "5"}}},
		{"AGGREGATION with COUNT limits buckets", []string{"1000", "2100", "AGGREGATION", "sum", "1000", "COUNT", "1"}, [][2]string{{"2000", "14"}}},
		{"COUNT before AGGREGATION order", []string{"1000", "2100", "COUNT", "1", "AGGREGATION", "sum", "1000"}, [][2]string{{"2000", "14"}}},
		{"missing key", []string{"1000", "2100", "COUNT", "1"}, [][2]string{}},
		{"empty range", []string{"3000", "4000", "AGGREGATION", "sum", "1000", "COUNT", "1"}, [][2]string{}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			args := append([]string{"rev"}, tc.args...)
			if tc.name == "missing key" {
				args = append([]string{"absent"}, tc.args...)
			}
			check(t, run(t, "TS.REVRANGE", args...), tc.want)
		})
	}
	check(t, run(t, "TS.RANGE", "rev", "1000", "2100"), [][2]string{{"1000", "8"}, {"1100", "2"}, {"1200", "5"}, {"2000", "4"}, {"2100", "10"}})
}
