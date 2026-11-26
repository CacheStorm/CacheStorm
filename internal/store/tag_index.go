package store

import (
	"errors"
	"slices"
	"strings"
	"sync"
	"sync/atomic"
)

const (
	TagShards    = 64
	TagShardMask = TagShards - 1
)

type tagShard struct {
	mu    sync.RWMutex
	index map[string]*strSet
}

func newTagShard() *tagShard {
	return &tagShard{
		index: make(map[string]*strSet),
	}
}

type TagIndex struct {
	shards    [TagShards]*tagShard
	hierarchy sync.RWMutex
	// hookMu guards onInvalidate. It is separate from hierarchy because the
	// callback is invoked on the hot path and must be callable from a store
	// callback that itself calls back into the store -- so it is never held
	// while the callback runs.
	hookMu sync.RWMutex
	// children holds the hierarchy: each parent tag's child tags. A tag
	// may have several parents; TAGLINK refuses a link that would close a
	// cycle.
	children map[string]map[string]struct{}
	// edges holds the version of every link, and in a cluster of every
	// removed one, so that nodes applying the same changes in any order
	// keep the one with the highest version (see ApplyEdge). clock is the
	// highest version seen, and digest the XOR of the edges' hashes, which
	// the nodes of a cluster compare to find out they differ.
	edges        map[[2]string]edgeVersion
	clock        uint64
	digest       atomic.Uint64
	onInvalidate func(tag string, keys []string)
	// tags counts the tags indexed, so removing a key that may have tags
	// skips the walk of every shard when no key has any.
	tags atomic.Int64
	// members counts the (tag, key) pairs indexed.
	members atomic.Int64
	// stats counts the INVALIDATE commands run on this index.
	stats tagStats
}

// tagStats are the INVALIDATE counters INFO reports.
type tagStats struct {
	invalidations   atomic.Int64
	invalidatedKeys atomic.Int64
	maxUsec         atomic.Int64
}

// TagStats is a snapshot of a tag index's size and INVALIDATE counters.
type TagStats struct {
	Tags, Members                  int64
	Invalidations, InvalidatedKeys int64
	MaxInvalidateUsec              int64
}

// Stats returns the index's size and INVALIDATE counters.
func (ti *TagIndex) Stats() TagStats {
	return TagStats{
		Tags:              ti.tags.Load(),
		Members:           ti.members.Load(),
		Invalidations:     ti.stats.invalidations.Load(),
		InvalidatedKeys:   ti.stats.invalidatedKeys.Load(),
		MaxInvalidateUsec: ti.stats.maxUsec.Load(),
	}
}

// RecordInvalidation counts one INVALIDATE that deleted keys keys and took
// usec microseconds.
func (ti *TagIndex) RecordInvalidation(keys int, usec int64) {
	ti.stats.invalidations.Add(1)
	ti.stats.invalidatedKeys.Add(int64(keys))
	for {
		cur := ti.stats.maxUsec.Load()
		if usec <= cur || ti.stats.maxUsec.CompareAndSwap(cur, usec) {
			return
		}
	}
}

// ResetStats zeroes the INVALIDATE counters (CONFIG RESETSTAT).
func (ti *TagIndex) ResetStats() {
	ti.stats.invalidations.Store(0)
	ti.stats.invalidatedKeys.Store(0)
	ti.stats.maxUsec.Store(0)
}

func NewTagIndex() *TagIndex {
	ti := &TagIndex{
		children: make(map[string]map[string]struct{}),
		edges:    make(map[[2]string]edgeVersion),
	}
	for i := 0; i < TagShards; i++ {
		ti.shards[i] = newTagShard()
	}
	return ti
}

func (ti *TagIndex) shardIndex(tag string) uint32 {
	return fnv32a(tag) & TagShardMask
}

func (ti *TagIndex) AddTags(key string, tags []string) {
	ti.AddTagsBatch([]string{key}, tags)
}

func (ti *TagIndex) AddTagsBatch(keys []string, tags []string) {
	if len(keys) == 0 || len(tags) == 0 {
		return
	}

	for _, tag := range tags {
		idx := ti.shardIndex(tag)
		shard := ti.shards[idx]
		shard.mu.Lock()
		set := shard.index[tag]
		if set == nil {
			set = newStrSet()
			shard.index[tag] = set
			ti.tags.Add(1)
		}
		added := 0
		for _, key := range keys {
			if set.Add(key) {
				added++
			}
		}
		ti.members.Add(int64(added))
		shard.mu.Unlock()
	}
}

func (ti *TagIndex) RemoveTags(key string, tags []string) {
	ti.RemoveTagsBatch([]string{key}, tags)
}

func (ti *TagIndex) RemoveTagsBatch(keys []string, tags []string) {
	if len(keys) == 0 || len(tags) == 0 {
		return
	}
	for _, tag := range tags {
		idx := ti.shardIndex(tag)
		shard := ti.shards[idx]
		shard.mu.Lock()
		if keySet, exists := shard.index[tag]; exists {
			removed := 0
			for _, k := range keys {
				if keySet.Remove(k) {
					removed++
				}
			}
			ti.members.Add(-int64(removed))
			if keySet.Len() == 0 {
				delete(shard.index, tag)
				ti.tags.Add(-1)
			}
		}
		shard.mu.Unlock()
	}
}

// RemoveKey drops key from the tags it had. The entry's tag list is the
// truth the index follows, so a key with no tags is in no tag's set and
// costs nothing, however many tags other keys have.
func (ti *TagIndex) RemoveKey(key string, tags []string) {
	if len(tags) > 0 {
		ti.RemoveTags(key, tags)
	}
}

func (ti *TagIndex) RemoveTagsBatchAllShards(keys []string) {
	if ti.tags.Load() == 0 {
		return
	}
	for i := 0; i < TagShards; i++ {
		shard := ti.shards[i]
		shard.mu.Lock()
		for tag, keySet := range shard.index {
			removed := 0
			for _, key := range keys {
				if keySet.Remove(key) {
					removed++
				}
			}
			ti.members.Add(-int64(removed))
			if keySet.Len() == 0 {
				delete(shard.index, tag)
				ti.tags.Add(-1)
			}
		}
		shard.mu.Unlock()
	}
}

func (ti *TagIndex) GetKeys(tag string) []string {
	idx := ti.shardIndex(tag)
	shard := ti.shards[idx]
	shard.mu.RLock()
	defer shard.mu.RUnlock()

	set := shard.index[tag]
	if set == nil {
		return []string{}
	}
	return set.AppendTo(make([]string, 0, set.Len()))
}

// SetOnInvalidate installs a callback fired after a tag's keys are removed
// from the index. The callback runs outside the shard lock so the consumer
// may call back into the store safely.
func (ti *TagIndex) SetOnInvalidate(fn func(tag string, keys []string)) {
	ti.hookMu.Lock()
	ti.onInvalidate = fn
	ti.hookMu.Unlock()
}

func (ti *TagIndex) Invalidate(tag string) []string {
	keys := ti.invalidate(tag)
	// Read the callback under the lock, but invoke it outside it: the callback
	// runs store code that may call back into the tag index.
	ti.hookMu.RLock()
	onInvalidate := ti.onInvalidate
	ti.hookMu.RUnlock()
	if onInvalidate != nil {
		onInvalidate(tag, keys)
	}
	return keys
}

// KeysUpTo calls fn with each key of tag, unless the tag has more than max
// keys, which it reports by returning false without calling fn.
func (ti *TagIndex) KeysUpTo(tag string, max int, fn func(key string)) bool {
	shard := ti.shards[ti.shardIndex(tag)]
	shard.mu.RLock()
	defer shard.mu.RUnlock()
	set := shard.index[tag]
	if set == nil {
		return true
	}
	if set.Len() > max {
		return false
	}
	set.Range(func(k string) bool {
		fn(k)
		return true
	})
	return true
}

// InvalidateIf invalidates tag only when held reports every key the tag has
// now. It reports false, and changes nothing, when one of them is not: the
// caller locked the keys the tag had earlier, and a key joined it since.
func (ti *TagIndex) InvalidateIf(tag string, held func(key string) bool) ([]string, bool) {
	shard := ti.shards[ti.shardIndex(tag)]
	shard.mu.Lock()
	set, ok := shard.index[tag]
	var keys []string
	if ok {
		keys = make([]string, 0, set.Len())
		covered := true
		set.Range(func(k string) bool {
			if !held(k) {
				covered = false
				return false
			}
			keys = append(keys, k)
			return true
		})
		if !covered {
			shard.mu.Unlock()
			return nil, false
		}
		delete(shard.index, tag)
		ti.tags.Add(-1)
		ti.members.Add(-int64(set.Len()))
	}
	shard.mu.Unlock()
	ti.hookMu.RLock()
	onInvalidate := ti.onInvalidate
	ti.hookMu.RUnlock()
	if onInvalidate != nil {
		onInvalidate(tag, keys)
	}
	return keys, true
}

func (ti *TagIndex) invalidate(tag string) []string {
	idx := ti.shardIndex(tag)
	shard := ti.shards[idx]
	shard.mu.Lock()
	defer shard.mu.Unlock()

	set, ok := shard.index[tag]
	if !ok {
		return []string{}
	}
	keys := set.AppendTo(make([]string, 0, set.Len()))
	delete(shard.index, tag)
	ti.tags.Add(-1)
	ti.members.Add(-int64(set.Len()))
	return keys
}

func (ti *TagIndex) Count(tag string) int {
	idx := ti.shardIndex(tag)
	shard := ti.shards[idx]
	shard.mu.RLock()
	defer shard.mu.RUnlock()
	if set := shard.index[tag]; set != nil {
		return set.Len()
	}
	return 0
}

func (ti *TagIndex) Tags() []string {
	tags := make([]string, 0)
	for i := 0; i < TagShards; i++ {
		shard := ti.shards[i]
		shard.mu.RLock()
		for tag := range shard.index {
			tags = append(tags, tag)
		}
		shard.mu.RUnlock()
	}
	return tags
}

// ErrTagLinkCycle is TAGLINK's error for a link that would make a tag its
// own descendant.
var ErrTagLinkCycle = errors.New("ERR the link would make the tag its own descendant")

// TagEdge is one link of the hierarchy as set or removed by a version:
// Version orders the changes, Node (the cluster node that made the change,
// empty outside a cluster) breaks ties, and Live is false for a removed
// link a cluster keeps so that an older addition cannot bring it back.
type TagEdge struct {
	Parent, Child string
	Live          bool
	Version       uint64
	Node          string
}

type edgeVersion struct {
	live bool
	ver  uint64
	node string
}

func (v edgeVersion) newer(than edgeVersion) bool {
	if v.ver != than.ver {
		return v.ver > than.ver
	}
	return v.node > than.node
}

func edgeHash(link [2]string, v edgeVersion) uint64 {
	h := uint64(14695981039346656037)
	mix := func(s string) {
		for i := 0; i < len(s); i++ {
			h ^= uint64(s[i])
			h *= 1099511628211
		}
		h ^= 0xff
		h *= 1099511628211
	}
	mix(link[0])
	mix(link[1])
	mix(v.node)
	for i := 0; i < 8; i++ {
		h ^= (v.ver >> (8 * i)) & 0xff
		h *= 1099511628211
	}
	if v.live {
		h ^= 1
		h *= 1099511628211
	}
	return h
}

// setEdgeLocked records link at v, or forgets it when v is nil, keeping
// children and the digest in step. The caller holds ti.hierarchy.
func (ti *TagIndex) setEdgeLocked(link [2]string, v *edgeVersion) {
	if old, ok := ti.edges[link]; ok {
		ti.digest.Store(ti.digest.Load() ^ edgeHash(link, old))
	}
	parent, child := link[0], link[1]
	if v == nil || !v.live {
		if children, ok := ti.children[parent]; ok {
			delete(children, child)
			if len(children) == 0 {
				delete(ti.children, parent)
			}
		}
	} else {
		if ti.children[parent] == nil {
			ti.children[parent] = make(map[string]struct{})
		}
		ti.children[parent][child] = struct{}{}
	}
	if v == nil {
		delete(ti.edges, link)
		return
	}
	ti.edges[link] = *v
	ti.digest.Store(ti.digest.Load() ^ edgeHash(link, *v))
	ti.clock = max(ti.clock, v.ver)
}

// Link adds child below parent. It refuses a link of a tag to itself or to
// one of its ancestors: CASCADE follows the links down, and a cycle made
// every tag on it a descendant of itself.
func (ti *TagIndex) Link(parent, child string) error {
	_, err := ti.LinkAt(parent, child, "")
	return err
}

// LinkAt is Link made by node: it returns the link with the version it
// got, for the other nodes to apply.
func (ti *TagIndex) LinkAt(parent, child, node string) (TagEdge, error) {
	ti.hierarchy.Lock()
	defer ti.hierarchy.Unlock()
	if parent == child || ti.reachesLocked(child, parent) {
		return TagEdge{}, ErrTagLinkCycle
	}
	v := edgeVersion{live: true, ver: ti.clock + 1, node: node}
	ti.setEdgeLocked([2]string{parent, child}, &v)
	return TagEdge{Parent: parent, Child: child, Live: true, Version: v.ver, Node: node}, nil
}

// reachesLocked reports whether to lies below from. The caller holds
// ti.hierarchy.
func (ti *TagIndex) reachesLocked(from, to string) bool {
	if len(ti.children[from]) == 0 {
		return false
	}
	found := false
	ti.walkLocked(from, func(tag string) bool {
		found = tag == to
		return !found
	})
	return found
}

// walkLocked calls fn for every tag below root once, breadth first, until
// fn returns false. It keeps its own queue, so a deep hierarchy costs heap,
// not stack, and a tag reached twice (several parents, or a cycle loaded
// from an older version that accepted one) is visited once. The caller
// holds ti.hierarchy.
func (ti *TagIndex) walkLocked(root string, fn func(tag string) bool) {
	visited := map[string]struct{}{root: {}}
	queue := []string{root}
	for len(queue) > 0 {
		tag := queue[0]
		queue = queue[1:]
		for child := range ti.children[tag] {
			if _, seen := visited[child]; seen {
				continue
			}
			visited[child] = struct{}{}
			if !fn(child) {
				return
			}
			queue = append(queue, child)
		}
	}
}

// Unlink removes the link between parent and child, and only that one: a
// child linked below other parents stays below them.
func (ti *TagIndex) Unlink(parent, child string) {
	ti.UnlinkAt(parent, child, "")
}

// UnlinkAt is Unlink made by node. Outside a cluster (node empty) the link
// is forgotten; a cluster node keeps it as removed, with a version, so the
// removal reaches the other nodes and an older link cannot return.
func (ti *TagIndex) UnlinkAt(parent, child, node string) TagEdge {
	ti.hierarchy.Lock()
	defer ti.hierarchy.Unlock()
	link := [2]string{parent, child}
	v := edgeVersion{ver: ti.clock + 1, node: node}
	if node == "" {
		ti.setEdgeLocked(link, nil)
	} else {
		ti.setEdgeLocked(link, &v)
	}
	return TagEdge{Parent: parent, Child: child, Version: v.ver, Node: node}
}

// ApplyEdge applies a link change another node made, unless the index has
// a newer one for the link, and reports whether it did. Nodes applying the
// same changes in any order end with the same hierarchy. It does not check
// for cycles: the node that made the change did.
func (ti *TagIndex) ApplyEdge(e TagEdge) bool {
	ti.hierarchy.Lock()
	defer ti.hierarchy.Unlock()
	link := [2]string{e.Parent, e.Child}
	v := edgeVersion{live: e.Live, ver: e.Version, node: e.Node}
	if old, ok := ti.edges[link]; ok && !v.newer(old) {
		ti.clock = max(ti.clock, e.Version)
		return false
	}
	if !e.Live && e.Node == "" {
		ti.setEdgeLocked(link, nil)
		ti.clock = max(ti.clock, e.Version)
		return true
	}
	ti.setEdgeLocked(link, &v)
	return true
}

// Edges returns every link with its version, removed ones included, sorted.
func (ti *TagIndex) Edges() []TagEdge {
	ti.hierarchy.RLock()
	out := make([]TagEdge, 0, len(ti.edges))
	for link, v := range ti.edges {
		out = append(out, TagEdge{Parent: link[0], Child: link[1], Live: v.live, Version: v.ver, Node: v.node})
	}
	ti.hierarchy.RUnlock()
	slices.SortFunc(out, func(a, b TagEdge) int {
		if c := strings.Compare(a.Parent, b.Parent); c != 0 {
			return c
		}
		return strings.Compare(a.Child, b.Child)
	})
	return out
}

// Digest is a hash of the hierarchy and its versions: two nodes with the
// same digest have the same links.
func (ti *TagIndex) Digest() uint64 {
	return ti.digest.Load()
}

// Links returns every parent and child pair of the hierarchy, sorted.
func (ti *TagIndex) Links() [][2]string {
	ti.hierarchy.RLock()
	var links [][2]string
	for parent, children := range ti.children {
		for child := range children {
			links = append(links, [2]string{parent, child})
		}
	}
	ti.hierarchy.RUnlock()
	slices.SortFunc(links, func(a, b [2]string) int {
		if c := strings.Compare(a[0], b[0]); c != 0 {
			return c
		}
		return strings.Compare(a[1], b[1])
	})
	return links
}

func (ti *TagIndex) GetChildren(tag string) []string {
	ti.hierarchy.RLock()
	defer ti.hierarchy.RUnlock()

	children := make([]string, 0, len(ti.children[tag]))
	for child := range ti.children[tag] {
		children = append(children, child)
	}
	slices.Sort(children)
	return children
}

// GetAllDescendants returns every tag below tag, each once, without tag
// itself.
func (ti *TagIndex) GetAllDescendants(tag string) []string {
	ti.hierarchy.RLock()
	defer ti.hierarchy.RUnlock()

	var result []string
	ti.walkLocked(tag, func(d string) bool {
		if d != tag {
			result = append(result, d)
		}
		return true
	})
	return result
}

func (ti *TagIndex) InvalidateCascade(tag string) map[string][]string {
	result := make(map[string][]string)

	result[tag] = ti.Invalidate(tag)

	descendants := ti.GetAllDescendants(tag)
	for _, desc := range descendants {
		result[desc] = ti.Invalidate(desc)
	}

	return result
}
