package command

import (
	"fmt"
	"sort"
	"sync"

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

// maxVectorDim bounds a vector store's dimension.
const maxVectorDim = 1 << 16

func cmdVECTORCREATE(ctx *Context) error {
	if ctx.ArgCount() < 2 {
		return ctx.WriteError(ErrWrongArgCount)
	}
	name := ctx.ArgString(0)
	dim := int(parseInt64(ctx.ArgString(1)))
	if dim < 1 || dim > maxVectorDim {
		return ctx.WriteError(fmt.Errorf("ERR dimension must be between 1 and %d", maxVectorDim))
	}
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
	// A vector carries one argument per component, so the argument count
	// bounds the allocation; a dim it does not match is refused before
	// anything is allocated.
	if ctx.ArgCount()-3 != dim {
		return ctx.WriteError(fmt.Errorf("ERR vector dimension mismatch: got %d values for dimension %d", ctx.ArgCount()-3, dim))
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
	// Components past the arguments given are zero; the store's dimension
	// bounds how many are worth allocating.
	if dim > maxVectorDim {
		return ctx.WriteError(fmt.Errorf("ERR dimension must be at most %d", maxVectorDim))
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
