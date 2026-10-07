package pool

import (
	"net"
	"sync/atomic"
	"testing"
)

type duplicateReleaseConnX struct {
	net.Conn
	closes atomic.Int32
}

func (c *duplicateReleaseConnX) Close() error { c.closes.Add(1); return nil }

func TestPoolDuplicateReleaseRetainsIdleConnection(t *testing.T) {
	raw := &duplicateReleaseConnX{}
	p := NewPool(PoolConfig{MaxSize: 2, MaxIdle: 1}, func() (net.Conn, error) { return raw, nil })
	defer p.Close()
	c, err := p.Get()
	if err != nil {
		t.Fatal(err)
	}
	if err := c.Close(); err != nil {
		t.Fatal(err)
	}
	if p.Stats().Idle != 1 || raw.closes.Load() != 0 {
		t.Fatal("unaffected control failed")
	}
	t.Log("CONTROL: first release retains one open idle connection")
	staleRelease := make(chan struct{})
	done := make(chan error, 1)
	go func() { <-staleRelease; done <- c.Close() }()
	close(staleRelease)
	if err := <-done; err != nil {
		t.Fatal(err)
	}
	stats := p.Stats()
	t.Logf("EXPECTED: idle=1 raw closes=0; ACTUAL: idle=%d raw closes=%d", stats.Idle, raw.closes.Load())
	if stats.Idle != 1 || raw.closes.Load() != 0 {
		t.Fatal("PROBLEM CONFIRMED")
	}
	for i := 0; i < 3; i++ {
		if err := c.Close(); err != nil || p.Stats().Idle != 1 || raw.closes.Load() != 0 {
			t.Fatal("repeated idle release changed ownership")
		}
	}
	reused, err := p.Get()
	if err != nil || reused != c || p.Stats().InUse != 1 {
		t.Fatalf("reacquire: conn=%v error=%v stats=%+v", reused, err, p.Stats())
	}
	if err := reused.Close(); err != nil || p.Stats().Idle != 1 {
		t.Fatal("release after reacquire failed")
	}
	first, second := &duplicateReleaseConnX{}, &duplicateReleaseConnX{}
	created := 0
	excess := NewPool(PoolConfig{MaxSize: 2, MaxIdle: 1}, func() (net.Conn, error) {
		created++
		if created == 1 {
			return first, nil
		}
		return second, nil
	})
	defer excess.Close()
	one, err := excess.Get()
	if err != nil {
		t.Fatal(err)
	}
	two, err := excess.Get()
	if err != nil {
		t.Fatal(err)
	}
	if err := one.Close(); err != nil {
		t.Fatal(err)
	}
	if err := two.Close(); err != nil {
		t.Fatal(err)
	}
	if excess.Stats().Idle != 1 || first.closes.Load() != 0 || second.closes.Load() != 1 {
		t.Fatal("legitimate MaxIdle eviction changed")
	}
	t.Log("FIX VERIFIED")
}
