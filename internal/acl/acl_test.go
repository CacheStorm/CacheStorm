package acl

import (
	"testing"
)

func TestNewACL(t *testing.T) {
	acl := NewACL()

	if acl == nil {
		t.Fatal("expected ACL")
	}

	if len(acl.Users) != 1 {
		t.Errorf("expected 1 user, got %d", len(acl.Users))
	}

	if _, ok := acl.Users["default"]; !ok {
		t.Error("expected default user")
	}
}

func TestCreateUser(t *testing.T) {
	acl := NewACL()

	user, err := acl.CreateUser("testuser")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if user.Name != "testuser" {
		t.Errorf("expected testuser, got %s", user.Name)
	}
}

func TestCreateUserAlreadyExists(t *testing.T) {
	acl := NewACL()
	acl.CreateUser("testuser")

	_, err := acl.CreateUser("testuser")
	if err != ErrUserAlreadyExists {
		t.Errorf("expected ErrUserAlreadyExists, got %v", err)
	}
}

func TestDeleteUser(t *testing.T) {
	acl := NewACL()
	acl.CreateUser("testuser")

	if err := acl.DeleteUser("testuser"); err != nil {
		t.Errorf("unexpected error: %v", err)
	}

	if _, ok := acl.GetUser("testuser"); ok {
		t.Error("user should be deleted")
	}
}

func TestDeleteDefaultUser(t *testing.T) {
	acl := NewACL()

	err := acl.DeleteUser("default")
	if err == nil {
		t.Error("expected error when deleting default user")
	}
}

func TestDeleteNonExistentUser(t *testing.T) {
	acl := NewACL()

	err := acl.DeleteUser("nonexistent")
	if err != ErrUserNotFound {
		t.Errorf("expected ErrUserNotFound, got %v", err)
	}
}

func TestGetUser(t *testing.T) {
	acl := NewACL()
	acl.CreateUser("testuser")

	user, ok := acl.GetUser("testuser")
	if !ok {
		t.Error("expected to find user")
	}
	if user.Name != "testuser" {
		t.Errorf("expected testuser, got %s", user.Name)
	}
}

func TestGetUserNotFound(t *testing.T) {
	acl := NewACL()

	_, ok := acl.GetUser("nonexistent")
	if ok {
		t.Error("expected not to find user")
	}
}

func TestListUsers(t *testing.T) {
	acl := NewACL()
	acl.CreateUser("user1")
	acl.CreateUser("user2")

	users := acl.ListUsers()
	if len(users) != 3 {
		t.Errorf("expected 3 users, got %d", len(users))
	}
}

func TestAuthenticateDefaultUser(t *testing.T) {
	acl := NewACL()

	user, err := acl.Authenticate("default", "")
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	if user.Name != "default" {
		t.Errorf("expected default, got %s", user.Name)
	}
}

func TestAuthenticateNonExistentUser(t *testing.T) {
	acl := NewACL()

	_, err := acl.Authenticate("nonexistent", "password")
	if err != ErrUserNotFound {
		t.Errorf("expected ErrUserNotFound, got %v", err)
	}
}

func TestAuthenticateDisabledUser(t *testing.T) {
	acl := NewACL()
	user, _ := acl.CreateUser("testuser")

	user.SetPassword("password")
	_, err := acl.Authenticate("testuser", "password")
	if err != ErrUserNotFound {
		t.Errorf("expected ErrUserNotFound for disabled user, got %v", err)
	}
}

func TestUserSetPassword(t *testing.T) {
	acl := NewACL()
	user, _ := acl.CreateUser("testuser")

	user.SetPassword("password")

	if user.NoPassword {
		t.Error("user should not be nopass")
	}

	if len(user.Passwords) != 1 {
		t.Errorf("expected 1 password, got %d", len(user.Passwords))
	}
}

func TestUserAddPassword(t *testing.T) {
	user := &User{Permissions: Permission{}}

	user.AddPassword("pass1")
	user.AddPassword("pass2")

	if len(user.Passwords) != 2 {
		t.Errorf("expected 2 passwords, got %d", len(user.Passwords))
	}
}

func TestUserRemovePasswords(t *testing.T) {
	user := &User{Permissions: Permission{}}
	user.SetPassword("password")

	user.RemovePasswords()

	if !user.NoPassword {
		t.Error("user should be nopass")
	}

	if len(user.Passwords) != 0 {
		t.Error("passwords should be empty")
	}
}

func TestUserEnableDisable(t *testing.T) {
	user := &User{Permissions: Permission{}}

	user.Enable()
	if !user.IsEnabled() {
		t.Error("user should be enabled")
	}

	user.Disable()
	if user.IsEnabled() {
		t.Error("user should be disabled")
	}
}

func TestUserAllowCommand(t *testing.T) {
	user := &User{Permissions: Permission{
		AllowedCommands: make(map[string]bool),
		DeniedCommands:  make(map[string]bool),
	}}

	user.AllowCommand("GET")

	if !user.CanExecuteCommand("GET") {
		t.Error("user should be able to execute GET")
	}
}

func TestUserAllowAllCommands(t *testing.T) {
	user := &User{Permissions: Permission{
		AllowedCommands: make(map[string]bool),
		DeniedCommands:  make(map[string]bool),
	}}

	user.AllowCommand("*")

	if !user.CanExecuteCommand("GET") {
		t.Error("user should be able to execute any command")
	}
}

func TestUserDenyCommand(t *testing.T) {
	user := &User{Permissions: Permission{
		AllowedCommands: map[string]bool{"*": true},
		DeniedCommands:  make(map[string]bool),
	}}

	user.DenyCommand("FLUSHALL")

	if user.CanExecuteCommand("FLUSHALL") {
		t.Error("user should not be able to execute FLUSHALL")
	}
}

func TestUserAllowKey(t *testing.T) {
	user := &User{Permissions: Permission{}}

	user.AllowKey("user:*")

	if !user.CanAccessKey("user:1") {
		t.Error("user should access user:1")
	}
}

func TestUserAllowAllKeys(t *testing.T) {
	user := &User{Permissions: Permission{}}

	user.AllowKey("*")

	if !user.CanAccessKey("anykey") {
		t.Error("user should access any key")
	}
}

func TestUserCanAccessChannel(t *testing.T) {
	user := &User{Permissions: Permission{}}

	user.AllowChannel("news:*")

	if !user.CanAccessChannel("news:updates") {
		t.Error("user should access channel")
	}
}

func TestUserToACLString(t *testing.T) {
	user := &User{
		Name:       "testuser",
		Enabled:    true,
		NoPassword: true,
		Permissions: Permission{
			AllowedCommands: map[string]bool{"*": true},
			AllowedKeys:     []string{"*"},
		},
	}

	s := user.ToACLString()

	if s == "" {
		t.Error("expected non-empty ACL string")
	}
}

func TestParseACLRule(t *testing.T) {
	user := &User{Permissions: Permission{
		AllowedCommands: make(map[string]bool),
		DeniedCommands:  make(map[string]bool),
	}}

	err := ParseACLRule("on +@all ~*", user)
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}

	if !user.IsEnabled() {
		t.Error("user should be enabled")
	}
}

func TestParseACLRuleWithPassword(t *testing.T) {
	user := &User{Permissions: Permission{}}

	ParseACLRule(">mypass", user)

	if user.NoPassword {
		t.Error("user should have password")
	}
}

func TestMatchKeyPattern(t *testing.T) {
	tests := []struct {
		key      string
		pattern  string
		expected bool
	}{
		{"test", "test", true},
		{"test", "*", true},
		{"user:1", "user:*", true},
		{"user:1:profile", "user:*:profile", true},
		{"test", "other", false},
		{"test", "test?", false},
	}

	for _, tt := range tests {
		result := matchKeyPattern(tt.key, tt.pattern)
		if result != tt.expected {
			t.Errorf("matchKeyPattern(%s, %s) = %v, expected %v", tt.key, tt.pattern, result, tt.expected)
		}
	}
}

func TestHashPassword(t *testing.T) {
	hash := hashPassword("password")

	if hash == "" {
		t.Error("expected non-empty hash")
	}

	if hash == "password" {
		t.Error("hash should not be plaintext")
	}
}

func TestAuthenticateWithPassword(t *testing.T) {
	acl := NewACL()
	user, _ := acl.CreateUser("testuser")
	user.SetPassword("mypassword")
	user.Enable()

	authUser, err := acl.Authenticate("testuser", "mypassword")
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	if authUser.Name != "testuser" {
		t.Errorf("expected testuser, got %s", authUser.Name)
	}
}

func TestAuthenticateInvalidPassword(t *testing.T) {
	acl := NewACL()
	user, _ := acl.CreateUser("testuser")
	user.SetPassword("mypassword")
	user.Enable()

	_, err := acl.Authenticate("testuser", "wrongpassword")
	if err != ErrInvalidPassword {
		t.Errorf("expected ErrInvalidPassword, got %v", err)
	}
}

func TestCanExecuteCommandDeniedAll(t *testing.T) {
	user := &User{Permissions: Permission{
		AllowedCommands: make(map[string]bool),
		DeniedCommands:  map[string]bool{"*": true},
	}}

	if user.CanExecuteCommand("GET") {
		t.Error("user should not execute any command")
	}
}

func TestCanExecuteCommandDeniedAllButAllowed(t *testing.T) {
	user := &User{Permissions: Permission{
		AllowedCommands: map[string]bool{"GET": true},
		DeniedCommands:  map[string]bool{"*": true},
	}}

	if !user.CanExecuteCommand("GET") {
		t.Error("user should execute GET")
	}

	if user.CanExecuteCommand("SET") {
		t.Error("user should not execute SET")
	}
}

func TestIsNoPassword(t *testing.T) {
	user := &User{NoPassword: true}

	if !user.IsNoPassword() {
		t.Error("user should be nopass")
	}
}

func TestParseACLRuleOff(t *testing.T) {
	user := &User{Enabled: true, Permissions: Permission{}}

	ParseACLRule("off", user)

	if user.IsEnabled() {
		t.Error("user should be disabled")
	}
}

func TestParseACLRuleNopass(t *testing.T) {
	user := &User{Permissions: Permission{}}
	user.SetPassword("password")

	ParseACLRule("nopass", user)

	if !user.NoPassword {
		t.Error("user should be nopass")
	}
}

func TestParseACLRuleDenyCommand(t *testing.T) {
	user := &User{Permissions: Permission{
		AllowedCommands: map[string]bool{"*": true},
		DeniedCommands:  make(map[string]bool),
	}}

	ParseACLRule("-FLUSHALL", user)

	if user.CanExecuteCommand("FLUSHALL") {
		t.Error("user should not execute FLUSHALL")
	}
}

func TestParseACLRuleAllowChannel(t *testing.T) {
	user := &User{Permissions: Permission{}}

	ParseACLRule("&news:*", user)

	if len(user.Permissions.AllowedChannels) != 1 {
		t.Error("expected 1 allowed channel")
	}
}
