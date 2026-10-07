package command

import (
	"bytes"
	"fmt"
	"reflect"
	"testing"

	"github.com/cachestorm/cachestorm/internal/resp"
	"github.com/cachestorm/cachestorm/internal/store"
)

func TestTSMAddDefaultBlockX(t *testing.T) {
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
	checkInts := func(t *testing.T, label string, value *resp.Value, want []int64) {
		t.Helper()
		if value.Type != resp.TypeArray {
			t.Fatalf("%s: expected integer array, got %v", label, value)
		}
		got := make([]int64, 0, len(value.Array))
		for _, item := range value.Array {
			if item.Type != resp.TypeInteger {
				t.Fatalf("%s: expected integer element, got %v", label, item)
			}
			got = append(got, item.Int)
		}
		if !reflect.DeepEqual(got, want) {
			t.Errorf("FAIL %s: got %v, want %v", label, got, want)
		} else {
			t.Logf("PASS %s: %v", label, got)
		}
	}
	checkInts(t, "multi-series control", run(t, "TS.MADD", "mA", "1000", "5", "mB", "1000", "6"), []int64{1000, 1000})
	value := run(t, "TS.MADD", "mA", "1000", "7")
	if value.Type != resp.TypeError || value.Err != "TSDB: duplicate timestamp" {
		t.Errorf("FAIL duplicate: expected error reply, got %v", value)
	}
	samples := run(t, "TS.RANGE", "mA", "0", "2000")
	if samples.Type != resp.TypeArray || len(samples.Array) != 1 {
		t.Fatalf("FAIL preserved: expected exactly one sample, got %v", samples)
	}
	sample := samples.Array[0]
	if sample.Type != resp.TypeArray || len(sample.Array) != 2 || fmt.Sprint(sample.Array[0].Int) != "1000" || string(sample.Array[1].Bulk) != "5" {
		t.Fatalf("FAIL preserved: wrong sample %v", sample)
	}
	checkInts(t, "post-rejection add control", run(t, "TS.MADD", "mA", "2000", "9"), []int64{2000})
	mSamples := run(t, "TS.RANGE", "mB", "0", "2000")
	if mSamples.Type != resp.TypeArray || len(mSamples.Array) != 1 {
		t.Fatalf("FAIL second series: expected exactly one sample, got %v", mSamples)
	}
	if mSamples.Array[0].Type != resp.TypeArray || len(mSamples.Array[0].Array) != 2 || fmt.Sprint(mSamples.Array[0].Array[0].Int) != "1000" || string(mSamples.Array[0].Array[1].Bulk) != "6" {
		t.Fatalf("FAIL second series: wrong sample %v", mSamples.Array[0])
	}
}
