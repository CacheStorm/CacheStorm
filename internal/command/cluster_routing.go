package command

import (
	"strconv"
	"strings"

	"github.com/cachestorm/cachestorm/internal/cluster"
	"github.com/cachestorm/cachestorm/internal/hashslot"
)

// ClusterConn is the cluster state of a connection: READONLY and ASKING,
// and what routing found for the command running: the slot of its keys plus
// one (0 when it named none) and whether it ran after ASKING.
type ClusterConn struct {
	ReadOnly bool
	Asking   bool
	slot     int
	asking   bool
}

// commandSlot is the slot routing found for the running command, plus one.
func (ctx *Context) commandSlot() int {
	if ctx.ClusterConn == nil {
		return 0
	}
	return ctx.ClusterConn.slot
}

// SetCluster turns cluster mode on. It is called once, before the server
// accepts connections.
func (r *Router) SetCluster(c *cluster.Cluster) {
	r.cluster = c
}

// Cluster returns the cluster state, nil when cluster mode is off.
func (r *Router) Cluster() *cluster.Cluster {
	return r.cluster
}

func (ctx *Context) clusterState() *cluster.Cluster {
	if r := ctx.dispatcher(); r != nil {
		return r.cluster
	}
	return nil
}

// takeAsking returns the connection's ASKING flag and clears it: the flag
// lasts for one command, the one after ASKING.
func (ctx *Context) takeAsking() bool {
	conn := ctx.ClusterConn
	if conn == nil {
		return false
	}
	asking := conn.Asking
	if ctx.Command != "ASKING" {
		conn.Asking = false
	}
	return asking
}

type redirectKind int

const (
	redirNone redirectKind = iota
	redirMoved
	redirAsk
	redirCrossSlot
	redirUnstable
	redirDown
	redirDownReadOnly
	redirUnbound
)

type redirect struct {
	kind redirectKind
	slot int
	to   cluster.Addr
}

func (rd redirect) message() string {
	switch rd.kind {
	case redirMoved:
		return "MOVED " + strconv.Itoa(rd.slot) + " " + rd.to.Endpoint()
	case redirAsk:
		return "ASK " + strconv.Itoa(rd.slot) + " " + rd.to.Endpoint()
	case redirCrossSlot:
		return "CROSSSLOT Keys in request don't hash to the same slot"
	case redirUnstable:
		return "TRYAGAIN Multiple keys request during rehashing of slot"
	case redirDown:
		return "CLUSTERDOWN The cluster is down"
	case redirDownReadOnly:
		return "CLUSTERDOWN The cluster is down and only accepts read commands"
	case redirUnbound:
		return "CLUSTERDOWN Hash slot not served"
	}
	return ""
}

// clusterContainerKeys returns the keys of the container commands whose
// subcommands name a key.
func clusterContainerKeys(cmd string, args [][]byte) []string {
	if len(args) < 2 {
		return nil
	}
	sub := strings.ToUpper(string(args[0]))
	switch cmd {
	case "OBJECT":
		switch sub {
		case "ENCODING", "FREQ", "IDLETIME", "REFCOUNT":
			return []string{string(args[1])}
		}
	case "MEMORY":
		if sub == "USAGE" {
			return []string{string(args[1])}
		}
	case "XINFO":
		switch sub {
		case "CONSUMERS", "GROUPS", "STREAM":
			return []string{string(args[1])}
		}
	case "XGROUP":
		switch sub {
		case "CREATE", "CREATECONSUMER", "DELCONSUMER", "DESTROY", "SETID":
			return []string{string(args[1])}
		}
	}
	return nil
}

// clusterHasKeys reports whether cmd can name keys at all. A command that
// cannot is never redirected.
func clusterHasKeys(cmd string) bool {
	spec, ok := redisCommandSpecs[cmd]
	if !ok {
		return !aclNoKeyCommands[cmd]
	}
	if spec.FirstKey > 0 {
		return true
	}
	switch cmd {
	case "OBJECT", "MEMORY", "XINFO", "XGROUP":
		return true
	}
	for _, f := range spec.Flags {
		if f == "movablekeys" {
			return true
		}
	}
	return false
}

// clusterKeys returns the keys cmd names in args for routing: the key specs
// of a Redis command, the movable-key logic of CommandKeys otherwise.
func clusterKeys(cmd string, args [][]byte) []string {
	spec, ok := redisCommandSpecs[cmd]
	if !ok {
		return CommandKeys(cmd, args)
	}
	switch cmd {
	case "OBJECT", "MEMORY", "XINFO", "XGROUP":
		return clusterContainerKeys(cmd, args)
	case "SORT", "SORT_RO":
		if len(args) == 0 {
			return nil
		}
		return append([]string{string(args[0])}, storeKeyAfter(args)...)
	}
	for _, f := range spec.Flags {
		if f == "movablekeys" {
			return CommandKeys(cmd, args)
		}
	}
	if spec.FirstKey <= 0 {
		return nil
	}
	argc := len(args) + 1
	last := spec.LastKey
	if last < 0 {
		last += argc
	}
	step := spec.KeyStep
	if step < 1 {
		step = 1
	}
	var keys []string
	for i := spec.FirstKey; i <= last && i < argc; i += step {
		keys = append(keys, string(args[i-1]))
	}
	return keys
}

func isShardChannelCommand(cmd string) bool {
	switch cmd {
	case "SPUBLISH", "SSUBSCRIBE", "SUNSUBSCRIBE":
		return true
	}
	return false
}

// keyPresent reports whether key holds a live value, without touching it.
func keyPresent(ctx *Context, key string) bool {
	entry, ok := ctx.Store.GetShard(key).Get(key)
	return ok && !entry.IsExpired()
}

// clusterRoute is Redis' getNodeByQuery: it decides whether this node
// serves cmds, the command or the queued commands of an EXEC, and the
// redirection to answer with when it does not. top is the command the client
// sent.
func (r *Router) clusterRoute(ctx *Context, top string, cmds []queuedCommand, asking bool) redirect {
	var (
		first                        string
		haveKey, multipleKeys        bool
		migrating, importing, pubsub bool
		write                        bool
		missing, existing, slot      int
		st                           cluster.SlotState
	)
	for _, qc := range cmds {
		if isShardChannelCommand(qc.cmd) {
			pubsub = true
		}
		if isWriteCommand(qc.cmd) {
			write = true
		}
		for _, key := range clusterKeys(qc.cmd, qc.args) {
			ks := hashslot.Key(key)
			if !haveKey {
				haveKey, first, slot = true, key, ks
				st = r.cluster.Slot(slot)
				if !st.Assigned {
					return redirect{kind: redirUnbound}
				}
				if st.Mine && st.MigratingSet {
					migrating = true
				} else if st.Importing {
					importing = true
				}
			} else if ks != slot {
				return redirect{kind: redirCrossSlot}
			} else if key != first {
				multipleKeys = true
			}
			if (migrating || importing) && !pubsub {
				if keyPresent(ctx, key) {
					existing++
				} else {
					missing++
				}
			}
		}
	}
	if !haveKey {
		return redirect{}
	}
	if !st.Healthy && !pubsub {
		if !st.AllowReadsWhenDown {
			return redirect{kind: redirDown}
		}
		if write {
			return redirect{kind: redirDownReadOnly}
		}
	}
	if ctx.ClusterConn != nil {
		ctx.ClusterConn.slot = slot + 1
	}
	if (migrating || importing) && top == "MIGRATE" {
		return redirect{}
	}
	if migrating && missing > 0 {
		if existing > 0 {
			return redirect{kind: redirUnstable}
		}
		return redirect{kind: redirAsk, slot: slot, to: st.Migrating}
	}
	if importing && (asking || commandHasFlag(top, "asking")) {
		if multipleKeys && missing > 0 {
			return redirect{kind: redirUnstable}
		}
		return redirect{}
	}
	if !st.Mine {
		return redirect{kind: redirMoved, slot: slot, to: st.Owner}
	}
	return redirect{}
}

// clusterRedirect applies cluster routing to the command in ctx before it
// runs or is queued, and answers with the redirection when this node does not
// serve it. A redirected command discards the transaction it was sent in.
func (r *Router) clusterRedirect(ctx *Context, asking bool) bool {
	if ctx.ClusterConn == nil {
		ctx.ClusterConn = &ClusterConn{}
	}
	ctx.ClusterConn.slot, ctx.ClusterConn.asking = 0, asking
	if ctx.FromMaster || ctx.Replaying {
		return false
	}
	var cmds []queuedCommand
	inMulti := ctx.Transaction != nil && ctx.Transaction.IsActive()
	if ctx.Command == "EXEC" {
		if !inMulti {
			return false
		}
		cmds = ctx.Transaction.GetQueued()
	} else {
		if !clusterHasKeys(ctx.Command) {
			return false
		}
		cmds = []queuedCommand{{cmd: ctx.Command, args: ctx.Args}}
	}
	rd := r.clusterRoute(ctx, ctx.Command, cmds, asking)
	if rd.kind == redirNone {
		return false
	}
	if inMulti {
		if ctx.Command == "EXEC" {
			ctx.Transaction.Clear()
			ctx.Transaction.ClearWatch()
		} else {
			ctx.Transaction.Abort()
		}
	}
	_ = ctx.Writer.WriteError(rd.message())
	return true
}

// clusterScriptCheck is Redis' scriptVerifyClusterState for one command a
// script runs: its keys must be served here and, unless the script allows
// cross-slot keys, be in the slot of the keys the script used before.
func (s *scriptRun) clusterScriptCheck(name string, args [][]byte) string {
	r := s.router
	if r == nil || r.cluster == nil || s.ctx.FromMaster || s.ctx.Replaying {
		return ""
	}
	if !clusterHasKeys(name) {
		return ""
	}
	asking := s.ctx.ClusterConn != nil && s.ctx.ClusterConn.asking
	probe := *s.ctx
	probe.ClusterConn = &ClusterConn{}
	rd := r.clusterRoute(&probe, name, []queuedCommand{{cmd: name, args: args}}, asking)
	switch rd.kind {
	case redirNone:
	case redirDownReadOnly:
		return "ERR Script attempted to execute a write command while the cluster is down and readonly"
	case redirDown:
		return "ERR Script attempted to execute a command while the cluster is down"
	case redirCrossSlot:
		return "ERR Command '" + strings.ToLower(name) + "' in script attempted to access keys that don't hash to the same slot"
	default:
		return "ERR Script attempted to access a non local key in a cluster node script"
	}
	if found := probe.ClusterConn.slot; found != 0 && !s.crossSlot {
		if s.slot == 0 {
			s.slot = found
		} else if s.slot != found {
			return "ERR Script attempted to access keys that do not hash to the same slot"
		}
	}
	return ""
}
