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
	index map[string]map[string]struct{}
}

func newTagShard() *tagShard {
	return &tagShard{
		index: make(map[string]map[string]struct{}),
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
	children     map[string]map[string]struct{}
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
			set = make(map[string]struct{})
			shard.index[tag] = set
			ti.tags.Add(1)
		}
		before := len(set)
		for _, key := range keys {
			set[key] = struct{}{}
		}
		ti.members.Add(int64(len(set) - before))
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
			before := len(keySet)
			for _, k := range keys {
				delete(keySet, k)
			}
			ti.members.Add(int64(len(keySet) - before))
			if len(keySet) == 0 {
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
			before := len(keySet)
			for _, key := range keys {
				delete(keySet, key)
			}
			ti.members.Add(int64(len(keySet) - before))
			if len(keySet) == 0 {
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

	keys := make([]string, 0, len(shard.index[tag]))
	for k := range shard.index[tag] {
		keys = append(keys, k)
	}
	return keys
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
	if len(set) > max {
		return false
	}
	for k := range set {
		fn(k)
	}
	return true
}

// InvalidateIf invalidates tag only when held reports every key the tag has
// now. It reports false, and changes nothing, when one of them is not: the
// caller locked the keys the tag had earlier, and a key joined it since.
func (ti *TagIndex) InvalidateIf(tag string, held func(key string) bool) ([]string, bool) {
	shard := ti.shards[ti.shardIndex(tag)]
	shard.mu.Lock()
	set, ok := shard.index[tag]
	keys := make([]string, 0, len(set))
	for k := range set {
		if !held(k) {
			shard.mu.Unlock()
			return nil, false
		}
		keys = append(keys, k)
	}
	if ok {
		delete(shard.index, tag)
		ti.tags.Add(-1)
		ti.members.Add(-int64(len(set)))
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

	keys := make([]string, 0, len(shard.index[tag]))
	set, ok := shard.index[tag]
	for k := range set {
		keys = append(keys, k)
	}
	if ok {
		delete(shard.index, tag)
		ti.tags.Add(-1)
		ti.members.Add(-int64(len(set)))
	}
	return keys
}

func (ti *TagIndex) Count(tag string) int {
	idx := ti.shardIndex(tag)
	shard := ti.shards[idx]
	shard.mu.RLock()
	defer shard.mu.RUnlock()
	return len(shard.index[tag])
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

// Link adds child below parent. It refuses a link of a tag to itself or to
// one of its ancestors: CASCADE follows the links down, and a cycle made
// every tag on it a descendant of itself.
func (ti *TagIndex) Link(parent, child string) error {
	ti.hierarchy.Lock()
	defer ti.hierarchy.Unlock()
	if parent == child || ti.reachesLocked(child, parent) {
		return ErrTagLinkCycle
	}
	if ti.children[parent] == nil {
		ti.children[parent] = make(map[string]struct{})
	}
	ti.children[parent][child] = struct{}{}
	return nil
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
	ti.hierarchy.Lock()
	defer ti.hierarchy.Unlock()

	if children, exists := ti.children[parent]; exists {
		delete(children, child)
		if len(children) == 0 {
			delete(ti.children, parent)
		}
	}
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
