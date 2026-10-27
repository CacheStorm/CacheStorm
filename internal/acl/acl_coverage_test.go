package acl

import (
	"strings"
	"testing"
)

// Cover DenyCommand with "all" keyword
func TestUserDenyCommandAll(t *testing.T) {
	user := &User{Permissions: Permission{
		AllowedCommands: map[string]bool{"GET": true},
		DeniedCommands:  make(map[string]bool),
	}}

	user.DenyCommand("all")

	if _, ok := user.Permissions.DeniedCommands["*"]; !ok {
		t.Error("expected * in denied commands after DenyCommand(all)")
	}
}

// Cover DenyCommand with "*" keyword
func TestUserDenyCommandStar(t *testing.T) {
	user := &User{Permissions: Permission{
		AllowedCommands: map[string]bool{"GET": true},
		DeniedCommands:  make(map[string]bool),
	}}

	user.DenyCommand("*")

	if _, ok := user.Permissions.DeniedCommands["*"]; !ok {
		t.Error("expected * in denied commands after DenyCommand(*)")
	}
}

// Cover AllowCommand with "all" keyword
func TestUserAllowCommandAll(t *testing.T) {
	user := &User{Permissions: Permission{
		AllowedCommands: make(map[string]bool),
		DeniedCommands:  make(map[string]bool),
	}}

	user.AllowCommand("all")

	if _, ok := user.Permissions.AllowedCommands["*"]; !ok {
		t.Error("expected * in allowed commands after AllowCommand(all)")
	}
}

// Cover CanExecuteCommand: no allowed commands, no denied commands, command not found
func TestCanExecuteCommandNoPermissions(t *testing.T) {
	user := &User{Permissions: Permission{
		AllowedCommands: make(map[string]bool),
		DeniedCommands:  make(map[string]bool),
	}}

	if user.CanExecuteCommand("GET") {
		t.Error("should not execute GET without any permissions")
	}
}

// Cover CanAccessKey with "~*" pattern
func TestCanAccessKeyTildeWildcard(t *testing.T) {
	user := &User{Permissions: Permission{
		AllowedKeys: []string{"~*"},
	}}

	if !user.CanAccessKey("anykey") {
		t.Error("expected access with ~* pattern")
	}
}

// Cover CanAccessKey: no patterns match
func TestCanAccessKeyNoMatch(t *testing.T) {
	user := &User{Permissions: Permission{
		AllowedKeys: []string{"user:*"},
	}}

	if user.CanAccessKey("admin:secret") {
		t.Error("should not access admin:secret with user:* pattern")
	}
}

// No ~pattern means no key access and no &pattern no channel access, as in
// Redis 7.
func TestEmptyPatternsDenyAccess(t *testing.T) {
	user := &User{}
	if user.CanAccessKey("anykey") {
		t.Error("a user without key patterns accessed a key")
	}
	if user.CanAccessChannel("anychannel") {
		t.Error("a user without channel patterns accessed a channel")
	}
}

// Cover CanAccessChannel: wildcard "*"
func TestCanAccessChannelWildcard(t *testing.T) {
	user := &User{Permissions: Permission{
		AllowedChannels: []string{"*"},
	}}

	if !user.CanAccessChannel("anychannel") {
		t.Error("expected access with * channel pattern")
	}
}

// Cover CanAccessChannel: specific pattern match
func TestCanAccessChannelPatternMatch(t *testing.T) {
	user := &User{Permissions: Permission{
		AllowedChannels: []string{"news:*"},
	}}

	if !user.CanAccessChannel("news:sports") {
		t.Error("expected access for news:sports with news:* pattern")
	}
}

// Cover CanAccessChannel: no match
func TestCanAccessChannelNoMatch(t *testing.T) {
	user := &User{Permissions: Permission{
		AllowedChannels: []string{"news:*"},
	}}

	if user.CanAccessChannel("admin:alerts") {
		t.Error("should not access admin:alerts with news:* pattern")
	}
}

// Cover ToACLString: disabled user
func TestToACLStringDisabledUser(t *testing.T) {
	user := &User{
		Name:       "disableduser",
		Enabled:    false,
		NoPassword: true,
		Permissions: Permission{
			AllowedCommands: map[string]bool{"*": true},
		},
	}

	s := user.ToACLString()
	if !strings.Contains(s, "off") {
		t.Error("expected 'off' in ACL string for disabled user")
	}
}

// Cover ToACLString: user with passwords (not nopass)
func TestToACLStringWithPasswords(t *testing.T) {
	user := &User{
		Name:       "passuser",
		Enabled:    true,
		NoPassword: false,
		Passwords:  []string{"abcdef1234567890abcdef1234567890abcdef1234567890abcdef1234567890"},
		Permissions: Permission{
			AllowedCommands: map[string]bool{"GET": true, "SET": true},
			DeniedCommands:  map[string]bool{"FLUSHALL": true},
			AllowedKeys:     []string{"user:*"},
			AllowedChannels: []string{"news:*"},
			CommandRules:    []string{"+get", "+set", "-flushall"},
		},
	}

	s := user.ToACLString()

	if !strings.Contains(s, "on") {
		t.Error("expected 'on' in ACL string")
	}
	if !strings.Contains(s, "#") {
		t.Error("expected password hash prefix '#' in ACL string")
	}
	if strings.Contains(s, "nopass") {
		t.Error("should not contain 'nopass' when user has passwords")
	}
	if !strings.Contains(s, "+get") || !strings.Contains(s, "+set") {
		t.Error("expected +get and +set in ACL string")
	}
	if !strings.Contains(s, "-flushall") {
		t.Error("expected -flushall in ACL string")
	}
	if !strings.Contains(s, "~user:*") {
		t.Error("expected ~user:* in ACL string")
	}
	if !strings.Contains(s, "&news:*") {
		t.Error("expected &news:* in ACL string")
	}
}

// Cover ToACLString: user with +@all (wildcard allowed)
func TestToACLStringWithAllCommands(t *testing.T) {
	user := &User{
		Name:       "allcmds",
		Enabled:    true,
		NoPassword: true,
		Permissions: Permission{
			AllowedCommands: map[string]bool{"*": true},
		},
	}

	s := user.ToACLString()
	if !strings.Contains(s, "+@all") {
		t.Error("expected '+@all' in ACL string for wildcard allowed")
	}
}

// Cover ParseACLRule: -@all (deny all commands)
func TestParseACLRuleDenyAll(t *testing.T) {
	user := &User{Permissions: Permission{
		AllowedCommands: map[string]bool{"*": true},
		DeniedCommands:  make(map[string]bool),
	}}

	err := ParseACLRule("-@all", user)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if _, ok := user.Permissions.DeniedCommands["*"]; !ok {
		t.Error("expected * in denied commands after -@all")
	}
}

// Cover ParseACLRule: +specific command (not @all)
func TestParseACLRuleAllowSpecificCommand(t *testing.T) {
	user := &User{Permissions: Permission{
		AllowedCommands: make(map[string]bool),
		DeniedCommands:  make(map[string]bool),
	}}

	err := ParseACLRule("+GET +SET", user)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !user.CanExecuteCommand("GET") {
		t.Error("expected GET to be allowed")
	}
	if !user.CanExecuteCommand("SET") {
		t.Error("expected SET to be allowed")
	}
}

// Cover ParseACLRule: -specific command (not @all)
func TestParseACLRuleDenySpecificCommand(t *testing.T) {
	user := &User{Permissions: Permission{
		AllowedCommands: map[string]bool{"*": true},
		DeniedCommands:  make(map[string]bool),
	}}

	err := ParseACLRule("-GET", user)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if user.CanExecuteCommand("GET") {
		t.Error("expected GET to be denied")
	}
}

// Cover ParseACLRule: ~key pattern
func TestParseACLRuleAllowKey(t *testing.T) {
	user := &User{Permissions: Permission{
		AllowedCommands: make(map[string]bool),
		DeniedCommands:  make(map[string]bool),
	}}

	err := ParseACLRule("~user:* ~data:*", user)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(user.Permissions.AllowedKeys) != 2 {
		t.Errorf("expected 2 allowed keys, got %d", len(user.Permissions.AllowedKeys))
	}
}

// Cover ParseACLRule: full complex rule
func TestParseACLRuleComplex(t *testing.T) {
	user := &User{Permissions: Permission{
		AllowedCommands: make(map[string]bool),
		DeniedCommands:  make(map[string]bool),
	}}

	err := ParseACLRule("on >mypassword +@all -FLUSHALL ~user:* &news:*", user)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !user.IsEnabled() {
		t.Error("expected user to be enabled")
	}
	if user.NoPassword {
		t.Error("expected user to have password")
	}
	if user.CanExecuteCommand("FLUSHALL") {
		t.Error("expected FLUSHALL to be denied")
	}
	if !user.CanAccessKey("user:123") {
		t.Error("expected access to user:123")
	}
}

// Cover ParseACLRule: empty rule
func TestParseACLRuleEmpty(t *testing.T) {
	user := &User{Permissions: Permission{
		AllowedCommands: make(map[string]bool),
		DeniedCommands:  make(map[string]bool),
	}}

	err := ParseACLRule("", user)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

// Cover matchKeyPattern: '?' wildcard matching a single character
func TestMatchKeyPatternQuestionMark(t *testing.T) {
	if !matchKeyPattern("ab", "a?") {
		t.Error("expected a? to match ab")
	}
	if matchKeyPattern("abc", "a?") {
		t.Error("expected a? to not match abc")
	}
}

// Cover matchKeyPattern: multiple wildcards
func TestMatchKeyPatternMultipleWildcards(t *testing.T) {
	if !matchKeyPattern("abcdef", "a*d*f") {
		t.Error("expected a*d*f to match abcdef")
	}
}

// Cover matchKeyPattern: trailing wildcard
func TestMatchKeyPatternTrailingWildcard(t *testing.T) {
	if !matchKeyPattern("abc", "a*") {
		t.Error("expected a* to match abc")
	}
}

// Cover matchKeyPattern: pattern longer than key (no match)
func TestMatchKeyPatternLongerPattern(t *testing.T) {
	if matchKeyPattern("a", "abc") {
		t.Error("expected abc to not match a")
	}
}

// Cover matchKeyPattern: empty key, empty pattern
func TestMatchKeyPatternEmpty(t *testing.T) {
	if !matchKeyPattern("", "") {
		t.Error("expected empty pattern to match empty key")
	}
}

// Cover matchKeyPattern: empty key, non-empty pattern
func TestMatchKeyPatternEmptyKeyNonEmptyPattern(t *testing.T) {
	if matchKeyPattern("", "abc") {
		t.Error("expected abc to not match empty key")
	}
}

// Cover matchKeyPattern: empty key, star pattern
func TestMatchKeyPatternEmptyKeyStar(t *testing.T) {
	if !matchKeyPattern("", "*") {
		t.Error("expected * to match empty key")
	}
}

// Cover matchKeyPattern: star in the middle consuming nothing
func TestMatchKeyPatternStarConsumingNothing(t *testing.T) {
	if !matchKeyPattern("ab", "a*b") {
		t.Error("expected a*b to match ab")
	}
}

// Cover matchKeyPattern: backtracking scenario
func TestMatchKeyPatternBacktrack(t *testing.T) {
	if !matchKeyPattern("aab", "a*b") {
		t.Error("expected a*b to match aab")
	}
	if !matchKeyPattern("aaabbb", "*bbb") {
		t.Error("expected *bbb to match aaabbb")
	}
}

// Cover CanAccessKey with specific pattern match using matchKeyPattern
func TestCanAccessKeySpecificPattern(t *testing.T) {
	user := &User{Permissions: Permission{
		AllowedKeys: []string{"cache:session:*"},
	}}

	if !user.CanAccessKey("cache:session:abc123") {
		t.Error("expected access to cache:session:abc123")
	}
	if user.CanAccessKey("cache:data:abc123") {
		t.Error("should not access cache:data:abc123")
	}
}

// Cover CanAccessChannel: specific pattern with matchKeyPattern (not * or ~*)
func TestCanAccessChannelSpecificPatternWithWildcard(t *testing.T) {
	user := &User{Permissions: Permission{
		AllowedChannels: []string{"events:user:*"},
	}}

	if !user.CanAccessChannel("events:user:login") {
		t.Error("expected access to events:user:login")
	}
	if user.CanAccessChannel("events:admin:login") {
		t.Error("should not access events:admin:login")
	}
}

// Cover DenyCommand: specific command removes from AllowedCommands
func TestDenyCommandRemovesFromAllowed(t *testing.T) {
	user := &User{Permissions: Permission{
		AllowedCommands: map[string]bool{"GET": true, "SET": true},
		DeniedCommands:  make(map[string]bool),
	}}

	user.DenyCommand("get") // lowercase input

	if _, ok := user.Permissions.AllowedCommands["GET"]; ok {
		t.Error("GET should be removed from allowed commands after deny")
	}
	if _, ok := user.Permissions.DeniedCommands["GET"]; !ok {
		t.Error("GET should be in denied commands")
	}
}

// Cover AllowCommand: specific command removes from DeniedCommands
func TestAllowCommandRemovesFromDenied(t *testing.T) {
	user := &User{Permissions: Permission{
		AllowedCommands: make(map[string]bool),
		DeniedCommands:  map[string]bool{"SET": true},
	}}

	user.AllowCommand("set") // lowercase input

	if _, ok := user.Permissions.DeniedCommands["SET"]; ok {
		t.Error("SET should be removed from denied commands after allow")
	}
	if _, ok := user.Permissions.AllowedCommands["SET"]; !ok {
		t.Error("SET should be in allowed commands")
	}
}

// Cover ParseACLRule: -@<non-all> (prefix - with @somegroup, not "all")
//
// This used to assert that -@read was silently accepted. That was the defect:
// only @all is implemented, so a `-@category` revocation was discarded while
// ACL SETUSER still replied +OK, leaving the category's commands permitted.
// Ignoring a deny OVER-permits, so the category must be reported instead.
func TestParseACLRuleDenyNonAllGroup(t *testing.T) {
	user := &User{Permissions: Permission{
		AllowedCommands: map[string]bool{"*": true},
		DeniedCommands:  make(map[string]bool),
	}}

	err := ParseACLRule("-@read", user)
	if err == nil {
		t.Fatal("expected an error for the unsupported category @read, got nil — " +
			"silently dropping a -@category leaves the commands it was meant to revoke permitted")
	}

	// The rule is rejected before anything is applied, so the user's grants are
	// untouched by the failed SETUSER.
	if _, ok := user.Permissions.DeniedCommands["*"]; ok {
		t.Error("a rejected rule must not mutate the user")
	}
	if !user.CanExecuteCommand("GET") {
		t.Error("a rejected rule must leave the pre-existing +@all grant intact")
	}
}

// Cover ParseACLRule: +@<non-all> (prefix + with @somegroup, not "all")
//
// Same reasoning as the deny case: +@read is not implemented, so it must be
// reported rather than silently dropped.
func TestParseACLRuleAllowNonAllGroup(t *testing.T) {
	user := &User{Permissions: Permission{
		AllowedCommands: make(map[string]bool),
		DeniedCommands:  make(map[string]bool),
	}}

	if err := ParseACLRule("+@read", user); err == nil {
		t.Fatal("expected an error for the unsupported category @read, got nil — " +
			"an operator is told the grant applied when it did not")
	}

	// Nothing may have been granted by the rejected rule.
	if _, ok := user.Permissions.AllowedCommands["*"]; ok {
		t.Error("a rejected rule must not grant *")
	}
	if user.CanExecuteCommand("GET") {
		t.Error("a rejected rule must not grant any command")
	}
}

// Cover matchKeyPattern: multiple trailing stars
func TestMatchKeyPatternMultipleTrailingStars(t *testing.T) {
	if !matchKeyPattern("abc", "abc***") {
		t.Error("expected abc*** to match abc")
	}
}

func TestSetUserRulesFollowRedis(t *testing.T) {
	a := NewACL()
	u, _ := a.CreateUser("r")
	if err := ParseACLRules([]string{"on", ">one", ">two", "~a:*", "&ch", "+get"}, u); err != nil {
		t.Fatal(err)
	}
	if len(u.Passwords) != 2 {
		t.Fatalf("> added %d passwords, want 2", len(u.Passwords))
	}
	if _, err := a.Authenticate("r", "one"); err != nil {
		t.Fatal("first password lost after adding a second")
	}
	if err := ParseACLRules([]string{"<one"}, u); err != nil || len(u.Passwords) != 1 {
		t.Fatalf("<one: err %v, passwords %d", err, len(u.Passwords))
	}
	if err := ParseACLRules([]string{"resetkeys", "resetchannels"}, u); err != nil || u.CanAccessKey("a:1") || u.CanAccessChannel("ch") {
		t.Fatal("resetkeys/resetchannels kept access")
	}
	if err := ParseACLRules([]string{"allkeys", "allchannels"}, u); err != nil || !u.CanAccessKey("x") || !u.CanAccessChannel("y") {
		t.Fatal("allkeys/allchannels did not grant access")
	}
	if err := ParseACLRules([]string{"reset"}, u); err != nil || u.IsEnabled() || u.CanAccessKey("x") || u.CanExecuteCommand("GET") || len(u.Passwords) != 0 {
		t.Fatalf("reset left %q", u.ToACLString())
	}
	for _, bad := range []string{"bogus", "#abc", "%R~k", "+"} {
		if err := ParseACLRules([]string{"on", bad}, u); err == nil {
			t.Errorf("%q was accepted", bad)
		}
	}
	if u.IsEnabled() {
		t.Fatal("a rejected rule list was partly applied")
	}
	if err := ParseACLRules([]string{"%RW~k:*"}, u); err != nil || !u.CanAccessKey("k:1") {
		t.Fatalf("%%RW~ = %v", err)
	}
}
