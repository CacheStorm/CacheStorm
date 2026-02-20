package acl

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"strings"
	"sync"
)

var (
	ErrUserNotFound      = errors.New("ERR User not found")
	ErrUserAlreadyExists = errors.New("ERR User already exists")
	ErrInvalidPassword   = errors.New("ERR Invalid password")
	ErrPermissionDenied  = errors.New("NOPERM No permission")
)

type Permission struct {
	AllowedCommands map[string]bool
	DeniedCommands  map[string]bool
	AllowedKeys     []string
	AllowedChannels []string
}

type User struct {
	Name        string
	Enabled     bool
	NoPassword  bool
	Passwords   []string
	Permissions Permission
	IsDefault   bool
	mu          sync.RWMutex
}

type ACL struct {
	Users       map[string]*User
	DefaultUser *User
	mu          sync.RWMutex
}

func NewACL() *ACL {
	acl := &ACL{
		Users: make(map[string]*User),
	}

	defaultUser := &User{
		Name:       "default",
		Enabled:    true,
		NoPassword: true,
		IsDefault:  true,
		Permissions: Permission{
			AllowedCommands: map[string]bool{"*": true},
			AllowedKeys:     []string{"*"},
			AllowedChannels: []string{"*"},
		},
	}
	acl.Users["default"] = defaultUser
	acl.DefaultUser = defaultUser

	return acl
}

func (a *ACL) CreateUser(name string) (*User, error) {
	a.mu.Lock()
	defer a.mu.Unlock()

	if _, exists := a.Users[name]; exists {
		return nil, ErrUserAlreadyExists
	}

	user := &User{
		Name:    name,
		Enabled: false,
		Permissions: Permission{
			AllowedCommands: make(map[string]bool),
			DeniedCommands:  make(map[string]bool),
		},
	}
	a.Users[name] = user
	return user, nil
}

func (a *ACL) DeleteUser(name string) error {
	a.mu.Lock()
	defer a.mu.Unlock()

	if name == "default" {
		return errors.New("ERR cannot delete default user")
	}

	if _, exists := a.Users[name]; !exists {
		return ErrUserNotFound
	}

	delete(a.Users, name)
	return nil
}

func (a *ACL) GetUser(name string) (*User, bool) {
	a.mu.RLock()
	defer a.mu.RUnlock()

	user, exists := a.Users[name]
	return user, exists
}

func (a *ACL) ListUsers() []string {
	a.mu.RLock()
	defer a.mu.RUnlock()

	users := make([]string, 0, len(a.Users))
	for name := range a.Users {
		users = append(users, name)
	}
	return users
}

func (a *ACL) Authenticate(username, password string) (*User, error) {
	a.mu.RLock()
	defer a.mu.RUnlock()

	user, exists := a.Users[username]
	if !exists {
		return nil, ErrUserNotFound
	}

	user.mu.RLock()
	defer user.mu.RUnlock()

	if !user.Enabled {
		return nil, ErrUserNotFound
	}

	if user.NoPassword {
		return user, nil
	}

	hashedPassword := hashPassword(password)
	for _, stored := range user.Passwords {
		if stored == hashedPassword || stored == password {
			return user, nil
		}
	}

	return nil, ErrInvalidPassword
}

func (u *User) SetPassword(password string) {
	u.mu.Lock()
	defer u.mu.Unlock()

	u.NoPassword = false
	hashed := hashPassword(password)
	u.Passwords = []string{hashed}
}

func (u *User) AddPassword(password string) {
	u.mu.Lock()
	defer u.mu.Unlock()

	hashed := hashPassword(password)
	u.Passwords = append(u.Passwords, hashed)
}

func (u *User) RemovePasswords() {
	u.mu.Lock()
	defer u.mu.Unlock()

	u.Passwords = nil
	u.NoPassword = true
}

func (u *User) Enable() {
	u.mu.Lock()
	defer u.mu.Unlock()
	u.Enabled = true
}

func (u *User) Disable() {
	u.mu.Lock()
	defer u.mu.Unlock()
	u.Enabled = false
}

func (u *User) IsEnabled() bool {
	u.mu.RLock()
	defer u.mu.RUnlock()
	return u.Enabled
}

func (u *User) IsNoPassword() bool {
	u.mu.RLock()
	defer u.mu.RUnlock()
	return u.NoPassword
}

func (u *User) AllowCommand(cmd string) {
	u.mu.Lock()
	defer u.mu.Unlock()

	if cmd == "*" || cmd == "all" {
		u.Permissions.AllowedCommands = map[string]bool{"*": true}
	} else {
		u.Permissions.AllowedCommands[strings.ToUpper(cmd)] = true
		delete(u.Permissions.DeniedCommands, strings.ToUpper(cmd))
	}
}

func (u *User) DenyCommand(cmd string) {
	u.mu.Lock()
	defer u.mu.Unlock()

	if cmd == "*" || cmd == "all" {
		u.Permissions.DeniedCommands = map[string]bool{"*": true}
	} else {
		u.Permissions.DeniedCommands[strings.ToUpper(cmd)] = true
		delete(u.Permissions.AllowedCommands, strings.ToUpper(cmd))
	}
}

func (u *User) AllowKey(pattern string) {
	u.mu.Lock()
	defer u.mu.Unlock()
	u.Permissions.AllowedKeys = append(u.Permissions.AllowedKeys, pattern)
}

func (u *User) AllowChannel(pattern string) {
	u.mu.Lock()
	defer u.mu.Unlock()
	u.Permissions.AllowedChannels = append(u.Permissions.AllowedChannels, pattern)
}

func (u *User) CanExecuteCommand(cmd string) bool {
	u.mu.RLock()
	defer u.mu.RUnlock()

	cmd = strings.ToUpper(cmd)

	if _, denied := u.Permissions.DeniedCommands["*"]; denied {
		if _, allowed := u.Permissions.AllowedCommands[cmd]; allowed {
			return true
		}
		return false
	}

	if _, allowed := u.Permissions.AllowedCommands["*"]; allowed {
		if _, denied := u.Permissions.DeniedCommands[cmd]; denied {
			return false
		}
		return true
	}

	if _, allowed := u.Permissions.AllowedCommands[cmd]; allowed {
		return true
	}

	return false
}

func (u *User) CanAccessKey(key string) bool {
	u.mu.RLock()
	defer u.mu.RUnlock()

	if len(u.Permissions.AllowedKeys) == 0 {
		return true
	}

	for _, pattern := range u.Permissions.AllowedKeys {
		if pattern == "*" || pattern == "~*" {
			return true
		}
		if matchKeyPattern(key, pattern) {
			return true
		}
	}

	return false
}

func (u *User) CanAccessChannel(channel string) bool {
	u.mu.RLock()
	defer u.mu.RUnlock()

	if len(u.Permissions.AllowedChannels) == 0 {
		return true
	}

	for _, pattern := range u.Permissions.AllowedChannels {
		if pattern == "*" {
			return true
		}
		if matchKeyPattern(channel, pattern) {
			return true
		}
	}

	return false
}

func (u *User) ToACLString() string {
	u.mu.RLock()
	defer u.mu.RUnlock()

	var parts []string
	parts = append(parts, "user", u.Name)

	if u.Enabled {
		parts = append(parts, "on")
	} else {
		parts = append(parts, "off")
	}

	if u.NoPassword {
		parts = append(parts, "nopass")
	} else {
		for _, pw := range u.Passwords {
			parts = append(parts, "#"+pw[:16])
		}
	}

	if _, ok := u.Permissions.AllowedCommands["*"]; ok {
		parts = append(parts, "+@all")
	} else {
		for cmd := range u.Permissions.AllowedCommands {
			parts = append(parts, "+"+cmd)
		}
	}

	for cmd := range u.Permissions.DeniedCommands {
		parts = append(parts, "-"+cmd)
	}

	for _, key := range u.Permissions.AllowedKeys {
		parts = append(parts, "~"+key)
	}

	for _, ch := range u.Permissions.AllowedChannels {
		parts = append(parts, "&"+ch)
	}

	return strings.Join(parts, " ")
}

func ParseACLRule(rule string, user *User) error {
	parts := strings.Fields(rule)
	for _, part := range parts {
		switch {
		case part == "on":
			user.Enable()
		case part == "off":
			user.Disable()
		case part == "nopass":
			user.RemovePasswords()
		case strings.HasPrefix(part, ">"):
			user.SetPassword(part[1:])
		case strings.HasPrefix(part, "+"):
			cmd := part[1:]
			if strings.HasPrefix(cmd, "@") {
				cmd = cmd[1:]
				if cmd == "all" {
					user.AllowCommand("*")
				}
			} else {
				user.AllowCommand(cmd)
			}
		case strings.HasPrefix(part, "-"):
			cmd := part[1:]
			if strings.HasPrefix(cmd, "@") {
				cmd = cmd[1:]
				if cmd == "all" {
					user.DenyCommand("*")
				}
			} else {
				user.DenyCommand(cmd)
			}
		case strings.HasPrefix(part, "~"):
			user.AllowKey(part[1:])
		case strings.HasPrefix(part, "&"):
			user.AllowChannel(part[1:])
		}
	}
	return nil
}

func hashPassword(password string) string {
	hash := sha256.Sum256([]byte(password))
	return hex.EncodeToString(hash[:])
}

func matchKeyPattern(key, pattern string) bool {
	if pattern == "*" {
		return true
	}

	si, pi := 0, 0
	starIdx, match := -1, 0

	for si < len(key) {
		if pi < len(pattern) && (pattern[pi] == '?' || pattern[pi] == key[si]) {
			si++
			pi++
		} else if pi < len(pattern) && pattern[pi] == '*' {
			starIdx = pi
			match = si
			pi++
		} else if starIdx != -1 {
			pi = starIdx + 1
			match++
			si = match
		} else {
			return false
		}
	}

	for pi < len(pattern) && pattern[pi] == '*' {
		pi++
	}

	return pi == len(pattern)
}
