package command

import (
	"errors"
	"fmt"
	"math"
	"math/rand"
	"strconv"
	"strings"

	"github.com/cachestorm/cachestorm/internal/resp"
	"github.com/cachestorm/cachestorm/internal/store"
)

func RegisterMLCommands(router *Router) {

	router.Register(&CommandDef{Name: "FEATURE.SET", Handler: cmdFEATURESET})
	router.Register(&CommandDef{Name: "FEATURE.GET", Handler: cmdFEATUREGET})
	router.Register(&CommandDef{Name: "FEATURE.DEL", Handler: cmdFEATUREDEL})
	router.Register(&CommandDef{Name: "FEATURE.INCR", Handler: cmdFEATUREINCR})
	router.Register(&CommandDef{Name: "FEATURE.VECTOR", Handler: cmdFEATUREVECTOR})

	router.Register(&CommandDef{Name: "EMBEDDING.CREATE", Handler: cmdEMBEDDINGCREATE})
	router.Register(&CommandDef{Name: "EMBEDDING.GET", Handler: cmdEMBEDDINGGET})
	router.Register(&CommandDef{Name: "EMBEDDING.DELETE", Handler: cmdEMBEDDINGDELETE})

	router.Register(&CommandDef{Name: "TENSOR.CREATE", Handler: cmdTENSORCREATE})
	router.Register(&CommandDef{Name: "TENSOR.GET", Handler: cmdTENSORGET})
	router.Register(&CommandDef{Name: "TENSOR.ADD", Handler: cmdTENSORADD})
	router.Register(&CommandDef{Name: "TENSOR.RESHAPE", Handler: cmdTENSORRESHAPE})
	router.Register(&CommandDef{Name: "TENSOR.DELETE", Handler: cmdTENSORDELETE})

	router.Register(&CommandDef{Name: "DATASET.CREATE", Handler: cmdDATASETCREATE})
	router.Register(&CommandDef{Name: "DATASET.ADD", Handler: cmdDATASETADD})
	router.Register(&CommandDef{Name: "DATASET.GET", Handler: cmdDATASETGET})
	router.Register(&CommandDef{Name: "DATASET.SPLIT", Handler: cmdDATASETSPLIT})
	router.Register(&CommandDef{Name: "DATASET.DELETE", Handler: cmdDATASETDELETE})

	router.Register(&CommandDef{Name: "MLXPERIMENT.CREATE", Handler: cmdMLXPERIMENTCREATE})
	router.Register(&CommandDef{Name: "MLXPERIMENT.LOG", Handler: cmdMLXPERIMENTLOG})
	router.Register(&CommandDef{Name: "MLXPERIMENT.METRICS", Handler: cmdMLXPERIMENTMETRICS})
	router.Register(&CommandDef{Name: "MLXPERIMENT.DELETE", Handler: cmdMLXPERIMENTDELETE})

	router.Register(&CommandDef{Name: "HYPERPARAM.SET", Handler: cmdHYPERPARAMSET})
	router.Register(&CommandDef{Name: "HYPERPARAM.GET", Handler: cmdHYPERPARAMGET})
	router.Register(&CommandDef{Name: "HYPERPARAM.DELETE", Handler: cmdHYPERPARAMDELETE})

}

// FEATURE, EMBEDDING, TENSOR, DATASET, MLXPERIMENT and HYPERPARAM keep
// each object in a hash key marked by a field naming its kind. Being keys
// they are per database, persisted, replicated and removed by DEL and
// FLUSHALL; changes are recorded as HSET, HDEL and DEL. Numbers are stored
// in full precision and replied with six decimals.

const (
	featureMarker      = "__feature"
	embeddingMarker    = "__embedding"
	tensorMarker       = "__tensor"
	datasetMarker      = "__dataset"
	mlxperimentMarker  = "__mlxperiment"
	hyperparamMarker   = "__hyperparam"
	maxTensorDimension = 32
)

func fmt6(v float64) *resp.Value { return resp.BulkString(fmt.Sprintf("%.6f", v)) }

func joinFloats(vs []float64) string {
	parts := make([]string, len(vs))
	for i, v := range vs {
		parts[i] = f64(v)
	}
	return strings.Join(parts, " ")
}

func splitFloats(s string) ([]float64, error) {
	if s == "" {
		return nil, nil
	}
	parts := strings.Split(s, " ")
	out := make([]float64, len(parts))
	for i, p := range parts {
		v, err := strconv.ParseFloat(p, 64)
		if err != nil {
			return nil, store.ErrWrongType
		}
		out[i] = v
	}
	return out, nil
}

// markedFloat reads the float field of the marked hash at key: ok false
// when the key or the field is missing.
func markedFloat(ctx *Context, key, marker, field string) (float64, bool, error) {
	h, err := readMarkedHash(ctx, key, marker)
	if err != nil || h == nil {
		return 0, false, err
	}
	h.RLock()
	raw, ok := h.Get(field)
	h.RUnlock()
	if !ok || field == marker {
		return 0, false, nil
	}
	v, err := strconv.ParseFloat(string(raw), 64)
	if err != nil {
		return 0, false, store.ErrWrongType
	}
	return v, true, nil
}

// --- FEATURE ---

// FEATURE.SET entity name value sets one feature of the entity.
func cmdFEATURESET(ctx *Context) error {
	if ctx.ArgCount() != 3 {
		return ctx.WriteError(ErrWrongArgCount)
	}
	entity, name := ctx.ArgString(0), ctx.ArgString(1)
	if err := reservedField(featureMarker, name); err != nil {
		return ctx.WriteError(err)
	}
	v, err := parseFiniteFloat(ctx.ArgString(2))
	if err != nil {
		return ctx.WriteError(err)
	}
	if _, err := readMarkedHash(ctx, entity, featureMarker); err != nil {
		return ctx.WriteError(err)
	}
	if err := setMarkedFields(ctx, entity, featureMarker, name, f64(v)); err != nil {
		return ctx.WriteError(err)
	}
	return ctx.WriteOK()
}

func cmdFEATUREGET(ctx *Context) error {
	if ctx.ArgCount() != 2 {
		return ctx.WriteError(ErrWrongArgCount)
	}
	v, ok, err := markedFloat(ctx, ctx.ArgString(0), featureMarker, ctx.ArgString(1))
	if err != nil {
		return ctx.WriteError(err)
	}
	if !ok {
		return ctx.WriteNull()
	}
	return ctx.WriteValue(fmt6(v))
}

// FEATURE.DEL entity name: 1 when the feature was there.
func cmdFEATUREDEL(ctx *Context) error {
	if ctx.ArgCount() != 2 {
		return ctx.WriteError(ErrWrongArgCount)
	}
	if _, err := readMarkedHash(ctx, ctx.ArgString(0), featureMarker); err != nil {
		return ctx.WriteError(err)
	}
	return ctx.WriteInteger(int64(deleteMarkedFields(ctx, ctx.ArgString(0), featureMarker, ctx.ArgString(1))))
}

// FEATURE.INCR entity name delta adds delta to the feature: its new value.
func cmdFEATUREINCR(ctx *Context) error {
	if ctx.ArgCount() != 3 {
		return ctx.WriteError(ErrWrongArgCount)
	}
	entity, name := ctx.ArgString(0), ctx.ArgString(1)
	if err := reservedField(featureMarker, name); err != nil {
		return ctx.WriteError(err)
	}
	delta, err := parseFiniteFloat(ctx.ArgString(2))
	if err != nil {
		return ctx.WriteError(err)
	}
	v, _, err := markedFloat(ctx, entity, featureMarker, name)
	if err != nil {
		return ctx.WriteError(err)
	}
	v += delta
	if math.IsInf(v, 0) {
		return ctx.WriteError(errors.New("ERR increment would produce NaN or Infinity"))
	}
	if err := setMarkedFields(ctx, entity, featureMarker, name, f64(v)); err != nil {
		return ctx.WriteError(err)
	}
	return ctx.WriteValue(fmt6(v))
}

// FEATURE.VECTOR entity: [name, value] pairs, sorted by name.
func cmdFEATUREVECTOR(ctx *Context) error {
	if ctx.ArgCount() != 1 {
		return ctx.WriteError(ErrWrongArgCount)
	}
	r, err := readMarked(ctx, ctx.ArgString(0), featureMarker)
	if err != nil {
		return ctx.WriteError(err)
	}
	out := make([]*resp.Value, 0, len(r))
	for _, name := range sortedKeys(r) {
		if name == featureMarker {
			continue
		}
		v, err := strconv.ParseFloat(r[name], 64)
		if err != nil {
			return ctx.WriteError(store.ErrWrongType)
		}
		out = append(out, resp.ArrayValue([]*resp.Value{resp.BulkString(name), fmt6(v)}))
	}
	return ctx.WriteArray(out)
}

// --- EMBEDDING ---

// An embedding's marker holds its dimension and its vector field the
// values, space separated.

func readEmbedding(ctx *Context, id string) ([]float64, error) {
	r, err := readMarked(ctx, id, embeddingMarker)
	if err != nil || r == nil {
		return nil, err
	}
	v, err := splitFloats(r["vector"])
	if err != nil || strconv.Itoa(len(v)) != r[embeddingMarker] {
		return nil, store.ErrWrongType
	}
	return v, nil
}

// EMBEDDING.CREATE id value [value ...] stores the vector, replacing one at
// id.
func cmdEMBEDDINGCREATE(ctx *Context) error {
	if ctx.ArgCount() < 2 {
		return ctx.WriteError(ErrWrongArgCount)
	}
	args := make([]string, ctx.ArgCount()-1)
	for i := range args {
		args[i] = ctx.ArgString(i + 1)
	}
	v, err := parseFloatArgs(args...)
	if err != nil {
		return ctx.WriteError(err)
	}
	if err := createMarked(ctx, ctx.ArgString(0), embeddingMarker, embeddingMarker, strconv.Itoa(len(v)), "vector", joinFloats(v)); err != nil {
		return ctx.WriteError(err)
	}
	return ctx.WriteOK()
}

func cmdEMBEDDINGGET(ctx *Context) error {
	if ctx.ArgCount() != 1 {
		return ctx.WriteError(ErrWrongArgCount)
	}
	v, err := readEmbedding(ctx, ctx.ArgString(0))
	if err != nil {
		return ctx.WriteError(err)
	}
	if v == nil {
		return ctx.WriteNull()
	}
	out := make([]*resp.Value, len(v))
	for i, x := range v {
		out[i] = fmt6(x)
	}
	return ctx.WriteArray(out)
}

func cmdEMBEDDINGDELETE(ctx *Context) error { return deleteMarked(ctx, embeddingMarker) }

// --- TENSOR ---

// A tensor holds its shape and data, each space separated.

type Tensor struct {
	Shape []int
	Data  []float64
}

func tensorShapeMatches(shape []int, n int) bool {
	if len(shape) == 0 || len(shape) > maxTensorDimension {
		return false
	}
	product := 1
	for _, d := range shape {
		if d <= 0 || product > n/d {
			return false
		}
		product *= d
	}
	return product == n
}

func parseShape(args []string) ([]int, error) {
	shape := make([]int, len(args))
	for i, a := range args {
		n, err := strconv.Atoi(a)
		if err != nil {
			return nil, ErrNotInteger
		}
		shape[i] = n
	}
	return shape, nil
}

func joinInts(vs []int) string {
	parts := make([]string, len(vs))
	for i, v := range vs {
		parts[i] = strconv.Itoa(v)
	}
	return strings.Join(parts, " ")
}

func readTensor(ctx *Context, name string) (*Tensor, error) {
	r, err := readMarked(ctx, name, tensorMarker)
	if err != nil || r == nil {
		return nil, err
	}
	shape, err := parseShape(strings.Fields(r["shape"]))
	if err != nil {
		return nil, store.ErrWrongType
	}
	data, err := splitFloats(r["data"])
	if err != nil || !tensorShapeMatches(shape, len(data)) {
		return nil, store.ErrWrongType
	}
	return &Tensor{shape, data}, nil
}

func loadTensor(ctx *Context, name, what string) (*Tensor, error) {
	t, err := readTensor(ctx, name)
	if err == nil && t == nil {
		err = fmt.Errorf("ERR %s not found", what)
	}
	return t, err
}

func saveTensor(ctx *Context, name string, t *Tensor) error {
	return updateRecord(ctx, name, "shape", joinInts(t.Shape), "data", joinFloats(t.Data))
}

// TENSOR.CREATE name dim [dim ...] | value [value ...] stores the tensor,
// replacing one at name.
func cmdTENSORCREATE(ctx *Context) error {
	if ctx.ArgCount() < 3 {
		return ctx.WriteError(ErrWrongArgCount)
	}
	args := make([]string, ctx.ArgCount()-1)
	for i := range args {
		args[i] = ctx.ArgString(i + 1)
	}
	sep := len(args)
	for i, a := range args {
		if a == "|" {
			sep = i
			break
		}
	}
	shape, err := parseShape(args[:sep])
	if err != nil {
		return ctx.WriteError(err)
	}
	var data []float64
	if sep < len(args) {
		if data, err = parseFloatArgs(args[sep+1:]...); err != nil {
			return ctx.WriteError(err)
		}
	}
	if !tensorShapeMatches(shape, len(data)) {
		return ctx.WriteError(fmt.Errorf("ERR tensor shape does not match data length %d", len(data)))
	}
	if err := createMarked(ctx, ctx.ArgString(0), tensorMarker, tensorMarker, "1", "shape", joinInts(shape), "data", joinFloats(data)); err != nil {
		return ctx.WriteError(err)
	}
	return ctx.WriteOK()
}

func cmdTENSORGET(ctx *Context) error {
	if ctx.ArgCount() != 1 {
		return ctx.WriteError(ErrWrongArgCount)
	}
	t, err := readTensor(ctx, ctx.ArgString(0))
	if err != nil {
		return ctx.WriteError(err)
	}
	if t == nil {
		return ctx.WriteNull()
	}
	shape := make([]*resp.Value, len(t.Shape))
	for i, d := range t.Shape {
		shape[i] = resp.IntegerValue(int64(d))
	}
	data := make([]*resp.Value, len(t.Data))
	for i, d := range t.Data {
		data[i] = fmt6(d)
	}
	return ctx.WriteArray([]*resp.Value{
		resp.BulkString("shape"), resp.ArrayValue(shape),
		resp.BulkString("data"), resp.ArrayValue(data),
	})
}

// TENSOR.ADD name other adds other to name element-wise.
func cmdTENSORADD(ctx *Context) error {
	if ctx.ArgCount() != 2 {
		return ctx.WriteError(ErrWrongArgCount)
	}
	t1, err := loadTensor(ctx, ctx.ArgString(0), "tensor")
	if err != nil {
		return ctx.WriteError(err)
	}
	t2, err := loadTensor(ctx, ctx.ArgString(1), "other tensor")
	if err != nil {
		return ctx.WriteError(err)
	}
	if len(t1.Data) != len(t2.Data) {
		return ctx.WriteError(fmt.Errorf("ERR tensor sizes differ: %d and %d", len(t1.Data), len(t2.Data)))
	}
	for i := range t1.Data {
		t1.Data[i] += t2.Data[i]
	}
	if err := saveTensor(ctx, ctx.ArgString(0), t1); err != nil {
		return ctx.WriteError(err)
	}
	return ctx.WriteOK()
}

// TENSOR.RESHAPE name dim [dim ...] changes the shape, keeping the data.
func cmdTENSORRESHAPE(ctx *Context) error {
	if ctx.ArgCount() < 2 {
		return ctx.WriteError(ErrWrongArgCount)
	}
	args := make([]string, ctx.ArgCount()-1)
	for i := range args {
		args[i] = ctx.ArgString(i + 1)
	}
	shape, err := parseShape(args)
	if err != nil {
		return ctx.WriteError(err)
	}
	t, err := loadTensor(ctx, ctx.ArgString(0), "tensor")
	if err != nil {
		return ctx.WriteError(err)
	}
	if !tensorShapeMatches(shape, len(t.Data)) {
		return ctx.WriteError(fmt.Errorf("ERR tensor shape does not match data length %d", len(t.Data)))
	}
	t.Shape = shape
	if err := updateRecord(ctx, ctx.ArgString(0), "shape", joinInts(shape)); err != nil {
		return ctx.WriteError(err)
	}
	return ctx.WriteOK()
}

func cmdTENSORDELETE(ctx *Context) error { return deleteMarked(ctx, tensorMarker) }

// --- DATASET ---

// A dataset is a log: the marker holds the item count and item i is the
// field i.

func cmdDATASETCREATE(ctx *Context) error {
	if ctx.ArgCount() != 1 {
		return ctx.WriteError(ErrWrongArgCount)
	}
	if err := createMarked(ctx, ctx.ArgString(0), datasetMarker, datasetMarker, "0"); err != nil {
		return ctx.WriteError(err)
	}
	return ctx.WriteOK()
}

// DATASET.ADD name item appends the item: the item count.
func cmdDATASETADD(ctx *Context) error {
	if ctx.ArgCount() != 2 {
		return ctx.WriteError(ErrWrongArgCount)
	}
	name := ctx.ArgString(0)
	n, _, ok, err := logHead(ctx, name, datasetMarker)
	if err != nil || !ok {
		return ctx.WriteError(orNotFound(err, "dataset"))
	}
	if err := appendLog(ctx, name, datasetMarker, n, ctx.ArgString(1)); err != nil {
		return ctx.WriteError(err)
	}
	return ctx.WriteInteger(int64(n + 1))
}

func cmdDATASETGET(ctx *Context) error {
	if ctx.ArgCount() != 1 {
		return ctx.WriteError(ErrWrongArgCount)
	}
	items, _, err := readLog(ctx, ctx.ArgString(0), datasetMarker)
	if err != nil {
		return ctx.WriteError(err)
	}
	if items == nil {
		return ctx.WriteNull()
	}
	return ctx.WriteArray(bulkStrings(items))
}

// DATASET.SPLIT name ratio [SEED seed]: the train and test parts, shuffled
// with seed when one is given.
func cmdDATASETSPLIT(ctx *Context) error {
	if ctx.ArgCount() != 2 && ctx.ArgCount() != 4 {
		return ctx.WriteError(ErrWrongArgCount)
	}
	ratio, err := strconv.ParseFloat(ctx.ArgString(1), 64)
	if err != nil || math.IsNaN(ratio) || ratio < 0 || ratio > 1 {
		return ctx.WriteError(fmt.Errorf("ERR ratio must be a number between 0 and 1"))
	}
	shuffle := false
	var seed int64
	if ctx.ArgCount() == 4 {
		if !strings.EqualFold(ctx.ArgString(2), "SEED") {
			return ctx.WriteError(ErrSyntaxError)
		}
		seed, err = strconv.ParseInt(ctx.ArgString(3), 10, 64)
		if err != nil {
			return ctx.WriteError(ErrNotInteger)
		}
		shuffle = true
	}
	items, _, err := readLog(ctx, ctx.ArgString(0), datasetMarker)
	if err != nil || items == nil {
		return ctx.WriteError(orNotFound(err, "dataset"))
	}
	if shuffle {
		rng := rand.New(rand.NewSource(seed))
		rng.Shuffle(len(items), func(i, j int) { items[i], items[j] = items[j], items[i] })
	}
	split := int(math.Floor(float64(len(items)) * ratio))
	return ctx.WriteArray([]*resp.Value{
		resp.BulkString("train"), resp.ArrayValue(bulkStrings(items[:split])),
		resp.BulkString("test"), resp.ArrayValue(bulkStrings(items[split:])),
	})
}

func cmdDATASETDELETE(ctx *Context) error { return deleteMarked(ctx, datasetMarker) }

// --- MLXPERIMENT ---

func cmdMLXPERIMENTCREATE(ctx *Context) error {
	if ctx.ArgCount() != 1 {
		return ctx.WriteError(ErrWrongArgCount)
	}
	if err := createMarked(ctx, ctx.ArgString(0), mlxperimentMarker, mlxperimentMarker, "1"); err != nil {
		return ctx.WriteError(err)
	}
	return ctx.WriteOK()
}

// MLXPERIMENT.LOG name metric value sets the metric.
func cmdMLXPERIMENTLOG(ctx *Context) error {
	if ctx.ArgCount() != 3 {
		return ctx.WriteError(ErrWrongArgCount)
	}
	name, metric := ctx.ArgString(0), ctx.ArgString(1)
	if err := reservedField(mlxperimentMarker, metric); err != nil {
		return ctx.WriteError(err)
	}
	v, err := parseFiniteFloat(ctx.ArgString(2))
	if err != nil {
		return ctx.WriteError(err)
	}
	if _, err := loadMarked(ctx, name, mlxperimentMarker, "experiment"); err != nil {
		return ctx.WriteError(err)
	}
	if err := updateRecord(ctx, name, metric, f64(v)); err != nil {
		return ctx.WriteError(err)
	}
	return ctx.WriteOK()
}

// MLXPERIMENT.METRICS name: metric, value pairs sorted by metric.
func cmdMLXPERIMENTMETRICS(ctx *Context) error {
	if ctx.ArgCount() != 1 {
		return ctx.WriteError(ErrWrongArgCount)
	}
	r, err := loadMarked(ctx, ctx.ArgString(0), mlxperimentMarker, "experiment")
	if err != nil {
		return ctx.WriteError(err)
	}
	out := make([]*resp.Value, 0, 2*len(r))
	for _, k := range sortedKeys(r) {
		if k == mlxperimentMarker {
			continue
		}
		v, err := strconv.ParseFloat(r[k], 64)
		if err != nil {
			return ctx.WriteError(store.ErrWrongType)
		}
		out = append(out, resp.BulkString(k), fmt6(v))
	}
	return ctx.WriteArray(out)
}

func cmdMLXPERIMENTDELETE(ctx *Context) error { return deleteMarked(ctx, mlxperimentMarker) }

// --- HYPERPARAM ---

// HYPERPARAM.SET name param value sets the parameter.
func cmdHYPERPARAMSET(ctx *Context) error {
	if ctx.ArgCount() != 3 {
		return ctx.WriteError(ErrWrongArgCount)
	}
	name, param := ctx.ArgString(0), ctx.ArgString(1)
	if err := reservedField(hyperparamMarker, param); err != nil {
		return ctx.WriteError(err)
	}
	if _, err := readMarkedHash(ctx, name, hyperparamMarker); err != nil {
		return ctx.WriteError(err)
	}
	if err := setMarkedFields(ctx, name, hyperparamMarker, param, ctx.ArgString(2)); err != nil {
		return ctx.WriteError(err)
	}
	return ctx.WriteOK()
}

func cmdHYPERPARAMGET(ctx *Context) error {
	if ctx.ArgCount() != 2 {
		return ctx.WriteError(ErrWrongArgCount)
	}
	param := ctx.ArgString(1)
	r, err := readMarked(ctx, ctx.ArgString(0), hyperparamMarker)
	if err != nil {
		return ctx.WriteError(err)
	}
	v, ok := r[param]
	if !ok || param == hyperparamMarker {
		return ctx.WriteNull()
	}
	return ctx.WriteBulkString(v)
}

func cmdHYPERPARAMDELETE(ctx *Context) error { return deleteMarked(ctx, hyperparamMarker) }
