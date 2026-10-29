package command

import (
	"crypto/rand"
	"encoding/hex"
	"errors"
	"math"
	"sort"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/cachestorm/cachestorm/internal/acl"
	"github.com/cachestorm/cachestorm/internal/resp"
	"github.com/cachestorm/cachestorm/internal/store"
	buildversion "github.com/cachestorm/cachestorm/internal/version"
)

var globalACL = acl.NewACL()

func RegisterServerCommands(router *Router) {
	router.Register(&CommandDef{Name: "PING", Handler: cmdPING})
	router.Register(&CommandDef{Name: "ECHO", Handler: cmdECHO})
	router.Register(&CommandDef{Name: "QUIT", Handler: cmdQUIT})
	router.Register(&CommandDef{Name: "RESET", Handler: cmdRESET})
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
	router.Register(&CommandDef{Name: "SORT", Handler: cmdSORT})
	router.Register(&CommandDef{Name: "SORT_RO", Handler: cmdSORTRO})
	router.Register(&CommandDef{Name: "SLOWLOG", Handler: cmdSLOWLOG})
	router.Register(&CommandDef{Name: "LASTSAVE", Handler: cmdLASTSAVE})
	router.Register(&CommandDef{Name: "LOLWUT", Handler: cmdLOLWUT})
	router.Register(&CommandDef{Name: "SHUTDOWN", Handler: cmdSHUTDOWN})
	router.Register(&CommandDef{Name: "SAVE", Handler: cmdSAVE})
	router.Register(&CommandDef{Name: "BGSAVE", Handler: cmdBGSAVE})
	router.Register(&CommandDef{Name: "BGREWRITEAOF", Handler: cmdBGREWRITEAOF})
	// SLAVEOF and REPLICAOF used to be registered here pointing at cmdSLAVEOF,
	// a stub that discards both arguments and replies +OK unconditionally.
	// Router.Register is last-write-wins, so those entries only worked because
	// RegisterReplicationCommands happens to run later in
	// internal/server/server.go and overwrote them with the functional
	// cmdREPLICAOF. Reorder those two lines and both commands would silently
	// resolve to the stub and stop configuring replication, with no error at
	// all. The functional handler owns both names; do not register them twice.
	router.Register(&CommandDef{Name: "LATENCY", Handler: cmdLATENCY})
	router.Register(&CommandDef{Name: "ACL", Handler: cmdACL})
	router.Register(&CommandDef{Name: "MONITOR", Handler: cmdMONITOR})
	router.Register(&CommandDef{Name: "SWAPDB", Handler: cmdSWAPDB})
	RegisterHelloCommands(router)
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
	router.Register(&CommandDef{Name: "PEXPIRETIME", Handler: cmdPEXPIRETIME})
	router.Register(&CommandDef{Name: "EXPIRETIME", Handler: cmdEXPIRETIME})
	router.Register(&CommandDef{Name: "PERSIST", Handler: cmdPERSIST})
	router.Register(&CommandDef{Name: "TYPE", Handler: cmdTYPE})
	router.Register(&CommandDef{Name: "RENAME", Handler: cmdRENAME})
	router.Register(&CommandDef{Name: "RENAMENX", Handler: cmdRENAMENX})
	router.Register(&CommandDef{Name: "KEYS", Handler: cmdKEYS})
	router.Register(&CommandDef{Name: "RANDOMKEY", Handler: cmdRANDOMKEY})
	router.Register(&CommandDef{Name: "UNLINK", Handler: cmdDEL})
	router.Register(&CommandDef{Name: "TOUCH", Handler: cmdTOUCH})
	router.Register(&CommandDef{Name: "DUMP", Handler: cmdDUMP})
	router.Register(&CommandDef{Name: "RESTORE", Handler: cmdRESTORE})
	router.Register(&CommandDef{Name: "RESTORE-ASKING", Handler: cmdRESTORE})
	router.Register(&CommandDef{Name: "COPY", Handler: cmdCOPY})
	router.Register(&CommandDef{Name: "MOVE", Handler: cmdMOVE})
	router.Register(&CommandDef{Name: "WAITAOF", Handler: cmdWAITAOF})
}

func cmdPING(ctx *Context) error {
	if ctx.ArgCount() > 1 {
		return ctx.WriteError(arityError("PING"))
	}
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

// cmdRESET returns the connection to the state of a new one, as Redis'
// clearClientConnectionState does. The client library name and version stay.
func cmdRESET(ctx *Context) error {
	if ctx.ArgCount() != 0 {
		return ctx.WriteError(arityError("reset"))
	}
	if ctx.Transaction != nil {
		ctx.Transaction.Clear()
		ctx.Transaction.ClearWatch()
	}
	if h := trackingHooksFor(ctx); h != nil && h.Disable != nil {
		h.Disable(ctx.ClientID)
	}
	if sub := ctx.Subscriber; sub != nil {
		if ps := ctx.Store.GetPubSub(); ps != nil {
			ps.UnsubscribeChannels(sub)
			ps.UnsubscribePatterns(sub)
			ps.SUnsubscribe(sub)
		}
	}
	ctx.DB = 0
	ctx.ACLUser = nil
	ctx.Username = ""
	ctx.SetAuthenticated(ctx.dispatcher() == nil || ctx.dispatcher().RequirePass() == "")
	ctx.ClientName = ""
	ctx.ReplyMode = "on"
	ctx.ResetConnection = true
	ctx.Writer.SetRESP3(false)
	return ctx.Writer.WriteSimpleString("RESET")
}

func cmdCOMMAND(ctx *Context) error {
	if ctx.ArgCount() == 0 {
		return cmdCommandInfo(ctx, nil)
	}
	if !checkRedisSubcommand(ctx, "command", nil) {
		return nil
	}
	switch strings.ToUpper(ctx.ArgString(0)) {
	case "COUNT":
		return ctx.WriteInteger(int64(len(registeredCommandNames(ctx))))
	case "DOCS":
		return cmdCommandDocs(ctx)
	case "GETKEYS":
		return cmdCommandGetKeys(ctx, false)
	case "GETKEYSANDFLAGS":
		return cmdCommandGetKeys(ctx, true)
	case "INFO":
		names := make([]string, 0, ctx.ArgCount()-1)
		for i := 1; i < ctx.ArgCount(); i++ {
			names = append(names, ctx.ArgString(i))
		}
		if len(names) == 0 {
			names = nil
		}
		return cmdCommandInfo(ctx, names)
	case "LIST":
		return cmdCommandList(ctx)
	default:
		return ctx.WriteArray(helpReply(commandHelp))
	}
}

func cmdINFO(ctx *Context) error {
	r := ctx.dispatcher()
	sections := make(map[string]bool)
	all, defaults := false, ctx.ArgCount() == 0
	for i := 0; i < ctx.ArgCount(); i++ {
		switch name := strings.ToLower(ctx.ArgString(i)); name {
		case "all", "everything":
			all = true
		case "default":
			defaults = true
		default:
			sections[name] = true
		}
	}
	want := func(name string) bool {
		if all || sections[name] {
			return true
		}
		return defaults && name != "commandstats" && name != "latencystats"
	}

	port := globalConfig.port
	if r != nil && r.Port() > 0 {
		port = r.Port()
	}

	var st ServerStatus
	if r != nil {
		st = r.getServerStatus()
	} else {
		st.Started = store.GlobalMetrics.StartTime
	}
	var w infoWriter
	if want("server") {
		infoServer(&w, &st, port, ctx.serverMode())
	}
	if want("clients") {
		infoClients(&w, &st, ctx)
	}
	if want("memory") {
		infoMemory(&w, ctx)
	}
	sb := &w.sb

	if want("persistence") && r != nil {
		_, _, info := r.snapshotHooks()
		if info != nil {
			sb.WriteString(info())
		}
	}

	if want("stats") {
		infoStats(&w, &st, ctx)
	}

	if want("replication") {
		var fn func() string
		if r != nil {
			fn = r.replicationInfo()
		}
		if fn != nil {
			sb.WriteString(fn())
		} else {
			sb.WriteString("# Replication\r\nrole:master\r\nconnected_slaves:0\r\n\r\n")
		}
	}

	if want("cpu") {
		infoCPU(&w)
	}

	if want("modules") {
		w.section("Modules")
		w.end()
	}

	if want("commandstats") {
		infoCommandStats(&w)
	}

	if want("errorstats") {
		infoErrorStats(&w)
	}

	if want("latencystats") {
		infoLatencyStats(&w)
	}

	if want("cluster") {
		if ctx.clusterState() != nil {
			sb.WriteString("# Cluster\r\ncluster_enabled:1\r\n\r\n")
		} else {
			sb.WriteString("# Cluster\r\ncluster_enabled:0\r\n\r\n")
		}
	}

	if want("keyspace") {
		sb.WriteString(keyspaceInfo(ctx))
	}

	// Sections are separated by a blank line, with none after the last.
	return ctx.WriteVerbatim(strings.TrimSuffix(sb.String(), "\r\n"))
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
	return expireGeneric(ctx, time.Now().UnixMilli(), true, "expire")
}

func cmdPEXPIRE(ctx *Context) error {
	return expireGeneric(ctx, time.Now().UnixMilli(), false, "pexpire")
}

func cmdEXPIREAT(ctx *Context) error {
	return expireGeneric(ctx, 0, true, "expireat")
}

func cmdPEXPIREAT(ctx *Context) error {
	return expireGeneric(ctx, 0, false, "pexpireat")
}

const (
	maxExpireAtSeconds = 9223372036    // MaxInt64 / time.Second
	maxExpireAtMillis  = 9223372036854 // MaxInt64 / time.Millisecond
)

var errInvalidExpireTime = errors.New("ERR invalid expire time")

const (
	expireFlagNX = 1 << iota
	expireFlagXX
	expireFlagGT
	expireFlagLT
)

// parseExpireFlags is Redis' parseExtendedExpireArgumentsOrReply.
func parseExpireFlags(args [][]byte) (int, error) {
	flags := 0
	for _, a := range args {
		switch strings.ToUpper(string(a)) {
		case "NX":
			flags |= expireFlagNX
		case "XX":
			flags |= expireFlagXX
		case "GT":
			flags |= expireFlagGT
		case "LT":
			flags |= expireFlagLT
		default:
			return 0, errors.New("ERR Unsupported option " + string(a))
		}
	}
	if flags&expireFlagNX != 0 && flags&(expireFlagXX|expireFlagGT|expireFlagLT) != 0 {
		return 0, errors.New("ERR NX and XX, GT or LT options at the same time are not compatible")
	}
	if flags&expireFlagGT != 0 && flags&expireFlagLT != 0 {
		return 0, errors.New("ERR GT and LT options at the same time are not compatible")
	}
	return flags, nil
}

// expireGeneric is Redis' expireGenericCommand: the instant is computed in
// milliseconds with overflow checks, NX/XX/GT/LT compare it against the
// current expiry (no expiry counts as infinite), and an instant that is not
// in the future deletes the key.
func expireGeneric(ctx *Context, basetimeMs int64, seconds bool, cmd string) error {
	if ctx.ArgCount() < 2 {
		return ctx.WriteError(ErrWrongArgCount)
	}
	key := ctx.ArgString(0)
	flags, err := parseExpireFlags(ctx.Args[2:])
	if err != nil {
		return ctx.WriteError(err)
	}
	when, ok := parseRedisInt(ctx.Arg(1))
	if !ok {
		return ctx.WriteError(ErrNotInteger)
	}
	if seconds {
		if when > math.MaxInt64/1000 || when < math.MinInt64/1000 {
			return ctx.WriteError(errInvalidExpireIn(cmd))
		}
		when *= 1000
	}
	if when > math.MaxInt64-basetimeMs {
		return ctx.WriteError(errInvalidExpireIn(cmd))
	}
	when += basetimeMs

	entry, exists := ctx.Store.Get(key)
	if !exists {
		ctx.PropagateAs()
		return ctx.WriteInteger(0)
	}
	if flags != 0 {
		current := entry.ExpiresAt.Load()
		volatile := current != 0
		if flags&expireFlagNX != 0 && volatile {
			ctx.PropagateAs()
			return ctx.WriteInteger(0)
		}
		if flags&expireFlagXX != 0 && !volatile {
			ctx.PropagateAs()
			return ctx.WriteInteger(0)
		}
		if flags&expireFlagGT != 0 && (!volatile || when <= current) {
			ctx.PropagateAs()
			return ctx.WriteInteger(0)
		}
		if flags&expireFlagLT != 0 && volatile && when >= current {
			ctx.PropagateAs()
			return ctx.WriteInteger(0)
		}
	}

	if when <= time.Now().UnixMilli() && !ctx.Replaying {
		ctx.Store.Delete(key)
		ctx.propagateDelete(key)
		ctx.notify(store.NotifyGeneric, "del", key)
		return ctx.WriteInteger(1)
	}
	if ctx.Store.SetExpiresAt(key, when) {
		ctx.propagateExpiry(key, when)
		ctx.notify(store.NotifyGeneric, "expire", key)
		return ctx.WriteInteger(1)
	}
	ctx.PropagateAs()
	return ctx.WriteInteger(0)
}

func cmdTTL(ctx *Context) error {
	if ctx.ArgCount() != 1 {
		return ctx.WriteError(ErrWrongArgCount)
	}

	ttl := ctx.Store.TTLMillis(ctx.ArgString(0))
	if ttl < 0 {
		return ctx.WriteInteger(ttl)
	}
	return ctx.WriteInteger((ttl + 500) / 1000)
}

func cmdPTTL(ctx *Context) error {
	if ctx.ArgCount() != 1 {
		return ctx.WriteError(ErrWrongArgCount)
	}

	return ctx.WriteInteger(ctx.Store.TTLMillis(ctx.ArgString(0)))
}

func cmdPEXPIRETIME(ctx *Context) error {
	if ctx.ArgCount() != 1 {
		return ctx.WriteError(ErrWrongArgCount)
	}
	key := ctx.ArgString(0)
	entry, exists := ctx.Store.Peek(key)
	if !exists {
		return ctx.WriteInteger(-2)
	}
	if entry.ExpiresAt.Load() == 0 {
		return ctx.WriteInteger(-1)
	}
	return ctx.WriteInteger(entry.ExpiresAt.Load())
}

func cmdEXPIRETIME(ctx *Context) error {
	if ctx.ArgCount() != 1 {
		return ctx.WriteError(ErrWrongArgCount)
	}
	key := ctx.ArgString(0)
	entry, exists := ctx.Store.Peek(key)
	if !exists {
		return ctx.WriteInteger(-2)
	}
	if entry.ExpiresAt.Load() == 0 {
		return ctx.WriteInteger(-1)
	}
	at := entry.ExpiresAt.Load()
	seconds := at / 1000
	if at%1000 >= 500 {
		seconds++
	}
	return ctx.WriteInteger(seconds)
}

func cmdPERSIST(ctx *Context) error {
	if ctx.ArgCount() != 1 {
		return ctx.WriteError(ErrWrongArgCount)
	}

	if key := ctx.ArgString(0); ctx.Store.Persist(key) {
		ctx.notify(store.NotifyGeneric, "persist", key)
		return ctx.WriteInteger(1)
	}
	ctx.PropagateAs()
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

var errNoSuchKey = errors.New("ERR no such key")

func cmdRENAME(ctx *Context) error {
	ctx.writeEffectsHandled = true
	if ctx.ArgCount() != 2 {
		return ctx.WriteError(ErrWrongArgCount)
	}
	changed, err := ctx.Store.Transfer(ctx.ArgString(0), ctx.Store, ctx.ArgString(1), store.TransferOptions{Replace: true})
	if err == store.ErrKeyNotFound {
		return ctx.WriteError(errNoSuchKey)
	}
	if err != nil {
		return ctx.WriteError(err)
	}
	if !changed {
		ctx.PropagateAs()
	} else {
		notifyRename(ctx, ctx.ArgString(0), ctx.ArgString(1))
	}
	return ctx.WriteOK()
}

func cmdRENAMENX(ctx *Context) error {
	ctx.writeEffectsHandled = true
	if ctx.ArgCount() != 2 {
		return ctx.WriteError(ErrWrongArgCount)
	}
	changed, err := ctx.Store.Transfer(ctx.ArgString(0), ctx.Store, ctx.ArgString(1), store.TransferOptions{})
	if err == store.ErrKeyNotFound {
		return ctx.WriteError(errNoSuchKey)
	}
	if err != nil {
		return ctx.WriteError(err)
	}
	if !changed {
		ctx.PropagateAs()
		return ctx.WriteInteger(0)
	}
	notifyRename(ctx, ctx.ArgString(0), ctx.ArgString(1))
	return ctx.WriteInteger(1)
}

func notifyRename(ctx *Context, from, to string) {
	ctx.notify(store.NotifyGeneric, "rename_from", from)
	ctx.notify(store.NotifyGeneric, "rename_to", to)
}

func cmdKEYS(ctx *Context) error {
	if ctx.ArgCount() != 1 {
		return ctx.WriteError(ErrWrongArgCount)
	}

	const maxKeysResult = 100000 // Safety cap to prevent OOM on large databases

	pattern := ctx.ArgString(0)
	matched := make([]*resp.Value, 0)
	for key, entry := range ctx.Store.GetAll() {
		if entry.IsExpired() || !matchPattern(key, pattern) {
			continue
		}
		matched = append(matched, resp.BulkString(key))
		if len(matched) >= maxKeysResult {
			break
		}
	}

	return ctx.WriteArray(matched)
}

func cmdRANDOMKEY(ctx *Context) error {
	key, ok := ctx.Store.RandomKey()
	if !ok {
		return ctx.WriteNullBulkString()
	}
	return ctx.WriteBulkString(key)
}

// matchPattern is Redis' glob matcher (stringmatchlen): *, ?, [abc], [a-z],
// [^a] and backslash escapes.
func matchPattern(s, pattern string) bool {
	return store.MatchPattern(s, pattern)
}

func cmdTOUCH(ctx *Context) error {
	if ctx.ArgCount() < 1 {
		return ctx.WriteError(ErrWrongArgCount)
	}

	touched := int64(0)
	for i := 0; i < ctx.ArgCount(); i++ {
		key := ctx.ArgString(i)
		if _, exists := ctx.Store.Get(key); exists {
			touched++
		}
	}

	return ctx.WriteInteger(touched)
}

func cmdAUTH(ctx *Context) error {
	if ctx.ArgCount() < 1 {
		return ctx.WriteError(ErrWrongArgCount)
	}

	// AUTH <username> <password> selects an ACL user. This is checked BEFORE
	// the requirepass gate so ACL users can authenticate on a server that has
	// no requirepass configured.
	if ctx.ArgCount() >= 2 {
		username := ctx.ArgString(0)
		password := ctx.ArgString(1)

		user, err := globalACL.Authenticate(username, password)
		if err != nil {
			recordACLDenial(ctx, "auth", "AUTH", username)
			ctx.SetAuthenticated(false)
			ctx.ACLUser = nil
			ctx.Username = ""
			return ctx.Writer.WriteError("WRONGPASS invalid username-password pair or user is disabled.")
		}

		ctx.SetAuthenticated(true)
		ctx.ACLUser = user
		ctx.Username = username
		return ctx.WriteOK()
	}

	password := ctx.ArgString(0)

	// The router dispatching this command holds the password: several
	// servers can share a process, each with its own router.
	r := ctx.dispatcher()
	if r == nil || r.RequirePass() == "" {
		return ctx.Writer.WriteError("ERR Client sent AUTH, but no password is set. Did you mean ACL SETUSER with >password?")
	}

	if !r.ValidatePassword(password) {
		recordACLDenial(ctx, "auth", "AUTH", "default")
		ctx.SetAuthenticated(false)
		return ctx.Writer.WriteError("WRONGPASS invalid username-password pair or user is disabled.")
	}

	ctx.SetAuthenticated(true)
	return ctx.WriteOK()
}

// SCAN walks the keyspace in the order of a 63-bit hash of each key, and the
// cursor is one more than the hash where the next call resumes. A key that is
// present for the whole iteration is therefore returned regardless of what is
// added or removed in between, which an index into an unordered key list
// could not guarantee.
func cmdSCAN(ctx *Context) error {
	if ctx.ArgCount() < 1 {
		return ctx.WriteError(ErrWrongArgCount)
	}

	cursor, err := strconv.ParseUint(ctx.ArgString(0), 10, 64)
	if err != nil {
		return ctx.WriteError(errors.New("ERR invalid cursor"))
	}

	count := 10
	pattern := ""
	typeName := ""
	for i := 1; i < ctx.ArgCount(); i++ {
		arg := strings.ToUpper(ctx.ArgString(i))
		if i+1 >= ctx.ArgCount() {
			return ctx.WriteError(ErrSyntaxError)
		}
		switch arg {
		case "COUNT":
			i++
			n, ok := parseRedisInt(ctx.Arg(i))
			if !ok {
				return ctx.WriteError(ErrNotInteger)
			}
			if n < 1 {
				return ctx.WriteError(ErrSyntaxError)
			}
			count = int(min(n, 1<<30))
		case "MATCH":
			i++
			pattern = ctx.ArgString(i)
		case "TYPE":
			i++
			typeName = strings.ToLower(ctx.ArgString(i))
		default:
			return ctx.WriteError(ErrSyntaxError)
		}
	}

	keys, next := scanKeyspace(ctx.Store, cursor, count, typeName != "")
	result := make([]*resp.Value, 0, len(keys))
	for _, k := range keys {
		if pattern != "" && !matchPattern(k.key, pattern) {
			continue
		}
		if typeName != "" && redisTypeName(k.entry.Value) != typeName {
			continue
		}
		result = append(result, resp.BulkString(k.key))
	}

	return ctx.WriteArray([]*resp.Value{
		resp.BulkString(strconv.FormatUint(next, 10)),
		resp.ArrayValue(result),
	})
}

// SCAN walks the keyspace shard by shard. The cursor's top bits name the
// shard and the rest is the smallest key hash the shard still has to return
// (see store.ScanShard), so a key present for the whole scan is returned
// exactly once.
const keyScanPosMask = 1<<store.ScanPosBits - 1

type scannedKey struct {
	key   string
	entry *store.Entry
}

// scanKeyspace returns the keys of one SCAN page and the next cursor (0 when
// the scan is complete); entries are filled in when withEntries is set.
func scanKeyspace(st *store.Store, cursor uint64, count int, withEntries bool) ([]scannedKey, uint64) {
	shard, pos := int(cursor>>store.ScanPosBits), cursor&keyScanPosMask
	var out []scannedKey
	for shard < store.NumShards {
		keys, entries, nextPos := st.ScanShard(shard, pos, count-len(out), withEntries)
		for i, k := range keys {
			sk := scannedKey{key: k}
			if withEntries {
				sk.entry = entries[i]
			}
			out = append(out, sk)
		}
		if nextPos != 0 {
			return out, uint64(shard)<<store.ScanPosBits | nextPos
		}
		shard, pos = shard+1, 0
		if len(out) >= count {
			break
		}
	}
	if shard >= store.NumShards {
		return out, 0
	}
	return out, uint64(shard) << store.ScanPosBits
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
			hotKeys = append(hotKeys, hotKey{key: key, access: uint64(entry.LFUCount())})
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

	if !checkRedisSubcommand(ctx, "client", nil) {
		return nil
	}

	switch strings.ToUpper(ctx.ArgString(0)) {
	case "LIST":
		return cmdClientList(ctx)
	case "SETNAME":
		if ctx.ArgCount() != 2 {
			return ctx.WriteError(ErrWrongArgCount)
		}
		// Redis stores the name for this connection and echoes it from GETNAME.
		// This was a stub that replied +OK and discarded the value, so a named
		// client still reported no name -- and SLOWLOG GET element 6 could never
		// carry one. CLIENT SETNAME with an empty argument clears the name.
		if !validClientName(ctx.ArgString(1)) {
			return ctx.WriteError(errClientNameChars)
		}
		ctx.ClientName = ctx.ArgString(1)
		return ctx.WriteOK()
	case "GETNAME":
		if ctx.ClientName == "" {
			return ctx.WriteNullBulkString()
		}
		return ctx.WriteBulkString(ctx.ClientName)
	case "ID":
		return ctx.WriteInteger(ctx.ClientID)
	case "KILL":
		return cmdClientKill(ctx)
	case "PAUSE":
		return cmdClientPause(ctx)
	case "UNPAUSE":
		return cmdClientUnpause(ctx)
	case "UNBLOCK":
		return cmdClientUnblock(ctx)
	case "REPLY":
		return cmdClientReply(ctx)
	case "TRACKING":
		return cmdClientTracking(ctx)
	case "SETINFO":
		if ctx.ArgCount() != 3 {
			return ctx.WriteError(arityError("client|setinfo"))
		}
		attr, value := strings.ToUpper(ctx.ArgString(1)), ctx.ArgString(2)
		if attr != "LIB-NAME" && attr != "LIB-VER" {
			return ctx.WriteError(errors.New("ERR Unrecognized option '" + ctx.ArgString(1) + "'"))
		}
		if !validClientName(value) {
			return ctx.WriteError(errors.New("ERR " + ctx.ArgString(1) + " cannot contain spaces, newlines or special characters."))
		}
		if attr == "LIB-NAME" {
			ctx.ClientLibName = value
		} else {
			ctx.ClientLibVer = value
		}
		return ctx.WriteOK()
	case "CACHING":
		return cmdClientCaching(ctx)
	case "TRACKINGINFO":
		return cmdClientTrackingInfo(ctx)
	case "NO-TOUCH", "NO-EVICT":
		var flag int8
		switch strings.ToLower(ctx.ArgString(1)) {
		case "on":
			flag = 1
		case "off":
			flag = -1
		default:
			return ctx.WriteError(ErrSyntaxError)
		}
		if strings.EqualFold(ctx.ArgString(0), "NO-TOUCH") {
			ctx.ClientNoTouch = flag
		} else {
			ctx.ClientNoEvict = flag
		}
		return ctx.WriteOK()
	case "INFO":
		return cmdClientInfo(ctx)
	case "GETREDIR":
		if h := trackingHooksFor(ctx); h != nil {
			return ctx.WriteInteger(h.Redirect(ctx.ClientID))
		}
		return ctx.WriteInteger(-1)
	default:
		return ctx.WriteArray(helpReply(clientHelp))
	}
}

func sortStoreDelete(ctx *Context, key string) {
	if ctx.Store.Delete(key) {
		ctx.notify(store.NotifyGeneric, "del", key)
	}
}

func cmdSORT(ctx *Context) error {
	return doSort(ctx, false)
}

func cmdSORTRO(ctx *Context) error {
	return doSort(ctx, true)
}

// sortExpandPattern expands a SORT BY/GET pattern for one element: the first
// '*' is replaced by the element. A pattern with no '*' expands to nothing:
// Redis skips sorting for such a BY pattern and misses such a GET pattern.
func sortExpandPattern(pattern, elem string) (string, bool) {
	i := strings.IndexByte(pattern, '*')
	if i < 0 {
		return pattern, false
	}
	return pattern[:i] + elem + pattern[i+1:], true
}

// sortLookupValue reads the external key a BY/GET pattern expands to. A
// pattern with no '*' is a miss; hash-field patterns (key->field) resolve the
// named hash field. Only string values and hash fields are usable; anything
// else is a miss, which BY treats as weight 0 (skipping the sort when every
// element misses) and GET reports as a nil element.
func sortLookupValue(ctx *Context, pattern, elem string) (string, bool) {
	expanded, ok := sortExpandPattern(pattern, elem)
	if !ok {
		return "", false
	}
	lookup, field, hasField := strings.Cut(expanded, "->")
	entry, exists := ctx.Store.Get(lookup)
	if !exists {
		return "", false
	}
	if hasField {
		hv, ok := entry.Value.(*store.HashValue)
		if !ok {
			return "", false
		}
		// cmdHSET mutates Fields under the hash write lock.
		hv.RLock()
		v, ok := hv.Fields[field]
		hv.RUnlock()
		if !ok {
			return "", false
		}
		return string(v), true
	}
	data, ok := stringValueOf(entry.Value)
	if !ok {
		return "", false
	}
	return string(data), true
}

// sortProjectElements applies each GET clause to every element, in the order
// the clauses were given, so each element yields len(patterns) values.
func sortProjectElements(ctx *Context, elements []string, patterns []string) []*resp.Value {
	out := make([]*resp.Value, 0, len(elements)*len(patterns))
	for _, elem := range elements {
		for _, p := range patterns {
			if p == "#" {
				out = append(out, resp.BulkString(elem))
				continue
			}
			if v, ok := sortLookupValue(ctx, p, elem); ok {
				out = append(out, resp.BulkString(v))
			} else {
				out = append(out, resp.NullBulkString())
			}
		}
	}
	return out
}

func doSort(ctx *Context, readOnly bool) error {
	if ctx.ArgCount() < 1 {
		return ctx.WriteError(ErrWrongArgCount)
	}

	key := ctx.ArgString(0)
	entry, exists := ctx.Store.Get(key)
	if exists {
		switch entry.Value.(type) {
		case *store.ListValue, *store.SetValue, *store.SortedSetValue:
		default:
			return ctx.WriteError(store.ErrWrongType)
		}
	}

	desc := false
	alpha := false
	var offset, count int64 = 0, -1
	storeKey := ""
	byPattern := ""
	nosort := false
	var getPatterns []string

	for i := 1; i < ctx.ArgCount(); i++ {
		arg := strings.ToUpper(ctx.ArgString(i))
		left := ctx.ArgCount() - i - 1
		switch {
		case arg == "DESC":
			desc = true
		case arg == "ASC":
			desc = false
		case arg == "ALPHA":
			alpha = true
		case arg == "LIMIT" && left >= 2:
			var err error
			if offset, err = strconv.ParseInt(ctx.ArgString(i+1), 10, 64); err != nil {
				return ctx.WriteError(ErrNotInteger)
			}
			if count, err = strconv.ParseInt(ctx.ArgString(i+2), 10, 64); err != nil {
				return ctx.WriteError(ErrNotInteger)
			}
			i += 2
		case arg == "STORE" && left >= 1 && !readOnly:
			storeKey = ctx.ArgString(i + 1)
			i++
		case arg == "BY" && left >= 1:
			byPattern = ctx.ArgString(i + 1)
			// A pattern without '*' cannot name a weight per element, so
			// Redis does not sort at all.
			if !strings.Contains(byPattern, "*") {
				nosort = true
			}
			i++
		case arg == "GET" && left >= 1:
			getPatterns = append(getPatterns, ctx.ArgString(i+1))
			i++
		default:
			return ctx.WriteError(ErrSyntaxError)
		}
	}

	if !exists {
		if storeKey != "" {
			sortStoreDelete(ctx, storeKey)
			return ctx.WriteInteger(0)
		}
		return ctx.WriteArray([]*resp.Value{})
	}

	var elements []string
	isSet := false
	switch v := entry.Value.(type) {
	// The command handlers mutate these collections under the value write
	// lock (cmdRPUSH/cmdLPOP, cmdSADD/cmdSREM), so the copy must be taken
	// under the matching read lock. No Store call happens inside these arms,
	// so holding RLock across the copy cannot self-deadlock.
	case *store.ListValue:
		v.RLock()
		for _, elem := range v.Elements {
			elements = append(elements, string(elem))
		}
		v.RUnlock()
	case *store.SetValue:
		isSet = true
		v.RLock()
		v.Range(func(member string) bool {
			elements = append(elements, member)
			return true
		})
		v.RUnlock()
	case *store.SortedSetValue:
		entries := v.GetSortedRange(0, -1, false, false)
		for _, e := range entries {
			elements = append(elements, e.Member)
		}
	}
	// An unsorted set has no order of its own; Redis sorts it
	// lexicographically when the result is stored or read by a script, so
	// the outcome is deterministic.
	if nosort && isSet && (storeKey != "" || ctx.InScript) {
		nosort, alpha, byPattern = false, true, ""
	}
	if nosort && desc && !isSet {
		for i, j := 0, len(elements)-1; i < j; i, j = i+1, j-1 {
			elements[i], elements[j] = elements[j], elements[i]
		}
	}

	if !nosort {
		type sortItem struct {
			elem   string
			score  float64
			cmp    string
			hasCmp bool
		}
		items := make([]sortItem, len(elements))
		for i, elem := range elements {
			it := sortItem{elem: elem}
			val, ok := elem, true
			if byPattern != "" {
				val, ok = sortLookupValue(ctx, byPattern, elem)
			}
			if alpha {
				it.cmp, it.hasCmp = val, ok
			} else if ok && val != "" {
				f, err := strconv.ParseFloat(strings.TrimLeft(val, " \t\n\v\f\r"), 64)
				if err != nil || math.IsNaN(f) {
					return ctx.WriteError(errors.New("ERR One or more scores can't be converted into double"))
				}
				it.score = f
			}
			items[i] = it
		}
		sort.SliceStable(items, func(i, j int) bool {
			a, b := items[i], items[j]
			var c int
			switch {
			case !alpha:
				switch {
				case a.score < b.score:
					c = -1
				case a.score > b.score:
					c = 1
				default:
					c = strings.Compare(a.elem, b.elem)
				}
			case byPattern == "":
				c = strings.Compare(a.elem, b.elem)
			case !a.hasCmp || !b.hasCmp:
				switch {
				case a.hasCmp == b.hasCmp:
					c = 0
				case !a.hasCmp:
					c = -1
				default:
					c = 1
				}
			default:
				c = strings.Compare(a.cmp, b.cmp)
			}
			if desc {
				c = -c
			}
			return c < 0
		})
		for i, it := range items {
			elements[i] = it.elem
		}
	}

	n := int64(len(elements))
	start := max(offset, 0)
	end := n - 1
	if count >= 0 {
		end = start + count - 1
	}
	if start >= n {
		start, end = n-1, n-2
	}
	if end >= n {
		end = n - 1
	}
	if n == 0 || end < start {
		start, end = 0, -1
	}
	elements = elements[start : end+1]

	// GET <pattern>: project each element through the external key its pattern
	// expands to, after LIMIT, so paging is decided on the sorted elements.
	if len(getPatterns) > 0 {
		projected := sortProjectElements(ctx, elements, getPatterns)

		if storeKey != "" {
			if len(projected) == 0 {
				sortStoreDelete(ctx, storeKey)
				return ctx.WriteInteger(0)
			}
			list := &store.ListValue{Elements: make([][]byte, 0, len(projected))}
			for _, p := range projected {
				if p.Type == resp.TypeNull || p.IsNull {
					list.Elements = append(list.Elements, nil)
					continue
				}
				list.Elements = append(list.Elements, p.Bulk)
			}
			ctx.Store.Set(storeKey, list, store.SetOptions{})
			ctx.notify(store.NotifyList, "sortstore", storeKey)
			return ctx.WriteInteger(int64(len(projected)))
		}

		return ctx.WriteArray(projected)
	}

	if storeKey != "" {
		if len(elements) == 0 {
			sortStoreDelete(ctx, storeKey)
			return ctx.WriteInteger(0)
		}

		list := &store.ListValue{Elements: make([][]byte, 0, len(elements))}
		for _, elem := range elements {
			list.Elements = append(list.Elements, []byte(elem))
		}
		ctx.Store.Set(storeKey, list, store.SetOptions{})
		ctx.notify(store.NotifyList, "sortstore", storeKey)
		return ctx.WriteInteger(int64(len(elements)))
	}

	result := make([]*resp.Value, 0, len(elements))
	for _, elem := range elements {
		result = append(result, resp.BulkString(elem))
	}

	return ctx.WriteArray(result)
}

// lastSaveTime is the Unix second of the most recent SAVE/BGSAVE, reported by
// LASTSAVE. It is touched by three command handlers (LASTSAVE reads it and
// lazily initialises it; SAVE and BGSAVE write it), so it must be atomic -- it
// was a bare int64, and one client's SAVE raced another's LASTSAVE.
var lastSaveTime atomic.Int64

// saveHook performs the real persistence work behind SAVE and BGSAVE. The
// composition root installs it during New() -- before the listener starts --
// pointing at the configured AOF flush, so SAVE is an actual durability barrier.
//
// Without it both handlers only stamped lastSaveTime and replied +OK: a client
// that issued SAVE as a durability barrier got a false guarantee, and with
// aof_sync "no" the buffered AOF was never moved to disk at all (Append syncs
// only for AOFAlways and the background syncLoop runs only for
// AOFEverySecond). nil means persistence is not configured, and SAVE then says
// so instead of claiming a success it did not perform.
//
// Startup-only, exactly like ConfigureSentinel: installed once before traffic.
var saveHook func() error

// SetSaveHook installs the durability action SAVE and BGSAVE invoke.
func SetSaveHook(fn func() error) {
	saveHook = fn
}

// runSaveHook executes the installed durability action, reporting plainly when
// there is nothing configured to persist to.
func runSaveHook() error {
	if saveHook == nil {
		return errors.New("ERR persistence is not configured")
	}
	return saveHook()
}

type SlowLogEntry struct {
	ID        int64
	StartTime int64
	Duration  int64
	Command   string
	Args      []string
	ClientIP  string
	ClientID  int64
}

type SlowLog struct {
	entries   []SlowLogEntry
	mu        sync.RWMutex
	maxLen    int
	slowLogSl int64
	nextID    int64
}

var globalSlowLog = &SlowLog{
	entries:   make([]SlowLogEntry, 0),
	maxLen:    128,
	slowLogSl: 10000,
}

func (s *SlowLog) Add(command string, args []string, duration int64, clientIP string, clientID int64) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if duration < s.slowLogSl {
		return
	}

	entry := SlowLogEntry{
		ID:        s.nextID,
		StartTime: time.Now().UnixMicro(),
		Duration:  duration,
		Command:   command,
		Args:      args,
		ClientIP:  clientIP,
		ClientID:  clientID,
	}
	s.nextID++

	s.entries = append([]SlowLogEntry{entry}, s.entries...)
	if len(s.entries) > s.maxLen {
		s.entries = s.entries[:s.maxLen]
	}
}

func (s *SlowLog) Get(count int) []SlowLogEntry {
	s.mu.RLock()
	defer s.mu.RUnlock()

	if count < 0 {
		count = len(s.entries)
	}
	if count > len(s.entries) {
		count = len(s.entries)
	}

	result := make([]SlowLogEntry, count)
	copy(result, s.entries[:count])
	return result
}

func (s *SlowLog) Len() int {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return len(s.entries)
}

func (s *SlowLog) Reset() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.entries = make([]SlowLogEntry, 0)
}

func cmdSLOWLOG(ctx *Context) error {
	if ctx.ArgCount() < 1 {
		return ctx.WriteError(ErrWrongArgCount)
	}

	if !checkRedisSubcommand(ctx, "slowlog", nil) {
		return nil
	}

	switch strings.ToUpper(ctx.ArgString(0)) {
	case "GET":
		if ctx.ArgCount() > 2 {
			return ctx.WriteError(errSubcommandSyntax(ctx.ArgString(0), "SLOWLOG"))
		}
		count := 10
		if ctx.ArgCount() > 1 {
			var err error
			count, err = strconv.Atoi(ctx.ArgString(1))
			if err != nil || count < -1 {
				return ctx.WriteError(errors.New("ERR count should be greater than or equal to -1"))
			}
		}

		// Read the recorder that production actually writes to. This handler
		// used to read command.globalSlowLog, a second SlowLog type that
		// nothing in production ever appended to (internal/server/
		// connection.go records into store.GlobalSlowLog), so SLOWLOG GET
		// and LEN always reported zero and RESET cleared nothing.
		//
		// store.SlowLog.Get treats n <= 0 as "all entries", but SLOWLOG's
		// documented contract is the opposite for 0: a count of -1 means all
		// entries, while 0 means "at most count entries", i.e. an empty
		// array. Normalise here so delegating does not silently change it.
		var entries []store.SlowLogEntry
		switch {
		case count == 0:
			entries = nil
		case count < 0:
			entries = store.GlobalSlowLog.Get(-1)
		default:
			entries = store.GlobalSlowLog.Get(count)
		}
		results := make([]*resp.Value, 0, len(entries))
		for _, entry := range entries {
			args := make([]*resp.Value, 0, len(entry.Args)+1)
			args = append(args, resp.BulkString(entry.Command))
			for _, a := range entry.Args {
				args = append(args, resp.BulkBytes(a))
			}
			results = append(results, resp.ArrayValue([]*resp.Value{
				resp.IntegerValue(entry.ID),
				// Redis reports field 2 as a Unix timestamp in SECONDS while
				// field 3 is in MICROSECONDS. Emitting UnixMicro() here
				// inflated the timestamp by 1e6, so a client rendering it as
				// a wall-clock date landed ~50,000 years in the future.
				resp.IntegerValue(entry.Timestamp.Unix()),
				resp.IntegerValue(entry.Duration.Microseconds()),
				resp.ArrayValue(args),
				resp.BulkString(entry.ClientIP),
				resp.BulkString(entry.ClientName),
			}))
		}
		return ctx.WriteArray(results)
	case "LEN":
		return ctx.WriteInteger(int64(store.GlobalSlowLog.Len()))
	case "RESET":
		store.GlobalSlowLog.Clear()
		return ctx.WriteOK()
	default:
		return ctx.WriteArray(helpReply(slowlogHelp))
	}
}

func cmdLASTSAVE(ctx *Context) error {
	if r := ctx.dispatcher(); r != nil {
		_, last, _ := r.snapshotHooks()
		if last != nil {
			return ctx.WriteInteger(last())
		}
	}
	if lastSaveTime.Load() == 0 {
		lastSaveTime.Store(time.Now().Unix())
	}
	return ctx.WriteInteger(lastSaveTime.Load())
}

func cmdLOLWUT(ctx *Context) error {
	version := 6
	if ctx.ArgCount() > 0 {
		arg := strings.ToUpper(ctx.ArgString(0))
		if arg == "VERSION" && ctx.ArgCount() > 1 {
			v, err := strconv.Atoi(ctx.ArgString(1))
			if err == nil {
				version = v
			}
		}
	}

	var output strings.Builder
	output.WriteString("\n")

	switch version {
	case 5:
		for i := 0; i < 6; i++ {
			output.WriteString("._ \n")
			for j := 0; j < 15; j++ {
				output.WriteString(" ")
			}
			output.WriteString(".\n")
		}
	default:
		output.WriteString("CacheStorm ver. " + buildversion.Version + "\n")
		output.WriteString("\n")
		output.WriteString("High-performance Redis-compatible cache with tag-based invalidation.\n")
	}

	return ctx.WriteVerbatim(output.String())
}

// ShutdownOptions are the modifiers of SHUTDOWN.
type ShutdownOptions struct {
	Save   bool
	NoSave bool
	Now    bool
	Force  bool
}

// SetShutdownHook installs the function SHUTDOWN stops the server with.
func (r *Router) SetShutdownHook(fn func(ShutdownOptions) error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.shutdown = fn
}

// cmdSHUTDOWN implements SHUTDOWN [NOSAVE|SAVE] [NOW] [FORCE] [ABORT]. On
// success it sends no reply: the connection closes as the server stops.
func cmdSHUTDOWN(ctx *Context) error {
	var o ShutdownOptions
	abort := false
	for i := 0; i < ctx.ArgCount(); i++ {
		switch strings.ToUpper(ctx.ArgString(i)) {
		case "NOSAVE":
			o.NoSave = true
		case "SAVE":
			o.Save = true
		case "NOW":
			o.Now = true
		case "FORCE":
			o.Force = true
		case "ABORT":
			abort = true
		default:
			return ctx.WriteError(ErrSyntaxError)
		}
	}
	if (o.Save && o.NoSave) || (abort && ctx.ArgCount() > 1) {
		return ctx.WriteError(ErrSyntaxError)
	}
	if abort {
		return ctx.Writer.WriteError("ERR No shutdown in progress.")
	}
	var hook func(ShutdownOptions) error
	if r := ctx.dispatcher(); r != nil {
		r.mu.RLock()
		hook = r.shutdown
		r.mu.RUnlock()
	}
	if hook == nil {
		return ctx.WriteError(errors.New("ERR SHUTDOWN is not available in this context"))
	}
	if err := hook(o); err != nil {
		return ctx.WriteError(err)
	}
	ctx.CloseConnection = true
	return nil
}

func cmdSAVE(ctx *Context) error {
	if r := ctx.dispatcher(); r != nil {
		save, _, _ := r.snapshotHooks()
		if save != nil {
			if err := save(false); err != nil {
				return ctx.WriteError(err)
			}
			return ctx.WriteOK()
		}
	}
	// SAVE is the explicit durability barrier, so it must actually persist
	// before reporting success. Stamping the timestamp and replying +OK while
	// doing no I/O gave clients a guarantee the server had not kept, and
	// lastSaveTime then recorded a save that never happened.
	if err := runSaveHook(); err != nil {
		return ctx.WriteError(err)
	}
	lastSaveTime.Store(time.Now().Unix())
	return ctx.WriteOK()
}

func cmdBGSAVE(ctx *Context) error {
	if ctx.ArgCount() > 1 || (ctx.ArgCount() == 1 && !strings.EqualFold(ctx.ArgString(0), "SCHEDULE")) {
		return ctx.WriteError(ErrSyntaxError)
	}
	if r := ctx.dispatcher(); r != nil {
		save, _, _ := r.snapshotHooks()
		if save != nil {
			if err := save(true); err != nil {
				return ctx.WriteError(err)
			}
			return ctx.WriteSimpleString("Background saving started")
		}
	}
	// BGSAVE is the background form of the same barrier. The configured flush
	// is cheap and idempotent, so it runs inline: reporting "started" without a
	// guaranteed write would reintroduce the same false guarantee.
	if err := runSaveHook(); err != nil {
		return ctx.WriteError(err)
	}
	lastSaveTime.Store(time.Now().Unix())
	return ctx.WriteSimpleString("Background saving started")
}

func cmdBGREWRITEAOF(ctx *Context) error {
	if ctx.ArgCount() != 0 {
		return ctx.WriteError(ErrWrongArgCount)
	}
	r := ctx.dispatcher()
	if r == nil {
		return ctx.WriteError(errors.New("ERR append only file is not enabled"))
	}
	rewrite := r.aofRewriteHook()
	if rewrite == nil {
		return ctx.WriteError(errors.New("ERR append only file is not enabled"))
	}
	if err := rewrite(); err != nil {
		return ctx.WriteError(err)
	}
	return ctx.WriteSimpleString("Background append only file rewriting started")
}

func cmdACL(ctx *Context) error {
	if ctx.ArgCount() < 1 {
		return ctx.WriteError(ErrWrongArgCount)
	}
	if !checkRedisSubcommand(ctx, "acl", nil) {
		return nil
	}

	subCmd := strings.ToUpper(ctx.ArgString(0))

	switch subCmd {
	case "LIST":
		users := globalACL.ListUsers()
		results := make([]*resp.Value, 0, len(users))
		for _, name := range users {
			if user, ok := globalACL.GetUser(name); ok {
				results = append(results, resp.BulkString(user.ToACLString()))
			}
		}
		return ctx.WriteArray(results)
	case "USERS":
		users := globalACL.ListUsers()
		results := make([]*resp.Value, 0, len(users))
		for _, name := range users {
			results = append(results, resp.BulkString(name))
		}
		return ctx.WriteArray(results)
	case "WHOAMI":
		return ctx.WriteBulkString(ctx.userName())
	case "CAT":
		return cmdACLCat(ctx)
	case "SETUSER":
		if ctx.ArgCount() < 2 {
			return ctx.WriteError(ErrWrongArgCount)
		}
		username := ctx.ArgString(1)
		ruleParts := make([]string, 0, len(ctx.Args)-2)
		for i := 2; i < len(ctx.Args); i++ {
			ruleParts = append(ruleParts, string(ctx.Args[i]))
		}
		if err := acl.ValidateACLRules(ruleParts); err != nil {
			return ctx.WriteError(err)
		}
		user, exists := globalACL.GetUser(username)
		if !exists {
			var err error
			user, err = globalACL.CreateUser(username)
			if err != nil {
				return ctx.WriteError(err)
			}
		}
		if err := acl.ParseACLRules(ruleParts, user); err != nil {
			return ctx.WriteError(err)
		}
		return ctx.WriteOK()
	case "DELUSER":
		if ctx.ArgCount() < 2 {
			return ctx.WriteError(ErrWrongArgCount)
		}
		deleted := int64(0)
		for i := 1; i < ctx.ArgCount(); i++ {
			username := ctx.ArgString(i)
			if err := globalACL.DeleteUser(username); err == nil {
				deleted++
			}
		}
		return ctx.WriteInteger(deleted)
	case "GETUSER":
		if ctx.ArgCount() < 2 {
			return ctx.WriteError(ErrWrongArgCount)
		}
		user, exists := globalACL.GetUser(ctx.ArgString(1))
		if !exists {
			return ctx.WriteNull()
		}
		return ctx.WriteValue(aclGetUserValue(user))
	case "LOAD", "SAVE":
		return ctx.Writer.WriteError("ERR This Redis instance is not configured to use an ACL file. You may want to specify users via the ACL SETUSER command and then issue a CONFIG REWRITE (assuming you have a Redis configuration file set) in order to store users in the Redis configuration.")
	case "LOG":
		return cmdACLLog(ctx)
	case "DRYRUN":
		if ctx.ArgCount() < 3 {
			return ctx.WriteError(arityError("acl|dryrun"))
		}
		return aclDryRun(ctx)
	case "GENPASS":
		if ctx.ArgCount() > 2 {
			return ctx.WriteError(errSubcommandSyntax(ctx.ArgString(0), "ACL"))
		}
		bits := 256
		if ctx.ArgCount() > 1 {
			var err error
			bits, err = strconv.Atoi(ctx.ArgString(1))
			if err != nil {
				return ctx.WriteError(ErrNotInteger)
			}
			if bits <= 0 || bits > 4096 {
				return ctx.WriteError(errors.New("ERR ACL GENPASS argument must be the number of bits for the output password, a positive number up to 4096"))
			}
		}
		return ctx.WriteBulkString(generatePassword(bits))
	case "HELP":
		return ctx.WriteArray(helpReply(aclHelp))
	default:
		return ctx.WriteError(errUnknownSubcommand(ctx.ArgString(0), "ACL"))
	}
}

// aclGetUserValue is the ACL GETUSER reply for user, in Redis 7.4's shape.
func aclGetUserValue(user *acl.User) *resp.Value {
	d := user.Describe()
	flags := make([]*resp.Value, len(d.Flags))
	for i, f := range d.Flags {
		flags[i] = resp.BulkString(f)
	}
	passwords := make([]*resp.Value, len(d.Passwords))
	for i, p := range d.Passwords {
		passwords[i] = resp.BulkString(p)
	}
	selectors := make([]*resp.Value, len(d.Selectors))
	for i, sel := range d.Selectors {
		selectors[i] = resp.PairsValue([]*resp.Value{
			resp.BulkString("commands"), resp.BulkString(sel.Commands),
			resp.BulkString("keys"), resp.BulkString(sel.Keys),
			resp.BulkString("channels"), resp.BulkString(sel.Channels),
		})
	}
	return resp.PairsValue([]*resp.Value{
		resp.BulkString("flags"), resp.SetValue(flags),
		resp.BulkString("passwords"), resp.ArrayValue(passwords),
		resp.BulkString("commands"), resp.BulkString(d.Commands),
		resp.BulkString("keys"), resp.BulkString(d.Keys),
		resp.BulkString("channels"), resp.BulkString(d.Channels),
		resp.BulkString("selectors"), resp.ArrayValue(selectors),
	})
}

// aclDryRun implements ACL DRYRUN username command [arg ...]: whether the
// user could run the command, checking its command, key and channel
// permissions. A denial is a bulk string reply, as in Redis.
func aclDryRun(ctx *Context) error {
	username := ctx.ArgString(1)
	user, exists := globalACL.GetUser(username)
	if !exists {
		return ctx.WriteError(errors.New("ERR User '" + username + "' not found"))
	}
	argv := make([]string, 0, ctx.ArgCount()-2)
	for i := 2; i < ctx.ArgCount(); i++ {
		argv = append(argv, ctx.ArgString(i))
	}
	name := CanonicalName(argv[0])
	r := ctx.dispatcher()
	var def *CommandDef
	found := false
	if r != nil {
		def, found = r.Get(name)
	}
	meta := lookupRedisCommandMeta(argv)
	if !found || (meta == nil && redisCommandMeta(argv[0]) != nil) {
		return ctx.WriteError(errors.New("ERR Command '" + argv[0] + "' not found"))
	}
	shown := strings.ToLower(argv[0])
	arity := def.Arity
	if meta != nil {
		shown, arity = meta.name, meta.arity
	}
	if arity != 0 && !arityFits(arity, len(argv)) {
		return ctx.WriteError(arityError(shown))
	}
	args := ctx.Args[3:]
	denial, item := aclCheck(user, name, args)
	switch denial {
	case acl.DeniedCommand:
		return ctx.WriteBulkString("User " + user.Name + " has no permissions to run the '" + shown + "' command")
	case acl.DeniedKey:
		return ctx.WriteBulkString("User " + user.Name + " has no permissions to access the '" + item + "' key")
	case acl.DeniedChannel:
		return ctx.WriteBulkString("User " + user.Name + " has no permissions to access the '" + item + "' channel")
	}
	return ctx.WriteOK()
}

// generatePassword returns a random password of bits bits as Redis' ACL
// GENPASS does: (bits+3)/4 lowercase hex digits.
func generatePassword(bits int) string {
	buf := make([]byte, (bits+7)/8)
	if _, err := rand.Read(buf); err != nil {
		panic(err)
	}
	return hex.EncodeToString(buf)[:(bits+3)/4]
}

// cmdMONITOR puts the connection in monitor mode: the server streams every
// command other clients run to it (see MonitorVisible).
func cmdMONITOR(ctx *Context) error {
	ctx.GetSubscriber()
	ctx.Monitor = true
	return ctx.WriteOK()
}

// MonitorVisible reports whether MONITOR shows cmd. As in Redis, admin
// commands and those that carry credentials are left out.
func MonitorVisible(cmd string) bool {
	switch cmd {
	case "AUTH", "HELLO", "MIGRATE":
		return false
	}
	return !commandHasFlag(cmd, "admin")
}

// SetWaitAOFHook installs the function WAITAOF waits with. It returns how
// many local AOFs (0 or 1) and replicas acknowledged the writes so far.
func (r *Router) SetWaitAOFHook(fn func(numLocal, numReplicas int64, timeout time.Duration, done <-chan struct{}) (int64, int64, error)) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.waitAOF = fn
}

// cmdWAITAOF implements WAITAOF numlocal numreplicas timeout.
func cmdWAITAOF(ctx *Context) error {
	if ctx.ArgCount() != 3 {
		return ctx.WriteError(arityError("waitaof"))
	}
	numLocal, err1 := strconv.ParseInt(ctx.ArgString(0), 10, 64)
	numReplicas, err2 := strconv.ParseInt(ctx.ArgString(1), 10, 64)
	if err1 != nil || err2 != nil || numLocal < 0 || numReplicas < 0 {
		return ctx.WriteError(ErrNotInteger)
	}
	ms, err := strconv.ParseInt(ctx.ArgString(2), 10, 64)
	if err != nil {
		return ctx.WriteError(errors.New("ERR timeout is not an integer or out of range"))
	}
	if ms < 0 {
		return ctx.WriteError(errors.New("ERR timeout is negative"))
	}
	var hook func(int64, int64, time.Duration, <-chan struct{}) (int64, int64, error)
	if r := ctx.dispatcher(); r != nil {
		r.mu.RLock()
		hook = r.waitAOF
		r.mu.RUnlock()
	}
	if hook == nil {
		if numLocal > 0 {
			return ctx.Writer.WriteError("ERR WAITAOF cannot be used when numlocal is set but appendonly is disabled.")
		}
		return ctx.WriteArray([]*resp.Value{resp.IntegerValue(0), resp.IntegerValue(0)})
	}
	timeout := time.Duration(ms) * time.Millisecond
	if ctx.InExec {
		timeout = -1
	}
	local, replicas, err := hook(numLocal, numReplicas, timeout, ctx.Done)
	if err != nil {
		return ctx.Writer.WriteError(err.Error())
	}
	return ctx.WriteArray([]*resp.Value{resp.IntegerValue(local), resp.IntegerValue(replicas)})
}
