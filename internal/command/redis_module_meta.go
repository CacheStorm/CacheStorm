package command

import (
	"strings"
	"sync"

	"github.com/cachestorm/cachestorm/internal/resp"
)

// The commands of the modules Redis 8 bundles are described as Redis 8.2
// describes them (redis_module_commands.go): their names (FT.* in capitals,
// the others in lower case), arity, flags and ACL categories such as
// @search and @json. Only COMMAND and ACL read these; the keys a command
// touches still come from CacheStorm's own tables.

type moduleCommandData struct {
	module string
	info   string
	docs   string
}

type moduleCommandMeta struct {
	info       *resp.Value
	docs       *resp.Value
	name       string
	module     string
	categories []string
}

var (
	moduleMetaOnce  sync.Once
	moduleMetaTable map[string]*moduleCommandMeta
)

// moduleCommand returns the Redis 8.2 description of a module command, or
// nil.
func moduleCommand(name string) *moduleCommandMeta {
	moduleMetaOnce.Do(func() {
		moduleMetaTable = make(map[string]*moduleCommandMeta, len(redisModuleCommands))
		for key, d := range redisModuleCommands {
			info, _ := decodeRESP3(d.info)
			docs, _ := decodeRESP3(d.docs)
			if info == nil || len(info.Array) < 7 {
				continue
			}
			m := &moduleCommandMeta{info: info, docs: docs, name: string(info.Array[0].Bulk), module: d.module}
			for _, c := range info.Array[6].Array {
				m.categories = append(m.categories, strings.TrimPrefix(c.Str, "@"))
			}
			moduleMetaTable[key] = m
		}
	})
	return moduleMetaTable[strings.ToLower(name)]
}

// moduleCategoryOrder is the order ACL CAT lists Redis 8.2's module
// categories in, after the core ones.
var moduleCategoryOrder = []string{"bloom", "cuckoo", "cms", "topk", "tdigest", "search", "timeseries", "json"}

// moduleCategoryMembers adds the registered module commands to the ACL
// categories they belong to.
func moduleCategoryMembers(category string) []string {
	var out []string
	for key := range redisModuleCommands {
		m := moduleCommand(key)
		if m == nil {
			continue
		}
		name := strings.ToUpper(key)
		if globalRouter != nil {
			if _, ok := globalRouter.Get(name); !ok {
				continue
			}
		}
		for _, c := range m.categories {
			if c == category {
				out = append(out, name)
			}
		}
	}
	return out
}

// commandDisplayName is a command's name as COMMAND and ACL show it.
func commandDisplayName(name string) string {
	if m := moduleCommand(name); m != nil {
		return m.name
	}
	return strings.ToLower(name)
}
