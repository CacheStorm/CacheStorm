package store

import (
	"fmt"
	"slices"
	"strings"
	"testing"
	"time"
)

// AppendLiveKeys offers every unexpired key once, appends those keep
// accepts after dst, and skips expired ones.
func TestAppendLiveKeys(t *testing.T) {
	s := NewStore()
	var want []string
	for i := range 300 {
		k := fmt.Sprint("k", i)
		if err := s.Set(k, &StringValue{Data: []byte("v")}, SetOptions{}); err != nil {
			t.Fatal(err)
		}
		if strings.HasSuffix(k, "7") {
			want = append(want, k)
		}
	}
	if err := s.Set("gone7", &StringValue{Data: []byte("v")}, SetOptions{TTL: time.Millisecond}); err != nil {
		t.Fatal(err)
	}
	time.Sleep(5 * time.Millisecond)
	offered := 0
	got := s.AppendLiveKeys([]string{"first"}, func(k string, e *Entry) bool {
		offered++
		return e != nil && strings.HasSuffix(k, "7")
	})
	if offered != 300 || got[0] != "first" {
		t.Fatalf("offered %d keys, dst head %q", offered, got[0])
	}
	got = got[1:]
	slices.Sort(got)
	slices.Sort(want)
	if !slices.Equal(got, want) {
		t.Fatalf("kept %d keys, want %d", len(got), len(want))
	}
}
