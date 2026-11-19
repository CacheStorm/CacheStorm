package command

import (
	"encoding/binary"
	"errors"
	"fmt"
	"math"
	"math/rand/v2"
	"strings"

	"github.com/cachestorm/cachestorm/internal/resp"
	"github.com/cachestorm/cachestorm/internal/store"
	"github.com/cachestorm/cachestorm/internal/vectorset"
)

// Redis 8's vector sets (the vectorset module of Redis 8.2), ported from
// modules/vector-sets/vset.c: replies, errors and defaults follow it.
func RegisterVectorSetCommands(router *Router) {
	for name, h := range map[string]func(*Context) error{
		"VADD": cmdVADD, "VREM": cmdVREM, "VSIM": cmdVSIM, "VDIM": cmdVDIM,
		"VCARD": cmdVCARD, "VEMB": cmdVEMB, "VLINKS": cmdVLINKS, "VINFO": cmdVINFO,
		"VSETATTR": cmdVSETATTR, "VGETATTR": cmdVGETATTR, "VRANDMEMBER": cmdVRANDMEMBER,
		"VISMEMBER": cmdVISMEMBER,
	} {
		router.Register(&CommandDef{Name: name, Handler: h})
	}
}

const (
	vsetDefaultBuildEF  = 200
	vsetDefaultSearchEF = 100
	vsetDefaultCount    = 10
)

var (
	errVsetVector = errors.New("ERR invalid vector specification")
)

func getVectorSet(ctx *Context, key string) (*store.VectorSetValue, error) {
	v, _, err := lookupValue[*store.VectorSetValue](ctx, key)
	return v, err
}

// vsetBool replies a boolean, an integer for RESP2.
func vsetBool(ctx *Context, b bool) error {
	return ctx.WriteValue(resp.BooleanValue(b))
}

func vsetDouble(f float64) *resp.Value {
	return resp.DoubleValue(formatZScore(f))
}

// parseVsetDouble parses as Redis's RedisModule_StringToDouble does.
func parseVsetDouble(b []byte) (float64, bool) {
	if len(b) == 0 || b[0] == ' ' || b[0] == '\t' || b[0] == '\n' || b[0] == '\r' || b[0] == '\v' || b[0] == '\f' {
		return 0, false
	}
	return parseZScore(string(b))
}

// parseVsetVector parses [REDUCE dim] FP32 blob | VALUES n v1 ... vn from
// argument start (an index into ctx.Args, which excludes the command name),
// as vset.c's parseVector does. It returns the vector, the REDUCE
// dimension (0 without one) and the arguments consumed; ok is false for
// any error.
func parseVsetVector(ctx *Context, start int, allowReduce bool) (vec []float32, reduce uint32, consumed int, ok bool) {
	argc := ctx.ArgCount()
	if allowReduce && argc > start+2 && strings.EqualFold(ctx.ArgString(start), "REDUCE") {
		r, valid := parseRedisInt(ctx.Arg(start + 1))
		if !valid || r <= 0 {
			return nil, 0, 0, false
		}
		reduce = uint32(r)
		if r > math.MaxUint32 {
			reduce = math.MaxUint32
		}
		start += 2
		consumed += 2
	}
	if start >= argc {
		return nil, 0, 0, false
	}
	switch format := ctx.ArgString(start); {
	case strings.EqualFold(format, "FP32"):
		if argc < start+2 {
			return nil, 0, 0, false
		}
		blob := ctx.Arg(start + 1)
		if len(blob)%4 != 0 || len(blob) < 4 || len(blob)/4 > store.MaxVectorDim {
			return nil, 0, 0, false
		}
		vec = make([]float32, len(blob)/4)
		for i := range vec {
			vec[i] = math.Float32frombits(binary.LittleEndian.Uint32(blob[i*4:]))
		}
		consumed += 2
	case strings.EqualFold(format, "VALUES"):
		if argc < start+2 {
			return nil, 0, 0, false
		}
		n, valid := parseRedisInt(ctx.Arg(start + 1))
		if !valid || n < 1 || n > store.MaxVectorDim || int64(argc) < int64(start)+2+n {
			return nil, 0, 0, false
		}
		vec = make([]float32, n)
		for i := range vec {
			f, valid := parseVsetDouble(ctx.Arg(start + 2 + i))
			if !valid {
				return nil, 0, 0, false
			}
			vec[i] = float32(f)
		}
		consumed += int(n) + 2
	default:
		return nil, 0, 0, false
	}
	if reduce > uint32(len(vec)) {
		return nil, 0, 0, false
	}
	return vec, reduce, consumed, true
}

// VADD key [REDUCE dim] FP32|VALUES vector element [CAS] [NOQUANT|Q8|BIN]
// [EF n] [SETATTR attrs] [M n]
func cmdVADD(ctx *Context) error {
	if ctx.ArgCount() < 4 {
		return ctx.WriteError(arityError("vadd"))
	}
	key := ctx.ArgString(0)
	vec, reduce, consumed, ok := parseVsetVector(ctx, 1, true)
	if !ok {
		return ctx.WriteError(errVsetVector)
	}
	if ctx.ArgCount()-1-consumed < 1 {
		return ctx.WriteError(arityError("vadd"))
	}
	ef := int64(vsetDefaultBuildEF)
	m := int64(vectorset.DefaultM)
	quant := vectorset.QuantQ8
	var attr []byte
	for j := 1 + consumed + 1; j < ctx.ArgCount(); j++ {
		opt := ctx.ArgString(j)
		switch {
		case strings.EqualFold(opt, "CAS"):
		case strings.EqualFold(opt, "EF") && j+1 < ctx.ArgCount():
			n, valid := parseRedisInt(ctx.Arg(j + 1))
			if !valid || n <= 0 || n > 1000000 {
				return ctx.WriteError(errors.New("ERR invalid EF"))
			}
			ef = n
			j++
		case strings.EqualFold(opt, "M") && j+1 < ctx.ArgCount():
			n, valid := parseRedisInt(ctx.Arg(j + 1))
			if !valid || n < vectorset.MinM || n > vectorset.MaxM {
				return ctx.WriteError(errors.New("ERR invalid M"))
			}
			m = n
			j++
		case strings.EqualFold(opt, "SETATTR") && j+1 < ctx.ArgCount():
			attr = ctx.Arg(j + 1)
			if attr == nil {
				attr = []byte{}
			}
			j++
		case strings.EqualFold(opt, "NOQUANT"):
			quant = vectorset.QuantNone
		case strings.EqualFold(opt, "BIN"):
			quant = vectorset.QuantBin
		case strings.EqualFold(opt, "Q8"):
			quant = vectorset.QuantQ8
		default:
			return ctx.WriteError(errors.New("ERR invalid option after element"))
		}
	}
	elem := ctx.ArgString(1 + consumed)
	v, err := getVectorSet(ctx, key)
	if err != nil {
		return ctx.WriteError(err)
	}
	dim := uint32(len(vec))
	if v == nil {
		setDim := dim
		if reduce != 0 {
			setDim = reduce
		}
		v = store.NewVectorSet(setDim, quant, uint32(m))
		if reduce != 0 {
			v.Proj = store.ProjectionMatrix(dim, reduce)
			v.ProjInput = dim
			vec = v.Project(vec)
		}
		v.Insert(elem, vec, attr, int(ef))
		if err := ctx.Store.Set(key, v, store.SetOptions{}); err != nil {
			return ctx.WriteError(err)
		}
		return vsetBool(ctx, true)
	}
	v.Lock()
	defer v.Unlock()
	ix := v.Index
	switch {
	case ix.Quant() != quant:
		return ctx.WriteError(errors.New("ERR asked quantization mismatch with existing vector set"))
	case ix.M() != uint32(m):
		return ctx.WriteError(errors.New("ERR asked M value mismatch with existing vector set"))
	case v.Proj == nil && ix.Dim() != dim || v.Proj != nil && ix.Dim() != reduce:
		return ctx.WriteError(fmt.Errorf("ERR Vector dimension mismatch - got %d but set has %d", dim, ix.Dim()))
	}
	if reduce != 0 && v.Proj == nil {
		return ctx.WriteError(errors.New("ERR cannot add projection to existing set without projection"))
	}
	if v.Proj != nil {
		if dim != v.ProjInput {
			return ctx.WriteError(fmt.Errorf("ERR Input dimension mismatch for projection - got %d but projection expects %d", dim, v.ProjInput))
		}
		vec = v.Project(vec)
	}
	// Redis replicates only an added element; an update changes the vector
	// and attributes too, so it is recorded as well.
	return vsetBool(ctx, v.Insert(elem, vec, attr, int(ef)))
}

// VSIM key ELE|FP32|VALUES vector-or-element [WITHSCORES] [WITHATTRIBS]
// [COUNT n] [EPSILON d] [EF n] [FILTER expr] [FILTER-EF n] [TRUTH] [NOTHREAD]
func cmdVSIM(ctx *Context) error {
	if ctx.ArgCount() < 3 {
		return ctx.WriteError(arityError("vsim"))
	}
	v, err := getVectorSet(ctx, ctx.ArgString(0))
	if err != nil {
		return ctx.WriteError(err)
	}
	if v == nil {
		return ctx.WriteArray([]*resp.Value{})
	}
	v.RLock()
	defer v.RUnlock()
	ix := v.Index
	var vec []float32
	vectorArgs := 0
	vtype := ctx.ArgString(1)
	if strings.EqualFold(vtype, "ELE") {
		n, ok := v.Elems[ctx.ArgString(2)]
		if !ok {
			return ctx.WriteError(errors.New("ERR element not found in set"))
		}
		vec = ix.NodeVector(n)
		vectorArgs = 2
	} else {
		// The arguments consumed are counted again below from the format,
		// as Redis does.
		parsed, _, _, ok := parseVsetVector(ctx, 1, false)
		if !ok {
			return ctx.WriteError(errVsetVector)
		}
		vec = parsed
		if v.Proj != nil && uint32(len(vec)) != ix.Dim() {
			if uint32(len(vec)) != v.ProjInput {
				return ctx.WriteError(fmt.Errorf("ERR Input dimension mismatch for projection - got %d but projection expects %d", len(vec), v.ProjInput))
			}
			vec = v.Project(vec)
		}
		switch {
		case strings.EqualFold(vtype, "FP32"):
			vectorArgs = 2
		case strings.EqualFold(vtype, "VALUES"):
			n, ok := parseRedisInt(ctx.Arg(2))
			if !ok {
				return ctx.WriteError(errors.New("ERR invalid vector dimension"))
			}
			vectorArgs = 2 + int(n)
		default:
			return ctx.WriteError(errors.New("ERR vector type must be ELE, FP32 or VALUES"))
		}
	}
	if uint32(len(vec)) != ix.Dim() {
		return ctx.WriteError(fmt.Errorf("ERR Vector dimension mismatch - got %d but set has %d", len(vec), ix.Dim()))
	}
	withScores, withAttribs, truth := false, false, false
	count, ef, filterEF := int64(vsetDefaultCount), int64(0), int64(0)
	epsilon := float32(2.0)
	var filter *vectorset.Expr
	for j := 1 + vectorArgs; j < ctx.ArgCount(); {
		opt := ctx.ArgString(j)
		more := j+1 < ctx.ArgCount()
		switch {
		case strings.EqualFold(opt, "WITHSCORES"):
			withScores = true
			j++
		case strings.EqualFold(opt, "WITHATTRIBS"):
			withAttribs = true
			j++
		case strings.EqualFold(opt, "TRUTH"):
			truth = true
			j++
		case strings.EqualFold(opt, "NOTHREAD"):
			j++
		case strings.EqualFold(opt, "COUNT") && more:
			n, ok := parseRedisInt(ctx.Arg(j + 1))
			if !ok || n <= 0 {
				return ctx.WriteError(errors.New("ERR invalid COUNT"))
			}
			count = n
			j += 2
		case strings.EqualFold(opt, "EPSILON") && more:
			f, ok := parseVsetDouble(ctx.Arg(j + 1))
			if !ok || f <= 0 {
				return ctx.WriteError(errors.New("ERR invalid EPSILON"))
			}
			epsilon = float32(f)
			j += 2
		case strings.EqualFold(opt, "EF") && more:
			n, ok := parseRedisInt(ctx.Arg(j + 1))
			if !ok || n <= 0 || n > 1000000 {
				return ctx.WriteError(errors.New("ERR invalid EF"))
			}
			ef = n
			j += 2
		case strings.EqualFold(opt, "FILTER-EF") && more:
			n, ok := parseRedisInt(ctx.Arg(j + 1))
			if !ok || n <= 0 {
				return ctx.WriteError(errors.New("ERR invalid FILTER-EF"))
			}
			filterEF = n
			j += 2
		case strings.EqualFold(opt, "FILTER") && more:
			src := ctx.ArgString(j + 1)
			e, errpos := vectorset.Compile(src)
			if e == nil {
				if errpos >= len(src) || errpos < 0 {
					errpos = 0
				}
				near := src[errpos:]
				if i := strings.IndexByte(near, 0); i >= 0 {
					near = near[:i]
				}
				near = strings.TrimRight(near, "\r\n")
				near = strings.NewReplacer("\r", " ", "\n", " ").Replace(near)
				return ctx.WriteError(errors.New("ERR syntax error in FILTER expression near: " + near))
			}
			filter = e
			j += 2
		default:
			return ctx.WriteError(errors.New("ERR syntax error in VSIM command"))
		}
	}
	if filterEF == 0 {
		filterEF = count * 100
	}
	if ef == 0 {
		ef = vsetDefaultSearchEF
	}
	if count > ef {
		ef = count
	}
	if uint64(ef) > ix.Len() {
		ef = int64(ix.Len())
	}
	var match func(any) bool
	if filter != nil {
		match = func(value any) bool {
			e := value.(*store.VectorElem)
			return e.Attr != nil && filter.Match(e.Attr)
		}
	}
	var nodes []*vectorset.Node
	var dists []float32
	switch {
	case truth:
		nodes, dists = ix.GroundTruth(vec, int(ef), match)
	case match == nil:
		nodes, dists = ix.Search(vec, int(ef), nil, 0)
	default:
		nodes, dists = ix.Search(vec, int(ef), match, int(uint32(filterEF)))
	}
	resp3 := ctx.Writer.IsRESP3()
	var out []*resp.Value
	for i := 0; i < len(nodes) && int64(i) < count; i++ {
		if dists[i]/2 > epsilon {
			break
		}
		e := nodes[i].Value.(*store.VectorElem)
		out = append(out, resp.BulkString(e.Name))
		var props []*resp.Value
		if withScores {
			props = append(props, vsetDouble(1.0-float64(dists[i])/2.0))
		}
		if withAttribs {
			if e.Attr != nil {
				props = append(props, resp.BulkBytes(e.Attr))
			} else {
				props = append(props, resp.NullBulkString())
			}
		}
		switch {
		case resp3 && len(props) == 2:
			out = append(out, resp.ArrayValue(props))
		default:
			out = append(out, props...)
		}
	}
	if resp3 && (withScores || withAttribs) {
		return ctx.WriteValue(resp.PairsValue(out))
	}
	return ctx.WriteArray(out)
}

func cmdVDIM(ctx *Context) error {
	if ctx.ArgCount() != 1 {
		return ctx.WriteError(arityError("vdim"))
	}
	v, err := getVectorSet(ctx, ctx.ArgString(0))
	if err != nil {
		return ctx.WriteError(err)
	}
	if v == nil {
		return ctx.WriteError(errors.New("ERR key does not exist"))
	}
	return ctx.WriteInteger(int64(v.Index.Dim()))
}

func cmdVCARD(ctx *Context) error {
	if ctx.ArgCount() != 1 {
		return ctx.WriteError(arityError("vcard"))
	}
	v, err := getVectorSet(ctx, ctx.ArgString(0))
	if err != nil {
		return ctx.WriteError(err)
	}
	if v == nil {
		return ctx.WriteInteger(0)
	}
	return ctx.WriteInteger(int64(v.Len()))
}

func cmdVREM(ctx *Context) error {
	if ctx.ArgCount() != 2 {
		return ctx.WriteError(arityError("vrem"))
	}
	key := ctx.ArgString(0)
	v, err := getVectorSet(ctx, key)
	if err != nil {
		return ctx.WriteError(err)
	}
	if v == nil {
		ctx.PropagateAs()
		return vsetBool(ctx, false)
	}
	v.Lock()
	removed := v.Remove(ctx.ArgString(1))
	empty := len(v.Elems) == 0
	v.Unlock()
	if !removed {
		ctx.PropagateAs()
		return vsetBool(ctx, false)
	}
	if empty {
		ctx.Store.Delete(key)
	}
	return vsetBool(ctx, true)
}

func cmdVEMB(ctx *Context) error {
	if ctx.ArgCount() < 2 {
		return ctx.WriteError(arityError("vemb"))
	}
	raw := false
	for j := 2; j < ctx.ArgCount(); j++ {
		if !strings.EqualFold(ctx.ArgString(j), "raw") {
			return ctx.WriteError(errors.New("ERR invalid option"))
		}
		raw = true
	}
	v, err := getVectorSet(ctx, ctx.ArgString(0))
	if err != nil {
		return ctx.WriteError(err)
	}
	if v == nil {
		return ctx.WriteNull()
	}
	v.RLock()
	defer v.RUnlock()
	n, ok := v.Elems[ctx.ArgString(1)]
	if !ok {
		return ctx.WriteNull()
	}
	if raw {
		out := []*resp.Value{resp.SimpleString(vsetQuantName(v.Index.Quant())), resp.BulkBytes(n.Raw()), vsetDouble(float64(n.L2()))}
		if v.Index.Quant() == vectorset.QuantQ8 {
			out = append(out, vsetDouble(float64(n.QRange())))
		}
		return ctx.WriteArray(out)
	}
	vec := v.Index.NodeVector(n)
	out := make([]*resp.Value, len(vec))
	for i, f := range vec {
		out[i] = vsetDouble(float64(f))
	}
	return ctx.WriteArray(out)
}

func vsetQuantName(q vectorset.Quant) string {
	switch q {
	case vectorset.QuantNone:
		return "f32"
	case vectorset.QuantQ8:
		return "int8"
	case vectorset.QuantBin:
		return "bin"
	}
	return "unknown"
}

func cmdVSETATTR(ctx *Context) error {
	if ctx.ArgCount() != 3 {
		return ctx.WriteError(arityError("vsetattr"))
	}
	v, err := getVectorSet(ctx, ctx.ArgString(0))
	if err != nil {
		return ctx.WriteError(err)
	}
	if v == nil {
		ctx.PropagateAs()
		return vsetBool(ctx, false)
	}
	v.Lock()
	ok := v.SetAttr(ctx.ArgString(1), ctx.Arg(2))
	v.Unlock()
	if !ok {
		ctx.PropagateAs()
	}
	return vsetBool(ctx, ok)
}

func cmdVGETATTR(ctx *Context) error {
	if ctx.ArgCount() != 2 {
		return ctx.WriteError(arityError("vgetattr"))
	}
	v, err := getVectorSet(ctx, ctx.ArgString(0))
	if err != nil {
		return ctx.WriteError(err)
	}
	if v == nil {
		return ctx.WriteNull()
	}
	v.RLock()
	defer v.RUnlock()
	n, ok := v.Elems[ctx.ArgString(1)]
	if !ok || n.Value.(*store.VectorElem).Attr == nil {
		return ctx.WriteNull()
	}
	return ctx.WriteBulkBytes(n.Value.(*store.VectorElem).Attr)
}

func cmdVLINKS(ctx *Context) error {
	if ctx.ArgCount() < 2 || ctx.ArgCount() > 3 {
		return ctx.WriteError(arityError("vlinks"))
	}
	withScores := ctx.ArgCount() == 3
	if withScores && !strings.EqualFold(ctx.ArgString(2), "WITHSCORES") {
		return ctx.WriteError(arityError("vlinks"))
	}
	v, err := getVectorSet(ctx, ctx.ArgString(0))
	if err != nil {
		return ctx.WriteError(err)
	}
	if v == nil {
		return ctx.WriteNull()
	}
	v.RLock()
	defer v.RUnlock()
	n, ok := v.Elems[ctx.ArgString(1)]
	if !ok {
		return ctx.WriteNull()
	}
	levels := make([]*resp.Value, 0, n.Level()+1)
	for l := int(n.Level()); l >= 0; l-- {
		links := n.Links(uint32(l))
		items := make([]*resp.Value, 0, len(links)*2)
		for _, link := range links {
			items = append(items, resp.BulkString(link.Value.(*store.VectorElem).Name))
			if withScores {
				d := v.Index.Distance(n, link)
				items = append(items, vsetDouble(float64(float32(1.0-float64(d)/2.0))))
			}
		}
		if withScores {
			levels = append(levels, resp.PairsValue(items))
		} else {
			levels = append(levels, resp.ArrayValue(items))
		}
	}
	return ctx.WriteArray(levels)
}

func cmdVINFO(ctx *Context) error {
	if ctx.ArgCount() != 1 {
		return ctx.WriteError(arityError("vinfo"))
	}
	v, err := getVectorSet(ctx, ctx.ArgString(0))
	if err != nil {
		return ctx.WriteError(err)
	}
	if v == nil {
		return ctx.WriteNullArray()
	}
	v.RLock()
	defer v.RUnlock()
	ix := v.Index
	return ctx.WriteValue(resp.PairsValue([]*resp.Value{
		resp.SimpleString("quant-type"), resp.SimpleString(vsetQuantName(ix.Quant())),
		resp.SimpleString("hnsw-m"), resp.IntegerValue(int64(ix.M())),
		resp.SimpleString("vector-dim"), resp.IntegerValue(int64(ix.Dim())),
		resp.SimpleString("projection-input-dim"), resp.IntegerValue(int64(v.ProjInput)),
		resp.SimpleString("size"), resp.IntegerValue(int64(ix.Len())),
		resp.SimpleString("max-level"), resp.IntegerValue(int64(ix.MaxLevel())),
		resp.SimpleString("attributes-count"), resp.IntegerValue(int64(v.NumAttribs)),
		resp.SimpleString("vset-uid"), resp.IntegerValue(int64(v.UID)),
		resp.SimpleString("hnsw-max-node-uid"), resp.IntegerValue(int64(ix.LastID())),
	}))
}

func cmdVRANDMEMBER(ctx *Context) error {
	if ctx.ArgCount() != 1 && ctx.ArgCount() != 2 {
		return ctx.WriteError(arityError("vrandmember"))
	}
	withCount := ctx.ArgCount() == 2
	count := int64(1)
	if withCount {
		n, ok := parseRedisInt(ctx.Arg(1))
		if !ok {
			return ctx.WriteError(errors.New("ERR COUNT value is not an integer"))
		}
		if n == 0 {
			return ctx.WriteArray([]*resp.Value{})
		}
		count = n
	}
	v, err := getVectorSet(ctx, ctx.ArgString(0))
	if err != nil {
		return ctx.WriteError(err)
	}
	if v == nil || v.Len() == 0 {
		if withCount {
			return ctx.WriteArray([]*resp.Value{})
		}
		return ctx.WriteNull()
	}
	v.RLock()
	defer v.RUnlock()
	ix := v.Index
	r := rand.New(rand.NewPCG(rand.Uint64(), rand.Uint64()))
	name := func(n *vectorset.Node) *resp.Value { return resp.BulkString(n.Value.(*store.VectorElem).Name) }
	if !withCount {
		return ctx.WriteValue(name(ix.RandomNode(r)))
	}
	size := int64(ix.Len())
	dups := count < 0
	abs := count
	if dups {
		abs = -count
		if abs < 0 {
			abs = math.MaxInt64
		}
	} else if abs > size {
		abs = size
	}
	if dups && abs > 1<<24 {
		return ctx.WriteError(errors.New("ERR value is out of range"))
	}
	out := make([]*resp.Value, 0, abs)
	switch {
	case dups:
		for i := int64(0); i < abs; i++ {
			out = append(out, name(ix.RandomNode(r)))
		}
	case float64(abs) <= float64(size)*0.2:
		seen := make(map[*vectorset.Node]bool, abs)
		for int64(len(out)) < abs {
			n := ix.RandomNode(r)
			if !seen[n] {
				seen[n] = true
				out = append(out, name(n))
			}
		}
	default:
		n := ix.RandomNode(r)
		for int64(len(out)) < abs {
			if n == nil {
				n = ix.Head()
			}
			out = append(out, name(n))
			n = n.Next()
		}
	}
	return ctx.WriteArray(out)
}

func cmdVISMEMBER(ctx *Context) error {
	if ctx.ArgCount() != 2 {
		return ctx.WriteError(arityError("vismember"))
	}
	v, err := getVectorSet(ctx, ctx.ArgString(0))
	if err != nil {
		return ctx.WriteError(err)
	}
	if v == nil {
		return vsetBool(ctx, false)
	}
	v.RLock()
	_, ok := v.Elems[ctx.ArgString(1)]
	v.RUnlock()
	return vsetBool(ctx, ok)
}
