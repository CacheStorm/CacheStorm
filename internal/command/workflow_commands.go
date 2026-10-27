package command

import (
	"fmt"
	"strings"
	"sync"

	"github.com/cachestorm/cachestorm/internal/resp"
	"github.com/cachestorm/cachestorm/internal/store"
)

func RegisterWorkflowCommands(router *Router) {
	router.Register(&CommandDef{Name: "STATEM.CREATE", Handler: cmdSTATEMCREATE})
	router.Register(&CommandDef{Name: "STATEM.DELETE", Handler: cmdSTATEMDELETE})
	router.Register(&CommandDef{Name: "STATEM.ADDSTATE", Handler: cmdSTATEMADDSTATE})
	router.Register(&CommandDef{Name: "STATEM.ADDTRANS", Handler: cmdSTATEMADDTRANS})
	router.Register(&CommandDef{Name: "STATEM.TRIGGER", Handler: cmdSTATEMTRIGGER})
	router.Register(&CommandDef{Name: "STATEM.CURRENT", Handler: cmdSTATEMCURRENT})
	router.Register(&CommandDef{Name: "STATEM.CANTRIGGER", Handler: cmdSTATEMCANTRIGGER})
	router.Register(&CommandDef{Name: "STATEM.EVENTS", Handler: cmdSTATEMEVENTS})
	router.Register(&CommandDef{Name: "STATEM.RESET", Handler: cmdSTATEMRESET})
	router.Register(&CommandDef{Name: "STATEM.ISFINAL", Handler: cmdSTATEMISFINAL})
	router.Register(&CommandDef{Name: "STATEM.INFO", Handler: cmdSTATEMINFO})
	router.Register(&CommandDef{Name: "STATEM.LIST", Handler: cmdSTATEMLIST})

	router.Register(&CommandDef{Name: "CHAINED.SET", Handler: cmdCHAINEDSET})
	router.Register(&CommandDef{Name: "CHAINED.GET", Handler: cmdCHAINEDGET})
	router.Register(&CommandDef{Name: "CHAINED.DEL", Handler: cmdCHAINEDDEL})
}

func cmdSTATEMCREATE(ctx *Context) error {
	if ctx.ArgCount() < 2 {
		return ctx.WriteError(ErrWrongArgCount)
	}

	name := ctx.ArgString(0)
	initial := ctx.ArgString(1)

	sm := store.GetOrCreateStateMachine(name, initial)

	return ctx.WriteArray([]*resp.Value{
		resp.BulkString("name"),
		resp.BulkString(sm.Name),
		resp.BulkString("initial"),
		resp.BulkString(sm.Initial),
		resp.BulkString("current"),
		resp.BulkString(sm.GetCurrentState()),
	})
}

func cmdSTATEMDELETE(ctx *Context) error {
	if ctx.ArgCount() < 1 {
		return ctx.WriteError(ErrWrongArgCount)
	}

	name := ctx.ArgString(0)

	if store.DeleteStateMachine(name) {
		return ctx.WriteInteger(1)
	}
	return ctx.WriteInteger(0)
}

func cmdSTATEMADDSTATE(ctx *Context) error {
	if ctx.ArgCount() < 2 {
		return ctx.WriteError(ErrWrongArgCount)
	}

	name := ctx.ArgString(0)
	stateName := ctx.ArgString(1)
	final := false
	if ctx.ArgCount() >= 3 {
		final = strings.ToUpper(ctx.ArgString(2)) == "TRUE"
	}

	sm, ok := store.GetStateMachine(name)
	if !ok {
		return ctx.WriteError(fmt.Errorf("ERR state machine not found"))
	}

	sm.AddState(stateName, final, "", "")

	return ctx.WriteOK()
}

func cmdSTATEMADDTRANS(ctx *Context) error {
	if ctx.ArgCount() < 4 {
		return ctx.WriteError(ErrWrongArgCount)
	}

	name := ctx.ArgString(0)
	from := ctx.ArgString(1)
	to := ctx.ArgString(2)
	event := ctx.ArgString(3)

	sm, ok := store.GetStateMachine(name)
	if !ok {
		return ctx.WriteError(fmt.Errorf("ERR state machine not found"))
	}

	sm.AddTransition(from, to, event)

	return ctx.WriteOK()
}

func cmdSTATEMTRIGGER(ctx *Context) error {
	if ctx.ArgCount() < 2 {
		return ctx.WriteError(ErrWrongArgCount)
	}

	name := ctx.ArgString(0)
	event := ctx.ArgString(1)

	sm, ok := store.GetStateMachine(name)
	if !ok {
		return ctx.WriteError(fmt.Errorf("ERR state machine not found"))
	}

	newState, err := sm.Trigger(event)
	if err != nil {
		return ctx.WriteError(err)
	}

	return ctx.WriteBulkString(newState)
}

func cmdSTATEMCURRENT(ctx *Context) error {
	if ctx.ArgCount() < 1 {
		return ctx.WriteError(ErrWrongArgCount)
	}

	name := ctx.ArgString(0)

	sm, ok := store.GetStateMachine(name)
	if !ok {
		return ctx.WriteError(fmt.Errorf("ERR state machine not found"))
	}

	return ctx.WriteBulkString(sm.GetCurrentState())
}

func cmdSTATEMCANTRIGGER(ctx *Context) error {
	if ctx.ArgCount() < 2 {
		return ctx.WriteError(ErrWrongArgCount)
	}

	name := ctx.ArgString(0)
	event := ctx.ArgString(1)

	sm, ok := store.GetStateMachine(name)
	if !ok {
		return ctx.WriteError(fmt.Errorf("ERR state machine not found"))
	}

	if sm.CanTrigger(event) {
		return ctx.WriteInteger(1)
	}
	return ctx.WriteInteger(0)
}

func cmdSTATEMEVENTS(ctx *Context) error {
	if ctx.ArgCount() < 1 {
		return ctx.WriteError(ErrWrongArgCount)
	}

	name := ctx.ArgString(0)

	sm, ok := store.GetStateMachine(name)
	if !ok {
		return ctx.WriteError(fmt.Errorf("ERR state machine not found"))
	}

	events := sm.GetValidEvents()

	results := make([]*resp.Value, len(events))
	for i, e := range events {
		results[i] = resp.BulkString(e)
	}

	return ctx.WriteArray(results)
}

func cmdSTATEMRESET(ctx *Context) error {
	if ctx.ArgCount() < 1 {
		return ctx.WriteError(ErrWrongArgCount)
	}

	name := ctx.ArgString(0)

	sm, ok := store.GetStateMachine(name)
	if !ok {
		return ctx.WriteError(fmt.Errorf("ERR state machine not found"))
	}

	sm.Reset()

	return ctx.WriteOK()
}

func cmdSTATEMISFINAL(ctx *Context) error {
	if ctx.ArgCount() < 1 {
		return ctx.WriteError(ErrWrongArgCount)
	}

	name := ctx.ArgString(0)

	sm, ok := store.GetStateMachine(name)
	if !ok {
		return ctx.WriteError(fmt.Errorf("ERR state machine not found"))
	}

	if sm.IsFinal() {
		return ctx.WriteInteger(1)
	}
	return ctx.WriteInteger(0)
}

func cmdSTATEMINFO(ctx *Context) error {
	if ctx.ArgCount() < 1 {
		return ctx.WriteError(ErrWrongArgCount)
	}

	name := ctx.ArgString(0)

	sm, ok := store.GetStateMachine(name)
	if !ok {
		return ctx.WriteError(fmt.Errorf("ERR state machine not found"))
	}

	info := sm.Info()

	return ctx.WriteArray([]*resp.Value{
		resp.BulkString("name"),
		resp.BulkString(mapString(info, "name")),
		resp.BulkString("current"),
		resp.BulkString(mapString(info, "current")),
		resp.BulkString("initial"),
		resp.BulkString(mapString(info, "initial")),
		resp.BulkString("is_final"),
		resp.BulkString(fmt.Sprintf("%v", info["is_final"])),
	})
}

func cmdSTATEMLIST(ctx *Context) error {
	names := store.ListStateMachines()

	results := make([]*resp.Value, len(names))
	for i, name := range names {
		results[i] = resp.BulkString(name)
	}

	return ctx.WriteArray(results)
}

var (
	chainedData   = make(map[string]map[string]string)
	chainedDataMu sync.RWMutex
)

func cmdCHAINEDSET(ctx *Context) error {
	if ctx.ArgCount() < 3 {
		return ctx.WriteError(ErrWrongArgCount)
	}

	rootKey := ctx.ArgString(0)
	path := ctx.ArgString(1)
	value := ctx.ArgString(2)

	chainedDataMu.Lock()
	defer chainedDataMu.Unlock()

	if _, exists := chainedData[rootKey]; !exists {
		chainedData[rootKey] = make(map[string]string)
	}

	chainedData[rootKey][path] = value

	return ctx.WriteOK()
}

func cmdCHAINEDGET(ctx *Context) error {
	if ctx.ArgCount() < 2 {
		return ctx.WriteError(ErrWrongArgCount)
	}

	rootKey := ctx.ArgString(0)
	path := ctx.ArgString(1)

	chainedDataMu.RLock()

	if root, exists := chainedData[rootKey]; exists {
		if val, ok := root[path]; ok {
			reply := val
			chainedDataMu.RUnlock()
			return ctx.WriteBulkString(reply)
		}
	}

	chainedDataMu.RUnlock()
	return ctx.WriteNull()
}

func cmdCHAINEDDEL(ctx *Context) error {
	if ctx.ArgCount() < 2 {
		return ctx.WriteError(ErrWrongArgCount)
	}

	rootKey := ctx.ArgString(0)
	path := ctx.ArgString(1)

	chainedDataMu.Lock()
	defer chainedDataMu.Unlock()

	if root, exists := chainedData[rootKey]; exists {
		if _, ok := root[path]; ok {
			delete(root, path)
			return ctx.WriteInteger(1)
		}
	}

	return ctx.WriteInteger(0)
}
