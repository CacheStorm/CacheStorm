package command

import (
	"bytes"
	"fmt"
	"reflect"
	"testing"

	"github.com/cachestorm/cachestorm/internal/resp"
	"github.com/cachestorm/cachestorm/internal/store"
)

func TestTSAddDefaultBlockX(t *testing.T) {
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
	checkInteger := func(t *testing.T, value *resp.Value, want int64) {
		t.Helper()
		if value.Type != resp.TypeInteger || value.Int != want {
			t.Fatalf("expected integer %d, got %v", want, value)
		}
	}
	checkError := func(t *testing.T, value *resp.Value) {
		t.Helper()
		if value.Type != resp.TypeError {
			t.Fatalf("FAIL: expected error reply, got %v", value)
		}
		t.Logf("PASS: duplicate rejected: %v", value)
	}
	checkSamples := func(t *testing.T, value *resp.Value, want [][2]string) {
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
			t.Logf("PASS: samples %v", got)
		}
	}
	checkInteger(t, run(t, "TS.ADD", "blk", "1000", "40"), 1000)
	checkError(t, run(t, "TS.ADD", "blk", "1000", "41"))
	checkSamples(t, run(t, "TS.RANGE", "blk", "1000", "1000"), [][2]string{{"1000", "40"}})
	checkError(t, run(t, "TS.ADD", "blk", "1000", "42"))
	checkSamples(t, run(t, "TS.RANGE", "blk", "1000", "1000"), [][2]string{{"1000", "40"}})
	checkInteger(t, run(t, "TS.ADD", "blk", "2000", "30"), 2000)
	checkInteger(t, run(t, "TS.ADD", "blk", "1000", "41", "ON_DUPLICATE", "LAST"), 1000)
	checkSamples(t, run(t, "TS.RANGE", "blk", "1000", "2000"), [][2]string{{"1000", "41"}, {"2000", "30"}})
	checkError(t, run(t, "TS.ADD", "blk", "2000", "31", "ON_DUPLICATE", "BLOCK"))
	checkSamples(t, run(t, "TS.RANGE", "blk", "1000", "2000"), [][2]string{{"1000", "41"}, {"2000", "30"}})
	checkInteger(t, run(t, "TS.ADD", "fresh", "500", "7"), 500)
}
