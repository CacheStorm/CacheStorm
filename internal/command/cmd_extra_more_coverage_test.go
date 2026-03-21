package command

import (
	"testing"

	"github.com/cachestorm/cachestorm/internal/store"
)

// ======================================================================
// EXTRA COMMANDS (extra_commands.go)
// ======================================================================

// --- SWIM ---

func TestCmdSWIMJOIN_Success(t *testing.T) {
	s := store.NewStore()
	ctx := discardCtx("SWIM.JOIN", bytesArgs("node1", "127.0.0.1:7000"), s)
	if err := cmdSWIMJOIN(ctx); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCmdSWIMJOIN_WrongArgs(t *testing.T) {
	s := store.NewStore()
	ctx := discardCtx("SWIM.JOIN", bytesArgs("node1"), s)
	if err := cmdSWIMJOIN(ctx); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCmdSWIMLEAVE_Success(t *testing.T) {
	s := store.NewStore()
	ctx := discardCtx("SWIM.JOIN", bytesArgs("nodelv", "127.0.0.1:7000"), s)
	_ = cmdSWIMJOIN(ctx)
	ctx2 := discardCtx("SWIM.LEAVE", bytesArgs("nodelv"), s)
	if err := cmdSWIMLEAVE(ctx2); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCmdSWIMLEAVE_WrongArgs(t *testing.T) {
	s := store.NewStore()
	ctx := discardCtx("SWIM.LEAVE", bytesArgs(), s)
	if err := cmdSWIMLEAVE(ctx); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCmdSWIMLEAVE_NotFound(t *testing.T) {
	s := store.NewStore()
	ctx := discardCtx("SWIM.LEAVE", bytesArgs("nonexistent"), s)
	if err := cmdSWIMLEAVE(ctx); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCmdSWIMMEMBERS_Success(t *testing.T) {
	s := store.NewStore()
	ctx := discardCtx("SWIM.MEMBERS", bytesArgs(), s)
	if err := cmdSWIMMEMBERS(ctx); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCmdSWIMPING_Success(t *testing.T) {
	s := store.NewStore()
	join := discardCtx("SWIM.JOIN", bytesArgs("nodep", "127.0.0.1:7001"), s)
	_ = cmdSWIMJOIN(join)
	ctx := discardCtx("SWIM.PING", bytesArgs("nodep"), s)
	if err := cmdSWIMPING(ctx); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCmdSWIMPING_WrongArgs(t *testing.T) {
	s := store.NewStore()
	ctx := discardCtx("SWIM.PING", bytesArgs(), s)
	if err := cmdSWIMPING(ctx); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCmdSWIMPING_NotFound(t *testing.T) {
	s := store.NewStore()
	ctx := discardCtx("SWIM.PING", bytesArgs("missing"), s)
	if err := cmdSWIMPING(ctx); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCmdSWIMSUSPECT_Success(t *testing.T) {
	s := store.NewStore()
	join := discardCtx("SWIM.JOIN", bytesArgs("nodes", "127.0.0.1:7002"), s)
	_ = cmdSWIMJOIN(join)
	ctx := discardCtx("SWIM.SUSPECT", bytesArgs("nodes"), s)
	if err := cmdSWIMSUSPECT(ctx); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCmdSWIMSUSPECT_WrongArgs(t *testing.T) {
	s := store.NewStore()
	ctx := discardCtx("SWIM.SUSPECT", bytesArgs(), s)
	if err := cmdSWIMSUSPECT(ctx); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCmdSWIMSUSPECT_NotFound(t *testing.T) {
	s := store.NewStore()
	ctx := discardCtx("SWIM.SUSPECT", bytesArgs("missing"), s)
	if err := cmdSWIMSUSPECT(ctx); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

// --- GOSSIP ---

func TestCmdGOSSIPJOIN_Success(t *testing.T) {
	s := store.NewStore()
	ctx := discardCtx("GOSSIP.JOIN", bytesArgs("gnode1", "10.0.0.1:5000"), s)
	if err := cmdGOSSIPJOIN(ctx); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCmdGOSSIPJOIN_WrongArgs(t *testing.T) {
	s := store.NewStore()
	ctx := discardCtx("GOSSIP.JOIN", bytesArgs("gnode1"), s)
	if err := cmdGOSSIPJOIN(ctx); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCmdGOSSIPLEAVE_Success(t *testing.T) {
	s := store.NewStore()
	ctx := discardCtx("GOSSIP.LEAVE", bytesArgs("gnode1"), s)
	if err := cmdGOSSIPLEAVE(ctx); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCmdGOSSIPLEAVE_WrongArgs(t *testing.T) {
	s := store.NewStore()
	ctx := discardCtx("GOSSIP.LEAVE", bytesArgs(), s)
	if err := cmdGOSSIPLEAVE(ctx); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCmdGOSSIPBROADCAST_Success(t *testing.T) {
	s := store.NewStore()
	ctx := discardCtx("GOSSIP.BROADCAST", bytesArgs("bkey", "bval"), s)
	if err := cmdGOSSIPBROADCAST(ctx); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCmdGOSSIPBROADCAST_WrongArgs(t *testing.T) {
	s := store.NewStore()
	ctx := discardCtx("GOSSIP.BROADCAST", bytesArgs("bkey"), s)
	if err := cmdGOSSIPBROADCAST(ctx); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCmdGOSSIPGET_Success(t *testing.T) {
	s := store.NewStore()
	_ = cmdGOSSIPBROADCAST(discardCtx("GOSSIP.BROADCAST", bytesArgs("ggetk", "ggetv"), s))
	ctx := discardCtx("GOSSIP.GET", bytesArgs("ggetk"), s)
	if err := cmdGOSSIPGET(ctx); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCmdGOSSIPGET_WrongArgs(t *testing.T) {
	s := store.NewStore()
	ctx := discardCtx("GOSSIP.GET", bytesArgs(), s)
	if err := cmdGOSSIPGET(ctx); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCmdGOSSIPGET_NotFound(t *testing.T) {
	s := store.NewStore()
	ctx := discardCtx("GOSSIP.GET", bytesArgs("nosuchkey"), s)
	if err := cmdGOSSIPGET(ctx); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCmdGOSSIPMEMBERS_Success(t *testing.T) {
	s := store.NewStore()
	ctx := discardCtx("GOSSIP.MEMBERS", bytesArgs(), s)
	if err := cmdGOSSIPMEMBERS(ctx); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

// --- ANTI ENTROPY ---

func TestCmdANTIENTROPYSYNC_Success(t *testing.T) {
	s := store.NewStore()
	ctx := discardCtx("ANTI_ENTROPY.SYNC", bytesArgs("aesync1", "1"), s)
	if err := cmdANTIENTROPYSYNC(ctx); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCmdANTIENTROPYSYNC_WrongArgs(t *testing.T) {
	s := store.NewStore()
	ctx := discardCtx("ANTI_ENTROPY.SYNC", bytesArgs("aesync1"), s)
	if err := cmdANTIENTROPYSYNC(ctx); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCmdANTIENTROPYDIFF_Success(t *testing.T) {
	s := store.NewStore()
	_ = cmdANTIENTROPYSYNC(discardCtx("", bytesArgs("aediff1", "5"), s))
	ctx := discardCtx("ANTI_ENTROPY.DIFF", bytesArgs("aediff1", "3"), s)
	if err := cmdANTIENTROPYDIFF(ctx); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCmdANTIENTROPYDIFF_WrongArgs(t *testing.T) {
	s := store.NewStore()
	ctx := discardCtx("ANTI_ENTROPY.DIFF", bytesArgs("x"), s)
	if err := cmdANTIENTROPYDIFF(ctx); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCmdANTIENTROPYDIFF_NotFound(t *testing.T) {
	s := store.NewStore()
	ctx := discardCtx("ANTI_ENTROPY.DIFF", bytesArgs("nonexist", "1"), s)
	if err := cmdANTIENTROPYDIFF(ctx); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCmdANTIENTROPYMERGE_Success(t *testing.T) {
	s := store.NewStore()
	_ = cmdANTIENTROPYSYNC(discardCtx("", bytesArgs("aemerge1", "2"), s))
	ctx := discardCtx("ANTI_ENTROPY.MERGE", bytesArgs("aemerge1", "5"), s)
	if err := cmdANTIENTROPYMERGE(ctx); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCmdANTIENTROPYMERGE_WrongArgs(t *testing.T) {
	s := store.NewStore()
	ctx := discardCtx("ANTI_ENTROPY.MERGE", bytesArgs("x"), s)
	if err := cmdANTIENTROPYMERGE(ctx); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCmdANTIENTROPYMERGE_NewEntry(t *testing.T) {
	s := store.NewStore()
	ctx := discardCtx("ANTI_ENTROPY.MERGE", bytesArgs("aenewmerge", "1"), s)
	if err := cmdANTIENTROPYMERGE(ctx); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCmdANTIENTROPYSTATUS_Success(t *testing.T) {
	s := store.NewStore()
	_ = cmdANTIENTROPYSYNC(discardCtx("", bytesArgs("aestat1", "1"), s))
	ctx := discardCtx("ANTI_ENTROPY.STATUS", bytesArgs("aestat1"), s)
	if err := cmdANTIENTROPYSTATUS(ctx); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCmdANTIENTROPYSTATUS_WrongArgs(t *testing.T) {
	s := store.NewStore()
	ctx := discardCtx("ANTI_ENTROPY.STATUS", bytesArgs(), s)
	if err := cmdANTIENTROPYSTATUS(ctx); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCmdANTIENTROPYSTATUS_NotFound(t *testing.T) {
	s := store.NewStore()
	ctx := discardCtx("ANTI_ENTROPY.STATUS", bytesArgs("missing"), s)
	if err := cmdANTIENTROPYSTATUS(ctx); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

// --- VECTOR CLOCK ---

func TestCmdVECTORCLOCKCREATE_Success(t *testing.T) {
	s := store.NewStore()
	ctx := discardCtx("VECTOR_CLOCK.CREATE", bytesArgs("vc1", "nodeA"), s)
	if err := cmdVECTORCLOCKCREATE(ctx); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCmdVECTORCLOCKCREATE_WrongArgs(t *testing.T) {
	s := store.NewStore()
	ctx := discardCtx("VECTOR_CLOCK.CREATE", bytesArgs("vc1"), s)
	if err := cmdVECTORCLOCKCREATE(ctx); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCmdVECTORCLOCKINCREMENT_Success(t *testing.T) {
	s := store.NewStore()
	_ = cmdVECTORCLOCKCREATE(discardCtx("", bytesArgs("vcinc", "nodeA"), s))
	ctx := discardCtx("VECTOR_CLOCK.INCREMENT", bytesArgs("vcinc", "nodeA"), s)
	if err := cmdVECTORCLOCKINCREMENT(ctx); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCmdVECTORCLOCKINCREMENT_WrongArgs(t *testing.T) {
	s := store.NewStore()
	ctx := discardCtx("VECTOR_CLOCK.INCREMENT", bytesArgs("x"), s)
	if err := cmdVECTORCLOCKINCREMENT(ctx); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCmdVECTORCLOCKINCREMENT_NewClock(t *testing.T) {
	s := store.NewStore()
	ctx := discardCtx("VECTOR_CLOCK.INCREMENT", bytesArgs("vcnew123", "nodeZ"), s)
	if err := cmdVECTORCLOCKINCREMENT(ctx); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCmdVECTORCLOCKCOMPARE_Success(t *testing.T) {
	s := store.NewStore()
	_ = cmdVECTORCLOCKCREATE(discardCtx("", bytesArgs("vcc1", "a"), s))
	_ = cmdVECTORCLOCKCREATE(discardCtx("", bytesArgs("vcc2", "a"), s))
	ctx := discardCtx("VECTOR_CLOCK.COMPARE", bytesArgs("vcc1", "vcc2"), s)
	if err := cmdVECTORCLOCKCOMPARE(ctx); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCmdVECTORCLOCKCOMPARE_WrongArgs(t *testing.T) {
	s := store.NewStore()
	ctx := discardCtx("VECTOR_CLOCK.COMPARE", bytesArgs("x"), s)
	if err := cmdVECTORCLOCKCOMPARE(ctx); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCmdVECTORCLOCKCOMPARE_NotFound(t *testing.T) {
	s := store.NewStore()
	ctx := discardCtx("VECTOR_CLOCK.COMPARE", bytesArgs("miss1", "miss2"), s)
	if err := cmdVECTORCLOCKCOMPARE(ctx); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCmdVECTORCLOCKMERGE_Success(t *testing.T) {
	s := store.NewStore()
	_ = cmdVECTORCLOCKCREATE(discardCtx("", bytesArgs("vcm1", "a"), s))
	_ = cmdVECTORCLOCKCREATE(discardCtx("", bytesArgs("vcm2", "a"), s))
	ctx := discardCtx("VECTOR_CLOCK.MERGE", bytesArgs("vcm1", "vcm2"), s)
	if err := cmdVECTORCLOCKMERGE(ctx); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCmdVECTORCLOCKMERGE_WrongArgs(t *testing.T) {
	s := store.NewStore()
	ctx := discardCtx("VECTOR_CLOCK.MERGE", bytesArgs("x"), s)
	if err := cmdVECTORCLOCKMERGE(ctx); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCmdVECTORCLOCKMERGE_NotFound(t *testing.T) {
	s := store.NewStore()
	ctx := discardCtx("VECTOR_CLOCK.MERGE", bytesArgs("miss1", "miss2"), s)
	if err := cmdVECTORCLOCKMERGE(ctx); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCmdVECTORCLOCKGET_Success(t *testing.T) {
	s := store.NewStore()
	_ = cmdVECTORCLOCKCREATE(discardCtx("", bytesArgs("vcget1", "a"), s))
	ctx := discardCtx("VECTOR_CLOCK.GET", bytesArgs("vcget1"), s)
	if err := cmdVECTORCLOCKGET(ctx); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCmdVECTORCLOCKGET_WrongArgs(t *testing.T) {
	s := store.NewStore()
	ctx := discardCtx("VECTOR_CLOCK.GET", bytesArgs(), s)
	if err := cmdVECTORCLOCKGET(ctx); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCmdVECTORCLOCKGET_NotFound(t *testing.T) {
	s := store.NewStore()
	ctx := discardCtx("VECTOR_CLOCK.GET", bytesArgs("nope"), s)
	if err := cmdVECTORCLOCKGET(ctx); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

// --- CRDT LWW ---

func TestCmdCRDTLWWSET_Success(t *testing.T) {
	s := store.NewStore()
	ctx := discardCtx("CRDT.LWW.SET", bytesArgs("lwwk", "lwwv"), s)
	if err := cmdCRDTLWWSET(ctx); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCmdCRDTLWWSET_WrongArgs(t *testing.T) {
	s := store.NewStore()
	ctx := discardCtx("CRDT.LWW.SET", bytesArgs("lwwk"), s)
	if err := cmdCRDTLWWSET(ctx); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCmdCRDTLWWSET_WithTimestamp(t *testing.T) {
	s := store.NewStore()
	ctx := discardCtx("CRDT.LWW.SET", bytesArgs("lwwts", "val", "999999999999"), s)
	if err := cmdCRDTLWWSET(ctx); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCmdCRDTLWWGET_Success(t *testing.T) {
	s := store.NewStore()
	_ = cmdCRDTLWWSET(discardCtx("", bytesArgs("lwwget1", "val1"), s))
	ctx := discardCtx("CRDT.LWW.GET", bytesArgs("lwwget1"), s)
	if err := cmdCRDTLWWGET(ctx); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCmdCRDTLWWGET_WrongArgs(t *testing.T) {
	s := store.NewStore()
	ctx := discardCtx("CRDT.LWW.GET", bytesArgs(), s)
	if err := cmdCRDTLWWGET(ctx); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCmdCRDTLWWGET_NotFound(t *testing.T) {
	s := store.NewStore()
	ctx := discardCtx("CRDT.LWW.GET", bytesArgs("nosuchkey"), s)
	if err := cmdCRDTLWWGET(ctx); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCmdCRDTLWWDELETE_Success(t *testing.T) {
	s := store.NewStore()
	_ = cmdCRDTLWWSET(discardCtx("", bytesArgs("lwwdel1", "val1"), s))
	ctx := discardCtx("CRDT.LWW.DELETE", bytesArgs("lwwdel1"), s)
	if err := cmdCRDTLWWDELETE(ctx); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCmdCRDTLWWDELETE_WrongArgs(t *testing.T) {
	s := store.NewStore()
	ctx := discardCtx("CRDT.LWW.DELETE", bytesArgs(), s)
	if err := cmdCRDTLWWDELETE(ctx); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCmdCRDTLWWDELETE_NotFound(t *testing.T) {
	s := store.NewStore()
	ctx := discardCtx("CRDT.LWW.DELETE", bytesArgs("nope"), s)
	if err := cmdCRDTLWWDELETE(ctx); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

// --- CRDT GCounter ---

func TestCmdCRDTGCOUNTERINCR_Success(t *testing.T) {
	s := store.NewStore()
	ctx := discardCtx("CRDT.GCOUNTER.INCR", bytesArgs("gc1", "nodeA"), s)
	if err := cmdCRDTGCOUNTERINCR(ctx); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCmdCRDTGCOUNTERINCR_WrongArgs(t *testing.T) {
	s := store.NewStore()
	ctx := discardCtx("CRDT.GCOUNTER.INCR", bytesArgs("gc1"), s)
	if err := cmdCRDTGCOUNTERINCR(ctx); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCmdCRDTGCOUNTERINCR_WithAmount(t *testing.T) {
	s := store.NewStore()
	ctx := discardCtx("CRDT.GCOUNTER.INCR", bytesArgs("gc2", "nodeA", "5"), s)
	if err := cmdCRDTGCOUNTERINCR(ctx); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCmdCRDTGCOUNTERGET_Success(t *testing.T) {
	s := store.NewStore()
	_ = cmdCRDTGCOUNTERINCR(discardCtx("", bytesArgs("gcget1", "nodeA"), s))
	ctx := discardCtx("CRDT.GCOUNTER.GET", bytesArgs("gcget1"), s)
	if err := cmdCRDTGCOUNTERGET(ctx); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCmdCRDTGCOUNTERGET_WrongArgs(t *testing.T) {
	s := store.NewStore()
	ctx := discardCtx("CRDT.GCOUNTER.GET", bytesArgs(), s)
	if err := cmdCRDTGCOUNTERGET(ctx); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCmdCRDTGCOUNTERGET_NotFound(t *testing.T) {
	s := store.NewStore()
	ctx := discardCtx("CRDT.GCOUNTER.GET", bytesArgs("nope"), s)
	if err := cmdCRDTGCOUNTERGET(ctx); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

// --- CRDT PNCounter ---

func TestCmdCRDTPNCOUNTERINCR_Success(t *testing.T) {
	s := store.NewStore()
	ctx := discardCtx("CRDT.PNCounter.INCR", bytesArgs("pnc1", "nodeA"), s)
	if err := cmdCRDTPNCOUNTERINCR(ctx); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCmdCRDTPNCOUNTERINCR_WrongArgs(t *testing.T) {
	s := store.NewStore()
	ctx := discardCtx("CRDT.PNCounter.INCR", bytesArgs("pnc1"), s)
	if err := cmdCRDTPNCOUNTERINCR(ctx); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCmdCRDTPNCOUNTERDECR_Success(t *testing.T) {
	s := store.NewStore()
	ctx := discardCtx("CRDT.PNCounter.DECR", bytesArgs("pnd1", "nodeA"), s)
	if err := cmdCRDTPNCOUNTERDECR(ctx); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCmdCRDTPNCOUNTERDECR_WrongArgs(t *testing.T) {
	s := store.NewStore()
	ctx := discardCtx("CRDT.PNCounter.DECR", bytesArgs("pnd1"), s)
	if err := cmdCRDTPNCOUNTERDECR(ctx); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCmdCRDTPNCOUNTERGET_Success(t *testing.T) {
	s := store.NewStore()
	_ = cmdCRDTPNCOUNTERINCR(discardCtx("", bytesArgs("pnget1", "nodeA"), s))
	ctx := discardCtx("CRDT.PNCounter.GET", bytesArgs("pnget1"), s)
	if err := cmdCRDTPNCOUNTERGET(ctx); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCmdCRDTPNCOUNTERGET_WrongArgs(t *testing.T) {
	s := store.NewStore()
	ctx := discardCtx("CRDT.PNCounter.GET", bytesArgs(), s)
	if err := cmdCRDTPNCOUNTERGET(ctx); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCmdCRDTPNCOUNTERGET_NotFound(t *testing.T) {
	s := store.NewStore()
	ctx := discardCtx("CRDT.PNCounter.GET", bytesArgs("nope"), s)
	if err := cmdCRDTPNCOUNTERGET(ctx); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

// --- CRDT GSet ---

func TestCmdCRDTGSETADD_Success(t *testing.T) {
	s := store.NewStore()
	ctx := discardCtx("CRDT.GSET.ADD", bytesArgs("gs1", "val1"), s)
	if err := cmdCRDTGSETADD(ctx); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCmdCRDTGSETADD_WrongArgs(t *testing.T) {
	s := store.NewStore()
	ctx := discardCtx("CRDT.GSET.ADD", bytesArgs("gs1"), s)
	if err := cmdCRDTGSETADD(ctx); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCmdCRDTGSETADD_Duplicate(t *testing.T) {
	s := store.NewStore()
	_ = cmdCRDTGSETADD(discardCtx("", bytesArgs("gsdup", "val1"), s))
	ctx := discardCtx("CRDT.GSET.ADD", bytesArgs("gsdup", "val1"), s)
	if err := cmdCRDTGSETADD(ctx); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCmdCRDTGSETGET_Success(t *testing.T) {
	s := store.NewStore()
	_ = cmdCRDTGSETADD(discardCtx("", bytesArgs("gsget1", "val1"), s))
	ctx := discardCtx("CRDT.GSET.GET", bytesArgs("gsget1"), s)
	if err := cmdCRDTGSETGET(ctx); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCmdCRDTGSETGET_WrongArgs(t *testing.T) {
	s := store.NewStore()
	ctx := discardCtx("CRDT.GSET.GET", bytesArgs(), s)
	if err := cmdCRDTGSETGET(ctx); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCmdCRDTGSETGET_NotFound(t *testing.T) {
	s := store.NewStore()
	ctx := discardCtx("CRDT.GSET.GET", bytesArgs("nope"), s)
	if err := cmdCRDTGSETGET(ctx); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

// --- CRDT ORSet ---

func TestCmdCRDTORSETADD_Success(t *testing.T) {
	s := store.NewStore()
	ctx := discardCtx("CRDT.ORSET.ADD", bytesArgs("or1", "val1"), s)
	if err := cmdCRDTORSETADD(ctx); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCmdCRDTORSETADD_WrongArgs(t *testing.T) {
	s := store.NewStore()
	ctx := discardCtx("CRDT.ORSET.ADD", bytesArgs("or1"), s)
	if err := cmdCRDTORSETADD(ctx); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCmdCRDTORSETREMOVE_Success(t *testing.T) {
	s := store.NewStore()
	_ = cmdCRDTORSETADD(discardCtx("", bytesArgs("orrm1", "val1"), s))
	ctx := discardCtx("CRDT.ORSET.REMOVE", bytesArgs("orrm1", "val1"), s)
	if err := cmdCRDTORSETREMOVE(ctx); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCmdCRDTORSETREMOVE_WrongArgs(t *testing.T) {
	s := store.NewStore()
	ctx := discardCtx("CRDT.ORSET.REMOVE", bytesArgs("or1"), s)
	if err := cmdCRDTORSETREMOVE(ctx); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCmdCRDTORSETREMOVE_NotFound(t *testing.T) {
	s := store.NewStore()
	ctx := discardCtx("CRDT.ORSET.REMOVE", bytesArgs("nope", "val"), s)
	if err := cmdCRDTORSETREMOVE(ctx); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCmdCRDTORSETGET_Success(t *testing.T) {
	s := store.NewStore()
	_ = cmdCRDTORSETADD(discardCtx("", bytesArgs("orget1", "val1"), s))
	ctx := discardCtx("CRDT.ORSET.GET", bytesArgs("orget1"), s)
	if err := cmdCRDTORSETGET(ctx); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCmdCRDTORSETGET_WrongArgs(t *testing.T) {
	s := store.NewStore()
	ctx := discardCtx("CRDT.ORSET.GET", bytesArgs(), s)
	if err := cmdCRDTORSETGET(ctx); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

// --- MERKLE ---

func TestCmdMERKLECREATE_Success(t *testing.T) {
	s := store.NewStore()
	ctx := discardCtx("MERKLE.CREATE", bytesArgs("mt1"), s)
	if err := cmdMERKLECREATE(ctx); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCmdMERKLECREATE_WrongArgs(t *testing.T) {
	s := store.NewStore()
	ctx := discardCtx("MERKLE.CREATE", bytesArgs(), s)
	if err := cmdMERKLECREATE(ctx); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCmdMERKLEADD_Success(t *testing.T) {
	s := store.NewStore()
	_ = cmdMERKLECREATE(discardCtx("", bytesArgs("mtadd1"), s))
	ctx := discardCtx("MERKLE.ADD", bytesArgs("mtadd1", "hello"), s)
	if err := cmdMERKLEADD(ctx); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCmdMERKLEADD_WrongArgs(t *testing.T) {
	s := store.NewStore()
	ctx := discardCtx("MERKLE.ADD", bytesArgs("x"), s)
	if err := cmdMERKLEADD(ctx); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCmdMERKLEADD_NotFound(t *testing.T) {
	s := store.NewStore()
	ctx := discardCtx("MERKLE.ADD", bytesArgs("nope", "data"), s)
	if err := cmdMERKLEADD(ctx); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCmdMERKLEVERIFY_Success(t *testing.T) {
	s := store.NewStore()
	_ = cmdMERKLECREATE(discardCtx("", bytesArgs("mtver1"), s))
	ctx := discardCtx("MERKLE.VERIFY", bytesArgs("mtver1", "somehash"), s)
	if err := cmdMERKLEVERIFY(ctx); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCmdMERKLEVERIFY_WrongArgs(t *testing.T) {
	s := store.NewStore()
	ctx := discardCtx("MERKLE.VERIFY", bytesArgs("x"), s)
	if err := cmdMERKLEVERIFY(ctx); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCmdMERKLEPROOF_Success(t *testing.T) {
	s := store.NewStore()
	_ = cmdMERKLECREATE(discardCtx("", bytesArgs("mtprf1"), s))
	ctx := discardCtx("MERKLE.PROOF", bytesArgs("mtprf1", "somehash"), s)
	if err := cmdMERKLEPROOF(ctx); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCmdMERKLEPROOF_WrongArgs(t *testing.T) {
	s := store.NewStore()
	ctx := discardCtx("MERKLE.PROOF", bytesArgs("x"), s)
	if err := cmdMERKLEPROOF(ctx); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCmdMERKLEPROOF_NotFound(t *testing.T) {
	s := store.NewStore()
	ctx := discardCtx("MERKLE.PROOF", bytesArgs("nosuch", "hash"), s)
	if err := cmdMERKLEPROOF(ctx); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCmdMERKLEROOT_Success(t *testing.T) {
	s := store.NewStore()
	_ = cmdMERKLECREATE(discardCtx("", bytesArgs("mtroot1"), s))
	ctx := discardCtx("MERKLE.ROOT", bytesArgs("mtroot1"), s)
	if err := cmdMERKLEROOT(ctx); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCmdMERKLEROOT_WrongArgs(t *testing.T) {
	s := store.NewStore()
	ctx := discardCtx("MERKLE.ROOT", bytesArgs(), s)
	if err := cmdMERKLEROOT(ctx); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCmdMERKLEROOT_NotFound(t *testing.T) {
	s := store.NewStore()
	ctx := discardCtx("MERKLE.ROOT", bytesArgs("nope"), s)
	if err := cmdMERKLEROOT(ctx); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

// --- RAFT ---

func TestCmdRAFTSTATE_Success(t *testing.T) {
	s := store.NewStore()
	ctx := discardCtx("RAFT.STATE", bytesArgs("raft1", "follower"), s)
	if err := cmdRAFTSTATE(ctx); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCmdRAFTSTATE_WrongArgs(t *testing.T) {
	s := store.NewStore()
	ctx := discardCtx("RAFT.STATE", bytesArgs("raft1"), s)
	if err := cmdRAFTSTATE(ctx); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCmdRAFTSTATE_Update(t *testing.T) {
	s := store.NewStore()
	_ = cmdRAFTSTATE(discardCtx("", bytesArgs("raftup", "follower"), s))
	ctx := discardCtx("RAFT.STATE", bytesArgs("raftup", "leader"), s)
	if err := cmdRAFTSTATE(ctx); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCmdRAFTLEADER_Success(t *testing.T) {
	s := store.NewStore()
	_ = cmdRAFTSTATE(discardCtx("", bytesArgs("raftld", "follower"), s))
	ctx := discardCtx("RAFT.LEADER", bytesArgs("raftld", "node1"), s)
	if err := cmdRAFTLEADER(ctx); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCmdRAFTLEADER_WrongArgs(t *testing.T) {
	s := store.NewStore()
	ctx := discardCtx("RAFT.LEADER", bytesArgs("x"), s)
	if err := cmdRAFTLEADER(ctx); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCmdRAFTLEADER_NotFound(t *testing.T) {
	s := store.NewStore()
	ctx := discardCtx("RAFT.LEADER", bytesArgs("nope", "node1"), s)
	if err := cmdRAFTLEADER(ctx); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCmdRAFTTERM_Success(t *testing.T) {
	s := store.NewStore()
	_ = cmdRAFTSTATE(discardCtx("", bytesArgs("raftterm", "follower"), s))
	ctx := discardCtx("RAFT.TERM", bytesArgs("raftterm"), s)
	if err := cmdRAFTTERM(ctx); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCmdRAFTTERM_WrongArgs(t *testing.T) {
	s := store.NewStore()
	ctx := discardCtx("RAFT.TERM", bytesArgs(), s)
	if err := cmdRAFTTERM(ctx); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCmdRAFTTERM_NotFound(t *testing.T) {
	s := store.NewStore()
	ctx := discardCtx("RAFT.TERM", bytesArgs("nope"), s)
	if err := cmdRAFTTERM(ctx); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCmdRAFTVOTE_Success(t *testing.T) {
	s := store.NewStore()
	_ = cmdRAFTSTATE(discardCtx("", bytesArgs("raftvote", "follower"), s))
	ctx := discardCtx("RAFT.VOTE", bytesArgs("raftvote", "candidate1"), s)
	if err := cmdRAFTVOTE(ctx); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCmdRAFTVOTE_WrongArgs(t *testing.T) {
	s := store.NewStore()
	ctx := discardCtx("RAFT.VOTE", bytesArgs("x"), s)
	if err := cmdRAFTVOTE(ctx); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCmdRAFTVOTE_NotFound(t *testing.T) {
	s := store.NewStore()
	ctx := discardCtx("RAFT.VOTE", bytesArgs("nope", "c1"), s)
	if err := cmdRAFTVOTE(ctx); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCmdRAFTAPPEND_Success(t *testing.T) {
	s := store.NewStore()
	_ = cmdRAFTSTATE(discardCtx("", bytesArgs("raftapp", "leader"), s))
	ctx := discardCtx("RAFT.APPEND", bytesArgs("raftapp", "entry1"), s)
	if err := cmdRAFTAPPEND(ctx); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCmdRAFTAPPEND_WrongArgs(t *testing.T) {
	s := store.NewStore()
	ctx := discardCtx("RAFT.APPEND", bytesArgs("x"), s)
	if err := cmdRAFTAPPEND(ctx); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCmdRAFTAPPEND_NotFound(t *testing.T) {
	s := store.NewStore()
	ctx := discardCtx("RAFT.APPEND", bytesArgs("nope", "entry1"), s)
	if err := cmdRAFTAPPEND(ctx); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCmdRAFTCOMMIT_Success(t *testing.T) {
	s := store.NewStore()
	_ = cmdRAFTSTATE(discardCtx("", bytesArgs("raftcom", "leader"), s))
	ctx := discardCtx("RAFT.COMMIT", bytesArgs("raftcom"), s)
	if err := cmdRAFTCOMMIT(ctx); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCmdRAFTCOMMIT_WrongArgs(t *testing.T) {
	s := store.NewStore()
	ctx := discardCtx("RAFT.COMMIT", bytesArgs(), s)
	if err := cmdRAFTCOMMIT(ctx); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCmdRAFTCOMMIT_NotFound(t *testing.T) {
	s := store.NewStore()
	ctx := discardCtx("RAFT.COMMIT", bytesArgs("nope"), s)
	if err := cmdRAFTCOMMIT(ctx); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

// --- SHARD ---

func TestCmdSHARDMAP_Success(t *testing.T) {
	s := store.NewStore()
	ctx := discardCtx("SHARD.MAP", bytesArgs("shard1", "mykey"), s)
	if err := cmdSHARDMAP(ctx); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCmdSHARDMAP_WrongArgs(t *testing.T) {
	s := store.NewStore()
	ctx := discardCtx("SHARD.MAP", bytesArgs("shard1"), s)
	if err := cmdSHARDMAP(ctx); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCmdSHARDMOVE_Success(t *testing.T) {
	s := store.NewStore()
	ctx := discardCtx("SHARD.MOVE", bytesArgs("shard1", "target"), s)
	if err := cmdSHARDMOVE(ctx); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCmdSHARDMOVE_WrongArgs(t *testing.T) {
	s := store.NewStore()
	ctx := discardCtx("SHARD.MOVE", bytesArgs("x"), s)
	if err := cmdSHARDMOVE(ctx); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCmdSHARDREBALANCE_WrongArgs(t *testing.T) {
	s := store.NewStore()
	ctx := discardCtx("SHARD.REBALANCE", bytesArgs(), s)
	if err := cmdSHARDREBALANCE(ctx); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCmdSHARDLIST_Success(t *testing.T) {
	s := store.NewStore()
	ctx := discardCtx("SHARD.LIST", bytesArgs(), s)
	if err := cmdSHARDLIST(ctx); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCmdSHARDSTATUS_WrongArgs(t *testing.T) {
	s := store.NewStore()
	ctx := discardCtx("SHARD.STATUS", bytesArgs(), s)
	if err := cmdSHARDSTATUS(ctx); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

// --- COMPRESSION ---

func TestCmdCOMPRESSIONCOMPRESS_Success(t *testing.T) {
	s := store.NewStore()
	ctx := discardCtx("COMPRESSION.COMPRESS", bytesArgs("hello world"), s)
	if err := cmdCOMPRESSIONCOMPRESS(ctx); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCmdCOMPRESSIONCOMPRESS_WrongArgs(t *testing.T) {
	s := store.NewStore()
	ctx := discardCtx("COMPRESSION.COMPRESS", bytesArgs(), s)
	if err := cmdCOMPRESSIONCOMPRESS(ctx); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCmdCOMPRESSIONDECOMPRESS_Success(t *testing.T) {
	s := store.NewStore()
	ctx := discardCtx("COMPRESSION.DECOMPRESS", bytesArgs("compressed:11"), s)
	if err := cmdCOMPRESSIONDECOMPRESS(ctx); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCmdCOMPRESSIONDECOMPRESS_WrongArgs(t *testing.T) {
	s := store.NewStore()
	ctx := discardCtx("COMPRESSION.DECOMPRESS", bytesArgs(), s)
	if err := cmdCOMPRESSIONDECOMPRESS(ctx); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCmdCOMPRESSIONINFO_Success(t *testing.T) {
	s := store.NewStore()
	ctx := discardCtx("COMPRESSION.INFO", bytesArgs(), s)
	if err := cmdCOMPRESSIONINFO(ctx); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

// --- DEDUP ---

func TestCmdDEDUPADD_Success(t *testing.T) {
	s := store.NewStore()
	ctx := discardCtx("DEDUP.ADD", bytesArgs("dedkey", "id1"), s)
	if err := cmdDEDUPADD(ctx); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCmdDEDUPADD_WrongArgs(t *testing.T) {
	s := store.NewStore()
	ctx := discardCtx("DEDUP.ADD", bytesArgs("dedkey"), s)
	if err := cmdDEDUPADD(ctx); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCmdDEDUPADD_WithTTL(t *testing.T) {
	s := store.NewStore()
	ctx := discardCtx("DEDUP.ADD", bytesArgs("dedkey2", "id2", "60000"), s)
	if err := cmdDEDUPADD(ctx); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCmdDEDUPCHECK_Success(t *testing.T) {
	s := store.NewStore()
	_ = cmdDEDUPADD(discardCtx("", bytesArgs("dedchk", "id1"), s))
	ctx := discardCtx("DEDUP.CHECK", bytesArgs("dedchk", "id1"), s)
	if err := cmdDEDUPCHECK(ctx); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCmdDEDUPCHECK_WrongArgs(t *testing.T) {
	s := store.NewStore()
	ctx := discardCtx("DEDUP.CHECK", bytesArgs("x"), s)
	if err := cmdDEDUPCHECK(ctx); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCmdDEDUPEXPIRE_Success(t *testing.T) {
	s := store.NewStore()
	ctx := discardCtx("DEDUP.EXPIRE", bytesArgs(), s)
	if err := cmdDEDUPEXPIRE(ctx); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCmdDEDUPCLEAR_Success(t *testing.T) {
	s := store.NewStore()
	ctx := discardCtx("DEDUP.CLEAR", bytesArgs(), s)
	if err := cmdDEDUPCLEAR(ctx); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

// --- BATCH ---

func TestCmdBATCHSUBMIT_Success(t *testing.T) {
	s := store.NewStore()
	ctx := discardCtx("BATCH.SUBMIT", bytesArgs("10"), s)
	if err := cmdBATCHSUBMIT(ctx); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCmdBATCHSUBMIT_WrongArgs(t *testing.T) {
	s := store.NewStore()
	ctx := discardCtx("BATCH.SUBMIT", bytesArgs(), s)
	if err := cmdBATCHSUBMIT(ctx); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCmdBATCHSTATUS_WrongArgs(t *testing.T) {
	s := store.NewStore()
	ctx := discardCtx("BATCH.STATUS", bytesArgs(), s)
	if err := cmdBATCHSTATUS(ctx); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCmdBATCHCANCEL_WrongArgs(t *testing.T) {
	s := store.NewStore()
	ctx := discardCtx("BATCH.CANCEL", bytesArgs(), s)
	if err := cmdBATCHCANCEL(ctx); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCmdBATCHLIST_Success(t *testing.T) {
	s := store.NewStore()
	ctx := discardCtx("BATCH.LIST", bytesArgs(), s)
	if err := cmdBATCHLIST(ctx); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

// --- DEADLINE ---

func TestCmdDEADLINESET_Success(t *testing.T) {
	s := store.NewStore()
	ctx := discardCtx("DEADLINE.SET", bytesArgs("dl1", "60000"), s)
	if err := cmdDEADLINESET(ctx); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCmdDEADLINESET_WrongArgs(t *testing.T) {
	s := store.NewStore()
	ctx := discardCtx("DEADLINE.SET", bytesArgs("dl1"), s)
	if err := cmdDEADLINESET(ctx); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCmdDEADLINESET_WithCallback(t *testing.T) {
	s := store.NewStore()
	ctx := discardCtx("DEADLINE.SET", bytesArgs("dl2", "60000", "myCallback"), s)
	if err := cmdDEADLINESET(ctx); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCmdDEADLINECHECK_Success(t *testing.T) {
	s := store.NewStore()
	_ = cmdDEADLINESET(discardCtx("", bytesArgs("dlchk", "60000"), s))
	ctx := discardCtx("DEADLINE.CHECK", bytesArgs("dlchk"), s)
	if err := cmdDEADLINECHECK(ctx); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCmdDEADLINECHECK_WrongArgs(t *testing.T) {
	s := store.NewStore()
	ctx := discardCtx("DEADLINE.CHECK", bytesArgs(), s)
	if err := cmdDEADLINECHECK(ctx); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCmdDEADLINECANCEL_Success(t *testing.T) {
	s := store.NewStore()
	_ = cmdDEADLINESET(discardCtx("", bytesArgs("dlcan", "60000"), s))
	ctx := discardCtx("DEADLINE.CANCEL", bytesArgs("dlcan"), s)
	if err := cmdDEADLINECANCEL(ctx); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCmdDEADLINECANCEL_WrongArgs(t *testing.T) {
	s := store.NewStore()
	ctx := discardCtx("DEADLINE.CANCEL", bytesArgs(), s)
	if err := cmdDEADLINECANCEL(ctx); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCmdDEADLINELIST_Success(t *testing.T) {
	s := store.NewStore()
	ctx := discardCtx("DEADLINE.LIST", bytesArgs(), s)
	if err := cmdDEADLINELIST(ctx); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

// --- SANITIZE ---

func TestCmdSANITIZESTRING_Success(t *testing.T) {
	s := store.NewStore()
	ctx := discardCtx("SANITIZE.STRING", bytesArgs("hello<script>"), s)
	if err := cmdSANITIZESTRING(ctx); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCmdSANITIZESTRING_WrongArgs(t *testing.T) {
	s := store.NewStore()
	ctx := discardCtx("SANITIZE.STRING", bytesArgs(), s)
	if err := cmdSANITIZESTRING(ctx); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCmdSANITIZEHTML_Success(t *testing.T) {
	s := store.NewStore()
	ctx := discardCtx("SANITIZE.HTML", bytesArgs("<b>bold</b>&\"'"), s)
	if err := cmdSANITIZEHTML(ctx); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCmdSANITIZEHTML_WrongArgs(t *testing.T) {
	s := store.NewStore()
	ctx := discardCtx("SANITIZE.HTML", bytesArgs(), s)
	if err := cmdSANITIZEHTML(ctx); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCmdSANITIZEJSON_Success(t *testing.T) {
	s := store.NewStore()
	ctx := discardCtx("SANITIZE.JSON", bytesArgs("he\"llo\\\n\r\t"), s)
	if err := cmdSANITIZEJSON(ctx); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCmdSANITIZEJSON_WrongArgs(t *testing.T) {
	s := store.NewStore()
	ctx := discardCtx("SANITIZE.JSON", bytesArgs(), s)
	if err := cmdSANITIZEJSON(ctx); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCmdSANITIZESQL_Success(t *testing.T) {
	s := store.NewStore()
	ctx := discardCtx("SANITIZE.SQL", bytesArgs("O'Reilly\\path"), s)
	if err := cmdSANITIZESQL(ctx); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCmdSANITIZESQL_WrongArgs(t *testing.T) {
	s := store.NewStore()
	ctx := discardCtx("SANITIZE.SQL", bytesArgs(), s)
	if err := cmdSANITIZESQL(ctx); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

// --- MASK ---

func TestCmdMASKCARD_Success(t *testing.T) {
	s := store.NewStore()
	ctx := discardCtx("MASK.CARD", bytesArgs("4111111111111111"), s)
	if err := cmdMASKCARD(ctx); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCmdMASKCARD_WrongArgs(t *testing.T) {
	s := store.NewStore()
	ctx := discardCtx("MASK.CARD", bytesArgs(), s)
	if err := cmdMASKCARD(ctx); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCmdMASKCARD_Short(t *testing.T) {
	s := store.NewStore()
	ctx := discardCtx("MASK.CARD", bytesArgs("12"), s)
	if err := cmdMASKCARD(ctx); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCmdMASKEMAIL_Success(t *testing.T) {
	s := store.NewStore()
	ctx := discardCtx("MASK.EMAIL", bytesArgs("user@example.com"), s)
	if err := cmdMASKEMAIL(ctx); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCmdMASKEMAIL_WrongArgs(t *testing.T) {
	s := store.NewStore()
	ctx := discardCtx("MASK.EMAIL", bytesArgs(), s)
	if err := cmdMASKEMAIL(ctx); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCmdMASKEMAIL_NoAt(t *testing.T) {
	s := store.NewStore()
	ctx := discardCtx("MASK.EMAIL", bytesArgs("noemail"), s)
	if err := cmdMASKEMAIL(ctx); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCmdMASKPHONE_Success(t *testing.T) {
	s := store.NewStore()
	ctx := discardCtx("MASK.PHONE", bytesArgs("1234567890"), s)
	if err := cmdMASKPHONE(ctx); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCmdMASKPHONE_WrongArgs(t *testing.T) {
	s := store.NewStore()
	ctx := discardCtx("MASK.PHONE", bytesArgs(), s)
	if err := cmdMASKPHONE(ctx); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCmdMASKPHONE_Short(t *testing.T) {
	s := store.NewStore()
	ctx := discardCtx("MASK.PHONE", bytesArgs("12"), s)
	if err := cmdMASKPHONE(ctx); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCmdMASKIP_Success(t *testing.T) {
	s := store.NewStore()
	ctx := discardCtx("MASK.IP", bytesArgs("192.168.1.1"), s)
	if err := cmdMASKIP(ctx); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCmdMASKIP_WrongArgs(t *testing.T) {
	s := store.NewStore()
	ctx := discardCtx("MASK.IP", bytesArgs(), s)
	if err := cmdMASKIP(ctx); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCmdMASKIP_Invalid(t *testing.T) {
	s := store.NewStore()
	ctx := discardCtx("MASK.IP", bytesArgs("not-an-ip"), s)
	if err := cmdMASKIP(ctx); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

// --- GATEWAY ---

func TestCmdGATEWAYCREATE_Success(t *testing.T) {
	s := store.NewStore()
	ctx := discardCtx("GATEWAY.CREATE", bytesArgs("mygw"), s)
	if err := cmdGATEWAYCREATE(ctx); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCmdGATEWAYCREATE_WrongArgs(t *testing.T) {
	s := store.NewStore()
	ctx := discardCtx("GATEWAY.CREATE", bytesArgs(), s)
	if err := cmdGATEWAYCREATE(ctx); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCmdGATEWAYDELETE_WrongArgs(t *testing.T) {
	s := store.NewStore()
	ctx := discardCtx("GATEWAY.DELETE", bytesArgs(), s)
	if err := cmdGATEWAYDELETE(ctx); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCmdGATEWAYDELETE_NotFound(t *testing.T) {
	s := store.NewStore()
	ctx := discardCtx("GATEWAY.DELETE", bytesArgs("nope"), s)
	if err := cmdGATEWAYDELETE(ctx); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCmdGATEWAYROUTE_WrongArgs(t *testing.T) {
	s := store.NewStore()
	ctx := discardCtx("GATEWAY.ROUTE", bytesArgs("gw1", "/api"), s)
	if err := cmdGATEWAYROUTE(ctx); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCmdGATEWAYLIST_Success(t *testing.T) {
	s := store.NewStore()
	ctx := discardCtx("GATEWAY.LIST", bytesArgs(), s)
	if err := cmdGATEWAYLIST(ctx); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCmdGATEWAYMETRICS_WrongArgs(t *testing.T) {
	s := store.NewStore()
	ctx := discardCtx("GATEWAY.METRICS", bytesArgs(), s)
	if err := cmdGATEWAYMETRICS(ctx); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

// --- THRESHOLD ---

func TestCmdTHRESHOLDSET_Success(t *testing.T) {
	s := store.NewStore()
	ctx := discardCtx("THRESHOLD.SET", bytesArgs("th1", "100"), s)
	if err := cmdTHRESHOLDSET(ctx); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCmdTHRESHOLDSET_WrongArgs(t *testing.T) {
	s := store.NewStore()
	ctx := discardCtx("THRESHOLD.SET", bytesArgs("th1"), s)
	if err := cmdTHRESHOLDSET(ctx); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCmdTHRESHOLDCHECK_Success(t *testing.T) {
	s := store.NewStore()
	_ = cmdTHRESHOLDSET(discardCtx("", bytesArgs("thchk", "50"), s))
	ctx := discardCtx("THRESHOLD.CHECK", bytesArgs("thchk", "60"), s)
	if err := cmdTHRESHOLDCHECK(ctx); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCmdTHRESHOLDCHECK_WrongArgs(t *testing.T) {
	s := store.NewStore()
	ctx := discardCtx("THRESHOLD.CHECK", bytesArgs("x"), s)
	if err := cmdTHRESHOLDCHECK(ctx); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCmdTHRESHOLDLIST_Success(t *testing.T) {
	s := store.NewStore()
	ctx := discardCtx("THRESHOLD.LIST", bytesArgs(), s)
	if err := cmdTHRESHOLDLIST(ctx); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCmdTHRESHOLDDELETE_Success(t *testing.T) {
	s := store.NewStore()
	_ = cmdTHRESHOLDSET(discardCtx("", bytesArgs("thdel", "10"), s))
	ctx := discardCtx("THRESHOLD.DELETE", bytesArgs("thdel"), s)
	if err := cmdTHRESHOLDDELETE(ctx); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCmdTHRESHOLDDELETE_WrongArgs(t *testing.T) {
	s := store.NewStore()
	ctx := discardCtx("THRESHOLD.DELETE", bytesArgs(), s)
	if err := cmdTHRESHOLDDELETE(ctx); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

// --- SWITCH ---

func TestCmdSWITCHSTATE_Success(t *testing.T) {
	s := store.NewStore()
	ctx := discardCtx("SWITCH.STATE", bytesArgs("sw1"), s)
	if err := cmdSWITCHSTATE(ctx); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCmdSWITCHSTATE_WrongArgs(t *testing.T) {
	s := store.NewStore()
	ctx := discardCtx("SWITCH.STATE", bytesArgs(), s)
	if err := cmdSWITCHSTATE(ctx); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCmdSWITCHTOGGLE_Success(t *testing.T) {
	s := store.NewStore()
	ctx := discardCtx("SWITCH.TOGGLE", bytesArgs("swtog"), s)
	if err := cmdSWITCHTOGGLE(ctx); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCmdSWITCHTOGGLE_WrongArgs(t *testing.T) {
	s := store.NewStore()
	ctx := discardCtx("SWITCH.TOGGLE", bytesArgs(), s)
	if err := cmdSWITCHTOGGLE(ctx); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCmdSWITCHON_Success(t *testing.T) {
	s := store.NewStore()
	ctx := discardCtx("SWITCH.ON", bytesArgs("swon"), s)
	if err := cmdSWITCHON(ctx); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCmdSWITCHON_WrongArgs(t *testing.T) {
	s := store.NewStore()
	ctx := discardCtx("SWITCH.ON", bytesArgs(), s)
	if err := cmdSWITCHON(ctx); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCmdSWITCHOFF_Success(t *testing.T) {
	s := store.NewStore()
	ctx := discardCtx("SWITCH.OFF", bytesArgs("swoff"), s)
	if err := cmdSWITCHOFF(ctx); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCmdSWITCHOFF_WrongArgs(t *testing.T) {
	s := store.NewStore()
	ctx := discardCtx("SWITCH.OFF", bytesArgs(), s)
	if err := cmdSWITCHOFF(ctx); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCmdSWITCHLIST_Success(t *testing.T) {
	s := store.NewStore()
	ctx := discardCtx("SWITCH.LIST", bytesArgs(), s)
	if err := cmdSWITCHLIST(ctx); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

// --- BOOKMARK ---

func TestCmdBOOKMARKSET_Success(t *testing.T) {
	s := store.NewStore()
	ctx := discardCtx("BOOKMARK.SET", bytesArgs("bk1", "val1"), s)
	if err := cmdBOOKMARKSET(ctx); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCmdBOOKMARKSET_WrongArgs(t *testing.T) {
	s := store.NewStore()
	ctx := discardCtx("BOOKMARK.SET", bytesArgs("bk1"), s)
	if err := cmdBOOKMARKSET(ctx); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCmdBOOKMARKGET_Success(t *testing.T) {
	s := store.NewStore()
	_ = cmdBOOKMARKSET(discardCtx("", bytesArgs("bkget", "val"), s))
	ctx := discardCtx("BOOKMARK.GET", bytesArgs("bkget"), s)
	if err := cmdBOOKMARKGET(ctx); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCmdBOOKMARKGET_WrongArgs(t *testing.T) {
	s := store.NewStore()
	ctx := discardCtx("BOOKMARK.GET", bytesArgs(), s)
	if err := cmdBOOKMARKGET(ctx); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCmdBOOKMARKDELETE_Success(t *testing.T) {
	s := store.NewStore()
	_ = cmdBOOKMARKSET(discardCtx("", bytesArgs("bkdel", "val"), s))
	ctx := discardCtx("BOOKMARK.DELETE", bytesArgs("bkdel"), s)
	if err := cmdBOOKMARKDELETE(ctx); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCmdBOOKMARKDELETE_WrongArgs(t *testing.T) {
	s := store.NewStore()
	ctx := discardCtx("BOOKMARK.DELETE", bytesArgs(), s)
	if err := cmdBOOKMARKDELETE(ctx); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCmdBOOKMARKLIST_Success(t *testing.T) {
	s := store.NewStore()
	ctx := discardCtx("BOOKMARK.LIST", bytesArgs(), s)
	if err := cmdBOOKMARKLIST(ctx); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

// --- REPLAYX ---

func TestCmdREPLAYXSTART_Success(t *testing.T) {
	s := store.NewStore()
	ctx := discardCtx("REPLAYX.START", bytesArgs("rp1"), s)
	if err := cmdREPLAYXSTART(ctx); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCmdREPLAYXSTART_WrongArgs(t *testing.T) {
	s := store.NewStore()
	ctx := discardCtx("REPLAYX.START", bytesArgs(), s)
	if err := cmdREPLAYXSTART(ctx); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCmdREPLAYXSTOP_Success(t *testing.T) {
	s := store.NewStore()
	_ = cmdREPLAYXSTART(discardCtx("", bytesArgs("rpstop"), s))
	ctx := discardCtx("REPLAYX.STOP", bytesArgs("rpstop"), s)
	if err := cmdREPLAYXSTOP(ctx); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCmdREPLAYXSTOP_WrongArgs(t *testing.T) {
	s := store.NewStore()
	ctx := discardCtx("REPLAYX.STOP", bytesArgs(), s)
	if err := cmdREPLAYXSTOP(ctx); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCmdREPLAYXPAUSE_Success(t *testing.T) {
	s := store.NewStore()
	_ = cmdREPLAYXSTART(discardCtx("", bytesArgs("rppause"), s))
	ctx := discardCtx("REPLAYX.PAUSE", bytesArgs("rppause"), s)
	if err := cmdREPLAYXPAUSE(ctx); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCmdREPLAYXPAUSE_WrongArgs(t *testing.T) {
	s := store.NewStore()
	ctx := discardCtx("REPLAYX.PAUSE", bytesArgs(), s)
	if err := cmdREPLAYXPAUSE(ctx); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCmdREPLAYXSPEED_Success(t *testing.T) {
	s := store.NewStore()
	_ = cmdREPLAYXSTART(discardCtx("", bytesArgs("rpspeed"), s))
	ctx := discardCtx("REPLAYX.SPEED", bytesArgs("rpspeed", "2.0"), s)
	if err := cmdREPLAYXSPEED(ctx); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCmdREPLAYXSPEED_WrongArgs(t *testing.T) {
	s := store.NewStore()
	ctx := discardCtx("REPLAYX.SPEED", bytesArgs("x"), s)
	if err := cmdREPLAYXSPEED(ctx); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

// --- ROUTE ---

func TestCmdROUTEADD_Success(t *testing.T) {
	s := store.NewStore()
	ctx := discardCtx("ROUTE.ADD", bytesArgs("rt1", "/api", "backend1"), s)
	if err := cmdROUTEADD(ctx); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCmdROUTEADD_WrongArgs(t *testing.T) {
	s := store.NewStore()
	ctx := discardCtx("ROUTE.ADD", bytesArgs("rt1", "/api"), s)
	if err := cmdROUTEADD(ctx); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCmdROUTEREMOVE_Success(t *testing.T) {
	s := store.NewStore()
	_ = cmdROUTEADD(discardCtx("", bytesArgs("rtrm", "/api", "backend1"), s))
	ctx := discardCtx("ROUTE.REMOVE", bytesArgs("rtrm", "/api"), s)
	if err := cmdROUTEREMOVE(ctx); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCmdROUTEREMOVE_WrongArgs(t *testing.T) {
	s := store.NewStore()
	ctx := discardCtx("ROUTE.REMOVE", bytesArgs("x"), s)
	if err := cmdROUTEREMOVE(ctx); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCmdROUTEMATCH_Success(t *testing.T) {
	s := store.NewStore()
	_ = cmdROUTEADD(discardCtx("", bytesArgs("rtmt", "*", "backend1"), s))
	ctx := discardCtx("ROUTE.MATCH", bytesArgs("rtmt", "/anything"), s)
	if err := cmdROUTEMATCH(ctx); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCmdROUTEMATCH_WrongArgs(t *testing.T) {
	s := store.NewStore()
	ctx := discardCtx("ROUTE.MATCH", bytesArgs("x"), s)
	if err := cmdROUTEMATCH(ctx); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCmdROUTELIST_Success(t *testing.T) {
	s := store.NewStore()
	_ = cmdROUTEADD(discardCtx("", bytesArgs("rtls", "/api", "back"), s))
	ctx := discardCtx("ROUTE.LIST", bytesArgs("rtls"), s)
	if err := cmdROUTELIST(ctx); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCmdROUTELIST_WrongArgs(t *testing.T) {
	s := store.NewStore()
	ctx := discardCtx("ROUTE.LIST", bytesArgs(), s)
	if err := cmdROUTELIST(ctx); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

// --- GHOST ---

func TestCmdGHOSTCREATE_Success(t *testing.T) {
	s := store.NewStore()
	ctx := discardCtx("GHOST.CREATE", bytesArgs("gh1"), s)
	if err := cmdGHOSTCREATE(ctx); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCmdGHOSTCREATE_WrongArgs(t *testing.T) {
	s := store.NewStore()
	ctx := discardCtx("GHOST.CREATE", bytesArgs(), s)
	if err := cmdGHOSTCREATE(ctx); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCmdGHOSTWRITE_Success(t *testing.T) {
	s := store.NewStore()
	_ = cmdGHOSTCREATE(discardCtx("", bytesArgs("ghw"), s))
	ctx := discardCtx("GHOST.WRITE", bytesArgs("ghw", "data1"), s)
	if err := cmdGHOSTWRITE(ctx); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCmdGHOSTWRITE_WrongArgs(t *testing.T) {
	s := store.NewStore()
	ctx := discardCtx("GHOST.WRITE", bytesArgs("x"), s)
	if err := cmdGHOSTWRITE(ctx); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCmdGHOSTREAD_Success(t *testing.T) {
	s := store.NewStore()
	_ = cmdGHOSTCREATE(discardCtx("", bytesArgs("ghr"), s))
	ctx := discardCtx("GHOST.READ", bytesArgs("ghr"), s)
	if err := cmdGHOSTREAD(ctx); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCmdGHOSTREAD_WrongArgs(t *testing.T) {
	s := store.NewStore()
	ctx := discardCtx("GHOST.READ", bytesArgs(), s)
	if err := cmdGHOSTREAD(ctx); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCmdGHOSTDELETE_Success(t *testing.T) {
	s := store.NewStore()
	_ = cmdGHOSTCREATE(discardCtx("", bytesArgs("ghd"), s))
	ctx := discardCtx("GHOST.DELETE", bytesArgs("ghd"), s)
	if err := cmdGHOSTDELETE(ctx); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCmdGHOSTDELETE_WrongArgs(t *testing.T) {
	s := store.NewStore()
	ctx := discardCtx("GHOST.DELETE", bytesArgs(), s)
	if err := cmdGHOSTDELETE(ctx); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

// --- PROBE ---

func TestCmdPROBECREATE_Success(t *testing.T) {
	s := store.NewStore()
	ctx := discardCtx("PROBE.CREATE", bytesArgs("pb1", "probe1", "http://target"), s)
	if err := cmdPROBECREATE(ctx); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCmdPROBECREATE_WrongArgs(t *testing.T) {
	s := store.NewStore()
	ctx := discardCtx("PROBE.CREATE", bytesArgs("pb1", "probe1"), s)
	if err := cmdPROBECREATE(ctx); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCmdPROBEDELETE_Success(t *testing.T) {
	s := store.NewStore()
	_ = cmdPROBECREATE(discardCtx("", bytesArgs("pbd", "p", "t"), s))
	ctx := discardCtx("PROBE.DELETE", bytesArgs("pbd"), s)
	if err := cmdPROBEDELETE(ctx); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCmdPROBEDELETE_WrongArgs(t *testing.T) {
	s := store.NewStore()
	ctx := discardCtx("PROBE.DELETE", bytesArgs(), s)
	if err := cmdPROBEDELETE(ctx); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCmdPROBERUN_Success(t *testing.T) {
	s := store.NewStore()
	_ = cmdPROBECREATE(discardCtx("", bytesArgs("pbrun", "p", "t"), s))
	ctx := discardCtx("PROBE.RUN", bytesArgs("pbrun"), s)
	if err := cmdPROBERUN(ctx); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCmdPROBERUN_WrongArgs(t *testing.T) {
	s := store.NewStore()
	ctx := discardCtx("PROBE.RUN", bytesArgs(), s)
	if err := cmdPROBERUN(ctx); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCmdPROBERESULTS_Success(t *testing.T) {
	s := store.NewStore()
	_ = cmdPROBECREATE(discardCtx("", bytesArgs("pbres", "p", "t"), s))
	ctx := discardCtx("PROBE.RESULTS", bytesArgs("pbres"), s)
	if err := cmdPROBERESULTS(ctx); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCmdPROBERESULTS_WrongArgs(t *testing.T) {
	s := store.NewStore()
	ctx := discardCtx("PROBE.RESULTS", bytesArgs(), s)
	if err := cmdPROBERESULTS(ctx); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCmdPROBELIST_Success(t *testing.T) {
	s := store.NewStore()
	ctx := discardCtx("PROBE.LIST", bytesArgs(), s)
	if err := cmdPROBELIST(ctx); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

// --- CANARY ---

func TestCmdCANARYCREATE_Success(t *testing.T) {
	s := store.NewStore()
	ctx := discardCtx("CANARY.CREATE", bytesArgs("cn1", "mycanary"), s)
	if err := cmdCANARYCREATE(ctx); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCmdCANARYCREATE_WrongArgs(t *testing.T) {
	s := store.NewStore()
	ctx := discardCtx("CANARY.CREATE", bytesArgs("cn1"), s)
	if err := cmdCANARYCREATE(ctx); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCmdCANARYDELETE_Success(t *testing.T) {
	s := store.NewStore()
	_ = cmdCANARYCREATE(discardCtx("", bytesArgs("cndel", "c"), s))
	ctx := discardCtx("CANARY.DELETE", bytesArgs("cndel"), s)
	if err := cmdCANARYDELETE(ctx); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCmdCANARYDELETE_WrongArgs(t *testing.T) {
	s := store.NewStore()
	ctx := discardCtx("CANARY.DELETE", bytesArgs(), s)
	if err := cmdCANARYDELETE(ctx); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCmdCANARYCHECK_Success(t *testing.T) {
	s := store.NewStore()
	_ = cmdCANARYCREATE(discardCtx("", bytesArgs("cnchk", "c"), s))
	ctx := discardCtx("CANARY.CHECK", bytesArgs("cnchk"), s)
	if err := cmdCANARYCHECK(ctx); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCmdCANARYCHECK_WrongArgs(t *testing.T) {
	s := store.NewStore()
	ctx := discardCtx("CANARY.CHECK", bytesArgs(), s)
	if err := cmdCANARYCHECK(ctx); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCmdCANARYSTATUS_Success(t *testing.T) {
	s := store.NewStore()
	_ = cmdCANARYCREATE(discardCtx("", bytesArgs("cnst", "c"), s))
	ctx := discardCtx("CANARY.STATUS", bytesArgs("cnst"), s)
	if err := cmdCANARYSTATUS(ctx); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCmdCANARYSTATUS_WrongArgs(t *testing.T) {
	s := store.NewStore()
	ctx := discardCtx("CANARY.STATUS", bytesArgs(), s)
	if err := cmdCANARYSTATUS(ctx); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCmdCANARYLIST_Success(t *testing.T) {
	s := store.NewStore()
	ctx := discardCtx("CANARY.LIST", bytesArgs(), s)
	if err := cmdCANARYLIST(ctx); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

// --- RAGE ---

func TestCmdRAGETEST_Success(t *testing.T) {
	s := store.NewStore()
	ctx := discardCtx("RAGE.TEST", bytesArgs("rg1"), s)
	if err := cmdRAGETEST(ctx); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCmdRAGETEST_WrongArgs(t *testing.T) {
	s := store.NewStore()
	ctx := discardCtx("RAGE.TEST", bytesArgs(), s)
	if err := cmdRAGETEST(ctx); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCmdRAGESTOP_Success(t *testing.T) {
	s := store.NewStore()
	_ = cmdRAGETEST(discardCtx("", bytesArgs("rgstop"), s))
	ctx := discardCtx("RAGE.STOP", bytesArgs("rgstop"), s)
	if err := cmdRAGESTOP(ctx); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCmdRAGESTOP_WrongArgs(t *testing.T) {
	s := store.NewStore()
	ctx := discardCtx("RAGE.STOP", bytesArgs(), s)
	if err := cmdRAGESTOP(ctx); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCmdRAGESTATS_Success(t *testing.T) {
	s := store.NewStore()
	_ = cmdRAGETEST(discardCtx("", bytesArgs("rgstats"), s))
	ctx := discardCtx("RAGE.STATS", bytesArgs("rgstats"), s)
	if err := cmdRAGESTATS(ctx); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCmdRAGESTATS_WrongArgs(t *testing.T) {
	s := store.NewStore()
	ctx := discardCtx("RAGE.STATS", bytesArgs(), s)
	if err := cmdRAGESTATS(ctx); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCmdRAGERESET_Success(t *testing.T) {
	s := store.NewStore()
	ctx := discardCtx("RAGE.RESET", bytesArgs("rgreset"), s)
	if err := cmdRAGERESET(ctx); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCmdRAGERESET_WrongArgs(t *testing.T) {
	s := store.NewStore()
	ctx := discardCtx("RAGE.RESET", bytesArgs(), s)
	if err := cmdRAGERESET(ctx); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

// --- GRID ---

func TestCmdGRIDCREATE_Success(t *testing.T) {
	s := store.NewStore()
	ctx := discardCtx("GRID.CREATE", bytesArgs("grid1", "10", "10"), s)
	if err := cmdGRIDCREATE(ctx); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCmdGRIDCREATE_WrongArgs(t *testing.T) {
	s := store.NewStore()
	ctx := discardCtx("GRID.CREATE", bytesArgs("grid1", "10"), s)
	if err := cmdGRIDCREATE(ctx); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCmdGRIDSET_Success(t *testing.T) {
	s := store.NewStore()
	_ = cmdGRIDCREATE(discardCtx("", bytesArgs("gset", "10", "10"), s))
	ctx := discardCtx("GRID.SET", bytesArgs("gset", "1", "2", "val"), s)
	if err := cmdGRIDSET(ctx); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCmdGRIDSET_WrongArgs(t *testing.T) {
	s := store.NewStore()
	ctx := discardCtx("GRID.SET", bytesArgs("g", "1", "2"), s)
	if err := cmdGRIDSET(ctx); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCmdGRIDGET_Success(t *testing.T) {
	s := store.NewStore()
	_ = cmdGRIDCREATE(discardCtx("", bytesArgs("gget", "10", "10"), s))
	ctx := discardCtx("GRID.GET", bytesArgs("gget", "1", "2"), s)
	if err := cmdGRIDGET(ctx); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCmdGRIDGET_WrongArgs(t *testing.T) {
	s := store.NewStore()
	ctx := discardCtx("GRID.GET", bytesArgs("g", "1"), s)
	if err := cmdGRIDGET(ctx); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCmdGRIDDELETE_Success(t *testing.T) {
	s := store.NewStore()
	_ = cmdGRIDCREATE(discardCtx("", bytesArgs("gdel", "10", "10"), s))
	ctx := discardCtx("GRID.DELETE", bytesArgs("gdel"), s)
	if err := cmdGRIDDELETE(ctx); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCmdGRIDDELETE_WrongArgs(t *testing.T) {
	s := store.NewStore()
	ctx := discardCtx("GRID.DELETE", bytesArgs(), s)
	if err := cmdGRIDDELETE(ctx); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCmdGRIDQUERY_Success(t *testing.T) {
	s := store.NewStore()
	_ = cmdGRIDCREATE(discardCtx("", bytesArgs("gquery", "10", "10"), s))
	ctx := discardCtx("GRID.QUERY", bytesArgs("gquery"), s)
	if err := cmdGRIDQUERY(ctx); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCmdGRIDQUERY_WrongArgs(t *testing.T) {
	s := store.NewStore()
	ctx := discardCtx("GRID.QUERY", bytesArgs(), s)
	if err := cmdGRIDQUERY(ctx); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCmdGRIDCLEAR_Success(t *testing.T) {
	s := store.NewStore()
	_ = cmdGRIDCREATE(discardCtx("", bytesArgs("gclear", "10", "10"), s))
	ctx := discardCtx("GRID.CLEAR", bytesArgs("gclear"), s)
	if err := cmdGRIDCLEAR(ctx); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCmdGRIDCLEAR_WrongArgs(t *testing.T) {
	s := store.NewStore()
	ctx := discardCtx("GRID.CLEAR", bytesArgs(), s)
	if err := cmdGRIDCLEAR(ctx); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

// --- TAPE ---

func TestCmdTAPECREATE_Success(t *testing.T) {
	s := store.NewStore()
	ctx := discardCtx("TAPE.CREATE", bytesArgs("tape1"), s)
	if err := cmdTAPECREATE(ctx); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCmdTAPECREATE_WrongArgs(t *testing.T) {
	s := store.NewStore()
	ctx := discardCtx("TAPE.CREATE", bytesArgs(), s)
	if err := cmdTAPECREATE(ctx); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCmdTAPEWRITE_Success(t *testing.T) {
	s := store.NewStore()
	_ = cmdTAPECREATE(discardCtx("", bytesArgs("tapew"), s))
	ctx := discardCtx("TAPE.WRITE", bytesArgs("tapew", "data1"), s)
	if err := cmdTAPEWRITE(ctx); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCmdTAPEWRITE_WrongArgs(t *testing.T) {
	s := store.NewStore()
	ctx := discardCtx("TAPE.WRITE", bytesArgs("x"), s)
	if err := cmdTAPEWRITE(ctx); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCmdTAPEREAD_Success(t *testing.T) {
	s := store.NewStore()
	_ = cmdTAPECREATE(discardCtx("", bytesArgs("taper"), s))
	_ = cmdTAPEWRITE(discardCtx("", bytesArgs("taper", "data1"), s))
	ctx := discardCtx("TAPE.READ", bytesArgs("taper"), s)
	if err := cmdTAPEREAD(ctx); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCmdTAPEREAD_WrongArgs(t *testing.T) {
	s := store.NewStore()
	ctx := discardCtx("TAPE.READ", bytesArgs(), s)
	if err := cmdTAPEREAD(ctx); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCmdTAPESEEK_Success(t *testing.T) {
	s := store.NewStore()
	_ = cmdTAPECREATE(discardCtx("", bytesArgs("tapesk"), s))
	_ = cmdTAPEWRITE(discardCtx("", bytesArgs("tapesk", "d0"), s))
	_ = cmdTAPEWRITE(discardCtx("", bytesArgs("tapesk", "d1"), s))
	ctx := discardCtx("TAPE.SEEK", bytesArgs("tapesk", "1"), s)
	if err := cmdTAPESEEK(ctx); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCmdTAPESEEK_WrongArgs(t *testing.T) {
	s := store.NewStore()
	ctx := discardCtx("TAPE.SEEK", bytesArgs("x"), s)
	if err := cmdTAPESEEK(ctx); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCmdTAPEDELETE_Success(t *testing.T) {
	s := store.NewStore()
	_ = cmdTAPECREATE(discardCtx("", bytesArgs("taped"), s))
	ctx := discardCtx("TAPE.DELETE", bytesArgs("taped"), s)
	if err := cmdTAPEDELETE(ctx); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCmdTAPEDELETE_WrongArgs(t *testing.T) {
	s := store.NewStore()
	ctx := discardCtx("TAPE.DELETE", bytesArgs(), s)
	if err := cmdTAPEDELETE(ctx); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

// --- SLICE ---

func TestCmdSLICECREATE_Success(t *testing.T) {
	s := store.NewStore()
	ctx := discardCtx("SLICE.CREATE", bytesArgs("sl1"), s)
	if err := cmdSLICECREATE(ctx); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCmdSLICECREATE_WrongArgs(t *testing.T) {
	s := store.NewStore()
	ctx := discardCtx("SLICE.CREATE", bytesArgs(), s)
	if err := cmdSLICECREATE(ctx); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCmdSLICEAPPEND_Success(t *testing.T) {
	s := store.NewStore()
	_ = cmdSLICECREATE(discardCtx("", bytesArgs("slapp"), s))
	ctx := discardCtx("SLICE.APPEND", bytesArgs("slapp", "v1", "v2"), s)
	if err := cmdSLICEAPPEND(ctx); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCmdSLICEAPPEND_WrongArgs(t *testing.T) {
	s := store.NewStore()
	ctx := discardCtx("SLICE.APPEND", bytesArgs("x"), s)
	if err := cmdSLICEAPPEND(ctx); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCmdSLICEGET_Success(t *testing.T) {
	s := store.NewStore()
	_ = cmdSLICECREATE(discardCtx("", bytesArgs("slget"), s))
	ctx := discardCtx("SLICE.GET", bytesArgs("slget"), s)
	if err := cmdSLICEGET(ctx); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCmdSLICEGET_WrongArgs(t *testing.T) {
	s := store.NewStore()
	ctx := discardCtx("SLICE.GET", bytesArgs(), s)
	if err := cmdSLICEGET(ctx); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCmdSLICEDELETE_Success(t *testing.T) {
	s := store.NewStore()
	_ = cmdSLICECREATE(discardCtx("", bytesArgs("sldel"), s))
	ctx := discardCtx("SLICE.DELETE", bytesArgs("sldel"), s)
	if err := cmdSLICEDELETE(ctx); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCmdSLICEDELETE_WrongArgs(t *testing.T) {
	s := store.NewStore()
	ctx := discardCtx("SLICE.DELETE", bytesArgs(), s)
	if err := cmdSLICEDELETE(ctx); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

// --- ROLLUPX ---

func TestCmdROLLUPXCREATE_Success(t *testing.T) {
	s := store.NewStore()
	ctx := discardCtx("ROLLUPX.CREATE", bytesArgs("ru1"), s)
	if err := cmdROLLUPXCREATE(ctx); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCmdROLLUPXCREATE_WrongArgs(t *testing.T) {
	s := store.NewStore()
	ctx := discardCtx("ROLLUPX.CREATE", bytesArgs(), s)
	if err := cmdROLLUPXCREATE(ctx); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCmdROLLUPXADD_Success(t *testing.T) {
	s := store.NewStore()
	_ = cmdROLLUPXCREATE(discardCtx("", bytesArgs("ruadd"), s))
	ctx := discardCtx("ROLLUPX.ADD", bytesArgs("ruadd", "3.14"), s)
	if err := cmdROLLUPXADD(ctx); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCmdROLLUPXADD_WrongArgs(t *testing.T) {
	s := store.NewStore()
	ctx := discardCtx("ROLLUPX.ADD", bytesArgs("x"), s)
	if err := cmdROLLUPXADD(ctx); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCmdROLLUPXGET_Success(t *testing.T) {
	s := store.NewStore()
	_ = cmdROLLUPXCREATE(discardCtx("", bytesArgs("ruget"), s))
	_ = cmdROLLUPXADD(discardCtx("", bytesArgs("ruget", "10.0"), s))
	ctx := discardCtx("ROLLUPX.GET", bytesArgs("ruget"), s)
	if err := cmdROLLUPXGET(ctx); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCmdROLLUPXGET_WrongArgs(t *testing.T) {
	s := store.NewStore()
	ctx := discardCtx("ROLLUPX.GET", bytesArgs(), s)
	if err := cmdROLLUPXGET(ctx); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCmdROLLUPXDELETE_Success(t *testing.T) {
	s := store.NewStore()
	_ = cmdROLLUPXCREATE(discardCtx("", bytesArgs("rudel"), s))
	ctx := discardCtx("ROLLUPX.DELETE", bytesArgs("rudel"), s)
	if err := cmdROLLUPXDELETE(ctx); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCmdROLLUPXDELETE_WrongArgs(t *testing.T) {
	s := store.NewStore()
	ctx := discardCtx("ROLLUPX.DELETE", bytesArgs(), s)
	if err := cmdROLLUPXDELETE(ctx); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

// --- BEACON ---

func TestCmdBEACONSTART_Success(t *testing.T) {
	s := store.NewStore()
	ctx := discardCtx("BEACON.START", bytesArgs("bc1"), s)
	if err := cmdBEACONSTART(ctx); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCmdBEACONSTART_WrongArgs(t *testing.T) {
	s := store.NewStore()
	ctx := discardCtx("BEACON.START", bytesArgs(), s)
	if err := cmdBEACONSTART(ctx); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCmdBEACONSTOP_Success(t *testing.T) {
	s := store.NewStore()
	_ = cmdBEACONSTART(discardCtx("", bytesArgs("bcstop"), s))
	ctx := discardCtx("BEACON.STOP", bytesArgs("bcstop"), s)
	if err := cmdBEACONSTOP(ctx); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCmdBEACONSTOP_WrongArgs(t *testing.T) {
	s := store.NewStore()
	ctx := discardCtx("BEACON.STOP", bytesArgs(), s)
	if err := cmdBEACONSTOP(ctx); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCmdBEACONLIST_Success(t *testing.T) {
	s := store.NewStore()
	ctx := discardCtx("BEACON.LIST", bytesArgs(), s)
	if err := cmdBEACONLIST(ctx); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCmdBEACONCHECK_Success(t *testing.T) {
	s := store.NewStore()
	_ = cmdBEACONSTART(discardCtx("", bytesArgs("bcchk"), s))
	ctx := discardCtx("BEACON.CHECK", bytesArgs("bcchk"), s)
	if err := cmdBEACONCHECK(ctx); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCmdBEACONCHECK_WrongArgs(t *testing.T) {
	s := store.NewStore()
	ctx := discardCtx("BEACON.CHECK", bytesArgs(), s)
	if err := cmdBEACONCHECK(ctx); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

// ======================================================================
// MORE COMMANDS (more_commands.go)
// ======================================================================

// --- SLIDING ---

func TestCmdSLIDINGCREATE_Success(t *testing.T) {
	s := store.NewStore()
	ctx := discardCtx("SLIDING.CREATE", bytesArgs("sw1", "10", "60000"), s)
	if err := cmdSLIDINGCREATE(ctx); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCmdSLIDINGCREATE_WrongArgs(t *testing.T) {
	s := store.NewStore()
	ctx := discardCtx("SLIDING.CREATE", bytesArgs("sw1", "10"), s)
	if err := cmdSLIDINGCREATE(ctx); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCmdSLIDINGCHECK_Success(t *testing.T) {
	s := store.NewStore()
	_ = cmdSLIDINGCREATE(discardCtx("", bytesArgs("swchk", "100", "60000"), s))
	ctx := discardCtx("SLIDING.CHECK", bytesArgs("swchk"), s)
	if err := cmdSLIDINGCHECK(ctx); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCmdSLIDINGCHECK_WrongArgs(t *testing.T) {
	s := store.NewStore()
	ctx := discardCtx("SLIDING.CHECK", bytesArgs(), s)
	if err := cmdSLIDINGCHECK(ctx); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCmdSLIDINGRESET_Success(t *testing.T) {
	s := store.NewStore()
	_ = cmdSLIDINGCREATE(discardCtx("", bytesArgs("swrst", "100", "60000"), s))
	ctx := discardCtx("SLIDING.RESET", bytesArgs("swrst"), s)
	if err := cmdSLIDINGRESET(ctx); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCmdSLIDINGRESET_WrongArgs(t *testing.T) {
	s := store.NewStore()
	ctx := discardCtx("SLIDING.RESET", bytesArgs(), s)
	if err := cmdSLIDINGRESET(ctx); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCmdSLIDINGDELETE_Success(t *testing.T) {
	s := store.NewStore()
	_ = cmdSLIDINGCREATE(discardCtx("", bytesArgs("swdel", "100", "60000"), s))
	ctx := discardCtx("SLIDING.DELETE", bytesArgs("swdel"), s)
	if err := cmdSLIDINGDELETE(ctx); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCmdSLIDINGDELETE_WrongArgs(t *testing.T) {
	s := store.NewStore()
	ctx := discardCtx("SLIDING.DELETE", bytesArgs(), s)
	if err := cmdSLIDINGDELETE(ctx); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCmdSLIDINGSTATS_Success(t *testing.T) {
	s := store.NewStore()
	_ = cmdSLIDINGCREATE(discardCtx("", bytesArgs("swsts", "100", "60000"), s))
	ctx := discardCtx("SLIDING.STATS", bytesArgs("swsts"), s)
	if err := cmdSLIDINGSTATS(ctx); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCmdSLIDINGSTATS_WrongArgs(t *testing.T) {
	s := store.NewStore()
	ctx := discardCtx("SLIDING.STATS", bytesArgs(), s)
	if err := cmdSLIDINGSTATS(ctx); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

// --- BUCKETX ---

func TestCmdBUCKETXCREATE_Success(t *testing.T) {
	s := store.NewStore()
	ctx := discardCtx("BUCKETX.CREATE", bytesArgs("bx1", "100", "10", "1000"), s)
	if err := cmdBUCKETXCREATE(ctx); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCmdBUCKETXCREATE_WrongArgs(t *testing.T) {
	s := store.NewStore()
	ctx := discardCtx("BUCKETX.CREATE", bytesArgs("bx1", "100", "10"), s)
	if err := cmdBUCKETXCREATE(ctx); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCmdBUCKETXTAKE_Success(t *testing.T) {
	s := store.NewStore()
	_ = cmdBUCKETXCREATE(discardCtx("", bytesArgs("bxtk", "100", "10", "1000"), s))
	ctx := discardCtx("BUCKETX.TAKE", bytesArgs("bxtk", "5"), s)
	if err := cmdBUCKETXTAKE(ctx); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCmdBUCKETXTAKE_WrongArgs(t *testing.T) {
	s := store.NewStore()
	ctx := discardCtx("BUCKETX.TAKE", bytesArgs("x"), s)
	if err := cmdBUCKETXTAKE(ctx); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCmdBUCKETXRETURN_Success(t *testing.T) {
	s := store.NewStore()
	_ = cmdBUCKETXCREATE(discardCtx("", bytesArgs("bxrt", "100", "10", "1000"), s))
	ctx := discardCtx("BUCKETX.RETURN", bytesArgs("bxrt", "5"), s)
	if err := cmdBUCKETXRETURN(ctx); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCmdBUCKETXRETURN_WrongArgs(t *testing.T) {
	s := store.NewStore()
	ctx := discardCtx("BUCKETX.RETURN", bytesArgs("x"), s)
	if err := cmdBUCKETXRETURN(ctx); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCmdBUCKETXREFILL_Success(t *testing.T) {
	s := store.NewStore()
	_ = cmdBUCKETXCREATE(discardCtx("", bytesArgs("bxrf", "100", "10", "1000"), s))
	ctx := discardCtx("BUCKETX.REFILL", bytesArgs("bxrf"), s)
	if err := cmdBUCKETXREFILL(ctx); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCmdBUCKETXREFILL_WrongArgs(t *testing.T) {
	s := store.NewStore()
	ctx := discardCtx("BUCKETX.REFILL", bytesArgs(), s)
	if err := cmdBUCKETXREFILL(ctx); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCmdBUCKETXDELETE_Success(t *testing.T) {
	s := store.NewStore()
	_ = cmdBUCKETXCREATE(discardCtx("", bytesArgs("bxdel", "100", "10", "1000"), s))
	ctx := discardCtx("BUCKETX.DELETE", bytesArgs("bxdel"), s)
	if err := cmdBUCKETXDELETE(ctx); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCmdBUCKETXDELETE_WrongArgs(t *testing.T) {
	s := store.NewStore()
	ctx := discardCtx("BUCKETX.DELETE", bytesArgs(), s)
	if err := cmdBUCKETXDELETE(ctx); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

// --- IDEMPOTENCY ---

func TestCmdIDEMPOTENCYSET_Success(t *testing.T) {
	s := store.NewStore()
	ctx := discardCtx("IDEMPOTENCY.SET", bytesArgs("ik1", "resp1", "60000"), s)
	if err := cmdIDEMPOTENCYSET(ctx); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCmdIDEMPOTENCYSET_WrongArgs(t *testing.T) {
	s := store.NewStore()
	ctx := discardCtx("IDEMPOTENCY.SET", bytesArgs("ik1", "resp1"), s)
	if err := cmdIDEMPOTENCYSET(ctx); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCmdIDEMPOTENCYGET_Success(t *testing.T) {
	s := store.NewStore()
	_ = cmdIDEMPOTENCYSET(discardCtx("", bytesArgs("ikget", "resp", "60000"), s))
	ctx := discardCtx("IDEMPOTENCY.GET", bytesArgs("ikget"), s)
	if err := cmdIDEMPOTENCYGET(ctx); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCmdIDEMPOTENCYGET_WrongArgs(t *testing.T) {
	s := store.NewStore()
	ctx := discardCtx("IDEMPOTENCY.GET", bytesArgs(), s)
	if err := cmdIDEMPOTENCYGET(ctx); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCmdIDEMPOTENCYCHECK_Success(t *testing.T) {
	s := store.NewStore()
	_ = cmdIDEMPOTENCYSET(discardCtx("", bytesArgs("ikchk", "resp", "600000"), s))
	ctx := discardCtx("IDEMPOTENCY.CHECK", bytesArgs("ikchk"), s)
	if err := cmdIDEMPOTENCYCHECK(ctx); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCmdIDEMPOTENCYCHECK_WrongArgs(t *testing.T) {
	s := store.NewStore()
	ctx := discardCtx("IDEMPOTENCY.CHECK", bytesArgs(), s)
	if err := cmdIDEMPOTENCYCHECK(ctx); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCmdIDEMPOTENCYDELETE_Success(t *testing.T) {
	s := store.NewStore()
	_ = cmdIDEMPOTENCYSET(discardCtx("", bytesArgs("ikdel", "resp", "60000"), s))
	ctx := discardCtx("IDEMPOTENCY.DELETE", bytesArgs("ikdel"), s)
	if err := cmdIDEMPOTENCYDELETE(ctx); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCmdIDEMPOTENCYDELETE_WrongArgs(t *testing.T) {
	s := store.NewStore()
	ctx := discardCtx("IDEMPOTENCY.DELETE", bytesArgs(), s)
	if err := cmdIDEMPOTENCYDELETE(ctx); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCmdIDEMPOTENCYLIST_Success(t *testing.T) {
	s := store.NewStore()
	ctx := discardCtx("IDEMPOTENCY.LIST", bytesArgs(), s)
	if err := cmdIDEMPOTENCYLIST(ctx); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

// --- EXPERIMENT ---

func TestCmdEXPERIMENTCREATE_Success(t *testing.T) {
	s := store.NewStore()
	ctx := discardCtx("EXPERIMENT.CREATE", bytesArgs("exp1", "A", "B"), s)
	if err := cmdEXPERIMENTCREATE(ctx); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCmdEXPERIMENTCREATE_WrongArgs(t *testing.T) {
	s := store.NewStore()
	ctx := discardCtx("EXPERIMENT.CREATE", bytesArgs("exp1"), s)
	if err := cmdEXPERIMENTCREATE(ctx); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCmdEXPERIMENTDELETE_Success(t *testing.T) {
	s := store.NewStore()
	_ = cmdEXPERIMENTCREATE(discardCtx("", bytesArgs("expdel", "A"), s))
	ctx := discardCtx("EXPERIMENT.DELETE", bytesArgs("expdel"), s)
	if err := cmdEXPERIMENTDELETE(ctx); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCmdEXPERIMENTDELETE_WrongArgs(t *testing.T) {
	s := store.NewStore()
	ctx := discardCtx("EXPERIMENT.DELETE", bytesArgs(), s)
	if err := cmdEXPERIMENTDELETE(ctx); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCmdEXPERIMENTASSIGN_Success(t *testing.T) {
	s := store.NewStore()
	_ = cmdEXPERIMENTCREATE(discardCtx("", bytesArgs("expass", "A", "B"), s))
	ctx := discardCtx("EXPERIMENT.ASSIGN", bytesArgs("expass", "user1"), s)
	if err := cmdEXPERIMENTASSIGN(ctx); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCmdEXPERIMENTASSIGN_WrongArgs(t *testing.T) {
	s := store.NewStore()
	ctx := discardCtx("EXPERIMENT.ASSIGN", bytesArgs("x"), s)
	if err := cmdEXPERIMENTASSIGN(ctx); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCmdEXPERIMENTTRACK_Success(t *testing.T) {
	s := store.NewStore()
	_ = cmdEXPERIMENTCREATE(discardCtx("", bytesArgs("exptr", "A"), s))
	ctx := discardCtx("EXPERIMENT.TRACK", bytesArgs("exptr", "user1", "click"), s)
	if err := cmdEXPERIMENTTRACK(ctx); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCmdEXPERIMENTTRACK_WrongArgs(t *testing.T) {
	s := store.NewStore()
	ctx := discardCtx("EXPERIMENT.TRACK", bytesArgs("x", "y"), s)
	if err := cmdEXPERIMENTTRACK(ctx); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCmdEXPERIMENTRESULTS_Success(t *testing.T) {
	s := store.NewStore()
	_ = cmdEXPERIMENTCREATE(discardCtx("", bytesArgs("expres", "A"), s))
	ctx := discardCtx("EXPERIMENT.RESULTS", bytesArgs("expres"), s)
	if err := cmdEXPERIMENTRESULTS(ctx); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCmdEXPERIMENTRESULTS_WrongArgs(t *testing.T) {
	s := store.NewStore()
	ctx := discardCtx("EXPERIMENT.RESULTS", bytesArgs(), s)
	if err := cmdEXPERIMENTRESULTS(ctx); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCmdEXPERIMENTLIST_Success(t *testing.T) {
	s := store.NewStore()
	ctx := discardCtx("EXPERIMENT.LIST", bytesArgs(), s)
	if err := cmdEXPERIMENTLIST(ctx); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

// --- ROLLOUT ---

func TestCmdROLLOUTCREATE_Success(t *testing.T) {
	s := store.NewStore()
	ctx := discardCtx("ROLLOUT.CREATE", bytesArgs("ro1", "50"), s)
	if err := cmdROLLOUTCREATE(ctx); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCmdROLLOUTCREATE_WrongArgs(t *testing.T) {
	s := store.NewStore()
	ctx := discardCtx("ROLLOUT.CREATE", bytesArgs(), s)
	if err := cmdROLLOUTCREATE(ctx); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCmdROLLOUTDELETE_Success(t *testing.T) {
	s := store.NewStore()
	_ = cmdROLLOUTCREATE(discardCtx("", bytesArgs("rodel"), s))
	ctx := discardCtx("ROLLOUT.DELETE", bytesArgs("rodel"), s)
	if err := cmdROLLOUTDELETE(ctx); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCmdROLLOUTDELETE_WrongArgs(t *testing.T) {
	s := store.NewStore()
	ctx := discardCtx("ROLLOUT.DELETE", bytesArgs(), s)
	if err := cmdROLLOUTDELETE(ctx); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCmdROLLOUTCHECK_Success(t *testing.T) {
	s := store.NewStore()
	_ = cmdROLLOUTCREATE(discardCtx("", bytesArgs("rochk", "100"), s))
	ctx := discardCtx("ROLLOUT.CHECK", bytesArgs("rochk", "user1"), s)
	if err := cmdROLLOUTCHECK(ctx); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCmdROLLOUTCHECK_WrongArgs(t *testing.T) {
	s := store.NewStore()
	ctx := discardCtx("ROLLOUT.CHECK", bytesArgs("x"), s)
	if err := cmdROLLOUTCHECK(ctx); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCmdROLLOUTPERCENTAGE_Success(t *testing.T) {
	s := store.NewStore()
	_ = cmdROLLOUTCREATE(discardCtx("", bytesArgs("ropct"), s))
	ctx := discardCtx("ROLLOUT.PERCENTAGE", bytesArgs("ropct", "75"), s)
	if err := cmdROLLOUTPERCENTAGE(ctx); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCmdROLLOUTPERCENTAGE_WrongArgs(t *testing.T) {
	s := store.NewStore()
	ctx := discardCtx("ROLLOUT.PERCENTAGE", bytesArgs("x"), s)
	if err := cmdROLLOUTPERCENTAGE(ctx); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCmdROLLOUTLIST_Success(t *testing.T) {
	s := store.NewStore()
	ctx := discardCtx("ROLLOUT.LIST", bytesArgs(), s)
	if err := cmdROLLOUTLIST(ctx); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

// --- SCHEMA ---

func TestCmdSCHEMAREGISTER_Success(t *testing.T) {
	s := store.NewStore()
	ctx := discardCtx("SCHEMA.REGISTER", bytesArgs("sch1", "{\"type\":\"object\"}"), s)
	if err := cmdSCHEMAREGISTER(ctx); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCmdSCHEMAREGISTER_WrongArgs(t *testing.T) {
	s := store.NewStore()
	ctx := discardCtx("SCHEMA.REGISTER", bytesArgs("sch1"), s)
	if err := cmdSCHEMAREGISTER(ctx); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCmdSCHEMAVALIDATE_Success(t *testing.T) {
	s := store.NewStore()
	ctx := discardCtx("SCHEMA.VALIDATE", bytesArgs("sch1", "{\"key\":\"val\"}"), s)
	if err := cmdSCHEMAVALIDATE(ctx); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCmdSCHEMAVALIDATE_WrongArgs(t *testing.T) {
	s := store.NewStore()
	ctx := discardCtx("SCHEMA.VALIDATE", bytesArgs("sch1"), s)
	if err := cmdSCHEMAVALIDATE(ctx); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCmdSCHEMADELETE_Success(t *testing.T) {
	s := store.NewStore()
	_ = cmdSCHEMAREGISTER(discardCtx("", bytesArgs("schdel", "{}"), s))
	ctx := discardCtx("SCHEMA.DELETE", bytesArgs("schdel"), s)
	if err := cmdSCHEMADELETE(ctx); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCmdSCHEMADELETE_WrongArgs(t *testing.T) {
	s := store.NewStore()
	ctx := discardCtx("SCHEMA.DELETE", bytesArgs(), s)
	if err := cmdSCHEMADELETE(ctx); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCmdSCHEMALIST_Success(t *testing.T) {
	s := store.NewStore()
	ctx := discardCtx("SCHEMA.LIST", bytesArgs(), s)
	if err := cmdSCHEMALIST(ctx); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

// --- PIPELINE ---

func TestCmdPIPELINECREATE_Success(t *testing.T) {
	s := store.NewStore()
	ctx := discardCtx("PIPELINE.CREATE", bytesArgs("pipe1"), s)
	if err := cmdPIPELINECREATE(ctx); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCmdPIPELINECREATE_WrongArgs(t *testing.T) {
	s := store.NewStore()
	ctx := discardCtx("PIPELINE.CREATE", bytesArgs(), s)
	if err := cmdPIPELINECREATE(ctx); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCmdPIPELINEADDSTAGE_Success(t *testing.T) {
	s := store.NewStore()
	_ = cmdPIPELINECREATE(discardCtx("", bytesArgs("pipeas"), s))
	ctx := discardCtx("PIPELINE.ADDSTAGE", bytesArgs("pipeas", "stage1"), s)
	if err := cmdPIPELINEADDSTAGE(ctx); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCmdPIPELINEADDSTAGE_WrongArgs(t *testing.T) {
	s := store.NewStore()
	ctx := discardCtx("PIPELINE.ADDSTAGE", bytesArgs("x"), s)
	if err := cmdPIPELINEADDSTAGE(ctx); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCmdPIPELINEEXECUTE_Success(t *testing.T) {
	s := store.NewStore()
	_ = cmdPIPELINECREATE(discardCtx("", bytesArgs("pipeex"), s))
	ctx := discardCtx("PIPELINE.EXECUTE", bytesArgs("pipeex"), s)
	if err := cmdPIPELINEEXECUTE(ctx); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCmdPIPELINEEXECUTE_WrongArgs(t *testing.T) {
	s := store.NewStore()
	ctx := discardCtx("PIPELINE.EXECUTE", bytesArgs(), s)
	if err := cmdPIPELINEEXECUTE(ctx); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCmdPIPELINESTATUS_Success(t *testing.T) {
	s := store.NewStore()
	_ = cmdPIPELINECREATE(discardCtx("", bytesArgs("pipest"), s))
	ctx := discardCtx("PIPELINE.STATUS", bytesArgs("pipest"), s)
	if err := cmdPIPELINESTATUS(ctx); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCmdPIPELINESTATUS_WrongArgs(t *testing.T) {
	s := store.NewStore()
	ctx := discardCtx("PIPELINE.STATUS", bytesArgs(), s)
	if err := cmdPIPELINESTATUS(ctx); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCmdPIPELINEDELETE_Success(t *testing.T) {
	s := store.NewStore()
	_ = cmdPIPELINECREATE(discardCtx("", bytesArgs("pipedel"), s))
	ctx := discardCtx("PIPELINE.DELETE", bytesArgs("pipedel"), s)
	if err := cmdPIPELINEDELETE(ctx); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCmdPIPELINEDELETE_WrongArgs(t *testing.T) {
	s := store.NewStore()
	ctx := discardCtx("PIPELINE.DELETE", bytesArgs(), s)
	if err := cmdPIPELINEDELETE(ctx); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCmdPIPELINELIST_Success(t *testing.T) {
	s := store.NewStore()
	ctx := discardCtx("PIPELINE.LIST", bytesArgs(), s)
	if err := cmdPIPELINELIST(ctx); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

// --- NOTIFY ---

func TestCmdNOTIFYCREATE_Success(t *testing.T) {
	s := store.NewStore()
	ctx := discardCtx("NOTIFY.CREATE", bytesArgs("notif1", "email", "Hello {{name}}"), s)
	if err := cmdNOTIFYCREATE(ctx); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCmdNOTIFYCREATE_WrongArgs(t *testing.T) {
	s := store.NewStore()
	ctx := discardCtx("NOTIFY.CREATE", bytesArgs("notif1", "email"), s)
	if err := cmdNOTIFYCREATE(ctx); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCmdNOTIFYSEND_Success(t *testing.T) {
	s := store.NewStore()
	ctx := discardCtx("NOTIFY.SEND", bytesArgs("notif1", "recipient"), s)
	if err := cmdNOTIFYSEND(ctx); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCmdNOTIFYSEND_WrongArgs(t *testing.T) {
	s := store.NewStore()
	ctx := discardCtx("NOTIFY.SEND", bytesArgs("x"), s)
	if err := cmdNOTIFYSEND(ctx); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCmdNOTIFYLIST_Success(t *testing.T) {
	s := store.NewStore()
	ctx := discardCtx("NOTIFY.LIST", bytesArgs(), s)
	if err := cmdNOTIFYLIST(ctx); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCmdNOTIFYDELETE_Success(t *testing.T) {
	s := store.NewStore()
	_ = cmdNOTIFYCREATE(discardCtx("", bytesArgs("notifdel", "email", "tmpl"), s))
	ctx := discardCtx("NOTIFY.DELETE", bytesArgs("notifdel"), s)
	if err := cmdNOTIFYDELETE(ctx); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCmdNOTIFYDELETE_WrongArgs(t *testing.T) {
	s := store.NewStore()
	ctx := discardCtx("NOTIFY.DELETE", bytesArgs(), s)
	if err := cmdNOTIFYDELETE(ctx); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCmdNOTIFYTEMPLATE_Success(t *testing.T) {
	s := store.NewStore()
	_ = cmdNOTIFYCREATE(discardCtx("", bytesArgs("notiftmpl", "email", "Hello"), s))
	ctx := discardCtx("NOTIFY.TEMPLATE", bytesArgs("notiftmpl"), s)
	if err := cmdNOTIFYTEMPLATE(ctx); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCmdNOTIFYTEMPLATE_WrongArgs(t *testing.T) {
	s := store.NewStore()
	ctx := discardCtx("NOTIFY.TEMPLATE", bytesArgs(), s)
	if err := cmdNOTIFYTEMPLATE(ctx); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

// --- ALERT ---

func TestCmdALERTCREATE_Success(t *testing.T) {
	s := store.NewStore()
	ctx := discardCtx("ALERT.CREATE", bytesArgs("alert1", "high cpu"), s)
	if err := cmdALERTCREATE(ctx); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCmdALERTCREATE_WrongArgs(t *testing.T) {
	s := store.NewStore()
	ctx := discardCtx("ALERT.CREATE", bytesArgs("alert1"), s)
	if err := cmdALERTCREATE(ctx); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCmdALERTTRIGGER_Success(t *testing.T) {
	s := store.NewStore()
	_ = cmdALERTCREATE(discardCtx("", bytesArgs("alerttrg", "msg"), s))
	ctx := discardCtx("ALERT.TRIGGER", bytesArgs("alerttrg"), s)
	if err := cmdALERTTRIGGER(ctx); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCmdALERTTRIGGER_WrongArgs(t *testing.T) {
	s := store.NewStore()
	ctx := discardCtx("ALERT.TRIGGER", bytesArgs(), s)
	if err := cmdALERTTRIGGER(ctx); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCmdALERTACKNOWLEDGE_Success(t *testing.T) {
	s := store.NewStore()
	_ = cmdALERTCREATE(discardCtx("", bytesArgs("alertack", "msg"), s))
	ctx := discardCtx("ALERT.ACKNOWLEDGE", bytesArgs("alertack"), s)
	if err := cmdALERTACKNOWLEDGE(ctx); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCmdALERTACKNOWLEDGE_WrongArgs(t *testing.T) {
	s := store.NewStore()
	ctx := discardCtx("ALERT.ACKNOWLEDGE", bytesArgs(), s)
	if err := cmdALERTACKNOWLEDGE(ctx); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCmdALERTRESOLVE_Success(t *testing.T) {
	s := store.NewStore()
	_ = cmdALERTCREATE(discardCtx("", bytesArgs("alertres", "msg"), s))
	ctx := discardCtx("ALERT.RESOLVE", bytesArgs("alertres"), s)
	if err := cmdALERTRESOLVE(ctx); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCmdALERTRESOLVE_WrongArgs(t *testing.T) {
	s := store.NewStore()
	ctx := discardCtx("ALERT.RESOLVE", bytesArgs(), s)
	if err := cmdALERTRESOLVE(ctx); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCmdALERTLIST_Success(t *testing.T) {
	s := store.NewStore()
	ctx := discardCtx("ALERT.LIST", bytesArgs(), s)
	if err := cmdALERTLIST(ctx); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCmdALERTHISTORY_Success(t *testing.T) {
	s := store.NewStore()
	_ = cmdALERTCREATE(discardCtx("", bytesArgs("alerthist", "msg"), s))
	ctx := discardCtx("ALERT.HISTORY", bytesArgs("alerthist"), s)
	if err := cmdALERTHISTORY(ctx); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCmdALERTHISTORY_WrongArgs(t *testing.T) {
	s := store.NewStore()
	ctx := discardCtx("ALERT.HISTORY", bytesArgs(), s)
	if err := cmdALERTHISTORY(ctx); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

// --- COUNTERX ---

func TestCmdCOUNTERXCREATE_Success(t *testing.T) {
	s := store.NewStore()
	ctx := discardCtx("COUNTERX.CREATE", bytesArgs("cx1"), s)
	if err := cmdCOUNTERXCREATE(ctx); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCmdCOUNTERXCREATE_WrongArgs(t *testing.T) {
	s := store.NewStore()
	ctx := discardCtx("COUNTERX.CREATE", bytesArgs(), s)
	if err := cmdCOUNTERXCREATE(ctx); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCmdCOUNTERXINCR_Success(t *testing.T) {
	s := store.NewStore()
	ctx := discardCtx("COUNTERX.INCR", bytesArgs("cxincr"), s)
	if err := cmdCOUNTERXINCR(ctx); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCmdCOUNTERXINCR_WrongArgs(t *testing.T) {
	s := store.NewStore()
	ctx := discardCtx("COUNTERX.INCR", bytesArgs(), s)
	if err := cmdCOUNTERXINCR(ctx); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCmdCOUNTERXDECR_Success(t *testing.T) {
	s := store.NewStore()
	ctx := discardCtx("COUNTERX.DECR", bytesArgs("cxdecr"), s)
	if err := cmdCOUNTERXDECR(ctx); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCmdCOUNTERXDECR_WrongArgs(t *testing.T) {
	s := store.NewStore()
	ctx := discardCtx("COUNTERX.DECR", bytesArgs(), s)
	if err := cmdCOUNTERXDECR(ctx); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCmdCOUNTERXGET_Success(t *testing.T) {
	s := store.NewStore()
	ctx := discardCtx("COUNTERX.GET", bytesArgs("cxget"), s)
	if err := cmdCOUNTERXGET(ctx); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCmdCOUNTERXGET_WrongArgs(t *testing.T) {
	s := store.NewStore()
	ctx := discardCtx("COUNTERX.GET", bytesArgs(), s)
	if err := cmdCOUNTERXGET(ctx); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCmdCOUNTERXRESET_Success(t *testing.T) {
	s := store.NewStore()
	ctx := discardCtx("COUNTERX.RESET", bytesArgs("cxreset"), s)
	if err := cmdCOUNTERXRESET(ctx); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCmdCOUNTERXRESET_WrongArgs(t *testing.T) {
	s := store.NewStore()
	ctx := discardCtx("COUNTERX.RESET", bytesArgs(), s)
	if err := cmdCOUNTERXRESET(ctx); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCmdCOUNTERXDELETE_Success(t *testing.T) {
	s := store.NewStore()
	_ = cmdCOUNTERXCREATE(discardCtx("", bytesArgs("cxdel"), s))
	ctx := discardCtx("COUNTERX.DELETE", bytesArgs("cxdel"), s)
	if err := cmdCOUNTERXDELETE(ctx); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCmdCOUNTERXDELETE_WrongArgs(t *testing.T) {
	s := store.NewStore()
	ctx := discardCtx("COUNTERX.DELETE", bytesArgs(), s)
	if err := cmdCOUNTERXDELETE(ctx); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

// --- GAUGE ---

func TestCmdGAUGECREATE_Success(t *testing.T) {
	s := store.NewStore()
	ctx := discardCtx("GAUGE.CREATE", bytesArgs("g1"), s)
	if err := cmdGAUGECREATE(ctx); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCmdGAUGECREATE_WrongArgs(t *testing.T) {
	s := store.NewStore()
	ctx := discardCtx("GAUGE.CREATE", bytesArgs(), s)
	if err := cmdGAUGECREATE(ctx); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCmdGAUGESET_Success(t *testing.T) {
	s := store.NewStore()
	ctx := discardCtx("GAUGE.SET", bytesArgs("gset", "42.5"), s)
	if err := cmdGAUGESET(ctx); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCmdGAUGESET_WrongArgs(t *testing.T) {
	s := store.NewStore()
	ctx := discardCtx("GAUGE.SET", bytesArgs("x"), s)
	if err := cmdGAUGESET(ctx); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCmdGAUGEGET_Success(t *testing.T) {
	s := store.NewStore()
	_ = cmdGAUGECREATE(discardCtx("", bytesArgs("gget2"), s))
	ctx := discardCtx("GAUGE.GET", bytesArgs("gget2"), s)
	if err := cmdGAUGEGET(ctx); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCmdGAUGEGET_WrongArgs(t *testing.T) {
	s := store.NewStore()
	ctx := discardCtx("GAUGE.GET", bytesArgs(), s)
	if err := cmdGAUGEGET(ctx); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCmdGAUGEINCR_Success(t *testing.T) {
	s := store.NewStore()
	ctx := discardCtx("GAUGE.INCR", bytesArgs("gincr"), s)
	if err := cmdGAUGEINCR(ctx); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCmdGAUGEINCR_WrongArgs(t *testing.T) {
	s := store.NewStore()
	ctx := discardCtx("GAUGE.INCR", bytesArgs(), s)
	if err := cmdGAUGEINCR(ctx); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCmdGAUGEDECR_Success(t *testing.T) {
	s := store.NewStore()
	ctx := discardCtx("GAUGE.DECR", bytesArgs("gdecr"), s)
	if err := cmdGAUGEDECR(ctx); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCmdGAUGEDECR_WrongArgs(t *testing.T) {
	s := store.NewStore()
	ctx := discardCtx("GAUGE.DECR", bytesArgs(), s)
	if err := cmdGAUGEDECR(ctx); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCmdGAUGEDELETE_Success(t *testing.T) {
	s := store.NewStore()
	_ = cmdGAUGECREATE(discardCtx("", bytesArgs("gdel2"), s))
	ctx := discardCtx("GAUGE.DELETE", bytesArgs("gdel2"), s)
	if err := cmdGAUGEDELETE(ctx); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCmdGAUGEDELETE_WrongArgs(t *testing.T) {
	s := store.NewStore()
	ctx := discardCtx("GAUGE.DELETE", bytesArgs(), s)
	if err := cmdGAUGEDELETE(ctx); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

// --- TRACE ---

func TestCmdTRACESTART_Success(t *testing.T) {
	s := store.NewStore()
	ctx := discardCtx("TRACE.START", bytesArgs("tr1"), s)
	if err := cmdTRACESTART(ctx); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCmdTRACESTART_WrongArgs(t *testing.T) {
	s := store.NewStore()
	ctx := discardCtx("TRACE.START", bytesArgs(), s)
	if err := cmdTRACESTART(ctx); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCmdTRACESPAN_Success(t *testing.T) {
	s := store.NewStore()
	_ = cmdTRACESTART(discardCtx("", bytesArgs("trspan"), s))
	ctx := discardCtx("TRACE.SPAN", bytesArgs("trspan", "span1", "s1"), s)
	if err := cmdTRACESPAN(ctx); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCmdTRACESPAN_WrongArgs(t *testing.T) {
	s := store.NewStore()
	ctx := discardCtx("TRACE.SPAN", bytesArgs("x", "y"), s)
	if err := cmdTRACESPAN(ctx); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCmdTRACEEND_Success(t *testing.T) {
	s := store.NewStore()
	_ = cmdTRACESTART(discardCtx("", bytesArgs("trend"), s))
	ctx := discardCtx("TRACE.END", bytesArgs("trend"), s)
	if err := cmdTRACEEND(ctx); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCmdTRACEEND_WrongArgs(t *testing.T) {
	s := store.NewStore()
	ctx := discardCtx("TRACE.END", bytesArgs(), s)
	if err := cmdTRACEEND(ctx); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCmdTRACEGET_Success(t *testing.T) {
	s := store.NewStore()
	_ = cmdTRACESTART(discardCtx("", bytesArgs("trget"), s))
	ctx := discardCtx("TRACE.GET", bytesArgs("trget"), s)
	if err := cmdTRACEGET(ctx); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCmdTRACEGET_WrongArgs(t *testing.T) {
	s := store.NewStore()
	ctx := discardCtx("TRACE.GET", bytesArgs(), s)
	if err := cmdTRACEGET(ctx); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCmdTRACELIST_Success(t *testing.T) {
	s := store.NewStore()
	ctx := discardCtx("TRACE.LIST", bytesArgs(), s)
	if err := cmdTRACELIST(ctx); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

// --- LOGX ---

func TestCmdLOGXWRITE_Success(t *testing.T) {
	s := store.NewStore()
	ctx := discardCtx("LOGX.WRITE", bytesArgs("log1", "INFO", "test message"), s)
	if err := cmdLOGXWRITE(ctx); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCmdLOGXWRITE_WrongArgs(t *testing.T) {
	s := store.NewStore()
	ctx := discardCtx("LOGX.WRITE", bytesArgs("log1", "INFO"), s)
	if err := cmdLOGXWRITE(ctx); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCmdLOGXREAD_Success(t *testing.T) {
	s := store.NewStore()
	_ = cmdLOGXWRITE(discardCtx("", bytesArgs("logrd", "INFO", "msg"), s))
	ctx := discardCtx("LOGX.READ", bytesArgs("logrd"), s)
	if err := cmdLOGXREAD(ctx); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCmdLOGXREAD_WrongArgs(t *testing.T) {
	s := store.NewStore()
	ctx := discardCtx("LOGX.READ", bytesArgs(), s)
	if err := cmdLOGXREAD(ctx); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCmdLOGXSEARCH_Success(t *testing.T) {
	s := store.NewStore()
	_ = cmdLOGXWRITE(discardCtx("", bytesArgs("logsr", "INFO", "hello world"), s))
	ctx := discardCtx("LOGX.SEARCH", bytesArgs("logsr", "hello"), s)
	if err := cmdLOGXSEARCH(ctx); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCmdLOGXSEARCH_WrongArgs(t *testing.T) {
	s := store.NewStore()
	ctx := discardCtx("LOGX.SEARCH", bytesArgs("x"), s)
	if err := cmdLOGXSEARCH(ctx); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCmdLOGXCLEAR_Success(t *testing.T) {
	s := store.NewStore()
	ctx := discardCtx("LOGX.CLEAR", bytesArgs("logcl"), s)
	if err := cmdLOGXCLEAR(ctx); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCmdLOGXCLEAR_WrongArgs(t *testing.T) {
	s := store.NewStore()
	ctx := discardCtx("LOGX.CLEAR", bytesArgs(), s)
	if err := cmdLOGXCLEAR(ctx); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCmdLOGXSTATS_Success(t *testing.T) {
	s := store.NewStore()
	ctx := discardCtx("LOGX.STATS", bytesArgs("logsts"), s)
	if err := cmdLOGXSTATS(ctx); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCmdLOGXSTATS_WrongArgs(t *testing.T) {
	s := store.NewStore()
	ctx := discardCtx("LOGX.STATS", bytesArgs(), s)
	if err := cmdLOGXSTATS(ctx); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

// --- APIKEY ---

func TestCmdAPIKEYCREATE_Success(t *testing.T) {
	s := store.NewStore()
	ctx := discardCtx("APIKEY.CREATE", bytesArgs("mykey"), s)
	if err := cmdAPIKEYCREATE(ctx); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCmdAPIKEYCREATE_WrongArgs(t *testing.T) {
	s := store.NewStore()
	ctx := discardCtx("APIKEY.CREATE", bytesArgs(), s)
	if err := cmdAPIKEYCREATE(ctx); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCmdAPIKEYVALIDATE_Success(t *testing.T) {
	s := store.NewStore()
	ctx := discardCtx("APIKEY.VALIDATE", bytesArgs("somekey"), s)
	if err := cmdAPIKEYVALIDATE(ctx); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCmdAPIKEYVALIDATE_WrongArgs(t *testing.T) {
	s := store.NewStore()
	ctx := discardCtx("APIKEY.VALIDATE", bytesArgs(), s)
	if err := cmdAPIKEYVALIDATE(ctx); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCmdAPIKEYREVOKE_Success(t *testing.T) {
	s := store.NewStore()
	ctx := discardCtx("APIKEY.REVOKE", bytesArgs("somekey"), s)
	if err := cmdAPIKEYREVOKE(ctx); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCmdAPIKEYREVOKE_WrongArgs(t *testing.T) {
	s := store.NewStore()
	ctx := discardCtx("APIKEY.REVOKE", bytesArgs(), s)
	if err := cmdAPIKEYREVOKE(ctx); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCmdAPIKEYLIST_Success(t *testing.T) {
	s := store.NewStore()
	ctx := discardCtx("APIKEY.LIST", bytesArgs(), s)
	if err := cmdAPIKEYLIST(ctx); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCmdAPIKEYUSAGE_WrongArgs(t *testing.T) {
	s := store.NewStore()
	ctx := discardCtx("APIKEY.USAGE", bytesArgs(), s)
	if err := cmdAPIKEYUSAGE(ctx); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

// --- QUOTAX ---

func TestCmdQUOTAXCREATE_Success(t *testing.T) {
	s := store.NewStore()
	ctx := discardCtx("QUOTAX.CREATE", bytesArgs("q1", "100", "60000"), s)
	if err := cmdQUOTAXCREATE(ctx); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCmdQUOTAXCREATE_WrongArgs(t *testing.T) {
	s := store.NewStore()
	ctx := discardCtx("QUOTAX.CREATE", bytesArgs("q1", "100"), s)
	if err := cmdQUOTAXCREATE(ctx); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCmdQUOTAXCHECK_Success(t *testing.T) {
	s := store.NewStore()
	_ = cmdQUOTAXCREATE(discardCtx("", bytesArgs("qchk", "100", "60000"), s))
	ctx := discardCtx("QUOTAX.CHECK", bytesArgs("qchk", "10"), s)
	if err := cmdQUOTAXCHECK(ctx); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCmdQUOTAXCHECK_WrongArgs(t *testing.T) {
	s := store.NewStore()
	ctx := discardCtx("QUOTAX.CHECK", bytesArgs("x"), s)
	if err := cmdQUOTAXCHECK(ctx); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCmdQUOTAXUSAGE_Success(t *testing.T) {
	s := store.NewStore()
	_ = cmdQUOTAXCREATE(discardCtx("", bytesArgs("qusg", "100", "60000"), s))
	ctx := discardCtx("QUOTAX.USAGE", bytesArgs("qusg"), s)
	if err := cmdQUOTAXUSAGE(ctx); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCmdQUOTAXUSAGE_WrongArgs(t *testing.T) {
	s := store.NewStore()
	ctx := discardCtx("QUOTAX.USAGE", bytesArgs(), s)
	if err := cmdQUOTAXUSAGE(ctx); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCmdQUOTAXRESET_Success(t *testing.T) {
	s := store.NewStore()
	_ = cmdQUOTAXCREATE(discardCtx("", bytesArgs("qrst", "100", "60000"), s))
	ctx := discardCtx("QUOTAX.RESET", bytesArgs("qrst"), s)
	if err := cmdQUOTAXRESET(ctx); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCmdQUOTAXRESET_WrongArgs(t *testing.T) {
	s := store.NewStore()
	ctx := discardCtx("QUOTAX.RESET", bytesArgs(), s)
	if err := cmdQUOTAXRESET(ctx); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCmdQUOTAXDELETE_Success(t *testing.T) {
	s := store.NewStore()
	_ = cmdQUOTAXCREATE(discardCtx("", bytesArgs("qdel", "100", "60000"), s))
	ctx := discardCtx("QUOTAX.DELETE", bytesArgs("qdel"), s)
	if err := cmdQUOTAXDELETE(ctx); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCmdQUOTAXDELETE_WrongArgs(t *testing.T) {
	s := store.NewStore()
	ctx := discardCtx("QUOTAX.DELETE", bytesArgs(), s)
	if err := cmdQUOTAXDELETE(ctx); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

// --- METER ---

func TestCmdMETERCREATE_Success(t *testing.T) {
	s := store.NewStore()
	ctx := discardCtx("METER.CREATE", bytesArgs("m1", "requests"), s)
	if err := cmdMETERCREATE(ctx); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCmdMETERCREATE_WrongArgs(t *testing.T) {
	s := store.NewStore()
	ctx := discardCtx("METER.CREATE", bytesArgs("m1"), s)
	if err := cmdMETERCREATE(ctx); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCmdMETERRECORD_Success(t *testing.T) {
	s := store.NewStore()
	_ = cmdMETERCREATE(discardCtx("", bytesArgs("mrec", "req"), s))
	ctx := discardCtx("METER.RECORD", bytesArgs("mrec", "5"), s)
	if err := cmdMETERRECORD(ctx); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCmdMETERRECORD_WrongArgs(t *testing.T) {
	s := store.NewStore()
	ctx := discardCtx("METER.RECORD", bytesArgs("x"), s)
	if err := cmdMETERRECORD(ctx); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCmdMETERGET_Success(t *testing.T) {
	s := store.NewStore()
	_ = cmdMETERCREATE(discardCtx("", bytesArgs("mget", "req"), s))
	ctx := discardCtx("METER.GET", bytesArgs("mget"), s)
	if err := cmdMETERGET(ctx); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCmdMETERGET_WrongArgs(t *testing.T) {
	s := store.NewStore()
	ctx := discardCtx("METER.GET", bytesArgs(), s)
	if err := cmdMETERGET(ctx); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCmdMETERBILLING_Success(t *testing.T) {
	s := store.NewStore()
	_ = cmdMETERCREATE(discardCtx("", bytesArgs("mbill", "req"), s))
	ctx := discardCtx("METER.BILLING", bytesArgs("mbill"), s)
	if err := cmdMETERBILLING(ctx); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCmdMETERBILLING_WrongArgs(t *testing.T) {
	s := store.NewStore()
	ctx := discardCtx("METER.BILLING", bytesArgs(), s)
	if err := cmdMETERBILLING(ctx); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCmdMETERDELETE_Success(t *testing.T) {
	s := store.NewStore()
	_ = cmdMETERCREATE(discardCtx("", bytesArgs("mdel", "req"), s))
	ctx := discardCtx("METER.DELETE", bytesArgs("mdel"), s)
	if err := cmdMETERDELETE(ctx); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCmdMETERDELETE_WrongArgs(t *testing.T) {
	s := store.NewStore()
	ctx := discardCtx("METER.DELETE", bytesArgs(), s)
	if err := cmdMETERDELETE(ctx); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

// --- TENANT ---

func TestCmdTENANTCREATE_Success(t *testing.T) {
	s := store.NewStore()
	ctx := discardCtx("TENANT.CREATE", bytesArgs("t1", "Acme"), s)
	if err := cmdTENANTCREATE(ctx); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCmdTENANTCREATE_WrongArgs(t *testing.T) {
	s := store.NewStore()
	ctx := discardCtx("TENANT.CREATE", bytesArgs("t1"), s)
	if err := cmdTENANTCREATE(ctx); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCmdTENANTDELETE_Success(t *testing.T) {
	s := store.NewStore()
	_ = cmdTENANTCREATE(discardCtx("", bytesArgs("tdel", "Acme"), s))
	ctx := discardCtx("TENANT.DELETE", bytesArgs("tdel"), s)
	if err := cmdTENANTDELETE(ctx); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCmdTENANTDELETE_WrongArgs(t *testing.T) {
	s := store.NewStore()
	ctx := discardCtx("TENANT.DELETE", bytesArgs(), s)
	if err := cmdTENANTDELETE(ctx); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCmdTENANTGET_Success(t *testing.T) {
	s := store.NewStore()
	_ = cmdTENANTCREATE(discardCtx("", bytesArgs("tget", "Acme"), s))
	ctx := discardCtx("TENANT.GET", bytesArgs("tget"), s)
	if err := cmdTENANTGET(ctx); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCmdTENANTGET_WrongArgs(t *testing.T) {
	s := store.NewStore()
	ctx := discardCtx("TENANT.GET", bytesArgs(), s)
	if err := cmdTENANTGET(ctx); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCmdTENANTLIST_Success(t *testing.T) {
	s := store.NewStore()
	ctx := discardCtx("TENANT.LIST", bytesArgs(), s)
	if err := cmdTENANTLIST(ctx); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCmdTENANTCONFIG_Success(t *testing.T) {
	s := store.NewStore()
	_ = cmdTENANTCREATE(discardCtx("", bytesArgs("tcfg", "Acme"), s))
	ctx := discardCtx("TENANT.CONFIG", bytesArgs("tcfg", "key", "val"), s)
	if err := cmdTENANTCONFIG(ctx); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCmdTENANTCONFIG_WrongArgs(t *testing.T) {
	s := store.NewStore()
	ctx := discardCtx("TENANT.CONFIG", bytesArgs("t1", "key"), s)
	if err := cmdTENANTCONFIG(ctx); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

// --- LEASE ---

func TestCmdLEASECREATE_Success(t *testing.T) {
	s := store.NewStore()
	ctx := discardCtx("LEASE.CREATE", bytesArgs("ls1", "holder1", "60000"), s)
	if err := cmdLEASECREATE(ctx); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCmdLEASECREATE_WrongArgs(t *testing.T) {
	s := store.NewStore()
	ctx := discardCtx("LEASE.CREATE", bytesArgs("ls1", "holder1"), s)
	if err := cmdLEASECREATE(ctx); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCmdLEASERENEW_Success(t *testing.T) {
	s := store.NewStore()
	_ = cmdLEASECREATE(discardCtx("", bytesArgs("lsrnw", "h", "60000"), s))
	ctx := discardCtx("LEASE.RENEW", bytesArgs("lsrnw", "120000"), s)
	if err := cmdLEASERENEW(ctx); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCmdLEASERENEW_WrongArgs(t *testing.T) {
	s := store.NewStore()
	ctx := discardCtx("LEASE.RENEW", bytesArgs("x"), s)
	if err := cmdLEASERENEW(ctx); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCmdLEASEREVOKE_Success(t *testing.T) {
	s := store.NewStore()
	_ = cmdLEASECREATE(discardCtx("", bytesArgs("lsrev", "h", "60000"), s))
	ctx := discardCtx("LEASE.REVOKE", bytesArgs("lsrev"), s)
	if err := cmdLEASEREVOKE(ctx); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCmdLEASEREVOKE_WrongArgs(t *testing.T) {
	s := store.NewStore()
	ctx := discardCtx("LEASE.REVOKE", bytesArgs(), s)
	if err := cmdLEASEREVOKE(ctx); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCmdLEASEGET_Success(t *testing.T) {
	s := store.NewStore()
	_ = cmdLEASECREATE(discardCtx("", bytesArgs("lsget", "h", "60000"), s))
	ctx := discardCtx("LEASE.GET", bytesArgs("lsget"), s)
	if err := cmdLEASEGET(ctx); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCmdLEASEGET_WrongArgs(t *testing.T) {
	s := store.NewStore()
	ctx := discardCtx("LEASE.GET", bytesArgs(), s)
	if err := cmdLEASEGET(ctx); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCmdLEASELIST_Success(t *testing.T) {
	s := store.NewStore()
	ctx := discardCtx("LEASE.LIST", bytesArgs(), s)
	if err := cmdLEASELIST(ctx); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

// --- HEAP ---

func TestCmdHEAPPUSH_Success(t *testing.T) {
	s := store.NewStore()
	ctx := discardCtx("HEAP.PUSH", bytesArgs("hp1", "42"), s)
	if err := cmdHEAPPUSH(ctx); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCmdHEAPPUSH_WrongArgs(t *testing.T) {
	s := store.NewStore()
	ctx := discardCtx("HEAP.PUSH", bytesArgs("hp1"), s)
	if err := cmdHEAPPUSH(ctx); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCmdHEAPPOP_Success(t *testing.T) {
	s := store.NewStore()
	_ = cmdHEAPPUSH(discardCtx("", bytesArgs("hppop", "10"), s))
	_ = cmdHEAPPUSH(discardCtx("", bytesArgs("hppop", "5"), s))
	ctx := discardCtx("HEAP.POP", bytesArgs("hppop"), s)
	if err := cmdHEAPPOP(ctx); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCmdHEAPPOP_WrongArgs(t *testing.T) {
	s := store.NewStore()
	ctx := discardCtx("HEAP.POP", bytesArgs(), s)
	if err := cmdHEAPPOP(ctx); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCmdHEAPPEEK_Success(t *testing.T) {
	s := store.NewStore()
	_ = cmdHEAPPUSH(discardCtx("", bytesArgs("hppeek", "7"), s))
	ctx := discardCtx("HEAP.PEEK", bytesArgs("hppeek"), s)
	if err := cmdHEAPPEEK(ctx); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCmdHEAPPEEK_WrongArgs(t *testing.T) {
	s := store.NewStore()
	ctx := discardCtx("HEAP.PEEK", bytesArgs(), s)
	if err := cmdHEAPPEEK(ctx); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCmdHEAPSIZE_Success(t *testing.T) {
	s := store.NewStore()
	ctx := discardCtx("HEAP.SIZE", bytesArgs("hpsize"), s)
	if err := cmdHEAPSIZE(ctx); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCmdHEAPSIZE_WrongArgs(t *testing.T) {
	s := store.NewStore()
	ctx := discardCtx("HEAP.SIZE", bytesArgs(), s)
	if err := cmdHEAPSIZE(ctx); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCmdHEAPDELETE_Success(t *testing.T) {
	s := store.NewStore()
	_ = cmdHEAPPUSH(discardCtx("", bytesArgs("hpdel", "1"), s))
	ctx := discardCtx("HEAP.DELETE", bytesArgs("hpdel"), s)
	if err := cmdHEAPDELETE(ctx); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCmdHEAPDELETE_WrongArgs(t *testing.T) {
	s := store.NewStore()
	ctx := discardCtx("HEAP.DELETE", bytesArgs(), s)
	if err := cmdHEAPDELETE(ctx); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

// --- BLOOMX ---

func TestCmdBLOOMXCREATE_Success(t *testing.T) {
	s := store.NewStore()
	ctx := discardCtx("BLOOMX.CREATE", bytesArgs("bf1", "1000", "3"), s)
	if err := cmdBLOOMXCREATE(ctx); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCmdBLOOMXCREATE_WrongArgs(t *testing.T) {
	s := store.NewStore()
	ctx := discardCtx("BLOOMX.CREATE", bytesArgs("bf1", "1000"), s)
	if err := cmdBLOOMXCREATE(ctx); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCmdBLOOMXADD_Success(t *testing.T) {
	s := store.NewStore()
	_ = cmdBLOOMXCREATE(discardCtx("", bytesArgs("bfadd", "1000", "3"), s))
	ctx := discardCtx("BLOOMX.ADD", bytesArgs("bfadd", "hello"), s)
	if err := cmdBLOOMXADD(ctx); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCmdBLOOMXADD_WrongArgs(t *testing.T) {
	s := store.NewStore()
	ctx := discardCtx("BLOOMX.ADD", bytesArgs("x"), s)
	if err := cmdBLOOMXADD(ctx); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCmdBLOOMXCHECK_Success(t *testing.T) {
	s := store.NewStore()
	_ = cmdBLOOMXCREATE(discardCtx("", bytesArgs("bfchk", "1000", "3"), s))
	_ = cmdBLOOMXADD(discardCtx("", bytesArgs("bfchk", "hello"), s))
	ctx := discardCtx("BLOOMX.CHECK", bytesArgs("bfchk", "hello"), s)
	if err := cmdBLOOMXCHECK(ctx); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCmdBLOOMXCHECK_WrongArgs(t *testing.T) {
	s := store.NewStore()
	ctx := discardCtx("BLOOMX.CHECK", bytesArgs("x"), s)
	if err := cmdBLOOMXCHECK(ctx); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCmdBLOOMXINFO_Success(t *testing.T) {
	s := store.NewStore()
	_ = cmdBLOOMXCREATE(discardCtx("", bytesArgs("bfinfo", "1000", "3"), s))
	ctx := discardCtx("BLOOMX.INFO", bytesArgs("bfinfo"), s)
	if err := cmdBLOOMXINFO(ctx); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCmdBLOOMXINFO_WrongArgs(t *testing.T) {
	s := store.NewStore()
	ctx := discardCtx("BLOOMX.INFO", bytesArgs(), s)
	if err := cmdBLOOMXINFO(ctx); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCmdBLOOMXDELETE_Success(t *testing.T) {
	s := store.NewStore()
	_ = cmdBLOOMXCREATE(discardCtx("", bytesArgs("bfdel", "1000", "3"), s))
	ctx := discardCtx("BLOOMX.DELETE", bytesArgs("bfdel"), s)
	if err := cmdBLOOMXDELETE(ctx); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCmdBLOOMXDELETE_WrongArgs(t *testing.T) {
	s := store.NewStore()
	ctx := discardCtx("BLOOMX.DELETE", bytesArgs(), s)
	if err := cmdBLOOMXDELETE(ctx); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

// --- SKETCH ---

func TestCmdSKETCHCREATE_Success(t *testing.T) {
	s := store.NewStore()
	ctx := discardCtx("SKETCH.CREATE", bytesArgs("sk1", "100", "5"), s)
	if err := cmdSKETCHCREATE(ctx); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCmdSKETCHCREATE_WrongArgs(t *testing.T) {
	s := store.NewStore()
	ctx := discardCtx("SKETCH.CREATE", bytesArgs("sk1", "100"), s)
	if err := cmdSKETCHCREATE(ctx); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCmdSKETCHUPDATE_Success(t *testing.T) {
	s := store.NewStore()
	_ = cmdSKETCHCREATE(discardCtx("", bytesArgs("skupd", "100", "5"), s))
	ctx := discardCtx("SKETCH.UPDATE", bytesArgs("skupd", "item1", "3"), s)
	if err := cmdSKETCHUPDATE(ctx); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCmdSKETCHUPDATE_WrongArgs(t *testing.T) {
	s := store.NewStore()
	ctx := discardCtx("SKETCH.UPDATE", bytesArgs("sk1", "item1"), s)
	if err := cmdSKETCHUPDATE(ctx); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCmdSKETCHQUERY_Success(t *testing.T) {
	s := store.NewStore()
	_ = cmdSKETCHCREATE(discardCtx("", bytesArgs("skqry", "100", "5"), s))
	_ = cmdSKETCHUPDATE(discardCtx("", bytesArgs("skqry", "item1", "3"), s))
	ctx := discardCtx("SKETCH.QUERY", bytesArgs("skqry", "item1"), s)
	if err := cmdSKETCHQUERY(ctx); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCmdSKETCHQUERY_WrongArgs(t *testing.T) {
	s := store.NewStore()
	ctx := discardCtx("SKETCH.QUERY", bytesArgs("x"), s)
	if err := cmdSKETCHQUERY(ctx); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCmdSKETCHMERGE_Success(t *testing.T) {
	s := store.NewStore()
	ctx := discardCtx("SKETCH.MERGE", bytesArgs("dest", "src1", "src2"), s)
	if err := cmdSKETCHMERGE(ctx); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCmdSKETCHMERGE_WrongArgs(t *testing.T) {
	s := store.NewStore()
	ctx := discardCtx("SKETCH.MERGE", bytesArgs("dest", "src1"), s)
	if err := cmdSKETCHMERGE(ctx); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCmdSKETCHDELETE_Success(t *testing.T) {
	s := store.NewStore()
	_ = cmdSKETCHCREATE(discardCtx("", bytesArgs("skdel", "100", "5"), s))
	ctx := discardCtx("SKETCH.DELETE", bytesArgs("skdel"), s)
	if err := cmdSKETCHDELETE(ctx); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCmdSKETCHDELETE_WrongArgs(t *testing.T) {
	s := store.NewStore()
	ctx := discardCtx("SKETCH.DELETE", bytesArgs(), s)
	if err := cmdSKETCHDELETE(ctx); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

// --- RINGBUFFER ---

func TestCmdRINGBUFFERCREATE_Success(t *testing.T) {
	s := store.NewStore()
	ctx := discardCtx("RINGBUFFER.CREATE", bytesArgs("rb1", "10"), s)
	if err := cmdRINGBUFFERCREATE(ctx); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCmdRINGBUFFERCREATE_WrongArgs(t *testing.T) {
	s := store.NewStore()
	ctx := discardCtx("RINGBUFFER.CREATE", bytesArgs("rb1"), s)
	if err := cmdRINGBUFFERCREATE(ctx); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCmdRINGBUFFERWRITE_Success(t *testing.T) {
	s := store.NewStore()
	_ = cmdRINGBUFFERCREATE(discardCtx("", bytesArgs("rbw", "10"), s))
	ctx := discardCtx("RINGBUFFER.WRITE", bytesArgs("rbw", "val1"), s)
	if err := cmdRINGBUFFERWRITE(ctx); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCmdRINGBUFFERWRITE_WrongArgs(t *testing.T) {
	s := store.NewStore()
	ctx := discardCtx("RINGBUFFER.WRITE", bytesArgs("x"), s)
	if err := cmdRINGBUFFERWRITE(ctx); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCmdRINGBUFFERREAD_Success(t *testing.T) {
	s := store.NewStore()
	_ = cmdRINGBUFFERCREATE(discardCtx("", bytesArgs("rbr", "10"), s))
	_ = cmdRINGBUFFERWRITE(discardCtx("", bytesArgs("rbr", "val1"), s))
	ctx := discardCtx("RINGBUFFER.READ", bytesArgs("rbr"), s)
	if err := cmdRINGBUFFERREAD(ctx); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCmdRINGBUFFERREAD_WrongArgs(t *testing.T) {
	s := store.NewStore()
	ctx := discardCtx("RINGBUFFER.READ", bytesArgs(), s)
	if err := cmdRINGBUFFERREAD(ctx); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCmdRINGBUFFERSIZE_Success(t *testing.T) {
	s := store.NewStore()
	_ = cmdRINGBUFFERCREATE(discardCtx("", bytesArgs("rbsz", "10"), s))
	ctx := discardCtx("RINGBUFFER.SIZE", bytesArgs("rbsz"), s)
	if err := cmdRINGBUFFERSIZE(ctx); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCmdRINGBUFFERSIZE_WrongArgs(t *testing.T) {
	s := store.NewStore()
	ctx := discardCtx("RINGBUFFER.SIZE", bytesArgs(), s)
	if err := cmdRINGBUFFERSIZE(ctx); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCmdRINGBUFFERDELETE_Success(t *testing.T) {
	s := store.NewStore()
	_ = cmdRINGBUFFERCREATE(discardCtx("", bytesArgs("rbdel", "10"), s))
	ctx := discardCtx("RINGBUFFER.DELETE", bytesArgs("rbdel"), s)
	if err := cmdRINGBUFFERDELETE(ctx); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCmdRINGBUFFERDELETE_WrongArgs(t *testing.T) {
	s := store.NewStore()
	ctx := discardCtx("RINGBUFFER.DELETE", bytesArgs(), s)
	if err := cmdRINGBUFFERDELETE(ctx); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

// --- WINDOW ---

func TestCmdWINDOWCREATE_Success(t *testing.T) {
	s := store.NewStore()
	ctx := discardCtx("WINDOW.CREATE", bytesArgs("win1", "10"), s)
	if err := cmdWINDOWCREATE(ctx); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCmdWINDOWCREATE_WrongArgs(t *testing.T) {
	s := store.NewStore()
	ctx := discardCtx("WINDOW.CREATE", bytesArgs("win1"), s)
	if err := cmdWINDOWCREATE(ctx); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCmdWINDOWADD_Success(t *testing.T) {
	s := store.NewStore()
	_ = cmdWINDOWCREATE(discardCtx("", bytesArgs("winadd", "10"), s))
	ctx := discardCtx("WINDOW.ADD", bytesArgs("winadd", "3.14"), s)
	if err := cmdWINDOWADD(ctx); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCmdWINDOWADD_WrongArgs(t *testing.T) {
	s := store.NewStore()
	ctx := discardCtx("WINDOW.ADD", bytesArgs("x"), s)
	if err := cmdWINDOWADD(ctx); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCmdWINDOWGET_Success(t *testing.T) {
	s := store.NewStore()
	_ = cmdWINDOWCREATE(discardCtx("", bytesArgs("winget", "10"), s))
	ctx := discardCtx("WINDOW.GET", bytesArgs("winget"), s)
	if err := cmdWINDOWGET(ctx); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCmdWINDOWGET_WrongArgs(t *testing.T) {
	s := store.NewStore()
	ctx := discardCtx("WINDOW.GET", bytesArgs(), s)
	if err := cmdWINDOWGET(ctx); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCmdWINDOWAGGREGATE_Sum(t *testing.T) {
	s := store.NewStore()
	_ = cmdWINDOWCREATE(discardCtx("", bytesArgs("winagg", "10"), s))
	_ = cmdWINDOWADD(discardCtx("", bytesArgs("winagg", "10.0"), s))
	ctx := discardCtx("WINDOW.AGGREGATE", bytesArgs("winagg", "sum"), s)
	if err := cmdWINDOWAGGREGATE(ctx); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCmdWINDOWAGGREGATE_Avg(t *testing.T) {
	s := store.NewStore()
	_ = cmdWINDOWCREATE(discardCtx("", bytesArgs("winagga", "10"), s))
	_ = cmdWINDOWADD(discardCtx("", bytesArgs("winagga", "10.0"), s))
	ctx := discardCtx("WINDOW.AGGREGATE", bytesArgs("winagga", "avg"), s)
	if err := cmdWINDOWAGGREGATE(ctx); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCmdWINDOWAGGREGATE_MinMax(t *testing.T) {
	s := store.NewStore()
	_ = cmdWINDOWCREATE(discardCtx("", bytesArgs("winaggm", "10"), s))
	_ = cmdWINDOWADD(discardCtx("", bytesArgs("winaggm", "5.0"), s))
	_ = cmdWINDOWADD(discardCtx("", bytesArgs("winaggm", "15.0"), s))
	ctx := discardCtx("WINDOW.AGGREGATE", bytesArgs("winaggm", "min"), s)
	if err := cmdWINDOWAGGREGATE(ctx); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	ctx2 := discardCtx("WINDOW.AGGREGATE", bytesArgs("winaggm", "max"), s)
	if err := cmdWINDOWAGGREGATE(ctx2); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCmdWINDOWAGGREGATE_WrongArgs(t *testing.T) {
	s := store.NewStore()
	ctx := discardCtx("WINDOW.AGGREGATE", bytesArgs("x"), s)
	if err := cmdWINDOWAGGREGATE(ctx); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCmdWINDOWDELETE_Success(t *testing.T) {
	s := store.NewStore()
	_ = cmdWINDOWCREATE(discardCtx("", bytesArgs("windel", "10"), s))
	ctx := discardCtx("WINDOW.DELETE", bytesArgs("windel"), s)
	if err := cmdWINDOWDELETE(ctx); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCmdWINDOWDELETE_WrongArgs(t *testing.T) {
	s := store.NewStore()
	ctx := discardCtx("WINDOW.DELETE", bytesArgs(), s)
	if err := cmdWINDOWDELETE(ctx); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

// --- FREQ ---

func TestCmdFREQCREATE_Success(t *testing.T) {
	s := store.NewStore()
	ctx := discardCtx("FREQ.CREATE", bytesArgs("fr1"), s)
	if err := cmdFREQCREATE(ctx); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCmdFREQCREATE_WrongArgs(t *testing.T) {
	s := store.NewStore()
	ctx := discardCtx("FREQ.CREATE", bytesArgs(), s)
	if err := cmdFREQCREATE(ctx); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCmdFREQADD_Success(t *testing.T) {
	s := store.NewStore()
	_ = cmdFREQCREATE(discardCtx("", bytesArgs("fradd"), s))
	ctx := discardCtx("FREQ.ADD", bytesArgs("fradd", "item1"), s)
	if err := cmdFREQADD(ctx); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCmdFREQADD_WrongArgs(t *testing.T) {
	s := store.NewStore()
	ctx := discardCtx("FREQ.ADD", bytesArgs("x"), s)
	if err := cmdFREQADD(ctx); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCmdFREQCOUNT_Success(t *testing.T) {
	s := store.NewStore()
	_ = cmdFREQCREATE(discardCtx("", bytesArgs("frcnt"), s))
	_ = cmdFREQADD(discardCtx("", bytesArgs("frcnt", "item1"), s))
	ctx := discardCtx("FREQ.COUNT", bytesArgs("frcnt", "item1"), s)
	if err := cmdFREQCOUNT(ctx); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCmdFREQCOUNT_WrongArgs(t *testing.T) {
	s := store.NewStore()
	ctx := discardCtx("FREQ.COUNT", bytesArgs("x"), s)
	if err := cmdFREQCOUNT(ctx); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCmdFREQTOP_Success(t *testing.T) {
	s := store.NewStore()
	_ = cmdFREQCREATE(discardCtx("", bytesArgs("frtop"), s))
	_ = cmdFREQADD(discardCtx("", bytesArgs("frtop", "item1"), s))
	ctx := discardCtx("FREQ.TOP", bytesArgs("frtop"), s)
	if err := cmdFREQTOP(ctx); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCmdFREQTOP_WrongArgs(t *testing.T) {
	s := store.NewStore()
	ctx := discardCtx("FREQ.TOP", bytesArgs(), s)
	if err := cmdFREQTOP(ctx); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCmdFREQDELETE_Success(t *testing.T) {
	s := store.NewStore()
	_ = cmdFREQCREATE(discardCtx("", bytesArgs("frdel"), s))
	ctx := discardCtx("FREQ.DELETE", bytesArgs("frdel"), s)
	if err := cmdFREQDELETE(ctx); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCmdFREQDELETE_WrongArgs(t *testing.T) {
	s := store.NewStore()
	ctx := discardCtx("FREQ.DELETE", bytesArgs(), s)
	if err := cmdFREQDELETE(ctx); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

// --- PARTITION ---

func TestCmdPARTITIONCREATE_Success(t *testing.T) {
	s := store.NewStore()
	ctx := discardCtx("PARTITION.CREATE", bytesArgs("pt1", "4"), s)
	if err := cmdPARTITIONCREATE(ctx); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCmdPARTITIONCREATE_WrongArgs(t *testing.T) {
	s := store.NewStore()
	ctx := discardCtx("PARTITION.CREATE", bytesArgs("pt1"), s)
	if err := cmdPARTITIONCREATE(ctx); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCmdPARTITIONADD_Success(t *testing.T) {
	s := store.NewStore()
	_ = cmdPARTITIONCREATE(discardCtx("", bytesArgs("ptadd", "4"), s))
	ctx := discardCtx("PARTITION.ADD", bytesArgs("ptadd", "key1", "val1"), s)
	if err := cmdPARTITIONADD(ctx); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCmdPARTITIONADD_WrongArgs(t *testing.T) {
	s := store.NewStore()
	ctx := discardCtx("PARTITION.ADD", bytesArgs("pt1", "key1"), s)
	if err := cmdPARTITIONADD(ctx); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCmdPARTITIONGET_Success(t *testing.T) {
	s := store.NewStore()
	_ = cmdPARTITIONCREATE(discardCtx("", bytesArgs("ptget", "4"), s))
	ctx := discardCtx("PARTITION.GET", bytesArgs("ptget", "0"), s)
	if err := cmdPARTITIONGET(ctx); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCmdPARTITIONGET_WrongArgs(t *testing.T) {
	s := store.NewStore()
	ctx := discardCtx("PARTITION.GET", bytesArgs("x"), s)
	if err := cmdPARTITIONGET(ctx); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCmdPARTITIONLIST_Success(t *testing.T) {
	s := store.NewStore()
	_ = cmdPARTITIONCREATE(discardCtx("", bytesArgs("ptlst", "4"), s))
	ctx := discardCtx("PARTITION.LIST", bytesArgs("ptlst"), s)
	if err := cmdPARTITIONLIST(ctx); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCmdPARTITIONLIST_WrongArgs(t *testing.T) {
	s := store.NewStore()
	ctx := discardCtx("PARTITION.LIST", bytesArgs(), s)
	if err := cmdPARTITIONLIST(ctx); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCmdPARTITIONDELETE_Success(t *testing.T) {
	s := store.NewStore()
	_ = cmdPARTITIONCREATE(discardCtx("", bytesArgs("ptdel", "4"), s))
	ctx := discardCtx("PARTITION.DELETE", bytesArgs("ptdel"), s)
	if err := cmdPARTITIONDELETE(ctx); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCmdPARTITIONDELETE_WrongArgs(t *testing.T) {
	s := store.NewStore()
	ctx := discardCtx("PARTITION.DELETE", bytesArgs(), s)
	if err := cmdPARTITIONDELETE(ctx); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

// --- RegisterExtraCommands / RegisterMoreCommands ---

func TestRegisterExtraCommands(t *testing.T) {
	router := NewRouter()
	RegisterExtraCommands(router)
	if _, ok := router.Get("SWIM.JOIN"); !ok {
		t.Fatal("expected SWIM.JOIN to be registered")
	}
}

func TestRegisterMoreCommands(t *testing.T) {
	router := NewRouter()
	RegisterMoreCommands(router)
	if _, ok := router.Get("SLIDING.CREATE"); !ok {
		t.Fatal("expected SLIDING.CREATE to be registered")
	}
}
