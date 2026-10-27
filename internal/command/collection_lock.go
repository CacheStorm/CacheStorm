package command

import (
	"github.com/cachestorm/cachestorm/internal/store"
)

// lockableCollection is a list, hash, set or sorted set: a value removed from
// the keyspace by Store.DeleteIfEmpty when emptied, which marks it dead.
type lockableCollection interface {
	*store.ListValue | *store.HashValue | *store.SetValue | *store.SortedSetValue
	store.Value
	Lock()
	Unlock()
	IsDead() bool
}

// lockOrCreate returns the collection at key write-locked, installing
// create() when the key is absent, and reports whether it was created. A
// value of another type is WRONGTYPE. The lookup and the creation are atomic
// (Store.GetOrCreate), and a value found dead — emptied and deleted after it
// was looked up — is retried, so a writer never adds to a value that is no
// longer stored. The caller unlocks the value and, when it may have left it
// empty, calls Store.DeleteIfEmpty.
func lockOrCreate[T lockableCollection](ctx *Context, key string, create func() T) (T, bool, error) {
	for {
		v, created, err := ctx.Store.GetOrCreate(key, func() store.Value { return create() })
		if err != nil {
			var zero T
			return zero, false, err
		}
		t, ok := v.(T)
		if !ok {
			var zero T
			return zero, false, store.ErrWrongType
		}
		t.Lock()
		if !t.IsDead() {
			return t, created, nil
		}
		t.Unlock()
	}
}

// lockExisting returns the live collection at key write-locked, or the zero
// value (nil) when the key is missing. A value of another type is WRONGTYPE.
// A value found dead is looked up again: it was deleted after the lookup, so
// the key may by now hold a new value.
func lockExisting[T lockableCollection](ctx *Context, key string) (T, error) {
	var zero T
	for {
		entry, exists := ctx.Store.Get(key)
		if !exists {
			return zero, nil
		}
		t, ok := entry.Value.(T)
		if !ok {
			return zero, store.ErrWrongType
		}
		t.Lock()
		if !t.IsDead() {
			return t, nil
		}
		t.Unlock()
	}
}

func newList() *store.ListValue { return &store.ListValue{Elements: make([][]byte, 0)} }
func newSortedSet() *store.SortedSetValue {
	return &store.SortedSetValue{Members: make(map[string]float64)}
}

// lockOrCreateList returns the list at key write-locked, creating it empty.
func lockOrCreateList(ctx *Context, key string) (*store.ListValue, error) {
	l, _, err := lockOrCreate(ctx, key, newList)
	return l, err
}

// lockOrCreateSortedSet returns the sorted set at key write-locked, creating
// it empty.
func lockOrCreateSortedSet(ctx *Context, key string) (*store.SortedSetValue, error) {
	z, _, err := lockOrCreate(ctx, key, newSortedSet)
	return z, err
}

// deleteIfEmpty removes key when it still holds the emptied value v. It is
// called after v's lock is released.
func deleteIfEmpty(ctx *Context, key string, v store.Value) {
	if ctx.Store.DeleteIfEmpty(key, v) {
		ctx.notify(store.NotifyGeneric, "del", key)
	}
}

// lockOrInstall is lockOrCreate for a writer whose whole effect on a missing
// key is to create it holding filled(): when it installs that value it
// reports true and returns it unlocked, the write already done. Otherwise the
// existing live value is returned write-locked.
func lockOrInstall[T lockableCollection](ctx *Context, key string, filled func() T) (T, bool, error) {
	for {
		v, created, err := ctx.Store.GetOrCreate(key, func() store.Value { return filled() })
		if err != nil {
			var zero T
			return zero, false, err
		}
		t, ok := v.(T)
		if !ok {
			var zero T
			return zero, false, store.ErrWrongType
		}
		if created {
			return t, true, nil
		}
		t.Lock()
		if !t.IsDead() {
			return t, false, nil
		}
		t.Unlock()
	}
}

func lockList(ctx *Context, key string) (*store.ListValue, error) {
	return lockExisting[*store.ListValue](ctx, key)
}

func lockHash(ctx *Context, key string) (*store.HashValue, error) {
	return lockExisting[*store.HashValue](ctx, key)
}

func lockSet(ctx *Context, key string) (*store.SetValue, error) {
	return lockExisting[*store.SetValue](ctx, key)
}

func lockSortedSet(ctx *Context, key string) (*store.SortedSetValue, error) {
	return lockExisting[*store.SortedSetValue](ctx, key)
}
