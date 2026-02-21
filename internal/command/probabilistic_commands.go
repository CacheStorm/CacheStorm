package command

import (
	"fmt"
	"strings"
	"sync"

	"github.com/cachestorm/cachestorm/internal/resp"
	"github.com/cachestorm/cachestorm/internal/store"
)

var (
	bloomFilters    = make(map[string]*store.BloomFilter)
	countMinSketch  = make(map[string]*store.CountMinSketch)
	topK            = make(map[string]*store.TopK)
	cuckooFilters   = make(map[string]*store.CuckooFilter)
	probabilisticMu sync.RWMutex
)

func RegisterProbabilisticCommands(router *Router) {
	router.Register(&CommandDef{Name: "BF.ADD", Handler: cmdBFADD})
	router.Register(&CommandDef{Name: "BF.EXISTS", Handler: cmdBFEXISTS})
	router.Register(&CommandDef{Name: "BF.INFO", Handler: cmdBFINFO})
	router.Register(&CommandDef{Name: "BF.RESERVE", Handler: cmdBFRESERVE})
	router.Register(&CommandDef{Name: "BF.MADD", Handler: cmdBFMADD})
	router.Register(&CommandDef{Name: "BF.MEXISTS", Handler: cmdBFMEXISTS})

	router.Register(&CommandDef{Name: "CF.ADD", Handler: cmdCFADD})
	router.Register(&CommandDef{Name: "CF.EXISTS", Handler: cmdCFEXISTS})
	router.Register(&CommandDef{Name: "CF.DEL", Handler: cmdCFDEL})
	router.Register(&CommandDef{Name: "CF.INFO", Handler: cmdCFINFO})
	router.Register(&CommandDef{Name: "CF.RESERVE", Handler: cmdCFRESERVE})

	router.Register(&CommandDef{Name: "CMS.INCRBY", Handler: cmdCMSINCRBY})
	router.Register(&CommandDef{Name: "CMS.QUERY", Handler: cmdCMSQUERY})
	router.Register(&CommandDef{Name: "CMS.INFO", Handler: cmdCMSINFO})
	router.Register(&CommandDef{Name: "CMS.INIT", Handler: cmdCMSINIT})

	router.Register(&CommandDef{Name: "TOPK.ADD", Handler: cmdTOPKADD})
	router.Register(&CommandDef{Name: "TOPK.QUERY", Handler: cmdTOPKQUERY})
	router.Register(&CommandDef{Name: "TOPK.LIST", Handler: cmdTOPKLIST})
	router.Register(&CommandDef{Name: "TOPK.INFO", Handler: cmdTOPKINFO})
	router.Register(&CommandDef{Name: "TOPK.RESERVE", Handler: cmdTOPKRESERVE})
}

func cmdBFADD(ctx *Context) error {
	if ctx.ArgCount() < 2 {
		return ctx.WriteError(ErrWrongArgCount)
	}

	key := ctx.ArgString(0)
	item := ctx.Arg(1)

	probabilisticMu.Lock()
	bf, exists := bloomFilters[key]
	if !exists {
		bf = store.NewBloomFilter(1000000, 0.01)
		bloomFilters[key] = bf
	}
	probabilisticMu.Unlock()

	bf.Add(item)
	return ctx.WriteInteger(1)
}

func cmdBFEXISTS(ctx *Context) error {
	if ctx.ArgCount() < 2 {
		return ctx.WriteError(ErrWrongArgCount)
	}

	key := ctx.ArgString(0)
	item := ctx.Arg(1)

	probabilisticMu.RLock()
	bf, exists := bloomFilters[key]
	probabilisticMu.RUnlock()

	if !exists {
		return ctx.WriteInteger(0)
	}

	if bf.Exists(item) {
		return ctx.WriteInteger(1)
	}
	return ctx.WriteInteger(0)
}

func cmdBFINFO(ctx *Context) error {
	if ctx.ArgCount() < 1 {
		return ctx.WriteError(ErrWrongArgCount)
	}

	key := ctx.ArgString(0)

	probabilisticMu.RLock()
	bf, exists := bloomFilters[key]
	probabilisticMu.RUnlock()

	if !exists {
		return ctx.WriteError(fmt.Errorf("ERR key not found"))
	}

	info := bf.Info()

	return ctx.WriteArray([]*resp.Value{
		resp.BulkString("size"),
		resp.IntegerValue(int64(info["size"].(uint))),
		resp.BulkString("hashes"),
		resp.IntegerValue(int64(info["hashes"].(uint))),
		resp.BulkString("count"),
		resp.IntegerValue(int64(info["count"].(uint))),
		resp.BulkString("bits_set"),
		resp.IntegerValue(int64(info["bits_set"].(int))),
		resp.BulkString("fill_rate"),
		resp.BulkString(fmt.Sprintf("%.4f", info["fill_rate"])),
	})
}

func cmdBFRESERVE(ctx *Context) error {
	if ctx.ArgCount() < 2 {
		return ctx.WriteError(ErrWrongArgCount)
	}

	key := ctx.ArgString(0)
	capacity := int(parseInt64(ctx.ArgString(1)))
	errorRate := 0.01

	if ctx.ArgCount() >= 3 {
		errorRate = parseJSONFloat(ctx.ArgString(2))
	}

	probabilisticMu.Lock()
	bloomFilters[key] = store.NewBloomFilter(uint(capacity*10), errorRate)
	probabilisticMu.Unlock()

	return ctx.WriteOK()
}

func cmdBFMADD(ctx *Context) error {
	if ctx.ArgCount() < 2 {
		return ctx.WriteError(ErrWrongArgCount)
	}

	key := ctx.ArgString(0)

	probabilisticMu.Lock()
	bf, exists := bloomFilters[key]
	if !exists {
		bf = store.NewBloomFilter(1000000, 0.01)
		bloomFilters[key] = bf
	}
	probabilisticMu.Unlock()

	results := make([]*resp.Value, 0, ctx.ArgCount()-1)
	for i := 1; i < ctx.ArgCount(); i++ {
		bf.Add(ctx.Arg(i))
		results = append(results, resp.IntegerValue(1))
	}

	return ctx.WriteArray(results)
}

func cmdBFMEXISTS(ctx *Context) error {
	if ctx.ArgCount() < 2 {
		return ctx.WriteError(ErrWrongArgCount)
	}

	key := ctx.ArgString(0)

	probabilisticMu.RLock()
	bf, exists := bloomFilters[key]
	probabilisticMu.RUnlock()

	if !exists {
		results := make([]*resp.Value, 0, ctx.ArgCount()-1)
		for i := 1; i < ctx.ArgCount(); i++ {
			results = append(results, resp.IntegerValue(0))
		}
		return ctx.WriteArray(results)
	}

	results := make([]*resp.Value, 0, ctx.ArgCount()-1)
	for i := 1; i < ctx.ArgCount(); i++ {
		if bf.Exists(ctx.Arg(i)) {
			results = append(results, resp.IntegerValue(1))
		} else {
			results = append(results, resp.IntegerValue(0))
		}
	}

	return ctx.WriteArray(results)
}

func cmdCFADD(ctx *Context) error {
	if ctx.ArgCount() < 2 {
		return ctx.WriteError(ErrWrongArgCount)
	}

	key := ctx.ArgString(0)
	item := ctx.Arg(1)

	probabilisticMu.Lock()
	cf, exists := cuckooFilters[key]
	if !exists {
		cf = store.NewCuckooFilter(1000000, 2)
		cuckooFilters[key] = cf
	}
	probabilisticMu.Unlock()

	if cf.Add(item) {
		return ctx.WriteInteger(1)
	}
	return ctx.WriteInteger(0)
}

func cmdCFEXISTS(ctx *Context) error {
	if ctx.ArgCount() < 2 {
		return ctx.WriteError(ErrWrongArgCount)
	}

	key := ctx.ArgString(0)
	item := ctx.Arg(1)

	probabilisticMu.RLock()
	cf, exists := cuckooFilters[key]
	probabilisticMu.RUnlock()

	if !exists {
		return ctx.WriteInteger(0)
	}

	if cf.Exists(item) {
		return ctx.WriteInteger(1)
	}
	return ctx.WriteInteger(0)
}

func cmdCFDEL(ctx *Context) error {
	if ctx.ArgCount() < 2 {
		return ctx.WriteError(ErrWrongArgCount)
	}

	key := ctx.ArgString(0)
	item := ctx.Arg(1)

	probabilisticMu.Lock()
	cf, exists := cuckooFilters[key]
	probabilisticMu.RUnlock()

	if !exists {
		return ctx.WriteInteger(0)
	}

	if cf.Delete(item) {
		return ctx.WriteInteger(1)
	}
	return ctx.WriteInteger(0)
}

func cmdCFINFO(ctx *Context) error {
	if ctx.ArgCount() < 1 {
		return ctx.WriteError(ErrWrongArgCount)
	}

	key := ctx.ArgString(0)

	probabilisticMu.RLock()
	cf, exists := cuckooFilters[key]
	probabilisticMu.RUnlock()

	if !exists {
		return ctx.WriteError(fmt.Errorf("ERR key not found"))
	}

	info := cf.Info()

	return ctx.WriteArray([]*resp.Value{
		resp.BulkString("size"),
		resp.IntegerValue(int64(info["size"].(uint))),
		resp.BulkString("bucket_size"),
		resp.IntegerValue(int64(info["bucket_size"].(uint))),
		resp.BulkString("count"),
		resp.IntegerValue(int64(info["count"].(uint))),
		resp.BulkString("load_factor"),
		resp.BulkString(fmt.Sprintf("%.4f", info["load_factor"])),
	})
}

func cmdCFRESERVE(ctx *Context) error {
	if ctx.ArgCount() < 2 {
		return ctx.WriteError(ErrWrongArgCount)
	}

	key := ctx.ArgString(0)
	capacity := int(parseInt64(ctx.ArgString(1)))
	bucketSize := uint(2)

	if ctx.ArgCount() >= 3 {
		bucketSize = uint(parseInt64(ctx.ArgString(2)))
	}

	probabilisticMu.Lock()
	cuckooFilters[key] = store.NewCuckooFilter(uint(capacity), bucketSize)
	probabilisticMu.Unlock()

	return ctx.WriteOK()
}

func cmdCMSINCRBY(ctx *Context) error {
	if ctx.ArgCount() < 3 || ctx.ArgCount()%2 == 0 {
		return ctx.WriteError(ErrWrongArgCount)
	}

	key := ctx.ArgString(0)

	probabilisticMu.Lock()
	cms, exists := countMinSketch[key]
	if !exists {
		cms = store.NewCountMinSketch(5, 1000)
		countMinSketch[key] = cms
	}
	probabilisticMu.Unlock()

	results := make([]*resp.Value, 0)
	for i := 1; i < ctx.ArgCount(); i += 2 {
		item := ctx.Arg(i)
		count := uint64(parseInt64(ctx.ArgString(i + 1)))
		result := cms.Add(item, uint(count))
		results = append(results, resp.IntegerValue(int64(result)))
	}

	return ctx.WriteArray(results)
}

func cmdCMSQUERY(ctx *Context) error {
	if ctx.ArgCount() < 2 {
		return ctx.WriteError(ErrWrongArgCount)
	}

	key := ctx.ArgString(0)

	probabilisticMu.RLock()
	cms, exists := countMinSketch[key]
	probabilisticMu.RUnlock()

	if !exists {
		results := make([]*resp.Value, 0, ctx.ArgCount()-1)
		for i := 1; i < ctx.ArgCount(); i++ {
			results = append(results, resp.IntegerValue(0))
		}
		return ctx.WriteArray(results)
	}

	results := make([]*resp.Value, 0, ctx.ArgCount()-1)
	for i := 1; i < ctx.ArgCount(); i++ {
		count := cms.Count(ctx.Arg(i))
		results = append(results, resp.IntegerValue(int64(count)))
	}

	return ctx.WriteArray(results)
}

func cmdCMSINFO(ctx *Context) error {
	if ctx.ArgCount() < 1 {
		return ctx.WriteError(ErrWrongArgCount)
	}

	key := ctx.ArgString(0)

	probabilisticMu.RLock()
	cms, exists := countMinSketch[key]
	probabilisticMu.RUnlock()

	if !exists {
		return ctx.WriteError(fmt.Errorf("ERR key not found"))
	}

	info := cms.Info()

	return ctx.WriteArray([]*resp.Value{
		resp.BulkString("depth"),
		resp.IntegerValue(int64(info["depth"].(uint))),
		resp.BulkString("width"),
		resp.IntegerValue(int64(info["width"].(uint))),
		resp.BulkString("count"),
		resp.IntegerValue(int64(info["count"].(uint64))),
	})
}

func cmdCMSINIT(ctx *Context) error {
	if ctx.ArgCount() < 3 {
		return ctx.WriteError(ErrWrongArgCount)
	}

	key := ctx.ArgString(0)
	depth := int(parseInt64(ctx.ArgString(1)))
	width := int(parseInt64(ctx.ArgString(2)))

	probabilisticMu.Lock()
	countMinSketch[key] = store.NewCountMinSketch(uint(depth), uint(width))
	probabilisticMu.Unlock()

	return ctx.WriteOK()
}

func cmdTOPKADD(ctx *Context) error {
	if ctx.ArgCount() < 2 {
		return ctx.WriteError(ErrWrongArgCount)
	}

	key := ctx.ArgString(0)

	probabilisticMu.Lock()
	tk, exists := topK[key]
	if !exists {
		tk = store.NewTopK(10)
		topK[key] = tk
	}
	probabilisticMu.Unlock()

	results := make([]*resp.Value, 0)
	for i := 1; i < ctx.ArgCount(); i++ {
		item := ctx.ArgString(i)
		count := tk.Add(item, 1)
		results = append(results, resp.BulkString(fmt.Sprintf("%d", count)))
	}

	return ctx.WriteArray(results)
}

func cmdTOPKQUERY(ctx *Context) error {
	if ctx.ArgCount() < 2 {
		return ctx.WriteError(ErrWrongArgCount)
	}

	key := ctx.ArgString(0)

	probabilisticMu.RLock()
	tk, exists := topK[key]
	probabilisticMu.RUnlock()

	if !exists {
		results := make([]*resp.Value, 0, ctx.ArgCount()-1)
		for i := 1; i < ctx.ArgCount(); i++ {
			results = append(results, resp.IntegerValue(0))
		}
		return ctx.WriteArray(results)
	}

	results := make([]*resp.Value, 0, ctx.ArgCount()-1)
	for i := 1; i < ctx.ArgCount(); i++ {
		count := tk.Query(ctx.ArgString(i))
		results = append(results, resp.IntegerValue(int64(count)))
	}

	return ctx.WriteArray(results)
}

func cmdTOPKLIST(ctx *Context) error {
	if ctx.ArgCount() < 1 {
		return ctx.WriteError(ErrWrongArgCount)
	}

	key := ctx.ArgString(0)
	withCount := false

	if ctx.ArgCount() >= 2 {
		withCount = strings.ToUpper(ctx.ArgString(1)) == "WITHCOUNT"
	}

	probabilisticMu.RLock()
	tk, exists := topK[key]
	probabilisticMu.RUnlock()

	if !exists {
		return ctx.WriteArray([]*resp.Value{})
	}

	if withCount {
		items := tk.ListWithCount()
		results := make([]*resp.Value, 0, len(items))
		for _, item := range items {
			results = append(results, resp.ArrayValue([]*resp.Value{
				resp.BulkString(item["item"].(string)),
				resp.IntegerValue(int64(item["count"].(uint64))),
			}))
		}
		return ctx.WriteArray(results)
	}

	items := tk.List()
	results := make([]*resp.Value, 0, len(items))
	for _, item := range items {
		results = append(results, resp.BulkString(item))
	}

	return ctx.WriteArray(results)
}

func cmdTOPKINFO(ctx *Context) error {
	if ctx.ArgCount() < 1 {
		return ctx.WriteError(ErrWrongArgCount)
	}

	key := ctx.ArgString(0)

	probabilisticMu.RLock()
	tk, exists := topK[key]
	probabilisticMu.RUnlock()

	if !exists {
		return ctx.WriteError(fmt.Errorf("ERR key not found"))
	}

	info := tk.Info()

	return ctx.WriteArray([]*resp.Value{
		resp.BulkString("k"),
		resp.IntegerValue(int64(info["k"].(int))),
		resp.BulkString("items"),
		resp.IntegerValue(int64(info["items"].(int))),
		resp.BulkString("total"),
		resp.IntegerValue(int64(info["total"].(uint64))),
	})
}

func cmdTOPKRESERVE(ctx *Context) error {
	if ctx.ArgCount() < 2 {
		return ctx.WriteError(ErrWrongArgCount)
	}

	key := ctx.ArgString(0)
	k := int(parseInt64(ctx.ArgString(1)))

	probabilisticMu.Lock()
	topK[key] = store.NewTopK(k)
	probabilisticMu.Unlock()

	return ctx.WriteOK()
}
