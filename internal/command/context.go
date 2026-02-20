package command

import (
	"errors"
	"sync/atomic"
	"time"

	"github.com/cachestorm/cachestorm/internal/resp"
	"github.com/cachestorm/cachestorm/internal/store"
)

var (
	ErrUnknownCommand  = errors.New("ERR unknown command")
	ErrWrongArgCount   = errors.New("ERR wrong number of arguments")
	ErrInvalidArg      = errors.New("ERR invalid argument")
	ErrSyntaxError     = errors.New("ERR syntax error")
	ErrNotInteger      = errors.New("ERR value is not an integer or out of range")
	ErrNotFloat        = errors.New("ERR value is not a valid float")
	ErrIndexOutOfRange = errors.New("ERR index out of range")
)

var subscriberID atomic.Int64

type Context struct {
	Command       string
	Args          [][]byte
	Store         *store.Store
	Writer        *resp.Writer
	StartTime     time.Time
	Authenticated bool
	ClientID      int64
	Namespace     string
	Transaction   *Transaction
	Subscriber    *store.Subscriber
	Username      string
	RemoteAddr    string
}

func NewContext(cmd string, args [][]byte, s *store.Store, w *resp.Writer) *Context {
	return &Context{
		Command:     cmd,
		Args:        args,
		Store:       s,
		Writer:      w,
		Transaction: NewTransaction(),
	}
}

func NewContextWithClient(cmd string, args [][]byte, s *store.Store, w *resp.Writer, clientID int64, remoteAddr string) *Context {
	return &Context{
		Command:     cmd,
		Args:        args,
		Store:       s,
		Writer:      w,
		Transaction: NewTransaction(),
		ClientID:    clientID,
		RemoteAddr:  remoteAddr,
	}
}

func (ctx *Context) IsAuthenticated() bool {
	return ctx.Authenticated
}

func (ctx *Context) SetAuthenticated(auth bool) {
	ctx.Authenticated = auth
}

func (ctx *Context) GetTransaction() *Transaction {
	if ctx.Transaction == nil {
		ctx.Transaction = NewTransaction()
	}
	return ctx.Transaction
}

func (ctx *Context) GetSubscriber() *store.Subscriber {
	if ctx.Subscriber == nil {
		ctx.Subscriber = store.NewSubscriber(subscriberID.Add(1))
	}
	return ctx.Subscriber
}

func (ctx *Context) Arg(n int) []byte {
	if n < 0 || n >= len(ctx.Args) {
		return nil
	}
	return ctx.Args[n]
}

func (ctx *Context) ArgString(n int) string {
	return string(ctx.Arg(n))
}

func (ctx *Context) ArgCount() int {
	return len(ctx.Args)
}

func (ctx *Context) WriteOK() error {
	return ctx.Writer.WriteOK()
}

func (ctx *Context) WriteError(err error) error {
	return ctx.Writer.WriteError(err.Error())
}

func (ctx *Context) WriteValue(v *resp.Value) error {
	return ctx.Writer.WriteValue(v)
}

func (ctx *Context) WriteSimpleString(s string) error {
	return ctx.Writer.WriteSimpleString(s)
}

func (ctx *Context) WriteBulkString(s string) error {
	return ctx.Writer.WriteBulkString(s)
}

func (ctx *Context) WriteBulkBytes(b []byte) error {
	return ctx.Writer.WriteBulkBytes(b)
}

func (ctx *Context) WriteInteger(n int64) error {
	return ctx.Writer.WriteInteger(n)
}

func (ctx *Context) WriteNull() error {
	return ctx.Writer.WriteNull()
}

func (ctx *Context) WriteNullBulkString() error {
	return ctx.Writer.WriteNullBulkString()
}

func (ctx *Context) WriteArray(items []*resp.Value) error {
	return ctx.Writer.WriteArray(items)
}
