package command

import (
	"bytes"
	"fmt"
	"testing"

	"github.com/cachestorm/cachestorm/internal/resp"
	"github.com/cachestorm/cachestorm/internal/store"
)

func TestTSIncrbyDecrbyTimestampX(t *testing.T) {
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
	checkError := func(t *testing.T, label string, value *resp.Value, want string) {
		t.Helper()
		if value.Type != resp.TypeError || value.Err != want {
			t.Errorf("FAIL %s: expected error %q, got %v", label, want, value)
		} else {
			t.Logf("PASS %s: %v", label, value)
		}
	}
	checkSample := func(t *testing.T, label string, key, ts, value string) {
		t.Helper()
		samples := run(t, "TS.RANGE", key, "0", "9223372036854775807")
		if samples.Type != resp.TypeArray || len(samples.Array) != 1 {
			t.Errorf("FAIL %s: expected exactly one sample, got %v", label, samples)
			return
		}
		s := samples.Array[0]
		if s.Type != resp.TypeArray || len(s.Array) != 2 || fmt.Sprint(s.Array[0].Int) != ts || string(s.Array[1].Bulk) != value {
			t.Errorf("FAIL %s: wrong sample %v, want [%s %s]", label, s, ts, value)
		} else {
			t.Logf("PASS %s: [%s %s]", label, ts, value)
		}
	}
	now := run(t, "TS.INCRBY", "k", "5")
	if now.Type != resp.TypeInteger || now.Int <= 1600000000000 {
		t.Errorf("FAIL now control: expected now-ms integer, got %v", now)
	}
	incr := run(t, "TS.INCRBY", "ki", "5", "TIMESTAMP", "1000")
	if incr.Type != resp.TypeInteger || incr.Int != 1000 {
		t.Errorf("FAIL incrby new: expected 1000, got %v", incr)
	}
	checkSample(t, "incrby new", "ki", "1000", "5")
	incr = run(t, "TS.INCRBY", "ki", "7", "TIMESTAMP", "1000")
	if incr.Type != resp.TypeInteger || incr.Int != 1000 {
		t.Errorf("FAIL incrby accumulate: expected 1000, got %v", incr)
	}
	checkSample(t, "incrby accumulate", "ki", "1000", "12")
	checkError(t, "incrby below max", run(t, "TS.INCRBY", "ki", "3", "TIMESTAMP", "999"), "TSDB: timestamp must be equal to or higher than the maximum existing timestamp")
	checkSample(t, "incrby below max preserved", "ki", "1000", "12")
	empty := run(t, "TS.INCRBY", "k2", "9", "TIMESTAMP", "500")
	if empty.Type != resp.TypeInteger || empty.Int != 500 {
		t.Errorf("FAIL incrby empty: expected 500, got %v", empty)
	}
	checkSample(t, "incrby empty", "k2", "500", "9")
	decr := run(t, "TS.DECRBY", "d", "4", "TIMESTAMP", "2000")
	if decr.Type != resp.TypeInteger || decr.Int != 2000 {
		t.Errorf("FAIL decrby new: expected 2000, got %v", decr)
	}
	decr = run(t, "TS.DECRBY", "d", "1", "TIMESTAMP", "2000")
	if decr.Type != resp.TypeInteger || decr.Int != 2000 {
		t.Errorf("FAIL decrby accumulate: expected 2000, got %v", decr)
	}
	checkSample(t, "decrby accumulate", "d", "2000", "-5")
	checkError(t, "decrby below max", run(t, "TS.DECRBY", "d", "3", "TIMESTAMP", "1999"), "TSDB: timestamp must be equal to or higher than the maximum existing timestamp")
	checkSample(t, "decrby below max preserved", "d", "2000", "-5")
}
