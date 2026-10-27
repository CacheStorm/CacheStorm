package command

import (
	"errors"
	"fmt"
	"hash/fnv"
	"math"
	"sort"
	"strconv"
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

	results := make([]*resp.Value, 0, len(actors))
	for id := range actors {
		results = append(results, resp.BulkString(id))
	}

	reply := results
	actorsMu.RUnlock()
	return ctx.WriteArray(reply)
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
	if ctx.ArgCount() != 3 {
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
	if !dag.Nodes[from] || !dag.Nodes[to] {
		dag.mu.Unlock()
		return ctx.WriteError(fmt.Errorf("ERR node not found"))
	}
	for _, c := range dag.Edges[from] {
		if c == to {
			dag.mu.Unlock()
			return ctx.WriteOK()
		}
	}
	if from == to || dag.reaches(to, from) {
		dag.mu.Unlock()
		return ctx.WriteError(fmt.Errorf("ERR edge would create a cycle"))
	}
	dag.Edges[from] = append(dag.Edges[from], to)
	dag.InDegree[to]++
	dag.mu.Unlock()

	return ctx.WriteOK()
}

// reaches reports whether target is reachable from start. Callers hold d.mu.
func (d *DAG) reaches(start, target string) bool {
	seen := map[string]bool{start: true}
	stack := []string{start}
	for len(stack) > 0 {
		node := stack[len(stack)-1]
		stack = stack[:len(stack)-1]
		if node == target {
			return true
		}
		for _, next := range d.Edges[node] {
			if !seen[next] {
				seen[next] = true
				stack = append(stack, next)
			}
		}
	}
	return false
}

func cmdDAGTOPO(ctx *Context) error {
	if ctx.ArgCount() != 1 {
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
	inDegree := make(map[string]int, len(dag.Nodes))
	ready := make([]string, 0)
	for node := range dag.Nodes {
		inDegree[node] = dag.InDegree[node]
		if inDegree[node] == 0 {
			ready = append(ready, node)
		}
	}
	sort.Strings(ready)

	// Kahn's algorithm, always taking the smallest ready node so the order is
	// deterministic.
	result := make([]*resp.Value, 0, len(dag.Nodes))
	for len(ready) > 0 {
		node := ready[0]
		ready = ready[1:]
		result = append(result, resp.BulkString(node))
		for _, child := range dag.Edges[node] {
			inDegree[child]--
			if inDegree[child] == 0 {
				i := sort.SearchStrings(ready, child)
				ready = append(ready, "")
				copy(ready[i+1:], ready[i:])
				ready[i] = child
			}
		}
	}
	complete := len(result) == len(dag.Nodes)
	dag.mu.RUnlock()

	if !complete {
		return ctx.WriteError(fmt.Errorf("ERR DAG contains a cycle"))
	}
	return ctx.WriteArray(result)
}

func cmdDAGPARENTS(ctx *Context) error {
	if ctx.ArgCount() != 2 {
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
	parents := make([]string, 0)
	for from, edges := range dag.Edges {
		for _, to := range edges {
			if to == node {
				parents = append(parents, from)
				break
			}
		}
	}
	dag.mu.RUnlock()
	sort.Strings(parents)

	results := make([]*resp.Value, len(parents))
	for i, p := range parents {
		results[i] = resp.BulkString(p)
	}

	return ctx.WriteArray(results)
}

func cmdDAGCHILDREN(ctx *Context) error {
	if ctx.ArgCount() != 2 {
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
	children := append([]string(nil), dag.Edges[node]...)
	dag.mu.RUnlock()
	sort.Strings(children)

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

	results := make([]*resp.Value, 0, len(dags))
	for name := range dags {
		results = append(results, resp.BulkString(name))
	}

	reply := results
	dagsMu.RUnlock()
	return ctx.WriteArray(reply)
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

	secret, exists := secrets[key]
	if !exists {
		secretsMu.RUnlock()
		return ctx.WriteNull()
	}

	reply := secret.Value
	secretsMu.RUnlock()
	return ctx.WriteBulkString(reply)
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

	keys := make([]string, 0, len(secrets))
	for k := range secrets {
		keys = append(keys, k)
	}

	results := make([]*resp.Value, len(keys))
	for i, k := range keys {
		results[i] = resp.BulkString(k)
	}

	reply := results
	secretsMu.RUnlock()
	return ctx.WriteArray(reply)
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

	secret, exists := secrets[key]
	if !exists {
		secretsMu.RUnlock()
		return ctx.WriteNull()
	}

	reply := []*resp.Value{
		resp.BulkString("version"),
		resp.IntegerValue(secret.Version),
		resp.BulkString("updated_at"),
		resp.IntegerValue(secret.UpdatedAt),
	}
	secretsMu.RUnlock()
	return ctx.WriteArray(reply)
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

	if nsConfig, exists := configs[ns]; exists {
		if value, ok := nsConfig[key]; ok {
			reply := value
			configsMu.RUnlock()
			return ctx.WriteBulkString(reply)
		}
	}

	configsMu.RUnlock()
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

	reply := results
	configsMu.RUnlock()
	return ctx.WriteArray(reply)
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
	// The loop below consumes key/value pairs from index 1, so ns alone or
	// ns+key imports nothing. Requiring a complete pair stops the command
	// from accepting a call that creates an empty namespace and reports 0.
	// The parity check matters too: `CONFIG.IMPORT ns k1 v1 k2` stored k1=v1,
	// silently discarded k2 and still answered :1, so the caller was told its
	// whole call had been applied when one setting never existed.
	if ctx.ArgCount() < 3 || (ctx.ArgCount()-1)%2 != 0 {
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

	nsConfig, exists := configs[ns]
	if !exists {
		reply := []*resp.Value{}
		configsMu.RUnlock()
		return ctx.WriteArray(reply)
	}

	results := make([]*resp.Value, 0)
	for k, v := range nsConfig {
		results = append(results, resp.BulkString(k), resp.BulkString(v))
	}

	reply := results
	configsMu.RUnlock()
	return ctx.WriteArray(reply)
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

	trie, exists := tries[name]
	if !exists {
		reply := int64(0)
		triesMu.RUnlock()
		return ctx.WriteInteger(reply)
	}

	node := trie.Root
	for _, c := range word {
		if _, exists := node.Children[c]; !exists {
			reply := int64(0)
			triesMu.RUnlock()
			return ctx.WriteInteger(reply)
		}
		node = node.Children[c]
	}

	if node.IsEnd {
		reply := int64(1)
		triesMu.RUnlock()
		return ctx.WriteInteger(reply)
	}
	reply := int64(0)
	triesMu.RUnlock()
	return ctx.WriteInteger(reply)
}

func cmdTRIEPREFIX(ctx *Context) error {
	if ctx.ArgCount() < 2 {
		return ctx.WriteError(ErrWrongArgCount)
	}

	name := ctx.ArgString(0)
	prefix := ctx.ArgString(1)

	triesMu.RLock()

	trie, exists := tries[name]
	if !exists {
		reply := int64(0)
		triesMu.RUnlock()
		return ctx.WriteInteger(reply)
	}

	node := trie.Root
	for _, c := range prefix {
		if _, exists := node.Children[c]; !exists {
			reply := int64(0)
			triesMu.RUnlock()
			return ctx.WriteInteger(reply)
		}
		node = node.Children[c]
	}

	reply := int64(1)
	triesMu.RUnlock()
	return ctx.WriteInteger(reply)
}

func cmdTRIEDELETE(ctx *Context) error {
	if ctx.ArgCount() < 2 {
		return ctx.WriteError(ErrWrongArgCount)
	}

	name := ctx.ArgString(0)
	word := ctx.ArgString(1)

	triesMu.Lock()
	defer triesMu.Unlock()

	trie, exists := tries[name]
	if !exists {
		return ctx.WriteInteger(0)
	}

	node := trie.Root
	for _, c := range word {
		next, ok := node.Children[c]
		if !ok {
			return ctx.WriteInteger(0)
		}
		node = next
	}
	if !node.IsEnd {
		return ctx.WriteInteger(0)
	}
	node.IsEnd = false

	return ctx.WriteInteger(1)
}

func cmdTRIEAUTOCOMPLETE(ctx *Context) error {
	if ctx.ArgCount() < 2 {
		return ctx.WriteError(ErrWrongArgCount)
	}

	name := ctx.ArgString(0)
	prefix := ctx.ArgString(1)

	triesMu.RLock()

	trie, exists := tries[name]
	if !exists {
		reply := []*resp.Value{}
		triesMu.RUnlock()
		return ctx.WriteArray(reply)
	}

	node := trie.Root
	for _, c := range prefix {
		if _, exists := node.Children[c]; !exists {
			reply := []*resp.Value{}
			triesMu.RUnlock()
			return ctx.WriteArray(reply)
		}
		node = node.Children[c]
	}

	results := make([]string, 0)
	collectWords(node, prefix, &results)

	respResults := make([]*resp.Value, len(results))
	for i, w := range results {
		respResults[i] = resp.BulkString(w)
	}

	reply := respResults
	triesMu.RUnlock()
	return ctx.WriteArray(reply)
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

const maxRingReplicas = 1024

// HashRing is a consistent-hash ring: every node owns Replicas virtual points
// on a 64-bit circle, and a key belongs to the first point at or after its
// own hash. Adding or removing a node only moves the keys of that node's
// points.
type HashRing struct {
	Name     string
	Nodes    []string
	Replicas int
	points   []ringPoint
}

type ringPoint struct {
	hash uint64
	node string
}

func ringHash(s string) uint64 {
	h := fnv.New64a()
	_, _ = h.Write([]byte(s))
	x := h.Sum64()
	// FNV alone spreads near-identical strings such as "node#1" and "node#2"
	// poorly across the circle; the splitmix64 finalizer fixes that.
	x ^= x >> 30
	x *= 0xbf58476d1ce4e5b9
	x ^= x >> 27
	x *= 0x94d049bb133111eb
	x ^= x >> 31
	return x
}

func (r *HashRing) rebuild() {
	points := make([]ringPoint, 0, len(r.Nodes)*r.Replicas)
	for _, node := range r.Nodes {
		for i := 0; i < r.Replicas; i++ {
			points = append(points, ringPoint{hash: ringHash(node + "#" + strconv.Itoa(i)), node: node})
		}
	}
	sort.Slice(points, func(i, j int) bool {
		if points[i].hash != points[j].hash {
			return points[i].hash < points[j].hash
		}
		return points[i].node < points[j].node
	})
	r.points = points
}

func (r *HashRing) locate(key string) string {
	h := ringHash(key)
	i := sort.Search(len(r.points), func(i int) bool { return r.points[i].hash >= h })
	if i == len(r.points) {
		i = 0
	}
	return r.points[i].node
}

func cmdRINGCREATE(ctx *Context) error {
	if ctx.ArgCount() != 2 {
		return ctx.WriteError(ErrWrongArgCount)
	}

	name := ctx.ArgString(0)
	replicas, err := strconv.Atoi(ctx.ArgString(1))
	if err != nil || replicas < 1 || replicas > maxRingReplicas {
		return ctx.WriteError(fmt.Errorf("ERR replicas must be an integer between 1 and %d", maxRingReplicas))
	}

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
	if ctx.ArgCount() != 2 {
		return ctx.WriteError(ErrWrongArgCount)
	}

	name := ctx.ArgString(0)
	node := ctx.ArgString(1)

	// The ring is looked up and mutated under a single write lock, so a
	// concurrent RING.CREATE cannot replace it between the two.
	ringsMu.Lock()
	ring, exists := rings[name]
	if !exists {
		ringsMu.Unlock()
		return ctx.WriteError(fmt.Errorf("ERR ring not found"))
	}

	i := sort.SearchStrings(ring.Nodes, node)
	if i == len(ring.Nodes) || ring.Nodes[i] != node {
		ring.Nodes = append(ring.Nodes, "")
		copy(ring.Nodes[i+1:], ring.Nodes[i:])
		ring.Nodes[i] = node
		ring.rebuild()
	}
	size := len(ring.Nodes)
	ringsMu.Unlock()

	return ctx.WriteInteger(int64(size))
}

func cmdRINGGET(ctx *Context) error {
	if ctx.ArgCount() != 2 {
		return ctx.WriteError(ErrWrongArgCount)
	}

	name := ctx.ArgString(0)
	key := ctx.ArgString(1)

	ringsMu.RLock()
	ring, exists := rings[name]
	if !exists {
		ringsMu.RUnlock()
		return ctx.WriteError(fmt.Errorf("ERR ring not found"))
	}
	if len(ring.points) == 0 {
		ringsMu.RUnlock()
		return ctx.WriteNull()
	}
	node := ring.locate(key)
	ringsMu.RUnlock()

	return ctx.WriteBulkString(node)
}

func cmdRINGNODES(ctx *Context) error {
	if ctx.ArgCount() != 1 {
		return ctx.WriteError(ErrWrongArgCount)
	}

	name := ctx.ArgString(0)

	ringsMu.RLock()
	ring, exists := rings[name]
	if !exists {
		ringsMu.RUnlock()
		return ctx.WriteError(fmt.Errorf("ERR ring not found"))
	}
	results := make([]*resp.Value, len(ring.Nodes))
	for i, n := range ring.Nodes {
		results[i] = resp.BulkString(n)
	}
	ringsMu.RUnlock()

	return ctx.WriteArray(results)
}

func cmdRINGREMOVE(ctx *Context) error {
	if ctx.ArgCount() != 2 {
		return ctx.WriteError(ErrWrongArgCount)
	}

	name := ctx.ArgString(0)
	node := ctx.ArgString(1)

	ringsMu.Lock()
	removed := false
	if ring, exists := rings[name]; exists {
		i := sort.SearchStrings(ring.Nodes, node)
		if i < len(ring.Nodes) && ring.Nodes[i] == node {
			ring.Nodes = append(ring.Nodes[:i], ring.Nodes[i+1:]...)
			ring.rebuild()
			removed = true
		}
	}
	ringsMu.Unlock()

	if removed {
		return ctx.WriteInteger(1)
	}
	return ctx.WriteInteger(0)
}

var (
	semaphores   = make(map[string]*Semaphore)
	semaphoresMu sync.RWMutex
)

type Semaphore struct {
	Value int64
	Max   int64
	// Waiters counts the SEM.ACQUIRE calls currently blocked on this
	// semaphore.
	Waiters int64
	mu      sync.Mutex
	// changed is closed, and replaced, whenever permits are released, so a
	// blocked SEM.ACQUIRE re-checks the value.
	changed  chan struct{}
	replaced bool
}

func newSemaphore(max int64) *Semaphore {
	return &Semaphore{Value: max, Max: max, changed: make(chan struct{})}
}

// signal wakes every blocked acquirer. Callers hold s.mu.
func (s *Semaphore) signal() {
	close(s.changed)
	s.changed = make(chan struct{})
}

var (
	errSemNotFound  = errors.New("ERR semaphore not found")
	errSemCount     = errors.New("ERR permit count must be a positive integer")
	errSemOverMax   = errors.New("ERR permit count exceeds the semaphore's maximum")
	errSemOverflow  = errors.New("ERR release would exceed the semaphore's maximum")
	errSemTimeout   = errors.New("ERR timeout must be a non-negative integer (milliseconds)")
	errSemCreateMax = errors.New("ERR max must be a positive integer")
)

func lookupSemaphore(name string) (*Semaphore, bool) {
	semaphoresMu.RLock()
	sem, exists := semaphores[name]
	semaphoresMu.RUnlock()
	return sem, exists
}

func parseSemCount(s string) (int64, error) {
	n, err := strconv.ParseInt(s, 10, 64)
	if err != nil || n < 1 {
		return 0, errSemCount
	}
	return n, nil
}

func cmdSEMCREATE(ctx *Context) error {
	if ctx.ArgCount() != 2 {
		return ctx.WriteError(ErrWrongArgCount)
	}

	name := ctx.ArgString(0)
	max, err := strconv.ParseInt(ctx.ArgString(1), 10, 64)
	if err != nil || max < 1 {
		return ctx.WriteError(errSemCreateMax)
	}

	semaphoresMu.Lock()
	old := semaphores[name]
	semaphores[name] = newSemaphore(max)
	semaphoresMu.Unlock()

	if old != nil {
		old.mu.Lock()
		old.replaced = true
		old.signal()
		old.mu.Unlock()
	}

	return ctx.WriteOK()
}

// acquire takes n permits, waiting for them when timeout is not negative
// (0 waits indefinitely). It reports whether the permits were taken.
func (s *Semaphore) acquire(ctx *Context, n int64, timeout time.Duration) (bool, error) {
	s.mu.Lock()
	if s.replaced {
		s.mu.Unlock()
		return false, errSemNotFound
	}
	if n > s.Max {
		s.mu.Unlock()
		return false, errSemOverMax
	}
	if s.Value >= n {
		s.Value -= n
		s.mu.Unlock()
		return true, nil
	}
	if timeout < 0 || ctx.InExec {
		s.mu.Unlock()
		return false, nil
	}
	s.Waiters++
	changed := s.changed
	s.mu.Unlock()

	if ctx.BeforeWait != nil {
		ctx.BeforeWait()
		defer ctx.AfterWait()
	}
	var expired <-chan time.Time
	if timeout > 0 {
		timer := time.NewTimer(timeout)
		defer timer.Stop()
		expired = timer.C
	}

	for {
		stop := false
		select {
		case <-changed:
		case <-expired:
			stop = true
		case <-ctx.Done:
			// The client is gone: permits taken now would never be released.
			s.mu.Lock()
			s.Waiters--
			s.mu.Unlock()
			return false, nil
		}
		s.mu.Lock()
		if s.replaced {
			s.Waiters--
			s.mu.Unlock()
			return false, errSemNotFound
		}
		if s.Value >= n {
			s.Value -= n
			s.Waiters--
			s.mu.Unlock()
			return true, nil
		}
		if stop {
			s.Waiters--
			s.mu.Unlock()
			return false, nil
		}
		changed = s.changed
		s.mu.Unlock()
	}
}

func cmdSEMACQUIRE(ctx *Context) error {
	if ctx.ArgCount() < 2 || ctx.ArgCount() > 3 {
		return ctx.WriteError(ErrWrongArgCount)
	}

	name := ctx.ArgString(0)
	n, err := parseSemCount(ctx.ArgString(1))
	if err != nil {
		return ctx.WriteError(err)
	}
	timeout := time.Duration(0)
	if ctx.ArgCount() == 3 {
		ms, err := strconv.ParseInt(ctx.ArgString(2), 10, 64)
		if err != nil || ms < 0 || ms > math.MaxInt64/int64(time.Millisecond) {
			return ctx.WriteError(errSemTimeout)
		}
		timeout = time.Duration(ms) * time.Millisecond
	}

	sem, exists := lookupSemaphore(name)
	if !exists {
		return ctx.WriteError(errSemNotFound)
	}

	ok, err := sem.acquire(ctx, n, timeout)
	if err != nil {
		return ctx.WriteError(err)
	}
	if ok {
		return ctx.WriteInteger(1)
	}
	return ctx.WriteInteger(0)
}

func cmdSEMRELEASE(ctx *Context) error {
	if ctx.ArgCount() != 2 {
		return ctx.WriteError(ErrWrongArgCount)
	}

	name := ctx.ArgString(0)
	n, err := parseSemCount(ctx.ArgString(1))
	if err != nil {
		return ctx.WriteError(err)
	}

	sem, exists := lookupSemaphore(name)
	if !exists {
		return ctx.WriteError(errSemNotFound)
	}

	sem.mu.Lock()
	if n > sem.Max-sem.Value {
		sem.mu.Unlock()
		return ctx.WriteError(errSemOverflow)
	}
	sem.Value += n
	sem.signal()
	sem.mu.Unlock()

	return ctx.WriteOK()
}

func cmdSEMTRYACQUIRE(ctx *Context) error {
	if ctx.ArgCount() != 2 {
		return ctx.WriteError(ErrWrongArgCount)
	}

	name := ctx.ArgString(0)
	n, err := parseSemCount(ctx.ArgString(1))
	if err != nil {
		return ctx.WriteError(err)
	}

	sem, exists := lookupSemaphore(name)
	if !exists {
		return ctx.WriteError(errSemNotFound)
	}

	ok, err := sem.acquire(ctx, n, -1)
	if err != nil {
		return ctx.WriteError(err)
	}
	if ok {
		return ctx.WriteInteger(1)
	}
	return ctx.WriteInteger(0)
}

func cmdSEMVALUE(ctx *Context) error {
	if ctx.ArgCount() != 1 {
		return ctx.WriteError(ErrWrongArgCount)
	}

	name := ctx.ArgString(0)

	sem, exists := lookupSemaphore(name)
	if !exists {
		return ctx.WriteError(errSemNotFound)
	}

	sem.mu.Lock()
	reply := []*resp.Value{
		resp.BulkString("value"),
		resp.IntegerValue(sem.Value),
		resp.BulkString("max"),
		resp.IntegerValue(sem.Max),
		resp.BulkString("waiters"),
		resp.IntegerValue(sem.Waiters),
	}
	sem.mu.Unlock()

	return ctx.WriteArray(reply)
}
