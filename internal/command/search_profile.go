package command

import (
	"bytes"
	"errors"
	"strconv"
	"strings"
	"time"

	"github.com/cachestorm/cachestorm/internal/resp"
	"github.com/cachestorm/cachestorm/internal/search"
)

// cmdFTPROFILE is FT.PROFILE index SEARCH|AGGREGATE [LIMITED] QUERY query
// [args]: the command's reply and RediSearch's profile of it, its iterators
// (with their read counts) and result processors. Times are measured
// here; per-iterator and per-processor times are not, and read 0.
func cmdFTPROFILE(ctx *Context) error {
	if ctx.ArgCount() < 4 {
		return ctx.WriteError(moduleArityError(ctx))
	}
	idx, ok := searchIndexesFor(ctx).Get(ctx.ArgString(0))
	if !ok {
		return ctx.WriteError(errors.New(ctx.ArgString(0) + ": no such index"))
	}
	kind := strings.ToUpper(ctx.ArgString(1))
	if kind != "SEARCH" && kind != "AGGREGATE" {
		return ctx.WriteError(errors.New("No `SEARCH` or `AGGREGATE` provided")) //nolint:staticcheck // RediSearch's reply text
	}
	at := 2
	limited := strings.EqualFold(ctx.ArgString(at), "LIMITED")
	if limited {
		at++
	}
	if at >= ctx.ArgCount() || !strings.EqualFold(ctx.ArgString(at), "QUERY") || at+1 >= ctx.ArgCount() {
		return ctx.WriteError(errors.New("The QUERY keyword is expected")) //nolint:staticcheck // RediSearch's reply text
	}
	for i := at + 2; i < ctx.ArgCount(); i++ {
		if strings.EqualFold(ctx.ArgString(i), "WITHCURSOR") {
			return ctx.WriteError(errors.New("FT.PROFILE does not support cursor"))
		}
	}
	start := time.Now()
	args := append([][]byte{ctx.Arg(0)}, ctx.Args[at+1:]...)
	var out bytes.Buffer
	w := resp.NewWriter(&out)
	resp3 := ctx.Writer.IsRESP3()
	w.SetRESP3(resp3)
	inner := NewContext("FT."+kind, args, ctx.Store, w)
	inner.router = ctx.router
	trace := &aggregateTrace{}
	inner.aggTrace = trace
	handler := cmdFTSEARCH
	if kind == "AGGREGATE" {
		handler = cmdFTAGGREGATE
	}
	if err := handler(inner); err != nil {
		return err
	}
	w.Flush()
	results, err := resp.NewReader(&out).ReadValue()
	if err != nil {
		return err
	}
	if results.Type == resp.TypeError {
		return ctx.WriteValue(results)
	}
	parsed := time.Since(start)
	opts := search.QueryOptions{Params: map[string]string{}}
	var req *searchRequest
	if kind == "SEARCH" {
		if req, err = parseSearchRequest(inner); err == nil {
			opts = req.opts
		}
	} else {
		opts = aggregateQueryOptions(inner)
	}
	// An aggregation's processors pull rows one at a time, so a pager or
	// an error can stop the query's iterators before their end.
	readLimit := -1
	var steps []profileStep
	var counts []int
	if kind == "AGGREGATE" {
		steps = aggregateProfileSteps(inner, idx)
		if hasArg(inner, 2, "ADDSCORES") {
			steps = append([]profileStep{{name: "Scorer"}}, steps...)
		}
		var indexRows int
		counts, indexRows = simulateProcessors(steps, trace)
		if counts[0] < indexRows {
			readLimit = indexRows
		}
	}
	var iters *search.IterProfile
	if query, err := search.ParseQuery(idx, inner.ArgString(1), opts); err == nil {
		iters = idx.Profile(ctx.Store, query, readLimit)
	} else {
		iters = &search.IterProfile{Type: "EMPTY", Estimate: -1}
	}
	matched := iters.Reads
	rows := countReplyRows(results, resp3, req)
	var rps []*resp.Value
	rp := func(name string, n int) {
		rps = append(rps, profileMap(resp3, "Type", resp.SimpleString(name), "Time", profileTime(resp3, 0), "Results processed", resp.IntegerValue(int64(n))))
	}
	if kind == "SEARCH" && req != nil {
		// RediSearch scores unless sorting by a field without WITHSCORES,
		// loads a sort field that is not SORTABLE before sorting, keeps the
		// top offset+num results and pages them when there is an offset.
		rp("Index", matched)
		if req.sortBy == "" && req.limit != 0 || req.withScores {
			rp("Scorer", matched)
		}
		if req.limit == 0 {
			rp("Counter", 1)
		} else {
			if req.sortBy != "" {
				if f := idx.Field(req.sortBy); f == nil || !f.Sortable {
					rp("Loader", matched)
				}
			}
			sorted := min(matched, req.offset+req.limit)
			rp("Sorter", processed(sorted, req.offset > 0 && matched >= req.offset+req.limit))
			if req.offset > 0 {
				rp("Pager/Limiter", min(max(sorted-req.offset, 0), req.limit))
			}
			if !req.noContent {
				rp("Loader", rows)
			}
		}
	} else {
		rp("Index", counts[0])
		for k, step := range steps {
			rp(step.name, counts[k+1])
		}
	}
	total := profileTime(resp3, float64(time.Since(start).Microseconds())/1000)
	shard := profileMap(resp3,
		"Total profile time", total,
		"Parsing time", profileTime(resp3, float64(parsed.Microseconds())/1000),
		"Workers queue time", profileTime(resp3, 0),
		"Pipeline creation time", profileTime(resp3, 0),
		"Total GIL time", total,
		"Warning", resp.ArrayValue([]*resp.Value{resp.SimpleString("None")}),
		"Iterators profile", iterProfileValue(iters, resp3, limited),
		"Result processors profile", resp.ArrayValue(rps))
	profile := profileMap(resp3, "Shards", resp.ArrayValue([]*resp.Value{shard}), "Coordinator", emptyProfileMap(resp3))
	if resp3 {
		return ctx.WriteValue(resp.PairsValue([]*resp.Value{resp.SimpleString("Results"), results, resp.SimpleString("Profile"), profile}))
	}
	return ctx.WriteArray([]*resp.Value{results, profile})
}

func profileMap(resp3 bool, kv ...any) *resp.Value {
	vals := make([]*resp.Value, 0, len(kv))
	for _, x := range kv {
		switch v := x.(type) {
		case string:
			vals = append(vals, resp.SimpleString(v))
		case *resp.Value:
			vals = append(vals, v)
		}
	}
	if resp3 {
		return resp.PairsValue(vals)
	}
	return resp.ArrayValue(vals)
}

func emptyProfileMap(resp3 bool) *resp.Value {
	if resp3 {
		return resp.PairsValue([]*resp.Value{})
	}
	return resp.ArrayValue([]*resp.Value{})
}

func profileTime(resp3 bool, ms float64) *resp.Value {
	if resp3 {
		return resp.DoubleValue(fpconvDtoa(ms))
	}
	return resp.BulkString(fpconvDtoa(ms))
}

func iterProfileValue(p *search.IterProfile, resp3, limited bool) *resp.Value {
	if p.Null {
		return resp.NullBulkString()
	}
	kv := []any{"Type", resp.SimpleString(p.Type)}
	if p.QueryType != "" {
		kv = append(kv, "Query type", resp.SimpleString(p.QueryType))
	}
	if p.HasTerm {
		kv = append(kv, "Term", resp.SimpleString(p.Term))
	}
	kv = append(kv, "Time", profileTime(resp3, 0), "Number of reading operations", resp.IntegerValue(int64(p.Reads)))
	switch {
	case p.Container && p.Type == "UNION" && limited && !strings.HasPrefix(p.QueryType, "UNION"):
		kv = append(kv, "Child iterators", resp.SimpleString("The number of iterators in the union is "+strconv.Itoa(len(p.Children))))
	case p.Container:
		children := make([]*resp.Value, len(p.Children))
		for i, c := range p.Children {
			children[i] = iterProfileValue(c, resp3, limited)
		}
		kv = append(kv, "Child iterators", resp.ArrayValue(children))
	case p.Child != nil:
		kv = append(kv, "Child iterator", iterProfileValue(p.Child, resp3, limited))
	}
	if p.Estimate >= 0 {
		kv = append(kv, "Estimated number of matches", resp.IntegerValue(int64(p.Estimate)))
	}
	return profileMap(resp3, kv...)
}

// countReplyRows counts the rows of an FT.SEARCH or FT.AGGREGATE reply.
func countReplyRows(v *resp.Value, resp3 bool, req *searchRequest) int {
	if resp3 {
		if r := pairsField(v, "results"); r != nil {
			return len(r.Array)
		}
		return 0
	}
	if v.Type != resp.TypeArray || len(v.Array) == 0 {
		return 0
	}
	n := len(v.Array) - 1
	if req == nil {
		return n
	}
	width := 1
	for _, on := range []bool{req.withScores, req.withPayloads, req.withSortKeys, !req.noContent} {
		if on {
			width++
		}
	}
	return n / width
}

// aggregateQueryOptions reads the query options of an FT.AGGREGATE.
func aggregateQueryOptions(ctx *Context) search.QueryOptions {
	opts := search.QueryOptions{Params: map[string]string{}}
	for i := 2; i < ctx.ArgCount(); i++ {
		switch strings.ToUpper(ctx.ArgString(i)) {
		case "VERBATIM":
			opts.Verbatim = true
		case "DIALECT":
			if i+1 < ctx.ArgCount() {
				opts.Dialect = parseDialect(ctx.ArgString(i + 1))
			}
		case "PARAMS":
			if params, last, err := countedArgs(ctx, i); err == nil {
				for j := 0; j+1 < len(params); j += 2 {
					opts.Params[params[j]] = params[j+1]
				}
				i = last
			}
		}
	}
	return opts
}

// aggregateProfileSteps names the result processors of an FT.AGGREGATE's
// steps, in order. A LOAD of SORTABLE fields whose values the index keeps
// as they were needs no loader, and the SORTBY and LIMIT of a segment are
// one sorter, paged when there is an offset.
// aggregateTrace is what FT.AGGREGATE's steps did: the rows the query
// gave, each FILTER's passing input positions and the row it failed on (or
// -1), and each GROUPBY's groups.
type aggregateTrace struct {
	matched int
	filters []filterTrace
	groups  []int
}

type filterTrace struct {
	passed []int
	failAt int
}

const (
	pullOK = iota
	pullEOF
	pullErr
)

// simulateProcessors replays RediSearch's result processor chain, each
// pulling from the one before it, and returns each processor's "Results
// processed" (calls less one, as RediSearch prints them), the Index first,
// and how many rows the Index gave; it reached its end unless it reports
// fewer.
func simulateProcessors(steps []profileStep, tr *aggregateTrace) ([]int, int) {
	calls := make([]int, len(steps)+1)
	indexRows := 0
	type state struct {
		in, out, ready int
		pass           map[int]bool
		failAt         int
		drained        bool
	}
	states := make([]state, len(steps)+1)
	fi, gi := 0, 0
	for k, st := range steps {
		s := &states[k+1]
		s.failAt = -1
		switch {
		case strings.HasPrefix(st.name, "Filter"):
			if fi < len(tr.filters) {
				s.pass = make(map[int]bool, len(tr.filters[fi].passed))
				for _, p := range tr.filters[fi].passed {
					s.pass[p] = true
				}
				s.failAt = tr.filters[fi].failAt
				fi++
			}
		case st.name == "Grouper":
			if gi < len(tr.groups) {
				s.ready = tr.groups[gi]
				gi++
			}
		}
	}
	var next func(k int) int
	next = func(k int) int {
		calls[k]++
		if k == 0 {
			if indexRows < tr.matched {
				indexRows++
				return pullOK
			}
			return pullEOF
		}
		st, s := steps[k-1], &states[k]
		switch {
		case strings.HasPrefix(st.name, "Filter"):
			for {
				if rc := next(k - 1); rc != pullOK {
					return rc
				}
				pos := s.in
				s.in++
				if pos == s.failAt {
					return pullErr
				}
				if s.pass == nil || s.pass[pos] {
					return pullOK
				}
			}
		case st.name == "Pager/Limiter":
			for s.in < st.offset {
				if rc := next(k - 1); rc != pullOK {
					return rc
				}
				s.in++
			}
			if s.in >= st.offset+st.limit {
				return pullEOF
			}
			s.in++
			return next(k - 1)
		case st.name == "Sorter" || st.name == "Grouper":
			if !s.drained {
				n := 0
				for {
					rc := next(k - 1)
					if rc == pullErr {
						return rc
					}
					if rc == pullEOF {
						break
					}
					n++
				}
				s.drained = true
				if st.name == "Sorter" {
					keep := st.offset + st.limit
					if keep == 0 {
						keep = 10
					}
					s.ready = min(n, keep)
				}
			}
			if s.out < s.ready {
				s.out++
				return pullOK
			}
			return pullEOF
		}
		return next(k - 1)
	}
	for next(len(steps)) == pullOK {
	}
	out := make([]int, len(calls))
	for k, c := range calls {
		out[k] = max(c-1, 0)
	}
	return out, indexRows
}

type profileStep struct {
	name          string
	offset, limit int
}

// processed is a result processor's "Results processed": what it yielded,
// less one when the processor after it stopped asking before its end, as
// RediSearch counts the calls that returned a result.
func processed(n int, stoppedEarly bool) int {
	if stoppedEarly && n > 0 {
		return n - 1
	}
	return n
}

func aggregateProfileSteps(ctx *Context, idx *search.Index) []profileStep {
	var out []profileStep
	arranged, first, all := false, true, false
	loaded := map[string]bool{}
	// A GROUPBY or SORTBY on the documents' own fields loads those neither
	// loaded nor sortable first.
	implicitLoad := func(fields []string) {
		if !first || all {
			return
		}
		need := false
		for _, f := range fields {
			if !strings.HasPrefix(f, "@") {
				continue
			}
			name := f[1:]
			if loaded[name] {
				continue
			}
			if sf := idx.Field(name); sf != nil && sf.Sortable {
				continue
			}
			loaded[name] = true
			need = true
		}
		if need {
			out = append(out, profileStep{name: "Loader"})
		}
	}
	for i := 2; i < ctx.ArgCount(); i++ {
		word := strings.ToUpper(ctx.ArgString(i))
		switch word {
		case "LOAD":
			if i+1 < ctx.ArgCount() && ctx.ArgString(i+1) == "*" {
				out = append(out, profileStep{name: "Loader"})
				all = true
				break
			}
			if fields, _, err := countedArgs(ctx, i); err == nil {
				step := false
				for j := 0; j < len(fields); j++ {
					name := strings.TrimPrefix(fields[j], "@")
					as := name
					if j+2 < len(fields) && strings.EqualFold(fields[j+1], "AS") {
						as = fields[j+2]
						j += 2
					}
					if loaded[as] {
						continue
					}
					loaded[as] = true
					f := idx.Field(name)
					if f == nil || !f.Sortable || !f.UNF && f.Type != search.FieldNumeric {
						step = true
					}
				}
				if step {
					out = append(out, profileStep{name: "Loader"})
				}
			}
		case "GROUPBY":
			if props, _, err := countedArgs(ctx, i); err == nil {
				implicitLoad(props)
			}
			out = append(out, profileStep{name: "Grouper"})
			arranged, first = false, false
		case "SORTBY", "LIMIT":
			if arranged {
				break
			}
			arranged = true
			if arr, err := scanArrange(ctx, i); err == nil {
				paged := arr.offset > 0 || arr.limit > 0 && !arr.sorted
				if arr.sorted {
					if word == "SORTBY" {
						if keys, _, err := countedArgs(ctx, i); err == nil {
							implicitLoad(keys)
						}
					}
					out = append(out, profileStep{name: "Sorter", offset: arr.offset, limit: arr.limit})
				}
				if paged {
					out = append(out, profileStep{name: "Pager/Limiter", offset: arr.offset, limit: arr.limit})
				}
			}
		case "APPLY", "FILTER":
			name := "Projector"
			if word == "FILTER" {
				name = "Filter"
			} else if i+3 < ctx.ArgCount() {
				loaded[ctx.ArgString(i+3)] = true
			}
			if i+1 < ctx.ArgCount() {
				if e, err := search.ParseExpr(ctx.ArgString(i + 1)); err == nil {
					name += " - " + search.ProfileExprType(e)
				}
			}
			out = append(out, profileStep{name: name})
		}
		i = profileStepEnd(ctx, i)
	}
	return out
}

// profileStepEnd is aggregateStepEnd for every FT.AGGREGATE step.
func profileStepEnd(ctx *Context, i int) int {
	n := ctx.ArgCount()
	switch strings.ToUpper(ctx.ArgString(i)) {
	case "SORTBY":
		if _, last, err := countedArgs(ctx, i); err == nil {
			i = last
			if i+2 < n && strings.EqualFold(ctx.ArgString(i+1), "MAX") {
				i += 2
			}
		}
		return i
	case "LIMIT":
		return i + 2
	case "FILTER":
		return i + 1
	case "GROUPBY":
		_, last, err := countedArgs(ctx, i)
		if err != nil {
			return i
		}
		i = last
		for i+2 < n && strings.EqualFold(ctx.ArgString(i+1), "REDUCE") {
			if _, last, err = countedArgs(ctx, i+2); err != nil {
				return i
			}
			i = last
			if i+2 < n && strings.EqualFold(ctx.ArgString(i+1), "AS") {
				i += 2
			}
		}
		return i
	}
	return aggregateStepEnd(ctx, i)
}
