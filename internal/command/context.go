package command

import (
	"errors"
	"sync/atomic"
	"time"

	"github.com/cachestorm/cachestorm/internal/acl"
	"github.com/cachestorm/cachestorm/internal/resp"
	"github.com/cachestorm/cachestorm/internal/store"
)

var (
	ErrUnknownCommand  = errors.New("ERR unknown command")
	ErrWrongArgCount   = errors.New("ERR wrong number of arguments")
	ErrInvalidArg      = errors.New("ERR invalid argument")
	ErrSyntaxError     = errors.New("ERR syntax error")
	ErrNotInteger      = errors.New("ERR value is not an integer or out of range")
	ErrNotFloat        = errors.New("ERR value is not a valid float")
	ErrOverflow        = errors.New("ERR increment or decrement would overflow")
	ErrFloatOverflow   = errors.New("ERR increment would produce NaN or Infinity")
	ErrIndexOutOfRange = errors.New("ERR index out of range")
)

var subscriberID atomic.Int64

type Context struct {
	Command       string
	Args          [][]byte
	Store         *store.Store
	Writer        *resp.Writer
	StartTime     time.Time
	Authenticated bool
	ClientID      int64
	Namespace     string
	Transaction   *Transaction
	Subscriber    *store.Subscriber
	Username      string
	RemoteAddr    string
	// ACLUser is the ACL user this command runs as, resolved from AUTH by the
	// connection. nil means the connection is running as the permissive default
	// user, which is how Redis treats unauthenticated clients.
	ACLUser *acl.User

	// ClientName is set by CLIENT SETNAME. A fresh Context is built per command,
	// so the connection carries it back (see internal/server/connection.go).
	ClientName string

	// InExec is set while EXEC runs a queued command. A blocking command
	// must not wait inside a transaction: Redis answers it as if the timeout
	// had already expired.
	InExec bool

	// Done, when non-nil, is closed once the client is known to have gone
	// away, so a blocking command can stop waiting for it.
	Done <-chan struct{}

	// passKey and passReady record the key a blocking pop was woken for,
	// so the wakeup can be passed to the next waiter (see passWakeup).
	passKey   string
	passReady func(string) bool
	passOwner bool

	// router is the router dispatching this command, recorded by
	// Router.Execute so EXEC runs its queue through the same handlers.
	router *Router

	// DB is the index of the selected database, the one Store is.
	DB int
	// DBs holds every database, so SELECT, MOVE, COPY ... DB, SWAPDB and
	// FLUSHALL can reach the others. nil means Store is the only database.
	DBs *store.Databases
}

func NewContext(cmd string, args [][]byte, s *store.Store, w *resp.Writer) *Context {
	return &Context{
		Command:     cmd,
		Args:        args,
		Store:       s,
		Writer:      w,
		StartTime:   time.Now(),
		Transaction: NewTransaction(),
	}
}

func NewContextWithClient(cmd string, args [][]byte, s *store.Store, w *resp.Writer, clientID int64, remoteAddr string) *Context {
	return &Context{
		Command:     cmd,
		Args:        args,
		Store:       s,
		Writer:      w,
		StartTime:   time.Now(),
		Transaction: NewTransaction(),
		ClientID:    clientID,
		RemoteAddr:  remoteAddr,
	}
}

// dispatcher returns the router running this command, falling back to the
// process router for contexts built outside Router.Execute.
func (ctx *Context) dispatcher() *Router {
	if ctx.router != nil {
		return ctx.router
	}
	return globalRouter
}

// waitForKeys blocks until one of the list keys is written to, the timeout
// expires or the client disconnects. Inside EXEC it reports an immediate
// timeout. A key that already holds a list counts as notified, which closes
// the window between the caller's failed pop and its registration as a waiter.
func (ctx *Context) waitForKeys(keys []string, timeout time.Duration) (string, bool) {
	return ctx.waitForKeysReady(keys, timeout, ctx.holdsType(func(v store.Value) bool {
		l, ok := v.(*store.ListValue)
		if !ok {
			return false
		}
		l.RLock()
		defer l.RUnlock()
		return len(l.Elements) > 0
	}))
}

// waitForKey is the single-key form of waitForKeys.
func (ctx *Context) waitForKey(key string, timeout time.Duration) bool {
	_, ok := ctx.waitForKeys([]string{key}, timeout)
	return ok
}

// waitForZSetKeys is waitForKeys for sorted-set keys.
func (ctx *Context) waitForZSetKeys(keys []string, timeout time.Duration) (string, bool) {
	return ctx.waitForKeysReady(keys, timeout, ctx.holdsType(func(v store.Value) bool {
		z, ok := v.(*store.SortedSetValue)
		if !ok {
			return false
		}
		z.RLock()
		defer z.RUnlock()
		return len(z.Members) > 0
	}))
}

// holdsType builds a readiness check that a key holds data the caller can
// pop: a value of the right type that is not empty. A key of another type, or
// a collection created empty a moment before its first element is added, is
// never ready, so neither can turn the caller's retry loop into a busy spin.
func (ctx *Context) holdsType(match func(store.Value) bool) func(string) bool {
	return func(key string) bool {
		entry, ok := ctx.Store.Get(key)
		return ok && match(entry.Value)
	}
}

func (ctx *Context) waitForKeysReady(keys []string, timeout time.Duration, ready func(string) bool) (string, bool) {
	if ctx.InExec {
		return "", false
	}
	var notified bool
	key, ok := ctx.Store.KeyNotifier().WaitForKeysOpts(keys, timeout, store.WaitOptions{
		Cancel:   ctx.Done,
		Ready:    ready,
		Consume:  true,
		Notified: &notified,
		// A command that waits again after a wakeup found nothing to take
		// keeps its place at the head of the queue.
		Front: ctx.passKey != "",
	})
	if ok {
		// Only the oldest blocked consumer is woken by a write; once this
		// command has taken its share, Router.Execute hands the key on to
		// the next one if data is left.
		ctx.passKey, ctx.passReady = key, ready
		ctx.passOwner = ctx.passOwner || notified
	}
	return key, ok
}

// waitForStreamKeys blocks like waitForKeysReady but as an observer: a stream
// entry is not consumed by reading it, so every blocked reader is woken by
// a write instead of only the oldest one.
func (ctx *Context) waitForStreamKeys(keys []string, timeout time.Duration, ready func(string) bool) (string, bool) {
	if ctx.InExec {
		return "", false
	}
	return ctx.Store.KeyNotifier().WaitForKeysOpts(keys, timeout, store.WaitOptions{
		Cancel: ctx.Done,
		Ready:  ready,
	})
}

// passWakeup hands a consumed key's wakeup to the next blocked client.
func (ctx *Context) passWakeup() {
	if ctx.passKey == "" {
		return
	}
	ctx.Store.KeyNotifier().Pass(ctx.passKey, ctx.passReady, ctx.passOwner)
	ctx.passKey, ctx.passReady, ctx.passOwner = "", nil, false
}

func (ctx *Context) IsAuthenticated() bool {
	return ctx.Authenticated
}

func (ctx *Context) SetAuthenticated(auth bool) {
	ctx.Authenticated = auth
}

func (ctx *Context) GetTransaction() *Transaction {
	if ctx.Transaction == nil {
		ctx.Transaction = NewTransaction()
	}
	return ctx.Transaction
}

func (ctx *Context) GetSubscriber() *store.Subscriber {
	if ctx.Subscriber == nil {
		ctx.Subscriber = store.NewSubscriber(subscriberID.Add(1))
	}
	return ctx.Subscriber
}

func (ctx *Context) Arg(n int) []byte {
	if n < 0 || n >= len(ctx.Args) {
		return nil
	}
	return ctx.Args[n]
}

func (ctx *Context) ArgString(n int) string {
	return string(ctx.Arg(n))
}

func (ctx *Context) ArgCount() int {
	return len(ctx.Args)
}

func (ctx *Context) WriteOK() error {
	return ctx.Writer.WriteOK()
}

func (ctx *Context) WriteError(err error) error {
	return ctx.Writer.WriteError(err.Error())
}

func (ctx *Context) WriteValue(v *resp.Value) error {
	return ctx.Writer.WriteValue(v)
}

func (ctx *Context) WriteSimpleString(s string) error {
	return ctx.Writer.WriteSimpleString(s)
}

func (ctx *Context) WriteBulkString(s string) error {
	return ctx.Writer.WriteBulkString(s)
}

func (ctx *Context) WriteBulkBytes(b []byte) error {
	return ctx.Writer.WriteBulkBytes(b)
}

func (ctx *Context) WriteInteger(n int64) error {
	return ctx.Writer.WriteInteger(n)
}

func (ctx *Context) WriteNull() error {
	return ctx.Writer.WriteNull()
}

func (ctx *Context) WriteNullArray() error {
	return ctx.Writer.WriteNullArray()
}

func (ctx *Context) WriteNullBulkString() error {
	return ctx.Writer.WriteNullBulkString()
}

func (ctx *Context) WriteArray(items []*resp.Value) error {
	return ctx.Writer.WriteArray(items)
}

// Safe map accessors to prevent panics from type assertions
func mapInt64(m map[string]interface{}, key string) int64 {
	v, _ := m[key].(int64)
	return v
}

func mapInt(m map[string]interface{}, key string) int {
	v, _ := m[key].(int)
	return v
}

func mapString(m map[string]interface{}, key string) string {
	v, _ := m[key].(string)
	return v
}

func mapUint(m map[string]interface{}, key string) uint {
	v, _ := m[key].(uint)
	return v
}
