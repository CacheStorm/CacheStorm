package store

import (
	"sync"
	"time"
)

type Job struct {
	ID         string
	Name       string
	Command    string
	Interval   time.Duration
	LastRun    time.Time
	NextRun    time.Time
	Runs       int64
	Errors     int64
	Enabled    bool
	PausedAt   time.Time
	CreatedAt  time.Time
	LastResult string
	LastError  string
}

type JobScheduler struct {
	Jobs    map[string]*Job
	mu      sync.RWMutex
	stopCh  chan struct{}
	running bool
}

func NewJobScheduler() *JobScheduler {
	return &JobScheduler{
		Jobs:   make(map[string]*Job),
		stopCh: make(chan struct{}),
	}
}

func (js *JobScheduler) Create(id, name, command string, interval time.Duration) *Job {
	js.mu.Lock()
	defer js.mu.Unlock()

	now := time.Now()
	job := &Job{
		ID:        id,
		Name:      name,
		Command:   command,
		Interval:  interval,
		NextRun:   now.Add(interval),
		CreatedAt: now,
		Enabled:   true,
	}

	js.Jobs[id] = job
	return job
}

func (js *JobScheduler) Get(id string) (*Job, bool) {
	js.mu.RLock()
	defer js.mu.RUnlock()
	job, ok := js.Jobs[id]
	return job, ok
}

func (js *JobScheduler) Delete(id string) bool {
	js.mu.Lock()
	defer js.mu.Unlock()

	if _, exists := js.Jobs[id]; !exists {
		return false
	}
	delete(js.Jobs, id)
	return true
}

func (js *JobScheduler) Enable(id string) bool {
	js.mu.Lock()
	defer js.mu.Unlock()

	job, exists := js.Jobs[id]
	if !exists {
		return false
	}
	job.Enabled = true
	job.PausedAt = time.Time{}
	return true
}

func (js *JobScheduler) Disable(id string) bool {
	js.mu.Lock()
	defer js.mu.Unlock()

	job, exists := js.Jobs[id]
	if !exists {
		return false
	}
	job.Enabled = false
	job.PausedAt = time.Now()
	return true
}

func (js *JobScheduler) Run(id string) (string, error) {
	js.mu.Lock()
	defer js.mu.Unlock()

	job, exists := js.Jobs[id]
	if !exists {
		return "", ErrJobNotFound
	}

	job.LastRun = time.Now()
	job.NextRun = job.LastRun.Add(job.Interval)
	job.Runs++

	return "OK", nil
}

func (js *JobScheduler) List() []*Job {
	js.mu.RLock()
	defer js.mu.RUnlock()

	jobs := make([]*Job, 0, len(js.Jobs))
	for _, job := range js.Jobs {
		jobs = append(jobs, job)
	}
	return jobs
}

func (js *JobScheduler) ListEnabled() []*Job {
	js.mu.RLock()
	defer js.mu.RUnlock()

	jobs := make([]*Job, 0)
	for _, job := range js.Jobs {
		if job.Enabled {
			jobs = append(jobs, job)
		}
	}
	return jobs
}

func (js *JobScheduler) UpdateInterval(id string, interval time.Duration) bool {
	js.mu.Lock()
	defer js.mu.Unlock()

	job, exists := js.Jobs[id]
	if !exists {
		return false
	}
	job.Interval = interval
	job.NextRun = job.LastRun.Add(interval)
	return true
}

func (js *JobScheduler) Stats(id string) map[string]interface{} {
	js.mu.RLock()
	defer js.mu.RUnlock()

	job, exists := js.Jobs[id]
	if !exists {
		return nil
	}

	return map[string]interface{}{
		"id":         job.ID,
		"name":       job.Name,
		"command":    job.Command,
		"interval":   job.Interval.Milliseconds(),
		"last_run":   job.LastRun.Unix(),
		"next_run":   job.NextRun.Unix(),
		"runs":       job.Runs,
		"errors":     job.Errors,
		"enabled":    job.Enabled,
		"created_at": job.CreatedAt.Unix(),
	}
}

func (js *JobScheduler) Reset(id string) bool {
	js.mu.Lock()
	defer js.mu.Unlock()

	job, exists := js.Jobs[id]
	if !exists {
		return false
	}
	job.Runs = 0
	job.Errors = 0
	job.LastRun = time.Time{}
	job.NextRun = time.Now().Add(job.Interval)
	job.LastResult = ""
	job.LastError = ""
	return true
}

var ErrJobNotFound = error(StoreError("job not found"))

type StoreError string

func (e StoreError) Error() string { return string(e) }

type CircuitBreaker struct {
	Name             string
	State            CircuitState
	Failures         int64
	Successes        int64
	LastFailure      time.Time
	LastSuccess      time.Time
	LastStateChange  time.Time
	FailureThreshold int
	SuccessThreshold int
	Timeout          time.Duration
	mu               sync.RWMutex
}

type CircuitState int

const (
	CircuitClosed CircuitState = iota
	CircuitOpen
	CircuitHalfOpen
)

func (s CircuitState) String() string {
	switch s {
	case CircuitClosed:
		return "closed"
	case CircuitOpen:
		return "open"
	case CircuitHalfOpen:
		return "half-open"
	default:
		return "unknown"
	}
}

func NewCircuitBreaker(name string, failureThreshold, successThreshold int, timeout time.Duration) *CircuitBreaker {
	return &CircuitBreaker{
		Name:             name,
		State:            CircuitClosed,
		FailureThreshold: failureThreshold,
		SuccessThreshold: successThreshold,
		Timeout:          timeout,
		LastStateChange:  time.Now(),
	}
}

func (cb *CircuitBreaker) Allow() bool {
	cb.mu.Lock()
	defer cb.mu.Unlock()

	switch cb.State {
	case CircuitClosed:
		return true
	case CircuitOpen:
		if time.Since(cb.LastFailure) > cb.Timeout {
			cb.State = CircuitHalfOpen
			cb.LastStateChange = time.Now()
			cb.Successes = 0
			return true
		}
		return false
	case CircuitHalfOpen:
		return true
	default:
		return false
	}
}

func (cb *CircuitBreaker) RecordSuccess() {
	cb.mu.Lock()
	defer cb.mu.Unlock()

	cb.Successes++
	cb.LastSuccess = time.Now()

	if cb.State == CircuitHalfOpen && int(cb.Successes) >= cb.SuccessThreshold {
		cb.State = CircuitClosed
		cb.LastStateChange = time.Now()
		cb.Failures = 0
	}
}

func (cb *CircuitBreaker) RecordFailure() {
	cb.mu.Lock()
	defer cb.mu.Unlock()

	cb.Failures++
	cb.LastFailure = time.Now()

	if cb.State == CircuitHalfOpen {
		cb.State = CircuitOpen
		cb.LastStateChange = time.Now()
	} else if cb.State == CircuitClosed && int(cb.Failures) >= cb.FailureThreshold {
		cb.State = CircuitOpen
		cb.LastStateChange = time.Now()
	}
}

func (cb *CircuitBreaker) GetState() CircuitState {
	cb.mu.RLock()
	defer cb.mu.RUnlock()
	return cb.State
}

func (cb *CircuitBreaker) Reset() {
	cb.mu.Lock()
	defer cb.mu.Unlock()
	cb.State = CircuitClosed
	cb.Failures = 0
	cb.Successes = 0
	cb.LastStateChange = time.Now()
}

func (cb *CircuitBreaker) Stats() map[string]interface{} {
	cb.mu.RLock()
	defer cb.mu.RUnlock()

	return map[string]interface{}{
		"name":              cb.Name,
		"state":             cb.State.String(),
		"failures":          cb.Failures,
		"successes":         cb.Successes,
		"failure_threshold": cb.FailureThreshold,
		"success_threshold": cb.SuccessThreshold,
		"timeout_ms":        cb.Timeout.Milliseconds(),
		"last_failure":      cb.LastFailure.Unix(),
		"last_success":      cb.LastSuccess.Unix(),
		"last_state_change": cb.LastStateChange.Unix(),
	}
}

type Session struct {
	ID        string
	Data      map[string]string
	CreatedAt time.Time
	UpdatedAt time.Time
	ExpiresAt time.Time
	mu        sync.RWMutex
}

func NewSession(id string, ttl time.Duration) *Session {
	now := time.Now()
	return &Session{
		ID:        id,
		Data:      make(map[string]string),
		CreatedAt: now,
		UpdatedAt: now,
		ExpiresAt: now.Add(ttl),
	}
}

func (s *Session) Get(key string) (string, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	val, ok := s.Data[key]
	return val, ok
}

func (s *Session) Set(key, value string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.Data[key] = value
	s.UpdatedAt = time.Now()
}

func (s *Session) Delete(key string) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, exists := s.Data[key]; !exists {
		return false
	}
	delete(s.Data, key)
	s.UpdatedAt = time.Now()
	return true
}

func (s *Session) GetAll() map[string]string {
	s.mu.RLock()
	defer s.mu.RUnlock()
	result := make(map[string]string, len(s.Data))
	for k, v := range s.Data {
		result[k] = v
	}
	return result
}

func (s *Session) Clear() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.Data = make(map[string]string)
	s.UpdatedAt = time.Now()
}

func (s *Session) Refresh(ttl time.Duration) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.ExpiresAt = time.Now().Add(ttl)
	s.UpdatedAt = time.Now()
}

func (s *Session) IsExpired() bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return time.Now().After(s.ExpiresAt)
}

func (s *Session) TTL() time.Duration {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return time.Until(s.ExpiresAt)
}

type SessionManager struct {
	Sessions map[string]*Session
	mu       sync.RWMutex
}

func NewSessionManager() *SessionManager {
	return &SessionManager{
		Sessions: make(map[string]*Session),
	}
}

func (sm *SessionManager) Create(id string, ttl time.Duration) *Session {
	sm.mu.Lock()
	defer sm.mu.Unlock()
	session := NewSession(id, ttl)
	sm.Sessions[id] = session
	return session
}

func (sm *SessionManager) Get(id string) (*Session, bool) {
	sm.mu.RLock()
	defer sm.mu.RUnlock()
	session, ok := sm.Sessions[id]
	if !ok {
		return nil, false
	}
	if session.IsExpired() {
		return nil, false
	}
	return session, true
}

func (sm *SessionManager) Delete(id string) bool {
	sm.mu.Lock()
	defer sm.mu.Unlock()
	if _, exists := sm.Sessions[id]; !exists {
		return false
	}
	delete(sm.Sessions, id)
	return true
}

func (sm *SessionManager) Exists(id string) bool {
	sm.mu.RLock()
	defer sm.mu.RUnlock()
	session, ok := sm.Sessions[id]
	if !ok {
		return false
	}
	return !session.IsExpired()
}

func (sm *SessionManager) Count() int64 {
	sm.mu.RLock()
	defer sm.mu.RUnlock()
	var count int64
	for _, session := range sm.Sessions {
		if !session.IsExpired() {
			count++
		}
	}
	return count
}

func (sm *SessionManager) Cleanup() int64 {
	sm.mu.Lock()
	defer sm.mu.Unlock()
	var cleaned int64
	for id, session := range sm.Sessions {
		if session.IsExpired() {
			delete(sm.Sessions, id)
			cleaned++
		}
	}
	return cleaned
}

func (sm *SessionManager) List() []string {
	sm.mu.RLock()
	defer sm.mu.RUnlock()
	ids := make([]string, 0, len(sm.Sessions))
	for id, session := range sm.Sessions {
		if !session.IsExpired() {
			ids = append(ids, id)
		}
	}
	return ids
}

var (
	GlobalJobScheduler   = NewJobScheduler()
	GlobalSessionManager = NewSessionManager()
	circuitBreakers      = make(map[string]*CircuitBreaker)
	circuitBreakersMu    sync.RWMutex
)

func GetOrCreateCircuitBreaker(name string, failureThreshold, successThreshold int, timeout time.Duration) *CircuitBreaker {
	circuitBreakersMu.Lock()
	defer circuitBreakersMu.Unlock()

	if cb, exists := circuitBreakers[name]; exists {
		return cb
	}

	cb := NewCircuitBreaker(name, failureThreshold, successThreshold, timeout)
	circuitBreakers[name] = cb
	return cb
}

func GetCircuitBreaker(name string) (*CircuitBreaker, bool) {
	circuitBreakersMu.RLock()
	defer circuitBreakersMu.RUnlock()
	cb, ok := circuitBreakers[name]
	return cb, ok
}

func DeleteCircuitBreaker(name string) bool {
	circuitBreakersMu.Lock()
	defer circuitBreakersMu.Unlock()
	if _, exists := circuitBreakers[name]; !exists {
		return false
	}
	delete(circuitBreakers, name)
	return true
}

func ListCircuitBreakers() []string {
	circuitBreakersMu.RLock()
	defer circuitBreakersMu.RUnlock()
	names := make([]string, 0, len(circuitBreakers))
	for name := range circuitBreakers {
		names = append(names, name)
	}
	return names
}
