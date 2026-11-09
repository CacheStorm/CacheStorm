package persistence

import (
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

func TestReadRedisAOFManifest(t *testing.T) {
	dir := t.TempDir()
	write := func(text string) {
		t.Helper()
		if err := os.WriteFile(filepath.Join(dir, "appendonly.aof.manifest"), []byte(text), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := ReadRedisAOFManifest(dir, "appendonly.aof"); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("no manifest: %v", err)
	}
	write("# comment\nfile a.base.rdb seq 3 type b\nfile old.incr.aof seq 1 type h\n" +
		"file 'b c.incr.aof' seq 3 type i\nfile \"d\\x41.incr.aof\" seq 4 type i startoffset 0\n")
	got, err := ReadRedisAOFManifest(dir, "appendonly.aof")
	if err != nil {
		t.Fatal(err)
	}
	want := &RedisAOF{Base: filepath.Join(dir, "a.base.rdb"), Incr: []string{filepath.Join(dir, "b c.incr.aof"), filepath.Join(dir, "dA.incr.aof")}}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("got %+v, want %+v", got, want)
	}
	for _, bad := range []string{
		"file a seq 1 type b\nfile b seq 2 type b\n",
		"file ../x seq 1 type b\n",
		"file a seq x type b\n",
		"file a seq 1 type z\n",
		"file \"a seq 1 type b\n",
		"file a seq 1\n",
		"# only a comment\n",
	} {
		write(bad)
		if _, err := ReadRedisAOFManifest(dir, "appendonly.aof"); err == nil {
			t.Errorf("manifest %q accepted", bad)
		}
	}
}
