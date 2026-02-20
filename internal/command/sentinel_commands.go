package command

import (
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/cachestorm/cachestorm/internal/resp"
	"github.com/cachestorm/cachestorm/internal/sentinel"
)

var globalSentinel *sentinel.Sentinel

func InitSentinel(cfg sentinel.Config) {
	globalSentinel = sentinel.New(cfg)
}

func GetSentinel() *sentinel.Sentinel {
	return globalSentinel
}

func cmdSENTINEL(ctx *Context) error {
	if ctx.ArgCount() < 1 {
		return ctx.WriteError(ErrWrongArgCount)
	}

	if globalSentinel == nil {
		return ctx.WriteError(fmt.Errorf("ERR sentinel not initialized"))
	}

	subCmd := strings.ToUpper(ctx.ArgString(0))

	switch subCmd {
	case "MASTERS":
		return handleSentinelMasters(ctx)
	case "MASTER":
		return handleSentinelMaster(ctx)
	case "SLAVES", "REPLICAS":
		return handleSentinelReplicas(ctx)
	case "GETMASTER":
		return handleSentinelGetMaster(ctx)
	case "MONITOR":
		return handleSentinelMonitor(ctx)
	case "REMOVE":
		return handleSentinelRemove(ctx)
	case "SET":
		return handleSentinelSet(ctx)
	case "RESET":
		return handleSentinelReset(ctx)
	case "FAILOVER":
		return handleSentinelFailover(ctx)
	case "CKQUORUM":
		return handleSentinelCKQuorum(ctx)
	case "INFO":
		return handleSentinelInfo(ctx)
	case "ISMASTERDOWN":
		return handleSentinelIsMasterDown(ctx)
	default:
		return ctx.WriteError(fmt.Errorf("ERR unknown subcommand '%s'", subCmd))
	}
}

func handleSentinelMasters(ctx *Context) error {
	masters := globalSentinel.Masters()
	result := make([]*resp.Value, 0, len(masters))

	for _, m := range masters {
		state := "ok"
		if m.State == sentinel.MasterStateSDown {
			state = "sdown"
		} else if m.State == sentinel.MasterStateODown {
			state = "odown"
		}

		result = append(result, resp.ArrayValue([]*resp.Value{
			resp.BulkString("name"),
			resp.BulkString(m.Name),
			resp.BulkString("ip"),
			resp.BulkString(m.Addr),
			resp.BulkString("port"),
			resp.IntegerValue(int64(m.Port)),
			resp.BulkString("flags"),
			resp.BulkString(strings.Join(m.Flags, ",")),
			resp.BulkString("num-replicas"),
			resp.IntegerValue(int64(m.NumReplicas)),
			resp.BulkString("num-other-sentinels"),
			resp.IntegerValue(int64(m.NumSentinels)),
			resp.BulkString("status"),
			resp.BulkString(state),
		}))
	}

	return ctx.WriteArray(result)
}

func handleSentinelMaster(ctx *Context) error {
	if ctx.ArgCount() < 2 {
		return ctx.WriteError(ErrWrongArgCount)
	}

	name := ctx.ArgString(1)
	master, ok := globalSentinel.GetMaster(name)
	if !ok {
		return ctx.WriteError(fmt.Errorf("ERR no such master '%s'", name))
	}

	state := "ok"
	if master.State == sentinel.MasterStateSDown {
		state = "sdown"
	} else if master.State == sentinel.MasterStateODown {
		state = "odown"
	}

	return ctx.WriteArray([]*resp.Value{
		resp.BulkString("name"),
		resp.BulkString(master.Name),
		resp.BulkString("ip"),
		resp.BulkString(master.Addr),
		resp.BulkString("port"),
		resp.IntegerValue(int64(master.Port)),
		resp.BulkString("flags"),
		resp.BulkString(strings.Join(master.Flags, ",")),
		resp.BulkString("num-replicas"),
		resp.IntegerValue(int64(master.NumReplicas)),
		resp.BulkString("num-other-sentinels"),
		resp.IntegerValue(int64(master.NumSentinels)),
		resp.BulkString("status"),
		resp.BulkString(state),
		resp.BulkString("epoch"),
		resp.IntegerValue(master.Epoch),
	})
}

func handleSentinelReplicas(ctx *Context) error {
	if ctx.ArgCount() < 2 {
		return ctx.WriteError(ErrWrongArgCount)
	}

	name := ctx.ArgString(1)
	master, ok := globalSentinel.GetMaster(name)
	if !ok {
		return ctx.WriteError(fmt.Errorf("ERR no such master '%s'", name))
	}

	result := make([]*resp.Value, 0, len(master.Replicas))
	for _, r := range master.Replicas {
		result = append(result, resp.ArrayValue([]*resp.Value{
			resp.BulkString("ip"),
			resp.BulkString(r.Addr),
			resp.BulkString("port"),
			resp.IntegerValue(int64(r.Port)),
			resp.BulkString("state"),
			resp.BulkString(r.State),
			resp.BulkString("offset"),
			resp.IntegerValue(r.Offset),
		}))
	}

	return ctx.WriteArray(result)
}

func handleSentinelGetMaster(ctx *Context) error {
	if ctx.ArgCount() < 2 {
		return ctx.WriteError(ErrWrongArgCount)
	}

	name := ctx.ArgString(1)
	addr, port, err := globalSentinel.GetMasterAddr(name)
	if err != nil {
		return ctx.WriteError(err)
	}

	return ctx.WriteArray([]*resp.Value{
		resp.BulkString(addr),
		resp.IntegerValue(int64(port)),
	})
}

func handleSentinelMonitor(ctx *Context) error {
	if ctx.ArgCount() < 5 {
		return ctx.WriteError(ErrWrongArgCount)
	}

	name := ctx.ArgString(1)
	addr := ctx.ArgString(2)
	port, err := strconv.Atoi(ctx.ArgString(3))
	if err != nil {
		return ctx.WriteError(ErrNotInteger)
	}
	quorum, err := strconv.Atoi(ctx.ArgString(4))
	if err != nil {
		return ctx.WriteError(ErrNotInteger)
	}

	if err := globalSentinel.Monitor(name, addr, port, quorum); err != nil {
		return ctx.WriteError(err)
	}

	return ctx.WriteOK()
}

func handleSentinelRemove(ctx *Context) error {
	if ctx.ArgCount() < 2 {
		return ctx.WriteError(ErrWrongArgCount)
	}

	name := ctx.ArgString(1)
	if err := globalSentinel.Remove(name); err != nil {
		return ctx.WriteError(err)
	}

	return ctx.WriteOK()
}

func handleSentinelSet(ctx *Context) error {
	if ctx.ArgCount() < 3 {
		return ctx.WriteError(ErrWrongArgCount)
	}

	name := ctx.ArgString(1)
	option := strings.ToLower(ctx.ArgString(2))
	value := ctx.ArgString(3)

	_, ok := globalSentinel.GetMaster(name)
	if !ok {
		return ctx.WriteError(fmt.Errorf("ERR no such master '%s'", name))
	}

	switch option {
	case "down-after-milliseconds":
		_, err := strconv.Atoi(value)
		if err != nil {
			return ctx.WriteError(ErrNotInteger)
		}
		return ctx.WriteOK()
	default:
		return ctx.WriteError(fmt.Errorf("ERR unknown option '%s'", option))
	}
}

func handleSentinelReset(ctx *Context) error {
	if ctx.ArgCount() < 2 {
		return ctx.WriteError(ErrWrongArgCount)
	}

	pattern := ctx.ArgString(1)
	count := globalSentinel.Reset(pattern)

	return ctx.WriteInteger(int64(count))
}

func handleSentinelFailover(ctx *Context) error {
	if ctx.ArgCount() < 2 {
		return ctx.WriteError(ErrWrongArgCount)
	}

	name := ctx.ArgString(1)
	if err := globalSentinel.Failover(name); err != nil {
		return ctx.WriteError(err)
	}

	return ctx.WriteOK()
}

func handleSentinelCKQuorum(ctx *Context) error {
	if ctx.ArgCount() < 2 {
		return ctx.WriteError(ErrWrongArgCount)
	}

	name := ctx.ArgString(1)
	count, err := globalSentinel.CKQUORUM(name)
	if err != nil {
		return ctx.WriteError(err)
	}

	return ctx.WriteInteger(int64(count))
}

func handleSentinelInfo(ctx *Context) error {
	info := globalSentinel.Info()

	var sb strings.Builder
	sb.WriteString("# Sentinel\r\n")
	sb.WriteString(fmt.Sprintf("sentinel_id:%s\r\n", info["sentinel_id"]))
	sb.WriteString(fmt.Sprintf("sentinel_addr:%s\r\n", info["sentinel_addr"]))
	sb.WriteString(fmt.Sprintf("sentinel_port:%d\r\n", info["sentinel_port"]))
	sb.WriteString(fmt.Sprintf("masters:%d\r\n", info["masters"]))
	sb.WriteString(fmt.Sprintf("running:%v\r\n", info["running"]))
	sb.WriteString(fmt.Sprintf("quorum:%d\r\n", info["quorum"]))

	return ctx.WriteBulkString(sb.String())
}

func handleSentinelIsMasterDown(ctx *Context) error {
	if ctx.ArgCount() < 3 {
		return ctx.WriteError(ErrWrongArgCount)
	}

	name := ctx.ArgString(0)
	master, ok := globalSentinel.GetMaster(name)
	if !ok {
		return ctx.WriteArray([]*resp.Value{
			resp.IntegerValue(0),
			resp.NullBulkString(),
		})
	}

	isDown := 0
	if master.State == sentinel.MasterStateSDown || master.State == sentinel.MasterStateODown {
		isDown = 1
	}

	return ctx.WriteArray([]*resp.Value{
		resp.IntegerValue(int64(isDown)),
		resp.BulkString("sentinel-1"),
	})
}

func RegisterSentinelCommands(router *Router) {
	router.Register(&CommandDef{Name: "SENTINEL", Handler: cmdSENTINEL})
}

func StartSentinel() error {
	if globalSentinel == nil {
		return fmt.Errorf("sentinel not initialized")
	}
	return globalSentinel.Start()
}

func StopSentinel() {
	if globalSentinel != nil {
		globalSentinel.Stop()
	}
}

func init() {
	_ = time.Second
}
