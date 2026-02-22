package store

import (
	"testing"
	"time"
)

func TestWorkflowStatusString(t *testing.T) {
	tests := []struct {
		status   WorkflowStatus
		expected string
	}{
		{WorkflowPending, "pending"},
		{WorkflowRunning, "running"},
		{WorkflowCompleted, "completed"},
		{WorkflowFailed, "failed"},
		{WorkflowPaused, "paused"},
		{WorkflowStatus(99), "unknown"},
	}

	for _, tt := range tests {
		if got := tt.status.String(); got != tt.expected {
			t.Errorf("WorkflowStatus(%d).String() = %s, want %s", tt.status, got, tt.expected)
		}
	}
}

func TestWorkflowManager(t *testing.T) {
	wm := NewWorkflowManager()

	t.Run("Template", func(t *testing.T) {
		steps := []WorkflowStep{
			{ID: "step1", Name: "First", Command: "cmd1"},
			{ID: "step2", Name: "Second", Command: "cmd2"},
		}

		template := wm.CreateTemplate("test-template", steps)
		if template == nil {
			t.Fatal("template should not be nil")
		}
		if template.Name != "test-template" {
			t.Errorf("template name = %s, want test-template", template.Name)
		}

		got, ok := wm.GetTemplate("test-template")
		if !ok {
			t.Error("template should exist")
		}
		if got.Name != "test-template" {
			t.Errorf("template name = %s, want test-template", got.Name)
		}

		_, ok = wm.GetTemplate("nonexistent")
		if ok {
			t.Error("nonexistent template should not exist")
		}

		if !wm.DeleteTemplate("test-template") {
			t.Error("delete should succeed")
		}
		if wm.DeleteTemplate("test-template") {
			t.Error("second delete should fail")
		}
	})

	t.Run("Workflow", func(t *testing.T) {
		steps := []WorkflowStep{
			{ID: "step1", Name: "First", Command: "cmd1"},
		}

		workflow := wm.Create("wf1", "test-workflow", steps)
		if workflow == nil {
			t.Fatal("workflow should not be nil")
		}

		got, ok := wm.Get("wf1")
		if !ok {
			t.Error("workflow should exist")
		}
		if got.ID != "wf1" {
			t.Errorf("workflow id = %s, want wf1", got.ID)
		}

		_, ok = wm.Get("nonexistent")
		if ok {
			t.Error("nonexistent workflow should not exist")
		}
	})

	t.Run("Start", func(t *testing.T) {
		steps := []WorkflowStep{{ID: "step1"}}
		workflow := wm.Create("wf2", "test", steps)

		if err := wm.Start("wf2"); err != nil {
			t.Errorf("start should succeed: %v", err)
		}
		if workflow.Status != WorkflowRunning {
			t.Errorf("status = %d, want %d", workflow.Status, WorkflowRunning)
		}

		if err := wm.Start("nonexistent"); err != ErrWorkflowNotFound {
			t.Errorf("start nonexistent should return ErrWorkflowNotFound: %v", err)
		}

		if err := wm.Start("wf2"); err != ErrWorkflowInvalidState {
			t.Errorf("start running workflow should return ErrWorkflowInvalidState: %v", err)
		}
	})

	t.Run("Pause", func(t *testing.T) {
		wm.Create("wf3", "test", []WorkflowStep{{ID: "s1"}})
		wm.Start("wf3")

		if err := wm.Pause("wf3"); err != nil {
			t.Errorf("pause should succeed: %v", err)
		}

		wm.Create("wf4", "test", []WorkflowStep{{ID: "s1"}})
		if err := wm.Pause("wf4"); err != ErrWorkflowInvalidState {
			t.Errorf("pause pending workflow should return ErrWorkflowInvalidState: %v", err)
		}

		if err := wm.Pause("nonexistent"); err != ErrWorkflowNotFound {
			t.Errorf("pause nonexistent should return ErrWorkflowNotFound: %v", err)
		}
	})

	t.Run("Complete", func(t *testing.T) {
		wm.Create("wf5", "test", []WorkflowStep{{ID: "s1"}})

		if err := wm.Complete("wf5"); err != nil {
			t.Errorf("complete should succeed: %v", err)
		}

		if err := wm.Complete("nonexistent"); err != ErrWorkflowNotFound {
			t.Errorf("complete nonexistent should return ErrWorkflowNotFound: %v", err)
		}
	})

	t.Run("Fail", func(t *testing.T) {
		wm.Create("wf6", "test", []WorkflowStep{{ID: "s1"}})

		if err := wm.Fail("wf6", "test error"); err != nil {
			t.Errorf("fail should succeed: %v", err)
		}
		if wf, _ := wm.Get("wf6"); wf.Error != "test error" {
			t.Errorf("error = %s, want test error", wf.Error)
		}

		if err := wm.Fail("nonexistent", "err"); err != ErrWorkflowNotFound {
			t.Errorf("fail nonexistent should return ErrWorkflowNotFound: %v", err)
		}
	})

	t.Run("Reset", func(t *testing.T) {
		wm.Create("wf7", "test", []WorkflowStep{{ID: "s1"}})
		wm.Start("wf7")
		wm.SetVariable("wf7", "key", "value")

		if err := wm.Reset("wf7"); err != nil {
			t.Errorf("reset should succeed: %v", err)
		}
		wf, _ := wm.Get("wf7")
		if wf.Status != WorkflowPending {
			t.Errorf("status = %d, want %d", wf.Status, WorkflowPending)
		}
		if wf.CurrentStep != 0 {
			t.Errorf("current step = %d, want 0", wf.CurrentStep)
		}

		if err := wm.Reset("nonexistent"); err != ErrWorkflowNotFound {
			t.Errorf("reset nonexistent should return ErrWorkflowNotFound: %v", err)
		}
	})

	t.Run("NextStep", func(t *testing.T) {
		steps := []WorkflowStep{
			{ID: "step1", Name: "First"},
			{ID: "step2", Name: "Second"},
		}
		wm.Create("wf8", "test", steps)

		step, err := wm.NextStep("wf8")
		if err != nil {
			t.Errorf("next step should succeed: %v", err)
		}
		if step.ID != "step1" {
			t.Errorf("step id = %s, want step1", step.ID)
		}

		step, err = wm.NextStep("wf8")
		if err != nil {
			t.Errorf("next step should succeed: %v", err)
		}
		if step.ID != "step2" {
			t.Errorf("step id = %s, want step2", step.ID)
		}

		_, err = wm.NextStep("wf8")
		if err != ErrNoMoreSteps {
			t.Errorf("no more steps should return ErrNoMoreSteps: %v", err)
		}

		_, err = wm.NextStep("nonexistent")
		if err != ErrWorkflowNotFound {
			t.Errorf("next step nonexistent should return ErrWorkflowNotFound: %v", err)
		}
	})

	t.Run("Variables", func(t *testing.T) {
		wm.Create("wf9", "test", []WorkflowStep{{ID: "s1"}})

		if err := wm.SetVariable("wf9", "key1", "value1"); err != nil {
			t.Errorf("set variable should succeed: %v", err)
		}

		val, ok := wm.GetVariable("wf9", "key1")
		if !ok {
			t.Error("variable should exist")
		}
		if val != "value1" {
			t.Errorf("value = %s, want value1", val)
		}

		_, ok = wm.GetVariable("wf9", "nonexistent")
		if ok {
			t.Error("nonexistent variable should not exist")
		}

		_, ok = wm.GetVariable("nonexistent", "key")
		if ok {
			t.Error("variable from nonexistent workflow should not exist")
		}

		if err := wm.SetVariable("nonexistent", "key", "val"); err != ErrWorkflowNotFound {
			t.Errorf("set variable nonexistent should return ErrWorkflowNotFound: %v", err)
		}
	})

	t.Run("List", func(t *testing.T) {
		wm.Create("list1", "test", []WorkflowStep{{ID: "s1"}})
		wm.Create("list2", "test", []WorkflowStep{{ID: "s1"}})

		list := wm.List()
		if len(list) < 2 {
			t.Errorf("list length = %d, want at least 2", len(list))
		}
	})

	t.Run("ListByStatus", func(t *testing.T) {
		wm.Create("status1", "test", []WorkflowStep{{ID: "s1"}})
		wm.Create("status2", "test", []WorkflowStep{{ID: "s1"}})
		wm.Start("status2")
		wm.Complete("status2")

		pending := wm.ListByStatus(WorkflowPending)
		completed := wm.ListByStatus(WorkflowCompleted)

		if len(pending) < 1 {
			t.Error("should have at least 1 pending")
		}
		if len(completed) < 1 {
			t.Error("should have at least 1 completed")
		}
	})

	t.Run("Delete", func(t *testing.T) {
		wm.Create("del1", "test", []WorkflowStep{{ID: "s1"}})

		if !wm.Delete("del1") {
			t.Error("delete should succeed")
		}
		if wm.Delete("del1") {
			t.Error("second delete should fail")
		}
	})

	t.Run("CreateFromTemplate", func(t *testing.T) {
		steps := []WorkflowStep{{ID: "s1", Name: "Step1"}}
		wm.CreateTemplate("tpl1", steps)

		wf, err := wm.CreateFromTemplate("tpl1", "wf-from-tpl")
		if err != nil {
			t.Errorf("create from template should succeed: %v", err)
		}
		if wf == nil {
			t.Fatal("workflow should not be nil")
		}
		if wf.Name != "tpl1" {
			t.Errorf("workflow name = %s, want tpl1", wf.Name)
		}

		_, err = wm.CreateFromTemplate("nonexistent", "wf2")
		if err != ErrTemplateNotFound {
			t.Errorf("create from nonexistent template should return ErrTemplateNotFound: %v", err)
		}
	})

	t.Run("WorkflowInfo", func(t *testing.T) {
		wf := &Workflow{
			ID:          "test",
			Name:        "Test Workflow",
			Status:      WorkflowRunning,
			CurrentStep: 1,
			Steps:       []WorkflowStep{{ID: "s1"}, {ID: "s2"}},
			StartedAt:   1000,
			CompletedAt: 2000,
			Error:       "none",
		}

		info := wf.Info()
		if info["id"] != "test" {
			t.Errorf("id = %v, want test", info["id"])
		}
		if info["status"] != "running" {
			t.Errorf("status = %v, want running", info["status"])
		}
	})
}

func TestStateMachine(t *testing.T) {
	t.Run("Basic", func(t *testing.T) {
		sm := NewStateMachine("test-sm", "initial")

		sm.AddState("initial", false, "onEnter1", "onExit1")
		sm.AddState("final", true, "onEnter2", "onExit2")
		sm.AddTransition("initial", "final", "complete")

		if sm.GetCurrentState() != "initial" {
			t.Errorf("current state = %s, want initial", sm.GetCurrentState())
		}

		if !sm.CanTrigger("complete") {
			t.Error("should be able to trigger complete")
		}
		if sm.CanTrigger("nonexistent") {
			t.Error("should not be able to trigger nonexistent event")
		}

		events := sm.GetValidEvents()
		if len(events) != 1 || events[0] != "complete" {
			t.Errorf("valid events = %v, want [complete]", events)
		}

		newState, err := sm.Trigger("complete")
		if err != nil {
			t.Errorf("trigger should succeed: %v", err)
		}
		if newState != "final" {
			t.Errorf("new state = %s, want final", newState)
		}

		_, err = sm.Trigger("complete")
		if err != ErrInvalidTransition {
			t.Errorf("invalid transition should return ErrInvalidTransition: %v", err)
		}

		if !sm.IsFinal() {
			t.Error("should be in final state")
		}
	})

	t.Run("Reset", func(t *testing.T) {
		sm := NewStateMachine("test", "initial")
		sm.AddTransition("initial", "done", "go")
		sm.Trigger("go")

		sm.Reset()
		if sm.GetCurrentState() != "initial" {
			t.Errorf("state = %s, want initial", sm.GetCurrentState())
		}
	})

	t.Run("Info", func(t *testing.T) {
		sm := NewStateMachine("test", "start")
		info := sm.Info()

		if info["name"] != "test" {
			t.Errorf("name = %v, want test", info["name"])
		}
		if info["current"] != "start" {
			t.Errorf("current = %v, want start", info["current"])
		}
	})
}

func TestGlobalStateMachines(t *testing.T) {
	sm1 := GetOrCreateStateMachine("global1", "init")
	if sm1 == nil {
		t.Fatal("state machine should not be nil")
	}

	sm2 := GetOrCreateStateMachine("global1", "init")
	if sm1 != sm2 {
		t.Error("should return same state machine")
	}

	sm3, ok := GetStateMachine("global1")
	if !ok || sm3 == nil {
		t.Error("state machine should exist")
	}

	_, ok = GetStateMachine("nonexistent")
	if ok {
		t.Error("nonexistent state machine should not exist")
	}

	list := ListStateMachines()
	if len(list) < 1 {
		t.Error("should have at least 1 state machine")
	}

	if !DeleteStateMachine("global1") {
		t.Error("delete should succeed")
	}
	if DeleteStateMachine("global1") {
		t.Error("second delete should fail")
	}
}

func TestGlobalWorkflowManager(t *testing.T) {
	if GlobalWorkflowManager == nil {
		t.Fatal("GlobalWorkflowManager should not be nil")
	}
}

func TestTimeSeriesValue(t *testing.T) {
	ts := NewTimeSeriesValue(time.Hour)

	t.Run("Type", func(t *testing.T) {
		if ts.Type() != DataTypeString {
			t.Errorf("type = %v, want %v", ts.Type(), DataTypeString)
		}
	})

	t.Run("SizeOf", func(t *testing.T) {
		ts.Add(1000, 1.0)
		size := ts.SizeOf()
		if size <= 0 {
			t.Errorf("size = %d, want > 0", size)
		}
	})

	t.Run("String", func(t *testing.T) {
		if ts.String() != "timeseries" {
			t.Errorf("string = %s, want timeseries", ts.String())
		}
	})

	t.Run("Clone", func(t *testing.T) {
		ts.Add(2000, 2.0)
		cloned := ts.Clone()
		if cloned == nil {
			t.Fatal("clone should not be nil")
		}
		clonedTs := cloned.(*TimeSeriesValue)
		if len(clonedTs.Samples) != len(ts.Samples) {
			t.Errorf("samples length = %d, want %d", len(clonedTs.Samples), len(ts.Samples))
		}
	})

	t.Run("Add", func(t *testing.T) {
		ts2 := NewTimeSeriesValue(0)
		ts := ts2

		ts.Add(3000, 3.0)
		if len(ts.Samples) == 0 {
			t.Error("should have samples")
		}

		ts.Add(0, 4.0)
		last := ts.Samples[len(ts.Samples)-1]
		if last.Timestamp == 0 {
			t.Error("timestamp should not be 0")
		}
	})

	t.Run("AddWithLabels", func(t *testing.T) {
		ts2 := NewTimeSeriesValue(0)
		labels := map[string]string{"sensor": "temp1"}
		ts2.AddWithLabels(4000, 4.0, labels)

		if len(ts2.Samples) == 0 {
			t.Fatal("should have samples")
		}
		if ts2.Samples[len(ts2.Samples)-1].Labels["sensor"] != "temp1" {
			t.Error("label should be set")
		}
		if ts2.Labels["sensor"] != "temp1" {
			t.Error("label should be in series labels")
		}
	})

	t.Run("Range", func(t *testing.T) {
		ts3 := NewTimeSeriesValue(0)
		ts3.Add(1000, 1.0)
		ts3.Add(2000, 2.0)
		ts3.Add(3000, 3.0)

		samples := ts3.Range(1500, 2500)
		if len(samples) != 1 {
			t.Errorf("samples length = %d, want 1", len(samples))
		}
		if samples[0].Value != 2.0 {
			t.Errorf("value = %f, want 2.0", samples[0].Value)
		}
	})

	t.Run("RangeWithCount", func(t *testing.T) {
		ts3 := NewTimeSeriesValue(0)
		ts3.Add(1000, 1.0)
		ts3.Add(2000, 2.0)
		ts3.Add(3000, 3.0)

		samples := ts3.RangeWithCount(0, 4000, 2)
		if len(samples) != 2 {
			t.Errorf("samples length = %d, want 2", len(samples))
		}
	})

	t.Run("Get", func(t *testing.T) {
		ts3 := NewTimeSeriesValue(0)
		ts3.Add(1000, 1.0)

		sample := ts3.Get(1000)
		if sample == nil {
			t.Fatal("sample should not be nil")
		}
		if sample.Value != 1.0 {
			t.Errorf("value = %f, want 1.0", sample.Value)
		}

		if ts3.Get(9999) != nil {
			t.Error("nonexistent sample should be nil")
		}
	})

	t.Run("Latest", func(t *testing.T) {
		ts3 := NewTimeSeriesValue(0)

		if ts3.Latest() != nil {
			t.Error("empty series latest should be nil")
		}

		ts3.Add(1000, 1.0)
		ts3.Add(3000, 3.0)
		ts3.Add(2000, 2.0)

		latest := ts3.Latest()
		if latest == nil || latest.Value != 3.0 {
			t.Errorf("latest value = %v, want 3.0", latest)
		}
	})

	t.Run("First", func(t *testing.T) {
		ts3 := NewTimeSeriesValue(0)

		if ts3.First() != nil {
			t.Error("empty series first should be nil")
		}

		ts3.Add(3000, 3.0)
		ts3.Add(1000, 1.0)
		ts3.Add(2000, 2.0)

		first := ts3.First()
		if first == nil || first.Value != 1.0 {
			t.Errorf("first value = %v, want 1.0", first)
		}
	})

	t.Run("Delete", func(t *testing.T) {
		ts3 := NewTimeSeriesValue(0)
		ts3.Add(1000, 1.0)
		ts3.Add(2000, 2.0)

		deleted := ts3.Delete(1000)
		if deleted != 1 {
			t.Errorf("deleted = %d, want 1", deleted)
		}
		if ts3.Get(1000) != nil {
			t.Error("sample should be deleted")
		}
	})

	t.Run("Len", func(t *testing.T) {
		ts3 := NewTimeSeriesValue(0)
		if ts3.Len() != 0 {
			t.Errorf("length = %d, want 0", ts3.Len())
		}
		ts3.Add(1000, 1.0)
		if ts3.Len() != 1 {
			t.Errorf("length = %d, want 1", ts3.Len())
		}
	})

	t.Run("Setters", func(t *testing.T) {
		ts3 := NewTimeSeriesValue(0)
		ts3.SetRetention(2 * time.Hour)
		ts3.SetLabels(map[string]string{"key": "value"})

		labels := ts3.GetLabels()
		if labels["key"] != "value" {
			t.Errorf("label = %s, want value", labels["key"])
		}
	})

	t.Run("Aggregation", func(t *testing.T) {
		ts3 := NewTimeSeriesValue(0)
		ts3.Add(1000, 1.0)
		ts3.Add(1100, 2.0)
		ts3.Add(2000, 3.0)
		ts3.Add(2100, 4.0)

		agg := ts3.Aggregation(0, 3000, "avg", 1000)
		if len(agg) == 0 {
			t.Fatal("aggregation should have results")
		}

		aggSum := ts3.Aggregation(0, 3000, "sum", 1000)
		aggMin := ts3.Aggregation(0, 3000, "min", 1000)
		aggMax := ts3.Aggregation(0, 3000, "max", 1000)
		aggCount := ts3.Aggregation(0, 3000, "count", 1000)
		aggFirst := ts3.Aggregation(0, 3000, "first", 1000)
		aggLast := ts3.Aggregation(0, 3000, "last", 1000)
		aggDefault := ts3.Aggregation(0, 3000, "unknown", 1000)

		if len(aggSum) == 0 || len(aggMin) == 0 || len(aggMax) == 0 || len(aggCount) == 0 || len(aggFirst) == 0 || len(aggLast) == 0 || len(aggDefault) == 0 {
			t.Error("all aggregations should have results")
		}

		emptyTs := NewTimeSeriesValue(0)
		if emptyTs.Aggregation(0, 1000, "avg", 100) != nil {
			t.Error("empty aggregation should be nil")
		}
	})
}

func TestTimeSeriesManager(t *testing.T) {
	m := NewTimeSeriesManager()

	t.Run("Create", func(t *testing.T) {
		labels := map[string]string{"sensor": "temp"}
		err := m.Create("key1", time.Hour, labels)
		if err != nil {
			t.Errorf("create should succeed: %v", err)
		}

		err = m.Create("key1", time.Hour, labels)
		if err != nil {
			t.Errorf("create duplicate should return nil: %v", err)
		}
	})

	t.Run("Get", func(t *testing.T) {
		ts, ok := m.Get("key1")
		if !ok || ts == nil {
			t.Error("should get timeseries")
		}

		_, ok = m.Get("nonexistent")
		if ok {
			t.Error("nonexistent should not exist")
		}
	})

	t.Run("QueryByLabels", func(t *testing.T) {
		m.Create("key2", time.Hour, map[string]string{"type": "cpu", "host": "server1"})
		m.Create("key3", time.Hour, map[string]string{"type": "cpu", "host": "server2"})

		keys := m.QueryByLabels(map[string]string{"type": "cpu"}, "")
		if len(keys) != 2 {
			t.Errorf("keys length = %d, want 2", len(keys))
		}

		keys = m.QueryByLabels(map[string]string{"host": "server1"}, "")
		if len(keys) != 1 {
			t.Errorf("keys length = %d, want 1", len(keys))
		}
	})

	t.Run("Delete", func(t *testing.T) {
		if !m.Delete("key2") {
			t.Error("delete should succeed")
		}
		if m.Delete("key2") {
			t.Error("second delete should fail")
		}
		if m.Delete("nonexistent") {
			t.Error("delete nonexistent should fail")
		}
	})
}

func TestEventManager(t *testing.T) {
	em := NewEventManager()

	t.Run("Emit", func(t *testing.T) {
		event := em.Emit("test-event", map[string]interface{}{"key": "value"})
		if event == nil {
			t.Fatal("event should not be nil")
		}
		if event.Name != "test-event" {
			t.Errorf("name = %s, want test-event", event.Name)
		}
		if event.ID == "" {
			t.Error("id should not be empty")
		}
	})

	t.Run("Subscribe", func(t *testing.T) {
		ch := em.Subscribe("sub-event")
		if ch == nil {
			t.Fatal("channel should not be nil")
		}

		em.Emit("sub-event", map[string]interface{}{"data": 123})

		select {
		case evt := <-ch:
			if evt.Name != "sub-event" {
				t.Errorf("event name = %s, want sub-event", evt.Name)
			}
		default:
			t.Error("should receive event")
		}

		em.Unsubscribe("sub-event", ch)
	})

	t.Run("GetEvents", func(t *testing.T) {
		em.Emit("get-test", nil)
		em.Emit("get-test", nil)
		em.Emit("other", nil)

		events := em.GetEvents("get-test", 10)
		if len(events) < 2 {
			t.Errorf("events length = %d, want >= 2", len(events))
		}

		allEvents := em.GetEvents("", 100)
		if len(allEvents) < 3 {
			t.Errorf("all events length = %d, want >= 3", len(allEvents))
		}
	})

	t.Run("Webhooks", func(t *testing.T) {
		wh := em.CreateWebhook("wh1", "http://example.com", "POST", []string{"event1", "event2"})
		if wh == nil {
			t.Fatal("webhook should not be nil")
		}

		got, ok := em.GetWebhook("wh1")
		if !ok || got == nil {
			t.Error("webhook should exist")
		}

		list := em.ListWebhooks()
		if len(list) == 0 {
			t.Error("should have webhooks")
		}

		wh.SetHeader("Content-Type", "application/json")
		if wh.Headers["Content-Type"] != "application/json" {
			t.Error("header should be set")
		}

		stats := wh.Stats()
		if stats["id"] != "wh1" {
			t.Errorf("stats id = %v, want wh1", stats["id"])
		}

		if !em.EnableWebhook("wh1") {
			t.Error("enable should succeed")
		}
		if !em.DisableWebhook("wh1") {
			t.Error("disable should succeed")
		}
		if em.EnableWebhook("nonexistent") {
			t.Error("enable nonexistent should fail")
		}
		if em.DisableWebhook("nonexistent") {
			t.Error("disable nonexistent should fail")
		}

		em.RecordWebhookHit("wh1", true)
		em.RecordWebhookHit("wh1", false)
		em.RecordWebhookHit("nonexistent", true)

		if !em.DeleteWebhook("wh1") {
			t.Error("delete should succeed")
		}
		if em.DeleteWebhook("wh1") {
			t.Error("second delete should fail")
		}
	})
}

func TestRLECompressor(t *testing.T) {
	c := &RLECompressor{}

	if c.Name() != "rle" {
		t.Errorf("name = %s, want rle", c.Name())
	}

	t.Run("CompressDecompress", func(t *testing.T) {
		data := []byte("aaabbbcccaaa")
		compressed, err := c.Compress(data)
		if err != nil {
			t.Errorf("compress should succeed: %v", err)
		}

		decompressed, err := c.Decompress(compressed)
		if err != nil {
			t.Errorf("decompress should succeed: %v", err)
		}

		if string(decompressed) != string(data) {
			t.Errorf("decompressed = %s, want %s", decompressed, data)
		}
	})

	t.Run("Empty", func(t *testing.T) {
		data := []byte{}
		compressed, _ := c.Compress(data)
		decompressed, _ := c.Decompress(compressed)
		if len(decompressed) != 0 {
			t.Error("empty should remain empty")
		}
	})
}

func TestLZ4Compressor(t *testing.T) {
	c := &LZ4Compressor{}

	if c.Name() != "lz4" {
		t.Errorf("name = %s, want lz4", c.Name())
	}

	t.Run("Empty", func(t *testing.T) {
		data := []byte{}
		compressed, _ := c.Compress(data)
		decompressed, _ := c.Decompress(compressed)
		if len(decompressed) != 0 {
			t.Error("empty should remain empty")
		}
	})

	t.Run("Compress", func(t *testing.T) {
		data := []byte("hello world")
		compressed, err := c.Compress(data)
		if err != nil {
			t.Errorf("compress should succeed: %v", err)
		}
		if len(compressed) == 0 {
			t.Error("compressed data should not be empty")
		}
	})
}

func TestRateLimiter(t *testing.T) {
	rl := NewRateLimiter()

	t.Run("Create", func(t *testing.T) {
		rl.Create("key1", 10, 2, time.Second)
		entry := rl.Requests["key1"]
		if entry == nil {
			t.Fatal("entry should not be nil")
		}
		if entry.MaxTokens != 10 {
			t.Errorf("max tokens = %d, want 10", entry.MaxTokens)
		}
	})

	t.Run("Allow", func(t *testing.T) {
		rl.Create("key2", 5, 1, time.Second)

		allowed, remaining, _ := rl.Allow("key2", 3)
		if !allowed {
			t.Error("should be allowed")
		}
		if remaining != 2 {
			t.Errorf("remaining = %d, want 2", remaining)
		}

		allowed, _, _ = rl.Allow("key2", 5)
		if allowed {
			t.Error("should not be allowed (not enough tokens)")
		}

		allowed, _, _ = rl.Allow("nonexistent", 1)
		if allowed {
			t.Error("nonexistent should not be allowed")
		}
	})

	t.Run("Get", func(t *testing.T) {
		rl.Create("key3", 10, 2, time.Second)

		tokens, max, rate, interval, exists := rl.Get("key3")
		if !exists {
			t.Error("should exist")
		}
		if tokens != 10 || max != 10 || rate != 2 || interval != time.Second {
			t.Errorf("unexpected values: tokens=%d, max=%d, rate=%d, interval=%v", tokens, max, rate, interval)
		}

		_, _, _, _, exists = rl.Get("nonexistent")
		if exists {
			t.Error("nonexistent should not exist")
		}
	})

	t.Run("Delete", func(t *testing.T) {
		rl.Create("key4", 10, 1, time.Second)

		if !rl.Delete("key4") {
			t.Error("delete should succeed")
		}
		if rl.Delete("key4") {
			t.Error("second delete should fail")
		}
	})

	t.Run("Reset", func(t *testing.T) {
		rl.Create("key5", 10, 1, time.Second)
		rl.Allow("key5", 5)

		if !rl.Reset("key5") {
			t.Error("reset should succeed")
		}
		tokens, _, _, _, _ := rl.Get("key5")
		if tokens != 10 {
			t.Errorf("tokens = %d, want 10", tokens)
		}

		if rl.Reset("nonexistent") {
			t.Error("reset nonexistent should fail")
		}
	})
}

func TestDistributedLock(t *testing.T) {
	dl := NewDistributedLock()

	t.Run("TryLock", func(t *testing.T) {
		if !dl.TryLock("lock1", "holder1", "token1", time.Second) {
			t.Error("try lock should succeed")
		}

		if dl.TryLock("lock1", "holder2", "token2", time.Second) {
			t.Error("try lock with different holder should fail")
		}

		if !dl.TryLock("lock1", "holder1", "token1", time.Second) {
			t.Error("try lock with same holder and token should succeed (renew)")
		}
	})

	t.Run("Unlock", func(t *testing.T) {
		if dl.Unlock("lock1", "holder2", "token2") {
			t.Error("unlock with wrong credentials should fail")
		}

		if !dl.Unlock("lock1", "holder1", "token1") {
			t.Error("unlock should succeed")
		}

		if dl.Unlock("lock1", "holder1", "token1") {
			t.Error("second unlock should fail")
		}

		if dl.Unlock("nonexistent", "h", "t") {
			t.Error("unlock nonexistent should fail")
		}
	})

	t.Run("Lock", func(t *testing.T) {
		dl.TryLock("lock2", "holder1", "token1", time.Second)

		if dl.Lock("lock2", "holder2", "token2", time.Second, 50*time.Millisecond) {
			t.Error("lock with timeout should fail when lock is held by another")
		}

		if !dl.Lock("lock2", "holder1", "token1", time.Second, 50*time.Millisecond) {
			t.Error("lock with same holder should succeed (renew)")
		}
	})

	t.Run("Renew", func(t *testing.T) {
		dl.TryLock("lock3", "holder1", "token1", time.Second)

		if !dl.Renew("lock3", "holder1", "token1", 2*time.Second) {
			t.Error("renew should succeed")
		}

		if dl.Renew("lock3", "holder2", "token2", time.Second) {
			t.Error("renew with wrong credentials should fail")
		}

		if dl.Renew("nonexistent", "h", "t", time.Second) {
			t.Error("renew nonexistent should fail")
		}
	})

	t.Run("GetHolder", func(t *testing.T) {
		dl.TryLock("lock4", "holder1", "token1", time.Second)

		holder, expires, exists := dl.GetHolder("lock4")
		if !exists {
			t.Error("lock should exist")
		}
		if holder != "holder1" {
			t.Errorf("holder = %s, want holder1", holder)
		}
		if expires.IsZero() {
			t.Error("expires should not be zero")
		}

		_, _, exists = dl.GetHolder("nonexistent")
		if exists {
			t.Error("nonexistent should not exist")
		}
	})

	t.Run("IsLocked", func(t *testing.T) {
		dl.TryLock("lock5", "h", "t", time.Second)

		if !dl.IsLocked("lock5") {
			t.Error("should be locked")
		}
		if dl.IsLocked("nonexistent") {
			t.Error("nonexistent should not be locked")
		}
	})
}

func TestIDGenerator(t *testing.T) {
	idg := NewIDGenerator()

	t.Run("Create", func(t *testing.T) {
		idg.Create("seq1", 1, 1, "PRE", "SUF", 5)

		seq, exists := idg.Sequences["seq1"]
		if !exists {
			t.Fatal("sequence should exist")
		}
		if seq.Prefix != "PRE" || seq.Suffix != "SUF" || seq.Padding != 5 {
			t.Errorf("unexpected sequence values")
		}
	})

	t.Run("Next", func(t *testing.T) {
		id, num, ok := idg.Next("seq1")
		if !ok {
			t.Error("next should succeed")
		}
		if num != 1 {
			t.Errorf("num = %d, want 1", num)
		}
		if id != "PRE00001SUF" {
			t.Errorf("id = %s, want PRE00001SUF", id)
		}

		_, _, ok = idg.Next("nonexistent")
		if ok {
			t.Error("next nonexistent should fail")
		}
	})

	t.Run("NextN", func(t *testing.T) {
		idg.Create("seq2", 1, 1, "", "", 0)

		ids, last, ok := idg.NextN("seq2", 3)
		if !ok {
			t.Error("next n should succeed")
		}
		if len(ids) != 3 {
			t.Errorf("ids length = %d, want 3", len(ids))
		}
		if last != 3 {
			t.Errorf("last = %d, want 3", last)
		}

		_, _, ok = idg.NextN("nonexistent", 3)
		if ok {
			t.Error("next n nonexistent should fail")
		}
	})

	t.Run("Current", func(t *testing.T) {
		idg.Create("seq3", 100, 1, "", "", 0)
		idg.Next("seq3")

		id, num, ok := idg.Current("seq3")
		if !ok {
			t.Error("current should succeed")
		}
		if num != 100 {
			t.Errorf("num = %d, want 100", num)
		}
		if id != "100" {
			t.Errorf("id = %s, want 100", id)
		}

		_, _, ok = idg.Current("nonexistent")
		if ok {
			t.Error("current nonexistent should fail")
		}
	})

	t.Run("Set", func(t *testing.T) {
		idg.Create("seq4", 1, 1, "", "", 0)

		if !idg.Set("seq4", 500) {
			t.Error("set should succeed")
		}
		_, num, _ := idg.Current("seq4")
		if num != 500 {
			t.Errorf("num = %d, want 500", num)
		}

		if idg.Set("nonexistent", 100) {
			t.Error("set nonexistent should fail")
		}
	})

	t.Run("Delete", func(t *testing.T) {
		idg.Create("seq5", 1, 1, "", "", 0)

		if !idg.Delete("seq5") {
			t.Error("delete should succeed")
		}
		if idg.Delete("seq5") {
			t.Error("second delete should fail")
		}
	})
}

func TestSnowflakeIDGenerator(t *testing.T) {
	t.Run("Next", func(t *testing.T) {
		gen := NewSnowflakeIDGenerator(1)

		id1 := gen.Next()
		id2 := gen.Next()

		if id1 == 0 {
			t.Error("id should not be 0")
		}
		if id1 == id2 {
			t.Error("ids should be unique")
		}
	})

	t.Run("Parse", func(t *testing.T) {
		gen := NewSnowflakeIDGenerator(5)
		id := gen.Next()

		parsed := gen.Parse(id)
		if parsed["node_id"] != 5 {
			t.Errorf("node_id = %d, want 5", parsed["node_id"])
		}
	})

	t.Run("InvalidNodeID", func(t *testing.T) {
		gen := NewSnowflakeIDGenerator(-1)
		if gen.nodeID != 0 {
			t.Errorf("node_id = %d, want 0", gen.nodeID)
		}

		gen = NewSnowflakeIDGenerator(99999)
		if gen.nodeID != 0 {
			t.Errorf("node_id for invalid should be 0, got %d", gen.nodeID)
		}
	})
}

func TestGlobalVariables(t *testing.T) {
	if GlobalRateLimiter == nil {
		t.Error("GlobalRateLimiter should not be nil")
	}
	if GlobalDistributedLock == nil {
		t.Error("GlobalDistributedLock should not be nil")
	}
	if GlobalIDGenerator == nil {
		t.Error("GlobalIDGenerator should not be nil")
	}
	if GlobalEventManager == nil {
		t.Error("GlobalEventManager should not be nil")
	}
}

func TestHelperFunctions(t *testing.T) {
	t.Run("generateID", func(t *testing.T) {
		id := generateID()
		if len(id) != 16 {
			t.Errorf("id length = %d, want 16", len(id))
		}
	})

	t.Run("absInt", func(t *testing.T) {
		if absInt(-5) != 5 {
			t.Error("absInt(-5) should be 5")
		}
		if absInt(5) != 5 {
			t.Error("absInt(5) should be 5")
		}
	})

	t.Run("min", func(t *testing.T) {
		if min(5, 10) != 5 {
			t.Error("min(5, 10) should be 5")
		}
		if min(10, 5) != 5 {
			t.Error("min(10, 5) should be 5")
		}
	})

	t.Run("formatNumber", func(t *testing.T) {
		if formatNumber(0, 0) != "0" {
			t.Errorf("formatNumber(0, 0) = %s, want 0", formatNumber(0, 0))
		}
		if formatNumber(123, 5) != "00123" {
			t.Errorf("formatNumber(123, 5) = %s, want 00123", formatNumber(123, 5))
		}
		if formatNumber(-123, 0) != "-123" {
			t.Errorf("formatNumber(-123, 0) = %s, want -123", formatNumber(-123, 0))
		}
	})
}

func TestJobScheduler(t *testing.T) {
	js := NewJobScheduler()

	t.Run("Create", func(t *testing.T) {
		job := js.Create("job1", "Test Job", "echo test", time.Minute)
		if job == nil {
			t.Fatal("job should not be nil")
		}
		if job.Name != "Test Job" {
			t.Errorf("name = %s, want Test Job", job.Name)
		}
	})

	t.Run("Get", func(t *testing.T) {
		job, ok := js.Get("job1")
		if !ok || job == nil {
			t.Error("job should exist")
		}

		_, ok = js.Get("nonexistent")
		if ok {
			t.Error("nonexistent job should not exist")
		}
	})

	t.Run("Delete", func(t *testing.T) {
		if !js.Delete("job1") {
			t.Error("delete should succeed")
		}
		if js.Delete("job1") {
			t.Error("second delete should fail")
		}
	})

	t.Run("Enable", func(t *testing.T) {
		js.Create("job2", "Test", "cmd", time.Minute)
		js.Disable("job2")

		if !js.Enable("job2") {
			t.Error("enable should succeed")
		}
		job, _ := js.Get("job2")
		if !job.Enabled {
			t.Error("job should be enabled")
		}

		if js.Enable("nonexistent") {
			t.Error("enable nonexistent should fail")
		}
	})

	t.Run("Disable", func(t *testing.T) {
		if !js.Disable("job2") {
			t.Error("disable should succeed")
		}
		job, _ := js.Get("job2")
		if job.Enabled {
			t.Error("job should be disabled")
		}

		if js.Disable("nonexistent") {
			t.Error("disable nonexistent should fail")
		}
	})

	t.Run("Run", func(t *testing.T) {
		js.Create("job3", "Test", "cmd", time.Minute)

		result, err := js.Run("job3")
		if err != nil {
			t.Errorf("run should succeed: %v", err)
		}
		if result != "OK" {
			t.Errorf("result = %s, want OK", result)
		}

		_, err = js.Run("nonexistent")
		if err != ErrJobNotFound {
			t.Errorf("run nonexistent should return ErrJobNotFound: %v", err)
		}
	})

	t.Run("List", func(t *testing.T) {
		js.Create("job4", "Test", "cmd", time.Minute)
		js.Create("job5", "Test", "cmd", time.Minute)

		list := js.List()
		if len(list) < 2 {
			t.Errorf("list length = %d, want >= 2", len(list))
		}
	})

	t.Run("ListEnabled", func(t *testing.T) {
		js.Create("job6", "Test", "cmd", time.Minute)
		js.Create("job7", "Test", "cmd", time.Minute)
		js.Disable("job7")

		enabled := js.ListEnabled()
		for _, j := range enabled {
			if !j.Enabled {
				t.Error("all listed jobs should be enabled")
			}
		}
	})

	t.Run("UpdateInterval", func(t *testing.T) {
		js.Create("job8", "Test", "cmd", time.Minute)

		if !js.UpdateInterval("job8", time.Hour) {
			t.Error("update interval should succeed")
		}
		job, _ := js.Get("job8")
		if job.Interval != time.Hour {
			t.Errorf("interval = %v, want 1h", job.Interval)
		}

		if js.UpdateInterval("nonexistent", time.Hour) {
			t.Error("update interval nonexistent should fail")
		}
	})

	t.Run("Stats", func(t *testing.T) {
		js.Create("job9", "Test", "cmd", time.Minute)

		stats := js.Stats("job9")
		if stats == nil {
			t.Fatal("stats should not be nil")
		}
		if stats["name"] != "Test" {
			t.Errorf("name = %v, want Test", stats["name"])
		}

		if js.Stats("nonexistent") != nil {
			t.Error("stats for nonexistent should be nil")
		}
	})

	t.Run("Reset", func(t *testing.T) {
		js.Create("job10", "Test", "cmd", time.Minute)
		js.Run("job10")

		if !js.Reset("job10") {
			t.Error("reset should succeed")
		}
		job, _ := js.Get("job10")
		if job.Runs != 0 {
			t.Errorf("runs = %d, want 0", job.Runs)
		}

		if js.Reset("nonexistent") {
			t.Error("reset nonexistent should fail")
		}
	})
}

func TestCircuitBreaker(t *testing.T) {
	t.Run("StateString", func(t *testing.T) {
		if CircuitClosed.String() != "closed" {
			t.Errorf("CircuitClosed.String() = %s, want closed", CircuitClosed.String())
		}
		if CircuitOpen.String() != "open" {
			t.Errorf("CircuitOpen.String() = %s, want open", CircuitOpen.String())
		}
		if CircuitHalfOpen.String() != "half-open" {
			t.Errorf("CircuitHalfOpen.String() = %s, want half-open", CircuitHalfOpen.String())
		}
		if CircuitState(99).String() != "unknown" {
			t.Errorf("Unknown state string = %s, want unknown", CircuitState(99).String())
		}
	})

	t.Run("Closed", func(t *testing.T) {
		cb := NewCircuitBreaker("test", 3, 2, time.Second)

		if !cb.Allow() {
			t.Error("closed circuit should allow")
		}
		if cb.GetState() != CircuitClosed {
			t.Error("state should be closed")
		}
	})

	t.Run("Open", func(t *testing.T) {
		cb := NewCircuitBreaker("test", 2, 1, time.Second)

		cb.RecordFailure()
		cb.RecordFailure()

		if cb.GetState() != CircuitOpen {
			t.Error("state should be open after failures")
		}
		if cb.Allow() {
			t.Error("open circuit should not allow")
		}
	})

	t.Run("HalfOpen", func(t *testing.T) {
		cb := NewCircuitBreaker("test", 1, 1, 10*time.Millisecond)
		cb.RecordFailure()

		time.Sleep(15 * time.Millisecond)

		if !cb.Allow() {
			t.Error("half-open circuit should allow after timeout")
		}
		if cb.GetState() != CircuitHalfOpen {
			t.Error("state should be half-open")
		}
	})

	t.Run("RecordSuccess", func(t *testing.T) {
		cb := NewCircuitBreaker("test", 1, 1, time.Second)
		cb.RecordFailure()
		cb.State = CircuitHalfOpen

		cb.RecordSuccess()
		if cb.Successes != 1 {
			t.Errorf("successes = %d, want 1", cb.Successes)
		}

		cb.RecordSuccess()
		if cb.GetState() != CircuitClosed {
			t.Error("state should be closed after success threshold")
		}
	})

	t.Run("Reset", func(t *testing.T) {
		cb := NewCircuitBreaker("test", 1, 1, time.Second)
		cb.RecordFailure()
		cb.Reset()

		if cb.GetState() != CircuitClosed {
			t.Error("state should be closed after reset")
		}
		if cb.Failures != 0 {
			t.Errorf("failures = %d, want 0", cb.Failures)
		}
	})

	t.Run("Stats", func(t *testing.T) {
		cb := NewCircuitBreaker("test", 3, 2, time.Second)
		stats := cb.Stats()

		if stats["name"] != "test" {
			t.Errorf("name = %v, want test", stats["name"])
		}
		if stats["state"] != "closed" {
			t.Errorf("state = %v, want closed", stats["state"])
		}
	})
}

func TestSession(t *testing.T) {
	t.Run("Basic", func(t *testing.T) {
		s := NewSession("sess1", time.Hour)

		s.Set("key1", "value1")
		val, ok := s.Get("key1")
		if !ok || val != "value1" {
			t.Errorf("get = %s, %v, want value1, true", val, ok)
		}

		_, ok = s.Get("nonexistent")
		if ok {
			t.Error("nonexistent key should not exist")
		}
	})

	t.Run("Delete", func(t *testing.T) {
		s := NewSession("sess2", time.Hour)
		s.Set("key1", "value1")

		if !s.Delete("key1") {
			t.Error("delete should succeed")
		}
		if s.Delete("key1") {
			t.Error("second delete should fail")
		}
	})

	t.Run("GetAll", func(t *testing.T) {
		s := NewSession("sess3", time.Hour)
		s.Set("key1", "value1")
		s.Set("key2", "value2")

		all := s.GetAll()
		if len(all) != 2 {
			t.Errorf("length = %d, want 2", len(all))
		}
	})

	t.Run("Clear", func(t *testing.T) {
		s := NewSession("sess4", time.Hour)
		s.Set("key1", "value1")
		s.Clear()

		if len(s.GetAll()) != 0 {
			t.Error("session should be cleared")
		}
	})

	t.Run("Refresh", func(t *testing.T) {
		s := NewSession("sess5", time.Millisecond)
		s.Refresh(time.Hour)

		if s.IsExpired() {
			t.Error("session should not be expired after refresh")
		}
	})

	t.Run("TTL", func(t *testing.T) {
		s := NewSession("sess6", time.Hour)
		ttl := s.TTL()

		if ttl <= 0 || ttl > time.Hour {
			t.Errorf("ttl = %v, want between 0 and 1h", ttl)
		}
	})
}

func TestSessionManager(t *testing.T) {
	sm := NewSessionManager()

	t.Run("Create", func(t *testing.T) {
		s := sm.Create("sess1", time.Hour)
		if s == nil {
			t.Fatal("session should not be nil")
		}
	})

	t.Run("Get", func(t *testing.T) {
		s, ok := sm.Get("sess1")
		if !ok || s == nil {
			t.Error("session should exist")
		}

		_, ok = sm.Get("nonexistent")
		if ok {
			t.Error("nonexistent session should not exist")
		}
	})

	t.Run("Exists", func(t *testing.T) {
		if !sm.Exists("sess1") {
			t.Error("session should exist")
		}
		if sm.Exists("nonexistent") {
			t.Error("nonexistent session should not exist")
		}
	})

	t.Run("Count", func(t *testing.T) {
		sm.Create("sess2", time.Hour)
		count := sm.Count()

		if count < 2 {
			t.Errorf("count = %d, want >= 2", count)
		}
	})

	t.Run("List", func(t *testing.T) {
		list := sm.List()
		if len(list) < 2 {
			t.Errorf("list length = %d, want >= 2", len(list))
		}
	})

	t.Run("Delete", func(t *testing.T) {
		if !sm.Delete("sess1") {
			t.Error("delete should succeed")
		}
		if sm.Delete("sess1") {
			t.Error("second delete should fail")
		}
	})

	t.Run("Cleanup", func(t *testing.T) {
		sm.Create("expired", -time.Second)
		time.Sleep(2 * time.Millisecond)

		cleaned := sm.Cleanup()
		if cleaned < 1 {
			t.Errorf("cleaned = %d, want >= 1", cleaned)
		}
	})
}

func TestGlobalCircuitBreakers(t *testing.T) {
	cb1 := GetOrCreateCircuitBreaker("global1", 3, 2, time.Second)
	if cb1 == nil {
		t.Fatal("circuit breaker should not be nil")
	}

	cb2 := GetOrCreateCircuitBreaker("global1", 3, 2, time.Second)
	if cb1 != cb2 {
		t.Error("should return same circuit breaker")
	}

	got, ok := GetCircuitBreaker("global1")
	if !ok || got == nil {
		t.Error("circuit breaker should exist")
	}

	_, ok = GetCircuitBreaker("nonexistent")
	if ok {
		t.Error("nonexistent circuit breaker should not exist")
	}

	list := ListCircuitBreakers()
	if len(list) < 1 {
		t.Error("should have at least 1 circuit breaker")
	}

	if !DeleteCircuitBreaker("global1") {
		t.Error("delete should succeed")
	}
	if DeleteCircuitBreaker("global1") {
		t.Error("second delete should fail")
	}
}

func TestAuditLog(t *testing.T) {
	al := NewAuditLog(100)

	t.Run("Log", func(t *testing.T) {
		id := al.Log("SET", "key1", []string{"value"}, "127.0.0.1", "user", true, 10)
		if id != 1 {
			t.Errorf("id = %d, want 1", id)
		}

		al.Enabled = false
		id = al.Log("GET", "key2", nil, "", "", true, 0)
		if id != 0 {
			t.Errorf("id when disabled = %d, want 0", id)
		}
		al.Enabled = true
	})

	t.Run("NewAuditLogDefault", func(t *testing.T) {
		al2 := NewAuditLog(0)
		if al2.MaxSize != 10000 {
			t.Errorf("max size = %d, want 10000", al2.MaxSize)
		}

		al2 = NewAuditLog(-1)
		if al2.MaxSize != 10000 {
			t.Errorf("max size = %d, want 10000", al2.MaxSize)
		}
	})

	t.Run("Get", func(t *testing.T) {
		entry := al.Get(1)
		if entry == nil {
			t.Fatal("entry should not be nil")
		}
		if entry.Command != "SET" {
			t.Errorf("command = %s, want SET", entry.Command)
		}

		if al.Get(999) != nil {
			t.Error("nonexistent entry should be nil")
		}
	})

	t.Run("GetRange", func(t *testing.T) {
		al2 := NewAuditLog(100)
		al2.Log("CMD1", "", nil, "", "", true, 0)
		time.Sleep(2 * time.Millisecond)
		al2.Log("CMD2", "", nil, "", "", true, 0)
		time.Sleep(2 * time.Millisecond)
		al2.Log("CMD3", "", nil, "", "", true, 0)

		entries := al2.GetRange(0, time.Now().UnixMilli())
		if len(entries) != 3 {
			t.Errorf("entries length = %d, want 3", len(entries))
		}
	})

	t.Run("GetByCommand", func(t *testing.T) {
		al2 := NewAuditLog(100)
		al2.Log("SET", "k1", nil, "", "", true, 0)
		al2.Log("GET", "k2", nil, "", "", true, 0)
		al2.Log("SET", "k3", nil, "", "", true, 0)

		entries := al2.GetByCommand("SET", 10)
		if len(entries) != 2 {
			t.Errorf("entries length = %d, want 2", len(entries))
		}
	})

	t.Run("GetByKey", func(t *testing.T) {
		al2 := NewAuditLog(100)
		al2.Log("SET", "key1", nil, "", "", true, 0)
		al2.Log("SET", "key2", nil, "", "", true, 0)
		al2.Log("GET", "key1", nil, "", "", true, 0)

		entries := al2.GetByKey("key1", 10)
		if len(entries) != 2 {
			t.Errorf("entries length = %d, want 2", len(entries))
		}
	})

	t.Run("Clear", func(t *testing.T) {
		al2 := NewAuditLog(100)
		al2.Log("SET", "k", nil, "", "", true, 0)
		al2.Clear()

		if al2.Count() != 0 {
			t.Error("audit log should be cleared")
		}
	})

	t.Run("Count", func(t *testing.T) {
		al2 := NewAuditLog(100)
		al2.Log("SET", "k", nil, "", "", true, 0)
		al2.Log("GET", "k", nil, "", "", false, 0)

		if al2.Count() != 2 {
			t.Errorf("count = %d, want 2", al2.Count())
		}
	})

	t.Run("Stats", func(t *testing.T) {
		al2 := NewAuditLog(100)
		al2.Log("SET", "k", nil, "", "", true, 0)
		al2.Log("GET", "k", nil, "", "", false, 0)

		stats := al2.Stats()
		if stats["total"] != int64(2) {
			t.Errorf("total = %v, want 2", stats["total"])
		}
		if stats["success"] != int64(1) {
			t.Errorf("success = %v, want 1", stats["success"])
		}
		if stats["failed"] != int64(1) {
			t.Errorf("failed = %v, want 1", stats["failed"])
		}
	})
}

func TestFeatureFlag(t *testing.T) {
	ffm := NewFeatureFlagManager()

	t.Run("Create", func(t *testing.T) {
		flag := ffm.Create("flag1", "Test flag")
		if flag == nil {
			t.Fatal("flag should not be nil")
		}
		if flag.Description != "Test flag" {
			t.Errorf("description = %s, want Test flag", flag.Description)
		}
	})

	t.Run("Get", func(t *testing.T) {
		flag, ok := ffm.Get("flag1")
		if !ok || flag == nil {
			t.Error("flag should exist")
		}

		_, ok = ffm.Get("nonexistent")
		if ok {
			t.Error("nonexistent flag should not exist")
		}
	})

	t.Run("Enable", func(t *testing.T) {
		if !ffm.Enable("flag1") {
			t.Error("enable should succeed")
		}
		if !ffm.IsEnabled("flag1") {
			t.Error("flag should be enabled")
		}

		if ffm.Enable("nonexistent") {
			t.Error("enable nonexistent should fail")
		}
	})

	t.Run("Disable", func(t *testing.T) {
		if !ffm.Disable("flag1") {
			t.Error("disable should succeed")
		}
		if ffm.IsEnabled("flag1") {
			t.Error("flag should be disabled")
		}

		if ffm.Disable("nonexistent") {
			t.Error("disable nonexistent should fail")
		}
	})

	t.Run("Toggle", func(t *testing.T) {
		ffm.Enable("flag1")
		ffm.Toggle("flag1")
		if ffm.IsEnabled("flag1") {
			t.Error("flag should be toggled to disabled")
		}

		if ffm.Toggle("nonexistent") {
			t.Error("toggle nonexistent should fail")
		}
	})

	t.Run("List", func(t *testing.T) {
		ffm.Create("flag2", "Test")
		list := ffm.List()

		if len(list) < 2 {
			t.Errorf("list length = %d, want >= 2", len(list))
		}
	})

	t.Run("ListEnabled", func(t *testing.T) {
		ffm.Enable("flag1")
		ffm.Create("flag3", "Test")
		ffm.Enable("flag3")
		ffm.Disable("flag3")

		enabled := ffm.ListEnabled()
		for _, name := range enabled {
			if !ffm.IsEnabled(name) {
				t.Errorf("flag %s should be enabled", name)
			}
		}
	})

	t.Run("Variants", func(t *testing.T) {
		ffm.Create("flag4", "Test")

		if !ffm.AddVariant("flag4", "color", "blue") {
			t.Error("add variant should succeed")
		}

		val, ok := ffm.GetVariant("flag4", "color")
		if !ok || val != "blue" {
			t.Errorf("variant = %s, %v, want blue, true", val, ok)
		}

		_, ok = ffm.GetVariant("flag4", "nonexistent")
		if ok {
			t.Error("nonexistent variant should not exist")
		}

		_, ok = ffm.GetVariant("nonexistent", "key")
		if ok {
			t.Error("variant from nonexistent flag should not exist")
		}

		if ffm.AddVariant("nonexistent", "k", "v") {
			t.Error("add variant to nonexistent should fail")
		}
	})

	t.Run("AddRule", func(t *testing.T) {
		ffm.Create("flag5", "Test")
		rule := FeatureRule{Attribute: "country", Operator: "==", Value: "US"}

		if !ffm.AddRule("flag5", rule) {
			t.Error("add rule should succeed")
		}

		if ffm.AddRule("nonexistent", rule) {
			t.Error("add rule to nonexistent should fail")
		}
	})

	t.Run("Delete", func(t *testing.T) {
		if !ffm.Delete("flag1") {
			t.Error("delete should succeed")
		}
		if ffm.Delete("flag1") {
			t.Error("second delete should fail")
		}
	})

	t.Run("IsEnabled", func(t *testing.T) {
		if ffm.IsEnabled("nonexistent") {
			t.Error("nonexistent flag should return false")
		}
	})
}

func TestAtomicCounter(t *testing.T) {
	ac := NewAtomicCounter()

	t.Run("Get", func(t *testing.T) {
		if ac.Get("counter1") != 0 {
			t.Error("nonexistent counter should be 0")
		}
	})

	t.Run("Set", func(t *testing.T) {
		ac.Set("counter1", 100)
		if ac.Get("counter1") != 100 {
			t.Errorf("counter = %d, want 100", ac.Get("counter1"))
		}
	})

	t.Run("Increment", func(t *testing.T) {
		result := ac.Increment("counter1", 10)
		if result != 110 {
			t.Errorf("result = %d, want 110", result)
		}
	})

	t.Run("Decrement", func(t *testing.T) {
		result := ac.Decrement("counter1", 5)
		if result != 105 {
			t.Errorf("result = %d, want 105", result)
		}
	})

	t.Run("List", func(t *testing.T) {
		ac.Set("counter2", 200)
		list := ac.List()

		if len(list) < 2 {
			t.Errorf("list length = %d, want >= 2", len(list))
		}
	})

	t.Run("GetAll", func(t *testing.T) {
		all := ac.GetAll()
		if len(all) < 2 {
			t.Errorf("length = %d, want >= 2", len(all))
		}
	})

	t.Run("Reset", func(t *testing.T) {
		if !ac.Reset("counter1") {
			t.Error("reset should succeed")
		}
		if ac.Get("counter1") != 0 {
			t.Errorf("counter = %d, want 0", ac.Get("counter1"))
		}

		if ac.Reset("nonexistent") {
			t.Error("reset nonexistent should fail")
		}
	})

	t.Run("ResetAll", func(t *testing.T) {
		ac.ResetAll()
		if len(ac.GetAll()) != 0 {
			t.Error("all counters should be reset")
		}
	})

	t.Run("Delete", func(t *testing.T) {
		ac.Set("counter3", 300)

		if !ac.Delete("counter3") {
			t.Error("delete should succeed")
		}
		if ac.Delete("counter3") {
			t.Error("second delete should fail")
		}
	})
}

func TestGlobalVariables2(t *testing.T) {
	if GlobalJobScheduler == nil {
		t.Error("GlobalJobScheduler should not be nil")
	}
	if GlobalSessionManager == nil {
		t.Error("GlobalSessionManager should not be nil")
	}
	if GlobalAuditLog == nil {
		t.Error("GlobalAuditLog should not be nil")
	}
	if GlobalFeatureFlags == nil {
		t.Error("GlobalFeatureFlags should not be nil")
	}
	if GlobalAtomicCounter == nil {
		t.Error("GlobalAtomicCounter should not be nil")
	}
}

func TestStoreError(t *testing.T) {
	err := StoreError("test error")
	if err.Error() != "test error" {
		t.Errorf("error = %s, want test error", err.Error())
	}
}

func TestEvictionController(t *testing.T) {
	s := NewStore()
	mt := NewMemoryTracker(1000000, 80, 90)
	ec := NewEvictionController(EvictionAllKeysLRU, 1000000, s, mt, 5)

	t.Run("CheckAndEvictNoMemory", func(t *testing.T) {
		ecZero := NewEvictionController(EvictionAllKeysLRU, 0, s, mt, 5)
		if err := ecZero.CheckAndEvict(); err != nil {
			t.Errorf("check and evict with 0 memory should succeed: %v", err)
		}
	})

	t.Run("CheckAndEvict", func(t *testing.T) {
		s2 := NewStore()
		mt2 := NewMemoryTracker(100, 80, 90)
		ec2 := NewEvictionController(EvictionAllKeysLRU, 100, s2, mt2, 5)

		for i := 0; i < 10; i++ {
			key := "key" + string(rune('0'+i))
			s2.Set(key, &StringValue{Data: []byte("value")}, SetOptions{})
		}

		mt2.Add(200)

		if err := ec2.CheckAndEvict(); err != nil {
			t.Errorf("check and evict should succeed: %v", err)
		}
	})

	t.Run("SetOnEvict", func(t *testing.T) {
		ec.SetOnEvict(func(key string, entry *Entry) {
		})
		if ec.onEvict == nil {
			t.Error("onEvict should be set")
		}
	})

	t.Run("ForceEvict", func(t *testing.T) {
		s3 := NewStore()
		mt3 := NewMemoryTracker(1000000, 80, 90)
		ec3 := NewEvictionController(EvictionAllKeysRandom, 1000000, s3, mt3, 5)

		s3.Set("key1", &StringValue{Data: []byte("value")}, SetOptions{})
		s3.Set("key2", &StringValue{Data: []byte("value")}, SetOptions{})
		s3.Set("key3", &StringValue{Data: []byte("value")}, SetOptions{})

		evicted := ec3.ForceEvict(10)
		if evicted < 0 {
			t.Errorf("evicted = %d, want >= 0", evicted)
		}
	})

	t.Run("SelectVictim", func(t *testing.T) {
		s4 := NewStore()
		mt4 := NewMemoryTracker(1000000, 80, 90)

		ecRandom := NewEvictionController(EvictionAllKeysRandom, 1000000, s4, mt4, 5)
		ecNone := NewEvictionController(EvictionNoEviction, 1000000, s4, mt4, 5)

		s4.Set("key1", &StringValue{Data: []byte("value")}, SetOptions{})

		if key := ecRandom.selectVictim(); key == "" {
			t.Error("Random should select a key")
		}
		if key := ecNone.selectVictim(); key != "" {
			t.Error("No eviction should not select a key")
		}
	})

	t.Run("SelectLFU", func(t *testing.T) {
		s5 := NewStore()
		mt5 := NewMemoryTracker(1000000, 80, 90)
		ec5 := NewEvictionController(EvictionAllKeysLFU, 1000000, s5, mt5, 5)

		if key := ec5.selectLFU(); key != "" {
			t.Error("LFU with no keys should return empty")
		}
	})

	t.Run("SelectRandom", func(t *testing.T) {
		s6 := NewStore()
		mt6 := NewMemoryTracker(1000000, 80, 90)
		ec6 := NewEvictionController(EvictionAllKeysRandom, 1000000, s6, mt6, 5)

		if key := ec6.selectRandom(); key != "" {
			t.Error("Random with no keys should return empty")
		}
	})

	t.Run("EvictKeys", func(t *testing.T) {
		s7 := NewStore()
		mt7 := NewMemoryTracker(1000000, 80, 90)
		ec7 := NewEvictionController(EvictionAllKeysLRU, 1000000, s7, mt7, 5)

		ec7.evictKeys(5)
	})
}

func TestEncodeGeohashInt(t *testing.T) {
	hash1 := EncodeGeohashInt(0, 0)
	if hash1 == 0 {
		t.Error("geohash for (0,0) should not be 0")
	}

	hash2 := EncodeGeohashInt(180, 90)
	if hash2 == 0 {
		t.Error("geohash for (180,90) should not be 0")
	}

	hash4 := EncodeGeohashInt(12.4924, 41.8902)
	if hash4 == 0 {
		t.Error("geohash should not be 0")
	}

	hash5 := EncodeGeohashInt(-179.9, -89.9)
	_ = hash5
}

func TestMemoryTrackerExtended(t *testing.T) {
	t.Run("Max", func(t *testing.T) {
		mt := NewMemoryTracker(1000000, 80, 90)
		if mt.Max() != 1000000 {
			t.Errorf("max = %d, want 1000000", mt.Max())
		}
	})

	t.Run("PressurePercent", func(t *testing.T) {
		mt := NewMemoryTracker(1000000, 80, 90)
		mt.Add(500000)
		pct := mt.PressurePercent()
		if pct < 40 || pct > 60 {
			t.Errorf("pressure percent = %f, want around 50", pct)
		}
	})
}
