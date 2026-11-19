package vectorset

import (
	"bytes"
	"compress/gzip"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"io"
	"math"
	"os"
	"strconv"
	"testing"
)

// The golden data was recorded from Redis 8.2.11: every VADD was followed by
// VLINKS to learn the random level Redis picked, so replaying the same
// operations with the same levels must rebuild exactly the same graph.

type hnswGoldenOp struct {
	Op    string   `json:"op"`
	Ele   string   `json:"ele"`
	Vec   []uint32 `json:"vec,omitempty"`
	Level int      `json:"level"`
	G     int      `json:"g"`
}

type hnswGoldenQuery struct {
	Vec       []uint32   `json:"vec"`
	Count     int        `json:"count"`
	EF        int        `json:"ef"`
	Truth     bool       `json:"truth"`
	FilterG   int        `json:"filter_g"`
	FilterEF  int        `json:"filter_ef"`
	HasFilter bool       `json:"has_filter"`
	Res       [][]string `json:"res"`
}

type hnswGoldenSerNode struct {
	Ele    string   `json:"ele"`
	Attr   string   `json:"attr"`
	Vec    string   `json:"vec"`
	Params []uint64 `json:"params"`
}

type hnswGoldenCase struct {
	Key           string                `json:"key"`
	Quant         string                `json:"quant"`
	Dim           int                   `json:"dim"`
	M             int                   `json:"m"`
	EF            int                   `json:"ef"`
	Ops           []hnswGoldenOp        `json:"ops"`
	Links         map[string][][]string `json:"links"`
	Queries       []hnswGoldenQuery     `json:"queries"`
	Dump          []hnswGoldenSerNode   `json:"dump"`
	DumpHeader    []uint64              `json:"dump_header"`
	RestoredLinks map[string][][]string `json:"restored_links"`
	RestoredQ     []hnswGoldenQuery     `json:"restored_queries"`
	Init          []hnswGoldenSerNode   `json:"init"`
}

type hnswGolden struct {
	Cases []hnswGoldenCase `json:"cases"`
}

type hnswTestVal struct {
	ele string
	g   int
}

func hnswQuantByName(s string) Quant {
	switch s {
	case "NOQUANT":
		return QuantNone
	case "BIN":
		return QuantBin
	}
	return QuantQ8
}

func hnswFloats(b []uint32) []float32 {
	v := make([]float32, len(b))
	for i, x := range b {
		v[i] = math.Float32frombits(x)
	}
	return v
}

func hnswLinkNames(n *Node) [][]string {
	out := make([][]string, 0, n.Level()+1)
	for l := int(n.Level()); l >= 0; l-- {
		names := []string{}
		for _, nb := range n.Links(uint32(l)) {
			names = append(names, nb.Value.(*hnswTestVal).ele)
		}
		out = append(out, names)
	}
	return out
}

func hnswCheckLinks(t *testing.T, label string, nodes map[string]*Node, want map[string][][]string) {
	t.Helper()
	if len(nodes) != len(want) {
		t.Fatalf("%s: %d nodes, want %d", label, len(nodes), len(want))
	}
	bad := 0
	for ele, wl := range want {
		n := nodes[ele]
		if n == nil {
			t.Fatalf("%s: missing %s", label, ele)
		}
		got := hnswLinkNames(n)
		gj, _ := json.Marshal(got)
		wj, _ := json.Marshal(wl)
		if !bytes.Equal(gj, wj) {
			bad++
			if bad <= 3 {
				t.Errorf("%s: links of %s\n got  %s\n want %s", label, ele, gj, wj)
			}
		}
	}
	if bad > 0 {
		t.Fatalf("%s: %d nodes with different links", label, bad)
	}
}

// hnswRunVSIM mirrors VSIM_execute's use of the library.
func hnswRunVSIM(ix *Index, q *hnswGoldenQuery) ([]*Node, []float32) {
	vec := hnswFloats(q.Vec)
	ef := q.EF
	if ef == 0 {
		ef = 100
	}
	if q.Count > ef {
		ef = q.Count
	}
	if uint64(ef) > ix.Len() {
		ef = int(ix.Len())
	}
	var filter func(any) bool
	if q.HasFilter {
		g := q.FilterG
		filter = func(v any) bool { return v.(*hnswTestVal).g == g }
	}
	var nodes []*Node
	var dists []float32
	switch {
	case q.Truth:
		nodes, dists = ix.GroundTruth(vec, ef, filter)
	case filter == nil:
		nodes, dists = ix.Search(vec, ef, nil, 0)
	default:
		fef := q.FilterEF
		if fef == 0 {
			fef = q.Count * 100
		}
		nodes, dists = ix.Search(vec, ef, filter, fef)
	}
	if len(nodes) > q.Count {
		nodes, dists = nodes[:q.Count], dists[:q.Count]
	}
	return nodes, dists
}

func hnswCheckQueries(t *testing.T, label string, ix *Index, qs []hnswGoldenQuery) {
	t.Helper()
	for qi := range qs {
		q := &qs[qi]
		nodes, dists := hnswRunVSIM(ix, q)
		if len(nodes) != len(q.Res) {
			t.Fatalf("%s query %d: %d results, want %d", label, qi, len(nodes), len(q.Res))
		}
		for i, n := range nodes {
			ele := n.Value.(*hnswTestVal).ele
			want, err := strconv.ParseFloat(q.Res[i][1], 64)
			if err != nil {
				t.Fatal(err)
			}
			score := 1.0 - float64(dists[i])/2.0
			if ele != q.Res[i][0] || score != want {
				t.Fatalf("%s query %d (truth=%v filter=%v ef=%d count=%d) result %d: got %s %v want %s %s",
					label, qi, q.Truth, q.HasFilter, q.EF, q.Count, i, ele, score, q.Res[i][0], q.Res[i][1])
			}
		}
	}
}

func hnswReplayGolden(t *testing.T, c *hnswGoldenCase) {
	quant := hnswQuantByName(c.Quant)
	ix := New(uint32(c.Dim), quant, uint32(c.M))
	nodes := map[string]*Node{}
	if len(c.Init) > 0 {
		ix, nodes = hnswLoadSerialized(t, c.Key, uint32(c.Dim), quant, uint32(c.M), c.Init)
	}
	var nextLevel uint32
	ix.levelHook = func() uint32 { return nextLevel }
	for _, op := range c.Ops {
		switch op.Op {
		case "add":
			if old := nodes[op.Ele]; old != nil {
				ix.Delete(old)
			}
			nextLevel = uint32(op.Level)
			nodes[op.Ele] = ix.Insert(hnswFloats(op.Vec), &hnswTestVal{op.Ele, op.G}, c.EF)
		case "rem":
			ix.Delete(nodes[op.Ele])
			delete(nodes, op.Ele)
		}
	}
	hnswCheckLinks(t, c.Key+" built", nodes, c.Links)
	if conn, rec := ix.ValidateGraph(); !rec || conn == 0 {
		t.Fatalf("%s: invalid graph connected=%d reciprocal=%v", c.Key, conn, rec)
	}
	hnswCheckQueries(t, c.Key+" built", ix, c.Queries)

	if len(c.Dump) == 0 {
		return
	}
	cfg := uint64(ix.Quant()) | uint64(ix.M()&0xffff)<<8
	if c.DumpHeader[0] != uint64(ix.Dim()) || c.DumpHeader[1] != ix.Len() || c.DumpHeader[2] != cfg {
		t.Fatalf("%s: header %v, have dim=%d len=%d cfg=%d", c.Key, c.DumpHeader, ix.Dim(), ix.Len(), cfg)
	}
	i := 0
	for n := ix.Head(); n != nil; n = n.Next() {
		want := c.Dump[i]
		i++
		vec, params := ix.SerializeNode(n)
		if ele := n.Value.(*hnswTestVal).ele; ele != want.Ele {
			t.Fatalf("%s: serialization order: %s, want %s", c.Key, ele, want.Ele)
		}
		if hex.EncodeToString(vec) != want.Vec {
			t.Fatalf("%s: vector of %s: %x want %s", c.Key, want.Ele, vec, want.Vec)
		}
		if len(params) != len(want.Params) {
			t.Fatalf("%s: params of %s: %v want %v", c.Key, want.Ele, params, want.Params)
		}
		for j := range params {
			g, w := params[j], want.Params[j]
			if j == len(params)-1 && quant != QuantQ8 {
				// Redis leaves the range uninitialized for these.
				g, w = uint64(uint32(g)), uint64(uint32(w))
			}
			if g != w {
				t.Fatalf("%s: params of %s differ at %d: %v want %v", c.Key, want.Ele, j, params, want.Params)
			}
		}
	}

	lx, loaded := hnswLoadSerialized(t, c.Key, uint32(c.DumpHeader[0]), Quant(c.DumpHeader[2]&0xff), uint32(c.DumpHeader[2]>>8&0xffff), c.Dump)
	if c.RestoredLinks == nil {
		return
	}
	hnswCheckLinks(t, c.Key+" restored", loaded, c.RestoredLinks)
	hnswCheckQueries(t, c.Key+" restored", lx, c.RestoredQ)
}

func hnswLoadSerialized(t *testing.T, key string, dim uint32, quant Quant, m uint32, sns []hnswGoldenSerNode) (*Index, map[string]*Node) {
	t.Helper()
	ix := New(dim, quant, m)
	nodes := map[string]*Node{}
	for _, sn := range sns {
		vec, err := hex.DecodeString(sn.Vec)
		if err != nil {
			t.Fatal(err)
		}
		g := -1
		if sn.Attr != "" {
			g = int(sn.Attr[len(sn.Attr)-2] - '0')
		}
		n, err := ix.InsertSerialized(vec, sn.Params, &hnswTestVal{sn.Ele, g})
		if err != nil {
			t.Fatalf("%s: InsertSerialized(%s): %v", key, sn.Ele, err)
		}
		nodes[sn.Ele] = n
	}
	if err := ix.Deserialize(0x1234, 0x5678); err != nil {
		t.Fatalf("%s: Deserialize: %v", key, err)
	}
	return ix, nodes
}

func hnswLoadGolden(t *testing.T, r io.Reader) *hnswGolden {
	t.Helper()
	var g hnswGolden
	if err := json.NewDecoder(r).Decode(&g); err != nil {
		t.Fatal(err)
	}
	return &g
}

func TestHNSWRedisGolden(t *testing.T) {
	var src io.Reader
	if p := os.Getenv("HNSW_GOLDEN"); p != "" {
		f, err := os.Open(p)
		if err != nil {
			t.Fatal(err)
		}
		defer f.Close()
		src = f
	} else {
		if hnswGoldenData == "" {
			t.Skip("no golden data")
		}
		zr, err := gzip.NewReader(base64.NewDecoder(base64.StdEncoding, bytes.NewReader([]byte(hnswGoldenData))))
		if err != nil {
			t.Fatal(err)
		}
		src = zr
	}
	g := hnswLoadGolden(t, src)
	for i := range g.Cases {
		c := &g.Cases[i]
		t.Run(c.Key, func(t *testing.T) { hnswReplayGolden(t, c) })
	}
}
