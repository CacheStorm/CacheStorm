package command

import (
	"errors"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/cachestorm/cachestorm/internal/resp"
	"github.com/cachestorm/cachestorm/internal/store"
)

type Config struct {
	mu                     sync.RWMutex
	maxMemory              int64
	maxMemoryPolicy        string
	timeout                int
	databases              int
	slowlogLogSlowerThan   int64
	slowlogMaxLen          int
	logLevel               string
	saveIntervals          []string
	appendOnly             bool
	appendFsync            string
	daemonize              bool
	pidfile                string
	port                   int
	bind                   string
	protectedMode          bool
	tcpKeepalive           int
	maxClients             int64
	maxMemorySamples       int
	lfuDecayTime           int
	lfuLogFactor           int
	activedefrag           bool
	lazyFreeLazyEviction   bool
	lazyFreeLazyExpire     bool
	lazyFreeLazyServerDel  bool
	hashMaxListpackEntries int
	hashMaxListpackValue   int
	listMaxListpackSize    int
	setMaxIntsetEntries    int64
	zsetMaxListpackEntries int
}

var globalConfig = &Config{
	maxMemory:              0,
	maxMemoryPolicy:        "noeviction",
	timeout:                0,
	databases:              16,
	slowlogLogSlowerThan:   10000,
	slowlogMaxLen:          128,
	logLevel:               "notice",
	saveIntervals:          []string{},
	appendOnly:             false,
	appendFsync:            "everysec",
	daemonize:              false,
	pidfile:                "",
	port:                   6380,
	bind:                   "0.0.0.0",
	protectedMode:          true,
	tcpKeepalive:           300,
	maxClients:             10000,
	maxMemorySamples:       5,
	lfuDecayTime:           1,
	lfuLogFactor:           10,
	activedefrag:           false,
	lazyFreeLazyEviction:   false,
	lazyFreeLazyExpire:     false,
	lazyFreeLazyServerDel:  false,
	hashMaxListpackEntries: 512,
	hashMaxListpackValue:   64,
	listMaxListpackSize:    -2,
	setMaxIntsetEntries:    512,
	zsetMaxListpackEntries: 128,
}

func RegisterConfigCommands(router *Router) {
	router.Register(&CommandDef{Name: "CONFIG", Handler: cmdCONFIG})
}

func cmdCONFIG(ctx *Context) error {
	if ctx.ArgCount() < 1 {
		return ctx.WriteError(ErrWrongArgCount)
	}

	subCmd := strings.ToUpper(ctx.ArgString(0))

	switch subCmd {
	case "GET":
		return cmdConfigGet(ctx)
	case "SET":
		return cmdConfigSet(ctx)
	case "RESETSTAT":
		return cmdConfigResetStat(ctx)
	case "REWRITE":
		h := configHooksFor(ctx)
		if h == nil || h.Rewrite == nil {
			return ctx.WriteError(errors.New("ERR The server is running without a config file"))
		}
		if err := h.Rewrite(); err != nil {
			return ctx.WriteError(err)
		}
		return ctx.WriteOK()
	default:
		return ctx.WriteError(ErrUnknownCommand)
	}
}

// ConfigHooks connect CONFIG GET/SET to the running server. Values returns
// the live value of every parameter the server owns; Set applies one
// parameter and reports whether the server owns it.
type ConfigHooks struct {
	Values func() map[string]string
	Set    func(name, value string) (bool, error)
	// Rewrite writes the running configuration back to the file it was
	// loaded from.
	Rewrite func() error
}

// SetConfigHooks installs the server's CONFIG hooks.
func (r *Router) SetConfigHooks(h *ConfigHooks) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.configHooks = h
}

func (r *Router) getConfigHooks() *ConfigHooks {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.configHooks
}

func configHooksFor(ctx *Context) *ConfigHooks {
	if r := ctx.dispatcher(); r != nil {
		return r.getConfigHooks()
	}
	return nil
}

func configValues(ctx *Context) map[string]string {
	c := globalConfig
	c.mu.RLock()
	values := map[string]string{
		"maxmemory":                 strconv.FormatInt(c.maxMemory, 10),
		"maxmemory-policy":          c.maxMemoryPolicy,
		"maxmemory-samples":         strconv.Itoa(c.maxMemorySamples),
		"maxclients":                strconv.FormatInt(c.maxClients, 10),
		"timeout":                   strconv.Itoa(c.timeout),
		"databases":                 strconv.Itoa(c.databases),
		"slowlog-log-slower-than":   strconv.FormatInt(c.slowlogLogSlowerThan, 10),
		"slowlog-max-len":           strconv.Itoa(c.slowlogMaxLen),
		"loglevel":                  c.logLevel,
		"daemonize":                 boolStr(c.daemonize),
		"pidfile":                   c.pidfile,
		"port":                      strconv.Itoa(c.port),
		"bind":                      c.bind,
		"protected-mode":            boolStr(c.protectedMode),
		"tcp-keepalive":             strconv.Itoa(c.tcpKeepalive),
		"save":                      strings.Join(c.saveIntervals, " "),
		"appendonly":                boolStr(c.appendOnly),
		"appendfsync":               c.appendFsync,
		"lfu-decay-time":            strconv.Itoa(c.lfuDecayTime),
		"lfu-log-factor":            strconv.Itoa(c.lfuLogFactor),
		"activedefrag":              boolStr(c.activedefrag),
		"lazyfree-lazy-eviction":    boolStr(c.lazyFreeLazyEviction),
		"lazyfree-lazy-expire":      boolStr(c.lazyFreeLazyExpire),
		"lazyfree-lazy-server-del":  boolStr(c.lazyFreeLazyServerDel),
		"hash-max-listpack-entries": strconv.Itoa(c.hashMaxListpackEntries),
		"hash-max-listpack-value":   strconv.Itoa(c.hashMaxListpackValue),
		"list-max-listpack-size":    strconv.Itoa(c.listMaxListpackSize),
		"set-max-intset-entries":    strconv.FormatInt(c.setMaxIntsetEntries, 10),
		"zset-max-listpack-entries": strconv.Itoa(c.zsetMaxListpackEntries),
		"latency-monitor-threshold": strconv.FormatInt(LatencyThreshold(), 10),
	}
	c.mu.RUnlock()
	if h := configHooksFor(ctx); h != nil && h.Values != nil {
		for name, value := range h.Values() {
			values[name] = value
		}
	}
	return values
}

// cmdConfigGet answers CONFIG GET parameter [parameter ...]; each parameter
// is a case-insensitive glob over the parameter names, as in Redis 7.
func cmdConfigGet(ctx *Context) error {
	if ctx.ArgCount() < 2 {
		return ctx.WriteError(ErrWrongArgCount)
	}
	values := configValues(ctx)
	names := make([]string, 0, len(values))
	for name := range values {
		names = append(names, name)
	}
	sort.Strings(names)
	results := make([]*resp.Value, 0)
	seen := make(map[string]bool)
	for i := 1; i < ctx.ArgCount(); i++ {
		pattern := strings.ToLower(ctx.ArgString(i))
		for _, name := range names {
			if !seen[name] && store.MatchPattern(name, pattern) {
				seen[name] = true
				results = append(results, resp.BulkString(name), resp.BulkString(values[name]))
			}
		}
	}
	return ctx.Writer.WriteValue(resp.PairsValue(results))
}

func boolStr(b bool) string {
	if b {
		return "yes"
	}
	return "no"
}

func cmdConfigSet(ctx *Context) error {
	// Index 0 holds the "SET" subcommand, so a well-formed call carries 1+2k
	// arguments: an ODD count, with at least one key/value pair. Testing for
	// even counts rejected every valid call, and accepted malformed ones — the
	// pair loop then read ctx.ArgString(i+1) past the end and silently did
	// nothing while replying +OK.
	if ctx.ArgCount() < 3 || ctx.ArgCount()%2 == 0 {
		return ctx.WriteError(ErrWrongArgCount)
	}

	c := globalConfig
	c.mu.Lock()
	defer c.mu.Unlock()

	validEvictionPolicies := map[string]bool{
		"noeviction":      true,
		"allkeys-lru":     true,
		"allkeys-lfu":     true,
		"allkeys-random":  true,
		"volatile-lru":    true,
		"volatile-lfu":    true,
		"volatile-random": true,
		"volatile-ttl":    true,
	}

	hooks := configHooksFor(ctx)
	for i := 1; i < ctx.ArgCount(); i += 2 {
		param := strings.ToLower(ctx.ArgString(i))
		value := ctx.ArgString(i + 1)

		if hooks != nil && hooks.Set != nil {
			handled, err := hooks.Set(param, value)
			if err != nil {
				return ctx.WriteError(err)
			}
			if handled {
				continue
			}
		}

		switch param {
		case "maxmemory":
			if v, err := strconv.ParseInt(value, 10, 64); err == nil {
				if v < 0 {
					return ctx.WriteError(errors.New("ERR Invalid argument '" + value + "' for CONFIG SET 'maxmemory'"))
				}
				c.maxMemory = v
			}
		case "maxmemory-policy":
			if !validEvictionPolicies[value] {
				return ctx.WriteError(errors.New("ERR Invalid argument '" + value + "' for CONFIG SET 'maxmemory-policy'"))
			}
			c.maxMemoryPolicy = value
		case "maxclients":
			if v, err := strconv.ParseInt(value, 10, 64); err == nil {
				if v < 1 {
					return ctx.WriteError(errors.New("ERR Invalid argument '" + value + "' for CONFIG SET 'maxclients'"))
				}
				c.maxClients = v
			}
		case "timeout":
			if v, err := strconv.Atoi(value); err == nil {
				if v < 0 {
					return ctx.WriteError(errors.New("ERR Invalid argument '" + value + "' for CONFIG SET 'timeout'"))
				}
				c.timeout = v
			}
		case "slowlog-log-slower-than":
			if v, err := strconv.ParseInt(value, 10, 64); err == nil {
				c.slowlogLogSlowerThan = v
				// Apply to the recorder the request path actually appends to
				// (store.GlobalSlowLog). Writing command.globalSlowLog instead
				// left the live threshold unchanged, so lowering it at runtime
				// looked successful via CONFIG GET but recorded nothing.
				// The parameter is in microseconds, as in Redis.
				if v < 0 {
					v = 0
				}
				store.GlobalSlowLog.SetThreshold(time.Duration(v) * time.Microsecond)
			}
		case "slowlog-max-len":
			if v, err := strconv.Atoi(value); err == nil {
				c.slowlogMaxLen = v
				store.GlobalSlowLog.SetMaxSize(v)
			}
		case "loglevel":
			c.logLevel = value
		case "appendonly":
			c.appendOnly = value == "yes"
		case "appendfsync":
			c.appendFsync = value
		case "tcp-keepalive":
			if v, err := strconv.Atoi(value); err == nil {
				c.tcpKeepalive = v
			}
		case "lfu-decay-time":
			if v, err := strconv.Atoi(value); err == nil {
				c.lfuDecayTime = v
			}
		case "lfu-log-factor":
			if v, err := strconv.Atoi(value); err == nil {
				c.lfuLogFactor = v
			}
		case "latency-monitor-threshold":
			v, err := strconv.ParseInt(value, 10, 64)
			if err != nil || v < 0 {
				return ctx.WriteError(errors.New("ERR CONFIG SET failed (possibly related to argument 'latency-monitor-threshold') - argument couldn't be parsed into an integer"))
			}
			SetLatencyThreshold(v)
		case "activedefrag":
			c.activedefrag = value == "yes"
		default:
			return ctx.WriteError(errors.New("ERR Unknown option or number of arguments for CONFIG SET '" + param + "'"))
		}
	}

	return ctx.WriteOK()
}

func cmdConfigResetStat(ctx *Context) error {
	// Mirror the sibling METRICS.RESET (monitoring_commands.go), which already
	// performs this reset. Without the call the handler answers +OK while every
	// counter keeps climbing, so a client that trusts the +OK keeps reading
	// stale, ever-growing statistics from INFO/STATSALL.
	store.GlobalMetrics.Reset()
	return ctx.WriteOK()
}

func GetConfig() *Config {
	return globalConfig
}
