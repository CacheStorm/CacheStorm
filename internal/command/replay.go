package command

import "github.com/cachestorm/cachestorm/internal/store"

func replayTimeSeries(manager *store.TimeSeriesManager, handler func(*Context, *store.TimeSeriesManager) error) func(*Context) error {
	return func(ctx *Context) error {
		errorsBefore := ctx.Writer.Errors()
		if err := handler(ctx, manager); err != nil {
			return err
		}
		if ctx.Writer.Errors() > errorsBefore {
			return nil
		}
		key := ctx.ArgString(0)
		value, ok := manager.Get(key)
		if !ok {
			return store.ErrKeyNotFound
		}
		if entry, exists := ctx.Store.Get(key); exists {
			if entry.Value != value {
				return store.ErrWrongType
			}
			ctx.Store.Reaccount(key)
			return nil
		}
		return ctx.Store.Set(key, value, store.SetOptions{})
	}
}
