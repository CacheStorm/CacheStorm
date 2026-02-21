package command

import (
	"fmt"
	"strings"

	"github.com/cachestorm/cachestorm/internal/resp"
	"github.com/cachestorm/cachestorm/internal/store"
)

func RegisterWorkflowCommands(router *Router) {
	router.Register(&CommandDef{Name: "WORKFLOW.CREATE", Handler: cmdWORKFLOWCREATE})
	router.Register(&CommandDef{Name: "WORKFLOW.DELETE", Handler: cmdWORKFLOWDELETE})
	router.Register(&CommandDef{Name: "WORKFLOW.GET", Handler: cmdWORKFLOWGET})
	router.Register(&CommandDef{Name: "WORKFLOW.LIST", Handler: cmdWORKFLOWLIST})
	router.Register(&CommandDef{Name: "WORKFLOW.START", Handler: cmdWORKFLOWSTART})
	router.Register(&CommandDef{Name: "WORKFLOW.PAUSE", Handler: cmdWORKFLOWPAUSE})
	router.Register(&CommandDef{Name: "WORKFLOW.COMPLETE", Handler: cmdWORKFLOWCOMPLETE})
	router.Register(&CommandDef{Name: "WORKFLOW.FAIL", Handler: cmdWORKFLOWFAIL})
	router.Register(&CommandDef{Name: "WORKFLOW.RESET", Handler: cmdWORKFLOWRESET})
	router.Register(&CommandDef{Name: "WORKFLOW.NEXT", Handler: cmdWORKFLOWNEXT})
	router.Register(&CommandDef{Name: "WORKFLOW.SETVAR", Handler: cmdWORKFLOWSETVAR})
	router.Register(&CommandDef{Name: "WORKFLOW.GETVAR", Handler: cmdWORKFLOWGETVAR})
	router.Register(&CommandDef{Name: "WORKFLOW.ADDSTEP", Handler: cmdWORKFLOWADDSTEP})

	router.Register(&CommandDef{Name: "TEMPLATE.CREATE", Handler: cmdTEMPLATECREATE})
	router.Register(&CommandDef{Name: "TEMPLATE.DELETE", Handler: cmdTEMPLATEDELETE})
	router.Register(&CommandDef{Name: "TEMPLATE.GET", Handler: cmdTEMPLATEGET})
	router.Register(&CommandDef{Name: "TEMPLATE.INSTANTIATE", Handler: cmdTEMPLATEINSTANTIATE})

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

	router.Register(&CommandDef{Name: "REACTIVE.WATCH", Handler: cmdREACTIVEWATCH})
	router.Register(&CommandDef{Name: "REACTIVE.UNWATCH", Handler: cmdREACTIVEUNWATCH})
	router.Register(&CommandDef{Name: "REACTIVE.TRIGGER", Handler: cmdREACTIVETRIGGER})
}

func cmdWORKFLOWCREATE(ctx *Context) error {
	if ctx.ArgCount() < 2 {
		return ctx.WriteError(ErrWrongArgCount)
	}

	id := ctx.ArgString(0)
	name := ctx.ArgString(1)

	steps := make([]store.WorkflowStep, 0)
	for i := 2; i+3 < ctx.ArgCount(); i += 4 {
		step := store.WorkflowStep{
			ID:      ctx.ArgString(i),
			Name:    ctx.ArgString(i + 1),
			Command: ctx.ArgString(i + 2),
			Timeout: parseInt64(ctx.ArgString(i + 3)),
		}
		steps = append(steps, step)
	}

	workflow := store.GlobalWorkflowManager.Create(id, name, steps)

	return ctx.WriteArray([]*resp.Value{
		resp.BulkString("id"),
		resp.BulkString(workflow.ID),
		resp.BulkString("name"),
		resp.BulkString(workflow.Name),
		resp.BulkString("status"),
		resp.BulkString(workflow.Status.String()),
	})
}

func cmdWORKFLOWDELETE(ctx *Context) error {
	if ctx.ArgCount() < 1 {
		return ctx.WriteError(ErrWrongArgCount)
	}

	id := ctx.ArgString(0)

	if store.GlobalWorkflowManager.Delete(id) {
		return ctx.WriteInteger(1)
	}
	return ctx.WriteInteger(0)
}

func cmdWORKFLOWGET(ctx *Context) error {
	if ctx.ArgCount() < 1 {
		return ctx.WriteError(ErrWrongArgCount)
	}

	id := ctx.ArgString(0)

	workflow, ok := store.GlobalWorkflowManager.Get(id)
	if !ok {
		return ctx.WriteNull()
	}

	info := workflow.Info()

	return ctx.WriteArray([]*resp.Value{
		resp.BulkString("id"),
		resp.BulkString(info["id"].(string)),
		resp.BulkString("name"),
		resp.BulkString(info["name"].(string)),
		resp.BulkString("status"),
		resp.BulkString(info["status"].(string)),
		resp.BulkString("current_step"),
		resp.IntegerValue(int64(info["current_step"].(int))),
		resp.BulkString("total_steps"),
		resp.IntegerValue(int64(info["total_steps"].(int))),
		resp.BulkString("error"),
		resp.BulkString(info["error"].(string)),
	})
}

func cmdWORKFLOWLIST(ctx *Context) error {
	workflows := store.GlobalWorkflowManager.List()

	results := make([]*resp.Value, 0)
	for _, w := range workflows {
		results = append(results,
			resp.BulkString(w.ID),
			resp.BulkString(w.Name),
			resp.BulkString(w.Status.String()),
		)
	}

	return ctx.WriteArray(results)
}

func cmdWORKFLOWSTART(ctx *Context) error {
	if ctx.ArgCount() < 1 {
		return ctx.WriteError(ErrWrongArgCount)
	}

	id := ctx.ArgString(0)

	err := store.GlobalWorkflowManager.Start(id)
	if err != nil {
		return ctx.WriteError(err)
	}

	return ctx.WriteOK()
}

func cmdWORKFLOWPAUSE(ctx *Context) error {
	if ctx.ArgCount() < 1 {
		return ctx.WriteError(ErrWrongArgCount)
	}

	id := ctx.ArgString(0)

	err := store.GlobalWorkflowManager.Pause(id)
	if err != nil {
		return ctx.WriteError(err)
	}

	return ctx.WriteOK()
}

func cmdWORKFLOWCOMPLETE(ctx *Context) error {
	if ctx.ArgCount() < 1 {
		return ctx.WriteError(ErrWrongArgCount)
	}

	id := ctx.ArgString(0)

	err := store.GlobalWorkflowManager.Complete(id)
	if err != nil {
		return ctx.WriteError(err)
	}

	return ctx.WriteOK()
}

func cmdWORKFLOWFAIL(ctx *Context) error {
	if ctx.ArgCount() < 2 {
		return ctx.WriteError(ErrWrongArgCount)
	}

	id := ctx.ArgString(0)
	errMsg := ctx.ArgString(1)

	err := store.GlobalWorkflowManager.Fail(id, errMsg)
	if err != nil {
		return ctx.WriteError(err)
	}

	return ctx.WriteOK()
}

func cmdWORKFLOWRESET(ctx *Context) error {
	if ctx.ArgCount() < 1 {
		return ctx.WriteError(ErrWrongArgCount)
	}

	id := ctx.ArgString(0)

	err := store.GlobalWorkflowManager.Reset(id)
	if err != nil {
		return ctx.WriteError(err)
	}

	return ctx.WriteOK()
}

func cmdWORKFLOWNEXT(ctx *Context) error {
	if ctx.ArgCount() < 1 {
		return ctx.WriteError(ErrWrongArgCount)
	}

	id := ctx.ArgString(0)

	step, err := store.GlobalWorkflowManager.NextStep(id)
	if err != nil {
		return ctx.WriteError(err)
	}

	return ctx.WriteArray([]*resp.Value{
		resp.BulkString("id"),
		resp.BulkString(step.ID),
		resp.BulkString("name"),
		resp.BulkString(step.Name),
		resp.BulkString("command"),
		resp.BulkString(step.Command),
	})
}

func cmdWORKFLOWSETVAR(ctx *Context) error {
	if ctx.ArgCount() < 3 {
		return ctx.WriteError(ErrWrongArgCount)
	}

	id := ctx.ArgString(0)
	key := ctx.ArgString(1)
	value := ctx.ArgString(2)

	err := store.GlobalWorkflowManager.SetVariable(id, key, value)
	if err != nil {
		return ctx.WriteError(err)
	}

	return ctx.WriteOK()
}

func cmdWORKFLOWGETVAR(ctx *Context) error {
	if ctx.ArgCount() < 2 {
		return ctx.WriteError(ErrWrongArgCount)
	}

	id := ctx.ArgString(0)
	key := ctx.ArgString(1)

	value, ok := store.GlobalWorkflowManager.GetVariable(id, key)
	if !ok {
		return ctx.WriteNull()
	}

	return ctx.WriteBulkString(value)
}

func cmdWORKFLOWADDSTEP(ctx *Context) error {
	if ctx.ArgCount() < 5 {
		return ctx.WriteError(ErrWrongArgCount)
	}

	id := ctx.ArgString(0)
	stepID := ctx.ArgString(1)
	name := ctx.ArgString(2)
	command := ctx.ArgString(3)
	timeout := parseInt64(ctx.ArgString(4))

	workflow, ok := store.GlobalWorkflowManager.Get(id)
	if !ok {
		return ctx.WriteError(fmt.Errorf("ERR workflow not found"))
	}

	step := store.WorkflowStep{
		ID:      stepID,
		Name:    name,
		Command: command,
		Timeout: timeout,
	}

	workflow.Steps = append(workflow.Steps, step)

	return ctx.WriteInteger(int64(len(workflow.Steps)))
}

func cmdTEMPLATECREATE(ctx *Context) error {
	if ctx.ArgCount() < 2 {
		return ctx.WriteError(ErrWrongArgCount)
	}

	name := ctx.ArgString(0)
	stepCount := int(parseInt64(ctx.ArgString(1)))

	steps := make([]store.WorkflowStep, 0)
	idx := 2

	for i := 0; i < stepCount && idx+3 < ctx.ArgCount(); i++ {
		step := store.WorkflowStep{
			ID:      ctx.ArgString(idx),
			Name:    ctx.ArgString(idx + 1),
			Command: ctx.ArgString(idx + 2),
			Timeout: parseInt64(ctx.ArgString(idx + 3)),
		}
		steps = append(steps, step)
		idx += 4
	}

	template := store.GlobalWorkflowManager.CreateTemplate(name, steps)

	return ctx.WriteArray([]*resp.Value{
		resp.BulkString("name"),
		resp.BulkString(template.Name),
		resp.BulkString("steps"),
		resp.IntegerValue(int64(len(template.Steps))),
	})
}

func cmdTEMPLATEDELETE(ctx *Context) error {
	if ctx.ArgCount() < 1 {
		return ctx.WriteError(ErrWrongArgCount)
	}

	name := ctx.ArgString(0)

	if store.GlobalWorkflowManager.DeleteTemplate(name) {
		return ctx.WriteInteger(1)
	}
	return ctx.WriteInteger(0)
}

func cmdTEMPLATEGET(ctx *Context) error {
	if ctx.ArgCount() < 1 {
		return ctx.WriteError(ErrWrongArgCount)
	}

	name := ctx.ArgString(0)

	template, ok := store.GlobalWorkflowManager.GetTemplate(name)
	if !ok {
		return ctx.WriteNull()
	}

	results := make([]*resp.Value, 0)
	results = append(results,
		resp.BulkString("name"),
		resp.BulkString(template.Name),
		resp.BulkString("steps"),
	)

	stepResults := make([]*resp.Value, 0)
	for _, s := range template.Steps {
		stepResults = append(stepResults, resp.ArrayValue([]*resp.Value{
			resp.BulkString(s.ID),
			resp.BulkString(s.Name),
			resp.BulkString(s.Command),
		}))
	}
	results = append(results, resp.ArrayValue(stepResults))

	return ctx.WriteArray(results)
}

func cmdTEMPLATEINSTANTIATE(ctx *Context) error {
	if ctx.ArgCount() < 2 {
		return ctx.WriteError(ErrWrongArgCount)
	}

	templateName := ctx.ArgString(0)
	workflowID := ctx.ArgString(1)

	workflow, err := store.GlobalWorkflowManager.CreateFromTemplate(templateName, workflowID)
	if err != nil {
		return ctx.WriteError(err)
	}

	return ctx.WriteArray([]*resp.Value{
		resp.BulkString("id"),
		resp.BulkString(workflow.ID),
		resp.BulkString("name"),
		resp.BulkString(workflow.Name),
		resp.BulkString("status"),
		resp.BulkString(workflow.Status.String()),
	})
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
		resp.BulkString(info["name"].(string)),
		resp.BulkString("current"),
		resp.BulkString(info["current"].(string)),
		resp.BulkString("initial"),
		resp.BulkString(info["initial"].(string)),
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
	chainedDataMu syncRWMutexExt
)

type syncRWMutexExt struct{}

func (m *syncRWMutexExt) Lock()    {}
func (m *syncRWMutexExt) Unlock()  {}
func (m *syncRWMutexExt) RLock()   {}
func (m *syncRWMutexExt) RUnlock() {}

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
	defer chainedDataMu.RUnlock()

	if root, exists := chainedData[rootKey]; exists {
		if val, ok := root[path]; ok {
			return ctx.WriteBulkString(val)
		}
	}

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

var (
	reactiveWatchers   = make(map[string][]string)
	reactiveWatchersMu syncRWMutexExt
)

func cmdREACTIVEWATCH(ctx *Context) error {
	if ctx.ArgCount() < 2 {
		return ctx.WriteError(ErrWrongArgCount)
	}

	key := ctx.ArgString(0)
	callback := ctx.ArgString(1)

	reactiveWatchersMu.Lock()
	defer reactiveWatchersMu.Unlock()

	reactiveWatchers[key] = append(reactiveWatchers[key], callback)

	return ctx.WriteInteger(int64(len(reactiveWatchers[key])))
}

func cmdREACTIVEUNWATCH(ctx *Context) error {
	if ctx.ArgCount() < 2 {
		return ctx.WriteError(ErrWrongArgCount)
	}

	key := ctx.ArgString(0)
	callback := ctx.ArgString(1)

	reactiveWatchersMu.Lock()
	defer reactiveWatchersMu.Unlock()

	if watchers, exists := reactiveWatchers[key]; exists {
		for i, w := range watchers {
			if w == callback {
				reactiveWatchers[key] = append(watchers[:i], watchers[i+1:]...)
				return ctx.WriteInteger(1)
			}
		}
	}

	return ctx.WriteInteger(0)
}

func cmdREACTIVETRIGGER(ctx *Context) error {
	if ctx.ArgCount() < 1 {
		return ctx.WriteError(ErrWrongArgCount)
	}

	key := ctx.ArgString(0)

	reactiveWatchersMu.RLock()
	watchers := reactiveWatchers[key]
	reactiveWatchersMu.RUnlock()

	return ctx.WriteInteger(int64(len(watchers)))
}
