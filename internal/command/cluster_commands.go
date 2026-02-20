package command

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/cachestorm/cachestorm/internal/cluster"
	"github.com/cachestorm/cachestorm/internal/resp"
)

var globalCluster *cluster.Cluster
var globalGossip *cluster.Gossip
var globalFailover *cluster.FailoverManager
var globalMigrator *cluster.SlotMigrator

func InitCluster(c *cluster.Cluster) {
	globalCluster = c
	globalGossip = cluster.NewGossip(c)
	globalFailover = cluster.NewFailoverManager(c, globalGossip)
	globalMigrator = cluster.NewSlotMigrator(c)
}

func RegisterClusterCommands(router *Router) {
	router.Register(&CommandDef{Name: "CLUSTER", Handler: cmdCLUSTER})
	router.Register(&CommandDef{Name: "CLUSTERINFO", Handler: cmdCLUSTERINFO})
	router.Register(&CommandDef{Name: "CLUSTERNODES", Handler: cmdCLUSTERNODES})
	router.Register(&CommandDef{Name: "CLUSTERSLOTS", Handler: cmdCLUSTERSLOTS})
	router.Register(&CommandDef{Name: "MIGRATE", Handler: cmdMIGRATE})
	router.Register(&CommandDef{Name: "ASKING", Handler: cmdASKING})
	router.Register(&CommandDef{Name: "READONLY", Handler: cmdREADONLY})
	router.Register(&CommandDef{Name: "READWRITE", Handler: cmdREADWRITE})
}

func cmdCLUSTER(ctx *Context) error {
	if ctx.ArgCount() < 1 {
		return ctx.WriteError(ErrWrongArgCount)
	}

	subCmd := strings.ToUpper(ctx.ArgString(0))

	switch subCmd {
	case "INFO":
		return cmdCLUSTERINFO(ctx)
	case "NODES":
		return cmdCLUSTERNODES(ctx)
	case "SLOTS":
		return cmdCLUSTERSLOTS(ctx)
	case "MEET":
		return handleClusterMeet(ctx)
	case "MYID":
		if globalCluster != nil {
			return ctx.WriteBulkString(globalCluster.Self().ID)
		}
		return ctx.WriteBulkString("node-1")
	case "RESET":
		return ctx.WriteOK()
	case "FAILOVER":
		return handleClusterFailover(ctx)
	case "REBALANCE":
		return handleClusterRebalance(ctx)
	case "HEALTH":
		return handleClusterHealth(ctx)
	case "STATS":
		return handleClusterStats(ctx)
	case "ADDSLOTS":
		return handleClusterAddSlots(ctx)
	case "DELSLOTS":
		return handleClusterDelSlots(ctx)
	case "SETSLOT":
		return handleClusterSetSlot(ctx)
	case "REPLICAS":
		return handleClusterReplicas(ctx)
	case "COUNTKEYSINSLOT":
		return handleClusterCountKeysInSlot(ctx)
	case "GETKEYSINSLOT":
		return handleClusterGetKeysInSlot(ctx)
	default:
		return ctx.WriteError(fmt.Errorf("ERR unknown subcommand '%s'", subCmd))
	}
}

func handleClusterMeet(ctx *Context) error {
	if ctx.ArgCount() < 2 {
		return ctx.WriteError(ErrWrongArgCount)
	}

	ip := ctx.ArgString(1)
	port := 7946
	if ctx.ArgCount() >= 3 {
		var err error
		port, err = strconv.Atoi(ctx.ArgString(2))
		if err != nil {
			return ctx.WriteError(ErrNotInteger)
		}
	}

	if globalGossip != nil {
		if err := globalGossip.Meet(ip, port); err != nil {
			return ctx.WriteError(err)
		}
	}

	return ctx.WriteOK()
}

func handleClusterFailover(ctx *Context) error {
	if ctx.ArgCount() < 1 {
		return ctx.WriteError(ErrWrongArgCount)
	}

	force := false
	takeover := false

	for i := 1; i < ctx.ArgCount(); i++ {
		arg := strings.ToUpper(ctx.ArgString(i))
		switch arg {
		case "FORCE":
			force = true
		case "TAKEOVER":
			takeover = true
		}
	}

	_ = force
	_ = takeover

	return ctx.WriteOK()
}

func handleClusterRebalance(ctx *Context) error {
	if globalCluster == nil {
		return ctx.WriteError(fmt.Errorf("ERR cluster not initialized"))
	}

	result := globalCluster.Rebalance()
	return ctx.WriteValue(mapToValue(result))
}

func handleClusterHealth(ctx *Context) error {
	if globalCluster == nil {
		return ctx.WriteError(fmt.Errorf("ERR cluster not initialized"))
	}

	health := globalCluster.CheckClusterHealth()
	return ctx.WriteValue(mapToValue(health))
}

func handleClusterStats(ctx *Context) error {
	if globalCluster == nil {
		return ctx.WriteError(fmt.Errorf("ERR cluster not initialized"))
	}

	stats := globalCluster.GetClusterStats()
	return ctx.WriteValue(mapToValue(stats))
}

func handleClusterAddSlots(ctx *Context) error {
	if ctx.ArgCount() < 1 {
		return ctx.WriteError(ErrWrongArgCount)
	}

	if globalCluster == nil {
		return ctx.WriteError(fmt.Errorf("ERR cluster not initialized"))
	}

	slots := make([]cluster.SlotRange, 0)
	for i := 1; i < ctx.ArgCount(); i++ {
		slot, err := strconv.Atoi(ctx.ArgString(i))
		if err != nil {
			return ctx.WriteError(ErrNotInteger)
		}
		slots = append(slots, cluster.SlotRange{Start: uint16(slot), End: uint16(slot)})
	}

	globalCluster.AssignSlots(slots)
	return ctx.WriteOK()
}

func handleClusterDelSlots(ctx *Context) error {
	if ctx.ArgCount() < 1 {
		return ctx.WriteError(ErrWrongArgCount)
	}

	return ctx.WriteOK()
}

func handleClusterSetSlot(ctx *Context) error {
	if ctx.ArgCount() < 3 {
		return ctx.WriteError(ErrWrongArgCount)
	}

	_ = ctx.ArgString(1)
	subCmd := strings.ToUpper(ctx.ArgString(2))
	_ = subCmd

	return ctx.WriteOK()
}

func handleClusterReplicas(ctx *Context) error {
	if ctx.ArgCount() < 1 {
		return ctx.WriteError(ErrWrongArgCount)
	}

	return ctx.WriteArray([]*resp.Value{})
}

func handleClusterCountKeysInSlot(ctx *Context) error {
	if ctx.ArgCount() < 1 {
		return ctx.WriteError(ErrWrongArgCount)
	}

	return ctx.WriteInteger(0)
}

func handleClusterGetKeysInSlot(ctx *Context) error {
	if ctx.ArgCount() < 2 {
		return ctx.WriteError(ErrWrongArgCount)
	}

	return ctx.WriteArray([]*resp.Value{})
}

func mapToValue(m map[string]interface{}) *resp.Value {
	items := make([]*resp.Value, 0, len(m)*2)
	for k, v := range m {
		items = append(items, resp.BulkString(k))
		switch val := v.(type) {
		case string:
			items = append(items, resp.BulkString(val))
		case int:
			items = append(items, resp.IntegerValue(int64(val)))
		case int64:
			items = append(items, resp.IntegerValue(val))
		case float64:
			items = append(items, resp.BulkString(fmt.Sprintf("%.2f", val)))
		case bool:
			if val {
				items = append(items, resp.IntegerValue(1))
			} else {
				items = append(items, resp.IntegerValue(0))
			}
		case []string:
			arr := make([]*resp.Value, len(val))
			for i, s := range val {
				arr[i] = resp.BulkString(s)
			}
			items = append(items, resp.ArrayValue(arr))
		default:
			items = append(items, resp.BulkString(fmt.Sprintf("%v", val)))
		}
	}
	return resp.ArrayValue(items)
}

func cmdCLUSTERINFO(ctx *Context) error {
	if ctx.ArgCount() != 0 {
		return ctx.WriteError(ErrWrongArgCount)
	}

	var sb strings.Builder
	sb.WriteString("cluster_state:ok\r\n")
	sb.WriteString("cluster_slots_assigned:16384\r\n")
	sb.WriteString("cluster_slots_ok:16384\r\n")
	sb.WriteString("cluster_slots_pfail:0\r\n")
	sb.WriteString("cluster_slots_fail:0\r\n")
	sb.WriteString("cluster_known_nodes:1\r\n")
	sb.WriteString("cluster_size:1\r\n")
	sb.WriteString("cluster_current_epoch:1\r\n")
	sb.WriteString("cluster_my_epoch:1\r\n")
	sb.WriteString("cluster_stats_messages_sent:0\r\n")
	sb.WriteString("cluster_stats_messages_received:0\r\n")

	return ctx.WriteBulkString(sb.String())
}

func cmdCLUSTERNODES(ctx *Context) error {
	if ctx.ArgCount() != 0 {
		return ctx.WriteError(ErrWrongArgCount)
	}

	nodeID := "node-1"
	addr := "127.0.0.1"
	port := 6380
	cport := 7946

	line := fmt.Sprintf("%s %s:%d@%d myself,master - 0 0 0 connected 0-16383",
		nodeID, addr, port, cport)

	return ctx.WriteBulkString(line)
}

func cmdCLUSTERSLOTS(ctx *Context) error {
	if ctx.ArgCount() != 0 {
		return ctx.WriteError(ErrWrongArgCount)
	}

	slots := []*resp.Value{
		resp.ArrayValue([]*resp.Value{
			resp.IntegerValue(0),
			resp.IntegerValue(16383),
			resp.ArrayValue([]*resp.Value{
				resp.BulkString("127.0.0.1"),
				resp.IntegerValue(6380),
				resp.BulkString("node-1"),
			}),
		}),
	}

	return ctx.WriteArray(slots)
}

func checkClusterRouting(_ *Context, key string) error {
	_ = key
	return nil
}

func cmdMIGRATE(ctx *Context) error {
	if ctx.ArgCount() < 4 {
		return ctx.WriteError(ErrWrongArgCount)
	}

	host := ctx.ArgString(0)
	port := ctx.ArgString(1)
	key := ctx.ArgString(2)
	destinationDB := ctx.ArgString(3)

	_ = host
	_ = port
	_ = key
	_ = destinationDB

	copy := false
	replace := false
	timeout := 0

	for i := 4; i < ctx.ArgCount(); i++ {
		arg := strings.ToUpper(ctx.ArgString(i))
		switch arg {
		case "COPY":
			copy = true
		case "REPLACE":
			replace = true
		case "AUTH":
			i++
		case "AUTH2":
			i += 2
		case "TIMEOUT":
			i++
			if i < ctx.ArgCount() {
				var err error
				timeout, err = strconv.Atoi(ctx.ArgString(i))
				if err != nil {
					return ctx.WriteError(ErrNotInteger)
				}
			}
		}
	}

	_ = copy
	_ = replace
	_ = timeout

	entry, exists := ctx.Store.Get(key)
	if !exists {
		return ctx.WriteBulkString("NOKEY")
	}

	_ = entry

	return ctx.WriteOK()
}

func cmdASKING(ctx *Context) error {
	return ctx.WriteOK()
}

func cmdREADONLY(ctx *Context) error {
	return ctx.WriteOK()
}

func cmdREADWRITE(ctx *Context) error {
	return ctx.WriteOK()
}

func init() {
	_ = cluster.NumSlots
	_ = strconv.Itoa(0)
}
