package server

import (
	"testing"

	"github.com/cachestorm/cachestorm/internal/resp"
)

// Connection-level coverage for the MULTI-KEY ACL branches.
//
// Rounds 31-35 ran a mutation sweep over internal/command/acl_enforcement.go and
// every key-extraction mutant was killed -- but all of them were killed by the
// UNIT tests in internal/command. Mutating aclKeysForCommand's RENAME branch to
// drop the destination left this whole package green, because no test here ever
// issued a RENAME. The unit tests assert argument SHAPE; these assert that a
// real client request actually reaches enforcement.
//
// Every case pairs a forbidden-key assertion with a same-command control on an
// allowed key, so a broken harness (bad credentials, an unregistered command)
// cannot make the NOPERM expectation pass vacuously.
//
// NOTE on "was the value left alone?" assertions: a key outside ~user:* cannot
// be read back over this connection at all -- GET on it returns NOPERM, which is
// the correct behaviour. So such an assertion is made only on a key the user IS
// allowed to read; for the forbidden key the NOPERM reply is the contract.
const aclMultiKeyUser = "acl_multikey_user"

// newACLMultiKeySession returns a session already authenticated as a user
// confined to ~user:* and granted the multi-key commands under test.
func newACLMultiKeySession(t *testing.T) *aclSession {
	t.Helper()
	a := newACLSession(t)

	if v := a.do("SET", "user:src", "v1"); !aclIsOK(v) {
		t.Fatalf("setup: SET user:src returned type %v %q, want +OK", v.Type, v.Str)
	}
	if v := a.do("SET", "other:1", "topsecret"); !aclIsOK(v) {
		t.Fatalf("setup: SET other:1 returned type %v %q, want +OK", v.Type, v.Str)
	}
	// Seed an out-of-pattern ZSET as the DEFAULT user, before AUTH: once the
	// restricted user is installed it could never create this itself (ZADD
	// would be NOPERM). Tests then point ZUNIONSTORE/ZUNION at it, so the only
	// possible outcomes are NOPERM (ACL works) or success (ACL bypassed).
	if v := a.do("ZADD", "other:zs", "1", "x"); v.Type != resp.TypeInteger {
		t.Fatalf("setup: ZADD other:zs returned type %v %q, want an integer", v.Type, v.Str)
	}
	// Out-of-pattern keys a SORT BY/GET pattern would expand to. Seeded as the
	// DEFAULT user because the restricted user could never create them.
	if v := a.do("SET", "other:s_alpha", "TOP-SECRET"); !aclIsOK(v) {
		t.Fatalf("setup: SET other:s_alpha returned type %v %q, want +OK", v.Type, v.Str)
	}
	if v := a.do("SET", "user:d_alpha", "visible"); !aclIsOK(v) {
		t.Fatalf("setup: SET user:d_alpha returned type %v %q, want +OK", v.Type, v.Str)
	}
	// An out-of-pattern GEO set, seeded as the DEFAULT user: GEOSEARCHSTORE
	// reads its source key, so a ~user:* user must not be able to reach it.
	if v := a.do("GEOADD", "other:geo", "13.361389", "38.115556", "Catania"); v.Type != resp.TypeInteger {
		t.Fatalf("setup: GEOADD other:geo returned type %v %q, want an integer", v.Type, v.Str)
	}

	// The commands are GRANTED; only the key pattern is restrictive. That is
	// what makes each case test key extraction rather than command permission.
	if v := a.do("ACL", "SETUSER", aclMultiKeyUser, "reset", "on", ">pw", "-@all",
		"+get", "+set", "+rename", "+renamenx", "+copy",
		"+mset", "+mget", "+sort", "+bitop", "+del", "+rpush",
		"+zadd", "+zunionstore", "+zinterstore", "+zunion", "+zinter",
		"+geoadd", "+georadiusbymember", "+geosearchstore",
		"+sadd", "+sinter", "+sintercard", "+sdiff", "+sunion", "+sdiffstore",
		"+sinterstore", "+sunionstore",
		"~user:*"); !aclIsOK(v) {
		t.Fatalf("setup: ACL SETUSER returned type %v %q, want +OK", v.Type, v.Str)
	}
	if v := a.do("AUTH", aclMultiKeyUser, "pw"); !aclIsOK(v) {
		t.Fatalf("AUTH returned type %v %q, want +OK", v.Type, v.Str)
	}
	return a
}

// aclIsNullBulk reports a nil reply ($-1), which is what GET returns for a key
// that does not exist. It is a null BULK STRING, not resp.TypeNull.
func aclIsNullBulk(v *resp.Value) bool {
	return v.Type == resp.TypeBulkString && v.IsNull
}

// RENAME must check BOTH sides. Checking only the source lets a ~user:* user
// rename a key OUT of the sandbox, which is the exact attack the branch's own
// comment warns about.
func TestACLRenameChecksDestinationKey(t *testing.T) {
	a := newACLMultiKeySession(t)

	// Control: both endpoints inside the pattern must still work.
	if v := a.do("RENAME", "user:src", "user:dst"); !aclIsOK(v) {
		t.Fatalf("control: RENAME between two allowed keys returned type %v %q, want +OK", v.Type, v.Str)
	}

	// The destination escapes the sandbox.
	if v := a.do("RENAME", "user:src", "other:1"); !aclIsNoperm(v) {
		t.Fatalf("RENAME user:src other:1 returned type %v %q, want NOPERM -- only the source key is checked", v.Type, v.Err)
	}

	// The SOURCE escapes the sandbox (the other direction).
	if v := a.do("RENAME", "other:1", "user:dst"); !aclIsNoperm(v) {
		t.Fatalf("RENAME other:1 user:dst returned type %v %q, want NOPERM", v.Type, v.Err)
	}
}

// RENAMENX shares the same extraction branch as RENAME.
func TestACLRenameNXChecksBothKeys(t *testing.T) {
	a := newACLMultiKeySession(t)

	if v := a.do("RENAMENX", "user:src", "user:dst2"); v.Type != resp.TypeInteger {
		t.Fatalf("control: RENAMENX between two allowed keys returned type %v, want an integer", v.Type)
	}
	if v := a.do("RENAMENX", "user:src", "other:1"); !aclIsNoperm(v) {
		t.Fatalf("RENAMENX user:src other:1 returned type %v %q, want NOPERM", v.Type, v.Err)
	}
}

// MSET is an alternating key/value command: only EVEN indices are keys. If the
// extractor treated every argument as a key it would refuse a legitimate MSET,
// so the control here proves the stride as well as the restriction.
func TestACLMSETChecksEveryKeyNotItsValues(t *testing.T) {
	a := newACLMultiKeySession(t)

	// Control: values are NOT keys, so this must not be refused.
	if v := a.do("MSET", "user:a", "va", "user:b", "vb"); !aclIsOK(v) {
		t.Fatalf("control: MSET of two allowed keys returned type %v %q, want +OK -- values are being treated as keys", v.Type, v.Str)
	}
	if v := a.do("GET", "user:a"); string(v.Bulk) != "va" {
		t.Fatalf("control: MSET did not store user:a (got %q, want \"va\")", v.Bulk)
	}

	// One key outside the pattern must refuse the WHOLE command.
	if v := a.do("MSET", "user:c", "vc", "other:1", "bad"); !aclIsNoperm(v) {
		t.Fatalf("MSET containing other:1 returned type %v %q, want NOPERM", v.Type, v.Err)
	}

	// The refused MSET must not have written its allowed half (user:c is
	// readable by this user, so atomicity is observable here).
	if v := a.do("GET", "user:c"); !aclIsNullBulk(v) {
		t.Fatalf("after a refused MSET, GET user:c returned type %v %q, want a null (the command must be atomic)", v.Type, v.Bulk)
	}
}

// MGET is an all-args command: every argument is a key, so a single forbidden
// key must refuse the whole read.
func TestACLMGETChecksEveryKey(t *testing.T) {
	a := newACLMultiKeySession(t)

	if v := a.do("MSET", "user:m1", "one", "user:m2", "two"); !aclIsOK(v) {
		t.Fatalf("setup: MSET returned type %v %q, want +OK", v.Type, v.Str)
	}
	if v := a.do("MGET", "user:m1", "user:m2"); v.Type != resp.TypeArray {
		t.Fatalf("control: MGET of two allowed keys returned type %v, want an array", v.Type)
	}
	if v := a.do("MGET", "user:m1", "other:1"); !aclIsNoperm(v) {
		t.Fatalf("MGET including other:1 returned type %v %q, want NOPERM -- not every argument is checked", v.Type, v.Err)
	}
}

// SORT writes to a STORE destination that is NOT args[0]. Dropping
// storeKeyAfter would let a sandboxed user write its result outside the pattern.
func TestACLSortStoreDestinationIsChecked(t *testing.T) {
	a := newACLMultiKeySession(t)

	// RPUSH replies the resulting list length, an integer -- not +OK.
	if v := a.do("RPUSH", "user:list", "3", "1", "2"); v.Type != resp.TypeInteger {
		t.Fatalf("setup: RPUSH returned type %v %q, want an integer", v.Type, v.Str)
	}

	// Control: a STORE destination inside the pattern must still work.
	// SORT ... STORE replies the stored list length, an integer -- not +OK.
	if v := a.do("SORT", "user:list", "STORE", "user:sorted"); v.Type != resp.TypeInteger {
		t.Fatalf("control: SORT ... STORE user:sorted returned type %v %q, want an integer", v.Type, v.Str)
	}
	if v := a.do("GET", "user:sorted"); aclIsNullBulk(v) {
		t.Fatal("control: SORT ... STORE did not create user:sorted")
	}

	// The STORE destination escapes the sandbox.
	if v := a.do("SORT", "user:list", "STORE", "other:1"); !aclIsNoperm(v) {
		t.Fatalf("SORT ... STORE other:1 returned type %v %q, want NOPERM -- the STORE destination is not checked", v.Type, v.Err)
	}
}

// BITOP's keys start at args[2]: args[0] is the operation and args[1] is the
// destination. Checking from the wrong index admits the operation word or skips
// the destination.
func TestACLBitOpSkipsOperationAndChecksDestination(t *testing.T) {
	a := newACLMultiKeySession(t)

	if v := a.do("SET", "user:b1", "aa"); !aclIsOK(v) {
		t.Fatalf("setup: SET user:b1 returned type %v %q, want +OK", v.Type, v.Str)
	}
	if v := a.do("SET", "user:b2", "bb"); !aclIsOK(v) {
		t.Fatalf("setup: SET user:b2 returned type %v %q, want +OK", v.Type, v.Str)
	}

	// Control: every key inside the pattern must still work. BITOP replies the
	// result length, an integer -- not +OK.
	if v := a.do("BITOP", "AND", "user:bdest", "user:b1", "user:b2"); v.Type != resp.TypeInteger {
		t.Fatalf("control: BITOP over allowed keys returned type %v %q, want an integer", v.Type, v.Str)
	}

	// A forbidden SOURCE operand must refuse the command.
	if v := a.do("BITOP", "AND", "user:bdest2", "user:b1", "other:1"); !aclIsNoperm(v) {
		t.Fatalf("BITOP with source other:1 returned type %v %q, want NOPERM", v.Type, v.Err)
	}

	// A forbidden DESTINATION must refuse the command too.
	if v := a.do("BITOP", "AND", "other:1", "user:b1", "user:b2"); !aclIsNoperm(v) {
		t.Fatalf("BITOP with destination other:1 returned type %v %q, want NOPERM", v.Type, v.Err)
	}
}

// ZUNIONSTORE writes to its DESTINATION key, which the command forms as
//
//	ZUNIONSTORE destination numkeys key [key ...]
//
// so args[0] is a key that must be permission-checked. Returning only the
// NUMKEYS-counted sources would let a ~user:* user compute into any key.
func TestACLZUnionStoreChecksDestinationKey(t *testing.T) {
	a := newACLMultiKeySession(t)

	if v := a.do("ZADD", "user:z1", "1", "a"); v.Type != resp.TypeInteger {
		t.Fatalf("setup: ZADD returned type %v %q, want an integer", v.Type, v.Str)
	}
	if v := a.do("ZADD", "user:z2", "1", "b"); v.Type != resp.TypeInteger {
		t.Fatalf("setup: ZADD returned type %v %q, want an integer", v.Type, v.Str)
	}
	// Control: destination inside the pattern must still work.
	if v := a.do("ZUNIONSTORE", "user:zdest", "2", "user:z1", "user:z2"); v.Type != resp.TypeInteger {
		t.Fatalf("control: ZUNIONSTORE to an allowed destination returned type %v %q, want an integer", v.Type, v.Str)
	}

	// The destination escapes the sandbox.
	if v := a.do("ZUNIONSTORE", "other:zs", "2", "user:z1", "user:z2"); !aclIsNoperm(v) {
		t.Fatalf("ZUNIONSTORE into other:zs returned type %v %q, want NOPERM -- the destination key is not checked", v.Type, v.Err)
	}
}

// The READ-ONLY variants take NO destination:
//
//	ZUNION numkeys key [key ...]
//
// so numkeys sits at args[0] and the keys follow it. Reading the count from the
// wrong index fails to parse and yields no keys at all, which would let a
// ~user:* user read any sorted set outside the sandbox.
func TestACLZUnionChecksEverySourceKey(t *testing.T) {
	a := newACLMultiKeySession(t)

	if v := a.do("ZADD", "user:z1", "1", "a"); v.Type != resp.TypeInteger {
		t.Fatalf("setup: ZADD returned type %v %q, want an integer", v.Type, v.Str)
	}
	if v := a.do("ZADD", "user:z2", "1", "b"); v.Type != resp.TypeInteger {
		t.Fatalf("setup: ZADD returned type %v %q, want an integer", v.Type, v.Str)
	}

	// Control: both sources inside the pattern must work.
	if v := a.do("ZUNION", "2", "user:z1", "user:z2"); v.Type != resp.TypeArray {
		t.Fatalf("control: ZUNION over allowed keys returned type %v %q, want an array", v.Type, v.Str)
	}

	// A forbidden SOURCE must refuse the read.
	if v := a.do("ZUNION", "2", "user:z1", "other:1"); !aclIsNoperm(v) {
		t.Fatalf("ZUNION including other:1 returned type %v %q, want NOPERM -- source keys are not checked", v.Type, v.Err)
	}

	// ZINTER takes the same shape.
	if v := a.do("ZINTER", "2", "user:z1", "other:1"); !aclIsNoperm(v) {
		t.Fatalf("ZINTER including other:1 returned type %v %q, want NOPERM", v.Type, v.Err)
	}
}

// GEORADIUSBYMEMBER accepts TWO destination keywords: STORE and STOREDIST, and
// writes to both. storeKeyAfter matched only the literal "STORE", so a STOREDIST
// destination was never permission-checked -- a ~user:* user could write the
// distance set into any key. (GEORADIUSSTORE is the Redis spelling of this
// write path but is NOT a registered command here; GEORADIUSBYMEMBER is.)
func TestACLGeoRadiusStoreChecksStoreDistDestination(t *testing.T) {
	a := newACLMultiKeySession(t)

	if v := a.do("GEOADD", "user:geo", "13.361389", "38.115556", "Palermo"); v.Type != resp.TypeInteger {
		t.Fatalf("setup: GEOADD returned type %v %q, want an integer", v.Type, v.Str)
	}

	// Control: a STORE destination inside the pattern must still work.
	if v := a.do("GEORADIUSBYMEMBER", "user:geo", "Palermo", "200", "km", "ASC",
		"STORE", "user:gstore"); v.Type != resp.TypeInteger {
		t.Fatalf("control: GEORADIUSBYMEMBER ... STORE returned type %v %q / err %q, want an integer", v.Type, v.Str, v.Err)
	}

	// A STOREDIST destination inside the pattern.
	if v := a.do("GEORADIUSBYMEMBER", "user:geo", "Palermo", "200", "km", "ASC",
		"STOREDIST", "user:gsdist"); v.Type != resp.TypeInteger {
		t.Fatalf("control: GEORADIUSBYMEMBER ... STOREDIST returned type %v %q / err %q, want an integer", v.Type, v.Str, v.Err)
	}

	// The STOREDIST destination escapes the sandbox.
	if v := a.do("GEORADIUSBYMEMBER", "user:geo", "Palermo", "200", "km", "ASC",
		"STOREDIST", "other:zs"); !aclIsNoperm(v) {
		t.Fatalf("GEORADIUSBYMEMBER ... STOREDIST other:zs returned type %v %q, want NOPERM -- the STOREDIST destination is not checked", v.Type, v.Err)
	}

	// The STORE destination escapes the sandbox.
	if v := a.do("GEORADIUSBYMEMBER", "user:geo", "Palermo", "200", "km", "ASC",
		"STORE", "other:zs"); !aclIsNoperm(v) {
		t.Fatalf("GEORADIUSBYMEMBER ... STORE other:zs returned type %v %q, want NOPERM", v.Type, v.Err)
	}
}

// SORT's BY and GET options name KEYS OUTSIDE args[0]: sortExpandPattern turns
// "other:s_*" plus the element into "other:s_alpha", and sortLookupValue then
// does ctx.Store.Get on it. aclKeysForCommand returned only args[0] and the
// STORE destination, so those lookups were never permission-checked and a
// ~user:* user could read any key's value through a SORT.
func TestACLSortByAndGetPatternsAreChecked(t *testing.T) {
	a := newACLMultiKeySession(t)

	if v := a.do("RPUSH", "user:list", "alpha"); v.Type != resp.TypeInteger {
		t.Fatalf("setup: RPUSH returned type %v %q, want an integer", v.Type, v.Str)
	}

	// Control: BY nosort references no external key and must still work.
	if v := a.do("SORT", "user:list", "BY", "nosort"); v.Type != resp.TypeArray {
		t.Fatalf("control: SORT ... BY nosort returned type %v %q, want an array", v.Type, v.Str)
	}

	// Control: GET # is the element itself, not a key.
	if v := a.do("SORT", "user:list", "GET", "#"); v.Type != resp.TypeArray {
		t.Fatalf("control: SORT ... GET # returned type %v %q, want an array", v.Type, v.Str)
	}

	// Control: a BY pattern expanding INSIDE the pattern must work.
	if v := a.do("SORT", "user:list", "BY", "user:d_*"); v.Type != resp.TypeArray {
		t.Fatalf("control: SORT ... BY user:d_* returned type %v %q, want an array", v.Type, v.Str)
	}

	// A GET pattern expanding OUTSIDE the sandbox must be refused.
	if v := a.do("SORT", "user:list", "GET", "other:s_*"); !aclIsNoperm(v) {
		t.Fatalf("SORT ... GET other:s_* returned type %v %q, want NOPERM -- the GET pattern's key is not checked", v.Type, v.Err)
	}

	// A BY pattern expanding OUTSIDE the sandbox must be refused.
	if v := a.do("SORT", "user:list", "BY", "other:s_*"); !aclIsNoperm(v) {
		t.Fatalf("SORT ... BY other:s_* returned type %v %q, want NOPERM -- the BY pattern's key is not checked", v.Type, v.Err)
	}
}

// GEOSEARCHSTORE takes TWO keys:
//
//	GEOSEACHSTORE destination source <FROMMEMBER m | FROMLONLAT lon lat> ...
//
// aclKeysForCommand had no entry for it, so it fell through to the fail-closed
// default and returned args[0] -- the destination only. The SOURCE at args[1]
// was therefore never permission-checked, letting a ~user:* user read any geo
// set outside the sandbox and store the result inside it.
func TestACLGeoSearchStoreChecksSourceKey(t *testing.T) {
	a := newACLMultiKeySession(t)

	if v := a.do("GEOADD", "user:geo", "13.361389", "38.115556", "Palermo"); v.Type != resp.TypeInteger {
		t.Fatalf("setup: GEOADD user:geo returned type %v %q, want an integer", v.Type, v.Str)
	}
	// other:geo is seeded by newACLMultiKeySession as the DEFAULT user -- the
	// restricted user could never create it.

	// Control: both keys inside the pattern must work.
	if v := a.do("GEOSEARCHSTORE", "user:gdst", "user:geo",
		"FROMLONLAT", "15", "37", "BYRADIUS", "200", "km", "ASC"); v.Type != resp.TypeInteger {
		t.Fatalf("control: GEOSEARCHSTORE with both keys allowed returned type %v %q / err %q, want an integer", v.Type, v.Str, v.Err)
	}

	// The SOURCE escapes the sandbox. The destination is inside it, so the
	// default fail-closed args[0] check passes and only the source decides.
	if v := a.do("GEOSEARCHSTORE", "user:gdst2", "other:geo",
		"FROMLONLAT", "15", "37", "BYRADIUS", "200", "km", "ASC"); !aclIsNoperm(v) {
		t.Fatalf("GEOSEARCHSTORE from other:geo returned type %v %q, want NOPERM -- the source key is not checked", v.Type, v.Err)
	}
}

// Redis's set-algebra key specs: SINTERCARD is the only NUMKEYS-counted one.
//
//	SINTER  key [key ...]
//	SDIFF   key [key ...]
//	SUNION  key [key ...]
//	SINTERCARD numkeys key [key ...]
//	SDIFFSTORE destination key [key ...]
//
// Every real key must be permission-checked, and SINTERCARD's count must not be
// mistaken for a key.
func TestACLSetAlgebraChecksRealKeys(t *testing.T) {
	a := newACLMultiKeySession(t)

	if v := a.do("SADD", "user:s1", "a"); v.Type != resp.TypeInteger {
		t.Fatalf("setup: SADD returned type %v %q, want an integer", v.Type, v.Str)
	}
	if v := a.do("SADD", "user:s2", "a"); v.Type != resp.TypeInteger {
		t.Fatalf("setup: SADD returned type %v %q, want an integer", v.Type, v.Str)
	}

	// Every key below is INSIDE ~user:*, so these must be allowed.
	cases := []struct {
		name string
		args []string
	}{
		{"SINTER", []string{"SINTER", "user:s1", "user:s2"}},
		{"SINTERCARD", []string{"SINTERCARD", "2", "user:s1", "user:s2"}},
		{"SDIFF", []string{"SDIFF", "user:s1", "user:s2"}},
		{"SUNION", []string{"SUNION", "user:s1", "user:s2"}},
		{"SDIFFSTORE", []string{"SDIFFSTORE", "user:sdst", "user:s1", "user:s2"}},
	}
	for _, c := range cases {
		v := a.do(c.args...)
		if aclIsNoperm(v) {
			t.Errorf("%s over two ALLOWED keys returned NOPERM -- the command is unusable under a key pattern", c.name)
		}
	}
}

// The other half of the contract: the SAME commands must still be REFUSED when
// one of their keys is outside ~user:*. The allow-path cases above would still
// pass if a regression returned the keys only when they were all permitted, so
// these pin the enforcement itself.
//
// each case supplies the arguments with the last source key swapped for a key
// outside the pattern. SDIFFSTORE additionally gets a destination-outside case,
// because its destination is a separate code path from its sources.
func TestACLSetAlgebraRefusesKeysOutsidePattern(t *testing.T) {
	a := newACLMultiKeySession(t)

	if v := a.do("SADD", "user:s1", "a"); v.Type != resp.TypeInteger {
		t.Fatalf("setup: SADD returned type %v %q, want an integer", v.Type, v.Str)
	}
	// other:zs already exists (seeded by the shared harness); it is outside the
	// pattern, so naming it must be refused rather than silently allowed.

	cases := []struct {
		name string
		args []string
	}{
		{"SINTER", []string{"SINTER", "user:s1", "other:zs"}},
		{"SINTERCARD", []string{"SINTERCARD", "2", "user:s1", "other:zs"}},
		{"SDIFF", []string{"SDIFF", "user:s1", "other:zs"}},
		{"SUNION", []string{"SUNION", "user:s1", "other:zs"}},
		{"SDIFFSTORE (source outside)", []string{"SDIFFSTORE", "user:sdst2", "user:s1", "other:zs"}},
		{"SDIFFSTORE (destination outside)", []string{"SDIFFSTORE", "other:zs", "user:s1", "user:s1"}},
	}
	for _, c := range cases {
		v := a.do(c.args...)
		if !aclIsNoperm(v) {
			t.Errorf("%s naming a key outside ~user:* returned type %v %q, want NOPERM -- a forbidden key is not enforced",
				c.name, v.Type, v.Err)
		}
	}
}

// SINTERSTORE and SUNIONSTORE share SDIFFSTORE's shape:
//
//	SINTERSTORE destination key [key ...]
//	SUNIONSTORE  destination key [key ...]
//
// Both the destination and every source must be checked.
func TestACLSetStoreAllowsSandboxedKeysAndRefusesOthers(t *testing.T) {
	a := newACLMultiKeySession(t)

	if v := a.do("SADD", "user:s1", "a"); v.Type != resp.TypeInteger {
		t.Fatalf("setup: SADD returned type %v %q, want an integer", v.Type, v.Str)
	}
	if v := a.do("SADD", "user:s2", "a"); v.Type != resp.TypeInteger {
		t.Fatalf("setup: SADD returned type %v %q, want an integer", v.Type, v.Str)
	}

	// ALLOW: destination and both sources inside ~user:*.
	allow := []struct {
		name string
		args []string
	}{
		{"SINTERSTORE", []string{"SINTERSTORE", "user:is", "user:s1", "user:s2"}},
		{"SUNIONSTORE", []string{"SUNIONSTORE", "user:us", "user:s1", "user:s2"}},
	}
	for _, c := range allow {
		if v := a.do(c.args...); aclIsNoperm(v) {
			t.Errorf("%s over keys INSIDE ~user:* returned NOPERM -- a permitted key was refused", c.name)
		}
	}

	// REFUSE: a source outside the pattern.
	refuseSource := []struct {
		name string
		args []string
	}{
		{"SINTERSTORE", []string{"SINTERSTORE", "user:is2", "user:s1", "other:zs"}},
		{"SUNIONSTORE", []string{"SUNIONSTORE", "user:us2", "user:s1", "other:zs"}},
	}
	for _, c := range refuseSource {
		if v := a.do(c.args...); !aclIsNoperm(v) {
			t.Errorf("%s with a source outside ~user:* returned type %v %q, want NOPERM", c.name, v.Type, v.Err)
		}
	}

	// REFUSE: the DESTINATION outside the pattern — a separate code path from
	// the sources, so it needs its own case.
	refuseDest := []struct {
		name string
		args []string
	}{
		{"SINTERSTORE", []string{"SINTERSTORE", "other:zs", "user:s1", "user:s1"}},
		{"SUNIONSTORE", []string{"SUNIONSTORE", "other:zs", "user:s1", "user:s1"}},
	}
	for _, c := range refuseDest {
		if v := a.do(c.args...); !aclIsNoperm(v) {
			t.Errorf("%s writing to a destination outside ~user:* returned type %v %q, want NOPERM", c.name, v.Type, v.Err)
		}
	}
}

// DEL is an all-args command: deleting a key outside the pattern must be
// refused even though the other arguments are allowed.
func TestACLDelChecksEveryKey(t *testing.T) {
	a := newACLMultiKeySession(t)

	if v := a.do("SET", "user:d1", "v"); !aclIsOK(v) {
		t.Fatalf("setup: SET user:d1 returned type %v %q, want +OK", v.Type, v.Str)
	}
	if v := a.do("SET", "user:d2", "v2"); !aclIsOK(v) {
		t.Fatalf("setup: SET user:d2 returned type %v %q, want +OK", v.Type, v.Str)
	}
	if v := a.do("DEL", "user:d1"); v.Type != resp.TypeInteger {
		t.Fatalf("control: DEL of an allowed key returned type %v, want an integer", v.Type)
	}
	if v := a.do("DEL", "user:d2", "other:1"); !aclIsNoperm(v) {
		t.Fatalf("DEL including other:1 returned type %v %q, want NOPERM", v.Type, v.Err)
	}
	// user:d2 is readable by this user: the refused DEL must not have removed it.
	if v := a.do("GET", "user:d2"); string(v.Bulk) != "v2" {
		t.Fatalf("after a refused DEL, GET user:d2 = %q, want \"v2\" (the command must be atomic)", v.Bulk)
	}
}
