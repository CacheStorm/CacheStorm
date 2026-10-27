package command

import (
	"errors"
	"strconv"
	"strings"
	"time"
)

// ClientFilter selects clients for CLIENT LIST and CLIENT KILL.
type ClientFilter struct {
	IDs    []int64
	Type   string
	Addr   string
	LAddr  string
	User   string
	MaxAge int64
	// SkipMe leaves the calling client out (CLIENT KILL's default).
	SkipMe bool
}

// ClientHooks connect the CLIENT administration subcommands to the server's
// connections.
type ClientHooks struct {
	// List renders the CLIENT LIST lines of the matching clients.
	List func(caller int64, f ClientFilter) string
	// Kill closes the matching clients and reports how many it closed.
	Kill func(caller int64, f ClientFilter) int
	// Pause holds client commands (writes only, or all) until the deadline.
	Pause   func(until time.Time, writesOnly bool)
	Unpause func()
	// Unblock ends a blocked command of client id, as a timeout or with the
	// UNBLOCKED error, and reports whether the client was blocked.
	Unblock func(id int64, withError bool) bool
}

// SetClientHooks installs the server's CLIENT administration hooks.
func (r *Router) SetClientHooks(h *ClientHooks) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.clientHooks = h
}

func clientHooksFor(ctx *Context) *ClientHooks {
	r := ctx.dispatcher()
	if r == nil {
		return nil
	}
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.clientHooks
}

var errNoSuchClient = errors.New("ERR No such client")

func validClientType(t string) bool {
	switch t {
	case "normal", "master", "replica", "slave", "pubsub":
		return true
	}
	return false
}

// parseClientFilter reads the filter options of CLIENT LIST (from LIST
// onwards) or CLIENT KILL.
func parseClientFilter(ctx *Context, start int, kill bool) (ClientFilter, error) {
	f := ClientFilter{SkipMe: kill}
	for i := start; i < ctx.ArgCount(); i++ {
		opt := strings.ToUpper(ctx.ArgString(i))
		if opt == "ID" && !kill {
			for i++; i < ctx.ArgCount(); i++ {
				id, err := strconv.ParseInt(ctx.ArgString(i), 10, 64)
				if err != nil || id <= 0 {
					return f, errors.New("ERR Invalid client ID")
				}
				f.IDs = append(f.IDs, id)
			}
			if len(f.IDs) == 0 {
				return f, ErrSyntaxError
			}
			break
		}
		if i+1 >= ctx.ArgCount() {
			return f, ErrSyntaxError
		}
		val := ctx.ArgString(i + 1)
		i++
		switch opt {
		case "ID":
			id, err := strconv.ParseInt(val, 10, 64)
			if err != nil || id <= 0 {
				return f, errors.New("ERR client-id should be greater than 0")
			}
			f.IDs = append(f.IDs, id)
		case "TYPE":
			t := strings.ToLower(val)
			if !validClientType(t) {
				return f, errors.New("ERR Unknown client type '" + val + "'")
			}
			if t == "slave" {
				t = "replica"
			}
			f.Type = t
		case "ADDR":
			if !kill {
				return f, ErrSyntaxError
			}
			f.Addr = val
		case "LADDR":
			if !kill {
				return f, ErrSyntaxError
			}
			f.LAddr = val
		case "USER":
			if !kill {
				return f, ErrSyntaxError
			}
			f.User = val
		case "SKIPME":
			if !kill {
				return f, ErrSyntaxError
			}
			switch strings.ToLower(val) {
			case "yes":
				f.SkipMe = true
			case "no":
				f.SkipMe = false
			default:
				return f, ErrSyntaxError
			}
		case "MAXAGE":
			if !kill {
				return f, ErrSyntaxError
			}
			n, err := strconv.ParseInt(val, 10, 64)
			if err != nil || n < 0 {
				return f, ErrSyntaxError
			}
			f.MaxAge = n
		default:
			return f, ErrSyntaxError
		}
	}
	return f, nil
}

func cmdClientList(ctx *Context) error {
	h := clientHooksFor(ctx)
	if h == nil {
		return ctx.WriteBulkString("id=" + strconv.FormatInt(ctx.ClientID, 10) + " name=" + ctx.ClientName + "\n")
	}
	f, err := parseClientFilter(ctx, 1, false)
	if err != nil {
		return ctx.WriteError(err)
	}
	return ctx.WriteBulkString(h.List(ctx.ClientID, f))
}

func cmdClientInfo(ctx *Context) error {
	h := clientHooksFor(ctx)
	if h == nil {
		return ctx.WriteBulkString("id=" + strconv.FormatInt(ctx.ClientID, 10) + " name=" + ctx.ClientName + "\n")
	}
	return ctx.WriteBulkString(h.List(ctx.ClientID, ClientFilter{IDs: []int64{ctx.ClientID}}))
}

func cmdClientKill(ctx *Context) error {
	if ctx.ArgCount() < 2 {
		return ctx.WriteError(arityError("client|kill"))
	}
	h := clientHooksFor(ctx)
	if ctx.ArgCount() == 2 {
		// The old form: CLIENT KILL addr:port, answered with OK.
		if h == nil || h.Kill(ctx.ClientID, ClientFilter{Addr: ctx.ArgString(1)}) == 0 {
			return ctx.WriteError(errNoSuchClient)
		}
		return ctx.WriteOK()
	}
	f, err := parseClientFilter(ctx, 1, true)
	if err != nil {
		return ctx.WriteError(err)
	}
	if h == nil {
		return ctx.WriteInteger(0)
	}
	return ctx.WriteInteger(int64(h.Kill(ctx.ClientID, f)))
}

func cmdClientPause(ctx *Context) error {
	if ctx.ArgCount() < 2 || ctx.ArgCount() > 3 {
		return ctx.WriteError(arityError("client|pause"))
	}
	ms, err := strconv.ParseInt(ctx.ArgString(1), 10, 64)
	if err != nil || ms < 0 {
		return ctx.WriteError(errors.New("ERR timeout is not an integer or out of range"))
	}
	writesOnly := false
	if ctx.ArgCount() == 3 {
		switch strings.ToUpper(ctx.ArgString(2)) {
		case "WRITE":
			writesOnly = true
		case "ALL":
		default:
			return ctx.WriteError(ErrSyntaxError)
		}
	}
	if h := clientHooksFor(ctx); h != nil {
		h.Pause(time.Now().Add(time.Duration(ms)*time.Millisecond), writesOnly)
	}
	return ctx.WriteOK()
}

func cmdClientUnpause(ctx *Context) error {
	if ctx.ArgCount() != 1 {
		return ctx.WriteError(arityError("client|unpause"))
	}
	if h := clientHooksFor(ctx); h != nil {
		h.Unpause()
	}
	return ctx.WriteOK()
}

func cmdClientUnblock(ctx *Context) error {
	if ctx.ArgCount() < 2 || ctx.ArgCount() > 3 {
		return ctx.WriteError(arityError("client|unblock"))
	}
	id, err := strconv.ParseInt(ctx.ArgString(1), 10, 64)
	if err != nil {
		return ctx.WriteError(ErrNotInteger)
	}
	withError := false
	if ctx.ArgCount() == 3 {
		switch strings.ToUpper(ctx.ArgString(2)) {
		case "ERROR":
			withError = true
		case "TIMEOUT":
		default:
			return ctx.WriteError(errors.New("ERR CLIENT UNBLOCK reason should be TIMEOUT or ERROR"))
		}
	}
	h := clientHooksFor(ctx)
	if h != nil && h.Unblock(id, withError) {
		return ctx.WriteInteger(1)
	}
	return ctx.WriteInteger(0)
}

func cmdClientReply(ctx *Context) error {
	if ctx.ArgCount() != 2 {
		return ctx.WriteError(arityError("client|reply"))
	}
	switch mode := strings.ToLower(ctx.ArgString(1)); mode {
	case "on":
		ctx.ReplyMode = mode
		return ctx.WriteOK()
	case "off", "skip":
		ctx.ReplyMode = mode
		return nil
	default:
		return ctx.WriteError(ErrSyntaxError)
	}
}
