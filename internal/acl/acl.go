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

// Key permission flags of a key pattern: ~pattern grants both, %R~pattern
// reads only and %W~pattern writes only.
const (
	KeyRead uint8 = 1 << iota
	KeyWrite
	KeyReadWrite = KeyRead | KeyWrite
)

// KeyPattern is a key glob with the access it grants.
type KeyPattern struct {
	Pattern string
	Flags   uint8
}

// Permission is a Redis ACL selector: the user's root permissions or one of
// its additional (...) selectors.
type Permission struct {
	AllowedCommands map[string]bool
	DeniedCommands  map[string]bool
	Keys            []KeyPattern
	AllowedChannels []string
	// CommandRules is the user's command rule history as Redis keeps it
	// (selector->command_rules): "+cmd", "-cmd", "+cmd|sub" and "+@cat"
	// entries in the order applied, a later rule for a command replacing
	// earlier ones for it and its subcommands. +@all and -@all clear it.
	CommandRules []string
}

type User struct {
	Name        string
	Enabled     bool
	NoPassword  bool
	Passwords   []string
	Permissions Permission
	// Selectors are the user's additional selectors; a command is allowed
	// when the root permissions or any selector allows all of it.
	Selectors []*Permission
	IsDefault bool
	// SkipSanitizePayload is the skip-sanitize-payload flag; Redis users
	// default to sanitize-payload.
	SkipSanitizePayload bool
	mu                  sync.RWMutex
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
			Keys:            []KeyPattern{{Pattern: "*", Flags: KeyReadWrite}},
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
	sort.Strings(users)
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
	u.Permissions.setCommand(cmd, true)
}

// DenyCommand revokes cmd the way AllowCommand grants it.
func (u *User) DenyCommand(cmd string) {
	u.mu.Lock()
	defer u.mu.Unlock()
	u.Permissions.setCommand(cmd, false)
}

func (p *Permission) setCommand(cmd string, allow bool) {
	p.setCommandBits(cmd, allow)
	if cmd == "*" || strings.EqualFold(cmd, "all") {
		p.CommandRules = nil
		return
	}
	p.recordRule(cmd, allow)
}

// recordRule appends rule to the command rule history the way Redis'
// ACLUpdateCommandRules does: earlier rules for the same command or category,
// and for the command's subcommands, are dropped first.
func (p *Permission) recordRule(rule string, allow bool) {
	rule = strings.ToLower(rule)
	kept := make([]string, 0, len(p.CommandRules)+1)
	for _, r := range p.CommandRules {
		body := r[1:]
		if body == rule || strings.HasPrefix(body, rule+"|") {
			continue
		}
		kept = append(kept, r)
	}
	if allow {
		kept = append(kept, "+"+rule)
	} else {
		kept = append(kept, "-"+rule)
	}
	p.CommandRules = kept
}

func (p *Permission) setCommandBits(cmd string, allow bool) {
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

// commandResolver reports whether a command exists and whether it is a
// container whose subcommands rules may name; with sub set, whether the
// container has that subcommand. The command package installs it.
var commandResolver func(cmd, sub string) (known, container, subKnown bool)

// SetCommandResolver installs the command table +command and -command rules
// are checked against.
func SetCommandResolver(fn func(cmd, sub string) (known, container, subKnown bool)) {
	commandResolver = fn
}

// commandRuleError is why ACLSetSelector refuses the command a +name or
// -name rule names, or "": an unknown command, a subcommand of a
// subcommand, an empty subcommand, or, for -, a first argument of a command
// without subcommands.
func commandRuleError(allow bool, name string) string {
	if commandResolver == nil {
		return ""
	}
	name = strings.ToLower(name)
	i := strings.LastIndexByte(name, '|')
	if i < 0 {
		if known, _, _ := commandResolver(name, ""); !known {
			return errUnknownCmd
		}
		return ""
	}
	cmd, sub := name[:i], name[i+1:]
	if j := strings.IndexByte(cmd, '|'); j >= 0 {
		if _, _, ok := commandResolver(cmd[:j], cmd[j+1:]); ok && allow {
			return errFirstArgSub
		}
		return errUnknownCmd
	}
	known, container, subKnown := commandResolver(cmd, sub)
	switch {
	case !known:
		return errUnknownCmd
	case allow && sub == "":
		return errSyntax
	case (container || !allow) && !subKnown:
		return errUnknownCmd
	}
	return ""
}

// AllowKey grants read and write access to the keys matching pattern.
func (u *User) AllowKey(pattern string) {
	u.mu.Lock()
	defer u.mu.Unlock()
	u.Permissions.Keys = append(u.Permissions.Keys, KeyPattern{Pattern: pattern, Flags: KeyReadWrite})
}

func (u *User) AllowChannel(pattern string) {
	u.mu.Lock()
	defer u.mu.Unlock()
	u.Permissions.AllowedChannels = append(u.Permissions.AllowedChannels, pattern)
}

// canRun reports whether the selector allows cmd, or its subcommand sub
// when sub is not empty: a rule for "CMD|SUB" wins over one for CMD.
func (p *Permission) canRun(cmd, sub string) bool {
	cmd = strings.ToUpper(cmd)
	if sub != "" {
		key := cmd + "|" + strings.ToUpper(sub)
		if p.AllowedCommands[key] {
			return true
		}
		if p.DeniedCommands[key] {
			return false
		}
	}
	if p.DeniedCommands["*"] {
		return p.AllowedCommands[cmd]
	}
	if p.AllowedCommands["*"] {
		return !p.DeniedCommands[cmd]
	}
	return p.AllowedCommands[cmd]
}

func (p *Permission) allKeys() bool {
	return len(p.Keys) == 1 && p.Keys[0].Flags == KeyReadWrite && (p.Keys[0].Pattern == "*" || p.Keys[0].Pattern == "~*")
}

func (p *Permission) allChannels() bool {
	return hasPattern(p.AllowedChannels, "*")
}

// keyAllowed reports whether a pattern grants every access in flags to key.
func (p *Permission) keyAllowed(key string, flags uint8) bool {
	for _, kp := range p.Keys {
		if kp.Flags&flags != flags {
			continue
		}
		if kp.Pattern == "*" || kp.Pattern == "~*" || matchKeyPattern(key, kp.Pattern) {
			return true
		}
	}
	return false
}

// channelAllowed checks a channel as Redis does: a PSUBSCRIBE pattern must
// equal an allowed pattern, a channel must match one.
func (p *Permission) channelAllowed(channel string, isPattern bool) bool {
	for _, pattern := range p.AllowedChannels {
		if pattern == "*" {
			return true
		}
		if isPattern {
			if pattern == channel {
				return true
			}
		} else if matchKeyPattern(channel, pattern) {
			return true
		}
	}
	return false
}

func (u *User) eachSelectorLocked(fn func(p *Permission) bool) bool {
	if fn(&u.Permissions) {
		return true
	}
	for _, s := range u.Selectors {
		if fn(s) {
			return true
		}
	}
	return false
}

// CanExecuteSubcommand reports whether a selector of the user allows sub of
// the container command cmd.
func (u *User) CanExecuteSubcommand(cmd, sub string) bool {
	u.mu.RLock()
	defer u.mu.RUnlock()
	return u.eachSelectorLocked(func(p *Permission) bool { return p.canRun(cmd, sub) })
}

// CanExecuteCommand reports whether a selector of the user allows cmd.
func (u *User) CanExecuteCommand(cmd string) bool {
	u.mu.RLock()
	defer u.mu.RUnlock()
	return u.eachSelectorLocked(func(p *Permission) bool { return p.canRun(cmd, "") })
}

// CanAccessKey reports whether a selector of the user may read and write key.
func (u *User) CanAccessKey(key string) bool {
	u.mu.RLock()
	defer u.mu.RUnlock()
	return u.eachSelectorLocked(func(p *Permission) bool { return p.keyAllowed(key, KeyReadWrite) })
}

// CanAccessChannel reports whether a selector of the user allows channel.
func (u *User) CanAccessChannel(channel string) bool {
	u.mu.RLock()
	defer u.mu.RUnlock()
	return u.eachSelectorLocked(func(p *Permission) bool { return p.channelAllowed(channel, false) })
}

// KeyRef is a key a command names and the access the command needs to it.
type KeyRef struct {
	Name  string
	Flags uint8
}

// ChannelRef is a channel a command names; Pattern marks a PSUBSCRIBE
// pattern.
type ChannelRef struct {
	Name    string
	Pattern bool
}

// Denial is the outcome of a permission check, in Redis' order of
// relevance.
type Denial int

const (
	Allowed Denial = iota
	DeniedCommand
	DeniedKey
	DeniedChannel
)

// Check reports whether the user may run cmd (with subcommand sub, or "")
// on the keys and channels it names, as Redis' ACLCheckAllUserCommandPerm
// does: some selector must allow the command, every key with the access it
// needs and every channel. When none does, the most relevant denial is
// returned with the key or channel refused. keys and channels are called at
// most once, and only when a selector restricts them.
func (u *User) Check(cmd, sub string, keys func() []KeyRef, channels func() []ChannelRef) (Denial, string) {
	u.mu.RLock()
	defer u.mu.RUnlock()
	var keyList []KeyRef
	var chanList []ChannelRef
	keysDone, chansDone := false, false
	worst, item, worstIdx := DeniedCommand, "", -1
	note := func(d Denial, what string, idx int) {
		if d > worst || (d == worst && idx > worstIdx) {
			worst, item, worstIdx = d, what, idx
		}
	}
	ok := u.eachSelectorLocked(func(p *Permission) bool {
		if !p.canRun(cmd, sub) {
			return false
		}
		if !p.allKeys() {
			if !keysDone {
				keysDone = true
				if keys != nil {
					keyList = keys()
				}
			}
			for i, k := range keyList {
				if !p.keyAllowed(k.Name, k.Flags) {
					note(DeniedKey, k.Name, i)
					return false
				}
			}
		}
		if !p.allChannels() {
			if !chansDone {
				chansDone = true
				if channels != nil {
					chanList = channels()
				}
			}
			for i, c := range chanList {
				if !p.channelAllowed(c.Name, c.Pattern) {
					note(DeniedChannel, c.Name, i)
					return false
				}
			}
		}
		return true
	})
	if ok {
		return Allowed, ""
	}
	return worst, item
}

// ToACLString renders the user as ACL LIST does.
func (u *User) ToACLString() string {
	u.mu.RLock()
	defer u.mu.RUnlock()

	parts := []string{"user", u.Name}
	parts = append(parts, u.flagsLocked()...)
	for _, pw := range u.Passwords {
		parts = append(parts, "#"+pw)
	}
	parts = append(parts, u.Permissions.describe())
	for _, s := range u.Selectors {
		parts = append(parts, "("+s.describe()+")")
	}
	return strings.Join(parts, " ")
}

// describe renders the selector as Redis' ACLDescribeSelector does: keys,
// channels, then command rules.
func (p *Permission) describe() string {
	parts := make([]string, 0, 4)
	if keys := p.describeKeys(); keys != "" {
		parts = append(parts, keys)
	}
	if p.allChannels() {
		parts = append(parts, "&*")
	} else {
		parts = append(parts, "resetchannels")
		for _, ch := range p.AllowedChannels {
			parts = append(parts, "&"+ch)
		}
	}
	parts = append(parts, p.commandRules())
	return strings.Join(parts, " ")
}

func (p *Permission) describeKeys() string {
	if p.allKeys() {
		return "~*"
	}
	keys := make([]string, len(p.Keys))
	for i, k := range p.Keys {
		switch k.Flags {
		case KeyRead:
			keys[i] = "%R~" + k.Pattern
		case KeyWrite:
			keys[i] = "%W~" + k.Pattern
		default:
			keys[i] = "~" + k.Pattern
		}
	}
	return strings.Join(keys, " ")
}

func (p *Permission) describeChannels() string {
	if p.allChannels() {
		return "&*"
	}
	chans := make([]string, len(p.AllowedChannels))
	for i, c := range p.AllowedChannels {
		chans[i] = "&" + c
	}
	return strings.Join(chans, " ")
}

// SelectorDescription is one selector as ACL GETUSER reports it.
type SelectorDescription struct {
	Commands string
	Keys     string
	Channels string
}

// UserDescription is what ACL GETUSER reports for a user.
type UserDescription struct {
	Flags     []string
	Passwords []string
	Commands  string
	Keys      string
	Channels  string
	Selectors []SelectorDescription
}

// Describe returns the user's flags, password hashes and permissions as
// ACL GETUSER reports them.
func (u *User) Describe() UserDescription {
	u.mu.RLock()
	defer u.mu.RUnlock()
	d := UserDescription{
		Flags:     u.flagsLocked(),
		Passwords: append([]string{}, u.Passwords...),
		Commands:  u.Permissions.commandRules(),
		Keys:      u.Permissions.describeKeys(),
		Channels:  u.Permissions.describeChannels(),
	}
	for _, s := range u.Selectors {
		d.Selectors = append(d.Selectors, SelectorDescription{
			Commands: s.commandRules(),
			Keys:     s.describeKeys(),
			Channels: s.describeChannels(),
		})
	}
	return d
}

func (u *User) flagsLocked() []string {
	flags := make([]string, 0, 3)
	if u.Enabled {
		flags = append(flags, "on")
	} else {
		flags = append(flags, "off")
	}
	if u.NoPassword {
		flags = append(flags, "nopass")
	}
	if u.SkipSanitizePayload {
		flags = append(flags, "skip-sanitize-payload")
	} else {
		flags = append(flags, "sanitize-payload")
	}
	return flags
}

// commandRules renders the command rules as Redis'
// ACLDescribeSelectorCommandRules does: +@all or -@all, then the rule
// history.
func (p *Permission) commandRules() string {
	base := "-@all"
	if p.AllowedCommands["*"] {
		base = "+@all"
	}
	if len(p.CommandRules) == 0 {
		return base
	}
	return base + " " + strings.Join(p.CommandRules, " ")
}

func hasPattern(patterns []string, p string) bool {
	for _, have := range patterns {
		if have == p {
			return true
		}
	}
	return false
}

// ParseACLRule applies the space-separated rules in rule to user.
func ParseACLRule(rule string, user *User) error {
	return ParseACLRules(strings.Fields(rule), user)
}

// MergeSelectorArgs joins the arguments of a "(...)" selector that spans
// several arguments into one, as Redis' ACLMergeSelectorArguments does.
func MergeSelectorArgs(parts []string) ([]string, error) {
	merged := make([]string, 0, len(parts))
	open := -1
	var sel string
	for i, part := range parts {
		if open == -1 && strings.HasPrefix(part, "(") && !strings.HasSuffix(part, ")") {
			open, sel = i, part
			continue
		}
		if open != -1 {
			sel += " " + part
			if strings.HasSuffix(part, ")") {
				merged = append(merged, sel)
				open = -1
			}
			continue
		}
		merged = append(merged, part)
	}
	if open != -1 {
		return nil, errors.New("ERR Unmatched parenthesis in acl selector starting at '" + parts[open] + "'.")
	}
	return merged, nil
}

// ValidateACLRules checks every rule without applying any, so a SETUSER with
// one bad rule changes nothing.
func ValidateACLRules(parts []string) error {
	_, err := mergeAndValidate(parts)
	return err
}

func mergeAndValidate(parts []string) ([]string, error) {
	parts, err := MergeSelectorArgs(parts)
	if err != nil {
		return nil, err
	}
	for _, part := range parts {
		if err := validateACLRule(part); err != nil {
			return nil, err
		}
	}
	return parts, nil
}

func ruleError(part, why string) error {
	return errors.New("ERR Error in ACL SETUSER modifier '" + part + "': " + why)
}

const (
	errSyntax        = "Syntax error"
	errUnknownCmd    = "Unknown command or category name in ACL"
	errFirstArgSub   = "Allowing first-arg of a subcommand is not supported"
	errBadHash       = "The password hash must be exactly 64 characters and contain only lowercase hexadecimal characters"
	errNoSuchPass    = "The password you are trying to remove from the user does not exist"
	errAfterAllKeys  = "Adding a pattern after the * pattern (or the 'allkeys' flag) is not valid and does not have any effect. Try 'resetkeys' to start with an empty list of patterns"
	errAfterAllChans = "Adding a pattern after the * pattern (or the 'allchannels' flag) is not valid and does not have any effect. Try 'resetchannels' to start with an empty list of channels"
)

func validateACLRule(part string) error {
	switch strings.ToLower(part) {
	case "on", "off", "nopass", "resetpass", "reset", "sanitize-payload",
		"skip-sanitize-payload", "clearselectors":
		return nil
	}
	if part == "" {
		return ruleError(part, errSyntax)
	}
	switch part[0] {
	case '>', '<':
		return nil
	case '#', '!':
		if !isPasswordHash(part[1:]) {
			return ruleError(part, errBadHash)
		}
		return nil
	case '(':
		if !strings.HasSuffix(part, ")") {
			return ruleError(part, errSyntax)
		}
		for _, rule := range strings.Fields(part[1 : len(part)-1]) {
			if why := validateSelectorRule(rule); why != "" {
				return ruleError(part, why)
			}
		}
		return nil
	}
	if why := validateSelectorRule(part); why != "" {
		return ruleError(part, why)
	}
	return nil
}

// validateSelectorRule checks a rule a selector accepts and returns why it
// is invalid, or "".
func validateSelectorRule(part string) string {
	switch strings.ToLower(part) {
	case "resetkeys", "resetchannels", "allkeys", "allchannels", "allcommands", "nocommands":
		return ""
	}
	if part == "" {
		return errSyntax
	}
	switch part[0] {
	case '~', '&':
		return ""
	case '%':
		if _, _, ok := parseKeyPermission(part); !ok {
			return errSyntax
		}
		return ""
	case '+', '-':
		if len(part) == 1 {
			return errSyntax
		}
		if part[1] == '@' {
			cat := strings.ToLower(part[2:])
			if cat == "all" {
				return ""
			}
			if categoryResolver == nil {
				return errUnknownCmd
			}
			if _, ok := categoryResolver(cat); !ok {
				return errUnknownCmd
			}
			return ""
		}
		return commandRuleError(part[0] == '+', part[1:])
	}
	return errSyntax
}

// parseKeyPermission parses %R~pattern, %W~pattern or %RW~pattern.
func parseKeyPermission(part string) (string, uint8, bool) {
	var flags uint8
	for i := 1; i < len(part); i++ {
		switch {
		case (part[i] == 'R' || part[i] == 'r') && flags&KeyRead == 0:
			flags |= KeyRead
		case (part[i] == 'W' || part[i] == 'w') && flags&KeyWrite == 0:
			flags |= KeyWrite
		case part[i] == '~':
			if flags == 0 || i == len(part)-1 {
				return "", 0, false
			}
			return part[i+1:], flags, true
		default:
			return "", 0, false
		}
	}
	return "", 0, false
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

// ParseACLRules applies rules to user with Redis' ACL SETUSER semantics:
// the rules are applied to a copy of the user, which replaces it only when
// every rule succeeded.
func ParseACLRules(parts []string, user *User) error {
	parts, err := mergeAndValidate(parts)
	if err != nil {
		return err
	}
	user.mu.Lock()
	defer user.mu.Unlock()
	work := user.copyLocked()
	for _, part := range parts {
		if why := work.applyRule(part); why != "" {
			return ruleError(part, why)
		}
	}
	user.Enabled, user.NoPassword, user.SkipSanitizePayload = work.Enabled, work.NoPassword, work.SkipSanitizePayload
	user.Passwords, user.Permissions, user.Selectors = work.Passwords, work.Permissions, work.Selectors
	return nil
}

func (u *User) copyLocked() *User {
	c := &User{
		Name:                u.Name,
		Enabled:             u.Enabled,
		NoPassword:          u.NoPassword,
		SkipSanitizePayload: u.SkipSanitizePayload,
		Passwords:           append([]string(nil), u.Passwords...),
		Permissions:         u.Permissions.clone(),
	}
	for _, s := range u.Selectors {
		cs := s.clone()
		c.Selectors = append(c.Selectors, &cs)
	}
	return c
}

func (p *Permission) clone() Permission {
	c := Permission{
		AllowedCommands: make(map[string]bool, len(p.AllowedCommands)),
		DeniedCommands:  make(map[string]bool, len(p.DeniedCommands)),
		Keys:            append([]KeyPattern(nil), p.Keys...),
		AllowedChannels: append([]string(nil), p.AllowedChannels...),
		CommandRules:    append([]string(nil), p.CommandRules...),
	}
	for k, v := range p.AllowedCommands {
		c.AllowedCommands[k] = v
	}
	for k, v := range p.DeniedCommands {
		c.DeniedCommands[k] = v
	}
	return c
}

// applyRule applies one rule to an unshared user and returns why it failed,
// or "".
func (u *User) applyRule(part string) string {
	switch strings.ToLower(part) {
	case "on":
		u.Enabled = true
		return ""
	case "off":
		u.Enabled = false
		return ""
	case "nopass":
		u.Passwords, u.NoPassword = nil, true
		return ""
	case "resetpass":
		u.Passwords, u.NoPassword = nil, false
		return ""
	case "sanitize-payload":
		u.SkipSanitizePayload = false
		return ""
	case "skip-sanitize-payload":
		u.SkipSanitizePayload = true
		return ""
	case "clearselectors":
		u.Selectors = nil
		return ""
	case "reset":
		u.Passwords, u.NoPassword, u.Enabled, u.SkipSanitizePayload = nil, false, false, false
		u.Selectors = nil
		u.Permissions.Keys, u.Permissions.AllowedChannels = nil, nil
		u.Permissions.setCommand("*", false)
		return ""
	}
	switch part[0] {
	case '>':
		u.addHash(hashPassword(part[1:]))
		return ""
	case '#':
		u.addHash(part[1:])
		return ""
	case '<':
		return u.removeHash(hashPassword(part[1:]))
	case '!':
		return u.removeHash(part[1:])
	case '(':
		sel := Permission{AllowedCommands: map[string]bool{}, DeniedCommands: map[string]bool{}}
		for _, rule := range strings.Fields(part[1 : len(part)-1]) {
			if why := sel.applyRule(rule); why != "" {
				return why
			}
		}
		u.Selectors = append(u.Selectors, &sel)
		return ""
	}
	return u.Permissions.applyRule(part)
}

// applyRule applies one selector rule and returns why it failed, or "".
func (p *Permission) applyRule(part string) string {
	switch strings.ToLower(part) {
	case "allcommands":
		p.setCommand("*", true)
		return ""
	case "nocommands":
		p.setCommand("*", false)
		return ""
	case "resetkeys":
		p.Keys = nil
		return ""
	case "resetchannels":
		p.AllowedChannels = nil
		return ""
	case "allkeys":
		p.Keys = []KeyPattern{{Pattern: "*", Flags: KeyReadWrite}}
		return ""
	case "allchannels":
		p.AllowedChannels = []string{"*"}
		return ""
	}
	switch part[0] {
	case '~', '%':
		pattern, flags := part[1:], KeyReadWrite
		if part[0] == '%' {
			pattern, flags, _ = parseKeyPermission(part)
		}
		if p.allKeys() {
			return errAfterAllKeys
		}
		if pattern == "*" && flags == KeyReadWrite {
			p.Keys = []KeyPattern{{Pattern: "*", Flags: KeyReadWrite}}
			return ""
		}
		for i := range p.Keys {
			if p.Keys[i].Pattern == pattern {
				p.Keys[i].Flags |= flags
				return ""
			}
		}
		p.Keys = append(p.Keys, KeyPattern{Pattern: pattern, Flags: flags})
	case '&':
		if p.allChannels() {
			return errAfterAllChans
		}
		if part[1:] == "*" {
			p.AllowedChannels = []string{"*"}
		} else if !hasPattern(p.AllowedChannels, part[1:]) {
			p.AllowedChannels = append(p.AllowedChannels, part[1:])
		}
	case '+', '-':
		allow := part[0] == '+'
		cmd := part[1:]
		if strings.HasPrefix(cmd, "@") {
			cat := strings.ToLower(cmd[1:])
			if cat != "all" {
				members, _ := categoryResolver(cat)
				for _, m := range members {
					p.setCommandBits(m, allow)
				}
				p.recordRule("@"+cat, allow)
				return ""
			}
			cmd = "*"
		}
		p.setCommand(cmd, allow)
	default:
		return errSyntax
	}
	return ""
}

func (u *User) addHash(h string) {
	u.NoPassword = false
	for _, stored := range u.Passwords {
		if stored == h {
			return
		}
	}
	u.Passwords = append(u.Passwords, h)
}

func (u *User) removeHash(h string) string {
	for i, stored := range u.Passwords {
		if stored == h {
			u.Passwords = append(u.Passwords[:i:i], u.Passwords[i+1:]...)
			return ""
		}
	}
	return errNoSuchPass
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
