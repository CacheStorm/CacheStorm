package command

import (
	"errors"
	"math"
	"strconv"
	"sync"
	"time"

	"github.com/cachestorm/cachestorm/internal/resp"
	"github.com/cachestorm/cachestorm/internal/store"
)

// A SEM semaphore is a hash key holding the permits free (value) and the
// permits it has (max); a SEMAPHOREX semaphore holds the permits taken
// (current) and max. Being keys, they are per database, persisted and
// replicated, so permits taken before a restart stay taken; changes are
// recorded as HSET. SEM.ACQUIRE may wait for permits: it waits without the
// write gate and wakes when the key is written or every lockPoll.

var (
	semKind         = recordKind{name: "semaphore", fields: []string{"value", "max"}}
	semXKind        = recordKind{name: "semaphore", fields: []string{"current", "max"}}
	errSemNotFound  = errors.New("ERR semaphore not found")
	errSemCount     = errors.New("ERR permit count must be a positive integer")
	errSemOverMax   = errors.New("ERR permit count exceeds the semaphore's maximum")
	errSemOverflow  = errors.New("ERR release would exceed the semaphore's maximum")
	errSemTimeout   = errors.New("ERR timeout must be a non-negative integer (milliseconds)")
	errSemCreateMax = errors.New("ERR max must be a positive integer")
)

// semWaiters counts the SEM.ACQUIRE calls blocked on each semaphore, by
// database and key, for SEM.VALUE.
var (
	semWaitersMu sync.Mutex
	semWaiters   = map[string]int64{}
)

func semWaiterKey(ctx *Context, key string) string {
	return strconv.Itoa(ctx.DB) + "\x00" + key
}

func addSemWaiter(ctx *Context, key string, d int64) {
	k := semWaiterKey(ctx, key)
	semWaitersMu.Lock()
	if semWaiters[k] += d; semWaiters[k] <= 0 {
		delete(semWaiters, k)
	}
	semWaitersMu.Unlock()
}

func parseSemCount(s string) (int64, error) {
	n, err := strconv.ParseInt(s, 10, 64)
	if err != nil || n < 1 {
		return 0, errSemCount
	}
	return n, nil
}

func loadSem(ctx *Context, key string) (value, maxPermits int64, err error) {
	r, err := semKind.read(ctx, key)
	if err != nil {
		return 0, 0, err
	}
	if r == nil {
		return 0, 0, errSemNotFound
	}
	value, maxPermits = r.int("value"), r.int("max")
	if r.bad || value < 0 || value > maxPermits {
		return 0, 0, store.ErrWrongType
	}
	return value, maxPermits, nil
}

func cmdSEMCREATE(ctx *Context) error {
	if ctx.ArgCount() != 2 {
		return ctx.WriteError(ErrWrongArgCount)
	}
	maxPermits, err := strconv.ParseInt(ctx.ArgString(1), 10, 64)
	if err != nil || maxPermits < 1 {
		return ctx.WriteError(errSemCreateMax)
	}
	key := ctx.ArgString(0)
	if err := createRecord(ctx, semKind, key, "value", i64(maxPermits), "max", i64(maxPermits)); err != nil {
		return err
	}
	ctx.Store.KeyNotifier().NotifyKey(key)
	return nil
}

// trySem takes n permits when they are free.
func trySem(ctx *Context, key string, n int64) (bool, error) {
	value, maxPermits, err := loadSem(ctx, key)
	if err != nil {
		return false, err
	}
	if n > maxPermits {
		return false, errSemOverMax
	}
	if value < n {
		return false, nil
	}
	return true, updateRecord(ctx, key, "value", i64(value-n))
}

func semReply(ctx *Context, ok bool, err error) error {
	if err != nil {
		return ctx.WriteError(err)
	}
	if !ok {
		ctx.PropagateAs()
	}
	return ctx.WriteInteger(utilityBoolToInt(ok))
}

// SEM.ACQUIRE name n [timeout_ms] takes n permits, waiting for them up to
// timeout_ms; 0 or no timeout waits until they are free. It does not wait
// inside MULTI or a script.
func cmdSEMACQUIRE(ctx *Context) error {
	if ctx.ArgCount() < 2 || ctx.ArgCount() > 3 {
		return ctx.WriteError(ErrWrongArgCount)
	}
	key := ctx.ArgString(0)
	n, err := parseSemCount(ctx.ArgString(1))
	if err != nil {
		return ctx.WriteError(err)
	}
	var deadline time.Time
	if ctx.ArgCount() == 3 {
		ms, err := strconv.ParseInt(ctx.ArgString(2), 10, 64)
		if err != nil || ms < 0 || ms > math.MaxInt64/int64(time.Millisecond) {
			return ctx.WriteError(errSemTimeout)
		}
		if ms > 0 {
			deadline = time.Now().Add(time.Duration(ms) * time.Millisecond)
		}
	}
	ok, err := trySem(ctx, key, n)
	if err != nil || ok || ctx.InExec {
		return semReply(ctx, ok, err)
	}
	addSemWaiter(ctx, key, 1)
	defer addSemWaiter(ctx, key, -1)
	for !ok && err == nil {
		wait := lockPoll
		if !deadline.IsZero() {
			remaining := time.Until(deadline)
			if remaining <= 0 {
				break
			}
			wait = min(remaining, lockPoll)
		}
		if ctx.Done != nil {
			select {
			case <-ctx.Done:
				return semReply(ctx, false, nil)
			default:
			}
		}
		ctx.waitForStreamKeys([]string{key}, wait, func(k string) bool {
			value, _, err := loadSem(ctx, k)
			return err != nil || value >= n
		})
		ok, err = trySem(ctx, key, n)
	}
	return semReply(ctx, ok, err)
}

func cmdSEMTRYACQUIRE(ctx *Context) error {
	if ctx.ArgCount() != 2 {
		return ctx.WriteError(ErrWrongArgCount)
	}
	n, err := parseSemCount(ctx.ArgString(1))
	if err != nil {
		return ctx.WriteError(err)
	}
	ok, err := trySem(ctx, ctx.ArgString(0), n)
	return semReply(ctx, ok, err)
}

func cmdSEMRELEASE(ctx *Context) error {
	if ctx.ArgCount() != 2 {
		return ctx.WriteError(ErrWrongArgCount)
	}
	key := ctx.ArgString(0)
	n, err := parseSemCount(ctx.ArgString(1))
	if err != nil {
		return ctx.WriteError(err)
	}
	value, maxPermits, err := loadSem(ctx, key)
	if err != nil {
		return ctx.WriteError(err)
	}
	if n > maxPermits-value {
		return ctx.WriteError(errSemOverflow)
	}
	if err := updateRecord(ctx, key, "value", i64(value+n)); err != nil {
		return ctx.WriteError(err)
	}
	ctx.Store.KeyNotifier().NotifyKey(key)
	return ctx.WriteOK()
}

func cmdSEMVALUE(ctx *Context) error {
	if ctx.ArgCount() != 1 {
		return ctx.WriteError(ErrWrongArgCount)
	}
	key := ctx.ArgString(0)
	value, maxPermits, err := loadSem(ctx, key)
	if err != nil {
		return ctx.WriteError(err)
	}
	semWaitersMu.Lock()
	waiters := semWaiters[semWaiterKey(ctx, key)]
	semWaitersMu.Unlock()
	return ctx.WriteArray([]*resp.Value{
		resp.BulkString("value"), resp.IntegerValue(value),
		resp.BulkString("max"), resp.IntegerValue(maxPermits),
		resp.BulkString("waiters"), resp.IntegerValue(waiters),
	})
}

func loadSemX(ctx *Context, key string) (current, maxPermits int64, err error) {
	r, err := semXKind.load(ctx, key)
	if err != nil {
		return 0, 0, err
	}
	current, maxPermits = r.int("current"), r.int("max")
	return current, maxPermits, errWrongTypeOr(r.bad, nil)
}

func semXCount(ctx *Context) (int64, error) {
	if ctx.ArgCount() < 2 {
		return 1, nil
	}
	return parseSemCount(ctx.ArgString(1))
}

func cmdSEMAPHOREXCREATE(ctx *Context) error {
	if ctx.ArgCount() != 2 {
		return ctx.WriteError(ErrWrongArgCount)
	}
	maxPermits, err := strconv.ParseInt(ctx.ArgString(1), 10, 64)
	if err != nil || maxPermits < 1 {
		return ctx.WriteError(errSemCreateMax)
	}
	return createRecord(ctx, semXKind, ctx.ArgString(0), "current", "0", "max", i64(maxPermits))
}

func cmdSEMAPHOREXACQUIRE(ctx *Context) error {
	if ctx.ArgCount() < 1 || ctx.ArgCount() > 2 {
		return ctx.WriteError(ErrWrongArgCount)
	}
	key := ctx.ArgString(0)
	n, err := semXCount(ctx)
	if err != nil {
		return ctx.WriteError(err)
	}
	current, maxPermits, err := loadSemX(ctx, key)
	if err != nil {
		return ctx.WriteError(err)
	}
	if n > maxPermits-current {
		ctx.PropagateAs()
		return ctx.WriteInteger(0)
	}
	if err := updateRecord(ctx, key, "current", i64(current+n)); err != nil {
		return ctx.WriteError(err)
	}
	return ctx.WriteInteger(1)
}

// SEMAPHOREX.RELEASE name [n] gives back n permits, never below none taken.
func cmdSEMAPHOREXRELEASE(ctx *Context) error {
	if ctx.ArgCount() < 1 || ctx.ArgCount() > 2 {
		return ctx.WriteError(ErrWrongArgCount)
	}
	key := ctx.ArgString(0)
	n, err := semXCount(ctx)
	if err != nil {
		return ctx.WriteError(err)
	}
	current, _, err := loadSemX(ctx, key)
	if err != nil {
		return ctx.WriteError(err)
	}
	if err := updateRecord(ctx, key, "current", i64(max(current-n, 0))); err != nil {
		return ctx.WriteError(err)
	}
	return ctx.WriteOK()
}

func cmdSEMAPHOREXSTATUS(ctx *Context) error {
	if ctx.ArgCount() != 1 {
		return ctx.WriteError(ErrWrongArgCount)
	}
	current, maxPermits, err := loadSemX(ctx, ctx.ArgString(0))
	if err != nil {
		return ctx.WriteError(err)
	}
	return ctx.WriteArray([]*resp.Value{
		resp.BulkString("name"), resp.BulkString(ctx.ArgString(0)),
		resp.BulkString("current"), resp.IntegerValue(current),
		resp.BulkString("max"), resp.IntegerValue(maxPermits),
		resp.BulkString("available"), resp.IntegerValue(maxPermits - current),
	})
}
