package acl

import (
	"crypto/sha256"
	"crypto/subtle"
	"encoding/hex"
	"errors"
	"sort"
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

// ToACLString renders the user as ACL LIST does.
func (u *User) ToACLString() string {
	u.mu.RLock()
	defer u.mu.RUnlock()

	parts := []string{"user", u.Name}
	if u.Enabled {
		parts = append(parts, "on")
	} else {
		parts = append(parts, "off")
	}
	if u.NoPassword {
		parts = append(parts, "nopass")
	}
	for _, pw := range u.Passwords {
		parts = append(parts, "#"+pw)
	}
	for _, key := range u.Permissions.AllowedKeys {
		parts = append(parts, "~"+key)
	}
	if len(u.Permissions.AllowedChannels) == 0 {
		parts = append(parts, "resetchannels")
	}
	for _, ch := range u.Permissions.AllowedChannels {
		parts = append(parts, "&"+ch)
	}
	if u.Permissions.AllowedCommands["*"] {
		parts = append(parts, "+@all")
	} else {
		parts = append(parts, "-@all")
	}
	parts = append(parts, sortedRules("+", u.Permissions.AllowedCommands)...)
	parts = append(parts, sortedRules("-", u.Permissions.DeniedCommands)...)
	return strings.Join(parts, " ")
}

func sortedRules(sign string, cmds map[string]bool) []string {
	out := make([]string, 0, len(cmds))
	for cmd := range cmds {
		if cmd != "*" {
			out = append(out, sign+strings.ToLower(cmd))
		}
	}
	sort.Strings(out)
	return out
}

// ParseACLRule applies the space-separated rules in rule to user.
func ParseACLRule(rule string, user *User) error {
	return ParseACLRules(strings.Fields(rule), user)
}

// ValidateACLRules checks every rule without applying any, so a SETUSER with
// one bad rule changes nothing.
func ValidateACLRules(parts []string) error {
	for _, part := range parts {
		if err := validateACLRule(part); err != nil {
			return err
		}
	}
	return nil
}

func validateACLRule(part string) error {
	bad := func(why string) error {
		return errors.New("ERR Error in ACL SETUSER modifier '" + part + "': " + why)
	}
	switch strings.ToLower(part) {
	case "on", "off", "nopass", "resetpass", "reset", "resetkeys", "resetchannels",
		"allkeys", "allchannels", "allcommands", "nocommands":
		return nil
	}
	if part == "" {
		return bad("Syntax error")
	}
	switch part[0] {
	case '>', '<', '~', '&':
		return nil
	case '#', '!':
		if !isPasswordHash(part[1:]) {
			return bad("The password hash must be exactly 64 characters and contain only lowercase hexadecimal characters")
		}
		return nil
	case '%':
		perm, _, ok := strings.Cut(part[1:], "~")
		if !ok || perm == "" {
			return bad("Syntax error")
		}
		if !strings.EqualFold(perm, "RW") && !strings.EqualFold(perm, "WR") {
			return bad("Read-only or write-only key permissions are not supported")
		}
		return nil
	case '+', '-':
		if len(part) > 2 && part[1] == '@' {
			cat := strings.ToLower(part[2:])
			if cat == "all" {
				return nil
			}
			if categoryResolver == nil {
				return bad("Unknown command or category name in ACL")
			}
			if _, ok := categoryResolver(cat); !ok {
				return bad("Unknown command or category name in ACL")
			}
			return nil
		}
		if len(part) == 1 {
			return bad("Syntax error")
		}
		return nil
	}
	return bad("Syntax error")
}

func isPasswordHash(h string) bool {
	if len(h) != 64 {
		return false
	}
	for i := 0; i < len(h); i++ {
		c := h[i]
		if (c < '0' || c > '9') && (c < 'a' || c > 'f') {
			return false
		}
	}
	return true
}

// ParseACLRules applies rules to user with Redis' ACL SETUSER semantics. The
// whole list is validated first.
func ParseACLRules(parts []string, user *User) error {
	if err := ValidateACLRules(parts); err != nil {
		return err
	}
	for _, part := range parts {
		user.applyRule(part)
	}
	return nil
}

func (u *User) applyRule(part string) {
	switch strings.ToLower(part) {
	case "on":
		u.Enable()
		return
	case "off":
		u.Disable()
		return
	case "nopass":
		u.RemovePasswords()
		return
	case "allcommands":
		u.AllowCommand("*")
		return
	case "nocommands":
		u.DenyCommand("*")
		return
	}
	u.mu.Lock()
	defer u.mu.Unlock()
	p := &u.Permissions
	switch strings.ToLower(part) {
	case "resetpass":
		u.Passwords, u.NoPassword = nil, false
		return
	case "reset":
		u.Passwords, u.NoPassword, u.Enabled = nil, false, false
		p.AllowedKeys, p.AllowedChannels = nil, nil
		u.setCommandLocked("*", false)
		return
	case "resetkeys":
		p.AllowedKeys = nil
		return
	case "resetchannels":
		p.AllowedChannels = nil
		return
	case "allkeys":
		p.AllowedKeys = []string{"*"}
		return
	case "allchannels":
		p.AllowedChannels = []string{"*"}
		return
	}
	switch part[0] {
	case '>':
		u.addHashLocked(hashPassword(part[1:]))
	case '#':
		u.addHashLocked(part[1:])
	case '<':
		u.removeHashLocked(hashPassword(part[1:]))
	case '!':
		u.removeHashLocked(part[1:])
	case '~':
		p.AllowedKeys = addPattern(p.AllowedKeys, part[1:])
	case '%':
		_, pattern, _ := strings.Cut(part[1:], "~")
		p.AllowedKeys = addPattern(p.AllowedKeys, pattern)
	case '&':
		p.AllowedChannels = addPattern(p.AllowedChannels, part[1:])
	case '+', '-':
		allow := part[0] == '+'
		cmd := part[1:]
		if strings.HasPrefix(cmd, "@") {
			cat := strings.ToLower(cmd[1:])
			if cat != "all" {
				members, _ := categoryResolver(cat)
				for _, m := range members {
					u.setCommandLocked(m, allow)
				}
				return
			}
			cmd = "*"
		}
		u.setCommandLocked(cmd, allow)
	}
}

func (u *User) addHashLocked(h string) {
	u.NoPassword = false
	for _, stored := range u.Passwords {
		if stored == h {
			return
		}
	}
	u.Passwords = append(u.Passwords, h)
}

func (u *User) removeHashLocked(h string) {
	for i, stored := range u.Passwords {
		if stored == h {
			u.Passwords = append(u.Passwords[:i:i], u.Passwords[i+1:]...)
			return
		}
	}
}

func addPattern(patterns []string, p string) []string {
	if p == "*" {
		return []string{"*"}
	}
	for _, have := range patterns {
		if have == "*" || have == p {
			return patterns
		}
	}
	return append(patterns, p)
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
