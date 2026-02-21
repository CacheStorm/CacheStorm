package command

import (
	"fmt"
	"sync"
	"time"

	"github.com/cachestorm/cachestorm/internal/resp"
)

func RegisterExtraCommands(router *Router) {
	router.Register(&CommandDef{Name: "SWIM.JOIN", Handler: cmdSWIMJOIN})
	router.Register(&CommandDef{Name: "SWIM.LEAVE", Handler: cmdSWIMLEAVE})
	router.Register(&CommandDef{Name: "SWIM.MEMBERS", Handler: cmdSWIMMEMBERS})
	router.Register(&CommandDef{Name: "SWIM.PING", Handler: cmdSWIMPING})
	router.Register(&CommandDef{Name: "SWIM.SUSPECT", Handler: cmdSWIMSUSPECT})

	router.Register(&CommandDef{Name: "GOSSIP.JOIN", Handler: cmdGOSSIPJOIN})
	router.Register(&CommandDef{Name: "GOSSIP.LEAVE", Handler: cmdGOSSIPLEAVE})
	router.Register(&CommandDef{Name: "GOSSIP.BROADCAST", Handler: cmdGOSSIPBROADCAST})
	router.Register(&CommandDef{Name: "GOSSIP.GET", Handler: cmdGOSSIPGET})
	router.Register(&CommandDef{Name: "GOSSIP.MEMBERS", Handler: cmdGOSSIPMEMBERS})

	router.Register(&CommandDef{Name: "ANTI_ENTROPY.SYNC", Handler: cmdANTIENTROPYSYNC})
	router.Register(&CommandDef{Name: "ANTI_ENTROPY.DIFF", Handler: cmdANTIENTROPYDIFF})
	router.Register(&CommandDef{Name: "ANTI_ENTROPY.MERGE", Handler: cmdANTIENTROPYMERGE})
	router.Register(&CommandDef{Name: "ANTI_ENTROPY.STATUS", Handler: cmdANTIENTROPYSTATUS})

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

	router.Register(&CommandDef{Name: "MERKLE.CREATE", Handler: cmdMERKLECREATE})
	router.Register(&CommandDef{Name: "MERKLE.ADD", Handler: cmdMERKLEADD})
	router.Register(&CommandDef{Name: "MERKLE.VERIFY", Handler: cmdMERKLEVERIFY})
	router.Register(&CommandDef{Name: "MERKLE.PROOF", Handler: cmdMERKLEPROOF})
	router.Register(&CommandDef{Name: "MERKLE.ROOT", Handler: cmdMERKLEROOT})

	router.Register(&CommandDef{Name: "RAFT.STATE", Handler: cmdRAFTSTATE})
	router.Register(&CommandDef{Name: "RAFT.LEADER", Handler: cmdRAFTLEADER})
	router.Register(&CommandDef{Name: "RAFT.TERM", Handler: cmdRAFTTERM})
	router.Register(&CommandDef{Name: "RAFT.VOTE", Handler: cmdRAFTVOTE})
	router.Register(&CommandDef{Name: "RAFT.APPEND", Handler: cmdRAFTAPPEND})
	router.Register(&CommandDef{Name: "RAFT.COMMIT", Handler: cmdRAFTCOMMIT})

	router.Register(&CommandDef{Name: "SHARD.MAP", Handler: cmdSHARDMAP})
	router.Register(&CommandDef{Name: "SHARD.MOVE", Handler: cmdSHARDMOVE})
	router.Register(&CommandDef{Name: "SHARD.REBALANCE", Handler: cmdSHARDREBALANCE})
	router.Register(&CommandDef{Name: "SHARD.LIST", Handler: cmdSHARDLIST})
	router.Register(&CommandDef{Name: "SHARD.STATUS", Handler: cmdSHARDSTATUS})

	router.Register(&CommandDef{Name: "COMPRESSION.COMPRESS", Handler: cmdCOMPRESSIONCOMPRESS})
	router.Register(&CommandDef{Name: "COMPRESSION.DECOMPRESS", Handler: cmdCOMPRESSIONDECOMPRESS})
	router.Register(&CommandDef{Name: "COMPRESSION.INFO", Handler: cmdCOMPRESSIONINFO})

	router.Register(&CommandDef{Name: "DEDUP.ADD", Handler: cmdDEDUPADD})
	router.Register(&CommandDef{Name: "DEDUP.CHECK", Handler: cmdDEDUPCHECK})
	router.Register(&CommandDef{Name: "DEDUP.EXPIRE", Handler: cmdDEDUPEXPIRE})
	router.Register(&CommandDef{Name: "DEDUP.CLEAR", Handler: cmdDEDUPCLEAR})

	router.Register(&CommandDef{Name: "BATCH.SUBMIT", Handler: cmdBATCHSUBMIT})
	router.Register(&CommandDef{Name: "BATCH.STATUS", Handler: cmdBATCHSTATUS})
	router.Register(&CommandDef{Name: "BATCH.CANCEL", Handler: cmdBATCHCANCEL})
	router.Register(&CommandDef{Name: "BATCH.LIST", Handler: cmdBATCHLIST})

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
	router.Register(&CommandDef{Name: "GATEWAY.METRICS", Handler: cmdGATEWAYMETRICS})

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

	router.Register(&CommandDef{Name: "REPLAYX.START", Handler: cmdREPLAYXSTART})
	router.Register(&CommandDef{Name: "REPLAYX.STOP", Handler: cmdREPLAYXSTOP})
	router.Register(&CommandDef{Name: "REPLAYX.PAUSE", Handler: cmdREPLAYXPAUSE})
	router.Register(&CommandDef{Name: "REPLAYX.SPEED", Handler: cmdREPLAYXSPEED})

	router.Register(&CommandDef{Name: "ROUTE.ADD", Handler: cmdROUTEADD})
	router.Register(&CommandDef{Name: "ROUTE.REMOVE", Handler: cmdROUTEREMOVE})
	router.Register(&CommandDef{Name: "ROUTE.MATCH", Handler: cmdROUTEMATCH})
	router.Register(&CommandDef{Name: "ROUTE.LIST", Handler: cmdROUTELIST})

	router.Register(&CommandDef{Name: "GHOST.CREATE", Handler: cmdGHOSTCREATE})
	router.Register(&CommandDef{Name: "GHOST.WRITE", Handler: cmdGHOSTWRITE})
	router.Register(&CommandDef{Name: "GHOST.READ", Handler: cmdGHOSTREAD})
	router.Register(&CommandDef{Name: "GHOST.DELETE", Handler: cmdGHOSTDELETE})

	router.Register(&CommandDef{Name: "PROBE.CREATE", Handler: cmdPROBECREATE})
	router.Register(&CommandDef{Name: "PROBE.DELETE", Handler: cmdPROBEDELETE})
	router.Register(&CommandDef{Name: "PROBE.RUN", Handler: cmdPROBERUN})
	router.Register(&CommandDef{Name: "PROBE.RESULTS", Handler: cmdPROBERESULTS})
	router.Register(&CommandDef{Name: "PROBE.LIST", Handler: cmdPROBELIST})

	router.Register(&CommandDef{Name: "CANARY.CREATE", Handler: cmdCANARYCREATE})
	router.Register(&CommandDef{Name: "CANARY.DELETE", Handler: cmdCANARYDELETE})
	router.Register(&CommandDef{Name: "CANARY.CHECK", Handler: cmdCANARYCHECK})
	router.Register(&CommandDef{Name: "CANARY.STATUS", Handler: cmdCANARYSTATUS})
	router.Register(&CommandDef{Name: "CANARY.LIST", Handler: cmdCANARYLIST})

	router.Register(&CommandDef{Name: "RAGE.TEST", Handler: cmdRAGETEST})
	router.Register(&CommandDef{Name: "RAGE.STOP", Handler: cmdRAGESTOP})
	router.Register(&CommandDef{Name: "RAGE.STATS", Handler: cmdRAGESTATS})
	router.Register(&CommandDef{Name: "RAGE.RESET", Handler: cmdRAGERESET})

	router.Register(&CommandDef{Name: "GRID.CREATE", Handler: cmdGRIDCREATE})
	router.Register(&CommandDef{Name: "GRID.SET", Handler: cmdGRIDSET})
	router.Register(&CommandDef{Name: "GRID.GET", Handler: cmdGRIDGET})
	router.Register(&CommandDef{Name: "GRID.DELETE", Handler: cmdGRIDDELETE})
	router.Register(&CommandDef{Name: "GRID.QUERY", Handler: cmdGRIDQUERY})
	router.Register(&CommandDef{Name: "GRID.CLEAR", Handler: cmdGRIDGET})

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

	router.Register(&CommandDef{Name: "BEACON.START", Handler: cmdBEACONSTART})
	router.Register(&CommandDef{Name: "BEACON.STOP", Handler: cmdBEACONSTOP})
	router.Register(&CommandDef{Name: "BEACON.LIST", Handler: cmdBEACONLIST})
	router.Register(&CommandDef{Name: "BEACON.CHECK", Handler: cmdBEACONCHECK})
}

var (
	swimMembers   = make(map[string]*SwimMember)
	swimMembersMu sync.RWMutex
)

type SwimMember struct {
	ID          string
	Addr        string
	Status      string
	LastSeen    int64
	Incarnation int64
}

func cmdSWIMJOIN(ctx *Context) error {
	if ctx.ArgCount() < 2 {
		return ctx.WriteError(ErrWrongArgCount)
	}
	id := ctx.ArgString(0)
	addr := ctx.ArgString(1)
	swimMembersMu.Lock()
	swimMembers[id] = &SwimMember{ID: id, Addr: addr, Status: "alive", LastSeen: time.Now().UnixMilli(), Incarnation: 0}
	swimMembersMu.Unlock()
	return ctx.WriteOK()
}

func cmdSWIMLEAVE(ctx *Context) error {
	if ctx.ArgCount() < 1 {
		return ctx.WriteError(ErrWrongArgCount)
	}
	id := ctx.ArgString(0)
	swimMembersMu.Lock()
	defer swimMembersMu.Unlock()
	if _, exists := swimMembers[id]; !exists {
		return ctx.WriteInteger(0)
	}
	delete(swimMembers, id)
	return ctx.WriteInteger(1)
}

func cmdSWIMMEMBERS(ctx *Context) error {
	swimMembersMu.RLock()
	defer swimMembersMu.RUnlock()
	results := make([]*resp.Value, 0)
	for _, m := range swimMembers {
		results = append(results, resp.ArrayValue([]*resp.Value{
			resp.BulkString("id"), resp.BulkString(m.ID),
			resp.BulkString("addr"), resp.BulkString(m.Addr),
			resp.BulkString("status"), resp.BulkString(m.Status),
		}))
	}
	return ctx.WriteArray(results)
}

func cmdSWIMPING(ctx *Context) error {
	if ctx.ArgCount() < 1 {
		return ctx.WriteError(ErrWrongArgCount)
	}
	id := ctx.ArgString(0)
	swimMembersMu.Lock()
	defer swimMembersMu.Unlock()
	if m, exists := swimMembers[id]; exists {
		m.LastSeen = time.Now().UnixMilli()
		m.Status = "alive"
		return ctx.WriteOK()
	}
	return ctx.WriteError(fmt.Errorf("ERR member not found"))
}

func cmdSWIMSUSPECT(ctx *Context) error {
	if ctx.ArgCount() < 1 {
		return ctx.WriteError(ErrWrongArgCount)
	}
	id := ctx.ArgString(0)
	swimMembersMu.Lock()
	defer swimMembersMu.Unlock()
	if m, exists := swimMembers[id]; exists {
		m.Status = "suspect"
		return ctx.WriteOK()
	}
	return ctx.WriteError(fmt.Errorf("ERR member not found"))
}

var (
	gossipMembers = make(map[string]*GossipMember)
	gossipData    = make(map[string]string)
	gossipMu      sync.RWMutex
)

type GossipMember struct {
	ID       string
	Addr     string
	LastSeen int64
}

func cmdGOSSIPJOIN(ctx *Context) error {
	if ctx.ArgCount() < 2 {
		return ctx.WriteError(ErrWrongArgCount)
	}
	id := ctx.ArgString(0)
	addr := ctx.ArgString(1)
	gossipMu.Lock()
	gossipMembers[id] = &GossipMember{ID: id, Addr: addr, LastSeen: time.Now().UnixMilli()}
	gossipMu.Unlock()
	return ctx.WriteOK()
}

func cmdGOSSIPLEAVE(ctx *Context) error {
	if ctx.ArgCount() < 1 {
		return ctx.WriteError(ErrWrongArgCount)
	}
	id := ctx.ArgString(0)
	gossipMu.Lock()
	defer gossipMu.Unlock()
	delete(gossipMembers, id)
	return ctx.WriteInteger(1)
}

func cmdGOSSIPBROADCAST(ctx *Context) error {
	if ctx.ArgCount() < 2 {
		return ctx.WriteError(ErrWrongArgCount)
	}
	key := ctx.ArgString(0)
	value := ctx.ArgString(1)
	gossipMu.Lock()
	gossipData[key] = value
	count := len(gossipMembers)
	gossipMu.Unlock()
	return ctx.WriteInteger(int64(count))
}

func cmdGOSSIPGET(ctx *Context) error {
	if ctx.ArgCount() < 1 {
		return ctx.WriteError(ErrWrongArgCount)
	}
	key := ctx.ArgString(0)
	gossipMu.RLock()
	val, exists := gossipData[key]
	gossipMu.RUnlock()
	if !exists {
		return ctx.WriteNull()
	}
	return ctx.WriteBulkString(val)
}

func cmdGOSSIPMEMBERS(ctx *Context) error {
	gossipMu.RLock()
	defer gossipMu.RUnlock()
	results := make([]*resp.Value, 0)
	for id := range gossipMembers {
		results = append(results, resp.BulkString(id))
	}
	return ctx.WriteArray(results)
}

var (
	antiEntropy   = make(map[string]*AntiEntropyState)
	antiEntropyMu sync.RWMutex
)

type AntiEntropyState struct {
	Name     string
	Version  int64
	Checksum string
}

func cmdANTIENTROPYSYNC(ctx *Context) error {
	if ctx.ArgCount() < 2 {
		return ctx.WriteError(ErrWrongArgCount)
	}
	name := ctx.ArgString(0)
	version := parseInt64(ctx.ArgString(1))
	antiEntropyMu.Lock()
	antiEntropy[name] = &AntiEntropyState{Name: name, Version: version, Checksum: generateUUID()[:8]}
	antiEntropyMu.Unlock()
	return ctx.WriteOK()
}

func cmdANTIENTROPYDIFF(ctx *Context) error {
	if ctx.ArgCount() < 2 {
		return ctx.WriteError(ErrWrongArgCount)
	}
	name := ctx.ArgString(0)
	version := parseInt64(ctx.ArgString(1))
	antiEntropyMu.RLock()
	state, exists := antiEntropy[name]
	antiEntropyMu.RUnlock()
	if !exists {
		return ctx.WriteArray([]*resp.Value{resp.BulkString("sync_needed"), resp.IntegerValue(1)})
	}
	if state.Version < version {
		return ctx.WriteArray([]*resp.Value{resp.BulkString("sync_needed"), resp.IntegerValue(1)})
	}
	return ctx.WriteArray([]*resp.Value{resp.BulkString("sync_needed"), resp.IntegerValue(0)})
}

func cmdANTIENTROPYMERGE(ctx *Context) error {
	if ctx.ArgCount() < 2 {
		return ctx.WriteError(ErrWrongArgCount)
	}
	name := ctx.ArgString(0)
	version := parseInt64(ctx.ArgString(1))
	antiEntropyMu.Lock()
	defer antiEntropyMu.Unlock()
	if state, exists := antiEntropy[name]; exists {
		if version > state.Version {
			state.Version = version
			state.Checksum = generateUUID()[:8]
		}
	} else {
		antiEntropy[name] = &AntiEntropyState{Name: name, Version: version, Checksum: generateUUID()[:8]}
	}
	return ctx.WriteOK()
}

func cmdANTIENTROPYSTATUS(ctx *Context) error {
	if ctx.ArgCount() < 1 {
		return ctx.WriteError(ErrWrongArgCount)
	}
	name := ctx.ArgString(0)
	antiEntropyMu.RLock()
	state, exists := antiEntropy[name]
	antiEntropyMu.RUnlock()
	if !exists {
		return ctx.WriteError(fmt.Errorf("ERR not found"))
	}
	return ctx.WriteArray([]*resp.Value{
		resp.BulkString("name"), resp.BulkString(state.Name),
		resp.BulkString("version"), resp.IntegerValue(state.Version),
		resp.BulkString("checksum"), resp.BulkString(state.Checksum),
	})
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
	vectorClocksMu.RUnlock()
	if !exists1 || !exists2 {
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
	if !v1Greater && !v2Greater {
		return ctx.WriteBulkString("equal")
	}
	if v1Greater && !v2Greater {
		return ctx.WriteBulkString("before")
	}
	if v2Greater && !v1Greater {
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
	crdtLWWMu.RUnlock()
	if !exists {
		return ctx.WriteNull()
	}
	return ctx.WriteBulkString(entry.Value)
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
	counter, exists := crdtGCounter[name]
	crdtGCounterMu.RUnlock()
	if !exists {
		return ctx.WriteInteger(0)
	}
	var total int64
	for _, v := range counter {
		total += v
	}
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
	crdtPNCounterMu.RUnlock()
	if !exists {
		return ctx.WriteInteger(0)
	}
	var pTotal, nTotal int64
	for _, v := range counter.P {
		pTotal += v
	}
	for _, v := range counter.N {
		nTotal += v
	}
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
	set, exists := crdtGSet[name]
	crdtGSetMu.RUnlock()
	if !exists {
		return ctx.WriteArray([]*resp.Value{})
	}
	results := make([]*resp.Value, 0)
	for v := range set {
		results = append(results, resp.BulkString(v))
	}
	return ctx.WriteArray(results)
}

var (
	crdtORSet   = make(map[string]map[string]bool)
	crdtORSetMu sync.RWMutex
)

func cmdCRDTORSETADD(ctx *Context) error {
	if ctx.ArgCount() < 2 {
		return ctx.WriteError(ErrWrongArgCount)
	}
	name := ctx.ArgString(0)
	value := ctx.ArgString(1)
	crdtORSetMu.Lock()
	defer crdtORSetMu.Unlock()
	if _, exists := crdtORSet[name]; !exists {
		crdtORSet[name] = make(map[string]bool)
	}
	crdtORSet[name][value] = true
	return ctx.WriteInteger(1)
}

func cmdCRDTORSETREMOVE(ctx *Context) error {
	if ctx.ArgCount() < 2 {
		return ctx.WriteError(ErrWrongArgCount)
	}
	name := ctx.ArgString(0)
	value := ctx.ArgString(1)
	crdtORSetMu.Lock()
	defer crdtORSetMu.Unlock()
	if _, exists := crdtORSet[name]; !exists {
		return ctx.WriteInteger(0)
	}
	if crdtORSet[name][value] {
		delete(crdtORSet[name], value)
		return ctx.WriteInteger(1)
	}
	return ctx.WriteInteger(0)
}

func cmdCRDTORSETGET(ctx *Context) error {
	if ctx.ArgCount() < 1 {
		return ctx.WriteError(ErrWrongArgCount)
	}
	name := ctx.ArgString(0)
	crdtORSetMu.RLock()
	set, exists := crdtORSet[name]
	crdtORSetMu.RUnlock()
	if !exists {
		return ctx.WriteArray([]*resp.Value{})
	}
	results := make([]*resp.Value, 0)
	for v := range set {
		results = append(results, resp.BulkString(v))
	}
	return ctx.WriteArray(results)
}

var (
	merkleTrees   = make(map[string]*MerkleTree)
	merkleTreesMu sync.RWMutex
)

type MerkleTree struct {
	Name  string
	Root  string
	Nodes map[string]string
}

func cmdMERKLECREATE(ctx *Context) error {
	if ctx.ArgCount() < 1 {
		return ctx.WriteError(ErrWrongArgCount)
	}
	name := ctx.ArgString(0)
	merkleTreesMu.Lock()
	merkleTrees[name] = &MerkleTree{Name: name, Root: "", Nodes: make(map[string]string)}
	merkleTreesMu.Unlock()
	return ctx.WriteOK()
}

func cmdMERKLEADD(ctx *Context) error {
	if ctx.ArgCount() < 2 {
		return ctx.WriteError(ErrWrongArgCount)
	}
	name := ctx.ArgString(0)
	data := ctx.ArgString(1)
	hash := generateUUID()[:16]
	merkleTreesMu.Lock()
	defer merkleTreesMu.Unlock()
	tree, exists := merkleTrees[name]
	if !exists {
		return ctx.WriteError(fmt.Errorf("ERR tree not found"))
	}
	tree.Nodes[hash] = data
	tree.Root = hash
	return ctx.WriteBulkString(hash)
}

func cmdMERKLEVERIFY(ctx *Context) error {
	if ctx.ArgCount() < 2 {
		return ctx.WriteError(ErrWrongArgCount)
	}
	name := ctx.ArgString(0)
	hash := ctx.ArgString(1)
	merkleTreesMu.RLock()
	tree, exists := merkleTrees[name]
	merkleTreesMu.RUnlock()
	if !exists {
		return ctx.WriteInteger(0)
	}
	_, ok := tree.Nodes[hash]
	if ok {
		return ctx.WriteInteger(1)
	}
	return ctx.WriteInteger(0)
}

func cmdMERKLEPROOF(ctx *Context) error {
	if ctx.ArgCount() < 2 {
		return ctx.WriteError(ErrWrongArgCount)
	}
	name := ctx.ArgString(0)
	hash := ctx.ArgString(1)
	merkleTreesMu.RLock()
	tree, exists := merkleTrees[name]
	merkleTreesMu.RUnlock()
	if !exists {
		return ctx.WriteError(fmt.Errorf("ERR tree not found"))
	}
	if data, ok := tree.Nodes[hash]; ok {
		return ctx.WriteArray([]*resp.Value{
			resp.BulkString("hash"), resp.BulkString(hash),
			resp.BulkString("data"), resp.BulkString(data),
		})
	}
	return ctx.WriteNull()
}

func cmdMERKLEROOT(ctx *Context) error {
	if ctx.ArgCount() < 1 {
		return ctx.WriteError(ErrWrongArgCount)
	}
	name := ctx.ArgString(0)
	merkleTreesMu.RLock()
	tree, exists := merkleTrees[name]
	merkleTreesMu.RUnlock()
	if !exists {
		return ctx.WriteNull()
	}
	return ctx.WriteBulkString(tree.Root)
}

var (
	raftState   = make(map[string]*RaftState)
	raftStateMu sync.RWMutex
)

type RaftState struct {
	Name     string
	State    string
	Leader   string
	Term     int64
	VotedFor string
	Log      []string
}

func cmdRAFTSTATE(ctx *Context) error {
	if ctx.ArgCount() < 2 {
		return ctx.WriteError(ErrWrongArgCount)
	}
	name := ctx.ArgString(0)
	state := ctx.ArgString(1)
	raftStateMu.Lock()
	defer raftStateMu.Unlock()
	if _, exists := raftState[name]; !exists {
		raftState[name] = &RaftState{Name: name, State: state, Leader: "", Term: 0, VotedFor: "", Log: make([]string, 0)}
	} else {
		raftState[name].State = state
	}
	return ctx.WriteOK()
}

func cmdRAFTLEADER(ctx *Context) error {
	if ctx.ArgCount() < 2 {
		return ctx.WriteError(ErrWrongArgCount)
	}
	name := ctx.ArgString(0)
	leader := ctx.ArgString(1)
	raftStateMu.Lock()
	defer raftStateMu.Unlock()
	if s, exists := raftState[name]; exists {
		s.Leader = leader
		return ctx.WriteOK()
	}
	return ctx.WriteError(fmt.Errorf("ERR raft not found"))
}

func cmdRAFTTERM(ctx *Context) error {
	if ctx.ArgCount() < 1 {
		return ctx.WriteError(ErrWrongArgCount)
	}
	name := ctx.ArgString(0)
	raftStateMu.Lock()
	defer raftStateMu.Unlock()
	if s, exists := raftState[name]; exists {
		s.Term++
		return ctx.WriteInteger(s.Term)
	}
	return ctx.WriteError(fmt.Errorf("ERR raft not found"))
}

func cmdRAFTVOTE(ctx *Context) error {
	if ctx.ArgCount() < 2 {
		return ctx.WriteError(ErrWrongArgCount)
	}
	name := ctx.ArgString(0)
	candidate := ctx.ArgString(1)
	raftStateMu.Lock()
	defer raftStateMu.Unlock()
	if s, exists := raftState[name]; exists {
		if s.VotedFor == "" || s.VotedFor == candidate {
			s.VotedFor = candidate
			return ctx.WriteInteger(1)
		}
		return ctx.WriteInteger(0)
	}
	return ctx.WriteError(fmt.Errorf("ERR raft not found"))
}

func cmdRAFTAPPEND(ctx *Context) error {
	if ctx.ArgCount() < 2 {
		return ctx.WriteError(ErrWrongArgCount)
	}
	name := ctx.ArgString(0)
	entry := ctx.ArgString(1)
	raftStateMu.Lock()
	defer raftStateMu.Unlock()
	if s, exists := raftState[name]; exists {
		s.Log = append(s.Log, entry)
		return ctx.WriteInteger(int64(len(s.Log)))
	}
	return ctx.WriteError(fmt.Errorf("ERR raft not found"))
}

func cmdRAFTCOMMIT(ctx *Context) error {
	if ctx.ArgCount() < 1 {
		return ctx.WriteError(ErrWrongArgCount)
	}
	name := ctx.ArgString(0)
	raftStateMu.RLock()
	s, exists := raftState[name]
	raftStateMu.RUnlock()
	if !exists {
		return ctx.WriteError(fmt.Errorf("ERR raft not found"))
	}
	return ctx.WriteInteger(int64(len(s.Log)))
}

var (
	shards   = make(map[string]*ShardState)
	shardsMu sync.RWMutex
)

type ShardState struct {
	Name  string
	Nodes []string
	Keys  map[string]string
}

func cmdSHARDMAP(ctx *Context) error {
	if ctx.ArgCount() < 2 {
		return ctx.WriteError(ErrWrongArgCount)
	}
	name := ctx.ArgString(0)
	key := ctx.ArgString(1)
	shardsMu.RLock()
	s, exists := shards[name]
	shardsMu.RUnlock()
	if !exists {
		return ctx.WriteNull()
	}
	if len(s.Nodes) == 0 {
		return ctx.WriteNull()
	}
	hash := hashString(key)
	nodeIdx := hash % len(s.Nodes)
	return ctx.WriteBulkString(s.Nodes[nodeIdx])
}

func cmdSHARDMOVE(ctx *Context) error {
	if ctx.ArgCount() < 2 {
		return ctx.WriteError(ErrWrongArgCount)
	}
	_ = ctx.ArgString(0)
	_ = ctx.ArgString(1)
	return ctx.WriteOK()
}

func cmdSHARDREBALANCE(ctx *Context) error {
	if ctx.ArgCount() < 1 {
		return ctx.WriteError(ErrWrongArgCount)
	}
	name := ctx.ArgString(0)
	shardsMu.RLock()
	s, exists := shards[name]
	shardsMu.RUnlock()
	if !exists {
		return ctx.WriteError(fmt.Errorf("ERR shard not found"))
	}
	return ctx.WriteInteger(int64(len(s.Nodes)))
}

func cmdSHARDLIST(ctx *Context) error {
	shardsMu.RLock()
	defer shardsMu.RUnlock()
	results := make([]*resp.Value, 0)
	for name := range shards {
		results = append(results, resp.BulkString(name))
	}
	return ctx.WriteArray(results)
}

func cmdSHARDSTATUS(ctx *Context) error {
	if ctx.ArgCount() < 1 {
		return ctx.WriteError(ErrWrongArgCount)
	}
	name := ctx.ArgString(0)
	shardsMu.RLock()
	s, exists := shards[name]
	shardsMu.RUnlock()
	if !exists {
		return ctx.WriteError(fmt.Errorf("ERR shard not found"))
	}
	return ctx.WriteArray([]*resp.Value{
		resp.BulkString("name"), resp.BulkString(s.Name),
		resp.BulkString("nodes"), resp.IntegerValue(int64(len(s.Nodes))),
		resp.BulkString("keys"), resp.IntegerValue(int64(len(s.Keys))),
	})
}

func cmdCOMPRESSIONCOMPRESS(ctx *Context) error {
	if ctx.ArgCount() < 1 {
		return ctx.WriteError(ErrWrongArgCount)
	}
	data := ctx.ArgString(0)
	return ctx.WriteBulkString(fmt.Sprintf("compressed:%d", len(data)))
}

func cmdCOMPRESSIONDECOMPRESS(ctx *Context) error {
	if ctx.ArgCount() < 1 {
		return ctx.WriteError(ErrWrongArgCount)
	}
	data := ctx.ArgString(0)
	return ctx.WriteBulkString(fmt.Sprintf("decompressed:%s", data))
}

func cmdCOMPRESSIONINFO(ctx *Context) error {
	return ctx.WriteArray([]*resp.Value{
		resp.BulkString("algorithm"), resp.BulkString("lz4"),
		resp.BulkString("level"), resp.IntegerValue(6),
	})
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
	defer dedupSetMu.RUnlock()
	fullKey := key + ":" + id
	if exp, exists := dedupSet[fullKey]; exists {
		if time.Now().UnixMilli() < exp {
			return ctx.WriteInteger(1)
		}
	}
	return ctx.WriteInteger(0)
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
	batches   = make(map[string]*Batch)
	batchesMu sync.RWMutex
)

type Batch struct {
	ID        string
	Status    string
	Progress  int
	Total     int
	CreatedAt int64
}

func cmdBATCHSUBMIT(ctx *Context) error {
	if ctx.ArgCount() < 1 {
		return ctx.WriteError(ErrWrongArgCount)
	}
	total := int(parseInt64(ctx.ArgString(0)))
	id := generateUUID()
	batchesMu.Lock()
	batches[id] = &Batch{ID: id, Status: "running", Progress: 0, Total: total, CreatedAt: time.Now().UnixMilli()}
	batchesMu.Unlock()
	return ctx.WriteBulkString(id)
}

func cmdBATCHSTATUS(ctx *Context) error {
	if ctx.ArgCount() < 1 {
		return ctx.WriteError(ErrWrongArgCount)
	}
	id := ctx.ArgString(0)
	batchesMu.RLock()
	batch, exists := batches[id]
	batchesMu.RUnlock()
	if !exists {
		return ctx.WriteError(fmt.Errorf("ERR batch not found"))
	}
	return ctx.WriteArray([]*resp.Value{
		resp.BulkString("id"), resp.BulkString(batch.ID),
		resp.BulkString("status"), resp.BulkString(batch.Status),
		resp.BulkString("progress"), resp.IntegerValue(int64(batch.Progress)),
		resp.BulkString("total"), resp.IntegerValue(int64(batch.Total)),
	})
}

func cmdBATCHCANCEL(ctx *Context) error {
	if ctx.ArgCount() < 1 {
		return ctx.WriteError(ErrWrongArgCount)
	}
	id := ctx.ArgString(0)
	batchesMu.Lock()
	defer batchesMu.Unlock()
	if batch, exists := batches[id]; exists {
		batch.Status = "cancelled"
		return ctx.WriteOK()
	}
	return ctx.WriteError(fmt.Errorf("ERR batch not found"))
}

func cmdBATCHLIST(ctx *Context) error {
	batchesMu.RLock()
	defer batchesMu.RUnlock()
	results := make([]*resp.Value, 0)
	for id, batch := range batches {
		results = append(results, resp.ArrayValue([]*resp.Value{
			resp.BulkString("id"), resp.BulkString(id),
			resp.BulkString("status"), resp.BulkString(batch.Status),
		}))
	}
	return ctx.WriteArray(results)
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
	defer deadlinesMu.RUnlock()
	results := make([]*resp.Value, 0)
	for id := range deadlines {
		results = append(results, resp.BulkString(id))
	}
	return ctx.WriteArray(results)
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
	defer gatewaysMu.RUnlock()
	results := make([]*resp.Value, 0)
	for id, gw := range gateways {
		results = append(results, resp.ArrayValue([]*resp.Value{
			resp.BulkString("id"), resp.BulkString(id),
			resp.BulkString("name"), resp.BulkString(gw.Name),
		}))
	}
	return ctx.WriteArray(results)
}

func cmdGATEWAYMETRICS(ctx *Context) error {
	if ctx.ArgCount() < 1 {
		return ctx.WriteError(ErrWrongArgCount)
	}
	id := ctx.ArgString(0)
	gatewaysMu.RLock()
	gw, exists := gateways[id]
	gatewaysMu.RUnlock()
	if !exists {
		return ctx.WriteError(fmt.Errorf("ERR gateway not found"))
	}
	return ctx.WriteArray([]*resp.Value{
		resp.BulkString("id"), resp.BulkString(gw.ID),
		resp.BulkString("name"), resp.BulkString(gw.Name),
		resp.BulkString("routes"), resp.IntegerValue(int64(len(gw.Routes))),
	})
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
	defer thresholdsMu.RUnlock()
	results := make([]*resp.Value, 0)
	for name, value := range thresholds {
		results = append(results, resp.ArrayValue([]*resp.Value{
			resp.BulkString("name"), resp.BulkString(name),
			resp.BulkString("value"), resp.IntegerValue(value),
		}))
	}
	return ctx.WriteArray(results)
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
	defer switchesMu.RUnlock()
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
	return ctx.WriteArray(results)
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
	defer bookmarksMu.RUnlock()
	results := make([]*resp.Value, 0)
	for key := range bookmarks {
		results = append(results, resp.BulkString(key))
	}
	return ctx.WriteArray(results)
}

var (
	replaysX    = make(map[string]*ReplayX)
	replaysXMux sync.RWMutex
)

type ReplayX struct {
	ID       string
	Status   string
	Speed    float64
	Position int64
}

func cmdREPLAYXSTART(ctx *Context) error {
	if ctx.ArgCount() < 1 {
		return ctx.WriteError(ErrWrongArgCount)
	}
	id := ctx.ArgString(0)
	replaysXMux.Lock()
	replaysX[id] = &ReplayX{ID: id, Status: "running", Speed: 1.0, Position: 0}
	replaysXMux.Unlock()
	return ctx.WriteOK()
}

func cmdREPLAYXSTOP(ctx *Context) error {
	if ctx.ArgCount() < 1 {
		return ctx.WriteError(ErrWrongArgCount)
	}
	id := ctx.ArgString(0)
	replaysXMux.Lock()
	defer replaysXMux.Unlock()
	if r, exists := replaysX[id]; exists {
		r.Status = "stopped"
		return ctx.WriteOK()
	}
	return ctx.WriteError(fmt.Errorf("ERR replay not found"))
}

func cmdREPLAYXPAUSE(ctx *Context) error {
	if ctx.ArgCount() < 1 {
		return ctx.WriteError(ErrWrongArgCount)
	}
	id := ctx.ArgString(0)
	replaysXMux.Lock()
	defer replaysXMux.Unlock()
	if r, exists := replaysX[id]; exists {
		r.Status = "paused"
		return ctx.WriteOK()
	}
	return ctx.WriteError(fmt.Errorf("ERR replay not found"))
}

func cmdREPLAYXSPEED(ctx *Context) error {
	if ctx.ArgCount() < 2 {
		return ctx.WriteError(ErrWrongArgCount)
	}
	id := ctx.ArgString(0)
	speed := parseFloatExt([]byte(ctx.ArgString(1)))
	replaysXMux.Lock()
	defer replaysXMux.Unlock()
	if r, exists := replaysX[id]; exists {
		r.Speed = speed
		return ctx.WriteOK()
	}
	return ctx.WriteError(fmt.Errorf("ERR replay not found"))
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
	ghostsMu.RUnlock()
	if !exists {
		return ctx.WriteArray([]*resp.Value{})
	}
	results := make([]*resp.Value, len(g.Data))
	for i, d := range g.Data {
		results[i] = resp.BulkString(d)
	}
	return ctx.WriteArray(results)
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

var (
	probes   = make(map[string]*Probe)
	probesMu sync.RWMutex
)

type Probe struct {
	ID      string
	Name    string
	Target  string
	Results []string
}

func cmdPROBECREATE(ctx *Context) error {
	if ctx.ArgCount() < 3 {
		return ctx.WriteError(ErrWrongArgCount)
	}
	id := ctx.ArgString(0)
	name := ctx.ArgString(1)
	target := ctx.ArgString(2)
	probesMu.Lock()
	probes[id] = &Probe{ID: id, Name: name, Target: target, Results: make([]string, 0)}
	probesMu.Unlock()
	return ctx.WriteOK()
}

func cmdPROBEDELETE(ctx *Context) error {
	if ctx.ArgCount() < 1 {
		return ctx.WriteError(ErrWrongArgCount)
	}
	id := ctx.ArgString(0)
	probesMu.Lock()
	defer probesMu.Unlock()
	if _, exists := probes[id]; !exists {
		return ctx.WriteInteger(0)
	}
	delete(probes, id)
	return ctx.WriteInteger(1)
}

func cmdPROBERUN(ctx *Context) error {
	if ctx.ArgCount() < 1 {
		return ctx.WriteError(ErrWrongArgCount)
	}
	id := ctx.ArgString(0)
	probesMu.Lock()
	defer probesMu.Unlock()
	p, exists := probes[id]
	if !exists {
		return ctx.WriteError(fmt.Errorf("ERR probe not found"))
	}
	p.Results = append(p.Results, "OK")
	return ctx.WriteOK()
}

func cmdPROBERESULTS(ctx *Context) error {
	if ctx.ArgCount() < 1 {
		return ctx.WriteError(ErrWrongArgCount)
	}
	id := ctx.ArgString(0)
	probesMu.RLock()
	p, exists := probes[id]
	probesMu.RUnlock()
	if !exists {
		return ctx.WriteArray([]*resp.Value{})
	}
	results := make([]*resp.Value, len(p.Results))
	for i, r := range p.Results {
		results[i] = resp.BulkString(r)
	}
	return ctx.WriteArray(results)
}

func cmdPROBELIST(ctx *Context) error {
	probesMu.RLock()
	defer probesMu.RUnlock()
	results := make([]*resp.Value, 0)
	for id := range probes {
		results = append(results, resp.BulkString(id))
	}
	return ctx.WriteArray(results)
}

var (
	canaries   = make(map[string]*Canary)
	canariesMu sync.RWMutex
)

type Canary struct {
	ID      string
	Name    string
	Status  string
	LastRun int64
}

func cmdCANARYCREATE(ctx *Context) error {
	if ctx.ArgCount() < 2 {
		return ctx.WriteError(ErrWrongArgCount)
	}
	id := ctx.ArgString(0)
	name := ctx.ArgString(1)
	canariesMu.Lock()
	canaries[id] = &Canary{ID: id, Name: name, Status: "pending", LastRun: 0}
	canariesMu.Unlock()
	return ctx.WriteOK()
}

func cmdCANARYDELETE(ctx *Context) error {
	if ctx.ArgCount() < 1 {
		return ctx.WriteError(ErrWrongArgCount)
	}
	id := ctx.ArgString(0)
	canariesMu.Lock()
	defer canariesMu.Unlock()
	if _, exists := canaries[id]; !exists {
		return ctx.WriteInteger(0)
	}
	delete(canaries, id)
	return ctx.WriteInteger(1)
}

func cmdCANARYCHECK(ctx *Context) error {
	if ctx.ArgCount() < 1 {
		return ctx.WriteError(ErrWrongArgCount)
	}
	id := ctx.ArgString(0)
	canariesMu.Lock()
	defer canariesMu.Unlock()
	c, exists := canaries[id]
	if !exists {
		return ctx.WriteError(fmt.Errorf("ERR canary not found"))
	}
	c.Status = "success"
	c.LastRun = time.Now().UnixMilli()
	return ctx.WriteOK()
}

func cmdCANARYSTATUS(ctx *Context) error {
	if ctx.ArgCount() < 1 {
		return ctx.WriteError(ErrWrongArgCount)
	}
	id := ctx.ArgString(0)
	canariesMu.RLock()
	c, exists := canaries[id]
	canariesMu.RUnlock()
	if !exists {
		return ctx.WriteError(fmt.Errorf("ERR canary not found"))
	}
	return ctx.WriteArray([]*resp.Value{
		resp.BulkString("id"), resp.BulkString(c.ID),
		resp.BulkString("name"), resp.BulkString(c.Name),
		resp.BulkString("status"), resp.BulkString(c.Status),
		resp.BulkString("last_run"), resp.IntegerValue(c.LastRun),
	})
}

func cmdCANARYLIST(ctx *Context) error {
	canariesMu.RLock()
	defer canariesMu.RUnlock()
	results := make([]*resp.Value, 0)
	for id := range canaries {
		results = append(results, resp.BulkString(id))
	}
	return ctx.WriteArray(results)
}

var (
	rageTests   = make(map[string]*RageTest)
	rageTestsMu sync.RWMutex
)

type RageTest struct {
	ID       string
	Running  bool
	Requests int64
	Errors   int64
}

func cmdRAGETEST(ctx *Context) error {
	if ctx.ArgCount() < 1 {
		return ctx.WriteError(ErrWrongArgCount)
	}
	id := ctx.ArgString(0)
	rageTestsMu.Lock()
	if _, exists := rageTests[id]; !exists {
		rageTests[id] = &RageTest{ID: id, Running: true, Requests: 0, Errors: 0}
	} else {
		rageTests[id].Running = true
	}
	rageTests[id].Requests++
	rageTestsMu.Unlock()
	return ctx.WriteOK()
}

func cmdRAGESTOP(ctx *Context) error {
	if ctx.ArgCount() < 1 {
		return ctx.WriteError(ErrWrongArgCount)
	}
	id := ctx.ArgString(0)
	rageTestsMu.Lock()
	defer rageTestsMu.Unlock()
	if r, exists := rageTests[id]; exists {
		r.Running = false
		return ctx.WriteOK()
	}
	return ctx.WriteError(fmt.Errorf("ERR test not found"))
}

func cmdRAGESTATS(ctx *Context) error {
	if ctx.ArgCount() < 1 {
		return ctx.WriteError(ErrWrongArgCount)
	}
	id := ctx.ArgString(0)
	rageTestsMu.RLock()
	r, exists := rageTests[id]
	rageTestsMu.RUnlock()
	if !exists {
		return ctx.WriteError(fmt.Errorf("ERR test not found"))
	}
	return ctx.WriteArray([]*resp.Value{
		resp.BulkString("id"), resp.BulkString(r.ID),
		resp.BulkString("running"), resp.BulkString(fmt.Sprintf("%v", r.Running)),
		resp.BulkString("requests"), resp.IntegerValue(r.Requests),
		resp.BulkString("errors"), resp.IntegerValue(r.Errors),
	})
}

func cmdRAGERESET(ctx *Context) error {
	if ctx.ArgCount() < 1 {
		return ctx.WriteError(ErrWrongArgCount)
	}
	id := ctx.ArgString(0)
	rageTestsMu.Lock()
	defer rageTestsMu.Unlock()
	delete(rageTests, id)
	return ctx.WriteOK()
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
	gridsMu.RUnlock()
	if !exists {
		return ctx.WriteNull()
	}
	key := x + "," + y
	if val, ok := g.Data[key]; ok {
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
	gridsMu.RUnlock()
	if !exists {
		return ctx.WriteArray([]*resp.Value{})
	}
	results := make([]*resp.Value, 0)
	for k, v := range g.Data {
		results = append(results, resp.ArrayValue([]*resp.Value{
			resp.BulkString("coord"), resp.BulkString(k),
			resp.BulkString("value"), resp.BulkString(v),
		}))
	}
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
	tapesMu.RUnlock()
	if !exists || len(t.Data) == 0 {
		return ctx.WriteNull()
	}
	if t.Pos >= len(t.Data) {
		return ctx.WriteNull()
	}
	return ctx.WriteBulkString(t.Data[t.Pos])
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

var (
	beacons   = make(map[string]*Beacon)
	beaconsMu sync.RWMutex
)

type Beacon struct {
	ID       string
	Running  bool
	LastPing int64
}

func cmdBEACONSTART(ctx *Context) error {
	if ctx.ArgCount() < 1 {
		return ctx.WriteError(ErrWrongArgCount)
	}
	id := ctx.ArgString(0)
	beaconsMu.Lock()
	beacons[id] = &Beacon{ID: id, Running: true, LastPing: time.Now().UnixMilli()}
	beaconsMu.Unlock()
	return ctx.WriteOK()
}

func cmdBEACONSTOP(ctx *Context) error {
	if ctx.ArgCount() < 1 {
		return ctx.WriteError(ErrWrongArgCount)
	}
	id := ctx.ArgString(0)
	beaconsMu.Lock()
	defer beaconsMu.Unlock()
	if b, exists := beacons[id]; exists {
		b.Running = false
		return ctx.WriteOK()
	}
	return ctx.WriteError(fmt.Errorf("ERR beacon not found"))
}

func cmdBEACONLIST(ctx *Context) error {
	beaconsMu.RLock()
	defer beaconsMu.RUnlock()
	results := make([]*resp.Value, 0)
	for id := range beacons {
		results = append(results, resp.BulkString(id))
	}
	return ctx.WriteArray(results)
}

func cmdBEACONCHECK(ctx *Context) error {
	if ctx.ArgCount() < 1 {
		return ctx.WriteError(ErrWrongArgCount)
	}
	id := ctx.ArgString(0)
	beaconsMu.RLock()
	b, exists := beacons[id]
	beaconsMu.RUnlock()
	if !exists {
		return ctx.WriteInteger(0)
	}
	if b.Running {
		return ctx.WriteInteger(1)
	}
	return ctx.WriteInteger(0)
}
