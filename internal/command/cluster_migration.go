package command

import (
	"errors"
	"fmt"
	"sort"
	"strconv"
	"strings"
	"sync/atomic"

	"github.com/cachestorm/cachestorm/internal/cluster"
	"github.com/cachestorm/cachestorm/internal/hashslot"
	"github.com/cachestorm/cachestorm/internal/resp"
)

// SlotMigrationStatus is one task of CLUSTER MIGRATION STATUS.
type SlotMigrationStatus struct {
	ID, Slots, Source, Dest, Operation, State, LastError  string
	Retries, CreateTime, StartTime, EndTime, WritePauseMs int64
}

// SlotMigrationController runs Redis 8.4's atomic slot migration; the
// server implements it.
type SlotMigrationController interface {
	// Import starts importing ranges from the node serving them and
	// returns the task ID.
	Import(ranges []cluster.SlotRange) (string, error)
	// Cancel cancels the task id, or every task for "", and returns how
	// many it cancelled.
	Cancel(id, reason string) int
	// CancelSlots cancels the tasks moving any slot of ranges, and
	// CancelNode those whose source or destination is the node id.
	CancelSlots(ranges []cluster.SlotRange, reason string)
	CancelNode(id, reason string)
	// Status lists the task id, or every task for "".
	Status(id string) []SlotMigrationStatus
	// SyncSlots runs CLUSTER SYNCSLOTS on an internal connection; args
	// start at the subcommand.
	SyncSlots(ctx *Context, args [][]byte) error
	// Info is what CLUSTER INFO reports of slot migration.
	Info() string
}

// SetSlotMigrationController installs the controller CLUSTER MIGRATION and
// CLUSTER SYNCSLOTS use.
func (r *Router) SetSlotMigrationController(c SlotMigrationController) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.slotMigration = c
}

func (r *Router) slotMigrationController() SlotMigrationController {
	if r == nil {
		return nil
	}
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.slotMigration
}

func (ctx *Context) slotMigration() SlotMigrationController {
	return ctx.dispatcher().slotMigrationController()
}

// The settings of atomic slot migration (cluster-slot-migration-*).
var (
	asmHandoffMaxLagBytes   atomic.Int64
	asmWritePauseTimeout    atomic.Int64
	asmSyncBufferDrainLimit atomic.Int64
	asmMaxArchivedTasks     atomic.Int64
)

func init() {
	asmHandoffMaxLagBytes.Store(1 << 20)
	asmWritePauseTimeout.Store(10000)
	asmSyncBufferDrainLimit.Store(60000)
	asmMaxArchivedTasks.Store(32)
}

// SlotMigrationSettings returns cluster-slot-migration-handoff-max-lag-bytes,
// -write-pause-timeout and -sync-buffer-drain-timeout (milliseconds) and
// -max-archived-tasks.
func SlotMigrationSettings() (maxLag, pauseTimeout, drainTimeout, maxArchived int64) {
	return asmHandoffMaxLagBytes.Load(), asmWritePauseTimeout.Load(), asmSyncBufferDrainLimit.Load(), asmMaxArchivedTasks.Load()
}

// ParseSlotRanges is Redis' parseSlotRangesOrReply: start and end slot
// pairs, sorted with adjacent ranges merged, none given twice.
func ParseSlotRanges(args [][]byte) ([]cluster.SlotRange, error) {
	if len(args) < 2 || len(args)%2 != 0 {
		return nil, errArity
	}
	ranges := make([]cluster.SlotRange, 0, len(args)/2)
	for i := 0; i < len(args); i += 2 {
		start, err := parseSlotArg(string(args[i]))
		if err != nil {
			return nil, err
		}
		end, err := parseSlotArg(string(args[i+1]))
		if err != nil {
			return nil, err
		}
		ranges = append(ranges, cluster.SlotRange{Start: start, End: end})
	}
	return NormalizeSlotRanges(ranges)
}

// errArity stands for the caller's own wrong number of arguments error.
var errArity = errors.New("arity")

// NormalizeSlotRanges is slotRangeArrayNormalizeAndValidate.
func NormalizeSlotRanges(ranges []cluster.SlotRange) ([]cluster.SlotRange, error) {
	if len(ranges) == 0 || len(ranges) >= cluster.Slots {
		return nil, fmt.Errorf("ERR invalid number of slot ranges: %d", len(ranges))
	}
	ranges = append([]cluster.SlotRange(nil), ranges...)
	sort.SliceStable(ranges, func(i, j int) bool { return ranges[i].Start < ranges[j].Start })
	merged := ranges[:1]
	for _, r := range ranges[1:] {
		if last := &merged[len(merged)-1]; last.End+1 == r.Start {
			last.End = r.End
		} else {
			merged = append(merged, r)
		}
	}
	used := make([]bool, cluster.Slots)
	for _, r := range merged {
		if r.Start >= cluster.Slots || r.End >= cluster.Slots {
			return nil, fmt.Errorf("ERR slot range is out of range: %d-%d", r.Start, r.End)
		}
		if r.Start > r.End {
			return nil, fmt.Errorf("ERR start slot number %d is greater than end slot number %d", r.Start, r.End)
		}
		for j := r.Start; j <= r.End; j++ {
			if used[j] {
				return nil, fmt.Errorf("ERR Slot %d specified multiple times", j)
			}
			used[j] = true
		}
	}
	return merged, nil
}

// SlotRangesString is slotRangeArrayToString: "0-100 200-300".
func SlotRangesString(ranges []cluster.SlotRange) string {
	parts := make([]string, len(ranges))
	for i, r := range ranges {
		parts[i] = strconv.Itoa(r.Start) + "-" + strconv.Itoa(r.End)
	}
	return strings.Join(parts, " ")
}

// CommandSlot is the slot of the keys cmd names in args: -1 when it names
// none, -2 when they are in different slots.
func CommandSlot(cmd string, args [][]byte) int {
	cmd = CanonicalName(cmd)
	if !clusterHasKeys(cmd) {
		return -1
	}
	slot := -1
	for _, k := range clusterKeys(cmd, args) {
		s := hashslot.Key(k)
		if slot == -1 {
			slot = s
		} else if s != slot {
			return -2
		}
	}
	return slot
}

// clusterMigration is CLUSTER MIGRATION IMPORT | CANCEL | STATUS.
func clusterMigration(ctx *Context, args [][]byte) error {
	ctl := ctx.slotMigration()
	if ctl == nil {
		return ctx.WriteError(ErrClusterDisabled)
	}
	arityErr := errors.New("ERR wrong number of arguments for 'cluster|migration' command")
	switch strings.ToLower(string(args[0])) {
	case "import":
		if len(args) == 1 || (len(args)-1)%2 != 0 {
			return ctx.WriteError(arityErr)
		}
		ranges, err := ParseSlotRanges(args[1:])
		if errors.Is(err, errArity) {
			return ctx.WriteError(arityErr)
		}
		if err != nil {
			return ctx.WriteError(err)
		}
		id, err := ctl.Import(ranges)
		if err != nil {
			return ctx.WriteError(err)
		}
		return ctx.WriteBulkString(id)
	case "cancel":
		id, err := migrationTarget(args, arityErr)
		if err != nil {
			return ctx.WriteError(err)
		}
		return ctx.WriteInteger(int64(ctl.Cancel(id, "user request")))
	case "status":
		id, err := migrationTarget(args, arityErr)
		if err != nil {
			return ctx.WriteError(err)
		}
		tasks := ctl.Status(id)
		if id != "" && len(tasks) > 1 {
			tasks = tasks[:1]
		}
		out := make([]*resp.Value, len(tasks))
		for i := range tasks {
			t := &tasks[i]
			out[i] = resp.PairsValue([]*resp.Value{
				resp.BulkString("id"), resp.BulkString(t.ID),
				resp.BulkString("slots"), resp.BulkString(t.Slots),
				resp.BulkString("source"), resp.BulkString(t.Source),
				resp.BulkString("dest"), resp.BulkString(t.Dest),
				resp.BulkString("operation"), resp.BulkString(t.Operation),
				resp.BulkString("state"), resp.BulkString(t.State),
				resp.BulkString("last_error"), resp.BulkString(t.LastError),
				resp.BulkString("retries"), resp.IntegerValue(t.Retries),
				resp.BulkString("create_time"), resp.IntegerValue(t.CreateTime),
				resp.BulkString("start_time"), resp.IntegerValue(t.StartTime),
				resp.BulkString("end_time"), resp.IntegerValue(t.EndTime),
				resp.BulkString("write_pause_ms"), resp.IntegerValue(t.WritePauseMs),
			})
		}
		return ctx.WriteArray(out)
	}
	return ctx.WriteError(errors.New("ERR unknown argument"))
}

// migrationTarget reads "ID <task-id>" (the ID) or "ALL" ("").
func migrationTarget(args [][]byte, arityErr error) (string, error) {
	if len(args) != 2 && len(args) != 3 {
		return "", arityErr
	}
	switch strings.ToLower(string(args[1])) {
	case "id":
		if len(args) != 3 {
			return "", arityErr
		}
		if len(args[2]) == 0 {
			return "\x00", nil
		}
		return string(args[2]), nil
	case "all":
		if len(args) != 2 {
			return "", arityErr
		}
		return "", nil
	}
	return "", errors.New("ERR unknown argument")
}

// clusterSyncSlots is CLUSTER SYNCSLOTS, the protocol between the nodes of
// an atomic slot migration. Only internal connections may run it.
func clusterSyncSlots(ctx *Context, c *cluster.Cluster, args [][]byte) error {
	internal := ctx.FromMaster || (ctx.ClusterConn != nil && ctx.ClusterConn.Internal)
	if !internal {
		ctx.CloseConnection = true
		return ctx.WriteError(errors.New("ERR CLUSTER SYNCSLOTS subcommands are only allowed for internal clients"))
	}
	if c.IsReplica() {
		if !ctx.FromMaster {
			ctx.CloseConnection = true
			return ctx.WriteError(errors.New("ERR CLUSTER SYNCSLOTS subcommands are only allowed for master"))
		}
		if !strings.EqualFold(string(args[0]), "conf") {
			return nil
		}
	}
	ctl := ctx.slotMigration()
	if ctl == nil {
		return ctx.WriteError(ErrClusterDisabled)
	}
	return ctl.SyncSlots(ctx, args)
}

// cmdTRIMSLOTS deletes the keys of slots this node does not serve, as
// Redis 8.4's TRIMSLOTS RANGES <numranges> <start> <end> ... does.
func cmdTRIMSLOTS(ctx *Context) error {
	c := ctx.clusterState()
	if c == nil {
		return ctx.WriteError(errors.New("ERR This instance has cluster support disabled"))
	}
	if ctx.ArgCount() < 4 {
		return ctx.WriteError(arityError(ctx.Command))
	}
	if !strings.EqualFold(ctx.ArgString(0), "ranges") {
		return ctx.WriteError(errors.New("ERR missing ranges argument"))
	}
	n, ok := parseRedisInt(ctx.Arg(1))
	if !ok {
		return ctx.WriteError(ErrNotInteger)
	}
	if n < 1 || n > cluster.Slots || int64(ctx.ArgCount()) != 2+n*2 {
		return ctx.WriteError(errors.New("ERR invalid number of ranges"))
	}
	ranges, err := ParseSlotRanges(ctx.Args[2:])
	if errors.Is(err, errArity) {
		return ctx.WriteError(arityError(ctx.Command))
	}
	if err != nil {
		return ctx.WriteError(err)
	}
	if !ctx.Replaying && !ctx.FromMaster && !c.IsReplica() {
		for _, r := range ranges {
			for j := r.Start; j <= r.End; j++ {
				if c.Slot(j).Mine {
					return ctx.WriteError(fmt.Errorf("ERR the slot %d is served by this node", j))
				}
			}
		}
	}
	TrimSlots(clusterDB(ctx), ranges)
	return ctx.WriteOK()
}

// TrimSlots deletes every key of ranges from db.
func TrimSlots(db interface {
	KeysInSlot(slot, max int) []string
	Delete(key string) bool
}, ranges []cluster.SlotRange) int {
	n := 0
	for _, r := range ranges {
		for j := r.Start; j <= r.End; j++ {
			for {
				keys := db.KeysInSlot(j, 1000)
				if len(keys) == 0 {
					break
				}
				deleted := 0
				for _, k := range keys {
					if db.Delete(k) {
						deleted++
					}
				}
				n += deleted
				if deleted == 0 {
					break
				}
			}
		}
	}
	return n
}

// TrimSlotsCommand is the TRIMSLOTS command that records a trim of ranges
// in the AOF and the replication stream.
func TrimSlotsCommand(ranges []cluster.SlotRange) ExecutedCommand {
	args := [][]byte{[]byte("RANGES"), []byte(strconv.Itoa(len(ranges)))}
	for _, r := range ranges {
		args = append(args, []byte(strconv.Itoa(r.Start)), []byte(strconv.Itoa(r.End)))
	}
	return ExecutedCommand{Name: "TRIMSLOTS", Args: args}
}
