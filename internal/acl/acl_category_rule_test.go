package acl

import "testing"

// TestParseACLRuleRejectsUnknownCategory pins the contract that ACL SETUSER must
// never report success for a rule it silently ignored. The category table is
// installed by the command package; without it (as in this package's tests)
// every category other than @all is unknown and must be refused, which is the
// fail-closed path. The categories themselves are tested in the command
// package.
//
// Redis ACL SETUSER accepts +command|all|@category and
// -command|all|@category, where @category names a whole command group
// (@read, @write, @admin, @dangerous, ...). ParseACLRule handled only "all":
// for any other @category the if had no else, so the token fell through the
// switch and was discarded.
//
// That is not a no-op for a REVOCATION. `ACL SETUSER bob +@all -@dangerous`
// returned +OK, and bob kept FLUSHALL, CONFIG, SHUTDOWN and the rest of the
// dangerous category -- an operator who wrote the rule believed those commands
// were revoked. Silently ignoring a grant merely under-permits; silently
// ignoring a deny over-permits.
func TestParseACLRuleRejectsUnknownCategory(t *testing.T) {
	// Control: an explicitly named command is a supported form. It must still
	// parse cleanly and actually deny. This passes before and after the fix, so
	// a broken harness cannot masquerade as the defect.
	ctl := newRuleUser()
	if err := ParseACLRule("+@all -FLUSHALL", ctl); err != nil {
		t.Fatalf("control failed: -FLUSHALL should be accepted, got %v", err)
	}
	if ctl.CanExecuteCommand("FLUSHALL") {
		t.Fatal("control failed: -FLUSHALL did not deny FLUSHALL")
	}
	if !ctl.CanExecuteCommand("GET") {
		t.Fatal("control failed: +@all did not grant GET")
	}

	// The contract: a category this parser cannot honour must be reported, not
	// swallowed. Red before the fix (err == nil and FLUSHALL still permitted).
	bob := newRuleUser()
	err := ParseACLRule("+@all -@dangerous", bob)
	if err == nil {
		t.Fatalf("FAIL: ParseACLRule(\"+@all -@dangerous\") returned nil — the @dangerous "+
			"category is not implemented, yet the rule was accepted and discarded, so the "+
			"dangerous commands the operator asked to revoke stayed permitted: FLUSHALL=%v "+
			"CONFIG=%v SHUTDOWN=%v",
			bob.CanExecuteCommand("FLUSHALL"),
			bob.CanExecuteCommand("CONFIG"),
			bob.CanExecuteCommand("SHUTDOWN"))
	}
	if bob.CanExecuteCommand("FLUSHALL") {
		t.Error("FAIL: FLUSHALL is still permitted after an operator revoked the dangerous category")
	}
}

// TestParseACLRuleRejectsUnknownGrantCategory covers the grant direction. A
// silently-ignored `+@read` under-grants rather than over-grants, but it is the
// same silent discard: the operator is told the rule applied and it did not.
func TestParseACLRuleRejectsUnknownGrantCategory(t *testing.T) {
	u := newRuleUser()
	if err := ParseACLRule("+@all -@all +@read", u); err == nil {
		t.Fatalf("FAIL: ParseACLRule accepted +@read, which is not implemented; the grant "+
			"was silently dropped (GET permitted = %v)", u.CanExecuteCommand("GET"))
	}
}

// newRuleUser builds a user with the maps initialised, matching what
// CreateUser produces.
func newRuleUser() *User {
	return &User{
		Enabled: true,
		Permissions: Permission{
			AllowedCommands: map[string]bool{},
			DeniedCommands:  map[string]bool{},
		},
	}
}
