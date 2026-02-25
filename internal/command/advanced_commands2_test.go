package command

import (
	"testing"

	"github.com/cachestorm/cachestorm/internal/store"
)

func TestAllAdvancedCommands2(t *testing.T) {
	s := store.NewStore()
	router := NewRouter()
	RegisterAdvancedCommands2(router)

	tests := []struct {
		name  string
		cmd   string
		args  [][]byte
		setup func()
	}{
		{"FILTER.CREATE", "FILTER.CREATE", [][]byte{[]byte("filter1"), []byte("expr")}, nil},
		{"FILTER.CREATE no args", "FILTER.CREATE", nil, nil},
		{"FILTER.DELETE exists", "FILTER.DELETE", [][]byte{[]byte("filter1")}, func() {
			filtersMu.Lock()
			filters["filter1"] = "expr"
			filtersMu.Unlock()
		}},
		{"FILTER.DELETE not found", "FILTER.DELETE", [][]byte{[]byte("notfound")}, nil},
		{"FILTER.DELETE no args", "FILTER.DELETE", nil, nil},
		{"FILTER.APPLY exists", "FILTER.APPLY", [][]byte{[]byte("filter1"), []byte("data")}, func() {
			filtersMu.Lock()
			filters["filter1"] = "expr"
			filtersMu.Unlock()
		}},
		{"FILTER.APPLY no args", "FILTER.APPLY", nil, nil},
		{"FILTER.LIST", "FILTER.LIST", nil, nil},

		{"TRANSFORM.CREATE", "TRANSFORM.CREATE", [][]byte{[]byte("transform1"), []byte("expr")}, nil},
		{"TRANSFORM.CREATE no args", "TRANSFORM.CREATE", nil, nil},
		{"TRANSFORM.DELETE exists", "TRANSFORM.DELETE", [][]byte{[]byte("transform1")}, func() {
			transformsMu.Lock()
			transforms["transform1"] = "expr"
			transformsMu.Unlock()
		}},
		{"TRANSFORM.DELETE not found", "TRANSFORM.DELETE", [][]byte{[]byte("notfound")}, nil},
		{"TRANSFORM.DELETE no args", "TRANSFORM.DELETE", nil, nil},
		{"TRANSFORM.APPLY exists", "TRANSFORM.APPLY", [][]byte{[]byte("transform1"), []byte("data")}, func() {
			transformsMu.Lock()
			transforms["transform1"] = "expr"
			transformsMu.Unlock()
		}},
		{"TRANSFORM.APPLY not found", "TRANSFORM.APPLY", [][]byte{[]byte("notfound"), []byte("data")}, nil},
		{"TRANSFORM.APPLY no args", "TRANSFORM.APPLY", nil, nil},
		{"TRANSFORM.LIST", "TRANSFORM.LIST", nil, nil},

		{"ENRICH.CREATE", "ENRICH.CREATE", [][]byte{[]byte("enrich1"), []byte("source")}, nil},
		{"ENRICH.CREATE no args", "ENRICH.CREATE", nil, nil},
		{"ENRICH.DELETE exists", "ENRICH.DELETE", [][]byte{[]byte("enrich1")}, func() {
			enrichersMu.Lock()
			enrichers["enrich1"] = "source"
			enrichersMu.Unlock()
		}},
		{"ENRICH.DELETE not found", "ENRICH.DELETE", [][]byte{[]byte("notfound")}, nil},
		{"ENRICH.DELETE no args", "ENRICH.DELETE", nil, nil},
		{"ENRICH.APPLY exists", "ENRICH.APPLY", [][]byte{[]byte("enrich1"), []byte("data")}, func() {
			enrichersMu.Lock()
			enrichers["enrich1"] = "source"
			enrichersMu.Unlock()
		}},
		{"ENRICH.APPLY not found", "ENRICH.APPLY", [][]byte{[]byte("notfound"), []byte("data")}, nil},
		{"ENRICH.APPLY no args", "ENRICH.APPLY", nil, nil},
		{"ENRICH.LIST", "ENRICH.LIST", nil, nil},

		{"VALIDATE.CREATE", "VALIDATE.CREATE", [][]byte{[]byte("validate1"), []byte("rule")}, nil},
		{"VALIDATE.CREATE no args", "VALIDATE.CREATE", nil, nil},
		{"VALIDATE.DELETE exists", "VALIDATE.DELETE", [][]byte{[]byte("validate1")}, func() {
			validatorsMu.Lock()
			validators["validate1"] = "rule"
			validatorsMu.Unlock()
		}},
		{"VALIDATE.DELETE not found", "VALIDATE.DELETE", [][]byte{[]byte("notfound")}, nil},
		{"VALIDATE.DELETE no args", "VALIDATE.DELETE", nil, nil},
		{"VALIDATE.CHECK exists", "VALIDATE.CHECK", [][]byte{[]byte("validate1"), []byte("data")}, func() {
			validatorsMu.Lock()
			validators["validate1"] = "rule"
			validatorsMu.Unlock()
		}},
		{"VALIDATE.CHECK not found", "VALIDATE.CHECK", [][]byte{[]byte("notfound"), []byte("data")}, nil},
		{"VALIDATE.CHECK no args", "VALIDATE.CHECK", nil, nil},
		{"VALIDATE.LIST", "VALIDATE.LIST", nil, nil},

		{"JOBX.CREATE", "JOBX.CREATE", [][]byte{[]byte("job1")}, nil},
		{"JOBX.CREATE no args", "JOBX.CREATE", nil, nil},
		{"JOBX.DELETE exists", "JOBX.DELETE", [][]byte{[]byte("job1")}, func() {
			jobsXMux.Lock()
			jobsX["job1"] = &JobX{ID: "job1", Name: "test", Status: "pending"}
			jobsXMux.Unlock()
		}},
		{"JOBX.DELETE not found", "JOBX.DELETE", [][]byte{[]byte("notfound")}, nil},
		{"JOBX.DELETE no args", "JOBX.DELETE", nil, nil},
		{"JOBX.RUN exists", "JOBX.RUN", [][]byte{[]byte("job1")}, func() {
			jobsXMux.Lock()
			jobsX["job1"] = &JobX{ID: "job1", Name: "test", Status: "pending"}
			jobsXMux.Unlock()
		}},
		{"JOBX.RUN not found", "JOBX.RUN", [][]byte{[]byte("notfound")}, nil},
		{"JOBX.RUN no args", "JOBX.RUN", nil, nil},
		{"JOBX.STATUS exists", "JOBX.STATUS", [][]byte{[]byte("job1")}, func() {
			jobsXMux.Lock()
			jobsX["job1"] = &JobX{ID: "job1", Name: "test", Status: "pending"}
			jobsXMux.Unlock()
		}},
		{"JOBX.STATUS not found", "JOBX.STATUS", [][]byte{[]byte("notfound")}, nil},
		{"JOBX.STATUS no args", "JOBX.STATUS", nil, nil},
		{"JOBX.LIST", "JOBX.LIST", nil, nil},

		{"STAGE.CREATE", "STAGE.CREATE", [][]byte{[]byte("stage1"), []byte("5")}, nil},
		{"STAGE.CREATE no args", "STAGE.CREATE", nil, nil},
		{"STAGE.DELETE exists", "STAGE.DELETE", [][]byte{[]byte("stage1")}, func() {
			stagesMu.Lock()
			stages["stage1"] = &Stage{Name: "stage1", Current: 0, Total: 5}
			stagesMu.Unlock()
		}},
		{"STAGE.DELETE not found", "STAGE.DELETE", [][]byte{[]byte("notfound")}, nil},
		{"STAGE.DELETE no args", "STAGE.DELETE", nil, nil},
		{"STAGE.NEXT exists", "STAGE.NEXT", [][]byte{[]byte("stage1")}, func() {
			stagesMu.Lock()
			stages["stage1"] = &Stage{Name: "stage1", Current: 0, Total: 5}
			stagesMu.Unlock()
		}},
		{"STAGE.NEXT not found", "STAGE.NEXT", [][]byte{[]byte("notfound")}, nil},
		{"STAGE.NEXT no args", "STAGE.NEXT", nil, nil},
		{"STAGE.PREV exists", "STAGE.PREV", [][]byte{[]byte("stage1")}, func() {
			stagesMu.Lock()
			stages["stage1"] = &Stage{Name: "stage1", Current: 2, Total: 5}
			stagesMu.Unlock()
		}},
		{"STAGE.PREV not found", "STAGE.PREV", [][]byte{[]byte("notfound")}, nil},
		{"STAGE.PREV no args", "STAGE.PREV", nil, nil},
		{"STAGE.LIST", "STAGE.LIST", nil, nil},

		{"CONTEXT.CREATE", "CONTEXT.CREATE", [][]byte{[]byte("ctx1")}, nil},
		{"CONTEXT.CREATE no args", "CONTEXT.CREATE", nil, nil},
		{"CONTEXT.DELETE exists", "CONTEXT.DELETE", [][]byte{[]byte("ctx1")}, func() {
			contextsMu.Lock()
			contexts["ctx1"] = make(map[string]string)
			contextsMu.Unlock()
		}},
		{"CONTEXT.DELETE not found", "CONTEXT.DELETE", [][]byte{[]byte("notfound")}, nil},
		{"CONTEXT.DELETE no args", "CONTEXT.DELETE", nil, nil},
		{"CONTEXT.SET exists", "CONTEXT.SET", [][]byte{[]byte("ctx1"), []byte("key"), []byte("value")}, func() {
			contextsMu.Lock()
			contexts["ctx1"] = make(map[string]string)
			contextsMu.Unlock()
		}},
		{"CONTEXT.SET not found", "CONTEXT.SET", [][]byte{[]byte("notfound"), []byte("key"), []byte("value")}, nil},
		{"CONTEXT.SET no args", "CONTEXT.SET", nil, nil},
		{"CONTEXT.GET exists", "CONTEXT.GET", [][]byte{[]byte("ctx1"), []byte("key")}, func() {
			contextsMu.Lock()
			contexts["ctx1"] = map[string]string{"key": "value"}
			contextsMu.Unlock()
		}},
		{"CONTEXT.GET not found", "CONTEXT.GET", [][]byte{[]byte("notfound"), []byte("key")}, nil},
		{"CONTEXT.GET no args", "CONTEXT.GET", nil, nil},
		{"CONTEXT.LIST", "CONTEXT.LIST", nil, nil},

		{"RULE.CREATE", "RULE.CREATE", [][]byte{[]byte("rule1"), []byte("condition")}, nil},
		{"RULE.CREATE no args", "RULE.CREATE", nil, nil},
		{"RULE.DELETE exists", "RULE.DELETE", [][]byte{[]byte("rule1")}, func() {
			rulesMu.Lock()
			rules["rule1"] = "condition"
			rulesMu.Unlock()
		}},
		{"RULE.DELETE not found", "RULE.DELETE", [][]byte{[]byte("notfound")}, nil},
		{"RULE.DELETE no args", "RULE.DELETE", nil, nil},
		{"RULE.EVAL exists", "RULE.EVAL", [][]byte{[]byte("rule1"), []byte("data")}, func() {
			rulesMu.Lock()
			rules["rule1"] = "condition"
			rulesMu.Unlock()
		}},
		{"RULE.EVAL not found", "RULE.EVAL", [][]byte{[]byte("notfound"), []byte("data")}, nil},
		{"RULE.EVAL no args", "RULE.EVAL", nil, nil},
		{"RULE.LIST", "RULE.LIST", nil, nil},

		{"POLICY.CREATE", "POLICY.CREATE", [][]byte{[]byte("policy1"), []byte("rules")}, nil},
		{"POLICY.CREATE no args", "POLICY.CREATE", nil, nil},
		{"POLICY.DELETE exists", "POLICY.DELETE", [][]byte{[]byte("policy1")}, func() {
			policiesMu.Lock()
			policies["policy1"] = "rules"
			policiesMu.Unlock()
		}},
		{"POLICY.DELETE not found", "POLICY.DELETE", [][]byte{[]byte("notfound")}, nil},
		{"POLICY.DELETE no args", "POLICY.DELETE", nil, nil},
		{"POLICY.CHECK", "POLICY.CHECK", [][]byte{[]byte("policy1"), []byte("action")}, nil},
		{"POLICY.CHECK no args", "POLICY.CHECK", nil, nil},
		{"POLICY.LIST", "POLICY.LIST", nil, nil},

		{"PERMIT.GRANT", "PERMIT.GRANT", [][]byte{[]byte("user1"), []byte("resource1"), []byte("read")}, nil},
		{"PERMIT.GRANT no args", "PERMIT.GRANT", nil, nil},
		{"PERMIT.REVOKE exists", "PERMIT.REVOKE", [][]byte{[]byte("user1"), []byte("resource1"), []byte("read")}, func() {
			permitsMu.Lock()
			permits["user1"] = map[string]bool{"user1:resource1:read": true}
			permitsMu.Unlock()
		}},
		{"PERMIT.REVOKE no args", "PERMIT.REVOKE", nil, nil},
		{"PERMIT.CHECK exists", "PERMIT.CHECK", [][]byte{[]byte("user1"), []byte("resource1"), []byte("read")}, func() {
			permitsMu.Lock()
			permits["user1"] = map[string]bool{"user1:resource1:read": true}
			permitsMu.Unlock()
		}},
		{"PERMIT.CHECK no args", "PERMIT.CHECK", nil, nil},
		{"PERMIT.LIST", "PERMIT.LIST", [][]byte{[]byte("user1")}, nil},
		{"PERMIT.LIST no args", "PERMIT.LIST", nil, nil},

		{"GRANT.CREATE", "GRANT.CREATE", [][]byte{[]byte("user1"), []byte("resource1"), []byte("read")}, nil},
		{"GRANT.CREATE no args", "GRANT.CREATE", nil, nil},
		{"GRANT.DELETE exists", "GRANT.DELETE", [][]byte{[]byte("grant1")}, func() {
			grantsMu.Lock()
			grants["grant1"] = &Grant{ID: "grant1", User: "user1", Resource: "resource1"}
			grantsMu.Unlock()
		}},
		{"GRANT.DELETE not found", "GRANT.DELETE", [][]byte{[]byte("notfound")}, nil},
		{"GRANT.DELETE no args", "GRANT.DELETE", nil, nil},
		{"GRANT.CHECK exists", "GRANT.CHECK", [][]byte{[]byte("user1"), []byte("resource1"), []byte("read")}, func() {
			grantsMu.Lock()
			grants["grant1"] = &Grant{ID: "grant1", User: "user1", Resource: "resource1", Actions: []string{"read"}}
			grantsMu.Unlock()
		}},
		{"GRANT.CHECK no args", "GRANT.CHECK", nil, nil},
		{"GRANT.LIST", "GRANT.LIST", nil, nil},

		{"CHAINX.CREATE", "CHAINX.CREATE", [][]byte{[]byte("chain1")}, nil},
		{"CHAINX.CREATE no args", "CHAINX.CREATE", nil, nil},
		{"CHAINX.DELETE exists", "CHAINX.DELETE", [][]byte{[]byte("chain1")}, func() {
			chainsXMux.Lock()
			chainsX["chain1"] = &ChainX{Name: "chain1", Steps: []string{"step1"}}
			chainsXMux.Unlock()
		}},
		{"CHAINX.DELETE not found", "CHAINX.DELETE", [][]byte{[]byte("notfound")}, nil},
		{"CHAINX.DELETE no args", "CHAINX.DELETE", nil, nil},
		{"CHAINX.EXECUTE exists", "CHAINX.EXECUTE", [][]byte{[]byte("chain1")}, func() {
			chainsXMux.Lock()
			chainsX["chain1"] = &ChainX{Name: "chain1", Steps: []string{"step1"}}
			chainsXMux.Unlock()
		}},
		{"CHAINX.EXECUTE not found", "CHAINX.EXECUTE", [][]byte{[]byte("notfound")}, nil},
		{"CHAINX.EXECUTE no args", "CHAINX.EXECUTE", nil, nil},
		{"CHAINX.LIST", "CHAINX.LIST", nil, nil},

		{"TASKX.CREATE", "TASKX.CREATE", [][]byte{[]byte("task1")}, nil},
		{"TASKX.CREATE no args", "TASKX.CREATE", nil, nil},
		{"TASKX.DELETE exists", "TASKX.DELETE", [][]byte{[]byte("task1")}, func() {
			tasksXMux.Lock()
			tasksX["task1"] = &TaskX{ID: "task1", Name: "test", Status: "pending"}
			tasksXMux.Unlock()
		}},
		{"TASKX.DELETE not found", "TASKX.DELETE", [][]byte{[]byte("notfound")}, nil},
		{"TASKX.DELETE no args", "TASKX.DELETE", nil, nil},
		{"TASKX.RUN exists", "TASKX.RUN", [][]byte{[]byte("task1")}, func() {
			tasksXMux.Lock()
			tasksX["task1"] = &TaskX{ID: "task1", Name: "test", Status: "pending"}
			tasksXMux.Unlock()
		}},
		{"TASKX.RUN not found", "TASKX.RUN", [][]byte{[]byte("notfound")}, nil},
		{"TASKX.RUN no args", "TASKX.RUN", nil, nil},
		{"TASKX.LIST", "TASKX.LIST", nil, nil},

		{"TIMER.CREATE", "TIMER.CREATE", [][]byte{[]byte("timer1"), []byte("5000")}, nil},
		{"TIMER.CREATE no args", "TIMER.CREATE", nil, nil},
		{"TIMER.DELETE exists", "TIMER.DELETE", [][]byte{[]byte("timer1")}, func() {
			timersMu.Lock()
			timers["timer1"] = &Timer{ID: "timer1", Name: "test", Duration: 5000, Running: true}
			timersMu.Unlock()
		}},
		{"TIMER.DELETE not found", "TIMER.DELETE", [][]byte{[]byte("notfound")}, nil},
		{"TIMER.DELETE no args", "TIMER.DELETE", nil, nil},
		{"TIMER.STATUS exists", "TIMER.STATUS", [][]byte{[]byte("timer1")}, func() {
			timersMu.Lock()
			timers["timer1"] = &Timer{ID: "timer1", Name: "test", Duration: 5000, StartTime: 0, Running: true}
			timersMu.Unlock()
		}},
		{"TIMER.STATUS not found", "TIMER.STATUS", [][]byte{[]byte("notfound")}, nil},
		{"TIMER.STATUS no args", "TIMER.STATUS", nil, nil},
		{"TIMER.LIST", "TIMER.LIST", nil, nil},

		{"COUNTERX2.CREATE", "COUNTERX2.CREATE", [][]byte{[]byte("counter1"), []byte("0")}, nil},
		{"COUNTERX2.CREATE no args", "COUNTERX2.CREATE", nil, nil},
		{"COUNTERX2.INCR", "COUNTERX2.INCR", [][]byte{[]byte("counter1"), []byte("5")}, func() {
			countersX3Mu.Lock()
			countersX3["counter1"] = 0
			countersX3Mu.Unlock()
		}},
		{"COUNTERX2.INCR no args", "COUNTERX2.INCR", nil, nil},
		{"COUNTERX2.DECR", "COUNTERX2.DECR", [][]byte{[]byte("counter1"), []byte("3")}, func() {
			countersX3Mu.Lock()
			countersX3["counter1"] = 10
			countersX3Mu.Unlock()
		}},
		{"COUNTERX2.DECR no args", "COUNTERX2.DECR", nil, nil},
		{"COUNTERX2.GET exists", "COUNTERX2.GET", [][]byte{[]byte("counter1")}, func() {
			countersX3Mu.Lock()
			countersX3["counter1"] = 10
			countersX3Mu.Unlock()
		}},
		{"COUNTERX2.GET not found", "COUNTERX2.GET", [][]byte{[]byte("notfound")}, nil},
		{"COUNTERX2.GET no args", "COUNTERX2.GET", nil, nil},
		{"COUNTERX2.LIST", "COUNTERX2.LIST", nil, nil},

		{"LEVEL.CREATE", "LEVEL.CREATE", [][]byte{[]byte("level1"), []byte("100")}, nil},
		{"LEVEL.CREATE no args", "LEVEL.CREATE", nil, nil},
		{"LEVEL.DELETE exists", "LEVEL.DELETE", [][]byte{[]byte("level1")}, func() {
			levelsMu.Lock()
			levels["level1"] = 100
			levelsMu.Unlock()
		}},
		{"LEVEL.DELETE not found", "LEVEL.DELETE", [][]byte{[]byte("notfound")}, nil},
		{"LEVEL.DELETE no args", "LEVEL.DELETE", nil, nil},
		{"LEVEL.SET exists", "LEVEL.SET", [][]byte{[]byte("level1"), []byte("75")}, func() {
			levelsMu.Lock()
			levels["level1"] = 100
			levelsMu.Unlock()
		}},
		{"LEVEL.SET not found", "LEVEL.SET", [][]byte{[]byte("notfound"), []byte("75")}, nil},
		{"LEVEL.SET no args", "LEVEL.SET", nil, nil},
		{"LEVEL.GET exists", "LEVEL.GET", [][]byte{[]byte("level1")}, func() {
			levelsMu.Lock()
			levels["level1"] = 100
			levels["level1_current"] = 50
			levelsMu.Unlock()
		}},
		{"LEVEL.GET not found", "LEVEL.GET", [][]byte{[]byte("notfound")}, nil},
		{"LEVEL.GET no args", "LEVEL.GET", nil, nil},
		{"LEVEL.LIST", "LEVEL.LIST", nil, nil},

		{"RECORD.CREATE", "RECORD.CREATE", [][]byte{[]byte("record1")}, nil},
		{"RECORD.CREATE no args", "RECORD.CREATE", nil, nil},
		{"RECORD.ADD exists", "RECORD.ADD", [][]byte{[]byte("record1"), []byte("key"), []byte("value")}, func() {
			recordsMu.Lock()
			records["record1"] = &Record{ID: "record1", Name: "test", Fields: make(map[string]string)}
			recordsMu.Unlock()
		}},
		{"RECORD.ADD not found", "RECORD.ADD", [][]byte{[]byte("notfound"), []byte("key"), []byte("value")}, nil},
		{"RECORD.ADD no args", "RECORD.ADD", nil, nil},
		{"RECORD.GET exists", "RECORD.GET", [][]byte{[]byte("record1")}, func() {
			recordsMu.Lock()
			records["record1"] = &Record{ID: "record1", Name: "test", Fields: map[string]string{"key": "value"}}
			recordsMu.Unlock()
		}},
		{"RECORD.GET not found", "RECORD.GET", [][]byte{[]byte("notfound")}, nil},
		{"RECORD.GET no args", "RECORD.GET", nil, nil},
		{"RECORD.DELETE exists", "RECORD.DELETE", [][]byte{[]byte("record1")}, func() {
			recordsMu.Lock()
			records["record1"] = &Record{ID: "record1", Name: "test", Fields: make(map[string]string)}
			recordsMu.Unlock()
		}},
		{"RECORD.DELETE not found", "RECORD.DELETE", [][]byte{[]byte("notfound")}, nil},
		{"RECORD.DELETE no args", "RECORD.DELETE", nil, nil},

		{"ENTITY.CREATE", "ENTITY.CREATE", [][]byte{[]byte("entity1"), []byte("type1")}, nil},
		{"ENTITY.CREATE no args", "ENTITY.CREATE", nil, nil},
		{"ENTITY.DELETE exists", "ENTITY.DELETE", [][]byte{[]byte("entity1")}, func() {
			entitiesMu.Lock()
			entities["entity1"] = &Entity{ID: "entity1", Type: "type1", Attributes: make(map[string]string)}
			entitiesMu.Unlock()
		}},
		{"ENTITY.DELETE not found", "ENTITY.DELETE", [][]byte{[]byte("notfound")}, nil},
		{"ENTITY.DELETE no args", "ENTITY.DELETE", nil, nil},
		{"ENTITY.GET exists", "ENTITY.GET", [][]byte{[]byte("entity1")}, func() {
			entitiesMu.Lock()
			entities["entity1"] = &Entity{ID: "entity1", Type: "type1", Attributes: make(map[string]string)}
			entitiesMu.Unlock()
		}},
		{"ENTITY.GET not found", "ENTITY.GET", [][]byte{[]byte("notfound")}, nil},
		{"ENTITY.GET no args", "ENTITY.GET", nil, nil},
		{"ENTITY.SET exists", "ENTITY.SET", [][]byte{[]byte("entity1"), []byte("key"), []byte("value")}, func() {
			entitiesMu.Lock()
			entities["entity1"] = &Entity{ID: "entity1", Type: "type1", Attributes: make(map[string]string)}
			entitiesMu.Unlock()
		}},
		{"ENTITY.SET not found", "ENTITY.SET", [][]byte{[]byte("notfound"), []byte("key"), []byte("value")}, nil},
		{"ENTITY.SET no args", "ENTITY.SET", nil, nil},
		{"ENTITY.LIST", "ENTITY.LIST", nil, nil},

		{"RELATION.CREATE", "RELATION.CREATE", [][]byte{[]byte("from1"), []byte("to1"), []byte("type1")}, nil},
		{"RELATION.CREATE no args", "RELATION.CREATE", nil, nil},
		{"RELATION.DELETE exists", "RELATION.DELETE", [][]byte{[]byte("rel1")}, func() {
			relationsMu.Lock()
			relations["rel1"] = &Relation{ID: "rel1", From: "from1", To: "to1", Type: "type1", Metadata: make(map[string]string)}
			relationsMu.Unlock()
		}},
		{"RELATION.DELETE not found", "RELATION.DELETE", [][]byte{[]byte("notfound")}, nil},
		{"RELATION.DELETE no args", "RELATION.DELETE", nil, nil},
		{"RELATION.GET exists", "RELATION.GET", [][]byte{[]byte("rel1")}, func() {
			relationsMu.Lock()
			relations["rel1"] = &Relation{ID: "rel1", From: "from1", To: "to1", Type: "type1", Metadata: make(map[string]string)}
			relationsMu.Unlock()
		}},
		{"RELATION.GET not found", "RELATION.GET", [][]byte{[]byte("notfound")}, nil},
		{"RELATION.GET no args", "RELATION.GET", nil, nil},
		{"RELATION.LIST", "RELATION.LIST", nil, nil},

		{"CONNECTIONX.CREATE", "CONNECTIONX.CREATE", [][]byte{[]byte("source1"), []byte("target1")}, nil},
		{"CONNECTIONX.CREATE no args", "CONNECTIONX.CREATE", nil, nil},
		{"CONNECTIONX.DELETE exists", "CONNECTIONX.DELETE", [][]byte{[]byte("conn1")}, func() {
			connectionsXMux.Lock()
			connectionsX["conn1"] = &ConnectionX{ID: "conn1", Source: "source1", Target: "target1", Status: "active"}
			connectionsXMux.Unlock()
		}},
		{"CONNECTIONX.DELETE not found", "CONNECTIONX.DELETE", [][]byte{[]byte("notfound")}, nil},
		{"CONNECTIONX.DELETE no args", "CONNECTIONX.DELETE", nil, nil},
		{"CONNECTIONX.STATUS exists", "CONNECTIONX.STATUS", [][]byte{[]byte("conn1")}, func() {
			connectionsXMux.Lock()
			connectionsX["conn1"] = &ConnectionX{ID: "conn1", Source: "source1", Target: "target1", Status: "active"}
			connectionsXMux.Unlock()
		}},
		{"CONNECTIONX.STATUS not found", "CONNECTIONX.STATUS", [][]byte{[]byte("notfound")}, nil},
		{"CONNECTIONX.STATUS no args", "CONNECTIONX.STATUS", nil, nil},
		{"CONNECTIONX.LIST", "CONNECTIONX.LIST", nil, nil},

		{"POOLX.CREATE", "POOLX.CREATE", [][]byte{[]byte("pool1"), []byte("5")}, nil},
		{"POOLX.CREATE no args", "POOLX.CREATE", nil, nil},
		{"POOLX.DELETE exists", "POOLX.DELETE", [][]byte{[]byte("pool1")}, func() {
			poolsXMux.Lock()
			poolsX["pool1"] = &PoolX{Name: "pool1", Size: 5, Available: []string{"r1"}, InUse: make(map[string]bool)}
			poolsXMux.Unlock()
		}},
		{"POOLX.DELETE not found", "POOLX.DELETE", [][]byte{[]byte("notfound")}, nil},
		{"POOLX.DELETE no args", "POOLX.DELETE", nil, nil},
		{"POOLX.ACQUIRE exists", "POOLX.ACQUIRE", [][]byte{[]byte("pool1")}, func() {
			poolsXMux.Lock()
			poolsX["pool1"] = &PoolX{Name: "pool1", Size: 5, Available: []string{"r1"}, InUse: make(map[string]bool)}
			poolsXMux.Unlock()
		}},
		{"POOLX.ACQUIRE not found", "POOLX.ACQUIRE", [][]byte{[]byte("notfound")}, nil},
		{"POOLX.ACQUIRE no args", "POOLX.ACQUIRE", nil, nil},
		{"POOLX.RELEASE exists", "POOLX.RELEASE", [][]byte{[]byte("pool1"), []byte("r1")}, func() {
			poolsXMux.Lock()
			poolsX["pool1"] = &PoolX{Name: "pool1", Size: 5, Available: []string{}, InUse: map[string]bool{"r1": true}}
			poolsXMux.Unlock()
		}},
		{"POOLX.RELEASE not found", "POOLX.RELEASE", [][]byte{[]byte("notfound"), []byte("r1")}, nil},
		{"POOLX.RELEASE no args", "POOLX.RELEASE", nil, nil},
		{"POOLX.STATUS exists", "POOLX.STATUS", [][]byte{[]byte("pool1")}, func() {
			poolsXMux.Lock()
			poolsX["pool1"] = &PoolX{Name: "pool1", Size: 5, Available: []string{"r1"}, InUse: make(map[string]bool)}
			poolsXMux.Unlock()
		}},
		{"POOLX.STATUS not found", "POOLX.STATUS", [][]byte{[]byte("notfound")}, nil},
		{"POOLX.STATUS no args", "POOLX.STATUS", nil, nil},

		{"BUFFERX.CREATE", "BUFFERX.CREATE", [][]byte{[]byte("buf1"), []byte("1024")}, nil},
		{"BUFFERX.CREATE no args", "BUFFERX.CREATE", nil, nil},
		{"BUFFERX.WRITE exists", "BUFFERX.WRITE", [][]byte{[]byte("buf1"), []byte("data")}, func() {
			buffersXMux.Lock()
			buffersX["buf1"] = &BufferX{Name: "buf1", Data: []byte{}}
			buffersXMux.Unlock()
		}},
		{"BUFFERX.WRITE not found", "BUFFERX.WRITE", [][]byte{[]byte("notfound"), []byte("data")}, nil},
		{"BUFFERX.WRITE no args", "BUFFERX.WRITE", nil, nil},
		{"BUFFERX.READ exists", "BUFFERX.READ", [][]byte{[]byte("buf1")}, func() {
			buffersXMux.Lock()
			buffersX["buf1"] = &BufferX{Name: "buf1", Data: []byte("testdata")}
			buffersXMux.Unlock()
		}},
		{"BUFFERX.READ not found", "BUFFERX.READ", [][]byte{[]byte("notfound")}, nil},
		{"BUFFERX.READ no args", "BUFFERX.READ", nil, nil},
		{"BUFFERX.DELETE exists", "BUFFERX.DELETE", [][]byte{[]byte("buf1")}, func() {
			buffersXMux.Lock()
			buffersX["buf1"] = &BufferX{Name: "buf1", Data: []byte{}}
			buffersXMux.Unlock()
		}},
		{"BUFFERX.DELETE not found", "BUFFERX.DELETE", [][]byte{[]byte("notfound")}, nil},
		{"BUFFERX.DELETE no args", "BUFFERX.DELETE", nil, nil},

		{"STREAMX.CREATE", "STREAMX.CREATE", [][]byte{[]byte("stream1")}, nil},
		{"STREAMX.CREATE no args", "STREAMX.CREATE", nil, nil},
		{"STREAMX.WRITE exists", "STREAMX.WRITE", [][]byte{[]byte("stream1"), []byte("data")}, func() {
			streamsXMux.Lock()
			streamsX["stream1"] = &StreamX{Name: "stream1", Data: []string{}}
			streamsXMux.Unlock()
		}},
		{"STREAMX.WRITE not found", "STREAMX.WRITE", [][]byte{[]byte("notfound"), []byte("data")}, nil},
		{"STREAMX.WRITE no args", "STREAMX.WRITE", nil, nil},
		{"STREAMX.READ exists", "STREAMX.READ", [][]byte{[]byte("stream1")}, func() {
			streamsXMux.Lock()
			streamsX["stream1"] = &StreamX{Name: "stream1", Data: []string{"data1"}}
			streamsXMux.Unlock()
		}},
		{"STREAMX.READ not found", "STREAMX.READ", [][]byte{[]byte("notfound")}, nil},
		{"STREAMX.READ no args", "STREAMX.READ", nil, nil},
		{"STREAMX.DELETE exists", "STREAMX.DELETE", [][]byte{[]byte("stream1")}, func() {
			streamsXMux.Lock()
			streamsX["stream1"] = &StreamX{Name: "stream1", Data: []string{}}
			streamsXMux.Unlock()
		}},
		{"STREAMX.DELETE not found", "STREAMX.DELETE", [][]byte{[]byte("notfound")}, nil},
		{"STREAMX.DELETE no args", "STREAMX.DELETE", nil, nil},

		{"EVENTX.CREATE", "EVENTX.CREATE", [][]byte{[]byte("event1")}, nil},
		{"EVENTX.CREATE no args", "EVENTX.CREATE", nil, nil},
		{"EVENTX.DELETE exists", "EVENTX.DELETE", [][]byte{[]byte("event1")}, func() {
			eventsXMux.Lock()
			eventsX["event1"] = &EventX{Name: "event1", Subscribers: make(map[string]bool), History: []string{}}
			eventsXMux.Unlock()
		}},
		{"EVENTX.DELETE not found", "EVENTX.DELETE", [][]byte{[]byte("notfound")}, nil},
		{"EVENTX.DELETE no args", "EVENTX.DELETE", nil, nil},
		{"EVENTX.EMIT exists", "EVENTX.EMIT", [][]byte{[]byte("event1"), []byte("payload")}, func() {
			eventsXMux.Lock()
			eventsX["event1"] = &EventX{Name: "event1", Subscribers: make(map[string]bool), History: []string{}}
			eventsXMux.Unlock()
		}},
		{"EVENTX.EMIT not found", "EVENTX.EMIT", [][]byte{[]byte("notfound"), []byte("payload")}, nil},
		{"EVENTX.EMIT no args", "EVENTX.EMIT", nil, nil},
		{"EVENTX.SUBSCRIBE exists", "EVENTX.SUBSCRIBE", [][]byte{[]byte("event1"), []byte("sub1")}, func() {
			eventsXMux.Lock()
			eventsX["event1"] = &EventX{Name: "event1", Subscribers: make(map[string]bool), History: []string{}}
			eventsXMux.Unlock()
		}},
		{"EVENTX.SUBSCRIBE not found", "EVENTX.SUBSCRIBE", [][]byte{[]byte("notfound"), []byte("sub1")}, nil},
		{"EVENTX.SUBSCRIBE no args", "EVENTX.SUBSCRIBE", nil, nil},
		{"EVENTX.LIST", "EVENTX.LIST", nil, nil},

		{"HOOK.CREATE", "HOOK.CREATE", [][]byte{[]byte("hook1"), []byte("trigger1"), []byte("action1")}, nil},
		{"HOOK.CREATE no args", "HOOK.CREATE", nil, nil},
		{"HOOK.DELETE exists", "HOOK.DELETE", [][]byte{[]byte("hook1")}, func() {
			hooksMu.Lock()
			hooks["hook1"] = &Hook{ID: "hook1", Name: "test", Trigger: "trigger1", Action: "action1"}
			hooksMu.Unlock()
		}},
		{"HOOK.DELETE not found", "HOOK.DELETE", [][]byte{[]byte("notfound")}, nil},
		{"HOOK.DELETE no args", "HOOK.DELETE", nil, nil},
		{"HOOK.TRIGGER exists", "HOOK.TRIGGER", [][]byte{[]byte("hook1")}, func() {
			hooksMu.Lock()
			hooks["hook1"] = &Hook{ID: "hook1", Name: "test", Trigger: "trigger1", Action: "action1"}
			hooksMu.Unlock()
		}},
		{"HOOK.TRIGGER not found", "HOOK.TRIGGER", [][]byte{[]byte("notfound")}, nil},
		{"HOOK.TRIGGER no args", "HOOK.TRIGGER", nil, nil},
		{"HOOK.LIST", "HOOK.LIST", nil, nil},

		{"MIDDLEWARE.CREATE", "MIDDLEWARE.CREATE", [][]byte{[]byte("mw1"), []byte("before1")}, nil},
		{"MIDDLEWARE.CREATE no args", "MIDDLEWARE.CREATE", nil, nil},
		{"MIDDLEWARE.DELETE exists", "MIDDLEWARE.DELETE", [][]byte{[]byte("mw1")}, func() {
			middlewaresMu.Lock()
			middlewares["mw1"] = &Middleware{ID: "mw1", Name: "test", Before: "before1"}
			middlewaresMu.Unlock()
		}},
		{"MIDDLEWARE.DELETE not found", "MIDDLEWARE.DELETE", [][]byte{[]byte("notfound")}, nil},
		{"MIDDLEWARE.DELETE no args", "MIDDLEWARE.DELETE", nil, nil},
		{"MIDDLEWARE.EXECUTE", "MIDDLEWARE.EXECUTE", [][]byte{[]byte("mw1"), []byte("data")}, nil},
		{"MIDDLEWARE.EXECUTE no args", "MIDDLEWARE.EXECUTE", nil, nil},
		{"MIDDLEWARE.LIST", "MIDDLEWARE.LIST", nil, nil},

		{"INTERCEPTOR.CREATE", "INTERCEPTOR.CREATE", [][]byte{[]byte("int1"), []byte("pattern1")}, nil},
		{"INTERCEPTOR.CREATE no args", "INTERCEPTOR.CREATE", nil, nil},
		{"INTERCEPTOR.DELETE exists", "INTERCEPTOR.DELETE", [][]byte{[]byte("int1")}, func() {
			interceptorsMu.Lock()
			interceptors["int1"] = &Interceptor{ID: "int1", Name: "test", Pattern: "pattern1"}
			interceptorsMu.Unlock()
		}},
		{"INTERCEPTOR.DELETE not found", "INTERCEPTOR.DELETE", [][]byte{[]byte("notfound")}, nil},
		{"INTERCEPTOR.DELETE no args", "INTERCEPTOR.DELETE", nil, nil},
		{"INTERCEPTOR.CHECK", "INTERCEPTOR.CHECK", [][]byte{[]byte("int1"), []byte("data")}, nil},
		{"INTERCEPTOR.CHECK no args", "INTERCEPTOR.CHECK", nil, nil},
		{"INTERCEPTOR.LIST", "INTERCEPTOR.LIST", nil, nil},

		{"GUARD.CREATE", "GUARD.CREATE", [][]byte{[]byte("guard1"), []byte("condition1")}, nil},
		{"GUARD.CREATE no args", "GUARD.CREATE", nil, nil},
		{"GUARD.DELETE exists", "GUARD.DELETE", [][]byte{[]byte("guard1")}, func() {
			guardsMu.Lock()
			guards["guard1"] = &Guard{ID: "guard1", Name: "test", Condition: "condition1"}
			guardsMu.Unlock()
		}},
		{"GUARD.DELETE not found", "GUARD.DELETE", [][]byte{[]byte("notfound")}, nil},
		{"GUARD.DELETE no args", "GUARD.DELETE", nil, nil},
		{"GUARD.CHECK", "GUARD.CHECK", [][]byte{[]byte("guard1")}, nil},
		{"GUARD.CHECK no args", "GUARD.CHECK", nil, nil},
		{"GUARD.LIST", "GUARD.LIST", nil, nil},

		{"PROXY.CREATE", "PROXY.CREATE", [][]byte{[]byte("proxy1"), []byte("http://localhost:8080")}, nil},
		{"PROXY.CREATE no args", "PROXY.CREATE", nil, nil},
		{"PROXY.DELETE exists", "PROXY.DELETE", [][]byte{[]byte("proxy1")}, func() {
			proxiesMu.Lock()
			proxies["proxy1"] = &Proxy{ID: "proxy1", Name: "test", Target: "http://localhost:8080"}
			proxiesMu.Unlock()
		}},
		{"PROXY.DELETE not found", "PROXY.DELETE", [][]byte{[]byte("notfound")}, nil},
		{"PROXY.DELETE no args", "PROXY.DELETE", nil, nil},
		{"PROXY.ROUTE", "PROXY.ROUTE", [][]byte{[]byte("proxy1"), []byte("/path")}, nil},
		{"PROXY.ROUTE no args", "PROXY.ROUTE", nil, nil},
		{"PROXY.LIST", "PROXY.LIST", nil, nil},

		{"CACHEX.CREATE", "CACHEX.CREATE", [][]byte{[]byte("cache1")}, nil},
		{"CACHEX.CREATE no args", "CACHEX.CREATE", nil, nil},
		{"CACHEX.DELETE exists", "CACHEX.DELETE", [][]byte{[]byte("cache1")}, func() {
			cachesXMux.Lock()
			cachesX["cache1"] = &CacheX{Name: "cache1", Data: make(map[string]string)}
			cachesXMux.Unlock()
		}},
		{"CACHEX.DELETE not found", "CACHEX.DELETE", [][]byte{[]byte("notfound")}, nil},
		{"CACHEX.DELETE no args", "CACHEX.DELETE", nil, nil},
		{"CACHEX.GET exists", "CACHEX.GET", [][]byte{[]byte("cache1"), []byte("key")}, func() {
			cachesXMux.Lock()
			cachesX["cache1"] = &CacheX{Name: "cache1", Data: map[string]string{"key": "value"}}
			cachesXMux.Unlock()
		}},
		{"CACHEX.GET not found", "CACHEX.GET", [][]byte{[]byte("notfound"), []byte("key")}, nil},
		{"CACHEX.GET no args", "CACHEX.GET", nil, nil},
		{"CACHEX.SET exists", "CACHEX.SET", [][]byte{[]byte("cache1"), []byte("key"), []byte("value")}, func() {
			cachesXMux.Lock()
			cachesX["cache1"] = &CacheX{Name: "cache1", Data: make(map[string]string)}
			cachesXMux.Unlock()
		}},
		{"CACHEX.SET not found", "CACHEX.SET", [][]byte{[]byte("notfound"), []byte("key"), []byte("value")}, nil},
		{"CACHEX.SET no args", "CACHEX.SET", nil, nil},
		{"CACHEX.LIST", "CACHEX.LIST", nil, nil},

		{"STOREX.CREATE", "STOREX.CREATE", [][]byte{[]byte("store1")}, nil},
		{"STOREX.CREATE no args", "STOREX.CREATE", nil, nil},
		{"STOREX.DELETE exists", "STOREX.DELETE", [][]byte{[]byte("store1")}, func() {
			storesXMux.Lock()
			storesX["store1"] = &StoreX{Name: "store1", Data: make(map[string]string)}
			storesXMux.Unlock()
		}},
		{"STOREX.DELETE not found", "STOREX.DELETE", [][]byte{[]byte("notfound")}, nil},
		{"STOREX.DELETE no args", "STOREX.DELETE", nil, nil},
		{"STOREX.PUT exists", "STOREX.PUT", [][]byte{[]byte("store1"), []byte("key"), []byte("value")}, func() {
			storesXMux.Lock()
			storesX["store1"] = &StoreX{Name: "store1", Data: make(map[string]string)}
			storesXMux.Unlock()
		}},
		{"STOREX.PUT not found", "STOREX.PUT", [][]byte{[]byte("notfound"), []byte("key"), []byte("value")}, nil},
		{"STOREX.PUT no args", "STOREX.PUT", nil, nil},
		{"STOREX.GET exists", "STOREX.GET", [][]byte{[]byte("store1"), []byte("key")}, func() {
			storesXMux.Lock()
			storesX["store1"] = &StoreX{Name: "store1", Data: map[string]string{"key": "value"}}
			storesXMux.Unlock()
		}},
		{"STOREX.GET not found", "STOREX.GET", [][]byte{[]byte("notfound"), []byte("key")}, nil},
		{"STOREX.GET no args", "STOREX.GET", nil, nil},
		{"STOREX.LIST", "STOREX.LIST", nil, nil},

		{"INDEX.CREATE", "INDEX.CREATE", [][]byte{[]byte("idx1")}, nil},
		{"INDEX.CREATE no args", "INDEX.CREATE", nil, nil},
		{"INDEX.DELETE exists", "INDEX.DELETE", [][]byte{[]byte("idx1")}, func() {
			indexesMu.Lock()
			indexes["idx1"] = &Index{Name: "idx1", Entries: make(map[string][]string)}
			indexesMu.Unlock()
		}},
		{"INDEX.DELETE not found", "INDEX.DELETE", [][]byte{[]byte("notfound")}, nil},
		{"INDEX.DELETE no args", "INDEX.DELETE", nil, nil},
		{"INDEX.ADD exists", "INDEX.ADD", [][]byte{[]byte("idx1"), []byte("key"), []byte("id1")}, func() {
			indexesMu.Lock()
			indexes["idx1"] = &Index{Name: "idx1", Entries: make(map[string][]string)}
			indexesMu.Unlock()
		}},
		{"INDEX.ADD not found", "INDEX.ADD", [][]byte{[]byte("notfound"), []byte("key"), []byte("id1")}, nil},
		{"INDEX.ADD no args", "INDEX.ADD", nil, nil},
		{"INDEX.SEARCH exists", "INDEX.SEARCH", [][]byte{[]byte("idx1"), []byte("key")}, func() {
			indexesMu.Lock()
			indexes["idx1"] = &Index{Name: "idx1", Entries: map[string][]string{"key": {"id1"}}}
			indexesMu.Unlock()
		}},
		{"INDEX.SEARCH not found", "INDEX.SEARCH", [][]byte{[]byte("notfound"), []byte("key")}, nil},
		{"INDEX.SEARCH no args", "INDEX.SEARCH", nil, nil},
		{"INDEX.LIST", "INDEX.LIST", nil, nil},

		{"QUERY.CREATE", "QUERY.CREATE", [][]byte{[]byte("query1"), []byte("SELECT *")}, nil},
		{"QUERY.CREATE no args", "QUERY.CREATE", nil, nil},
		{"QUERY.DELETE exists", "QUERY.DELETE", [][]byte{[]byte("query1")}, func() {
			queriesMu.Lock()
			queries["query1"] = "SELECT *"
			queriesMu.Unlock()
		}},
		{"QUERY.DELETE not found", "QUERY.DELETE", [][]byte{[]byte("notfound")}, nil},
		{"QUERY.DELETE no args", "QUERY.DELETE", nil, nil},
		{"QUERY.EXECUTE", "QUERY.EXECUTE", [][]byte{[]byte("query1")}, nil},
		{"QUERY.EXECUTE no args", "QUERY.EXECUTE", nil, nil},
		{"QUERY.LIST", "QUERY.LIST", nil, nil},

		{"VIEW.CREATE", "VIEW.CREATE", [][]byte{[]byte("view1"), []byte("definition")}, nil},
		{"VIEW.CREATE no args", "VIEW.CREATE", nil, nil},
		{"VIEW.DELETE exists", "VIEW.DELETE", [][]byte{[]byte("view1")}, func() {
			viewsMu.Lock()
			views["view1"] = "definition"
			viewsMu.Unlock()
		}},
		{"VIEW.DELETE not found", "VIEW.DELETE", [][]byte{[]byte("notfound")}, nil},
		{"VIEW.DELETE no args", "VIEW.DELETE", nil, nil},
		{"VIEW.GET exists", "VIEW.GET", [][]byte{[]byte("view1")}, func() {
			viewsMu.Lock()
			views["view1"] = "definition"
			viewsMu.Unlock()
		}},
		{"VIEW.GET not found", "VIEW.GET", [][]byte{[]byte("notfound")}, nil},
		{"VIEW.GET no args", "VIEW.GET", nil, nil},
		{"VIEW.LIST", "VIEW.LIST", nil, nil},

		{"REPORT.CREATE", "REPORT.CREATE", [][]byte{[]byte("report1"), []byte("template")}, nil},
		{"REPORT.CREATE no args", "REPORT.CREATE", nil, nil},
		{"REPORT.DELETE exists", "REPORT.DELETE", [][]byte{[]byte("report1")}, func() {
			reportsMu.Lock()
			reports["report1"] = &Report{ID: "report1", Name: "test", Template: "template"}
			reportsMu.Unlock()
		}},
		{"REPORT.DELETE not found", "REPORT.DELETE", [][]byte{[]byte("notfound")}, nil},
		{"REPORT.DELETE no args", "REPORT.DELETE", nil, nil},
		{"REPORT.GENERATE", "REPORT.GENERATE", [][]byte{[]byte("report1")}, nil},
		{"REPORT.GENERATE no args", "REPORT.GENERATE", nil, nil},
		{"REPORT.LIST", "REPORT.LIST", nil, nil},

		{"AUDITX.LOG", "AUDITX.LOG", [][]byte{[]byte("log1"), []byte("action1"), []byte("user1")}, nil},
		{"AUDITX.LOG no args", "AUDITX.LOG", nil, nil},
		{"AUDITX.GET exists", "AUDITX.GET", [][]byte{[]byte("log1")}, func() {
			auditsXMux.Lock()
			auditsX["log1"] = []*AuditEntryX{{Timestamp: 1, Action: "action1", User: "user1"}}
			auditsXMux.Unlock()
		}},
		{"AUDITX.GET not found", "AUDITX.GET", [][]byte{[]byte("notfound")}, nil},
		{"AUDITX.GET no args", "AUDITX.GET", nil, nil},
		{"AUDITX.SEARCH exists", "AUDITX.SEARCH", [][]byte{[]byte("log1"), []byte("action1")}, func() {
			auditsXMux.Lock()
			auditsX["log1"] = []*AuditEntryX{{Timestamp: 1, Action: "action1", User: "user1"}}
			auditsXMux.Unlock()
		}},
		{"AUDITX.SEARCH not found", "AUDITX.SEARCH", [][]byte{[]byte("notfound"), []byte("action1")}, nil},
		{"AUDITX.SEARCH no args", "AUDITX.SEARCH", nil, nil},
		{"AUDITX.LIST", "AUDITX.LIST", nil, nil},

		{"TOKEN.CREATE", "TOKEN.CREATE", [][]byte{[]byte("user1"), []byte("3600000")}, nil},
		{"TOKEN.CREATE no args", "TOKEN.CREATE", nil, nil},
		{"TOKEN.DELETE exists", "TOKEN.DELETE", [][]byte{[]byte("token1")}, func() {
			tokensMu.Lock()
			tokens["token1"] = &Token{ID: "token1", User: "user1", ExpiresAt: 9999999999999}
			tokensMu.Unlock()
		}},
		{"TOKEN.DELETE not found", "TOKEN.DELETE", [][]byte{[]byte("notfound")}, nil},
		{"TOKEN.DELETE no args", "TOKEN.DELETE", nil, nil},
		{"TOKEN.VALIDATE exists", "TOKEN.VALIDATE", [][]byte{[]byte("token1")}, func() {
			tokensMu.Lock()
			tokens["token1"] = &Token{ID: "token1", User: "user1", ExpiresAt: 9999999999999}
			tokensMu.Unlock()
		}},
		{"TOKEN.VALIDATE not found", "TOKEN.VALIDATE", [][]byte{[]byte("notfound")}, nil},
		{"TOKEN.VALIDATE no args", "TOKEN.VALIDATE", nil, nil},
		{"TOKEN.REFRESH exists", "TOKEN.REFRESH", [][]byte{[]byte("token1"), []byte("3600000")}, func() {
			tokensMu.Lock()
			tokens["token1"] = &Token{ID: "token1", User: "user1", ExpiresAt: 9999999999999}
			tokensMu.Unlock()
		}},
		{"TOKEN.REFRESH not found", "TOKEN.REFRESH", [][]byte{[]byte("notfound"), []byte("3600000")}, nil},
		{"TOKEN.REFRESH no args", "TOKEN.REFRESH", nil, nil},
		{"TOKEN.LIST", "TOKEN.LIST", nil, nil},

		{"SESSIONX.CREATE", "SESSIONX.CREATE", [][]byte{[]byte("user1"), []byte("3600000")}, nil},
		{"SESSIONX.CREATE no args", "SESSIONX.CREATE", nil, nil},
		{"SESSIONX.DELETE exists", "SESSIONX.DELETE", [][]byte{[]byte("sess1")}, func() {
			sessionsXMux.Lock()
			sessionsX["sess1"] = &SessionX{ID: "sess1", User: "user1", Data: make(map[string]string)}
			sessionsXMux.Unlock()
		}},
		{"SESSIONX.DELETE not found", "SESSIONX.DELETE", [][]byte{[]byte("notfound")}, nil},
		{"SESSIONX.DELETE no args", "SESSIONX.DELETE", nil, nil},
		{"SESSIONX.GET exists", "SESSIONX.GET", [][]byte{[]byte("sess1")}, func() {
			sessionsXMux.Lock()
			sessionsX["sess1"] = &SessionX{ID: "sess1", User: "user1", Data: make(map[string]string)}
			sessionsXMux.Unlock()
		}},
		{"SESSIONX.GET not found", "SESSIONX.GET", [][]byte{[]byte("notfound")}, nil},
		{"SESSIONX.GET no args", "SESSIONX.GET", nil, nil},
		{"SESSIONX.SET exists", "SESSIONX.SET", [][]byte{[]byte("sess1"), []byte("key"), []byte("value")}, func() {
			sessionsXMux.Lock()
			sessionsX["sess1"] = &SessionX{ID: "sess1", User: "user1", Data: make(map[string]string)}
			sessionsXMux.Unlock()
		}},
		{"SESSIONX.SET not found", "SESSIONX.SET", [][]byte{[]byte("notfound"), []byte("key"), []byte("value")}, nil},
		{"SESSIONX.SET no args", "SESSIONX.SET", nil, nil},
		{"SESSIONX.LIST", "SESSIONX.LIST", nil, nil},

		{"PROFILE.CREATE", "PROFILE.CREATE", [][]byte{[]byte("user1")}, nil},
		{"PROFILE.CREATE no args", "PROFILE.CREATE", nil, nil},
		{"PROFILE.DELETE exists", "PROFILE.DELETE", [][]byte{[]byte("prof1")}, func() {
			profilesMu.Lock()
			profiles["prof1"] = &Profile{ID: "prof1", User: "user1", Attributes: make(map[string]string)}
			profilesMu.Unlock()
		}},
		{"PROFILE.DELETE not found", "PROFILE.DELETE", [][]byte{[]byte("notfound")}, nil},
		{"PROFILE.DELETE no args", "PROFILE.DELETE", nil, nil},
		{"PROFILE.GET exists", "PROFILE.GET", [][]byte{[]byte("prof1")}, func() {
			profilesMu.Lock()
			profiles["prof1"] = &Profile{ID: "prof1", User: "user1", Attributes: make(map[string]string)}
			profilesMu.Unlock()
		}},
		{"PROFILE.GET not found", "PROFILE.GET", [][]byte{[]byte("notfound")}, nil},
		{"PROFILE.GET no args", "PROFILE.GET", nil, nil},
		{"PROFILE.SET exists", "PROFILE.SET", [][]byte{[]byte("prof1"), []byte("key"), []byte("value")}, func() {
			profilesMu.Lock()
			profiles["prof1"] = &Profile{ID: "prof1", User: "user1", Attributes: make(map[string]string)}
			profilesMu.Unlock()
		}},
		{"PROFILE.SET not found", "PROFILE.SET", [][]byte{[]byte("notfound"), []byte("key"), []byte("value")}, nil},
		{"PROFILE.SET no args", "PROFILE.SET", nil, nil},
		{"PROFILE.LIST", "PROFILE.LIST", nil, nil},

		{"ROLEX.CREATE", "ROLEX.CREATE", [][]byte{[]byte("role1")}, nil},
		{"ROLEX.CREATE no args", "ROLEX.CREATE", nil, nil},
		{"ROLEX.DELETE exists", "ROLEX.DELETE", [][]byte{[]byte("role1")}, func() {
			rolesXMux.Lock()
			rolesX["role1"] = &RoleX{ID: "role1", Name: "role1", Permissions: []string{}}
			rolesXMux.Unlock()
		}},
		{"ROLEX.DELETE not found", "ROLEX.DELETE", [][]byte{[]byte("notfound")}, nil},
		{"ROLEX.DELETE no args", "ROLEX.DELETE", nil, nil},
		{"ROLEX.ASSIGN", "ROLEX.ASSIGN", [][]byte{[]byte("role1"), []byte("user1"), []byte("perm1")}, nil},
		{"ROLEX.ASSIGN no args", "ROLEX.ASSIGN", nil, nil},
		{"ROLEX.CHECK", "ROLEX.CHECK", [][]byte{[]byte("role1"), []byte("user1"), []byte("perm1")}, nil},
		{"ROLEX.CHECK no args", "ROLEX.CHECK", nil, nil},
		{"ROLEX.LIST", "ROLEX.LIST", nil, nil},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.setup != nil {
				tt.setup()
			}
			handler, ok := router.Get(tt.cmd)
			if !ok {
				t.Fatalf("Command %s not found", tt.cmd)
			}
			ctx := newTestContext(tt.cmd, tt.args, s)
			if err := handler.Handler(ctx); err != nil {
				t.Errorf("Unexpected error: %v", err)
			}
		})
	}
}
