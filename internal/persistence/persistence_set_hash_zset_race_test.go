package persistence

import (
	"bufio"
	"bytes"
	"fmt"
	"sync"
	"testing"

	"github.com/cachestorm/cachestorm/internal/store"
)

func TestPersistenceSetHashZSetValueRaceX(t *testing.T) {
	setVal := &store.SetValue{Members: map[string]struct{}{"a": {}, "b": {}}}
	hashVal := &store.HashValue{Fields: map[string][]byte{"f": []byte("1")}}
	zsetVal := &store.SortedSetValue{Members: map[string]float64{"m": 1}}
	writer := NewRDBWriter(store.NewStore(), RDBConfig{})
	rewriter := &AOFRewriter{}

	const iterations = 300
	var wg sync.WaitGroup
	wg.Add(2)
	go func() {
		defer wg.Done()
		for i := 0; i < iterations; i++ {
			setVal.Lock()
			setVal.Members[fmt.Sprint(i)] = struct{}{}
			setVal.Unlock()
			hashVal.Lock()
			hashVal.Fields[fmt.Sprint(i)] = []byte(fmt.Sprint(i))
			hashVal.Unlock()
			zsetVal.Lock()
			zsetVal.Members[fmt.Sprint(i)] = float64(i)
			zsetVal.Unlock()
		}
	}()
	go func() {
		defer wg.Done()
		var rdbBuf bytes.Buffer
		aofBuf := bufio.NewWriter(&bytes.Buffer{})
		for i := 0; i < iterations; i++ {
			if err := writer.writeValue(&rdbBuf, setVal, 2); err != nil {
				t.Error(err)
				return
			}
			if err := writer.writeValue(&rdbBuf, hashVal, 3); err != nil {
				t.Error(err)
				return
			}
			if err := writer.writeValue(&rdbBuf, zsetVal, 4); err != nil {
				t.Error(err)
				return
			}
			if err := rewriter.writeValueCommands(aofBuf, "k", setVal); err != nil {
				t.Error(err)
				return
			}
			if err := rewriter.writeValueCommands(aofBuf, "k", hashVal); err != nil {
				t.Error(err)
				return
			}
			if err := rewriter.writeValueCommands(aofBuf, "k", zsetVal); err != nil {
				t.Error(err)
				return
			}
		}
	}()
	wg.Wait()

	setVal.RLock()
	setLen := len(setVal.Members)
	setVal.RUnlock()
	hashVal.RLock()
	hashLen := len(hashVal.Fields)
	hashVal.RUnlock()
	zsetVal.RLock()
	zsetLen := len(zsetVal.Members)
	zsetVal.RUnlock()
	if setLen != 2+iterations || hashLen != 1+iterations || zsetLen != 1+iterations {
		t.Fatalf("FAIL post-join control: set=%d hash=%d zset=%d", setLen, hashLen, zsetLen)
	}
	var final bytes.Buffer
	if err := writer.writeValue(&final, setVal, 2); err != nil {
		t.Fatal(err)
	}
	if final.Len() == 0 {
		t.Fatal("FAIL sequential control: empty RDB output")
	}
	t.Logf("PASS persistence writers vs locked Set/Hash/ZSet mutation under -race")
}
