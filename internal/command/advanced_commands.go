package command

import (
	"hash/fnv"
	"strings"
	"sync"

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
	var result strings.Builder
	for _, c := range s {
		if c >= 'a' && c <= 'z' {
			result.WriteString(string(c - 32))
		} else {
			result.WriteString(string(c))
		}
	}
	return result.String()
}

func toLower(s string) string {
	var result strings.Builder
	for _, c := range s {
		if c >= 'A' && c <= 'Z' {
			result.WriteString(string(c + 32))
		} else {
			result.WriteString(string(c))
		}
	}
	return result.String()
}

func reverse(s string) string {
	runes := []rune(s)
	for i, j := 0, len(runes)-1; i < j; i, j = i+1, j-1 {
		runes[i], runes[j] = runes[j], runes[i]
	}
	return string(runes)
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
