package command

import (
	"errors"
	"fmt"
	"strconv"
	"strings"

	"github.com/cachestorm/cachestorm/internal/resp"
	"github.com/cachestorm/cachestorm/internal/store"
)

// A message queue is a stream key read through the consumer group
// "msgqueue". Its first entry, 0-1, holds the queue's settings and is never
// delivered. Where a message stands is kept in the group's pending entries:
//
//   - not yet delivered: after the group's last delivered ID;
//   - being processed: owned by the consumer "vt:<ms>", named after the
//     visibility timeout it was taken with, and delivered less than that
//     long ago;
//   - waiting for redelivery: owned by "retry" (NACK, REQUEUE), or by a
//     "vt:" consumer whose timeout passed while retries remain;
//   - dead-lettered: owned by "dead", or by a "vt:" consumer whose timeout
//     passed with no retries left.
//
// A message's retry count is its delivery count, less the delivery under
// way. Every change is recorded as the stream commands it amounts to, so
// the AOF and the replication stream are ones Redis loads.
const (
	mqGroup      = "msgqueue"
	mqRetry      = "retry"
	mqDead       = "dead"
	mqVisPrefix  = "vt:"
	mqMaxRetries = "max_retries"
)

var (
	mqMetaID            = store.StreamID{Ms: 0, Seq: 1}
	errMsgQueueNotFound = errors.New("ERR queue not found")
	errNotAMsgQueue     = store.ErrWrongType
)

type msgQueue struct {
	key        string
	stream     *store.StreamValue
	maxRetries int64
	now        int64
}

func openMsgQueue(ctx *Context, key string) (*msgQueue, error) {
	stream, err := lookupStream(ctx, key)
	if err != nil {
		return nil, err
	}
	if stream == nil {
		return nil, errMsgQueueNotFound
	}
	if !stream.HasGroup(mqGroup) {
		return nil, errNotAMsgQueue
	}
	meta := stream.GetEntryByID(mqMetaID.String())
	if meta == nil {
		return nil, errNotAMsgQueue
	}
	n, err := strconv.ParseInt(string(meta.FieldMap()[mqMaxRetries]), 10, 64)
	if err != nil || n < 1 {
		return nil, errNotAMsgQueue
	}
	return &msgQueue{key: key, stream: stream, maxRetries: n, now: ctx.nowMS()}, nil
}

func visibilityTimeout(consumer string) (int64, bool) {
	s, ok := strings.CutPrefix(consumer, mqVisPrefix)
	if !ok {
		return 0, false
	}
	n, err := strconv.ParseInt(s, 10, 64)
	return n, err == nil && n > 0
}

func (q *msgQueue) inFlight(p store.PendingEntry) bool {
	t, ok := visibilityTimeout(p.Consumer)
	return ok && q.now-p.DeliveryTS < t
}

func (q *msgQueue) timedOut(p store.PendingEntry) bool {
	_, ok := visibilityTimeout(p.Consumer)
	return ok && !q.inFlight(p)
}

func (q *msgQueue) dead(p store.PendingEntry) bool {
	return p.Consumer == mqDead || (q.timedOut(p) && p.Deliveries >= q.maxRetries)
}

func (q *msgQueue) redeliverable(p store.PendingEntry) bool {
	return p.Consumer == mqRetry || (q.timedOut(p) && p.Deliveries < q.maxRetries)
}

func (q *msgQueue) body(id store.StreamID) string {
	e := q.stream.GetEntryByID(id.String())
	if e == nil {
		return ""
	}
	return string(e.FieldMap()["body"])
}

func messageReply(id, body string, retries int64) []*resp.Value {
	return []*resp.Value{
		resp.BulkString("id"), resp.BulkString(id),
		resp.BulkString("body"), resp.BulkString(body),
		resp.BulkString("retries"), resp.IntegerValue(retries),
	}
}

func cmdArgs(args ...string) [][]byte {
	out := make([][]byte, len(args))
	for i, a := range args {
		out[i] = []byte(a)
	}
	return out
}

// claimRecord is the XCLAIM that puts a pending entry where a queue command
// left it, with its delivery time and count spelled out.
func (q *msgQueue) claimRecord(ctx *Context, consumer string, id string, deliveries int64, extra ...string) ExecutedCommand {
	args := append([]string{q.key, mqGroup, consumer, "0", id,
		"TIME", strconv.FormatInt(q.now, 10), "RETRYCOUNT", strconv.FormatInt(deliveries, 10), "JUSTID"}, extra...)
	return ExecutedCommand{Name: "XCLAIM", Args: cmdArgs(args...), DB: ctx.DB}
}

func cmdMSGQUEUECREATE(ctx *Context) error {
	if ctx.ArgCount() < 1 || ctx.ArgCount() > 2 {
		return ctx.WriteError(ErrWrongArgCount)
	}
	key := ctx.ArgString(0)
	maxRetries := int64(3)
	if ctx.ArgCount() == 2 {
		v, err := strconv.ParseInt(ctx.ArgString(1), 10, 32)
		if err != nil || v < 1 {
			return ctx.WriteError(fmt.Errorf("ERR max retries must be a positive integer"))
		}
		maxRetries = v
	}
	if _, err := openMsgQueue(ctx, key); err != nil && !errors.Is(err, errMsgQueueNotFound) {
		return ctx.WriteError(err)
	}
	retries := strconv.FormatInt(maxRetries, 10)
	stream := store.NewStreamValue(0)
	if _, err := stream.AddEntry(store.StreamAddArgs{ID: mqMetaID, IDGiven: true, SeqGiven: true},
		[]store.StreamField{{Name: mqMaxRetries, Value: []byte(retries)}}, uint64(ctx.nowMS())); err != nil {
		return ctx.WriteError(err)
	}
	stream.CreateGroupAt(mqGroup, mqMetaID, false, 1)
	if err := ctx.Store.Set(key, stream, store.SetOptions{}); err != nil {
		return ctx.WriteError(err)
	}
	ctx.PropagateAs(
		ExecutedCommand{Name: "DEL", Args: cmdArgs(key), DB: ctx.DB},
		ExecutedCommand{Name: "XADD", Args: cmdArgs(key, mqMetaID.String(), mqMaxRetries, retries), DB: ctx.DB},
		ExecutedCommand{Name: "XGROUP", Args: cmdArgs("CREATE", key, mqGroup, mqMetaID.String(), "ENTRIESREAD", "1"), DB: ctx.DB},
	)
	return ctx.WriteOK()
}

func cmdMSGQUEUEPUBLISH(ctx *Context) error {
	if ctx.ArgCount() != 2 {
		return ctx.WriteError(ErrWrongArgCount)
	}
	q, err := openMsgQueue(ctx, ctx.ArgString(0))
	if err != nil {
		return ctx.WriteError(err)
	}
	entry, err := q.stream.AddEntry(store.StreamAddArgs{}, []store.StreamField{{Name: "body", Value: ctx.Arg(1)}}, uint64(ctx.nowMS()))
	if err != nil {
		return ctx.WriteError(err)
	}
	ctx.PropagateAs(ExecutedCommand{Name: "XADD", Args: [][]byte{[]byte(q.key), []byte(entry.ID), []byte("body"), ctx.Arg(1)}, DB: ctx.DB})
	ctx.notify(store.NotifyStream, "xadd", q.key)
	ctx.Store.KeyNotifier().NotifyKey(q.key)
	return ctx.WriteBulkString(entry.ID)
}

func cmdMSGQUEUECONSUME(ctx *Context) error {
	if ctx.ArgCount() < 1 || ctx.ArgCount() > 2 {
		return ctx.WriteError(ErrWrongArgCount)
	}
	timeout := int64(5000)
	if ctx.ArgCount() == 2 {
		v, err := strconv.ParseInt(ctx.ArgString(1), 10, 64)
		if err != nil || v < 1 {
			return ctx.WriteError(fmt.Errorf("ERR visibility timeout must be a positive integer"))
		}
		timeout = v
	}
	q, err := openMsgQueue(ctx, ctx.ArgString(0))
	if err != nil {
		return ctx.WriteError(err)
	}
	consumer := mqVisPrefix + strconv.FormatInt(timeout, 10)
	item, p, entriesRead, claimed, found, _ := q.stream.TakeForGroup(mqGroup, consumer, q.redeliverable, q.now)
	if !found {
		ctx.PropagateAs()
		return ctx.WriteNull()
	}
	id := item.ID.String()
	if claimed {
		ctx.PropagateAs(q.claimRecord(ctx, consumer, id, p.Deliveries))
	} else {
		ctx.PropagateAs(
			q.claimRecord(ctx, consumer, id, p.Deliveries, "FORCE", "LASTID", id),
			ExecutedCommand{Name: "XGROUP", Args: cmdArgs("SETID", q.key, mqGroup, id, "ENTRIESREAD", strconv.FormatInt(entriesRead, 10)), DB: ctx.DB},
		)
	}
	return ctx.WriteArray(messageReply(id, string(item.Entry.FieldMap()["body"]), p.Deliveries-1))
}

// settle moves a message the way decide says and records the move.
func (q *msgQueue) settle(ctx *Context, rawID string, decide func(store.PendingEntry) (string, int64, bool)) (string, bool) {
	id, err := parseStrictStreamID(rawID)
	if err != nil || id == mqMetaID {
		ctx.PropagateAs()
		return "", false
	}
	consumer, deliveries, moved := q.stream.MovePendingIf(mqGroup, id, q.now, decide)
	if !moved {
		ctx.PropagateAs()
		return "", false
	}
	ctx.PropagateAs(q.claimRecord(ctx, consumer, id.String(), deliveries))
	return consumer, true
}

func cmdMSGQUEUEACK(ctx *Context) error {
	if ctx.ArgCount() != 2 {
		return ctx.WriteError(ErrWrongArgCount)
	}
	q, err := openMsgQueue(ctx, ctx.ArgString(0))
	if err != nil {
		return ctx.WriteError(err)
	}
	id, err := parseStrictStreamID(ctx.ArgString(1))
	if err != nil || id == mqMetaID || !q.stream.AckAndDeleteIf(mqGroup, id, q.inFlight) {
		ctx.PropagateAs()
		return ctx.WriteInteger(0)
	}
	ctx.PropagateAs(
		ExecutedCommand{Name: "XACK", Args: cmdArgs(q.key, mqGroup, id.String()), DB: ctx.DB},
		ExecutedCommand{Name: "XDEL", Args: cmdArgs(q.key, id.String()), DB: ctx.DB},
	)
	return ctx.WriteInteger(1)
}

func cmdMSGQUEUENACK(ctx *Context) error {
	if ctx.ArgCount() != 2 {
		return ctx.WriteError(ErrWrongArgCount)
	}
	q, err := openMsgQueue(ctx, ctx.ArgString(0))
	if err != nil {
		return ctx.WriteError(err)
	}
	to, moved := q.settle(ctx, ctx.ArgString(1), func(p store.PendingEntry) (string, int64, bool) {
		if !q.inFlight(p) {
			return "", 0, false
		}
		if p.Deliveries >= q.maxRetries {
			return mqDead, p.Deliveries, true
		}
		return mqRetry, p.Deliveries, true
	})
	switch {
	case !moved:
		return ctx.WriteInteger(0)
	case to == mqDead:
		return ctx.WriteInteger(2)
	}
	return ctx.WriteInteger(1)
}

func cmdMSGQUEUEREQUEUE(ctx *Context) error {
	if ctx.ArgCount() != 2 {
		return ctx.WriteError(ErrWrongArgCount)
	}
	q, err := openMsgQueue(ctx, ctx.ArgString(0))
	if err != nil {
		return ctx.WriteError(err)
	}
	if _, moved := q.settle(ctx, ctx.ArgString(1), func(p store.PendingEntry) (string, int64, bool) {
		return mqRetry, 0, q.dead(p)
	}); !moved {
		return ctx.WriteInteger(0)
	}
	return ctx.WriteInteger(1)
}

func cmdMSGQUEUEDEADLETTER(ctx *Context) error {
	if ctx.ArgCount() != 1 {
		return ctx.WriteError(ErrWrongArgCount)
	}
	q, err := openMsgQueue(ctx, ctx.ArgString(0))
	if err != nil {
		return ctx.WriteError(err)
	}
	pel, _ := q.stream.PendingOf(mqGroup)
	results := []*resp.Value{}
	for _, p := range pel {
		if !q.dead(p) {
			continue
		}
		id, err := store.ParseStreamID(p.ID, 0)
		if err != nil {
			continue
		}
		results = append(results, resp.ArrayValue(messageReply(p.ID, q.body(id), p.Deliveries)))
	}
	return ctx.WriteArray(results)
}

func cmdMSGQUEUEPURGE(ctx *Context) error {
	if ctx.ArgCount() != 1 {
		return ctx.WriteError(ErrWrongArgCount)
	}
	q, err := openMsgQueue(ctx, ctx.ArgString(0))
	if err != nil {
		return ctx.WriteError(err)
	}
	pel, _ := q.stream.PendingOf(mqGroup)
	acked := []string{q.key, mqGroup}
	deleted := []string{q.key}
	for _, p := range pel {
		id, err := store.ParseStreamID(p.ID, 0)
		if err != nil || !q.redeliverable(p) {
			continue
		}
		if q.stream.AckAndDeleteIf(mqGroup, id, q.redeliverable) {
			acked = append(acked, p.ID)
			deleted = append(deleted, p.ID)
		}
	}
	fresh, _ := q.stream.Undelivered(mqGroup)
	n := q.stream.DeleteIDs(fresh)
	for _, id := range fresh {
		deleted = append(deleted, id.String())
	}
	n += int64(len(acked) - 2)
	var record []ExecutedCommand
	if len(acked) > 2 {
		record = append(record, ExecutedCommand{Name: "XACK", Args: cmdArgs(acked...), DB: ctx.DB})
	}
	if len(deleted) > 1 {
		record = append(record, ExecutedCommand{Name: "XDEL", Args: cmdArgs(deleted...), DB: ctx.DB})
	}
	ctx.PropagateAs(record...)
	return ctx.WriteInteger(n)
}

func cmdMSGQUEUESTATS(ctx *Context) error {
	if ctx.ArgCount() != 1 {
		return ctx.WriteError(ErrWrongArgCount)
	}
	q, err := openMsgQueue(ctx, ctx.ArgString(0))
	if err != nil {
		return ctx.WriteError(err)
	}
	pending, _ := q.stream.UndeliveredCount(mqGroup)
	var processing, dead int64
	pel, _ := q.stream.PendingOf(mqGroup)
	for _, p := range pel {
		switch {
		case q.inFlight(p):
			processing++
		case q.dead(p):
			dead++
		case q.redeliverable(p):
			pending++
		}
	}
	return ctx.WriteArray([]*resp.Value{
		resp.BulkString("name"), resp.BulkString(q.key),
		resp.BulkString("pending"), resp.IntegerValue(pending),
		resp.BulkString("processing"), resp.IntegerValue(processing),
		resp.BulkString("dead_letters"), resp.IntegerValue(dead),
		resp.BulkString("max_retries"), resp.IntegerValue(q.maxRetries),
	})
}

func cmdMSGQUEUEDELETE(ctx *Context) error {
	if ctx.ArgCount() != 1 {
		return ctx.WriteError(ErrWrongArgCount)
	}
	key := ctx.ArgString(0)
	if _, err := openMsgQueue(ctx, key); err != nil {
		ctx.PropagateAs()
		if errors.Is(err, errMsgQueueNotFound) {
			return ctx.WriteInteger(0)
		}
		return ctx.WriteError(err)
	}
	ctx.Store.Delete(key)
	ctx.PropagateAs(ExecutedCommand{Name: "DEL", Args: cmdArgs(key), DB: ctx.DB})
	return ctx.WriteInteger(1)
}
