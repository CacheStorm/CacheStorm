package persistence

import (
	"bufio"
	"bytes"
	"fmt"
	"sync"
	"testing"

	"github.com/cachestorm/cachestorm/internal/store"
)

func TestPersistenceListValueRaceX(t *testing.T) {
	list := &store.ListValue{Elements: [][]byte{[]byte("a"), []byte("b"), []byte("c")}}
	writer := NewRDBWriter(store.NewStore(), RDBConfig{})
	rewriter := &AOFRewriter{}

	const iterations = 400
	var wg sync.WaitGroup
	wg.Add(2)
	go func() {
		defer wg.Done()
		for i := 0; i < iterations; i++ {
			list.Lock()
			list.Elements = append(list.Elements, []byte(fmt.Sprint(i)))
			list.Unlock()
		}
	}()
	go func() {
		defer wg.Done()
		var rdbBuf bytes.Buffer
		aofBuf := bufio.NewWriter(&bytes.Buffer{})
		for i := 0; i < iterations; i++ {
			if err := writer.writeValue(&rdbBuf, list, 1); err != nil {
				t.Error(err)
				return
			}
			if err := rewriter.writeValueCommands(aofBuf, "l", list); err != nil {
				t.Error(err)
				return
			}
		}
	}()
	wg.Wait()

	list.RLock()
	got := len(list.Elements)
	list.RUnlock()
	if got != 3+iterations {
		t.Fatalf("FAIL post-join control: len=%d, want %d", got, 3+iterations)
	}
	var final bytes.Buffer
	if err := writer.writeValue(&final, list, 1); err != nil {
		t.Fatal(err)
	}
	if final.Len() == 0 {
		t.Fatal("FAIL sequential control: empty RDB output")
	}
	t.Logf("PASS persistence writers vs locked concurrent mutation under -race")
}
