package command

import (
	"errors"
	"sort"
	"strconv"
	"strings"

	"github.com/cachestorm/cachestorm/internal/resp"
	"github.com/cachestorm/cachestorm/internal/search"
	"github.com/cachestorm/cachestorm/internal/store"
)

// The FT commands follow RediSearch, the search module built into Redis 8:
// an index covers the HASH or JSON keys under its prefixes and queries read
// those keys as they run. Replies were checked against Redis 8.2. FT.ADD,
// FT.DEL and FT.GET, which RediSearch 2 dropped, remain for old AOF files:
// FT.ADD writes the document as a hash.

func RegisterSearchCommands(router *Router) {
	for name, handler := range map[string]func(*Context) error{
		"FT.CREATE": cmdFTCREATE, "FT.DROPINDEX": cmdFTDROPINDEX, "FT.INFO": cmdFTINFO,
		"FT.SEARCH": cmdFTSEARCH, "FT.AGGREGATE": cmdFTAGGREGATE, "FT._LIST": cmdFTLIST,
		"FT.TAGVALS": cmdFTTAGVALS, "FT.ALIASADD": cmdFTALIASADD, "FT.ALIASDEL": cmdFTALIASDEL,
		"FT.ALIASUPDATE": cmdFTALIASUPDATE, "FT.EXPLAIN": cmdFTEXPLAIN,
		"FT.ADD": cmdFTADD, "FT.DEL": cmdFTDEL, "FT.GET": cmdFTGET,
	} {
		router.Register(&CommandDef{Name: name, Handler: handler})
	}
}

func searchIndexesFor(ctx *Context) *search.IndexManager {
	if r := ctx.dispatcher(); r != nil {
		if m := r.SearchIndexes(); m != nil {
			return m
		}
	}
	return search.GetIndexManager()
}

func errNoSuchIndex(name string) error { return errors.New("No such index " + name) }

func cmdFTCREATE(ctx *Context) error {
	if ctx.ArgCount() < 2 {
		return ctx.WriteError(arityError(ctx.Command))
	}
	args := make([]string, ctx.ArgCount()-1)
	for i := range args {
		args[i] = ctx.ArgString(i + 1)
	}
	idx, err := search.ParseIndex(ctx.ArgString(0), args)
	if err != nil {
		return ctx.WriteError(err)
	}
	if err := searchIndexesFor(ctx).Create(idx); err != nil {
		return ctx.WriteError(err)
	}
	return ctx.WriteOK()
}

// indexDocuments returns the documents of idx in the current database that
// still hold, by key name; keys limits them when not nil.
func indexDocuments(ctx *Context, idx *search.Index, keys map[string]bool) []*search.Document {
	all, _ := idx.Documents(ctx.Store)
	docs := make([]*search.Document, 0, len(all))
	for _, d := range all {
		if keys != nil && !keys[d.Key] {
			continue
		}
		if d = idx.Fresh(ctx.Store, d); d != nil {
			docs = append(docs, d)
		}
	}
	sort.Slice(docs, func(i, j int) bool { return docs[i].Key < docs[j].Key })
	return docs
}

func cmdFTDROPINDEX(ctx *Context) error {
	if ctx.ArgCount() < 1 || ctx.ArgCount() > 2 {
		return ctx.WriteError(arityError(ctx.Command))
	}
	manager := searchIndexesFor(ctx)
	idx, ok := manager.Get(ctx.ArgString(0))
	if !ok {
		return ctx.WriteError(errors.New(ctx.ArgString(0) + ": no such index"))
	}
	if ctx.ArgCount() == 2 {
		if !strings.EqualFold(ctx.ArgString(1), "DD") {
			return ctx.WriteError(ErrSyntaxError)
		}
		for _, doc := range indexDocuments(ctx, idx, nil) {
			ctx.Store.Delete(doc.Key)
		}
	}
	manager.Drop(idx.Name)
	return ctx.WriteOK()
}

func cmdFTLIST(ctx *Context) error {
	names := searchIndexesFor(ctx).List()
	out := make([]*resp.Value, len(names))
	for i, n := range names {
		out[i] = resp.SimpleString(n)
	}
	return ctx.WriteArray(out)
}

func cmdFTINFO(ctx *Context) error {
	if ctx.ArgCount() != 1 {
		return ctx.WriteError(arityError(ctx.Command))
	}
	idx, ok := searchIndexesFor(ctx).Get(ctx.ArgString(0))
	if !ok {
		return ctx.WriteError(search.ErrUnknownIndex)
	}
	keyType := "HASH"
	if idx.JSON {
		keyType = "JSON"
	}
	prefixes := make([]*resp.Value, len(idx.Prefixes))
	for i, p := range idx.Prefixes {
		prefixes[i] = resp.SimpleString(p)
	}
	attrs := make([]*resp.Value, 0, len(idx.Fields))
	for _, f := range idx.Fields {
		a := []*resp.Value{resp.SimpleString("identifier"), resp.SimpleString(f.Identifier),
			resp.SimpleString("attribute"), resp.SimpleString(f.Name), resp.SimpleString("type"), resp.SimpleString(f.Type.String())}
		switch f.Type {
		case search.FieldText:
			a = append(a, resp.SimpleString("WEIGHT"), resp.BulkString(search.FormatNumber(f.Weight)))
			if f.NoStem {
				a = append(a, resp.SimpleString("NOSTEM"))
			}
		case search.FieldTag:
			a = append(a, resp.SimpleString("SEPARATOR"), resp.BulkString(string(f.Separator)))
			if f.CaseSensitive {
				a = append(a, resp.SimpleString("CASESENSITIVE"))
			}
		}
		if f.Sortable {
			a = append(a, resp.SimpleString("SORTABLE"))
		}
		if f.NoIndex {
			a = append(a, resp.SimpleString("NOINDEX"))
		}
		attrs = append(attrs, resp.ArrayValue(a))
	}
	docs, terms, records := idx.Summary(ctx.Store)
	return ctx.WriteArray([]*resp.Value{
		resp.SimpleString("index_name"), resp.SimpleString(idx.Name),
		resp.SimpleString("index_options"), resp.ArrayValue([]*resp.Value{}),
		resp.SimpleString("index_definition"), resp.ArrayValue([]*resp.Value{
			resp.SimpleString("key_type"), resp.SimpleString(keyType),
			resp.SimpleString("prefixes"), resp.ArrayValue(prefixes),
			resp.SimpleString("default_score"), resp.BulkString(search.FormatNumber(idx.Score)),
		}),
		resp.SimpleString("attributes"), resp.ArrayValue(attrs),
		resp.SimpleString("num_docs"), resp.IntegerValue(int64(docs)),
		resp.SimpleString("max_doc_id"), resp.IntegerValue(int64(docs)),
		resp.SimpleString("num_terms"), resp.IntegerValue(int64(terms)),
		resp.SimpleString("num_records"), resp.IntegerValue(int64(records)),
		resp.SimpleString("indexing"), resp.IntegerValue(0),
		resp.SimpleString("percent_indexed"), resp.BulkString("1"),
		resp.SimpleString("hash_indexing_failures"), resp.IntegerValue(0),
	})
}

// countedArgs reads the n arguments after a count at args[i]; it returns
// them and the index of the last one.
func countedArgs(ctx *Context, i int) ([]string, int, error) {
	if i+1 >= ctx.ArgCount() {
		return nil, i, errors.New("Bad arguments for " + strings.ToUpper(ctx.ArgString(i)) + ": Expected an argument, but none provided") //nolint:staticcheck // RediSearch's reply text
	}
	n, err := strconv.Atoi(ctx.ArgString(i + 1))
	if err != nil || n < 0 || i+1+n >= ctx.ArgCount() {
		return nil, i, errors.New("Bad arguments for " + strings.ToUpper(ctx.ArgString(i)) + ": Expected an argument, but none provided") //nolint:staticcheck // RediSearch's reply text
	}
	out := make([]string, n)
	for j := range out {
		out[j] = ctx.ArgString(i + 2 + j)
	}
	return out, i + 1 + n, nil
}

type searchRequest struct {
	noContent, withScores, withSortKeys bool
	opts                                search.QueryOptions
	filters                             []search.Node
	inKeys                              map[string]bool
	returns                             [][2]string
	returnSet                           bool
	sortBy                              string
	sortDesc                            bool
	offset, limit                       int
}

func parseSearchRequest(ctx *Context) (*searchRequest, error) {
	r := &searchRequest{limit: 10, opts: search.QueryOptions{Params: map[string]string{}}}
	n := ctx.ArgCount()
	for i := 2; i < n; i++ {
		switch strings.ToUpper(ctx.ArgString(i)) {
		case "NOCONTENT":
			r.noContent = true
		case "VERBATIM":
			r.opts.Verbatim = true
		case "NOSTOPWORDS":
			r.opts.NoStopwords = true
		case "WITHSCORES":
			r.withScores = true
		case "WITHSORTKEYS":
			r.withSortKeys = true
		case "WITHPAYLOADS", "INORDER", "EXPLAINSCORE":
		case "FILTER":
			if i+3 >= n {
				return nil, errors.New("Bad arguments for FILTER: Expected an argument, but none provided") //nolint:staticcheck // RediSearch's reply text
			}
			f, err := search.NumericFilter(ctx.ArgString(i+1), ctx.ArgString(i+2), ctx.ArgString(i+3))
			if err != nil {
				return nil, err
			}
			r.filters = append(r.filters, f)
			i += 3
		case "GEOFILTER":
			if i+5 >= n {
				return nil, errors.New("Bad arguments for GEOFILTER: Expected an argument, but none provided") //nolint:staticcheck // RediSearch's reply text
			}
			f, err := search.GeoFilter(ctx.ArgString(i+1), ctx.ArgString(i+2), ctx.ArgString(i+3), ctx.ArgString(i+4), ctx.ArgString(i+5))
			if err != nil {
				return nil, err
			}
			r.filters = append(r.filters, f)
			i += 5
		case "INKEYS":
			keys, last, err := countedArgs(ctx, i)
			if err != nil {
				return nil, err
			}
			r.inKeys = make(map[string]bool)
			for _, k := range keys {
				r.inKeys[k] = true
			}
			i = last
		case "INFIELDS":
			fields, last, err := countedArgs(ctx, i)
			if err != nil {
				return nil, err
			}
			r.opts.InFields, i = fields, last
		case "RETURN":
			fields, last, err := countedArgs(ctx, i)
			if err != nil {
				return nil, err
			}
			r.returnSet, i = true, last
			for j := 0; j < len(fields); j++ {
				name, as := fields[j], fields[j]
				if j+2 < len(fields) && strings.EqualFold(fields[j+1], "AS") {
					as = fields[j+2]
					j += 2
				}
				r.returns = append(r.returns, [2]string{name, as})
			}
			if len(fields) == 0 {
				r.noContent = true
			}
		case "SORTBY":
			if i+1 >= n {
				return nil, errors.New("Bad arguments for SORTBY: Expected an argument, but none provided") //nolint:staticcheck // RediSearch's reply text
			}
			r.sortBy = strings.TrimPrefix(ctx.ArgString(i+1), "@")
			i++
			if i+1 < n {
				switch strings.ToUpper(ctx.ArgString(i + 1)) {
				case "DESC":
					r.sortDesc = true
					i++
				case "ASC":
					i++
				}
			}
			if i+1 < n && strings.EqualFold(ctx.ArgString(i+1), "WITHCOUNT") {
				i++
			}
		case "LIMIT":
			if i+2 >= n {
				return nil, errors.New("Bad arguments for LIMIT: Expected an argument, but none provided") //nolint:staticcheck // RediSearch's reply text
			}
			off, err1 := strconv.Atoi(ctx.ArgString(i + 1))
			num, err2 := strconv.Atoi(ctx.ArgString(i + 2))
			if err1 != nil || err2 != nil || off < 0 || num < 0 {
				return nil, errors.New("Bad arguments for LIMIT: Value is outside acceptable bounds") //nolint:staticcheck // RediSearch's reply text
			}
			r.offset, r.limit = off, num
			i += 2
		case "PARAMS":
			params, last, err := countedArgs(ctx, i)
			if err != nil {
				return nil, err
			}
			for j := 0; j+1 < len(params); j += 2 {
				r.opts.Params[params[j]] = params[j+1]
			}
			i = last
		case "SUMMARIZE", "HIGHLIGHT":
			for more := true; more && i+1 < n; {
				switch strings.ToUpper(ctx.ArgString(i + 1)) {
				case "FIELDS":
					_, last, err := countedArgs(ctx, i+1)
					if err != nil {
						return nil, err
					}
					i = last
				case "FRAGS", "LEN", "SEPARATOR":
					i += 2
				case "TAGS":
					i += 3
				default:
					more = false
				}
			}
		case "SLOP", "TIMEOUT", "LANGUAGE", "EXPANDER", "SCORER", "PAYLOAD", "DIALECT":
			i++
		}
	}
	return r, nil
}

func cmdFTSEARCH(ctx *Context) error {
	if ctx.ArgCount() < 2 {
		return ctx.WriteError(arityError(ctx.Command))
	}
	idx, ok := searchIndexesFor(ctx).Get(ctx.ArgString(0))
	if !ok {
		return ctx.WriteError(errNoSuchIndex(ctx.ArgString(0)))
	}
	req, err := parseSearchRequest(ctx)
	if err != nil {
		return ctx.WriteError(err)
	}
	query, err := search.ParseQuery(idx, ctx.ArgString(1), req.opts)
	if err != nil {
		return ctx.WriteError(err)
	}
	if len(req.filters) > 0 {
		query = search.And(append([]search.Node{query}, req.filters...)...)
	}
	type hit struct {
		doc     *search.Document
		score   float64
		sortKey string
		sortNum float64
		sortOK  bool
		isNum   bool
	}
	var hits []hit
	for _, h := range idx.Search(ctx.Store, query, req.opts) {
		if req.inKeys == nil || req.inKeys[h.Doc.Key] {
			hits = append(hits, hit{doc: h.Doc, score: h.Score})
		}
	}
	if req.sortBy != "" {
		for i := range hits {
			h := &hits[i]
			h.sortKey, h.sortOK = search.FieldValue(idx, h.doc, req.sortBy)
			if !h.sortOK {
				// Only SORTABLE fields are kept in the index; others are
				// read from the key.
				if full := idx.Content(ctx.Store, h.doc.Key); full != nil {
					h.sortKey, h.sortOK = search.FieldValue(idx, full, req.sortBy)
				}
			}
			if f, err := strconv.ParseFloat(h.sortKey, 64); err == nil {
				h.sortNum, h.isNum = f, true
			}
		}
		sort.Slice(hits, func(i, j int) bool {
			a, b := &hits[i], &hits[j]
			if a.sortOK != b.sortOK {
				return a.sortOK
			}
			c := 0
			switch {
			case a.isNum && b.isNum:
				if a.sortNum < b.sortNum {
					c = -1
				} else if a.sortNum > b.sortNum {
					c = 1
				}
			default:
				c = strings.Compare(a.sortKey, b.sortKey)
			}
			if c == 0 {
				return a.doc.Key < b.doc.Key
			}
			if req.sortDesc {
				return c > 0
			}
			return c < 0
		})
	} else {
		sort.Slice(hits, func(i, j int) bool {
			if hits[i].score != hits[j].score {
				return hits[i].score > hits[j].score
			}
			return hits[i].doc.Key < hits[j].doc.Key
		})
	}
	out := []*resp.Value{resp.IntegerValue(int64(len(hits)))}
	for i := req.offset; i < len(hits) && i < req.offset+req.limit; i++ {
		h := hits[i]
		// The documents returned are read again if their key changed in a
		// way no command reported.
		if h.doc = idx.Content(ctx.Store, h.doc.Key); h.doc == nil {
			continue
		}
		out = append(out, resp.BulkString(h.doc.Key))
		if req.withScores {
			out = append(out, resp.BulkString(strconv.FormatFloat(h.score, 'g', -1, 64)))
		}
		if req.withSortKeys {
			out = append(out, sortKeyReply(idx, h.doc, req.sortBy))
		}
		if !req.noContent {
			out = append(out, documentReply(idx, h.doc, req))
		}
	}
	return ctx.WriteArray(out)
}

func sortKeyReply(idx *search.Index, doc *search.Document, field string) *resp.Value {
	v, ok := search.FieldValue(idx, doc, field)
	if field == "" || !ok {
		return resp.NullBulkString()
	}
	if f, err := strconv.ParseFloat(v, 64); err == nil {
		return resp.BulkString("#" + search.FormatNumber(f))
	}
	return resp.BulkString("$" + strings.ToLower(v))
}

// documentReply is a result's fields: RETURN's, or every hash field (the
// SORTBY field first) or the JSON document as "$".
func documentReply(idx *search.Index, doc *search.Document, req *searchRequest) *resp.Value {
	var out []*resp.Value
	switch {
	case req.returnSet:
		for _, r := range req.returns {
			if v, ok := search.FieldValue(idx, doc, r[0]); ok {
				out = append(out, resp.BulkString(r[1]), resp.BulkString(v))
			}
		}
	case doc.JSON != nil:
		out = append(out, resp.BulkString("$"), resp.BulkBytes(store.AppendJSON(nil, doc.JSON)))
	default:
		first := ""
		if req.sortBy != "" {
			if f := idx.Field(req.sortBy); f != nil {
				first = f.Identifier
			} else {
				first = req.sortBy
			}
			if v, ok := doc.HashField(first); ok {
				out = append(out, resp.BulkString(first), resp.BulkString(v))
			} else {
				first = ""
			}
		}
		for _, kv := range doc.Fields {
			if kv[0] != first {
				out = append(out, resp.BulkString(kv[0]), resp.BulkString(kv[1]))
			}
		}
	}
	return resp.ArrayValue(out)
}

func cmdFTAGGREGATE(ctx *Context) error {
	if ctx.ArgCount() < 2 {
		return ctx.WriteError(arityError(ctx.Command))
	}
	idx, ok := searchIndexesFor(ctx).Get(ctx.ArgString(0))
	if !ok {
		return ctx.WriteError(errNoSuchIndex(ctx.ArgString(0)))
	}
	opts := search.QueryOptions{Params: map[string]string{}}
	n := ctx.ArgCount()
	for i := 2; i < n; i++ {
		switch strings.ToUpper(ctx.ArgString(i)) {
		case "VERBATIM":
			opts.Verbatim = true
		case "PARAMS":
			params, last, err := countedArgs(ctx, i)
			if err != nil {
				return ctx.WriteError(err)
			}
			for j := 0; j+1 < len(params); j += 2 {
				opts.Params[params[j]] = params[j+1]
			}
			i = last
		}
	}
	query, err := search.ParseQuery(idx, ctx.ArgString(1), opts)
	if err != nil {
		return ctx.WriteError(err)
	}
	found := idx.Search(ctx.Store, query, opts)
	sort.Slice(found, func(i, j int) bool { return found[i].Doc.Key < found[j].Doc.Key })
	var rows []*search.Row
	for _, h := range found {
		if d := idx.Content(ctx.Store, h.Doc.Key); d != nil {
			rows = append(rows, idx.NewRow(d))
		}
	}
	offset, limit := 0, -1
	for i := 2; i < n; i++ {
		word := strings.ToUpper(ctx.ArgString(i))
		switch word {
		case "LOAD":
			if i+1 < n && ctx.ArgString(i+1) == "*" {
				for _, r := range rows {
					r.Load([]string{"*"})
				}
				i++
				continue
			}
			fields, last, err := countedArgs(ctx, i)
			if err != nil {
				return ctx.WriteError(err)
			}
			for _, r := range rows {
				r.Load(fields)
			}
			i = last
		case "GROUPBY":
			props, last, err := countedArgs(ctx, i)
			if err != nil {
				return ctx.WriteError(err)
			}
			i = last
			var reducers []search.Reducer
			for i+1 < n && strings.EqualFold(ctx.ArgString(i+1), "REDUCE") {
				if i+2 >= n {
					return ctx.WriteError(errors.New("Bad arguments for REDUCE: Expected an argument, but none provided")) //nolint:staticcheck // RediSearch's reply text
				}
				fn := ctx.ArgString(i + 2)
				args, last, err := countedArgs(ctx, i+2)
				if err != nil {
					return ctx.WriteError(err)
				}
				red := search.Reducer{Fn: fn, Args: args}
				i = last
				if i+2 < n && strings.EqualFold(ctx.ArgString(i+1), "AS") {
					red.As = ctx.ArgString(i + 2)
					i += 2
				}
				reducers = append(reducers, red)
			}
			if rows, err = search.Group(rows, props, reducers); err != nil {
				return ctx.WriteError(err)
			}
		case "SORTBY":
			props, last, err := countedArgs(ctx, i)
			if err != nil {
				return ctx.WriteError(err)
			}
			i = last
			var keys []search.SortKey
			for j := 0; j < len(props); j++ {
				k := search.SortKey{Field: strings.TrimPrefix(props[j], "@")}
				if j+1 < len(props) && (strings.EqualFold(props[j+1], "DESC") || strings.EqualFold(props[j+1], "ASC")) {
					k.Desc = strings.EqualFold(props[j+1], "DESC")
					j++
				}
				keys = append(keys, k)
			}
			search.SortRows(rows, keys)
			if i+2 < n && strings.EqualFold(ctx.ArgString(i+1), "MAX") {
				if m, err := strconv.Atoi(ctx.ArgString(i + 2)); err == nil && m < len(rows) {
					rows = rows[:m]
				}
				i += 2
			}
		case "APPLY":
			if i+3 >= n || !strings.EqualFold(ctx.ArgString(i+2), "AS") {
				return ctx.WriteError(errors.New("Bad arguments for APPLY: AS is required")) //nolint:staticcheck // RediSearch's reply text
			}
			expr, err := search.ParseExpr(ctx.ArgString(i + 1))
			if err != nil {
				return ctx.WriteError(err)
			}
			name := ctx.ArgString(i + 3)
			for _, r := range rows {
				r.Set(name, expr.Eval(r))
			}
			i += 3
		case "FILTER":
			if i+1 >= n {
				return ctx.WriteError(errors.New("Bad arguments for FILTER: Expected an argument, but none provided")) //nolint:staticcheck // RediSearch's reply text
			}
			expr, err := search.ParseExpr(ctx.ArgString(i + 1))
			if err != nil {
				return ctx.WriteError(err)
			}
			kept := rows[:0]
			for _, r := range rows {
				if expr.Truthy(r) {
					kept = append(kept, r)
				}
			}
			rows = kept
			i++
		case "LIMIT":
			if i+2 >= n {
				return ctx.WriteError(errors.New("Bad arguments for LIMIT: Expected an argument, but none provided")) //nolint:staticcheck // RediSearch's reply text
			}
			off, err1 := strconv.Atoi(ctx.ArgString(i + 1))
			num, err2 := strconv.Atoi(ctx.ArgString(i + 2))
			if err1 != nil || err2 != nil || off < 0 || num < 0 {
				return ctx.WriteError(errors.New("Bad arguments for LIMIT: Value is outside acceptable bounds")) //nolint:staticcheck // RediSearch's reply text
			}
			offset, limit = off, num
			i += 2
		case "PARAMS":
			_, last, err := countedArgs(ctx, i)
			if err != nil {
				return ctx.WriteError(err)
			}
			i = last
		case "TIMEOUT", "DIALECT":
			i++
		}
	}
	out := []*resp.Value{resp.IntegerValue(int64(len(rows)))}
	for i := offset; i < len(rows) && (limit < 0 || i < offset+limit); i++ {
		r := rows[i]
		fields := make([]*resp.Value, 0, 2*len(r.Names))
		for j, name := range r.Names {
			v := r.Values[j]
			if strings.Contains(v, "\x00") || strings.HasPrefix(name, "__generated_aliastolist") {
				list := []*resp.Value{}
				if v != "" {
					for _, item := range strings.Split(v, "\x00") {
						list = append(list, resp.BulkString(item))
					}
				}
				fields = append(fields, resp.BulkString(name), resp.ArrayValue(list))
				continue
			}
			fields = append(fields, resp.BulkString(name), resp.BulkString(v))
		}
		out = append(out, resp.ArrayValue(fields))
	}
	return ctx.WriteArray(out)
}

func cmdFTTAGVALS(ctx *Context) error {
	if ctx.ArgCount() != 2 {
		return ctx.WriteError(arityError(ctx.Command))
	}
	idx, ok := searchIndexesFor(ctx).Get(ctx.ArgString(0))
	if !ok {
		return ctx.WriteError(search.ErrUnknownIndex)
	}
	f := idx.Field(ctx.ArgString(1))
	if f == nil || f.Type != search.FieldTag {
		return ctx.WriteError(errors.New("No such field `" + ctx.ArgString(1) + "`")) //nolint:staticcheck // RediSearch's reply text
	}
	tags := idx.TagValues(ctx.Store, f)
	out := make([]*resp.Value, len(tags))
	for i, t := range tags {
		out[i] = resp.BulkString(t)
	}
	return ctx.WriteArray(out)
}

func cmdFTALIASADD(ctx *Context) error {
	if ctx.ArgCount() != 2 {
		return ctx.WriteError(arityError(ctx.Command))
	}
	if err := searchIndexesFor(ctx).AddAlias(ctx.ArgString(0), ctx.ArgString(1), false); err != nil {
		return ctx.WriteError(err)
	}
	return ctx.WriteOK()
}

func cmdFTALIASUPDATE(ctx *Context) error {
	if ctx.ArgCount() != 2 {
		return ctx.WriteError(arityError(ctx.Command))
	}
	if err := searchIndexesFor(ctx).AddAlias(ctx.ArgString(0), ctx.ArgString(1), true); err != nil {
		return ctx.WriteError(err)
	}
	return ctx.WriteOK()
}

func cmdFTALIASDEL(ctx *Context) error {
	if ctx.ArgCount() != 1 {
		return ctx.WriteError(arityError(ctx.Command))
	}
	if err := searchIndexesFor(ctx).DelAlias(ctx.ArgString(0)); err != nil {
		return ctx.WriteError(err)
	}
	return ctx.WriteOK()
}

func cmdFTEXPLAIN(ctx *Context) error {
	if ctx.ArgCount() < 2 {
		return ctx.WriteError(arityError(ctx.Command))
	}
	idx, ok := searchIndexesFor(ctx).Get(ctx.ArgString(0))
	if !ok {
		return ctx.WriteError(errNoSuchIndex(ctx.ArgString(0)))
	}
	if _, err := search.ParseQuery(idx, ctx.ArgString(1), search.QueryOptions{}); err != nil {
		return ctx.WriteError(err)
	}
	return ctx.WriteBulkString(ctx.ArgString(1) + "\n")
}

// cmdFTADD is RediSearch 1's FT.ADD index docId score [REPLACE [PARTIAL]]
// [NOSAVE] [LANGUAGE l] [PAYLOAD p] FIELDS field value ...: the document is
// written as a hash at docId.
func cmdFTADD(ctx *Context) error {
	if ctx.ArgCount() < 3 {
		return ctx.WriteError(arityError(ctx.Command))
	}
	if _, ok := searchIndexesFor(ctx).Get(ctx.ArgString(0)); !ok {
		return ctx.WriteError(search.ErrUnknownIndex)
	}
	score, err := strconv.ParseFloat(ctx.ArgString(2), 64)
	if err != nil || score < 0 || score > 1 {
		return ctx.WriteError(errors.New("ERR Score must be between 0 and 1"))
	}
	key := ctx.ArgString(1)
	replace, partial := false, false
	fieldsAt := -1
	for i := 3; i < ctx.ArgCount() && fieldsAt < 0; i++ {
		switch strings.ToUpper(ctx.ArgString(i)) {
		case "REPLACE":
			replace = true
		case "PARTIAL":
			partial = true
		case "LANGUAGE", "PAYLOAD":
			i++
		case "FIELDS":
			fieldsAt = i + 1
		}
	}
	if fieldsAt < 0 || (ctx.ArgCount()-fieldsAt)%2 != 0 {
		return ctx.WriteError(arityError(ctx.Command))
	}
	if ctx.Store.Exists(key) {
		if !replace {
			return ctx.WriteError(errors.New("Document already exists")) //nolint:staticcheck // RediSearch's reply text
		}
		if !partial {
			ctx.Store.Delete(key)
		}
	}
	if _, err := ctx.Store.ApplyHashPairs(key, false, ctx.Args[fieldsAt:]); err != nil {
		return ctx.WriteError(err)
	}
	touchSearchKey(ctx, key)
	return ctx.WriteOK()
}

func cmdFTDEL(ctx *Context) error {
	if ctx.ArgCount() < 2 || ctx.ArgCount() > 3 {
		return ctx.WriteError(arityError(ctx.Command))
	}
	if _, ok := searchIndexesFor(ctx).Get(ctx.ArgString(0)); !ok {
		return ctx.WriteError(search.ErrUnknownIndex)
	}
	if ctx.Store.Delete(ctx.ArgString(1)) {
		touchSearchKey(ctx, ctx.ArgString(1))
		return ctx.WriteInteger(1)
	}
	return ctx.WriteInteger(0)
}

func cmdFTGET(ctx *Context) error {
	if ctx.ArgCount() != 2 {
		return ctx.WriteError(arityError(ctx.Command))
	}
	idx, ok := searchIndexesFor(ctx).Get(ctx.ArgString(0))
	if !ok {
		return ctx.WriteError(search.ErrUnknownIndex)
	}
	entry, exists := ctx.Store.Get(ctx.ArgString(1))
	if !exists {
		return ctx.WriteNullBulkString()
	}
	doc, ok := idx.Load(ctx.ArgString(1), entry, true)
	if !ok {
		return ctx.WriteNullBulkString()
	}
	return ctx.WriteValue(documentReply(idx, doc, &searchRequest{}))
}

// touchSearchKey marks key changed for every index of the server.
func touchSearchKey(ctx *Context, key string) {
	for _, idx := range searchIndexesFor(ctx).All() {
		idx.Touch(ctx.Store, key)
	}
}

// TouchSearchKey marks a key the store removed (expired, evicted) as
// changed for the router's indexes.
func (r *Router) TouchSearchKey(st *store.Store, key string) {
	m := r.SearchIndexes()
	if m == nil {
		m = search.GetIndexManager()
	}
	for _, idx := range m.All() {
		idx.Touch(st, key)
	}
}
