package command

import (
	"strconv"
	"strings"
	"time"

	"github.com/cachestorm/cachestorm/internal/resp"
	"github.com/cachestorm/cachestorm/internal/store"
)

func RegisterStringCommands(router *Router) {
	router.Register(&CommandDef{Name: "SET", Handler: cmdSET})
	router.Register(&CommandDef{Name: "GET", Handler: cmdGET})
	router.Register(&CommandDef{Name: "DEL", Handler: cmdDEL})
	router.Register(&CommandDef{Name: "EXISTS", Handler: cmdEXISTS})
	router.Register(&CommandDef{Name: "MSET", Handler: cmdMSET})
	router.Register(&CommandDef{Name: "MGET", Handler: cmdMGET})
	router.Register(&CommandDef{Name: "INCR", Handler: cmdINCR})
	router.Register(&CommandDef{Name: "DECR", Handler: cmdDECR})
	router.Register(&CommandDef{Name: "INCRBY", Handler: cmdINCRBY})
	router.Register(&CommandDef{Name: "DECRBY", Handler: cmdDECRBY})
	router.Register(&CommandDef{Name: "INCRBYFLOAT", Handler: cmdINCRBYFLOAT})
	router.Register(&CommandDef{Name: "APPEND", Handler: cmdAPPEND})
	router.Register(&CommandDef{Name: "STRLEN", Handler: cmdSTRLEN})
	router.Register(&CommandDef{Name: "GETRANGE", Handler: cmdGETRANGE})
	router.Register(&CommandDef{Name: "SETRANGE", Handler: cmdSETRANGE})
	router.Register(&CommandDef{Name: "SETNX", Handler: cmdSETNX})
	router.Register(&CommandDef{Name: "SETEX", Handler: cmdSETEX})
	router.Register(&CommandDef{Name: "PSETEX", Handler: cmdPSETEX})
	router.Register(&CommandDef{Name: "MSETNX", Handler: cmdMSETNX})
	router.Register(&CommandDef{Name: "GETSET", Handler: cmdGETSET})
	router.Register(&CommandDef{Name: "GETDEL", Handler: cmdGETDEL})
	router.Register(&CommandDef{Name: "GETEX", Handler: cmdGETEX})
	router.Register(&CommandDef{Name: "LCS", Handler: cmdLCS})
	router.Register(&CommandDef{Name: "SUBSTR", Handler: cmdGETRANGE})
}

func cmdSET(ctx *Context) error {
	if ctx.ArgCount() < 2 {
		return ctx.WriteError(ErrWrongArgCount)
	}

	key := ctx.ArgString(0)
	value := ctx.Arg(1)

	opts := store.SetOptions{}
	args := ctx.Args[2:]
	getOldValue := false

	for i := 0; i < len(args); i++ {
		arg := string(args[i])
		switch arg {
		case "EX":
			i++
			if i >= len(args) {
				return ctx.WriteError(ErrSyntaxError)
			}
			sec, err := strconv.ParseInt(string(args[i]), 10, 64)
			if err != nil {
				return ctx.WriteError(ErrNotInteger)
			}
			opts.TTL = time.Duration(sec) * time.Second
		case "PX":
			i++
			if i >= len(args) {
				return ctx.WriteError(ErrSyntaxError)
			}
			ms, err := strconv.ParseInt(string(args[i]), 10, 64)
			if err != nil {
				return ctx.WriteError(ErrNotInteger)
			}
			opts.TTL = time.Duration(ms) * time.Millisecond
		case "EXAT":
			i++
			if i >= len(args) {
				return ctx.WriteError(ErrSyntaxError)
			}
			ts, err := strconv.ParseInt(string(args[i]), 10, 64)
			if err != nil {
				return ctx.WriteError(ErrNotInteger)
			}
			opts.TTL = time.Until(time.Unix(ts, 0))
		case "PXAT":
			i++
			if i >= len(args) {
				return ctx.WriteError(ErrSyntaxError)
			}
			ts, err := strconv.ParseInt(string(args[i]), 10, 64)
			if err != nil {
				return ctx.WriteError(ErrNotInteger)
			}
			opts.TTL = time.Until(time.Unix(0, ts*int64(time.Millisecond)))
		case "NX":
			opts.NX = true
		case "XX":
			opts.XX = true
		case "KEEPTTL":
			opts.KeepTTL = true
		case "GET":
			getOldValue = true
		default:
			return ctx.WriteError(ErrSyntaxError)
		}
	}

	if opts.NX && opts.XX {
		return ctx.WriteError(ErrSyntaxError)
	}

	if getOldValue {
		entry, exists := ctx.Store.Get(key)
		if exists {
			strVal, ok := entry.Value.(*store.StringValue)
			if !ok {
				return ctx.WriteError(store.ErrWrongType)
			}
			oldValue := make([]byte, len(strVal.Data))
			copy(oldValue, strVal.Data)

			err := ctx.Store.Set(key, &store.StringValue{Data: value}, opts)
			if err != nil {
				return ctx.WriteError(err)
			}
			return ctx.WriteBulkBytes(oldValue)
		}

		err := ctx.Store.Set(key, &store.StringValue{Data: value}, opts)
		if err != nil {
			return ctx.WriteError(err)
		}
		return ctx.WriteNullBulkString()
	}

	err := ctx.Store.Set(key, &store.StringValue{Data: value}, opts)
	if err != nil {
		if err == store.ErrKeyExists {
			return ctx.WriteNull()
		}
		if err == store.ErrKeyNotFound {
			return ctx.WriteNull()
		}
		return ctx.WriteError(err)
	}

	return ctx.WriteOK()
}

func cmdGET(ctx *Context) error {
	if ctx.ArgCount() != 1 {
		return ctx.WriteError(ErrWrongArgCount)
	}

	key := ctx.ArgString(0)
	entry, exists := ctx.Store.Get(key)
	if !exists {
		return ctx.WriteNullBulkString()
	}

	strVal, ok := entry.Value.(*store.StringValue)
	if !ok {
		return ctx.WriteError(store.ErrWrongType)
	}

	return ctx.WriteBulkBytes(strVal.Data)
}

func cmdDEL(ctx *Context) error {
	if ctx.ArgCount() < 1 {
		return ctx.WriteError(ErrWrongArgCount)
	}

	deleted := int64(0)
	for i := 0; i < ctx.ArgCount(); i++ {
		if ctx.Store.Delete(ctx.ArgString(i)) {
			deleted++
		}
	}

	return ctx.WriteInteger(deleted)
}

func cmdEXISTS(ctx *Context) error {
	if ctx.ArgCount() < 1 {
		return ctx.WriteError(ErrWrongArgCount)
	}

	count := int64(0)
	for i := 0; i < ctx.ArgCount(); i++ {
		if ctx.Store.Exists(ctx.ArgString(i)) {
			count++
		}
	}

	return ctx.WriteInteger(count)
}

func cmdMSET(ctx *Context) error {
	if ctx.ArgCount() < 2 || ctx.ArgCount()%2 != 0 {
		return ctx.WriteError(ErrWrongArgCount)
	}

	for i := 0; i < ctx.ArgCount(); i += 2 {
		key := ctx.ArgString(i)
		value := ctx.Arg(i + 1)
		ctx.Store.Set(key, &store.StringValue{Data: value}, store.SetOptions{})
	}

	return ctx.WriteOK()
}

func cmdMGET(ctx *Context) error {
	if ctx.ArgCount() < 1 {
		return ctx.WriteError(ErrWrongArgCount)
	}

	results := make([]*resp.Value, ctx.ArgCount())
	for i := 0; i < ctx.ArgCount(); i++ {
		entry, exists := ctx.Store.Get(ctx.ArgString(i))
		if !exists {
			results[i] = resp.NullBulkString()
			continue
		}
		strVal, ok := entry.Value.(*store.StringValue)
		if !ok {
			results[i] = resp.NullBulkString()
			continue
		}
		results[i] = resp.BulkBytes(strVal.Data)
	}

	return ctx.WriteArray(results)
}

func cmdINCR(ctx *Context) error {
	return incrBy(ctx, 1)
}

func cmdDECR(ctx *Context) error {
	return incrBy(ctx, -1)
}

func cmdINCRBY(ctx *Context) error {
	if ctx.ArgCount() != 2 {
		return ctx.WriteError(ErrWrongArgCount)
	}
	incr, err := strconv.ParseInt(ctx.ArgString(1), 10, 64)
	if err != nil {
		return ctx.WriteError(ErrNotInteger)
	}
	return incrBy(ctx, incr)
}

func cmdDECRBY(ctx *Context) error {
	if ctx.ArgCount() != 2 {
		return ctx.WriteError(ErrWrongArgCount)
	}
	decr, err := strconv.ParseInt(ctx.ArgString(1), 10, 64)
	if err != nil {
		return ctx.WriteError(ErrNotInteger)
	}
	return incrBy(ctx, -decr)
}

func incrBy(ctx *Context, incr int64) error {
	if ctx.ArgCount() < 1 {
		return ctx.WriteError(ErrWrongArgCount)
	}

	key := ctx.ArgString(0)
	entry, exists := ctx.Store.Get(key)

	var newVal int64
	if !exists {
		newVal = incr
	} else {
		strVal, ok := entry.Value.(*store.StringValue)
		if !ok {
			return ctx.WriteError(store.ErrWrongType)
		}
		current, err := strconv.ParseInt(string(strVal.Data), 10, 64)
		if err != nil {
			return ctx.WriteError(ErrNotInteger)
		}
		newVal = current + incr
	}

	ctx.Store.Set(key, &store.StringValue{Data: []byte(strconv.FormatInt(newVal, 10))}, store.SetOptions{})
	return ctx.WriteInteger(newVal)
}

func cmdAPPEND(ctx *Context) error {
	if ctx.ArgCount() != 2 {
		return ctx.WriteError(ErrWrongArgCount)
	}

	key := ctx.ArgString(0)
	suffix := ctx.Arg(1)

	entry, exists := ctx.Store.Get(key)
	var newData []byte
	if !exists {
		newData = suffix
	} else {
		strVal, ok := entry.Value.(*store.StringValue)
		if !ok {
			return ctx.WriteError(store.ErrWrongType)
		}
		newData = append(strVal.Data, suffix...)
	}

	ctx.Store.Set(key, &store.StringValue{Data: newData}, store.SetOptions{})
	return ctx.WriteInteger(int64(len(newData)))
}

func cmdSTRLEN(ctx *Context) error {
	if ctx.ArgCount() != 1 {
		return ctx.WriteError(ErrWrongArgCount)
	}

	key := ctx.ArgString(0)
	entry, exists := ctx.Store.Get(key)
	if !exists {
		return ctx.WriteInteger(0)
	}

	strVal, ok := entry.Value.(*store.StringValue)
	if !ok {
		return ctx.WriteError(store.ErrWrongType)
	}

	return ctx.WriteInteger(int64(len(strVal.Data)))
}

func cmdGETRANGE(ctx *Context) error {
	if ctx.ArgCount() != 3 {
		return ctx.WriteError(ErrWrongArgCount)
	}

	key := ctx.ArgString(0)
	start, err := strconv.Atoi(ctx.ArgString(1))
	if err != nil {
		return ctx.WriteError(ErrNotInteger)
	}
	end, err := strconv.Atoi(ctx.ArgString(2))
	if err != nil {
		return ctx.WriteError(ErrNotInteger)
	}

	entry, exists := ctx.Store.Get(key)
	if !exists {
		return ctx.WriteBulkString("")
	}

	strVal, ok := entry.Value.(*store.StringValue)
	if !ok {
		return ctx.WriteError(store.ErrWrongType)
	}

	data := strVal.Data
	length := len(data)
	if length == 0 {
		return ctx.WriteBulkString("")
	}

	if start < 0 {
		start = length + start
	}
	if end < 0 {
		end = length + end
	}
	if start < 0 {
		start = 0
	}
	if end >= length {
		end = length - 1
	}
	if start > end {
		return ctx.WriteBulkString("")
	}

	return ctx.WriteBulkBytes(data[start : end+1])
}

func cmdSETRANGE(ctx *Context) error {
	if ctx.ArgCount() != 3 {
		return ctx.WriteError(ErrWrongArgCount)
	}

	key := ctx.ArgString(0)
	offset, err := strconv.Atoi(ctx.ArgString(1))
	if err != nil {
		return ctx.WriteError(ErrNotInteger)
	}
	if offset < 0 {
		return ctx.WriteError(ErrInvalidArg)
	}
	value := ctx.Arg(2)

	entry, exists := ctx.Store.Get(key)
	var data []byte
	if !exists {
		data = []byte{}
	} else {
		strVal, ok := entry.Value.(*store.StringValue)
		if !ok {
			return ctx.WriteError(store.ErrWrongType)
		}
		data = strVal.Data
	}

	newLen := offset + len(value)
	if newLen > len(data) {
		newData := make([]byte, newLen)
		copy(newData, data)
		data = newData
	}

	copy(data[offset:], value)
	ctx.Store.Set(key, &store.StringValue{Data: data}, store.SetOptions{})
	return ctx.WriteInteger(int64(len(data)))
}

func cmdSETNX(ctx *Context) error {
	if ctx.ArgCount() != 2 {
		return ctx.WriteError(ErrWrongArgCount)
	}

	key := ctx.ArgString(0)
	value := ctx.Arg(1)

	err := ctx.Store.Set(key, &store.StringValue{Data: value}, store.SetOptions{NX: true})
	if err != nil {
		if err == store.ErrKeyExists {
			return ctx.WriteInteger(0)
		}
		return ctx.WriteError(err)
	}

	return ctx.WriteInteger(1)
}

func cmdSETEX(ctx *Context) error {
	if ctx.ArgCount() != 3 {
		return ctx.WriteError(ErrWrongArgCount)
	}

	key := ctx.ArgString(0)
	sec, err := strconv.ParseInt(ctx.ArgString(1), 10, 64)
	if err != nil {
		return ctx.WriteError(ErrNotInteger)
	}
	if sec <= 0 {
		return ctx.WriteError(ErrInvalidArg)
	}
	value := ctx.Arg(2)

	ctx.Store.Set(key, &store.StringValue{Data: value}, store.SetOptions{TTL: time.Duration(sec) * time.Second})
	return ctx.WriteOK()
}

func cmdPSETEX(ctx *Context) error {
	if ctx.ArgCount() != 3 {
		return ctx.WriteError(ErrWrongArgCount)
	}

	key := ctx.ArgString(0)
	ms, err := strconv.ParseInt(ctx.ArgString(1), 10, 64)
	if err != nil {
		return ctx.WriteError(ErrNotInteger)
	}
	if ms <= 0 {
		return ctx.WriteError(ErrInvalidArg)
	}
	value := ctx.Arg(2)

	ctx.Store.Set(key, &store.StringValue{Data: value}, store.SetOptions{TTL: time.Duration(ms) * time.Millisecond})
	return ctx.WriteOK()
}

func cmdMSETNX(ctx *Context) error {
	if ctx.ArgCount() < 2 || ctx.ArgCount()%2 != 0 {
		return ctx.WriteError(ErrWrongArgCount)
	}

	keys := make([]string, 0, ctx.ArgCount()/2)
	for i := 0; i < ctx.ArgCount(); i += 2 {
		key := ctx.ArgString(i)
		if ctx.Store.Exists(key) {
			return ctx.WriteInteger(0)
		}
		keys = append(keys, key)
	}

	for i := 0; i < ctx.ArgCount(); i += 2 {
		key := ctx.ArgString(i)
		value := ctx.Arg(i + 1)
		ctx.Store.Set(key, &store.StringValue{Data: value}, store.SetOptions{})
	}

	return ctx.WriteInteger(1)
}

func cmdGETSET(ctx *Context) error {
	if ctx.ArgCount() != 2 {
		return ctx.WriteError(ErrWrongArgCount)
	}

	key := ctx.ArgString(0)
	newValue := ctx.Arg(1)

	entry, exists := ctx.Store.Get(key)

	ctx.Store.Set(key, &store.StringValue{Data: newValue}, store.SetOptions{})

	if !exists {
		return ctx.WriteNullBulkString()
	}

	strVal, ok := entry.Value.(*store.StringValue)
	if !ok {
		return ctx.WriteError(store.ErrWrongType)
	}

	return ctx.WriteBulkBytes(strVal.Data)
}

func cmdGETDEL(ctx *Context) error {
	if ctx.ArgCount() != 1 {
		return ctx.WriteError(ErrWrongArgCount)
	}

	key := ctx.ArgString(0)
	entry, exists := ctx.Store.Get(key)
	if !exists {
		return ctx.WriteNullBulkString()
	}

	strVal, ok := entry.Value.(*store.StringValue)
	if !ok {
		return ctx.WriteError(store.ErrWrongType)
	}

	result := make([]byte, len(strVal.Data))
	copy(result, strVal.Data)

	ctx.Store.Delete(key)

	return ctx.WriteBulkBytes(result)
}

func cmdGETEX(ctx *Context) error {
	if ctx.ArgCount() < 1 {
		return ctx.WriteError(ErrWrongArgCount)
	}

	key := ctx.ArgString(0)
	entry, exists := ctx.Store.Get(key)
	if !exists {
		return ctx.WriteNullBulkString()
	}

	strVal, ok := entry.Value.(*store.StringValue)
	if !ok {
		return ctx.WriteError(store.ErrWrongType)
	}

	result := make([]byte, len(strVal.Data))
	copy(result, strVal.Data)

	if ctx.ArgCount() > 1 {
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
			case "EXAT":
				i++
				if i >= ctx.ArgCount() {
					return ctx.WriteError(ErrSyntaxError)
				}
				ts, err := strconv.ParseInt(ctx.ArgString(i), 10, 64)
				if err != nil {
					return ctx.WriteError(ErrNotInteger)
				}
				ctx.Store.SetExpiresAt(key, time.Unix(ts, 0).UnixNano())
			case "PXAT":
				i++
				if i >= ctx.ArgCount() {
					return ctx.WriteError(ErrSyntaxError)
				}
				ts, err := strconv.ParseInt(ctx.ArgString(i), 10, 64)
				if err != nil {
					return ctx.WriteError(ErrNotInteger)
				}
				ctx.Store.SetExpiresAt(key, ts*int64(time.Millisecond))
			case "PERSIST":
				ctx.Store.Persist(key)
			default:
				return ctx.WriteError(ErrSyntaxError)
			}
		}
	}

	return ctx.WriteBulkBytes(result)
}

func cmdINCRBYFLOAT(ctx *Context) error {
	if ctx.ArgCount() != 2 {
		return ctx.WriteError(ErrWrongArgCount)
	}

	key := ctx.ArgString(0)
	incr, err := strconv.ParseFloat(ctx.ArgString(1), 64)
	if err != nil {
		return ctx.WriteError(ErrInvalidArg)
	}

	entry, exists := ctx.Store.Get(key)
	if !exists {
		result := strconv.FormatFloat(incr, 'f', -1, 64)
		ctx.Store.Set(key, &store.StringValue{Data: []byte(result)}, store.SetOptions{})
		return ctx.WriteBulkString(result)
	}

	strVal, ok := entry.Value.(*store.StringValue)
	if !ok {
		return ctx.WriteError(store.ErrWrongType)
	}

	current, err := strconv.ParseFloat(string(strVal.Data), 64)
	if err != nil {
		return ctx.WriteError(ErrInvalidArg)
	}

	result := strconv.FormatFloat(current+incr, 'f', -1, 64)
	ctx.Store.Set(key, &store.StringValue{Data: []byte(result)}, store.SetOptions{})
	return ctx.WriteBulkString(result)
}

func cmdLCS(ctx *Context) error {
	if ctx.ArgCount() < 2 {
		return ctx.WriteError(ErrWrongArgCount)
	}

	key1 := ctx.ArgString(0)
	key2 := ctx.ArgString(1)

	idx := false
	lenOnly := false
	minMatchLen := 1
	withMatchLen := false

	for i := 2; i < ctx.ArgCount(); i++ {
		arg := strings.ToUpper(ctx.ArgString(i))
		switch arg {
		case "IDX":
			idx = true
		case "LEN":
			lenOnly = true
		case "MINMATCHLEN":
			i++
			if i >= ctx.ArgCount() {
				return ctx.WriteError(ErrSyntaxError)
			}
			var err error
			minMatchLen, err = strconv.Atoi(ctx.ArgString(i))
			if err != nil {
				return ctx.WriteError(ErrNotInteger)
			}
			if minMatchLen < 1 {
				minMatchLen = 1
			}
		case "WITHMATCHLEN":
			withMatchLen = true
		default:
			return ctx.WriteError(ErrSyntaxError)
		}
	}

	entry1, exists1 := ctx.Store.Get(key1)
	entry2, exists2 := ctx.Store.Get(key2)

	if !exists1 || !exists2 {
		if lenOnly {
			return ctx.WriteInteger(0)
		}
		return ctx.WriteBulkString("")
	}

	strVal1, ok1 := entry1.Value.(*store.StringValue)
	strVal2, ok2 := entry2.Value.(*store.StringValue)

	if !ok1 || !ok2 {
		return ctx.WriteError(store.ErrWrongType)
	}

	s1 := string(strVal1.Data)
	s2 := string(strVal2.Data)

	if len(s1) == 0 || len(s2) == 0 {
		if lenOnly {
			return ctx.WriteInteger(0)
		}
		return ctx.WriteBulkString("")
	}

	m := len(s1)
	n := len(s2)

	dp := make([][]int, m+1)
	for i := range dp {
		dp[i] = make([]int, n+1)
	}

	for i := 1; i <= m; i++ {
		for j := 1; j <= n; j++ {
			if s1[i-1] == s2[j-1] {
				dp[i][j] = dp[i-1][j-1] + 1
			} else if dp[i-1][j] > dp[i][j-1] {
				dp[i][j] = dp[i-1][j]
			} else {
				dp[i][j] = dp[i][j-1]
			}
		}
	}

	lcsLen := dp[m][n]

	if lenOnly {
		return ctx.WriteInteger(int64(lcsLen))
	}

	if !idx {
		result := make([]byte, 0, lcsLen)
		i, j := m, n
		for i > 0 && j > 0 {
			if s1[i-1] == s2[j-1] {
				result = append([]byte{s1[i-1]}, result...)
				i--
				j--
			} else if dp[i-1][j] > dp[i][j-1] {
				i--
			} else {
				j--
			}
		}
		return ctx.WriteBulkString(string(result))
	}

	type Match struct {
		A1, A2 int
		B1, B2 int
		Len    int
	}
	matches := make([]Match, 0)

	i, j := m, n
	for i > 0 && j > 0 {
		if s1[i-1] == s2[j-1] {
			endI, endJ := i, j
			for i > 0 && j > 0 && s1[i-1] == s2[j-1] {
				i--
				j--
			}
			matchLen := endI - i
			if matchLen >= minMatchLen {
				matches = append([]Match{{A1: i, A2: endI - 1, B1: j, B2: endJ - 1, Len: matchLen}}, matches...)
			}
		} else if dp[i-1][j] > dp[i][j-1] {
			i--
		} else {
			j--
		}
	}

	result := make([]*resp.Value, 0, len(matches)*2+2)
	result = append(result, resp.BulkString("matches"))
	matchArray := make([]*resp.Value, 0, len(matches))
	for _, match := range matches {
		matchEntry := []*resp.Value{
			resp.ArrayValue([]*resp.Value{
				resp.IntegerValue(int64(match.A1)),
				resp.IntegerValue(int64(match.A2)),
			}),
			resp.ArrayValue([]*resp.Value{
				resp.IntegerValue(int64(match.B1)),
				resp.IntegerValue(int64(match.B2)),
			}),
		}
		if withMatchLen {
			matchEntry = append(matchEntry, resp.IntegerValue(int64(match.Len)))
		}
		matchArray = append(matchArray, resp.ArrayValue(matchEntry))
	}
	result = append(result, resp.ArrayValue(matchArray))
	result = append(result, resp.BulkString("len"))
	result = append(result, resp.IntegerValue(int64(lcsLen)))

	return ctx.WriteArray(result)
}
