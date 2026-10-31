package store

import (
	"fmt"
	"testing"
)

func roundTrip(t *testing.T, v ExtensionValue) Value {
	t.Helper()
	data, err := v.MarshalBinary()
	if err != nil {
		t.Fatal(err)
	}
	back, ok, err := DecodeExtension(v.ExtensionType(), data)
	if !ok || err != nil {
		t.Fatalf("decode %T: ok=%v err=%v", v, ok, err)
	}
	if _, _, err := DecodeExtension(v.ExtensionType(), data[:len(data)-1]); err == nil {
		t.Fatalf("a truncated %T decoded", v)
	}
	return back
}

func TestBloomScalesAndRoundTrips(t *testing.T) {
	b := NewBloomValue(10, 0.01, 2)
	for i := 0; i < 100; i++ {
		if _, err := b.Add([]byte(fmt.Sprint(i))); err != nil {
			t.Fatal(err)
		}
	}
	info := b.Info()
	if info.Filters < 3 || info.Capacity < 100 || info.Items < 95 {
		t.Fatalf("scaling filter info = %+v", info)
	}
	if added, _ := b.Add([]byte("5")); added {
		t.Fatal("an item already added was added again")
	}
	back := roundTrip(t, b).(*BloomValue)
	for i := 0; i < 100; i++ {
		if !back.Exists([]byte(fmt.Sprint(i))) {
			t.Fatalf("item %d lost in the round trip", i)
		}
	}
	fixed := NewBloomValue(2, 0.01, 0)
	fixed.Add([]byte("a")) //nolint:errcheck
	fixed.Add([]byte("b")) //nolint:errcheck
	if _, err := fixed.Add([]byte("c")); err != ErrBloomFull {
		t.Fatalf("a full non-scaling filter accepted an item: %v", err)
	}
}

func TestCuckooCountsDeletesAndExpands(t *testing.T) {
	c := NewCuckooValue(4, 2, 20, 0)
	c.Add([]byte("x"), false) //nolint:errcheck
	c.Add([]byte("x"), false) //nolint:errcheck
	if added, _ := c.Add([]byte("x"), true); added || c.Count([]byte("x")) != 2 {
		t.Fatal("ADDNX added a present item, or the count is off")
	}
	if !c.Delete([]byte("x")) || c.Count([]byte("x")) != 1 || c.Info().Deleted != 1 {
		t.Fatal("delete did not remove one copy")
	}
	full := false
	for i := 0; i < 64 && !full; i++ {
		_, err := c.Add([]byte(fmt.Sprint(i)), false)
		full = err == ErrCuckooFull
	}
	if !full {
		t.Fatal("a filter that cannot expand never filled")
	}
	grow := NewCuckooValue(4, 2, 20, 2)
	for i := 0; i < 64; i++ {
		if _, err := grow.Add([]byte(fmt.Sprint(i)), false); err != nil {
			t.Fatal(err)
		}
	}
	back := roundTrip(t, grow).(*CuckooValue)
	if back.Info().Filters < 2 {
		t.Fatal("an expanding filter did not add filters")
	}
	for i := 0; i < 64; i++ {
		if back.Count([]byte(fmt.Sprint(i))) == 0 {
			t.Fatalf("item %d lost", i)
		}
	}
}

func TestCMSOverflowMergeAndRoundTrip(t *testing.T) {
	m := NewCMSValue(100, 4)
	if n, err := m.IncrBy([]byte("a"), 5); n != 5 || err != nil {
		t.Fatalf("IncrBy = %d, %v", n, err)
	}
	if n, err := m.IncrBy([]byte("a"), 1<<32); n != 1<<32-1 || err != ErrCMSOverflow {
		t.Fatalf("overflowing IncrBy = %d, %v", n, err)
	}
	src := NewCMSValue(100, 4)
	src.IncrBy([]byte("b"), 3) //nolint:errcheck
	dst := NewCMSValue(100, 4)
	dst.Merge([]*CMSValue{src, src}, []int64{2, 1})
	if dst.Query([]byte("b")) != 9 || dst.Count != 9 {
		t.Fatalf("weighted merge = %d (count %d)", dst.Query([]byte("b")), dst.Count)
	}
	if roundTrip(t, dst).(*CMSValue).Query([]byte("b")) != 9 {
		t.Fatal("count lost in the round trip")
	}
}

func TestTopKKeepsHeavyItems(t *testing.T) {
	tk := NewTopKValue(2, 50, 5, 0.9)
	tk.Add([]byte("hot"), 100)
	tk.Add([]byte("warm"), 10)
	for i := 0; i < 50; i++ {
		tk.Add([]byte(fmt.Sprint("cold", i)), 1)
	}
	list := tk.List()
	if len(list) != 2 || list[0].Item != "hot" || list[1].Item != "warm" {
		t.Fatalf("top list = %+v", list)
	}
	back := roundTrip(t, tk).(*TopKValue)
	if !back.Query([]byte("hot")) || back.Count([]byte("hot")) < 100 {
		t.Fatal("top list lost in the round trip")
	}
}
