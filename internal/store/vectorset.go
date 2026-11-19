package store

import (
	"errors"
	"math"
	"math/rand/v2"
	"sync"
	"sync/atomic"

	"github.com/cachestorm/cachestorm/internal/vectorset"
)

// ErrModuleNoCopy is COPY's error for a value of a module type that, as in
// Redis, cannot be copied.
var ErrModuleNoCopy = errors.New("ERR not supported for this module key")

// MaxVectorDim is the largest vector dimension Redis accepts.
const MaxVectorDim = 1 << 16

// vectorSetUIDs numbers the vector sets of this process, as Redis's
// vset-uid does from 0.
var vectorSetUIDs atomic.Uint64

// VectorElem is the value each graph node carries: the element and its JSON
// attributes, nil when it has none.
type VectorElem struct {
	Name string
	Attr []byte
}

// VectorSetValue is a vector set: an HNSW graph of the elements' vectors,
// the element-to-node map, and the projection matrix of a set made with
// REDUCE. Readers hold mu for reading, so searches run concurrently;
// changes hold it for writing.
type VectorSetValue struct {
	mu         sync.RWMutex
	Index      *vectorset.Index
	Elems      map[string]*vectorset.Node
	Proj       []float32
	ProjInput  uint32
	NumAttribs uint64
	UID        uint64
}

// NewVectorSet creates an empty vector set of vectors of dim components.
func NewVectorSet(dim uint32, quant vectorset.Quant, m uint32) *VectorSetValue {
	return &VectorSetValue{
		Index: vectorset.New(dim, quant, m),
		Elems: map[string]*vectorset.Node{},
		UID:   vectorSetUIDs.Add(1) - 1,
	}
}

func (v *VectorSetValue) Lock()    { v.mu.Lock() }
func (v *VectorSetValue) Unlock()  { v.mu.Unlock() }
func (v *VectorSetValue) RLock()   { v.mu.RLock() }
func (v *VectorSetValue) RUnlock() { v.mu.RUnlock() }

func (v *VectorSetValue) Type() DataType { return DataTypeVectorSet }
func (v *VectorSetValue) String() string { return "vectorset" }

// NoCopy marks a value COPY refuses, as Redis does for a module type with
// no copy callback.
func (v *VectorSetValue) NoCopy() {}

func (v *VectorSetValue) SizeOf() int64 {
	v.mu.RLock()
	defer v.mu.RUnlock()
	size := int64(96) + v.Index.MemUsage() + int64(len(v.Proj))*4
	for name, n := range v.Elems {
		size += int64(len(name))*2 + 64
		if e, ok := n.Value.(*VectorElem); ok {
			size += int64(len(e.Attr))
		}
	}
	return size
}

// Clone copies the set through its serialized form, keeping the graph.
func (v *VectorSetValue) Clone() Value {
	v.mu.RLock()
	defer v.mu.RUnlock()
	c := &VectorSetValue{
		Index:      vectorset.New(v.Index.Dim(), v.Index.Quant(), v.Index.M()),
		Elems:      make(map[string]*vectorset.Node, len(v.Elems)),
		Proj:       append([]float32(nil), v.Proj...),
		ProjInput:  v.ProjInput,
		NumAttribs: v.NumAttribs,
		UID:        vectorSetUIDs.Add(1) - 1,
	}
	for n := v.Index.Head(); n != nil; n = n.Next() {
		vec, params := v.Index.SerializeNode(n)
		e := n.Value.(*VectorElem)
		cn, err := c.Index.InsertSerialized(vec, params, &VectorElem{Name: e.Name, Attr: e.Attr})
		if err != nil {
			panic("vectorset: cloning a valid graph failed: " + err.Error())
		}
		c.Elems[e.Name] = cn
	}
	if err := c.Index.Deserialize(rand.Uint64(), rand.Uint64()); err != nil {
		panic("vectorset: cloning a valid graph failed: " + err.Error())
	}
	return c
}

// Project applies the set's projection matrix to an input vector, as
// Redis's applyProjection does.
func (v *VectorSetValue) Project(in []float32) []float32 {
	out := make([]float32, v.Index.Dim())
	in32 := int(v.ProjInput)
	for i := range out {
		row := v.Proj[i*in32 : (i+1)*in32]
		var sum float32
		for j, w := range row {
			sum += float32(w * in[j])
		}
		out[i] = sum
	}
	return out
}

// ProjectionMatrix is Redis's createProjectionMatrix: a Hadamard-like
// pattern of +-1 scaled by 1/sqrt(input), deterministic so replicas agree.
func ProjectionMatrix(input, output uint32) []float32 {
	m := make([]float32, int(input)*int(output))
	scale := float32(1.0 / math.Sqrt(float64(input)))
	for i := uint32(0); i < output; i++ {
		for j := uint32(0); j < input; j++ {
			value := float32(1)
			if popcount32(i&j)%2 == 1 {
				value = -1
			}
			m[i*input+j] = value * scale
		}
	}
	return m
}

func popcount32(n uint32) int {
	c := 0
	for n != 0 {
		c += int(n & 1)
		n >>= 1
	}
	return c
}

// Insert adds the element with vector vec (already projected), or, if it
// exists, reinserts it with the new vector, as Redis's vectorSetInsert
// does. attr nil leaves an existing element's attributes as they are; an
// empty attr removes them. It reports whether the element is new.
func (v *VectorSetValue) Insert(name string, vec []float32, attr []byte, ef int) bool {
	if n, ok := v.Elems[name]; ok {
		e := n.Value.(*VectorElem)
		v.Index.Delete(n)
		v.Elems[name] = v.Index.Insert(vec, e, ef)
		if attr != nil {
			v.setAttr(e, attr)
		}
		return false
	}
	// A new element keeps even an empty SETATTR, as Redis does.
	e := &VectorElem{Name: name}
	if attr != nil {
		e.Attr = append([]byte{}, attr...)
		v.NumAttribs++
	}
	v.Elems[name] = v.Index.Insert(vec, e, ef)
	return true
}

// SetAttr sets an element's attributes, removing them when attr is empty.
func (v *VectorSetValue) SetAttr(name string, attr []byte) bool {
	n, ok := v.Elems[name]
	if !ok {
		return false
	}
	v.setAttr(n.Value.(*VectorElem), attr)
	return true
}

func (v *VectorSetValue) setAttr(e *VectorElem, attr []byte) {
	if e.Attr != nil {
		v.NumAttribs--
	}
	e.Attr = nil
	if len(attr) > 0 {
		e.Attr = append([]byte(nil), attr...)
		v.NumAttribs++
	}
}

// Remove deletes an element and reports whether it was there.
func (v *VectorSetValue) Remove(name string) bool {
	n, ok := v.Elems[name]
	if !ok {
		return false
	}
	if n.Value.(*VectorElem).Attr != nil {
		v.NumAttribs--
	}
	delete(v.Elems, name)
	v.Index.Delete(n)
	return true
}

// Len is the number of elements.
func (v *VectorSetValue) Len() int {
	v.mu.RLock()
	defer v.mu.RUnlock()
	return len(v.Elems)
}
