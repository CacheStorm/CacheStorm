package command

import (
	"encoding/binary"
	"errors"
	"fmt"
	"math"
	"sort"
	"strconv"
	"strings"

	"github.com/cachestorm/cachestorm/internal/resp"
	"github.com/cachestorm/cachestorm/internal/store"
)

// A VECTOR store is a hash key holding its dimension in the field __vector,
// whether it normalizes in __normalize, and each vector as the field v:<id>
// holding its components as little-endian float64s. Being a key, a store is
// per database, persisted and replicated; changes are recorded as HSET, HDEL
// and DEL. Searches scan the store.

const (
	vectorMarker    = "__vector"
	vectorNormalize = "__normalize"
	vectorPrefix    = "v:"
	maxVectorDim    = 1 << 16
)

var (
	errVectorStoreNotFound = errors.New("ERR vector store not found")
	errVectorNotFound      = errors.New("ERR vector not found")
	errVectorComponent     = errors.New("ERR vector components must be finite floats")
)

type vectorStore struct {
	dim       int
	normalize bool
	vectors   map[string][]float64
}

// readVectorStore returns the store at name, nil when the key is missing.
func readVectorStore(ctx *Context, name string) (*vectorStore, error) {
	h, err := readMarkedHash(ctx, name, vectorMarker)
	if err != nil || h == nil {
		return nil, err
	}
	h.RLock()
	fields := h.Fields()
	h.RUnlock()
	dim, err := strconv.Atoi(string(fields[vectorMarker]))
	if err != nil || dim < 1 || dim > maxVectorDim {
		return nil, store.ErrWrongType
	}
	s := &vectorStore{dim: dim, normalize: string(fields[vectorNormalize]) == "1", vectors: map[string][]float64{}}
	for f, raw := range fields {
		switch {
		case f == vectorMarker || f == vectorNormalize:
		case strings.HasPrefix(f, vectorPrefix) && len(raw) == 8*dim:
			v := make([]float64, dim)
			for i := range v {
				v[i] = math.Float64frombits(binary.LittleEndian.Uint64(raw[8*i:]))
			}
			s.vectors[f[len(vectorPrefix):]] = v
		default:
			return nil, store.ErrWrongType
		}
	}
	return s, nil
}

func loadVectorStore(ctx *Context, name string) (*vectorStore, error) {
	s, err := readVectorStore(ctx, name)
	if err == nil && s == nil {
		err = errVectorStoreNotFound
	}
	return s, err
}

func (s *vectorStore) pair(id1, id2 string) ([]float64, []float64, error) {
	a, ok1 := s.vectors[id1]
	b, ok2 := s.vectors[id2]
	if !ok1 || !ok2 {
		return nil, nil, errVectorNotFound
	}
	return a, b, nil
}

func encodeVector(v []float64) string {
	buf := make([]byte, 8*len(v))
	for i, x := range v {
		binary.LittleEndian.PutUint64(buf[8*i:], math.Float64bits(x))
	}
	return string(buf)
}

func normalizeVector(v []float64) {
	norm := 0.0
	for _, x := range v {
		norm += x * x
	}
	if norm = math.Sqrt(norm); norm > 0 {
		for i := range v {
			v[i] /= norm
		}
	}
}

// parseVector reads n components from the arguments starting at from.
func parseVector(ctx *Context, from, n int) ([]float64, error) {
	if ctx.ArgCount()-from != n {
		return nil, fmt.Errorf("ERR vector dimension mismatch: got %d values for dimension %d", ctx.ArgCount()-from, n)
	}
	v := make([]float64, n)
	for i := range v {
		x, err := strconv.ParseFloat(ctx.ArgString(from+i), 64)
		if err != nil || math.IsNaN(x) || math.IsInf(x, 0) {
			return nil, errVectorComponent
		}
		v[i] = x
	}
	return v, nil
}

func parseVectorDim(arg string) (int, error) {
	dim, err := strconv.Atoi(arg)
	if err != nil || dim < 1 || dim > maxVectorDim {
		return 0, fmt.Errorf("ERR dimension must be between 1 and %d", maxVectorDim)
	}
	return dim, nil
}

// VECTOR.CREATE name dim [NORMALIZE] makes an empty store, replacing one
// already at name.
func cmdVECTORCREATE(ctx *Context) error {
	if ctx.ArgCount() < 2 || ctx.ArgCount() > 3 {
		return ctx.WriteError(ErrWrongArgCount)
	}
	name := ctx.ArgString(0)
	dim, err := parseVectorDim(ctx.ArgString(1))
	if err != nil {
		return ctx.WriteError(err)
	}
	normalize := "0"
	if ctx.ArgCount() == 3 {
		if !strings.EqualFold(ctx.ArgString(2), "NORMALIZE") {
			return ctx.WriteError(ErrSyntaxError)
		}
		normalize = "1"
	}
	if _, err := readVectorStore(ctx, name); err != nil {
		return ctx.WriteError(err)
	}
	if err := replaceRecord(ctx, name, vectorMarker, strconv.Itoa(dim), vectorNormalize, normalize); err != nil {
		return ctx.WriteError(err)
	}
	return ctx.WriteOK()
}

// VECTOR.ADD name id dim v1 ... vdim stores the vector, replacing one with
// the same id; a normalizing store keeps it at unit length.
func cmdVECTORADD(ctx *Context) error {
	if ctx.ArgCount() < 4 {
		return ctx.WriteError(ErrWrongArgCount)
	}
	name, id := ctx.ArgString(0), ctx.ArgString(1)
	dim, err := parseVectorDim(ctx.ArgString(2))
	if err != nil {
		return ctx.WriteError(err)
	}
	v, err := parseVector(ctx, 3, dim)
	if err != nil {
		return ctx.WriteError(err)
	}
	s, err := loadVectorStore(ctx, name)
	if err != nil {
		return ctx.WriteError(err)
	}
	if dim != s.dim {
		return ctx.WriteError(fmt.Errorf("ERR vector dimension mismatch: expected %d", s.dim))
	}
	if s.normalize {
		normalizeVector(v)
	}
	if err := updateRecord(ctx, name, vectorPrefix+id, encodeVector(v)); err != nil {
		return ctx.WriteError(err)
	}
	return ctx.WriteOK()
}

func formatComponent(x float64) *resp.Value { return resp.BulkString(fmt.Sprintf("%.6f", x)) }

func cmdVECTORGET(ctx *Context) error {
	if ctx.ArgCount() != 2 {
		return ctx.WriteError(ErrWrongArgCount)
	}
	s, err := loadVectorStore(ctx, ctx.ArgString(0))
	if err != nil {
		return ctx.WriteError(err)
	}
	v, ok := s.vectors[ctx.ArgString(1)]
	if !ok {
		return ctx.WriteNull()
	}
	out := make([]*resp.Value, len(v))
	for i, x := range v {
		out[i] = formatComponent(x)
	}
	return ctx.WriteArray(out)
}

func cmdVECTORDELETE(ctx *Context) error {
	if ctx.ArgCount() != 2 {
		return ctx.WriteError(ErrWrongArgCount)
	}
	name := ctx.ArgString(0)
	if _, err := loadVectorStore(ctx, name); err != nil {
		return ctx.WriteError(err)
	}
	return ctx.WriteInteger(int64(deleteMarkedFields(ctx, name, vectorMarker, vectorPrefix+ctx.ArgString(1))))
}

// VECTOR.SEARCH name k dim v1 ... vdim replies the k vectors most similar to
// the query by cosine similarity, most similar first, ties by id.
func cmdVECTORSEARCH(ctx *Context) error {
	if ctx.ArgCount() < 4 {
		return ctx.WriteError(ErrWrongArgCount)
	}
	k, err := strconv.Atoi(ctx.ArgString(1))
	if err != nil || k < 0 {
		return ctx.WriteError(errors.New("ERR k must be a non-negative integer"))
	}
	dim, err := parseVectorDim(ctx.ArgString(2))
	if err != nil {
		return ctx.WriteError(err)
	}
	query, err := parseVector(ctx, 3, dim)
	if err != nil {
		return ctx.WriteError(err)
	}
	s, err := loadVectorStore(ctx, ctx.ArgString(0))
	if err != nil {
		return ctx.WriteError(err)
	}
	if dim != s.dim {
		return ctx.WriteError(fmt.Errorf("ERR vector dimension mismatch: expected %d", s.dim))
	}
	type hit struct {
		id    string
		score float64
	}
	hits := make([]hit, 0, len(s.vectors))
	for id, v := range s.vectors {
		hits = append(hits, hit{id, cosineSimilarity(query, v)})
	}
	sort.Slice(hits, func(i, j int) bool {
		if hits[i].score != hits[j].score {
			return hits[i].score > hits[j].score
		}
		return hits[i].id < hits[j].id
	})
	out := make([]*resp.Value, 0, min(k, len(hits)))
	for _, h := range hits[:min(k, len(hits))] {
		out = append(out, resp.ArrayValue([]*resp.Value{
			resp.BulkString("id"), resp.BulkString(h.id),
			resp.BulkString("score"), formatComponent(h.score),
		}))
	}
	return ctx.WriteArray(out)
}

func cosineSimilarity(a, b []float64) float64 {
	if len(a) != len(b) {
		return 0
	}
	var dot, normA, normB float64
	for i := range a {
		dot += a[i] * b[i]
		normA += a[i] * a[i]
		normB += b[i] * b[i]
	}
	if normA == 0 || normB == 0 {
		return 0
	}
	return dot / (math.Sqrt(normA) * math.Sqrt(normB))
}

func cmdVECTORSIMILARITY(ctx *Context) error {
	if ctx.ArgCount() != 3 {
		return ctx.WriteError(ErrWrongArgCount)
	}
	s, err := loadVectorStore(ctx, ctx.ArgString(0))
	if err != nil {
		return ctx.WriteError(err)
	}
	a, b, err := s.pair(ctx.ArgString(1), ctx.ArgString(2))
	if err != nil {
		return ctx.WriteError(err)
	}
	return ctx.WriteBulkString(fmt.Sprintf("%.6f", cosineSimilarity(a, b)))
}

func cmdVECTORNORMALIZE(ctx *Context) error {
	if ctx.ArgCount() != 2 {
		return ctx.WriteError(ErrWrongArgCount)
	}
	name, id := ctx.ArgString(0), ctx.ArgString(1)
	s, err := loadVectorStore(ctx, name)
	if err != nil {
		return ctx.WriteError(err)
	}
	v, ok := s.vectors[id]
	if !ok {
		return ctx.WriteError(errVectorNotFound)
	}
	normalizeVector(v)
	if err := updateRecord(ctx, name, vectorPrefix+id, encodeVector(v)); err != nil {
		return ctx.WriteError(err)
	}
	return ctx.WriteOK()
}

func cmdVECTORDIMENSIONS(ctx *Context) error {
	if ctx.ArgCount() != 1 {
		return ctx.WriteError(ErrWrongArgCount)
	}
	s, err := loadVectorStore(ctx, ctx.ArgString(0))
	if err != nil {
		return ctx.WriteError(err)
	}
	return ctx.WriteInteger(int64(s.dim))
}

// VECTOR.MERGE name id1 id2 [newid] stores the mean of two vectors as newid
// (id1_id2 by default) and replies its id.
func cmdVECTORMERGE(ctx *Context) error {
	if ctx.ArgCount() < 3 || ctx.ArgCount() > 4 {
		return ctx.WriteError(ErrWrongArgCount)
	}
	name, id1, id2 := ctx.ArgString(0), ctx.ArgString(1), ctx.ArgString(2)
	newID := id1 + "_" + id2
	if ctx.ArgCount() == 4 {
		newID = ctx.ArgString(3)
	}
	s, err := loadVectorStore(ctx, name)
	if err != nil {
		return ctx.WriteError(err)
	}
	a, b, err := s.pair(id1, id2)
	if err != nil {
		return ctx.WriteError(err)
	}
	merged := make([]float64, len(a))
	for i := range a {
		merged[i] = (a[i] + b[i]) / 2
	}
	if s.normalize {
		normalizeVector(merged)
	}
	if err := updateRecord(ctx, name, vectorPrefix+newID, encodeVector(merged)); err != nil {
		return ctx.WriteError(err)
	}
	return ctx.WriteBulkString(newID)
}

func cmdVECTORSTATS(ctx *Context) error {
	if ctx.ArgCount() != 1 {
		return ctx.WriteError(ErrWrongArgCount)
	}
	name := ctx.ArgString(0)
	s, err := loadVectorStore(ctx, name)
	if err != nil {
		return ctx.WriteError(err)
	}
	return ctx.WriteArray([]*resp.Value{
		resp.BulkString("name"), resp.BulkString(name),
		resp.BulkString("dimensions"), resp.IntegerValue(int64(s.dim)),
		resp.BulkString("vectors"), resp.IntegerValue(int64(len(s.vectors))),
		resp.BulkString("normalize"), resp.BulkString(strconv.FormatBool(s.normalize)),
	})
}
