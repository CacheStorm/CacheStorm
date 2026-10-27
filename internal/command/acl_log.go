package command

import (
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/cachestorm/cachestorm/internal/resp"
)

// aclLogMaxLen is acllog-max-len: how many ACL LOG entries are kept.
const aclLogMaxLen = 128

// aclLogGroupWindow is how long a repeated denial adds to the existing entry
// instead of starting a new one, as in Redis.
const aclLogGroupWindow = 60 * time.Second

type aclLogEntry struct {
	id         int64
	count      int64
	reason     string
	context    string
	object     string
	username   string
	clientInfo string
	created    time.Time
	updated    time.Time
}

var aclLog = struct {
	mu      sync.Mutex
	entries []*aclLogEntry // newest first
	nextID  int64
}{}

func aclLogContext(ctx *Context) string {
	switch {
	case ctx.InScript:
		return "lua"
	case ctx.InExec:
		return "multi"
	default:
		return "toplevel"
	}
}

// recordACLDenial adds a denied command, key, channel or authentication to
// ACL LOG.
func recordACLDenial(ctx *Context, reason, object, username string) {
	if username == "" {
		username = "default"
	}
	context := aclLogContext(ctx)
	clientInfo := "id=" + strconv.FormatInt(ctx.ClientID, 10) + " addr=" + ctx.RemoteAddr + " name=" + ctx.ClientName +
		" db=" + strconv.Itoa(ctx.DB) + " user=" + username + " cmd=" + strings.ToLower(ctx.Command)
	now := time.Now()
	aclLog.mu.Lock()
	defer aclLog.mu.Unlock()
	for _, e := range aclLog.entries {
		if e.reason == reason && e.context == context && e.object == object && e.username == username &&
			now.Sub(e.updated) < aclLogGroupWindow {
			e.count++
			e.updated = now
			e.clientInfo = clientInfo
			return
		}
	}
	e := &aclLogEntry{id: aclLog.nextID, count: 1, reason: reason, context: context, object: object,
		username: username, clientInfo: clientInfo, created: now, updated: now}
	aclLog.nextID++
	aclLog.entries = append([]*aclLogEntry{e}, aclLog.entries...)
	if len(aclLog.entries) > aclLogMaxLen {
		aclLog.entries = aclLog.entries[:aclLogMaxLen]
	}
}

// cmdACLLog implements ACL LOG [count | RESET].
func cmdACLLog(ctx *Context) error {
	count := 10
	if ctx.ArgCount() > 2 {
		return ctx.WriteError(arityError("acl|log"))
	}
	if ctx.ArgCount() == 2 {
		if strings.EqualFold(ctx.ArgString(1), "RESET") {
			aclLog.mu.Lock()
			aclLog.entries = nil
			aclLog.mu.Unlock()
			return ctx.WriteOK()
		}
		n, err := strconv.Atoi(ctx.ArgString(1))
		if err != nil || n < 0 {
			return ctx.WriteError(ErrNotInteger)
		}
		count = n
	}
	now := time.Now()
	aclLog.mu.Lock()
	defer aclLog.mu.Unlock()
	if count > len(aclLog.entries) {
		count = len(aclLog.entries)
	}
	out := make([]*resp.Value, 0, count)
	for _, e := range aclLog.entries[:count] {
		age := now.Sub(e.created).Seconds()
		out = append(out, resp.PairsValue([]*resp.Value{
			resp.BulkString("count"), resp.IntegerValue(e.count),
			resp.BulkString("reason"), resp.BulkString(e.reason),
			resp.BulkString("context"), resp.BulkString(e.context),
			resp.BulkString("object"), resp.BulkString(e.object),
			resp.BulkString("username"), resp.BulkString(e.username),
			resp.BulkString("age-seconds"), resp.DoubleValue(strconv.FormatFloat(age, 'f', 3, 64)),
			resp.BulkString("client-info"), resp.BulkString(e.clientInfo),
			resp.BulkString("entry-id"), resp.IntegerValue(e.id),
			resp.BulkString("timestamp-created"), resp.IntegerValue(e.created.UnixMilli()),
			resp.BulkString("timestamp-last-updated"), resp.IntegerValue(e.updated.UnixMilli()),
		}))
	}
	return ctx.WriteArray(out)
}
