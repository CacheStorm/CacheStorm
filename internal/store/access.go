package store

import (
	"math/rand/v2"
	"sync/atomic"
	"time"
)

// An entry's access word keeps what Redis keeps in an object's 24 lru bits,
// for both eviction policies at once: the unix second of the last access
// (bits 24 and up), and the LFU counter (bits 0-7) with the minute, modulo
// 2^16, it was last decremented (bits 8-23). The counter follows Redis'
// LFULogIncr and LFUDecrAndReturn.
const lfuInitVal = 5

var (
	lfuLogFactor atomic.Int64
	lfuDecayTime atomic.Int64
)

func init() {
	lfuLogFactor.Store(10)
	lfuDecayTime.Store(1)
}

// SetLFUParams sets lfu-log-factor and lfu-decay-time.
func SetLFUParams(logFactor, decayTime int64) {
	lfuLogFactor.Store(logFactor)
	lfuDecayTime.Store(decayTime)
}

func lfuMinutes(sec int64) uint64 { return uint64(sec/60) & 0xFFFF }

func packAccess(sec int64, ldt, counter uint64) uint64 {
	return uint64(sec)<<24 | ldt<<8 | counter
}

func newAccess(sec int64) uint64 { return packAccess(sec, lfuMinutes(sec), lfuInitVal) }

// lfuDecr is LFUDecrAndReturn: the counter less one for every lfu-decay-time
// minutes since it was last decremented.
func lfuDecr(v uint64, nowMin uint64) uint64 {
	counter, ldt := v&0xFF, (v>>8)&0xFFFF
	elapsed := nowMin - ldt
	if nowMin < ldt {
		elapsed = 65535 - ldt + nowMin
	}
	decay := lfuDecayTime.Load()
	if decay <= 0 {
		return counter
	}
	periods := elapsed / uint64(decay)
	if periods > counter {
		return 0
	}
	return counter - periods
}

// lfuLogIncr is LFULogIncr: the counter grows with a probability that falls
// as it rises, by lfu-log-factor.
func lfuLogIncr(counter uint64) uint64 {
	if counter == 255 {
		return 255
	}
	base := float64(counter) - lfuInitVal
	if base < 0 {
		base = 0
	}
	if rand.Float64() < 1/(base*float64(lfuLogFactor.Load())+1) {
		counter++
	}
	return counter
}

// Touch records an access to the key: its last access time and Redis' LFU
// counter.
func (e *Entry) Touch() {
	now := time.Now().Unix()
	nowMin := lfuMinutes(now)
	for {
		old := e.access.Load()
		next := packAccess(now, nowMin, lfuLogIncr(lfuDecr(old, nowMin)))
		if e.access.CompareAndSwap(old, next) {
			return
		}
	}
}

// inheritAccess gives an entry replacing prev under the same key prev's LFU
// counter, as Redis' dbSetValue keeps the old object's, counting the lookup
// that found prev.
func (e *Entry) inheritAccess(prev *Entry) {
	e.access.Store(prev.access.Load())
	e.Touch()
}

// LastAccess is the unix second of the key's last access.
func (e *Entry) LastAccess() int64 { return int64(e.access.Load() >> 24) }

// IdleSeconds is how long ago the key was last accessed, as OBJECT IDLETIME
// reports it.
func (e *Entry) IdleSeconds() int64 { return max(time.Now().Unix()-e.LastAccess(), 0) }

// LFUCount is the key's LFU counter as OBJECT FREQ reports it, decayed to
// now.
func (e *Entry) LFUCount() uint8 {
	return uint8(lfuDecr(e.access.Load(), lfuMinutes(time.Now().Unix())))
}

// SetIdleSeconds makes the key last accessed idle seconds ago, as RESTORE's
// IDLETIME does.
func (e *Entry) SetIdleSeconds(idle int64) {
	for {
		old := e.access.Load()
		if e.access.CompareAndSwap(old, packAccess(time.Now().Unix()-idle, 0, 0)|old&0xFFFFFF) {
			return
		}
	}
}

// SetLFUCount sets the key's LFU counter, as RESTORE's FREQ does.
func (e *Entry) SetLFUCount(counter uint8) {
	now := time.Now().Unix()
	for {
		old := e.access.Load()
		if e.access.CompareAndSwap(old, old&^0xFFFFFF|lfuMinutes(now)<<8|uint64(counter)) {
			return
		}
	}
}
