package command

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"math"
	"strconv"
	"sync"
	"time"

	"github.com/cachestorm/cachestorm/internal/resp"
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
	watchedKeys map[string]int64
}

type queuedCommand struct {
	cmd  string
	args [][]byte
}

func NewTransaction() *Transaction {
	return &Transaction{
		queued:      make([]queuedCommand, 0),
		active:      false,
		watchedKeys: make(map[string]int64),
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

func (t *Transaction) ClearWatch() {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.watchedKeys = make(map[string]int64)
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

func (t *Transaction) Watch(key string, version int64) {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.watchedKeys[key] = version
}

func (t *Transaction) CheckWatchedVersions(getVersion func(key string) int64) bool {
	t.mu.Lock()
	defer t.mu.Unlock()
	for key, version := range t.watchedKeys {
		currentVersion := getVersion(key)
		if currentVersion != version {
			return false
		}
	}
	return true
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
			return ctx.WriteNull()
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

	for _, qc := range queued {
		result := executeQueuedCommand(ctx, qc)
		results = append(results, result)
		if result.Type != resp.TypeError {
			executed = append(executed, ExecutedCommand{Name: qc.cmd, Args: qc.args})
		}
	}

	if r := ctx.dispatcher(); r != nil && len(executed) > 0 {
		if hook := r.postExecHook(); hook != nil {
			hook(executed)
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
		key := ctx.ArgString(i)
		version := ctx.Store.GetVersion(key)
		ctx.Transaction.Watch(key, version)
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
	r := ctx.dispatcher()
	if r == nil {
		return resp.ErrorValue(ErrUnknownCommand.Error())
	}
	def, ok := r.Get(qc.cmd)
	if !ok {
		return resp.ErrorValue("ERR unknown command '" + qc.cmd + "'")
	}

	var output bytes.Buffer
	sub := *ctx
	sub.Command = qc.cmd
	sub.Args = qc.args
	sub.Writer = resp.NewWriter(&output)
	sub.InExec = true
	sub.StartTime = time.Now()

	err := def.Handler(&sub)

	ctx.Authenticated = sub.Authenticated
	ctx.ACLUser = sub.ACLUser
	ctx.ClientName = sub.ClientName
	ctx.Subscriber = sub.Subscriber

	if err != nil {
		return resp.ErrorValue(err.Error())
	}
	if err := sub.Writer.Flush(); err != nil {
		return resp.ErrorValue(err.Error())
	}

	reader := resp.NewReader(&output)
	var values []*resp.Value
	for {
		v, err := reader.ReadValue()
		if err == io.EOF {
			break
		}
		if err != nil {
			return resp.ErrorValue("ERR " + err.Error())
		}
		values = append(values, v)
	}
	switch len(values) {
	case 0:
		return resp.NullBulkString()
	case 1:
		return values[0]
	default:
		return resp.ArrayValue(values)
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

func float64ToString(f float64) string {
	return fmt.Sprintf("%g", f)
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

func parseInt(data []byte) (int64, error) {
	return strconv.ParseInt(string(data), 10, 64)
}

func int64ToBytes(n int64) []byte {
	if n == 0 {
		return []byte("0")
	}

	var negative bool
	if n < 0 {
		negative = true
		n = -n
	}

	var buf [20]byte
	i := len(buf)

	for n > 0 {
		i--
		buf[i] = byte('0' + n%10)
		n /= 10
	}

	if negative {
		i--
		buf[i] = '-'
	}

	return buf[i:]
}
