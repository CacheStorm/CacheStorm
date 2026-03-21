package command

import (
	"strings"
	"testing"

	"github.com/cachestorm/cachestorm/internal/store"
)

// =============================================================================
// SERVER COMMANDS
// =============================================================================

func TestCmdPING_Success(t *testing.T) {
	s := store.NewStore()
	ctx, buf := bufCtx("PING", bytesArgs(), s)
	err := cmdPING(ctx)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(buf.String(), "PONG") {
		t.Errorf("expected PONG, got %q", buf.String())
	}
}

func TestCmdPING_WithArg(t *testing.T) {
	s := store.NewStore()
	ctx, buf := bufCtx("PING", bytesArgs("hello"), s)
	err := cmdPING(ctx)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(buf.String(), "hello") {
		t.Errorf("expected hello, got %q", buf.String())
	}
}

func TestCmdECHO_Success(t *testing.T) {
	s := store.NewStore()
	ctx, buf := bufCtx("ECHO", bytesArgs("world"), s)
	err := cmdECHO(ctx)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(buf.String(), "world") {
		t.Errorf("expected world, got %q", buf.String())
	}
}

func TestCmdECHO_WrongArgs(t *testing.T) {
	s := store.NewStore()
	ctx := discardCtx("ECHO", bytesArgs(), s)
	err := cmdECHO(ctx)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCmdQUIT(t *testing.T) {
	s := store.NewStore()
	ctx := discardCtx("QUIT", bytesArgs(), s)
	err := cmdQUIT(ctx)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCmdCOMMAND_NoArgs(t *testing.T) {
	s := store.NewStore()
	ctx := discardCtx("COMMAND", bytesArgs(), s)
	err := cmdCOMMAND(ctx)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCmdCOMMAND_COUNT(t *testing.T) {
	s := store.NewStore()
	ctx := discardCtx("COMMAND", bytesArgs("COUNT"), s)
	err := cmdCOMMAND(ctx)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCmdCOMMAND_LIST(t *testing.T) {
	s := store.NewStore()
	ctx := discardCtx("COMMAND", bytesArgs("LIST"), s)
	err := cmdCOMMAND(ctx)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCmdCOMMAND_DOCS(t *testing.T) {
	s := store.NewStore()
	ctx := discardCtx("COMMAND", bytesArgs("DOCS", "GET"), s)
	err := cmdCOMMAND(ctx)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCmdCOMMAND_GETKEYS(t *testing.T) {
	s := store.NewStore()
	ctx := discardCtx("COMMAND", bytesArgs("GETKEYS", "GET", "mykey"), s)
	err := cmdCOMMAND(ctx)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCmdCOMMAND_Unknown(t *testing.T) {
	s := store.NewStore()
	ctx := discardCtx("COMMAND", bytesArgs("BADSUBCMD"), s)
	err := cmdCOMMAND(ctx)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCmdINFO(t *testing.T) {
	s := store.NewStore()
	// InitReplicationManager removed — singleton already initialized by other tests
	ctx := discardCtx("INFO", bytesArgs(), s)
	err := cmdINFO(ctx)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCmdDBSIZE(t *testing.T) {
	s := store.NewStore()
	ctx := discardCtx("DBSIZE", bytesArgs(), s)
	err := cmdDBSIZE(ctx)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCmdFLUSHDB(t *testing.T) {
	s := store.NewStore()
	s.Set("k1", &store.StringValue{Data: []byte("v1")}, store.SetOptions{})
	ctx := discardCtx("FLUSHDB", bytesArgs(), s)
	err := cmdFLUSHDB(ctx)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCmdFLUSHALL(t *testing.T) {
	s := store.NewStore()
	ctx := discardCtx("FLUSHALL", bytesArgs(), s)
	err := cmdFLUSHALL(ctx)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCmdTIME(t *testing.T) {
	s := store.NewStore()
	ctx := discardCtx("TIME", bytesArgs(), s)
	err := cmdTIME(ctx)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCmdEXPIRE_Success(t *testing.T) {
	s := store.NewStore()
	s.Set("k1", &store.StringValue{Data: []byte("v")}, store.SetOptions{})
	ctx := discardCtx("EXPIRE", bytesArgs("k1", "10"), s)
	err := cmdEXPIRE(ctx)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCmdEXPIRE_WrongArgs(t *testing.T) {
	s := store.NewStore()
	ctx := discardCtx("EXPIRE", bytesArgs("k1"), s)
	err := cmdEXPIRE(ctx)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCmdEXPIRE_NotInteger(t *testing.T) {
	s := store.NewStore()
	ctx := discardCtx("EXPIRE", bytesArgs("k1", "abc"), s)
	err := cmdEXPIRE(ctx)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCmdPEXPIRE_Success(t *testing.T) {
	s := store.NewStore()
	s.Set("k1", &store.StringValue{Data: []byte("v")}, store.SetOptions{})
	ctx := discardCtx("PEXPIRE", bytesArgs("k1", "10000"), s)
	err := cmdPEXPIRE(ctx)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCmdEXPIREAT_Success(t *testing.T) {
	s := store.NewStore()
	s.Set("k1", &store.StringValue{Data: []byte("v")}, store.SetOptions{})
	ctx := discardCtx("EXPIREAT", bytesArgs("k1", "9999999999"), s)
	err := cmdEXPIREAT(ctx)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCmdPEXPIREAT_Success(t *testing.T) {
	s := store.NewStore()
	s.Set("k1", &store.StringValue{Data: []byte("v")}, store.SetOptions{})
	ctx := discardCtx("PEXPIREAT", bytesArgs("k1", "9999999999000"), s)
	err := cmdPEXPIREAT(ctx)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCmdTTL_Success(t *testing.T) {
	s := store.NewStore()
	ctx := discardCtx("TTL", bytesArgs("k1"), s)
	err := cmdTTL(ctx)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCmdPTTL_Success(t *testing.T) {
	s := store.NewStore()
	ctx := discardCtx("PTTL", bytesArgs("k1"), s)
	err := cmdPTTL(ctx)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCmdPEXPIRETIME_Success(t *testing.T) {
	s := store.NewStore()
	s.Set("k1", &store.StringValue{Data: []byte("v")}, store.SetOptions{})
	ctx := discardCtx("PEXPIRETIME", bytesArgs("k1"), s)
	err := cmdPEXPIRETIME(ctx)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCmdPEXPIRETIME_Missing(t *testing.T) {
	s := store.NewStore()
	ctx := discardCtx("PEXPIRETIME", bytesArgs("nonexist"), s)
	err := cmdPEXPIRETIME(ctx)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCmdEXPIRETIME_Success(t *testing.T) {
	s := store.NewStore()
	s.Set("k1", &store.StringValue{Data: []byte("v")}, store.SetOptions{})
	ctx := discardCtx("EXPIRETIME", bytesArgs("k1"), s)
	err := cmdEXPIRETIME(ctx)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCmdPERSIST_Success(t *testing.T) {
	s := store.NewStore()
	ctx := discardCtx("PERSIST", bytesArgs("k1"), s)
	err := cmdPERSIST(ctx)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCmdTYPE_Success(t *testing.T) {
	s := store.NewStore()
	s.Set("k1", &store.StringValue{Data: []byte("v")}, store.SetOptions{})
	ctx := discardCtx("TYPE", bytesArgs("k1"), s)
	err := cmdTYPE(ctx)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCmdTYPE_Missing(t *testing.T) {
	s := store.NewStore()
	ctx := discardCtx("TYPE", bytesArgs("missing"), s)
	err := cmdTYPE(ctx)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCmdRENAME_Success(t *testing.T) {
	s := store.NewStore()
	s.Set("k1", &store.StringValue{Data: []byte("v")}, store.SetOptions{})
	ctx := discardCtx("RENAME", bytesArgs("k1", "k2"), s)
	err := cmdRENAME(ctx)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCmdRENAME_WrongArgs(t *testing.T) {
	s := store.NewStore()
	ctx := discardCtx("RENAME", bytesArgs("k1"), s)
	err := cmdRENAME(ctx)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCmdRENAMENX_Success(t *testing.T) {
	s := store.NewStore()
	s.Set("k1", &store.StringValue{Data: []byte("v")}, store.SetOptions{})
	ctx := discardCtx("RENAMENX", bytesArgs("k1", "k2"), s)
	err := cmdRENAMENX(ctx)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCmdKEYS_Success(t *testing.T) {
	s := store.NewStore()
	s.Set("key1", &store.StringValue{Data: []byte("v")}, store.SetOptions{})
	ctx := discardCtx("KEYS", bytesArgs("*"), s)
	err := cmdKEYS(ctx)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCmdRANDOMKEY_Success(t *testing.T) {
	s := store.NewStore()
	s.Set("key1", &store.StringValue{Data: []byte("v")}, store.SetOptions{})
	ctx := discardCtx("RANDOMKEY", bytesArgs(), s)
	err := cmdRANDOMKEY(ctx)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCmdRANDOMKEY_Empty(t *testing.T) {
	s := store.NewStore()
	ctx := discardCtx("RANDOMKEY", bytesArgs(), s)
	err := cmdRANDOMKEY(ctx)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCmdTOUCH_Success(t *testing.T) {
	s := store.NewStore()
	s.Set("k1", &store.StringValue{Data: []byte("v")}, store.SetOptions{})
	ctx := discardCtx("TOUCH", bytesArgs("k1"), s)
	err := cmdTOUCH(ctx)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCmdDUMP_Success(t *testing.T) {
	s := store.NewStore()
	s.Set("k1", &store.StringValue{Data: []byte("v")}, store.SetOptions{})
	ctx := discardCtx("DUMP", bytesArgs("k1"), s)
	err := cmdDUMP(ctx)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCmdDUMP_Missing(t *testing.T) {
	s := store.NewStore()
	ctx := discardCtx("DUMP", bytesArgs("nosuchkey"), s)
	err := cmdDUMP(ctx)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCmdCOPY_Success(t *testing.T) {
	s := store.NewStore()
	s.Set("src", &store.StringValue{Data: []byte("v")}, store.SetOptions{})
	ctx := discardCtx("COPY", bytesArgs("src", "dst"), s)
	err := cmdCOPY(ctx)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCmdCOPY_Replace(t *testing.T) {
	s := store.NewStore()
	s.Set("src", &store.StringValue{Data: []byte("v")}, store.SetOptions{})
	s.Set("dst", &store.StringValue{Data: []byte("old")}, store.SetOptions{})
	ctx := discardCtx("COPY", bytesArgs("src", "dst", "REPLACE"), s)
	err := cmdCOPY(ctx)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCmdSCAN_Success(t *testing.T) {
	s := store.NewStore()
	s.Set("k1", &store.StringValue{Data: []byte("v")}, store.SetOptions{})
	ctx := discardCtx("SCAN", bytesArgs("0"), s)
	err := cmdSCAN(ctx)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCmdSCAN_WithMatch(t *testing.T) {
	s := store.NewStore()
	s.Set("k1", &store.StringValue{Data: []byte("v")}, store.SetOptions{})
	ctx := discardCtx("SCAN", bytesArgs("0", "MATCH", "k*", "COUNT", "100"), s)
	err := cmdSCAN(ctx)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCmdHOTKEYS_Success(t *testing.T) {
	s := store.NewStore()
	s.Set("k1", &store.StringValue{Data: []byte("v")}, store.SetOptions{})
	ctx := discardCtx("HOTKEYS", bytesArgs(), s)
	err := cmdHOTKEYS(ctx)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCmdMEMINFO_Success(t *testing.T) {
	s := store.NewStore()
	ctx := discardCtx("MEMINFO", bytesArgs(), s)
	err := cmdMEMINFO(ctx)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCmdSLOWLOG_LEN(t *testing.T) {
	s := store.NewStore()
	ctx := discardCtx("SLOWLOG", bytesArgs("LEN"), s)
	err := cmdSLOWLOG(ctx)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCmdSLOWLOG_RESET(t *testing.T) {
	s := store.NewStore()
	ctx := discardCtx("SLOWLOG", bytesArgs("RESET"), s)
	err := cmdSLOWLOG(ctx)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCmdWAIT_Success(t *testing.T) {
	s := store.NewStore()
	ctx := discardCtx("WAIT", bytesArgs("0", "0"), s)
	err := cmdWAIT(ctx)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCmdROLE_NoRepl(t *testing.T) {
	s := store.NewStore()
	oldMgr := replManager
	replManager = nil
	defer func() { replManager = oldMgr }()
	ctx := discardCtx("ROLE", bytesArgs(), s)
	err := cmdROLE(ctx)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCmdROLE_WithRepl(t *testing.T) {
	s := store.NewStore()
	// InitReplicationManager removed — singleton already initialized by other tests
	ctx := discardCtx("ROLE", bytesArgs(), s)
	err := cmdROLE(ctx)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCmdLASTSAVE(t *testing.T) {
	s := store.NewStore()
	ctx := discardCtx("LASTSAVE", bytesArgs(), s)
	err := cmdLASTSAVE(ctx)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCmdLOLWUT(t *testing.T) {
	s := store.NewStore()
	ctx := discardCtx("LOLWUT", bytesArgs(), s)
	err := cmdLOLWUT(ctx)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCmdSHUTDOWN(t *testing.T) {
	s := store.NewStore()
	ctx := discardCtx("SHUTDOWN", bytesArgs(), s)
	err := cmdSHUTDOWN(ctx)
	// SHUTDOWN always returns an error("SHUTDOWN") by design
	if err == nil {
		t.Fatal("expected SHUTDOWN error")
	}
	if err.Error() != "SHUTDOWN" {
		t.Fatalf("expected SHUTDOWN error, got %v", err)
	}
}

func TestCmdSAVE(t *testing.T) {
	s := store.NewStore()
	ctx := discardCtx("SAVE", bytesArgs(), s)
	err := cmdSAVE(ctx)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCmdBGSAVE(t *testing.T) {
	s := store.NewStore()
	ctx := discardCtx("BGSAVE", bytesArgs(), s)
	err := cmdBGSAVE(ctx)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCmdBGREWRITEAOF(t *testing.T) {
	s := store.NewStore()
	ctx := discardCtx("BGREWRITEAOF", bytesArgs(), s)
	err := cmdBGREWRITEAOF(ctx)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCmdSLAVEOF(t *testing.T) {
	s := store.NewStore()
	ctx := discardCtx("SLAVEOF", bytesArgs("NO", "ONE"), s)
	err := cmdSLAVEOF(ctx)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCmdLATENCY(t *testing.T) {
	s := store.NewStore()
	ctx := discardCtx("LATENCY", bytesArgs("LATEST"), s)
	err := cmdLATENCY(ctx)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCmdSTRALGO(t *testing.T) {
	s := store.NewStore()
	ctx := discardCtx("STRALGO", bytesArgs("LCS", "KEYS", "a", "b"), s)
	err := cmdSTRALGO(ctx)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCmdMODULE(t *testing.T) {
	s := store.NewStore()
	ctx := discardCtx("MODULE", bytesArgs("LIST"), s)
	err := cmdMODULE(ctx)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCmdMONITOR(t *testing.T) {
	s := store.NewStore()
	ctx := discardCtx("MONITOR", bytesArgs(), s)
	err := cmdMONITOR(ctx)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCmdSWAPDB(t *testing.T) {
	s := store.NewStore()
	ctx := discardCtx("SWAPDB", bytesArgs("0", "1"), s)
	err := cmdSWAPDB(ctx)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCmdDEBUGSEGFAULT(t *testing.T) {
	s := store.NewStore()
	ctx := discardCtx("DEBUGSEGFAULT", bytesArgs(), s)
	err := cmdDEBUGSEGFAULT(ctx)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCmdCLIENT_Success(t *testing.T) {
	s := store.NewStore()
	ctx := discardCtx("CLIENT", bytesArgs("LIST"), s)
	ctx.ClientID = 1
	err := cmdCLIENT(ctx)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCmdCLIENT_ID(t *testing.T) {
	s := store.NewStore()
	ctx := discardCtx("CLIENT", bytesArgs("ID"), s)
	ctx.ClientID = 42
	err := cmdCLIENT(ctx)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCmdCLIENT_SETNAME(t *testing.T) {
	s := store.NewStore()
	ctx := discardCtx("CLIENT", bytesArgs("SETNAME", "conn1"), s)
	err := cmdCLIENT(ctx)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCmdCLIENT_GETNAME(t *testing.T) {
	s := store.NewStore()
	ctx := discardCtx("CLIENT", bytesArgs("GETNAME"), s)
	err := cmdCLIENT(ctx)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCmdMOVE_Success(t *testing.T) {
	s := store.NewStore()
	s.Set("k1", &store.StringValue{Data: []byte("v")}, store.SetOptions{})
	ctx := discardCtx("MOVE", bytesArgs("k1", "1"), s)
	err := cmdMOVE(ctx)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCmdWAITAOF(t *testing.T) {
	s := store.NewStore()
	ctx := discardCtx("WAITAOF", bytesArgs("0", "0", "0"), s)
	err := cmdWAITAOF(ctx)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCmdACL_WHOAMI(t *testing.T) {
	s := store.NewStore()
	ctx := discardCtx("ACL", bytesArgs("WHOAMI"), s)
	ctx.Username = "default"
	err := cmdACL(ctx)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCmdACL_LIST(t *testing.T) {
	s := store.NewStore()
	ctx := discardCtx("ACL", bytesArgs("LIST"), s)
	err := cmdACL(ctx)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCmdSORT_Success(t *testing.T) {
	s := store.NewStore()
	lv := &store.ListValue{Elements: [][]byte{[]byte("3"), []byte("1"), []byte("2")}}
	s.Set("mylist", lv, store.SetOptions{})
	ctx := discardCtx("SORT", bytesArgs("mylist"), s)
	err := cmdSORT(ctx)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCmdSORTRO_Success(t *testing.T) {
	s := store.NewStore()
	lv := &store.ListValue{Elements: [][]byte{[]byte("3"), []byte("1"), []byte("2")}}
	s.Set("mylist", lv, store.SetOptions{})
	ctx := discardCtx("SORT_RO", bytesArgs("mylist"), s)
	err := cmdSORTRO(ctx)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCmdRESTORE_Success(t *testing.T) {
	s := store.NewStore()
	// Dump format: "CACHSTORM001" + type byte + expiry + ":" + data
	dump := "CACHSTORM001\x010:"
	ctx := discardCtx("RESTORE", bytesArgs("newkey", "0", dump+"hello"), s)
	err := cmdRESTORE(ctx)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

// =============================================================================
// STRING COMMANDS (remaining uncovered)
// =============================================================================

func TestCmdGETSET_Success(t *testing.T) {
	s := store.NewStore()
	s.Set("k", &store.StringValue{Data: []byte("old")}, store.SetOptions{})
	ctx := discardCtx("GETSET", bytesArgs("k", "new"), s)
	err := cmdGETSET(ctx)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCmdGETSET_WrongArgs_Misc(t *testing.T) {
	s := store.NewStore()
	ctx := discardCtx("GETSET", bytesArgs("k"), s)
	err := cmdGETSET(ctx)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCmdGETDEL_Success_Misc(t *testing.T) {
	s := store.NewStore()
	s.Set("k", &store.StringValue{Data: []byte("v")}, store.SetOptions{})
	ctx := discardCtx("GETDEL", bytesArgs("k"), s)
	err := cmdGETDEL(ctx)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if s.Exists("k") {
		t.Error("key should be deleted")
	}
}

func TestCmdGETEX_Success(t *testing.T) {
	s := store.NewStore()
	s.Set("k", &store.StringValue{Data: []byte("v")}, store.SetOptions{})
	ctx := discardCtx("GETEX", bytesArgs("k", "EX", "100"), s)
	err := cmdGETEX(ctx)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCmdGETEX_Persist(t *testing.T) {
	s := store.NewStore()
	s.Set("k", &store.StringValue{Data: []byte("v")}, store.SetOptions{})
	ctx := discardCtx("GETEX", bytesArgs("k", "PERSIST"), s)
	err := cmdGETEX(ctx)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCmdGETEX_Missing(t *testing.T) {
	s := store.NewStore()
	ctx := discardCtx("GETEX", bytesArgs("nosuchkey"), s)
	err := cmdGETEX(ctx)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCmdINCRBYFLOAT_Success(t *testing.T) {
	s := store.NewStore()
	s.Set("k", &store.StringValue{Data: []byte("10")}, store.SetOptions{})
	ctx := discardCtx("INCRBYFLOAT", bytesArgs("k", "1.5"), s)
	err := cmdINCRBYFLOAT(ctx)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCmdINCRBYFLOAT_NewKey_Misc(t *testing.T) {
	s := store.NewStore()
	ctx := discardCtx("INCRBYFLOAT", bytesArgs("newk", "5.5"), s)
	err := cmdINCRBYFLOAT(ctx)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCmdINCRBYFLOAT_WrongArgs(t *testing.T) {
	s := store.NewStore()
	ctx := discardCtx("INCRBYFLOAT", bytesArgs("k"), s)
	err := cmdINCRBYFLOAT(ctx)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCmdPSETEX_Success_Misc(t *testing.T) {
	s := store.NewStore()
	ctx := discardCtx("PSETEX", bytesArgs("k", "10000", "val"), s)
	err := cmdPSETEX(ctx)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCmdMSETNX_Success(t *testing.T) {
	s := store.NewStore()
	ctx := discardCtx("MSETNX", bytesArgs("k1", "v1", "k2", "v2"), s)
	err := cmdMSETNX(ctx)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCmdMSETNX_ExistingKey(t *testing.T) {
	s := store.NewStore()
	s.Set("k1", &store.StringValue{Data: []byte("old")}, store.SetOptions{})
	ctx := discardCtx("MSETNX", bytesArgs("k1", "v1", "k2", "v2"), s)
	err := cmdMSETNX(ctx)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCmdSETRANGE_Success_Misc(t *testing.T) {
	s := store.NewStore()
	s.Set("k", &store.StringValue{Data: []byte("Hello World")}, store.SetOptions{})
	ctx := discardCtx("SETRANGE", bytesArgs("k", "6", "Redis"), s)
	err := cmdSETRANGE(ctx)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCmdGETRANGE_Success_Misc(t *testing.T) {
	s := store.NewStore()
	s.Set("k", &store.StringValue{Data: []byte("Hello World")}, store.SetOptions{})
	ctx := discardCtx("GETRANGE", bytesArgs("k", "0", "4"), s)
	err := cmdGETRANGE(ctx)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCmdLCS_Success(t *testing.T) {
	s := store.NewStore()
	s.Set("k1", &store.StringValue{Data: []byte("ohmytext")}, store.SetOptions{})
	s.Set("k2", &store.StringValue{Data: []byte("mynewtext")}, store.SetOptions{})
	ctx := discardCtx("LCS", bytesArgs("k1", "k2"), s)
	err := cmdLCS(ctx)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCmdLCS_LEN_Misc(t *testing.T) {
	s := store.NewStore()
	s.Set("k1", &store.StringValue{Data: []byte("ohmytext")}, store.SetOptions{})
	s.Set("k2", &store.StringValue{Data: []byte("mynewtext")}, store.SetOptions{})
	ctx := discardCtx("LCS", bytesArgs("k1", "k2", "LEN"), s)
	err := cmdLCS(ctx)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCmdLCS_IDX_Misc(t *testing.T) {
	s := store.NewStore()
	s.Set("k1", &store.StringValue{Data: []byte("ohmytext")}, store.SetOptions{})
	s.Set("k2", &store.StringValue{Data: []byte("mynewtext")}, store.SetOptions{})
	ctx := discardCtx("LCS", bytesArgs("k1", "k2", "IDX", "WITHMATCHLEN"), s)
	err := cmdLCS(ctx)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

// =============================================================================
// TRANSACTION COMMANDS
// =============================================================================

func TestCmdMULTI_Success(t *testing.T) {
	s := store.NewStore()
	ctx := discardCtx("MULTI", bytesArgs(), s)
	ctx.Transaction = NewTransaction()
	err := cmdMULTI(ctx)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !ctx.Transaction.IsActive() {
		t.Error("transaction should be active")
	}
}

func TestCmdEXEC_WithoutMulti(t *testing.T) {
	s := store.NewStore()
	ctx := discardCtx("EXEC", bytesArgs(), s)
	ctx.Transaction = NewTransaction()
	err := cmdEXEC(ctx)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCmdEXEC_WithMulti(t *testing.T) {
	s := store.NewStore()
	ctx := discardCtx("EXEC", bytesArgs(), s)
	ctx.Transaction = NewTransaction()
	ctx.Transaction.Start()
	ctx.Transaction.Queue("SET", [][]byte{[]byte("k1"), []byte("v1")})
	err := cmdEXEC(ctx)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCmdDISCARD_Success(t *testing.T) {
	s := store.NewStore()
	ctx := discardCtx("DISCARD", bytesArgs(), s)
	ctx.Transaction = NewTransaction()
	ctx.Transaction.Start()
	err := cmdDISCARD(ctx)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCmdDISCARD_WithoutMulti(t *testing.T) {
	s := store.NewStore()
	ctx := discardCtx("DISCARD", bytesArgs(), s)
	ctx.Transaction = NewTransaction()
	err := cmdDISCARD(ctx)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCmdWATCH_Success(t *testing.T) {
	s := store.NewStore()
	ctx := discardCtx("WATCH", bytesArgs("k1", "k2"), s)
	ctx.Transaction = NewTransaction()
	err := cmdWATCH(ctx)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCmdWATCH_InsideMulti(t *testing.T) {
	s := store.NewStore()
	ctx := discardCtx("WATCH", bytesArgs("k1"), s)
	ctx.Transaction = NewTransaction()
	ctx.Transaction.Start()
	err := cmdWATCH(ctx)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCmdUNWATCH_Success(t *testing.T) {
	s := store.NewStore()
	ctx := discardCtx("UNWATCH", bytesArgs(), s)
	ctx.Transaction = NewTransaction()
	err := cmdUNWATCH(ctx)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

// =============================================================================
// UTILITY_EXT COMMANDS (audit, flags, counters, backup, memory)
// =============================================================================

func TestCmdAUDITLOG_Success(t *testing.T) {
	s := store.NewStore()
	ctx := discardCtx("AUDIT.LOG", bytesArgs("SET", "k1", "v1"), s)
	err := cmdAUDITLOG(ctx)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCmdAUDITLOG_WrongArgs(t *testing.T) {
	s := store.NewStore()
	ctx := discardCtx("AUDIT.LOG", bytesArgs(), s)
	err := cmdAUDITLOG(ctx)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCmdAUDITGET_Success(t *testing.T) {
	s := store.NewStore()
	store.GlobalAuditLog.Log("SET", "k", nil, "", "", true, 0)
	ctx := discardCtx("AUDIT.GET", bytesArgs("1"), s)
	err := cmdAUDITGET(ctx)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCmdAUDITGETRANGE_Success(t *testing.T) {
	s := store.NewStore()
	ctx := discardCtx("AUDIT.GETRANGE", bytesArgs("0", "100"), s)
	err := cmdAUDITGETRANGE(ctx)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCmdAUDITGETBYCMD_Success(t *testing.T) {
	s := store.NewStore()
	ctx := discardCtx("AUDIT.GETBYCMD", bytesArgs("SET"), s)
	err := cmdAUDITGETBYCMD(ctx)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCmdAUDITGETBYKEY_Success(t *testing.T) {
	s := store.NewStore()
	ctx := discardCtx("AUDIT.GETBYKEY", bytesArgs("k1"), s)
	err := cmdAUDITGETBYKEY(ctx)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCmdAUDITCLEAR(t *testing.T) {
	s := store.NewStore()
	ctx := discardCtx("AUDIT.CLEAR", bytesArgs(), s)
	err := cmdAUDITCLEAR(ctx)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCmdAUDITCOUNT(t *testing.T) {
	s := store.NewStore()
	ctx := discardCtx("AUDIT.COUNT", bytesArgs(), s)
	err := cmdAUDITCOUNT(ctx)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCmdAUDITSTATS(t *testing.T) {
	s := store.NewStore()
	ctx := discardCtx("AUDIT.STATS", bytesArgs(), s)
	err := cmdAUDITSTATS(ctx)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCmdAUDITENABLE(t *testing.T) {
	s := store.NewStore()
	ctx := discardCtx("AUDIT.ENABLE", bytesArgs(), s)
	err := cmdAUDITENABLE(ctx)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCmdAUDITDISABLE(t *testing.T) {
	s := store.NewStore()
	ctx := discardCtx("AUDIT.DISABLE", bytesArgs(), s)
	err := cmdAUDITDISABLE(ctx)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCmdFLAGCREATE_Success(t *testing.T) {
	s := store.NewStore()
	ctx := discardCtx("FLAG.CREATE", bytesArgs("test-flag", "desc"), s)
	err := cmdFLAGCREATE(ctx)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCmdFLAGDELETE_Success(t *testing.T) {
	s := store.NewStore()
	store.GlobalFeatureFlags.Create("del-flag", "")
	ctx := discardCtx("FLAG.DELETE", bytesArgs("del-flag"), s)
	err := cmdFLAGDELETE(ctx)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCmdFLAGGET_Success(t *testing.T) {
	s := store.NewStore()
	store.GlobalFeatureFlags.Create("get-flag", "desc")
	ctx := discardCtx("FLAG.GET", bytesArgs("get-flag"), s)
	err := cmdFLAGGET(ctx)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCmdFLAGGET_Missing(t *testing.T) {
	s := store.NewStore()
	ctx := discardCtx("FLAG.GET", bytesArgs("nonexistent"), s)
	err := cmdFLAGGET(ctx)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCmdFLAGENABLE_Success(t *testing.T) {
	s := store.NewStore()
	store.GlobalFeatureFlags.Create("en-flag", "")
	ctx := discardCtx("FLAG.ENABLE", bytesArgs("en-flag"), s)
	err := cmdFLAGENABLE(ctx)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCmdFLAGDISABLE_Success(t *testing.T) {
	s := store.NewStore()
	store.GlobalFeatureFlags.Create("dis-flag", "")
	ctx := discardCtx("FLAG.DISABLE", bytesArgs("dis-flag"), s)
	err := cmdFLAGDISABLE(ctx)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCmdFLAGTOGGLE_Success(t *testing.T) {
	s := store.NewStore()
	store.GlobalFeatureFlags.Create("tog-flag", "")
	ctx := discardCtx("FLAG.TOGGLE", bytesArgs("tog-flag"), s)
	err := cmdFLAGTOGGLE(ctx)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCmdFLAGISENABLED(t *testing.T) {
	s := store.NewStore()
	store.GlobalFeatureFlags.Create("ise-flag", "")
	ctx := discardCtx("FLAG.ISENABLED", bytesArgs("ise-flag"), s)
	err := cmdFLAGISENABLED(ctx)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCmdFLAGLIST(t *testing.T) {
	s := store.NewStore()
	ctx := discardCtx("FLAG.LIST", bytesArgs(), s)
	err := cmdFLAGLIST(ctx)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCmdFLAGLISTENABLED(t *testing.T) {
	s := store.NewStore()
	ctx := discardCtx("FLAG.LISTENABLED", bytesArgs(), s)
	err := cmdFLAGLISTENABLED(ctx)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCmdFLAGADDVARIANT_Success(t *testing.T) {
	s := store.NewStore()
	store.GlobalFeatureFlags.Create("var-flag", "")
	ctx := discardCtx("FLAG.ADDVARIANT", bytesArgs("var-flag", "variant1", "val1"), s)
	err := cmdFLAGADDVARIANT(ctx)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCmdFLAGGETVARIANT_Success(t *testing.T) {
	s := store.NewStore()
	store.GlobalFeatureFlags.Create("gv-flag", "")
	store.GlobalFeatureFlags.AddVariant("gv-flag", "v1", "val1")
	ctx := discardCtx("FLAG.GETVARIANT", bytesArgs("gv-flag", "v1"), s)
	err := cmdFLAGGETVARIANT(ctx)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCmdFLAGADDRULE_Success(t *testing.T) {
	s := store.NewStore()
	store.GlobalFeatureFlags.Create("rule-flag", "")
	ctx := discardCtx("FLAG.ADDRULE", bytesArgs("rule-flag", "env", "eq", "prod"), s)
	err := cmdFLAGADDRULE(ctx)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCmdCOUNTERGET(t *testing.T) {
	s := store.NewStore()
	ctx := discardCtx("COUNTER.GET", bytesArgs("c1"), s)
	err := cmdCOUNTERGET(ctx)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCmdCOUNTERSET(t *testing.T) {
	s := store.NewStore()
	ctx := discardCtx("COUNTER.SET", bytesArgs("c1", "42"), s)
	err := cmdCOUNTERSET(ctx)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCmdCOUNTERINCR(t *testing.T) {
	s := store.NewStore()
	ctx := discardCtx("COUNTER.INCR", bytesArgs("c1"), s)
	err := cmdCOUNTERINCR(ctx)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCmdCOUNTERDECR(t *testing.T) {
	s := store.NewStore()
	ctx := discardCtx("COUNTER.DECR", bytesArgs("c1"), s)
	err := cmdCOUNTERDECR(ctx)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCmdCOUNTERINCRBY(t *testing.T) {
	s := store.NewStore()
	ctx := discardCtx("COUNTER.INCRBY", bytesArgs("c1", "10"), s)
	err := cmdCOUNTERINCRBY(ctx)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCmdCOUNTERDECRBY(t *testing.T) {
	s := store.NewStore()
	ctx := discardCtx("COUNTER.DECRBY", bytesArgs("c1", "5"), s)
	err := cmdCOUNTERDECRBY(ctx)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCmdCOUNTERDELETE(t *testing.T) {
	s := store.NewStore()
	store.GlobalAtomicCounter.Set("del-c", 1)
	ctx := discardCtx("COUNTER.DELETE", bytesArgs("del-c"), s)
	err := cmdCOUNTERDELETE(ctx)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCmdCOUNTERLIST(t *testing.T) {
	s := store.NewStore()
	ctx := discardCtx("COUNTER.LIST", bytesArgs(), s)
	err := cmdCOUNTERLIST(ctx)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCmdCOUNTERGETALL(t *testing.T) {
	s := store.NewStore()
	ctx := discardCtx("COUNTER.GETALL", bytesArgs(), s)
	err := cmdCOUNTERGETALL(ctx)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCmdCOUNTERRESET(t *testing.T) {
	s := store.NewStore()
	store.GlobalAtomicCounter.Set("reset-c", 99)
	ctx := discardCtx("COUNTER.RESET", bytesArgs("reset-c"), s)
	err := cmdCOUNTERRESET(ctx)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCmdCOUNTERRESETALL(t *testing.T) {
	s := store.NewStore()
	ctx := discardCtx("COUNTER.RESETALL", bytesArgs(), s)
	err := cmdCOUNTERRESETALL(ctx)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCmdBACKUPCREATE_Success(t *testing.T) {
	s := store.NewStore()
	s.Set("k1", &store.StringValue{Data: []byte("v1")}, store.SetOptions{})
	ctx := discardCtx("BACKUP.CREATE", bytesArgs("bk1"), s)
	err := cmdBACKUPCREATE(ctx)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCmdBACKUPRESTORE_Success(t *testing.T) {
	s := store.NewStore()
	s.Set("k1", &store.StringValue{Data: []byte("v1")}, store.SetOptions{})
	cctx := discardCtx("BACKUP.CREATE", bytesArgs("bk-restore"), s)
	cmdBACKUPCREATE(cctx)
	s.Flush()
	ctx := discardCtx("BACKUP.RESTORE", bytesArgs("bk-restore"), s)
	err := cmdBACKUPRESTORE(ctx)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCmdBACKUPLIST(t *testing.T) {
	s := store.NewStore()
	ctx := discardCtx("BACKUP.LIST", bytesArgs(), s)
	err := cmdBACKUPLIST(ctx)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCmdBACKUPDELETE(t *testing.T) {
	s := store.NewStore()
	ctx := discardCtx("BACKUP.DELETE", bytesArgs("nonexist"), s)
	err := cmdBACKUPDELETE(ctx)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCmdMEMORYTRIM(t *testing.T) {
	s := store.NewStore()
	ctx := discardCtx("MEMORY.TRIM", bytesArgs(), s)
	err := cmdMEMORYTRIM(ctx)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCmdMEMORYFRAG(t *testing.T) {
	s := store.NewStore()
	ctx := discardCtx("MEMORY.FRAG", bytesArgs(), s)
	err := cmdMEMORYFRAG(ctx)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCmdMEMORYPURGE(t *testing.T) {
	s := store.NewStore()
	ctx := discardCtx("MEMORY.PURGE", bytesArgs(), s)
	err := cmdMEMORYPURGE(ctx)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCmdMEMORYALLOC(t *testing.T) {
	s := store.NewStore()
	ctx := discardCtx("MEMORY.ALLOC", bytesArgs("64"), s)
	err := cmdMEMORYALLOC(ctx)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

// =============================================================================
// WORKFLOW COMMANDS
// =============================================================================

func TestCmdWORKFLOWCREATE_Success(t *testing.T) {
	s := store.NewStore()
	ctx := discardCtx("WORKFLOW.CREATE", bytesArgs("wf1", "MyWorkflow"), s)
	err := cmdWORKFLOWCREATE(ctx)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCmdWORKFLOWDELETE_Success(t *testing.T) {
	s := store.NewStore()
	store.GlobalWorkflowManager.Create("wf-del", "WF", nil)
	ctx := discardCtx("WORKFLOW.DELETE", bytesArgs("wf-del"), s)
	err := cmdWORKFLOWDELETE(ctx)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCmdWORKFLOWGET_Success(t *testing.T) {
	s := store.NewStore()
	store.GlobalWorkflowManager.Create("wf-get", "WF", nil)
	ctx := discardCtx("WORKFLOW.GET", bytesArgs("wf-get"), s)
	err := cmdWORKFLOWGET(ctx)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCmdWORKFLOWGET_Missing(t *testing.T) {
	s := store.NewStore()
	ctx := discardCtx("WORKFLOW.GET", bytesArgs("nosuch"), s)
	err := cmdWORKFLOWGET(ctx)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCmdWORKFLOWLIST(t *testing.T) {
	s := store.NewStore()
	ctx := discardCtx("WORKFLOW.LIST", bytesArgs(), s)
	err := cmdWORKFLOWLIST(ctx)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCmdWORKFLOWSTART_Success(t *testing.T) {
	s := store.NewStore()
	store.GlobalWorkflowManager.Create("wf-start", "WF", []store.WorkflowStep{{ID: "s1", Name: "step1", Command: "cmd"}})
	ctx := discardCtx("WORKFLOW.START", bytesArgs("wf-start"), s)
	err := cmdWORKFLOWSTART(ctx)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCmdWORKFLOWPAUSE_Success(t *testing.T) {
	s := store.NewStore()
	store.GlobalWorkflowManager.Create("wf-pause", "WF", []store.WorkflowStep{{ID: "s1", Name: "step1", Command: "cmd"}})
	store.GlobalWorkflowManager.Start("wf-pause")
	ctx := discardCtx("WORKFLOW.PAUSE", bytesArgs("wf-pause"), s)
	err := cmdWORKFLOWPAUSE(ctx)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCmdWORKFLOWCOMPLETE_Success(t *testing.T) {
	s := store.NewStore()
	store.GlobalWorkflowManager.Create("wf-comp", "WF", []store.WorkflowStep{{ID: "s1", Name: "step1", Command: "cmd"}})
	store.GlobalWorkflowManager.Start("wf-comp")
	ctx := discardCtx("WORKFLOW.COMPLETE", bytesArgs("wf-comp"), s)
	err := cmdWORKFLOWCOMPLETE(ctx)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCmdWORKFLOWFAIL_Success(t *testing.T) {
	s := store.NewStore()
	store.GlobalWorkflowManager.Create("wf-fail", "WF", []store.WorkflowStep{{ID: "s1", Name: "step1", Command: "cmd"}})
	store.GlobalWorkflowManager.Start("wf-fail")
	ctx := discardCtx("WORKFLOW.FAIL", bytesArgs("wf-fail", "something broke"), s)
	err := cmdWORKFLOWFAIL(ctx)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCmdWORKFLOWRESET_Success(t *testing.T) {
	s := store.NewStore()
	store.GlobalWorkflowManager.Create("wf-reset", "WF", nil)
	ctx := discardCtx("WORKFLOW.RESET", bytesArgs("wf-reset"), s)
	err := cmdWORKFLOWRESET(ctx)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCmdWORKFLOWSETVAR_Success(t *testing.T) {
	s := store.NewStore()
	store.GlobalWorkflowManager.Create("wf-var", "WF", nil)
	ctx := discardCtx("WORKFLOW.SETVAR", bytesArgs("wf-var", "key1", "val1"), s)
	err := cmdWORKFLOWSETVAR(ctx)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCmdWORKFLOWGETVAR_Success(t *testing.T) {
	s := store.NewStore()
	store.GlobalWorkflowManager.Create("wf-gv", "WF", nil)
	store.GlobalWorkflowManager.SetVariable("wf-gv", "k1", "v1")
	ctx := discardCtx("WORKFLOW.GETVAR", bytesArgs("wf-gv", "k1"), s)
	err := cmdWORKFLOWGETVAR(ctx)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

// =============================================================================
// TEMPLATE COMMANDS (workflow templates)
// =============================================================================

func TestCmdTEMPLATECREATE_Success(t *testing.T) {
	s := store.NewStore()
	ctx := discardCtx("TEMPLATE.CREATE", bytesArgs("tmpl1", "0"), s)
	err := cmdTEMPLATECREATE(ctx)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCmdTEMPLATEDELETE_Success(t *testing.T) {
	s := store.NewStore()
	store.GlobalWorkflowManager.CreateTemplate("del-tmpl", nil)
	ctx := discardCtx("TEMPLATE.DELETE", bytesArgs("del-tmpl"), s)
	err := cmdTEMPLATEDELETE(ctx)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCmdTEMPLATEGET_Success(t *testing.T) {
	s := store.NewStore()
	store.GlobalWorkflowManager.CreateTemplate("get-tmpl", nil)
	ctx := discardCtx("TEMPLATE.GET", bytesArgs("get-tmpl"), s)
	err := cmdTEMPLATEGET(ctx)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCmdTEMPLATEINSTANTIATE_Success(t *testing.T) {
	s := store.NewStore()
	store.GlobalWorkflowManager.CreateTemplate("inst-tmpl", nil)
	ctx := discardCtx("TEMPLATE.INSTANTIATE", bytesArgs("inst-tmpl", "wf-inst"), s)
	err := cmdTEMPLATEINSTANTIATE(ctx)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

// =============================================================================
// STATE MACHINE COMMANDS
// =============================================================================

func TestCmdSTATEMCREATE_Success(t *testing.T) {
	s := store.NewStore()
	ctx := discardCtx("STATEM.CREATE", bytesArgs("sm1", "idle"), s)
	err := cmdSTATEMCREATE(ctx)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCmdSTATEMDELETE_Success(t *testing.T) {
	s := store.NewStore()
	store.GetOrCreateStateMachine("sm-del", "idle")
	ctx := discardCtx("STATEM.DELETE", bytesArgs("sm-del"), s)
	err := cmdSTATEMDELETE(ctx)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCmdSTATEMADDSTATE_Success(t *testing.T) {
	s := store.NewStore()
	store.GetOrCreateStateMachine("sm-as", "idle")
	ctx := discardCtx("STATEM.ADDSTATE", bytesArgs("sm-as", "running"), s)
	err := cmdSTATEMADDSTATE(ctx)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCmdSTATEMADDTRANS_Success(t *testing.T) {
	s := store.NewStore()
	sm := store.GetOrCreateStateMachine("sm-at", "idle")
	sm.AddState("running", false, "", "")
	ctx := discardCtx("STATEM.ADDTRANS", bytesArgs("sm-at", "idle", "running", "start"), s)
	err := cmdSTATEMADDTRANS(ctx)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCmdSTATEMTRIGGER_Success(t *testing.T) {
	s := store.NewStore()
	sm := store.GetOrCreateStateMachine("sm-tr", "idle")
	sm.AddState("running", false, "", "")
	sm.AddTransition("idle", "running", "start")
	ctx := discardCtx("STATEM.TRIGGER", bytesArgs("sm-tr", "start"), s)
	err := cmdSTATEMTRIGGER(ctx)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCmdSTATEMCURRENT_Success(t *testing.T) {
	s := store.NewStore()
	store.GetOrCreateStateMachine("sm-cur", "idle")
	ctx := discardCtx("STATEM.CURRENT", bytesArgs("sm-cur"), s)
	err := cmdSTATEMCURRENT(ctx)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCmdSTATEMCANTRIGGER_Success(t *testing.T) {
	s := store.NewStore()
	store.GetOrCreateStateMachine("sm-ct", "idle")
	ctx := discardCtx("STATEM.CANTRIGGER", bytesArgs("sm-ct", "go"), s)
	err := cmdSTATEMCANTRIGGER(ctx)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCmdSTATEMEVENTS(t *testing.T) {
	s := store.NewStore()
	store.GetOrCreateStateMachine("sm-ev", "idle")
	ctx := discardCtx("STATEM.EVENTS", bytesArgs("sm-ev"), s)
	err := cmdSTATEMEVENTS(ctx)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCmdSTATEMRESET(t *testing.T) {
	s := store.NewStore()
	store.GetOrCreateStateMachine("sm-reset", "idle")
	ctx := discardCtx("STATEM.RESET", bytesArgs("sm-reset"), s)
	err := cmdSTATEMRESET(ctx)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCmdSTATEMISFINAL(t *testing.T) {
	s := store.NewStore()
	store.GetOrCreateStateMachine("sm-final", "idle")
	ctx := discardCtx("STATEM.ISFINAL", bytesArgs("sm-final"), s)
	err := cmdSTATEMISFINAL(ctx)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCmdSTATEMINFO(t *testing.T) {
	s := store.NewStore()
	store.GetOrCreateStateMachine("sm-info", "idle")
	ctx := discardCtx("STATEM.INFO", bytesArgs("sm-info"), s)
	err := cmdSTATEMINFO(ctx)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCmdSTATEMLIST(t *testing.T) {
	s := store.NewStore()
	ctx := discardCtx("STATEM.LIST", bytesArgs(), s)
	err := cmdSTATEMLIST(ctx)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

// Chained and Reactive
func TestCmdCHAINEDSET_Success(t *testing.T) {
	s := store.NewStore()
	ctx := discardCtx("CHAINED.SET", bytesArgs("root", "path.a", "val"), s)
	err := cmdCHAINEDSET(ctx)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCmdCHAINEDGET_Success(t *testing.T) {
	s := store.NewStore()
	cmdCHAINEDSET(discardCtx("CHAINED.SET", bytesArgs("root2", "path.a", "val"), s))
	ctx := discardCtx("CHAINED.GET", bytesArgs("root2", "path.a"), s)
	err := cmdCHAINEDGET(ctx)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCmdCHAINEDDEL_Success(t *testing.T) {
	s := store.NewStore()
	cmdCHAINEDSET(discardCtx("CHAINED.SET", bytesArgs("root3", "path.a", "val"), s))
	ctx := discardCtx("CHAINED.DEL", bytesArgs("root3", "path.a"), s)
	err := cmdCHAINEDDEL(ctx)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCmdREACTIVEWATCH_Success(t *testing.T) {
	s := store.NewStore()
	ctx := discardCtx("REACTIVE.WATCH", bytesArgs("key1", "callback1"), s)
	err := cmdREACTIVEWATCH(ctx)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCmdREACTIVEUNWATCH_Success(t *testing.T) {
	s := store.NewStore()
	cmdREACTIVEWATCH(discardCtx("REACTIVE.WATCH", bytesArgs("unwkey", "cb1"), s))
	ctx := discardCtx("REACTIVE.UNWATCH", bytesArgs("unwkey", "cb1"), s)
	err := cmdREACTIVEUNWATCH(ctx)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCmdREACTIVETRIGGER_Success(t *testing.T) {
	s := store.NewStore()
	ctx := discardCtx("REACTIVE.TRIGGER", bytesArgs("trigkey"), s)
	err := cmdREACTIVETRIGGER(ctx)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

// =============================================================================
// REPLICATION COMMANDS
// =============================================================================

func TestCmdREPLCONF_Subcommands(t *testing.T) {
	s := store.NewStore()
	InitReplicationManager(s)
	clientID := int64(99900)

	// LISTENING-PORT
	ctx := discardCtx("REPLCONF", bytesArgs("LISTENING-PORT", "6380"), s)
	ctx.ClientID = clientID
	ctx.RemoteAddr = "127.0.0.1:12345"
	if err := cmdREPLCONF(ctx); err != nil {
		t.Fatalf("LISTENING-PORT: %v", err)
	}

	// IP-ADDRESS (reuses same clientID)
	ctx2 := discardCtx("REPLCONF", bytesArgs("IP-ADDRESS", "10.0.0.1"), s)
	ctx2.ClientID = clientID
	if err := cmdREPLCONF(ctx2); err != nil {
		t.Fatalf("IP-ADDRESS: %v", err)
	}

	// CAPA (reuses same clientID)
	ctx3 := discardCtx("REPLCONF", bytesArgs("CAPA", "eof", "psync2"), s)
	ctx3.ClientID = clientID
	ctx3.RemoteAddr = "127.0.0.1:12345"
	if err := cmdREPLCONF(ctx3); err != nil {
		t.Fatalf("CAPA: %v", err)
	}

	// ACK
	ctx4 := discardCtx("REPLCONF", bytesArgs("ACK", "0"), s)
	ctx4.ClientID = clientID
	if err := cmdREPLCONF(ctx4); err != nil {
		t.Fatalf("ACK: %v", err)
	}

	// Clean up global state to avoid interfering with other tests
	replManager.RemoveReplica(clientID)
}

func TestCmdSYNC_Success(t *testing.T) {
	s := store.NewStore()
	// InitReplicationManager removed — singleton already initialized by other tests
	ctx := discardCtx("SYNC", bytesArgs(), s)
	err := cmdSYNC(ctx)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCmdPSYNC_FullResync(t *testing.T) {
	s := store.NewStore()
	// InitReplicationManager removed — singleton already initialized by other tests
	ctx := discardCtx("PSYNC", bytesArgs("?", "-1"), s)
	err := cmdPSYNC(ctx)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCmdREPLICAOF_NoOne(t *testing.T) {
	s := store.NewStore()
	// InitReplicationManager removed — singleton already initialized by other tests
	ctx := discardCtx("REPLICAOF", bytesArgs("NO", "ONE"), s)
	err := cmdREPLICAOF(ctx)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCmdREPLICAOF_Host(t *testing.T) {
	s := store.NewStore()
	// InitReplicationManager removed — singleton already initialized by other tests
	ctx := discardCtx("REPLICAOF", bytesArgs("127.0.0.1", "6379"), s)
	err := cmdREPLICAOF(ctx)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

// =============================================================================
// MONITORING COMMANDS
// =============================================================================

func TestCmdMETRICSGET(t *testing.T) {
	s := store.NewStore()
	ctx := discardCtx("METRICS.GET", bytesArgs(), s)
	err := cmdMETRICSGET(ctx)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCmdMETRICSRESET(t *testing.T) {
	s := store.NewStore()
	ctx := discardCtx("METRICS.RESET", bytesArgs(), s)
	err := cmdMETRICSRESET(ctx)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCmdMETRICSCMD(t *testing.T) {
	s := store.NewStore()
	ctx := discardCtx("METRICS.CMD", bytesArgs("GET"), s)
	err := cmdMETRICSCMD(ctx)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCmdSLOWLOGGET(t *testing.T) {
	s := store.NewStore()
	ctx := discardCtx("SLOWLOG.GET", bytesArgs(), s)
	err := cmdSLOWLOGGET(ctx)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCmdSLOWLOGLEN(t *testing.T) {
	s := store.NewStore()
	ctx := discardCtx("SLOWLOG.LEN", bytesArgs(), s)
	err := cmdSLOWLOGLEN(ctx)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCmdSLOWLOGRESET(t *testing.T) {
	s := store.NewStore()
	ctx := discardCtx("SLOWLOG.RESET", bytesArgs(), s)
	err := cmdSLOWLOGRESET(ctx)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCmdSLOWLOGCONFIG(t *testing.T) {
	s := store.NewStore()
	ctx := discardCtx("SLOWLOG.CONFIG", bytesArgs("THRESHOLD", "100"), s)
	err := cmdSLOWLOGCONFIG(ctx)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCmdSTATSKEYSPACE(t *testing.T) {
	s := store.NewStore()
	ctx := discardCtx("STATS.KEYSPACE", bytesArgs(), s)
	err := cmdSTATSKEYSPACE(ctx)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCmdSTATSMEMORY(t *testing.T) {
	s := store.NewStore()
	ctx := discardCtx("STATS.MEMORY", bytesArgs(), s)
	err := cmdSTATSMEMORY(ctx)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCmdSTATSCPU(t *testing.T) {
	s := store.NewStore()
	ctx := discardCtx("STATS.CPU", bytesArgs(), s)
	err := cmdSTATSCPU(ctx)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCmdSTATSCLIENTS(t *testing.T) {
	s := store.NewStore()
	ctx := discardCtx("STATS.CLIENTS", bytesArgs(), s)
	err := cmdSTATSCLIENTS(ctx)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCmdSTATSALL(t *testing.T) {
	s := store.NewStore()
	ctx := discardCtx("STATS.ALL", bytesArgs(), s)
	err := cmdSTATSALL(ctx)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCmdHEALTHCHECK(t *testing.T) {
	s := store.NewStore()
	ctx := discardCtx("HEALTH.CHECK", bytesArgs(), s)
	err := cmdHEALTHCHECK(ctx)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCmdHEALTHLIVENESS(t *testing.T) {
	s := store.NewStore()
	ctx := discardCtx("HEALTH.LIVENESS", bytesArgs(), s)
	err := cmdHEALTHLIVENESS(ctx)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCmdHEALTHREADINESS(t *testing.T) {
	s := store.NewStore()
	ctx := discardCtx("HEALTH.READINESS", bytesArgs(), s)
	err := cmdHEALTHREADINESS(ctx)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

// =============================================================================
// CLUSTER COMMANDS
// =============================================================================

func TestCmdCLUSTER_INFO(t *testing.T) {
	s := store.NewStore()
	ctx := discardCtx("CLUSTER", bytesArgs("INFO"), s)
	err := cmdCLUSTER(ctx)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCmdCLUSTER_NODES(t *testing.T) {
	s := store.NewStore()
	ctx := discardCtx("CLUSTER", bytesArgs("NODES"), s)
	err := cmdCLUSTER(ctx)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCmdCLUSTER_SLOTS(t *testing.T) {
	s := store.NewStore()
	ctx := discardCtx("CLUSTER", bytesArgs("SLOTS"), s)
	err := cmdCLUSTER(ctx)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCmdCLUSTER_MYID(t *testing.T) {
	s := store.NewStore()
	ctx := discardCtx("CLUSTER", bytesArgs("MYID"), s)
	err := cmdCLUSTER(ctx)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCmdCLUSTER_RESET(t *testing.T) {
	s := store.NewStore()
	ctx := discardCtx("CLUSTER", bytesArgs("RESET"), s)
	err := cmdCLUSTER(ctx)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCmdCLUSTER_FAILOVER(t *testing.T) {
	s := store.NewStore()
	ctx := discardCtx("CLUSTER", bytesArgs("FAILOVER"), s)
	err := cmdCLUSTER(ctx)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCmdCLUSTERINFO_Direct(t *testing.T) {
	s := store.NewStore()
	ctx := discardCtx("CLUSTERINFO", bytesArgs(), s)
	err := cmdCLUSTERINFO(ctx)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCmdCLUSTERNODES_Direct(t *testing.T) {
	s := store.NewStore()
	ctx := discardCtx("CLUSTERNODES", bytesArgs(), s)
	err := cmdCLUSTERNODES(ctx)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCmdCLUSTERSLOTS_Direct(t *testing.T) {
	s := store.NewStore()
	ctx := discardCtx("CLUSTERSLOTS", bytesArgs(), s)
	err := cmdCLUSTERSLOTS(ctx)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCmdMIGRATE_Success(t *testing.T) {
	s := store.NewStore()
	s.Set("migkey", &store.StringValue{Data: []byte("v")}, store.SetOptions{})
	ctx := discardCtx("MIGRATE", bytesArgs("127.0.0.1", "6380", "migkey", "0"), s)
	err := cmdMIGRATE(ctx)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCmdASKING(t *testing.T) {
	s := store.NewStore()
	ctx := discardCtx("ASKING", bytesArgs(), s)
	err := cmdASKING(ctx)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCmdREADONLY(t *testing.T) {
	s := store.NewStore()
	ctx := discardCtx("READONLY", bytesArgs(), s)
	err := cmdREADONLY(ctx)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCmdREADWRITE(t *testing.T) {
	s := store.NewStore()
	ctx := discardCtx("READWRITE", bytesArgs(), s)
	err := cmdREADWRITE(ctx)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

// =============================================================================
// SCHEDULER COMMANDS (jobs, circuits, sessions)
// =============================================================================

func TestCmdJOBCREATE_Success(t *testing.T) {
	s := store.NewStore()
	ctx := discardCtx("JOB.CREATE", bytesArgs("j1", "myjob", "echo hello", "1000"), s)
	err := cmdJOBCREATE(ctx)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCmdJOBDELETE_Success(t *testing.T) {
	s := store.NewStore()
	store.GlobalJobScheduler.Create("j-del", "myjob", "cmd", 1000)
	ctx := discardCtx("JOB.DELETE", bytesArgs("j-del"), s)
	err := cmdJOBDELETE(ctx)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCmdJOBGET_Success(t *testing.T) {
	s := store.NewStore()
	store.GlobalJobScheduler.Create("j-get", "myjob", "cmd", 1000)
	ctx := discardCtx("JOB.GET", bytesArgs("j-get"), s)
	err := cmdJOBGET(ctx)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCmdJOBLIST(t *testing.T) {
	s := store.NewStore()
	ctx := discardCtx("JOB.LIST", bytesArgs(), s)
	err := cmdJOBLIST(ctx)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCmdJOBENABLE_Success(t *testing.T) {
	s := store.NewStore()
	store.GlobalJobScheduler.Create("j-en", "myjob", "cmd", 1000)
	ctx := discardCtx("JOB.ENABLE", bytesArgs("j-en"), s)
	err := cmdJOBENABLE(ctx)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCmdJOBDISABLE_Success(t *testing.T) {
	s := store.NewStore()
	store.GlobalJobScheduler.Create("j-dis", "myjob", "cmd", 1000)
	ctx := discardCtx("JOB.DISABLE", bytesArgs("j-dis"), s)
	err := cmdJOBDISABLE(ctx)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCmdJOBRUN_Success(t *testing.T) {
	s := store.NewStore()
	store.GlobalJobScheduler.Create("j-run", "myjob", "echo ok", 1000)
	ctx := discardCtx("JOB.RUN", bytesArgs("j-run"), s)
	err := cmdJOBRUN(ctx)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCmdJOBSTATS_Success(t *testing.T) {
	s := store.NewStore()
	store.GlobalJobScheduler.Create("j-stats", "myjob", "cmd", 1000)
	ctx := discardCtx("JOB.STATS", bytesArgs("j-stats"), s)
	err := cmdJOBSTATS(ctx)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCmdJOBRESET_Success(t *testing.T) {
	s := store.NewStore()
	store.GlobalJobScheduler.Create("j-reset", "myjob", "cmd", 1000)
	ctx := discardCtx("JOB.RESET", bytesArgs("j-reset"), s)
	err := cmdJOBRESET(ctx)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCmdJOBUPDATE_Success(t *testing.T) {
	s := store.NewStore()
	store.GlobalJobScheduler.Create("j-up", "myjob", "cmd", 1000)
	ctx := discardCtx("JOB.UPDATE", bytesArgs("j-up", "2000"), s)
	err := cmdJOBUPDATE(ctx)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCmdCIRCUITCREATE_Success(t *testing.T) {
	s := store.NewStore()
	ctx := discardCtx("CIRCUIT.CREATE", bytesArgs("cb1", "5", "2", "30000"), s)
	err := cmdCIRCUITCREATE(ctx)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCmdCIRCUITDELETE_Success(t *testing.T) {
	s := store.NewStore()
	store.GetOrCreateCircuitBreaker("cb-del", 5, 2, 30000)
	ctx := discardCtx("CIRCUIT.DELETE", bytesArgs("cb-del"), s)
	err := cmdCIRCUITDELETE(ctx)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCmdCIRCUITALLOW_Success(t *testing.T) {
	s := store.NewStore()
	store.GetOrCreateCircuitBreaker("cb-allow", 5, 2, 30000)
	ctx := discardCtx("CIRCUIT.ALLOW", bytesArgs("cb-allow"), s)
	err := cmdCIRCUITALLOW(ctx)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCmdCIRCUITSUCCESS_Success(t *testing.T) {
	s := store.NewStore()
	store.GetOrCreateCircuitBreaker("cb-succ", 5, 2, 30000)
	ctx := discardCtx("CIRCUIT.SUCCESS", bytesArgs("cb-succ"), s)
	err := cmdCIRCUITSUCCESS(ctx)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCmdCIRCUITFAILURE_Success(t *testing.T) {
	s := store.NewStore()
	store.GetOrCreateCircuitBreaker("cb-fail", 5, 2, 30000)
	ctx := discardCtx("CIRCUIT.FAILURE", bytesArgs("cb-fail"), s)
	err := cmdCIRCUITFAILURE(ctx)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCmdCIRCUITSTATE_Success(t *testing.T) {
	s := store.NewStore()
	store.GetOrCreateCircuitBreaker("cb-state", 5, 2, 30000)
	ctx := discardCtx("CIRCUIT.STATE", bytesArgs("cb-state"), s)
	err := cmdCIRCUITSTATE(ctx)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCmdCIRCUITRESET_Success(t *testing.T) {
	s := store.NewStore()
	store.GetOrCreateCircuitBreaker("cb-reset", 5, 2, 30000)
	ctx := discardCtx("CIRCUIT.RESET", bytesArgs("cb-reset"), s)
	err := cmdCIRCUITRESET(ctx)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCmdCIRCUITSTATS_Success(t *testing.T) {
	s := store.NewStore()
	store.GetOrCreateCircuitBreaker("cb-stats", 5, 2, 30000)
	ctx := discardCtx("CIRCUIT.STATS", bytesArgs("cb-stats"), s)
	err := cmdCIRCUITSTATS(ctx)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCmdCIRCUITLIST(t *testing.T) {
	s := store.NewStore()
	ctx := discardCtx("CIRCUIT.LIST", bytesArgs(), s)
	err := cmdCIRCUITLIST(ctx)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCmdSESSIONCREATE_Success(t *testing.T) {
	s := store.NewStore()
	ctx := discardCtx("SESSION.CREATE", bytesArgs("sess1", "60000"), s)
	err := cmdSESSIONCREATE(ctx)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCmdSESSIONSET_Success(t *testing.T) {
	s := store.NewStore()
	store.GlobalSessionManager.Create("sess-set", 60000)
	ctx := discardCtx("SESSION.SET", bytesArgs("sess-set", "k1", "v1"), s)
	err := cmdSESSIONSET(ctx)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCmdSESSIONGET_Success(t *testing.T) {
	s := store.NewStore()
	sess := store.GlobalSessionManager.Create("sess-get", 60000)
	sess.Set("k1", "v1")
	ctx := discardCtx("SESSION.GET", bytesArgs("sess-get", "k1"), s)
	err := cmdSESSIONGET(ctx)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCmdSESSIONDELETE_Success(t *testing.T) {
	s := store.NewStore()
	store.GlobalSessionManager.Create("sess-del", 60000)
	ctx := discardCtx("SESSION.DELETE", bytesArgs("sess-del"), s)
	err := cmdSESSIONDELETE(ctx)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCmdSESSIONEXISTS(t *testing.T) {
	s := store.NewStore()
	store.GlobalSessionManager.Create("sess-ex", 60000)
	ctx := discardCtx("SESSION.EXISTS", bytesArgs("sess-ex"), s)
	err := cmdSESSIONEXISTS(ctx)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCmdSESSIONTTL(t *testing.T) {
	s := store.NewStore()
	store.GlobalSessionManager.Create("sess-ttl", 60000)
	ctx := discardCtx("SESSION.TTL", bytesArgs("sess-ttl"), s)
	err := cmdSESSIONTTL(ctx)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCmdSESSIONREFRESH(t *testing.T) {
	s := store.NewStore()
	store.GlobalSessionManager.Create("sess-ref", 60000)
	ctx := discardCtx("SESSION.REFRESH", bytesArgs("sess-ref", "120000"), s)
	err := cmdSESSIONREFRESH(ctx)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCmdSESSIONCLEAR(t *testing.T) {
	s := store.NewStore()
	store.GlobalSessionManager.Create("sess-clr", 60000)
	ctx := discardCtx("SESSION.CLEAR", bytesArgs("sess-clr"), s)
	err := cmdSESSIONCLEAR(ctx)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCmdSESSIONALL(t *testing.T) {
	s := store.NewStore()
	store.GlobalSessionManager.Create("sess-all", 60000)
	ctx := discardCtx("SESSION.ALL", bytesArgs("sess-all"), s)
	err := cmdSESSIONALL(ctx)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCmdSESSIONLIST(t *testing.T) {
	s := store.NewStore()
	ctx := discardCtx("SESSION.LIST", bytesArgs(), s)
	err := cmdSESSIONLIST(ctx)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCmdSESSIONCOUNT(t *testing.T) {
	s := store.NewStore()
	ctx := discardCtx("SESSION.COUNT", bytesArgs(), s)
	err := cmdSESSIONCOUNT(ctx)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCmdSESSIONCLEANUP(t *testing.T) {
	s := store.NewStore()
	ctx := discardCtx("SESSION.CLEANUP", bytesArgs(), s)
	err := cmdSESSIONCLEANUP(ctx)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

// =============================================================================
// DS COMMANDS (priority queues, LRU, token/leaky buckets, sliding window, debounce, throttle)
// =============================================================================

func TestCmdPQCREATE_Success(t *testing.T) {
	s := store.NewStore()
	ctx := discardCtx("PQ.CREATE", bytesArgs("pq1"), s)
	err := cmdPQCREATE(ctx)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCmdPQPUSH_Success(t *testing.T) {
	s := store.NewStore()
	ctx := discardCtx("PQ.PUSH", bytesArgs("pq-push", "item1", "5"), s)
	err := cmdPQPUSH(ctx)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCmdPQPOP_Success(t *testing.T) {
	s := store.NewStore()
	cmdPQPUSH(discardCtx("PQ.PUSH", bytesArgs("pq-pop", "item1", "5"), s))
	ctx := discardCtx("PQ.POP", bytesArgs("pq-pop"), s)
	err := cmdPQPOP(ctx)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCmdPQPEEK_Success(t *testing.T) {
	s := store.NewStore()
	cmdPQPUSH(discardCtx("PQ.PUSH", bytesArgs("pq-peek", "item1", "5"), s))
	ctx := discardCtx("PQ.PEEK", bytesArgs("pq-peek"), s)
	err := cmdPQPEEK(ctx)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCmdPQLEN_Success(t *testing.T) {
	s := store.NewStore()
	ctx := discardCtx("PQ.LEN", bytesArgs("pq-len"), s)
	err := cmdPQLEN(ctx)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCmdPQCLEAR_Success(t *testing.T) {
	s := store.NewStore()
	ctx := discardCtx("PQ.CLEAR", bytesArgs("pq-clear"), s)
	err := cmdPQCLEAR(ctx)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCmdPQGETALL_Success(t *testing.T) {
	s := store.NewStore()
	cmdPQPUSH(discardCtx("PQ.PUSH", bytesArgs("pq-all", "item1", "5"), s))
	ctx := discardCtx("PQ.GETALL", bytesArgs("pq-all"), s)
	err := cmdPQGETALL(ctx)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCmdLRUCREATE_Success(t *testing.T) {
	s := store.NewStore()
	ctx := discardCtx("LRU.CREATE", bytesArgs("lru1", "100"), s)
	err := cmdLRUCREATE(ctx)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCmdLRUSET_Success(t *testing.T) {
	s := store.NewStore()
	cmdLRUCREATE(discardCtx("LRU.CREATE", bytesArgs("lru-set", "10"), s))
	ctx := discardCtx("LRU.SET", bytesArgs("lru-set", "k1", "v1"), s)
	err := cmdLRUSET(ctx)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCmdLRUGET_Success(t *testing.T) {
	s := store.NewStore()
	cmdLRUCREATE(discardCtx("LRU.CREATE", bytesArgs("lru-get", "10"), s))
	cmdLRUSET(discardCtx("LRU.SET", bytesArgs("lru-get", "k1", "v1"), s))
	ctx := discardCtx("LRU.GET", bytesArgs("lru-get", "k1"), s)
	err := cmdLRUGET(ctx)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCmdLRUDEL_Success(t *testing.T) {
	s := store.NewStore()
	cmdLRUCREATE(discardCtx("LRU.CREATE", bytesArgs("lru-del", "10"), s))
	ctx := discardCtx("LRU.DEL", bytesArgs("lru-del", "k1"), s)
	err := cmdLRUDEL(ctx)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCmdLRUCLEAR_Success(t *testing.T) {
	s := store.NewStore()
	cmdLRUCREATE(discardCtx("LRU.CREATE", bytesArgs("lru-clr", "10"), s))
	ctx := discardCtx("LRU.CLEAR", bytesArgs("lru-clr"), s)
	err := cmdLRUCLEAR(ctx)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCmdLRUKEYS_Success(t *testing.T) {
	s := store.NewStore()
	cmdLRUCREATE(discardCtx("LRU.CREATE", bytesArgs("lru-keys", "10"), s))
	ctx := discardCtx("LRU.KEYS", bytesArgs("lru-keys"), s)
	err := cmdLRUKEYS(ctx)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCmdLRUSTATS_Success(t *testing.T) {
	s := store.NewStore()
	cmdLRUCREATE(discardCtx("LRU.CREATE", bytesArgs("lru-stats", "10"), s))
	ctx := discardCtx("LRU.STATS", bytesArgs("lru-stats"), s)
	err := cmdLRUSTATS(ctx)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCmdTOKENBUCKETCREATE_Success(t *testing.T) {
	s := store.NewStore()
	ctx := discardCtx("TOKENBUCKET.CREATE", bytesArgs("tb1", "10", "1"), s)
	err := cmdTOKENBUCKETCREATE(ctx)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCmdTOKENBUCKETCONSUME_Success(t *testing.T) {
	s := store.NewStore()
	cmdTOKENBUCKETCREATE(discardCtx("TOKENBUCKET.CREATE", bytesArgs("tb-con", "10", "1"), s))
	ctx := discardCtx("TOKENBUCKET.CONSUME", bytesArgs("tb-con", "1"), s)
	err := cmdTOKENBUCKETCONSUME(ctx)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCmdTOKENBUCKETAVAILABLE_Success(t *testing.T) {
	s := store.NewStore()
	cmdTOKENBUCKETCREATE(discardCtx("TOKENBUCKET.CREATE", bytesArgs("tb-avail", "10", "1"), s))
	ctx := discardCtx("TOKENBUCKET.AVAILABLE", bytesArgs("tb-avail"), s)
	err := cmdTOKENBUCKETAVAILABLE(ctx)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCmdTOKENBUCKETRESET_Success(t *testing.T) {
	s := store.NewStore()
	cmdTOKENBUCKETCREATE(discardCtx("TOKENBUCKET.CREATE", bytesArgs("tb-reset", "10", "1"), s))
	ctx := discardCtx("TOKENBUCKET.RESET", bytesArgs("tb-reset"), s)
	err := cmdTOKENBUCKETRESET(ctx)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCmdTOKENBUCKETDELETE_Success(t *testing.T) {
	s := store.NewStore()
	cmdTOKENBUCKETCREATE(discardCtx("TOKENBUCKET.CREATE", bytesArgs("tb-del", "10", "1"), s))
	ctx := discardCtx("TOKENBUCKET.DELETE", bytesArgs("tb-del"), s)
	err := cmdTOKENBUCKETDELETE(ctx)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCmdDEBOUNCESET_Success(t *testing.T) {
	s := store.NewStore()
	ctx := discardCtx("DEBOUNCE.SET", bytesArgs("db1", "val", "1000"), s)
	err := cmdDEBOUNCESET(ctx)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCmdDEBOUNCEGET_Success(t *testing.T) {
	s := store.NewStore()
	cmdDEBOUNCESET(discardCtx("DEBOUNCE.SET", bytesArgs("db-get", "val", "1000"), s))
	ctx := discardCtx("DEBOUNCE.GET", bytesArgs("db-get"), s)
	err := cmdDEBOUNCEGET(ctx)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCmdDEBOUNCECALL_Success(t *testing.T) {
	s := store.NewStore()
	ctx := discardCtx("DEBOUNCE.CALL", bytesArgs("db-call"), s)
	err := cmdDEBOUNCECALL(ctx)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCmdDEBOUNCEDELETE_Success(t *testing.T) {
	s := store.NewStore()
	ctx := discardCtx("DEBOUNCE.DELETE", bytesArgs("db-del"), s)
	err := cmdDEBOUNCEDELETE(ctx)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCmdTHROTTLESET_Success(t *testing.T) {
	s := store.NewStore()
	ctx := discardCtx("THROTTLE.SET", bytesArgs("th1", "1000"), s)
	err := cmdTHROTTLESET(ctx)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCmdTHROTTLECALL_Success(t *testing.T) {
	s := store.NewStore()
	cmdTHROTTLESET(discardCtx("THROTTLE.SET", bytesArgs("th-call", "1000"), s))
	ctx := discardCtx("THROTTLE.CALL", bytesArgs("th-call"), s)
	err := cmdTHROTTLECALL(ctx)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCmdTHROTTLERESET_Success(t *testing.T) {
	s := store.NewStore()
	cmdTHROTTLESET(discardCtx("THROTTLE.SET", bytesArgs("th-reset", "1000"), s))
	ctx := discardCtx("THROTTLE.RESET", bytesArgs("th-reset"), s)
	err := cmdTHROTTLERESET(ctx)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCmdTHROTTLEDELETE_Success(t *testing.T) {
	s := store.NewStore()
	cmdTHROTTLESET(discardCtx("THROTTLE.SET", bytesArgs("th-del", "1000"), s))
	ctx := discardCtx("THROTTLE.DELETE", bytesArgs("th-del"), s)
	err := cmdTHROTTLEDELETE(ctx)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

// =============================================================================
// ML COMMANDS
// =============================================================================

func TestCmdMODELCREATE_Success(t *testing.T) {
	s := store.NewStore()
	ctx := discardCtx("MODEL.CREATE", bytesArgs("m1", "linear"), s)
	err := cmdMODELCREATE(ctx)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCmdMODELTRAIN_Success(t *testing.T) {
	s := store.NewStore()
	cmdMODELCREATE(discardCtx("MODEL.CREATE", bytesArgs("m-tr", "linear"), s))
	ctx := discardCtx("MODEL.TRAIN", bytesArgs("m-tr", "data"), s)
	err := cmdMODELTRAIN(ctx)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCmdMODELPREDICT_Success(t *testing.T) {
	s := store.NewStore()
	cmdMODELCREATE(discardCtx("MODEL.CREATE", bytesArgs("m-pred", "linear"), s))
	ctx := discardCtx("MODEL.PREDICT", bytesArgs("m-pred", "input"), s)
	err := cmdMODELPREDICT(ctx)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCmdMODELDELETE_Success(t *testing.T) {
	s := store.NewStore()
	cmdMODELCREATE(discardCtx("MODEL.CREATE", bytesArgs("m-del", "linear"), s))
	ctx := discardCtx("MODEL.DELETE", bytesArgs("m-del"), s)
	err := cmdMODELDELETE(ctx)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCmdMODELLIST_Success(t *testing.T) {
	s := store.NewStore()
	ctx := discardCtx("MODEL.LIST", bytesArgs(), s)
	err := cmdMODELLIST(ctx)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCmdMODELSTATUS_Success(t *testing.T) {
	s := store.NewStore()
	cmdMODELCREATE(discardCtx("MODEL.CREATE", bytesArgs("m-status", "linear"), s))
	ctx := discardCtx("MODEL.STATUS", bytesArgs("m-status"), s)
	err := cmdMODELSTATUS(ctx)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCmdFEATURESET_Success(t *testing.T) {
	s := store.NewStore()
	ctx := discardCtx("FEATURE.SET", bytesArgs("entity1", "feat1", "1.5"), s)
	err := cmdFEATURESET(ctx)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCmdFEATUREGET_Success(t *testing.T) {
	s := store.NewStore()
	cmdFEATURESET(discardCtx("FEATURE.SET", bytesArgs("e-get", "f1", "2.5"), s))
	ctx := discardCtx("FEATURE.GET", bytesArgs("e-get", "f1"), s)
	err := cmdFEATUREGET(ctx)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCmdFEATUREDEL_Success(t *testing.T) {
	s := store.NewStore()
	cmdFEATURESET(discardCtx("FEATURE.SET", bytesArgs("e-del", "f1", "2.5"), s))
	ctx := discardCtx("FEATURE.DEL", bytesArgs("e-del", "f1"), s)
	err := cmdFEATUREDEL(ctx)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCmdFEATUREINCR_Success(t *testing.T) {
	s := store.NewStore()
	ctx := discardCtx("FEATURE.INCR", bytesArgs("e-incr", "f1", "1.0"), s)
	err := cmdFEATUREINCR(ctx)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCmdSENTIMENTANALYZE_Success(t *testing.T) {
	s := store.NewStore()
	ctx := discardCtx("SENTIMENT.ANALYZE", bytesArgs("I love this product"), s)
	err := cmdSENTIMENTANALYZE(ctx)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCmdNLPTOKENIZE_Success(t *testing.T) {
	s := store.NewStore()
	ctx := discardCtx("NLP.TOKENIZE", bytesArgs("hello world"), s)
	err := cmdNLPTOKENIZE(ctx)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCmdSIMILARITYCOSINE_Success(t *testing.T) {
	s := store.NewStore()
	ctx := discardCtx("SIMILARITY.COSINE", bytesArgs("1,0", "0,1"), s)
	err := cmdSIMILARITYCOSINE(ctx)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

// =============================================================================
// FUNCTION COMMANDS
// =============================================================================

func TestCmdFUNCTION_LIST(t *testing.T) {
	s := store.NewStore()
	ctx := discardCtx("FUNCTION", bytesArgs("LIST"), s)
	err := cmdFUNCTION(ctx)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCmdFUNCTION_STATS(t *testing.T) {
	s := store.NewStore()
	ctx := discardCtx("FUNCTION", bytesArgs("STATS"), s)
	err := cmdFUNCTION(ctx)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCmdFUNCTION_FLUSH(t *testing.T) {
	s := store.NewStore()
	ctx := discardCtx("FUNCTION", bytesArgs("FLUSH"), s)
	err := cmdFUNCTION(ctx)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCmdFUNCTION_DUMP(t *testing.T) {
	s := store.NewStore()
	ctx := discardCtx("FUNCTION", bytesArgs("DUMP"), s)
	err := cmdFUNCTION(ctx)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCmdFUNCTION_CREATE(t *testing.T) {
	s := store.NewStore()
	code := `redis = {hello = function() return "world" end}`
	ctx := discardCtx("FUNCTION", bytesArgs("CREATE", "mylib", code), s)
	err := cmdFUNCTION(ctx)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCmdFUNCTION_DELETE(t *testing.T) {
	s := store.NewStore()
	code := `redis = {hello = function() return "world" end}`
	cmdFUNCTION(discardCtx("FUNCTION", bytesArgs("CREATE", "dellib", code), s))
	ctx := discardCtx("FUNCTION", bytesArgs("DELETE", "dellib"), s)
	err := cmdFUNCTION(ctx)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCmdFUNCTION_WrongArgs(t *testing.T) {
	s := store.NewStore()
	ctx := discardCtx("FUNCTION", bytesArgs(), s)
	err := cmdFUNCTION(ctx)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

// =============================================================================
// TEMPLATE COMMANDS (eval, validate, string utilities)
// =============================================================================

func TestCmdEVALEXPR_Success(t *testing.T) {
	s := store.NewStore()
	ctx := discardCtx("EVAL.EXPR", bytesArgs("2+3"), s)
	err := cmdEVALEXPR(ctx)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCmdEVALFORMAT_Success(t *testing.T) {
	s := store.NewStore()
	ctx := discardCtx("EVAL.FORMAT", bytesArgs("Hello {}", "World"), s)
	err := cmdEVALFORMAT(ctx)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCmdEVALJSONPATH_Success(t *testing.T) {
	s := store.NewStore()
	ctx := discardCtx("EVAL.JSONPATH", bytesArgs(`{"name":"Alice"}`, "$.name"), s)
	err := cmdEVALJSONPATH(ctx)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCmdEVALTEMPLATE_Success(t *testing.T) {
	s := store.NewStore()
	ctx := discardCtx("EVAL.TEMPLATE", bytesArgs("Hello {{name}}", "name", "World"), s)
	err := cmdEVALTEMPLATE(ctx)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCmdEVALREGEX_Success(t *testing.T) {
	s := store.NewStore()
	ctx := discardCtx("EVAL.REGEX", bytesArgs("hello", "hello world"), s)
	err := cmdEVALREGEX(ctx)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCmdEVALREGEXMATCH_Success(t *testing.T) {
	s := store.NewStore()
	ctx := discardCtx("EVAL.REGEXMATCH", bytesArgs("hello", "hello hello"), s)
	err := cmdEVALREGEXMATCH(ctx)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCmdEVALREGEXREPLACE_Success(t *testing.T) {
	s := store.NewStore()
	ctx := discardCtx("EVAL.REGEXREPLACE", bytesArgs("world", "earth", "hello world"), s)
	err := cmdEVALREGEXREPLACE(ctx)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCmdVALIDATEEMAIL_Success(t *testing.T) {
	s := store.NewStore()
	ctx := discardCtx("VALIDATE.EMAIL", bytesArgs("test@example.com"), s)
	err := cmdVALIDATEEMAIL(ctx)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCmdVALIDATEURL_Success(t *testing.T) {
	s := store.NewStore()
	ctx := discardCtx("VALIDATE.URL", bytesArgs("https://example.com"), s)
	err := cmdVALIDATEURL(ctx)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCmdVALIDATEIP_Success(t *testing.T) {
	s := store.NewStore()
	ctx := discardCtx("VALIDATE.IP", bytesArgs("192.168.1.1"), s)
	err := cmdVALIDATEIP(ctx)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCmdVALIDATEJSON_Success(t *testing.T) {
	s := store.NewStore()
	ctx := discardCtx("VALIDATE.JSON", bytesArgs(`{"key":"val"}`), s)
	err := cmdVALIDATEJSON(ctx)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCmdVALIDATEINT_Success(t *testing.T) {
	s := store.NewStore()
	ctx := discardCtx("VALIDATE.INT", bytesArgs("42"), s)
	err := cmdVALIDATEINT(ctx)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCmdVALIDATEFLOAT_Success(t *testing.T) {
	s := store.NewStore()
	ctx := discardCtx("VALIDATE.FLOAT", bytesArgs("3.14"), s)
	err := cmdVALIDATEFLOAT(ctx)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCmdVALIDATEALPHA_Success(t *testing.T) {
	s := store.NewStore()
	ctx := discardCtx("VALIDATE.ALPHA", bytesArgs("abcDEF"), s)
	err := cmdVALIDATEALPHA(ctx)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCmdVALIDATEALPHANUM_Success(t *testing.T) {
	s := store.NewStore()
	ctx := discardCtx("VALIDATE.ALPHANUM", bytesArgs("abc123"), s)
	err := cmdVALIDATEALPHANUM(ctx)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCmdVALIDATELENGTH_Success(t *testing.T) {
	s := store.NewStore()
	ctx := discardCtx("VALIDATE.LENGTH", bytesArgs("hello", "1", "10"), s)
	err := cmdVALIDATELENGTH(ctx)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCmdVALIDATERANGE_Success(t *testing.T) {
	s := store.NewStore()
	ctx := discardCtx("VALIDATE.RANGE", bytesArgs("5", "1", "10"), s)
	err := cmdVALIDATERANGE(ctx)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCmdSTRREVERSE_Success(t *testing.T) {
	s := store.NewStore()
	ctx, buf := bufCtx("STR.REVERSE", bytesArgs("hello"), s)
	err := cmdSTRREVERSE(ctx)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(buf.String(), "olleh") {
		t.Errorf("expected olleh, got %q", buf.String())
	}
}

func TestCmdSTRREPEAT_Success(t *testing.T) {
	s := store.NewStore()
	ctx := discardCtx("STR.REPEAT", bytesArgs("ab", "3"), s)
	err := cmdSTRREPEAT(ctx)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCmdSTRSPLIT_Success(t *testing.T) {
	s := store.NewStore()
	ctx := discardCtx("STR.SPLIT", bytesArgs("a,b,c", ","), s)
	err := cmdSTRSPLIT(ctx)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCmdSTRJOIN_Success(t *testing.T) {
	s := store.NewStore()
	ctx := discardCtx("STR.JOIN", bytesArgs(",", "a", "b", "c"), s)
	err := cmdSTRJOIN(ctx)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCmdSTRCONTAINS_Success(t *testing.T) {
	s := store.NewStore()
	ctx := discardCtx("STR.CONTAINS", bytesArgs("hello world", "world"), s)
	err := cmdSTRCONTAINS(ctx)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCmdSTRSTARTSWITH_Success(t *testing.T) {
	s := store.NewStore()
	ctx := discardCtx("STR.STARTSWITH", bytesArgs("hello world", "hello"), s)
	err := cmdSTRSTARTSWITH(ctx)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCmdSTRENDSWITH_Success(t *testing.T) {
	s := store.NewStore()
	ctx := discardCtx("STR.ENDSWITH", bytesArgs("hello world", "world"), s)
	err := cmdSTRENDSWITH(ctx)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCmdSTRINDEX_Success(t *testing.T) {
	s := store.NewStore()
	ctx := discardCtx("STR.INDEX", bytesArgs("hello world", "world"), s)
	err := cmdSTRINDEX(ctx)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCmdSTRLASTINDEX_Success(t *testing.T) {
	s := store.NewStore()
	ctx := discardCtx("STR.LASTINDEX", bytesArgs("hello world world", "world"), s)
	err := cmdSTRLASTINDEX(ctx)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCmdSTRREPLACE_Success(t *testing.T) {
	s := store.NewStore()
	ctx := discardCtx("STR.REPLACE", bytesArgs("hello world", "world", "earth"), s)
	err := cmdSTRREPLACE(ctx)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCmdSTRTRIM_Success(t *testing.T) {
	s := store.NewStore()
	ctx := discardCtx("STR.TRIM", bytesArgs("  hello  "), s)
	err := cmdSTRTRIM(ctx)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCmdSTRTRIMLEFT_Success(t *testing.T) {
	s := store.NewStore()
	ctx := discardCtx("STR.TRIMLEFT", bytesArgs("  hello"), s)
	err := cmdSTRTRIMLEFT(ctx)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCmdSTRTRIMRIGHT_Success(t *testing.T) {
	s := store.NewStore()
	ctx := discardCtx("STR.TRIMRIGHT", bytesArgs("hello  "), s)
	err := cmdSTRTRIMRIGHT(ctx)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCmdSTRTITLE_Success(t *testing.T) {
	s := store.NewStore()
	ctx := discardCtx("STR.TITLE", bytesArgs("hello"), s)
	err := cmdSTRTITLE(ctx)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCmdSTRWORDS_Success(t *testing.T) {
	s := store.NewStore()
	ctx := discardCtx("STR.WORDS", bytesArgs("hello beautiful world"), s)
	err := cmdSTRWORDS(ctx)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCmdSTRLINES_Success(t *testing.T) {
	s := store.NewStore()
	ctx := discardCtx("STR.LINES", bytesArgs("line1\nline2\nline3"), s)
	err := cmdSTRLINES(ctx)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCmdSTRTRUNCATE_Success(t *testing.T) {
	s := store.NewStore()
	ctx := discardCtx("STR.TRUNCATE", bytesArgs("hello world", "5"), s)
	err := cmdSTRTRUNCATE(ctx)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCmdSTRPADLEFT_Success(t *testing.T) {
	s := store.NewStore()
	ctx := discardCtx("STR.PADLEFT", bytesArgs("42", "5", "0"), s)
	err := cmdSTRPADLEFT(ctx)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCmdSTRPADRIGHT_Success(t *testing.T) {
	s := store.NewStore()
	ctx := discardCtx("STR.PADRIGHT", bytesArgs("42", "5", "0"), s)
	err := cmdSTRPADRIGHT(ctx)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

// =============================================================================
// UTILITY COMMANDS (rate limiter, locks, IDs)
// =============================================================================

func TestCmdRLCREATE_Success(t *testing.T) {
	s := store.NewStore()
	ctx := discardCtx("RL.CREATE", bytesArgs("rl1", "10", "5", "1000"), s)
	err := cmdRLCREATE(ctx)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCmdRLALLOW_Success(t *testing.T) {
	s := store.NewStore()
	store.GlobalRateLimiter.Create("rl-allow", 10, 5, 1000)
	ctx := discardCtx("RL.ALLOW", bytesArgs("rl-allow", "1"), s)
	err := cmdRLALLOW(ctx)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCmdRLGET_Success(t *testing.T) {
	s := store.NewStore()
	store.GlobalRateLimiter.Create("rl-get", 10, 5, 1000)
	ctx := discardCtx("RL.GET", bytesArgs("rl-get"), s)
	err := cmdRLGET(ctx)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCmdRLDELETE_Success(t *testing.T) {
	s := store.NewStore()
	store.GlobalRateLimiter.Create("rl-del", 10, 5, 1000)
	ctx := discardCtx("RL.DELETE", bytesArgs("rl-del"), s)
	err := cmdRLDELETE(ctx)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCmdRLRESET_Success(t *testing.T) {
	s := store.NewStore()
	store.GlobalRateLimiter.Create("rl-reset", 10, 5, 1000)
	ctx := discardCtx("RL.RESET", bytesArgs("rl-reset"), s)
	err := cmdRLRESET(ctx)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCmdIDCREATE_Success(t *testing.T) {
	s := store.NewStore()
	ctx := discardCtx("ID.CREATE", bytesArgs("seq1", "1", "1"), s)
	err := cmdIDCREATE(ctx)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCmdIDNEXT_Success(t *testing.T) {
	s := store.NewStore()
	store.GlobalIDGenerator.Create("seq-next", 1, 1, "", "", 0)
	ctx := discardCtx("ID.NEXT", bytesArgs("seq-next"), s)
	err := cmdIDNEXT(ctx)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCmdIDCURRENT_Success(t *testing.T) {
	s := store.NewStore()
	store.GlobalIDGenerator.Create("seq-cur", 1, 1, "", "", 0)
	ctx := discardCtx("ID.CURRENT", bytesArgs("seq-cur"), s)
	err := cmdIDCURRENT(ctx)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCmdSNOWFLAKENEXT_Success(t *testing.T) {
	s := store.NewStore()
	ctx := discardCtx("SNOWFLAKE.NEXT", bytesArgs("1"), s)
	err := cmdSNOWFLAKENEXT(ctx)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

// =============================================================================
// DEBUG COMMANDS
// =============================================================================

func TestCmdDEBUG_Reload(t *testing.T) {
	s := store.NewStore()
	ctx := discardCtx("DEBUG", bytesArgs("RELOAD"), s)
	err := cmdDEBUG(ctx)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCmdDEBUG_Digest(t *testing.T) {
	s := store.NewStore()
	ctx := discardCtx("DEBUG", bytesArgs("DIGEST"), s)
	err := cmdDEBUG(ctx)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCmdDEBUG_Segfault(t *testing.T) {
	s := store.NewStore()
	ctx := discardCtx("DEBUG", bytesArgs("SEGFAULT"), s)
	err := cmdDEBUG(ctx)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCmdDEBUG_StructSize(t *testing.T) {
	s := store.NewStore()
	ctx := discardCtx("DEBUG", bytesArgs("STRUCTSIZE"), s)
	err := cmdDEBUG(ctx)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCmdDEBUG_Unknown(t *testing.T) {
	s := store.NewStore()
	ctx := discardCtx("DEBUG", bytesArgs("BADSUBCMD"), s)
	err := cmdDEBUG(ctx)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCmdOBJECT_Encoding(t *testing.T) {
	s := store.NewStore()
	s.Set("k1", &store.StringValue{Data: []byte("v")}, store.SetOptions{})
	ctx := discardCtx("OBJECT", bytesArgs("ENCODING", "k1"), s)
	err := cmdOBJECT(ctx)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCmdOBJECT_FREQ(t *testing.T) {
	s := store.NewStore()
	s.Set("k1", &store.StringValue{Data: []byte("v")}, store.SetOptions{})
	ctx := discardCtx("OBJECT", bytesArgs("FREQ", "k1"), s)
	err := cmdOBJECT(ctx)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCmdOBJECT_Missing(t *testing.T) {
	s := store.NewStore()
	ctx := discardCtx("OBJECT", bytesArgs("ENCODING", "nosuch"), s)
	err := cmdOBJECT(ctx)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCmdMEMORY_Usage(t *testing.T) {
	s := store.NewStore()
	s.Set("k1", &store.StringValue{Data: []byte("v")}, store.SetOptions{})
	ctx := discardCtx("MEMORY", bytesArgs("USAGE", "k1"), s)
	err := cmdMEMORY(ctx)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCmdMEMORY_Stats(t *testing.T) {
	s := store.NewStore()
	ctx := discardCtx("MEMORY", bytesArgs("STATS"), s)
	err := cmdMEMORY(ctx)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCmdMEMORY_Doctor(t *testing.T) {
	s := store.NewStore()
	ctx := discardCtx("MEMORY", bytesArgs("DOCTOR"), s)
	err := cmdMEMORY(ctx)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCmdMEMORY_MallocStats(t *testing.T) {
	s := store.NewStore()
	ctx := discardCtx("MEMORY", bytesArgs("MALLOC-STATS"), s)
	err := cmdMEMORY(ctx)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCmdMEMORY_Purge(t *testing.T) {
	s := store.NewStore()
	ctx := discardCtx("MEMORY", bytesArgs("PURGE"), s)
	err := cmdMEMORY(ctx)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}
