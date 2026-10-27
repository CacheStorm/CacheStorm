package acl

import (
	"reflect"
	"testing"
)

func TestCommandRulesRenderLikeRedis(t *testing.T) {
	SetCategoryResolver(func(cat string) ([]string, bool) {
		switch cat {
		case "read":
			return []string{"GET", "HGET"}, true
		case "dangerous":
			return []string{"FLUSHDB", "CONFIG|SET"}, true
		}
		return nil, false
	})
	defer SetCategoryResolver(nil)
	for _, tc := range []struct {
		rules []string
		want  string
	}{
		{nil, "-@all"},
		{[]string{"+get", "+set", "-del"}, "-@all +get +set -del"},
		{[]string{"+@all", "-@dangerous", "+flushdb", "-get"}, "+@all -@dangerous +flushdb -get"},
		{[]string{"+config|get", "+config|set", "-config"}, "-@all -config"},
		{[]string{"+config", "-config|set"}, "-@all +config -config|set"},
		{[]string{"+@read", "-@read"}, "-@all -@read"},
		{[]string{"+GET", "+get"}, "-@all +get"},
		{[]string{"+get", "allcommands"}, "+@all"},
		{[]string{"+@all", "-get", "nocommands", "+set"}, "-@all +set"},
	} {
		u := &User{Name: "u"}
		if err := ParseACLRules(tc.rules, u); err != nil {
			t.Fatalf("%v: %v", tc.rules, err)
		}
		if got := u.Describe().Commands; got != tc.want {
			t.Errorf("%v: commands %q, want %q", tc.rules, got, tc.want)
		}
	}
}

func TestDescribeAndACLString(t *testing.T) {
	u := &User{Name: "u1"}
	if err := ParseACLRules([]string{"on", ">pw", "~k*", "&ch", "+get"}, u); err != nil {
		t.Fatal(err)
	}
	d := u.Describe()
	if !reflect.DeepEqual(d.Flags, []string{"on", "sanitize-payload"}) || d.Keys != "~k*" || d.Channels != "&ch" || len(d.Passwords) != 1 {
		t.Fatalf("describe %+v", d)
	}
	want := "user u1 on sanitize-payload #" + d.Passwords[0] + " ~k* resetchannels &ch -@all +get"
	if got := u.ToACLString(); got != want {
		t.Fatalf("ACL string %q, want %q", got, want)
	}
	if err := ParseACLRules([]string{"skip-sanitize-payload", "nopass", "allkeys", "allchannels"}, u); err != nil {
		t.Fatal(err)
	}
	if got := u.ToACLString(); got != "user u1 on nopass skip-sanitize-payload ~* &* -@all +get" {
		t.Fatalf("ACL string %q", got)
	}
	a := NewACL()
	_, _ = a.CreateUser("zed")
	_, _ = a.CreateUser("alice")
	if got := a.ListUsers(); !reflect.DeepEqual(got, []string{"alice", "default", "zed"}) {
		t.Fatalf("users %q", got)
	}
}
