package command

import (
	"errors"
	"fmt"
	"strconv"
	"strings"
)

// ErrClusterDisabled is the reply of a Redis instance running without
// cluster support to every cluster command.
var ErrClusterDisabled = errors.New("ERR This instance has cluster support disabled")

func RegisterClusterCommands(router *Router) {
	router.Register(&CommandDef{Name: "CLUSTER", Handler: cmdClusterDisabled})
	router.Register(&CommandDef{Name: "CLUSTERINFO", Handler: cmdClusterDisabled})
	router.Register(&CommandDef{Name: "CLUSTERNODES", Handler: cmdClusterDisabled})
	router.Register(&CommandDef{Name: "CLUSTERSLOTS", Handler: cmdClusterDisabled})
	router.Register(&CommandDef{Name: "MIGRATE", Handler: cmdMIGRATE})
	router.Register(&CommandDef{Name: "ASKING", Handler: cmdClusterDisabled})
	router.Register(&CommandDef{Name: "READONLY", Handler: cmdClusterDisabled})
	router.Register(&CommandDef{Name: "READWRITE", Handler: cmdClusterDisabled})
}

// cmdClusterDisabled answers the cluster commands the way a Redis instance
// started without cluster-enabled does: this release has no cluster mode.
func cmdClusterDisabled(ctx *Context) error {
	return ctx.WriteError(ErrClusterDisabled)
}

func cmdMIGRATE(ctx *Context) error {
	if ctx.ArgCount() < 4 {
		return ctx.WriteError(ErrWrongArgCount)
	}

	host := ctx.ArgString(0)
	port := ctx.ArgString(1)
	key := ctx.ArgString(2)
	destinationDB := ctx.ArgString(3)

	_ = host
	_ = port
	_ = key
	_ = destinationDB

	copy := false
	replace := false
	timeout := 0

	for i := 4; i < ctx.ArgCount(); i++ {
		arg := strings.ToUpper(ctx.ArgString(i))
		switch arg {
		case "COPY":
			copy = true
		case "REPLACE":
			replace = true
		case "AUTH":
			i++
		case "AUTH2":
			i += 2
		case "TIMEOUT":
			i++
			if i < ctx.ArgCount() {
				var err error
				timeout, err = strconv.Atoi(ctx.ArgString(i))
				if err != nil {
					return ctx.WriteError(ErrNotInteger)
				}
			}
		}
	}

	_ = copy
	_ = replace
	_ = timeout

	entry, exists := ctx.Store.Get(key)
	if !exists {
		return ctx.WriteBulkString("NOKEY")
	}

	// This build cannot reach a destination instance, so no transfer is
	// possible. Answering +OK would tell the caller the key has moved when
	// nothing was transferred — and, without COPY, Redis only reports OK once
	// the key has also been removed from this instance. Refuse explicitly
	// instead of claiming a migration that never ran.
	_ = entry
	return ctx.WriteError(fmt.Errorf("ERR MIGRATE is not supported by this server (destination %s:%s db %s)", host, port, destinationDB))
}
