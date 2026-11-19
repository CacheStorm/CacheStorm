package vectorset

import (
	"encoding/binary"
	"errors"
	"math"
	"slices"
)

var (
	errHNSWParams     = errors.New("vector set node index loading error")
	errHNSWVectorSize = errors.New("mismatching vector dimension")
	errHNSWCorrupted  = errors.New("vector set graph is corrupted")
)

// SerializeNode is hnsw_serialize_node: the stored vector bytes and the
// params array (id, version/level word, per layer num_links, max_links,
// linked ids and packed worst link info, then packed l2 and range).
func (ix *Index) SerializeNode(n *Node) (vector []byte, params []uint64) {
	num := 2 + 1
	for i := range n.layers {
		num += 3 + len(n.layers[i].links)
	}
	params = make([]uint64, 0, num)
	params = append(params, n.id, uint64(n.level&0xff)|hnswSerVersion<<24)
	for i := range n.layers {
		l := &n.layers[i]
		params = append(params, uint64(len(l.links)), uint64(l.maxLinks))
		for _, nb := range l.links {
			params = append(params, nb.id)
		}
		params = append(params, uint64(math.Float32bits(l.worstDist))<<32|uint64(l.worstIdx))
	}
	params = append(params, uint64(math.Float32bits(n.qrange))<<32|uint64(math.Float32bits(n.l2)))
	return n.Raw(), params
}

// InsertSerialized is hnsw_insert_serialized. The links are kept as ids
// until Deserialize resolves them. Like Redis, it rejects inconsistent
// params but leaves the remaining validation to Deserialize; on any error
// the whole index must be discarded.
func (ix *Index) InsertSerialized(vector []byte, params []uint64, value any) (*Node, error) {
	if uint64(len(vector)) != uint64(ix.QuantBytes()) {
		return nil, errHNSWVectorSize
	}
	if len(params) < 2 || uint64(len(params)) > math.MaxUint32 {
		return nil, errHNSWParams
	}
	plen := uint32(len(params))
	id := params[0]
	level := uint32(params[1] & 0xff)
	version := uint32((params[1] & 0xff000000) >> 24)
	if version > hnswSerVersion || level >= MaxLevel {
		return nil, errHNSWParams
	}
	hasWorst := uint32(0)
	if version > 0 {
		hasWorst = 1
	}
	if id >= ix.lastID {
		ix.lastID = id
	}

	node := ix.newNode(id, level)
	switch ix.quant {
	case QuantQ8:
		node.qvec = make([]int8, len(vector))
		for i, b := range vector {
			node.qvec[i] = int8(b)
		}
	case QuantBin:
		node.bvec = make([]uint64, len(vector)/8)
		for i := range node.bvec {
			node.bvec[i] = binary.LittleEndian.Uint64(vector[i*8:])
		}
	default:
		node.fvec = make([]float32, len(vector)/4)
		for i := range node.fvec {
			node.fvec[i] = math.Float32frombits(binary.LittleEndian.Uint32(vector[i*4:]))
		}
	}

	ids := make([][]uint64, level+1)
	idx := uint32(2)
	for i := uint32(0); i <= level; i++ {
		if idx+2+hasWorst > plen {
			return nil, errHNSWParams
		}
		numLinks := uint32(params[idx])
		maxLinks := uint32(params[idx+1])
		idx += 2
		if numLinks > maxLinks || maxLinks > MaxM*4 {
			return nil, errHNSWParams
		}
		l := &node.layers[i]
		if maxLinks > l.maxLinks {
			l.maxLinks = maxLinks
			l.links = make([]*Node, 0, numLinks)
		}
		if idx+numLinks+hasWorst > plen {
			return nil, errHNSWParams
		}
		ids[i] = slices.Clone(params[idx : idx+numLinks])
		idx += numLinks
		if hasWorst != 0 {
			wi := params[idx]
			idx++
			l.worstIdx = uint32(wi)
			l.worstDist = math.Float32frombits(uint32(wi >> 32))
			if numLinks > 0 && l.worstIdx >= numLinks {
				return nil, errHNSWParams
			}
		} else {
			l.worstIdx = hnswWorstLinkMissing
			l.worstDist = 0
		}
	}
	if idx >= plen {
		return nil, errHNSWParams
	}
	lr := params[idx]
	node.l2 = math.Float32frombits(uint32(lr))
	node.qrange = math.Float32frombits(uint32(lr >> 32))

	node.Value = value
	ix.addNode(node)
	if ix.pending == nil {
		ix.pending = make(map[*Node][][]uint64)
	}
	ix.pending[node] = ids

	if ix.ep == nil || level > ix.maxLevel {
		ix.maxLevel = level
		ix.ep = node
	}
	return node, nil
}

// Deserialize is hnsw_deserialize_index: it resolves the link ids of the
// nodes added with InsertSerialized and rejects duplicated, self, dangling,
// wrong-level or non-reciprocal links. As in Redis, each layer's links end
// up sorted by node id. salt0/salt1 should be random.
func (ix *Index) Deserialize(salt0, salt1 uint64) error {
	byID := make(map[uint64]*Node, ix.count)
	for n := ix.head; n != nil; n = n.next {
		if _, ok := byID[n.id]; !ok {
			byID[n.id] = n
		}
	}

	var acc0, acc1 uint64
	for n := ix.head; n != nil; n = n.next {
		ids, ok := ix.pending[n]
		if !ok {
			continue
		}
		for i := uint32(0); i <= n.level; i++ {
			layerIDs := ids[i]
			slices.Sort(layerIDs)
			for j := 1; j < len(layerIDs); j++ {
				if layerIDs[j] == layerIDs[j-1] {
					return errHNSWCorrupted
				}
			}
			l := &n.layers[i]
			l.links = l.links[:0]
			for _, lid := range layerIDs {
				if lid == n.id {
					return errHNSWCorrupted
				}
				h1, h2 := hnswPairMixer128(salt0, salt1, n.id, lid, uint64(i))
				acc0 ^= h1
				acc1 ^= h2
				nb := byID[lid]
				if nb == nil || nb.level < i {
					return errHNSWCorrupted
				}
				l.links = append(l.links, nb)
			}
			if l.worstIdx == hnswWorstLinkMissing {
				ix.updateWorst(n, i)
			}
		}
	}
	if acc0 != 0 || acc1 != 0 {
		return errHNSWCorrupted
	}
	ix.pending = nil
	return nil
}
