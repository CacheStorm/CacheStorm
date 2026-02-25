package command

import (
	"testing"

	"github.com/cachestorm/cachestorm/internal/store"
)

func TestMLCommandsExtensive(t *testing.T) {
	s := store.NewStore()
	router := NewRouter()
	RegisterMLCommands(router)

	tests := []struct {
		name string
		cmd  string
		args [][]byte
	}{
		{"ML.MODEL.CREATE no args", "ML.MODEL.CREATE", nil},
		{"ML.MODEL.CREATE model", "ML.MODEL.CREATE", [][]byte{[]byte("model1"), []byte("neural")}},
		{"ML.MODEL.TRAIN no args", "ML.MODEL.TRAIN", nil},
		{"ML.MODEL.TRAIN not found", "ML.MODEL.TRAIN", [][]byte{[]byte("notfound"), []byte("data")}},
		{"ML.MODEL.PREDICT no args", "ML.MODEL.PREDICT", nil},
		{"ML.MODEL.PREDICT not found", "ML.MODEL.PREDICT", [][]byte{[]byte("notfound"), []byte("input")}},
		{"ML.MODEL.SAVE no args", "ML.MODEL.SAVE", nil},
		{"ML.MODEL.SAVE not found", "ML.MODEL.SAVE", [][]byte{[]byte("notfound"), []byte("path")}},
		{"ML.MODEL.LOAD no args", "ML.MODEL.LOAD", nil},
		{"ML.MODEL.LOAD path", "ML.MODEL.LOAD", [][]byte{[]byte("model2"), []byte("path")}},
		{"ML.MODEL.DELETE no args", "ML.MODEL.DELETE", nil},
		{"ML.MODEL.DELETE not found", "ML.MODEL.DELETE", [][]byte{[]byte("notfound")}},
		{"ML.MODEL.LIST", "ML.MODEL.LIST", nil},
		{"ML.MODEL.INFO no args", "ML.MODEL.INFO", nil},
		{"ML.MODEL.INFO not found", "ML.MODEL.INFO", [][]byte{[]byte("notfound")}},
		{"ML.FEATURE.EXTRACT no args", "ML.FEATURE.EXTRACT", nil},
		{"ML.FEATURE.EXTRACT data", "ML.FEATURE.EXTRACT", [][]byte{[]byte("data1"), []byte("method")}},
		{"ML.FEATURE.SELECT no args", "ML.FEATURE.SELECT", nil},
		{"ML.FEATURE.SELECT features", "ML.FEATURE.SELECT", [][]byte{[]byte("[1,2,3]"), []byte("2")}},
		{"ML.FEATURE.SCALE no args", "ML.FEATURE.SCALE", nil},
		{"ML.FEATURE.SCALE data", "ML.FEATURE.SCALE", [][]byte{[]byte("[1,2,3]")}},
		{"ML.FEATURE.VECTOR no args", "ML.FEATURE.VECTOR", nil},
		{"ML.FEATURE.VECTOR text", "ML.FEATURE.VECTOR", [][]byte{[]byte("text1"), []byte("tfidf")}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			runCommandTest(t, router, s, tt.cmd, tt.args)
		})
	}
}

func TestMLCommandsAdvanced(t *testing.T) {
	s := store.NewStore()
	router := NewRouter()
	RegisterMLCommands(router)

	tests := []struct {
		name string
		cmd  string
		args [][]byte
	}{
		{"TENSOR.CREATE no args", "TENSOR.CREATE", nil},
		{"TENSOR.CREATE tensor", "TENSOR.CREATE", [][]byte{[]byte("tensor1"), []byte("[1,2,3]")}},
		{"TENSOR.GET no args", "TENSOR.GET", nil},
		{"TENSOR.GET not found", "TENSOR.GET", [][]byte{[]byte("notfound")}},
		{"TENSOR.DELETE no args", "TENSOR.DELETE", nil},
		{"TENSOR.DELETE not found", "TENSOR.DELETE", [][]byte{[]byte("notfound")}},
		{"TENSOR.SHAPE no args", "TENSOR.SHAPE", nil},
		{"TENSOR.SHAPE not found", "TENSOR.SHAPE", [][]byte{[]byte("notfound")}},
		{"TENSOR.RESHAPE no args", "TENSOR.RESHAPE", nil},
		{"TENSOR.RESHAPE not found", "TENSOR.RESHAPE", [][]byte{[]byte("notfound"), []byte("[2,2]")}},
		{"TENSOR.SLICE no args", "TENSOR.SLICE", nil},
		{"TENSOR.SLICE not found", "TENSOR.SLICE", [][]byte{[]byte("notfound"), []byte("0:2")}},
		{"TENSOR.CONCAT no args", "TENSOR.CONCAT", nil},
		{"TENSOR.CONCAT missing args", "TENSOR.CONCAT", [][]byte{[]byte("tensor1")}},
		{"TENSOR.ADD no args", "TENSOR.ADD", nil},
		{"TENSOR.ADD missing args", "TENSOR.ADD", [][]byte{[]byte("tensor1")}},
		{"TENSOR.MUL no args", "TENSOR.MUL", nil},
		{"TENSOR.MUL missing args", "TENSOR.MUL", [][]byte{[]byte("tensor1")}},
		{"TENSOR.TRANSPOSE no args", "TENSOR.TRANSPOSE", nil},
		{"TENSOR.TRANSPOSE not found", "TENSOR.TRANSPOSE", [][]byte{[]byte("notfound")}},
		{"TENSOR.DOT no args", "TENSOR.DOT", nil},
		{"TENSOR.DOT missing args", "TENSOR.DOT", [][]byte{[]byte("tensor1")}},
		{"TENSOR.NORM no args", "TENSOR.NORM", nil},
		{"TENSOR.NORM not found", "TENSOR.NORM", [][]byte{[]byte("notfound")}},
		{"TENSOR.MEAN no args", "TENSOR.MEAN", nil},
		{"TENSOR.MEAN not found", "TENSOR.MEAN", [][]byte{[]byte("notfound")}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			runCommandTest(t, router, s, tt.cmd, tt.args)
		})
	}
}

func TestMLCommandsDataset(t *testing.T) {
	s := store.NewStore()
	router := NewRouter()
	RegisterMLCommands(router)

	tests := []struct {
		name string
		cmd  string
		args [][]byte
	}{
		{"DATASET.CREATE no args", "DATASET.CREATE", nil},
		{"DATASET.CREATE dataset", "DATASET.CREATE", [][]byte{[]byte("dataset1")}},
		{"DATASET.ADD no args", "DATASET.ADD", nil},
		{"DATASET.ADD not found", "DATASET.ADD", [][]byte{[]byte("notfound"), []byte("data")}},
		{"DATASET.GET no args", "DATASET.GET", nil},
		{"DATASET.GET not found", "DATASET.GET", [][]byte{[]byte("notfound"), []byte("0")}},
		{"DATASET.SIZE no args", "DATASET.SIZE", nil},
		{"DATASET.SIZE not found", "DATASET.SIZE", [][]byte{[]byte("notfound")}},
		{"DATASET.SPLIT no args", "DATASET.SPLIT", nil},
		{"DATASET.SPLIT not found", "DATASET.SPLIT", [][]byte{[]byte("notfound"), []byte("0.8")}},
		{"DATASET.SHUFFLE no args", "DATASET.SHUFFLE", nil},
		{"DATASET.SHUFFLE not found", "DATASET.SHUFFLE", [][]byte{[]byte("notfound")}},
		{"DATASET.BATCH no args", "DATASET.BATCH", nil},
		{"DATASET.BATCH not found", "DATASET.BATCH", [][]byte{[]byte("notfound"), []byte("32")}},
		{"DATASET.NORMALIZE no args", "DATASET.NORMALIZE", nil},
		{"DATASET.NORMALIZE not found", "DATASET.NORMALIZE", [][]byte{[]byte("notfound")}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			runCommandTest(t, router, s, tt.cmd, tt.args)
		})
	}
}
