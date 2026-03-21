package command

import (
	"crypto/sha256"
	"crypto/subtle"
	"encoding/hex"
	"io"
	"strings"
	"sync"
	"time"

	"github.com/cachestorm/cachestorm/internal/resp"
)

type CommandDef struct { //nolint:revive // Command prefix is intentional for clarity
	Name    string
	Handler func(ctx *Context) error
	Arity   int
	Flags   []string
}

type Router struct {
	mu          sync.RWMutex
	commands    map[string]*CommandDef
	requirePass string
	postExecute func(cmd string, args [][]byte)
}

// globalRouter is set during NewRouter() for access from command handlers (e.g. AUTH)
var globalRouter *Router

func NewRouter() *Router {
	r := &Router{
		commands: make(map[string]*CommandDef),
	}
	globalRouter = r
	return r
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

func (r *Router) Register(def *CommandDef) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.commands[def.Name] = def
}

func (r *Router) Get(name string) (*CommandDef, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	cmd, ok := r.commands[name]
	return cmd, ok
}

// Commands that are allowed before authentication
var noAuthCommands = map[string]bool{
	"AUTH":    true,
	"HELLO":   true,
	"PING":    true,
	"QUIT":    true,
	"RESET":   true,
	"COMMAND": true,
}

func (r *Router) SetPostExecute(fn func(cmd string, args [][]byte)) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.postExecute = fn
}

func (r *Router) Execute(ctx *Context) error {
	cmd, ok := r.Get(ctx.Command)
	if !ok {
		return ErrUnknownCommand
	}

	// Enforce authentication if requirepass is set
	upperCmd := strings.ToUpper(ctx.Command)
	if r.RequirePass() != "" && !ctx.IsAuthenticated() && !noAuthCommands[upperCmd] {
		return ctx.Writer.WriteError("NOAUTH Authentication required.")
	}

	ctx.StartTime = time.Now()
	err := cmd.Handler(ctx)

	// Post-execute hook (AOF persistence)
	if err == nil && r.postExecute != nil {
		r.postExecute(ctx.Command, ctx.Args)
	}

	return err
}

// ExecuteSilent runs a command without auth checks or post-execute hooks.
// Used for AOF replay where Writer may be nil — creates a discard writer.
func (r *Router) ExecuteSilent(ctx *Context) error {
	cmd, ok := r.Get(ctx.Command)
	if !ok {
		return ErrUnknownCommand
	}
	if ctx.Writer == nil {
		ctx.Writer = resp.NewWriter(io.Discard)
	}
	ctx.Authenticated = true
	ctx.StartTime = time.Now()
	return cmd.Handler(ctx)
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
