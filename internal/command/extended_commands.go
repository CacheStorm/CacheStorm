package command

import (
	"errors"
	"fmt"
	"math"
	"sort"
	"strconv"
	"strings"
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

	router.Register(&CommandDef{Name: "LEADER.ELECT", Handler: cmdLEADERELECT})
	router.Register(&CommandDef{Name: "LEADER.RENEW", Handler: cmdLEADERRENEW})
	router.Register(&CommandDef{Name: "LEADER.RESIGN", Handler: cmdLEADERRESIGN})
	router.Register(&CommandDef{Name: "LEADER.CURRENT", Handler: cmdLEADERCURRENT})
	router.Register(&CommandDef{Name: "LEADER.HISTORY", Handler: cmdLEADERHISTORY})

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
	msgQueuesMu sync.Mutex
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
	Retries   int
	CreatedAt int64
	AckAt     int64
}

// reclaimExpired treats every delivery whose ack deadline has passed as a
// NACK: the message is redelivered, or dead-lettered once it is out of
// retries. Callers hold msgQueuesMu.
func (q *MessageQueue) reclaimExpired(now int64) {
	var expired []*QueuedMessage
	for _, msg := range q.AckWait {
		if msg.AckAt <= now {
			expired = append(expired, msg)
		}
	}
	if len(expired) == 0 {
		return
	}
	sort.Slice(expired, func(i, j int) bool {
		if expired[i].AckAt != expired[j].AckAt {
			return expired[i].AckAt < expired[j].AckAt
		}
		return expired[i].ID < expired[j].ID
	})
	for _, msg := range expired {
		q.retry(msg)
	}
}

// retry takes an unacknowledged message back and reports whether it was
// dead-lettered rather than queued for redelivery.
func (q *MessageQueue) retry(msg *QueuedMessage) bool {
	delete(q.AckWait, msg.ID)
	msg.AckAt = 0
	msg.Retries++
	if msg.Retries >= q.MaxRetries {
		q.DeadLetters = append(q.DeadLetters, msg)
		return true
	}
	q.Messages = append(q.Messages, msg)
	return false
}

// lookupMsgQueue returns the named queue with its expired deliveries already
// reclaimed. Callers hold msgQueuesMu.
func lookupMsgQueue(name string) (*MessageQueue, bool) {
	queue, exists := msgQueues[name]
	if exists {
		queue.reclaimExpired(time.Now().UnixMilli())
	}
	return queue, exists
}

var errMsgQueueNotFound = errors.New("ERR queue not found")

func cmdMSGQUEUECREATE(ctx *Context) error {
	if ctx.ArgCount() < 1 || ctx.ArgCount() > 2 {
		return ctx.WriteError(ErrWrongArgCount)
	}
	name := ctx.ArgString(0)
	maxRetries := int64(3)
	if ctx.ArgCount() == 2 {
		v, err := strconv.ParseInt(ctx.ArgString(1), 10, 32)
		if err != nil || v < 1 {
			return ctx.WriteError(fmt.Errorf("ERR max retries must be a positive integer"))
		}
		maxRetries = v
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
	if ctx.ArgCount() != 2 {
		return ctx.WriteError(ErrWrongArgCount)
	}
	name := ctx.ArgString(0)
	body := ctx.ArgString(1)
	msgQueuesMu.Lock()
	queue, exists := msgQueues[name]
	if !exists {
		msgQueuesMu.Unlock()
		return ctx.WriteError(errMsgQueueNotFound)
	}
	msg := &QueuedMessage{
		ID:        generateUUID(),
		Body:      body,
		CreatedAt: time.Now().UnixMilli(),
	}
	queue.Messages = append(queue.Messages, msg)
	msgQueuesMu.Unlock()
	return ctx.WriteBulkString(msg.ID)
}

func cmdMSGQUEUECONSUME(ctx *Context) error {
	if ctx.ArgCount() < 1 || ctx.ArgCount() > 2 {
		return ctx.WriteError(ErrWrongArgCount)
	}
	name := ctx.ArgString(0)
	timeout := int64(5000)
	if ctx.ArgCount() == 2 {
		v, err := strconv.ParseInt(ctx.ArgString(1), 10, 64)
		if err != nil || v < 1 {
			return ctx.WriteError(fmt.Errorf("ERR visibility timeout must be a positive integer"))
		}
		timeout = v
	}
	msgQueuesMu.Lock()
	queue, exists := lookupMsgQueue(name)
	if !exists {
		msgQueuesMu.Unlock()
		return ctx.WriteError(errMsgQueueNotFound)
	}
	if len(queue.Messages) == 0 {
		msgQueuesMu.Unlock()
		return ctx.WriteNull()
	}
	msg := queue.Messages[0]
	queue.Messages[0] = nil
	queue.Messages = queue.Messages[1:]
	now := time.Now().UnixMilli()
	if timeout > math.MaxInt64-now {
		msg.AckAt = math.MaxInt64
	} else {
		msg.AckAt = now + timeout
	}
	queue.AckWait[msg.ID] = msg
	reply := []*resp.Value{
		resp.BulkString("id"), resp.BulkString(msg.ID),
		resp.BulkString("body"), resp.BulkString(msg.Body),
		resp.BulkString("retries"), resp.IntegerValue(int64(msg.Retries)),
	}
	msgQueuesMu.Unlock()
	return ctx.WriteArray(reply)
}

func cmdMSGQUEUEACK(ctx *Context) error {
	if ctx.ArgCount() != 2 {
		return ctx.WriteError(ErrWrongArgCount)
	}
	name := ctx.ArgString(0)
	msgID := ctx.ArgString(1)
	msgQueuesMu.Lock()
	queue, exists := lookupMsgQueue(name)
	if !exists {
		msgQueuesMu.Unlock()
		return ctx.WriteError(errMsgQueueNotFound)
	}
	_, ok := queue.AckWait[msgID]
	delete(queue.AckWait, msgID)
	msgQueuesMu.Unlock()
	if ok {
		return ctx.WriteInteger(1)
	}
	return ctx.WriteInteger(0)
}

func cmdMSGQUEUENACK(ctx *Context) error {
	if ctx.ArgCount() != 2 {
		return ctx.WriteError(ErrWrongArgCount)
	}
	name := ctx.ArgString(0)
	msgID := ctx.ArgString(1)
	msgQueuesMu.Lock()
	queue, exists := lookupMsgQueue(name)
	if !exists {
		msgQueuesMu.Unlock()
		return ctx.WriteError(errMsgQueueNotFound)
	}
	reply := int64(0)
	if msg, ok := queue.AckWait[msgID]; ok {
		reply = 1
		if queue.retry(msg) {
			reply = 2
		}
	}
	msgQueuesMu.Unlock()
	return ctx.WriteInteger(reply)
}

func cmdMSGQUEUEDEADLETTER(ctx *Context) error {
	if ctx.ArgCount() != 1 {
		return ctx.WriteError(ErrWrongArgCount)
	}
	name := ctx.ArgString(0)
	msgQueuesMu.Lock()
	queue, exists := lookupMsgQueue(name)
	if !exists {
		msgQueuesMu.Unlock()
		return ctx.WriteError(errMsgQueueNotFound)
	}
	results := make([]*resp.Value, 0, len(queue.DeadLetters))
	for _, msg := range queue.DeadLetters {
		results = append(results, resp.ArrayValue([]*resp.Value{
			resp.BulkString("id"), resp.BulkString(msg.ID),
			resp.BulkString("body"), resp.BulkString(msg.Body),
			resp.BulkString("retries"), resp.IntegerValue(int64(msg.Retries)),
		}))
	}
	msgQueuesMu.Unlock()
	return ctx.WriteArray(results)
}

func cmdMSGQUEUEREQUEUE(ctx *Context) error {
	if ctx.ArgCount() != 2 {
		return ctx.WriteError(ErrWrongArgCount)
	}
	name := ctx.ArgString(0)
	msgID := ctx.ArgString(1)
	msgQueuesMu.Lock()
	queue, exists := lookupMsgQueue(name)
	if !exists {
		msgQueuesMu.Unlock()
		return ctx.WriteError(errMsgQueueNotFound)
	}
	reply := int64(0)
	for i, msg := range queue.DeadLetters {
		if msg.ID == msgID {
			queue.DeadLetters = append(queue.DeadLetters[:i], queue.DeadLetters[i+1:]...)
			msg.Retries = 0
			queue.Messages = append(queue.Messages, msg)
			reply = 1
			break
		}
	}
	msgQueuesMu.Unlock()
	return ctx.WriteInteger(reply)
}

func cmdMSGQUEUEPURGE(ctx *Context) error {
	if ctx.ArgCount() != 1 {
		return ctx.WriteError(ErrWrongArgCount)
	}
	name := ctx.ArgString(0)
	msgQueuesMu.Lock()
	queue, exists := lookupMsgQueue(name)
	if !exists {
		msgQueuesMu.Unlock()
		return ctx.WriteError(errMsgQueueNotFound)
	}
	count := len(queue.Messages)
	queue.Messages = make([]*QueuedMessage, 0)
	msgQueuesMu.Unlock()
	return ctx.WriteInteger(int64(count))
}

func cmdMSGQUEUESTATS(ctx *Context) error {
	if ctx.ArgCount() != 1 {
		return ctx.WriteError(ErrWrongArgCount)
	}
	name := ctx.ArgString(0)
	msgQueuesMu.Lock()
	queue, exists := lookupMsgQueue(name)
	if !exists {
		msgQueuesMu.Unlock()
		return ctx.WriteError(errMsgQueueNotFound)
	}
	reply := []*resp.Value{
		resp.BulkString("name"), resp.BulkString(queue.Name),
		resp.BulkString("pending"), resp.IntegerValue(int64(len(queue.Messages))),
		resp.BulkString("processing"), resp.IntegerValue(int64(len(queue.AckWait))),
		resp.BulkString("dead_letters"), resp.IntegerValue(int64(len(queue.DeadLetters))),
		resp.BulkString("max_retries"), resp.IntegerValue(int64(queue.MaxRetries)),
	}
	msgQueuesMu.Unlock()
	return ctx.WriteArray(reply)
}

func cmdMSGQUEUEDELETE(ctx *Context) error {
	if ctx.ArgCount() != 1 {
		return ctx.WriteError(ErrWrongArgCount)
	}
	name := ctx.ArgString(0)
	msgQueuesMu.Lock()
	_, exists := msgQueues[name]
	delete(msgQueues, name)
	msgQueuesMu.Unlock()
	if !exists {
		return ctx.WriteInteger(0)
	}
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
	instances, exists := services[name]
	if !exists {
		reply := []*resp.Value{}
		servicesMu.RUnlock()
		return ctx.WriteArray(reply)
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
	reply := results
	servicesMu.RUnlock()
	return ctx.WriteArray(reply)
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
	results := make([]*resp.Value, 0)
	for name := range services {
		results = append(results, resp.BulkString(name))
	}
	reply := results
	servicesMu.RUnlock()
	return ctx.WriteArray(reply)
}

func cmdSERVICEHEALTHY(ctx *Context) error {
	if ctx.ArgCount() < 1 {
		return ctx.WriteError(ErrWrongArgCount)
	}
	name := ctx.ArgString(0)
	servicesMu.RLock()
	instances, exists := services[name]
	if !exists {
		reply := []*resp.Value{}
		servicesMu.RUnlock()
		return ctx.WriteArray(reply)
	}
	results := make([]*resp.Value, 0)
	for _, inst := range instances {
		if time.Now().UnixMilli()-inst.LastSeen < 30000 {
			results = append(results, resp.BulkString(inst.ID))
		}
	}
	reply := results
	servicesMu.RUnlock()
	return ctx.WriteArray(reply)
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

type HealthCheckResultX struct {
	Timestamp int64
	Status    string
	Latency   int64
	Message   string
}

type CronJobRun struct {
	Timestamp int64
	Status    string
	Output    string
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
	normalize := ctx.ArgCount() >= 3 && ctx.ArgString(2) == "NORMALIZE"

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
	// Same guard as cmdVECTORSEARCH: dim comes straight from the client and is
	// used as a make length, so a negative value panics the handler and the
	// per-connection recover drops the client with no reply.
	if dim < 0 {
		return ctx.WriteError(ErrInvalidArg)
	}
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
	if dim != store.Dim || ctx.ArgCount()-3 != dim {
		return ctx.WriteError(fmt.Errorf("ERR vector dimension mismatch: expected %d", store.Dim))
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
	if !exists {
		vectorStoresMu.RUnlock()
		return ctx.WriteError(fmt.Errorf("ERR vector store not found"))
	}
	vec, ok := store.Vectors[id]
	if !ok {
		vectorStoresMu.RUnlock()
		return ctx.WriteNull()
	}
	results := make([]*resp.Value, len(vec.Data))
	for i, v := range vec.Data {
		results[i] = resp.BulkString(fmt.Sprintf("%.6f", v))
	}
	vectorStoresMu.RUnlock()
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
	// make rejects a negative length, and dim is taken straight from the client
	// and used as an allocation size with nothing bounding it. Validate before
	// the make: the negative case used to panic the handler, which the
	// per-connection recover turned into a dropped connection with no reply.
	if dim < 0 {
		return ctx.WriteError(ErrInvalidArg)
	}
	query := make([]float64, dim)
	for i := 0; i < dim && i+3 < ctx.ArgCount(); i++ {
		query[i] = parseFloatExt([]byte(ctx.ArgString(3 + i)))
	}
	type scoreResult struct {
		ID    string
		Score float64
	}
	vectorStoresMu.RLock()
	store, exists := vectorStores[name]
	if !exists {
		vectorStoresMu.RUnlock()
		return ctx.WriteError(fmt.Errorf("ERR vector store not found"))
	}
	results := make([]scoreResult, 0, len(store.Vectors))
	for id, vec := range store.Vectors {
		score := cosineSimilarity(query, vec.Data)
		results = append(results, scoreResult{ID: id, Score: score})
	}
	vectorStoresMu.RUnlock()
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
	Name string
	Docs map[string]map[string]string
	// Indexes maps an indexed field to value -> set of document ids. It is
	// kept in step with Docs by every write, so a lookup through it always
	// agrees with a scan.
	Indexes map[string]map[string]map[string]struct{}
}

func newDocStore(name string) *DocStore {
	return &DocStore{
		Name:    name,
		Docs:    make(map[string]map[string]string),
		Indexes: make(map[string]map[string]map[string]struct{}),
	}
}

func (s *DocStore) indexDoc(id string, doc map[string]string) {
	for field, idx := range s.Indexes {
		if value, ok := doc[field]; ok {
			ids := idx[value]
			if ids == nil {
				ids = make(map[string]struct{})
				idx[value] = ids
			}
			ids[id] = struct{}{}
		}
	}
}

func (s *DocStore) unindexDoc(id string, doc map[string]string) {
	for field, idx := range s.Indexes {
		value, ok := doc[field]
		if !ok {
			continue
		}
		if ids := idx[value]; ids != nil {
			delete(ids, id)
			if len(ids) == 0 {
				delete(idx, value)
			}
		}
	}
}

// matching returns the ids of the documents whose field equals value, in id
// order, through the field's index when there is one.
func (s *DocStore) matching(field, value string) []string {
	var ids []string
	if idx, ok := s.Indexes[field]; ok {
		for id := range idx[value] {
			ids = append(ids, id)
		}
	} else {
		for id, doc := range s.Docs {
			if v, ok := doc[field]; ok && v == value {
				ids = append(ids, id)
			}
		}
	}
	sort.Strings(ids)
	return ids
}

func docReply(id string, doc map[string]string) []*resp.Value {
	fields := make([]string, 0, len(doc))
	for k := range doc {
		fields = append(fields, k)
	}
	sort.Strings(fields)
	out := make([]*resp.Value, 0, 2+2*len(fields))
	out = append(out, resp.BulkString("_id"), resp.BulkString(id))
	for _, k := range fields {
		out = append(out, resp.BulkString(k), resp.BulkString(doc[k]))
	}
	return out
}

var errDocStoreNotFound = errors.New("ERR doc store not found")

func cmdDOCINSERT(ctx *Context) error {
	// name, id, then field/value PAIRS. Without this, `DOC.INSERT s d` stored an
	// empty phantom document and returned its id, and `DOC.INSERT s d f1 v1 f2`
	// silently dropped f2 -- both while reporting success.
	if ctx.ArgCount() < 4 || (ctx.ArgCount()-2)%2 != 0 {
		return ctx.WriteError(ErrWrongArgCount)
	}
	name := ctx.ArgString(0)
	id := ctx.ArgString(1)
	doc := make(map[string]string)
	for i := 2; i+1 < ctx.ArgCount(); i += 2 {
		doc[ctx.ArgString(i)] = ctx.ArgString(i + 1)
	}
	docStoresMu.Lock()
	store, exists := docStores[name]
	if !exists {
		store = newDocStore(name)
		docStores[name] = store
	}
	if old, ok := store.Docs[id]; ok {
		store.unindexDoc(id, old)
	}
	store.Docs[id] = doc
	store.indexDoc(id, doc)
	docStoresMu.Unlock()
	return ctx.WriteBulkString(id)
}

func cmdDOCFIND(ctx *Context) error {
	if ctx.ArgCount() != 1 && ctx.ArgCount() != 3 {
		return ctx.WriteError(ErrWrongArgCount)
	}
	name := ctx.ArgString(0)
	docStoresMu.RLock()
	store, exists := docStores[name]
	if !exists {
		docStoresMu.RUnlock()
		return ctx.WriteArray([]*resp.Value{})
	}
	var ids []string
	if ctx.ArgCount() == 3 {
		ids = store.matching(ctx.ArgString(1), ctx.ArgString(2))
	} else {
		ids = make([]string, 0, len(store.Docs))
		for id := range store.Docs {
			ids = append(ids, id)
		}
		sort.Strings(ids)
	}
	results := make([]*resp.Value, 0, len(ids))
	for _, id := range ids {
		results = append(results, resp.ArrayValue(docReply(id, store.Docs[id])))
	}
	docStoresMu.RUnlock()
	return ctx.WriteArray(results)
}

func cmdDOCFINDONE(ctx *Context) error {
	if ctx.ArgCount() != 3 {
		return ctx.WriteError(ErrWrongArgCount)
	}
	name := ctx.ArgString(0)
	docStoresMu.RLock()
	store, exists := docStores[name]
	if !exists {
		docStoresMu.RUnlock()
		return ctx.WriteNull()
	}
	ids := store.matching(ctx.ArgString(1), ctx.ArgString(2))
	if len(ids) == 0 {
		docStoresMu.RUnlock()
		return ctx.WriteNull()
	}
	reply := docReply(ids[0], store.Docs[ids[0]])
	docStoresMu.RUnlock()
	return ctx.WriteArray(reply)
}

func cmdDOCUPDATE(ctx *Context) error {
	// name, id, then optional field/value PAIRS. The pair count must be even:
	// `DOC.UPDATE s id dangling` applied nothing yet still replied +OK, so the
	// caller was told an update happened when none did.
	if ctx.ArgCount() < 2 || (ctx.ArgCount()-2)%2 != 0 {
		return ctx.WriteError(ErrWrongArgCount)
	}
	name := ctx.ArgString(0)
	id := ctx.ArgString(1)
	docStoresMu.Lock()
	store, exists := docStores[name]
	if !exists {
		docStoresMu.Unlock()
		return ctx.WriteError(errDocStoreNotFound)
	}
	doc, ok := store.Docs[id]
	if !ok {
		docStoresMu.Unlock()
		return ctx.WriteError(fmt.Errorf("ERR document not found"))
	}
	store.unindexDoc(id, doc)
	for i := 2; i+1 < ctx.ArgCount(); i += 2 {
		doc[ctx.ArgString(i)] = ctx.ArgString(i + 1)
	}
	store.indexDoc(id, doc)
	docStoresMu.Unlock()
	return ctx.WriteOK()
}

func cmdDOCDELETE(ctx *Context) error {
	if ctx.ArgCount() != 2 {
		return ctx.WriteError(ErrWrongArgCount)
	}
	name := ctx.ArgString(0)
	id := ctx.ArgString(1)
	docStoresMu.Lock()
	removed := false
	if store, exists := docStores[name]; exists {
		if doc, ok := store.Docs[id]; ok {
			store.unindexDoc(id, doc)
			delete(store.Docs, id)
			removed = true
		}
	}
	docStoresMu.Unlock()
	if removed {
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
	if !exists {
		docStoresMu.RUnlock()
		return ctx.WriteInteger(0)
	}
	count := int64(len(store.Docs))
	docStoresMu.RUnlock()
	return ctx.WriteInteger(count)
}

func docDOCDISTINCT(ctx *Context) error {
	if ctx.ArgCount() != 2 {
		return ctx.WriteError(ErrWrongArgCount)
	}
	name := ctx.ArgString(0)
	field := ctx.ArgString(1)
	docStoresMu.RLock()
	store, exists := docStores[name]
	if !exists {
		docStoresMu.RUnlock()
		return ctx.WriteArray([]*resp.Value{})
	}
	seen := make(map[string]struct{})
	for _, doc := range store.Docs {
		if value, ok := doc[field]; ok {
			seen[value] = struct{}{}
		}
	}
	docStoresMu.RUnlock()
	values := make([]string, 0, len(seen))
	for value := range seen {
		values = append(values, value)
	}
	sort.Strings(values)
	results := make([]*resp.Value, len(values))
	for i, value := range values {
		results[i] = resp.BulkString(value)
	}
	return ctx.WriteArray(results)
}

func cmdDOCAGGREGATE(ctx *Context) error {
	if ctx.ArgCount() != 2 {
		return ctx.WriteError(ErrWrongArgCount)
	}
	name := ctx.ArgString(0)
	aggType := strings.ToLower(ctx.ArgString(1))
	if aggType != "count" && aggType != "fields" {
		return ctx.WriteError(fmt.Errorf("ERR unknown aggregation type"))
	}
	docStoresMu.RLock()
	store, exists := docStores[name]
	if !exists {
		docStoresMu.RUnlock()
		if aggType == "count" {
			return ctx.WriteInteger(0)
		}
		return ctx.WriteArray([]*resp.Value{})
	}
	if aggType == "count" {
		count := int64(len(store.Docs))
		docStoresMu.RUnlock()
		return ctx.WriteInteger(count)
	}
	fields := make(map[string]struct{})
	for _, doc := range store.Docs {
		for field := range doc {
			fields[field] = struct{}{}
		}
	}
	docStoresMu.RUnlock()
	names := make([]string, 0, len(fields))
	for field := range fields {
		names = append(names, field)
	}
	sort.Strings(names)
	results := make([]*resp.Value, len(names))
	for i, field := range names {
		results[i] = resp.BulkString(field)
	}
	return ctx.WriteArray(results)
}

func cmdDOCINDEX(ctx *Context) error {
	if ctx.ArgCount() != 2 {
		return ctx.WriteError(ErrWrongArgCount)
	}
	name := ctx.ArgString(0)
	field := ctx.ArgString(1)
	docStoresMu.Lock()
	store, exists := docStores[name]
	if !exists {
		store = newDocStore(name)
		docStores[name] = store
	}
	if _, ok := store.Indexes[field]; !ok {
		idx := make(map[string]map[string]struct{})
		for id, doc := range store.Docs {
			if value, ok := doc[field]; ok {
				if idx[value] == nil {
					idx[value] = make(map[string]struct{})
				}
				idx[value][id] = struct{}{}
			}
		}
		store.Indexes[field] = idx
	}
	docStoresMu.Unlock()
	return ctx.WriteOK()
}

func cmdDOCDROPINDEX(ctx *Context) error {
	if ctx.ArgCount() != 2 {
		return ctx.WriteError(ErrWrongArgCount)
	}
	name := ctx.ArgString(0)
	field := ctx.ArgString(1)
	docStoresMu.Lock()
	store, exists := docStores[name]
	if !exists {
		docStoresMu.Unlock()
		return ctx.WriteError(errDocStoreNotFound)
	}
	_, ok := store.Indexes[field]
	delete(store.Indexes, field)
	docStoresMu.Unlock()
	if ok {
		return ctx.WriteInteger(1)
	}
	return ctx.WriteInteger(0)
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
	if !exists || election.Leader == "" || time.Now().UnixMilli() > election.ExpiresAt {
		leadersMu.RUnlock()
		return ctx.WriteNull()
	}
	reply := []*resp.Value{
		resp.BulkString("leader"), resp.BulkString(election.Leader),
		resp.BulkString("term"), resp.IntegerValue(election.Term),
		resp.BulkString("expires_at"), resp.IntegerValue(election.ExpiresAt),
	}
	leadersMu.RUnlock()
	return ctx.WriteArray(reply)
}

func cmdLEADERHISTORY(ctx *Context) error {
	if ctx.ArgCount() < 1 {
		return ctx.WriteError(ErrWrongArgCount)
	}
	name := ctx.ArgString(0)
	leadersMu.RLock()
	election, exists := leaders[name]
	if !exists {
		reply := []*resp.Value{}
		leadersMu.RUnlock()
		return ctx.WriteArray(reply)
	}
	results := make([]*resp.Value, 0)
	for _, leader := range election.History {
		results = append(results, resp.BulkString(leader))
	}
	reply := results
	leadersMu.RUnlock()
	return ctx.WriteArray(reply)
}

type SentinelAlertX struct {
	Timestamp int64
	Value     int64
	Message   string
}

type Replay struct {
	ID       string
	Status   string
	Speed    float64
	Position int64
	Total    int64
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
