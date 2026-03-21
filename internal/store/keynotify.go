package store

import (
	"sync"
	"time"
)

// KeyNotifier provides a wait/notify mechanism for blocking commands.
// Waiters block on a channel until the key is written to, or timeout expires.
type KeyNotifier struct {
	mu      sync.Mutex
	waiters map[string][]chan struct{}
}

func NewKeyNotifier() *KeyNotifier {
	return &KeyNotifier{
		waiters: make(map[string][]chan struct{}),
	}
}

// WaitForKey blocks until the key receives a write notification or the timeout expires.
// Returns true if notified, false if timed out.
func (kn *KeyNotifier) WaitForKey(key string, timeout time.Duration) bool {
	ch := make(chan struct{}, 1)

	kn.mu.Lock()
	kn.waiters[key] = append(kn.waiters[key], ch)
	kn.mu.Unlock()

	defer func() {
		kn.mu.Lock()
		kn.removeWaiter(key, ch)
		kn.mu.Unlock()
	}()

	if timeout == 0 {
		// Non-blocking check — don't wait
		select {
		case <-ch:
			return true
		default:
			return false
		}
	}

	timer := time.NewTimer(timeout)
	defer timer.Stop()

	select {
	case <-ch:
		return true
	case <-timer.C:
		return false
	}
}

// WaitForKeys blocks until any of the keys receives a notification or timeout expires.
// Returns the key that was notified and true, or empty string and false on timeout.
func (kn *KeyNotifier) WaitForKeys(keys []string, timeout time.Duration) (string, bool) {
	ch := make(chan string, 1)
	perKey := make([]chan struct{}, len(keys))

	kn.mu.Lock()
	for i, key := range keys {
		notify := make(chan struct{}, 1)
		perKey[i] = notify
		kn.waiters[key] = append(kn.waiters[key], notify)
	}
	kn.mu.Unlock()

	// Start goroutines that forward per-key notifications
	done := make(chan struct{})
	for i, key := range keys {
		go func(k string, n chan struct{}) {
			select {
			case <-n:
				select {
				case ch <- k:
				default:
				}
			case <-done:
			}
		}(key, perKey[i])
	}

	defer func() {
		close(done)
		kn.mu.Lock()
		for i, key := range keys {
			kn.removeWaiter(key, perKey[i])
		}
		kn.mu.Unlock()
	}()

	if timeout == 0 {
		select {
		case k := <-ch:
			return k, true
		default:
			return "", false
		}
	}

	timer := time.NewTimer(timeout)
	defer timer.Stop()

	select {
	case k := <-ch:
		return k, true
	case <-timer.C:
		return "", false
	}
}

// NotifyKey wakes all waiters on a key. Called after writes (LPUSH, RPUSH, ZADD, etc.)
func (kn *KeyNotifier) NotifyKey(key string) {
	kn.mu.Lock()
	waiters := kn.waiters[key]
	if len(waiters) > 0 {
		// Wake all waiters
		for _, ch := range waiters {
			select {
			case ch <- struct{}{}:
			default:
			}
		}
		delete(kn.waiters, key)
	}
	kn.mu.Unlock()
}

func (kn *KeyNotifier) removeWaiter(key string, ch chan struct{}) {
	waiters := kn.waiters[key]
	for i, w := range waiters {
		if w == ch {
			kn.waiters[key] = append(waiters[:i], waiters[i+1:]...)
			break
		}
	}
	if len(kn.waiters[key]) == 0 {
		delete(kn.waiters, key)
	}
}
