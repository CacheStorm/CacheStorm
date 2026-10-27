package command

import (
	"bytes"
	"context"
	"crypto/sha1"
	"encoding/binary"
	"encoding/hex"
	"errors"
	"fmt"
	"maps"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	lua "github.com/yuin/gopher-lua"

	"github.com/cachestorm/cachestorm/internal/resp"
	"github.com/cachestorm/cachestorm/internal/store"
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
	// Loaded marks a library from FUNCTION LOAD ("#!lua name=..."): its
	// functions are called by their bare name. Libraries from the older
	// FUNCTION CREATE are called as library.function.
	Loaded bool
}

type FunctionRegistry struct {
	mu        sync.RWMutex
	libraries map[string]*Library
	// functions indexes every function as "library.function".
	functions map[string]*Function
	// named indexes the functions of FUNCTION LOAD libraries by name; Redis
	// function names are global.
	named map[string]*Function
	store *store.Store
}

var functionRegistry *FunctionRegistry
var functionOnce sync.Once

func NewFunctionRegistry(s *store.Store) *FunctionRegistry {
	return &FunctionRegistry{
		libraries: make(map[string]*Library),
		functions: make(map[string]*Function),
		named:     make(map[string]*Function),
		store:     s,
	}
}

func (r *FunctionRegistry) CloneForStore(s *store.Store) *FunctionRegistry {
	r.mu.RLock()
	defer r.mu.RUnlock()
	n := NewFunctionRegistry(s)
	n.libraries = maps.Clone(r.libraries)
	n.functions = maps.Clone(r.functions)
	n.named = maps.Clone(r.named)
	return n
}

func GetFunctionRegistry(s *store.Store) *FunctionRegistry {
	functionOnce.Do(func() { functionRegistry = NewFunctionRegistry(s) })
	return functionRegistry
}

func functionRegistryFor(ctx *Context) *FunctionRegistry {
	if r := ctx.dispatcher(); r != nil {
		r.mu.RLock()
		registry := r.functionRegistry
		r.mu.RUnlock()
		if registry != nil {
			return registry
		}
	}
	return GetFunctionRegistry(ctx.Store)
}

const maxLibraries = 1000 // Maximum number of function libraries

var validFunctionFlags = map[string]bool{
	"no-writes": true, "allow-oom": true, "allow-stale": true,
	"no-cluster": true, "allow-cross-slot-keys": true,
}

const errFunctionNameText = "Library names can only contain letters, numbers, or underscores(_) and must be at least one character long"

// redisRegisterFunctionAlias returns the Redis 7 registration entry point as
// an alias for this engine's native redis.<name> = function carry:
// redis.register_function(name, fn) simply declares redis.<name> = fn, so both
// declaration conventions share one lookup path at call time.
func redisRegisterFunctionAlias(L *lua.LState) *lua.LFunction {
	return L.NewFunction(func(L *lua.LState) int {
		name := L.ToString(1)
		if name == "" {
			return 0
		}
		fn := L.Get(2)
		if tbl, ok := L.GetGlobal("redis").(*lua.LTable); ok {
			L.SetField(tbl, name, fn)
		}
		return 0
	})
}

func validFunctionName(name string) bool {
	if name == "" {
		return false
	}
	for i := 0; i < len(name); i++ {
		c := name[i]
		if c != '_' && (c < 'a' || c > 'z') && (c < 'A' || c > 'Z') && (c < '0' || c > '9') {
			return false
		}
	}
	return true
}

// registeredFunction is one redis.register_function call.
type registeredFunction struct {
	name        string
	callback    *lua.LFunction
	flags       []string
	description string
}

// functionCollector records redis.register_function calls. With alias set it
// also declares redis.<name> = callback, the FUNCTION CREATE convention.
type functionCollector struct {
	order []string
	fns   map[string]*registeredFunction
	alias bool
}

func newFunctionCollector(alias bool) *functionCollector {
	return &functionCollector{fns: make(map[string]*registeredFunction), alias: alias}
}

func (c *functionCollector) luaFunc(L *lua.LState) *lua.LFunction {
	return L.NewFunction(func(L *lua.LState) int {
		rf, msg := parseRegisterFunction(L)
		if msg != "" {
			raiseLuaErr(L, msg)
			return 0
		}
		if _, dup := c.fns[rf.name]; dup {
			raiseLuaErr(L, "ERR Function already exists in the library")
			return 0
		}
		c.fns[rf.name] = rf
		c.order = append(c.order, rf.name)
		if c.alias {
			if tbl, ok := L.GetGlobal("redis").(*lua.LTable); ok {
				tbl.RawSetString(rf.name, rf.callback)
			}
		}
		return 0
	})
}

// parseRegisterFunction validates the arguments of redis.register_function
// the way Redis does: (name, callback) or a table of named arguments.
func parseRegisterFunction(L *lua.LState) (*registeredFunction, string) {
	rf := &registeredFunction{}
	switch L.GetTop() {
	case 1:
		tbl, ok := L.Get(1).(*lua.LTable)
		if !ok {
			return nil, "ERR calling redis.register_function with a single argument is only applicable to Lua table (representing named arguments)."
		}
		msg := ""
		tbl.ForEach(func(k, v lua.LValue) {
			if msg != "" {
				return
			}
			key, _ := k.(lua.LString)
			switch string(key) {
			case "function_name":
				s, ok := v.(lua.LString)
				if !ok {
					msg = "ERR function_name argument given to redis.register_function must be a string"
					return
				}
				rf.name = string(s)
			case "callback":
				fn, ok := v.(*lua.LFunction)
				if !ok {
					msg = "ERR callback argument given to redis.register_function must be a function"
					return
				}
				rf.callback = fn
			case "description":
				s, ok := v.(lua.LString)
				if !ok {
					msg = "ERR description argument given to redis.register_function must be a string"
					return
				}
				rf.description = string(s)
			case "flags":
				ft, ok := v.(*lua.LTable)
				if !ok {
					msg = "ERR flags argument to redis.register_function must be a table representing function flags"
					return
				}
				for i := 1; i <= ft.Len(); i++ {
					f, ok := ft.RawGetInt(i).(lua.LString)
					if !ok || !validFunctionFlags[string(f)] {
						msg = "ERR unknown flag given"
						return
					}
					rf.flags = append(rf.flags, string(f))
				}
			default:
				msg = "ERR unknown argument given to redis.register_function"
			}
		})
		if msg != "" {
			return nil, msg
		}
		if rf.name == "" {
			return nil, "ERR redis.register_function must get a function name argument"
		}
		if rf.callback == nil {
			return nil, "ERR redis.register_function must get a callback argument"
		}
	case 2:
		s, ok := L.Get(1).(lua.LString)
		if !ok {
			return nil, "ERR first argument to redis.register_function must be a string"
		}
		fn, ok := L.Get(2).(*lua.LFunction)
		if !ok {
			return nil, "ERR second argument to redis.register_function must be a function"
		}
		rf.name, rf.callback = string(s), fn
	default:
		return nil, "ERR wrong number of arguments to redis.register_function"
	}
	if !validFunctionName(rf.name) {
		return nil, "ERR " + errFunctionNameText
	}
	return rf, ""
}

// parseLibraryShebang reads the "#!lua name=<lib>" first line of a FUNCTION
// LOAD payload. The returned body keeps the line count, so error lines match.
func parseLibraryShebang(code string) (string, string, error) {
	if !strings.HasPrefix(code, "#!") {
		return "", "", errors.New("ERR Missing library metadata")
	}
	line, body := code, ""
	if nl := strings.IndexByte(code, '\n'); nl >= 0 {
		line, body = code[:nl], code[nl:]
	}
	parts := strings.Fields(line[2:])
	engine := ""
	if len(parts) > 0 {
		engine = parts[0]
	}
	if !strings.EqualFold(engine, "lua") {
		return "", "", fmt.Errorf("ERR Engine '%s' not found", engine)
	}
	name := ""
	for _, p := range parts[1:] {
		if !strings.HasPrefix(p, "name=") {
			return "", "", fmt.Errorf("ERR Invalid metadata value given: %s", p)
		}
		name = p[len("name="):]
	}
	if name == "" {
		return "", "", errors.New("ERR Library name was not given")
	}
	if !validFunctionName(name) {
		return "", "", errors.New("ERR " + errFunctionNameText)
	}
	return name, body, nil
}

// collectLoadedFunctions runs a FUNCTION LOAD library body in the load-time
// environment Redis gives libraries: only the redis registration API, no
// other globals, and no redis.call.
func collectLoadedFunctions(body string) (*functionCollector, error) {
	L := lua.NewState(lua.Options{SkipOpenLibs: true})
	defer L.Close()

	fn, err := L.Load(strings.NewReader(body), luaChunkFunction)
	if err != nil {
		return nil, errors.New("ERR Error compiling function: " + luaCompileError(err))
	}

	coll := newFunctionCollector(false)
	redisTable := L.NewTable()
	redisTable.RawSetString("register_function", coll.luaFunc(L))
	installLoadTimeHelpers(L, redisTable)
	L.SetGlobal("redis", redisTable)
	protectGlobals(L)
	// The load-time redis table has no redis.call & co; reading one fails
	// the way Redis reports it.
	if mt, ok := redisTable.Metatable.(*lua.LTable); ok {
		mt.RawSetString("__index", L.NewFunction(func(L *lua.LState) int {
			L.RaiseError("Script attempted to access nonexistent global variable '%s'", L.ToString(2))
			return 0
		}))
	}

	tctx, cancel := context.WithTimeout(context.Background(), luaScriptTimeout)
	defer cancel()
	L.SetContext(tctx)

	L.Push(fn)
	if err := L.PCall(0, 0, nil); err != nil {
		if tctx.Err() != nil {
			return nil, errors.New("ERR FUNCTION LOAD timeout")
		}
		msg := err.Error()
		if apiErr, ok := err.(*lua.ApiError); ok && apiErr.Object != nil {
			msg = "ERR " + apiErr.Object.String()
			if tbl, ok := apiErr.Object.(*lua.LTable); ok {
				msg = "ERR unknown error"
				if s, ok := tbl.RawGetString("err").(lua.LString); ok {
					msg = string(s)
				}
			}
		}
		return nil, errors.New("ERR Error registering functions: " + msg)
	}
	if len(coll.order) == 0 {
		return nil, errors.New("ERR No functions registered")
	}
	return coll, nil
}

// installLoadTimeHelpers adds the redis.* entries available while a library
// loads besides register_function.
func installLoadTimeHelpers(L *lua.LState, redisTable *lua.LTable) {
	full := L.NewTable()
	installRedisHelpers(L, full)
	for _, name := range []string{"log", "setresp", "LOG_DEBUG", "LOG_VERBOSE", "LOG_NOTICE", "LOG_WARNING", "REDIS_VERSION", "REDIS_VERSION_NUM"} {
		redisTable.RawSetString(name, full.RawGetString(name))
	}
}

func (r *FunctionRegistry) ensureMaps() {
	if r.libraries == nil {
		r.libraries = make(map[string]*Library)
	}
	if r.functions == nil {
		r.functions = make(map[string]*Function)
	}
	if r.named == nil {
		r.named = make(map[string]*Function)
	}
}

// buildLoadedLibrary validates a FUNCTION LOAD payload into a library.
func buildLoadedLibrary(code string) (*Library, error) {
	name, body, err := parseLibraryShebang(code)
	if err != nil {
		return nil, err
	}
	coll, err := collectLoadedFunctions(body)
	if err != nil {
		return nil, err
	}
	sum := sha1.Sum([]byte(code))
	lib := &Library{
		Name:      name,
		Code:      code,
		SHA:       hex.EncodeToString(sum[:]),
		Functions: make(map[string]*Function),
		CreatedAt: time.Now(),
		Engine:    "LUA",
		Loaded:    true,
	}
	for _, fnName := range coll.order {
		rf := coll.fns[fnName]
		fnSHA := sha1.Sum([]byte(name + ":" + fnName + code))
		lib.Functions[fnName] = &Function{
			Name:        fnName,
			Library:     name,
			Code:        fnName,
			SHA:         hex.EncodeToString(fnSHA[:]),
			Description: rf.description,
			Flags:       rf.flags,
			CreatedAt:   time.Now(),
		}
	}
	return lib, nil
}

// buildCreatedLibrary evaluates a FUNCTION CREATE library: every function the
// code leaves in the redis table (redis.<name> = fn, or register_function)
// becomes a library function.
func buildCreatedLibrary(name string, code string) (*Library, error) {
	sha := sha1.Sum([]byte(code))
	lib := &Library{
		Name:      name,
		Code:      code,
		SHA:       hex.EncodeToString(sha[:]),
		Functions: make(map[string]*Function),
		CreatedAt: time.Now(),
		Engine:    "LUA",
	}

	L := lua.NewState()
	defer L.Close()

	coll := newFunctionCollector(true)
	redisTable := L.NewTable()
	L.SetGlobal("redis", redisTable)
	L.SetField(redisTable, "register_function", coll.luaFunc(L))

	if err := L.DoString(code); err != nil {
		return nil, fmt.Errorf("ERR failed to load library: %v", err)
	}

	if tbl, ok := L.GetGlobal("redis").(*lua.LTable); ok {
		L.ForEach(tbl, func(key, value lua.LValue) {
			strKey, ok := key.(lua.LString)
			if !ok {
				return
			}
			if _, ok := value.(*lua.LFunction); !ok {
				return
			}
			fnName := string(strKey)
			if strings.HasPrefix(fnName, "_") || fnName == "register_function" {
				return
			}
			fnSHA := sha1.Sum([]byte(name + ":" + fnName + code))
			function := &Function{
				Name:      fnName,
				Library:   name,
				Code:      fnName,
				SHA:       hex.EncodeToString(fnSHA[:]),
				CreatedAt: time.Now(),
			}
			if rf, ok := coll.fns[fnName]; ok {
				function.Flags = rf.flags
				function.Description = rf.description
			}
			lib.Functions[fnName] = function
		})
	}
	return lib, nil
}

// installLocked adds lib, replacing a library of the same name when replace
// is set. The caller holds r.mu.
func (r *FunctionRegistry) installLocked(lib *Library, replace bool, existsErr string) error {
	r.ensureMaps()
	old, exists := r.libraries[lib.Name]
	if exists && !replace {
		return errors.New(existsErr)
	}
	if !exists && len(r.libraries) >= maxLibraries {
		return fmt.Errorf("ERR maximum number of libraries (%d) reached", maxLibraries)
	}
	if lib.Loaded {
		for fnName := range lib.Functions {
			if f, ok := r.named[fnName]; ok && f.Library != lib.Name {
				return fmt.Errorf("ERR Function %s already exists", fnName)
			}
		}
	}
	if exists {
		r.removeLocked(old)
	}
	for fnName, function := range lib.Functions {
		r.functions[lib.Name+"."+fnName] = function
		if lib.Loaded {
			r.named[fnName] = function
		}
	}
	r.libraries[lib.Name] = lib
	return nil
}

func (r *FunctionRegistry) removeLocked(lib *Library) {
	for fnName := range lib.Functions {
		delete(r.functions, lib.Name+"."+fnName)
		if lib.Loaded {
			if f, ok := r.named[fnName]; ok && f.Library == lib.Name {
				delete(r.named, fnName)
			}
		}
	}
	delete(r.libraries, lib.Name)
}

// LoadLibrary implements FUNCTION LOAD and returns the library name.
func (r *FunctionRegistry) LoadLibrary(code string, replace bool) (string, error) {
	lib, err := buildLoadedLibrary(code)
	if err != nil {
		return "", err
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	if err := r.installLocked(lib, replace, fmt.Sprintf("ERR Library '%s' already exists", lib.Name)); err != nil {
		return "", err
	}
	return lib.Name, nil
}

func (r *FunctionRegistry) CreateLibrary(name string, code string, replace bool) error {
	r.mu.RLock()
	_, exists := r.libraries[name]
	r.mu.RUnlock()
	if exists && !replace {
		return fmt.Errorf("ERR library '%s' already exists", name)
	}

	lib, err := buildCreatedLibrary(name, code)
	if err != nil {
		return err
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.installLocked(lib, replace, fmt.Sprintf("ERR library '%s' already exists", name))
}

func (r *FunctionRegistry) DeleteLibrary(name string) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	lib, exists := r.libraries[name]
	if !exists {
		return errors.New("ERR Library not found")
	}
	r.removeLocked(lib)
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
		if pattern == "" || store.MatchPattern(name, pattern) {
			result = append(result, lib)
		}
	}
	sort.Slice(result, func(i, j int) bool {
		return result[i].Name < result[j].Name
	})
	return result
}

// CallFunction runs lib.fn with no ACL guard; use CallFunctionGuarded for a
// script run on behalf of an ACL user.
func (r *FunctionRegistry) CallFunction(libName string, fnName string, keys []string, args []string) (interface{}, error) {
	return r.callFunction(libName, fnName, keys, args, nil)
}

// CallFunctionGuarded runs lib.fn with the guard applied to every redis.call /
// redis.pcall the function makes, so keys the function touches — declared in
// numkeys or not — are subject to the invoking user's key patterns.
func (r *FunctionRegistry) CallFunctionGuarded(libName string, fnName string, keys []string, args []string, guard ScriptCallGuard) (interface{}, error) {
	return r.callFunction(libName, fnName, keys, args, guard)
}

func (r *FunctionRegistry) callFunction(libName string, fnName string, keys []string, args []string, guard ScriptCallGuard) (interface{}, error) {
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
	if lib.Loaded {
		code = libraryBody(code)
	}
	r.mu.RUnlock()

	se := NewScriptEngine(r.store)
	L := se.createState(keys, args, guard)
	defer L.Close()
	coll := newFunctionCollector(true)
	L.SetField(L.GetGlobal("redis"), "register_function", coll.luaFunc(L))

	if err := se.protected(func() error { return L.DoString(code) }); err != nil {
		return nil, fmt.Errorf("ERR failed to load library: %v", err)
	}

	if luaFn := functionCallback(L, coll, fnName); luaFn != nil {
		if err := se.protected(func() error {
			// Redis Functions convention: the function receives the
			// declared keys and args as two table parameters. Zero-param
			// library functions simply ignore them, so the older
			// redis.KEYS/redis.ARGV convention keeps working.
			return L.CallByParam(lua.P{
				Fn:      luaFn,
				NRet:    1,
				Protect: true,
			}, luaStringList(L, keys), luaStringList(L, args))
		}); err != nil {
			return nil, fmt.Errorf("ERR %v", err)
		}
		ret := L.Get(-1)
		L.Pop(1)
		return luaToGo(ret), nil
	}

	return nil, fmt.Errorf("ERR function '%s' not callable", fnName)
}

func luaStringList(L *lua.LState, items []string) *lua.LTable {
	tbl := L.CreateTable(len(items), 0)
	for _, s := range items {
		tbl.Append(lua.LString(s))
	}
	return tbl
}

// functionCallback finds fnName among the registered callbacks, then among
// the redis.<name> fields of the FUNCTION CREATE convention.
func functionCallback(L *lua.LState, coll *functionCollector, fnName string) *lua.LFunction {
	if rf, ok := coll.fns[fnName]; ok {
		return rf.callback
	}
	if tbl, ok := L.GetGlobal("redis").(*lua.LTable); ok {
		if fn, ok := tbl.RawGetString(fnName).(*lua.LFunction); ok {
			return fn
		}
	}
	return nil
}

func (r *FunctionRegistry) GetFunction(libName string, fnName string) (*Function, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	fn, ok := r.functions[libName+"."+fnName]
	return fn, ok
}

// resolveFunction finds the function FCALL names: a FUNCTION LOAD function by
// its bare name, else a FUNCTION CREATE function as library.function.
func (r *FunctionRegistry) resolveFunction(name string) (*Library, *Function, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	if f, ok := r.named[name]; ok {
		if lib, ok := r.libraries[f.Library]; ok {
			return lib, f, true
		}
	}
	if dot := strings.IndexByte(name, '.'); dot > 0 {
		if lib, ok := r.libraries[name[:dot]]; ok {
			if f, ok := lib.Functions[name[dot+1:]]; ok {
				return lib, f, true
			}
		}
	}
	return nil, nil, false
}

// libraryBody is the code of a FUNCTION LOAD library without its shebang
// line (the line itself is kept empty).
func libraryBody(code string) string {
	if _, body, err := parseLibraryShebang(code); err == nil {
		return body
	}
	return code
}

func hasFlag(flags []string, flag string) bool {
	for _, f := range flags {
		if f == flag {
			return true
		}
	}
	return false
}

// runFunction runs fn of lib for the client in ctx and returns its reply.
func runFunction(ctx *Context, lib *Library, fn *Function, keys, args []string, readOnly bool) *resp.Value {
	code := lib.Code
	if lib.Loaded {
		code = libraryBody(code)
	}
	engine := NewScriptEngine(ctx.Store)
	run := newScriptRun(ctx, engine, fn.Name, luaChunkFunction, readOnly || hasFlag(fn.Flags, "no-writes"))
	L := engine.newState(keys, args, run.guard, run)
	defer L.Close()

	coll := newFunctionCollector(!lib.Loaded)
	L.SetField(L.GetGlobal("redis"), "register_function", coll.luaFunc(L))

	chunk, err := L.Load(strings.NewReader(code), luaChunkFunction)
	if err != nil {
		return resp.ErrorValue("ERR Error compiling function: " + luaCompileError(err))
	}

	tctx, cancel := context.WithTimeout(context.Background(), luaScriptTimeout)
	defer cancel()
	L.SetContext(tctx)

	L.Push(chunk)
	if err := L.PCall(0, 0, run.errHandler(L)); err != nil {
		return run.finish(L, tctx, err)
	}
	callback := functionCallback(L, coll, fn.Name)
	if callback == nil {
		return resp.ErrorValue("ERR Function not found")
	}
	L.Push(callback)
	L.Push(luaStringList(L, keys))
	L.Push(luaStringList(L, args))
	err = L.PCall(2, 1, run.errHandler(L))
	return run.finish(L, tctx, err)
}

func cmdFUNCTION(ctx *Context) error {
	if ctx.ArgCount() < 1 {
		return ctx.WriteError(ErrWrongArgCount)
	}

	subCmd := strings.ToUpper(ctx.ArgString(0))
	registry := functionRegistryFor(ctx)

	switch subCmd {
	case "LOAD":
		return handleFunctionLoad(ctx, registry)
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
	case "KILL":
		return ctx.WriteError(replyError("NOTBUSY No scripts in execution right now."))
	default:
		return ctx.WriteError(replyError("ERR unknown subcommand '" + ctx.ArgString(0) + "'. Try FUNCTION HELP."))
	}
}

func functionArgError(sub string) error {
	return errors.New("ERR wrong number of arguments for 'function|" + sub + "' command")
}

func handleFunctionLoad(ctx *Context, registry *FunctionRegistry) error {
	if ctx.ArgCount() < 2 {
		return ctx.WriteError(functionArgError("load"))
	}
	replace := false
	for i := 1; i < ctx.ArgCount()-1; i++ {
		opt := ctx.ArgString(i)
		if !strings.EqualFold(opt, "REPLACE") {
			return ctx.WriteError(fmt.Errorf("ERR Unknown option given: %s", opt))
		}
		replace = true
	}
	name, err := registry.LoadLibrary(ctx.ArgString(ctx.ArgCount()-1), replace)
	if err != nil {
		return ctx.WriteError(err)
	}
	return ctx.WriteBulkString(name)
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
	if ctx.ArgCount() != 2 {
		return ctx.WriteError(functionArgError("delete"))
	}

	libName := ctx.ArgString(1)
	if err := registry.DeleteLibrary(libName); err != nil {
		return ctx.WriteError(err)
	}

	return ctx.WriteOK()
}

func handleFunctionList(ctx *Context, registry *FunctionRegistry) error {
	pattern := ""
	withCode := false
	for i := 1; i < ctx.ArgCount(); i++ {
		arg := ctx.ArgString(i)
		switch strings.ToUpper(arg) {
		case "WITHCODE":
			withCode = true
		case "LIBRARYNAME":
			if i+1 >= ctx.ArgCount() {
				return ctx.WriteError(errors.New("ERR library name argument was not given"))
			}
			i++
			pattern = ctx.ArgString(i)
		default:
			return ctx.WriteError(fmt.Errorf("ERR Unknown argument %s", arg))
		}
	}

	libraries := registry.ListLibraries(pattern)
	result := make([]*resp.Value, 0, len(libraries))

	for _, lib := range libraries {
		names := make([]string, 0, len(lib.Functions))
		for fnName := range lib.Functions {
			names = append(names, fnName)
		}
		sort.Strings(names)
		fnList := make([]*resp.Value, 0, len(names))
		for _, fnName := range names {
			fn := lib.Functions[fnName]
			desc := resp.NullBulkString()
			if fn.Description != "" {
				desc = resp.BulkString(fn.Description)
			}
			flags := make([]*resp.Value, 0, len(fn.Flags))
			for _, f := range fn.Flags {
				flags = append(flags, resp.SimpleString(f))
			}
			fnList = append(fnList, resp.ArrayValue([]*resp.Value{
				resp.BulkString("name"),
				resp.BulkString(fnName),
				resp.BulkString("description"),
				desc,
				resp.BulkString("flags"),
				resp.ArrayValue(flags),
			}))
		}

		libInfo := []*resp.Value{
			resp.BulkString("library_name"),
			resp.BulkString(lib.Name),
			resp.BulkString("engine"),
			resp.BulkString(lib.Engine),
			resp.BulkString("functions"),
			resp.ArrayValue(fnList),
		}
		if withCode {
			libInfo = append(libInfo, resp.BulkString("library_code"), resp.BulkString(lib.Code))
		}
		result = append(result, resp.ArrayValue(libInfo))
	}

	return ctx.WriteArray(result)
}

// functionDumpMagic starts a FUNCTION DUMP payload: a version byte, then for
// each library its name, whether it came from FUNCTION LOAD, and its code.
const functionDumpMagic = "CSFN\x01"

func handleFunctionDump(ctx *Context, registry *FunctionRegistry) error {
	return ctx.WriteBulkBytes(registry.DumpPayload())
}

func (registry *FunctionRegistry) DumpPayload() []byte {
	libs := registry.ListLibraries("")
	var buf bytes.Buffer
	buf.WriteString(functionDumpMagic)
	for _, lib := range libs {
		writeDumpString(&buf, lib.Name)
		if lib.Loaded {
			buf.WriteByte(1)
		} else {
			buf.WriteByte(0)
		}
		writeDumpString(&buf, lib.Code)
	}
	return buf.Bytes()
}

func writeDumpString(buf *bytes.Buffer, s string) {
	var n [binary.MaxVarintLen64]byte
	buf.Write(n[:binary.PutUvarint(n[:], uint64(len(s)))])
	buf.WriteString(s)
}

func readDumpString(b []byte, pos int) (string, int, bool) {
	n, w := binary.Uvarint(b[pos:])
	if w <= 0 || uint64(len(b)-pos-w) < n {
		return "", 0, false
	}
	start := pos + w
	return string(b[start : start+int(n)]), start + int(n), true
}

var errFunctionPayload = errors.New("ERR DUMP payload version or checksum are wrong")

func handleFunctionRestore(ctx *Context, registry *FunctionRegistry) error {
	if ctx.ArgCount() < 2 || ctx.ArgCount() > 3 {
		return ctx.WriteError(functionArgError("restore"))
	}
	policy := "APPEND"
	if ctx.ArgCount() == 3 {
		policy = strings.ToUpper(ctx.ArgString(2))
		if policy != "APPEND" && policy != "REPLACE" && policy != "FLUSH" {
			return ctx.WriteError(replyError("ERR Wrong restore policy given, value should be either FLUSH, APPEND or REPLACE."))
		}
	}
	payload := ctx.Args[1]
	if !bytes.HasPrefix(payload, []byte(functionDumpMagic)) {
		return ctx.WriteError(errFunctionPayload)
	}
	var libs []*Library
	for pos := len(functionDumpMagic); pos < len(payload); {
		name, next, ok := readDumpString(payload, pos)
		if !ok || next >= len(payload) {
			return ctx.WriteError(errFunctionPayload)
		}
		loaded := payload[next] == 1
		code, after, ok := readDumpString(payload, next+1)
		if !ok {
			return ctx.WriteError(errFunctionPayload)
		}
		pos = after
		var lib *Library
		var err error
		if loaded {
			lib, err = buildLoadedLibrary(code)
		} else {
			lib, err = buildCreatedLibrary(name, code)
		}
		if err != nil {
			return ctx.WriteError(err)
		}
		libs = append(libs, lib)
	}

	registry.mu.Lock()
	defer registry.mu.Unlock()
	registry.ensureMaps()
	if policy == "APPEND" {
		for _, lib := range libs {
			if _, exists := registry.libraries[lib.Name]; exists {
				return ctx.WriteError(fmt.Errorf("ERR Library %s already exists", lib.Name))
			}
		}
	}
	savedLibs, savedFns, savedNamed := registry.libraries, registry.functions, registry.named
	if policy == "FLUSH" {
		registry.libraries = make(map[string]*Library)
		registry.functions = make(map[string]*Function)
		registry.named = make(map[string]*Function)
	} else {
		registry.libraries = copyLibraries(savedLibs)
		registry.functions = copyFunctions(savedFns)
		registry.named = copyFunctions(savedNamed)
	}
	for _, lib := range libs {
		if err := registry.installLocked(lib, true, ""); err != nil {
			registry.libraries, registry.functions, registry.named = savedLibs, savedFns, savedNamed
			return ctx.WriteError(err)
		}
	}
	return ctx.WriteOK()
}

func copyLibraries(m map[string]*Library) map[string]*Library {
	out := make(map[string]*Library, len(m))
	for k, v := range m {
		out[k] = v
	}
	return out
}

func copyFunctions(m map[string]*Function) map[string]*Function {
	out := make(map[string]*Function, len(m))
	for k, v := range m {
		out[k] = v
	}
	return out
}

func handleFunctionStats(ctx *Context, registry *FunctionRegistry) error {
	registry.mu.RLock()
	libCount, fnCount := len(registry.libraries), 0
	for _, lib := range registry.libraries {
		fnCount += len(lib.Functions)
	}
	registry.mu.RUnlock()

	return ctx.WriteArray([]*resp.Value{
		resp.BulkString("running_script"),
		resp.NullBulkString(),
		resp.BulkString("engines"),
		resp.ArrayValue([]*resp.Value{
			resp.BulkString("LUA"),
			resp.ArrayValue([]*resp.Value{
				resp.BulkString("libraries_count"),
				resp.IntegerValue(int64(libCount)),
				resp.BulkString("functions_count"),
				resp.IntegerValue(int64(fnCount)),
			}),
		}),
	})
}

func handleFunctionFlush(ctx *Context, registry *FunctionRegistry) error {
	if ctx.ArgCount() > 2 {
		return ctx.WriteError(functionArgError("flush"))
	}
	if ctx.ArgCount() == 2 {
		if mode := strings.ToUpper(ctx.ArgString(1)); mode != "SYNC" && mode != "ASYNC" {
			return ctx.WriteError(errors.New("ERR FUNCTION FLUSH only supports SYNC|ASYNC option"))
		}
	}
	registry.mu.Lock()
	defer registry.mu.Unlock()

	registry.libraries = make(map[string]*Library)
	registry.functions = make(map[string]*Function)
	registry.named = make(map[string]*Function)

	return ctx.WriteOK()
}

// fcallKeysAndArgs parses "FCALL name numkeys key... arg...". A call without
// numkeys (only reachable when the handler is invoked directly) has no keys.
func fcallKeysAndArgs(ctx *Context) ([]string, []string, error) {
	if ctx.ArgCount() < 2 {
		return nil, nil, nil
	}
	n, err := strconv.ParseInt(ctx.ArgString(1), 10, 64)
	if err != nil {
		return nil, nil, errors.New("ERR Bad number of keys provided")
	}
	if n < 0 {
		return nil, nil, errors.New("ERR Number of keys can't be negative")
	}
	if n > int64(ctx.ArgCount()-2) {
		return nil, nil, errors.New("ERR Number of keys can't be greater than number of args")
	}
	keys := make([]string, 0, n)
	args := make([]string, 0, ctx.ArgCount()-2-int(n))
	for i := 2; i < ctx.ArgCount(); i++ {
		if i < 2+int(n) {
			keys = append(keys, ctx.ArgString(i))
		} else {
			args = append(args, ctx.ArgString(i))
		}
	}
	return keys, args, nil
}

func cmdFCALL(ctx *Context) error {
	return fcall(ctx, false)
}

func cmdFCALL_RO(ctx *Context) error {
	return fcall(ctx, true)
}

func fcall(ctx *Context, readOnly bool) error {
	if ctx.ArgCount() < 1 {
		return ctx.WriteError(ErrWrongArgCount)
	}
	keys, args, err := fcallKeysAndArgs(ctx)
	if err != nil {
		return ctx.WriteError(err)
	}
	registry := functionRegistryFor(ctx)
	lib, fn, ok := registry.resolveFunction(ctx.ArgString(0))
	if !ok {
		return ctx.WriteError(errors.New("ERR Function not found"))
	}
	if readOnly && !hasFlag(fn.Flags, "no-writes") {
		return ctx.WriteError(replyError("ERR Can not execute a script with write flag using *_ro command."))
	}
	return ctx.WriteValue(runFunction(ctx, lib, fn, keys, args, false))
}

func RegisterFunctionCommands(router *Router) {
	router.Register(&CommandDef{Name: "FUNCTION", Handler: cmdFUNCTION})
	router.Register(&CommandDef{Name: "FCALL", Handler: cmdFCALL})
	router.Register(&CommandDef{Name: "FCALL_RO", Handler: cmdFCALL_RO})
}
