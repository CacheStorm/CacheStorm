package server

import (
	"strings"
	"testing"

	"github.com/cachestorm/cachestorm/internal/resp"
)

func TestSetACLTransportReadIsolationX(t *testing.T) {
	s := startReplServer(t, nil)
	admin := dialRepl(t, s)
	for key, member := range map[string]string{"user:s": "public", "secret": "classified", "user:out": "public"} {
		if v := admin.do("SADD", key, member); v.Type != resp.TypeInteger {
			t.Fatal(v)
		}
	}
	user := "read_isolation_set_user"
	if v := admin.do("ACL", "SETUSER", user, "on", ">pw", "-@all", "+sunion", "+sinter", "+sdiff", "+sunionstore", "+sinterstore", "+sdiffstore", "+smembers", "+sintercard", "~user:*"); !aclIsOK(v) {
		t.Fatal(v)
	}
	t.Cleanup(func() { admin.do("ACL", "DELUSER", user) })
	c := dialRepl(t, s)
	if v := c.do("AUTH", user, "pw"); !aclIsOK(v) {
		t.Fatal(v)
	}
	for _, name := range []string{"SUNION", "SINTER", "SDIFF", "SUNIONSTORE", "SINTERSTORE", "SDIFFSTORE"} {
		control := []string{name, "user:s"}
		denied := []string{name, "secret"}
		if strings.HasSuffix(name, "STORE") {
			control = []string{name, "user:out", "user:s"}
			denied = []string{name, "user:out", "secret"}
		}
		if v := c.do(control...); v.Type == resp.TypeError {
			t.Fatalf("authorized %s=%+v", name, v)
		}
		if v := c.do(denied...); !aclIsNoperm(v) {
			t.Fatalf("unauthorized %s=%+v", name, v)
		}
		if strings.HasSuffix(name, "STORE") {
			if v := c.do(name, "secret", "user:s"); !aclIsNoperm(v) {
				t.Fatalf("forbidden destination %s=%+v", name, v)
			}
		}
	}
	if v := c.do("SINTERCARD", "1", "user:s"); v.Type != resp.TypeInteger || v.Int != 1 {
		t.Fatalf("counted control=%+v", v)
	}
	if v := c.do("SINTERCARD", "1", "secret"); !aclIsNoperm(v) {
		t.Fatal(v)
	}
	if v := admin.do("SMEMBERS", "secret"); v.Type != resp.TypeArray || len(v.Array) != 1 || string(v.Array[0].Bulk) != "classified" {
		t.Fatalf("secret changed: %+v", v)
	}
	if v := c.do("SMEMBERS", "user:out"); v.Type != resp.TypeArray || len(v.Array) != 1 || string(v.Array[0].Bulk) != "public" {
		t.Fatalf("output changed: %+v", v)
	}
	for _, key := range []string{"0", "2"} {
		if v := admin.do("SADD", key, "m"); v.Type != resp.TypeInteger {
			t.Fatal(v)
		}
		if v := c.do("SUNION", key); !aclIsNoperm(v) {
			t.Fatalf("numeric key bypassed permissions: %+v", v)
		}
	}
}
