package persistence

import (
	"bufio"
	"bytes"
	"fmt"
	"sync"
	"testing"

	"github.com/cachestorm/cachestorm/internal/store"
)

func TestPersistenceGeoValueRaceX(t *testing.T) {
	geoVal := store.NewGeoValue()
	geoVal.Add("seed", 1, 1)
	writer := NewRDBWriter(store.NewStore(), RDBConfig{})
	rewriter := &AOFRewriter{}

	const iterations = 300
	var wg sync.WaitGroup
	wg.Add(2)
	go func() {
		defer wg.Done()
		for i := 0; i < iterations; i++ {
			geoVal.Add(fmt.Sprint(100+i), float64(i), float64(i))
		}
	}()
	go func() {
		defer wg.Done()
		var rdbBuf bytes.Buffer
		aofBuf := bufio.NewWriter(&bytes.Buffer{})
		for i := 0; i < iterations; i++ {
			if err := writer.writeValue(&rdbBuf, geoVal, 4); err != nil {
				t.Error(err)
				return
			}
			if err := rewriter.writeValueCommands(aofBuf, "k", geoVal); err != nil {
				t.Error(err)
				return
			}
		}
	}()
	wg.Wait()

	geoLen := len(geoVal.Points)
	if geoLen != 1+iterations {
		t.Fatalf("FAIL post-join control: geo=%d, want %d", geoLen, 1+iterations)
	}
	var final bytes.Buffer
	if err := writer.writeValue(&final, geoVal, 4); err != nil {
		t.Fatal(err)
	}
	if final.Len() == 0 {
		t.Fatal("FAIL sequential control: empty output")
	}
	t.Logf("PASS persistence writers vs concurrent GeoValue mutation under -race")
}
