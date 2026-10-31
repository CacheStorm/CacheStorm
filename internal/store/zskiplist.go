package store

import "math/rand/v2"

// zskiplist orders a sorted set's members by (score, member) the way Redis'
// zskiplist does, with spans so that ranks cost O(log N).
type zskiplist struct {
	header *zslNode
	tail   *zslNode
	length int
	level  int
}

type zslNode struct {
	member   string
	score    float64
	backward *zslNode
	level    []zslLevel
}

type zslLevel struct {
	forward *zslNode
	span    int
}

const (
	zslMaxLevel = 32
)

func newZskiplist() *zskiplist {
	return &zskiplist{header: &zslNode{level: make([]zslLevel, zslMaxLevel)}, level: 1}
}

func zslRandomLevel() int {
	level := 1
	for level < zslMaxLevel && rand.Uint32()&3 == 0 {
		level++
	}
	return level
}

// newZslNode allocates a node with its level array in the same block, so a
// search step reads one block instead of two. Most nodes have one or two
// levels.
func newZslNode(level int) *zslNode {
	switch level {
	case 1:
		n := &struct {
			zslNode
			lv [1]zslLevel
		}{}
		n.level = n.lv[:]
		return &n.zslNode
	case 2:
		n := &struct {
			zslNode
			lv [2]zslLevel
		}{}
		n.level = n.lv[:]
		return &n.zslNode
	case 3:
		n := &struct {
			zslNode
			lv [3]zslLevel
		}{}
		n.level = n.lv[:]
		return &n.zslNode
	case 4:
		n := &struct {
			zslNode
			lv [4]zslLevel
		}{}
		n.level = n.lv[:]
		return &n.zslNode
	}
	return &zslNode{level: make([]zslLevel, level)}
}

func zslLess(score float64, member string, n *zslNode) bool {
	return score < n.score || (score == n.score && member < n.member)
}

func (zsl *zskiplist) insert(score float64, member string) {
	var update [zslMaxLevel]*zslNode
	var rank [zslMaxLevel]int
	x := zsl.header
	for i := zsl.level - 1; i >= 0; i-- {
		if i < zsl.level-1 {
			rank[i] = rank[i+1]
		}
		for x.level[i].forward != nil && zslNodeBefore(x.level[i].forward, score, member) {
			rank[i] += x.level[i].span
			x = x.level[i].forward
		}
		update[i] = x
	}
	level := zslRandomLevel()
	if level > zsl.level {
		for i := zsl.level; i < level; i++ {
			rank[i] = 0
			update[i] = zsl.header
			update[i].level[i].span = zsl.length
		}
		zsl.level = level
	}
	n := newZslNode(level)
	n.member, n.score = member, score
	for i := 0; i < level; i++ {
		n.level[i].forward = update[i].level[i].forward
		update[i].level[i].forward = n
		n.level[i].span = update[i].level[i].span - (rank[0] - rank[i])
		update[i].level[i].span = rank[0] - rank[i] + 1
	}
	for i := level; i < zsl.level; i++ {
		update[i].level[i].span++
	}
	if update[0] != zsl.header {
		n.backward = update[0]
	}
	if n.level[0].forward != nil {
		n.level[0].forward.backward = n
	} else {
		zsl.tail = n
	}
	zsl.length++
}

func (zsl *zskiplist) delete(score float64, member string) bool {
	var update [zslMaxLevel]*zslNode
	x := zsl.header
	for i := zsl.level - 1; i >= 0; i-- {
		for x.level[i].forward != nil && zslNodeBefore(x.level[i].forward, score, member) {
			x = x.level[i].forward
		}
		update[i] = x
	}
	x = x.level[0].forward
	if x == nil || x.score != score || x.member != member {
		return false
	}
	zsl.unlink(x, &update)
	return true
}

// zslNodeBefore reports whether n sorts before (score, member).
func zslNodeBefore(n *zslNode, score float64, member string) bool {
	return n.score < score || (n.score == score && n.member < member)
}

func (zsl *zskiplist) unlink(x *zslNode, update *[zslMaxLevel]*zslNode) {
	for i := 0; i < zsl.level; i++ {
		if update[i].level[i].forward == x {
			update[i].level[i].span += x.level[i].span - 1
			update[i].level[i].forward = x.level[i].forward
		} else {
			update[i].level[i].span--
		}
	}
	if x.level[0].forward != nil {
		x.level[0].forward.backward = x.backward
	} else {
		zsl.tail = x.backward
	}
	for zsl.level > 1 && zsl.header.level[zsl.level-1].forward == nil {
		zsl.level--
	}
	zsl.length--
}

// rank returns the 1-based rank of (score, member), 0 when it is absent.
func (zsl *zskiplist) rank(score float64, member string) int {
	r := 0
	x := zsl.header
	for i := zsl.level - 1; i >= 0; i-- {
		for x.level[i].forward != nil && !zslLess(score, member, x.level[i].forward) {
			r += x.level[i].span
			x = x.level[i].forward
		}
		if x != zsl.header && x.score == score && x.member == member {
			return r
		}
	}
	return 0
}

// byRank returns the node at the 1-based rank, nil when out of range.
func (zsl *zskiplist) byRank(rank int) *zslNode {
	if rank < 1 || rank > zsl.length {
		return nil
	}
	traversed := 0
	x := zsl.header
	for i := zsl.level - 1; i >= 0; i-- {
		for x.level[i].forward != nil && traversed+x.level[i].span <= rank {
			traversed += x.level[i].span
			x = x.level[i].forward
		}
		if traversed == rank {
			return x
		}
	}
	return nil
}

// countBefore returns how many nodes satisfy before, which must hold for a
// prefix of the list.
func (zsl *zskiplist) countBefore(before func(*zslNode) bool) int {
	r := 0
	x := zsl.header
	for i := zsl.level - 1; i >= 0; i-- {
		for x.level[i].forward != nil && before(x.level[i].forward) {
			r += x.level[i].span
			x = x.level[i].forward
		}
	}
	return r
}

func (zsl *zskiplist) first() *zslNode { return zsl.header.level[0].forward }
