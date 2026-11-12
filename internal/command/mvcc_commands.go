package command

import (
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"sync"
	"sync/atomic"
	"time"

	"github.com/cachestorm/cachestorm/internal/resp"
	"github.com/cachestorm/cachestorm/internal/store"
)

func RegisterMVCCCommands(router *Router) {

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

type MVCCChange struct {
	Key       string
	OldValue  string
	NewValue  string
	Operation string
}

// haversine delegates to store.Haversine, the math-package implementation
// this repo already ships and that GEODIST/GEOSEARCH use. The previous local
// copy reimplemented sin/cos/atan/sqrt as truncated series, which made
// co-located points return NaN (sqrt's Newton iteration divides 0/0 at a == 0),
// lost ~0.6 km on NYC->LA, and diverged to ~-1e+60 km across 180 degrees of
// longitude because the Maclaurin atan series does not converge for |y/x| > 1.
//
// Argument orders differ: this helper takes lat-first, store.Haversine lon-first.
func haversine(lat1, lon1, lat2, lon2 float64) float64 {
	return store.Haversine(lon1, lat1, lon2, lat2)
}

func computeBlockHash(b *Block) string {
	h := sha256.New()
	var buf [8]byte
	binary.BigEndian.PutUint64(buf[:], uint64(b.Index))
	h.Write(buf[:])
	binary.BigEndian.PutUint64(buf[:], uint64(b.Timestamp))
	h.Write(buf[:])
	for _, field := range []string{b.Data, b.PrevHash} {
		binary.BigEndian.PutUint64(buf[:], uint64(len(field)))
		h.Write(buf[:])
		h.Write([]byte(field))
	}
	return hex.EncodeToString(h.Sum(nil))
}

func validateBlocks(blocks []*Block) bool {
	prevHash := chainGenesisPrevHash
	for i, b := range blocks {
		if b.Index != int64(i) || b.PrevHash != prevHash || b.Hash != computeBlockHash(b) {
			return false
		}
		prevHash = b.Hash
	}
	return true
}

var (
	connections   = make(map[int64]*ConnectionInfo)
	connectionsMu sync.RWMutex
)

// TrackConnection registers a live connection in the registry that
// CONNECTION.INFO, CONNECTION.LIST and CONNECTION.COUNT report.
// The connection keeps the returned entry to count its commands with Bump
// without a registry lookup.
func TrackConnection(id int64, remoteAddr string) *ConnectionInfo {
	now := time.Now().UnixMilli()
	connectionsMu.Lock()
	info := &ConnectionInfo{ID: id, RemoteAddr: remoteAddr, ConnectedAt: now}
	info.LastActive.Store(now)
	connections[id] = info
	connectionsMu.Unlock()
	return info
}

// Bump counts a command against the connection, which was last active at
// now.
func (c *ConnectionInfo) Bump(now time.Time) {
	c.Commands.Add(1)
	c.LastActive.Store(now.UnixMilli())
}

// UntrackConnection removes a connection from the registry once it is gone.
func UntrackConnection(id int64) {
	connectionsMu.Lock()
	delete(connections, id)
	connectionsMu.Unlock()
}

// BumpConnectionCommand records that a connection issued one more command.
// The counters are atomic, so every command takes only the registry's read
// lock instead of serialising all connections on its write lock.
func BumpConnectionCommand(id int64) { BumpConnectionCommandAt(id, time.Now()) }

// BumpConnectionCommandAt counts a command against the connection, which
// was last active at now.
func BumpConnectionCommandAt(id int64, now time.Time) {
	connectionsMu.RLock()
	if c, exists := connections[id]; exists {
		c.Commands.Add(1)
		c.LastActive.Store(now.UnixMilli())
	}
	connectionsMu.RUnlock()
}

type ConnectionInfo struct {
	ID          int64
	RemoteAddr  string
	ConnectedAt int64
	LastActive  atomic.Int64
	Commands    atomic.Int64
}

func cmdCONNECTIONLIST(ctx *Context) error {
	connectionsMu.RLock()

	results := make([]*resp.Value, 0)
	for id, conn := range connections {
		results = append(results,
			resp.IntegerValue(id),
			resp.BulkString(conn.RemoteAddr),
		)
	}

	reply := results
	connectionsMu.RUnlock()
	return ctx.WriteArray(reply)
}

func cmdCONNECTIONKILL(ctx *Context) error {
	if ctx.ArgCount() < 1 {
		return ctx.WriteError(ErrWrongArgCount)
	}

	id := parseInt64(ctx.ArgString(0))
	// The connection is closed through CLIENT KILL; deleting the registry
	// entry alone left the client connected while reporting it killed.
	if h := clientHooksFor(ctx); h != nil {
		return ctx.WriteInteger(int64(h.Kill(ctx.ClientID, ClientFilter{IDs: []int64{id}})))
	}
	return ctx.WriteInteger(0)
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

	if !exists {
		connectionsMu.RUnlock()
		return ctx.WriteNull()
	}

	reply := []*resp.Value{
		resp.BulkString("id"),
		resp.IntegerValue(conn.ID),
		resp.BulkString("remote_addr"),
		resp.BulkString(conn.RemoteAddr),
		resp.BulkString("connected_at"),
		resp.IntegerValue(conn.ConnectedAt),
		resp.BulkString("commands"),
		resp.IntegerValue(conn.Commands.Load()),
	}
	connectionsMu.RUnlock()
	return ctx.WriteArray(reply)
}

type Plugin struct {
	Name      string
	Enabled   bool
	LoadedAt  int64
	Functions map[string]string
}
