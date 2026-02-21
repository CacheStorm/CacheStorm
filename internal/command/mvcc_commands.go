package command

import (
	"fmt"
	"sync"
	"time"

	"github.com/cachestorm/cachestorm/internal/resp"
	"github.com/cachestorm/cachestorm/internal/store"
)

func RegisterMVCCCommands(router *Router) {
	router.Register(&CommandDef{Name: "MVCC.BEGIN", Handler: cmdMVCCBEGIN})
	router.Register(&CommandDef{Name: "MVCC.COMMIT", Handler: cmdMVCCCOMMIT})
	router.Register(&CommandDef{Name: "MVCC.ROLLBACK", Handler: cmdMVCCROLLBACK})
	router.Register(&CommandDef{Name: "MVCC.GET", Handler: cmdMVCCGET})
	router.Register(&CommandDef{Name: "MVCC.SET", Handler: cmdMVCCSET})
	router.Register(&CommandDef{Name: "MVCC.DELETE", Handler: cmdMVCCDELETE})
	router.Register(&CommandDef{Name: "MVCC.STATUS", Handler: cmdMVCCSTATUS})
	router.Register(&CommandDef{Name: "MVCC.SNAPSHOT", Handler: cmdMVCCSNAPSHOT})

	router.Register(&CommandDef{Name: "SPATIAL.CREATE", Handler: cmdSPATIALCREATE})
	router.Register(&CommandDef{Name: "SPATIAL.ADD", Handler: cmdSPATIALADD})
	router.Register(&CommandDef{Name: "SPATIAL.NEARBY", Handler: cmdSPATIALNEARBY})
	router.Register(&CommandDef{Name: "SPATIAL.WITHIN", Handler: cmdSPATIALWITHIN})
	router.Register(&CommandDef{Name: "SPATIAL.DELETE", Handler: cmdSPATIALDELETE})
	router.Register(&CommandDef{Name: "SPATIAL.LIST", Handler: cmdSPATIALLIST})

	router.Register(&CommandDef{Name: "CHAIN.CREATE", Handler: cmdCHAINCREATE})
	router.Register(&CommandDef{Name: "CHAIN.ADD", Handler: cmdCHAINADD})
	router.Register(&CommandDef{Name: "CHAIN.GET", Handler: cmdCHAINGET})
	router.Register(&CommandDef{Name: "CHAIN.VALIDATE", Handler: cmdCHAINVALIDATE})
	router.Register(&CommandDef{Name: "CHAIN.LENGTH", Handler: cmdCHAINLENGTH})
	router.Register(&CommandDef{Name: "CHAIN.LAST", Handler: cmdCHAINLAST})

	router.Register(&CommandDef{Name: "ANALYTICS.INCR", Handler: cmdANALYTICSINCR})
	router.Register(&CommandDef{Name: "ANALYTICS.DECR", Handler: cmdANALYTICSDECR})
	router.Register(&CommandDef{Name: "ANALYTICS.GET", Handler: cmdANALYTICSGET})
	router.Register(&CommandDef{Name: "ANALYTICS.SUM", Handler: cmdANALYTICSSUM})
	router.Register(&CommandDef{Name: "ANALYTICS.AVG", Handler: cmdANALYTICSAVG})
	router.Register(&CommandDef{Name: "ANALYTICS.MIN", Handler: cmdANALYTICSMIN})
	router.Register(&CommandDef{Name: "ANALYTICS.MAX", Handler: cmdANALYTICSMAX})
	router.Register(&CommandDef{Name: "ANALYTICS.COUNT", Handler: cmdANALYTICSCOUNT})
	router.Register(&CommandDef{Name: "ANALYTICS.CLEAR", Handler: cmdANALYTICSCLEAR})

	router.Register(&CommandDef{Name: "CONNECTION.LIST", Handler: cmdCONNECTIONLIST})
	router.Register(&CommandDef{Name: "CONNECTION.KILL", Handler: cmdCONNECTIONKILL})
	router.Register(&CommandDef{Name: "CONNECTION.COUNT", Handler: cmdCONNECTIONCOUNT})
	router.Register(&CommandDef{Name: "CONNECTION.INFO", Handler: cmdCONNECTIONINFO})

	router.Register(&CommandDef{Name: "PLUGIN.LOAD", Handler: cmdPLUGINLOAD})
	router.Register(&CommandDef{Name: "PLUGIN.UNLOAD", Handler: cmdPLUGINUNLOAD})
	router.Register(&CommandDef{Name: "PLUGIN.LIST", Handler: cmdPLUGINLIST})
	router.Register(&CommandDef{Name: "PLUGIN.CALL", Handler: cmdPLUGINCALL})
	router.Register(&CommandDef{Name: "PLUGIN.INFO", Handler: cmdPLUGININFO})

	router.Register(&CommandDef{Name: "ROLLUP.CREATE", Handler: cmdROLLUPCREATE})
	router.Register(&CommandDef{Name: "ROLLUP.ADD", Handler: cmdROLLUPADD})
	router.Register(&CommandDef{Name: "ROLLUP.GET", Handler: cmdROLLUPGET})
	router.Register(&CommandDef{Name: "ROLLUP.DELETE", Handler: cmdROLLUPDELETE})

	router.Register(&CommandDef{Name: "COOLDOWN.SET", Handler: cmdCOOLDOWNSET})
	router.Register(&CommandDef{Name: "COOLDOWN.CHECK", Handler: cmdCOOLDOWNCHECK})
	router.Register(&CommandDef{Name: "COOLDOWN.RESET", Handler: cmdCOOLDOWNRESET})
	router.Register(&CommandDef{Name: "COOLDOWN.DELETE", Handler: cmdCOOLDOWNDELETE})
	router.Register(&CommandDef{Name: "COOLDOWN.LIST", Handler: cmdCOOLDOWNLIST})

	router.Register(&CommandDef{Name: "QUOTA.SET", Handler: cmdQUOTASET})
	router.Register(&CommandDef{Name: "QUOTA.CHECK", Handler: cmdQUOTACHECK})
	router.Register(&CommandDef{Name: "QUOTA.USE", Handler: cmdQUOTAUSE})
	router.Register(&CommandDef{Name: "QUOTA.RESET", Handler: cmdQUOTARESET})
	router.Register(&CommandDef{Name: "QUOTA.DELETE", Handler: cmdQUOTADELETE})
	router.Register(&CommandDef{Name: "QUOTA.LIST", Handler: cmdQUOTALIST})
}

var (
	mvccTxns   = make(map[int64]*MVCCTransaction)
	mvccTxnsMu sync.RWMutex
	mvccNextID int64
)

type MVCCTransaction struct {
	ID        int64
	Status    string
	CreatedAt int64
	Changes   map[string]*MVCCChange
}

type MVCCChange struct {
	Key       string
	OldValue  string
	NewValue  string
	Operation string
}

func cmdMVCCBEGIN(ctx *Context) error {
	mvccTxnsMu.Lock()
	mvccNextID++
	id := mvccNextID
	mvccTxns[id] = &MVCCTransaction{
		ID:        id,
		Status:    "active",
		CreatedAt: time.Now().UnixMilli(),
		Changes:   make(map[string]*MVCCChange),
	}
	mvccTxnsMu.Unlock()

	return ctx.WriteInteger(id)
}

func cmdMVCCCOMMIT(ctx *Context) error {
	if ctx.ArgCount() < 1 {
		return ctx.WriteError(ErrWrongArgCount)
	}

	id := parseInt64(ctx.ArgString(0))

	mvccTxnsMu.Lock()
	defer mvccTxnsMu.Unlock()

	txn, exists := mvccTxns[id]
	if !exists {
		return ctx.WriteError(fmt.Errorf("ERR transaction not found"))
	}

	if txn.Status != "active" {
		return ctx.WriteError(fmt.Errorf("ERR transaction not active"))
	}

	for key, change := range txn.Changes {
		if change.Operation == "set" {
			ctx.Store.Set(key, &store.StringValue{Data: []byte(change.NewValue)}, store.SetOptions{})
		} else if change.Operation == "delete" {
			ctx.Store.Delete(key)
		}
	}

	txn.Status = "committed"
	delete(mvccTxns, id)

	return ctx.WriteOK()
}

func cmdMVCCROLLBACK(ctx *Context) error {
	if ctx.ArgCount() < 1 {
		return ctx.WriteError(ErrWrongArgCount)
	}

	id := parseInt64(ctx.ArgString(0))

	mvccTxnsMu.Lock()
	defer mvccTxnsMu.Unlock()

	txn, exists := mvccTxns[id]
	if !exists {
		return ctx.WriteError(fmt.Errorf("ERR transaction not found"))
	}

	txn.Status = "rolledback"
	delete(mvccTxns, id)

	return ctx.WriteOK()
}

func cmdMVCCGET(ctx *Context) error {
	if ctx.ArgCount() < 2 {
		return ctx.WriteError(ErrWrongArgCount)
	}

	id := parseInt64(ctx.ArgString(0))
	key := ctx.ArgString(1)

	mvccTxnsMu.RLock()
	txn, exists := mvccTxns[id]
	mvccTxnsMu.RUnlock()

	if !exists {
		return ctx.WriteError(fmt.Errorf("ERR transaction not found"))
	}

	if change, ok := txn.Changes[key]; ok {
		if change.Operation == "delete" {
			return ctx.WriteNull()
		}
		return ctx.WriteBulkString(change.NewValue)
	}

	entry, ok := ctx.Store.Get(key)
	if !ok {
		return ctx.WriteNull()
	}

	return ctx.WriteBulkString(entry.Value.String())
}

func cmdMVCCSET(ctx *Context) error {
	if ctx.ArgCount() < 3 {
		return ctx.WriteError(ErrWrongArgCount)
	}

	id := parseInt64(ctx.ArgString(0))
	key := ctx.ArgString(1)
	value := ctx.ArgString(2)

	mvccTxnsMu.RLock()
	txn, exists := mvccTxns[id]
	mvccTxnsMu.RUnlock()

	if !exists {
		return ctx.WriteError(fmt.Errorf("ERR transaction not found"))
	}

	if txn.Status != "active" {
		return ctx.WriteError(fmt.Errorf("ERR transaction not active"))
	}

	oldValue := ""
	if entry, ok := ctx.Store.Get(key); ok {
		oldValue = entry.Value.String()
	}

	txn.Changes[key] = &MVCCChange{
		Key:       key,
		OldValue:  oldValue,
		NewValue:  value,
		Operation: "set",
	}

	return ctx.WriteOK()
}

func cmdMVCCDELETE(ctx *Context) error {
	if ctx.ArgCount() < 2 {
		return ctx.WriteError(ErrWrongArgCount)
	}

	id := parseInt64(ctx.ArgString(0))
	key := ctx.ArgString(1)

	mvccTxnsMu.RLock()
	txn, exists := mvccTxns[id]
	mvccTxnsMu.RUnlock()

	if !exists {
		return ctx.WriteError(fmt.Errorf("ERR transaction not found"))
	}

	if txn.Status != "active" {
		return ctx.WriteError(fmt.Errorf("ERR transaction not active"))
	}

	oldValue := ""
	if entry, ok := ctx.Store.Get(key); ok {
		oldValue = entry.Value.String()
	}

	txn.Changes[key] = &MVCCChange{
		Key:       key,
		OldValue:  oldValue,
		NewValue:  "",
		Operation: "delete",
	}

	return ctx.WriteOK()
}

func cmdMVCCSTATUS(ctx *Context) error {
	if ctx.ArgCount() < 1 {
		return ctx.WriteError(ErrWrongArgCount)
	}

	id := parseInt64(ctx.ArgString(0))

	mvccTxnsMu.RLock()
	txn, exists := mvccTxns[id]
	mvccTxnsMu.RUnlock()

	if !exists {
		return ctx.WriteNull()
	}

	return ctx.WriteArray([]*resp.Value{
		resp.BulkString("id"),
		resp.IntegerValue(txn.ID),
		resp.BulkString("status"),
		resp.BulkString(txn.Status),
		resp.BulkString("changes"),
		resp.IntegerValue(int64(len(txn.Changes))),
	})
}

func cmdMVCCSNAPSHOT(ctx *Context) error {
	keys := ctx.Store.Keys()

	results := make([]*resp.Value, 0)
	for _, key := range keys {
		entry, ok := ctx.Store.Get(key)
		if ok {
			results = append(results,
				resp.BulkString(key),
				resp.BulkString(entry.Value.String()),
			)
		}
	}

	return ctx.WriteArray(results)
}

var (
	spatialIndexes   = make(map[string]*SpatialIndex)
	spatialIndexesMu sync.RWMutex
)

type SpatialIndex struct {
	Name   string
	Points map[string]*GeoPoint
}

type GeoPoint struct {
	ID   string
	Lat  float64
	Lon  float64
	Data string
}

func cmdSPATIALCREATE(ctx *Context) error {
	if ctx.ArgCount() < 1 {
		return ctx.WriteError(ErrWrongArgCount)
	}

	name := ctx.ArgString(0)

	spatialIndexesMu.Lock()
	spatialIndexes[name] = &SpatialIndex{
		Name:   name,
		Points: make(map[string]*GeoPoint),
	}
	spatialIndexesMu.Unlock()

	return ctx.WriteOK()
}

func cmdSPATIALADD(ctx *Context) error {
	if ctx.ArgCount() < 4 {
		return ctx.WriteError(ErrWrongArgCount)
	}

	name := ctx.ArgString(0)
	id := ctx.ArgString(1)
	lat := parseFloatExt([]byte(ctx.ArgString(2)))
	lon := parseFloatExt([]byte(ctx.ArgString(3)))
	data := ""
	if ctx.ArgCount() >= 5 {
		data = ctx.ArgString(4)
	}

	spatialIndexesMu.RLock()
	idx, exists := spatialIndexes[name]
	spatialIndexesMu.RUnlock()

	if !exists {
		return ctx.WriteError(fmt.Errorf("ERR spatial index not found"))
	}

	idx.Points[id] = &GeoPoint{
		ID:   id,
		Lat:  lat,
		Lon:  lon,
		Data: data,
	}

	return ctx.WriteOK()
}

func cmdSPATIALNEARBY(ctx *Context) error {
	if ctx.ArgCount() < 4 {
		return ctx.WriteError(ErrWrongArgCount)
	}

	name := ctx.ArgString(0)
	lat := parseFloatExt([]byte(ctx.ArgString(1)))
	lon := parseFloatExt([]byte(ctx.ArgString(2)))
	radius := parseFloatExt([]byte(ctx.ArgString(3)))

	spatialIndexesMu.RLock()
	idx, exists := spatialIndexes[name]
	spatialIndexesMu.RUnlock()

	if !exists {
		return ctx.WriteError(fmt.Errorf("ERR spatial index not found"))
	}

	results := make([]*resp.Value, 0)
	for id, point := range idx.Points {
		dist := haversine(lat, lon, point.Lat, point.Lon)
		if dist <= radius {
			results = append(results,
				resp.BulkString(id),
				resp.BulkString(fmt.Sprintf("%.6f", dist)),
			)
		}
	}

	return ctx.WriteArray(results)
}

func cmdSPATIALWITHIN(ctx *Context) error {
	if ctx.ArgCount() < 5 {
		return ctx.WriteError(ErrWrongArgCount)
	}

	name := ctx.ArgString(0)
	minLat := parseFloatExt([]byte(ctx.ArgString(1)))
	minLon := parseFloatExt([]byte(ctx.ArgString(2)))
	maxLat := parseFloatExt([]byte(ctx.ArgString(3)))
	maxLon := parseFloatExt([]byte(ctx.ArgString(4)))

	spatialIndexesMu.RLock()
	idx, exists := spatialIndexes[name]
	spatialIndexesMu.RUnlock()

	if !exists {
		return ctx.WriteError(fmt.Errorf("ERR spatial index not found"))
	}

	results := make([]*resp.Value, 0)
	for id, point := range idx.Points {
		if point.Lat >= minLat && point.Lat <= maxLat &&
			point.Lon >= minLon && point.Lon <= maxLon {
			results = append(results, resp.BulkString(id))
		}
	}

	return ctx.WriteArray(results)
}

func cmdSPATIALDELETE(ctx *Context) error {
	if ctx.ArgCount() < 2 {
		return ctx.WriteError(ErrWrongArgCount)
	}

	name := ctx.ArgString(0)
	id := ctx.ArgString(1)

	spatialIndexesMu.RLock()
	idx, exists := spatialIndexes[name]
	spatialIndexesMu.RUnlock()

	if !exists {
		return ctx.WriteInteger(0)
	}

	if _, ok := idx.Points[id]; ok {
		delete(idx.Points, id)
		return ctx.WriteInteger(1)
	}

	return ctx.WriteInteger(0)
}

func cmdSPATIALLIST(ctx *Context) error {
	spatialIndexesMu.RLock()
	defer spatialIndexesMu.RUnlock()

	results := make([]*resp.Value, 0)
	for name := range spatialIndexes {
		results = append(results, resp.BulkString(name))
	}

	return ctx.WriteArray(results)
}

func haversine(lat1, lon1, lat2, lon2 float64) float64 {
	const R = 6371

	dLat := (lat2 - lat1) * 0.01745329252
	dLon := (lon2 - lon1) * 0.01745329252

	lat1Rad := lat1 * 0.01745329252
	lat2Rad := lat2 * 0.01745329252

	a := sin(dLat/2)*sin(dLat/2) + cos(lat1Rad)*cos(lat2Rad)*sin(dLon/2)*sin(dLon/2)
	c := 2 * atan2(sqrt(a), sqrt(1-a))

	return R * c
}

func sin(x float64) float64 {
	result := x
	term := x
	for i := 1; i < 10; i++ {
		term *= -x * x / float64(2*i*(2*i+1))
		result += term
	}
	return result
}

func cos(x float64) float64 {
	return sin(x + 1.57079632679)
}

func sqrt(x float64) float64 {
	if x < 0 {
		return 0
	}
	z := x
	for i := 0; i < 10; i++ {
		z = (z + x/z) / 2
	}
	return z
}

func atan2(y, x float64) float64 {
	if x == 0 {
		if y > 0 {
			return 1.57079632679
		} else if y < 0 {
			return -1.57079632679
		}
		return 0
	}
	return atan(y / x)
}

func atan(x float64) float64 {
	result := x
	term := x
	x2 := x * x
	for i := 1; i < 10; i++ {
		term *= -x2
		result += term / float64(2*i+1)
	}
	return result
}

var (
	chains   = make(map[string]*BlockChain)
	chainsMu sync.RWMutex
)

type BlockChain struct {
	Name   string
	Blocks []*Block
}

type Block struct {
	Index     int64
	Timestamp int64
	Data      string
	PrevHash  string
	Hash      string
}

func cmdCHAINCREATE(ctx *Context) error {
	if ctx.ArgCount() < 1 {
		return ctx.WriteError(ErrWrongArgCount)
	}

	name := ctx.ArgString(0)

	chainsMu.Lock()
	chains[name] = &BlockChain{
		Name:   name,
		Blocks: make([]*Block, 0),
	}
	chainsMu.Unlock()

	return ctx.WriteOK()
}

func cmdCHAINADD(ctx *Context) error {
	if ctx.ArgCount() < 2 {
		return ctx.WriteError(ErrWrongArgCount)
	}

	name := ctx.ArgString(0)
	data := ctx.ArgString(1)

	chainsMu.RLock()
	chain, exists := chains[name]
	chainsMu.RUnlock()

	if !exists {
		return ctx.WriteError(fmt.Errorf("ERR chain not found"))
	}

	var index int64 = 0
	var prevHash string = "0"
	if len(chain.Blocks) > 0 {
		lastBlock := chain.Blocks[len(chain.Blocks)-1]
		index = lastBlock.Index + 1
		prevHash = lastBlock.Hash
	}

	hash := computeHash(index, data, prevHash)

	block := &Block{
		Index:     index,
		Timestamp: time.Now().UnixMilli(),
		Data:      data,
		PrevHash:  prevHash,
		Hash:      hash,
	}

	chain.Blocks = append(chain.Blocks, block)

	return ctx.WriteInteger(index)
}

func computeHash(index int64, data, prevHash string) string {
	input := fmt.Sprintf("%d%s%s", index, data, prevHash)
	hash := uint32(0)
	for _, c := range input {
		hash = hash*31 + uint32(c)
	}
	return fmt.Sprintf("%08x", hash)
}

func cmdCHAINGET(ctx *Context) error {
	if ctx.ArgCount() < 2 {
		return ctx.WriteError(ErrWrongArgCount)
	}

	name := ctx.ArgString(0)
	index := parseInt64(ctx.ArgString(1))

	chainsMu.RLock()
	chain, exists := chains[name]
	chainsMu.RUnlock()

	if !exists {
		return ctx.WriteError(fmt.Errorf("ERR chain not found"))
	}

	if index < 0 || int(index) >= len(chain.Blocks) {
		return ctx.WriteNull()
	}

	block := chain.Blocks[index]

	return ctx.WriteArray([]*resp.Value{
		resp.BulkString("index"),
		resp.IntegerValue(block.Index),
		resp.BulkString("timestamp"),
		resp.IntegerValue(block.Timestamp),
		resp.BulkString("data"),
		resp.BulkString(block.Data),
		resp.BulkString("hash"),
		resp.BulkString(block.Hash),
		resp.BulkString("prev_hash"),
		resp.BulkString(block.PrevHash),
	})
}

func cmdCHAINVALIDATE(ctx *Context) error {
	if ctx.ArgCount() < 1 {
		return ctx.WriteError(ErrWrongArgCount)
	}

	name := ctx.ArgString(0)

	chainsMu.RLock()
	chain, exists := chains[name]
	chainsMu.RUnlock()

	if !exists {
		return ctx.WriteError(fmt.Errorf("ERR chain not found"))
	}

	for i := 1; i < len(chain.Blocks); i++ {
		current := chain.Blocks[i]
		previous := chain.Blocks[i-1]

		if current.PrevHash != previous.Hash {
			return ctx.WriteInteger(0)
		}

		expectedHash := computeHash(current.Index, current.Data, current.PrevHash)
		if current.Hash != expectedHash {
			return ctx.WriteInteger(0)
		}
	}

	return ctx.WriteInteger(1)
}

func cmdCHAINLENGTH(ctx *Context) error {
	if ctx.ArgCount() < 1 {
		return ctx.WriteError(ErrWrongArgCount)
	}

	name := ctx.ArgString(0)

	chainsMu.RLock()
	chain, exists := chains[name]
	chainsMu.RUnlock()

	if !exists {
		return ctx.WriteInteger(0)
	}

	return ctx.WriteInteger(int64(len(chain.Blocks)))
}

func cmdCHAINLAST(ctx *Context) error {
	if ctx.ArgCount() < 1 {
		return ctx.WriteError(ErrWrongArgCount)
	}

	name := ctx.ArgString(0)

	chainsMu.RLock()
	chain, exists := chains[name]
	chainsMu.RUnlock()

	if !exists || len(chain.Blocks) == 0 {
		return ctx.WriteNull()
	}

	block := chain.Blocks[len(chain.Blocks)-1]

	return ctx.WriteArray([]*resp.Value{
		resp.BulkString("index"),
		resp.IntegerValue(block.Index),
		resp.BulkString("data"),
		resp.BulkString(block.Data),
		resp.BulkString("hash"),
		resp.BulkString(block.Hash),
	})
}

var (
	analytics   = make(map[string]*AnalyticsData)
	analyticsMu sync.RWMutex
)

type AnalyticsData struct {
	Values []float64
	Sum    float64
	Count  int64
	Min    float64
	Max    float64
}

func cmdANALYTICSINCR(ctx *Context) error {
	if ctx.ArgCount() < 2 {
		return ctx.WriteError(ErrWrongArgCount)
	}

	name := ctx.ArgString(0)
	value := parseFloatExt([]byte(ctx.ArgString(1)))

	analyticsMu.Lock()
	defer analyticsMu.Unlock()

	data, exists := analytics[name]
	if !exists {
		data = &AnalyticsData{
			Values: make([]float64, 0),
			Min:    value,
			Max:    value,
		}
		analytics[name] = data
	}

	data.Values = append(data.Values, value)
	data.Sum += value
	data.Count++
	if value < data.Min {
		data.Min = value
	}
	if value > data.Max {
		data.Max = value
	}

	return ctx.WriteInteger(data.Count)
}

func cmdANALYTICSDECR(ctx *Context) error {
	if ctx.ArgCount() < 2 {
		return ctx.WriteError(ErrWrongArgCount)
	}

	name := ctx.ArgString(0)
	value := -parseFloatExt([]byte(ctx.ArgString(1)))

	analyticsMu.Lock()
	defer analyticsMu.Unlock()

	data, exists := analytics[name]
	if !exists {
		data = &AnalyticsData{
			Values: make([]float64, 0),
			Min:    value,
			Max:    value,
		}
		analytics[name] = data
	}

	data.Values = append(data.Values, value)
	data.Sum += value
	data.Count++
	if value < data.Min {
		data.Min = value
	}
	if value > data.Max {
		data.Max = value
	}

	return ctx.WriteInteger(data.Count)
}

func cmdANALYTICSGET(ctx *Context) error {
	if ctx.ArgCount() < 1 {
		return ctx.WriteError(ErrWrongArgCount)
	}

	name := ctx.ArgString(0)

	analyticsMu.RLock()
	data, exists := analytics[name]
	analyticsMu.RUnlock()

	if !exists {
		return ctx.WriteNull()
	}

	return ctx.WriteArray([]*resp.Value{
		resp.BulkString("count"),
		resp.IntegerValue(data.Count),
		resp.BulkString("sum"),
		resp.BulkString(fmt.Sprintf("%.2f", data.Sum)),
		resp.BulkString("min"),
		resp.BulkString(fmt.Sprintf("%.2f", data.Min)),
		resp.BulkString("max"),
		resp.BulkString(fmt.Sprintf("%.2f", data.Max)),
	})
}

func cmdANALYTICSSUM(ctx *Context) error {
	if ctx.ArgCount() < 1 {
		return ctx.WriteError(ErrWrongArgCount)
	}

	name := ctx.ArgString(0)

	analyticsMu.RLock()
	data, exists := analytics[name]
	analyticsMu.RUnlock()

	if !exists {
		return ctx.WriteBulkString("0")
	}

	return ctx.WriteBulkString(fmt.Sprintf("%.2f", data.Sum))
}

func cmdANALYTICSAVG(ctx *Context) error {
	if ctx.ArgCount() < 1 {
		return ctx.WriteError(ErrWrongArgCount)
	}

	name := ctx.ArgString(0)

	analyticsMu.RLock()
	data, exists := analytics[name]
	analyticsMu.RUnlock()

	if !exists || data.Count == 0 {
		return ctx.WriteBulkString("0")
	}

	return ctx.WriteBulkString(fmt.Sprintf("%.2f", data.Sum/float64(data.Count)))
}

func cmdANALYTICSMIN(ctx *Context) error {
	if ctx.ArgCount() < 1 {
		return ctx.WriteError(ErrWrongArgCount)
	}

	name := ctx.ArgString(0)

	analyticsMu.RLock()
	data, exists := analytics[name]
	analyticsMu.RUnlock()

	if !exists {
		return ctx.WriteBulkString("0")
	}

	return ctx.WriteBulkString(fmt.Sprintf("%.2f", data.Min))
}

func cmdANALYTICSMAX(ctx *Context) error {
	if ctx.ArgCount() < 1 {
		return ctx.WriteError(ErrWrongArgCount)
	}

	name := ctx.ArgString(0)

	analyticsMu.RLock()
	data, exists := analytics[name]
	analyticsMu.RUnlock()

	if !exists {
		return ctx.WriteBulkString("0")
	}

	return ctx.WriteBulkString(fmt.Sprintf("%.2f", data.Max))
}

func cmdANALYTICSCOUNT(ctx *Context) error {
	if ctx.ArgCount() < 1 {
		return ctx.WriteError(ErrWrongArgCount)
	}

	name := ctx.ArgString(0)

	analyticsMu.RLock()
	data, exists := analytics[name]
	analyticsMu.RUnlock()

	if !exists {
		return ctx.WriteInteger(0)
	}

	return ctx.WriteInteger(data.Count)
}

func cmdANALYTICSCLEAR(ctx *Context) error {
	if ctx.ArgCount() < 1 {
		return ctx.WriteError(ErrWrongArgCount)
	}

	name := ctx.ArgString(0)

	analyticsMu.Lock()
	delete(analytics, name)
	analyticsMu.Unlock()

	return ctx.WriteOK()
}

var (
	connections   = make(map[int64]*ConnectionInfo)
	connectionsMu sync.RWMutex
)

type ConnectionInfo struct {
	ID          int64
	RemoteAddr  string
	ConnectedAt int64
	LastActive  int64
	Commands    int64
}

func cmdCONNECTIONLIST(ctx *Context) error {
	connectionsMu.RLock()
	defer connectionsMu.RUnlock()

	results := make([]*resp.Value, 0)
	for id, conn := range connections {
		results = append(results,
			resp.IntegerValue(id),
			resp.BulkString(conn.RemoteAddr),
		)
	}

	return ctx.WriteArray(results)
}

func cmdCONNECTIONKILL(ctx *Context) error {
	if ctx.ArgCount() < 1 {
		return ctx.WriteError(ErrWrongArgCount)
	}

	id := parseInt64(ctx.ArgString(0))

	connectionsMu.Lock()
	defer connectionsMu.Unlock()

	if _, exists := connections[id]; !exists {
		return ctx.WriteInteger(0)
	}

	delete(connections, id)
	return ctx.WriteInteger(1)
}

func cmdCONNECTIONCOUNT(ctx *Context) error {
	connectionsMu.RLock()
	count := int64(len(connections))
	connectionsMu.RUnlock()

	return ctx.WriteInteger(count)
}

func cmdCONNECTIONINFO(ctx *Context) error {
	if ctx.ArgCount() < 1 {
		return ctx.WriteError(ErrWrongArgCount)
	}

	id := parseInt64(ctx.ArgString(0))

	connectionsMu.RLock()
	conn, exists := connections[id]
	connectionsMu.RUnlock()

	if !exists {
		return ctx.WriteNull()
	}

	return ctx.WriteArray([]*resp.Value{
		resp.BulkString("id"),
		resp.IntegerValue(conn.ID),
		resp.BulkString("remote_addr"),
		resp.BulkString(conn.RemoteAddr),
		resp.BulkString("connected_at"),
		resp.IntegerValue(conn.ConnectedAt),
		resp.BulkString("commands"),
		resp.IntegerValue(conn.Commands),
	})
}

var (
	plugins   = make(map[string]*Plugin)
	pluginsMu sync.RWMutex
)

type Plugin struct {
	Name      string
	Enabled   bool
	LoadedAt  int64
	Functions map[string]string
}

func cmdPLUGINLOAD(ctx *Context) error {
	if ctx.ArgCount() < 1 {
		return ctx.WriteError(ErrWrongArgCount)
	}

	name := ctx.ArgString(0)

	pluginsMu.Lock()
	defer pluginsMu.Unlock()

	plugins[name] = &Plugin{
		Name:      name,
		Enabled:   true,
		LoadedAt:  time.Now().UnixMilli(),
		Functions: make(map[string]string),
	}

	return ctx.WriteOK()
}

func cmdPLUGINUNLOAD(ctx *Context) error {
	if ctx.ArgCount() < 1 {
		return ctx.WriteError(ErrWrongArgCount)
	}

	name := ctx.ArgString(0)

	pluginsMu.Lock()
	defer pluginsMu.Unlock()

	if _, exists := plugins[name]; !exists {
		return ctx.WriteInteger(0)
	}

	delete(plugins, name)
	return ctx.WriteInteger(1)
}

func cmdPLUGINLIST(ctx *Context) error {
	pluginsMu.RLock()
	defer pluginsMu.RUnlock()

	results := make([]*resp.Value, 0)
	for name, p := range plugins {
		results = append(results,
			resp.BulkString(name),
			resp.BulkString(fmt.Sprintf("%v", p.Enabled)),
		)
	}

	return ctx.WriteArray(results)
}

func cmdPLUGINCALL(ctx *Context) error {
	if ctx.ArgCount() < 2 {
		return ctx.WriteError(ErrWrongArgCount)
	}

	name := ctx.ArgString(0)
	fn := ctx.ArgString(1)

	pluginsMu.RLock()
	plugin, exists := plugins[name]
	pluginsMu.RUnlock()

	if !exists {
		return ctx.WriteError(fmt.Errorf("ERR plugin not found"))
	}

	if !plugin.Enabled {
		return ctx.WriteError(fmt.Errorf("ERR plugin disabled"))
	}

	return ctx.WriteBulkString(fn + ":ok")
}

func cmdPLUGININFO(ctx *Context) error {
	if ctx.ArgCount() < 1 {
		return ctx.WriteError(ErrWrongArgCount)
	}

	name := ctx.ArgString(0)

	pluginsMu.RLock()
	plugin, exists := plugins[name]
	pluginsMu.RUnlock()

	if !exists {
		return ctx.WriteNull()
	}

	return ctx.WriteArray([]*resp.Value{
		resp.BulkString("name"),
		resp.BulkString(plugin.Name),
		resp.BulkString("enabled"),
		resp.BulkString(fmt.Sprintf("%v", plugin.Enabled)),
		resp.BulkString("loaded_at"),
		resp.IntegerValue(plugin.LoadedAt),
	})
}

var (
	rollups   = make(map[string]*Rollup)
	rollupsMu sync.RWMutex
)

type Rollup struct {
	Name     string
	Data     map[int64]float64
	Interval int64
}

func cmdROLLUPCREATE(ctx *Context) error {
	if ctx.ArgCount() < 2 {
		return ctx.WriteError(ErrWrongArgCount)
	}

	name := ctx.ArgString(0)
	interval := parseInt64(ctx.ArgString(1))

	rollupsMu.Lock()
	rollups[name] = &Rollup{
		Name:     name,
		Data:     make(map[int64]float64),
		Interval: interval,
	}
	rollupsMu.Unlock()

	return ctx.WriteOK()
}

func cmdROLLUPADD(ctx *Context) error {
	if ctx.ArgCount() < 3 {
		return ctx.WriteError(ErrWrongArgCount)
	}

	name := ctx.ArgString(0)
	timestamp := parseInt64(ctx.ArgString(1))
	value := parseFloatExt([]byte(ctx.ArgString(2)))

	rollupsMu.RLock()
	rollup, exists := rollups[name]
	rollupsMu.RUnlock()

	if !exists {
		return ctx.WriteError(fmt.Errorf("ERR rollup not found"))
	}

	bucket := (timestamp / rollup.Interval) * rollup.Interval
	rollup.Data[bucket] += value

	return ctx.WriteOK()
}

func cmdROLLUPGET(ctx *Context) error {
	if ctx.ArgCount() < 2 {
		return ctx.WriteError(ErrWrongArgCount)
	}

	name := ctx.ArgString(0)
	timestamp := parseInt64(ctx.ArgString(1))

	rollupsMu.RLock()
	rollup, exists := rollups[name]
	rollupsMu.RUnlock()

	if !exists {
		return ctx.WriteError(fmt.Errorf("ERR rollup not found"))
	}

	bucket := (timestamp / rollup.Interval) * rollup.Interval
	value := rollup.Data[bucket]

	return ctx.WriteBulkString(fmt.Sprintf("%.2f", value))
}

func cmdROLLUPDELETE(ctx *Context) error {
	if ctx.ArgCount() < 1 {
		return ctx.WriteError(ErrWrongArgCount)
	}

	name := ctx.ArgString(0)

	rollupsMu.Lock()
	defer rollupsMu.Unlock()

	if _, exists := rollups[name]; !exists {
		return ctx.WriteInteger(0)
	}

	delete(rollups, name)
	return ctx.WriteInteger(1)
}

var (
	cooldowns   = make(map[string]*Cooldown)
	cooldownsMu sync.RWMutex
)

type Cooldown struct {
	Key      string
	Duration int64
	LastUsed int64
}

func cmdCOOLDOWNSET(ctx *Context) error {
	if ctx.ArgCount() < 2 {
		return ctx.WriteError(ErrWrongArgCount)
	}

	key := ctx.ArgString(0)
	durationMs := parseInt64(ctx.ArgString(1))

	cooldownsMu.Lock()
	cooldowns[key] = &Cooldown{
		Key:      key,
		Duration: durationMs,
		LastUsed: 0,
	}
	cooldownsMu.Unlock()

	return ctx.WriteOK()
}

func cmdCOOLDOWNCHECK(ctx *Context) error {
	if ctx.ArgCount() < 1 {
		return ctx.WriteError(ErrWrongArgCount)
	}

	key := ctx.ArgString(0)

	cooldownsMu.RLock()
	cd, exists := cooldowns[key]
	cooldownsMu.RUnlock()

	if !exists {
		return ctx.WriteInteger(1)
	}

	now := time.Now().UnixMilli()
	if now >= cd.LastUsed+cd.Duration {
		cd.LastUsed = now
		return ctx.WriteInteger(1)
	}

	remaining := (cd.LastUsed + cd.Duration) - now
	return ctx.WriteArray([]*resp.Value{
		resp.IntegerValue(0),
		resp.IntegerValue(remaining),
	})
}

func cmdCOOLDOWNRESET(ctx *Context) error {
	if ctx.ArgCount() < 1 {
		return ctx.WriteError(ErrWrongArgCount)
	}

	key := ctx.ArgString(0)

	cooldownsMu.RLock()
	cd, exists := cooldowns[key]
	cooldownsMu.RUnlock()

	if !exists {
		return ctx.WriteInteger(0)
	}

	cd.LastUsed = 0
	return ctx.WriteInteger(1)
}

func cmdCOOLDOWNDELETE(ctx *Context) error {
	if ctx.ArgCount() < 1 {
		return ctx.WriteError(ErrWrongArgCount)
	}

	key := ctx.ArgString(0)

	cooldownsMu.Lock()
	defer cooldownsMu.Unlock()

	if _, exists := cooldowns[key]; !exists {
		return ctx.WriteInteger(0)
	}

	delete(cooldowns, key)
	return ctx.WriteInteger(1)
}

func cmdCOOLDOWNLIST(ctx *Context) error {
	cooldownsMu.RLock()
	defer cooldownsMu.RUnlock()

	results := make([]*resp.Value, 0)
	for key, cd := range cooldowns {
		results = append(results,
			resp.BulkString(key),
			resp.IntegerValue(cd.Duration),
		)
	}

	return ctx.WriteArray(results)
}

var (
	quotas   = make(map[string]*Quota)
	quotasMu sync.RWMutex
)

type Quota struct {
	Key     string
	Limit   int64
	Used    int64
	ResetAt int64
}

func cmdQUOTASET(ctx *Context) error {
	if ctx.ArgCount() < 3 {
		return ctx.WriteError(ErrWrongArgCount)
	}

	key := ctx.ArgString(0)
	limit := parseInt64(ctx.ArgString(1))
	windowMs := parseInt64(ctx.ArgString(2))

	quotasMu.Lock()
	quotas[key] = &Quota{
		Key:     key,
		Limit:   limit,
		Used:    0,
		ResetAt: time.Now().UnixMilli() + windowMs,
	}
	quotasMu.Unlock()

	return ctx.WriteOK()
}

func cmdQUOTACHECK(ctx *Context) error {
	if ctx.ArgCount() < 1 {
		return ctx.WriteError(ErrWrongArgCount)
	}

	key := ctx.ArgString(0)

	quotasMu.RLock()
	quota, exists := quotas[key]
	quotasMu.RUnlock()

	if !exists {
		return ctx.WriteError(fmt.Errorf("ERR quota not found"))
	}

	now := time.Now().UnixMilli()
	if now >= quota.ResetAt {
		quota.Used = 0
		quota.ResetAt = now + (quota.ResetAt - (now - quota.Limit*1000))
	}

	remaining := quota.Limit - quota.Used

	return ctx.WriteArray([]*resp.Value{
		resp.BulkString("limit"),
		resp.IntegerValue(quota.Limit),
		resp.BulkString("used"),
		resp.IntegerValue(quota.Used),
		resp.BulkString("remaining"),
		resp.IntegerValue(remaining),
		resp.BulkString("reset_at"),
		resp.IntegerValue(quota.ResetAt),
	})
}

func cmdQUOTAUSE(ctx *Context) error {
	if ctx.ArgCount() < 2 {
		return ctx.WriteError(ErrWrongArgCount)
	}

	key := ctx.ArgString(0)
	amount := parseInt64(ctx.ArgString(1))

	quotasMu.RLock()
	quota, exists := quotas[key]
	quotasMu.RUnlock()

	if !exists {
		return ctx.WriteError(fmt.Errorf("ERR quota not found"))
	}

	now := time.Now().UnixMilli()
	if now >= quota.ResetAt {
		quota.Used = 0
	}

	if quota.Used+amount > quota.Limit {
		return ctx.WriteInteger(0)
	}

	quota.Used += amount
	return ctx.WriteInteger(1)
}

func cmdQUOTARESET(ctx *Context) error {
	if ctx.ArgCount() < 1 {
		return ctx.WriteError(ErrWrongArgCount)
	}

	key := ctx.ArgString(0)

	quotasMu.RLock()
	quota, exists := quotas[key]
	quotasMu.RUnlock()

	if !exists {
		return ctx.WriteInteger(0)
	}

	quota.Used = 0
	return ctx.WriteInteger(1)
}

func cmdQUOTADELETE(ctx *Context) error {
	if ctx.ArgCount() < 1 {
		return ctx.WriteError(ErrWrongArgCount)
	}

	key := ctx.ArgString(0)

	quotasMu.Lock()
	defer quotasMu.Unlock()

	if _, exists := quotas[key]; !exists {
		return ctx.WriteInteger(0)
	}

	delete(quotas, key)
	return ctx.WriteInteger(1)
}

func cmdQUOTALIST(ctx *Context) error {
	quotasMu.RLock()
	defer quotasMu.RUnlock()

	results := make([]*resp.Value, 0)
	for key, quota := range quotas {
		results = append(results,
			resp.BulkString(key),
			resp.IntegerValue(quota.Limit),
			resp.IntegerValue(quota.Used),
		)
	}

	return ctx.WriteArray(results)
}
