package command

import (
	"testing"

	"github.com/cachestorm/cachestorm/internal/store"
)

// ======================================================================
// INTEGRATION COMMANDS (integration_commands.go)
// ======================================================================

func TestDeep2_CircuitBreakerCreate(t *testing.T) {
	s := store.NewStore()
	ctx := discardCtx("CIRCUITBREAKER.CREATE", bytesArgs("cb1", "5", "30000"), s)
	if err := cmdCIRCUITBREAKERCREATE(ctx); err != nil {
		t.Fatal(err)
	}
	// too few args
	ctx2 := discardCtx("CIRCUITBREAKER.CREATE", bytesArgs("cb1"), s)
	if err := cmdCIRCUITBREAKERCREATE(ctx2); err != nil {
		t.Fatal(err)
	}
}

func TestDeep2_CircuitBreakerState(t *testing.T) {
	s := store.NewStore()
	// Create first
	ctx := discardCtx("CIRCUITBREAKER.CREATE", bytesArgs("cbstate1", "3", "1"), s)
	cmdCIRCUITBREAKERCREATE(ctx)
	// State check
	ctx2 := discardCtx("CIRCUITBREAKER.STATE", bytesArgs("cbstate1"), s)
	if err := cmdCIRCUITBREAKERSTATE(ctx2); err != nil {
		t.Fatal(err)
	}
	// Not found
	ctx3 := discardCtx("CIRCUITBREAKER.STATE", bytesArgs("nonexistent"), s)
	if err := cmdCIRCUITBREAKERSTATE(ctx3); err != nil {
		t.Fatal(err)
	}
	// too few args
	ctx4 := discardCtx("CIRCUITBREAKER.STATE", bytesArgs(), s)
	if err := cmdCIRCUITBREAKERSTATE(ctx4); err != nil {
		t.Fatal(err)
	}
}

func TestDeep2_CircuitBreakerTrip(t *testing.T) {
	s := store.NewStore()
	ctx := discardCtx("CIRCUITBREAKER.CREATE", bytesArgs("cbtrip1", "2", "30000"), s)
	cmdCIRCUITBREAKERCREATE(ctx)
	// Trip once
	ctx2 := discardCtx("CIRCUITBREAKER.TRIP", bytesArgs("cbtrip1"), s)
	if err := cmdCIRCUITBREAKERTRIP(ctx2); err != nil {
		t.Fatal(err)
	}
	// Trip again to exceed threshold
	ctx3 := discardCtx("CIRCUITBREAKER.TRIP", bytesArgs("cbtrip1"), s)
	if err := cmdCIRCUITBREAKERTRIP(ctx3); err != nil {
		t.Fatal(err)
	}
	// Not found
	ctx4 := discardCtx("CIRCUITBREAKER.TRIP", bytesArgs("nonexistent"), s)
	if err := cmdCIRCUITBREAKERTRIP(ctx4); err != nil {
		t.Fatal(err)
	}
	// too few args
	ctx5 := discardCtx("CIRCUITBREAKER.TRIP", bytesArgs(), s)
	if err := cmdCIRCUITBREAKERTRIP(ctx5); err != nil {
		t.Fatal(err)
	}
}

func TestDeep2_CircuitBreakerReset(t *testing.T) {
	s := store.NewStore()
	ctx := discardCtx("CIRCUITBREAKER.CREATE", bytesArgs("cbreset1", "3", "30000"), s)
	cmdCIRCUITBREAKERCREATE(ctx)
	ctx2 := discardCtx("CIRCUITBREAKER.RESET", bytesArgs("cbreset1"), s)
	if err := cmdCIRCUITBREAKERRESET(ctx2); err != nil {
		t.Fatal(err)
	}
	// Not found
	ctx3 := discardCtx("CIRCUITBREAKER.RESET", bytesArgs("nonexistent"), s)
	if err := cmdCIRCUITBREAKERRESET(ctx3); err != nil {
		t.Fatal(err)
	}
	// too few args
	ctx4 := discardCtx("CIRCUITBREAKER.RESET", bytesArgs(), s)
	if err := cmdCIRCUITBREAKERRESET(ctx4); err != nil {
		t.Fatal(err)
	}
}

func TestDeep2_RateLimitCreate(t *testing.T) {
	s := store.NewStore()
	ctx := discardCtx("RATELIMIT.CREATE", bytesArgs("rl1", "10", "60000"), s)
	if err := cmdRATELIMITCREATE(ctx); err != nil {
		t.Fatal(err)
	}
	// too few args
	ctx2 := discardCtx("RATELIMIT.CREATE", bytesArgs("rl1"), s)
	if err := cmdRATELIMITCREATE(ctx2); err != nil {
		t.Fatal(err)
	}
}

func TestDeep2_RateLimitCheck(t *testing.T) {
	s := store.NewStore()
	ctx := discardCtx("RATELIMIT.CREATE", bytesArgs("rlcheck1", "10", "60000"), s)
	cmdRATELIMITCREATE(ctx)
	ctx2 := discardCtx("RATELIMIT.CHECK", bytesArgs("rlcheck1", "client1"), s)
	if err := cmdRATELIMITCHECK(ctx2); err != nil {
		t.Fatal(err)
	}
	// Not found
	ctx3 := discardCtx("RATELIMIT.CHECK", bytesArgs("nonexistent", "client1"), s)
	if err := cmdRATELIMITCHECK(ctx3); err != nil {
		t.Fatal(err)
	}
	// too few args
	ctx4 := discardCtx("RATELIMIT.CHECK", bytesArgs("rl1"), s)
	if err := cmdRATELIMITCHECK(ctx4); err != nil {
		t.Fatal(err)
	}
}

func TestDeep2_RateLimitReset(t *testing.T) {
	s := store.NewStore()
	ctx := discardCtx("RATELIMIT.CREATE", bytesArgs("rlreset1", "10", "60000"), s)
	cmdRATELIMITCREATE(ctx)
	ctx2 := discardCtx("RATELIMIT.RESET", bytesArgs("rlreset1"), s)
	if err := cmdRATELIMITRESET(ctx2); err != nil {
		t.Fatal(err)
	}
	// Not found
	ctx3 := discardCtx("RATELIMIT.RESET", bytesArgs("nonexistent"), s)
	if err := cmdRATELIMITRESET(ctx3); err != nil {
		t.Fatal(err)
	}
}

func TestDeep2_RateLimitDelete(t *testing.T) {
	s := store.NewStore()
	ctx := discardCtx("RATELIMIT.CREATE", bytesArgs("rldel1", "10", "60000"), s)
	cmdRATELIMITCREATE(ctx)
	ctx2 := discardCtx("RATELIMIT.DELETE", bytesArgs("rldel1"), s)
	if err := cmdRATELIMITDELETE(ctx2); err != nil {
		t.Fatal(err)
	}
	// Not found
	ctx3 := discardCtx("RATELIMIT.DELETE", bytesArgs("nonexistent"), s)
	if err := cmdRATELIMITDELETE(ctx3); err != nil {
		t.Fatal(err)
	}
}

func TestDeep2_CacheLock(t *testing.T) {
	s := store.NewStore()
	ctx := discardCtx("CACHE.LOCK", bytesArgs("lockkey1", "holder1", "30000"), s)
	if err := cmdCACHELOCK(ctx); err != nil {
		t.Fatal(err)
	}
	// Lock same key with different holder - should fail
	ctx2 := discardCtx("CACHE.LOCK", bytesArgs("lockkey1", "holder2", "30000"), s)
	if err := cmdCACHELOCK(ctx2); err != nil {
		t.Fatal(err)
	}
	// Lock same key with same holder - should succeed (renew)
	ctx3 := discardCtx("CACHE.LOCK", bytesArgs("lockkey1", "holder1", "30000"), s)
	if err := cmdCACHELOCK(ctx3); err != nil {
		t.Fatal(err)
	}
	// too few args
	ctx4 := discardCtx("CACHE.LOCK", bytesArgs("k"), s)
	if err := cmdCACHELOCK(ctx4); err != nil {
		t.Fatal(err)
	}
}

func TestDeep2_CacheUnlock(t *testing.T) {
	s := store.NewStore()
	ctx := discardCtx("CACHE.LOCK", bytesArgs("unlockkey1", "holder1", "30000"), s)
	cmdCACHELOCK(ctx)
	// Unlock with correct holder
	ctx2 := discardCtx("CACHE.UNLOCK", bytesArgs("unlockkey1", "holder1"), s)
	if err := cmdCACHEUNLOCK(ctx2); err != nil {
		t.Fatal(err)
	}
	// Unlock with wrong holder
	ctx3 := discardCtx("CACHE.UNLOCK", bytesArgs("unlockkey1", "holder2"), s)
	if err := cmdCACHEUNLOCK(ctx3); err != nil {
		t.Fatal(err)
	}
	// too few args
	ctx4 := discardCtx("CACHE.UNLOCK", bytesArgs("k"), s)
	if err := cmdCACHEUNLOCK(ctx4); err != nil {
		t.Fatal(err)
	}
}

func TestDeep2_CacheLocked(t *testing.T) {
	s := store.NewStore()
	ctx := discardCtx("CACHE.LOCK", bytesArgs("lockedkey1", "holder1", "30000"), s)
	cmdCACHELOCK(ctx)
	ctx2 := discardCtx("CACHE.LOCKED", bytesArgs("lockedkey1"), s)
	if err := cmdCACHELOCKED(ctx2); err != nil {
		t.Fatal(err)
	}
	// Not locked
	ctx3 := discardCtx("CACHE.LOCKED", bytesArgs("nolock"), s)
	if err := cmdCACHELOCKED(ctx3); err != nil {
		t.Fatal(err)
	}
}

func TestDeep2_CacheRefresh(t *testing.T) {
	s := store.NewStore()
	ctx := discardCtx("CACHE.LOCK", bytesArgs("refreshkey1", "holder1", "30000"), s)
	cmdCACHELOCK(ctx)
	ctx2 := discardCtx("CACHE.REFRESH", bytesArgs("refreshkey1", "holder1", "60000"), s)
	if err := cmdCACHEREFRESH(ctx2); err != nil {
		t.Fatal(err)
	}
	// Wrong holder
	ctx3 := discardCtx("CACHE.REFRESH", bytesArgs("refreshkey1", "holder2", "60000"), s)
	if err := cmdCACHEREFRESH(ctx3); err != nil {
		t.Fatal(err)
	}
	// too few args
	ctx4 := discardCtx("CACHE.REFRESH", bytesArgs("k"), s)
	if err := cmdCACHEREFRESH(ctx4); err != nil {
		t.Fatal(err)
	}
}

func TestDeep2_NetWhois(t *testing.T) {
	s := store.NewStore()
	ctx := discardCtx("NET.WHOIS", bytesArgs("example.com"), s)
	if err := cmdNETWHOIS(ctx); err != nil {
		t.Fatal(err)
	}
	ctx2 := discardCtx("NET.WHOIS", bytesArgs(), s)
	if err := cmdNETWHOIS(ctx2); err != nil {
		t.Fatal(err)
	}
}

func TestDeep2_NetDNS(t *testing.T) {
	s := store.NewStore()
	ctx := discardCtx("NET.DNS", bytesArgs("example.com"), s)
	if err := cmdNETDNS(ctx); err != nil {
		t.Fatal(err)
	}
}

func TestDeep2_NetPing(t *testing.T) {
	s := store.NewStore()
	ctx := discardCtx("NET.PING", bytesArgs("127.0.0.1"), s)
	if err := cmdNETPING(ctx); err != nil {
		t.Fatal(err)
	}
}

func TestDeep2_NetPort(t *testing.T) {
	s := store.NewStore()
	ctx := discardCtx("NET.PORT", bytesArgs("127.0.0.1", "80"), s)
	if err := cmdNETPORT(ctx); err != nil {
		t.Fatal(err)
	}
}

func TestDeep2_ArrayPush(t *testing.T) {
	s := store.NewStore()
	ctx := discardCtx("ARRAY.PUSH", bytesArgs("arr1", "a", "b", "c"), s)
	if err := cmdARRAYPUSH(ctx); err != nil {
		t.Fatal(err)
	}
	ctx2 := discardCtx("ARRAY.PUSH", bytesArgs("arr1"), s)
	if err := cmdARRAYPUSH(ctx2); err != nil {
		t.Fatal(err)
	}
}

func TestDeep2_ArrayPop(t *testing.T) {
	s := store.NewStore()
	ctx := discardCtx("ARRAY.PUSH", bytesArgs("arrpop1", "a", "b"), s)
	cmdARRAYPUSH(ctx)
	ctx2 := discardCtx("ARRAY.POP", bytesArgs("arrpop1"), s)
	if err := cmdARRAYPOP(ctx2); err != nil {
		t.Fatal(err)
	}
	// Empty array
	ctx3 := discardCtx("ARRAY.POP", bytesArgs("nonexistent"), s)
	if err := cmdARRAYPOP(ctx3); err != nil {
		t.Fatal(err)
	}
}

func TestDeep2_ArrayShift(t *testing.T) {
	s := store.NewStore()
	ctx := discardCtx("ARRAY.PUSH", bytesArgs("arrshift1", "a", "b"), s)
	cmdARRAYPUSH(ctx)
	ctx2 := discardCtx("ARRAY.SHIFT", bytesArgs("arrshift1"), s)
	if err := cmdARRAYSHIFT(ctx2); err != nil {
		t.Fatal(err)
	}
	ctx3 := discardCtx("ARRAY.SHIFT", bytesArgs("nonexistent"), s)
	if err := cmdARRAYSHIFT(ctx3); err != nil {
		t.Fatal(err)
	}
}

func TestDeep2_ArrayUnshift(t *testing.T) {
	s := store.NewStore()
	ctx := discardCtx("ARRAY.PUSH", bytesArgs("arrunshift1", "b", "c"), s)
	cmdARRAYPUSH(ctx)
	ctx2 := discardCtx("ARRAY.UNSHIFT", bytesArgs("arrunshift1", "a"), s)
	if err := cmdARRAYUNSHIFT(ctx2); err != nil {
		t.Fatal(err)
	}
}

func TestDeep2_ArraySlice(t *testing.T) {
	s := store.NewStore()
	ctx := discardCtx("ARRAY.PUSH", bytesArgs("arrslice1", "a", "b", "c", "d"), s)
	cmdARRAYPUSH(ctx)
	ctx2 := discardCtx("ARRAY.SLICE", bytesArgs("arrslice1", "1", "3"), s)
	if err := cmdARRAYSLICE(ctx2); err != nil {
		t.Fatal(err)
	}
	// nonexistent
	ctx3 := discardCtx("ARRAY.SLICE", bytesArgs("nonexistent", "0", "1"), s)
	if err := cmdARRAYSLICE(ctx3); err != nil {
		t.Fatal(err)
	}
}

func TestDeep2_ArraySplice(t *testing.T) {
	s := store.NewStore()
	ctx := discardCtx("ARRAY.PUSH", bytesArgs("arrsplice1", "a", "b", "c", "d"), s)
	cmdARRAYPUSH(ctx)
	ctx2 := discardCtx("ARRAY.SPLICE", bytesArgs("arrsplice1", "1", "2", "x", "y"), s)
	if err := cmdARRAYSPLICE(ctx2); err != nil {
		t.Fatal(err)
	}
	// nonexistent
	ctx3 := discardCtx("ARRAY.SPLICE", bytesArgs("nonexistent2", "0", "0"), s)
	if err := cmdARRAYSPLICE(ctx3); err != nil {
		t.Fatal(err)
	}
}

func TestDeep2_ArrayReverse(t *testing.T) {
	s := store.NewStore()
	ctx := discardCtx("ARRAY.PUSH", bytesArgs("arrrev1", "a", "b", "c"), s)
	cmdARRAYPUSH(ctx)
	ctx2 := discardCtx("ARRAY.REVERSE", bytesArgs("arrrev1"), s)
	if err := cmdARRAYREVERSE(ctx2); err != nil {
		t.Fatal(err)
	}
	// nonexistent
	ctx3 := discardCtx("ARRAY.REVERSE", bytesArgs("nonexistent"), s)
	if err := cmdARRAYREVERSE(ctx3); err != nil {
		t.Fatal(err)
	}
}

func TestDeep2_ArraySort(t *testing.T) {
	s := store.NewStore()
	ctx := discardCtx("ARRAY.PUSH", bytesArgs("arrsort1", "c", "a", "b"), s)
	cmdARRAYPUSH(ctx)
	ctx2 := discardCtx("ARRAY.SORT", bytesArgs("arrsort1"), s)
	if err := cmdARRAYSORT(ctx2); err != nil {
		t.Fatal(err)
	}
	// DESC
	ctx3 := discardCtx("ARRAY.SORT", bytesArgs("arrsort1", "DESC"), s)
	if err := cmdARRAYSORT(ctx3); err != nil {
		t.Fatal(err)
	}
	// nonexistent
	ctx4 := discardCtx("ARRAY.SORT", bytesArgs("nonexistent"), s)
	if err := cmdARRAYSORT(ctx4); err != nil {
		t.Fatal(err)
	}
}

func TestDeep2_ArrayUnique(t *testing.T) {
	s := store.NewStore()
	ctx := discardCtx("ARRAY.PUSH", bytesArgs("arruniq1", "a", "b", "a", "c"), s)
	cmdARRAYPUSH(ctx)
	ctx2 := discardCtx("ARRAY.UNIQUE", bytesArgs("arruniq1"), s)
	if err := cmdARRAYUNIQUE(ctx2); err != nil {
		t.Fatal(err)
	}
	// nonexistent
	ctx3 := discardCtx("ARRAY.UNIQUE", bytesArgs("nonexistent"), s)
	if err := cmdARRAYUNIQUE(ctx3); err != nil {
		t.Fatal(err)
	}
}

func TestDeep2_ArrayFlatten(t *testing.T) {
	s := store.NewStore()
	ctx := discardCtx("ARRAY.PUSH", bytesArgs("arrflat1", "a", "b"), s)
	cmdARRAYPUSH(ctx)
	ctx2 := discardCtx("ARRAY.FLATTEN", bytesArgs("arrflat1"), s)
	if err := cmdARRAYFLATTEN(ctx2); err != nil {
		t.Fatal(err)
	}
	ctx3 := discardCtx("ARRAY.FLATTEN", bytesArgs("nonexistent"), s)
	if err := cmdARRAYFLATTEN(ctx3); err != nil {
		t.Fatal(err)
	}
}

func TestDeep2_ArrayMerge(t *testing.T) {
	s := store.NewStore()
	ctx := discardCtx("ARRAY.PUSH", bytesArgs("arrm1", "a"), s)
	cmdARRAYPUSH(ctx)
	ctx2 := discardCtx("ARRAY.PUSH", bytesArgs("arrm2", "b"), s)
	cmdARRAYPUSH(ctx2)
	ctx3 := discardCtx("ARRAY.MERGE", bytesArgs("arrm1", "arrm2", "dummy"), s)
	if err := cmdARRAYMERGE(ctx3); err != nil {
		t.Fatal(err)
	}
	// src nonexistent
	ctx4 := discardCtx("ARRAY.MERGE", bytesArgs("arrm1", "nonexistent", "dummy"), s)
	if err := cmdARRAYMERGE(ctx4); err != nil {
		t.Fatal(err)
	}
}

func TestDeep2_ArrayIntersect(t *testing.T) {
	s := store.NewStore()
	ctx := discardCtx("ARRAY.PUSH", bytesArgs("arrint1", "a", "b", "c"), s)
	cmdARRAYPUSH(ctx)
	ctx2 := discardCtx("ARRAY.PUSH", bytesArgs("arrint2", "b", "c", "d"), s)
	cmdARRAYPUSH(ctx2)
	ctx3 := discardCtx("ARRAY.INTERSECT", bytesArgs("arrint1", "arrint2"), s)
	if err := cmdARRAYINTERSECT(ctx3); err != nil {
		t.Fatal(err)
	}
}

func TestDeep2_ArrayDiff(t *testing.T) {
	s := store.NewStore()
	ctx := discardCtx("ARRAY.PUSH", bytesArgs("arrdiff1", "a", "b", "c"), s)
	cmdARRAYPUSH(ctx)
	ctx2 := discardCtx("ARRAY.PUSH", bytesArgs("arrdiff2", "b"), s)
	cmdARRAYPUSH(ctx2)
	ctx3 := discardCtx("ARRAY.DIFF", bytesArgs("arrdiff1", "arrdiff2"), s)
	if err := cmdARRAYDIFF(ctx3); err != nil {
		t.Fatal(err)
	}
}

func TestDeep2_ArrayIndexOf(t *testing.T) {
	s := store.NewStore()
	ctx := discardCtx("ARRAY.PUSH", bytesArgs("arridx1", "a", "b", "c"), s)
	cmdARRAYPUSH(ctx)
	ctx2 := discardCtx("ARRAY.INDEXOF", bytesArgs("arridx1", "b"), s)
	if err := cmdARRAYINDEXOF(ctx2); err != nil {
		t.Fatal(err)
	}
	// not found
	ctx3 := discardCtx("ARRAY.INDEXOF", bytesArgs("arridx1", "z"), s)
	if err := cmdARRAYINDEXOF(ctx3); err != nil {
		t.Fatal(err)
	}
	// nonexistent array
	ctx4 := discardCtx("ARRAY.INDEXOF", bytesArgs("nonexistent", "a"), s)
	if err := cmdARRAYINDEXOF(ctx4); err != nil {
		t.Fatal(err)
	}
}

func TestDeep2_ArrayLastIndexOf(t *testing.T) {
	s := store.NewStore()
	ctx := discardCtx("ARRAY.PUSH", bytesArgs("arrlast1", "a", "b", "a"), s)
	cmdARRAYPUSH(ctx)
	ctx2 := discardCtx("ARRAY.LASTINDEXOF", bytesArgs("arrlast1", "a"), s)
	if err := cmdARRAYLASTINDEXOF(ctx2); err != nil {
		t.Fatal(err)
	}
	ctx3 := discardCtx("ARRAY.LASTINDEXOF", bytesArgs("nonexistent", "a"), s)
	if err := cmdARRAYLASTINDEXOF(ctx3); err != nil {
		t.Fatal(err)
	}
}

func TestDeep2_ArrayIncludes(t *testing.T) {
	s := store.NewStore()
	ctx := discardCtx("ARRAY.PUSH", bytesArgs("arrinc1", "a", "b"), s)
	cmdARRAYPUSH(ctx)
	ctx2 := discardCtx("ARRAY.INCLUDES", bytesArgs("arrinc1", "a"), s)
	if err := cmdARRAYINCLUDES(ctx2); err != nil {
		t.Fatal(err)
	}
	ctx3 := discardCtx("ARRAY.INCLUDES", bytesArgs("arrinc1", "z"), s)
	if err := cmdARRAYINCLUDES(ctx3); err != nil {
		t.Fatal(err)
	}
	ctx4 := discardCtx("ARRAY.INCLUDES", bytesArgs("nonexistent", "a"), s)
	if err := cmdARRAYINCLUDES(ctx4); err != nil {
		t.Fatal(err)
	}
}

func TestDeep2_ObjectCommands(t *testing.T) {
	s := store.NewStore()

	// OBJECT.SET
	ctx := discardCtx("OBJECT.SET", bytesArgs("obj1", "key1", "val1"), s)
	if err := cmdOBJECTSET(ctx); err != nil {
		t.Fatal(err)
	}
	ctx = discardCtx("OBJECT.SET", bytesArgs("obj1", "key2", "val2"), s)
	cmdOBJECTSET(ctx)

	// OBJECT.KEYS
	ctx = discardCtx("OBJECT.KEYS", bytesArgs("obj1"), s)
	if err := cmdOBJECTKEYS(ctx); err != nil {
		t.Fatal(err)
	}
	ctx = discardCtx("OBJECT.KEYS", bytesArgs("nonexistent"), s)
	cmdOBJECTKEYS(ctx)

	// OBJECT.VALUES
	ctx = discardCtx("OBJECT.VALUES", bytesArgs("obj1"), s)
	if err := cmdOBJECTVALUES(ctx); err != nil {
		t.Fatal(err)
	}
	ctx = discardCtx("OBJECT.VALUES", bytesArgs("nonexistent"), s)
	cmdOBJECTVALUES(ctx)

	// OBJECT.ENTRIES
	ctx = discardCtx("OBJECT.ENTRIES", bytesArgs("obj1"), s)
	if err := cmdOBJECTENTRIES(ctx); err != nil {
		t.Fatal(err)
	}
	ctx = discardCtx("OBJECT.ENTRIES", bytesArgs("nonexistent"), s)
	cmdOBJECTENTRIES(ctx)

	// OBJECT.FROMENTRIES
	ctx = discardCtx("OBJECT.FROMENTRIES", bytesArgs("obj2", "a", "1", "b", "2"), s)
	if err := cmdOBJECTFROMENTRIES(ctx); err != nil {
		t.Fatal(err)
	}

	// OBJECT.MERGE
	ctx = discardCtx("OBJECT.MERGE", bytesArgs("obj1", "obj2", "dummy"), s)
	if err := cmdOBJECTMERGE(ctx); err != nil {
		t.Fatal(err)
	}
	// merge with nonexistent src
	ctx = discardCtx("OBJECT.MERGE", bytesArgs("obj1", "nonexistent", "dummy"), s)
	cmdOBJECTMERGE(ctx)

	// OBJECT.PICK
	ctx = discardCtx("OBJECT.PICK", bytesArgs("obj1", "key1"), s)
	if err := cmdOBJECTPICK(ctx); err != nil {
		t.Fatal(err)
	}
	ctx = discardCtx("OBJECT.PICK", bytesArgs("nonexistent", "key1"), s)
	cmdOBJECTPICK(ctx)

	// OBJECT.HAS
	ctx = discardCtx("OBJECT.HAS", bytesArgs("obj1", "key1"), s)
	if err := cmdOBJECTHAS(ctx); err != nil {
		t.Fatal(err)
	}
	ctx = discardCtx("OBJECT.HAS", bytesArgs("nonexistent", "key1"), s)
	cmdOBJECTHAS(ctx)

	// OBJECT.GET
	ctx = discardCtx("OBJECT.GET", bytesArgs("obj1", "key1"), s)
	if err := cmdOBJECTGET(ctx); err != nil {
		t.Fatal(err)
	}
	ctx = discardCtx("OBJECT.GET", bytesArgs("nonexistent", "key1"), s)
	cmdOBJECTGET(ctx)

	// OBJECT.DELETE
	ctx = discardCtx("OBJECT.SET", bytesArgs("objdel1", "k1", "v1"), s)
	cmdOBJECTSET(ctx)
	ctx = discardCtx("OBJECT.DELETE", bytesArgs("objdel1", "k1"), s)
	if err := cmdOBJECTDELETE(ctx); err != nil {
		t.Fatal(err)
	}
	ctx = discardCtx("OBJECT.DELETE", bytesArgs("nonexistent", "k1"), s)
	cmdOBJECTDELETE(ctx)

	// OBJECT.OMIT
	ctx = discardCtx("OBJECT.SET", bytesArgs("objomit1", "k1", "v1"), s)
	cmdOBJECTSET(ctx)
	ctx = discardCtx("OBJECT.SET", bytesArgs("objomit1", "k2", "v2"), s)
	cmdOBJECTSET(ctx)
	ctx = discardCtx("OBJECT.OMIT", bytesArgs("objomit1", "k1"), s)
	if err := cmdOBJECTOMIT(ctx); err != nil {
		t.Fatal(err)
	}
	ctx = discardCtx("OBJECT.OMIT", bytesArgs("nonexistent", "k1"), s)
	cmdOBJECTOMIT(ctx)
}

func TestDeep2_MathCommands(t *testing.T) {
	s := store.NewStore()

	tests := []struct {
		name string
		fn   func(*Context) error
		args []string
	}{
		{"ADD", cmdMATHADD, []string{"10", "20"}},
		{"SUB", cmdMATHSUB, []string{"30", "10"}},
		{"MUL", cmdMATHMUL, []string{"5", "6"}},
		{"DIV", cmdMATHDIV, []string{"30", "5"}},
		{"MOD", cmdMATHMOD, []string{"10", "3"}},
		{"POW", cmdMATHPOW, []string{"2", "10"}},
		{"SQRT", cmdMATHSQRT, []string{"16"}},
		{"ABS", cmdMATHABS, []string{"-5"}},
		{"MIN", cmdMATHMIN, []string{"3", "1", "5"}},
		{"MAX", cmdMATHMAX, []string{"3", "1", "5"}},
		{"FLOOR", cmdMATHFLOOR, []string{"3"}},
		{"CEIL", cmdMATHCEIL, []string{"3"}},
		{"ROUND", cmdMATHROUND, []string{"3"}},
		{"RANDOM", cmdMATHRANDOM, []string{"1", "100"}},
		{"SUM", cmdMATHSUM, []string{"1", "2", "3"}},
		{"AVG", cmdMATHAVG, []string{"10", "20", "30"}},
		{"MEDIAN", cmdMATHMEDIAN, []string{"1", "2", "3"}},
		{"MEDIAN_EVEN", cmdMATHMEDIAN, []string{"1", "2", "3", "4"}},
		{"STDDEV", cmdMATHSTDDEV, []string{"2", "4", "4", "4", "5", "5", "7", "9"}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := discardCtx("MATH."+tt.name, bytesArgs(tt.args...), s)
			if err := tt.fn(ctx); err != nil {
				t.Fatal(err)
			}
		})
	}

	// DIV by zero
	ctx := discardCtx("MATH.DIV", bytesArgs("10", "0"), s)
	if err := cmdMATHDIV(ctx); err != nil {
		t.Fatal(err)
	}

	// MOD by zero
	ctx = discardCtx("MATH.MOD", bytesArgs("10", "0"), s)
	if err := cmdMATHMOD(ctx); err != nil {
		t.Fatal(err)
	}

	// RANDOM with no args
	ctx = discardCtx("MATH.RANDOM", bytesArgs(), s)
	if err := cmdMATHRANDOM(ctx); err != nil {
		t.Fatal(err)
	}
}

func TestDeep2_GeoEncodeAndDecode(t *testing.T) {
	s := store.NewStore()
	ctx := discardCtx("GEO.ENCODE", bytesArgs("37", "-122"), s)
	if err := cmdGEOENCODE(ctx); err != nil {
		t.Fatal(err)
	}
	ctx2 := discardCtx("GEO.DECODE", bytesArgs("9q8yyk8yuv27"), s)
	if err := cmdGEODECODE(ctx2); err != nil {
		t.Fatal(err)
	}
}

func TestDeep2_GeoDistance(t *testing.T) {
	s := store.NewStore()
	ctx := discardCtx("GEO.DISTANCE", bytesArgs("37", "-122", "40", "-74"), s)
	if err := cmdGEODISTANCE(ctx); err != nil {
		t.Fatal(err)
	}
}

func TestDeep2_GeoBoundingBox(t *testing.T) {
	s := store.NewStore()
	ctx := discardCtx("GEO.BOUNDINGBOX", bytesArgs("37", "-122", "100"), s)
	if err := cmdGEOBOUNDINGBOX(ctx); err != nil {
		t.Fatal(err)
	}
}

func TestDeep2_CaptchaGenerateAndVerify(t *testing.T) {
	s := store.NewStore()
	ctx := discardCtx("CAPTCHA.GENERATE", bytesArgs("cap1"), s)
	if err := cmdCAPTCHAGENERATE(ctx); err != nil {
		t.Fatal(err)
	}
	ctx2 := discardCtx("CAPTCHA.VERIFY", bytesArgs("cap1", "anything"), s)
	if err := cmdCAPTCHAVERIFY(ctx2); err != nil {
		t.Fatal(err)
	}
}

func TestDeep2_SequenceCommands(t *testing.T) {
	s := store.NewStore()
	// NEXT
	ctx := discardCtx("SEQUENCE.NEXT", bytesArgs("seq1"), s)
	if err := cmdSEQUENCENEXT(ctx); err != nil {
		t.Fatal(err)
	}
	// CURRENT
	ctx = discardCtx("SEQUENCE.CURRENT", bytesArgs("seq1"), s)
	if err := cmdSEQUENCECURRENT(ctx); err != nil {
		t.Fatal(err)
	}
	// RESET
	ctx = discardCtx("SEQUENCE.RESET", bytesArgs("seq1"), s)
	if err := cmdSEQUENCERESET(ctx); err != nil {
		t.Fatal(err)
	}
	// SET
	ctx = discardCtx("SEQUENCE.SET", bytesArgs("seq1", "100"), s)
	if err := cmdSEQUENCESET(ctx); err != nil {
		t.Fatal(err)
	}
}

// ======================================================================
// ENCODING COMMANDS (encoding_commands.go)
// ======================================================================

func TestDeep2_MsgpackEncodeDecode(t *testing.T) {
	s := store.NewStore()
	ctx := discardCtx("MSGPACK.ENCODE", bytesArgs("hello world"), s)
	if err := cmdMSGPACKENCODE(ctx); err != nil {
		t.Fatal(err)
	}
	// Encode then decode
	encoded := msgpackEncode([]byte("hello world"))
	ctx2 := discardCtx("MSGPACK.DECODE", [][]byte{encoded}, s)
	if err := cmdMSGPACKDECODE(ctx2); err != nil {
		t.Fatal(err)
	}
	// Bad data
	ctx3 := discardCtx("MSGPACK.DECODE", [][]byte{[]byte{0xFF, 0x01}}, s)
	if err := cmdMSGPACKDECODE(ctx3); err != nil {
		t.Fatal(err)
	}
}

func TestDeep2_BsonEncodeDecode(t *testing.T) {
	s := store.NewStore()
	ctx := discardCtx("BSON.ENCODE", bytesArgs("hello"), s)
	if err := cmdBSONENCODE(ctx); err != nil {
		t.Fatal(err)
	}
	encoded := bsonEncode([]byte("hello"))
	ctx2 := discardCtx("BSON.DECODE", [][]byte{encoded}, s)
	if err := cmdBSONDECODE(ctx2); err != nil {
		t.Fatal(err)
	}
	// Bad data
	ctx3 := discardCtx("BSON.DECODE", [][]byte{[]byte{0x01, 0x02}}, s)
	if err := cmdBSONDECODE(ctx3); err != nil {
		t.Fatal(err)
	}
}

func TestDeep2_UrlEncodeDecode(t *testing.T) {
	s := store.NewStore()
	ctx := discardCtx("URL.ENCODE", bytesArgs("hello world&foo=bar"), s)
	if err := cmdURLENCODE(ctx); err != nil {
		t.Fatal(err)
	}
	ctx2 := discardCtx("URL.DECODE", bytesArgs("hello+world%26foo%3Dbar"), s)
	if err := cmdURLDECODE(ctx2); err != nil {
		t.Fatal(err)
	}
}

func TestDeep2_XmlEncodeDecode(t *testing.T) {
	s := store.NewStore()
	ctx := discardCtx("XML.ENCODE", bytesArgs("name", "test<value>"), s)
	if err := cmdXMLENCODE(ctx); err != nil {
		t.Fatal(err)
	}
	ctx2 := discardCtx("XML.DECODE", bytesArgs("<name>test&lt;value&gt;</name>"), s)
	if err := cmdXMLDECODE(ctx2); err != nil {
		t.Fatal(err)
	}
	// Bad XML
	ctx3 := discardCtx("XML.DECODE", bytesArgs("not xml"), s)
	if err := cmdXMLDECODE(ctx3); err != nil {
		t.Fatal(err)
	}
}

func TestDeep2_YamlEncodeDecode(t *testing.T) {
	s := store.NewStore()
	ctx := discardCtx("YAML.ENCODE", bytesArgs("key", "value"), s)
	if err := cmdYAMLENCODE(ctx); err != nil {
		t.Fatal(err)
	}
	ctx2 := discardCtx("YAML.DECODE", bytesArgs("key: value"), s)
	if err := cmdYAMLDECODE(ctx2); err != nil {
		t.Fatal(err)
	}
}

func TestDeep2_TomlEncodeDecode(t *testing.T) {
	s := store.NewStore()
	ctx := discardCtx("TOML.ENCODE", bytesArgs("key", "value"), s)
	if err := cmdTOMLENCODE(ctx); err != nil {
		t.Fatal(err)
	}
	ctx2 := discardCtx("TOML.DECODE", bytesArgs("key = \"value\""), s)
	if err := cmdTOMLDECODE(ctx2); err != nil {
		t.Fatal(err)
	}
}

func TestDeep2_CborEncodeDecode(t *testing.T) {
	s := store.NewStore()
	ctx := discardCtx("CBOR.ENCODE", bytesArgs("hello"), s)
	if err := cmdCBORENCODE(ctx); err != nil {
		t.Fatal(err)
	}
	encoded := cborEncode([]byte("hello"))
	ctx2 := discardCtx("CBOR.DECODE", [][]byte{encoded}, s)
	if err := cmdCBORDECODE(ctx2); err != nil {
		t.Fatal(err)
	}
	// Bad data
	ctx3 := discardCtx("CBOR.DECODE", [][]byte{[]byte{0xFF}}, s)
	if err := cmdCBORDECODE(ctx3); err != nil {
		t.Fatal(err)
	}
}

func TestDeep2_CsvEncodeDecode(t *testing.T) {
	s := store.NewStore()
	ctx := discardCtx("CSV.ENCODE", bytesArgs("a", "b,c", "d"), s)
	if err := cmdCSVENCODE(ctx); err != nil {
		t.Fatal(err)
	}
	ctx2 := discardCtx("CSV.DECODE", bytesArgs("a,\"b,c\",d"), s)
	if err := cmdCSVDECODE(ctx2); err != nil {
		t.Fatal(err)
	}
}

func TestDeep2_UuidCommands(t *testing.T) {
	s := store.NewStore()
	ctx := discardCtx("UUID.GEN", bytesArgs(), s)
	if err := cmdUUIDGEN(ctx); err != nil {
		t.Fatal(err)
	}
	ctx2 := discardCtx("UUID.VALIDATE", bytesArgs("550e8400-e29b-41d4-a716-446655440000"), s)
	if err := cmdUUIDVALIDATE(ctx2); err != nil {
		t.Fatal(err)
	}
	// Invalid uuid
	ctx3 := discardCtx("UUID.VALIDATE", bytesArgs("not-a-uuid"), s)
	if err := cmdUUIDVALIDATE(ctx3); err != nil {
		t.Fatal(err)
	}
	ctx4 := discardCtx("UUID.VERSION", bytesArgs("550e8400-e29b-41d4-a716-446655440000"), s)
	if err := cmdUUIDVERSION(ctx4); err != nil {
		t.Fatal(err)
	}
	// Invalid uuid for version
	ctx5 := discardCtx("UUID.VERSION", bytesArgs("not-a-uuid"), s)
	if err := cmdUUIDVERSION(ctx5); err != nil {
		t.Fatal(err)
	}
}

func TestDeep2_UlidCommands(t *testing.T) {
	s := store.NewStore()
	ctx := discardCtx("ULID.GEN", bytesArgs(), s)
	if err := cmdULIDGEN(ctx); err != nil {
		t.Fatal(err)
	}
	// Generate a ULID, then extract
	ulid := generateULID()
	ctx2 := discardCtx("ULID.EXTRACT", bytesArgs(ulid), s)
	if err := cmdULIDEXTRACT(ctx2); err != nil {
		t.Fatal(err)
	}
	// Invalid ULID
	ctx3 := discardCtx("ULID.EXTRACT", bytesArgs("short"), s)
	if err := cmdULIDEXTRACT(ctx3); err != nil {
		t.Fatal(err)
	}
}

func TestDeep2_TimestampCommands(t *testing.T) {
	s := store.NewStore()

	ctx := discardCtx("TIMESTAMP.NOW", bytesArgs(), s)
	if err := cmdTIMESTAMPNOW(ctx); err != nil {
		t.Fatal(err)
	}

	ctx = discardCtx("TIMESTAMP.PARSE", bytesArgs("1700000000"), s)
	if err := cmdTIMESTAMPPARSE(ctx); err != nil {
		t.Fatal(err)
	}

	ctx = discardCtx("TIMESTAMP.PARSE", bytesArgs("1700000000000"), s)
	if err := cmdTIMESTAMPPARSE(ctx); err != nil {
		t.Fatal(err)
	}

	ctx = discardCtx("TIMESTAMP.PARSE", bytesArgs("2023-11-14T22:13:20Z"), s)
	if err := cmdTIMESTAMPPARSE(ctx); err != nil {
		t.Fatal(err)
	}

	ctx = discardCtx("TIMESTAMP.FORMAT", bytesArgs("1700000000", "2006-01-02"), s)
	if err := cmdTIMESTAMPFORMAT(ctx); err != nil {
		t.Fatal(err)
	}

	units := []string{"seconds", "minutes", "hours", "days", "weeks", "months", "years"}
	for _, u := range units {
		ctx = discardCtx("TIMESTAMP.ADD", bytesArgs("1700000000", u, "1"), s)
		if err := cmdTIMESTAMPADD(ctx); err != nil {
			t.Fatalf("TIMESTAMP.ADD %s: %v", u, err)
		}
	}
	// unknown unit
	ctx = discardCtx("TIMESTAMP.ADD", bytesArgs("1700000000", "foobar", "1"), s)
	if err := cmdTIMESTAMPADD(ctx); err != nil {
		t.Fatal(err)
	}

	diffUnits := []string{"seconds", "minutes", "hours", "days", "milliseconds", "microseconds", "nanoseconds"}
	for _, u := range diffUnits {
		ctx = discardCtx("TIMESTAMP.DIFF", bytesArgs("1700000000", "1700003600", u), s)
		if err := cmdTIMESTAMPDIFF(ctx); err != nil {
			t.Fatalf("TIMESTAMP.DIFF %s: %v", u, err)
		}
	}

	startOfUnits := []string{"second", "minute", "hour", "day", "week", "month", "year"}
	for _, u := range startOfUnits {
		ctx = discardCtx("TIMESTAMP.STARTOF", bytesArgs("1700000000", u), s)
		if err := cmdTIMESTAMPSTARTOF(ctx); err != nil {
			t.Fatalf("TIMESTAMP.STARTOF %s: %v", u, err)
		}
	}

	endOfUnits := []string{"second", "minute", "hour", "day", "month", "year"}
	for _, u := range endOfUnits {
		ctx = discardCtx("TIMESTAMP.ENDOF", bytesArgs("1700000000", u), s)
		if err := cmdTIMESTAMPENDOF(ctx); err != nil {
			t.Fatalf("TIMESTAMP.ENDOF %s: %v", u, err)
		}
	}
}

func TestDeep2_DiffText(t *testing.T) {
	s := store.NewStore()
	ctx := discardCtx("DIFF.TEXT", bytesArgs("line1\nline2\nline3", "line1\nline4\nline3"), s)
	if err := cmdDIFFTEXT(ctx); err != nil {
		t.Fatal(err)
	}
}

func TestDeep2_DiffJSON(t *testing.T) {
	s := store.NewStore()
	ctx := discardCtx("DIFF.JSON", bytesArgs("{\"a\":\"1\",\"b\":\"2\"}", "{\"a\":\"1\",\"c\":\"3\"}"), s)
	if err := cmdDIFFJSON(ctx); err != nil {
		t.Fatal(err)
	}
}

func TestDeep2_PoolCommands(t *testing.T) {
	s := store.NewStore()
	ctx := discardCtx("POOL.CREATE", bytesArgs("pool1", "5"), s)
	if err := cmdPOOLCREATE(ctx); err != nil {
		t.Fatal(err)
	}
	ctx = discardCtx("POOL.PUT", bytesArgs("pool1", "item1"), s)
	if err := cmdPOOLPUT(ctx); err != nil {
		t.Fatal(err)
	}
	ctx = discardCtx("POOL.PUT", bytesArgs("nonexistent", "item1"), s)
	if err := cmdPOOLPUT(ctx); err != nil {
		t.Fatal(err)
	}
	ctx = discardCtx("POOL.GET", bytesArgs("pool1"), s)
	if err := cmdPOOLGET(ctx); err != nil {
		t.Fatal(err)
	}
	// empty pool
	ctx = discardCtx("POOL.GET", bytesArgs("pool1"), s)
	if err := cmdPOOLGET(ctx); err != nil {
		t.Fatal(err)
	}
	ctx = discardCtx("POOL.GET", bytesArgs("nonexistent"), s)
	if err := cmdPOOLGET(ctx); err != nil {
		t.Fatal(err)
	}
	ctx = discardCtx("POOL.STATS", bytesArgs("pool1"), s)
	if err := cmdPOOLSTATS(ctx); err != nil {
		t.Fatal(err)
	}
	ctx = discardCtx("POOL.STATS", bytesArgs("nonexistent"), s)
	if err := cmdPOOLSTATS(ctx); err != nil {
		t.Fatal(err)
	}
	ctx = discardCtx("POOL.CLEAR", bytesArgs("pool1"), s)
	if err := cmdPOOLCLEAR(ctx); err != nil {
		t.Fatal(err)
	}
	ctx = discardCtx("POOL.CLEAR", bytesArgs("nonexistent"), s)
	if err := cmdPOOLCLEAR(ctx); err != nil {
		t.Fatal(err)
	}
}

// ======================================================================
// SERVER COMMANDS (server_commands.go)
// ======================================================================

func TestDeep2_ServerCommandDocs(t *testing.T) {
	s := store.NewStore()
	router := NewRouter()
	RegisterServerCommands(router)
	// COMMAND DOCS GET
	ctx := discardCtx("COMMAND", bytesArgs("DOCS", "GET"), s)
	if err := cmdCOMMAND(ctx); err != nil {
		t.Fatal(err)
	}
	// COMMAND DOCS unknown
	ctx = discardCtx("COMMAND", bytesArgs("DOCS", "NONEXISTENTCMD"), s)
	if err := cmdCOMMAND(ctx); err != nil {
		t.Fatal(err)
	}
	// COMMAND GETKEYS GET mykey
	ctx = discardCtx("COMMAND", bytesArgs("GETKEYS", "GET", "mykey"), s)
	if err := cmdCOMMAND(ctx); err != nil {
		t.Fatal(err)
	}
	// COMMAND GETKEYS MGET
	ctx = discardCtx("COMMAND", bytesArgs("GETKEYS", "MGET", "k1", "k2"), s)
	if err := cmdCOMMAND(ctx); err != nil {
		t.Fatal(err)
	}
	// COMMAND GETKEYS MSET
	ctx = discardCtx("COMMAND", bytesArgs("GETKEYS", "MSET", "k1", "v1", "k2", "v2"), s)
	if err := cmdCOMMAND(ctx); err != nil {
		t.Fatal(err)
	}
	// COMMAND GETKEYS LPUSH
	ctx = discardCtx("COMMAND", bytesArgs("GETKEYS", "LPUSH", "mylist", "val"), s)
	if err := cmdCOMMAND(ctx); err != nil {
		t.Fatal(err)
	}
	// COMMAND GETKEYS BLPOP
	ctx = discardCtx("COMMAND", bytesArgs("GETKEYS", "BLPOP", "list1", "list2", "0"), s)
	if err := cmdCOMMAND(ctx); err != nil {
		t.Fatal(err)
	}
	// COMMAND GETKEYS RENAME
	ctx = discardCtx("COMMAND", bytesArgs("GETKEYS", "RENAME", "old", "new"), s)
	if err := cmdCOMMAND(ctx); err != nil {
		t.Fatal(err)
	}
	// COMMAND GETKEYS SINTER
	ctx = discardCtx("COMMAND", bytesArgs("GETKEYS", "SINTER", "s1", "s2"), s)
	if err := cmdCOMMAND(ctx); err != nil {
		t.Fatal(err)
	}
	// COMMAND LIST
	ctx = discardCtx("COMMAND", bytesArgs("LIST"), s)
	if err := cmdCOMMAND(ctx); err != nil {
		t.Fatal(err)
	}
	// COMMAND COUNT
	ctx = discardCtx("COMMAND", bytesArgs("COUNT"), s)
	if err := cmdCOMMAND(ctx); err != nil {
		t.Fatal(err)
	}
	// COMMAND unknown subcommand
	ctx = discardCtx("COMMAND", bytesArgs("FOOBAR"), s)
	if err := cmdCOMMAND(ctx); err != nil {
		t.Fatal(err)
	}
	// COMMAND no args
	ctx = discardCtx("COMMAND", bytesArgs(), s)
	if err := cmdCOMMAND(ctx); err != nil {
		t.Fatal(err)
	}
}

func TestDeep2_ServerSortCommands(t *testing.T) {
	s := store.NewStore()
	// Set up a list
	s.Set("sortlist", &store.ListValue{Elements: [][]byte{[]byte("3"), []byte("1"), []byte("2")}}, store.SetOptions{})

	ctx := discardCtx("SORT", bytesArgs("sortlist"), s)
	if err := cmdSORT(ctx); err != nil {
		t.Fatal(err)
	}
	ctx = discardCtx("SORT_RO", bytesArgs("sortlist"), s)
	if err := cmdSORTRO(ctx); err != nil {
		t.Fatal(err)
	}
}

func TestDeep2_ServerSlowlog(t *testing.T) {
	s := store.NewStore()
	// RESET first to ensure clean state
	ctx := discardCtx("SLOWLOG", bytesArgs("RESET"), s)
	if err := cmdSLOWLOG(ctx); err != nil {
		t.Fatal(err)
	}
	// Add an entry to the slow log so GET doesn't trip on empty
	globalSlowLog.Add("GET", []string{"key"}, 999999, "127.0.0.1", 1)
	ctx = discardCtx("SLOWLOG", bytesArgs("GET", "1"), s)
	if err := cmdSLOWLOG(ctx); err != nil {
		t.Fatal(err)
	}
	ctx = discardCtx("SLOWLOG", bytesArgs("LEN"), s)
	if err := cmdSLOWLOG(ctx); err != nil {
		t.Fatal(err)
	}
}

func TestDeep2_ServerWait(t *testing.T) {
	s := store.NewStore()
	ctx := discardCtx("WAIT", bytesArgs("0", "0"), s)
	if err := cmdWAIT(ctx); err != nil {
		t.Fatal(err)
	}
}

func TestDeep2_ServerRole(t *testing.T) {
	s := store.NewStore()
	ctx := discardCtx("ROLE", bytesArgs(), s)
	if err := cmdROLE(ctx); err != nil {
		t.Fatal(err)
	}
}

func TestDeep2_ServerLastsave(t *testing.T) {
	s := store.NewStore()
	ctx := discardCtx("LASTSAVE", bytesArgs(), s)
	if err := cmdLASTSAVE(ctx); err != nil {
		t.Fatal(err)
	}
}

func TestDeep2_ServerLolwut(t *testing.T) {
	s := store.NewStore()
	ctx := discardCtx("LOLWUT", bytesArgs(), s)
	if err := cmdLOLWUT(ctx); err != nil {
		t.Fatal(err)
	}
}

func TestDeep2_ServerSave(t *testing.T) {
	s := store.NewStore()
	ctx := discardCtx("SAVE", bytesArgs(), s)
	if err := cmdSAVE(ctx); err != nil {
		t.Fatal(err)
	}
}

func TestDeep2_ServerBgsave(t *testing.T) {
	s := store.NewStore()
	ctx := discardCtx("BGSAVE", bytesArgs(), s)
	if err := cmdBGSAVE(ctx); err != nil {
		t.Fatal(err)
	}
}

func TestDeep2_ServerBgrewriteaof(t *testing.T) {
	s := store.NewStore()
	ctx := discardCtx("BGREWRITEAOF", bytesArgs(), s)
	if err := cmdBGREWRITEAOF(ctx); err != nil {
		t.Fatal(err)
	}
}

func TestDeep2_ServerSlaveof(t *testing.T) {
	s := store.NewStore()
	ctx := discardCtx("SLAVEOF", bytesArgs("NO", "ONE"), s)
	if err := cmdSLAVEOF(ctx); err != nil {
		t.Fatal(err)
	}
}

func TestDeep2_ServerLatency(t *testing.T) {
	s := store.NewStore()
	ctx := discardCtx("LATENCY", bytesArgs("LATEST"), s)
	if err := cmdLATENCY(ctx); err != nil {
		t.Fatal(err)
	}
}

func TestDeep2_ServerStralgo(t *testing.T) {
	s := store.NewStore()
	ctx := discardCtx("STRALGO", bytesArgs("LCS", "STRINGS", "ohmytext", "mynewtext"), s)
	if err := cmdSTRALGO(ctx); err != nil {
		t.Fatal(err)
	}
}

func TestDeep2_ServerModule(t *testing.T) {
	s := store.NewStore()
	ctx := discardCtx("MODULE", bytesArgs("LIST"), s)
	if err := cmdMODULE(ctx); err != nil {
		t.Fatal(err)
	}
}

func TestDeep2_ServerACL(t *testing.T) {
	s := store.NewStore()
	ctx := discardCtx("ACL", bytesArgs("LIST"), s)
	if err := cmdACL(ctx); err != nil {
		t.Fatal(err)
	}
	ctx = discardCtx("ACL", bytesArgs("WHOAMI"), s)
	if err := cmdACL(ctx); err != nil {
		t.Fatal(err)
	}
	ctx = discardCtx("ACL", bytesArgs("USERS"), s)
	if err := cmdACL(ctx); err != nil {
		t.Fatal(err)
	}
	ctx = discardCtx("ACL", bytesArgs("CAT"), s)
	if err := cmdACL(ctx); err != nil {
		t.Fatal(err)
	}
}

func TestDeep2_ServerMonitor(t *testing.T) {
	s := store.NewStore()
	ctx := discardCtx("MONITOR", bytesArgs(), s)
	if err := cmdMONITOR(ctx); err != nil {
		t.Fatal(err)
	}
}

func TestDeep2_ServerSwapDB(t *testing.T) {
	s := store.NewStore()
	ctx := discardCtx("SWAPDB", bytesArgs("0", "1"), s)
	if err := cmdSWAPDB(ctx); err != nil {
		t.Fatal(err)
	}
}

func TestDeep2_ServerSync(t *testing.T) {
	s := store.NewStore()
	ctx := discardCtx("SYNC", bytesArgs(), s)
	if err := cmdSYNC(ctx); err != nil {
		t.Fatal(err)
	}
}

func TestDeep2_ServerPSync(t *testing.T) {
	s := store.NewStore()
	ctx := discardCtx("PSYNC", bytesArgs("?", "-1"), s)
	if err := cmdPSYNC(ctx); err != nil {
		t.Fatal(err)
	}
}

func TestDeep2_ServerMove(t *testing.T) {
	s := store.NewStore()
	s.Set("movekey", &store.StringValue{Data: []byte("val")}, store.SetOptions{})
	ctx := discardCtx("MOVE", bytesArgs("movekey", "1"), s)
	if err := cmdMOVE(ctx); err != nil {
		t.Fatal(err)
	}
}

func TestDeep2_ServerWaitAOF(t *testing.T) {
	s := store.NewStore()
	ctx := discardCtx("WAITAOF", bytesArgs("0", "0", "0"), s)
	if err := cmdWAITAOF(ctx); err != nil {
		t.Fatal(err)
	}
}

func TestDeep2_ServerShutdown(t *testing.T) {
	s := store.NewStore()
	ctx := discardCtx("SHUTDOWN", bytesArgs(), s)
	err := cmdSHUTDOWN(ctx)
	// SHUTDOWN is expected to return an error
	if err == nil {
		t.Fatal("expected SHUTDOWN error")
	}
}

func TestDeep2_ServerDebugSegfault(t *testing.T) {
	s := store.NewStore()
	ctx := discardCtx("DEBUGSEGFAULT", bytesArgs(), s)
	if err := cmdDEBUGSEGFAULT(ctx); err != nil {
		t.Fatal(err)
	}
}

// ======================================================================
// EXTENDED COMMANDS (extended_commands.go)
// ======================================================================

func TestDeep2_MsgQueueCommands(t *testing.T) {
	s := store.NewStore()

	// CREATE
	ctx := discardCtx("MSGQUEUE.CREATE", bytesArgs("mq1", "3"), s)
	if err := cmdMSGQUEUECREATE(ctx); err != nil {
		t.Fatal(err)
	}

	// PUBLISH
	ctx = discardCtx("MSGQUEUE.PUBLISH", bytesArgs("mq1", "hello"), s)
	if err := cmdMSGQUEUEPUBLISH(ctx); err != nil {
		t.Fatal(err)
	}
	// PUBLISH to nonexistent
	ctx = discardCtx("MSGQUEUE.PUBLISH", bytesArgs("nonexistent", "hello"), s)
	if err := cmdMSGQUEUEPUBLISH(ctx); err != nil {
		t.Fatal(err)
	}

	// CONSUME
	ctx = discardCtx("MSGQUEUE.CONSUME", bytesArgs("mq1"), s)
	if err := cmdMSGQUEUECONSUME(ctx); err != nil {
		t.Fatal(err)
	}

	// STATS
	ctx = discardCtx("MSGQUEUE.STATS", bytesArgs("mq1"), s)
	if err := cmdMSGQUEUESTATS(ctx); err != nil {
		t.Fatal(err)
	}

	// PURGE
	ctx = discardCtx("MSGQUEUE.PURGE", bytesArgs("mq1"), s)
	if err := cmdMSGQUEUEPURGE(ctx); err != nil {
		t.Fatal(err)
	}

	// DELETE
	ctx = discardCtx("MSGQUEUE.DELETE", bytesArgs("mq1"), s)
	if err := cmdMSGQUEUEDELETE(ctx); err != nil {
		t.Fatal(err)
	}
	ctx = discardCtx("MSGQUEUE.DELETE", bytesArgs("nonexistent"), s)
	if err := cmdMSGQUEUEDELETE(ctx); err != nil {
		t.Fatal(err)
	}
}

func TestDeep2_MsgQueueAckNack(t *testing.T) {
	s := store.NewStore()

	ctx := discardCtx("MSGQUEUE.CREATE", bytesArgs("mqack1", "1"), s)
	cmdMSGQUEUECREATE(ctx)

	ctx = discardCtx("MSGQUEUE.PUBLISH", bytesArgs("mqack1", "msg1"), s)
	cmdMSGQUEUEPUBLISH(ctx)

	ctx = discardCtx("MSGQUEUE.CONSUME", bytesArgs("mqack1"), s)
	cmdMSGQUEUECONSUME(ctx)

	// ACK with nonexistent queue
	ctx = discardCtx("MSGQUEUE.ACK", bytesArgs("nonexistent", "fakeid"), s)
	if err := cmdMSGQUEUEACK(ctx); err != nil {
		t.Fatal(err)
	}

	// ACK with unknown msg id
	ctx = discardCtx("MSGQUEUE.ACK", bytesArgs("mqack1", "unknownid"), s)
	if err := cmdMSGQUEUEACK(ctx); err != nil {
		t.Fatal(err)
	}

	// NACK
	ctx = discardCtx("MSGQUEUE.NACK", bytesArgs("mqack1", "unknownid"), s)
	if err := cmdMSGQUEUENACK(ctx); err != nil {
		t.Fatal(err)
	}

	// DEADLETTER
	ctx = discardCtx("MSGQUEUE.DEADLETTER", bytesArgs("mqack1"), s)
	if err := cmdMSGQUEUEDEADLETTER(ctx); err != nil {
		t.Fatal(err)
	}

	// REQUEUE
	ctx = discardCtx("MSGQUEUE.REQUEUE", bytesArgs("mqack1", "fakeid"), s)
	if err := cmdMSGQUEUEREQUEUE(ctx); err != nil {
		t.Fatal(err)
	}
}

func TestDeep2_ServiceCommands(t *testing.T) {
	s := store.NewStore()

	ctx := discardCtx("SERVICE.REGISTER", bytesArgs("svc1", "inst1", "127.0.0.1", "8080"), s)
	if err := cmdSERVICEREGISTER(ctx); err != nil {
		t.Fatal(err)
	}

	ctx = discardCtx("SERVICE.DISCOVER", bytesArgs("svc1"), s)
	if err := cmdSERVICEDISCOVER(ctx); err != nil {
		t.Fatal(err)
	}

	ctx = discardCtx("SERVICE.HEARTBEAT", bytesArgs("svc1", "inst1"), s)
	if err := cmdSERVICEHEARTBEAT(ctx); err != nil {
		t.Fatal(err)
	}

	ctx = discardCtx("SERVICE.LIST", bytesArgs(), s)
	if err := cmdSERVICELIST(ctx); err != nil {
		t.Fatal(err)
	}

	ctx = discardCtx("SERVICE.HEALTHY", bytesArgs("svc1"), s)
	if err := cmdSERVICEHEALTHY(ctx); err != nil {
		t.Fatal(err)
	}

	ctx = discardCtx("SERVICE.WEIGHT", bytesArgs("svc1", "inst1", "200"), s)
	if err := cmdSERVICEWEIGHT(ctx); err != nil {
		t.Fatal(err)
	}

	ctx = discardCtx("SERVICE.TAGS", bytesArgs("svc1", "inst1", "tag1", "tag2"), s)
	if err := cmdSERVICETAGS(ctx); err != nil {
		t.Fatal(err)
	}

	ctx = discardCtx("SERVICE.DEREGISTER", bytesArgs("svc1", "inst1"), s)
	if err := cmdSERVICEDEREGISTER(ctx); err != nil {
		t.Fatal(err)
	}
}

func TestDeep2_HealthXCommands(t *testing.T) {
	s := store.NewStore()

	ctx := discardCtx("HEALTHX.REGISTER", bytesArgs("hcheck1", "http", "http://localhost:8080/health", "30"), s)
	if err := cmdHEALTHXREGISTER(ctx); err != nil {
		t.Fatal(err)
	}

	ctx = discardCtx("HEALTHX.CHECK", bytesArgs("hcheck1"), s)
	if err := cmdHEALTHXCHECK(ctx); err != nil {
		t.Fatal(err)
	}

	ctx = discardCtx("HEALTHX.STATUS", bytesArgs("hcheck1"), s)
	if err := cmdHEALTHXSTATUS(ctx); err != nil {
		t.Fatal(err)
	}

	ctx = discardCtx("HEALTHX.HISTORY", bytesArgs("hcheck1"), s)
	if err := cmdHEALTHXHISTORY(ctx); err != nil {
		t.Fatal(err)
	}

	ctx = discardCtx("HEALTHX.LIST", bytesArgs(), s)
	if err := cmdHEALTHXLIST(ctx); err != nil {
		t.Fatal(err)
	}

	ctx = discardCtx("HEALTHX.UNREGISTER", bytesArgs("hcheck1"), s)
	if err := cmdHEALTHXUNREGISTER(ctx); err != nil {
		t.Fatal(err)
	}
}

func TestDeep2_CronCommands(t *testing.T) {
	s := store.NewStore()
	ctx := discardCtx("CRON.ADD", bytesArgs("cron1", "*/5 * * * *", "ECHO", "hello"), s)
	if err := cmdCRONADD(ctx); err != nil {
		t.Fatal(err)
	}
	ctx = discardCtx("CRON.LIST", bytesArgs(), s)
	if err := cmdCRONLIST(ctx); err != nil {
		t.Fatal(err)
	}
	ctx = discardCtx("CRON.TRIGGER", bytesArgs("cron1"), s)
	if err := cmdCRONTRIGGER(ctx); err != nil {
		t.Fatal(err)
	}
	ctx = discardCtx("CRON.PAUSE", bytesArgs("cron1"), s)
	if err := cmdCRONPAUSE(ctx); err != nil {
		t.Fatal(err)
	}
	ctx = discardCtx("CRON.RESUME", bytesArgs("cron1"), s)
	if err := cmdCRONRESUME(ctx); err != nil {
		t.Fatal(err)
	}
	ctx = discardCtx("CRON.NEXT", bytesArgs("cron1"), s)
	if err := cmdCRONNEXT(ctx); err != nil {
		t.Fatal(err)
	}
	ctx = discardCtx("CRON.HISTORY", bytesArgs("cron1"), s)
	if err := cmdCRONHISTORY(ctx); err != nil {
		t.Fatal(err)
	}
	ctx = discardCtx("CRON.REMOVE", bytesArgs("cron1"), s)
	if err := cmdCRONREMOVE(ctx); err != nil {
		t.Fatal(err)
	}
}

// ======================================================================
// MVCC COMMANDS (mvcc_commands.go)
// ======================================================================

func TestDeep2_MVCCFullCycle(t *testing.T) {
	s := store.NewStore()

	// BEGIN
	ctx := discardCtx("MVCC.BEGIN", bytesArgs(), s)
	if err := cmdMVCCBEGIN(ctx); err != nil {
		t.Fatal(err)
	}

	// We need the ID, so lets use the global counter
	id := mvccNextID

	// SET
	ctx = discardCtx("MVCC.SET", bytesArgs(intToStr(id), "mykey", "myval"), s)
	if err := cmdMVCCSET(ctx); err != nil {
		t.Fatal(err)
	}

	// GET
	ctx = discardCtx("MVCC.GET", bytesArgs(intToStr(id), "mykey"), s)
	if err := cmdMVCCGET(ctx); err != nil {
		t.Fatal(err)
	}

	// STATUS
	ctx = discardCtx("MVCC.STATUS", bytesArgs(intToStr(id)), s)
	if err := cmdMVCCSTATUS(ctx); err != nil {
		t.Fatal(err)
	}

	// COMMIT
	ctx = discardCtx("MVCC.COMMIT", bytesArgs(intToStr(id)), s)
	if err := cmdMVCCCOMMIT(ctx); err != nil {
		t.Fatal(err)
	}

	// nonexistent
	ctx = discardCtx("MVCC.STATUS", bytesArgs("99999"), s)
	if err := cmdMVCCSTATUS(ctx); err != nil {
		t.Fatal(err)
	}
}

func TestDeep2_MVCCRollback(t *testing.T) {
	s := store.NewStore()
	ctx := discardCtx("MVCC.BEGIN", bytesArgs(), s)
	cmdMVCCBEGIN(ctx)
	id := mvccNextID

	ctx = discardCtx("MVCC.SET", bytesArgs(intToStr(id), "k", "v"), s)
	cmdMVCCSET(ctx)

	ctx = discardCtx("MVCC.ROLLBACK", bytesArgs(intToStr(id)), s)
	if err := cmdMVCCROLLBACK(ctx); err != nil {
		t.Fatal(err)
	}
	// nonexistent
	ctx = discardCtx("MVCC.ROLLBACK", bytesArgs("99999"), s)
	if err := cmdMVCCROLLBACK(ctx); err != nil {
		t.Fatal(err)
	}
}

func TestDeep2_MVCCDelete(t *testing.T) {
	s := store.NewStore()
	s.Set("delkey", &store.StringValue{Data: []byte("val")}, store.SetOptions{})

	ctx := discardCtx("MVCC.BEGIN", bytesArgs(), s)
	cmdMVCCBEGIN(ctx)
	id := mvccNextID

	ctx = discardCtx("MVCC.DELETE", bytesArgs(intToStr(id), "delkey"), s)
	if err := cmdMVCCDELETE(ctx); err != nil {
		t.Fatal(err)
	}

	ctx = discardCtx("MVCC.COMMIT", bytesArgs(intToStr(id)), s)
	if err := cmdMVCCCOMMIT(ctx); err != nil {
		t.Fatal(err)
	}
}

func TestDeep2_MVCCSnapshot(t *testing.T) {
	s := store.NewStore()
	s.Set("snapkey", &store.StringValue{Data: []byte("snapval")}, store.SetOptions{})
	ctx := discardCtx("MVCC.SNAPSHOT", bytesArgs(), s)
	if err := cmdMVCCSNAPSHOT(ctx); err != nil {
		t.Fatal(err)
	}
}

func TestDeep2_SpatialCommands(t *testing.T) {
	s := store.NewStore()
	ctx := discardCtx("SPATIAL.CREATE", bytesArgs("idx1"), s)
	if err := cmdSPATIALCREATE(ctx); err != nil {
		t.Fatal(err)
	}
	ctx = discardCtx("SPATIAL.ADD", bytesArgs("idx1", "pt1", "37", "-122", "data1"), s)
	if err := cmdSPATIALADD(ctx); err != nil {
		t.Fatal(err)
	}
	ctx = discardCtx("SPATIAL.NEARBY", bytesArgs("idx1", "37", "-122", "100"), s)
	if err := cmdSPATIALNEARBY(ctx); err != nil {
		t.Fatal(err)
	}
	ctx = discardCtx("SPATIAL.WITHIN", bytesArgs("idx1", "36", "-123", "38", "-121"), s)
	if err := cmdSPATIALWITHIN(ctx); err != nil {
		t.Fatal(err)
	}
	ctx = discardCtx("SPATIAL.LIST", bytesArgs("idx1"), s)
	if err := cmdSPATIALLIST(ctx); err != nil {
		t.Fatal(err)
	}
	ctx = discardCtx("SPATIAL.DELETE", bytesArgs("idx1", "pt1"), s)
	if err := cmdSPATIALDELETE(ctx); err != nil {
		t.Fatal(err)
	}
}

func TestDeep2_ChainCommands(t *testing.T) {
	s := store.NewStore()
	ctx := discardCtx("CHAIN.CREATE", bytesArgs("chain1"), s)
	if err := cmdCHAINCREATE(ctx); err != nil {
		t.Fatal(err)
	}
	ctx = discardCtx("CHAIN.ADD", bytesArgs("chain1", "data1"), s)
	if err := cmdCHAINADD(ctx); err != nil {
		t.Fatal(err)
	}
	ctx = discardCtx("CHAIN.ADD", bytesArgs("chain1", "data2"), s)
	cmdCHAINADD(ctx)
	ctx = discardCtx("CHAIN.GET", bytesArgs("chain1", "0"), s)
	if err := cmdCHAINGET(ctx); err != nil {
		t.Fatal(err)
	}
	ctx = discardCtx("CHAIN.VALIDATE", bytesArgs("chain1"), s)
	if err := cmdCHAINVALIDATE(ctx); err != nil {
		t.Fatal(err)
	}
	ctx = discardCtx("CHAIN.LENGTH", bytesArgs("chain1"), s)
	if err := cmdCHAINLENGTH(ctx); err != nil {
		t.Fatal(err)
	}
	ctx = discardCtx("CHAIN.LAST", bytesArgs("chain1"), s)
	if err := cmdCHAINLAST(ctx); err != nil {
		t.Fatal(err)
	}
}

func TestDeep2_AnalyticsCommands(t *testing.T) {
	s := store.NewStore()
	ctx := discardCtx("ANALYTICS.INCR", bytesArgs("metric1", "5"), s)
	if err := cmdANALYTICSINCR(ctx); err != nil {
		t.Fatal(err)
	}
	ctx = discardCtx("ANALYTICS.DECR", bytesArgs("metric1", "2"), s)
	if err := cmdANALYTICSDECR(ctx); err != nil {
		t.Fatal(err)
	}
	ctx = discardCtx("ANALYTICS.GET", bytesArgs("metric1"), s)
	if err := cmdANALYTICSGET(ctx); err != nil {
		t.Fatal(err)
	}
	ctx = discardCtx("ANALYTICS.SUM", bytesArgs("metric1"), s)
	if err := cmdANALYTICSSUM(ctx); err != nil {
		t.Fatal(err)
	}
	ctx = discardCtx("ANALYTICS.AVG", bytesArgs("metric1"), s)
	if err := cmdANALYTICSAVG(ctx); err != nil {
		t.Fatal(err)
	}
	ctx = discardCtx("ANALYTICS.MIN", bytesArgs("metric1"), s)
	if err := cmdANALYTICSMIN(ctx); err != nil {
		t.Fatal(err)
	}
	ctx = discardCtx("ANALYTICS.MAX", bytesArgs("metric1"), s)
	if err := cmdANALYTICSMAX(ctx); err != nil {
		t.Fatal(err)
	}
	ctx = discardCtx("ANALYTICS.COUNT", bytesArgs("metric1"), s)
	if err := cmdANALYTICSCOUNT(ctx); err != nil {
		t.Fatal(err)
	}
	ctx = discardCtx("ANALYTICS.CLEAR", bytesArgs("metric1"), s)
	if err := cmdANALYTICSCLEAR(ctx); err != nil {
		t.Fatal(err)
	}
}

func TestDeep2_ConnectionCommands(t *testing.T) {
	s := store.NewStore()
	ctx := discardCtx("CONNECTION.LIST", bytesArgs(), s)
	if err := cmdCONNECTIONLIST(ctx); err != nil {
		t.Fatal(err)
	}
	ctx = discardCtx("CONNECTION.COUNT", bytesArgs(), s)
	if err := cmdCONNECTIONCOUNT(ctx); err != nil {
		t.Fatal(err)
	}
	ctx = discardCtx("CONNECTION.KILL", bytesArgs("fakeid"), s)
	if err := cmdCONNECTIONKILL(ctx); err != nil {
		t.Fatal(err)
	}
	ctx = discardCtx("CONNECTION.INFO", bytesArgs("fakeid"), s)
	if err := cmdCONNECTIONINFO(ctx); err != nil {
		t.Fatal(err)
	}
}

func TestDeep2_PluginCommands(t *testing.T) {
	s := store.NewStore()
	ctx := discardCtx("PLUGIN.LOAD", bytesArgs("plug1", "1.0"), s)
	if err := cmdPLUGINLOAD(ctx); err != nil {
		t.Fatal(err)
	}
	ctx = discardCtx("PLUGIN.LIST", bytesArgs(), s)
	if err := cmdPLUGINLIST(ctx); err != nil {
		t.Fatal(err)
	}
	ctx = discardCtx("PLUGIN.INFO", bytesArgs("plug1"), s)
	if err := cmdPLUGININFO(ctx); err != nil {
		t.Fatal(err)
	}
	ctx = discardCtx("PLUGIN.CALL", bytesArgs("plug1", "func1"), s)
	if err := cmdPLUGINCALL(ctx); err != nil {
		t.Fatal(err)
	}
	ctx = discardCtx("PLUGIN.UNLOAD", bytesArgs("plug1"), s)
	if err := cmdPLUGINUNLOAD(ctx); err != nil {
		t.Fatal(err)
	}
}

func TestDeep2_RollupCommands(t *testing.T) {
	s := store.NewStore()
	ctx := discardCtx("ROLLUP.CREATE", bytesArgs("roll1", "60"), s)
	if err := cmdROLLUPCREATE(ctx); err != nil {
		t.Fatal(err)
	}
	ctx = discardCtx("ROLLUP.ADD", bytesArgs("roll1", "10"), s)
	if err := cmdROLLUPADD(ctx); err != nil {
		t.Fatal(err)
	}
	ctx = discardCtx("ROLLUP.GET", bytesArgs("roll1"), s)
	if err := cmdROLLUPGET(ctx); err != nil {
		t.Fatal(err)
	}
	ctx = discardCtx("ROLLUP.DELETE", bytesArgs("roll1"), s)
	if err := cmdROLLUPDELETE(ctx); err != nil {
		t.Fatal(err)
	}
}

func TestDeep2_CooldownCommands(t *testing.T) {
	s := store.NewStore()
	ctx := discardCtx("COOLDOWN.SET", bytesArgs("cd1", "5000"), s)
	if err := cmdCOOLDOWNSET(ctx); err != nil {
		t.Fatal(err)
	}
	ctx = discardCtx("COOLDOWN.CHECK", bytesArgs("cd1"), s)
	if err := cmdCOOLDOWNCHECK(ctx); err != nil {
		t.Fatal(err)
	}
	ctx = discardCtx("COOLDOWN.LIST", bytesArgs(), s)
	if err := cmdCOOLDOWNLIST(ctx); err != nil {
		t.Fatal(err)
	}
	ctx = discardCtx("COOLDOWN.RESET", bytesArgs("cd1"), s)
	if err := cmdCOOLDOWNRESET(ctx); err != nil {
		t.Fatal(err)
	}
	ctx = discardCtx("COOLDOWN.DELETE", bytesArgs("cd1"), s)
	if err := cmdCOOLDOWNDELETE(ctx); err != nil {
		t.Fatal(err)
	}
}

func TestDeep2_QuotaCommands(t *testing.T) {
	s := store.NewStore()
	ctx := discardCtx("QUOTA.SET", bytesArgs("q1", "100"), s)
	if err := cmdQUOTASET(ctx); err != nil {
		t.Fatal(err)
	}
	ctx = discardCtx("QUOTA.CHECK", bytesArgs("q1"), s)
	if err := cmdQUOTACHECK(ctx); err != nil {
		t.Fatal(err)
	}
	ctx = discardCtx("QUOTA.USE", bytesArgs("q1", "10"), s)
	if err := cmdQUOTAUSE(ctx); err != nil {
		t.Fatal(err)
	}
	ctx = discardCtx("QUOTA.LIST", bytesArgs(), s)
	if err := cmdQUOTALIST(ctx); err != nil {
		t.Fatal(err)
	}
	ctx = discardCtx("QUOTA.RESET", bytesArgs("q1"), s)
	if err := cmdQUOTARESET(ctx); err != nil {
		t.Fatal(err)
	}
	ctx = discardCtx("QUOTA.DELETE", bytesArgs("q1"), s)
	if err := cmdQUOTADELETE(ctx); err != nil {
		t.Fatal(err)
	}
}

// ======================================================================
// EVENT COMMANDS (event_commands.go)
// ======================================================================

func TestDeep2_EventEmit(t *testing.T) {
	s := store.NewStore()
	ctx := discardCtx("EVENT.EMIT", bytesArgs("testevent", "key1", "val1"), s)
	if err := cmdEVENTEMIT(ctx); err != nil {
		t.Fatal(err)
	}
}

func TestDeep2_EventGet(t *testing.T) {
	s := store.NewStore()
	ctx := discardCtx("EVENT.EMIT", bytesArgs("evget1"), s)
	cmdEVENTEMIT(ctx)
	ctx = discardCtx("EVENT.GET", bytesArgs("evget1"), s)
	if err := cmdEVENTGET(ctx); err != nil {
		t.Fatal(err)
	}
	ctx = discardCtx("EVENT.GET", bytesArgs("evget1", "5"), s)
	if err := cmdEVENTGET(ctx); err != nil {
		t.Fatal(err)
	}
}

func TestDeep2_EventList(t *testing.T) {
	s := store.NewStore()
	ctx := discardCtx("EVENT.LIST", bytesArgs(), s)
	if err := cmdEVENTLIST(ctx); err != nil {
		t.Fatal(err)
	}
}

func TestDeep2_EventClear(t *testing.T) {
	s := store.NewStore()
	ctx := discardCtx("EVENT.CLEAR", bytesArgs(), s)
	if err := cmdEVENTCLEAR(ctx); err != nil {
		t.Fatal(err)
	}
}

func TestDeep2_WebhookCommands(t *testing.T) {
	s := store.NewStore()
	ctx := discardCtx("WEBHOOK.CREATE", bytesArgs("wh1", "http://example.com", "POST", "evt1"), s)
	if err := cmdWEBHOOKCREATE(ctx); err != nil {
		t.Fatal(err)
	}
	ctx = discardCtx("WEBHOOK.GET", bytesArgs("wh1"), s)
	if err := cmdWEBHOOKGET(ctx); err != nil {
		t.Fatal(err)
	}
	ctx = discardCtx("WEBHOOK.GET", bytesArgs("nonexistent"), s)
	if err := cmdWEBHOOKGET(ctx); err != nil {
		t.Fatal(err)
	}
	ctx = discardCtx("WEBHOOK.LIST", bytesArgs(), s)
	if err := cmdWEBHOOKLIST(ctx); err != nil {
		t.Fatal(err)
	}
	ctx = discardCtx("WEBHOOK.DISABLE", bytesArgs("wh1"), s)
	if err := cmdWEBHOOKDISABLE(ctx); err != nil {
		t.Fatal(err)
	}
	ctx = discardCtx("WEBHOOK.ENABLE", bytesArgs("wh1"), s)
	if err := cmdWEBHOOKENABLE(ctx); err != nil {
		t.Fatal(err)
	}
	ctx = discardCtx("WEBHOOK.STATS", bytesArgs("wh1"), s)
	if err := cmdWEBHOOKSTATS(ctx); err != nil {
		t.Fatal(err)
	}
	ctx = discardCtx("WEBHOOK.STATS", bytesArgs("nonexistent"), s)
	if err := cmdWEBHOOKSTATS(ctx); err != nil {
		t.Fatal(err)
	}
	ctx = discardCtx("WEBHOOK.DELETE", bytesArgs("wh1"), s)
	if err := cmdWEBHOOKDELETE(ctx); err != nil {
		t.Fatal(err)
	}
	ctx = discardCtx("WEBHOOK.DELETE", bytesArgs("nonexistent"), s)
	if err := cmdWEBHOOKDELETE(ctx); err != nil {
		t.Fatal(err)
	}
}

func TestDeep2_CompressRLE(t *testing.T) {
	s := store.NewStore()
	ctx := discardCtx("COMPRESS.RLE", bytesArgs("aaabbbccc"), s)
	if err := cmdCOMPRESSRLE(ctx); err != nil {
		t.Fatal(err)
	}
}

func TestDeep2_DecompressRLE(t *testing.T) {
	s := store.NewStore()
	compressor := &store.RLECompressor{}
	compressed, _ := compressor.Compress([]byte("aaabbbccc"))
	ctx := discardCtx("DECOMPRESS.RLE", [][]byte{compressed}, s)
	if err := cmdDECOMPRESSRLE(ctx); err != nil {
		t.Fatal(err)
	}
}

func TestDeep2_CompressLZ4(t *testing.T) {
	s := store.NewStore()
	ctx := discardCtx("COMPRESS.LZ4", bytesArgs("hello world"), s)
	if err := cmdCOMPRESSLZ4(ctx); err != nil {
		t.Fatal(err)
	}
}

func TestDeep2_DecompressLZ4(t *testing.T) {
	s := store.NewStore()
	compressor := &store.LZ4Compressor{}
	compressed, _ := compressor.Compress([]byte("hello world"))
	ctx := discardCtx("DECOMPRESS.LZ4", [][]byte{compressed}, s)
	if err := cmdDECOMPRESSLZ4(ctx); err != nil {
		t.Fatal(err)
	}
}

func TestDeep2_CompressCustom(t *testing.T) {
	s := store.NewStore()
	ctx := discardCtx("COMPRESS.CUSTOM", bytesArgs("RLE", "aaabbb"), s)
	if err := cmdCOMPRESSCUSTOM(ctx); err != nil {
		t.Fatal(err)
	}
	ctx = discardCtx("COMPRESS.CUSTOM", bytesArgs("LZ4", "hello"), s)
	if err := cmdCOMPRESSCUSTOM(ctx); err != nil {
		t.Fatal(err)
	}
	// Unknown algo
	ctx = discardCtx("COMPRESS.CUSTOM", bytesArgs("ZSTD", "hello"), s)
	if err := cmdCOMPRESSCUSTOM(ctx); err != nil {
		t.Fatal(err)
	}
}

func TestDeep2_QueueCommands(t *testing.T) {
	s := store.NewStore()
	ctx := discardCtx("QUEUE.CREATE", bytesArgs("q1"), s)
	if err := cmdQUEUECREATE(ctx); err != nil {
		t.Fatal(err)
	}
	ctx = discardCtx("QUEUE.PUSH", bytesArgs("q1", "item1"), s)
	if err := cmdQUEUEPUSH(ctx); err != nil {
		t.Fatal(err)
	}
	ctx = discardCtx("QUEUE.PEEK", bytesArgs("q1"), s)
	if err := cmdQUEUEPEEK(ctx); err != nil {
		t.Fatal(err)
	}
	ctx = discardCtx("QUEUE.LEN", bytesArgs("q1"), s)
	if err := cmdQUEUELEN(ctx); err != nil {
		t.Fatal(err)
	}
	ctx = discardCtx("QUEUE.POP", bytesArgs("q1"), s)
	if err := cmdQUEUEPOP(ctx); err != nil {
		t.Fatal(err)
	}
	// Pop from empty
	ctx = discardCtx("QUEUE.POP", bytesArgs("q1"), s)
	if err := cmdQUEUEPOP(ctx); err != nil {
		t.Fatal(err)
	}
	ctx = discardCtx("QUEUE.CLEAR", bytesArgs("q1"), s)
	if err := cmdQUEUECLEAR(ctx); err != nil {
		t.Fatal(err)
	}
}

func TestDeep2_StackCommands(t *testing.T) {
	s := store.NewStore()
	ctx := discardCtx("STACK.CREATE", bytesArgs("st1"), s)
	if err := cmdSTACKCREATE(ctx); err != nil {
		t.Fatal(err)
	}
	ctx = discardCtx("STACK.PUSH", bytesArgs("st1", "item1"), s)
	if err := cmdSTACKPUSH(ctx); err != nil {
		t.Fatal(err)
	}
	ctx = discardCtx("STACK.PUSH", bytesArgs("st1", "item2"), s)
	cmdSTACKPUSH(ctx)
	ctx = discardCtx("STACK.PEEK", bytesArgs("st1"), s)
	if err := cmdSTACKPEEK(ctx); err != nil {
		t.Fatal(err)
	}
	ctx = discardCtx("STACK.LEN", bytesArgs("st1"), s)
	if err := cmdSTACKLEN(ctx); err != nil {
		t.Fatal(err)
	}
	ctx = discardCtx("STACK.POP", bytesArgs("st1"), s)
	if err := cmdSTACKPOP(ctx); err != nil {
		t.Fatal(err)
	}
	// Pop from empty
	ctx = discardCtx("STACK.POP", bytesArgs("nonexistent"), s)
	if err := cmdSTACKPOP(ctx); err != nil {
		t.Fatal(err)
	}
	ctx = discardCtx("STACK.CLEAR", bytesArgs("st1"), s)
	if err := cmdSTACKCLEAR(ctx); err != nil {
		t.Fatal(err)
	}
}

// ======================================================================
// UTILITY EXT COMMANDS (utility_ext_commands.go)
// ======================================================================

func TestDeep2_AuditCommands(t *testing.T) {
	s := store.NewStore()
	ctx := discardCtx("AUDIT.ENABLE", bytesArgs(), s)
	if err := cmdAUDITENABLE(ctx); err != nil {
		t.Fatal(err)
	}
	ctx = discardCtx("AUDIT.LOG", bytesArgs("SET", "mykey", "myval"), s)
	if err := cmdAUDITLOG(ctx); err != nil {
		t.Fatal(err)
	}
	ctx = discardCtx("AUDIT.LOG", bytesArgs("GET"), s)
	if err := cmdAUDITLOG(ctx); err != nil {
		t.Fatal(err)
	}
	ctx = discardCtx("AUDIT.COUNT", bytesArgs(), s)
	if err := cmdAUDITCOUNT(ctx); err != nil {
		t.Fatal(err)
	}
	ctx = discardCtx("AUDIT.GET", bytesArgs("1"), s)
	if err := cmdAUDITGET(ctx); err != nil {
		t.Fatal(err)
	}
	// nonexistent
	ctx = discardCtx("AUDIT.GET", bytesArgs("99999"), s)
	if err := cmdAUDITGET(ctx); err != nil {
		t.Fatal(err)
	}
	ctx = discardCtx("AUDIT.GETRANGE", bytesArgs("0", "9999999999999"), s)
	if err := cmdAUDITGETRANGE(ctx); err != nil {
		t.Fatal(err)
	}
	ctx = discardCtx("AUDIT.GETBYCMD", bytesArgs("SET"), s)
	if err := cmdAUDITGETBYCMD(ctx); err != nil {
		t.Fatal(err)
	}
	ctx = discardCtx("AUDIT.GETBYCMD", bytesArgs("SET", "10"), s)
	if err := cmdAUDITGETBYCMD(ctx); err != nil {
		t.Fatal(err)
	}
	ctx = discardCtx("AUDIT.GETBYKEY", bytesArgs("mykey"), s)
	if err := cmdAUDITGETBYKEY(ctx); err != nil {
		t.Fatal(err)
	}
	ctx = discardCtx("AUDIT.GETBYKEY", bytesArgs("mykey", "10"), s)
	if err := cmdAUDITGETBYKEY(ctx); err != nil {
		t.Fatal(err)
	}
	ctx = discardCtx("AUDIT.STATS", bytesArgs(), s)
	if err := cmdAUDITSTATS(ctx); err != nil {
		t.Fatal(err)
	}
	ctx = discardCtx("AUDIT.DISABLE", bytesArgs(), s)
	if err := cmdAUDITDISABLE(ctx); err != nil {
		t.Fatal(err)
	}
	ctx = discardCtx("AUDIT.CLEAR", bytesArgs(), s)
	if err := cmdAUDITCLEAR(ctx); err != nil {
		t.Fatal(err)
	}
}

func TestDeep2_FlagCommands(t *testing.T) {
	s := store.NewStore()
	ctx := discardCtx("FLAG.CREATE", bytesArgs("flag1", "test flag"), s)
	if err := cmdFLAGCREATE(ctx); err != nil {
		t.Fatal(err)
	}
	ctx = discardCtx("FLAG.CREATE", bytesArgs("flag2"), s)
	cmdFLAGCREATE(ctx)

	ctx = discardCtx("FLAG.GET", bytesArgs("flag1"), s)
	if err := cmdFLAGGET(ctx); err != nil {
		t.Fatal(err)
	}
	ctx = discardCtx("FLAG.GET", bytesArgs("nonexistent"), s)
	if err := cmdFLAGGET(ctx); err != nil {
		t.Fatal(err)
	}
	ctx = discardCtx("FLAG.ENABLE", bytesArgs("flag1"), s)
	if err := cmdFLAGENABLE(ctx); err != nil {
		t.Fatal(err)
	}
	ctx = discardCtx("FLAG.ISENABLED", bytesArgs("flag1"), s)
	if err := cmdFLAGISENABLED(ctx); err != nil {
		t.Fatal(err)
	}
	ctx = discardCtx("FLAG.DISABLE", bytesArgs("flag1"), s)
	if err := cmdFLAGDISABLE(ctx); err != nil {
		t.Fatal(err)
	}
	ctx = discardCtx("FLAG.TOGGLE", bytesArgs("flag1"), s)
	if err := cmdFLAGTOGGLE(ctx); err != nil {
		t.Fatal(err)
	}
	ctx = discardCtx("FLAG.LIST", bytesArgs(), s)
	if err := cmdFLAGLIST(ctx); err != nil {
		t.Fatal(err)
	}
	ctx = discardCtx("FLAG.LISTENABLED", bytesArgs(), s)
	if err := cmdFLAGLISTENABLED(ctx); err != nil {
		t.Fatal(err)
	}
	ctx = discardCtx("FLAG.ADDVARIANT", bytesArgs("flag1", "color", "blue"), s)
	if err := cmdFLAGADDVARIANT(ctx); err != nil {
		t.Fatal(err)
	}
	// nonexistent flag
	ctx = discardCtx("FLAG.ADDVARIANT", bytesArgs("nonexistent", "k", "v"), s)
	if err := cmdFLAGADDVARIANT(ctx); err != nil {
		t.Fatal(err)
	}
	ctx = discardCtx("FLAG.GETVARIANT", bytesArgs("flag1", "color"), s)
	if err := cmdFLAGGETVARIANT(ctx); err != nil {
		t.Fatal(err)
	}
	ctx = discardCtx("FLAG.GETVARIANT", bytesArgs("nonexistent", "k"), s)
	if err := cmdFLAGGETVARIANT(ctx); err != nil {
		t.Fatal(err)
	}
	ctx = discardCtx("FLAG.ADDRULE", bytesArgs("flag1", "country", "eq", "US"), s)
	if err := cmdFLAGADDRULE(ctx); err != nil {
		t.Fatal(err)
	}
	ctx = discardCtx("FLAG.ADDRULE", bytesArgs("nonexistent", "a", "b", "c"), s)
	if err := cmdFLAGADDRULE(ctx); err != nil {
		t.Fatal(err)
	}
	ctx = discardCtx("FLAG.DELETE", bytesArgs("flag1"), s)
	if err := cmdFLAGDELETE(ctx); err != nil {
		t.Fatal(err)
	}
	ctx = discardCtx("FLAG.DELETE", bytesArgs("nonexistent"), s)
	if err := cmdFLAGDELETE(ctx); err != nil {
		t.Fatal(err)
	}
}

func TestDeep2_CounterCommands(t *testing.T) {
	s := store.NewStore()
	ctx := discardCtx("COUNTER.SET", bytesArgs("cnt1", "10"), s)
	if err := cmdCOUNTERSET(ctx); err != nil {
		t.Fatal(err)
	}
	ctx = discardCtx("COUNTER.GET", bytesArgs("cnt1"), s)
	if err := cmdCOUNTERGET(ctx); err != nil {
		t.Fatal(err)
	}
	ctx = discardCtx("COUNTER.INCR", bytesArgs("cnt1"), s)
	if err := cmdCOUNTERINCR(ctx); err != nil {
		t.Fatal(err)
	}
	ctx = discardCtx("COUNTER.DECR", bytesArgs("cnt1"), s)
	if err := cmdCOUNTERDECR(ctx); err != nil {
		t.Fatal(err)
	}
	ctx = discardCtx("COUNTER.INCRBY", bytesArgs("cnt1", "5"), s)
	if err := cmdCOUNTERINCRBY(ctx); err != nil {
		t.Fatal(err)
	}
	ctx = discardCtx("COUNTER.DECRBY", bytesArgs("cnt1", "3"), s)
	if err := cmdCOUNTERDECRBY(ctx); err != nil {
		t.Fatal(err)
	}
	ctx = discardCtx("COUNTER.LIST", bytesArgs(), s)
	if err := cmdCOUNTERLIST(ctx); err != nil {
		t.Fatal(err)
	}
	ctx = discardCtx("COUNTER.GETALL", bytesArgs(), s)
	if err := cmdCOUNTERGETALL(ctx); err != nil {
		t.Fatal(err)
	}
	ctx = discardCtx("COUNTER.RESET", bytesArgs("cnt1"), s)
	if err := cmdCOUNTERRESET(ctx); err != nil {
		t.Fatal(err)
	}
	ctx = discardCtx("COUNTER.RESET", bytesArgs("nonexistent"), s)
	if err := cmdCOUNTERRESET(ctx); err != nil {
		t.Fatal(err)
	}
	ctx = discardCtx("COUNTER.RESETALL", bytesArgs(), s)
	if err := cmdCOUNTERRESETALL(ctx); err != nil {
		t.Fatal(err)
	}
	ctx = discardCtx("COUNTER.DELETE", bytesArgs("cnt1"), s)
	if err := cmdCOUNTERDELETE(ctx); err != nil {
		t.Fatal(err)
	}
}

func TestDeep2_BackupCommands(t *testing.T) {
	s := store.NewStore()
	s.Set("bkkey", &store.StringValue{Data: []byte("bkval")}, store.SetOptions{})
	ctx := discardCtx("BACKUP.CREATE", bytesArgs("bk1"), s)
	if err := cmdBACKUPCREATE(ctx); err != nil {
		t.Fatal(err)
	}
	ctx = discardCtx("BACKUP.LIST", bytesArgs(), s)
	if err := cmdBACKUPLIST(ctx); err != nil {
		t.Fatal(err)
	}
	ctx = discardCtx("BACKUP.RESTORE", bytesArgs("bk1"), s)
	if err := cmdBACKUPRESTORE(ctx); err != nil {
		t.Fatal(err)
	}
	ctx = discardCtx("BACKUP.RESTORE", bytesArgs("nonexistent"), s)
	if err := cmdBACKUPRESTORE(ctx); err != nil {
		t.Fatal(err)
	}
	ctx = discardCtx("BACKUP.DELETE", bytesArgs("bk1"), s)
	if err := cmdBACKUPDELETE(ctx); err != nil {
		t.Fatal(err)
	}
	ctx = discardCtx("BACKUP.DELETE", bytesArgs("nonexistent"), s)
	if err := cmdBACKUPDELETE(ctx); err != nil {
		t.Fatal(err)
	}
}

func TestDeep2_MemoryCommands(t *testing.T) {
	s := store.NewStore()
	ctx := discardCtx("MEMORY.TRIM", bytesArgs(), s)
	if err := cmdMEMORYTRIM(ctx); err != nil {
		t.Fatal(err)
	}
	ctx = discardCtx("MEMORY.FRAG", bytesArgs(), s)
	if err := cmdMEMORYFRAG(ctx); err != nil {
		t.Fatal(err)
	}
	ctx = discardCtx("MEMORY.PURGE", bytesArgs(), s)
	if err := cmdMEMORYPURGE(ctx); err != nil {
		t.Fatal(err)
	}
	ctx = discardCtx("MEMORY.ALLOC", bytesArgs("1024"), s)
	if err := cmdMEMORYALLOC(ctx); err != nil {
		t.Fatal(err)
	}
}

// ======================================================================
// ML COMMANDS (ml_commands.go)
// ======================================================================

func TestDeep2_MLModelCommands(t *testing.T) {
	s := store.NewStore()
	ctx := discardCtx("MODEL.CREATE", bytesArgs("mdl1", "regression"), s)
	if err := cmdMODELCREATE(ctx); err != nil {
		t.Fatal(err)
	}
	ctx = discardCtx("MODEL.TRAIN", bytesArgs("mdl1", "data1"), s)
	if err := cmdMODELTRAIN(ctx); err != nil {
		t.Fatal(err)
	}
	ctx = discardCtx("MODEL.PREDICT", bytesArgs("mdl1", "input1"), s)
	if err := cmdMODELPREDICT(ctx); err != nil {
		t.Fatal(err)
	}
	ctx = discardCtx("MODEL.LIST", bytesArgs(), s)
	if err := cmdMODELLIST(ctx); err != nil {
		t.Fatal(err)
	}
	ctx = discardCtx("MODEL.STATUS", bytesArgs("mdl1"), s)
	if err := cmdMODELSTATUS(ctx); err != nil {
		t.Fatal(err)
	}
	ctx = discardCtx("MODEL.DELETE", bytesArgs("mdl1"), s)
	if err := cmdMODELDELETE(ctx); err != nil {
		t.Fatal(err)
	}
	ctx = discardCtx("MODEL.DELETE", bytesArgs("nonexistent"), s)
	cmdMODELDELETE(ctx)
}

func TestDeep2_FeatureCommands(t *testing.T) {
	s := store.NewStore()
	ctx := discardCtx("FEATURE.SET", bytesArgs("user1", "age", "25"), s)
	if err := cmdFEATURESET(ctx); err != nil {
		t.Fatal(err)
	}
	ctx = discardCtx("FEATURE.GET", bytesArgs("user1", "age"), s)
	if err := cmdFEATUREGET(ctx); err != nil {
		t.Fatal(err)
	}
	ctx = discardCtx("FEATURE.GET", bytesArgs("nonexistent", "k"), s)
	cmdFEATUREGET(ctx)
	ctx = discardCtx("FEATURE.INCR", bytesArgs("user1", "age", "1"), s)
	if err := cmdFEATUREINCR(ctx); err != nil {
		t.Fatal(err)
	}
	ctx = discardCtx("FEATURE.NORMALIZE", bytesArgs("user1", "minmax"), s)
	if err := cmdFEATURENORMALIZE(ctx); err != nil {
		t.Fatal(err)
	}
	ctx = discardCtx("FEATURE.VECTOR", bytesArgs("user1"), s)
	if err := cmdFEATUREVECTOR(ctx); err != nil {
		t.Fatal(err)
	}
	ctx = discardCtx("FEATURE.VECTOR", bytesArgs("nonexistent"), s)
	cmdFEATUREVECTOR(ctx)
	ctx = discardCtx("FEATURE.DEL", bytesArgs("user1", "age"), s)
	if err := cmdFEATUREDEL(ctx); err != nil {
		t.Fatal(err)
	}
	ctx = discardCtx("FEATURE.DEL", bytesArgs("nonexistent", "k"), s)
	cmdFEATUREDEL(ctx)
}

func TestDeep2_EmbeddingCommands(t *testing.T) {
	s := store.NewStore()
	ctx := discardCtx("EMBEDDING.CREATE", bytesArgs("emb1", "1", "2", "3"), s)
	if err := cmdEMBEDDINGCREATE(ctx); err != nil {
		t.Fatal(err)
	}
	ctx = discardCtx("EMBEDDING.GET", bytesArgs("emb1"), s)
	if err := cmdEMBEDDINGGET(ctx); err != nil {
		t.Fatal(err)
	}
	ctx = discardCtx("EMBEDDING.GET", bytesArgs("nonexistent"), s)
	cmdEMBEDDINGGET(ctx)
	ctx = discardCtx("EMBEDDING.SEARCH", bytesArgs("emb1", "5"), s)
	if err := cmdEMBEDDINGSEARCH(ctx); err != nil {
		t.Fatal(err)
	}
	ctx = discardCtx("EMBEDDING.CREATE", bytesArgs("emb2", "4", "5", "6"), s)
	cmdEMBEDDINGCREATE(ctx)
	ctx = discardCtx("EMBEDDING.SIMILAR", bytesArgs("emb1", "5"), s)
	if err := cmdEMBEDDINGSIMILAR(ctx); err != nil {
		t.Fatal(err)
	}
	ctx = discardCtx("EMBEDDING.DELETE", bytesArgs("emb1"), s)
	if err := cmdEMBEDDINGDELETE(ctx); err != nil {
		t.Fatal(err)
	}
}

func TestDeep2_SentimentAnalyze(t *testing.T) {
	s := store.NewStore()
	ctx := discardCtx("SENTIMENT.ANALYZE", bytesArgs("I love this product"), s)
	if err := cmdSENTIMENTANALYZE(ctx); err != nil {
		t.Fatal(err)
	}
	ctx = discardCtx("SENTIMENT.BATCH", bytesArgs("great", "terrible", "ok"), s)
	if err := cmdSENTIMENTBATCH(ctx); err != nil {
		t.Fatal(err)
	}
}

func TestDeep2_NLPCommands(t *testing.T) {
	s := store.NewStore()
	ctx := discardCtx("NLP.TOKENIZE", bytesArgs("hello world foo"), s)
	if err := cmdNLPTOKENIZE(ctx); err != nil {
		t.Fatal(err)
	}
	ctx = discardCtx("NLP.ENTITIES", bytesArgs("John lives in New York"), s)
	if err := cmdNLPENTITIES(ctx); err != nil {
		t.Fatal(err)
	}
	ctx = discardCtx("NLP.KEYWORDS", bytesArgs("the quick brown fox"), s)
	if err := cmdNLPKEYWORDS(ctx); err != nil {
		t.Fatal(err)
	}
	ctx = discardCtx("NLP.SUMMARIZE", bytesArgs("long text goes here and it should be summarized"), s)
	if err := cmdNLPSUMMARIZE(ctx); err != nil {
		t.Fatal(err)
	}
}

func TestDeep2_SimilarityCommands(t *testing.T) {
	s := store.NewStore()
	ctx := discardCtx("SIMILARITY.COSINE", bytesArgs("1", "2", "3", "4", "5", "6"), s)
	if err := cmdSIMILARITYCOSINE(ctx); err != nil {
		t.Fatal(err)
	}
	ctx = discardCtx("SIMILARITY.EUCLIDEAN", bytesArgs("1", "2", "3", "4", "5", "6"), s)
	if err := cmdSIMILARITYEUCLIDEAN(ctx); err != nil {
		t.Fatal(err)
	}
	ctx = discardCtx("SIMILARITY.JACCARD", bytesArgs("a", "b", "c", "|", "b", "c", "d"), s)
	if err := cmdSIMILARITYJACCARD(ctx); err != nil {
		t.Fatal(err)
	}
	ctx = discardCtx("SIMILARITY.DOTPRODUCT", bytesArgs("1", "2", "3", "4", "5", "6"), s)
	if err := cmdSIMILARITYDOTPRODUCT(ctx); err != nil {
		t.Fatal(err)
	}
}

// ======================================================================
// ADVANCED COMMANDS (advanced_commands.go)
// ======================================================================

func TestDeep2_ActorCommands(t *testing.T) {
	s := store.NewStore()
	ctx := discardCtx("ACTOR.CREATE", bytesArgs("act1"), s)
	if err := cmdACTORCREATE(ctx); err != nil {
		t.Fatal(err)
	}
	ctx = discardCtx("ACTOR.SEND", bytesArgs("act1", "msg1"), s)
	if err := cmdACTORSEND(ctx); err != nil {
		t.Fatal(err)
	}
	ctx = discardCtx("ACTOR.SEND", bytesArgs("nonexistent", "msg1"), s)
	if err := cmdACTORSEND(ctx); err != nil {
		t.Fatal(err)
	}
	ctx = discardCtx("ACTOR.POKE", bytesArgs("act1"), s)
	if err := cmdACTORPOKE(ctx); err != nil {
		t.Fatal(err)
	}
	ctx = discardCtx("ACTOR.PEEK", bytesArgs("act1"), s)
	if err := cmdACTORPEEK(ctx); err != nil {
		t.Fatal(err)
	}
	ctx = discardCtx("ACTOR.LEN", bytesArgs("act1"), s)
	if err := cmdACTORLEN(ctx); err != nil {
		t.Fatal(err)
	}
	ctx = discardCtx("ACTOR.LIST", bytesArgs(), s)
	if err := cmdACTORLIST(ctx); err != nil {
		t.Fatal(err)
	}
	ctx = discardCtx("ACTOR.RECV", bytesArgs("act1"), s)
	if err := cmdACTORRECV(ctx); err != nil {
		t.Fatal(err)
	}
	ctx = discardCtx("ACTOR.RECV", bytesArgs("nonexistent"), s)
	if err := cmdACTORRECV(ctx); err != nil {
		t.Fatal(err)
	}
	ctx = discardCtx("ACTOR.CLEAR", bytesArgs("act1"), s)
	if err := cmdACTORCLEAR(ctx); err != nil {
		t.Fatal(err)
	}
	ctx = discardCtx("ACTOR.CLEAR", bytesArgs("nonexistent"), s)
	cmdACTORCLEAR(ctx)
	ctx = discardCtx("ACTOR.DELETE", bytesArgs("act1"), s)
	if err := cmdACTORDELETE(ctx); err != nil {
		t.Fatal(err)
	}
	ctx = discardCtx("ACTOR.DELETE", bytesArgs("nonexistent"), s)
	cmdACTORDELETE(ctx)
}

func TestDeep2_DAGCommands(t *testing.T) {
	s := store.NewStore()
	ctx := discardCtx("DAG.CREATE", bytesArgs("dag1"), s)
	if err := cmdDAGCREATE(ctx); err != nil {
		t.Fatal(err)
	}
	ctx = discardCtx("DAG.ADDNODE", bytesArgs("dag1", "A"), s)
	if err := cmdDAGADDNODE(ctx); err != nil {
		t.Fatal(err)
	}
	ctx = discardCtx("DAG.ADDNODE", bytesArgs("dag1", "B"), s)
	cmdDAGADDNODE(ctx)
	ctx = discardCtx("DAG.ADDNODE", bytesArgs("dag1", "C"), s)
	cmdDAGADDNODE(ctx)
	// duplicate node
	ctx = discardCtx("DAG.ADDNODE", bytesArgs("dag1", "A"), s)
	cmdDAGADDNODE(ctx)
	ctx = discardCtx("DAG.ADDEDGE", bytesArgs("dag1", "A", "B"), s)
	if err := cmdDAGADDEDGE(ctx); err != nil {
		t.Fatal(err)
	}
	ctx = discardCtx("DAG.ADDEDGE", bytesArgs("dag1", "B", "C"), s)
	cmdDAGADDEDGE(ctx)
	ctx = discardCtx("DAG.TOPO", bytesArgs("dag1"), s)
	if err := cmdDAGTOPO(ctx); err != nil {
		t.Fatal(err)
	}
	ctx = discardCtx("DAG.PARENTS", bytesArgs("dag1", "B"), s)
	if err := cmdDAGPARENTS(ctx); err != nil {
		t.Fatal(err)
	}
	ctx = discardCtx("DAG.CHILDREN", bytesArgs("dag1", "A"), s)
	if err := cmdDAGCHILDREN(ctx); err != nil {
		t.Fatal(err)
	}
	ctx = discardCtx("DAG.LIST", bytesArgs(), s)
	if err := cmdDAGLIST(ctx); err != nil {
		t.Fatal(err)
	}
	ctx = discardCtx("DAG.DELETE", bytesArgs("dag1"), s)
	if err := cmdDAGDELETE(ctx); err != nil {
		t.Fatal(err)
	}
}

func TestDeep2_ParallelCommands(t *testing.T) {
	s := store.NewStore()
	ctx := discardCtx("PARALLEL.EXEC", bytesArgs("cmd1", "cmd2"), s)
	if err := cmdPARALLELEXEC(ctx); err != nil {
		t.Fatal(err)
	}
	ctx = discardCtx("PARALLEL.MAP", bytesArgs("a", "b", "c"), s)
	if err := cmdPARALLELMAP(ctx); err != nil {
		t.Fatal(err)
	}
	ctx = discardCtx("PARALLEL.REDUCE", bytesArgs("1", "2", "3"), s)
	if err := cmdPARALLELREDUCE(ctx); err != nil {
		t.Fatal(err)
	}
	ctx = discardCtx("PARALLEL.FILTER", bytesArgs("a", "b", "c"), s)
	if err := cmdPARALLELFILTER(ctx); err != nil {
		t.Fatal(err)
	}
}

func TestDeep2_SecretCommands(t *testing.T) {
	s := store.NewStore()
	ctx := discardCtx("SECRET.SET", bytesArgs("sec1", "mysecret"), s)
	if err := cmdSECRETSET(ctx); err != nil {
		t.Fatal(err)
	}
	ctx = discardCtx("SECRET.GET", bytesArgs("sec1"), s)
	if err := cmdSECRETGET(ctx); err != nil {
		t.Fatal(err)
	}
	ctx = discardCtx("SECRET.LIST", bytesArgs(), s)
	if err := cmdSECRETLIST(ctx); err != nil {
		t.Fatal(err)
	}
	ctx = discardCtx("SECRET.ROTATE", bytesArgs("sec1", "newsecret"), s)
	if err := cmdSECRETROTATE(ctx); err != nil {
		t.Fatal(err)
	}
	ctx = discardCtx("SECRET.VERSION", bytesArgs("sec1"), s)
	if err := cmdSECRETVERSION(ctx); err != nil {
		t.Fatal(err)
	}
	ctx = discardCtx("SECRET.DELETE", bytesArgs("sec1"), s)
	if err := cmdSECRETDELETE(ctx); err != nil {
		t.Fatal(err)
	}
}

func TestDeep2_TrieCommands(t *testing.T) {
	s := store.NewStore()
	ctx := discardCtx("TRIE.ADD", bytesArgs("hello"), s)
	if err := cmdTRIEADD(ctx); err != nil {
		t.Fatal(err)
	}
	ctx = discardCtx("TRIE.ADD", bytesArgs("help"), s)
	cmdTRIEADD(ctx)
	ctx = discardCtx("TRIE.ADD", bytesArgs("world"), s)
	cmdTRIEADD(ctx)
	ctx = discardCtx("TRIE.SEARCH", bytesArgs("hello"), s)
	if err := cmdTRIESEARCH(ctx); err != nil {
		t.Fatal(err)
	}
	ctx = discardCtx("TRIE.PREFIX", bytesArgs("hel"), s)
	if err := cmdTRIEPREFIX(ctx); err != nil {
		t.Fatal(err)
	}
	ctx = discardCtx("TRIE.AUTOCOMPLETE", bytesArgs("hel"), s)
	if err := cmdTRIEAUTOCOMPLETE(ctx); err != nil {
		t.Fatal(err)
	}
	ctx = discardCtx("TRIE.DELETE", bytesArgs("hello"), s)
	if err := cmdTRIEDELETE(ctx); err != nil {
		t.Fatal(err)
	}
}

func TestDeep2_RingCommands(t *testing.T) {
	s := store.NewStore()
	ctx := discardCtx("RING.CREATE", bytesArgs("ring1"), s)
	if err := cmdRINGCREATE(ctx); err != nil {
		t.Fatal(err)
	}
	ctx = discardCtx("RING.ADD", bytesArgs("ring1", "node1"), s)
	if err := cmdRINGADD(ctx); err != nil {
		t.Fatal(err)
	}
	ctx = discardCtx("RING.ADD", bytesArgs("ring1", "node2"), s)
	cmdRINGADD(ctx)
	ctx = discardCtx("RING.GET", bytesArgs("ring1", "somekey"), s)
	if err := cmdRINGGET(ctx); err != nil {
		t.Fatal(err)
	}
	ctx = discardCtx("RING.NODES", bytesArgs("ring1"), s)
	if err := cmdRINGNODES(ctx); err != nil {
		t.Fatal(err)
	}
	ctx = discardCtx("RING.REMOVE", bytesArgs("ring1", "node1"), s)
	if err := cmdRINGREMOVE(ctx); err != nil {
		t.Fatal(err)
	}
}

func TestDeep2_SemaphoreCommands(t *testing.T) {
	s := store.NewStore()
	ctx := discardCtx("SEM.CREATE", bytesArgs("sem1", "3"), s)
	if err := cmdSEMCREATE(ctx); err != nil {
		t.Fatal(err)
	}
	ctx = discardCtx("SEM.ACQUIRE", bytesArgs("sem1"), s)
	if err := cmdSEMACQUIRE(ctx); err != nil {
		t.Fatal(err)
	}
	ctx = discardCtx("SEM.VALUE", bytesArgs("sem1"), s)
	if err := cmdSEMVALUE(ctx); err != nil {
		t.Fatal(err)
	}
	ctx = discardCtx("SEM.TRYACQUIRE", bytesArgs("sem1"), s)
	if err := cmdSEMTRYACQUIRE(ctx); err != nil {
		t.Fatal(err)
	}
	ctx = discardCtx("SEM.RELEASE", bytesArgs("sem1"), s)
	if err := cmdSEMRELEASE(ctx); err != nil {
		t.Fatal(err)
	}
}

// ======================================================================
// TRANSACTION COMMANDS (transaction_commands.go)
// ======================================================================

func TestDeep2_TransactionMultiExec(t *testing.T) {
	s := store.NewStore()
	tx := NewTransaction()

	ctx := discardCtx("MULTI", bytesArgs(), s)
	ctx.Transaction = tx
	if err := cmdMULTI(ctx); err != nil {
		t.Fatal(err)
	}

	// Queue some commands
	tx.Queue("SET", [][]byte{[]byte("txkey1"), []byte("txval1")})
	tx.Queue("GET", [][]byte{[]byte("txkey1")})
	tx.Queue("DEL", [][]byte{[]byte("txkey1")})

	ctx = discardCtx("EXEC", bytesArgs(), s)
	ctx.Transaction = tx
	if err := cmdEXEC(ctx); err != nil {
		t.Fatal(err)
	}
}

func TestDeep2_TransactionDiscard(t *testing.T) {
	s := store.NewStore()
	tx := NewTransaction()

	ctx := discardCtx("MULTI", bytesArgs(), s)
	ctx.Transaction = tx
	cmdMULTI(ctx)

	tx.Queue("SET", [][]byte{[]byte("key"), []byte("val")})

	ctx = discardCtx("DISCARD", bytesArgs(), s)
	ctx.Transaction = tx
	if err := cmdDISCARD(ctx); err != nil {
		t.Fatal(err)
	}
}

func TestDeep2_TransactionDiscardWithoutMulti(t *testing.T) {
	s := store.NewStore()
	tx := NewTransaction()
	ctx := discardCtx("DISCARD", bytesArgs(), s)
	ctx.Transaction = tx
	if err := cmdDISCARD(ctx); err != nil {
		t.Fatal(err)
	}
}

func TestDeep2_TransactionExecWithoutMulti(t *testing.T) {
	s := store.NewStore()
	tx := NewTransaction()
	ctx := discardCtx("EXEC", bytesArgs(), s)
	ctx.Transaction = tx
	if err := cmdEXEC(ctx); err != nil {
		t.Fatal(err)
	}
}

func TestDeep2_TransactionWatch(t *testing.T) {
	s := store.NewStore()
	s.Set("watchkey", &store.StringValue{Data: []byte("val")}, store.SetOptions{})
	tx := NewTransaction()

	ctx := discardCtx("WATCH", bytesArgs("watchkey"), s)
	ctx.Transaction = tx
	if err := cmdWATCH(ctx); err != nil {
		t.Fatal(err)
	}

	ctx = discardCtx("UNWATCH", bytesArgs(), s)
	ctx.Transaction = tx
	if err := cmdUNWATCH(ctx); err != nil {
		t.Fatal(err)
	}
}

func TestDeep2_TransactionWatchInsideMulti(t *testing.T) {
	s := store.NewStore()
	tx := NewTransaction()
	ctx := discardCtx("MULTI", bytesArgs(), s)
	ctx.Transaction = tx
	cmdMULTI(ctx)

	ctx = discardCtx("WATCH", bytesArgs("key"), s)
	ctx.Transaction = tx
	if err := cmdWATCH(ctx); err != nil {
		t.Fatal(err)
	}
}

func TestDeep2_TransactionExecEmptyQueue(t *testing.T) {
	s := store.NewStore()
	tx := NewTransaction()
	ctx := discardCtx("MULTI", bytesArgs(), s)
	ctx.Transaction = tx
	cmdMULTI(ctx)

	ctx = discardCtx("EXEC", bytesArgs(), s)
	ctx.Transaction = tx
	if err := cmdEXEC(ctx); err != nil {
		t.Fatal(err)
	}
}

func TestDeep2_ExecuteQueuedCommands(t *testing.T) {
	s := store.NewStore()
	tx := NewTransaction()

	ctx := discardCtx("MULTI", bytesArgs(), s)
	ctx.Transaction = tx
	cmdMULTI(ctx)

	cmds := []struct {
		cmd  string
		args [][]byte
	}{
		{"SET", [][]byte{[]byte("k1"), []byte("v1")}},
		{"GET", [][]byte{[]byte("k1")}},
		{"INCR", [][]byte{[]byte("counter")}},
		{"DECR", [][]byte{[]byte("counter")}},
		{"INCRBY", [][]byte{[]byte("counter"), []byte("5")}},
		{"DECRBY", [][]byte{[]byte("counter"), []byte("2")}},
		{"APPEND", [][]byte{[]byte("k1"), []byte("_extra")}},
		{"STRLEN", [][]byte{[]byte("k1")}},
		{"SETNX", [][]byte{[]byte("k2"), []byte("v2")}},
		{"MSET", [][]byte{[]byte("m1"), []byte("v1"), []byte("m2"), []byte("v2")}},
		{"MGET", [][]byte{[]byte("m1"), []byte("m2")}},
		{"GETSET", [][]byte{[]byte("k1"), []byte("newval")}},
		{"EXISTS", [][]byte{[]byte("k1")}},
		{"TYPE", [][]byte{[]byte("k1")}},
		{"PING", nil},
		{"ECHO", [][]byte{[]byte("hello")}},
		{"DBSIZE", nil},
		{"DEL", [][]byte{[]byte("k1")}},
		{"UNLINK", [][]byte{[]byte("m1")}},
		{"RENAME", [][]byte{[]byte("m2"), []byte("m3")}},
		{"FLUSHDB", nil},
	}

	for _, c := range cmds {
		tx.Queue(c.cmd, c.args)
	}

	ctx = discardCtx("EXEC", bytesArgs(), s)
	ctx.Transaction = tx
	if err := cmdEXEC(ctx); err != nil {
		t.Fatal(err)
	}
}

func TestDeep2_ExecuteQueuedCommandsHash(t *testing.T) {
	s := store.NewStore()
	tx := NewTransaction()
	ctx := discardCtx("MULTI", bytesArgs(), s)
	ctx.Transaction = tx
	cmdMULTI(ctx)

	tx.Queue("HSET", [][]byte{[]byte("h1"), []byte("f1"), []byte("v1")})
	tx.Queue("HGET", [][]byte{[]byte("h1"), []byte("f1")})
	tx.Queue("HEXISTS", [][]byte{[]byte("h1"), []byte("f1")})
	tx.Queue("HLEN", [][]byte{[]byte("h1")})
	tx.Queue("HDEL", [][]byte{[]byte("h1"), []byte("f1")})

	ctx = discardCtx("EXEC", bytesArgs(), s)
	ctx.Transaction = tx
	if err := cmdEXEC(ctx); err != nil {
		t.Fatal(err)
	}
}

func TestDeep2_ExecuteQueuedCommandsList(t *testing.T) {
	s := store.NewStore()
	tx := NewTransaction()
	ctx := discardCtx("MULTI", bytesArgs(), s)
	ctx.Transaction = tx
	cmdMULTI(ctx)

	tx.Queue("LPUSH", [][]byte{[]byte("list1"), []byte("a"), []byte("b")})
	tx.Queue("RPUSH", [][]byte{[]byte("list1"), []byte("c")})
	tx.Queue("LLEN", [][]byte{[]byte("list1")})
	tx.Queue("LPOP", [][]byte{[]byte("list1")})
	tx.Queue("RPOP", [][]byte{[]byte("list1")})

	ctx = discardCtx("EXEC", bytesArgs(), s)
	ctx.Transaction = tx
	if err := cmdEXEC(ctx); err != nil {
		t.Fatal(err)
	}
}

func TestDeep2_ExecuteQueuedCommandsSet(t *testing.T) {
	s := store.NewStore()
	tx := NewTransaction()
	ctx := discardCtx("MULTI", bytesArgs(), s)
	ctx.Transaction = tx
	cmdMULTI(ctx)

	tx.Queue("SADD", [][]byte{[]byte("set1"), []byte("a"), []byte("b")})
	tx.Queue("SCARD", [][]byte{[]byte("set1")})
	tx.Queue("SISMEMBER", [][]byte{[]byte("set1"), []byte("a")})
	tx.Queue("SREM", [][]byte{[]byte("set1"), []byte("a")})

	ctx = discardCtx("EXEC", bytesArgs(), s)
	ctx.Transaction = tx
	if err := cmdEXEC(ctx); err != nil {
		t.Fatal(err)
	}
}

func TestDeep2_ExecuteQueuedCommandsSortedSet(t *testing.T) {
	s := store.NewStore()
	tx := NewTransaction()
	ctx := discardCtx("MULTI", bytesArgs(), s)
	ctx.Transaction = tx
	cmdMULTI(ctx)

	tx.Queue("ZADD", [][]byte{[]byte("zset1"), []byte("1"), []byte("a"), []byte("2"), []byte("b")})
	tx.Queue("ZCARD", [][]byte{[]byte("zset1")})
	tx.Queue("ZSCORE", [][]byte{[]byte("zset1"), []byte("a")})
	tx.Queue("ZREM", [][]byte{[]byte("zset1"), []byte("a")})

	ctx = discardCtx("EXEC", bytesArgs(), s)
	ctx.Transaction = tx
	if err := cmdEXEC(ctx); err != nil {
		t.Fatal(err)
	}
}

func TestDeep2_ExecuteQueuedDefaultCase(t *testing.T) {
	s := store.NewStore()
	tx := NewTransaction()
	ctx := discardCtx("MULTI", bytesArgs(), s)
	ctx.Transaction = tx
	cmdMULTI(ctx)

	tx.Queue("UNKNOWN_CMD", [][]byte{[]byte("arg1")})

	ctx = discardCtx("EXEC", bytesArgs(), s)
	ctx.Transaction = tx
	if err := cmdEXEC(ctx); err != nil {
		t.Fatal(err)
	}
}

// ======================================================================
// Helper function
// ======================================================================

func intToStr(n int64) string {
	if n == 0 {
		return "0"
	}
	neg := false
	if n < 0 {
		neg = true
		n = -n
	}
	buf := make([]byte, 0, 20)
	for n > 0 {
		buf = append(buf, byte('0'+n%10))
		n /= 10
	}
	if neg {
		buf = append(buf, '-')
	}
	// reverse
	for i, j := 0, len(buf)-1; i < j; i, j = i+1, j-1 {
		buf[i], buf[j] = buf[j], buf[i]
	}
	return string(buf)
}
