package store

import (
	"sync"
	"time"
)

type Workflow struct {
	ID          string
	Name        string
	Steps       []WorkflowStep
	CurrentStep int
	Status      WorkflowStatus
	Variables   map[string]string
	StartedAt   int64
	CompletedAt int64
	Error       string
}

type WorkflowStep struct {
	ID      string
	Name    string
	Command string
	Args    []string
	Timeout int64
	OnFail  string
}

type WorkflowStatus int

const (
	WorkflowPending WorkflowStatus = iota
	WorkflowRunning
	WorkflowCompleted
	WorkflowFailed
	WorkflowPaused
)

func (s WorkflowStatus) String() string {
	switch s {
	case WorkflowPending:
		return "pending"
	case WorkflowRunning:
		return "running"
	case WorkflowCompleted:
		return "completed"
	case WorkflowFailed:
		return "failed"
	case WorkflowPaused:
		return "paused"
	default:
		return "unknown"
	}
}

type WorkflowManager struct {
	Workflows map[string]*Workflow
	Templates map[string]*WorkflowTemplate
	mu        sync.RWMutex
}

type WorkflowTemplate struct {
	Name  string
	Steps []WorkflowStep
}

func NewWorkflowManager() *WorkflowManager {
	return &WorkflowManager{
		Workflows: make(map[string]*Workflow),
		Templates: make(map[string]*WorkflowTemplate),
	}
}

func (wm *WorkflowManager) CreateTemplate(name string, steps []WorkflowStep) *WorkflowTemplate {
	wm.mu.Lock()
	defer wm.mu.Unlock()

	template := &WorkflowTemplate{
		Name:  name,
		Steps: steps,
	}

	wm.Templates[name] = template
	return template
}

func (wm *WorkflowManager) GetTemplate(name string) (*WorkflowTemplate, bool) {
	wm.mu.RLock()
	defer wm.mu.RUnlock()
	t, ok := wm.Templates[name]
	return t, ok
}

func (wm *WorkflowManager) DeleteTemplate(name string) bool {
	wm.mu.Lock()
	defer wm.mu.Unlock()

	if _, exists := wm.Templates[name]; !exists {
		return false
	}
	delete(wm.Templates, name)
	return true
}

func (wm *WorkflowManager) CreateFromTemplate(templateName, id string) (*Workflow, error) {
	wm.mu.Lock()
	defer wm.mu.Unlock()

	template, exists := wm.Templates[templateName]
	if !exists {
		return nil, ErrTemplateNotFound
	}

	steps := make([]WorkflowStep, len(template.Steps))
	copy(steps, template.Steps)

	workflow := &Workflow{
		ID:        id,
		Name:      templateName,
		Steps:     steps,
		Status:    WorkflowPending,
		Variables: make(map[string]string),
	}

	wm.Workflows[id] = workflow
	return workflow, nil
}

func (wm *WorkflowManager) Create(id, name string, steps []WorkflowStep) *Workflow {
	wm.mu.Lock()
	defer wm.mu.Unlock()

	workflow := &Workflow{
		ID:        id,
		Name:      name,
		Steps:     steps,
		Status:    WorkflowPending,
		Variables: make(map[string]string),
	}

	wm.Workflows[id] = workflow
	return workflow
}

func (wm *WorkflowManager) Get(id string) (*Workflow, bool) {
	wm.mu.RLock()
	defer wm.mu.RUnlock()
	w, ok := wm.Workflows[id]
	return w, ok
}

func (wm *WorkflowManager) Delete(id string) bool {
	wm.mu.Lock()
	defer wm.mu.Unlock()

	if _, exists := wm.Workflows[id]; !exists {
		return false
	}
	delete(wm.Workflows, id)
	return true
}

func (wm *WorkflowManager) Start(id string) error {
	wm.mu.Lock()
	defer wm.mu.Unlock()

	workflow, exists := wm.Workflows[id]
	if !exists {
		return ErrWorkflowNotFound
	}

	if workflow.Status != WorkflowPending && workflow.Status != WorkflowPaused {
		return ErrWorkflowInvalidState
	}

	workflow.Status = WorkflowRunning
	workflow.StartedAt = time.Now().UnixMilli()

	return nil
}

func (wm *WorkflowManager) Pause(id string) error {
	wm.mu.Lock()
	defer wm.mu.Unlock()

	workflow, exists := wm.Workflows[id]
	if !exists {
		return ErrWorkflowNotFound
	}

	if workflow.Status != WorkflowRunning {
		return ErrWorkflowInvalidState
	}

	workflow.Status = WorkflowPaused

	return nil
}

func (wm *WorkflowManager) Complete(id string) error {
	wm.mu.Lock()
	defer wm.mu.Unlock()

	workflow, exists := wm.Workflows[id]
	if !exists {
		return ErrWorkflowNotFound
	}

	workflow.Status = WorkflowCompleted
	workflow.CompletedAt = time.Now().UnixMilli()

	return nil
}

func (wm *WorkflowManager) Fail(id, errMsg string) error {
	wm.mu.Lock()
	defer wm.mu.Unlock()

	workflow, exists := wm.Workflows[id]
	if !exists {
		return ErrWorkflowNotFound
	}

	workflow.Status = WorkflowFailed
	workflow.Error = errMsg
	workflow.CompletedAt = time.Now().UnixMilli()

	return nil
}

func (wm *WorkflowManager) Reset(id string) error {
	wm.mu.Lock()
	defer wm.mu.Unlock()

	workflow, exists := wm.Workflows[id]
	if !exists {
		return ErrWorkflowNotFound
	}

	workflow.Status = WorkflowPending
	workflow.CurrentStep = 0
	workflow.StartedAt = 0
	workflow.CompletedAt = 0
	workflow.Error = ""
	workflow.Variables = make(map[string]string)

	return nil
}

func (wm *WorkflowManager) NextStep(id string) (WorkflowStep, error) {
	wm.mu.Lock()
	defer wm.mu.Unlock()

	workflow, exists := wm.Workflows[id]
	if !exists {
		return WorkflowStep{}, ErrWorkflowNotFound
	}

	if workflow.CurrentStep >= len(workflow.Steps) {
		return WorkflowStep{}, ErrNoMoreSteps
	}

	step := workflow.Steps[workflow.CurrentStep]
	workflow.CurrentStep++

	return step, nil
}

func (wm *WorkflowManager) SetVariable(id, key, value string) error {
	wm.mu.Lock()
	defer wm.mu.Unlock()

	workflow, exists := wm.Workflows[id]
	if !exists {
		return ErrWorkflowNotFound
	}

	workflow.Variables[key] = value
	return nil
}

func (wm *WorkflowManager) GetVariable(id, key string) (string, bool) {
	wm.mu.RLock()
	defer wm.mu.RUnlock()

	workflow, exists := wm.Workflows[id]
	if !exists {
		return "", false
	}

	val, ok := workflow.Variables[key]
	return val, ok
}

func (wm *WorkflowManager) List() []*Workflow {
	wm.mu.RLock()
	defer wm.mu.RUnlock()

	workflows := make([]*Workflow, 0, len(wm.Workflows))
	for _, w := range wm.Workflows {
		workflows = append(workflows, w)
	}
	return workflows
}

func (wm *WorkflowManager) ListByStatus(status WorkflowStatus) []*Workflow {
	wm.mu.RLock()
	defer wm.mu.RUnlock()

	workflows := make([]*Workflow, 0)
	for _, w := range wm.Workflows {
		if w.Status == status {
			workflows = append(workflows, w)
		}
	}
	return workflows
}

func (w *Workflow) Info() map[string]interface{} {
	return map[string]interface{}{
		"id":           w.ID,
		"name":         w.Name,
		"status":       w.Status.String(),
		"current_step": w.CurrentStep,
		"total_steps":  len(w.Steps),
		"started_at":   w.StartedAt,
		"completed_at": w.CompletedAt,
		"error":        w.Error,
	}
}

type StateMachine struct {
	Name        string
	States      map[string]State
	Initial     string
	Current     string
	Transitions []Transition
	mu          sync.RWMutex
}

type State struct {
	Name    string
	Final   bool
	OnEnter string
	OnExit  string
}

type Transition struct {
	From  string
	To    string
	Event string
}

func NewStateMachine(name, initial string) *StateMachine {
	return &StateMachine{
		Name:    name,
		States:  make(map[string]State),
		Initial: initial,
		Current: initial,
	}
}

func (sm *StateMachine) AddState(name string, final bool, onEnter, onExit string) {
	sm.mu.Lock()
	defer sm.mu.Unlock()

	sm.States[name] = State{
		Name:    name,
		Final:   final,
		OnEnter: onEnter,
		OnExit:  onExit,
	}
}

func (sm *StateMachine) AddTransition(from, to, event string) {
	sm.mu.Lock()
	defer sm.mu.Unlock()

	sm.Transitions = append(sm.Transitions, Transition{
		From:  from,
		To:    to,
		Event: event,
	})
}

func (sm *StateMachine) Trigger(event string) (string, error) {
	sm.mu.Lock()
	defer sm.mu.Unlock()

	for _, t := range sm.Transitions {
		if t.From == sm.Current && t.Event == event {
			sm.Current = t.To
			return sm.Current, nil
		}
	}

	return sm.Current, ErrInvalidTransition
}

func (sm *StateMachine) GetCurrentState() string {
	sm.mu.RLock()
	defer sm.mu.RUnlock()
	return sm.Current
}

func (sm *StateMachine) CanTrigger(event string) bool {
	sm.mu.RLock()
	defer sm.mu.RUnlock()

	for _, t := range sm.Transitions {
		if t.From == sm.Current && t.Event == event {
			return true
		}
	}
	return false
}

func (sm *StateMachine) GetValidEvents() []string {
	sm.mu.RLock()
	defer sm.mu.RUnlock()

	events := make([]string, 0)
	for _, t := range sm.Transitions {
		if t.From == sm.Current {
			events = append(events, t.Event)
		}
	}
	return events
}

func (sm *StateMachine) Reset() {
	sm.mu.Lock()
	defer sm.mu.Unlock()
	sm.Current = sm.Initial
}

func (sm *StateMachine) IsFinal() bool {
	sm.mu.RLock()
	defer sm.mu.RUnlock()

	if state, ok := sm.States[sm.Current]; ok {
		return state.Final
	}
	return false
}

func (sm *StateMachine) Info() map[string]interface{} {
	sm.mu.RLock()
	defer sm.mu.RUnlock()

	return map[string]interface{}{
		"name":     sm.Name,
		"current":  sm.Current,
		"initial":  sm.Initial,
		"is_final": sm.IsFinal(),
	}
}

var (
	ErrTemplateNotFound     = StoreError("template not found")
	ErrWorkflowNotFound     = StoreError("workflow not found")
	ErrWorkflowInvalidState = StoreError("invalid workflow state")
	ErrNoMoreSteps          = StoreError("no more steps")
	ErrInvalidTransition    = StoreError("invalid transition")
)

var (
	GlobalWorkflowManager = NewWorkflowManager()
	stateMachines         = make(map[string]*StateMachine)
	stateMachinesMu       sync.RWMutex
)

func GetOrCreateStateMachine(name, initial string) *StateMachine {
	stateMachinesMu.Lock()
	defer stateMachinesMu.Unlock()

	if sm, exists := stateMachines[name]; exists {
		return sm
	}

	sm := NewStateMachine(name, initial)
	stateMachines[name] = sm
	return sm
}

func GetStateMachine(name string) (*StateMachine, bool) {
	stateMachinesMu.RLock()
	defer stateMachinesMu.RUnlock()
	sm, ok := stateMachines[name]
	return sm, ok
}

func DeleteStateMachine(name string) bool {
	stateMachinesMu.Lock()
	defer stateMachinesMu.Unlock()

	if _, exists := stateMachines[name]; !exists {
		return false
	}
	delete(stateMachines, name)
	return true
}

func ListStateMachines() []string {
	stateMachinesMu.RLock()
	defer stateMachinesMu.RUnlock()

	names := make([]string, 0, len(stateMachines))
	for name := range stateMachines {
		names = append(names, name)
	}
	return names
}
