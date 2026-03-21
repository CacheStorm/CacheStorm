package command

import (
	"fmt"
	"strings"
	"testing"

	"github.com/cachestorm/cachestorm/internal/store"
)

// ===========================================================================
// RESILIENCE COMMANDS
// ===========================================================================

func TestCmdCIRCUITXCREATE_Success(t *testing.T) {
	s := store.NewStore()
	ctx := discardCtx("CIRCUITX.CREATE", bytesArgs("mycirc", "5", "1000"), s)
	if err := cmdCIRCUITXCREATE(ctx); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCmdCIRCUITXCREATE_WrongArgs(t *testing.T) {
	s := store.NewStore()
	ctx := discardCtx("CIRCUITX.CREATE", bytesArgs("mycirc"), s)
	if err := cmdCIRCUITXCREATE(ctx); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCmdCIRCUITXOPEN_Success(t *testing.T) {
	s := store.NewStore()
	ctx := discardCtx("CIRCUITX.CREATE", bytesArgs("oc", "5", "1000"), s)
	cmdCIRCUITXCREATE(ctx)
	ctx2 := discardCtx("CIRCUITX.OPEN", bytesArgs("oc"), s)
	if err := cmdCIRCUITXOPEN(ctx2); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCmdCIRCUITXOPEN_WrongArgs(t *testing.T) {
	s := store.NewStore()
	ctx := discardCtx("CIRCUITX.OPEN", bytesArgs(), s)
	if err := cmdCIRCUITXOPEN(ctx); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCmdCIRCUITXCLOSE_Success(t *testing.T) {
	s := store.NewStore()
	cmdCIRCUITXCREATE(discardCtx("", bytesArgs("cc", "5", "1000"), s))
	if err := cmdCIRCUITXCLOSE(discardCtx("", bytesArgs("cc"), s)); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCmdCIRCUITXCLOSE_WrongArgs(t *testing.T) {
	s := store.NewStore()
	if err := cmdCIRCUITXCLOSE(discardCtx("", bytesArgs(), s)); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCmdCIRCUITXHALFOPEN_Success(t *testing.T) {
	s := store.NewStore()
	cmdCIRCUITXCREATE(discardCtx("", bytesArgs("hoc", "5", "1000"), s))
	if err := cmdCIRCUITXHALFOPEN(discardCtx("", bytesArgs("hoc"), s)); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCmdCIRCUITXHALFOPEN_WrongArgs(t *testing.T) {
	s := store.NewStore()
	if err := cmdCIRCUITXHALFOPEN(discardCtx("", bytesArgs(), s)); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCmdCIRCUITXSTATUS_Success(t *testing.T) {
	s := store.NewStore()
	cmdCIRCUITXCREATE(discardCtx("", bytesArgs("sc", "5", "1000"), s))
	if err := cmdCIRCUITXSTATUS(discardCtx("", bytesArgs("sc"), s)); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCmdCIRCUITXSTATUS_WrongArgs(t *testing.T) {
	s := store.NewStore()
	if err := cmdCIRCUITXSTATUS(discardCtx("", bytesArgs(), s)); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCmdCIRCUITXMETRICS_Success(t *testing.T) {
	s := store.NewStore()
	cmdCIRCUITXCREATE(discardCtx("", bytesArgs("mc", "5", "1000"), s))
	if err := cmdCIRCUITXMETRICS(discardCtx("", bytesArgs("mc"), s)); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCmdCIRCUITXMETRICS_WrongArgs(t *testing.T) {
	s := store.NewStore()
	if err := cmdCIRCUITXMETRICS(discardCtx("", bytesArgs(), s)); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCmdCIRCUITXRESET_Success(t *testing.T) {
	s := store.NewStore()
	cmdCIRCUITXCREATE(discardCtx("", bytesArgs("rc", "5", "1000"), s))
	if err := cmdCIRCUITXRESET(discardCtx("", bytesArgs("rc"), s)); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCmdCIRCUITXRESET_WrongArgs(t *testing.T) {
	s := store.NewStore()
	if err := cmdCIRCUITXRESET(discardCtx("", bytesArgs(), s)); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCmdCIRCUITXDELETE_Success(t *testing.T) {
	s := store.NewStore()
	cmdCIRCUITXCREATE(discardCtx("", bytesArgs("dc", "5", "1000"), s))
	if err := cmdCIRCUITXDELETE(discardCtx("", bytesArgs("dc"), s)); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCmdCIRCUITXDELETE_WrongArgs(t *testing.T) {
	s := store.NewStore()
	if err := cmdCIRCUITXDELETE(discardCtx("", bytesArgs(), s)); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCmdRATELIMITERCREATE_Success(t *testing.T) {
	s := store.NewStore()
	if err := cmdRATELIMITERCREATE(discardCtx("", bytesArgs("rl1", "10", "1000"), s)); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCmdRATELIMITERCREATE_WrongArgs(t *testing.T) {
	s := store.NewStore()
	if err := cmdRATELIMITERCREATE(discardCtx("", bytesArgs("rl1"), s)); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCmdRATELIMITERTRY_Success(t *testing.T) {
	s := store.NewStore()
	cmdRATELIMITERCREATE(discardCtx("", bytesArgs("rlt", "10", "60000"), s))
	if err := cmdRATELIMITERTRY(discardCtx("", bytesArgs("rlt"), s)); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCmdRATELIMITERTRY_WrongArgs(t *testing.T) {
	s := store.NewStore()
	if err := cmdRATELIMITERTRY(discardCtx("", bytesArgs(), s)); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCmdRATELIMITERWAIT_Success(t *testing.T) {
	s := store.NewStore()
	cmdRATELIMITERCREATE(discardCtx("", bytesArgs("rlw", "10", "60000"), s))
	if err := cmdRATELIMITERWAIT(discardCtx("", bytesArgs("rlw"), s)); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCmdRATELIMITERWAIT_WrongArgs(t *testing.T) {
	s := store.NewStore()
	if err := cmdRATELIMITERWAIT(discardCtx("", bytesArgs(), s)); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCmdRATELIMITERRESET_Success(t *testing.T) {
	s := store.NewStore()
	cmdRATELIMITERCREATE(discardCtx("", bytesArgs("rlr", "10", "60000"), s))
	if err := cmdRATELIMITERRESET(discardCtx("", bytesArgs("rlr"), s)); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCmdRATELIMITERRESET_WrongArgs(t *testing.T) {
	s := store.NewStore()
	if err := cmdRATELIMITERRESET(discardCtx("", bytesArgs(), s)); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCmdRATELIMITERSTATUS_Success(t *testing.T) {
	s := store.NewStore()
	cmdRATELIMITERCREATE(discardCtx("", bytesArgs("rls", "10", "60000"), s))
	if err := cmdRATELIMITERSTATUS(discardCtx("", bytesArgs("rls"), s)); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCmdRATELIMITERSTATUS_WrongArgs(t *testing.T) {
	s := store.NewStore()
	if err := cmdRATELIMITERSTATUS(discardCtx("", bytesArgs(), s)); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCmdRATELIMITERDELETE_Success(t *testing.T) {
	s := store.NewStore()
	cmdRATELIMITERCREATE(discardCtx("", bytesArgs("rld", "10", "60000"), s))
	if err := cmdRATELIMITERDELETE(discardCtx("", bytesArgs("rld"), s)); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCmdRATELIMITERDELETE_WrongArgs(t *testing.T) {
	s := store.NewStore()
	if err := cmdRATELIMITERDELETE(discardCtx("", bytesArgs(), s)); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCmdRETRYCREATE_Success(t *testing.T) {
	s := store.NewStore()
	if err := cmdRETRYCREATE(discardCtx("", bytesArgs("ret1", "3", "100"), s)); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCmdRETRYCREATE_WrongArgs(t *testing.T) {
	s := store.NewStore()
	if err := cmdRETRYCREATE(discardCtx("", bytesArgs("ret1"), s)); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCmdRETRYEXECUTE_Success(t *testing.T) {
	s := store.NewStore()
	cmdRETRYCREATE(discardCtx("", bytesArgs("retex", "3", "100"), s))
	if err := cmdRETRYEXECUTE(discardCtx("", bytesArgs("retex", "cmd"), s)); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCmdRETRYEXECUTE_WrongArgs(t *testing.T) {
	s := store.NewStore()
	if err := cmdRETRYEXECUTE(discardCtx("", bytesArgs("x"), s)); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCmdRETRYSTATUS_Success(t *testing.T) {
	s := store.NewStore()
	cmdRETRYCREATE(discardCtx("", bytesArgs("retst", "3", "100"), s))
	if err := cmdRETRYSTATUS(discardCtx("", bytesArgs("retst"), s)); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCmdRETRYSTATUS_WrongArgs(t *testing.T) {
	s := store.NewStore()
	if err := cmdRETRYSTATUS(discardCtx("", bytesArgs(), s)); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCmdRETRYDELETE_Success(t *testing.T) {
	s := store.NewStore()
	cmdRETRYCREATE(discardCtx("", bytesArgs("retdel", "3", "100"), s))
	if err := cmdRETRYDELETE(discardCtx("", bytesArgs("retdel"), s)); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCmdRETRYDELETE_WrongArgs(t *testing.T) {
	s := store.NewStore()
	if err := cmdRETRYDELETE(discardCtx("", bytesArgs(), s)); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCmdTIMEOUTCREATE_Success(t *testing.T) {
	s := store.NewStore()
	if err := cmdTIMEOUTCREATE(discardCtx("", bytesArgs("to1", "5000"), s)); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCmdTIMEOUTCREATE_WrongArgs(t *testing.T) {
	s := store.NewStore()
	if err := cmdTIMEOUTCREATE(discardCtx("", bytesArgs("to1"), s)); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCmdTIMEOUTEXECUTE_Success(t *testing.T) {
	s := store.NewStore()
	cmdTIMEOUTCREATE(discardCtx("", bytesArgs("toex", "999999"), s))
	if err := cmdTIMEOUTEXECUTE(discardCtx("", bytesArgs("toex", "cmd"), s)); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCmdTIMEOUTEXECUTE_WrongArgs(t *testing.T) {
	s := store.NewStore()
	if err := cmdTIMEOUTEXECUTE(discardCtx("", bytesArgs("x"), s)); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCmdTIMEOUTDELETE_Success(t *testing.T) {
	s := store.NewStore()
	cmdTIMEOUTCREATE(discardCtx("", bytesArgs("todel", "5000"), s))
	if err := cmdTIMEOUTDELETE(discardCtx("", bytesArgs("todel"), s)); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCmdTIMEOUTDELETE_WrongArgs(t *testing.T) {
	s := store.NewStore()
	if err := cmdTIMEOUTDELETE(discardCtx("", bytesArgs(), s)); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCmdBULKHEADCREATE_Success(t *testing.T) {
	s := store.NewStore()
	if err := cmdBULKHEADCREATE(discardCtx("", bytesArgs("bh1", "5"), s)); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCmdBULKHEADCREATE_WrongArgs(t *testing.T) {
	s := store.NewStore()
	if err := cmdBULKHEADCREATE(discardCtx("", bytesArgs("bh1"), s)); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCmdBULKHEADACQUIRE_Success(t *testing.T) {
	s := store.NewStore()
	cmdBULKHEADCREATE(discardCtx("", bytesArgs("bhacq", "5"), s))
	if err := cmdBULKHEADACQUIRE(discardCtx("", bytesArgs("bhacq"), s)); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCmdBULKHEADACQUIRE_WrongArgs(t *testing.T) {
	s := store.NewStore()
	if err := cmdBULKHEADACQUIRE(discardCtx("", bytesArgs(), s)); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCmdBULKHEADRELEASE_Success(t *testing.T) {
	s := store.NewStore()
	cmdBULKHEADCREATE(discardCtx("", bytesArgs("bhrel", "5"), s))
	cmdBULKHEADACQUIRE(discardCtx("", bytesArgs("bhrel"), s))
	if err := cmdBULKHEADRELEASE(discardCtx("", bytesArgs("bhrel"), s)); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCmdBULKHEADRELEASE_WrongArgs(t *testing.T) {
	s := store.NewStore()
	if err := cmdBULKHEADRELEASE(discardCtx("", bytesArgs(), s)); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCmdBULKHEADSTATUS_Success(t *testing.T) {
	s := store.NewStore()
	cmdBULKHEADCREATE(discardCtx("", bytesArgs("bhst", "5"), s))
	if err := cmdBULKHEADSTATUS(discardCtx("", bytesArgs("bhst"), s)); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCmdBULKHEADSTATUS_WrongArgs(t *testing.T) {
	s := store.NewStore()
	if err := cmdBULKHEADSTATUS(discardCtx("", bytesArgs(), s)); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCmdBULKHEADDELETE_Success(t *testing.T) {
	s := store.NewStore()
	cmdBULKHEADCREATE(discardCtx("", bytesArgs("bhdel", "5"), s))
	if err := cmdBULKHEADDELETE(discardCtx("", bytesArgs("bhdel"), s)); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCmdBULKHEADDELETE_WrongArgs(t *testing.T) {
	s := store.NewStore()
	if err := cmdBULKHEADDELETE(discardCtx("", bytesArgs(), s)); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCmdFALLBACKCREATE_Success(t *testing.T) {
	s := store.NewStore()
	if err := cmdFALLBACKCREATE(discardCtx("", bytesArgs("fb1", "default_action"), s)); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCmdFALLBACKCREATE_WrongArgs(t *testing.T) {
	s := store.NewStore()
	if err := cmdFALLBACKCREATE(discardCtx("", bytesArgs("fb1"), s)); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCmdFALLBACKEXECUTE_Success(t *testing.T) {
	s := store.NewStore()
	cmdFALLBACKCREATE(discardCtx("", bytesArgs("fbex", "fallback_val"), s))
	if err := cmdFALLBACKEXECUTE(discardCtx("", bytesArgs("fbex"), s)); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCmdFALLBACKEXECUTE_WrongArgs(t *testing.T) {
	s := store.NewStore()
	if err := cmdFALLBACKEXECUTE(discardCtx("", bytesArgs(), s)); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCmdFALLBACKDELETE_Success(t *testing.T) {
	s := store.NewStore()
	cmdFALLBACKCREATE(discardCtx("", bytesArgs("fbdel", "act"), s))
	if err := cmdFALLBACKDELETE(discardCtx("", bytesArgs("fbdel"), s)); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCmdFALLBACKDELETE_WrongArgs(t *testing.T) {
	s := store.NewStore()
	if err := cmdFALLBACKDELETE(discardCtx("", bytesArgs(), s)); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCmdOBSERVABILITYTRACE_Success(t *testing.T) {
	s := store.NewStore()
	if err := cmdOBSERVABILITYTRACE(discardCtx("", bytesArgs("trace1", "data"), s)); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCmdOBSERVABILITYTRACE_WrongArgs(t *testing.T) {
	s := store.NewStore()
	if err := cmdOBSERVABILITYTRACE(discardCtx("", bytesArgs("x"), s)); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCmdOBSERVABILITYMETRIC_Success(t *testing.T) {
	s := store.NewStore()
	if err := cmdOBSERVABILITYMETRIC(discardCtx("", bytesArgs("met", "counter", "1"), s)); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCmdOBSERVABILITYMETRIC_WrongArgs(t *testing.T) {
	s := store.NewStore()
	if err := cmdOBSERVABILITYMETRIC(discardCtx("", bytesArgs("met"), s)); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCmdOBSERVABILITYLOG_Success(t *testing.T) {
	s := store.NewStore()
	if err := cmdOBSERVABILITYLOG(discardCtx("", bytesArgs("INFO", "message"), s)); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCmdOBSERVABILITYLOG_WrongArgs(t *testing.T) {
	s := store.NewStore()
	if err := cmdOBSERVABILITYLOG(discardCtx("", bytesArgs("x"), s)); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCmdOBSERVABILITYSPAN_Success(t *testing.T) {
	s := store.NewStore()
	if err := cmdOBSERVABILITYSPAN(discardCtx("", bytesArgs("span1", "data"), s)); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCmdOBSERVABILITYSPAN_WrongArgs(t *testing.T) {
	s := store.NewStore()
	if err := cmdOBSERVABILITYSPAN(discardCtx("", bytesArgs("x"), s)); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCmdTELEMETRYRECORD_Success(t *testing.T) {
	s := store.NewStore()
	if err := cmdTELEMETRYRECORD(discardCtx("", bytesArgs("metric1", "1000", "3.14"), s)); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCmdTELEMETRYRECORD_WrongArgs(t *testing.T) {
	s := store.NewStore()
	if err := cmdTELEMETRYRECORD(discardCtx("", bytesArgs("x", "y"), s)); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCmdTELEMETRYQUERY_Success(t *testing.T) {
	s := store.NewStore()
	cmdTELEMETRYRECORD(discardCtx("", bytesArgs("tq_metric", "1000", "3.14"), s))
	if err := cmdTELEMETRYQUERY(discardCtx("", bytesArgs("tq_metric"), s)); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCmdTELEMETRYQUERY_WrongArgs(t *testing.T) {
	s := store.NewStore()
	if err := cmdTELEMETRYQUERY(discardCtx("", bytesArgs(), s)); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCmdTELEMETRYEXPORT_Success(t *testing.T) {
	s := store.NewStore()
	if err := cmdTELEMETRYEXPORT(discardCtx("", bytesArgs("json"), s)); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCmdTELEMETRYEXPORT_WrongArgs(t *testing.T) {
	s := store.NewStore()
	if err := cmdTELEMETRYEXPORT(discardCtx("", bytesArgs(), s)); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCmdDIAGNOSTICRUN_Success(t *testing.T) {
	s := store.NewStore()
	if err := cmdDIAGNOSTICRUN(discardCtx("", bytesArgs("check1", "health"), s)); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCmdDIAGNOSTICRUN_WrongArgs(t *testing.T) {
	s := store.NewStore()
	if err := cmdDIAGNOSTICRUN(discardCtx("", bytesArgs("x"), s)); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCmdDIAGNOSTICLIST_Success(t *testing.T) {
	s := store.NewStore()
	if err := cmdDIAGNOSTICLIST(discardCtx("", bytesArgs(), s)); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCmdPROFILESTART_Success(t *testing.T) {
	s := store.NewStore()
	if err := cmdPROFILESTART(discardCtx("", bytesArgs("prof1"), s)); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCmdPROFILESTART_WrongArgs(t *testing.T) {
	s := store.NewStore()
	if err := cmdPROFILESTART(discardCtx("", bytesArgs(), s)); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCmdPROFILEXLIST_Success(t *testing.T) {
	s := store.NewStore()
	if err := cmdPROFILEXLIST(discardCtx("", bytesArgs(), s)); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCmdHEAPSTATS_Success(t *testing.T) {
	s := store.NewStore()
	if err := cmdHEAPSTATS(discardCtx("", bytesArgs(), s)); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCmdHEAPDUMP_Success(t *testing.T) {
	s := store.NewStore()
	if err := cmdHEAPDUMP(discardCtx("", bytesArgs(), s)); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCmdHEAPGC_Success(t *testing.T) {
	s := store.NewStore()
	if err := cmdHEAPGC(discardCtx("", bytesArgs(), s)); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCmdMEMORYXALLOC_Success(t *testing.T) {
	s := store.NewStore()
	if err := cmdMEMORYXALLOC(discardCtx("", bytesArgs("pool1", "1024"), s)); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCmdMEMORYXALLOC_WrongArgs(t *testing.T) {
	s := store.NewStore()
	if err := cmdMEMORYXALLOC(discardCtx("", bytesArgs("x"), s)); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCmdMEMORYXFREE_Success(t *testing.T) {
	s := store.NewStore()
	cmdMEMORYXALLOC(discardCtx("", bytesArgs("mfree", "1024"), s))
	if err := cmdMEMORYXFREE(discardCtx("", bytesArgs("mfree", "512"), s)); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCmdMEMORYXFREE_WrongArgs(t *testing.T) {
	s := store.NewStore()
	if err := cmdMEMORYXFREE(discardCtx("", bytesArgs("x"), s)); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCmdMEMORYXSTATS_Success(t *testing.T) {
	s := store.NewStore()
	cmdMEMORYXALLOC(discardCtx("", bytesArgs("mstat", "1024"), s))
	if err := cmdMEMORYXSTATS(discardCtx("", bytesArgs("mstat"), s)); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCmdMEMORYXSTATS_WrongArgs(t *testing.T) {
	s := store.NewStore()
	if err := cmdMEMORYXSTATS(discardCtx("", bytesArgs(), s)); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCmdMEMORYXTRACK_Success(t *testing.T) {
	s := store.NewStore()
	if err := cmdMEMORYXTRACK(discardCtx("", bytesArgs("tracker", "alloc"), s)); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCmdMEMORYXTRACK_WrongArgs(t *testing.T) {
	s := store.NewStore()
	if err := cmdMEMORYXTRACK(discardCtx("", bytesArgs("x"), s)); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCmdCONPOOLCREATE_Success(t *testing.T) {
	s := store.NewStore()
	if err := cmdCONPOOLCREATE(discardCtx("", bytesArgs("cp1", "5"), s)); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCmdCONPOOLCREATE_WrongArgs(t *testing.T) {
	s := store.NewStore()
	if err := cmdCONPOOLCREATE(discardCtx("", bytesArgs("x"), s)); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCmdCONPOOLGET_Success(t *testing.T) {
	s := store.NewStore()
	cmdCONPOOLCREATE(discardCtx("", bytesArgs("cpg", "3"), s))
	if err := cmdCONPOOLGET(discardCtx("", bytesArgs("cpg"), s)); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCmdCONPOOLGET_WrongArgs(t *testing.T) {
	s := store.NewStore()
	if err := cmdCONPOOLGET(discardCtx("", bytesArgs(), s)); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCmdCONPOOLRETURN_Success(t *testing.T) {
	s := store.NewStore()
	cmdCONPOOLCREATE(discardCtx("", bytesArgs("cpr", "3"), s))
	if err := cmdCONPOOLRETURN(discardCtx("", bytesArgs("cpr", "conn-0"), s)); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCmdCONPOOLRETURN_WrongArgs(t *testing.T) {
	s := store.NewStore()
	if err := cmdCONPOOLRETURN(discardCtx("", bytesArgs("x"), s)); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCmdCONPOOLSTATUS_Success(t *testing.T) {
	s := store.NewStore()
	cmdCONPOOLCREATE(discardCtx("", bytesArgs("cps", "3"), s))
	if err := cmdCONPOOLSTATUS(discardCtx("", bytesArgs("cps"), s)); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCmdCONPOOLSTATUS_WrongArgs(t *testing.T) {
	s := store.NewStore()
	if err := cmdCONPOOLSTATUS(discardCtx("", bytesArgs(), s)); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCmdCONPOOLDELETE_Success(t *testing.T) {
	s := store.NewStore()
	cmdCONPOOLCREATE(discardCtx("", bytesArgs("cpd", "3"), s))
	if err := cmdCONPOOLDELETE(discardCtx("", bytesArgs("cpd"), s)); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCmdCONPOOLDELETE_WrongArgs(t *testing.T) {
	s := store.NewStore()
	if err := cmdCONPOOLDELETE(discardCtx("", bytesArgs(), s)); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCmdBATCHXCREATE_Success(t *testing.T) {
	s := store.NewStore()
	if err := cmdBATCHXCREATE(discardCtx("", bytesArgs("batch1"), s)); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCmdBATCHXCREATE_WrongArgs(t *testing.T) {
	s := store.NewStore()
	if err := cmdBATCHXCREATE(discardCtx("", bytesArgs(), s)); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCmdPIPELINEXSTART_Success(t *testing.T) {
	s := store.NewStore()
	if err := cmdPIPELINEXSTART(discardCtx("", bytesArgs(), s)); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCmdTRANSXBEGIN_Success(t *testing.T) {
	s := store.NewStore()
	if err := cmdTRANSXBEGIN(discardCtx("", bytesArgs(), s)); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCmdLOCKXACQUIRE_Success(t *testing.T) {
	s := store.NewStore()
	if err := cmdLOCKXACQUIRE(discardCtx("", bytesArgs("key1", "holder1", "5000"), s)); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCmdLOCKXACQUIRE_WrongArgs(t *testing.T) {
	s := store.NewStore()
	if err := cmdLOCKXACQUIRE(discardCtx("", bytesArgs("x", "y"), s)); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCmdLOCKXRELEASE_Success(t *testing.T) {
	s := store.NewStore()
	cmdLOCKXACQUIRE(discardCtx("", bytesArgs("lrel", "holder1", "5000"), s))
	if err := cmdLOCKXRELEASE(discardCtx("", bytesArgs("lrel", "holder1"), s)); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCmdLOCKXRELEASE_WrongArgs(t *testing.T) {
	s := store.NewStore()
	if err := cmdLOCKXRELEASE(discardCtx("", bytesArgs("x"), s)); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCmdLOCKXEXTEND_Success(t *testing.T) {
	s := store.NewStore()
	cmdLOCKXACQUIRE(discardCtx("", bytesArgs("lext", "holder1", "5000"), s))
	if err := cmdLOCKXEXTEND(discardCtx("", bytesArgs("lext", "holder1", "10000"), s)); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCmdLOCKXEXTEND_WrongArgs(t *testing.T) {
	s := store.NewStore()
	if err := cmdLOCKXEXTEND(discardCtx("", bytesArgs("x", "y"), s)); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCmdLOCKXSTATUS_Success(t *testing.T) {
	s := store.NewStore()
	cmdLOCKXACQUIRE(discardCtx("", bytesArgs("lst", "holder1", "5000"), s))
	if err := cmdLOCKXSTATUS(discardCtx("", bytesArgs("lst"), s)); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCmdLOCKXSTATUS_WrongArgs(t *testing.T) {
	s := store.NewStore()
	if err := cmdLOCKXSTATUS(discardCtx("", bytesArgs(), s)); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCmdSEMAPHOREXCREATE_Success(t *testing.T) {
	s := store.NewStore()
	if err := cmdSEMAPHOREXCREATE(discardCtx("", bytesArgs("sem1", "5"), s)); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCmdSEMAPHOREXCREATE_WrongArgs(t *testing.T) {
	s := store.NewStore()
	if err := cmdSEMAPHOREXCREATE(discardCtx("", bytesArgs("x"), s)); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCmdSEMAPHOREXACQUIRE_Success(t *testing.T) {
	s := store.NewStore()
	cmdSEMAPHOREXCREATE(discardCtx("", bytesArgs("semacq", "5"), s))
	if err := cmdSEMAPHOREXACQUIRE(discardCtx("", bytesArgs("semacq"), s)); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCmdSEMAPHOREXACQUIRE_WrongArgs(t *testing.T) {
	s := store.NewStore()
	if err := cmdSEMAPHOREXACQUIRE(discardCtx("", bytesArgs(), s)); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCmdSEMAPHOREXRELEASE_Success(t *testing.T) {
	s := store.NewStore()
	cmdSEMAPHOREXCREATE(discardCtx("", bytesArgs("semrel", "5"), s))
	cmdSEMAPHOREXACQUIRE(discardCtx("", bytesArgs("semrel"), s))
	if err := cmdSEMAPHOREXRELEASE(discardCtx("", bytesArgs("semrel"), s)); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCmdSEMAPHOREXRELEASE_WrongArgs(t *testing.T) {
	s := store.NewStore()
	if err := cmdSEMAPHOREXRELEASE(discardCtx("", bytesArgs(), s)); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCmdSEMAPHOREXSTATUS_Success(t *testing.T) {
	s := store.NewStore()
	cmdSEMAPHOREXCREATE(discardCtx("", bytesArgs("semst", "5"), s))
	if err := cmdSEMAPHOREXSTATUS(discardCtx("", bytesArgs("semst"), s)); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCmdSEMAPHOREXSTATUS_WrongArgs(t *testing.T) {
	s := store.NewStore()
	if err := cmdSEMAPHOREXSTATUS(discardCtx("", bytesArgs(), s)); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCmdASYNCSUBMIT_Success(t *testing.T) {
	s := store.NewStore()
	if err := cmdASYNCSUBMIT(discardCtx("", bytesArgs("job1"), s)); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCmdASYNCSUBMIT_WrongArgs(t *testing.T) {
	s := store.NewStore()
	if err := cmdASYNCSUBMIT(discardCtx("", bytesArgs(), s)); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCmdPROMISECREATE_Success(t *testing.T) {
	s := store.NewStore()
	if err := cmdPROMISECREATE(discardCtx("", bytesArgs(), s)); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCmdFUTURECREATE_Success(t *testing.T) {
	s := store.NewStore()
	if err := cmdFUTURECREATE(discardCtx("", bytesArgs(), s)); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCmdOBSERVABLECREATE_Success(t *testing.T) {
	s := store.NewStore()
	if err := cmdOBSERVABLECREATE(discardCtx("", bytesArgs(), s)); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCmdSTREAMPROCCREATE_Success(t *testing.T) {
	s := store.NewStore()
	if err := cmdSTREAMPROCCREATE(discardCtx("", bytesArgs("stream1"), s)); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCmdSTREAMPROCCREATE_WrongArgs(t *testing.T) {
	s := store.NewStore()
	if err := cmdSTREAMPROCCREATE(discardCtx("", bytesArgs(), s)); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCmdSTREAMPROCPUSH_Success(t *testing.T) {
	s := store.NewStore()
	cmdSTREAMPROCCREATE(discardCtx("", bytesArgs("sp_push"), s))
	if err := cmdSTREAMPROCPUSH(discardCtx("", bytesArgs("sp_push", "val1"), s)); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCmdSTREAMPROCPUSH_WrongArgs(t *testing.T) {
	s := store.NewStore()
	if err := cmdSTREAMPROCPUSH(discardCtx("", bytesArgs("x"), s)); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCmdSTREAMPROCPOP_Success(t *testing.T) {
	s := store.NewStore()
	cmdSTREAMPROCCREATE(discardCtx("", bytesArgs("sp_pop"), s))
	cmdSTREAMPROCPUSH(discardCtx("", bytesArgs("sp_pop", "val1"), s))
	if err := cmdSTREAMPROCPOP(discardCtx("", bytesArgs("sp_pop"), s)); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCmdSTREAMPROCPOP_WrongArgs(t *testing.T) {
	s := store.NewStore()
	if err := cmdSTREAMPROCPOP(discardCtx("", bytesArgs(), s)); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCmdSTREAMPROCPEEK_Success(t *testing.T) {
	s := store.NewStore()
	cmdSTREAMPROCCREATE(discardCtx("", bytesArgs("sp_peek"), s))
	cmdSTREAMPROCPUSH(discardCtx("", bytesArgs("sp_peek", "val1"), s))
	if err := cmdSTREAMPROCPEEK(discardCtx("", bytesArgs("sp_peek"), s)); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCmdSTREAMPROCPEEK_WrongArgs(t *testing.T) {
	s := store.NewStore()
	if err := cmdSTREAMPROCPEEK(discardCtx("", bytesArgs(), s)); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCmdSTREAMPROCDELETE_Success(t *testing.T) {
	s := store.NewStore()
	cmdSTREAMPROCCREATE(discardCtx("", bytesArgs("sp_del"), s))
	if err := cmdSTREAMPROCDELETE(discardCtx("", bytesArgs("sp_del"), s)); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCmdSTREAMPROCDELETE_WrongArgs(t *testing.T) {
	s := store.NewStore()
	if err := cmdSTREAMPROCDELETE(discardCtx("", bytesArgs(), s)); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCmdEVENTSOURCINGAPPEND_Success(t *testing.T) {
	s := store.NewStore()
	if err := cmdEVENTSOURCINGAPPEND(discardCtx("", bytesArgs("es_stream", "created", "data1"), s)); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCmdEVENTSOURCINGAPPEND_WrongArgs(t *testing.T) {
	s := store.NewStore()
	if err := cmdEVENTSOURCINGAPPEND(discardCtx("", bytesArgs("x", "y"), s)); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCmdEVENTSOURCINGREPLAY_Success(t *testing.T) {
	s := store.NewStore()
	cmdEVENTSOURCINGAPPEND(discardCtx("", bytesArgs("es_replay", "created", "data1"), s))
	if err := cmdEVENTSOURCINGREPLAY(discardCtx("", bytesArgs("es_replay"), s)); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCmdEVENTSOURCINGREPLAY_WrongArgs(t *testing.T) {
	s := store.NewStore()
	if err := cmdEVENTSOURCINGREPLAY(discardCtx("", bytesArgs(), s)); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCmdEVENTSOURCINGSNAPSHOT_Success(t *testing.T) {
	s := store.NewStore()
	if err := cmdEVENTSOURCINGSNAPSHOT(discardCtx("", bytesArgs("stream1", "snap1"), s)); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCmdEVENTSOURCINGSNAPSHOT_WrongArgs(t *testing.T) {
	s := store.NewStore()
	if err := cmdEVENTSOURCINGSNAPSHOT(discardCtx("", bytesArgs("x"), s)); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCmdEVENTSOURCINGGET_Success(t *testing.T) {
	s := store.NewStore()
	cmdEVENTSOURCINGAPPEND(discardCtx("", bytesArgs("es_get", "created", "data1"), s))
	if err := cmdEVENTSOURCINGGET(discardCtx("", bytesArgs("es_get", "1"), s)); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCmdEVENTSOURCINGGET_WrongArgs(t *testing.T) {
	s := store.NewStore()
	if err := cmdEVENTSOURCINGGET(discardCtx("", bytesArgs("x"), s)); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCmdCOMPACTMERGE_Success(t *testing.T) {
	s := store.NewStore()
	if err := cmdCOMPACTMERGE(discardCtx("", bytesArgs("ns1"), s)); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCmdCOMPACTMERGE_WrongArgs(t *testing.T) {
	s := store.NewStore()
	if err := cmdCOMPACTMERGE(discardCtx("", bytesArgs(), s)); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCmdCOMPACTSTATUS_Success(t *testing.T) {
	s := store.NewStore()
	if err := cmdCOMPACTSTATUS(discardCtx("", bytesArgs("ns1"), s)); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCmdCOMPACTSTATUS_WrongArgs(t *testing.T) {
	s := store.NewStore()
	if err := cmdCOMPACTSTATUS(discardCtx("", bytesArgs(), s)); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCmdBACKPRESSURECREATE_Success(t *testing.T) {
	s := store.NewStore()
	if err := cmdBACKPRESSURECREATE(discardCtx("", bytesArgs("bp1", "100", "20"), s)); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCmdBACKPRESSURECREATE_WrongArgs(t *testing.T) {
	s := store.NewStore()
	if err := cmdBACKPRESSURECREATE(discardCtx("", bytesArgs("x", "y"), s)); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCmdBACKPRESSURECHECK_Success(t *testing.T) {
	s := store.NewStore()
	cmdBACKPRESSURECREATE(discardCtx("", bytesArgs("bpc", "100", "20"), s))
	if err := cmdBACKPRESSURECHECK(discardCtx("", bytesArgs("bpc", "10"), s)); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCmdBACKPRESSURECHECK_WrongArgs(t *testing.T) {
	s := store.NewStore()
	if err := cmdBACKPRESSURECHECK(discardCtx("", bytesArgs("x"), s)); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCmdBACKPRESSURESTATUS_Success(t *testing.T) {
	s := store.NewStore()
	cmdBACKPRESSURECREATE(discardCtx("", bytesArgs("bps", "100", "20"), s))
	if err := cmdBACKPRESSURESTATUS(discardCtx("", bytesArgs("bps"), s)); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCmdBACKPRESSURESTATUS_WrongArgs(t *testing.T) {
	s := store.NewStore()
	if err := cmdBACKPRESSURESTATUS(discardCtx("", bytesArgs(), s)); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCmdTHROTTLEXCREATE_Success(t *testing.T) {
	s := store.NewStore()
	if err := cmdTHROTTLEXCREATE(discardCtx("", bytesArgs("thr1", "100"), s)); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCmdTHROTTLEXCREATE_WrongArgs(t *testing.T) {
	s := store.NewStore()
	if err := cmdTHROTTLEXCREATE(discardCtx("", bytesArgs("x"), s)); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCmdTHROTTLEXCHECK_Success(t *testing.T) {
	s := store.NewStore()
	cmdTHROTTLEXCREATE(discardCtx("", bytesArgs("thrc", "100"), s))
	if err := cmdTHROTTLEXCHECK(discardCtx("", bytesArgs("thrc"), s)); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCmdTHROTTLEXCHECK_WrongArgs(t *testing.T) {
	s := store.NewStore()
	if err := cmdTHROTTLEXCHECK(discardCtx("", bytesArgs(), s)); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCmdTHROTTLEXSTATUS_Success(t *testing.T) {
	s := store.NewStore()
	cmdTHROTTLEXCREATE(discardCtx("", bytesArgs("thrs", "100"), s))
	if err := cmdTHROTTLEXSTATUS(discardCtx("", bytesArgs("thrs"), s)); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCmdTHROTTLEXSTATUS_WrongArgs(t *testing.T) {
	s := store.NewStore()
	if err := cmdTHROTTLEXSTATUS(discardCtx("", bytesArgs(), s)); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCmdDEBOUNCEXCREATE_Success(t *testing.T) {
	s := store.NewStore()
	if err := cmdDEBOUNCEXCREATE(discardCtx("", bytesArgs("deb1", "500"), s)); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCmdDEBOUNCEXCREATE_WrongArgs(t *testing.T) {
	s := store.NewStore()
	if err := cmdDEBOUNCEXCREATE(discardCtx("", bytesArgs("x"), s)); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCmdDEBOUNCEXCALL_Success(t *testing.T) {
	s := store.NewStore()
	cmdDEBOUNCEXCREATE(discardCtx("", bytesArgs("debc", "500"), s))
	if err := cmdDEBOUNCEXCALL(discardCtx("", bytesArgs("debc", "arg1"), s)); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCmdDEBOUNCEXCALL_WrongArgs(t *testing.T) {
	s := store.NewStore()
	if err := cmdDEBOUNCEXCALL(discardCtx("", bytesArgs("x"), s)); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCmdDEBOUNCEXCANCEL_Success(t *testing.T) {
	s := store.NewStore()
	cmdDEBOUNCEXCREATE(discardCtx("", bytesArgs("debcan", "500"), s))
	if err := cmdDEBOUNCEXCANCEL(discardCtx("", bytesArgs("debcan"), s)); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCmdDEBOUNCEXCANCEL_WrongArgs(t *testing.T) {
	s := store.NewStore()
	if err := cmdDEBOUNCEXCANCEL(discardCtx("", bytesArgs(), s)); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCmdDEBOUNCEXFLUSH_Success(t *testing.T) {
	s := store.NewStore()
	cmdDEBOUNCEXCREATE(discardCtx("", bytesArgs("debfl", "500"), s))
	cmdDEBOUNCEXCALL(discardCtx("", bytesArgs("debfl", "arg1"), s))
	if err := cmdDEBOUNCEXFLUSH(discardCtx("", bytesArgs("debfl"), s)); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCmdDEBOUNCEXFLUSH_WrongArgs(t *testing.T) {
	s := store.NewStore()
	if err := cmdDEBOUNCEXFLUSH(discardCtx("", bytesArgs(), s)); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCmdCOALESCECREATE_Success(t *testing.T) {
	s := store.NewStore()
	if err := cmdCOALESCECREATE(discardCtx("", bytesArgs("coal1"), s)); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCmdCOALESCECREATE_WrongArgs(t *testing.T) {
	s := store.NewStore()
	if err := cmdCOALESCECREATE(discardCtx("", bytesArgs(), s)); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCmdCOALESCEADD_Success(t *testing.T) {
	s := store.NewStore()
	cmdCOALESCECREATE(discardCtx("", bytesArgs("coaladd"), s))
	if err := cmdCOALESCEADD(discardCtx("", bytesArgs("coaladd", "val1"), s)); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCmdCOALESCEADD_WrongArgs(t *testing.T) {
	s := store.NewStore()
	if err := cmdCOALESCEADD(discardCtx("", bytesArgs("x"), s)); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCmdCOALESCEGET_Success(t *testing.T) {
	s := store.NewStore()
	cmdCOALESCECREATE(discardCtx("", bytesArgs("coalget"), s))
	cmdCOALESCEADD(discardCtx("", bytesArgs("coalget", "val1"), s))
	if err := cmdCOALESCEGET(discardCtx("", bytesArgs("coalget"), s)); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCmdCOALESCEGET_WrongArgs(t *testing.T) {
	s := store.NewStore()
	if err := cmdCOALESCEGET(discardCtx("", bytesArgs(), s)); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCmdCOALESCECLEAR_Success(t *testing.T) {
	s := store.NewStore()
	cmdCOALESCECREATE(discardCtx("", bytesArgs("coalclr"), s))
	if err := cmdCOALESCECLEAR(discardCtx("", bytesArgs("coalclr"), s)); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCmdCOALESCECLEAR_WrongArgs(t *testing.T) {
	s := store.NewStore()
	if err := cmdCOALESCECLEAR(discardCtx("", bytesArgs(), s)); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCmdAGGREGATORCREATE_Success(t *testing.T) {
	s := store.NewStore()
	if err := cmdAGGREGATORCREATE(discardCtx("", bytesArgs("agg1", "sum"), s)); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCmdAGGREGATORCREATE_WrongArgs(t *testing.T) {
	s := store.NewStore()
	if err := cmdAGGREGATORCREATE(discardCtx("", bytesArgs("x"), s)); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCmdAGGREGATORADD_Success(t *testing.T) {
	s := store.NewStore()
	cmdAGGREGATORCREATE(discardCtx("", bytesArgs("aggadd", "sum"), s))
	if err := cmdAGGREGATORADD(discardCtx("", bytesArgs("aggadd", "3.14"), s)); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCmdAGGREGATORADD_WrongArgs(t *testing.T) {
	s := store.NewStore()
	if err := cmdAGGREGATORADD(discardCtx("", bytesArgs("x"), s)); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCmdAGGREGATORGET_Success(t *testing.T) {
	s := store.NewStore()
	cmdAGGREGATORCREATE(discardCtx("", bytesArgs("aggget", "sum"), s))
	cmdAGGREGATORADD(discardCtx("", bytesArgs("aggget", "10"), s))
	if err := cmdAGGREGATORGET(discardCtx("", bytesArgs("aggget"), s)); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCmdAGGREGATORGET_WrongArgs(t *testing.T) {
	s := store.NewStore()
	if err := cmdAGGREGATORGET(discardCtx("", bytesArgs(), s)); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCmdAGGREGATORRESET_Success(t *testing.T) {
	s := store.NewStore()
	cmdAGGREGATORCREATE(discardCtx("", bytesArgs("aggrst", "sum"), s))
	if err := cmdAGGREGATORRESET(discardCtx("", bytesArgs("aggrst"), s)); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCmdAGGREGATORRESET_WrongArgs(t *testing.T) {
	s := store.NewStore()
	if err := cmdAGGREGATORRESET(discardCtx("", bytesArgs(), s)); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCmdWINDOWXCREATE_Success(t *testing.T) {
	s := store.NewStore()
	if err := cmdWINDOWXCREATE(discardCtx("", bytesArgs("win1", "10"), s)); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCmdWINDOWXCREATE_WrongArgs(t *testing.T) {
	s := store.NewStore()
	if err := cmdWINDOWXCREATE(discardCtx("", bytesArgs("x"), s)); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCmdWINDOWXADD_Success(t *testing.T) {
	s := store.NewStore()
	cmdWINDOWXCREATE(discardCtx("", bytesArgs("winadd", "10"), s))
	if err := cmdWINDOWXADD(discardCtx("", bytesArgs("winadd", "5.0"), s)); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCmdWINDOWXADD_WrongArgs(t *testing.T) {
	s := store.NewStore()
	if err := cmdWINDOWXADD(discardCtx("", bytesArgs("x"), s)); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCmdWINDOWXGET_Success(t *testing.T) {
	s := store.NewStore()
	cmdWINDOWXCREATE(discardCtx("", bytesArgs("winget", "10"), s))
	if err := cmdWINDOWXGET(discardCtx("", bytesArgs("winget"), s)); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCmdWINDOWXGET_WrongArgs(t *testing.T) {
	s := store.NewStore()
	if err := cmdWINDOWXGET(discardCtx("", bytesArgs(), s)); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCmdWINDOWXAGGREGATE_Success(t *testing.T) {
	s := store.NewStore()
	cmdWINDOWXCREATE(discardCtx("", bytesArgs("winagg", "10"), s))
	cmdWINDOWXADD(discardCtx("", bytesArgs("winagg", "5.0"), s))
	cmdWINDOWXADD(discardCtx("", bytesArgs("winagg", "15.0"), s))
	if err := cmdWINDOWXAGGREGATE(discardCtx("", bytesArgs("winagg", "sum"), s)); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCmdWINDOWXAGGREGATE_WrongArgs(t *testing.T) {
	s := store.NewStore()
	if err := cmdWINDOWXAGGREGATE(discardCtx("", bytesArgs("x"), s)); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCmdJOINXCREATE_Success(t *testing.T) {
	s := store.NewStore()
	if err := cmdJOINXCREATE(discardCtx("", bytesArgs("join1"), s)); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCmdJOINXCREATE_WrongArgs(t *testing.T) {
	s := store.NewStore()
	if err := cmdJOINXCREATE(discardCtx("", bytesArgs(), s)); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCmdJOINXADD_Success(t *testing.T) {
	s := store.NewStore()
	cmdJOINXCREATE(discardCtx("", bytesArgs("joinadd"), s))
	if err := cmdJOINXADD(discardCtx("", bytesArgs("joinadd", "left", "val1"), s)); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCmdJOINXADD_WrongArgs(t *testing.T) {
	s := store.NewStore()
	if err := cmdJOINXADD(discardCtx("", bytesArgs("x", "y"), s)); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCmdJOINXGET_Success(t *testing.T) {
	s := store.NewStore()
	cmdJOINXCREATE(discardCtx("", bytesArgs("joinget"), s))
	if err := cmdJOINXGET(discardCtx("", bytesArgs("joinget"), s)); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCmdJOINXGET_WrongArgs(t *testing.T) {
	s := store.NewStore()
	if err := cmdJOINXGET(discardCtx("", bytesArgs(), s)); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCmdJOINXDELETE_Success(t *testing.T) {
	s := store.NewStore()
	cmdJOINXCREATE(discardCtx("", bytesArgs("joindel"), s))
	if err := cmdJOINXDELETE(discardCtx("", bytesArgs("joindel"), s)); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCmdJOINXDELETE_WrongArgs(t *testing.T) {
	s := store.NewStore()
	if err := cmdJOINXDELETE(discardCtx("", bytesArgs(), s)); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCmdSHUFFLECREATE_Success(t *testing.T) {
	s := store.NewStore()
	if err := cmdSHUFFLECREATE(discardCtx("", bytesArgs("shuf1"), s)); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCmdSHUFFLECREATE_WrongArgs(t *testing.T) {
	s := store.NewStore()
	if err := cmdSHUFFLECREATE(discardCtx("", bytesArgs(), s)); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCmdSHUFFLEADD_Success(t *testing.T) {
	s := store.NewStore()
	cmdSHUFFLECREATE(discardCtx("", bytesArgs("shufadd"), s))
	if err := cmdSHUFFLEADD(discardCtx("", bytesArgs("shufadd", "val1"), s)); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCmdSHUFFLEADD_WrongArgs(t *testing.T) {
	s := store.NewStore()
	if err := cmdSHUFFLEADD(discardCtx("", bytesArgs("x"), s)); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCmdSHUFFLEGET_Success(t *testing.T) {
	s := store.NewStore()
	cmdSHUFFLECREATE(discardCtx("", bytesArgs("shufget"), s))
	cmdSHUFFLEADD(discardCtx("", bytesArgs("shufget", "val1"), s))
	if err := cmdSHUFFLEGET(discardCtx("", bytesArgs("shufget"), s)); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCmdSHUFFLEGET_WrongArgs(t *testing.T) {
	s := store.NewStore()
	if err := cmdSHUFFLEGET(discardCtx("", bytesArgs(), s)); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCmdPARTITIONXCREATE_Success(t *testing.T) {
	s := store.NewStore()
	if err := cmdPARTITIONXCREATE(discardCtx("", bytesArgs("part1", "4"), s)); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCmdPARTITIONXCREATE_WrongArgs(t *testing.T) {
	s := store.NewStore()
	if err := cmdPARTITIONXCREATE(discardCtx("", bytesArgs("x"), s)); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

// ===========================================================================
// INTEGRATION COMMANDS
// ===========================================================================

func TestCmdCIRCUITBREAKERCREATE_Success(t *testing.T) {
	s := store.NewStore()
	if err := cmdCIRCUITBREAKERCREATE(discardCtx("", bytesArgs("cb1", "5", "1000"), s)); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCmdCIRCUITBREAKERCREATE_WrongArgs(t *testing.T) {
	s := store.NewStore()
	if err := cmdCIRCUITBREAKERCREATE(discardCtx("", bytesArgs("cb1"), s)); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCmdCIRCUITBREAKERSTATE_Success(t *testing.T) {
	s := store.NewStore()
	cmdCIRCUITBREAKERCREATE(discardCtx("", bytesArgs("cbst", "5", "1000"), s))
	if err := cmdCIRCUITBREAKERSTATE(discardCtx("", bytesArgs("cbst"), s)); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCmdCIRCUITBREAKERSTATE_WrongArgs(t *testing.T) {
	s := store.NewStore()
	if err := cmdCIRCUITBREAKERSTATE(discardCtx("", bytesArgs(), s)); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCmdCIRCUITBREAKERTRIP_Success(t *testing.T) {
	s := store.NewStore()
	cmdCIRCUITBREAKERCREATE(discardCtx("", bytesArgs("cbtrip", "5", "1000"), s))
	if err := cmdCIRCUITBREAKERTRIP(discardCtx("", bytesArgs("cbtrip"), s)); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCmdCIRCUITBREAKERTRIP_WrongArgs(t *testing.T) {
	s := store.NewStore()
	if err := cmdCIRCUITBREAKERTRIP(discardCtx("", bytesArgs(), s)); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCmdCIRCUITBREAKERRESET_Success(t *testing.T) {
	s := store.NewStore()
	cmdCIRCUITBREAKERCREATE(discardCtx("", bytesArgs("cbreset", "5", "1000"), s))
	if err := cmdCIRCUITBREAKERRESET(discardCtx("", bytesArgs("cbreset"), s)); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCmdCIRCUITBREAKERRESET_WrongArgs(t *testing.T) {
	s := store.NewStore()
	if err := cmdCIRCUITBREAKERRESET(discardCtx("", bytesArgs(), s)); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCmdRATELIMITCREATE_Success(t *testing.T) {
	s := store.NewStore()
	if err := cmdRATELIMITCREATE(discardCtx("", bytesArgs("rl_ext1", "10", "1000"), s)); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCmdRATELIMITCREATE_WrongArgs(t *testing.T) {
	s := store.NewStore()
	if err := cmdRATELIMITCREATE(discardCtx("", bytesArgs("x"), s)); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCmdRATELIMITCHECK_Success(t *testing.T) {
	s := store.NewStore()
	cmdRATELIMITCREATE(discardCtx("", bytesArgs("rl_chk", "10", "60000"), s))
	if err := cmdRATELIMITCHECK(discardCtx("", bytesArgs("rl_chk", "client1"), s)); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCmdRATELIMITCHECK_WrongArgs(t *testing.T) {
	s := store.NewStore()
	if err := cmdRATELIMITCHECK(discardCtx("", bytesArgs("x"), s)); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCmdRATELIMITRESET_Success(t *testing.T) {
	s := store.NewStore()
	cmdRATELIMITCREATE(discardCtx("", bytesArgs("rl_rst", "10", "60000"), s))
	if err := cmdRATELIMITRESET(discardCtx("", bytesArgs("rl_rst"), s)); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCmdRATELIMITRESET_WrongArgs(t *testing.T) {
	s := store.NewStore()
	if err := cmdRATELIMITRESET(discardCtx("", bytesArgs(), s)); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCmdRATELIMITDELETE_Success(t *testing.T) {
	s := store.NewStore()
	cmdRATELIMITCREATE(discardCtx("", bytesArgs("rl_del", "10", "60000"), s))
	if err := cmdRATELIMITDELETE(discardCtx("", bytesArgs("rl_del"), s)); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCmdRATELIMITDELETE_WrongArgs(t *testing.T) {
	s := store.NewStore()
	if err := cmdRATELIMITDELETE(discardCtx("", bytesArgs(), s)); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCmdCACHELOCK_Success(t *testing.T) {
	s := store.NewStore()
	if err := cmdCACHELOCK(discardCtx("", bytesArgs("key1", "holder1", "5000"), s)); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCmdCACHELOCK_WrongArgs(t *testing.T) {
	s := store.NewStore()
	if err := cmdCACHELOCK(discardCtx("", bytesArgs("key1", "holder1"), s)); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCmdCACHEUNLOCK_Success(t *testing.T) {
	s := store.NewStore()
	cmdCACHELOCK(discardCtx("", bytesArgs("clk1", "holder1", "5000"), s))
	if err := cmdCACHEUNLOCK(discardCtx("", bytesArgs("clk1", "holder1"), s)); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCmdCACHEUNLOCK_WrongArgs(t *testing.T) {
	s := store.NewStore()
	if err := cmdCACHEUNLOCK(discardCtx("", bytesArgs("x"), s)); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCmdCACHELOCKED_Success(t *testing.T) {
	s := store.NewStore()
	if err := cmdCACHELOCKED(discardCtx("", bytesArgs("nonexist"), s)); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCmdCACHELOCKED_WrongArgs(t *testing.T) {
	s := store.NewStore()
	if err := cmdCACHELOCKED(discardCtx("", bytesArgs(), s)); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCmdCACHEREFRESH_Success(t *testing.T) {
	s := store.NewStore()
	cmdCACHELOCK(discardCtx("", bytesArgs("cref", "holder1", "5000"), s))
	if err := cmdCACHEREFRESH(discardCtx("", bytesArgs("cref", "holder1", "10000"), s)); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCmdCACHEREFRESH_WrongArgs(t *testing.T) {
	s := store.NewStore()
	if err := cmdCACHEREFRESH(discardCtx("", bytesArgs("x", "y"), s)); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCmdNETWHOIS_Success(t *testing.T) {
	s := store.NewStore()
	if err := cmdNETWHOIS(discardCtx("", bytesArgs("example.com"), s)); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCmdNETWHOIS_WrongArgs(t *testing.T) {
	s := store.NewStore()
	if err := cmdNETWHOIS(discardCtx("", bytesArgs(), s)); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCmdNETDNS_Success(t *testing.T) {
	s := store.NewStore()
	if err := cmdNETDNS(discardCtx("", bytesArgs("example.com"), s)); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCmdNETDNS_WrongArgs(t *testing.T) {
	s := store.NewStore()
	if err := cmdNETDNS(discardCtx("", bytesArgs(), s)); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCmdNETPING_Success(t *testing.T) {
	s := store.NewStore()
	if err := cmdNETPING(discardCtx("", bytesArgs("127.0.0.1"), s)); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCmdNETPING_WrongArgs(t *testing.T) {
	s := store.NewStore()
	if err := cmdNETPING(discardCtx("", bytesArgs(), s)); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCmdNETPORT_Success(t *testing.T) {
	s := store.NewStore()
	if err := cmdNETPORT(discardCtx("", bytesArgs("127.0.0.1", "8080"), s)); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCmdNETPORT_WrongArgs(t *testing.T) {
	s := store.NewStore()
	if err := cmdNETPORT(discardCtx("", bytesArgs("x"), s)); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCmdARRAYPUSH_Success(t *testing.T) {
	s := store.NewStore()
	if err := cmdARRAYPUSH(discardCtx("", bytesArgs("arr1", "val1"), s)); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCmdARRAYPUSH_WrongArgs(t *testing.T) {
	s := store.NewStore()
	if err := cmdARRAYPUSH(discardCtx("", bytesArgs("arr1"), s)); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCmdARRAYPOP_Success(t *testing.T) {
	s := store.NewStore()
	cmdARRAYPUSH(discardCtx("", bytesArgs("arrpop", "v1"), s))
	if err := cmdARRAYPOP(discardCtx("", bytesArgs("arrpop"), s)); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCmdARRAYPOP_WrongArgs(t *testing.T) {
	s := store.NewStore()
	if err := cmdARRAYPOP(discardCtx("", bytesArgs(), s)); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCmdARRAYSHIFT_Success(t *testing.T) {
	s := store.NewStore()
	cmdARRAYPUSH(discardCtx("", bytesArgs("arrshift", "v1"), s))
	if err := cmdARRAYSHIFT(discardCtx("", bytesArgs("arrshift"), s)); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCmdARRAYSHIFT_WrongArgs(t *testing.T) {
	s := store.NewStore()
	if err := cmdARRAYSHIFT(discardCtx("", bytesArgs(), s)); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCmdARRAYUNSHIFT_Success(t *testing.T) {
	s := store.NewStore()
	if err := cmdARRAYUNSHIFT(discardCtx("", bytesArgs("arrunsh", "v1"), s)); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCmdARRAYUNSHIFT_WrongArgs(t *testing.T) {
	s := store.NewStore()
	if err := cmdARRAYUNSHIFT(discardCtx("", bytesArgs("x"), s)); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCmdARRAYSLICE_Success(t *testing.T) {
	s := store.NewStore()
	cmdARRAYPUSH(discardCtx("", bytesArgs("arrslice", "a", "b", "c"), s))
	if err := cmdARRAYSLICE(discardCtx("", bytesArgs("arrslice", "0", "2"), s)); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCmdARRAYSLICE_WrongArgs(t *testing.T) {
	s := store.NewStore()
	if err := cmdARRAYSLICE(discardCtx("", bytesArgs("x", "0"), s)); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCmdARRAYREVERSE_Success(t *testing.T) {
	s := store.NewStore()
	cmdARRAYPUSH(discardCtx("", bytesArgs("arrrev", "a", "b"), s))
	if err := cmdARRAYREVERSE(discardCtx("", bytesArgs("arrrev"), s)); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCmdARRAYREVERSE_WrongArgs(t *testing.T) {
	s := store.NewStore()
	if err := cmdARRAYREVERSE(discardCtx("", bytesArgs(), s)); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCmdARRAYSORT_Success(t *testing.T) {
	s := store.NewStore()
	cmdARRAYPUSH(discardCtx("", bytesArgs("arrsort", "c", "a", "b"), s))
	if err := cmdARRAYSORT(discardCtx("", bytesArgs("arrsort"), s)); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCmdARRAYSORT_WrongArgs(t *testing.T) {
	s := store.NewStore()
	if err := cmdARRAYSORT(discardCtx("", bytesArgs(), s)); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCmdARRAYUNIQUE_Success(t *testing.T) {
	s := store.NewStore()
	cmdARRAYPUSH(discardCtx("", bytesArgs("arruniq", "a", "a", "b"), s))
	if err := cmdARRAYUNIQUE(discardCtx("", bytesArgs("arruniq"), s)); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCmdARRAYUNIQUE_WrongArgs(t *testing.T) {
	s := store.NewStore()
	if err := cmdARRAYUNIQUE(discardCtx("", bytesArgs(), s)); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCmdARRAYFLATTEN_Success(t *testing.T) {
	s := store.NewStore()
	cmdARRAYPUSH(discardCtx("", bytesArgs("arrflat", "a", "b"), s))
	if err := cmdARRAYFLATTEN(discardCtx("", bytesArgs("arrflat"), s)); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCmdARRAYFLATTEN_WrongArgs(t *testing.T) {
	s := store.NewStore()
	if err := cmdARRAYFLATTEN(discardCtx("", bytesArgs(), s)); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCmdARRAYINTERSECT_Success(t *testing.T) {
	s := store.NewStore()
	cmdARRAYPUSH(discardCtx("", bytesArgs("aint1", "a", "b", "c"), s))
	cmdARRAYPUSH(discardCtx("", bytesArgs("aint2", "b", "c", "d"), s))
	if err := cmdARRAYINTERSECT(discardCtx("", bytesArgs("aint1", "aint2"), s)); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCmdARRAYINTERSECT_WrongArgs(t *testing.T) {
	s := store.NewStore()
	if err := cmdARRAYINTERSECT(discardCtx("", bytesArgs("x"), s)); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCmdARRAYDIFF_Success(t *testing.T) {
	s := store.NewStore()
	cmdARRAYPUSH(discardCtx("", bytesArgs("adiff1", "a", "b", "c"), s))
	cmdARRAYPUSH(discardCtx("", bytesArgs("adiff2", "b"), s))
	if err := cmdARRAYDIFF(discardCtx("", bytesArgs("adiff1", "adiff2"), s)); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCmdARRAYDIFF_WrongArgs(t *testing.T) {
	s := store.NewStore()
	if err := cmdARRAYDIFF(discardCtx("", bytesArgs("x"), s)); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCmdARRAYINDEXOF_Success(t *testing.T) {
	s := store.NewStore()
	cmdARRAYPUSH(discardCtx("", bytesArgs("aidx", "a", "b"), s))
	if err := cmdARRAYINDEXOF(discardCtx("", bytesArgs("aidx", "b"), s)); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCmdARRAYINDEXOF_WrongArgs(t *testing.T) {
	s := store.NewStore()
	if err := cmdARRAYINDEXOF(discardCtx("", bytesArgs("x"), s)); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCmdARRAYLASTINDEXOF_Success(t *testing.T) {
	s := store.NewStore()
	cmdARRAYPUSH(discardCtx("", bytesArgs("alidx", "a", "b", "a"), s))
	if err := cmdARRAYLASTINDEXOF(discardCtx("", bytesArgs("alidx", "a"), s)); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCmdARRAYLASTINDEXOF_WrongArgs(t *testing.T) {
	s := store.NewStore()
	if err := cmdARRAYLASTINDEXOF(discardCtx("", bytesArgs("x"), s)); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCmdARRAYINCLUDES_Success(t *testing.T) {
	s := store.NewStore()
	cmdARRAYPUSH(discardCtx("", bytesArgs("ainc", "a", "b"), s))
	if err := cmdARRAYINCLUDES(discardCtx("", bytesArgs("ainc", "a"), s)); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCmdARRAYINCLUDES_WrongArgs(t *testing.T) {
	s := store.NewStore()
	if err := cmdARRAYINCLUDES(discardCtx("", bytesArgs("x"), s)); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCmdMATHADD_Success(t *testing.T) {
	s := store.NewStore()
	ctx, buf := bufCtx("MATH.ADD", bytesArgs("10", "20"), s)
	if err := cmdMATHADD(ctx); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(buf.String(), "30") {
		t.Errorf("Expected 30, got %q", buf.String())
	}
}

func TestCmdMATHADD_WrongArgs(t *testing.T) {
	s := store.NewStore()
	if err := cmdMATHADD(discardCtx("", bytesArgs("10"), s)); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCmdMATHSUB_Success(t *testing.T) {
	s := store.NewStore()
	if err := cmdMATHSUB(discardCtx("", bytesArgs("30", "10"), s)); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCmdMATHSUB_WrongArgs(t *testing.T) {
	s := store.NewStore()
	if err := cmdMATHSUB(discardCtx("", bytesArgs("10"), s)); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCmdMATHMUL_Success(t *testing.T) {
	s := store.NewStore()
	if err := cmdMATHMUL(discardCtx("", bytesArgs("5", "6"), s)); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCmdMATHMUL_WrongArgs(t *testing.T) {
	s := store.NewStore()
	if err := cmdMATHMUL(discardCtx("", bytesArgs("5"), s)); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCmdMATHDIV_Success(t *testing.T) {
	s := store.NewStore()
	if err := cmdMATHDIV(discardCtx("", bytesArgs("30", "5"), s)); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCmdMATHDIV_WrongArgs(t *testing.T) {
	s := store.NewStore()
	if err := cmdMATHDIV(discardCtx("", bytesArgs("10"), s)); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCmdMATHMOD_Success(t *testing.T) {
	s := store.NewStore()
	if err := cmdMATHMOD(discardCtx("", bytesArgs("10", "3"), s)); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCmdMATHMOD_WrongArgs(t *testing.T) {
	s := store.NewStore()
	if err := cmdMATHMOD(discardCtx("", bytesArgs("10"), s)); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCmdMATHPOW_Success(t *testing.T) {
	s := store.NewStore()
	if err := cmdMATHPOW(discardCtx("", bytesArgs("2", "10"), s)); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCmdMATHPOW_WrongArgs(t *testing.T) {
	s := store.NewStore()
	if err := cmdMATHPOW(discardCtx("", bytesArgs("2"), s)); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCmdMATHSQRT_Success(t *testing.T) {
	s := store.NewStore()
	if err := cmdMATHSQRT(discardCtx("", bytesArgs("16"), s)); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCmdMATHSQRT_WrongArgs(t *testing.T) {
	s := store.NewStore()
	if err := cmdMATHSQRT(discardCtx("", bytesArgs(), s)); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCmdMATHABS_Success(t *testing.T) {
	s := store.NewStore()
	if err := cmdMATHABS(discardCtx("", bytesArgs("-5"), s)); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCmdMATHABS_WrongArgs(t *testing.T) {
	s := store.NewStore()
	if err := cmdMATHABS(discardCtx("", bytesArgs(), s)); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCmdMATHMIN_Success(t *testing.T) {
	s := store.NewStore()
	if err := cmdMATHMIN(discardCtx("", bytesArgs("5", "3"), s)); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCmdMATHMIN_WrongArgs(t *testing.T) {
	s := store.NewStore()
	if err := cmdMATHMIN(discardCtx("", bytesArgs("5"), s)); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCmdMATHMAX_Success(t *testing.T) {
	s := store.NewStore()
	if err := cmdMATHMAX(discardCtx("", bytesArgs("5", "3"), s)); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCmdMATHMAX_WrongArgs(t *testing.T) {
	s := store.NewStore()
	if err := cmdMATHMAX(discardCtx("", bytesArgs("5"), s)); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCmdMATHFLOOR_Success(t *testing.T) {
	s := store.NewStore()
	if err := cmdMATHFLOOR(discardCtx("", bytesArgs("3.7"), s)); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCmdMATHFLOOR_WrongArgs(t *testing.T) {
	s := store.NewStore()
	if err := cmdMATHFLOOR(discardCtx("", bytesArgs(), s)); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCmdMATHCEIL_Success(t *testing.T) {
	s := store.NewStore()
	if err := cmdMATHCEIL(discardCtx("", bytesArgs("3.2"), s)); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCmdMATHCEIL_WrongArgs(t *testing.T) {
	s := store.NewStore()
	if err := cmdMATHCEIL(discardCtx("", bytesArgs(), s)); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCmdMATHROUND_Success(t *testing.T) {
	s := store.NewStore()
	if err := cmdMATHROUND(discardCtx("", bytesArgs("3.5"), s)); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCmdMATHROUND_WrongArgs(t *testing.T) {
	s := store.NewStore()
	if err := cmdMATHROUND(discardCtx("", bytesArgs(), s)); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCmdMATHRANDOM_Success(t *testing.T) {
	s := store.NewStore()
	if err := cmdMATHRANDOM(discardCtx("", bytesArgs("1", "100"), s)); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCmdMATHSUM_Success(t *testing.T) {
	s := store.NewStore()
	if err := cmdMATHSUM(discardCtx("", bytesArgs("1", "2", "3"), s)); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCmdMATHSUM_WrongArgs(t *testing.T) {
	s := store.NewStore()
	if err := cmdMATHSUM(discardCtx("", bytesArgs(), s)); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCmdMATHAVG_Success(t *testing.T) {
	s := store.NewStore()
	if err := cmdMATHAVG(discardCtx("", bytesArgs("10", "20", "30"), s)); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCmdMATHAVG_WrongArgs(t *testing.T) {
	s := store.NewStore()
	if err := cmdMATHAVG(discardCtx("", bytesArgs(), s)); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCmdMATHMEDIAN_Success(t *testing.T) {
	s := store.NewStore()
	if err := cmdMATHMEDIAN(discardCtx("", bytesArgs("1", "3", "2"), s)); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCmdMATHMEDIAN_WrongArgs(t *testing.T) {
	s := store.NewStore()
	if err := cmdMATHMEDIAN(discardCtx("", bytesArgs(), s)); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCmdMATHSTDDEV_Success(t *testing.T) {
	s := store.NewStore()
	if err := cmdMATHSTDDEV(discardCtx("", bytesArgs("10", "20"), s)); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCmdMATHSTDDEV_WrongArgs(t *testing.T) {
	s := store.NewStore()
	if err := cmdMATHSTDDEV(discardCtx("", bytesArgs("10"), s)); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCmdGEOENCODE_Success(t *testing.T) {
	s := store.NewStore()
	if err := cmdGEOENCODE(discardCtx("", bytesArgs("40.7128", "-74.0060"), s)); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCmdGEODECODE_Success(t *testing.T) {
	s := store.NewStore()
	if err := cmdGEODECODE(discardCtx("", bytesArgs("dr5r7p"), s)); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCmdGEODISTANCE_Success(t *testing.T) {
	s := store.NewStore()
	if err := cmdGEODISTANCE(discardCtx("", bytesArgs("40.7128", "-74.0060", "34.0522", "-118.2437"), s)); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCmdOBJECTSET_Success(t *testing.T) {
	s := store.NewStore()
	if err := cmdOBJECTSET(discardCtx("", bytesArgs("obj1", "key1", "val1"), s)); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCmdOBJECTSET_WrongArgs(t *testing.T) {
	s := store.NewStore()
	if err := cmdOBJECTSET(discardCtx("", bytesArgs("obj1", "key1"), s)); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCmdOBJECTGET_Success(t *testing.T) {
	s := store.NewStore()
	cmdOBJECTSET(discardCtx("", bytesArgs("objget", "k", "v"), s))
	if err := cmdOBJECTGET(discardCtx("", bytesArgs("objget", "k"), s)); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCmdOBJECTGET_WrongArgs(t *testing.T) {
	s := store.NewStore()
	if err := cmdOBJECTGET(discardCtx("", bytesArgs("x"), s)); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCmdOBJECTKEYS_Success(t *testing.T) {
	s := store.NewStore()
	cmdOBJECTSET(discardCtx("", bytesArgs("objkeys", "k1", "v1"), s))
	if err := cmdOBJECTKEYS(discardCtx("", bytesArgs("objkeys"), s)); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCmdOBJECTKEYS_WrongArgs(t *testing.T) {
	s := store.NewStore()
	if err := cmdOBJECTKEYS(discardCtx("", bytesArgs(), s)); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCmdOBJECTVALUES_Success(t *testing.T) {
	s := store.NewStore()
	cmdOBJECTSET(discardCtx("", bytesArgs("objvals", "k1", "v1"), s))
	if err := cmdOBJECTVALUES(discardCtx("", bytesArgs("objvals"), s)); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCmdOBJECTVALUES_WrongArgs(t *testing.T) {
	s := store.NewStore()
	if err := cmdOBJECTVALUES(discardCtx("", bytesArgs(), s)); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCmdOBJECTENTRIES_Success(t *testing.T) {
	s := store.NewStore()
	cmdOBJECTSET(discardCtx("", bytesArgs("objent", "k1", "v1"), s))
	if err := cmdOBJECTENTRIES(discardCtx("", bytesArgs("objent"), s)); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCmdOBJECTENTRIES_WrongArgs(t *testing.T) {
	s := store.NewStore()
	if err := cmdOBJECTENTRIES(discardCtx("", bytesArgs(), s)); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCmdOBJECTHAS_Success(t *testing.T) {
	s := store.NewStore()
	cmdOBJECTSET(discardCtx("", bytesArgs("objhas", "k1", "v1"), s))
	if err := cmdOBJECTHAS(discardCtx("", bytesArgs("objhas", "k1"), s)); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCmdOBJECTHAS_WrongArgs(t *testing.T) {
	s := store.NewStore()
	if err := cmdOBJECTHAS(discardCtx("", bytesArgs("x"), s)); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCmdOBJECTDELETE_Success(t *testing.T) {
	s := store.NewStore()
	cmdOBJECTSET(discardCtx("", bytesArgs("objdel", "k1", "v1"), s))
	if err := cmdOBJECTDELETE(discardCtx("", bytesArgs("objdel", "k1"), s)); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCmdOBJECTDELETE_WrongArgs(t *testing.T) {
	s := store.NewStore()
	if err := cmdOBJECTDELETE(discardCtx("", bytesArgs("x"), s)); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCmdSEQUENCENEXT_Success(t *testing.T) {
	s := store.NewStore()
	if err := cmdSEQUENCENEXT(discardCtx("", bytesArgs("seq1"), s)); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCmdSEQUENCENEXT_WrongArgs(t *testing.T) {
	s := store.NewStore()
	if err := cmdSEQUENCENEXT(discardCtx("", bytesArgs(), s)); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCmdSEQUENCECURRENT_Success(t *testing.T) {
	s := store.NewStore()
	cmdSEQUENCENEXT(discardCtx("", bytesArgs("seqcur"), s))
	if err := cmdSEQUENCECURRENT(discardCtx("", bytesArgs("seqcur"), s)); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCmdSEQUENCECURRENT_WrongArgs(t *testing.T) {
	s := store.NewStore()
	if err := cmdSEQUENCECURRENT(discardCtx("", bytesArgs(), s)); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

// ===========================================================================
// ENCODING COMMANDS
// ===========================================================================

func TestCmdMSGPACKENCODE_Success(t *testing.T) {
	s := store.NewStore()
	if err := cmdMSGPACKENCODE(discardCtx("", bytesArgs("hello"), s)); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCmdMSGPACKENCODE_WrongArgs(t *testing.T) {
	s := store.NewStore()
	if err := cmdMSGPACKENCODE(discardCtx("", bytesArgs(), s)); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCmdBSONENCODE_Success(t *testing.T) {
	s := store.NewStore()
	if err := cmdBSONENCODE(discardCtx("", bytesArgs("data"), s)); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCmdBSONENCODE_WrongArgs(t *testing.T) {
	s := store.NewStore()
	if err := cmdBSONENCODE(discardCtx("", bytesArgs(), s)); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCmdURLENCODE_Success(t *testing.T) {
	s := store.NewStore()
	if err := cmdURLENCODE(discardCtx("", bytesArgs("hello world"), s)); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCmdURLENCODE_WrongArgs(t *testing.T) {
	s := store.NewStore()
	if err := cmdURLENCODE(discardCtx("", bytesArgs(), s)); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCmdURLDECODE_Success(t *testing.T) {
	s := store.NewStore()
	if err := cmdURLDECODE(discardCtx("", bytesArgs("hello+world"), s)); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCmdURLDECODE_WrongArgs(t *testing.T) {
	s := store.NewStore()
	if err := cmdURLDECODE(discardCtx("", bytesArgs(), s)); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCmdXMLENCODE_Success(t *testing.T) {
	s := store.NewStore()
	if err := cmdXMLENCODE(discardCtx("", bytesArgs("name", "test"), s)); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCmdXMLENCODE_WrongArgs(t *testing.T) {
	s := store.NewStore()
	if err := cmdXMLENCODE(discardCtx("", bytesArgs("x"), s)); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCmdXMLDECODE_Success(t *testing.T) {
	s := store.NewStore()
	if err := cmdXMLDECODE(discardCtx("", bytesArgs("<name>test</name>"), s)); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCmdXMLDECODE_WrongArgs(t *testing.T) {
	s := store.NewStore()
	if err := cmdXMLDECODE(discardCtx("", bytesArgs(), s)); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCmdYAMLENCODE_Success(t *testing.T) {
	s := store.NewStore()
	if err := cmdYAMLENCODE(discardCtx("", bytesArgs("key", "value"), s)); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCmdYAMLENCODE_WrongArgs(t *testing.T) {
	s := store.NewStore()
	if err := cmdYAMLENCODE(discardCtx("", bytesArgs(), s)); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCmdYAMLDECODE_Success(t *testing.T) {
	s := store.NewStore()
	if err := cmdYAMLDECODE(discardCtx("", bytesArgs("key: value"), s)); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCmdYAMLDECODE_WrongArgs(t *testing.T) {
	s := store.NewStore()
	if err := cmdYAMLDECODE(discardCtx("", bytesArgs(), s)); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCmdTOMLENCODE_Success(t *testing.T) {
	s := store.NewStore()
	if err := cmdTOMLENCODE(discardCtx("", bytesArgs("key", "value"), s)); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCmdTOMLENCODE_WrongArgs(t *testing.T) {
	s := store.NewStore()
	if err := cmdTOMLENCODE(discardCtx("", bytesArgs("x"), s)); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCmdTOMLDECODE_Success(t *testing.T) {
	s := store.NewStore()
	if err := cmdTOMLDECODE(discardCtx("", bytesArgs(`key = "value"`), s)); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCmdTOMLDECODE_WrongArgs(t *testing.T) {
	s := store.NewStore()
	if err := cmdTOMLDECODE(discardCtx("", bytesArgs(), s)); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCmdCBORENCODE_Success(t *testing.T) {
	s := store.NewStore()
	if err := cmdCBORENCODE(discardCtx("", bytesArgs("hello"), s)); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCmdCBORENCODE_WrongArgs(t *testing.T) {
	s := store.NewStore()
	if err := cmdCBORENCODE(discardCtx("", bytesArgs(), s)); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCmdCSVENCODE_Success(t *testing.T) {
	s := store.NewStore()
	if err := cmdCSVENCODE(discardCtx("", bytesArgs("a", "b", "c"), s)); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCmdCSVENCODE_WrongArgs(t *testing.T) {
	s := store.NewStore()
	if err := cmdCSVENCODE(discardCtx("", bytesArgs(), s)); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCmdCSVDECODE_Success(t *testing.T) {
	s := store.NewStore()
	if err := cmdCSVDECODE(discardCtx("", bytesArgs("a,b,c"), s)); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCmdCSVDECODE_WrongArgs(t *testing.T) {
	s := store.NewStore()
	if err := cmdCSVDECODE(discardCtx("", bytesArgs(), s)); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCmdUUIDGEN_Success(t *testing.T) {
	s := store.NewStore()
	if err := cmdUUIDGEN(discardCtx("", bytesArgs(), s)); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCmdUUIDVALIDATE_Success(t *testing.T) {
	s := store.NewStore()
	if err := cmdUUIDVALIDATE(discardCtx("", bytesArgs("550e8400-e29b-41d4-a716-446655440000"), s)); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCmdUUIDVALIDATE_WrongArgs(t *testing.T) {
	s := store.NewStore()
	if err := cmdUUIDVALIDATE(discardCtx("", bytesArgs(), s)); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCmdUUIDVERSION_Success(t *testing.T) {
	s := store.NewStore()
	if err := cmdUUIDVERSION(discardCtx("", bytesArgs("550e8400-e29b-41d4-a716-446655440000"), s)); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCmdUUIDVERSION_WrongArgs(t *testing.T) {
	s := store.NewStore()
	if err := cmdUUIDVERSION(discardCtx("", bytesArgs(), s)); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCmdULIDGEN_Success(t *testing.T) {
	s := store.NewStore()
	if err := cmdULIDGEN(discardCtx("", bytesArgs(), s)); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCmdTIMESTAMPNOW_Success(t *testing.T) {
	s := store.NewStore()
	if err := cmdTIMESTAMPNOW(discardCtx("", bytesArgs(), s)); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCmdTIMESTAMPPARSE_Success(t *testing.T) {
	s := store.NewStore()
	if err := cmdTIMESTAMPPARSE(discardCtx("", bytesArgs("1700000000"), s)); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCmdTIMESTAMPPARSE_WrongArgs(t *testing.T) {
	s := store.NewStore()
	if err := cmdTIMESTAMPPARSE(discardCtx("", bytesArgs(), s)); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCmdTIMESTAMPFORMAT_Success(t *testing.T) {
	s := store.NewStore()
	if err := cmdTIMESTAMPFORMAT(discardCtx("", bytesArgs("1700000000", "2006-01-02"), s)); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCmdTIMESTAMPFORMAT_WrongArgs(t *testing.T) {
	s := store.NewStore()
	if err := cmdTIMESTAMPFORMAT(discardCtx("", bytesArgs("x"), s)); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCmdTIMESTAMPADD_Success(t *testing.T) {
	s := store.NewStore()
	if err := cmdTIMESTAMPADD(discardCtx("", bytesArgs("1700000000", "hours", "5"), s)); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCmdTIMESTAMPADD_WrongArgs(t *testing.T) {
	s := store.NewStore()
	if err := cmdTIMESTAMPADD(discardCtx("", bytesArgs("x", "y"), s)); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCmdTIMESTAMPDIFF_Success(t *testing.T) {
	s := store.NewStore()
	if err := cmdTIMESTAMPDIFF(discardCtx("", bytesArgs("1700000000", "1700003600", "seconds"), s)); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCmdTIMESTAMPDIFF_WrongArgs(t *testing.T) {
	s := store.NewStore()
	if err := cmdTIMESTAMPDIFF(discardCtx("", bytesArgs("x", "y"), s)); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCmdTIMESTAMPSTARTOF_Success(t *testing.T) {
	s := store.NewStore()
	if err := cmdTIMESTAMPSTARTOF(discardCtx("", bytesArgs("1700000000", "day"), s)); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCmdTIMESTAMPSTARTOF_WrongArgs(t *testing.T) {
	s := store.NewStore()
	if err := cmdTIMESTAMPSTARTOF(discardCtx("", bytesArgs("x"), s)); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCmdTIMESTAMPENDOF_Success(t *testing.T) {
	s := store.NewStore()
	if err := cmdTIMESTAMPENDOF(discardCtx("", bytesArgs("1700000000", "day"), s)); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCmdTIMESTAMPENDOF_WrongArgs(t *testing.T) {
	s := store.NewStore()
	if err := cmdTIMESTAMPENDOF(discardCtx("", bytesArgs("x"), s)); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCmdDIFFTEXT_Success(t *testing.T) {
	s := store.NewStore()
	if err := cmdDIFFTEXT(discardCtx("", bytesArgs("hello\nworld", "hello\nfoo"), s)); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCmdDIFFTEXT_WrongArgs(t *testing.T) {
	s := store.NewStore()
	if err := cmdDIFFTEXT(discardCtx("", bytesArgs("x"), s)); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCmdDIFFJSON_Success(t *testing.T) {
	s := store.NewStore()
	if err := cmdDIFFJSON(discardCtx("", bytesArgs(`{"a":"1"}`, `{"a":"2","b":"3"}`), s)); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCmdDIFFJSON_WrongArgs(t *testing.T) {
	s := store.NewStore()
	if err := cmdDIFFJSON(discardCtx("", bytesArgs("x"), s)); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCmdPOOLCREATE_Success(t *testing.T) {
	s := store.NewStore()
	if err := cmdPOOLCREATE(discardCtx("", bytesArgs("pool1", "10"), s)); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCmdPOOLCREATE_WrongArgs(t *testing.T) {
	s := store.NewStore()
	if err := cmdPOOLCREATE(discardCtx("", bytesArgs("x"), s)); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCmdPOOLGET_Success(t *testing.T) {
	s := store.NewStore()
	if err := cmdPOOLGET(discardCtx("", bytesArgs("nonexist_pool"), s)); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCmdPOOLGET_WrongArgs(t *testing.T) {
	s := store.NewStore()
	if err := cmdPOOLGET(discardCtx("", bytesArgs(), s)); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCmdPOOLPUT_Success(t *testing.T) {
	s := store.NewStore()
	cmdPOOLCREATE(discardCtx("", bytesArgs("poolput", "10"), s))
	if err := cmdPOOLPUT(discardCtx("", bytesArgs("poolput", "item1"), s)); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCmdPOOLPUT_WrongArgs(t *testing.T) {
	s := store.NewStore()
	if err := cmdPOOLPUT(discardCtx("", bytesArgs("x"), s)); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

// ===========================================================================
// MVCC COMMANDS
// ===========================================================================

func TestCmdMVCCBEGIN_Success(t *testing.T) {
	s := store.NewStore()
	if err := cmdMVCCBEGIN(discardCtx("", bytesArgs(), s)); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCmdMVCCBeginCommitRollback(t *testing.T) {
	s := store.NewStore()

	// Begin a transaction (returns id)
	ctx1, buf1 := bufCtx("MVCC.BEGIN", bytesArgs(), s)
	if err := cmdMVCCBEGIN(ctx1); err != nil {
		t.Fatalf("MVCC.BEGIN: %v", err)
	}
	// Extract transaction ID from output - it's an integer
	out := buf1.String()
	_ = out // id written

	// Begin another, then set and commit
	ctx2 := discardCtx("MVCC.BEGIN", bytesArgs(), s)
	cmdMVCCBEGIN(ctx2)
	// Get current nextID from the global state
	mvccTxnsMu.RLock()
	var txnID int64
	for id := range mvccTxns {
		txnID = id
	}
	mvccTxnsMu.RUnlock()

	idStr := fmt.Sprintf("%d", txnID)

	// SET within txn
	if err := cmdMVCCSET(discardCtx("", bytesArgs(idStr, "mykey", "myval"), s)); err != nil {
		t.Fatalf("MVCC.SET: %v", err)
	}

	// GET within txn
	if err := cmdMVCCGET(discardCtx("", bytesArgs(idStr, "mykey"), s)); err != nil {
		t.Fatalf("MVCC.GET: %v", err)
	}

	// STATUS
	if err := cmdMVCCSTATUS(discardCtx("", bytesArgs(idStr), s)); err != nil {
		t.Fatalf("MVCC.STATUS: %v", err)
	}

	// COMMIT
	if err := cmdMVCCCOMMIT(discardCtx("", bytesArgs(idStr), s)); err != nil {
		t.Fatalf("MVCC.COMMIT: %v", err)
	}
}

func TestCmdMVCCCOMMIT_WrongArgs(t *testing.T) {
	s := store.NewStore()
	if err := cmdMVCCCOMMIT(discardCtx("", bytesArgs(), s)); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCmdMVCCROLLBACK_WrongArgs(t *testing.T) {
	s := store.NewStore()
	if err := cmdMVCCROLLBACK(discardCtx("", bytesArgs(), s)); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCmdMVCCGET_WrongArgs(t *testing.T) {
	s := store.NewStore()
	if err := cmdMVCCGET(discardCtx("", bytesArgs("1"), s)); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCmdMVCCSET_WrongArgs(t *testing.T) {
	s := store.NewStore()
	if err := cmdMVCCSET(discardCtx("", bytesArgs("1", "key"), s)); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCmdMVCCDELETE_WrongArgs(t *testing.T) {
	s := store.NewStore()
	if err := cmdMVCCDELETE(discardCtx("", bytesArgs("1"), s)); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCmdMVCCSTATUS_WrongArgs(t *testing.T) {
	s := store.NewStore()
	if err := cmdMVCCSTATUS(discardCtx("", bytesArgs(), s)); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCmdMVCCSNAPSHOT_Success(t *testing.T) {
	s := store.NewStore()
	if err := cmdMVCCSNAPSHOT(discardCtx("", bytesArgs(), s)); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCmdSPATIALCREATE_Success(t *testing.T) {
	s := store.NewStore()
	if err := cmdSPATIALCREATE(discardCtx("", bytesArgs("idx1"), s)); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCmdSPATIALCREATE_WrongArgs(t *testing.T) {
	s := store.NewStore()
	if err := cmdSPATIALCREATE(discardCtx("", bytesArgs(), s)); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCmdSPATIALADD_Success(t *testing.T) {
	s := store.NewStore()
	cmdSPATIALCREATE(discardCtx("", bytesArgs("spadd"), s))
	if err := cmdSPATIALADD(discardCtx("", bytesArgs("spadd", "p1", "40.7128", "-74.006"), s)); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCmdSPATIALADD_WrongArgs(t *testing.T) {
	s := store.NewStore()
	if err := cmdSPATIALADD(discardCtx("", bytesArgs("x", "y", "z"), s)); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCmdSPATIALNEARBY_Success(t *testing.T) {
	s := store.NewStore()
	cmdSPATIALCREATE(discardCtx("", bytesArgs("spnb"), s))
	cmdSPATIALADD(discardCtx("", bytesArgs("spnb", "p1", "40.7128", "-74.006"), s))
	if err := cmdSPATIALNEARBY(discardCtx("", bytesArgs("spnb", "40.71", "-74.01", "10"), s)); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCmdSPATIALNEARBY_WrongArgs(t *testing.T) {
	s := store.NewStore()
	if err := cmdSPATIALNEARBY(discardCtx("", bytesArgs("x", "y", "z"), s)); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCmdSPATIALDELETE_Success(t *testing.T) {
	s := store.NewStore()
	cmdSPATIALCREATE(discardCtx("", bytesArgs("spdel"), s))
	cmdSPATIALADD(discardCtx("", bytesArgs("spdel", "p1", "40.7128", "-74.006"), s))
	if err := cmdSPATIALDELETE(discardCtx("", bytesArgs("spdel", "p1"), s)); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCmdSPATIALDELETE_WrongArgs(t *testing.T) {
	s := store.NewStore()
	if err := cmdSPATIALDELETE(discardCtx("", bytesArgs("x"), s)); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCmdSPATIALLIST_Success(t *testing.T) {
	s := store.NewStore()
	if err := cmdSPATIALLIST(discardCtx("", bytesArgs(), s)); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCmdCHAINCREATE_Success(t *testing.T) {
	s := store.NewStore()
	if err := cmdCHAINCREATE(discardCtx("", bytesArgs("chain1"), s)); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCmdCHAINCREATE_WrongArgs(t *testing.T) {
	s := store.NewStore()
	if err := cmdCHAINCREATE(discardCtx("", bytesArgs(), s)); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCmdCHAINADD_Success(t *testing.T) {
	s := store.NewStore()
	cmdCHAINCREATE(discardCtx("", bytesArgs("chainadd"), s))
	if err := cmdCHAINADD(discardCtx("", bytesArgs("chainadd", "block_data"), s)); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCmdCHAINADD_WrongArgs(t *testing.T) {
	s := store.NewStore()
	if err := cmdCHAINADD(discardCtx("", bytesArgs("x"), s)); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCmdCHAINVALIDATE_Success(t *testing.T) {
	s := store.NewStore()
	cmdCHAINCREATE(discardCtx("", bytesArgs("chainval"), s))
	cmdCHAINADD(discardCtx("", bytesArgs("chainval", "block1"), s))
	cmdCHAINADD(discardCtx("", bytesArgs("chainval", "block2"), s))
	if err := cmdCHAINVALIDATE(discardCtx("", bytesArgs("chainval"), s)); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCmdCHAINVALIDATE_WrongArgs(t *testing.T) {
	s := store.NewStore()
	if err := cmdCHAINVALIDATE(discardCtx("", bytesArgs(), s)); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCmdCHAINLENGTH_Success(t *testing.T) {
	s := store.NewStore()
	cmdCHAINCREATE(discardCtx("", bytesArgs("chainlen"), s))
	if err := cmdCHAINLENGTH(discardCtx("", bytesArgs("chainlen"), s)); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCmdCHAINLENGTH_WrongArgs(t *testing.T) {
	s := store.NewStore()
	if err := cmdCHAINLENGTH(discardCtx("", bytesArgs(), s)); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCmdCHAINLAST_Success(t *testing.T) {
	s := store.NewStore()
	cmdCHAINCREATE(discardCtx("", bytesArgs("chainlast"), s))
	cmdCHAINADD(discardCtx("", bytesArgs("chainlast", "block1"), s))
	if err := cmdCHAINLAST(discardCtx("", bytesArgs("chainlast"), s)); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCmdCHAINLAST_WrongArgs(t *testing.T) {
	s := store.NewStore()
	if err := cmdCHAINLAST(discardCtx("", bytesArgs(), s)); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCmdANALYTICSINCR_Success(t *testing.T) {
	s := store.NewStore()
	if err := cmdANALYTICSINCR(discardCtx("", bytesArgs("counter1", "5"), s)); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCmdANALYTICSINCR_WrongArgs(t *testing.T) {
	s := store.NewStore()
	if err := cmdANALYTICSINCR(discardCtx("", bytesArgs("x"), s)); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCmdANALYTICSDECR_Success(t *testing.T) {
	s := store.NewStore()
	if err := cmdANALYTICSDECR(discardCtx("", bytesArgs("counter2", "3"), s)); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCmdANALYTICSDECR_WrongArgs(t *testing.T) {
	s := store.NewStore()
	if err := cmdANALYTICSDECR(discardCtx("", bytesArgs("x"), s)); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCmdANALYTICSGET_Success(t *testing.T) {
	s := store.NewStore()
	cmdANALYTICSINCR(discardCtx("", bytesArgs("aget", "5"), s))
	if err := cmdANALYTICSGET(discardCtx("", bytesArgs("aget"), s)); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCmdANALYTICSGET_WrongArgs(t *testing.T) {
	s := store.NewStore()
	if err := cmdANALYTICSGET(discardCtx("", bytesArgs(), s)); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCmdANALYTICSSUM_Success(t *testing.T) {
	s := store.NewStore()
	cmdANALYTICSINCR(discardCtx("", bytesArgs("asum", "5"), s))
	if err := cmdANALYTICSSUM(discardCtx("", bytesArgs("asum"), s)); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCmdANALYTICSSUM_WrongArgs(t *testing.T) {
	s := store.NewStore()
	if err := cmdANALYTICSSUM(discardCtx("", bytesArgs(), s)); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCmdANALYTICSAVG_Success(t *testing.T) {
	s := store.NewStore()
	cmdANALYTICSINCR(discardCtx("", bytesArgs("aavg", "10"), s))
	if err := cmdANALYTICSAVG(discardCtx("", bytesArgs("aavg"), s)); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCmdANALYTICSAVG_WrongArgs(t *testing.T) {
	s := store.NewStore()
	if err := cmdANALYTICSAVG(discardCtx("", bytesArgs(), s)); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCmdANALYTICSMIN_Success(t *testing.T) {
	s := store.NewStore()
	cmdANALYTICSINCR(discardCtx("", bytesArgs("amin", "3"), s))
	if err := cmdANALYTICSMIN(discardCtx("", bytesArgs("amin"), s)); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCmdANALYTICSMIN_WrongArgs(t *testing.T) {
	s := store.NewStore()
	if err := cmdANALYTICSMIN(discardCtx("", bytesArgs(), s)); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCmdANALYTICSMAX_Success(t *testing.T) {
	s := store.NewStore()
	cmdANALYTICSINCR(discardCtx("", bytesArgs("amax", "7"), s))
	if err := cmdANALYTICSMAX(discardCtx("", bytesArgs("amax"), s)); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCmdANALYTICSMAX_WrongArgs(t *testing.T) {
	s := store.NewStore()
	if err := cmdANALYTICSMAX(discardCtx("", bytesArgs(), s)); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCmdANALYTICSCOUNT_Success(t *testing.T) {
	s := store.NewStore()
	cmdANALYTICSINCR(discardCtx("", bytesArgs("acount", "1"), s))
	if err := cmdANALYTICSCOUNT(discardCtx("", bytesArgs("acount"), s)); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCmdANALYTICSCOUNT_WrongArgs(t *testing.T) {
	s := store.NewStore()
	if err := cmdANALYTICSCOUNT(discardCtx("", bytesArgs(), s)); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCmdANALYTICSCLEAR_Success(t *testing.T) {
	s := store.NewStore()
	cmdANALYTICSINCR(discardCtx("", bytesArgs("aclr", "1"), s))
	if err := cmdANALYTICSCLEAR(discardCtx("", bytesArgs("aclr"), s)); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCmdANALYTICSCLEAR_WrongArgs(t *testing.T) {
	s := store.NewStore()
	if err := cmdANALYTICSCLEAR(discardCtx("", bytesArgs(), s)); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCmdCONNECTIONLIST_Success(t *testing.T) {
	s := store.NewStore()
	if err := cmdCONNECTIONLIST(discardCtx("", bytesArgs(), s)); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCmdCONNECTIONCOUNT_Success(t *testing.T) {
	s := store.NewStore()
	if err := cmdCONNECTIONCOUNT(discardCtx("", bytesArgs(), s)); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCmdCONNECTIONKILL_Success(t *testing.T) {
	s := store.NewStore()
	if err := cmdCONNECTIONKILL(discardCtx("", bytesArgs("999"), s)); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCmdCONNECTIONKILL_WrongArgs(t *testing.T) {
	s := store.NewStore()
	if err := cmdCONNECTIONKILL(discardCtx("", bytesArgs(), s)); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCmdCONNECTIONINFO_Success(t *testing.T) {
	s := store.NewStore()
	if err := cmdCONNECTIONINFO(discardCtx("", bytesArgs("999"), s)); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCmdCONNECTIONINFO_WrongArgs(t *testing.T) {
	s := store.NewStore()
	if err := cmdCONNECTIONINFO(discardCtx("", bytesArgs(), s)); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCmdPLUGINLOAD_Success(t *testing.T) {
	s := store.NewStore()
	if err := cmdPLUGINLOAD(discardCtx("", bytesArgs("myplugin"), s)); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCmdPLUGINLOAD_WrongArgs(t *testing.T) {
	s := store.NewStore()
	if err := cmdPLUGINLOAD(discardCtx("", bytesArgs(), s)); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCmdPLUGINUNLOAD_Success(t *testing.T) {
	s := store.NewStore()
	cmdPLUGINLOAD(discardCtx("", bytesArgs("plgunload"), s))
	if err := cmdPLUGINUNLOAD(discardCtx("", bytesArgs("plgunload"), s)); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCmdPLUGINUNLOAD_WrongArgs(t *testing.T) {
	s := store.NewStore()
	if err := cmdPLUGINUNLOAD(discardCtx("", bytesArgs(), s)); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCmdPLUGINLIST_Success(t *testing.T) {
	s := store.NewStore()
	if err := cmdPLUGINLIST(discardCtx("", bytesArgs(), s)); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCmdPLUGINCALL_Success(t *testing.T) {
	s := store.NewStore()
	cmdPLUGINLOAD(discardCtx("", bytesArgs("plgcall"), s))
	if err := cmdPLUGINCALL(discardCtx("", bytesArgs("plgcall", "myfunc"), s)); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCmdPLUGINCALL_WrongArgs(t *testing.T) {
	s := store.NewStore()
	if err := cmdPLUGINCALL(discardCtx("", bytesArgs("x"), s)); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCmdPLUGININFO_Success(t *testing.T) {
	s := store.NewStore()
	cmdPLUGINLOAD(discardCtx("", bytesArgs("plginfo"), s))
	if err := cmdPLUGININFO(discardCtx("", bytesArgs("plginfo"), s)); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCmdPLUGININFO_WrongArgs(t *testing.T) {
	s := store.NewStore()
	if err := cmdPLUGININFO(discardCtx("", bytesArgs(), s)); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCmdROLLUPCREATE_Success(t *testing.T) {
	s := store.NewStore()
	if err := cmdROLLUPCREATE(discardCtx("", bytesArgs("ru1", "3600"), s)); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCmdROLLUPCREATE_WrongArgs(t *testing.T) {
	s := store.NewStore()
	if err := cmdROLLUPCREATE(discardCtx("", bytesArgs("x"), s)); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCmdROLLUPADD_Success(t *testing.T) {
	s := store.NewStore()
	cmdROLLUPCREATE(discardCtx("", bytesArgs("ruadd", "3600"), s))
	if err := cmdROLLUPADD(discardCtx("", bytesArgs("ruadd", "1000", "5.0"), s)); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCmdROLLUPADD_WrongArgs(t *testing.T) {
	s := store.NewStore()
	if err := cmdROLLUPADD(discardCtx("", bytesArgs("x", "y"), s)); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCmdROLLUPGET_Success(t *testing.T) {
	s := store.NewStore()
	cmdROLLUPCREATE(discardCtx("", bytesArgs("ruget", "3600"), s))
	if err := cmdROLLUPGET(discardCtx("", bytesArgs("ruget", "1000"), s)); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCmdROLLUPGET_WrongArgs(t *testing.T) {
	s := store.NewStore()
	if err := cmdROLLUPGET(discardCtx("", bytesArgs("x"), s)); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCmdROLLUPDELETE_Success(t *testing.T) {
	s := store.NewStore()
	cmdROLLUPCREATE(discardCtx("", bytesArgs("rudel", "3600"), s))
	if err := cmdROLLUPDELETE(discardCtx("", bytesArgs("rudel"), s)); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCmdROLLUPDELETE_WrongArgs(t *testing.T) {
	s := store.NewStore()
	if err := cmdROLLUPDELETE(discardCtx("", bytesArgs(), s)); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCmdCOOLDOWNSET_Success(t *testing.T) {
	s := store.NewStore()
	if err := cmdCOOLDOWNSET(discardCtx("", bytesArgs("cd1", "5000"), s)); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCmdCOOLDOWNSET_WrongArgs(t *testing.T) {
	s := store.NewStore()
	if err := cmdCOOLDOWNSET(discardCtx("", bytesArgs("x"), s)); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCmdCOOLDOWNCHECK_Success(t *testing.T) {
	s := store.NewStore()
	cmdCOOLDOWNSET(discardCtx("", bytesArgs("cdchk", "5000"), s))
	if err := cmdCOOLDOWNCHECK(discardCtx("", bytesArgs("cdchk"), s)); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCmdCOOLDOWNCHECK_WrongArgs(t *testing.T) {
	s := store.NewStore()
	if err := cmdCOOLDOWNCHECK(discardCtx("", bytesArgs(), s)); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCmdCOOLDOWNRESET_Success(t *testing.T) {
	s := store.NewStore()
	cmdCOOLDOWNSET(discardCtx("", bytesArgs("cdrst", "5000"), s))
	if err := cmdCOOLDOWNRESET(discardCtx("", bytesArgs("cdrst"), s)); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCmdCOOLDOWNRESET_WrongArgs(t *testing.T) {
	s := store.NewStore()
	if err := cmdCOOLDOWNRESET(discardCtx("", bytesArgs(), s)); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCmdCOOLDOWNDELETE_Success(t *testing.T) {
	s := store.NewStore()
	cmdCOOLDOWNSET(discardCtx("", bytesArgs("cddel", "5000"), s))
	if err := cmdCOOLDOWNDELETE(discardCtx("", bytesArgs("cddel"), s)); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCmdCOOLDOWNDELETE_WrongArgs(t *testing.T) {
	s := store.NewStore()
	if err := cmdCOOLDOWNDELETE(discardCtx("", bytesArgs(), s)); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCmdCOOLDOWNLIST_Success(t *testing.T) {
	s := store.NewStore()
	if err := cmdCOOLDOWNLIST(discardCtx("", bytesArgs(), s)); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCmdQUOTASET_Success(t *testing.T) {
	s := store.NewStore()
	if err := cmdQUOTASET(discardCtx("", bytesArgs("q1", "100", "60000"), s)); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCmdQUOTASET_WrongArgs(t *testing.T) {
	s := store.NewStore()
	if err := cmdQUOTASET(discardCtx("", bytesArgs("x", "y"), s)); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCmdQUOTACHECK_Success(t *testing.T) {
	s := store.NewStore()
	cmdQUOTASET(discardCtx("", bytesArgs("qchk", "100", "60000"), s))
	if err := cmdQUOTACHECK(discardCtx("", bytesArgs("qchk"), s)); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCmdQUOTACHECK_WrongArgs(t *testing.T) {
	s := store.NewStore()
	if err := cmdQUOTACHECK(discardCtx("", bytesArgs(), s)); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

// ===========================================================================
// EVENT COMMANDS
// ===========================================================================

func TestCmdEVENTEMIT_Success(t *testing.T) {
	s := store.NewStore()
	if err := cmdEVENTEMIT(discardCtx("", bytesArgs("user.created", "name", "alice"), s)); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCmdEVENTEMIT_WrongArgs(t *testing.T) {
	s := store.NewStore()
	if err := cmdEVENTEMIT(discardCtx("", bytesArgs(), s)); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCmdEVENTGET_Success(t *testing.T) {
	s := store.NewStore()
	cmdEVENTEMIT(discardCtx("", bytesArgs("test.event"), s))
	if err := cmdEVENTGET(discardCtx("", bytesArgs("test.event"), s)); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCmdEVENTGET_WrongArgs(t *testing.T) {
	s := store.NewStore()
	if err := cmdEVENTGET(discardCtx("", bytesArgs(), s)); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCmdEVENTLIST_Success(t *testing.T) {
	s := store.NewStore()
	if err := cmdEVENTLIST(discardCtx("", bytesArgs(), s)); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCmdEVENTCLEAR_Success(t *testing.T) {
	s := store.NewStore()
	if err := cmdEVENTCLEAR(discardCtx("", bytesArgs(), s)); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCmdWEBHOOKCREATE_Success(t *testing.T) {
	s := store.NewStore()
	if err := cmdWEBHOOKCREATE(discardCtx("", bytesArgs("wh1", "http://example.com", "POST"), s)); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCmdWEBHOOKCREATE_WrongArgs(t *testing.T) {
	s := store.NewStore()
	if err := cmdWEBHOOKCREATE(discardCtx("", bytesArgs("wh1", "url"), s)); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCmdWEBHOOKDELETE_Success(t *testing.T) {
	s := store.NewStore()
	cmdWEBHOOKCREATE(discardCtx("", bytesArgs("whdel", "http://example.com", "POST"), s))
	if err := cmdWEBHOOKDELETE(discardCtx("", bytesArgs("whdel"), s)); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCmdWEBHOOKDELETE_WrongArgs(t *testing.T) {
	s := store.NewStore()
	if err := cmdWEBHOOKDELETE(discardCtx("", bytesArgs(), s)); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCmdWEBHOOKGET_Success(t *testing.T) {
	s := store.NewStore()
	cmdWEBHOOKCREATE(discardCtx("", bytesArgs("whget", "http://example.com", "POST"), s))
	if err := cmdWEBHOOKGET(discardCtx("", bytesArgs("whget"), s)); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCmdWEBHOOKGET_WrongArgs(t *testing.T) {
	s := store.NewStore()
	if err := cmdWEBHOOKGET(discardCtx("", bytesArgs(), s)); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCmdWEBHOOKLIST_Success(t *testing.T) {
	s := store.NewStore()
	if err := cmdWEBHOOKLIST(discardCtx("", bytesArgs(), s)); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCmdWEBHOOKENABLE_Success(t *testing.T) {
	s := store.NewStore()
	cmdWEBHOOKCREATE(discardCtx("", bytesArgs("when", "http://example.com", "POST"), s))
	if err := cmdWEBHOOKENABLE(discardCtx("", bytesArgs("when"), s)); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCmdWEBHOOKENABLE_WrongArgs(t *testing.T) {
	s := store.NewStore()
	if err := cmdWEBHOOKENABLE(discardCtx("", bytesArgs(), s)); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCmdWEBHOOKDISABLE_Success(t *testing.T) {
	s := store.NewStore()
	cmdWEBHOOKCREATE(discardCtx("", bytesArgs("whdis", "http://example.com", "POST"), s))
	if err := cmdWEBHOOKDISABLE(discardCtx("", bytesArgs("whdis"), s)); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCmdWEBHOOKDISABLE_WrongArgs(t *testing.T) {
	s := store.NewStore()
	if err := cmdWEBHOOKDISABLE(discardCtx("", bytesArgs(), s)); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCmdWEBHOOKSTATS_Success(t *testing.T) {
	s := store.NewStore()
	cmdWEBHOOKCREATE(discardCtx("", bytesArgs("whstats", "http://example.com", "POST"), s))
	if err := cmdWEBHOOKSTATS(discardCtx("", bytesArgs("whstats"), s)); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCmdWEBHOOKSTATS_WrongArgs(t *testing.T) {
	s := store.NewStore()
	if err := cmdWEBHOOKSTATS(discardCtx("", bytesArgs(), s)); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCmdCOMPRESSRLE_Success(t *testing.T) {
	s := store.NewStore()
	if err := cmdCOMPRESSRLE(discardCtx("", bytesArgs("aaabbbccc"), s)); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCmdCOMPRESSRLE_WrongArgs(t *testing.T) {
	s := store.NewStore()
	if err := cmdCOMPRESSRLE(discardCtx("", bytesArgs(), s)); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCmdCOMPRESSLZ4_Success(t *testing.T) {
	s := store.NewStore()
	if err := cmdCOMPRESSLZ4(discardCtx("", bytesArgs("hello world hello world"), s)); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCmdCOMPRESSLZ4_WrongArgs(t *testing.T) {
	s := store.NewStore()
	if err := cmdCOMPRESSLZ4(discardCtx("", bytesArgs(), s)); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCmdCOMPRESSCUSTOM_Success(t *testing.T) {
	s := store.NewStore()
	if err := cmdCOMPRESSCUSTOM(discardCtx("", bytesArgs("RLE", "aabbcc"), s)); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCmdCOMPRESSCUSTOM_WrongArgs(t *testing.T) {
	s := store.NewStore()
	if err := cmdCOMPRESSCUSTOM(discardCtx("", bytesArgs("x"), s)); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCmdQUEUECREATE_Success(t *testing.T) {
	s := store.NewStore()
	if err := cmdQUEUECREATE(discardCtx("", bytesArgs("q1"), s)); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCmdQUEUECREATE_WrongArgs(t *testing.T) {
	s := store.NewStore()
	if err := cmdQUEUECREATE(discardCtx("", bytesArgs(), s)); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCmdQUEUEPUSH_Success(t *testing.T) {
	s := store.NewStore()
	cmdQUEUECREATE(discardCtx("", bytesArgs("qpush"), s))
	if err := cmdQUEUEPUSH(discardCtx("", bytesArgs("qpush", "item1"), s)); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCmdQUEUEPUSH_WrongArgs(t *testing.T) {
	s := store.NewStore()
	if err := cmdQUEUEPUSH(discardCtx("", bytesArgs("x"), s)); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCmdQUEUEPOP_Success(t *testing.T) {
	s := store.NewStore()
	cmdQUEUECREATE(discardCtx("", bytesArgs("qpop"), s))
	cmdQUEUEPUSH(discardCtx("", bytesArgs("qpop", "item1"), s))
	if err := cmdQUEUEPOP(discardCtx("", bytesArgs("qpop"), s)); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCmdQUEUEPOP_WrongArgs(t *testing.T) {
	s := store.NewStore()
	if err := cmdQUEUEPOP(discardCtx("", bytesArgs(), s)); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCmdQUEUEPEEK_Success(t *testing.T) {
	s := store.NewStore()
	cmdQUEUECREATE(discardCtx("", bytesArgs("qpeek"), s))
	cmdQUEUEPUSH(discardCtx("", bytesArgs("qpeek", "item1"), s))
	if err := cmdQUEUEPEEK(discardCtx("", bytesArgs("qpeek"), s)); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCmdQUEUEPEEK_WrongArgs(t *testing.T) {
	s := store.NewStore()
	if err := cmdQUEUEPEEK(discardCtx("", bytesArgs(), s)); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCmdQUEUELEN_Success(t *testing.T) {
	s := store.NewStore()
	cmdQUEUECREATE(discardCtx("", bytesArgs("qlen"), s))
	if err := cmdQUEUELEN(discardCtx("", bytesArgs("qlen"), s)); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCmdQUEUELEN_WrongArgs(t *testing.T) {
	s := store.NewStore()
	if err := cmdQUEUELEN(discardCtx("", bytesArgs(), s)); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCmdQUEUECLEAR_Success(t *testing.T) {
	s := store.NewStore()
	cmdQUEUECREATE(discardCtx("", bytesArgs("qclr"), s))
	if err := cmdQUEUECLEAR(discardCtx("", bytesArgs("qclr"), s)); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCmdQUEUECLEAR_WrongArgs(t *testing.T) {
	s := store.NewStore()
	if err := cmdQUEUECLEAR(discardCtx("", bytesArgs(), s)); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCmdSTACKCREATE_Success(t *testing.T) {
	s := store.NewStore()
	if err := cmdSTACKCREATE(discardCtx("", bytesArgs("stk1"), s)); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCmdSTACKCREATE_WrongArgs(t *testing.T) {
	s := store.NewStore()
	if err := cmdSTACKCREATE(discardCtx("", bytesArgs(), s)); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCmdSTACKPUSH_Success(t *testing.T) {
	s := store.NewStore()
	cmdSTACKCREATE(discardCtx("", bytesArgs("stkpush"), s))
	if err := cmdSTACKPUSH(discardCtx("", bytesArgs("stkpush", "item1"), s)); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCmdSTACKPUSH_WrongArgs(t *testing.T) {
	s := store.NewStore()
	if err := cmdSTACKPUSH(discardCtx("", bytesArgs("x"), s)); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCmdSTACKPOP_Success(t *testing.T) {
	s := store.NewStore()
	cmdSTACKCREATE(discardCtx("", bytesArgs("stkpop"), s))
	cmdSTACKPUSH(discardCtx("", bytesArgs("stkpop", "item1"), s))
	if err := cmdSTACKPOP(discardCtx("", bytesArgs("stkpop"), s)); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCmdSTACKPOP_WrongArgs(t *testing.T) {
	s := store.NewStore()
	if err := cmdSTACKPOP(discardCtx("", bytesArgs(), s)); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCmdSTACKPEEK_Success(t *testing.T) {
	s := store.NewStore()
	cmdSTACKCREATE(discardCtx("", bytesArgs("stkpeek"), s))
	cmdSTACKPUSH(discardCtx("", bytesArgs("stkpeek", "item1"), s))
	if err := cmdSTACKPEEK(discardCtx("", bytesArgs("stkpeek"), s)); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCmdSTACKPEEK_WrongArgs(t *testing.T) {
	s := store.NewStore()
	if err := cmdSTACKPEEK(discardCtx("", bytesArgs(), s)); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCmdSTACKLEN_Success(t *testing.T) {
	s := store.NewStore()
	cmdSTACKCREATE(discardCtx("", bytesArgs("stklen"), s))
	if err := cmdSTACKLEN(discardCtx("", bytesArgs("stklen"), s)); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCmdSTACKLEN_WrongArgs(t *testing.T) {
	s := store.NewStore()
	if err := cmdSTACKLEN(discardCtx("", bytesArgs(), s)); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCmdSTACKCLEAR_Success(t *testing.T) {
	s := store.NewStore()
	cmdSTACKCREATE(discardCtx("", bytesArgs("stkclr"), s))
	if err := cmdSTACKCLEAR(discardCtx("", bytesArgs("stkclr"), s)); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCmdSTACKCLEAR_WrongArgs(t *testing.T) {
	s := store.NewStore()
	if err := cmdSTACKCLEAR(discardCtx("", bytesArgs(), s)); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

// ===========================================================================
// EXTENDED COMMANDS
// ===========================================================================

func TestCmdMSGQUEUECREATE_Success(t *testing.T) {
	s := store.NewStore()
	if err := cmdMSGQUEUECREATE(discardCtx("", bytesArgs("mq1"), s)); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCmdMSGQUEUECREATE_WrongArgs(t *testing.T) {
	s := store.NewStore()
	if err := cmdMSGQUEUECREATE(discardCtx("", bytesArgs(), s)); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCmdMSGQUEUEPUBLISH_Success(t *testing.T) {
	s := store.NewStore()
	cmdMSGQUEUECREATE(discardCtx("", bytesArgs("mqpub"), s))
	if err := cmdMSGQUEUEPUBLISH(discardCtx("", bytesArgs("mqpub", "hello"), s)); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCmdMSGQUEUEPUBLISH_WrongArgs(t *testing.T) {
	s := store.NewStore()
	if err := cmdMSGQUEUEPUBLISH(discardCtx("", bytesArgs("x"), s)); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCmdMSGQUEUECONSUME_Success(t *testing.T) {
	s := store.NewStore()
	cmdMSGQUEUECREATE(discardCtx("", bytesArgs("mqcon"), s))
	cmdMSGQUEUEPUBLISH(discardCtx("", bytesArgs("mqcon", "hello"), s))
	if err := cmdMSGQUEUECONSUME(discardCtx("", bytesArgs("mqcon"), s)); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCmdMSGQUEUECONSUME_WrongArgs(t *testing.T) {
	s := store.NewStore()
	if err := cmdMSGQUEUECONSUME(discardCtx("", bytesArgs(), s)); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCmdMSGQUEUESTATS_Success(t *testing.T) {
	s := store.NewStore()
	cmdMSGQUEUECREATE(discardCtx("", bytesArgs("mqst"), s))
	if err := cmdMSGQUEUESTATS(discardCtx("", bytesArgs("mqst"), s)); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCmdMSGQUEUESTATS_WrongArgs(t *testing.T) {
	s := store.NewStore()
	if err := cmdMSGQUEUESTATS(discardCtx("", bytesArgs(), s)); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCmdMSGQUEUEPURGE_Success(t *testing.T) {
	s := store.NewStore()
	cmdMSGQUEUECREATE(discardCtx("", bytesArgs("mqpur"), s))
	if err := cmdMSGQUEUEPURGE(discardCtx("", bytesArgs("mqpur"), s)); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCmdMSGQUEUEPURGE_WrongArgs(t *testing.T) {
	s := store.NewStore()
	if err := cmdMSGQUEUEPURGE(discardCtx("", bytesArgs(), s)); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCmdMSGQUEUEDELETE_Success(t *testing.T) {
	s := store.NewStore()
	cmdMSGQUEUECREATE(discardCtx("", bytesArgs("mqdel"), s))
	if err := cmdMSGQUEUEDELETE(discardCtx("", bytesArgs("mqdel"), s)); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCmdMSGQUEUEDELETE_WrongArgs(t *testing.T) {
	s := store.NewStore()
	if err := cmdMSGQUEUEDELETE(discardCtx("", bytesArgs(), s)); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCmdMSGQUEUEDEADLETTER_Success(t *testing.T) {
	s := store.NewStore()
	cmdMSGQUEUECREATE(discardCtx("", bytesArgs("mqdl"), s))
	if err := cmdMSGQUEUEDEADLETTER(discardCtx("", bytesArgs("mqdl"), s)); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCmdMSGQUEUEDEADLETTER_WrongArgs(t *testing.T) {
	s := store.NewStore()
	if err := cmdMSGQUEUEDEADLETTER(discardCtx("", bytesArgs(), s)); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCmdSERVICEREGISTER_Success(t *testing.T) {
	s := store.NewStore()
	if err := cmdSERVICEREGISTER(discardCtx("", bytesArgs("svc1", "id1", "127.0.0.1", "8080"), s)); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCmdSERVICEREGISTER_WrongArgs(t *testing.T) {
	s := store.NewStore()
	if err := cmdSERVICEREGISTER(discardCtx("", bytesArgs("svc1", "id1", "addr"), s)); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCmdSERVICEDEREGISTER_Success(t *testing.T) {
	s := store.NewStore()
	cmdSERVICEREGISTER(discardCtx("", bytesArgs("svcdel", "id1", "127.0.0.1", "8080"), s))
	if err := cmdSERVICEDEREGISTER(discardCtx("", bytesArgs("svcdel", "id1"), s)); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCmdSERVICEDEREGISTER_WrongArgs(t *testing.T) {
	s := store.NewStore()
	if err := cmdSERVICEDEREGISTER(discardCtx("", bytesArgs("x"), s)); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCmdSERVICEDISCOVER_Success(t *testing.T) {
	s := store.NewStore()
	cmdSERVICEREGISTER(discardCtx("", bytesArgs("svcdis", "id1", "127.0.0.1", "8080"), s))
	if err := cmdSERVICEDISCOVER(discardCtx("", bytesArgs("svcdis"), s)); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCmdSERVICEDISCOVER_WrongArgs(t *testing.T) {
	s := store.NewStore()
	if err := cmdSERVICEDISCOVER(discardCtx("", bytesArgs(), s)); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCmdSERVICEHEARTBEAT_Success(t *testing.T) {
	s := store.NewStore()
	cmdSERVICEREGISTER(discardCtx("", bytesArgs("svchb", "id1", "127.0.0.1", "8080"), s))
	if err := cmdSERVICEHEARTBEAT(discardCtx("", bytesArgs("svchb", "id1"), s)); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCmdSERVICEHEARTBEAT_WrongArgs(t *testing.T) {
	s := store.NewStore()
	if err := cmdSERVICEHEARTBEAT(discardCtx("", bytesArgs("x"), s)); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCmdSERVICELIST_Success(t *testing.T) {
	s := store.NewStore()
	if err := cmdSERVICELIST(discardCtx("", bytesArgs(), s)); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCmdSERVICEHEALTHY_Success(t *testing.T) {
	s := store.NewStore()
	cmdSERVICEREGISTER(discardCtx("", bytesArgs("svch", "id1", "127.0.0.1", "8080"), s))
	if err := cmdSERVICEHEALTHY(discardCtx("", bytesArgs("svch"), s)); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCmdSERVICEHEALTHY_WrongArgs(t *testing.T) {
	s := store.NewStore()
	if err := cmdSERVICEHEALTHY(discardCtx("", bytesArgs(), s)); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

// ===========================================================================
// Register functions (ensure they don't panic)
// ===========================================================================

func TestRegisterResilienceCommands(t *testing.T) {
	router := NewRouter()
	RegisterResilienceCommands(router)
}

func TestRegisterIntegrationCommands(t *testing.T) {
	router := NewRouter()
	RegisterIntegrationCommands(router)
}

func TestRegisterExtendedCommands(t *testing.T) {
	router := NewRouter()
	RegisterExtendedCommands(router)
}

func TestRegisterEncodingCommands(t *testing.T) {
	router := NewRouter()
	RegisterEncodingCommands(router)
}

func TestRegisterMVCCCommands(t *testing.T) {
	router := NewRouter()
	RegisterMVCCCommands(router)
}

func TestRegisterEventCommands(t *testing.T) {
	router := NewRouter()
	RegisterEventCommands(router)
}
