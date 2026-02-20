package command

import (
	"strconv"
	"strings"
	"sync"

	"github.com/cachestorm/cachestorm/internal/resp"
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
	port:                   6379,
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
		return ctx.WriteOK()
	default:
		return ctx.WriteError(ErrUnknownCommand)
	}
}

func cmdConfigGet(ctx *Context) error {
	if ctx.ArgCount() != 2 {
		return ctx.WriteError(ErrWrongArgCount)
	}

	pattern := strings.ToLower(ctx.ArgString(1))
	results := make([]*resp.Value, 0)

	addConfig := func(name, value string) {
		if pattern == "*" || strings.Contains(strings.ToLower(name), pattern) || matchConfigPattern(name, pattern) {
			results = append(results, resp.BulkString(name), resp.BulkString(value))
		}
	}

	c := globalConfig
	c.mu.RLock()
	defer c.mu.RUnlock()

	addConfig("maxmemory", strconv.FormatInt(c.maxMemory, 10))
	addConfig("maxmemory-policy", c.maxMemoryPolicy)
	addConfig("maxmemory-samples", strconv.Itoa(c.maxMemorySamples))
	addConfig("maxclients", strconv.FormatInt(c.maxClients, 10))
	addConfig("timeout", strconv.Itoa(c.timeout))
	addConfig("databases", strconv.Itoa(c.databases))
	addConfig("slowlog-log-slower-than", strconv.FormatInt(c.slowlogLogSlowerThan, 10))
	addConfig("slowlog-max-len", strconv.Itoa(c.slowlogMaxLen))
	addConfig("loglevel", c.logLevel)
	addConfig("daemonize", boolStr(c.daemonize))
	addConfig("pidfile", c.pidfile)
	addConfig("port", strconv.Itoa(c.port))
	addConfig("bind", c.bind)
	addConfig("protected-mode", boolStr(c.protectedMode))
	addConfig("tcp-keepalive", strconv.Itoa(c.tcpKeepalive))
	addConfig("save", strings.Join(c.saveIntervals, " "))
	addConfig("appendonly", boolStr(c.appendOnly))
	addConfig("appendfsync", c.appendFsync)
	addConfig("lfu-decay-time", strconv.Itoa(c.lfuDecayTime))
	addConfig("lfu-log-factor", strconv.Itoa(c.lfuLogFactor))
	addConfig("activedefrag", boolStr(c.activedefrag))
	addConfig("lazy-free-lazy-eviction", boolStr(c.lazyFreeLazyEviction))
	addConfig("lazy-free-lazy-expire", boolStr(c.lazyFreeLazyExpire))
	addConfig("lazy-free-lazy-server-del", boolStr(c.lazyFreeLazyServerDel))
	addConfig("hash-max-listpack-entries", strconv.Itoa(c.hashMaxListpackEntries))
	addConfig("hash-max-listpack-value", strconv.Itoa(c.hashMaxListpackValue))
	addConfig("list-max-listpack-size", strconv.Itoa(c.listMaxListpackSize))
	addConfig("set-max-intset-entries", strconv.FormatInt(c.setMaxIntsetEntries, 10))
	addConfig("zset-max-listpack-entries", strconv.Itoa(c.zsetMaxListpackEntries))

	return ctx.WriteArray(results)
}

func matchConfigPattern(name, pattern string) bool {
	if strings.Contains(pattern, "*") {
		return true
	}
	return strings.Contains(name, pattern)
}

func boolStr(b bool) string {
	if b {
		return "yes"
	}
	return "no"
}

func cmdConfigSet(ctx *Context) error {
	if ctx.ArgCount()%2 != 0 {
		return ctx.WriteError(ErrWrongArgCount)
	}

	c := globalConfig
	c.mu.Lock()
	defer c.mu.Unlock()

	for i := 1; i < ctx.ArgCount(); i += 2 {
		param := strings.ToLower(ctx.ArgString(i))
		value := ctx.ArgString(i + 1)

		switch param {
		case "maxmemory":
			if v, err := strconv.ParseInt(value, 10, 64); err == nil {
				c.maxMemory = v
			}
		case "maxmemory-policy":
			c.maxMemoryPolicy = value
		case "maxclients":
			if v, err := strconv.ParseInt(value, 10, 64); err == nil {
				c.maxClients = v
			}
		case "timeout":
			if v, err := strconv.Atoi(value); err == nil {
				c.timeout = v
			}
		case "slowlog-log-slower-than":
			if v, err := strconv.ParseInt(value, 10, 64); err == nil {
				c.slowlogLogSlowerThan = v
				globalSlowLog.mu.Lock()
				globalSlowLog.slowLogSl = v
				globalSlowLog.mu.Unlock()
			}
		case "slowlog-max-len":
			if v, err := strconv.Atoi(value); err == nil {
				c.slowlogMaxLen = v
				globalSlowLog.mu.Lock()
				globalSlowLog.maxLen = v
				globalSlowLog.mu.Unlock()
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
		case "activedefrag":
			c.activedefrag = value == "yes"
		}
	}

	return ctx.WriteOK()
}

func cmdConfigResetStat(ctx *Context) error {
	// Reset stats
	return ctx.WriteOK()
}

func GetConfig() *Config {
	return globalConfig
}
