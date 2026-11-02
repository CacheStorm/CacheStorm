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
	if ctx.DB != 0 {
		return ctx.WriteError(errors.New("Cannot create index on db != 0")) //nolint:staticcheck // RediSearch's reply text
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
	idx.Open(ctx.Store)
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
	search.SortByID(docs)
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
	return ctx.WriteValue(resp.SetValue(out))
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

//nolint:staticcheck // RediSearch's reply texts
var (
	errLimitArity  = errors.New("LIMIT requires two arguments")
	errLimitNumber = errors.New("LIMIT needs two numeric arguments")
	errLimitOffset = errors.New("The `offset` of the LIMIT must be 0 when `num` is 0")
)

// parseDialect reads a DIALECT argument; anything but a number is the
// default dialect.
func parseDialect(s string) int {
	if d, err := strconv.Atoi(s); err == nil {
		return d
	}
	return 1
}

func errMissingProperty(name string) error {
	return errors.New("Could not find the value for a parameter name, consider using EXISTS if applicable for " + name) //nolint:staticcheck // RediSearch's reply text
}

// parseLimit reads the offset and count after LIMIT at args[i].
func parseLimit(ctx *Context, i int) (offset, num int, err error) {
	if i+2 >= ctx.ArgCount() {
		return 0, 0, errLimitArity
	}
	off, err1 := strconv.ParseUint(ctx.ArgString(i+1), 10, 31)
	cnt, err2 := strconv.ParseUint(ctx.ArgString(i+2), 10, 31)
	if err1 != nil || err2 != nil {
		return 0, 0, errLimitNumber
	}
	if cnt == 0 && off != 0 {
		return 0, 0, errLimitOffset
	}
	return int(off), int(cnt), nil
}

type searchRequest struct {
	noContent, withScores, withSortKeys bool
	withPayloads                        bool
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
		case "WITHPAYLOADS":
			r.withPayloads = true
		case "INORDER", "EXPLAINSCORE":
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
				if j+1 < len(fields) && strings.EqualFold(fields[j+1], "AS") {
					if j+2 >= len(fields) {
						return nil, errors.New("RETURN path AS name - must be accompanied with NAME") //nolint:staticcheck // RediSearch's reply text
					}
					if as = fields[j+2]; strings.EqualFold(as, "AS") {
						return nil, errors.New("Alias for RETURN cannot be `AS`") //nolint:staticcheck // RediSearch's reply text
					}
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
			off, num, err := parseLimit(ctx, i)
			if err != nil {
				return nil, err
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
		case "DIALECT":
			if i+1 < n {
				r.opts.Dialect = parseDialect(ctx.ArgString(i + 1))
			}
			i++
		case "SLOP", "TIMEOUT", "LANGUAGE", "EXPANDER", "SCORER", "PAYLOAD":
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
				// Ties go by document id, the later first when descending.
				return a.doc.Before(b.doc) != req.sortDesc
			}
			if req.sortDesc {
				return c > 0
			}
			return c < 0
		})
	} else {
		sort.SliceStable(hits, func(i, j int) bool {
			if hits[i].score != hits[j].score {
				return hits[i].score > hits[j].score
			}
			return hits[i].doc.Before(hits[j].doc)
		})
	}
	resp3 := ctx.Writer.IsRESP3()
	var results []*resp.Value
	order := &fieldOrder{}
	for i := req.offset; i < len(hits) && i < req.offset+req.limit; i++ {
		h := hits[i]
		key := h.doc.Key
		// The documents returned are read again if their key changed in a
		// way no command reported.
		if h.doc = idx.Content(ctx.Store, key); h.doc == nil {
			continue
		}
		var fields []*resp.Value
		add := func(name string, v *resp.Value) {
			if resp3 {
				fields = append(fields, resp.SimpleString(name))
			}
			fields = append(fields, v)
		}
		add("id", resp.BulkString(key))
		if req.withScores {
			add("score", resp.DoubleValue(fpconvDtoa(h.score)))
		}
		if req.withPayloads {
			add("payload", resp.NullBulkString())
		}
		if req.withSortKeys {
			add("sortkey", sortKeyReply(idx, h.doc, req.sortBy))
		}
		if !req.noContent {
			doc := documentReply(idx, h.doc, req, order)
			if resp3 {
				doc = resp.PairsValue(doc.Array)
			}
			add("extra_attributes", doc)
		}
		if resp3 {
			add("values", resp.ArrayValue([]*resp.Value{}))
			results = append(results, resp.PairsValue(fields))
		} else {
			results = append(results, fields...)
		}
	}
	return ctx.WriteValue(searchReply(resp3, len(hits), results))
}

// searchReply is FT.SEARCH's and FT.AGGREGATE's reply: the total and the
// results, flat for RESP2 and as RediSearch's map for RESP3.
func searchReply(resp3 bool, total int, results []*resp.Value, warnings ...string) *resp.Value {
	if !resp3 {
		return resp.ArrayValue(append([]*resp.Value{resp.IntegerValue(int64(total))}, results...))
	}
	if results == nil {
		results = []*resp.Value{}
	}
	warn := []*resp.Value{}
	for _, w := range warnings {
		warn = append(warn, resp.SimpleString(w))
	}
	return resp.PairsValue([]*resp.Value{
		resp.SimpleString("attributes"), resp.ArrayValue([]*resp.Value{}),
		resp.SimpleString("format"), resp.SimpleString("STRING"),
		resp.SimpleString("results"), resp.ArrayValue(results),
		resp.SimpleString("total_results"), resp.IntegerValue(int64(total)),
		resp.SimpleString("warning"), resp.ArrayValue(warn),
	})
}

func sortKeyReply(idx *search.Index, doc *search.Document, field string) *resp.Value {
	v, ok := search.FieldValue(idx, doc, field)
	if field == "" || !ok {
		return resp.NullBulkString()
	}
	if f, err := strconv.ParseFloat(v, 64); err == nil {
		return resp.BulkString("#" + strconv.FormatFloat(f, 'g', 17, 64))
	}
	return resp.BulkString("$" + strings.ToLower(v))
}

// fieldOrder is the order a search's replies give fields in: RediSearch
// lists a result's fields in the order the query first met them, the
// SORTBY field first.
type fieldOrder struct {
	names []string
	seen  map[string]bool
}

func (o *fieldOrder) add(name string) {
	if o.seen == nil {
		o.seen = make(map[string]bool)
	}
	if !o.seen[name] {
		o.seen[name] = true
		o.names = append(o.names, name)
	}
}

// documentReply is a result's fields: RETURN's, or every hash field, or
// the JSON document as "$", in the order o keeps.
func documentReply(idx *search.Index, doc *search.Document, req *searchRequest, o *fieldOrder) *resp.Value {
	if o == nil {
		o = &fieldOrder{}
	}
	sortName := ""
	if req.sortBy != "" {
		sortName = req.sortBy
		if f := idx.Field(req.sortBy); f != nil && !idx.JSON {
			sortName = f.Identifier
		}
		o.add(sortName)
	}
	var out []*resp.Value
	switch {
	case req.returnSet:
		values := make(map[string]string, len(req.returns))
		for _, r := range req.returns {
			o.add(r[1])
			if v, ok := search.FieldValue(idx, doc, r[0]); ok {
				values[r[1]] = v
			}
		}
		for _, name := range o.names {
			if v, ok := values[name]; ok {
				out = append(out, resp.BulkString(name), resp.BulkString(v))
			}
		}
	case doc.JSON != nil:
		out = append(out, resp.BulkString("$"), resp.BulkBytes(store.AppendJSON(nil, doc.JSON)))
	default:
		values := make(map[string]string, len(doc.Fields))
		for _, kv := range doc.Fields {
			o.add(kv[0])
			values[kv[0]] = kv[1]
		}
		for _, name := range o.names {
			if v, ok := values[name]; ok {
				out = append(out, resp.BulkString(name), resp.BulkString(v))
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
		case "DIALECT":
			if i+1 < n {
				opts.Dialect = parseDialect(ctx.ArgString(i + 1))
			}
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
	sort.Slice(found, func(i, j int) bool { return found[i].Doc.Before(found[j].Doc) })
	addScores := false
	for i := 2; i < n; i++ {
		if strings.EqualFold(ctx.ArgString(i), "ADDSCORES") {
			addScores = true
		}
	}
	var rows []*search.Row
	// read is how many documents the query had read when a row came from
	// each: RediSearch's total counts the documents read before the first
	// row it sends, unless a GROUPBY or SORTBY read them all first.
	read := make(map[*search.Row]int)
	for i, h := range found {
		if d := idx.Content(ctx.Store, h.Doc.Key); d != nil {
			r := idx.NewRow(d)
			if addScores {
				r.Set("__score", strconv.FormatFloat(h.Score, 'g', 12, 64))
			}
			rows = append(rows, r)
			read[r] = i + 1
		}
	}
	grouped, sorted := false, false
	offset, limit := 0, -1
	// failed holds the rows an APPLY or FILTER could not evaluate. The
	// rows stream through the steps, so an error stops the reply where
	// the row comes, unless a GROUPBY or SORTBY needs every row first.
	failed := make(map[*search.Row]error)
	barrier := func() error {
		for _, r := range rows {
			if err, ok := failed[r]; ok {
				return err
			}
		}
		return nil
	}
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
			if err := barrier(); err != nil {
				return ctx.WriteError(err)
			}
			if rows, err = search.Group(rows, props, reducers); err != nil {
				return ctx.WriteError(err)
			}
			grouped = true
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
			if err := barrier(); err != nil {
				return ctx.WriteError(err)
			}
			search.SortRows(rows, keys)
			sorted = true
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
				if _, ok := failed[r]; ok {
					continue
				}
				if missing, ok := expr.Missing(r); ok {
					failed[r] = errMissingProperty(missing)
					continue
				}
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
				if _, ok := failed[r]; ok {
					kept = append(kept, r)
					continue
				}
				if missing, ok := expr.Missing(r); ok {
					failed[r] = errMissingProperty(missing)
					kept = append(kept, r)
					continue
				}
				if expr.Truthy(r) {
					kept = append(kept, r)
				}
			}
			rows = kept
			i++
		case "LIMIT":
			off, num, err := parseLimit(ctx, i)
			if err != nil {
				return ctx.WriteError(err)
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
	resp3 := ctx.Writer.IsRESP3()
	end := len(rows)
	if limit >= 0 {
		end = min(end, offset+limit)
	}
	var warning error
	for i := 0; i < end; i++ {
		if err, ok := failed[rows[i]]; ok {
			if i <= offset {
				return ctx.WriteError(err)
			}
			end, warning = i, err
			break
		}
	}
	total := len(rows)
	if !grouped {
		// A RESP2 reply sends the total after the first row, a RESP3 one
		// after the last row or the error.
		total = len(found)
		switch {
		case sorted || limit == 0 || offset >= len(rows):
		case !resp3:
			total = read[rows[offset]]
		case warning != nil:
			total = read[rows[end]]
		case limit > 0 && offset+limit <= len(rows):
			total = read[rows[offset+limit-1]]
		}
	}
	results := []*resp.Value{}
	for i := offset; i < end; i++ {
		r := rows[i]
		fields := make([]*resp.Value, 0, 2*len(r.Names))
		for j, name := range r.Names {
			v := r.Values[j]
			if r.IsNull(name) {
				fields = append(fields, resp.BulkString(name), resp.NullBulkString())
				continue
			}
			if r.IsList(name) {
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
		if resp3 {
			results = append(results, resp.PairsValue([]*resp.Value{
				resp.SimpleString("extra_attributes"), resp.PairsValue(fields),
				resp.SimpleString("values"), resp.ArrayValue([]*resp.Value{}),
			}))
		} else {
			results = append(results, resp.ArrayValue(fields))
		}
	}
	var warnings []string
	if warning != nil {
		warnings = append(warnings, warning.Error())
	}
	return ctx.WriteValue(searchReply(resp3, total, results, warnings...))
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
	return ctx.WriteValue(resp.SetValue(out))
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
	opts := search.QueryOptions{Params: map[string]string{}}
	for i := 2; i < ctx.ArgCount(); i++ {
		switch strings.ToUpper(ctx.ArgString(i)) {
		case "DIALECT":
			if i+1 < ctx.ArgCount() {
				opts.Dialect = parseDialect(ctx.ArgString(i + 1))
				i++
			}
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
	return ctx.WriteBulkString(idx.Explain(query))
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
	return ctx.WriteValue(documentReply(idx, doc, &searchRequest{}, nil))
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
