package command

import (
	"fmt"
	"sync"
	"time"

	"github.com/cachestorm/cachestorm/internal/resp"
)

func RegisterActorCommands(router *Router) {
	router.Register(&CommandDef{Name: "ACTOR.CREATE", Handler: cmdACTORCREATE})
	router.Register(&CommandDef{Name: "ACTOR.DELETE", Handler: cmdACTORDELETE})
	router.Register(&CommandDef{Name: "ACTOR.SEND", Handler: cmdACTORSEND})
	router.Register(&CommandDef{Name: "ACTOR.RECV", Handler: cmdACTORRECV})
	router.Register(&CommandDef{Name: "ACTOR.POKE", Handler: cmdACTORPOKE})
	router.Register(&CommandDef{Name: "ACTOR.PEEK", Handler: cmdACTORPEEK})
	router.Register(&CommandDef{Name: "ACTOR.LEN", Handler: cmdACTORLEN})
	router.Register(&CommandDef{Name: "ACTOR.LIST", Handler: cmdACTORLIST})
	router.Register(&CommandDef{Name: "ACTOR.CLEAR", Handler: cmdACTORCLEAR})

	router.Register(&CommandDef{Name: "DAG.CREATE", Handler: cmdDAGCREATE})
	router.Register(&CommandDef{Name: "DAG.ADDNODE", Handler: cmdDAGADDNODE})
	router.Register(&CommandDef{Name: "DAG.ADDEDGE", Handler: cmdDAGADDEDGE})
	router.Register(&CommandDef{Name: "DAG.TOPO", Handler: cmdDAGTOPO})
	router.Register(&CommandDef{Name: "DAG.PARENTS", Handler: cmdDAGPARENTS})
	router.Register(&CommandDef{Name: "DAG.CHILDREN", Handler: cmdDAGCHILDREN})
	router.Register(&CommandDef{Name: "DAG.DELETE", Handler: cmdDAGDELETE})
	router.Register(&CommandDef{Name: "DAG.LIST", Handler: cmdDAGLIST})

	router.Register(&CommandDef{Name: "PARALLEL.EXEC", Handler: cmdPARALLELEXEC})
	router.Register(&CommandDef{Name: "PARALLEL.MAP", Handler: cmdPARALLELMAP})
	router.Register(&CommandDef{Name: "PARALLEL.REDUCE", Handler: cmdPARALLELREDUCE})
	router.Register(&CommandDef{Name: "PARALLEL.FILTER", Handler: cmdPARALLELFILTER})

	router.Register(&CommandDef{Name: "SECRET.SET", Handler: cmdSECRETSET})
	router.Register(&CommandDef{Name: "SECRET.GET", Handler: cmdSECRETGET})
	router.Register(&CommandDef{Name: "SECRET.DELETE", Handler: cmdSECRETDELETE})
	router.Register(&CommandDef{Name: "SECRET.LIST", Handler: cmdSECRETLIST})
	router.Register(&CommandDef{Name: "SECRET.ROTATE", Handler: cmdSECRETROTATE})
	router.Register(&CommandDef{Name: "SECRET.VERSION", Handler: cmdSECRETVERSION})

	router.Register(&CommandDef{Name: "CONFIG.SET", Handler: cmdCONFIGSET})
	router.Register(&CommandDef{Name: "CONFIG.GET", Handler: cmdCONFIGGET})
	router.Register(&CommandDef{Name: "CONFIG.DELETE", Handler: cmdCONFIGDELETE})
	router.Register(&CommandDef{Name: "CONFIG.LIST", Handler: cmdCONFIGLIST})
	router.Register(&CommandDef{Name: "CONFIG.NAMESPACE", Handler: cmdCONFIGNAMESPACE})
	router.Register(&CommandDef{Name: "CONFIG.IMPORT", Handler: cmdCONFIGIMPORT})
	router.Register(&CommandDef{Name: "CONFIG.EXPORT", Handler: cmdCONFIGEXPORT})

	router.Register(&CommandDef{Name: "TRIE.ADD", Handler: cmdTRIEADD})
	router.Register(&CommandDef{Name: "TRIE.SEARCH", Handler: cmdTRIESEARCH})
	router.Register(&CommandDef{Name: "TRIE.PREFIX", Handler: cmdTRIEPREFIX})
	router.Register(&CommandDef{Name: "TRIE.DELETE", Handler: cmdTRIEDELETE})
	router.Register(&CommandDef{Name: "TRIE.AUTOCOMPLETE", Handler: cmdTRIEAUTOCOMPLETE})

	router.Register(&CommandDef{Name: "RING.CREATE", Handler: cmdRINGCREATE})
	router.Register(&CommandDef{Name: "RING.ADD", Handler: cmdRINGADD})
	router.Register(&CommandDef{Name: "RING.GET", Handler: cmdRINGGET})
	router.Register(&CommandDef{Name: "RING.NODES", Handler: cmdRINGNODES})
	router.Register(&CommandDef{Name: "RING.REMOVE", Handler: cmdRINGREMOVE})

	router.Register(&CommandDef{Name: "SEM.ACQUIRE", Handler: cmdSEMACQUIRE})
	router.Register(&CommandDef{Name: "SEM.RELEASE", Handler: cmdSEMRELEASE})
	router.Register(&CommandDef{Name: "SEM.TRYACQUIRE", Handler: cmdSEMTRYACQUIRE})
	router.Register(&CommandDef{Name: "SEM.VALUE", Handler: cmdSEMVALUE})
	router.Register(&CommandDef{Name: "SEM.CREATE", Handler: cmdSEMCREATE})
}

var (
	actors   = make(map[string]*Actor)
	actorsMu sync.RWMutex
)

type Actor struct {
	ID      string
	Mailbox []string
	mu      sync.RWMutex
}

func cmdACTORCREATE(ctx *Context) error {
	if ctx.ArgCount() < 1 {
		return ctx.WriteError(ErrWrongArgCount)
	}

	id := ctx.ArgString(0)

	actorsMu.Lock()
	actors[id] = &Actor{
		ID:      id,
		Mailbox: make([]string, 0),
	}
	actorsMu.Unlock()

	return ctx.WriteOK()
}

func cmdACTORDELETE(ctx *Context) error {
	if ctx.ArgCount() < 1 {
		return ctx.WriteError(ErrWrongArgCount)
	}

	id := ctx.ArgString(0)

	actorsMu.Lock()
	defer actorsMu.Unlock()

	if _, exists := actors[id]; !exists {
		return ctx.WriteInteger(0)
	}

	delete(actors, id)
	return ctx.WriteInteger(1)
}

func cmdACTORSEND(ctx *Context) error {
	if ctx.ArgCount() < 2 {
		return ctx.WriteError(ErrWrongArgCount)
	}

	id := ctx.ArgString(0)
	msg := ctx.ArgString(1)

	actorsMu.RLock()
	actor, exists := actors[id]
	actorsMu.RUnlock()

	if !exists {
		return ctx.WriteError(fmt.Errorf("ERR actor not found"))
	}

	actor.mu.Lock()
	actor.Mailbox = append(actor.Mailbox, msg)
	len := len(actor.Mailbox)
	actor.mu.Unlock()

	return ctx.WriteInteger(int64(len))
}

func cmdACTORRECV(ctx *Context) error {
	if ctx.ArgCount() < 1 {
		return ctx.WriteError(ErrWrongArgCount)
	}

	id := ctx.ArgString(0)

	actorsMu.RLock()
	actor, exists := actors[id]
	actorsMu.RUnlock()

	if !exists {
		return ctx.WriteError(fmt.Errorf("ERR actor not found"))
	}

	actor.mu.Lock()
	defer actor.mu.Unlock()

	if len(actor.Mailbox) == 0 {
		return ctx.WriteNull()
	}

	msg := actor.Mailbox[0]
	actor.Mailbox = actor.Mailbox[1:]

	return ctx.WriteBulkString(msg)
}

func cmdACTORPOKE(ctx *Context) error {
	if ctx.ArgCount() < 1 {
		return ctx.WriteError(ErrWrongArgCount)
	}

	id := ctx.ArgString(0)

	actorsMu.RLock()
	actor, exists := actors[id]
	actorsMu.RUnlock()

	if !exists {
		return ctx.WriteError(fmt.Errorf("ERR actor not found"))
	}

	actor.mu.Lock()
	defer actor.mu.Unlock()

	if len(actor.Mailbox) == 0 {
		return ctx.WriteNull()
	}

	return ctx.WriteBulkString(actor.Mailbox[0])
}

func cmdACTORPEEK(ctx *Context) error {
	return cmdACTORPOKE(ctx)
}

func cmdACTORLEN(ctx *Context) error {
	if ctx.ArgCount() < 1 {
		return ctx.WriteError(ErrWrongArgCount)
	}

	id := ctx.ArgString(0)

	actorsMu.RLock()
	actor, exists := actors[id]
	actorsMu.RUnlock()

	if !exists {
		return ctx.WriteInteger(0)
	}

	actor.mu.RLock()
	len := len(actor.Mailbox)
	actor.mu.RUnlock()

	return ctx.WriteInteger(int64(len))
}

func cmdACTORLIST(ctx *Context) error {
	actorsMu.RLock()
	defer actorsMu.RUnlock()

	results := make([]*resp.Value, 0, len(actors))
	for id := range actors {
		results = append(results, resp.BulkString(id))
	}

	return ctx.WriteArray(results)
}

func cmdACTORCLEAR(ctx *Context) error {
	if ctx.ArgCount() < 1 {
		return ctx.WriteError(ErrWrongArgCount)
	}

	id := ctx.ArgString(0)

	actorsMu.RLock()
	actor, exists := actors[id]
	actorsMu.RUnlock()

	if !exists {
		return ctx.WriteOK()
	}

	actor.mu.Lock()
	actor.Mailbox = make([]string, 0)
	actor.mu.Unlock()

	return ctx.WriteOK()
}

var (
	dags   = make(map[string]*DAG)
	dagsMu sync.RWMutex
)

type DAG struct {
	Name     string
	Nodes    map[string]bool
	Edges    map[string][]string
	InDegree map[string]int
	mu       sync.RWMutex
}

func cmdDAGCREATE(ctx *Context) error {
	if ctx.ArgCount() < 1 {
		return ctx.WriteError(ErrWrongArgCount)
	}

	name := ctx.ArgString(0)

	dagsMu.Lock()
	dags[name] = &DAG{
		Name:     name,
		Nodes:    make(map[string]bool),
		Edges:    make(map[string][]string),
		InDegree: make(map[string]int),
	}
	dagsMu.Unlock()

	return ctx.WriteOK()
}

func cmdDAGADDNODE(ctx *Context) error {
	if ctx.ArgCount() < 2 {
		return ctx.WriteError(ErrWrongArgCount)
	}

	name := ctx.ArgString(0)
	node := ctx.ArgString(1)

	dagsMu.RLock()
	dag, exists := dags[name]
	dagsMu.RUnlock()

	if !exists {
		return ctx.WriteError(fmt.Errorf("ERR DAG not found"))
	}

	dag.mu.Lock()
	defer dag.mu.Unlock()

	if dag.Nodes[node] {
		return ctx.WriteInteger(0)
	}

	dag.Nodes[node] = true
	dag.InDegree[node] = 0

	return ctx.WriteInteger(1)
}

func cmdDAGADDEDGE(ctx *Context) error {
	if ctx.ArgCount() < 3 {
		return ctx.WriteError(ErrWrongArgCount)
	}

	name := ctx.ArgString(0)
	from := ctx.ArgString(1)
	to := ctx.ArgString(2)

	dagsMu.RLock()
	dag, exists := dags[name]
	dagsMu.RUnlock()

	if !exists {
		return ctx.WriteError(fmt.Errorf("ERR DAG not found"))
	}

	dag.mu.Lock()
	defer dag.mu.Unlock()

	if !dag.Nodes[from] || !dag.Nodes[to] {
		return ctx.WriteError(fmt.Errorf("ERR node not found"))
	}

	dag.Edges[from] = append(dag.Edges[from], to)
	dag.InDegree[to]++

	return ctx.WriteOK()
}

func cmdDAGTOPO(ctx *Context) error {
	if ctx.ArgCount() < 1 {
		return ctx.WriteError(ErrWrongArgCount)
	}

	name := ctx.ArgString(0)

	dagsMu.RLock()
	dag, exists := dags[name]
	dagsMu.RUnlock()

	if !exists {
		return ctx.WriteError(fmt.Errorf("ERR DAG not found"))
	}

	dag.mu.RLock()
	defer dag.mu.RUnlock()

	inDegree := make(map[string]int)
	for k, v := range dag.InDegree {
		inDegree[k] = v
	}

	queue := make([]string, 0)
	for node := range dag.Nodes {
		if inDegree[node] == 0 {
			queue = append(queue, node)
		}
	}

	result := make([]string, 0)
	for len(queue) > 0 {
		node := queue[0]
		queue = queue[1:]
		result = append(result, node)

		for _, neighbor := range dag.Edges[node] {
			inDegree[neighbor]--
			if inDegree[neighbor] == 0 {
				queue = append(queue, neighbor)
			}
		}
	}

	results := make([]*resp.Value, len(result))
	for i, node := range result {
		results[i] = resp.BulkString(node)
	}

	return ctx.WriteArray(results)
}

func cmdDAGPARENTS(ctx *Context) error {
	if ctx.ArgCount() < 2 {
		return ctx.WriteError(ErrWrongArgCount)
	}

	name := ctx.ArgString(0)
	node := ctx.ArgString(1)

	dagsMu.RLock()
	dag, exists := dags[name]
	dagsMu.RUnlock()

	if !exists {
		return ctx.WriteError(fmt.Errorf("ERR DAG not found"))
	}

	dag.mu.RLock()
	defer dag.mu.RUnlock()

	parents := make([]string, 0)
	for from, edges := range dag.Edges {
		for _, to := range edges {
			if to == node {
				parents = append(parents, from)
				break
			}
		}
	}

	results := make([]*resp.Value, len(parents))
	for i, p := range parents {
		results[i] = resp.BulkString(p)
	}

	return ctx.WriteArray(results)
}

func cmdDAGCHILDREN(ctx *Context) error {
	if ctx.ArgCount() < 2 {
		return ctx.WriteError(ErrWrongArgCount)
	}

	name := ctx.ArgString(0)
	node := ctx.ArgString(1)

	dagsMu.RLock()
	dag, exists := dags[name]
	dagsMu.RUnlock()

	if !exists {
		return ctx.WriteError(fmt.Errorf("ERR DAG not found"))
	}

	dag.mu.RLock()
	defer dag.mu.RUnlock()

	children := dag.Edges[node]

	results := make([]*resp.Value, len(children))
	for i, c := range children {
		results[i] = resp.BulkString(c)
	}

	return ctx.WriteArray(results)
}

func cmdDAGDELETE(ctx *Context) error {
	if ctx.ArgCount() < 1 {
		return ctx.WriteError(ErrWrongArgCount)
	}

	name := ctx.ArgString(0)

	dagsMu.Lock()
	defer dagsMu.Unlock()

	if _, exists := dags[name]; !exists {
		return ctx.WriteInteger(0)
	}

	delete(dags, name)
	return ctx.WriteInteger(1)
}

func cmdDAGLIST(ctx *Context) error {
	dagsMu.RLock()
	defer dagsMu.RUnlock()

	results := make([]*resp.Value, 0, len(dags))
	for name := range dags {
		results = append(results, resp.BulkString(name))
	}

	return ctx.WriteArray(results)
}

func cmdPARALLELEXEC(ctx *Context) error {
	if ctx.ArgCount() < 1 {
		return ctx.WriteError(ErrWrongArgCount)
	}

	results := make([]*resp.Value, ctx.ArgCount())
	var wg sync.WaitGroup
	var mu sync.Mutex

	for i := 0; i < ctx.ArgCount(); i++ {
		wg.Add(1)
		go func(idx int, cmd string) {
			defer wg.Done()
			mu.Lock()
			results[idx] = resp.BulkString(cmd + ":ok")
			mu.Unlock()
		}(i, ctx.ArgString(i))
	}

	wg.Wait()

	return ctx.WriteArray(results)
}

func cmdPARALLELMAP(ctx *Context) error {
	if ctx.ArgCount() < 2 {
		return ctx.WriteError(ErrWrongArgCount)
	}

	operation := ctx.ArgString(0)

	results := make([]*resp.Value, ctx.ArgCount()-1)
	var wg sync.WaitGroup
	var mu sync.Mutex

	for i := 1; i < ctx.ArgCount(); i++ {
		wg.Add(1)
		go func(idx int, val string) {
			defer wg.Done()
			var result string
			switch operation {
			case "upper":
				result = toUpper(val)
			case "lower":
				result = toLower(val)
			case "reverse":
				result = reverse(val)
			case "double":
				result = val + val
			default:
				result = val
			}
			mu.Lock()
			results[idx] = resp.BulkString(result)
			mu.Unlock()
		}(i-1, ctx.ArgString(i))
	}

	wg.Wait()

	return ctx.WriteArray(results)
}

func cmdPARALLELREDUCE(ctx *Context) error {
	if ctx.ArgCount() < 3 {
		return ctx.WriteError(ErrWrongArgCount)
	}

	operation := ctx.ArgString(0)
	_ = ctx.ArgString(1)

	var result int64
	for i := 2; i < ctx.ArgCount(); i++ {
		val := parseInt64(ctx.ArgString(i))
		switch operation {
		case "sum":
			result += val
		case "max":
			if i == 2 || val > result {
				result = val
			}
		case "min":
			if i == 2 || val < result {
				result = val
			}
		}
	}

	return ctx.WriteInteger(result)
}

func cmdPARALLELFILTER(ctx *Context) error {
	if ctx.ArgCount() < 2 {
		return ctx.WriteError(ErrWrongArgCount)
	}

	condition := ctx.ArgString(0)

	results := make([]*resp.Value, 0)
	var mu sync.Mutex
	var wg sync.WaitGroup

	for i := 1; i < ctx.ArgCount(); i++ {
		wg.Add(1)
		go func(val string) {
			defer wg.Done()
			include := false
			switch condition {
			case "even":
				n := parseInt64(val)
				include = n%2 == 0
			case "odd":
				n := parseInt64(val)
				include = n%2 != 0
			case "positive":
				n := parseInt64(val)
				include = n > 0
			case "negative":
				n := parseInt64(val)
				include = n < 0
			}
			if include {
				mu.Lock()
				results = append(results, resp.BulkString(val))
				mu.Unlock()
			}
		}(ctx.ArgString(i))
	}

	wg.Wait()

	return ctx.WriteArray(results)
}

func toUpper(s string) string {
	result := ""
	for _, c := range s {
		if c >= 'a' && c <= 'z' {
			result += string(c - 32)
		} else {
			result += string(c)
		}
	}
	return result
}

func toLower(s string) string {
	result := ""
	for _, c := range s {
		if c >= 'A' && c <= 'Z' {
			result += string(c + 32)
		} else {
			result += string(c)
		}
	}
	return result
}

func reverse(s string) string {
	runes := []rune(s)
	for i, j := 0, len(runes)-1; i < j; i, j = i+1, j-1 {
		runes[i], runes[j] = runes[j], runes[i]
	}
	return string(runes)
}

var (
	secrets   = make(map[string]*Secret)
	secretsMu sync.RWMutex
)

type Secret struct {
	Value     string
	Version   int64
	CreatedAt int64
	UpdatedAt int64
}

func cmdSECRETSET(ctx *Context) error {
	if ctx.ArgCount() < 2 {
		return ctx.WriteError(ErrWrongArgCount)
	}

	key := ctx.ArgString(0)
	value := ctx.ArgString(1)

	secretsMu.Lock()
	defer secretsMu.Unlock()

	now := time.Now().UnixMilli()
	if secret, exists := secrets[key]; exists {
		secret.Value = value
		secret.Version++
		secret.UpdatedAt = now
	} else {
		secrets[key] = &Secret{
			Value:     value,
			Version:   1,
			CreatedAt: now,
			UpdatedAt: now,
		}
	}

	return ctx.WriteOK()
}

func cmdSECRETGET(ctx *Context) error {
	if ctx.ArgCount() < 1 {
		return ctx.WriteError(ErrWrongArgCount)
	}

	key := ctx.ArgString(0)

	secretsMu.RLock()
	defer secretsMu.RUnlock()

	secret, exists := secrets[key]
	if !exists {
		return ctx.WriteNull()
	}

	return ctx.WriteBulkString(secret.Value)
}

func cmdSECRETDELETE(ctx *Context) error {
	if ctx.ArgCount() < 1 {
		return ctx.WriteError(ErrWrongArgCount)
	}

	key := ctx.ArgString(0)

	secretsMu.Lock()
	defer secretsMu.Unlock()

	if _, exists := secrets[key]; !exists {
		return ctx.WriteInteger(0)
	}

	delete(secrets, key)
	return ctx.WriteInteger(1)
}

func cmdSECRETLIST(ctx *Context) error {
	secretsMu.RLock()
	defer secretsMu.RUnlock()

	keys := make([]string, 0, len(secrets))
	for k := range secrets {
		keys = append(keys, k)
	}

	results := make([]*resp.Value, len(keys))
	for i, k := range keys {
		results[i] = resp.BulkString(k)
	}

	return ctx.WriteArray(results)
}

func cmdSECRETROTATE(ctx *Context) error {
	if ctx.ArgCount() < 2 {
		return ctx.WriteError(ErrWrongArgCount)
	}

	key := ctx.ArgString(0)
	newValue := ctx.ArgString(1)

	secretsMu.Lock()
	defer secretsMu.Unlock()

	secret, exists := secrets[key]
	if !exists {
		return ctx.WriteError(fmt.Errorf("ERR secret not found"))
	}

	secret.Value = newValue
	secret.Version++
	secret.UpdatedAt = time.Now().UnixMilli()

	return ctx.WriteInteger(secret.Version)
}

func cmdSECRETVERSION(ctx *Context) error {
	if ctx.ArgCount() < 1 {
		return ctx.WriteError(ErrWrongArgCount)
	}

	key := ctx.ArgString(0)

	secretsMu.RLock()
	defer secretsMu.RUnlock()

	secret, exists := secrets[key]
	if !exists {
		return ctx.WriteNull()
	}

	return ctx.WriteArray([]*resp.Value{
		resp.BulkString("version"),
		resp.IntegerValue(secret.Version),
		resp.BulkString("updated_at"),
		resp.IntegerValue(secret.UpdatedAt),
	})
}

var (
	configs   = make(map[string]map[string]string)
	configsMu sync.RWMutex
)

func cmdCONFIGSET(ctx *Context) error {
	if ctx.ArgCount() < 3 {
		return ctx.WriteError(ErrWrongArgCount)
	}

	ns := ctx.ArgString(0)
	key := ctx.ArgString(1)
	value := ctx.ArgString(2)

	configsMu.Lock()
	defer configsMu.Unlock()

	if _, exists := configs[ns]; !exists {
		configs[ns] = make(map[string]string)
	}

	configs[ns][key] = value

	return ctx.WriteOK()
}

func cmdCONFIGGET(ctx *Context) error {
	if ctx.ArgCount() < 2 {
		return ctx.WriteError(ErrWrongArgCount)
	}

	ns := ctx.ArgString(0)
	key := ctx.ArgString(1)

	configsMu.RLock()
	defer configsMu.RUnlock()

	if nsConfig, exists := configs[ns]; exists {
		if value, ok := nsConfig[key]; ok {
			return ctx.WriteBulkString(value)
		}
	}

	return ctx.WriteNull()
}

func cmdCONFIGDELETE(ctx *Context) error {
	if ctx.ArgCount() < 2 {
		return ctx.WriteError(ErrWrongArgCount)
	}

	ns := ctx.ArgString(0)
	key := ctx.ArgString(1)

	configsMu.Lock()
	defer configsMu.Unlock()

	if nsConfig, exists := configs[ns]; exists {
		if _, ok := nsConfig[key]; ok {
			delete(nsConfig, key)
			return ctx.WriteInteger(1)
		}
	}

	return ctx.WriteInteger(0)
}

func cmdCONFIGLIST(ctx *Context) error {
	ns := ""
	if ctx.ArgCount() >= 1 {
		ns = ctx.ArgString(0)
	}

	configsMu.RLock()
	defer configsMu.RUnlock()

	results := make([]*resp.Value, 0)

	if ns != "" {
		if nsConfig, exists := configs[ns]; exists {
			for k, v := range nsConfig {
				results = append(results, resp.BulkString(k), resp.BulkString(v))
			}
		}
	} else {
		for ns, nsConfig := range configs {
			for k, v := range nsConfig {
				results = append(results,
					resp.BulkString(ns+":"+k),
					resp.BulkString(v),
				)
			}
		}
	}

	return ctx.WriteArray(results)
}

func cmdCONFIGNAMESPACE(ctx *Context) error {
	if ctx.ArgCount() < 1 {
		return ctx.WriteError(ErrWrongArgCount)
	}

	ns := ctx.ArgString(0)

	configsMu.Lock()
	defer configsMu.Unlock()

	if _, exists := configs[ns]; !exists {
		configs[ns] = make(map[string]string)
	}

	return ctx.WriteOK()
}

func cmdCONFIGIMPORT(ctx *Context) error {
	if ctx.ArgCount() < 2 {
		return ctx.WriteError(ErrWrongArgCount)
	}

	ns := ctx.ArgString(0)

	configsMu.Lock()
	defer configsMu.Unlock()

	if _, exists := configs[ns]; !exists {
		configs[ns] = make(map[string]string)
	}

	count := 0
	for i := 1; i+1 < ctx.ArgCount(); i += 2 {
		key := ctx.ArgString(i)
		value := ctx.ArgString(i + 1)
		configs[ns][key] = value
		count++
	}

	return ctx.WriteInteger(int64(count))
}

func cmdCONFIGEXPORT(ctx *Context) error {
	if ctx.ArgCount() < 1 {
		return ctx.WriteError(ErrWrongArgCount)
	}

	ns := ctx.ArgString(0)

	configsMu.RLock()
	defer configsMu.RUnlock()

	nsConfig, exists := configs[ns]
	if !exists {
		return ctx.WriteArray([]*resp.Value{})
	}

	results := make([]*resp.Value, 0)
	for k, v := range nsConfig {
		results = append(results, resp.BulkString(k), resp.BulkString(v))
	}

	return ctx.WriteArray(results)
}

var (
	tries   = make(map[string]*Trie)
	triesMu sync.RWMutex
)

type Trie struct {
	Root *TrieNode
}

type TrieNode struct {
	Children map[rune]*TrieNode
	IsEnd    bool
}

func cmdTRIEADD(ctx *Context) error {
	if ctx.ArgCount() < 2 {
		return ctx.WriteError(ErrWrongArgCount)
	}

	name := ctx.ArgString(0)
	word := ctx.ArgString(1)

	triesMu.Lock()
	defer triesMu.Unlock()

	if _, exists := tries[name]; !exists {
		tries[name] = &Trie{Root: &TrieNode{Children: make(map[rune]*TrieNode)}}
	}

	trie := tries[name]
	node := trie.Root

	for _, c := range word {
		if _, exists := node.Children[c]; !exists {
			node.Children[c] = &TrieNode{Children: make(map[rune]*TrieNode)}
		}
		node = node.Children[c]
	}

	node.IsEnd = true

	return ctx.WriteOK()
}

func cmdTRIESEARCH(ctx *Context) error {
	if ctx.ArgCount() < 2 {
		return ctx.WriteError(ErrWrongArgCount)
	}

	name := ctx.ArgString(0)
	word := ctx.ArgString(1)

	triesMu.RLock()
	defer triesMu.RUnlock()

	trie, exists := tries[name]
	if !exists {
		return ctx.WriteInteger(0)
	}

	node := trie.Root
	for _, c := range word {
		if _, exists := node.Children[c]; !exists {
			return ctx.WriteInteger(0)
		}
		node = node.Children[c]
	}

	if node.IsEnd {
		return ctx.WriteInteger(1)
	}
	return ctx.WriteInteger(0)
}

func cmdTRIEPREFIX(ctx *Context) error {
	if ctx.ArgCount() < 2 {
		return ctx.WriteError(ErrWrongArgCount)
	}

	name := ctx.ArgString(0)
	prefix := ctx.ArgString(1)

	triesMu.RLock()
	defer triesMu.RUnlock()

	trie, exists := tries[name]
	if !exists {
		return ctx.WriteInteger(0)
	}

	node := trie.Root
	for _, c := range prefix {
		if _, exists := node.Children[c]; !exists {
			return ctx.WriteInteger(0)
		}
		node = node.Children[c]
	}

	return ctx.WriteInteger(1)
}

func cmdTRIEDELETE(ctx *Context) error {
	if ctx.ArgCount() < 2 {
		return ctx.WriteError(ErrWrongArgCount)
	}

	name := ctx.ArgString(0)
	_ = ctx.ArgString(1)

	triesMu.RLock()
	_, exists := tries[name]
	triesMu.RUnlock()

	if !exists {
		return ctx.WriteInteger(0)
	}

	return ctx.WriteInteger(1)
}

func cmdTRIEAUTOCOMPLETE(ctx *Context) error {
	if ctx.ArgCount() < 2 {
		return ctx.WriteError(ErrWrongArgCount)
	}

	name := ctx.ArgString(0)
	prefix := ctx.ArgString(1)

	triesMu.RLock()
	defer triesMu.RUnlock()

	trie, exists := tries[name]
	if !exists {
		return ctx.WriteArray([]*resp.Value{})
	}

	node := trie.Root
	for _, c := range prefix {
		if _, exists := node.Children[c]; !exists {
			return ctx.WriteArray([]*resp.Value{})
		}
		node = node.Children[c]
	}

	results := make([]string, 0)
	collectWords(node, prefix, &results)

	respResults := make([]*resp.Value, len(results))
	for i, w := range results {
		respResults[i] = resp.BulkString(w)
	}

	return ctx.WriteArray(respResults)
}

func collectWords(node *TrieNode, prefix string, results *[]string) {
	if node.IsEnd {
		*results = append(*results, prefix)
	}

	for c, child := range node.Children {
		collectWords(child, prefix+string(c), results)
	}
}

var (
	rings   = make(map[string]*HashRing)
	ringsMu sync.RWMutex
)

type HashRing struct {
	Name     string
	Nodes    []string
	Replicas int
}

func cmdRINGCREATE(ctx *Context) error {
	if ctx.ArgCount() < 2 {
		return ctx.WriteError(ErrWrongArgCount)
	}

	name := ctx.ArgString(0)
	replicas := int(parseInt64(ctx.ArgString(1)))

	ringsMu.Lock()
	rings[name] = &HashRing{
		Name:     name,
		Nodes:    make([]string, 0),
		Replicas: replicas,
	}
	ringsMu.Unlock()

	return ctx.WriteOK()
}

func cmdRINGADD(ctx *Context) error {
	if ctx.ArgCount() < 2 {
		return ctx.WriteError(ErrWrongArgCount)
	}

	name := ctx.ArgString(0)
	node := ctx.ArgString(1)

	ringsMu.RLock()
	ring, exists := rings[name]
	ringsMu.RUnlock()

	if !exists {
		return ctx.WriteError(fmt.Errorf("ERR ring not found"))
	}

	ringsMu.Lock()
	ring.Nodes = append(ring.Nodes, node)
	ringsMu.Unlock()

	return ctx.WriteInteger(int64(len(ring.Nodes)))
}

func cmdRINGGET(ctx *Context) error {
	if ctx.ArgCount() < 2 {
		return ctx.WriteError(ErrWrongArgCount)
	}

	name := ctx.ArgString(0)
	key := ctx.ArgString(1)

	ringsMu.RLock()
	ring, exists := rings[name]
	ringsMu.RUnlock()

	if !exists {
		return ctx.WriteError(fmt.Errorf("ERR ring not found"))
	}

	if len(ring.Nodes) == 0 {
		return ctx.WriteNull()
	}

	hash := hashKey(key)
	idx := hash % uint64(len(ring.Nodes))

	return ctx.WriteBulkString(ring.Nodes[idx])
}

func hashKey(s string) uint64 {
	var hash uint64
	for _, c := range s {
		hash = hash*31 + uint64(c)
	}
	return hash
}

func cmdRINGNODES(ctx *Context) error {
	if ctx.ArgCount() < 1 {
		return ctx.WriteError(ErrWrongArgCount)
	}

	name := ctx.ArgString(0)

	ringsMu.RLock()
	ring, exists := rings[name]
	ringsMu.RUnlock()

	if !exists {
		return ctx.WriteError(fmt.Errorf("ERR ring not found"))
	}

	results := make([]*resp.Value, len(ring.Nodes))
	for i, n := range ring.Nodes {
		results[i] = resp.BulkString(n)
	}

	return ctx.WriteArray(results)
}

func cmdRINGREMOVE(ctx *Context) error {
	if ctx.ArgCount() < 2 {
		return ctx.WriteError(ErrWrongArgCount)
	}

	name := ctx.ArgString(0)
	node := ctx.ArgString(1)

	ringsMu.RLock()
	ring, exists := rings[name]
	ringsMu.RUnlock()

	if !exists {
		return ctx.WriteInteger(0)
	}

	ringsMu.Lock()
	defer ringsMu.Unlock()

	for i, n := range ring.Nodes {
		if n == node {
			ring.Nodes = append(ring.Nodes[:i], ring.Nodes[i+1:]...)
			return ctx.WriteInteger(1)
		}
	}

	return ctx.WriteInteger(0)
}

var (
	semaphores   = make(map[string]*Semaphore)
	semaphoresMu sync.RWMutex
)

type Semaphore struct {
	Value   int64
	Max     int64
	Waiters int64
	mu      sync.Mutex
}

func cmdSEMCREATE(ctx *Context) error {
	if ctx.ArgCount() < 2 {
		return ctx.WriteError(ErrWrongArgCount)
	}

	name := ctx.ArgString(0)
	max := parseInt64(ctx.ArgString(1))

	semaphoresMu.Lock()
	semaphores[name] = &Semaphore{
		Value: max,
		Max:   max,
	}
	semaphoresMu.Unlock()

	return ctx.WriteOK()
}

func cmdSEMACQUIRE(ctx *Context) error {
	if ctx.ArgCount() < 2 {
		return ctx.WriteError(ErrWrongArgCount)
	}

	name := ctx.ArgString(0)
	n := parseInt64(ctx.ArgString(1))

	semaphoresMu.RLock()
	sem, exists := semaphores[name]
	semaphoresMu.RUnlock()

	if !exists {
		return ctx.WriteError(fmt.Errorf("ERR semaphore not found"))
	}

	sem.mu.Lock()
	defer sem.mu.Unlock()

	if sem.Value >= n {
		sem.Value -= n
		return ctx.WriteInteger(1)
	}

	sem.Waiters++
	return ctx.WriteInteger(0)
}

func cmdSEMRELEASE(ctx *Context) error {
	if ctx.ArgCount() < 2 {
		return ctx.WriteError(ErrWrongArgCount)
	}

	name := ctx.ArgString(0)
	n := parseInt64(ctx.ArgString(1))

	semaphoresMu.RLock()
	sem, exists := semaphores[name]
	semaphoresMu.RUnlock()

	if !exists {
		return ctx.WriteError(fmt.Errorf("ERR semaphore not found"))
	}

	sem.mu.Lock()
	defer sem.mu.Unlock()

	sem.Value += n
	if sem.Value > sem.Max {
		sem.Value = sem.Max
	}

	if sem.Waiters > 0 {
		sem.Waiters--
	}

	return ctx.WriteOK()
}

func cmdSEMTRYACQUIRE(ctx *Context) error {
	if ctx.ArgCount() < 2 {
		return ctx.WriteError(ErrWrongArgCount)
	}

	name := ctx.ArgString(0)
	n := parseInt64(ctx.ArgString(1))

	semaphoresMu.RLock()
	sem, exists := semaphores[name]
	semaphoresMu.RUnlock()

	if !exists {
		return ctx.WriteError(fmt.Errorf("ERR semaphore not found"))
	}

	sem.mu.Lock()
	defer sem.mu.Unlock()

	if sem.Value >= n {
		sem.Value -= n
		return ctx.WriteInteger(1)
	}

	return ctx.WriteInteger(0)
}

func cmdSEMVALUE(ctx *Context) error {
	if ctx.ArgCount() < 1 {
		return ctx.WriteError(ErrWrongArgCount)
	}

	name := ctx.ArgString(0)

	semaphoresMu.RLock()
	sem, exists := semaphores[name]
	semaphoresMu.RUnlock()

	if !exists {
		return ctx.WriteError(fmt.Errorf("ERR semaphore not found"))
	}

	sem.mu.Lock()
	defer sem.mu.Unlock()

	return ctx.WriteArray([]*resp.Value{
		resp.BulkString("value"),
		resp.IntegerValue(sem.Value),
		resp.BulkString("max"),
		resp.IntegerValue(sem.Max),
		resp.BulkString("waiters"),
		resp.IntegerValue(sem.Waiters),
	})
}
