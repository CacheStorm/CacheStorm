package plugin

import "github.com/cachestorm/cachestorm/internal/command"

type Plugin interface {
	Name() string
	Version() string
	Init(config interface{}) error
	Close() error
}

type BeforeCommandHook interface {
	Plugin
	BeforeCommand(ctx *command.Context) error
}

type AfterCommandHook interface {
	Plugin
	AfterCommand(ctx *command.Context)
}

type OnEvictHook interface {
	Plugin
	OnEvict(key string, value interface{})
}

type OnExpireHook interface {
	Plugin
	OnExpire(key string, value interface{})
}

type OnTagInvalidateHook interface {
	Plugin
	OnTagInvalidate(tag string, keys []string)
}

type OnStartupHook interface {
	Plugin
	OnStartup() error
}

type OnShutdownHook interface {
	Plugin
	OnShutdown() error
}

type CustomCommandProvider interface {
	Plugin
	Commands() map[string]func(*command.Context) error
}

type HTTPEndpointProvider interface {
	Plugin
	HTTPRoutes() []HTTPRoute
}

type HTTPRoute struct {
	Method  string
	Path    string
	Handler interface{}
}
