package vectorset

import (
	"encoding/binary"
	"math"
	"math/rand/v2"
	"sync"
)

// Quant is the vector quantization of an index, with Redis's HNSW_QUANT_*
// numbering (it is part of the RDB encoding).
type Quant uint8

const (
	QuantNone Quant = 0
	QuantQ8   Quant = 1
	QuantBin  Quant = 2
)

const (
	DefaultM = 16
	MinM     = 4
	MaxM     = 4096

	// MaxLevel is HNSW_MAX_LEVEL: node levels are 0..MaxLevel-1.
	MaxLevel = 16
	// DefaultEF is HNSW_EF_C, the exploration factor used to relink nodes
	// after deletions.
	DefaultEF = 200

	hnswMaxCandidates    = 256
	hnswSerVersion       = 1
	hnswWorstLinkMissing = math.MaxUint32
	// threshold = HNSW_P * RAND_MAX with glibc's RAND_MAX = 2^31-1.
	hnswLevelThreshold = 536870911
)

type hnswLayer struct {
	links     []*Node
	maxLinks  uint32
	worstIdx  uint32
	worstDist float32
}

// Node is an element of the graph. Value belongs to the caller.
type Node struct {
	Value any

	id     uint64
	level  uint32
	slot   uint32
	l2     float32
	qrange float32
	fvec   []float32
	qvec   []int8
	bvec   []uint64
	prev   *Node
	next   *Node
	layers []hnswLayer
}

func (n *Node) ID() uint64      { return n.id }
func (n *Node) Level() uint32   { return n.level }
func (n *Node) L2() float32     { return n.l2 }
func (n *Node) QRange() float32 { return n.qrange }
func (n *Node) Next() *Node     { return n.next }

// Links returns the neighbors at the given layer, in Redis's link order.
// The slice is owned by the index and must not be modified.
func (n *Node) Links(layer uint32) []*Node {
	if layer > n.level {
		return nil
	}
	return n.layers[layer].links
}

// Raw returns the stored vector bytes: little-endian float32 components,
// int8 quants, or little-endian 64-bit words of sign bits.
func (n *Node) Raw() []byte {
	switch {
	case n.qvec != nil:
		b := make([]byte, len(n.qvec))
		for i, v := range n.qvec {
			b[i] = byte(v)
		}
		return b
	case n.bvec != nil:
		b := make([]byte, len(n.bvec)*8)
		for i, v := range n.bvec {
			binary.LittleEndian.PutUint64(b[i*8:], v)
		}
		return b
	default:
		b := make([]byte, len(n.fvec)*4)
		for i, v := range n.fvec {
			binary.LittleEndian.PutUint32(b[i*4:], math.Float32bits(v))
		}
		return b
	}
}

// Index is a port of Redis 8.2's HNSW (modules/vector-sets/hnsw.c). It is
// not internally locked: Search, GroundTruth, Distance, NodeVector,
// RandomNode, Links and the accessors may run concurrently with each other,
// while Insert, Delete, InsertSerialized and Deserialize need exclusive
// access.
type Index struct {
	ep       *Node
	head     *Node
	m        uint32
	maxLevel uint32
	dim      uint32
	quant    Quant
	count    uint64
	lastID   uint64

	rng       *rand.Rand
	levelHook func() uint32

	nextSlot  uint32
	freeSlots []uint32

	pending map[*Node][][]uint64

	insQueues [MaxLevel][]hnswPQItem
}

// New creates an index. m == 0 selects DefaultM and m is capped at MaxM, as
// hnsw_new does; callers enforce MinM themselves.
func New(dim uint32, quant Quant, m uint32) *Index {
	if m == 0 {
		m = DefaultM
	} else if m > MaxM {
		m = MaxM
	}
	return &Index{
		m:     m,
		dim:   dim,
		quant: quant,
		rng:   rand.New(rand.NewPCG(rand.Uint64(), rand.Uint64())),
	}
}

func (ix *Index) Dim() uint32      { return ix.dim }
func (ix *Index) M() uint32        { return ix.m }
func (ix *Index) Quant() Quant     { return ix.quant }
func (ix *Index) Len() uint64      { return ix.count }
func (ix *Index) MaxLevel() uint32 { return ix.maxLevel }
func (ix *Index) LastID() uint64   { return ix.lastID }
func (ix *Index) Head() *Node      { return ix.head }
func (ix *Index) EntryPoint() *Node {
	return ix.ep
}

// SeedLevels makes the random level choice of later inserts deterministic.
func (ix *Index) SeedLevels(s1, s2 uint64) {
	ix.rng = rand.New(rand.NewPCG(s1, s2))
}

// QuantBytes is hnsw_quants_bytes.
func (ix *Index) QuantBytes() uint32 {
	switch ix.quant {
	case QuantQ8:
		return ix.dim
	case QuantBin:
		return (ix.dim + 63) / 64 * 8
	default:
		return ix.dim * 4
	}
}

// MemUsage estimates the bytes used by the index, in the spirit of
// VectorSetMemUsage.
func (ix *Index) MemUsage() int64 {
	const nodeStruct = 160
	const layerStruct = 40
	n := int64(ix.count)
	if n == 0 {
		return 96
	}
	size := int64(96) + n*nodeStruct + n*int64(ix.QuantBytes())
	size += n * layerStruct * 4 / 3
	l0 := int64(ix.m) * 2
	size += 8 * l0 * n
	size += 8 * (l0 / 2) * n / 3
	return size
}

func (ix *Index) distance(a, b *Node) float32 {
	switch ix.quant {
	case QuantQ8:
		return hnswDistanceQ8(a.qvec, b.qvec, a.qrange, b.qrange)
	case QuantBin:
		return hnswDistanceBin(a.bvec, b.bvec, ix.dim)
	default:
		return hnswDistanceFloat(a.fvec, b.fvec)
	}
}

// Distance is hnsw_distance: 1 - cosine similarity, in [0, 2].
func (ix *Index) Distance(a, b *Node) float32 { return ix.distance(a, b) }

// NodeVector is hnsw_get_node_vector: the de-quantized, de-normalized
// approximation of the inserted vector.
func (ix *Index) NodeVector(n *Node) []float32 {
	vec := make([]float32, ix.dim)
	switch ix.quant {
	case QuantNone:
		copy(vec, n.fvec)
	case QuantQ8:
		for j, q := range n.qvec {
			vec[j] = float32(float32(q)*n.qrange) / 127
		}
	case QuantBin:
		for j := range vec {
			if n.bvec[j/64]&(1<<uint(j&63)) != 0 {
				vec[j] = 1
			} else {
				vec[j] = -1
			}
		}
		return vec
	}
	for j := range vec {
		vec[j] *= n.l2
	}
	return vec
}

// setVector stores vec (already normalized or not, per Redis's caller)
// quantizing it if the index is quantized.
func (ix *Index) setVector(n *Node, vec []float32) {
	switch ix.quant {
	case QuantQ8:
		n.qvec = make([]int8, ix.dim)
		n.qrange = hnswQuantizeQ8(vec, n.qvec)
	case QuantBin:
		n.bvec = make([]uint64, (ix.dim+63)/64)
		hnswQuantizeBin(vec, n.bvec)
	default:
		n.fvec = vec
	}
}

// queryNode is hnsw_init_tmp_node with a not normalized query.
func (ix *Index) queryNode(query []float32) *Node {
	v := make([]float32, ix.dim)
	copy(v, query)
	hnswNormalize(v)
	q := &Node{}
	ix.setVector(q, v)
	return q
}

func (ix *Index) newNode(id uint64, level uint32) *Node {
	if id == 0 {
		ix.lastID++
		id = ix.lastID
	}
	n := &Node{id: id, level: level, l2: 1, layers: make([]hnswLayer, level+1)}
	for i := range n.layers {
		ml := ix.m
		if i == 0 {
			ml = ix.m * 2
		}
		n.layers[i].maxLinks = ml
		n.layers[i].links = make([]*Node, 0, ml)
	}
	return n
}

func (ix *Index) addNode(n *Node) {
	if k := len(ix.freeSlots); k > 0 {
		n.slot = ix.freeSlots[k-1]
		ix.freeSlots = ix.freeSlots[:k-1]
	} else {
		n.slot = ix.nextSlot
		ix.nextSlot++
	}
	n.next = ix.head
	n.prev = nil
	if ix.head != nil {
		ix.head.prev = n
	}
	ix.head = n
	ix.count++
}

func (ix *Index) randomLevel() uint32 {
	if ix.levelHook != nil {
		return ix.levelHook()
	}
	level := uint32(0)
	for ix.rng.Int32() < hnswLevelThreshold && level < MaxLevel-1 {
		level++
	}
	return level
}

/* ============================ Priority queue ============================ */

type hnswPQItem struct {
	node *Node
	dist float32
}

// hnswPQ keeps the best (smallest distance) item at the end of the array,
// exactly like Redis's pqueue, including how ties are placed.
type hnswPQ struct {
	items []hnswPQItem
	count uint32
}

func (pq *hnswPQ) push(n *Node, d float32) {
	items := pq.items
	capa := uint32(len(items))
	if capa == 0 {
		return
	}
	if pq.count < capa {
		i := pq.count
		for i > 0 && items[i-1].dist < d {
			items[i] = items[i-1]
			i--
		}
		items[i] = hnswPQItem{n, d}
		pq.count++
		return
	}
	if d >= items[0].dist {
		return
	}
	i := uint32(0)
	for i < capa-1 && items[i+1].dist > d {
		items[i] = items[i+1]
		i++
	}
	items[i] = hnswPQItem{n, d}
}

func (pq *hnswPQ) pop() (*Node, float32) {
	pq.count--
	it := pq.items[pq.count]
	return it.node, it.dist
}

func (pq *hnswPQ) maxDist() float32 {
	if pq.count == 0 {
		return float32(math.Inf(1))
	}
	return pq.items[0].dist
}

func (pq *hnswPQ) node(i uint32) *Node   { return pq.items[pq.count-i-1].node }
func (pq *hnswPQ) dist(i uint32) float32 { return pq.items[pq.count-i-1].dist }
func (pq *hnswPQ) capacity() uint32      { return uint32(len(pq.items)) }

/* ============================ Visited tracking ==========================
 * Redis keeps a visited epoch per node per thread slot. Here each search
 * borrows a private scratch area from a pool: an epoch array indexed by the
 * node's dense slot number, so concurrent readers never share mutable
 * state and nodes carry no per-thread data. */

type hnswScratch struct {
	marks []uint32
	epoch uint32
	cand  []hnswPQItem
	res   []hnswPQItem
}

var hnswScratchPool = sync.Pool{New: func() any {
	return &hnswScratch{cand: make([]hnswPQItem, hnswMaxCandidates)}
}}

func getHNSWScratch() *hnswScratch { return hnswScratchPool.Get().(*hnswScratch) }

func putHNSWScratch(sc *hnswScratch) {
	clear(sc.cand)
	clear(sc.res)
	hnswScratchPool.Put(sc)
}

func (sc *hnswScratch) begin(slots uint32) uint32 {
	if uint32(len(sc.marks)) < slots {
		n := uint32(len(sc.marks)) * 2
		if n < slots {
			n = slots
		}
		sc.marks = make([]uint32, n)
	}
	sc.epoch++
	if sc.epoch == 0 {
		clear(sc.marks)
		sc.epoch = 1
	}
	return sc.epoch
}

/* ============================ Graph search ============================== */

// searchLayer is search_layer_with_filter. Results are accumulated into res,
// whose capacity is the ef of the search.
func (ix *Index) searchLayer(sc *hnswScratch, query, ep *Node, res *hnswPQ, layer uint32, filter func(any) bool, maxCandidates uint32) {
	epoch := sc.begin(ix.nextSlot)
	marks := sc.marks
	cand := hnswPQ{items: sc.cand[:hnswMaxCandidates]}
	ef := res.capacity()
	evaluated := uint32(1)

	d := ix.distance(query, ep)
	cand.push(ep, d)
	if filter == nil || filter(ep.Value) {
		res.push(ep, d)
	}
	marks[ep.slot] = epoch

	for cand.count > 0 {
		if filter != nil && maxCandidates != 0 && evaluated >= maxCandidates {
			break
		}
		cur, curDist := cand.pop()
		evaluated++
		if res.count >= ef && curDist > res.maxDist() {
			break
		}
		if layer > cur.level {
			continue
		}
		for _, nb := range cur.layers[layer].links {
			if marks[nb.slot] == epoch {
				continue
			}
			marks[nb.slot] = epoch
			nd := ix.distance(query, nb)
			furthest := res.maxDist()
			if filter == nil {
				if nd < furthest || res.count < ef {
					cand.push(nb, nd)
					res.push(nb, nd)
				}
			} else {
				if nd < furthest || cand.count < ef {
					cand.push(nb, nd)
				}
				if filter(nb.Value) {
					if nd < furthest || res.count < ef {
						res.push(nb, nd)
					}
				}
			}
		}
	}
}

// greedyDescend runs the ef=1 searches from the top layer down to (but not
// including) stopLayer, returning the entry point for stopLayer.
func (ix *Index) greedyDescend(sc *hnswScratch, query, ep *Node, from, stopLayer uint32) *Node {
	var one [1]hnswPQItem
	for lc := from; lc > stopLayer; lc-- {
		res := hnswPQ{items: one[:]}
		ix.searchLayer(sc, query, ep, &res, lc, nil, 0)
		if res.count > 0 {
			ep = res.node(0)
		}
	}
	return ep
}

func (ix *Index) clampK(k int) uint32 {
	if uint64(k) > ix.count {
		return uint32(ix.count)
	}
	return uint32(k)
}

// Search is hnsw_search_with_filter with a not normalized query (filter nil
// is plain hnsw_search). maxCandidates bounds the effort of filtered
// searches; 0 means unbounded.
func (ix *Index) Search(query []float32, k int, filter func(value any) bool, maxCandidates int) ([]*Node, []float32) {
	if k <= 0 || ix.ep == nil || len(query) != int(ix.dim) {
		return nil, nil
	}
	if maxCandidates < 0 {
		maxCandidates = 0
	}
	q := ix.queryNode(query)
	sc := getHNSWScratch()
	defer putHNSWScratch(sc)

	ep := ix.greedyDescend(sc, q, ix.ep, ix.maxLevel, 0)
	kk := ix.clampK(k)
	if uint32(cap(sc.res)) < kk {
		sc.res = make([]hnswPQItem, kk)
	}
	res := hnswPQ{items: sc.res[:kk]}
	mc := uint32(maxCandidates)
	if uint64(maxCandidates) > math.MaxUint32 {
		mc = math.MaxUint32
	}
	ix.searchLayer(sc, q, ep, &res, 0, filter, mc)
	return copyHNSWResults(&res)
}

func copyHNSWResults(res *hnswPQ) ([]*Node, []float32) {
	nodes := make([]*Node, res.count)
	dists := make([]float32, res.count)
	for i := uint32(0); i < res.count; i++ {
		nodes[i] = res.node(i)
		dists[i] = res.dist(i)
	}
	return nodes, dists
}

// GroundTruth is hnsw_ground_truth_with_filter: an exact linear scan in
// list order.
func (ix *Index) GroundTruth(query []float32, k int, filter func(value any) bool) ([]*Node, []float32) {
	if k <= 0 || len(query) != int(ix.dim) {
		return nil, nil
	}
	q := ix.queryNode(query)
	res := hnswPQ{items: make([]hnswPQItem, ix.clampK(k))}
	for cur := ix.head; cur != nil; cur = cur.next {
		if filter != nil && !filter(cur.Value) {
			continue
		}
		res.push(cur, ix.distance(q, cur))
	}
	return copyHNSWResults(&res)
}

/* ============================ Linking =================================== */

func (ix *Index) updateWorst(n *Node, layer uint32) {
	l := &n.layers[layer]
	var worstDist float32
	worstIdx := uint32(0)
	for i, nb := range l.links {
		if d := ix.distance(n, nb); d > worstDist {
			worstDist = d
			worstIdx = uint32(i)
		}
	}
	l.worstDist = worstDist
	l.worstIdx = worstIdx
}

func updateWorstOnAdd(n *Node, layer, added uint32, d float32) {
	l := &n.layers[layer]
	if len(l.links) == 1 || d > l.worstDist {
		l.worstDist = d
		l.worstIdx = added
	}
}

func (ix *Index) updateWorstOnRemove(n *Node, layer, removed uint32) {
	l := &n.layers[layer]
	switch {
	case len(l.links) == 0:
		l.worstDist = 0
		l.worstIdx = 0
	case removed == l.worstIdx:
		ix.updateWorst(n, layer)
	case removed < l.worstIdx:
		l.worstIdx--
	}
}

// removeLinkAt deletes links[j] shifting the rest left (memmove in Redis).
func removeLinkAt(l *hnswLayer, j int) {
	n := len(l.links)
	copy(l.links[j:], l.links[j+1:])
	l.links[n-1] = nil
	l.links = l.links[:n-1]
}

func appendLink(l *hnswLayer, n *Node) uint32 {
	idx := uint32(len(l.links))
	l.links = append(l.links, n)
	return idx
}

// selectNeighbors is select_neighbors, aggressiveness levels included.
func (ix *Index) selectNeighbors(cands *hnswPQ, newNode *Node, layer, required uint32, aggressive int) {
	nl := &newNode.layers[layer]
	for i := uint32(0); i < cands.count; i++ {
		neighbor := cands.node(i)
		if neighbor == newNode {
			continue
		}
		dist := cands.dist(i)

		if uint32(len(nl.links)) >= nl.maxLinks || uint32(len(nl.links)) >= required {
			return
		}
		if neighbor.level < layer {
			continue
		}

		if aggressive != 0 {
			dup := false
			for _, l := range nl.links {
				if l == neighbor {
					dup = true
					break
				}
			}
			if dup {
				continue
			}
		} else {
			failed := false
			for _, l := range nl.links {
				if ix.distance(neighbor, l) < dist {
					failed = true
					break
				}
			}
			if failed {
				continue
			}
		}

		bl := &neighbor.layers[layer]
		if uint32(len(bl.links)) < bl.maxLinks {
			idx := appendLink(bl, newNode)
			updateWorstOnAdd(neighbor, layer, idx, dist)
			idx = appendLink(nl, neighbor)
			updateWorstOnAdd(newNode, layer, idx, dist)
			continue
		}

		if aggressive == 0 && dist >= bl.worstDist {
			continue
		}

		worst := bl.links[bl.worstIdx]
		if aggressive == 0 && uint32(len(worst.layers[layer].links)) <= ix.m/2 {
			continue
		} else if aggressive == 1 && uint32(len(worst.layers[layer].links)) <= ix.m/4 {
			continue
		}

		if aggressive == 2 && uint32(len(worst.layers[layer].links)) <= ix.m/4 {
			worst = nil
			worstIdx := uint32(0)
			var maxDist float32
			for j, toDrop := range bl.links {
				if uint32(len(toDrop.layers[layer].links)) <= ix.m/4 {
					continue
				}
				ld := ix.distance(neighbor, toDrop)
				if worst == nil || ld > maxDist {
					worst = toDrop
					maxDist = ld
					worstIdx = uint32(j)
				}
			}
			if worst != nil {
				bl.worstDist = maxDist
				bl.worstIdx = worstIdx
			} else {
				limit := ix.m * 2
				if layer == 0 {
					limit = ix.m * 3
				}
				if bl.maxLinks >= limit {
					continue
				}
				bl.maxLinks++
				if uint32(cap(bl.links)) < bl.maxLinks {
					grown := make([]*Node, len(bl.links), bl.maxLinks)
					copy(grown, bl.links)
					bl.links = grown
				}
				idx := appendLink(bl, newNode)
				updateWorstOnAdd(neighbor, layer, idx, dist)
				idx = appendLink(nl, neighbor)
				updateWorstOnAdd(newNode, layer, idx, dist)
				continue
			}
		}

		wl := &worst.layers[layer]
		for j, l := range wl.links {
			if l == neighbor {
				removeLinkAt(wl, j)
				ix.updateWorstOnRemove(worst, layer, uint32(j))
				break
			}
		}

		bl.links[bl.worstIdx] = newNode
		ix.updateWorst(neighbor, layer)

		idx := appendLink(nl, neighbor)
		updateWorstOnAdd(newNode, layer, idx, dist)
	}
}

/* ============================ Insertion ================================= */

// Insert is hnsw_insert with a float vector that is not yet normalized.
// ef <= 0 selects DefaultEF. It returns nil if vec has the wrong dimension.
func (ix *Index) Insert(vec []float32, value any, ef int) *Node {
	if len(vec) != int(ix.dim) {
		return nil
	}
	if ef <= 0 {
		ef = DefaultEF
	}
	level := ix.randomLevel()
	node := ix.newNode(0, level)
	v := make([]float32, ix.dim)
	copy(v, vec)
	if l2, ok := hnswNormalize(v); ok {
		node.l2 = l2
	}
	ix.setVector(node, v)
	node.Value = value

	if ix.ep == nil {
		ix.ep = node
		ix.maxLevel = level
		ix.addNode(node)
		return node
	}

	sc := getHNSWScratch()
	defer putHNSWScratch(sc)

	curEp := ix.greedyDescend(sc, node, ix.ep, ix.maxLevel, level)

	efc := ix.clampK(ef)
	top := min(level, ix.maxLevel)
	var queues [MaxLevel]hnswPQ
	for lc := int(top); lc >= 0; lc-- {
		if uint32(cap(ix.insQueues[lc])) < efc {
			ix.insQueues[lc] = make([]hnswPQItem, efc)
		}
		queues[lc] = hnswPQ{items: ix.insQueues[lc][:efc]}
		ix.searchLayer(sc, node, curEp, &queues[lc], uint32(lc), nil, 0)
		if queues[lc].count > 0 {
			curEp = queues[lc].node(0)
		}
	}

	for lc := int(top); lc >= 0; lc-- {
		q := &queues[lc]
		layer := uint32(lc)
		ix.selectNeighbors(q, node, layer, ix.m, 0)
		if lc == 0 && uint32(len(node.layers[0].links)) < ix.m/2 {
			ix.selectNeighbors(q, node, layer, ix.m, 1)
			if uint32(len(node.layers[0].links)) < ix.m/4 {
				ix.selectNeighbors(q, node, layer, ix.m/4, 2)
			}
		}
		clear(q.items)
	}

	if level > ix.maxLevel {
		ix.ep = node
		ix.maxLevel = level
	}
	ix.addNode(node)
	return node
}

/* ============================ Deletion ================================== */

func (ix *Index) unlink(n *Node) {
	for level := uint32(0); level <= n.level; level++ {
		for _, linked := range n.layers[level].links {
			ll := &linked.layers[level]
			for j, l := range ll.links {
				if l == n {
					removeLinkAt(ll, j)
					ix.updateWorstOnRemove(linked, level, uint32(j))
					break
				}
			}
		}
	}

	if n.prev != nil {
		n.prev.next = n.next
	} else {
		ix.head = n.next
	}
	if n.next != nil {
		n.next.prev = n.prev
	}
	ix.count--

	if n == ix.ep {
		ix.ep = nil
		ix.maxLevel = 0
		for level := int(n.level); level >= 0; level-- {
			if len(n.layers[level].links) > 0 {
				ix.ep = n.layers[level].links[0]
				break
			}
		}
		if ix.ep == nil {
			newMax := uint32(0)
			for cur := ix.head; cur != nil; cur = cur.next {
				if cur != n && cur.level >= newMax {
					newMax = cur.level
					ix.ep = cur
				}
			}
		}
		if ix.ep != nil {
			ix.maxLevel = ix.ep.level
		}
	}
	n.prev = nil
	n.next = nil
}

// Delete is hnsw_delete_node: the node is removed and its former neighbors
// are relinked among themselves (or with the rest of the graph).
func (ix *Index) Delete(n *Node) {
	ix.unlink(n)
	for j := uint32(0); j <= n.level; j++ {
		ix.reconnect(n.layers[j].links, j)
	}
	ix.freeSlots = append(ix.freeSlots, n.slot)
	for j := range n.layers {
		n.layers[j].links = nil
	}
}

// reconnect is hnsw_reconnect_nodes.
func (ix *Index) reconnect(nodes []*Node, layer uint32) {
	count := len(nodes)
	if count == 0 {
		return
	}
	distances := make([]float32, count*count)
	for i := range count {
		for j := i + 1; j < count; j++ {
			d := ix.distance(nodes[i], nodes[j])
			distances[i*count+j] = d
			distances[j*count+i] = d
		}
	}

	rowAvgs := make([]float32, count)
	for i := range count {
		var sum float32
		valid := 0
		for j := range count {
			if i != j {
				sum += distances[i*count+j]
				valid++
			}
		}
		if valid != 0 {
			rowAvgs[i] = sum / float32(valid)
		}
	}

	const w1 = float32(0.7)
	const w2 = float32(0.3)
	scores := make([]float32, count*count)
	for i := range count {
		li := nodes[i].layers[layer].links
		for j := range count {
			if i == j {
				scores[i*count+j] = -1
				continue
			}
			linked := false
			for _, l := range li {
				if l == nodes[j] {
					linked = true
					break
				}
			}
			if linked {
				scores[i*count+j] = -1
				continue
			}
			dist := distances[i*count+j]
			var avgI, avgJ float32
			if count > 2 {
				avgI = float32(float32(rowAvgs[i]*float32(count-1))-dist) / float32(count-2)
				avgJ = float32(float32(rowAvgs[j]*float32(count-1))-dist) / float32(count-2)
			}
			scores[i*count+j] = float32(w1*(2-dist)) + float32(w2*((avgI+avgJ)/2))
		}
	}

	hasRoom := func(n *Node) bool {
		l := &n.layers[layer]
		return uint32(len(l.links)) < l.maxLinks
	}

	used := make([]bool, count)
	for {
		maxScore := float32(-1)
		bestI, bestJ := -1, -1
		for i := range count {
			if used[i] || !hasRoom(nodes[i]) {
				continue
			}
			for j := range count {
				if i == j || used[j] {
					continue
				}
				score := scores[i*count+j]
				if score < 0 {
					continue
				}
				if score > maxScore && hasRoom(nodes[j]) {
					maxScore = score
					bestI, bestJ = i, j
				}
			}
		}
		if bestJ == -1 {
			break
		}
		dist := distances[bestI*count+bestJ]
		idx := appendLink(&nodes[bestI].layers[layer], nodes[bestJ])
		updateWorstOnAdd(nodes[bestI], layer, idx, dist)
		idx = appendLink(&nodes[bestJ].layers[layer], nodes[bestI])
		updateWorstOnAdd(nodes[bestJ], layer, idx, dist)
		used[bestI] = true
		used[bestJ] = true
	}

	var sc *hnswScratch
	defer func() {
		if sc != nil {
			putHNSWScratch(sc)
		}
	}()
	for i := range count {
		if used[i] || !hasRoom(nodes[i]) {
			continue
		}
		cands := hnswPQ{items: make([]hnswPQItem, count)}
		for j := range count {
			if i != j && hasRoom(nodes[j]) {
				cands.push(nodes[j], distances[i*count+j])
			}
		}
		wanted := uint32(len(nodes[i].layers[layer].links)) + 1
		if cands.count > 0 {
			ix.selectNeighbors(&cands, nodes[i], layer, wanted, 1)
		}
		if uint32(len(nodes[i].layers[layer].links)) == wanted {
			continue
		}
		if ix.ep == nil {
			continue
		}
		if sc == nil {
			sc = getHNSWScratch()
		}
		curEp := ix.greedyDescend(sc, nodes[i], ix.ep, ix.maxLevel, layer)
		// Redis reads past the node's layers here when the entry point is
		// below this layer (only reachable after unusual entry point
		// replacements); we skip the broader search instead.
		if curEp.level < layer {
			continue
		}
		res := hnswPQ{items: make([]hnswPQItem, min(uint64(DefaultEF), ix.count))}
		ix.searchLayer(sc, nodes[i], curEp, &res, layer, nil, 0)
		aggr := 2
		if uint32(len(nodes[i].layers[layer].links)) > ix.m/2 {
			aggr = 1
		}
		ix.selectNeighbors(&res, nodes[i], layer, wanted, aggr)
	}
}

/* ============================ Random node =============================== */

// RandomNode is hnsw_random_node: a random descent from the entry point
// followed by a random walk on layer 0. r nil uses the global source.
func (ix *Index) RandomNode(r *rand.Rand) *Node {
	if ix.count == 0 || ix.ep == nil {
		return nil
	}
	rnd := rand.Int32
	if r != nil {
		rnd = r.Int32
	}
	cur := ix.ep
	for level := ix.maxLevel; level > 0; level-- {
		if cur.level < level || len(cur.layers[level].links) == 0 {
			continue
		}
		links := cur.layers[level].links
		cur = links[uint32(rnd())%uint32(len(links))]
	}
	walks := uint32(math.Log2(float64(ix.count+1)) * 3)
	walks += uint32(rnd() & 1)
	for range walks {
		links := cur.layers[0].links
		if len(links) == 0 {
			return cur
		}
		cur = links[uint32(rnd())%uint32(len(links))]
	}
	return cur
}

/* ============================ Validation ================================ */

// ValidateGraph is hnsw_validate_graph without the debug output: the number
// of nodes reachable from the entry point and whether every link has its
// backlink.
func (ix *Index) ValidateGraph() (connected uint64, reciprocal bool) {
	if ix.ep == nil {
		return 0, true
	}
	reciprocal = true
	visited := make(map[*Node]struct{}, ix.count)
	stack := []*Node{ix.ep}
	visited[ix.ep] = struct{}{}
	connected = 1
	for len(stack) > 0 {
		cur := stack[len(stack)-1]
		stack = stack[:len(stack)-1]
		for level := uint32(0); level <= cur.level; level++ {
			for _, nb := range cur.layers[level].links {
				back := false
				if level <= nb.level {
					for _, l := range nb.layers[level].links {
						if l == cur {
							back = true
							break
						}
					}
				}
				if !back {
					reciprocal = false
				}
				if _, ok := visited[nb]; !ok {
					visited[nb] = struct{}{}
					connected++
					stack = append(stack, nb)
				}
			}
		}
	}
	return connected, reciprocal
}
