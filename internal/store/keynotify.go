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
	// consumers marks the waiters that will take the data they are woken
	// for (list and sorted-set pops). A write wakes only the oldest of them,
	// so blocked clients are served first come first served, as in Redis;
	// the others stay registered until the data reaches them.
	consumers map[chan struct{}]bool
	// wakeHooks tells a multi-key waiter which key woke it.
	wakeHooks map[chan struct{}]func(string)
	// woken holds waiters already signalled: a waiter registered on several
	// keys is woken once, and a later write to another of its keys must go
	// to the next waiter instead of being absorbed by it.
	woken map[chan struct{}]bool
	// inflight records, per key, the consumer that was woken and has not
	// finished yet. Until it passes the key on, a further write wakes no other
	// consumer: the woken one is served first, so a second push cannot let a
	// later waiter overtake it. A token older than inflightGrace is ignored,
	// so a consumer that never passes cannot stall the key.
	inflight map[string]inflightToken
}

type inflightToken struct {
	ch    chan struct{}
	since time.Time
}

const inflightGrace = time.Second

func NewKeyNotifier() *KeyNotifier {
	return &KeyNotifier{
		waiters:   make(map[string][]chan struct{}),
		consumers: make(map[chan struct{}]bool),
	}
}

// WaitOptions tunes WaitForKeysOpts.
type WaitOptions struct {
	// Cancel, when non-nil, ends the wait early (the client went away) and
	// reports a timeout.
	Cancel <-chan struct{}
	// Ready, when non-nil, is checked for every key after the waiters are
	// registered: a write that landed between the caller's last look at the
	// data and the registration has already fired its notification, so
	// without the check the caller would sleep through it.
	Ready func(key string) bool
	// Consume marks the caller as a consumer of the data: it is woken only
	// when it is the oldest consumer waiting on the key.
	Consume bool
	// Notified, when non-nil, is set to whether the wait ended by a
	// notification (true) rather than the Ready check; only a notified
	// consumer holds the key's in-flight turn and must Pass it.
	Notified *bool
	// Front registers the waiter ahead of the others. A consumer that was
	// woken but found the data already taken waits again with Front, so it
	// keeps the place it had instead of going to the back of the queue.
	Front bool
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
	return kn.WaitForKeysCancel(keys, timeout, nil, nil)
}

// WaitForKeysCancel waits as an observer (every waiter is woken) with the
// Cancel and Ready options of WaitForKeysOpts.
func (kn *KeyNotifier) WaitForKeysCancel(keys []string, timeout time.Duration, cancel <-chan struct{}, ready func(key string) bool) (string, bool) {
	return kn.WaitForKeysOpts(keys, timeout, WaitOptions{Cancel: cancel, Ready: ready})
}

// WaitForKeysOpts blocks until one of keys is notified, the timeout expires
// or opts.Cancel is closed. It returns the notified key and true, or "" and
// false.
func (kn *KeyNotifier) WaitForKeysOpts(keys []string, timeout time.Duration, opts WaitOptions) (string, bool) {
	// One channel serves every key: the waiter is woken once whichever key
	// fires. which records the key so the caller knows where to look.
	ch := make(chan struct{}, 1)
	var which string
	var whichMu sync.Mutex

	kn.mu.Lock()
	for _, key := range keys {
		if opts.Front {
			kn.waiters[key] = append([]chan struct{}{ch}, kn.waiters[key]...)
		} else {
			kn.waiters[key] = append(kn.waiters[key], ch)
		}
	}
	if opts.Consume {
		kn.consumers[ch] = true
	}
	kn.mu.Unlock()

	woken := func() (string, bool) {
		whichMu.Lock()
		defer whichMu.Unlock()
		return which, which != ""
	}
	kn.recordWake(ch, func(key string) {
		whichMu.Lock()
		if which == "" {
			which = key
		}
		whichMu.Unlock()
	})

	finish := func(notified bool) {
		kn.mu.Lock()
		for _, key := range keys {
			kn.removeWaiter(key, ch)
		}
		delete(kn.consumers, ch)
		delete(kn.wakeHooks, ch)
		delete(kn.woken, ch)
		kn.mu.Unlock()
		// A consumer that was woken but is leaving without the data (it
		// timed out or its client went away at the same moment) passes the
		// wakeup on, or the next consumer would sleep with data present.
		if !notified && opts.Consume {
			if key, ok := woken(); ok {
				kn.release(key, ch)
				kn.NotifyKey(key)
			}
		}
	}

	if opts.Ready != nil {
		for _, key := range keys {
			// Data on a key with an older consumer waiting belongs to that
			// consumer; checking it here would let a newcomer jump the queue.
			if opts.Consume && kn.consumerAhead(key, ch) {
				continue
			}
			if opts.Ready(key) {
				finish(true)
				return key, true
			}
		}
	}

	if timeout == 0 {
		select {
		case <-ch:
			finish(true)
			if opts.Notified != nil {
				*opts.Notified = true
			}
			return woken()
		default:
			finish(false)
			return "", false
		}
	}

	timer := time.NewTimer(timeout)
	defer timer.Stop()

	select {
	case <-ch:
		finish(true)
		if opts.Notified != nil {
			*opts.Notified = true
		}
		return woken()
	case <-timer.C:
	case <-opts.Cancel:
	}
	finish(false)
	return "", false
}

// consumerAhead reports whether a consumer that has not been woken yet is
// registered on key ahead of ch.
func (kn *KeyNotifier) consumerAhead(key string, ch chan struct{}) bool {
	kn.mu.Lock()
	defer kn.mu.Unlock()
	for _, w := range kn.waiters[key] {
		if w == ch {
			return false
		}
		if kn.consumers[w] && !kn.woken[w] {
			return true
		}
	}
	return false
}

// recordWake registers fn to learn which key woke the waiter behind ch.
func (kn *KeyNotifier) recordWake(ch chan struct{}, fn func(key string)) {
	kn.mu.Lock()
	if kn.wakeHooks == nil {
		kn.wakeHooks = make(map[chan struct{}]func(string))
	}
	kn.wakeHooks[ch] = fn
	kn.mu.Unlock()
}

// NotifyKey wakes the waiters on a key after a write (LPUSH, ZADD, XADD,
// ...). Every observer is woken, and the oldest consumer: a consumer that
// leaves the data for others passes the wakeup on (see Pass).
func (kn *KeyNotifier) NotifyKey(key string) {
	kn.mu.Lock()
	waiters := kn.waiters[key]
	if len(waiters) > 0 {
		kept := waiters[:0]
		consumerWoken := false
		if tok, busy := kn.inflight[key]; busy && time.Since(tok.since) < inflightGrace {
			consumerWoken = true
		}
		for _, ch := range waiters {
			if kn.woken[ch] {
				continue
			}
			if kn.consumers[ch] {
				if consumerWoken {
					kept = append(kept, ch)
					continue
				}
				consumerWoken = true
				if kn.inflight == nil {
					kn.inflight = make(map[string]inflightToken)
				}
				kn.inflight[key] = inflightToken{ch: ch, since: time.Now()}
			}
			if fn := kn.wakeHooks[ch]; fn != nil {
				fn(key)
			}
			if kn.woken == nil {
				kn.woken = make(map[chan struct{}]bool)
			}
			kn.woken[ch] = true
			select {
			case ch <- struct{}{}:
			default:
			}
		}
		if len(kept) == 0 {
			delete(kn.waiters, key)
		} else {
			kn.waiters[key] = kept
		}
	}
	kn.mu.Unlock()
}

// WaiterCount reports how many clients are blocked on key.
func (kn *KeyNotifier) WaiterCount(key string) int {
	kn.mu.Lock()
	defer kn.mu.Unlock()
	return len(kn.waiters[key])
}

// Pass ends a woken consumer's turn on key and hands the wakeup to the next
// consumer if the key still holds data. owner says whether the caller was
// woken by a notification and so holds the turn.
func (kn *KeyNotifier) Pass(key string, ready func(string) bool, owner bool) {
	if owner {
		kn.mu.Lock()
		delete(kn.inflight, key)
		kn.mu.Unlock()
	}
	if ready == nil || ready(key) {
		kn.NotifyKey(key)
	}
}

// release drops ch's in-flight turn on key, if it holds it.
func (kn *KeyNotifier) release(key string, ch chan struct{}) {
	kn.mu.Lock()
	if tok, ok := kn.inflight[key]; ok && tok.ch == ch {
		delete(kn.inflight, key)
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
