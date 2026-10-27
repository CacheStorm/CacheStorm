package command

import (
	"fmt"
	"sync"

	"github.com/cachestorm/cachestorm/internal/resp"
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

var (
	features   = make(map[string]map[string]float64)
	featuresMx sync.RWMutex
)

func cmdFEATURESET(ctx *Context) error {
	if ctx.ArgCount() < 3 {
		return ctx.WriteError(ErrWrongArgCount)
	}
	entity := ctx.ArgString(0)
	key := ctx.ArgString(1)
	value := parseFloatExt([]byte(ctx.ArgString(2)))
	featuresMx.Lock()
	defer featuresMx.Unlock()
	if _, exists := features[entity]; !exists {
		features[entity] = make(map[string]float64)
	}
	features[entity][key] = value
	return ctx.WriteOK()
}

func cmdFEATUREGET(ctx *Context) error {
	if ctx.ArgCount() < 2 {
		return ctx.WriteError(ErrWrongArgCount)
	}
	entity := ctx.ArgString(0)
	key := ctx.ArgString(1)
	featuresMx.RLock()
	e, exists := features[entity]
	if !exists {
		featuresMx.RUnlock()
		return ctx.WriteNull()
	}
	val, exists := e[key]
	if !exists {
		featuresMx.RUnlock()
		return ctx.WriteNull()
	}
	reply := fmt.Sprintf("%.6f", val)
	featuresMx.RUnlock()
	return ctx.WriteBulkString(reply)
}

func cmdFEATUREDEL(ctx *Context) error {
	if ctx.ArgCount() < 2 {
		return ctx.WriteError(ErrWrongArgCount)
	}
	entity := ctx.ArgString(0)
	key := ctx.ArgString(1)
	featuresMx.Lock()
	defer featuresMx.Unlock()
	e, exists := features[entity]
	if !exists {
		return ctx.WriteInteger(0)
	}
	if _, exists := e[key]; !exists {
		return ctx.WriteInteger(0)
	}
	delete(e, key)
	return ctx.WriteInteger(1)
}

func cmdFEATUREINCR(ctx *Context) error {
	if ctx.ArgCount() < 3 {
		return ctx.WriteError(ErrWrongArgCount)
	}
	entity := ctx.ArgString(0)
	key := ctx.ArgString(1)
	delta := parseFloatExt([]byte(ctx.ArgString(2)))
	featuresMx.Lock()
	defer featuresMx.Unlock()
	if _, exists := features[entity]; !exists {
		features[entity] = make(map[string]float64)
	}
	features[entity][key] += delta
	return ctx.WriteBulkString(fmt.Sprintf("%.6f", features[entity][key]))
}

func cmdFEATUREVECTOR(ctx *Context) error {
	if ctx.ArgCount() < 1 {
		return ctx.WriteError(ErrWrongArgCount)
	}
	entity := ctx.ArgString(0)
	featuresMx.RLock()
	e, exists := features[entity]
	results := make([]*resp.Value, 0, len(e))
	for k, v := range e {
		results = append(results, resp.ArrayValue([]*resp.Value{
			resp.BulkString(k), resp.BulkString(fmt.Sprintf("%.6f", v)),
		}))
	}
	featuresMx.RUnlock()
	if !exists {
		return ctx.WriteArray([]*resp.Value{})
	}
	return ctx.WriteArray(results)
}

var (
	embeddings   = make(map[string]*Embedding)
	embeddingsMx sync.RWMutex
)

type Embedding struct {
	ID     string
	Vector []float64
}

func cmdEMBEDDINGCREATE(ctx *Context) error {
	if ctx.ArgCount() < 2 {
		return ctx.WriteError(ErrWrongArgCount)
	}
	id := ctx.ArgString(0)
	vector := make([]float64, 0)
	for i := 1; i < ctx.ArgCount(); i++ {
		vector = append(vector, parseFloatExt([]byte(ctx.ArgString(i))))
	}
	embeddingsMx.Lock()
	defer embeddingsMx.Unlock()
	embeddings[id] = &Embedding{ID: id, Vector: vector}
	return ctx.WriteOK()
}

func cmdEMBEDDINGGET(ctx *Context) error {
	if ctx.ArgCount() < 1 {
		return ctx.WriteError(ErrWrongArgCount)
	}
	id := ctx.ArgString(0)
	embeddingsMx.RLock()
	e, exists := embeddings[id]
	embeddingsMx.RUnlock()
	if !exists {
		return ctx.WriteNull()
	}
	results := make([]*resp.Value, len(e.Vector))
	for i, v := range e.Vector {
		results[i] = resp.BulkString(fmt.Sprintf("%.6f", v))
	}
	return ctx.WriteArray(results)
}

func cmdEMBEDDINGDELETE(ctx *Context) error {
	if ctx.ArgCount() < 1 {
		return ctx.WriteError(ErrWrongArgCount)
	}
	id := ctx.ArgString(0)
	embeddingsMx.Lock()
	defer embeddingsMx.Unlock()
	if _, exists := embeddings[id]; !exists {
		return ctx.WriteInteger(0)
	}
	delete(embeddings, id)
	return ctx.WriteInteger(1)
}

var (
	tensors   = make(map[string]*Tensor)
	tensorsMx sync.RWMutex
)

type Tensor struct {
	Name  string
	Shape []int
	Data  []float64
}

func tensorShapeMatches(shape []int, n int) bool {
	product := 1
	for _, d := range shape {
		if d <= 0 || product > n/d {
			return false
		}
		product *= d
	}
	return product == n
}

func cmdTENSORCREATE(ctx *Context) error {
	if ctx.ArgCount() < 3 {
		return ctx.WriteError(ErrWrongArgCount)
	}
	name := ctx.ArgString(0)
	shape := make([]int, 0)
	data := make([]float64, 0)
	i := 1
	for i < ctx.ArgCount() && ctx.ArgString(i) != "|" {
		shape = append(shape, int(parseInt64(ctx.ArgString(i))))
		i++
	}
	if i < ctx.ArgCount() && ctx.ArgString(i) == "|" {
		i++
	}
	for i < ctx.ArgCount() {
		data = append(data, parseFloatExt([]byte(ctx.ArgString(i))))
		i++
	}
	if !tensorShapeMatches(shape, len(data)) {
		return ctx.WriteError(fmt.Errorf("ERR tensor shape does not match data length %d", len(data)))
	}
	tensorsMx.Lock()
	defer tensorsMx.Unlock()
	tensors[name] = &Tensor{Name: name, Shape: shape, Data: data}
	return ctx.WriteOK()
}

func cmdTENSORGET(ctx *Context) error {
	if ctx.ArgCount() < 1 {
		return ctx.WriteError(ErrWrongArgCount)
	}
	name := ctx.ArgString(0)
	tensorsMx.RLock()
	t, exists := tensors[name]
	if !exists {
		tensorsMx.RUnlock()
		return ctx.WriteNull()
	}
	shape := make([]*resp.Value, len(t.Shape))
	for i, s := range t.Shape {
		shape[i] = resp.IntegerValue(int64(s))
	}
	data := make([]*resp.Value, len(t.Data))
	for i, d := range t.Data {
		data[i] = resp.BulkString(fmt.Sprintf("%.6f", d))
	}
	reply := []*resp.Value{
		resp.BulkString("shape"), resp.ArrayValue(shape),
		resp.BulkString("data"), resp.ArrayValue(data),
	}
	tensorsMx.RUnlock()
	return ctx.WriteArray(reply)
}

func cmdTENSORADD(ctx *Context) error {
	if ctx.ArgCount() < 2 {
		return ctx.WriteError(ErrWrongArgCount)
	}
	name := ctx.ArgString(0)
	other := ctx.ArgString(1)
	tensorsMx.Lock()
	defer tensorsMx.Unlock()
	t1, exists := tensors[name]
	if !exists {
		return ctx.WriteError(fmt.Errorf("ERR tensor not found"))
	}
	t2, exists := tensors[other]
	if !exists {
		return ctx.WriteError(fmt.Errorf("ERR other tensor not found"))
	}
	if len(t1.Data) != len(t2.Data) {
		return ctx.WriteError(fmt.Errorf("ERR tensor sizes differ: %d and %d", len(t1.Data), len(t2.Data)))
	}
	result := make([]float64, len(t1.Data))
	for i := range t1.Data {
		result[i] = t1.Data[i] + t2.Data[i]
	}
	t1.Data = result
	return ctx.WriteOK()
}

func cmdTENSORRESHAPE(ctx *Context) error {
	if ctx.ArgCount() < 2 {
		return ctx.WriteError(ErrWrongArgCount)
	}
	name := ctx.ArgString(0)
	newShape := make([]int, 0)
	for i := 1; i < ctx.ArgCount(); i++ {
		newShape = append(newShape, int(parseInt64(ctx.ArgString(i))))
	}
	tensorsMx.Lock()
	defer tensorsMx.Unlock()
	t, exists := tensors[name]
	if !exists {
		return ctx.WriteError(fmt.Errorf("ERR tensor not found"))
	}
	if !tensorShapeMatches(newShape, len(t.Data)) {
		return ctx.WriteError(fmt.Errorf("ERR tensor shape does not match data length %d", len(t.Data)))
	}
	t.Shape = newShape
	return ctx.WriteOK()
}

func cmdTENSORDELETE(ctx *Context) error {
	if ctx.ArgCount() < 1 {
		return ctx.WriteError(ErrWrongArgCount)
	}
	name := ctx.ArgString(0)
	tensorsMx.Lock()
	defer tensorsMx.Unlock()
	if _, exists := tensors[name]; !exists {
		return ctx.WriteInteger(0)
	}
	delete(tensors, name)
	return ctx.WriteInteger(1)
}

var (
	datasets   = make(map[string]*Dataset)
	datasetsMx sync.RWMutex
)

type Dataset struct {
	Name string
	Data []string
}

func cmdDATASETCREATE(ctx *Context) error {
	if ctx.ArgCount() < 1 {
		return ctx.WriteError(ErrWrongArgCount)
	}
	name := ctx.ArgString(0)
	datasetsMx.Lock()
	defer datasetsMx.Unlock()
	datasets[name] = &Dataset{Name: name, Data: make([]string, 0)}
	return ctx.WriteOK()
}

func cmdDATASETADD(ctx *Context) error {
	if ctx.ArgCount() < 2 {
		return ctx.WriteError(ErrWrongArgCount)
	}
	name := ctx.ArgString(0)
	item := ctx.ArgString(1)
	datasetsMx.Lock()
	defer datasetsMx.Unlock()
	d, exists := datasets[name]
	if !exists {
		return ctx.WriteError(fmt.Errorf("ERR dataset not found"))
	}
	d.Data = append(d.Data, item)
	return ctx.WriteInteger(int64(len(d.Data)))
}

func cmdDATASETGET(ctx *Context) error {
	if ctx.ArgCount() < 1 {
		return ctx.WriteError(ErrWrongArgCount)
	}
	name := ctx.ArgString(0)
	datasetsMx.RLock()
	d, exists := datasets[name]
	if !exists {
		datasetsMx.RUnlock()
		return ctx.WriteNull()
	}
	results := make([]*resp.Value, len(d.Data))
	for i, item := range d.Data {
		results[i] = resp.BulkString(item)
	}
	reply := results
	datasetsMx.RUnlock()
	return ctx.WriteArray(reply)
}

func cmdDATASETSPLIT(ctx *Context) error {
	if ctx.ArgCount() < 2 {
		return ctx.WriteError(ErrWrongArgCount)
	}
	name := ctx.ArgString(0)
	ratio := parseFloatExt([]byte(ctx.ArgString(1)))
	datasetsMx.RLock()
	d, exists := datasets[name]
	datasetsMx.RUnlock()
	if !exists {
		return ctx.WriteError(fmt.Errorf("ERR dataset not found"))
	}
	split := int(float64(len(d.Data)) * ratio)
	return ctx.WriteArray([]*resp.Value{
		resp.BulkString("train"), resp.IntegerValue(int64(split)),
		resp.BulkString("test"), resp.IntegerValue(int64(len(d.Data) - split)),
	})
}

func cmdDATASETDELETE(ctx *Context) error {
	if ctx.ArgCount() < 1 {
		return ctx.WriteError(ErrWrongArgCount)
	}
	name := ctx.ArgString(0)
	datasetsMx.Lock()
	defer datasetsMx.Unlock()
	if _, exists := datasets[name]; !exists {
		return ctx.WriteInteger(0)
	}
	delete(datasets, name)
	return ctx.WriteInteger(1)
}

var (
	mlExperiments   = make(map[string]*MLExperiment)
	mlExperimentsMx sync.RWMutex
)

type MLExperiment struct {
	Name    string
	Metrics map[string]float64
}

func cmdMLXPERIMENTCREATE(ctx *Context) error {
	if ctx.ArgCount() < 1 {
		return ctx.WriteError(ErrWrongArgCount)
	}
	name := ctx.ArgString(0)
	mlExperimentsMx.Lock()
	defer mlExperimentsMx.Unlock()
	mlExperiments[name] = &MLExperiment{Name: name, Metrics: make(map[string]float64)}
	return ctx.WriteOK()
}

func cmdMLXPERIMENTLOG(ctx *Context) error {
	if ctx.ArgCount() < 3 {
		return ctx.WriteError(ErrWrongArgCount)
	}
	name := ctx.ArgString(0)
	key := ctx.ArgString(1)
	value := parseFloatExt([]byte(ctx.ArgString(2)))
	mlExperimentsMx.Lock()
	defer mlExperimentsMx.Unlock()
	e, exists := mlExperiments[name]
	if !exists {
		return ctx.WriteError(fmt.Errorf("ERR experiment not found"))
	}
	e.Metrics[key] = value
	return ctx.WriteOK()
}

func cmdMLXPERIMENTMETRICS(ctx *Context) error {
	if ctx.ArgCount() < 1 {
		return ctx.WriteError(ErrWrongArgCount)
	}
	name := ctx.ArgString(0)
	mlExperimentsMx.RLock()
	e, exists := mlExperiments[name]
	mlExperimentsMx.RUnlock()
	if !exists {
		return ctx.WriteError(fmt.Errorf("ERR experiment not found"))
	}
	results := make([]*resp.Value, 0)
	for k, v := range e.Metrics {
		results = append(results, resp.BulkString(k), resp.BulkString(fmt.Sprintf("%.6f", v)))
	}
	return ctx.WriteArray(results)
}

func cmdMLXPERIMENTDELETE(ctx *Context) error {
	if ctx.ArgCount() < 1 {
		return ctx.WriteError(ErrWrongArgCount)
	}
	name := ctx.ArgString(0)
	mlExperimentsMx.Lock()
	defer mlExperimentsMx.Unlock()
	if _, exists := mlExperiments[name]; !exists {
		return ctx.WriteInteger(0)
	}
	delete(mlExperiments, name)
	return ctx.WriteInteger(1)
}

var (
	hyperparams   = make(map[string]*Hyperparams)
	hyperparamsMx sync.RWMutex
)

type Hyperparams struct {
	Name   string
	Params map[string]string
}

func cmdHYPERPARAMSET(ctx *Context) error {
	if ctx.ArgCount() < 3 {
		return ctx.WriteError(ErrWrongArgCount)
	}
	name := ctx.ArgString(0)
	key := ctx.ArgString(1)
	value := ctx.ArgString(2)
	hyperparamsMx.Lock()
	defer hyperparamsMx.Unlock()
	if _, exists := hyperparams[name]; !exists {
		hyperparams[name] = &Hyperparams{Name: name, Params: make(map[string]string)}
	}
	hyperparams[name].Params[key] = value
	return ctx.WriteOK()
}

func cmdHYPERPARAMGET(ctx *Context) error {
	if ctx.ArgCount() < 2 {
		return ctx.WriteError(ErrWrongArgCount)
	}
	name := ctx.ArgString(0)
	key := ctx.ArgString(1)
	hyperparamsMx.RLock()
	h, exists := hyperparams[name]
	hyperparamsMx.RUnlock()
	if !exists {
		return ctx.WriteNull()
	}
	val, exists := h.Params[key]
	if !exists {
		return ctx.WriteNull()
	}
	return ctx.WriteBulkString(val)
}

func cmdHYPERPARAMDELETE(ctx *Context) error {
	if ctx.ArgCount() < 1 {
		return ctx.WriteError(ErrWrongArgCount)
	}
	name := ctx.ArgString(0)
	hyperparamsMx.Lock()
	defer hyperparamsMx.Unlock()
	if _, exists := hyperparams[name]; !exists {
		return ctx.WriteInteger(0)
	}
	delete(hyperparams, name)
	return ctx.WriteInteger(1)
}
