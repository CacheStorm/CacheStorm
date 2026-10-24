package command

import (
	"crypto/sha256"
	"crypto/subtle"
	"encoding/hex"
	"errors"
	"io"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/cachestorm/cachestorm/internal/acl"
	"github.com/cachestorm/cachestorm/internal/logger"
	"github.com/cachestorm/cachestorm/internal/resp"
)

type CommandDef struct { //nolint:revive // Command prefix is intentional for clarity
	Name    string
	Handler func(ctx *Context) error
	Arity   int
	Flags   []string
}

type Router struct {
	mu       sync.RWMutex
	commands map[string]*CommandDef
	// duplicates records command names registered more than once. Register is
	// last-write-wins, so a second registration silently replaced the first;
	// six names were registered in two files at once and nothing said so.
	duplicates  map[string]struct{}
	requirePass string
	postExecute func(db int, cmd string, args [][]byte)
	// postExec receives the commands an EXEC ran, so the transaction is
	// persisted as one unit instead of command by command.
	postExec func(cmds []ExecutedCommand)
	// persisted reports whether a command is recorded in the AOF and the
	// replication stream; script effects are filtered through it.
	persisted func(cmd string, args [][]byte) bool
	// port is the TCP port the server listens on, for INFO.
	port int
	// replInfo renders the INFO replication section.
	replInfo func() string
}

// commandSpec is the arity, flags and key positions Redis reports for one of
// its commands (see redis_command_specs.go).
type commandSpec struct {
	Arity    int
	Flags    []string
	FirstKey int
	LastKey  int
	KeyStep  int
}

// arityError is the Redis reply for a call with the wrong number of
// arguments.
func arityError(cmd string) error {
	return errors.New("ERR wrong number of arguments for '" + strings.ToLower(cmd) + "' command")
}

// checkArity reports whether args fits the Redis arity of cmd. The arity
// counts the command name; a negative arity is a minimum. Commands Redis does
// not have are not checked here.
func checkArity(cmd string, args int) bool {
	spec, ok := redisCommandSpecs[cmd]
	if !ok {
		return true
	}
	n := args + 1
	if spec.Arity >= 0 {
		return n == spec.Arity
	}
	return n >= -spec.Arity
}

// ExecutedCommand is one command an EXEC ran without an error reply, with
// the database it ran in.
type ExecutedCommand struct {
	Name string
	Args [][]byte
	DB   int
}

// globalRouter is set during NewRouter() for access from command handlers (e.g. AUTH)
var globalRouter *Router

func NewRouter() *Router {
	r := &Router{
		commands:   make(map[string]*CommandDef),
		duplicates: make(map[string]struct{}),
	}
	globalRouter = r
	return r
}

// DuplicateRegistrations returns the command names that were registered more
// than once on this router. Production wiring should return an empty set.
func (r *Router) DuplicateRegistrations() []string {
	r.mu.RLock()
	defer r.mu.RUnlock()

	names := make([]string, 0, len(r.duplicates))
	for name := range r.duplicates {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}

func (r *Router) SetRequirePass(password string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.requirePass = password
}

func (r *Router) RequirePass() string {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.requirePass
}

func (r *Router) ValidatePassword(password string) bool {
	r.mu.RLock()
	rp := r.requirePass
	r.mu.RUnlock()

	if rp == "" {
		return true
	}

	// Constant-time comparison to prevent timing attacks
	expected := sha256.Sum256([]byte(rp))
	actual := sha256.Sum256([]byte(password))
	return subtle.ConstantTimeCompare(expected[:], actual[:]) == 1
}

// PasswordHash returns the SHA256 hash of the configured password (for ACL integration)
func (r *Router) PasswordHash() string {
	r.mu.RLock()
	rp := r.requirePass
	r.mu.RUnlock()
	if rp == "" {
		return ""
	}
	hash := sha256.Sum256([]byte(rp))
	return hex.EncodeToString(hash[:])
}

// Register adds a command to the router.
//
// Registration is REFUSED for a name that is already registered: the first
// handler wins and the later one is discarded. Register used to be plain
// last-write-wins with no diagnostic, and six command names ended up registered
// in two files at once. For REPLICAOF/SLAVEOF the two handlers were not
// equivalent — the stub cmdSLAVEOF discards both arguments and replies +OK — so
// which one answered depended entirely on the order of the Register*Commands
// calls in internal/server/server.go. Keeping the first handler removes that
// dependency: a later duplicate can no longer shadow an earlier one.
//
// A refused duplicate is still recorded (see DuplicateRegistrations) and logged,
// so the collision is reported rather than merely prevented. Logging happens
// after the lock is released so a log write never blocks dispatch.
func (r *Router) Register(def *CommandDef) {
	def.Name = CanonicalName(def.Name)
	r.mu.Lock()
	if _, exists := r.commands[def.Name]; exists {
		if r.duplicates == nil {
			r.duplicates = make(map[string]struct{})
		}
		r.duplicates[def.Name] = struct{}{}
		r.mu.Unlock()

		logger.Warn().
			Str("command", def.Name).
			Msg("duplicate command registration refused: the first handler is kept; " +
				"each command name must have exactly one registration site")
		return
	}
	r.commands[def.Name] = def
	r.mu.Unlock()
}

func (r *Router) Get(name string) (*CommandDef, bool) {
	name = CanonicalName(name)
	r.mu.RLock()
	defer r.mu.RUnlock()
	cmd, ok := r.commands[name]
	return cmd, ok
}

// CanonicalName upper-cases an ASCII command name. Redis command names are
// case-insensitive and many clients send them in lower case. A name that is
// already upper case is returned as is, without allocating.
func CanonicalName(name string) string {
	for i := 0; i < len(name); i++ {
		if c := name[i]; c >= 'a' && c <= 'z' {
			return strings.ToUpper(name)
		}
	}
	return name
}

// Commands that are allowed before authentication: the ones Redis flags
// no_auth. PING and COMMAND used to be exempt as well, so a client with no
// password could probe a requirepass server.
var noAuthCommands = map[string]bool{
	"AUTH":  true,
	"HELLO": true,
	"QUIT":  true,
	"RESET": true,
}

func (r *Router) SetPostExecute(fn func(cmd string, args [][]byte)) {
	if fn == nil {
		r.SetPostExecuteDB(nil)
		return
	}
	r.SetPostExecuteDB(func(_ int, cmd string, args [][]byte) { fn(cmd, args) })
}

// SetPostExecuteDB installs the per-command hook with the index of the
// database the command ran in, which the AOF needs to record SELECTs.
func (r *Router) SetPostExecuteDB(fn func(db int, cmd string, args [][]byte)) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.postExecute = fn
}

// SetPostExec installs the hook that receives the commands a successful EXEC
// ran. The router's per-command hook never sees them, so without this hook a
// transaction's writes are not persisted.
func (r *Router) SetPostExec(fn func(cmds []ExecutedCommand)) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.postExec = fn
}

// SetPersistFilter installs the predicate that decides which commands are
// recorded in the AOF and the replication stream.
func (r *Router) SetPersistFilter(fn func(cmd string, args [][]byte) bool) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.persisted = fn
}

// Persisted reports whether cmd is recorded in the AOF and the replication
// stream. Without a filter every command counts as recorded.
func (r *Router) Persisted(cmd string, args [][]byte) bool {
	r.mu.RLock()
	fn := r.persisted
	r.mu.RUnlock()
	if fn == nil {
		return true
	}
	return fn(cmd, args)
}

// SetPort records the TCP port the server listens on, reported by INFO.
func (r *Router) SetPort(port int) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.port = port
}

// Port returns the port recorded by SetPort.
func (r *Router) Port() int {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.port
}

// SetReplicationInfo installs the renderer of the INFO replication section.
func (r *Router) SetReplicationInfo(fn func() string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.replInfo = fn
}

func (r *Router) replicationInfo() func() string {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.replInfo
}

func (r *Router) postExecuteHook() func(db int, cmd string, args [][]byte) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.postExecute
}

// record hands what a command executed to the persistence hooks: its
// override when the handler set one, nothing when it answered with an error,
// itself otherwise.
func (r *Router) record(ctx *Context, db int, errsBefore int64) {
	if cmds, ok := ctx.Propagation(); ok {
		switch len(cmds) {
		case 0:
		case 1:
			if hook := r.postExecuteHook(); hook != nil {
				hook(cmds[0].DB, cmds[0].Name, cmds[0].Args)
			}
		default:
			if hook := r.postExecHook(); hook != nil {
				hook(cmds)
			}
		}
		return
	}
	if ctx.Writer != nil && ctx.Writer.Errors() > errsBefore {
		return
	}
	if hook := r.postExecuteHook(); hook != nil {
		hook(db, ctx.Command, ctx.Args)
	}
}

func (r *Router) postExecHook() func(cmds []ExecutedCommand) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.postExec
}

func (r *Router) Execute(ctx *Context) error {
	ctx.router = r
	ctx.Command = CanonicalName(ctx.Command)
	cmd, ok := r.Get(ctx.Command)
	if !ok {
		// Redis refuses to run a transaction once a command failed to queue.
		if ctx.Transaction != nil && ctx.Transaction.IsActive() {
			ctx.Transaction.Abort()
		}
		return ErrUnknownCommand
	}

	// Redis rejects a call with the wrong number of arguments before running
	// or queueing it, and a transaction that queued one is discarded at EXEC.
	if !checkArity(ctx.Command, len(ctx.Args)) {
		if ctx.Transaction != nil && ctx.Transaction.IsActive() {
			ctx.Transaction.Abort()
		}
		return ctx.Writer.WriteError(arityError(ctx.Command).Error())
	}

	// Enforce authentication if requirepass is set
	upperCmd := strings.ToUpper(ctx.Command)
	if r.RequirePass() != "" && !ctx.IsAuthenticated() && !noAuthCommands[upperCmd] {
		return ctx.Writer.WriteError("NOAUTH Authentication required.")
	}

	// Enforce the per-connection ACL user's command and key permissions. A
	// connection that never ran ACL AUTH has no ACLUser and keeps the
	// permissive default-user behaviour. enforceACL writes its own NOPERM
	// reply, so the router must stop on the bool and return nil — returning
	// the error would make the caller write a second reply.
	if enforceACL(ctx, upperCmd) {
		return nil
	}

	// maxmemory: Redis refuses a command that may grow memory while used
	// memory is over the limit, before running or queueing it; a refused
	// command discards the transaction it was sent in, and an EXEC whose
	// transaction queued one is refused as a whole.
	if overMemoryFor(ctx, ctx.Command) {
		if ctx.Transaction != nil && ctx.Transaction.IsActive() {
			ctx.Transaction.Abort()
		}
		return ctx.Writer.WriteError(errOOM.Error())
	}
	if ctx.Command == "EXEC" && ctx.Transaction != nil && ctx.Transaction.IsActive() && execDeniedForMemory(ctx) {
		ctx.Transaction.Clear()
		ctx.Transaction.ClearWatch()
		return ctx.Writer.WriteError(errExecAbortOOM.Error())
	}

	// MULTI queues commands for atomic execution at EXEC.
	if ctx.Transaction != nil && ctx.Transaction.IsActive() && !isTransactionControl(ctx.Command) {
		ctx.Transaction.Queue(ctx.Command, ctx.Args)
		return ctx.Writer.WriteSimpleString("QUEUED")
	}

	ctx.StartTime = time.Now()
	db := ctx.DB
	errsBefore := ctx.Writer.Errors()
	err := cmd.Handler(ctx)
	ctx.passWakeup()
	reaccountWrittenKeys(ctx)
	touchWrittenKeys(ctx)
	signalWrittenKeys(ctx)

	// Post-execute hook (AOF persistence and replication)
	if err == nil {
		r.record(ctx, db, errsBefore)
	}

	return err
}

// isTransactionControl reports whether cmd runs immediately even inside
// MULTI: transaction bookkeeping is never queued, and UNWATCH executes
// immediately because the replay dispatcher has no UNWATCH case.
func isTransactionControl(cmd string) bool {
	switch strings.ToUpper(cmd) {
	case "MULTI", "EXEC", "DISCARD", "WATCH", "UNWATCH":
		return true
	}
	return false
}

// ExecuteSilent runs a command without auth checks or post-execute hooks.
// Used for AOF replay where Writer may be nil — creates a discard writer.
func (r *Router) ExecuteSilent(ctx *Context) error {
	ctx.router = r
	ctx.Command = CanonicalName(ctx.Command)
	cmd, ok := r.Get(ctx.Command)
	if !ok {
		return ErrUnknownCommand
	}
	if ctx.Writer == nil {
		ctx.Writer = resp.NewWriter(io.Discard)
	}
	ctx.Authenticated = true
	ctx.StartTime = time.Now()
	err := cmd.Handler(ctx)
	reaccountWrittenKeys(ctx)
	return err
}

// ExecuteHTTP runs a command from the HTTP API with auth enforcement and post-execute hooks.
// Returns the result as an interface{} suitable for JSON serialization.
func (r *Router) ExecuteHTTP(ctx *Context) (interface{}, error) {
	ctx.router = r
	ctx.Command = CanonicalName(ctx.Command)
	cmd, ok := r.Get(ctx.Command)
	if !ok {
		return nil, ErrUnknownCommand
	}

	// Create a capture writer to collect the response
	if ctx.Writer == nil {
		ctx.Writer = resp.NewWriter(io.Discard)
	}
	ctx.StartTime = time.Now()

	// requirepass: the HTTP path enforces the same authentication gate as
	// the TCP path. The HTTP middleware records whether a request actually
	// authenticated (shared password, ACL identity, or session); when HTTP
	// auth is disabled the request is anonymous, so with requirepass set
	// only auth-exempt commands may run — the API can no longer sit wide
	// open behind a requirepass deployment. requirepass unset: gate off.
	upperCmd := strings.ToUpper(ctx.Command)
	if r.RequirePass() != "" && !ctx.IsAuthenticated() && !noAuthCommands[upperCmd] {
		ctx.Writer.WriteError("NOAUTH Authentication required")
		return nil, errors.New("NOAUTH Authentication required")
	}

	// ACL: the HTTP path runs with the same per-user command and key rules
	// as the TCP path. A caller that authenticates via ACL and sets
	// ctx.ACLUser must not lose those restrictions here; requests without an
	// ACL user keep the permissive default behaviour. The refusal reply is
	// written to ctx.Writer and the error is returned so HTTP callers can
	// surface it.
	if enforceACL(ctx, upperCmd) {
		return nil, acl.ErrPermissionDenied
	}

	if overMemoryFor(ctx, ctx.Command) {
		ctx.Writer.WriteError(errOOM.Error())
		return nil, errOOM
	}

	db := ctx.DB
	errsBefore := ctx.Writer.Errors()
	err := cmd.Handler(ctx)
	ctx.passWakeup()
	reaccountWrittenKeys(ctx)
	touchWrittenKeys(ctx)
	signalWrittenKeys(ctx)
	if err != nil {
		return nil, err
	}

	// Post-execute hook (AOF persistence and replication)
	r.record(ctx, db, errsBefore)

	// Return a simple acknowledgement - the actual response was written to the RESP writer
	return "OK", nil
}

func (r *Router) Commands() map[string]*CommandDef {
	r.mu.RLock()
	defer r.mu.RUnlock()
	result := make(map[string]*CommandDef, len(r.commands))
	for k, v := range r.commands {
		result[k] = v
	}
	return result
}
