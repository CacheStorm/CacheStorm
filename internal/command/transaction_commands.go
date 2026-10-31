package command

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"math"
	"strconv"
	"sync"
	"sync/atomic"
	"time"

	"github.com/cachestorm/cachestorm/internal/resp"
	"github.com/cachestorm/cachestorm/internal/store"
)

var (
	ErrExecWithoutMulti    = errors.New("ERR EXEC without MULTI")
	ErrDiscardWithoutMulti = errors.New("ERR DISCARD without MULTI")
	ErrWatchInMulti        = errors.New("ERR WATCH inside MULTI is not allowed")
	ErrMultiNested         = errors.New("ERR MULTI calls can not be nested")
	ErrExecAbort           = errors.New("EXECABORT Transaction discarded because of previous errors.") //nolint:staticcheck // Redis reply text, clients match it verbatim
)

type Transaction struct {
	mu          sync.Mutex
	queued      []queuedCommand
	active      bool
	aborted     bool
	watchedKeys map[watchedKey]int64
}

// watchedKey is a key in one database. A nil store is a key recorded through
// the legacy Watch, checked against the getVersion passed to
// CheckWatchedVersions.
type watchedKey struct {
	db  *store.Store
	key string
}

type queuedCommand struct {
	cmd  string
	args [][]byte
}

func NewTransaction() *Transaction {
	return &Transaction{
		queued:      make([]queuedCommand, 0),
		active:      false,
		watchedKeys: make(map[watchedKey]int64),
	}
}

func (t *Transaction) Queue(cmd string, args [][]byte) {
	t.mu.Lock()
	defer t.mu.Unlock()
	argsCopy := make([][]byte, len(args))
	for i, arg := range args {
		argsCopy[i] = bytes.Clone(arg)
	}
	t.queued = append(t.queued, queuedCommand{cmd: cmd, args: argsCopy})
}

func (t *Transaction) GetQueued() []queuedCommand {
	t.mu.Lock()
	defer t.mu.Unlock()
	result := make([]queuedCommand, len(t.queued))
	copy(result, t.queued)
	return result
}

func (t *Transaction) Clear() {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.queued = t.queued[:0]
	t.active = false
	t.aborted = false
}

// Abort marks the open transaction so EXEC discards it, the Redis behaviour
// after a command failed to queue.
func (t *Transaction) Abort() {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.aborted = true
}

func (t *Transaction) IsAborted() bool {
	t.mu.Lock()
	defer t.mu.Unlock()
	return t.aborted
}

// ClearWatch drops every watch, releasing the registrations held in the
// watched databases.
func (t *Transaction) ClearWatch() {
	t.mu.Lock()
	watched := t.watchedKeys
	t.watchedKeys = make(map[watchedKey]int64)
	t.mu.Unlock()
	if len(watched) > 0 {
		watchingClients.Add(-1)
	}
	for wk := range watched {
		if wk.db != nil {
			wk.db.UnwatchKey(wk.key)
		}
	}
}

func (t *Transaction) IsActive() bool {
	t.mu.Lock()
	defer t.mu.Unlock()
	return t.active
}

func (t *Transaction) Start() {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.active = true
	t.aborted = false
	t.queued = t.queued[:0]
}

// Watch records key at version without registering it with a database; it
// is checked against the getVersion given to CheckWatchedVersions.
func (t *Transaction) Watch(key string, version int64) {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.noteWatchLocked()
	t.watchedKeys[watchedKey{key: key}] = version
}

// watchingClients counts the transactions watching keys, INFO's
// watching_clients.
var watchingClients atomic.Int64

func (t *Transaction) noteWatchLocked() {
	if len(t.watchedKeys) == 0 {
		watchingClients.Add(1)
	}
}

// WatchIn watches key in database db: any modification of the key in that
// database, by any client, makes the following EXEC fail. Watching the same
// key twice keeps the first baseline.
func (t *Transaction) WatchIn(db *store.Store, key string) {
	wk := watchedKey{db: db, key: key}
	t.mu.Lock()
	defer t.mu.Unlock()
	if _, ok := t.watchedKeys[wk]; ok {
		return
	}
	t.noteWatchLocked()
	t.watchedKeys[wk] = db.WatchKey(key)
}

// CheckWatchedVersions reports whether no watched key changed since it was
// watched. Keys watched in a database are checked there; legacy keys use
// getVersion.
func (t *Transaction) CheckWatchedVersions(getVersion func(key string) int64) bool {
	t.mu.Lock()
	defer t.mu.Unlock()
	for wk, version := range t.watchedKeys {
		var current int64
		if wk.db != nil {
			current = wk.db.GetVersion(wk.key)
		} else if getVersion != nil {
			current = getVersion(wk.key)
		}
		if current != version {
			return false
		}
	}
	return true
}

// Footprint returns the number of watched keys and the bytes of the queued
// commands, for CLIENT LIST's watch and multi-mem.
func (t *Transaction) Footprint() (watched int, queuedBytes int64) {
	t.mu.Lock()
	defer t.mu.Unlock()
	for _, qc := range t.queued {
		queuedBytes += int64(len(qc.cmd))
		for _, a := range qc.args {
			queuedBytes += int64(len(a))
		}
	}
	return len(t.watchedKeys), queuedBytes
}

func (t *Transaction) HasWatchedKeys() bool {
	t.mu.Lock()
	defer t.mu.Unlock()
	return len(t.watchedKeys) > 0
}

func RegisterTransactionCommands(router *Router) {
	router.Register(&CommandDef{Name: "MULTI", Handler: cmdMULTI})
	router.Register(&CommandDef{Name: "EXEC", Handler: cmdEXEC})
	router.Register(&CommandDef{Name: "DISCARD", Handler: cmdDISCARD})
	router.Register(&CommandDef{Name: "WATCH", Handler: cmdWATCH})
	router.Register(&CommandDef{Name: "UNWATCH", Handler: cmdUNWATCH})
}

func cmdMULTI(ctx *Context) error {
	if ctx.Transaction.IsActive() {
		return ctx.WriteError(ErrMultiNested)
	}
	// Watches recorded before MULTI stay armed: EXEC's dirty check needs the
	// baseline taken at WATCH time (MULTI does not clear watches).
	ctx.Transaction.Start()
	return ctx.WriteOK()
}

func cmdEXEC(ctx *Context) error {
	if !ctx.Transaction.IsActive() {
		return ctx.WriteError(ErrExecWithoutMulti)
	}

	if ctx.Transaction.IsAborted() {
		ctx.Transaction.Clear()
		ctx.Transaction.ClearWatch()
		return ctx.WriteError(ErrExecAbort)
	}

	if ctx.Transaction.HasWatchedKeys() {
		if !ctx.Transaction.CheckWatchedVersions(ctx.Store.GetVersion) {
			ctx.Transaction.Clear()
			ctx.Transaction.ClearWatch()
			// A failed WATCH check is a null array in RESP2, null in RESP3.
			if ctx.Writer.IsRESP3() {
				return ctx.WriteNull()
			}
			return ctx.WriteNullArray()
		}
	}

	queued := ctx.Transaction.GetQueued()
	ctx.Transaction.Clear()
	ctx.Transaction.ClearWatch()

	if len(queued) == 0 {
		return ctx.WriteArray([]*resp.Value{})
	}

	results := make([]*resp.Value, 0, len(queued))
	executed := make([]ExecutedCommand, 0, len(queued))

	var wakes []func()
	ctx.wakeAfter = &wakes
	for _, qc := range queued {
		result, recorded := runQueuedCommand(ctx, qc)
		results = append(results, result)
		executed = append(executed, recorded...)
	}
	ctx.wakeAfter = nil
	for _, wake := range wakes {
		wake()
	}

	if r := ctx.dispatcher(); r != nil && len(executed) > 0 {
		if hook := r.postExecHook(); hook != nil {
			if err := hook(ctx, executed); err != nil {
				return err
			}
		}
	}

	return ctx.WriteArray(results)
}

func cmdDISCARD(ctx *Context) error {
	if !ctx.Transaction.IsActive() {
		return ctx.WriteError(ErrDiscardWithoutMulti)
	}

	ctx.Transaction.Clear()
	ctx.Transaction.ClearWatch()
	return ctx.WriteOK()
}

func cmdWATCH(ctx *Context) error {
	if ctx.Transaction.IsActive() {
		return ctx.WriteError(ErrWatchInMulti)
	}

	for i := 0; i < ctx.ArgCount(); i++ {
		ctx.Transaction.WatchIn(ctx.Store, ctx.ArgString(i))
	}

	return ctx.WriteOK()
}

func cmdUNWATCH(ctx *Context) error {
	ctx.Transaction.ClearWatch()
	return ctx.WriteOK()
}

// executeQueuedCommand runs one queued command through its registered handler
// and returns the reply it wrote. The handler is the one a standalone call
// uses, so a command behaves the same inside and outside a transaction.
func executeQueuedCommand(ctx *Context, qc queuedCommand) *resp.Value {
	v, _ := runQueuedCommand(ctx, qc)
	return v
}

// runQueuedCommand runs one queued command like executeQueuedCommand and also
// returns what it records in the AOF and the replication stream: nothing for
// an error reply, the handler's override when it set one, the command itself
// (when the router records it) otherwise.
func runQueuedCommand(ctx *Context, qc queuedCommand) (*resp.Value, []ExecutedCommand) {
	db := ctx.DB
	v, sub := execQueued(ctx, qc)
	if v.Type == resp.TypeError || sub == nil {
		return v, nil
	}
	if cmds, ok := sub.Propagation(); ok {
		return v, cmds
	}
	if r := ctx.dispatcher(); r != nil && !r.Persisted(qc.cmd, qc.args) {
		return v, nil
	}
	return v, []ExecutedCommand{{Name: qc.cmd, Args: qc.args, DB: db}}
}

func execQueued(ctx *Context, qc queuedCommand) (*resp.Value, *Context) {
	r := ctx.dispatcher()
	if r == nil {
		return resp.ErrorValue(ErrUnknownCommand.Error()), nil
	}
	def, ok := r.Get(qc.cmd)
	if !ok {
		return resp.ErrorValue("ERR unknown command '" + qc.cmd + "'"), nil
	}

	if ctx.timeSnapshot == 0 {
		ctx.timeSnapshot = time.Now().UnixMilli()
	}
	var output bytes.Buffer
	sub := *ctx
	sub.Command = qc.cmd
	sub.Args = qc.args
	sub.Writer = resp.NewWriter(&output)
	// EXEC replies in the client's protocol; scripts see RESP2 replies
	// unless they called redis.setresp(3).
	sub.Writer.SetRESP3(ctx.Command == "EXEC" && ctx.Writer.IsRESP3() || ctx.Command != "EXEC" && ctx.scriptRESP3)
	sub.InExec = true
	sub.InScript = ctx.Command != "EXEC"
	sub.StartTime = time.Now()
	sub.propagate = nil
	sub.propagateSet = false
	sub.writeEffectsHandled = false

	before := watchedKeysBefore(&sub)
	sub.waited = 0
	err := def.Handler(&sub)
	recordCall(qc.cmd, qc.args, time.Since(sub.StartTime)-sub.waited, err != nil || sub.Writer.Errors() > 0)
	if err != nil {
		countErrorReply(err.Error())
	}
	reaccountWrittenKeys(&sub)
	if ctx.wakeAfter != nil {
		// Blocked clients are served once the whole transaction ran, so none
		// of them observes it half applied.
		st, name, args := sub.Store, sub.Command, sub.Args
		*ctx.wakeAfter = append(*ctx.wakeAfter, func() {
			signalWrittenKeys(&Context{Store: st, Command: name, Args: args})
		})
	} else {
		signalWrittenKeys(&sub)
	}
	touchWrittenKeys(&sub, before, err != nil || sub.Writer.Errors() > 0)

	// SELECT inside the transaction switches the database for the commands
	// queued after it, and for the connection once EXEC returns.
	ctx.Store = sub.Store
	ctx.DB = sub.DB
	ctx.DBs = sub.DBs
	ctx.Authenticated = sub.Authenticated
	ctx.ACLUser = sub.ACLUser
	ctx.ClientName = sub.ClientName
	ctx.Subscriber = sub.Subscriber

	if err != nil {
		return resp.ErrorValue(err.Error()), &sub
	}
	if err := sub.Writer.Flush(); err != nil {
		return resp.ErrorValue(err.Error()), &sub
	}

	reader := resp.NewReader(&output)
	var values []*resp.Value
	for {
		v, err := reader.ReadValue()
		if err == io.EOF {
			break
		}
		if err != nil {
			return resp.ErrorValue("ERR " + err.Error()), &sub
		}
		values = append(values, v)
	}
	switch len(values) {
	case 0:
		return resp.NullBulkString(), &sub
	case 1:
		return values[0], &sub
	default:
		return resp.ArrayValue(values), &sub
	}
}

func parseFloat(data []byte) (float64, error) {
	var result float64
	var negative bool
	var decimal bool
	var decimalPlaces float64 = 1
	i := 0
	if len(data) > 0 && data[0] == '-' {
		negative = true
		i = 1
	}
	for ; i < len(data); i++ {
		if data[i] == '.' {
			if decimal {
				return 0, fmt.Errorf("not a float")
			}
			decimal = true
			continue
		}
		if data[i] < '0' || data[i] > '9' {
			return 0, fmt.Errorf("not a float")
		}
		result = result*10 + float64(data[i]-'0')
		if decimal {
			decimalPlaces *= 10
		}
	}
	if decimal {
		result /= decimalPlaces
	}
	if negative {
		result = -result
	}
	return result, nil
}

// computeIntIncr parses current as a base-10 int64 and applies incr with an
// overflow guard, mirroring Redis: non-integer values and boundary overflows
// are rejected instead of wrapping.
func computeIntIncr(current []byte, incr int64) (int64, error) {
	parsed, err := strconv.ParseInt(string(current), 10, 64)
	if err != nil {
		return 0, ErrNotInteger
	}
	if (incr > 0 && parsed > math.MaxInt64-incr) || (incr < 0 && parsed < math.MinInt64-incr) {
		return 0, ErrOverflow
	}
	return parsed + incr, nil
}
