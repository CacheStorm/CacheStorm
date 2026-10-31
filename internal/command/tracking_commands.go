package command

import (
	"errors"
	"strconv"
	"strings"

	"github.com/cachestorm/cachestorm/internal/resp"
	"github.com/cachestorm/cachestorm/internal/store"
)

// TrackingOptions are the options of CLIENT TRACKING ON.
type TrackingOptions struct {
	Redirect int64
	BCast    bool
	Prefixes []string
	OptIn    bool
	OptOut   bool
	NoLoop   bool
}

// TrackingHooks connect the CLIENT tracking subcommands to the server's
// invalidation table.
type TrackingHooks struct {
	// Enable turns tracking on for the client, whose out-of-band messages go
	// through sub.
	Enable  func(clientID int64, sub *store.Subscriber, opts TrackingOptions) error
	Disable func(clientID int64)
	// Caching records CLIENT CACHING yes|no for the client's next command.
	Caching  func(clientID int64, yes bool) error
	Info     func(clientID int64) *resp.Value
	Redirect func(clientID int64) int64
}

// SetTrackingHooks installs the server's client-tracking implementation.
func (r *Router) SetTrackingHooks(h *TrackingHooks) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.trackingHooks = h
}

func trackingHooksFor(ctx *Context) *TrackingHooks {
	r := ctx.dispatcher()
	if r == nil {
		return nil
	}
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.trackingHooks
}

var errTrackingUnsupported = errors.New("ERR client tracking is not supported by this server")

// cmdClientTracking implements CLIENT TRACKING ON|OFF [REDIRECT id]
// [PREFIX prefix ...] [BCAST] [OPTIN] [OPTOUT] [NOLOOP].
func cmdClientTracking(ctx *Context) error {
	if ctx.ArgCount() < 2 {
		return ctx.WriteError(arityError("client|tracking"))
	}
	var on bool
	switch strings.ToUpper(ctx.ArgString(1)) {
	case "ON":
		on = true
	case "OFF":
	default:
		return ctx.WriteError(ErrSyntaxError)
	}
	var opts TrackingOptions
	for i := 2; i < ctx.ArgCount(); i++ {
		switch strings.ToUpper(ctx.ArgString(i)) {
		case "REDIRECT":
			if i+1 >= ctx.ArgCount() {
				return ctx.WriteError(ErrSyntaxError)
			}
			if opts.Redirect != 0 {
				return ctx.WriteError(errors.New("ERR A client can only redirect to a single other client"))
			}
			id, err := strconv.ParseInt(ctx.ArgString(i+1), 10, 64)
			if err != nil {
				return ctx.WriteError(ErrNotInteger)
			}
			opts.Redirect = id
			i++
		case "BCAST":
			opts.BCast = true
		case "PREFIX":
			if i+1 >= ctx.ArgCount() {
				return ctx.WriteError(ErrSyntaxError)
			}
			opts.Prefixes = append(opts.Prefixes, ctx.ArgString(i+1))
			i++
		case "OPTIN":
			opts.OptIn = true
		case "OPTOUT":
			opts.OptOut = true
		case "NOLOOP":
			opts.NoLoop = true
		default:
			return ctx.WriteError(ErrSyntaxError)
		}
	}
	h := trackingHooksFor(ctx)
	if !on {
		if h != nil {
			h.Disable(ctx.ClientID)
		}
		return ctx.WriteOK()
	}
	if h == nil {
		return ctx.WriteError(errTrackingUnsupported)
	}
	if err := h.Enable(ctx.ClientID, ctx.GetSubscriber(), opts); err != nil {
		return ctx.WriteError(err)
	}
	return ctx.WriteOK()
}

// cmdClientCaching implements CLIENT CACHING YES|NO.
func cmdClientCaching(ctx *Context) error {
	if ctx.ArgCount() != 2 {
		return ctx.WriteError(arityError("client|caching"))
	}
	var yes bool
	switch strings.ToUpper(ctx.ArgString(1)) {
	case "YES":
		yes = true
	case "NO":
	default:
		return ctx.WriteError(ErrSyntaxError)
	}
	h := trackingHooksFor(ctx)
	if h == nil {
		return ctx.WriteError(errors.New("ERR CLIENT CACHING can be called only when the client is in tracking mode with OPTIN or OPTOUT mode enabled"))
	}
	if err := h.Caching(ctx.ClientID, yes); err != nil {
		return ctx.WriteError(err)
	}
	return ctx.WriteOK()
}

// cmdClientTrackingInfo implements CLIENT TRACKINGINFO.
func cmdClientTrackingInfo(ctx *Context) error {
	if h := trackingHooksFor(ctx); h != nil {
		return ctx.Writer.WriteValue(h.Info(ctx.ClientID))
	}
	return ctx.Writer.WriteValue(resp.PairsValue([]*resp.Value{
		resp.BulkString("flags"), resp.ArrayValue([]*resp.Value{resp.BulkString("off")}),
		resp.BulkString("redirect"), resp.IntegerValue(-1),
		resp.BulkString("prefixes"), resp.ArrayValue([]*resp.Value{}),
	}))
}
