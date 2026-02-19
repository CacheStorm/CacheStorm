package store

import (
	"sync"
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
	children  map[string]map[string]struct{}
	parents   map[string]string
}

func NewTagIndex() *TagIndex {
	ti := &TagIndex{
		children: make(map[string]map[string]struct{}),
		parents:  make(map[string]string),
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
	for _, tag := range tags {
		idx := ti.shardIndex(tag)
		shard := ti.shards[idx]
		shard.mu.Lock()
		if shard.index[tag] == nil {
			shard.index[tag] = make(map[string]struct{})
		}
		shard.index[tag][key] = struct{}{}
		shard.mu.Unlock()
	}
}

func (ti *TagIndex) RemoveTags(key string, tags []string) {
	for _, tag := range tags {
		idx := ti.shardIndex(tag)
		shard := ti.shards[idx]
		shard.mu.Lock()
		if keys, exists := shard.index[tag]; exists {
			delete(keys, key)
			if len(keys) == 0 {
				delete(shard.index, tag)
			}
		}
		shard.mu.Unlock()
	}
}

func (ti *TagIndex) RemoveKey(key string, tags []string) {
	if len(tags) > 0 {
		ti.RemoveTags(key, tags)
		return
	}
	for i := 0; i < TagShards; i++ {
		shard := ti.shards[i]
		shard.mu.Lock()
		for tag, keys := range shard.index {
			delete(keys, key)
			if len(keys) == 0 {
				delete(shard.index, tag)
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

func (ti *TagIndex) Invalidate(tag string) []string {
	idx := ti.shardIndex(tag)
	shard := ti.shards[idx]
	shard.mu.Lock()
	defer shard.mu.Unlock()

	keys := make([]string, 0, len(shard.index[tag]))
	for k := range shard.index[tag] {
		keys = append(keys, k)
	}
	delete(shard.index, tag)
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

func (ti *TagIndex) Link(parent, child string) {
	ti.hierarchy.Lock()
	defer ti.hierarchy.Unlock()

	if ti.children[parent] == nil {
		ti.children[parent] = make(map[string]struct{})
	}
	ti.children[parent][child] = struct{}{}
	ti.parents[child] = parent
}

func (ti *TagIndex) Unlink(parent, child string) {
	ti.hierarchy.Lock()
	defer ti.hierarchy.Unlock()

	if children, exists := ti.children[parent]; exists {
		delete(children, child)
		if len(children) == 0 {
			delete(ti.children, parent)
		}
	}
	delete(ti.parents, child)
}

func (ti *TagIndex) GetChildren(tag string) []string {
	ti.hierarchy.RLock()
	defer ti.hierarchy.RUnlock()

	children := make([]string, 0, len(ti.children[tag]))
	for child := range ti.children[tag] {
		children = append(children, child)
	}
	return children
}

func (ti *TagIndex) GetAllDescendants(tag string) []string {
	ti.hierarchy.RLock()
	defer ti.hierarchy.RUnlock()

	descendants := make(map[string]struct{})
	ti.collectDescendants(tag, descendants)

	result := make([]string, 0, len(descendants))
	for d := range descendants {
		result = append(result, d)
	}
	return result
}

func (ti *TagIndex) collectDescendants(tag string, descendants map[string]struct{}) {
	for child := range ti.children[tag] {
		descendants[child] = struct{}{}
		ti.collectDescendants(child, descendants)
	}
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
