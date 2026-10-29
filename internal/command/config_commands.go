package command

import (
	"errors"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/cachestorm/cachestorm/internal/config"
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
	setMaxListpackEntries  int64
	setMaxListpackValue    int64
	// params holds the values CONFIG SET gave the parameters in
	// configParams.
	params map[string]string
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
	setMaxListpackEntries:  128,
	setMaxListpackValue:    64,
}

func RegisterConfigCommands(router *Router) {
	router.Register(&CommandDef{Name: "CONFIG", Handler: cmdCONFIG})
}

func cmdCONFIG(ctx *Context) error {
	if ctx.ArgCount() < 1 {
		return ctx.WriteError(ErrWrongArgCount)
	}

	if !checkRedisSubcommand(ctx, "config", nil) {
		return nil
	}

	switch strings.ToUpper(ctx.ArgString(0)) {
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
		return ctx.WriteArray(helpReply(configHelp))
	}
}

const configHelp = `CONFIG <subcommand> [<arg> [value] [opt] ...]. Subcommands are:
GET <pattern>
    Return parameters matching the glob-like <pattern> and their values.
SET <directive> <value>
    Set the configuration <directive> to <value>.
RESETSTAT
    Reset statistics reported by the INFO command.
REWRITE
    Rewrite the configuration file.
HELP
    Print this help.`

// ConfigSetError is a CONFIG SET failure for the parameter name, worded as
// Redis words it.
func ConfigSetError(name, reason string) error {
	return errors.New("ERR CONFIG SET failed (possibly related to argument '" + name + "') - " + reason)
}

// ParseConfigInt parses the integer value of parameter name as Redis does,
// with its errors for a value that is not an integer or is out of range.
func ParseConfigInt(name, value string, lo, hi int64) (int64, error) {
	v, err := strconv.ParseInt(value, 10, 64)
	if err != nil {
		return 0, ConfigSetError(name, "argument couldn't be parsed into an integer")
	}
	if v < lo || v > hi {
		return 0, ConfigSetError(name, "argument must be between "+strconv.FormatInt(lo, 10)+" and "+strconv.FormatInt(hi, 10)+" inclusive")
	}
	return v, nil
}

// ParseConfigBool parses a yes/no parameter as Redis does.
func ParseConfigBool(name, value string) (bool, error) {
	switch strings.ToLower(value) {
	case "yes":
		return true, nil
	case "no":
		return false, nil
	}
	return false, ConfigSetError(name, "argument must be 'yes' or 'no'")
}

// ParseConfigEnum checks an enumerated parameter against its values, in
// the order Redis lists them in its error.
func ParseConfigEnum(name, value string, values ...string) (string, error) {
	for _, v := range values {
		if strings.EqualFold(v, value) {
			return v, nil
		}
	}
	return "", ConfigSetError(name, "argument(s) must be one of the following: "+strings.Join(values, ", "))
}

const (
	configIntMax  = 1<<31 - 1
	configIntMin  = -1 << 31
	configLongMax = 1<<63 - 1
)

// configSettable lists the parameters CONFIG SET accepts besides those the
// server's hooks own.
var configSettable = map[string]bool{
	"maxmemory": true, "maxmemory-policy": true, "maxmemory-samples": true, "maxclients": true,
	"timeout": true, "slowlog-log-slower-than": true, "slowlog-max-len": true, "loglevel": true,
	"appendonly": true, "appendfsync": true, "tcp-keepalive": true, "lfu-decay-time": true,
	"lfu-log-factor": true, "latency-monitor-threshold": true, "latency-tracking": true,
	"latency-tracking-info-percentiles": true, "activedefrag": true,
	"notify-keyspace-events": true, "lazyfree-lazy-eviction": true, "lazyfree-lazy-expire": true,
	"lazyfree-lazy-server-del": true, "hash-max-listpack-entries": true, "hash-max-listpack-value": true,
	"list-max-listpack-size": true, "set-max-intset-entries": true, "zset-max-listpack-entries": true,
	"databases": true, "daemonize": true, "pidfile": true, "port": true, "bind": true,
	"protected-mode": true, "save": true,
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
		"maxmemory":                         strconv.FormatInt(c.maxMemory, 10),
		"maxmemory-policy":                  c.maxMemoryPolicy,
		"maxmemory-samples":                 strconv.Itoa(c.maxMemorySamples),
		"maxclients":                        strconv.FormatInt(c.maxClients, 10),
		"timeout":                           strconv.Itoa(c.timeout),
		"databases":                         strconv.Itoa(c.databases),
		"slowlog-log-slower-than":           strconv.FormatInt(c.slowlogLogSlowerThan, 10),
		"slowlog-max-len":                   strconv.Itoa(c.slowlogMaxLen),
		"loglevel":                          c.logLevel,
		"daemonize":                         boolStr(c.daemonize),
		"pidfile":                           c.pidfile,
		"port":                              strconv.Itoa(c.port),
		"bind":                              c.bind,
		"protected-mode":                    boolStr(c.protectedMode),
		"tcp-keepalive":                     strconv.Itoa(c.tcpKeepalive),
		"save":                              strings.Join(c.saveIntervals, " "),
		"appendonly":                        boolStr(c.appendOnly),
		"appendfsync":                       c.appendFsync,
		"lfu-decay-time":                    strconv.Itoa(c.lfuDecayTime),
		"lfu-log-factor":                    strconv.Itoa(c.lfuLogFactor),
		"activedefrag":                      boolStr(c.activedefrag),
		"lazyfree-lazy-eviction":            boolStr(c.lazyFreeLazyEviction),
		"lazyfree-lazy-expire":              boolStr(c.lazyFreeLazyExpire),
		"lazyfree-lazy-server-del":          boolStr(c.lazyFreeLazyServerDel),
		"hash-max-listpack-entries":         strconv.Itoa(c.hashMaxListpackEntries),
		"hash-max-listpack-value":           strconv.Itoa(c.hashMaxListpackValue),
		"list-max-listpack-size":            strconv.Itoa(c.listMaxListpackSize),
		"set-max-intset-entries":            strconv.FormatInt(c.setMaxIntsetEntries, 10),
		"zset-max-listpack-entries":         strconv.Itoa(c.zsetMaxListpackEntries),
		"latency-monitor-threshold":         strconv.FormatInt(LatencyThreshold(), 10),
		"latency-tracking":                  boolStr(commandStats.tracking.Load()),
		"latency-tracking-info-percentiles": formatLatencyPercentiles(),
	}
	for name, p := range configParams {
		values[name] = c.paramValueLocked(name, p)
	}
	c.mu.RUnlock()
	if ctx.Store != nil {
		values["notify-keyspace-events"] = store.FormatNotifyFlags(ctx.Store.NotifyFlags())
	}
	if h := configHooksFor(ctx); h != nil && h.Values != nil {
		for name, value := range h.Values() {
			values[name] = value
		}
	}
	for alias, name := range configAliases {
		values[alias] = values[name]
	}
	return values
}

// cmdConfigGet answers CONFIG GET parameter [parameter ...]; each parameter
// is a case-insensitive glob over the parameter names, as in Redis 7.
func cmdConfigGet(ctx *Context) error {
	if ctx.ArgCount() < 2 {
		return ctx.WriteError(arityError("config|get"))
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

// configParamName is the lower-case name of a CONFIG SET parameter, with an
// alias resolved to the parameter it names.
func configParamName(name string) string {
	name = strings.ToLower(name)
	if canonical, ok := configAliases[name]; ok {
		return canonical
	}
	return name
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
		return ctx.WriteError(arityError("config|set"))
	}
	known := configValues(ctx)
	seen := make(map[string]bool, ctx.ArgCount()/2)
	for i := 1; i < ctx.ArgCount(); i += 2 {
		param := configParamName(ctx.ArgString(i))
		if _, ok := known[param]; !ok && !configSettable[param] {
			return ctx.WriteError(errors.New("ERR Unknown option or number of arguments for CONFIG SET - '" + ctx.ArgString(i) + "'"))
		}
		if seen[param] {
			return ctx.WriteError(ConfigSetError(ctx.ArgString(i), "duplicate parameter"))
		}
		seen[param] = true
	}

	c := globalConfig
	c.mu.Lock()
	defer c.mu.Unlock()

	hooks := configHooksFor(ctx)
	apply := func(param, value string) error {
		if hooks != nil && hooks.Set != nil {
			handled, err := hooks.Set(param, value)
			if err != nil || handled {
				return err
			}
		}
		return c.setLocked(ctx, param, value)
	}
	// Like Redis, a CONFIG SET of several parameters applies all of them or
	// none: a failure restores the parameters already set.
	applied := make([]string, 0, ctx.ArgCount()/2)
	for i := 1; i < ctx.ArgCount(); i += 2 {
		param := configParamName(ctx.ArgString(i))
		if err := apply(param, ctx.ArgString(i+1)); err != nil {
			if given := strings.ToLower(ctx.ArgString(i)); given != param {
				err = errors.New(strings.Replace(err.Error(), "argument '"+param+"')", "argument '"+given+"')", 1))
			}
			for j := len(applied) - 1; j >= 0; j-- {
				if old, ok := known[applied[j]]; ok {
					_ = apply(applied[j], old) //nolint:errcheck // the old value was valid when set
				}
			}
			return ctx.WriteError(err)
		}
		applied = append(applied, param)
	}

	return ctx.WriteOK()
}

// setLocked applies one CONFIG SET parameter the server's hooks do not own.
func (c *Config) setLocked(ctx *Context, param, value string) error {
	switch param {
	case "maxmemory":
		v, err := config.ParseMemorySize(value)
		if err != nil || v < 0 {
			return ConfigSetError(param, "argument must be a memory value")
		}
		c.maxMemory = v
	case "maxmemory-policy":
		v, err := ParseConfigEnum(param, value, "volatile-lru", "volatile-lfu", "volatile-random", "volatile-ttl",
			"allkeys-lru", "allkeys-lfu", "allkeys-random", "noeviction")
		if err != nil {
			return err
		}
		c.maxMemoryPolicy = v
	case "maxmemory-samples":
		v, err := ParseConfigInt(param, value, 1, 64)
		if err != nil {
			return err
		}
		c.maxMemorySamples = int(v)
	case "maxclients":
		v, err := ParseConfigInt(param, value, 1, 1<<32-1)
		if err != nil {
			return err
		}
		c.maxClients = v
	case "timeout":
		v, err := ParseConfigInt(param, value, 0, configIntMax)
		if err != nil {
			return err
		}
		c.timeout = int(v)
	case "slowlog-log-slower-than":
		v, err := ParseConfigInt(param, value, -1, configLongMax)
		if err != nil {
			return err
		}
		c.slowlogLogSlowerThan = v
		// Apply to the recorder the request path actually appends to
		// (store.GlobalSlowLog); the parameter is in microseconds.
		if v < 0 {
			v = 0
		}
		store.GlobalSlowLog.SetThreshold(time.Duration(v) * time.Microsecond)
	case "slowlog-max-len":
		v, err := ParseConfigInt(param, value, 0, configLongMax)
		if err != nil {
			return err
		}
		c.slowlogMaxLen = int(v)
		store.GlobalSlowLog.SetMaxSize(int(v))
	case "loglevel":
		v, err := ParseConfigEnum(param, value, "debug", "verbose", "notice", "warning", "nothing")
		if err != nil {
			return err
		}
		c.logLevel = v
	case "appendonly", "activedefrag", "lazyfree-lazy-eviction", "lazyfree-lazy-expire", "lazyfree-lazy-server-del", "protected-mode":
		v, err := ParseConfigBool(param, value)
		if err != nil {
			return err
		}
		switch param {
		case "appendonly":
			c.appendOnly = v
		case "activedefrag":
			c.activedefrag = v
		case "lazyfree-lazy-eviction":
			c.lazyFreeLazyEviction = v
		case "lazyfree-lazy-expire":
			c.lazyFreeLazyExpire = v
		case "lazyfree-lazy-server-del":
			c.lazyFreeLazyServerDel = v
		case "protected-mode":
			c.protectedMode = v
		}
	case "appendfsync":
		v, err := ParseConfigEnum(param, value, "everysec", "always", "no")
		if err != nil {
			return err
		}
		c.appendFsync = v
	case "tcp-keepalive", "lfu-decay-time", "lfu-log-factor":
		v, err := ParseConfigInt(param, value, 0, configIntMax)
		if err != nil {
			return err
		}
		switch param {
		case "tcp-keepalive":
			c.tcpKeepalive = int(v)
		case "lfu-decay-time":
			c.lfuDecayTime = int(v)
		default:
			c.lfuLogFactor = int(v)
		}
		store.SetLFUParams(int64(c.lfuLogFactor), int64(c.lfuDecayTime))
	case "hash-max-listpack-entries", "hash-max-listpack-value", "set-max-intset-entries", "zset-max-listpack-entries":
		var v int64
		if param == "hash-max-listpack-value" {
			u, ok := redisMemtoull(value)
			if !ok {
				return ConfigSetError(param, "argument must be a memory value")
			}
			if u > configLongMax {
				return ConfigSetError(param, "argument must be between 0 and 9223372036854775807 inclusive")
			}
			v = int64(u)
		} else {
			var err error
			if v, err = ParseConfigInt(param, value, 0, configLongMax); err != nil {
				return err
			}
		}
		switch param {
		case "hash-max-listpack-entries":
			c.hashMaxListpackEntries = int(v)
			store.SetHashListpackLimits(v, int64(c.hashMaxListpackValue))
		case "hash-max-listpack-value":
			c.hashMaxListpackValue = int(v)
			store.SetHashListpackLimits(int64(c.hashMaxListpackEntries), v)
		case "set-max-intset-entries":
			c.setMaxIntsetEntries = v
			store.SetSetEncodingLimits(v, c.setMaxListpackEntries, c.setMaxListpackValue)
		default:
			c.zsetMaxListpackEntries = int(v)
			_, value := store.ZsetListpackLimits()
			store.SetZsetListpackLimits(v, value)
		}
	case "list-max-listpack-size":
		v, err := ParseConfigInt(param, value, configIntMin, configIntMax)
		if err != nil {
			return err
		}
		c.listMaxListpackSize = int(v)
	case "latency-monitor-threshold":
		v, err := ParseConfigInt(param, value, 0, configLongMax)
		if err != nil {
			return err
		}
		SetLatencyThreshold(v)
	case "latency-tracking":
		v, err := ParseConfigBool(param, value)
		if err != nil {
			return err
		}
		commandStats.tracking.Store(v)
	case "latency-tracking-info-percentiles":
		ps, msg := parseLatencyPercentiles(value)
		if msg != "" {
			return ConfigSetError(param, msg)
		}
		commandStats.percentiles.Store(&ps)
	case "notify-keyspace-events":
		flags, ok := store.ParseNotifyFlags(value)
		if !ok {
			return ConfigSetError(param, "Invalid event class character. Use 'Ag$lshzxeKEtmdn'.")
		}
		if ctx.Store != nil {
			ctx.Store.SetNotifyFlags(flags)
		}
	case "port":
		if _, err := ParseConfigInt(param, value, 0, 65535); err != nil {
			return err
		}
		return ConfigSetError(param, "can't set immutable config")
	case "daemonize":
		if _, err := ParseConfigBool(param, value); err != nil {
			return err
		}
		return ConfigSetError(param, "can't set immutable config")
	case "databases", "pidfile", "bind":
		return ConfigSetError(param, "can't set immutable config")
	case "save":
		c.saveIntervals = strings.Fields(value)
	default:
		if p, ok := configParams[param]; ok {
			return c.setParamLocked(param, value, p)
		}
		return errors.New("ERR Unknown option or number of arguments for CONFIG SET - '" + param + "'")
	}
	return nil
}

func cmdConfigResetStat(ctx *Context) error {
	// Mirror the sibling METRICS.RESET (monitoring_commands.go), which already
	// performs this reset. Without the call the handler answers +OK while every
	// counter keeps climbing, so a client that trusts the +OK keeps reading
	// stale, ever-growing statistics from INFO/STATSALL.
	store.GlobalMetrics.Reset()
	resetCommandStats()
	ctx.Store.ResetMemoryPeak()
	return ctx.WriteOK()
}

func GetConfig() *Config {
	return globalConfig
}
