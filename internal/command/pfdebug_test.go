package command

import (
	"strings"
	"testing"

	"github.com/cachestorm/cachestorm/internal/store"
)

func TestPFDebugMatchesRedis(t *testing.T) {
	s := store.NewStore()
	r := NewRouter()
	RegisterHyperLogLogCommands(r)
	RegisterStringCommands(r)
	runCmd(t, s, r, "PFADD", "h", "a", "b", "c", "d", "e")
	if got := runCmd(t, s, r, "PFDEBUG", "ENCODING", "h"); got != "+sparse\r\n" {
		t.Fatalf("ENCODING = %q", got)
	}
	if got := runCmd(t, s, r, "PFDEBUG", "DECODE", "h"); got != "+Z:7292 v:1,1 Z:1143 v:1,1 Z:4274 v:2,1 Z:2445 v:1,1 Z:622 v:1,1 Z:603\r\n" {
		t.Fatalf("DECODE = %q", got)
	}
	if got := runCmd(t, s, r, "PFDEBUG", "GETREG", "h"); !strings.HasPrefix(got, "*16384\r\n") {
		t.Fatalf("GETREG = %.40q", got)
	}
	if got := runCmd(t, s, r, "PFDEBUG", "TODENSE", "h"); got != ":0\r\n" {
		t.Fatalf("TODENSE after GETREG = %q, want 0 since GETREG converted it", got)
	}
	if got := runCmd(t, s, r, "PFDEBUG", "DECODE", "h"); got != "-ERR HLL encoding is not sparse\r\n" {
		t.Fatalf("DECODE of dense = %q", got)
	}
	if got := runCmd(t, s, r, "PFCOUNT", "h"); got != ":5\r\n" {
		t.Fatalf("PFCOUNT after the conversion = %q", got)
	}
	runCmd(t, s, r, "PFADD", "h2", "x")
	if got := runCmd(t, s, r, "PFDEBUG", "TODENSE", "h2"); got != ":1\r\n" {
		t.Fatalf("TODENSE = %q", got)
	}
	if got := runCmd(t, s, r, "PFDEBUG", "FOO", "h"); got != "-ERR Unknown PFDEBUG subcommand 'FOO'\r\n" {
		t.Fatalf("unknown subcommand = %q", got)
	}
	if got := runCmd(t, s, r, "PFDEBUG", "ENCODING", "nokey"); got != "-ERR The specified key does not exist\r\n" {
		t.Fatalf("missing key = %q", got)
	}
	runCmd(t, s, r, "SET", "str", "x")
	if got := runCmd(t, s, r, "PFDEBUG", "ENCODING", "str"); !strings.HasPrefix(got, "-WRONGTYPE") {
		t.Fatalf("non-HLL = %q", got)
	}
	if got := runCmd(t, s, r, "PFSELFTEST"); got != "+OK\r\n" {
		t.Fatalf("PFSELFTEST = %q", got)
	}
}
