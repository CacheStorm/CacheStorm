package command

import (
	"bytes"
	"fmt"
	"runtime"
	"sync"
	"testing"

	"github.com/cachestorm/cachestorm/internal/resp"
	"github.com/cachestorm/cachestorm/internal/store"
)

func TestListConcurrentRaceX(t *testing.T) {
	s := store.NewStore()
	router := NewRouter()
	RegisterListCommands(router)
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

	if v := run("RPUSH", "k", "a", "b", "c"); v.Type != resp.TypeInteger || v.Int != 3 {
		t.Fatalf("seed failed: %v", v)
	}
	if v := run("LRANGE", "k", "0", "-1"); v.Type != resp.TypeArray || len(v.Array) != 3 {
		t.Fatalf("FAIL sequential control: got %v", v)
	}
	t.Log("PASS sequential control: [a b c]")

	const iterations = 400
	var wg sync.WaitGroup
	wg.Add(2)
	go func() {
		defer wg.Done()
		for i := 0; i < iterations; i++ {
			if v := run("RPUSH", "k", fmt.Sprint(i)); v.Type != resp.TypeInteger {
				t.Errorf("writer: RPUSH failed: %v", v)
				return
			}
			runtime.Gosched()
		}
	}()
	go func() {
		defer wg.Done()
		for i := 0; i < iterations; i++ {
			run("LRANGE", "k", "0", "-1")
			run("LLEN", "k")
			runtime.Gosched()
		}
	}()
	wg.Wait()

	total := run("LLEN", "k")
	if total.Type != resp.TypeInteger || total.Int != 3+iterations {
		t.Errorf("FAIL post-join control: LLEN=%v, want %d", total, 3+iterations)
	} else {
		t.Log("PASS post-join control: all elements present")
	}
	if last := run("LRANGE", "k", "-1", "-1"); last.Type != resp.TypeArray || len(last.Array) != 1 ||
		string(last.Array[0].Bulk) != fmt.Sprint(iterations-1) {
		t.Errorf("FAIL post-join last element: got %v", last)
	} else {
		t.Log("PASS post-join last element")
	}
}
