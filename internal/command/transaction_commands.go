package command

import (
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/cachestorm/cachestorm/internal/resp"
	"github.com/cachestorm/cachestorm/internal/store"
)

var (
	ErrExecWithoutMulti    = errors.New("ERR EXEC without MULTI")
	ErrDiscardWithoutMulti = errors.New("ERR DISCARD without MULTI")
	ErrWatchInMulti        = errors.New("ERR WATCH inside MULTI is not allowed")
)

type Transaction struct {
	mu          sync.Mutex
	queued      []queuedCommand
	active      bool
	watchedKeys map[string]int64
}

type queuedCommand struct {
	cmd  string
	args [][]byte
}

func NewTransaction() *Transaction {
	return &Transaction{
		queued:      make([]queuedCommand, 0),
		active:      false,
		watchedKeys: make(map[string]int64),
	}
}

func (t *Transaction) Queue(cmd string, args [][]byte) {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.queued = append(t.queued, queuedCommand{cmd: cmd, args: args})
}

func (t *Transaction) GetQueued() []queuedCommand {
	t.mu.Lock()
	defer t.mu.Unlock()
	result := make([]queuedCommand, len(t.queued))
	copy(result, t.queued)
	return result
}

func (t *Transaction) Clear() {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.queued = t.queued[:0]
	t.active = false
}

func (t *Transaction) ClearWatch() {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.watchedKeys = make(map[string]int64)
}

func (t *Transaction) IsActive() bool {
	t.mu.Lock()
	defer t.mu.Unlock()
	return t.active
}

func (t *Transaction) Start() {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.active = true
	t.queued = t.queued[:0]
}

func (t *Transaction) Watch(key string, version int64) {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.watchedKeys[key] = version
}

func (t *Transaction) CheckWatchedVersions(getVersion func(key string) int64) bool {
	t.mu.Lock()
	defer t.mu.Unlock()
	for key, version := range t.watchedKeys {
		currentVersion := getVersion(key)
		if currentVersion != version {
			return false
		}
	}
	return true
}

func (t *Transaction) HasWatchedKeys() bool {
	t.mu.Lock()
	defer t.mu.Unlock()
	return len(t.watchedKeys) > 0
}

func RegisterTransactionCommands(router *Router) {
	router.Register(&CommandDef{Name: "MULTI", Handler: cmdMULTI})
	router.Register(&CommandDef{Name: "EXEC", Handler: cmdEXEC})
	router.Register(&CommandDef{Name: "DISCARD", Handler: cmdDISCARD})
	router.Register(&CommandDef{Name: "WATCH", Handler: cmdWATCH})
	router.Register(&CommandDef{Name: "UNWATCH", Handler: cmdUNWATCH})
}

func cmdMULTI(ctx *Context) error {
	if ctx.Transaction.HasWatchedKeys() {
		ctx.Transaction.ClearWatch()
	}
	ctx.Transaction.Start()
	return ctx.WriteOK()
}

func cmdEXEC(ctx *Context) error {
	if !ctx.Transaction.IsActive() {
		return ctx.WriteError(ErrExecWithoutMulti)
	}

	if ctx.Transaction.HasWatchedKeys() {
		if !ctx.Transaction.CheckWatchedVersions(ctx.Store.GetVersion) {
			ctx.Transaction.Clear()
			ctx.Transaction.ClearWatch()
			return ctx.WriteNull()
		}
	}

	queued := ctx.Transaction.GetQueued()
	ctx.Transaction.Clear()

	if len(queued) == 0 {
		return ctx.WriteArray([]*resp.Value{})
	}

	results := make([]*resp.Value, 0, len(queued))

	for _, qc := range queued {
		result := executeQueuedCommand(ctx, qc)
		results = append(results, result)
	}

	return ctx.WriteArray(results)
}

func cmdDISCARD(ctx *Context) error {
	if !ctx.Transaction.IsActive() {
		return ctx.WriteError(ErrDiscardWithoutMulti)
	}

	ctx.Transaction.Clear()
	ctx.Transaction.ClearWatch()
	return ctx.WriteOK()
}

func cmdWATCH(ctx *Context) error {
	if ctx.Transaction.IsActive() {
		return ctx.WriteError(ErrWatchInMulti)
	}

	for i := 0; i < ctx.ArgCount(); i++ {
		key := ctx.ArgString(i)
		version := ctx.Store.GetVersion(key)
		ctx.Transaction.Watch(key, version)
	}

	return ctx.WriteOK()
}

func cmdUNWATCH(ctx *Context) error {
	ctx.Transaction.ClearWatch()
	return ctx.WriteOK()
}

func executeQueuedCommand(ctx *Context, qc queuedCommand) *resp.Value {
	switch qc.cmd {
	case "SET":
		if len(qc.args) >= 2 {
			key := string(qc.args[0])
			value := qc.args[1]
			ctx.Store.Set(key, &store.StringValue{Data: value}, store.SetOptions{})
			return resp.SimpleString("OK")
		}
		return resp.ErrorValue("ERR wrong number of arguments")
	case "GET":
		if len(qc.args) >= 1 {
			key := string(qc.args[0])
			if entry, exists := ctx.Store.Get(key); exists {
				if sv, ok := entry.Value.(*store.StringValue); ok {
					return resp.BulkBytes(sv.Data)
				}
			}
			return resp.NullBulkString()
		}
		return resp.ErrorValue("ERR wrong number of arguments")
	case "DEL":
		if len(qc.args) >= 1 {
			deleted := int64(0)
			for _, arg := range qc.args {
				if ctx.Store.Delete(string(arg)) {
					deleted++
				}
			}
			return resp.IntegerValue(deleted)
		}
		return resp.ErrorValue("ERR wrong number of arguments")
	case "INCR":
		if len(qc.args) >= 1 {
			key := string(qc.args[0])
			if entry, exists := ctx.Store.Get(key); exists {
				if sv, ok := entry.Value.(*store.StringValue); ok {
					var newVal int64
					for _, b := range sv.Data {
						newVal = newVal*10 + int64(b-'0')
					}
					newVal++
					ctx.Store.Set(key, &store.StringValue{Data: []byte(int64ToBytes(newVal))}, store.SetOptions{})
					return resp.IntegerValue(newVal)
				}
			}
			ctx.Store.Set(key, &store.StringValue{Data: []byte("1")}, store.SetOptions{})
			return resp.IntegerValue(1)
		}
		return resp.ErrorValue("ERR wrong number of arguments")
	case "DECR":
		if len(qc.args) >= 1 {
			key := string(qc.args[0])
			if entry, exists := ctx.Store.Get(key); exists {
				if sv, ok := entry.Value.(*store.StringValue); ok {
					var newVal int64
					for _, b := range sv.Data {
						newVal = newVal*10 + int64(b-'0')
					}
					newVal--
					ctx.Store.Set(key, &store.StringValue{Data: []byte(int64ToBytes(newVal))}, store.SetOptions{})
					return resp.IntegerValue(newVal)
				}
			}
			ctx.Store.Set(key, &store.StringValue{Data: []byte("-1")}, store.SetOptions{})
			return resp.IntegerValue(-1)
		}
		return resp.ErrorValue("ERR wrong number of arguments")
	case "EXPIRE":
		if len(qc.args) >= 2 {
			key := string(qc.args[0])
			seconds, err := parseInt(qc.args[1])
			if err != nil {
				return resp.ErrorValue("ERR value is not an integer")
			}
			if _, exists := ctx.Store.Get(key); exists {
				ctx.Store.SetTTL(key, time.Duration(seconds)*time.Second)
				return resp.IntegerValue(1)
			}
			return resp.IntegerValue(0)
		}
		return resp.ErrorValue("ERR wrong number of arguments")
	case "TTL":
		if len(qc.args) >= 1 {
			key := string(qc.args[0])
			ttl := ctx.Store.GetTTL(key)
			if ttl == -2*time.Second {
				return resp.IntegerValue(-2)
			}
			if ttl == -1*time.Second {
				return resp.IntegerValue(-1)
			}
			return resp.IntegerValue(int64(ttl / time.Second))
		}
		return resp.ErrorValue("ERR wrong number of arguments")
	case "PERSIST":
		if len(qc.args) >= 1 {
			key := string(qc.args[0])
			if _, exists := ctx.Store.Get(key); exists {
				if ctx.Store.GetTTL(key) > 0 {
					ctx.Store.Persist(key)
					return resp.IntegerValue(1)
				}
				return resp.IntegerValue(0)
			}
			return resp.IntegerValue(0)
		}
		return resp.ErrorValue("ERR wrong number of arguments")
	case "TYPE":
		if len(qc.args) >= 1 {
			key := string(qc.args[0])
			if entry, exists := ctx.Store.Get(key); exists {
				return resp.SimpleString(entry.Value.Type().String())
			}
			return resp.SimpleString("none")
		}
		return resp.ErrorValue("ERR wrong number of arguments")
	case "EXISTS":
		count := int64(0)
		for _, arg := range qc.args {
			if _, exists := ctx.Store.Get(string(arg)); exists {
				count++
			}
		}
		return resp.IntegerValue(count)
	case "APPEND":
		if len(qc.args) >= 2 {
			key := string(qc.args[0])
			value := qc.args[1]
			if entry, exists := ctx.Store.Get(key); exists {
				if sv, ok := entry.Value.(*store.StringValue); ok {
					newData := append(sv.Data, value...)
					ctx.Store.Set(key, &store.StringValue{Data: newData}, store.SetOptions{})
					return resp.IntegerValue(int64(len(newData)))
				}
				return resp.ErrorValue("WRONGTYPE Operation against a key holding the wrong kind of value")
			}
			ctx.Store.Set(key, &store.StringValue{Data: value}, store.SetOptions{})
			return resp.IntegerValue(int64(len(value)))
		}
		return resp.ErrorValue("ERR wrong number of arguments")
	case "MSET":
		if len(qc.args) >= 2 && len(qc.args)%2 == 0 {
			for i := 0; i < len(qc.args); i += 2 {
				key := string(qc.args[i])
				value := qc.args[i+1]
				ctx.Store.Set(key, &store.StringValue{Data: value}, store.SetOptions{})
			}
			return resp.SimpleString("OK")
		}
		return resp.ErrorValue("ERR wrong number of arguments")
	case "MGET":
		if len(qc.args) >= 1 {
			results := make([]*resp.Value, len(qc.args))
			for i, arg := range qc.args {
				key := string(arg)
				if entry, exists := ctx.Store.Get(key); exists {
					if sv, ok := entry.Value.(*store.StringValue); ok {
						results[i] = resp.BulkBytes(sv.Data)
					} else {
						results[i] = resp.NullValue()
					}
				} else {
					results[i] = resp.NullValue()
				}
			}
			return resp.ArrayValue(results)
		}
		return resp.ErrorValue("ERR wrong number of arguments")
	default:
		return resp.ErrorValue("ERR command not supported in transaction")
	}
}

func parseInt(data []byte) (int64, error) {
	var result int64
	var negative bool
	i := 0
	if len(data) > 0 && data[0] == '-' {
		negative = true
		i = 1
	}
	for ; i < len(data); i++ {
		if data[i] < '0' || data[i] > '9' {
			return 0, fmt.Errorf("not an integer")
		}
		result = result*10 + int64(data[i]-'0')
	}
	if negative {
		result = -result
	}
	return result, nil
}

func int64ToBytes(n int64) []byte {
	if n == 0 {
		return []byte("0")
	}

	var negative bool
	if n < 0 {
		negative = true
		n = -n
	}

	var buf [20]byte
	i := len(buf)

	for n > 0 {
		i--
		buf[i] = byte('0' + n%10)
		n /= 10
	}

	if negative {
		i--
		buf[i] = '-'
	}

	return buf[i:]
}
