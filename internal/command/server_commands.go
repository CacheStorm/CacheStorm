package command

import (
	"errors"
	"strconv"
	"strings"
	"time"

	"github.com/cachestorm/cachestorm/internal/resp"
	"github.com/cachestorm/cachestorm/internal/store"
)

func RegisterServerCommands(router *Router) {
	router.Register(&CommandDef{Name: "PING", Handler: cmdPING})
	router.Register(&CommandDef{Name: "ECHO", Handler: cmdECHO})
	router.Register(&CommandDef{Name: "QUIT", Handler: cmdQUIT})
	router.Register(&CommandDef{Name: "COMMAND", Handler: cmdCOMMAND})
	router.Register(&CommandDef{Name: "INFO", Handler: cmdINFO})
	router.Register(&CommandDef{Name: "DBSIZE", Handler: cmdDBSIZE})
	router.Register(&CommandDef{Name: "FLUSHDB", Handler: cmdFLUSHDB})
	router.Register(&CommandDef{Name: "FLUSHALL", Handler: cmdFLUSHALL})
	router.Register(&CommandDef{Name: "TIME", Handler: cmdTIME})
	router.Register(&CommandDef{Name: "AUTH", Handler: cmdAUTH})
	router.Register(&CommandDef{Name: "SCAN", Handler: cmdSCAN})
	router.Register(&CommandDef{Name: "HOTKEYS", Handler: cmdHOTKEYS})
	router.Register(&CommandDef{Name: "MEMINFO", Handler: cmdMEMINFO})
}

func RegisterClientCommands(router *Router) {
	router.Register(&CommandDef{Name: "CLIENT", Handler: cmdCLIENT})
}

func RegisterKeyCommands(router *Router) {
	router.Register(&CommandDef{Name: "EXPIRE", Handler: cmdEXPIRE})
	router.Register(&CommandDef{Name: "PEXPIRE", Handler: cmdPEXPIRE})
	router.Register(&CommandDef{Name: "EXPIREAT", Handler: cmdEXPIREAT})
	router.Register(&CommandDef{Name: "PEXPIREAT", Handler: cmdPEXPIREAT})
	router.Register(&CommandDef{Name: "TTL", Handler: cmdTTL})
	router.Register(&CommandDef{Name: "PTTL", Handler: cmdPTTL})
	router.Register(&CommandDef{Name: "PERSIST", Handler: cmdPERSIST})
	router.Register(&CommandDef{Name: "TYPE", Handler: cmdTYPE})
	router.Register(&CommandDef{Name: "RENAME", Handler: cmdRENAME})
	router.Register(&CommandDef{Name: "RENAMENX", Handler: cmdRENAMENX})
	router.Register(&CommandDef{Name: "KEYS", Handler: cmdKEYS})
	router.Register(&CommandDef{Name: "RANDOMKEY", Handler: cmdRANDOMKEY})
	router.Register(&CommandDef{Name: "UNLINK", Handler: cmdDEL})
}

func cmdPING(ctx *Context) error {
	if ctx.ArgCount() == 0 {
		return ctx.WriteSimpleString("PONG")
	}
	return ctx.WriteBulkBytes(ctx.Arg(0))
}

func cmdECHO(ctx *Context) error {
	if ctx.ArgCount() != 1 {
		return ctx.WriteError(ErrWrongArgCount)
	}
	return ctx.WriteBulkBytes(ctx.Arg(0))
}

func cmdQUIT(ctx *Context) error {
	ctx.WriteOK()
	return nil
}

func cmdCOMMAND(ctx *Context) error {
	return ctx.WriteArray([]*resp.Value{})
}

func cmdINFO(ctx *Context) error {
	var sb strings.Builder

	sb.WriteString("# Server\r\n")
	sb.WriteString("cachestorm_version:1.0.0\r\n")
	sb.WriteString("arch_bits:64\r\n")
	sb.WriteString("tcp_port:6380\r\n")
	sb.WriteString("\r\n")

	sb.WriteString("# Memory\r\n")
	sb.WriteString("used_memory:")
	sb.WriteString(strconv.FormatInt(ctx.Store.MemUsage(), 10))
	sb.WriteString("\r\n")
	sb.WriteString("\r\n")

	sb.WriteString("# Keyspace\r\n")
	sb.WriteString("db0:keys=")
	sb.WriteString(strconv.FormatInt(ctx.Store.KeyCount(), 10))
	sb.WriteString(",expires=0\r\n")

	return ctx.WriteBulkString(sb.String())
}

func cmdDBSIZE(ctx *Context) error {
	return ctx.WriteInteger(ctx.Store.KeyCount())
}

func cmdFLUSHDB(ctx *Context) error {
	ctx.Store.Flush()
	return ctx.WriteOK()
}

func cmdFLUSHALL(ctx *Context) error {
	ctx.Store.Flush()
	return ctx.WriteOK()
}

func cmdTIME(ctx *Context) error {
	now := time.Now()
	sec := now.Unix()
	usec := now.Nanosecond() / 1000

	return ctx.WriteArray([]*resp.Value{
		resp.BulkString(strconv.FormatInt(sec, 10)),
		resp.BulkString(strconv.FormatInt(int64(usec), 10)),
	})
}

func cmdEXPIRE(ctx *Context) error {
	if ctx.ArgCount() != 2 {
		return ctx.WriteError(ErrWrongArgCount)
	}

	key := ctx.ArgString(0)
	sec, err := strconv.ParseInt(ctx.ArgString(1), 10, 64)
	if err != nil {
		return ctx.WriteError(ErrNotInteger)
	}

	if ctx.Store.SetTTL(key, time.Duration(sec)*time.Second) {
		return ctx.WriteInteger(1)
	}
	return ctx.WriteInteger(0)
}

func cmdPEXPIRE(ctx *Context) error {
	if ctx.ArgCount() != 2 {
		return ctx.WriteError(ErrWrongArgCount)
	}

	key := ctx.ArgString(0)
	ms, err := strconv.ParseInt(ctx.ArgString(1), 10, 64)
	if err != nil {
		return ctx.WriteError(ErrNotInteger)
	}

	if ctx.Store.SetTTL(key, time.Duration(ms)*time.Millisecond) {
		return ctx.WriteInteger(1)
	}
	return ctx.WriteInteger(0)
}

func cmdEXPIREAT(ctx *Context) error {
	if ctx.ArgCount() != 2 {
		return ctx.WriteError(ErrWrongArgCount)
	}

	key := ctx.ArgString(0)
	ts, err := strconv.ParseInt(ctx.ArgString(1), 10, 64)
	if err != nil {
		return ctx.WriteError(ErrNotInteger)
	}

	expiresAt := time.Unix(ts, 0).UnixNano()
	if ctx.Store.SetExpiresAt(key, expiresAt) {
		return ctx.WriteInteger(1)
	}
	return ctx.WriteInteger(0)
}

func cmdPEXPIREAT(ctx *Context) error {
	if ctx.ArgCount() != 2 {
		return ctx.WriteError(ErrWrongArgCount)
	}

	key := ctx.ArgString(0)
	ts, err := strconv.ParseInt(ctx.ArgString(1), 10, 64)
	if err != nil {
		return ctx.WriteError(ErrNotInteger)
	}

	expiresAt := time.Unix(0, ts*int64(time.Millisecond)).UnixNano()
	if ctx.Store.SetExpiresAt(key, expiresAt) {
		return ctx.WriteInteger(1)
	}
	return ctx.WriteInteger(0)
}

func cmdTTL(ctx *Context) error {
	if ctx.ArgCount() != 1 {
		return ctx.WriteError(ErrWrongArgCount)
	}

	ttl := ctx.Store.TTL(ctx.ArgString(0))
	return ctx.WriteInteger(int64(ttl / time.Second))
}

func cmdPTTL(ctx *Context) error {
	if ctx.ArgCount() != 1 {
		return ctx.WriteError(ErrWrongArgCount)
	}

	ttl := ctx.Store.TTL(ctx.ArgString(0))
	return ctx.WriteInteger(int64(ttl / time.Millisecond))
}

func cmdPERSIST(ctx *Context) error {
	if ctx.ArgCount() != 1 {
		return ctx.WriteError(ErrWrongArgCount)
	}

	if ctx.Store.Persist(ctx.ArgString(0)) {
		return ctx.WriteInteger(1)
	}
	return ctx.WriteInteger(0)
}

func cmdTYPE(ctx *Context) error {
	if ctx.ArgCount() != 1 {
		return ctx.WriteError(ErrWrongArgCount)
	}

	dt := ctx.Store.Type(ctx.ArgString(0))
	if dt == 0 {
		return ctx.WriteSimpleString("none")
	}
	return ctx.WriteSimpleString(dt.String())
}

func cmdRENAME(ctx *Context) error {
	if ctx.ArgCount() != 2 {
		return ctx.WriteError(ErrWrongArgCount)
	}

	oldKey := ctx.ArgString(0)
	newKey := ctx.ArgString(1)

	entry, exists := ctx.Store.Get(oldKey)
	if !exists {
		return ctx.WriteError(store.ErrKeyNotFound)
	}

	ctx.Store.Delete(oldKey)
	ctx.Store.SetEntry(newKey, entry)

	return ctx.WriteOK()
}

func cmdRENAMENX(ctx *Context) error {
	if ctx.ArgCount() != 2 {
		return ctx.WriteError(ErrWrongArgCount)
	}

	oldKey := ctx.ArgString(0)
	newKey := ctx.ArgString(1)

	entry, exists := ctx.Store.Get(oldKey)
	if !exists {
		return ctx.WriteError(store.ErrKeyNotFound)
	}

	if ctx.Store.Exists(newKey) {
		return ctx.WriteInteger(0)
	}

	ctx.Store.Delete(oldKey)
	ctx.Store.SetEntry(newKey, entry)

	return ctx.WriteInteger(1)
}

func cmdKEYS(ctx *Context) error {
	if ctx.ArgCount() != 1 {
		return ctx.WriteError(ErrWrongArgCount)
	}

	pattern := ctx.ArgString(0)
	keys := ctx.Store.Keys()

	matched := make([]*resp.Value, 0)
	for _, key := range keys {
		if matchPattern(key, pattern) {
			matched = append(matched, resp.BulkString(key))
		}
	}

	return ctx.WriteArray(matched)
}

func cmdRANDOMKEY(ctx *Context) error {
	keys := ctx.Store.Keys()
	if len(keys) == 0 {
		return ctx.WriteNullBulkString()
	}

	idx := time.Now().UnixNano() % int64(len(keys))
	return ctx.WriteBulkString(keys[idx])
}

func matchPattern(s, pattern string) bool {
	if pattern == "*" {
		return true
	}

	si, pi := 0, 0
	starIdx, match := -1, 0

	for si < len(s) {
		if pi < len(pattern) && (pattern[pi] == '?' || pattern[pi] == s[si]) {
			si++
			pi++
		} else if pi < len(pattern) && pattern[pi] == '*' {
			starIdx = pi
			match = si
			pi++
		} else if starIdx != -1 {
			pi = starIdx + 1
			match++
			si = match
		} else {
			return false
		}
	}

	for pi < len(pattern) && pattern[pi] == '*' {
		pi++
	}

	return pi == len(pattern)
}

func cmdAUTH(ctx *Context) error {
	if ctx.ArgCount() < 1 {
		return ctx.WriteError(ErrWrongArgCount)
	}

	password := ctx.ArgString(0)

	_ = password

	ctx.SetAuthenticated(true)
	return ctx.WriteOK()
}

func cmdSCAN(ctx *Context) error {
	if ctx.ArgCount() < 1 {
		return ctx.WriteError(ErrWrongArgCount)
	}

	cursor, err := strconv.Atoi(ctx.ArgString(0))
	if err != nil {
		return ctx.WriteError(ErrNotInteger)
	}

	count := 10
	pattern := "*"
	match := ""

	for i := 1; i < ctx.ArgCount(); i++ {
		arg := strings.ToUpper(ctx.ArgString(i))
		switch arg {
		case "COUNT":
			i++
			if i >= ctx.ArgCount() {
				return ctx.WriteError(ErrSyntaxError)
			}
			count, err = strconv.Atoi(ctx.ArgString(i))
			if err != nil {
				return ctx.WriteError(ErrNotInteger)
			}
		case "MATCH":
			i++
			if i >= ctx.ArgCount() {
				return ctx.WriteError(ErrSyntaxError)
			}
			pattern = ctx.ArgString(i)
		default:
			return ctx.WriteError(ErrSyntaxError)
		}
	}

	keys := ctx.Store.Keys()

	filtered := make([]string, 0)
	for _, key := range keys {
		if matchPattern(key, pattern) {
			filtered = append(filtered, key)
		}
	}

	start := cursor
	if start >= len(filtered) {
		start = 0
	}

	end := start + count
	if end > len(filtered) {
		end = len(filtered)
	}

	nextCursor := 0
	if end < len(filtered) {
		nextCursor = end
	}

	result := make([]*resp.Value, 0, end-start)
	for i := start; i < end; i++ {
		result = append(result, resp.BulkString(filtered[i]))
	}

	_ = match

	return ctx.WriteArray([]*resp.Value{
		resp.BulkString(strconv.Itoa(nextCursor)),
		resp.ArrayValue(result),
	})
}

func cmdHOTKEYS(ctx *Context) error {
	count := 10
	if ctx.ArgCount() >= 1 {
		var err error
		count, err = strconv.Atoi(ctx.ArgString(0))
		if err != nil {
			return ctx.WriteError(ErrNotInteger)
		}
	}

	keys := ctx.Store.Keys()
	type hotKey struct {
		key    string
		access uint64
	}

	hotKeys := make([]hotKey, 0, len(keys))
	for _, key := range keys {
		entry, exists := ctx.Store.Get(key)
		if exists {
			hotKeys = append(hotKeys, hotKey{key: key, access: entry.AccessCount})
		}
	}

	for i := 0; i < len(hotKeys)-1; i++ {
		for j := i + 1; j < len(hotKeys); j++ {
			if hotKeys[j].access > hotKeys[i].access {
				hotKeys[i], hotKeys[j] = hotKeys[j], hotKeys[i]
			}
		}
	}

	if count > len(hotKeys) {
		count = len(hotKeys)
	}

	result := make([]*resp.Value, 0, count)
	for i := 0; i < count; i++ {
		result = append(result, resp.ArrayValue([]*resp.Value{
			resp.BulkString(hotKeys[i].key),
			resp.IntegerValue(int64(hotKeys[i].access)),
		}))
	}

	return ctx.WriteArray(result)
}

func cmdMEMINFO(ctx *Context) error {
	var sb strings.Builder

	sb.WriteString("# Memory\r\n")
	sb.WriteString("used_memory:")
	sb.WriteString(strconv.FormatInt(ctx.Store.MemUsage(), 10))
	sb.WriteString("\r\n")
	sb.WriteString("keys:")
	sb.WriteString(strconv.FormatInt(ctx.Store.KeyCount(), 10))
	sb.WriteString("\r\n")

	if ctx.Store.KeyCount() > 0 {
		avgSize := ctx.Store.MemUsage() / ctx.Store.KeyCount()
		sb.WriteString("avg_entry_size:")
		sb.WriteString(strconv.FormatInt(avgSize, 10))
		sb.WriteString("\r\n")
	}

	tagIndex := ctx.Store.GetTagIndex()
	if tagIndex != nil {
		tags := tagIndex.Tags()
		sb.WriteString("tags:")
		sb.WriteString(strconv.Itoa(len(tags)))
		sb.WriteString("\r\n")
	}

	return ctx.WriteBulkString(sb.String())
}

func cmdCLIENT(ctx *Context) error {
	if ctx.ArgCount() < 1 {
		return ctx.WriteError(ErrWrongArgCount)
	}

	subCmd := strings.ToUpper(ctx.ArgString(0))

	switch subCmd {
	case "LIST":
		return ctx.WriteBulkString("id=1 addr=127.0.0.1:0 name= age=0 idle=0\n")
	case "SETNAME":
		if ctx.ArgCount() != 2 {
			return ctx.WriteError(ErrWrongArgCount)
		}
		return ctx.WriteOK()
	case "GETNAME":
		return ctx.WriteNullBulkString()
	case "ID":
		return ctx.WriteInteger(ctx.ClientID)
	default:
		return ctx.WriteError(errors.New("ERR unknown subcommand '" + subCmd + "'"))
	}
}
