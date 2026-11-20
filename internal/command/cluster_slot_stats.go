package command

import (
	"errors"
	"fmt"
	"sort"
	"strconv"
	"strings"
	"sync/atomic"

	"github.com/cachestorm/cachestorm/internal/cluster"
	"github.com/cachestorm/cachestorm/internal/resp"
)

// slotStatsEnabled is cluster-slot-stats-enabled: CLUSTER SLOT-STATS then
// also reports each slot's CPU time and network bytes, which every command
// with keys pays to count.
var slotStatsEnabled atomic.Bool

//nolint:staticcheck // Redis' reply texts
var (
	errSlotStatsMetric   = errors.New("ERR Unrecognized sort metric for ORDERBY.")
	errSlotStatsLimit    = errors.New("ERR Limit has to lie in between 1 and 16384 (maximum number of slots).")
	errSlotStatsMultiple = errors.New("ERR Multiple filters of the same type are disallowed.")
)

// Metrics CLUSTER SLOT-STATS ORDERBY sorts by.
const (
	slotStatKeyCount = iota
	slotStatCPU
	slotStatBytesIn
	slotStatBytesOut
)

// clusterSlotStats is Redis' clusterSlotStatsCommand.
func clusterSlotStats(ctx *Context, c *cluster.Cluster, args [][]byte) error {
	db := clusterDB(ctx)
	enabled := slotStatsEnabled.Load()
	stat := func(slot, metric int) uint64 {
		if metric == slotStatKeyCount {
			return uint64(db.CountKeysInSlot(slot))
		}
		u := c.SlotUsageOf(slot)
		switch metric {
		case slotStatCPU:
			return u.CPUUsec
		case slotStatBytesIn:
			return u.BytesIn
		}
		return u.BytesOut
	}
	entry := func(slot int) *resp.Value {
		pairs := []*resp.Value{resp.BulkString("key-count"), resp.IntegerValue(int64(stat(slot, slotStatKeyCount)))}
		if enabled {
			u := c.SlotUsageOf(slot)
			pairs = append(pairs,
				resp.BulkString("cpu-usec"), resp.IntegerValue(int64(u.CPUUsec)),
				resp.BulkString("network-bytes-in"), resp.IntegerValue(int64(u.BytesIn)),
				resp.BulkString("network-bytes-out"), resp.IntegerValue(int64(u.BytesOut)))
		}
		return resp.ArrayValue([]*resp.Value{resp.IntegerValue(int64(slot)), resp.PairsValue(pairs)})
	}

	switch {
	case len(args) == 3 && strings.EqualFold(string(args[0]), "SLOTSRANGE"):
		start, err := parseSlotArg(string(args[1]))
		if err != nil {
			return ctx.WriteError(err)
		}
		end, err := parseSlotArg(string(args[2]))
		if err != nil {
			return ctx.WriteError(err)
		}
		if start > end {
			return ctx.WriteError(fmt.Errorf("ERR Start slot number %d is greater than end slot number %d", start, end))
		}
		out := []*resp.Value{}
		for _, slot := range c.ShardSlots() {
			if slot >= start && slot <= end {
				out = append(out, entry(slot))
			}
		}
		return ctx.WriteArray(out)

	case len(args) >= 2 && strings.EqualFold(string(args[0]), "ORDERBY"):
		var metric int
		switch m := strings.ToLower(string(args[1])); {
		case m == "key-count":
			metric = slotStatKeyCount
		case m == "cpu-usec" && enabled:
			metric = slotStatCPU
		case m == "network-bytes-in" && enabled:
			metric = slotStatBytesIn
		case m == "network-bytes-out" && enabled:
			metric = slotStatBytesOut
		default:
			return ctx.WriteError(errSlotStatsMetric)
		}
		desc, limit := true, cluster.Slots
		limits, orders := 0, 0
		for i := 2; i < len(args); i++ {
			switch a := string(args[i]); {
			case strings.EqualFold(a, "LIMIT") && i+1 < len(args):
				n, ok := parseRedisInt(args[i+1])
				if !ok {
					return ctx.WriteError(ErrNotInteger)
				}
				if n < 1 || n > cluster.Slots {
					return ctx.WriteError(errSlotStatsLimit)
				}
				limit = int(n)
				i++
				limits++
			case strings.EqualFold(a, "ASC"):
				desc = false
				orders++
			case strings.EqualFold(a, "DESC"):
				desc = true
				orders++
			default:
				return ctx.WriteError(ErrSyntaxError)
			}
			if limits > 1 || orders > 1 {
				return ctx.WriteError(errSlotStatsMultiple)
			}
		}
		type slotStat struct {
			slot int
			stat uint64
		}
		slots := c.ShardSlots()
		sorted := make([]slotStat, len(slots))
		for i, s := range slots {
			sorted[i] = slotStat{s, stat(s, metric)}
		}
		sort.Slice(sorted, func(i, j int) bool {
			a, b := sorted[i], sorted[j]
			if a.stat == b.stat {
				return a.slot < b.slot
			}
			return (a.stat > b.stat) == desc
		})
		out := make([]*resp.Value, min(limit, len(sorted)))
		for i := range out {
			out[i] = entry(sorted[i].slot)
		}
		return ctx.WriteArray(out)
	}
	return ctx.WriteError(errors.New("ERR unknown subcommand or wrong number of arguments for '" + ctx.ArgString(0) + "'. Try CLUSTER HELP."))
}

// RecordSlotUsage counts a command the connection ran towards its slot's
// CLUSTER SLOT-STATS, as Redis does once the command is processed: its
// request's bytes even when it was queued or redirected, its run time and
// the bytes of its reply when it ran. nameLen is the length of the command
// name the client sent.
func (ctx *Context) RecordSlotUsage(nameLen, replyBytes int) {
	if !slotStatsEnabled.Load() {
		return
	}
	c := ctx.clusterState()
	slot := ctx.commandSlot() - 1
	if c == nil || slot < 0 || ctx.FromMaster || ctx.Replaying {
		return
	}
	in := requestBytes(nameLen, ctx.Args)
	if ctx.Command == "EXEC" {
		// The MULTI that opened the transaction: *1\r\n$5\r\nmulti\r\n.
		in += 15
	}
	var cpu, out uint64
	if !ctx.EndTime.IsZero() {
		if d := ctx.EndTime.Sub(ctx.StartTime) - ctx.waited; d > 0 {
			cpu = uint64(d.Microseconds())
		}
		out = uint64(max(replyBytes, 0))
	}
	c.AddSlotUsage(slot, cpu, in, out)
}

// requestBytes is the size of a command sent as a RESP array.
func requestBytes(nameLen int, args [][]byte) uint64 {
	n := uint64(len(strconv.Itoa(len(args)+1)) + 3)
	n += uint64(len(strconv.Itoa(nameLen))+3) + uint64(nameLen) + 2
	for _, a := range args {
		n += uint64(len(strconv.Itoa(len(a)))+3) + uint64(len(a)) + 2
	}
	return n
}
