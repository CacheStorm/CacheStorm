package command

import (
	"errors"
	"slices"
	"strings"

	"github.com/cachestorm/cachestorm/internal/persistence"
	"github.com/cachestorm/cachestorm/internal/store"
)

// RegisterHImportCommands registers Redis 8.10's HIMPORT: a connection
// names a fieldset once with HIMPORT PREPARE and then creates hashes of
// those fields from their values alone with HIMPORT SET.
func RegisterHImportCommands(router *Router) {
	router.Register(&CommandDef{Name: "HIMPORT", Handler: cmdHIMPORT})
}

// himportFieldset is a prepared fieldset: its fields in Redis' template
// order (shorter first, then bytewise) and, for each, the position of its
// value among HIMPORT SET's values.
type himportFieldset struct {
	fields []string
	order  []int
}

var (
	errHImportDuplicate = errors.New("ERR duplicate field name in fieldset")
	errHImportNoSet     = errors.New("ERR no such fieldset")
	errHImportCount     = errors.New("ERR value count does not match fieldset field count")
)

// himportFieldsets returns the connection's fieldsets, created on demand
// when create is set. They live on the connection's Transaction, the state
// a connection keeps between its commands, until HIMPORT DISCARD,
// DISCARDALL, RESET or the connection closing.
func (tx *Transaction) himportFieldsets(create bool) map[string]*himportFieldset {
	if tx.himport == nil && create {
		tx.himport = make(map[string]*himportFieldset)
	}
	return tx.himport
}

func cmdHIMPORT(ctx *Context) error {
	if !checkRedisSubcommand(ctx, "himport", nil) {
		return nil
	}
	tx := ctx.Transaction
	if tx == nil {
		tx = NewTransaction()
		ctx.Transaction = tx
	}
	switch strings.ToUpper(ctx.ArgString(0)) {
	case "PREPARE":
		return himportPrepare(ctx, tx)
	case "SET":
		return himportSet(ctx, tx)
	case "DISCARD":
		tx.mu.Lock()
		sets := tx.himportFieldsets(false)
		_, ok := sets[ctx.ArgString(1)]
		delete(sets, ctx.ArgString(1))
		if len(sets) == 0 {
			tx.himport = nil
		}
		tx.mu.Unlock()
		return ctx.WriteInteger(int64(boolInt(ok)))
	default: // DISCARDALL
		tx.mu.Lock()
		n := len(tx.himport)
		tx.himport = nil
		tx.mu.Unlock()
		return ctx.WriteInteger(int64(n))
	}
}

// himportFieldCmp orders fields as Redis' sdscmplen: by length, then bytes.
func himportFieldCmp(a, b string) int {
	if len(a) != len(b) {
		if len(a) < len(b) {
			return -1
		}
		return 1
	}
	return strings.Compare(a, b)
}

func himportPrepare(ctx *Context, tx *Transaction) error {
	given := make([]string, ctx.ArgCount()-2)
	for i := range given {
		given[i] = ctx.ArgString(i + 2)
	}
	order := make([]int, len(given))
	for i := range order {
		order[i] = i
	}
	slices.SortStableFunc(order, func(a, b int) int { return himportFieldCmp(given[a], given[b]) })
	fields := make([]string, len(given))
	for i, at := range order {
		fields[i] = given[at]
		if i > 0 && fields[i] == fields[i-1] {
			return ctx.WriteError(errHImportDuplicate)
		}
	}
	tx.mu.Lock()
	tx.himportFieldsets(true)[ctx.ArgString(1)] = &himportFieldset{fields: fields, order: order}
	tx.mu.Unlock()
	return ctx.WriteOK()
}

// himportSet is HIMPORT SET key fieldset value...: it replaces key with a
// hash of the fieldset's fields, as Redis' setKey does (the key loses its
// TTL), and goes to replicas and the AOF as RESTORE ... REPLACE, as Redis
// sends it.
func himportSet(ctx *Context, tx *Transaction) error {
	ctx.writeEffectsHandled = true
	key := ctx.ArgString(1)
	if entry, ok := ctx.Store.Get(key); ok {
		if _, isHash := entry.Value.(*store.HashValue); !isHash {
			ctx.PropagateAs()
			return ctx.WriteError(store.ErrWrongType)
		}
	}
	tx.mu.Lock()
	fs := tx.himportFieldsets(false)[ctx.ArgString(2)]
	tx.mu.Unlock()
	if fs == nil {
		ctx.PropagateAs()
		return ctx.WriteError(errHImportNoSet)
	}
	values := ctx.Args[3:]
	if len(values) != len(fs.fields) {
		ctx.PropagateAs()
		return ctx.WriteError(errHImportCount)
	}
	h := store.NewHash(len(fs.fields))
	for i, f := range fs.fields {
		h.SetFieldKeepTTL(f, append([]byte(nil), values[fs.order[i]]...))
	}
	payload, err := persistence.EncodeDump(h)
	if err != nil {
		ctx.PropagateAs()
		return ctx.WriteError(err)
	}
	if err := ctx.replaceKey(key, h, store.SetOptions{}); err != nil {
		ctx.PropagateAs()
		return ctx.WriteError(err)
	}
	ctx.PropagateAs(ExecutedCommand{Name: "RESTORE", Args: [][]byte{[]byte(key), []byte("0"), payload, []byte("REPLACE")}, DB: ctx.DB})
	var subkeys []string
	if ctx.Store.NotifyingSubkeys(store.NotifyHash) {
		subkeys = fs.fields
	}
	ctx.notifyFields("hset", key, subkeys)
	return ctx.WriteOK()
}
