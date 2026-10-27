package command

import (
	"testing"

	"github.com/cachestorm/cachestorm/internal/acl"
)

func categoryUser(t *testing.T, rule string) *acl.User {
	t.Helper()
	u := &acl.User{Name: "u", Permissions: acl.Permission{AllowedCommands: map[string]bool{}, DeniedCommands: map[string]bool{}}}
	if err := acl.ParseACLRule(rule, u); err != nil {
		t.Fatalf("ParseACLRule(%q): %v", rule, err)
	}
	return u
}

func TestACLCategoriesFollowRedisCommandTable(t *testing.T) {
	u := categoryUser(t, "+@all -@dangerous")
	if u.CanExecuteCommand("FLUSHALL") || u.CanExecuteCommand("KEYS") {
		t.Fatal("-@dangerous left FLUSHALL or KEYS permitted")
	}
	if u.CanExecuteSubcommand("CONFIG", "SET") || u.CanExecuteSubcommand("CLIENT", "KILL") {
		t.Fatal("-@dangerous left CONFIG SET or CLIENT KILL permitted")
	}
	if !u.CanExecuteCommand("GET") || !u.CanExecuteSubcommand("CLIENT", "ID") {
		t.Fatal("-@dangerous revoked GET or CLIENT ID")
	}

	r := categoryUser(t, "-@all +@read")
	if !r.CanExecuteCommand("GET") || !r.CanExecuteCommand("HGETALL") || r.CanExecuteCommand("SET") {
		t.Fatalf("+@read: GET=%v HGETALL=%v SET=%v", r.CanExecuteCommand("GET"), r.CanExecuteCommand("HGETALL"), r.CanExecuteCommand("SET"))
	}

	o := categoryUser(t, "+@all -@dangerous +flushall")
	if !o.CanExecuteCommand("FLUSHALL") || o.CanExecuteCommand("FLUSHDB") {
		t.Fatal("a later +flushall must re-grant FLUSHALL only")
	}

	s := categoryUser(t, "-@all +config|get")
	if !s.CanExecuteSubcommand("CONFIG", "GET") || s.CanExecuteSubcommand("CONFIG", "SET") {
		t.Fatal("+config|get must grant only that subcommand")
	}
	w := categoryUser(t, "+@all -config +config|get")
	if !w.CanExecuteSubcommand("CONFIG", "GET") || w.CanExecuteSubcommand("CONFIG", "REWRITE") {
		t.Fatal("-config +config|get must leave only CONFIG GET")
	}

	bad := &acl.User{Name: "b", Permissions: acl.Permission{AllowedCommands: map[string]bool{}, DeniedCommands: map[string]bool{}}}
	if err := acl.ParseACLRule("on +@nosuchcategory", bad); err == nil || bad.IsEnabled() {
		t.Fatalf("an unknown category was accepted (err=%v) or applied part of the rule", err)
	}
}
