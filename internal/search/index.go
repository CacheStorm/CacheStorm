// Package search implements the query engine behind the FT commands, after
// RediSearch, the search module built into Redis 8. An index is a
// definition: which keys it covers (HASH or JSON keys under its prefixes)
// and how their fields are read. Queries read those keys when they run, so
// an index always agrees with the keyspace.
package search

import (
	"errors"
	"sort"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"

	"github.com/cachestorm/cachestorm/internal/store"
)

// FieldType is a schema field's type.
type FieldType uint8

const (
	FieldText FieldType = iota
	FieldNumeric
	FieldTag
	FieldGeo
	FieldVector
)

var fieldTypeNames = map[FieldType]string{FieldText: "TEXT", FieldNumeric: "NUMERIC", FieldTag: "TAG", FieldGeo: "GEO", FieldVector: "VECTOR"}

func (t FieldType) String() string { return fieldTypeNames[t] }

// Field is a schema field: Identifier is the hash field or JSON path,
// Name the attribute queries use.
type Field struct {
	Identifier    string
	Name          string
	Type          FieldType
	Weight        float64
	NoStem        bool
	Sortable      bool
	NoIndex       bool
	Separator     byte
	CaseSensitive bool
	path          *store.JSONPath
	pos           int
}

// Index is an index definition. Args are the FT.CREATE arguments after the
// index name, which rebuild it.
type Index struct {
	Name      string
	JSON      bool
	Prefixes  []string
	Fields    []Field
	Stopwords map[string]bool
	Language  string
	Score     float64
	Args      []string
	cacheSet  cacheSet
}

//nolint:staticcheck // RediSearch's reply texts
var (
	ErrIndexExists   = errors.New("Index already exists")
	ErrUnknownIndex  = errors.New("Unknown index name")
	ErrAliasExists   = errors.New("Alias already exists")
	ErrAliasMissing  = errors.New("Alias does not exist")
	ErrSchemaMissing = errors.New("Fields arguments are missing")
)

// DefaultStopwords are RediSearch's English stop words.
var DefaultStopwords = []string{"a", "is", "the", "an", "and", "are", "as", "at", "be", "but", "by", "for", "if", "in",
	"into", "it", "no", "not", "of", "on", "or", "such", "that", "their", "then", "there", "these", "they", "this", "to",
	"was", "will", "with"}

func stopwordSet(words []string) map[string]bool {
	m := make(map[string]bool, len(words))
	for _, w := range words {
		m[strings.ToLower(w)] = true
	}
	return m
}

// ParseIndex reads an FT.CREATE definition: the arguments after the index
// name.
func ParseIndex(name string, args []string) (*Index, error) {
	idx := &Index{Name: name, Score: 1, Language: "english", Stopwords: stopwordSet(DefaultStopwords), Args: append([]string(nil), args...)}
	i := 0
	for ; i < len(args); i++ {
		switch strings.ToUpper(args[i]) {
		case "ON":
			if i+1 >= len(args) {
				return nil, errors.New("ERR Invalid ON argument")
			}
			i++
			switch strings.ToUpper(args[i]) {
			case "HASH":
			case "JSON":
				idx.JSON = true
			default:
				return nil, errors.New("ERR Invalid argument `" + args[i] + "` for ON")
			}
		case "PREFIX":
			if i+1 >= len(args) {
				return nil, errors.New("ERR Bad arguments for PREFIX: Expected an argument, but none provided")
			}
			n, err := strconv.Atoi(args[i+1])
			if err != nil || n < 0 || i+2+n > len(args) {
				return nil, errors.New("ERR Bad arguments for PREFIX: Expected an argument, but none provided")
			}
			idx.Prefixes = append(idx.Prefixes, args[i+2:i+2+n]...)
			i += 1 + n
		case "STOPWORDS":
			if i+1 >= len(args) {
				return nil, errors.New("ERR Bad arguments for STOPWORDS: Expected an argument, but none provided")
			}
			n, err := strconv.Atoi(args[i+1])
			if err != nil || n < 0 || i+2+n > len(args) {
				return nil, errors.New("ERR Bad arguments for STOPWORDS: Expected an argument, but none provided")
			}
			idx.Stopwords = stopwordSet(args[i+2 : i+2+n])
			i += 1 + n
		case "LANGUAGE":
			if i+1 < len(args) {
				i++
				idx.Language = strings.ToLower(args[i])
			}
		case "SCORE":
			if i+1 < len(args) {
				i++
				if s, err := strconv.ParseFloat(args[i], 64); err == nil {
					idx.Score = s
				}
			}
		case "FILTER", "LANGUAGE_FIELD", "SCORE_FIELD", "PAYLOAD_FIELD", "TEMPORARY":
			i++
		case "MAXTEXTFIELDS", "NOOFFSETS", "NOHL", "NOFIELDS", "NOFREQS", "SKIPINITIALSCAN":
		case "SCHEMA":
			fields, err := parseSchema(args[i+1:], idx.JSON)
			if err != nil {
				return nil, err
			}
			idx.Fields = fields
			i = len(args)
		}
	}
	if len(idx.Fields) == 0 {
		return nil, ErrSchemaMissing
	}
	if len(idx.Prefixes) == 0 {
		idx.Prefixes = []string{""}
	}
	return idx, nil
}

func parseSchema(args []string, json bool) ([]Field, error) {
	var fields []Field
	for i := 0; i < len(args); {
		f := Field{Identifier: args[i], Name: args[i], Weight: 1, Separator: ','}
		i++
		if i+1 < len(args) && strings.EqualFold(args[i], "AS") {
			f.Name = args[i+1]
			i += 2
		}
		if i >= len(args) {
			return nil, errors.New("ERR Field `" + f.Identifier + "` does not have a type")
		}
		switch strings.ToUpper(args[i]) {
		case "TEXT":
			f.Type = FieldText
		case "NUMERIC":
			f.Type = FieldNumeric
		case "TAG":
			f.Type = FieldTag
		case "GEO":
			f.Type = FieldGeo
		case "VECTOR":
			f.Type = FieldVector
			// VECTOR algorithm count attr value ...
			if i+2 < len(args) {
				n, err := strconv.Atoi(args[i+2])
				if err != nil || n < 0 {
					return nil, errors.New("ERR Bad arguments for vector similarity number of parameters")
				}
				i += 2 + n
			}
		default:
			return nil, errors.New("ERR Invalid field type for field `" + f.Identifier + "`")
		}
		i++
		for i < len(args) {
			switch strings.ToUpper(args[i]) {
			case "WEIGHT":
				if i+1 < len(args) {
					if w, err := strconv.ParseFloat(args[i+1], 64); err == nil {
						f.Weight = w
					}
				}
				i += 2
				continue
			case "SEPARATOR":
				if i+1 < len(args) && args[i+1] != "" {
					f.Separator = args[i+1][0]
				}
				i += 2
				continue
			case "NOSTEM":
				f.NoStem = true
			case "SORTABLE":
				f.Sortable = true
			case "NOINDEX":
				f.NoIndex = true
			case "CASESENSITIVE":
				f.CaseSensitive = true
			case "UNF", "WITHSUFFIXTRIE", "INDEXEMPTY", "INDEXMISSING", "PHONETIC":
				if strings.EqualFold(args[i], "PHONETIC") {
					i++
				}
			default:
				goto done
			}
			i++
		}
	done:
		f.pos = len(fields)
		if json {
			p, err := store.ParseJSONPath(f.Identifier)
			if err != nil {
				return nil, errors.New("ERR Invalid JSONPath `" + f.Identifier + "`")
			}
			f.path = p
		}
		fields = append(fields, f)
	}
	return fields, nil
}

// Field returns the schema field a query names, by attribute or
// identifier.
func (idx *Index) Field(name string) *Field {
	for i := range idx.Fields {
		if idx.Fields[i].Name == name {
			return &idx.Fields[i]
		}
	}
	for i := range idx.Fields {
		if idx.Fields[i].Identifier == name {
			return &idx.Fields[i]
		}
	}
	return nil
}

// Covers reports whether key falls under the index's prefixes.
func (idx *Index) Covers(key string) bool {
	for _, p := range idx.Prefixes {
		if strings.HasPrefix(key, p) {
			return true
		}
	}
	return false
}

// IndexManager holds the indexes and aliases of a server.
type IndexManager struct {
	mu      sync.RWMutex
	indexes map[string]*Index
	aliases map[string]string
	count   atomic.Int32
}

// All returns the indexes; it is cheap when there are none.
func (m *IndexManager) All() []*Index {
	if m.count.Load() == 0 {
		return nil
	}
	m.mu.RLock()
	defer m.mu.RUnlock()
	out := make([]*Index, 0, len(m.indexes))
	for _, idx := range m.indexes {
		out = append(out, idx)
	}
	return out
}

func NewIndexManager() *IndexManager {
	return &IndexManager{indexes: make(map[string]*Index), aliases: make(map[string]string)}
}

var defaultManager = NewIndexManager()

// GetIndexManager returns the process-wide manager, for routers without
// their own.
func GetIndexManager() *IndexManager { return defaultManager }

// Create adds an index.
func (m *IndexManager) Create(idx *Index) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if _, ok := m.indexes[idx.Name]; ok {
		return ErrIndexExists
	}
	m.indexes[idx.Name] = idx
	m.count.Store(int32(len(m.indexes)))
	return nil
}

// Drop removes an index and its aliases.
func (m *IndexManager) Drop(name string) (*Index, bool) {
	m.mu.Lock()
	defer m.mu.Unlock()
	idx, ok := m.indexes[m.resolve(name)]
	if !ok {
		return nil, false
	}
	delete(m.indexes, idx.Name)
	m.count.Store(int32(len(m.indexes)))
	for a, target := range m.aliases {
		if target == idx.Name {
			delete(m.aliases, a)
		}
	}
	return idx, true
}

func (m *IndexManager) resolve(name string) string {
	if target, ok := m.aliases[name]; ok {
		return target
	}
	return name
}

// Get returns an index by name or alias.
func (m *IndexManager) Get(name string) (*Index, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	idx, ok := m.indexes[m.resolve(name)]
	return idx, ok
}

// List returns the index names, sorted.
func (m *IndexManager) List() []string {
	m.mu.RLock()
	defer m.mu.RUnlock()
	names := make([]string, 0, len(m.indexes))
	for n := range m.indexes {
		names = append(names, n)
	}
	sort.Strings(names)
	return names
}

// AddAlias points alias at index; update allows moving an existing alias.
func (m *IndexManager) AddAlias(alias, index string, update bool) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if _, ok := m.indexes[index]; !ok {
		return ErrUnknownIndex
	}
	if _, ok := m.aliases[alias]; ok && !update {
		return ErrAliasExists
	}
	m.aliases[alias] = index
	return nil
}

func (m *IndexManager) DelAlias(alias string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if _, ok := m.aliases[alias]; !ok {
		return ErrAliasMissing
	}
	delete(m.aliases, alias)
	return nil
}

// Commands returns the FT.CREATE and FT.ALIASADD commands that rebuild the
// indexes, for snapshots, the AOF and replicas.
func (m *IndexManager) Commands() [][]string {
	m.mu.RLock()
	defer m.mu.RUnlock()
	var out [][]string
	for _, n := range sortedKeys(m.indexes) {
		out = append(out, append([]string{"FT.CREATE", n}, m.indexes[n].Args...))
	}
	for _, a := range sortedKeys(m.aliases) {
		out = append(out, []string{"FT.ALIASADD", a, m.aliases[a]})
	}
	return out
}

func sortedKeys[V any](m map[string]V) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}

// Reset drops every index and alias, as FLUSHALL leaves them in Redis; it
// is for replicas that load a full copy.
func (m *IndexManager) Reset() {
	m.mu.Lock()
	m.indexes = make(map[string]*Index)
	m.aliases = make(map[string]string)
	m.count.Store(0)
	m.mu.Unlock()
}
