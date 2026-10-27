package command

import (
	"bytes"
	"errors"
	"fmt"
	"math"
	"net"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/cachestorm/cachestorm/internal/cluster"
	"github.com/cachestorm/cachestorm/internal/hashslot"
	"github.com/cachestorm/cachestorm/internal/persistence"
	"github.com/cachestorm/cachestorm/internal/resp"
	"github.com/cachestorm/cachestorm/internal/store"
)

// ErrClusterDisabled is the reply of a Redis instance running without
// cluster support to every cluster command.
var ErrClusterDisabled = errors.New("ERR This instance has cluster support disabled")

func RegisterClusterCommands(router *Router) {
	router.Register(&CommandDef{Name: "CLUSTER", Handler: cmdCLUSTER})
	router.Register(&CommandDef{Name: "CLUSTERINFO", Handler: clusterAlias("INFO")})
	router.Register(&CommandDef{Name: "CLUSTERNODES", Handler: clusterAlias("NODES")})
	router.Register(&CommandDef{Name: "CLUSTERSLOTS", Handler: clusterAlias("SLOTS")})
	router.Register(&CommandDef{Name: "MIGRATE", Handler: cmdMIGRATE})
	router.Register(&CommandDef{Name: "ASKING", Handler: cmdASKING})
	router.Register(&CommandDef{Name: "READONLY", Handler: cmdREADONLY})
	router.Register(&CommandDef{Name: "READWRITE", Handler: cmdREADWRITE})
}

// ClusterEnabled reports whether the server runs in cluster mode.
func (ctx *Context) ClusterEnabled() bool {
	return ctx.clusterState() != nil
}

// serverMode is the redis_mode INFO and HELLO report.
func (ctx *Context) serverMode() string {
	if ctx.ClusterEnabled() {
		return "cluster"
	}
	return "standalone"
}

func clusterAlias(sub string) func(*Context) error {
	return func(ctx *Context) error {
		if ctx.ArgCount() != 0 {
			return ctx.WriteError(arityError(ctx.Command))
		}
		ctx.Args = [][]byte{[]byte(sub)}
		return cmdCLUSTER(ctx)
	}
}

func cmdASKING(ctx *Context) error {
	if ctx.clusterState() == nil {
		return ctx.WriteError(ErrClusterDisabled)
	}
	if ctx.ClusterConn != nil {
		ctx.ClusterConn.Asking = true
	}
	return ctx.WriteOK()
}

func cmdREADONLY(ctx *Context) error {
	if ctx.clusterState() == nil {
		return ctx.WriteError(ErrClusterDisabled)
	}
	if ctx.ClusterConn != nil {
		ctx.ClusterConn.ReadOnly = true
	}
	return ctx.WriteOK()
}

func cmdREADWRITE(ctx *Context) error {
	if ctx.clusterState() == nil {
		return ctx.WriteError(ErrClusterDisabled)
	}
	if ctx.ClusterConn != nil {
		ctx.ClusterConn.ReadOnly = false
	}
	return ctx.WriteOK()
}

// clusterSubcommandArity is the Redis arity of each CLUSTER subcommand,
// counting CLUSTER and the subcommand; a negative arity is a minimum.
var clusterSubcommandArity = map[string]int{
	"ADDSLOTS": -3, "ADDSLOTSRANGE": -4, "BUMPEPOCH": 2, "COUNT-FAILURE-REPORTS": 3,
	"COUNTKEYSINSLOT": 3, "DELSLOTS": -3, "DELSLOTSRANGE": -4, "FAILOVER": -2,
	"FLUSHSLOTS": 2, "FORGET": 3, "GETKEYSINSLOT": 4, "HELP": 2, "INFO": 2,
	"KEYSLOT": 3, "LINKS": 2, "MEET": -4, "MYID": 2, "MYSHARDID": 2, "NODES": 2,
	"REPLICAS": 3, "REPLICATE": 3, "RESET": -2, "SAVECONFIG": 2,
	"SET-CONFIG-EPOCH": 3, "SETSLOT": -4, "SHARDS": 2, "SLAVES": 3, "SLOTS": 2,
}

var clusterHelp = []string{
	"CLUSTER <subcommand> [<arg> [value] [opt] ...]. Subcommands are:",
	"COUNTKEYSINSLOT <slot>",
	"    Return the number of keys in <slot>.",
	"GETKEYSINSLOT <slot> <count>",
	"    Return key names stored by current node in a slot.",
	"INFO",
	"    Return information about the cluster.",
	"KEYSLOT <key>",
	"    Return the hash slot for <key>.",
	"MYID",
	"    Return the node id.",
	"MYSHARDID",
	"    Return the node's shard id.",
	"NODES",
	"    Return cluster configuration seen by node. Output format:",
	"    <id> <ip:port@bus-port[,hostname]> <flags> <master> <pings> <pongs> <epoch> <link> <slot> ...",
	"REPLICAS <node-id>",
	"    Return <node-id> replicas.",
	"SLOTS",
	"    Return information about slots range mappings. Each range is made of:",
	"    start, end, master and replicas IP addresses, ports and ids",
	"SHARDS",
	"    Return information about slot range mappings and the nodes associated with them.",
	"ADDSLOTS <slot> [<slot> ...]",
	"    Assign slots to current node.",
	"ADDSLOTSRANGE <start slot> <end slot> [<start slot> <end slot> ...]",
	"    Assign slots which are between <start-slot> and <end-slot> to current node.",
	"BUMPEPOCH",
	"    Advance the cluster config epoch.",
	"COUNT-FAILURE-REPORTS <node-id>",
	"    Return number of failure reports for <node-id>.",
	"DELSLOTS <slot> [<slot> ...]",
	"    Delete slots information from current node.",
	"DELSLOTSRANGE <start slot> <end slot> [<start slot> <end slot> ...]",
	"    Delete slots information which are between <start-slot> and <end-slot> from current node.",
	"FAILOVER [FORCE|TAKEOVER]",
	"    Promote current replica node to being a master.",
	"FORGET <node-id>",
	"    Remove a node from the cluster.",
	"FLUSHSLOTS",
	"    Delete current node own slots information.",
	"MEET <ip> <port> [<bus-port>]",
	"    Connect nodes into a working cluster.",
	"REPLICATE <node-id>",
	"    Configure current node as replica to <node-id>.",
	"RESET [HARD|SOFT]",
	"    Reset current node (default: soft).",
	"SET-CONFIG-EPOCH <epoch>",
	"    Set config epoch of current node.",
	"SETSLOT <slot> (IMPORTING <node-id>|MIGRATING <node-id>|STABLE|NODE <node-id>)",
	"    Set slot state.",
	"SAVECONFIG",
	"    Force saving cluster configuration on disk.",
	"LINKS",
	"    Return information about all network links between this node and its peers.",
	"    Output format is an array where each array element is a map containing attributes of a link",
	"HELP",
	"    Print this help.",
}

var errInvalidSlot = errors.New("ERR Invalid or out of range slot")

// parseSlotArg is Redis' getSlotOrReply.
func parseSlotArg(arg string) (int, error) {
	n, err := strconv.ParseInt(arg, 10, 64)
	if err != nil || n < 0 || n >= cluster.Slots {
		return 0, errInvalidSlot
	}
	return int(n), nil
}

func cmdCLUSTER(ctx *Context) error {
	sub := strings.ToUpper(ctx.ArgString(0))
	arity, known := clusterSubcommandArity[sub]
	if !known {
		return ctx.WriteError(errors.New("ERR unknown subcommand '" + ctx.ArgString(0) + "'. Try CLUSTER HELP."))
	}
	if n := ctx.ArgCount() + 1; (arity >= 0 && n != arity) || (arity < 0 && n < -arity) {
		return ctx.WriteError(errors.New("ERR wrong number of arguments for 'cluster|" + strings.ToLower(sub) + "' command"))
	}
	c := ctx.clusterState()
	if c == nil {
		return ctx.WriteError(ErrClusterDisabled)
	}
	args := ctx.Args[1:]
	switch sub {
	case "HELP":
		out := make([]*resp.Value, len(clusterHelp))
		for i, l := range clusterHelp {
			out[i] = resp.SimpleString(l)
		}
		return ctx.WriteArray(out)
	case "INFO":
		return ctx.Writer.WriteVerbatim("txt", c.Info())
	case "MYID":
		return ctx.WriteBulkString(c.MyID())
	case "MYSHARDID":
		return ctx.WriteBulkString(c.MyShardID())
	case "NODES":
		return ctx.Writer.WriteVerbatim("txt", c.Nodes())
	case "SLOTS":
		return ctx.WriteArray(clusterSlotsReply(c))
	case "SHARDS":
		return ctx.WriteArray(clusterShardsReply(ctx, c))
	case "LINKS":
		links := c.Links()
		out := make([]*resp.Value, len(links))
		for i, l := range links {
			out[i] = resp.PairsValue([]*resp.Value{
				resp.BulkString("direction"), resp.BulkString(l.Direction),
				resp.BulkString("node"), resp.BulkString(l.Node),
				resp.BulkString("create-time"), resp.IntegerValue(l.CreateTime),
				resp.BulkString("events"), resp.BulkString(l.Events),
				resp.BulkString("send-buffer-allocated"), resp.IntegerValue(l.SendBuffer),
				resp.BulkString("send-buffer-used"), resp.IntegerValue(l.SendBuffer),
			})
		}
		return ctx.WriteArray(out)
	case "MEET":
		if len(args) > 3 {
			return ctx.WriteError(errors.New("ERR unknown subcommand or wrong number of arguments for '" + ctx.ArgString(0) + "'. Try CLUSTER HELP."))
		}
		port, err := strconv.ParseInt(string(args[1]), 10, 64)
		if err != nil {
			return ctx.WriteError(fmt.Errorf("ERR Invalid base port specified: %s", args[1]))
		}
		cport := port + 10000
		if len(args) == 3 {
			if cport, err = strconv.ParseInt(string(args[2]), 10, 64); err != nil {
				return ctx.WriteError(fmt.Errorf("ERR Invalid bus port specified: %s", args[2]))
			}
		}
		if port < 0 || port > 65535 || cport < 0 || cport > 65535 || c.Meet(string(args[0]), int(port), int(cport)) != nil {
			return ctx.WriteError(fmt.Errorf("ERR Invalid node address specified: %s:%s", args[0], args[1]))
		}
		return ctx.WriteOK()
	case "FORGET":
		if err := c.Forget(string(args[0])); err != nil {
			return ctx.WriteError(err)
		}
		return ctx.WriteOK()
	case "KEYSLOT":
		return ctx.WriteInteger(int64(hashslot.Key(string(args[0]))))
	case "COUNTKEYSINSLOT":
		slot, err := strconv.ParseInt(string(args[0]), 10, 64)
		if err != nil {
			return ctx.WriteError(ErrNotInteger)
		}
		if slot < 0 || slot >= cluster.Slots {
			return ctx.WriteError(errors.New("ERR Invalid slot"))
		}
		return ctx.WriteInteger(int64(clusterDB(ctx).CountKeysInSlot(int(slot))))
	case "GETKEYSINSLOT":
		slot, err := strconv.ParseInt(string(args[0]), 10, 64)
		if err != nil {
			return ctx.WriteError(ErrNotInteger)
		}
		max, err := strconv.ParseInt(string(args[1]), 10, 64)
		if err != nil {
			return ctx.WriteError(ErrNotInteger)
		}
		if slot < 0 || slot >= cluster.Slots || max < 0 {
			return ctx.WriteError(errors.New("ERR Invalid slot or number of keys"))
		}
		if max > math.MaxInt32 {
			max = math.MaxInt32
		}
		keys := clusterDB(ctx).KeysInSlot(int(slot), int(max))
		out := make([]*resp.Value, len(keys))
		for i, k := range keys {
			out[i] = resp.BulkString(k)
		}
		return ctx.WriteArray(out)
	case "ADDSLOTS", "DELSLOTS":
		ranges := make([]cluster.SlotRange, 0, len(args))
		for _, a := range args {
			slot, err := parseSlotArg(string(a))
			if err != nil {
				return ctx.WriteError(err)
			}
			ranges = append(ranges, cluster.SlotRange{Start: slot, End: slot})
		}
		return clusterAssign(ctx, c, ranges, sub == "DELSLOTS", false)
	case "ADDSLOTSRANGE", "DELSLOTSRANGE":
		if len(args)%2 != 0 {
			return ctx.WriteError(errors.New("ERR wrong number of arguments for 'cluster|" + strings.ToLower(sub) + "' command"))
		}
		return clusterAssign(ctx, c, nil, sub == "DELSLOTSRANGE", true)
	case "FLUSHSLOTS":
		if clusterDB(ctx).KeyCount() != 0 {
			return ctx.WriteError(errors.New("ERR DB must be empty to perform CLUSTER FLUSHSLOTS.")) //nolint:staticcheck // Redis reply text
		}
		c.FlushSlots()
		return ctx.WriteOK()
	case "SETSLOT":
		return clusterSetSlot(ctx, c, args)
	case "RESET":
		hard := false
		if len(args) > 1 {
			return ctx.WriteError(errors.New("ERR unknown subcommand or wrong number of arguments for '" + ctx.ArgString(0) + "'. Try CLUSTER HELP."))
		}
		if len(args) == 1 {
			switch strings.ToUpper(string(args[0])) {
			case "HARD":
				hard = true
			case "SOFT":
			default:
				return ctx.WriteError(ErrSyntaxError)
			}
		}
		replica := c.IsReplica()
		if !replica && clusterDB(ctx).KeyCount() != 0 {
			return ctx.WriteError(errors.New("ERR CLUSTER RESET can't be called with master nodes containing keys"))
		}
		c.Reset(hard)
		if replica {
			// A replica that leaves its master drops the data it had from it.
			ctx.databases().FlushAll()
			ctx.PropagateAs(ExecutedCommand{Name: "FLUSHALL", DB: ctx.DB})
		}
		return ctx.WriteOK()
	case "SAVECONFIG":
		if err := c.Save(); err != nil {
			return ctx.WriteError(errors.New("ERR error saving the cluster node config: " + saveErrorText(err)))
		}
		return ctx.WriteOK()
	case "BUMPEPOCH":
		bumped, epoch := c.BumpEpoch()
		word := "STILL"
		if bumped {
			word = "BUMPED"
		}
		return ctx.WriteSimpleString(word + " " + strconv.FormatUint(epoch, 10))
	case "SET-CONFIG-EPOCH":
		epoch, err := strconv.ParseInt(string(args[0]), 10, 64)
		if err != nil {
			return ctx.WriteError(ErrNotInteger)
		}
		if err := c.SetConfigEpoch(epoch); err != nil {
			return ctx.WriteError(err)
		}
		return ctx.WriteOK()
	case "COUNT-FAILURE-REPORTS":
		n, err := c.CountFailureReports(string(args[0]))
		if err != nil {
			return ctx.WriteError(err)
		}
		return ctx.WriteInteger(int64(n))
	case "REPLICAS", "SLAVES":
		lines, err := c.Replicas(string(args[0]))
		if err != nil {
			return ctx.WriteError(err)
		}
		out := make([]*resp.Value, len(lines))
		for i, l := range lines {
			out[i] = resp.BulkString(l)
		}
		return ctx.WriteArray(out)
	case "REPLICATE":
		if err := c.Replicate(string(args[0]), clusterDB(ctx).KeyCount() == 0); err != nil {
			return ctx.WriteError(err)
		}
		return ctx.WriteOK()
	case "FAILOVER":
		if len(args) > 1 {
			return ctx.WriteError(errors.New("ERR unknown subcommand or wrong number of arguments for '" + ctx.ArgString(0) + "'. Try CLUSTER HELP."))
		}
		force, takeover := false, false
		if len(args) == 1 {
			switch strings.ToUpper(string(args[0])) {
			case "FORCE":
				force = true
			case "TAKEOVER":
				force, takeover = true, true
			default:
				return ctx.WriteError(ErrSyntaxError)
			}
		}
		if err := c.Failover(force, takeover); err != nil {
			return ctx.WriteError(err)
		}
		return ctx.WriteOK()
	}
	return ctx.WriteError(errors.New("ERR unknown subcommand '" + ctx.ArgString(0) + "'. Try CLUSTER HELP."))
}

// clusterDB is database 0, the only one cluster mode serves.
func clusterDB(ctx *Context) *store.Store {
	if dbs := ctx.databases(); dbs != nil {
		if db := dbs.DB(0); db != nil {
			return db
		}
	}
	return ctx.Store
}

// clusterAssign runs ADDSLOTS/DELSLOTS (ranges given) and their RANGE forms
// (pairs read from the arguments), with Redis' order of checks.
func clusterAssign(ctx *Context, c *cluster.Cluster, ranges []cluster.SlotRange, del, pairs bool) error {
	seen := make([]bool, cluster.Slots)
	if pairs {
		args := ctx.Args[1:]
		for i := 0; i+1 < len(args); i += 2 {
			start, err := parseSlotArg(string(args[i]))
			if err != nil {
				return ctx.WriteError(err)
			}
			end, err := parseSlotArg(string(args[i+1]))
			if err != nil {
				return ctx.WriteError(err)
			}
			if start > end {
				return ctx.WriteError(fmt.Errorf("ERR start slot number %d is greater than end slot number %d", start, end))
			}
			r := cluster.SlotRange{Start: start, End: end}
			if err := c.CheckSlots(seen, del, r); err != nil {
				return ctx.WriteError(err)
			}
			ranges = append(ranges, r)
		}
	} else {
		for _, r := range ranges {
			if err := c.CheckSlots(seen, del, r); err != nil {
				return ctx.WriteError(err)
			}
		}
	}
	var err error
	if del {
		err = c.DelSlots(ranges)
	} else {
		err = c.AddSlots(ranges)
	}
	if err != nil {
		return ctx.WriteError(err)
	}
	return ctx.WriteOK()
}

func clusterSetSlot(ctx *Context, c *cluster.Cluster, args [][]byte) error {
	if c.IsReplica() {
		return ctx.WriteError(errors.New("ERR Please use SETSLOT only with masters.")) //nolint:staticcheck // Redis reply text
	}
	slot, err := parseSlotArg(string(args[0]))
	if err != nil {
		return ctx.WriteError(err)
	}
	action := strings.ToUpper(string(args[1]))
	node := ""
	valid := false
	switch action {
	case "MIGRATING", "IMPORTING", "NODE":
		valid = len(args) == 3
	case "STABLE":
		valid = len(args) == 2
	}
	if !valid {
		return ctx.WriteError(errors.New("ERR Invalid CLUSTER SETSLOT action or number of arguments. Try CLUSTER HELP"))
	}
	if len(args) >= 3 {
		node = string(args[2])
	}
	db := clusterDB(ctx)
	if err := c.SetSlot(slot, action, node, db.CountKeysInSlot); err != nil {
		return ctx.WriteError(err)
	}
	return ctx.WriteOK()
}

func saveErrorText(err error) string {
	var pathErr *os.PathError
	if errors.As(err, &pathErr) {
		return pathErr.Err.Error()
	}
	return err.Error()
}

func clusterNodeValue(n cluster.NodeView) *resp.Value {
	var meta []*resp.Value
	if n.Hostname != "" {
		meta = append(meta, resp.BulkString("hostname"), resp.BulkString(n.Hostname))
	}
	return resp.ArrayValue([]*resp.Value{
		resp.BulkString(n.IP),
		resp.IntegerValue(int64(n.Port)),
		resp.BulkString(n.ID),
		resp.PairsValue(meta),
	})
}

// clusterSlotsReply is the CLUSTER SLOTS reply: per range of slots, the
// start, the end, the master and its replicas, each as
// [ip, port, id, {metadata}].
func clusterSlotsReply(c *cluster.Cluster) []*resp.Value {
	entries := c.SlotMap()
	out := make([]*resp.Value, 0, len(entries))
	for _, e := range entries {
		item := []*resp.Value{resp.IntegerValue(int64(e.Start)), resp.IntegerValue(int64(e.End))}
		for _, n := range e.Nodes {
			item = append(item, clusterNodeValue(n))
		}
		out = append(out, resp.ArrayValue(item))
	}
	return out
}

// clusterShardsReply is the CLUSTER SHARDS reply: per shard, a map of its
// slot pairs and of its nodes' details.
func clusterShardsReply(ctx *Context, c *cluster.Cluster) []*resp.Value {
	offset := replicationOffset(ctx)
	shards := c.Shards()
	out := make([]*resp.Value, 0, len(shards))
	for _, sh := range shards {
		slots := make([]*resp.Value, 0, 2*len(sh.Slots))
		for _, r := range sh.Slots {
			slots = append(slots, resp.IntegerValue(int64(r.Start)), resp.IntegerValue(int64(r.End)))
		}
		nodes := make([]*resp.Value, 0, len(sh.Nodes))
		for _, n := range sh.Nodes {
			fields := []*resp.Value{resp.BulkString("id"), resp.BulkString(n.ID)}
			if n.Port != 0 {
				fields = append(fields, resp.BulkString("port"), resp.IntegerValue(int64(n.Port)))
			}
			if n.TLSPort != 0 {
				fields = append(fields, resp.BulkString("tls-port"), resp.IntegerValue(int64(n.TLSPort)))
			}
			fields = append(fields, resp.BulkString("ip"), resp.BulkString(n.IP), resp.BulkString("endpoint"), resp.BulkString(n.IP))
			if n.Hostname != "" {
				fields = append(fields, resp.BulkString("hostname"), resp.BulkString(n.Hostname))
			}
			nodeOffset := n.ReplOffset
			if n.Myself {
				nodeOffset = offset
			}
			role, health := "master", "online"
			if n.Replica {
				role = "replica"
			}
			switch {
			case n.Failing:
				health = "fail"
			case n.Replica && nodeOffset == 0:
				health = "loading"
			}
			fields = append(fields,
				resp.BulkString("role"), resp.BulkString(role),
				resp.BulkString("replication-offset"), resp.IntegerValue(nodeOffset),
				resp.BulkString("health"), resp.BulkString(health))
			nodes = append(nodes, resp.PairsValue(fields))
		}
		out = append(out, resp.PairsValue([]*resp.Value{
			resp.BulkString("slots"), resp.ArrayValue(slots),
			resp.BulkString("nodes"), resp.ArrayValue(nodes),
		}))
	}
	return out
}

// replicationOffset is master_repl_offset from INFO replication.
func replicationOffset(ctx *Context) int64 {
	r := ctx.dispatcher()
	if r == nil {
		return 0
	}
	fn := r.replicationInfo()
	if fn == nil {
		return 0
	}
	for _, line := range strings.Split(fn(), "\r\n") {
		if v, ok := strings.CutPrefix(line, "master_repl_offset:"); ok {
			if n, err := strconv.ParseInt(v, 10, 64); err == nil {
				return n
			}
		}
	}
	return 0
}

// cmdMIGRATE implements MIGRATE host port key|"" destination-db timeout
// [COPY] [REPLACE] [AUTH password | AUTH2 username password]
// [KEYS key [key ...]] as Redis' migrateCommand. The keys are sent pipelined
// as RESTORE (RESTORE-ASKING in cluster mode, so an importing target accepts
// them) with their DUMP payload and remaining TTL, while a reader collects
// the replies; the timeout bounds each network operation. Without COPY the
// keys the target accepted are then deleted here, which the AOF and the
// replicas record as DEL. The command holds the write gate exclusively, so
// no write lands on a key between its dump and its deletion.
func cmdMIGRATE(ctx *Context) error {
	if ctx.ArgCount() < 5 {
		return ctx.WriteError(arityError("migrate"))
	}
	host, port := ctx.ArgString(0), ctx.ArgString(1)
	copyKeys, replace := false, false
	var auth [][]byte
	keys := []string{ctx.ArgString(2)}
	for i := 5; i < ctx.ArgCount(); i++ {
		more := ctx.ArgCount() - 1 - i
		switch strings.ToUpper(ctx.ArgString(i)) {
		case "COPY":
			copyKeys = true
		case "REPLACE":
			replace = true
		case "AUTH":
			if more < 1 {
				return ctx.WriteError(ErrSyntaxError)
			}
			auth = [][]byte{ctx.Arg(i + 1)}
			i++
		case "AUTH2":
			if more < 2 {
				return ctx.WriteError(ErrSyntaxError)
			}
			auth = [][]byte{ctx.Arg(i + 1), ctx.Arg(i + 2)}
			i += 2
		case "KEYS":
			if ctx.ArgString(2) != "" {
				return ctx.WriteError(errors.New("ERR When using MIGRATE KEYS option, the key argument must be set to the empty string"))
			}
			keys = nil
			for i++; i < ctx.ArgCount(); i++ {
				keys = append(keys, ctx.ArgString(i))
			}
		default:
			return ctx.WriteError(ErrSyntaxError)
		}
	}
	timeoutMS, err := strconv.ParseInt(ctx.ArgString(4), 10, 64)
	if err != nil {
		return ctx.WriteError(ErrNotInteger)
	}
	db, err := strconv.ParseInt(ctx.ArgString(3), 10, 64)
	if err != nil {
		return ctx.WriteError(ErrNotInteger)
	}
	if timeoutMS <= 0 {
		timeoutMS = 1000
	}

	type item struct {
		key     string
		ttl     int64
		payload []byte
	}
	var items []item
	now := time.Now().UnixMilli()
	for _, key := range keys {
		entry, ok := ctx.Store.Get(key)
		if !ok || entry.IsExpired() {
			continue
		}
		ttl := int64(0)
		if at := entry.ExpiresAt.Load(); at > 0 {
			if ttl = at - now; ttl < 0 {
				continue
			}
			if ttl < 1 {
				ttl = 1
			}
		}
		payload, err := persistence.EncodeDump(entry.Value)
		if err != nil {
			return ctx.WriteError(err)
		}
		items = append(items, item{key, ttl, payload})
	}
	if len(items) == 0 {
		return ctx.WriteSimpleString("NOKEY")
	}

	timeout := time.Duration(timeoutMS) * time.Millisecond
	raw, err := net.DialTimeout("tcp", net.JoinHostPort(host, port), timeout)
	if err != nil {
		return ctx.WriteError(fmt.Errorf("IOERR error or timeout connecting to the client"))
	}
	conn := &deadlineConn{Conn: raw, timeout: timeout}
	defer conn.Close()

	restore := []byte("RESTORE")
	if ctx.ClusterEnabled() {
		restore = []byte("RESTORE-ASKING")
	}
	var out bytes.Buffer
	w := resp.NewWriter(&out)
	send := func(args ...[]byte) {
		vals := make([]*resp.Value, len(args))
		for i, a := range args {
			vals[i] = resp.BulkBytes(a)
		}
		_ = w.WriteArray(vals)
	}
	if auth != nil {
		send(append([][]byte{[]byte("AUTH")}, auth...)...)
	}
	send([]byte("SELECT"), []byte(strconv.FormatInt(db, 10)))
	for _, it := range items {
		args := [][]byte{restore, []byte(it.key), []byte(strconv.FormatInt(it.ttl, 10)), it.payload}
		if replace {
			args = append(args, []byte("REPLACE"))
		}
		send(args...)
	}

	// The replies are read while the commands are written, so a target
	// that answers before it has read everything never stalls the pipe.
	expected := len(items) + 1
	if auth != nil {
		expected++
	}
	type reply struct {
		v   *resp.Value
		err error
	}
	replies := make(chan reply, expected)
	go func() {
		rd := resp.NewReader(conn)
		for i := 0; i < expected; i++ {
			v, err := rd.ReadValue()
			replies <- reply{v, err}
			if err != nil {
				return
			}
		}
	}()
	const chunk = 64 * 1024
	data := out.Bytes()
	for len(data) > 0 {
		n := min(chunk, len(data))
		if _, err := conn.Write(data[:n]); err != nil {
			conn.Close()
			return ctx.WriteError(fmt.Errorf("IOERR error or timeout writing to target instance"))
		}
		data = data[n:]
	}
	readErr := errors.New("IOERR error or timeout reading to target instance")
	for i := expected - len(items); i > 0; i-- {
		r := <-replies
		if r.err != nil {
			conn.Close()
			return ctx.WriteError(readErr)
		}
		if r.v.Type == resp.TypeError {
			conn.Close()
			return ctx.Writer.WriteError("ERR Target instance replied with error: " + r.v.Err)
		}
	}
	var moved []string
	var firstErr string
	ioFailed := false
	for _, it := range items {
		r := <-replies
		if r.err != nil {
			ioFailed = true
			break
		}
		if r.v.Type == resp.TypeError {
			if firstErr == "" {
				firstErr = r.v.Err
			}
			continue
		}
		moved = append(moved, it.key)
	}
	if !copyKeys && len(moved) > 0 {
		delArgs := make([][]byte, 0, len(moved))
		for _, key := range moved {
			if ctx.Store.Delete(key) {
				delArgs = append(delArgs, []byte(key))
			}
		}
		if len(delArgs) > 0 {
			ctx.PropagateAs(ExecutedCommand{Name: "DEL", Args: delArgs, DB: ctx.DB})
		} else {
			ctx.PropagateAs()
		}
	} else {
		ctx.PropagateAs()
	}
	switch {
	case firstErr != "":
		return ctx.Writer.WriteError("ERR Target instance replied with error: " + firstErr)
	case ioFailed:
		return ctx.WriteError(readErr)
	}
	return ctx.WriteOK()
}

// deadlineConn applies the MIGRATE timeout to each read and write, as
// Redis' syncRead and syncWrite do, rather than to the whole transfer.
type deadlineConn struct {
	net.Conn
	timeout time.Duration
}

func (c *deadlineConn) Read(p []byte) (int, error) {
	_ = c.SetReadDeadline(time.Now().Add(c.timeout))
	return c.Conn.Read(p)
}

func (c *deadlineConn) Write(p []byte) (int, error) {
	_ = c.SetWriteDeadline(time.Now().Add(c.timeout))
	return c.Conn.Write(p)
}
