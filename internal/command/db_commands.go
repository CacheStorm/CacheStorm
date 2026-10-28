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
	if index != 0 && ctx.clusterRefuses() {
		return ctx.WriteError(errors.New("ERR SELECT is not allowed in cluster mode"))
	}
	if err := ctx.selectDB(index); err != nil {
		return ctx.WriteError(err)
	}
	return ctx.WriteOK()
}

// clusterRefuses reports whether cluster mode refuses a client's use of a
// database other than 0. Replayed and replicated commands are not refused.
func (ctx *Context) clusterRefuses() bool {
	return !ctx.Replaying && !ctx.LegacyReplay && !ctx.FromMaster && ctx.clusterState() != nil
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
	if ctx.clusterRefuses() {
		return ctx.WriteError(errors.New("ERR SWAPDB is not allowed in cluster mode"))
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
	ctx.writeEffectsHandled = true
	if ctx.ArgCount() != 2 {
		return ctx.WriteError(ErrWrongArgCount)
	}
	if ctx.clusterRefuses() {
		return ctx.WriteError(errors.New("ERR MOVE is not allowed in cluster mode"))
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
	changed, err := ctx.Store.Transfer(key, dst, key, store.TransferOptions{})
	if err == store.ErrKeyNotFound {
		ctx.PropagateAs()
		return ctx.WriteInteger(0)
	}
	if err != nil {
		return ctx.WriteError(err)
	}
	if !changed {
		ctx.PropagateAs()
		return ctx.WriteInteger(0)
	}
	ctx.notify(store.NotifyGeneric, "move_from", key)
	dst.Notify(store.NotifyGeneric, "move_to", key)
	return ctx.WriteInteger(1)
}

func cmdCOPY(ctx *Context) error {
	ctx.writeEffectsHandled = true
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
			if index != 0 && ctx.clusterRefuses() {
				return ctx.WriteError(errDBIndexOutOfRange)
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

	if (ctx.Replaying || ctx.LegacyReplay) && len(dstKey) > store.MaxKeySize {
		ctx.PropagateAs()
		return ctx.WriteInteger(1)
	}
	changed, err := ctx.Store.Transfer(srcKey, dst, dstKey, store.TransferOptions{Copy: true, Replace: replace})
	if err == store.ErrKeyNotFound {
		ctx.PropagateAs()
		return ctx.WriteInteger(0)
	}
	if err != nil {
		return ctx.WriteError(err)
	}
	if !changed {
		ctx.PropagateAs()
		return ctx.WriteInteger(0)
	}
	dst.Notify(store.NotifyGeneric, "copy_to", dstKey)
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

// notifyKeyMisses publishes the keymiss event for every key a read-only
// command names that does not exist, as Redis' lookupKeyRead does.
func notifyKeyMisses(ctx *Context) {
	spec, ok := redisCommandSpecs[ctx.Command]
	if !ok || !hasFlag(spec.Flags, "readonly") {
		return
	}
	for _, key := range ctx.CommandKeys() {
		if !ctx.Store.Exists(key) {
			ctx.notify(store.NotifyKeyMiss, "keymiss", key)
		}
	}
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

// signalWrittenKeys wakes clients blocked on the keys a write command named,
// as Redis' signalKeyAsReady does. The push commands notify on their own, but
// other writers that can fill a list or sorted set (LMOVE and RPOPLPUSH into
// a new destination, SORT ... STORE, ZUNIONSTORE, RESTORE, ...) did not, so a
// client blocked on the destination slept through the data. A write with no
// client blocked anywhere costs one atomic load.
func signalWrittenKeys(ctx *Context) {
	if ctx.writeEffectsHandled || ctx.Store == nil || !isWriteCommand(ctx.Command) {
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

// touchWrittenKeys bumps the watch version of the keys a write command
// writes, so a WATCH on any of them aborts the watcher's EXEC. Handlers that
// modify a value in place never pass through Store.Set, so this is what makes
// every write visible to WATCH.
//
// Like Redis' signalModifiedKey it leaves the keys alone when the command
// changed nothing: it failed, declared that it propagates nothing, or found
// a key absent and left it so (before records which keys existed).
func touchWrittenKeys(ctx *Context, before []bool, failed bool) {
	if failed || ctx.writeEffectsHandled || ctx.Store == nil || !ctx.Store.HasWatchedKeys() || !isWriteCommand(ctx.Command) || streamGroupWrites[ctx.Command] {
		return
	}
	if cmds, ok := ctx.Propagation(); ok && len(cmds) == 0 {
		return
	}
	keys := writtenKeys(ctx.Command, ctx.Args)
	if len(before) == len(keys) {
		touched := keys[:0:0]
		for i, k := range keys {
			if before[i] || ctx.Store.Exists(k) {
				touched = append(touched, k)
			}
		}
		keys = touched
	}
	ctx.Store.TouchKeys(keys)
}

// writtenKeys returns the keys of a write command that it may modify: the
// keys its Redis key specs flag RW, OW or RM, leaving out the ones it only
// reads (the sources of BITOP, SINTERSTORE, ...). A key with no flags, or a
// command Redis has no key specs for, counts as written.
func writtenKeys(cmd string, args [][]byte) []string {
	argv := make([]string, 0, len(args)+1)
	argv = append(argv, cmd)
	for _, a := range args {
		argv = append(argv, string(a))
	}
	meta := lookupRedisCommandMeta(argv)
	if meta == nil || len(meta.keySpecs) == 0 {
		return aclKeysForCommand(cmd, args)
	}
	refs, ok := keysFromSpecs(meta, argv)
	if !ok {
		refs = fallbackCommandKeys(meta, argv)
	}
	if len(refs) == 0 {
		return aclKeysForCommand(cmd, args)
	}
	keys := make([]string, 0, len(refs))
	for _, k := range refs {
		if len(k.flags) == 0 || slicesContainsAny(k.flags, "RW", "OW", "RM") {
			keys = append(keys, k.name)
		}
	}
	return keys
}

func slicesContainsAny(flags []string, want ...string) bool {
	for _, f := range flags {
		for _, w := range want {
			if f == w {
				return true
			}
		}
	}
	return false
}

// streamGroupWrites change a stream's consumer groups or ID counters, not its
// entries; Redis does not count them as modifying the key for WATCH.
var streamGroupWrites = map[string]bool{
	"XREADGROUP": true, "XCLAIM": true, "XAUTOCLAIM": true, "XACK": true, "XSETID": true,
}

// watchedKeysBefore records, while a key is watched, which keys a write
// command names exist before it runs (see touchWrittenKeys).
func watchedKeysBefore(ctx *Context) []bool {
	if ctx.Store == nil || !ctx.Store.HasWatchedKeys() || !isWriteCommand(ctx.Command) {
		return nil
	}
	keys := writtenKeys(ctx.Command, ctx.Args)
	existed := make([]bool, len(keys))
	for i, k := range keys {
		existed[i] = ctx.Store.Exists(k)
	}
	return existed
}

// IsWriteCommand reports whether Redis flags cmd as a write command.
func IsWriteCommand(cmd string) bool {
	return isWriteCommand(CanonicalName(cmd))
}
