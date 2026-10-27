package command

import (
	"errors"
	"strconv"
	"strings"
	"time"

	"github.com/cachestorm/cachestorm/internal/resp"
	"github.com/cachestorm/cachestorm/internal/store"
)

// RegisterHashTTLCommands registers hash field expiration: the Redis 7.4
// HEXPIRE family, HTTL family and HPERSIST, and Redis 8's HGETEX and HSETEX.
func RegisterHashTTLCommands(router *Router) {
	router.Register(&CommandDef{Name: "HEXPIRE", Handler: cmdHEXPIRE})
	router.Register(&CommandDef{Name: "HPEXPIRE", Handler: cmdHPEXPIRE})
	router.Register(&CommandDef{Name: "HEXPIREAT", Handler: cmdHEXPIREAT})
	router.Register(&CommandDef{Name: "HPEXPIREAT", Handler: cmdHPEXPIREAT})
	router.Register(&CommandDef{Name: "HTTL", Handler: cmdHTTL})
	router.Register(&CommandDef{Name: "HPTTL", Handler: cmdHPTTL})
	router.Register(&CommandDef{Name: "HEXPIRETIME", Handler: cmdHEXPIRETIME})
	router.Register(&CommandDef{Name: "HPEXPIRETIME", Handler: cmdHPEXPIRETIME})
	router.Register(&CommandDef{Name: "HPERSIST", Handler: cmdHPERSIST})
	router.Register(&CommandDef{Name: "HGETEX", Handler: cmdHGETEX})
	router.Register(&CommandDef{Name: "HSETEX", Handler: cmdHSETEX})
}

var (
	errHFENumFieldsZero     = errors.New("ERR Parameter `numFields` should be greater than 0")
	errHFENumFieldsPositive = errors.New("ERR Number of fields must be a positive integer")
	errHFENumFieldsMismatch = errors.New("ERR The `numfields` parameter must match the number of arguments")
	errHFEFieldsMissing     = errors.New("ERR Mandatory argument FIELDS is missing or not at the right position")
	errHFEExpireNegative    = errors.New("ERR invalid expire time, must be >= 0")
	errHSETEXNumFields      = errors.New("ERR invalid number of fields")
	errHSETEXCondition      = errors.New("ERR Only one of FXX or FNX arguments can be specified")
	errHSETEXExpiry         = errors.New("ERR Only one of EX, PX, EXAT, PXAT or KEEPTTL arguments can be specified")
)

func errHFEExpireTime(cmd string) error {
	return errors.New("ERR invalid expire time in '" + strings.ToLower(cmd) + "' command")
}

// hfeExpireAt converts a field expiry argument to unix milliseconds the way
// Redis 7.4 does: unitMS is 1000 for seconds, absolute is false for a time
// relative to now.
func hfeExpireAt(ctx *Context, arg string, unitMS int64, absolute bool) (int64, error) {
	t, err := strconv.ParseInt(arg, 10, 64)
	if err != nil {
		return 0, ErrNotInteger
	}
	if t < 0 {
		return 0, errHFEExpireNegative
	}
	if unitMS == 1000 {
		if t > store.MaxHashFieldExpireMS/1000 {
			return 0, errHFEExpireTime(ctx.Command)
		}
		t *= 1000
	}
	var base int64
	if !absolute {
		base = time.Now().UnixMilli()
	}
	if t > store.MaxHashFieldExpireMS-base {
		return 0, errHFEExpireTime(ctx.Command)
	}
	return t + base, nil
}

// hfeFields parses "FIELDS numfields field ..." starting at args[at] for the
// HTTL family, HPERSIST and HGETEX.
func hfeFields(ctx *Context, at int) ([]string, error) {
	if at >= ctx.ArgCount() || !strings.EqualFold(ctx.ArgString(at), "FIELDS") {
		return nil, errHFEFieldsMissing
	}
	if at+1 >= ctx.ArgCount() {
		return nil, arityError(ctx.Command)
	}
	n, err := strconv.ParseInt(ctx.ArgString(at+1), 10, 64)
	if err != nil || n < 1 {
		return nil, errHFENumFieldsPositive
	}
	if n != int64(ctx.ArgCount()-at-2) {
		return nil, errHFENumFieldsMismatch
	}
	fields := make([]string, n)
	for i := range fields {
		fields[i] = ctx.ArgString(at + 2 + i)
	}
	return fields, nil
}

func hfeIntegers(n int, v int64) []*resp.Value {
	out := make([]*resp.Value, n)
	for i := range out {
		out[i] = resp.IntegerValue(v)
	}
	return out
}

func hfeArgs(key string, rest ...string) [][]byte {
	out := make([][]byte, 0, len(rest)+1)
	out = append(out, []byte(key))
	for _, s := range rest {
		out = append(out, []byte(s))
	}
	return out
}

// hfePropagation is what a field expiry command records: an HDEL for every
// field it deleted, then HPEXPIREAT for the fields it gave an expiry and
// HPERSIST for those it persisted.
func hfePropagation(ctx *Context, key string, deleted []string, at int64, updated, persisted []string) {
	cmds := make([]ExecutedCommand, 0, len(deleted)+2)
	for _, f := range deleted {
		cmds = append(cmds, ExecutedCommand{Name: "HDEL", Args: hfeArgs(key, f), DB: ctx.DB})
	}
	if len(updated) > 0 {
		args := append([]string{strconv.FormatInt(at, 10), "FIELDS", strconv.Itoa(len(updated))}, updated...)
		cmds = append(cmds, ExecutedCommand{Name: "HPEXPIREAT", Args: hfeArgs(key, args...), DB: ctx.DB})
	}
	if len(persisted) > 0 {
		args := append([]string{"FIELDS", strconv.Itoa(len(persisted))}, persisted...)
		cmds = append(cmds, ExecutedCommand{Name: "HPERSIST", Args: hfeArgs(key, args...), DB: ctx.DB})
	}
	ctx.PropagateAs(cmds...)
	if len(cmds) > 0 {
		ctx.Store.TouchKey(key)
	}
}

func cmdHEXPIRE(ctx *Context) error    { return hexpireGeneric(ctx, 1000, false) }
func cmdHPEXPIRE(ctx *Context) error   { return hexpireGeneric(ctx, 1, false) }
func cmdHEXPIREAT(ctx *Context) error  { return hexpireGeneric(ctx, 1000, true) }
func cmdHPEXPIREAT(ctx *Context) error { return hexpireGeneric(ctx, 1, true) }

const (
	hfeCondNone = iota
	hfeCondNX
	hfeCondXX
	hfeCondGT
	hfeCondLT
)

func hexpireGeneric(ctx *Context, unitMS int64, absolute bool) error {
	ctx.writeEffectsHandled = true
	if ctx.ArgCount() < 5 {
		return ctx.WriteError(arityError(ctx.Command))
	}
	key := ctx.ArgString(0)
	hash, err := getHash(ctx, key)
	if err != nil {
		return ctx.WriteError(err)
	}
	at, err := hfeExpireAt(ctx, ctx.ArgString(1), unitMS, absolute)
	if err != nil {
		return ctx.WriteError(err)
	}
	cond, fieldsAt := hfeCondNone, 2
	switch strings.ToUpper(ctx.ArgString(2)) {
	case "NX":
		cond = hfeCondNX
	case "XX":
		cond = hfeCondXX
	case "GT":
		cond = hfeCondGT
	case "LT":
		cond = hfeCondLT
	}
	if cond != hfeCondNone {
		fieldsAt = 3
	}
	if !strings.EqualFold(ctx.ArgString(fieldsAt), "FIELDS") {
		return ctx.WriteError(errHFEFieldsMissing)
	}
	n, err := strconv.ParseInt(ctx.ArgString(fieldsAt+1), 10, 64)
	if err != nil || n < 1 {
		return ctx.WriteError(errHFENumFieldsZero)
	}
	if n != int64(ctx.ArgCount()-fieldsAt-2) {
		return ctx.WriteError(errHFENumFieldsMismatch)
	}
	fields := ctx.Args[fieldsAt+2:]
	if hash == nil {
		ctx.PropagateAs()
		return ctx.WriteArray(hfeIntegers(len(fields), -2))
	}
	hash, err = lockHash(ctx, key)
	if err != nil {
		return ctx.WriteError(err)
	}
	if hash == nil {
		ctx.PropagateAs()
		return ctx.WriteArray(hfeIntegers(len(fields), -2))
	}
	deletes := ctx.Store.HashFieldExpiryDeletes()
	now := time.Now().UnixMilli()
	results := make([]*resp.Value, len(fields))
	var deleted, updated []string
	for i, fb := range fields {
		f := string(fb)
		if _, ok := hash.Fields[f]; !ok {
			results[i] = resp.IntegerValue(-2)
			continue
		}
		cur := hash.FieldExpireAt(f)
		skip := false
		switch cond {
		case hfeCondNX:
			skip = cur != 0
		case hfeCondXX:
			skip = cur == 0
		case hfeCondGT:
			skip = cur == 0 || at <= cur
		case hfeCondLT:
			skip = cur != 0 && at >= cur
		}
		if skip {
			results[i] = resp.IntegerValue(0)
			continue
		}
		if deletes && at <= now {
			hash.DeleteField(f)
			deleted = append(deleted, f)
			results[i] = resp.IntegerValue(2)
			continue
		}
		hash.SetFieldExpireAt(f, at)
		updated = append(updated, f)
		results[i] = resp.IntegerValue(1)
	}
	empty := len(hash.Fields) == 0
	hash.Unlock()
	if len(deleted) > 0 {
		ctx.notify(store.NotifyHash, "hdel", key)
	} else if len(updated) > 0 {
		ctx.notify(store.NotifyHash, "hexpire", key)
	}
	if empty {
		deleteIfEmpty(ctx, key, hash)
	} else if len(updated) > 0 {
		ctx.Store.ScheduleHashFieldExpiry(key, hash)
	}
	hfePropagation(ctx, key, deleted, at, updated, nil)
	return ctx.WriteArray(results)
}

func cmdHTTL(ctx *Context) error         { return httlGeneric(ctx, 1000, false) }
func cmdHPTTL(ctx *Context) error        { return httlGeneric(ctx, 1, false) }
func cmdHEXPIRETIME(ctx *Context) error  { return httlGeneric(ctx, 1000, true) }
func cmdHPEXPIRETIME(ctx *Context) error { return httlGeneric(ctx, 1, true) }

func httlGeneric(ctx *Context, unitMS int64, absolute bool) error {
	if ctx.ArgCount() < 4 {
		return ctx.WriteError(arityError(ctx.Command))
	}
	hash, err := getHash(ctx, ctx.ArgString(0))
	if err != nil {
		return ctx.WriteError(err)
	}
	fields, err := hfeFields(ctx, 1)
	if err != nil {
		return ctx.WriteError(err)
	}
	if hash == nil {
		return ctx.WriteArray(hfeIntegers(len(fields), -2))
	}
	now := time.Now().UnixMilli()
	var base int64
	if !absolute {
		base = now
	}
	results := make([]*resp.Value, len(fields))
	hash.RLock()
	for i, f := range fields {
		if _, ok := hash.Fields[f]; !ok {
			results[i] = resp.IntegerValue(-2)
			continue
		}
		at := hash.FieldExpireAt(f)
		switch {
		case at == 0:
			results[i] = resp.IntegerValue(-1)
		case at < now:
			results[i] = resp.IntegerValue(-2)
		case unitMS == 1000:
			results[i] = resp.IntegerValue((at + 999 - base) / 1000)
		default:
			results[i] = resp.IntegerValue(at - base)
		}
	}
	hash.RUnlock()
	return ctx.WriteArray(results)
}

func cmdHPERSIST(ctx *Context) error {
	ctx.writeEffectsHandled = true
	if ctx.ArgCount() < 4 {
		return ctx.WriteError(arityError(ctx.Command))
	}
	key := ctx.ArgString(0)
	hash, err := getHash(ctx, key)
	if err != nil {
		return ctx.WriteError(err)
	}
	fields, err := hfeFields(ctx, 1)
	if err != nil {
		return ctx.WriteError(err)
	}
	if hash != nil {
		hash, err = lockHash(ctx, key)
		if err != nil {
			return ctx.WriteError(err)
		}
	}
	if hash == nil {
		ctx.PropagateAs()
		return ctx.WriteArray(hfeIntegers(len(fields), -2))
	}
	results := make([]*resp.Value, len(fields))
	var persisted []string
	for i, f := range fields {
		switch {
		case !hashHasField(hash, f):
			results[i] = resp.IntegerValue(-2)
		case hash.PersistField(f):
			persisted = append(persisted, f)
			results[i] = resp.IntegerValue(1)
		default:
			results[i] = resp.IntegerValue(-1)
		}
	}
	hash.Unlock()
	if len(persisted) > 0 {
		ctx.notify(store.NotifyHash, "hpersist", key)
	}
	hfePropagation(ctx, key, nil, 0, nil, persisted)
	return ctx.WriteArray(results)
}

func hashHasField(hash *store.HashValue, f string) bool {
	_, ok := hash.Fields[f]
	return ok
}

const (
	hgetexNone = iota
	hgetexExpire
	hgetexPersist
)

// cmdHGETEX is Redis 8's HGETEX key [EX s | PX ms | EXAT t | PXAT t |
// PERSIST] FIELDS numfields field ...: it returns the values of the fields
// and sets or removes their expiry.
func cmdHGETEX(ctx *Context) error {
	ctx.writeEffectsHandled = true
	if ctx.ArgCount() < 4 {
		return ctx.WriteError(arityError(ctx.Command))
	}
	key := ctx.ArgString(0)
	hash, err := getHash(ctx, key)
	if err != nil {
		return ctx.WriteError(err)
	}
	mode, at, fieldsAt := hgetexNone, int64(0), 1
	switch opt := strings.ToUpper(ctx.ArgString(1)); opt {
	case "EX", "PX", "EXAT", "PXAT":
		unitMS := int64(1)
		if opt == "EX" || opt == "EXAT" {
			unitMS = 1000
		}
		at, err = hfeExpireAt(ctx, ctx.ArgString(2), unitMS, opt == "EXAT" || opt == "PXAT")
		if err != nil {
			return ctx.WriteError(err)
		}
		mode, fieldsAt = hgetexExpire, 3
	case "PERSIST":
		mode, fieldsAt = hgetexPersist, 2
	}
	fields, err := hfeFields(ctx, fieldsAt)
	if err != nil {
		return ctx.WriteError(err)
	}
	if hash != nil {
		if mode == hgetexNone {
			hash.RLock()
		} else {
			hash, err = lockHash(ctx, key)
			if err != nil {
				return ctx.WriteError(err)
			}
		}
	}
	results := make([]*resp.Value, len(fields))
	if hash == nil {
		for i := range results {
			results[i] = resp.NullBulkString()
		}
		ctx.PropagateAs()
		return ctx.WriteArray(results)
	}
	deletes := ctx.Store.HashFieldExpiryDeletes()
	now := time.Now().UnixMilli()
	var deleted, updated, persisted []string
	for i, f := range fields {
		val, ok := hash.Fields[f]
		if !ok {
			results[i] = resp.NullBulkString()
			continue
		}
		results[i] = resp.BulkBytes(val)
		switch mode {
		case hgetexExpire:
			if deletes && at <= now {
				hash.DeleteField(f)
				deleted = append(deleted, f)
			} else {
				hash.SetFieldExpireAt(f, at)
				updated = append(updated, f)
			}
		case hgetexPersist:
			if hash.PersistField(f) {
				persisted = append(persisted, f)
			}
		}
	}
	if mode == hgetexNone {
		hash.RUnlock()
		ctx.PropagateAs()
		return ctx.WriteArray(results)
	}
	empty := len(hash.Fields) == 0
	hash.Unlock()
	switch {
	case len(deleted) > 0:
		ctx.notify(store.NotifyHash, "hdel", key)
	case len(updated) > 0:
		ctx.notify(store.NotifyHash, "hexpire", key)
	case len(persisted) > 0:
		ctx.notify(store.NotifyHash, "hpersist", key)
	}
	if empty {
		deleteIfEmpty(ctx, key, hash)
	} else if len(updated) > 0 {
		ctx.Store.ScheduleHashFieldExpiry(key, hash)
	}
	hfePropagation(ctx, key, deleted, at, updated, persisted)
	return ctx.WriteArray(results)
}

// cmdHSETEX is Redis 8's HSETEX key [FNX | FXX] [EX s | PX ms | EXAT t |
// PXAT t | KEEPTTL] FIELDS numfields field value ...: it sets the fields,
// all or none (FNX: only when none exists, FXX: only when all exist), and
// gives them the expiry, keeps theirs (KEEPTTL) or removes it.
func cmdHSETEX(ctx *Context) error {
	if ctx.ArgCount() < 5 {
		return ctx.WriteError(arityError(ctx.Command))
	}
	key := ctx.ArgString(0)
	var opts store.HashSetExOptions
	condSet, expirySet := false, false
	expiryArg := ""
	first, count := 0, 0
parse:
	for i := 1; i < ctx.ArgCount(); i++ {
		arg := ctx.ArgString(i)
		switch opt := strings.ToUpper(arg); opt {
		case "FIELDS":
			if i >= ctx.ArgCount()-3 {
				return ctx.WriteError(arityError(ctx.Command))
			}
			n, err := strconv.ParseInt(ctx.ArgString(i+1), 10, 64)
			if err != nil || n < 1 || n > 2147483647 {
				return ctx.WriteError(errHSETEXNumFields)
			}
			first, count = i+2, int(n)
			break parse
		case "FNX", "FXX":
			if condSet {
				return ctx.WriteError(errHSETEXCondition)
			}
			condSet = true
			opts.FNX, opts.FXX = opt == "FNX", opt == "FXX"
		case "KEEPTTL":
			if expirySet {
				return ctx.WriteError(errHSETEXExpiry)
			}
			expirySet, opts.KeepTTL = true, true
		case "EX", "PX", "EXAT", "PXAT":
			if expirySet {
				return ctx.WriteError(errHSETEXExpiry)
			}
			if i+1 >= ctx.ArgCount() {
				return ctx.WriteError(arityError(ctx.Command))
			}
			unitMS := int64(1)
			if opt == "EX" || opt == "EXAT" {
				unitMS = 1000
			}
			at, err := hfeExpireAt(ctx, ctx.ArgString(i+1), unitMS, opt == "EXAT" || opt == "PXAT")
			if err != nil {
				return ctx.WriteError(err)
			}
			expirySet, opts.ExpireAt, expiryArg = true, at, opt
			i++
		default:
			return ctx.WriteError(errors.New("ERR unknown argument: " + arg))
		}
	}
	if count == 0 || ctx.ArgCount()-first != count*2 {
		return ctx.WriteError(arityError(ctx.Command))
	}
	if _, err := getHash(ctx, key); err != nil {
		return ctx.WriteError(err)
	}
	pairs := ctx.Args[first:]
	ctx.writeEffectsHandled = true
	var applied bool
	var err error
	if expiryArg != "" && ctx.Store.HashFieldExpiryDeletes() && opts.ExpireAt <= time.Now().UnixMilli() {
		applied, err = hsetexDelete(ctx, key, pairs, opts)
		if err != nil {
			return ctx.WriteError(err)
		}
		if applied {
			fields := make([]string, 0, count)
			for i := 0; i < len(pairs); i += 2 {
				fields = append(fields, string(pairs[i]))
			}
			hfePropagation(ctx, key, fields, 0, nil, nil)
			ctx.Store.TouchKey(key)
		} else {
			ctx.PropagateAs()
		}
		return ctx.WriteInteger(boolInt(applied))
	}
	applied, err = ctx.Store.ApplyHashSetEx(key, pairs, opts)
	if err != nil {
		return ctx.WriteError(err)
	}
	if !applied {
		ctx.PropagateAs()
		return ctx.WriteInteger(0)
	}
	ctx.notify(store.NotifyHash, "hset", key)
	if expiryArg != "" {
		ctx.notify(store.NotifyHash, "hexpire", key)
	}
	args := []string{}
	if opts.KeepTTL {
		args = append(args, "KEEPTTL")
	} else if expiryArg != "" {
		args = append(args, "PXAT", strconv.FormatInt(opts.ExpireAt, 10))
	}
	args = append(args, "FIELDS", strconv.Itoa(count))
	for _, p := range pairs {
		args = append(args, string(p))
	}
	ctx.PropagateAs(ExecutedCommand{Name: "HSETEX", Args: hfeArgs(key, args...), DB: ctx.DB})
	return ctx.WriteInteger(1)
}

func boolInt(b bool) int64 {
	if b {
		return 1
	}
	return 0
}

// hsetexDelete is HSETEX with an expiry already in the past: the condition
// is checked as for a set, and the fields end up deleted.
func hsetexDelete(ctx *Context, key string, pairs [][]byte, opts store.HashSetExOptions) (bool, error) {
	hash, err := lockHash(ctx, key)
	if err != nil {
		return false, err
	}
	if hash == nil {
		if opts.FXX {
			return false, nil
		}
		ctx.notify(store.NotifyHash, "hset", key)
		ctx.notify(store.NotifyHash, "hdel", key)
		ctx.notify(store.NotifyGeneric, "del", key)
		return true, nil
	}
	for i := 0; i < len(pairs); i += 2 {
		_, exists := hash.Fields[string(pairs[i])]
		if (opts.FNX && exists) || (opts.FXX && !exists) {
			hash.Unlock()
			return false, nil
		}
	}
	for i := 0; i < len(pairs); i += 2 {
		hash.DeleteField(string(pairs[i]))
	}
	empty := len(hash.Fields) == 0
	hash.Unlock()
	ctx.notify(store.NotifyHash, "hset", key)
	ctx.notify(store.NotifyHash, "hdel", key)
	if empty {
		deleteIfEmpty(ctx, key, hash)
	}
	return true, nil
}
