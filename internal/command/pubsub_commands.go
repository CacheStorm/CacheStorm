package command

import (
	"errors"
	"strconv"
	"strings"

	"github.com/cachestorm/cachestorm/internal/resp"
)

var ErrPubSubNotAvailable = errors.New("ERR pub/sub not available")

func RegisterPubSubCommands(router *Router) {
	router.Register(&CommandDef{Name: "SUBSCRIBE", Handler: cmdSUBSCRIBE})
	router.Register(&CommandDef{Name: "UNSUBSCRIBE", Handler: cmdUNSUBSCRIBE})
	router.Register(&CommandDef{Name: "PSUBSCRIBE", Handler: cmdPSUBSCRIBE})
	router.Register(&CommandDef{Name: "PUNSUBSCRIBE", Handler: cmdPUNSUBSCRIBE})
	router.Register(&CommandDef{Name: "PUBLISH", Handler: cmdPUBLISH})
	router.Register(&CommandDef{Name: "PUBSUB", Handler: cmdPUBSUB})
}

func cmdSUBSCRIBE(ctx *Context) error {
	if ctx.ArgCount() < 1 {
		return ctx.WriteError(ErrWrongArgCount)
	}

	channels := make([]string, ctx.ArgCount())
	for i := 0; i < ctx.ArgCount(); i++ {
		channels[i] = ctx.ArgString(i)
	}

	ps := ctx.Store.GetPubSub()
	if ps == nil {
		return ctx.WriteError(ErrPubSubNotAvailable)
	}

	sub := ctx.GetSubscriber()
	count := ps.Subscribe(sub, channels...)

	for _, ch := range channels {
		ctx.Writer.WriteValue(resp.ArrayValue([]*resp.Value{
			resp.SimpleString("subscribe"),
			resp.BulkString(ch),
			resp.IntegerValue(int64(count)),
		}))
	}

	return nil
}

func cmdUNSUBSCRIBE(ctx *Context) error {
	ps := ctx.Store.GetPubSub()
	if ps == nil {
		return ctx.WriteError(ErrPubSubNotAvailable)
	}

	sub := ctx.GetSubscriber()

	channels := make([]string, ctx.ArgCount())
	for i := 0; i < ctx.ArgCount(); i++ {
		channels[i] = ctx.ArgString(i)
	}

	count := ps.Unsubscribe(sub, channels...)

	if len(channels) == 0 {
		ctx.Writer.WriteValue(resp.ArrayValue([]*resp.Value{
			resp.SimpleString("unsubscribe"),
			resp.NullBulkString(),
			resp.IntegerValue(0),
		}))
	} else {
		for _, ch := range channels {
			ctx.Writer.WriteValue(resp.ArrayValue([]*resp.Value{
				resp.SimpleString("unsubscribe"),
				resp.BulkString(ch),
				resp.IntegerValue(int64(count)),
			}))
		}
	}

	return nil
}

func cmdPSUBSCRIBE(ctx *Context) error {
	if ctx.ArgCount() < 1 {
		return ctx.WriteError(ErrWrongArgCount)
	}

	patterns := make([]string, ctx.ArgCount())
	for i := 0; i < ctx.ArgCount(); i++ {
		patterns[i] = ctx.ArgString(i)
	}

	ps := ctx.Store.GetPubSub()
	if ps == nil {
		return ctx.WriteError(ErrPubSubNotAvailable)
	}

	sub := ctx.GetSubscriber()
	count := ps.PSubscribe(sub, patterns...)

	for _, p := range patterns {
		ctx.Writer.WriteValue(resp.ArrayValue([]*resp.Value{
			resp.SimpleString("psubscribe"),
			resp.BulkString(p),
			resp.IntegerValue(int64(count)),
		}))
	}

	return nil
}

func cmdPUNSUBSCRIBE(ctx *Context) error {
	ps := ctx.Store.GetPubSub()
	if ps == nil {
		return ctx.WriteError(ErrPubSubNotAvailable)
	}

	sub := ctx.GetSubscriber()

	patterns := make([]string, ctx.ArgCount())
	for i := 0; i < ctx.ArgCount(); i++ {
		patterns[i] = ctx.ArgString(i)
	}

	count := ps.PUnsubscribe(sub, patterns...)

	if len(patterns) == 0 {
		ctx.Writer.WriteValue(resp.ArrayValue([]*resp.Value{
			resp.SimpleString("punsubscribe"),
			resp.NullBulkString(),
			resp.IntegerValue(0),
		}))
	} else {
		for _, p := range patterns {
			ctx.Writer.WriteValue(resp.ArrayValue([]*resp.Value{
				resp.SimpleString("punsubscribe"),
				resp.BulkString(p),
				resp.IntegerValue(int64(count)),
			}))
		}
	}

	return nil
}

func cmdPUBLISH(ctx *Context) error {
	if ctx.ArgCount() != 2 {
		return ctx.WriteError(ErrWrongArgCount)
	}

	channel := ctx.ArgString(0)
	message := ctx.Arg(1)

	ps := ctx.Store.GetPubSub()
	if ps == nil {
		return ctx.WriteInteger(0)
	}

	count := ps.Publish(channel, message)
	return ctx.WriteInteger(int64(count))
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

	switch subCmd {
	case "CHANNELS":
		pattern := ""
		if ctx.ArgCount() >= 2 {
			pattern = ctx.ArgString(1)
		}
		channels := ps.Channels(pattern)
		results := make([]*resp.Value, 0, len(channels))
		for _, ch := range channels {
			results = append(results, resp.BulkString(ch))
		}
		return ctx.WriteArray(results)

	case "NUMSUB":
		channels := make([]string, ctx.ArgCount()-1)
		for i := 1; i < ctx.ArgCount(); i++ {
			channels[i-1] = ctx.ArgString(i)
		}
		if len(channels) == 0 {
			return ctx.WriteArray([]*resp.Value{})
		}
		numsub := ps.NumSub(channels...)
		results := make([]*resp.Value, 0, len(numsub)*2)
		for _, ch := range channels {
			results = append(results, resp.BulkString(ch))
			results = append(results, resp.IntegerValue(int64(numsub[ch])))
		}
		return ctx.WriteArray(results)

	case "NUMPAT":
		return ctx.WriteInteger(int64(ps.NumPat()))

	default:
		return ctx.WriteError(ErrUnknownCommand)
	}
}

func init() {
	_ = strconv.Itoa(0)
}
