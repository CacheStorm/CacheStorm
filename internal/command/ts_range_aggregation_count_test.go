package command

import (
	"bytes"
	"fmt"
	"reflect"
	"testing"

	"github.com/cachestorm/cachestorm/internal/resp"
)

func TestTSRangeAggregationCountX(t *testing.T) {
	router := NewRouter()
	RegisterTSCommands(router)
	key := t.Name()
	t.Cleanup(func() { tsManager.Delete(key) })
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
	for _, sample := range [][2]string{{"1200", "5"}, {"1000", "8"}, {"2100", "10"}, {"1100", "2"}, {"3000", "6"}, {"2000", "4"}} {
		value := run(t, "TS.ADD", key, sample[0], sample[1])
		if value.Type != resp.TypeInteger || fmt.Sprint(value.Int) != sample[0] {
			t.Fatalf("seed failed: %v", value)
		}
	}
	allBuckets := [][2]string{{"1000", "15"}, {"2000", "14"}, {"3000", "6"}}
	for _, tc := range []struct {
		name string
		args []string
		want [][2]string
	}{
		{"raw COUNT control", []string{"1000", "3000", "COUNT", "1"}, [][2]string{{"1000", "8"}}},
		{"unlimited aggregation control", []string{"1000", "3000", "AGGREGATION", "SUM", "1000"}, allBuckets},
		{"COUNT one bucket before aggregation", []string{"1000", "3000", "COUNT", "1", "AGGREGATION", "SUM", "1000"}, allBuckets[:1]},
		{"COUNT one bucket after aggregation", []string{"1000", "3000", "AGGREGATION", "SUM", "1000", "COUNT", "1"}, allBuckets[:1]},
		{"COUNT two buckets", []string{"1000", "3000", "COUNT", "2", "AGGREGATION", "SUM", "1000"}, allBuckets[:2]},
		{"exact bucket count", []string{"1000", "3000", "COUNT", "3", "AGGREGATION", "SUM", "1000"}, allBuckets},
		{"count above bucket count", []string{"1000", "3000", "COUNT", "4", "AGGREGATION", "SUM", "1000"}, allBuckets},
		{"maximum count", []string{"1000", "3000", "COUNT", "9223372036854775807", "AGGREGATION", "SUM", "1000"}, allBuckets},
		{"AVG computes whole first bucket", []string{"1000", "3000", "COUNT", "1", "AGGREGATION", "AVG", "1000"}, [][2]string{{"1000", "5"}}},
		{"COUNT aggregate and COUNT limit", []string{"1000", "3000", "COUNT", "1", "AGGREGATION", "COUNT", "1000"}, [][2]string{{"1000", "3"}}},
		{"empty range", []string{"4000", "5000", "COUNT", "1", "AGGREGATION", "SUM", "1000"}, [][2]string{}},
		{"single bucket", []string{"1000", "1200", "COUNT", "2", "AGGREGATION", "SUM", "1000"}, allBuckets[:1]},
	} {
		t.Run(tc.name, func(t *testing.T) {
			check(t, run(t, "TS.RANGE", append([]string{key}, tc.args...)...), tc.want)
		})
	}
	check(t, run(t, "TS.RANGE", key, "1000", "3000"), [][2]string{{"1000", "8"}, {"1100", "2"}, {"1200", "5"}, {"2000", "4"}, {"2100", "10"}, {"3000", "6"}})
}
