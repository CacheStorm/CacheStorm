package command

import (
	"sync"
	"time"
)

type CommandDef struct {
	Name    string
	Handler func(ctx *Context) error
	Arity   int
	Flags   []string
}

type Router struct {
	mu       sync.RWMutex
	commands map[string]*CommandDef
}

func NewRouter() *Router {
	return &Router{
		commands: make(map[string]*CommandDef),
	}
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

func (r *Router) Execute(ctx *Context) error {
	cmd, ok := r.Get(ctx.Command)
	if !ok {
		return ErrUnknownCommand
	}
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
