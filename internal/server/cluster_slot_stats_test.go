package server

import (
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/cachestorm/cachestorm/internal/config"
)

// reqLen is the size of a command sent as a RESP array.
func reqLen(args ...string) int64 {
	n := len(strconv.Itoa(len(args))) + 3
	for _, a := range args {
		n += len(strconv.Itoa(len(a))) + 3 + len(a) + 2
	}
	return int64(n)
}

func TestClusterSlotStats(t *testing.T) {
	s, c := startClusterServer(t, t.TempDir(), func(cfg *config.Config) { cfg.Cluster.RequireFullCoverage = false })
	for _, r := range [][2]string{{"0", "100"}, {"200", "300"}} {
		if got := c.str("CLUSTER", "ADDSLOTSRANGE", r[0], r[1]); got != "OK" {
			t.Fatalf("ADDSLOTSRANGE: %s", got)
		}
	}
	c.str("CONFIG", "SET", "cluster-slot-stats-enabled", "no")
	k := keyInSlot(t, "k", 5)
	if got := c.str("SET", k, "abc"); got != "OK" {
		t.Fatalf("SET: %s", got)
	}

	for _, tc := range [][2]string{
		{"[[5 [key-count 1]]]", "SLOTSRANGE 5 5"},
		{"[[99 [key-count 0]] [100 [key-count 0]] [200 [key-count 0]] [201 [key-count 0]]]", "SLOTSRANGE 99 201"},
		{"[]", "SLOTSRANGE 150 160"},
		{"[[5 [key-count 1]] [0 [key-count 0]]]", "ORDERBY key-count LIMIT 2"},
		{"[[0 [key-count 0]]]", "ORDERBY KEY-COUNT asc limit 1"},
		{"ERR:ERR Unrecognized sort metric for ORDERBY.", "ORDERBY cpu-usec"},
		{"ERR:ERR Limit has to lie in between 1 and 16384 (maximum number of slots).", "ORDERBY key-count LIMIT 0"},
		{"ERR:ERR value is not an integer or out of range", "ORDERBY key-count LIMIT x"},
		{"ERR:ERR syntax error", "ORDERBY key-count LIMIT"},
		{"ERR:ERR Multiple filters of the same type are disallowed.", "ORDERBY key-count ASC DESC"},
		{"ERR:ERR Multiple filters of the same type are disallowed.", "ORDERBY key-count LIMIT 1 LIMIT 2"},
		{"ERR:ERR syntax error", "ORDERBY key-count foo"},
		{"ERR:ERR Start slot number 9 is greater than end slot number 3", "SLOTSRANGE 9 3"},
		{"ERR:ERR Invalid or out of range slot", "SLOTSRANGE 0 16384"},
		{"ERR:ERR unknown subcommand or wrong number of arguments for 'SLOT-STATS'. Try CLUSTER HELP.", "SLOTSRANGE 1 2 3"},
		{"ERR:ERR unknown subcommand or wrong number of arguments for 'SLOT-STATS'. Try CLUSTER HELP.", "FOO bar"},
		{"ERR:ERR wrong number of arguments for 'cluster|slot-stats' command", "SLOTSRANGE"},
	} {
		args := append([]string{"CLUSTER", "SLOT-STATS"}, splitWords(tc[1])...)
		if got := c.str(args...); got != tc[0] {
			t.Fatalf("%s: %s, want %s", tc[1], got, tc[0])
		}
	}

	if got := c.str("CONFIG", "SET", "cluster-slot-stats-enabled", "yes"); got != "OK" {
		t.Fatalf("CONFIG SET: %s", got)
	}
	defer c.str("CONFIG", "SET", "cluster-slot-stats-enabled", "no")
	usage := func(slot int) (cpu, in, out int64) {
		t.Helper()
		v := c.do("CLUSTER", "SLOT-STATS", "SLOTSRANGE", strconv.Itoa(slot), strconv.Itoa(slot))
		if len(v.Array) != 1 || len(v.Array[0].Array) != 2 || len(v.Array[0].Array[1].Array) != 8 {
			t.Fatalf("SLOT-STATS reply %s", str(v))
		}
		m := v.Array[0].Array[1].Array
		return m[3].Int, m[5].Int, m[7].Int
	}
	if cpu, in, out := usage(5); cpu != 0 || in != 0 || out != 0 {
		t.Fatalf("usage before any command: %d %d %d", cpu, in, out)
	}
	c.str("SET", k, "abc")
	c.str("GET", k)
	c.str("PING")
	c.str("MULTI")
	c.str("INCR", k)
	c.str("EXEC")
	wantIn := reqLen("SET", k, "abc") + reqLen("GET", k) + reqLen("INCR", k) + reqLen("EXEC") + 15
	wantOut := int64(len("+OK\r\n") + len("$3\r\nabc\r\n") + len("*1\r\n-ERR value is not an integer or out of range\r\n"))
	if _, in, out := usage(5); in != wantIn || out != wantOut {
		t.Fatalf("bytes in/out %d %d, want %d %d", in, out, wantIn, wantOut)
	}
	if got := c.str("CLUSTER", "SLOT-STATS", "ORDERBY", "network-bytes-in", "LIMIT", "1"); got[:4] != "[[5 " {
		t.Fatalf("ORDERBY network-bytes-in: %s", got)
	}
	// SPUBLISH also counts the message each subscriber got.
	ch := keyInSlot(t, "ch", 5)
	sub := dialRepl(t, s)
	sub.do("SSUBSCRIBE", ch)
	wantIn += reqLen("SSUBSCRIBE", ch)
	wantOut += int64(len("*3\r\n$10\r\nssubscribe\r\n$" + strconv.Itoa(len(ch)) + "\r\n" + ch + "\r\n:1\r\n"))
	if _, in, out := usage(5); in != wantIn || out != wantOut {
		t.Fatalf("after SSUBSCRIBE bytes in/out %d %d, want %d %d", in, out, wantIn, wantOut)
	}
	_, _, before := usage(5)
	c.str("SPUBLISH", ch, "hello")
	msg := int64(len("*3\r\n$8\r\nsmessage\r\n$"+strconv.Itoa(len(ch))+"\r\n"+ch+"\r\n$5\r\nhello\r\n") + len(":1\r\n"))
	if _, _, out := usage(5); out-before != msg {
		t.Fatalf("SPUBLISH bytes out %d, want %d", out-before, msg)
	}
	wantIn += reqLen("SPUBLISH", ch, "hello")

	// A command refused before routing counts towards no slot.
	c.str("GET", k, "extra")
	if _, in, _ := usage(5); in != wantIn {
		t.Fatalf("a refused command was counted: %d", in)
	}

	if got := c.str("CONFIG", "RESETSTAT"); got != "OK" {
		t.Fatalf("RESETSTAT: %s", got)
	}
	if cpu, in, out := usage(5); cpu != 0 || in != 0 || out != 0 {
		t.Fatalf("usage after RESETSTAT: %d %d %d", cpu, in, out)
	}
	c.str("GET", k)
	if _, in, _ := usage(5); in == 0 {
		t.Fatal("GET was not counted")
	}
	c.str("DEL", k)
	c.str("CLUSTER", "DELSLOTS", "5")
	c.str("CLUSTER", "ADDSLOTS", "5")
	if cpu, in, out := usage(5); cpu != 0 || in != 0 || out != 0 {
		t.Fatalf("usage after the slot changed owner: %d %d %d", cpu, in, out)
	}
}

func splitWords(s string) []string {
	var out []string
	word := ""
	for _, r := range s + " " {
		if r == ' ' {
			if word != "" {
				out = append(out, word)
			}
			word = ""
			continue
		}
		word += string(r)
	}
	return out
}

// A master counts the replication stream a command feeds, once per replica,
// towards the command's slot.
func TestClusterSlotStatsReplicationBytes(t *testing.T) {
	nodes := startBusNodes(t, 2)
	master, replica := nodes[0], nodes[1]
	formMesh(t, nodes)
	if got := master.c.str("CLUSTER", "ADDSLOTSRANGE", "0", "16383"); got != "OK" {
		t.Fatalf("ADDSLOTSRANGE: %s", got)
	}
	eventually(t, 10*time.Second, "the slots to converge", func() bool { return converged(nodes) })
	if got := replica.c.str("CLUSTER", "REPLICATE", master.id); got != "OK" {
		t.Fatalf("REPLICATE: %s", got)
	}
	eventually(t, 15*time.Second, "the replica to sync", func() bool {
		return strings.Contains(master.c.str("INFO", "replication"), "state=online")
	})
	master.c.str("CONFIG", "SET", "cluster-slot-stats-enabled", "yes")
	defer master.c.str("CONFIG", "SET", "cluster-slot-stats-enabled", "no")
	k := keyInSlot(t, "k", 7)
	master.c.str("SET", k, "v")
	v := master.c.do("CLUSTER", "SLOT-STATS", "SLOTSRANGE", "7", "7")
	out := v.Array[0].Array[1].Array[7].Int
	if want := int64(len("+OK\r\n")) + reqLen("SET", k, "v"); out != want {
		t.Fatalf("bytes out %d, want %d", out, want)
	}
}
