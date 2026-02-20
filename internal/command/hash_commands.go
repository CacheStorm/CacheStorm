package command

import (
	"strconv"
	"strings"
	"time"

	"github.com/cachestorm/cachestorm/internal/resp"
	"github.com/cachestorm/cachestorm/internal/store"
)

func RegisterHashCommands(router *Router) {
	router.Register(&CommandDef{Name: "HSET", Handler: cmdHSET})
	router.Register(&CommandDef{Name: "HGET", Handler: cmdHGET})
	router.Register(&CommandDef{Name: "HMSET", Handler: cmdHMSET})
	router.Register(&CommandDef{Name: "HMGET", Handler: cmdHMGET})
	router.Register(&CommandDef{Name: "HGETALL", Handler: cmdHGETALL})
	router.Register(&CommandDef{Name: "HDEL", Handler: cmdHDEL})
	router.Register(&CommandDef{Name: "HEXISTS", Handler: cmdHEXISTS})
	router.Register(&CommandDef{Name: "HLEN", Handler: cmdHLEN})
	router.Register(&CommandDef{Name: "HKEYS", Handler: cmdHKEYS})
	router.Register(&CommandDef{Name: "HVALS", Handler: cmdHVALS})
	router.Register(&CommandDef{Name: "HINCRBY", Handler: cmdHINCRBY})
	router.Register(&CommandDef{Name: "HINCRBYFLOAT", Handler: cmdHINCRBYFLOAT})
	router.Register(&CommandDef{Name: "HSETNX", Handler: cmdHSETNX})
	router.Register(&CommandDef{Name: "HSTRLEN", Handler: cmdHSTRLEN})
	router.Register(&CommandDef{Name: "HSCAN", Handler: cmdHSCAN})
	router.Register(&CommandDef{Name: "HRANDFIELD", Handler: cmdHRANDFIELD})
	router.Register(&CommandDef{Name: "HGETDEL", Handler: cmdHGETDEL})
	router.Register(&CommandDef{Name: "HGETEX", Handler: cmdHGETEX})
}

func getOrCreateHash(ctx *Context, key string) (*store.HashValue, error) {
	entry, exists := ctx.Store.Get(key)
	if !exists {
		hash := &store.HashValue{Fields: make(map[string][]byte)}
		ctx.Store.Set(key, hash, store.SetOptions{})
		return hash, nil
	}

	hash, ok := entry.Value.(*store.HashValue)
	if !ok {
		return nil, store.ErrWrongType
	}

	return hash, nil
}

func getHash(ctx *Context, key string) (*store.HashValue, error) {
	entry, exists := ctx.Store.Get(key)
	if !exists {
		return nil, nil
	}

	hash, ok := entry.Value.(*store.HashValue)
	if !ok {
		return nil, store.ErrWrongType
	}

	return hash, nil
}

func cmdHSET(ctx *Context) error {
	if ctx.ArgCount() < 2 || ctx.ArgCount()%2 == 0 {
		return ctx.WriteError(ErrWrongArgCount)
	}

	key := ctx.ArgString(0)
	hash, err := getOrCreateHash(ctx, key)
	if err != nil {
		return ctx.WriteError(err)
	}

	hash.Lock()
	defer hash.Unlock()

	added := 0
	for i := 1; i < ctx.ArgCount(); i += 2 {
		field := ctx.ArgString(i)
		value := ctx.Arg(i + 1)
		if _, exists := hash.Fields[field]; !exists {
			added++
		}
		hash.Fields[field] = value
	}

	return ctx.WriteInteger(int64(added))
}

func cmdHGET(ctx *Context) error {
	if ctx.ArgCount() != 2 {
		return ctx.WriteError(ErrWrongArgCount)
	}

	key := ctx.ArgString(0)
	field := ctx.ArgString(1)

	hash, err := getHash(ctx, key)
	if err != nil {
		return ctx.WriteError(err)
	}
	if hash == nil {
		return ctx.WriteNullBulkString()
	}

	hash.RLock()
	defer hash.RUnlock()
	value, exists := hash.Fields[field]
	if !exists {
		return ctx.WriteNullBulkString()
	}

	return ctx.WriteBulkBytes(value)
}

func cmdHMSET(ctx *Context) error {
	if ctx.ArgCount() < 3 || ctx.ArgCount()%2 == 0 {
		return ctx.WriteError(ErrWrongArgCount)
	}

	key := ctx.ArgString(0)
	hash, err := getOrCreateHash(ctx, key)
	if err != nil {
		return ctx.WriteError(err)
	}

	hash.Lock()
	defer hash.Unlock()
	for i := 1; i < ctx.ArgCount(); i += 2 {
		field := ctx.ArgString(i)
		value := ctx.Arg(i + 1)
		hash.Fields[field] = value
	}

	return ctx.WriteOK()
}

func cmdHMGET(ctx *Context) error {
	if ctx.ArgCount() < 2 {
		return ctx.WriteError(ErrWrongArgCount)
	}

	key := ctx.ArgString(0)
	hash, err := getHash(ctx, key)
	if err != nil {
		return ctx.WriteError(err)
	}

	results := make([]*resp.Value, ctx.ArgCount()-1)
	if hash == nil {
		for i := 1; i < ctx.ArgCount(); i++ {
			results[i-1] = resp.NullBulkString()
		}
		return ctx.WriteArray(results)
	}

	hash.RLock()
	defer hash.RUnlock()
	for i := 1; i < ctx.ArgCount(); i++ {
		field := ctx.ArgString(i)
		value, exists := hash.Fields[field]
		if !exists {
			results[i-1] = resp.NullBulkString()
		} else {
			results[i-1] = resp.BulkBytes(value)
		}
	}

	return ctx.WriteArray(results)
}

func cmdHGETALL(ctx *Context) error {
	if ctx.ArgCount() != 1 {
		return ctx.WriteError(ErrWrongArgCount)
	}

	key := ctx.ArgString(0)
	hash, err := getHash(ctx, key)
	if err != nil {
		return ctx.WriteError(err)
	}
	if hash == nil {
		return ctx.WriteArray([]*resp.Value{})
	}

	hash.RLock()
	defer hash.RUnlock()
	results := make([]*resp.Value, 0, len(hash.Fields)*2)
	for field, value := range hash.Fields {
		results = append(results, resp.BulkString(field))
		results = append(results, resp.BulkBytes(value))
	}

	return ctx.WriteArray(results)
}

func cmdHDEL(ctx *Context) error {
	if ctx.ArgCount() < 2 {
		return ctx.WriteError(ErrWrongArgCount)
	}

	key := ctx.ArgString(0)
	hash, err := getHash(ctx, key)
	if err != nil {
		return ctx.WriteError(err)
	}
	if hash == nil {
		return ctx.WriteInteger(0)
	}

	hash.Lock()
	defer hash.Unlock()
	deleted := 0
	for i := 1; i < ctx.ArgCount(); i++ {
		field := ctx.ArgString(i)
		if _, exists := hash.Fields[field]; exists {
			delete(hash.Fields, field)
			deleted++
		}
	}

	if len(hash.Fields) == 0 {
		ctx.Store.Delete(key)
	}

	return ctx.WriteInteger(int64(deleted))
}

func cmdHEXISTS(ctx *Context) error {
	if ctx.ArgCount() != 2 {
		return ctx.WriteError(ErrWrongArgCount)
	}

	key := ctx.ArgString(0)
	field := ctx.ArgString(1)

	hash, err := getHash(ctx, key)
	if err != nil {
		return ctx.WriteError(err)
	}
	if hash == nil {
		return ctx.WriteInteger(0)
	}

	hash.RLock()
	defer hash.RUnlock()
	if _, exists := hash.Fields[field]; exists {
		return ctx.WriteInteger(1)
	}
	return ctx.WriteInteger(0)
}

func cmdHLEN(ctx *Context) error {
	if ctx.ArgCount() != 1 {
		return ctx.WriteError(ErrWrongArgCount)
	}

	key := ctx.ArgString(0)
	hash, err := getHash(ctx, key)
	if err != nil {
		return ctx.WriteError(err)
	}
	if hash == nil {
		return ctx.WriteInteger(0)
	}

	hash.RLock()
	defer hash.RUnlock()
	return ctx.WriteInteger(int64(len(hash.Fields)))
}

func cmdHKEYS(ctx *Context) error {
	if ctx.ArgCount() != 1 {
		return ctx.WriteError(ErrWrongArgCount)
	}

	key := ctx.ArgString(0)
	hash, err := getHash(ctx, key)
	if err != nil {
		return ctx.WriteError(err)
	}
	if hash == nil {
		return ctx.WriteArray([]*resp.Value{})
	}

	hash.RLock()
	defer hash.RUnlock()
	keys := make([]*resp.Value, 0, len(hash.Fields))
	for field := range hash.Fields {
		keys = append(keys, resp.BulkString(field))
	}

	return ctx.WriteArray(keys)
}

func cmdHVALS(ctx *Context) error {
	if ctx.ArgCount() != 1 {
		return ctx.WriteError(ErrWrongArgCount)
	}

	key := ctx.ArgString(0)
	hash, err := getHash(ctx, key)
	if err != nil {
		return ctx.WriteError(err)
	}
	if hash == nil {
		return ctx.WriteArray([]*resp.Value{})
	}

	hash.RLock()
	defer hash.RUnlock()
	vals := make([]*resp.Value, 0, len(hash.Fields))
	for _, value := range hash.Fields {
		vals = append(vals, resp.BulkBytes(value))
	}

	return ctx.WriteArray(vals)
}

func cmdHINCRBY(ctx *Context) error {
	if ctx.ArgCount() != 3 {
		return ctx.WriteError(ErrWrongArgCount)
	}

	key := ctx.ArgString(0)
	field := ctx.ArgString(1)
	incr, err := strconv.ParseInt(ctx.ArgString(2), 10, 64)
	if err != nil {
		return ctx.WriteError(ErrNotInteger)
	}

	hash, err := getOrCreateHash(ctx, key)
	if err != nil {
		return ctx.WriteError(err)
	}

	hash.Lock()
	defer hash.Unlock()
	var newVal int64
	if current, exists := hash.Fields[field]; exists {
		currentInt, err := strconv.ParseInt(string(current), 10, 64)
		if err != nil {
			return ctx.WriteError(ErrNotInteger)
		}
		newVal = currentInt + incr
	} else {
		newVal = incr
	}

	hash.Fields[field] = []byte(strconv.FormatInt(newVal, 10))
	return ctx.WriteInteger(newVal)
}

func cmdHINCRBYFLOAT(ctx *Context) error {
	if ctx.ArgCount() != 3 {
		return ctx.WriteError(ErrWrongArgCount)
	}

	key := ctx.ArgString(0)
	field := ctx.ArgString(1)
	incr, err := strconv.ParseFloat(ctx.ArgString(2), 64)
	if err != nil {
		return ctx.WriteError(ErrInvalidArg)
	}

	hash, err := getOrCreateHash(ctx, key)
	if err != nil {
		return ctx.WriteError(err)
	}

	hash.Lock()
	defer hash.Unlock()
	var newVal float64
	if current, exists := hash.Fields[field]; exists {
		currentFloat, err := strconv.ParseFloat(string(current), 64)
		if err != nil {
			return ctx.WriteError(ErrInvalidArg)
		}
		newVal = currentFloat + incr
	} else {
		newVal = incr
	}

	result := strconv.FormatFloat(newVal, 'f', -1, 64)
	hash.Fields[field] = []byte(result)
	return ctx.WriteBulkString(result)
}

func cmdHSETNX(ctx *Context) error {
	if ctx.ArgCount() != 3 {
		return ctx.WriteError(ErrWrongArgCount)
	}

	key := ctx.ArgString(0)
	field := ctx.ArgString(1)
	value := ctx.Arg(2)

	hash, err := getOrCreateHash(ctx, key)
	if err != nil {
		return ctx.WriteError(err)
	}

	hash.Lock()
	defer hash.Unlock()
	if _, exists := hash.Fields[field]; exists {
		return ctx.WriteInteger(0)
	}

	hash.Fields[field] = value
	return ctx.WriteInteger(1)
}

func cmdHSTRLEN(ctx *Context) error {
	if ctx.ArgCount() != 2 {
		return ctx.WriteError(ErrWrongArgCount)
	}

	key := ctx.ArgString(0)
	field := ctx.ArgString(1)

	hash, err := getHash(ctx, key)
	if err != nil {
		return ctx.WriteError(err)
	}
	if hash == nil {
		return ctx.WriteInteger(0)
	}

	hash.RLock()
	defer hash.RUnlock()
	value, exists := hash.Fields[field]
	if !exists {
		return ctx.WriteInteger(0)
	}

	return ctx.WriteInteger(int64(len(value)))
}

func cmdHSCAN(ctx *Context) error {
	if ctx.ArgCount() < 2 {
		return ctx.WriteError(ErrWrongArgCount)
	}

	key := ctx.ArgString(0)
	cursor, err := strconv.Atoi(ctx.ArgString(1))
	if err != nil {
		return ctx.WriteError(ErrNotInteger)
	}

	count := 10
	pattern := "*"

	for i := 2; i < ctx.ArgCount(); i++ {
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

	hash, err := getHash(ctx, key)
	if err != nil {
		return ctx.WriteError(err)
	}
	if hash == nil {
		return ctx.WriteArray([]*resp.Value{
			resp.BulkString("0"),
			resp.ArrayValue([]*resp.Value{}),
		})
	}

	hash.RLock()
	defer hash.RUnlock()
	fields := make([]string, 0, len(hash.Fields))
	for field := range hash.Fields {
		if matchPattern(field, pattern) {
			fields = append(fields, field)
		}
	}

	start := cursor
	if start >= len(fields) {
		start = 0
	}

	end := start + count
	if end > len(fields) {
		end = len(fields)
	}

	nextCursor := 0
	if end < len(fields) {
		nextCursor = end
	}

	result := make([]*resp.Value, 0, (end-start)*2)
	for i := start; i < end; i++ {
		field := fields[i]
		result = append(result, resp.BulkString(field))
		result = append(result, resp.BulkBytes(hash.Fields[field]))
	}

	return ctx.WriteArray([]*resp.Value{
		resp.BulkString(strconv.Itoa(nextCursor)),
		resp.ArrayValue(result),
	})
}

func cmdHRANDFIELD(ctx *Context) error {
	if ctx.ArgCount() < 1 {
		return ctx.WriteError(ErrWrongArgCount)
	}

	key := ctx.ArgString(0)
	count := 1
	withValues := false

	for i := 1; i < ctx.ArgCount(); i++ {
		arg := strings.ToUpper(ctx.ArgString(i))
		switch arg {
		case "WITHVALUES":
			withValues = true
		default:
			var err error
			count, err = strconv.Atoi(ctx.ArgString(i))
			if err != nil {
				return ctx.WriteError(ErrNotInteger)
			}
		}
	}

	hash, err := getHash(ctx, key)
	if err != nil {
		return ctx.WriteError(err)
	}
	if hash == nil {
		return ctx.WriteNullBulkString()
	}

	hash.RLock()
	fields := make([]string, 0, len(hash.Fields))
	for f := range hash.Fields {
		fields = append(fields, f)
	}
	hash.RUnlock()

	if count == 0 || len(fields) == 0 {
		return ctx.WriteNullBulkString()
	}

	if count == 1 && !withValues {
		return ctx.WriteBulkString(fields[0])
	}

	hash.RLock()
	defer hash.RUnlock()
	result := make([]*resp.Value, 0, count*2)
	if count > 0 {
		for i := 0; i < count && i < len(fields); i++ {
			field := fields[i]
			result = append(result, resp.BulkString(field))
			if withValues {
				result = append(result, resp.BulkBytes(hash.Fields[field]))
			}
		}
	} else {
		for i := 0; i < -count; i++ {
			field := fields[i%len(fields)]
			result = append(result, resp.BulkString(field))
			if withValues {
				result = append(result, resp.BulkBytes(hash.Fields[field]))
			}
		}
	}

	return ctx.WriteArray(result)
}

func cmdHGETDEL(ctx *Context) error {
	if ctx.ArgCount() < 2 {
		return ctx.WriteError(ErrWrongArgCount)
	}

	key := ctx.ArgString(0)
	fields := make([]string, 0, ctx.ArgCount()-1)
	for i := 1; i < ctx.ArgCount(); i++ {
		fields = append(fields, ctx.ArgString(i))
	}

	hash, err := getHash(ctx, key)
	if err != nil {
		return ctx.WriteError(err)
	}
	if hash == nil {
		if len(fields) == 1 {
			return ctx.WriteNullBulkString()
		}
		results := make([]*resp.Value, len(fields))
		for i := range results {
			results[i] = resp.NullValue()
		}
		return ctx.WriteArray(results)
	}

	hash.Lock()
	defer hash.Unlock()
	if len(fields) == 1 {
		value, exists := hash.Fields[fields[0]]
		if !exists {
			return ctx.WriteNullBulkString()
		}
		delete(hash.Fields, fields[0])
		if len(hash.Fields) == 0 {
			ctx.Store.Delete(key)
		}
		return ctx.WriteBulkBytes(value)
	}

	results := make([]*resp.Value, 0, len(fields))
	for _, field := range fields {
		if value, exists := hash.Fields[field]; exists {
			results = append(results, resp.BulkBytes(value))
			delete(hash.Fields, field)
		} else {
			results = append(results, resp.NullValue())
		}
	}
	if len(hash.Fields) == 0 {
		ctx.Store.Delete(key)
	}
	return ctx.WriteArray(results)
}

func cmdHGETEX(ctx *Context) error {
	if ctx.ArgCount() < 2 {
		return ctx.WriteError(ErrWrongArgCount)
	}

	key := ctx.ArgString(0)
	fields := make([]string, 0)

	for i := 1; i < ctx.ArgCount(); i++ {
		arg := strings.ToUpper(ctx.ArgString(i))
		switch arg {
		case "EX", "PX", "EXAT", "PXAT", "PERSIST":
			break
		default:
			if strings.HasPrefix(arg, "F") && i+1 < ctx.ArgCount() {
				i++
				fields = append(fields, ctx.ArgString(i))
			} else if !strings.ContainsAny(arg, "0123456789") {
				fields = append(fields, ctx.ArgString(i))
			}
		}
	}

	hash, err := getHash(ctx, key)
	if err != nil {
		return ctx.WriteError(err)
	}
	if hash == nil {
		if len(fields) == 1 {
			return ctx.WriteNullBulkString()
		}
		results := make([]*resp.Value, len(fields))
		for i := range results {
			results[i] = resp.NullValue()
		}
		return ctx.WriteArray(results)
	}

	for i := 1; i < ctx.ArgCount(); i++ {
		arg := strings.ToUpper(ctx.ArgString(i))
		switch arg {
		case "EX":
			i++
			if i >= ctx.ArgCount() {
				return ctx.WriteError(ErrSyntaxError)
			}
			sec, err := strconv.ParseInt(ctx.ArgString(i), 10, 64)
			if err != nil {
				return ctx.WriteError(ErrNotInteger)
			}
			ctx.Store.SetTTL(key, time.Duration(sec)*time.Second)
		case "PX":
			i++
			if i >= ctx.ArgCount() {
				return ctx.WriteError(ErrSyntaxError)
			}
			ms, err := strconv.ParseInt(ctx.ArgString(i), 10, 64)
			if err != nil {
				return ctx.WriteError(ErrNotInteger)
			}
			ctx.Store.SetTTL(key, time.Duration(ms)*time.Millisecond)
		case "PERSIST":
			ctx.Store.Persist(key)
		}
	}

	if len(fields) == 0 {
		return ctx.WriteNullBulkString()
	}

	hash.RLock()
	defer hash.RUnlock()
	if len(fields) == 1 {
		value, exists := hash.Fields[fields[0]]
		if !exists {
			return ctx.WriteNullBulkString()
		}
		return ctx.WriteBulkBytes(value)
	}

	results := make([]*resp.Value, 0, len(fields))
	for _, field := range fields {
		if value, exists := hash.Fields[field]; exists {
			results = append(results, resp.BulkBytes(value))
		} else {
			results = append(results, resp.NullValue())
		}
	}
	return ctx.WriteArray(results)
}
