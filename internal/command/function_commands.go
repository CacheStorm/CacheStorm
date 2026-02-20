package command

import (
	"crypto/sha1"
	"encoding/hex"
	"fmt"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/cachestorm/cachestorm/internal/resp"
	"github.com/cachestorm/cachestorm/internal/store"
	lua "github.com/yuin/gopher-lua"
)

type Function struct {
	Name        string
	Library     string
	Code        string
	SHA         string
	Description string
	Flags       []string
	CreatedAt   time.Time
}

type Library struct {
	Name      string
	Code      string
	SHA       string
	Functions map[string]*Function
	CreatedAt time.Time
	Engine    string
}

type FunctionRegistry struct {
	mu        sync.RWMutex
	libraries map[string]*Library
	functions map[string]*Function
	store     *store.Store
}

var functionRegistry *FunctionRegistry
var functionOnce sync.Once

func GetFunctionRegistry(s *store.Store) *FunctionRegistry {
	functionOnce.Do(func() {
		functionRegistry = &FunctionRegistry{
			libraries: make(map[string]*Library),
			functions: make(map[string]*Function),
			store:     s,
		}
	})
	return functionRegistry
}

func (r *FunctionRegistry) CreateLibrary(name string, code string, replace bool) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	if _, exists := r.libraries[name]; exists && !replace {
		return fmt.Errorf("ERR library '%s' already exists", name)
	}

	sha := sha1.Sum([]byte(code))
	shaStr := hex.EncodeToString(sha[:])

	lib := &Library{
		Name:      name,
		Code:      code,
		SHA:       shaStr,
		Functions: make(map[string]*Function),
		CreatedAt: time.Now(),
		Engine:    "lua",
	}

	L := lua.NewState()
	defer L.Close()

	if err := L.DoString(code); err != nil {
		return fmt.Errorf("ERR failed to load library: %v", err)
	}

	redisTbl := L.GetGlobal("redis")
	if tbl, ok := redisTbl.(*lua.LTable); ok {
		L.ForEach(tbl, func(key, value lua.LValue) {
			if strKey, ok := key.(lua.LString); ok {
				if _, ok := value.(*lua.LFunction); ok {
					fnName := string(strKey)
					if !strings.HasPrefix(fnName, "_") {
						fnSHA := sha1.Sum([]byte(name + ":" + fnName + code))
						function := &Function{
							Name:      fnName,
							Library:   name,
							Code:      fnName,
							SHA:       hex.EncodeToString(fnSHA[:]),
							CreatedAt: time.Now(),
						}
						lib.Functions[fnName] = function
						r.functions[name+"."+fnName] = function
					}
				}
			}
		})
	}

	if oldLib, exists := r.libraries[name]; exists {
		for fnName := range oldLib.Functions {
			delete(r.functions, name+"."+fnName)
		}
	}

	r.libraries[name] = lib
	return nil
}

func (r *FunctionRegistry) DeleteLibrary(name string) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	lib, exists := r.libraries[name]
	if !exists {
		return fmt.Errorf("ERR library '%s' not found", name)
	}

	for fnName := range lib.Functions {
		delete(r.functions, name+"."+fnName)
	}

	delete(r.libraries, name)
	return nil
}

func (r *FunctionRegistry) GetLibrary(name string) (*Library, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	lib, ok := r.libraries[name]
	return lib, ok
}

func (r *FunctionRegistry) ListLibraries(pattern string) []*Library {
	r.mu.RLock()
	defer r.mu.RUnlock()

	result := make([]*Library, 0)
	for name, lib := range r.libraries {
		if pattern == "" || globMatch(name, pattern) {
			result = append(result, lib)
		}
	}
	sort.Slice(result, func(i, j int) bool {
		return result[i].Name < result[j].Name
	})
	return result
}

func (r *FunctionRegistry) CallFunction(libName string, fnName string, keys []string, args []string) (interface{}, error) {
	r.mu.RLock()
	lib, libExists := r.libraries[libName]
	if !libExists {
		r.mu.RUnlock()
		return nil, fmt.Errorf("ERR library '%s' not found", libName)
	}

	_, fnExists := lib.Functions[fnName]
	if !fnExists {
		r.mu.RUnlock()
		return nil, fmt.Errorf("ERR function '%s' not found in library '%s'", fnName, libName)
	}
	code := lib.Code
	r.mu.RUnlock()

	se := NewScriptEngine(r.store)
	L := se.CreateState(keys, args)
	defer L.Close()

	if err := L.DoString(code); err != nil {
		return nil, fmt.Errorf("ERR failed to load library: %v", err)
	}

	redisTbl := L.GetGlobal("redis")
	if tbl, ok := redisTbl.(*lua.LTable); ok {
		fn := L.GetField(tbl, fnName)
		if luaFn, ok := fn.(*lua.LFunction); ok {
			if err := L.CallByParam(lua.P{
				Fn:      luaFn,
				NRet:    1,
				Protect: true,
			}); err != nil {
				return nil, fmt.Errorf("ERR %v", err)
			}
			ret := L.Get(-1)
			L.Pop(1)
			return luaToGo(ret), nil
		}
	}

	return nil, fmt.Errorf("ERR function '%s' not callable", fnName)
}

func (r *FunctionRegistry) GetFunction(libName string, fnName string) (*Function, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	fn, ok := r.functions[libName+"."+fnName]
	return fn, ok
}

func globMatch(s, pattern string) bool {
	if pattern == "*" {
		return true
	}
	if strings.HasPrefix(pattern, "*") && strings.HasSuffix(pattern, "*") {
		middle := pattern[1 : len(pattern)-1]
		return strings.Contains(s, middle)
	}
	if strings.HasPrefix(pattern, "*") {
		return strings.HasSuffix(s, pattern[1:])
	}
	if strings.HasSuffix(pattern, "*") {
		return strings.HasPrefix(s, pattern[:len(pattern)-1])
	}
	return s == pattern
}

func cmdFUNCTION(ctx *Context) error {
	if ctx.ArgCount() < 1 {
		return ctx.WriteError(ErrWrongArgCount)
	}

	subCmd := strings.ToUpper(ctx.ArgString(0))
	registry := GetFunctionRegistry(ctx.Store)

	switch subCmd {
	case "CREATE":
		return handleFunctionCreate(ctx, registry)
	case "DELETE":
		return handleFunctionDelete(ctx, registry)
	case "LIST":
		return handleFunctionList(ctx, registry)
	case "DUMP":
		return handleFunctionDump(ctx, registry)
	case "RESTORE":
		return handleFunctionRestore(ctx, registry)
	case "STATS":
		return handleFunctionStats(ctx, registry)
	case "FLUSH":
		return handleFunctionFlush(ctx, registry)
	default:
		return ctx.WriteError(fmt.Errorf("ERR unknown subcommand '%s'", subCmd))
	}
}

func handleFunctionCreate(ctx *Context, registry *FunctionRegistry) error {
	if ctx.ArgCount() < 3 {
		return ctx.WriteError(ErrWrongArgCount)
	}

	libName := ctx.ArgString(1)
	code := ctx.ArgString(2)
	replace := false

	for i := 3; i < ctx.ArgCount(); i++ {
		arg := strings.ToUpper(ctx.ArgString(i))
		if arg == "REPLACE" {
			replace = true
		}
	}

	if err := registry.CreateLibrary(libName, code, replace); err != nil {
		return ctx.WriteError(err)
	}

	lib, _ := registry.GetLibrary(libName)
	return ctx.WriteBulkString(lib.SHA)
}

func handleFunctionDelete(ctx *Context, registry *FunctionRegistry) error {
	if ctx.ArgCount() < 2 {
		return ctx.WriteError(ErrWrongArgCount)
	}

	libName := ctx.ArgString(1)
	if err := registry.DeleteLibrary(libName); err != nil {
		return ctx.WriteError(err)
	}

	return ctx.WriteOK()
}

func handleFunctionList(ctx *Context, registry *FunctionRegistry) error {
	pattern := "*"
	if ctx.ArgCount() >= 2 {
		pattern = ctx.ArgString(1)
	}

	libraries := registry.ListLibraries(pattern)
	result := make([]*resp.Value, 0, len(libraries))

	for _, lib := range libraries {
		libInfo := []*resp.Value{
			resp.BulkString("library_name"),
			resp.BulkString(lib.Name),
			resp.BulkString("engine"),
			resp.BulkString(lib.Engine),
		}

		fnList := make([]*resp.Value, 0)
		for fnName, fn := range lib.Functions {
			fnInfo := resp.ArrayValue([]*resp.Value{
				resp.BulkString("name"),
				resp.BulkString(fnName),
				resp.BulkString("description"),
				resp.BulkString(fn.Description),
				resp.BulkString("flags"),
				resp.ArrayValue([]*resp.Value{}),
			})
			fnList = append(fnList, fnInfo)
		}

		libInfo = append(libInfo, resp.BulkString("functions"))
		libInfo = append(libInfo, resp.ArrayValue(fnList))

		result = append(result, resp.ArrayValue(libInfo))
	}

	return ctx.WriteArray(result)
}

func handleFunctionDump(ctx *Context, registry *FunctionRegistry) error {
	registry.mu.RLock()
	defer registry.mu.RUnlock()

	var result strings.Builder
	for name, lib := range registry.libraries {
		result.WriteString(fmt.Sprintf("LIBRARY %s ENGINE %s CODE %s\n", name, lib.Engine, lib.SHA))
	}

	return ctx.WriteBulkString(result.String())
}

func handleFunctionRestore(ctx *Context, registry *FunctionRegistry) error {
	if ctx.ArgCount() < 2 {
		return ctx.WriteError(ErrWrongArgCount)
	}

	return ctx.WriteOK()
}

func handleFunctionStats(ctx *Context, registry *FunctionRegistry) error {
	registry.mu.RLock()
	defer registry.mu.RUnlock()

	stats := []*resp.Value{
		resp.ArrayValue([]*resp.Value{
			resp.BulkString("libraries_count"),
			resp.IntegerValue(int64(len(registry.libraries))),
		}),
		resp.ArrayValue([]*resp.Value{
			resp.BulkString("functions_count"),
			resp.IntegerValue(int64(len(registry.functions))),
		}),
	}

	return ctx.WriteArray(stats)
}

func handleFunctionFlush(ctx *Context, registry *FunctionRegistry) error {
	registry.mu.Lock()
	defer registry.mu.Unlock()

	registry.libraries = make(map[string]*Library)
	registry.functions = make(map[string]*Function)

	return ctx.WriteOK()
}

func cmdFCALL(ctx *Context) error {
	if ctx.ArgCount() < 1 {
		return ctx.WriteError(ErrWrongArgCount)
	}

	libAndFn := ctx.ArgString(0)
	parts := strings.SplitN(libAndFn, ".", 2)
	var libName, fnName string
	if len(parts) == 2 {
		libName = parts[0]
		fnName = parts[1]
	} else {
		return ctx.WriteError(fmt.Errorf("ERR invalid function name format, use library.function"))
	}

	numKeys := 0
	offset := 1

	if ctx.ArgCount() >= 2 {
		nk, err := parseFuncInt(ctx.ArgString(1))
		if err == nil {
			numKeys = int(nk)
			offset = 2
		}
	}

	keys := make([]string, 0)
	args := make([]string, 0)

	for i := offset; i < offset+numKeys && i < ctx.ArgCount(); i++ {
		keys = append(keys, ctx.ArgString(i))
	}
	for i := offset + numKeys; i < ctx.ArgCount(); i++ {
		args = append(args, ctx.ArgString(i))
	}

	registry := GetFunctionRegistry(ctx.Store)
	result, err := registry.CallFunction(libName, fnName, keys, args)
	if err != nil {
		return ctx.WriteError(err)
	}

	return writeLuaResult(ctx, result)
}

func cmdFCALL_RO(ctx *Context) error {
	return cmdFCALL(ctx)
}

func parseFuncInt(s string) (int64, error) {
	var n int64
	for _, c := range s {
		if c >= '0' && c <= '9' {
			n = n*10 + int64(c-'0')
		} else {
			return 0, fmt.Errorf("not an integer")
		}
	}
	return n, nil
}

func RegisterFunctionCommands(router *Router) {
	router.Register(&CommandDef{Name: "FUNCTION", Handler: cmdFUNCTION})
	router.Register(&CommandDef{Name: "FCALL", Handler: cmdFCALL})
	router.Register(&CommandDef{Name: "FCALL_RO", Handler: cmdFCALL_RO})
}
