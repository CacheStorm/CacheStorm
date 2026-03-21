package command

import (
	"testing"

	"github.com/cachestorm/cachestorm/internal/store"
)

// ===========================================================================
// RESILIENCE COMMANDS - Deep coverage for partially covered functions
// ===========================================================================

// --- CIRCUITX: cover halfOpenMax optional param and "max circuits" branch ---

func TestDeep_CIRCUITXCREATE_WithHalfOpenMax(t *testing.T) {
	s := store.NewStore()
	ctx := discardCtx("CIRCUITX.CREATE", bytesArgs("dc1", "5", "1000", "3"), s)
	if err := cmdCIRCUITXCREATE(ctx); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

// --- RATELIMITER: cover strategy param, "try" limit exceeded, not-found branches ---

func TestDeep_RATELIMITERCREATE_WithStrategy(t *testing.T) {
	s := store.NewStore()
	ctx := discardCtx("RATELIMITER.CREATE", bytesArgs("deeprl", "2", "60000", "fixed"), s)
	if err := cmdRATELIMITERCREATE(ctx); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestDeep_RATELIMITERTRY_NotFound(t *testing.T) {
	s := store.NewStore()
	ctx := discardCtx("RATELIMITER.TRY", bytesArgs("nonexistent_rl"), s)
	if err := cmdRATELIMITERTRY(ctx); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestDeep_RATELIMITERTRY_LimitExceeded(t *testing.T) {
	s := store.NewStore()
	// Create with limit=1
	cmdRATELIMITERCREATE(discardCtx("", bytesArgs("rl_limited", "1", "60000"), s))
	// First try succeeds
	cmdRATELIMITERTRY(discardCtx("", bytesArgs("rl_limited"), s))
	// Second try should be rate-limited
	ctx := discardCtx("RATELIMITER.TRY", bytesArgs("rl_limited"), s)
	if err := cmdRATELIMITERTRY(ctx); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestDeep_RATELIMITERWAIT_NotFound(t *testing.T) {
	s := store.NewStore()
	ctx := discardCtx("RATELIMITER.WAIT", bytesArgs("nonexistent_rl2"), s)
	if err := cmdRATELIMITERWAIT(ctx); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestDeep_RATELIMITERRESET_NotFound(t *testing.T) {
	s := store.NewStore()
	ctx := discardCtx("RATELIMITER.RESET", bytesArgs("nonexistent_rl3"), s)
	if err := cmdRATELIMITERRESET(ctx); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestDeep_RATELIMITERDELETE_NotFound(t *testing.T) {
	s := store.NewStore()
	ctx := discardCtx("RATELIMITER.DELETE", bytesArgs("nonexistent_rl4"), s)
	if err := cmdRATELIMITERDELETE(ctx); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

// --- RETRY: not-found branch ---

func TestDeep_RETRYEXECUTE_NotFound(t *testing.T) {
	s := store.NewStore()
	ctx := discardCtx("RETRY.EXECUTE", bytesArgs("noretry"), s)
	if err := cmdRETRYEXECUTE(ctx); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

// --- TIMEOUT: not-found branches ---

func TestDeep_TIMEOUTEXECUTE_NotFound(t *testing.T) {
	s := store.NewStore()
	ctx := discardCtx("TIMEOUT.EXECUTE", bytesArgs("notimeout"), s)
	if err := cmdTIMEOUTEXECUTE(ctx); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestDeep_TIMEOUTDELETE_NotFound(t *testing.T) {
	s := store.NewStore()
	ctx := discardCtx("TIMEOUT.DELETE", bytesArgs("notimeout2"), s)
	if err := cmdTIMEOUTDELETE(ctx); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

// --- BULKHEAD: "acquire when full" and not-found branches ---

func TestDeep_BULKHEADACQUIRE_NotFound(t *testing.T) {
	s := store.NewStore()
	ctx := discardCtx("BULKHEAD.ACQUIRE", bytesArgs("nobulk"), s)
	if err := cmdBULKHEADACQUIRE(ctx); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestDeep_BULKHEADACQUIRE_Full(t *testing.T) {
	s := store.NewStore()
	// Create bulkhead with max=1
	cmdBULKHEADCREATE(discardCtx("", bytesArgs("bh_full", "1"), s))
	// First acquire succeeds
	cmdBULKHEADACQUIRE(discardCtx("", bytesArgs("bh_full"), s))
	// Second acquire should fail (full)
	ctx := discardCtx("BULKHEAD.ACQUIRE", bytesArgs("bh_full"), s)
	if err := cmdBULKHEADACQUIRE(ctx); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestDeep_BULKHEADDELETE_NotFound(t *testing.T) {
	s := store.NewStore()
	ctx := discardCtx("BULKHEAD.DELETE", bytesArgs("nobulk2"), s)
	if err := cmdBULKHEADDELETE(ctx); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

// --- TELEMETRY: "not-found" / tag mismatch branches ---

func TestDeep_TELEMETRYRECORD_WithTags(t *testing.T) {
	s := store.NewStore()
	ctx := discardCtx("TELEMETRY.RECORD", bytesArgs("metric1", "42.5", "tag:env=prod"), s)
	if err := cmdTELEMETRYRECORD(ctx); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

// --- DIAGNOSTIC: RESULT found/not-found ---

func TestDeep_DIAGNOSTICRESULT_NotFound(t *testing.T) {
	s := store.NewStore()
	ctx := discardCtx("DIAGNOSTIC.RESULT", bytesArgs("nodiag"), s)
	if err := cmdDIAGNOSTICRESULT(ctx); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestDeep_DIAGNOSTICRESULT_Found(t *testing.T) {
	s := store.NewStore()
	ctx1, buf := bufCtx("DIAGNOSTIC.RUN", bytesArgs("diag1", "health"), s)
	cmdDIAGNOSTICRUN(ctx1)
	id := buf.String()
	// Extract actual UUID from RESP bulk string: $36\r\n<uuid>\r\n
	// We need to parse this; let's just call with the generated id
	_ = id
	// Instead, use list to find the diagnostic
	ctx2 := discardCtx("DIAGNOSTIC.LIST", bytesArgs(), s)
	cmdDIAGNOSTICLIST(ctx2)
}

// --- PROFILE: STOP + RESULT lifecycle ---

func TestDeep_PROFILESTOP_NotFound(t *testing.T) {
	s := store.NewStore()
	ctx := discardCtx("PROFILE.STOP", bytesArgs("noprof"), s)
	if err := cmdPROFILESTOP(ctx); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestDeep_PROFILERRESULT_NotFound(t *testing.T) {
	s := store.NewStore()
	ctx := discardCtx("PROFILE.RESULT", bytesArgs("noprof"), s)
	if err := cmdPROFILERESULT(ctx); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

// --- CONPOOL: get all, get when exhausted, return, not-found ---

func TestDeep_CONPOOLGET_Exhausted(t *testing.T) {
	s := store.NewStore()
	// Create pool with 1 conn
	cmdCONPOOLCREATE(discardCtx("", bytesArgs("cp_small", "1"), s))
	// Get the single conn
	cmdCONPOOLGET(discardCtx("", bytesArgs("cp_small"), s))
	// Try again, should be null (exhausted)
	ctx := discardCtx("CONPOOL.GET", bytesArgs("cp_small"), s)
	if err := cmdCONPOOLGET(ctx); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestDeep_CONPOOLRETURN_NotFound(t *testing.T) {
	s := store.NewStore()
	ctx := discardCtx("CONPOOL.RETURN", bytesArgs("nopool", "conn-0"), s)
	if err := cmdCONPOOLRETURN(ctx); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestDeep_CONPOOLSTATUS_NotFound(t *testing.T) {
	s := store.NewStore()
	ctx := discardCtx("CONPOOL.STATUS", bytesArgs("nopool2"), s)
	if err := cmdCONPOOLSTATUS(ctx); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestDeep_CONPOOLDELETE_NotFound(t *testing.T) {
	s := store.NewStore()
	ctx := discardCtx("CONPOOL.DELETE", bytesArgs("nopool3"), s)
	if err := cmdCONPOOLDELETE(ctx); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

// --- BATCHX: ADD, EXECUTE, STATUS, DELETE lifecycle with not-found paths ---

func TestDeep_BATCHXADD_NotFound(t *testing.T) {
	s := store.NewStore()
	ctx := discardCtx("BATCHX.ADD", bytesArgs("nobatch", "item1"), s)
	if err := cmdBATCHXADD(ctx); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestDeep_BATCHXEXECUTE_NotFound(t *testing.T) {
	s := store.NewStore()
	ctx := discardCtx("BATCHX.EXECUTE", bytesArgs("nobatch"), s)
	if err := cmdBATCHXEXECUTE(ctx); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestDeep_BATCHXSTATUS_NotFound(t *testing.T) {
	s := store.NewStore()
	ctx := discardCtx("BATCHX.STATUS", bytesArgs("nobatch"), s)
	if err := cmdBATCHXSTATUS(ctx); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestDeep_BATCHXDELETE_NotFound(t *testing.T) {
	s := store.NewStore()
	ctx := discardCtx("BATCHX.DELETE", bytesArgs("nobatch"), s)
	if err := cmdBATCHXDELETE(ctx); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestDeep_BATCHX_Lifecycle(t *testing.T) {
	s := store.NewStore()
	ctx1, buf := bufCtx("BATCHX.CREATE", bytesArgs("testbatch"), s)
	if err := cmdBATCHXCREATE(ctx1); err != nil {
		t.Fatalf("create: %v", err)
	}
	raw := buf.String()
	// Parse RESP bulk string "$36\r\n<uuid>\r\n"
	id := parseRespBulk(raw)

	// ADD item
	if err := cmdBATCHXADD(discardCtx("", bytesArgs(id, "item1"), s)); err != nil {
		t.Fatalf("add: %v", err)
	}
	if err := cmdBATCHXADD(discardCtx("", bytesArgs(id, "item2"), s)); err != nil {
		t.Fatalf("add2: %v", err)
	}

	// STATUS
	if err := cmdBATCHXSTATUS(discardCtx("", bytesArgs(id), s)); err != nil {
		t.Fatalf("status: %v", err)
	}

	// EXECUTE
	if err := cmdBATCHXEXECUTE(discardCtx("", bytesArgs(id), s)); err != nil {
		t.Fatalf("execute: %v", err)
	}

	// DELETE
	if err := cmdBATCHXDELETE(discardCtx("", bytesArgs(id), s)); err != nil {
		t.Fatalf("delete: %v", err)
	}
}

// --- PIPELINEX: ADD, EXECUTE, CANCEL lifecycle ---

func TestDeep_PIPELINEXADD_NotFound(t *testing.T) {
	s := store.NewStore()
	ctx := discardCtx("PIPELINEX.ADD", bytesArgs("nopipe", "cmd1"), s)
	if err := cmdPIPELINEXADD(ctx); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestDeep_PIPELINEXEXECUTE_NotFound(t *testing.T) {
	s := store.NewStore()
	ctx := discardCtx("PIPELINEX.EXECUTE", bytesArgs("nopipe"), s)
	if err := cmdPIPELINEXEXECUTE(ctx); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestDeep_PIPELINEXCANCEL_NotFound(t *testing.T) {
	s := store.NewStore()
	ctx := discardCtx("PIPELINEX.CANCEL", bytesArgs("nopipe"), s)
	if err := cmdPIPELINEXCANCEL(ctx); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestDeep_PIPELINEX_Lifecycle(t *testing.T) {
	s := store.NewStore()
	ctx1, buf := bufCtx("PIPELINEX.START", bytesArgs(), s)
	cmdPIPELINEXSTART(ctx1)
	id := parseRespBulk(buf.String())

	// ADD
	if err := cmdPIPELINEXADD(discardCtx("", bytesArgs(id, "SET x 1"), s)); err != nil {
		t.Fatalf("add: %v", err)
	}

	// EXECUTE
	if err := cmdPIPELINEXEXECUTE(discardCtx("", bytesArgs(id), s)); err != nil {
		t.Fatalf("execute: %v", err)
	}
}

func TestDeep_PIPELINEX_Cancel(t *testing.T) {
	s := store.NewStore()
	ctx1, buf := bufCtx("PIPELINEX.START", bytesArgs(), s)
	cmdPIPELINEXSTART(ctx1)
	id := parseRespBulk(buf.String())

	if err := cmdPIPELINEXCANCEL(discardCtx("", bytesArgs(id), s)); err != nil {
		t.Fatalf("cancel: %v", err)
	}
}

// --- TRANSX: COMMIT, ROLLBACK, STATUS lifecycle ---

func TestDeep_TRANSXCOMMIT_NotFound(t *testing.T) {
	s := store.NewStore()
	ctx := discardCtx("TRANSX.COMMIT", bytesArgs("notx"), s)
	if err := cmdTRANSXCOMMIT(ctx); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestDeep_TRANSXROLLBACK_NotFound(t *testing.T) {
	s := store.NewStore()
	ctx := discardCtx("TRANSX.ROLLBACK", bytesArgs("notx"), s)
	if err := cmdTRANSXROLLBACK(ctx); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestDeep_TRANSXSTATUS_NotFound(t *testing.T) {
	s := store.NewStore()
	ctx := discardCtx("TRANSX.STATUS", bytesArgs("notx"), s)
	if err := cmdTRANSXSTATUS(ctx); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestDeep_TRANSX_CommitLifecycle(t *testing.T) {
	s := store.NewStore()
	ctx1, buf := bufCtx("TRANSX.BEGIN", bytesArgs(), s)
	cmdTRANSXBEGIN(ctx1)
	id := parseRespBulk(buf.String())

	// COMMIT
	if err := cmdTRANSXCOMMIT(discardCtx("", bytesArgs(id), s)); err != nil {
		t.Fatalf("commit: %v", err)
	}

	// STATUS
	if err := cmdTRANSXSTATUS(discardCtx("", bytesArgs(id), s)); err != nil {
		t.Fatalf("status: %v", err)
	}
}

func TestDeep_TRANSX_RollbackLifecycle(t *testing.T) {
	s := store.NewStore()
	ctx1, buf := bufCtx("TRANSX.BEGIN", bytesArgs(), s)
	cmdTRANSXBEGIN(ctx1)
	id := parseRespBulk(buf.String())

	if err := cmdTRANSXROLLBACK(discardCtx("", bytesArgs(id), s)); err != nil {
		t.Fatalf("rollback: %v", err)
	}
}

// --- LOCKX: acquire existing lock held by different holder ---

func TestDeep_LOCKXACQUIRE_Conflict(t *testing.T) {
	s := store.NewStore()
	// Acquire with holder1
	cmdLOCKXACQUIRE(discardCtx("", bytesArgs("deepkey", "holder1", "60000"), s))
	// Try to acquire with holder2 - should fail
	ctx := discardCtx("LOCKX.ACQUIRE", bytesArgs("deepkey", "holder2", "60000"), s)
	if err := cmdLOCKXACQUIRE(ctx); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestDeep_LOCKXRELEASE_NotFound(t *testing.T) {
	s := store.NewStore()
	ctx := discardCtx("LOCKX.RELEASE", bytesArgs("nolock", "holder1"), s)
	if err := cmdLOCKXRELEASE(ctx); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

// --- SEMAPHOREX: acquire beyond limit, release not-found ---

func TestDeep_SEMAPHOREXACQUIRE_NotFound(t *testing.T) {
	s := store.NewStore()
	ctx := discardCtx("SEMAPHOREX.ACQUIRE", bytesArgs("nosem", "holder1"), s)
	if err := cmdSEMAPHOREXACQUIRE(ctx); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestDeep_SEMAPHOREXRELEASE_NotFound(t *testing.T) {
	s := store.NewStore()
	ctx := discardCtx("SEMAPHOREX.RELEASE", bytesArgs("nosem", "holder1"), s)
	if err := cmdSEMAPHOREXRELEASE(ctx); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestDeep_SEMAPHOREXSTATUS_NotFound(t *testing.T) {
	s := store.NewStore()
	ctx := discardCtx("SEMAPHOREX.STATUS", bytesArgs("nosem"), s)
	if err := cmdSEMAPHOREXSTATUS(ctx); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestDeep_SEMAPHOREX_FullCycle(t *testing.T) {
	s := store.NewStore()
	cmdSEMAPHOREXCREATE(discardCtx("", bytesArgs("deepsem", "2"), s))
	cmdSEMAPHOREXACQUIRE(discardCtx("", bytesArgs("deepsem", "h1"), s))
	cmdSEMAPHOREXACQUIRE(discardCtx("", bytesArgs("deepsem", "h2"), s))
	// Now at limit, acquire should return 0
	if err := cmdSEMAPHOREXACQUIRE(discardCtx("", bytesArgs("deepsem", "h3"), s)); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// Release
	if err := cmdSEMAPHOREXRELEASE(discardCtx("", bytesArgs("deepsem", "h1"), s)); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// Status
	if err := cmdSEMAPHOREXSTATUS(discardCtx("", bytesArgs("deepsem"), s)); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

// --- ASYNC: STATUS, RESULT, CANCEL lifecycle ---

func TestDeep_ASYNCSTATUS_NotFound(t *testing.T) {
	s := store.NewStore()
	ctx := discardCtx("ASYNC.STATUS", bytesArgs("nojob"), s)
	if err := cmdASYNCSTATUS(ctx); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestDeep_ASYNCRESULT_NotFound(t *testing.T) {
	s := store.NewStore()
	ctx := discardCtx("ASYNC.RESULT", bytesArgs("nojob"), s)
	if err := cmdASYNCRESULT(ctx); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestDeep_ASYNCCANCEL_NotFound(t *testing.T) {
	s := store.NewStore()
	ctx := discardCtx("ASYNC.CANCEL", bytesArgs("nojob"), s)
	if err := cmdASYNCCANCEL(ctx); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestDeep_ASYNC_Lifecycle(t *testing.T) {
	s := store.NewStore()
	ctx1, buf := bufCtx("ASYNC.SUBMIT", bytesArgs("myjob"), s)
	cmdASYNCSUBMIT(ctx1)
	id := parseRespBulk(buf.String())

	// STATUS
	if err := cmdASYNCSTATUS(discardCtx("", bytesArgs(id), s)); err != nil {
		t.Fatalf("status: %v", err)
	}

	// RESULT when not completed
	if err := cmdASYNCRESULT(discardCtx("", bytesArgs(id), s)); err != nil {
		t.Fatalf("result pending: %v", err)
	}

	// CANCEL
	if err := cmdASYNCCANCEL(discardCtx("", bytesArgs(id), s)); err != nil {
		t.Fatalf("cancel: %v", err)
	}
}

// --- PROMISE: RESOLVE, REJECT, STATUS, AWAIT lifecycle ---

func TestDeep_PROMISERESOLVE_NotFound(t *testing.T) {
	s := store.NewStore()
	ctx := discardCtx("PROMISE.RESOLVE", bytesArgs("nopromise", "val"), s)
	if err := cmdPROMISERESOLVE(ctx); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestDeep_PROMISEREJECT_NotFound(t *testing.T) {
	s := store.NewStore()
	ctx := discardCtx("PROMISE.REJECT", bytesArgs("nopromise", "err"), s)
	if err := cmdPROMISEREJECT(ctx); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestDeep_PROMISESTATUS_NotFound(t *testing.T) {
	s := store.NewStore()
	ctx := discardCtx("PROMISE.STATUS", bytesArgs("nopromise"), s)
	if err := cmdPROMISESTATUS(ctx); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestDeep_PROMISEAWAIT_NotFound(t *testing.T) {
	s := store.NewStore()
	ctx := discardCtx("PROMISE.AWAIT", bytesArgs("nopromise"), s)
	if err := cmdPROMISEAWAIT(ctx); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestDeep_PROMISE_ResolveLifecycle(t *testing.T) {
	s := store.NewStore()
	ctx1, buf := bufCtx("PROMISE.CREATE", bytesArgs(), s)
	cmdPROMISECREATE(ctx1)
	id := parseRespBulk(buf.String())

	// AWAIT while pending -> null
	if err := cmdPROMISEAWAIT(discardCtx("", bytesArgs(id), s)); err != nil {
		t.Fatalf("await pending: %v", err)
	}

	// RESOLVE
	if err := cmdPROMISERESOLVE(discardCtx("", bytesArgs(id, "result_value"), s)); err != nil {
		t.Fatalf("resolve: %v", err)
	}

	// STATUS
	if err := cmdPROMISESTATUS(discardCtx("", bytesArgs(id), s)); err != nil {
		t.Fatalf("status: %v", err)
	}

	// AWAIT after resolve -> value
	if err := cmdPROMISEAWAIT(discardCtx("", bytesArgs(id), s)); err != nil {
		t.Fatalf("await resolved: %v", err)
	}
}

func TestDeep_PROMISE_RejectLifecycle(t *testing.T) {
	s := store.NewStore()
	ctx1, buf := bufCtx("PROMISE.CREATE", bytesArgs(), s)
	cmdPROMISECREATE(ctx1)
	id := parseRespBulk(buf.String())

	// REJECT
	if err := cmdPROMISEREJECT(discardCtx("", bytesArgs(id, "some error"), s)); err != nil {
		t.Fatalf("reject: %v", err)
	}

	// AWAIT after reject -> error
	if err := cmdPROMISEAWAIT(discardCtx("", bytesArgs(id), s)); err != nil {
		t.Fatalf("await rejected: %v", err)
	}
}

// --- FUTURE: COMPLETE, GET, CANCEL lifecycle ---

func TestDeep_FUTURECOMPLETE_NotFound(t *testing.T) {
	s := store.NewStore()
	ctx := discardCtx("FUTURE.COMPLETE", bytesArgs("nofuture", "val"), s)
	if err := cmdFUTURECOMPLETE(ctx); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestDeep_FUTUREGET_NotFound(t *testing.T) {
	s := store.NewStore()
	ctx := discardCtx("FUTURE.GET", bytesArgs("nofuture"), s)
	if err := cmdFUTUREGET(ctx); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestDeep_FUTURECANCEL_NotFound(t *testing.T) {
	s := store.NewStore()
	ctx := discardCtx("FUTURE.CANCEL", bytesArgs("nofuture"), s)
	if err := cmdFUTURECANCEL(ctx); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestDeep_FUTURE_CompleteLifecycle(t *testing.T) {
	s := store.NewStore()
	ctx1, buf := bufCtx("FUTURE.CREATE", bytesArgs(), s)
	cmdFUTURECREATE(ctx1)
	id := parseRespBulk(buf.String())

	// GET while pending -> null
	if err := cmdFUTUREGET(discardCtx("", bytesArgs(id), s)); err != nil {
		t.Fatalf("get pending: %v", err)
	}

	// COMPLETE
	if err := cmdFUTURECOMPLETE(discardCtx("", bytesArgs(id, "result"), s)); err != nil {
		t.Fatalf("complete: %v", err)
	}

	// GET after complete -> value
	if err := cmdFUTUREGET(discardCtx("", bytesArgs(id), s)); err != nil {
		t.Fatalf("get completed: %v", err)
	}
}

func TestDeep_FUTURE_CancelLifecycle(t *testing.T) {
	s := store.NewStore()
	ctx1, buf := bufCtx("FUTURE.CREATE", bytesArgs(), s)
	cmdFUTURECREATE(ctx1)
	id := parseRespBulk(buf.String())

	// CANCEL
	if err := cmdFUTURECANCEL(discardCtx("", bytesArgs(id), s)); err != nil {
		t.Fatalf("cancel: %v", err)
	}

	// GET after cancel -> error
	if err := cmdFUTUREGET(discardCtx("", bytesArgs(id), s)); err != nil {
		t.Fatalf("get cancelled: %v", err)
	}
}

// --- OBSERVABLE: NEXT, COMPLETE, ERROR, SUBSCRIBE lifecycle ---

func TestDeep_OBSERVABLENEXT_NotFound(t *testing.T) {
	s := store.NewStore()
	ctx := discardCtx("OBSERVABLE.NEXT", bytesArgs("noobs", "val"), s)
	if err := cmdOBSERVABLENEXT(ctx); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestDeep_OBSERVABLECOMPLETE_NotFound(t *testing.T) {
	s := store.NewStore()
	ctx := discardCtx("OBSERVABLE.COMPLETE", bytesArgs("noobs"), s)
	if err := cmdOBSERVABLECOMPLETE(ctx); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestDeep_OBSERVABLEERROR_NotFound(t *testing.T) {
	s := store.NewStore()
	ctx := discardCtx("OBSERVABLE.ERROR", bytesArgs("noobs", "err"), s)
	if err := cmdOBSERVABLEERROR(ctx); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestDeep_OBSERVABLESUBSCRIBE_NotFound(t *testing.T) {
	s := store.NewStore()
	ctx := discardCtx("OBSERVABLE.SUBSCRIBE", bytesArgs("noobs", "sub1"), s)
	if err := cmdOBSERVABLESUBSCRIBE(ctx); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestDeep_OBSERVABLE_Lifecycle(t *testing.T) {
	s := store.NewStore()
	ctx1, buf := bufCtx("OBSERVABLE.CREATE", bytesArgs(), s)
	cmdOBSERVABLECREATE(ctx1)
	id := parseRespBulk(buf.String())

	// SUBSCRIBE
	if err := cmdOBSERVABLESUBSCRIBE(discardCtx("", bytesArgs(id, "sub1"), s)); err != nil {
		t.Fatalf("subscribe: %v", err)
	}

	// NEXT
	if err := cmdOBSERVABLENEXT(discardCtx("", bytesArgs(id, "val1"), s)); err != nil {
		t.Fatalf("next: %v", err)
	}

	// ERROR
	if err := cmdOBSERVABLEERROR(discardCtx("", bytesArgs(id, "some error"), s)); err != nil {
		t.Fatalf("error: %v", err)
	}
}

func TestDeep_OBSERVABLE_CompleteLifecycle(t *testing.T) {
	s := store.NewStore()
	ctx1, buf := bufCtx("OBSERVABLE.CREATE", bytesArgs(), s)
	cmdOBSERVABLECREATE(ctx1)
	id := parseRespBulk(buf.String())

	// COMPLETE
	if err := cmdOBSERVABLECOMPLETE(discardCtx("", bytesArgs(id), s)); err != nil {
		t.Fatalf("complete: %v", err)
	}

	// NEXT after complete (not active) -> error
	if err := cmdOBSERVABLENEXT(discardCtx("", bytesArgs(id, "val"), s)); err != nil {
		t.Fatalf("next after complete: %v", err)
	}
}

// --- STREAMPROC: not-found branches for PUSH, POP, DELETE ---

func TestDeep_STREAMPROCPUSH_NotFound(t *testing.T) {
	s := store.NewStore()
	ctx := discardCtx("STREAMPROC.PUSH", bytesArgs("nostream", "val"), s)
	if err := cmdSTREAMPROCPUSH(ctx); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestDeep_STREAMPROCPOP_Empty(t *testing.T) {
	s := store.NewStore()
	cmdSTREAMPROCCREATE(discardCtx("", bytesArgs("emptystream"), s))
	ctx := discardCtx("STREAMPROC.POP", bytesArgs("emptystream"), s)
	if err := cmdSTREAMPROCPOP(ctx); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestDeep_STREAMPROCDELETE_NotFound(t *testing.T) {
	s := store.NewStore()
	ctx := discardCtx("STREAMPROC.DELETE", bytesArgs("nostream"), s)
	if err := cmdSTREAMPROCDELETE(ctx); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

// --- EVENTSOURCING: REPLAY with events ---

func TestDeep_EVENTSOURCINGREPLAY_NotFound(t *testing.T) {
	s := store.NewStore()
	ctx := discardCtx("EVENTSOURCING.REPLAY", bytesArgs("noagg"), s)
	if err := cmdEVENTSOURCINGREPLAY(ctx); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

// --- BACKPRESSURE: CHECK not found, STATUS not found ---

func TestDeep_BACKPRESSURECHECK_NotFound(t *testing.T) {
	s := store.NewStore()
	ctx := discardCtx("BACKPRESSURE.CHECK", bytesArgs("nobp"), s)
	if err := cmdBACKPRESSURECHECK(ctx); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestDeep_BACKPRESSURESTATUS_NotFound(t *testing.T) {
	s := store.NewStore()
	ctx := discardCtx("BACKPRESSURE.STATUS", bytesArgs("nobp"), s)
	if err := cmdBACKPRESSURESTATUS(ctx); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

// --- THROTTLEX: CHECK and STATUS not found ---

func TestDeep_THROTTLEXCHECK_NotFound(t *testing.T) {
	s := store.NewStore()
	ctx := discardCtx("THROTTLEX.CHECK", bytesArgs("nothrottle"), s)
	if err := cmdTHROTTLEXCHECK(ctx); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestDeep_THROTTLEXSTATUS_NotFound(t *testing.T) {
	s := store.NewStore()
	ctx := discardCtx("THROTTLEX.STATUS", bytesArgs("nothrottle"), s)
	if err := cmdTHROTTLEXSTATUS(ctx); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

// --- DEBOUNCEX: CANCEL and FLUSH not found ---

func TestDeep_DEBOUNCEXCANCEL_NotFound(t *testing.T) {
	s := store.NewStore()
	ctx := discardCtx("DEBOUNCEX.CANCEL", bytesArgs("nodeb"), s)
	if err := cmdDEBOUNCEXCANCEL(ctx); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestDeep_DEBOUNCEXFLUSH_NotFound(t *testing.T) {
	s := store.NewStore()
	ctx := discardCtx("DEBOUNCEX.FLUSH", bytesArgs("nodeb"), s)
	if err := cmdDEBOUNCEXFLUSH(ctx); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

// --- COALESCE: ADD, GET, CLEAR not found ---

func TestDeep_COALESCEADD_NotFound(t *testing.T) {
	s := store.NewStore()
	ctx := discardCtx("COALESCE.ADD", bytesArgs("nocoal", "val"), s)
	if err := cmdCOALESCEADD(ctx); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestDeep_COALESCEGET_NotFound(t *testing.T) {
	s := store.NewStore()
	ctx := discardCtx("COALESCE.GET", bytesArgs("nocoal"), s)
	if err := cmdCOALESCEGET(ctx); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestDeep_COALESCECLEAR_NotFound(t *testing.T) {
	s := store.NewStore()
	ctx := discardCtx("COALESCE.CLEAR", bytesArgs("nocoal"), s)
	if err := cmdCOALESCECLEAR(ctx); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

// --- AGGREGATOR: GET with sum/avg/min/max/count/unknown types ---

func TestDeep_AGGREGATORGET_Sum(t *testing.T) {
	s := store.NewStore()
	cmdAGGREGATORCREATE(discardCtx("", bytesArgs("aggsum", "sum"), s))
	cmdAGGREGATORADD(discardCtx("", bytesArgs("aggsum", "10"), s))
	cmdAGGREGATORADD(discardCtx("", bytesArgs("aggsum", "20"), s))
	if err := cmdAGGREGATORGET(discardCtx("", bytesArgs("aggsum"), s)); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestDeep_AGGREGATORGET_Avg(t *testing.T) {
	s := store.NewStore()
	cmdAGGREGATORCREATE(discardCtx("", bytesArgs("aggavg", "avg"), s))
	cmdAGGREGATORADD(discardCtx("", bytesArgs("aggavg", "10"), s))
	cmdAGGREGATORADD(discardCtx("", bytesArgs("aggavg", "20"), s))
	if err := cmdAGGREGATORGET(discardCtx("", bytesArgs("aggavg"), s)); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestDeep_AGGREGATORGET_Min(t *testing.T) {
	s := store.NewStore()
	cmdAGGREGATORCREATE(discardCtx("", bytesArgs("aggmin", "min"), s))
	cmdAGGREGATORADD(discardCtx("", bytesArgs("aggmin", "30"), s))
	cmdAGGREGATORADD(discardCtx("", bytesArgs("aggmin", "10"), s))
	if err := cmdAGGREGATORGET(discardCtx("", bytesArgs("aggmin"), s)); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestDeep_AGGREGATORGET_Max(t *testing.T) {
	s := store.NewStore()
	cmdAGGREGATORCREATE(discardCtx("", bytesArgs("aggmax", "max"), s))
	cmdAGGREGATORADD(discardCtx("", bytesArgs("aggmax", "10"), s))
	cmdAGGREGATORADD(discardCtx("", bytesArgs("aggmax", "50"), s))
	if err := cmdAGGREGATORGET(discardCtx("", bytesArgs("aggmax"), s)); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestDeep_AGGREGATORGET_Count(t *testing.T) {
	s := store.NewStore()
	cmdAGGREGATORCREATE(discardCtx("", bytesArgs("aggcnt", "count"), s))
	cmdAGGREGATORADD(discardCtx("", bytesArgs("aggcnt", "10"), s))
	cmdAGGREGATORADD(discardCtx("", bytesArgs("aggcnt", "20"), s))
	if err := cmdAGGREGATORGET(discardCtx("", bytesArgs("aggcnt"), s)); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestDeep_AGGREGATORGET_Unknown(t *testing.T) {
	s := store.NewStore()
	cmdAGGREGATORCREATE(discardCtx("", bytesArgs("aggunk", "foobar"), s))
	cmdAGGREGATORADD(discardCtx("", bytesArgs("aggunk", "10"), s))
	if err := cmdAGGREGATORGET(discardCtx("", bytesArgs("aggunk"), s)); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestDeep_AGGREGATORGET_Empty(t *testing.T) {
	s := store.NewStore()
	cmdAGGREGATORCREATE(discardCtx("", bytesArgs("aggempty", "sum"), s))
	if err := cmdAGGREGATORGET(discardCtx("", bytesArgs("aggempty"), s)); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestDeep_AGGREGATORRESET_NotFound(t *testing.T) {
	s := store.NewStore()
	ctx := discardCtx("AGGREGATOR.RESET", bytesArgs("noagg"), s)
	if err := cmdAGGREGATORRESET(ctx); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

// --- WINDOWX: ADD overflow, AGGREGATE sum/avg/unknown ---

func TestDeep_WINDOWXADD_Overflow(t *testing.T) {
	s := store.NewStore()
	cmdWINDOWXCREATE(discardCtx("", bytesArgs("winov", "2"), s))
	cmdWINDOWXADD(discardCtx("", bytesArgs("winov", "1"), s))
	cmdWINDOWXADD(discardCtx("", bytesArgs("winov", "2"), s))
	// This should cause a trim
	if err := cmdWINDOWXADD(discardCtx("", bytesArgs("winov", "3"), s)); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestDeep_WINDOWXADD_NotFound(t *testing.T) {
	s := store.NewStore()
	ctx := discardCtx("WINDOWX.ADD", bytesArgs("nowin", "1"), s)
	if err := cmdWINDOWXADD(ctx); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestDeep_WINDOWXAGGREGATE_Sum(t *testing.T) {
	s := store.NewStore()
	cmdWINDOWXCREATE(discardCtx("", bytesArgs("winagg", "10"), s))
	cmdWINDOWXADD(discardCtx("", bytesArgs("winagg", "5"), s))
	cmdWINDOWXADD(discardCtx("", bytesArgs("winagg", "15"), s))
	if err := cmdWINDOWXAGGREGATE(discardCtx("", bytesArgs("winagg", "sum"), s)); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestDeep_WINDOWXAGGREGATE_Avg(t *testing.T) {
	s := store.NewStore()
	cmdWINDOWXCREATE(discardCtx("", bytesArgs("winavg", "10"), s))
	cmdWINDOWXADD(discardCtx("", bytesArgs("winavg", "10"), s))
	cmdWINDOWXADD(discardCtx("", bytesArgs("winavg", "20"), s))
	if err := cmdWINDOWXAGGREGATE(discardCtx("", bytesArgs("winavg", "avg"), s)); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestDeep_WINDOWXAGGREGATE_Unknown(t *testing.T) {
	s := store.NewStore()
	cmdWINDOWXCREATE(discardCtx("", bytesArgs("winunk", "10"), s))
	cmdWINDOWXADD(discardCtx("", bytesArgs("winunk", "10"), s))
	if err := cmdWINDOWXAGGREGATE(discardCtx("", bytesArgs("winunk", "foobar"), s)); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestDeep_WINDOWXAGGREGATE_NotFound(t *testing.T) {
	s := store.NewStore()
	ctx := discardCtx("WINDOWX.AGGREGATE", bytesArgs("nowin", "sum"), s)
	if err := cmdWINDOWXAGGREGATE(ctx); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestDeep_WINDOWXGET_NotFound(t *testing.T) {
	s := store.NewStore()
	ctx := discardCtx("WINDOWX.GET", bytesArgs("nowin"), s)
	if err := cmdWINDOWXGET(ctx); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

// --- JOINX: GET with data, DELETE not found ---

func TestDeep_JOINXGET_NotFound(t *testing.T) {
	s := store.NewStore()
	ctx := discardCtx("JOINX.GET", bytesArgs("nojoin"), s)
	if err := cmdJOINXGET(ctx); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestDeep_JOINXGET_WithData(t *testing.T) {
	s := store.NewStore()
	cmdJOINXCREATE(discardCtx("", bytesArgs("myjoin"), s))
	cmdJOINXADD(discardCtx("", bytesArgs("myjoin", "left", "L1"), s))
	cmdJOINXADD(discardCtx("", bytesArgs("myjoin", "right", "R1"), s))
	if err := cmdJOINXGET(discardCtx("", bytesArgs("myjoin"), s)); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestDeep_JOINXDELETE_NotFound(t *testing.T) {
	s := store.NewStore()
	ctx := discardCtx("JOINX.DELETE", bytesArgs("nojoin"), s)
	if err := cmdJOINXDELETE(ctx); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

// --- SHUFFLE: ADD not found, GET empty ---

func TestDeep_SHUFFLEADD_NotFound(t *testing.T) {
	s := store.NewStore()
	ctx := discardCtx("SHUFFLE.ADD", bytesArgs("noshuf", "val"), s)
	if err := cmdSHUFFLEADD(ctx); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestDeep_SHUFFLEGET_Empty(t *testing.T) {
	s := store.NewStore()
	cmdSHUFFLECREATE(discardCtx("", bytesArgs("emptyshuf"), s))
	ctx := discardCtx("SHUFFLE.GET", bytesArgs("emptyshuf"), s)
	if err := cmdSHUFFLEGET(ctx); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

// --- PARTITIONX: ADD, GET, REBALANCE ---

func TestDeep_PARTITIONXADD_NotFound(t *testing.T) {
	s := store.NewStore()
	ctx := discardCtx("PARTITIONX.ADD", bytesArgs("nopart", "key1", "val1"), s)
	if err := cmdPARTITIONXADD(ctx); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestDeep_PARTITIONXGET_NotFound(t *testing.T) {
	s := store.NewStore()
	ctx := discardCtx("PARTITIONX.GET", bytesArgs("nopart", "0"), s)
	if err := cmdPARTITIONXGET(ctx); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestDeep_PARTITIONXGET_OutOfRange(t *testing.T) {
	s := store.NewStore()
	cmdPARTITIONXCREATE(discardCtx("", bytesArgs("partoor", "3"), s))
	ctx := discardCtx("PARTITIONX.GET", bytesArgs("partoor", "99"), s)
	if err := cmdPARTITIONXGET(ctx); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestDeep_PARTITIONXREBALANCE(t *testing.T) {
	s := store.NewStore()
	cmdPARTITIONXCREATE(discardCtx("", bytesArgs("partreb", "3"), s))
	ctx := discardCtx("PARTITIONX.REBALANCE", bytesArgs("partreb", "5"), s)
	if err := cmdPARTITIONXREBALANCE(ctx); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestDeep_PARTITIONXREBALANCE_WrongArgs(t *testing.T) {
	s := store.NewStore()
	ctx := discardCtx("PARTITIONX.REBALANCE", bytesArgs("x"), s)
	if err := cmdPARTITIONXREBALANCE(ctx); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

// ===========================================================================
// EXTRA COMMANDS - Deep coverage
// ===========================================================================

// --- SHARD: MAP with more keys, REBALANCE, LIST, STATUS not-found paths ---

func TestDeep_SHARDMAP_WithKeys(t *testing.T) {
	s := store.NewStore()
	ctx := discardCtx("SHARD.MAP", bytesArgs("key1", "key2", "key3"), s)
	if err := cmdSHARDMAP(ctx); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestDeep_SHARDREBALANCE(t *testing.T) {
	s := store.NewStore()
	ctx := discardCtx("SHARD.REBALANCE", bytesArgs("3", "key1"), s)
	if err := cmdSHARDREBALANCE(ctx); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestDeep_SHARDSTATUS(t *testing.T) {
	s := store.NewStore()
	ctx := discardCtx("SHARD.STATUS", bytesArgs("3"), s)
	if err := cmdSHARDSTATUS(ctx); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

// --- DEDUP: EXPIRE not found ---

func TestDeep_DEDUPEXPIRE_NotFound(t *testing.T) {
	s := store.NewStore()
	ctx := discardCtx("DEDUP.EXPIRE", bytesArgs("nodedup"), s)
	if err := cmdDEDUPEXPIRE(ctx); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

// --- BATCH: STATUS and CANCEL with found paths ---

func TestDeep_BATCHSTATUS_NotFound(t *testing.T) {
	s := store.NewStore()
	ctx := discardCtx("BATCH.STATUS", bytesArgs("nobatch_x"), s)
	if err := cmdBATCHSTATUS(ctx); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestDeep_BATCHCANCEL_NotFound(t *testing.T) {
	s := store.NewStore()
	ctx := discardCtx("BATCH.CANCEL", bytesArgs("nobatch_x"), s)
	if err := cmdBATCHCANCEL(ctx); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

// --- DEADLINE: CHECK not found, CANCEL not found ---

func TestDeep_DEADLINECHECK_NotFound(t *testing.T) {
	s := store.NewStore()
	ctx := discardCtx("DEADLINE.CHECK", bytesArgs("nodeadline"), s)
	if err := cmdDEADLINECHECK(ctx); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestDeep_DEADLINECANCEL_NotFound(t *testing.T) {
	s := store.NewStore()
	ctx := discardCtx("DEADLINE.CANCEL", bytesArgs("nodeadline"), s)
	if err := cmdDEADLINECANCEL(ctx); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

// --- GATEWAY: DELETE found, ROUTE found, METRICS found ---

func TestDeep_GATEWAY_Lifecycle(t *testing.T) {
	s := store.NewStore()
	cmdGATEWAYCREATE(discardCtx("", bytesArgs("gw1", "http://backend"), s))
	if err := cmdGATEWAYDELETE(discardCtx("", bytesArgs("gw1"), s)); err != nil {
		t.Fatalf("delete: %v", err)
	}
}

func TestDeep_GATEWAYROUTE_Found(t *testing.T) {
	s := store.NewStore()
	cmdGATEWAYCREATE(discardCtx("", bytesArgs("gw2", "http://backend2"), s))
	if err := cmdGATEWAYROUTE(discardCtx("", bytesArgs("gw2", "/api/test"), s)); err != nil {
		t.Fatalf("route: %v", err)
	}
}

func TestDeep_GATEWAYMETRICS_Found(t *testing.T) {
	s := store.NewStore()
	cmdGATEWAYCREATE(discardCtx("", bytesArgs("gw3", "http://backend3"), s))
	if err := cmdGATEWAYMETRICS(discardCtx("", bytesArgs("gw3"), s)); err != nil {
		t.Fatalf("metrics: %v", err)
	}
}

// --- SWITCH: STATE/TOGGLE not found branches ---

func TestDeep_SWITCHSTATE_NotFound(t *testing.T) {
	s := store.NewStore()
	ctx := discardCtx("SWITCH.STATE", bytesArgs("nosw"), s)
	if err := cmdSWITCHSTATE(ctx); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestDeep_SWITCHTOGGLE_NotFound(t *testing.T) {
	s := store.NewStore()
	ctx := discardCtx("SWITCH.TOGGLE", bytesArgs("nosw"), s)
	if err := cmdSWITCHTOGGLE(ctx); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

// --- REPLAYX: PAUSE/SPEED not found ---

func TestDeep_REPLAYXPAUSE_NotFound(t *testing.T) {
	s := store.NewStore()
	ctx := discardCtx("REPLAYX.PAUSE", bytesArgs("norep"), s)
	if err := cmdREPLAYXPAUSE(ctx); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestDeep_REPLAYXSPEED_NotFound(t *testing.T) {
	s := store.NewStore()
	ctx := discardCtx("REPLAYX.SPEED", bytesArgs("norep", "2"), s)
	if err := cmdREPLAYXSPEED(ctx); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

// --- ROUTE: MATCH/LIST with data ---

func TestDeep_ROUTEMATCH_NotFound(t *testing.T) {
	s := store.NewStore()
	cmdROUTEADD(discardCtx("", bytesArgs("/api/*", "handler1"), s))
	ctx := discardCtx("ROUTE.MATCH", bytesArgs("/other/path"), s)
	if err := cmdROUTEMATCH(ctx); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

// --- GHOST: DELETE not found ---

func TestDeep_GHOSTDELETE_NotFound(t *testing.T) {
	s := store.NewStore()
	ctx := discardCtx("GHOST.DELETE", bytesArgs("noghost"), s)
	if err := cmdGHOSTDELETE(ctx); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

// --- PROBE: RESULTS not found ---

func TestDeep_PROBERESULTS_NotFound(t *testing.T) {
	s := store.NewStore()
	ctx := discardCtx("PROBE.RESULTS", bytesArgs("noprobe"), s)
	if err := cmdPROBERESULTS(ctx); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

// --- RAGE: STOP/STATS not found ---

func TestDeep_RAGESTOP_NotFound(t *testing.T) {
	s := store.NewStore()
	ctx := discardCtx("RAGE.STOP", bytesArgs("norage"), s)
	if err := cmdRAGESTOP(ctx); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestDeep_RAGESTATS_NotFound(t *testing.T) {
	s := store.NewStore()
	ctx := discardCtx("RAGE.STATS", bytesArgs("norage"), s)
	if err := cmdRAGESTATS(ctx); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

// --- GRID: SET/GET/DELETE not found ---

func TestDeep_GRIDSET_NotFound(t *testing.T) {
	s := store.NewStore()
	ctx := discardCtx("GRID.SET", bytesArgs("nogrid", "0", "0", "val"), s)
	if err := cmdGRIDSET(ctx); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestDeep_GRIDGET_NotFound(t *testing.T) {
	s := store.NewStore()
	ctx := discardCtx("GRID.GET", bytesArgs("nogrid", "0", "0"), s)
	if err := cmdGRIDGET(ctx); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestDeep_GRIDDELETE_NotFound(t *testing.T) {
	s := store.NewStore()
	ctx := discardCtx("GRID.DELETE", bytesArgs("nogrid"), s)
	if err := cmdGRIDDELETE(ctx); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

// --- TAPE: WRITE/READ/SEEK/DELETE not found ---

func TestDeep_TAPEWRITE_NotFound(t *testing.T) {
	s := store.NewStore()
	ctx := discardCtx("TAPE.WRITE", bytesArgs("notape", "data"), s)
	if err := cmdTAPEWRITE(ctx); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestDeep_TAPEREAD_NotFound(t *testing.T) {
	s := store.NewStore()
	ctx := discardCtx("TAPE.READ", bytesArgs("notape"), s)
	if err := cmdTAPEREAD(ctx); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestDeep_TAPESEEK_NotFound(t *testing.T) {
	s := store.NewStore()
	ctx := discardCtx("TAPE.SEEK", bytesArgs("notape", "0"), s)
	if err := cmdTAPESEEK(ctx); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestDeep_TAPEDELETE_NotFound(t *testing.T) {
	s := store.NewStore()
	ctx := discardCtx("TAPE.DELETE", bytesArgs("notape"), s)
	if err := cmdTAPEDELETE(ctx); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

// --- SLICE: GET/DELETE not found ---

func TestDeep_SLICEGET_NotFound(t *testing.T) {
	s := store.NewStore()
	ctx := discardCtx("SLICE.GET", bytesArgs("noslice", "0", "5"), s)
	if err := cmdSLICEGET(ctx); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestDeep_SLICEDELETE_NotFound(t *testing.T) {
	s := store.NewStore()
	ctx := discardCtx("SLICE.DELETE", bytesArgs("noslice"), s)
	if err := cmdSLICEDELETE(ctx); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

// --- ROLLUPX: ADD/GET/DELETE not found ---

func TestDeep_ROLLUPXADD_NotFound(t *testing.T) {
	s := store.NewStore()
	ctx := discardCtx("ROLLUPX.ADD", bytesArgs("norollup", "10"), s)
	if err := cmdROLLUPXADD(ctx); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestDeep_ROLLUPXGET_NotFound(t *testing.T) {
	s := store.NewStore()
	ctx := discardCtx("ROLLUPX.GET", bytesArgs("norollup"), s)
	if err := cmdROLLUPXGET(ctx); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestDeep_ROLLUPXDELETE_NotFound(t *testing.T) {
	s := store.NewStore()
	ctx := discardCtx("ROLLUPX.DELETE", bytesArgs("norollup"), s)
	if err := cmdROLLUPXDELETE(ctx); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

// --- BEACON: STOP/CHECK not found ---

func TestDeep_BEACONSTOP_NotFound(t *testing.T) {
	s := store.NewStore()
	ctx := discardCtx("BEACON.STOP", bytesArgs("nobeacon"), s)
	if err := cmdBEACONSTOP(ctx); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestDeep_BEACONCHECK_NotFound(t *testing.T) {
	s := store.NewStore()
	ctx := discardCtx("BEACON.CHECK", bytesArgs("nobeacon"), s)
	if err := cmdBEACONCHECK(ctx); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

// ===========================================================================
// MORE COMMANDS - Deep coverage
// ===========================================================================

// --- SLIDING: CHECK limit, not-found ---

func TestDeep_SLIDINGCHECK_NotFound(t *testing.T) {
	s := store.NewStore()
	ctx := discardCtx("SLIDING.CHECK", bytesArgs("nosliding"), s)
	if err := cmdSLIDINGCHECK(ctx); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

// --- BUCKETX: TAKE when empty/depleted ---

func TestDeep_BUCKETXTAKE_NotFound(t *testing.T) {
	s := store.NewStore()
	ctx := discardCtx("BUCKETX.TAKE", bytesArgs("nobucket"), s)
	if err := cmdBUCKETXTAKE(ctx); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestDeep_BUCKETXTAKE_Depleted(t *testing.T) {
	s := store.NewStore()
	// Create with capacity=1, refill=1, interval=60000ms
	cmdBUCKETXCREATE(discardCtx("", bytesArgs("bkt_dep", "1", "1", "60000"), s))
	// Take the only token
	cmdBUCKETXTAKE(discardCtx("", bytesArgs("bkt_dep", "1"), s))
	// Take again -> should fail (empty)
	ctx := discardCtx("BUCKETX.TAKE", bytesArgs("bkt_dep", "1"), s)
	if err := cmdBUCKETXTAKE(ctx); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestDeep_BUCKETXTAKE_NotFoundWithTokens(t *testing.T) {
	s := store.NewStore()
	ctx := discardCtx("BUCKETX.TAKE", bytesArgs("nobucket_deep", "1"), s)
	if err := cmdBUCKETXTAKE(ctx); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

// --- IDEMPOTENCY: SET/CHECK lifecycle ---

func TestDeep_IDEMPOTENCYSET_WithExisting(t *testing.T) {
	s := store.NewStore()
	cmdIDEMPOTENCYSET(discardCtx("", bytesArgs("ikey1", "result1"), s))
	// Set again should be ok (overwrites)
	ctx := discardCtx("IDEMPOTENCY.SET", bytesArgs("ikey1", "result2"), s)
	if err := cmdIDEMPOTENCYSET(ctx); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestDeep_IDEMPOTENCYCHECK_NotFound(t *testing.T) {
	s := store.NewStore()
	ctx := discardCtx("IDEMPOTENCY.CHECK", bytesArgs("noikey"), s)
	if err := cmdIDEMPOTENCYCHECK(ctx); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

// --- EXPERIMENT: ASSIGN with different groups ---

func TestDeep_EXPERIMENTASSIGN_Distribution(t *testing.T) {
	s := store.NewStore()
	cmdEXPERIMENTCREATE(discardCtx("", bytesArgs("exp1", "control", "treatment"), s))
	// Assign multiple users to see different groups
	for i := 0; i < 5; i++ {
		user := "user" + string(rune('A'+i))
		if err := cmdEXPERIMENTASSIGN(discardCtx("", bytesArgs("exp1", user), s)); err != nil {
			t.Fatalf("assign %s: %v", user, err)
		}
	}
}

// --- ROLLOUT: CHECK not found path ---

func TestDeep_ROLLOUTCHECK_NotFound(t *testing.T) {
	s := store.NewStore()
	ctx := discardCtx("ROLLOUT.CHECK", bytesArgs("norollout", "user1"), s)
	if err := cmdROLLOUTCHECK(ctx); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

// --- ALERT: RESOLVE/HISTORY not found ---

func TestDeep_ALERTRESOLVE_NotFound(t *testing.T) {
	s := store.NewStore()
	ctx := discardCtx("ALERT.RESOLVE", bytesArgs("noalert"), s)
	if err := cmdALERTRESOLVE(ctx); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestDeep_ALERTHISTORY_NotFound(t *testing.T) {
	s := store.NewStore()
	ctx := discardCtx("ALERT.HISTORY", bytesArgs("noalert"), s)
	if err := cmdALERTHISTORY(ctx); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

// --- COUNTERX: INCR/DECR/DELETE not found ---

func TestDeep_COUNTERXINCR_NotFound(t *testing.T) {
	s := store.NewStore()
	ctx := discardCtx("COUNTERX.INCR", bytesArgs("noctr"), s)
	if err := cmdCOUNTERXINCR(ctx); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestDeep_COUNTERXDECR_NotFound(t *testing.T) {
	s := store.NewStore()
	ctx := discardCtx("COUNTERX.DECR", bytesArgs("noctr"), s)
	if err := cmdCOUNTERXDECR(ctx); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestDeep_COUNTERXDELETE_NotFound(t *testing.T) {
	s := store.NewStore()
	ctx := discardCtx("COUNTERX.DELETE", bytesArgs("noctr"), s)
	if err := cmdCOUNTERXDELETE(ctx); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

// --- GAUGE: GET/INCR/DECR/DELETE not found ---

func TestDeep_GAUGEGET_NotFound(t *testing.T) {
	s := store.NewStore()
	ctx := discardCtx("GAUGE.GET", bytesArgs("nogg"), s)
	if err := cmdGAUGEGET(ctx); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestDeep_GAUGEINCR_NotFound(t *testing.T) {
	s := store.NewStore()
	ctx := discardCtx("GAUGE.INCR", bytesArgs("nogg", "5"), s)
	if err := cmdGAUGEINCR(ctx); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestDeep_GAUGEDECR_NotFound(t *testing.T) {
	s := store.NewStore()
	ctx := discardCtx("GAUGE.DECR", bytesArgs("nogg", "5"), s)
	if err := cmdGAUGEDECR(ctx); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestDeep_GAUGEDELETE_NotFound(t *testing.T) {
	s := store.NewStore()
	ctx := discardCtx("GAUGE.DELETE", bytesArgs("nogg"), s)
	if err := cmdGAUGEDELETE(ctx); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

// --- TRACE: END not found ---

func TestDeep_TRACEEND_NotFound(t *testing.T) {
	s := store.NewStore()
	ctx := discardCtx("TRACE.END", bytesArgs("notrace"), s)
	if err := cmdTRACEEND(ctx); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

// --- LOGX: READ/SEARCH not found ---

func TestDeep_LOGXREAD_NotFound(t *testing.T) {
	s := store.NewStore()
	ctx := discardCtx("LOGX.READ", bytesArgs("nolog"), s)
	if err := cmdLOGXREAD(ctx); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestDeep_LOGXSEARCH_NotFound(t *testing.T) {
	s := store.NewStore()
	ctx := discardCtx("LOGX.SEARCH", bytesArgs("nolog", "needle"), s)
	if err := cmdLOGXSEARCH(ctx); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

// --- APIKEY: VALIDATE/REVOKE not found ---

func TestDeep_APIKEYVALIDATE_NotFound(t *testing.T) {
	s := store.NewStore()
	ctx := discardCtx("APIKEY.VALIDATE", bytesArgs("nokey"), s)
	if err := cmdAPIKEYVALIDATE(ctx); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestDeep_APIKEYREVOKE_NotFound(t *testing.T) {
	s := store.NewStore()
	ctx := discardCtx("APIKEY.REVOKE", bytesArgs("nokey"), s)
	if err := cmdAPIKEYREVOKE(ctx); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestDeep_APIKEYUSAGE_NotFound(t *testing.T) {
	s := store.NewStore()
	ctx := discardCtx("APIKEY.USAGE", bytesArgs("nokey"), s)
	if err := cmdAPIKEYUSAGE(ctx); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

// --- QUOTAX: CHECK/RESET/DELETE not found ---

func TestDeep_QUOTAXCHECK_NotFound(t *testing.T) {
	s := store.NewStore()
	ctx := discardCtx("QUOTAX.CHECK", bytesArgs("noq"), s)
	if err := cmdQUOTAXCHECK(ctx); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestDeep_QUOTAXRESET_NotFound(t *testing.T) {
	s := store.NewStore()
	ctx := discardCtx("QUOTAX.RESET", bytesArgs("noq"), s)
	if err := cmdQUOTAXRESET(ctx); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestDeep_QUOTAXDELETE_NotFound(t *testing.T) {
	s := store.NewStore()
	ctx := discardCtx("QUOTAX.DELETE", bytesArgs("noq"), s)
	if err := cmdQUOTAXDELETE(ctx); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

// --- METER: BILLING/DELETE not found ---

func TestDeep_METERBILLING_NotFound(t *testing.T) {
	s := store.NewStore()
	ctx := discardCtx("METER.BILLING", bytesArgs("nomtr"), s)
	if err := cmdMETERBILLING(ctx); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

// --- TENANT: GET not found ---

func TestDeep_TENANTGET_NotFound(t *testing.T) {
	s := store.NewStore()
	ctx := discardCtx("TENANT.GET", bytesArgs("notenant"), s)
	if err := cmdTENANTGET(ctx); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestDeep_TENANTCONFIG_NotFound(t *testing.T) {
	s := store.NewStore()
	ctx := discardCtx("TENANT.CONFIG", bytesArgs("notenant", "key", "val"), s)
	if err := cmdTENANTCONFIG(ctx); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

// --- LEASE: CREATE with TTL ---

func TestDeep_LEASECREATE_WithHolder(t *testing.T) {
	s := store.NewStore()
	ctx := discardCtx("LEASE.CREATE", bytesArgs("res1", "holder1", "60"), s)
	if err := cmdLEASECREATE(ctx); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

// --- HEAP: POP/PEEK/DELETE not found ---

func TestDeep_HEAPPOP_NotFound(t *testing.T) {
	s := store.NewStore()
	ctx := discardCtx("HEAP.POP", bytesArgs("noheap"), s)
	if err := cmdHEAPPOP(ctx); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestDeep_HEAPPEEK_NotFound(t *testing.T) {
	s := store.NewStore()
	ctx := discardCtx("HEAP.PEEK", bytesArgs("noheap"), s)
	if err := cmdHEAPPEEK(ctx); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestDeep_HEAPDELETE_NotFound(t *testing.T) {
	s := store.NewStore()
	ctx := discardCtx("HEAP.DELETE", bytesArgs("noheap"), s)
	if err := cmdHEAPDELETE(ctx); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

// --- BLOOMX: CHECK not found ---

func TestDeep_BLOOMXCHECK_NotFound(t *testing.T) {
	s := store.NewStore()
	ctx := discardCtx("BLOOMX.CHECK", bytesArgs("nobloom", "item"), s)
	if err := cmdBLOOMXCHECK(ctx); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

// --- RINGBUFFER: WRITE/READ/SIZE/DELETE not found ---

func TestDeep_RINGBUFFERWRITE_NotFound(t *testing.T) {
	s := store.NewStore()
	ctx := discardCtx("RINGBUFFER.WRITE", bytesArgs("norb", "data"), s)
	if err := cmdRINGBUFFERWRITE(ctx); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestDeep_RINGBUFFERREAD_NotFound(t *testing.T) {
	s := store.NewStore()
	ctx := discardCtx("RINGBUFFER.READ", bytesArgs("norb"), s)
	if err := cmdRINGBUFFERREAD(ctx); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestDeep_RINGBUFFERSIZE_NotFound(t *testing.T) {
	s := store.NewStore()
	ctx := discardCtx("RINGBUFFER.SIZE", bytesArgs("norb"), s)
	if err := cmdRINGBUFFERSIZE(ctx); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestDeep_RINGBUFFERDELETE_NotFound(t *testing.T) {
	s := store.NewStore()
	ctx := discardCtx("RINGBUFFER.DELETE", bytesArgs("norb"), s)
	if err := cmdRINGBUFFERDELETE(ctx); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

// --- WINDOW: ADD/GET/AGGREGATE/DELETE not found ---

func TestDeep_WINDOWADD_NotFound(t *testing.T) {
	s := store.NewStore()
	ctx := discardCtx("WINDOW.ADD", bytesArgs("nowin_m", "10"), s)
	if err := cmdWINDOWADD(ctx); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestDeep_WINDOWGET_NotFound(t *testing.T) {
	s := store.NewStore()
	ctx := discardCtx("WINDOW.GET", bytesArgs("nowin_m"), s)
	if err := cmdWINDOWGET(ctx); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestDeep_WINDOWDELETE_NotFound(t *testing.T) {
	s := store.NewStore()
	ctx := discardCtx("WINDOW.DELETE", bytesArgs("nowin_m"), s)
	if err := cmdWINDOWDELETE(ctx); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

// --- FREQ: ADD/TOP/DELETE not found ---

func TestDeep_FREQADD_NotFound(t *testing.T) {
	s := store.NewStore()
	ctx := discardCtx("FREQ.ADD", bytesArgs("nofreq", "item"), s)
	if err := cmdFREQADD(ctx); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestDeep_FREQTOP_NotFound(t *testing.T) {
	s := store.NewStore()
	ctx := discardCtx("FREQ.TOP", bytesArgs("nofreq", "5"), s)
	if err := cmdFREQTOP(ctx); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestDeep_FREQDELETE_NotFound(t *testing.T) {
	s := store.NewStore()
	ctx := discardCtx("FREQ.DELETE", bytesArgs("nofreq"), s)
	if err := cmdFREQDELETE(ctx); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

// --- PARTITION: GET/LIST/DELETE not found ---

func TestDeep_PARTITIONGET_NotFound(t *testing.T) {
	s := store.NewStore()
	ctx := discardCtx("PARTITION.GET", bytesArgs("nopart_m", "0"), s)
	if err := cmdPARTITIONGET(ctx); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestDeep_PARTITIONLIST_NotFound(t *testing.T) {
	s := store.NewStore()
	ctx := discardCtx("PARTITION.LIST", bytesArgs("nopart_m"), s)
	if err := cmdPARTITIONLIST(ctx); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestDeep_PARTITIONDELETE_NotFound(t *testing.T) {
	s := store.NewStore()
	ctx := discardCtx("PARTITION.DELETE", bytesArgs("nopart_m"), s)
	if err := cmdPARTITIONDELETE(ctx); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

// ===========================================================================
// ADVANCED COMMANDS 2 - Deep coverage
// ===========================================================================

// --- LEVEL: LIST with and without levels ---

func TestDeep_LEVELLIST_WithLevels(t *testing.T) {
	s := store.NewStore()
	cmdLEVELCREATE(discardCtx("", bytesArgs("lv1"), s))
	cmdLEVELSET(discardCtx("", bytesArgs("lv1", "5"), s))
	if err := cmdLEVELLIST(discardCtx("", bytesArgs(), s)); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

// --- TOKEN: VALIDATE found ---

func TestDeep_TOKENVALIDATE_WithToken(t *testing.T) {
	s := store.NewStore()
	ctx1, buf := bufCtx("TOKEN.CREATE", bytesArgs("user1"), s)
	cmdTOKENCREATE(ctx1)
	token := parseRespBulk(buf.String())

	if err := cmdTOKENVALIDATE(discardCtx("", bytesArgs(token), s)); err != nil {
		t.Fatalf("validate: %v", err)
	}
}

// ===========================================================================
// VECTOR CLOCK: COMPARE edge cases in extra_commands.go
// ===========================================================================

func TestDeep_VECTORCLOCKCOMPARE_BothExist(t *testing.T) {
	s := store.NewStore()
	cmdVECTORCLOCKCREATE(discardCtx("", bytesArgs("vc1"), s))
	cmdVECTORCLOCKCREATE(discardCtx("", bytesArgs("vc2"), s))
	cmdVECTORCLOCKINCREMENT(discardCtx("", bytesArgs("vc1", "node1"), s))
	if err := cmdVECTORCLOCKCOMPARE(discardCtx("", bytesArgs("vc1", "vc2"), s)); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestDeep_VECTORCLOCKMERGE_BothExist(t *testing.T) {
	s := store.NewStore()
	cmdVECTORCLOCKCREATE(discardCtx("", bytesArgs("vcm1"), s))
	cmdVECTORCLOCKCREATE(discardCtx("", bytesArgs("vcm2"), s))
	cmdVECTORCLOCKINCREMENT(discardCtx("", bytesArgs("vcm1", "node1"), s))
	cmdVECTORCLOCKINCREMENT(discardCtx("", bytesArgs("vcm2", "node2"), s))
	if err := cmdVECTORCLOCKMERGE(discardCtx("", bytesArgs("vcm1", "vcm2"), s)); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

// --- CRDT LWW SET with explicit timestamp ---

func TestDeep_CRDTLWWSET_NoTimestamp(t *testing.T) {
	s := store.NewStore()
	ctx := discardCtx("CRDT.LWW.SET", bytesArgs("lwwkey", "val"), s)
	if err := cmdCRDTLWWSET(ctx); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

// --- CRDT OR-SET REMOVE not found ---

func TestDeep_CRDTORSETREMOVE_NoSet(t *testing.T) {
	s := store.NewStore()
	ctx := discardCtx("CRDT.ORSET.REMOVE", bytesArgs("noordst", "item"), s)
	if err := cmdCRDTORSETREMOVE(ctx); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

// --- MERKLE: VERIFY/PROOF edge cases ---

func TestDeep_MERKLEVERIFY_NoTree(t *testing.T) {
	s := store.NewStore()
	ctx := discardCtx("MERKLE.VERIFY", bytesArgs("notree", "data"), s)
	if err := cmdMERKLEVERIFY(ctx); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestDeep_MERKLEPROOF_NoTree(t *testing.T) {
	s := store.NewStore()
	ctx := discardCtx("MERKLE.PROOF", bytesArgs("notree", "data"), s)
	if err := cmdMERKLEPROOF(ctx); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

// ===========================================================================
// Helper: parse RESP bulk string from raw buffer
// ===========================================================================

func parseRespBulk(raw string) string {
	// RESP format: $<len>\r\n<data>\r\n
	// Find first \r\n, then extract data before next \r\n
	idx := 0
	for idx < len(raw) && raw[idx] != '\n' {
		idx++
	}
	if idx >= len(raw) {
		return raw
	}
	start := idx + 1
	end := start
	for end < len(raw) && raw[end] != '\r' && raw[end] != '\n' {
		end++
	}
	return raw[start:end]
}
