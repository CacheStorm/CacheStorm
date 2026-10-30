package store

import (
	"sync"
	"sync/atomic"
	"time"
)

// unixClock is the current unix second, refreshed every clockTick so a key
// access does not read the system clock, as Redis' LRU clock is refreshed
// by its cron rather than read per access.
var (
	unixClock atomic.Int64
	clockOnce sync.Once
)

const clockTick = 100 * time.Millisecond

func nowUnix() int64 {
	if sec := unixClock.Load(); sec != 0 {
		return sec
	}
	clockOnce.Do(func() {
		unixClock.Store(time.Now().Unix())
		go func() {
			for now := range time.NewTicker(clockTick).C {
				unixClock.Store(now.Unix())
			}
		}()
	})
	return unixClock.Load()
}
