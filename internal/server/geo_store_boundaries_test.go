package server

import (
	"bytes"
	"strings"
	"testing"

	"github.com/cachestorm/cachestorm/internal/command"
	"github.com/cachestorm/cachestorm/internal/resp"
	"github.com/cachestorm/cachestorm/internal/store"
)

func TestACLGeoRadiusChecksDestination(t *testing.T) {
	a := newACLSession(t)
	if got := a.do("ACL", "SETUSER", "geo_keys_user", "on", "on", ">pw", "-@all", "+geoadd", "+georadius", "+georadiusbymember", "~user:*"); !aclIsOK(got) {
		t.Fatalf("ACL setup=%+v", got)
	}
	if got := a.do("AUTH", "geo_keys_user", "pw"); !aclIsOK(got) {
		t.Fatalf("authentication=%+v", got)
	}
	if got := a.do("GEOADD", "user:geo", "0", "0", "STORE"); got.Type != resp.TypeInteger {
		t.Fatalf("seed=%+v", got)
	}
	if got := a.do("GEORADIUS", "user:geo", "0", "0", "1", "km", "STORE", "user:dest"); got.Type != resp.TypeInteger || got.Int != 1 {
		t.Fatalf("allowed control=%+v", got)
	}
	if got := a.do("GEORADIUS", "user:geo", "0", "0", "1", "km", "STORE", "other:dest"); !aclIsNoperm(got) {
		t.Fatalf("EXPECTED: forbidden destination NOPERM | ACTUAL: %+v", got)
	}
	if got := a.do("GEORADIUSBYMEMBER", "user:geo", "STORE", "1", "km", "STORE", "user:member-dest"); got.Type != resp.TypeInteger || got.Int != 1 {
		t.Fatalf("member label was mistaken for a key: %+v", got)
	}
}

func TestGeoRadiusReadsPreserveWatchAndAOF(t *testing.T) {
	cfg := aofTxConfig(t)
	s, run := aofTxStart(t, cfg)
	run("GEOADD", "geo", "0", "0", "member")
	version := s.store.WatchKey("geo")
	defer s.store.UnwatchKey("geo")
	size := s.aof.Size()
	for _, name := range []string{"GEORADIUS", "GEORADIUS_RO", "GEORADIUSBYMEMBER", "GEORADIUSBYMEMBER_RO"} {
		args := []string{"geo", "0", "0", "1", "km"}
		if strings.Contains(name, "BYMEMBER") {
			args = []string{"geo", "member", "1", "km"}
		}
		if got := run(name, args...); !strings.HasPrefix(got, "*1\r\n") {
			t.Fatalf("%s read=%q", name, got)
		}
	}
	if s.store.GetVersion("geo") != version || s.aof.Size() != size {
		t.Fatal("radius read changed WATCH or journal")
	}
	if got := run("GEORADIUS_RO", "geo", "0", "0", "1", "km", "STORE", "dest"); !strings.HasPrefix(got, "-ERR") || s.store.Exists("dest") {
		t.Fatalf("readonly radius wrote destination: %q", got)
	}
}

func TestGeoRadiusMemberAOFFailureDoesNotAcknowledge(t *testing.T) {
	s := fullAOFServer(t)
	seed := &store.SortedSetValue{Members: map[string]float64{"member": store.GeoScore(0, 0)}}
	if err := s.store.Set("geo", seed, store.SetOptions{}); err != nil {
		t.Fatal(err)
	}
	var out bytes.Buffer
	ctx := command.NewContext("GEORADIUSBYMEMBER", [][]byte{[]byte("geo"), []byte("member"), []byte("1"), []byte("km"), []byte("STORE"), []byte("dest")}, s.store, resp.NewWriter(&out))
	if err := s.router.Execute(ctx); err != nil {
		_ = ctx.Writer.WriteError(err.Error())
	}
	if got := out.String(); !strings.HasPrefix(got, "-MISCONF ") || strings.Count(got, "\r\n") != 1 {
		t.Fatalf("EXPECTED: only MISCONF | ACTUAL: %q", got)
	}
}
