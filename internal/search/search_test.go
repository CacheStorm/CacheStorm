package search

import (
	"fmt"
	"runtime"
	"sort"
	"strings"
	"sync"
	"testing"

	"github.com/cachestorm/cachestorm/internal/store"
)

func hashOf(kv ...string) *store.HashValue {
	h := &store.HashValue{Fields: map[string][]byte{}}
	for i := 0; i+1 < len(kv); i += 2 {
		h.Fields[kv[i]] = []byte(kv[i+1])
	}
	return h
}

func keys(hits []Hit) string {
	var out []string
	for _, h := range hits {
		out = append(out, h.Doc.Key)
	}
	sort.Strings(out)
	return strings.Join(out, " ")
}

func TestQueriesFollowTheKeyspace(t *testing.T) {
	st := store.NewStore()
	idx, err := ParseIndex("idx", []string{"ON", "HASH", "PREFIX", "1", "doc:", "SCHEMA", "title", "TEXT", "price", "NUMERIC", "tag", "TAG"})
	if err != nil {
		t.Fatal(err)
	}
	st.Set("doc:1", hashOf("title", "hello world", "price", "10", "tag", "a,b"), store.SetOptions{})
	st.Set("doc:2", hashOf("title", "goodbye worlds", "price", "20", "tag", "B"), store.SetOptions{})
	st.Set("other", hashOf("title", "hello"), store.SetOptions{})
	run := func(q string, opts QueryOptions) string {
		t.Helper()
		n, err := ParseQuery(idx, q, opts)
		if err != nil {
			t.Fatalf("%s: %v", q, err)
		}
		return keys(idx.Search(st, n, opts))
	}
	for q, want := range map[string]string{
		"hello": "doc:1", "world": "doc:1 doc:2", "hel*": "doc:1", "-hello": "doc:2",
		`"hello world"`: "doc:1", "hello|goodbye world": "doc:1 doc:2", "the": "",
		"@price:[15 +inf]": "doc:2", "@price:[(10 20]": "doc:2", "@tag:{b}": "doc:1 doc:2",
		"@tag:{a|zz}": "doc:1", "*": "doc:1 doc:2", "@title:(hello|goodbye) @price:[0 15]": "doc:1",
	} {
		if got := run(q, QueryOptions{}); got != want {
			t.Errorf("%s = %q, want %q", q, got, want)
		}
	}
	if got := run("world", QueryOptions{Verbatim: true}); got != "doc:1" {
		t.Errorf("VERBATIM world = %q", got)
	}

	// A write the index is told about, a removal it is not, a flush.
	st.Set("doc:3", hashOf("title", "hello again"), store.SetOptions{})
	idx.Touch(st, "doc:3")
	if got := run("hello", QueryOptions{}); got != "doc:1 doc:3" {
		t.Fatalf("after a write = %q", got)
	}
	st.Delete("doc:1")
	n, _ := ParseQuery(idx, "hello", QueryOptions{})
	for _, h := range idx.Search(st, n, QueryOptions{}) {
		if h.Doc.Key == "doc:1" && idx.Fresh(st, h.Doc) != nil {
			t.Fatal("Fresh kept a deleted document")
		}
	}
	st.Flush()
	if got := run("*", QueryOptions{}); got != "" {
		t.Fatalf("after a flush = %q", got)
	}

	if _, err := ParseQuery(idx, "(", QueryOptions{}); err == nil {
		t.Fatal("an unbalanced query parsed")
	}
}

func TestSearchIsSafeAgainstConcurrentWrites(t *testing.T) {
	st := store.NewStore()
	idx, _ := ParseIndex("idx", []string{"SCHEMA", "title", "TEXT"})
	var wg sync.WaitGroup
	wg.Add(2)
	go func() {
		defer wg.Done()
		for i := 0; i < 300; i++ {
			k := fmt.Sprint("k", i%20)
			st.Set(k, hashOf("title", fmt.Sprint("word", i%7)), store.SetOptions{})
			idx.Touch(st, k)
		}
	}()
	go func() {
		defer wg.Done()
		n, _ := ParseQuery(idx, "word3|word4", QueryOptions{})
		for i := 0; i < 300; i++ {
			for _, h := range idx.Search(st, n, QueryOptions{}) {
				idx.Fresh(st, h.Doc)
			}
		}
	}()
	wg.Wait()
}

func TestAggregateExpressions(t *testing.T) {
	r := &Row{}
	r.Set("price", "20")
	r.Set("name", "Bob")
	for expr, want := range map[string]string{
		"@price*2": "40", "@price/8": "2.5", "(@price+1)%7": "0", "@price>10 && @price<30": "1",
		"upper(@name)": "BOB", "strlen(@name)": "3", "substr(@name, 1, 2)": "ob", "!exists(@missing)": "1",
	} {
		e, err := ParseExpr(expr)
		if err != nil {
			t.Fatalf("%s: %v", expr, err)
		}
		if got := e.Eval(r); got != want {
			t.Errorf("%s = %q, want %q", expr, got, want)
		}
	}
}

func cacheCount(idx *Index) int {
	idx.cacheSet.mu.Lock()
	defer idx.cacheSet.mu.Unlock()
	return len(idx.cacheSet.caches)
}

// Writes to stores no query has read must not pin those stores, and a
// store that is gone must not keep its cache.
func TestCachesDoNotPinStores(t *testing.T) {
	idx, _ := ParseIndex("idx", []string{"SCHEMA", "title", "TEXT"})
	for i := 0; i < 100; i++ {
		st := store.NewStore()
		st.Set("k", hashOf("title", "x"), store.SetOptions{})
		idx.Touch(st, "k")
	}
	if n := cacheCount(idx); n != 0 {
		t.Fatalf("Touch created %d caches", n)
	}
	func() {
		st := store.NewStore()
		idx.Documents(st)
	}()
	runtime.GC()
	idx.Documents(store.NewStore())
	if n := cacheCount(idx); n != 1 {
		t.Fatalf("%d caches after the first store was collected, want 1", n)
	}
}
