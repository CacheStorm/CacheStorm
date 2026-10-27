package command

import (
	"bytes"
	"errors"
	"fmt"
	"net"
	"strconv"
	"strings"
	"time"

	"github.com/cachestorm/cachestorm/internal/persistence"
	"github.com/cachestorm/cachestorm/internal/resp"
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

// cmdMIGRATE implements MIGRATE host port key|"" destination-db timeout
// [COPY] [REPLACE] [AUTH password | AUTH2 username password]
// [KEYS key [key ...]]. Each key is sent as RESTORE with its DUMP payload
// and remaining TTL; without COPY the keys are then deleted here, which the
// AOF and the replicas record as DEL.
func cmdMIGRATE(ctx *Context) error {
	if ctx.ArgCount() < 5 {
		return ctx.WriteError(arityError("migrate"))
	}
	host, port := ctx.ArgString(0), ctx.ArgString(1)
	db, err := strconv.Atoi(ctx.ArgString(3))
	if err != nil || db < 0 {
		return ctx.WriteError(ErrNotInteger)
	}
	timeoutMS, err := strconv.ParseInt(ctx.ArgString(4), 10, 64)
	if err != nil || timeoutMS < 0 {
		return ctx.WriteError(ErrNotInteger)
	}
	if timeoutMS == 0 {
		timeoutMS = 1000
	}
	copyKeys, replace := false, false
	var auth [][]byte
	keys := []string{ctx.ArgString(2)}
	for i := 5; i < ctx.ArgCount(); i++ {
		switch strings.ToUpper(ctx.ArgString(i)) {
		case "COPY":
			copyKeys = true
		case "REPLACE":
			replace = true
		case "AUTH":
			if i+1 >= ctx.ArgCount() {
				return ctx.WriteError(ErrSyntaxError)
			}
			auth = [][]byte{ctx.Arg(i + 1)}
			i++
		case "AUTH2":
			if i+2 >= ctx.ArgCount() {
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

	type item struct {
		key     string
		ttl     int64
		payload []byte
	}
	var items []item
	now := time.Now().UnixMilli()
	for _, key := range keys {
		entry, ok := ctx.Store.Get(key)
		if !ok {
			continue
		}
		payload, err := persistence.EncodeDump(entry.Value)
		if err != nil {
			return ctx.WriteError(err)
		}
		ttl := int64(0)
		if at := entry.ExpiresAt.Load(); at > 0 {
			if ttl = at - now; ttl < 1 {
				ttl = 1
			}
		}
		items = append(items, item{key, ttl, payload})
	}
	if len(items) == 0 {
		return ctx.WriteSimpleString("NOKEY")
	}

	timeout := time.Duration(timeoutMS) * time.Millisecond
	conn, err := net.DialTimeout("tcp", net.JoinHostPort(host, port), timeout)
	if err != nil {
		return ctx.WriteError(fmt.Errorf("IOERR error or timeout connecting to the client"))
	}
	defer conn.Close()
	_ = conn.SetDeadline(time.Now().Add(timeout))

	var out bytes.Buffer
	w := resp.NewWriter(&out)
	send := func(args ...[]byte) {
		vals := make([]*resp.Value, len(args))
		for i, a := range args {
			vals[i] = resp.BulkBytes(a)
		}
		_ = w.WriteArray(vals)
	}
	expected := 0
	if auth != nil {
		send(append([][]byte{[]byte("AUTH")}, auth...)...)
		expected++
	}
	send([]byte("SELECT"), []byte(strconv.Itoa(db)))
	expected++
	for _, it := range items {
		args := [][]byte{[]byte("RESTORE"), []byte(it.key), []byte(strconv.FormatInt(it.ttl, 10)), it.payload}
		if replace {
			args = append(args, []byte("REPLACE"))
		}
		send(args...)
	}
	if _, err := conn.Write(out.Bytes()); err != nil {
		return ctx.WriteError(fmt.Errorf("IOERR error or timeout writing to target instance"))
	}
	rd := resp.NewReader(conn)
	for i := 0; i < expected; i++ {
		v, err := rd.ReadValue()
		if err != nil {
			return ctx.WriteError(fmt.Errorf("IOERR error or timeout reading to target instance"))
		}
		if v.Type == resp.TypeError {
			return ctx.Writer.WriteError("ERR Target instance replied with error: " + strings.TrimPrefix(v.Err, "ERR "))
		}
	}
	var moved []string
	var firstErr string
	for _, it := range items {
		v, err := rd.ReadValue()
		if err != nil {
			return ctx.WriteError(fmt.Errorf("IOERR error or timeout reading to target instance"))
		}
		if v.Type == resp.TypeError {
			if firstErr == "" {
				firstErr = v.Err
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
	if firstErr != "" {
		return ctx.Writer.WriteError("ERR Target instance replied with error: " + strings.TrimPrefix(firstErr, "ERR "))
	}
	return ctx.WriteOK()
}
