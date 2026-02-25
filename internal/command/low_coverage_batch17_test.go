package command

import (
	"testing"

	"github.com/cachestorm/cachestorm/internal/store"
)

func TestLowCoverageBatch17_GeoDistance(t *testing.T) {
	s := store.NewStore()
	router := NewRouter()
	RegisterIntegrationCommands(router)

	t.Run("GEO.DISTANCE wrong args", func(t *testing.T) {
		_ = runHandler(t, router, "GEO.DISTANCE", nil, s)
	})

	t.Run("GEO.DISTANCE same point", func(t *testing.T) {
		_ = runHandler(t, router, "GEO.DISTANCE", [][]byte{
			[]byte("0"), []byte("0"), []byte("0"), []byte("0"),
		}, s)
	})

	t.Run("GEO.DISTANCE different points", func(t *testing.T) {
		_ = runHandler(t, router, "GEO.DISTANCE", [][]byte{
			[]byte("40.7128"), []byte("-74.0060"), []byte("51.5074"), []byte("-0.1278"),
		}, s)
	})

	t.Run("GEO.DISTANCE with unit km", func(t *testing.T) {
		_ = runHandler(t, router, "GEO.DISTANCE", [][]byte{
			[]byte("0"), []byte("0"), []byte("10"), []byte("10"), []byte("km"),
		}, s)
	})

	t.Run("GEO.DISTANCE with unit m", func(t *testing.T) {
		_ = runHandler(t, router, "GEO.DISTANCE", [][]byte{
			[]byte("0"), []byte("0"), []byte("10"), []byte("10"), []byte("m"),
		}, s)
	})

	t.Run("GEO.DISTANCE with unit mi", func(t *testing.T) {
		_ = runHandler(t, router, "GEO.DISTANCE", [][]byte{
			[]byte("0"), []byte("0"), []byte("10"), []byte("10"), []byte("mi"),
		}, s)
	})

	t.Run("GEO.DISTANCE with unit ft", func(t *testing.T) {
		_ = runHandler(t, router, "GEO.DISTANCE", [][]byte{
			[]byte("0"), []byte("0"), []byte("10"), []byte("10"), []byte("ft"),
		}, s)
	})

	t.Run("GEO.DISTANCE negative lat/lon", func(t *testing.T) {
		_ = runHandler(t, router, "GEO.DISTANCE", [][]byte{
			[]byte("-10"), []byte("-10"), []byte("10"), []byte("10"),
		}, s)
	})

	t.Run("GEO.DISTANCE crossing equator", func(t *testing.T) {
		_ = runHandler(t, router, "GEO.DISTANCE", [][]byte{
			[]byte("-5"), []byte("0"), []byte("5"), []byte("0"),
		}, s)
	})

	t.Run("GEO.DISTANCE crossing prime meridian", func(t *testing.T) {
		_ = runHandler(t, router, "GEO.DISTANCE", [][]byte{
			[]byte("0"), []byte("-5"), []byte("0"), []byte("5"),
		}, s)
	})
}

func TestLowCoverageBatch17_SpatialWithin(t *testing.T) {
	s := store.NewStore()
	router := NewRouter()
	RegisterMVCCCommands(router)

	t.Run("SPATIAL.WITHIN create and query", func(t *testing.T) {
		_ = runHandler(t, router, "SPATIAL.CREATE", [][]byte{[]byte("myindex")}, s)
		_ = runHandler(t, router, "SPATIAL.ADD", [][]byte{
			[]byte("myindex"),
			[]byte("point1"),
			[]byte("40.7128"),
			[]byte("-74.0060"),
		}, s)
		_ = runHandler(t, router, "SPATIAL.WITHIN", [][]byte{
			[]byte("myindex"),
			[]byte("40.7128"),
			[]byte("-74.0060"),
			[]byte("100"),
		}, s)
	})

	t.Run("SPATIAL.WITHIN non-existent index", func(t *testing.T) {
		_ = runHandler(t, router, "SPATIAL.WITHIN", [][]byte{
			[]byte("nonexistent"),
			[]byte("0"),
			[]byte("0"),
			[]byte("100"),
		}, s)
	})
}

func TestLowCoverageBatch17_ArrayMergeMore(t *testing.T) {
	s := store.NewStore()
	router := NewRouter()
	RegisterIntegrationCommands(router)

	t.Run("ARRAY.MERGE with multiple elements", func(t *testing.T) {
		_ = runHandler(t, router, "ARRAY.PUSH", [][]byte{[]byte("arr1"), []byte("a"), []byte("b"), []byte("c")}, s)
		_ = runHandler(t, router, "ARRAY.PUSH", [][]byte{[]byte("arr2"), []byte("d"), []byte("e"), []byte("f")}, s)
		_ = runHandler(t, router, "ARRAY.MERGE", [][]byte{[]byte("arr1"), []byte("arr2")}, s)
	})

	t.Run("ARRAY.MERGE with empty arrays", func(t *testing.T) {
		_ = runHandler(t, router, "ARRAY.PUSH", [][]byte{[]byte("empty_arr")}, s)
		_ = runHandler(t, router, "ARRAY.MERGE", [][]byte{[]byte("empty_dest"), []byte("empty_arr")}, s)
	})

	t.Run("ARRAY.MERGE with wrong args", func(t *testing.T) {
		_ = runHandler(t, router, "ARRAY.MERGE", [][]byte{[]byte("onlyone")}, s)
	})
}

func TestLowCoverageBatch17_ObjectMergeMore(t *testing.T) {
	s := store.NewStore()
	router := NewRouter()
	RegisterIntegrationCommands(router)

	t.Run("OBJECT.MERGE with multiple keys", func(t *testing.T) {
		_ = runHandler(t, router, "OBJECT.SET", [][]byte{[]byte("obj1"), []byte("k1"), []byte("v1")}, s)
		_ = runHandler(t, router, "OBJECT.SET", [][]byte{[]byte("obj1"), []byte("k2"), []byte("v2")}, s)
		_ = runHandler(t, router, "OBJECT.SET", [][]byte{[]byte("obj2"), []byte("k3"), []byte("v3")}, s)
		_ = runHandler(t, router, "OBJECT.SET", [][]byte{[]byte("obj2"), []byte("k4"), []byte("v4")}, s)
		_ = runHandler(t, router, "OBJECT.MERGE", [][]byte{[]byte("obj1"), []byte("obj2")}, s)
	})

	t.Run("OBJECT.MERGE with wrong args", func(t *testing.T) {
		_ = runHandler(t, router, "OBJECT.MERGE", [][]byte{[]byte("onlyone")}, s)
	})
}

func TestLowCoverageBatch17_CallFunctionMore(t *testing.T) {
	s := store.NewStore()
	router := NewRouter()
	RegisterFunctionCommands(router)

	t.Run("FUNCTION.CREATE with multiple functions", func(t *testing.T) {
		_ = runHandler(t, router, "FUNCTION", [][]byte{
			[]byte("CREATE"),
			[]byte("multilib"),
			[]byte("redis.add = function(a, b) return tonumber(a) + tonumber(b) end redis.sub = function(a, b) return tonumber(a) - tonumber(b) end"),
		}, s)

		_ = runHandler(t, router, "FCALL", [][]byte{
			[]byte("multilib.add"),
			[]byte("0"),
			[]byte("10"),
			[]byte("5"),
		}, s)

		_ = runHandler(t, router, "FCALL", [][]byte{
			[]byte("multilib.sub"),
			[]byte("0"),
			[]byte("10"),
			[]byte("5"),
		}, s)
	})

	t.Run("FCALL with function not found", func(t *testing.T) {
		_ = runHandler(t, router, "FUNCTION", [][]byte{
			[]byte("CREATE"),
			[]byte("simplib"),
			[]byte("redis.only = function() return 1 end"),
		}, s)

		_ = runHandler(t, router, "FCALL", [][]byte{
			[]byte("simplib.notfound"),
			[]byte("0"),
		}, s)
	})

	t.Run("FUNCTION DELETE", func(t *testing.T) {
		_ = runHandler(t, router, "FUNCTION", [][]byte{
			[]byte("CREATE"),
			[]byte("dellib"),
			[]byte("redis.test = function() return 1 end"),
		}, s)

		_ = runHandler(t, router, "FUNCTION", [][]byte{
			[]byte("DELETE"),
			[]byte("dellib"),
		}, s)
	})

	t.Run("FUNCTION RESTORE", func(t *testing.T) {
		_ = runHandler(t, router, "FUNCTION", [][]byte{
			[]byte("RESTORE"),
			[]byte("dGVzdA=="),
		}, s)
	})
}

func TestLowCoverageBatch17_CborEncoding(t *testing.T) {
	s := store.NewStore()
	router := NewRouter()
	RegisterEncodingCommands(router)

	t.Run("CBOR.ENCODE various data", func(t *testing.T) {
		_ = runHandler(t, router, "CBOR.ENCODE", [][]byte{[]byte("test data")}, s)
		_ = runHandler(t, router, "CBOR.ENCODE", [][]byte{[]byte("123")}, s)
		_ = runHandler(t, router, "CBOR.ENCODE", [][]byte{[]byte("{\"key\":\"value\"}")}, s)
	})

	t.Run("CBOR.DECODE various data", func(t *testing.T) {
		_ = runHandler(t, router, "CBOR.DECODE", [][]byte{[]byte("\x64test")}, s)
		_ = runHandler(t, router, "CBOR.DECODE", [][]byte{[]byte("invalid")}, s)
	})
}

func TestLowCoverageBatch17_SketchMerge(t *testing.T) {
	s := store.NewStore()
	router := NewRouter()
	RegisterMoreCommands(router)

	t.Run("SKETCH.MERGE basic", func(t *testing.T) {
		_ = runHandler(t, router, "SKETCH.CREATE", [][]byte{[]byte("sketch1"), []byte("1000")}, s)
		_ = runHandler(t, router, "SKETCH.CREATE", [][]byte{[]byte("sketch2"), []byte("1000")}, s)
		_ = runHandler(t, router, "SKETCH.UPDATE", [][]byte{[]byte("sketch1"), []byte("item1")}, s)
		_ = runHandler(t, router, "SKETCH.UPDATE", [][]byte{[]byte("sketch2"), []byte("item2")}, s)
		_ = runHandler(t, router, "SKETCH.MERGE", [][]byte{[]byte("sketch1"), []byte("sketch2")}, s)
	})
}

func TestLowCoverageBatch17_CircuitStats(t *testing.T) {
	s := store.NewStore()
	router := NewRouter()
	RegisterSchedulerCommands(router)

	t.Run("CIRCUIT.STATS with circuit", func(t *testing.T) {
		_ = runHandler(t, router, "CIRCUIT.CREATE", [][]byte{[]byte("mycircuit"), []byte("5"), []byte("30"), []byte("60")}, s)
		_ = runHandler(t, router, "CIRCUIT.STATS", [][]byte{[]byte("mycircuit")}, s)
	})
}

func TestLowCoverageBatch17_XAutoClaimMore(t *testing.T) {
	s := store.NewStore()
	router := NewRouter()
	RegisterStreamCommands(router)

	t.Run("XAUTOCLAIM with stream", func(t *testing.T) {
		_ = runHandler(t, router, "XADD", [][]byte{[]byte("mystream"), []byte("*"), []byte("field"), []byte("value")}, s)
		_ = runHandler(t, router, "XGROUP", [][]byte{[]byte("CREATE"), []byte("mystream"), []byte("mygroup"), []byte("$")}, s)
		_ = runHandler(t, router, "XAUTOCLAIM", [][]byte{
			[]byte("mystream"),
			[]byte("mygroup"),
			[]byte("consumer1"),
			[]byte("0"),
			[]byte("10"),
		}, s)
	})
}

func TestLowCoverageBatch17_TemplateCommandsMore(t *testing.T) {
	s := store.NewStore()
	router := NewRouter()
	RegisterTemplateCommands(router)

	t.Run("TEMPLATE.MATCH various patterns", func(t *testing.T) {
		_ = runHandler(t, router, "TEMPLATE.MATCH", [][]byte{[]byte("hello.*"), []byte("hello world")}, s)
		_ = runHandler(t, router, "TEMPLATE.MATCH", [][]byte{[]byte("\\d+"), []byte("12345")}, s)
		_ = runHandler(t, router, "TEMPLATE.MATCH", [][]byte{[]byte("[a-z]+"), []byte("abc")}, s)
	})

	t.Run("TEMPLATE.RENDER with data", func(t *testing.T) {
		_ = runHandler(t, router, "TEMPLATE.RENDER", [][]byte{[]byte("Hello {{.Name}}"), []byte("{\"Name\":\"World\"}")}, s)
		_ = runHandler(t, router, "TEMPLATE.RENDER", [][]byte{[]byte("Count: {{.Count}}"), []byte("{\"Count\":42}")}, s)
	})
}
