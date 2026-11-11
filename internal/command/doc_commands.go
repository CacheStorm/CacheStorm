package command

import (
	"encoding/json"
	"errors"
	"sort"
	"strings"

	"github.com/cachestorm/cachestorm/internal/resp"
	"github.com/cachestorm/cachestorm/internal/store"
)

// A DOC store is a hash key marked by the field __doc. Each document is the
// field d:<id> holding the document's fields as a JSON object, and each
// declared index is the field i:<field>. Being a key, a store is per
// database, persisted and replicated; inserts and updates are recorded as
// HSET, deletes as HDEL. Lookups by field scan the store's documents; an
// index declaration is kept with the store for clients that manage them.

const (
	docMarker      = "__doc"
	docFieldPrefix = "d:"
	docIndexPrefix = "i:"
)

var (
	errDocStoreNotFound = errors.New("ERR doc store not found")
	errDocNotFound      = errors.New("ERR document not found")
)

type docStore struct {
	docs    map[string]map[string]string
	indexes map[string]bool
}

// readDocStore returns the store at name, nil when it does not exist.
func readDocStore(ctx *Context, name string) (*docStore, error) {
	h, err := readMarkedHash(ctx, name, docMarker)
	if err != nil || h == nil {
		return nil, err
	}
	s := &docStore{docs: map[string]map[string]string{}, indexes: map[string]bool{}}
	for f, raw := range markedFields(h, docMarker) {
		switch {
		case strings.HasPrefix(f, docFieldPrefix):
			doc := map[string]string{}
			if json.Unmarshal(raw, &doc) != nil {
				return nil, store.ErrWrongType
			}
			s.docs[f[len(docFieldPrefix):]] = doc
		case strings.HasPrefix(f, docIndexPrefix):
			s.indexes[f[len(docIndexPrefix):]] = true
		default:
			return nil, store.ErrWrongType
		}
	}
	return s, nil
}

// ids returns the ids of the documents, or of those whose field equals
// value when field is set, sorted.
func (s *docStore) ids(field, value string, filter bool) []string {
	if s == nil {
		return nil
	}
	ids := make([]string, 0, len(s.docs))
	for id, doc := range s.docs {
		if v, ok := doc[field]; !filter || (ok && v == value) {
			ids = append(ids, id)
		}
	}
	sort.Strings(ids)
	return ids
}

func docReply(id string, doc map[string]string) []*resp.Value {
	fields := make([]string, 0, len(doc))
	for k := range doc {
		fields = append(fields, k)
	}
	sort.Strings(fields)
	out := make([]*resp.Value, 0, 2+2*len(fields))
	out = append(out, resp.BulkString("_id"), resp.BulkString(id))
	for _, k := range fields {
		out = append(out, resp.BulkString(k), resp.BulkString(doc[k]))
	}
	return out
}

func saveDoc(ctx *Context, name, id string, doc map[string]string) error {
	raw, err := json.Marshal(doc)
	if err != nil {
		return err
	}
	return setMarkedFields(ctx, name, docMarker, docFieldPrefix+id, string(raw))
}

// DOC.INSERT name id field value [field value ...] stores the document,
// replacing one with the same id.
func cmdDOCINSERT(ctx *Context) error {
	if ctx.ArgCount() < 4 || (ctx.ArgCount()-2)%2 != 0 {
		return ctx.WriteError(ErrWrongArgCount)
	}
	name, id := ctx.ArgString(0), ctx.ArgString(1)
	if _, err := readDocStore(ctx, name); err != nil {
		return ctx.WriteError(err)
	}
	doc := make(map[string]string)
	for i := 2; i+1 < ctx.ArgCount(); i += 2 {
		doc[ctx.ArgString(i)] = ctx.ArgString(i + 1)
	}
	if err := saveDoc(ctx, name, id, doc); err != nil {
		return ctx.WriteError(err)
	}
	return ctx.WriteBulkString(id)
}

func cmdDOCFIND(ctx *Context) error {
	if ctx.ArgCount() != 1 && ctx.ArgCount() != 3 {
		return ctx.WriteError(ErrWrongArgCount)
	}
	s, err := readDocStore(ctx, ctx.ArgString(0))
	if err != nil {
		return ctx.WriteError(err)
	}
	var ids []string
	if ctx.ArgCount() == 3 {
		ids = s.ids(ctx.ArgString(1), ctx.ArgString(2), true)
	} else {
		ids = s.ids("", "", false)
	}
	results := make([]*resp.Value, 0, len(ids))
	for _, id := range ids {
		results = append(results, resp.ArrayValue(docReply(id, s.docs[id])))
	}
	return ctx.WriteArray(results)
}

func cmdDOCFINDONE(ctx *Context) error {
	if ctx.ArgCount() != 3 {
		return ctx.WriteError(ErrWrongArgCount)
	}
	s, err := readDocStore(ctx, ctx.ArgString(0))
	if err != nil {
		return ctx.WriteError(err)
	}
	ids := s.ids(ctx.ArgString(1), ctx.ArgString(2), true)
	if len(ids) == 0 {
		return ctx.WriteNull()
	}
	return ctx.WriteArray(docReply(ids[0], s.docs[ids[0]]))
}

// DOC.UPDATE name id [field value ...] sets fields of an existing document.
func cmdDOCUPDATE(ctx *Context) error {
	if ctx.ArgCount() < 2 || (ctx.ArgCount()-2)%2 != 0 {
		return ctx.WriteError(ErrWrongArgCount)
	}
	name, id := ctx.ArgString(0), ctx.ArgString(1)
	s, err := readDocStore(ctx, name)
	if err != nil {
		return ctx.WriteError(err)
	}
	if s == nil {
		return ctx.WriteError(errDocStoreNotFound)
	}
	doc, ok := s.docs[id]
	if !ok {
		return ctx.WriteError(errDocNotFound)
	}
	if ctx.ArgCount() == 2 {
		ctx.PropagateAs()
		return ctx.WriteOK()
	}
	for i := 2; i+1 < ctx.ArgCount(); i += 2 {
		doc[ctx.ArgString(i)] = ctx.ArgString(i + 1)
	}
	if err := saveDoc(ctx, name, id, doc); err != nil {
		return ctx.WriteError(err)
	}
	return ctx.WriteOK()
}

func cmdDOCDELETE(ctx *Context) error {
	if ctx.ArgCount() != 2 {
		return ctx.WriteError(ErrWrongArgCount)
	}
	name := ctx.ArgString(0)
	if _, err := readDocStore(ctx, name); err != nil {
		return ctx.WriteError(err)
	}
	return ctx.WriteInteger(int64(deleteMarkedFields(ctx, name, docMarker, docFieldPrefix+ctx.ArgString(1))))
}

func cmdDOCCOUNT(ctx *Context) error {
	if ctx.ArgCount() != 1 {
		return ctx.WriteError(ErrWrongArgCount)
	}
	s, err := readDocStore(ctx, ctx.ArgString(0))
	if err != nil {
		return ctx.WriteError(err)
	}
	if s == nil {
		return ctx.WriteInteger(0)
	}
	return ctx.WriteInteger(int64(len(s.docs)))
}

func sortedBulk(set map[string]struct{}) []*resp.Value {
	values := make([]string, 0, len(set))
	for v := range set {
		values = append(values, v)
	}
	sort.Strings(values)
	out := make([]*resp.Value, len(values))
	for i, v := range values {
		out[i] = resp.BulkString(v)
	}
	return out
}

func docDOCDISTINCT(ctx *Context) error {
	if ctx.ArgCount() != 2 {
		return ctx.WriteError(ErrWrongArgCount)
	}
	s, err := readDocStore(ctx, ctx.ArgString(0))
	if err != nil {
		return ctx.WriteError(err)
	}
	seen := make(map[string]struct{})
	if s != nil {
		for _, doc := range s.docs {
			if value, ok := doc[ctx.ArgString(1)]; ok {
				seen[value] = struct{}{}
			}
		}
	}
	return ctx.WriteArray(sortedBulk(seen))
}

// DOC.AGGREGATE name COUNT|FIELDS replies the document count or the field
// names the documents use.
func cmdDOCAGGREGATE(ctx *Context) error {
	if ctx.ArgCount() != 2 {
		return ctx.WriteError(ErrWrongArgCount)
	}
	aggType := strings.ToLower(ctx.ArgString(1))
	if aggType != "count" && aggType != "fields" {
		return ctx.WriteError(errors.New("ERR unknown aggregation type"))
	}
	s, err := readDocStore(ctx, ctx.ArgString(0))
	if err != nil {
		return ctx.WriteError(err)
	}
	if aggType == "count" {
		if s == nil {
			return ctx.WriteInteger(0)
		}
		return ctx.WriteInteger(int64(len(s.docs)))
	}
	fields := make(map[string]struct{})
	if s != nil {
		for _, doc := range s.docs {
			for f := range doc {
				fields[f] = struct{}{}
			}
		}
	}
	return ctx.WriteArray(sortedBulk(fields))
}

// DOC.INDEX name field declares an index on field, creating the store.
func cmdDOCINDEX(ctx *Context) error {
	if ctx.ArgCount() != 2 {
		return ctx.WriteError(ErrWrongArgCount)
	}
	name := ctx.ArgString(0)
	s, err := readDocStore(ctx, name)
	if err != nil {
		return ctx.WriteError(err)
	}
	if s != nil && s.indexes[ctx.ArgString(1)] {
		ctx.PropagateAs()
		return ctx.WriteOK()
	}
	if err := setMarkedFields(ctx, name, docMarker, docIndexPrefix+ctx.ArgString(1), ""); err != nil {
		return ctx.WriteError(err)
	}
	return ctx.WriteOK()
}

func cmdDOCDROPINDEX(ctx *Context) error {
	if ctx.ArgCount() != 2 {
		return ctx.WriteError(ErrWrongArgCount)
	}
	name := ctx.ArgString(0)
	s, err := readDocStore(ctx, name)
	if err != nil {
		return ctx.WriteError(err)
	}
	if s == nil {
		return ctx.WriteError(errDocStoreNotFound)
	}
	return ctx.WriteInteger(int64(deleteMarkedFields(ctx, name, docMarker, docIndexPrefix+ctx.ArgString(1))))
}
