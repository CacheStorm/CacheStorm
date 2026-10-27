package search

import (
	"reflect"
	"testing"
)

func TestCommandsOrderIndexesDocumentsAliases(t *testing.T) {
	m := NewIndexManager()
	if err := m.CreateIndex("b", Schema{Fields: []FieldSchema{{Name: "t", Type: "TEXT", Sortable: true}, {Name: "n", Type: "TAG", NoIndex: true}}}); err != nil {
		t.Fatal(err)
	}
	if err := m.CreateIndex("a", Schema{}); err != nil {
		t.Fatal(err)
	}
	idx, _ := m.GetIndex("b")
	if err := idx.AddDocument(&Document{ID: "d", Score: 0.5, Fields: map[string]string{"t": "x", "n": "y"}}); err != nil {
		t.Fatal(err)
	}
	if err := m.AddAlias("z", "b"); err != nil {
		t.Fatal(err)
	}
	want := [][]string{
		{"FT.CREATE", "a"},
		{"FT.CREATE", "b", "SCHEMA", "t", "TEXT", "SORTABLE", "n", "TAG", "NOINDEX"},
		{"FT.ADD", "b", "d", "0.5", "REPLACE", "FIELDS", "n", "y", "t", "x"},
		{"FT.ALIASADD", "z", "b"},
	}
	if got := m.Commands(); !reflect.DeepEqual(got, want) {
		t.Fatalf("Commands() = %q, want %q", got, want)
	}
	if got := NewIndexManager().Commands(); len(got) != 0 {
		t.Fatalf("empty manager produced %q", got)
	}
}
