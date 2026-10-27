package acl

import (
	"crypto/sha256"
	"crypto/subtle"
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
			DeniedCommands:  make(map[string]bool),
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
		if subtle.ConstantTimeCompare([]byte(stored), []byte(hashedPassword)) == 1 {
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

// AllowCommand grants cmd, a command or "CMD|SUB" subcommand; "*" or "all"
// grants every command and clears earlier rules, as +@all does. Granting a
// whole command also replaces the rules of its subcommands.
func (u *User) AllowCommand(cmd string) {
	u.mu.Lock()
	defer u.mu.Unlock()
	u.setCommandLocked(cmd, true)
}

// DenyCommand revokes cmd the way AllowCommand grants it.
func (u *User) DenyCommand(cmd string) {
	u.mu.Lock()
	defer u.mu.Unlock()
	u.setCommandLocked(cmd, false)
}

func (u *User) setCommandLocked(cmd string, allow bool) {
	p := &u.Permissions
	if p.AllowedCommands == nil {
		p.AllowedCommands = make(map[string]bool)
	}
	if p.DeniedCommands == nil {
		p.DeniedCommands = make(map[string]bool)
	}
	if cmd == "*" || strings.EqualFold(cmd, "all") {
		if allow {
			p.AllowedCommands = map[string]bool{"*": true}
			p.DeniedCommands = make(map[string]bool)
		} else {
			p.DeniedCommands = map[string]bool{"*": true}
			p.AllowedCommands = make(map[string]bool)
		}
		return
	}
	cmd = strings.ToUpper(cmd)
	if !strings.Contains(cmd, "|") {
		prefix := cmd + "|"
		for k := range p.AllowedCommands {
			if strings.HasPrefix(k, prefix) {
				delete(p.AllowedCommands, k)
			}
		}
		for k := range p.DeniedCommands {
			if strings.HasPrefix(k, prefix) {
				delete(p.DeniedCommands, k)
			}
		}
	}
	if allow {
		p.AllowedCommands[cmd] = true
		delete(p.DeniedCommands, cmd)
	} else {
		p.DeniedCommands[cmd] = true
		delete(p.AllowedCommands, cmd)
	}
}

// categoryResolver returns the commands and "CMD|SUB" subcommands of an ACL
// category, and whether the category exists. The command package installs
// it from Redis' command table.
var categoryResolver func(category string) ([]string, bool)

// SetCategoryResolver installs the category table used by +@category and
// -@category rules.
func SetCategoryResolver(fn func(category string) ([]string, bool)) {
	categoryResolver = fn
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

// CanExecuteSubcommand reports whether the user may run sub of the container
// command cmd: a rule for "CMD|SUB" wins over one for CMD.
func (u *User) CanExecuteSubcommand(cmd, sub string) bool {
	key := strings.ToUpper(cmd) + "|" + strings.ToUpper(sub)
	u.mu.RLock()
	allowed, isAllowed := u.Permissions.AllowedCommands[key]
	denied, isDenied := u.Permissions.DeniedCommands[key]
	u.mu.RUnlock()
	if isAllowed && allowed {
		return true
	}
	if isDenied && denied {
		return false
	}
	return u.CanExecuteCommand(cmd)
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

	// Validate the WHOLE rule before mutating anything, so a rule with an
	// unknown category changes nothing and reports the error.
	for _, part := range parts {
		if len(part) > 2 && (part[0] == '+' || part[0] == '-') && part[1] == '@' {
			cat := strings.ToLower(part[2:])
			if cat == "all" {
				continue
			}
			if categoryResolver == nil {
				return errors.New("ERR Error in ACL SETUSER modifier '" + part + "': Unknown command or category name in ACL")
			}
			if _, ok := categoryResolver(cat); !ok {
				return errors.New("ERR Error in ACL SETUSER modifier '" + part + "': Unknown command or category name in ACL")
			}
		}
	}

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
		case part == "allcommands":
			user.AllowCommand("*")
		case part == "nocommands":
			user.DenyCommand("*")
		case strings.HasPrefix(part, "+") || strings.HasPrefix(part, "-"):
			allow := part[0] == '+'
			cmd := part[1:]
			if strings.HasPrefix(cmd, "@") {
				cat := strings.ToLower(cmd[1:])
				if cat == "all" {
					cmd = "*"
				} else {
					members, _ := categoryResolver(cat)
					user.mu.Lock()
					for _, m := range members {
						user.setCommandLocked(m, allow)
					}
					user.mu.Unlock()
					continue
				}
			}
			if allow {
				user.AllowCommand(cmd)
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
