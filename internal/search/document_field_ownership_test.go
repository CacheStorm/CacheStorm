package search_test

import (
	"github.com/cachestorm/cachestorm/internal/search"
	"reflect"
	"testing"
)

type auditResult struct {
	Name      string
	Want, Got any
}

func auditMust(err error) {
	if err != nil {
		panic(err)
	}
}
func auditCheck(t *testing.T, results []auditResult) {
	t.Helper()
	for _, result := range results {
		if !reflect.DeepEqual(result.Want, result.Got) {
			t.Errorf("%s: got %v, want %v", result.Name, result.Got, result.Want)
		}
	}
}
func Search(verify bool) []auditResult {
	im := search.NewIndexManager()
	auditMust(im.CreateIndex("i", search.Schema{Fields: []search.FieldSchema{{Name: "title"}}}))
	idx, _ := im.GetIndex("i")
	doc := &search.Document{ID: "d", Fields: map[string]string{"title": "hello"}, Metadata: map[string]any{"label": "original"}}
	auditMust(idx.AddDocument(doc))
	results := []auditResult{{"control: indexed term", 1, idx.Search("hello", 10, 0).Total}}
	release, done := make(chan struct{}), make(chan struct{})
	go func() { <-release; doc.Fields["title"] = "changed"; close(done) }()
	close(release)
	<-done
	stored, _ := idx.GetDocument("d")
	results = append(results, auditResult{"input fields after gated mutation", "hello", stored.Fields["title"]})
	if verify {
		result := idx.Search("hello", 10, 0).Documents[0]
		release, done = make(chan struct{}), make(chan struct{})
		go func() { <-release; result.Fields["title"] = "resultchanged"; close(done) }()
		close(release)
		<-done
		results = append(results, auditResult{"search result detached", "hello", idx.SearchField("title", "hello", 10, 0).Documents[0].Fields["title"]})
		stored.Fields["title"] = "getterchanged"
		fieldauditResult := idx.SearchField("title", "hello", 10, 0).Documents[0]
		fieldauditResult.Fields["title"] = "fieldchanged"
		fresh, _ := idx.GetDocument("d")
		results = append(results, auditResult{"getter and field result detached", "hello", fresh.Fields["title"]})
		auditMust(idx.AddDocument(&search.Document{ID: "d", Fields: map[string]string{"title": "replacement"}}))
		results = append(results, auditResult{"replacement removes old term", 0, idx.Search("hello", 10, 0).Total}, auditResult{"replacement indexed", 1, idx.Search("replacement", 10, 0).Total})
		auditMust(idx.AddDocument(&search.Document{ID: "empty"}))
		_, exists := idx.GetDocument("missing")
		results = append(results, auditResult{"missing document", false, exists}, auditResult{"empty fields preserved", 2, idx.DocumentCount()})
	}
	return results
}

func TestAuditSearchDocumentFieldOwnership(t *testing.T) { auditCheck(t, Search(true)) }
