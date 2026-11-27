package command

import (
	"errors"
	"strconv"
	"strings"
	"time"
	"unsafe"

	"github.com/cachestorm/cachestorm/internal/hashslot"
	"github.com/cachestorm/cachestorm/internal/resp"
	"github.com/cachestorm/cachestorm/internal/store"
)

func RegisterTagCommands(router *Router) {
	router.Register(&CommandDef{Name: "SETTAG", Handler: cmdSETTAG})
	router.Register(&CommandDef{Name: "TAGS", Handler: cmdTAGS})
	router.Register(&CommandDef{Name: "ADDTAG", Handler: cmdADDTAG})
	router.Register(&CommandDef{Name: "REMTAG", Handler: cmdREMTAG})
	router.Register(&CommandDef{Name: "INVALIDATE", Handler: cmdINVALIDATE})
	router.Register(&CommandDef{Name: "TAGKEYS", Handler: cmdTAGKEYS})
	router.Register(&CommandDef{Name: "TAGCOUNT", Handler: cmdTAGCOUNT})
	router.Register(&CommandDef{Name: "TAGLINK", Handler: cmdTAGLINK})
	router.Register(&CommandDef{Name: "TAGUNLINK", Handler: cmdTAGUNLINK})
	router.Register(&CommandDef{Name: "TAGCHILDREN", Handler: cmdTAGCHILDREN})
	router.Register(&CommandDef{Name: "TAGSYNC", Handler: cmdTAGSYNC})
}

// cmdTAGSYNC answers the tag hierarchy with every link's version, removed
// links included, as [parent child live version node] entries: the nodes
// of a cluster whose hierarchy digests differ fetch each other's and merge
// them. Only another node's internal connection may run it.
func cmdTAGSYNC(ctx *Context) error {
	if ctx.ClusterConn == nil || !ctx.ClusterConn.Internal {
		return ctx.WriteError(errors.New("ERR TAGSYNC is used between the nodes of a cluster"))
	}
	edges := ctx.Store.GetTagIndex().Edges()
	out := make([]*resp.Value, 0, len(edges))
	for _, e := range edges {
		live := int64(0)
		if e.Live {
			live = 1
		}
		out = append(out, resp.ArrayValue([]*resp.Value{
			resp.BulkString(e.Parent), resp.BulkString(e.Child), resp.IntegerValue(live),
			resp.BulkString(strconv.FormatUint(e.Version, 10)), resp.BulkString(e.Node),
		}))
	}
	return ctx.WriteArray(out)
}

// ParseTagSync parses a TAGSYNC reply.
func ParseTagSync(v *resp.Value) ([]store.TagEdge, error) {
	if v == nil || v.Type != resp.TypeArray {
		return nil, errors.New("TAGSYNC: not an array")
	}
	edges := make([]store.TagEdge, 0, len(v.Array))
	for _, item := range v.Array {
		if item == nil || len(item.Array) != 5 {
			return nil, errors.New("TAGSYNC: bad entry")
		}
		ver, err := strconv.ParseUint(string(item.Array[3].Bulk), 10, 64)
		if err != nil {
			return nil, errors.New("TAGSYNC: bad version")
		}
		edges = append(edges, store.TagEdge{
			Parent: string(item.Array[0].Bulk), Child: string(item.Array[1].Bulk),
			Live: item.Array[2].Int == 1, Version: ver, Node: string(item.Array[4].Bulk),
		})
	}
	return edges, nil
}

func cmdSETTAG(ctx *Context) error {
	if ctx.ArgCount() < 3 {
		return ctx.WriteError(ErrWrongArgCount)
	}

	key := ctx.ArgString(0)
	value := ctx.Arg(1)

	tags := make([]string, 0, ctx.ArgCount()-2)
	for i := 2; i < ctx.ArgCount(); i++ {
		tags = append(tags, ctx.ArgString(i))
	}

	opts := store.SetOptions{Tags: tags}
	err := ctx.Store.Set(key, &store.StringValue{Data: value}, opts)
	if err != nil {
		return ctx.WriteError(err)
	}

	return ctx.WriteOK()
}

func cmdTAGS(ctx *Context) error {
	if ctx.ArgCount() != 1 {
		return ctx.WriteError(ErrWrongArgCount)
	}

	key := ctx.ArgString(0)
	entry, exists := ctx.Store.Get(key)
	if !exists {
		return ctx.WriteArray([]*resp.Value{})
	}

	tags := make([]*resp.Value, 0, len(entry.TagList()))
	for _, tag := range entry.TagList() {
		tags = append(tags, resp.BulkString(tag))
	}

	return ctx.WriteArray(tags)
}

func cmdADDTAG(ctx *Context) error {
	if ctx.ArgCount() < 2 {
		return ctx.WriteError(ErrWrongArgCount)
	}
	added, _ := ctx.Store.AddKeyTags(ctx.ArgString(0), tagArgs(ctx))
	return ctx.WriteInteger(int64(added))
}

func cmdREMTAG(ctx *Context) error {
	if ctx.ArgCount() < 2 {
		return ctx.WriteError(ErrWrongArgCount)
	}
	removed, _ := ctx.Store.RemoveKeyTags(ctx.ArgString(0), tagArgs(ctx))
	return ctx.WriteInteger(int64(removed))
}

// tagArgs is the tags ADDTAG and REMTAG name after the key.
func tagArgs(ctx *Context) []string {
	tags := make([]string, ctx.ArgCount()-1)
	for i := range tags {
		tags[i] = ctx.ArgString(i + 1)
	}
	return tags
}

// tagOptions strips the options a tag command takes after its arguments:
// LOCAL, and for INVALIDATE CASCADE, each at most once, in any order and
// any case. At least min arguments stay, so INVALIDATE LOCAL invalidates
// the tag named LOCAL.
func tagOptions(args [][]byte, min int, allowCascade bool) (rest [][]byte, cascade, local bool) {
	for len(args) > min {
		last := string(args[len(args)-1])
		switch {
		case !local && strings.EqualFold(last, "LOCAL"):
			local = true
		case allowCascade && !cascade && strings.EqualFold(last, "CASCADE"):
			cascade = true
		default:
			return args, cascade, local
		}
		args = args[:len(args)-1]
	}
	return args, cascade, local
}

// InvalidateTags returns the tags INVALIDATE tag [tag ...] [CASCADE]
// [LOCAL] with args reaches: the tags named, and with CASCADE every tag
// below them, each once. CASCADE is matched in any case, like Redis'
// options; a single argument is always a tag. The write gate plans its hold
// from the same list.
func InvalidateTags(ti *store.TagIndex, args [][]byte) []string {
	args, cascade, _ := tagOptions(args, 1, true)
	tags := make([]string, 0, len(args))
	seen := make(map[string]struct{}, len(args))
	add := func(tag string) {
		if _, dup := seen[tag]; !dup {
			seen[tag] = struct{}{}
			tags = append(tags, tag)
		}
	}
	for _, a := range args {
		add(string(a))
	}
	if cascade {
		for _, root := range tags[:len(tags):len(tags)] {
			for _, d := range ti.GetAllDescendants(root) {
				add(d)
			}
		}
	}
	return tags
}

// tagFanout returns the fan-out a tag command uses to reach the other
// masters, or nil when it runs on this node only: outside a cluster, with
// LOCAL, from another node or a master, in a replay, inside MULTI or a
// script (whose commands run on one node, as Redis runs a script's keyless
// commands), or on a connection that cannot answer after releasing its
// locks.
func (ctx *Context) tagFanout(local bool) TagFanout {
	if local || !ctx.CanDefer || ctx.InExec || ctx.InScript || ctx.FromMaster || ctx.Replaying || ctx.LegacyReplay {
		return nil
	}
	if ctx.ClusterConn != nil && ctx.ClusterConn.Internal {
		return nil
	}
	r := ctx.dispatcher()
	if r == nil || r.cluster == nil {
		return nil
	}
	return r.fanout()
}

// withLocal is args with LOCAL appended, for the other masters.
func withLocal(args [][]byte) [][]byte {
	out := make([][]byte, len(args), len(args)+1)
	copy(out, args)
	return append(out, []byte("LOCAL"))
}

// fanoutError is the reply of a cluster-wide tag command some master did
// not answer: what was done, and which masters were not reached. The
// command is idempotent, so the client can send it again.
func fanoutError(done string, replies []FanoutReply) string {
	var sb strings.Builder
	sb.WriteString("TAGFANOUT ")
	sb.WriteString(done)
	sb.WriteString("; not reached:")
	for _, r := range replies {
		if r.Err != nil {
			sb.WriteByte(' ')
			sb.WriteString(r.Node)
			sb.WriteString(" (")
			sb.WriteString(r.Err.Error())
			sb.WriteByte(')')
		}
	}
	return sb.String()
}

// fanoutReplyError returns a master's reply as an error when it is one.
func fanoutReplyError(r FanoutReply) error {
	if r.Err == nil && r.Reply != nil && r.Reply.Type == resp.TypeError {
		return errors.New(r.Reply.Err)
	}
	return r.Err
}

func cmdINVALIDATE(ctx *Context) error {
	if ctx.ArgCount() < 1 {
		return ctx.WriteError(ErrWrongArgCount)
	}

	start := time.Now()
	ti := ctx.Store.GetTagIndex()
	tags := InvalidateTags(ti, ctx.Args)
	// With KeyHeld the command holds only the stripes of the keys its tags
	// had when it was planned. A tag that has gained a key since makes it
	// take the whole gate before it changes anything: taking it lets other
	// writes (a SWAPDB, a TAGLINK, writes to the keys) run, so the command
	// then starts over from what they left.
	var keysToDelete []string
	striped := ctx.KeyHeld != nil && ctx.Escalate != nil
	if striped {
		var held bool
		if keysToDelete, held = ti.InvalidateAllIf(tags, ctx.KeyHeld); !held {
			ctx.Escalate()
			ctx.followSwap()
			ti = ctx.Store.GetTagIndex()
			tags = InvalidateTags(ti, ctx.Args)
			striped = false
		}
	}
	if !striped {
		for _, t := range tags {
			keysToDelete = append(keysToDelete, ti.Invalidate(t)...)
		}
	}

	if len(tags) > 1 {
		// A key with several of the tags is listed once per tag.
		uniqueKeys := make([]string, 0, len(keysToDelete))
		seen := make(map[string]struct{}, len(keysToDelete))
		for _, key := range keysToDelete {
			if _, exists := seen[key]; !exists {
				seen[key] = struct{}{}
				uniqueKeys = append(uniqueKeys, key)
			}
		}
		keysToDelete = uniqueKeys
	}
	deleted := ctx.Store.DeleteInvalidated(keysToDelete, tags)
	// Replicas, the AOF and a slot migration get the keys deleted, not the
	// tags: writes to other keys may have run beside a striped run, and a
	// slot migration copies only the writes naming keys of its slots.
	propagateDeletes(ctx, deleted)
	announceDeletes(ctx, deleted)
	elapsed := time.Since(start)
	ti.RecordInvalidation(len(deleted), elapsed.Microseconds())
	RecordLatency("invalidate", elapsed)
	_, _, local := tagOptions(ctx.Args, 1, true)
	if f := ctx.tagFanout(local); f != nil {
		args := withLocal(ctx.Args)
		n := int64(len(deleted))
		ctx.AfterGate = func() error {
			replies := f.Send("INVALIDATE", args)
			total, failed := n, false
			for i := range replies {
				if err := fanoutReplyError(replies[i]); err != nil {
					replies[i].Err, failed = err, true
					continue
				}
				total += replies[i].Reply.Int
			}
			if failed {
				return ctx.WriteError(errors.New(fanoutError(strconv.FormatInt(total, 10)+" keys deleted", replies)))
			}
			return ctx.WriteInteger(total)
		}
		return nil
	}
	return ctx.WriteInteger(int64(len(deleted)))
}

// announceDeletes tells what follows the keyspace that keys were deleted by
// a command that names none of them: each key publishes the del keyspace
// notification DEL publishes, and search indexes drop the documents, which
// they otherwise learn of only through the keys a command names.
func announceDeletes(ctx *Context, keys []string) {
	for _, key := range keys {
		ctx.notify(store.NotifyGeneric, "del", key)
	}
	indexes := searchIndexesFor(ctx).All()
	for _, idx := range indexes {
		for _, key := range keys {
			idx.Touch(ctx.Store, key)
		}
	}
}

func cmdTAGKEYS(ctx *Context) error {
	args, _, local := tagOptions(ctx.Args, 1, false)
	if len(args) != 1 {
		return ctx.WriteError(ErrWrongArgCount)
	}
	var keys []string
	if !ctx.servesOtherMaster(local) {
		keys = ctx.Store.GetTagIndex().GetKeys(string(args[0]))
	}
	reply := func(keys []string) error {
		results := make([]*resp.Value, 0, len(keys))
		for _, key := range keys {
			results = append(results, resp.BulkString(key))
		}
		return ctx.WriteArray(results)
	}
	if f := ctx.tagFanout(local); f != nil {
		fargs := withLocal(args)
		ctx.AfterGate = func() error {
			replies := f.Send("TAGKEYS", fargs)
			failed := false
			for i := range replies {
				if err := fanoutReplyError(replies[i]); err != nil {
					replies[i].Err, failed = err, true
					continue
				}
				for _, k := range replies[i].Reply.Array {
					keys = append(keys, string(k.Bulk))
				}
			}
			if failed {
				return ctx.WriteError(errors.New(fanoutError(strconv.Itoa(len(keys))+" keys found", replies)))
			}
			return reply(keys)
		}
		return nil
	}
	return reply(keys)
}

func cmdTAGCOUNT(ctx *Context) error {
	args, _, local := tagOptions(ctx.Args, 1, false)
	if len(args) != 1 {
		return ctx.WriteError(ErrWrongArgCount)
	}
	var count int64
	if !ctx.servesOtherMaster(local) {
		count = int64(ctx.Store.GetTagIndex().Count(string(args[0])))
	}
	if f := ctx.tagFanout(local); f != nil {
		fargs := withLocal(args)
		ctx.AfterGate = func() error {
			replies := f.Send("TAGCOUNT", fargs)
			failed := false
			for i := range replies {
				if err := fanoutReplyError(replies[i]); err != nil {
					replies[i].Err, failed = err, true
					continue
				}
				count += replies[i].Reply.Int
			}
			if failed {
				return ctx.WriteError(errors.New(fanoutError(strconv.FormatInt(count, 10)+" keys counted", replies)))
			}
			return ctx.WriteInteger(count)
		}
		return nil
	}
	return ctx.WriteInteger(count)
}

// servesOtherMaster reports whether a cluster-wide read on this node should
// leave its own keys out: a replica's keys are its master's, which the
// fan-out asks.
func (ctx *Context) servesOtherMaster(local bool) bool {
	if local || ctx.tagFanout(false) == nil {
		return false
	}
	c := ctx.clusterState()
	return c != nil && c.IsReplica()
}

func cmdTAGLINK(ctx *Context) error   { return tagLinkCommand(ctx, true) }
func cmdTAGUNLINK(ctx *Context) error { return tagLinkCommand(ctx, false) }

// TagEdgeCommand is the command that recreates e: a plain TAGLINK outside a
// cluster, and in a cluster TAGLINK or TAGUNLINK with the change's version,
// which a node applies only when it has no newer change of the link.
func TagEdgeCommand(e store.TagEdge) []string {
	if e.Node == "" {
		return []string{"TAGLINK", e.Parent, e.Child}
	}
	name := "TAGLINK"
	if !e.Live {
		name = "TAGUNLINK"
	}
	return []string{name, e.Parent, e.Child, "VERSION", strconv.FormatUint(e.Version, 10), e.Node}
}

// linkArgs parses TAGLINK and TAGUNLINK: parent child [LOCAL], or the
// versioned form parent child VERSION version node, which only other nodes,
// a master and a replay send.
func linkArgs(ctx *Context, link bool) (parent, child string, edge *store.TagEdge, local bool, err error) {
	args := ctx.Args
	parent, child = string(args[0]), string(args[1])
	switch {
	case len(args) == 2:
	case len(args) == 3 && strings.EqualFold(string(args[2]), "LOCAL"):
		local = true
	case len(args) == 5 && strings.EqualFold(string(args[2]), "VERSION"):
		internal := ctx.FromMaster || ctx.Replaying || ctx.LegacyReplay || (ctx.ClusterConn != nil && ctx.ClusterConn.Internal)
		ver, perr := strconv.ParseUint(string(args[3]), 10, 64)
		if !internal || perr != nil || len(args[4]) == 0 {
			return "", "", nil, false, ErrSyntaxError
		}
		edge = &store.TagEdge{Parent: parent, Child: child, Live: link, Version: ver, Node: string(args[4])}
	default:
		return "", "", nil, false, ErrSyntaxError
	}
	return parent, child, edge, local, nil
}

// linkFanout is the fan-out a cluster-wide TAGLINK or TAGUNLINK uses. Unlike
// the other tag commands it also serves a change another node forwarded:
// the node coordinating the hierarchy hands it to every master.
func (ctx *Context) linkFanout(local bool) TagFanout {
	if local || !ctx.CanDefer || ctx.InExec || ctx.InScript || ctx.FromMaster || ctx.Replaying || ctx.LegacyReplay {
		return nil
	}
	r := ctx.dispatcher()
	if r == nil || r.cluster == nil {
		return nil
	}
	return r.fanout()
}

// tagCoordinator is the node that orders the hierarchy's changes in a
// cluster: the master of slot 0, or this node while slot 0 has none. One
// node checking every link for cycles and numbering the changes keeps two
// links made at once on two nodes from closing a cycle.
func (ctx *Context) tagCoordinator() string {
	c := ctx.clusterState()
	if st := c.Slot(0); st.Assigned {
		return st.Owner.ID
	}
	return c.MyID()
}

func tagLinkCommand(ctx *Context, link bool) error {
	name := "TAGUNLINK"
	if link {
		name = "TAGLINK"
	}
	parent, child, edge, local, err := linkArgs(ctx, link)
	if err != nil {
		return ctx.WriteError(err)
	}
	ti := ctx.Store.GetTagIndex()
	if edge != nil {
		ti.ApplyEdge(*edge)
		return ctx.WriteOK()
	}
	node := ""
	if c := ctx.clusterState(); c != nil {
		node = c.MyID()
	}
	f := ctx.linkFanout(local)
	if f != nil {
		// A change forwarded by another node is not forwarded again, even
		// when the two disagree for a moment on who coordinates.
		forwarded := ctx.ClusterConn != nil && ctx.ClusterConn.Internal
		if coord := ctx.tagCoordinator(); coord != node && !forwarded {
			ctx.PropagateAs()
			args := [][]byte{[]byte(parent), []byte(child)}
			ctx.AfterGate = func() error {
				r := f.Ask(coord, name, args)
				if err := fanoutReplyError(r); err != nil {
					if r.Err != nil {
						err = errors.New("TAGFANOUT the coordinating node " + coord + " was not reached (" + err.Error() + ")")
					}
					return ctx.WriteError(err)
				}
				return ctx.WriteOK()
			}
			return nil
		}
	}
	var e store.TagEdge
	if link {
		if e, err = ti.LinkAt(parent, child, node); err != nil {
			if ctx.Replaying || ctx.LegacyReplay || ctx.FromMaster {
				// An AOF or a master of an older version accepted a link that
				// closes a cycle; it is left out rather than stopping the
				// replay.
				ctx.PropagateAs()
				return ctx.WriteOK()
			}
			return ctx.WriteError(err)
		}
	} else {
		e = ti.UnlinkAt(parent, child, node)
	}
	cmd := TagEdgeCommand(e)
	if node != "" {
		ctx.PropagateAs(ExecutedCommand{Name: cmd[0], Args: stringArgsToBytes(cmd[1:]), DB: ctx.DB})
	}
	if f != nil {
		args := stringArgsToBytes(cmd[1:])
		ctx.AfterGate = func() error {
			replies := f.Send(cmd[0], args)
			reached := 0
			failed := false
			for i := range replies {
				if err := fanoutReplyError(replies[i]); err != nil {
					replies[i].Err, failed = err, true
					continue
				}
				reached++
			}
			if failed {
				return ctx.WriteError(errors.New(fanoutError("the change reached this node and "+strconv.Itoa(reached)+" other masters; the others get it when they next exchange the hierarchy", replies)))
			}
			return ctx.WriteOK()
		}
		return nil
	}
	return ctx.WriteOK()
}

func cmdTAGCHILDREN(ctx *Context) error {
	if ctx.ArgCount() != 1 {
		return ctx.WriteError(ErrWrongArgCount)
	}

	tag := ctx.ArgString(0)
	children := ctx.Store.GetTagIndex().GetChildren(tag)

	results := make([]*resp.Value, 0, len(children))
	for _, child := range children {
		results = append(results, resp.BulkString(child))
	}

	return ctx.WriteArray(results)
}

// deleteChunk bounds the keys of one propagated DEL.
const deleteChunk = 1024

// propagateDeletes records the deletion of keys, in place of the command,
// as DELs: of at most deleteChunk keys, and in a cluster one per slot, as a
// slot migration copies only single-slot writes to the slots it moves.
func propagateDeletes(ctx *Context, keys []string) {
	if len(keys) == 0 {
		ctx.PropagateAs()
		return
	}
	groups := [][]string{keys}
	if ctx.ClusterEnabled() {
		bySlot := map[int][]string{}
		var order []int
		for _, k := range keys {
			slot := hashslot.Key(k)
			if _, seen := bySlot[slot]; !seen {
				order = append(order, slot)
			}
			bySlot[slot] = append(bySlot[slot], k)
		}
		groups = groups[:0]
		for _, slot := range order {
			groups = append(groups, bySlot[slot])
		}
	}
	var cmds []ExecutedCommand
	for _, group := range groups {
		for len(group) > 0 {
			n := min(len(group), deleteChunk)
			// The arguments are the key strings' own bytes: the deleted
			// keys are not used again, and what records the DEL (the AOF,
			// replicas, tracking) only reads them.
			args := make([][]byte, n)
			for i, k := range group[:n] {
				args[i] = unsafe.Slice(unsafe.StringData(k), len(k))
			}
			cmds = append(cmds, ExecutedCommand{Name: "DEL", Args: args, DB: ctx.DB})
			group = group[n:]
		}
	}
	ctx.PropagateAs(cmds...)
}

// tagsInfo is CacheStorm's Tags section of INFO, summed over the databases:
// how many tags and (tag, key) pairs the indexes hold, how many links the
// hierarchies have, and what INVALIDATE has done since the last CONFIG
// RESETSTAT.
func tagsInfo(ctx *Context) string {
	var t store.TagStats
	links := 0
	ctx.databases().ForEach(func(_ int, s *store.Store) {
		ti := s.GetTagIndex()
		st := ti.Stats()
		t.Tags += st.Tags
		t.Members += st.Members
		t.Invalidations += st.Invalidations
		t.InvalidatedKeys += st.InvalidatedKeys
		t.MaxInvalidateUsec = max(t.MaxInvalidateUsec, st.MaxInvalidateUsec)
		links += len(ti.Links())
	})
	var sb strings.Builder
	sb.WriteString("# Tags\r\n")
	for _, f := range []struct {
		name string
		v    int64
	}{
		{"tags", t.Tags},
		{"tag_memberships", t.Members},
		{"tag_links", int64(links)},
		{"invalidations", t.Invalidations},
		{"invalidated_keys", t.InvalidatedKeys},
		{"invalidate_max_usec", t.MaxInvalidateUsec},
	} {
		sb.WriteString(f.name)
		sb.WriteByte(':')
		sb.WriteString(strconv.FormatInt(f.v, 10))
		sb.WriteString("\r\n")
	}
	sb.WriteString("\r\n")
	return sb.String()
}
