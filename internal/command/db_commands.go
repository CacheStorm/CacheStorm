package command

import (
	"errors"
	"strconv"
	"strings"

	"github.com/cachestorm/cachestorm/internal/store"
)

var (
	errSameObject        = errors.New("ERR source and destination objects are the same")
	errInvalidFirstDB    = errors.New("ERR invalid first DB index")
	errInvalidSecondDB   = errors.New("ERR invalid second DB index")
	errDBIndexOutOfRange = store.ErrDBIndexOutOfRange
)

// databases returns the databases this command can reach. A context built
// without them (tests, embedded use) gets a private set whose database 0 is
// its Store.
func (ctx *Context) databases() *store.Databases {
	if ctx.DBs == nil {
		ctx.DBs = store.NewDatabases(ctx.Store, store.DefaultDatabases)
	}
	return ctx.DBs
}

// selectDB makes database index the one the following commands run in.
func (ctx *Context) selectDB(index int) error {
	dbs := ctx.databases()
	if !dbs.Valid(index) {
		return errDBIndexOutOfRange
	}
	ctx.Store = dbs.DB(index)
	ctx.DB = index
	return nil
}

// parseDBIndex parses a database index argument the way Redis does: any
// integer parses, the range is checked separately.
func parseDBIndex(arg string) (int, bool) {
	n, err := strconv.ParseInt(arg, 10, 64)
	if err != nil || n < -1<<31 || n > 1<<31-1 {
		return 0, false
	}
	return int(n), true
}

func cmdSELECT(ctx *Context) error {
	if ctx.ArgCount() != 1 {
		return ctx.WriteError(ErrWrongArgCount)
	}
	index, ok := parseDBIndex(ctx.ArgString(0))
	if !ok {
		return ctx.WriteError(ErrNotInteger)
	}
	if err := ctx.selectDB(index); err != nil {
		return ctx.WriteError(err)
	}
	return ctx.WriteOK()
}

func cmdDBSIZE(ctx *Context) error {
	return ctx.WriteInteger(ctx.Store.KeyCount())
}

// flushArgsValid accepts the optional ASYNC or SYNC of FLUSHDB and FLUSHALL.
// Both flush synchronously.
func flushArgsValid(ctx *Context) bool {
	if ctx.ArgCount() == 0 {
		return true
	}
	if ctx.ArgCount() > 1 {
		return false
	}
	switch strings.ToUpper(ctx.ArgString(0)) {
	case "ASYNC", "SYNC":
		return true
	}
	return false
}

func cmdFLUSHDB(ctx *Context) error {
	if !flushArgsValid(ctx) {
		return ctx.WriteError(ErrSyntaxError)
	}
	ctx.Store.Flush()
	return ctx.WriteOK()
}

func cmdFLUSHALL(ctx *Context) error {
	if !flushArgsValid(ctx) {
		return ctx.WriteError(ErrSyntaxError)
	}
	ctx.databases().FlushAll()
	return ctx.WriteOK()
}

func cmdSWAPDB(ctx *Context) error {
	if ctx.ArgCount() != 2 {
		return ctx.WriteError(ErrWrongArgCount)
	}
	a, ok := parseDBIndex(ctx.ArgString(0))
	if !ok {
		return ctx.WriteError(errInvalidFirstDB)
	}
	b, ok := parseDBIndex(ctx.ArgString(1))
	if !ok {
		return ctx.WriteError(errInvalidSecondDB)
	}
	dbs := ctx.databases()
	if err := dbs.Swap(a, b); err != nil {
		return ctx.WriteError(err)
	}
	// The selected database now holds other data.
	ctx.Store = dbs.DB(ctx.DB)
	return ctx.WriteOK()
}

func cmdMOVE(ctx *Context) error {
	if ctx.ArgCount() != 2 {
		return ctx.WriteError(ErrWrongArgCount)
	}
	key := ctx.ArgString(0)
	index, ok := parseDBIndex(ctx.ArgString(1))
	if !ok {
		return ctx.WriteError(ErrNotInteger)
	}
	dbs := ctx.databases()
	if !dbs.Valid(index) {
		return ctx.WriteError(errDBIndexOutOfRange)
	}
	dst := dbs.DB(index)
	if dst == ctx.Store {
		return ctx.WriteError(errSameObject)
	}
	entry, exists := ctx.Store.Get(key)
	if !exists {
		return ctx.WriteInteger(0)
	}
	if dst.Exists(key) {
		return ctx.WriteInteger(0)
	}
	dst.SetEntry(key, entry)
	ctx.Store.Delete(key)
	dst.KeyNotifier().NotifyKey(key)
	return ctx.WriteInteger(1)
}

func cmdCOPY(ctx *Context) error {
	if ctx.ArgCount() < 2 {
		return ctx.WriteError(ErrWrongArgCount)
	}

	srcKey := ctx.ArgString(0)
	dstKey := ctx.ArgString(1)

	replace := false
	dst := ctx.Store

	for i := 2; i < ctx.ArgCount(); i++ {
		switch strings.ToUpper(ctx.ArgString(i)) {
		case "REPLACE":
			replace = true
		case "DB":
			if i+1 >= ctx.ArgCount() {
				return ctx.WriteError(ErrSyntaxError)
			}
			i++
			index, ok := parseDBIndex(ctx.ArgString(i))
			if !ok {
				return ctx.WriteError(ErrNotInteger)
			}
			dbs := ctx.databases()
			if !dbs.Valid(index) {
				return ctx.WriteError(errDBIndexOutOfRange)
			}
			dst = dbs.DB(index)
		default:
			return ctx.WriteError(ErrSyntaxError)
		}
	}

	if dst == ctx.Store && srcKey == dstKey {
		return ctx.WriteError(errSameObject)
	}

	entry, exists := ctx.Store.Get(srcKey)
	if !exists {
		return ctx.WriteInteger(0)
	}

	if !replace && dst.Exists(dstKey) {
		return ctx.WriteInteger(0)
	}

	newEntry := store.NewEntry(entry.Value.Clone())
	newEntry.ExpiresAt.Store(entry.ExpiresAt.Load())
	if len(entry.Tags) > 0 {
		newEntry.Tags = make([]string, len(entry.Tags))
		copy(newEntry.Tags, entry.Tags)
	}

	dst.SetEntry(dstKey, newEntry)
	dst.KeyNotifier().NotifyKey(dstKey)
	return ctx.WriteInteger(1)
}

// keyspaceInfo is the Keyspace section of INFO: one line per non-empty
// database.
func keyspaceInfo(ctx *Context) string {
	var sb strings.Builder
	sb.WriteString("# Keyspace\r\n")
	ctx.databases().ForEach(func(index int, s *store.Store) {
		keys := s.KeyCount()
		if keys == 0 {
			return
		}
		expires, avgTTL := s.ExpiryStats()
		sb.WriteString("db")
		sb.WriteString(strconv.Itoa(index))
		sb.WriteString(":keys=")
		sb.WriteString(strconv.FormatInt(keys, 10))
		sb.WriteString(",expires=")
		sb.WriteString(strconv.FormatInt(expires, 10))
		sb.WriteString(",avg_ttl=")
		sb.WriteString(strconv.FormatInt(avgTTL, 10))
		sb.WriteString(",subexpiry=0\r\n")
	})
	sb.WriteString("\r\n")
	return sb.String()
}

// isWriteCommand reports whether Redis flags cmd as a write.
func isWriteCommand(cmd string) bool {
	spec, ok := redisCommandSpecs[cmd]
	if !ok {
		return false
	}
	for _, f := range spec.Flags {
		if f == "write" {
			return true
		}
	}
	return false
}

// touchWrittenKeys bumps the watch version of the keys a write command
// named, so a WATCH on any of them aborts the watcher's EXEC. Handlers that
// modify a value in place never pass through Store.Set, so this is what makes
// every write visible to WATCH.
// signalWrittenKeys wakes clients blocked on the keys a write command named,
// as Redis' signalKeyAsReady does. The push commands notify on their own, but
// other writers that can fill a list or sorted set (LMOVE and RPOPLPUSH into
// a new destination, SORT ... STORE, ZUNIONSTORE, RESTORE, ...) did not, so a
// client blocked on the destination slept through the data. A write with no
// client blocked anywhere costs one atomic load.
func signalWrittenKeys(ctx *Context) {
	if ctx.Store == nil || !isWriteCommand(ctx.Command) {
		return
	}
	kn := ctx.Store.KeyNotifier()
	if !kn.HasWaiters() {
		return
	}
	for _, key := range aclKeysForCommand(ctx.Command, ctx.Args) {
		kn.NotifyKey(key)
	}
}

func touchWrittenKeys(ctx *Context) {
	if ctx.Store == nil || !ctx.Store.HasWatchedKeys() || !isWriteCommand(ctx.Command) {
		return
	}
	ctx.Store.TouchKeys(aclKeysForCommand(ctx.Command, ctx.Args))
}
