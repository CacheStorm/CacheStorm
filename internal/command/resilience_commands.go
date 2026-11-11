package command

import (
	"errors"
	"runtime"
	"strconv"
	"time"

	"github.com/cachestorm/cachestorm/internal/resp"
)

func RegisterResilienceCommands(router *Router) {
	router.Register(&CommandDef{Name: "CIRCUITX.CREATE", Handler: cmdCIRCUITXCREATE})
	router.Register(&CommandDef{Name: "CIRCUITX.OPEN", Handler: cmdCIRCUITXOPEN})
	router.Register(&CommandDef{Name: "CIRCUITX.CLOSE", Handler: cmdCIRCUITXCLOSE})
	router.Register(&CommandDef{Name: "CIRCUITX.HALFOPEN", Handler: cmdCIRCUITXHALFOPEN})
	router.Register(&CommandDef{Name: "CIRCUITX.STATUS", Handler: cmdCIRCUITXSTATUS})
	router.Register(&CommandDef{Name: "CIRCUITX.METRICS", Handler: cmdCIRCUITXMETRICS})
	router.Register(&CommandDef{Name: "CIRCUITX.RESET", Handler: cmdCIRCUITXRESET})
	router.Register(&CommandDef{Name: "CIRCUITX.DELETE", Handler: cmdCIRCUITXDELETE})
	router.Register(&CommandDef{Name: "CIRCUITX.ALLOW", Handler: cmdCIRCUITXALLOW})
	router.Register(&CommandDef{Name: "CIRCUITX.RECORD", Handler: cmdCIRCUITXRECORD})

	router.Register(&CommandDef{Name: "RATELIMITER.CREATE", Handler: cmdRATELIMITERCREATE})
	router.Register(&CommandDef{Name: "RATELIMITER.TRY", Handler: cmdRATELIMITERTRY})
	router.Register(&CommandDef{Name: "RATELIMITER.WAIT", Handler: cmdRATELIMITERWAIT})
	router.Register(&CommandDef{Name: "RATELIMITER.RESET", Handler: cmdRATELIMITERRESET})
	router.Register(&CommandDef{Name: "RATELIMITER.STATUS", Handler: cmdRATELIMITERSTATUS})
	router.Register(&CommandDef{Name: "RATELIMITER.DELETE", Handler: cmdRATELIMITERDELETE})

	router.Register(&CommandDef{Name: "BULKHEAD.CREATE", Handler: cmdBULKHEADCREATE})
	router.Register(&CommandDef{Name: "BULKHEAD.ACQUIRE", Handler: cmdBULKHEADACQUIRE})
	router.Register(&CommandDef{Name: "BULKHEAD.RELEASE", Handler: cmdBULKHEADRELEASE})
	router.Register(&CommandDef{Name: "BULKHEAD.STATUS", Handler: cmdBULKHEADSTATUS})
	router.Register(&CommandDef{Name: "BULKHEAD.DELETE", Handler: cmdBULKHEADDELETE})

	router.Register(&CommandDef{Name: "TELEMETRY.RECORD", Handler: cmdTELEMETRYRECORD})
	router.Register(&CommandDef{Name: "TELEMETRY.QUERY", Handler: cmdTELEMETRYQUERY})

	router.Register(&CommandDef{Name: "PROFILEX.LIST", Handler: cmdPROFILEXLIST})

	router.Register(&CommandDef{Name: "HEAP.STATS", Handler: cmdHEAPSTATS})
	router.Register(&CommandDef{Name: "HEAP.GC", Handler: cmdHEAPGC})

	router.Register(&CommandDef{Name: "LOCKX.ACQUIRE", Handler: cmdLOCKXACQUIRE})
	router.Register(&CommandDef{Name: "LOCKX.RELEASE", Handler: cmdLOCKXRELEASE})
	router.Register(&CommandDef{Name: "LOCKX.EXTEND", Handler: cmdLOCKXEXTEND})
	router.Register(&CommandDef{Name: "LOCKX.STATUS", Handler: cmdLOCKXSTATUS})

	router.Register(&CommandDef{Name: "SEMAPHOREX.CREATE", Handler: cmdSEMAPHOREXCREATE})
	router.Register(&CommandDef{Name: "SEMAPHOREX.ACQUIRE", Handler: cmdSEMAPHOREXACQUIRE})
	router.Register(&CommandDef{Name: "SEMAPHOREX.RELEASE", Handler: cmdSEMAPHOREXRELEASE})
	router.Register(&CommandDef{Name: "SEMAPHOREX.STATUS", Handler: cmdSEMAPHOREXSTATUS})

	router.Register(&CommandDef{Name: "PROMISE.CREATE", Handler: cmdPROMISECREATE})
	router.Register(&CommandDef{Name: "PROMISE.RESOLVE", Handler: cmdPROMISERESOLVE})
	router.Register(&CommandDef{Name: "PROMISE.REJECT", Handler: cmdPROMISEREJECT})
	router.Register(&CommandDef{Name: "PROMISE.STATUS", Handler: cmdPROMISESTATUS})
	router.Register(&CommandDef{Name: "PROMISE.AWAIT", Handler: cmdPROMISEAWAIT})

	router.Register(&CommandDef{Name: "FUTURE.CREATE", Handler: cmdFUTURECREATE})
	router.Register(&CommandDef{Name: "FUTURE.COMPLETE", Handler: cmdFUTURECOMPLETE})
	router.Register(&CommandDef{Name: "FUTURE.GET", Handler: cmdFUTUREGET})
	router.Register(&CommandDef{Name: "FUTURE.CANCEL", Handler: cmdFUTURECANCEL})

	router.Register(&CommandDef{Name: "STREAMPROC.CREATE", Handler: cmdSTREAMPROCCREATE})
	router.Register(&CommandDef{Name: "STREAMPROC.PUSH", Handler: cmdSTREAMPROCPUSH})
	router.Register(&CommandDef{Name: "STREAMPROC.POP", Handler: cmdSTREAMPROCPOP})
	router.Register(&CommandDef{Name: "STREAMPROC.PEEK", Handler: cmdSTREAMPROCPEEK})
	router.Register(&CommandDef{Name: "STREAMPROC.DELETE", Handler: cmdSTREAMPROCDELETE})

	router.Register(&CommandDef{Name: "EVENTSOURCING.APPEND", Handler: cmdEVENTSOURCINGAPPEND})
	router.Register(&CommandDef{Name: "EVENTSOURCING.REPLAY", Handler: cmdEVENTSOURCINGREPLAY})
	router.Register(&CommandDef{Name: "EVENTSOURCING.GET", Handler: cmdEVENTSOURCINGGET})

	router.Register(&CommandDef{Name: "BACKPRESSURE.CREATE", Handler: cmdBACKPRESSURECREATE})
	router.Register(&CommandDef{Name: "BACKPRESSURE.CHECK", Handler: cmdBACKPRESSURECHECK})
	router.Register(&CommandDef{Name: "BACKPRESSURE.STATUS", Handler: cmdBACKPRESSURESTATUS})

	router.Register(&CommandDef{Name: "THROTTLEX.CREATE", Handler: cmdTHROTTLEXCREATE})
	router.Register(&CommandDef{Name: "THROTTLEX.CHECK", Handler: cmdTHROTTLEXCHECK})
	router.Register(&CommandDef{Name: "THROTTLEX.STATUS", Handler: cmdTHROTTLEXSTATUS})

	router.Register(&CommandDef{Name: "DEBOUNCEX.CREATE", Handler: cmdDEBOUNCEXCREATE})
	router.Register(&CommandDef{Name: "DEBOUNCEX.CALL", Handler: cmdDEBOUNCEXCALL})
	router.Register(&CommandDef{Name: "DEBOUNCEX.CANCEL", Handler: cmdDEBOUNCEXCANCEL})
	router.Register(&CommandDef{Name: "DEBOUNCEX.FLUSH", Handler: cmdDEBOUNCEXFLUSH})

	router.Register(&CommandDef{Name: "COALESCE.CREATE", Handler: cmdCOALESCECREATE})
	router.Register(&CommandDef{Name: "COALESCE.ADD", Handler: cmdCOALESCEADD})
	router.Register(&CommandDef{Name: "COALESCE.GET", Handler: cmdCOALESCEGET})
	router.Register(&CommandDef{Name: "COALESCE.CLEAR", Handler: cmdCOALESCECLEAR})

	router.Register(&CommandDef{Name: "AGGREGATOR.CREATE", Handler: cmdAGGREGATORCREATE})
	router.Register(&CommandDef{Name: "AGGREGATOR.ADD", Handler: cmdAGGREGATORADD})
	router.Register(&CommandDef{Name: "AGGREGATOR.GET", Handler: cmdAGGREGATORGET})
	router.Register(&CommandDef{Name: "AGGREGATOR.RESET", Handler: cmdAGGREGATORRESET})

	router.Register(&CommandDef{Name: "WINDOWX.CREATE", Handler: cmdWINDOWXCREATE})
	router.Register(&CommandDef{Name: "WINDOWX.ADD", Handler: cmdWINDOWXADD})
	router.Register(&CommandDef{Name: "WINDOWX.GET", Handler: cmdWINDOWXGET})
	router.Register(&CommandDef{Name: "WINDOWX.AGGREGATE", Handler: cmdWINDOWXAGGREGATE})

	router.Register(&CommandDef{Name: "JOINX.CREATE", Handler: cmdJOINXCREATE})
	router.Register(&CommandDef{Name: "JOINX.ADD", Handler: cmdJOINXADD})
	router.Register(&CommandDef{Name: "JOINX.GET", Handler: cmdJOINXGET})
	router.Register(&CommandDef{Name: "JOINX.DELETE", Handler: cmdJOINXDELETE})

	router.Register(&CommandDef{Name: "SHUFFLE.CREATE", Handler: cmdSHUFFLECREATE})
	router.Register(&CommandDef{Name: "SHUFFLE.ADD", Handler: cmdSHUFFLEADD})
	router.Register(&CommandDef{Name: "SHUFFLE.GET", Handler: cmdSHUFFLEGET})

	router.Register(&CommandDef{Name: "PARTITIONX.CREATE", Handler: cmdPARTITIONXCREATE})
	router.Register(&CommandDef{Name: "PARTITIONX.ADD", Handler: cmdPARTITIONXADD})
	router.Register(&CommandDef{Name: "PARTITIONX.GET", Handler: cmdPARTITIONXGET})
}

// resilienceMaxWait caps how long PROMISE.AWAIT, FUTURE.GET,
// RATELIMITER.WAIT and BULKHEAD.ACQUIRE block; a zero timeout means the cap.
const resilienceMaxWait = 5 * time.Minute

func parseResilienceTimeout(s string) (time.Duration, error) {
	ms, err := strconv.ParseInt(s, 10, 64)
	if err != nil || ms < 0 {
		return 0, errors.New("ERR timeout is not an integer or out of range")
	}
	if ms == 0 || ms > resilienceMaxWait.Milliseconds() {
		return resilienceMaxWait, nil
	}
	return time.Duration(ms) * time.Millisecond, nil
}

type Timeout struct {
	Name    string
	Timeout int64
	Created int64
}

func cmdHEAPSTATS(ctx *Context) error {
	var ms runtime.MemStats
	runtime.ReadMemStats(&ms)
	return ctx.WriteArray([]*resp.Value{
		resp.BulkString("allocations"), resp.IntegerValue(int64(ms.Mallocs)),
		resp.BulkString("frees"), resp.IntegerValue(int64(ms.Frees)),
		resp.BulkString("in_use"), resp.IntegerValue(int64(ms.HeapInuse)),
		resp.BulkString("heap_alloc"), resp.IntegerValue(int64(ms.HeapAlloc)),
		resp.BulkString("heap_sys"), resp.IntegerValue(int64(ms.HeapSys)),
		resp.BulkString("gc_cycles"), resp.IntegerValue(int64(ms.NumGC)),
	})
}

func cmdHEAPGC(ctx *Context) error {
	runtime.GC()
	return ctx.WriteOK()
}
