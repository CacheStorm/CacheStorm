package acl

import (
	"strings"
	"testing"
)

// `ACL SETUSER default -<command>` must apply the deny rule. NewACL is the
// constructor behind the command layer's globalACL, and ParseACLRule is the
// function cmdACL hands the rule to. The default user is the only user whose
// permission maps are built outside CreateUser, so a specific-command deny
// used to write into a nil DeniedCommands map and panic.
func TestParseACLRuleDenyCommandOnDefaultUser(t *testing.T) {
	a := NewACL()

	if err := ParseACLRule("-get", a.DefaultUser); err != nil {
		t.Fatalf("ParseACLRule(-get) on default user: %v", err)
	}
	if a.DefaultUser.CanExecuteCommand("GET") {
		t.Fatal("GET should be denied for the default user after -get")
	}
	if got := a.DefaultUser.ToACLString(); !strings.Contains(got, "-GET") {
		t.Fatalf("ToACLString should report the -GET deny, got %q", got)
	}
	// The deny is specific: an unrelated command stays allowed.
	if !a.DefaultUser.CanExecuteCommand("SET") {
		t.Fatal("SET should remain allowed for the default user")
	}
}

// Control: a user created by CreateUser already carries a DeniedCommands map,
// so the same rule was always fine there. The defect was the default user's
// construction, not the rule parser.
func TestParseACLRuleDenyCommandOnCreatedUser(t *testing.T) {
	a := NewACL()

	user, err := a.CreateUser("alice")
	if err != nil {
		t.Fatalf("CreateUser: %v", err)
	}
	if err := ParseACLRule("+get -get", user); err != nil {
		t.Fatalf("ParseACLRule on created user: %v", err)
	}
	if user.CanExecuteCommand("GET") {
		t.Fatal("GET should be denied after +get -get")
	}
}
