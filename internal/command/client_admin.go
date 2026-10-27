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

// clientHelp, aclHelp, moduleHelp, slowlogHelp and memoryHelp are the HELP
// texts of Redis 7.4.
const clientHelp = `CLIENT <subcommand> [<arg> [value] [opt] ...]. Subcommands are:
CACHING (YES|NO)
    Enable/disable tracking of the keys for next command in OPTIN/OPTOUT modes.
GETREDIR
    Return the client ID we are redirecting to when tracking is enabled.
GETNAME
    Return the name of the current connection.
ID
    Return the ID of the current connection.
INFO
    Return information about the current client connection.
KILL <ip:port>
    Kill connection made from <ip:port>.
KILL <option> <value> [<option> <value> [...]]
    Kill connections. Options are:
    * ADDR (<ip:port>|<unixsocket>:0)
      Kill connections made from the specified address
    * LADDR (<ip:port>|<unixsocket>:0)
      Kill connections made to specified local address
    * TYPE (NORMAL|MASTER|REPLICA|PUBSUB)
      Kill connections by type.
    * USER <username>
      Kill connections authenticated by <username>.
    * SKIPME (YES|NO)
      Skip killing current connection (default: yes).
    * ID <client-id>
      Kill connections by client id.
    * MAXAGE <maxage>
      Kill connections older than the specified age.
LIST [options ...]
    Return information about client connections. Options:
    * TYPE (NORMAL|MASTER|REPLICA|PUBSUB)
      Return clients of specified type.
UNPAUSE
    Stop the current client pause, resuming traffic.
PAUSE <timeout> [WRITE|ALL]
    Suspend all, or just write, clients for <timeout> milliseconds.
REPLY (ON|OFF|SKIP)
    Control the replies sent to the current connection.
SETNAME <name>
    Assign the name <name> to the current connection.
SETINFO <option> <value>
    Set client meta attr. Options are:
    * LIB-NAME: the client lib name.
    * LIB-VER: the client lib version.
UNBLOCK <clientid> [TIMEOUT|ERROR]
    Unblock the specified blocked client.
TRACKING (ON|OFF) [REDIRECT <id>] [BCAST] [PREFIX <prefix> [...]]
         [OPTIN] [OPTOUT] [NOLOOP]
    Control server assisted client side caching.
TRACKINGINFO
    Report tracking status for the current connection.
NO-EVICT (ON|OFF)
    Protect current client connection from eviction.
NO-TOUCH (ON|OFF)
    Will not touch LRU/LFU stats when this mode is on.
HELP
    Print this help.`

const aclHelp = `ACL <subcommand> [<arg> [value] [opt] ...]. Subcommands are:
CAT [<category>]
    List all commands that belong to <category>, or all command categories
    when no category is specified.
DELUSER <username> [<username> ...]
    Delete a list of users.
DRYRUN <username> <command> [<arg> ...]
    Returns whether the user can execute the given command without executing the command.
GETUSER <username>
    Get the user's details.
GENPASS [<bits>]
    Generate a secure 256-bit user password. The optional ` + "`bits`" + ` argument can
    be used to specify a different size.
LIST
    Show users details in config file format.
LOAD
    Reload users from the ACL file.
LOG [<count> | RESET]
    Show the ACL log entries.
SAVE
    Save the current config to the ACL file.
SETUSER <username> <attribute> [<attribute> ...]
    Create or modify a user with the specified attributes.
USERS
    List all the registered usernames.
WHOAMI
    Return the current connection username.
HELP
    Print this help.`

const moduleHelp = `MODULE <subcommand> [<arg> [value] [opt] ...]. Subcommands are:
LIST
    Return a list of loaded modules.
LOAD <path> [<arg> ...]
    Load a module library from <path>, passing to it any optional arguments.
LOADEX <path> [[CONFIG NAME VALUE] [CONFIG NAME VALUE]] [ARGS ...]
    Load a module library from <path>, while passing it module configurations and optional arguments.
UNLOAD <name>
    Unload a module.
HELP
    Print this help.`

const slowlogHelp = `SLOWLOG <subcommand> [<arg> [value] [opt] ...]. Subcommands are:
GET [<count>]
    Return top <count> entries from the slowlog (default: 10, -1 mean all).
    Entries are made of:
    id, timestamp, time in microseconds, arguments array, client IP and port,
    client name
LEN
    Return the length of the slowlog.
RESET
    Reset the slowlog.
HELP
    Print this help.`

const memoryHelp = `MEMORY <subcommand> [<arg> [value] [opt] ...]. Subcommands are:
DOCTOR
    Return memory problems reports.
MALLOC-STATS
    Return internal statistics report from the memory allocator.
PURGE
    Attempt to purge dirty pages for reclamation by the allocator.
STATS
    Return information about the memory usage of the server.
USAGE <key> [SAMPLES <count>]
    Return memory in bytes used by <key> and its value. Nested values are
    sampled up to <count> times (default: 5, 0 means sample all).
HELP
    Print this help.`
