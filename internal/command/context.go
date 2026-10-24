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

	// router is the router dispatching this command, recorded by
	// Router.Execute so EXEC runs its queue through the same handlers.
	router *Router
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
		_, ok := v.(*store.ListValue)
		return ok
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
		_, ok := v.(*store.SortedSetValue)
		return ok
	}))
}

// holdsType builds a readiness check that a key holds a value of the type the
// caller pops from. A key of another type is never ready, so a wrong-type key
// cannot turn the caller's retry loop into a busy spin.
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
	return ctx.Store.KeyNotifier().WaitForKeysCancel(keys, timeout, ctx.Done, ready)
}

// waitForStreamKeys is waitForKeys for streams. A stream that exists may hold
// no entry the reader wants, so existence is not readiness and only a write
// notification wakes the reader.
func (ctx *Context) waitForStreamKeys(keys []string, timeout time.Duration) (string, bool) {
	if ctx.InExec {
		return "", false
	}
	return ctx.Store.KeyNotifier().WaitForKeysCancel(keys, timeout, ctx.Done, nil)
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
