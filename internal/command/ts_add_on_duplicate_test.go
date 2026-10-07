package command

import (
	"bytes"
	"testing"

	"github.com/cachestorm/cachestorm/internal/resp"
	"github.com/cachestorm/cachestorm/internal/store"
)

func TestTSAddOnDuplicateX(t *testing.T) {
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
	valueAt := func(t *testing.T, key string, sample [2]string) string {
		t.Helper()
		samples := run(t, "TS.RANGE", key, sample[0], sample[0])
		if samples.Type != resp.TypeArray || len(samples.Array) != 1 || samples.Array[0].Type != resp.TypeArray || len(samples.Array[0].Array) != 2 {
			t.Fatalf("expected exactly one sample at %s, got %v", sample[0], samples)
		}
		if samples.Array[0].Array[0].Type != resp.TypeInteger {
			t.Fatalf("expected integer timestamp, got %v", samples.Array[0].Array[0])
		}
		return string(samples.Array[0].Array[1].Bulk)
	}
	for _, tc := range []struct {
		name      string
		created   string
		onDup     string
		added     string
		wantValue string
	}{
		{"default BLOCK rejects duplicate", "40", "", "41", "40"},
		{"last override", "40", "LAST", "41", "41"},
		{"first ignore", "40", "FIRST", "40", "40"},
		{"block error reply", "40", "BLOCK", "40", "40"},
		{"min keeps smaller", "40", "MIN", "39", "39"},
		{"max keeps larger", "40", "MAX", "41", "41"},
		{"sum accumulates", "40", "SUM", "41", "81"},
		{"lowercase LAST", "40", "last", "41", "41"},
		{"mixed-case Max", "40", "Max", "41", "41"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			key := t.Name()
			checkInteger(t, run(t, "TS.ADD", key, "1000", tc.created), 1000)
			if tc.onDup == "" {
				reply := run(t, "TS.ADD", key, "1000", tc.added)
				if reply.Type != resp.TypeError {
					t.Fatalf("FAIL: default policy must reject duplicates, got %v", reply)
				}
				samples := run(t, "TS.RANGE", key, "1000", "1000")
				if samples.Type != resp.TypeArray || len(samples.Array) != 1 || samples.Array[0].Type != resp.TypeArray || len(samples.Array[0].Array) != 2 || samples.Array[0].Array[1].Type != resp.TypeBulkString || string(samples.Array[0].Array[1].Bulk) != tc.wantValue {
					t.Fatalf("FAIL: default policy left wrong samples: %v", samples)
				}
				t.Logf("PASS: default BLOCK rejected duplicate, value preserved: %s", string(samples.Array[0].Array[1].Bulk))
				return
			}
			reply := run(t, "TS.ADD", key, "1000", tc.added, "ON_DUPLICATE", tc.onDup)
			if tc.onDup == "BLOCK" {
				if reply.Type != resp.TypeError {
					t.Fatalf("FAIL: expected error reply, got %v", reply)
				}
				t.Logf("PASS: duplicate rejected: %v", reply)
			} else {
				checkInteger(t, reply, 1000)
			}
			got := valueAt(t, key, [2]string{"1000", tc.wantValue})
			if got != tc.wantValue {
				t.Errorf("FAIL: value=%s, want %s", got, tc.wantValue)
			} else {
				t.Logf("PASS: value=%s", got)
			}
		})
	}
	checkInteger(t, run(t, "TS.ADD", "unique", "2000", "50"), 2000)
	if got := valueAt(t, "unique", [2]string{"2000", "50"}); got != "50" {
		t.Errorf("unique non-duplicate control failed: %s", got)
	}
}
