package command

import (
	"errors"
	"strconv"
	"strings"

	"github.com/cachestorm/cachestorm/internal/resp"
	"github.com/cachestorm/cachestorm/internal/store"
)

// The JSON commands follow RedisJSON, the module built into Redis 8: a path
// starting with $ is JSONPath and answers per match, any other path is the
// legacy syntax and answers for its first match. Replies and error texts
// were checked against Redis 8.2; some of RedisJSON's carry no ERR prefix.

//nolint:staticcheck // RedisJSON's reply texts
var (
	errJSONWrongKeyType   = errors.New("Existing key has wrong Redis type")
	errJSONNoKey          = errors.New("ERR could not perform this operation on a key that doesn't exist")
	errJSONRootOnly       = errors.New("ERR new objects must be created at the root")
	errJSONStaticPath     = errors.New("Err wrong static path")
	errJSONPathMissing    = errors.New("ERR Path does not exist")
	errJSONNotNumber      = errors.New("ERR Path does not exist or does not contains a number")
	errJSONNotString      = errors.New("ERR Path does not exist or not a string")
	errJSONNotArray       = errors.New("ERR Path does not exist or not an array")
	errJSONNotObject      = errors.New("ERR Path does not exist or not an object")
	errJSONNotBool        = errors.New("ERR Path does not exist or not a bool")
	errJSONExpectString   = errors.New("WRONGTYPE wrong type of path value - expected string")
	errJSONExpectArray    = errors.New("WRONGTYPE wrong type of path value - expected array")
	errJSONExpectObject   = errors.New("WRONGTYPE wrong type of path value - expected object")
	errJSONBadNumber      = errors.New("bad input number")
	errJSONNotFinite      = errors.New("result is not a number")
	errJSONIndexBounds    = errors.New("ERR index out of bounds")
	errJSONParseInteger   = errors.New("Couldn't parse as integer")
	errJSONDebugSubcomand = errors.New("ERR unknown subcommand - try `JSON.DEBUG HELP`")
)

func RegisterJSONCommands(router *Router) {
	for name, handler := range map[string]func(*Context) error{
		"JSON.GET": cmdJSONGET, "JSON.SET": cmdJSONSET, "JSON.MGET": cmdJSONMGET, "JSON.MSET": cmdJSONMSET,
		"JSON.MERGE": cmdJSONMERGE, "JSON.DEL": cmdJSONDEL, "JSON.FORGET": cmdJSONDEL, "JSON.TYPE": cmdJSONTYPE,
		"JSON.NUMINCRBY": cmdJSONNUMINCRBY, "JSON.NUMMULTBY": cmdJSONNUMMULTBY,
		"JSON.STRAPPEND": cmdJSONSTRAPPEND, "JSON.STRLEN": cmdJSONSTRLEN,
		"JSON.ARRAPPEND": cmdJSONARRAPPEND, "JSON.ARRINDEX": cmdJSONARRINDEX, "JSON.ARRINSERT": cmdJSONARRINSERT,
		"JSON.ARRLEN": cmdJSONARRLEN, "JSON.ARRPOP": cmdJSONARRPOP, "JSON.ARRTRIM": cmdJSONARRTRIM,
		"JSON.OBJKEYS": cmdJSONOBJKEYS, "JSON.OBJLEN": cmdJSONOBJLEN, "JSON.TOGGLE": cmdJSONTOGGLE,
		"JSON.CLEAR": cmdJSONCLEAR, "JSON.RESP": cmdJSONRESP, "JSON.DEBUG": cmdJSONDEBUG,
	} {
		router.Register(&CommandDef{Name: name, Handler: handler})
	}
}

// getJSON returns the document at key, nil when the key is absent.
func getJSON(ctx *Context, key string) (*store.JSONValue, error) {
	entry, exists := ctx.Store.Get(key)
	if !exists {
		return nil, nil
	}
	jv, ok := entry.Value.(*store.JSONValue)
	if !ok {
		return nil, errJSONWrongKeyType
	}
	return jv, nil
}

func parseJSONPathArg(ctx *Context, i int) (*store.JSONPath, error) {
	if i >= ctx.ArgCount() {
		return store.RootJSONPath(), nil
	}
	return store.ParseJSONPath(ctx.ArgString(i))
}

// parseJSONArg parses a JSON argument; prefixed errors carry ERR, as some
// RedisJSON commands report them.
func parseJSONArg(b []byte, prefixed bool) (*store.JNode, error) {
	n, err := store.ParseJSON(string(b))
	if err != nil && prefixed {
		return nil, errors.New("ERR " + err.Error())
	}
	return n, err
}

func jsonBulk(n *store.JNode) *resp.Value {
	return resp.BulkBytes(store.AppendJSON(nil, n))
}

func jsonArrayBulk(nodes []*store.JNode) *resp.Value {
	out := []byte{'['}
	for i, n := range nodes {
		if i > 0 {
			out = append(out, ',')
		}
		if n == nil {
			out = append(out, "null"...)
		} else {
			out = store.AppendJSON(out, n)
		}
	}
	return resp.BulkBytes(append(out, ']'))
}

// replaceMatch puts v where m sits; it returns the new document root.
func replaceMatch(root *store.JNode, m store.JSONMatch, v *store.JNode) *store.JNode {
	switch {
	case m.Parent == nil:
		return v
	case m.Parent.Kind == store.JArray:
		m.Parent.Arr[m.Index] = v
	default:
		m.Parent.Put(m.Key, v)
	}
	return root
}

// jsonWrite runs fn on the document at key under its write lock and, when
// fn reports a change, accounts for it.
func jsonWrite(ctx *Context, key string, jv *store.JSONValue, fn func() bool) {
	jv.Lock()
	changed := fn()
	jv.Unlock()
	if changed {
		valueWritten(ctx, key)
	}
}

// perMatch answers a command that works on each match: an array of
// replies for a JSONPath, the first match's reply (or legacyErr when no
// match qualifies) for a legacy path.
func perMatch(ctx *Context, path *store.JSONPath, matches []store.JSONMatch, fn func(m store.JSONMatch) *resp.Value, legacyErr error) error {
	replies := make([]*resp.Value, 0, len(matches))
	for _, m := range matches {
		replies = append(replies, fn(m))
	}
	if !path.Legacy {
		return ctx.WriteArray(replies)
	}
	var last *resp.Value
	for _, r := range replies {
		if !r.IsNull {
			last = r
		}
	}
	if last == nil {
		return ctx.WriteError(legacyErr)
	}
	return ctx.WriteValue(last)
}

func cmdJSONSET(ctx *Context) error {
	if ctx.ArgCount() != 3 && ctx.ArgCount() != 4 {
		return ctx.WriteError(arityError(ctx.Command))
	}
	nx, xx := false, false
	if ctx.ArgCount() == 4 {
		switch strings.ToUpper(ctx.ArgString(3)) {
		case "NX":
			nx = true
		case "XX":
			xx = true
		default:
			return ctx.WriteError(ErrSyntaxError)
		}
	}
	key := ctx.ArgString(0)
	path, err := store.ParseJSONPath(ctx.ArgString(1))
	if err != nil {
		return ctx.WriteError(err)
	}
	value, err := parseJSONArg(ctx.Arg(2), false)
	if err != nil {
		return ctx.WriteError(err)
	}
	if path.Legacy && !path.Static() {
		return ctx.WriteError(errJSONStaticPath)
	}
	jv, err := getJSON(ctx, key)
	if err != nil {
		return ctx.WriteError(err)
	}
	if jv == nil {
		if xx {
			return ctx.WriteNullBulkString()
		}
		if err := jsonRootOnly(path); err != nil {
			return ctx.WriteError(err)
		}
		if err := ctx.Store.Set(key, &store.JSONValue{Root: value}, store.SetOptions{}); err != nil {
			return ctx.WriteError(err)
		}
		return ctx.WriteOK()
	}
	var reply *resp.Value
	jsonWrite(ctx, key, jv, func() bool {
		targets, err := path.SetTargets(jv.Root)
		if err != nil {
			reply = resp.ErrorValue(err.Error())
			return false
		}
		if len(targets) == 0 {
			reply = resp.NullBulkString()
			return false
		}
		exists := targets[0].Node != nil || targets[0].Parent == nil
		if (nx && exists) || (xx && !exists) {
			reply = resp.NullBulkString()
			return false
		}
		for i, m := range targets {
			v := value
			if i > 0 {
				v = value.Clone()
			}
			jv.Root = replaceMatch(jv.Root, m, v)
		}
		reply = resp.SimpleString("OK")
		return true
	})
	return ctx.WriteValue(reply)
}

// jsonRootOnly refuses to create a document anywhere but at the root.
func jsonRootOnly(path *store.JSONPath) error {
	if path.IsRoot() {
		return nil
	}
	if path.Legacy && !path.Static() {
		return errJSONStaticPath
	}
	return errJSONRootOnly
}

// parseJSONGetOptions reads JSON.GET's formatting options; it returns the
// format (nil for compact output) and the index of the first path.
func parseJSONGetOptions(ctx *Context, from int) (*store.JSONFormat, int) {
	var format *store.JSONFormat
	i := from
	for ; i < ctx.ArgCount(); i++ {
		word := strings.ToUpper(ctx.ArgString(i))
		if word == "NOESCAPE" {
			continue
		}
		if word != "INDENT" && word != "NEWLINE" && word != "SPACE" || i+1 >= ctx.ArgCount() {
			break
		}
		if format == nil {
			format = &store.JSONFormat{}
		}
		i++
		switch word {
		case "INDENT":
			format.Indent = ctx.ArgString(i)
		case "NEWLINE":
			format.Newline = ctx.ArgString(i)
		case "SPACE":
			format.Space = ctx.ArgString(i)
		}
	}
	return format, i
}

func appendJSONOut(dst []byte, n *store.JNode, format *store.JSONFormat) []byte {
	if format == nil {
		return store.AppendJSON(dst, n)
	}
	return store.AppendJSONFormatted(dst, n, format)
}

// jsonGet renders the paths of JSON.GET or JSON.MGET for root; a legacy
// path that selects nothing is errJSONPathMissing.
func jsonGet(root *store.JNode, paths []*store.JSONPath, format *store.JSONFormat) ([]byte, error) {
	matchesOf := func(p *store.JSONPath) *store.JNode {
		arr := &store.JNode{Kind: store.JArray, Arr: []*store.JNode{}}
		for _, m := range p.Eval(root) {
			arr.Arr = append(arr.Arr, m.Node)
		}
		return arr
	}
	if len(paths) == 1 {
		p := paths[0]
		if !p.Legacy {
			return appendJSONOut(nil, matchesOf(p), format), nil
		}
		matches := p.Eval(root)
		if len(matches) == 0 {
			return nil, errJSONPathMissing
		}
		return appendJSONOut(nil, matches[0].Node, format), nil
	}
	legacy := true
	for _, p := range paths {
		legacy = legacy && p.Legacy
	}
	obj := &store.JNode{Kind: store.JObject}
	for _, p := range paths {
		if !legacy {
			obj.Put(p.Text, matchesOf(p))
			continue
		}
		matches := p.Eval(root)
		if len(matches) == 0 {
			return nil, errJSONPathMissing
		}
		obj.Put(p.Text, matches[0].Node)
	}
	return appendJSONOut(nil, obj, format), nil
}

func parseJSONPaths(ctx *Context, from, to int) ([]*store.JSONPath, error) {
	var paths []*store.JSONPath
	for i := from; i < to; i++ {
		p, err := store.ParseJSONPath(ctx.ArgString(i))
		if err != nil {
			return nil, err
		}
		paths = append(paths, p)
	}
	if len(paths) == 0 {
		paths = append(paths, store.RootJSONPath())
	}
	return paths, nil
}

func cmdJSONGET(ctx *Context) error {
	if ctx.ArgCount() < 1 {
		return ctx.WriteError(arityError(ctx.Command))
	}
	format, first := parseJSONGetOptions(ctx, 1)
	paths, err := parseJSONPaths(ctx, first, ctx.ArgCount())
	if err != nil {
		return ctx.WriteError(err)
	}
	jv, err := getJSON(ctx, ctx.ArgString(0))
	if err != nil {
		return ctx.WriteError(err)
	}
	if jv == nil {
		return ctx.WriteNullBulkString()
	}
	jv.RLock()
	out, err := jsonGet(jv.Root, paths, format)
	jv.RUnlock()
	if err != nil {
		return ctx.WriteError(err)
	}
	return ctx.WriteBulkBytes(out)
}

func cmdJSONMGET(ctx *Context) error {
	if ctx.ArgCount() < 2 {
		return ctx.WriteError(arityError(ctx.Command))
	}
	path, err := store.ParseJSONPath(ctx.ArgString(ctx.ArgCount() - 1))
	if err != nil {
		return ctx.WriteError(err)
	}
	replies := make([]*resp.Value, ctx.ArgCount()-1)
	for i := range replies {
		replies[i] = resp.NullBulkString()
		jv, err := getJSON(ctx, ctx.ArgString(i))
		if err != nil || jv == nil {
			continue
		}
		jv.RLock()
		out, err := jsonGet(jv.Root, []*store.JSONPath{path}, nil)
		jv.RUnlock()
		if err == nil {
			replies[i] = resp.BulkBytes(out)
		}
	}
	return ctx.WriteArray(replies)
}

func cmdJSONMSET(ctx *Context) error {
	if ctx.ArgCount() < 3 || ctx.ArgCount()%3 != 0 {
		return ctx.WriteError(arityError(ctx.Command))
	}
	type triple struct {
		key   string
		path  *store.JSONPath
		value *store.JNode
		jv    *store.JSONValue
	}
	var triples []triple
	for i := 0; i < ctx.ArgCount(); i += 3 {
		path, err := store.ParseJSONPath(ctx.ArgString(i + 1))
		if err != nil {
			return ctx.WriteError(err)
		}
		value, err := parseJSONArg(ctx.Arg(i+2), false)
		if err != nil {
			return ctx.WriteError(err)
		}
		jv, err := getJSON(ctx, ctx.ArgString(i))
		if err != nil {
			return ctx.WriteError(err)
		}
		if jv == nil {
			if err := jsonRootOnly(path); err != nil {
				return ctx.WriteError(err)
			}
		}
		triples = append(triples, triple{ctx.ArgString(i), path, value, jv})
	}
	for _, t := range triples {
		jv, err := getJSON(ctx, t.key)
		if err != nil {
			return ctx.WriteError(err)
		}
		if jv == nil {
			if err := ctx.Store.Set(t.key, &store.JSONValue{Root: t.value}, store.SetOptions{}); err != nil {
				return ctx.WriteError(err)
			}
			continue
		}
		jsonWrite(ctx, t.key, jv, func() bool {
			targets, err := t.path.SetTargets(jv.Root)
			if err != nil || len(targets) == 0 {
				return false
			}
			for i, m := range targets {
				v := t.value
				if i > 0 {
					v = t.value.Clone()
				}
				jv.Root = replaceMatch(jv.Root, m, v)
			}
			return true
		})
	}
	return ctx.WriteOK()
}

// mergePatch applies an RFC 7386 merge patch to target.
func mergePatch(target, patch *store.JNode) *store.JNode {
	if patch.Kind != store.JObject {
		return patch.Clone()
	}
	if target == nil || target.Kind != store.JObject {
		target = &store.JNode{Kind: store.JObject, Keys: []string{}, Vals: []*store.JNode{}}
	}
	for i, k := range patch.Keys {
		v := patch.Vals[i]
		if v.Kind == store.JNull {
			target.Remove(k)
			continue
		}
		target.Put(k, mergePatch(target.Get(k), v))
	}
	return target
}

func cmdJSONMERGE(ctx *Context) error {
	if ctx.ArgCount() != 3 {
		return ctx.WriteError(arityError(ctx.Command))
	}
	key := ctx.ArgString(0)
	path, err := store.ParseJSONPath(ctx.ArgString(1))
	if err != nil {
		return ctx.WriteError(err)
	}
	patch, err := parseJSONArg(ctx.Arg(2), false)
	if err != nil {
		return ctx.WriteError(err)
	}
	jv, err := getJSON(ctx, key)
	if err != nil {
		return ctx.WriteError(err)
	}
	if jv == nil {
		if err := jsonRootOnly(path); err != nil {
			return ctx.WriteError(err)
		}
		if err := ctx.Store.Set(key, &store.JSONValue{Root: mergePatch(nil, patch)}, store.SetOptions{}); err != nil {
			return ctx.WriteError(err)
		}
		return ctx.WriteOK()
	}
	var reply *resp.Value
	jsonWrite(ctx, key, jv, func() bool {
		targets, err := path.SetTargets(jv.Root)
		if err != nil || len(targets) == 0 {
			reply = resp.NullBulkString()
			return false
		}
		for _, m := range targets {
			jv.Root = replaceMatch(jv.Root, m, mergePatch(m.Node, patch))
		}
		reply = resp.SimpleString("OK")
		return true
	})
	return ctx.WriteValue(reply)
}

func cmdJSONDEL(ctx *Context) error {
	if ctx.ArgCount() < 1 || ctx.ArgCount() > 2 {
		return ctx.WriteError(arityError(ctx.Command))
	}
	key := ctx.ArgString(0)
	path, err := parseJSONPathArg(ctx, 1)
	if err != nil {
		return ctx.WriteError(err)
	}
	jv, err := getJSON(ctx, key)
	if err != nil {
		return ctx.WriteError(err)
	}
	if jv == nil {
		return ctx.WriteInteger(0)
	}
	if path.IsRoot() {
		ctx.Store.Delete(key)
		return ctx.WriteInteger(1)
	}
	deleted := 0
	jsonWrite(ctx, key, jv, func() bool {
		matches := path.Eval(jv.Root)
		// Array elements go by identity, so removing one does not shift
		// the positions of the others; object keys go in match order.
		doomed := make(map[*store.JNode]bool)
		for _, m := range matches {
			if m.Parent.Kind == store.JArray {
				doomed[m.Node] = true
			} else if m.Parent.Remove(m.Key) {
				deleted++
			}
		}
		for _, m := range matches {
			if m.Parent.Kind != store.JArray || !doomed[m.Node] {
				continue
			}
			kept := m.Parent.Arr[:0]
			for _, c := range m.Parent.Arr {
				if doomed[c] {
					deleted++
					delete(doomed, c)
					continue
				}
				kept = append(kept, c)
			}
			m.Parent.Arr = kept
		}
		return deleted > 0
	})
	return ctx.WriteInteger(int64(deleted))
}

func cmdJSONTYPE(ctx *Context) error {
	if ctx.ArgCount() < 1 || ctx.ArgCount() > 2 {
		return ctx.WriteError(arityError(ctx.Command))
	}
	path, err := parseJSONPathArg(ctx, 1)
	if err != nil {
		return ctx.WriteError(err)
	}
	jv, err := getJSON(ctx, ctx.ArgString(0))
	if err != nil {
		return ctx.WriteError(err)
	}
	if jv == nil {
		return ctx.WriteNullBulkString()
	}
	jv.RLock()
	defer jv.RUnlock()
	matches := path.Eval(jv.Root)
	if path.Legacy {
		if len(matches) == 0 {
			return ctx.WriteNullBulkString()
		}
		return ctx.WriteBulkString(matches[0].Node.TypeName())
	}
	replies := make([]*resp.Value, len(matches))
	for i, m := range matches {
		replies[i] = resp.BulkString(m.Node.TypeName())
	}
	return ctx.WriteArray(replies)
}

// jsonWriteTargets loads the document a write command on (key, path)
// changes; a missing key is errJSONNoKey.
func jsonWriteTargets(ctx *Context, pathArg int) (*store.JSONValue, *store.JSONPath, error) {
	path, err := parseJSONPathArg(ctx, pathArg)
	if err != nil {
		return nil, nil, err
	}
	jv, err := getJSON(ctx, ctx.ArgString(0))
	if err != nil {
		return nil, nil, err
	}
	if jv == nil {
		return nil, nil, errJSONNoKey
	}
	return jv, path, nil
}

func jsonNumOp(ctx *Context, mult bool) error {
	if ctx.ArgCount() != 3 {
		return ctx.WriteError(arityError(ctx.Command))
	}
	by, err := parseJSONArg(ctx.Arg(2), true)
	if err != nil {
		return ctx.WriteError(err)
	}
	if !by.IsNumber() {
		return ctx.WriteError(errJSONBadNumber)
	}
	key := ctx.ArgString(0)
	jv, path, err := jsonWriteTargets(ctx, 1)
	if err != nil {
		return ctx.WriteError(err)
	}
	var results []*store.JNode
	var opErr error
	jsonWrite(ctx, key, jv, func() bool {
		changed := false
		for _, m := range path.Eval(jv.Root) {
			if !m.Node.IsNumber() {
				results = append(results, nil)
				continue
			}
			n, err := store.JSONArith(m.Node, by, mult)
			if err != nil {
				opErr = errJSONNotFinite
				return changed
			}
			jv.Root = replaceMatch(jv.Root, m, n)
			results = append(results, n)
			changed = true
		}
		return changed
	})
	if opErr != nil {
		return ctx.WriteError(opErr)
	}
	if !path.Legacy {
		return ctx.WriteValue(jsonArrayBulk(results))
	}
	for i := len(results) - 1; i >= 0; i-- {
		if results[i] != nil {
			return ctx.WriteValue(jsonBulk(results[i]))
		}
	}
	return ctx.WriteError(errJSONNotNumber)
}

func cmdJSONNUMINCRBY(ctx *Context) error { return jsonNumOp(ctx, false) }
func cmdJSONNUMMULTBY(ctx *Context) error { return jsonNumOp(ctx, true) }

func cmdJSONSTRAPPEND(ctx *Context) error {
	if ctx.ArgCount() != 2 && ctx.ArgCount() != 3 {
		return ctx.WriteError(arityError(ctx.Command))
	}
	pathArg, valueArg := 1, 2
	if ctx.ArgCount() == 2 {
		pathArg, valueArg = 2, 1
	}
	suffix, err := parseJSONArg(ctx.Arg(valueArg), true)
	if err != nil {
		return ctx.WriteError(err)
	}
	if suffix.Kind != store.JString {
		return ctx.WriteError(errJSONNotString)
	}
	return jsonModify(ctx, pathArg, errJSONNotString, func(_ *store.JSONValue, m store.JSONMatch) *resp.Value {
		if m.Node.Kind != store.JString {
			return nil
		}
		m.Node.Str += suffix.Str
		return resp.IntegerValue(int64(len(m.Node.Str)))
	})
}

// jsonRead runs a read-only per-match command: missing keys reply nil, a
// legacy path that selects nothing replies missingLegacy (nil when it is
// nil), and fn answers each match (nil for a match of the wrong kind,
// which a legacy path reports as wrongLegacy).
func jsonRead(ctx *Context, pathArg int, missingLegacy, wrongLegacy error, fn func(n *store.JNode) *resp.Value) error {
	path, err := parseJSONPathArg(ctx, pathArg)
	if err != nil {
		return ctx.WriteError(err)
	}
	jv, err := getJSON(ctx, ctx.ArgString(0))
	if err != nil {
		return ctx.WriteError(err)
	}
	if jv == nil {
		return ctx.WriteNullBulkString()
	}
	jv.RLock()
	defer jv.RUnlock()
	matches := path.Eval(jv.Root)
	if path.Legacy {
		if len(matches) == 0 {
			if missingLegacy == nil {
				return ctx.WriteNullBulkString()
			}
			return ctx.WriteError(missingLegacy)
		}
		reply := fn(matches[0].Node)
		if reply == nil {
			return ctx.WriteError(wrongLegacy)
		}
		return ctx.WriteValue(reply)
	}
	replies := make([]*resp.Value, len(matches))
	for i, m := range matches {
		if replies[i] = fn(m.Node); replies[i] == nil {
			replies[i] = resp.NullBulkString()
		}
	}
	return ctx.WriteArray(replies)
}

func cmdJSONSTRLEN(ctx *Context) error {
	if ctx.ArgCount() < 1 || ctx.ArgCount() > 2 {
		return ctx.WriteError(arityError(ctx.Command))
	}
	return jsonRead(ctx, 1, errJSONPathMissing, errJSONExpectString, func(n *store.JNode) *resp.Value {
		if n.Kind != store.JString {
			return nil
		}
		return resp.IntegerValue(int64(len(n.Str)))
	})
}

func cmdJSONARRLEN(ctx *Context) error {
	if ctx.ArgCount() < 1 || ctx.ArgCount() > 2 {
		return ctx.WriteError(arityError(ctx.Command))
	}
	return jsonRead(ctx, 1, errJSONPathMissing, errJSONNotArray, func(n *store.JNode) *resp.Value {
		if n.Kind != store.JArray {
			return nil
		}
		return resp.IntegerValue(int64(len(n.Arr)))
	})
}

func cmdJSONOBJLEN(ctx *Context) error {
	if ctx.ArgCount() < 1 || ctx.ArgCount() > 2 {
		return ctx.WriteError(arityError(ctx.Command))
	}
	return jsonRead(ctx, 1, nil, errJSONExpectObject, func(n *store.JNode) *resp.Value {
		if n.Kind != store.JObject {
			return nil
		}
		return resp.IntegerValue(int64(len(n.Keys)))
	})
}

func cmdJSONOBJKEYS(ctx *Context) error {
	if ctx.ArgCount() < 1 || ctx.ArgCount() > 2 {
		return ctx.WriteError(arityError(ctx.Command))
	}
	return jsonRead(ctx, 1, nil, errJSONNotObject, func(n *store.JNode) *resp.Value {
		if n.Kind != store.JObject {
			return nil
		}
		keys := make([]*resp.Value, len(n.Keys))
		for i, k := range n.Keys {
			keys[i] = resp.BulkString(k)
		}
		return resp.ArrayValue(keys)
	})
}

// jsonResp is JSON.RESP's form of n.
func jsonResp(n *store.JNode) *resp.Value {
	switch n.Kind {
	case store.JNull:
		return resp.NullBulkString()
	case store.JBool:
		return resp.SimpleString(strconv.FormatBool(n.Bool))
	case store.JInt:
		return resp.IntegerValue(n.Int)
	case store.JUint, store.JFloat:
		return jsonBulk(n)
	case store.JString:
		return resp.BulkString(n.Str)
	case store.JArray:
		out := []*resp.Value{resp.SimpleString("[")}
		for _, v := range n.Arr {
			out = append(out, jsonResp(v))
		}
		return resp.ArrayValue(out)
	default:
		out := []*resp.Value{resp.SimpleString("{")}
		for i, k := range n.Keys {
			out = append(out, resp.BulkString(k), jsonResp(n.Vals[i]))
		}
		return resp.ArrayValue(out)
	}
}

func cmdJSONRESP(ctx *Context) error {
	if ctx.ArgCount() < 1 || ctx.ArgCount() > 2 {
		return ctx.WriteError(arityError(ctx.Command))
	}
	return jsonRead(ctx, 1, errJSONPathMissing, errJSONPathMissing, jsonResp)
}

func cmdJSONDEBUG(ctx *Context) error {
	if ctx.ArgCount() < 1 {
		return ctx.WriteError(arityError(ctx.Command))
	}
	switch strings.ToUpper(ctx.ArgString(0)) {
	case "HELP":
		return ctx.WriteArray([]*resp.Value{
			resp.BulkString("MEMORY <key> [path] - reports memory usage"),
			resp.BulkString("HELP                - this message"),
		})
	case "MEMORY":
		if ctx.ArgCount() < 2 || ctx.ArgCount() > 3 {
			return ctx.WriteError(arityError(ctx.Command))
		}
		sub := NewContext(ctx.Command, ctx.Args[1:], ctx.Store, ctx.Writer)
		if jv, err := getJSON(sub, sub.ArgString(0)); err == nil && jv == nil {
			return ctx.WriteInteger(0)
		}
		return jsonRead(sub, 1, errJSONPathMissing, errJSONPathMissing, func(n *store.JNode) *resp.Value {
			return resp.IntegerValue(n.MemorySize())
		})
	}
	return ctx.WriteError(errJSONDebugSubcomand)
}

// jsonModify runs a per-match write command: fn changes one match and
// replies for it, nil for a match of the wrong kind.
func jsonModify(ctx *Context, pathArg int, legacyErr error, fn func(jv *store.JSONValue, m store.JSONMatch) *resp.Value) error {
	key := ctx.ArgString(0)
	jv, path, err := jsonWriteTargets(ctx, pathArg)
	if err != nil {
		return ctx.WriteError(err)
	}
	var matches []store.JSONMatch
	var replies []*resp.Value
	jsonWrite(ctx, key, jv, func() bool {
		matches = path.Eval(jv.Root)
		changed := false
		for _, m := range matches {
			r := fn(jv, m)
			if r == nil {
				r = resp.NullBulkString()
			} else {
				changed = true
			}
			replies = append(replies, r)
		}
		return changed
	})
	i := 0
	return perMatch(ctx, path, matches, func(store.JSONMatch) *resp.Value { i++; return replies[i-1] }, legacyErr)
}

func parseJSONValues(ctx *Context, from int, prefixed bool) ([]*store.JNode, error) {
	values := make([]*store.JNode, 0, ctx.ArgCount()-from)
	for i := from; i < ctx.ArgCount(); i++ {
		v, err := parseJSONArg(ctx.Arg(i), prefixed)
		if err != nil {
			return nil, err
		}
		values = append(values, v)
	}
	return values, nil
}

func cmdJSONARRAPPEND(ctx *Context) error {
	if ctx.ArgCount() < 3 {
		return ctx.WriteError(arityError(ctx.Command))
	}
	values, err := parseJSONValues(ctx, 2, false)
	if err != nil {
		return ctx.WriteError(err)
	}
	return jsonModify(ctx, 1, errJSONNotArray, func(_ *store.JSONValue, m store.JSONMatch) *resp.Value {
		if m.Node.Kind != store.JArray {
			return nil
		}
		for _, v := range values {
			m.Node.Arr = append(m.Node.Arr, v.Clone())
		}
		return resp.IntegerValue(int64(len(m.Node.Arr)))
	})
}

func cmdJSONARRINSERT(ctx *Context) error {
	if ctx.ArgCount() < 4 {
		return ctx.WriteError(arityError(ctx.Command))
	}
	index, err := strconv.Atoi(ctx.ArgString(2))
	if err != nil {
		return ctx.WriteError(errJSONParseInteger)
	}
	values, err := parseJSONValues(ctx, 3, false)
	if err != nil {
		return ctx.WriteError(err)
	}
	position := func(n *store.JNode) (int, bool) {
		at := index
		if at < 0 {
			at += len(n.Arr)
		}
		return at, at >= 0 && at <= len(n.Arr)
	}
	jv, err := getJSON(ctx, ctx.ArgString(0))
	if err == nil && jv != nil {
		path, perr := parseJSONPathArg(ctx, 1)
		if perr == nil {
			jv.RLock()
			for _, m := range path.Eval(jv.Root) {
				if _, ok := position(m.Node); m.Node.Kind == store.JArray && !ok {
					jv.RUnlock()
					return ctx.WriteError(errJSONIndexBounds)
				}
			}
			jv.RUnlock()
		}
	}
	return jsonModify(ctx, 1, errJSONNotArray, func(_ *store.JSONValue, m store.JSONMatch) *resp.Value {
		if m.Node.Kind != store.JArray {
			return nil
		}
		at, ok := position(m.Node)
		if !ok {
			return nil
		}
		inserted := make([]*store.JNode, 0, len(m.Node.Arr)+len(values))
		inserted = append(inserted, m.Node.Arr[:at]...)
		for _, v := range values {
			inserted = append(inserted, v.Clone())
		}
		m.Node.Arr = append(inserted, m.Node.Arr[at:]...)
		return resp.IntegerValue(int64(len(m.Node.Arr)))
	})
}

func cmdJSONARRINDEX(ctx *Context) error {
	if ctx.ArgCount() < 3 || ctx.ArgCount() > 5 {
		return ctx.WriteError(arityError(ctx.Command))
	}
	value, err := parseJSONArg(ctx.Arg(2), true)
	if err != nil {
		return ctx.WriteError(err)
	}
	bounds := []int{0, 0}
	for i := 3; i < ctx.ArgCount(); i++ {
		if bounds[i-3], err = strconv.Atoi(ctx.ArgString(i)); err != nil {
			return ctx.WriteError(errJSONParseInteger)
		}
	}
	jv, err := getJSON(ctx, ctx.ArgString(0))
	if err != nil {
		return ctx.WriteError(err)
	}
	if jv == nil {
		return ctx.WriteError(errJSONPathMissing)
	}
	return jsonRead(ctx, 1, errJSONPathMissing, errJSONExpectArray, func(n *store.JNode) *resp.Value {
		if n.Kind != store.JArray {
			return nil
		}
		size := len(n.Arr)
		start, stop := bounds[0], bounds[1]
		if start < 0 {
			start = max(start+size, 0)
		}
		if stop <= 0 {
			stop += size
		}
		stop = min(stop, size)
		for i := start; i < stop; i++ {
			if jsonStrictEqual(n.Arr[i], value) {
				return resp.IntegerValue(int64(i))
			}
		}
		return resp.IntegerValue(-1)
	})
}

// jsonStrictEqual compares as JSON.ARRINDEX does: an integer never equals
// a float.
func jsonStrictEqual(a, b *store.JNode) bool {
	if (a.Kind == store.JFloat) != (b.Kind == store.JFloat) && a.IsNumber() && b.IsNumber() {
		return false
	}
	return a.Equal(b)
}

func cmdJSONARRPOP(ctx *Context) error {
	if ctx.ArgCount() < 1 || ctx.ArgCount() > 3 {
		return ctx.WriteError(arityError(ctx.Command))
	}
	index := -1
	if ctx.ArgCount() == 3 {
		if n, err := strconv.Atoi(ctx.ArgString(2)); err == nil {
			index = n
		}
	}
	key := ctx.ArgString(0)
	jv, path, err := jsonWriteTargets(ctx, 1)
	if err != nil {
		return ctx.WriteError(err)
	}
	var replies []*resp.Value
	var notArray bool
	jsonWrite(ctx, key, jv, func() bool {
		changed := false
		for _, m := range path.Eval(jv.Root) {
			if m.Node.Kind != store.JArray {
				notArray = true
				replies = append(replies, resp.NullBulkString())
				continue
			}
			arr := m.Node.Arr
			if len(arr) == 0 {
				replies = append(replies, resp.NullBulkString())
				continue
			}
			at := index
			if at < 0 {
				at += len(arr)
			}
			at = min(max(at, 0), len(arr)-1)
			replies = append(replies, jsonBulk(arr[at]))
			copy(arr[at:], arr[at+1:])
			m.Node.Arr = arr[:len(arr)-1]
			changed = true
		}
		return changed
	})
	if !path.Legacy {
		return ctx.WriteArray(replies)
	}
	if len(replies) == 0 || notArray && replies[0].IsNull {
		return ctx.WriteError(errJSONNotArray)
	}
	return ctx.WriteValue(replies[0])
}

func cmdJSONARRTRIM(ctx *Context) error {
	if ctx.ArgCount() != 4 {
		return ctx.WriteError(arityError(ctx.Command))
	}
	start, err1 := strconv.Atoi(ctx.ArgString(2))
	stop, err2 := strconv.Atoi(ctx.ArgString(3))
	if err1 != nil || err2 != nil {
		return ctx.WriteError(errJSONParseInteger)
	}
	return jsonModify(ctx, 1, errJSONNotArray, func(_ *store.JSONValue, m store.JSONMatch) *resp.Value {
		if m.Node.Kind != store.JArray {
			return nil
		}
		size := len(m.Node.Arr)
		from, to := start, stop
		if from < 0 {
			from = max(from+size, 0)
		}
		if to < 0 {
			to += size
		}
		to = min(to, size-1)
		if from >= size || from > to {
			m.Node.Arr = []*store.JNode{}
		} else {
			m.Node.Arr = append([]*store.JNode(nil), m.Node.Arr[from:to+1]...)
		}
		return resp.IntegerValue(int64(len(m.Node.Arr)))
	})
}

func cmdJSONTOGGLE(ctx *Context) error {
	if ctx.ArgCount() != 2 {
		return ctx.WriteError(arityError(ctx.Command))
	}
	key := ctx.ArgString(0)
	jv, path, err := jsonWriteTargets(ctx, 1)
	if err != nil {
		return ctx.WriteError(err)
	}
	var replies []*resp.Value
	var last *store.JNode
	jsonWrite(ctx, key, jv, func() bool {
		for _, m := range path.Eval(jv.Root) {
			if m.Node.Kind != store.JBool {
				replies = append(replies, resp.NullBulkString())
				continue
			}
			m.Node.Bool = !m.Node.Bool
			last = m.Node
			replies = append(replies, resp.IntegerValue(boolInt64(m.Node.Bool)))
		}
		return last != nil
	})
	if !path.Legacy {
		return ctx.WriteArray(replies)
	}
	if last == nil {
		return ctx.WriteError(errJSONNotBool)
	}
	return ctx.WriteBulkString(strconv.FormatBool(last.Bool))
}

func cmdJSONCLEAR(ctx *Context) error {
	if ctx.ArgCount() < 1 || ctx.ArgCount() > 2 {
		return ctx.WriteError(arityError(ctx.Command))
	}
	key := ctx.ArgString(0)
	jv, path, err := jsonWriteTargets(ctx, 1)
	if err != nil {
		return ctx.WriteError(err)
	}
	cleared := 0
	jsonWrite(ctx, key, jv, func() bool {
		for _, m := range path.Eval(jv.Root) {
			switch m.Node.Kind {
			case store.JArray:
				m.Node.Arr = []*store.JNode{}
			case store.JObject:
				m.Node.Keys, m.Node.Vals = []string{}, []*store.JNode{}
			case store.JInt, store.JUint, store.JFloat:
				*m.Node = store.JNode{Kind: store.JInt}
			default:
				continue
			}
			cleared++
		}
		return cleared > 0
	})
	return ctx.WriteInteger(int64(cleared))
}
