package command

import (
	"errors"
	"math"
	"net"
	"strconv"
	"strings"
	"sync/atomic"

	"github.com/cachestorm/cachestorm/internal/resp"
	"github.com/cachestorm/cachestorm/internal/store"
)

// The rest of Redis 7.4's CONFIG parameters. Each takes Redis' values and
// gives Redis' errors; a few drive CacheStorm (the encoding limits,
// hll-sparse-max-bytes, acllog-max-len), the others are kept so that CONFIG
// GET and CONFIG SET answer as Redis does for settings CacheStorm has no use
// for.

type configKind uint8

const (
	cfgBool configKind = iota
	cfgInt
	cfgMemory
	cfgEnum
	cfgString
	cfgSpecial
)

type configParam struct {
	kind      configKind
	def       string
	lo, hi    int64
	enum      []string
	immutable bool
	// unsigned lets a memory value range up to 2^64-1.
	unsigned bool
	// check validates a cfgString or cfgSpecial value and returns the value
	// CONFIG GET then shows.
	check func(name, value string) (string, error)
	// apply hands a new numeric value to the code it configures.
	apply func(v int64)
	// applyText is apply for a parameter that is not a number.
	applyText func(v string)
}

const defaultOutputBufferLimit = "normal 0 0 0 slave 268435456 67108864 60 pubsub 33554432 8388608 60"

func boolParam(def string) *configParam { return &configParam{kind: cfgBool, def: def} }
func intParam(def string, lo, hi int64) *configParam {
	return &configParam{kind: cfgInt, def: def, lo: lo, hi: hi}
}
func memParam(def string, lo, hi int64) *configParam {
	return &configParam{kind: cfgMemory, def: def, lo: lo, hi: hi}
}
func enumParam(def string, values ...string) *configParam {
	return &configParam{kind: cfgEnum, def: def, enum: values}
}
func stringParam(def string) *configParam { return &configParam{kind: cfgString, def: def} }
func immutableParam(def string) *configParam {
	return &configParam{kind: cfgString, def: def, immutable: true}
}

var aclLogMaxLenValue atomic.Int64

func init() { aclLogMaxLenValue.Store(128) }

var configParams = map[string]*configParam{
	"acl-pubsub-default":                  enumParam("resetchannels", "allchannels", "resetchannels"),
	"aclfile":                             immutableParam(""),
	"acllog-max-len":                      {kind: cfgInt, def: "128", lo: 0, hi: configLongMax, apply: aclLogMaxLenValue.Store},
	"active-defrag-cycle-max":             intParam("25", 1, 99),
	"active-defrag-cycle-min":             intParam("1", 1, 99),
	"active-defrag-ignore-bytes":          memParam("104857600", 1, configLongMax),
	"active-defrag-max-scan-fields":       intParam("1000", 1, configLongMax),
	"active-defrag-threshold-lower":       intParam("10", 0, 1000),
	"active-defrag-threshold-upper":       intParam("100", 0, 1000),
	"active-expire-effort":                intParam("1", 1, 10),
	"activerehashing":                     boolParam("yes"),
	"always-show-logo":                    immutableParam("no"),
	"aof-load-truncated":                  boolParam("yes"),
	"aof-rewrite-cpulist":                 immutableParam(""),
	"aof-rewrite-incremental-fsync":       boolParam("yes"),
	"aof-timestamp-enabled":               boolParam("no"),
	"aof-use-rdb-preamble":                boolParam("yes"),
	"appenddirname":                       immutableParam("appendonlydir"),
	"bgsave-cpulist":                      immutableParam(""),
	"bind-source-addr":                    stringParam(""),
	"bio-cpulist":                         immutableParam(""),
	"busy-reply-threshold":                intParam("5000", 0, configLongMax),
	"client-output-buffer-limit":          {kind: cfgSpecial, def: defaultOutputBufferLimit, check: checkOutputBufferLimit, applyText: applyOutputBufferLimit},
	"client-query-buffer-limit":           {kind: cfgMemory, def: "1073741824", lo: 1 << 20, hi: configLongMax, apply: resp.SetQueryBufferLimit},
	"cluster-allow-pubsubshard-when-down": boolParam("yes"),
	"cluster-allow-reads-when-down":       boolParam("no"),
	"cluster-allow-replica-migration":     boolParam("yes"),
	"cluster-announce-bus-port":           intParam("0", 0, 65535),
	"cluster-announce-hostname":           {kind: cfgString, check: checkAnnounceHostname},
	"cluster-announce-human-nodename":     {kind: cfgString, check: checkHumanNodename},
	"cluster-announce-ip":                 {kind: cfgString, check: checkAnnounceIP},
	"cluster-announce-port":               intParam("0", 0, 65535),
	"cluster-announce-tls-port":           intParam("0", 0, 65535),
	"cluster-config-file":                 immutableParam("nodes.conf"),
	"cluster-enabled":                     immutableParam("no"),
	"cluster-link-sendbuf-limit":          {kind: cfgMemory, def: "0", lo: 0, hi: configLongMax, unsigned: true},
	"cluster-migration-barrier":           intParam("1", 0, configIntMax),
	"cluster-node-timeout":                intParam("15000", 0, configLongMax),
	"cluster-port":                        immutableParam("0"),
	"cluster-preferred-endpoint-type":     enumParam("ip", "ip", "hostname", "unknown-endpoint"),
	"cluster-replica-no-failover":         boolParam("no"),
	"cluster-replica-validity-factor":     intParam("10", 0, configIntMax),
	"cluster-require-full-coverage":       boolParam("yes"),
	"crash-log-enabled":                   boolParam("yes"),
	"crash-memcheck-enabled":              boolParam("yes"),
	"disable-thp":                         immutableParam("yes"),
	"dynamic-hz":                          boolParam("yes"),
	"enable-debug-command":                immutableParam("no"),
	"enable-module-command":               immutableParam("no"),
	"enable-protected-configs":            immutableParam("no"),
	"hide-user-data-from-log":             boolParam("no"),
	"hll-sparse-max-bytes":                {kind: cfgMemory, def: "3000", lo: 0, hi: configLongMax, apply: store.SetHLLSparseMaxBytes},
	"hz":                                  {kind: cfgSpecial, def: "10", check: checkHz},
	"ignore-warnings":                     stringParam(""),
	"io-threads":                          immutableParam("1"),
	"io-threads-do-reads":                 immutableParam("no"),
	"jemalloc-bg-thread":                  boolParam("yes"),
	"lazyfree-lazy-user-del":              boolParam("no"),
	"lazyfree-lazy-user-flush":            boolParam("no"),
	"list-compress-depth":                 intParam("0", 0, configIntMax),
	"locale-collate":                      {kind: cfgString, check: checkLocale},
	"logfile":                             immutableParam(""),
	"masterauth":                          stringParam(""),
	"masteruser":                          stringParam(""),
	"max-new-connections-per-cycle":       intParam("10", 1, 1000),
	"max-new-tls-connections-per-cycle":   intParam("1", 1, 1000),
	"maxmemory-clients":                   {kind: cfgSpecial, def: "0", check: checkMaxmemoryClients},
	"maxmemory-eviction-tenacity":         intParam("10", 0, 100),
	"min-replicas-max-lag":                intParam("10", 0, configIntMax),
	"min-replicas-to-write":               intParam("0", 0, configIntMax),
	"no-appendfsync-on-rewrite":           boolParam("no"),
	"oom-score-adj":                       enumParam("no", "no", "yes", "relative", "absolute"),
	"oom-score-adj-values":                {kind: cfgSpecial, def: "0 200 800", check: checkOOMScoreAdjValues},
	"proc-title-template":                 {kind: cfgString, def: "{title} {listen-addr} {server-mode}", check: checkProcTitleTemplate},
	"propagation-error-behavior":          enumParam("ignore", "ignore", "panic", "panic-on-replicas"),
	"proto-max-bulk-len":                  {kind: cfgMemory, def: "536870912", lo: 1 << 20, hi: configLongMax, apply: resp.SetProtoMaxBulkLen},
	"rdb-del-sync-files":                  boolParam("no"),
	"rdb-save-incremental-fsync":          boolParam("yes"),
	"rdbchecksum":                         immutableParam("yes"),
	"rdbcompression":                      boolParam("yes"),
	"repl-backlog-size":                   memParam("1048576", 1, configLongMax),
	"repl-backlog-ttl":                    intParam("3600", 0, configLongMax),
	"repl-disable-tcp-nodelay":            boolParam("no"),
	"repl-diskless-load":                  enumParam("disabled", "disabled", "on-empty-db", "swapdb"),
	"repl-diskless-sync":                  boolParam("yes"),
	"repl-diskless-sync-delay":            intParam("5", 0, configIntMax),
	"repl-diskless-sync-max-replicas":     intParam("0", 0, configIntMax),
	"repl-ping-replica-period":            intParam("10", 1, configIntMax),
	"repl-timeout":                        intParam("60", 1, configIntMax),
	"replica-announce-ip":                 stringParam(""),
	"replica-announce-port":               intParam("0", 0, 65535),
	"replica-announced":                   boolParam("yes"),
	"replica-ignore-disk-write-errors":    boolParam("no"),
	"replica-ignore-maxmemory":            boolParam("yes"),
	"replica-lazy-flush":                  boolParam("no"),
	"replica-priority":                    intParam("100", 0, configIntMax),
	"replica-read-only":                   boolParam("yes"),
	"replica-serve-stale-data":            boolParam("yes"),
	"replicaof":                           immutableParam(""),
	"sanitize-dump-payload":               enumParam("no", "no", "yes", "clients"),
	"server-cpulist":                      immutableParam(""),
	"set-max-listpack-entries":            {kind: cfgInt, def: "128", lo: 0, hi: configLongMax, apply: setSetListpackEntries},
	"set-max-listpack-value":              {kind: cfgInt, def: "64", lo: 0, hi: configLongMax, apply: setSetListpackValue},
	"set-proc-title":                      immutableParam("yes"),
	"shutdown-on-sigint":                  {kind: cfgSpecial, def: "default", check: checkShutdownFlags},
	"shutdown-on-sigterm":                 {kind: cfgSpecial, def: "default", check: checkShutdownFlags},
	"shutdown-timeout":                    intParam("10", 0, configIntMax),
	"socket-mark-id":                      immutableParam("0"),
	"stop-writes-on-bgsave-error":         boolParam("yes"),
	"stream-node-max-bytes":               memParam("4096", 0, configLongMax),
	"stream-node-max-entries":             intParam("100", 0, configLongMax),
	"supervised":                          immutableParam("no"),
	"syslog-enabled":                      immutableParam("no"),
	"syslog-facility":                     immutableParam("local0"),
	"syslog-ident":                        immutableParam("redis"),
	"tcp-backlog":                         immutableParam("511"),
	"tls-auth-clients":                    enumParam("yes", "no", "yes", "optional"),
	"tls-ca-cert-dir":                     stringParam(""),
	"tls-ca-cert-file":                    stringParam(""),
	"tls-cert-file":                       stringParam(""),
	"tls-ciphers":                         stringParam(""),
	"tls-ciphersuites":                    stringParam(""),
	"tls-client-cert-file":                stringParam(""),
	"tls-client-key-file":                 stringParam(""),
	"tls-client-key-file-pass":            stringParam(""),
	"tls-cluster":                         {kind: cfgSpecial, def: "no", check: checkTLSFlag},
	"tls-dh-params-file":                  stringParam(""),
	"tls-key-file":                        stringParam(""),
	"tls-key-file-pass":                   stringParam(""),
	"tls-port":                            {kind: cfgSpecial, def: "0", check: checkTLSPort},
	"tls-prefer-server-ciphers":           boolParam("no"),
	"tls-protocols":                       stringParam(""),
	"tls-replication":                     {kind: cfgSpecial, def: "no", check: checkTLSFlag},
	"tls-session-cache-size":              intParam("20480", 0, configIntMax),
	"tls-session-cache-timeout":           intParam("300", 0, configIntMax),
	"tls-session-caching":                 boolParam("yes"),
	"tracking-table-max-keys":             intParam("1000000", 0, configLongMax),
	"unixsocket":                          immutableParam(""),
	"unixsocketperm":                      immutableParam("0"),
	"zset-max-listpack-value":             {kind: cfgMemory, def: "64", lo: 0, hi: configLongMax, apply: setZsetListpackValue},
}

// configAliases maps Redis' old parameter names to the names they alias.
var configAliases = map[string]string{
	"aof_rewrite_cpulist":           "aof-rewrite-cpulist",
	"bgsave_cpulist":                "bgsave-cpulist",
	"bio_cpulist":                   "bio-cpulist",
	"server_cpulist":                "server-cpulist",
	"cluster-slave-no-failover":     "cluster-replica-no-failover",
	"cluster-slave-validity-factor": "cluster-replica-validity-factor",
	"hash-max-ziplist-entries":      "hash-max-listpack-entries",
	"hash-max-ziplist-value":        "hash-max-listpack-value",
	"list-max-ziplist-size":         "list-max-listpack-size",
	"lua-time-limit":                "busy-reply-threshold",
	"min-slaves-max-lag":            "min-replicas-max-lag",
	"min-slaves-to-write":           "min-replicas-to-write",
	"repl-ping-slave-period":        "repl-ping-replica-period",
	"slave-announce-ip":             "replica-announce-ip",
	"slave-announce-port":           "replica-announce-port",
	"slave-ignore-maxmemory":        "replica-ignore-maxmemory",
	"slave-lazy-flush":              "replica-lazy-flush",
	"slave-priority":                "replica-priority",
	"slave-read-only":               "replica-read-only",
	"slave-serve-stale-data":        "replica-serve-stale-data",
	"slaveof":                       "replicaof",
	"zset-max-ziplist-entries":      "zset-max-listpack-entries",
	"zset-max-ziplist-value":        "zset-max-listpack-value",
}

func setSetListpackEntries(v int64) {
	c := globalConfig
	c.setMaxListpackEntries = v
	store.SetSetEncodingLimits(c.setMaxIntsetEntries, v, c.setMaxListpackValue)
}

func setSetListpackValue(v int64) {
	c := globalConfig
	c.setMaxListpackValue = v
	store.SetSetEncodingLimits(c.setMaxIntsetEntries, c.setMaxListpackEntries, v)
}

func setZsetListpackValue(v int64) {
	entries, _ := store.ZsetListpackLimits()
	store.SetZsetListpackLimits(entries, v)
}

// paramValueLocked is the value CONFIG GET shows for a registry parameter.
// The caller holds c.mu.
func (c *Config) paramValueLocked(name string, p *configParam) string {
	if v, ok := c.params[name]; ok {
		return v
	}
	return p.def
}

// setParamLocked applies CONFIG SET to a registry parameter. The caller holds
// c.mu.
func (c *Config) setParamLocked(name, value string, p *configParam) error {
	if p.immutable {
		return ConfigSetError(name, "can't set immutable config")
	}
	var shown string
	var n int64
	switch p.kind {
	case cfgBool:
		b, err := ParseConfigBool(name, value)
		if err != nil {
			return err
		}
		shown = boolStr(b)
	case cfgInt:
		v, err := ParseConfigInt(name, value, p.lo, p.hi)
		if err != nil {
			return err
		}
		n, shown = v, strconv.FormatInt(v, 10)
	case cfgMemory:
		v, ok := redisMemtoull(value)
		if !ok {
			return ConfigSetError(name, "argument must be a memory value")
		}
		if p.unsigned {
			shown = strconv.FormatUint(v, 10)
			break
		}
		if v < uint64(p.lo) || v > uint64(p.hi) {
			return ConfigSetError(name, "argument must be between "+strconv.FormatInt(p.lo, 10)+" and "+strconv.FormatInt(p.hi, 10)+" inclusive")
		}
		n, shown = int64(v), strconv.FormatUint(v, 10)
	case cfgEnum:
		v, err := ParseConfigEnum(name, value, p.enum...)
		if err != nil {
			return err
		}
		shown = v
	default:
		shown = value
		if p.check != nil {
			v, err := p.check(name, value)
			if err != nil {
				return err
			}
			shown = v
		}
	}
	if c.params == nil {
		c.params = make(map[string]string)
	}
	c.params[name] = shown
	if p.apply != nil {
		p.apply(n)
	}
	if p.applyText != nil {
		p.applyText(shown)
	}
	return nil
}

// redisMemtoull parses a memory value as Redis' memtoull does: a
// non-negative integer with an optional b, k, kb, m, mb, g or gb unit.
func redisMemtoull(s string) (uint64, bool) {
	lower := strings.ToLower(s)
	mul := uint64(1)
	for _, u := range []struct {
		suffix string
		mul    uint64
	}{{"gb", 1 << 30}, {"mb", 1 << 20}, {"kb", 1 << 10}, {"g", 1e9}, {"m", 1e6}, {"k", 1e3}, {"b", 1}} {
		if strings.HasSuffix(lower, u.suffix) {
			lower, mul = strings.TrimSuffix(lower, u.suffix), u.mul
			break
		}
	}
	if lower == "" || lower[0] == '-' || lower[0] == '+' {
		return 0, false
	}
	v, err := strconv.ParseUint(lower, 10, 64)
	if err != nil {
		// strtoull saturates.
		var numErr *strconv.NumError
		if !errors.As(err, &numErr) || numErr.Err != strconv.ErrRange {
			return 0, false
		}
		v = math.MaxUint64
	}
	if v > math.MaxUint64/mul {
		return math.MaxUint64, true
	}
	return v * mul, true
}

// ValidateConfigParam checks value for the CONFIG parameter name as CONFIG
// SET would, without setting it. It is for the server's CONFIG SET hook,
// which runs with the configuration locked.
func ValidateConfigParam(name, value string) error {
	p, ok := configParams[name]
	if !ok {
		return nil
	}
	c := globalConfig
	old, had := c.params[name]
	cp := *p
	cp.apply, cp.applyText = nil, nil
	if err := c.setParamLocked(name, value, &cp); err != nil {
		return err
	}
	if had {
		c.params[name] = old
	} else {
		delete(c.params, name)
	}
	return nil
}

func checkHz(name, value string) (string, error) {
	v, err := ParseConfigInt(name, value, 0, configIntMax)
	if err != nil {
		return "", err
	}
	return strconv.FormatInt(min(max(v, 1), 500), 10), nil
}

func checkTLSPort(name, value string) (string, error) {
	v, err := ParseConfigInt(name, value, 0, 65535)
	if err != nil {
		return "", err
	}
	if v != 0 {
		return "", ConfigSetError(name, "Unable to update TLS configuration. Check server logs.")
	}
	return "0", nil
}

func checkTLSFlag(name, value string) (string, error) {
	on, err := ParseConfigBool(name, value)
	if err != nil {
		return "", err
	}
	if on {
		return "", ConfigSetError(name, "Unable to update TLS configuration. Check server logs.")
	}
	return "no", nil
}

func checkAnnounceIP(name, value string) (string, error) {
	if value != "" && net.ParseIP(value) == nil {
		return "", ConfigSetError(name, "Cluster announce IP must be a valid IPv4 or IPv6 address")
	}
	return value, nil
}

func checkAnnounceHostname(name, value string) (string, error) {
	for _, c := range value {
		if !isAlnumRune(c) && c != '-' && c != '.' {
			return "", ConfigSetError(name, "Hostnames may only contain alphanumeric characters, hyphens or dots")
		}
	}
	return value, nil
}

func checkHumanNodename(name, value string) (string, error) {
	for _, c := range value {
		if !isAlnumRune(c) && !strings.ContainsRune("!#$%&()*+:;<>?@[]^{|}~-._", c) {
			return "", ConfigSetError(name, "Announced human node name contained invalid character")
		}
	}
	return value, nil
}

func isAlnumRune(c rune) bool {
	return c >= 'a' && c <= 'z' || c >= 'A' && c <= 'Z' || c >= '0' && c <= '9'
}

func checkLocale(name, value string) (string, error) {
	switch value {
	case "", "C", "POSIX", "C.UTF-8", "C.utf8":
		return value, nil
	}
	return "", ConfigSetError(name, "Invalid locale name")
}

func checkProcTitleTemplate(name, value string) (string, error) {
	invalid := ConfigSetError(name, "template format is invalid or contains unknown variables")
	if strings.TrimSpace(value) == "" {
		return "", invalid
	}
	rest := value
	for {
		i := strings.IndexAny(rest, "{}")
		if i < 0 {
			return value, nil
		}
		if rest[i] == '}' {
			return "", invalid
		}
		j := strings.IndexByte(rest[i:], '}')
		if j < 0 {
			return "", invalid
		}
		switch rest[i+1 : i+j] {
		case "title", "listen-addr", "server-mode", "port", "tls-port", "unixsocket", "config-file":
		default:
			return "", invalid
		}
		rest = rest[i+j+1:]
	}
}

func checkMaxmemoryClients(name, value string) (string, error) {
	if pct, ok := strings.CutSuffix(value, "%"); ok {
		v, err := strconv.ParseInt(pct, 10, 64)
		if err != nil || v < 0 {
			return "", ConfigSetError(name, "argument must be a memory or percent value")
		}
		if v > 100 {
			return "", ConfigSetError(name, "percentage argument must be less or equal to 100")
		}
		return strconv.FormatInt(v, 10) + "%", nil
	}
	v, ok := redisMemtoull(value)
	if !ok {
		return "", ConfigSetError(name, "argument must be a memory or percent value")
	}
	return strconv.FormatUint(v, 10), nil
}

func checkOOMScoreAdjValues(name, value string) (string, error) {
	fields := strings.Fields(value)
	if len(fields) != 3 {
		return "", ConfigSetError(name, "wrong number of arguments")
	}
	for i, f := range fields {
		v, err := strconv.ParseInt(f, 10, 64)
		if err != nil || v < -2000 || v > 2000 {
			return "", ConfigSetError(name, "Invalid oom-score-adj-values, elements must be between -2000 and 2000.")
		}
		fields[i] = strconv.FormatInt(v, 10)
	}
	return strings.Join(fields, " "), nil
}

func checkShutdownFlags(name, value string) (string, error) {
	order := []string{"save", "nosave", "now", "force"}
	set := make(map[string]bool)
	for _, f := range strings.Fields(strings.ToLower(value)) {
		switch f {
		case "default", "save", "nosave", "now", "force":
			set[f] = true
		default:
			return "", ConfigSetError(name, "argument(s) must be one of the following: default, save, nosave, now, force")
		}
	}
	if set["save"] && set["nosave"] {
		return "", ConfigSetError(name, "shutdown options SAVE and NOSAVE can't be used simultaneously")
	}
	var out []string
	for _, f := range order {
		if set[f] {
			out = append(out, f)
		}
	}
	if len(out) == 0 {
		return "default", nil
	}
	return strings.Join(out, " "), nil
}

// applyOutputBufferLimit hands the pubsub class's limits, as
// checkOutputBufferLimit normalised them, to the subscriber queues.
func applyOutputBufferLimit(v string) {
	f := strings.Fields(v)
	for i := 0; i+3 < len(f); i += 4 {
		if f[i] != "pubsub" {
			continue
		}
		hard, errH := strconv.ParseUint(f[i+1], 10, 64)
		soft, errS := strconv.ParseUint(f[i+2], 10, 64)
		secs, errT := strconv.ParseInt(f[i+3], 10, 64)
		if errH != nil || errS != nil || errT != nil {
			return
		}
		store.SetPubsubOutputLimits(int64(min(hard, math.MaxInt64)), int64(min(soft, math.MaxInt64)), secs)
	}
}

// checkOutputBufferLimit applies client-output-buffer-limit's class/hard/
// soft/seconds groups over the current value.
func checkOutputBufferLimit(name, value string) (string, error) {
	fields := strings.Fields(value)
	if len(fields)%4 != 0 {
		return "", ConfigSetError(name, "Wrong number of arguments in buffer limit configuration.")
	}
	cur, ok := globalConfig.params[name]
	if !ok {
		cur = defaultOutputBufferLimit
	}
	current := strings.Fields(cur)
	limits := map[string][3]string{}
	for i := 0; i+3 < len(current); i += 4 {
		limits[current[i]] = [3]string{current[i+1], current[i+2], current[i+3]}
	}
	for i := 0; i < len(fields); i += 4 {
		class := strings.ToLower(fields[i])
		switch class {
		case "normal", "pubsub":
		case "replica", "slave":
			class = "slave"
		default:
			return "", ConfigSetError(name, "Invalid client class specified in buffer limit configuration.")
		}
		hard, okH := redisMemtoull(fields[i+1])
		soft, okS := redisMemtoull(fields[i+2])
		secs, err := strconv.ParseInt(fields[i+3], 10, 64)
		if !okH || !okS || err != nil || secs < 0 {
			return "", ConfigSetError(name, "Error in hard, soft or soft_seconds setting in buffer limit configuration.")
		}
		limits[class] = [3]string{strconv.FormatUint(hard, 10), strconv.FormatUint(soft, 10), strconv.FormatInt(secs, 10)}
	}
	var out []string
	for _, class := range []string{"normal", "slave", "pubsub"} {
		l := limits[class]
		out = append(out, class, l[0], l[1], l[2])
	}
	return strings.Join(out, " "), nil
}
