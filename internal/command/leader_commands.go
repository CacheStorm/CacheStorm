package command

import (
	"errors"
	"strconv"

	"github.com/cachestorm/cachestorm/internal/resp"
	"github.com/cachestorm/cachestorm/internal/store"
)

// An election is a hash key holding the leader (empty when there is none),
// its term, the unix millisecond its leadership expires at, and the leaders
// of the last hundred terms as fields "h:<term>". Being a key, it is per
// database, persisted and replicated, so a restarted server or a replica
// knows the leader and its term. An election changes as HSET of its new
// values; a new term rewrites the hash (DEL and HSET) to drop the oldest
// history.

var (
	leaderKind        = recordKind{name: "election", fields: []string{"leader", "term", "expires_at"}, extra: true}
	errNoElection     = errors.New("ERR election not found")
	errLeaderTTL      = errors.New("ERR ttl must be a positive integer")
	leaderHistorySize = int64(100)
)

func historyField(term int64) string { return "h:" + strconv.FormatInt(term, 10) }

func leaderTTL(ctx *Context, at int) (int64, error) {
	if ctx.ArgCount() <= at {
		return 10000, nil
	}
	n, err := strconv.ParseInt(ctx.ArgString(at), 10, 64)
	if err != nil || n <= 0 {
		return 0, errLeaderTTL
	}
	return n, nil
}

type election struct {
	leader          string
	term, expiresAt int64
	r               *record
}

func readElection(ctx *Context, key string, history bool) (*election, error) {
	var extra []string
	r, err := leaderKind.read(ctx, key)
	if err != nil || r == nil {
		return nil, err
	}
	term := r.int("term")
	if history && !r.bad {
		for t := max(1, term-leaderHistorySize+1); t <= term; t++ {
			extra = append(extra, historyField(t))
		}
		if r, err = leaderKind.read(ctx, key, extra...); err != nil || r == nil {
			return nil, err
		}
	}
	e := &election{leader: r.vals["leader"], term: r.int("term"), expiresAt: r.int("expires_at"), r: r}
	if r.bad || e.term < 0 {
		return nil, store.ErrWrongType
	}
	return e, nil
}

func (e *election) active(now int64) bool { return e.leader != "" && now <= e.expiresAt }

// LEADER.ELECT name candidate [ttl_ms] makes candidate the leader of a new
// term when there is none or its leadership expired.
func cmdLEADERELECT(ctx *Context) error {
	if ctx.ArgCount() < 2 || ctx.ArgCount() > 3 {
		return ctx.WriteError(ErrWrongArgCount)
	}
	key, candidate := ctx.ArgString(0), ctx.ArgString(1)
	ttl, err := leaderTTL(ctx, 2)
	if err != nil {
		return ctx.WriteError(err)
	}
	e, err := readElection(ctx, key, true)
	if err != nil {
		return ctx.WriteError(err)
	}
	now := ctx.nowMS()
	if e != nil && e.active(now) {
		ctx.PropagateAs()
		return ctx.WriteArray([]*resp.Value{
			resp.BulkString("elected"), resp.IntegerValue(0),
			resp.BulkString("current_leader"), resp.BulkString(e.leader),
		})
	}
	term := int64(1)
	if e != nil {
		term = e.term + 1
	}
	pairs := []string{"leader", candidate, "term", i64(term), "expires_at", i64(now + ttl)}
	if e != nil {
		for t := max(1, term-leaderHistorySize+1); t < term; t++ {
			if v, ok := e.r.vals[historyField(t)]; ok {
				pairs = append(pairs, historyField(t), v)
			}
		}
	}
	pairs = append(pairs, historyField(term), candidate)
	if err := replaceRecord(ctx, key, pairs...); err != nil {
		return ctx.WriteError(err)
	}
	return ctx.WriteArray([]*resp.Value{
		resp.BulkString("elected"), resp.IntegerValue(1),
		resp.BulkString("term"), resp.IntegerValue(term),
	})
}

// leaderChange applies change to the election when candidate leads it.
func leaderChange(ctx *Context, pairs func(now int64) ([]string, error)) error {
	if ctx.ArgCount() < 2 {
		return ctx.WriteError(ErrWrongArgCount)
	}
	key, candidate := ctx.ArgString(0), ctx.ArgString(1)
	e, err := readElection(ctx, key, false)
	if err != nil {
		return ctx.WriteError(err)
	}
	if e == nil {
		return ctx.WriteError(errNoElection)
	}
	if e.leader != candidate || candidate == "" {
		ctx.PropagateAs()
		return ctx.WriteInteger(0)
	}
	p, err := pairs(ctx.nowMS())
	if err != nil {
		return ctx.WriteError(err)
	}
	if err := updateRecord(ctx, key, p...); err != nil {
		return ctx.WriteError(err)
	}
	return ctx.WriteInteger(1)
}

// LEADER.RENEW name candidate [ttl_ms] extends the candidate's leadership.
func cmdLEADERRENEW(ctx *Context) error {
	if ctx.ArgCount() > 3 {
		return ctx.WriteError(ErrWrongArgCount)
	}
	return leaderChange(ctx, func(now int64) ([]string, error) {
		ttl, err := leaderTTL(ctx, 2)
		return []string{"expires_at", i64(now + ttl)}, err
	})
}

// LEADER.RESIGN name candidate ends the candidate's leadership.
func cmdLEADERRESIGN(ctx *Context) error {
	if ctx.ArgCount() != 2 {
		return ctx.WriteError(ErrWrongArgCount)
	}
	return leaderChange(ctx, func(int64) ([]string, error) {
		return []string{"leader", "", "expires_at", "0"}, nil
	})
}

func cmdLEADERCURRENT(ctx *Context) error {
	if ctx.ArgCount() != 1 {
		return ctx.WriteError(ErrWrongArgCount)
	}
	e, err := readElection(ctx, ctx.ArgString(0), false)
	if err != nil {
		return ctx.WriteError(err)
	}
	if e == nil || !e.active(ctx.nowMS()) {
		return ctx.WriteNull()
	}
	return ctx.WriteArray([]*resp.Value{
		resp.BulkString("leader"), resp.BulkString(e.leader),
		resp.BulkString("term"), resp.IntegerValue(e.term),
		resp.BulkString("expires_at"), resp.IntegerValue(e.expiresAt),
	})
}

// LEADER.HISTORY name lists the leaders of the last hundred terms, oldest
// first.
func cmdLEADERHISTORY(ctx *Context) error {
	if ctx.ArgCount() != 1 {
		return ctx.WriteError(ErrWrongArgCount)
	}
	e, err := readElection(ctx, ctx.ArgString(0), true)
	if err != nil {
		return ctx.WriteError(err)
	}
	out := []*resp.Value{}
	if e != nil {
		for t := max(1, e.term-leaderHistorySize+1); t <= e.term; t++ {
			if v, ok := e.r.vals[historyField(t)]; ok {
				out = append(out, resp.BulkString(v))
			}
		}
	}
	return ctx.WriteArray(out)
}
