package command

import (
	"errors"
	"sort"
	"strings"

	"github.com/cachestorm/cachestorm/internal/resp"
	"github.com/cachestorm/cachestorm/internal/store"
)

var ErrPubSubNotAvailable = errors.New("ERR pub/sub not available")

func RegisterPubSubCommands(router *Router) {
	router.Register(&CommandDef{Name: "SUBSCRIBE", Handler: cmdSUBSCRIBE})
	router.Register(&CommandDef{Name: "UNSUBSCRIBE", Handler: cmdUNSUBSCRIBE})
	router.Register(&CommandDef{Name: "PSUBSCRIBE", Handler: cmdPSUBSCRIBE})
	router.Register(&CommandDef{Name: "PUNSUBSCRIBE", Handler: cmdPUNSUBSCRIBE})
	router.Register(&CommandDef{Name: "PUBLISH", Handler: cmdPUBLISH})
	router.Register(&CommandDef{Name: "PUBSUB", Handler: cmdPUBSUB})
	router.Register(&CommandDef{Name: "SSUBSCRIBE", Handler: cmdSSUBSCRIBE})
	router.Register(&CommandDef{Name: "SUNSUBSCRIBE", Handler: cmdSUNSUBSCRIBE})
	router.Register(&CommandDef{Name: "SPUBLISH", Handler: cmdSPUBLISH})
}

func argStrings(ctx *Context) []string {
	out := make([]string, ctx.ArgCount())
	for i := range out {
		out[i] = ctx.ArgString(i)
	}
	return out
}

// writeSubAck writes one (un)subscribe confirmation: [kind, name, count],
// with a nil name for an unsubscribe-all that found nothing to leave.
func writeSubAck(ctx *Context, kind string, name *string, count int) error {
	nameVal := resp.NullBulkString()
	if name != nil {
		nameVal = resp.BulkString(*name)
	}
	return ctx.Writer.WriteValue(resp.ArrayValue([]*resp.Value{
		resp.BulkString(kind),
		nameVal,
		resp.IntegerValue(int64(count)),
	}))
}

func writeSubAcks(ctx *Context, kind string, acks []store.SubAck) error {
	for i := range acks {
		if err := writeSubAck(ctx, kind, &acks[i].Name, acks[i].Count); err != nil {
			return err
		}
	}
	return nil
}

type subscribeFn func(ps *store.PubSub, sub *store.Subscriber, names ...string) []store.SubAck

func doSubscribe(ctx *Context, kind string, fn subscribeFn) error {
	if ctx.ArgCount() < 1 {
		return ctx.WriteError(ErrWrongArgCount)
	}
	ps := ctx.Store.GetPubSub()
	if ps == nil {
		return ctx.WriteError(ErrPubSubNotAvailable)
	}
	return writeSubAcks(ctx, kind, fn(ps, ctx.GetSubscriber(), argStrings(ctx)...))
}

// doUnsubscribe answers (P|S)UNSUBSCRIBE. Without arguments it confirms each
// name actually left; when there were none Redis sends a single ack with a
// nil name and the client's remaining count.
func doUnsubscribe(ctx *Context, kind string, fn subscribeFn, remaining func(*store.Subscriber) int) error {
	ps := ctx.Store.GetPubSub()
	if ps == nil {
		return ctx.WriteError(ErrPubSubNotAvailable)
	}
	names := argStrings(ctx)
	sub := ctx.Subscriber
	if sub == nil {
		if len(names) == 0 {
			return writeSubAck(ctx, kind, nil, 0)
		}
		for i := range names {
			if err := writeSubAck(ctx, kind, &names[i], 0); err != nil {
				return err
			}
		}
		return nil
	}
	acks := fn(ps, sub, names...)
	if len(names) == 0 && len(acks) == 0 {
		return writeSubAck(ctx, kind, nil, remaining(sub))
	}
	return writeSubAcks(ctx, kind, acks)
}

func cmdSUBSCRIBE(ctx *Context) error {
	return doSubscribe(ctx, "subscribe", (*store.PubSub).SubscribeChannels)
}

func cmdUNSUBSCRIBE(ctx *Context) error {
	return doUnsubscribe(ctx, "unsubscribe", (*store.PubSub).UnsubscribeChannels, (*store.Subscriber).SubscriptionCount)
}

func cmdPSUBSCRIBE(ctx *Context) error {
	return doSubscribe(ctx, "psubscribe", (*store.PubSub).SubscribePatterns)
}

func cmdPUNSUBSCRIBE(ctx *Context) error {
	return doUnsubscribe(ctx, "punsubscribe", (*store.PubSub).UnsubscribePatterns, (*store.Subscriber).SubscriptionCount)
}

func cmdSSUBSCRIBE(ctx *Context) error {
	return doSubscribe(ctx, "ssubscribe", (*store.PubSub).SSubscribe)
}

func cmdSUNSUBSCRIBE(ctx *Context) error {
	return doUnsubscribe(ctx, "sunsubscribe", (*store.PubSub).SUnsubscribe, (*store.Subscriber).ShardSubscriptionCount)
}

func cmdPUBLISH(ctx *Context) error {
	if ctx.ArgCount() != 2 {
		return ctx.WriteError(ErrWrongArgCount)
	}
	ps := ctx.Store.GetPubSub()
	if ps == nil {
		return ctx.WriteInteger(0)
	}
	return ctx.WriteInteger(int64(ps.Publish(ctx.ArgString(0), ctx.Arg(1))))
}

func cmdSPUBLISH(ctx *Context) error {
	if ctx.ArgCount() != 2 {
		return ctx.WriteError(ErrWrongArgCount)
	}
	ps := ctx.Store.GetPubSub()
	if ps == nil {
		return ctx.WriteInteger(0)
	}
	return ctx.WriteInteger(int64(ps.SPublish(ctx.ArgString(0), ctx.Arg(1))))
}

func writeNameList(ctx *Context, names []string) error {
	sort.Strings(names)
	results := make([]*resp.Value, 0, len(names))
	for _, n := range names {
		results = append(results, resp.BulkString(n))
	}
	return ctx.WriteArray(results)
}

func writeNumSub(ctx *Context, channels []string, counts map[string]int) error {
	results := make([]*resp.Value, 0, len(channels)*2)
	for _, ch := range channels {
		results = append(results, resp.BulkString(ch), resp.IntegerValue(int64(counts[ch])))
	}
	return ctx.WriteArray(results)
}

func cmdPUBSUB(ctx *Context) error {
	if ctx.ArgCount() < 1 {
		return ctx.WriteError(ErrWrongArgCount)
	}

	subCmd := strings.ToUpper(ctx.ArgString(0))

	ps := ctx.Store.GetPubSub()
	if ps == nil {
		return ctx.WriteError(ErrPubSubNotAvailable)
	}

	wrongArgs := errors.New("ERR wrong number of arguments for 'pubsub|" + strings.ToLower(subCmd) + "' command")
	rest := argStrings(ctx)[1:]

	switch subCmd {
	case "CHANNELS", "SHARDCHANNELS":
		if len(rest) > 1 {
			return ctx.WriteError(wrongArgs)
		}
		pattern := ""
		if len(rest) == 1 {
			pattern = rest[0]
		}
		if subCmd == "CHANNELS" {
			return writeNameList(ctx, ps.Channels(pattern))
		}
		return writeNameList(ctx, ps.ShardChannels(pattern))

	case "NUMSUB":
		return writeNumSub(ctx, rest, ps.NumSub(rest...))

	case "SHARDNUMSUB":
		return writeNumSub(ctx, rest, ps.ShardNumSub(rest...))

	case "NUMPAT":
		if len(rest) != 0 {
			return ctx.WriteError(wrongArgs)
		}
		return ctx.WriteInteger(int64(ps.NumPat()))

	case "HELP":
		lines := []string{
			"PUBSUB <subcommand> [<arg> [value] [opt] ...]. Subcommands are:",
			"CHANNELS [<pattern>]",
			"    Return the currently active channels matching a <pattern> (default: '*').",
			"NUMPAT",
			"    Return number of subscriptions to patterns.",
			"NUMSUB [<channel> ...]",
			"    Return the number of subscribers for the specified channels, excluding",
			"    pattern subscriptions(default: no channels).",
			"SHARDCHANNELS [<pattern>]",
			"    Return the currently active shard level channels matching a <pattern> (default: '*').",
			"SHARDNUMSUB [<shardchannel> ...]",
			"    Return the number of subscribers for the specified shard level channel(s)",
			"HELP",
			"    Print this help.",
		}
		results := make([]*resp.Value, 0, len(lines))
		for _, l := range lines {
			results = append(results, resp.SimpleString(l))
		}
		return ctx.WriteArray(results)

	default:
		return ctx.WriteError(errors.New("ERR unknown subcommand '" + ctx.ArgString(0) + "'. Try PUBSUB HELP."))
	}
}
