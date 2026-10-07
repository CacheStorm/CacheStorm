package command

import (
	"bytes"
	"testing"
	"time"

	"github.com/cachestorm/cachestorm/internal/resp"
	"github.com/cachestorm/cachestorm/internal/store"
)

func TestXReadBlockZeroX(t *testing.T) {
	s := store.NewStore()
	router := NewRouter()
	RegisterStreamCommands(router)
	run := func(name string, args ...string) *resp.Value {
		t.Helper()
		raw := make([][]byte, len(args))
		for i, arg := range args {
			raw[i] = []byte(arg)
		}
		var output bytes.Buffer
		if err := router.Execute(NewContext(name, raw, s, resp.NewWriter(&output))); err != nil {
			t.Fatal(err)
		}
		value, err := resp.NewReader(&output).ReadValue()
		if err != nil {
			t.Fatal(err)
		}
		return value
	}
	if run("XADD", "s", "1-0", "f", "old").Type != resp.TypeBulkString {
		t.Fatalf("seed failed")
	}
	value := run("XREAD", "BLOCK", "0", "STREAMS", "s", "0")
	if value.Type != resp.TypeArray || len(value.Array) != 1 ||
		string(value.Array[0].Array[0].Bulk) != "s" ||
		len(value.Array[0].Array[1].Array) != 1 ||
		string(value.Array[0].Array[1].Array[0].Array[0].Bulk) != "1-0" {
		t.Errorf("FAIL sync control: expected existing entry with BLOCK 0, got %v", value)
	} else {
		t.Log("PASS sync control: data served immediately under BLOCK 0")
	}
	started := time.Now()
	value = run("XREAD", "BLOCK", "100", "STREAMS", "s", "$")
	if value.Type != resp.TypeNull {
		t.Errorf("FAIL timeout control: expected null on timeout, got %v", value)
	}
	if elapsed := time.Since(started); elapsed > 5*time.Second {
		t.Errorf("FAIL timeout control: elapsed %v", elapsed)
	} else {
		t.Logf("PASS timeout control: null after %v", elapsed)
	}
	type blockResult struct {
		value *resp.Value
		err   error
	}
	done := make(chan blockResult, 1)
	go func() {
		var output bytes.Buffer
		err := router.Execute(NewContext("XREAD", [][]byte{[]byte("BLOCK"), []byte("0"), []byte("STREAMS"), []byte("s"), []byte("$")}, s, resp.NewWriter(&output)))
		var value *resp.Value
		if err == nil {
			value, err = resp.NewReader(&output).ReadValue()
		}
		done <- blockResult{value, err}
	}()
	select {
	case res := <-done:
		if res.err != nil {
			t.Errorf("FAIL block-zero: returned error %v instead of blocking", res.err)
		} else {
			t.Errorf("FAIL block-zero: returned %v immediately instead of blocking", res.value)
		}
	case <-time.After(200 * time.Millisecond):
		t.Log("PASS block-zero: still blocked after 200ms with no data")
	}
	if run("XADD", "s", "2-0", "f", "new").Type != resp.TypeBulkString {
		t.Fatalf("wake seed failed")
	}
	select {
	case res := <-done:
		if res.err != nil {
			t.Fatalf("FAIL wake: %v", res.err)
		}
		v := res.value
		if v.Type != resp.TypeArray || len(v.Array) != 1 ||
			string(v.Array[0].Array[0].Bulk) != "s" ||
			len(v.Array[0].Array[1].Array) != 1 ||
			string(v.Array[0].Array[1].Array[0].Array[0].Bulk) != "2-0" ||
			string(v.Array[0].Array[1].Array[0].Array[1].Array[0].Bulk) != "f" ||
			string(v.Array[0].Array[1].Array[0].Array[1].Array[1].Bulk) != "new" {
			t.Errorf("FAIL wake: expected [[s [[2-0 [f new]]]]], got %v", v)
		} else {
			t.Log("PASS wake: delivered [2-0 [f new]] after BLOCK 0")
		}
	case <-time.After(2 * time.Second):
		t.Fatal("FAIL wake: no reply within 2s of the new entry")
	}
}
