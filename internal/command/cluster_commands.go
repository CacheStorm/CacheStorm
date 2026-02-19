package command

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/cachestorm/cachestorm/internal/cluster"
	"github.com/cachestorm/cachestorm/internal/resp"
)

func RegisterClusterCommands(router *Router) {
	router.Register(&CommandDef{Name: "CLUSTER", Handler: cmdCLUSTER})
	router.Register(&CommandDef{Name: "CLUSTERINFO", Handler: cmdCLUSTERINFO})
	router.Register(&CommandDef{Name: "CLUSTERNODES", Handler: cmdCLUSTERNODES})
	router.Register(&CommandDef{Name: "CLUSTERSLOTS", Handler: cmdCLUSTERSLOTS})
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
		return ctx.WriteError(fmt.Errorf("ERR CLUSTER MEET not implemented"))
	case "MYID":
		return ctx.WriteBulkString("node-1")
	case "RESET":
		return ctx.WriteOK()
	default:
		return ctx.WriteError(fmt.Errorf("ERR unknown subcommand '%s'", subCmd))
	}
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

func checkClusterRouting(ctx *Context, key string) error {
	return nil
}

func init() {
	_ = cluster.NumSlots
	_ = strconv.Itoa(0)
}
