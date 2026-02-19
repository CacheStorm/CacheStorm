package auth

import (
	"errors"
	"sync"

	"github.com/cachestorm/cachestorm/internal/command"
	"github.com/cachestorm/cachestorm/internal/plugin"
)

var (
	ErrAuthRequired = errors.New("NOAUTH Authentication required")
	ErrAuthFailed   = errors.New("ERR invalid password")
)

type AuthPlugin struct {
	mu          sync.RWMutex
	password    string
	enabled     bool
	authChecker func(string) bool
}

func New(password string, enabled bool) *AuthPlugin {
	return &AuthPlugin{
		password: password,
		enabled:  enabled,
	}
}

func (a *AuthPlugin) Name() string    { return "auth" }
func (a *AuthPlugin) Version() string { return "1.0.0" }

func (a *AuthPlugin) Init(config interface{}) error {
	return nil
}

func (a *AuthPlugin) Close() error {
	return nil
}

func (a *AuthPlugin) BeforeCommand(ctx *command.Context) error {
	if !a.enabled {
		return nil
	}

	noAuthCommands := map[string]bool{
		"AUTH":    true,
		"PING":    true,
		"QUIT":    true,
		"COMMAND": true,
		"ECHO":    true,
	}

	if noAuthCommands[ctx.Command] {
		return nil
	}

	if !ctx.IsAuthenticated() {
		return ErrAuthRequired
	}

	return nil
}

func (a *AuthPlugin) Authenticate(password string) error {
	a.mu.RLock()
	defer a.mu.RUnlock()

	if password != a.password {
		return ErrAuthFailed
	}

	return nil
}

func (a *AuthPlugin) SetPassword(password string) {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.password = password
}

func (a *AuthPlugin) IsEnabled() bool {
	return a.enabled
}

var _ plugin.BeforeCommandHook = (*AuthPlugin)(nil)
