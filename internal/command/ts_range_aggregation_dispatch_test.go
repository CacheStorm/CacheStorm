package command

import (
	"bytes"
	"fmt"
	"reflect"
	"strings"
	"testing"

	"github.com/cachestorm/cachestorm/internal/resp"
)

func TestTSRangeAggregationDispatchX(t *testing.T) {
	router := NewRouter()
	RegisterTSCommands(router)
	key := t.Name()
	t.Cleanup(func() { tsManager.Delete(key) })
	run := func(name string, args ...string) *resp.Value {
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
	check := func(t *testing.T, label string, value *resp.Value, want [][2]string) {
		t.Helper()
		if value.Type != resp.TypeArray {
			t.Fatalf("%s: expected array, got %v", label, value)
		}
		got := make([][2]string, 0, len(value.Array))
		for _, sample := range value.Array {
			if sample.Type != resp.TypeArray || len(sample.Array) != 2 || sample.Array[0].Type != resp.TypeInteger || sample.Array[1].Type != resp.TypeBulkString {
				t.Fatalf("%s: malformed sample %v", label, sample)
			}
			got = append(got, [2]string{fmt.Sprint(sample.Array[0].Int), string(sample.Array[1].Bulk)})
		}
		if !reflect.DeepEqual(got, want) {
			t.Errorf("FAIL %s: got %v, want %v", label, got, want)
		} else {
			t.Logf("PASS %s: %v", label, got)
		}
	}
	for _, sample := range [][2]string{{"1200", "5"}, {"1000", "8"}, {"2100", "10"}, {"1100", "2"}, {"2000", "4"}} {
		value := run("TS.ADD", key, sample[0], sample[1])
		if value.Type != resp.TypeInteger || fmt.Sprint(value.Int) != sample[0] {
			t.Fatalf("seed failed: %v", value)
		}
	}
	check(t, "raw range control", run("TS.RANGE", key, "1000", "2100"), [][2]string{{"1000", "8"}, {"1100", "2"}, {"1200", "5"}, {"2000", "4"}, {"2100", "10"}})
	check(t, "COUNT control", run("TS.RANGE", key, "1000", "2100", "COUNT", "1"), [][2]string{{"1000", "8"}})
	for _, tc := range []struct {
		name string
		want [2]string
	}{
		{"sum", [2]string{"15", "14"}},
		{"avg", [2]string{"5", "7"}},
		{"min", [2]string{"2", "4"}},
		{"max", [2]string{"8", "10"}},
		{"count", [2]string{"3", "2"}},
		{"first", [2]string{"8", "4"}},
		{"last", [2]string{"5", "10"}},
	} {
		for _, spelling := range []string{tc.name, strings.ToUpper(tc.name), strings.ToUpper(tc.name[:1]) + tc.name[1:]} {
			t.Run(spelling, func(t *testing.T) {
				check(t, spelling, run("TS.RANGE", key, "1000", "2100", "AGGREGATION", spelling, "1000"), [][2]string{{"1000", tc.want[0]}, {"2000", tc.want[1]}})
			})
		}
	}
	check(t, "inclusive range boundary", run("TS.RANGE", key, "1200", "2000", "AGGREGATION", "SUM", "1000"), [][2]string{{"1000", "5"}, {"2000", "4"}})
	check(t, "empty range", run("TS.RANGE", key, "3000", "4000", "AGGREGATION", "SUM", "1000"), [][2]string{})
	check(t, "read leaves samples unchanged", run("TS.RANGE", key, "1000", "2100"), [][2]string{{"1000", "8"}, {"1100", "2"}, {"1200", "5"}, {"2000", "4"}, {"2100", "10"}})
}
