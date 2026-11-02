package command

import (
	"errors"
	"fmt"
	"math"
	"sort"
	"strconv"
	"strings"
	"time"

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
		"FT.ADD": cmdFTADD, "FT.DEL": cmdFTDEL, "FT.GET": cmdFTGET, "FT.ALTER": cmdFTALTER,
		"FT.CURSOR": cmdFTCURSOR, "FT.SYNUPDATE": cmdFTSYNUPDATE, "FT.SYNDUMP": cmdFTSYNDUMP,
		"FT.SYNADD": cmdFTSYNADD, "FT.DICTADD": cmdFTDICTADD, "FT.DICTDEL": cmdFTDICTDEL,
		"FT.DICTDUMP": cmdFTDICTDUMP, "FT.SPELLCHECK": cmdFTSPELLCHECK,
		"FT.EXPLAINCLI": cmdFTEXPLAINCLI, "FT.CONFIG": cmdFTCONFIG, "FT.PROFILE": cmdFTPROFILE, "FT.MGET": cmdFTMGET, "FT.DROP": cmdFTDROPINDEX,
		"FT._DROPIFX": cmdFTDROPIFX, "FT._DROPINDEXIFX": cmdFTDROPIFX, "FT._CREATEIFNX": cmdFTCREATEIFNX,
		"FT._ALTERIFNX": cmdFTALTERIFNX, "FT._ALIASADDIFNX": cmdFTALIASADDIFNX, "FT._ALIASDELIFX": cmdFTALIASDELIFX,
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

// ftArityError is RediSearch's arity error, which names its internal
// command.
func ftArityError(name string) error {
	return errors.New("ERR wrong number of arguments for '_" + name + "' command")
}

func cmdFTCREATE(ctx *Context) error {
	if ctx.ArgCount() < 4 {
		return ctx.WriteError(ftArityError("FT.CREATE"))
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

// cmdFTALTER is FT.ALTER index [SKIPINITIALSCAN] SCHEMA ADD field options ...
func cmdFTALTER(ctx *Context) error {
	if ctx.ArgCount() < 4 {
		return ctx.WriteError(ftArityError("FT.ALTER"))
	}
	manager := searchIndexesFor(ctx)
	idx, ok := manager.Get(ctx.ArgString(0))
	if !ok {
		return ctx.WriteError(errors.New(ctx.ArgString(0) + ": no such index"))
	}
	i := 1
	skip := strings.EqualFold(ctx.ArgString(i), "SKIPINITIALSCAN")
	if skip {
		i++
	}
	if !strings.EqualFold(ctx.ArgString(i), "SCHEMA") {
		return ctx.WriteError(errors.New("ALTER must be followed by SCHEMA")) //nolint:staticcheck // RediSearch's reply text
	}
	if i+1 >= ctx.ArgCount() || !strings.EqualFold(ctx.ArgString(i+1), "ADD") {
		return ctx.WriteError(errors.New("Unknown action passed to ALTER SCHEMA")) //nolint:staticcheck // RediSearch's reply text
	}
	args := make([]string, 0, ctx.ArgCount()-i-2)
	for j := i + 2; j < ctx.ArgCount(); j++ {
		args = append(args, ctx.ArgString(j))
	}
	altered, err := idx.Alter(args, skip)
	if err != nil {
		return ctx.WriteError(err)
	}
	manager.Replace(altered)
	return ctx.WriteOK()
}

// cmdFTDROPINDEX is FT.DROPINDEX index [DD] and the older FT.DROP index
// [KEEPDOCS], which deletes the documents unless told to keep them.
func cmdFTDROPINDEX(ctx *Context) error {
	if ctx.ArgCount() < 1 || ctx.ArgCount() > 2 {
		return ctx.WriteError(arityError(ctx.Command))
	}
	manager := searchIndexesFor(ctx)
	idx, ok := manager.Get(ctx.ArgString(0))
	if !ok {
		return ctx.WriteError(errors.New(ctx.ArgString(0) + ": no such index"))
	}
	drop := ctx.Command == "FT.DROP"
	delDocs := drop
	if ctx.ArgCount() == 2 {
		switch arg := ctx.ArgString(1); {
		case strings.EqualFold(arg, "_FORCEKEEPDOCS"):
			delDocs = false
		case drop && strings.EqualFold(arg, "KEEPDOCS"):
			delDocs = false
		case !drop && strings.EqualFold(arg, "DD"):
			delDocs = true
		default:
			return ctx.WriteError(errors.New("Unknown argument")) //nolint:staticcheck // RediSearch's reply text
		}
	}
	if delDocs {
		for _, doc := range indexDocuments(ctx, idx, nil) {
			ctx.Store.Delete(doc.Key)
		}
	}
	manager.Drop(idx.Name)
	return ctx.WriteOK()
}

// The _IFNX and _IFX variants, which RediSearch's coordinator sends to
// shards, succeed when there is nothing to do.
func cmdFTDROPIFX(ctx *Context) error {
	if ctx.ArgCount() >= 1 && ctx.ArgCount() <= 2 {
		if _, ok := searchIndexesFor(ctx).Get(ctx.ArgString(0)); !ok {
			return ctx.WriteOK()
		}
	}
	return cmdFTDROPINDEX(ctx)
}

func cmdFTCREATEIFNX(ctx *Context) error {
	if ctx.ArgCount() >= 4 {
		if _, ok := searchIndexesFor(ctx).Get(ctx.ArgString(0)); ok {
			return ctx.WriteOK()
		}
	}
	return cmdFTCREATE(ctx)
}

func cmdFTALTERIFNX(ctx *Context) error {
	if ctx.ArgCount() >= 4 {
		if idx, ok := searchIndexesFor(ctx).Get(ctx.ArgString(0)); ok {
			at := 2
			if strings.EqualFold(ctx.ArgString(1), "SKIPINITIALSCAN") {
				at = 3
			}
			if at+1 < ctx.ArgCount() && strings.EqualFold(ctx.ArgString(at), "ADD") && idx.Field(ctx.ArgString(at+1)) != nil {
				return ctx.WriteOK()
			}
		}
	}
	return cmdFTALTER(ctx)
}

func cmdFTALIASADDIFNX(ctx *Context) error {
	if ctx.ArgCount() == 2 {
		m := searchIndexesFor(ctx)
		if target, ok := m.Get(ctx.ArgString(0)); ok {
			if idx, ok := m.Get(ctx.ArgString(1)); ok && idx == target {
				return ctx.WriteOK()
			}
		}
	}
	return cmdFTALIASADD(ctx)
}

func cmdFTALIASDELIFX(ctx *Context) error {
	if ctx.ArgCount() == 1 && !searchIndexesFor(ctx).HasAlias(ctx.ArgString(0)) {
		return ctx.WriteOK()
	}
	return cmdFTALIASDEL(ctx)
}

// cmdFTEXPLAINCLI is FT.EXPLAIN with the tree's lines as an array.
func cmdFTEXPLAINCLI(ctx *Context) error {
	if ctx.ArgCount() < 2 {
		return ctx.WriteError(moduleArityError(ctx))
	}
	if _, ok := searchIndexesFor(ctx).Get(ctx.ArgString(0)); !ok {
		return ctx.WriteError(errors.New(ctx.ArgString(0) + ": no such index"))
	}
	tree, err := explainQuery(ctx)
	if err != nil {
		return ctx.WriteError(err)
	}
	lines := strings.Split(tree, "\n")
	vals := make([]*resp.Value, len(lines))
	for i, l := range lines {
		vals[i] = resp.SimpleString(l)
	}
	return ctx.WriteArray(vals)
}

// cmdFTMGET is FT.MGET index key [key ...]: each document's fields, or a
// null for a key the index does not hold.
func cmdFTMGET(ctx *Context) error {
	if ctx.ArgCount() < 2 {
		return ctx.WriteError(moduleArityError(ctx))
	}
	idx, ok := searchIndexesFor(ctx).Get(ctx.ArgString(0))
	if !ok {
		return ctx.WriteError(errors.New(ctx.ArgString(0) + ": no such index"))
	}
	out := make([]*resp.Value, 0, ctx.ArgCount()-1)
	for i := 1; i < ctx.ArgCount(); i++ {
		key := ctx.ArgString(i)
		entry, exists := ctx.Store.Get(key)
		var doc *search.Document
		if exists && idx.Covers(key) {
			doc, _ = idx.Load(key, entry, true)
		}
		if doc == nil {
			out = append(out, resp.NullBulkString())
			continue
		}
		out = append(out, documentReply(idx, doc, &searchRequest{}, nil, nil))
	}
	return ctx.WriteArray(out)
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
		case search.FieldVector:
			v := f.Vector
			a = append(a, resp.SimpleString("algorithm"), resp.SimpleString(v.Algorithm), resp.SimpleString("data_type"), resp.SimpleString(v.Type),
				resp.SimpleString("dim"), resp.IntegerValue(int64(v.Dim)), resp.SimpleString("distance_metric"), resp.SimpleString(v.Metric))
			if v.Algorithm == "HNSW" {
				m, ef := int64(16), int64(200)
				for _, p := range v.Params {
					if x, err := strconv.ParseInt(p[1], 10, 64); err == nil {
						switch p[0] {
						case "M":
							m = x
						case "EF_CONSTRUCTION":
							ef = x
						}
					}
				}
				a = append(a, resp.SimpleString("M"), resp.IntegerValue(m), resp.SimpleString("ef_construction"), resp.IntegerValue(ef))
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
	idle, total, own := searchIndexesFor(ctx).CursorStats(idx)
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
		resp.SimpleString("cursor_stats"), resp.ArrayValue([]*resp.Value{
			resp.SimpleString("global_idle"), resp.IntegerValue(int64(idle)),
			resp.SimpleString("global_total"), resp.IntegerValue(int64(total)),
			resp.SimpleString("index_capacity"), resp.IntegerValue(int64(search.IndexCursorLimit())),
			resp.SimpleString("index_total"), resp.IntegerValue(int64(own)),
		}),
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
	maxSearch, maxAggregate := uint64(search.MaxSearchResults()), uint64(search.MaxAggregateResults())
	switch {
	case cnt == 0:
	case ctx.Command == "FT.SEARCH" && cnt > maxSearch:
		return 0, 0, fmt.Errorf("LIMIT exceeds maximum of %d", maxSearch)
	case ctx.Command != "FT.SEARCH" && cnt > maxAggregate:
		return 0, 0, fmt.Errorf("LIMIT exceeds maximum of %d", maxAggregate)
	case off > maxSearch:
		return 0, 0, fmt.Errorf("OFFSET exceeds maximum of %d", maxSearch)
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
	cursor                              cursorSettings
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
		case "WITHCURSOR":
			last, err := parseCursorSettings(ctx, i, &r.cursor)
			if err != nil {
				return nil, err
			}
			i = last
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
		yields  [][2]string
		sortKey string
		sortNum float64
		sortOK  bool
		isNum   bool
	}
	found, err := idx.Search(ctx.Store, query, req.opts)
	if err != nil {
		return ctx.WriteError(err)
	}
	var hits []hit
	for _, h := range found {
		if req.inKeys == nil || req.inKeys[h.Doc.Key] {
			hits = append(hits, hit{doc: h.Doc, score: h.Score, yields: h.Yields})
		}
	}
	if req.sortBy != "" {
		for i := range hits {
			h := &hits[i]
			h.sortKey, h.sortOK = yieldValue(h.yields, req.sortBy)
			if !h.sortOK {
				h.sortKey, h.sortOK = search.FieldValue(idx, h.doc, req.sortBy)
			}
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
	var entries [][]*resp.Value
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
			add("sortkey", sortKeyReply(idx, h.doc, req.sortBy, h.yields))
		}
		if !req.noContent {
			doc := documentReply(idx, h.doc, req, order, h.yields)
			if resp3 {
				doc = resp.PairsValue(doc.Array)
			}
			add("extra_attributes", doc)
		}
		if resp3 {
			add("values", resp.ArrayValue([]*resp.Value{}))
			fields = []*resp.Value{resp.PairsValue(fields)}
		}
		entries = append(entries, fields)
	}
	results := &searchResults{entries: entries, total: len(hits)}
	if !req.cursor.on {
		reply, _, err := results.chunk(ctx, math.MaxInt)
		if err != nil {
			return ctx.WriteError(err)
		}
		return ctx.WriteValue(reply)
	}
	manager := searchIndexesFor(ctx)
	c, err := manager.ReserveCursor(idx, req.cursor.maxIdle, results)
	if err != nil {
		return ctx.WriteError(err)
	}
	c.Chunk = req.cursor.count
	return readCursor(ctx, manager, c, 0)
}

// searchResults holds an FT.SEARCH's sorted documents, each one's reply
// entries ready; a cursor sends them in chunks, the total with the first.
type searchResults struct {
	entries [][]*resp.Value
	total   int
	next    int
	sent    bool
}

func (s *searchResults) chunk(ctx *Context, n int) (*resp.Value, bool, error) {
	total := 0
	if !s.sent {
		total = s.total
	}
	s.sent = true
	to := s.next + min(n, len(s.entries)-s.next)
	var results []*resp.Value
	for _, e := range s.entries[s.next:to] {
		results = append(results, e...)
	}
	full := to-s.next == n
	s.next = to
	return searchReply(ctx.Writer.IsRESP3(), total, results), full, nil
}

type cursorState interface {
	chunk(ctx *Context, n int) (*resp.Value, bool, error)
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

// yieldValue returns a field a vector query yielded.
func yieldValue(yields [][2]string, name string) (string, bool) {
	for _, y := range yields {
		if y[0] == name {
			return y[1], true
		}
	}
	return "", false
}

func sortKeyReply(idx *search.Index, doc *search.Document, field string, yields [][2]string) *resp.Value {
	v, ok := yieldValue(yields, field)
	if !ok {
		v, ok = search.FieldValue(idx, doc, field)
	}
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
func documentReply(idx *search.Index, doc *search.Document, req *searchRequest, o *fieldOrder, yields [][2]string) *resp.Value {
	if o == nil {
		o = &fieldOrder{}
	}
	// A vector query's distance fields are made before any other.
	for _, y := range yields {
		o.add(y[0])
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
			if v, ok := yieldValue(yields, r[0]); ok {
				values[r[1]] = v
			} else if v, ok := search.FieldValue(idx, doc, r[0]); ok {
				values[r[1]] = v
			}
		}
		for _, name := range o.names {
			if v, ok := values[name]; ok {
				out = append(out, resp.BulkString(name), resp.BulkString(v))
			}
		}
	case doc.JSON != nil:
		for _, y := range yields {
			out = append(out, resp.BulkString(y[0]), resp.BulkString(y[1]))
		}
		out = append(out, resp.BulkString("$"), resp.BulkBytes(store.AppendJSON(nil, doc.JSON)))
	default:
		values := make(map[string]string, len(doc.Fields))
		for _, y := range yields {
			values[y[0]] = y[1]
		}
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
	var cursor cursorSettings
	n := ctx.ArgCount()
	for i := 2; i < n; i++ {
		switch strings.ToUpper(ctx.ArgString(i)) {
		case "WITHCURSOR":
			last, err := parseCursorSettings(ctx, i, &cursor)
			if err != nil {
				return ctx.WriteError(err)
			}
			i = last
		case "WITHCOUNT":
			return ctx.WriteError(errors.New("FT.AGGREGATE + WITHCOUNT is not available when `ENABLE_UNSTABLE_FEATURES` is off")) //nolint:staticcheck // RediSearch's reply text
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
	found, err := idx.Search(ctx.Store, query, opts)
	if err != nil {
		return ctx.WriteError(err)
	}
	if !search.NearestFirst(query) {
		sort.Slice(found, func(i, j int) bool { return found[i].Doc.Before(found[j].Doc) })
	}
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
	docs := make(map[*search.Row]*search.Document)
	for i, h := range found {
		if d := idx.Content(ctx.Store, h.Doc.Key); d != nil {
			r := idx.NewRow(d)
			r.SetDocID(int64(i + 1))
			for _, y := range h.Yields {
				r.Set(y[0], y[1])
			}
			if addScores {
				r.SetFloat("__score", h.Score)
			}
			rows = append(rows, r)
			read[r] = i + 1
			docs[r] = h.Doc
		}
	}
	if ctx.aggTrace != nil {
		ctx.aggTrace.matched = len(rows)
	}
	pipeline := newPipelineKeys(idx)
	for _, y := range search.Yields(query) {
		pipeline.add(y)
	}
	if addScores {
		pipeline.add("__score")
	}
	grouped, sorted, arranged, groups := false, false, false, 0
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
				pipeline.all = pipeline.root
				i++
				continue
			}
			fields, last, err := countedArgs(ctx, i)
			if err != nil {
				return ctx.WriteError(err)
			}
			for j := 0; j < len(fields); j++ {
				name := strings.TrimPrefix(fields[j], "@")
				as := name
				if j+2 < len(fields) && strings.EqualFold(fields[j+1], "AS") {
					as = fields[j+2]
					j += 2
				}
				pipeline.add(as)
				for _, r := range rows {
					r.LoadAs(name, as)
				}
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
			if err := pipeline.group(rows, props, reducers); err != nil {
				return ctx.WriteError(err)
			}
			if err := barrier(); err != nil {
				return ctx.WriteError(err)
			}
			if rows, err = search.Group(rows, props, reducers); err != nil {
				return ctx.WriteError(err)
			}
			grouped, arranged, groups = true, false, len(rows)
			if ctx.aggTrace != nil {
				ctx.aggTrace.groups = append(ctx.aggTrace.groups, len(rows))
			}
		case "SORTBY", "LIMIT":
			last := aggregateStepEnd(ctx, i)
			if word == "SORTBY" {
				if _, end, err := countedArgs(ctx, i); err == nil {
					last = end
					if end+2 < n && strings.EqualFold(ctx.ArgString(end+1), "MAX") {
						last = end + 2
					}
				}
			} else {
				last = i + 2
			}
			if arranged {
				i = last
				continue
			}
			arranged = true
			arr, err := scanArrange(ctx, i)
			if err != nil {
				return ctx.WriteError(err)
			}
			i = last
			if arr.sorted {
				if err := pipeline.sortBy(rows, arr.keys); err != nil {
					return ctx.WriteError(err)
				}
				if err := barrier(); err != nil {
					return ctx.WriteError(err)
				}
				keep := len(rows)
				if max := arr.offset + arr.limit; !arr.limited || arr.limit != 0 {
					keep = max
					if keep == 0 {
						keep = 10
					}
				}
				rows = search.SortRowsTop(rows, arr.keys, keep)
				sorted = true
			}
			switch {
			case arr.limited && arr.limit == 0:
				offset, limit = 0, 0
			case arr.offset == 0 && (arr.limit == 0 || arr.sorted):
			case arr.last && !arr.filterAfter:
				offset, limit = arr.offset, arr.limit
			default:
				from := min(arr.offset, len(rows))
				rows = rows[from:min(from+arr.limit, len(rows))]
			}
		case "APPLY":
			if i+3 >= n || !strings.EqualFold(ctx.ArgString(i+2), "AS") {
				return ctx.WriteError(errors.New("Bad arguments for APPLY: AS is required")) //nolint:staticcheck // RediSearch's reply text
			}
			expr, err := search.ParseExpr(ctx.ArgString(i + 1))
			if err != nil {
				return ctx.WriteError(err)
			}
			if err := pipeline.readExpr(rows, expr); err != nil {
				return ctx.WriteError(err)
			}
			name := ctx.ArgString(i + 3)
			pipeline.add(name)
			for _, r := range rows {
				if _, ok := failed[r]; ok {
					continue
				}
				if missing, ok := expr.Missing(r); ok {
					failed[r] = errMissingProperty(missing)
					continue
				}
				if v, f, num := expr.EvalTyped(r); num {
					r.SetFloat(name, f)
				} else {
					r.Set(name, v)
				}
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
			if err := pipeline.readExpr(rows, expr); err != nil {
				return ctx.WriteError(err)
			}
			kept := rows[:0]
			trace := filterTrace{failAt: -1}
			for pos, r := range rows {
				if _, ok := failed[r]; ok {
					if trace.failAt < 0 {
						trace.failAt = pos
					}
					kept = append(kept, r)
					continue
				}
				if missing, ok := expr.Missing(r); ok {
					failed[r] = errMissingProperty(missing)
					if trace.failAt < 0 {
						trace.failAt = pos
					}
					kept = append(kept, r)
					continue
				}
				if expr.Truthy(r) {
					trace.passed = append(trace.passed, pos)
					kept = append(kept, r)
				}
			}
			rows = kept
			if ctx.aggTrace != nil {
				ctx.aggTrace.filters = append(ctx.aggTrace.filters, trace)
			}
			i++
		case "PARAMS":
			_, last, err := countedArgs(ctx, i)
			if err != nil {
				return ctx.WriteError(err)
			}
			i = last
		case "TIMEOUT", "DIALECT":
			i++
		case "WITHCURSOR":
			last, err := parseCursorSettings(ctx, i, &cursor)
			if err != nil {
				return ctx.WriteError(err)
			}
			i = last
		}
	}
	hits := make([]*search.Document, len(found))
	for i, h := range found {
		hits[i] = h.Doc
	}
	agg := &aggregation{idx: idx, hits: hits, order: pipeline.order, rows: rows, read: read, docs: docs, failed: failed, found: len(found), next: offset, end: len(rows), limitEnd: -1}
	if limit >= 0 {
		agg.end, agg.limitEnd = min(len(rows), offset+limit), offset+limit
	}
	if grouped || sorted {
		agg.ranked, agg.total = true, len(found)
		if grouped {
			agg.total = groups
		}
	}
	if !cursor.on {
		reply, _, err := agg.chunk(ctx, math.MaxInt)
		if err != nil {
			return ctx.WriteError(err)
		}
		return ctx.WriteValue(reply)
	}
	manager := searchIndexesFor(ctx)
	c, err := manager.ReserveCursor(idx, cursor.maxIdle, agg)
	if err != nil {
		return ctx.WriteError(err)
	}
	c.Chunk = cursor.count
	return readCursor(ctx, manager, c, 0)
}

// aggregation holds an FT.AGGREGATE's rows; a cursor sends them in
// chunks. RediSearch streams rows that no GROUPBY or SORTBY gathers, and
// each reply's total counts the documents the query read for it: in RESP2
// until its first row, in RESP3 until its last row, or all that were left
// once the query ran out of documents. A gathered reply sends its total
// once.
type aggregation struct {
	idx      *search.Index
	order    []string
	rows     []*search.Row
	docs     map[*search.Row]*search.Document
	hits     []*search.Document
	read     map[*search.Row]int
	failed   map[*search.Row]error
	found    int
	next     int
	end      int
	limitEnd int
	pos      int
	ranked   bool
	total    int
	sent     bool
}

// chunk replies with up to n rows and reports whether rows remain.
func (a *aggregation) chunk(ctx *Context, n int) (*resp.Value, bool, error) {
	resp3 := ctx.Writer.IsRESP3()
	first := !a.sent
	a.sent = true
	if !first && !a.ranked {
		a.dropStale(ctx.Store, n)
	}
	from, scan := min(a.next, a.end), a.next
	if first {
		scan = 0
	}
	to := max(from, min(a.end, from+min(n, a.end)))
	var warning error
	for i := scan; i < to; i++ {
		if err, ok := a.failed[a.rows[i]]; ok {
			if i <= from {
				return nil, false, err
			}
			to, warning = i, err
			break
		}
	}
	full := warning == nil && to-from == n
	pos := a.pos
	switch {
	case warning != nil:
		a.pos = a.read[a.rows[to]]
	case to > from && (full || to == a.limitEnd):
		a.pos = a.read[a.rows[to-1]]
	case to == from && !first && from >= a.limitEnd && a.limitEnd >= 0:
	default:
		a.pos = a.found
	}
	// Documents written or deleted since the query are skipped unread.
	read := func(lo, hi int) int {
		if first || hi <= lo {
			return hi - lo
		}
		n := 0
		for _, live := range a.idx.Current(ctx.Store, a.hits[lo:hi]) {
			if live {
				n++
			}
		}
		return n
	}
	var total int
	if !a.ranked {
		total = read(pos, a.pos)
		if !resp3 && to > from {
			total = read(pos, a.read[a.rows[from]])
		}
	}
	if a.ranked && first {
		total = a.total
	}
	a.next = to
	results := make([]*resp.Value, 0, to-from)
	for _, r := range a.rows[from:to] {
		results = append(results, aggregateRowReply(r, resp3, a.order))
	}
	var warnings []string
	if warning != nil {
		warnings = append(warnings, warning.Error())
	}
	return searchReply(resp3, total, results, warnings...), full, nil
}

// dropStale removes, from the next n rows, those whose document was
// written or deleted since the query: RediSearch's index iterator skips
// deleted documents, and a written one is indexed again under a new id.
func (a *aggregation) dropStale(st *store.Store, n int) {
	from := a.next
	for kept := 0; kept < n && from < len(a.rows); {
		window := a.rows[from:min(len(a.rows), from+min(n-kept, len(a.rows)))]
		docs := make([]*search.Document, len(window))
		for i, r := range window {
			docs[i] = a.docs[r]
		}
		live := a.idx.Current(st, docs)
		j := from
		for i, r := range window {
			if live[i] {
				a.rows[j] = r
				j++
			}
		}
		kept += j - from
		a.rows = append(a.rows[:j], a.rows[from+len(window):]...)
		from = j
	}
	a.end = len(a.rows)
	if a.limitEnd >= 0 {
		a.end = min(a.end, a.limitEnd)
	}
}

// aggregateRowReply sends a row's fields in the order of the pipeline's
// lookup keys, then the fields LOAD * read.
func aggregateRowReply(r *search.Row, resp3 bool, order []string) *resp.Value {
	fields := make([]*resp.Value, 0, 2*len(r.Names))
	names := make([]string, 0, len(r.Names))
	listed := make(map[string]bool, len(order))
	for _, name := range order {
		if _, ok := r.Value(name); ok {
			names = append(names, name)
			listed[name] = true
		}
	}
	for _, name := range r.Names {
		if !listed[name] {
			names = append(names, name)
		}
	}
	for _, name := range names {
		v, _ := r.Value(name)
		switch {
		case r.IsNull(name):
			fields = append(fields, resp.BulkString(name), resp.NullBulkString())
		case r.IsList(name):
			list := []*resp.Value{}
			for _, item := range r.ListItems(name) {
				if item.Null {
					list = append(list, resp.NullBulkString())
				} else {
					list = append(list, resp.BulkString(item.Value))
				}
			}
			fields = append(fields, resp.BulkString(name), resp.ArrayValue(list))
		default:
			fields = append(fields, resp.BulkString(name), resp.BulkString(v))
		}
	}
	if !resp3 {
		return resp.ArrayValue(fields)
	}
	return resp.PairsValue([]*resp.Value{
		resp.SimpleString("extra_attributes"), resp.PairsValue(fields),
		resp.SimpleString("values"), resp.ArrayValue([]*resp.Value{}),
	})
}

type cursorSettings struct {
	on      bool
	count   int
	maxIdle time.Duration
}

// parseCursorSettings reads WITHCURSOR's MAXIDLE and COUNT after args[i]
// and returns the index of the last argument they use.
func parseCursorSettings(ctx *Context, i int, cs *cursorSettings) (int, error) {
	cs.on = true
	for i+1 < ctx.ArgCount() {
		name := strings.ToUpper(ctx.ArgString(i + 1))
		if name != "MAXIDLE" && name != "COUNT" {
			break
		}
		if i+2 >= ctx.ArgCount() {
			return i, errors.New("Bad arguments for " + name + ": Expected an argument, but none provided") //nolint:staticcheck // RediSearch's reply text
		}
		v, err := strconv.ParseInt(ctx.ArgString(i+2), 10, 64)
		if err != nil {
			return i, errors.New("Bad arguments for " + name + ": Could not convert argument to expected type") //nolint:staticcheck // RediSearch's reply text
		}
		if v < 1 || v > math.MaxUint32 {
			return i, errors.New("Bad arguments for " + name + ": Value is outside acceptable bounds") //nolint:staticcheck // RediSearch's reply text
		}
		if name == "COUNT" {
			cs.count = int(v)
		} else {
			cs.maxIdle = time.Duration(v) * time.Millisecond
		}
		i += 2
	}
	if cs.maxIdle == 0 || cs.maxIdle > search.CursorMaxIdle() {
		cs.maxIdle = search.CursorMaxIdle()
	}
	if cs.count == 0 {
		cs.count = search.CursorReadSize
	}
	return i, nil
}

// readCursor sends a cursor's next chunk; a count other than 0 becomes the
// cursor's chunk size, and a negative one sends every row left.
func readCursor(ctx *Context, manager *search.IndexManager, c *search.Cursor, count int64) error {
	if count != 0 {
		c.Chunk = int(max(count, -1))
	}
	n := c.Chunk
	if n < 0 {
		n = math.MaxInt
	}
	reply, more, err := c.State.(cursorState).chunk(ctx, n)
	if err != nil {
		manager.FreeCursor(c)
		return ctx.WriteError(err)
	}
	id := int64(c.ID)
	if more {
		manager.PauseCursor(c)
	} else {
		manager.FreeCursor(c)
		id = 0
	}
	return ctx.WriteValue(resp.ArrayValue([]*resp.Value{reply, resp.IntegerValue(id)}))
}

// cmdFTCURSOR reads and closes FT.AGGREGATE's cursors. Any existing index
// may name a cursor, as in RediSearch, whose cursors are global.
func cmdFTCURSOR(ctx *Context) error {
	if ctx.ArgCount() < 3 {
		return ctx.WriteError(errors.New("ERR wrong number of arguments for 'FT.CURSOR' command"))
	}
	manager := searchIndexesFor(ctx)
	if _, ok := manager.Get(ctx.ArgString(1)); !ok {
		return ctx.WriteError(errors.New(ctx.ArgString(1) + ": no such index"))
	}
	cid, err := strconv.ParseInt(ctx.ArgString(2), 10, 64)
	if err != nil {
		return ctx.WriteError(errors.New("Bad cursor ID")) //nolint:staticcheck // RediSearch's reply text
	}
	notFound := func() error {
		return ctx.WriteError(fmt.Errorf("Cursor not found, id: %d", int32(cid))) //nolint:staticcheck // RediSearch's reply text
	}
	switch strings.ToUpper(ctx.ArgString(0)) {
	case "READ":
		var count int64
		if ctx.ArgCount() > 4 {
			if !strings.EqualFold(ctx.ArgString(3), "COUNT") {
				return ctx.WriteError(fmt.Errorf("Unknown argument `%s`", ctx.ArgString(3))) //nolint:staticcheck // RediSearch's reply text
			}
			if count, err = strconv.ParseInt(ctx.ArgString(4), 10, 64); err != nil {
				return ctx.WriteError(fmt.Errorf("Bad value for COUNT: `%s`", ctx.ArgString(4))) //nolint:staticcheck // RediSearch's reply text
			}
		}
		c := manager.TakeCursor(uint64(cid))
		if c == nil {
			return notFound()
		}
		if !manager.CursorIndexAlive(c) {
			manager.FreeCursor(c)
			return ctx.WriteError(errors.New("The index was dropped while the cursor was idle")) //nolint:staticcheck // RediSearch's reply text
		}
		return readCursor(ctx, manager, c, count)
	case "PROFILE":
		c := manager.TakeCursor(uint64(cid))
		if c == nil {
			return notFound()
		}
		manager.PauseCursor(c)
		return ctx.WriteError(fmt.Errorf("cursor request is not profile, id: %d", int32(cid)))
	case "DEL":
		if !manager.PurgeCursor(uint64(cid)) {
			return ctx.WriteError(errors.New("Cursor does not exist")) //nolint:staticcheck // RediSearch's reply text
		}
		return ctx.WriteOK()
	case "GC":
		return ctx.WriteInteger(int64(manager.CollectCursors()))
	}
	return ctx.WriteError(errors.New("Unknown subcommand")) //nolint:staticcheck // RediSearch's reply text
}

// cmdFTSYNUPDATE is FT.SYNUPDATE index group [SKIPINITIALSCAN] term ...
func cmdFTSYNUPDATE(ctx *Context) error {
	if ctx.ArgCount() < 3 {
		return ctx.WriteError(ftArityError("FT.SYNUPDATE"))
	}
	idx, ok := searchIndexesFor(ctx).Get(ctx.ArgString(0))
	if !ok {
		return ctx.WriteError(errors.New(ctx.ArgString(0) + ": no such index"))
	}
	skip := strings.EqualFold(ctx.ArgString(2), "SKIPINITIALSCAN")
	from := 2
	if skip {
		from = 3
	}
	terms := make([]string, 0, ctx.ArgCount()-from)
	for i := from; i < ctx.ArgCount(); i++ {
		terms = append(terms, ctx.ArgString(i))
	}
	if err := idx.UpdateSynonyms(ctx.ArgString(1), terms, skip); err != nil {
		return ctx.WriteError(err)
	}
	return ctx.WriteOK()
}

// cmdFTSYNDUMP replies with each synonym term and its groups, in the order
// the terms were added; RediSearch lists them in its hash table's order.
func cmdFTSYNDUMP(ctx *Context) error {
	if ctx.ArgCount() != 1 {
		return ctx.WriteError(moduleArityError(ctx))
	}
	idx, ok := searchIndexesFor(ctx).Get(ctx.ArgString(0))
	if !ok {
		return ctx.WriteError(errors.New(ctx.ArgString(0) + ": no such index"))
	}
	terms, groups := idx.Synonyms()
	out := make([]*resp.Value, 0, 2*len(terms))
	for i, t := range terms {
		ids := make([]*resp.Value, len(groups[i]))
		for j, g := range groups[i] {
			ids[j] = resp.BulkString(g)
		}
		out = append(out, resp.BulkString(t), resp.ArrayValue(ids))
	}
	if ctx.Writer.IsRESP3() {
		return ctx.WriteValue(resp.PairsValue(out))
	}
	return ctx.WriteArray(out)
}

func cmdFTSYNADD(ctx *Context) error {
	return ctx.WriteError(errors.New("No longer supported, use FT.SYNUPDATE")) //nolint:staticcheck // RediSearch's reply text
}

func cmdFTDICTADD(ctx *Context) error {
	if ctx.ArgCount() < 2 {
		return ctx.WriteError(ftArityError("FT.DICTADD"))
	}
	return ctx.WriteInteger(int64(searchIndexesFor(ctx).DictAdd(ctx.ArgString(0), argStrings(ctx)[1:])))
}

func cmdFTDICTDEL(ctx *Context) error {
	if ctx.ArgCount() < 2 {
		return ctx.WriteError(ftArityError("FT.DICTDEL"))
	}
	return ctx.WriteInteger(int64(searchIndexesFor(ctx).DictDel(ctx.ArgString(0), argStrings(ctx)[1:])))
}

func cmdFTDICTDUMP(ctx *Context) error {
	if ctx.ArgCount() != 1 {
		return ctx.WriteError(moduleArityError(ctx))
	}
	terms := searchIndexesFor(ctx).DictDump(ctx.ArgString(0))
	out := make([]*resp.Value, len(terms))
	for i, t := range terms {
		out[i] = resp.BulkString(t)
	}
	if ctx.Writer.IsRESP3() {
		return ctx.WriteValue(resp.SetValue(out))
	}
	return ctx.WriteArray(out)
}

// argIndex is RMUtil_ArgExists: the first argument from start that equals
// name, ignoring case, or 0.
func argIndex(ctx *Context, name string, start int) int {
	for i := max(start, 1); i <= ctx.ArgCount(); i++ {
		if strings.EqualFold(ctx.ArgString(i-1), name) {
			return i
		}
	}
	return 0
}

// cmdFTSPELLCHECK is FT.SPELLCHECK index query [DISTANCE d] [TERMS
// INCLUDE|EXCLUDE dict]... [DIALECT n] [PARAMS ...]. Arguments are found
// anywhere after the query, as RediSearch looks them up.
func cmdFTSPELLCHECK(ctx *Context) error {
	if ctx.ArgCount() < 2 {
		return ctx.WriteError(moduleArityError(ctx))
	}
	manager := searchIndexesFor(ctx)
	idx, ok := manager.Get(ctx.ArgString(0))
	if !ok {
		return ctx.WriteError(errors.New(ctx.ArgString(0) + ": no such index"))
	}
	n := ctx.ArgCount() + 1 // argv positions as RediSearch counts them, the command at 0
	arg := func(i int) string { return ctx.ArgString(i - 1) }
	// RediSearch parses the query without a stop word list, so stop words
	// are checked too.
	opts := search.QueryOptions{Params: map[string]string{}, NoStopwords: true}
	if p := argIndex(ctx, "DIALECT", 3); p > 0 && p+1 < n {
		opts.Dialect = parseDialect(arg(p + 1))
	}
	if p := argIndex(ctx, "PARAMS", 3); p > 0 && p+1 < n {
		if count, err := strconv.Atoi(arg(p + 1)); err == nil {
			for j := p + 2; j+1 < n && j+1 < p+2+count; j += 2 {
				opts.Params[arg(j)] = arg(j + 1)
			}
		}
	}
	query, err := search.ParseQuery(idx, ctx.ArgString(1), opts)
	if err != nil {
		return ctx.WriteError(err)
	}
	distance := 1
	if p := argIndex(ctx, "DISTANCE", 0); p > 0 {
		if p+1 >= n {
			return ctx.WriteError(errors.New("DISTANCE arg is given but no DISTANCE comes after"))
		}
		d, err := strconv.ParseInt(arg(p+1), 10, 64)
		if err != nil || d < 1 || d > 4 {
			return ctx.WriteError(errors.New("bad distance given, distance must be a natural number between 1 to 4"))
		}
		distance = int(d)
	}
	var include, exclude []string
	for p := argIndex(ctx, "TERMS", 1); p > 0; p = argIndex(ctx, "TERMS", p+1) {
		if p+2 >= n {
			return ctx.WriteError(errors.New("TERM arg is given but no TERM params comes after"))
		}
		switch strings.ToUpper(arg(p + 1)) {
		case "INCLUDE":
			include = append(include, arg(p+2))
		case "EXCLUDE":
			exclude = append(exclude, arg(p+2))
		default:
			return ctx.WriteError(errors.New("bad format, exclude/include operation was not given"))
		}
	}
	full := argIndex(ctx, "FULLSCOREINFO", 0) > 0
	for _, name := range append(append([]string(nil), include...), exclude...) {
		if !manager.DictExists(name) {
			return ctx.WriteError(errors.New("Dict does not exist: " + name)) //nolint:staticcheck // RediSearch's reply text
		}
	}
	terms, total := manager.SpellCheck(idx, ctx.Store, query, distance, include, exclude, full)
	resp3 := ctx.Writer.IsRESP3()
	score := func(f float64) *resp.Value {
		if resp3 {
			return resp.DoubleValue(fpconvDtoa(f))
		}
		return resp.BulkString(fpconvDtoa(f))
	}
	var results []*resp.Value
	for _, t := range terms {
		if t.Found {
			if resp3 {
				results = append(results, resp.BulkString(t.Term), resp.ErrorValue("term exists in index"))
			} else {
				results = append(results, resp.ArrayValue([]*resp.Value{resp.SimpleString("TERM"), resp.BulkString(t.Term), resp.SimpleString("term exists in index")}))
			}
			continue
		}
		suggestions := make([]*resp.Value, len(t.Suggestions))
		for i, s := range t.Suggestions {
			if resp3 {
				suggestions[i] = resp.PairsValue([]*resp.Value{resp.BulkString(s), score(t.Scores[i])})
			} else {
				suggestions[i] = resp.ArrayValue([]*resp.Value{score(t.Scores[i]), resp.BulkString(s)})
			}
		}
		if resp3 {
			results = append(results, resp.BulkString(t.Term), resp.ArrayValue(suggestions))
		} else {
			results = append(results, resp.ArrayValue([]*resp.Value{resp.SimpleString("TERM"), resp.BulkString(t.Term), resp.ArrayValue(suggestions)}))
		}
	}
	if results == nil {
		results = []*resp.Value{}
	}
	if resp3 {
		var root []*resp.Value
		if full {
			root = append(root, resp.SimpleString("total_docs"), resp.IntegerValue(int64(total)))
		}
		root = append(root, resp.SimpleString("results"), resp.PairsValue(results))
		return ctx.WriteValue(resp.PairsValue(root))
	}
	if full {
		results = append([]*resp.Value{resp.IntegerValue(int64(total))}, results...)
	}
	return ctx.WriteArray(results)
}

// cmdFTCONFIG is FT.CONFIG GET|SET|HELP option [value].
func cmdFTCONFIG(ctx *Context) error {
	if ctx.ArgCount() < 2 {
		return ctx.WriteError(moduleArityError(ctx))
	}
	resp3 := ctx.Writer.IsRESP3()
	value := func(v *string) *resp.Value {
		if v == nil {
			return resp.NullBulkString()
		}
		return resp.BulkString(*v)
	}
	switch strings.ToUpper(ctx.ArgString(0)) {
	case "GET", "HELP":
		help := strings.EqualFold(ctx.ArgString(0), "HELP")
		var items []*resp.Value
		for _, o := range search.ConfigGet(ctx.ArgString(1)) {
			v := value(o.Value)
			if help {
				desc := []*resp.Value{resp.SimpleString("Description"), resp.SimpleString(o.Help), resp.SimpleString("Value"), v}
				if resp3 {
					v = resp.PairsValue(desc)
				} else {
					v = resp.ArrayValue(append([]*resp.Value{resp.SimpleString(o.Name)}, desc...))
				}
			}
			switch {
			case resp3:
				items = append(items, resp.SimpleString(o.Name), v)
			case help:
				items = append(items, v)
			default:
				items = append(items, resp.ArrayValue([]*resp.Value{resp.SimpleString(o.Name), v}))
			}
		}
		if items == nil {
			items = []*resp.Value{}
		}
		if resp3 {
			return ctx.WriteValue(resp.PairsValue(items))
		}
		return ctx.WriteArray(items)
	case "SET":
		excess, err := search.ConfigSet(ctx.ArgString(1), argStrings(ctx)[2:])
		if err != nil {
			return ctx.WriteError(err)
		}
		if excess {
			return ctx.WriteSimpleString("EXCESSARGS")
		}
		return ctx.WriteOK()
	}
	return ctx.WriteSimpleString("No such configuration action")
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
	tree, err := explainQuery(ctx)
	if err != nil {
		return ctx.WriteError(err)
	}
	return ctx.WriteBulkString(tree)
}

func explainQuery(ctx *Context) (string, error) {
	idx, ok := searchIndexesFor(ctx).Get(ctx.ArgString(0))
	if !ok {
		return "", errNoSuchIndex(ctx.ArgString(0))
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
				return "", err
			}
			for j := 0; j+1 < len(params); j += 2 {
				opts.Params[params[j]] = params[j+1]
			}
			i = last
		}
	}
	query, err := search.ParseQuery(idx, ctx.ArgString(1), opts)
	if err != nil {
		return "", err
	}
	return idx.Explain(query), nil
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
	return ctx.WriteValue(documentReply(idx, doc, &searchRequest{}, nil, nil))
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
