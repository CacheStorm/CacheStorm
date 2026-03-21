package command

import (
	"testing"

	"github.com/cachestorm/cachestorm/internal/store"
)

// ======================================================================
// FILTER COMMANDS
// ======================================================================

func TestCmdFILTERCREATE_Success(t *testing.T) {
	s := store.NewStore()
	ctx := discardCtx("FILTER.CREATE", bytesArgs("myfilter", "x > 10"), s)
	if err := cmdFILTERCREATE(ctx); err != nil {
		t.Fatal(err)
	}
}

func TestCmdFILTERCREATE_TooFewArgs(t *testing.T) {
	s := store.NewStore()
	ctx := discardCtx("FILTER.CREATE", bytesArgs("myfilter"), s)
	if err := cmdFILTERCREATE(ctx); err != nil {
		t.Fatal(err)
	}
}

func TestCmdFILTERDELETE_Success(t *testing.T) {
	s := store.NewStore()
	// Create first
	ctx := discardCtx("FILTER.CREATE", bytesArgs("delfilter", "x > 5"), s)
	cmdFILTERCREATE(ctx)
	// Delete
	ctx = discardCtx("FILTER.DELETE", bytesArgs("delfilter"), s)
	if err := cmdFILTERDELETE(ctx); err != nil {
		t.Fatal(err)
	}
}

func TestCmdFILTERDELETE_NotFound(t *testing.T) {
	s := store.NewStore()
	ctx := discardCtx("FILTER.DELETE", bytesArgs("nonexistent"), s)
	if err := cmdFILTERDELETE(ctx); err != nil {
		t.Fatal(err)
	}
}

func TestCmdFILTERDELETE_TooFewArgs(t *testing.T) {
	s := store.NewStore()
	ctx := discardCtx("FILTER.DELETE", bytesArgs(), s)
	if err := cmdFILTERDELETE(ctx); err != nil {
		t.Fatal(err)
	}
}

func TestCmdFILTERAPPLY_Success(t *testing.T) {
	s := store.NewStore()
	ctx := discardCtx("FILTER.APPLY", bytesArgs("myfilter", "somedata"), s)
	if err := cmdFILTERAPPLY(ctx); err != nil {
		t.Fatal(err)
	}
}

func TestCmdFILTERAPPLY_TooFewArgs(t *testing.T) {
	s := store.NewStore()
	ctx := discardCtx("FILTER.APPLY", bytesArgs("myfilter"), s)
	if err := cmdFILTERAPPLY(ctx); err != nil {
		t.Fatal(err)
	}
}

func TestCmdFILTERLIST(t *testing.T) {
	s := store.NewStore()
	ctx := discardCtx("FILTER.LIST", bytesArgs(), s)
	if err := cmdFILTERLIST(ctx); err != nil {
		t.Fatal(err)
	}
}

// ======================================================================
// TRANSFORM COMMANDS
// ======================================================================

func TestCmdTRANSFORMCREATE_Success(t *testing.T) {
	s := store.NewStore()
	ctx := discardCtx("TRANSFORM.CREATE", bytesArgs("mytrans", "uppercase"), s)
	if err := cmdTRANSFORMCREATE(ctx); err != nil {
		t.Fatal(err)
	}
}

func TestCmdTRANSFORMCREATE_TooFewArgs(t *testing.T) {
	s := store.NewStore()
	ctx := discardCtx("TRANSFORM.CREATE", bytesArgs("mytrans"), s)
	if err := cmdTRANSFORMCREATE(ctx); err != nil {
		t.Fatal(err)
	}
}

func TestCmdTRANSFORMDELETE_Success(t *testing.T) {
	s := store.NewStore()
	ctx := discardCtx("TRANSFORM.CREATE", bytesArgs("deltrans", "upper"), s)
	cmdTRANSFORMCREATE(ctx)
	ctx = discardCtx("TRANSFORM.DELETE", bytesArgs("deltrans"), s)
	if err := cmdTRANSFORMDELETE(ctx); err != nil {
		t.Fatal(err)
	}
}

func TestCmdTRANSFORMDELETE_NotFound(t *testing.T) {
	s := store.NewStore()
	ctx := discardCtx("TRANSFORM.DELETE", bytesArgs("nonexistent"), s)
	if err := cmdTRANSFORMDELETE(ctx); err != nil {
		t.Fatal(err)
	}
}

func TestCmdTRANSFORMDELETE_TooFewArgs(t *testing.T) {
	s := store.NewStore()
	ctx := discardCtx("TRANSFORM.DELETE", bytesArgs(), s)
	if err := cmdTRANSFORMDELETE(ctx); err != nil {
		t.Fatal(err)
	}
}

func TestCmdTRANSFORMAPPLY_Success(t *testing.T) {
	s := store.NewStore()
	ctx := discardCtx("TRANSFORM.CREATE", bytesArgs("applytrans", "upper"), s)
	cmdTRANSFORMCREATE(ctx)
	ctx = discardCtx("TRANSFORM.APPLY", bytesArgs("applytrans", "hello"), s)
	if err := cmdTRANSFORMAPPLY(ctx); err != nil {
		t.Fatal(err)
	}
}

func TestCmdTRANSFORMAPPLY_NotFound(t *testing.T) {
	s := store.NewStore()
	ctx := discardCtx("TRANSFORM.APPLY", bytesArgs("nonexistent", "hello"), s)
	if err := cmdTRANSFORMAPPLY(ctx); err != nil {
		t.Fatal(err)
	}
}

func TestCmdTRANSFORMAPPLY_TooFewArgs(t *testing.T) {
	s := store.NewStore()
	ctx := discardCtx("TRANSFORM.APPLY", bytesArgs("mytrans"), s)
	if err := cmdTRANSFORMAPPLY(ctx); err != nil {
		t.Fatal(err)
	}
}

func TestCmdTRANSFORMLIST(t *testing.T) {
	s := store.NewStore()
	ctx := discardCtx("TRANSFORM.LIST", bytesArgs(), s)
	if err := cmdTRANSFORMLIST(ctx); err != nil {
		t.Fatal(err)
	}
}

// ======================================================================
// ENRICH COMMANDS
// ======================================================================

func TestCmdENRICHCREATE_Success(t *testing.T) {
	s := store.NewStore()
	ctx := discardCtx("ENRICH.CREATE", bytesArgs("myenricher", "datasource"), s)
	if err := cmdENRICHCREATE(ctx); err != nil {
		t.Fatal(err)
	}
}

func TestCmdENRICHCREATE_TooFewArgs(t *testing.T) {
	s := store.NewStore()
	ctx := discardCtx("ENRICH.CREATE", bytesArgs("myenricher"), s)
	if err := cmdENRICHCREATE(ctx); err != nil {
		t.Fatal(err)
	}
}

func TestCmdENRICHDELETE_Success(t *testing.T) {
	s := store.NewStore()
	ctx := discardCtx("ENRICH.CREATE", bytesArgs("delenrich", "src"), s)
	cmdENRICHCREATE(ctx)
	ctx = discardCtx("ENRICH.DELETE", bytesArgs("delenrich"), s)
	if err := cmdENRICHDELETE(ctx); err != nil {
		t.Fatal(err)
	}
}

func TestCmdENRICHDELETE_NotFound(t *testing.T) {
	s := store.NewStore()
	ctx := discardCtx("ENRICH.DELETE", bytesArgs("nonexistent"), s)
	if err := cmdENRICHDELETE(ctx); err != nil {
		t.Fatal(err)
	}
}

func TestCmdENRICHDELETE_TooFewArgs(t *testing.T) {
	s := store.NewStore()
	ctx := discardCtx("ENRICH.DELETE", bytesArgs(), s)
	if err := cmdENRICHDELETE(ctx); err != nil {
		t.Fatal(err)
	}
}

func TestCmdENRICHAPPLY_Success(t *testing.T) {
	s := store.NewStore()
	ctx := discardCtx("ENRICH.CREATE", bytesArgs("applyenrich", "src"), s)
	cmdENRICHCREATE(ctx)
	ctx = discardCtx("ENRICH.APPLY", bytesArgs("applyenrich", "data"), s)
	if err := cmdENRICHAPPLY(ctx); err != nil {
		t.Fatal(err)
	}
}

func TestCmdENRICHAPPLY_NotFound(t *testing.T) {
	s := store.NewStore()
	ctx := discardCtx("ENRICH.APPLY", bytesArgs("nonexistent", "data"), s)
	if err := cmdENRICHAPPLY(ctx); err != nil {
		t.Fatal(err)
	}
}

func TestCmdENRICHAPPLY_TooFewArgs(t *testing.T) {
	s := store.NewStore()
	ctx := discardCtx("ENRICH.APPLY", bytesArgs("myenricher"), s)
	if err := cmdENRICHAPPLY(ctx); err != nil {
		t.Fatal(err)
	}
}

func TestCmdENRICHLIST(t *testing.T) {
	s := store.NewStore()
	ctx := discardCtx("ENRICH.LIST", bytesArgs(), s)
	if err := cmdENRICHLIST(ctx); err != nil {
		t.Fatal(err)
	}
}

// ======================================================================
// VALIDATE COMMANDS
// ======================================================================

func TestCmdVALIDATECREATE_Success(t *testing.T) {
	s := store.NewStore()
	ctx := discardCtx("VALIDATE.CREATE", bytesArgs("myval", "len > 0"), s)
	if err := cmdVALIDATECREATE(ctx); err != nil {
		t.Fatal(err)
	}
}

func TestCmdVALIDATECREATE_TooFewArgs(t *testing.T) {
	s := store.NewStore()
	ctx := discardCtx("VALIDATE.CREATE", bytesArgs("myval"), s)
	if err := cmdVALIDATECREATE(ctx); err != nil {
		t.Fatal(err)
	}
}

func TestCmdVALIDATEDELETE_Success(t *testing.T) {
	s := store.NewStore()
	ctx := discardCtx("VALIDATE.CREATE", bytesArgs("delval", "len > 0"), s)
	cmdVALIDATECREATE(ctx)
	ctx = discardCtx("VALIDATE.DELETE", bytesArgs("delval"), s)
	if err := cmdVALIDATEDELETE(ctx); err != nil {
		t.Fatal(err)
	}
}

func TestCmdVALIDATEDELETE_NotFound(t *testing.T) {
	s := store.NewStore()
	ctx := discardCtx("VALIDATE.DELETE", bytesArgs("nonexistent"), s)
	if err := cmdVALIDATEDELETE(ctx); err != nil {
		t.Fatal(err)
	}
}

func TestCmdVALIDATEDELETE_TooFewArgs(t *testing.T) {
	s := store.NewStore()
	ctx := discardCtx("VALIDATE.DELETE", bytesArgs(), s)
	if err := cmdVALIDATEDELETE(ctx); err != nil {
		t.Fatal(err)
	}
}

func TestCmdVALIDATECHECK_Success(t *testing.T) {
	s := store.NewStore()
	ctx := discardCtx("VALIDATE.CREATE", bytesArgs("checkval", "len > 0"), s)
	cmdVALIDATECREATE(ctx)
	ctx = discardCtx("VALIDATE.CHECK", bytesArgs("checkval", "data"), s)
	if err := cmdVALIDATECHECK(ctx); err != nil {
		t.Fatal(err)
	}
}

func TestCmdVALIDATECHECK_NotFound(t *testing.T) {
	s := store.NewStore()
	ctx := discardCtx("VALIDATE.CHECK", bytesArgs("nonexistent", "data"), s)
	if err := cmdVALIDATECHECK(ctx); err != nil {
		t.Fatal(err)
	}
}

func TestCmdVALIDATECHECK_TooFewArgs(t *testing.T) {
	s := store.NewStore()
	ctx := discardCtx("VALIDATE.CHECK", bytesArgs("myval"), s)
	if err := cmdVALIDATECHECK(ctx); err != nil {
		t.Fatal(err)
	}
}

func TestCmdVALIDATELIST(t *testing.T) {
	s := store.NewStore()
	ctx := discardCtx("VALIDATE.LIST", bytesArgs(), s)
	if err := cmdVALIDATELIST(ctx); err != nil {
		t.Fatal(err)
	}
}

// ======================================================================
// JOBX COMMANDS
// ======================================================================

func TestCmdJOBXCREATE_Success(t *testing.T) {
	s := store.NewStore()
	ctx := discardCtx("JOBX.CREATE", bytesArgs("myjob"), s)
	if err := cmdJOBXCREATE(ctx); err != nil {
		t.Fatal(err)
	}
}

func TestCmdJOBXCREATE_TooFewArgs(t *testing.T) {
	s := store.NewStore()
	ctx := discardCtx("JOBX.CREATE", bytesArgs(), s)
	if err := cmdJOBXCREATE(ctx); err != nil {
		t.Fatal(err)
	}
}

func TestCmdJOBXDELETE_NotFound(t *testing.T) {
	s := store.NewStore()
	ctx := discardCtx("JOBX.DELETE", bytesArgs("nonexistent"), s)
	if err := cmdJOBXDELETE(ctx); err != nil {
		t.Fatal(err)
	}
}

func TestCmdJOBXDELETE_TooFewArgs(t *testing.T) {
	s := store.NewStore()
	ctx := discardCtx("JOBX.DELETE", bytesArgs(), s)
	if err := cmdJOBXDELETE(ctx); err != nil {
		t.Fatal(err)
	}
}

func TestCmdJOBXRUN_NotFound(t *testing.T) {
	s := store.NewStore()
	ctx := discardCtx("JOBX.RUN", bytesArgs("nonexistent"), s)
	if err := cmdJOBXRUN(ctx); err != nil {
		t.Fatal(err)
	}
}

func TestCmdJOBXRUN_TooFewArgs(t *testing.T) {
	s := store.NewStore()
	ctx := discardCtx("JOBX.RUN", bytesArgs(), s)
	if err := cmdJOBXRUN(ctx); err != nil {
		t.Fatal(err)
	}
}

func TestCmdJOBXSTATUS_NotFound(t *testing.T) {
	s := store.NewStore()
	ctx := discardCtx("JOBX.STATUS", bytesArgs("nonexistent"), s)
	if err := cmdJOBXSTATUS(ctx); err != nil {
		t.Fatal(err)
	}
}

func TestCmdJOBXSTATUS_TooFewArgs(t *testing.T) {
	s := store.NewStore()
	ctx := discardCtx("JOBX.STATUS", bytesArgs(), s)
	if err := cmdJOBXSTATUS(ctx); err != nil {
		t.Fatal(err)
	}
}

func TestCmdJOBXLIST(t *testing.T) {
	s := store.NewStore()
	ctx := discardCtx("JOBX.LIST", bytesArgs(), s)
	if err := cmdJOBXLIST(ctx); err != nil {
		t.Fatal(err)
	}
}

// JOBX lifecycle: create, run, status, delete
func TestCmdJOBX_Lifecycle(t *testing.T) {
	s := store.NewStore()
	// Create -- capture the ID via bufCtx
	ctx, buf := bufCtx("JOBX.CREATE", bytesArgs("lifecycle-job"), s)
	if err := cmdJOBXCREATE(ctx); err != nil {
		t.Fatal(err)
	}
	// Extract UUID from bulk string response: $36\r\n<uuid>\r\n
	output := buf.String()
	// Find the UUID between first \r\n and second \r\n
	id := extractBulkString(output)
	if id == "" {
		t.Fatalf("could not extract job ID from output: %q", output)
	}
	// Run
	ctx = discardCtx("JOBX.RUN", bytesArgs(id), s)
	if err := cmdJOBXRUN(ctx); err != nil {
		t.Fatal(err)
	}
	// Status
	ctx = discardCtx("JOBX.STATUS", bytesArgs(id), s)
	if err := cmdJOBXSTATUS(ctx); err != nil {
		t.Fatal(err)
	}
	// Delete
	ctx = discardCtx("JOBX.DELETE", bytesArgs(id), s)
	if err := cmdJOBXDELETE(ctx); err != nil {
		t.Fatal(err)
	}
}

// extractBulkString extracts the payload from a RESP bulk string: $N\r\n<payload>\r\n
func extractBulkString(s string) string {
	// Find the first \r\n (end of length prefix)
	idx := 0
	for idx < len(s) {
		if idx+1 < len(s) && s[idx] == '\r' && s[idx+1] == '\n' {
			break
		}
		idx++
	}
	if idx >= len(s)-1 {
		return ""
	}
	start := idx + 2
	// Find the next \r\n
	end := start
	for end < len(s) {
		if end+1 < len(s) && s[end] == '\r' && s[end+1] == '\n' {
			break
		}
		end++
	}
	if end > start {
		return s[start:end]
	}
	return ""
}

// ======================================================================
// STAGE COMMANDS
// ======================================================================

func TestCmdSTAGECREATE_Success(t *testing.T) {
	s := store.NewStore()
	ctx := discardCtx("STAGE.CREATE", bytesArgs("mystage", "5"), s)
	if err := cmdSTAGECREATE(ctx); err != nil {
		t.Fatal(err)
	}
}

func TestCmdSTAGECREATE_TooFewArgs(t *testing.T) {
	s := store.NewStore()
	ctx := discardCtx("STAGE.CREATE", bytesArgs("mystage"), s)
	if err := cmdSTAGECREATE(ctx); err != nil {
		t.Fatal(err)
	}
}

func TestCmdSTAGEDELETE_Success(t *testing.T) {
	s := store.NewStore()
	ctx := discardCtx("STAGE.CREATE", bytesArgs("delstage", "3"), s)
	cmdSTAGECREATE(ctx)
	ctx = discardCtx("STAGE.DELETE", bytesArgs("delstage"), s)
	if err := cmdSTAGEDELETE(ctx); err != nil {
		t.Fatal(err)
	}
}

func TestCmdSTAGEDELETE_NotFound(t *testing.T) {
	s := store.NewStore()
	ctx := discardCtx("STAGE.DELETE", bytesArgs("nonexistent"), s)
	if err := cmdSTAGEDELETE(ctx); err != nil {
		t.Fatal(err)
	}
}

func TestCmdSTAGEDELETE_TooFewArgs(t *testing.T) {
	s := store.NewStore()
	ctx := discardCtx("STAGE.DELETE", bytesArgs(), s)
	if err := cmdSTAGEDELETE(ctx); err != nil {
		t.Fatal(err)
	}
}

func TestCmdSTAGENEXT_Success(t *testing.T) {
	s := store.NewStore()
	ctx := discardCtx("STAGE.CREATE", bytesArgs("nextstage", "5"), s)
	cmdSTAGECREATE(ctx)
	ctx = discardCtx("STAGE.NEXT", bytesArgs("nextstage"), s)
	if err := cmdSTAGENEXT(ctx); err != nil {
		t.Fatal(err)
	}
}

func TestCmdSTAGENEXT_NotFound(t *testing.T) {
	s := store.NewStore()
	ctx := discardCtx("STAGE.NEXT", bytesArgs("nonexistent"), s)
	if err := cmdSTAGENEXT(ctx); err != nil {
		t.Fatal(err)
	}
}

func TestCmdSTAGENEXT_TooFewArgs(t *testing.T) {
	s := store.NewStore()
	ctx := discardCtx("STAGE.NEXT", bytesArgs(), s)
	if err := cmdSTAGENEXT(ctx); err != nil {
		t.Fatal(err)
	}
}

func TestCmdSTAGEPREV_Success(t *testing.T) {
	s := store.NewStore()
	ctx := discardCtx("STAGE.CREATE", bytesArgs("prevstage", "5"), s)
	cmdSTAGECREATE(ctx)
	// Advance to stage 1 first
	ctx = discardCtx("STAGE.NEXT", bytesArgs("prevstage"), s)
	cmdSTAGENEXT(ctx)
	// Then go back
	ctx = discardCtx("STAGE.PREV", bytesArgs("prevstage"), s)
	if err := cmdSTAGEPREV(ctx); err != nil {
		t.Fatal(err)
	}
}

func TestCmdSTAGEPREV_AtZero(t *testing.T) {
	s := store.NewStore()
	ctx := discardCtx("STAGE.CREATE", bytesArgs("zerostage", "5"), s)
	cmdSTAGECREATE(ctx)
	ctx = discardCtx("STAGE.PREV", bytesArgs("zerostage"), s)
	if err := cmdSTAGEPREV(ctx); err != nil {
		t.Fatal(err)
	}
}

func TestCmdSTAGEPREV_NotFound(t *testing.T) {
	s := store.NewStore()
	ctx := discardCtx("STAGE.PREV", bytesArgs("nonexistent"), s)
	if err := cmdSTAGEPREV(ctx); err != nil {
		t.Fatal(err)
	}
}

func TestCmdSTAGEPREV_TooFewArgs(t *testing.T) {
	s := store.NewStore()
	ctx := discardCtx("STAGE.PREV", bytesArgs(), s)
	if err := cmdSTAGEPREV(ctx); err != nil {
		t.Fatal(err)
	}
}

func TestCmdSTAGELIST(t *testing.T) {
	s := store.NewStore()
	ctx := discardCtx("STAGE.LIST", bytesArgs(), s)
	if err := cmdSTAGELIST(ctx); err != nil {
		t.Fatal(err)
	}
}

// ======================================================================
// CONTEXT COMMANDS
// ======================================================================

func TestCmdCONTEXTCREATE_Success(t *testing.T) {
	s := store.NewStore()
	ctx := discardCtx("CONTEXT.CREATE", bytesArgs("myctx"), s)
	if err := cmdCONTEXTCREATE(ctx); err != nil {
		t.Fatal(err)
	}
}

func TestCmdCONTEXTCREATE_TooFewArgs(t *testing.T) {
	s := store.NewStore()
	ctx := discardCtx("CONTEXT.CREATE", bytesArgs(), s)
	if err := cmdCONTEXTCREATE(ctx); err != nil {
		t.Fatal(err)
	}
}

func TestCmdCONTEXTDELETE_Success(t *testing.T) {
	s := store.NewStore()
	ctx := discardCtx("CONTEXT.CREATE", bytesArgs("delctx"), s)
	cmdCONTEXTCREATE(ctx)
	ctx = discardCtx("CONTEXT.DELETE", bytesArgs("delctx"), s)
	if err := cmdCONTEXTDELETE(ctx); err != nil {
		t.Fatal(err)
	}
}

func TestCmdCONTEXTDELETE_NotFound(t *testing.T) {
	s := store.NewStore()
	ctx := discardCtx("CONTEXT.DELETE", bytesArgs("nonexistent"), s)
	if err := cmdCONTEXTDELETE(ctx); err != nil {
		t.Fatal(err)
	}
}

func TestCmdCONTEXTDELETE_TooFewArgs(t *testing.T) {
	s := store.NewStore()
	ctx := discardCtx("CONTEXT.DELETE", bytesArgs(), s)
	if err := cmdCONTEXTDELETE(ctx); err != nil {
		t.Fatal(err)
	}
}

func TestCmdCONTEXTSET_Success(t *testing.T) {
	s := store.NewStore()
	ctx := discardCtx("CONTEXT.CREATE", bytesArgs("setctx"), s)
	cmdCONTEXTCREATE(ctx)
	ctx = discardCtx("CONTEXT.SET", bytesArgs("setctx", "key1", "val1"), s)
	if err := cmdCONTEXTSET(ctx); err != nil {
		t.Fatal(err)
	}
}

func TestCmdCONTEXTSET_NotFound(t *testing.T) {
	s := store.NewStore()
	ctx := discardCtx("CONTEXT.SET", bytesArgs("nonexistent", "key1", "val1"), s)
	if err := cmdCONTEXTSET(ctx); err != nil {
		t.Fatal(err)
	}
}

func TestCmdCONTEXTSET_TooFewArgs(t *testing.T) {
	s := store.NewStore()
	ctx := discardCtx("CONTEXT.SET", bytesArgs("myctx", "key1"), s)
	if err := cmdCONTEXTSET(ctx); err != nil {
		t.Fatal(err)
	}
}

func TestCmdCONTEXTGET_Success(t *testing.T) {
	s := store.NewStore()
	ctx := discardCtx("CONTEXT.CREATE", bytesArgs("getctx"), s)
	cmdCONTEXTCREATE(ctx)
	ctx = discardCtx("CONTEXT.SET", bytesArgs("getctx", "k", "v"), s)
	cmdCONTEXTSET(ctx)
	ctx = discardCtx("CONTEXT.GET", bytesArgs("getctx", "k"), s)
	if err := cmdCONTEXTGET(ctx); err != nil {
		t.Fatal(err)
	}
}

func TestCmdCONTEXTGET_KeyNotFound(t *testing.T) {
	s := store.NewStore()
	ctx := discardCtx("CONTEXT.CREATE", bytesArgs("getctx2"), s)
	cmdCONTEXTCREATE(ctx)
	ctx = discardCtx("CONTEXT.GET", bytesArgs("getctx2", "missing"), s)
	if err := cmdCONTEXTGET(ctx); err != nil {
		t.Fatal(err)
	}
}

func TestCmdCONTEXTGET_ContextNotFound(t *testing.T) {
	s := store.NewStore()
	ctx := discardCtx("CONTEXT.GET", bytesArgs("nonexistent", "k"), s)
	if err := cmdCONTEXTGET(ctx); err != nil {
		t.Fatal(err)
	}
}

func TestCmdCONTEXTGET_TooFewArgs(t *testing.T) {
	s := store.NewStore()
	ctx := discardCtx("CONTEXT.GET", bytesArgs("myctx"), s)
	if err := cmdCONTEXTGET(ctx); err != nil {
		t.Fatal(err)
	}
}

func TestCmdCONTEXTLIST(t *testing.T) {
	s := store.NewStore()
	ctx := discardCtx("CONTEXT.LIST", bytesArgs(), s)
	if err := cmdCONTEXTLIST(ctx); err != nil {
		t.Fatal(err)
	}
}

// ======================================================================
// RULE COMMANDS
// ======================================================================

func TestCmdRULECREATE_Success(t *testing.T) {
	s := store.NewStore()
	ctx := discardCtx("RULE.CREATE", bytesArgs("myrule", "x > 5"), s)
	if err := cmdRULECREATE(ctx); err != nil {
		t.Fatal(err)
	}
}

func TestCmdRULECREATE_TooFewArgs(t *testing.T) {
	s := store.NewStore()
	ctx := discardCtx("RULE.CREATE", bytesArgs("myrule"), s)
	if err := cmdRULECREATE(ctx); err != nil {
		t.Fatal(err)
	}
}

func TestCmdRULEDELETE_Success(t *testing.T) {
	s := store.NewStore()
	ctx := discardCtx("RULE.CREATE", bytesArgs("delrule", "x > 5"), s)
	cmdRULECREATE(ctx)
	ctx = discardCtx("RULE.DELETE", bytesArgs("delrule"), s)
	if err := cmdRULEDELETE(ctx); err != nil {
		t.Fatal(err)
	}
}

func TestCmdRULEDELETE_NotFound(t *testing.T) {
	s := store.NewStore()
	ctx := discardCtx("RULE.DELETE", bytesArgs("nonexistent"), s)
	if err := cmdRULEDELETE(ctx); err != nil {
		t.Fatal(err)
	}
}

func TestCmdRULEDELETE_TooFewArgs(t *testing.T) {
	s := store.NewStore()
	ctx := discardCtx("RULE.DELETE", bytesArgs(), s)
	if err := cmdRULEDELETE(ctx); err != nil {
		t.Fatal(err)
	}
}

func TestCmdRULEEVAL_Success(t *testing.T) {
	s := store.NewStore()
	ctx := discardCtx("RULE.CREATE", bytesArgs("evalrule", "x > 5"), s)
	cmdRULECREATE(ctx)
	ctx = discardCtx("RULE.EVAL", bytesArgs("evalrule", "10"), s)
	if err := cmdRULEEVAL(ctx); err != nil {
		t.Fatal(err)
	}
}

func TestCmdRULEEVAL_NotFound(t *testing.T) {
	s := store.NewStore()
	ctx := discardCtx("RULE.EVAL", bytesArgs("nonexistent", "10"), s)
	if err := cmdRULEEVAL(ctx); err != nil {
		t.Fatal(err)
	}
}

func TestCmdRULEEVAL_TooFewArgs(t *testing.T) {
	s := store.NewStore()
	ctx := discardCtx("RULE.EVAL", bytesArgs("myrule"), s)
	if err := cmdRULEEVAL(ctx); err != nil {
		t.Fatal(err)
	}
}

func TestCmdRULELIST(t *testing.T) {
	s := store.NewStore()
	ctx := discardCtx("RULE.LIST", bytesArgs(), s)
	if err := cmdRULELIST(ctx); err != nil {
		t.Fatal(err)
	}
}

// ======================================================================
// POLICY COMMANDS
// ======================================================================

func TestCmdPOLICYCREATE_Success(t *testing.T) {
	s := store.NewStore()
	ctx := discardCtx("POLICY.CREATE", bytesArgs("mypolicy", "deny-all"), s)
	if err := cmdPOLICYCREATE(ctx); err != nil {
		t.Fatal(err)
	}
}

func TestCmdPOLICYCREATE_TooFewArgs(t *testing.T) {
	s := store.NewStore()
	ctx := discardCtx("POLICY.CREATE", bytesArgs("mypolicy"), s)
	if err := cmdPOLICYCREATE(ctx); err != nil {
		t.Fatal(err)
	}
}

func TestCmdPOLICYDELETE_Success(t *testing.T) {
	s := store.NewStore()
	ctx := discardCtx("POLICY.CREATE", bytesArgs("delpol", "deny-all"), s)
	cmdPOLICYCREATE(ctx)
	ctx = discardCtx("POLICY.DELETE", bytesArgs("delpol"), s)
	if err := cmdPOLICYDELETE(ctx); err != nil {
		t.Fatal(err)
	}
}

func TestCmdPOLICYDELETE_NotFound(t *testing.T) {
	s := store.NewStore()
	ctx := discardCtx("POLICY.DELETE", bytesArgs("nonexistent"), s)
	if err := cmdPOLICYDELETE(ctx); err != nil {
		t.Fatal(err)
	}
}

func TestCmdPOLICYDELETE_TooFewArgs(t *testing.T) {
	s := store.NewStore()
	ctx := discardCtx("POLICY.DELETE", bytesArgs(), s)
	if err := cmdPOLICYDELETE(ctx); err != nil {
		t.Fatal(err)
	}
}

func TestCmdPOLICYCHECK_Success(t *testing.T) {
	s := store.NewStore()
	ctx := discardCtx("POLICY.CHECK", bytesArgs("mypolicy", "data"), s)
	if err := cmdPOLICYCHECK(ctx); err != nil {
		t.Fatal(err)
	}
}

func TestCmdPOLICYCHECK_TooFewArgs(t *testing.T) {
	s := store.NewStore()
	ctx := discardCtx("POLICY.CHECK", bytesArgs("mypolicy"), s)
	if err := cmdPOLICYCHECK(ctx); err != nil {
		t.Fatal(err)
	}
}

func TestCmdPOLICYLIST(t *testing.T) {
	s := store.NewStore()
	ctx := discardCtx("POLICY.LIST", bytesArgs(), s)
	if err := cmdPOLICYLIST(ctx); err != nil {
		t.Fatal(err)
	}
}

// ======================================================================
// PERMIT COMMANDS
// ======================================================================

func TestCmdPERMITGRANT_Success(t *testing.T) {
	s := store.NewStore()
	ctx := discardCtx("PERMIT.GRANT", bytesArgs("alice", "db", "read"), s)
	if err := cmdPERMITGRANT(ctx); err != nil {
		t.Fatal(err)
	}
}

func TestCmdPERMITGRANT_TooFewArgs(t *testing.T) {
	s := store.NewStore()
	ctx := discardCtx("PERMIT.GRANT", bytesArgs("alice", "db"), s)
	if err := cmdPERMITGRANT(ctx); err != nil {
		t.Fatal(err)
	}
}

func TestCmdPERMITREVOKE_Success(t *testing.T) {
	s := store.NewStore()
	ctx := discardCtx("PERMIT.GRANT", bytesArgs("bob", "db", "write"), s)
	cmdPERMITGRANT(ctx)
	ctx = discardCtx("PERMIT.REVOKE", bytesArgs("bob", "db", "write"), s)
	if err := cmdPERMITREVOKE(ctx); err != nil {
		t.Fatal(err)
	}
}

func TestCmdPERMITREVOKE_NoUser(t *testing.T) {
	s := store.NewStore()
	ctx := discardCtx("PERMIT.REVOKE", bytesArgs("nouser", "db", "write"), s)
	if err := cmdPERMITREVOKE(ctx); err != nil {
		t.Fatal(err)
	}
}

func TestCmdPERMITREVOKE_TooFewArgs(t *testing.T) {
	s := store.NewStore()
	ctx := discardCtx("PERMIT.REVOKE", bytesArgs("alice", "db"), s)
	if err := cmdPERMITREVOKE(ctx); err != nil {
		t.Fatal(err)
	}
}

func TestCmdPERMITCHECK_Success(t *testing.T) {
	s := store.NewStore()
	ctx := discardCtx("PERMIT.GRANT", bytesArgs("charlie", "db", "read"), s)
	cmdPERMITGRANT(ctx)
	ctx = discardCtx("PERMIT.CHECK", bytesArgs("charlie", "db", "read"), s)
	if err := cmdPERMITCHECK(ctx); err != nil {
		t.Fatal(err)
	}
}

func TestCmdPERMITCHECK_NotGranted(t *testing.T) {
	s := store.NewStore()
	ctx := discardCtx("PERMIT.CHECK", bytesArgs("noone", "db", "read"), s)
	if err := cmdPERMITCHECK(ctx); err != nil {
		t.Fatal(err)
	}
}

func TestCmdPERMITCHECK_TooFewArgs(t *testing.T) {
	s := store.NewStore()
	ctx := discardCtx("PERMIT.CHECK", bytesArgs("alice", "db"), s)
	if err := cmdPERMITCHECK(ctx); err != nil {
		t.Fatal(err)
	}
}

func TestCmdPERMITLIST_Success(t *testing.T) {
	s := store.NewStore()
	ctx := discardCtx("PERMIT.GRANT", bytesArgs("dave", "db", "read"), s)
	cmdPERMITGRANT(ctx)
	ctx = discardCtx("PERMIT.LIST", bytesArgs("dave"), s)
	if err := cmdPERMITLIST(ctx); err != nil {
		t.Fatal(err)
	}
}

func TestCmdPERMITLIST_NoUser(t *testing.T) {
	s := store.NewStore()
	ctx := discardCtx("PERMIT.LIST", bytesArgs("noone"), s)
	if err := cmdPERMITLIST(ctx); err != nil {
		t.Fatal(err)
	}
}

func TestCmdPERMITLIST_TooFewArgs(t *testing.T) {
	s := store.NewStore()
	ctx := discardCtx("PERMIT.LIST", bytesArgs(), s)
	if err := cmdPERMITLIST(ctx); err != nil {
		t.Fatal(err)
	}
}

// ======================================================================
// GRANT COMMANDS
// ======================================================================

func TestCmdGRANTCREATE_Success(t *testing.T) {
	s := store.NewStore()
	ctx := discardCtx("GRANT.CREATE", bytesArgs("alice", "resource1", "read", "write"), s)
	if err := cmdGRANTCREATE(ctx); err != nil {
		t.Fatal(err)
	}
}

func TestCmdGRANTCREATE_TooFewArgs(t *testing.T) {
	s := store.NewStore()
	ctx := discardCtx("GRANT.CREATE", bytesArgs("alice", "resource1"), s)
	if err := cmdGRANTCREATE(ctx); err != nil {
		t.Fatal(err)
	}
}

func TestCmdGRANTDELETE_NotFound(t *testing.T) {
	s := store.NewStore()
	ctx := discardCtx("GRANT.DELETE", bytesArgs("nonexistent"), s)
	if err := cmdGRANTDELETE(ctx); err != nil {
		t.Fatal(err)
	}
}

func TestCmdGRANTDELETE_TooFewArgs(t *testing.T) {
	s := store.NewStore()
	ctx := discardCtx("GRANT.DELETE", bytesArgs(), s)
	if err := cmdGRANTDELETE(ctx); err != nil {
		t.Fatal(err)
	}
}

func TestCmdGRANTCHECK_Success(t *testing.T) {
	s := store.NewStore()
	ctx := discardCtx("GRANT.CREATE", bytesArgs("grantuser", "res", "*"), s)
	cmdGRANTCREATE(ctx)
	ctx = discardCtx("GRANT.CHECK", bytesArgs("grantuser", "res", "anything"), s)
	if err := cmdGRANTCHECK(ctx); err != nil {
		t.Fatal(err)
	}
}

func TestCmdGRANTCHECK_NoMatch(t *testing.T) {
	s := store.NewStore()
	ctx := discardCtx("GRANT.CHECK", bytesArgs("nouser", "nores", "noact"), s)
	if err := cmdGRANTCHECK(ctx); err != nil {
		t.Fatal(err)
	}
}

func TestCmdGRANTCHECK_TooFewArgs(t *testing.T) {
	s := store.NewStore()
	ctx := discardCtx("GRANT.CHECK", bytesArgs("u", "r"), s)
	if err := cmdGRANTCHECK(ctx); err != nil {
		t.Fatal(err)
	}
}

func TestCmdGRANTLIST(t *testing.T) {
	s := store.NewStore()
	ctx := discardCtx("GRANT.LIST", bytesArgs(), s)
	if err := cmdGRANTLIST(ctx); err != nil {
		t.Fatal(err)
	}
}

// ======================================================================
// CHAINX COMMANDS
// ======================================================================

func TestCmdCHAINXCREATE_Success(t *testing.T) {
	s := store.NewStore()
	ctx := discardCtx("CHAINX.CREATE", bytesArgs("mychain"), s)
	if err := cmdCHAINXCREATE(ctx); err != nil {
		t.Fatal(err)
	}
}

func TestCmdCHAINXCREATE_TooFewArgs(t *testing.T) {
	s := store.NewStore()
	ctx := discardCtx("CHAINX.CREATE", bytesArgs(), s)
	if err := cmdCHAINXCREATE(ctx); err != nil {
		t.Fatal(err)
	}
}

func TestCmdCHAINXDELETE_Success(t *testing.T) {
	s := store.NewStore()
	ctx := discardCtx("CHAINX.CREATE", bytesArgs("delchain"), s)
	cmdCHAINXCREATE(ctx)
	ctx = discardCtx("CHAINX.DELETE", bytesArgs("delchain"), s)
	if err := cmdCHAINXDELETE(ctx); err != nil {
		t.Fatal(err)
	}
}

func TestCmdCHAINXDELETE_NotFound(t *testing.T) {
	s := store.NewStore()
	ctx := discardCtx("CHAINX.DELETE", bytesArgs("nonexistent"), s)
	if err := cmdCHAINXDELETE(ctx); err != nil {
		t.Fatal(err)
	}
}

func TestCmdCHAINXDELETE_TooFewArgs(t *testing.T) {
	s := store.NewStore()
	ctx := discardCtx("CHAINX.DELETE", bytesArgs(), s)
	if err := cmdCHAINXDELETE(ctx); err != nil {
		t.Fatal(err)
	}
}

func TestCmdCHAINXEXECUTE_Success(t *testing.T) {
	s := store.NewStore()
	ctx := discardCtx("CHAINX.CREATE", bytesArgs("execchain"), s)
	cmdCHAINXCREATE(ctx)
	ctx = discardCtx("CHAINX.EXECUTE", bytesArgs("execchain"), s)
	if err := cmdCHAINXEXECUTE(ctx); err != nil {
		t.Fatal(err)
	}
}

func TestCmdCHAINXEXECUTE_NotFound(t *testing.T) {
	s := store.NewStore()
	ctx := discardCtx("CHAINX.EXECUTE", bytesArgs("nonexistent"), s)
	if err := cmdCHAINXEXECUTE(ctx); err != nil {
		t.Fatal(err)
	}
}

func TestCmdCHAINXEXECUTE_TooFewArgs(t *testing.T) {
	s := store.NewStore()
	ctx := discardCtx("CHAINX.EXECUTE", bytesArgs(), s)
	if err := cmdCHAINXEXECUTE(ctx); err != nil {
		t.Fatal(err)
	}
}

func TestCmdCHAINXLIST(t *testing.T) {
	s := store.NewStore()
	ctx := discardCtx("CHAINX.LIST", bytesArgs(), s)
	if err := cmdCHAINXLIST(ctx); err != nil {
		t.Fatal(err)
	}
}

// ======================================================================
// TASKX COMMANDS
// ======================================================================

func TestCmdTASKXCREATE_Success(t *testing.T) {
	s := store.NewStore()
	ctx := discardCtx("TASKX.CREATE", bytesArgs("mytask"), s)
	if err := cmdTASKXCREATE(ctx); err != nil {
		t.Fatal(err)
	}
}

func TestCmdTASKXCREATE_TooFewArgs(t *testing.T) {
	s := store.NewStore()
	ctx := discardCtx("TASKX.CREATE", bytesArgs(), s)
	if err := cmdTASKXCREATE(ctx); err != nil {
		t.Fatal(err)
	}
}

func TestCmdTASKXDELETE_NotFound(t *testing.T) {
	s := store.NewStore()
	ctx := discardCtx("TASKX.DELETE", bytesArgs("nonexistent"), s)
	if err := cmdTASKXDELETE(ctx); err != nil {
		t.Fatal(err)
	}
}

func TestCmdTASKXDELETE_TooFewArgs(t *testing.T) {
	s := store.NewStore()
	ctx := discardCtx("TASKX.DELETE", bytesArgs(), s)
	if err := cmdTASKXDELETE(ctx); err != nil {
		t.Fatal(err)
	}
}

func TestCmdTASKXRUN_NotFound(t *testing.T) {
	s := store.NewStore()
	ctx := discardCtx("TASKX.RUN", bytesArgs("nonexistent"), s)
	if err := cmdTASKXRUN(ctx); err != nil {
		t.Fatal(err)
	}
}

func TestCmdTASKXRUN_TooFewArgs(t *testing.T) {
	s := store.NewStore()
	ctx := discardCtx("TASKX.RUN", bytesArgs(), s)
	if err := cmdTASKXRUN(ctx); err != nil {
		t.Fatal(err)
	}
}

func TestCmdTASKXLIST(t *testing.T) {
	s := store.NewStore()
	ctx := discardCtx("TASKX.LIST", bytesArgs(), s)
	if err := cmdTASKXLIST(ctx); err != nil {
		t.Fatal(err)
	}
}

// TASKX lifecycle
func TestCmdTASKX_Lifecycle(t *testing.T) {
	s := store.NewStore()
	ctx, buf := bufCtx("TASKX.CREATE", bytesArgs("lifecycle-task"), s)
	if err := cmdTASKXCREATE(ctx); err != nil {
		t.Fatal(err)
	}
	id := extractBulkString(buf.String())
	if id == "" {
		t.Fatal("could not extract task ID")
	}
	ctx = discardCtx("TASKX.RUN", bytesArgs(id), s)
	if err := cmdTASKXRUN(ctx); err != nil {
		t.Fatal(err)
	}
	ctx = discardCtx("TASKX.DELETE", bytesArgs(id), s)
	if err := cmdTASKXDELETE(ctx); err != nil {
		t.Fatal(err)
	}
}

// ======================================================================
// TIMER COMMANDS
// ======================================================================

func TestCmdTIMERCREATE_Success(t *testing.T) {
	s := store.NewStore()
	ctx := discardCtx("TIMER.CREATE", bytesArgs("mytimer", "5000"), s)
	if err := cmdTIMERCREATE(ctx); err != nil {
		t.Fatal(err)
	}
}

func TestCmdTIMERCREATE_TooFewArgs(t *testing.T) {
	s := store.NewStore()
	ctx := discardCtx("TIMER.CREATE", bytesArgs("mytimer"), s)
	if err := cmdTIMERCREATE(ctx); err != nil {
		t.Fatal(err)
	}
}

func TestCmdTIMERDELETE_NotFound(t *testing.T) {
	s := store.NewStore()
	ctx := discardCtx("TIMER.DELETE", bytesArgs("nonexistent"), s)
	if err := cmdTIMERDELETE(ctx); err != nil {
		t.Fatal(err)
	}
}

func TestCmdTIMERDELETE_TooFewArgs(t *testing.T) {
	s := store.NewStore()
	ctx := discardCtx("TIMER.DELETE", bytesArgs(), s)
	if err := cmdTIMERDELETE(ctx); err != nil {
		t.Fatal(err)
	}
}

func TestCmdTIMERSTATUS_NotFound(t *testing.T) {
	s := store.NewStore()
	ctx := discardCtx("TIMER.STATUS", bytesArgs("nonexistent"), s)
	if err := cmdTIMERSTATUS(ctx); err != nil {
		t.Fatal(err)
	}
}

func TestCmdTIMERSTATUS_TooFewArgs(t *testing.T) {
	s := store.NewStore()
	ctx := discardCtx("TIMER.STATUS", bytesArgs(), s)
	if err := cmdTIMERSTATUS(ctx); err != nil {
		t.Fatal(err)
	}
}

func TestCmdTIMERLIST(t *testing.T) {
	s := store.NewStore()
	ctx := discardCtx("TIMER.LIST", bytesArgs(), s)
	if err := cmdTIMERLIST(ctx); err != nil {
		t.Fatal(err)
	}
}

// TIMER lifecycle
func TestCmdTIMER_Lifecycle(t *testing.T) {
	s := store.NewStore()
	ctx, buf := bufCtx("TIMER.CREATE", bytesArgs("lctimer", "60000"), s)
	if err := cmdTIMERCREATE(ctx); err != nil {
		t.Fatal(err)
	}
	id := extractBulkString(buf.String())
	if id == "" {
		t.Fatal("could not extract timer ID")
	}
	ctx = discardCtx("TIMER.STATUS", bytesArgs(id), s)
	if err := cmdTIMERSTATUS(ctx); err != nil {
		t.Fatal(err)
	}
	ctx = discardCtx("TIMER.DELETE", bytesArgs(id), s)
	if err := cmdTIMERDELETE(ctx); err != nil {
		t.Fatal(err)
	}
}

// ======================================================================
// COUNTERX2 COMMANDS
// ======================================================================

func TestCmdCOUNTERX2CREATE_Success(t *testing.T) {
	s := store.NewStore()
	ctx := discardCtx("COUNTERX2.CREATE", bytesArgs("myctr"), s)
	if err := cmdCOUNTERX2CREATE(ctx); err != nil {
		t.Fatal(err)
	}
}

func TestCmdCOUNTERX2CREATE_WithInitVal(t *testing.T) {
	s := store.NewStore()
	ctx := discardCtx("COUNTERX2.CREATE", bytesArgs("myctr2", "100"), s)
	if err := cmdCOUNTERX2CREATE(ctx); err != nil {
		t.Fatal(err)
	}
}

func TestCmdCOUNTERX2CREATE_TooFewArgs(t *testing.T) {
	s := store.NewStore()
	ctx := discardCtx("COUNTERX2.CREATE", bytesArgs(), s)
	if err := cmdCOUNTERX2CREATE(ctx); err != nil {
		t.Fatal(err)
	}
}

func TestCmdCOUNTERX2INCR_Success(t *testing.T) {
	s := store.NewStore()
	ctx := discardCtx("COUNTERX2.CREATE", bytesArgs("incrctr"), s)
	cmdCOUNTERX2CREATE(ctx)
	ctx = discardCtx("COUNTERX2.INCR", bytesArgs("incrctr"), s)
	if err := cmdCOUNTERX2INCR(ctx); err != nil {
		t.Fatal(err)
	}
}

func TestCmdCOUNTERX2INCR_WithAmount(t *testing.T) {
	s := store.NewStore()
	ctx := discardCtx("COUNTERX2.CREATE", bytesArgs("incrctr2"), s)
	cmdCOUNTERX2CREATE(ctx)
	ctx = discardCtx("COUNTERX2.INCR", bytesArgs("incrctr2", "10"), s)
	if err := cmdCOUNTERX2INCR(ctx); err != nil {
		t.Fatal(err)
	}
}

func TestCmdCOUNTERX2INCR_TooFewArgs(t *testing.T) {
	s := store.NewStore()
	ctx := discardCtx("COUNTERX2.INCR", bytesArgs(), s)
	if err := cmdCOUNTERX2INCR(ctx); err != nil {
		t.Fatal(err)
	}
}

func TestCmdCOUNTERX2DECR_Success(t *testing.T) {
	s := store.NewStore()
	ctx := discardCtx("COUNTERX2.CREATE", bytesArgs("decrctr"), s)
	cmdCOUNTERX2CREATE(ctx)
	ctx = discardCtx("COUNTERX2.DECR", bytesArgs("decrctr"), s)
	if err := cmdCOUNTERX2DECR(ctx); err != nil {
		t.Fatal(err)
	}
}

func TestCmdCOUNTERX2DECR_WithAmount(t *testing.T) {
	s := store.NewStore()
	ctx := discardCtx("COUNTERX2.CREATE", bytesArgs("decrctr2", "100"), s)
	cmdCOUNTERX2CREATE(ctx)
	ctx = discardCtx("COUNTERX2.DECR", bytesArgs("decrctr2", "5"), s)
	if err := cmdCOUNTERX2DECR(ctx); err != nil {
		t.Fatal(err)
	}
}

func TestCmdCOUNTERX2DECR_TooFewArgs(t *testing.T) {
	s := store.NewStore()
	ctx := discardCtx("COUNTERX2.DECR", bytesArgs(), s)
	if err := cmdCOUNTERX2DECR(ctx); err != nil {
		t.Fatal(err)
	}
}

func TestCmdCOUNTERX2GET_Success(t *testing.T) {
	s := store.NewStore()
	ctx := discardCtx("COUNTERX2.CREATE", bytesArgs("getctr", "42"), s)
	cmdCOUNTERX2CREATE(ctx)
	ctx = discardCtx("COUNTERX2.GET", bytesArgs("getctr"), s)
	if err := cmdCOUNTERX2GET(ctx); err != nil {
		t.Fatal(err)
	}
}

func TestCmdCOUNTERX2GET_NotFound(t *testing.T) {
	s := store.NewStore()
	ctx := discardCtx("COUNTERX2.GET", bytesArgs("nonexistent"), s)
	if err := cmdCOUNTERX2GET(ctx); err != nil {
		t.Fatal(err)
	}
}

func TestCmdCOUNTERX2GET_TooFewArgs(t *testing.T) {
	s := store.NewStore()
	ctx := discardCtx("COUNTERX2.GET", bytesArgs(), s)
	if err := cmdCOUNTERX2GET(ctx); err != nil {
		t.Fatal(err)
	}
}

func TestCmdCOUNTERX2LIST(t *testing.T) {
	s := store.NewStore()
	ctx := discardCtx("COUNTERX2.LIST", bytesArgs(), s)
	if err := cmdCOUNTERX2LIST(ctx); err != nil {
		t.Fatal(err)
	}
}

// ======================================================================
// LEVEL COMMANDS
// ======================================================================

func TestCmdLEVELCREATE_Success(t *testing.T) {
	s := store.NewStore()
	ctx := discardCtx("LEVEL.CREATE", bytesArgs("mylevel", "10"), s)
	if err := cmdLEVELCREATE(ctx); err != nil {
		t.Fatal(err)
	}
}

func TestCmdLEVELCREATE_TooFewArgs(t *testing.T) {
	s := store.NewStore()
	ctx := discardCtx("LEVEL.CREATE", bytesArgs("mylevel"), s)
	if err := cmdLEVELCREATE(ctx); err != nil {
		t.Fatal(err)
	}
}

func TestCmdLEVELDELETE_Success(t *testing.T) {
	s := store.NewStore()
	ctx := discardCtx("LEVEL.CREATE", bytesArgs("dellevel", "10"), s)
	cmdLEVELCREATE(ctx)
	ctx = discardCtx("LEVEL.DELETE", bytesArgs("dellevel"), s)
	if err := cmdLEVELDELETE(ctx); err != nil {
		t.Fatal(err)
	}
}

func TestCmdLEVELDELETE_NotFound(t *testing.T) {
	s := store.NewStore()
	ctx := discardCtx("LEVEL.DELETE", bytesArgs("nonexistent"), s)
	if err := cmdLEVELDELETE(ctx); err != nil {
		t.Fatal(err)
	}
}

func TestCmdLEVELDELETE_TooFewArgs(t *testing.T) {
	s := store.NewStore()
	ctx := discardCtx("LEVEL.DELETE", bytesArgs(), s)
	if err := cmdLEVELDELETE(ctx); err != nil {
		t.Fatal(err)
	}
}

func TestCmdLEVELSET_Success(t *testing.T) {
	s := store.NewStore()
	ctx := discardCtx("LEVEL.SET", bytesArgs("setlevel", "5"), s)
	if err := cmdLEVELSET(ctx); err != nil {
		t.Fatal(err)
	}
}

func TestCmdLEVELSET_TooFewArgs(t *testing.T) {
	s := store.NewStore()
	ctx := discardCtx("LEVEL.SET", bytesArgs("mylevel"), s)
	if err := cmdLEVELSET(ctx); err != nil {
		t.Fatal(err)
	}
}

func TestCmdLEVELGET_Success(t *testing.T) {
	s := store.NewStore()
	ctx := discardCtx("LEVEL.CREATE", bytesArgs("getlevel", "10"), s)
	cmdLEVELCREATE(ctx)
	ctx = discardCtx("LEVEL.SET", bytesArgs("getlevel", "3"), s)
	cmdLEVELSET(ctx)
	ctx = discardCtx("LEVEL.GET", bytesArgs("getlevel"), s)
	if err := cmdLEVELGET(ctx); err != nil {
		t.Fatal(err)
	}
}

func TestCmdLEVELGET_TooFewArgs(t *testing.T) {
	s := store.NewStore()
	ctx := discardCtx("LEVEL.GET", bytesArgs(), s)
	if err := cmdLEVELGET(ctx); err != nil {
		t.Fatal(err)
	}
}

func TestCmdLEVELLIST(t *testing.T) {
	s := store.NewStore()
	ctx := discardCtx("LEVEL.LIST", bytesArgs(), s)
	if err := cmdLEVELLIST(ctx); err != nil {
		t.Fatal(err)
	}
}

// ======================================================================
// RECORD COMMANDS
// ======================================================================

func TestCmdRECORDCREATE_Success(t *testing.T) {
	s := store.NewStore()
	ctx := discardCtx("RECORD.CREATE", bytesArgs("myrec"), s)
	if err := cmdRECORDCREATE(ctx); err != nil {
		t.Fatal(err)
	}
}

func TestCmdRECORDCREATE_TooFewArgs(t *testing.T) {
	s := store.NewStore()
	ctx := discardCtx("RECORD.CREATE", bytesArgs(), s)
	if err := cmdRECORDCREATE(ctx); err != nil {
		t.Fatal(err)
	}
}

func TestCmdRECORDADD_NotFound(t *testing.T) {
	s := store.NewStore()
	ctx := discardCtx("RECORD.ADD", bytesArgs("nonexistent", "key", "val"), s)
	if err := cmdRECORDADD(ctx); err != nil {
		t.Fatal(err)
	}
}

func TestCmdRECORDADD_TooFewArgs(t *testing.T) {
	s := store.NewStore()
	ctx := discardCtx("RECORD.ADD", bytesArgs("id", "key"), s)
	if err := cmdRECORDADD(ctx); err != nil {
		t.Fatal(err)
	}
}

func TestCmdRECORDGET_NotFound(t *testing.T) {
	s := store.NewStore()
	ctx := discardCtx("RECORD.GET", bytesArgs("nonexistent"), s)
	if err := cmdRECORDGET(ctx); err != nil {
		t.Fatal(err)
	}
}

func TestCmdRECORDGET_TooFewArgs(t *testing.T) {
	s := store.NewStore()
	ctx := discardCtx("RECORD.GET", bytesArgs(), s)
	if err := cmdRECORDGET(ctx); err != nil {
		t.Fatal(err)
	}
}

func TestCmdRECORDDELETE_NotFound(t *testing.T) {
	s := store.NewStore()
	ctx := discardCtx("RECORD.DELETE", bytesArgs("nonexistent"), s)
	if err := cmdRECORDDELETE(ctx); err != nil {
		t.Fatal(err)
	}
}

func TestCmdRECORDDELETE_TooFewArgs(t *testing.T) {
	s := store.NewStore()
	ctx := discardCtx("RECORD.DELETE", bytesArgs(), s)
	if err := cmdRECORDDELETE(ctx); err != nil {
		t.Fatal(err)
	}
}

// RECORD lifecycle
func TestCmdRECORD_Lifecycle(t *testing.T) {
	s := store.NewStore()
	ctx, buf := bufCtx("RECORD.CREATE", bytesArgs("test-record"), s)
	if err := cmdRECORDCREATE(ctx); err != nil {
		t.Fatal(err)
	}
	id := extractBulkString(buf.String())
	if id == "" {
		t.Fatal("could not extract record ID")
	}
	ctx = discardCtx("RECORD.ADD", bytesArgs(id, "name", "Alice"), s)
	if err := cmdRECORDADD(ctx); err != nil {
		t.Fatal(err)
	}
	ctx = discardCtx("RECORD.GET", bytesArgs(id), s)
	if err := cmdRECORDGET(ctx); err != nil {
		t.Fatal(err)
	}
	ctx = discardCtx("RECORD.DELETE", bytesArgs(id), s)
	if err := cmdRECORDDELETE(ctx); err != nil {
		t.Fatal(err)
	}
}

// ======================================================================
// ENTITY COMMANDS
// ======================================================================

func TestCmdENTITYCREATE_Success(t *testing.T) {
	s := store.NewStore()
	ctx := discardCtx("ENTITY.CREATE", bytesArgs("ent1", "user"), s)
	if err := cmdENTITYCREATE(ctx); err != nil {
		t.Fatal(err)
	}
}

func TestCmdENTITYCREATE_TooFewArgs(t *testing.T) {
	s := store.NewStore()
	ctx := discardCtx("ENTITY.CREATE", bytesArgs("ent1"), s)
	if err := cmdENTITYCREATE(ctx); err != nil {
		t.Fatal(err)
	}
}

func TestCmdENTITYDELETE_Success(t *testing.T) {
	s := store.NewStore()
	ctx := discardCtx("ENTITY.CREATE", bytesArgs("delent", "user"), s)
	cmdENTITYCREATE(ctx)
	ctx = discardCtx("ENTITY.DELETE", bytesArgs("delent"), s)
	if err := cmdENTITYDELETE(ctx); err != nil {
		t.Fatal(err)
	}
}

func TestCmdENTITYDELETE_NotFound(t *testing.T) {
	s := store.NewStore()
	ctx := discardCtx("ENTITY.DELETE", bytesArgs("nonexistent"), s)
	if err := cmdENTITYDELETE(ctx); err != nil {
		t.Fatal(err)
	}
}

func TestCmdENTITYDELETE_TooFewArgs(t *testing.T) {
	s := store.NewStore()
	ctx := discardCtx("ENTITY.DELETE", bytesArgs(), s)
	if err := cmdENTITYDELETE(ctx); err != nil {
		t.Fatal(err)
	}
}

func TestCmdENTITYGET_Success(t *testing.T) {
	s := store.NewStore()
	ctx := discardCtx("ENTITY.CREATE", bytesArgs("getent", "user"), s)
	cmdENTITYCREATE(ctx)
	ctx = discardCtx("ENTITY.GET", bytesArgs("getent"), s)
	if err := cmdENTITYGET(ctx); err != nil {
		t.Fatal(err)
	}
}

func TestCmdENTITYGET_NotFound(t *testing.T) {
	s := store.NewStore()
	ctx := discardCtx("ENTITY.GET", bytesArgs("nonexistent"), s)
	if err := cmdENTITYGET(ctx); err != nil {
		t.Fatal(err)
	}
}

func TestCmdENTITYGET_TooFewArgs(t *testing.T) {
	s := store.NewStore()
	ctx := discardCtx("ENTITY.GET", bytesArgs(), s)
	if err := cmdENTITYGET(ctx); err != nil {
		t.Fatal(err)
	}
}

func TestCmdENTITYSET_Success(t *testing.T) {
	s := store.NewStore()
	ctx := discardCtx("ENTITY.CREATE", bytesArgs("setent", "user"), s)
	cmdENTITYCREATE(ctx)
	ctx = discardCtx("ENTITY.SET", bytesArgs("setent", "email", "test@test.com"), s)
	if err := cmdENTITYSET(ctx); err != nil {
		t.Fatal(err)
	}
}

func TestCmdENTITYSET_NotFound(t *testing.T) {
	s := store.NewStore()
	ctx := discardCtx("ENTITY.SET", bytesArgs("nonexistent", "key", "val"), s)
	if err := cmdENTITYSET(ctx); err != nil {
		t.Fatal(err)
	}
}

func TestCmdENTITYSET_TooFewArgs(t *testing.T) {
	s := store.NewStore()
	ctx := discardCtx("ENTITY.SET", bytesArgs("ent1", "key"), s)
	if err := cmdENTITYSET(ctx); err != nil {
		t.Fatal(err)
	}
}

func TestCmdENTITYLIST(t *testing.T) {
	s := store.NewStore()
	ctx := discardCtx("ENTITY.LIST", bytesArgs(), s)
	if err := cmdENTITYLIST(ctx); err != nil {
		t.Fatal(err)
	}
}

// ======================================================================
// RELATION COMMANDS
// ======================================================================

func TestCmdRELATIONCREATE_Success(t *testing.T) {
	s := store.NewStore()
	ctx := discardCtx("RELATION.CREATE", bytesArgs("a", "b", "friends"), s)
	if err := cmdRELATIONCREATE(ctx); err != nil {
		t.Fatal(err)
	}
}

func TestCmdRELATIONCREATE_TooFewArgs(t *testing.T) {
	s := store.NewStore()
	ctx := discardCtx("RELATION.CREATE", bytesArgs("a", "b"), s)
	if err := cmdRELATIONCREATE(ctx); err != nil {
		t.Fatal(err)
	}
}

func TestCmdRELATIONDELETE_NotFound(t *testing.T) {
	s := store.NewStore()
	ctx := discardCtx("RELATION.DELETE", bytesArgs("nonexistent"), s)
	if err := cmdRELATIONDELETE(ctx); err != nil {
		t.Fatal(err)
	}
}

func TestCmdRELATIONDELETE_TooFewArgs(t *testing.T) {
	s := store.NewStore()
	ctx := discardCtx("RELATION.DELETE", bytesArgs(), s)
	if err := cmdRELATIONDELETE(ctx); err != nil {
		t.Fatal(err)
	}
}

func TestCmdRELATIONGET_NotFound(t *testing.T) {
	s := store.NewStore()
	ctx := discardCtx("RELATION.GET", bytesArgs("nonexistent"), s)
	if err := cmdRELATIONGET(ctx); err != nil {
		t.Fatal(err)
	}
}

func TestCmdRELATIONGET_TooFewArgs(t *testing.T) {
	s := store.NewStore()
	ctx := discardCtx("RELATION.GET", bytesArgs(), s)
	if err := cmdRELATIONGET(ctx); err != nil {
		t.Fatal(err)
	}
}

func TestCmdRELATIONLIST(t *testing.T) {
	s := store.NewStore()
	ctx := discardCtx("RELATION.LIST", bytesArgs(), s)
	if err := cmdRELATIONLIST(ctx); err != nil {
		t.Fatal(err)
	}
}

// RELATION lifecycle
func TestCmdRELATION_Lifecycle(t *testing.T) {
	s := store.NewStore()
	ctx, buf := bufCtx("RELATION.CREATE", bytesArgs("x", "y", "linked"), s)
	if err := cmdRELATIONCREATE(ctx); err != nil {
		t.Fatal(err)
	}
	id := extractBulkString(buf.String())
	if id == "" {
		t.Fatal("could not extract relation ID")
	}
	ctx = discardCtx("RELATION.GET", bytesArgs(id), s)
	if err := cmdRELATIONGET(ctx); err != nil {
		t.Fatal(err)
	}
	ctx = discardCtx("RELATION.DELETE", bytesArgs(id), s)
	if err := cmdRELATIONDELETE(ctx); err != nil {
		t.Fatal(err)
	}
}

// ======================================================================
// CONNECTIONX COMMANDS
// ======================================================================

func TestCmdCONNECTIONXCREATE_Success(t *testing.T) {
	s := store.NewStore()
	ctx := discardCtx("CONNECTIONX.CREATE", bytesArgs("src", "dst"), s)
	if err := cmdCONNECTIONXCREATE(ctx); err != nil {
		t.Fatal(err)
	}
}

func TestCmdCONNECTIONXCREATE_TooFewArgs(t *testing.T) {
	s := store.NewStore()
	ctx := discardCtx("CONNECTIONX.CREATE", bytesArgs("src"), s)
	if err := cmdCONNECTIONXCREATE(ctx); err != nil {
		t.Fatal(err)
	}
}

func TestCmdCONNECTIONXDELETE_NotFound(t *testing.T) {
	s := store.NewStore()
	ctx := discardCtx("CONNECTIONX.DELETE", bytesArgs("nonexistent"), s)
	if err := cmdCONNECTIONXDELETE(ctx); err != nil {
		t.Fatal(err)
	}
}

func TestCmdCONNECTIONXDELETE_TooFewArgs(t *testing.T) {
	s := store.NewStore()
	ctx := discardCtx("CONNECTIONX.DELETE", bytesArgs(), s)
	if err := cmdCONNECTIONXDELETE(ctx); err != nil {
		t.Fatal(err)
	}
}

func TestCmdCONNECTIONXSTATUS_NotFound(t *testing.T) {
	s := store.NewStore()
	ctx := discardCtx("CONNECTIONX.STATUS", bytesArgs("nonexistent"), s)
	if err := cmdCONNECTIONXSTATUS(ctx); err != nil {
		t.Fatal(err)
	}
}

func TestCmdCONNECTIONXSTATUS_TooFewArgs(t *testing.T) {
	s := store.NewStore()
	ctx := discardCtx("CONNECTIONX.STATUS", bytesArgs(), s)
	if err := cmdCONNECTIONXSTATUS(ctx); err != nil {
		t.Fatal(err)
	}
}

func TestCmdCONNECTIONXLIST(t *testing.T) {
	s := store.NewStore()
	ctx := discardCtx("CONNECTIONX.LIST", bytesArgs(), s)
	if err := cmdCONNECTIONXLIST(ctx); err != nil {
		t.Fatal(err)
	}
}

// CONNECTIONX lifecycle
func TestCmdCONNECTIONX_Lifecycle(t *testing.T) {
	s := store.NewStore()
	ctx, buf := bufCtx("CONNECTIONX.CREATE", bytesArgs("host1", "host2"), s)
	if err := cmdCONNECTIONXCREATE(ctx); err != nil {
		t.Fatal(err)
	}
	id := extractBulkString(buf.String())
	if id == "" {
		t.Fatal("could not extract connection ID")
	}
	ctx = discardCtx("CONNECTIONX.STATUS", bytesArgs(id), s)
	if err := cmdCONNECTIONXSTATUS(ctx); err != nil {
		t.Fatal(err)
	}
	ctx = discardCtx("CONNECTIONX.DELETE", bytesArgs(id), s)
	if err := cmdCONNECTIONXDELETE(ctx); err != nil {
		t.Fatal(err)
	}
}

// ======================================================================
// POOLX COMMANDS
// ======================================================================

func TestCmdPOOLXCREATE_Success(t *testing.T) {
	s := store.NewStore()
	ctx := discardCtx("POOLX.CREATE", bytesArgs("mypool", "3"), s)
	if err := cmdPOOLXCREATE(ctx); err != nil {
		t.Fatal(err)
	}
}

func TestCmdPOOLXCREATE_TooFewArgs(t *testing.T) {
	s := store.NewStore()
	ctx := discardCtx("POOLX.CREATE", bytesArgs("mypool"), s)
	if err := cmdPOOLXCREATE(ctx); err != nil {
		t.Fatal(err)
	}
}

func TestCmdPOOLXDELETE_Success(t *testing.T) {
	s := store.NewStore()
	ctx := discardCtx("POOLX.CREATE", bytesArgs("delpool", "2"), s)
	cmdPOOLXCREATE(ctx)
	ctx = discardCtx("POOLX.DELETE", bytesArgs("delpool"), s)
	if err := cmdPOOLXDELETE(ctx); err != nil {
		t.Fatal(err)
	}
}

func TestCmdPOOLXDELETE_NotFound(t *testing.T) {
	s := store.NewStore()
	ctx := discardCtx("POOLX.DELETE", bytesArgs("nonexistent"), s)
	if err := cmdPOOLXDELETE(ctx); err != nil {
		t.Fatal(err)
	}
}

func TestCmdPOOLXDELETE_TooFewArgs(t *testing.T) {
	s := store.NewStore()
	ctx := discardCtx("POOLX.DELETE", bytesArgs(), s)
	if err := cmdPOOLXDELETE(ctx); err != nil {
		t.Fatal(err)
	}
}

func TestCmdPOOLXACQUIRE_Success(t *testing.T) {
	s := store.NewStore()
	ctx := discardCtx("POOLX.CREATE", bytesArgs("acqpool", "2"), s)
	cmdPOOLXCREATE(ctx)
	ctx = discardCtx("POOLX.ACQUIRE", bytesArgs("acqpool"), s)
	if err := cmdPOOLXACQUIRE(ctx); err != nil {
		t.Fatal(err)
	}
}

func TestCmdPOOLXACQUIRE_Empty(t *testing.T) {
	s := store.NewStore()
	ctx := discardCtx("POOLX.CREATE", bytesArgs("emptypool", "0"), s)
	cmdPOOLXCREATE(ctx)
	ctx = discardCtx("POOLX.ACQUIRE", bytesArgs("emptypool"), s)
	if err := cmdPOOLXACQUIRE(ctx); err != nil {
		t.Fatal(err)
	}
}

func TestCmdPOOLXACQUIRE_NotFound(t *testing.T) {
	s := store.NewStore()
	ctx := discardCtx("POOLX.ACQUIRE", bytesArgs("nonexistent"), s)
	if err := cmdPOOLXACQUIRE(ctx); err != nil {
		t.Fatal(err)
	}
}

func TestCmdPOOLXACQUIRE_TooFewArgs(t *testing.T) {
	s := store.NewStore()
	ctx := discardCtx("POOLX.ACQUIRE", bytesArgs(), s)
	if err := cmdPOOLXACQUIRE(ctx); err != nil {
		t.Fatal(err)
	}
}

func TestCmdPOOLXRELEASE_Success(t *testing.T) {
	s := store.NewStore()
	ctx := discardCtx("POOLX.CREATE", bytesArgs("relpool", "2"), s)
	cmdPOOLXCREATE(ctx)
	// Acquire resource-0
	ctx = discardCtx("POOLX.ACQUIRE", bytesArgs("relpool"), s)
	cmdPOOLXACQUIRE(ctx)
	// Release resource-0
	ctx = discardCtx("POOLX.RELEASE", bytesArgs("relpool", "resource-0"), s)
	if err := cmdPOOLXRELEASE(ctx); err != nil {
		t.Fatal(err)
	}
}

func TestCmdPOOLXRELEASE_NotInUse(t *testing.T) {
	s := store.NewStore()
	ctx := discardCtx("POOLX.CREATE", bytesArgs("relpool2", "2"), s)
	cmdPOOLXCREATE(ctx)
	ctx = discardCtx("POOLX.RELEASE", bytesArgs("relpool2", "resource-0"), s)
	if err := cmdPOOLXRELEASE(ctx); err != nil {
		t.Fatal(err)
	}
}

func TestCmdPOOLXRELEASE_NotFound(t *testing.T) {
	s := store.NewStore()
	ctx := discardCtx("POOLX.RELEASE", bytesArgs("nonexistent", "resource-0"), s)
	if err := cmdPOOLXRELEASE(ctx); err != nil {
		t.Fatal(err)
	}
}

func TestCmdPOOLXRELEASE_TooFewArgs(t *testing.T) {
	s := store.NewStore()
	ctx := discardCtx("POOLX.RELEASE", bytesArgs("mypool"), s)
	if err := cmdPOOLXRELEASE(ctx); err != nil {
		t.Fatal(err)
	}
}

func TestCmdPOOLXSTATUS_Success(t *testing.T) {
	s := store.NewStore()
	ctx := discardCtx("POOLX.CREATE", bytesArgs("statpool", "3"), s)
	cmdPOOLXCREATE(ctx)
	ctx = discardCtx("POOLX.STATUS", bytesArgs("statpool"), s)
	if err := cmdPOOLXSTATUS(ctx); err != nil {
		t.Fatal(err)
	}
}

func TestCmdPOOLXSTATUS_NotFound(t *testing.T) {
	s := store.NewStore()
	ctx := discardCtx("POOLX.STATUS", bytesArgs("nonexistent"), s)
	if err := cmdPOOLXSTATUS(ctx); err != nil {
		t.Fatal(err)
	}
}

func TestCmdPOOLXSTATUS_TooFewArgs(t *testing.T) {
	s := store.NewStore()
	ctx := discardCtx("POOLX.STATUS", bytesArgs(), s)
	if err := cmdPOOLXSTATUS(ctx); err != nil {
		t.Fatal(err)
	}
}

// ======================================================================
// BUFFERX COMMANDS
// ======================================================================

func TestCmdBUFFERXCREATE_Success(t *testing.T) {
	s := store.NewStore()
	ctx := discardCtx("BUFFERX.CREATE", bytesArgs("mybuf", "1024"), s)
	if err := cmdBUFFERXCREATE(ctx); err != nil {
		t.Fatal(err)
	}
}

func TestCmdBUFFERXCREATE_TooFewArgs(t *testing.T) {
	s := store.NewStore()
	ctx := discardCtx("BUFFERX.CREATE", bytesArgs("mybuf"), s)
	if err := cmdBUFFERXCREATE(ctx); err != nil {
		t.Fatal(err)
	}
}

func TestCmdBUFFERXWRITE_Success(t *testing.T) {
	s := store.NewStore()
	ctx := discardCtx("BUFFERX.CREATE", bytesArgs("wbuf", "0"), s)
	cmdBUFFERXCREATE(ctx)
	ctx = discardCtx("BUFFERX.WRITE", bytesArgs("wbuf", "hello"), s)
	if err := cmdBUFFERXWRITE(ctx); err != nil {
		t.Fatal(err)
	}
}

func TestCmdBUFFERXWRITE_NotFound(t *testing.T) {
	s := store.NewStore()
	ctx := discardCtx("BUFFERX.WRITE", bytesArgs("nonexistent", "data"), s)
	if err := cmdBUFFERXWRITE(ctx); err != nil {
		t.Fatal(err)
	}
}

func TestCmdBUFFERXWRITE_TooFewArgs(t *testing.T) {
	s := store.NewStore()
	ctx := discardCtx("BUFFERX.WRITE", bytesArgs("mybuf"), s)
	if err := cmdBUFFERXWRITE(ctx); err != nil {
		t.Fatal(err)
	}
}

func TestCmdBUFFERXREAD_Success(t *testing.T) {
	s := store.NewStore()
	ctx := discardCtx("BUFFERX.CREATE", bytesArgs("rbuf", "0"), s)
	cmdBUFFERXCREATE(ctx)
	ctx = discardCtx("BUFFERX.WRITE", bytesArgs("rbuf", "data"), s)
	cmdBUFFERXWRITE(ctx)
	ctx = discardCtx("BUFFERX.READ", bytesArgs("rbuf"), s)
	if err := cmdBUFFERXREAD(ctx); err != nil {
		t.Fatal(err)
	}
}

func TestCmdBUFFERXREAD_NotFound(t *testing.T) {
	s := store.NewStore()
	ctx := discardCtx("BUFFERX.READ", bytesArgs("nonexistent"), s)
	if err := cmdBUFFERXREAD(ctx); err != nil {
		t.Fatal(err)
	}
}

func TestCmdBUFFERXREAD_TooFewArgs(t *testing.T) {
	s := store.NewStore()
	ctx := discardCtx("BUFFERX.READ", bytesArgs(), s)
	if err := cmdBUFFERXREAD(ctx); err != nil {
		t.Fatal(err)
	}
}

func TestCmdBUFFERXDELETE_Success(t *testing.T) {
	s := store.NewStore()
	ctx := discardCtx("BUFFERX.CREATE", bytesArgs("delbuf", "0"), s)
	cmdBUFFERXCREATE(ctx)
	ctx = discardCtx("BUFFERX.DELETE", bytesArgs("delbuf"), s)
	if err := cmdBUFFERXDELETE(ctx); err != nil {
		t.Fatal(err)
	}
}

func TestCmdBUFFERXDELETE_NotFound(t *testing.T) {
	s := store.NewStore()
	ctx := discardCtx("BUFFERX.DELETE", bytesArgs("nonexistent"), s)
	if err := cmdBUFFERXDELETE(ctx); err != nil {
		t.Fatal(err)
	}
}

func TestCmdBUFFERXDELETE_TooFewArgs(t *testing.T) {
	s := store.NewStore()
	ctx := discardCtx("BUFFERX.DELETE", bytesArgs(), s)
	if err := cmdBUFFERXDELETE(ctx); err != nil {
		t.Fatal(err)
	}
}

// ======================================================================
// STREAMX COMMANDS
// ======================================================================

func TestCmdSTREAMXCREATE_Success(t *testing.T) {
	s := store.NewStore()
	ctx := discardCtx("STREAMX.CREATE", bytesArgs("mystream"), s)
	if err := cmdSTREAMXCREATE(ctx); err != nil {
		t.Fatal(err)
	}
}

func TestCmdSTREAMXCREATE_TooFewArgs(t *testing.T) {
	s := store.NewStore()
	ctx := discardCtx("STREAMX.CREATE", bytesArgs(), s)
	if err := cmdSTREAMXCREATE(ctx); err != nil {
		t.Fatal(err)
	}
}

func TestCmdSTREAMXWRITE_Success(t *testing.T) {
	s := store.NewStore()
	ctx := discardCtx("STREAMX.CREATE", bytesArgs("wstream"), s)
	cmdSTREAMXCREATE(ctx)
	ctx = discardCtx("STREAMX.WRITE", bytesArgs("wstream", "msg1"), s)
	if err := cmdSTREAMXWRITE(ctx); err != nil {
		t.Fatal(err)
	}
}

func TestCmdSTREAMXWRITE_NotFound(t *testing.T) {
	s := store.NewStore()
	ctx := discardCtx("STREAMX.WRITE", bytesArgs("nonexistent", "data"), s)
	if err := cmdSTREAMXWRITE(ctx); err != nil {
		t.Fatal(err)
	}
}

func TestCmdSTREAMXWRITE_TooFewArgs(t *testing.T) {
	s := store.NewStore()
	ctx := discardCtx("STREAMX.WRITE", bytesArgs("mystream"), s)
	if err := cmdSTREAMXWRITE(ctx); err != nil {
		t.Fatal(err)
	}
}

func TestCmdSTREAMXREAD_Success(t *testing.T) {
	s := store.NewStore()
	ctx := discardCtx("STREAMX.CREATE", bytesArgs("rstream"), s)
	cmdSTREAMXCREATE(ctx)
	ctx = discardCtx("STREAMX.WRITE", bytesArgs("rstream", "msg1"), s)
	cmdSTREAMXWRITE(ctx)
	ctx = discardCtx("STREAMX.READ", bytesArgs("rstream"), s)
	if err := cmdSTREAMXREAD(ctx); err != nil {
		t.Fatal(err)
	}
}

func TestCmdSTREAMXREAD_NotFound(t *testing.T) {
	s := store.NewStore()
	ctx := discardCtx("STREAMX.READ", bytesArgs("nonexistent"), s)
	if err := cmdSTREAMXREAD(ctx); err != nil {
		t.Fatal(err)
	}
}

func TestCmdSTREAMXREAD_TooFewArgs(t *testing.T) {
	s := store.NewStore()
	ctx := discardCtx("STREAMX.READ", bytesArgs(), s)
	if err := cmdSTREAMXREAD(ctx); err != nil {
		t.Fatal(err)
	}
}

func TestCmdSTREAMXDELETE_Success(t *testing.T) {
	s := store.NewStore()
	ctx := discardCtx("STREAMX.CREATE", bytesArgs("delstream"), s)
	cmdSTREAMXCREATE(ctx)
	ctx = discardCtx("STREAMX.DELETE", bytesArgs("delstream"), s)
	if err := cmdSTREAMXDELETE(ctx); err != nil {
		t.Fatal(err)
	}
}

func TestCmdSTREAMXDELETE_NotFound(t *testing.T) {
	s := store.NewStore()
	ctx := discardCtx("STREAMX.DELETE", bytesArgs("nonexistent"), s)
	if err := cmdSTREAMXDELETE(ctx); err != nil {
		t.Fatal(err)
	}
}

func TestCmdSTREAMXDELETE_TooFewArgs(t *testing.T) {
	s := store.NewStore()
	ctx := discardCtx("STREAMX.DELETE", bytesArgs(), s)
	if err := cmdSTREAMXDELETE(ctx); err != nil {
		t.Fatal(err)
	}
}

// ======================================================================
// EVENTX COMMANDS
// ======================================================================

func TestCmdEVENTXCREATE_Success(t *testing.T) {
	s := store.NewStore()
	ctx := discardCtx("EVENTX.CREATE", bytesArgs("myevent"), s)
	if err := cmdEVENTXCREATE(ctx); err != nil {
		t.Fatal(err)
	}
}

func TestCmdEVENTXCREATE_TooFewArgs(t *testing.T) {
	s := store.NewStore()
	ctx := discardCtx("EVENTX.CREATE", bytesArgs(), s)
	if err := cmdEVENTXCREATE(ctx); err != nil {
		t.Fatal(err)
	}
}

func TestCmdEVENTXDELETE_Success(t *testing.T) {
	s := store.NewStore()
	ctx := discardCtx("EVENTX.CREATE", bytesArgs("delevent"), s)
	cmdEVENTXCREATE(ctx)
	ctx = discardCtx("EVENTX.DELETE", bytesArgs("delevent"), s)
	if err := cmdEVENTXDELETE(ctx); err != nil {
		t.Fatal(err)
	}
}

func TestCmdEVENTXDELETE_NotFound(t *testing.T) {
	s := store.NewStore()
	ctx := discardCtx("EVENTX.DELETE", bytesArgs("nonexistent"), s)
	if err := cmdEVENTXDELETE(ctx); err != nil {
		t.Fatal(err)
	}
}

func TestCmdEVENTXDELETE_TooFewArgs(t *testing.T) {
	s := store.NewStore()
	ctx := discardCtx("EVENTX.DELETE", bytesArgs(), s)
	if err := cmdEVENTXDELETE(ctx); err != nil {
		t.Fatal(err)
	}
}

func TestCmdEVENTXEMIT_Success(t *testing.T) {
	s := store.NewStore()
	ctx := discardCtx("EVENTX.CREATE", bytesArgs("emitevent"), s)
	cmdEVENTXCREATE(ctx)
	ctx = discardCtx("EVENTX.EMIT", bytesArgs("emitevent", "payload"), s)
	if err := cmdEVENTXEMIT(ctx); err != nil {
		t.Fatal(err)
	}
}

func TestCmdEVENTXEMIT_NotFound(t *testing.T) {
	s := store.NewStore()
	ctx := discardCtx("EVENTX.EMIT", bytesArgs("nonexistent", "payload"), s)
	if err := cmdEVENTXEMIT(ctx); err != nil {
		t.Fatal(err)
	}
}

func TestCmdEVENTXEMIT_TooFewArgs(t *testing.T) {
	s := store.NewStore()
	ctx := discardCtx("EVENTX.EMIT", bytesArgs("myevent"), s)
	if err := cmdEVENTXEMIT(ctx); err != nil {
		t.Fatal(err)
	}
}

func TestCmdEVENTXSUBSCRIBE_Success(t *testing.T) {
	s := store.NewStore()
	ctx := discardCtx("EVENTX.CREATE", bytesArgs("subevent"), s)
	cmdEVENTXCREATE(ctx)
	ctx = discardCtx("EVENTX.SUBSCRIBE", bytesArgs("subevent", "client1"), s)
	if err := cmdEVENTXSUBSCRIBE(ctx); err != nil {
		t.Fatal(err)
	}
}

func TestCmdEVENTXSUBSCRIBE_NotFound(t *testing.T) {
	s := store.NewStore()
	ctx := discardCtx("EVENTX.SUBSCRIBE", bytesArgs("nonexistent", "client1"), s)
	if err := cmdEVENTXSUBSCRIBE(ctx); err != nil {
		t.Fatal(err)
	}
}

func TestCmdEVENTXSUBSCRIBE_TooFewArgs(t *testing.T) {
	s := store.NewStore()
	ctx := discardCtx("EVENTX.SUBSCRIBE", bytesArgs("myevent"), s)
	if err := cmdEVENTXSUBSCRIBE(ctx); err != nil {
		t.Fatal(err)
	}
}

func TestCmdEVENTXLIST(t *testing.T) {
	s := store.NewStore()
	ctx := discardCtx("EVENTX.LIST", bytesArgs(), s)
	if err := cmdEVENTXLIST(ctx); err != nil {
		t.Fatal(err)
	}
}

// ======================================================================
// HOOK COMMANDS
// ======================================================================

func TestCmdHOOKCREATE_Success(t *testing.T) {
	s := store.NewStore()
	ctx := discardCtx("HOOK.CREATE", bytesArgs("myhook", "on_set", "log"), s)
	if err := cmdHOOKCREATE(ctx); err != nil {
		t.Fatal(err)
	}
}

func TestCmdHOOKCREATE_TooFewArgs(t *testing.T) {
	s := store.NewStore()
	ctx := discardCtx("HOOK.CREATE", bytesArgs("myhook", "on_set"), s)
	if err := cmdHOOKCREATE(ctx); err != nil {
		t.Fatal(err)
	}
}

func TestCmdHOOKDELETE_NotFound(t *testing.T) {
	s := store.NewStore()
	ctx := discardCtx("HOOK.DELETE", bytesArgs("nonexistent"), s)
	if err := cmdHOOKDELETE(ctx); err != nil {
		t.Fatal(err)
	}
}

func TestCmdHOOKDELETE_TooFewArgs(t *testing.T) {
	s := store.NewStore()
	ctx := discardCtx("HOOK.DELETE", bytesArgs(), s)
	if err := cmdHOOKDELETE(ctx); err != nil {
		t.Fatal(err)
	}
}

func TestCmdHOOKTRIGGER_NotFound(t *testing.T) {
	s := store.NewStore()
	ctx := discardCtx("HOOK.TRIGGER", bytesArgs("nonexistent"), s)
	if err := cmdHOOKTRIGGER(ctx); err != nil {
		t.Fatal(err)
	}
}

func TestCmdHOOKTRIGGER_TooFewArgs(t *testing.T) {
	s := store.NewStore()
	ctx := discardCtx("HOOK.TRIGGER", bytesArgs(), s)
	if err := cmdHOOKTRIGGER(ctx); err != nil {
		t.Fatal(err)
	}
}

func TestCmdHOOKLIST(t *testing.T) {
	s := store.NewStore()
	ctx := discardCtx("HOOK.LIST", bytesArgs(), s)
	if err := cmdHOOKLIST(ctx); err != nil {
		t.Fatal(err)
	}
}

// HOOK lifecycle
func TestCmdHOOK_Lifecycle(t *testing.T) {
	s := store.NewStore()
	ctx, buf := bufCtx("HOOK.CREATE", bytesArgs("lchook", "on_del", "notify"), s)
	if err := cmdHOOKCREATE(ctx); err != nil {
		t.Fatal(err)
	}
	id := extractBulkString(buf.String())
	if id == "" {
		t.Fatal("could not extract hook ID")
	}
	ctx = discardCtx("HOOK.TRIGGER", bytesArgs(id), s)
	if err := cmdHOOKTRIGGER(ctx); err != nil {
		t.Fatal(err)
	}
	ctx = discardCtx("HOOK.DELETE", bytesArgs(id), s)
	if err := cmdHOOKDELETE(ctx); err != nil {
		t.Fatal(err)
	}
}

// ======================================================================
// MIDDLEWARE COMMANDS
// ======================================================================

func TestCmdMIDDLEWARECREATE_Success(t *testing.T) {
	s := store.NewStore()
	ctx := discardCtx("MIDDLEWARE.CREATE", bytesArgs("mymw", "auth"), s)
	if err := cmdMIDDLEWARECREATE(ctx); err != nil {
		t.Fatal(err)
	}
}

func TestCmdMIDDLEWARECREATE_WithAfter(t *testing.T) {
	s := store.NewStore()
	ctx := discardCtx("MIDDLEWARE.CREATE", bytesArgs("mymw2", "auth", "log"), s)
	if err := cmdMIDDLEWARECREATE(ctx); err != nil {
		t.Fatal(err)
	}
}

func TestCmdMIDDLEWARECREATE_TooFewArgs(t *testing.T) {
	s := store.NewStore()
	ctx := discardCtx("MIDDLEWARE.CREATE", bytesArgs("mymw"), s)
	if err := cmdMIDDLEWARECREATE(ctx); err != nil {
		t.Fatal(err)
	}
}

func TestCmdMIDDLEWAREDELETE_NotFound(t *testing.T) {
	s := store.NewStore()
	ctx := discardCtx("MIDDLEWARE.DELETE", bytesArgs("nonexistent"), s)
	if err := cmdMIDDLEWAREDELETE(ctx); err != nil {
		t.Fatal(err)
	}
}

func TestCmdMIDDLEWAREDELETE_TooFewArgs(t *testing.T) {
	s := store.NewStore()
	ctx := discardCtx("MIDDLEWARE.DELETE", bytesArgs(), s)
	if err := cmdMIDDLEWAREDELETE(ctx); err != nil {
		t.Fatal(err)
	}
}

func TestCmdMIDDLEWAREEXECUTE_Success(t *testing.T) {
	s := store.NewStore()
	ctx := discardCtx("MIDDLEWARE.EXECUTE", bytesArgs("id", "data"), s)
	if err := cmdMIDDLEWAREEXECUTE(ctx); err != nil {
		t.Fatal(err)
	}
}

func TestCmdMIDDLEWAREEXECUTE_TooFewArgs(t *testing.T) {
	s := store.NewStore()
	ctx := discardCtx("MIDDLEWARE.EXECUTE", bytesArgs("id"), s)
	if err := cmdMIDDLEWAREEXECUTE(ctx); err != nil {
		t.Fatal(err)
	}
}

func TestCmdMIDDLEWARELIST(t *testing.T) {
	s := store.NewStore()
	ctx := discardCtx("MIDDLEWARE.LIST", bytesArgs(), s)
	if err := cmdMIDDLEWARELIST(ctx); err != nil {
		t.Fatal(err)
	}
}

// MIDDLEWARE lifecycle with delete
func TestCmdMIDDLEWARE_Lifecycle(t *testing.T) {
	s := store.NewStore()
	ctx, buf := bufCtx("MIDDLEWARE.CREATE", bytesArgs("lcmw", "before_action"), s)
	if err := cmdMIDDLEWARECREATE(ctx); err != nil {
		t.Fatal(err)
	}
	id := extractBulkString(buf.String())
	if id == "" {
		t.Fatal("could not extract middleware ID")
	}
	ctx = discardCtx("MIDDLEWARE.DELETE", bytesArgs(id), s)
	if err := cmdMIDDLEWAREDELETE(ctx); err != nil {
		t.Fatal(err)
	}
}

// ======================================================================
// INTERCEPTOR COMMANDS
// ======================================================================

func TestCmdINTERCEPTORCREATE_Success(t *testing.T) {
	s := store.NewStore()
	ctx := discardCtx("INTERCEPTOR.CREATE", bytesArgs("myint", ".*"), s)
	if err := cmdINTERCEPTORCREATE(ctx); err != nil {
		t.Fatal(err)
	}
}

func TestCmdINTERCEPTORCREATE_TooFewArgs(t *testing.T) {
	s := store.NewStore()
	ctx := discardCtx("INTERCEPTOR.CREATE", bytesArgs("myint"), s)
	if err := cmdINTERCEPTORCREATE(ctx); err != nil {
		t.Fatal(err)
	}
}

func TestCmdINTERCEPTORDELETE_NotFound(t *testing.T) {
	s := store.NewStore()
	ctx := discardCtx("INTERCEPTOR.DELETE", bytesArgs("nonexistent"), s)
	if err := cmdINTERCEPTORDELETE(ctx); err != nil {
		t.Fatal(err)
	}
}

func TestCmdINTERCEPTORDELETE_TooFewArgs(t *testing.T) {
	s := store.NewStore()
	ctx := discardCtx("INTERCEPTOR.DELETE", bytesArgs(), s)
	if err := cmdINTERCEPTORDELETE(ctx); err != nil {
		t.Fatal(err)
	}
}

func TestCmdINTERCEPTORCHECK_Success(t *testing.T) {
	s := store.NewStore()
	ctx := discardCtx("INTERCEPTOR.CHECK", bytesArgs("id", "data"), s)
	if err := cmdINTERCEPTORCHECK(ctx); err != nil {
		t.Fatal(err)
	}
}

func TestCmdINTERCEPTORCHECK_TooFewArgs(t *testing.T) {
	s := store.NewStore()
	ctx := discardCtx("INTERCEPTOR.CHECK", bytesArgs("id"), s)
	if err := cmdINTERCEPTORCHECK(ctx); err != nil {
		t.Fatal(err)
	}
}

func TestCmdINTERCEPTORLIST(t *testing.T) {
	s := store.NewStore()
	ctx := discardCtx("INTERCEPTOR.LIST", bytesArgs(), s)
	if err := cmdINTERCEPTORLIST(ctx); err != nil {
		t.Fatal(err)
	}
}

// INTERCEPTOR lifecycle
func TestCmdINTERCEPTOR_Lifecycle(t *testing.T) {
	s := store.NewStore()
	ctx, buf := bufCtx("INTERCEPTOR.CREATE", bytesArgs("lcint", "GET.*"), s)
	if err := cmdINTERCEPTORCREATE(ctx); err != nil {
		t.Fatal(err)
	}
	id := extractBulkString(buf.String())
	if id == "" {
		t.Fatal("could not extract interceptor ID")
	}
	ctx = discardCtx("INTERCEPTOR.DELETE", bytesArgs(id), s)
	if err := cmdINTERCEPTORDELETE(ctx); err != nil {
		t.Fatal(err)
	}
}

// ======================================================================
// GUARD COMMANDS
// ======================================================================

func TestCmdGUARDCREATE_Success(t *testing.T) {
	s := store.NewStore()
	ctx := discardCtx("GUARD.CREATE", bytesArgs("myguard", "auth_required"), s)
	if err := cmdGUARDCREATE(ctx); err != nil {
		t.Fatal(err)
	}
}

func TestCmdGUARDCREATE_TooFewArgs(t *testing.T) {
	s := store.NewStore()
	ctx := discardCtx("GUARD.CREATE", bytesArgs("myguard"), s)
	if err := cmdGUARDCREATE(ctx); err != nil {
		t.Fatal(err)
	}
}

func TestCmdGUARDDELETE_NotFound(t *testing.T) {
	s := store.NewStore()
	ctx := discardCtx("GUARD.DELETE", bytesArgs("nonexistent"), s)
	if err := cmdGUARDDELETE(ctx); err != nil {
		t.Fatal(err)
	}
}

func TestCmdGUARDDELETE_TooFewArgs(t *testing.T) {
	s := store.NewStore()
	ctx := discardCtx("GUARD.DELETE", bytesArgs(), s)
	if err := cmdGUARDDELETE(ctx); err != nil {
		t.Fatal(err)
	}
}

func TestCmdGUARDCHECK_Success(t *testing.T) {
	s := store.NewStore()
	ctx := discardCtx("GUARD.CHECK", bytesArgs("id"), s)
	if err := cmdGUARDCHECK(ctx); err != nil {
		t.Fatal(err)
	}
}

func TestCmdGUARDCHECK_TooFewArgs(t *testing.T) {
	s := store.NewStore()
	ctx := discardCtx("GUARD.CHECK", bytesArgs(), s)
	if err := cmdGUARDCHECK(ctx); err != nil {
		t.Fatal(err)
	}
}

func TestCmdGUARDLIST(t *testing.T) {
	s := store.NewStore()
	ctx := discardCtx("GUARD.LIST", bytesArgs(), s)
	if err := cmdGUARDLIST(ctx); err != nil {
		t.Fatal(err)
	}
}

// GUARD lifecycle
func TestCmdGUARD_Lifecycle(t *testing.T) {
	s := store.NewStore()
	ctx, buf := bufCtx("GUARD.CREATE", bytesArgs("lcguard", "role_check"), s)
	if err := cmdGUARDCREATE(ctx); err != nil {
		t.Fatal(err)
	}
	id := extractBulkString(buf.String())
	if id == "" {
		t.Fatal("could not extract guard ID")
	}
	ctx = discardCtx("GUARD.DELETE", bytesArgs(id), s)
	if err := cmdGUARDDELETE(ctx); err != nil {
		t.Fatal(err)
	}
}

// ======================================================================
// PROXY COMMANDS
// ======================================================================

func TestCmdPROXYCREATE_Success(t *testing.T) {
	s := store.NewStore()
	ctx := discardCtx("PROXY.CREATE", bytesArgs("myproxy", "http://target"), s)
	if err := cmdPROXYCREATE(ctx); err != nil {
		t.Fatal(err)
	}
}

func TestCmdPROXYCREATE_TooFewArgs(t *testing.T) {
	s := store.NewStore()
	ctx := discardCtx("PROXY.CREATE", bytesArgs("myproxy"), s)
	if err := cmdPROXYCREATE(ctx); err != nil {
		t.Fatal(err)
	}
}

func TestCmdPROXYDELETE_NotFound(t *testing.T) {
	s := store.NewStore()
	ctx := discardCtx("PROXY.DELETE", bytesArgs("nonexistent"), s)
	if err := cmdPROXYDELETE(ctx); err != nil {
		t.Fatal(err)
	}
}

func TestCmdPROXYDELETE_TooFewArgs(t *testing.T) {
	s := store.NewStore()
	ctx := discardCtx("PROXY.DELETE", bytesArgs(), s)
	if err := cmdPROXYDELETE(ctx); err != nil {
		t.Fatal(err)
	}
}

func TestCmdPROXYROUTE_Success(t *testing.T) {
	s := store.NewStore()
	ctx := discardCtx("PROXY.ROUTE", bytesArgs("id", "/path"), s)
	if err := cmdPROXYROUTE(ctx); err != nil {
		t.Fatal(err)
	}
}

func TestCmdPROXYROUTE_TooFewArgs(t *testing.T) {
	s := store.NewStore()
	ctx := discardCtx("PROXY.ROUTE", bytesArgs("id"), s)
	if err := cmdPROXYROUTE(ctx); err != nil {
		t.Fatal(err)
	}
}

func TestCmdPROXYLIST(t *testing.T) {
	s := store.NewStore()
	ctx := discardCtx("PROXY.LIST", bytesArgs(), s)
	if err := cmdPROXYLIST(ctx); err != nil {
		t.Fatal(err)
	}
}

// PROXY lifecycle
func TestCmdPROXY_Lifecycle(t *testing.T) {
	s := store.NewStore()
	ctx, buf := bufCtx("PROXY.CREATE", bytesArgs("lcproxy", "http://backend"), s)
	if err := cmdPROXYCREATE(ctx); err != nil {
		t.Fatal(err)
	}
	id := extractBulkString(buf.String())
	if id == "" {
		t.Fatal("could not extract proxy ID")
	}
	ctx = discardCtx("PROXY.DELETE", bytesArgs(id), s)
	if err := cmdPROXYDELETE(ctx); err != nil {
		t.Fatal(err)
	}
}

// ======================================================================
// CACHEX COMMANDS
// ======================================================================

func TestCmdCACHEXCREATE_Success(t *testing.T) {
	s := store.NewStore()
	ctx := discardCtx("CACHEX.CREATE", bytesArgs("mycache"), s)
	if err := cmdCACHEXCREATE(ctx); err != nil {
		t.Fatal(err)
	}
}

func TestCmdCACHEXCREATE_TooFewArgs(t *testing.T) {
	s := store.NewStore()
	ctx := discardCtx("CACHEX.CREATE", bytesArgs(), s)
	if err := cmdCACHEXCREATE(ctx); err != nil {
		t.Fatal(err)
	}
}

func TestCmdCACHEXDELETE_Success(t *testing.T) {
	s := store.NewStore()
	ctx := discardCtx("CACHEX.CREATE", bytesArgs("delcache"), s)
	cmdCACHEXCREATE(ctx)
	ctx = discardCtx("CACHEX.DELETE", bytesArgs("delcache"), s)
	if err := cmdCACHEXDELETE(ctx); err != nil {
		t.Fatal(err)
	}
}

func TestCmdCACHEXDELETE_NotFound(t *testing.T) {
	s := store.NewStore()
	ctx := discardCtx("CACHEX.DELETE", bytesArgs("nonexistent"), s)
	if err := cmdCACHEXDELETE(ctx); err != nil {
		t.Fatal(err)
	}
}

func TestCmdCACHEXDELETE_TooFewArgs(t *testing.T) {
	s := store.NewStore()
	ctx := discardCtx("CACHEX.DELETE", bytesArgs(), s)
	if err := cmdCACHEXDELETE(ctx); err != nil {
		t.Fatal(err)
	}
}

func TestCmdCACHEXGET_Success(t *testing.T) {
	s := store.NewStore()
	ctx := discardCtx("CACHEX.CREATE", bytesArgs("getcache"), s)
	cmdCACHEXCREATE(ctx)
	ctx = discardCtx("CACHEX.SET", bytesArgs("getcache", "k1", "v1"), s)
	cmdCACHEXSET(ctx)
	ctx = discardCtx("CACHEX.GET", bytesArgs("getcache", "k1"), s)
	if err := cmdCACHEXGET(ctx); err != nil {
		t.Fatal(err)
	}
}

func TestCmdCACHEXGET_KeyMiss(t *testing.T) {
	s := store.NewStore()
	ctx := discardCtx("CACHEX.CREATE", bytesArgs("misscache"), s)
	cmdCACHEXCREATE(ctx)
	ctx = discardCtx("CACHEX.GET", bytesArgs("misscache", "nokey"), s)
	if err := cmdCACHEXGET(ctx); err != nil {
		t.Fatal(err)
	}
}

func TestCmdCACHEXGET_CacheMiss(t *testing.T) {
	s := store.NewStore()
	ctx := discardCtx("CACHEX.GET", bytesArgs("nonexistent", "k"), s)
	if err := cmdCACHEXGET(ctx); err != nil {
		t.Fatal(err)
	}
}

func TestCmdCACHEXGET_TooFewArgs(t *testing.T) {
	s := store.NewStore()
	ctx := discardCtx("CACHEX.GET", bytesArgs("cache"), s)
	if err := cmdCACHEXGET(ctx); err != nil {
		t.Fatal(err)
	}
}

func TestCmdCACHEXSET_Success(t *testing.T) {
	s := store.NewStore()
	ctx := discardCtx("CACHEX.CREATE", bytesArgs("setcache"), s)
	cmdCACHEXCREATE(ctx)
	ctx = discardCtx("CACHEX.SET", bytesArgs("setcache", "k1", "v1"), s)
	if err := cmdCACHEXSET(ctx); err != nil {
		t.Fatal(err)
	}
}

func TestCmdCACHEXSET_NotFound(t *testing.T) {
	s := store.NewStore()
	ctx := discardCtx("CACHEX.SET", bytesArgs("nonexistent", "k", "v"), s)
	if err := cmdCACHEXSET(ctx); err != nil {
		t.Fatal(err)
	}
}

func TestCmdCACHEXSET_TooFewArgs(t *testing.T) {
	s := store.NewStore()
	ctx := discardCtx("CACHEX.SET", bytesArgs("cache", "k"), s)
	if err := cmdCACHEXSET(ctx); err != nil {
		t.Fatal(err)
	}
}

func TestCmdCACHEXLIST(t *testing.T) {
	s := store.NewStore()
	ctx := discardCtx("CACHEX.LIST", bytesArgs(), s)
	if err := cmdCACHEXLIST(ctx); err != nil {
		t.Fatal(err)
	}
}

// ======================================================================
// STOREX COMMANDS
// ======================================================================

func TestCmdSTOREXCREATE_Success(t *testing.T) {
	s := store.NewStore()
	ctx := discardCtx("STOREX.CREATE", bytesArgs("mystore"), s)
	if err := cmdSTOREXCREATE(ctx); err != nil {
		t.Fatal(err)
	}
}

func TestCmdSTOREXCREATE_TooFewArgs(t *testing.T) {
	s := store.NewStore()
	ctx := discardCtx("STOREX.CREATE", bytesArgs(), s)
	if err := cmdSTOREXCREATE(ctx); err != nil {
		t.Fatal(err)
	}
}

func TestCmdSTOREXDELETE_Success(t *testing.T) {
	s := store.NewStore()
	ctx := discardCtx("STOREX.CREATE", bytesArgs("delstore"), s)
	cmdSTOREXCREATE(ctx)
	ctx = discardCtx("STOREX.DELETE", bytesArgs("delstore"), s)
	if err := cmdSTOREXDELETE(ctx); err != nil {
		t.Fatal(err)
	}
}

func TestCmdSTOREXDELETE_NotFound(t *testing.T) {
	s := store.NewStore()
	ctx := discardCtx("STOREX.DELETE", bytesArgs("nonexistent"), s)
	if err := cmdSTOREXDELETE(ctx); err != nil {
		t.Fatal(err)
	}
}

func TestCmdSTOREXDELETE_TooFewArgs(t *testing.T) {
	s := store.NewStore()
	ctx := discardCtx("STOREX.DELETE", bytesArgs(), s)
	if err := cmdSTOREXDELETE(ctx); err != nil {
		t.Fatal(err)
	}
}

func TestCmdSTOREXPUT_Success(t *testing.T) {
	s := store.NewStore()
	ctx := discardCtx("STOREX.CREATE", bytesArgs("putstore"), s)
	cmdSTOREXCREATE(ctx)
	ctx = discardCtx("STOREX.PUT", bytesArgs("putstore", "k1", "v1"), s)
	if err := cmdSTOREXPUT(ctx); err != nil {
		t.Fatal(err)
	}
}

func TestCmdSTOREXPUT_NotFound(t *testing.T) {
	s := store.NewStore()
	ctx := discardCtx("STOREX.PUT", bytesArgs("nonexistent", "k", "v"), s)
	if err := cmdSTOREXPUT(ctx); err != nil {
		t.Fatal(err)
	}
}

func TestCmdSTOREXPUT_TooFewArgs(t *testing.T) {
	s := store.NewStore()
	ctx := discardCtx("STOREX.PUT", bytesArgs("store", "k"), s)
	if err := cmdSTOREXPUT(ctx); err != nil {
		t.Fatal(err)
	}
}

func TestCmdSTOREXGET_Success(t *testing.T) {
	s := store.NewStore()
	ctx := discardCtx("STOREX.CREATE", bytesArgs("getstore"), s)
	cmdSTOREXCREATE(ctx)
	ctx = discardCtx("STOREX.PUT", bytesArgs("getstore", "k1", "v1"), s)
	cmdSTOREXPUT(ctx)
	ctx = discardCtx("STOREX.GET", bytesArgs("getstore", "k1"), s)
	if err := cmdSTOREXGET(ctx); err != nil {
		t.Fatal(err)
	}
}

func TestCmdSTOREXGET_KeyMiss(t *testing.T) {
	s := store.NewStore()
	ctx := discardCtx("STOREX.CREATE", bytesArgs("missstore"), s)
	cmdSTOREXCREATE(ctx)
	ctx = discardCtx("STOREX.GET", bytesArgs("missstore", "nokey"), s)
	if err := cmdSTOREXGET(ctx); err != nil {
		t.Fatal(err)
	}
}

func TestCmdSTOREXGET_StoreMiss(t *testing.T) {
	s := store.NewStore()
	ctx := discardCtx("STOREX.GET", bytesArgs("nonexistent", "k"), s)
	if err := cmdSTOREXGET(ctx); err != nil {
		t.Fatal(err)
	}
}

func TestCmdSTOREXGET_TooFewArgs(t *testing.T) {
	s := store.NewStore()
	ctx := discardCtx("STOREX.GET", bytesArgs("store"), s)
	if err := cmdSTOREXGET(ctx); err != nil {
		t.Fatal(err)
	}
}

func TestCmdSTOREXLIST(t *testing.T) {
	s := store.NewStore()
	ctx := discardCtx("STOREX.LIST", bytesArgs(), s)
	if err := cmdSTOREXLIST(ctx); err != nil {
		t.Fatal(err)
	}
}

// ======================================================================
// INDEX COMMANDS
// ======================================================================

func TestCmdINDEXCREATE_Success(t *testing.T) {
	s := store.NewStore()
	ctx := discardCtx("INDEX.CREATE", bytesArgs("myidx"), s)
	if err := cmdINDEXCREATE(ctx); err != nil {
		t.Fatal(err)
	}
}

func TestCmdINDEXCREATE_TooFewArgs(t *testing.T) {
	s := store.NewStore()
	ctx := discardCtx("INDEX.CREATE", bytesArgs(), s)
	if err := cmdINDEXCREATE(ctx); err != nil {
		t.Fatal(err)
	}
}

func TestCmdINDEXDELETE_Success(t *testing.T) {
	s := store.NewStore()
	ctx := discardCtx("INDEX.CREATE", bytesArgs("delidx"), s)
	cmdINDEXCREATE(ctx)
	ctx = discardCtx("INDEX.DELETE", bytesArgs("delidx"), s)
	if err := cmdINDEXDELETE(ctx); err != nil {
		t.Fatal(err)
	}
}

func TestCmdINDEXDELETE_NotFound(t *testing.T) {
	s := store.NewStore()
	ctx := discardCtx("INDEX.DELETE", bytesArgs("nonexistent"), s)
	if err := cmdINDEXDELETE(ctx); err != nil {
		t.Fatal(err)
	}
}

func TestCmdINDEXDELETE_TooFewArgs(t *testing.T) {
	s := store.NewStore()
	ctx := discardCtx("INDEX.DELETE", bytesArgs(), s)
	if err := cmdINDEXDELETE(ctx); err != nil {
		t.Fatal(err)
	}
}

func TestCmdINDEXADD_Success(t *testing.T) {
	s := store.NewStore()
	ctx := discardCtx("INDEX.CREATE", bytesArgs("addidx"), s)
	cmdINDEXCREATE(ctx)
	ctx = discardCtx("INDEX.ADD", bytesArgs("addidx", "name", "doc1"), s)
	if err := cmdINDEXADD(ctx); err != nil {
		t.Fatal(err)
	}
}

func TestCmdINDEXADD_NotFound(t *testing.T) {
	s := store.NewStore()
	ctx := discardCtx("INDEX.ADD", bytesArgs("nonexistent", "key", "id"), s)
	if err := cmdINDEXADD(ctx); err != nil {
		t.Fatal(err)
	}
}

func TestCmdINDEXADD_TooFewArgs(t *testing.T) {
	s := store.NewStore()
	ctx := discardCtx("INDEX.ADD", bytesArgs("idx", "key"), s)
	if err := cmdINDEXADD(ctx); err != nil {
		t.Fatal(err)
	}
}

func TestCmdINDEXSEARCH_Success(t *testing.T) {
	s := store.NewStore()
	ctx := discardCtx("INDEX.CREATE", bytesArgs("searchidx"), s)
	cmdINDEXCREATE(ctx)
	ctx = discardCtx("INDEX.ADD", bytesArgs("searchidx", "name", "doc1"), s)
	cmdINDEXADD(ctx)
	ctx = discardCtx("INDEX.SEARCH", bytesArgs("searchidx", "name"), s)
	if err := cmdINDEXSEARCH(ctx); err != nil {
		t.Fatal(err)
	}
}

func TestCmdINDEXSEARCH_NoMatch(t *testing.T) {
	s := store.NewStore()
	ctx := discardCtx("INDEX.CREATE", bytesArgs("nosearchidx"), s)
	cmdINDEXCREATE(ctx)
	ctx = discardCtx("INDEX.SEARCH", bytesArgs("nosearchidx", "nokey"), s)
	if err := cmdINDEXSEARCH(ctx); err != nil {
		t.Fatal(err)
	}
}

func TestCmdINDEXSEARCH_NotFound(t *testing.T) {
	s := store.NewStore()
	ctx := discardCtx("INDEX.SEARCH", bytesArgs("nonexistent", "key"), s)
	if err := cmdINDEXSEARCH(ctx); err != nil {
		t.Fatal(err)
	}
}

func TestCmdINDEXSEARCH_TooFewArgs(t *testing.T) {
	s := store.NewStore()
	ctx := discardCtx("INDEX.SEARCH", bytesArgs("idx"), s)
	if err := cmdINDEXSEARCH(ctx); err != nil {
		t.Fatal(err)
	}
}

func TestCmdINDEXLIST(t *testing.T) {
	s := store.NewStore()
	ctx := discardCtx("INDEX.LIST", bytesArgs(), s)
	if err := cmdINDEXLIST(ctx); err != nil {
		t.Fatal(err)
	}
}

// ======================================================================
// QUERY COMMANDS
// ======================================================================

func TestCmdQUERYCREATE_Success(t *testing.T) {
	s := store.NewStore()
	ctx := discardCtx("QUERY.CREATE", bytesArgs("myquery", "SELECT * FROM data"), s)
	if err := cmdQUERYCREATE(ctx); err != nil {
		t.Fatal(err)
	}
}

func TestCmdQUERYCREATE_TooFewArgs(t *testing.T) {
	s := store.NewStore()
	ctx := discardCtx("QUERY.CREATE", bytesArgs("myquery"), s)
	if err := cmdQUERYCREATE(ctx); err != nil {
		t.Fatal(err)
	}
}

func TestCmdQUERYDELETE_Success(t *testing.T) {
	s := store.NewStore()
	ctx := discardCtx("QUERY.CREATE", bytesArgs("delquery", "SELECT 1"), s)
	cmdQUERYCREATE(ctx)
	ctx = discardCtx("QUERY.DELETE", bytesArgs("delquery"), s)
	if err := cmdQUERYDELETE(ctx); err != nil {
		t.Fatal(err)
	}
}

func TestCmdQUERYDELETE_NotFound(t *testing.T) {
	s := store.NewStore()
	ctx := discardCtx("QUERY.DELETE", bytesArgs("nonexistent"), s)
	if err := cmdQUERYDELETE(ctx); err != nil {
		t.Fatal(err)
	}
}

func TestCmdQUERYDELETE_TooFewArgs(t *testing.T) {
	s := store.NewStore()
	ctx := discardCtx("QUERY.DELETE", bytesArgs(), s)
	if err := cmdQUERYDELETE(ctx); err != nil {
		t.Fatal(err)
	}
}

func TestCmdQUERYEXECUTE_Success(t *testing.T) {
	s := store.NewStore()
	ctx := discardCtx("QUERY.EXECUTE", bytesArgs("myquery"), s)
	if err := cmdQUERYEXECUTE(ctx); err != nil {
		t.Fatal(err)
	}
}

func TestCmdQUERYEXECUTE_TooFewArgs(t *testing.T) {
	s := store.NewStore()
	ctx := discardCtx("QUERY.EXECUTE", bytesArgs(), s)
	if err := cmdQUERYEXECUTE(ctx); err != nil {
		t.Fatal(err)
	}
}

func TestCmdQUERYLIST(t *testing.T) {
	s := store.NewStore()
	ctx := discardCtx("QUERY.LIST", bytesArgs(), s)
	if err := cmdQUERYLIST(ctx); err != nil {
		t.Fatal(err)
	}
}

// ======================================================================
// VIEW COMMANDS
// ======================================================================

func TestCmdVIEWCREATE_Success(t *testing.T) {
	s := store.NewStore()
	ctx := discardCtx("VIEW.CREATE", bytesArgs("myview", "SELECT * FROM data"), s)
	if err := cmdVIEWCREATE(ctx); err != nil {
		t.Fatal(err)
	}
}

func TestCmdVIEWCREATE_TooFewArgs(t *testing.T) {
	s := store.NewStore()
	ctx := discardCtx("VIEW.CREATE", bytesArgs("myview"), s)
	if err := cmdVIEWCREATE(ctx); err != nil {
		t.Fatal(err)
	}
}

func TestCmdVIEWDELETE_Success(t *testing.T) {
	s := store.NewStore()
	ctx := discardCtx("VIEW.CREATE", bytesArgs("delview", "SELECT 1"), s)
	cmdVIEWCREATE(ctx)
	ctx = discardCtx("VIEW.DELETE", bytesArgs("delview"), s)
	if err := cmdVIEWDELETE(ctx); err != nil {
		t.Fatal(err)
	}
}

func TestCmdVIEWDELETE_NotFound(t *testing.T) {
	s := store.NewStore()
	ctx := discardCtx("VIEW.DELETE", bytesArgs("nonexistent"), s)
	if err := cmdVIEWDELETE(ctx); err != nil {
		t.Fatal(err)
	}
}

func TestCmdVIEWDELETE_TooFewArgs(t *testing.T) {
	s := store.NewStore()
	ctx := discardCtx("VIEW.DELETE", bytesArgs(), s)
	if err := cmdVIEWDELETE(ctx); err != nil {
		t.Fatal(err)
	}
}

func TestCmdVIEWGET_Success(t *testing.T) {
	s := store.NewStore()
	ctx := discardCtx("VIEW.CREATE", bytesArgs("getview", "SELECT 1"), s)
	cmdVIEWCREATE(ctx)
	ctx = discardCtx("VIEW.GET", bytesArgs("getview"), s)
	if err := cmdVIEWGET(ctx); err != nil {
		t.Fatal(err)
	}
}

func TestCmdVIEWGET_NotFound(t *testing.T) {
	s := store.NewStore()
	ctx := discardCtx("VIEW.GET", bytesArgs("nonexistent"), s)
	if err := cmdVIEWGET(ctx); err != nil {
		t.Fatal(err)
	}
}

func TestCmdVIEWGET_TooFewArgs(t *testing.T) {
	s := store.NewStore()
	ctx := discardCtx("VIEW.GET", bytesArgs(), s)
	if err := cmdVIEWGET(ctx); err != nil {
		t.Fatal(err)
	}
}

func TestCmdVIEWLIST(t *testing.T) {
	s := store.NewStore()
	ctx := discardCtx("VIEW.LIST", bytesArgs(), s)
	if err := cmdVIEWLIST(ctx); err != nil {
		t.Fatal(err)
	}
}

// ======================================================================
// REPORT COMMANDS
// ======================================================================

func TestCmdREPORTCREATE_Success(t *testing.T) {
	s := store.NewStore()
	ctx := discardCtx("REPORT.CREATE", bytesArgs("myreport", "template1"), s)
	if err := cmdREPORTCREATE(ctx); err != nil {
		t.Fatal(err)
	}
}

func TestCmdREPORTCREATE_TooFewArgs(t *testing.T) {
	s := store.NewStore()
	ctx := discardCtx("REPORT.CREATE", bytesArgs("myreport"), s)
	if err := cmdREPORTCREATE(ctx); err != nil {
		t.Fatal(err)
	}
}

func TestCmdREPORTDELETE_NotFound(t *testing.T) {
	s := store.NewStore()
	ctx := discardCtx("REPORT.DELETE", bytesArgs("nonexistent"), s)
	if err := cmdREPORTDELETE(ctx); err != nil {
		t.Fatal(err)
	}
}

func TestCmdREPORTDELETE_TooFewArgs(t *testing.T) {
	s := store.NewStore()
	ctx := discardCtx("REPORT.DELETE", bytesArgs(), s)
	if err := cmdREPORTDELETE(ctx); err != nil {
		t.Fatal(err)
	}
}

func TestCmdREPORTGENERATE_Success(t *testing.T) {
	s := store.NewStore()
	ctx := discardCtx("REPORT.GENERATE", bytesArgs("id"), s)
	if err := cmdREPORTGENERATE(ctx); err != nil {
		t.Fatal(err)
	}
}

func TestCmdREPORTGENERATE_TooFewArgs(t *testing.T) {
	s := store.NewStore()
	ctx := discardCtx("REPORT.GENERATE", bytesArgs(), s)
	if err := cmdREPORTGENERATE(ctx); err != nil {
		t.Fatal(err)
	}
}

func TestCmdREPORTLIST(t *testing.T) {
	s := store.NewStore()
	ctx := discardCtx("REPORT.LIST", bytesArgs(), s)
	if err := cmdREPORTLIST(ctx); err != nil {
		t.Fatal(err)
	}
}

// REPORT lifecycle
func TestCmdREPORT_Lifecycle(t *testing.T) {
	s := store.NewStore()
	ctx, buf := bufCtx("REPORT.CREATE", bytesArgs("lcrep", "tmpl"), s)
	if err := cmdREPORTCREATE(ctx); err != nil {
		t.Fatal(err)
	}
	id := extractBulkString(buf.String())
	if id == "" {
		t.Fatal("could not extract report ID")
	}
	ctx = discardCtx("REPORT.DELETE", bytesArgs(id), s)
	if err := cmdREPORTDELETE(ctx); err != nil {
		t.Fatal(err)
	}
}

// ======================================================================
// AUDITX COMMANDS
// ======================================================================

func TestCmdAUDITXLOG_Success(t *testing.T) {
	s := store.NewStore()
	ctx := discardCtx("AUDITX.LOG", bytesArgs("mylog", "login", "alice"), s)
	if err := cmdAUDITXLOG(ctx); err != nil {
		t.Fatal(err)
	}
}

func TestCmdAUDITXLOG_WithResource(t *testing.T) {
	s := store.NewStore()
	ctx := discardCtx("AUDITX.LOG", bytesArgs("mylog2", "read", "bob", "db"), s)
	if err := cmdAUDITXLOG(ctx); err != nil {
		t.Fatal(err)
	}
}

func TestCmdAUDITXLOG_TooFewArgs(t *testing.T) {
	s := store.NewStore()
	ctx := discardCtx("AUDITX.LOG", bytesArgs("mylog", "login"), s)
	if err := cmdAUDITXLOG(ctx); err != nil {
		t.Fatal(err)
	}
}

func TestCmdAUDITXGET_Success(t *testing.T) {
	s := store.NewStore()
	ctx := discardCtx("AUDITX.LOG", bytesArgs("getlog", "action", "user"), s)
	cmdAUDITXLOG(ctx)
	ctx = discardCtx("AUDITX.GET", bytesArgs("getlog"), s)
	if err := cmdAUDITXGET(ctx); err != nil {
		t.Fatal(err)
	}
}

func TestCmdAUDITXGET_NotFound(t *testing.T) {
	s := store.NewStore()
	ctx := discardCtx("AUDITX.GET", bytesArgs("nonexistent"), s)
	if err := cmdAUDITXGET(ctx); err != nil {
		t.Fatal(err)
	}
}

func TestCmdAUDITXGET_TooFewArgs(t *testing.T) {
	s := store.NewStore()
	ctx := discardCtx("AUDITX.GET", bytesArgs(), s)
	if err := cmdAUDITXGET(ctx); err != nil {
		t.Fatal(err)
	}
}

func TestCmdAUDITXSEARCH_ByAction(t *testing.T) {
	s := store.NewStore()
	ctx := discardCtx("AUDITX.LOG", bytesArgs("searchlog", "login", "alice"), s)
	cmdAUDITXLOG(ctx)
	ctx = discardCtx("AUDITX.SEARCH", bytesArgs("searchlog", "login"), s)
	if err := cmdAUDITXSEARCH(ctx); err != nil {
		t.Fatal(err)
	}
}

func TestCmdAUDITXSEARCH_ByUser(t *testing.T) {
	s := store.NewStore()
	ctx := discardCtx("AUDITX.LOG", bytesArgs("searchlog2", "read", "bob"), s)
	cmdAUDITXLOG(ctx)
	ctx = discardCtx("AUDITX.SEARCH", bytesArgs("searchlog2", "bob"), s)
	if err := cmdAUDITXSEARCH(ctx); err != nil {
		t.Fatal(err)
	}
}

func TestCmdAUDITXSEARCH_NotFound(t *testing.T) {
	s := store.NewStore()
	ctx := discardCtx("AUDITX.SEARCH", bytesArgs("nonexistent", "query"), s)
	if err := cmdAUDITXSEARCH(ctx); err != nil {
		t.Fatal(err)
	}
}

func TestCmdAUDITXSEARCH_TooFewArgs(t *testing.T) {
	s := store.NewStore()
	ctx := discardCtx("AUDITX.SEARCH", bytesArgs("log"), s)
	if err := cmdAUDITXSEARCH(ctx); err != nil {
		t.Fatal(err)
	}
}

func TestCmdAUDITXLIST(t *testing.T) {
	s := store.NewStore()
	ctx := discardCtx("AUDITX.LIST", bytesArgs(), s)
	if err := cmdAUDITXLIST(ctx); err != nil {
		t.Fatal(err)
	}
}

// ======================================================================
// TOKEN COMMANDS
// ======================================================================

func TestCmdTOKENCREATE_Success(t *testing.T) {
	s := store.NewStore()
	ctx := discardCtx("TOKEN.CREATE", bytesArgs("alice", "3600000"), s)
	if err := cmdTOKENCREATE(ctx); err != nil {
		t.Fatal(err)
	}
}

func TestCmdTOKENCREATE_TooFewArgs(t *testing.T) {
	s := store.NewStore()
	ctx := discardCtx("TOKEN.CREATE", bytesArgs("alice"), s)
	if err := cmdTOKENCREATE(ctx); err != nil {
		t.Fatal(err)
	}
}

func TestCmdTOKENDELETE_NotFound(t *testing.T) {
	s := store.NewStore()
	ctx := discardCtx("TOKEN.DELETE", bytesArgs("nonexistent"), s)
	if err := cmdTOKENDELETE(ctx); err != nil {
		t.Fatal(err)
	}
}

func TestCmdTOKENDELETE_TooFewArgs(t *testing.T) {
	s := store.NewStore()
	ctx := discardCtx("TOKEN.DELETE", bytesArgs(), s)
	if err := cmdTOKENDELETE(ctx); err != nil {
		t.Fatal(err)
	}
}

func TestCmdTOKENVALIDATE_NotFound(t *testing.T) {
	s := store.NewStore()
	ctx := discardCtx("TOKEN.VALIDATE", bytesArgs("nonexistent"), s)
	if err := cmdTOKENVALIDATE(ctx); err != nil {
		t.Fatal(err)
	}
}

func TestCmdTOKENVALIDATE_TooFewArgs(t *testing.T) {
	s := store.NewStore()
	ctx := discardCtx("TOKEN.VALIDATE", bytesArgs(), s)
	if err := cmdTOKENVALIDATE(ctx); err != nil {
		t.Fatal(err)
	}
}

func TestCmdTOKENREFRESH_NotFound(t *testing.T) {
	s := store.NewStore()
	ctx := discardCtx("TOKEN.REFRESH", bytesArgs("nonexistent", "1000"), s)
	if err := cmdTOKENREFRESH(ctx); err != nil {
		t.Fatal(err)
	}
}

func TestCmdTOKENREFRESH_TooFewArgs(t *testing.T) {
	s := store.NewStore()
	ctx := discardCtx("TOKEN.REFRESH", bytesArgs("id"), s)
	if err := cmdTOKENREFRESH(ctx); err != nil {
		t.Fatal(err)
	}
}

func TestCmdTOKENLIST(t *testing.T) {
	s := store.NewStore()
	ctx := discardCtx("TOKEN.LIST", bytesArgs(), s)
	if err := cmdTOKENLIST(ctx); err != nil {
		t.Fatal(err)
	}
}

// TOKEN lifecycle
func TestCmdTOKEN_Lifecycle(t *testing.T) {
	s := store.NewStore()
	ctx, buf := bufCtx("TOKEN.CREATE", bytesArgs("alice", "9999999"), s)
	if err := cmdTOKENCREATE(ctx); err != nil {
		t.Fatal(err)
	}
	id := extractBulkString(buf.String())
	if id == "" {
		t.Fatal("could not extract token ID")
	}
	ctx = discardCtx("TOKEN.VALIDATE", bytesArgs(id), s)
	if err := cmdTOKENVALIDATE(ctx); err != nil {
		t.Fatal(err)
	}
	ctx = discardCtx("TOKEN.REFRESH", bytesArgs(id, "9999999"), s)
	if err := cmdTOKENREFRESH(ctx); err != nil {
		t.Fatal(err)
	}
	ctx = discardCtx("TOKEN.DELETE", bytesArgs(id), s)
	if err := cmdTOKENDELETE(ctx); err != nil {
		t.Fatal(err)
	}
}

// ======================================================================
// SESSIONX COMMANDS
// ======================================================================

func TestCmdSESSIONXCREATE_Success(t *testing.T) {
	s := store.NewStore()
	ctx := discardCtx("SESSIONX.CREATE", bytesArgs("alice", "3600000"), s)
	if err := cmdSESSIONXCREATE(ctx); err != nil {
		t.Fatal(err)
	}
}

func TestCmdSESSIONXCREATE_TooFewArgs(t *testing.T) {
	s := store.NewStore()
	ctx := discardCtx("SESSIONX.CREATE", bytesArgs("alice"), s)
	if err := cmdSESSIONXCREATE(ctx); err != nil {
		t.Fatal(err)
	}
}

func TestCmdSESSIONXDELETE_NotFound(t *testing.T) {
	s := store.NewStore()
	ctx := discardCtx("SESSIONX.DELETE", bytesArgs("nonexistent"), s)
	if err := cmdSESSIONXDELETE(ctx); err != nil {
		t.Fatal(err)
	}
}

func TestCmdSESSIONXDELETE_TooFewArgs(t *testing.T) {
	s := store.NewStore()
	ctx := discardCtx("SESSIONX.DELETE", bytesArgs(), s)
	if err := cmdSESSIONXDELETE(ctx); err != nil {
		t.Fatal(err)
	}
}

func TestCmdSESSIONXGET_NotFound(t *testing.T) {
	s := store.NewStore()
	ctx := discardCtx("SESSIONX.GET", bytesArgs("nonexistent"), s)
	if err := cmdSESSIONXGET(ctx); err != nil {
		t.Fatal(err)
	}
}

func TestCmdSESSIONXGET_TooFewArgs(t *testing.T) {
	s := store.NewStore()
	ctx := discardCtx("SESSIONX.GET", bytesArgs(), s)
	if err := cmdSESSIONXGET(ctx); err != nil {
		t.Fatal(err)
	}
}

func TestCmdSESSIONXSET_NotFound(t *testing.T) {
	s := store.NewStore()
	ctx := discardCtx("SESSIONX.SET", bytesArgs("nonexistent", "key", "val"), s)
	if err := cmdSESSIONXSET(ctx); err != nil {
		t.Fatal(err)
	}
}

func TestCmdSESSIONXSET_TooFewArgs(t *testing.T) {
	s := store.NewStore()
	ctx := discardCtx("SESSIONX.SET", bytesArgs("id", "key"), s)
	if err := cmdSESSIONXSET(ctx); err != nil {
		t.Fatal(err)
	}
}

func TestCmdSESSIONXLIST(t *testing.T) {
	s := store.NewStore()
	ctx := discardCtx("SESSIONX.LIST", bytesArgs(), s)
	if err := cmdSESSIONXLIST(ctx); err != nil {
		t.Fatal(err)
	}
}

// SESSIONX lifecycle
func TestCmdSESSIONX_Lifecycle(t *testing.T) {
	s := store.NewStore()
	ctx, buf := bufCtx("SESSIONX.CREATE", bytesArgs("bob", "9999999"), s)
	if err := cmdSESSIONXCREATE(ctx); err != nil {
		t.Fatal(err)
	}
	id := extractBulkString(buf.String())
	if id == "" {
		t.Fatal("could not extract session ID")
	}
	ctx = discardCtx("SESSIONX.SET", bytesArgs(id, "theme", "dark"), s)
	if err := cmdSESSIONXSET(ctx); err != nil {
		t.Fatal(err)
	}
	ctx = discardCtx("SESSIONX.GET", bytesArgs(id), s)
	if err := cmdSESSIONXGET(ctx); err != nil {
		t.Fatal(err)
	}
	ctx = discardCtx("SESSIONX.DELETE", bytesArgs(id), s)
	if err := cmdSESSIONXDELETE(ctx); err != nil {
		t.Fatal(err)
	}
}

// ======================================================================
// PROFILE COMMANDS
// ======================================================================

func TestCmdPROFILECREATE_Success(t *testing.T) {
	s := store.NewStore()
	ctx := discardCtx("PROFILE.CREATE", bytesArgs("alice"), s)
	if err := cmdPROFILECREATE(ctx); err != nil {
		t.Fatal(err)
	}
}

func TestCmdPROFILECREATE_TooFewArgs(t *testing.T) {
	s := store.NewStore()
	ctx := discardCtx("PROFILE.CREATE", bytesArgs(), s)
	if err := cmdPROFILECREATE(ctx); err != nil {
		t.Fatal(err)
	}
}

func TestCmdPROFILEDELETE_NotFound(t *testing.T) {
	s := store.NewStore()
	ctx := discardCtx("PROFILE.DELETE", bytesArgs("nonexistent"), s)
	if err := cmdPROFILEDELETE(ctx); err != nil {
		t.Fatal(err)
	}
}

func TestCmdPROFILEDELETE_TooFewArgs(t *testing.T) {
	s := store.NewStore()
	ctx := discardCtx("PROFILE.DELETE", bytesArgs(), s)
	if err := cmdPROFILEDELETE(ctx); err != nil {
		t.Fatal(err)
	}
}

func TestCmdPROFILEGET_NotFound(t *testing.T) {
	s := store.NewStore()
	ctx := discardCtx("PROFILE.GET", bytesArgs("nonexistent"), s)
	if err := cmdPROFILEGET(ctx); err != nil {
		t.Fatal(err)
	}
}

func TestCmdPROFILEGET_TooFewArgs(t *testing.T) {
	s := store.NewStore()
	ctx := discardCtx("PROFILE.GET", bytesArgs(), s)
	if err := cmdPROFILEGET(ctx); err != nil {
		t.Fatal(err)
	}
}

func TestCmdPROFILESET_NotFound(t *testing.T) {
	s := store.NewStore()
	ctx := discardCtx("PROFILE.SET", bytesArgs("nonexistent", "key", "val"), s)
	if err := cmdPROFILESET(ctx); err != nil {
		t.Fatal(err)
	}
}

func TestCmdPROFILESET_TooFewArgs(t *testing.T) {
	s := store.NewStore()
	ctx := discardCtx("PROFILE.SET", bytesArgs("id", "key"), s)
	if err := cmdPROFILESET(ctx); err != nil {
		t.Fatal(err)
	}
}

func TestCmdPROFILELIST(t *testing.T) {
	s := store.NewStore()
	ctx := discardCtx("PROFILE.LIST", bytesArgs(), s)
	if err := cmdPROFILELIST(ctx); err != nil {
		t.Fatal(err)
	}
}

// PROFILE lifecycle
func TestCmdPROFILE_Lifecycle(t *testing.T) {
	s := store.NewStore()
	ctx, buf := bufCtx("PROFILE.CREATE", bytesArgs("charlie"), s)
	if err := cmdPROFILECREATE(ctx); err != nil {
		t.Fatal(err)
	}
	id := extractBulkString(buf.String())
	if id == "" {
		t.Fatal("could not extract profile ID")
	}
	ctx = discardCtx("PROFILE.SET", bytesArgs(id, "bio", "hello"), s)
	if err := cmdPROFILESET(ctx); err != nil {
		t.Fatal(err)
	}
	ctx = discardCtx("PROFILE.GET", bytesArgs(id), s)
	if err := cmdPROFILEGET(ctx); err != nil {
		t.Fatal(err)
	}
	ctx = discardCtx("PROFILE.DELETE", bytesArgs(id), s)
	if err := cmdPROFILEDELETE(ctx); err != nil {
		t.Fatal(err)
	}
}

// ======================================================================
// ROLEX COMMANDS
// ======================================================================

func TestCmdROLEXCREATE_Success(t *testing.T) {
	s := store.NewStore()
	ctx := discardCtx("ROLEX.CREATE", bytesArgs("admin"), s)
	if err := cmdROLEXCREATE(ctx); err != nil {
		t.Fatal(err)
	}
}

func TestCmdROLEXCREATE_TooFewArgs(t *testing.T) {
	s := store.NewStore()
	ctx := discardCtx("ROLEX.CREATE", bytesArgs(), s)
	if err := cmdROLEXCREATE(ctx); err != nil {
		t.Fatal(err)
	}
}

func TestCmdROLEXDELETE_NotFound(t *testing.T) {
	s := store.NewStore()
	ctx := discardCtx("ROLEX.DELETE", bytesArgs("nonexistent"), s)
	if err := cmdROLEXDELETE(ctx); err != nil {
		t.Fatal(err)
	}
}

func TestCmdROLEXDELETE_TooFewArgs(t *testing.T) {
	s := store.NewStore()
	ctx := discardCtx("ROLEX.DELETE", bytesArgs(), s)
	if err := cmdROLEXDELETE(ctx); err != nil {
		t.Fatal(err)
	}
}

func TestCmdROLEXASSIGN_Success(t *testing.T) {
	s := store.NewStore()
	ctx := discardCtx("ROLEX.ASSIGN", bytesArgs("user1", "role1", "perm1"), s)
	if err := cmdROLEXASSIGN(ctx); err != nil {
		t.Fatal(err)
	}
}

func TestCmdROLEXASSIGN_TooFewArgs(t *testing.T) {
	s := store.NewStore()
	ctx := discardCtx("ROLEX.ASSIGN", bytesArgs("user1", "role1"), s)
	if err := cmdROLEXASSIGN(ctx); err != nil {
		t.Fatal(err)
	}
}

func TestCmdROLEXCHECK_Success(t *testing.T) {
	s := store.NewStore()
	ctx := discardCtx("ROLEX.CHECK", bytesArgs("user1", "role1", "perm1"), s)
	if err := cmdROLEXCHECK(ctx); err != nil {
		t.Fatal(err)
	}
}

func TestCmdROLEXCHECK_TooFewArgs(t *testing.T) {
	s := store.NewStore()
	ctx := discardCtx("ROLEX.CHECK", bytesArgs("user1", "role1"), s)
	if err := cmdROLEXCHECK(ctx); err != nil {
		t.Fatal(err)
	}
}

func TestCmdROLEXLIST(t *testing.T) {
	s := store.NewStore()
	ctx := discardCtx("ROLEX.LIST", bytesArgs(), s)
	if err := cmdROLEXLIST(ctx); err != nil {
		t.Fatal(err)
	}
}

// ROLEX lifecycle
func TestCmdROLEX_Lifecycle(t *testing.T) {
	s := store.NewStore()
	ctx, buf := bufCtx("ROLEX.CREATE", bytesArgs("editor"), s)
	if err := cmdROLEXCREATE(ctx); err != nil {
		t.Fatal(err)
	}
	id := extractBulkString(buf.String())
	if id == "" {
		t.Fatal("could not extract role ID")
	}
	ctx = discardCtx("ROLEX.DELETE", bytesArgs(id), s)
	if err := cmdROLEXDELETE(ctx); err != nil {
		t.Fatal(err)
	}
}

// ======================================================================
// RegisterAdvancedCommands2
// ======================================================================

func TestRegisterAdvancedCommands2(t *testing.T) {
	r := NewRouter()
	RegisterAdvancedCommands2(r)
	// Spot check a few commands were registered
	for _, name := range []string{
		"FILTER.CREATE", "TRANSFORM.APPLY", "ENRICH.LIST",
		"VALIDATE.CHECK", "JOBX.CREATE", "STAGE.NEXT",
		"CONTEXT.SET", "RULE.EVAL", "POLICY.CHECK",
		"PERMIT.GRANT", "GRANT.CREATE", "CHAINX.EXECUTE",
		"TASKX.RUN", "TIMER.STATUS", "COUNTERX2.INCR",
		"LEVEL.SET", "RECORD.ADD", "ENTITY.SET",
		"RELATION.CREATE", "CONNECTIONX.STATUS", "POOLX.ACQUIRE",
		"BUFFERX.WRITE", "STREAMX.READ", "EVENTX.EMIT",
		"HOOK.TRIGGER", "MIDDLEWARE.EXECUTE", "INTERCEPTOR.CHECK",
		"GUARD.CHECK", "PROXY.ROUTE", "CACHEX.SET",
		"STOREX.PUT", "INDEX.SEARCH", "QUERY.EXECUTE",
		"VIEW.GET", "REPORT.GENERATE", "AUDITX.LOG",
		"TOKEN.VALIDATE", "SESSIONX.SET", "PROFILE.SET",
		"ROLEX.ASSIGN",
	} {
		if _, ok := r.Get(name); !ok {
			t.Errorf("command %s not registered", name)
		}
	}
}
