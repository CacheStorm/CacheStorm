package command

import (
	"bytes"
	"testing"

	"github.com/cachestorm/cachestorm/internal/resp"
	"github.com/cachestorm/cachestorm/internal/store"
)

func TestSlowlogGetAllX(t *testing.T) {
	previous := globalSlowLog
	globalSlowLog = &SlowLog{
		entries:   make([]SlowLogEntry, 0),
		maxLen:    100,
		slowLogSl: 0,
	}
	t.Cleanup(func() { globalSlowLog = previous })

	router := NewRouter()
	RegisterServerCommands(router)
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
	count := func(name string, args ...string) int {
		t.Helper()
		v := run(name, args...)
		if v.Type != resp.TypeArray {
			t.Fatalf("%s %v: expected array, got %v", name, args, v)
		}
		return len(v.Array)
	}

	for i := 0; i < 12; i++ {
		globalSlowLog.Add("GET", []string{"k"}, 1, "127.0.0.1:0", 1)
	}

	if v := run("SLOWLOG", "LEN"); v.Type != resp.TypeInteger || v.Int != 12 {
		t.Fatalf("FAIL LEN control: got %v, want 12", v)
	}
	t.Log("PASS LEN control: 12")

	if got := count("SLOWLOG", "GET", "-1"); got != 12 {
		t.Errorf("FAIL GET -1 returns all: got %d entries, want 12", got)
	} else {
		t.Log("PASS GET -1 returns all: 12")
	}

	if got := count("SLOWLOG", "GET"); got != 10 {
		t.Errorf("FAIL omitted-count default: got %d entries, want 10", got)
	} else {
		t.Log("PASS omitted-count default: 10")
	}

	if got := count("SLOWLOG", "GET", "5"); got != 5 {
		t.Errorf("FAIL GET 5: got %d entries, want 5", got)
	} else {
		t.Log("PASS GET 5: 5")
	}

	if got := count("SLOWLOG", "GET", "100"); got != 12 {
		t.Errorf("FAIL GET above total: got %d entries, want 12", got)
	} else {
		t.Log("PASS GET above total: 12")
	}

	if got := count("SLOWLOG", "GET", "0"); got != 0 {
		t.Errorf("FAIL GET 0: got %d entries, want 0 (at most count entries)", got)
	} else {
		t.Log("PASS GET 0: empty")
	}

	newest := run("SLOWLOG", "GET", "1")
	if newest.Type != resp.TypeArray || len(newest.Array) != 1 ||
		newest.Array[0].Array[0].Type != resp.TypeInteger || newest.Array[0].Array[0].Int != 11 {
		t.Errorf("FAIL newest-first control: got %v, want latest id 11 first", newest)
	} else {
		t.Log("PASS newest-first control: id 11 first")
	}

	if v := run("SLOWLOG", "RESET"); v.Type != resp.TypeSimpleString || v.Str != "OK" {
		t.Fatalf("FAIL RESET control: got %v", v)
	}
	if got := count("SLOWLOG", "GET"); got != 0 {
		t.Errorf("FAIL post-RESET control: got %d entries, want 0", got)
	} else {
		t.Log("PASS post-RESET control: empty")
	}

	_ = store.NewStore
}
