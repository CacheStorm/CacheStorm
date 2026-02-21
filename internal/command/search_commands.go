package command

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/cachestorm/cachestorm/internal/resp"
	"github.com/cachestorm/cachestorm/internal/search"
)

func RegisterSearchCommands(router *Router) {
	router.Register(&CommandDef{Name: "FT.CREATE", Handler: cmdFTCREATE})
	router.Register(&CommandDef{Name: "FT.DROPINDEX", Handler: cmdFTDROPINDEX})
	router.Register(&CommandDef{Name: "FT.INFO", Handler: cmdFTINFO})
	router.Register(&CommandDef{Name: "FT.SEARCH", Handler: cmdFTSEARCH})
	router.Register(&CommandDef{Name: "FT.ADD", Handler: cmdFTADD})
	router.Register(&CommandDef{Name: "FT.DEL", Handler: cmdFTDEL})
	router.Register(&CommandDef{Name: "FT.GET", Handler: cmdFTGET})
	router.Register(&CommandDef{Name: "FT._LIST", Handler: cmdFTLIST})
	router.Register(&CommandDef{Name: "FT.AGGREGATE", Handler: cmdFTAGGREGATE})
	router.Register(&CommandDef{Name: "FT.TAGVALS", Handler: cmdFTTAGVALS})
	router.Register(&CommandDef{Name: "FT.ALIASADD", Handler: cmdFTALIASADD})
	router.Register(&CommandDef{Name: "FT.ALIASDEL", Handler: cmdFTALIASDEL})
}

func cmdFTCREATE(ctx *Context) error {
	if ctx.ArgCount() < 1 {
		return ctx.WriteError(ErrWrongArgCount)
	}

	indexName := ctx.ArgString(0)
	schema := search.Schema{Fields: []search.FieldSchema{}}

	for i := 1; i < ctx.ArgCount(); i++ {
		arg := strings.ToUpper(ctx.ArgString(i))
		switch arg {
		case "SCHEMA":
			for i+1 < ctx.ArgCount() {
				i++
				fieldName := ctx.ArgString(i)
				if strings.HasPrefix(strings.ToUpper(fieldName), "ON") ||
					strings.HasPrefix(strings.ToUpper(fieldName), "PREFIX") ||
					strings.HasPrefix(strings.ToUpper(fieldName), "STOPWORDS") {
					break
				}

				fieldSchema := search.FieldSchema{
					Name: fieldName,
					Type: "TEXT",
				}

				for i+1 < ctx.ArgCount() {
					i++
					nextArg := strings.ToUpper(ctx.ArgString(i))
					if nextArg == "TEXT" || nextArg == "NUMERIC" || nextArg == "TAG" || nextArg == "GEO" {
						fieldSchema.Type = nextArg
					} else if nextArg == "SORTABLE" {
						fieldSchema.Sortable = true
					} else if nextArg == "NOINDEX" {
						fieldSchema.NoIndex = true
					} else {
						i--
						break
					}
				}

				schema.Fields = append(schema.Fields, fieldSchema)
			}
		}
	}

	manager := search.GetIndexManager()
	if err := manager.CreateIndex(indexName, schema); err != nil {
		return ctx.WriteError(err)
	}

	return ctx.WriteOK()
}

func cmdFTDROPINDEX(ctx *Context) error {
	if ctx.ArgCount() < 1 {
		return ctx.WriteError(ErrWrongArgCount)
	}

	indexName := ctx.ArgString(0)
	manager := search.GetIndexManager()

	if !manager.DropIndex(indexName) {
		return ctx.WriteError(fmt.Errorf("ERR index not found"))
	}

	return ctx.WriteOK()
}

func cmdFTINFO(ctx *Context) error {
	if ctx.ArgCount() < 1 {
		return ctx.WriteError(ErrWrongArgCount)
	}

	indexName := ctx.ArgString(0)
	manager := search.GetIndexManager()

	idx, ok := manager.GetIndex(indexName)
	if !ok {
		return ctx.WriteError(fmt.Errorf("ERR index not found"))
	}

	info := idx.Info()

	results := []*resp.Value{
		resp.BulkString("index_name"),
		resp.BulkString(info["name"].(string)),
		resp.BulkString("index_options"),
		resp.ArrayValue([]*resp.Value{}),
		resp.BulkString("fields"),
		formatSchema(idx.Schema),
		resp.BulkString("num_docs"),
		resp.IntegerValue(int64(info["document_count"].(int))),
	}

	return ctx.WriteArray(results)
}

func formatSchema(schema search.Schema) *resp.Value {
	fields := make([]*resp.Value, 0, len(schema.Fields))
	for _, f := range schema.Fields {
		fields = append(fields, resp.ArrayValue([]*resp.Value{
			resp.BulkString(f.Name),
			resp.BulkString("type"),
			resp.BulkString(f.Type),
		}))
	}
	return resp.ArrayValue(fields)
}

func cmdFTSEARCH(ctx *Context) error {
	if ctx.ArgCount() < 2 {
		return ctx.WriteError(ErrWrongArgCount)
	}

	indexName := ctx.ArgString(0)
	query := ctx.ArgString(1)

	limit := 10
	offset := 0
	noContent := false
	returnFields := []string{}

	for i := 2; i < ctx.ArgCount(); i++ {
		arg := strings.ToUpper(ctx.ArgString(i))
		switch arg {
		case "LIMIT":
			if i+2 < ctx.ArgCount() {
				offset = int(parseInt64(ctx.ArgString(i + 1)))
				limit = int(parseInt64(ctx.ArgString(i + 2)))
				i += 2
			}
		case "NOCONTENT":
			noContent = true
		case "RETURN":
			if i+1 < ctx.ArgCount() {
				i++
				numFields := int(parseInt64(ctx.ArgString(i)))
				for j := 0; j < numFields && i+1 < ctx.ArgCount(); j++ {
					i++
					returnFields = append(returnFields, ctx.ArgString(i))
				}
			}
		}
	}

	manager := search.GetIndexManager()
	idx, ok := manager.GetIndex(indexName)
	if !ok {
		return ctx.WriteError(fmt.Errorf("ERR index not found"))
	}

	result := idx.Search(query, limit, offset)

	results := make([]*resp.Value, 0, len(result.Documents)*2+1)
	results = append(results, resp.IntegerValue(int64(result.Total)))

	for _, doc := range result.Documents {
		results = append(results, resp.BulkString(doc.ID))

		if !noContent {
			fields := make([]*resp.Value, 0)
			for fname, fval := range doc.Fields {
				if len(returnFields) == 0 || contains(returnFields, fname) {
					fields = append(fields, resp.BulkString(fname), resp.BulkString(fval))
				}
			}
			results = append(results, resp.ArrayValue(fields))
		}
	}

	return ctx.WriteArray(results)
}

func cmdFTADD(ctx *Context) error {
	if ctx.ArgCount() < 3 {
		return ctx.WriteError(ErrWrongArgCount)
	}

	indexName := ctx.ArgString(0)
	docID := ctx.ArgString(1)
	score := parseJSONFloat(ctx.ArgString(2))

	fields := make(map[string]string)
	replace := false

	for i := 3; i < ctx.ArgCount(); i++ {
		arg := strings.ToUpper(ctx.ArgString(i))
		switch arg {
		case "FIELDS":
			for i+2 < ctx.ArgCount() {
				i++
				fieldName := ctx.ArgString(i)
				i++
				fieldValue := ctx.ArgString(i)
				fields[fieldName] = fieldValue
			}
		case "REPLACE":
			replace = true
		}
	}

	manager := search.GetIndexManager()
	idx, ok := manager.GetIndex(indexName)
	if !ok {
		return ctx.WriteError(fmt.Errorf("ERR index not found"))
	}

	doc := &search.Document{
		ID:     docID,
		Fields: fields,
		Score:  score,
	}

	_ = replace

	if err := idx.AddDocument(doc); err != nil {
		return ctx.WriteError(err)
	}

	return ctx.WriteOK()
}

func cmdFTDEL(ctx *Context) error {
	if ctx.ArgCount() < 2 {
		return ctx.WriteError(ErrWrongArgCount)
	}

	indexName := ctx.ArgString(0)
	docID := ctx.ArgString(1)

	manager := search.GetIndexManager()
	idx, ok := manager.GetIndex(indexName)
	if !ok {
		return ctx.WriteError(fmt.Errorf("ERR index not found"))
	}

	if !idx.DeleteDocument(docID) {
		return ctx.WriteInteger(0)
	}

	return ctx.WriteInteger(1)
}

func cmdFTGET(ctx *Context) error {
	if ctx.ArgCount() < 2 {
		return ctx.WriteError(ErrWrongArgCount)
	}

	indexName := ctx.ArgString(0)
	docID := ctx.ArgString(1)

	manager := search.GetIndexManager()
	idx, ok := manager.GetIndex(indexName)
	if !ok {
		return ctx.WriteError(fmt.Errorf("ERR index not found"))
	}

	doc, ok := idx.GetDocument(docID)
	if !ok {
		return ctx.WriteNull()
	}

	results := make([]*resp.Value, 0)
	for fname, fval := range doc.Fields {
		results = append(results, resp.BulkString(fname), resp.BulkString(fval))
	}

	return ctx.WriteArray(results)
}

func cmdFTLIST(ctx *Context) error {
	manager := search.GetIndexManager()
	indexes := manager.ListIndexes()

	results := make([]*resp.Value, 0, len(indexes))
	for _, name := range indexes {
		results = append(results, resp.BulkString(name))
	}

	return ctx.WriteArray(results)
}

func cmdFTAGGREGATE(ctx *Context) error {
	if ctx.ArgCount() < 2 {
		return ctx.WriteError(ErrWrongArgCount)
	}

	indexName := ctx.ArgString(0)
	query := ctx.ArgString(1)

	manager := search.GetIndexManager()
	idx, ok := manager.GetIndex(indexName)
	if !ok {
		return ctx.WriteError(fmt.Errorf("ERR index not found"))
	}

	result := idx.Search(query, 100, 0)

	results := []*resp.Value{
		resp.IntegerValue(int64(result.Total)),
	}

	for _, doc := range result.Documents {
		row := []*resp.Value{resp.BulkString(doc.ID)}
		for fname, fval := range doc.Fields {
			row = append(row, resp.BulkString(fname), resp.BulkString(fval))
		}
		results = append(results, resp.ArrayValue(row))
	}

	return ctx.WriteArray(results)
}

func cmdFTTAGVALS(ctx *Context) error {
	if ctx.ArgCount() < 2 {
		return ctx.WriteError(ErrWrongArgCount)
	}

	indexName := ctx.ArgString(0)
	_ = indexName
	fieldName := ctx.ArgString(1)
	_ = fieldName

	return ctx.WriteArray([]*resp.Value{})
}

func cmdFTALIASADD(ctx *Context) error {
	if ctx.ArgCount() < 2 {
		return ctx.WriteError(ErrWrongArgCount)
	}

	alias := ctx.ArgString(0)
	index := ctx.ArgString(1)

	manager := search.GetIndexManager()
	if _, ok := manager.GetIndex(index); !ok {
		return ctx.WriteError(fmt.Errorf("ERR index not found"))
	}

	_ = alias
	return ctx.WriteOK()
}

func cmdFTALIASDEL(ctx *Context) error {
	if ctx.ArgCount() < 1 {
		return ctx.WriteError(ErrWrongArgCount)
	}

	alias := ctx.ArgString(0)
	_ = alias

	return ctx.WriteOK()
}

func contains(slice []string, item string) bool {
	for _, s := range slice {
		if s == item {
			return true
		}
	}
	return false
}

func init() {
	_ = strconv.Itoa(0)
}
