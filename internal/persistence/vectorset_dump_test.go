package persistence

import (
	"bytes"
	"math"
	"testing"

	"github.com/cachestorm/cachestorm/internal/store"
	"github.com/cachestorm/cachestorm/internal/vectorset"
)

// Redis 8.2's DUMP of a Q8 vector set: VADD vg VALUES 3 1 2 3 a SETATTR
// '{"x":1}', VADD vg VALUES 3 -1 0.5 2 b.
const redisVectorSetDump = `0781bde72da2bb1eb40002030202025001020205016205000503c1207f0207020202800100000002010220020102813e9807dc0000000002813f5f74824012a47605016105077b2278223a317d05032a557f0207020102800100000002010220020202813e9807dc0000000002813f4d41b2406f7751000c006779c8e7b7ae86aa`

func TestRestoreRedisVectorSet(t *testing.T) {
	payload := hexPayload(t, redisVectorSetDump)
	v, err := DecodeDump(payload)
	if err != nil {
		t.Fatal(err)
	}
	vs := v.(*store.VectorSetValue)
	if vs.Index.Dim() != 3 || vs.Index.Quant() != vectorset.QuantQ8 || vs.Index.M() != 16 || len(vs.Elems) != 2 || vs.NumAttribs != 1 {
		t.Fatalf("dim %d quant %d M %d elems %d attribs %d", vs.Index.Dim(), vs.Index.Quant(), vs.Index.M(), len(vs.Elems), vs.NumAttribs)
	}
	a := vs.Elems["a"]
	// VEMB vg a RAW on Redis: int8, "*U\x7f", 3.7416574954986572, 0.8017836809158325.
	if !bytes.Equal(a.Raw(), []byte("*U\x7f")) || a.L2() != float32(3.7416574954986572) || a.QRange() != float32(0.8017836809158325) {
		t.Fatalf("a: raw %q l2 %v range %v", a.Raw(), a.L2(), a.QRange())
	}
	if got := string(a.Value.(*store.VectorElem).Attr); got != `{"x":1}` {
		t.Fatalf("a's attributes %q", got)
	}
	// VEMB vg a: 0.992125928401947 2.007874011993408 3.
	want := []float64{0.992125928401947, 2.007874011993408, 3}
	for i, f := range vs.Index.NodeVector(a) {
		if float64(f) != want[i] {
			t.Fatalf("VEMB a[%d] = %v, Redis gives %v", i, f, want[i])
		}
	}
	nodes, dists := vs.Index.GroundTruth(vs.Index.NodeVector(vs.Elems["b"]), 2, nil)
	if len(nodes) != 2 || nodes[0] != vs.Elems["b"] || 1-float64(dists[0])/2 != 0.9994409680366516 || 1-float64(dists[1])/2 != 0.8519260883331299 {
		t.Fatalf("VSIM ELE b TRUTH: %v %v", nodes, dists)
	}

	again, err := EncodeDump(vs)
	if err != nil {
		t.Fatal(err)
	}
	back, err := DecodeDump(again)
	if err != nil {
		t.Fatal(err)
	}
	if b := back.(*store.VectorSetValue); len(b.Elems) != 2 || !bytes.Equal(b.Elems["b"].Raw(), vs.Elems["b"].Raw()) {
		t.Fatal("round trip lost elements")
	}
	for n := 1; n < len(payload)-10; n++ {
		cut := resealDump(append(append([]byte(nil), payload[:n]...), payload[len(payload)-10:]...))
		if _, err := DecodeDump(cut); err == nil {
			t.Fatalf("a payload cut at %d restored", n)
		}
	}
}

func TestVectorSetDumpRoundTrip(t *testing.T) {
	for _, quant := range []vectorset.Quant{vectorset.QuantNone, vectorset.QuantQ8, vectorset.QuantBin} {
		vs := store.NewVectorSet(4, quant, 8)
		vs.Proj = store.ProjectionMatrix(6, 4)
		vs.ProjInput = 6
		for i := 0; i < 60; i++ {
			in := []float32{float32(i), float32(i % 7), -1, float32(math.Sin(float64(i))), 2, float32(i % 3)}
			var attr []byte
			if i%4 == 0 {
				attr = []byte(`{"i":1}`)
			}
			vs.Insert(string(rune('A'+i)), vs.Project(in), attr, 50)
		}
		vs.Remove("B")
		payload, err := EncodeDump(vs)
		if err != nil {
			t.Fatal(err)
		}
		v, err := DecodeDump(payload)
		if err != nil {
			t.Fatal(err)
		}
		got := v.(*store.VectorSetValue)
		if len(got.Elems) != 59 || got.NumAttribs != vs.NumAttribs || got.ProjInput != 6 || len(got.Proj) != 24 || got.Index.M() != 8 {
			t.Fatalf("quant %d: %d elems, %d attribs", quant, len(got.Elems), got.NumAttribs)
		}
		for name, n := range vs.Elems {
			if !bytes.Equal(got.Elems[name].Raw(), n.Raw()) || len(got.Elems[name].Links(0)) != len(n.Links(0)) {
				t.Fatalf("quant %d: element %s changed", quant, name)
			}
		}
		if _, ok := got.Index.ValidateGraph(); !ok {
			t.Fatalf("quant %d: loaded graph is not reciprocal", quant)
		}
	}
}
