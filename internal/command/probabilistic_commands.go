package command

import (
	"fmt"
	"strconv"
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

// probabilisticMaxBytes bounds the memory one probabilistic structure may
// occupy. The probabilistic structures are held outside the store's own
// value accounting, so the store's MaxValueSize ceiling does not cover them;
// without a bound of their own a single command could ask for an allocation
// the process cannot satisfy.
const probabilisticMaxBytes = 512 * 1024 * 1024

// parseProbabilisticSize parses a size argument for a probabilistic
// structure. The size must be a positive integer.
//
// The shared parseInt64 helper returns 0 for any input it cannot read, so a
// typo'd size was indistinguishable from a legitimate zero — and zero is
// exactly the value these structures cannot survive. A zero-width bloom
// filter divides by its zero modulus in store.BloomFilter.Add, a zero-width
// count-min sketch divides by its zero modulus in store.CountMinSketch.Add,
// and a negative top-k makes store.TopK.List allocate with a negative
// capacity. A negative size was worse still: it became a huge uint and the
// constructor allocated an impossible slice and panicked.
//
// The name is used in the error reply so the client can tell which argument
// was wrong.
func parseProbabilisticSize(arg, name string) (int64, error) {
	size, err := strconv.ParseInt(arg, 10, 64)
	if err != nil {
		return 0, fmt.Errorf("ERR %s is not an integer or out of range", name)
	}
	if size <= 0 {
		return 0, fmt.Errorf("ERR %s must be positive", name)
	}
	return size, nil
}

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
		resp.IntegerValue(int64(mapUint(info, "size"))),
		resp.BulkString("hashes"),
		resp.IntegerValue(int64(mapUint(info, "hashes"))),
		resp.BulkString("count"),
		resp.IntegerValue(int64(mapUint(info, "count"))),
		resp.BulkString("bits_set"),
		resp.IntegerValue(int64(mapInt(info, "bits_set"))),
		resp.BulkString("fill_rate"),
		resp.BulkString(fmt.Sprintf("%.4f", info["fill_rate"])),
	})
}

func cmdBFRESERVE(ctx *Context) error {
	if ctx.ArgCount() < 2 {
		return ctx.WriteError(ErrWrongArgCount)
	}

	key := ctx.ArgString(0)
	capacity, err := parseProbabilisticSize(ctx.ArgString(1), "capacity")
	if err != nil {
		return ctx.WriteError(err)
	}
	// The filter is capacity*10 bits, and a bool bit occupies a byte, so the
	// bit array must stay inside the memory ceiling. Dividing the ceiling
	// BEFORE multiplying also keeps the product from overflowing int64: an
	// overflowed product became a negative int, then a huge uint, and the
	// constructor allocated an impossible slice and panicked.
	if capacity > probabilisticMaxBytes/10 {
		return ctx.WriteError(fmt.Errorf("ERR capacity is too large"))
	}
	errorRate := 0.01

	if ctx.ArgCount() >= 3 {
		errorRate = parseJSONFloat(ctx.ArgString(2))
	}

	// Build the filter BEFORE taking the lock. The constructor can panic on
	// an impossible allocation, and a panic inside the locked section
	// strands the global mutex: every later probabilistic command in the
	// server then blocks forever.
	bf := store.NewBloomFilter(uint(capacity*10), errorRate)

	probabilisticMu.Lock()
	bloomFilters[key] = bf
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

	probabilisticMu.RLock()
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
	capacity, err := parseProbabilisticSize(ctx.ArgString(1), "capacity")
	if err != nil {
		return ctx.WriteError(err)
	}
	bucketSize := int64(2)

	if ctx.ArgCount() >= 3 {
		size, err := parseProbabilisticSize(ctx.ArgString(2), "bucket size")
		if err != nil {
			return ctx.WriteError(err)
		}
		bucketSize = size
	}
	// The filter is capacity buckets of bucketSize bytes each. Bound the
	// product so the allocation stays inside the memory ceiling, and compute
	// it without overflowing: an overflowed product became a huge uint and
	// the constructor allocated an impossible slice and panicked.
	if bucketSize > probabilisticMaxBytes/capacity {
		return ctx.WriteError(fmt.Errorf("ERR capacity and bucket size are too large"))
	}

	// Build the filter BEFORE taking the lock. The constructor can panic on
	// an impossible allocation, and a panic inside the locked section
	// strands the global mutex: every later probabilistic command in the
	// server then blocks forever.
	cf := store.NewCuckooFilter(uint(capacity), uint(bucketSize))

	probabilisticMu.Lock()
	cuckooFilters[key] = cf
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
	depth, err := parseProbabilisticSize(ctx.ArgString(1), "depth")
	if err != nil {
		return ctx.WriteError(err)
	}
	width, err := parseProbabilisticSize(ctx.ArgString(2), "width")
	if err != nil {
		return ctx.WriteError(err)
	}
	// The sketch is depth*width counters of 8 bytes each. Bound the product
	// so the allocation stays inside the memory ceiling, and compute it
	// without overflowing: an overflowed product would become a huge uint
	// and the constructor would allocate an impossible slice and panic.
	if depth > probabilisticMaxBytes/8/width {
		return ctx.WriteError(fmt.Errorf("ERR depth and width are too large"))
	}

	// Build the sketch BEFORE taking the lock: a panic inside the locked
	// section would strand the global mutex and block every later
	// probabilistic command forever.
	cms := store.NewCountMinSketch(uint(depth), uint(width))

	probabilisticMu.Lock()
	countMinSketch[key] = cms
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
			itemStr, _ := item["item"].(string)
			countVal, _ := item["count"].(uint64)
			results = append(results, resp.ArrayValue([]*resp.Value{
				resp.BulkString(itemStr),
				resp.IntegerValue(int64(countVal)),
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

	kVal, _ := info["k"].(int)
	itemsVal, _ := info["items"].(int)
	totalVal, _ := info["total"].(uint64)

	return ctx.WriteArray([]*resp.Value{
		resp.BulkString("k"),
		resp.IntegerValue(int64(kVal)),
		resp.BulkString("items"),
		resp.IntegerValue(int64(itemsVal)),
		resp.BulkString("total"),
		resp.IntegerValue(int64(totalVal)),
	})
}

func cmdTOPKRESERVE(ctx *Context) error {
	if ctx.ArgCount() < 2 {
		return ctx.WriteError(ErrWrongArgCount)
	}

	key := ctx.ArgString(0)
	k, err := parseProbabilisticSize(ctx.ArgString(1), "topk")
	if err != nil {
		return ctx.WriteError(err)
	}
	// The top-k keeps at most k items, and each retained item costs a map
	// entry plus a string header. Bound k so the retained set stays inside
	// the memory ceiling.
	if k > probabilisticMaxBytes/64 {
		return ctx.WriteError(fmt.Errorf("ERR topk is too large"))
	}

	probabilisticMu.Lock()
	topK[key] = store.NewTopK(int(k))
	probabilisticMu.Unlock()

	return ctx.WriteOK()
}
