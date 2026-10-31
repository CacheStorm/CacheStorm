package command

import (
	"errors"
	"math"
	"strconv"
	"strings"

	"github.com/cachestorm/cachestorm/internal/resp"
	"github.com/cachestorm/cachestorm/internal/store"
)

// The probabilistic structures follow RedisBloom: Bloom and cuckoo filters,
// count-min sketches and top-k lists are keys of their own types, with
// RedisBloom's replies and error texts (some of which carry no ERR prefix).

// probabilisticMaxBytes bounds the memory one structure may take, so a
// single command cannot ask for an allocation the process cannot satisfy.
const probabilisticMaxBytes = 512 * 1024 * 1024

//nolint:staticcheck // RedisBloom's reply texts
var (
	errBloomNotFound      = errors.New("ERR not found")
	errBloomExists        = errors.New("ERR item exists")
	errBloomErrorRate     = errors.New("ERR bad error rate")
	errBloomErrorRange    = errors.New("ERR error rate must be in the range (0.000000, 1.000000)")
	errBloomCapacity      = errors.New("ERR bad capacity")
	errBloomCapacityRange = errors.New("ERR capacity must be in the range [1, 1073741824]")
	errBloomExpansion     = errors.New("ERR bad expansion")
	errBloomExpansionRng  = errors.New("ERR expansion must be in the range [0, 32768]")
	errBloomNonScaling    = errors.New("Nonscaling filters cannot expand")
	errBloomInfoField     = errors.New("Invalid information value")
	errBloomBadCapacity   = errors.New("Bad capacity")
	errBloomBadErrorRate  = errors.New("Bad error rate")
	errBloomBadExpansion  = errors.New("Bad expansion")
	errBloomUnknownArg    = errors.New("Unknown argument received")
	errSketchTooLarge     = errors.New("ERR Insufficient memory to create filter")
	errCuckooNotFound     = errors.New("Not found")
	errCuckooCapacity     = errors.New("Capacity must be in the range [2 * BUCKETSIZE, 1073741824]")
	errCuckooBucketParse  = errors.New("Couldn't parse BUCKETSIZE")
	errCuckooBucketSize   = errors.New("BUCKETSIZE: value must be in the range [1, 255]")
	errCuckooIterParse    = errors.New("Couldn't parse MAXITERATIONS")
	errCuckooIterations   = errors.New("MAXITERATIONS: value must be in the range [1, 65535]")
	errCuckooExpParse     = errors.New("Couldn't parse EXPANSION")
	errCuckooExpansion    = errors.New("EXPANSION: value must be in the range [0, 32768]")
	errCMSNoKey           = errors.New("CMS: key does not exist")
	errCMSExists          = errors.New("CMS: key already exists")
	errCMSWidth           = errors.New("CMS: invalid width")
	errCMSDepth           = errors.New("CMS: invalid depth")
	errCMSOverestimation  = errors.New("CMS: invalid overestimation value")
	errCMSProb            = errors.New("CMS: invalid prob value")
	errCMSNumber          = errors.New("CMS: Cannot parse number")
	errCMSNegative        = errors.New("CMS: Number cannot be negative")
	errCMSKeys            = errors.New("CMS: wrong number of keys")
	errCMSKeysWeights     = errors.New("CMS: wrong number of keys/weights")
	errCMSWeight          = errors.New("CMS: invalid weight value")
	errCMSDims            = errors.New("CMS: width/depth is not equal")
	errTopKNoKey          = errors.New("TopK: key does not exist")
	errTopKExists         = errors.New("TopK: key already exists")
	errTopKK              = errors.New("TopK: invalid k")
	errTopKWidth          = errors.New("TopK: invalid width")
	errTopKDepth          = errors.New("TopK: invalid depth")
	errTopKDecay          = errors.New("TopK: invalid decay value. must be '<= 1' & '> 0'")
	errTopKIncrement      = errors.New("TopK: increment must be an integer greater or equal to 0                            and smaller or equal to 100,000")
	errTopKWithCount      = errors.New("WITHCOUNT keyword expected")
)

const (
	bloomDefaultCapacity  = 100
	bloomDefaultErrorRate = 0.01
	bloomDefaultExpansion = 2
	cuckooDefaultCapacity = 1000
	topkDefaultWidth      = 8
	topkDefaultDepth      = 7
	topkDefaultDecay      = 0.9
)

func RegisterProbabilisticCommands(router *Router) {
	router.Register(&CommandDef{Name: "BF.RESERVE", Handler: cmdBFRESERVE})
	router.Register(&CommandDef{Name: "BF.ADD", Handler: cmdBFADD})
	router.Register(&CommandDef{Name: "BF.MADD", Handler: cmdBFMADD})
	router.Register(&CommandDef{Name: "BF.INSERT", Handler: cmdBFINSERT})
	router.Register(&CommandDef{Name: "BF.EXISTS", Handler: cmdBFEXISTS})
	router.Register(&CommandDef{Name: "BF.MEXISTS", Handler: cmdBFMEXISTS})
	router.Register(&CommandDef{Name: "BF.INFO", Handler: cmdBFINFO})
	router.Register(&CommandDef{Name: "BF.CARD", Handler: cmdBFCARD})

	router.Register(&CommandDef{Name: "CF.RESERVE", Handler: cmdCFRESERVE})
	router.Register(&CommandDef{Name: "CF.ADD", Handler: cmdCFADD})
	router.Register(&CommandDef{Name: "CF.ADDNX", Handler: cmdCFADDNX})
	router.Register(&CommandDef{Name: "CF.INSERT", Handler: cmdCFINSERT})
	router.Register(&CommandDef{Name: "CF.INSERTNX", Handler: cmdCFINSERTNX})
	router.Register(&CommandDef{Name: "CF.EXISTS", Handler: cmdCFEXISTS})
	router.Register(&CommandDef{Name: "CF.MEXISTS", Handler: cmdCFMEXISTS})
	router.Register(&CommandDef{Name: "CF.COUNT", Handler: cmdCFCOUNT})
	router.Register(&CommandDef{Name: "CF.DEL", Handler: cmdCFDEL})
	router.Register(&CommandDef{Name: "CF.INFO", Handler: cmdCFINFO})

	router.Register(&CommandDef{Name: "CMS.INITBYDIM", Handler: cmdCMSINITBYDIM})
	router.Register(&CommandDef{Name: "CMS.INITBYPROB", Handler: cmdCMSINITBYPROB})
	router.Register(&CommandDef{Name: "CMS.INCRBY", Handler: cmdCMSINCRBY})
	router.Register(&CommandDef{Name: "CMS.QUERY", Handler: cmdCMSQUERY})
	router.Register(&CommandDef{Name: "CMS.MERGE", Handler: cmdCMSMERGE})
	router.Register(&CommandDef{Name: "CMS.INFO", Handler: cmdCMSINFO})

	router.Register(&CommandDef{Name: "TOPK.RESERVE", Handler: cmdTOPKRESERVE})
	router.Register(&CommandDef{Name: "TOPK.ADD", Handler: cmdTOPKADD})
	router.Register(&CommandDef{Name: "TOPK.INCRBY", Handler: cmdTOPKINCRBY})
	router.Register(&CommandDef{Name: "TOPK.QUERY", Handler: cmdTOPKQUERY})
	router.Register(&CommandDef{Name: "TOPK.COUNT", Handler: cmdTOPKCOUNT})
	router.Register(&CommandDef{Name: "TOPK.LIST", Handler: cmdTOPKLIST})
	router.Register(&CommandDef{Name: "TOPK.INFO", Handler: cmdTOPKINFO})
}

// lookupValue returns the value of type T at key; ok is false when the key
// is absent, and a key of another type is ErrWrongType.
func lookupValue[T store.Value](ctx *Context, key string) (T, bool, error) {
	var zero T
	entry, exists := ctx.Store.Get(key)
	if !exists {
		return zero, false, nil
	}
	v, ok := entry.Value.(T)
	if !ok {
		return zero, false, store.ErrWrongType
	}
	return v, true, nil
}

func parseFloatArg(b []byte) (float64, bool) {
	v, err := strconv.ParseFloat(string(b), 64)
	return v, err == nil && !math.IsNaN(v)
}

func parseIntArg(b []byte) (int64, bool) {
	v, err := strconv.ParseInt(string(b), 10, 64)
	return v, err == nil
}

// newBloom checks a filter's parameters and builds it.
func newBloom(capacity int64, errorRate float64, expansion int64) (*store.BloomValue, error) {
	if !(errorRate > 0 && errorRate < 1) {
		return nil, errBloomErrorRange
	}
	if capacity < 1 || capacity > 1<<30 {
		return nil, errBloomCapacityRange
	}
	if expansion < 0 || expansion > 32768 {
		return nil, errBloomExpansionRng
	}
	bits := float64(capacity) * -math.Log(errorRate) / (math.Ln2 * math.Ln2)
	if bits/8 > probabilisticMaxBytes {
		return nil, errSketchTooLarge
	}
	return store.NewBloomValue(uint64(capacity), errorRate, uint32(expansion)), nil
}

func cmdBFRESERVE(ctx *Context) error {
	if ctx.ArgCount() < 3 {
		return ctx.WriteError(arityError(ctx.Command))
	}
	key := ctx.ArgString(0)
	errorRate, ok := parseFloatArg(ctx.Arg(1))
	if !ok {
		return ctx.WriteError(errBloomErrorRate)
	}
	capacity, ok := parseIntArg(ctx.Arg(2))
	if !ok {
		return ctx.WriteError(errBloomCapacity)
	}
	expansion, nonScaling := int64(bloomDefaultExpansion), false
	for i := 3; i < ctx.ArgCount(); i++ {
		switch strings.ToUpper(ctx.ArgString(i)) {
		case "NONSCALING":
			nonScaling = true
		case "EXPANSION":
			if i+1 >= ctx.ArgCount() {
				return ctx.WriteError(arityError(ctx.Command))
			}
			i++
			if expansion, ok = parseIntArg(ctx.Arg(i)); !ok {
				return ctx.WriteError(errBloomExpansion)
			}
		}
	}
	if nonScaling {
		if hasArg(ctx, 3, "EXPANSION") {
			return ctx.WriteError(errBloomNonScaling)
		}
		expansion = 0
	}
	if _, exists, err := lookupValue[*store.BloomValue](ctx, key); err != nil {
		return ctx.WriteError(err)
	} else if exists {
		return ctx.WriteError(errBloomExists)
	}
	bf, err := newBloom(capacity, errorRate, expansion)
	if err != nil {
		return ctx.WriteError(err)
	}
	if err := ctx.Store.Set(key, bf, store.SetOptions{}); err != nil {
		return ctx.WriteError(err)
	}
	return ctx.WriteOK()
}

func hasArg(ctx *Context, from int, word string) bool {
	for i := from; i < ctx.ArgCount(); i++ {
		if strings.EqualFold(ctx.ArgString(i), word) {
			return true
		}
	}
	return false
}

// bloomAdd adds items to the filter at key, creating it with create's
// parameters when absent (nil create refuses a missing key). It replies with
// one integer per item, ending at the first error, or a single integer.
func bloomAdd(ctx *Context, key string, items [][]byte, create func() (*store.BloomValue, error), single bool) error {
	bf, exists, err := lookupValue[*store.BloomValue](ctx, key)
	if err != nil {
		return ctx.WriteError(err)
	}
	if !exists {
		if create == nil {
			return ctx.WriteError(errBloomNotFound)
		}
		if bf, err = create(); err != nil {
			return ctx.WriteError(err)
		}
		if err := ctx.Store.Set(key, bf, store.SetOptions{}); err != nil {
			return ctx.WriteError(err)
		}
	}
	replies := make([]*resp.Value, 0, len(items))
	for _, item := range items {
		added, err := bf.Add(item)
		if err != nil {
			replies = append(replies, resp.ErrorValue(err.Error()))
			break
		}
		replies = append(replies, resp.IntegerValue(boolInt64(added)))
	}
	valueWritten(ctx, key)
	if single {
		return ctx.WriteValue(replies[0])
	}
	return ctx.WriteArray(replies)
}

func boolInt64(b bool) int64 {
	if b {
		return 1
	}
	return 0
}

func defaultBloom() (*store.BloomValue, error) {
	return newBloom(bloomDefaultCapacity, bloomDefaultErrorRate, bloomDefaultExpansion)
}

func cmdBFADD(ctx *Context) error {
	if ctx.ArgCount() != 2 {
		return ctx.WriteError(arityError(ctx.Command))
	}
	return bloomAdd(ctx, ctx.ArgString(0), ctx.Args[1:], defaultBloom, true)
}

func cmdBFMADD(ctx *Context) error {
	if ctx.ArgCount() < 2 {
		return ctx.WriteError(arityError(ctx.Command))
	}
	return bloomAdd(ctx, ctx.ArgString(0), ctx.Args[1:], defaultBloom, false)
}

func cmdBFINSERT(ctx *Context) error {
	if ctx.ArgCount() < 3 {
		return ctx.WriteError(arityError(ctx.Command))
	}
	capacity, errorRate, expansion := int64(bloomDefaultCapacity), bloomDefaultErrorRate, int64(bloomDefaultExpansion)
	noCreate, nonScaling := false, false
	var items [][]byte
	for i := 1; i < ctx.ArgCount() && items == nil; i++ {
		var ok bool
		switch strings.ToUpper(ctx.ArgString(i)) {
		case "CAPACITY":
			if i++; i >= ctx.ArgCount() {
				return ctx.WriteError(arityError(ctx.Command))
			}
			if capacity, ok = parseIntArg(ctx.Arg(i)); !ok {
				return ctx.WriteError(errBloomBadCapacity)
			}
		case "ERROR":
			if i++; i >= ctx.ArgCount() {
				return ctx.WriteError(arityError(ctx.Command))
			}
			if errorRate, ok = parseFloatArg(ctx.Arg(i)); !ok {
				return ctx.WriteError(errBloomBadErrorRate)
			}
		case "EXPANSION":
			if i++; i >= ctx.ArgCount() {
				return ctx.WriteError(arityError(ctx.Command))
			}
			if expansion, ok = parseIntArg(ctx.Arg(i)); !ok {
				return ctx.WriteError(errBloomBadExpansion)
			}
		case "NOCREATE":
			noCreate = true
		case "NONSCALING":
			nonScaling = true
		case "ITEMS":
			if i+1 >= ctx.ArgCount() {
				return ctx.WriteError(arityError(ctx.Command))
			}
			items = ctx.Args[i+1:]
		default:
			return ctx.WriteError(errBloomUnknownArg)
		}
	}
	if items == nil {
		return ctx.WriteError(arityError(ctx.Command))
	}
	if nonScaling {
		expansion = 0
	}
	var create func() (*store.BloomValue, error)
	if !noCreate {
		create = func() (*store.BloomValue, error) { return newBloom(capacity, errorRate, expansion) }
	}
	return bloomAdd(ctx, ctx.ArgString(0), items, create, false)
}

// bloomExists answers BF.EXISTS and BF.MEXISTS: a missing key, or one of
// another type, holds no item, as in RedisBloom.
func bloomExists(ctx *Context, items [][]byte, single bool) error {
	bf, exists, err := lookupValue[*store.BloomValue](ctx, ctx.ArgString(0))
	exists = exists && err == nil
	replies := make([]*resp.Value, len(items))
	for i, item := range items {
		replies[i] = resp.IntegerValue(boolInt64(exists && bf.Exists(item)))
	}
	if single {
		return ctx.WriteValue(replies[0])
	}
	return ctx.WriteArray(replies)
}

func cmdBFEXISTS(ctx *Context) error {
	if ctx.ArgCount() != 2 {
		return ctx.WriteError(arityError(ctx.Command))
	}
	return bloomExists(ctx, ctx.Args[1:], true)
}

func cmdBFMEXISTS(ctx *Context) error {
	if ctx.ArgCount() < 2 {
		return ctx.WriteError(arityError(ctx.Command))
	}
	return bloomExists(ctx, ctx.Args[1:], false)
}

func cmdBFINFO(ctx *Context) error {
	if ctx.ArgCount() < 1 || ctx.ArgCount() > 2 {
		return ctx.WriteError(arityError(ctx.Command))
	}
	bf, exists, err := lookupValue[*store.BloomValue](ctx, ctx.ArgString(0))
	if err != nil {
		return ctx.WriteError(err)
	}
	if !exists {
		return ctx.WriteError(errBloomNotFound)
	}
	info := bf.Info()
	expansion := resp.NullBulkString()
	if info.Expansion > 0 {
		expansion = resp.IntegerValue(int64(info.Expansion))
	}
	fields := []struct {
		name, label string
		value       *resp.Value
	}{
		{"CAPACITY", "Capacity", resp.IntegerValue(int64(info.Capacity))},
		{"SIZE", "Size", resp.IntegerValue(int64(info.Size))},
		{"FILTERS", "Number of filters", resp.IntegerValue(int64(info.Filters))},
		{"ITEMS", "Number of items inserted", resp.IntegerValue(int64(info.Items))},
		{"EXPANSION", "Expansion rate", expansion},
	}
	if ctx.ArgCount() == 2 {
		want := strings.ToUpper(ctx.ArgString(1))
		for _, f := range fields {
			if f.name == want {
				return ctx.WriteArray([]*resp.Value{f.value})
			}
		}
		return ctx.WriteError(errBloomInfoField)
	}
	reply := make([]*resp.Value, 0, 2*len(fields))
	for _, f := range fields {
		reply = append(reply, resp.SimpleString(f.label), f.value)
	}
	return ctx.WriteArray(reply)
}

func cmdBFCARD(ctx *Context) error {
	if ctx.ArgCount() != 1 {
		return ctx.WriteError(arityError(ctx.Command))
	}
	bf, exists, err := lookupValue[*store.BloomValue](ctx, ctx.ArgString(0))
	if err != nil {
		return ctx.WriteError(err)
	}
	if !exists {
		return ctx.WriteInteger(0)
	}
	return ctx.WriteInteger(int64(bf.Info().Items))
}

// newCuckoo checks a cuckoo filter's parameters and builds it.
func newCuckoo(capacity, bucketSize, maxIterations, expansion int64) (*store.CuckooValue, error) {
	if bucketSize < 1 || bucketSize > 255 {
		return nil, errCuckooBucketSize
	}
	if capacity < 2*bucketSize || capacity > 1<<30 {
		return nil, errCuckooCapacity
	}
	if maxIterations < 1 || maxIterations > 65535 {
		return nil, errCuckooIterations
	}
	if expansion < 0 || expansion > 32768 {
		return nil, errCuckooExpansion
	}
	if store.CuckooBuckets(uint64(capacity), uint8(bucketSize))*uint64(bucketSize) > probabilisticMaxBytes {
		return nil, errSketchTooLarge
	}
	return store.NewCuckooValue(uint64(capacity), uint8(bucketSize), uint16(maxIterations), uint16(expansion)), nil
}

func defaultCuckoo(capacity int64) func() (*store.CuckooValue, error) {
	return func() (*store.CuckooValue, error) { return newCuckoo(capacity, 2, 20, 1) }
}

func cmdCFRESERVE(ctx *Context) error {
	if ctx.ArgCount() < 2 || ctx.ArgCount()%2 != 0 {
		return ctx.WriteError(arityError(ctx.Command))
	}
	key := ctx.ArgString(0)
	capacity, ok := parseIntArg(ctx.Arg(1))
	if !ok {
		return ctx.WriteError(errBloomBadCapacity)
	}
	bucketSize, maxIterations, expansion := int64(2), int64(20), int64(1)
	for i := 2; i < ctx.ArgCount(); i += 2 {
		v, ok := parseIntArg(ctx.Arg(i + 1))
		switch strings.ToUpper(ctx.ArgString(i)) {
		case "BUCKETSIZE":
			if !ok {
				return ctx.WriteError(errCuckooBucketParse)
			}
			bucketSize = v
		case "MAXITERATIONS":
			if !ok {
				return ctx.WriteError(errCuckooIterParse)
			}
			maxIterations = v
		case "EXPANSION":
			if !ok {
				return ctx.WriteError(errCuckooExpParse)
			}
			expansion = v
		default:
			return ctx.WriteError(arityError(ctx.Command))
		}
	}
	if _, exists, err := lookupValue[*store.CuckooValue](ctx, key); err != nil {
		return ctx.WriteError(err)
	} else if exists {
		return ctx.WriteError(errBloomExists)
	}
	cf, err := newCuckoo(capacity, bucketSize, maxIterations, expansion)
	if err != nil {
		return ctx.WriteError(err)
	}
	if err := ctx.Store.Set(key, cf, store.SetOptions{}); err != nil {
		return ctx.WriteError(err)
	}
	return ctx.WriteOK()
}

// cuckooAdd adds items like bloomAdd; nx adds only absent items.
func cuckooAdd(ctx *Context, key string, items [][]byte, create func() (*store.CuckooValue, error), nx, single bool) error {
	cf, exists, err := lookupValue[*store.CuckooValue](ctx, key)
	if err != nil {
		return ctx.WriteError(err)
	}
	if !exists {
		if create == nil {
			return ctx.WriteError(errBloomNotFound)
		}
		if cf, err = create(); err != nil {
			return ctx.WriteError(err)
		}
		if err := ctx.Store.Set(key, cf, store.SetOptions{}); err != nil {
			return ctx.WriteError(err)
		}
	}
	replies := make([]*resp.Value, 0, len(items))
	for _, item := range items {
		added, err := cf.Add(item, nx)
		if err != nil {
			replies = append(replies, resp.ErrorValue(err.Error()))
			continue
		}
		replies = append(replies, resp.IntegerValue(boolInt64(added)))
	}
	valueWritten(ctx, key)
	if single {
		return ctx.WriteValue(replies[0])
	}
	return ctx.WriteArray(replies)
}

func cmdCFADD(ctx *Context) error {
	if ctx.ArgCount() != 2 {
		return ctx.WriteError(arityError(ctx.Command))
	}
	return cuckooAdd(ctx, ctx.ArgString(0), ctx.Args[1:], defaultCuckoo(cuckooDefaultCapacity), false, true)
}

func cmdCFADDNX(ctx *Context) error {
	if ctx.ArgCount() != 2 {
		return ctx.WriteError(arityError(ctx.Command))
	}
	return cuckooAdd(ctx, ctx.ArgString(0), ctx.Args[1:], defaultCuckoo(cuckooDefaultCapacity), true, true)
}

func cuckooInsert(ctx *Context, nx bool) error {
	if ctx.ArgCount() < 3 {
		return ctx.WriteError(arityError(ctx.Command))
	}
	capacity, noCreate := int64(cuckooDefaultCapacity), false
	var items [][]byte
	for i := 1; i < ctx.ArgCount() && items == nil; i++ {
		switch strings.ToUpper(ctx.ArgString(i)) {
		case "CAPACITY":
			if i++; i >= ctx.ArgCount() {
				return ctx.WriteError(arityError(ctx.Command))
			}
			var ok bool
			if capacity, ok = parseIntArg(ctx.Arg(i)); !ok {
				return ctx.WriteError(errBloomBadCapacity)
			}
		case "NOCREATE":
			noCreate = true
		case "ITEMS":
			if i+1 >= ctx.ArgCount() {
				return ctx.WriteError(arityError(ctx.Command))
			}
			items = ctx.Args[i+1:]
		default:
			return ctx.WriteError(errBloomUnknownArg)
		}
	}
	if items == nil {
		return ctx.WriteError(arityError(ctx.Command))
	}
	create := defaultCuckoo(capacity)
	if noCreate {
		create = nil
	}
	return cuckooAdd(ctx, ctx.ArgString(0), items, create, nx, false)
}

func cmdCFINSERT(ctx *Context) error   { return cuckooInsert(ctx, false) }
func cmdCFINSERTNX(ctx *Context) error { return cuckooInsert(ctx, true) }

// cuckooCounts answers CF.EXISTS, CF.MEXISTS and CF.COUNT: a missing key,
// or one of another type, holds no item, as in RedisBloom.
func cuckooCounts(ctx *Context, exists bool, single bool) error {
	cf, found, err := lookupValue[*store.CuckooValue](ctx, ctx.ArgString(0))
	found = found && err == nil
	items := ctx.Args[1:]
	replies := make([]*resp.Value, len(items))
	for i, item := range items {
		var n uint64
		if found {
			n = cf.Count(item)
		}
		if exists {
			n = min(n, 1)
		}
		replies[i] = resp.IntegerValue(int64(n))
	}
	if single {
		return ctx.WriteValue(replies[0])
	}
	return ctx.WriteArray(replies)
}

func cmdCFEXISTS(ctx *Context) error {
	if ctx.ArgCount() != 2 {
		return ctx.WriteError(arityError(ctx.Command))
	}
	return cuckooCounts(ctx, true, true)
}

func cmdCFMEXISTS(ctx *Context) error {
	if ctx.ArgCount() < 2 {
		return ctx.WriteError(arityError(ctx.Command))
	}
	return cuckooCounts(ctx, true, false)
}

func cmdCFCOUNT(ctx *Context) error {
	if ctx.ArgCount() != 2 {
		return ctx.WriteError(arityError(ctx.Command))
	}
	return cuckooCounts(ctx, false, true)
}

func cmdCFDEL(ctx *Context) error {
	if ctx.ArgCount() != 2 {
		return ctx.WriteError(arityError(ctx.Command))
	}
	key := ctx.ArgString(0)
	cf, exists, err := lookupValue[*store.CuckooValue](ctx, key)
	if err != nil {
		return ctx.WriteError(err)
	}
	if !exists {
		return ctx.WriteError(errCuckooNotFound)
	}
	deleted := cf.Delete(ctx.Arg(1))
	if deleted {
		valueWritten(ctx, key)
	}
	return ctx.WriteInteger(boolInt64(deleted))
}

func cmdCFINFO(ctx *Context) error {
	if ctx.ArgCount() != 1 {
		return ctx.WriteError(arityError(ctx.Command))
	}
	cf, exists, err := lookupValue[*store.CuckooValue](ctx, ctx.ArgString(0))
	if err != nil {
		return ctx.WriteError(err)
	}
	if !exists {
		return ctx.WriteError(errBloomNotFound)
	}
	info := cf.Info()
	return ctx.WriteArray([]*resp.Value{
		resp.SimpleString("Size"), resp.IntegerValue(int64(info.Size)),
		resp.SimpleString("Number of buckets"), resp.IntegerValue(int64(info.Buckets)),
		resp.SimpleString("Number of filters"), resp.IntegerValue(int64(info.Filters)),
		resp.SimpleString("Number of items inserted"), resp.IntegerValue(int64(info.Inserted)),
		resp.SimpleString("Number of items deleted"), resp.IntegerValue(int64(info.Deleted)),
		resp.SimpleString("Bucket size"), resp.IntegerValue(int64(info.BucketSize)),
		resp.SimpleString("Expansion rate"), resp.IntegerValue(int64(info.Expansion)),
		resp.SimpleString("Max iterations"), resp.IntegerValue(int64(info.MaxIterations)),
	})
}

// createCMS stores a new sketch at key unless the key exists, of any type.
func createCMS(ctx *Context, key string, width, depth int64) error {
	if width*depth*4 > probabilisticMaxBytes {
		return ctx.WriteError(errSketchTooLarge)
	}
	if ctx.Store.Exists(key) {
		return ctx.WriteError(errCMSExists)
	}
	if err := ctx.Store.Set(key, store.NewCMSValue(uint32(width), uint32(depth)), store.SetOptions{}); err != nil {
		return ctx.WriteError(err)
	}
	return ctx.WriteOK()
}

func cmdCMSINITBYDIM(ctx *Context) error {
	if ctx.ArgCount() != 3 {
		return ctx.WriteError(arityError(ctx.Command))
	}
	width, ok := parseIntArg(ctx.Arg(1))
	if !ok || width < 1 || width > math.MaxUint32 {
		return ctx.WriteError(errCMSWidth)
	}
	depth, ok := parseIntArg(ctx.Arg(2))
	if !ok || depth < 1 || depth > math.MaxUint32 {
		return ctx.WriteError(errCMSDepth)
	}
	return createCMS(ctx, ctx.ArgString(0), width, depth)
}

func cmdCMSINITBYPROB(ctx *Context) error {
	if ctx.ArgCount() != 3 {
		return ctx.WriteError(arityError(ctx.Command))
	}
	overestimation, ok := parseFloatArg(ctx.Arg(1))
	if !ok || overestimation <= 0 || overestimation >= 1 {
		return ctx.WriteError(errCMSOverestimation)
	}
	prob, ok := parseFloatArg(ctx.Arg(2))
	if !ok || prob <= 0 || prob >= 1 {
		return ctx.WriteError(errCMSProb)
	}
	width := int64(math.Ceil(2 / overestimation))
	depth := int64(math.Ceil(math.Log(prob) / math.Log(0.5)))
	return createCMS(ctx, ctx.ArgString(0), width, depth)
}

func getCMS(ctx *Context, key string) (*store.CMSValue, error) {
	m, exists, err := lookupValue[*store.CMSValue](ctx, key)
	if err != nil {
		return nil, err
	}
	if !exists {
		return nil, errCMSNoKey
	}
	return m, nil
}

func cmdCMSINCRBY(ctx *Context) error {
	if ctx.ArgCount() < 3 || ctx.ArgCount()%2 == 0 {
		return ctx.WriteError(arityError(ctx.Command))
	}
	key := ctx.ArgString(0)
	m, err := getCMS(ctx, key)
	if err != nil {
		return ctx.WriteError(err)
	}
	incrs := make([]uint64, 0, ctx.ArgCount()/2)
	for i := 2; i < ctx.ArgCount(); i += 2 {
		n, err := strconv.ParseInt(ctx.ArgString(i), 10, 64)
		if err != nil {
			return ctx.WriteError(errCMSNumber)
		}
		if n < 0 {
			return ctx.WriteError(errCMSNegative)
		}
		incrs = append(incrs, uint64(n))
	}
	replies := make([]*resp.Value, len(incrs))
	for i, incr := range incrs {
		count, err := m.IncrBy(ctx.Arg(1+2*i), incr)
		if err != nil {
			replies[i] = resp.ErrorValue(err.Error())
			continue
		}
		replies[i] = resp.IntegerValue(int64(count))
	}
	valueWritten(ctx, key)
	return ctx.WriteArray(replies)
}

func cmdCMSQUERY(ctx *Context) error {
	if ctx.ArgCount() < 2 {
		return ctx.WriteError(arityError(ctx.Command))
	}
	m, err := getCMS(ctx, ctx.ArgString(0))
	if err != nil {
		return ctx.WriteError(err)
	}
	replies := make([]*resp.Value, ctx.ArgCount()-1)
	for i := range replies {
		replies[i] = resp.IntegerValue(int64(m.Query(ctx.Arg(1 + i))))
	}
	return ctx.WriteArray(replies)
}

func cmdCMSMERGE(ctx *Context) error {
	if ctx.ArgCount() < 3 {
		return ctx.WriteError(arityError(ctx.Command))
	}
	numKeys, err := strconv.Atoi(ctx.ArgString(1))
	if err != nil || numKeys < 1 {
		return ctx.WriteError(errCMSNoKey)
	}
	rest := ctx.ArgCount() - 2
	if numKeys > rest {
		return ctx.WriteError(errCMSKeys)
	}
	weights := make([]int64, numKeys)
	for i := range weights {
		weights[i] = 1
	}
	if extra := rest - numKeys; extra > 0 {
		if !strings.EqualFold(ctx.ArgString(2+numKeys), "WEIGHTS") {
			return ctx.WriteError(errCMSKeys)
		}
		if extra-1 != numKeys {
			return ctx.WriteError(errCMSKeysWeights)
		}
		for i := range weights {
			w, err := strconv.ParseInt(ctx.ArgString(3+numKeys+i), 10, 64)
			if err != nil {
				return ctx.WriteError(errCMSWeight)
			}
			weights[i] = w
		}
	}
	destKey := ctx.ArgString(0)
	dest, err := getCMS(ctx, destKey)
	if err != nil {
		return ctx.WriteError(err)
	}
	srcs := make([]*store.CMSValue, numKeys)
	for i := range srcs {
		if srcs[i], err = getCMS(ctx, ctx.ArgString(2+i)); err != nil {
			return ctx.WriteError(err)
		}
		if srcs[i].Width != dest.Width || srcs[i].Depth != dest.Depth {
			return ctx.WriteError(errCMSDims)
		}
	}
	dest.Merge(srcs, weights)
	valueWritten(ctx, destKey)
	return ctx.WriteOK()
}

func cmdCMSINFO(ctx *Context) error {
	if ctx.ArgCount() != 1 {
		return ctx.WriteError(arityError(ctx.Command))
	}
	m, err := getCMS(ctx, ctx.ArgString(0))
	if err != nil {
		return ctx.WriteError(err)
	}
	return ctx.WriteArray([]*resp.Value{
		resp.SimpleString("width"), resp.IntegerValue(int64(m.Width)),
		resp.SimpleString("depth"), resp.IntegerValue(int64(m.Depth)),
		resp.SimpleString("count"), resp.IntegerValue(int64(m.Total())),
	})
}

func cmdTOPKRESERVE(ctx *Context) error {
	if ctx.ArgCount() != 2 && ctx.ArgCount() != 5 {
		return ctx.WriteError(arityError(ctx.Command))
	}
	key := ctx.ArgString(0)
	k, ok := parseIntArg(ctx.Arg(1))
	if !ok || k < 1 || k > math.MaxUint32 {
		return ctx.WriteError(errTopKK)
	}
	width, depth, decay := int64(topkDefaultWidth), int64(topkDefaultDepth), topkDefaultDecay
	if ctx.ArgCount() == 5 {
		if width, ok = parseIntArg(ctx.Arg(2)); !ok || width < 1 || width > math.MaxUint32 {
			return ctx.WriteError(errTopKWidth)
		}
		if depth, ok = parseIntArg(ctx.Arg(3)); !ok || depth < 1 || depth > math.MaxUint32 {
			return ctx.WriteError(errTopKDepth)
		}
		if decay, ok = parseFloatArg(ctx.Arg(4)); !ok || decay <= 0 || decay > 1 {
			return ctx.WriteError(errTopKDecay)
		}
	}
	if width*depth*8 > probabilisticMaxBytes {
		return ctx.WriteError(errSketchTooLarge)
	}
	if ctx.Store.Exists(key) {
		return ctx.WriteError(errTopKExists)
	}
	if err := ctx.Store.Set(key, store.NewTopKValue(uint32(k), uint32(width), uint32(depth), decay), store.SetOptions{}); err != nil {
		return ctx.WriteError(err)
	}
	return ctx.WriteOK()
}

func getTopK(ctx *Context, key string) (*store.TopKValue, error) {
	t, exists, err := lookupValue[*store.TopKValue](ctx, key)
	if err != nil {
		return nil, err
	}
	if !exists {
		return nil, errTopKNoKey
	}
	return t, nil
}

func droppedReply(dropped string, ok bool) *resp.Value {
	if !ok {
		return resp.NullBulkString()
	}
	return resp.BulkString(dropped)
}

func cmdTOPKADD(ctx *Context) error {
	if ctx.ArgCount() < 2 {
		return ctx.WriteError(arityError(ctx.Command))
	}
	key := ctx.ArgString(0)
	t, err := getTopK(ctx, key)
	if err != nil {
		return ctx.WriteError(err)
	}
	replies := make([]*resp.Value, ctx.ArgCount()-1)
	for i := range replies {
		replies[i] = droppedReply(t.Add(ctx.Arg(1+i), 1))
	}
	valueWritten(ctx, key)
	return ctx.WriteArray(replies)
}

func cmdTOPKINCRBY(ctx *Context) error {
	if ctx.ArgCount() < 3 || ctx.ArgCount()%2 == 0 {
		return ctx.WriteError(arityError(ctx.Command))
	}
	key := ctx.ArgString(0)
	t, err := getTopK(ctx, key)
	if err != nil {
		return ctx.WriteError(err)
	}
	replies := make([]*resp.Value, 0, ctx.ArgCount()/2)
	for i := 1; i < ctx.ArgCount(); i += 2 {
		incr, ok := parseIntArg(ctx.Arg(i + 1))
		if !ok || incr < 0 || incr > 100000 {
			replies = append(replies, resp.ErrorValue(errTopKIncrement.Error()))
			break
		}
		replies = append(replies, droppedReply(t.Add(ctx.Arg(i), uint32(incr))))
	}
	valueWritten(ctx, key)
	return ctx.WriteArray(replies)
}

func cmdTOPKQUERY(ctx *Context) error {
	if ctx.ArgCount() < 2 {
		return ctx.WriteError(arityError(ctx.Command))
	}
	t, err := getTopK(ctx, ctx.ArgString(0))
	if err != nil {
		return ctx.WriteError(err)
	}
	replies := make([]*resp.Value, ctx.ArgCount()-1)
	for i := range replies {
		replies[i] = resp.IntegerValue(boolInt64(t.Query(ctx.Arg(1 + i))))
	}
	return ctx.WriteArray(replies)
}

func cmdTOPKCOUNT(ctx *Context) error {
	if ctx.ArgCount() < 2 {
		return ctx.WriteError(arityError(ctx.Command))
	}
	t, err := getTopK(ctx, ctx.ArgString(0))
	if err != nil {
		return ctx.WriteError(err)
	}
	replies := make([]*resp.Value, ctx.ArgCount()-1)
	for i := range replies {
		replies[i] = resp.IntegerValue(int64(t.Count(ctx.Arg(1 + i))))
	}
	return ctx.WriteArray(replies)
}

func cmdTOPKLIST(ctx *Context) error {
	if ctx.ArgCount() < 1 || ctx.ArgCount() > 2 {
		return ctx.WriteError(arityError(ctx.Command))
	}
	withCount := ctx.ArgCount() == 2
	if withCount && !strings.EqualFold(ctx.ArgString(1), "WITHCOUNT") {
		return ctx.WriteError(errTopKWithCount)
	}
	t, err := getTopK(ctx, ctx.ArgString(0))
	if err != nil {
		return ctx.WriteError(err)
	}
	var replies []*resp.Value
	for _, it := range t.List() {
		replies = append(replies, resp.BulkString(it.Item))
		if withCount {
			replies = append(replies, resp.IntegerValue(int64(it.Count)))
		}
	}
	return ctx.WriteArray(replies)
}

func cmdTOPKINFO(ctx *Context) error {
	if ctx.ArgCount() != 1 {
		return ctx.WriteError(arityError(ctx.Command))
	}
	t, err := getTopK(ctx, ctx.ArgString(0))
	if err != nil {
		return ctx.WriteError(err)
	}
	return ctx.WriteArray([]*resp.Value{
		resp.SimpleString("k"), resp.IntegerValue(int64(t.K)),
		resp.SimpleString("width"), resp.IntegerValue(int64(t.Width)),
		resp.SimpleString("depth"), resp.IntegerValue(int64(t.Depth)),
		resp.SimpleString("decay"), resp.BulkString(strconv.FormatFloat(t.Decay, 'g', -1, 64)),
	})
}
