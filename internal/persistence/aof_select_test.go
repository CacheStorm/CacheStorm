package persistence

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/cachestorm/cachestorm/internal/store"
)

func startTestAOF(t *testing.T) (*AOFWriter, string) {
	t.Helper()
	dir := t.TempDir()
	w := NewAOFWriter(AOFConfig{Enabled: true, Filename: "appendonly.aof", DataDir: dir, SyncPolicy: AOFAlways})
	if err := w.Start(); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(w.Stop)
	return w, filepath.Join(dir, "appendonly.aof")
}

func loadNames(t *testing.T, path string) []string {
	t.Helper()
	cmds, err := NewAOFReader().Load(path)
	if err != nil {
		t.Fatal(err)
	}
	out := make([]string, len(cmds))
	for i, c := range cmds {
		parts := []string{c.Name}
		for _, a := range c.Args {
			parts = append(parts, string(a))
		}
		out[i] = strings.Join(parts, " ")
	}
	return out
}

func TestAOFRecordsDatabaseSwitches(t *testing.T) {
	w, path := startTestAOF(t)
	arg := func(s ...string) [][]byte {
		out := make([][]byte, len(s))
		for i, x := range s {
			out[i] = []byte(x)
		}
		return out
	}
	must := func(err error) {
		t.Helper()
		if err != nil {
			t.Fatal(err)
		}
	}
	must(w.AppendDB(0, "SET", arg("a", "0")))
	must(w.AppendDB(3, "SET", arg("b", "3")))
	must(w.AppendDB(3, "SET", arg("c", "3")))
	must(w.AppendTransaction([]Command{
		{Name: "SET", Args: arg("d", "3"), DB: 3},
		{Name: "SET", Args: arg("e", "5"), DB: 5},
	}))
	must(w.AppendDB(0, "SET", arg("f", "0")))
	if w.CurrentDB() != 0 {
		t.Fatalf("CurrentDB: %d", w.CurrentDB())
	}

	want := []string{
		"SET a 0",
		"SELECT 3", "SET b 3",
		"SET c 3",
		"MULTI", "SET d 3", "SELECT 5", "SET e 5", "EXEC",
		"SELECT 0", "SET f 0",
	}
	if got := loadNames(t, path); strings.Join(got, "|") != strings.Join(want, "|") {
		t.Fatalf("AOF:\n got %q\nwant %q", got, want)
	}
}

func TestAOFUnknownDatabaseStartsWithSelect(t *testing.T) {
	w, path := startTestAOF(t)
	w.SetCurrentDB(-1)
	if err := w.AppendDB(0, "SET", [][]byte{[]byte("a"), []byte("1")}); err != nil {
		t.Fatal(err)
	}
	if got := loadNames(t, path); len(got) != 2 || got[0] != "SELECT 0" {
		t.Fatalf("AOF: %q", got)
	}
}

type multiDBSource struct{ dbs []map[string]interface{} }

func (m multiDBSource) GetAll() map[string]interface{} { return m.dbs[0] }

func (m multiDBSource) DatabaseEntries() []map[string]interface{} { return m.dbs }

func TestAOFRewriteCoversEveryDatabase(t *testing.T) {
	entry := func(v string) interface{} { return store.NewEntry(&store.StringValue{Data: []byte(v)}) }
	src := multiDBSource{dbs: []map[string]interface{}{
		{"a": entry("0")},
		{},
		{"b": entry("2")},
	}}
	path := filepath.Join(t.TempDir(), "appendonly.aof")
	if err := NewAOFRewriter(AOFConfig{}, src).Rewrite(path); err != nil {
		t.Fatal(err)
	}
	want := "SET a 0|SELECT 2|SET b 2|SELECT 0"
	if got := strings.Join(loadNames(t, path), "|"); got != want {
		t.Fatalf("rewrite: got %q want %q", got, want)
	}
	_ = os.Remove(path)
}
