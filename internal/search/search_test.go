package search

import (
	"testing"
)

func TestNewIndexManager(t *testing.T) {
	im := NewIndexManager()
	if im == nil {
		t.Fatal("expected index manager")
	}
	if im.indexes == nil {
		t.Error("indexes map should be initialized")
	}
}

func TestGetIndexManager(t *testing.T) {
	im1 := GetIndexManager()
	im2 := GetIndexManager()
	if im1 != im2 {
		t.Error("expected same global index manager instance")
	}
}

func TestCreateIndex(t *testing.T) {
	im := NewIndexManager()

	schema := Schema{
		Fields: []FieldSchema{
			{Name: "title", Type: "text"},
			{Name: "body", Type: "text"},
		},
	}

	err := im.CreateIndex("test_idx", schema)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	idx, ok := im.GetIndex("test_idx")
	if !ok {
		t.Fatal("expected to find index")
	}
	if idx.Name != "test_idx" {
		t.Errorf("expected name 'test_idx', got '%s'", idx.Name)
	}
	if len(idx.Schema.Fields) != 2 {
		t.Errorf("expected 2 fields, got %d", len(idx.Schema.Fields))
	}
}

func TestCreateIndexDuplicate(t *testing.T) {
	im := NewIndexManager()
	schema := Schema{Fields: []FieldSchema{{Name: "title", Type: "text"}}}

	im.CreateIndex("test_idx", schema)
	err := im.CreateIndex("test_idx", schema)
	if err != nil {
		t.Errorf("duplicate create should return nil error, got %v", err)
	}
}

func TestDropIndex(t *testing.T) {
	im := NewIndexManager()
	schema := Schema{Fields: []FieldSchema{{Name: "title", Type: "text"}}}
	im.CreateIndex("test_idx", schema)

	if !im.DropIndex("test_idx") {
		t.Error("expected drop to return true")
	}

	_, ok := im.GetIndex("test_idx")
	if ok {
		t.Error("should not find dropped index")
	}

	if im.DropIndex("nonexistent") {
		t.Error("expected drop of nonexistent index to return false")
	}
}

func TestListIndexes(t *testing.T) {
	im := NewIndexManager()
	schema := Schema{Fields: []FieldSchema{{Name: "title", Type: "text"}}}

	im.CreateIndex("idx_a", schema)
	im.CreateIndex("idx_b", schema)

	list := im.ListIndexes()
	if len(list) != 2 {
		t.Errorf("expected 2 indexes, got %d", len(list))
	}
}

func TestAddDocument(t *testing.T) {
	im := NewIndexManager()
	schema := Schema{
		Fields: []FieldSchema{
			{Name: "title", Type: "text"},
			{Name: "body", Type: "text"},
		},
	}
	im.CreateIndex("test_idx", schema)
	idx, _ := im.GetIndex("test_idx")

	doc := &Document{
		ID:     "doc1",
		Fields: map[string]string{"title": "Hello World", "body": "This is a test"},
	}

	err := idx.AddDocument(doc)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if idx.DocumentCount() != 1 {
		t.Errorf("expected 1 document, got %d", idx.DocumentCount())
	}
}

func TestAddDocumentNoIndex(t *testing.T) {
	im := NewIndexManager()
	schema := Schema{
		Fields: []FieldSchema{
			{Name: "title", Type: "text"},
			{Name: "secret", Type: "text", NoIndex: true},
		},
	}
	im.CreateIndex("test_idx", schema)
	idx, _ := im.GetIndex("test_idx")

	doc := &Document{
		ID:     "doc1",
		Fields: map[string]string{"title": "Hello", "secret": "hidden value"},
	}

	idx.AddDocument(doc)

	if len(idx.Inverted["hidden"]) > 0 {
		t.Error("secret field should not be indexed")
	}
}

func TestGetDocument(t *testing.T) {
	im := NewIndexManager()
	schema := Schema{Fields: []FieldSchema{{Name: "title", Type: "text"}}}
	im.CreateIndex("test_idx", schema)
	idx, _ := im.GetIndex("test_idx")

	doc := &Document{ID: "doc1", Fields: map[string]string{"title": "Hello"}}
	idx.AddDocument(doc)

	retrieved, ok := idx.GetDocument("doc1")
	if !ok {
		t.Fatal("expected to find document")
	}
	if retrieved.ID != "doc1" {
		t.Errorf("expected ID 'doc1', got '%s'", retrieved.ID)
	}

	_, ok = idx.GetDocument("nonexistent")
	if ok {
		t.Error("should not find nonexistent document")
	}
}

func TestDeleteDocument(t *testing.T) {
	im := NewIndexManager()
	schema := Schema{Fields: []FieldSchema{{Name: "title", Type: "text"}}}
	im.CreateIndex("test_idx", schema)
	idx, _ := im.GetIndex("test_idx")

	doc := &Document{ID: "doc1", Fields: map[string]string{"title": "Hello World"}}
	idx.AddDocument(doc)

	if !idx.DeleteDocument("doc1") {
		t.Error("expected delete to return true")
	}

	if idx.DocumentCount() != 0 {
		t.Errorf("expected 0 documents, got %d", idx.DocumentCount())
	}

	if idx.DeleteDocument("nonexistent") {
		t.Error("expected delete of nonexistent document to return false")
	}
}

func TestSearch(t *testing.T) {
	im := NewIndexManager()
	schema := Schema{Fields: []FieldSchema{{Name: "title", Type: "text"}}}
	im.CreateIndex("test_idx", schema)
	idx, _ := im.GetIndex("test_idx")

	idx.AddDocument(&Document{ID: "doc1", Fields: map[string]string{"title": "hello world"}})
	idx.AddDocument(&Document{ID: "doc2", Fields: map[string]string{"title": "hello there"}})
	idx.AddDocument(&Document{ID: "doc3", Fields: map[string]string{"title": "goodbye world"}})

	result := idx.Search("hello", 10, 0)
	if result.Total != 2 {
		t.Errorf("expected total 2, got %d", result.Total)
	}
	if len(result.Documents) != 2 {
		t.Errorf("expected 2 documents, got %d", len(result.Documents))
	}
}

func TestSearchWithLimit(t *testing.T) {
	im := NewIndexManager()
	schema := Schema{Fields: []FieldSchema{{Name: "title", Type: "text"}}}
	im.CreateIndex("test_idx", schema)
	idx, _ := im.GetIndex("test_idx")

	idx.AddDocument(&Document{ID: "doc1", Fields: map[string]string{"title": "hello"}})
	idx.AddDocument(&Document{ID: "doc2", Fields: map[string]string{"title": "hello"}})
	idx.AddDocument(&Document{ID: "doc3", Fields: map[string]string{"title": "hello"}})

	result := idx.Search("hello", 2, 0)
	if result.Total != 3 {
		t.Errorf("expected total 3, got %d", result.Total)
	}
	if len(result.Documents) != 2 {
		t.Errorf("expected 2 documents with limit, got %d", len(result.Documents))
	}
}

func TestSearchWithOffset(t *testing.T) {
	im := NewIndexManager()
	schema := Schema{Fields: []FieldSchema{{Name: "title", Type: "text"}}}
	im.CreateIndex("test_idx", schema)
	idx, _ := im.GetIndex("test_idx")

	idx.AddDocument(&Document{ID: "doc1", Fields: map[string]string{"title": "hello"}})
	idx.AddDocument(&Document{ID: "doc2", Fields: map[string]string{"title": "hello"}})
	idx.AddDocument(&Document{ID: "doc3", Fields: map[string]string{"title": "hello"}})

	result := idx.Search("hello", 2, 1)
	if result.Total != 3 {
		t.Errorf("expected total 3, got %d", result.Total)
	}
	if len(result.Documents) != 2 {
		t.Errorf("expected 2 documents with offset+limit, got %d", len(result.Documents))
	}
}

func TestSearchOffsetTooLarge(t *testing.T) {
	im := NewIndexManager()
	schema := Schema{Fields: []FieldSchema{{Name: "title", Type: "text"}}}
	im.CreateIndex("test_idx", schema)
	idx, _ := im.GetIndex("test_idx")

	idx.AddDocument(&Document{ID: "doc1", Fields: map[string]string{"title": "hello"}})

	result := idx.Search("hello", 10, 100)
	if result.Total != 1 {
		t.Errorf("expected total 1, got %d", result.Total)
	}
	if len(result.Documents) != 0 {
		t.Errorf("expected 0 documents with large offset, got %d", len(result.Documents))
	}
}

func TestSearchNoResults(t *testing.T) {
	im := NewIndexManager()
	schema := Schema{Fields: []FieldSchema{{Name: "title", Type: "text"}}}
	im.CreateIndex("test_idx", schema)
	idx, _ := im.GetIndex("test_idx")

	idx.AddDocument(&Document{ID: "doc1", Fields: map[string]string{"title": "hello"}})

	result := idx.Search("nonexistent", 10, 0)
	if result.Total != 0 {
		t.Errorf("expected total 0, got %d", result.Total)
	}
}

func TestSearchField(t *testing.T) {
	im := NewIndexManager()
	schema := Schema{
		Fields: []FieldSchema{
			{Name: "title", Type: "text"},
			{Name: "body", Type: "text"},
		},
	}
	im.CreateIndex("test_idx", schema)
	idx, _ := im.GetIndex("test_idx")

	idx.AddDocument(&Document{ID: "doc1", Fields: map[string]string{
		"title": "hello",
		"body":  "world",
	}})
	idx.AddDocument(&Document{ID: "doc2", Fields: map[string]string{
		"title": "goodbye",
		"body":  "hello",
	}})

	result := idx.SearchField("title", "hello", 10, 0)
	if result.Total != 1 {
		t.Errorf("expected total 1, got %d", result.Total)
	}
	if len(result.Documents) != 1 {
		t.Errorf("expected 1 document, got %d", len(result.Documents))
	}
}

func TestDocumentCount(t *testing.T) {
	im := NewIndexManager()
	schema := Schema{Fields: []FieldSchema{{Name: "title", Type: "text"}}}
	im.CreateIndex("test_idx", schema)
	idx, _ := im.GetIndex("test_idx")

	if idx.DocumentCount() != 0 {
		t.Errorf("expected 0 documents, got %d", idx.DocumentCount())
	}

	idx.AddDocument(&Document{ID: "doc1", Fields: map[string]string{"title": "hello"}})
	if idx.DocumentCount() != 1 {
		t.Errorf("expected 1 document, got %d", idx.DocumentCount())
	}
}

func TestTokenize(t *testing.T) {
	im := NewIndexManager()
	schema := Schema{Fields: []FieldSchema{{Name: "title", Type: "text"}}}
	im.CreateIndex("test_idx", schema)
	idx, _ := im.GetIndex("test_idx")

	tokens := idx.tokenize("Hello World! This is a TEST.")
	expectedTokens := []string{"hello", "world", "this", "is", "test"}

	if len(tokens) != len(expectedTokens) {
		t.Errorf("expected %d tokens, got %d", len(expectedTokens), len(tokens))
		return
	}

	for i, token := range tokens {
		if token != expectedTokens[i] {
			t.Errorf("expected token '%s', got '%s'", expectedTokens[i], token)
		}
	}
}

func TestTokenizeSingleChars(t *testing.T) {
	im := NewIndexManager()
	schema := Schema{Fields: []FieldSchema{{Name: "title", Type: "text"}}}
	im.CreateIndex("test_idx", schema)
	idx, _ := im.GetIndex("test_idx")

	tokens := idx.tokenize("a b c d")
	for _, token := range tokens {
		if len(token) <= 1 {
			t.Errorf("single char tokens should be filtered, got '%s'", token)
		}
	}
}

func TestIndexInfo(t *testing.T) {
	im := NewIndexManager()
	schema := Schema{
		Fields: []FieldSchema{
			{Name: "title", Type: "text"},
			{Name: "body", Type: "text"},
		},
	}
	im.CreateIndex("test_idx", schema)
	idx, _ := im.GetIndex("test_idx")

	idx.AddDocument(&Document{ID: "doc1", Fields: map[string]string{
		"title": "hello world",
		"body":  "test content",
	}})

	info := idx.Info()
	if info["name"] != "test_idx" {
		t.Errorf("expected name 'test_idx', got '%v'", info["name"])
	}
	if info["document_count"] != 1 {
		t.Errorf("expected 1 document, got %v", info["document_count"])
	}
	if info["field_count"] != 2 {
		t.Errorf("expected 2 fields, got %v", info["field_count"])
	}
}

func TestGetFieldSchema(t *testing.T) {
	im := NewIndexManager()
	schema := Schema{
		Fields: []FieldSchema{
			{Name: "title", Type: "text", Sortable: true},
			{Name: "body", Type: "text"},
		},
	}
	im.CreateIndex("test_idx", schema)
	idx, _ := im.GetIndex("test_idx")

	fs := idx.getFieldSchema("title")
	if fs == nil {
		t.Fatal("expected to find field schema")
	}
	if fs.Name != "title" {
		t.Errorf("expected name 'title', got '%s'", fs.Name)
	}
	if !fs.Sortable {
		t.Error("expected Sortable to be true")
	}

	fs = idx.getFieldSchema("nonexistent")
	if fs != nil {
		t.Error("expected nil for nonexistent field")
	}
}
