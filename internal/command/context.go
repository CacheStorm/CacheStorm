package command

import (
	"errors"
	"net"
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
	// ClientLibName and ClientLibVer are set by CLIENT SETINFO and, like
	// ClientName, carried back to the connection after the command.
	ClientLibName string
	ClientLibVer  string
	// ReplyMode is set by CLIENT REPLY: "on", "off" or "skip"; empty when
	// the command did not change it.
	ReplyMode string
	// Monitor is set by MONITOR: the connection then receives every command.
	Monitor bool
	// CloseConnection asks the connection to close once the command ran,
	// without a reply of its own (SHUTDOWN).
	CloseConnection bool
	// ResetConnection is set by RESET: the connection drops its tracking,
	// MONITOR, cluster flags and CLIENT REPLY mode.
	ResetConnection bool
	// InScript is set for the commands a script runs (InExec is also set).
	InScript bool

	FromMaster   bool
	Replaying    bool
	LegacyReplay bool

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
	passToken store.WakeToken

	// router is the router dispatching this command, recorded by
	// Router.Execute so EXEC runs its queue through the same handlers.
	router *Router

	// BeforeWait and AfterWait, when set, bracket the time a blocking
	// command waits for data, so the connection can release the write
	// ordering locks it holds while nothing is being written.
	BeforeWait func()
	AfterWait  func()

	// ReplicaReadOnly is set when the command runs on a read-only replica:
	// a script may run, but every write it attempts is refused.
	ReplicaReadOnly bool

	// ReplConf holds what REPLCONF told this connection; it belongs to the
	// connection and outlives the per-command Context.
	ReplConf *ReplicaConf

	// Handoff is set by PSYNC/SYNC: once the command returns, the connection
	// hands its socket to it and passes every later command to the handler it
	// returns instead of the router.
	Handoff func(conn net.Conn) ConnHandler

	// propagate replaces what the command records in the AOF and the
	// replication stream when propagateSet is true; an empty list records
	// nothing.
	propagate           []ExecutedCommand
	propagateSet        bool
	writeEffectsHandled bool

	// wakeAfter, while EXEC runs its queue, collects the wakeups of blocked
	// clients its commands trigger, delivered once the queue has run.
	wakeAfter *[]func()

	// DB is the index of the selected database, the one Store is.
	DB int
	// DBs holds every database, so SELECT, MOVE, COPY ... DB, SWAPDB and
	// FLUSHALL can reach the others. nil means Store is the only database.
	DBs *store.Databases

	// ClusterConn is the connection's cluster state (READONLY, ASKING and
	// the routing of the running command); nil without a connection.
	ClusterConn *ClusterConn

	// keys caches CommandKeys for keysCmd and keysArgs, which the gate, the
	// memory accounting and client tracking each need once per command.
	keys     []string
	keysCmd  string
	keysArgs [][]byte
	keysSet  bool
}

// CommandKeys returns the keys the command names, computed once per command
// and arguments.
func (ctx *Context) CommandKeys() []string {
	if ctx.keysSet && ctx.keysCmd == ctx.Command && sameArgs(ctx.keysArgs, ctx.Args) {
		return ctx.keys
	}
	ctx.keys = CommandKeys(ctx.Command, ctx.Args)
	ctx.keysCmd, ctx.keysArgs, ctx.keysSet = ctx.Command, ctx.Args, true
	return ctx.keys
}

// sameArgs reports whether a and b are the same argument slice.
func sameArgs(a, b [][]byte) bool {
	return len(a) == len(b) && (len(a) == 0 || &a[0] == &b[0])
}

// ReplicaConf is the replica information a connection announced with
// REPLCONF before PSYNC.
type ReplicaConf struct {
	ListeningPort int
	IP            string
	Capa          []string
}

// ConnHandler serves a connection that was handed off by a command.
type ConnHandler interface {
	// Command handles one command read from the connection; it writes any
	// reply itself.
	Command(cmd string, args [][]byte)
	// Closed is called once the connection is closed.
	Closed()
}

// PropagateAs makes the command record cmds, instead of itself, in the AOF
// and the replication stream. With no cmds nothing is recorded.
func (ctx *Context) PropagateAs(cmds ...ExecutedCommand) {
	ctx.propagate = append(ctx.propagate[:0:0], cmds...)
	ctx.propagateSet = true
}

// Propagation returns what the command records, and whether it overrode its
// own name and arguments.
func (ctx *Context) Propagation() ([]ExecutedCommand, bool) {
	return ctx.propagate, ctx.propagateSet
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

// NewConnectionContext builds the context of one command on a connection
// that keeps its own transaction state, so none is allocated per command.
func NewConnectionContext(cmd string, args [][]byte, s *store.Store, w *resp.Writer, clientID int64, remoteAddr string, tx *Transaction) *Context {
	return &Context{
		Command:     cmd,
		Args:        args,
		Store:       s,
		Writer:      w,
		StartTime:   time.Now(),
		Transaction: tx,
		ClientID:    clientID,
		RemoteAddr:  remoteAddr,
	}
}

// userName is the ACL user the connection runs as.
func (ctx *Context) userName() string {
	if ctx.ACLUser != nil {
		return ctx.ACLUser.Name
	}
	if ctx.Username != "" {
		return ctx.Username
	}
	return "default"
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
	if ctx.BeforeWait != nil {
		ctx.BeforeWait()
		defer ctx.AfterWait()
	}
	key, ok := ctx.Store.KeyNotifier().WaitForKeysOpts(keys, timeout, store.WaitOptions{
		Token:   &ctx.passToken,
		Cancel:  ctx.Done,
		Ready:   ready,
		Consume: true,
		// A command that waits again after a wakeup found nothing to take
		// keeps its place at the head of the queue.
		Front: ctx.passKey != "",
	})
	if ok {
		// Only the oldest blocked consumer is woken by a write; once this
		// command has taken its share, Router.Execute hands the key on to
		// the next one if data is left.
		ctx.passKey, ctx.passReady = key, ready
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
	if ctx.BeforeWait != nil {
		ctx.BeforeWait()
		defer ctx.AfterWait()
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
	ctx.Store.KeyNotifier().PassToken(ctx.passKey, ctx.passReady, ctx.passToken)
	ctx.passToken = store.WakeToken{}
	ctx.passKey, ctx.passReady = "", nil
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
