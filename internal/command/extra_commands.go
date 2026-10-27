package command

import (
	"fmt"
	"sort"
	"strconv"
	"sync"
	"sync/atomic"
	"time"

	"github.com/cachestorm/cachestorm/internal/resp"
)

func RegisterExtraCommands(router *Router) {

	router.Register(&CommandDef{Name: "VECTOR_CLOCK.CREATE", Handler: cmdVECTORCLOCKCREATE})
	router.Register(&CommandDef{Name: "VECTOR_CLOCK.INCREMENT", Handler: cmdVECTORCLOCKINCREMENT})
	router.Register(&CommandDef{Name: "VECTOR_CLOCK.COMPARE", Handler: cmdVECTORCLOCKCOMPARE})
	router.Register(&CommandDef{Name: "VECTOR_CLOCK.MERGE", Handler: cmdVECTORCLOCKMERGE})
	router.Register(&CommandDef{Name: "VECTOR_CLOCK.GET", Handler: cmdVECTORCLOCKGET})

	router.Register(&CommandDef{Name: "CRDT.LWW.SET", Handler: cmdCRDTLWWSET})
	router.Register(&CommandDef{Name: "CRDT.LWW.GET", Handler: cmdCRDTLWWGET})
	router.Register(&CommandDef{Name: "CRDT.LWW.DELETE", Handler: cmdCRDTLWWDELETE})
	router.Register(&CommandDef{Name: "CRDT.GCOUNTER.INCR", Handler: cmdCRDTGCOUNTERINCR})
	router.Register(&CommandDef{Name: "CRDT.GCOUNTER.GET", Handler: cmdCRDTGCOUNTERGET})
	router.Register(&CommandDef{Name: "CRDT.PNCounter.INCR", Handler: cmdCRDTPNCOUNTERINCR})
	router.Register(&CommandDef{Name: "CRDT.PNCounter.DECR", Handler: cmdCRDTPNCOUNTERDECR})
	router.Register(&CommandDef{Name: "CRDT.PNCounter.GET", Handler: cmdCRDTPNCOUNTERGET})
	router.Register(&CommandDef{Name: "CRDT.GSET.ADD", Handler: cmdCRDTGSETADD})
	router.Register(&CommandDef{Name: "CRDT.GSET.GET", Handler: cmdCRDTGSETGET})
	router.Register(&CommandDef{Name: "CRDT.ORSET.ADD", Handler: cmdCRDTORSETADD})
	router.Register(&CommandDef{Name: "CRDT.ORSET.REMOVE", Handler: cmdCRDTORSETREMOVE})
	router.Register(&CommandDef{Name: "CRDT.ORSET.GET", Handler: cmdCRDTORSETGET})
	router.Register(&CommandDef{Name: "CRDT.ORSET.TAGS", Handler: cmdCRDTORSETTAGS})

	router.Register(&CommandDef{Name: "DEDUP.ADD", Handler: cmdDEDUPADD})
	router.Register(&CommandDef{Name: "DEDUP.CHECK", Handler: cmdDEDUPCHECK})
	router.Register(&CommandDef{Name: "DEDUP.EXPIRE", Handler: cmdDEDUPEXPIRE})
	router.Register(&CommandDef{Name: "DEDUP.CLEAR", Handler: cmdDEDUPCLEAR})

	router.Register(&CommandDef{Name: "DEADLINE.SET", Handler: cmdDEADLINESET})
	router.Register(&CommandDef{Name: "DEADLINE.CHECK", Handler: cmdDEADLINECHECK})
	router.Register(&CommandDef{Name: "DEADLINE.CANCEL", Handler: cmdDEADLINECANCEL})
	router.Register(&CommandDef{Name: "DEADLINE.LIST", Handler: cmdDEADLINELIST})

	router.Register(&CommandDef{Name: "SANITIZE.STRING", Handler: cmdSANITIZESTRING})
	router.Register(&CommandDef{Name: "SANITIZE.HTML", Handler: cmdSANITIZEHTML})
	router.Register(&CommandDef{Name: "SANITIZE.JSON", Handler: cmdSANITIZEJSON})
	router.Register(&CommandDef{Name: "SANITIZE.SQL", Handler: cmdSANITIZESQL})

	router.Register(&CommandDef{Name: "MASK.CARD", Handler: cmdMASKCARD})
	router.Register(&CommandDef{Name: "MASK.EMAIL", Handler: cmdMASKEMAIL})
	router.Register(&CommandDef{Name: "MASK.PHONE", Handler: cmdMASKPHONE})
	router.Register(&CommandDef{Name: "MASK.IP", Handler: cmdMASKIP})

	router.Register(&CommandDef{Name: "GATEWAY.CREATE", Handler: cmdGATEWAYCREATE})
	router.Register(&CommandDef{Name: "GATEWAY.DELETE", Handler: cmdGATEWAYDELETE})
	router.Register(&CommandDef{Name: "GATEWAY.ROUTE", Handler: cmdGATEWAYROUTE})
	router.Register(&CommandDef{Name: "GATEWAY.LIST", Handler: cmdGATEWAYLIST})

	router.Register(&CommandDef{Name: "THRESHOLD.SET", Handler: cmdTHRESHOLDSET})
	router.Register(&CommandDef{Name: "THRESHOLD.CHECK", Handler: cmdTHRESHOLDCHECK})
	router.Register(&CommandDef{Name: "THRESHOLD.LIST", Handler: cmdTHRESHOLDLIST})
	router.Register(&CommandDef{Name: "THRESHOLD.DELETE", Handler: cmdTHRESHOLDDELETE})

	router.Register(&CommandDef{Name: "SWITCH.STATE", Handler: cmdSWITCHSTATE})
	router.Register(&CommandDef{Name: "SWITCH.TOGGLE", Handler: cmdSWITCHTOGGLE})
	router.Register(&CommandDef{Name: "SWITCH.ON", Handler: cmdSWITCHON})
	router.Register(&CommandDef{Name: "SWITCH.OFF", Handler: cmdSWITCHOFF})
	router.Register(&CommandDef{Name: "SWITCH.LIST", Handler: cmdSWITCHLIST})

	router.Register(&CommandDef{Name: "BOOKMARK.SET", Handler: cmdBOOKMARKSET})
	router.Register(&CommandDef{Name: "BOOKMARK.GET", Handler: cmdBOOKMARKGET})
	router.Register(&CommandDef{Name: "BOOKMARK.DELETE", Handler: cmdBOOKMARKDELETE})
	router.Register(&CommandDef{Name: "BOOKMARK.LIST", Handler: cmdBOOKMARKLIST})

	router.Register(&CommandDef{Name: "ROUTE.ADD", Handler: cmdROUTEADD})
	router.Register(&CommandDef{Name: "ROUTE.REMOVE", Handler: cmdROUTEREMOVE})
	router.Register(&CommandDef{Name: "ROUTE.MATCH", Handler: cmdROUTEMATCH})
	router.Register(&CommandDef{Name: "ROUTE.LIST", Handler: cmdROUTELIST})

	router.Register(&CommandDef{Name: "GHOST.CREATE", Handler: cmdGHOSTCREATE})
	router.Register(&CommandDef{Name: "GHOST.WRITE", Handler: cmdGHOSTWRITE})
	router.Register(&CommandDef{Name: "GHOST.READ", Handler: cmdGHOSTREAD})
	router.Register(&CommandDef{Name: "GHOST.DELETE", Handler: cmdGHOSTDELETE})

	router.Register(&CommandDef{Name: "GRID.CREATE", Handler: cmdGRIDCREATE})
	router.Register(&CommandDef{Name: "GRID.SET", Handler: cmdGRIDSET})
	router.Register(&CommandDef{Name: "GRID.GET", Handler: cmdGRIDGET})
	router.Register(&CommandDef{Name: "GRID.DELETE", Handler: cmdGRIDDELETE})
	router.Register(&CommandDef{Name: "GRID.QUERY", Handler: cmdGRIDQUERY})
	router.Register(&CommandDef{Name: "GRID.CLEAR", Handler: cmdGRIDCLEAR})

	router.Register(&CommandDef{Name: "TAPE.CREATE", Handler: cmdTAPECREATE})
	router.Register(&CommandDef{Name: "TAPE.WRITE", Handler: cmdTAPEWRITE})
	router.Register(&CommandDef{Name: "TAPE.READ", Handler: cmdTAPEREAD})
	router.Register(&CommandDef{Name: "TAPE.SEEK", Handler: cmdTAPESEEK})
	router.Register(&CommandDef{Name: "TAPE.DELETE", Handler: cmdTAPEDELETE})

	router.Register(&CommandDef{Name: "SLICE.CREATE", Handler: cmdSLICECREATE})
	router.Register(&CommandDef{Name: "SLICE.APPEND", Handler: cmdSLICEAPPEND})
	router.Register(&CommandDef{Name: "SLICE.GET", Handler: cmdSLICEGET})
	router.Register(&CommandDef{Name: "SLICE.DELETE", Handler: cmdSLICEDELETE})

	router.Register(&CommandDef{Name: "ROLLUPX.CREATE", Handler: cmdROLLUPXCREATE})
	router.Register(&CommandDef{Name: "ROLLUPX.ADD", Handler: cmdROLLUPXADD})
	router.Register(&CommandDef{Name: "ROLLUPX.GET", Handler: cmdROLLUPXGET})
	router.Register(&CommandDef{Name: "ROLLUPX.DELETE", Handler: cmdROLLUPXDELETE})

}

var (
	vectorClocks   = make(map[string]map[string]int64)
	vectorClocksMu sync.RWMutex
)

func cmdVECTORCLOCKCREATE(ctx *Context) error {
	if ctx.ArgCount() < 2 {
		return ctx.WriteError(ErrWrongArgCount)
	}
	name := ctx.ArgString(0)
	nodeID := ctx.ArgString(1)
	vectorClocksMu.Lock()
	vectorClocks[name] = map[string]int64{nodeID: 0}
	vectorClocksMu.Unlock()
	return ctx.WriteOK()
}

func cmdVECTORCLOCKINCREMENT(ctx *Context) error {
	if ctx.ArgCount() < 2 {
		return ctx.WriteError(ErrWrongArgCount)
	}
	name := ctx.ArgString(0)
	nodeID := ctx.ArgString(1)
	vectorClocksMu.Lock()
	defer vectorClocksMu.Unlock()
	vc, exists := vectorClocks[name]
	if !exists {
		vc = make(map[string]int64)
		vectorClocks[name] = vc
	}
	vc[nodeID]++
	return ctx.WriteInteger(vc[nodeID])
}

func cmdVECTORCLOCKCOMPARE(ctx *Context) error {
	if ctx.ArgCount() < 2 {
		return ctx.WriteError(ErrWrongArgCount)
	}
	name1 := ctx.ArgString(0)
	name2 := ctx.ArgString(1)
	vectorClocksMu.RLock()
	vc1, exists1 := vectorClocks[name1]
	vc2, exists2 := vectorClocks[name2]
	if !exists1 || !exists2 {
		vectorClocksMu.RUnlock()
		return ctx.WriteError(fmt.Errorf("ERR clock not found"))
	}
	allKeys := make(map[string]bool)
	for k := range vc1 {
		allKeys[k] = true
	}
	for k := range vc2 {
		allKeys[k] = true
	}
	v1Greater := false
	v2Greater := false
	for k := range allKeys {
		v1 := vc1[k]
		v2 := vc2[k]
		if v1 > v2 {
			v1Greater = true
		}
		if v2 > v1 {
			v2Greater = true
		}
	}
	vectorClocksMu.RUnlock()
	if !v1Greater && !v2Greater {
		return ctx.WriteBulkString("equal")
	}
	if v2Greater && !v1Greater {
		return ctx.WriteBulkString("before")
	}
	if v1Greater && !v2Greater {
		return ctx.WriteBulkString("after")
	}
	return ctx.WriteBulkString("concurrent")
}

func cmdVECTORCLOCKMERGE(ctx *Context) error {
	if ctx.ArgCount() < 2 {
		return ctx.WriteError(ErrWrongArgCount)
	}
	name1 := ctx.ArgString(0)
	name2 := ctx.ArgString(1)
	vectorClocksMu.Lock()
	defer vectorClocksMu.Unlock()
	vc1, exists1 := vectorClocks[name1]
	vc2, exists2 := vectorClocks[name2]
	if !exists1 || !exists2 {
		return ctx.WriteError(fmt.Errorf("ERR clock not found"))
	}
	for k, v := range vc2 {
		if vc1[k] < v {
			vc1[k] = v
		}
	}
	return ctx.WriteOK()
}

func cmdVECTORCLOCKGET(ctx *Context) error {
	if ctx.ArgCount() < 1 {
		return ctx.WriteError(ErrWrongArgCount)
	}
	name := ctx.ArgString(0)
	vectorClocksMu.RLock()
	vc, exists := vectorClocks[name]
	vectorClocksMu.RUnlock()
	if !exists {
		return ctx.WriteArray([]*resp.Value{})
	}
	results := make([]*resp.Value, 0)
	for k, v := range vc {
		results = append(results, resp.BulkString(k), resp.IntegerValue(v))
	}
	return ctx.WriteArray(results)
}

var (
	crdtLWW   = make(map[string]*LWWEntry)
	crdtLWWMu sync.RWMutex
)

type LWWEntry struct {
	Value     string
	Timestamp int64
}

func cmdCRDTLWWSET(ctx *Context) error {
	if ctx.ArgCount() < 2 {
		return ctx.WriteError(ErrWrongArgCount)
	}
	key := ctx.ArgString(0)
	value := ctx.ArgString(1)
	ts := time.Now().UnixNano()
	if ctx.ArgCount() >= 3 {
		ts = parseInt64(ctx.ArgString(2))
	}
	crdtLWWMu.Lock()
	defer crdtLWWMu.Unlock()
	if entry, exists := crdtLWW[key]; exists {
		if ts > entry.Timestamp {
			entry.Value = value
			entry.Timestamp = ts
		}
	} else {
		crdtLWW[key] = &LWWEntry{Value: value, Timestamp: ts}
	}
	return ctx.WriteOK()
}

func cmdCRDTLWWGET(ctx *Context) error {
	if ctx.ArgCount() < 1 {
		return ctx.WriteError(ErrWrongArgCount)
	}
	key := ctx.ArgString(0)
	crdtLWWMu.RLock()
	entry, exists := crdtLWW[key]
	if !exists {
		crdtLWWMu.RUnlock()
		return ctx.WriteNull()
	}
	reply := entry.Value
	crdtLWWMu.RUnlock()
	return ctx.WriteBulkString(reply)
}

func cmdCRDTLWWDELETE(ctx *Context) error {
	if ctx.ArgCount() < 1 {
		return ctx.WriteError(ErrWrongArgCount)
	}
	key := ctx.ArgString(0)
	crdtLWWMu.Lock()
	defer crdtLWWMu.Unlock()
	if _, exists := crdtLWW[key]; !exists {
		return ctx.WriteInteger(0)
	}
	delete(crdtLWW, key)
	return ctx.WriteInteger(1)
}

var (
	crdtGCounter   = make(map[string]map[string]int64)
	crdtGCounterMu sync.RWMutex
)

func cmdCRDTGCOUNTERINCR(ctx *Context) error {
	if ctx.ArgCount() < 2 {
		return ctx.WriteError(ErrWrongArgCount)
	}
	name := ctx.ArgString(0)
	nodeID := ctx.ArgString(1)
	amount := int64(1)
	if ctx.ArgCount() >= 3 {
		amount = parseInt64(ctx.ArgString(2))
	}
	crdtGCounterMu.Lock()
	defer crdtGCounterMu.Unlock()
	if _, exists := crdtGCounter[name]; !exists {
		crdtGCounter[name] = make(map[string]int64)
	}
	crdtGCounter[name][nodeID] += amount
	return ctx.WriteInteger(crdtGCounter[name][nodeID])
}

func cmdCRDTGCOUNTERGET(ctx *Context) error {
	if ctx.ArgCount() < 1 {
		return ctx.WriteError(ErrWrongArgCount)
	}
	name := ctx.ArgString(0)
	crdtGCounterMu.RLock()
	var total int64
	for _, v := range crdtGCounter[name] {
		total += v
	}
	crdtGCounterMu.RUnlock()
	return ctx.WriteInteger(total)
}

var (
	crdtPNCounter   = make(map[string]*PNCounterState)
	crdtPNCounterMu sync.RWMutex
)

type PNCounterState struct {
	P map[string]int64
	N map[string]int64
}

func cmdCRDTPNCOUNTERINCR(ctx *Context) error {
	if ctx.ArgCount() < 2 {
		return ctx.WriteError(ErrWrongArgCount)
	}
	name := ctx.ArgString(0)
	nodeID := ctx.ArgString(1)
	crdtPNCounterMu.Lock()
	defer crdtPNCounterMu.Unlock()
	if _, exists := crdtPNCounter[name]; !exists {
		crdtPNCounter[name] = &PNCounterState{P: make(map[string]int64), N: make(map[string]int64)}
	}
	crdtPNCounter[name].P[nodeID]++
	return ctx.WriteOK()
}

func cmdCRDTPNCOUNTERDECR(ctx *Context) error {
	if ctx.ArgCount() < 2 {
		return ctx.WriteError(ErrWrongArgCount)
	}
	name := ctx.ArgString(0)
	nodeID := ctx.ArgString(1)
	crdtPNCounterMu.Lock()
	defer crdtPNCounterMu.Unlock()
	if _, exists := crdtPNCounter[name]; !exists {
		crdtPNCounter[name] = &PNCounterState{P: make(map[string]int64), N: make(map[string]int64)}
	}
	crdtPNCounter[name].N[nodeID]++
	return ctx.WriteOK()
}

func cmdCRDTPNCOUNTERGET(ctx *Context) error {
	if ctx.ArgCount() < 1 {
		return ctx.WriteError(ErrWrongArgCount)
	}
	name := ctx.ArgString(0)
	crdtPNCounterMu.RLock()
	counter, exists := crdtPNCounter[name]
	if !exists {
		crdtPNCounterMu.RUnlock()
		return ctx.WriteInteger(0)
	}
	// Sum the counters under the read lock. The lock used to be released
	// right after the lookup, so the maps were iterated with no lock at all
	// while CRDT.PNCounter.INCR/DECR wrote them under the write lock:
	// concurrent map iteration and map write is a FATAL Go runtime error.
	var pTotal, nTotal int64
	for _, v := range counter.P {
		pTotal += v
	}
	for _, v := range counter.N {
		nTotal += v
	}
	crdtPNCounterMu.RUnlock()
	return ctx.WriteInteger(pTotal - nTotal)
}

var (
	crdtGSet   = make(map[string]map[string]bool)
	crdtGSetMu sync.RWMutex
)

func cmdCRDTGSETADD(ctx *Context) error {
	if ctx.ArgCount() < 2 {
		return ctx.WriteError(ErrWrongArgCount)
	}
	name := ctx.ArgString(0)
	value := ctx.ArgString(1)
	crdtGSetMu.Lock()
	defer crdtGSetMu.Unlock()
	if _, exists := crdtGSet[name]; !exists {
		crdtGSet[name] = make(map[string]bool)
	}
	if crdtGSet[name][value] {
		return ctx.WriteInteger(0)
	}
	crdtGSet[name][value] = true
	return ctx.WriteInteger(1)
}

func cmdCRDTGSETGET(ctx *Context) error {
	if ctx.ArgCount() < 1 {
		return ctx.WriteError(ErrWrongArgCount)
	}
	name := ctx.ArgString(0)
	crdtGSetMu.RLock()
	results := make([]*resp.Value, 0, len(crdtGSet[name]))
	for v := range crdtGSet[name] {
		results = append(results, resp.BulkString(v))
	}
	crdtGSetMu.RUnlock()
	return ctx.WriteArray(results)
}

var (
	crdtORSet   = make(map[string]*orSetState)
	crdtORSetMu sync.RWMutex
	crdtORSetID atomic.Int64
)

type orSetState struct {
	tags       map[string]map[string]struct{}
	tombstones map[string]map[string]struct{}
}

func newORSetState() *orSetState {
	return &orSetState{
		tags:       make(map[string]map[string]struct{}),
		tombstones: make(map[string]map[string]struct{}),
	}
}

func cmdCRDTORSETADD(ctx *Context) error {
	if ctx.ArgCount() < 2 || ctx.ArgCount() > 3 {
		return ctx.WriteError(ErrWrongArgCount)
	}
	name := ctx.ArgString(0)
	value := ctx.ArgString(1)
	var tag string
	if ctx.ArgCount() == 3 {
		tag = ctx.ArgString(2)
		if tag == "" {
			return ctx.WriteError(fmt.Errorf("ERR tag must not be empty"))
		}
	} else {
		tag = "auto-" + strconv.FormatInt(crdtORSetID.Add(1), 10)
	}
	crdtORSetMu.Lock()
	set, exists := crdtORSet[name]
	if !exists {
		set = newORSetState()
		crdtORSet[name] = set
	}
	if _, removed := set.tombstones[value][tag]; !removed {
		if set.tags[value] == nil {
			set.tags[value] = make(map[string]struct{})
		}
		set.tags[value][tag] = struct{}{}
	}
	crdtORSetMu.Unlock()
	return ctx.WriteBulkString(tag)
}

func cmdCRDTORSETREMOVE(ctx *Context) error {
	if ctx.ArgCount() < 2 {
		return ctx.WriteError(ErrWrongArgCount)
	}
	name := ctx.ArgString(0)
	value := ctx.ArgString(1)
	crdtORSetMu.Lock()
	set, exists := crdtORSet[name]
	if !exists {
		crdtORSetMu.Unlock()
		return ctx.WriteInteger(0)
	}
	var observed []string
	if ctx.ArgCount() > 2 {
		for i := 2; i < ctx.ArgCount(); i++ {
			observed = append(observed, ctx.ArgString(i))
		}
	} else {
		for tag := range set.tags[value] {
			observed = append(observed, tag)
		}
	}
	removed := int64(0)
	for _, tag := range observed {
		if set.tombstones[value] == nil {
			set.tombstones[value] = make(map[string]struct{})
		}
		set.tombstones[value][tag] = struct{}{}
		if _, ok := set.tags[value][tag]; ok {
			delete(set.tags[value], tag)
			removed++
		}
	}
	if len(set.tags[value]) == 0 {
		delete(set.tags, value)
	}
	crdtORSetMu.Unlock()
	return ctx.WriteInteger(removed)
}

func cmdCRDTORSETGET(ctx *Context) error {
	if ctx.ArgCount() < 1 {
		return ctx.WriteError(ErrWrongArgCount)
	}
	name := ctx.ArgString(0)
	crdtORSetMu.RLock()
	set, exists := crdtORSet[name]
	values := make([]string, 0)
	if exists {
		for v := range set.tags {
			values = append(values, v)
		}
	}
	crdtORSetMu.RUnlock()
	sort.Strings(values)
	results := make([]*resp.Value, len(values))
	for i, v := range values {
		results[i] = resp.BulkString(v)
	}
	return ctx.WriteArray(results)
}

func cmdCRDTORSETTAGS(ctx *Context) error {
	if ctx.ArgCount() != 2 {
		return ctx.WriteError(ErrWrongArgCount)
	}
	name := ctx.ArgString(0)
	value := ctx.ArgString(1)
	crdtORSetMu.RLock()
	tags := make([]string, 0)
	if set, exists := crdtORSet[name]; exists {
		for tag := range set.tags[value] {
			tags = append(tags, tag)
		}
	}
	crdtORSetMu.RUnlock()
	sort.Strings(tags)
	results := make([]*resp.Value, len(tags))
	for i, t := range tags {
		results[i] = resp.BulkString(t)
	}
	return ctx.WriteArray(results)
}

var (
	dedupSet   = make(map[string]int64)
	dedupSetMu sync.RWMutex
)

func cmdDEDUPADD(ctx *Context) error {
	if ctx.ArgCount() < 2 {
		return ctx.WriteError(ErrWrongArgCount)
	}
	key := ctx.ArgString(0)
	id := ctx.ArgString(1)
	ttl := int64(3600000)
	if ctx.ArgCount() >= 3 {
		ttl = parseInt64(ctx.ArgString(2))
	}
	dedupSetMu.Lock()
	defer dedupSetMu.Unlock()
	fullKey := key + ":" + id
	if _, exists := dedupSet[fullKey]; exists {
		return ctx.WriteInteger(0)
	}
	dedupSet[fullKey] = time.Now().UnixMilli() + ttl
	return ctx.WriteInteger(1)
}

func cmdDEDUPCHECK(ctx *Context) error {
	if ctx.ArgCount() < 2 {
		return ctx.WriteError(ErrWrongArgCount)
	}
	key := ctx.ArgString(0)
	id := ctx.ArgString(1)
	dedupSetMu.RLock()
	fullKey := key + ":" + id
	if exp, exists := dedupSet[fullKey]; exists {
		if time.Now().UnixMilli() < exp {
			reply := int64(1)
			dedupSetMu.RUnlock()
			return ctx.WriteInteger(reply)
		}
	}
	reply := int64(0)
	dedupSetMu.RUnlock()
	return ctx.WriteInteger(reply)
}

func cmdDEDUPEXPIRE(ctx *Context) error {
	dedupSetMu.Lock()
	defer dedupSetMu.Unlock()
	now := time.Now().UnixMilli()
	count := 0
	for k, exp := range dedupSet {
		if now >= exp {
			delete(dedupSet, k)
			count++
		}
	}
	return ctx.WriteInteger(int64(count))
}

func cmdDEDUPCLEAR(ctx *Context) error {
	dedupSetMu.Lock()
	defer dedupSetMu.Unlock()
	count := len(dedupSet)
	dedupSet = make(map[string]int64)
	return ctx.WriteInteger(int64(count))
}

var (
	deadlines   = make(map[string]*Deadline)
	deadlinesMu sync.RWMutex
)

type Deadline struct {
	ID        string
	ExpiresAt int64
	Callback  string
}

func cmdDEADLINESET(ctx *Context) error {
	if ctx.ArgCount() < 2 {
		return ctx.WriteError(ErrWrongArgCount)
	}
	id := ctx.ArgString(0)
	ttlMs := parseInt64(ctx.ArgString(1))
	callback := ""
	if ctx.ArgCount() >= 3 {
		callback = ctx.ArgString(2)
	}
	deadlinesMu.Lock()
	deadlines[id] = &Deadline{ID: id, ExpiresAt: time.Now().UnixMilli() + ttlMs, Callback: callback}
	deadlinesMu.Unlock()
	return ctx.WriteOK()
}

func cmdDEADLINECHECK(ctx *Context) error {
	if ctx.ArgCount() < 1 {
		return ctx.WriteError(ErrWrongArgCount)
	}
	id := ctx.ArgString(0)
	deadlinesMu.RLock()
	d, exists := deadlines[id]
	deadlinesMu.RUnlock()
	if !exists {
		return ctx.WriteInteger(-1)
	}
	remaining := d.ExpiresAt - time.Now().UnixMilli()
	if remaining <= 0 {
		return ctx.WriteInteger(0)
	}
	return ctx.WriteInteger(remaining)
}

func cmdDEADLINECANCEL(ctx *Context) error {
	if ctx.ArgCount() < 1 {
		return ctx.WriteError(ErrWrongArgCount)
	}
	id := ctx.ArgString(0)
	deadlinesMu.Lock()
	defer deadlinesMu.Unlock()
	if _, exists := deadlines[id]; !exists {
		return ctx.WriteInteger(0)
	}
	delete(deadlines, id)
	return ctx.WriteInteger(1)
}

func cmdDEADLINELIST(ctx *Context) error {
	deadlinesMu.RLock()
	results := make([]*resp.Value, 0)
	for id := range deadlines {
		results = append(results, resp.BulkString(id))
	}
	reply := results
	deadlinesMu.RUnlock()
	return ctx.WriteArray(reply)
}

func cmdSANITIZESTRING(ctx *Context) error {
	if ctx.ArgCount() < 1 {
		return ctx.WriteError(ErrWrongArgCount)
	}
	input := ctx.ArgString(0)
	result := make([]byte, 0)
	for _, c := range input {
		if (c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z') || (c >= '0' && c <= '9') || c == ' ' || c == '-' || c == '_' {
			result = append(result, byte(c))
		}
	}
	return ctx.WriteBulkString(string(result))
}

func cmdSANITIZEHTML(ctx *Context) error {
	if ctx.ArgCount() < 1 {
		return ctx.WriteError(ErrWrongArgCount)
	}
	input := ctx.ArgString(0)
	result := ""
	for _, c := range input {
		switch c {
		case '<':
			result += "&lt;"
		case '>':
			result += "&gt;"
		case '&':
			result += "&amp;"
		case '"':
			result += "&quot;"
		case '\'':
			result += "&#39;"
		default:
			result += string(c)
		}
	}
	return ctx.WriteBulkString(result)
}

func cmdSANITIZEJSON(ctx *Context) error {
	if ctx.ArgCount() < 1 {
		return ctx.WriteError(ErrWrongArgCount)
	}
	input := ctx.ArgString(0)
	result := ""
	for _, c := range input {
		switch c {
		case '"':
			result += "\\\""
		case '\\':
			result += "\\\\"
		case '\n':
			result += "\\n"
		case '\r':
			result += "\\r"
		case '\t':
			result += "\\t"
		default:
			result += string(c)
		}
	}
	return ctx.WriteBulkString(result)
}

func cmdSANITIZESQL(ctx *Context) error {
	if ctx.ArgCount() < 1 {
		return ctx.WriteError(ErrWrongArgCount)
	}
	input := ctx.ArgString(0)
	result := ""
	for _, c := range input {
		switch c {
		case '\'':
			result += "''"
		case '\\':
			result += "\\\\"
		default:
			result += string(c)
		}
	}
	return ctx.WriteBulkString(result)
}

func cmdMASKCARD(ctx *Context) error {
	if ctx.ArgCount() < 1 {
		return ctx.WriteError(ErrWrongArgCount)
	}
	card := ctx.ArgString(0)
	if len(card) < 4 {
		return ctx.WriteBulkString("****")
	}
	return ctx.WriteBulkString("****-****-****-" + card[len(card)-4:])
}

func cmdMASKEMAIL(ctx *Context) error {
	if ctx.ArgCount() < 1 {
		return ctx.WriteError(ErrWrongArgCount)
	}
	email := ctx.ArgString(0)
	atIdx := -1
	for i, c := range email {
		if c == '@' {
			atIdx = i
			break
		}
	}
	if atIdx <= 0 {
		return ctx.WriteBulkString("***@***.***")
	}
	masked := "***" + email[atIdx:]
	return ctx.WriteBulkString(masked)
}

func cmdMASKPHONE(ctx *Context) error {
	if ctx.ArgCount() < 1 {
		return ctx.WriteError(ErrWrongArgCount)
	}
	phone := ctx.ArgString(0)
	if len(phone) < 4 {
		return ctx.WriteBulkString("****")
	}
	return ctx.WriteBulkString("***-***-" + phone[len(phone)-4:])
}

func cmdMASKIP(ctx *Context) error {
	if ctx.ArgCount() < 1 {
		return ctx.WriteError(ErrWrongArgCount)
	}
	ip := ctx.ArgString(0)
	parts := splitBy(ip, '.')
	if len(parts) != 4 {
		return ctx.WriteBulkString("***.***.***.***")
	}
	return ctx.WriteBulkString(parts[0] + ".***.***.***")
}

func splitBy(s string, sep byte) []string {
	parts := make([]string, 0)
	start := 0
	for i := 0; i < len(s); i++ {
		if s[i] == sep {
			parts = append(parts, s[start:i])
			start = i + 1
		}
	}
	parts = append(parts, s[start:])
	return parts
}

var (
	gateways   = make(map[string]*Gateway)
	gatewaysMu sync.RWMutex
)

type Gateway struct {
	ID        string
	Name      string
	Routes    map[string]string
	CreatedAt int64
}

func cmdGATEWAYCREATE(ctx *Context) error {
	if ctx.ArgCount() < 1 {
		return ctx.WriteError(ErrWrongArgCount)
	}
	name := ctx.ArgString(0)
	id := generateUUID()
	gatewaysMu.Lock()
	gateways[id] = &Gateway{ID: id, Name: name, Routes: make(map[string]string), CreatedAt: time.Now().UnixMilli()}
	gatewaysMu.Unlock()
	return ctx.WriteBulkString(id)
}

func cmdGATEWAYDELETE(ctx *Context) error {
	if ctx.ArgCount() < 1 {
		return ctx.WriteError(ErrWrongArgCount)
	}
	id := ctx.ArgString(0)
	gatewaysMu.Lock()
	defer gatewaysMu.Unlock()
	if _, exists := gateways[id]; !exists {
		return ctx.WriteInteger(0)
	}
	delete(gateways, id)
	return ctx.WriteInteger(1)
}

func cmdGATEWAYROUTE(ctx *Context) error {
	if ctx.ArgCount() < 3 {
		return ctx.WriteError(ErrWrongArgCount)
	}
	id := ctx.ArgString(0)
	pattern := ctx.ArgString(1)
	target := ctx.ArgString(2)
	gatewaysMu.Lock()
	defer gatewaysMu.Unlock()
	gw, exists := gateways[id]
	if !exists {
		return ctx.WriteError(fmt.Errorf("ERR gateway not found"))
	}
	gw.Routes[pattern] = target
	return ctx.WriteOK()
}

func cmdGATEWAYLIST(ctx *Context) error {
	gatewaysMu.RLock()
	results := make([]*resp.Value, 0)
	for id, gw := range gateways {
		results = append(results, resp.ArrayValue([]*resp.Value{
			resp.BulkString("id"), resp.BulkString(id),
			resp.BulkString("name"), resp.BulkString(gw.Name),
		}))
	}
	reply := results
	gatewaysMu.RUnlock()
	return ctx.WriteArray(reply)
}

var (
	thresholds   = make(map[string]int64)
	thresholdsMu sync.RWMutex
)

func cmdTHRESHOLDSET(ctx *Context) error {
	if ctx.ArgCount() < 2 {
		return ctx.WriteError(ErrWrongArgCount)
	}
	name := ctx.ArgString(0)
	value := parseInt64(ctx.ArgString(1))
	thresholdsMu.Lock()
	thresholds[name] = value
	thresholdsMu.Unlock()
	return ctx.WriteOK()
}

func cmdTHRESHOLDCHECK(ctx *Context) error {
	if ctx.ArgCount() < 2 {
		return ctx.WriteError(ErrWrongArgCount)
	}
	name := ctx.ArgString(0)
	value := parseInt64(ctx.ArgString(1))
	thresholdsMu.RLock()
	threshold, exists := thresholds[name]
	thresholdsMu.RUnlock()
	if !exists {
		return ctx.WriteInteger(0)
	}
	if value >= threshold {
		return ctx.WriteInteger(1)
	}
	return ctx.WriteInteger(0)
}

func cmdTHRESHOLDLIST(ctx *Context) error {
	thresholdsMu.RLock()
	results := make([]*resp.Value, 0)
	for name, value := range thresholds {
		results = append(results, resp.ArrayValue([]*resp.Value{
			resp.BulkString("name"), resp.BulkString(name),
			resp.BulkString("value"), resp.IntegerValue(value),
		}))
	}
	reply := results
	thresholdsMu.RUnlock()
	return ctx.WriteArray(reply)
}

func cmdTHRESHOLDDELETE(ctx *Context) error {
	if ctx.ArgCount() < 1 {
		return ctx.WriteError(ErrWrongArgCount)
	}
	name := ctx.ArgString(0)
	thresholdsMu.Lock()
	defer thresholdsMu.Unlock()
	if _, exists := thresholds[name]; !exists {
		return ctx.WriteInteger(0)
	}
	delete(thresholds, name)
	return ctx.WriteInteger(1)
}

var (
	switches   = make(map[string]bool)
	switchesMu sync.RWMutex
)

func cmdSWITCHSTATE(ctx *Context) error {
	if ctx.ArgCount() < 1 {
		return ctx.WriteError(ErrWrongArgCount)
	}
	name := ctx.ArgString(0)
	switchesMu.RLock()
	state, exists := switches[name]
	switchesMu.RUnlock()
	if !exists {
		return ctx.WriteInteger(0)
	}
	if state {
		return ctx.WriteInteger(1)
	}
	return ctx.WriteInteger(0)
}

func cmdSWITCHTOGGLE(ctx *Context) error {
	if ctx.ArgCount() < 1 {
		return ctx.WriteError(ErrWrongArgCount)
	}
	name := ctx.ArgString(0)
	switchesMu.Lock()
	defer switchesMu.Unlock()
	switches[name] = !switches[name]
	if switches[name] {
		return ctx.WriteInteger(1)
	}
	return ctx.WriteInteger(0)
}

func cmdSWITCHON(ctx *Context) error {
	if ctx.ArgCount() < 1 {
		return ctx.WriteError(ErrWrongArgCount)
	}
	name := ctx.ArgString(0)
	switchesMu.Lock()
	switches[name] = true
	switchesMu.Unlock()
	return ctx.WriteOK()
}

func cmdSWITCHOFF(ctx *Context) error {
	if ctx.ArgCount() < 1 {
		return ctx.WriteError(ErrWrongArgCount)
	}
	name := ctx.ArgString(0)
	switchesMu.Lock()
	switches[name] = false
	switchesMu.Unlock()
	return ctx.WriteOK()
}

func cmdSWITCHLIST(ctx *Context) error {
	switchesMu.RLock()
	results := make([]*resp.Value, 0)
	for name, state := range switches {
		stateVal := "off"
		if state {
			stateVal = "on"
		}
		results = append(results, resp.ArrayValue([]*resp.Value{
			resp.BulkString("name"), resp.BulkString(name),
			resp.BulkString("state"), resp.BulkString(stateVal),
		}))
	}
	reply := results
	switchesMu.RUnlock()
	return ctx.WriteArray(reply)
}

var (
	bookmarks   = make(map[string]*Bookmark)
	bookmarksMu sync.RWMutex
)

type Bookmark struct {
	Key       string
	Value     string
	CreatedAt int64
}

func cmdBOOKMARKSET(ctx *Context) error {
	if ctx.ArgCount() < 2 {
		return ctx.WriteError(ErrWrongArgCount)
	}
	key := ctx.ArgString(0)
	value := ctx.ArgString(1)
	bookmarksMu.Lock()
	bookmarks[key] = &Bookmark{Key: key, Value: value, CreatedAt: time.Now().UnixMilli()}
	bookmarksMu.Unlock()
	return ctx.WriteOK()
}

func cmdBOOKMARKGET(ctx *Context) error {
	if ctx.ArgCount() < 1 {
		return ctx.WriteError(ErrWrongArgCount)
	}
	key := ctx.ArgString(0)
	bookmarksMu.RLock()
	bookmark, exists := bookmarks[key]
	bookmarksMu.RUnlock()
	if !exists {
		return ctx.WriteNull()
	}
	return ctx.WriteBulkString(bookmark.Value)
}

func cmdBOOKMARKDELETE(ctx *Context) error {
	if ctx.ArgCount() < 1 {
		return ctx.WriteError(ErrWrongArgCount)
	}
	key := ctx.ArgString(0)
	bookmarksMu.Lock()
	defer bookmarksMu.Unlock()
	if _, exists := bookmarks[key]; !exists {
		return ctx.WriteInteger(0)
	}
	delete(bookmarks, key)
	return ctx.WriteInteger(1)
}

func cmdBOOKMARKLIST(ctx *Context) error {
	bookmarksMu.RLock()
	results := make([]*resp.Value, 0)
	for key := range bookmarks {
		results = append(results, resp.BulkString(key))
	}
	reply := results
	bookmarksMu.RUnlock()
	return ctx.WriteArray(reply)
}

var (
	routes   = make(map[string]map[string]string)
	routesMu sync.RWMutex
)

func cmdROUTEADD(ctx *Context) error {
	if ctx.ArgCount() < 3 {
		return ctx.WriteError(ErrWrongArgCount)
	}
	name := ctx.ArgString(0)
	pattern := ctx.ArgString(1)
	target := ctx.ArgString(2)
	routesMu.Lock()
	defer routesMu.Unlock()
	if _, exists := routes[name]; !exists {
		routes[name] = make(map[string]string)
	}
	routes[name][pattern] = target
	return ctx.WriteOK()
}

func cmdROUTEREMOVE(ctx *Context) error {
	if ctx.ArgCount() < 2 {
		return ctx.WriteError(ErrWrongArgCount)
	}
	name := ctx.ArgString(0)
	pattern := ctx.ArgString(1)
	routesMu.Lock()
	defer routesMu.Unlock()
	if r, exists := routes[name]; exists {
		if _, ok := r[pattern]; ok {
			delete(r, pattern)
			return ctx.WriteInteger(1)
		}
	}
	return ctx.WriteInteger(0)
}

func cmdROUTEMATCH(ctx *Context) error {
	if ctx.ArgCount() < 2 {
		return ctx.WriteError(ErrWrongArgCount)
	}
	name := ctx.ArgString(0)
	path := ctx.ArgString(1)
	routesMu.RLock()
	r, exists := routes[name]
	routesMu.RUnlock()
	if !exists {
		return ctx.WriteNull()
	}
	for pattern, target := range r {
		if matchPattern(path, pattern) {
			return ctx.WriteBulkString(target)
		}
	}
	return ctx.WriteNull()
}

func matchPatternX(path, pattern string) bool {
	if pattern == "*" || pattern == path {
		return true
	}
	return false
}

func cmdROUTELIST(ctx *Context) error {
	if ctx.ArgCount() < 1 {
		return ctx.WriteError(ErrWrongArgCount)
	}
	name := ctx.ArgString(0)
	routesMu.RLock()
	r, exists := routes[name]
	routesMu.RUnlock()
	if !exists {
		return ctx.WriteArray([]*resp.Value{})
	}
	results := make([]*resp.Value, 0)
	for pattern, target := range r {
		results = append(results, resp.ArrayValue([]*resp.Value{
			resp.BulkString("pattern"), resp.BulkString(pattern),
			resp.BulkString("target"), resp.BulkString(target),
		}))
	}
	return ctx.WriteArray(results)
}

var (
	ghosts   = make(map[string]*Ghost)
	ghostsMu sync.RWMutex
)

type Ghost struct {
	ID   string
	Data []string
}

func cmdGHOSTCREATE(ctx *Context) error {
	if ctx.ArgCount() < 1 {
		return ctx.WriteError(ErrWrongArgCount)
	}
	id := ctx.ArgString(0)
	ghostsMu.Lock()
	ghosts[id] = &Ghost{ID: id, Data: make([]string, 0)}
	ghostsMu.Unlock()
	return ctx.WriteOK()
}

func cmdGHOSTWRITE(ctx *Context) error {
	if ctx.ArgCount() < 2 {
		return ctx.WriteError(ErrWrongArgCount)
	}
	id := ctx.ArgString(0)
	data := ctx.ArgString(1)
	ghostsMu.Lock()
	defer ghostsMu.Unlock()
	if g, exists := ghosts[id]; exists {
		g.Data = append(g.Data, data)
		return ctx.WriteOK()
	}
	return ctx.WriteError(fmt.Errorf("ERR ghost not found"))
}

func cmdGHOSTREAD(ctx *Context) error {
	if ctx.ArgCount() < 1 {
		return ctx.WriteError(ErrWrongArgCount)
	}
	id := ctx.ArgString(0)
	ghostsMu.RLock()
	g, exists := ghosts[id]
	if !exists {
		reply := []*resp.Value{}
		ghostsMu.RUnlock()
		return ctx.WriteArray(reply)
	}
	results := make([]*resp.Value, len(g.Data))
	for i, d := range g.Data {
		results[i] = resp.BulkString(d)
	}
	reply := results
	ghostsMu.RUnlock()
	return ctx.WriteArray(reply)
}

func cmdGHOSTDELETE(ctx *Context) error {
	if ctx.ArgCount() < 1 {
		return ctx.WriteError(ErrWrongArgCount)
	}
	id := ctx.ArgString(0)
	ghostsMu.Lock()
	defer ghostsMu.Unlock()
	if _, exists := ghosts[id]; !exists {
		return ctx.WriteInteger(0)
	}
	delete(ghosts, id)
	return ctx.WriteInteger(1)
}

type Probe struct {
	ID      string
	Name    string
	Target  string
	Results []string
}

var (
	grids   = make(map[string]*Grid)
	gridsMu sync.RWMutex
)

type Grid struct {
	Name   string
	Width  int
	Height int
	Data   map[string]string
}

func cmdGRIDCREATE(ctx *Context) error {
	if ctx.ArgCount() < 3 {
		return ctx.WriteError(ErrWrongArgCount)
	}
	name := ctx.ArgString(0)
	width := int(parseInt64(ctx.ArgString(1)))
	height := int(parseInt64(ctx.ArgString(2)))
	gridsMu.Lock()
	grids[name] = &Grid{Name: name, Width: width, Height: height, Data: make(map[string]string)}
	gridsMu.Unlock()
	return ctx.WriteOK()
}

func cmdGRIDSET(ctx *Context) error {
	if ctx.ArgCount() < 4 {
		return ctx.WriteError(ErrWrongArgCount)
	}
	name := ctx.ArgString(0)
	x := ctx.ArgString(1)
	y := ctx.ArgString(2)
	value := ctx.ArgString(3)
	gridsMu.Lock()
	defer gridsMu.Unlock()
	if g, exists := grids[name]; exists {
		key := x + "," + y
		g.Data[key] = value
		return ctx.WriteOK()
	}
	return ctx.WriteError(fmt.Errorf("ERR grid not found"))
}

func cmdGRIDGET(ctx *Context) error {
	if ctx.ArgCount() < 3 {
		return ctx.WriteError(ErrWrongArgCount)
	}
	name := ctx.ArgString(0)
	x := ctx.ArgString(1)
	y := ctx.ArgString(2)
	gridsMu.RLock()
	g, exists := grids[name]
	if !exists {
		gridsMu.RUnlock()
		return ctx.WriteNull()
	}
	key := x + "," + y
	// Read the cell under the read lock. The lock used to be released right
	// after the lookup, so the map was read with no lock at all while
	// GRID.SET wrote it under the write lock: concurrent map read and map
	// write is a FATAL Go runtime error.
	val, ok := g.Data[key]
	gridsMu.RUnlock()
	if ok {
		return ctx.WriteBulkString(val)
	}
	return ctx.WriteNull()
}

func cmdGRIDDELETE(ctx *Context) error {
	if ctx.ArgCount() < 1 {
		return ctx.WriteError(ErrWrongArgCount)
	}
	name := ctx.ArgString(0)
	gridsMu.Lock()
	defer gridsMu.Unlock()
	if _, exists := grids[name]; !exists {
		return ctx.WriteInteger(0)
	}
	delete(grids, name)
	return ctx.WriteInteger(1)
}

func cmdGRIDQUERY(ctx *Context) error {
	if ctx.ArgCount() < 1 {
		return ctx.WriteError(ErrWrongArgCount)
	}
	name := ctx.ArgString(0)
	gridsMu.RLock()
	g, exists := grids[name]
	if !exists {
		gridsMu.RUnlock()
		return ctx.WriteArray([]*resp.Value{})
	}
	// Iterate the cells under the read lock. The lock used to be released
	// right after the lookup, so the map was iterated with no lock at all
	// while GRID.SET wrote it under the write lock: concurrent map iteration
	// and map write is a FATAL Go runtime error.
	results := make([]*resp.Value, 0)
	for k, v := range g.Data {
		results = append(results, resp.ArrayValue([]*resp.Value{
			resp.BulkString("coord"), resp.BulkString(k),
			resp.BulkString("value"), resp.BulkString(v),
		}))
	}
	gridsMu.RUnlock()
	return ctx.WriteArray(results)
}

func cmdGRIDCLEAR(ctx *Context) error {
	if ctx.ArgCount() < 1 {
		return ctx.WriteError(ErrWrongArgCount)
	}
	name := ctx.ArgString(0)
	gridsMu.Lock()
	defer gridsMu.Unlock()
	if g, exists := grids[name]; exists {
		g.Data = make(map[string]string)
		return ctx.WriteOK()
	}
	return ctx.WriteError(fmt.Errorf("ERR grid not found"))
}

var (
	tapes   = make(map[string]*Tape)
	tapesMu sync.RWMutex
)

type Tape struct {
	Name string
	Data []string
	Pos  int
}

func cmdTAPECREATE(ctx *Context) error {
	if ctx.ArgCount() < 1 {
		return ctx.WriteError(ErrWrongArgCount)
	}
	name := ctx.ArgString(0)
	tapesMu.Lock()
	tapes[name] = &Tape{Name: name, Data: make([]string, 0), Pos: 0}
	tapesMu.Unlock()
	return ctx.WriteOK()
}

func cmdTAPEWRITE(ctx *Context) error {
	if ctx.ArgCount() < 2 {
		return ctx.WriteError(ErrWrongArgCount)
	}
	name := ctx.ArgString(0)
	data := ctx.ArgString(1)
	tapesMu.Lock()
	defer tapesMu.Unlock()
	if t, exists := tapes[name]; exists {
		t.Data = append(t.Data, data)
		return ctx.WriteOK()
	}
	return ctx.WriteError(fmt.Errorf("ERR tape not found"))
}

func cmdTAPEREAD(ctx *Context) error {
	if ctx.ArgCount() < 1 {
		return ctx.WriteError(ErrWrongArgCount)
	}
	name := ctx.ArgString(0)
	tapesMu.RLock()
	t, exists := tapes[name]
	if !exists || len(t.Data) == 0 {
		tapesMu.RUnlock()
		return ctx.WriteNull()
	}
	if t.Pos >= len(t.Data) {
		tapesMu.RUnlock()
		return ctx.WriteNull()
	}
	reply := t.Data[t.Pos]
	tapesMu.RUnlock()
	return ctx.WriteBulkString(reply)
}

func cmdTAPESEEK(ctx *Context) error {
	if ctx.ArgCount() < 2 {
		return ctx.WriteError(ErrWrongArgCount)
	}
	name := ctx.ArgString(0)
	pos := int(parseInt64(ctx.ArgString(1)))
	tapesMu.Lock()
	defer tapesMu.Unlock()
	if t, exists := tapes[name]; exists {
		if pos >= 0 && pos < len(t.Data) {
			t.Pos = pos
			return ctx.WriteOK()
		}
	}
	return ctx.WriteError(fmt.Errorf("ERR tape not found or invalid position"))
}

func cmdTAPEDELETE(ctx *Context) error {
	if ctx.ArgCount() < 1 {
		return ctx.WriteError(ErrWrongArgCount)
	}
	name := ctx.ArgString(0)
	tapesMu.Lock()
	defer tapesMu.Unlock()
	if _, exists := tapes[name]; !exists {
		return ctx.WriteInteger(0)
	}
	delete(tapes, name)
	return ctx.WriteInteger(1)
}

var (
	slices   = make(map[string][]string)
	slicesMu sync.RWMutex
)

func cmdSLICECREATE(ctx *Context) error {
	if ctx.ArgCount() < 1 {
		return ctx.WriteError(ErrWrongArgCount)
	}
	name := ctx.ArgString(0)
	slicesMu.Lock()
	slices[name] = make([]string, 0)
	slicesMu.Unlock()
	return ctx.WriteOK()
}

func cmdSLICEAPPEND(ctx *Context) error {
	if ctx.ArgCount() < 2 {
		return ctx.WriteError(ErrWrongArgCount)
	}
	name := ctx.ArgString(0)
	for i := 1; i < ctx.ArgCount(); i++ {
		value := ctx.ArgString(i)
		slicesMu.Lock()
		slices[name] = append(slices[name], value)
		slicesMu.Unlock()
	}
	return ctx.WriteOK()
}

func cmdSLICEGET(ctx *Context) error {
	if ctx.ArgCount() < 1 {
		return ctx.WriteError(ErrWrongArgCount)
	}
	name := ctx.ArgString(0)
	slicesMu.RLock()
	slice, exists := slices[name]
	slicesMu.RUnlock()
	if !exists {
		return ctx.WriteArray([]*resp.Value{})
	}
	results := make([]*resp.Value, len(slice))
	for i, v := range slice {
		results[i] = resp.BulkString(v)
	}
	return ctx.WriteArray(results)
}

func cmdSLICEDELETE(ctx *Context) error {
	if ctx.ArgCount() < 1 {
		return ctx.WriteError(ErrWrongArgCount)
	}
	name := ctx.ArgString(0)
	slicesMu.Lock()
	defer slicesMu.Unlock()
	if _, exists := slices[name]; !exists {
		return ctx.WriteInteger(0)
	}
	delete(slices, name)
	return ctx.WriteInteger(1)
}

var (
	rollupsX    = make(map[string]*RollupX)
	rollupsXMux sync.RWMutex
)

type RollupX struct {
	Name string
	Data []float64
}

func cmdROLLUPXCREATE(ctx *Context) error {
	if ctx.ArgCount() < 1 {
		return ctx.WriteError(ErrWrongArgCount)
	}
	name := ctx.ArgString(0)
	rollupsXMux.Lock()
	rollupsX[name] = &RollupX{Name: name, Data: make([]float64, 0)}
	rollupsXMux.Unlock()
	return ctx.WriteOK()
}

func cmdROLLUPXADD(ctx *Context) error {
	if ctx.ArgCount() < 2 {
		return ctx.WriteError(ErrWrongArgCount)
	}
	name := ctx.ArgString(0)
	value := parseFloatExt([]byte(ctx.ArgString(1)))
	rollupsXMux.Lock()
	defer rollupsXMux.Unlock()
	if r, exists := rollupsX[name]; exists {
		r.Data = append(r.Data, value)
		return ctx.WriteOK()
	}
	return ctx.WriteError(fmt.Errorf("ERR rollup not found"))
}

func cmdROLLUPXGET(ctx *Context) error {
	if ctx.ArgCount() < 1 {
		return ctx.WriteError(ErrWrongArgCount)
	}
	name := ctx.ArgString(0)
	rollupsXMux.RLock()
	r, exists := rollupsX[name]
	rollupsXMux.RUnlock()
	if !exists || len(r.Data) == 0 {
		return ctx.WriteBulkString("0")
	}
	var sum float64
	for _, v := range r.Data {
		sum += v
	}
	avg := sum / float64(len(r.Data))
	return ctx.WriteBulkString(fmt.Sprintf("%.6f", avg))
}

func cmdROLLUPXDELETE(ctx *Context) error {
	if ctx.ArgCount() < 1 {
		return ctx.WriteError(ErrWrongArgCount)
	}
	name := ctx.ArgString(0)
	rollupsXMux.Lock()
	defer rollupsXMux.Unlock()
	if _, exists := rollupsX[name]; !exists {
		return ctx.WriteInteger(0)
	}
	delete(rollupsX, name)
	return ctx.WriteInteger(1)
}
