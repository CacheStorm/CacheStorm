package command

import (
	"errors"
	"fmt"
	"math"
	"regexp"
	"sort"
	"strconv"
	"sync"
	"time"

	"github.com/cachestorm/cachestorm/internal/resp"
)

func RegisterAdvancedCommands2(router *Router) {

	router.Register(&CommandDef{Name: "VALIDATE.CREATE", Handler: cmdVALIDATECREATE})
	router.Register(&CommandDef{Name: "VALIDATE.DELETE", Handler: cmdVALIDATEDELETE})
	router.Register(&CommandDef{Name: "VALIDATE.CHECK", Handler: cmdVALIDATECHECK})
	router.Register(&CommandDef{Name: "VALIDATE.LIST", Handler: cmdVALIDATELIST})

	router.Register(&CommandDef{Name: "STAGE.CREATE", Handler: cmdSTAGECREATE})
	router.Register(&CommandDef{Name: "STAGE.DELETE", Handler: cmdSTAGEDELETE})
	router.Register(&CommandDef{Name: "STAGE.NEXT", Handler: cmdSTAGENEXT})
	router.Register(&CommandDef{Name: "STAGE.PREV", Handler: cmdSTAGEPREV})
	router.Register(&CommandDef{Name: "STAGE.LIST", Handler: cmdSTAGELIST})

	router.Register(&CommandDef{Name: "CONTEXT.CREATE", Handler: cmdCONTEXTCREATE})
	router.Register(&CommandDef{Name: "CONTEXT.DELETE", Handler: cmdCONTEXTDELETE})
	router.Register(&CommandDef{Name: "CONTEXT.SET", Handler: cmdCONTEXTSET})
	router.Register(&CommandDef{Name: "CONTEXT.GET", Handler: cmdCONTEXTGET})
	router.Register(&CommandDef{Name: "CONTEXT.LIST", Handler: cmdCONTEXTLIST})

	router.Register(&CommandDef{Name: "PERMIT.GRANT", Handler: cmdPERMITGRANT})
	router.Register(&CommandDef{Name: "PERMIT.REVOKE", Handler: cmdPERMITREVOKE})
	router.Register(&CommandDef{Name: "PERMIT.CHECK", Handler: cmdPERMITCHECK})
	router.Register(&CommandDef{Name: "PERMIT.LIST", Handler: cmdPERMITLIST})

	router.Register(&CommandDef{Name: "GRANT.CREATE", Handler: cmdGRANTCREATE})
	router.Register(&CommandDef{Name: "GRANT.DELETE", Handler: cmdGRANTDELETE})
	router.Register(&CommandDef{Name: "GRANT.CHECK", Handler: cmdGRANTCHECK})
	router.Register(&CommandDef{Name: "GRANT.LIST", Handler: cmdGRANTLIST})

	router.Register(&CommandDef{Name: "TIMER.CREATE", Handler: cmdTIMERCREATE})
	router.Register(&CommandDef{Name: "TIMER.DELETE", Handler: cmdTIMERDELETE})
	router.Register(&CommandDef{Name: "TIMER.STATUS", Handler: cmdTIMERSTATUS})
	router.Register(&CommandDef{Name: "TIMER.LIST", Handler: cmdTIMERLIST})

	router.Register(&CommandDef{Name: "COUNTERX2.CREATE", Handler: cmdCOUNTERX2CREATE})
	router.Register(&CommandDef{Name: "COUNTERX2.INCR", Handler: cmdCOUNTERX2INCR})
	router.Register(&CommandDef{Name: "COUNTERX2.DECR", Handler: cmdCOUNTERX2DECR})
	router.Register(&CommandDef{Name: "COUNTERX2.GET", Handler: cmdCOUNTERX2GET})
	router.Register(&CommandDef{Name: "COUNTERX2.LIST", Handler: cmdCOUNTERX2LIST})

	router.Register(&CommandDef{Name: "LEVEL.CREATE", Handler: cmdLEVELCREATE})
	router.Register(&CommandDef{Name: "LEVEL.DELETE", Handler: cmdLEVELDELETE})
	router.Register(&CommandDef{Name: "LEVEL.SET", Handler: cmdLEVELSET})
	router.Register(&CommandDef{Name: "LEVEL.GET", Handler: cmdLEVELGET})
	router.Register(&CommandDef{Name: "LEVEL.LIST", Handler: cmdLEVELLIST})

	router.Register(&CommandDef{Name: "RECORD.CREATE", Handler: cmdRECORDCREATE})
	router.Register(&CommandDef{Name: "RECORD.ADD", Handler: cmdRECORDADD})
	router.Register(&CommandDef{Name: "RECORD.GET", Handler: cmdRECORDGET})
	router.Register(&CommandDef{Name: "RECORD.DELETE", Handler: cmdRECORDDELETE})

	router.Register(&CommandDef{Name: "ENTITY.CREATE", Handler: cmdENTITYCREATE})
	router.Register(&CommandDef{Name: "ENTITY.DELETE", Handler: cmdENTITYDELETE})
	router.Register(&CommandDef{Name: "ENTITY.GET", Handler: cmdENTITYGET})
	router.Register(&CommandDef{Name: "ENTITY.SET", Handler: cmdENTITYSET})
	router.Register(&CommandDef{Name: "ENTITY.LIST", Handler: cmdENTITYLIST})

	router.Register(&CommandDef{Name: "RELATION.CREATE", Handler: cmdRELATIONCREATE})
	router.Register(&CommandDef{Name: "RELATION.DELETE", Handler: cmdRELATIONDELETE})
	router.Register(&CommandDef{Name: "RELATION.GET", Handler: cmdRELATIONGET})
	router.Register(&CommandDef{Name: "RELATION.LIST", Handler: cmdRELATIONLIST})

	router.Register(&CommandDef{Name: "POOLX.CREATE", Handler: cmdPOOLXCREATE})
	router.Register(&CommandDef{Name: "POOLX.DELETE", Handler: cmdPOOLXDELETE})
	router.Register(&CommandDef{Name: "POOLX.ACQUIRE", Handler: cmdPOOLXACQUIRE})
	router.Register(&CommandDef{Name: "POOLX.RELEASE", Handler: cmdPOOLXRELEASE})
	router.Register(&CommandDef{Name: "POOLX.STATUS", Handler: cmdPOOLXSTATUS})

	router.Register(&CommandDef{Name: "BUFFERX.CREATE", Handler: cmdBUFFERXCREATE})
	router.Register(&CommandDef{Name: "BUFFERX.WRITE", Handler: cmdBUFFERXWRITE})
	router.Register(&CommandDef{Name: "BUFFERX.READ", Handler: cmdBUFFERXREAD})
	router.Register(&CommandDef{Name: "BUFFERX.DELETE", Handler: cmdBUFFERXDELETE})

	router.Register(&CommandDef{Name: "STREAMX.CREATE", Handler: cmdSTREAMXCREATE})
	router.Register(&CommandDef{Name: "STREAMX.WRITE", Handler: cmdSTREAMXWRITE})
	router.Register(&CommandDef{Name: "STREAMX.READ", Handler: cmdSTREAMXREAD})
	router.Register(&CommandDef{Name: "STREAMX.DELETE", Handler: cmdSTREAMXDELETE})

	router.Register(&CommandDef{Name: "CACHEX.CREATE", Handler: cmdCACHEXCREATE})
	router.Register(&CommandDef{Name: "CACHEX.DELETE", Handler: cmdCACHEXDELETE})
	router.Register(&CommandDef{Name: "CACHEX.GET", Handler: cmdCACHEXGET})
	router.Register(&CommandDef{Name: "CACHEX.SET", Handler: cmdCACHEXSET})
	router.Register(&CommandDef{Name: "CACHEX.LIST", Handler: cmdCACHEXLIST})

	router.Register(&CommandDef{Name: "STOREX.CREATE", Handler: cmdSTOREXCREATE})
	router.Register(&CommandDef{Name: "STOREX.DELETE", Handler: cmdSTOREXDELETE})
	router.Register(&CommandDef{Name: "STOREX.PUT", Handler: cmdSTOREXPUT})
	router.Register(&CommandDef{Name: "STOREX.GET", Handler: cmdSTOREXGET})
	router.Register(&CommandDef{Name: "STOREX.LIST", Handler: cmdSTOREXLIST})

	router.Register(&CommandDef{Name: "INDEX.CREATE", Handler: cmdINDEXCREATE})
	router.Register(&CommandDef{Name: "INDEX.DELETE", Handler: cmdINDEXDELETE})
	router.Register(&CommandDef{Name: "INDEX.ADD", Handler: cmdINDEXADD})
	router.Register(&CommandDef{Name: "INDEX.SEARCH", Handler: cmdINDEXSEARCH})
	router.Register(&CommandDef{Name: "INDEX.LIST", Handler: cmdINDEXLIST})

	router.Register(&CommandDef{Name: "VIEW.CREATE", Handler: cmdVIEWCREATE})
	router.Register(&CommandDef{Name: "VIEW.DELETE", Handler: cmdVIEWDELETE})
	router.Register(&CommandDef{Name: "VIEW.GET", Handler: cmdVIEWGET})
	router.Register(&CommandDef{Name: "VIEW.LIST", Handler: cmdVIEWLIST})

	router.Register(&CommandDef{Name: "AUDITX.LOG", Handler: cmdAUDITXLOG})
	router.Register(&CommandDef{Name: "AUDITX.GET", Handler: cmdAUDITXGET})
	router.Register(&CommandDef{Name: "AUDITX.SEARCH", Handler: cmdAUDITXSEARCH})
	router.Register(&CommandDef{Name: "AUDITX.LIST", Handler: cmdAUDITXLIST})

	router.Register(&CommandDef{Name: "TOKEN.CREATE", Handler: cmdTOKENCREATE})
	router.Register(&CommandDef{Name: "TOKEN.DELETE", Handler: cmdTOKENDELETE})
	router.Register(&CommandDef{Name: "TOKEN.VALIDATE", Handler: cmdTOKENVALIDATE})
	router.Register(&CommandDef{Name: "TOKEN.REFRESH", Handler: cmdTOKENREFRESH})
	router.Register(&CommandDef{Name: "TOKEN.LIST", Handler: cmdTOKENLIST})

	router.Register(&CommandDef{Name: "SESSIONX.CREATE", Handler: cmdSESSIONXCREATE})
	router.Register(&CommandDef{Name: "SESSIONX.DELETE", Handler: cmdSESSIONXDELETE})
	router.Register(&CommandDef{Name: "SESSIONX.GET", Handler: cmdSESSIONXGET})
	router.Register(&CommandDef{Name: "SESSIONX.SET", Handler: cmdSESSIONXSET})
	router.Register(&CommandDef{Name: "SESSIONX.LIST", Handler: cmdSESSIONXLIST})

	router.Register(&CommandDef{Name: "PROFILE.CREATE", Handler: cmdPROFILECREATE})
	router.Register(&CommandDef{Name: "PROFILE.DELETE", Handler: cmdPROFILEDELETE})
	router.Register(&CommandDef{Name: "PROFILE.GET", Handler: cmdPROFILEGET})
	router.Register(&CommandDef{Name: "PROFILE.SET", Handler: cmdPROFILESET})
	router.Register(&CommandDef{Name: "PROFILE.LIST", Handler: cmdPROFILELIST})

}

var (
	validators   = make(map[string]*regexp.Regexp)
	validatorsMu sync.RWMutex
)

func cmdVALIDATECREATE(ctx *Context) error {
	if ctx.ArgCount() < 2 {
		return ctx.WriteError(ErrWrongArgCount)
	}
	name := ctx.ArgString(0)
	rule, err := regexp.Compile(ctx.ArgString(1))
	if err != nil {
		return ctx.WriteError(fmt.Errorf("ERR invalid validation pattern: %v", err))
	}
	validatorsMu.Lock()
	validators[name] = rule
	validatorsMu.Unlock()
	return ctx.WriteOK()
}

func cmdVALIDATEDELETE(ctx *Context) error {
	if ctx.ArgCount() < 1 {
		return ctx.WriteError(ErrWrongArgCount)
	}
	name := ctx.ArgString(0)
	validatorsMu.Lock()
	defer validatorsMu.Unlock()
	if _, exists := validators[name]; !exists {
		return ctx.WriteInteger(0)
	}
	delete(validators, name)
	return ctx.WriteInteger(1)
}

func cmdVALIDATECHECK(ctx *Context) error {
	if ctx.ArgCount() < 2 {
		return ctx.WriteError(ErrWrongArgCount)
	}
	name := ctx.ArgString(0)
	value := ctx.ArgString(1)
	validatorsMu.RLock()
	rule, exists := validators[name]
	validatorsMu.RUnlock()
	if !exists {
		return ctx.WriteError(fmt.Errorf("ERR validator not found"))
	}
	if rule.MatchString(value) {
		return ctx.WriteInteger(1)
	}
	return ctx.WriteInteger(0)
}

func cmdVALIDATELIST(ctx *Context) error {
	validatorsMu.RLock()
	results := make([]*resp.Value, 0)
	for name := range validators {
		results = append(results, resp.BulkString(name))
	}
	reply := results
	validatorsMu.RUnlock()
	return ctx.WriteArray(reply)
}

var (
	stages   = make(map[string]*Stage)
	stagesMu sync.RWMutex
)

type Stage struct {
	Name    string
	Current int
	Total   int
}

func cmdSTAGECREATE(ctx *Context) error {
	if ctx.ArgCount() < 2 {
		return ctx.WriteError(ErrWrongArgCount)
	}
	name := ctx.ArgString(0)
	total := int(parseInt64(ctx.ArgString(1)))
	stagesMu.Lock()
	stages[name] = &Stage{Name: name, Current: 0, Total: total}
	stagesMu.Unlock()
	return ctx.WriteOK()
}

func cmdSTAGEDELETE(ctx *Context) error {
	if ctx.ArgCount() < 1 {
		return ctx.WriteError(ErrWrongArgCount)
	}
	name := ctx.ArgString(0)
	stagesMu.Lock()
	defer stagesMu.Unlock()
	if _, exists := stages[name]; !exists {
		return ctx.WriteInteger(0)
	}
	delete(stages, name)
	return ctx.WriteInteger(1)
}

func cmdSTAGENEXT(ctx *Context) error {
	if ctx.ArgCount() < 1 {
		return ctx.WriteError(ErrWrongArgCount)
	}
	name := ctx.ArgString(0)
	stagesMu.Lock()
	defer stagesMu.Unlock()
	if stage, exists := stages[name]; exists {
		if stage.Current < stage.Total {
			stage.Current++
		}
		return ctx.WriteInteger(int64(stage.Current))
	}
	return ctx.WriteError(fmt.Errorf("ERR stage not found"))
}

func cmdSTAGEPREV(ctx *Context) error {
	if ctx.ArgCount() < 1 {
		return ctx.WriteError(ErrWrongArgCount)
	}
	name := ctx.ArgString(0)
	stagesMu.Lock()
	defer stagesMu.Unlock()
	if stage, exists := stages[name]; exists {
		if stage.Current > 0 {
			stage.Current--
		}
		return ctx.WriteInteger(int64(stage.Current))
	}
	return ctx.WriteError(fmt.Errorf("ERR stage not found"))
}

func cmdSTAGELIST(ctx *Context) error {
	stagesMu.RLock()
	results := make([]*resp.Value, 0)
	for name, stage := range stages {
		results = append(results, resp.ArrayValue([]*resp.Value{
			resp.BulkString("name"), resp.BulkString(name),
			resp.BulkString("current"), resp.IntegerValue(int64(stage.Current)),
			resp.BulkString("total"), resp.IntegerValue(int64(stage.Total)),
		}))
	}
	reply := results
	stagesMu.RUnlock()
	return ctx.WriteArray(reply)
}

var (
	contexts   = make(map[string]map[string]string)
	contextsMu sync.RWMutex
)

func cmdCONTEXTCREATE(ctx *Context) error {
	if ctx.ArgCount() < 1 {
		return ctx.WriteError(ErrWrongArgCount)
	}
	name := ctx.ArgString(0)
	contextsMu.Lock()
	contexts[name] = make(map[string]string)
	contextsMu.Unlock()
	return ctx.WriteOK()
}

func cmdCONTEXTDELETE(ctx *Context) error {
	if ctx.ArgCount() < 1 {
		return ctx.WriteError(ErrWrongArgCount)
	}
	name := ctx.ArgString(0)
	contextsMu.Lock()
	defer contextsMu.Unlock()
	if _, exists := contexts[name]; !exists {
		return ctx.WriteInteger(0)
	}
	delete(contexts, name)
	return ctx.WriteInteger(1)
}

func cmdCONTEXTSET(ctx *Context) error {
	if ctx.ArgCount() < 3 {
		return ctx.WriteError(ErrWrongArgCount)
	}
	name := ctx.ArgString(0)
	key := ctx.ArgString(1)
	value := ctx.ArgString(2)
	contextsMu.Lock()
	defer contextsMu.Unlock()
	if c, exists := contexts[name]; exists {
		c[key] = value
		return ctx.WriteOK()
	}
	return ctx.WriteError(fmt.Errorf("ERR context not found"))
}

func cmdCONTEXTGET(ctx *Context) error {
	if ctx.ArgCount() < 2 {
		return ctx.WriteError(ErrWrongArgCount)
	}
	name := ctx.ArgString(0)
	key := ctx.ArgString(1)
	contextsMu.RLock()
	if c, exists := contexts[name]; exists {
		if val, ok := c[key]; ok {
			reply := val
			contextsMu.RUnlock()
			return ctx.WriteBulkString(reply)
		}
	}
	contextsMu.RUnlock()
	return ctx.WriteNull()
}

func cmdCONTEXTLIST(ctx *Context) error {
	contextsMu.RLock()
	results := make([]*resp.Value, 0)
	for name := range contexts {
		results = append(results, resp.BulkString(name))
	}
	reply := results
	contextsMu.RUnlock()
	return ctx.WriteArray(reply)
}

var (
	permits   = make(map[string]map[string]bool)
	permitsMu sync.RWMutex
)

func cmdPERMITGRANT(ctx *Context) error {
	if ctx.ArgCount() < 3 {
		return ctx.WriteError(ErrWrongArgCount)
	}
	user := ctx.ArgString(0)
	resource := ctx.ArgString(1)
	action := ctx.ArgString(2)
	permitsMu.Lock()
	defer permitsMu.Unlock()
	key := user + ":" + resource + ":" + action
	if _, exists := permits[user]; !exists {
		permits[user] = make(map[string]bool)
	}
	permits[user][key] = true
	return ctx.WriteOK()
}

func cmdPERMITREVOKE(ctx *Context) error {
	if ctx.ArgCount() < 3 {
		return ctx.WriteError(ErrWrongArgCount)
	}
	user := ctx.ArgString(0)
	resource := ctx.ArgString(1)
	action := ctx.ArgString(2)
	permitsMu.Lock()
	defer permitsMu.Unlock()
	key := user + ":" + resource + ":" + action
	if p, exists := permits[user]; exists {
		if _, granted := p[key]; granted {
			delete(p, key)
			return ctx.WriteInteger(1)
		}
	}
	return ctx.WriteInteger(0)
}

func cmdPERMITCHECK(ctx *Context) error {
	if ctx.ArgCount() < 3 {
		return ctx.WriteError(ErrWrongArgCount)
	}
	user := ctx.ArgString(0)
	resource := ctx.ArgString(1)
	action := ctx.ArgString(2)
	permitsMu.RLock()
	key := user + ":" + resource + ":" + action
	if p, exists := permits[user]; exists {
		if p[key] {
			reply := int64(1)
			permitsMu.RUnlock()
			return ctx.WriteInteger(reply)
		}
	}
	reply := int64(0)
	permitsMu.RUnlock()
	return ctx.WriteInteger(reply)
}

func cmdPERMITLIST(ctx *Context) error {
	if ctx.ArgCount() < 1 {
		return ctx.WriteError(ErrWrongArgCount)
	}
	user := ctx.ArgString(0)
	permitsMu.RLock()
	results := make([]*resp.Value, 0)
	if p, exists := permits[user]; exists {
		for k := range p {
			results = append(results, resp.BulkString(k))
		}
	}
	reply := results
	permitsMu.RUnlock()
	return ctx.WriteArray(reply)
}

var (
	grants   = make(map[string]*Grant)
	grantsMu sync.RWMutex
)

type Grant struct {
	ID       string
	User     string
	Resource string
	Actions  []string
}

func cmdGRANTCREATE(ctx *Context) error {
	if ctx.ArgCount() < 3 {
		return ctx.WriteError(ErrWrongArgCount)
	}
	user := ctx.ArgString(0)
	resource := ctx.ArgString(1)
	actions := make([]string, 0)
	for i := 2; i < ctx.ArgCount(); i++ {
		actions = append(actions, ctx.ArgString(i))
	}
	id := generateUUID()
	grantsMu.Lock()
	grants[id] = &Grant{ID: id, User: user, Resource: resource, Actions: actions}
	grantsMu.Unlock()
	return ctx.WriteBulkString(id)
}

func cmdGRANTDELETE(ctx *Context) error {
	if ctx.ArgCount() < 1 {
		return ctx.WriteError(ErrWrongArgCount)
	}
	id := ctx.ArgString(0)
	grantsMu.Lock()
	defer grantsMu.Unlock()
	if _, exists := grants[id]; !exists {
		return ctx.WriteInteger(0)
	}
	delete(grants, id)
	return ctx.WriteInteger(1)
}

func cmdGRANTCHECK(ctx *Context) error {
	if ctx.ArgCount() < 3 {
		return ctx.WriteError(ErrWrongArgCount)
	}
	user := ctx.ArgString(0)
	resource := ctx.ArgString(1)
	action := ctx.ArgString(2)
	grantsMu.RLock()
	for _, g := range grants {
		if g.User == user && g.Resource == resource {
			for _, a := range g.Actions {
				if a == action || a == "*" {
					reply := int64(1)
					grantsMu.RUnlock()
					return ctx.WriteInteger(reply)
				}
			}
		}
	}
	reply := int64(0)
	grantsMu.RUnlock()
	return ctx.WriteInteger(reply)
}

func cmdGRANTLIST(ctx *Context) error {
	grantsMu.RLock()
	results := make([]*resp.Value, 0)
	for id, g := range grants {
		results = append(results, resp.ArrayValue([]*resp.Value{
			resp.BulkString("id"), resp.BulkString(id),
			resp.BulkString("user"), resp.BulkString(g.User),
			resp.BulkString("resource"), resp.BulkString(g.Resource),
		}))
	}
	reply := results
	grantsMu.RUnlock()
	return ctx.WriteArray(reply)
}

var (
	timers   = make(map[string]*Timer)
	timersMu sync.RWMutex
)

type Timer struct {
	ID        string
	Name      string
	Duration  int64
	StartTime int64
	Running   bool
}

func cmdTIMERCREATE(ctx *Context) error {
	if ctx.ArgCount() < 2 {
		return ctx.WriteError(ErrWrongArgCount)
	}
	name := ctx.ArgString(0)
	duration := parseInt64(ctx.ArgString(1))
	id := generateUUID()
	timersMu.Lock()
	timers[id] = &Timer{ID: id, Name: name, Duration: duration, StartTime: time.Now().UnixMilli(), Running: true}
	timersMu.Unlock()
	return ctx.WriteBulkString(id)
}

func cmdTIMERDELETE(ctx *Context) error {
	if ctx.ArgCount() < 1 {
		return ctx.WriteError(ErrWrongArgCount)
	}
	id := ctx.ArgString(0)
	timersMu.Lock()
	defer timersMu.Unlock()
	if _, exists := timers[id]; !exists {
		return ctx.WriteInteger(0)
	}
	delete(timers, id)
	return ctx.WriteInteger(1)
}

func cmdTIMERSTATUS(ctx *Context) error {
	if ctx.ArgCount() < 1 {
		return ctx.WriteError(ErrWrongArgCount)
	}
	id := ctx.ArgString(0)
	timersMu.RLock()
	timer, exists := timers[id]
	if !exists {
		timersMu.RUnlock()
		return ctx.WriteError(fmt.Errorf("ERR timer not found"))
	}
	elapsed := time.Now().UnixMilli() - timer.StartTime
	remaining := timer.Duration - elapsed
	if remaining < 0 {
		remaining = 0
	}
	running := timer.Running && remaining > 0
	timerID := timer.ID
	timersMu.RUnlock()
	return ctx.WriteArray([]*resp.Value{
		resp.BulkString("id"), resp.BulkString(timerID),
		resp.BulkString("running"), resp.BulkString(fmt.Sprintf("%v", running)),
		resp.BulkString("remaining_ms"), resp.IntegerValue(remaining),
	})
}

func cmdTIMERLIST(ctx *Context) error {
	timersMu.RLock()
	results := make([]*resp.Value, 0)
	for id := range timers {
		results = append(results, resp.BulkString(id))
	}
	reply := results
	timersMu.RUnlock()
	return ctx.WriteArray(reply)
}

var (
	countersX3   = make(map[string]int64)
	countersX3Mu sync.RWMutex
)

func cmdCOUNTERX2CREATE(ctx *Context) error {
	if ctx.ArgCount() < 1 {
		return ctx.WriteError(ErrWrongArgCount)
	}
	name := ctx.ArgString(0)
	initVal := int64(0)
	if ctx.ArgCount() >= 2 {
		initVal = parseInt64(ctx.ArgString(1))
	}
	countersX3Mu.Lock()
	countersX3[name] = initVal
	countersX3Mu.Unlock()
	return ctx.WriteOK()
}

func cmdCOUNTERX2INCR(ctx *Context) error {
	if ctx.ArgCount() < 1 {
		return ctx.WriteError(ErrWrongArgCount)
	}
	name := ctx.ArgString(0)
	incr := int64(1)
	if ctx.ArgCount() >= 2 {
		incr = parseInt64(ctx.ArgString(1))
	}
	countersX3Mu.Lock()
	defer countersX3Mu.Unlock()
	countersX3[name] += incr
	return ctx.WriteInteger(countersX3[name])
}

func cmdCOUNTERX2DECR(ctx *Context) error {
	if ctx.ArgCount() < 1 {
		return ctx.WriteError(ErrWrongArgCount)
	}
	name := ctx.ArgString(0)
	decr := int64(1)
	if ctx.ArgCount() >= 2 {
		decr = parseInt64(ctx.ArgString(1))
	}
	countersX3Mu.Lock()
	defer countersX3Mu.Unlock()
	countersX3[name] -= decr
	return ctx.WriteInteger(countersX3[name])
}

func cmdCOUNTERX2GET(ctx *Context) error {
	if ctx.ArgCount() < 1 {
		return ctx.WriteError(ErrWrongArgCount)
	}
	name := ctx.ArgString(0)
	countersX3Mu.RLock()
	val, exists := countersX3[name]
	countersX3Mu.RUnlock()
	if !exists {
		return ctx.WriteInteger(0)
	}
	return ctx.WriteInteger(val)
}

func cmdCOUNTERX2LIST(ctx *Context) error {
	countersX3Mu.RLock()
	results := make([]*resp.Value, 0)
	for name, val := range countersX3 {
		results = append(results, resp.ArrayValue([]*resp.Value{
			resp.BulkString("name"), resp.BulkString(name),
			resp.BulkString("value"), resp.IntegerValue(val),
		}))
	}
	reply := results
	countersX3Mu.RUnlock()
	return ctx.WriteArray(reply)
}

var (
	levels   = make(map[string]*LevelGauge)
	levelsMu sync.RWMutex
)

// LevelGauge is a named level bounded by [0, Max].
type LevelGauge struct {
	Current int64
	Max     int64
}

var errLevelNotFound = errors.New("ERR level not found")

func cmdLEVELCREATE(ctx *Context) error {
	if ctx.ArgCount() != 2 {
		return ctx.WriteError(ErrWrongArgCount)
	}
	name := ctx.ArgString(0)
	maxLevel, err := strconv.ParseInt(ctx.ArgString(1), 10, 64)
	if err != nil || maxLevel < 0 {
		return ctx.WriteError(fmt.Errorf("ERR max level must be a non-negative integer"))
	}
	levelsMu.Lock()
	levels[name] = &LevelGauge{Max: maxLevel}
	levelsMu.Unlock()
	return ctx.WriteOK()
}

func cmdLEVELDELETE(ctx *Context) error {
	if ctx.ArgCount() != 1 {
		return ctx.WriteError(ErrWrongArgCount)
	}
	name := ctx.ArgString(0)
	levelsMu.Lock()
	_, exists := levels[name]
	delete(levels, name)
	levelsMu.Unlock()
	if !exists {
		return ctx.WriteInteger(0)
	}
	return ctx.WriteInteger(1)
}

func cmdLEVELSET(ctx *Context) error {
	if ctx.ArgCount() != 2 {
		return ctx.WriteError(ErrWrongArgCount)
	}
	name := ctx.ArgString(0)
	level, err := strconv.ParseInt(ctx.ArgString(1), 10, 64)
	if err != nil {
		return ctx.WriteError(ErrNotInteger)
	}
	levelsMu.Lock()
	gauge, exists := levels[name]
	if !exists {
		levelsMu.Unlock()
		return ctx.WriteError(errLevelNotFound)
	}
	if level < 0 || level > gauge.Max {
		max := gauge.Max
		levelsMu.Unlock()
		return ctx.WriteError(fmt.Errorf("ERR level must be between 0 and %d", max))
	}
	gauge.Current = level
	levelsMu.Unlock()
	return ctx.WriteOK()
}

func cmdLEVELGET(ctx *Context) error {
	if ctx.ArgCount() != 1 {
		return ctx.WriteError(ErrWrongArgCount)
	}
	name := ctx.ArgString(0)
	levelsMu.RLock()
	gauge, exists := levels[name]
	var current, max int64
	if exists {
		current, max = gauge.Current, gauge.Max
	}
	levelsMu.RUnlock()
	if !exists {
		return ctx.WriteError(errLevelNotFound)
	}
	return ctx.WriteArray([]*resp.Value{
		resp.BulkString("current"), resp.IntegerValue(current),
		resp.BulkString("max"), resp.IntegerValue(max),
	})
}

func cmdLEVELLIST(ctx *Context) error {
	levelsMu.RLock()
	names := make([]string, 0, len(levels))
	for name := range levels {
		names = append(names, name)
	}
	levelsMu.RUnlock()
	sort.Strings(names)
	results := make([]*resp.Value, len(names))
	for i, name := range names {
		results[i] = resp.BulkString(name)
	}
	return ctx.WriteArray(results)
}

var (
	records   = make(map[string]*Record)
	recordsMu sync.RWMutex
)

type Record struct {
	ID     string
	Name   string
	Fields map[string]string
}

func cmdRECORDCREATE(ctx *Context) error {
	if ctx.ArgCount() < 1 {
		return ctx.WriteError(ErrWrongArgCount)
	}
	name := ctx.ArgString(0)
	id := generateUUID()
	recordsMu.Lock()
	records[id] = &Record{ID: id, Name: name, Fields: make(map[string]string)}
	recordsMu.Unlock()
	return ctx.WriteBulkString(id)
}

func cmdRECORDADD(ctx *Context) error {
	if ctx.ArgCount() < 3 {
		return ctx.WriteError(ErrWrongArgCount)
	}
	id := ctx.ArgString(0)
	key := ctx.ArgString(1)
	value := ctx.ArgString(2)
	recordsMu.Lock()
	defer recordsMu.Unlock()
	if r, exists := records[id]; exists {
		r.Fields[key] = value
		return ctx.WriteOK()
	}
	return ctx.WriteError(fmt.Errorf("ERR record not found"))
}

func cmdRECORDGET(ctx *Context) error {
	if ctx.ArgCount() < 1 {
		return ctx.WriteError(ErrWrongArgCount)
	}
	id := ctx.ArgString(0)
	recordsMu.RLock()
	r, exists := records[id]
	if !exists {
		reply := fmt.Errorf("ERR record not found")
		recordsMu.RUnlock()
		return ctx.WriteError(reply)
	}
	results := make([]*resp.Value, 0)
	results = append(results, resp.BulkString("id"), resp.BulkString(r.ID), resp.BulkString("name"), resp.BulkString(r.Name))
	for k, v := range r.Fields {
		results = append(results, resp.BulkString(k), resp.BulkString(v))
	}
	reply := results
	recordsMu.RUnlock()
	return ctx.WriteArray(reply)
}

func cmdRECORDDELETE(ctx *Context) error {
	if ctx.ArgCount() < 1 {
		return ctx.WriteError(ErrWrongArgCount)
	}
	id := ctx.ArgString(0)
	recordsMu.Lock()
	defer recordsMu.Unlock()
	if _, exists := records[id]; !exists {
		return ctx.WriteInteger(0)
	}
	delete(records, id)
	return ctx.WriteInteger(1)
}

var (
	entities   = make(map[string]*Entity)
	entitiesMu sync.RWMutex
)

type Entity struct {
	ID         string
	Type       string
	Attributes map[string]string
}

func cmdENTITYCREATE(ctx *Context) error {
	if ctx.ArgCount() < 2 {
		return ctx.WriteError(ErrWrongArgCount)
	}
	id := ctx.ArgString(0)
	entityType := ctx.ArgString(1)
	entitiesMu.Lock()
	entities[id] = &Entity{ID: id, Type: entityType, Attributes: make(map[string]string)}
	entitiesMu.Unlock()
	return ctx.WriteOK()
}

func cmdENTITYDELETE(ctx *Context) error {
	if ctx.ArgCount() < 1 {
		return ctx.WriteError(ErrWrongArgCount)
	}
	id := ctx.ArgString(0)
	entitiesMu.Lock()
	defer entitiesMu.Unlock()
	if _, exists := entities[id]; !exists {
		return ctx.WriteInteger(0)
	}
	delete(entities, id)
	return ctx.WriteInteger(1)
}

func cmdENTITYGET(ctx *Context) error {
	if ctx.ArgCount() < 1 {
		return ctx.WriteError(ErrWrongArgCount)
	}
	id := ctx.ArgString(0)
	entitiesMu.RLock()
	e, exists := entities[id]
	if !exists {
		reply := fmt.Errorf("ERR entity not found")
		entitiesMu.RUnlock()
		return ctx.WriteError(reply)
	}
	results := make([]*resp.Value, 0)
	results = append(results, resp.BulkString("id"), resp.BulkString(e.ID), resp.BulkString("type"), resp.BulkString(e.Type))
	for k, v := range e.Attributes {
		results = append(results, resp.BulkString(k), resp.BulkString(v))
	}
	reply := results
	entitiesMu.RUnlock()
	return ctx.WriteArray(reply)
}

func cmdENTITYSET(ctx *Context) error {
	if ctx.ArgCount() < 3 {
		return ctx.WriteError(ErrWrongArgCount)
	}
	id := ctx.ArgString(0)
	key := ctx.ArgString(1)
	value := ctx.ArgString(2)
	entitiesMu.Lock()
	defer entitiesMu.Unlock()
	if e, exists := entities[id]; exists {
		e.Attributes[key] = value
		return ctx.WriteOK()
	}
	return ctx.WriteError(fmt.Errorf("ERR entity not found"))
}

func cmdENTITYLIST(ctx *Context) error {
	entitiesMu.RLock()
	results := make([]*resp.Value, 0)
	for id := range entities {
		results = append(results, resp.BulkString(id))
	}
	reply := results
	entitiesMu.RUnlock()
	return ctx.WriteArray(reply)
}

var (
	relations   = make(map[string]*Relation)
	relationsMu sync.RWMutex
)

type Relation struct {
	ID       string
	From     string
	To       string
	Type     string
	Metadata map[string]string
}

func cmdRELATIONCREATE(ctx *Context) error {
	if ctx.ArgCount() < 3 {
		return ctx.WriteError(ErrWrongArgCount)
	}
	from := ctx.ArgString(0)
	to := ctx.ArgString(1)
	relType := ctx.ArgString(2)
	id := generateUUID()
	relationsMu.Lock()
	relations[id] = &Relation{ID: id, From: from, To: to, Type: relType, Metadata: make(map[string]string)}
	relationsMu.Unlock()
	return ctx.WriteBulkString(id)
}

func cmdRELATIONDELETE(ctx *Context) error {
	if ctx.ArgCount() < 1 {
		return ctx.WriteError(ErrWrongArgCount)
	}
	id := ctx.ArgString(0)
	relationsMu.Lock()
	defer relationsMu.Unlock()
	if _, exists := relations[id]; !exists {
		return ctx.WriteInteger(0)
	}
	delete(relations, id)
	return ctx.WriteInteger(1)
}

func cmdRELATIONGET(ctx *Context) error {
	if ctx.ArgCount() < 1 {
		return ctx.WriteError(ErrWrongArgCount)
	}
	id := ctx.ArgString(0)
	relationsMu.RLock()
	r, exists := relations[id]
	relationsMu.RUnlock()
	if !exists {
		return ctx.WriteError(fmt.Errorf("ERR relation not found"))
	}
	return ctx.WriteArray([]*resp.Value{
		resp.BulkString("id"), resp.BulkString(r.ID),
		resp.BulkString("from"), resp.BulkString(r.From),
		resp.BulkString("to"), resp.BulkString(r.To),
		resp.BulkString("type"), resp.BulkString(r.Type),
	})
}

func cmdRELATIONLIST(ctx *Context) error {
	relationsMu.RLock()
	results := make([]*resp.Value, 0)
	for id := range relations {
		results = append(results, resp.BulkString(id))
	}
	reply := results
	relationsMu.RUnlock()
	return ctx.WriteArray(reply)
}

var (
	poolsX    = make(map[string]*PoolX)
	poolsXMux sync.RWMutex
)

type PoolX struct {
	Name      string
	Size      int
	Available []string
	InUse     map[string]bool
}

func cmdPOOLXCREATE(ctx *Context) error {
	if ctx.ArgCount() < 2 {
		return ctx.WriteError(ErrWrongArgCount)
	}
	name := ctx.ArgString(0)
	size := int(parseInt64(ctx.ArgString(1)))
	available := make([]string, size)
	for i := 0; i < size; i++ {
		available[i] = fmt.Sprintf("resource-%d", i)
	}
	poolsXMux.Lock()
	poolsX[name] = &PoolX{Name: name, Size: size, Available: available, InUse: make(map[string]bool)}
	poolsXMux.Unlock()
	return ctx.WriteOK()
}

func cmdPOOLXDELETE(ctx *Context) error {
	if ctx.ArgCount() < 1 {
		return ctx.WriteError(ErrWrongArgCount)
	}
	name := ctx.ArgString(0)
	poolsXMux.Lock()
	defer poolsXMux.Unlock()
	if _, exists := poolsX[name]; !exists {
		return ctx.WriteInteger(0)
	}
	delete(poolsX, name)
	return ctx.WriteInteger(1)
}

func cmdPOOLXACQUIRE(ctx *Context) error {
	if ctx.ArgCount() < 1 {
		return ctx.WriteError(ErrWrongArgCount)
	}
	name := ctx.ArgString(0)
	poolsXMux.Lock()
	defer poolsXMux.Unlock()
	pool, exists := poolsX[name]
	if !exists {
		return ctx.WriteError(fmt.Errorf("ERR pool not found"))
	}
	if len(pool.Available) == 0 {
		return ctx.WriteNull()
	}
	resource := pool.Available[0]
	pool.Available = pool.Available[1:]
	pool.InUse[resource] = true
	return ctx.WriteBulkString(resource)
}

func cmdPOOLXRELEASE(ctx *Context) error {
	if ctx.ArgCount() < 2 {
		return ctx.WriteError(ErrWrongArgCount)
	}
	name := ctx.ArgString(0)
	resource := ctx.ArgString(1)
	poolsXMux.Lock()
	defer poolsXMux.Unlock()
	pool, exists := poolsX[name]
	if !exists {
		return ctx.WriteError(fmt.Errorf("ERR pool not found"))
	}
	if pool.InUse[resource] {
		delete(pool.InUse, resource)
		pool.Available = append(pool.Available, resource)
		return ctx.WriteInteger(1)
	}
	return ctx.WriteInteger(0)
}

func cmdPOOLXSTATUS(ctx *Context) error {
	if ctx.ArgCount() < 1 {
		return ctx.WriteError(ErrWrongArgCount)
	}
	name := ctx.ArgString(0)
	poolsXMux.RLock()
	pool, exists := poolsX[name]
	if !exists {
		reply := fmt.Errorf("ERR pool not found")
		poolsXMux.RUnlock()
		return ctx.WriteError(reply)
	}
	reply := []*resp.Value{
		resp.BulkString("name"), resp.BulkString(pool.Name),
		resp.BulkString("size"), resp.IntegerValue(int64(pool.Size)),
		resp.BulkString("available"), resp.IntegerValue(int64(len(pool.Available))),
		resp.BulkString("in_use"), resp.IntegerValue(int64(len(pool.InUse))),
	}
	poolsXMux.RUnlock()
	return ctx.WriteArray(reply)
}

var (
	buffersX    = make(map[string]*BufferX)
	buffersXMux sync.RWMutex
)

type BufferX struct {
	Name string
	Data []byte
}

func cmdBUFFERXCREATE(ctx *Context) error {
	if ctx.ArgCount() < 2 {
		return ctx.WriteError(ErrWrongArgCount)
	}
	name := ctx.ArgString(0)
	size := int(parseInt64(ctx.ArgString(1)))
	if size < 0 {
		return ctx.WriteError(fmt.Errorf("ERR size must not be negative"))
	}
	buffersXMux.Lock()
	buffersX[name] = &BufferX{Name: name, Data: make([]byte, 0, size)}
	buffersXMux.Unlock()
	return ctx.WriteOK()
}

func cmdBUFFERXWRITE(ctx *Context) error {
	if ctx.ArgCount() < 2 {
		return ctx.WriteError(ErrWrongArgCount)
	}
	name := ctx.ArgString(0)
	data := ctx.ArgString(1)
	buffersXMux.Lock()
	defer buffersXMux.Unlock()
	if b, exists := buffersX[name]; exists {
		b.Data = append(b.Data, []byte(data)...)
		return ctx.WriteOK()
	}
	return ctx.WriteError(fmt.Errorf("ERR buffer not found"))
}

func cmdBUFFERXREAD(ctx *Context) error {
	if ctx.ArgCount() < 1 {
		return ctx.WriteError(ErrWrongArgCount)
	}
	name := ctx.ArgString(0)
	buffersXMux.RLock()
	b, exists := buffersX[name]
	if !exists {
		buffersXMux.RUnlock()
		return ctx.WriteNull()
	}
	reply := string(b.Data)
	buffersXMux.RUnlock()
	return ctx.WriteBulkString(reply)
}

func cmdBUFFERXDELETE(ctx *Context) error {
	if ctx.ArgCount() < 1 {
		return ctx.WriteError(ErrWrongArgCount)
	}
	name := ctx.ArgString(0)
	buffersXMux.Lock()
	defer buffersXMux.Unlock()
	if _, exists := buffersX[name]; !exists {
		return ctx.WriteInteger(0)
	}
	delete(buffersX, name)
	return ctx.WriteInteger(1)
}

var (
	streamsX    = make(map[string]*StreamX)
	streamsXMux sync.RWMutex
)

type StreamX struct {
	Name string
	Data []string
}

func cmdSTREAMXCREATE(ctx *Context) error {
	if ctx.ArgCount() < 1 {
		return ctx.WriteError(ErrWrongArgCount)
	}
	name := ctx.ArgString(0)
	streamsXMux.Lock()
	streamsX[name] = &StreamX{Name: name, Data: make([]string, 0)}
	streamsXMux.Unlock()
	return ctx.WriteOK()
}

func cmdSTREAMXWRITE(ctx *Context) error {
	if ctx.ArgCount() < 2 {
		return ctx.WriteError(ErrWrongArgCount)
	}
	name := ctx.ArgString(0)
	data := ctx.ArgString(1)
	streamsXMux.Lock()
	defer streamsXMux.Unlock()
	if s, exists := streamsX[name]; exists {
		s.Data = append(s.Data, data)
		return ctx.WriteOK()
	}
	return ctx.WriteError(fmt.Errorf("ERR stream not found"))
}

func cmdSTREAMXREAD(ctx *Context) error {
	if ctx.ArgCount() < 1 {
		return ctx.WriteError(ErrWrongArgCount)
	}
	name := ctx.ArgString(0)
	streamsXMux.RLock()
	s, exists := streamsX[name]
	if !exists {
		reply := []*resp.Value{}
		streamsXMux.RUnlock()
		return ctx.WriteArray(reply)
	}
	results := make([]*resp.Value, len(s.Data))
	for i, d := range s.Data {
		results[i] = resp.BulkString(d)
	}
	reply := results
	streamsXMux.RUnlock()
	return ctx.WriteArray(reply)
}

func cmdSTREAMXDELETE(ctx *Context) error {
	if ctx.ArgCount() < 1 {
		return ctx.WriteError(ErrWrongArgCount)
	}
	name := ctx.ArgString(0)
	streamsXMux.Lock()
	defer streamsXMux.Unlock()
	if _, exists := streamsX[name]; !exists {
		return ctx.WriteInteger(0)
	}
	delete(streamsX, name)
	return ctx.WriteInteger(1)
}

var (
	cachesX    = make(map[string]*CacheX)
	cachesXMux sync.RWMutex
)

type CacheX struct {
	Name string
	Data map[string]string
}

func cmdCACHEXCREATE(ctx *Context) error {
	if ctx.ArgCount() < 1 {
		return ctx.WriteError(ErrWrongArgCount)
	}
	name := ctx.ArgString(0)
	cachesXMux.Lock()
	cachesX[name] = &CacheX{Name: name, Data: make(map[string]string)}
	cachesXMux.Unlock()
	return ctx.WriteOK()
}

func cmdCACHEXDELETE(ctx *Context) error {
	if ctx.ArgCount() < 1 {
		return ctx.WriteError(ErrWrongArgCount)
	}
	name := ctx.ArgString(0)
	cachesXMux.Lock()
	defer cachesXMux.Unlock()
	if _, exists := cachesX[name]; !exists {
		return ctx.WriteInteger(0)
	}
	delete(cachesX, name)
	return ctx.WriteInteger(1)
}

func cmdCACHEXGET(ctx *Context) error {
	if ctx.ArgCount() < 2 {
		return ctx.WriteError(ErrWrongArgCount)
	}
	name := ctx.ArgString(0)
	key := ctx.ArgString(1)
	cachesXMux.RLock()
	if c, exists := cachesX[name]; exists {
		if val, ok := c.Data[key]; ok {
			reply := val
			cachesXMux.RUnlock()
			return ctx.WriteBulkString(reply)
		}
	}
	cachesXMux.RUnlock()
	return ctx.WriteNull()
}

func cmdCACHEXSET(ctx *Context) error {
	if ctx.ArgCount() < 3 {
		return ctx.WriteError(ErrWrongArgCount)
	}
	name := ctx.ArgString(0)
	key := ctx.ArgString(1)
	value := ctx.ArgString(2)
	cachesXMux.Lock()
	defer cachesXMux.Unlock()
	if c, exists := cachesX[name]; exists {
		c.Data[key] = value
		return ctx.WriteOK()
	}
	return ctx.WriteError(fmt.Errorf("ERR cache not found"))
}

func cmdCACHEXLIST(ctx *Context) error {
	cachesXMux.RLock()
	results := make([]*resp.Value, 0)
	for name := range cachesX {
		results = append(results, resp.BulkString(name))
	}
	reply := results
	cachesXMux.RUnlock()
	return ctx.WriteArray(reply)
}

var (
	storesX    = make(map[string]*StoreX)
	storesXMux sync.RWMutex
)

type StoreX struct {
	Name string
	Data map[string]string
}

func cmdSTOREXCREATE(ctx *Context) error {
	if ctx.ArgCount() < 1 {
		return ctx.WriteError(ErrWrongArgCount)
	}
	name := ctx.ArgString(0)
	storesXMux.Lock()
	storesX[name] = &StoreX{Name: name, Data: make(map[string]string)}
	storesXMux.Unlock()
	return ctx.WriteOK()
}

func cmdSTOREXDELETE(ctx *Context) error {
	if ctx.ArgCount() < 1 {
		return ctx.WriteError(ErrWrongArgCount)
	}
	name := ctx.ArgString(0)
	storesXMux.Lock()
	defer storesXMux.Unlock()
	if _, exists := storesX[name]; !exists {
		return ctx.WriteInteger(0)
	}
	delete(storesX, name)
	return ctx.WriteInteger(1)
}

func cmdSTOREXPUT(ctx *Context) error {
	if ctx.ArgCount() < 3 {
		return ctx.WriteError(ErrWrongArgCount)
	}
	name := ctx.ArgString(0)
	key := ctx.ArgString(1)
	value := ctx.ArgString(2)
	storesXMux.Lock()
	defer storesXMux.Unlock()
	if s, exists := storesX[name]; exists {
		s.Data[key] = value
		return ctx.WriteOK()
	}
	return ctx.WriteError(fmt.Errorf("ERR store not found"))
}

func cmdSTOREXGET(ctx *Context) error {
	if ctx.ArgCount() < 2 {
		return ctx.WriteError(ErrWrongArgCount)
	}
	name := ctx.ArgString(0)
	key := ctx.ArgString(1)
	storesXMux.RLock()
	if s, exists := storesX[name]; exists {
		if val, ok := s.Data[key]; ok {
			reply := val
			storesXMux.RUnlock()
			return ctx.WriteBulkString(reply)
		}
	}
	storesXMux.RUnlock()
	return ctx.WriteNull()
}

func cmdSTOREXLIST(ctx *Context) error {
	storesXMux.RLock()
	results := make([]*resp.Value, 0)
	for name := range storesX {
		results = append(results, resp.BulkString(name))
	}
	reply := results
	storesXMux.RUnlock()
	return ctx.WriteArray(reply)
}

var (
	indexes   = make(map[string]*Index)
	indexesMu sync.RWMutex
)

type Index struct {
	Name    string
	Entries map[string][]string
}

func cmdINDEXCREATE(ctx *Context) error {
	if ctx.ArgCount() < 1 {
		return ctx.WriteError(ErrWrongArgCount)
	}
	name := ctx.ArgString(0)
	indexesMu.Lock()
	indexes[name] = &Index{Name: name, Entries: make(map[string][]string)}
	indexesMu.Unlock()
	return ctx.WriteOK()
}

func cmdINDEXDELETE(ctx *Context) error {
	if ctx.ArgCount() < 1 {
		return ctx.WriteError(ErrWrongArgCount)
	}
	name := ctx.ArgString(0)
	indexesMu.Lock()
	defer indexesMu.Unlock()
	if _, exists := indexes[name]; !exists {
		return ctx.WriteInteger(0)
	}
	delete(indexes, name)
	return ctx.WriteInteger(1)
}

func cmdINDEXADD(ctx *Context) error {
	if ctx.ArgCount() < 3 {
		return ctx.WriteError(ErrWrongArgCount)
	}
	name := ctx.ArgString(0)
	key := ctx.ArgString(1)
	id := ctx.ArgString(2)
	indexesMu.Lock()
	defer indexesMu.Unlock()
	if idx, exists := indexes[name]; exists {
		idx.Entries[key] = append(idx.Entries[key], id)
		return ctx.WriteOK()
	}
	return ctx.WriteError(fmt.Errorf("ERR index not found"))
}

func cmdINDEXSEARCH(ctx *Context) error {
	if ctx.ArgCount() < 2 {
		return ctx.WriteError(ErrWrongArgCount)
	}
	name := ctx.ArgString(0)
	key := ctx.ArgString(1)
	indexesMu.RLock()
	idx, exists := indexes[name]
	if !exists {
		reply := []*resp.Value{}
		indexesMu.RUnlock()
		return ctx.WriteArray(reply)
	}
	results := make([]*resp.Value, 0)
	if ids, ok := idx.Entries[key]; ok {
		for _, id := range ids {
			results = append(results, resp.BulkString(id))
		}
	}
	reply := results
	indexesMu.RUnlock()
	return ctx.WriteArray(reply)
}

func cmdINDEXLIST(ctx *Context) error {
	indexesMu.RLock()
	results := make([]*resp.Value, 0)
	for name := range indexes {
		results = append(results, resp.BulkString(name))
	}
	reply := results
	indexesMu.RUnlock()
	return ctx.WriteArray(reply)
}

var (
	views   = make(map[string]string)
	viewsMu sync.RWMutex
)

func cmdVIEWCREATE(ctx *Context) error {
	if ctx.ArgCount() < 2 {
		return ctx.WriteError(ErrWrongArgCount)
	}
	name := ctx.ArgString(0)
	def := ctx.ArgString(1)
	viewsMu.Lock()
	views[name] = def
	viewsMu.Unlock()
	return ctx.WriteOK()
}

func cmdVIEWDELETE(ctx *Context) error {
	if ctx.ArgCount() < 1 {
		return ctx.WriteError(ErrWrongArgCount)
	}
	name := ctx.ArgString(0)
	viewsMu.Lock()
	defer viewsMu.Unlock()
	if _, exists := views[name]; !exists {
		return ctx.WriteInteger(0)
	}
	delete(views, name)
	return ctx.WriteInteger(1)
}

func cmdVIEWGET(ctx *Context) error {
	if ctx.ArgCount() < 1 {
		return ctx.WriteError(ErrWrongArgCount)
	}
	name := ctx.ArgString(0)
	viewsMu.RLock()
	def, exists := views[name]
	viewsMu.RUnlock()
	if !exists {
		return ctx.WriteError(fmt.Errorf("ERR view not found"))
	}
	return ctx.WriteBulkString(def)
}

func cmdVIEWLIST(ctx *Context) error {
	viewsMu.RLock()
	results := make([]*resp.Value, 0)
	for name := range views {
		results = append(results, resp.BulkString(name))
	}
	reply := results
	viewsMu.RUnlock()
	return ctx.WriteArray(reply)
}

var (
	auditsX    = make(map[string][]*AuditEntryX)
	auditsXMux sync.RWMutex
)

type AuditEntryX struct {
	Timestamp int64
	Action    string
	User      string
	Resource  string
}

func cmdAUDITXLOG(ctx *Context) error {
	if ctx.ArgCount() < 3 {
		return ctx.WriteError(ErrWrongArgCount)
	}
	logName := ctx.ArgString(0)
	action := ctx.ArgString(1)
	user := ctx.ArgString(2)
	resource := ""
	if ctx.ArgCount() >= 4 {
		resource = ctx.ArgString(3)
	}
	auditsXMux.Lock()
	defer auditsXMux.Unlock()
	if _, exists := auditsX[logName]; !exists {
		auditsX[logName] = make([]*AuditEntryX, 0)
	}
	auditsX[logName] = append(auditsX[logName], &AuditEntryX{
		Timestamp: time.Now().UnixMilli(),
		Action:    action,
		User:      user,
		Resource:  resource,
	})
	return ctx.WriteOK()
}

func cmdAUDITXGET(ctx *Context) error {
	if ctx.ArgCount() < 1 {
		return ctx.WriteError(ErrWrongArgCount)
	}
	logName := ctx.ArgString(0)
	auditsXMux.RLock()
	entries, exists := auditsX[logName]
	auditsXMux.RUnlock()
	if !exists {
		return ctx.WriteArray([]*resp.Value{})
	}
	results := make([]*resp.Value, 0)
	for _, e := range entries {
		results = append(results, resp.ArrayValue([]*resp.Value{
			resp.BulkString("timestamp"), resp.IntegerValue(e.Timestamp),
			resp.BulkString("action"), resp.BulkString(e.Action),
			resp.BulkString("user"), resp.BulkString(e.User),
			resp.BulkString("resource"), resp.BulkString(e.Resource),
		}))
	}
	return ctx.WriteArray(results)
}

func cmdAUDITXSEARCH(ctx *Context) error {
	if ctx.ArgCount() < 2 {
		return ctx.WriteError(ErrWrongArgCount)
	}
	logName := ctx.ArgString(0)
	query := ctx.ArgString(1)
	auditsXMux.RLock()
	entries, exists := auditsX[logName]
	auditsXMux.RUnlock()
	if !exists {
		return ctx.WriteArray([]*resp.Value{})
	}
	results := make([]*resp.Value, 0)
	for _, e := range entries {
		if e.Action == query || e.User == query || e.Resource == query {
			results = append(results, resp.ArrayValue([]*resp.Value{
				resp.BulkString("timestamp"), resp.IntegerValue(e.Timestamp),
				resp.BulkString("action"), resp.BulkString(e.Action),
				resp.BulkString("user"), resp.BulkString(e.User),
			}))
		}
	}
	return ctx.WriteArray(results)
}

func cmdAUDITXLIST(ctx *Context) error {
	auditsXMux.RLock()
	results := make([]*resp.Value, 0)
	for name := range auditsX {
		results = append(results, resp.BulkString(name))
	}
	reply := results
	auditsXMux.RUnlock()
	return ctx.WriteArray(reply)
}

var (
	tokens   = make(map[string]*Token)
	tokensMu sync.RWMutex
)

type Token struct {
	ID        string
	User      string
	ExpiresAt int64
}

func cmdTOKENCREATE(ctx *Context) error {
	if ctx.ArgCount() < 2 {
		return ctx.WriteError(ErrWrongArgCount)
	}
	user := ctx.ArgString(0)
	ttlMs := parseInt64(ctx.ArgString(1))
	id := generateUUID()
	tokensMu.Lock()
	tokens[id] = &Token{ID: id, User: user, ExpiresAt: time.Now().UnixMilli() + ttlMs}
	tokensMu.Unlock()
	return ctx.WriteBulkString(id)
}

func cmdTOKENDELETE(ctx *Context) error {
	if ctx.ArgCount() < 1 {
		return ctx.WriteError(ErrWrongArgCount)
	}
	id := ctx.ArgString(0)
	tokensMu.Lock()
	defer tokensMu.Unlock()
	if _, exists := tokens[id]; !exists {
		return ctx.WriteInteger(0)
	}
	delete(tokens, id)
	return ctx.WriteInteger(1)
}

func cmdTOKENVALIDATE(ctx *Context) error {
	if ctx.ArgCount() < 1 {
		return ctx.WriteError(ErrWrongArgCount)
	}
	id := ctx.ArgString(0)
	tokensMu.RLock()
	token, exists := tokens[id]
	if !exists {
		reply := int64(0)
		tokensMu.RUnlock()
		return ctx.WriteInteger(reply)
	}
	if time.Now().UnixMilli() > token.ExpiresAt {
		reply := int64(0)
		tokensMu.RUnlock()
		return ctx.WriteInteger(reply)
	}
	reply := int64(1)
	tokensMu.RUnlock()
	return ctx.WriteInteger(reply)
}

func cmdTOKENREFRESH(ctx *Context) error {
	if ctx.ArgCount() < 2 {
		return ctx.WriteError(ErrWrongArgCount)
	}
	id := ctx.ArgString(0)
	ttlMs := parseInt64(ctx.ArgString(1))
	tokensMu.Lock()
	defer tokensMu.Unlock()
	if token, exists := tokens[id]; exists {
		token.ExpiresAt = time.Now().UnixMilli() + ttlMs
		return ctx.WriteOK()
	}
	return ctx.WriteError(fmt.Errorf("ERR token not found"))
}

func cmdTOKENLIST(ctx *Context) error {
	tokensMu.RLock()
	results := make([]*resp.Value, 0)
	for id := range tokens {
		results = append(results, resp.BulkString(id))
	}
	reply := results
	tokensMu.RUnlock()
	return ctx.WriteArray(reply)
}

var (
	sessionsX        = make(map[string]*SessionX)
	sessionsXMux     sync.Mutex
	sessionsXSweptAt int64
)

type SessionX struct {
	ID        string
	User      string
	Data      map[string]string
	CreatedAt int64
	ExpiresAt int64
}

var errSessionXNotFound = errors.New("ERR session not found")

// liveSessionX returns the session if it exists and has not expired, deleting
// it when it has. Callers hold sessionsXMux.
func liveSessionX(id string, now int64) (*SessionX, bool) {
	s, exists := sessionsX[id]
	if !exists {
		return nil, false
	}
	if s.ExpiresAt <= now {
		delete(sessionsX, id)
		return nil, false
	}
	return s, true
}

// sweepSessionsX drops every expired session. Callers hold sessionsXMux.
func sweepSessionsX(now int64) {
	sessionsXSweptAt = now
	for id, s := range sessionsX {
		if s.ExpiresAt <= now {
			delete(sessionsX, id)
		}
	}
}

func cmdSESSIONXCREATE(ctx *Context) error {
	if ctx.ArgCount() != 2 {
		return ctx.WriteError(ErrWrongArgCount)
	}
	user := ctx.ArgString(0)
	ttlMs, err := strconv.ParseInt(ctx.ArgString(1), 10, 64)
	if err != nil || ttlMs < 1 {
		return ctx.WriteError(fmt.Errorf("ERR ttl must be a positive integer (milliseconds)"))
	}
	id := generateUUID()
	now := time.Now().UnixMilli()
	expiresAt := int64(math.MaxInt64)
	if ttlMs <= math.MaxInt64-now {
		expiresAt = now + ttlMs
	}
	sessionsXMux.Lock()
	if now-sessionsXSweptAt >= 1000 {
		sweepSessionsX(now)
	}
	sessionsX[id] = &SessionX{ID: id, User: user, Data: make(map[string]string), CreatedAt: now, ExpiresAt: expiresAt}
	sessionsXMux.Unlock()
	return ctx.WriteBulkString(id)
}

func cmdSESSIONXDELETE(ctx *Context) error {
	if ctx.ArgCount() != 1 {
		return ctx.WriteError(ErrWrongArgCount)
	}
	id := ctx.ArgString(0)
	sessionsXMux.Lock()
	_, exists := liveSessionX(id, time.Now().UnixMilli())
	delete(sessionsX, id)
	sessionsXMux.Unlock()
	if !exists {
		return ctx.WriteInteger(0)
	}
	return ctx.WriteInteger(1)
}

func cmdSESSIONXGET(ctx *Context) error {
	if ctx.ArgCount() != 1 {
		return ctx.WriteError(ErrWrongArgCount)
	}
	id := ctx.ArgString(0)
	now := time.Now().UnixMilli()
	sessionsXMux.Lock()
	session, exists := liveSessionX(id, now)
	if !exists {
		sessionsXMux.Unlock()
		return ctx.WriteError(errSessionXNotFound)
	}
	keys := make([]string, 0, len(session.Data))
	for k := range session.Data {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	data := make([]*resp.Value, 0, 2*len(keys))
	for _, k := range keys {
		data = append(data, resp.BulkString(k), resp.BulkString(session.Data[k]))
	}
	reply := []*resp.Value{
		resp.BulkString("id"), resp.BulkString(session.ID),
		resp.BulkString("user"), resp.BulkString(session.User),
		resp.BulkString("created_at"), resp.IntegerValue(session.CreatedAt),
		resp.BulkString("expires_at"), resp.IntegerValue(session.ExpiresAt),
		resp.BulkString("ttl"), resp.IntegerValue(session.ExpiresAt - now),
		resp.BulkString("data"), resp.ArrayValue(data),
	}
	sessionsXMux.Unlock()
	return ctx.WriteArray(reply)
}

func cmdSESSIONXSET(ctx *Context) error {
	if ctx.ArgCount() < 3 || (ctx.ArgCount()-1)%2 != 0 {
		return ctx.WriteError(ErrWrongArgCount)
	}
	id := ctx.ArgString(0)
	sessionsXMux.Lock()
	s, exists := liveSessionX(id, time.Now().UnixMilli())
	if !exists {
		sessionsXMux.Unlock()
		return ctx.WriteError(errSessionXNotFound)
	}
	for i := 1; i+1 < ctx.ArgCount(); i += 2 {
		s.Data[ctx.ArgString(i)] = ctx.ArgString(i + 1)
	}
	sessionsXMux.Unlock()
	return ctx.WriteOK()
}

func cmdSESSIONXLIST(ctx *Context) error {
	sessionsXMux.Lock()
	sweepSessionsX(time.Now().UnixMilli())
	ids := make([]string, 0, len(sessionsX))
	for id := range sessionsX {
		ids = append(ids, id)
	}
	sessionsXMux.Unlock()
	sort.Strings(ids)
	results := make([]*resp.Value, len(ids))
	for i, id := range ids {
		results[i] = resp.BulkString(id)
	}
	return ctx.WriteArray(results)
}

var (
	profiles   = make(map[string]*Profile)
	profilesMu sync.RWMutex
)

type Profile struct {
	ID         string
	User       string
	Attributes map[string]string
}

func cmdPROFILECREATE(ctx *Context) error {
	if ctx.ArgCount() < 1 {
		return ctx.WriteError(ErrWrongArgCount)
	}
	user := ctx.ArgString(0)
	id := generateUUID()
	profilesMu.Lock()
	profiles[id] = &Profile{ID: id, User: user, Attributes: make(map[string]string)}
	profilesMu.Unlock()
	return ctx.WriteBulkString(id)
}

func cmdPROFILEDELETE(ctx *Context) error {
	if ctx.ArgCount() < 1 {
		return ctx.WriteError(ErrWrongArgCount)
	}
	id := ctx.ArgString(0)
	profilesMu.Lock()
	defer profilesMu.Unlock()
	if _, exists := profiles[id]; !exists {
		return ctx.WriteInteger(0)
	}
	delete(profiles, id)
	return ctx.WriteInteger(1)
}

func cmdPROFILEGET(ctx *Context) error {
	if ctx.ArgCount() < 1 {
		return ctx.WriteError(ErrWrongArgCount)
	}
	id := ctx.ArgString(0)
	profilesMu.RLock()
	profile, exists := profiles[id]
	if !exists {
		reply := fmt.Errorf("ERR profile not found")
		profilesMu.RUnlock()
		return ctx.WriteError(reply)
	}
	results := make([]*resp.Value, 0)
	results = append(results, resp.BulkString("id"), resp.BulkString(profile.ID), resp.BulkString("user"), resp.BulkString(profile.User))
	for k, v := range profile.Attributes {
		results = append(results, resp.BulkString(k), resp.BulkString(v))
	}
	reply := results
	profilesMu.RUnlock()
	return ctx.WriteArray(reply)
}

func cmdPROFILESET(ctx *Context) error {
	if ctx.ArgCount() < 3 {
		return ctx.WriteError(ErrWrongArgCount)
	}
	id := ctx.ArgString(0)
	key := ctx.ArgString(1)
	value := ctx.ArgString(2)
	profilesMu.Lock()
	defer profilesMu.Unlock()
	if p, exists := profiles[id]; exists {
		p.Attributes[key] = value
		return ctx.WriteOK()
	}
	return ctx.WriteError(fmt.Errorf("ERR profile not found"))
}

func cmdPROFILELIST(ctx *Context) error {
	profilesMu.RLock()
	results := make([]*resp.Value, 0)
	for id := range profiles {
		results = append(results, resp.BulkString(id))
	}
	reply := results
	profilesMu.RUnlock()
	return ctx.WriteArray(reply)
}
