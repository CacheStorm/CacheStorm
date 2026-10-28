package acl

import (
	"reflect"
	"testing"
)

func TestKeyPermissionFlags(t *testing.T) {
	u := &User{Name: "u"}
	if err := ParseACLRules([]string{"on", "+@all", "%R~r:*", "%W~w:*", "~rw:*", "%R~w:*"}, u); err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct {
		key   string
		flags uint8
		want  Denial
	}{
		{"r:1", KeyRead, Allowed},
		{"r:1", KeyWrite, DeniedKey},
		{"r:1", KeyReadWrite, DeniedKey},
		{"w:1", KeyReadWrite, Allowed},
		{"rw:1", KeyReadWrite, Allowed},
		{"other", KeyRead, DeniedKey},
	} {
		got, item := u.Check("GET", "", func() []KeyRef { return []KeyRef{{Name: tc.key, Flags: tc.flags}} }, nil)
		if got != tc.want || (got == DeniedKey && item != tc.key) {
			t.Errorf("%s flags %d: %v %q, want %v", tc.key, tc.flags, got, item, tc.want)
		}
	}
	if got := u.Describe().Keys; got != "%R~r:* ~w:* ~rw:*" {
		t.Fatalf("keys %q", got)
	}
}

func TestSelectorsAllowWhatRootDenies(t *testing.T) {
	SetCategoryResolver(func(string) ([]string, bool) { return nil, false })
	defer SetCategoryResolver(nil)
	u := &User{Name: "s"}
	rules := []string{"on", "nopass", "+get", "~a:*", "(+set", "~b:*)", "(+del %W~c:*)"}
	if err := ParseACLRules(rules, u); err != nil {
		t.Fatal(err)
	}
	check := func(cmd, key string, flags uint8) (Denial, string) {
		return u.Check(cmd, "", func() []KeyRef { return []KeyRef{{Name: key, Flags: flags}} }, nil)
	}
	if d, _ := check("GET", "a:1", KeyRead); d != Allowed {
		t.Fatal("root selector refused GET a:1")
	}
	if d, _ := check("SET", "b:1", KeyWrite); d != Allowed {
		t.Fatal("selector refused SET b:1")
	}
	if d, item := check("SET", "a:1", KeyWrite); d != DeniedKey || item != "a:1" {
		t.Fatalf("SET a:1 = %v %q, want a key denial", d, item)
	}
	if d, _ := check("GET", "b:1", KeyRead); d != DeniedKey {
		t.Fatalf("GET b:1 = %v, want a key denial", d)
	}
	if d, _ := check("DEL", "c:1", KeyWrite); d != Allowed {
		t.Fatal("selector refused DEL c:1")
	}
	if d, _ := check("HSET", "a:1", KeyWrite); d != DeniedCommand {
		t.Fatalf("HSET = %v, want a command denial", d)
	}
	want := "user s on nopass sanitize-payload ~a:* resetchannels -@all +get (~b:* resetchannels -@all +set) (%W~c:* resetchannels -@all +del)"
	if got := u.ToACLString(); got != want {
		t.Fatalf("ACL string\n%q\nwant\n%q", got, want)
	}
	d := u.Describe()
	if !reflect.DeepEqual(d.Selectors, []SelectorDescription{{"-@all +set", "~b:*", ""}, {"-@all +del", "%W~c:*", ""}}) {
		t.Fatalf("selectors %+v", d.Selectors)
	}
	if err := ParseACLRules([]string{"clearselectors"}, u); err != nil || len(u.Selectors) != 0 {
		t.Fatalf("clearselectors: %v, %d left", err, len(u.Selectors))
	}
}

func TestStatefulRuleErrorsChangeNothing(t *testing.T) {
	u := &User{Name: "e"}
	if err := ParseACLRules([]string{"allkeys", "allchannels"}, u); err != nil {
		t.Fatal(err)
	}
	for _, rules := range [][]string{{"on", "~x"}, {"on", "&x"}, {"on", "<nope"}} {
		if err := ParseACLRules(rules, u); err == nil {
			t.Errorf("%v was accepted", rules)
		}
	}
	if u.IsEnabled() {
		t.Fatal("a failed SETUSER changed the user")
	}
	if _, err := MergeSelectorArgs([]string{"(+get", "~a"}); err == nil || err.Error() != "ERR Unmatched parenthesis in acl selector starting at '(+get'." {
		t.Fatalf("unmatched: %v", err)
	}
}

func TestPatternSubscriptionMustMatchLiterally(t *testing.T) {
	u := &User{Name: "p"}
	if err := ParseACLRules([]string{"on", "+@all", "&news:*"}, u); err != nil {
		t.Fatal(err)
	}
	chans := func(name string, pattern bool) func() []ChannelRef {
		return func() []ChannelRef { return []ChannelRef{{Name: name, Pattern: pattern}} }
	}
	if d, _ := u.Check("SUBSCRIBE", "", nil, chans("news:1", false)); d != Allowed {
		t.Fatal("SUBSCRIBE news:1 refused")
	}
	if d, _ := u.Check("PSUBSCRIBE", "", nil, chans("news:*", true)); d != Allowed {
		t.Fatal("PSUBSCRIBE news:* refused")
	}
	if d, _ := u.Check("PSUBSCRIBE", "", nil, chans("news:1*", true)); d != DeniedChannel {
		t.Fatalf("PSUBSCRIBE news:1* = %v, want a channel denial", d)
	}
}
