package command

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/cachestorm/cachestorm/internal/resp"
	"github.com/cachestorm/cachestorm/internal/store"
)

func RegisterJSONCommands(router *Router) {
	router.Register(&CommandDef{Name: "JSON.GET", Handler: cmdJSONGET})
	router.Register(&CommandDef{Name: "JSON.SET", Handler: cmdJSONSET})
	router.Register(&CommandDef{Name: "JSON.DEL", Handler: cmdJSONDEL})
	router.Register(&CommandDef{Name: "JSON.TYPE", Handler: cmdJSONTYPE})
	router.Register(&CommandDef{Name: "JSON.NUMINCRBY", Handler: cmdJSONNUMINCRBY})
	router.Register(&CommandDef{Name: "JSON.NUMMULTBY", Handler: cmdJSONNUMMULTBY})
	router.Register(&CommandDef{Name: "JSON.STRAPPEND", Handler: cmdJSONSTRAPPEND})
	router.Register(&CommandDef{Name: "JSON.STRLEN", Handler: cmdJSONSTRLEN})
	router.Register(&CommandDef{Name: "JSON.ARRAPPEND", Handler: cmdJSONARRAPPEND})
	router.Register(&CommandDef{Name: "JSON.ARRLEN", Handler: cmdJSONARRLEN})
	router.Register(&CommandDef{Name: "JSON.OBJLEN", Handler: cmdJSONOBJLEN})
	router.Register(&CommandDef{Name: "JSON.OBJKEYS", Handler: cmdJSONOBJKEYS})
	router.Register(&CommandDef{Name: "JSON.MGET", Handler: cmdJSONMGET})
	router.Register(&CommandDef{Name: "JSON.MSET", Handler: cmdJSONMSET})
}

func getOrCreateJSONValue(s *store.Store, key string) (*store.JSONValue, error) {
	entry, exists := s.Get(key)
	if !exists {
		return store.NewJSONValue(make(map[string]interface{}))
	}

	if jv, ok := entry.Value.(*store.JSONValue); ok {
		return jv, nil
	}

	return store.NewJSONValue(entry.Value.String())
}

func cmdJSONGET(ctx *Context) error {
	if ctx.ArgCount() < 1 {
		return ctx.WriteError(ErrWrongArgCount)
	}

	key := ctx.ArgString(0)
	path := "$"
	if ctx.ArgCount() >= 2 {
		path = ctx.ArgString(1)
	}

	jv, err := getOrCreateJSONValue(ctx.Store, key)
	if err != nil {
		return ctx.WriteError(err)
	}

	result, err := jv.GetPath(path)
	if err != nil {
		return ctx.WriteError(err)
	}

	if result == nil {
		return ctx.WriteNull()
	}

	b, err := json.Marshal(result)
	if err != nil {
		return ctx.WriteError(err)
	}

	return ctx.WriteBulkBytes(b)
}

func cmdJSONSET(ctx *Context) error {
	if ctx.ArgCount() < 3 {
		return ctx.WriteError(ErrWrongArgCount)
	}

	key := ctx.ArgString(0)
	path := ctx.ArgString(1)
	jsonStr := ctx.ArgString(2)

	var value interface{}
	if err := json.Unmarshal([]byte(jsonStr), &value); err != nil {
		return ctx.WriteError(fmt.Errorf("ERR invalid JSON: %v", err))
	}

	jv, err := getOrCreateJSONValue(ctx.Store, key)
	if err != nil {
		return ctx.WriteError(err)
	}

	if err := jv.SetPath(path, value); err != nil {
		return ctx.WriteError(err)
	}

	ctx.Store.Set(key, jv, store.SetOptions{})
	return ctx.WriteOK()
}

func cmdJSONDEL(ctx *Context) error {
	if ctx.ArgCount() < 1 {
		return ctx.WriteError(ErrWrongArgCount)
	}

	key := ctx.ArgString(0)
	path := "$"
	if ctx.ArgCount() >= 2 {
		path = ctx.ArgString(1)
	}

	if path == "$" || path == "." || path == "" {
		deleted := ctx.Store.Delete(key)
		if deleted {
			return ctx.WriteInteger(1)
		}
		return ctx.WriteInteger(0)
	}

	entry, exists := ctx.Store.Get(key)
	if !exists {
		return ctx.WriteInteger(0)
	}

	jv, ok := entry.Value.(*store.JSONValue)
	if !ok {
		return ctx.WriteInteger(0)
	}

	if err := jv.DeletePath(path); err != nil {
		return ctx.WriteError(err)
	}

	return ctx.WriteInteger(1)
}

func cmdJSONTYPE(ctx *Context) error {
	if ctx.ArgCount() < 1 {
		return ctx.WriteError(ErrWrongArgCount)
	}

	key := ctx.ArgString(0)
	path := "$"
	if ctx.ArgCount() >= 2 {
		path = ctx.ArgString(1)
	}

	jv, err := getOrCreateJSONValue(ctx.Store, key)
	if err != nil {
		return ctx.WriteError(err)
	}

	typeStr, err := jv.TypeAt(path)
	if err != nil {
		return ctx.WriteError(err)
	}

	return ctx.WriteBulkString(typeStr)
}

func cmdJSONNUMINCRBY(ctx *Context) error {
	if ctx.ArgCount() < 3 {
		return ctx.WriteError(ErrWrongArgCount)
	}

	key := ctx.ArgString(0)
	path := ctx.ArgString(1)
	increment := parseJSONFloat(ctx.ArgString(2))

	entry, exists := ctx.Store.Get(key)
	if !exists {
		return ctx.WriteError(fmt.Errorf("ERR key not found"))
	}

	jv, ok := entry.Value.(*store.JSONValue)
	if !ok {
		return ctx.WriteError(fmt.Errorf("ERR value is not JSON"))
	}

	result, err := jv.NumIncrBy(path, increment)
	if err != nil {
		return ctx.WriteError(err)
	}

	return ctx.WriteBulkString(fmt.Sprintf("%.f", result))
}

func cmdJSONNUMMULTBY(ctx *Context) error {
	if ctx.ArgCount() < 3 {
		return ctx.WriteError(ErrWrongArgCount)
	}

	key := ctx.ArgString(0)
	path := ctx.ArgString(1)
	multiplier := parseJSONFloat(ctx.ArgString(2))

	entry, exists := ctx.Store.Get(key)
	if !exists {
		return ctx.WriteError(fmt.Errorf("ERR key not found"))
	}

	jv, ok := entry.Value.(*store.JSONValue)
	if !ok {
		return ctx.WriteError(fmt.Errorf("ERR value is not JSON"))
	}

	val, err := jv.GetPath(path)
	if err != nil {
		return ctx.WriteError(err)
	}

	if num, ok := val.(float64); ok {
		result := num * multiplier
		jv.SetPath(path, result)
		return ctx.WriteBulkString(fmt.Sprintf("%.f", result))
	}

	return ctx.WriteError(fmt.Errorf("ERR value is not a number"))
}

func cmdJSONSTRAPPEND(ctx *Context) error {
	if ctx.ArgCount() < 3 {
		return ctx.WriteError(ErrWrongArgCount)
	}

	key := ctx.ArgString(0)
	path := ctx.ArgString(1)
	appendStr := ctx.ArgString(2)

	entry, exists := ctx.Store.Get(key)
	if !exists {
		return ctx.WriteError(fmt.Errorf("ERR key not found"))
	}

	jv, ok := entry.Value.(*store.JSONValue)
	if !ok {
		return ctx.WriteError(fmt.Errorf("ERR value is not JSON"))
	}

	val, err := jv.GetPath(path)
	if err != nil {
		return ctx.WriteError(err)
	}

	if str, ok := val.(string); ok {
		newStr := str + appendStr
		jv.SetPath(path, newStr)
		return ctx.WriteInteger(int64(len(newStr)))
	}

	return ctx.WriteError(fmt.Errorf("ERR value is not a string"))
}

func cmdJSONSTRLEN(ctx *Context) error {
	if ctx.ArgCount() < 1 {
		return ctx.WriteError(ErrWrongArgCount)
	}

	key := ctx.ArgString(0)
	path := "$"
	if ctx.ArgCount() >= 2 {
		path = ctx.ArgString(1)
	}

	jv, err := getOrCreateJSONValue(ctx.Store, key)
	if err != nil {
		return ctx.WriteError(err)
	}

	length, err := jv.StrLen(path)
	if err != nil {
		return ctx.WriteError(err)
	}

	return ctx.WriteInteger(int64(length))
}

func cmdJSONARRAPPEND(ctx *Context) error {
	if ctx.ArgCount() < 3 {
		return ctx.WriteError(ErrWrongArgCount)
	}

	key := ctx.ArgString(0)
	path := ctx.ArgString(1)

	values := make([]interface{}, 0, ctx.ArgCount()-2)
	for i := 2; i < ctx.ArgCount(); i++ {
		var v interface{}
		if err := json.Unmarshal(ctx.Arg(i), &v); err != nil {
			values = append(values, ctx.ArgString(i))
		} else {
			values = append(values, v)
		}
	}

	entry, exists := ctx.Store.Get(key)
	if !exists {
		return ctx.WriteError(fmt.Errorf("ERR key not found"))
	}

	jv, ok := entry.Value.(*store.JSONValue)
	if !ok {
		return ctx.WriteError(fmt.Errorf("ERR value is not JSON"))
	}

	length, err := jv.ArrAppend(path, values)
	if err != nil {
		return ctx.WriteError(err)
	}

	return ctx.WriteInteger(int64(length))
}

func cmdJSONARRLEN(ctx *Context) error {
	if ctx.ArgCount() < 1 {
		return ctx.WriteError(ErrWrongArgCount)
	}

	key := ctx.ArgString(0)
	path := "$"
	if ctx.ArgCount() >= 2 {
		path = ctx.ArgString(1)
	}

	jv, err := getOrCreateJSONValue(ctx.Store, key)
	if err != nil {
		return ctx.WriteError(err)
	}

	length, err := jv.ArrLen(path)
	if err != nil {
		return ctx.WriteError(err)
	}

	return ctx.WriteInteger(int64(length))
}

func cmdJSONOBJLEN(ctx *Context) error {
	if ctx.ArgCount() < 1 {
		return ctx.WriteError(ErrWrongArgCount)
	}

	key := ctx.ArgString(0)
	path := "$"
	if ctx.ArgCount() >= 2 {
		path = ctx.ArgString(1)
	}

	jv, err := getOrCreateJSONValue(ctx.Store, key)
	if err != nil {
		return ctx.WriteError(err)
	}

	length, err := jv.ObjLen(path)
	if err != nil {
		return ctx.WriteError(err)
	}

	return ctx.WriteInteger(int64(length))
}

func cmdJSONOBJKEYS(ctx *Context) error {
	if ctx.ArgCount() < 1 {
		return ctx.WriteError(ErrWrongArgCount)
	}

	key := ctx.ArgString(0)
	path := "$"
	if ctx.ArgCount() >= 2 {
		path = ctx.ArgString(1)
	}

	jv, err := getOrCreateJSONValue(ctx.Store, key)
	if err != nil {
		return ctx.WriteError(err)
	}

	val, err := jv.GetPath(path)
	if err != nil {
		return ctx.WriteError(err)
	}

	if obj, ok := val.(map[string]interface{}); ok {
		keys := make([]*resp.Value, 0, len(obj))
		for k := range obj {
			keys = append(keys, resp.BulkString(k))
		}
		return ctx.WriteArray(keys)
	}

	return ctx.WriteNull()
}

func cmdJSONMGET(ctx *Context) error {
	if ctx.ArgCount() < 2 {
		return ctx.WriteError(ErrWrongArgCount)
	}

	path := ctx.ArgString(0)
	keys := make([]string, ctx.ArgCount()-1)
	for i := 1; i < ctx.ArgCount(); i++ {
		keys[i-1] = ctx.ArgString(i)
	}

	results := make([]*resp.Value, 0, len(keys))
	for _, key := range keys {
		jv, err := getOrCreateJSONValue(ctx.Store, key)
		if err != nil {
			results = append(results, resp.NullBulkString())
			continue
		}

		val, err := jv.GetPath(path)
		if err != nil || val == nil {
			results = append(results, resp.NullBulkString())
			continue
		}

		b, err := json.Marshal(val)
		if err != nil {
			results = append(results, resp.NullBulkString())
			continue
		}

		results = append(results, resp.BulkBytes(b))
	}

	return ctx.WriteArray(results)
}

func cmdJSONMSET(ctx *Context) error {
	if ctx.ArgCount() < 3 || ctx.ArgCount()%3 != 0 {
		return ctx.WriteError(ErrWrongArgCount)
	}

	for i := 0; i < ctx.ArgCount(); i += 3 {
		key := ctx.ArgString(i)
		path := ctx.ArgString(i + 1)
		jsonStr := ctx.ArgString(i + 2)

		var value interface{}
		if err := json.Unmarshal([]byte(jsonStr), &value); err != nil {
			return ctx.WriteError(fmt.Errorf("ERR invalid JSON: %v", err))
		}

		jv, err := getOrCreateJSONValue(ctx.Store, key)
		if err != nil {
			return ctx.WriteError(err)
		}

		if err := jv.SetPath(path, value); err != nil {
			return ctx.WriteError(err)
		}

		ctx.Store.Set(key, jv, store.SetOptions{})
	}

	return ctx.WriteOK()
}

func parseJSONFloat(s string) float64 {
	var result float64
	var sign float64 = 1
	i := 0

	if len(s) > 0 && s[0] == '-' {
		sign = -1
		i = 1
	}

	for i < len(s) && s[i] >= '0' && s[i] <= '9' {
		result = result*10 + float64(s[i]-'0')
		i++
	}

	if i < len(s) && s[i] == '.' {
		i++
		decimal := 0.1
		for i < len(s) && s[i] >= '0' && s[i] <= '9' {
			result += float64(s[i]-'0') * decimal
			decimal *= 0.1
			i++
		}
	}

	return result * sign
}

func init() {
	_ = strings.ToLower("")
}
