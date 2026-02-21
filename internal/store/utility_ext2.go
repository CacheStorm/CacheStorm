package store

import (
	"sync"
	"time"
)

type AuditLog struct {
	Entries []*AuditEntry
	MaxSize int
	Enabled bool
	mu      sync.RWMutex
}

type AuditEntry struct {
	ID        int64
	Timestamp int64
	Command   string
	Key       string
	Args      []string
	ClientIP  string
	User      string
	Success   bool
	Duration  int64
}

func NewAuditLog(maxSize int) *AuditLog {
	if maxSize <= 0 {
		maxSize = 10000
	}
	return &AuditLog{
		Entries: make([]*AuditEntry, 0, maxSize),
		MaxSize: maxSize,
		Enabled: true,
	}
}

func (al *AuditLog) Log(command, key string, args []string, clientIP, user string, success bool, duration int64) int64 {
	al.mu.Lock()
	defer al.mu.Unlock()

	if !al.Enabled {
		return 0
	}

	entry := &AuditEntry{
		ID:        int64(len(al.Entries) + 1),
		Timestamp: time.Now().UnixMilli(),
		Command:   command,
		Key:       key,
		Args:      args,
		ClientIP:  clientIP,
		User:      user,
		Success:   success,
		Duration:  duration,
	}

	al.Entries = append(al.Entries, entry)

	if len(al.Entries) > al.MaxSize {
		al.Entries = al.Entries[len(al.Entries)-al.MaxSize:]
	}

	return entry.ID
}

func (al *AuditLog) Get(id int64) *AuditEntry {
	al.mu.RLock()
	defer al.mu.RUnlock()

	for _, e := range al.Entries {
		if e.ID == id {
			return e
		}
	}
	return nil
}

func (al *AuditLog) GetRange(start, end int64) []*AuditEntry {
	al.mu.RLock()
	defer al.mu.RUnlock()

	var result []*AuditEntry
	for _, e := range al.Entries {
		if e.Timestamp >= start && e.Timestamp <= end {
			result = append(result, e)
		}
	}
	return result
}

func (al *AuditLog) GetByCommand(cmd string, limit int) []*AuditEntry {
	al.mu.RLock()
	defer al.mu.RUnlock()

	var result []*AuditEntry
	for i := len(al.Entries) - 1; i >= 0 && len(result) < limit; i-- {
		if al.Entries[i].Command == cmd {
			result = append(result, al.Entries[i])
		}
	}
	return result
}

func (al *AuditLog) GetByKey(key string, limit int) []*AuditEntry {
	al.mu.RLock()
	defer al.mu.RUnlock()

	var result []*AuditEntry
	for i := len(al.Entries) - 1; i >= 0 && len(result) < limit; i-- {
		if al.Entries[i].Key == key {
			result = append(result, al.Entries[i])
		}
	}
	return result
}

func (al *AuditLog) Clear() {
	al.mu.Lock()
	defer al.mu.Unlock()
	al.Entries = al.Entries[:0]
}

func (al *AuditLog) Count() int64 {
	al.mu.RLock()
	defer al.mu.RUnlock()
	return int64(len(al.Entries))
}

func (al *AuditLog) Stats() map[string]interface{} {
	al.mu.RLock()
	defer al.mu.RUnlock()

	success := int64(0)
	failed := int64(0)
	cmdCount := make(map[string]int64)

	for _, e := range al.Entries {
		if e.Success {
			success++
		} else {
			failed++
		}
		cmdCount[e.Command]++
	}

	return map[string]interface{}{
		"total":    int64(len(al.Entries)),
		"success":  success,
		"failed":   failed,
		"enabled":  al.Enabled,
		"max_size": al.MaxSize,
		"commands": cmdCount,
	}
}

type FeatureFlag struct {
	Name        string
	Enabled     bool
	Description string
	Rules       []FeatureRule
	Variants    map[string]string
	CreatedAt   int64
	UpdatedAt   int64
	mu          sync.RWMutex
}

type FeatureRule struct {
	Attribute string
	Operator  string
	Value     string
}

type FeatureFlagManager struct {
	Flags map[string]*FeatureFlag
	mu    sync.RWMutex
}

func NewFeatureFlagManager() *FeatureFlagManager {
	return &FeatureFlagManager{
		Flags: make(map[string]*FeatureFlag),
	}
}

func (ffm *FeatureFlagManager) Create(name, description string) *FeatureFlag {
	ffm.mu.Lock()
	defer ffm.mu.Unlock()

	now := time.Now().UnixMilli()
	flag := &FeatureFlag{
		Name:        name,
		Enabled:     false,
		Description: description,
		Rules:       make([]FeatureRule, 0),
		Variants:    make(map[string]string),
		CreatedAt:   now,
		UpdatedAt:   now,
	}

	ffm.Flags[name] = flag
	return flag
}

func (ffm *FeatureFlagManager) Get(name string) (*FeatureFlag, bool) {
	ffm.mu.RLock()
	defer ffm.mu.RUnlock()
	flag, ok := ffm.Flags[name]
	return flag, ok
}

func (ffm *FeatureFlagManager) Delete(name string) bool {
	ffm.mu.Lock()
	defer ffm.mu.Unlock()

	if _, exists := ffm.Flags[name]; !exists {
		return false
	}
	delete(ffm.Flags, name)
	return true
}

func (ffm *FeatureFlagManager) Enable(name string) bool {
	ffm.mu.Lock()
	defer ffm.mu.Unlock()

	if flag, ok := ffm.Flags[name]; ok {
		flag.Enabled = true
		flag.UpdatedAt = time.Now().UnixMilli()
		return true
	}
	return false
}

func (ffm *FeatureFlagManager) Disable(name string) bool {
	ffm.mu.Lock()
	defer ffm.mu.Unlock()

	if flag, ok := ffm.Flags[name]; ok {
		flag.Enabled = false
		flag.UpdatedAt = time.Now().UnixMilli()
		return true
	}
	return false
}

func (ffm *FeatureFlagManager) IsEnabled(name string) bool {
	ffm.mu.RLock()
	defer ffm.mu.RUnlock()

	if flag, ok := ffm.Flags[name]; ok {
		return flag.Enabled
	}
	return false
}

func (ffm *FeatureFlagManager) Toggle(name string) bool {
	ffm.mu.Lock()
	defer ffm.mu.Unlock()

	if flag, ok := ffm.Flags[name]; ok {
		flag.Enabled = !flag.Enabled
		flag.UpdatedAt = time.Now().UnixMilli()
		return true
	}
	return false
}

func (ffm *FeatureFlagManager) List() []string {
	ffm.mu.RLock()
	defer ffm.mu.RUnlock()

	names := make([]string, 0, len(ffm.Flags))
	for name := range ffm.Flags {
		names = append(names, name)
	}
	return names
}

func (ffm *FeatureFlagManager) ListEnabled() []string {
	ffm.mu.RLock()
	defer ffm.mu.RUnlock()

	names := make([]string, 0)
	for name, flag := range ffm.Flags {
		if flag.Enabled {
			names = append(names, name)
		}
	}
	return names
}

func (ffm *FeatureFlagManager) AddVariant(name, key, value string) bool {
	ffm.mu.Lock()
	defer ffm.mu.Unlock()

	if flag, ok := ffm.Flags[name]; ok {
		flag.Variants[key] = value
		flag.UpdatedAt = time.Now().UnixMilli()
		return true
	}
	return false
}

func (ffm *FeatureFlagManager) GetVariant(name, key string) (string, bool) {
	ffm.mu.RLock()
	defer ffm.mu.RUnlock()

	if flag, ok := ffm.Flags[name]; ok {
		val, exists := flag.Variants[key]
		return val, exists
	}
	return "", false
}

func (ffm *FeatureFlagManager) AddRule(name string, rule FeatureRule) bool {
	ffm.mu.Lock()
	defer ffm.mu.Unlock()

	if flag, ok := ffm.Flags[name]; ok {
		flag.Rules = append(flag.Rules, rule)
		flag.UpdatedAt = time.Now().UnixMilli()
		return true
	}
	return false
}

type AtomicCounter struct {
	Counters map[string]int64
	mu       sync.RWMutex
}

func NewAtomicCounter() *AtomicCounter {
	return &AtomicCounter{
		Counters: make(map[string]int64),
	}
}

func (ac *AtomicCounter) Get(name string) int64 {
	ac.mu.RLock()
	defer ac.mu.RUnlock()
	return ac.Counters[name]
}

func (ac *AtomicCounter) Set(name string, value int64) {
	ac.mu.Lock()
	defer ac.mu.Unlock()
	ac.Counters[name] = value
}

func (ac *AtomicCounter) Increment(name string, delta int64) int64 {
	ac.mu.Lock()
	defer ac.mu.Unlock()
	ac.Counters[name] += delta
	return ac.Counters[name]
}

func (ac *AtomicCounter) Decrement(name string, delta int64) int64 {
	return ac.Increment(name, -delta)
}

func (ac *AtomicCounter) Delete(name string) bool {
	ac.mu.Lock()
	defer ac.mu.Unlock()

	if _, exists := ac.Counters[name]; !exists {
		return false
	}
	delete(ac.Counters, name)
	return true
}

func (ac *AtomicCounter) List() []string {
	ac.mu.RLock()
	defer ac.mu.RUnlock()

	names := make([]string, 0, len(ac.Counters))
	for name := range ac.Counters {
		names = append(names, name)
	}
	return names
}

func (ac *AtomicCounter) GetAll() map[string]int64 {
	ac.mu.RLock()
	defer ac.mu.RUnlock()

	result := make(map[string]int64, len(ac.Counters))
	for k, v := range ac.Counters {
		result[k] = v
	}
	return result
}

func (ac *AtomicCounter) Reset(name string) bool {
	ac.mu.Lock()
	defer ac.mu.Unlock()

	if _, exists := ac.Counters[name]; !exists {
		return false
	}
	ac.Counters[name] = 0
	return true
}

func (ac *AtomicCounter) ResetAll() {
	ac.mu.Lock()
	defer ac.mu.Unlock()
	ac.Counters = make(map[string]int64)
}

var (
	GlobalAuditLog      = NewAuditLog(10000)
	GlobalFeatureFlags  = NewFeatureFlagManager()
	GlobalAtomicCounter = NewAtomicCounter()
)
