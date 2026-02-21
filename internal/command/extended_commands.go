package command

import (
	"fmt"
	"sort"
	"sync"
	"time"

	"github.com/cachestorm/cachestorm/internal/resp"
)

func RegisterExtendedCommands(router *Router) {
	router.Register(&CommandDef{Name: "MSGQUEUE.CREATE", Handler: cmdMSGQUEUECREATE})
	router.Register(&CommandDef{Name: "MSGQUEUE.PUBLISH", Handler: cmdMSGQUEUEPUBLISH})
	router.Register(&CommandDef{Name: "MSGQUEUE.CONSUME", Handler: cmdMSGQUEUECONSUME})
	router.Register(&CommandDef{Name: "MSGQUEUE.ACK", Handler: cmdMSGQUEUEACK})
	router.Register(&CommandDef{Name: "MSGQUEUE.NACK", Handler: cmdMSGQUEUENACK})
	router.Register(&CommandDef{Name: "MSGQUEUE.DEADLETTER", Handler: cmdMSGQUEUEDEADLETTER})
	router.Register(&CommandDef{Name: "MSGQUEUE.REQUEUE", Handler: cmdMSGQUEUEREQUEUE})
	router.Register(&CommandDef{Name: "MSGQUEUE.PURGE", Handler: cmdMSGQUEUEPURGE})
	router.Register(&CommandDef{Name: "MSGQUEUE.STATS", Handler: cmdMSGQUEUESTATS})
	router.Register(&CommandDef{Name: "MSGQUEUE.DELETE", Handler: cmdMSGQUEUEDELETE})

	router.Register(&CommandDef{Name: "SERVICE.REGISTER", Handler: cmdSERVICEREGISTER})
	router.Register(&CommandDef{Name: "SERVICE.DEREGISTER", Handler: cmdSERVICEDEREGISTER})
	router.Register(&CommandDef{Name: "SERVICE.DISCOVER", Handler: cmdSERVICEDISCOVER})
	router.Register(&CommandDef{Name: "SERVICE.HEARTBEAT", Handler: cmdSERVICEHEARTBEAT})
	router.Register(&CommandDef{Name: "SERVICE.LIST", Handler: cmdSERVICELIST})
	router.Register(&CommandDef{Name: "SERVICE.HEALTHY", Handler: cmdSERVICEHEALTHY})
	router.Register(&CommandDef{Name: "SERVICE.WEIGHT", Handler: cmdSERVICEWEIGHT})
	router.Register(&CommandDef{Name: "SERVICE.TAGS", Handler: cmdSERVICETAGS})

	router.Register(&CommandDef{Name: "HEALTHX.REGISTER", Handler: cmdHEALTHXREGISTER})
	router.Register(&CommandDef{Name: "HEALTHX.UNREGISTER", Handler: cmdHEALTHXUNREGISTER})
	router.Register(&CommandDef{Name: "HEALTHX.CHECK", Handler: cmdHEALTHXCHECK})
	router.Register(&CommandDef{Name: "HEALTHX.STATUS", Handler: cmdHEALTHXSTATUS})
	router.Register(&CommandDef{Name: "HEALTHX.HISTORY", Handler: cmdHEALTHXHISTORY})
	router.Register(&CommandDef{Name: "HEALTHX.LIST", Handler: cmdHEALTHXLIST})

	router.Register(&CommandDef{Name: "CRON.ADD", Handler: cmdCRONADD})
	router.Register(&CommandDef{Name: "CRON.REMOVE", Handler: cmdCRONREMOVE})
	router.Register(&CommandDef{Name: "CRON.LIST", Handler: cmdCRONLIST})
	router.Register(&CommandDef{Name: "CRON.TRIGGER", Handler: cmdCRONTRIGGER})
	router.Register(&CommandDef{Name: "CRON.PAUSE", Handler: cmdCRONPAUSE})
	router.Register(&CommandDef{Name: "CRON.RESUME", Handler: cmdCRONRESUME})
	router.Register(&CommandDef{Name: "CRON.NEXT", Handler: cmdCRONNEXT})
	router.Register(&CommandDef{Name: "CRON.HISTORY", Handler: cmdCRONHISTORY})

	router.Register(&CommandDef{Name: "VECTOR.CREATE", Handler: cmdVECTORCREATE})
	router.Register(&CommandDef{Name: "VECTOR.ADD", Handler: cmdVECTORADD})
	router.Register(&CommandDef{Name: "VECTOR.GET", Handler: cmdVECTORGET})
	router.Register(&CommandDef{Name: "VECTOR.DELETE", Handler: cmdVECTORDELETE})
	router.Register(&CommandDef{Name: "VECTOR.SEARCH", Handler: cmdVECTORSEARCH})
	router.Register(&CommandDef{Name: "VECTOR.SIMILARITY", Handler: cmdVECTORSIMILARITY})
	router.Register(&CommandDef{Name: "VECTOR.NORMALIZE", Handler: cmdVECTORNORMALIZE})
	router.Register(&CommandDef{Name: "VECTOR.DIMENSIONS", Handler: cmdVECTORDIMENSIONS})
	router.Register(&CommandDef{Name: "VECTOR.MERGE", Handler: cmdVECTORMERGE})
	router.Register(&CommandDef{Name: "VECTOR.STATS", Handler: cmdVECTORSTATS})

	router.Register(&CommandDef{Name: "DOC.INSERT", Handler: cmdDOCINSERT})
	router.Register(&CommandDef{Name: "DOC.FIND", Handler: cmdDOCFIND})
	router.Register(&CommandDef{Name: "DOC.FINDONE", Handler: cmdDOCFINDONE})
	router.Register(&CommandDef{Name: "DOC.UPDATE", Handler: cmdDOCUPDATE})
	router.Register(&CommandDef{Name: "DOC.DELETE", Handler: cmdDOCDELETE})
	router.Register(&CommandDef{Name: "DOC.COUNT", Handler: cmdDOCCOUNT})
	router.Register(&CommandDef{Name: "DOC.DISTINCT", Handler: docDOCDISTINCT})
	router.Register(&CommandDef{Name: "DOC.AGGREGATE", Handler: cmdDOCAGGREGATE})
	router.Register(&CommandDef{Name: "DOC.INDEX", Handler: cmdDOCINDEX})
	router.Register(&CommandDef{Name: "DOC.DROPINDEX", Handler: cmdDOCDROPINDEX})

	router.Register(&CommandDef{Name: "TOPIC.SUBSCRIBE", Handler: cmdTOPICSUBSCRIBE})
	router.Register(&CommandDef{Name: "TOPIC.UNSUBSCRIBE", Handler: cmdTOPICUNSUBSCRIBE})
	router.Register(&CommandDef{Name: "TOPIC.PUBLISH", Handler: cmdTOPICPUBLISH})
	router.Register(&CommandDef{Name: "TOPIC.SUBSCRIBERS", Handler: cmdTOPICSUBSCRIBERS})
	router.Register(&CommandDef{Name: "TOPIC.LIST", Handler: cmdTOPICLIST})
	router.Register(&CommandDef{Name: "TOPIC.HISTORY", Handler: cmdTOPICHISTORY})

	router.Register(&CommandDef{Name: "WS.CONNECT", Handler: cmdWSCONNECT})
	router.Register(&CommandDef{Name: "WS.DISCONNECT", Handler: cmdWSDISCONNECT})
	router.Register(&CommandDef{Name: "WS.SEND", Handler: cmdWSSEND})
	router.Register(&CommandDef{Name: "WS.BROADCAST", Handler: cmdWSBROADCAST})
	router.Register(&CommandDef{Name: "WS.LIST", Handler: cmdWSLIST})
	router.Register(&CommandDef{Name: "WS.ROOMS", Handler: cmdWSROOMS})
	router.Register(&CommandDef{Name: "WS.JOIN", Handler: cmdWSJOIN})
	router.Register(&CommandDef{Name: "WS.LEAVE", Handler: cmdWSLEAVE})

	router.Register(&CommandDef{Name: "LEADER.ELECT", Handler: cmdLEADERELECT})
	router.Register(&CommandDef{Name: "LEADER.RENEW", Handler: cmdLEADERRENEW})
	router.Register(&CommandDef{Name: "LEADER.RESIGN", Handler: cmdLEADERRESIGN})
	router.Register(&CommandDef{Name: "LEADER.CURRENT", Handler: cmdLEADERCURRENT})
	router.Register(&CommandDef{Name: "LEADER.HISTORY", Handler: cmdLEADERHISTORY})

	router.Register(&CommandDef{Name: "MEMO.CACHE", Handler: cmdMEMOCACHE})
	router.Register(&CommandDef{Name: "MEMO.INVALIDATE", Handler: cmdMEMOINVALIDATE})
	router.Register(&CommandDef{Name: "MEMO.STATS", Handler: cmdMEMOSTATS})
	router.Register(&CommandDef{Name: "MEMO.CLEAR", Handler: cmdMEMOCLEAR})
	router.Register(&CommandDef{Name: "MEMO.WARM", Handler: cmdMEMOWARM})

	router.Register(&CommandDef{Name: "SENTINELX.WATCH", Handler: cmdSENTINELXWATCH})
	router.Register(&CommandDef{Name: "SENTINELX.UNWATCH", Handler: cmdSENTINELXUNWATCH})
	router.Register(&CommandDef{Name: "SENTINELX.STATUS", Handler: cmdSENTINELXSTATUS})
	router.Register(&CommandDef{Name: "SENTINELX.ALERTS", Handler: cmdSENTINELXALERTS})
	router.Register(&CommandDef{Name: "SENTINELX.CONFIG", Handler: cmdSENTINELXCONFIG})

	router.Register(&CommandDef{Name: "BACKUPX.CREATE", Handler: cmdBACKUPXCREATE})
	router.Register(&CommandDef{Name: "BACKUPX.RESTORE", Handler: cmdBACKUPXRESTORE})
	router.Register(&CommandDef{Name: "BACKUPX.LIST", Handler: cmdBACKUPXLIST})
	router.Register(&CommandDef{Name: "BACKUPX.DELETE", Handler: cmdBACKUPXDELETE})

	router.Register(&CommandDef{Name: "REPLAY.START", Handler: cmdREPLAYSTART})
	router.Register(&CommandDef{Name: "REPLAY.STOP", Handler: cmdREPLAYSTOP})
	router.Register(&CommandDef{Name: "REPLAY.STATUS", Handler: cmdREPLAYSTATUS})
	router.Register(&CommandDef{Name: "REPLAY.SPEED", Handler: cmdREPLAYSPEED})
	router.Register(&CommandDef{Name: "REPLAY.SEEK", Handler: cmdREPLAYSEEK})

	router.Register(&CommandDef{Name: "AGG.SUM", Handler: cmdAGGSUM})
	router.Register(&CommandDef{Name: "AGG.AVG", Handler: cmdAGGAVG})
	router.Register(&CommandDef{Name: "AGG.MIN", Handler: cmdAGGMIN})
	router.Register(&CommandDef{Name: "AGG.MAX", Handler: cmdAGGMAX})
	router.Register(&CommandDef{Name: "AGG.COUNT", Handler: cmdAGGCOUNT})
	router.Register(&CommandDef{Name: "AGG.PUSH", Handler: cmdAGGPUSH})
	router.Register(&CommandDef{Name: "AGG.CLEAR", Handler: cmdAGGCLEAR})
}

var (
	msgQueues   = make(map[string]*MessageQueue)
	msgQueuesMu sync.RWMutex
)

type MessageQueue struct {
	Name        string
	Messages    []*QueuedMessage
	DeadLetters []*QueuedMessage
	AckWait     map[string]*QueuedMessage
	MaxRetries  int
}

type QueuedMessage struct {
	ID        string
	Body      string
	Status    string
	Retries   int
	CreatedAt int64
	AckAt     int64
}

func cmdMSGQUEUECREATE(ctx *Context) error {
	if ctx.ArgCount() < 1 {
		return ctx.WriteError(ErrWrongArgCount)
	}
	name := ctx.ArgString(0)
	maxRetries := int64(3)
	if ctx.ArgCount() >= 2 {
		maxRetries = parseInt64(ctx.ArgString(1))
	}
	msgQueuesMu.Lock()
	msgQueues[name] = &MessageQueue{
		Name:        name,
		Messages:    make([]*QueuedMessage, 0),
		DeadLetters: make([]*QueuedMessage, 0),
		AckWait:     make(map[string]*QueuedMessage),
		MaxRetries:  int(maxRetries),
	}
	msgQueuesMu.Unlock()
	return ctx.WriteOK()
}

func cmdMSGQUEUEPUBLISH(ctx *Context) error {
	if ctx.ArgCount() < 2 {
		return ctx.WriteError(ErrWrongArgCount)
	}
	name := ctx.ArgString(0)
	body := ctx.ArgString(1)
	msgQueuesMu.Lock()
	defer msgQueuesMu.Unlock()
	queue, exists := msgQueues[name]
	if !exists {
		return ctx.WriteError(fmt.Errorf("ERR queue not found"))
	}
	msg := &QueuedMessage{
		ID:        generateUUID(),
		Body:      body,
		Status:    "pending",
		Retries:   0,
		CreatedAt: time.Now().UnixMilli(),
	}
	queue.Messages = append(queue.Messages, msg)
	return ctx.WriteBulkString(msg.ID)
}

func cmdMSGQUEUECONSUME(ctx *Context) error {
	if ctx.ArgCount() < 1 {
		return ctx.WriteError(ErrWrongArgCount)
	}
	name := ctx.ArgString(0)
	timeout := int64(5000)
	if ctx.ArgCount() >= 2 {
		timeout = parseInt64(ctx.ArgString(1))
	}
	msgQueuesMu.Lock()
	defer msgQueuesMu.Unlock()
	queue, exists := msgQueues[name]
	if !exists {
		return ctx.WriteError(fmt.Errorf("ERR queue not found"))
	}
	for len(queue.Messages) > 0 {
		msg := queue.Messages[0]
		queue.Messages = queue.Messages[1:]
		if msg.Status == "pending" {
			msg.Status = "processing"
			msg.AckAt = time.Now().UnixMilli() + timeout
			queue.AckWait[msg.ID] = msg
			return ctx.WriteArray([]*resp.Value{
				resp.BulkString("id"), resp.BulkString(msg.ID),
				resp.BulkString("body"), resp.BulkString(msg.Body),
				resp.BulkString("retries"), resp.IntegerValue(int64(msg.Retries)),
			})
		}
	}
	return ctx.WriteNull()
}

func cmdMSGQUEUEACK(ctx *Context) error {
	if ctx.ArgCount() < 2 {
		return ctx.WriteError(ErrWrongArgCount)
	}
	name := ctx.ArgString(0)
	msgID := ctx.ArgString(1)
	msgQueuesMu.Lock()
	defer msgQueuesMu.Unlock()
	queue, exists := msgQueues[name]
	if !exists {
		return ctx.WriteError(fmt.Errorf("ERR queue not found"))
	}
	if _, ok := queue.AckWait[msgID]; ok {
		delete(queue.AckWait, msgID)
		return ctx.WriteInteger(1)
	}
	return ctx.WriteInteger(0)
}

func cmdMSGQUEUENACK(ctx *Context) error {
	if ctx.ArgCount() < 2 {
		return ctx.WriteError(ErrWrongArgCount)
	}
	name := ctx.ArgString(0)
	msgID := ctx.ArgString(1)
	msgQueuesMu.Lock()
	defer msgQueuesMu.Unlock()
	queue, exists := msgQueues[name]
	if !exists {
		return ctx.WriteError(fmt.Errorf("ERR queue not found"))
	}
	if msg, ok := queue.AckWait[msgID]; ok {
		delete(queue.AckWait, msgID)
		msg.Retries++
		msg.Status = "pending"
		if msg.Retries >= queue.MaxRetries {
			queue.DeadLetters = append(queue.DeadLetters, msg)
			return ctx.WriteInteger(2)
		}
		queue.Messages = append(queue.Messages, msg)
		return ctx.WriteInteger(1)
	}
	return ctx.WriteInteger(0)
}

func cmdMSGQUEUEDEADLETTER(ctx *Context) error {
	if ctx.ArgCount() < 1 {
		return ctx.WriteError(ErrWrongArgCount)
	}
	name := ctx.ArgString(0)
	msgQueuesMu.RLock()
	queue, exists := msgQueues[name]
	msgQueuesMu.RUnlock()
	if !exists {
		return ctx.WriteError(fmt.Errorf("ERR queue not found"))
	}
	results := make([]*resp.Value, 0)
	for _, msg := range queue.DeadLetters {
		results = append(results, resp.ArrayValue([]*resp.Value{
			resp.BulkString("id"), resp.BulkString(msg.ID),
			resp.BulkString("body"), resp.BulkString(msg.Body),
			resp.BulkString("retries"), resp.IntegerValue(int64(msg.Retries)),
		}))
	}
	return ctx.WriteArray(results)
}

func cmdMSGQUEUEREQUEUE(ctx *Context) error {
	if ctx.ArgCount() < 2 {
		return ctx.WriteError(ErrWrongArgCount)
	}
	name := ctx.ArgString(0)
	msgID := ctx.ArgString(1)
	msgQueuesMu.Lock()
	defer msgQueuesMu.Unlock()
	queue, exists := msgQueues[name]
	if !exists {
		return ctx.WriteError(fmt.Errorf("ERR queue not found"))
	}
	for i, msg := range queue.DeadLetters {
		if msg.ID == msgID {
			queue.DeadLetters = append(queue.DeadLetters[:i], queue.DeadLetters[i+1:]...)
			msg.Retries = 0
			msg.Status = "pending"
			queue.Messages = append(queue.Messages, msg)
			return ctx.WriteInteger(1)
		}
	}
	return ctx.WriteInteger(0)
}

func cmdMSGQUEUEPURGE(ctx *Context) error {
	if ctx.ArgCount() < 1 {
		return ctx.WriteError(ErrWrongArgCount)
	}
	name := ctx.ArgString(0)
	msgQueuesMu.Lock()
	defer msgQueuesMu.Unlock()
	queue, exists := msgQueues[name]
	if !exists {
		return ctx.WriteError(fmt.Errorf("ERR queue not found"))
	}
	count := len(queue.Messages)
	queue.Messages = make([]*QueuedMessage, 0)
	return ctx.WriteInteger(int64(count))
}

func cmdMSGQUEUESTATS(ctx *Context) error {
	if ctx.ArgCount() < 1 {
		return ctx.WriteError(ErrWrongArgCount)
	}
	name := ctx.ArgString(0)
	msgQueuesMu.RLock()
	queue, exists := msgQueues[name]
	msgQueuesMu.RUnlock()
	if !exists {
		return ctx.WriteError(fmt.Errorf("ERR queue not found"))
	}
	return ctx.WriteArray([]*resp.Value{
		resp.BulkString("name"), resp.BulkString(queue.Name),
		resp.BulkString("pending"), resp.IntegerValue(int64(len(queue.Messages))),
		resp.BulkString("processing"), resp.IntegerValue(int64(len(queue.AckWait))),
		resp.BulkString("dead_letters"), resp.IntegerValue(int64(len(queue.DeadLetters))),
		resp.BulkString("max_retries"), resp.IntegerValue(int64(queue.MaxRetries)),
	})
}

func cmdMSGQUEUEDELETE(ctx *Context) error {
	if ctx.ArgCount() < 1 {
		return ctx.WriteError(ErrWrongArgCount)
	}
	name := ctx.ArgString(0)
	msgQueuesMu.Lock()
	defer msgQueuesMu.Unlock()
	if _, exists := msgQueues[name]; !exists {
		return ctx.WriteInteger(0)
	}
	delete(msgQueues, name)
	return ctx.WriteInteger(1)
}

var (
	services   = make(map[string]map[string]*ServiceInstance)
	servicesMu sync.RWMutex
)

type ServiceInstance struct {
	ID         string
	Name       string
	Address    string
	Port       int
	Weight     int
	Tags       []string
	Metadata   map[string]string
	LastSeen   int64
	Registered int64
}

func cmdSERVICEREGISTER(ctx *Context) error {
	if ctx.ArgCount() < 4 {
		return ctx.WriteError(ErrWrongArgCount)
	}
	name := ctx.ArgString(0)
	id := ctx.ArgString(1)
	address := ctx.ArgString(2)
	port := int(parseInt64(ctx.ArgString(3)))
	servicesMu.Lock()
	defer servicesMu.Unlock()
	if _, exists := services[name]; !exists {
		services[name] = make(map[string]*ServiceInstance)
	}
	instance := &ServiceInstance{
		ID:         id,
		Name:       name,
		Address:    address,
		Port:       port,
		Weight:     100,
		Tags:       []string{},
		Metadata:   make(map[string]string),
		LastSeen:   time.Now().UnixMilli(),
		Registered: time.Now().UnixMilli(),
	}
	if ctx.ArgCount() >= 5 {
		instance.Weight = int(parseInt64(ctx.ArgString(4)))
	}
	services[name][id] = instance
	return ctx.WriteOK()
}

func cmdSERVICEDEREGISTER(ctx *Context) error {
	if ctx.ArgCount() < 2 {
		return ctx.WriteError(ErrWrongArgCount)
	}
	name := ctx.ArgString(0)
	id := ctx.ArgString(1)
	servicesMu.Lock()
	defer servicesMu.Unlock()
	if instances, exists := services[name]; exists {
		if _, ok := instances[id]; ok {
			delete(instances, id)
			if len(instances) == 0 {
				delete(services, name)
			}
			return ctx.WriteInteger(1)
		}
	}
	return ctx.WriteInteger(0)
}

func cmdSERVICEDISCOVER(ctx *Context) error {
	if ctx.ArgCount() < 1 {
		return ctx.WriteError(ErrWrongArgCount)
	}
	name := ctx.ArgString(0)
	servicesMu.RLock()
	defer servicesMu.RUnlock()
	instances, exists := services[name]
	if !exists {
		return ctx.WriteArray([]*resp.Value{})
	}
	results := make([]*resp.Value, 0)
	for _, inst := range instances {
		if time.Now().UnixMilli()-inst.LastSeen < 30000 {
			results = append(results, resp.ArrayValue([]*resp.Value{
				resp.BulkString("id"), resp.BulkString(inst.ID),
				resp.BulkString("address"), resp.BulkString(inst.Address),
				resp.BulkString("port"), resp.IntegerValue(int64(inst.Port)),
				resp.BulkString("weight"), resp.IntegerValue(int64(inst.Weight)),
			}))
		}
	}
	return ctx.WriteArray(results)
}

func cmdSERVICEHEARTBEAT(ctx *Context) error {
	if ctx.ArgCount() < 2 {
		return ctx.WriteError(ErrWrongArgCount)
	}
	name := ctx.ArgString(0)
	id := ctx.ArgString(1)
	servicesMu.Lock()
	defer servicesMu.Unlock()
	if instances, exists := services[name]; exists {
		if inst, ok := instances[id]; ok {
			inst.LastSeen = time.Now().UnixMilli()
			return ctx.WriteOK()
		}
	}
	return ctx.WriteError(fmt.Errorf("ERR service not found"))
}

func cmdSERVICELIST(ctx *Context) error {
	servicesMu.RLock()
	defer servicesMu.RUnlock()
	results := make([]*resp.Value, 0)
	for name := range services {
		results = append(results, resp.BulkString(name))
	}
	return ctx.WriteArray(results)
}

func cmdSERVICEHEALTHY(ctx *Context) error {
	if ctx.ArgCount() < 1 {
		return ctx.WriteError(ErrWrongArgCount)
	}
	name := ctx.ArgString(0)
	servicesMu.RLock()
	defer servicesMu.RUnlock()
	instances, exists := services[name]
	if !exists {
		return ctx.WriteArray([]*resp.Value{})
	}
	results := make([]*resp.Value, 0)
	for _, inst := range instances {
		if time.Now().UnixMilli()-inst.LastSeen < 30000 {
			results = append(results, resp.BulkString(inst.ID))
		}
	}
	return ctx.WriteArray(results)
}

func cmdSERVICEWEIGHT(ctx *Context) error {
	if ctx.ArgCount() < 3 {
		return ctx.WriteError(ErrWrongArgCount)
	}
	name := ctx.ArgString(0)
	id := ctx.ArgString(1)
	weight := int(parseInt64(ctx.ArgString(2)))
	servicesMu.Lock()
	defer servicesMu.Unlock()
	if instances, exists := services[name]; exists {
		if inst, ok := instances[id]; ok {
			inst.Weight = weight
			return ctx.WriteOK()
		}
	}
	return ctx.WriteError(fmt.Errorf("ERR service not found"))
}

func cmdSERVICETAGS(ctx *Context) error {
	if ctx.ArgCount() < 3 {
		return ctx.WriteError(ErrWrongArgCount)
	}
	name := ctx.ArgString(0)
	id := ctx.ArgString(1)
	tags := make([]string, 0)
	for i := 2; i < ctx.ArgCount(); i++ {
		tags = append(tags, ctx.ArgString(i))
	}
	servicesMu.Lock()
	defer servicesMu.Unlock()
	if instances, exists := services[name]; exists {
		if inst, ok := instances[id]; ok {
			inst.Tags = tags
			return ctx.WriteOK()
		}
	}
	return ctx.WriteError(fmt.Errorf("ERR service not found"))
}

var (
	healthChecksX    = make(map[string]*HealthCheckX)
	healthChecksXMux sync.RWMutex
)

type HealthCheckX struct {
	Name      string
	Target    string
	Type      string
	Interval  int64
	Timeout   int64
	Status    string
	LastCheck int64
	History   []*HealthCheckResultX
}

type HealthCheckResultX struct {
	Timestamp int64
	Status    string
	Latency   int64
	Message   string
}

func cmdHEALTHXREGISTER(ctx *Context) error {
	if ctx.ArgCount() < 3 {
		return ctx.WriteError(ErrWrongArgCount)
	}
	name := ctx.ArgString(0)
	target := ctx.ArgString(1)
	checkType := ctx.ArgString(2)
	interval := int64(30000)
	if ctx.ArgCount() >= 4 {
		interval = parseInt64(ctx.ArgString(3))
	}
	healthChecksXMux.Lock()
	healthChecksX[name] = &HealthCheckX{
		Name:      name,
		Target:    target,
		Type:      checkType,
		Interval:  interval,
		Timeout:   5000,
		Status:    "unknown",
		LastCheck: 0,
		History:   make([]*HealthCheckResultX, 0),
	}
	healthChecksXMux.Unlock()
	return ctx.WriteOK()
}

func cmdHEALTHXUNREGISTER(ctx *Context) error {
	if ctx.ArgCount() < 1 {
		return ctx.WriteError(ErrWrongArgCount)
	}
	name := ctx.ArgString(0)
	healthChecksXMux.Lock()
	defer healthChecksXMux.Unlock()
	if _, exists := healthChecksX[name]; !exists {
		return ctx.WriteInteger(0)
	}
	delete(healthChecksX, name)
	return ctx.WriteInteger(1)
}

func cmdHEALTHXCHECK(ctx *Context) error {
	if ctx.ArgCount() < 1 {
		return ctx.WriteError(ErrWrongArgCount)
	}
	name := ctx.ArgString(0)
	healthChecksXMux.Lock()
	defer healthChecksXMux.Unlock()
	check, exists := healthChecksX[name]
	if !exists {
		return ctx.WriteError(fmt.Errorf("ERR health check not found"))
	}
	now := time.Now().UnixMilli()
	check.LastCheck = now
	check.Status = "healthy"
	result := &HealthCheckResultX{
		Timestamp: now,
		Status:    "healthy",
		Latency:   1,
		Message:   "OK",
	}
	check.History = append(check.History, result)
	if len(check.History) > 100 {
		check.History = check.History[1:]
	}
	return ctx.WriteArray([]*resp.Value{
		resp.BulkString("name"), resp.BulkString(check.Name),
		resp.BulkString("status"), resp.BulkString(check.Status),
		resp.BulkString("latency_ms"), resp.IntegerValue(result.Latency),
	})
}

func cmdHEALTHXSTATUS(ctx *Context) error {
	if ctx.ArgCount() < 1 {
		return ctx.WriteError(ErrWrongArgCount)
	}
	name := ctx.ArgString(0)
	healthChecksXMux.RLock()
	check, exists := healthChecksX[name]
	healthChecksXMux.RUnlock()
	if !exists {
		return ctx.WriteError(fmt.Errorf("ERR health check not found"))
	}
	return ctx.WriteArray([]*resp.Value{
		resp.BulkString("name"), resp.BulkString(check.Name),
		resp.BulkString("target"), resp.BulkString(check.Target),
		resp.BulkString("type"), resp.BulkString(check.Type),
		resp.BulkString("status"), resp.BulkString(check.Status),
		resp.BulkString("last_check"), resp.IntegerValue(check.LastCheck),
	})
}

func cmdHEALTHXHISTORY(ctx *Context) error {
	if ctx.ArgCount() < 1 {
		return ctx.WriteError(ErrWrongArgCount)
	}
	name := ctx.ArgString(0)
	limit := 10
	if ctx.ArgCount() >= 2 {
		limit = int(parseInt64(ctx.ArgString(1)))
	}
	healthChecksXMux.RLock()
	check, exists := healthChecksX[name]
	healthChecksXMux.RUnlock()
	if !exists {
		return ctx.WriteError(fmt.Errorf("ERR health check not found"))
	}
	results := make([]*resp.Value, 0)
	start := len(check.History) - limit
	if start < 0 {
		start = 0
	}
	for i := start; i < len(check.History); i++ {
		r := check.History[i]
		results = append(results, resp.ArrayValue([]*resp.Value{
			resp.BulkString("timestamp"), resp.IntegerValue(r.Timestamp),
			resp.BulkString("status"), resp.BulkString(r.Status),
			resp.BulkString("latency_ms"), resp.IntegerValue(r.Latency),
			resp.BulkString("message"), resp.BulkString(r.Message),
		}))
	}
	return ctx.WriteArray(results)
}

func cmdHEALTHXLIST(ctx *Context) error {
	healthChecksXMux.RLock()
	defer healthChecksXMux.RUnlock()
	results := make([]*resp.Value, 0)
	for name, check := range healthChecksX {
		results = append(results, resp.ArrayValue([]*resp.Value{
			resp.BulkString("name"), resp.BulkString(name),
			resp.BulkString("status"), resp.BulkString(check.Status),
		}))
	}
	return ctx.WriteArray(results)
}

var (
	cronJobs   = make(map[string]*CronJob)
	cronJobsMu sync.RWMutex
)

type CronJob struct {
	Name    string
	Expr    string
	Command string
	Status  string
	LastRun int64
	NextRun int64
	History []*CronJobRun
	Paused  bool
}

type CronJobRun struct {
	Timestamp int64
	Status    string
	Output    string
}

func cmdCRONADD(ctx *Context) error {
	if ctx.ArgCount() < 3 {
		return ctx.WriteError(ErrWrongArgCount)
	}
	name := ctx.ArgString(0)
	expr := ctx.ArgString(1)
	cmd := ctx.ArgString(2)
	cronJobsMu.Lock()
	cronJobs[name] = &CronJob{
		Name:    name,
		Expr:    expr,
		Command: cmd,
		Status:  "active",
		LastRun: 0,
		NextRun: time.Now().UnixMilli() + 60000,
		History: make([]*CronJobRun, 0),
		Paused:  false,
	}
	cronJobsMu.Unlock()
	return ctx.WriteOK()
}

func cmdCRONREMOVE(ctx *Context) error {
	if ctx.ArgCount() < 1 {
		return ctx.WriteError(ErrWrongArgCount)
	}
	name := ctx.ArgString(0)
	cronJobsMu.Lock()
	defer cronJobsMu.Unlock()
	if _, exists := cronJobs[name]; !exists {
		return ctx.WriteInteger(0)
	}
	delete(cronJobs, name)
	return ctx.WriteInteger(1)
}

func cmdCRONLIST(ctx *Context) error {
	cronJobsMu.RLock()
	defer cronJobsMu.RUnlock()
	results := make([]*resp.Value, 0)
	for name, job := range cronJobs {
		results = append(results, resp.ArrayValue([]*resp.Value{
			resp.BulkString("name"), resp.BulkString(name),
			resp.BulkString("expr"), resp.BulkString(job.Expr),
			resp.BulkString("status"), resp.BulkString(job.Status),
			resp.BulkString("paused"), resp.BulkString(fmt.Sprintf("%v", job.Paused)),
		}))
	}
	return ctx.WriteArray(results)
}

func cmdCRONTRIGGER(ctx *Context) error {
	if ctx.ArgCount() < 1 {
		return ctx.WriteError(ErrWrongArgCount)
	}
	name := ctx.ArgString(0)
	cronJobsMu.Lock()
	defer cronJobsMu.Unlock()
	job, exists := cronJobs[name]
	if !exists {
		return ctx.WriteError(fmt.Errorf("ERR cron job not found"))
	}
	job.LastRun = time.Now().UnixMilli()
	job.History = append(job.History, &CronJobRun{
		Timestamp: job.LastRun,
		Status:    "success",
		Output:    "triggered manually",
	})
	if len(job.History) > 100 {
		job.History = job.History[1:]
	}
	return ctx.WriteOK()
}

func cmdCRONPAUSE(ctx *Context) error {
	if ctx.ArgCount() < 1 {
		return ctx.WriteError(ErrWrongArgCount)
	}
	name := ctx.ArgString(0)
	cronJobsMu.Lock()
	defer cronJobsMu.Unlock()
	job, exists := cronJobs[name]
	if !exists {
		return ctx.WriteError(fmt.Errorf("ERR cron job not found"))
	}
	job.Paused = true
	job.Status = "paused"
	return ctx.WriteOK()
}

func cmdCRONRESUME(ctx *Context) error {
	if ctx.ArgCount() < 1 {
		return ctx.WriteError(ErrWrongArgCount)
	}
	name := ctx.ArgString(0)
	cronJobsMu.Lock()
	defer cronJobsMu.Unlock()
	job, exists := cronJobs[name]
	if !exists {
		return ctx.WriteError(fmt.Errorf("ERR cron job not found"))
	}
	job.Paused = false
	job.Status = "active"
	return ctx.WriteOK()
}

func cmdCRONNEXT(ctx *Context) error {
	if ctx.ArgCount() < 1 {
		return ctx.WriteError(ErrWrongArgCount)
	}
	name := ctx.ArgString(0)
	cronJobsMu.RLock()
	job, exists := cronJobs[name]
	cronJobsMu.RUnlock()
	if !exists {
		return ctx.WriteError(fmt.Errorf("ERR cron job not found"))
	}
	return ctx.WriteInteger(job.NextRun)
}

func cmdCRONHISTORY(ctx *Context) error {
	if ctx.ArgCount() < 1 {
		return ctx.WriteError(ErrWrongArgCount)
	}
	name := ctx.ArgString(0)
	limit := 10
	if ctx.ArgCount() >= 2 {
		limit = int(parseInt64(ctx.ArgString(1)))
	}
	cronJobsMu.RLock()
	job, exists := cronJobs[name]
	cronJobsMu.RUnlock()
	if !exists {
		return ctx.WriteError(fmt.Errorf("ERR cron job not found"))
	}
	results := make([]*resp.Value, 0)
	start := len(job.History) - limit
	if start < 0 {
		start = 0
	}
	for i := start; i < len(job.History); i++ {
		r := job.History[i]
		results = append(results, resp.ArrayValue([]*resp.Value{
			resp.BulkString("timestamp"), resp.IntegerValue(r.Timestamp),
			resp.BulkString("status"), resp.BulkString(r.Status),
			resp.BulkString("output"), resp.BulkString(r.Output),
		}))
	}
	return ctx.WriteArray(results)
}

var (
	vectorStores   = make(map[string]*VectorStore)
	vectorStoresMu sync.RWMutex
)

type VectorStore struct {
	Name      string
	Dim       int
	Vectors   map[string]*Vector
	Normalize bool
}

type Vector struct {
	ID   string
	Data []float64
	Meta map[string]string
}

func cmdVECTORCREATE(ctx *Context) error {
	if ctx.ArgCount() < 2 {
		return ctx.WriteError(ErrWrongArgCount)
	}
	name := ctx.ArgString(0)
	dim := int(parseInt64(ctx.ArgString(1)))
	normalize := false
	if ctx.ArgCount() >= 3 && ctx.ArgString(2) == "NORMALIZE" {
		normalize = true
	}
	vectorStoresMu.Lock()
	vectorStores[name] = &VectorStore{
		Name:      name,
		Dim:       dim,
		Vectors:   make(map[string]*Vector),
		Normalize: normalize,
	}
	vectorStoresMu.Unlock()
	return ctx.WriteOK()
}

func cmdVECTORADD(ctx *Context) error {
	if ctx.ArgCount() < 3 {
		return ctx.WriteError(ErrWrongArgCount)
	}
	name := ctx.ArgString(0)
	id := ctx.ArgString(1)
	dim := int(parseInt64(ctx.ArgString(2)))
	data := make([]float64, dim)
	for i := 0; i < dim && i+3 < ctx.ArgCount(); i++ {
		data[i] = parseFloatExt([]byte(ctx.ArgString(3 + i)))
	}
	vectorStoresMu.Lock()
	defer vectorStoresMu.Unlock()
	store, exists := vectorStores[name]
	if !exists {
		return ctx.WriteError(fmt.Errorf("ERR vector store not found"))
	}
	if store.Normalize {
		norm := 0.0
		for _, v := range data {
			norm += v * v
		}
		norm = sqrtFloat(norm)
		if norm > 0 {
			for i := range data {
				data[i] /= norm
			}
		}
	}
	store.Vectors[id] = &Vector{ID: id, Data: data, Meta: make(map[string]string)}
	return ctx.WriteOK()
}

func cmdVECTORGET(ctx *Context) error {
	if ctx.ArgCount() < 2 {
		return ctx.WriteError(ErrWrongArgCount)
	}
	name := ctx.ArgString(0)
	id := ctx.ArgString(1)
	vectorStoresMu.RLock()
	store, exists := vectorStores[name]
	vectorStoresMu.RUnlock()
	if !exists {
		return ctx.WriteError(fmt.Errorf("ERR vector store not found"))
	}
	vec, ok := store.Vectors[id]
	if !ok {
		return ctx.WriteNull()
	}
	results := make([]*resp.Value, len(vec.Data))
	for i, v := range vec.Data {
		results[i] = resp.BulkString(fmt.Sprintf("%.6f", v))
	}
	return ctx.WriteArray(results)
}

func cmdVECTORDELETE(ctx *Context) error {
	if ctx.ArgCount() < 2 {
		return ctx.WriteError(ErrWrongArgCount)
	}
	name := ctx.ArgString(0)
	id := ctx.ArgString(1)
	vectorStoresMu.Lock()
	defer vectorStoresMu.Unlock()
	store, exists := vectorStores[name]
	if !exists {
		return ctx.WriteError(fmt.Errorf("ERR vector store not found"))
	}
	if _, ok := store.Vectors[id]; ok {
		delete(store.Vectors, id)
		return ctx.WriteInteger(1)
	}
	return ctx.WriteInteger(0)
}

func cmdVECTORSEARCH(ctx *Context) error {
	if ctx.ArgCount() < 3 {
		return ctx.WriteError(ErrWrongArgCount)
	}
	name := ctx.ArgString(0)
	k := int(parseInt64(ctx.ArgString(1)))
	dim := int(parseInt64(ctx.ArgString(2)))
	query := make([]float64, dim)
	for i := 0; i < dim && i+3 < ctx.ArgCount(); i++ {
		query[i] = parseFloatExt([]byte(ctx.ArgString(3 + i)))
	}
	vectorStoresMu.RLock()
	store, exists := vectorStores[name]
	vectorStoresMu.RUnlock()
	if !exists {
		return ctx.WriteError(fmt.Errorf("ERR vector store not found"))
	}
	type scoreResult struct {
		ID    string
		Score float64
	}
	results := make([]scoreResult, 0)
	for id, vec := range store.Vectors {
		score := cosineSimilarity(query, vec.Data)
		results = append(results, scoreResult{ID: id, Score: score})
	}
	sort.Slice(results, func(i, j int) bool { return results[i].Score > results[j].Score })
	if k > len(results) {
		k = len(results)
	}
	output := make([]*resp.Value, 0)
	for i := 0; i < k; i++ {
		output = append(output, resp.ArrayValue([]*resp.Value{
			resp.BulkString("id"), resp.BulkString(results[i].ID),
			resp.BulkString("score"), resp.BulkString(fmt.Sprintf("%.6f", results[i].Score)),
		}))
	}
	return ctx.WriteArray(output)
}

func cosineSimilarity(a, b []float64) float64 {
	if len(a) != len(b) {
		return 0
	}
	var dot, normA, normB float64
	for i := range a {
		dot += a[i] * b[i]
		normA += a[i] * a[i]
		normB += b[i] * b[i]
	}
	if normA == 0 || normB == 0 {
		return 0
	}
	return dot / (sqrtFloat(normA) * sqrtFloat(normB))
}

func cmdVECTORSIMILARITY(ctx *Context) error {
	if ctx.ArgCount() < 3 {
		return ctx.WriteError(ErrWrongArgCount)
	}
	name := ctx.ArgString(0)
	id1 := ctx.ArgString(1)
	id2 := ctx.ArgString(2)
	vectorStoresMu.RLock()
	store, exists := vectorStores[name]
	vectorStoresMu.RUnlock()
	if !exists {
		return ctx.WriteError(fmt.Errorf("ERR vector store not found"))
	}
	vec1, ok1 := store.Vectors[id1]
	vec2, ok2 := store.Vectors[id2]
	if !ok1 || !ok2 {
		return ctx.WriteError(fmt.Errorf("ERR vector not found"))
	}
	sim := cosineSimilarity(vec1.Data, vec2.Data)
	return ctx.WriteBulkString(fmt.Sprintf("%.6f", sim))
}

func cmdVECTORNORMALIZE(ctx *Context) error {
	if ctx.ArgCount() < 2 {
		return ctx.WriteError(ErrWrongArgCount)
	}
	name := ctx.ArgString(0)
	id := ctx.ArgString(1)
	vectorStoresMu.Lock()
	defer vectorStoresMu.Unlock()
	store, exists := vectorStores[name]
	if !exists {
		return ctx.WriteError(fmt.Errorf("ERR vector store not found"))
	}
	vec, ok := store.Vectors[id]
	if !ok {
		return ctx.WriteError(fmt.Errorf("ERR vector not found"))
	}
	norm := 0.0
	for _, v := range vec.Data {
		norm += v * v
	}
	norm = sqrtFloat(norm)
	if norm > 0 {
		for i := range vec.Data {
			vec.Data[i] /= norm
		}
	}
	return ctx.WriteOK()
}

func cmdVECTORDIMENSIONS(ctx *Context) error {
	if ctx.ArgCount() < 1 {
		return ctx.WriteError(ErrWrongArgCount)
	}
	name := ctx.ArgString(0)
	vectorStoresMu.RLock()
	store, exists := vectorStores[name]
	vectorStoresMu.RUnlock()
	if !exists {
		return ctx.WriteError(fmt.Errorf("ERR vector store not found"))
	}
	return ctx.WriteInteger(int64(store.Dim))
}

func cmdVECTORMERGE(ctx *Context) error {
	if ctx.ArgCount() < 3 {
		return ctx.WriteError(ErrWrongArgCount)
	}
	name := ctx.ArgString(0)
	id1 := ctx.ArgString(1)
	id2 := ctx.ArgString(2)
	newID := id1 + "_" + id2
	if ctx.ArgCount() >= 4 {
		newID = ctx.ArgString(3)
	}
	vectorStoresMu.Lock()
	defer vectorStoresMu.Unlock()
	store, exists := vectorStores[name]
	if !exists {
		return ctx.WriteError(fmt.Errorf("ERR vector store not found"))
	}
	vec1, ok1 := store.Vectors[id1]
	vec2, ok2 := store.Vectors[id2]
	if !ok1 || !ok2 {
		return ctx.WriteError(fmt.Errorf("ERR vector not found"))
	}
	if len(vec1.Data) != len(vec2.Data) {
		return ctx.WriteError(fmt.Errorf("ERR dimension mismatch"))
	}
	merged := make([]float64, len(vec1.Data))
	for i := range vec1.Data {
		merged[i] = (vec1.Data[i] + vec2.Data[i]) / 2
	}
	store.Vectors[newID] = &Vector{ID: newID, Data: merged, Meta: make(map[string]string)}
	return ctx.WriteBulkString(newID)
}

func cmdVECTORSTATS(ctx *Context) error {
	if ctx.ArgCount() < 1 {
		return ctx.WriteError(ErrWrongArgCount)
	}
	name := ctx.ArgString(0)
	vectorStoresMu.RLock()
	store, exists := vectorStores[name]
	vectorStoresMu.RUnlock()
	if !exists {
		return ctx.WriteError(fmt.Errorf("ERR vector store not found"))
	}
	return ctx.WriteArray([]*resp.Value{
		resp.BulkString("name"), resp.BulkString(store.Name),
		resp.BulkString("dimensions"), resp.IntegerValue(int64(store.Dim)),
		resp.BulkString("vectors"), resp.IntegerValue(int64(len(store.Vectors))),
		resp.BulkString("normalize"), resp.BulkString(fmt.Sprintf("%v", store.Normalize)),
	})
}

var (
	docStores   = make(map[string]*DocStore)
	docStoresMu sync.RWMutex
)

type DocStore struct {
	Name    string
	Docs    map[string]map[string]string
	Indexes map[string]map[string][]string
}

func cmdDOCINSERT(ctx *Context) error {
	if ctx.ArgCount() < 2 {
		return ctx.WriteError(ErrWrongArgCount)
	}
	name := ctx.ArgString(0)
	id := ctx.ArgString(1)
	doc := make(map[string]string)
	for i := 2; i+1 < ctx.ArgCount(); i += 2 {
		doc[ctx.ArgString(i)] = ctx.ArgString(i + 1)
	}
	docStoresMu.Lock()
	defer docStoresMu.Unlock()
	store, exists := docStores[name]
	if !exists {
		store = &DocStore{Name: name, Docs: make(map[string]map[string]string), Indexes: make(map[string]map[string][]string)}
		docStores[name] = store
	}
	store.Docs[id] = doc
	for field, value := range doc {
		if _, ok := store.Indexes[field]; !ok {
			store.Indexes[field] = make(map[string][]string)
		}
		store.Indexes[field][value] = append(store.Indexes[field][value], id)
	}
	return ctx.WriteBulkString(id)
}

func cmdDOCFIND(ctx *Context) error {
	if ctx.ArgCount() < 1 {
		return ctx.WriteError(ErrWrongArgCount)
	}
	name := ctx.ArgString(0)
	docStoresMu.RLock()
	store, exists := docStores[name]
	docStoresMu.RUnlock()
	if !exists {
		return ctx.WriteArray([]*resp.Value{})
	}
	var filterField, filterValue string
	if ctx.ArgCount() >= 3 {
		filterField = ctx.ArgString(1)
		filterValue = ctx.ArgString(2)
	}
	results := make([]*resp.Value, 0)
	if filterField != "" {
		if idx, ok := store.Indexes[filterField]; ok {
			if ids, ok2 := idx[filterValue]; ok2 {
				for _, id := range ids {
					if doc, ok3 := store.Docs[id]; ok3 {
						docResult := []*resp.Value{resp.BulkString("_id"), resp.BulkString(id)}
						for k, v := range doc {
							docResult = append(docResult, resp.BulkString(k), resp.BulkString(v))
						}
						results = append(results, resp.ArrayValue(docResult))
					}
				}
			}
		}
	} else {
		for id, doc := range store.Docs {
			docResult := []*resp.Value{resp.BulkString("_id"), resp.BulkString(id)}
			for k, v := range doc {
				docResult = append(docResult, resp.BulkString(k), resp.BulkString(v))
			}
			results = append(results, resp.ArrayValue(docResult))
		}
	}
	return ctx.WriteArray(results)
}

func cmdDOCFINDONE(ctx *Context) error {
	if ctx.ArgCount() < 3 {
		return ctx.WriteError(ErrWrongArgCount)
	}
	name := ctx.ArgString(0)
	field := ctx.ArgString(1)
	value := ctx.ArgString(2)
	docStoresMu.RLock()
	store, exists := docStores[name]
	docStoresMu.RUnlock()
	if !exists {
		return ctx.WriteNull()
	}
	if idx, ok := store.Indexes[field]; ok {
		if ids, ok2 := idx[value]; ok2 && len(ids) > 0 {
			if doc, ok3 := store.Docs[ids[0]]; ok3 {
				results := []*resp.Value{resp.BulkString("_id"), resp.BulkString(ids[0])}
				for k, v := range doc {
					results = append(results, resp.BulkString(k), resp.BulkString(v))
				}
				return ctx.WriteArray(results)
			}
		}
	}
	return ctx.WriteNull()
}

func cmdDOCUPDATE(ctx *Context) error {
	if ctx.ArgCount() < 2 {
		return ctx.WriteError(ErrWrongArgCount)
	}
	name := ctx.ArgString(0)
	id := ctx.ArgString(1)
	docStoresMu.Lock()
	defer docStoresMu.Unlock()
	store, exists := docStores[name]
	if !exists {
		return ctx.WriteError(fmt.Errorf("ERR doc store not found"))
	}
	doc, ok := store.Docs[id]
	if !ok {
		return ctx.WriteError(fmt.Errorf("ERR document not found"))
	}
	for i := 2; i+1 < ctx.ArgCount(); i += 2 {
		doc[ctx.ArgString(i)] = ctx.ArgString(i + 1)
	}
	return ctx.WriteOK()
}

func cmdDOCDELETE(ctx *Context) error {
	if ctx.ArgCount() < 2 {
		return ctx.WriteError(ErrWrongArgCount)
	}
	name := ctx.ArgString(0)
	id := ctx.ArgString(1)
	docStoresMu.Lock()
	defer docStoresMu.Unlock()
	store, exists := docStores[name]
	if !exists {
		return ctx.WriteInteger(0)
	}
	if _, ok := store.Docs[id]; ok {
		delete(store.Docs, id)
		return ctx.WriteInteger(1)
	}
	return ctx.WriteInteger(0)
}

func cmdDOCCOUNT(ctx *Context) error {
	if ctx.ArgCount() < 1 {
		return ctx.WriteError(ErrWrongArgCount)
	}
	name := ctx.ArgString(0)
	docStoresMu.RLock()
	store, exists := docStores[name]
	docStoresMu.RUnlock()
	if !exists {
		return ctx.WriteInteger(0)
	}
	return ctx.WriteInteger(int64(len(store.Docs)))
}

func docDOCDISTINCT(ctx *Context) error {
	if ctx.ArgCount() < 2 {
		return ctx.WriteError(ErrWrongArgCount)
	}
	name := ctx.ArgString(0)
	field := ctx.ArgString(1)
	docStoresMu.RLock()
	store, exists := docStores[name]
	docStoresMu.RUnlock()
	if !exists {
		return ctx.WriteArray([]*resp.Value{})
	}
	results := make([]*resp.Value, 0)
	if idx, ok := store.Indexes[field]; ok {
		for value := range idx {
			results = append(results, resp.BulkString(value))
		}
	}
	return ctx.WriteArray(results)
}

func cmdDOCAGGREGATE(ctx *Context) error {
	if ctx.ArgCount() < 2 {
		return ctx.WriteError(ErrWrongArgCount)
	}
	name := ctx.ArgString(0)
	aggType := ctx.ArgString(1)
	docStoresMu.RLock()
	store, exists := docStores[name]
	docStoresMu.RUnlock()
	if !exists {
		return ctx.WriteInteger(0)
	}
	switch aggType {
	case "count":
		return ctx.WriteInteger(int64(len(store.Docs)))
	case "fields":
		fields := make(map[string]bool)
		for _, doc := range store.Docs {
			for field := range doc {
				fields[field] = true
			}
		}
		results := make([]*resp.Value, 0)
		for field := range fields {
			results = append(results, resp.BulkString(field))
		}
		return ctx.WriteArray(results)
	default:
		return ctx.WriteError(fmt.Errorf("ERR unknown aggregation type"))
	}
}

func cmdDOCINDEX(ctx *Context) error {
	if ctx.ArgCount() < 2 {
		return ctx.WriteError(ErrWrongArgCount)
	}
	name := ctx.ArgString(0)
	field := ctx.ArgString(1)
	docStoresMu.Lock()
	defer docStoresMu.Unlock()
	store, exists := docStores[name]
	if !exists {
		store = &DocStore{Name: name, Docs: make(map[string]map[string]string), Indexes: make(map[string]map[string][]string)}
		docStores[name] = store
	}
	if _, ok := store.Indexes[field]; !ok {
		store.Indexes[field] = make(map[string][]string)
		for id, doc := range store.Docs {
			if value, exists := doc[field]; exists {
				store.Indexes[field][value] = append(store.Indexes[field][value], id)
			}
		}
	}
	return ctx.WriteOK()
}

func cmdDOCDROPINDEX(ctx *Context) error {
	if ctx.ArgCount() < 2 {
		return ctx.WriteError(ErrWrongArgCount)
	}
	name := ctx.ArgString(0)
	field := ctx.ArgString(1)
	docStoresMu.Lock()
	defer docStoresMu.Unlock()
	store, exists := docStores[name]
	if !exists {
		return ctx.WriteError(fmt.Errorf("ERR doc store not found"))
	}
	if _, ok := store.Indexes[field]; ok {
		delete(store.Indexes, field)
		return ctx.WriteInteger(1)
	}
	return ctx.WriteInteger(0)
}

var (
	topicSubs = make(map[string]map[string]bool)
	topicHist = make(map[string][]*TopicMessage)
	topicMu   sync.RWMutex
)

type TopicMessage struct {
	ID        string
	Topic     string
	Message   string
	Timestamp int64
}

func cmdTOPICSUBSCRIBE(ctx *Context) error {
	if ctx.ArgCount() < 2 {
		return ctx.WriteError(ErrWrongArgCount)
	}
	topic := ctx.ArgString(0)
	clientID := ctx.ArgString(1)
	topicMu.Lock()
	defer topicMu.Unlock()
	if _, exists := topicSubs[topic]; !exists {
		topicSubs[topic] = make(map[string]bool)
	}
	topicSubs[topic][clientID] = true
	return ctx.WriteOK()
}

func cmdTOPICUNSUBSCRIBE(ctx *Context) error {
	if ctx.ArgCount() < 2 {
		return ctx.WriteError(ErrWrongArgCount)
	}
	topic := ctx.ArgString(0)
	clientID := ctx.ArgString(1)
	topicMu.Lock()
	defer topicMu.Unlock()
	if subs, exists := topicSubs[topic]; exists {
		delete(subs, clientID)
		if len(subs) == 0 {
			delete(topicSubs, topic)
		}
	}
	return ctx.WriteOK()
}

func cmdTOPICPUBLISH(ctx *Context) error {
	if ctx.ArgCount() < 2 {
		return ctx.WriteError(ErrWrongArgCount)
	}
	topic := ctx.ArgString(0)
	message := ctx.ArgString(1)
	topicMu.Lock()
	defer topicMu.Unlock()
	msg := &TopicMessage{ID: generateUUID(), Topic: topic, Message: message, Timestamp: time.Now().UnixMilli()}
	topicHist[topic] = append(topicHist[topic], msg)
	if len(topicHist[topic]) > 100 {
		topicHist[topic] = topicHist[topic][1:]
	}
	subs := topicSubs[topic]
	return ctx.WriteInteger(int64(len(subs)))
}

func cmdTOPICSUBSCRIBERS(ctx *Context) error {
	if ctx.ArgCount() < 1 {
		return ctx.WriteError(ErrWrongArgCount)
	}
	topic := ctx.ArgString(0)
	topicMu.RLock()
	defer topicMu.RUnlock()
	subs, exists := topicSubs[topic]
	if !exists {
		return ctx.WriteArray([]*resp.Value{})
	}
	results := make([]*resp.Value, 0)
	for clientID := range subs {
		results = append(results, resp.BulkString(clientID))
	}
	return ctx.WriteArray(results)
}

func cmdTOPICLIST(ctx *Context) error {
	topicMu.RLock()
	defer topicMu.RUnlock()
	results := make([]*resp.Value, 0)
	for topic := range topicSubs {
		results = append(results, resp.BulkString(topic))
	}
	return ctx.WriteArray(results)
}

func cmdTOPICHISTORY(ctx *Context) error {
	if ctx.ArgCount() < 1 {
		return ctx.WriteError(ErrWrongArgCount)
	}
	topic := ctx.ArgString(0)
	limit := 10
	if ctx.ArgCount() >= 2 {
		limit = int(parseInt64(ctx.ArgString(1)))
	}
	topicMu.RLock()
	defer topicMu.RUnlock()
	hist, exists := topicHist[topic]
	if !exists {
		return ctx.WriteArray([]*resp.Value{})
	}
	start := len(hist) - limit
	if start < 0 {
		start = 0
	}
	results := make([]*resp.Value, 0)
	for i := start; i < len(hist); i++ {
		msg := hist[i]
		results = append(results, resp.ArrayValue([]*resp.Value{
			resp.BulkString("id"), resp.BulkString(msg.ID),
			resp.BulkString("message"), resp.BulkString(msg.Message),
			resp.BulkString("timestamp"), resp.IntegerValue(msg.Timestamp),
		}))
	}
	return ctx.WriteArray(results)
}

var (
	wsConns = make(map[string]*WSConnection)
	wsRooms = make(map[string]map[string]bool)
	wsMu    sync.RWMutex
)

type WSConnection struct {
	ID        string
	Rooms     []string
	Metadata  map[string]string
	CreatedAt int64
}

func cmdWSCONNECT(ctx *Context) error {
	if ctx.ArgCount() < 1 {
		return ctx.WriteError(ErrWrongArgCount)
	}
	id := ctx.ArgString(0)
	wsMu.Lock()
	defer wsMu.Unlock()
	wsConns[id] = &WSConnection{ID: id, Rooms: make([]string, 0), Metadata: make(map[string]string), CreatedAt: time.Now().UnixMilli()}
	return ctx.WriteOK()
}

func cmdWSDISCONNECT(ctx *Context) error {
	if ctx.ArgCount() < 1 {
		return ctx.WriteError(ErrWrongArgCount)
	}
	id := ctx.ArgString(0)
	wsMu.Lock()
	defer wsMu.Unlock()
	if conn, exists := wsConns[id]; exists {
		for _, room := range conn.Rooms {
			if members, ok := wsRooms[room]; ok {
				delete(members, id)
				if len(members) == 0 {
					delete(wsRooms, room)
				}
			}
		}
		delete(wsConns, id)
	}
	return ctx.WriteOK()
}

func cmdWSSEND(ctx *Context) error {
	if ctx.ArgCount() < 2 {
		return ctx.WriteError(ErrWrongArgCount)
	}
	id := ctx.ArgString(0)
	wsMu.RLock()
	_, exists := wsConns[id]
	wsMu.RUnlock()
	if !exists {
		return ctx.WriteError(fmt.Errorf("ERR connection not found"))
	}
	return ctx.WriteOK()
}

func cmdWSBROADCAST(ctx *Context) error {
	if ctx.ArgCount() < 2 {
		return ctx.WriteError(ErrWrongArgCount)
	}
	room := ctx.ArgString(0)
	wsMu.RLock()
	defer wsMu.RUnlock()
	members, exists := wsRooms[room]
	if !exists {
		return ctx.WriteInteger(0)
	}
	return ctx.WriteInteger(int64(len(members)))
}

func cmdWSLIST(ctx *Context) error {
	wsMu.RLock()
	defer wsMu.RUnlock()
	results := make([]*resp.Value, 0)
	for id := range wsConns {
		results = append(results, resp.BulkString(id))
	}
	return ctx.WriteArray(results)
}

func cmdWSROOMS(ctx *Context) error {
	wsMu.RLock()
	defer wsMu.RUnlock()
	results := make([]*resp.Value, 0)
	for room := range wsRooms {
		results = append(results, resp.BulkString(room))
	}
	return ctx.WriteArray(results)
}

func cmdWSJOIN(ctx *Context) error {
	if ctx.ArgCount() < 2 {
		return ctx.WriteError(ErrWrongArgCount)
	}
	id := ctx.ArgString(0)
	room := ctx.ArgString(1)
	wsMu.Lock()
	defer wsMu.Unlock()
	conn, exists := wsConns[id]
	if !exists {
		return ctx.WriteError(fmt.Errorf("ERR connection not found"))
	}
	conn.Rooms = append(conn.Rooms, room)
	if _, ok := wsRooms[room]; !ok {
		wsRooms[room] = make(map[string]bool)
	}
	wsRooms[room][id] = true
	return ctx.WriteOK()
}

func cmdWSLEAVE(ctx *Context) error {
	if ctx.ArgCount() < 2 {
		return ctx.WriteError(ErrWrongArgCount)
	}
	id := ctx.ArgString(0)
	room := ctx.ArgString(1)
	wsMu.Lock()
	defer wsMu.Unlock()
	conn, exists := wsConns[id]
	if !exists {
		return ctx.WriteError(fmt.Errorf("ERR connection not found"))
	}
	newRooms := make([]string, 0)
	for _, r := range conn.Rooms {
		if r != room {
			newRooms = append(newRooms, r)
		}
	}
	conn.Rooms = newRooms
	if members, ok := wsRooms[room]; ok {
		delete(members, id)
		if len(members) == 0 {
			delete(wsRooms, room)
		}
	}
	return ctx.WriteOK()
}

var (
	leaders   = make(map[string]*LeaderElection)
	leadersMu sync.RWMutex
)

type LeaderElection struct {
	Name      string
	Leader    string
	Term      int64
	ExpiresAt int64
	History   []string
}

func cmdLEADERELECT(ctx *Context) error {
	if ctx.ArgCount() < 2 {
		return ctx.WriteError(ErrWrongArgCount)
	}
	name := ctx.ArgString(0)
	candidate := ctx.ArgString(1)
	ttl := int64(10000)
	if ctx.ArgCount() >= 3 {
		ttl = parseInt64(ctx.ArgString(2))
	}
	leadersMu.Lock()
	defer leadersMu.Unlock()
	election, exists := leaders[name]
	if !exists {
		election = &LeaderElection{Name: name, Leader: "", Term: 0, History: make([]string, 0)}
		leaders[name] = election
	}
	now := time.Now().UnixMilli()
	if election.Leader == "" || now > election.ExpiresAt {
		election.Term++
		election.Leader = candidate
		election.ExpiresAt = now + ttl
		election.History = append(election.History, candidate)
		if len(election.History) > 100 {
			election.History = election.History[1:]
		}
		return ctx.WriteArray([]*resp.Value{
			resp.BulkString("elected"), resp.IntegerValue(1),
			resp.BulkString("term"), resp.IntegerValue(election.Term),
		})
	}
	return ctx.WriteArray([]*resp.Value{
		resp.BulkString("elected"), resp.IntegerValue(0),
		resp.BulkString("current_leader"), resp.BulkString(election.Leader),
	})
}

func cmdLEADERRENEW(ctx *Context) error {
	if ctx.ArgCount() < 2 {
		return ctx.WriteError(ErrWrongArgCount)
	}
	name := ctx.ArgString(0)
	candidate := ctx.ArgString(1)
	ttl := int64(10000)
	if ctx.ArgCount() >= 3 {
		ttl = parseInt64(ctx.ArgString(2))
	}
	leadersMu.Lock()
	defer leadersMu.Unlock()
	election, exists := leaders[name]
	if !exists {
		return ctx.WriteError(fmt.Errorf("ERR election not found"))
	}
	if election.Leader == candidate {
		election.ExpiresAt = time.Now().UnixMilli() + ttl
		return ctx.WriteInteger(1)
	}
	return ctx.WriteInteger(0)
}

func cmdLEADERRESIGN(ctx *Context) error {
	if ctx.ArgCount() < 2 {
		return ctx.WriteError(ErrWrongArgCount)
	}
	name := ctx.ArgString(0)
	candidate := ctx.ArgString(1)
	leadersMu.Lock()
	defer leadersMu.Unlock()
	election, exists := leaders[name]
	if !exists {
		return ctx.WriteError(fmt.Errorf("ERR election not found"))
	}
	if election.Leader == candidate {
		election.Leader = ""
		election.ExpiresAt = 0
		return ctx.WriteInteger(1)
	}
	return ctx.WriteInteger(0)
}

func cmdLEADERCURRENT(ctx *Context) error {
	if ctx.ArgCount() < 1 {
		return ctx.WriteError(ErrWrongArgCount)
	}
	name := ctx.ArgString(0)
	leadersMu.RLock()
	election, exists := leaders[name]
	leadersMu.RUnlock()
	if !exists {
		return ctx.WriteNull()
	}
	return ctx.WriteArray([]*resp.Value{
		resp.BulkString("leader"), resp.BulkString(election.Leader),
		resp.BulkString("term"), resp.IntegerValue(election.Term),
		resp.BulkString("expires_at"), resp.IntegerValue(election.ExpiresAt),
	})
}

func cmdLEADERHISTORY(ctx *Context) error {
	if ctx.ArgCount() < 1 {
		return ctx.WriteError(ErrWrongArgCount)
	}
	name := ctx.ArgString(0)
	leadersMu.RLock()
	election, exists := leaders[name]
	leadersMu.RUnlock()
	if !exists {
		return ctx.WriteArray([]*resp.Value{})
	}
	results := make([]*resp.Value, 0)
	for _, leader := range election.History {
		results = append(results, resp.BulkString(leader))
	}
	return ctx.WriteArray(results)
}

var (
	memoCache = make(map[string]*MemoEntry)
	memoStats = make(map[string]int64)
	memoMu    sync.RWMutex
)

type MemoEntry struct {
	Key       string
	Value     string
	ExpiresAt int64
	Hits      int64
}

func cmdMEMOCACHE(ctx *Context) error {
	if ctx.ArgCount() < 3 {
		return ctx.WriteError(ErrWrongArgCount)
	}
	key := ctx.ArgString(0)
	value := ctx.ArgString(1)
	ttl := parseInt64(ctx.ArgString(2))
	memoMu.Lock()
	defer memoMu.Unlock()
	memoCache[key] = &MemoEntry{Key: key, Value: value, ExpiresAt: time.Now().UnixMilli() + ttl, Hits: 0}
	memoStats["sets"]++
	return ctx.WriteOK()
}

func cmdMEMOINVALIDATE(ctx *Context) error {
	if ctx.ArgCount() < 1 {
		return ctx.WriteError(ErrWrongArgCount)
	}
	key := ctx.ArgString(0)
	memoMu.Lock()
	defer memoMu.Unlock()
	if _, exists := memoCache[key]; exists {
		delete(memoCache, key)
		return ctx.WriteInteger(1)
	}
	return ctx.WriteInteger(0)
}

func cmdMEMOSTATS(ctx *Context) error {
	memoMu.RLock()
	defer memoMu.RUnlock()
	return ctx.WriteArray([]*resp.Value{
		resp.BulkString("entries"), resp.IntegerValue(int64(len(memoCache))),
		resp.BulkString("sets"), resp.IntegerValue(memoStats["sets"]),
	})
}

func cmdMEMOCLEAR(ctx *Context) error {
	memoMu.Lock()
	defer memoMu.Unlock()
	count := len(memoCache)
	memoCache = make(map[string]*MemoEntry)
	memoStats["sets"] = 0
	return ctx.WriteInteger(int64(count))
}

func cmdMEMOWARM(ctx *Context) error {
	if ctx.ArgCount() < 1 || ctx.ArgCount()%2 != 1 {
		return ctx.WriteError(ErrWrongArgCount)
	}
	ttl := int64(3600000)
	memoMu.Lock()
	defer memoMu.Unlock()
	count := 0
	for i := 0; i+1 < ctx.ArgCount(); i += 2 {
		key := ctx.ArgString(i)
		value := ctx.ArgString(i + 1)
		memoCache[key] = &MemoEntry{Key: key, Value: value, ExpiresAt: time.Now().UnixMilli() + ttl, Hits: 0}
		count++
	}
	memoStats["sets"] += int64(count)
	return ctx.WriteInteger(int64(count))
}

var (
	sentinelsX    = make(map[string]*SentinelWatchX)
	sentinelsXMux sync.RWMutex
)

type SentinelWatchX struct {
	Name      string
	Target    string
	Threshold int64
	Status    string
	Alerts    []*SentinelAlertX
}

type SentinelAlertX struct {
	Timestamp int64
	Value     int64
	Message   string
}

func cmdSENTINELXWATCH(ctx *Context) error {
	if ctx.ArgCount() < 3 {
		return ctx.WriteError(ErrWrongArgCount)
	}
	name := ctx.ArgString(0)
	target := ctx.ArgString(1)
	threshold := parseInt64(ctx.ArgString(2))
	sentinelsXMux.Lock()
	sentinelsX[name] = &SentinelWatchX{Name: name, Target: target, Threshold: threshold, Status: "ok", Alerts: make([]*SentinelAlertX, 0)}
	sentinelsXMux.Unlock()
	return ctx.WriteOK()
}

func cmdSENTINELXUNWATCH(ctx *Context) error {
	if ctx.ArgCount() < 1 {
		return ctx.WriteError(ErrWrongArgCount)
	}
	name := ctx.ArgString(0)
	sentinelsXMux.Lock()
	defer sentinelsXMux.Unlock()
	if _, exists := sentinelsX[name]; !exists {
		return ctx.WriteInteger(0)
	}
	delete(sentinelsX, name)
	return ctx.WriteInteger(1)
}

func cmdSENTINELXSTATUS(ctx *Context) error {
	if ctx.ArgCount() < 1 {
		return ctx.WriteError(ErrWrongArgCount)
	}
	name := ctx.ArgString(0)
	sentinelsXMux.RLock()
	watch, exists := sentinelsX[name]
	sentinelsXMux.RUnlock()
	if !exists {
		return ctx.WriteError(fmt.Errorf("ERR sentinel not found"))
	}
	return ctx.WriteArray([]*resp.Value{
		resp.BulkString("name"), resp.BulkString(watch.Name),
		resp.BulkString("target"), resp.BulkString(watch.Target),
		resp.BulkString("threshold"), resp.IntegerValue(watch.Threshold),
		resp.BulkString("status"), resp.BulkString(watch.Status),
	})
}

func cmdSENTINELXALERTS(ctx *Context) error {
	if ctx.ArgCount() < 1 {
		return ctx.WriteError(ErrWrongArgCount)
	}
	name := ctx.ArgString(0)
	sentinelsXMux.RLock()
	watch, exists := sentinelsX[name]
	sentinelsXMux.RUnlock()
	if !exists {
		return ctx.WriteError(fmt.Errorf("ERR sentinel not found"))
	}
	results := make([]*resp.Value, 0)
	for _, alert := range watch.Alerts {
		results = append(results, resp.ArrayValue([]*resp.Value{
			resp.BulkString("timestamp"), resp.IntegerValue(alert.Timestamp),
			resp.BulkString("value"), resp.IntegerValue(alert.Value),
			resp.BulkString("message"), resp.BulkString(alert.Message),
		}))
	}
	return ctx.WriteArray(results)
}

func cmdSENTINELXCONFIG(ctx *Context) error {
	if ctx.ArgCount() < 3 {
		return ctx.WriteError(ErrWrongArgCount)
	}
	name := ctx.ArgString(0)
	param := ctx.ArgString(1)
	value := ctx.ArgString(2)
	sentinelsXMux.Lock()
	defer sentinelsXMux.Unlock()
	watch, exists := sentinelsX[name]
	if !exists {
		return ctx.WriteError(fmt.Errorf("ERR sentinel not found"))
	}
	switch param {
	case "threshold":
		watch.Threshold = parseInt64(value)
	case "target":
		watch.Target = value
	default:
		return ctx.WriteError(fmt.Errorf("ERR unknown parameter"))
	}
	return ctx.WriteOK()
}

var (
	backupsX    = make(map[string]*BackupX)
	backupsXMux sync.RWMutex
)

type BackupX struct {
	ID        string
	Name      string
	Type      string
	Size      int64
	CreatedAt int64
}

func cmdBACKUPXCREATE(ctx *Context) error {
	if ctx.ArgCount() < 1 {
		return ctx.WriteError(ErrWrongArgCount)
	}
	name := ctx.ArgString(0)
	backupType := "full"
	if ctx.ArgCount() >= 2 {
		backupType = ctx.ArgString(1)
	}
	id := generateUUID()
	backupsXMux.Lock()
	backupsX[id] = &BackupX{ID: id, Name: name, Type: backupType, Size: 0, CreatedAt: time.Now().UnixMilli()}
	backupsXMux.Unlock()
	return ctx.WriteBulkString(id)
}

func cmdBACKUPXRESTORE(ctx *Context) error {
	if ctx.ArgCount() < 1 {
		return ctx.WriteError(ErrWrongArgCount)
	}
	id := ctx.ArgString(0)
	backupsXMux.RLock()
	_, exists := backupsX[id]
	backupsXMux.RUnlock()
	if !exists {
		return ctx.WriteError(fmt.Errorf("ERR backup not found"))
	}
	return ctx.WriteOK()
}

func cmdBACKUPXLIST(ctx *Context) error {
	backupsXMux.RLock()
	defer backupsXMux.RUnlock()
	results := make([]*resp.Value, 0)
	for id, backup := range backupsX {
		results = append(results, resp.ArrayValue([]*resp.Value{
			resp.BulkString("id"), resp.BulkString(id),
			resp.BulkString("name"), resp.BulkString(backup.Name),
			resp.BulkString("type"), resp.BulkString(backup.Type),
			resp.BulkString("created_at"), resp.IntegerValue(backup.CreatedAt),
		}))
	}
	return ctx.WriteArray(results)
}

func cmdBACKUPXDELETE(ctx *Context) error {
	if ctx.ArgCount() < 1 {
		return ctx.WriteError(ErrWrongArgCount)
	}
	id := ctx.ArgString(0)
	backupsXMux.Lock()
	defer backupsXMux.Unlock()
	if _, exists := backupsX[id]; !exists {
		return ctx.WriteInteger(0)
	}
	delete(backupsX, id)
	return ctx.WriteInteger(1)
}

var (
	replays   = make(map[string]*Replay)
	replaysMu sync.RWMutex
)

type Replay struct {
	ID       string
	Status   string
	Speed    float64
	Position int64
	Total    int64
}

func cmdREPLAYSTART(ctx *Context) error {
	if ctx.ArgCount() < 1 {
		return ctx.WriteError(ErrWrongArgCount)
	}
	id := ctx.ArgString(0)
	replaysMu.Lock()
	replays[id] = &Replay{ID: id, Status: "running", Speed: 1.0, Position: 0, Total: 100}
	replaysMu.Unlock()
	return ctx.WriteOK()
}

func cmdREPLAYSTOP(ctx *Context) error {
	if ctx.ArgCount() < 1 {
		return ctx.WriteError(ErrWrongArgCount)
	}
	id := ctx.ArgString(0)
	replaysMu.Lock()
	defer replaysMu.Unlock()
	if replay, exists := replays[id]; exists {
		replay.Status = "stopped"
		return ctx.WriteOK()
	}
	return ctx.WriteError(fmt.Errorf("ERR replay not found"))
}

func cmdREPLAYSTATUS(ctx *Context) error {
	if ctx.ArgCount() < 1 {
		return ctx.WriteError(ErrWrongArgCount)
	}
	id := ctx.ArgString(0)
	replaysMu.RLock()
	replay, exists := replays[id]
	replaysMu.RUnlock()
	if !exists {
		return ctx.WriteError(fmt.Errorf("ERR replay not found"))
	}
	return ctx.WriteArray([]*resp.Value{
		resp.BulkString("id"), resp.BulkString(replay.ID),
		resp.BulkString("status"), resp.BulkString(replay.Status),
		resp.BulkString("speed"), resp.BulkString(fmt.Sprintf("%.2f", replay.Speed)),
		resp.BulkString("position"), resp.IntegerValue(replay.Position),
		resp.BulkString("total"), resp.IntegerValue(replay.Total),
	})
}

func cmdREPLAYSPEED(ctx *Context) error {
	if ctx.ArgCount() < 2 {
		return ctx.WriteError(ErrWrongArgCount)
	}
	id := ctx.ArgString(0)
	speed := parseFloatExt([]byte(ctx.ArgString(1)))
	replaysMu.Lock()
	defer replaysMu.Unlock()
	if replay, exists := replays[id]; exists {
		replay.Speed = speed
		return ctx.WriteOK()
	}
	return ctx.WriteError(fmt.Errorf("ERR replay not found"))
}

func cmdREPLAYSEEK(ctx *Context) error {
	if ctx.ArgCount() < 2 {
		return ctx.WriteError(ErrWrongArgCount)
	}
	id := ctx.ArgString(0)
	position := parseInt64(ctx.ArgString(1))
	replaysMu.Lock()
	defer replaysMu.Unlock()
	if replay, exists := replays[id]; exists {
		replay.Position = position
		return ctx.WriteOK()
	}
	return ctx.WriteError(fmt.Errorf("ERR replay not found"))
}

var (
	aggData   = make(map[string][]float64)
	aggDataMu sync.RWMutex
)

func cmdAGGPUSH(ctx *Context) error {
	if ctx.ArgCount() < 2 {
		return ctx.WriteError(ErrWrongArgCount)
	}
	key := ctx.ArgString(0)
	aggDataMu.Lock()
	defer aggDataMu.Unlock()
	for i := 1; i < ctx.ArgCount(); i++ {
		aggData[key] = append(aggData[key], parseFloatExt([]byte(ctx.ArgString(i))))
	}
	return ctx.WriteInteger(int64(len(aggData[key])))
}

func cmdAGGCLEAR(ctx *Context) error {
	if ctx.ArgCount() < 1 {
		return ctx.WriteError(ErrWrongArgCount)
	}
	key := ctx.ArgString(0)
	aggDataMu.Lock()
	defer aggDataMu.Unlock()
	delete(aggData, key)
	return ctx.WriteOK()
}

func cmdAGGSUM(ctx *Context) error {
	if ctx.ArgCount() < 1 {
		return ctx.WriteError(ErrWrongArgCount)
	}
	key := ctx.ArgString(0)
	aggDataMu.RLock()
	data, exists := aggData[key]
	aggDataMu.RUnlock()
	if !exists {
		return ctx.WriteInteger(0)
	}
	var sum float64
	for _, v := range data {
		sum += v
	}
	return ctx.WriteBulkString(fmt.Sprintf("%.2f", sum))
}

func cmdAGGAVG(ctx *Context) error {
	if ctx.ArgCount() < 1 {
		return ctx.WriteError(ErrWrongArgCount)
	}
	key := ctx.ArgString(0)
	aggDataMu.RLock()
	data, exists := aggData[key]
	aggDataMu.RUnlock()
	if !exists || len(data) == 0 {
		return ctx.WriteBulkString("0")
	}
	var sum float64
	for _, v := range data {
		sum += v
	}
	return ctx.WriteBulkString(fmt.Sprintf("%.2f", sum/float64(len(data))))
}

func cmdAGGMIN(ctx *Context) error {
	if ctx.ArgCount() < 1 {
		return ctx.WriteError(ErrWrongArgCount)
	}
	key := ctx.ArgString(0)
	aggDataMu.RLock()
	data, exists := aggData[key]
	aggDataMu.RUnlock()
	if !exists || len(data) == 0 {
		return ctx.WriteBulkString("0")
	}
	min := data[0]
	for _, v := range data {
		if v < min {
			min = v
		}
	}
	return ctx.WriteBulkString(fmt.Sprintf("%.2f", min))
}

func cmdAGGMAX(ctx *Context) error {
	if ctx.ArgCount() < 1 {
		return ctx.WriteError(ErrWrongArgCount)
	}
	key := ctx.ArgString(0)
	aggDataMu.RLock()
	data, exists := aggData[key]
	aggDataMu.RUnlock()
	if !exists || len(data) == 0 {
		return ctx.WriteBulkString("0")
	}
	max := data[0]
	for _, v := range data {
		if v > max {
			max = v
		}
	}
	return ctx.WriteBulkString(fmt.Sprintf("%.2f", max))
}

func cmdAGGCOUNT(ctx *Context) error {
	if ctx.ArgCount() < 1 {
		return ctx.WriteError(ErrWrongArgCount)
	}
	key := ctx.ArgString(0)
	aggDataMu.RLock()
	data, exists := aggData[key]
	aggDataMu.RUnlock()
	if !exists {
		return ctx.WriteInteger(0)
	}
	return ctx.WriteInteger(int64(len(data)))
}
