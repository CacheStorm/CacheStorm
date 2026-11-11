package server

import (
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
)

// SERVICE registries, DOC stores, AGG series and VECTOR stores are keys: they survive a
// restart through an AOF holding only hash and list commands, and a key of
// another type is refused with WRONGTYPE.
func TestServiceDocAggVectorAreKeys(t *testing.T) {
	cfg := aofTxConfig(t)
	s := startToggleServer(t, cfg)
	c := dialRepl(t, s)
	ok := func(args ...string) {
		t.Helper()
		if v := c.do(args...); v.Type == '-' {
			t.Fatalf("%v: %v", args, v.Err)
		}
	}
	ok("SERVICE.REGISTER", "api", "a", "10.0.0.1", "8080")
	ok("SERVICE.REGISTER", "api", "b", "10.0.0.2", "8081", "50")
	ok("SERVICE.REGISTER", "api", "c", "10.0.0.3", "8082")
	ok("SERVICE.TAGS", "api", "a", "blue", "v2")
	ok("SERVICE.WEIGHT", "api", "a", "70")
	ok("SERVICE.DEREGISTER", "api", "c")
	ok("SERVICE.REGISTER", "db", "x", "10.0.1.1", "5432")
	ok("DOC.INSERT", "users", "u1", "name", "ada", "role", "admin")
	ok("DOC.INSERT", "users", "u2", "name", "bob", "role", "user")
	ok("DOC.INSERT", "users", "u3", "name", "cy", "role", "user")
	ok("DOC.UPDATE", "users", "u2", "role", "admin")
	ok("DOC.DELETE", "users", "u3")
	ok("DOC.INDEX", "users", "role")
	ok("AGG.PUSH", "lat", "1.5", "2.5", "8")
	ok("VECTOR.CREATE", "emb", "3", "NORMALIZE")
	ok("VECTOR.ADD", "emb", "a", "3", "3", "0", "4")
	ok("VECTOR.ADD", "emb", "b", "3", "0", "1", "0")
	ok("VECTOR.ADD", "emb", "c", "3", "1", "1", "0")
	ok("VECTOR.DELETE", "emb", "c")
	ok("VECTOR.MERGE", "emb", "a", "b", "ab")
	ok("SET", "plain", "v")
	ok("HSET", "h", "f", "v")
	for _, args := range [][]string{
		{"SERVICE.REGISTER", "plain", "a", "h", "1"}, {"SERVICE.DISCOVER", "h"}, {"SERVICE.HEARTBEAT", "users", "a"},
		{"DOC.INSERT", "h", "1", "f", "v"}, {"DOC.FIND", "api"}, {"DOC.COUNT", "plain"},
		{"AGG.PUSH", "h", "1"}, {"AGG.SUM", "users"},
		{"VECTOR.ADD", "h", "x", "1", "1"}, {"VECTOR.CREATE", "users", "2"}, {"VECTOR.STATS", "api"},
	} {
		if got := c.do(args...); got.Type != '-' || !strings.HasPrefix(got.Err, "WRONGTYPE") {
			t.Fatalf("%v: %v", args, got)
		}
	}
	for _, args := range [][]string{
		{"SERVICE.REGISTER", "api", "d", "h", "99999"}, {"SERVICE.WEIGHT", "api", "a", "-1"},
		{"SERVICE.REGISTER", "api", "__service", "h", "1"}, {"AGG.PUSH", "lat", "nan"}, {"AGG.PUSH", "lat", "x"},
		{"DOC.UPDATE", "users", "nope", "a", "b"},
		{"VECTOR.ADD", "emb", "z", "2", "1", "1"}, {"VECTOR.ADD", "emb", "z", "3", "1", "inf", "1"},
		{"VECTOR.SEARCH", "emb", "1", "2", "1", "1"}, {"VECTOR.CREATE", "e2", "0"},
	} {
		if got := c.do(args...); got.Type != '-' {
			t.Fatalf("%v accepted: %v", args, got)
		}
	}
	stopReplServer(s)

	data, err := os.ReadFile(filepath.Join(cfg.Persistence.DataDir, "appendonly.aof"))
	if err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"SERVICE.", "DOC.", "AGG.", "VECTOR."} {
		if strings.Contains(string(data), name) {
			t.Fatalf("the AOF records %s commands", name)
		}
	}

	s = startToggleServer(t, cfg)
	defer stopReplServer(s)
	c = dialRepl(t, s)
	got := c.do("SERVICE.DISCOVER", "api")
	if len(got.Array) != 2 {
		t.Fatalf("SERVICE.DISCOVER after restart: %+v", got)
	}
	a := got.Array[0].Array
	if string(a[1].Bulk) != "a" || string(a[3].Bulk) != "10.0.0.1" || a[5].Int != 8080 || a[7].Int != 70 || len(a[9].Array) != 2 {
		t.Fatalf("instance a after restart: %+v", a)
	}
	if got := c.do("SERVICE.LIST"); len(got.Array) != 2 || string(got.Array[0].Bulk) != "api" || string(got.Array[1].Bulk) != "db" {
		t.Fatalf("SERVICE.LIST: %+v", got)
	}
	if got := c.do("SERVICE.HEALTHY", "api"); len(got.Array) != 2 {
		t.Fatalf("SERVICE.HEALTHY: %+v", got)
	}
	if got := c.do("DOC.FIND", "users", "role", "admin"); len(got.Array) != 2 || string(got.Array[1].Array[1].Bulk) != "u2" {
		t.Fatalf("DOC.FIND after restart: %+v", got)
	}
	if got := c.do("DOC.COUNT", "users"); got.Int != 2 {
		t.Fatalf("DOC.COUNT after restart: %v", got)
	}
	if got := c.do("DOC.DROPINDEX", "users", "role"); got.Int != 1 {
		t.Fatalf("DOC.DROPINDEX of the restored index: %v", got)
	}
	if got := c.do("AGG.SUM", "lat"); string(got.Bulk) != "12.00" {
		t.Fatalf("AGG.SUM after restart: %+v", got)
	}
	if got := c.do("AGG.MAX", "lat"); string(got.Bulk) != "8.00" {
		t.Fatalf("AGG.MAX after restart: %+v", got)
	}
	if got := c.do("VECTOR.GET", "emb", "a"); len(got.Array) != 3 || string(got.Array[0].Bulk) != "0.600000" || string(got.Array[2].Bulk) != "0.800000" {
		t.Fatalf("VECTOR.GET after restart: %+v", got)
	}
	if got := c.do("VECTOR.SEARCH", "emb", "2", "3", "0", "2", "0"); len(got.Array) != 2 || string(got.Array[0].Array[1].Bulk) != "b" || string(got.Array[1].Array[1].Bulk) != "ab" {
		t.Fatalf("VECTOR.SEARCH after restart: %+v", got)
	}
	if got := c.do("VECTOR.STATS", "emb"); len(got.Array) != 8 || got.Array[5].Int != 3 || string(got.Array[7].Bulk) != "true" {
		t.Fatalf("VECTOR.STATS after restart: %+v", got)
	}
	if got := c.do("TYPE", "lat"); got.Str != "list" {
		t.Fatalf("TYPE lat: %v", got)
	}
	for _, args := range [][]string{{"SERVICE.DEREGISTER", "api", "a"}, {"SERVICE.DEREGISTER", "api", "b"}, {"SERVICE.DEREGISTER", "db", "x"}, {"DOC.DELETE", "users", "u1"}, {"DOC.DELETE", "users", "u2"}} {
		if got := c.do(args...); got.Int != 1 {
			t.Fatalf("%v: %v", args, got)
		}
	}
	ok = func(args ...string) {
		t.Helper()
		if v := c.do(args...); v.Type == '-' {
			t.Fatalf("%v: %v", args, v.Err)
		}
	}
	ok("AGG.CLEAR", "lat")
	ok("DEL", "emb")
	if got := c.do("DBSIZE"); got.Int != 2 {
		t.Fatalf("DBSIZE once every registry, store and series is emptied: %v", got)
	}
}

// Concurrent updates of different documents of one store all land: each is
// a read-modify-write of the store's key, serialized by the write gate.
func TestDocConcurrentUpdatesAllLand(t *testing.T) {
	cfg := aofTxConfig(t)
	s := startToggleServer(t, cfg)
	defer stopReplServer(s)
	c := dialRepl(t, s)
	c.do("DOC.INSERT", "d", "x", "n", "0")
	var wg sync.WaitGroup
	for w := range 8 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			cw := dialRepl(t, s)
			for i := range 25 {
				cw.do("DOC.UPDATE", "d", "x", "w"+string(rune('a'+w))+string(rune('a'+i)), "1")
			}
		}()
	}
	wg.Wait()
	got := c.do("DOC.FINDONE", "d", "n", "0")
	if want := 2 + 2*(1+8*25); len(got.Array) != want {
		t.Fatalf("the document holds %d reply items, want %d", len(got.Array), want)
	}
}
