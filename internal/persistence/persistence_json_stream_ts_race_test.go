package persistence

import (
	"bufio"
	"bytes"
	"fmt"
	"sync"
	"testing"

	"github.com/cachestorm/cachestorm/internal/store"
)

func TestPersistenceJSONStreamTSValueRaceX(t *testing.T) {
	jsonVal := &store.JSONValue{Data: []byte(`{"a":1}`)}
	streamVal := &store.StreamValue{
		Entries: []*store.StreamEntry{{ID: "1-0", Fields: map[string][]byte{"f": []byte("v")}}},
		LastID:  "1-0",
	}
	tsVal := &store.TimeSeriesValue{
		Samples: []store.TimeSeriesSample{{Timestamp: 1, Value: 1}},
	}
	writer := NewRDBWriter(store.NewStore(), RDBConfig{})
	rewriter := &AOFRewriter{}

	const iterations = 300
	var wg sync.WaitGroup
	wg.Add(2)
	go func() {
		defer wg.Done()
		for i := 0; i < iterations; i++ {
			if err := jsonVal.Set(map[string]interface{}{"i": i}); err != nil {
				t.Error(err)
				return
			}
			if _, err := streamVal.Add(fmt.Sprintf("%d-0", 2+i), map[string][]byte{"f": []byte(fmt.Sprint(i))}); err != nil {
				t.Error(err)
				return
			}
			tsVal.Add(int64(100+i), float64(i))
		}
	}()
	go func() {
		defer wg.Done()
		var rdbBuf bytes.Buffer
		aofBuf := bufio.NewWriter(&bytes.Buffer{})
		for i := 0; i < iterations; i++ {
			if err := writer.writeValue(&rdbBuf, jsonVal, 5); err != nil {
				t.Error(err)
				return
			}
			if err := writer.writeValue(&rdbBuf, streamVal, 6); err != nil {
				t.Error(err)
				return
			}
			if err := writer.writeValue(&rdbBuf, tsVal, 7); err != nil {
				t.Error(err)
				return
			}
			if err := rewriter.writeValueCommands(aofBuf, "k", jsonVal); err != nil {
				t.Error(err)
				return
			}
			if err := rewriter.writeValueCommands(aofBuf, "k", streamVal); err != nil {
				t.Error(err)
				return
			}
			if err := rewriter.writeValueCommands(aofBuf, "k", tsVal); err != nil {
				t.Error(err)
				return
			}
		}
	}()
	wg.Wait()

	streamVal.RLock()
	streamLen := len(streamVal.Entries)
	streamVal.RUnlock()
	if streamLen != 1+iterations {
		t.Fatalf("FAIL post-join control: stream=%d, want %d", streamLen, 1+iterations)
	}
	if latest := tsVal.Latest(); latest == nil || latest.Timestamp != int64(100+iterations-1) {
		t.Fatalf("FAIL post-join control: latest=%v", latest)
	}
	var final bytes.Buffer
	if err := writer.writeValue(&final, jsonVal, 5); err != nil {
		t.Fatal(err)
	}
	if final.Len() == 0 {
		t.Fatal("FAIL sequential control: empty output")
	}
	t.Logf("PASS persistence writers vs store-mutator JSON/Stream/TS under -race")
}
