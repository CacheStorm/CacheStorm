package command

import (
	"errors"
	"strconv"
	"strings"

	"github.com/cachestorm/cachestorm/internal/resp"
)

var (
	errNoProto     = errors.New("NOPROTO unsupported protocol version")
	errHelloNoAuth = errors.New("NOAUTH HELLO must be called with the client already authenticated, " +
		"otherwise the HELLO <proto> AUTH <user> <pass> option can be used to authenticate the client " +
		"and select the RESP protocol version at the same time")
	errClientNameChars = errors.New("ERR Client names cannot contain spaces, newlines or special characters.") //nolint:staticcheck // Redis reply text
)

// helloRole reports the replication role HELLO announces.
var helloRole = func() string { return "master" }

func RegisterHelloCommands(router *Router) {
	router.Register(&CommandDef{Name: "HELLO", Handler: cmdHELLO})
}

// cmdHELLO implements HELLO [protover [AUTH username password] [SETNAME name]].
// The server speaks RESP2 only, so HELLO 3 answers NOPROTO; clients such as
// go-redis then fall back to RESP2 and AUTH/SELECT, as they do against a
// Redis that refuses RESP3.
func cmdHELLO(ctx *Context) error {
	if ctx.ArgCount() > 0 {
		proto, err := strconv.ParseInt(ctx.ArgString(0), 10, 64)
		if err != nil {
			return ctx.WriteError(errors.New("ERR Protocol version is not an integer or out of range"))
		}
		if proto != 2 {
			return ctx.WriteError(errNoProto)
		}
	}

	var authUser, authPass, name string
	authGiven, nameGiven := false, false
	for i := 1; i < ctx.ArgCount(); i++ {
		opt := strings.ToUpper(ctx.ArgString(i))
		switch {
		case opt == "AUTH" && i+2 < ctx.ArgCount():
			authUser, authPass = ctx.ArgString(i+1), ctx.ArgString(i+2)
			authGiven = true
			i += 2
		case opt == "SETNAME" && i+1 < ctx.ArgCount():
			name = ctx.ArgString(i + 1)
			nameGiven = true
			i++
		default:
			return ctx.WriteError(errors.New("ERR Syntax error in HELLO option '" + ctx.ArgString(i) + "'"))
		}
	}

	if nameGiven && !validClientName(name) {
		return ctx.WriteError(errClientNameChars)
	}

	if authGiven {
		if err := helloAuthenticate(ctx, authUser, authPass); err != nil {
			return ctx.WriteError(err)
		}
	} else if globalRouter != nil && globalRouter.RequirePass() != "" && !ctx.IsAuthenticated() {
		return ctx.WriteError(errHelloNoAuth)
	}

	if nameGiven {
		ctx.ClientName = name
	}

	return ctx.WriteArray([]*resp.Value{
		resp.BulkString("server"), resp.BulkString("redis"),
		resp.BulkString("version"), resp.BulkString(helloVersion()),
		resp.BulkString("proto"), resp.IntegerValue(2),
		resp.BulkString("id"), resp.IntegerValue(ctx.ClientID),
		resp.BulkString("mode"), resp.BulkString("standalone"),
		resp.BulkString("role"), resp.BulkString(helloRole()),
		resp.BulkString("modules"), resp.ArrayValue([]*resp.Value{}),
	})
}

// helloVersion is the Redis version CacheStorm is compatible with, which is
// what clients compare against when they decide what to send; the
// CacheStorm release (version.Version) is reported by INFO.
func helloVersion() string {
	return "7.4.0"
}

// helloAuthenticate applies HELLO's AUTH option: the default user checks the
// requirepass password, any other name an ACL user.
func helloAuthenticate(ctx *Context, username, password string) error {
	wrongPass := errors.New("WRONGPASS invalid username-password pair or user is disabled.") //nolint:staticcheck // Redis reply text
	if username == "default" {
		if globalRouter != nil && globalRouter.RequirePass() != "" && !globalRouter.ValidatePassword(password) {
			ctx.SetAuthenticated(false)
			return wrongPass
		}
		ctx.SetAuthenticated(true)
		ctx.ACLUser = nil
		ctx.Username = ""
		return nil
	}
	user, err := globalACL.Authenticate(username, password)
	if err != nil {
		ctx.SetAuthenticated(false)
		ctx.ACLUser = nil
		ctx.Username = ""
		return wrongPass
	}
	ctx.SetAuthenticated(true)
	ctx.ACLUser = user
	ctx.Username = username
	return nil
}

// validClientName applies Redis' rule for client names: printable ASCII
// without spaces.
func validClientName(name string) bool {
	for i := 0; i < len(name); i++ {
		if c := name[i]; c < '!' || c > '~' {
			return false
		}
	}
	return true
}
