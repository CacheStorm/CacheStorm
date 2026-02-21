package command

import (
	"fmt"
	"sync"
	"time"

	"github.com/cachestorm/cachestorm/internal/resp"
)

func RegisterMLCommands(router *Router) {
	router.Register(&CommandDef{Name: "MODEL.CREATE", Handler: cmdMODELCREATE})
	router.Register(&CommandDef{Name: "MODEL.TRAIN", Handler: cmdMODELTRAIN})
	router.Register(&CommandDef{Name: "MODEL.PREDICT", Handler: cmdMODELPREDICT})
	router.Register(&CommandDef{Name: "MODEL.DELETE", Handler: cmdMODELDELETE})
	router.Register(&CommandDef{Name: "MODEL.LIST", Handler: cmdMODELLIST})
	router.Register(&CommandDef{Name: "MODEL.STATUS", Handler: cmdMODELSTATUS})

	router.Register(&CommandDef{Name: "FEATURE.SET", Handler: cmdFEATURESET})
	router.Register(&CommandDef{Name: "FEATURE.GET", Handler: cmdFEATUREGET})
	router.Register(&CommandDef{Name: "FEATURE.DEL", Handler: cmdFEATUREDEL})
	router.Register(&CommandDef{Name: "FEATURE.INCR", Handler: cmdFEATUREINCR})
	router.Register(&CommandDef{Name: "FEATURE.NORMALIZE", Handler: cmdFEATURENORMALIZE})
	router.Register(&CommandDef{Name: "FEATURE.VECTOR", Handler: cmdFEATUREVECTOR})

	router.Register(&CommandDef{Name: "EMBEDDING.CREATE", Handler: cmdEMBEDDINGCREATE})
	router.Register(&CommandDef{Name: "EMBEDDING.GET", Handler: cmdEMBEDDINGGET})
	router.Register(&CommandDef{Name: "EMBEDDING.SEARCH", Handler: cmdEMBEDDINGSEARCH})
	router.Register(&CommandDef{Name: "EMBEDDING.SIMILAR", Handler: cmdEMBEDDINGSIMILAR})
	router.Register(&CommandDef{Name: "EMBEDDING.DELETE", Handler: cmdEMBEDDINGDELETE})

	router.Register(&CommandDef{Name: "TENSOR.CREATE", Handler: cmdTENSORCREATE})
	router.Register(&CommandDef{Name: "TENSOR.GET", Handler: cmdTENSORGET})
	router.Register(&CommandDef{Name: "TENSOR.ADD", Handler: cmdTENSORADD})
	router.Register(&CommandDef{Name: "TENSOR.MATMUL", Handler: cmdTENSORMATMUL})
	router.Register(&CommandDef{Name: "TENSOR.RESHAPE", Handler: cmdTENSORRESHAPE})
	router.Register(&CommandDef{Name: "TENSOR.DELETE", Handler: cmdTENSORDELETE})

	router.Register(&CommandDef{Name: "CLASSIFIER.CREATE", Handler: cmdCLASSIFIERCREATE})
	router.Register(&CommandDef{Name: "CLASSIFIER.TRAIN", Handler: cmdCLASSIFIERTRAIN})
	router.Register(&CommandDef{Name: "CLASSIFIER.PREDICT", Handler: cmdCLASSIFIERPREDICT})
	router.Register(&CommandDef{Name: "CLASSIFIER.DELETE", Handler: cmdCLASSIFIERDELETE})

	router.Register(&CommandDef{Name: "REGRESSOR.CREATE", Handler: cmdREGRESSORCREATE})
	router.Register(&CommandDef{Name: "REGRESSOR.TRAIN", Handler: cmdREGRESSORTRAIN})
	router.Register(&CommandDef{Name: "REGRESSOR.PREDICT", Handler: cmdREGRESSORPREDICT})
	router.Register(&CommandDef{Name: "REGRESSOR.DELETE", Handler: cmdREGRESSORDELETE})

	router.Register(&CommandDef{Name: "CLUSTER.CREATE", Handler: cmdCLUSTERMLCREATE})
	router.Register(&CommandDef{Name: "CLUSTER.FIT", Handler: cmdCLUSTERFIT})
	router.Register(&CommandDef{Name: "CLUSTER.PREDICT", Handler: cmdCLUSTERPREDICT})
	router.Register(&CommandDef{Name: "CLUSTER.CENTROIDS", Handler: cmdCLUSTERCENTROIDS})
	router.Register(&CommandDef{Name: "CLUSTER.DELETE", Handler: cmdCLUSTERMLDELETE})

	router.Register(&CommandDef{Name: "ANOMALY.CREATE", Handler: cmdANOMALYCREATE})
	router.Register(&CommandDef{Name: "ANOMALY.DETECT", Handler: cmdANOMALYDETECT})
	router.Register(&CommandDef{Name: "ANOMALY.LEARN", Handler: cmdANOMALYLEARN})
	router.Register(&CommandDef{Name: "ANOMALY.DELETE", Handler: cmdANOMALYDELETE})

	router.Register(&CommandDef{Name: "SENTIMENT.ANALYZE", Handler: cmdSENTIMENTANALYZE})
	router.Register(&CommandDef{Name: "SENTIMENT.BATCH", Handler: cmdSENTIMENTBATCH})

	router.Register(&CommandDef{Name: "NLP.TOKENIZE", Handler: cmdNLPTOKENIZE})
	router.Register(&CommandDef{Name: "NLP.ENTITIES", Handler: cmdNLPENTITIES})
	router.Register(&CommandDef{Name: "NLP.KEYWORDS", Handler: cmdNLPKEYWORDS})
	router.Register(&CommandDef{Name: "NLP.SUMMARIZE", Handler: cmdNLPSUMMARIZE})

	router.Register(&CommandDef{Name: "SIMILARITY.COSINE", Handler: cmdSIMILARITYCOSINE})
	router.Register(&CommandDef{Name: "SIMILARITY.EUCLIDEAN", Handler: cmdSIMILARITYEUCLIDEAN})
	router.Register(&CommandDef{Name: "SIMILARITY.JACCARD", Handler: cmdSIMILARITYJACCARD})
	router.Register(&CommandDef{Name: "SIMILARITY.DOTPRODUCT", Handler: cmdSIMILARITYDOTPRODUCT})

	router.Register(&CommandDef{Name: "DATASET.CREATE", Handler: cmdDATASETCREATE})
	router.Register(&CommandDef{Name: "DATASET.ADD", Handler: cmdDATASETADD})
	router.Register(&CommandDef{Name: "DATASET.GET", Handler: cmdDATASETGET})
	router.Register(&CommandDef{Name: "DATASET.SPLIT", Handler: cmdDATASETSPLIT})
	router.Register(&CommandDef{Name: "DATASET.DELETE", Handler: cmdDATASETDELETE})

	router.Register(&CommandDef{Name: "MLXPERIMENT.CREATE", Handler: cmdMLXPERIMENTCREATE})
	router.Register(&CommandDef{Name: "MLXPERIMENT.LOG", Handler: cmdMLXPERIMENTLOG})
	router.Register(&CommandDef{Name: "MLXPERIMENT.METRICS", Handler: cmdMLXPERIMENTMETRICS})
	router.Register(&CommandDef{Name: "MLXPERIMENT.COMPARE", Handler: cmdMLXPERIMENTCOMPARE})
	router.Register(&CommandDef{Name: "MLXPERIMENT.DELETE", Handler: cmdMLXPERIMENTDELETE})

	router.Register(&CommandDef{Name: "PIPELINEML.CREATE", Handler: cmdPIPELINEMLCREATE})
	router.Register(&CommandDef{Name: "PIPELINEML.ADD", Handler: cmdPIPELINEMLADD})
	router.Register(&CommandDef{Name: "PIPELINEML.RUN", Handler: cmdPIPELINEMLRUN})
	router.Register(&CommandDef{Name: "PIPELINEML.DELETE", Handler: cmdPIPELINEMLDELETE})

	router.Register(&CommandDef{Name: "HYPERPARAM.SET", Handler: cmdHYPERPARAMSET})
	router.Register(&CommandDef{Name: "HYPERPARAM.GET", Handler: cmdHYPERPARAMGET})
	router.Register(&CommandDef{Name: "HYPERPARAM.SEARCH", Handler: cmdHYPERPARAMSEARCH})
	router.Register(&CommandDef{Name: "HYPERPARAM.DELETE", Handler: cmdHYPERPARAMDELETE})

	router.Register(&CommandDef{Name: "EVALUATOR.CREATE", Handler: cmdEVALUATORCREATE})
	router.Register(&CommandDef{Name: "EVALUATOR.RUN", Handler: cmdEVALUATORRUN})
	router.Register(&CommandDef{Name: "EVALUATOR.METRICS", Handler: cmdEVALUATORMETRICS})
	router.Register(&CommandDef{Name: "EVALUATOR.DELETE", Handler: cmdEVALUATORDELETE})

	router.Register(&CommandDef{Name: "RECOMMEND.CREATE", Handler: cmdRECOMMENDCREATE})
	router.Register(&CommandDef{Name: "RECOMMEND.TRAIN", Handler: cmdRECOMMENDTRAIN})
	router.Register(&CommandDef{Name: "RECOMMEND.GET", Handler: cmdRECOMMENDGET})
	router.Register(&CommandDef{Name: "RECOMMEND.DELETE", Handler: cmdRECOMMENDDELETE})

	router.Register(&CommandDef{Name: "TIMEFORECAST.CREATE", Handler: cmdTIMEFORECASTCREATE})
	router.Register(&CommandDef{Name: "TIMEFORECAST.TRAIN", Handler: cmdTIMEFORECASTTRAIN})
	router.Register(&CommandDef{Name: "TIMEFORECAST.PREDICT", Handler: cmdTIMEFORECASTPREDICT})
	router.Register(&CommandDef{Name: "TIMEFORECAST.DELETE", Handler: cmdTIMEFORECASTDELETE})
}

var (
	mlModels   = make(map[string]*MLModel)
	mlModelsMx sync.RWMutex
)

type MLModel struct {
	Name      string
	Type      string
	Status    string
	Features  []string
	CreatedAt int64
	TrainedAt int64
}

func cmdMODELCREATE(ctx *Context) error {
	if ctx.ArgCount() < 2 {
		return ctx.WriteError(ErrWrongArgCount)
	}
	name := ctx.ArgString(0)
	modelType := ctx.ArgString(1)
	mlModelsMx.Lock()
	defer mlModelsMx.Unlock()
	mlModels[name] = &MLModel{
		Name:      name,
		Type:      modelType,
		Status:    "created",
		Features:  make([]string, 0),
		CreatedAt: time.Now().UnixMilli(),
	}
	return ctx.WriteOK()
}

func cmdMODELTRAIN(ctx *Context) error {
	if ctx.ArgCount() < 2 {
		return ctx.WriteError(ErrWrongArgCount)
	}
	name := ctx.ArgString(0)
	_ = ctx.ArgString(1)
	mlModelsMx.Lock()
	defer mlModelsMx.Unlock()
	m, exists := mlModels[name]
	if !exists {
		return ctx.WriteError(fmt.Errorf("ERR model not found"))
	}
	m.Status = "trained"
	m.TrainedAt = time.Now().UnixMilli()
	return ctx.WriteOK()
}

func cmdMODELPREDICT(ctx *Context) error {
	if ctx.ArgCount() < 2 {
		return ctx.WriteError(ErrWrongArgCount)
	}
	name := ctx.ArgString(0)
	_ = ctx.ArgString(1)
	mlModelsMx.RLock()
	_, exists := mlModels[name]
	mlModelsMx.RUnlock()
	if !exists {
		return ctx.WriteError(fmt.Errorf("ERR model not found"))
	}
	return ctx.WriteBulkString("prediction_result")
}

func cmdMODELDELETE(ctx *Context) error {
	if ctx.ArgCount() < 1 {
		return ctx.WriteError(ErrWrongArgCount)
	}
	name := ctx.ArgString(0)
	mlModelsMx.Lock()
	defer mlModelsMx.Unlock()
	if _, exists := mlModels[name]; !exists {
		return ctx.WriteInteger(0)
	}
	delete(mlModels, name)
	return ctx.WriteInteger(1)
}

func cmdMODELLIST(ctx *Context) error {
	mlModelsMx.RLock()
	defer mlModelsMx.RUnlock()
	results := make([]*resp.Value, 0)
	for name, m := range mlModels {
		results = append(results, resp.ArrayValue([]*resp.Value{
			resp.BulkString("name"), resp.BulkString(name),
			resp.BulkString("type"), resp.BulkString(m.Type),
			resp.BulkString("status"), resp.BulkString(m.Status),
		}))
	}
	return ctx.WriteArray(results)
}

func cmdMODELSTATUS(ctx *Context) error {
	if ctx.ArgCount() < 1 {
		return ctx.WriteError(ErrWrongArgCount)
	}
	name := ctx.ArgString(0)
	mlModelsMx.RLock()
	m, exists := mlModels[name]
	mlModelsMx.RUnlock()
	if !exists {
		return ctx.WriteError(fmt.Errorf("ERR model not found"))
	}
	return ctx.WriteArray([]*resp.Value{
		resp.BulkString("name"), resp.BulkString(m.Name),
		resp.BulkString("type"), resp.BulkString(m.Type),
		resp.BulkString("status"), resp.BulkString(m.Status),
		resp.BulkString("features"), resp.IntegerValue(int64(len(m.Features))),
	})
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
	defer featuresMx.RUnlock()
	e, exists := features[entity]
	if !exists {
		return ctx.WriteNull()
	}
	val, exists := e[key]
	if !exists {
		return ctx.WriteNull()
	}
	return ctx.WriteBulkString(fmt.Sprintf("%.6f", val))
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

func cmdFEATURENORMALIZE(ctx *Context) error {
	if ctx.ArgCount() < 2 {
		return ctx.WriteError(ErrWrongArgCount)
	}
	entity := ctx.ArgString(0)
	method := ctx.ArgString(1)
	featuresMx.RLock()
	e, exists := features[entity]
	featuresMx.RUnlock()
	if !exists {
		return ctx.WriteError(fmt.Errorf("ERR entity not found"))
	}
	_ = method
	_ = e
	return ctx.WriteOK()
}

func cmdFEATUREVECTOR(ctx *Context) error {
	if ctx.ArgCount() < 1 {
		return ctx.WriteError(ErrWrongArgCount)
	}
	entity := ctx.ArgString(0)
	featuresMx.RLock()
	e, exists := features[entity]
	featuresMx.RUnlock()
	if !exists {
		return ctx.WriteArray([]*resp.Value{})
	}
	results := make([]*resp.Value, 0)
	for k, v := range e {
		results = append(results, resp.ArrayValue([]*resp.Value{
			resp.BulkString(k), resp.BulkString(fmt.Sprintf("%.6f", v)),
		}))
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

func cmdEMBEDDINGSEARCH(ctx *Context) error {
	if ctx.ArgCount() < 2 {
		return ctx.WriteError(ErrWrongArgCount)
	}
	_ = ctx.ArgString(0)
	k := parseInt64(ctx.ArgString(1))
	embeddingsMx.RLock()
	defer embeddingsMx.RUnlock()
	results := make([]*resp.Value, 0)
	count := int64(0)
	for id := range embeddings {
		if count >= k {
			break
		}
		results = append(results, resp.BulkString(id))
		count++
	}
	return ctx.WriteArray(results)
}

func cmdEMBEDDINGSIMILAR(ctx *Context) error {
	if ctx.ArgCount() < 2 {
		return ctx.WriteError(ErrWrongArgCount)
	}
	id := ctx.ArgString(0)
	threshold := parseFloatExt([]byte(ctx.ArgString(1)))
	embeddingsMx.RLock()
	_, exists := embeddings[id]
	embeddingsMx.RUnlock()
	if !exists {
		return ctx.WriteError(fmt.Errorf("ERR embedding not found"))
	}
	_ = threshold
	return ctx.WriteArray([]*resp.Value{resp.BulkString("similar_id")})
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
	tensorsMx.RUnlock()
	if !exists {
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
	return ctx.WriteArray([]*resp.Value{
		resp.BulkString("shape"), resp.ArrayValue(shape),
		resp.BulkString("data"), resp.ArrayValue(data),
	})
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
	result := make([]float64, len(t1.Data))
	for i := range t1.Data {
		result[i] = t1.Data[i] + t2.Data[i]
	}
	t1.Data = result
	return ctx.WriteOK()
}

func cmdTENSORMATMUL(ctx *Context) error {
	if ctx.ArgCount() < 2 {
		return ctx.WriteError(ErrWrongArgCount)
	}
	name := ctx.ArgString(0)
	other := ctx.ArgString(1)
	tensorsMx.RLock()
	_, exists := tensors[name]
	if !exists {
		tensorsMx.RUnlock()
		return ctx.WriteError(fmt.Errorf("ERR tensor not found"))
	}
	_, exists = tensors[other]
	tensorsMx.RUnlock()
	if !exists {
		return ctx.WriteError(fmt.Errorf("ERR other tensor not found"))
	}
	return ctx.WriteBulkString("result_tensor_id")
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
	classifiers   = make(map[string]*Classifier)
	classifiersMx sync.RWMutex
)

type Classifier struct {
	Name    string
	Labels  []string
	Trained bool
}

func cmdCLASSIFIERCREATE(ctx *Context) error {
	if ctx.ArgCount() < 2 {
		return ctx.WriteError(ErrWrongArgCount)
	}
	name := ctx.ArgString(0)
	labels := make([]string, 0)
	for i := 1; i < ctx.ArgCount(); i++ {
		labels = append(labels, ctx.ArgString(i))
	}
	classifiersMx.Lock()
	defer classifiersMx.Unlock()
	classifiers[name] = &Classifier{Name: name, Labels: labels, Trained: false}
	return ctx.WriteOK()
}

func cmdCLASSIFIERTRAIN(ctx *Context) error {
	if ctx.ArgCount() < 1 {
		return ctx.WriteError(ErrWrongArgCount)
	}
	name := ctx.ArgString(0)
	classifiersMx.Lock()
	defer classifiersMx.Unlock()
	c, exists := classifiers[name]
	if !exists {
		return ctx.WriteError(fmt.Errorf("ERR classifier not found"))
	}
	c.Trained = true
	return ctx.WriteOK()
}

func cmdCLASSIFIERPREDICT(ctx *Context) error {
	if ctx.ArgCount() < 2 {
		return ctx.WriteError(ErrWrongArgCount)
	}
	name := ctx.ArgString(0)
	_ = ctx.ArgString(1)
	classifiersMx.RLock()
	c, exists := classifiers[name]
	classifiersMx.RUnlock()
	if !exists {
		return ctx.WriteError(fmt.Errorf("ERR classifier not found"))
	}
	if len(c.Labels) == 0 {
		return ctx.WriteNull()
	}
	return ctx.WriteBulkString(c.Labels[0])
}

func cmdCLASSIFIERDELETE(ctx *Context) error {
	if ctx.ArgCount() < 1 {
		return ctx.WriteError(ErrWrongArgCount)
	}
	name := ctx.ArgString(0)
	classifiersMx.Lock()
	defer classifiersMx.Unlock()
	if _, exists := classifiers[name]; !exists {
		return ctx.WriteInteger(0)
	}
	delete(classifiers, name)
	return ctx.WriteInteger(1)
}

var (
	regressors   = make(map[string]*Regressor)
	regressorsMx sync.RWMutex
)

type Regressor struct {
	Name    string
	Trained bool
}

func cmdREGRESSORCREATE(ctx *Context) error {
	if ctx.ArgCount() < 1 {
		return ctx.WriteError(ErrWrongArgCount)
	}
	name := ctx.ArgString(0)
	regressorsMx.Lock()
	defer regressorsMx.Unlock()
	regressors[name] = &Regressor{Name: name, Trained: false}
	return ctx.WriteOK()
}

func cmdREGRESSORTRAIN(ctx *Context) error {
	if ctx.ArgCount() < 1 {
		return ctx.WriteError(ErrWrongArgCount)
	}
	name := ctx.ArgString(0)
	regressorsMx.Lock()
	defer regressorsMx.Unlock()
	r, exists := regressors[name]
	if !exists {
		return ctx.WriteError(fmt.Errorf("ERR regressor not found"))
	}
	r.Trained = true
	return ctx.WriteOK()
}

func cmdREGRESSORPREDICT(ctx *Context) error {
	if ctx.ArgCount() < 2 {
		return ctx.WriteError(ErrWrongArgCount)
	}
	name := ctx.ArgString(0)
	_ = ctx.ArgString(1)
	regressorsMx.RLock()
	_, exists := regressors[name]
	regressorsMx.RUnlock()
	if !exists {
		return ctx.WriteError(fmt.Errorf("ERR regressor not found"))
	}
	return ctx.WriteBulkString("0.5")
}

func cmdREGRESSORDELETE(ctx *Context) error {
	if ctx.ArgCount() < 1 {
		return ctx.WriteError(ErrWrongArgCount)
	}
	name := ctx.ArgString(0)
	regressorsMx.Lock()
	defer regressorsMx.Unlock()
	if _, exists := regressors[name]; !exists {
		return ctx.WriteInteger(0)
	}
	delete(regressors, name)
	return ctx.WriteInteger(1)
}

var (
	clusterMLs   = make(map[string]*ClusterML)
	clusterMLsMx sync.RWMutex
)

type ClusterML struct {
	Name      string
	K         int
	Trained   bool
	Centroids [][]float64
}

func cmdCLUSTERMLCREATE(ctx *Context) error {
	if ctx.ArgCount() < 2 {
		return ctx.WriteError(ErrWrongArgCount)
	}
	name := ctx.ArgString(0)
	k := int(parseInt64(ctx.ArgString(1)))
	clusterMLsMx.Lock()
	defer clusterMLsMx.Unlock()
	clusterMLs[name] = &ClusterML{Name: name, K: k, Trained: false, Centroids: make([][]float64, 0)}
	return ctx.WriteOK()
}

func cmdCLUSTERFIT(ctx *Context) error {
	if ctx.ArgCount() < 1 {
		return ctx.WriteError(ErrWrongArgCount)
	}
	name := ctx.ArgString(0)
	clusterMLsMx.Lock()
	defer clusterMLsMx.Unlock()
	c, exists := clusterMLs[name]
	if !exists {
		return ctx.WriteError(fmt.Errorf("ERR cluster not found"))
	}
	c.Trained = true
	c.Centroids = make([][]float64, c.K)
	for i := 0; i < c.K; i++ {
		c.Centroids[i] = []float64{0.0}
	}
	return ctx.WriteOK()
}

func cmdCLUSTERPREDICT(ctx *Context) error {
	if ctx.ArgCount() < 2 {
		return ctx.WriteError(ErrWrongArgCount)
	}
	name := ctx.ArgString(0)
	_ = ctx.ArgString(1)
	clusterMLsMx.RLock()
	_, exists := clusterMLs[name]
	clusterMLsMx.RUnlock()
	if !exists {
		return ctx.WriteError(fmt.Errorf("ERR cluster not found"))
	}
	return ctx.WriteInteger(0)
}

func cmdCLUSTERCENTROIDS(ctx *Context) error {
	if ctx.ArgCount() < 1 {
		return ctx.WriteError(ErrWrongArgCount)
	}
	name := ctx.ArgString(0)
	clusterMLsMx.RLock()
	c, exists := clusterMLs[name]
	clusterMLsMx.RUnlock()
	if !exists {
		return ctx.WriteError(fmt.Errorf("ERR cluster not found"))
	}
	results := make([]*resp.Value, 0)
	for i, centroid := range c.Centroids {
		pts := make([]*resp.Value, len(centroid))
		for j, pt := range centroid {
			pts[j] = resp.BulkString(fmt.Sprintf("%.6f", pt))
		}
		results = append(results, resp.ArrayValue([]*resp.Value{
			resp.IntegerValue(int64(i)), resp.ArrayValue(pts),
		}))
	}
	return ctx.WriteArray(results)
}

func cmdCLUSTERMLDELETE(ctx *Context) error {
	if ctx.ArgCount() < 1 {
		return ctx.WriteError(ErrWrongArgCount)
	}
	name := ctx.ArgString(0)
	clusterMLsMx.Lock()
	defer clusterMLsMx.Unlock()
	if _, exists := clusterMLs[name]; !exists {
		return ctx.WriteInteger(0)
	}
	delete(clusterMLs, name)
	return ctx.WriteInteger(1)
}

var (
	anomalyDetectors   = make(map[string]*AnomalyDetector)
	anomalyDetectorsMx sync.RWMutex
)

type AnomalyDetector struct {
	Name    string
	Trained bool
}

func cmdANOMALYCREATE(ctx *Context) error {
	if ctx.ArgCount() < 1 {
		return ctx.WriteError(ErrWrongArgCount)
	}
	name := ctx.ArgString(0)
	anomalyDetectorsMx.Lock()
	defer anomalyDetectorsMx.Unlock()
	anomalyDetectors[name] = &AnomalyDetector{Name: name, Trained: false}
	return ctx.WriteOK()
}

func cmdANOMALYDETECT(ctx *Context) error {
	if ctx.ArgCount() < 2 {
		return ctx.WriteError(ErrWrongArgCount)
	}
	name := ctx.ArgString(0)
	_ = ctx.ArgString(1)
	anomalyDetectorsMx.RLock()
	_, exists := anomalyDetectors[name]
	anomalyDetectorsMx.RUnlock()
	if !exists {
		return ctx.WriteError(fmt.Errorf("ERR anomaly detector not found"))
	}
	return ctx.WriteInteger(0)
}

func cmdANOMALYLEARN(ctx *Context) error {
	if ctx.ArgCount() < 1 {
		return ctx.WriteError(ErrWrongArgCount)
	}
	name := ctx.ArgString(0)
	anomalyDetectorsMx.Lock()
	defer anomalyDetectorsMx.Unlock()
	a, exists := anomalyDetectors[name]
	if !exists {
		return ctx.WriteError(fmt.Errorf("ERR anomaly detector not found"))
	}
	a.Trained = true
	return ctx.WriteOK()
}

func cmdANOMALYDELETE(ctx *Context) error {
	if ctx.ArgCount() < 1 {
		return ctx.WriteError(ErrWrongArgCount)
	}
	name := ctx.ArgString(0)
	anomalyDetectorsMx.Lock()
	defer anomalyDetectorsMx.Unlock()
	if _, exists := anomalyDetectors[name]; !exists {
		return ctx.WriteInteger(0)
	}
	delete(anomalyDetectors, name)
	return ctx.WriteInteger(1)
}

func cmdSENTIMENTANALYZE(ctx *Context) error {
	if ctx.ArgCount() < 1 {
		return ctx.WriteError(ErrWrongArgCount)
	}
	_ = ctx.ArgString(0)
	return ctx.WriteArray([]*resp.Value{
		resp.BulkString("sentiment"), resp.BulkString("neutral"),
		resp.BulkString("score"), resp.BulkString("0.0"),
	})
}

func cmdSENTIMENTBATCH(ctx *Context) error {
	if ctx.ArgCount() < 1 {
		return ctx.WriteError(ErrWrongArgCount)
	}
	results := make([]*resp.Value, 0)
	for i := 0; i < ctx.ArgCount(); i++ {
		results = append(results, resp.ArrayValue([]*resp.Value{
			resp.BulkString("sentiment"), resp.BulkString("neutral"),
		}))
	}
	return ctx.WriteArray(results)
}

func cmdNLPTOKENIZE(ctx *Context) error {
	if ctx.ArgCount() < 1 {
		return ctx.WriteError(ErrWrongArgCount)
	}
	text := ctx.ArgString(0)
	tokens := make([]*resp.Value, 0)
	for _, t := range []byte(text) {
		if t == ' ' {
			continue
		}
		tokens = append(tokens, resp.BulkString(string(t)))
	}
	return ctx.WriteArray(tokens)
}

func cmdNLPENTITIES(ctx *Context) error {
	if ctx.ArgCount() < 1 {
		return ctx.WriteError(ErrWrongArgCount)
	}
	_ = ctx.ArgString(0)
	return ctx.WriteArray([]*resp.Value{})
}

func cmdNLPKEYWORDS(ctx *Context) error {
	if ctx.ArgCount() < 1 {
		return ctx.WriteError(ErrWrongArgCount)
	}
	_ = ctx.ArgString(0)
	return ctx.WriteArray([]*resp.Value{})
}

func cmdNLPSUMMARIZE(ctx *Context) error {
	if ctx.ArgCount() < 1 {
		return ctx.WriteError(ErrWrongArgCount)
	}
	text := ctx.ArgString(0)
	if len(text) > 100 {
		return ctx.WriteBulkString(text[:100] + "...")
	}
	return ctx.WriteBulkString(text)
}

func cmdSIMILARITYCOSINE(ctx *Context) error {
	if ctx.ArgCount() < 2 {
		return ctx.WriteError(ErrWrongArgCount)
	}
	_ = ctx.ArgString(0)
	_ = ctx.ArgString(1)
	return ctx.WriteBulkString("0.5")
}

func cmdSIMILARITYEUCLIDEAN(ctx *Context) error {
	if ctx.ArgCount() < 2 {
		return ctx.WriteError(ErrWrongArgCount)
	}
	_ = ctx.ArgString(0)
	_ = ctx.ArgString(1)
	return ctx.WriteBulkString("0.5")
}

func cmdSIMILARITYJACCARD(ctx *Context) error {
	if ctx.ArgCount() < 2 {
		return ctx.WriteError(ErrWrongArgCount)
	}
	_ = ctx.ArgString(0)
	_ = ctx.ArgString(1)
	return ctx.WriteBulkString("0.5")
}

func cmdSIMILARITYDOTPRODUCT(ctx *Context) error {
	if ctx.ArgCount() < 2 {
		return ctx.WriteError(ErrWrongArgCount)
	}
	_ = ctx.ArgString(0)
	_ = ctx.ArgString(1)
	return ctx.WriteBulkString("0.5")
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
	datasetsMx.RUnlock()
	if !exists {
		return ctx.WriteNull()
	}
	results := make([]*resp.Value, len(d.Data))
	for i, item := range d.Data {
		results[i] = resp.BulkString(item)
	}
	return ctx.WriteArray(results)
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

func cmdMLXPERIMENTCOMPARE(ctx *Context) error {
	if ctx.ArgCount() < 2 {
		return ctx.WriteError(ErrWrongArgCount)
	}
	_ = ctx.ArgString(0)
	_ = ctx.ArgString(1)
	return ctx.WriteArray([]*resp.Value{
		resp.BulkString("diff"), resp.BulkString("0.0"),
	})
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
	mlPipelines   = make(map[string]*MLPipeline)
	mlPipelinesMx sync.RWMutex
)

type MLPipeline struct {
	Name  string
	Steps []string
}

func cmdPIPELINEMLCREATE(ctx *Context) error {
	if ctx.ArgCount() < 1 {
		return ctx.WriteError(ErrWrongArgCount)
	}
	name := ctx.ArgString(0)
	mlPipelinesMx.Lock()
	defer mlPipelinesMx.Unlock()
	mlPipelines[name] = &MLPipeline{Name: name, Steps: make([]string, 0)}
	return ctx.WriteOK()
}

func cmdPIPELINEMLADD(ctx *Context) error {
	if ctx.ArgCount() < 2 {
		return ctx.WriteError(ErrWrongArgCount)
	}
	name := ctx.ArgString(0)
	step := ctx.ArgString(1)
	mlPipelinesMx.Lock()
	defer mlPipelinesMx.Unlock()
	p, exists := mlPipelines[name]
	if !exists {
		return ctx.WriteError(fmt.Errorf("ERR pipeline not found"))
	}
	p.Steps = append(p.Steps, step)
	return ctx.WriteOK()
}

func cmdPIPELINEMLRUN(ctx *Context) error {
	if ctx.ArgCount() < 1 {
		return ctx.WriteError(ErrWrongArgCount)
	}
	name := ctx.ArgString(0)
	mlPipelinesMx.RLock()
	p, exists := mlPipelines[name]
	mlPipelinesMx.RUnlock()
	if !exists {
		return ctx.WriteError(fmt.Errorf("ERR pipeline not found"))
	}
	return ctx.WriteInteger(int64(len(p.Steps)))
}

func cmdPIPELINEMLDELETE(ctx *Context) error {
	if ctx.ArgCount() < 1 {
		return ctx.WriteError(ErrWrongArgCount)
	}
	name := ctx.ArgString(0)
	mlPipelinesMx.Lock()
	defer mlPipelinesMx.Unlock()
	if _, exists := mlPipelines[name]; !exists {
		return ctx.WriteInteger(0)
	}
	delete(mlPipelines, name)
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

func cmdHYPERPARAMSEARCH(ctx *Context) error {
	if ctx.ArgCount() < 1 {
		return ctx.WriteError(ErrWrongArgCount)
	}
	_ = ctx.ArgString(0)
	return ctx.WriteArray([]*resp.Value{
		resp.ArrayValue([]*resp.Value{resp.BulkString("param1"), resp.BulkString("value1")}),
	})
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

var (
	evaluators   = make(map[string]*Evaluator)
	evaluatorsMx sync.RWMutex
)

type Evaluator struct {
	Name    string
	Metrics []string
}

func cmdEVALUATORCREATE(ctx *Context) error {
	if ctx.ArgCount() < 1 {
		return ctx.WriteError(ErrWrongArgCount)
	}
	name := ctx.ArgString(0)
	metrics := make([]string, 0)
	for i := 1; i < ctx.ArgCount(); i++ {
		metrics = append(metrics, ctx.ArgString(i))
	}
	evaluatorsMx.Lock()
	defer evaluatorsMx.Unlock()
	evaluators[name] = &Evaluator{Name: name, Metrics: metrics}
	return ctx.WriteOK()
}

func cmdEVALUATORRUN(ctx *Context) error {
	if ctx.ArgCount() < 1 {
		return ctx.WriteError(ErrWrongArgCount)
	}
	name := ctx.ArgString(0)
	evaluatorsMx.RLock()
	e, exists := evaluators[name]
	evaluatorsMx.RUnlock()
	if !exists {
		return ctx.WriteError(fmt.Errorf("ERR evaluator not found"))
	}
	results := make([]*resp.Value, 0)
	for _, m := range e.Metrics {
		results = append(results, resp.ArrayValue([]*resp.Value{
			resp.BulkString(m), resp.BulkString("0.5"),
		}))
	}
	return ctx.WriteArray(results)
}

func cmdEVALUATORMETRICS(ctx *Context) error {
	if ctx.ArgCount() < 1 {
		return ctx.WriteError(ErrWrongArgCount)
	}
	name := ctx.ArgString(0)
	evaluatorsMx.RLock()
	e, exists := evaluators[name]
	evaluatorsMx.RUnlock()
	if !exists {
		return ctx.WriteError(fmt.Errorf("ERR evaluator not found"))
	}
	results := make([]*resp.Value, len(e.Metrics))
	for i, m := range e.Metrics {
		results[i] = resp.BulkString(m)
	}
	return ctx.WriteArray(results)
}

func cmdEVALUATORDELETE(ctx *Context) error {
	if ctx.ArgCount() < 1 {
		return ctx.WriteError(ErrWrongArgCount)
	}
	name := ctx.ArgString(0)
	evaluatorsMx.Lock()
	defer evaluatorsMx.Unlock()
	if _, exists := evaluators[name]; !exists {
		return ctx.WriteInteger(0)
	}
	delete(evaluators, name)
	return ctx.WriteInteger(1)
}

var (
	recommenders   = make(map[string]*Recommender)
	recommendersMx sync.RWMutex
)

type Recommender struct {
	Name string
}

func cmdRECOMMENDCREATE(ctx *Context) error {
	if ctx.ArgCount() < 1 {
		return ctx.WriteError(ErrWrongArgCount)
	}
	name := ctx.ArgString(0)
	recommendersMx.Lock()
	defer recommendersMx.Unlock()
	recommenders[name] = &Recommender{Name: name}
	return ctx.WriteOK()
}

func cmdRECOMMENDTRAIN(ctx *Context) error {
	if ctx.ArgCount() < 1 {
		return ctx.WriteError(ErrWrongArgCount)
	}
	name := ctx.ArgString(0)
	recommendersMx.Lock()
	defer recommendersMx.Unlock()
	_, exists := recommenders[name]
	if !exists {
		return ctx.WriteError(fmt.Errorf("ERR recommender not found"))
	}
	return ctx.WriteOK()
}

func cmdRECOMMENDGET(ctx *Context) error {
	if ctx.ArgCount() < 2 {
		return ctx.WriteError(ErrWrongArgCount)
	}
	name := ctx.ArgString(0)
	user := ctx.ArgString(1)
	recommendersMx.RLock()
	_, exists := recommenders[name]
	recommendersMx.RUnlock()
	if !exists {
		return ctx.WriteError(fmt.Errorf("ERR recommender not found"))
	}
	_ = user
	return ctx.WriteArray([]*resp.Value{
		resp.BulkString("item1"), resp.BulkString("item2"),
	})
}

func cmdRECOMMENDDELETE(ctx *Context) error {
	if ctx.ArgCount() < 1 {
		return ctx.WriteError(ErrWrongArgCount)
	}
	name := ctx.ArgString(0)
	recommendersMx.Lock()
	defer recommendersMx.Unlock()
	if _, exists := recommenders[name]; !exists {
		return ctx.WriteInteger(0)
	}
	delete(recommenders, name)
	return ctx.WriteInteger(1)
}

var (
	timeForecasters   = make(map[string]*TimeForecaster)
	timeForecastersMx sync.RWMutex
)

type TimeForecaster struct {
	Name string
}

func cmdTIMEFORECASTCREATE(ctx *Context) error {
	if ctx.ArgCount() < 1 {
		return ctx.WriteError(ErrWrongArgCount)
	}
	name := ctx.ArgString(0)
	timeForecastersMx.Lock()
	defer timeForecastersMx.Unlock()
	timeForecasters[name] = &TimeForecaster{Name: name}
	return ctx.WriteOK()
}

func cmdTIMEFORECASTTRAIN(ctx *Context) error {
	if ctx.ArgCount() < 1 {
		return ctx.WriteError(ErrWrongArgCount)
	}
	name := ctx.ArgString(0)
	timeForecastersMx.Lock()
	defer timeForecastersMx.Unlock()
	_, exists := timeForecasters[name]
	if !exists {
		return ctx.WriteError(fmt.Errorf("ERR forecaster not found"))
	}
	return ctx.WriteOK()
}

func cmdTIMEFORECASTPREDICT(ctx *Context) error {
	if ctx.ArgCount() < 2 {
		return ctx.WriteError(ErrWrongArgCount)
	}
	name := ctx.ArgString(0)
	steps := parseInt64(ctx.ArgString(1))
	timeForecastersMx.RLock()
	_, exists := timeForecasters[name]
	timeForecastersMx.RUnlock()
	if !exists {
		return ctx.WriteError(fmt.Errorf("ERR forecaster not found"))
	}
	results := make([]*resp.Value, steps)
	for i := int64(0); i < steps; i++ {
		results[i] = resp.BulkString(fmt.Sprintf("%.2f", float64(i)*0.1))
	}
	return ctx.WriteArray(results)
}

func cmdTIMEFORECASTDELETE(ctx *Context) error {
	if ctx.ArgCount() < 1 {
		return ctx.WriteError(ErrWrongArgCount)
	}
	name := ctx.ArgString(0)
	timeForecastersMx.Lock()
	defer timeForecastersMx.Unlock()
	if _, exists := timeForecasters[name]; !exists {
		return ctx.WriteInteger(0)
	}
	delete(timeForecasters, name)
	return ctx.WriteInteger(1)
}
