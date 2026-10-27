package search

import (
	"errors"
	"maps"
	"regexp"
	"sort"
	"strings"
	"sync"
)

type Document struct {
	ID       string
	Fields   map[string]string
	Score    float64
	Metadata map[string]interface{}
}

type Index struct {
	Name       string
	Schema     Schema
	Inverted   map[string]map[string][]int
	Documents  map[string]*Document
	FieldIndex map[string]map[string][]string
	mu         sync.RWMutex
}

type Schema struct {
	Fields []FieldSchema
}

type FieldSchema struct {
	Name     string
	Type     string
	Sortable bool
	NoIndex  bool
}

type SearchResult struct {
	Total     int
	Documents []*Document
}

type IndexManager struct {
	mu      sync.RWMutex
	indexes map[string]*Index
	// aliases maps an alias to the index it names (FT.ALIASADD).
	aliases map[string]string
}

var globalIndexManager = NewIndexManager()

func NewIndexManager() *IndexManager {
	return &IndexManager{
		indexes: make(map[string]*Index),
		aliases: make(map[string]string),
	}
}

func GetIndexManager() *IndexManager {
	return globalIndexManager
}

func (m *IndexManager) CreateIndex(name string, schema Schema) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if _, exists := m.indexes[name]; exists {
		return nil
	}

	m.indexes[name] = &Index{
		Name:       name,
		Schema:     schema,
		Inverted:   make(map[string]map[string][]int),
		Documents:  make(map[string]*Document),
		FieldIndex: make(map[string]map[string][]string),
	}

	for _, field := range schema.Fields {
		m.indexes[name].FieldIndex[field.Name] = make(map[string][]string)
	}

	return nil
}

func (m *IndexManager) DropIndex(name string) bool {
	m.mu.Lock()
	defer m.mu.Unlock()

	if _, exists := m.indexes[name]; !exists {
		return false
	}

	delete(m.indexes, name)
	for alias, target := range m.aliases {
		if target == name {
			delete(m.aliases, alias)
		}
	}
	return true
}

// The alias errors carry RediSearch's reply texts.
//
//nolint:staticcheck // RediSearch's error texts are capitalised
var (
	ErrAliasExists  = errors.New("Alias already exists")
	ErrAliasMissing = errors.New("Alias does not exist")
	ErrNoIndex      = errors.New("Unknown index name")
)

// AddAlias makes alias another name for index.
func (m *IndexManager) AddAlias(alias, index string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if _, ok := m.aliases[alias]; ok {
		return ErrAliasExists
	}
	if _, ok := m.indexes[index]; !ok {
		return ErrNoIndex
	}
	m.aliases[alias] = index
	return nil
}

// UpdateAlias points alias at index, adding it when missing.
func (m *IndexManager) UpdateAlias(alias, index string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if _, ok := m.indexes[index]; !ok {
		return ErrNoIndex
	}
	m.aliases[alias] = index
	return nil
}

// DelAlias removes alias.
func (m *IndexManager) DelAlias(alias string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if _, ok := m.aliases[alias]; !ok {
		return ErrAliasMissing
	}
	delete(m.aliases, alias)
	return nil
}

func (m *IndexManager) GetIndex(name string) (*Index, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	idx, ok := m.indexes[name]
	if !ok {
		if target, aliased := m.aliases[name]; aliased {
			idx, ok = m.indexes[target]
		}
	}
	return idx, ok
}

// TagValues returns the distinct values of a tag field across the index's
// documents: each value split on commas, trimmed and lower-cased, sorted.
func (idx *Index) TagValues(field string) []string {
	idx.mu.RLock()
	defer idx.mu.RUnlock()
	seen := make(map[string]struct{})
	for _, doc := range idx.Documents {
		v, ok := doc.Fields[field]
		if !ok {
			continue
		}
		for _, tag := range strings.Split(v, ",") {
			if tag = strings.ToLower(strings.TrimSpace(tag)); tag != "" {
				seen[tag] = struct{}{}
			}
		}
	}
	out := make([]string, 0, len(seen))
	for t := range seen {
		out = append(out, t)
	}
	sort.Strings(out)
	return out
}

func (m *IndexManager) ListIndexes() []string {
	m.mu.RLock()
	defer m.mu.RUnlock()

	names := make([]string, 0, len(m.indexes))
	for name := range m.indexes {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}

func (idx *Index) AddDocument(doc *Document) error {
	idx.mu.Lock()
	defer idx.mu.Unlock()
	doc = copyDocumentFields(doc)

	// Reconcile a document that is already indexed. Re-adding (including
	// FT.ADD ... REPLACE) used to overwrite idx.Documents[doc.ID] and then
	// APPEND fresh postings, leaving the previous revision's postings behind:
	// idx.Inverted is keyed by docID, so the old term stayed mapped to this
	// document and FT.SEARCH kept returning it as a match for content the
	// document no longer held. Remove the old postings first, exactly as
	// DeleteDocument does.
	if prev, exists := idx.Documents[doc.ID]; exists {
		idx.removePostings(prev, doc.ID)
	}

	idx.Documents[doc.ID] = doc

	for fieldName, fieldValue := range doc.Fields {
		fieldSchema := idx.getFieldSchema(fieldName)
		if fieldSchema != nil && fieldSchema.NoIndex {
			continue
		}

		tokens := idx.tokenize(fieldValue)
		for _, token := range tokens {
			if idx.FieldIndex[fieldName] == nil {
				idx.FieldIndex[fieldName] = make(map[string][]string)
			}
			idx.FieldIndex[fieldName][token] = append(idx.FieldIndex[fieldName][token], doc.ID)

			if idx.Inverted[token] == nil {
				idx.Inverted[token] = make(map[string][]int)
			}
			if idx.Inverted[token][doc.ID] == nil {
				idx.Inverted[token][doc.ID] = []int{}
			}
		}
	}

	return nil
}

// removePostings strips every index entry that references docID. The caller
// must hold idx.mu.
func (idx *Index) removePostings(doc *Document, docID string) {
	for fieldName, fieldValue := range doc.Fields {
		tokens := idx.tokenize(fieldValue)
		for _, token := range tokens {
			if idx.FieldIndex[fieldName] != nil {
				docs := idx.FieldIndex[fieldName][token]
				newDocs := make([]string, 0)
				for _, d := range docs {
					if d != docID {
						newDocs = append(newDocs, d)
					}
				}
				idx.FieldIndex[fieldName][token] = newDocs
			}

			if idx.Inverted[token] != nil {
				delete(idx.Inverted[token], docID)
			}
		}
	}
}

func (idx *Index) DeleteDocument(docID string) bool {
	idx.mu.Lock()
	defer idx.mu.Unlock()

	doc, exists := idx.Documents[docID]
	if !exists {
		return false
	}

	idx.removePostings(doc, docID)
	delete(idx.Documents, docID)
	return true
}

func (idx *Index) Search(query string, limit, offset int) *SearchResult {
	idx.mu.RLock()
	defer idx.mu.RUnlock()

	queryTokens := idx.tokenize(query)
	docScores := make(map[string]float64)

	for _, token := range queryTokens {
		if idx.Inverted[token] != nil {
			for docID := range idx.Inverted[token] {
				docScores[docID]++
			}
		}
	}

	type scoredDoc struct {
		id    string
		score float64
	}

	scored := make([]scoredDoc, 0, len(docScores))
	for docID, score := range docScores {
		scored = append(scored, scoredDoc{id: docID, score: score})
	}

	sort.Slice(scored, func(i, j int) bool {
		return scored[i].score > scored[j].score
	})

	total := len(scored)
	// Clamp a negative window. cmdFTSEARCH rejects one up front, but this guard
	// keeps the engine total for every caller: the `offset >= len(scored)` test
	// below is FALSE for a negative offset, so the result loop would index
	// scored[-1] and panic.
	if offset < 0 {
		offset = 0
	}
	if limit < 0 {
		limit = total
	}
	if offset >= len(scored) {
		return &SearchResult{Total: total, Documents: []*Document{}}
	}

	end := len(scored)
	if limit < end-offset {
		end = offset + limit
	}

	docs := make([]*Document, 0, end-offset)
	for i := offset; i < end; i++ {
		if doc, ok := idx.Documents[scored[i].id]; ok {
			resultDoc := copyDocumentFields(doc)
			resultDoc.Score = scored[i].score
			docs = append(docs, resultDoc)
		}
	}

	return &SearchResult{Total: total, Documents: docs}
}

func (idx *Index) SearchField(fieldName, value string, limit, offset int) *SearchResult {
	idx.mu.RLock()
	defer idx.mu.RUnlock()

	tokens := idx.tokenize(value)
	docScores := make(map[string]float64)

	for _, token := range tokens {
		if idx.FieldIndex[fieldName] != nil && idx.FieldIndex[fieldName][token] != nil {
			for _, docID := range idx.FieldIndex[fieldName][token] {
				docScores[docID]++
			}
		}
	}

	type scoredDoc struct {
		id    string
		score float64
	}

	scored := make([]scoredDoc, 0, len(docScores))
	for docID, score := range docScores {
		scored = append(scored, scoredDoc{id: docID, score: score})
	}

	sort.Slice(scored, func(i, j int) bool {
		return scored[i].score > scored[j].score
	})

	total := len(scored)
	// Clamp a negative window. cmdFTSEARCH rejects one up front, but this guard
	// keeps the engine total for every caller: the `offset >= len(scored)` test
	// below is FALSE for a negative offset, so the result loop would index
	// scored[-1] and panic.
	if offset < 0 {
		offset = 0
	}
	if limit < 0 {
		limit = total
	}
	if offset >= len(scored) {
		return &SearchResult{Total: total, Documents: []*Document{}}
	}

	end := len(scored)
	if limit < end-offset {
		end = offset + limit
	}

	docs := make([]*Document, 0, end-offset)
	for i := offset; i < end; i++ {
		if doc, ok := idx.Documents[scored[i].id]; ok {
			resultDoc := copyDocumentFields(doc)
			resultDoc.Score = scored[i].score
			docs = append(docs, resultDoc)
		}
	}

	return &SearchResult{Total: total, Documents: docs}
}

func (idx *Index) GetDocument(docID string) (*Document, bool) {
	idx.mu.RLock()
	defer idx.mu.RUnlock()
	doc, ok := idx.Documents[docID]
	if !ok {
		return nil, false
	}
	return copyDocumentFields(doc), true
}

func copyDocumentFields(doc *Document) *Document {
	result := *doc
	result.Fields = maps.Clone(doc.Fields)
	return &result
}

func (idx *Index) DocumentCount() int {
	idx.mu.RLock()
	defer idx.mu.RUnlock()
	return len(idx.Documents)
}

func (idx *Index) tokenize(text string) []string {
	text = strings.ToLower(text)

	reg := regexp.MustCompile(`[^a-z0-9\s]`)
	text = reg.ReplaceAllString(text, " ")

	words := strings.Fields(text)

	tokens := make([]string, 0, len(words))
	for _, word := range words {
		if len(word) > 1 {
			tokens = append(tokens, word)
		}
	}

	return tokens
}

func (idx *Index) getFieldSchema(name string) *FieldSchema {
	for i := range idx.Schema.Fields {
		if idx.Schema.Fields[i].Name == name {
			return &idx.Schema.Fields[i]
		}
	}
	return nil
}

func (idx *Index) Info() map[string]interface{} {
	idx.mu.RLock()
	defer idx.mu.RUnlock()

	indexSize := 0
	for _, fieldIndex := range idx.FieldIndex {
		indexSize += len(fieldIndex)
	}

	return map[string]interface{}{
		"name":           idx.Name,
		"document_count": len(idx.Documents),
		"index_size":     indexSize,
		"field_count":    len(idx.Schema.Fields),
	}
}
